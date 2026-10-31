package comercialhttp

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/comercial/comercialapp"
)

// ClienteHandler trata /api/clientes/* (Authenticated, escopo por carteira
// decidido pelos casos de uso).
type ClienteHandler struct{ casos *comercialapp.Casos }

// NovoClienteHandler exige os casos de uso (nil: panic na montagem).
func NovoClienteHandler(c *comercialapp.Casos) *ClienteHandler {
	if c == nil {
		panic("comercialhttp: NovoClienteHandler exige os casos de uso")
	}
	return &ClienteHandler{casos: c}
}

// ListClientes GET /api/clientes?page&limit&uf&segmento&ativo&q&order_by&order_dir
func (h *ClienteHandler) ListClientes(w http.ResponseWriter, r *http.Request) {
	const tag = "[clientes] ListClientes"
	filtro, err := filtroClienteDaQuery(r)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	res, err := h.casos.ListarClientes(r.Context(), principalDoEscopo(r), filtro, httpx.PaginaDaQuery(r))
	responderPaginado(w, tag, res, err, novoClienteJSON)
}

// GetCliente GET /api/clientes/{id}.
func (h *ClienteHandler) GetCliente(w http.ResponseWriter, r *http.Request) {
	const tag = "[clientes] GetCliente"
	if id, ok := idDoPath(w, r, tag); ok {
		c, err := h.casos.ObterCliente(r.Context(), principalDoEscopo(r), id)
		responder(w, tag, http.StatusOK, c, err, novoClienteJSON)
	}
}

// CreateCliente POST /api/clientes → 201 (o body só é lido depois do escopo).
func (h *ClienteHandler) CreateCliente(w http.ResponseWriter, r *http.Request) {
	const tag = "[clientes] CreateCliente"
	c, err := h.casos.CriarCliente(r.Context(), principalDoEscopo(r), leitor(r, dadosCliente))
	responder(w, tag, http.StatusCreated, c, err, novoClienteJSON)
}

// UpdateCliente PUT /api/clientes/{id} (posse antes do decode, E-05).
func (h *ClienteHandler) UpdateCliente(w http.ResponseWriter, r *http.Request) {
	const tag = "[clientes] UpdateCliente"
	if id, ok := idDoPath(w, r, tag); ok {
		c, err := h.casos.AtualizarCliente(r.Context(), principalDoEscopo(r), id, leitor(r, dadosCliente))
		responder(w, tag, http.StatusOK, c, err, novoClienteJSON)
	}
}

// ToggleAtivoCliente PATCH /api/clientes/{id}/inativar; body opcional
// {"ativo": bool} (omitido = toggle), lido depois da posse.
func (h *ClienteHandler) ToggleAtivoCliente(w http.ResponseWriter, r *http.Request) {
	const tag = "[clientes] ToggleAtivoCliente"
	if id, ok := idDoPath(w, r, tag); ok {
		lerAtivo := func() (*bool, error) { return httpx.LerAtivoOpcional(r) }
		c, err := h.casos.AlternarAtivoCliente(r.Context(), principalDoEscopo(r), id, lerAtivo)
		responder(w, tag, http.StatusOK, c, err, novoClienteJSON)
	}
}
