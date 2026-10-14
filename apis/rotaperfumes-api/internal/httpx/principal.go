package httpx

import (
	"context"
	"net/http"

	"github.com/rotaperfumes/shared/kernel"
)

// chavePrincipal é a chave de contexto do Principal. O tipo não é exportado
// e o pacote não exporta setter (O4/O5): só o Autenticador grava o
// Principal, depois de a autenticação aprovar o request.
type chavePrincipal struct{}

func comPrincipal(ctx context.Context, p kernel.Principal) context.Context {
	return context.WithValue(ctx, chavePrincipal{}, p)
}

// PrincipalFrom devolve o Principal do request autenticado. ok=false em rota
// Public ou se algo falhou; em handler protegido, !ok → 500 "erro interno"
// (use kernel.ErrPrincipalInvalido com ResponderErro).
func PrincipalFrom(r *http.Request) (kernel.Principal, bool) {
	return PrincipalDoContexto(r.Context())
}

// PrincipalDoContexto é PrincipalFrom a partir do contexto (ponte de leitura
// para os handlers legados via middleware.GetUserID/GetRole até o ARQ-07).
// Só leitura: não existe setter exportado.
func PrincipalDoContexto(ctx context.Context) (kernel.Principal, bool) {
	p, ok := ctx.Value(chavePrincipal{}).(kernel.Principal)
	if !ok || !p.Valido() {
		return kernel.Principal{}, false
	}
	return p, true
}
