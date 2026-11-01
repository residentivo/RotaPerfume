package notificacaosmtp

// Strategy de conexão TLS (§5.6): tlsImplicito (porta 465, TLS desde o
// connect) e startTLS (texto puro promovido; aborta se não anunciado). Não
// existe estratégia sem TLS (M-01).

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"

	"github.com/rotaperfumes/shared/notificacao"
)

// estrategiaTLS faz o handshake sobre a conexão TCP já aberta e devolve o
// cliente SMTP sob TLS.
type estrategiaTLS interface {
	abrir(ctx context.Context, conn net.Conn, serverName string, cfg *tls.Config) (*smtp.Client, error)
}

// estrategiaDoModo escolhe a Strategy pelo modo já resolvido no construtor
// (ResolverModoTLS só devolve os dois modos).
func estrategiaDoModo(m notificacao.ModoTLS) estrategiaTLS {
	if m == notificacao.ModoTLSImplicito {
		return tlsImplicito{}
	}
	return startTLS{}
}

// tlsImplicito: handshake TLS logo após o connect (SMTPS).
type tlsImplicito struct{}

func (tlsImplicito) abrir(ctx context.Context, conn net.Conn, serverName string, cfg *tls.Config) (*smtp.Client, error) {
	tlsConn := tls.Client(conn, cfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return nil, fmt.Errorf("notificacaosmtp: smtp tls implícito: %w", err)
	}
	client, err := smtp.NewClient(tlsConn, serverName)
	if err != nil {
		return nil, fmt.Errorf("notificacaosmtp: smtp client: %w", err)
	}
	return client, nil
}

// startTLS: lê o banner em texto puro, exige STARTTLS anunciado e promove.
type startTLS struct{}

func (startTLS) abrir(_ context.Context, conn net.Conn, serverName string, cfg *tls.Config) (*smtp.Client, error) {
	client, err := smtp.NewClient(conn, serverName)
	if err != nil {
		return nil, fmt.Errorf("notificacaosmtp: smtp client: %w", err)
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		_ = client.Close()
		return nil, errors.New("notificacaosmtp: smtp starttls: not supported by server")
	}
	if err := client.StartTLS(cfg); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("notificacaosmtp: smtp starttls: %w", err)
	}
	return client, nil
}
