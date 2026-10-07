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
	"github.com/rotaperfumes/shared/vlog"
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
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetMetrics", "chamando r.URL.Query().Get e atribuindo resultado a periodo")
	periodo := r.URL.Query().Get("periodo")
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetMetrics", "verificando se periodo == \"\"")
	if periodo == "" {
		vlog.Printf("dashboard_handler.go", "DashboardHandler.GetMetrics", "atribuindo periodo = \"...\"")
		periodo = "month"
	}
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetMetrics", "verificando se periodo != \"...\" && periodo != \"...\" && periodo != \"...\"")
	if periodo != "today" && periodo != "week" && periodo != "month" {
		writeJSON(w, http.StatusBadRequest, nil, "periodo deve ser 'today', 'week' ou 'month'")
		return
	}

	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetMetrics", "chamando h.resolverEscopo e atribuindo resultado a scope, ok")
	scope, ok := h.resolverEscopo(w, r, "GetMetrics")
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetMetrics", "verificando se !ok")
	if !ok {
		return
	}
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetMetrics", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusOK, h.svc.EmptyMetrics(periodo, scope.Desligado), "")
		return
	}

	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetMetrics", "chamando h.svc.GetMetrics e atribuindo resultado a metrics, err")
	metrics, err := h.svc.GetMetrics(r.Context(), h.db, periodo, scope.VendedorID)
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetMetrics", "verificando se err != nil")
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
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendas", "definindo dias := 30")
	dias := 30
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendas", "chamando r.URL.Query().Get e atribuindo resultado a d e verificando se d != \"\"")
	if d := r.URL.Query().Get("dias"); d != "" {
		vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendas", "chamando parseIntDefault e atribuindo resultado a n e verificando se n > 0 && n <= 365")
		if n := parseIntDefault(d, 30); n > 0 && n <= 365 {
			vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendas", "atribuindo dias = n")
			dias = n
		}
	}

	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendas", "chamando h.resolverEscopo e atribuindo resultado a scope, ok")
	scope, ok := h.resolverEscopo(w, r, "GetVendas")
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendas", "verificando se !ok")
	if !ok {
		return
	}
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendas", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusOK, h.svc.EmptyVendasSeries(dias), "")
		return
	}

	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendas", "chamando h.svc.GetVendasSeries e atribuindo resultado a series, err")
	series, err := h.svc.GetVendasSeries(r.Context(), h.db, dias, scope.VendedorID)
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendas", "verificando se err != nil")
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
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendedores", "chamando services.ParsePagination e atribuindo resultado a page, limit")
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendedores", "chamando h.resolverEscopo e atribuindo resultado a scope, ok")
	scope, ok := h.resolverEscopo(w, r, "GetVendedores")
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendedores", "verificando se !ok")
	if !ok {
		return
	}
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendedores", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSONWithPagination(w, http.StatusOK, []map[string]any{}, page, limit, 0, 0)
		return
	}

	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendedores", "chamando h.svc.GetVendedoresRanking e atribuindo resultado a vendedores, total, err")
	vendedores, total, err := h.svc.GetVendedoresRanking(r.Context(), h.db, page, limit, scope.VendedorID)
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendedores", "verificando se err != nil")
	if err != nil {
		log.Printf("[dashboard] GetVendedores: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendedores", "definindo pages := total / limit")
	pages := total / limit
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendedores", "verificando se total%%limit != 0")
	if total%limit != 0 {
		vlog.Printf("dashboard_handler.go", "DashboardHandler.GetVendedores", "incrementando pages")
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
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetClientes", "chamando r.URL.Query().Get e atribuindo resultado a periodo")
	periodo := r.URL.Query().Get("periodo")
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetClientes", "verificando se periodo == \"\"")
	if periodo == "" {
		vlog.Printf("dashboard_handler.go", "DashboardHandler.GetClientes", "atribuindo periodo = \"...\"")
		periodo = "month"
	}
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetClientes", "verificando se periodo != \"...\" && periodo != \"...\" && periodo != \"...\"")
	if periodo != "today" && periodo != "week" && periodo != "month" {
		writeJSON(w, http.StatusBadRequest, nil, "periodo deve ser 'today', 'week' ou 'month'")
		return
	}

	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetClientes", "chamando h.resolverEscopo e atribuindo resultado a scope, ok")
	scope, ok := h.resolverEscopo(w, r, "GetClientes")
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetClientes", "verificando se !ok")
	if !ok {
		return
	}
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetClientes", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusOK, h.svc.EmptyClienteMetrics(periodo), "")
		return
	}

	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetClientes", "chamando h.svc.GetClienteMetrics e atribuindo resultado a metrics, err")
	metrics, err := h.svc.GetClienteMetrics(r.Context(), h.db, periodo, scope.VendedorID)
	vlog.Printf("dashboard_handler.go", "DashboardHandler.GetClientes", "verificando se err != nil")
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
	vlog.Printf("dashboard_handler.go", "DashboardHandler.resolverEscopo", "chamando resolverVendedorScopeBase e atribuindo resultado a base, err")
	base, err := resolverVendedorScopeBase(r.Context(), h.db)
	vlog.Printf("dashboard_handler.go", "DashboardHandler.resolverEscopo", "verificando se err != nil")
	if err != nil {
		log.Printf("[dashboard] %s: %v", acao, err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return dashboardScope{}, false
	}
	vlog.Printf("dashboard_handler.go", "DashboardHandler.resolverEscopo", "montando dashboardScope em scope")
	scope := dashboardScope{vendedorScope: base}

	vlog.Printf("dashboard_handler.go", "DashboardHandler.resolverEscopo", "verificando se base.Restrito && base.VendedorID > 0")
	if base.Restrito && base.VendedorID > 0 {
		vlog.Printf("dashboard_handler.go", "DashboardHandler.resolverEscopo", "chamando h.vendedorRepo.IsDesligado e atribuindo resultado a desligado, err")
		desligado, err := h.vendedorRepo.IsDesligado(r.Context(), h.db, base.VendedorID)
		vlog.Printf("dashboard_handler.go", "DashboardHandler.resolverEscopo", "avaliando switch de condições")
		switch {
		case errors.Is(err, repositories.ErrNotFound):
			vlog.Printf("dashboard_handler.go", "DashboardHandler.resolverEscopo", "atribuindo scope.VendedorInexistente = true")
			// Vínculo aponta para vendedor inexistente: sem acesso, mas não
			// é caso de desligamento (vendedor_desligado=false).
			scope.VendedorInexistente = true
		case err != nil:
			log.Printf("[dashboard] %s vendedor desligado: %v", acao, err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return dashboardScope{}, false
		default:
			vlog.Printf("dashboard_handler.go", "DashboardHandler.resolverEscopo", "atribuindo scope.Desligado = desligado")
			scope.Desligado = desligado
		}
	}

	vlog.Printf("dashboard_handler.go", "DashboardHandler.resolverEscopo", "verificando se h.svc.Cfg.Verbose")
	if h.svc.Cfg.Verbose {
		log.Printf("[dashboard] %s escopo restrito=%t vendedor_id=%d desligado=%t sem_acesso=%t",
			acao, scope.Restrito, scope.VendedorID, scope.Desligado, scope.SemAcesso())
	}
	return scope, true
}

func parseIntDefault(s string, fallback int) int {
	vlog.Printf("dashboard_handler.go", "parseIntDefault", "definindo n := 0")
	n := 0
	vlog.Printf("dashboard_handler.go", "parseIntDefault", "iniciando loop sobre s")
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return fallback
		}
	}
	vlog.Printf("dashboard_handler.go", "parseIntDefault", "loop sobre s concluído: %d itens", len(s))
	return n
}
