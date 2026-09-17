// Package middleware_test contém testes unitários para o SecurityHeadersMiddleware.
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
)

var securityHeadersDummyHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{}`))
}

// ------------------------------------------------------------------
// SecurityHeadersMiddleware — headers básicos sempre presentes
// ------------------------------------------------------------------

func TestSecurityHeadersMiddleware_HeadersPresentes(t *testing.T) {
	m := middleware.SecurityHeadersMiddleware()

	req := httptest.NewRequest("GET", "/api/produtos", nil)
	req.Host = "api.rotaperfumes.com"
	w := httptest.NewRecorder()
	m(securityHeadersDummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "default-src 'none'; frame-ancestors 'none'", w.Header().Get("Content-Security-Policy"))
}

func TestSecurityHeadersMiddleware_ChamaHandlerDownstream(t *testing.T) {
	m := middleware.SecurityHeadersMiddleware()

	req := httptest.NewRequest("GET", "/api/produtos", nil)
	w := httptest.NewRecorder()
	m(securityHeadersDummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{}`, w.Body.String())
}

// ------------------------------------------------------------------
// SecurityHeadersMiddleware — HSTS condicional por host (localhost vs prod)
// ------------------------------------------------------------------

func TestSecurityHeadersMiddleware_HSTS_Parametrizado(t *testing.T) {
	m := middleware.SecurityHeadersMiddleware()

	cases := []struct {
		nome     string
		host     string
		wantHSTS bool
	}{
		{"localhost sem porta", "localhost", false},
		{"localhost com porta", "localhost:8080", false},
		{"127.0.0.1 sem porta", "127.0.0.1", false},
		{"127.0.0.1 com porta", "127.0.0.1:8080", false},
		{"0.0.0.0 com porta", "0.0.0.0:8080", false},
		{"IPv6 loopback sem porta", "::1", false},
		{"IPv6 loopback com porta", "[::1]:8080", false},
		{"domínio de produção", "api.rotaperfumes.com", true},
		{"domínio de produção com porta", "api.rotaperfumes.com:443", true},
	}

	for _, c := range cases {
		t.Run(c.nome, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/produtos", nil)
			req.Host = c.host
			w := httptest.NewRecorder()
			m(securityHeadersDummyHandler).ServeHTTP(w, req)

			hsts := w.Header().Get("Strict-Transport-Security")
			if c.wantHSTS {
				assert.NotEmpty(t, hsts, "host %q deveria receber HSTS", c.host)
				assert.Contains(t, hsts, "max-age=")
			} else {
				assert.Empty(t, hsts, "host %q NÃO deveria receber HSTS", c.host)
			}
		})
	}
}
