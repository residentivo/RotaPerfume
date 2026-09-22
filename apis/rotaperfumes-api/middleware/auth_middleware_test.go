// Package middleware_test contém testes unitários para o JWTMiddleware.
package middleware_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

// config de teste com segredo fixo.
func testCfg() *config.Config {
	return &config.Config{
		JWTSecret:  "middleware-test-secret-abc123",
		JWTIssuer:  "rotaperfumes-test",
		JWTTTL:     1 * time.Hour,
		BCryptCost: 4,
	}
}

// generateToken é um helper que gera um JWT com os claims desejados.
func generateToken(t *testing.T, cfg *config.Config, uid int64, role string, ttl time.Duration, secret string) string {
	if secret == "" {
		secret = cfg.JWTSecret
	}
	// Cria token manualmente para controlar ExpiresAt.
	now := time.Now()
	claims := services.AuthClaims{
		UserID: uid,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWTIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   fmt.Sprintf("%d", uid),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

// dummyHandler é um http.Handler que verifica que foi chamado.
// Coloca os claims injetados pelo middleware nos headers de resposta
// para que o teste possa validá-los via w.Result().
var dummyHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUserID(r.Context())
	role, ok2 := middleware.GetRole(r.Context())
	if ok {
		w.Header().Set("X-User-ID", fmt.Sprintf("%d", uid))
	}
	if ok2 {
		w.Header().Set("X-User-Role", role)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{}`))
}

// ------------------------------------------------------------------
// JWTMiddleware — token válido
// ------------------------------------------------------------------

func TestJWTMiddleware_ValidToken(t *testing.T) {
	cfg := testCfg()
	m := middleware.JWTMiddleware(cfg, true, false)

	token := generateToken(t, cfg, 42, "admin", 1*time.Hour, "")

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	m(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "handler downstream deve ser chamado")
	assert.Equal(t, "42", w.Header().Get("X-User-ID"), "userID deve estar injetado via header")
	assert.Equal(t, "admin", w.Header().Get("X-User-Role"), "role deve estar injetado via header")
}

func TestJWTMiddleware_ValidToken_NormalRole(t *testing.T) {
	cfg := testCfg()
	m := middleware.JWTMiddleware(cfg, true, false)

	token := generateToken(t, cfg, 7, "normal", 1*time.Hour, "")

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	m(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "normal", w.Header().Get("X-User-Role"))
}

// ------------------------------------------------------------------
// JWTMiddleware — token ausente ou malformado
// ------------------------------------------------------------------

func TestJWTMiddleware_MissingToken(t *testing.T) {
	cfg := testCfg()
	m := middleware.JWTMiddleware(cfg, true, false)

	t.Run("sem header authorization", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		w := httptest.NewRecorder()
		m(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "authorization header ausente")
	})

	t.Run("header vazio", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "")
		w := httptest.NewRecorder()
		m(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("apenas Bearer sem token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer")
		w := httptest.NewRecorder()
		m(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("token com prefixo errado", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Basic abc123")
		w := httptest.NewRecorder()
		m(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rota pública (protected=false) não exige token", func(t *testing.T) {
		pub := middleware.JWTMiddleware(cfg, false, false)
		req := httptest.NewRequest("GET", "/public", nil)
		w := httptest.NewRecorder()
		pub(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "rota pública deve permitir acesso sem token")
	})
}

// ------------------------------------------------------------------
// JWTMiddleware — token inválido / manipulado
// ------------------------------------------------------------------

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	cfg := testCfg()
	m := middleware.JWTMiddleware(cfg, true, false)

	t.Run("string aleatória", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer nao-eh-um-jwt-valido")
		w := httptest.NewRecorder()
		m(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "inválido")
	})

	t.Run("token manipulado", func(t *testing.T) {
		token := generateToken(t, cfg, 1, "admin", 1*time.Hour, "")
		// Altera a payload (antes da assinatura).
		manipulado := token[:len(token)-5] + "XXXXX"
		if manipulado == token {
			manipulado = token[:len(token)/2] + "MANIPULADO" + token[len(token)/2:]
		}

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+manipulado)
		w := httptest.NewRecorder()
		m(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("segredo errado", func(t *testing.T) {
		token := generateToken(t, cfg, 1, "admin", 1*time.Hour, "outro-segredo")

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		m(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// ------------------------------------------------------------------
// JWTMiddleware — token expirado
// ------------------------------------------------------------------

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	cfg := testCfg()
	m := middleware.JWTMiddleware(cfg, true, false)

	// Token com TTL negativo = expirado no momento da geração.
	token := generateToken(t, cfg, 1, "admin", -1*time.Hour, "")

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	m(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "inválido")
}

// ------------------------------------------------------------------
// JWTMiddleware — requireAdmin
// ------------------------------------------------------------------

func TestJWTMiddleware_RequireAdmin_Success(t *testing.T) {
	cfg := testCfg()
	// protected=true, requireAdmin=true.
	m := middleware.JWTMiddleware(cfg, true, true)

	token := generateToken(t, cfg, 1, "admin", 1*time.Hour, "")

	req := httptest.NewRequest("GET", "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	m(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "admin com token válido deve passar")
	assert.Equal(t, "admin", w.Header().Get("X-User-Role"), "role deve estar injetado")
}

func TestJWTMiddleware_RequireAdmin_Forbidden(t *testing.T) {
	cfg := testCfg()
	m := middleware.JWTMiddleware(cfg, true, true)

	t.Run("role normal é bloqueado", func(t *testing.T) {
		token := generateToken(t, cfg, 2, "normal", 1*time.Hour, "")

		req := httptest.NewRequest("GET", "/admin-only", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		m(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "administradores")
	})

	t.Run("token de admin expirado é 401 (não chega no requireAdmin)", func(t *testing.T) {
		token := generateToken(t, cfg, 1, "admin", -1*time.Hour, "")

		req := httptest.NewRequest("GET", "/admin-only", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		m(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code, "token expirado retorna 401 antes de verificar role")
	})

	t.Run("sem token em rota admin é 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin-only", nil)
		w := httptest.NewRecorder()
		m(dummyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("parametrizado: roles não-admin", func(t *testing.T) {
		for _, role := range []string{"normal", "", "vendedor", "root", "ADMIN"} {
			t.Run(role, func(t *testing.T) {
				token := generateToken(t, cfg, 3, role, 1*time.Hour, "")

				req := httptest.NewRequest("GET", "/admin-only", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				m(dummyHandler).ServeHTTP(w, req)

				assert.Equal(t, http.StatusForbidden, w.Code,
					"role %q deve ser bloqueada", role)
			})
		}
	})
}

// ------------------------------------------------------------------
// Helpers — GetUserID / GetRole
// ------------------------------------------------------------------

func TestGetUserID_NotSet(t *testing.T) {
	// Contexto sem injeção retorna zero values.
	uid, ok := middleware.GetUserID(context.Background())
	assert.False(t, ok)
	assert.Equal(t, int64(0), uid)
}

func TestGetRole_NotSet(t *testing.T) {
	role, ok := middleware.GetRole(context.Background())
	assert.False(t, ok)
	assert.Equal(t, "", role)
}

// ------------------------------------------------------------------
// Casos combinatórios
// ------------------------------------------------------------------

func TestJWTMiddleware_Combinatorial(t *testing.T) {
	cfg := testCfg()
	normalToken := generateToken(t, cfg, 2, "normal", 1*time.Hour, "")
	expiredToken := generateToken(t, cfg, 1, "admin", -1*time.Hour, "")

	cases := []struct {
		name         string
		token        string
		protected    bool
		requireAdmin bool
		wantStatus   int
	}{
		{"público sem token", "", false, false, http.StatusOK},
		{"protegido sem token", "", true, false, http.StatusUnauthorized},
		{"protegido sem token admin", "", true, true, http.StatusUnauthorized},
		{"protegido admin com token normal", normalToken, true, true, http.StatusForbidden},
		{"protegido com token expirado", expiredToken, true, false, http.StatusUnauthorized},
		{"protegido admin com token expirado", expiredToken, true, true, http.StatusUnauthorized},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := middleware.JWTMiddleware(cfg, c.protected, c.requireAdmin)
			req := httptest.NewRequest("GET", "/test", nil)
			if c.token != "" {
				req.Header.Set("Authorization", "Bearer "+c.token)
			}
			w := httptest.NewRecorder()
			m(dummyHandler).ServeHTTP(w, req)

			assert.Equal(t, c.wantStatus, w.Code, "caso: %s", c.name)
		})
	}
}

func TestJWTMiddleware_ResponseFormat(t *testing.T) {
	cfg := testCfg()
	m := middleware.JWTMiddleware(cfg, true, false)

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	m(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "json")
	assert.Contains(t, w.Body.String(), `"success":false`)
	assert.Contains(t, w.Body.String(), `"error"`)
}

// ------------------------------------------------------------------
// TestJWTMiddleware_Context — verifica que o contexto é injetado corretamente
// ------------------------------------------------------------------

func TestJWTMiddleware_Context(t *testing.T) {
	cfg := testCfg()
	m := middleware.JWTMiddleware(cfg, true, false)
	token := generateToken(t, cfg, 99, "normal", 1*time.Hour, "")

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	m(dummyHandler).ServeHTTP(w, req)

	// Verifica via headers de resposta (o dummyHandler escreve os valores lá).
	body := w.Body.String()
	if body == "" {
		// Tenta ler JSON do body
		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
	}
	assert.Equal(t, http.StatusOK, w.Code)
	uidHeader := w.Header().Get("X-User-ID")
	roleHeader := w.Header().Get("X-User-Role")
	assert.Equal(t, "99", uidHeader, "userID injetado via header X-User-ID")
	assert.Equal(t, "normal", roleHeader, "role injetado via header X-User-Role")
}
