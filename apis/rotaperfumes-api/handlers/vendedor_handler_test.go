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
// ListVendedores GET /api/vendedores
// ---------------------------------------------------------------------------

func TestListVendedores_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT id, nome, regiao, uf\s+FROM vendedores\s+WHERE data_desligamento IS NULL\s+ORDER BY nome ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf"}).
			AddRow(int64(1), "Vendedor Um", "Sudeste", "SP").
			AddRow(int64(2), "Vendedor Dois", "Sul", "PR"))

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].([]any)
	require.Len(t, data, 2)
	first := data[0].(map[string]any)
	assert.Equal(t, "Vendedor Um", first["nome"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListVendedores_ListaVazia(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT id, nome, regiao, uf\s+FROM vendedores\s+WHERE data_desligamento IS NULL\s+ORDER BY nome ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf"}))

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	// data pode ser null (slice nula) quando não há resultados.
	assert.True(t, body["data"] == nil || len(body["data"].([]any)) == 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListVendedores_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT id, nome, regiao, uf\s+FROM vendedores\s+WHERE data_desligamento IS NULL\s+ORDER BY nome ASC`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListVendedores_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "acesso restrito a administradores", body["error"])
}

func TestListVendedores_NaoAutenticado(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
