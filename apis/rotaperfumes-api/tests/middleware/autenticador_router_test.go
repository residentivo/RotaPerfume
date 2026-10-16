package middleware_test

// ARQ-07 (pendência a): os testes passam pelo httpx.Router REAL. Antes
// (ARQ-03) um repassador traduzia o par de booleanos antigo e, para
// protected=false, devolvia o próprio handler: os testes "rota pública não
// consulta o banco" eram tautológicos. Agora a rota é registrada no Router
// com a política tipada e é o Router que decide que rota Public não passa
// pelo Autenticador (§5.9).

import (
	"context"
	"net/http"
	"strings"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
)

// porPolitica registra o handler em todos os métodos e caminhos de um
// Router real com a política p e devolve o mux.
func porPolitica(a *httpx.Autenticador, p httpx.Policy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		r := httpx.NovoRouter(a)
		for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			r.Handle(httpx.Rota{Metodo: m, Padrao: "/{resto...}", Policy: p, Arquivo: "teste.go", Funcao: "Teste.Handler", Handler: next.ServeHTTP})
		}
		return r.Mux()
	}
}

// politica traduz as colunas protected/requireAdmin das tabelas de casos.
func politica(protected, requireAdmin bool) httpx.Policy {
	switch {
	case !protected:
		return httpx.Public
	case requireAdmin:
		return httpx.AdminOnly
	default:
		return httpx.Authenticated
	}
}

// autenticador é o httpx.Autenticador com o JWT real e o checker informado
// (nil: panic na montagem, I-03).
func autenticador(cfg *config.Config, checker httpx.ConsultaStatus) *httpx.Autenticador {
	return httpx.NovoAutenticador(identidadecrypto.NovoJWT(cfg), checker)
}

// semBanco é a variante "sem checker" (C11) dos testes antigos: o status
// vem do próprio token (ativo, role do claim), por um checker falso que lê
// o token que este invólucro põe no contexto.
func semBanco(cfg *config.Config, p httpx.Policy) func(http.Handler) http.Handler {
	auth := porPolitica(autenticador(cfg, statusDoToken{identidadecrypto.NovoJWT(cfg)}), p)
	return func(next http.Handler) http.Handler {
		interno := auth(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			interno.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), chaveToken{}, tokenBruto(r))))
		})
	}
}

// tokenBruto lê o token como o Autenticador (Bearer ou cookie).
func tokenBruto(r *http.Request) string {
	if _, tok, ok := strings.Cut(r.Header.Get("Authorization"), " "); ok {
		return tok
	}
	if c, err := r.Cookie(httpx.CookieAcesso); err == nil {
		return c.Value
	}
	return ""
}

// statusDoToken: o role do claim vale como role vigente.
type statusDoToken struct{ jwt *identidadecrypto.JWT }

func (s statusDoToken) StatusParaAutenticacao(ctx context.Context, _ int64) (identidade.StatusAcesso, error) {
	tok, _ := ctx.Value(chaveToken{}).(string)
	c, err := s.jwt.ValidarAcesso(tok)
	if err != nil {
		return identidade.StatusAcesso{}, err
	}
	return identidade.StatusAcesso{Ativo: true, Role: c.Role}, nil
}

type chaveToken struct{}
