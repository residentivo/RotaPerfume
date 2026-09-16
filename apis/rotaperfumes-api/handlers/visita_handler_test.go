package handlers_test

import (
	"bytes"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// visitaColunasRegexH reflete a constante visitaColunas do repositório.
const visitaColunasRegexH = `visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min, created_at, updated_at`

func visitaRowsForHandler() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"visita_id", "cliente_id", "vendedor_id", "data_visita", "resultado", "duracao_min",
		"created_at", "updated_at",
	}).AddRow(int64(1), int64(100), int64(1), now, "Positiva", 30, now, now)
}

func emptyVisitaRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"visita_id", "cliente_id", "vendedor_id", "data_visita", "resultado", "duracao_min",
		"created_at", "updated_at",
	})
}

func validVisitaPayload() map[string]any {
	return map[string]any{
		"cliente_id":  100,
		"vendedor_id": 1,
		"data_visita": "2024-01-15",
		"resultado":   "Positiva",
		"duracao_min": 30,
	}
}

// ---------------------------------------------------------------------------
// ListVisitas GET /api/visitas
// ---------------------------------------------------------------------------

func TestListVisitas_Success_Admin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas ORDER BY visita_id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas", nil)
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

func TestListVisitas_NegadoParaNaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestListVisitas_ComFiltros(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas WHERE cliente_id = \? AND vendedor_id = \? AND resultado = \?`).
		WithArgs(int64(100), int64(1), "Positiva").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE cliente_id = \? AND vendedor_id = \? AND resultado = \? ORDER BY visita_id ASC LIMIT \? OFFSET \?`).
		WithArgs(int64(100), int64(1), "Positiva", 20, 0).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas?cliente_id=100&vendedor_id=1&resultado=Positiva", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListVisitas_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListVisitas_ListaVazia(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas ORDER BY visita_id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(emptyVisitaRows())

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas", nil)
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

// ---------------------------------------------------------------------------
// GetVisita GET /api/visitas/{id}
// ---------------------------------------------------------------------------

func TestGetVisita_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Positiva", data["resultado"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVisita_NaoEncontrada(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "visita não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVisita_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestGetVisita_NegadoParaNaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestGetVisita_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreateVisita POST /api/visitas
// ---------------------------------------------------------------------------

func TestCreateVisita_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO visitas`).
		WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30).
		WillReturnResult(sqlmock.NewResult(9, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/visitas", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(9), data["visita_id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateVisita_NegadoParaNaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("POST", server.URL+"/api/visitas", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCreateVisita_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/visitas", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreateVisita_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		mock    func(mock sqlmock.Sqlmock)
		wantMsg string
	}{
		{
			nome: "cliente_id ausente",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["cliente_id"] = 0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "cliente_id é obrigatório e deve existir",
		},
		{
			nome: "vendedor_id ausente",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["vendedor_id"] = 0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "vendedor_id é obrigatório e deve existir",
		},
		{
			nome: "resultado vazio",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["resultado"] = ""
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "resultado é obrigatório",
		},
		{
			nome: "duracao_min negativa",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["duracao_min"] = -1
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "duracao_min deve ser maior ou igual a zero",
		},
		{
			nome: "data_visita ausente",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["data_visita"] = ""
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "data_visita inválida (use o formato AAAA-MM-DD)",
		},
		{
			nome: "cliente_id inexistente",
			payload: func() map[string]any {
				return validVisitaPayload()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnError(sql.ErrNoRows)
			},
			wantMsg: "cliente_id é obrigatório e deve existir",
		},
		{
			nome: "vendedor_id inexistente",
			payload: func() map[string]any {
				return validVisitaPayload()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrNoRows)
			},
			wantMsg: "vendedor_id é obrigatório e deve existir",
		},
		{
			nome: "data_visita em formato inválido",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["data_visita"] = "15/01/2024"
				return p
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			},
			wantMsg: "data_visita inválida (use o formato AAAA-MM-DD)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			tc.mock(mock)

			req, _ := http.NewRequest("POST", server.URL+"/api/visitas", makeJSON(tc.payload()))
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, tc.wantMsg, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCreateVisita_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnError(sql.ErrConnDone)

	req, _ := http.NewRequest("POST", server.URL+"/api/visitas", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UpdateVisita PUT /api/visitas/{id}
// ---------------------------------------------------------------------------

func TestUpdateVisita_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`UPDATE visitas`).
		WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Positiva", data["resultado"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateVisita_NegadoParaNaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpdateVisita_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/abc", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdateVisita_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestUpdateVisita_ValidacaoNegocio(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	payload := validVisitaPayload()
	payload["duracao_min"] = -5

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "duracao_min deve ser maior ou igual a zero", body["error"])
}

func TestUpdateVisita_NaoEncontrada(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`UPDATE visitas`).
		WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/999", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "visita não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateVisita_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`UPDATE visitas`).
		WillReturnError(sql.ErrConnDone)

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}
