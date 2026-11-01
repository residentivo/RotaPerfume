package notificacaosmtp

import (
	"context"
	"log"

	"github.com/rotaperfumes/shared/notificacao"
	"github.com/rotaperfumes/shared/vlog"
)

// Noop é o Null Object do envio (§5.15): usado quando o SMTP não está
// configurado (dev sem SMTP_USER/SMTP_PASSWORD). Não faz I/O; só registra
// que o envio foi pulado, com o e-mail mascarado e nunca a senha (M-02).
type Noop struct{}

// NovoNoop cria o envio "só log".
func NovoNoop() Noop { return Noop{} }

// EnviarSenhaInicial não envia: registra o envio pulado e devolve nil.
func (Noop) EnviarSenhaInicial(_ context.Context, destinatario, _, _ string) error {
	log.Printf("[email] envio pulado: SMTP não configurado — destinatario=%s (senha gerada, ver auditoria de senha_historico)", vlog.MaskEmail(destinatario))
	return nil
}

// EnviarAlertaReusoToken não envia: registra o alerta pulado (sem o e-mail do
// usuário) e devolve notificacao.ErrSMTPNaoConfigurado, para o notificador
// não registrar o alerta como "enviado" (LOG-01).
func (Noop) EnviarAlertaReusoToken(_ context.Context, _, _ string, a notificacao.AlertaReuso, paraAdmin bool) error {
	log.Printf("[email] alerta de seguranca pulado: SMTP não configurado user_id=%d para_admin=%v", a.UsuarioID, paraAdmin)
	return notificacao.ErrSMTPNaoConfigurado
}
