// Package handlers contém handlers HTTP de histórico de senhas.
package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/models"
)

// SenhaHistoricoHandler trata as rotas /api/senha-historico/*.
type SenhaHistoricoHandler struct {
	db  *sql.DB
	svc *services.SenhaHistoricoService
}

// NewSenhaHistoricoHandler cria um SenhaHistoricoHandler.
func NewSenhaHistoricoHandler(db *sql.DB) *SenhaHistoricoHandler {
	return &SenhaHistoricoHandler{
		db:  db,
		svc: services.NewSenhaHistoricoService(),
	}
}

// ListarTodos GET /api/senha-historico
//
// Query params: page (default 1), limit (default 20, max 100),
// order_by (id|usuario_id|tipo_reset|created_at; default id),
// order_dir (asc|desc; default desc).
// Response: {success, data: [{id, usuario_id, resetado_por_id, ip_origem, user_agent, tipo_reset, created_at}], pagination}
// Admin only.
func (h *SenhaHistoricoHandler) ListarTodos(w http.ResponseWriter, r *http.Request) {
	role, ok := middleware.GetRole(r.Context())
	if !ok || role != models.RoleAdmin {
		writeJSON(w, http.StatusForbidden, nil, "acesso restrito a administradores")
		return
	}

	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)
	orderBy := strings.TrimSpace(r.URL.Query().Get("order_by"))
	orderDir := parseOrderDirQuery(r.URL.Query().Get("order_dir"))

	ctx := r.Context()
	historicos, total, err := h.svc.ListarTodos(ctx, h.db, page, limit, orderBy, orderDir)
	if err != nil {
		log.Printf("[senha-historico] ListarTodos: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	out := make([]map[string]any, 0, len(historicos))
	for _, h := range historicos {
		// Nota: senha_hash_anterior (bcrypt) é mantido no model para uso
		// interno/auditoria em banco, mas NUNCA deve ser serializado na
		// resposta HTTP — mesmo sendo um hash, sua exposição facilita
		// ataques offline (ex. em caso de vazamento de logs/rede).
		item := map[string]any{
			"id":         h.ID,
			"usuario_id": h.UsuarioID,
			"ip_origem":  h.IPOrigem,
			"user_agent": h.UserAgent,
			"tipo_reset": h.TipoReset,
			"created_at": h.CreatedAt,
		}
		if h.ResetadoPorID.Valid {
			item["resetado_por_id"] = h.ResetadoPorID.Int64
		}
		out = append(out, item)
	}

	writeJSONWithPagination(w, http.StatusOK, out, page, limit, total, pages)
}

// ListarPorUsuario GET /api/senha-historico/{usuario_id}
//
// Query params: page (default 1), limit (default 20, max 100),
// order_by (id|usuario_id|tipo_reset|created_at; default id),
// order_dir (asc|desc; default desc).
// Response: {success, data: [...], pagination}
// Admin only.
func (h *SenhaHistoricoHandler) ListarPorUsuario(w http.ResponseWriter, r *http.Request) {
	role, ok := middleware.GetRole(r.Context())
	if !ok || role != models.RoleAdmin {
		writeJSON(w, http.StatusForbidden, nil, "acesso restrito a administradores")
		return
	}

	// Extrai usuario_id da path.
	path := r.URL.Path
	parts := strings.Split(path, "/")
	var usuarioIDStr string
	for i, p := range parts {
		if p == "senha-historico" && i+1 < len(parts) {
			usuarioIDStr = parts[i+1]
			break
		}
	}
	if usuarioIDStr == "" {
		writeJSON(w, http.StatusBadRequest, nil, "usuario_id é obrigatório na URL")
		return
	}

	usuarioID, err := strconv.ParseInt(usuarioIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "usuario_id inválido")
		return
	}

	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)
	orderBy := strings.TrimSpace(r.URL.Query().Get("order_by"))
	orderDir := parseOrderDirQuery(r.URL.Query().Get("order_dir"))

	ctx := r.Context()
	historicos, total, err := h.svc.ListarPorUsuario(ctx, h.db, usuarioID, page, limit, orderBy, orderDir)
	if err != nil {
		log.Printf("[senha-historico] ListarPorUsuario: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	out := make([]map[string]any, 0, len(historicos))
	for _, h := range historicos {
		// senha_hash_anterior não é exposto na resposta HTTP (ver nota em ListarTodos).
		item := map[string]any{
			"id":         h.ID,
			"usuario_id": h.UsuarioID,
			"ip_origem":  h.IPOrigem,
			"user_agent": h.UserAgent,
			"tipo_reset": h.TipoReset,
			"created_at": h.CreatedAt,
		}
		if h.ResetadoPorID.Valid {
			item["resetado_por_id"] = h.ResetadoPorID.Int64
		}
		out = append(out, item)
	}

	writeJSONWithPagination(w, http.StatusOK, out, page, limit, total, pages)
}
