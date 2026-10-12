package services

// Transporte SMTP do SMTPEmailService (MAIL-01): conexão com TLS implícito
// (SMTPS, porta 465) ou STARTTLS (587/25), AUTH obrigatório e só sobre TLS,
// e prazos de rede. Credenciais e conteúdo da mensagem nunca são logados.

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/vlog"
)

// Modos TLS aceitos em SMTP_TLS_MODE.
const (
	// SMTPModoTLSImplicito: TLS desde o connect (SMTPS, RFC 8314).
	SMTPModoTLSImplicito = "implicit"
	// SMTPModoSTARTTLS: conexão em texto puro promovida com STARTTLS.
	SMTPModoSTARTTLS = "starttls"
)

// portaSMTPS é a porta padrão do SMTP com TLS implícito.
const portaSMTPS = "465"

// Prazos de rede do envio.
const (
	smtpTimeoutConexao = 10 * time.Second // dial TCP
	smtpTimeoutSessao  = 30 * time.Second // da conexão ao QUIT
)

var (
	// ErrSMTPModoTLSInvalido indica SMTP_TLS_MODE fora de implicit|starttls.
	ErrSMTPModoTLSInvalido = errors.New("services: SMTP_TLS_MODE inválido (use implicit ou starttls)")
	// ErrSMTPSemAuth: o servidor não anunciou AUTH; o envio é abortado em vez
	// de seguir sem autenticar.
	ErrSMTPSemAuth = errors.New("servidor não anunciou AUTH (envio abortado, credenciais não enviadas)")
	// ErrSMTPAuthSemTLS: proteção para nunca enviar credenciais em texto puro.
	ErrSMTPAuthSemTLS = errors.New("conexão sem TLS (credenciais não enviadas)")
	// ErrSMTPMecanismoAuth: o servidor não oferece PLAIN nem LOGIN.
	ErrSMTPMecanismoAuth = errors.New("servidor não oferece AUTH PLAIN nem LOGIN")
)

// ResolverModoTLSSMTP devolve o modo TLS efetivo. modo vazio (ou "auto")
// infere pela porta: 465 → implicit; qualquer outra → starttls.
func ResolverModoTLSSMTP(modo, porta string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(modo)) {
	case SMTPModoTLSImplicito:
		return SMTPModoTLSImplicito, nil
	case SMTPModoSTARTTLS:
		return SMTPModoSTARTTLS, nil
	case "", "auto":
		if strings.TrimSpace(porta) == portaSMTPS {
			return SMTPModoTLSImplicito, nil
		}
		return SMTPModoSTARTTLS, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrSMTPModoTLSInvalido, modo)
	}
}

// ResolverServerNameTLSSMTP devolve o nome verificado no certificado:
// serverName (SMTP_TLS_SERVER_NAME) se preenchido, senão o host.
func ResolverServerNameTLSSMTP(host, serverName string) string {
	if nome := strings.TrimSpace(serverName); nome != "" {
		return nome
	}
	return host
}

// enviar entrega uma mensagem: conecta (TLS implícito ou STARTTLS),
// autentica, transmite e encerra com QUIT.
func (s *SMTPEmailService) enviar(ctx context.Context, to, subject, body string) error {
	// CR/LF é checado no valor cru (TrimSpace esconderia um "\r\n" final);
	// depois apara — o mesmo valor aparado vai para RCPT TO e para To:.
	vlog.Printf("email_smtp.go", "SMTPEmailService.enviar", "validando destinatário %s (sem CR/LF, não vazio)", vlog.MaskEmail(to))
	if !destinatarioValido(to) {
		return ErrDestinatarioInvalido
	}
	to = strings.TrimSpace(to)

	client, pararVigia, err := s.conectar(ctx)
	if err != nil {
		return err
	}
	defer pararVigia()
	defer client.Close()

	if err := s.autenticar(client); err != nil {
		return err
	}
	if err := s.transmitir(client, to, subject, body); err != nil {
		return err
	}
	vlog.Printf("email_smtp.go", "SMTPEmailService.enviar", "enviando QUIT")
	return client.Quit()
}

// conectar abre a conexão, aplica prazos e devolve o cliente SMTP já sob TLS
// e a função que desliga o vigia de cancelamento do ctx (chamar ao terminar).
func (s *SMTPEmailService) conectar(ctx context.Context) (*smtp.Client, func() bool, error) {
	addr := net.JoinHostPort(s.host, s.port)
	vlog.Printf("email_smtp.go", "SMTPEmailService.conectar", "abrindo TCP com %s (modo TLS=%s, timeout %s)", addr, s.modoTLS, smtpTimeoutConexao)
	d := net.Dialer{Timeout: smtpTimeoutConexao}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("services: smtp dial: %w", err)
	}
	vlog.Printf("email_smtp.go", "SMTPEmailService.conectar", "definindo prazo da sessão SMTP")
	_ = conn.SetDeadline(prazoSessaoSMTP(ctx))
	// Cancelar o ctx no meio da conversa fecha a conexão (destrava leituras).
	pararVigia := context.AfterFunc(ctx, func() { _ = conn.Close() })

	client, err := s.abrirCliente(ctx, conn)
	if err != nil {
		pararVigia()
		_ = conn.Close()
		return nil, nil, err
	}
	return client, pararVigia, nil
}

// abrirCliente faz o handshake conforme o modo TLS e cria o cliente SMTP.
func (s *SMTPEmailService) abrirCliente(ctx context.Context, conn net.Conn) (*smtp.Client, error) {
	if s.modoTLS == SMTPModoTLSImplicito {
		vlog.Printf("email_smtp.go", "SMTPEmailService.abrirCliente", "handshake TLS implícito (ServerName=%q)", s.serverName)
		tlsConn := tls.Client(conn, s.tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return nil, fmt.Errorf("services: smtp tls implícito: %w", err)
		}
		conn = tlsConn
	}
	vlog.Printf("email_smtp.go", "SMTPEmailService.abrirCliente", "lendo banner e criando cliente SMTP")
	client, err := smtp.NewClient(conn, s.serverName)
	if err != nil {
		return nil, fmt.Errorf("services: smtp client: %w", err)
	}
	if s.modoTLS == SMTPModoSTARTTLS {
		if err := s.iniciarSTARTTLS(client); err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	return client, nil
}

// iniciarSTARTTLS exige que o servidor anuncie STARTTLS e promove a conexão.
func (s *SMTPEmailService) iniciarSTARTTLS(client *smtp.Client) error {
	vlog.Printf("email_smtp.go", "SMTPEmailService.iniciarSTARTTLS", "verificando se o servidor anuncia STARTTLS")
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return errors.New("services: smtp starttls: not supported by server")
	}
	vlog.Printf("email_smtp.go", "SMTPEmailService.iniciarSTARTTLS", "iniciando STARTTLS (ServerName=%q)", s.serverName)
	if err := client.StartTLS(s.tlsConfig); err != nil {
		return fmt.Errorf("services: smtp starttls: %w", err)
	}
	return nil
}

// autenticar exige TLS ativo e AUTH anunciado; nunca pula a autenticação.
func (s *SMTPEmailService) autenticar(client *smtp.Client) error {
	vlog.Printf("email_smtp.go", "SMTPEmailService.autenticar", "verificando se a conexão está sob TLS")
	if _, ok := client.TLSConnectionState(); !ok {
		return fmt.Errorf("services: smtp auth: %w", ErrSMTPAuthSemTLS)
	}
	ok, mecanismos := client.Extension("AUTH")
	vlog.Printf("email_smtp.go", "SMTPEmailService.autenticar", "servidor anunciou AUTH=%t mecanismos=%q", ok, mecanismos)
	if !ok {
		return fmt.Errorf("services: smtp auth: %w", ErrSMTPSemAuth)
	}
	auth, nome, err := escolherAuthSMTP(mecanismos, s.user, s.pass, s.serverName)
	if err != nil {
		return fmt.Errorf("services: smtp auth: %w", err)
	}
	vlog.Printf("email_smtp.go", "SMTPEmailService.autenticar", "autenticando com AUTH %s (credenciais não logadas)", nome)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("services: smtp auth: %w", err)
	}
	return nil
}

// transmitir envia MAIL FROM, RCPT TO e DATA.
func (s *SMTPEmailService) transmitir(client *smtp.Client, to, subject, body string) error {
	vlog.Printf("email_smtp.go", "SMTPEmailService.transmitir", "enviando MAIL FROM")
	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("services: smtp mail from: %w", err)
	}
	vlog.Printf("email_smtp.go", "SMTPEmailService.transmitir", "enviando RCPT TO para %s", vlog.MaskEmail(to))
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("services: smtp rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("services: smtp data: %w", err)
	}
	msg := buildMessage(s.from, to, subject, body)
	vlog.Printf("email_smtp.go", "SMTPEmailService.transmitir", "escrevendo %d bytes da mensagem (conteúdo não logado)", len(msg))
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("services: smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("services: smtp close: %w", err)
	}
	return nil
}

// prazoSessaoSMTP devolve o menor entre o prazo do ctx e agora+smtpTimeoutSessao.
func prazoSessaoSMTP(ctx context.Context) time.Time {
	prazo := time.Now().Add(smtpTimeoutSessao)
	if d, ok := ctx.Deadline(); ok && d.Before(prazo) {
		return d
	}
	return prazo
}

// escolherAuthSMTP prefere PLAIN e cai para LOGIN se for o único oferecido.
func escolherAuthSMTP(mecanismos, usuario, senha, host string) (smtp.Auth, string, error) {
	oferecidos := strings.Fields(strings.ToUpper(mecanismos))
	switch {
	case contem(oferecidos, "PLAIN"):
		// smtp.PlainAuth recusa por conta própria conexão sem TLS e host divergente.
		return smtp.PlainAuth("", usuario, senha, host), "PLAIN", nil
	case contem(oferecidos, "LOGIN"):
		return &loginAuthSMTP{usuario: usuario, senha: senha, host: host}, "LOGIN", nil
	default:
		return nil, "", fmt.Errorf("%w (anunciados: %q)", ErrSMTPMecanismoAuth, mecanismos)
	}
}

func contem(lista []string, alvo string) bool {
	for _, item := range lista {
		if item == alvo {
			return true
		}
	}
	return false
}

// loginAuthSMTP implementa AUTH LOGIN (não existe no net/smtp), com as mesmas
// proteções do PlainAuth: só sobre TLS e só para o host esperado.
type loginAuthSMTP struct {
	usuario, senha, host string
}

func (a *loginAuthSMTP) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS {
		return "", nil, ErrSMTPAuthSemTLS
	}
	if server.Name != a.host {
		return "", nil, errors.New("wrong host name")
	}
	return "LOGIN", nil, nil
}

func (a *loginAuthSMTP) Next(desafio []byte, mais bool) ([]byte, error) {
	if !mais {
		return nil, nil
	}
	pergunta := strings.ToLower(string(desafio))
	switch {
	case strings.Contains(pergunta, "user"):
		return []byte(a.usuario), nil
	case strings.Contains(pergunta, "pass"):
		return []byte(a.senha), nil
	default:
		return nil, fmt.Errorf("desafio AUTH LOGIN inesperado: %q", desafio)
	}
}
