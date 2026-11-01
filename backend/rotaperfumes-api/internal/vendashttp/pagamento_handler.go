package vendashttp

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
)

// PagamentoHandler trata /api/pagamentos/* (Authenticated; escopo herdado
// do pedido, V-02). O nome do tipo e dos métodos é o do legado (rótulo do
// vlog de rota).
type PagamentoHandler struct{ casos *vendasapp.Casos }

// NovoPagamentoHandler exige os casos de uso (nil: panic na montagem).
func NovoPagamentoHandler(c *vendasapp.Casos) *PagamentoHandler {
	if c == nil {
		panic("vendashttp: NovoPagamentoHandler exige os casos de uso")
	}
	return &PagamentoHandler{casos: c}
}

// ListPagamentos GET /api/pagamentos?page&limit&status_pagamento&forma_pagamento&pedido_id&vencimento_de&vencimento_ate&order_by&order_dir
func (h *PagamentoHandler) ListPagamentos(w http.ResponseWriter, r *http.Request) {
	const tag = "[pagamentos] ListPagamentos"
	res, err := h.casos.ListarPagamentos(r.Context(), principalDoEscopo(r), leitorFiltroPagamento(r), httpx.PaginaDaQuery(r))
	httpx.ResponderPaginado(w, tag, res, err, novoPagamentoJSON)
}

// GetPagamento GET /api/pagamentos/{id}.
func (h *PagamentoHandler) GetPagamento(w http.ResponseWriter, r *http.Request) {
	const tag = "[pagamentos] GetPagamento"
	if id, ok := httpx.IDOuErro(w, r, tag, "id"); ok {
		p, err := h.casos.ObterPagamento(r.Context(), principalDoEscopo(r), id)
		httpx.Responder(w, tag, http.StatusOK, p, err, novoPagamentoJSON)
	}
}

// CreatePagamento POST /api/pagamentos → 201 (valor_liquido explícito).
func (h *PagamentoHandler) CreatePagamento(w http.ResponseWriter, r *http.Request) {
	const tag = "[pagamentos] CreatePagamento"
	p, err := h.casos.CriarPagamento(r.Context(), principalDoEscopo(r), httpx.LeitorJSON(r, dadosNovoPagamento))
	httpx.Responder(w, tag, http.StatusCreated, p, err, novoPagamentoJSON)
}

// UpdatePagamento PUT /api/pagamentos/{id} (pedido_id não é editável, V-02).
func (h *PagamentoHandler) UpdatePagamento(w http.ResponseWriter, r *http.Request) {
	const tag = "[pagamentos] UpdatePagamento"
	if id, ok := httpx.IDOuErro(w, r, tag, "id"); ok {
		p, err := h.casos.AtualizarPagamento(r.Context(), principalDoEscopo(r), id, httpx.LeitorJSON(r, dadosPagamento))
		httpx.Responder(w, tag, http.StatusOK, p, err, novoPagamentoJSON)
	}
}

// DeletePagamento DELETE /api/pagamentos/{id} → 204 (409 se quitado).
func (h *PagamentoHandler) DeletePagamento(w http.ResponseWriter, r *http.Request) {
	const tag = "[pagamentos] DeletePagamento"
	if id, ok := httpx.IDOuErro(w, r, tag, "id"); ok {
		httpx.ResponderSemConteudo(w, tag, h.casos.ExcluirPagamento(r.Context(), principalDoEscopo(r), id))
	}
}
