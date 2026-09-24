package handlers_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Testes do escopo por carteira nas rotas de ESCRITA de pedidos e pagamentos
// (CreatePedido/UpdatePedido/CreatePagamento/UpdatePagamento) para usuário
// role=normal. Reaproveita os helpers de pedido_handler_test.go e
// pagamento_handler_test.go.

const (
	carteiraAtivaRegexH = `SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`
	escopoUsuarioRegexH = `SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`
	pedidoByIDRegexH    = `SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`
	itensByPedidoRegexH = `SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`
	pagamentoByIDRegexH = `SELECT ` + pagamentoColunasRegexH + pagamentoFromRegexH + ` WHERE pagamento_id = \? LIMIT 1`
	updatePedidoRegexH  = `UPDATE pedidos\s+SET cliente_id = \?, vendedor_id = \?, data_pedido = \?, canal = \?, status = \?, valor_total = \?\s+WHERE pedido_id_origem = \?`
	insertPedidoRegexH  = `INSERT INTO pedidos \(cliente_id, vendedor_id, data_pedido, canal, status, valor_total\)`
	insertItemRegexH    = `INSERT INTO itens_pedido \(pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`
	updatePagamentoRegH = `UPDATE pagamentos\s+SET forma_pagamento = \?, parcelas = \?, valor = \?, taxa_pct = \?, valor_liquido = \?, data_vencimento = \?, data_pagamento = \?, status_pagamento = \?\s+WHERE pagamento_id = \?`
	insertPagamentoRegH = `INSERT INTO pagamentos \(pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento\)`
	msgClienteForaCartH = "cliente não pertence à carteira deste vendedor"
	msgSemVendedorH     = "usuário sem vendedor vinculado"
)

// expectEscopoSemVendedorH: usuário role=normal com id_vendedor NULL.
func expectEscopoSemVendedorH(mock sqlmock.Sqlmock, userID int64) {
	mock.ExpectQuery(escopoUsuarioRegexH).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
}

// expectCarteiraInexistenteH: nenhum vínculo ativo vendedor x cliente.
func expectCarteiraInexistenteH(mock sqlmock.Sqlmock, vendedorID, clienteID int64) {
	mock.ExpectQuery(carteiraAtivaRegexH).
		WithArgs(vendedorID, clienteID).
		WillReturnError(sql.ErrNoRows)
}

// pedidoRowsVendedorH é como pedidoRowsForHandler, mas com vendedor_id
// parametrizável.
func pedidoRowsVendedorH(idOrigem, vendedorID int64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(pedidoColunasHeaderForHandler()).
		AddRow(idOrigem, int64(1), vendedorID, now, "App", "Faturado", 230.0, now, now, "Cliente Teste", "Vendedor Teste")
}

// expectPedidoAtualH registra a leitura do pedido atual (GetPedidoDetalhe:
// cabeçalho + itens) feita pelo UpdatePedido no escopo restrito.
func expectPedidoAtualH(mock sqlmock.Sqlmock, pedidoID, vendedorID int64) {
	mock.ExpectQuery(pedidoByIDRegexH).WithArgs(pedidoID).WillReturnRows(pedidoRowsVendedorH(pedidoID, vendedorID))
	mock.ExpectQuery(itensByPedidoRegexH).WithArgs(pedidoID).WillReturnRows(itemPedidoRowsForHandler(pedidoID))
}

func doReq(t *testing.T, method, url, token string, body map[string]any) (int, map[string]any) {
	t.Helper()
	var req *http.Request
	if body != nil {
		req, _ = http.NewRequest(method, url, makeJSON(body))
	} else {
		req, _ = http.NewRequest(method, url, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, decodeResponse(t, readBody(t, resp))
}

// ---------------------------------------------------------------------------
// CreatePedido — escopo por carteira
// ---------------------------------------------------------------------------

// Usuário normal (vendedor 7) envia vendedor_id=2 (outro vendedor) e
// cliente da própria carteira: o pedido é gravado com vendedor_id=7.
func TestCreatePedido_Normal_ForcaVendedorDoUsuario(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	token := generateToken(t, testCfg(), 2, "normal")

	expectEscopoCarteiraPedidoH(mock, 2, 7, 1)
	mock.ExpectBegin()
	mock.ExpectExec(insertPedidoRegexH).
		WithArgs(int64(1), int64(7), sqlmock.AnyArg(), "App", "Faturado", 230.0).
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectExec(insertItemRegexH).WithArgs(int64(100), int64(10), 2, 100.0, 10.0, 180.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(insertItemRegexH).WithArgs(int64(100), int64(11), 1, 50.0, 0.0, 50.0).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(100)).WillReturnRows(pedidoRowsVendedorH(100, 7))
	mock.ExpectQuery(itensByPedidoRegexH).WithArgs(int64(100)).WillReturnRows(itemPedidoRowsForHandler(100))

	payload := validPedidoPayload()
	payload["vendedor_id"] = 2 // tentativa de forjar pedido para outro vendedor

	status, body := doReq(t, "POST", server.URL+"/api/pedidos", token, payload)

	assert.Equal(t, http.StatusCreated, status)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(7), data["vendedor_id"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreatePedido_Normal_Negado(t *testing.T) {
	testCases := []struct {
		nome       string
		setup      func(mock sqlmock.Sqlmock)
		wantStatus int
		wantMsg    string
	}{
		{
			nome: "cliente fora da carteira",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 7)
				expectCarteiraInexistenteH(mock, 7, 1)
			},
			wantStatus: http.StatusBadRequest,
			wantMsg:    msgClienteForaCartH,
		},
		{
			nome:       "usuário sem vendedor vinculado (id_vendedor NULL)",
			setup:      func(mock sqlmock.Sqlmock) { expectEscopoSemVendedorH(mock, 2) },
			wantStatus: http.StatusForbidden,
			wantMsg:    msgSemVendedorH,
		},
		{
			nome: "usuário inexistente na tabela usuarios",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(escopoUsuarioRegexH).WithArgs(int64(2)).WillReturnError(sql.ErrNoRows)
			},
			wantStatus: http.StatusForbidden,
			wantMsg:    msgSemVendedorH,
		},
		{
			nome: "erro de banco ao resolver escopo",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(escopoUsuarioRegexH).WithArgs(int64(2)).WillReturnError(sqlmock.ErrCancelled)
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "erro interno",
		},
		{
			nome: "erro de banco ao checar carteira",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 7)
				mock.ExpectQuery(carteiraAtivaRegexH).WithArgs(int64(7), int64(1)).WillReturnError(sqlmock.ErrCancelled)
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "erro interno",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			token := generateToken(t, testCfg(), 2, "normal")
			tc.setup(mock)

			status, body := doReq(t, "POST", server.URL+"/api/pedidos", token, validPedidoPayload())

			assert.Equal(t, tc.wantStatus, status)
			assert.Equal(t, tc.wantMsg, body["error"])
			// Nenhum INSERT registrado: qualquer escrita inesperada faria o
			// sqlmock falhar (500) e/ou deixaria expectativas pendentes.
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UpdatePedido — escopo por carteira
// ---------------------------------------------------------------------------

func TestUpdatePedido_Normal_Negado(t *testing.T) {
	testCases := []struct {
		nome       string
		url        string
		setup      func(mock sqlmock.Sqlmock)
		wantStatus int
		wantMsg    string
	}{
		{
			nome: "pedido de outro vendedor",
			url:  "/api/pedidos/1",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 99)
				expectPedidoAtualH(mock, 1, 2)
			},
			wantStatus: http.StatusNotFound,
			wantMsg:    "pedido não encontrado",
		},
		{
			nome: "pedido inexistente",
			url:  "/api/pedidos/999",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 2)
				mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(999)).WillReturnRows(emptyPedidoRowsForHandler())
			},
			wantStatus: http.StatusNotFound,
			wantMsg:    "pedido não encontrado",
		},
		{
			nome:       "usuário sem vendedor vinculado",
			url:        "/api/pedidos/1",
			setup:      func(mock sqlmock.Sqlmock) { expectEscopoSemVendedorH(mock, 2) },
			wantStatus: http.StatusNotFound,
			wantMsg:    "pedido não encontrado",
		},
		{
			nome: "próprio pedido com cliente fora da carteira",
			url:  "/api/pedidos/1",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 2)
				expectPedidoAtualH(mock, 1, 2)
				expectCarteiraInexistenteH(mock, 2, 1)
			},
			wantStatus: http.StatusBadRequest,
			wantMsg:    msgClienteForaCartH,
		},
		{
			nome: "erro de banco ao resolver escopo",
			url:  "/api/pedidos/1",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(escopoUsuarioRegexH).WithArgs(int64(2)).WillReturnError(sqlmock.ErrCancelled)
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "erro interno",
		},
		{
			nome: "erro de banco ao buscar pedido atual",
			url:  "/api/pedidos/1",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 2)
				mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnError(sqlmock.ErrCancelled)
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "erro interno",
		},
		{
			nome: "erro de banco ao checar carteira",
			url:  "/api/pedidos/1",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 2)
				expectPedidoAtualH(mock, 1, 2)
				mock.ExpectQuery(carteiraAtivaRegexH).WithArgs(int64(2), int64(1)).WillReturnError(sqlmock.ErrCancelled)
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "erro interno",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			token := generateToken(t, testCfg(), 2, "normal")
			tc.setup(mock)

			status, body := doReq(t, "PUT", server.URL+tc.url, token, validPedidoPayload())

			assert.Equal(t, tc.wantStatus, status)
			assert.Equal(t, tc.wantMsg, body["error"])
			// Nenhum BEGIN/UPDATE esperado — o fluxo é interrompido antes.
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 404 de "outro vendedor" e de "inexistente" devem ter corpo idêntico (não
// revelar existência de pedidos de terceiros).
func TestUpdatePedido_Normal_404IndistinguivelDeInexistente(t *testing.T) {
	executar := func(url string, setup func(sqlmock.Sqlmock)) (int, map[string]any) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		setup(mock)
		status, body := doReq(t, "PUT", server.URL+url, generateToken(t, testCfg(), 2, "normal"), validPedidoPayload())
		assert.NoError(t, mock.ExpectationsWereMet())
		return status, body
	}

	stOutro, bodyOutro := executar("/api/pedidos/1", func(m sqlmock.Sqlmock) {
		expectEscopoUsuarioH(m, 2, 99)
		expectPedidoAtualH(m, 1, 2)
	})
	stInex, bodyInex := executar("/api/pedidos/999", func(m sqlmock.Sqlmock) {
		expectEscopoUsuarioH(m, 2, 99)
		m.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(999)).WillReturnRows(emptyPedidoRowsForHandler())
	})

	assert.Equal(t, http.StatusNotFound, stOutro)
	assert.Equal(t, stOutro, stInex)
	assert.Equal(t, bodyOutro, bodyInex)
}

// Usuário normal (vendedor 2) edita o próprio pedido enviando vendedor_id=99:
// o UPDATE é executado com vendedor_id=2 (mantido).
func TestUpdatePedido_Normal_NaoTrocaVendedor(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	token := generateToken(t, testCfg(), 2, "normal")

	expectEscopoUsuarioH(mock, 2, 2)
	expectPedidoAtualH(mock, 1, 2)
	expectCarteiraAtivaH(mock, 2, 1)

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexH).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Pendente"))
	mock.ExpectQuery(selectItensAtuaisTxRegexH).WithArgs(int64(1)).WillReturnRows(itensAtuaisRowsVazioH())
	mock.ExpectExec(updatePedidoRegexH).
		WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(insertItemRegexH).WithArgs(int64(1), int64(10), 2, 100.0, 10.0, 180.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(insertItemRegexH).WithArgs(int64(1), int64(11), 1, 50.0, 0.0, 50.0).
		WillReturnResult(sqlmock.NewResult(2, 1))
	expectBaixaEstoquePorItemH(mock, 10, "SKU-010")
	expectBaixaEstoquePorItemH(mock, 11, "SKU-011")
	mock.ExpectCommit()
	expectPedidoAtualH(mock, 1, 2)

	payload := validPedidoPayload()
	payload["vendedor_id"] = 99

	status, body := doReq(t, "PUT", server.URL+"/api/pedidos/1", token, payload)

	assert.Equal(t, http.StatusOK, status)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(2), data["vendedor_id"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreatePagamento — escopo por carteira
// ---------------------------------------------------------------------------

func TestCreatePagamento_Normal_Negado(t *testing.T) {
	testCases := []struct {
		nome       string
		setup      func(mock sqlmock.Sqlmock)
		wantStatus int
		wantMsg    string
	}{
		{
			nome: "pedido de outro vendedor",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 99)
				mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsVendedorH(1, 2))
			},
			wantStatus: http.StatusNotFound,
			wantMsg:    "pedido não encontrado",
		},
		{
			nome: "pedido inexistente",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 2)
				mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(emptyPedidoRowsForHandler())
			},
			wantStatus: http.StatusNotFound,
			wantMsg:    "pedido não encontrado",
		},
		{
			nome:       "usuário sem vendedor vinculado",
			setup:      func(mock sqlmock.Sqlmock) { expectEscopoSemVendedorH(mock, 2) },
			wantStatus: http.StatusForbidden,
			wantMsg:    msgSemVendedorH,
		},
		{
			nome: "erro de banco ao resolver escopo",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(escopoUsuarioRegexH).WithArgs(int64(2)).WillReturnError(sqlmock.ErrCancelled)
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "erro interno",
		},
		{
			nome: "erro de banco ao buscar vendedor do pedido",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 2)
				mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnError(sqlmock.ErrCancelled)
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "erro interno",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			token := generateToken(t, testCfg(), 2, "normal")
			tc.setup(mock)

			status, body := doReq(t, "POST", server.URL+"/api/pagamentos", token, validPagamentoPayload())

			assert.Equal(t, tc.wantStatus, status)
			assert.Equal(t, tc.wantMsg, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCreatePagamento_Normal_404IndistinguivelDeInexistente(t *testing.T) {
	executar := func(setup func(sqlmock.Sqlmock)) (int, map[string]any) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		setup(mock)
		status, body := doReq(t, "POST", server.URL+"/api/pagamentos", generateToken(t, testCfg(), 2, "normal"), validPagamentoPayload())
		assert.NoError(t, mock.ExpectationsWereMet())
		return status, body
	}

	stOutro, bodyOutro := executar(func(m sqlmock.Sqlmock) {
		expectEscopoUsuarioH(m, 2, 99)
		m.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsVendedorH(1, 2))
	})
	stInex, bodyInex := executar(func(m sqlmock.Sqlmock) {
		expectEscopoUsuarioH(m, 2, 99)
		m.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(emptyPedidoRowsForHandler())
	})

	assert.Equal(t, http.StatusNotFound, stOutro)
	assert.Equal(t, stOutro, stInex)
	assert.Equal(t, bodyOutro, bodyInex)
}

func TestCreatePagamento_Normal_ProprioEscopo(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	token := generateToken(t, testCfg(), 2, "normal")

	expectEscopoUsuarioH(mock, 2, 2)
	mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsVendedorH(1, 2))
	mock.ExpectQuery(`SELECT 1 FROM pedidos WHERE pedido_id_origem = \? LIMIT 1`).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(insertPagamentoRegH).
		WithArgs(int64(1), "PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto").
		WillReturnResult(sqlmock.NewResult(1, 1))

	status, body := doReq(t, "POST", server.URL+"/api/pagamentos", token, validPagamentoPayload())

	assert.Equal(t, http.StatusCreated, status)
	assert.True(t, body["success"].(bool))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// pedido_id <= 0 no escopo restrito: não consulta vendedor do pedido e cai
// na validação do service (400).
func TestCreatePagamento_Normal_PedidoIDAusente(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	token := generateToken(t, testCfg(), 2, "normal")
	expectEscopoUsuarioH(mock, 2, 2)

	payload := validPagamentoPayload()
	payload["pedido_id"] = 0

	status, body := doReq(t, "POST", server.URL+"/api/pagamentos", token, payload)

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "pedido_id é obrigatório", body["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UpdatePagamento — escopo por carteira
// ---------------------------------------------------------------------------

func TestUpdatePagamento_Normal_Negado(t *testing.T) {
	testCases := []struct {
		nome       string
		setup      func(mock sqlmock.Sqlmock)
		wantStatus int
		wantMsg    string
	}{
		{
			nome: "pagamento de pedido de outro vendedor",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 99)
				mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))
				mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsVendedorH(1, 2))
			},
			wantStatus: http.StatusNotFound,
			wantMsg:    "pagamento não encontrado",
		},
		{
			nome: "pagamento inexistente",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 2)
				mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(emptyPagamentoRowsForHandler())
			},
			wantStatus: http.StatusNotFound,
			wantMsg:    "pagamento não encontrado",
		},
		{
			nome: "pedido do pagamento inexistente",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 2)
				mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))
				mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(emptyPedidoRowsForHandler())
			},
			wantStatus: http.StatusNotFound,
			wantMsg:    "pagamento não encontrado",
		},
		{
			nome:       "usuário sem vendedor vinculado",
			setup:      func(mock sqlmock.Sqlmock) { expectEscopoSemVendedorH(mock, 2) },
			wantStatus: http.StatusNotFound,
			wantMsg:    "pagamento não encontrado",
		},
		{
			nome: "erro de banco ao resolver escopo",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(escopoUsuarioRegexH).WithArgs(int64(2)).WillReturnError(sqlmock.ErrCancelled)
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "erro interno",
		},
		{
			nome: "erro de banco ao buscar pagamento atual",
			setup: func(mock sqlmock.Sqlmock) {
				expectEscopoUsuarioH(mock, 2, 2)
				mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnError(sqlmock.ErrCancelled)
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "erro interno",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			token := generateToken(t, testCfg(), 2, "normal")
			tc.setup(mock)

			payload := validPagamentoPayload()
			delete(payload, "pedido_id")
			status, body := doReq(t, "PUT", server.URL+"/api/pagamentos/1", token, payload)

			assert.Equal(t, tc.wantStatus, status)
			assert.Equal(t, tc.wantMsg, body["error"])
			// Nenhum UPDATE esperado.
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpdatePagamento_Normal_ProprioEscopo(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	token := generateToken(t, testCfg(), 2, "normal")

	expectEscopoUsuarioH(mock, 2, 2)
	mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))
	mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsVendedorH(1, 2))
	mock.ExpectExec(updatePagamentoRegH).
		WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))

	payload := validPagamentoPayload()
	delete(payload, "pedido_id")

	status, body := doReq(t, "PUT", server.URL+"/api/pagamentos/1", token, payload)

	assert.Equal(t, http.StatusOK, status)
	assert.True(t, body["success"].(bool))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreatePagamento (admin) — 404 único para pedido inexistente
// ---------------------------------------------------------------------------

// Admin com pedido inexistente recebe o mesmo 404 "pedido não encontrado"
// do escopo restrito — status único para pedido ausente, em qualquer papel.
func TestCreatePagamento_Admin_PedidoInexistente_404IgualNormal(t *testing.T) {
	executar := func(role string, setup func(sqlmock.Sqlmock)) (int, map[string]any) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		setup(mock)
		status, body := doReq(t, "POST", server.URL+"/api/pagamentos", generateToken(t, testCfg(), 2, role), validPagamentoPayload())
		assert.NoError(t, mock.ExpectationsWereMet())
		return status, body
	}

	stAdmin, bodyAdmin := executar("admin", func(m sqlmock.Sqlmock) {
		m.ExpectQuery(`SELECT 1 FROM pedidos WHERE pedido_id_origem = \? LIMIT 1`).WithArgs(int64(1)).
			WillReturnError(sql.ErrNoRows)
	})
	stNormal, bodyNormal := executar("normal", func(m sqlmock.Sqlmock) {
		expectEscopoUsuarioH(m, 2, 99)
		m.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(emptyPedidoRowsForHandler())
	})

	assert.Equal(t, http.StatusNotFound, stAdmin)
	assert.Equal(t, "pedido não encontrado", bodyAdmin["error"])
	assert.Equal(t, stNormal, stAdmin)
	assert.Equal(t, bodyNormal, bodyAdmin)
}

// Admin com pedido_id <= 0 continua 400 (validação do service), sem
// consultar o banco.
func TestCreatePagamento_Admin_PedidoIDAusente_400(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	payload := validPagamentoPayload()
	payload["pedido_id"] = 0

	status, body := doReq(t, "POST", server.URL+"/api/pagamentos", generateToken(t, testCfg(), 1, "admin"), payload)

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "pedido_id é obrigatório", body["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}
