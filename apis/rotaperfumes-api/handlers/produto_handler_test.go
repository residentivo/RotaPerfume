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

// produtoColunasRegex reflete a constante produtoColunas do repositório.
const produtoColunasRegex = `id, sku, descricao, categoria, marca, COALESCE\(nota_olfativa, ''\), preco_tabela, custo_unitario, unidade, data_lancamento, ativo, created_at, updated_at`

func produtoColunasHeaderForHandler() []string {
	return []string{
		"id", "sku", "descricao", "categoria", "marca", "nota_olfativa",
		"preco_tabela", "custo_unitario", "unidade", "data_lancamento", "ativo", "created_at", "updated_at",
	}
}

func produtoRowsForHandler() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(produtoColunasHeaderForHandler()).
		AddRow(int64(1), "SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
			99.90, 45.00, "UN", nil, true, now, now)
}

func emptyProdutoRowsForHandler() *sqlmock.Rows {
	return sqlmock.NewRows(produtoColunasHeaderForHandler())
}

// ---------------------------------------------------------------------------
// ListProdutos GET /api/produtos
// ---------------------------------------------------------------------------

func TestListProdutos_Success_Admin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos ORDER BY id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(produtoRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/produtos", nil)
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

func TestListProdutos_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/produtos", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "acesso restrito a administradores", body["error"])
}

func TestListProdutos_ComFiltros(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos WHERE categoria = \? AND marca = \? AND ativo = \? AND \(descricao LIKE \? OR sku LIKE \?\)`).
		WithArgs("Perfumaria", "Marca X", true, "%Perfume%", "%Perfume%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE categoria = \? AND marca = \? AND ativo = \? AND \(descricao LIKE \? OR sku LIKE \?\) ORDER BY id ASC LIMIT \? OFFSET \?`).
		WithArgs("Perfumaria", "Marca X", true, "%Perfume%", "%Perfume%", 20, 0).
		WillReturnRows(produtoRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/produtos?categoria=Perfumaria&marca=Marca+X&ativo=true&q=Perfume", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListProdutos_OrderBy(t *testing.T) {
	testCases := []struct {
		nome        string
		query       string
		orderRegexp string
	}{
		{"order_by e order_dir válidos", "order_by=preco_tabela&order_dir=desc", `ORDER BY preco_tabela DESC`},
		{"order_dir maiúsculo (case-insensitive)", "order_by=marca&order_dir=DESC", `ORDER BY marca DESC`},
		{"order_dir inválido cai no default", "order_by=marca&order_dir=sideways", `ORDER BY marca ASC`},
		{"order_by fora da whitelist cai no default", "order_by=preco_secreto&order_dir=asc", `ORDER BY id ASC`},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos ` + tc.orderRegexp + ` LIMIT \? OFFSET \?`).
				WithArgs(20, 0).
				WillReturnRows(produtoRowsForHandler())

			req, _ := http.NewRequest("GET", server.URL+"/api/produtos?"+tc.query, nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestListProdutos_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/produtos", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetProduto GET /api/produtos/{id}
// ---------------------------------------------------------------------------

func TestGetProduto_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(produtoRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/produtos/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Perfume Teste", data["descricao"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProduto_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyProdutoRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/produtos/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "produto não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProduto_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/produtos/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestGetProduto_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/produtos/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// ToggleAtivoProduto PATCH /api/produtos/{id}/inativar
// ---------------------------------------------------------------------------

func TestToggleAtivoProduto_Toggle_SemBody(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// produto atual está ativo=true -> toggle inverte para false
	mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(produtoRowsForHandler())
	mock.ExpectExec(`UPDATE produtos SET ativo = \? WHERE id = \?`).
		WithArgs(false, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("PATCH", server.URL+"/api/produtos/1/inativar", nil)
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

func TestToggleAtivoProduto_ComBodyExplicito(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(produtoRowsForHandler())
	mock.ExpectExec(`UPDATE produtos SET ativo = \? WHERE id = \?`).
		WithArgs(true, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("PATCH", server.URL+"/api/produtos/1/inativar", makeJSON(map[string]any{"ativo": true}))
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

func TestToggleAtivoProduto_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyProdutoRowsForHandler())

	req, _ := http.NewRequest("PATCH", server.URL+"/api/produtos/999/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestToggleAtivoProduto_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PATCH", server.URL+"/api/produtos/abc/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestToggleAtivoProduto_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PATCH", server.URL+"/api/produtos/1/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// CreateProduto POST /api/produtos
// ---------------------------------------------------------------------------

func validProdutoPayload() map[string]any {
	return map[string]any{
		"sku":             "SKU-001",
		"descricao":       "Perfume Teste",
		"categoria":       "Perfumaria",
		"marca":           "Marca X",
		"nota_olfativa":   "Cítrico",
		"preco_tabela":    99.90,
		"custo_unitario":  45.00,
		"unidade":         "UN",
		"data_lancamento": "2024-01-15",
	}
}

func TestCreateProduto_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`INSERT INTO produtos \(sku, descricao, categoria, marca, nota_olfativa, preco_tabela, custo_unitario, unidade, data_lancamento, ativo\)`).
		WithArgs("SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico", 99.90, 45.00, "UN", sqlmock.AnyArg(), true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/produtos", makeJSON(validProdutoPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Perfume Teste", data["descricao"])
	assert.Equal(t, float64(1), data["id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateProduto_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("POST", server.URL+"/api/produtos", makeJSON(validProdutoPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "acesso restrito a administradores", body["error"])
}

func TestCreateProduto_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/produtos", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreateProduto_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		wantMsg string
	}{
		{
			nome: "sku vazio",
			payload: func() map[string]any {
				p := validProdutoPayload()
				p["sku"] = ""
				return p
			},
			wantMsg: "sku é obrigatório",
		},
		{
			nome: "descricao vazia",
			payload: func() map[string]any {
				p := validProdutoPayload()
				p["descricao"] = ""
				return p
			},
			wantMsg: "descrição é obrigatória",
		},
		{
			nome: "categoria vazia",
			payload: func() map[string]any {
				p := validProdutoPayload()
				p["categoria"] = ""
				return p
			},
			wantMsg: "categoria é obrigatória",
		},
		{
			nome: "marca vazia",
			payload: func() map[string]any {
				p := validProdutoPayload()
				p["marca"] = ""
				return p
			},
			wantMsg: "marca é obrigatória",
		},
		{
			nome: "unidade vazia",
			payload: func() map[string]any {
				p := validProdutoPayload()
				p["unidade"] = ""
				return p
			},
			wantMsg: "unidade é obrigatória",
		},
		{
			nome: "preco_tabela negativo",
			payload: func() map[string]any {
				p := validProdutoPayload()
				p["preco_tabela"] = -1
				return p
			},
			wantMsg: "preco_tabela deve ser maior ou igual a zero",
		},
		{
			nome: "custo_unitario negativo",
			payload: func() map[string]any {
				p := validProdutoPayload()
				p["custo_unitario"] = -1
				return p
			},
			wantMsg: "custo_unitario deve ser maior ou igual a zero",
		},
		{
			nome: "data_lancamento inválida",
			payload: func() map[string]any {
				p := validProdutoPayload()
				p["data_lancamento"] = "15/01/2024"
				return p
			},
			wantMsg: "data_lancamento inválida (use o formato AAAA-MM-DD)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, _ := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			req, _ := http.NewRequest("POST", server.URL+"/api/produtos", makeJSON(tc.payload()))
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, tc.wantMsg, body["error"])
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateProduto PUT /api/produtos/{id}
// ---------------------------------------------------------------------------

func TestUpdateProduto_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE produtos\s+SET descricao = \?, categoria = \?, marca = \?, nota_olfativa = \?, preco_tabela = \?, custo_unitario = \?, unidade = \?, data_lancamento = \?\s+WHERE id = \?`).
		WithArgs("Perfume Teste", "Perfumaria", "Marca X", "Cítrico", 99.90, 45.00, "UN", sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(produtoRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/produtos/1", makeJSON(validProdutoPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Perfume Teste", data["descricao"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateProduto_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PUT", server.URL+"/api/produtos/1", makeJSON(validProdutoPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpdateProduto_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/produtos/abc", makeJSON(validProdutoPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdateProduto_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/produtos/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestUpdateProduto_ValidacaoNegocio(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	payload := validProdutoPayload()
	payload["unidade"] = ""

	req, _ := http.NewRequest("PUT", server.URL+"/api/produtos/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "unidade é obrigatória", body["error"])
}

func TestUpdateProduto_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE produtos\s+SET descricao = \?, categoria = \?, marca = \?, nota_olfativa = \?, preco_tabela = \?, custo_unitario = \?, unidade = \?, data_lancamento = \?\s+WHERE id = \?`).
		WithArgs("Perfume Teste", "Perfumaria", "Marca X", "Cítrico", 99.90, 45.00, "UN", sqlmock.AnyArg(), int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("PUT", server.URL+"/api/produtos/999", makeJSON(validProdutoPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "produto não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}
