package notificacao

import (
	"fmt"
	"strings"
)

// ModoTLS é como a conexão SMTP é cifrada (MAIL-01, M-01). Não existe modo
// sem TLS: a escolha da Strategy de conexão (notificacaosmtp) é só entre os
// dois valores abaixo.
type ModoTLS string

const (
	// ModoTLSImplicito: TLS desde o connect (SMTPS, RFC 8314, porta 465).
	ModoTLSImplicito ModoTLS = "implicit"
	// ModoSTARTTLS: conexão em texto puro promovida com STARTTLS (587/25);
	// aborta se o servidor não anunciar STARTTLS.
	ModoSTARTTLS ModoTLS = "starttls"
)

// portaSMTPS é a porta padrão do SMTP com TLS implícito.
const portaSMTPS = "465"

// ResolverModoTLS devolve o modo efetivo de SMTP_TLS_MODE. Vazio (ou "auto")
// infere pela porta: 465 → implicit; qualquer outra → starttls.
func ResolverModoTLS(modo, porta string) (ModoTLS, error) {
	switch strings.ToLower(strings.TrimSpace(modo)) {
	case string(ModoTLSImplicito):
		return ModoTLSImplicito, nil
	case string(ModoSTARTTLS):
		return ModoSTARTTLS, nil
	case "", "auto":
		if strings.TrimSpace(porta) == portaSMTPS {
			return ModoTLSImplicito, nil
		}
		return ModoSTARTTLS, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrModoTLSInvalido, modo)
	}
}

// String devolve o valor de SMTP_TLS_MODE ("implicit" ou "starttls").
func (m ModoTLS) String() string { return string(m) }
