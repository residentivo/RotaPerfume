package middleware_test

// ARQ-03: o middleware JWT legado (JWTMiddleware, JWTMiddlewareWithUserCheck)
// saiu; a autenticação é o httpx.Autenticador. Os testes (mesmos nomes e
// asserções) passam por estes dois montadores, que só traduzem o par de
// booleanos antigo para a política tipada:
//   - protected=false → rota Public: o Autenticador nem é montado (rota
//     pública não autentica nem consulta o banco);
//   - protected=true  → Exigir(Authenticated); requireAdmin → AdminOnly.
// jwtMiddleware reproduz a variante "sem checker" (C11): o status vem do
// próprio token (ativo, role do claim), sem banco.

import (
	"context"
	"net/http"
	"strings"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
)

func jwtMiddlewareWithUserCheck(cfg *config.Config, checker httpx.ConsultaStatus, protected, requireAdmin bool) func(http.Handler) http.Handler {
	a := httpx.NovoAutenticador(identidadecrypto.NovoJWT(cfg), checker) // checker nil: panic (I-03)
	return politica(a, protected, requireAdmin)
}

func jwtMiddleware(cfg *config.Config, protected, requireAdmin bool) func(http.Handler) http.Handler {
	auth := jwtMiddlewareWithUserCheck(cfg, statusDoToken{identidadecrypto.NovoJWT(cfg)}, protected, requireAdmin)
	return func(next http.Handler) http.Handler {
		interno := auth(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			interno.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), chaveToken{}, tokenBruto(r))))
		})
	}
}

// tokenBruto lê o token como o Autenticador (Bearer ou cookie), só para o
// statusDoToken saber o role do claim.
func tokenBruto(r *http.Request) string {
	if _, tok, ok := strings.Cut(r.Header.Get("Authorization"), " "); ok {
		return tok
	}
	if c, err := r.Cookie(httpx.CookieAcesso); err == nil {
		return c.Value
	}
	return ""
}

func politica(a *httpx.Autenticador, protected, requireAdmin bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		switch {
		case !protected:
			return next
		case requireAdmin:
			return a.Exigir(httpx.AdminOnly, next)
		default:
			return a.Exigir(httpx.Authenticated, next)
		}
	}
}

// statusDoToken é a consulta "sem banco" do JWTMiddleware legado: o role do
// claim vale como role vigente.
type statusDoToken struct{ jwt *identidadecrypto.JWT }

func (s statusDoToken) StatusParaAutenticacao(ctx context.Context, userID int64) (identidade.StatusAcesso, error) {
	tok, _ := tokenDoContexto(ctx)
	c, err := s.jwt.ValidarAcesso(tok)
	if err != nil {
		return identidade.StatusAcesso{}, err
	}
	return identidade.StatusAcesso{Ativo: true, Role: c.Role}, nil
}

type chaveToken struct{}

func tokenDoContexto(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(chaveToken{}).(string)
	return t, ok
}
