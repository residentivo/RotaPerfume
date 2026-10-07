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
	"github.com/rotaperfumes/shared/vlog"
)

// ErrSMTPNaoConfigurado é retornado quando as credenciais SMTP não estão
// presentes no ambiente: por NewSMTPEmailService e pelo
// NoopEmailService.EnviarAlertaReusoToken (envio pulado). EnviarSenhaInicial
// do Noop apenas loga e devolve nil.
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
	vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "verificando se host/usuário/senha/remetente SMTP estão preenchidos (valores não logados)")
	if cfg.SMTPHost == "" || cfg.SMTPUser == "" || cfg.SMTPPassword == "" || cfg.SMTPFrom == "" {
		return nil, ErrSMTPNaoConfigurado
	}
	vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "verificando se foi injetada config TLS (injetada=%t)", tlsCfg != nil)
	if tlsCfg == nil {
		vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "criando config TLS padrão")
		tlsCfg = &tls.Config{}
	} else {
		vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "clonando config TLS injetada")
		tlsCfg = tlsCfg.Clone()
	}
	vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "forçando verificação de certificado (InsecureSkipVerify=false)")
	tlsCfg.InsecureSkipVerify = false
	vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "verificando se MinVersion é inferior a TLS 1.2")
	if tlsCfg.MinVersion < tls.VersionTLS12 {
		vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "elevando MinVersion para TLS 1.2")
		tlsCfg.MinVersion = tls.VersionTLS12
	}
	vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "definindo ServerName do TLS com o host SMTP")
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
	vlog.Printf("email_service.go", "SMTPEmailService.EnviarSenhaInicial", "definindo assunto do e-mail de senha inicial para %s", vlog.MaskEmail(destinatario))
	assunto := "Sua senha de acesso — RotaPerfumes"
	vlog.Printf("email_service.go", "SMTPEmailService.EnviarSenhaInicial", "montando corpo do e-mail (corpo e senha não logados)")
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
	vlog.Printf("email_service.go", "SMTPEmailService.EnviarAlertaReusoToken", "validando destinatário %s (usuario_id=%d, para_admin=%t)", vlog.MaskEmail(destinatario), a.UsuarioID, paraAdmin)
	if !destinatarioValido(destinatario) {
		return ErrDestinatarioInvalido
	}
	return s.enviar(ctx, destinatario, assuntoAlertaSeguranca, montarCorpoAlertaReuso(nomeUsuario, a, paraAdmin))
}

// montarCorpoAlertaReuso monta o texto do alerta. Nome, IP e User-Agent são
// saneados (sem caracteres de controle) e o User-Agent é truncado.
func montarCorpoAlertaReuso(nomeUsuario string, a AlertaReuso, paraAdmin bool) string {
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "saneando nome do usuário (valor não logado)")
	nome := removerControles(nomeUsuario)
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "verificando se o nome saneado ficou vazio")
	if nome == "" {
		vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "usando nome genérico")
		nome = "usuário"
	}
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "saneando IP de origem (valor não logado)")
	ip := removerControles(a.IP)
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "saneando e truncando User-Agent em %d runes (valor não logado)", maxUserAgentAlerta)
	ua := truncarRunes(removerControles(a.UserAgent), maxUserAgentAlerta)
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "formatando data/hora do reuso no fuso de Brasília")
	quando := a.Quando.In(tz.Local).Format("02/01/2006 15:04:05") + " (horário de Brasília)"

	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "declarando builder do corpo do alerta")
	var sb strings.Builder
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "verificando se o alerta é para admin (para_admin=%t)", paraAdmin)
	if paraAdmin {
		vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo cabeçalho do alerta para admin")
		sb.WriteString("Alerta de segurança: reuso de sessão já encerrada (possível roubo de token).\r\n\r\n")
	} else {
		vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo saudação ao usuário")
		sb.WriteString("Olá, " + nome + ".\r\n\r\n")
		vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo aviso de credencial reutilizada")
		sb.WriteString("Detectamos o uso de uma credencial de sessão já substituída na sua conta RotaPerfumes, o que pode indicar acesso indevido.\r\n\r\n")
	}
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo data/hora no corpo")
	sb.WriteString("Data/hora: " + quando + "\r\n")
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo IP de origem no corpo")
	sb.WriteString("IP de origem: " + ip + "\r\n")
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo navegador/dispositivo no corpo")
	sb.WriteString("Navegador/dispositivo: " + ua + "\r\n")
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "verificando se inclui identificadores internos (para_admin=%t)", paraAdmin)
	if paraAdmin {
		vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo identificação do usuário (usuario_id=%d)", a.UsuarioID)
		sb.WriteString(fmt.Sprintf("Usuário: %s (user_id=%d, e-mail=%s)\r\n", nome, a.UsuarioID, removerControles(a.EmailUsuario)))
		vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo token_id=%d", a.TokenID)
		sb.WriteString(fmt.Sprintf("token_id: %d\r\n", a.TokenID))
		vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo aviso de sessões encerradas (admin)")
		sb.WriteString("\r\nTodas as sessões do usuário foram encerradas automaticamente.\r\n")
		return sb.String()
	}
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo aviso de sessões encerradas (usuário)")
	sb.WriteString("\r\nPor segurança, encerramos todas as suas sessões; faça login novamente.\r\n")
	vlog.Printf("email_service.go", "montarCorpoAlertaReuso", "escrevendo orientação de troca de senha")
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
	vlog.Printf("email_service.go", "truncarRunes", "convertendo texto para runes (valor não logado)")
	r := []rune(s)
	vlog.Printf("email_service.go", "truncarRunes", "verificando se len=%d cabe no máximo %d", len(r), max)
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
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "validando destinatário %s (sem CR/LF, não vazio)", vlog.MaskEmail(to))
	if !destinatarioValido(to) {
		return ErrDestinatarioInvalido
	}
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "aparando espaços do destinatário")
	to = strings.TrimSpace(to)
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "montando endereço host:porta do servidor SMTP")
	addr := net.JoinHostPort(s.host, s.port)

	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "declarando dialer TCP")
	var d net.Dialer
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "abrindo conexão TCP com o servidor SMTP")
	conn, err := d.DialContext(ctx, "tcp", addr)
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "verificando se err != nil após DialContext")
	if err != nil {
		return fmt.Errorf("services: smtp dial: %w", err)
	}
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "agendando fechamento da conexão TCP")
	defer conn.Close()

	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "criando cliente SMTP sobre a conexão")
	client, err := smtp.NewClient(conn, s.host)
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "verificando se err != nil após smtp.NewClient")
	if err != nil {
		return fmt.Errorf("services: smtp client: %w", err)
	}
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "agendando fechamento do cliente SMTP")
	defer client.Close()

	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "verificando se o servidor suporta STARTTLS")
	if ok, _ := client.Extension("STARTTLS"); ok {
		vlog.Printf("email_service.go", "SMTPEmailService.enviar", "iniciando STARTTLS e verificando erro")
		if err := client.StartTLS(s.tlsConfig); err != nil {
			return fmt.Errorf("services: smtp starttls: %w", err)
		}
	} else {
		return fmt.Errorf("services: smtp starttls: not supported by server")
	}

	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "preparando autenticação PLAIN (credenciais não logadas)")
	auth := smtp.PlainAuth("", s.user, s.pass, s.host)
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "verificando se o servidor suporta AUTH")
	if ok, _ := client.Extension("AUTH"); ok {
		vlog.Printf("email_service.go", "SMTPEmailService.enviar", "autenticando no servidor SMTP e verificando erro")
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("services: smtp auth: %w", err)
		}
	}

	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "enviando MAIL FROM e verificando erro")
	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("services: smtp mail from: %w", err)
	}
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "enviando RCPT TO para %s e verificando erro", vlog.MaskEmail(to))
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("services: smtp rcpt to: %w", err)
	}

	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "abrindo writer DATA")
	w, err := client.Data()
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "verificando se err != nil após client.Data")
	if err != nil {
		return fmt.Errorf("services: smtp data: %w", err)
	}

	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "montando mensagem RFC 5322 (conteúdo não logado)")
	msg := buildMessage(s.from, to, subject, body)
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "escrevendo %d bytes da mensagem e verificando erro", len(msg))
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("services: smtp write: %w", err)
	}
	vlog.Printf("email_service.go", "SMTPEmailService.enviar", "fechando writer DATA e verificando erro")
	if err := w.Close(); err != nil {
		return fmt.Errorf("services: smtp close: %w", err)
	}

	return client.Quit()
}

// buildMessage monta o payload RFC 5322 mínimo (headers + corpo em texto puro).
func buildMessage(from, to, subject, body string) string {
	vlog.Printf("email_service.go", "buildMessage", "declarando builder da mensagem")
	var sb strings.Builder
	vlog.Printf("email_service.go", "buildMessage", "escrevendo cabeçalho From")
	sb.WriteString("From: " + from + "\r\n")
	vlog.Printf("email_service.go", "buildMessage", "escrevendo cabeçalho To para %s", vlog.MaskEmail(to))
	sb.WriteString("To: " + to + "\r\n")
	vlog.Printf("email_service.go", "buildMessage", "escrevendo cabeçalho Subject")
	sb.WriteString("Subject: " + subject + "\r\n")
	vlog.Printf("email_service.go", "buildMessage", "escrevendo cabeçalho MIME-Version")
	sb.WriteString("MIME-Version: 1.0\r\n")
	vlog.Printf("email_service.go", "buildMessage", "escrevendo cabeçalho Content-Type")
	sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	vlog.Printf("email_service.go", "buildMessage", "escrevendo linha em branco separadora")
	sb.WriteString("\r\n")
	vlog.Printf("email_service.go", "buildMessage", "escrevendo corpo (%d bytes, conteúdo não logado)", len(body))
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
	log.Printf("[email] envio pulado: SMTP não configurado — destinatario=%s (senha gerada, ver auditoria de senha_historico)", vlog.MaskEmail(destinatario))
	return nil
}

// EnviarAlertaReusoToken não envia e-mail; registra que o alerta foi pulado
// (sem e-mail do usuário no log) e devolve ErrSMTPNaoConfigurado, para que o
// chamador não registre o alerta como "enviado" (LOG-01).
func (n *NoopEmailService) EnviarAlertaReusoToken(ctx context.Context, destinatario, nomeUsuario string, a AlertaReuso, paraAdmin bool) error {
	log.Printf("[email] alerta de seguranca pulado: SMTP não configurado user_id=%d para_admin=%v", a.UsuarioID, paraAdmin)
	return ErrSMTPNaoConfigurado
}
