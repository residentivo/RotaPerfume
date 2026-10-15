package vendashttp

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
)

// PedidoHandler trata /api/pedidos/* (Authenticated, escopo por carteira).
// O nome do tipo e dos métodos é o do legado: é o rótulo do vlog de rota.
type PedidoHandler struct{ casos *vendasapp.Casos }

// NovoPedidoHandler exige os casos de uso (nil: panic na montagem).
func NovoPedidoHandler(c *vendasapp.Casos) *PedidoHandler {
	if c == nil {
		panic("vendashttp: NovoPedidoHandler exige os casos de uso")
	}
	return &PedidoHandler{casos: c}
}

// ListPedidos GET /api/pedidos?page&limit&status&canal&cliente_id&vendedor_id&data_inicio&data_fim&q&order_by&order_dir
func (h *PedidoHandler) ListPedidos(w http.ResponseWriter, r *http.Request) {
	const tag = "[pedidos] ListPedidos"
	res, err := h.casos.ListarPedidos(r.Context(), principalDoEscopo(r), filtroPedidoDaQuery(r), httpx.PaginaDaQuery(r))
	httpx.ResponderPaginado(w, tag, res, err, novoPedidoJSON)
}

// GetPedido GET /api/pedidos/{id} → o pedido com os itens.
func (h *PedidoHandler) GetPedido(w http.ResponseWriter, r *http.Request) {
	const tag = "[pedidos] GetPedido"
	if id, ok := httpx.IDOuErro(w, r, tag, "id"); ok {
		p, err := h.casos.ObterPedido(r.Context(), principalDoEscopo(r), id)
		httpx.Responder(w, tag, http.StatusOK, p, err, novoPedidoDetalheJSON)
	}
}

// CreatePedido POST /api/pedidos → 201 (valores calculados no backend).
func (h *PedidoHandler) CreatePedido(w http.ResponseWriter, r *http.Request) {
	const tag = "[pedidos] CreatePedido"
	p, err := h.casos.CriarPedido(r.Context(), principalDoEscopo(r), httpx.LeitorJSON(r, dadosPedido))
	httpx.Responder(w, tag, http.StatusCreated, p, err, novoPedidoDetalheJSON)
}

// UpdatePedido PUT /api/pedidos/{id}: itens substituídos integralmente.
func (h *PedidoHandler) UpdatePedido(w http.ResponseWriter, r *http.Request) {
	const tag = "[pedidos] UpdatePedido"
	if id, ok := httpx.IDOuErro(w, r, tag, "id"); ok {
		p, err := h.casos.AtualizarPedido(r.Context(), principalDoEscopo(r), id, httpx.LeitorJSON(r, dadosPedido))
		httpx.Responder(w, tag, http.StatusOK, p, err, novoPedidoDetalheJSON)
	}
}

// DeletePedido DELETE /api/pedidos/{id} → 204 (409 com pagamentos ou
// Faturado, V-01).
func (h *PedidoHandler) DeletePedido(w http.ResponseWriter, r *http.Request) {
	const tag = "[pedidos] DeletePedido"
	if id, ok := httpx.IDOuErro(w, r, tag, "id"); ok {
		httpx.ResponderSemConteudo(w, tag, h.casos.ExcluirPedido(r.Context(), principalDoEscopo(r), id))
	}
}
