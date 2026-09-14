package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"

	"github.com/rotaperfumes/shared/config"
)

// ErrSMTPNaoConfigurado é retornado (ou apenas logado, conforme a implementação)
// quando as credenciais SMTP não estão presentes no ambiente.
var ErrSMTPNaoConfigurado = errors.New("services: SMTP não configurado")

// EmailService abstrai o envio de emails transacionais do sistema.
//
// A interface existe para permitir mocks/fakes em testes — nenhum teste deve
// depender de um servidor SMTP real.
type EmailService interface {
	// EnviarSenhaInicial envia ao destinatário a senha recém-gerada (criação
	// de conta ou reset administrativo). O corpo do email é padronizado
	// internamente pela implementação.
	EnviarSenhaInicial(ctx context.Context, destinatario, nomeUsuario, senha string) error
}

// ---------------------------------------------------------------------------
// Implementação SMTP (Gmail com STARTTLS)
// ---------------------------------------------------------------------------

// SMTPEmailService envia emails via SMTP com STARTTLS (compatível com Gmail
// usando uma "senha de app" — myaccount.google.com/apppasswords).
type SMTPEmailService struct {
	host string
	port string
	user string
	pass string
	from string
}

// NewSMTPEmailService cria um EmailService real a partir da configuração.
// Retorna erro se host/user/password/from não estiverem preenchidos —
// use NewNoopEmailService como fallback quando SMTP não estiver configurado.
func NewSMTPEmailService(cfg *config.Config) (*SMTPEmailService, error) {
	if cfg.SMTPHost == "" || cfg.SMTPUser == "" || cfg.SMTPPassword == "" || cfg.SMTPFrom == "" {
		return nil, ErrSMTPNaoConfigurado
	}
	return &SMTPEmailService{
		host: cfg.SMTPHost,
		port: cfg.SMTPPort,
		user: cfg.SMTPUser,
		pass: cfg.SMTPPassword,
		from: cfg.SMTPFrom,
	}, nil
}

// EnviarSenhaInicial envia a senha gerada para o email cadastrado do usuário.
// Nunca loga a senha em texto claro.
func (s *SMTPEmailService) EnviarSenhaInicial(ctx context.Context, destinatario, nomeUsuario, senha string) error {
	assunto := "Sua senha de acesso — RotaPerfumes"
	corpo := fmt.Sprintf(
		"Olá, %s.\r\n\r\n"+
			"Sua senha de acesso ao sistema RotaPerfumes foi gerada:\r\n\r\n"+
			"    %s\r\n\r\n"+
			"Por segurança, recomendamos trocar essa senha assim que possível após o login.\r\n\r\n"+
			"Se você não solicitou isso, contate o administrador do sistema.\r\n",
		nomeUsuario, senha,
	)
	return s.enviar(ctx, destinatario, assunto, corpo)
}

// enviar executa o handshake SMTP manual com STARTTLS (necessário para Gmail
// na porta 587 — net/smtp.SendMail não faz STARTTLS explícito).
func (s *SMTPEmailService) enviar(ctx context.Context, to, subject, body string) error {
	addr := net.JoinHostPort(s.host, s.port)

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("services: smtp dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("services: smtp client: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{ServerName: s.host}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("services: smtp starttls: %w", err)
		}
	}else {
    // Se o servidor não suporta STARTTLS, interrompe a execução por segurança
    return fmt.Errorf("services: smtp: servidor nao suporta STARTTLS")
}

	auth := smtp.PlainAuth("", s.user, s.pass, s.host)
	if ok, _ := client.Extension("AUTH"); ok {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("services: smtp auth: %w", err)
		}
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("services: smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("services: smtp rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("services: smtp data: %w", err)
	}

	msg := buildMessage(s.from, to, subject, body)
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("services: smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("services: smtp close: %w", err)
	}

	return client.Quit()
}

// buildMessage monta o payload RFC 5322 mínimo (headers + corpo em texto puro).
func buildMessage(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

// ---------------------------------------------------------------------------
// Implementação Noop (fallback de dev / testes)
// ---------------------------------------------------------------------------

// NoopEmailService é usado quando SMTP não está configurado (dev sem
// SMTP_USER/SMTP_PASSWORD). Apenas loga que o envio foi pulado — nunca loga
// a senha em texto claro.
type NoopEmailService struct{}

// NewNoopEmailService cria um EmailService "log-only".
func NewNoopEmailService() *NoopEmailService {
	return &NoopEmailService{}
}

// EnviarSenhaInicial não envia email de fato; apenas registra em log que o
// envio foi pulado por falta de configuração SMTP.
func (n *NoopEmailService) EnviarSenhaInicial(ctx context.Context, destinatario, nomeUsuario, senha string) error {
	log.Printf("[email] envio pulado: SMTP não configurado — destinatario=%s (senha gerada, ver auditoria de senha_historico)", destinatario)
	return nil
}
