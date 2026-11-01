package comercialhttp

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/comercial/comercialapp"
	"github.com/rotaperfumes/shared/kernel"
)

// errClienteIDPathInvalido é o 400 do {clienteId} não numérico.
var errClienteIDPathInvalido = kernel.NovoErro(kernel.KindInvalido, "cliente id inválido")

// VendedorHandler trata /api/vendedores/* (lista e carteira Authenticated
// com escopo; gestão AdminOnly). O nome do tipo e dos métodos é o rótulo do
// vlog de rota, igual ao legado.
type VendedorHandler struct{ casos *comercialapp.Casos }

// NovoVendedorHandler exige os casos de uso (nil: panic na montagem).
func NovoVendedorHandler(c *comercialapp.Casos) *VendedorHandler {
	if c == nil {
		panic("comercialhttp: NovoVendedorHandler exige os casos de uso")
	}
	return &VendedorHandler{casos: c}
}

// ListVendedores GET /api/vendedores (sem paginação: lista de seleção).
func (h *VendedorHandler) ListVendedores(w http.ResponseWriter, r *http.Request) {
	const tag = "[vendedores] ListVendedores"
	vs, err := h.casos.ListarVendedores(r.Context(), principalDoEscopo(r))
	responderLista(w, tag, vs, err, novoVendedorResumoJSON)
}

// GetVendedor GET /api/vendedores/{id} (com os clientes da carteira ativa).
func (h *VendedorHandler) GetVendedor(w http.ResponseWriter, r *http.Request) {
	const tag = "[vendedores] GetVendedor"
	if p, id, ok := principalEID(w, r, tag); ok {
		d, err := h.casos.ObterVendedor(r.Context(), p, id)
		responder(w, tag, http.StatusOK, d, err, novoDetalheVendedorJSON)
	}
}

// ListClientesDoVendedor GET /api/vendedores/{id}/clientes.
func (h *VendedorHandler) ListClientesDoVendedor(w http.ResponseWriter, r *http.Request) {
	const tag = "[vendedores] ListClientesDoVendedor"
	if id, ok := idDoPath(w, r, tag); ok {
		cs, err := h.casos.ClientesDoVendedor(r.Context(), principalDoEscopo(r), id)
		responderLista(w, tag, cs, err, novoClienteResumoJSON)
	}
}

// CreateVendedor POST /api/vendedores → 201.
func (h *VendedorHandler) CreateVendedor(w http.ResponseWriter, r *http.Request) {
	const tag = "[vendedores] CreateVendedor"
	p, ok := principal(w, r, tag)
	if !ok {
		return
	}
	d, err := leitor(r, dadosVendedor)()
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	v, err := h.casos.CriarVendedor(r.Context(), p, d)
	responder(w, tag, http.StatusCreated, v, err, novoVendedorJSON)
}

// UpdateVendedor PUT /api/vendedores/{id} (data_desligamento não é editável).
func (h *VendedorHandler) UpdateVendedor(w http.ResponseWriter, r *http.Request) {
	const tag = "[vendedores] UpdateVendedor"
	p, id, ok := principalEID(w, r, tag)
	if !ok {
		return
	}
	d, err := leitor(r, dadosVendedor)()
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	v, err := h.casos.AtualizarVendedor(r.Context(), p, id, d)
	responder(w, tag, http.StatusOK, v, err, novoVendedorJSON)
}

// DeleteVendedor DELETE /api/vendedores/{id}: soft-delete (I-09).
func (h *VendedorHandler) DeleteVendedor(w http.ResponseWriter, r *http.Request) {
	const tag = "[vendedores] DeleteVendedor"
	if p, id, ok := principalEID(w, r, tag); ok {
		v, err := h.casos.DesligarVendedor(r.Context(), p, id)
		responder(w, tag, http.StatusOK, v, err, novoVendedorJSON)
	}
}

// ReativarVendedor POST /api/vendedores/{id}/reativar.
func (h *VendedorHandler) ReativarVendedor(w http.ResponseWriter, r *http.Request) {
	const tag = "[vendedores] ReativarVendedor"
	if p, id, ok := principalEID(w, r, tag); ok {
		v, err := h.casos.ReativarVendedor(r.Context(), p, id)
		responder(w, tag, http.StatusOK, v, err, novoVendedorJSON)
	}
}

// VincularCliente POST /api/vendedores/{id}/clientes → 201 com o resumo.
func (h *VendedorHandler) VincularCliente(w http.ResponseWriter, r *http.Request) {
	const tag = "[vendedores] VincularCliente"
	p, id, ok := principalEID(w, r, tag)
	if !ok {
		return
	}
	var req vincularClienteRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	c, err := h.casos.VincularCliente(r.Context(), p, id, req.ClienteID)
	responder(w, tag, http.StatusCreated, c, err, novoClienteResumoJSON)
}

// DesvincularCliente DELETE /api/vendedores/{id}/clientes/{clienteId}.
func (h *VendedorHandler) DesvincularCliente(w http.ResponseWriter, r *http.Request) {
	const tag = "[vendedores] DesvincularCliente"
	p, id, ok := principalEID(w, r, tag)
	if !ok {
		return
	}
	clienteID, err := httpx.IDDoPath(r, "clienteId")
	if err != nil {
		httpx.ResponderErro(w, tag, errClienteIDPathInvalido)
		return
	}
	if err := h.casos.DesvincularCliente(r.Context(), p, id, clienteID); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil, "")
}
