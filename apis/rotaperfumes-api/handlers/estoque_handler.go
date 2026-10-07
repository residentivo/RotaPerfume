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
	"github.com/rotaperfumes/shared/vlog"
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
	vlog.Printf("estoque_handler.go", "parseRupturaQuery", "avaliando switch sobre strings.ToLower(...)")
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		vlog.Printf("estoque_handler.go", "parseRupturaQuery", "definindo b := true")
		b := true
		return &b
	case "false":
		vlog.Printf("estoque_handler.go", "parseRupturaQuery", "definindo b := false")
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
	vlog.Printf("estoque_handler.go", "EstoqueHandler.ListEstoque", "chamando services.ParsePagination e atribuindo resultado a page, limit")
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	vlog.Printf("estoque_handler.go", "EstoqueHandler.ListEstoque", "chamando strings.EqualFold e atribuindo resultado a historico")
	historico := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("historico")), "true")

	vlog.Printf("estoque_handler.go", "EstoqueHandler.ListEstoque", "montando services.EstoqueFiltro em filtro")
	filtro := services.EstoqueFiltro{
		SKU:       strings.TrimSpace(r.URL.Query().Get("sku")),
		DataDe:    strings.TrimSpace(r.URL.Query().Get("data_de")),
		DataAte:   strings.TrimSpace(r.URL.Query().Get("data_ate")),
		Ruptura:   parseRupturaQuery(r.URL.Query().Get("ruptura")),
		OrderBy:   strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir:  parseOrderDirQuery(r.URL.Query().Get("order_dir")),
		Historico: historico,
	}

	vlog.Printf("estoque_handler.go", "EstoqueHandler.ListEstoque", "chamando h.svc.ListEstoque e atribuindo resultado a registros, total, err")
	registros, total, err := h.svc.ListEstoque(r.Context(), h.db, page, limit, filtro)
	vlog.Printf("estoque_handler.go", "EstoqueHandler.ListEstoque", "verificando se err != nil")
	if err != nil {
		vlog.Printf("estoque_handler.go", "EstoqueHandler.ListEstoque", "chamando estoqueErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := estoqueErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[estoque] ListEstoque: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("estoque_handler.go", "EstoqueHandler.ListEstoque", "definindo pages := total / limit")
	pages := total / limit
	vlog.Printf("estoque_handler.go", "EstoqueHandler.ListEstoque", "verificando se total%%limit != 0")
	if total%limit != 0 {
		vlog.Printf("estoque_handler.go", "EstoqueHandler.ListEstoque", "incrementando pages")
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, registros, page, limit, total, pages)
}

// GetEstoque GET /api/estoque/{id}
//
// Response: {success, data: estoque, error}
// Acesso comum.
func (h *EstoqueHandler) GetEstoque(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("estoque_handler.go", "EstoqueHandler.GetEstoque", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("estoque_handler.go", "EstoqueHandler.GetEstoque", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("estoque_handler.go", "EstoqueHandler.GetEstoque", "chamando h.svc.GetEstoqueByID e atribuindo resultado a registro, err")
	registro, err := h.svc.GetEstoqueByID(r.Context(), h.db, id)
	vlog.Printf("estoque_handler.go", "EstoqueHandler.GetEstoque", "verificando se err != nil")
	if err != nil {
		vlog.Printf("estoque_handler.go", "EstoqueHandler.GetEstoque", "chamando estoqueErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
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
	vlog.Printf("estoque_handler.go", "estoqueErroParaStatus", "avaliando switch de condições")
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
// ruptura é sempre derivada de saldo <= 0.
// Retorna: 201 com o registro criado.
// Admin only.
func (h *EstoqueHandler) CreateEstoque(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("estoque_handler.go", "EstoqueHandler.CreateEstoque", "declarando variável req")
	var req CreateEstoqueRequest
	vlog.Printf("estoque_handler.go", "EstoqueHandler.CreateEstoque", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("estoque_handler.go", "EstoqueHandler.CreateEstoque", "montando services.EstoqueInput em input")
	input := services.EstoqueInput{
		SKU:          req.SKU,
		DataSnapshot: req.DataSnapshot,
		Saldo:        req.Saldo,
	}

	vlog.Printf("estoque_handler.go", "EstoqueHandler.CreateEstoque", "chamando h.svc.CreateEstoque e atribuindo resultado a registro, err")
	registro, err := h.svc.CreateEstoque(r.Context(), h.db, input)
	vlog.Printf("estoque_handler.go", "EstoqueHandler.CreateEstoque", "verificando se err != nil")
	if err != nil {
		vlog.Printf("estoque_handler.go", "EstoqueHandler.CreateEstoque", "chamando estoqueErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := estoqueErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[estoque] CreateEstoque: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("estoque_handler.go", "EstoqueHandler.CreateEstoque", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[estoque] criado manualmente: id=%d sku=%s por usuario role=%s", registro.ID, registro.SKU, role)
	writeJSON(w, http.StatusCreated, registro, "")
}

// UpdateEstoque PUT /api/estoque/{id}
//
// Body: { "saldo": number }
// sku e data_snapshot não são editáveis por esta rota. ruptura é sempre
// derivada de saldo <= 0.
// Retorna: 200 com o registro atualizado, 404 se não existir, 400 se o
// payload for inválido.
// Admin only.
func (h *EstoqueHandler) UpdateEstoque(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("estoque_handler.go", "EstoqueHandler.UpdateEstoque", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("estoque_handler.go", "EstoqueHandler.UpdateEstoque", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("estoque_handler.go", "EstoqueHandler.UpdateEstoque", "declarando variável req")
	var req UpdateEstoqueRequest
	vlog.Printf("estoque_handler.go", "EstoqueHandler.UpdateEstoque", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("estoque_handler.go", "EstoqueHandler.UpdateEstoque", "montando services.EstoqueInput em input")
	input := services.EstoqueInput{
		Saldo: req.Saldo,
	}

	vlog.Printf("estoque_handler.go", "EstoqueHandler.UpdateEstoque", "chamando h.svc.UpdateEstoque e atribuindo resultado a registro, err")
	registro, err := h.svc.UpdateEstoque(r.Context(), h.db, id, input)
	vlog.Printf("estoque_handler.go", "EstoqueHandler.UpdateEstoque", "verificando se err != nil")
	if err != nil {
		vlog.Printf("estoque_handler.go", "EstoqueHandler.UpdateEstoque", "chamando estoqueErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := estoqueErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[estoque] UpdateEstoque: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("estoque_handler.go", "EstoqueHandler.UpdateEstoque", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[estoque] atualizado manualmente: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, registro, "")
}
