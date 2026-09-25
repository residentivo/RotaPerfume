package handlers_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
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

const revokeCondSQL = `UPDATE refresh_tokens SET revoked_at = \? WHERE id = \? AND revoked_at IS NULL`

func expectUsuarioAtivo(mock sqlmock.Sqlmock, uid int64) {
	agora := time.Now()
	mock.ExpectQuery(usuarioByIDSQL).WithArgs(uid).
		WillReturnRows(sqlmock.NewRows(usuarioCols).
			AddRow(uid, "Ana", "ana@test.com", "hash", "normal", int64(2), true, false, agora, agora, nil, "Vend"))
}

func refreshCookies(resp *http.Response) (access, refresh string) {
	for _, c := range resp.Cookies() {
		switch c.Name {
		case "access_token":
			access = c.Value
		case "refresh_token":
			refresh = c.Value
		}
	}
	return access, refresh
}

// TestRefresh_Sucesso valida o fluxo completo: valida o token, busca o
// usuário, revoga o antigo de forma atômica (UPDATE condicional em tx),
// insere o novo na mesma tx e devolve access+refresh via Set-Cookie HttpOnly.
func TestRefresh_Sucesso(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	expectRefreshValido(mock, 5)
	expectUsuarioAtivo(mock, 5)
	mock.ExpectBegin()
	mock.ExpectExec(revokeCondSQL).WithArgs(sqlmock.AnyArg(), int64(10)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectCommit()

	resp := postRefresh(t, server.URL+"/api/auth/refresh", map[string]string{"refresh_token": refreshTokenTexto}, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, true, body["success"])
	data := body["data"].(map[string]any)
	assert.Equal(t, "Bearer", data["token_type"])
	assert.Equal(t, float64(24*60*60), data["expires_in"])

	for _, c := range resp.Cookies() {
		assert.True(t, c.HttpOnly, "cookie %s deve ser HttpOnly", c.Name)
	}
	access, refresh := refreshCookies(resp)
	assert.NotEmpty(t, access, "novo access_token deve vir em Set-Cookie")
	assert.NotEmpty(t, refresh)
	assert.NotEqual(t, refreshTokenTexto, refresh, "refresh token deve ser rotacionado")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRefresh_FalhasNaRotacao cobre (SEC-02) os erros depois da validação:
// token revogado por requisição concorrente (UPDATE afeta 0 linhas) → 401;
// erros de banco na revogação/begin/insert/commit → 500. Em nenhum caso um
// token é emitido (sem Set-Cookie).
func TestRefresh_FalhasNaRotacao(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(mock sqlmock.Sqlmock)
		wantStatus int
		wantErr    string
	}{
		{
			name: "revogação concorrente (0 linhas) retorna 401",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(revokeCondSQL).WithArgs(sqlmock.AnyArg(), int64(10)).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    "refresh token revogado",
		},
		{
			name: "erro de banco na revogação retorna 500",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(revokeCondSQL).WillReturnError(errors.New("falha revoke"))
				mock.ExpectRollback()
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    "erro interno",
		},
		{
			name: "erro ao abrir transação retorna 500",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("falha begin"))
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    "erro interno",
		},
		{
			name: "insert do novo refresh falha retorna 500 e desfaz a revogação",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(revokeCondSQL).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnError(errors.New("falha insert"))
				mock.ExpectRollback()
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    "erro interno",
		},
		{
			name: "commit falha retorna 500",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(revokeCondSQL).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnResult(sqlmock.NewResult(11, 1))
				mock.ExpectCommit().WillReturnError(errors.New("falha commit"))
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    "erro interno",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			expectRefreshValido(mock, 5)
			expectUsuarioAtivo(mock, 5)
			tc.setup(mock)

			resp := postRefresh(t, server.URL+"/api/auth/refresh", map[string]string{"refresh_token": refreshTokenTexto}, "")
			defer resp.Body.Close()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, false, body["success"])
			assert.Nil(t, body["data"])
			assert.Equal(t, tc.wantErr, body["error"])
			access, refresh := refreshCookies(resp)
			assert.Empty(t, access, "nenhum access_token pode ser emitido")
			assert.Empty(t, refresh, "nenhum refresh_token pode ser emitido")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRefresh_CorridaParalela (SEC-02): duas chamadas simultâneas com o mesmo
// refresh token. O UPDATE condicional só afeta uma linha para uma delas; a
// outra recebe 401 sem emitir tokens. Resultado esperado: um 200 e um 401.
func TestRefresh_CorridaParalela(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	mock.MatchExpectationsInOrder(false)

	for i := 0; i < 2; i++ {
		expectRefreshValido(mock, 5)
		expectUsuarioAtivo(mock, 5)
		mock.ExpectBegin()
	}
	mock.ExpectExec(revokeCondSQL).WithArgs(sqlmock.AnyArg(), int64(10)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(revokeCondSQL).WithArgs(sqlmock.AnyArg(), int64(10)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectCommit()
	mock.ExpectRollback()

	type resultado struct {
		status  int
		refresh string
	}
	var wg sync.WaitGroup
	resultados := make(chan resultado, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			resp := postRefresh(t, server.URL+"/api/auth/refresh", map[string]string{"refresh_token": refreshTokenTexto}, "")
			defer resp.Body.Close()
			_, refresh := refreshCookies(resp)
			resultados <- resultado{status: resp.StatusCode, refresh: refresh}
		}()
	}
	close(start)
	wg.Wait()
	close(resultados)

	got := map[int]int{}
	emitidos := 0
	for r := range resultados {
		got[r.status]++
		if r.refresh != "" {
			emitidos++
		}
	}
	assert.Equal(t, map[int]int{http.StatusOK: 1, http.StatusUnauthorized: 1}, got)
	assert.Equal(t, 1, emitidos, "só um par de tokens pode ser emitido")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRefresh_RevogacaoConcorrenteContaNoRateLimit: o 401 por revogação
// concorrente registra falha no refreshLimiter; após 10 falhas, 429.
func TestRefresh_RevogacaoConcorrenteContaNoRateLimit(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	for i := 0; i < 10; i++ {
		expectRefreshValido(mock, 5)
		expectUsuarioAtivo(mock, 5)
		mock.ExpectBegin()
		mock.ExpectExec(revokeCondSQL).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()
		resp := postRefresh(t, server.URL+"/api/auth/refresh", map[string]string{"refresh_token": refreshTokenTexto}, "")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	}

	resp := postRefresh(t, server.URL+"/api/auth/refresh", map[string]string{"refresh_token": refreshTokenTexto}, "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
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
				mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE id = \? AND revoked_at IS NULL`).
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
