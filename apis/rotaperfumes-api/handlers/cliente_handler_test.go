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

func TestListClientes_OrderBy(t *testing.T) {
	testCases := []struct {
		nome        string
		query       string
		orderRegexp string
	}{
		{"order_by e order_dir válidos", "order_by=razao_social&order_dir=desc", `ORDER BY razao_social DESC`},
		{"order_dir inválido cai no default (asc)", "order_by=cnpj&order_dir=sideways", `ORDER BY cnpj ASC`},
		{"order_by fora da whitelist cai no default", "order_by=segredo&order_dir=asc", `ORDER BY id ASC`},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes ` + tc.orderRegexp + ` LIMIT \? OFFSET \?`).
				WithArgs(20, 0).
				WillReturnRows(clienteRowsForHandler())

			req, _ := http.NewRequest("GET", server.URL+"/api/clientes?"+tc.query, nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestListClientes_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes ORDER BY id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(clienteRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
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

func TestGetCliente_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(clienteRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
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

func TestToggleAtivoCliente_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(clienteRowsForHandler())
	mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE id = \?`).
		WithArgs(false, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("PATCH", server.URL+"/api/clientes/1/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].(map[string]any)
	assert.Equal(t, false, data["ativo"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreateCliente POST /api/clientes
// ---------------------------------------------------------------------------

func validClientePayload() map[string]any {
	return map[string]any{
		"cnpj":          "12345678000199",
		"razao_social":  "Empresa Teste LTDA",
		"segmento":      "varejo",
		"cidade":        "São Paulo",
		"uf":            "SP",
		"bairro":        "Centro",
		"data_cadastro": "2024-01-15",
	}
}

func TestCreateCliente_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COALESCE\(MAX\(cliente_id_origem\), 0\) \+ 1 FROM clientes`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(101)))
	mock.ExpectExec(`INSERT INTO clientes \(cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo\)`).
		WithArgs(int64(101), "12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/clientes", makeJSON(validClientePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Empresa Teste LTDA", data["razao_social"])
	assert.Equal(t, float64(1), data["id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateCliente_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT COALESCE\(MAX\(cliente_id_origem\), 0\) \+ 1 FROM clientes`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(101)))
	mock.ExpectExec(`INSERT INTO clientes \(cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo\)`).
		WithArgs(int64(101), "12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/clientes", makeJSON(validClientePayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateCliente_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/clientes", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreateCliente_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		wantMsg string
	}{
		{
			nome: "razao_social vazia",
			payload: func() map[string]any {
				p := validClientePayload()
				p["razao_social"] = ""
				return p
			},
			wantMsg: "razão social é obrigatória",
		},
		{
			nome: "cnpj vazio",
			payload: func() map[string]any {
				p := validClientePayload()
				p["cnpj"] = ""
				return p
			},
			wantMsg: "cnpj é obrigatório",
		},
		{
			nome: "segmento vazio",
			payload: func() map[string]any {
				p := validClientePayload()
				p["segmento"] = ""
				return p
			},
			wantMsg: "segmento é obrigatório",
		},
		{
			nome: "cidade vazia",
			payload: func() map[string]any {
				p := validClientePayload()
				p["cidade"] = ""
				return p
			},
			wantMsg: "cidade é obrigatória",
		},
		{
			nome: "uf inválida",
			payload: func() map[string]any {
				p := validClientePayload()
				p["uf"] = "S"
				return p
			},
			wantMsg: "uf deve ter 2 letras",
		},
		{
			nome: "data_cadastro inválida",
			payload: func() map[string]any {
				p := validClientePayload()
				p["data_cadastro"] = "15/01/2024"
				return p
			},
			wantMsg: "data_cadastro inválida (use o formato AAAA-MM-DD)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, _ := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			req, _ := http.NewRequest("POST", server.URL+"/api/clientes", makeJSON(tc.payload()))
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
// UpdateCliente PUT /api/clientes/{id}
// ---------------------------------------------------------------------------

func TestUpdateCliente_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE clientes\s+SET cnpj = \?, razao_social = \?, segmento = \?, cidade = \?, uf = \?, bairro = \?, data_cadastro = \?\s+WHERE id = \?`).
		WithArgs("12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(clienteRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/clientes/1", makeJSON(validClientePayload()))
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

func TestUpdateCliente_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectExec(`UPDATE clientes\s+SET cnpj = \?, razao_social = \?, segmento = \?, cidade = \?, uf = \?, bairro = \?, data_cadastro = \?\s+WHERE id = \?`).
		WithArgs("12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(clienteRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/clientes/1", makeJSON(validClientePayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateCliente_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/clientes/abc", makeJSON(validClientePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdateCliente_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/clientes/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestUpdateCliente_ValidacaoNegocio(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	payload := validClientePayload()
	payload["uf"] = "SPX"

	req, _ := http.NewRequest("PUT", server.URL+"/api/clientes/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "uf deve ter 2 letras", body["error"])
}

func TestUpdateCliente_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE clientes\s+SET cnpj = \?, razao_social = \?, segmento = \?, cidade = \?, uf = \?, bairro = \?, data_cadastro = \?\s+WHERE id = \?`).
		WithArgs("12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("PUT", server.URL+"/api/clientes/999", makeJSON(validClientePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "cliente não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}
