package comercialhttp

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/comercial/comercialapp"
)

// VisitaHandler trata /api/visitas/* (Authenticated, escopo por carteira).
type VisitaHandler struct{ casos *comercialapp.Casos }

// NovoVisitaHandler exige os casos de uso (nil: panic na montagem).
func NovoVisitaHandler(c *comercialapp.Casos) *VisitaHandler {
	if c == nil {
		panic("comercialhttp: NovoVisitaHandler exige os casos de uso")
	}
	return &VisitaHandler{casos: c}
}

// ListVisitas GET /api/visitas?page&limit&cliente_id&vendedor_id&resultado&data_visita_de&data_visita_ate&q&order_by&order_dir
func (h *VisitaHandler) ListVisitas(w http.ResponseWriter, r *http.Request) {
	const tag = "[visitas] ListVisitas"
	filtro, err := filtroVisitaDaQuery(r)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	res, err := h.casos.ListarVisitas(r.Context(), principalDoEscopo(r), filtro, httpx.PaginaDaQuery(r))
	responderPaginado(w, tag, res, err, novoVisitaJSON)
}

// GetVisita GET /api/visitas/{id}.
func (h *VisitaHandler) GetVisita(w http.ResponseWriter, r *http.Request) {
	const tag = "[visitas] GetVisita"
	if id, ok := idDoPath(w, r, tag); ok {
		v, err := h.casos.ObterVisita(r.Context(), principalDoEscopo(r), id)
		responder(w, tag, http.StatusOK, v, err, novoVisitaJSON)
	}
}

// CreateVisita POST /api/visitas → 201.
func (h *VisitaHandler) CreateVisita(w http.ResponseWriter, r *http.Request) {
	const tag = "[visitas] CreateVisita"
	v, err := h.casos.CriarVisita(r.Context(), principalDoEscopo(r), leitor(r, dadosVisita))
	responder(w, tag, http.StatusCreated, v, err, novoVisitaJSON)
}

// UpdateVisita PUT /api/visitas/{id}.
func (h *VisitaHandler) UpdateVisita(w http.ResponseWriter, r *http.Request) {
	const tag = "[visitas] UpdateVisita"
	if id, ok := idDoPath(w, r, tag); ok {
		v, err := h.casos.AtualizarVisita(r.Context(), principalDoEscopo(r), id, leitor(r, dadosVisita))
		responder(w, tag, http.StatusOK, v, err, novoVisitaJSON)
	}
}

// DeleteVisita DELETE /api/visitas/{id} → 204.
func (h *VisitaHandler) DeleteVisita(w http.ResponseWriter, r *http.Request) {
	const tag = "[visitas] DeleteVisita"
	if id, ok := idDoPath(w, r, tag); ok {
		if err := h.casos.ExcluirVisita(r.Context(), principalDoEscopo(r), id); err != nil {
			httpx.ResponderErro(w, tag, err)
			return
		}
		httpx.SemConteudo(w)
	}
}
