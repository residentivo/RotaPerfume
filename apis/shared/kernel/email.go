package kernel

import "strings"

// Email é um endereço normalizado (trim + minúsculas), com formato básico
// validado e sem CR/LF (M-01: nunca vira cabeçalho SMTP injetado).
type Email struct {
	valor string
}

// ErrEmailInvalido é a mensagem pública de e-mail inválido (a mesma do
// legado, usuario_service.go).
var ErrEmailInvalido = NovoErro(KindInvalido, "email inválido")

// NovoEmail valida e normaliza. O CR/LF é recusado ANTES do trim, para que
// "a@b.com\r\n" não passe por normalização.
func NovoEmail(s string) (Email, error) {
	if strings.ContainsAny(s, "\r\n") {
		return Email{}, ErrEmailInvalido
	}
	v := strings.ToLower(strings.TrimSpace(s))
	if !formatoEmailValido(v) {
		return Email{}, ErrEmailInvalido
	}
	return Email{valor: v}, nil
}

// formatoEmailValido: exatamente um "@", parte local não vazia, domínio com
// ponto interno e nenhum espaço ou caractere de controle.
func formatoEmailValido(v string) bool {
	for _, r := range v {
		if r <= ' ' || r == 0x7f {
			return false
		}
	}
	local, dominio, ok := strings.Cut(v, "@")
	if !ok || local == "" || strings.Contains(dominio, "@") {
		return false
	}
	ponto := strings.LastIndex(dominio, ".")
	return ponto > 0 && ponto < len(dominio)-1
}

// String devolve o endereço normalizado. Não use em log: use Mascarado.
func (e Email) String() string { return e.valor }

// Mascarado devolve o e-mail para log ("a***@dominio"), com o mesmo
// resultado de vlog.MaskEmail (o kernel não importa vlog; a equivalência é
// coberta por teste).
func (e Email) Mascarado() string {
	at := strings.Index(e.valor, "@")
	if at <= 0 {
		return "***"
	}
	return e.valor[:1] + "***" + e.valor[at:]
}
