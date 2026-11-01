package kernel

import (
	"context"
	"strconv"
	"strings"
	"unicode"
)

// chaveRota é a chave de contexto da rota HTTP (tipo não exportado).
type chaveRota struct{}

// ComRota guarda no contexto o path da requisição (sem query string), só
// para os logs operacionais da aplicação (N4: "[escopo] bloqueado
// vendedor_desligado … rota="). Gravado pelo httpx.Autenticador; a aplicação
// lê com RotaDe sem conhecer net/http. ARQ-07 NB-C: o path já vem
// decodificado (%0A vira "\n"), então é saneado aqui, no ponto único.
func ComRota(ctx context.Context, rota string) context.Context {
	return context.WithValue(ctx, chaveRota{}, SanearRota(rota))
}

// RotaDe devolve a rota gravada por ComRota ("" fora de requisição HTTP, ex.:
// CLI).
func RotaDe(ctx context.Context) string {
	r, _ := ctx.Value(chaveRota{}).(string)
	return r
}

// SanearRota troca caracteres de controle (e os separadores de linha
// Unicode) pela forma escapada de Go (ex.: "\n" → `\n`), para um path
// decodificado não injetar linhas falsas no log (ARQ-07 NB-C). Path comum
// volta inalterado, sem alocação.
func SanearRota(rota string) string {
	if strings.IndexFunc(rota, ehQuebraDeLog) < 0 {
		return rota
	}
	var b strings.Builder
	for _, r := range rota {
		if !ehQuebraDeLog(r) {
			b.WriteRune(r)
			continue
		}
		q := strconv.QuoteRuneToASCII(r)
		b.WriteString(q[1 : len(q)-1])
	}
	return b.String()
}

func ehQuebraDeLog(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp)
}
