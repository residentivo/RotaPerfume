// Package middleware_test contém testes unitários para o CORSMiddleware.
package middleware_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
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
	m := httpx.CORS(allowed)

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
	m := httpx.CORS(allowed)

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
	m := httpx.CORS([]string{"http://localhost:3000"})

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
	m := httpx.CORS([]string{"http://localhost:3000"})

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
	m := httpx.CORS([]string{"http://localhost:3000"})

	req := httptest.NewRequest("GET", "/api/produtos", nil)
	w := httptest.NewRecorder()
	m(corsDummyHandler).ServeHTTP(w, req)

	assert.Contains(t, w.Header().Values("Vary"), "Origin")
}

// ------------------------------------------------------------------
// CORSMiddleware — SEC-17: "*" e "null" na lista são ignorados (sem curinga)
// ------------------------------------------------------------------

func TestCORSMiddleware_Wildcard(t *testing.T) {
	for _, lista := range [][]string{{"*"}, {" * "}, {"null"}, {"NULL"}, {"*", "null"}} {
		for _, origem := range []string{"http://qualquer-coisa.com", "null", "*"} {
			t.Run(strings.Join(lista, ",")+"/"+origem, func(t *testing.T) {
				m := httpx.CORS(lista)

				req := httptest.NewRequest("GET", "/api/produtos", nil)
				req.Header.Set("Origin", origem)
				w := httptest.NewRecorder()
				m(corsDummyHandler).ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))

				pre := httptest.NewRequest(http.MethodOptions, "/api/produtos", nil)
				pre.Header.Set("Origin", origem)
				w = httptest.NewRecorder()
				m(corsDummyHandler).ServeHTTP(w, pre)
				assert.Equal(t, http.StatusNoContent, w.Code, "OPTIONS continua 204")
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
			})
		}
	}
}

// SEC-17: o curinga ignorado não derruba as origens explícitas da lista, e o
// descarte gera um único log na montagem (nada por request).
func TestCORSMiddleware_CuringaIgnorado_MantemExplicitasELogaUmaVez(t *testing.T) {
	var buf bytes.Buffer
	saida := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saida) })

	m := httpx.CORS([]string{"*", "https://app.rotaperfumes.com", "null"})
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/produtos", nil)
		req.Header.Set("Origin", "https://app.rotaperfumes.com")
		w := httptest.NewRecorder()
		m(corsDummyHandler).ServeHTTP(w, req)
		assert.Equal(t, "https://app.rotaperfumes.com", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	}
	assert.Equal(t, 1, strings.Count(buf.String(), "[cors] entrada curinga/null ignorada na allowlist"))
}

// ------------------------------------------------------------------
// CORSMiddleware — parametrizado, comportamento esperado por origem
// ------------------------------------------------------------------

func TestCORSMiddleware_Parametrizado(t *testing.T) {
	allowed := []string{"http://localhost:3000", "https://app.rotaperfumes.com"}
	m := httpx.CORS(allowed)

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
