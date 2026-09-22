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

// EstoqueHandler trata as rotas /api/estoque/*.
type EstoqueHandler struct {
	db  *sql.DB
	svc *services.EstoqueService
}

// NewEstoqueHandler cria um EstoqueHandler com pool de conexão injetado.
func NewEstoqueHandler(db *sql.DB, cfg *config.Config) *EstoqueHandler {
	return &EstoqueHandler{
		db:  db,
		svc: services.NewEstoqueService(db, cfg),
	}
}

// parseRupturaQuery lê o query param "ruptura" ("true"/"false"). Qualquer
// outro valor (incluindo ausente/vazio) é tratado como "sem filtro" (nil).
func parseRupturaQuery(v string) *bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		b := true
		return &b
	case "false":
		b := false
		return &b
	default:
		return nil
	}
}

// ListEstoque GET /api/estoque
//
// Query params (opcionais): page (default 1), limit (default 20, max 100),
// sku, data_de, data_ate (AAAA-MM-DD), ruptura (true|false),
// order_by (id|data_snapshot|sku|saldo|ruptura|created_at|updated_at;
// default data_snapshot), order_dir (asc|desc; default desc),
// historico (true|false; default false).
//
// Por padrão (historico=false ou ausente) retorna apenas a última posição de
// cada SKU: sem filtro de data, a última posição geral; com data_de/data_ate,
// a última posição dentro do período informado. Com historico=true, retorna
// a série temporal completa (todos os snapshots que casarem com o filtro).
//
// Response: {success, data: [estoque...], error, pagination: {page, limit, total, pages}}
// Acesso comum.
func (h *EstoqueHandler) ListEstoque(w http.ResponseWriter, r *http.Request) {
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	historico := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("historico")), "true")

	filtro := services.EstoqueFiltro{
		SKU:       strings.TrimSpace(r.URL.Query().Get("sku")),
		DataDe:    strings.TrimSpace(r.URL.Query().Get("data_de")),
		DataAte:   strings.TrimSpace(r.URL.Query().Get("data_ate")),
		Ruptura:   parseRupturaQuery(r.URL.Query().Get("ruptura")),
		OrderBy:   strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir:  parseOrderDirQuery(r.URL.Query().Get("order_dir")),
		Historico: historico,
	}

	registros, total, err := h.svc.ListEstoque(r.Context(), h.db, page, limit, filtro)
	if err != nil {
		if status, msg, ok := estoqueErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[estoque] ListEstoque: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, registros, page, limit, total, pages)
}

// GetEstoque GET /api/estoque/{id}
//
// Response: {success, data: estoque, error}
// Acesso comum.
func (h *EstoqueHandler) GetEstoque(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	registro, err := h.svc.GetEstoqueByID(r.Context(), h.db, id)
	if err != nil {
		if status, msg, ok := estoqueErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[estoque] GetEstoque: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, registro, "")
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CreateEstoqueRequest body do POST /api/estoque (ajuste manual). ruptura
// NÃO é aceita como input — é sempre derivada de saldo <= 0 no service.
type CreateEstoqueRequest struct {
	SKU          string `json:"sku"`
	DataSnapshot string `json:"data_snapshot"` // formato AAAA-MM-DD
	Saldo        int    `json:"saldo"`
}

// UpdateEstoqueRequest body do PUT /api/estoque/{id} (ajuste manual). sku e
// data_snapshot não são editáveis (chave de negócio). ruptura NÃO é aceita
// como input — é sempre derivada de saldo <= 0 no service.
type UpdateEstoqueRequest struct {
	Saldo int `json:"saldo"`
}

// estoqueErroParaStatus mapeia erros de validação/negócio do EstoqueService
// para o status HTTP e mensagem apropriados. Retorna ok=false se o erro não
// for reconhecido (cabe ao chamador tratar como erro interno).
func estoqueErroParaStatus(err error) (status int, msg string, ok bool) {
	switch {
	case errors.Is(err, services.ErrEstoqueNaoEncontrado):
		return http.StatusNotFound, "registro de estoque não encontrado", true
	case errors.Is(err, services.ErrEstoqueSKUObrigatorio):
		return http.StatusBadRequest, "sku é obrigatório", true
	case errors.Is(err, services.ErrEstoqueSKUInexistente):
		return http.StatusBadRequest, "sku não corresponde a um produto cadastrado", true
	case errors.Is(err, services.ErrEstoqueSaldoInvalido):
		return http.StatusBadRequest, "saldo deve ser maior ou igual a zero", true
	case errors.Is(err, services.ErrEstoqueDataObrigatoria):
		return http.StatusBadRequest, "data_snapshot é obrigatória", true
	case errors.Is(err, services.ErrEstoqueDataInvalida):
		return http.StatusBadRequest, "data_snapshot inválida (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrEstoqueDataFutura):
		return http.StatusBadRequest, "data_snapshot não pode ser uma data futura", true
	default:
		return 0, "", false
	}
}

// CreateEstoque POST /api/estoque
//
// Body: { "sku": string, "data_snapshot": "AAAA-MM-DD", "saldo": number }
// ruptura é sempre derivada de saldo <= 0. origem é sempre gravada como
// "manual".
// Retorna: 201 com o registro criado.
// Admin only.
func (h *EstoqueHandler) CreateEstoque(w http.ResponseWriter, r *http.Request) {
	var req CreateEstoqueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	input := services.EstoqueInput{
		SKU:          req.SKU,
		DataSnapshot: req.DataSnapshot,
		Saldo:        req.Saldo,
	}

	registro, err := h.svc.CreateEstoque(r.Context(), h.db, input)
	if err != nil {
		if status, msg, ok := estoqueErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[estoque] CreateEstoque: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[estoque] criado manualmente: id=%d sku=%s por usuario role=%s", registro.ID, registro.SKU, role)
	writeJSON(w, http.StatusCreated, registro, "")
}

// UpdateEstoque PUT /api/estoque/{id}
//
// Body: { "saldo": number }
// sku e data_snapshot não são editáveis por esta rota. ruptura é sempre
// derivada de saldo <= 0. origem é sempre gravada como "manual".
// Retorna: 200 com o registro atualizado, 404 se não existir, 400 se o
// payload for inválido.
// Admin only.
func (h *EstoqueHandler) UpdateEstoque(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	var req UpdateEstoqueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	input := services.EstoqueInput{
		Saldo: req.Saldo,
	}

	registro, err := h.svc.UpdateEstoque(r.Context(), h.db, id, input)
	if err != nil {
		if status, msg, ok := estoqueErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[estoque] UpdateEstoque: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[estoque] atualizado manualmente: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, registro, "")
}
