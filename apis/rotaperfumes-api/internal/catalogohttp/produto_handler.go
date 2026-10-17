package catalogohttp

import (
	"log"
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/kernel"
)

// ProdutoHandler trata /api/produtos/* (leitura Authenticated; escrita
// AdminOnly, C-01, aplicado pela rota). O nome do tipo e dos métodos é o
// rótulo do vlog de rota, igual ao legado.
type ProdutoHandler struct{ casos *catalogoapp.Casos }

// NovoProdutoHandler exige os casos de uso (nil: panic na montagem).
func NovoProdutoHandler(c *catalogoapp.Casos) *ProdutoHandler {
	if c == nil {
		panic("catalogohttp: NovoProdutoHandler exige os casos de uso")
	}
	return &ProdutoHandler{casos: c}
}

// ListProdutos GET /api/produtos?page&limit&categoria&marca&ativo&q&order_by&order_dir
func (h *ProdutoHandler) ListProdutos(w http.ResponseWriter, r *http.Request) {
	res, err := h.casos.ListarProdutos(r.Context(), filtroProdutoDaQuery(r), httpx.PaginaDaQuery(r))
	if err != nil {
		httpx.ResponderErro(w, "[produtos] ListProdutos", err)
		return
	}
	httpx.JSONPaginadoCom(w, res, produtoParaResposta)
}

// GetProduto GET /api/produtos/{id}
func (h *ProdutoHandler) GetProduto(w http.ResponseWriter, r *http.Request) {
	const tag = "[produtos] GetProduto"
	id, err := httpx.IDDoPathPermissivo(r, "id")
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, err := h.casos.ObterProduto(r.Context(), id)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	httpx.JSON(w, http.StatusOK, produtoParaResposta(*p), "")
}

// CreateProduto POST /api/produtos (o produto nasce ativo) → 201.
func (h *ProdutoHandler) CreateProduto(w http.ResponseWriter, r *http.Request) {
	const tag = "[produtos] CreateProduto"
	principal, ok := principalOuErro(w, r, tag)
	if !ok {
		return
	}
	var req criarProdutoRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, err := h.casos.CriarProduto(r.Context(), req.paraDados())
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	log.Printf("[produtos] criado: id=%d por usuario role=%s", p.ID, principal.Role())
	httpx.JSON(w, http.StatusCreated, produtoParaResposta(*p), "")
}

// UpdateProduto PUT /api/produtos/{id} (sku e ativo não são editáveis).
func (h *ProdutoHandler) UpdateProduto(w http.ResponseWriter, r *http.Request) {
	const tag = "[produtos] UpdateProduto"
	principal, ok := principalOuErro(w, r, tag)
	if !ok {
		return
	}
	id, err := httpx.IDDoPathPermissivo(r, "id")
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	var req atualizarProdutoRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, err := h.casos.AtualizarProduto(r.Context(), id, req.paraDados())
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	log.Printf("[produtos] atualizado: id=%d por usuario role=%s", id, principal.Role())
	httpx.JSON(w, http.StatusOK, produtoParaResposta(*p), "")
}

// ToggleAtivoProduto PATCH /api/produtos/{id}/inativar; body opcional
// {"ativo": bool} (omitido = toggle).
func (h *ProdutoHandler) ToggleAtivoProduto(w http.ResponseWriter, r *http.Request) {
	const tag = "[produtos] ToggleAtivoProduto"
	principal, ok := principalOuErro(w, r, tag)
	if !ok {
		return
	}
	id, err := httpx.IDDoPathPermissivo(r, "id")
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	ativo, err := httpx.LerAtivoOpcional(r)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, err := h.casos.AlternarAtivoProduto(r.Context(), id, ativo)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	log.Printf("[produtos] ativo=%t: id=%d por usuario role=%s", p.Ativo, id, principal.Role())
	httpx.JSON(w, http.StatusOK, produtoParaResposta(*p), "")
}

// principalOuErro devolve o Principal do request autenticado. Em rota
// protegida ele sempre existe (o Autenticador falha fechado antes); se
// faltar, responde 500 "erro interno" sem chamar o caso de uso (§9).
func principalOuErro(w http.ResponseWriter, r *http.Request, tag string) (kernel.Principal, bool) {
	p, ok := httpx.PrincipalFrom(r)
	if !ok {
		httpx.ResponderErro(w, tag, kernel.ErrPrincipalInvalido)
	}
	return p, ok
}
