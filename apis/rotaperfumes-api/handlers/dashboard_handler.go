// Package handlers contém handlers HTTP da API.
package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/repositories"
)

// DashboardHandler trata as rotas /api/dashboard/*.
type DashboardHandler struct {
	db           *sql.DB
	svc          *services.DashboardService
	vendedorRepo *repositories.VendedorRepository
}

// NewDashboardHandler cria um DashboardHandler com pool de conexão injetado.
func NewDashboardHandler(db *sql.DB, cfg *config.Config) *DashboardHandler {
	return &DashboardHandler{
		db:           db,
		svc:          services.NewDashboardService(db, cfg),
		vendedorRepo: repositories.NewVendedorRepository(),
	}
}

// GetMetrics GET /api/dashboard/metrics
// (escopo por vendedor: normal vê só os próprios números)
// Query params (opcionais): periodo (today|week|month), ano (YYYY), mes (1-12).
// Resposta: métricas gerais de vendas do dia, semana ou mes, mais o flag
// vendedor_desligado. Usuário normal sem vendedor vinculado (ou com vendedor
// desligado) recebe as métricas zeradas (mesmo formato); vendedor_desligado
// é true somente no bloqueio por desligamento.
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
		writeJSON(w, http.StatusOK, h.svc.EmptyMetrics(periodo, scope.Desligado), "")
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
// (escopo por carteira: normal vê só os clientes da própria carteira ativa)
// Query params (opcionais): periodo (today|week|month).
// Resposta: {periodo, total_clientes, total_ativos, total_inativos,
//
//	novos_no_periodo, por_segmento: [{segmento, total}], por_uf: [{uf, total}]}
//
// Usuário normal sem acesso (sem vendedor ou vendedor desligado) recebe as
// contagens zeradas e listas vazias ([]).
func (h *DashboardHandler) GetClientes(w http.ResponseWriter, r *http.Request) {
	periodo := r.URL.Query().Get("periodo")
	if periodo == "" {
		periodo = "month"
	}
	if periodo != "today" && periodo != "week" && periodo != "month" {
		writeJSON(w, http.StatusBadRequest, nil, "periodo deve ser 'today', 'week' ou 'month'")
		return
	}

	scope, ok := h.resolverEscopo(w, r, "GetClientes")
	if !ok {
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusOK, h.svc.EmptyClienteMetrics(periodo), "")
		return
	}

	metrics, err := h.svc.GetClienteMetrics(r.Context(), h.db, periodo, scope.VendedorID)
	if err != nil {
		log.Printf("[dashboard] GetClientes: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, metrics, "")
}

// dashboardScope estende o escopo de vendedor com o bloqueio por
// desligamento, exclusivo do Dashboard.
//
// Desligado=true: usuário normal cujo vendedor vinculado tem
// data_desligamento preenchida. VendedorID é mantido (nunca zerado) — o
// bloqueio é feito pelo handler via SemAcesso(), que devolve respostas
// vazias sem consultar o repositório. Zerar VendedorID e seguir adiante
// seria perigoso: vendedorFilter trata vendedorID <= 0 como "sem filtro"
// (dados globais).
//
// VendedorInexistente=true: o vínculo id_vendedor aponta para um vendedor
// que não existe mais — também sem acesso, mas sem o aviso de desligamento.
type dashboardScope struct {
	vendedorScope
	Desligado           bool
	VendedorInexistente bool
}

// SemAcesso reporta se o Dashboard deve responder vazio: usuário normal sem
// vendedor vinculado, com vendedor inexistente ou com vendedor desligado.
func (s dashboardScope) SemAcesso() bool {
	return s.vendedorScope.SemAcesso() || s.Desligado || s.VendedorInexistente
}

// resolverEscopo resolve o escopo de vendedor da requisição e, para usuário
// normal com vendedor vinculado, verifica se o vendedor está desligado
// (fail-closed: erro de banco vira 500). Em caso de erro, registra log,
// responde 500 e retorna ok=false.
func (h *DashboardHandler) resolverEscopo(w http.ResponseWriter, r *http.Request, acao string) (dashboardScope, bool) {
	base, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[dashboard] %s: %v", acao, err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return dashboardScope{}, false
	}
	scope := dashboardScope{vendedorScope: base}

	if base.Restrito && base.VendedorID > 0 {
		desligado, err := h.vendedorRepo.IsDesligado(r.Context(), h.db, base.VendedorID)
		switch {
		case errors.Is(err, repositories.ErrNotFound):
			// Vínculo aponta para vendedor inexistente: sem acesso, mas não
			// é caso de desligamento (vendedor_desligado=false).
			scope.VendedorInexistente = true
		case err != nil:
			log.Printf("[dashboard] %s vendedor desligado: %v", acao, err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return dashboardScope{}, false
		default:
			scope.Desligado = desligado
		}
	}

	if h.svc.Cfg.Verbose {
		log.Printf("[dashboard] %s escopo restrito=%t vendedor_id=%d desligado=%t sem_acesso=%t",
			acao, scope.Restrito, scope.VendedorID, scope.Desligado, scope.SemAcesso())
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
