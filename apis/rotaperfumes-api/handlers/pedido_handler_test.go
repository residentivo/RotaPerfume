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

// Regexes/colunas que espelham as constantes de apis/shared/repositories/pedido_repository.go.
const pedidoColunasRegexH = `p\.pedido_id_origem, p\.cliente_id, p\.vendedor_id, p\.data_pedido, p\.canal, p\.status, p\.valor_total, p\.created_at, p\.updated_at, c\.razao_social, v\.nome`
const pedidoFromRegexH = ` FROM pedidos p JOIN clientes c ON c\.cliente_id_origem = p\.cliente_id JOIN vendedores v ON v\.id = p\.vendedor_id`
const itemPedidoColunasRegexH = `i\.item_id_origem, i\.pedido_id, i\.produto_id, i\.quantidade, i\.preco_praticado, i\.desconto_pct, i\.valor_bruto, i\.created_at, i\.updated_at, pr\.sku, pr\.descricao`
const itemPedidoFromRegexH = ` FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id`

func pedidoColunasHeaderForHandler() []string {
	return []string{
		"pedido_id_origem", "cliente_id", "vendedor_id", "data_pedido",
		"canal", "status", "valor_total", "created_at", "updated_at", "cliente_nome", "vendedor_nome",
	}
}

func pedidoRowsForHandler(idOrigem int64, valorTotal float64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(pedidoColunasHeaderForHandler()).
		AddRow(idOrigem, int64(1), int64(2), now, "App", "Faturado", valorTotal, now, now, "Cliente Teste", "Vendedor Teste")
}

func emptyPedidoRowsForHandler() *sqlmock.Rows {
	return sqlmock.NewRows(pedidoColunasHeaderForHandler())
}

func itemPedidoColunasHeaderForHandler() []string {
	return []string{
		"item_id_origem", "pedido_id", "produto_id", "quantidade",
		"preco_praticado", "desconto_pct", "valor_bruto", "created_at", "updated_at", "produto_sku", "produto_descricao",
	}
}

func itemPedidoRowsForHandler(pedidoID int64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(itemPedidoColunasHeaderForHandler()).
		AddRow(int64(1), pedidoID, int64(10), 2, 100.0, 10.0, 180.0, now, now, "SKU-010", "Produto A").
		AddRow(int64(2), pedidoID, int64(11), 1, 50.0, 0.0, 50.0, now, now, "SKU-011", "Produto B")
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

// expectCreatePedidoSuccessH registra os mocks para uma criação bem-sucedida
// de pedido a partir de validPedidoPayload() (valor_total=230.0).
// pedido_id_origem=100 é obtido via LastInsertId() do INSERT em pedidos, não
// mais via SELECT MAX(...) + 1 manual.
func expectCreatePedidoSuccessH(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO pedidos \(cliente_id, vendedor_id, data_pedido, canal, status, valor_total\)`).
		WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0).
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido \(pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(100), int64(10), 2, 100.0, 10.0, 180.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido \(pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(100), int64(11), 1, 50.0, 0.0, 50.0).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(pedidoRowsForHandler(100, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(100)).
		WillReturnRows(itemPedidoRowsForHandler(100))
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
	mock.ExpectQuery(`SELECT `+pedidoColunasRegexH+pedidoFromRegexH+` ORDER BY p\.pedido_id_origem DESC LIMIT \? OFFSET \?`).
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
	mock.ExpectQuery(`SELECT COUNT\(\*\)`+pedidoFromRegexH+whereRegex).
		WithArgs("Faturado", "App", int64(1), int64(2), "2024-01-01", "2024-01-31", "%Teste%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))
	mock.ExpectQuery(`SELECT `+pedidoColunasRegexH+pedidoFromRegexH+whereRegex+` ORDER BY p\.pedido_id_origem DESC LIMIT \? OFFSET \?`).
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

func TestListPedidos_OrderBy(t *testing.T) {
	testCases := []struct {
		nome        string
		query       string
		orderRegexp string
	}{
		{"order_by e order_dir válidos", "order_by=valor_total&order_dir=asc", `ORDER BY p\.valor_total ASC`},
		{"order_by com alias de outra tabela (cliente_nome)", "order_by=cliente_nome&order_dir=asc", `ORDER BY c\.razao_social ASC`},
		{"order_dir maiúsculo (case-insensitive)", "order_by=status&order_dir=ASC", `ORDER BY p\.status ASC`},
		{"order_dir inválido cai no default (desc)", "order_by=status&order_dir=sideways", `ORDER BY p\.status DESC`},
		{"order_by fora da whitelist cai no default", "order_by=id_secreto&order_dir=asc", `ORDER BY p\.pedido_id_origem ASC`},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			mock.ExpectQuery(`SELECT COUNT\(\*\)` + pedidoFromRegexH).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			mock.ExpectQuery(`SELECT `+pedidoColunasRegexH+pedidoFromRegexH+` `+tc.orderRegexp+` LIMIT \? OFFSET \?`).
				WithArgs(20, 0).
				WillReturnRows(pedidoRowsForHandler(1, 230.0))

			req, _ := http.NewRequest("GET", server.URL+"/api/pedidos?"+tc.query, nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestListPedidos_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	vendedorWhere := ` WHERE p\.vendedor_id = \?`
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(2)))
	mock.ExpectQuery(`SELECT COUNT\(\*\)` + pedidoFromRegexH + vendedorWhere).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+pedidoColunasRegexH+pedidoFromRegexH+vendedorWhere+` ORDER BY p\.pedido_id_origem DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(2), 20, 0).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
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

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler(1))

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

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
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

func TestGetPedido_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(2)))
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler(1))

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPedido_NegadoParaNaoAdminForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// pedidoRowsForHandler(1, ...) tem vendedor_id=2, mas o usuário autenticado
	// está vinculado ao vendedor 99 — não deve conseguir ver o pedido.
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	// O service busca cabeçalho + itens numa única chamada (GetPedidoDetalhe);
	// a checagem de propriedade acontece só depois, no handler — os itens são
	// descartados na resposta (404), mas a query ainda é executada.
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler(1))

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pedido não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
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
	assert.Equal(t, float64(100), data["pedido_id_origem"])
	assert.Equal(t, float64(230), data["valor_total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreatePedido_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	expectCreatePedidoSuccessH(mock)

	req, _ := http.NewRequest("POST", server.URL+"/api/pedidos", makeJSON(validPedidoPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
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

// selectStatusForUpdateRegexH/selectItensAtuaisTxRegexH espelham as queries
// executadas dentro da tx de PedidoRepository.UpdateComItens (SELECT status
// ... FOR UPDATE + leitura dos itens atuais para comparação).
const selectStatusForUpdateRegexH = `SELECT status FROM pedidos WHERE pedido_id_origem = \? FOR UPDATE`
const selectItensAtuaisTxRegexH = `SELECT produto_id, quantidade, preco_praticado, desconto_pct FROM itens_pedido WHERE pedido_id = \? ORDER BY item_id_origem ASC`

func itensAtuaisRowsVazioH() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"produto_id", "quantidade", "preco_praticado", "desconto_pct"})
}

// expectBaixaEstoquePorItemH registra os mocks da baixa de estoque
// (AjustarSaldoPorFaturamento) disparada na transição de status para
// "Faturado", para o produto/sku informados (sem registro prévio no dia).
func expectBaixaEstoquePorItemH(mock sqlmock.Sqlmock, produtoID int64, sku string) {
	mock.ExpectQuery(`SELECT sku FROM produtos WHERE id = \? LIMIT 1`).
		WithArgs(produtoID).
		WillReturnRows(sqlmock.NewRows([]string{"sku"}).AddRow(sku))
	mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO estoque`).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func TestUpdatePedido_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexH).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Pendente"))
	mock.ExpectQuery(selectItensAtuaisTxRegexH).
		WithArgs(int64(1)).
		WillReturnRows(itensAtuaisRowsVazioH())
	mock.ExpectExec(`UPDATE pedidos\s+SET cliente_id = \?, vendedor_id = \?, data_pedido = \?, canal = \?, status = \?, valor_total = \?\s+WHERE pedido_id_origem = \?`).
		WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`INSERT INTO itens_pedido \(pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(1), int64(10), 2, 100.0, 10.0, 180.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido \(pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(1), int64(11), 1, 50.0, 0.0, 50.0).
		WillReturnResult(sqlmock.NewResult(2, 1))
	expectBaixaEstoquePorItemH(mock, 10, "SKU-010")
	expectBaixaEstoquePorItemH(mock, 11, "SKU-011")
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler(1))

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

func TestUpdatePedido_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexH).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Pendente"))
	mock.ExpectQuery(selectItensAtuaisTxRegexH).
		WithArgs(int64(1)).
		WillReturnRows(itensAtuaisRowsVazioH())
	mock.ExpectExec(`UPDATE pedidos\s+SET cliente_id = \?, vendedor_id = \?, data_pedido = \?, canal = \?, status = \?, valor_total = \?\s+WHERE pedido_id_origem = \?`).
		WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`INSERT INTO itens_pedido \(pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(1), int64(10), 2, 100.0, 10.0, 180.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido \(pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(1), int64(11), 1, 50.0, 0.0, 50.0).
		WillReturnResult(sqlmock.NewResult(2, 1))
	expectBaixaEstoquePorItemH(mock, 10, "SKU-010")
	expectBaixaEstoquePorItemH(mock, 11, "SKU-011")
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler(1))

	req, _ := http.NewRequest("PUT", server.URL+"/api/pedidos/1", makeJSON(validPedidoPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
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
	mock.ExpectQuery(selectStatusForUpdateRegexH).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)
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

// TestUpdatePedido_JaFaturadoTentaAlterarItens_409 cobre a exigência do
// SecBrain: pedido já "Faturado" não pode ter os itens alterados no PUT
// (apenas o status) — o repositório retorna
// ErrPedidoJaFaturadoNaoPodeAlterarItens, mapeado para HTTP 409.
func TestUpdatePedido_JaFaturadoTentaAlterarItens_409(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexH).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Faturado"))
	mock.ExpectQuery(selectItensAtuaisTxRegexH).
		WithArgs(int64(1)).
		WillReturnRows(itensAtuaisRowsVazioH())
	mock.ExpectRollback()

	req, _ := http.NewRequest("PUT", server.URL+"/api/pedidos/1", makeJSON(validPedidoPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// DeletePedido DELETE /api/pedidos/{id}
// ---------------------------------------------------------------------------

// pedidoRowsForHandlerStatus é igual a pedidoRowsForHandler, mas permite
// parametrizar o status (pedidoRowsForHandler sempre retorna "Faturado", que
// bloquearia a exclusão nos cenários de sucesso do DeletePedido).
func pedidoRowsForHandlerStatus(idOrigem int64, valorTotal float64, status string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(pedidoColunasHeaderForHandler()).
		AddRow(idOrigem, int64(1), int64(2), now, "App", status, valorTotal, now, now, "Cliente Teste", "Vendedor Teste")
}

const deleteItensPedidoRegexH = `DELETE FROM itens_pedido WHERE pedido_id = \?`
const deletePedidoRegexH = `DELETE FROM pedidos WHERE pedido_id_origem = \?`
const existsPagamentoPorPedidoRegexH = `SELECT 1 FROM pagamentos WHERE pedido_id = \? LIMIT 1`

func TestDeletePedido_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandlerStatus(1, 230.0, "Em separação"))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler(1))
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandlerStatus(1, 230.0, "Em separação"))
	mock.ExpectQuery(existsPagamentoPorPedidoRegexH).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec(deleteItensPedidoRegexH).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(deletePedidoRegexH).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	req, _ := http.NewRequest("DELETE", server.URL+"/api/pedidos/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeletePedido_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(2)))
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandlerStatus(1, 230.0, "Em separação"))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler(1))
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandlerStatus(1, 230.0, "Em separação"))
	mock.ExpectQuery(existsPagamentoPorPedidoRegexH).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec(deleteItensPedidoRegexH).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(deletePedidoRegexH).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	req, _ := http.NewRequest("DELETE", server.URL+"/api/pedidos/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeletePedido_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyPedidoRowsForHandler())

	req, _ := http.NewRequest("DELETE", server.URL+"/api/pedidos/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pedido não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestDeletePedido_NegadoParaNaoAdminForaDaCarteira cobre o cenário de IDOR:
// vendedor não-admin tentando excluir pedido de outro vendedor recebe 404
// (não 403, para não confirmar a existência do registro).
func TestDeletePedido_NegadoParaNaoAdminForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// pedidoRowsForHandler(1, ...) tem vendedor_id=2, mas o usuário
	// autenticado está vinculado ao vendedor 99.
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler(1))

	req, _ := http.NewRequest("DELETE", server.URL+"/api/pedidos/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pedido não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeletePedido_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("DELETE", server.URL+"/api/pedidos/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

// TestDeletePedido_PagamentosVinculados_409 cobre a regra do SecBrain:
// pedido com pagamentos vinculados não pode ser excluído.
func TestDeletePedido_PagamentosVinculados_409(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandlerStatus(1, 230.0, "Em separação"))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler(1))
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandlerStatus(1, 230.0, "Em separação"))
	mock.ExpectQuery(existsPagamentoPorPedidoRegexH).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	req, _ := http.NewRequest("DELETE", server.URL+"/api/pedidos/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pedido possui pagamentos vinculados", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestDeletePedido_Faturado_409 cobre a regra do SecBrain: pedido faturado
// não pode ser excluído, apenas ter o status alterado.
func TestDeletePedido_Faturado_409(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// pedidoRowsForHandler default já retorna status "Faturado".
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRowsForHandler(1))
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(existsPagamentoPorPedidoRegexH).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("DELETE", server.URL+"/api/pedidos/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pedido faturado não pode ser excluído, apenas ter o status alterado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}
