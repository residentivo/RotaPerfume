// Package handlers_test contém testes de integração dos handlers HTTP.
package handlers_test

import (
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// GetMetrics GET /api/dashboard/metrics
// ---------------------------------------------------------------------------

// mockDashboardMetricsQueries prepara os 4 mocks consumidos em sequência por
// GetMetrics (vendas totais, total de pedidos, top vendedores, metas).
// Retorna listas vazias para top_vendedores/metas_vendedores para evitar as
// queries adicionais de enriquecimento (só disparadas quando há resultados).
func mockDashboardMetricsQueries(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
		WillReturnRows(sqlmock.NewRows([]string{"valor", "quantidade"}).AddRow(1000.0, 5))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos\s+WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(5))
	mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}))
	mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.regiao,\s+v\.uf,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}))
}

func TestGetMetrics_Success_Month(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mockDashboardMetricsQueries(mock)

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "month", data["periodo"])
	assert.Equal(t, float64(1000), data["total_vendas_valor"])
	assert.Equal(t, float64(5), data["total_pedidos"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMetrics_Success_Today(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mockDashboardMetricsQueries(mock)

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/metrics?periodo=today", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].(map[string]any)
	assert.Equal(t, "today", data["periodo"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMetrics_PeriodoInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/metrics?periodo=ano", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "periodo deve ser 'today' ou 'month'", body["error"])
}

func TestGetMetrics_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mockDashboardMetricsQueries(mock)

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMetrics_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetVendas GET /api/dashboard/vendas
// ---------------------------------------------------------------------------

func TestGetVendas_Success_DiasDefault(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT DATE\(data_pedido\) AS data,\s+COALESCE\(SUM\(valor_total\), 0\) AS valor,\s+COUNT\(\*\) AS quantidade\s+FROM pedidos`).
		WithArgs(30).
		WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/vendas", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].([]any)
	assert.Len(t, data, 30)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVendas_ComDiasCustom(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT DATE\(data_pedido\) AS data,\s+COALESCE\(SUM\(valor_total\), 0\) AS valor,\s+COUNT\(\*\) AS quantidade\s+FROM pedidos`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/vendas?dias=7", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].([]any)
	assert.Len(t, data, 7)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVendas_DiasForaDoIntervalo_UsaDefault(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// dias=9999 é > 365, então o handler ignora e usa o default (30).
	mock.ExpectQuery(`SELECT DATE\(data_pedido\) AS data,\s+COALESCE\(SUM\(valor_total\), 0\) AS valor,\s+COUNT\(\*\) AS quantidade\s+FROM pedidos`).
		WithArgs(30).
		WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/vendas?dias=9999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVendas_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT DATE\(data_pedido\) AS data,\s+COALESCE\(SUM\(valor_total\), 0\) AS valor,\s+COUNT\(\*\) AS quantidade\s+FROM pedidos`).
		WithArgs(30).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/vendas", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVendas_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT DATE\(data_pedido\) AS data,\s+COALESCE\(SUM\(valor_total\), 0\) AS valor,\s+COUNT\(\*\) AS quantidade\s+FROM pedidos`).
		WithArgs(30).
		WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/vendas", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].([]any)
	assert.Len(t, data, 30)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetVendedores GET /api/dashboard/vendedores
// ---------------------------------------------------------------------------

func TestGetVendedores_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT id, nome, regiao, uf, meta_mensal\s+FROM vendedores\s+WHERE data_desligamento IS NULL\s+ORDER BY meta_mensal DESC\s+LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal"}))

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	pagination := body["pagination"].(map[string]any)
	assert.Equal(t, float64(0), pagination["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVendedores_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVendedores_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT id, nome, regiao, uf, meta_mensal\s+FROM vendedores\s+WHERE data_desligamento IS NULL\s+ORDER BY meta_mensal DESC\s+LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal"}))

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	pagination := body["pagination"].(map[string]any)
	assert.Equal(t, float64(0), pagination["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetClientes GET /api/dashboard/clientes
// ---------------------------------------------------------------------------

func mockDashboardClientesQueries(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \?`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(80))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \?`).
		WithArgs(false).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(20))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	mock.ExpectQuery(`SELECT segmento, COUNT\(\*\) AS total\s+FROM clientes\s+GROUP BY segmento`).
		WillReturnRows(sqlmock.NewRows([]string{"segmento", "total"}).AddRow("varejo", 60))
	mock.ExpectQuery(`SELECT uf, COUNT\(\*\) AS total\s+FROM clientes\s+GROUP BY uf`).
		WillReturnRows(sqlmock.NewRows([]string{"uf", "total"}).AddRow("SP", 40))
}

func TestGetClientes_Success_Month(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mockDashboardClientesQueries(mock)

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].(map[string]any)
	assert.Equal(t, "month", data["periodo"])
	assert.Equal(t, float64(100), data["total_clientes"])
	assert.Equal(t, float64(80), data["total_ativos"])
	assert.Equal(t, float64(20), data["total_inativos"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetClientes_PeriodoInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/clientes?periodo=ano", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "periodo deve ser 'today' ou 'month'", body["error"])
}

func TestGetClientes_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetClientes_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mockDashboardClientesQueries(mock)

	req, _ := http.NewRequest("GET", server.URL+"/api/dashboard/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].(map[string]any)
	assert.Equal(t, "month", data["periodo"])

	assert.NoError(t, mock.ExpectationsWereMet())
}
