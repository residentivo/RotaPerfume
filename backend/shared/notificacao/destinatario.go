package notificacao

import "strings"

// Destinatario é um endereço de e-mail pronto para o RCPT TO e o cabeçalho
// To: não vazio, sem CR/LF e aparado.
type Destinatario string

// NovoDestinatario valida o valor CRU (TrimSpace esconderia um "\r\n" final)
// e devolve o valor aparado: o mesmo vai para o RCPT TO e para o To:.
func NovoDestinatario(bruto string) (Destinatario, error) {
	if !DestinatarioValido(bruto) {
		return "", ErrDestinatarioInvalido
	}
	return Destinatario(strings.TrimSpace(bruto)), nil
}

// DestinatarioValido recusa destinatários vazios ou com CR/LF: o cabeçalho
// To: é escrito sem escape.
func DestinatarioValido(bruto string) bool {
	return strings.TrimSpace(bruto) != "" && !strings.ContainsAny(bruto, "\r\n")
}

// String devolve o endereço.
func (d Destinatario) String() string { return string(d) }
