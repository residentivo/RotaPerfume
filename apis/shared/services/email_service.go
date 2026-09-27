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
	"time"
	"unicode"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tz"
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

// AlertaReuso descreve um reuso de refresh token já rotacionado (SEC-09),
// usado para montar o e-mail de alerta de segurança.
type AlertaReuso struct {
	UsuarioID    int64
	EmailUsuario string
	IP           string
	UserAgent    string
	TokenID      int64
	Quando       time.Time
}

// AlertaSegurancaSender envia alertas de segurança por e-mail. É separada de
// EmailService para não quebrar os fakes existentes.
type AlertaSegurancaSender interface {
	// EnviarAlertaReusoToken avisa sobre reuso de refresh token. paraAdmin
	// controla se o corpo inclui os identificadores internos (user_id, e-mail
	// do usuário, token_id) — nunca incluídos no e-mail ao próprio usuário.
	EnviarAlertaReusoToken(ctx context.Context, destinatario, nomeUsuario string, a AlertaReuso, paraAdmin bool) error
}

// ErrDestinatarioInvalido é retornado quando o destinatário contém CR/LF
// (tentativa de injeção de cabeçalho) ou está vazio.
var ErrDestinatarioInvalido = errors.New("services: destinatário de e-mail inválido")

// assuntoAlertaSeguranca é fixo — nunca contém dados do usuário/requisição.
const assuntoAlertaSeguranca = "Alerta de segurança — RotaPerfumes"

// maxUserAgentAlerta é o tamanho máximo (em runes) do User-Agent no alerta.
const maxUserAgentAlerta = 120

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
	// tlsConfig é usado no STARTTLS. ServerName é sempre o host configurado
	// (validação do certificado nunca é desligada por aqui).
	tlsConfig *tls.Config
}

// NewSMTPEmailService cria um EmailService real a partir da configuração.
// Retorna erro se host/user/password/from não estiverem preenchidos —
// use NewNoopEmailService como fallback quando SMTP não estiver configurado.
func NewSMTPEmailService(cfg *config.Config) (*SMTPEmailService, error) {
	return NewSMTPEmailServiceWithTLSConfig(cfg, nil)
}

// NewSMTPEmailServiceWithTLSConfig é NewSMTPEmailService com a configuração
// TLS do STARTTLS injetável (ex.: RootCAs de uma CA própria, ou a CA de um
// servidor SMTP de teste). tlsCfg nil usa as raízes do sistema. A config é
// clonada e, sobre o clone, o comando impõe (SEC-10): InsecureSkipVerify
// sempre false (o certificado do servidor é sempre verificado, mesmo que o
// chamador tenha desligado), MinVersion mínima TLS 1.2 (só sobe; uma config
// que já exige TLS 1.3 é mantida) e ServerName = cfg.SMTPHost.
func NewSMTPEmailServiceWithTLSConfig(cfg *config.Config, tlsCfg *tls.Config) (*SMTPEmailService, error) {
	if cfg.SMTPHost == "" || cfg.SMTPUser == "" || cfg.SMTPPassword == "" || cfg.SMTPFrom == "" {
		return nil, ErrSMTPNaoConfigurado
	}
	if tlsCfg == nil {
		tlsCfg = &tls.Config{}
	} else {
		tlsCfg = tlsCfg.Clone()
	}
	tlsCfg.InsecureSkipVerify = false
	if tlsCfg.MinVersion < tls.VersionTLS12 {
		tlsCfg.MinVersion = tls.VersionTLS12
	}
	tlsCfg.ServerName = cfg.SMTPHost
	return &SMTPEmailService{
		host:      cfg.SMTPHost,
		port:      cfg.SMTPPort,
		user:      cfg.SMTPUser,
		pass:      cfg.SMTPPassword,
		from:      cfg.SMTPFrom,
		tlsConfig: tlsCfg,
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

// EnviarAlertaReusoToken envia o alerta de reuso de refresh token (SEC-09).
// Nunca inclui o token, hash ou senha; ao usuário também omite user_id e
// token_id.
func (s *SMTPEmailService) EnviarAlertaReusoToken(ctx context.Context, destinatario, nomeUsuario string, a AlertaReuso, paraAdmin bool) error {
	if !destinatarioValido(destinatario) {
		return ErrDestinatarioInvalido
	}
	return s.enviar(ctx, destinatario, assuntoAlertaSeguranca, montarCorpoAlertaReuso(nomeUsuario, a, paraAdmin))
}

// montarCorpoAlertaReuso monta o texto do alerta. Nome, IP e User-Agent são
// saneados (sem caracteres de controle) e o User-Agent é truncado.
func montarCorpoAlertaReuso(nomeUsuario string, a AlertaReuso, paraAdmin bool) string {
	nome := removerControles(nomeUsuario)
	if nome == "" {
		nome = "usuário"
	}
	ip := removerControles(a.IP)
	ua := truncarRunes(removerControles(a.UserAgent), maxUserAgentAlerta)
	quando := a.Quando.In(tz.Local).Format("02/01/2006 15:04:05") + " (horário de Brasília)"

	var sb strings.Builder
	if paraAdmin {
		sb.WriteString("Alerta de segurança: reuso de sessão já encerrada (possível roubo de token).\r\n\r\n")
	} else {
		sb.WriteString("Olá, " + nome + ".\r\n\r\n")
		sb.WriteString("Detectamos o uso de uma credencial de sessão já substituída na sua conta RotaPerfumes, o que pode indicar acesso indevido.\r\n\r\n")
	}
	sb.WriteString("Data/hora: " + quando + "\r\n")
	sb.WriteString("IP de origem: " + ip + "\r\n")
	sb.WriteString("Navegador/dispositivo: " + ua + "\r\n")
	if paraAdmin {
		sb.WriteString(fmt.Sprintf("Usuário: %s (user_id=%d, e-mail=%s)\r\n", nome, a.UsuarioID, removerControles(a.EmailUsuario)))
		sb.WriteString(fmt.Sprintf("token_id: %d\r\n", a.TokenID))
		sb.WriteString("\r\nTodas as sessões do usuário foram encerradas automaticamente.\r\n")
		return sb.String()
	}
	sb.WriteString("\r\nPor segurança, encerramos todas as suas sessões; faça login novamente.\r\n")
	sb.WriteString("Se você não reconhece esse acesso, troque a senha e contate o administrador do sistema.\r\n")
	return sb.String()
}

// removerControles remove caracteres de controle (CR, LF, TAB, NUL etc.) e
// separadores de linha Unicode, evitando quebra/injeção no corpo do e-mail.
func removerControles(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return -1
		}
		return r
	}, s)
}

// truncarRunes limita s a max runes (sem cortar caracteres UTF-8 ao meio).
func truncarRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// destinatarioValido recusa destinatários vazios ou com CR/LF — buildMessage
// escreve o cabeçalho To: sem escape.
func destinatarioValido(to string) bool {
	return strings.TrimSpace(to) != "" && !strings.ContainsAny(to, "\r\n")
}

// enviar executa o handshake SMTP manual com STARTTLS (necessário para Gmail
// na porta 587 — net/smtp.SendMail não faz STARTTLS explícito).
func (s *SMTPEmailService) enviar(ctx context.Context, to, subject, body string) error {
	// CR/LF é checado no valor cru (TrimSpace esconderia um "\r\n" final);
	// depois apara — o mesmo valor aparado vai para RCPT TO e para To:.
	if !destinatarioValido(to) {
		return ErrDestinatarioInvalido
	}
	to = strings.TrimSpace(to)
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
		if err := client.StartTLS(s.tlsConfig); err != nil {
			return fmt.Errorf("services: smtp starttls: %w", err)
		}
	} else {
		return fmt.Errorf("services: smtp starttls: not supported by server")
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

// EnviarAlertaReusoToken não envia e-mail; apenas registra que o alerta foi
// pulado (sem e-mail do usuário no log).
func (n *NoopEmailService) EnviarAlertaReusoToken(ctx context.Context, destinatario, nomeUsuario string, a AlertaReuso, paraAdmin bool) error {
	log.Printf("[email] alerta de seguranca pulado: SMTP não configurado user_id=%d para_admin=%v", a.UsuarioID, paraAdmin)
	return nil
}
