// Package middleware_test contém testes unitários para o CORSMiddleware.
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
)

var corsDummyHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{}`))
}

// ------------------------------------------------------------------
// CORSMiddleware — origem exata permitida
// ------------------------------------------------------------------

func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	allowed := []string{"http://localhost:3000", "https://app.rotaperfumes.com"}
	m := middleware.CORSMiddleware(allowed)

	req := httptest.NewRequest("GET", "/api/produtos", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	m(corsDummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

// ------------------------------------------------------------------
// CORSMiddleware — origem fora da lista é rejeitada (sem headers CORS)
// ------------------------------------------------------------------

func TestCORSMiddleware_RejectedOrigin(t *testing.T) {
	allowed := []string{"http://localhost:3000"}
	m := middleware.CORSMiddleware(allowed)

	testCases := []string{
		"http://localhost:4000", // porta diferente — não deve dar prefix-match
		"http://evil.com",
		"http://127.0.0.1:3000",  // host diferente, mesma porta
		"https://localhost:3000", // esquema diferente
	}

	for _, origin := range testCases {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/produtos", nil)
			req.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			m(corsDummyHandler).ServeHTTP(w, req)

			// Handler downstream ainda é chamado (CORS não bloqueia a requisição
			// em si — apenas omite os headers que o browser exige para JS ler
			// a resposta cross-origin), mas nenhum header CORS deve ser setado.
			assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"),
				"origem %q não deve receber Access-Control-Allow-Origin", origin)
			assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
		})
	}
}

// ------------------------------------------------------------------
// CORSMiddleware — sem header Origin (requisição same-origin/server-to-server)
// ------------------------------------------------------------------

func TestCORSMiddleware_NoOriginHeader(t *testing.T) {
	m := middleware.CORSMiddleware([]string{"http://localhost:3000"})

	req := httptest.NewRequest("GET", "/api/produtos", nil)
	w := httptest.NewRecorder()
	m(corsDummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

// ------------------------------------------------------------------
// CORSMiddleware — preflight OPTIONS
// ------------------------------------------------------------------

func TestCORSMiddleware_Preflight(t *testing.T) {
	m := middleware.CORSMiddleware([]string{"http://localhost:3000"})

	req := httptest.NewRequest("OPTIONS", "/api/produtos", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	m(corsDummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, w.Body.String(), "preflight não deve chamar o handler downstream")
}

// ------------------------------------------------------------------
// CORSMiddleware — Vary: Origin sempre presente
// ------------------------------------------------------------------

func TestCORSMiddleware_VaryHeader(t *testing.T) {
	m := middleware.CORSMiddleware([]string{"http://localhost:3000"})

	req := httptest.NewRequest("GET", "/api/produtos", nil)
	w := httptest.NewRecorder()
	m(corsDummyHandler).ServeHTTP(w, req)

	assert.Contains(t, w.Header().Values("Vary"), "Origin")
}

// ------------------------------------------------------------------
// CORSMiddleware — wildcard "*" libera qualquer origem
// ------------------------------------------------------------------

func TestCORSMiddleware_Wildcard(t *testing.T) {
	m := middleware.CORSMiddleware([]string{"*"})

	req := httptest.NewRequest("GET", "/api/produtos", nil)
	req.Header.Set("Origin", "http://qualquer-coisa.com")
	w := httptest.NewRecorder()
	m(corsDummyHandler).ServeHTTP(w, req)

	assert.Equal(t, "http://qualquer-coisa.com", w.Header().Get("Access-Control-Allow-Origin"))
}

// ------------------------------------------------------------------
// CORSMiddleware — parametrizado, comportamento esperado por origem
// ------------------------------------------------------------------

func TestCORSMiddleware_Parametrizado(t *testing.T) {
	allowed := []string{"http://localhost:3000", "https://app.rotaperfumes.com"}
	m := middleware.CORSMiddleware(allowed)

	cases := []struct {
		nome        string
		origin      string
		wantAllowed bool
	}{
		{"origem dev exata", "http://localhost:3000", true},
		{"origem produção exata", "https://app.rotaperfumes.com", true},
		{"porta diferente do dev", "http://localhost:3001", false},
		{"subdomínio não listado", "https://staging.app.rotaperfumes.com", false},
		{"http em vez de https", "http://app.rotaperfumes.com", false},
		{"origem totalmente distinta", "http://malicious-site.com", false},
	}

	for _, c := range cases {
		t.Run(c.nome, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/produtos", nil)
			req.Header.Set("Origin", c.origin)
			w := httptest.NewRecorder()
			m(corsDummyHandler).ServeHTTP(w, req)

			if c.wantAllowed {
				assert.Equal(t, c.origin, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}
