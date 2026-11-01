// Package notificacaoapp é a aplicação do contexto Notificação (ARQ-07): o
// AlertaSegurancaNotifier (SEC-09), assinante do evento de reuso de refresh
// token publicado por Identidade (o bootstrap faz a ligação), e as portas
// que ele consome. notificacaosmtp e o adapter de Identidade (no bootstrap)
// as satisfazem implicitamente (docs/arquitetura-ddd.md §4.1).
package notificacaoapp

import (
	"context"

	"github.com/rotaperfumes/shared/notificacao"
)

// EnviadorAlerta envia o alerta de reuso por e-mail. paraAdmin controla se o
// corpo inclui os identificadores internos (user_id, e-mail do usuário,
// token_id), nunca incluídos no e-mail ao próprio usuário. O Noop devolve
// notificacao.ErrSMTPNaoConfigurado (envio pulado).
type EnviadorAlerta interface {
	EnviarAlertaReusoToken(ctx context.Context, destinatario, nomeUsuario string, a notificacao.AlertaReuso, paraAdmin bool) error
}

// Contatos busca o nome e o e-mail do usuário do alerta (porta para o
// contexto Identidade, ligada no bootstrap).
type Contatos interface {
	ContatoDoUsuario(ctx context.Context, usuarioID int64) (notificacao.Contato, error)
}

// Executor roda o envio fora da requisição (produção: EmGoroutine; testes:
// síncrono ou com sinalização de fim).
type Executor func(func())

// EmGoroutine é o Executor de produção.
func EmGoroutine(f func()) { go f() }
