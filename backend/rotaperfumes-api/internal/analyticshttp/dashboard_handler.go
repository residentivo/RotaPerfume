// Package analyticshttp é a camada HTTP do contexto Analytics (ARQ-06):
// handlers finos sobre as consultas do analyticsapp e os DTOs do contrato.
package analyticshttp

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsapp"
	"github.com/rotaperfumes/shared/kernel"
)

// DashboardHandler trata /api/dashboard/* (Authenticated; escopo por
// vendedor: o normal vê só os próprios números; desligado, sem vínculo ou
// órfão recebem 200 zerado, D-01). O nome do tipo e dos métodos é o do
// legado (rótulo do vlog de rota).
type DashboardHandler struct{ consultas *analyticsapp.Consultas }

// NovoDashboardHandler exige as consultas (nil: panic na montagem).
func NovoDashboardHandler(c *analyticsapp.Consultas) *DashboardHandler {
	if c == nil {
		panic("analyticshttp: NovoDashboardHandler exige as consultas")
	}
	return &DashboardHandler{consultas: c}
}

// GetMetrics GET /api/dashboard/metrics?periodo=today|week|month (default
// month; inválido → 400 antes de resolver o escopo).
func (h *DashboardHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	const tag = "[dashboard] GetMetrics"
	periodo, err := analytics.ParsePeriodo(r.URL.Query().Get("periodo"))
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	m, err := h.consultas.Metricas(r.Context(), analyticsapp.ConsultaMetricas{Principal: principalDoEscopo(r), Periodo: periodo})
	httpx.Responder(w, tag, http.StatusOK, &m, err, metricasJSON)
}

// GetVendas GET /api/dashboard/vendas?dias=N (1..365; fora disso, 30).
func (h *DashboardHandler) GetVendas(w http.ResponseWriter, r *http.Request) {
	const tag = "[dashboard] GetVendas"
	q := analyticsapp.ConsultaSerieVendas{Principal: principalDoEscopo(r), Dias: analytics.ParseDias(r.URL.Query().Get("dias"))}
	s, err := h.consultas.SerieVendas(r.Context(), q)
	httpx.Responder(w, tag, http.StatusOK, &s, err, serieJSON)
}

// GetVendedores GET /api/dashboard/vendedores?page&limit (ranking paginado).
func (h *DashboardHandler) GetVendedores(w http.ResponseWriter, r *http.Request) {
	const tag = "[dashboard] GetVendedores"
	res, err := h.consultas.Ranking(r.Context(), analyticsapp.ConsultaRanking{Principal: principalDoEscopo(r), Pagina: httpx.PaginaDaQuery(r)})
	httpx.ResponderPaginado(w, tag, res, err, linhaRankingJSON)
}

// GetClientes GET /api/dashboard/clientes?periodo=today|week|month (default
// month; inválido → 400 antes de resolver o escopo).
func (h *DashboardHandler) GetClientes(w http.ResponseWriter, r *http.Request) {
	const tag = "[dashboard] GetClientes"
	periodo, err := analytics.ParsePeriodo(r.URL.Query().Get("periodo"))
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	m, err := h.consultas.MetricasClientes(r.Context(), analyticsapp.ConsultaMetricasClientes{Principal: principalDoEscopo(r), Periodo: periodo})
	httpx.Responder(w, tag, http.StatusOK, &m, err, metricasClientesJSON)
}

// GetFunil GET /api/dashboard/funil?janela=30d|90d|365d|all (DASH-01;
// default all; inválido → 400 antes de resolver o escopo). O escopo vem só
// do Principal: não há ?vendedor_id=.
func (h *DashboardHandler) GetFunil(w http.ResponseWriter, r *http.Request) {
	const tag = "[dashboard] GetFunil"
	janela, err := analytics.ParseJanela(r.URL.Query().Get("janela"))
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	f, err := h.consultas.Funil(r.Context(), analyticsapp.ConsultaFunil{Principal: principalDoEscopo(r), Janela: janela})
	httpx.Responder(w, tag, http.StatusOK, &f, err, funilJSON)
}

// principalDoEscopo é o Principal das rotas com escopo de carteira. A falta
// dele NÃO vira 500 aqui: o autorizador trata o Principal inválido como
// SemAcesso, sem consulta (lição 2 do ARQ-04; resposta zerada).
func principalDoEscopo(r *http.Request) kernel.Principal {
	p, _ := httpx.PrincipalFrom(r)
	return p
}
