package handlers_test

// ARQ-04 (TestBrain): pedidos, pagamentos (ARQ-05: vendashttp, escopo pelo
// AutorizadorEscopo) e o dashboard legado (ponte handlers/scope.go) também
// precisam tratar "sem Principal no contexto" como SemAcesso, igual às rotas
// do Comercial (TestEscopo_SemUsuarioNoContexto): resposta vazia/404, SEM
// tocar no banco. Mata o mutante em que o escopo zero value vira "global"
// quando httpx.PrincipalFrom falha.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/rotaperfumes-api/internal/vendashttp"
)

func TestARQ04_PonteEscopo_SemPrincipal_SemAcessoSemSQL(t *testing.T) {
	type montar func(h *vendashttp.PedidoHandler, p *vendashttp.PagamentoHandler, d *handlers.DashboardHandler) http.HandlerFunc
	casos := []struct {
		nome       string
		metodo     string
		path       string
		id         string
		handler    montar
		wantStatus int
		wantErro   string // "" = 200 com data vazia (lista) ou zerada (dashboard)
		lista      bool
	}{
		{"ListPedidos", http.MethodGet, "/api/pedidos", "",
			func(h *vendashttp.PedidoHandler, _ *vendashttp.PagamentoHandler, _ *handlers.DashboardHandler) http.HandlerFunc {
				return h.ListPedidos
			}, http.StatusOK, "", true},
		{"GetPedido", http.MethodGet, "/api/pedidos/1", "1",
			func(h *vendashttp.PedidoHandler, _ *vendashttp.PagamentoHandler, _ *handlers.DashboardHandler) http.HandlerFunc {
				return h.GetPedido
			}, http.StatusNotFound, "pedido não encontrado", false},
		{"DeletePedido", http.MethodDelete, "/api/pedidos/1", "1",
			func(h *vendashttp.PedidoHandler, _ *vendashttp.PagamentoHandler, _ *handlers.DashboardHandler) http.HandlerFunc {
				return h.DeletePedido
			}, http.StatusNotFound, "pedido não encontrado", false},
		{"ListPagamentos", http.MethodGet, "/api/pagamentos", "",
			func(_ *vendashttp.PedidoHandler, p *vendashttp.PagamentoHandler, _ *handlers.DashboardHandler) http.HandlerFunc {
				return p.ListPagamentos
			}, http.StatusOK, "", true},
		{"GetPagamento", http.MethodGet, "/api/pagamentos/1", "1",
			func(_ *vendashttp.PedidoHandler, p *vendashttp.PagamentoHandler, _ *handlers.DashboardHandler) http.HandlerFunc {
				return p.GetPagamento
			}, http.StatusNotFound, "pagamento não encontrado", false},
		{"DeletePagamento", http.MethodDelete, "/api/pagamentos/1", "1",
			func(_ *vendashttp.PedidoHandler, p *vendashttp.PagamentoHandler, _ *handlers.DashboardHandler) http.HandlerFunc {
				return p.DeletePagamento
			}, http.StatusNotFound, "pagamento não encontrado", false},
		{"Dashboard GetMetrics", http.MethodGet, "/api/dashboard/metrics", "",
			func(_ *vendashttp.PedidoHandler, _ *vendashttp.PagamentoHandler, d *handlers.DashboardHandler) http.HandlerFunc {
				return d.GetMetrics
			}, http.StatusOK, "", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			// Sem expectativas: qualquer SQL falha (e vira 500).
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			defer db.Close()
			escopo := bootstrap.NovoComercial(db).Escopo()
			h := c.handler(
				novoPedidoHandler(db, testCfg()),
				novoPagamentoHandler(db, testCfg()),
				handlers.NewDashboardHandler(db, testCfg(), escopo),
			)

			r := httptest.NewRequest(c.metodo, c.path, nil) // sem Principal no contexto
			if c.id != "" {
				r.SetPathValue("id", c.id)
			}
			rec := httptest.NewRecorder()
			h(rec, r)

			assert.Equal(t, c.wantStatus, rec.Code, rec.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			if c.wantErro != "" {
				assert.Equal(t, c.wantErro, body["error"])
			}
			if c.lista {
				assert.Equal(t, []any{}, body["data"])
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "sem Principal não pode consultar o banco")
		})
	}
}
