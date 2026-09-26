package middleware_test

// SEC-06 (complemento do 🔴 TestBrain, Lote 6): o front autentica pelo cookie
// HttpOnly access_token (não pelo header). A checagem de ativo/role tem de
// valer também nesse caminho. E as rotas públicas (login/refresh/logout)
// seguem sem consultar o banco, qualquer que seja o token enviado.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
)

func TestUserCheck_CookieAccessToken(t *testing.T) {
	cfg := testCfg()
	casos := []struct {
		nome       string
		checker    *mockChecker
		requireAdm bool
		wantStatus int
		wantCalled bool
		wantMsg    string
	}{
		{"ativo", &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "normal"}}, false, http.StatusOK, true, ""},
		{"inativo", &mockChecker{status: middleware.UserStatus{Ativo: false, Role: "normal"}}, false, http.StatusUnauthorized, false, "usuário inativo"},
		{"inexistente", &mockChecker{err: middleware.ErrUserNotFound}, false, http.StatusUnauthorized, false, "usuário inativo"},
		{"erro de banco", &mockChecker{err: errors.New("db down")}, false, http.StatusInternalServerError, false, "erro interno"},
		{"admin rebaixado em rota admin", &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "normal"}}, true, http.StatusForbidden, false, "acesso restrito a administradores"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				dummyHandler(w, r)
			})
			req := httptest.NewRequest("GET", "/protected", nil)
			req.AddCookie(&http.Cookie{Name: "access_token", Value: generateToken(t, cfg, 42, "admin", time.Hour, "")})
			w := httptest.NewRecorder()
			middleware.JWTMiddlewareWithUserCheck(cfg, c.checker, true, c.requireAdm)(next).ServeHTTP(w, req)

			assert.Equal(t, c.wantStatus, w.Code)
			assert.Equal(t, c.wantCalled, called)
			assert.Equal(t, 1, c.checker.calls, "cookie também consulta o banco")
			assert.Equal(t, int64(42), c.checker.lastID)
			if c.wantMsg != "" {
				assert.Contains(t, w.Body.String(), c.wantMsg)
			}
		})
	}
}

func TestUserCheck_RotaPublica_QualquerTokenSegueSemConsultarBanco(t *testing.T) {
	cfg := testCfg()
	casos := []struct {
		nome   string
		header string
		cookie string
	}{
		{"header mal formado", "Basic abc", ""},
		{"header sem esquema", "so-um-pedaco", ""},
		{"JWT com assinatura errada", "Bearer " + generateToken(t, cfg, 7, "admin", time.Hour, "outro-segredo"), ""},
		{"JWT expirado", "Bearer " + generateToken(t, cfg, 7, "admin", -time.Minute, ""), ""},
		{"cookie lixo", "", "nao.e.jwt"},
		{"cookie válido de usuário que o banco diria inativo", "", generateToken(t, cfg, 7, "normal", time.Hour, "")},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			chk := &mockChecker{status: middleware.UserStatus{Ativo: false}}
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				dummyHandler(w, r)
			})
			req := httptest.NewRequest("POST", "/api/auth/refresh", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			if c.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "access_token", Value: c.cookie})
			}
			w := httptest.NewRecorder()
			middleware.JWTMiddlewareWithUserCheck(cfg, chk, false, false)(next).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, called, "rota pública sempre chega ao handler")
			assert.Equal(t, 0, chk.calls, "rota pública não consulta o banco")
		})
	}
}
