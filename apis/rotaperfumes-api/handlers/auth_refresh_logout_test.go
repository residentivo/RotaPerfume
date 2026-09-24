package handlers_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Testes de POST /api/auth/refresh, POST /api/auth/logout e dos caminhos de
// erro de GET /api/auth/me. O DB é mockado via sqlmock; os tokens em texto
// puro são convertidos para o hash SHA-256 que o RefreshTokenService usa
// para buscar em refresh_tokens.

const refreshTokenTexto = "refresh-token-de-teste"

func hashRefresh(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

var refreshTokenCols = []string{"id", "usuario_id", "token_hash", "expires_at", "revoked_at", "ip_origem", "user_agent"}

var usuarioCols = []string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}

const findRefreshSQL = `SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent\s+FROM refresh_tokens\s+WHERE token_hash = \?`
const usuarioByIDSQL = `FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.id\s+=\s+\?\s+LIMIT\s+1`

func expectRefreshValido(mock sqlmock.Sqlmock, uid int64) {
	mock.ExpectQuery(findRefreshSQL).
		WithArgs(hashRefresh(refreshTokenTexto)).
		WillReturnRows(sqlmock.NewRows(refreshTokenCols).
			AddRow(int64(10), uid, hashRefresh(refreshTokenTexto), time.Now().Add(time.Hour), nil, "127.0.0.1", "go-test"))
}

func postRefresh(t *testing.T, url string, body any, cookie string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, makeJSON(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: cookie})
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// TestRefresh_Cenarios cobre os caminhos de erro de validação do refresh
// token (ausente, não encontrado, expirado, revogado, erro de banco).
func TestRefresh_Cenarios(t *testing.T) {
	agora := time.Now()
	cases := []struct {
		name       string
		body       any
		cookie     string
		setup      func(mock sqlmock.Sqlmock)
		wantStatus int
		wantErr    string
	}{
		{
			name:       "sem token no body nem cookie",
			body:       map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantErr:    "refresh_token é obrigatório (body ou cookie)",
		},
		{
			name: "token não encontrado",
			body: map[string]string{"refresh_token": refreshTokenTexto},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(findRefreshSQL).WillReturnRows(sqlmock.NewRows(refreshTokenCols))
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    "refresh token inválido",
		},
		{
			name:   "token expirado (via cookie)",
			body:   map[string]string{},
			cookie: refreshTokenTexto,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(findRefreshSQL).WithArgs(hashRefresh(refreshTokenTexto)).
					WillReturnRows(sqlmock.NewRows(refreshTokenCols).
						AddRow(int64(1), int64(3), "h", agora.Add(-time.Hour), nil, "ip", "ua"))
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    "refresh token expirado",
		},
		{
			name: "token revogado",
			body: map[string]string{"refresh_token": refreshTokenTexto},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(findRefreshSQL).
					WillReturnRows(sqlmock.NewRows(refreshTokenCols).
						AddRow(int64(1), int64(3), "h", agora.Add(time.Hour), agora.Add(-time.Minute), "ip", "ua"))
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    "refresh token revogado",
		},
		{
			name: "erro de banco ao validar",
			body: map[string]string{"refresh_token": refreshTokenTexto},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(findRefreshSQL).WillReturnError(errors.New("db down"))
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    "erro interno",
		},
		{
			name: "erro ao buscar usuário",
			body: map[string]string{"refresh_token": refreshTokenTexto},
			setup: func(mock sqlmock.Sqlmock) {
				expectRefreshValido(mock, 3)
				mock.ExpectQuery(usuarioByIDSQL).WithArgs(int64(3)).WillReturnError(errors.New("db down"))
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    "erro interno",
		},
		{
			name: "usuário inativo",
			body: map[string]string{"refresh_token": refreshTokenTexto},
			setup: func(mock sqlmock.Sqlmock) {
				expectRefreshValido(mock, 3)
				mock.ExpectQuery(usuarioByIDSQL).WithArgs(int64(3)).
					WillReturnRows(sqlmock.NewRows(usuarioCols).
						AddRow(int64(3), "Inativo", "i@test.com", "hash", "normal", nil, false, false, agora, agora, nil, nil))
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    "usuário inativo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			if tc.setup != nil {
				tc.setup(mock)
			}

			resp := postRefresh(t, server.URL+"/api/auth/refresh", tc.body, tc.cookie)
			defer resp.Body.Close()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, tc.wantErr, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRefresh_Sucesso valida o fluxo completo: valida o token, busca o
// usuário, revoga o antigo (single-use), gera novo access+refresh via
// Set-Cookie HttpOnly. Parametrizado para os casos em que a revogação ou a
// geração do novo refresh token falham (o handler só loga, não falha).
func TestRefresh_Sucesso(t *testing.T) {
	cases := []struct {
		name              string
		revokeErr         error
		insertErr         error
		wantRefreshCookie bool
	}{
		{name: "tudo ok", wantRefreshCookie: true},
		{name: "revogação falha mas segue", revokeErr: errors.New("falha revoke"), wantRefreshCookie: true},
		{name: "insert do novo refresh falha, sem cookie de refresh", insertErr: errors.New("falha insert"), wantRefreshCookie: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			agora := time.Now()
			expectRefreshValido(mock, 5)
			mock.ExpectQuery(usuarioByIDSQL).WithArgs(int64(5)).
				WillReturnRows(sqlmock.NewRows(usuarioCols).
					AddRow(int64(5), "Ana", "ana@test.com", "hash", "normal", int64(2), true, false, agora, agora, nil, "Vend"))
			// RevokeToken: busca pelo hash e faz UPDATE.
			expectRefreshValido(mock, 5)
			if tc.revokeErr != nil {
				mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE id = \?`).WillReturnError(tc.revokeErr)
			} else {
				mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE id = \?`).
					WithArgs(sqlmock.AnyArg(), int64(10)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if tc.insertErr != nil {
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnError(tc.insertErr)
			} else {
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnResult(sqlmock.NewResult(11, 1))
			}

			resp := postRefresh(t, server.URL+"/api/auth/refresh", map[string]string{"refresh_token": refreshTokenTexto}, "")
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, true, body["success"])
			data := body["data"].(map[string]any)
			assert.Equal(t, "Bearer", data["token_type"])
			assert.Equal(t, float64(24*60*60), data["expires_in"])

			var access, refresh string
			for _, c := range resp.Cookies() {
				switch c.Name {
				case "access_token":
					access = c.Value
					assert.True(t, c.HttpOnly)
				case "refresh_token":
					refresh = c.Value
					assert.True(t, c.HttpOnly)
				}
			}
			assert.NotEmpty(t, access, "novo access_token deve vir em Set-Cookie")
			if tc.wantRefreshCookie {
				assert.NotEmpty(t, refresh)
				assert.NotEqual(t, refreshTokenTexto, refresh, "refresh token deve ser rotacionado")
			} else {
				assert.Empty(t, refresh)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRefresh_RateLimitPorIP garante que, após refreshMaxFailures (10)
// falhas do mesmo IP, a próxima tentativa recebe 429 sem consultar o banco.
func TestRefresh_RateLimitPorIP(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	for i := 0; i < 10; i++ {
		mock.ExpectQuery(findRefreshSQL).WillReturnRows(sqlmock.NewRows(refreshTokenCols))
		resp := postRefresh(t, server.URL+"/api/auth/refresh", map[string]string{"refresh_token": "forjado"}, "")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	}

	resp := postRefresh(t, server.URL+"/api/auth/refresh", map[string]string{"refresh_token": "forjado"}, "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestLogout_Cenarios cobre logout com token no body, no cookie, sem token,
// token inexistente e erro de banco — todos retornam 200 e limpam cookies.
func TestLogout_Cenarios(t *testing.T) {
	cases := []struct {
		name   string
		body   any
		cookie string
		setup  func(mock sqlmock.Sqlmock)
	}{
		{name: "sem token", body: map[string]string{}},
		{
			name: "token no body revogado",
			body: map[string]string{"refresh_token": refreshTokenTexto},
			setup: func(mock sqlmock.Sqlmock) {
				expectRefreshValido(mock, 1)
				mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE id = \?`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:   "token no cookie inexistente",
			body:   map[string]string{},
			cookie: refreshTokenTexto,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(findRefreshSQL).WithArgs(hashRefresh(refreshTokenTexto)).
					WillReturnRows(sqlmock.NewRows(refreshTokenCols))
			},
		},
		{
			name: "erro de banco ao revogar",
			body: map[string]string{"refresh_token": refreshTokenTexto},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(findRefreshSQL).WillReturnError(errors.New("db down"))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			if tc.setup != nil {
				tc.setup(mock)
			}

			resp := postRefresh(t, server.URL+"/api/auth/logout", tc.body, tc.cookie)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, "logout realizado", body["data"].(map[string]any)["mensagem"])

			cleared := map[string]bool{}
			for _, c := range resp.Cookies() {
				if c.MaxAge < 0 {
					cleared[c.Name] = true
				}
			}
			assert.True(t, cleared["access_token"], "access_token deve ser expirado")
			assert.True(t, cleared["refresh_token"], "refresh_token deve ser expirado")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestMe_Erros cobre 404 (usuário do token não existe mais) e 500.
func TestMe_Erros(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(mock sqlmock.Sqlmock)
		wantStatus int
	}{
		{
			name: "usuário não encontrado",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(usuarioByIDSQL).WithArgs(int64(77)).WillReturnRows(sqlmock.NewRows(usuarioCols))
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "erro de banco",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(usuarioByIDSQL).WithArgs(int64(77)).WillReturnError(errors.New("db down"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			tc.setup(mock)

			req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/auth/me", nil)
			req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 77, "normal"))
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
