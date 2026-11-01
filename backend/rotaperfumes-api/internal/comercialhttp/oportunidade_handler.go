package comercialhttp

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/comercial/comercialapp"
)

// OportunidadeHandler trata /api/oportunidades/* (Authenticated, escopo por
// carteira).
type OportunidadeHandler struct{ casos *comercialapp.Casos }

// NovoOportunidadeHandler exige os casos de uso (nil: panic na montagem).
func NovoOportunidadeHandler(c *comercialapp.Casos) *OportunidadeHandler {
	if c == nil {
		panic("comercialhttp: NovoOportunidadeHandler exige os casos de uso")
	}
	return &OportunidadeHandler{casos: c}
}

// ListOportunidades GET /api/oportunidades?page&limit&cliente_id&vendedor_id&etapa&origem&data_abertura_de&data_abertura_ate&q&order_by&order_dir
func (h *OportunidadeHandler) ListOportunidades(w http.ResponseWriter, r *http.Request) {
	const tag = "[oportunidades] ListOportunidades"
	filtro, err := filtroOportunidadeDaQuery(r)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	res, err := h.casos.ListarOportunidades(r.Context(), principalDoEscopo(r), filtro, httpx.PaginaDaQuery(r))
	responderPaginado(w, tag, res, err, novoOportunidadeJSON)
}

// GetOportunidade GET /api/oportunidades/{id}.
func (h *OportunidadeHandler) GetOportunidade(w http.ResponseWriter, r *http.Request) {
	const tag = "[oportunidades] GetOportunidade"
	if id, ok := idDoPath(w, r, tag); ok {
		o, err := h.casos.ObterOportunidade(r.Context(), principalDoEscopo(r), id)
		responder(w, tag, http.StatusOK, o, err, novoOportunidadeJSON)
	}
}

// CreateOportunidade POST /api/oportunidades → 201.
func (h *OportunidadeHandler) CreateOportunidade(w http.ResponseWriter, r *http.Request) {
	const tag = "[oportunidades] CreateOportunidade"
	o, err := h.casos.CriarOportunidade(r.Context(), principalDoEscopo(r), leitor(r, dadosOportunidade))
	responder(w, tag, http.StatusCreated, o, err, novoOportunidadeJSON)
}

// UpdateOportunidade PUT /api/oportunidades/{id}.
func (h *OportunidadeHandler) UpdateOportunidade(w http.ResponseWriter, r *http.Request) {
	const tag = "[oportunidades] UpdateOportunidade"
	if id, ok := idDoPath(w, r, tag); ok {
		o, err := h.casos.AtualizarOportunidade(r.Context(), principalDoEscopo(r), id, leitor(r, dadosOportunidade))
		responder(w, tag, http.StatusOK, o, err, novoOportunidadeJSON)
	}
}

// DeleteOportunidade DELETE /api/oportunidades/{id} → 204.
func (h *OportunidadeHandler) DeleteOportunidade(w http.ResponseWriter, r *http.Request) {
	const tag = "[oportunidades] DeleteOportunidade"
	if id, ok := idDoPath(w, r, tag); ok {
		if err := h.casos.ExcluirOportunidade(r.Context(), principalDoEscopo(r), id); err != nil {
			httpx.ResponderErro(w, tag, err)
			return
		}
		httpx.SemConteudo(w)
	}
}
