// Package notificacaosmtp é a infraestrutura do contexto Notificação
// (ARQ-07): o Adapter SMTP que satisfaz as portas de envio
// (identidadeapp.EnviadorSenhaInicial e notificacaoapp.EnviadorAlerta), com
// a conexão escolhida por Strategy (TLS implícito ou STARTTLS, MAIL-01), e o
// Null Object Noop para quando o SMTP não está configurado. Credenciais e
// conteúdo da mensagem nunca são logados (M-01, M-02).
package notificacaosmtp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/notificacao"
	"github.com/rotaperfumes/shared/vlog"
)

const arqSMTP = "smtp.go"

// SMTP envia e-mails sempre cifrados (TLS implícito ou STARTTLS), com AUTH
// obrigatório e só sobre TLS. É singleton por processo (bootstrap).
type SMTP struct {
	host       string
	porta      string
	usuario    string
	senha      string
	remetente  string
	modo       notificacao.ModoTLS
	conexao    estrategiaTLS
	serverName string
	// tlsConfig é usado no handshake; a verificação do certificado nunca é
	// desligada por aqui (SEC-10).
	tlsConfig *tls.Config
}

// Novo cria o SMTP a partir da configuração. Sem host/usuário/senha/
// remetente devolve notificacao.ErrSMTPNaoConfigurado (o bootstrap cai no
// Noop); qualquer outro erro é de configuração e deve impedir a subida.
func Novo(cfg *config.Config) (*SMTP, error) {
	return NovoComTLS(cfg, nil)
}

// NovoComTLS é Novo com a configuração TLS injetável (CA própria, ou a CA de
// um servidor de teste); nil usa as raízes do sistema. Sobre um clone, impõe
// (SEC-10, MAIL-01): InsecureSkipVerify sempre false, MinVersion ≥ TLS 1.2
// (só sobe) e ServerName = SMTP_TLS_SERVER_NAME ou SMTP_HOST.
// SMTP_TLS_CA_FILE, se definido, soma CAs às raízes.
func NovoComTLS(cfg *config.Config, tlsCfg *tls.Config) (*SMTP, error) {
	if cfg.SMTPHost == "" || cfg.SMTPUser == "" || cfg.SMTPPassword == "" || cfg.SMTPFrom == "" {
		vlog.Printf(arqSMTP, "NovoComTLS", "decisão smtp_configurado=false")
		return nil, notificacao.ErrSMTPNaoConfigurado
	}
	modo, err := notificacao.ResolverModoTLS(cfg.SMTPTLSMode, cfg.SMTPPort)
	if err != nil {
		return nil, err
	}
	serverName := ResolverServerName(cfg.SMTPHost, cfg.SMTPTLSServerName)
	tlsCfg, err = montarConfigTLS(tlsCfg, serverName, cfg.SMTPTLSCAFile)
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqSMTP, "NovoComTLS", "decisão smtp_configurado=true modo_tls=%s", modo)
	return &SMTP{
		host:       cfg.SMTPHost,
		porta:      cfg.SMTPPort,
		usuario:    cfg.SMTPUser,
		senha:      cfg.SMTPPassword,
		remetente:  cfg.SMTPFrom,
		modo:       modo,
		conexao:    estrategiaDoModo(modo),
		serverName: serverName,
		tlsConfig:  tlsCfg,
	}, nil
}

// ResolverServerName devolve o nome verificado no certificado e usado no
// AUTH: serverName (SMTP_TLS_SERVER_NAME) se preenchido, senão o host.
func ResolverServerName(host, serverName string) string {
	if nome := strings.TrimSpace(serverName); nome != "" {
		return nome
	}
	return host
}

// ModoTLS devolve o modo TLS efetivo.
func (s *SMTP) ModoTLS() notificacao.ModoTLS { return s.modo }

// ServerNameTLS devolve o nome verificado no certificado do servidor.
func (s *SMTP) ServerNameTLS() string { return s.serverName }

// EnviarSenhaInicial envia a senha gerada ao e-mail do usuário (porta
// identidadeapp.EnviadorSenhaInicial). Nunca loga a senha (M-02).
func (s *SMTP) EnviarSenhaInicial(ctx context.Context, destinatario, nomeUsuario, senha string) error {
	return s.enviar(ctx, destinatario, notificacao.MensagemSenhaInicial(nomeUsuario, senha))
}

// EnviarAlertaReusoToken envia o alerta de reuso de refresh token (porta
// notificacaoapp.EnviadorAlerta, SEC-09). Ao usuário, sem user_id/token_id.
func (s *SMTP) EnviarAlertaReusoToken(ctx context.Context, destinatario, nomeUsuario string, a notificacao.AlertaReuso, paraAdmin bool) error {
	if !notificacao.DestinatarioValido(destinatario) {
		return notificacao.ErrDestinatarioInvalido
	}
	return s.enviar(ctx, destinatario, notificacao.MensagemAlertaReuso(nomeUsuario, a, paraAdmin))
}

// montarConfigTLS clona (ou cria) a config TLS e aplica as regras do SEC-10 e
// do MAIL-01.
func montarConfigTLS(base *tls.Config, serverName, caFile string) (*tls.Config, error) {
	tlsCfg := &tls.Config{}
	if base != nil {
		tlsCfg = base.Clone()
	}
	tlsCfg.InsecureSkipVerify = false
	if tlsCfg.MinVersion < tls.VersionTLS12 {
		tlsCfg.MinVersion = tls.VersionTLS12
	}
	tlsCfg.ServerName = serverName
	if caFile == "" {
		return tlsCfg, nil
	}
	pool, err := carregarCAs(caFile, tlsCfg.RootCAs)
	if err != nil {
		return nil, err
	}
	tlsCfg.RootCAs = pool
	return tlsCfg, nil
}

// carregarCAs acrescenta os certificados PEM do arquivo a uma cópia de base
// (ou às raízes do sistema, se base for nil).
func carregarCAs(caminho string, base *x509.CertPool) (*x509.CertPool, error) {
	pemBytes, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("notificacaosmtp: SMTP_TLS_CA_FILE: %w", err)
	}
	var pool *x509.CertPool
	if base != nil {
		pool = base.Clone()
	} else if pool, err = x509.SystemCertPool(); err != nil {
		vlog.Printf(arqSMTP, "carregarCAs", "decisão raizes_do_sistema=indisponiveis (usando só o arquivo)")
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("notificacaosmtp: SMTP_TLS_CA_FILE %q: nenhum certificado PEM válido", caminho)
	}
	return pool, nil
}
