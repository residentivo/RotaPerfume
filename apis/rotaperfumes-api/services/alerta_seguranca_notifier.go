package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

// Limites do envio de alertas de segurança (SEC-09).
const (
	// AlertaDedupJanela: no máximo um alerta por usuário nesta janela. Necessário
	// porque o token reutilizado continua com motivo "rotacao" (a revogação em
	// massa só atinge revoked_at IS NULL) e cada replay geraria novo alerta.
	AlertaDedupJanela = 30 * time.Minute
	// AlertaTetoPorHora: teto global de alertas por janela fixa de 1 hora.
	AlertaTetoPorHora = 20
	alertaTetoJanela  = time.Hour
	// AlertaMaxEnviosSimultaneos: envios em andamento ao mesmo tempo.
	AlertaMaxEnviosSimultaneos = 4
	// alertaTimeoutEnvio: ctx próprio do envio (nunca o da requisição).
	alertaTimeoutEnvio = 30 * time.Second
)

// UsuarioLookup busca o usuário do alerta. Satisfeita por
// *repositories.UsuarioRepository; injetável em testes.
type UsuarioLookup interface {
	GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Usuario, error)
}

// AlertaSegurancaNotifier envia, de forma assíncrona e limitada, o alerta de
// reuso de refresh token ao usuário e aos administradores.
type AlertaSegurancaNotifier struct {
	db     *sql.DB
	repo   UsuarioLookup
	sender sharedsvc.AlertaSegurancaSender
	admins []string
	now    func() time.Time
	run    func(func())

	mu          sync.Mutex
	ultimoEnvio map[int64]time.Time
	tetoInicio  time.Time
	tetoCount   int

	sem chan struct{}
}

// NewAlertaSegurancaNotifier cria o notifier com as dependências padrão
// (repositório de usuários, relógio real e execução em goroutine).
func NewAlertaSegurancaNotifier(db *sql.DB, sender sharedsvc.AlertaSegurancaSender, admins []string) *AlertaSegurancaNotifier {
	return &AlertaSegurancaNotifier{
		db:          db,
		repo:        repositories.NewUsuarioRepository(),
		sender:      sender,
		admins:      append([]string(nil), admins...),
		now:         time.Now,
		run:         func(f func()) { go f() },
		ultimoEnvio: make(map[int64]time.Time),
		sem:         make(chan struct{}, AlertaMaxEnviosSimultaneos),
	}
}

// SetNow substitui o relógio (testes).
func (n *AlertaSegurancaNotifier) SetNow(now func() time.Time) { n.now = now }

// SetRun substitui o executor assíncrono (testes: ex. execução síncrona).
func (n *AlertaSegurancaNotifier) SetRun(run func(func())) { n.run = run }

// SetUsuarioLookup substitui a busca de usuário (testes).
func (n *AlertaSegurancaNotifier) SetUsuarioLookup(l UsuarioLookup) { n.repo = l }

// Notificar agenda o alerta de reuso de refresh token. Dedup, teto e fila são
// verificados de forma síncrona; a busca do usuário e os envios rodam em
// n.run com contexto próprio. Nunca bloqueia a requisição.
func (n *AlertaSegurancaNotifier) Notificar(userID, tokenID int64, ip, ua string) {
	agora := n.now()
	reserva, motivo := n.reservar(userID, agora)
	if motivo != "" {
		log.Printf("[auth][seguranca] alerta suprimido (%s): user_id=%d", motivo, userID)
		return
	}

	select {
	case n.sem <- struct{}{}:
	default:
		// Descarte por fila cheia não pode deixar o usuário 30 min sem
		// alerta nem consumir o teto: desfaz a reserva desta chamada.
		n.desfazerReserva(reserva)
		log.Printf("[auth][seguranca] alerta suprimido (fila cheia): user_id=%d", userID)
		return
	}

	alerta := sharedsvc.AlertaReuso{UsuarioID: userID, IP: ip, UserAgent: ua, TokenID: tokenID, Quando: agora}
	n.run(func() {
		defer func() { <-n.sem }()
		n.enviar(alerta)
	})
}

// reservaAlerta identifica o que uma chamada de reservar marcou, para que
// desfazerReserva só reverta a própria marcação.
type reservaAlerta struct {
	userID int64
	marca  time.Time // valor gravado em ultimoEnvio[userID]
	janela time.Time // início da janela do teto em que a vaga foi consumida
}

// reservar aplica dedup por usuário e teto global; devolve o motivo da
// supressão ou "" quando o alerta pode seguir (já marcado como enviado —
// uma falha posterior no envio não desmarca).
func (n *AlertaSegurancaNotifier) reservar(userID int64, agora time.Time) (reservaAlerta, string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.podarVencidos(agora)
	if ultimo, ok := n.ultimoEnvio[userID]; ok && agora.Sub(ultimo) < AlertaDedupJanela {
		return reservaAlerta{}, "dedup"
	}

	if n.tetoInicio.IsZero() || agora.Sub(n.tetoInicio) >= alertaTetoJanela {
		n.tetoInicio = agora
		n.tetoCount = 0
	}
	if n.tetoCount >= AlertaTetoPorHora {
		return reservaAlerta{}, "teto"
	}

	n.tetoCount++
	n.ultimoEnvio[userID] = agora
	return reservaAlerta{userID: userID, marca: agora, janela: n.tetoInicio}, ""
}

// desfazerReserva reverte uma reserva cujo envio não chegou a ser agendado:
// remove a marcação de dedup (só se ainda for a desta reserva) e devolve a
// vaga do teto (só se a janela ainda for a mesma).
func (n *AlertaSegurancaNotifier) desfazerReserva(r reservaAlerta) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if marca, ok := n.ultimoEnvio[r.userID]; ok && marca.Equal(r.marca) {
		delete(n.ultimoEnvio, r.userID)
	}
	if n.tetoInicio.Equal(r.janela) && n.tetoCount > 0 {
		n.tetoCount--
	}
}

// podarVencidos remove marcações fora da janela de dedup. Chamar com mu.
func (n *AlertaSegurancaNotifier) podarVencidos(agora time.Time) {
	for id, t := range n.ultimoEnvio {
		if agora.Sub(t) >= AlertaDedupJanela {
			delete(n.ultimoEnvio, id)
		}
	}
}

// enviar busca o usuário e dispara o e-mail ao usuário e a cada admin.
func (n *AlertaSegurancaNotifier) enviar(a sharedsvc.AlertaReuso) {
	ctx, cancel := context.WithTimeout(context.Background(), alertaTimeoutEnvio)
	defer cancel()

	nome := ""
	u, err := n.repo.GetByID(ctx, n.db, a.UsuarioID)
	if err != nil {
		log.Printf("[auth][seguranca] alerta: usuário não carregado (só admins): user_id=%d: %v", a.UsuarioID, err)
	} else if u != nil {
		nome = u.Nome
		a.EmailUsuario = strings.TrimSpace(u.Email)
	}

	var enviados, pulados resultadoEnvio
	registrar := func(destino string, paraAdmin bool) {
		err := n.sender.EnviarAlertaReusoToken(ctx, destino, nome, a, paraAdmin)
		switch {
		case err == nil:
			enviados.contar(paraAdmin)
		case errors.Is(err, sharedsvc.ErrSMTPNaoConfigurado):
			pulados.contar(paraAdmin)
		default:
			log.Printf("[auth][seguranca] falha ao enviar alerta: user_id=%d: %s", a.UsuarioID, ocultarEmail(err, a.EmailUsuario))
		}
	}

	if a.EmailUsuario != "" {
		registrar(a.EmailUsuario, false)
	}
	for _, admin := range n.admins {
		registrar(admin, true)
	}

	if enviados.vazio() && pulados.vazio() {
		log.Printf("[auth][seguranca] alerta sem destinatários entregues: user_id=%d", a.UsuarioID)
		return
	}
	if !enviados.vazio() {
		log.Printf("[auth][seguranca] alerta enviado: user_id=%d destinos=%s", a.UsuarioID, enviados)
	}
	if !pulados.vazio() {
		log.Printf("[auth][seguranca] alerta pulado (SMTP não configurado): user_id=%d destinos=%s", a.UsuarioID, pulados)
	}
}

// resultadoEnvio conta destinos de um alerta (usuário e admins) para o log.
type resultadoEnvio struct {
	usuario bool
	admins  int
}

func (r *resultadoEnvio) contar(paraAdmin bool) {
	if paraAdmin {
		r.admins++
		return
	}
	r.usuario = true
}

func (r resultadoEnvio) vazio() bool { return !r.usuario && r.admins == 0 }

// String formata os destinos sem endereços: "usuario", "admin(N)" ou ambos.
func (r resultadoEnvio) String() string {
	destinos := make([]string, 0, 2)
	if r.usuario {
		destinos = append(destinos, "usuario")
	}
	if r.admins > 0 {
		destinos = append(destinos, fmt.Sprintf("admin(%d)", r.admins))
	}
	return strings.Join(destinos, ",")
}

// ocultarEmail remove o e-mail do usuário da mensagem de erro (servidores SMTP
// costumam ecoar o destinatário) — o log nunca deve conter esse e-mail.
func ocultarEmail(err error, email string) string {
	msg := err.Error()
	if email == "" {
		return msg
	}
	return strings.ReplaceAll(msg, email, "[email-usuario]")
}
