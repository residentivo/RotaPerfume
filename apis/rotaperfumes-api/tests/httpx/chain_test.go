package httpx_test

// Chain (F-01), CORS (O3), Logging (F-09) e Trace (nível 1 do vlog).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
)

func marcador(nome string, ordem *[]string) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*ordem = append(*ordem, nome)
			next.ServeHTTP(w, r)
		})
	}
}

func TestChain_OrdemDoMaisExternoParaOMaisInterno(t *testing.T) {
	var ordem []string
	final := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ordem = append(ordem, "handler") })
	h := httpx.Chain(final, marcador("A", &ordem), marcador("B", &ordem), marcador("C", &ordem))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, []string{"A", "B", "C", "handler"}, ordem)

	ordem = nil
	httpx.Chain(final).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, []string{"handler"}, ordem, "sem middlewares, só o handler")
}

// O3: todo OPTIONS → 204 sem chamar o próximo, com ou sem Origin.
func TestCORS_OptionsSempre204SemChamarNext(t *testing.T) {
	casos := []struct {
		nome     string
		origem   string
		wantACAO string
	}{
		{"sem Origin", "", ""},
		{"Origin permitida", "http://app.test", "http://app.test"},
		{"Origin fora da allowlist", "http://evil.test", ""},
		{"prefixo da permitida não vale", "http://app.test.evil", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			chamado := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { chamado = true })
			req := httptest.NewRequest(http.MethodOptions, "/api/auth/login", nil)
			if c.origem != "" {
				req.Header.Set("Origin", c.origem)
			}
			w := httptest.NewRecorder()
			httpx.CORS([]string{"http://app.test"})(next).ServeHTTP(w, req)

			assert.Equal(t, http.StatusNoContent, w.Code)
			assert.False(t, chamado, "preflight nunca chega ao mux")
			assert.Equal(t, c.wantACAO, w.Header().Get("Access-Control-Allow-Origin"))
			assert.Contains(t, w.Header().Values("Vary"), "Origin")
		})
	}
}

// F-01: com a cadeia da API, OPTIONS → 204 e não-OPTIONS segue com os
// security headers aplicados mesmo no 405 do mux.
func TestChain_DaAPI_CORSAntesDoMux(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", handlerOK)
	h := httpx.Chain(mux, httpx.CORS([]string{"http://app.test"}), httpx.SecurityHeaders)

	w := servir(h, http.MethodOptions, "/api/auth/login", "")
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = servir(h, http.MethodGet, "/api/auth/login", "")
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "POST", w.Header().Get("Allow"))
}

func TestLogging_SemQueryString(t *testing.T) {
	out := capturarLog(t, false)
	h := httpx.Logging(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/clientes?q=razao-secreta&token=abc", nil))

	linha := out.String()
	assert.True(t, strings.HasPrefix(linha, "[http] GET /api/clientes -> 418 ("), linha)
	assert.NotContains(t, linha, "razao-secreta")
	assert.NotContains(t, linha, "token=")
}

func TestLogging_StatusPadrao200(t *testing.T) {
	out := capturarLog(t, false)
	h := httpx.Logging(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/x", nil))
	assert.Contains(t, out.String(), "[http] POST /x -> 200 (")
}

func TestTrace_LigadoEDesligado(t *testing.T) {
	h := httpx.Trace("x_handler.go", "XHandler.Get", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	out := capturarLog(t, false)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/x?segredo=1", nil))
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Empty(t, out.String(), "vlog desligado por padrão (F-09)")

	out = capturarLog(t, true)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x?segredo=1", nil))
	assert.Equal(t, "[x_handler.go] [XHandler.Get] início GET\n[x_handler.go] [XHandler.Get] fim status=201\n", out.String())
}
