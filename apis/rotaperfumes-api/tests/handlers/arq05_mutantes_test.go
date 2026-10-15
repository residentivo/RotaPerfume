package handlers_test

// ARQ-05 (TestBrain): V-03 de ponta a ponta. valor_total e valor_bruto
// enviados no payload são ignorados: o backend grava e devolve os valores
// calculados pelo domínio (item 1: 2*100*(1-10/100)=180; item 2: 1*50=50;
// total 230). Mata os mutantes em que o DTO/domínio passam a aceitar esses
// campos do body (os testes legados nunca os enviavam).

import (
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func arq05PayloadComValoresForjados() map[string]any {
	p := validPedidoPayload()
	p["valor_total"] = 9999.0
	itens := p["itens"].([]map[string]any)
	for _, it := range itens {
		it["valor_bruto"] = 9999.0
	}
	return p
}

func arq05ExpectUpdatePedidoAdmin(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexH).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Em separação"))
	mock.ExpectQuery(selectItensAtuaisTxRegexH).WithArgs(int64(1)).WillReturnRows(itensAtuaisRowsVazioH())
	mock.ExpectExec(`UPDATE pedidos\s+SET cliente_id = \?, vendedor_id = \?, data_pedido = \?, canal = \?, status = \?, valor_total = \?\s+WHERE pedido_id_origem = \?`).
		WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`INSERT INTO itens_pedido \(pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(1), int64(10), 2, 100.0, 10.0, 180.0).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido \(pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(1), int64(11), 1, 50.0, 0.0, 50.0).WillReturnResult(sqlmock.NewResult(2, 1))
	expectBaixaEstoquePorItemH(mock, 10, "SKU-010")
	expectBaixaEstoquePorItemH(mock, 11, "SKU-011")
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).WillReturnRows(pedidoRowsForHandler(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).WillReturnRows(itemPedidoRowsForHandler(1))
}

func TestARQ05_V03_ValoresDoPayloadIgnorados(t *testing.T) {
	casos := []struct {
		nome       string
		metodo     string
		url        string
		prepara    func(sqlmock.Sqlmock)
		wantStatus int
	}{
		{"POST /api/pedidos", http.MethodPost, "/api/pedidos", expectCreatePedidoSuccessH, http.StatusCreated},
		{"PUT /api/pedidos/1", http.MethodPut, "/api/pedidos/1", arq05ExpectUpdatePedidoAdmin, http.StatusOK},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			c.prepara(mock)

			status, body := doReq(t, c.metodo, server.URL+c.url, generateToken(t, testCfg(), 1, "admin"),
				arq05PayloadComValoresForjados())

			require.Equal(t, c.wantStatus, status, "body: %v", body)
			data, ok := body["data"].(map[string]any)
			require.True(t, ok, "resposta sem data: %v", body)
			assert.Equal(t, float64(230), data["valor_total"])
			assert.NoError(t, mock.ExpectationsWereMet(), "INSERT/UPDATE com os valores calculados (230/180/50), nunca 9999")
		})
	}
}
