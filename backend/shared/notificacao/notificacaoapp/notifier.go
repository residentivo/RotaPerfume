package notificacaoapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/rotaperfumes/shared/notificacao"
	"github.com/rotaperfumes/shared/vlog"
)

const arqNotifier = "notifier.go"

// Limites do envio de alertas de segurança (SEC-09, I-08).
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
	// AlertaTimeoutEnvio: ctx próprio do envio (nunca o da requisição).
	AlertaTimeoutEnvio = 30 * time.Second
)

// Deps são as dependências do notificador, todas obrigatórias (panic na
// montagem se faltar alguma).
type Deps struct {
	Contatos Contatos
	Enviador EnviadorAlerta
	// Admins recebem cópia de todo alerta (copiados na montagem).
	Admins []string
	// Relogio afeta só dedup, teto e o horário do alerta; o prazo do envio
	// usa o tempo real.
	Relogio  func() time.Time
	Executor Executor
}

// AlertaSegurancaNotifier envia, de forma assíncrona e limitada, o alerta de
// reuso de refresh token ao usuário e aos administradores. Singleton por
// processo (o estado de dedup e teto é dele).
type AlertaSegurancaNotifier struct {
	contatos Contatos
	enviador EnviadorAlerta
	admins   []string
	agora    func() time.Time
	executar Executor

	mu          sync.Mutex
	ultimoEnvio map[int64]time.Time
	tetoInicio  time.Time
	tetoCount   int
	// ARQ-07 NB-B: fechado (marcado por Aguardar) recusa novos envios, para
	// nenhum emAndamento.Add correr com o Wait do shutdown; ativos permite o
	// caminho rápido de Aguardar. Ambos protegidos por mu.
	fechado bool
	ativos  int

	sem         chan struct{}
	emAndamento sync.WaitGroup
}

// NovoAlertaSegurancaNotifier monta o notificador.
func NovoAlertaSegurancaNotifier(d Deps) *AlertaSegurancaNotifier {
	if d.Contatos == nil || d.Enviador == nil || d.Relogio == nil || d.Executor == nil {
		panic("notificacaoapp: NovoAlertaSegurancaNotifier exige Contatos, Enviador, Relogio e Executor")
	}
	return &AlertaSegurancaNotifier{
		contatos:    d.Contatos,
		enviador:    d.Enviador,
		admins:      append([]string(nil), d.Admins...),
		agora:       d.Relogio,
		executar:    d.Executor,
		ultimoEnvio: make(map[int64]time.Time),
		sem:         make(chan struct{}, AlertaMaxEnviosSimultaneos),
	}
}

// Notificar agenda o alerta de reuso de refresh token. Dedup, teto e fila são
// verificados de forma síncrona; a busca do usuário e os envios rodam no
// Executor com contexto próprio. Nunca bloqueia a requisição.
func (n *AlertaSegurancaNotifier) Notificar(userID, tokenID int64, ip, ua string) {
	agora := n.agora()
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
	if !n.registrarAndamento() {
		// ARQ-07 NB-B: shutdown em curso; o envio não é agendado.
		<-n.sem
		n.desfazerReserva(reserva)
		log.Printf("[auth][seguranca] alerta suprimido (encerrando): user_id=%d", userID)
		return
	}
	vlog.Printf(arqNotifier, "AlertaSegurancaNotifier.Notificar", "decisão alerta agendado user_id=%d", userID)
	alerta := notificacao.AlertaReuso{UsuarioID: userID, IP: ip, UserAgent: ua, TokenID: tokenID, Quando: agora}
	n.executar(func() {
		defer n.concluirAndamento()
		defer func() { <-n.sem }()
		n.enviar(alerta)
	})
}

// Aguardar espera os alertas em andamento terminarem, por até timeout. O
// bootstrap chama antes de fechar o pool, para um envio não perder a busca do
// usuário no meio do shutdown (NB-6). Devolve false se o prazo acabou.
// ARQ-07 NB-B: marca o notificador como fechado antes do Wait (Notificar
// depois disso recusa) e volta na hora se não há envio em andamento.
func (n *AlertaSegurancaNotifier) Aguardar(timeout time.Duration) bool {
	n.mu.Lock()
	n.fechado = true
	nada := n.ativos == 0
	n.mu.Unlock()
	if nada {
		return true
	}
	fim := make(chan struct{})
	go func() {
		n.emAndamento.Wait()
		close(fim)
	}()
	select {
	case <-fim:
		return true
	case <-time.After(timeout):
		return false
	}
}

// registrarAndamento conta um envio em andamento, sob mu e só se o
// notificador não foi fechado (ARQ-07 NB-B). false = encerrando.
func (n *AlertaSegurancaNotifier) registrarAndamento() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.fechado {
		return false
	}
	n.ativos++
	n.emAndamento.Add(1)
	return true
}

// concluirAndamento é o par de registrarAndamento, ao fim do envio.
func (n *AlertaSegurancaNotifier) concluirAndamento() {
	n.mu.Lock()
	n.ativos--
	n.mu.Unlock()
	n.emAndamento.Done()
}

// reservaAlerta identifica o que uma chamada de reservar marcou, para que
// desfazerReserva só reverta a própria marcação.
type reservaAlerta struct {
	userID int64
	marca  time.Time // valor gravado em ultimoEnvio[userID]
	janela time.Time // início da janela do teto em que a vaga foi consumida
}

// reservar aplica dedup por usuário e teto global; devolve o motivo da
// supressão ou "" quando o alerta pode seguir (já marcado como enviado: uma
// falha posterior no envio não desmarca).
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

// enviar busca o usuário e dispara o e-mail ao usuário e a cada admin, com um
// único contexto próprio de AlertaTimeoutEnvio.
func (n *AlertaSegurancaNotifier) enviar(a notificacao.AlertaReuso) {
	ctx, cancel := context.WithTimeout(context.Background(), AlertaTimeoutEnvio)
	defer cancel()

	nome := ""
	contato, err := n.contatos.ContatoDoUsuario(ctx, a.UsuarioID)
	if err != nil {
		log.Printf("[auth][seguranca] alerta: usuário não carregado (só admins): user_id=%d: %v", a.UsuarioID, err)
	} else {
		nome = contato.Nome
		a.EmailUsuario = strings.TrimSpace(contato.Email)
	}

	var enviados, pulados resultadoEnvio
	registrar := func(destino string, paraAdmin bool) {
		err := n.enviador.EnviarAlertaReusoToken(ctx, destino, nome, a, paraAdmin)
		switch {
		case err == nil:
			enviados.contar(paraAdmin)
		case errors.Is(err, notificacao.ErrSMTPNaoConfigurado):
			pulados.contar(paraAdmin)
		default:
			// ARQ-07 NB-B: oculta também o destino (o admin ecoado no erro).
			log.Printf("[auth][seguranca] falha ao enviar alerta: user_id=%d: %s", a.UsuarioID, vlog.OcultarEmail(err, a.EmailUsuario, destino))
		}
	}
	if a.EmailUsuario != "" {
		registrar(a.EmailUsuario, false)
	}
	for _, admin := range n.admins {
		registrar(admin, true)
	}
	n.registrarResultado(a.UsuarioID, enviados, pulados)
}

// registrarResultado loga o desfecho do alerta, sem endereços.
func (n *AlertaSegurancaNotifier) registrarResultado(userID int64, enviados, pulados resultadoEnvio) {
	if enviados.vazio() && pulados.vazio() {
		log.Printf("[auth][seguranca] alerta sem destinatários entregues: user_id=%d", userID)
		return
	}
	if !enviados.vazio() {
		log.Printf("[auth][seguranca] alerta enviado: user_id=%d destinos=%s", userID, enviados)
	}
	if !pulados.vazio() {
		log.Printf("[auth][seguranca] alerta pulado (SMTP não configurado): user_id=%d destinos=%s", userID, pulados)
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
