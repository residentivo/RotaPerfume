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
	"github.com/rotaperfumes/shared/models"
)

// ClienteHandler trata as rotas /api/clientes/*.
type ClienteHandler struct {
	db  *sql.DB
	svc *services.ClienteService
}

// NewClienteHandler cria um ClienteHandler com pool de conexão injetado.
func NewClienteHandler(db *sql.DB, cfg *config.Config) *ClienteHandler {
	return &ClienteHandler{
		db:  db,
		svc: services.NewClienteService(db, cfg),
	}
}

// parseAtivoQuery lê o query param "ativo" ("true"/"false"). Qualquer outro
// valor (incluindo ausente/vazio) é tratado como "sem filtro" (nil).
func parseAtivoQuery(v string) *bool {
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

// ListClientes GET /api/clientes
//
// Query params (opcionais): page (default 1), limit (default 20, max 100),
// uf, segmento, ativo (true|false), q (busca em razao_social OU cnpj).
// Response: {success, data: [cliente...], error, pagination: {page, limit, total, pages}}
// Admin only.
func (h *ClienteHandler) ListClientes(w http.ResponseWriter, r *http.Request) {
	role, ok := middleware.GetRole(r.Context())
	if !ok || role != models.RoleAdmin {
		writeJSON(w, http.StatusForbidden, nil, "acesso restrito a administradores")
		return
	}

	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	filtro := services.ClienteFiltro{
		UF:       strings.TrimSpace(r.URL.Query().Get("uf")),
		Segmento: strings.TrimSpace(r.URL.Query().Get("segmento")),
		Ativo:    parseAtivoQuery(r.URL.Query().Get("ativo")),
		Q:        strings.TrimSpace(r.URL.Query().Get("q")),
	}

	clientes, total, err := h.svc.ListClientes(r.Context(), h.db, page, limit, filtro)
	if err != nil {
		log.Printf("[clientes] ListClientes: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, clientes, page, limit, total, pages)
}

// GetCliente GET /api/clientes/{id}
//
// Response: {success, data: cliente, error}
// Admin only.
func (h *ClienteHandler) GetCliente(w http.ResponseWriter, r *http.Request) {
	role, ok := middleware.GetRole(r.Context())
	if !ok || role != models.RoleAdmin {
		writeJSON(w, http.StatusForbidden, nil, "acesso restrito a administradores")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	cliente, err := h.svc.GetClienteByID(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrClienteNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "cliente não encontrado")
			return
		}
		log.Printf("[clientes] GetCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, cliente, "")
}

// ToggleAtivoClienteRequest body do PATCH /api/clientes/{id}/inativar.
type ToggleAtivoClienteRequest struct {
	Ativo *bool `json:"ativo"` // omitido = toggle
}

// ToggleAtivoCliente PATCH /api/clientes/{id}/inativar
//
// Body opcional: { "ativo": bool } — omitido = toggle
// Response: {success, data: cliente atualizado, error}
// Admin only.
func (h *ClienteHandler) ToggleAtivoCliente(w http.ResponseWriter, r *http.Request) {
	role, ok := middleware.GetRole(r.Context())
	if !ok || role != models.RoleAdmin {
		writeJSON(w, http.StatusForbidden, nil, "acesso restrito a administradores")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	var req ToggleAtivoClienteRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // body opcional, ignora erro

	cliente, err := h.svc.ToggleAtivoCliente(r.Context(), h.db, id, req.Ativo)
	if err != nil {
		if errors.Is(err, services.ErrClienteNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "cliente não encontrado")
			return
		}
		log.Printf("[clientes] ToggleAtivoCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[clientes] ativo=%t: id=%d por admin=%s", cliente.Ativo, id, role)
	writeJSON(w, http.StatusOK, cliente, "")
}
