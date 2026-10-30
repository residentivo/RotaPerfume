package handlers_test

// Lote 24 (TestBrain), ponta a ponta pelo router real:
//   SEC-20: com TRUST_PROXY_HEADERS=true, 6 logins errados variando o item da
//   ESQUERDA do X-Forwarded-For e fixando o da direita → 429 no 6º (a chave
//   do rate limit é o item mais à direita). Variando o da direita, nenhum
//   bloqueio (controle).
//   SEC-21: com COOKIE_SECURE=true e Host forjado (localhost:8080,
//   localhost.atacante.com, 127.0.0.1, 0.0.0.0), login, refresh, logout e
//   reset-password mandam Secure no Set-Cookie cru; logout e reset-password
//   limpam os dois cookies com os atributos da criação. Com
//   COOKIE_SECURE=false nenhum Set-Cookie leva Secure.

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/config"
)

const loginPorEmailSQL = `FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`

// sec2xServidor monta o router real com a config de teste ajustada.
func sec2xServidor(t *testing.T, ajustar func(*config.Config)) (*httptest.Server, sqlmock.Sqlmock, *config.Config) {
	t.Helper()
	cfg := testCfg()
	ajustar(cfg)
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	ident := novaIdentidadeTeste(db, cfg)
	server := httptest.NewServer(bootstrap.HandlerHTTP(cfg, fakeUserStatusChecker{}, bootstrap.NovosHandlersCom(db, cfg, ident.casos)))
	t.Cleanup(func() { server.Close(); db.Close() })
	return server, mock, cfg
}

func sec2xPost(t *testing.T, url, host, bearer string, body any, cookies ...*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, makeJSON(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if host != "" {
		req.Host = host
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// ---------------------------------------------------------------- SEC-20

func TestSEC20_HTTP_RateLimitPeloItemMaisADireita(t *testing.T) {
	casos := []struct {
		nome      string
		xff       func(i int) string
		want429No int // 0 = nenhuma tentativa bloqueada
	}{
		{"varia a esquerda, fixa a direita → 429 na 6ª", func(i int) string { return fmt.Sprintf("6.6.6.%d, 203.0.113.5", i) }, 6},
		{"fixa a esquerda, varia a direita → sem bloqueio (controle)", func(i int) string { return fmt.Sprintf("203.0.113.5, 6.6.6.%d", i) }, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, mock, _ := sec2xServidor(t, func(cfg *config.Config) { cfg.TrustProxyHeaders = true })
			for i := 1; i <= 6; i++ {
				if i != c.want429No {
					mock.ExpectQuery(loginPorEmailSQL).WithArgs("ninguem@test.com").WillReturnError(sql.ErrNoRows)
				}
				req, err := http.NewRequest(http.MethodPost, server.URL+"/api/auth/login",
					makeJSON(map[string]string{"email": "ninguem@test.com", "password": "errada", "captchaToken": "ok"}))
				require.NoError(t, err)
				req.Header.Set("X-Forwarded-For", c.xff(i))
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				readBody(t, resp)
				resp.Body.Close()
				if i == c.want429No {
					assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "tentativa %d", i)
				} else {
					assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "tentativa %d", i)
				}
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "a tentativa bloqueada não consulta o banco")
		})
	}
}

// ---------------------------------------------------------------- SEC-21

var sec21Hosts = []string{"localhost:8080", "localhost.atacante.com", "127.0.0.1", "0.0.0.0"}

// sec21SetCookies devolve o Set-Cookie cru por nome de cookie.
func sec21SetCookies(t *testing.T, resp *http.Response) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, sc := range resp.Header.Values("Set-Cookie") {
		nome, _, _ := strings.Cut(sc, "=")
		m[nome] = sc
	}
	return m
}

func sec21Attrs(sc string) map[string]bool {
	m := map[string]bool{}
	for _, p := range strings.Split(sc, ";")[1:] {
		m[strings.TrimSpace(p)] = true
	}
	return m
}

func sec21ExpectLogin(t *testing.T, mock sqlmock.Sqlmock, cfg *config.Config) {
	t.Helper()
	hash, err := gerarHashSenha(cfg.HashSenha, "senha-correta")
	require.NoError(t, err)
	mock.ExpectQuery(loginPorEmailSQL).WithArgs("admin@test.com").WillReturnRows(sqlmock.NewRows(usuarioCols).
		AddRow(int64(1), "Admin", "admin@test.com", hash, "admin", nil, true, false, time.Now(), time.Now(), nil, nil))
	mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE usuarios SET ultimo_login_at = \? WHERE id = \?`).WillReturnResult(sqlmock.NewResult(0, 1))
}

func sec21ExpectResetPassword(t *testing.T, mock sqlmock.Sqlmock, cfg *config.Config) {
	t.Helper()
	hash, err := gerarHashSenha(cfg.HashSenha, "senha-atual")
	require.NoError(t, err)
	mock.ExpectQuery(usuarioByIDSQL).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows(usuarioCols).
		AddRow(int64(2), "User 2", "user2@test.com", hash, "normal", nil, true, true, time.Now(), time.Now(), nil, nil))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`FROM senha_historico sh`).WillReturnRows(sqlmock.NewRows([]string{"id", "usuario_id", "resetado_por_id",
		"senha_hash_anterior", "ip_origem", "user_agent", "tipo_reset", "created_at", "usuario_nome", "resetado_por_nome"}))
	mock.ExpectExec(`INSERT INTO senha_historico`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \?`).WillReturnResult(sqlmock.NewResult(0, 0))
}

type sec21Fluxo struct {
	nome   string
	limpa  bool // logout e reset-password expiram os cookies
	chamar func(t *testing.T, server *httptest.Server, mock sqlmock.Sqlmock, cfg *config.Config, host string) *http.Response
}

var sec21Fluxos = []sec21Fluxo{
	{"login", false, func(t *testing.T, s *httptest.Server, mock sqlmock.Sqlmock, cfg *config.Config, host string) *http.Response {
		sec21ExpectLogin(t, mock, cfg)
		return sec2xPost(t, s.URL+"/api/auth/login", host, "",
			map[string]string{"email": "admin@test.com", "password": "senha-correta", "captchaToken": "ok"})
	}},
	{"refresh", false, func(t *testing.T, s *httptest.Server, mock sqlmock.Sqlmock, _ *config.Config, host string) *http.Response {
		expectRefreshSucesso(mock)
		return sec2xPost(t, s.URL+"/api/auth/refresh", host, "", map[string]string{},
			&http.Cookie{Name: "refresh_token", Value: refreshTokenTexto})
	}},
	{"logout", true, func(t *testing.T, s *httptest.Server, _ sqlmock.Sqlmock, _ *config.Config, host string) *http.Response {
		return sec2xPost(t, s.URL+"/api/auth/logout", host, "", map[string]string{})
	}},
	{"reset-password", true, func(t *testing.T, s *httptest.Server, mock sqlmock.Sqlmock, cfg *config.Config, host string) *http.Response {
		sec21ExpectResetPassword(t, mock, cfg)
		return sec2xPost(t, s.URL+"/api/auth/reset-password", host, generateToken(t, cfg, 2, "normal"),
			map[string]any{"senha_atual": "senha-atual", "nova_senha": "nova-senha-123", "captchaToken": "ok"})
	}},
}

func TestSEC21_HTTP_SecureIndependeDoHost(t *testing.T) {
	for _, seguro := range []bool{true, false} {
		for _, host := range sec21Hosts {
			for _, f := range sec21Fluxos {
				t.Run(fmt.Sprintf("COOKIE_SECURE=%t/%s/%s", seguro, host, f.nome), func(t *testing.T) {
					server, mock, cfg := sec2xServidor(t, func(c *config.Config) { c.CookieSecure = seguro })
					resp := f.chamar(t, server, mock, cfg, host)
					raw := readBody(t, resp)
					require.Equal(t, http.StatusOK, resp.StatusCode, "%s", raw)

					crus := sec21SetCookies(t, resp)
					require.Contains(t, crus, "access_token")
					require.Contains(t, crus, "refresh_token")
					for nome, sameSite := range map[string]string{"access_token": "SameSite=Strict", "refresh_token": "SameSite=Lax"} {
						a := sec21Attrs(crus[nome])
						assert.Equal(t, seguro, a["Secure"], "Secure segue COOKIE_SECURE, nunca o Host: %s", crus[nome])
						assert.True(t, a["HttpOnly"], crus[nome])
						assert.True(t, a["Path=/"], crus[nome])
						assert.True(t, a[sameSite], crus[nome])
						if f.limpa {
							assert.True(t, strings.HasPrefix(crus[nome], nome+"=;"), "valor vazio: %s", crus[nome])
							assert.True(t, a["Max-Age=0"], crus[nome])
							assert.True(t, a["Expires=Thu, 01 Jan 1970 00:00:00 GMT"], crus[nome])
						} else {
							assert.False(t, strings.HasPrefix(crus[nome], nome+"=;"), "token emitido: %s", crus[nome])
						}
					}
					assert.NoError(t, mock.ExpectationsWereMet())
				})
			}
		}
	}
}
