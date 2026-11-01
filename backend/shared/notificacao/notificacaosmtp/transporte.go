package notificacaosmtp

// Transporte (MAIL-01): conexão pela Strategy de TLS, AUTH obrigatório e só
// sobre TLS, MAIL FROM/RCPT TO/DATA e QUIT, com prazos de rede. Credenciais
// e conteúdo da mensagem nunca são logados.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/notificacao"
	"github.com/rotaperfumes/shared/vlog"
)

const arqTransporte = "transporte.go"

// Prazos de rede do envio.
const (
	timeoutConexao = 10 * time.Second // dial TCP
	timeoutSessao  = 30 * time.Second // da conexão ao QUIT
)

var (
	// ErrSemAuth: o servidor não anunciou AUTH; o envio é abortado em vez de
	// seguir sem autenticar.
	ErrSemAuth = errors.New("servidor não anunciou AUTH (envio abortado, credenciais não enviadas)")
	// ErrAuthSemTLS: proteção para nunca enviar credenciais em texto puro.
	ErrAuthSemTLS = errors.New("conexão sem TLS (credenciais não enviadas)")
	// ErrMecanismoAuth: o servidor não oferece PLAIN nem LOGIN.
	ErrMecanismoAuth = errors.New("servidor não oferece AUTH PLAIN nem LOGIN")
)

// enviar entrega uma mensagem: valida o destinatário, conecta, autentica,
// transmite e encerra com QUIT.
func (s *SMTP) enviar(ctx context.Context, bruto string, m notificacao.Mensagem) (err error) {
	defer vlog.Trace(arqTransporte, "SMTP.enviar", "para=%s modo_tls=%s", vlog.MaskEmail(bruto), s.modo)(&err)
	destino, err := notificacao.NovoDestinatario(bruto)
	if err != nil {
		return err
	}
	client, pararVigia, err := s.conectar(ctx)
	if err != nil {
		return err
	}
	defer pararVigia()
	defer client.Close()

	if err := s.autenticar(client); err != nil {
		return err
	}
	if err := s.transmitir(client, destino, m); err != nil {
		return err
	}
	return client.Quit()
}

// conectar abre o TCP, aplica o prazo da sessão e entrega a conexão à
// Strategy de TLS. Devolve também a função que desliga o vigia de
// cancelamento do ctx (chamar ao terminar).
func (s *SMTP) conectar(ctx context.Context) (*smtp.Client, func() bool, error) {
	d := net.Dialer{Timeout: timeoutConexao}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(s.host, s.porta))
	if err != nil {
		return nil, nil, fmt.Errorf("notificacaosmtp: smtp dial: %w", err)
	}
	_ = conn.SetDeadline(prazoSessao(ctx))
	// Cancelar o ctx no meio da conversa fecha a conexão (destrava leituras).
	pararVigia := context.AfterFunc(ctx, func() { _ = conn.Close() })

	client, err := s.conexao.abrir(ctx, conn, s.serverName, s.tlsConfig)
	if err != nil {
		pararVigia()
		_ = conn.Close()
		return nil, nil, err
	}
	return client, pararVigia, nil
}

// autenticar exige TLS ativo e AUTH anunciado; nunca pula a autenticação.
func (s *SMTP) autenticar(client *smtp.Client) error {
	if _, ok := client.TLSConnectionState(); !ok {
		return fmt.Errorf("notificacaosmtp: smtp auth: %w", ErrAuthSemTLS)
	}
	ok, mecanismos := client.Extension("AUTH")
	if !ok {
		return fmt.Errorf("notificacaosmtp: smtp auth: %w", ErrSemAuth)
	}
	auth, nome, err := escolherAuth(mecanismos, s.usuario, s.senha, s.serverName)
	if err != nil {
		return fmt.Errorf("notificacaosmtp: smtp auth: %w", err)
	}
	vlog.Printf(arqTransporte, "SMTP.autenticar", "decisão auth=%s (credenciais não logadas)", nome)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("notificacaosmtp: smtp auth: %w", err)
	}
	return nil
}

// transmitir envia MAIL FROM, RCPT TO e DATA.
func (s *SMTP) transmitir(client *smtp.Client, destino notificacao.Destinatario, m notificacao.Mensagem) error {
	if err := client.Mail(s.remetente); err != nil {
		return fmt.Errorf("notificacaosmtp: smtp mail from: %w", err)
	}
	if err := client.Rcpt(destino.String()); err != nil {
		return fmt.Errorf("notificacaosmtp: smtp rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("notificacaosmtp: smtp data: %w", err)
	}
	if _, err := w.Write([]byte(montarPayload(s.remetente, destino, m))); err != nil {
		return fmt.Errorf("notificacaosmtp: smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("notificacaosmtp: smtp close: %w", err)
	}
	return nil
}

// montarPayload monta o payload RFC 5322 mínimo (cabeçalhos + texto puro).
func montarPayload(de string, para notificacao.Destinatario, m notificacao.Mensagem) string {
	var sb strings.Builder
	sb.WriteString("From: " + de + "\r\n")
	sb.WriteString("To: " + para.String() + "\r\n")
	sb.WriteString("Subject: " + m.Assunto + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(m.Corpo)
	return sb.String()
}

// prazoSessao devolve o menor entre o prazo do ctx e agora+timeoutSessao.
func prazoSessao(ctx context.Context) time.Time {
	prazo := time.Now().Add(timeoutSessao)
	if d, ok := ctx.Deadline(); ok && d.Before(prazo) {
		return d
	}
	return prazo
}
