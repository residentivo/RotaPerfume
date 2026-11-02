package httpx_test

// SEC-33 (Lote 31): com COOKIE_SECURE=true os cookies se chamam
// __Host-access_token e __Host-refresh_token (Secure, Path=/, sem Domain) e
// o backend lê SÓ esses nomes; com false (dev em HTTP) ficam access_token e
// refresh_token sem Secure. No modo seguro, definir e limpar também expiram
// os nomes antigos (transição).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
)

func sec33Autenticado(t *testing.T, seguro bool, nomeCookie string) int {
	t.Helper()
	a := httpx.NovoAutenticador(identidadecrypto.NovoJWT(testCfg()), checkerFake{}).ComCookieSeguro(seguro)
	h := a.Exigir(httpx.Authenticated, http.HandlerFunc(handlerOK))
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	req.AddCookie(&http.Cookie{Name: nomeCookie, Value: token(t, 2, "normal")})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code
}

func TestSEC33_Autenticador_ModoSeguroLeSoHost(t *testing.T) {
	assert.Equal(t, http.StatusOK, sec33Autenticado(t, true, "__Host-access_token"))
	assert.Equal(t, http.StatusUnauthorized, sec33Autenticado(t, true, "access_token"), "nunca cai para o nome antigo")
	assert.Equal(t, http.StatusOK, sec33Autenticado(t, false, "access_token"), "dev: nome antigo")
	assert.Equal(t, http.StatusUnauthorized, sec33Autenticado(t, false, "__Host-access_token"))
}

func TestSEC33_Autenticador_PadraoEhSeguro(t *testing.T) {
	a := httpx.NovoAutenticador(identidadecrypto.NovoJWT(testCfg()), checkerFake{})
	h := a.Exigir(httpx.Authenticated, http.HandlerFunc(handlerOK))
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token(t, 2, "normal")})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "sem ComCookieSeguro, só __Host- (fail closed)")
}

func TestSEC33_CookieRefreshDe_PorPar(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	r.AddCookie(&http.Cookie{Name: "refresh_token", Value: "antigo"})
	assert.Equal(t, "", httpx.CookieRefreshDe(r, httpx.CookiesSeguros), "modo seguro não lê o antigo")
	assert.Equal(t, "antigo", httpx.CookieRefreshDe(r, httpx.CookiesDev))
	r.AddCookie(&http.Cookie{Name: "__Host-refresh_token", Value: "novo"})
	assert.Equal(t, "novo", httpx.CookieRefreshDe(r, httpx.CookiesSeguros))
}

func TestSEC33_DefinirELimpar_ModoSeguroExpiraAntigos(t *testing.T) {
	for nome, escrever := range map[string]func(w http.ResponseWriter){
		"definir": func(w http.ResponseWriter) { httpx.DefinirCookiesSessao(w, "a", "r", time.Hour, true) },
		"limpar":  func(w http.ResponseWriter) { httpx.LimparCookiesSessao(w, true) },
	} {
		t.Run(nome, func(t *testing.T) {
			w := httptest.NewRecorder()
			escrever(w)
			scs := w.Result().Header.Values("Set-Cookie")
			require.Len(t, scs, 4)
			vistos := map[string]string{}
			for _, sc := range scs {
				n, _, _ := strings.Cut(sc, "=")
				vistos[n] = sc
				assert.Contains(t, sc, "Path=/")
				assert.Contains(t, sc, "; Secure")
				assert.NotContains(t, sc, "Domain=")
			}
			for _, antigo := range []string{"access_token", "refresh_token"} {
				assert.Contains(t, vistos[antigo], "Max-Age=0", "antigo expirado")
			}
			assert.Contains(t, vistos["__Host-access_token"], "SameSite=Strict")
			assert.Contains(t, vistos["__Host-refresh_token"], "SameSite=Lax")
		})
	}
	w := httptest.NewRecorder()
	httpx.DefinirCookiesSessao(w, "a", "r", time.Hour, false)
	assert.Len(t, w.Result().Header.Values("Set-Cookie"), 2, "dev não emite expiração de nomes antigos")
}
