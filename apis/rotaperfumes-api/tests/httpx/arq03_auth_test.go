package httpx_test

// ARQ-03: Autenticador nativo (I-01/I-02) e IP do cliente (I-14).

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

// validadorFixo devolve sempre os mesmos claims (a assinatura é do adapter
// JWT; aqui só a checagem defensiva do Autenticador importa).
type validadorFixo struct {
	c   identidade.ClaimsAcesso
	err error
}

func (v validadorFixo) ValidarAcesso(string) (identidade.ClaimsAcesso, error) { return v.c, v.err }

func TestAutenticador_ChecagemDefensivaDeExpiracao(t *testing.T) {
	expirou := time.Now().Add(-time.Minute)
	calls := 0
	a := httpx.NovoAutenticador(validadorFixo{c: identidade.ClaimsAcesso{UserID: 2, Role: kernel.RoleNormal, ExpiraEm: &expirou}}, checkerFake{calls: &calls})
	w := servir(a.Exigir(httpx.Authenticated, http.HandlerFunc(handlerOK)), "GET", "/x", "tok")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "access token expirado — use /api/auth/refresh")
	assert.Zero(t, calls, "token recusado não consulta o banco (I-03)")
}

func TestAutenticador_BearerSemToken_E_ValidadorRecusa(t *testing.T) {
	calls := 0
	a := httpx.NovoAutenticador(validadorFixo{err: errors.New("inválido")}, checkerFake{calls: &calls})
	h := a.Exigir(httpx.Authenticated, http.HandlerFunc(handlerOK))

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer ")
	req.AddCookie(&http.Cookie{Name: httpx.CookieAcesso, Value: "cookie-valido"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "authorization header ausente", "Bearer vazio não cai no cookie (I-02)")

	w = servir(h, "GET", "/x", "qualquer")
	assert.Contains(t, w.Body.String(), "token inválido ou expirado")
	assert.Zero(t, calls)
}

func TestIPDoCliente(t *testing.T) {
	casos := []struct {
		nome   string
		remoto string
		xff    string
		real   string
		confia bool
		want   string
	}{
		{"sem proxy: RemoteAddr sem porta", "10.0.0.1:5555", "1.1.1.1", "2.2.2.2", false, "10.0.0.1"},
		{"proxy confiável: 1º do XFF", "10.0.0.1:5555", " 1.1.1.1 , 9.9.9.9", "2.2.2.2", true, "1.1.1.1"},
		{"proxy confiável sem XFF: X-Real-IP", "10.0.0.1:5555", "", "2.2.2.2", true, "2.2.2.2"},
		{"proxy confiável sem headers: RemoteAddr", "10.0.0.1:5555", "", "", true, "10.0.0.1"},
		{"RemoteAddr sem porta fica como está", "10.0.0.1", "", "", false, "10.0.0.1"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = c.remoto
			if c.xff != "" {
				r.Header.Set("X-Forwarded-For", c.xff)
			}
			if c.real != "" {
				r.Header.Set("X-Real-IP", c.real)
			}
			assert.Equal(t, c.want, httpx.IPDoCliente(r, c.confia))
		})
	}
}

func TestCookiesDeSessao(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "https://api.exemplo.com/x", nil)
	httpx.DefinirCookiesSessao(w, r, "acesso", "refresh", time.Hour)
	cs := w.Result().Cookies()
	if assert.Len(t, cs, 2) {
		assert.Equal(t, httpx.CookieAcesso, cs[0].Name)
		assert.True(t, cs[0].HttpOnly && cs[0].Secure)
		assert.Equal(t, http.SameSiteStrictMode, cs[0].SameSite)
		assert.Equal(t, 3600, cs[0].MaxAge)
		assert.Equal(t, http.SameSiteLaxMode, cs[1].SameSite)
		assert.Equal(t, 7*24*3600, cs[1].MaxAge)
	}
	local := httptest.NewRequest("POST", "http://localhost:8080/x", nil)
	w = httptest.NewRecorder()
	httpx.DefinirCookiesSessao(w, local, "a", "r", time.Hour)
	assert.False(t, w.Result().Cookies()[0].Secure, "SEC-21 preservado: Secure por r.Host")
	assert.Equal(t, "", httpx.CookieRefreshDe(local))
}

func TestPrincipalDoContexto_SemPrincipal(t *testing.T) {
	_, ok := httpx.PrincipalDoContexto(context.Background())
	assert.False(t, ok)
}
