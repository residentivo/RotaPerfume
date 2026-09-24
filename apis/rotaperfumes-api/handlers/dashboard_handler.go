// Package handlers contém handlers HTTP da API.
package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

// DashboardHandler trata as rotas /api/dashboard/*.
type DashboardHandler struct {
	db  *sql.DB
	svc *services.DashboardService
}

// NewDashboardHandler cria um DashboardHandler com pool de conexão injetado.
func NewDashboardHandler(db *sql.DB, cfg *config.Config) *DashboardHandler {
	return &DashboardHandler{
		db:  db,
		svc: services.NewDashboardService(db, cfg),
	}
}

// GetMetrics GET /api/dashboard/metrics
// (escopo por vendedor: normal vê só os próprios números)
// Query params (opcionais): periodo (today|week|month), ano (YYYY), mes (1-12).
// Resposta: métricas gerais de vendas do dia, semana ou mes. Usuário normal
// sem vendedor vinculado recebe as métricas zeradas (mesmo formato).
func (h *DashboardHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	periodo := r.URL.Query().Get("periodo")
	if periodo == "" {
		periodo = "month"
	}
	if periodo != "today" && periodo != "week" && periodo != "month" {
		writeJSON(w, http.StatusBadRequest, nil, "periodo deve ser 'today', 'week' ou 'month'")
		return
	}

	scope, ok := h.resolverEscopo(w, r, "GetMetrics")
	if !ok {
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusOK, h.svc.EmptyMetrics(periodo), "")
		return
	}

	metrics, err := h.svc.GetMetrics(r.Context(), h.db, periodo, scope.VendedorID)
	if err != nil {
		log.Printf("[dashboard] GetMetrics: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, metrics, "")
}

// GetVendas GET /api/dashboard/vendas
// (escopo por vendedor: normal vê só os próprios números)
// Query params (opcionais): dias (default 30, max 365).
// Resposta: serie temporal de vendas dos últimos N dias. Usuário normal sem
// vendedor vinculado recebe a série com todos os dias zerados.
func (h *DashboardHandler) GetVendas(w http.ResponseWriter, r *http.Request) {
	dias := 30
	if d := r.URL.Query().Get("dias"); d != "" {
		if n := parseIntDefault(d, 30); n > 0 && n <= 365 {
			dias = n
		}
	}

	scope, ok := h.resolverEscopo(w, r, "GetVendas")
	if !ok {
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusOK, h.svc.EmptyVendasSeries(dias), "")
		return
	}

	series, err := h.svc.GetVendasSeries(r.Context(), h.db, dias, scope.VendedorID)
	if err != nil {
		log.Printf("[dashboard] GetVendas: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, series, "")
}

// GetVendedores GET /api/dashboard/vendedores
// (escopo por vendedor: normal vê só os próprios números)
// Query params (opcionais): page (default 1), limit (default 20, max 100).
// Resposta: ranking de vendedores com vendas, meta e percentual. Usuário
// normal recebe só a própria linha; sem vendedor vinculado, lista vazia
// (total=0, pages=0).
func (h *DashboardHandler) GetVendedores(w http.ResponseWriter, r *http.Request) {
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	scope, ok := h.resolverEscopo(w, r, "GetVendedores")
	if !ok {
		return
	}
	if scope.SemAcesso() {
		writeJSONWithPagination(w, http.StatusOK, []map[string]any{}, page, limit, 0, 0)
		return
	}

	vendedores, total, err := h.svc.GetVendedoresRanking(r.Context(), h.db, page, limit, scope.VendedorID)
	if err != nil {
		log.Printf("[dashboard] GetVendedores: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, vendedores, page, limit, total, pages)
}

// GetClientes GET /api/dashboard/clientes
// Query params (opcionais): periodo (today|week|month).
// Resposta: {periodo, total_clientes, total_ativos, total_inativos,
//
//	novos_no_periodo, por_segmento: [{segmento, total}], por_uf: [{uf, total}]}
func (h *DashboardHandler) GetClientes(w http.ResponseWriter, r *http.Request) {
	periodo := r.URL.Query().Get("periodo")
	if periodo == "" {
		periodo = "month"
	}
	if periodo != "today" && periodo != "week" && periodo != "month" {
		writeJSON(w, http.StatusBadRequest, nil, "periodo deve ser 'today', 'week' ou 'month'")
		return
	}

	metrics, err := h.svc.GetClienteMetrics(r.Context(), h.db, periodo)
	if err != nil {
		log.Printf("[dashboard] GetClientes: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, metrics, "")
}

// resolverEscopo resolve o escopo de vendedor da requisição. Em caso de
// erro, registra log, responde 500 e retorna ok=false.
func (h *DashboardHandler) resolverEscopo(w http.ResponseWriter, r *http.Request, acao string) (vendedorScope, bool) {
	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[dashboard] %s: %v", acao, err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return vendedorScope{}, false
	}
	if h.svc.Cfg.Verbose {
		log.Printf("[dashboard] %s escopo restrito=%t vendedor_id=%d", acao, scope.Restrito, scope.VendedorID)
	}
	return scope, true
}

func parseIntDefault(s string, fallback int) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return fallback
		}
	}
	return n
}
