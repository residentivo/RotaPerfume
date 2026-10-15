package handlers_test

// ARQ-05: contrato JSON byte a byte das rotas de Vendas contra cópias
// CONGELADAS dos models legados apagados (shared/models/pedido.go,
// item_pedido.go, pagamento.go e repositories.PedidoListagem/
// ItemPedidoDetalhe/PedidoDetalhe), serializadas pelo envelope legado (map +
// Encoder); e os quatro níveis do vlog sem valores digitados (P1, O7).

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	legadoPedido struct {
		PedidoIDOrigem int64     `json:"pedido_id_origem"`
		ClienteID      int64     `json:"cliente_id"`
		VendedorID     int64     `json:"vendedor_id"`
		DataPedido     time.Time `json:"data_pedido"`
		Canal          string    `json:"canal"`
		Status         string    `json:"status"`
		ValorTotal     float64   `json:"valor_total"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
	}
	legadoPedidoListagem struct {
		legadoPedido
		ClienteNome  string `json:"cliente_nome"`
		VendedorNome string `json:"vendedor_nome"`
	}
	legadoItemPedido struct {
		ItemIDOrigem   int64     `json:"item_id_origem"`
		PedidoID       int64     `json:"pedido_id"`
		ProdutoID      int64     `json:"produto_id"`
		Quantidade     int       `json:"quantidade"`
		PrecoPraticado float64   `json:"preco_praticado"`
		DescontoPct    float64   `json:"desconto_pct"`
		ValorBruto     float64   `json:"valor_bruto"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
	}
	legadoItemPedidoDetalhe struct {
		legadoItemPedido
		ProdutoSKU       string `json:"produto_sku"`
		ProdutoDescricao string `json:"produto_descricao"`
	}
	legadoPedidoDetalhe struct {
		legadoPedidoListagem
		Itens []legadoItemPedidoDetalhe `json:"itens"`
	}
	legadoPagamento struct {
		PagamentoID     int64      `json:"pagamento_id"`
		PedidoID        int64      `json:"pedido_id"`
		FormaPagamento  string     `json:"forma_pagamento"`
		Parcelas        uint8      `json:"parcelas"`
		Valor           float64    `json:"valor"`
		TaxaPct         float64    `json:"taxa_pct"`
		ValorLiquido    float64    `json:"valor_liquido"`
		DataVencimento  time.Time  `json:"data_vencimento"`
		DataPagamento   *time.Time `json:"data_pagamento"`
		StatusPagamento string     `json:"status_pagamento"`
		CreatedAt       time.Time  `json:"created_at"`
		UpdatedAt       time.Time  `json:"updated_at"`
	}
)

func TestARQ05_ContratoJSON_ByteAByte(t *testing.T) {
	quando := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	rowsPedido := func() *sqlmock.Rows {
		return sqlmock.NewRows(pedidoColunasHeaderForHandler()).
			AddRow(int64(7), int64(1), int64(2), quando, "App", "Entregue", 230.5, quando, quando, "ACME", "Ana")
	}
	cabecalho := legadoPedidoListagem{legadoPedido: legadoPedido{PedidoIDOrigem: 7, ClienteID: 1, VendedorID: 2, DataPedido: quando,
		Canal: "App", Status: "Entregue", ValorTotal: 230.5, CreatedAt: quando, UpdatedAt: quando}, ClienteNome: "ACME", VendedorNome: "Ana"}
	paginacao := func(total, pages int) map[string]any {
		return map[string]any{"page": 1, "limit": 20, "total": total, "pages": pages}
	}

	t.Run("GET /api/pedidos/{id} com itens", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`FROM pedidos p .+ WHERE p\.pedido_id_origem = \? LIMIT 1`).WithArgs(int64(7)).WillReturnRows(rowsPedido())
		mock.ExpectQuery(`FROM itens_pedido i`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows(itemPedidoColunasHeaderForHandler()).
			AddRow(int64(1), int64(7), int64(10), 2, 100.0, 10.0, 180.0, quando, quando, "SKU-10", "Perfume"))
		status, body := getComToken(t, server.URL+"/api/pedidos/7", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": legadoPedidoDetalhe{
			legadoPedidoListagem: cabecalho, Itens: []legadoItemPedidoDetalhe{{legadoItemPedido: legadoItemPedido{
				ItemIDOrigem: 1, PedidoID: 7, ProdutoID: 10, Quantidade: 2, PrecoPraticado: 100, DescontoPct: 10, ValorBruto: 180,
				CreatedAt: quando, UpdatedAt: quando}, ProdutoSKU: "SKU-10", ProdutoDescricao: "Perfume"}}}}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/pedidos/{id} sem itens: itens null", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`FROM pedidos p .+ WHERE p\.pedido_id_origem = \? LIMIT 1`).WithArgs(int64(7)).WillReturnRows(rowsPedido())
		mock.ExpectQuery(`FROM itens_pedido i`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows(itemPedidoColunasHeaderForHandler()))
		status, body := getComToken(t, server.URL+"/api/pedidos/7", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true,
			"data": legadoPedidoDetalhe{legadoPedidoListagem: cabecalho}}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/pedidos: listagem sem itens; sem linhas, data null", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos p`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
		mock.ExpectQuery(`ORDER BY p\.pedido_id_origem DESC LIMIT`).WillReturnRows(rowsPedido())
		status, body := getComToken(t, server.URL+"/api/pedidos", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true,
			"data": []legadoPedidoListagem{cabecalho}, "pagination": paginacao(1, 1)}), body)

		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos p`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
		mock.ExpectQuery(`ORDER BY p\.pedido_id_origem DESC LIMIT`).WillReturnRows(sqlmock.NewRows(pedidoColunasHeaderForHandler()))
		status, body = getComToken(t, server.URL+"/api/pedidos", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": nil, "pagination": paginacao(0, 0)}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("sem carteira: data [] sem SQL de listagem (E-03)", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		for _, rota := range []string{"/api/pedidos", "/api/pagamentos?pedido_id=abc"} {
			mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
			status, body := getComToken(t, server.URL+rota, 2, "normal")
			assert.Equal(t, http.StatusOK, status, rota)
			assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": []any{}, "pagination": paginacao(0, 0)}), body, rota)
		}
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/pagamentos/{id} com data_pagamento null", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`FROM pagamentos WHERE pagamento_id = \? LIMIT 1`).WithArgs(int64(3)).WillReturnRows(
			sqlmock.NewRows(pagamentoColunasHeaderForHandler()).
				AddRow(int64(3), int64(7), "PIX", uint8(2), 100.0, 1.5, 98.5, quando, nil, "Em aberto", quando, quando))
		status, body := getComToken(t, server.URL+"/api/pagamentos/3", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": legadoPagamento{
			PagamentoID: 3, PedidoID: 7, FormaPagamento: "PIX", Parcelas: 2, Valor: 100, TaxaPct: 1.5, ValorLiquido: 98.5,
			DataVencimento: quando, StatusPagamento: "Em aberto", CreatedAt: quando, UpdatedAt: quando}}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("POST /api/pagamentos com releitura falhando: objeto em memória (BUG-09)", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`SELECT 1 FROM pedidos WHERE pedido_id_origem = \? LIMIT 1`).WithArgs(int64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		mock.ExpectExec(`INSERT INTO pagamentos`).WillReturnResult(sqlmock.NewResult(9, 1))
		mock.ExpectQuery(`FROM pagamentos WHERE pagamento_id = \? LIMIT 1`).WithArgs(int64(9)).WillReturnError(errors.New("db down"))
		pagamento := validPagamentoPayload()
		pagamento["data_pagamento"] = "2024-01-16"
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/pagamentos", makeJSON(pagamento))
		req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		venc := time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local)
		pago := time.Date(2024, 1, 16, 0, 0, 0, 0, time.Local)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": legadoPagamento{
			PagamentoID: 9, PedidoID: 1, FormaPagamento: "PIX", Parcelas: 1, Valor: 100, ValorLiquido: 100,
			DataVencimento: venc, DataPagamento: &pago, StatusPagamento: "Em aberto"}}), string(readBody(t, resp)))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// P1 / O7: os quatro níveis do vlog nas rotas de Vendas, sem a busca
// digitada nem texto de SQL.
func TestARQ05_Vlog_NiveisSemValores(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	normal := generateToken(t, testCfg(), 7, "normal")
	out := capturarVlogLigado(t)

	mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(10))
	mock.ExpectQuery(reVendedorDesligado).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(0))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos p`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
	mock.ExpectQuery(`c\.razao_social LIKE \?`).WillReturnRows(sqlmock.NewRows(pedidoColunasHeaderForHandler()))
	status, _ := getComToken2(t, server.URL+"/api/pedidos?q=BuscaSecreta", normal)
	require.Equal(t, http.StatusOK, status)

	// Pedido de outro vendedor: decisão fora_da_carteira e 404.
	mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(10))
	mock.ExpectQuery(reVendedorDesligado).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(0))
	mock.ExpectQuery(`FROM pedidos p .+ WHERE p\.pedido_id_origem = \? LIMIT 1`).WithArgs(int64(1)).WillReturnRows(pedidoRowsForHandler(1, 1.0))
	mock.ExpectQuery(`FROM itens_pedido i`).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows(itemPedidoColunasHeaderForHandler()))
	status, _ = getComToken2(t, server.URL+"/api/pedidos/1", normal)
	require.Equal(t, http.StatusNotFound, status)
	require.NoError(t, mock.ExpectationsWereMet())

	saida := out.String()
	for _, nivel := range []string{
		"[pedido_handler.go] [PedidoHandler.ListPedidos] início GET",                     // 1. rota
		"[pedidos.go] [Vendas.ListarPedidos] início page=1 limit=20 busca=true ordem=id", // 2. caso de uso
		"[pedidos.go] [Vendas.ListarPedidos] fim ok",
		"[pedidos.go] [Pedidos.Listar] SELECT pedidos linhas=0 total=0",       // 4. query
		"[pedidos.go] [Vendas.detalheNoEscopo] decisão fora_da_carteira id=1", // 3. decisão
		"[pedidos.go] [Vendas.ObterPedido] fim erro=nao_encontrado",           // erro só pelo Kind
		"[pedidos.go] [Pedidos.Itens] SELECT itens_pedido linhas=0",
	} {
		assert.Contains(t, saida, nivel)
	}
	for _, valor := range []string{"BuscaSecreta", "LIKE", "SELECT COUNT", "razao_social"} {
		assert.NotContains(t, saida, valor, "o log não leva valores digitados nem texto de SQL")
	}
}
