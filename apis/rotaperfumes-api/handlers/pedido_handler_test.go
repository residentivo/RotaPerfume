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

// Regexes/colunas que espelham as constantes de apis/shared/repositories/pedido_repository.go.
const pedidoColunasRegexH = `p\.id, p\.pedido_id_origem, p\.cliente_id, p\.vendedor_id, p\.data_pedido, p\.canal, p\.status, p\.valor_total, p\.created_at, p\.updated_at, c\.razao_social, v\.nome`
const pedidoFromRegexH = ` FROM pedidos p JOIN clientes c ON c\.id = p\.cliente_id JOIN vendedores v ON v\.id = p\.vendedor_id`
const itemPedidoColunasRegexH = `i\.id, i\.item_id_origem, i\.pedido_id, i\.produto_id, i\.quantidade, i\.preco_praticado, i\.desconto_pct, i\.valor_bruto, i\.created_at, i\.updated_at, pr\.sku, pr\.descricao`
const itemPedidoFromRegexH = ` FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id`

func pedidoColunasHeaderForHandler() []string {
	return []string{
		"id", "pedido_id_origem", "cliente_id", "vendedor_id", "data_pedido",
		"canal", "status", "valor_total", "created_at", "updated_at", "cliente_nome", "vendedor_nome",
	}
}

func pedidoRowsForHandler(id int64, valorTotal float64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(pedidoColunasHeaderForHandler()).
		AddRow(id, int64(100), int64(1), int64(2), now, "App", "Faturado", valorTotal, now, now, "Cliente Teste", "Vendedor Teste")
}

func emptyPedidoRowsForHandler() *sqlmock.Rows {
	return sqlmock.NewRows(pedidoColunasHeaderForHandler())
}

func itemPedidoColunasHeaderForHandler() []string {
	return []string{
		"id", "item_id_origem", "pedido_id", "produto_id", "quantidade",
		"preco_praticado", "desconto_pct", "valor_bruto", "created_at", "updated_at", "produto_sku", "produto_descricao",
	}
}

func itemPedidoRowsForHandler() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(itemPedidoColunasHeaderForHandler()).
		AddRow(int64(1), int64(1), int64(1), int64(10), 2, 100.0, 10.0, 180.0, now, now, "SKU-010", "Produto A").
		AddRow(int64(2), int64(2), int64(1), int64(11), 1, 50.0, 0.0, 50.0, now, now, "SKU-011", "Produto B")
}

// validPedidoPayload retorna um payload válido com dois itens:
// item 1: 2 * 100.0 * (1 - 10/100) = 180.0
// item 2: 1 * 50.0  * (1 - 0/100)  = 50.0
// valor_total esperado = 230.0
func validPedidoPayload() map[string]any {
	return map[string]any{
		"cliente_id":  1,
		"vendedor_id": 2,
		"data_pedido": "2024-01-15",
		"canal":       "App",
		"status":      "Faturado",
		"itens": []map[string]any{
			{"produto_id": 10, "quantidade": 2, "preco_praticado": 100.0, "desconto_pct": 10.0},
			{"produto_id": 11, "quantidade": 1, "preco_praticado": 50.0, "desconto_pct": 0.0},
		},
	}
}

// expectCreateComItensSuccessH registra os mocks para uma criação/atualização
// bem-sucedida de pedido a partir de validPedidoPayload() (valor_total=230.0).
func expectCreatePedidoSuccessH(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(pedido_id_origem\), 0\) \+ 1 FROM pedidos`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(100)))

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO pedidos \(pedido_id_origem, cliente_id, vendedor_id, data_pedido, canal, status, valor_total\)`).
		WithArgs(int64(100), int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(item_id_origem\), 0\) \+ 1 FROM itens_pedido`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(1)))
	mock.ExpectExec(`INSERT INTO itens_pedido \(item_id_origem, pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(1), int64(1), int64(10), 2, 100.0, 10.0, 180.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido \(item_id_origem, pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(2), int64(1), int64(11), 1, 50.0, 0.0, 50.0).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler())
}

// ---------------------------------------------------------------------------
// ListPedidos GET /api/pedidos
// ---------------------------------------------------------------------------

func TestListPedidos_Success_Admin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\)` + pedidoFromRegexH).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` ORDER BY p\.id DESC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos", nil)
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

func TestListPedidos_ComFiltrosEPaginacao(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	whereRegex := ` WHERE p\.status = \? AND p\.canal = \? AND p\.cliente_id = \? AND p\.vendedor_id = \? AND p\.data_pedido >= \? AND p\.data_pedido <= \? AND c\.razao_social LIKE \?`
	mock.ExpectQuery(`SELECT COUNT\(\*\)` + pedidoFromRegexH + whereRegex).
		WithArgs("Faturado", "App", int64(1), int64(2), "2024-01-01", "2024-01-31", "%Teste%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + whereRegex + ` ORDER BY p\.id DESC LIMIT \? OFFSET \?`).
		WithArgs("Faturado", "App", int64(1), int64(2), "2024-01-01", "2024-01-31", "%Teste%", 10, 10).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos?page=2&limit=10&status=Faturado&canal=App&cliente_id=1&vendedor_id=2&data_inicio=2024-01-01&data_fim=2024-01-31&q=Teste", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	pagination := body["pagination"].(map[string]any)
	assert.Equal(t, float64(2), pagination["page"])
	assert.Equal(t, float64(10), pagination["limit"])
	assert.Equal(t, float64(25), pagination["total"])
	assert.Equal(t, float64(3), pagination["pages"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListPedidos_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "acesso restrito a administradores", body["error"])
}

func TestListPedidos_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\)` + pedidoFromRegexH).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetPedido GET /api/pedidos/{id}
// ---------------------------------------------------------------------------

func TestGetPedido_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(230), data["valor_total"])
	itens := data["itens"].([]any)
	assert.Len(t, itens, 2)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPedido_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyPedidoRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pedido não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPedido_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestGetPedido_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// CreatePedido POST /api/pedidos
// ---------------------------------------------------------------------------

func TestCreatePedido_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	expectCreatePedidoSuccessH(mock)

	req, _ := http.NewRequest("POST", server.URL+"/api/pedidos", makeJSON(validPedidoPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(1), data["id"])
	assert.Equal(t, float64(230), data["valor_total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreatePedido_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("POST", server.URL+"/api/pedidos", makeJSON(validPedidoPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "acesso restrito a administradores", body["error"])
}

func TestCreatePedido_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/pedidos", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreatePedido_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		wantMsg string
	}{
		{
			nome: "cliente_id ausente",
			payload: func() map[string]any {
				p := validPedidoPayload()
				p["cliente_id"] = 0
				return p
			},
			wantMsg: "cliente_id é obrigatório",
		},
		{
			nome: "vendedor_id ausente",
			payload: func() map[string]any {
				p := validPedidoPayload()
				p["vendedor_id"] = 0
				return p
			},
			wantMsg: "vendedor_id é obrigatório",
		},
		{
			nome: "data_pedido inválida",
			payload: func() map[string]any {
				p := validPedidoPayload()
				p["data_pedido"] = "15/01/2024"
				return p
			},
			wantMsg: "data_pedido inválida (use o formato AAAA-MM-DD)",
		},
		{
			nome: "canal inválido",
			payload: func() map[string]any {
				p := validPedidoPayload()
				p["canal"] = "Email"
				return p
			},
			wantMsg: "canal inválido (use: App, Telefone, Visita, WhatsApp)",
		},
		{
			nome: "status inválido",
			payload: func() map[string]any {
				p := validPedidoPayload()
				p["status"] = "Pendente"
				return p
			},
			wantMsg: "status inválido (use: Cancelado, Em separação, Entregue, Faturado)",
		},
		{
			nome: "itens vazio",
			payload: func() map[string]any {
				p := validPedidoPayload()
				p["itens"] = []map[string]any{}
				return p
			},
			wantMsg: "o pedido deve ter ao menos um item",
		},
		{
			nome: "produto_id ausente em um item",
			payload: func() map[string]any {
				p := validPedidoPayload()
				p["itens"] = []map[string]any{{"produto_id": 0, "quantidade": 1, "preco_praticado": 10.0, "desconto_pct": 0.0}}
				return p
			},
			wantMsg: "produto_id é obrigatório em todos os itens",
		},
		{
			nome: "quantidade inválida em um item",
			payload: func() map[string]any {
				p := validPedidoPayload()
				p["itens"] = []map[string]any{{"produto_id": 1, "quantidade": 0, "preco_praticado": 10.0, "desconto_pct": 0.0}}
				return p
			},
			wantMsg: "quantidade deve ser maior que zero em todos os itens",
		},
		{
			nome: "preco_praticado negativo em um item",
			payload: func() map[string]any {
				p := validPedidoPayload()
				p["itens"] = []map[string]any{{"produto_id": 1, "quantidade": 1, "preco_praticado": -1.0, "desconto_pct": 0.0}}
				return p
			},
			wantMsg: "preco_praticado deve ser maior ou igual a zero em todos os itens",
		},
		{
			nome: "desconto_pct fora do intervalo em um item",
			payload: func() map[string]any {
				p := validPedidoPayload()
				p["itens"] = []map[string]any{{"produto_id": 1, "quantidade": 1, "preco_praticado": 10.0, "desconto_pct": 150.0}}
				return p
			},
			wantMsg: "desconto_pct deve estar entre 0 e 100 em todos os itens",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, _ := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			req, _ := http.NewRequest("POST", server.URL+"/api/pedidos", makeJSON(tc.payload()))
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
// UpdatePedido PUT /api/pedidos/{id}
// ---------------------------------------------------------------------------

func TestUpdatePedido_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE pedidos\s+SET cliente_id = \?, vendedor_id = \?, data_pedido = \?, canal = \?, status = \?, valor_total = \?\s+WHERE id = \?`).
		WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(item_id_origem\), 0\) \+ 1 FROM itens_pedido`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(1)))
	mock.ExpectExec(`INSERT INTO itens_pedido \(item_id_origem, pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(1), int64(1), int64(10), 2, 100.0, 10.0, 180.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido \(item_id_origem, pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(2), int64(1), int64(11), 1, 50.0, 0.0, 50.0).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/pedidos/1", makeJSON(validPedidoPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(230), data["valor_total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePedido_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PUT", server.URL+"/api/pedidos/1", makeJSON(validPedidoPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpdatePedido_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/pedidos/abc", makeJSON(validPedidoPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdatePedido_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/pedidos/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestUpdatePedido_ValidacaoNegocio(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	payload := validPedidoPayload()
	payload["canal"] = "Email"

	req, _ := http.NewRequest("PUT", server.URL+"/api/pedidos/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "canal inválido (use: App, Telefone, Visita, WhatsApp)", body["error"])
}

func TestUpdatePedido_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE pedidos\s+SET cliente_id = \?, vendedor_id = \?, data_pedido = \?, canal = \?, status = \?, valor_total = \?\s+WHERE id = \?`).
		WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	req, _ := http.NewRequest("PUT", server.URL+"/api/pedidos/999", makeJSON(validPedidoPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pedido não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}
