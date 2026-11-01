package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/notificacao"
	"github.com/rotaperfumes/shared/notificacao/notificacaoapp"
	"github.com/rotaperfumes/shared/notificacao/notificacaosmtp"
	"github.com/rotaperfumes/shared/vlog"
)

// Email é o envio de e-mail do processo: o Adapter SMTP ou o Null Object
// Noop satisfazem as duas portas (senha gerada e alerta de segurança).
type Email interface {
	identidadeapp.EnviadorSenhaInicial
	notificacaoapp.EnviadorAlerta
}

// NovoEmail escolhe o envio pela config (§5.10/§5.15): SMTP real quando
// SMTP_HOST/USER/PASSWORD/FROM estão presentes, ou o Noop (só log, e-mail
// mascarado), para o `make dev-api` funcionar sem SMTP. Config SMTP
// inválida (não só ausente) derruba o processo (MAIL-01, M-01).
func NovoEmail(cfg *config.Config) Email {
	svc, err := notificacaosmtp.Novo(cfg)
	if err != nil && !errors.Is(err, notificacao.ErrSMTPNaoConfigurado) {
		log.Fatalf("[server] configuração SMTP inválida: %v", err)
	}
	if err != nil {
		log.Printf("[server] SMTP não configurado — emails de senha inicial/reset serão apenas logados (%v)", err)
		log.Printf("[server] SMTP não configurado — alertas de segurança só no log")
		return notificacaosmtp.NovoNoop()
	}
	log.Printf("[server] EmailService SMTP configurado: host=%s port=%s tls=%s server_name=%s from=%s",
		cfg.SMTPHost, cfg.SMTPPort, svc.ModoTLS(), svc.ServerNameTLS(), vlog.MaskEmail(cfg.SMTPFrom))
	return svc
}

// NovoNotificadorAlerta monta o notificador de alerta de reuso (SEC-09,
// I-08) sobre o pool: o contato do usuário vem de Identidade, o envio é o
// informado e a execução é em goroutine. Singleton por processo.
func NovoNotificadorAlerta(pool *sql.DB, enviador notificacaoapp.EnviadorAlerta, admins []string) *notificacaoapp.AlertaSegurancaNotifier {
	return notificacaoapp.NovoAlertaSegurancaNotifier(notificacaoapp.Deps{
		Contatos: NovosContatos(pool),
		Enviador: enviador,
		Admins:   admins,
		Relogio:  time.Now,
		Executor: notificacaoapp.EmGoroutine,
	})
}

// NovosContatos é a porta notificacaoapp.Contatos sobre o adapter MariaDB de
// Identidade (anti-corrupção: Notificação não conhece o Usuario).
func NovosContatos(pool *sql.DB) notificacaoapp.Contatos {
	return contatosIdentidade{usuarios: identidademysql.NovoUsuarios(pool)}
}

type contatosIdentidade struct{ usuarios *identidademysql.Usuarios }

func (c contatosIdentidade) ContatoDoUsuario(ctx context.Context, id int64) (notificacao.Contato, error) {
	u, err := c.usuarios.PorID(ctx, id)
	if err != nil {
		return notificacao.Contato{}, err
	}
	return notificacao.Contato{Nome: u.Nome, Email: u.Email}, nil
}

// assinanteAlerta liga o evento de reuso de Identidade ao notificador
// (Observer, §5.12).
func assinanteAlerta(n *notificacaoapp.AlertaSegurancaNotifier) func(context.Context, identidade.ReusoDeRefreshDetectado) {
	return func(_ context.Context, e identidade.ReusoDeRefreshDetectado) {
		n.Notificar(e.UsuarioID, e.TokenID, e.IP, e.UserAgent)
	}
}
