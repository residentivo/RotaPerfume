// Package handlers_test contém testes de integração dos handlers HTTP.
package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const senhaHistoricoColunas = `sh\.id, sh\.usuario_id, sh\.resetado_por_id, sh\.senha_hash_anterior, sh\.ip_origem, sh\.user_agent, sh\.tipo_reset, sh\.created_at, u1\.nome AS usuario_nome, u2\.nome AS resetado_por_nome`

const senhaHistoricoJoins = `FROM senha_historico sh\s+LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id\s+LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id`

// senhaHistoricoRows monta uma linha "completa": reset feito por outro
// usuário (admin), então resetado_por_id/resetado_por_nome preenchidos.
func senhaHistoricoRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "usuario_id", "resetado_por_id", "senha_hash_anterior", "ip_origem", "user_agent", "tipo_reset", "created_at",
		"usuario_nome", "resetado_por_nome",
	}).AddRow(int64(1), int64(2), int64(1), "hash-antigo", "127.0.0.1", "go-test", "admin", now, "Usuario Teste", "Admin Teste")
}

// senhaHistoricoRowsSemResetadoPor monta uma linha em que o próprio usuário
// trocou a senha: resetado_por_id/resetado_por_nome ficam NULL.
func senhaHistoricoRowsSemResetadoPor() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "usuario_id", "resetado_por_id", "senha_hash_anterior", "ip_origem", "user_agent", "tipo_reset", "created_at",
		"usuario_nome", "resetado_por_nome",
	}).AddRow(int64(2), int64(2), nil, "hash-antigo", "127.0.0.1", "go-test", "usuario", now, "Usuario Teste", nil)
}

// ---------------------------------------------------------------------------
// ListarTodos GET /api/senha-historico
// ---------------------------------------------------------------------------

func TestListarTodos_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+senhaHistoricoColunas+`\s+`+senhaHistoricoJoins+`\s+ORDER BY sh\.id DESC\s+LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(senhaHistoricoRows())

	req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].([]any)
	require.Len(t, data, 1)
	item := data[0].(map[string]any)
	assert.Equal(t, float64(1), item["resetado_por_id"])
	assert.Equal(t, "admin", item["tipo_reset"])
	assert.Equal(t, "Usuario Teste", item["usuario_nome"], "usuario_nome deve estar sempre presente na resposta")
	assert.Equal(t, "Admin Teste", item["resetado_por_nome"], "resetado_por_nome deve estar presente quando o reset foi feito por outro usuário")
	_, hasHash := item["senha_hash_anterior"]
	assert.False(t, hasHash, "senha_hash_anterior NÃO deve ser serializado na resposta HTTP")

	pagination := body["pagination"].(map[string]any)
	assert.Equal(t, float64(1), pagination["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListarTodos_SemResetadoPor(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+senhaHistoricoColunas+`\s+`+senhaHistoricoJoins+`\s+ORDER BY sh\.id DESC\s+LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(senhaHistoricoRowsSemResetadoPor())

	req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].([]any)
	require.Len(t, data, 1)
	item := data[0].(map[string]any)
	assert.Equal(t, "Usuario Teste", item["usuario_nome"], "usuario_nome deve estar sempre presente na resposta")
	_, hasResetadoPorID := item["resetado_por_id"]
	assert.False(t, hasResetadoPorID, "resetado_por_id não deve aparecer quando o próprio usuário trocou a senha")
	_, hasResetadoPorNome := item["resetado_por_nome"]
	assert.False(t, hasResetadoPorNome, "resetado_por_nome não deve aparecer quando o próprio usuário trocou a senha")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListarTodos_OrderBy(t *testing.T) {
	testCases := []struct {
		nome        string
		query       string
		orderRegexp string
	}{
		{"order_by e order_dir válidos", "order_by=tipo_reset&order_dir=asc", `ORDER BY sh\.tipo_reset ASC`},
		{"order_dir inválido cai no default (desc)", "order_by=usuario_id&order_dir=sideways", `ORDER BY sh\.usuario_id DESC`},
		{"order_by fora da whitelist cai no default", "order_by=ip_origem&order_dir=asc", `ORDER BY sh\.id ASC`},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico`).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			mock.ExpectQuery(`SELECT `+senhaHistoricoColunas+`\s+`+senhaHistoricoJoins+`\s+`+tc.orderRegexp+`\s+LIMIT \? OFFSET \?`).
				WithArgs(20, 0).
				WillReturnRows(senhaHistoricoRows())

			req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico?"+tc.query, nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestListarTodos_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "acesso restrito a administradores", body["error"])
}

func TestListarTodos_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// ListarPorUsuario GET /api/senha-historico/{usuario_id}
// ---------------------------------------------------------------------------

func TestListarPorUsuario_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+senhaHistoricoColunas+`\s+`+senhaHistoricoJoins+`\s+WHERE sh\.usuario_id = \?\s+ORDER BY sh\.id DESC\s+LIMIT \? OFFSET \?`).
		WithArgs(int64(5), 20, 0).
		WillReturnRows(senhaHistoricoRows())

	req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico/5", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].([]any)
	require.Len(t, data, 1)
	item := data[0].(map[string]any)
	_, hasHash := item["senha_hash_anterior"]
	assert.False(t, hasHash, "senha_hash_anterior NÃO deve ser serializado na resposta HTTP")
	assert.Equal(t, "Usuario Teste", item["usuario_nome"], "usuario_nome deve estar sempre presente na resposta")
	assert.Equal(t, "Admin Teste", item["resetado_por_nome"], "resetado_por_nome deve estar presente quando o reset foi feito por outro usuário")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListarPorUsuario_SemResetadoPor(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+senhaHistoricoColunas+`\s+`+senhaHistoricoJoins+`\s+WHERE sh\.usuario_id = \?\s+ORDER BY sh\.id DESC\s+LIMIT \? OFFSET \?`).
		WithArgs(int64(5), 20, 0).
		WillReturnRows(senhaHistoricoRowsSemResetadoPor())

	req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico/5", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].([]any)
	require.Len(t, data, 1)
	item := data[0].(map[string]any)
	assert.Equal(t, "Usuario Teste", item["usuario_nome"], "usuario_nome deve estar sempre presente na resposta")
	_, hasResetadoPorNome := item["resetado_por_nome"]
	assert.False(t, hasResetadoPorNome, "resetado_por_nome não deve aparecer quando o próprio usuário trocou a senha")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListarPorUsuario_UsuarioIDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "usuario_id inválido", body["error"])
}

func TestListarPorUsuario_UsuarioIDAusente(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// Barra final sem id: usuario_id vazio.
	req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico/", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "usuario_id é obrigatório na URL", body["error"])
}

func TestListarPorUsuario_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico/5", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestListarPorUsuario_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(5)).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/senha-historico/5", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}
