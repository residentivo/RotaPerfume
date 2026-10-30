package catalogohttp

import (
	"log"
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
)

// EstoqueHandler trata /api/estoque/* (tudo AdminOnly, C-01, aplicado pela
// rota).
type EstoqueHandler struct{ casos *catalogoapp.Casos }

// NovoEstoqueHandler exige os casos de uso (nil: panic na montagem).
func NovoEstoqueHandler(c *catalogoapp.Casos) *EstoqueHandler {
	if c == nil {
		panic("catalogohttp: NovoEstoqueHandler exige os casos de uso")
	}
	return &EstoqueHandler{casos: c}
}

// ListEstoque GET /api/estoque?page&limit&sku&data_de&data_ate&ruptura&historico&order_by&order_dir
// (padrão: última posição de cada SKU; historico=true: série completa).
func (h *EstoqueHandler) ListEstoque(w http.ResponseWriter, r *http.Request) {
	res, err := h.casos.ListarEstoque(r.Context(), consultaEstoqueDaQuery(r), httpx.PaginaDaQuery(r))
	if err != nil {
		httpx.ResponderErro(w, "[estoque] ListEstoque", err)
		return
	}
	httpx.JSONPaginadoCom(w, res, estoqueParaResposta)
}

// GetEstoque GET /api/estoque/{id}
func (h *EstoqueHandler) GetEstoque(w http.ResponseWriter, r *http.Request) {
	const tag = "[estoque] GetEstoque"
	id, err := httpx.IDDoPath(r, "id")
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	e, err := h.casos.ObterEstoque(r.Context(), id)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	httpx.JSON(w, http.StatusOK, estoqueParaResposta(*e), "")
}

// CreateEstoque POST /api/estoque (ajuste manual) → 201.
func (h *EstoqueHandler) CreateEstoque(w http.ResponseWriter, r *http.Request) {
	const tag = "[estoque] CreateEstoque"
	principal, ok := principalOuErro(w, r, tag)
	if !ok {
		return
	}
	var req criarEstoqueRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	e, err := h.casos.AjustarEstoque(r.Context(), catalogo.DadosEstoque(req))
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	log.Printf("[estoque] criado manualmente: id=%d sku=%q por usuario role=%s", e.ID, e.SKU, principal.Role())
	httpx.JSON(w, http.StatusCreated, estoqueParaResposta(*e), "")
}

// UpdateEstoque PUT /api/estoque/{id}; body {"saldo": number}.
func (h *EstoqueHandler) UpdateEstoque(w http.ResponseWriter, r *http.Request) {
	const tag = "[estoque] UpdateEstoque"
	principal, ok := principalOuErro(w, r, tag)
	if !ok {
		return
	}
	id, err := httpx.IDDoPath(r, "id")
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	var req atualizarEstoqueRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	e, err := h.casos.AtualizarSaldo(r.Context(), id, req.Saldo)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	log.Printf("[estoque] atualizado manualmente: id=%d por usuario role=%s", id, principal.Role())
	httpx.JSON(w, http.StatusOK, estoqueParaResposta(*e), "")
}
