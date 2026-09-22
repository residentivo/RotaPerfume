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

// oportunidadeColunasRegexH reflete a constante oportunidadeColunas do repositório.
const oportunidadeColunasRegexH = `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda, created_at, updated_at`

func oportunidadeRowsForHandler() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"oportunidade_id", "cliente_id", "vendedor_id", "origem", "data_abertura", "etapa",
		"probabilidade_pct", "valor_estimado", "data_fechamento", "ciclo_dias", "motivo_perda",
		"created_at", "updated_at",
	}).AddRow(int64(1), int64(100), int64(1), "Site", now, "Prospeccao", 10.0, 1000.0, nil, nil, nil, now, now)
}

func emptyOportunidadeRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"oportunidade_id", "cliente_id", "vendedor_id", "origem", "data_abertura", "etapa",
		"probabilidade_pct", "valor_estimado", "data_fechamento", "ciclo_dias", "motivo_perda",
		"created_at", "updated_at",
	})
}

func validOportunidadePayload() map[string]any {
	return map[string]any{
		"cliente_id":        100,
		"vendedor_id":       1,
		"origem":            "Site",
		"data_abertura":     "2024-01-15",
		"etapa":             "Prospeccao",
		"probabilidade_pct": 10.0,
		"valor_estimado":    1000.0,
	}
}

// ---------------------------------------------------------------------------
// ListOportunidades GET /api/oportunidades
// ---------------------------------------------------------------------------

func TestListOportunidades_Success_Admin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+oportunidadeColunasRegexH+` FROM oportunidades ORDER BY oportunidade_id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades", nil)
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

func TestListOportunidades_NegadoParaNaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestListOportunidades_ComFiltros(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades WHERE cliente_id = \? AND vendedor_id = \? AND etapa = \?`).
		WithArgs(int64(100), int64(1), "Prospeccao").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+oportunidadeColunasRegexH+` FROM oportunidades WHERE cliente_id = \? AND vendedor_id = \? AND etapa = \? ORDER BY oportunidade_id ASC LIMIT \? OFFSET \?`).
		WithArgs(int64(100), int64(1), "Prospeccao", 20, 0).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades?cliente_id=100&vendedor_id=1&etapa=Prospeccao", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListOportunidades_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetOportunidade GET /api/oportunidades/{id}
// ---------------------------------------------------------------------------

func TestGetOportunidade_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Prospeccao", data["etapa"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOportunidade_NaoEncontrada(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "oportunidade não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOportunidade_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestGetOportunidade_NegadoParaNaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// CreateOportunidade POST /api/oportunidades
// ---------------------------------------------------------------------------

func TestCreateOportunidade_Success(t *testing.T) {
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
	mock.ExpectExec(`INSERT INTO oportunidades`).
		WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(9, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/oportunidades", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(9), data["oportunidade_id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOportunidade_NegadoParaNaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("POST", server.URL+"/api/oportunidades", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCreateOportunidade_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/oportunidades", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreateOportunidade_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		mock    func(mock sqlmock.Sqlmock)
		wantMsg string
	}{
		{
			nome: "cliente_id ausente",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["cliente_id"] = 0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "cliente_id é obrigatório e deve existir",
		},
		{
			nome: "vendedor_id ausente",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["vendedor_id"] = 0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "vendedor_id é obrigatório e deve existir",
		},
		{
			nome: "origem vazia",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["origem"] = ""
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "origem é obrigatória",
		},
		{
			nome: "etapa vazia",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["etapa"] = ""
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "etapa é obrigatória",
		},
		{
			nome: "probabilidade fora do range",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["probabilidade_pct"] = 150.0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "probabilidade_pct deve estar entre 0 e 100",
		},
		{
			nome: "valor_estimado negativo",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["valor_estimado"] = -1.0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "valor_estimado deve ser maior ou igual a zero",
		},
		{
			nome: "etapa Fechado perdido sem motivo_perda",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["etapa"] = "Fechado perdido"
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "motivo_perda é obrigatório quando etapa = \"Fechado perdido\"",
		},
		{
			nome: "cliente_id inexistente",
			payload: func() map[string]any {
				return validOportunidadePayload()
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
				return validOportunidadePayload()
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
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			tc.mock(mock)

			req, _ := http.NewRequest("POST", server.URL+"/api/oportunidades", makeJSON(tc.payload()))
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

// ---------------------------------------------------------------------------
// UpdateOportunidade PUT /api/oportunidades/{id}
// ---------------------------------------------------------------------------

func TestUpdateOportunidade_Success(t *testing.T) {
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
	mock.ExpectExec(`UPDATE oportunidades`).
		WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/1", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Prospeccao", data["etapa"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateOportunidade_NegadoParaNaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/1", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpdateOportunidade_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/abc", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdateOportunidade_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestUpdateOportunidade_ValidacaoNegocio(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	payload := validOportunidadePayload()
	payload["probabilidade_pct"] = -5.0

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "probabilidade_pct deve estar entre 0 e 100", body["error"])
}

func TestUpdateOportunidade_NaoEncontrada(t *testing.T) {
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
	mock.ExpectExec(`UPDATE oportunidades`).
		WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/999", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "oportunidade não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListOportunidades_ListaVazia(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT `+oportunidadeColunasRegexH+` FROM oportunidades ORDER BY oportunidade_id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(emptyOportunidadeRows())

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades", nil)
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
