package handlers_test

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// estoqueColunasRegexH reflete a constante estoqueColunas do repositório.
const estoqueColunasRegexH = `id, data_snapshot, sku, saldo, ruptura, origem, created_at, updated_at`

func estoqueColunasHeaderForHandler() []string {
	return []string{"id", "data_snapshot", "sku", "saldo", "ruptura", "origem", "created_at", "updated_at"}
}

func estoqueRowsForHandler(id int64, saldo int, ruptura bool, origem string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(estoqueColunasHeaderForHandler()).
		AddRow(id, now, "SKU-001", saldo, ruptura, origem, now, now)
}

func emptyEstoqueRowsForHandler() *sqlmock.Rows {
	return sqlmock.NewRows(estoqueColunasHeaderForHandler())
}

// ---------------------------------------------------------------------------
// ListEstoque GET /api/estoque
// ---------------------------------------------------------------------------

func TestListEstoque_Success_Admin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM \(.+\) ranked WHERE rn = 1`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT .+ FROM \(.+\) ranked WHERE rn = 1 ORDER BY data_snapshot DESC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(estoqueRowsForHandler(1, 10, false, "import_csv"))

	req, _ := http.NewRequest("GET", server.URL+"/api/estoque", nil)
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

// TestListEstoque_PermitidoParaNaoAdmin cobre que GET /api/estoque é acesso
// comum (não exige role admin), diferente de POST/PUT.
func TestListEstoque_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM \(.+\) ranked WHERE rn = 1`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT .+ FROM \(.+\) ranked WHERE rn = 1 ORDER BY data_snapshot DESC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(estoqueRowsForHandler(1, 10, false, "import_csv"))

	req, _ := http.NewRequest("GET", server.URL+"/api/estoque", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListEstoque_Historico(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM estoque WHERE sku = \?`).
		WithArgs("SKU-001").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(`SELECT `+estoqueColunasRegexH+` FROM estoque WHERE sku = \? ORDER BY data_snapshot DESC LIMIT \? OFFSET \?`).
		WithArgs("SKU-001", 20, 0).
		WillReturnRows(estoqueRowsForHandler(1, 10, false, "import_csv"))

	req, _ := http.NewRequest("GET", server.URL+"/api/estoque?sku=SKU-001&historico=true", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListEstoque_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM \(.+\) ranked WHERE rn = 1`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/estoque", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetEstoque GET /api/estoque/{id}
// ---------------------------------------------------------------------------

func TestGetEstoque_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + estoqueColunasRegexH + ` FROM estoque WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRowsForHandler(1, 10, false, "import_csv"))

	req, _ := http.NewRequest("GET", server.URL+"/api/estoque/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "SKU-001", data["sku"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetEstoque_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT ` + estoqueColunasRegexH + ` FROM estoque WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRowsForHandler(1, 10, false, "import_csv"))

	req, _ := http.NewRequest("GET", server.URL+"/api/estoque/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetEstoque_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + estoqueColunasRegexH + ` FROM estoque WHERE id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyEstoqueRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/estoque/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "registro de estoque não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetEstoque_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/estoque/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

// ---------------------------------------------------------------------------
// CreateEstoque POST /api/estoque (admin only)
// ---------------------------------------------------------------------------

func validEstoquePayload() map[string]any {
	return map[string]any{
		"sku":           "SKU-001",
		"data_snapshot": "2024-06-01",
		"saldo":         10,
	}
}

func TestCreateEstoque_Success_Admin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`).
		WithArgs("SKU-001").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO estoque`).
		WithArgs("2024-06-01", "SKU-001", 10, false, "manual").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/estoque", makeJSON(validEstoquePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "SKU-001", data["sku"])
	assert.Equal(t, false, data["ruptura"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateEstoque_NegadoParaNaoAdmin cobre a exigência de acesso admin-only
// para POST /api/estoque.
func TestCreateEstoque_NegadoParaNaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("POST", server.URL+"/api/estoque", makeJSON(validEstoquePayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCreateEstoque_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/estoque", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreateEstoque_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		mock    func(mock sqlmock.Sqlmock)
		wantMsg string
	}{
		{
			nome: "sku vazio",
			payload: func() map[string]any {
				p := validEstoquePayload()
				p["sku"] = ""
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "sku é obrigatório",
		},
		{
			nome: "data_snapshot vazia",
			payload: func() map[string]any {
				p := validEstoquePayload()
				p["data_snapshot"] = ""
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "data_snapshot é obrigatória",
		},
		{
			nome: "data_snapshot em formato inválido",
			payload: func() map[string]any {
				p := validEstoquePayload()
				p["data_snapshot"] = "01/06/2024"
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "data_snapshot inválida (use o formato AAAA-MM-DD)",
		},
		{
			nome: "data_snapshot futura",
			payload: func() map[string]any {
				p := validEstoquePayload()
				p["data_snapshot"] = time.Now().AddDate(0, 0, 5).Format("2006-01-02")
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "data_snapshot não pode ser uma data futura",
		},
		{
			nome: "saldo negativo",
			payload: func() map[string]any {
				p := validEstoquePayload()
				p["saldo"] = -1
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "saldo deve ser maior ou igual a zero",
		},
		{
			nome:    "sku inexistente em produtos",
			payload: validEstoquePayload,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`).
					WithArgs("SKU-001").
					WillReturnRows(sqlmock.NewRows([]string{"1"}))
			},
			wantMsg: "sku não corresponde a um produto cadastrado",
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

			req, _ := http.NewRequest("POST", server.URL+"/api/estoque", makeJSON(tc.payload()))
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
// UpdateEstoque PUT /api/estoque/{id} (admin only)
// ---------------------------------------------------------------------------

func TestUpdateEstoque_Success_Admin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + estoqueColunasRegexH + ` FROM estoque WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRowsForHandler(1, 10, false, "manual"))
	mock.ExpectExec(`UPDATE estoque`).
		WithArgs(25, false, "manual", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + estoqueColunasRegexH + ` FROM estoque WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRowsForHandler(1, 25, false, "manual"))

	req, _ := http.NewRequest("PUT", server.URL+"/api/estoque/1", makeJSON(map[string]any{"saldo": 25}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(25), data["saldo"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateEstoque_NegadoParaNaoAdmin cobre a exigência de acesso admin-only
// para PUT /api/estoque/{id}.
func TestUpdateEstoque_NegadoParaNaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PUT", server.URL+"/api/estoque/1", makeJSON(map[string]any{"saldo": 25}))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpdateEstoque_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/estoque/abc", makeJSON(map[string]any{"saldo": 25}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdateEstoque_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/estoque/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestUpdateEstoque_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + estoqueColunasRegexH + ` FROM estoque WHERE id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyEstoqueRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/estoque/999", makeJSON(map[string]any{"saldo": 25}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "registro de estoque não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateEstoque_SaldoNegativo(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + estoqueColunasRegexH + ` FROM estoque WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRowsForHandler(1, 10, false, "manual"))

	req, _ := http.NewRequest("PUT", server.URL+"/api/estoque/1", makeJSON(map[string]any{"saldo": -5}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "saldo deve ser maior ou igual a zero", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}
