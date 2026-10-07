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
	"github.com/rotaperfumes/shared/vlog"
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
		db:     db,
		repo:   repositories.NewUsuarioRepository(),
		sender: sender,
		admins: append([]string(nil), admins...),
		now:    time.Now,
		run: func(f func()) {
			vlog.Printf("alerta_seguranca_notifier.go", "NewAlertaSegurancaNotifier.func", "disparando goroutine: f")
			go f()
		},
		ultimoEnvio: make(map[int64]time.Time),
		sem:         make(chan struct{}, AlertaMaxEnviosSimultaneos),
	}
}

// SetNow substitui o relógio (testes).
func (n *AlertaSegurancaNotifier) SetNow(now func() time.Time) {
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.SetNow", "atribuindo now a n.now")
	n.now = now
}

// SetRun substitui o executor assíncrono (testes: ex. execução síncrona).
func (n *AlertaSegurancaNotifier) SetRun(run func(func())) {
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.SetRun", "atribuindo run a n.run")
	n.run = run
}

// SetUsuarioLookup substitui a busca de usuário (testes).
func (n *AlertaSegurancaNotifier) SetUsuarioLookup(l UsuarioLookup) {
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.SetUsuarioLookup", "atribuindo l a n.repo")
	n.repo = l
}

// Notificar agenda o alerta de reuso de refresh token. Dedup, teto e fila são
// verificados de forma síncrona; a busca do usuário e os envios rodam em
// n.run com contexto próprio. Nunca bloqueia a requisição.
func (n *AlertaSegurancaNotifier) Notificar(userID, tokenID int64, ip, ua string) {
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.Notificar", "chamando n.now e declarando agora")
	agora := n.now()
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.Notificar", "chamando n.reservar e declarando reserva, motivo")
	reserva, motivo := n.reservar(userID, agora)
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.Notificar", "verificando condição motivo != \"\"")
	if motivo != "" {
		log.Printf("[auth][seguranca] alerta suprimido (%s): user_id=%d", motivo, userID)
		return
	}

	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.Notificar", "aguardando select entre canais")
	select {
	case n.sem <- struct{}{}:
	default:
		// Descarte por fila cheia não pode deixar o usuário 30 min sem
		// alerta nem consumir o teto: desfaz a reserva desta chamada.
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.Notificar", "chamando n.desfazerReserva")
		n.desfazerReserva(reserva)
		log.Printf("[auth][seguranca] alerta suprimido (fila cheia): user_id=%d", userID)
		return
	}

	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.Notificar", "montando literal sharedsvc.AlertaReuso e declarando alerta")
	alerta := sharedsvc.AlertaReuso{UsuarioID: userID, IP: ip, UserAgent: ua, TokenID: tokenID, Quando: agora}
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.Notificar", "chamando n.run")
	n.run(func() {
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.Notificar.func", "agendando defer: função anônima")
		defer func() {
			vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.Notificar.func.func", "<-n.sem")
			<-n.sem
		}()
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.Notificar.func", "chamando n.enviar")
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
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.reservar", "chamando n.mu.Lock")
	n.mu.Lock()
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.reservar", "agendando defer: n.mu.Unlock")
	defer n.mu.Unlock()

	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.reservar", "chamando n.podarVencidos")
	n.podarVencidos(agora)
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.reservar", "declarando ultimo, ok com n.ultimoEnvio[userID] e verificando condição ok && agora.Sub(ultimo) < AlertaDedupJanela")
	if ultimo, ok := n.ultimoEnvio[userID]; ok && agora.Sub(ultimo) < AlertaDedupJanela {
		return reservaAlerta{}, "dedup"
	}

	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.reservar", "verificando condição n.tetoInicio.IsZero() || agora.Sub(n.tetoInicio) >= alertaTetoJanela")
	if n.tetoInicio.IsZero() || agora.Sub(n.tetoInicio) >= alertaTetoJanela {
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.reservar", "atribuindo agora a n.tetoInicio")
		n.tetoInicio = agora
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.reservar", "atribuindo 0 a n.tetoCount")
		n.tetoCount = 0
	}
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.reservar", "verificando condição n.tetoCount >= AlertaTetoPorHora")
	if n.tetoCount >= AlertaTetoPorHora {
		return reservaAlerta{}, "teto"
	}

	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.reservar", "incrementando n.tetoCount")
	n.tetoCount++
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.reservar", "atribuindo agora a n.ultimoEnvio[userID]")
	n.ultimoEnvio[userID] = agora
	return reservaAlerta{userID: userID, marca: agora, janela: n.tetoInicio}, ""
}

// desfazerReserva reverte uma reserva cujo envio não chegou a ser agendado:
// remove a marcação de dedup (só se ainda for a desta reserva) e devolve a
// vaga do teto (só se a janela ainda for a mesma).
func (n *AlertaSegurancaNotifier) desfazerReserva(r reservaAlerta) {
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.desfazerReserva", "chamando n.mu.Lock")
	n.mu.Lock()
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.desfazerReserva", "agendando defer: n.mu.Unlock")
	defer n.mu.Unlock()

	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.desfazerReserva", "declarando marca, ok com n.ultimoEnvio[r.userID] e verificando condição ok && marca.Equal(r.marca)")
	if marca, ok := n.ultimoEnvio[r.userID]; ok && marca.Equal(r.marca) {
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.desfazerReserva", "chamando delete")
		delete(n.ultimoEnvio, r.userID)
	}
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.desfazerReserva", "verificando condição n.tetoInicio.Equal(r.janela) && n.tetoCount > 0")
	if n.tetoInicio.Equal(r.janela) && n.tetoCount > 0 {
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.desfazerReserva", "decrementando n.tetoCount")
		n.tetoCount--
	}
}

// podarVencidos remove marcações fora da janela de dedup. Chamar com mu.
func (n *AlertaSegurancaNotifier) podarVencidos(agora time.Time) {
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.podarVencidos", "iniciando loop range sobre n.ultimoEnvio")
	for id, t := range n.ultimoEnvio {
		if agora.Sub(t) >= AlertaDedupJanela {
			delete(n.ultimoEnvio, id)
		}
	}
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.podarVencidos", "loop range concluído sobre n.ultimoEnvio: %d itens", len(n.ultimoEnvio))
}

// enviar busca o usuário e dispara o e-mail ao usuário e a cada admin.
func (n *AlertaSegurancaNotifier) enviar(a sharedsvc.AlertaReuso) {
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "chamando context.WithTimeout e declarando ctx, cancel")
	ctx, cancel := context.WithTimeout(context.Background(), alertaTimeoutEnvio)
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "agendando defer: cancel")
	defer cancel()

	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "declarando nome com literal string")
	nome := ""
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "chamando n.repo.GetByID e declarando u, err")
	u, err := n.repo.GetByID(ctx, n.db, a.UsuarioID)
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "verificando condição err != nil")
	if err != nil {
		log.Printf("[auth][seguranca] alerta: usuário não carregado (só admins): user_id=%d: %v", a.UsuarioID, err)
	} else if u != nil {
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "atribuindo u.Nome a nome")
		nome = u.Nome
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "chamando strings.TrimSpace e atribuindo a a.EmailUsuario")
		a.EmailUsuario = strings.TrimSpace(u.Email)
	}

	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "declarando enviados, pulados")
	var enviados, pulados resultadoEnvio
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "definindo função anônima e declarando registrar")
	registrar := func(destino string, paraAdmin bool) {
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar.func", "chamando n.sender.EnviarAlertaReusoToken e declarando err")
		err := n.sender.EnviarAlertaReusoToken(ctx, destino, nome, a, paraAdmin)
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar.func", "avaliando switch")
		switch {
		case err == nil:
			vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar.func", "chamando enviados.contar")
			enviados.contar(paraAdmin)
		case errors.Is(err, sharedsvc.ErrSMTPNaoConfigurado):
			vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar.func", "chamando pulados.contar")
			pulados.contar(paraAdmin)
		default:
			log.Printf("[auth][seguranca] falha ao enviar alerta: user_id=%d: %s", a.UsuarioID, ocultarEmail(err, a.EmailUsuario))
		}
	}

	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "verificando condição a.EmailUsuario != \"\"")
	if a.EmailUsuario != "" {
		vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "chamando registrar")
		registrar(a.EmailUsuario, false)
	}
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "iniciando loop range sobre n.admins")
	for _, admin := range n.admins {
		registrar(admin, true)
	}
	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "loop range concluído sobre n.admins: %d itens", len(n.admins))

	vlog.Printf("alerta_seguranca_notifier.go", "AlertaSegurancaNotifier.enviar", "verificando condição enviados.vazio() && pulados.vazio()")
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
	vlog.Printf("alerta_seguranca_notifier.go", "resultadoEnvio.contar", "verificando condição paraAdmin")
	if paraAdmin {
		vlog.Printf("alerta_seguranca_notifier.go", "resultadoEnvio.contar", "incrementando r.admins")
		r.admins++
		return
	}
	vlog.Printf("alerta_seguranca_notifier.go", "resultadoEnvio.contar", "atribuindo true a r.usuario")
	r.usuario = true
}

func (r resultadoEnvio) vazio() bool { return !r.usuario && r.admins == 0 }

// String formata os destinos sem endereços: "usuario", "admin(N)" ou ambos.
func (r resultadoEnvio) String() string {
	vlog.Printf("alerta_seguranca_notifier.go", "resultadoEnvio.String", "chamando make e declarando destinos")
	destinos := make([]string, 0, 2)
	vlog.Printf("alerta_seguranca_notifier.go", "resultadoEnvio.String", "verificando condição r.usuario")
	if r.usuario {
		vlog.Printf("alerta_seguranca_notifier.go", "resultadoEnvio.String", "chamando append e atribuindo a destinos")
		destinos = append(destinos, "usuario")
	}
	vlog.Printf("alerta_seguranca_notifier.go", "resultadoEnvio.String", "verificando condição r.admins > 0")
	if r.admins > 0 {
		vlog.Printf("alerta_seguranca_notifier.go", "resultadoEnvio.String", "chamando append e atribuindo a destinos")
		destinos = append(destinos, fmt.Sprintf("admin(%d)", r.admins))
	}
	return strings.Join(destinos, ",")
}

// ocultarEmail remove o e-mail do usuário da mensagem de erro (servidores SMTP
// costumam ecoar o destinatário) — o log nunca deve conter esse e-mail.
func ocultarEmail(err error, email string) string {
	vlog.Printf("alerta_seguranca_notifier.go", "ocultarEmail", "chamando err.Error e declarando msg")
	msg := err.Error()
	vlog.Printf("alerta_seguranca_notifier.go", "ocultarEmail", "verificando condição email == \"\"")
	if email == "" {
		return msg
	}
	return strings.ReplaceAll(msg, email, "[email-usuario]")
}
