package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clienteColunasRegex reflete a constante clienteColunas do repositório.
const clienteColunasRegex = `id, cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, COALESCE\(bairro, ''\), data_cadastro, ativo, created_at, updated_at`

func clienteRowsForHandler() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
		"data_cadastro", "ativo", "created_at", "updated_at",
	}).AddRow(int64(1), int64(100), "12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro",
		now, true, now, now)
}

func emptyClienteRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
		"data_cadastro", "ativo", "created_at", "updated_at",
	})
}

// ---------------------------------------------------------------------------
// ListClientes GET /api/clientes
// ---------------------------------------------------------------------------

func TestListClientes_Success_Admin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes ORDER BY id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(clienteRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].([]any)
	assert.Len(t, data, 1)
	pagination := body["pagination"].(map[string]any)
	assert.Equal(t, float64(1), pagination["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListClientes_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "acesso restrito a administradores", body["error"])
}

func TestListClientes_ComFiltros(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE uf = \? AND segmento = \? AND ativo = \? AND \(razao_social LIKE \? OR cnpj LIKE \?\)`).
		WithArgs("SP", "varejo", true, "%Empresa%", "%Empresa%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE uf = \? AND segmento = \? AND ativo = \? AND \(razao_social LIKE \? OR cnpj LIKE \?\) ORDER BY id ASC LIMIT \? OFFSET \?`).
		WithArgs("SP", "varejo", true, "%Empresa%", "%Empresa%", 20, 0).
		WillReturnRows(clienteRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes?uf=SP&segmento=varejo&ativo=true&q=Empresa", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListClientes_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetCliente GET /api/clientes/{id}
// ---------------------------------------------------------------------------

func TestGetCliente_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(clienteRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Empresa Teste LTDA", data["razao_social"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCliente_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyClienteRows())

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "cliente não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCliente_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestGetCliente_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// ToggleAtivoCliente PATCH /api/clientes/{id}/inativar
// ---------------------------------------------------------------------------

func TestToggleAtivoCliente_Toggle_SemBody(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// cliente atual está ativo=true -> toggle inverte para false
	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(clienteRowsForHandler())
	mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE id = \?`).
		WithArgs(false, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("PATCH", server.URL+"/api/clientes/1/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].(map[string]any)
	assert.Equal(t, false, data["ativo"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestToggleAtivoCliente_ComBodyExplicito(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(clienteRowsForHandler())
	mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE id = \?`).
		WithArgs(true, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("PATCH", server.URL+"/api/clientes/1/inativar", makeJSON(map[string]any{"ativo": true}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].(map[string]any)
	assert.Equal(t, true, data["ativo"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestToggleAtivoCliente_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyClienteRows())

	req, _ := http.NewRequest("PATCH", server.URL+"/api/clientes/999/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestToggleAtivoCliente_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PATCH", server.URL+"/api/clientes/abc/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestToggleAtivoCliente_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PATCH", server.URL+"/api/clientes/1/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
