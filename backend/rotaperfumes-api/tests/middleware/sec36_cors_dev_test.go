package middleware_test

// SEC-36 (Lote 31): as origens de dev (localhost:3000) só entram na
// allowlist de CORS com COOKIE_SECURE=false; o CORS não envia mais
// Access-Control-Expose-Headers: Set-Cookie (sem efeito no navegador).

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/config"
)

func sec36Load(t *testing.T, cookieSecure string) *config.Config {
	t.Helper()
	for k, v := range map[string]string{
		"JWT_SECRET": "segredo-jwt-de-teste-com-32-bytes-ou-mais", "JWT_TTL": "24h",
		"PASSWORD_PEPPER": "pepper-de-teste-com-pelo-menos-32-bytes",
		"DB_USUARIO":      "u", "DB_SENHA": "s", "DB_HOST": "localhost", "COOKIE_SECURE": cookieSecure,
		"CORS_ALLOWED_ORIGINS": "https://app.rotaperfumes.com", "TURNSTILE_HOSTNAMES": "",
		"TURNSTILE_SECRET_KEY": "", "DB_TLS_CA": "", "DB_TLS_SERVER_NAME": "",
	} {
		t.Setenv(k, v)
	}
	cfg, err := config.Load()
	require.NoError(t, err)
	return cfg
}

func sec36Req(origens []string, origem string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/produtos", nil)
	req.Header.Set("Origin", origem)
	w := httptest.NewRecorder()
	httpx.CORS(origens)(corsDummyHandler).ServeHTTP(w, req)
	return w
}

func TestSEC36_Producao_LocalhostSemCORS(t *testing.T) {
	cfg := sec36Load(t, "true")
	for _, o := range []string{"http://localhost:3000", "http://127.0.0.1:3000"} {
		w := sec36Req(cfg.CORSAllowedOrigins, o)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"), o)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"), o)
	}
	w := sec36Req(cfg.CORSAllowedOrigins, "https://app.rotaperfumes.com")
	assert.Equal(t, "https://app.rotaperfumes.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, w.Header().Values("Access-Control-Expose-Headers"), "sem Expose-Headers: Set-Cookie")
}

func TestSEC36_Dev_LocalhostComCORS(t *testing.T) {
	cfg := sec36Load(t, "false")
	w := sec36Req(cfg.CORSAllowedOrigins, "http://localhost:3000")
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Empty(t, w.Header().Values("Access-Control-Expose-Headers"))
}
