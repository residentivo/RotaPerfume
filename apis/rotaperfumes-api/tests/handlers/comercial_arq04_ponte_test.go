package handlers_test

// ARQ-04: os handlers legados de Vendas (pedidos, pagamentos) passaram a
// resolver o escopo pelo AutorizadorEscopo (ponte handlers/scope.go). Estes
// casos fixam o contrato R3 dessas rotas pela API pública, para valer
// também depois do ARQ-05: erro de infraestrutura (inclusive na resolução do
// escopo) → 500 "erro interno", nunca "sem filtro". Sem expectativa no
// sqlmock, a primeira consulta falha.

import (
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARQ04_PonteEscopo_Vendas_ErroDeInfra500(t *testing.T) {
	casos := []struct {
		metodo, path string
		body         map[string]any
		uid          int64
		role         string
		preparar     func(m sqlmock.Sqlmock)
	}{
		{http.MethodGet, "/api/pedidos/1", nil, 1, "admin", nil},
		{http.MethodPost, "/api/pedidos", validPedidoPayload(), 1, "admin", nil},
		{http.MethodDelete, "/api/pedidos/1", nil, 1, "admin", nil},
		{http.MethodGet, "/api/pagamentos/1", nil, 1, "admin", nil},
		{http.MethodPost, "/api/pagamentos", validPagamentoPayload(), 1, "admin", nil},
		{http.MethodDelete, "/api/pagamentos/1", nil, 1, "admin", nil},
		// Erro ao resolver o escopo do usuário restrito: 500, sem consultar
		// pagamentos (fail-closed).
		{http.MethodDelete, "/api/pagamentos/1", nil, 7, "normal", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reUsuarioVendedor).WithArgs(int64(7)).WillReturnError(errors.New("db down"))
		}},
	}
	for _, c := range casos {
		t.Run(c.metodo+" "+c.path+" "+c.role, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			if c.preparar != nil {
				c.preparar(mock)
			}
			var corpo = makeJSON(c.body)
			req, _ := http.NewRequest(c.metodo, server.URL+c.path, corpo)
			req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), c.uid, c.role))
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
			assert.JSONEq(t, `{"success":false,"error":"erro interno"}`, string(readBody(t, resp)))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// V-02 pela ponte: sem carteira, ou falha ao buscar o vendedor do pedido do
// pagamento (Get/Delete) → 404 "pagamento não encontrado", nunca 500 nem o
// registro; e falha na exclusão de um registro achado → 500.
func TestARQ04_PonteEscopo_Vendas_PosseEFalhas(t *testing.T) {
	const (
		rePagamento = `SELECT ` + pagamentoColunasRegexH + pagamentoFromRegexH + ` WHERE pagamento_id = \? LIMIT 1`
		rePedido    = `SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`
		reItens     = `SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH
	)
	restritoAtivo := func(m sqlmock.Sqlmock) {
		m.ExpectQuery(reUsuarioVendedor).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(2)))
		expectVendedorDesligado(m, int64(2), false)
	}
	casos := []struct {
		nome, metodo, path, role, erro string
		status                         int
		preparar                       func(m sqlmock.Sqlmock)
	}{
		{"delete pagamento sem carteira", http.MethodDelete, "/api/pagamentos/1", "normal", "pagamento não encontrado", http.StatusNotFound,
			func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reUsuarioVendedor).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
			}},
		{"get pagamento: falha no vendedor do pedido", http.MethodGet, "/api/pagamentos/1", "normal", "pagamento não encontrado", http.StatusNotFound,
			func(m sqlmock.Sqlmock) {
				restritoAtivo(m)
				m.ExpectQuery(rePagamento).WithArgs(int64(1)).WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))
				m.ExpectQuery(rePedido).WithArgs(int64(1)).WillReturnError(errors.New("db down"))
			}},
		{"delete pagamento: falha no vendedor do pedido", http.MethodDelete, "/api/pagamentos/1", "normal", "pagamento não encontrado", http.StatusNotFound,
			func(m sqlmock.Sqlmock) {
				restritoAtivo(m)
				m.ExpectQuery(rePagamento).WithArgs(int64(1)).WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))
				m.ExpectQuery(rePedido).WithArgs(int64(1)).WillReturnError(errors.New("db down"))
			}},
		{"delete pagamento admin: falha ao excluir", http.MethodDelete, "/api/pagamentos/1", "admin", "erro interno", http.StatusInternalServerError,
			func(m sqlmock.Sqlmock) {
				m.ExpectQuery(rePagamento).WithArgs(int64(1)).WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))
			}},
		{"delete pedido admin: falha ao excluir", http.MethodDelete, "/api/pedidos/1", "admin", "erro interno", http.StatusInternalServerError,
			func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery(selectStatusForUpdateRegexH).WithArgs(int64(1)).WillReturnError(errors.New("db down"))
				m.ExpectRollback()
			}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			c.preparar(mock)
			req, _ := http.NewRequest(c.metodo, server.URL+c.path, nil)
			uid := map[string]int64{"admin": 1, "normal": 2}[c.role] // o checker fake devolve o role pelo id
			req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), uid, c.role))
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, c.status, resp.StatusCode)
			assert.Equal(t, c.erro, decodeResponse(t, readBody(t, resp))["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
