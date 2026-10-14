package bootstrap

import (
	"errors"
	"log"

	"github.com/rotaperfumes/shared/config"
	sharedsvc "github.com/rotaperfumes/shared/services"
	"github.com/rotaperfumes/shared/vlog"
)

// novoEmail escolhe o EmailService pela config: SMTP real quando
// SMTP_USER/SMTP_PASSWORD/SMTP_FROM estão presentes, ou o Null Object noop
// (só log, e-mail mascarado) — assim `make dev-api` funciona sem SMTP.
// Config SMTP inválida (não só ausente) derruba o processo (MAIL-01, M-01).
//
// Devolve também o AlertaSegurancaSender (SEC-09): a mesma instância
// implementa as duas interfaces.
func novoEmail(cfg *config.Config) (sharedsvc.EmailService, sharedsvc.AlertaSegurancaSender) {
	svc, err := sharedsvc.NewSMTPEmailService(cfg)
	if err != nil && !errors.Is(err, sharedsvc.ErrSMTPNaoConfigurado) {
		log.Fatalf("[server] configuração SMTP inválida: %v", err)
	}
	if err != nil {
		log.Printf("[server] SMTP não configurado — emails de senha inicial/reset serão apenas logados (%v)", err)
		log.Printf("[server] SMTP não configurado — alertas de segurança só no log")
		noop := sharedsvc.NewNoopEmailService()
		return noop, noop
	}
	log.Printf("[server] EmailService SMTP configurado: host=%s port=%s tls=%s server_name=%s from=%s",
		cfg.SMTPHost, cfg.SMTPPort, svc.ModoTLS(), svc.ServerNameTLS(), vlog.MaskEmail(cfg.SMTPFrom))
	return svc, svc
}
