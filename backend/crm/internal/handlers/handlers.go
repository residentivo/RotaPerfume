package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"backend/crm/internal/models"
	"backend/crm/internal/services"
)

// Handler contém todos os handlers
type Handler struct {
	clienteService      *services.ClienteService
	vendedorService     *services.VendedorService
	carteiraService     *services.CarteiraService
	visitaService       *services.VisitaService
	oportunidadeService *services.OportunidadeService
	dashboardService    *services.DashboardService
}

// NewHandler cria um novo handler com todas as dependências
func NewHandler(
	clienteService *services.ClienteService,
	vendedorService *services.VendedorService,
	carteiraService *services.CarteiraService,
	visitaService *services.VisitaService,
	oportunidadeService *services.OportunidadeService,
	dashboardService *services.DashboardService,
) *Handler {
	return &Handler{
		clienteService:      clienteService,
		vendedorService:     vendedorService,
		carteiraService:     carteiraService,
		visitaService:       visitaService,
		oportunidadeService: oportunidadeService,
		dashboardService:    dashboardService,
	}
}

// ========== Cliente Handlers ==========

// ListarClientes lista todos os clientes com filtros
func (h *Handler) ListarClientes(w http.ResponseWriter, r *http.Request) {
	filtro := models.ClienteFiltro{
		UF:       r.URL.Query().Get("uf"),
		Segmento: r.URL.Query().Get("segmento"),
		Cidade:   r.URL.Query().Get("cidade"),
	}
	if ativo := r.URL.Query().Get("ativo"); ativo != "" {
		val := "N"
		if ativo == "true" || ativo == "1" || ativo == "S" {
			val = "S"
		}
		filtro.Ativo = &val
	}

	clientes, err := h.clienteService.ListarTodos(r.Context(), filtro)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clientes)
}

// BuscarCliente busca um cliente por ID
func (h *Handler) BuscarCliente(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, `{"erro": "ID inválido"}`, http.StatusBadRequest)
		return
	}

	cliente, err := h.clienteService.BuscarPorID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cliente)
}

// CriarCliente cria um novo cliente
func (h *Handler) CriarCliente(w http.ResponseWriter, r *http.Request) {
	var input models.ClienteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"erro": "requisição inválida"}`, http.StatusBadRequest)
		return
	}

	cliente, err := h.clienteService.Criar(r.Context(), input)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cliente)
}

// AtualizarCliente atualiza um cliente
func (h *Handler) AtualizarCliente(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, `{"erro": "ID inválido"}`, http.StatusBadRequest)
		return
	}

	var input models.ClienteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"erro": "requisição inválida"}`, http.StatusBadRequest)
		return
	}

	cliente, err := h.clienteService.Atualizar(r.Context(), id, input)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cliente)
}

// ========== Vendedor Handlers ==========

// ListarVendedores lista todos os vendedores
func (h *Handler) ListarVendedores(w http.ResponseWriter, r *http.Request) {
	filtro := models.VendedorFiltro{
		UF:     r.URL.Query().Get("uf"),
		Regiao: r.URL.Query().Get("regiao"),
	}
	if ativo := r.URL.Query().Get("ativo"); ativo == "true" || ativo == "1" {
		filtro.Ativo = true
	}

	vendedores, err := h.vendedorService.ListarTodos(r.Context(), filtro)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vendedores)
}

// BuscarVendedor busca um vendedor por ID
func (h *Handler) BuscarVendedor(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, `{"erro": "ID inválido"}`, http.StatusBadRequest)
		return
	}

	vendedor, err := h.vendedorService.BuscarPorID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vendedor)
}

// ========== Carteira Handlers ==========

// ListarCarteira lista a carteira de um vendedor ou todos
func (h *Handler) ListarCarteira(w http.ResponseWriter, r *http.Request) {
	vendedorIDStr := r.URL.Query().Get("vendedor_id")
	if vendedorIDStr != "" {
		vendedorID, err := strconv.ParseInt(vendedorIDStr, 10, 64)
		if err != nil {
			http.Error(w, `{"erro": "vendedor_id inválido"}`, http.StatusBadRequest)
			return
		}
		carteiras, err := h.carteiraService.ListarPorVendedor(r.Context(), vendedorID)
		if err != nil {
			http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(carteiras)
		return
	}

	carteiras, err := h.carteiraService.ListarTodos(r.Context())
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(carteiras)
}

// CriarCarteira cria uma nova associação de carteira
func (h *Handler) CriarCarteira(w http.ResponseWriter, r *http.Request) {
	var input models.CarteiraInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"erro": "requisição inválida"}`, http.StatusBadRequest)
		return
	}

	carteira, err := h.carteiraService.Criar(r.Context(), input)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(carteira)
}

// ========== Visita Handlers ==========

// ListarVisitas lista visitas com filtros
func (h *Handler) ListarVisitas(w http.ResponseWriter, r *http.Request) {
	vendedorIDStr := r.URL.Query().Get("vendedor_id")
	clienteIDStr := r.URL.Query().Get("cliente_id")

	if vendedorIDStr != "" {
		vendedorID, err := strconv.ParseInt(vendedorIDStr, 10, 64)
		if err != nil {
			http.Error(w, `{"erro": "vendedor_id inválido"}`, http.StatusBadRequest)
			return
		}
		visitas, err := h.visitaService.ListarPorVendedor(r.Context(), vendedorID)
		if err != nil {
			http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(visitas)
		return
	}

	if clienteIDStr != "" {
		clienteID, err := strconv.ParseInt(clienteIDStr, 10, 64)
		if err != nil {
			http.Error(w, `{"erro": "cliente_id inválido"}`, http.StatusBadRequest)
			return
		}
		visitas, err := h.visitaService.ListarPorCliente(r.Context(), clienteID)
		if err != nil {
			http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(visitas)
		return
	}

	// Se nenhum filtro, retorna vazio
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]models.Visita{})
}

// CriarVisita cria uma nova visita
func (h *Handler) CriarVisita(w http.ResponseWriter, r *http.Request) {
	var input models.VisitaInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"erro": "requisição inválida"}`, http.StatusBadRequest)
		return
	}

	visita, err := h.visitaService.Criar(r.Context(), input)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(visita)
}

// ========== Oportunidade Handlers ==========

// ListarOportunidades lista oportunidades com filtros
func (h *Handler) ListarOportunidades(w http.ResponseWriter, r *http.Request) {
	filtro := models.OportunidadeFiltro{
		Etapa: r.URL.Query().Get("etapa"),
	}
	if vendedorIDStr := r.URL.Query().Get("vendedor_id"); vendedorIDStr != "" {
		vendedorID, err := strconv.ParseInt(vendedorIDStr, 10, 64)
		if err != nil {
			http.Error(w, `{"erro": "vendedor_id inválido"}`, http.StatusBadRequest)
			return
		}
		filtro.VendedorID = vendedorID
	}

	oportunidades, err := h.oportunidadeService.ListarTodos(r.Context(), filtro)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(oportunidades)
}

// BuscarOportunidade busca uma oportunidade por ID
func (h *Handler) BuscarOportunidade(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, `{"erro": "ID inválido"}`, http.StatusBadRequest)
		return
	}

	oportunidade, err := h.oportunidadeService.BuscarPorID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(oportunidade)
}

// CriarOportunidade cria uma nova oportunidade
func (h *Handler) CriarOportunidade(w http.ResponseWriter, r *http.Request) {
	var input models.OportunidadeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"erro": "requisição inválida"}`, http.StatusBadRequest)
		return
	}

	oportunidade, err := h.oportunidadeService.Criar(r.Context(), input)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(oportunidade)
}

// AtualizarOportunidade atualiza uma oportunidade
func (h *Handler) AtualizarOportunidade(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, `{"erro": "ID inválido"}`, http.StatusBadRequest)
		return
	}

	var input models.OportunidadeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"erro": "requisição inválida"}`, http.StatusBadRequest)
		return
	}

	oportunidade, err := h.oportunidadeService.Atualizar(r.Context(), id, input)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(oportunidade)
}

// ========== Dashboard Handlers ==========

// RankingVendas retorna ranking de vendedores
func (h *Handler) RankingVendas(w http.ResponseWriter, r *http.Request) {
	ranking, err := h.dashboardService.RankingVendedores(r.Context())
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ranking)
}

// DistribuicaoCarteira retorna distribuição de carteira
func (h *Handler) DistribuicaoCarteira(w http.ResponseWriter, r *http.Request) {
	dist, err := h.dashboardService.DistribuicaoCarteira(r.Context())
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dist)
}

// ========== Health Check ==========

// HealthCheck retorna status da API
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"service": "crm-api",
	})
}
