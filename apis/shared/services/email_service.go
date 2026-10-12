package services

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"os"
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
// Implementação SMTP (TLS implícito na 465 ou STARTTLS na 587/25)
// ---------------------------------------------------------------------------

// SMTPEmailService envia emails via SMTP sempre cifrado: TLS implícito
// (SMTPS) ou STARTTLS, conforme SMTP_TLS_MODE ou a porta. A autenticação é
// obrigatória e só acontece sobre TLS (ver email_smtp.go).
type SMTPEmailService struct {
	host string
	port string
	user string
	pass string
	from string
	// modoTLS é SMTPModoTLSImplicito ou SMTPModoSTARTTLS.
	modoTLS string
	// serverName é o nome verificado no certificado e usado no AUTH
	// (SMTP_TLS_SERVER_NAME ou, se vazio, o host).
	serverName string
	// tlsConfig é usado no handshake (implícito ou STARTTLS). A validação do
	// certificado nunca é desligada por aqui.
	tlsConfig *tls.Config
}

// ModoTLS devolve o modo TLS efetivo (SMTPModoTLSImplicito ou SMTPModoSTARTTLS).
func (s *SMTPEmailService) ModoTLS() string { return s.modoTLS }

// ServerNameTLS devolve o nome verificado no certificado do servidor SMTP.
func (s *SMTPEmailService) ServerNameTLS() string { return s.serverName }

// NewSMTPEmailService cria um EmailService real a partir da configuração.
// Retorna erro se host/user/password/from não estiverem preenchidos —
// use NewNoopEmailService como fallback quando SMTP não estiver configurado.
func NewSMTPEmailService(cfg *config.Config) (*SMTPEmailService, error) {
	return NewSMTPEmailServiceWithTLSConfig(cfg, nil)
}

// NewSMTPEmailServiceWithTLSConfig é NewSMTPEmailService com a configuração
// TLS injetável (ex.: RootCAs de uma CA própria, ou a CA de um servidor SMTP
// de teste). tlsCfg nil usa as raízes do sistema. A config é clonada e, sobre
// o clone, o construtor impõe (SEC-10): InsecureSkipVerify sempre false (o
// certificado do servidor é sempre verificado, mesmo que o chamador tenha
// desligado), MinVersion mínima TLS 1.2 (só sobe; TLS 1.3 é mantido) e
// ServerName = SMTP_TLS_SERVER_NAME ou, se vazio, SMTP_HOST (MAIL-01).
// SMTP_TLS_CA_FILE, se definido, acrescenta CAs confiáveis às raízes.
//
// Erros além de ErrSMTPNaoConfigurado (modo TLS inválido, CA ilegível) são
// de configuração e devem impedir a subida, não cair no noop.
func NewSMTPEmailServiceWithTLSConfig(cfg *config.Config, tlsCfg *tls.Config) (*SMTPEmailService, error) {
	vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "verificando se host/usuário/senha/remetente SMTP estão preenchidos (valores não logados)")
	if cfg.SMTPHost == "" || cfg.SMTPUser == "" || cfg.SMTPPassword == "" || cfg.SMTPFrom == "" {
		return nil, ErrSMTPNaoConfigurado
	}
	vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "resolvendo modo TLS (SMTP_TLS_MODE=%q, porta=%s)", cfg.SMTPTLSMode, cfg.SMTPPort)
	modo, err := ResolverModoTLSSMTP(cfg.SMTPTLSMode, cfg.SMTPPort)
	if err != nil {
		return nil, err
	}
	serverName := ResolverServerNameTLSSMTP(cfg.SMTPHost, cfg.SMTPTLSServerName)
	vlog.Printf("email_service.go", "NewSMTPEmailServiceWithTLSConfig", "modo TLS=%s, ServerName=%q", modo, serverName)
	tlsCfg, err = montarConfigTLSSMTP(tlsCfg, serverName, cfg.SMTPTLSCAFile)
	if err != nil {
		return nil, err
	}
	return &SMTPEmailService{
		host:       cfg.SMTPHost,
		port:       cfg.SMTPPort,
		user:       cfg.SMTPUser,
		pass:       cfg.SMTPPassword,
		from:       cfg.SMTPFrom,
		modoTLS:    modo,
		serverName: serverName,
		tlsConfig:  tlsCfg,
	}, nil
}

// montarConfigTLSSMTP clona (ou cria) a config TLS e aplica as regras de
// segurança do SEC-10 e do MAIL-01.
func montarConfigTLSSMTP(base *tls.Config, serverName, caFile string) (*tls.Config, error) {
	vlog.Printf("email_service.go", "montarConfigTLSSMTP", "verificando se foi injetada config TLS (injetada=%t)", base != nil)
	tlsCfg := &tls.Config{}
	if base != nil {
		vlog.Printf("email_service.go", "montarConfigTLSSMTP", "clonando config TLS injetada")
		tlsCfg = base.Clone()
	}
	vlog.Printf("email_service.go", "montarConfigTLSSMTP", "forçando verificação de certificado (InsecureSkipVerify=false)")
	tlsCfg.InsecureSkipVerify = false
	if tlsCfg.MinVersion < tls.VersionTLS12 {
		vlog.Printf("email_service.go", "montarConfigTLSSMTP", "elevando MinVersion para TLS 1.2")
		tlsCfg.MinVersion = tls.VersionTLS12
	}
	tlsCfg.ServerName = serverName
	if caFile == "" {
		return tlsCfg, nil
	}
	vlog.Printf("email_service.go", "montarConfigTLSSMTP", "carregando CAs extras de SMTP_TLS_CA_FILE")
	pool, err := carregarCAsSMTP(caFile, tlsCfg.RootCAs)
	if err != nil {
		return nil, err
	}
	tlsCfg.RootCAs = pool
	return tlsCfg, nil
}

// carregarCAsSMTP acrescenta os certificados PEM de caminho a uma cópia de
// base (ou às raízes do sistema, se base for nil).
func carregarCAsSMTP(caminho string, base *x509.CertPool) (*x509.CertPool, error) {
	pemBytes, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("services: SMTP_TLS_CA_FILE: %w", err)
	}
	var pool *x509.CertPool
	if base != nil {
		pool = base.Clone()
	} else if pool, err = x509.SystemCertPool(); err != nil {
		vlog.Printf("email_service.go", "carregarCAsSMTP", "raízes do sistema indisponíveis (%v); usando só o arquivo", err)
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("services: SMTP_TLS_CA_FILE %q: nenhum certificado PEM válido", caminho)
	}
	return pool, nil
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
