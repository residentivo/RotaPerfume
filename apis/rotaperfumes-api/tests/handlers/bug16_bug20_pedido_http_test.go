package handlers_test

// BUG-16 / BUG-20 (Lote 25) pelo HTTP (router real + sqlmock):
//   - PUT /api/pedidos/{id}: Faturado só vai para Entregue; Entregue é
//     terminal; Entregue só a partir de Faturado → 400 com a mensagem exata,
//     rollback e nenhum UPDATE. Itens diferentes em Faturado/Entregue → 409
//     com o texto antigo. Reenviar o mesmo status é idempotente (200, sem
//     baixa). Faturado → Entregue com os mesmos itens → 200, sem baixa.
//   - POST /api/pedidos: criar já Entregue → 400 sem SQL; criar Faturado
//     baixa o estoque na mesma tx (falha na baixa → rollback, 500).
//   - DELETE /api/pedidos/{id}: Entregue → 409 "pedido entregue não pode ser
//     excluído"; pagamentos vinculados vêm antes do status.

import (
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

const (
	msgBUG16FaturadoSoEntregue = "pedido faturado só pode ter o status alterado para Entregue"
	msgBUG16EntregueTerminal   = "pedido entregue não pode ter o status alterado"
	msgBUG16EntregueSemFatura  = "pedido só pode ser marcado como Entregue depois de Faturado"
	msgBUG16ItensFaturado      = "pedido já faturado: não é possível alterar os itens, apenas o status"
)

// bug16ItensIguaisH são os itens de validPedidoPayload() como o banco os lê.
func bug16ItensIguaisH() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"produto_id", "quantidade", "preco_praticado", "desconto_pct"}).
		AddRow(int64(10), 2, 100.0, 10.0).AddRow(int64(11), 1, 50.0, 0.0)
}

func bug16PayloadH(status string) map[string]any {
	p := validPedidoPayload()
	p["status"] = status
	return p
}

func bug16ExpectLockH(mock sqlmock.Sqlmock, statusAtual string, itens *sqlmock.Rows) {
	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexH).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(statusAtual))
	mock.ExpectQuery(selectItensAtuaisTxRegexH).WithArgs(int64(1)).WillReturnRows(itens)
}

func bug16ExpectReleituraH(mock sqlmock.Sqlmock, status string) {
	mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsForHandlerStatus(1, 230.0, status))
	mock.ExpectQuery(itensByPedidoRegexH).WithArgs(int64(1)).WillReturnRows(itemPedidoRowsForHandler(1))
}

func TestBUG16_HTTP_PUT_TransicaoProibida_400RollbackSemUpdate(t *testing.T) {
	casos := []struct {
		atual, destino string
		msg            string
	}{
		{"Faturado", "Cancelado", msgBUG16FaturadoSoEntregue},
		{"Faturado", "Em separação", msgBUG16FaturadoSoEntregue},
		{"Entregue", "Faturado", msgBUG16EntregueTerminal},
		{"Entregue", "Cancelado", msgBUG16EntregueTerminal},
		{"Entregue", "Em separação", msgBUG16EntregueTerminal},
		{"Em separação", "Entregue", msgBUG16EntregueSemFatura},
		{"Cancelado", "Entregue", msgBUG16EntregueSemFatura},
	}
	for _, c := range casos {
		for _, papel := range []string{"admin", "normal"} {
			t.Run(papel+"/"+c.atual+"→"+c.destino, func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()
				uid := int64(1)
				if papel == "normal" {
					uid = 2
					expectEscopoUsuarioH(mock, 2, 2)
					mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsForHandlerStatus(1, 230.0, c.atual))
					mock.ExpectQuery(itensByPedidoRegexH).WithArgs(int64(1)).WillReturnRows(itemPedidoRowsForHandler(1))
					expectCarteiraAtivaH(mock, 2, 1)
				}
				bug16ExpectLockH(mock, c.atual, bug16ItensIguaisH())
				mock.ExpectRollback()

				status, body := doReq(t, http.MethodPut, server.URL+"/api/pedidos/1", generateToken(t, testCfg(), uid, papel), bug16PayloadH(c.destino))
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, c.msg, body["error"])
				assert.Equal(t, false, body["success"])
				assert.NoError(t, mock.ExpectationsWereMet(), "sem UPDATE, sem DELETE de itens, sem baixa")
			})
		}
	}
}

// Itens diferentes num pedido Faturado/Entregue: 409 com o texto antigo
// (contrato). A transição permitida não muda isso.
func TestBUG16_HTTP_PUT_ItensDiferentesPosFaturamento_409(t *testing.T) {
	for _, c := range []struct{ atual, destino string }{{"Faturado", "Entregue"}, {"Faturado", "Faturado"}, {"Entregue", "Entregue"}} {
		t.Run(c.atual+"→"+c.destino, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			bug16ExpectLockH(mock, c.atual, itensAtuaisRowsVazioH())
			mock.ExpectRollback()

			status, body := doReq(t, http.MethodPut, server.URL+"/api/pedidos/1", generateToken(t, testCfg(), 1, "admin"), bug16PayloadH(c.destino))
			assert.Equal(t, http.StatusConflict, status)
			assert.Equal(t, msgBUG16ItensFaturado, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Transições permitidas a partir do pós-faturamento (itens iguais): só o
// cabeçalho é gravado; sem DELETE/INSERT de itens e sem segunda baixa.
func TestBUG16_HTTP_PUT_PosFaturamentoPermitido_200SemBaixa(t *testing.T) {
	for _, c := range []struct{ atual, destino string }{{"Faturado", "Entregue"}, {"Faturado", "Faturado"}, {"Entregue", "Entregue"}} {
		t.Run(c.atual+"→"+c.destino, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			bug16ExpectLockH(mock, c.atual, bug16ItensIguaisH())
			mock.ExpectExec(updatePedidoRegexH).WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", c.destino, 230.0, int64(1)).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			bug16ExpectReleituraH(mock, c.destino)

			status, body := doReq(t, http.MethodPut, server.URL+"/api/pedidos/1", generateToken(t, testCfg(), 1, "admin"), bug16PayloadH(c.destino))
			assert.Equal(t, http.StatusOK, status, "body: %v", body)
			data, _ := body["data"].(map[string]any)
			assert.Equal(t, c.destino, data["status"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Reenvio do mesmo status não faturado e transições livres entre os não
// faturados continuam reescrevendo os itens (sem baixa).
func TestBUG16_HTTP_PUT_NaoFaturados_200(t *testing.T) {
	for _, c := range []struct{ atual, destino string }{{"Em separação", "Em separação"}, {"Em separação", "Cancelado"}, {"Cancelado", "Em separação"}} {
		t.Run(c.atual+"→"+c.destino, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			bug16ExpectLockH(mock, c.atual, bug16ItensIguaisH())
			mock.ExpectExec(updatePedidoRegexH).WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", c.destino, 230.0, int64(1)).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(deleteItensPedidoRegexH).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 2))
			mock.ExpectExec(insertItemRegexH).WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec(insertItemRegexH).WillReturnResult(sqlmock.NewResult(2, 1))
			mock.ExpectCommit()
			bug16ExpectReleituraH(mock, c.destino)

			status, _ := doReq(t, http.MethodPut, server.URL+"/api/pedidos/1", generateToken(t, testCfg(), 1, "admin"), bug16PayloadH(c.destino))
			assert.Equal(t, http.StatusOK, status)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// BUG-20: POST já Entregue → 400 antes de qualquer SQL (admin e normal:
// no normal, só o escopo e a carteira; nunca o BEGIN).
func TestBUG20_HTTP_POST_Entregue_400SemSQL(t *testing.T) {
	for _, papel := range []string{"admin", "normal"} {
		t.Run(papel, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			uid := int64(1)
			if papel == "normal" {
				uid = 2
				expectEscopoCarteiraPedidoH(mock, 2, 2, 1)
			}
			status, body := doReq(t, http.MethodPost, server.URL+"/api/pedidos", generateToken(t, testCfg(), uid, papel), bug16PayloadH("Entregue"))
			assert.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, msgBUG16EntregueSemFatura, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// BUG-20: POST Faturado com falha na baixa → rollback do INSERT e 500.
func TestBUG20_HTTP_POST_Faturado_FalhaNaBaixa_Rollback500(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec(insertPedidoRegexH).WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0).WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectExec(insertItemRegexH).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(insertItemRegexH).WillReturnResult(sqlmock.NewResult(2, 1))
	expectBaixaEstoquePorItemH(mock, 10, "SKU-010")
	mock.ExpectQuery(`SELECT sku FROM produtos WHERE id = \? LIMIT 1`).WithArgs(int64(11)).WillReturnError(errors.New("db down"))
	mock.ExpectRollback()

	status, body := doReq(t, http.MethodPost, server.URL+"/api/pedidos", generateToken(t, testCfg(), 1, "admin"), bug16PayloadH("Faturado"))
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "erro interno", body["error"])
	assert.NoError(t, mock.ExpectationsWereMet(), "sem commit e sem releitura")
}

// POST em status sem baixa: só INSERT de cabeçalho e itens.
func TestBUG20_HTTP_POST_SemBaixa(t *testing.T) {
	for _, st := range []string{"Em separação", "Cancelado"} {
		t.Run(st, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectExec(insertPedidoRegexH).WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", st, 230.0).WillReturnResult(sqlmock.NewResult(100, 1))
			mock.ExpectExec(insertItemRegexH).WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec(insertItemRegexH).WillReturnResult(sqlmock.NewResult(2, 1))
			mock.ExpectCommit()
			mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(100)).WillReturnRows(pedidoRowsForHandlerStatus(100, 230.0, st))
			mock.ExpectQuery(itensByPedidoRegexH).WithArgs(int64(100)).WillReturnRows(itemPedidoRowsForHandler(100))

			status, _ := doReq(t, http.MethodPost, server.URL+"/api/pedidos", generateToken(t, testCfg(), 1, "admin"), bug16PayloadH(st))
			assert.Equal(t, http.StatusCreated, status)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// DELETE: pagamentos vinculados primeiro, depois Faturado/Entregue.
func TestBUG16_HTTP_DELETE_TravasPorStatus(t *testing.T) {
	casos := []struct {
		status     string
		pagamentos bool
		want       int
		msg        string
	}{
		{"Entregue", false, http.StatusConflict, "pedido entregue não pode ser excluído"},
		{"Entregue", true, http.StatusConflict, "pedido possui pagamentos vinculados"},
		{"Faturado", false, http.StatusConflict, "pedido faturado não pode ser excluído, apenas ter o status alterado"},
		{"Faturado", true, http.StatusConflict, "pedido possui pagamentos vinculados"},
		{"Cancelado", false, http.StatusNoContent, ""},
	}
	for _, c := range casos {
		t.Run(c.status+"/"+map[bool]string{true: "com pagamentos", false: "sem pagamentos"}[c.pagamentos], func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsForHandlerStatus(1, 230.0, c.status))
			mock.ExpectQuery(itensByPedidoRegexH).WithArgs(int64(1)).WillReturnRows(itemPedidoRowsForHandler(1))
			mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsForHandlerStatus(1, 230.0, c.status))
			if c.pagamentos {
				mock.ExpectQuery(existsPagamentoPorPedidoRegexH).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			} else {
				mock.ExpectQuery(existsPagamentoPorPedidoRegexH).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"1"}))
			}
			if c.want == http.StatusNoContent {
				mock.ExpectBegin()
				mock.ExpectExec(deleteItensPedidoRegexH).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 2))
				mock.ExpectExec(deletePedidoRegexH).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}

			req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/pedidos/1", nil)
			req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
			resp, err := http.DefaultClient.Do(req)
			if !assert.NoError(t, err) {
				return
			}
			defer resp.Body.Close()
			assert.Equal(t, c.want, resp.StatusCode)
			if c.msg != "" {
				assert.Equal(t, c.msg, decodeResponse(t, readBody(t, resp))["error"])
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "nenhum DELETE quando bloqueado")
		})
	}
}
