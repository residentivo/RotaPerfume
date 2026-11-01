// Package notificacao é o domínio do contexto Notificação (ARQ-07): o alerta
// de reuso de refresh token (SEC-09), o destinatário validado, o modo TLS do
// SMTP (MAIL-01) e o texto das mensagens. Puro: sem rede, sem log e sem
// config. O envio é da infraestrutura (notificacaosmtp) e o agendamento do
// alerta, da aplicação (notificacaoapp).
package notificacao

import (
	"errors"
	"time"
)

var (
	// ErrSMTPNaoConfigurado: as credenciais SMTP não estão no ambiente. É o
	// erro do construtor do SMTP (o bootstrap cai no Noop) e o do
	// Noop.EnviarAlertaReusoToken (envio pulado, LOG-01).
	ErrSMTPNaoConfigurado = errors.New("notificacao: SMTP não configurado")
	// ErrDestinatarioInvalido: destinatário vazio ou com CR/LF (tentativa de
	// injeção de cabeçalho, M-01).
	ErrDestinatarioInvalido = errors.New("notificacao: destinatário de e-mail inválido")
	// ErrModoTLSInvalido: SMTP_TLS_MODE fora de implicit|starttls.
	ErrModoTLSInvalido = errors.New("notificacao: SMTP_TLS_MODE inválido (use implicit ou starttls)")
)

// AlertaReuso descreve um reuso de refresh token já rotacionado (SEC-09).
// Nunca carrega o token, o hash nem a senha.
type AlertaReuso struct {
	UsuarioID    int64
	EmailUsuario string
	IP           string
	UserAgent    string
	TokenID      int64
	Quando       time.Time
}

// Contato é o nome e o e-mail do usuário do alerta (lidos de Identidade pela
// porta notificacaoapp.Contatos).
type Contato struct {
	Nome  string
	Email string
}
