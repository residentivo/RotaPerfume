// Package handlers contém handlers HTTP.
package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

// VisitaHandler trata as rotas /api/visitas/*.
type VisitaHandler struct {
	db  *sql.DB
	svc *services.VisitaService
}

// NewVisitaHandler cria um VisitaHandler com pool de conexão injetado.
func NewVisitaHandler(db *sql.DB, cfg *config.Config) *VisitaHandler {
	return &VisitaHandler{
		db:  db,
		svc: services.NewVisitaService(db, cfg),
	}
}

// ListVisitas GET /api/visitas
//
// Query params (opcionais): page (default 1), limit (default 20, max 100),
// cliente_id, vendedor_id, resultado, data_visita_de, data_visita_ate
// (formato AAAA-MM-DD), q (busca em resultado), order_by
// (id|visita_id|data_visita|duracao_min|resultado|created_at|updated_at;
// default id), order_dir (asc|desc; default asc).
// Response: {success, data: [visita...], error, pagination: {page, limit, total, pages}}
// Acesso comum: usuário role=normal só enxerga visitas da própria carteira
// (vendedor_id da query é ignorado e forçado ao vendedor vinculado).
func (h *VisitaHandler) ListVisitas(w http.ResponseWriter, r *http.Request) {
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[visitas] ListVisitas escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSONWithPagination(w, http.StatusOK, []any{}, page, limit, 0, 0)
		return
	}

	filtro := services.VisitaFiltro{
		ClienteID:     parseInt64Query(r.URL.Query().Get("cliente_id")),
		VendedorID:    parseInt64Query(r.URL.Query().Get("vendedor_id")),
		Resultado:     strings.TrimSpace(r.URL.Query().Get("resultado")),
		DataVisitaDe:  strings.TrimSpace(r.URL.Query().Get("data_visita_de")),
		DataVisitaAte: strings.TrimSpace(r.URL.Query().Get("data_visita_ate")),
		Q:             strings.TrimSpace(r.URL.Query().Get("q")),
		OrderBy:       strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir:      parseOrderDirQuery(r.URL.Query().Get("order_dir")),
	}
	if scope.Restrito {
		// Usuário role=normal: força o filtro à própria carteira, ignorando
		// qualquer vendedor_id vindo da query string (evita bypass via URL).
		filtro.VendedorID = scope.VendedorID
	}

	visitas, total, err := h.svc.ListVisitas(r.Context(), h.db, page, limit, filtro)
	if err != nil {
		log.Printf("[visitas] ListVisitas: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, visitas, page, limit, total, pages)
}

// GetVisita GET /api/visitas/{id}
//
// Response: {success, data: visita, error}
// Acesso comum: usuário role=normal só enxerga visita da própria carteira
// (404 — não 403 — se pertencer a outro vendedor, para não permitir
// enumeração de IDs).
func (h *VisitaHandler) GetVisita(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[visitas] GetVisita escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
		return
	}

	visita, err := h.svc.GetVisitaByID(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrVisitaNaoEncontrada) {
			writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
			return
		}
		log.Printf("[visitas] GetVisita: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	if scope.Restrito && !scope.PermiteVendedor(visita.VendedorID) {
		writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
		return
	}

	writeJSON(w, http.StatusOK, visita, "")
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CreateVisitaRequest body do POST /api/visitas.
type CreateVisitaRequest struct {
	ClienteID  int64  `json:"cliente_id"`
	VendedorID int64  `json:"vendedor_id"`
	DataVisita string `json:"data_visita"` // formato AAAA-MM-DD
	Resultado  string `json:"resultado"`
	DuracaoMin int    `json:"duracao_min"`
}

// UpdateVisitaRequest body do PUT /api/visitas/{id}.
type UpdateVisitaRequest struct {
	ClienteID  int64  `json:"cliente_id"`
	VendedorID int64  `json:"vendedor_id"`
	DataVisita string `json:"data_visita"` // formato AAAA-MM-DD
	Resultado  string `json:"resultado"`
	DuracaoMin int    `json:"duracao_min"`
}

// visitaErroParaStatus mapeia erros de validação/negócio do VisitaService
// para o status HTTP e mensagem apropriados. Retorna ok=false se o erro não
// for reconhecido (cabe ao chamador tratar como erro interno).
func visitaErroParaStatus(err error) (status int, msg string, ok bool) {
	switch {
	case errors.Is(err, services.ErrVisitaNaoEncontrada):
		return http.StatusNotFound, "visita não encontrada", true
	case errors.Is(err, services.ErrVisitaClienteInvalido):
		return http.StatusBadRequest, "cliente_id é obrigatório e deve existir", true
	case errors.Is(err, services.ErrVisitaVendedorInvalido):
		return http.StatusBadRequest, "vendedor_id é obrigatório e deve existir", true
	case errors.Is(err, services.ErrVisitaDataInvalida):
		return http.StatusBadRequest, "data_visita inválida (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrVisitaResultadoInvalido):
		return http.StatusBadRequest, "resultado é obrigatório", true
	case errors.Is(err, services.ErrVisitaDuracaoInvalida):
		return http.StatusBadRequest, "duracao_min deve ser maior ou igual a zero", true
	default:
		return 0, "", false
	}
}

// CreateVisita POST /api/visitas
//
// Body: { "cliente_id": number, "vendedor_id": number,
// "data_visita": "AAAA-MM-DD", "resultado": string, "duracao_min": number }
// Retorna: 201 com a visita criada.
// Acesso comum: usuário role=normal só pode criar visita para a própria
// carteira (vendedor_id do payload é ignorado e forçado ao vendedor
// vinculado; cliente_id deve pertencer à carteira ativa desse vendedor).
func (h *VisitaHandler) CreateVisita(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.GetRole(r.Context())

	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[visitas] CreateVisita escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusForbidden, nil, "usuário sem vendedor vinculado")
		return
	}

	var req CreateVisitaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	if scope.Restrito {
		// Usuário role=normal: nunca confia no vendedor_id do payload — força
		// à própria carteira (evita forjar registro para outro vendedor).
		req.VendedorID = scope.VendedorID

		pertence, err := clienteNaCarteiraDoVendedor(r.Context(), h.db, scope.VendedorID, req.ClienteID)
		if err != nil {
			log.Printf("[visitas] CreateVisita checar carteira: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		if !pertence {
			writeJSON(w, http.StatusBadRequest, nil, "cliente não pertence à carteira deste vendedor")
			return
		}
	}

	input := services.VisitaInput{
		ClienteID:  req.ClienteID,
		VendedorID: req.VendedorID,
		DataVisita: req.DataVisita,
		Resultado:  req.Resultado,
		DuracaoMin: req.DuracaoMin,
	}

	visita, err := h.svc.CreateVisita(r.Context(), h.db, input)
	if err != nil {
		if status, msg, ok := visitaErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[visitas] CreateVisita: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[visitas] criada: id=%d por usuario role=%s", visita.VisitaID, role)
	writeJSON(w, http.StatusCreated, visita, "")
}

// UpdateVisita PUT /api/visitas/{id}
//
// Body: igual ao de CreateVisita.
// Retorna: 200 com a visita atualizada, 404 se não existir, 400 se o
// payload for inválido.
// Acesso comum: usuário role=normal só pode atualizar visita da própria
// carteira (404 se pertencer a outro vendedor) e não pode reatribuir
// vendedor_id para outro vendedor.
func (h *VisitaHandler) UpdateVisita(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[visitas] UpdateVisita escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
		return
	}

	atual, err := h.svc.GetVisitaByID(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrVisitaNaoEncontrada) {
			writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
			return
		}
		log.Printf("[visitas] UpdateVisita buscar atual: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.Restrito && !scope.PermiteVendedor(atual.VendedorID) {
		writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
		return
	}

	var req UpdateVisitaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	if scope.Restrito {
		// Usuário role=normal: nunca confia no vendedor_id do payload — força
		// à própria carteira (evita reatribuir o registro a outro vendedor).
		req.VendedorID = scope.VendedorID

		pertence, err := clienteNaCarteiraDoVendedor(r.Context(), h.db, scope.VendedorID, req.ClienteID)
		if err != nil {
			log.Printf("[visitas] UpdateVisita checar carteira: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		if !pertence {
			writeJSON(w, http.StatusBadRequest, nil, "cliente não pertence à carteira deste vendedor")
			return
		}
	}

	input := services.VisitaInput{
		ClienteID:  req.ClienteID,
		VendedorID: req.VendedorID,
		DataVisita: req.DataVisita,
		Resultado:  req.Resultado,
		DuracaoMin: req.DuracaoMin,
	}

	visita, err := h.svc.UpdateVisita(r.Context(), h.db, id, input)
	if err != nil {
		if status, msg, ok := visitaErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[visitas] UpdateVisita: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[visitas] atualizada: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, visita, "")
}
