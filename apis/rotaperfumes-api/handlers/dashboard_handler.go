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
// Query params (opcionais): periodo (today|month), ano (YYYY), mes (1-12).
// Resposta: métricas gerais de vendas do dia ou mes.
func (h *DashboardHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	periodo := r.URL.Query().Get("periodo")
	if periodo == "" {
		periodo = "month"
	}
	if periodo != "today" && periodo != "month" {
		writeJSON(w, http.StatusBadRequest, nil, "periodo deve ser 'today' ou 'month'")
		return
	}

	metrics, err := h.svc.GetMetrics(r.Context(), h.db, periodo)
	if err != nil {
		log.Printf("[dashboard] GetMetrics: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, metrics, "")
}

// GetVendas GET /api/dashboard/vendas
// Query params (opcionais): dias (default 30, max 365).
// Resposta: serie temporal de vendas dos últimos N dias.
func (h *DashboardHandler) GetVendas(w http.ResponseWriter, r *http.Request) {
	dias := 30
	if d := r.URL.Query().Get("dias"); d != "" {
		if n := parseIntDefault(d, 30); n > 0 && n <= 365 {
			dias = n
		}
	}

	series, err := h.svc.GetVendasSeries(r.Context(), h.db, dias)
	if err != nil {
		log.Printf("[dashboard] GetVendas: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, series, "")
}

// GetVendedores GET /api/dashboard/vendedores
// Query params (opcionais): page (default 1), limit (default 20, max 100).
// Resposta: ranking de vendedores com vendas, meta e percentual.
func (h *DashboardHandler) GetVendedores(w http.ResponseWriter, r *http.Request) {
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	vendedores, total, err := h.svc.GetVendedoresRanking(r.Context(), h.db, page, limit)
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
