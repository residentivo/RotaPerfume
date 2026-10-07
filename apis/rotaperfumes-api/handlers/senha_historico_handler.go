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
	"github.com/rotaperfumes/shared/vlog"
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
// order_by (id|usuario_id|usuario_nome|resetado_por_nome|ip_origem|tipo_reset|created_at; default id; desempate id desc),
// order_dir (asc|desc; default desc).
// Response: {success, data: [{id, usuario_id, usuario_nome, resetado_por_id, resetado_por_nome, ip_origem, user_agent, tipo_reset, created_at}], pagination}
// Admin only.
func (h *SenhaHistoricoHandler) ListarTodos(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "chamando middleware.GetRole e atribuindo resultado a role, ok")
	role, ok := middleware.GetRole(r.Context())
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "verificando se !ok || role != models.RoleAdmin")
	if !ok || role != models.RoleAdmin {
		writeJSON(w, http.StatusForbidden, nil, "acesso restrito a administradores")
		return
	}

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "chamando services.ParsePagination e atribuindo resultado a page, limit")
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "chamando strings.TrimSpace e atribuindo resultado a orderBy")
	orderBy := strings.TrimSpace(r.URL.Query().Get("order_by"))
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "chamando parseOrderDirQuery e atribuindo resultado a orderDir")
	orderDir := parseOrderDirQuery(r.URL.Query().Get("order_dir"))

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "chamando r.Context e atribuindo resultado a ctx")
	ctx := r.Context()
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "chamando h.svc.ListarTodos e atribuindo resultado a historicos, total, err")
	historicos, total, err := h.svc.ListarTodos(ctx, h.db, page, limit, orderBy, orderDir)
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "verificando se err != nil")
	if err != nil {
		log.Printf("[senha-historico] ListarTodos: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "definindo pages := total / limit")
	pages := total / limit
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "verificando se total%%limit != 0")
	if total%limit != 0 {
		vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "incrementando pages")
		pages++
	}

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "chamando make e atribuindo resultado a out")
	out := make([]map[string]any, 0, len(historicos))
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "iniciando loop sobre historicos")
	for _, h := range historicos {
		// Nota: senha_hash_anterior (Argon2id ou bcrypt legado) é mantido no model para uso
		// interno/auditoria em banco, mas NUNCA deve ser serializado na
		// resposta HTTP — mesmo sendo um hash, sua exposição facilita
		// ataques offline (ex. em caso de vazamento de logs/rede).
		item := map[string]any{
			"id":           h.ID,
			"usuario_id":   h.UsuarioID,
			"usuario_nome": h.UsuarioNome,
			"ip_origem":    h.IPOrigem,
			"user_agent":   h.UserAgent,
			"tipo_reset":   h.TipoReset,
			"created_at":   h.CreatedAt,
		}
		if h.ResetadoPorID.Valid {
			item["resetado_por_id"] = h.ResetadoPorID.Int64
		}
		if h.ResetadoPorNome.Valid {
			item["resetado_por_nome"] = h.ResetadoPorNome.String
		}
		out = append(out, item)
	}

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarTodos", "loop sobre historicos concluído: %d itens", len(historicos))
	writeJSONWithPagination(w, http.StatusOK, out, page, limit, total, pages)
}

// ListarPorUsuario GET /api/senha-historico/{usuario_id}
//
// Query params: page (default 1), limit (default 20, max 100),
// order_by (id|usuario_id|usuario_nome|resetado_por_nome|ip_origem|tipo_reset|created_at; default id; desempate id desc),
// order_dir (asc|desc; default desc).
// Response: {success, data: [...], pagination}
// Admin only.
func (h *SenhaHistoricoHandler) ListarPorUsuario(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "chamando middleware.GetRole e atribuindo resultado a role, ok")
	role, ok := middleware.GetRole(r.Context())
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "verificando se !ok || role != models.RoleAdmin")
	if !ok || role != models.RoleAdmin {
		writeJSON(w, http.StatusForbidden, nil, "acesso restrito a administradores")
		return
	}

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "definindo path := r.URL.Path")
	// Extrai usuario_id da path.
	path := r.URL.Path
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "chamando strings.Split e atribuindo resultado a parts")
	parts := strings.Split(path, "/")
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "declarando variável usuarioIDStr")
	var usuarioIDStr string
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "iniciando loop sobre parts")
	for i, p := range parts {
		if p == "senha-historico" && i+1 < len(parts) {
			usuarioIDStr = parts[i+1]
			break
		}
	}
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "loop sobre parts concluído: %d itens", len(parts))
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "verificando se usuarioIDStr == \"\"")
	if usuarioIDStr == "" {
		writeJSON(w, http.StatusBadRequest, nil, "usuario_id é obrigatório na URL")
		return
	}

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "chamando strconv.ParseInt e atribuindo resultado a usuarioID, err")
	usuarioID, err := strconv.ParseInt(usuarioIDStr, 10, 64)
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "usuario_id inválido")
		return
	}

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "chamando services.ParsePagination e atribuindo resultado a page, limit")
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "chamando strings.TrimSpace e atribuindo resultado a orderBy")
	orderBy := strings.TrimSpace(r.URL.Query().Get("order_by"))
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "chamando parseOrderDirQuery e atribuindo resultado a orderDir")
	orderDir := parseOrderDirQuery(r.URL.Query().Get("order_dir"))

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "chamando r.Context e atribuindo resultado a ctx")
	ctx := r.Context()
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "chamando h.svc.ListarPorUsuario e atribuindo resultado a historicos, total, err")
	historicos, total, err := h.svc.ListarPorUsuario(ctx, h.db, usuarioID, page, limit, orderBy, orderDir)
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "verificando se err != nil")
	if err != nil {
		log.Printf("[senha-historico] ListarPorUsuario: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "definindo pages := total / limit")
	pages := total / limit
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "verificando se total%%limit != 0")
	if total%limit != 0 {
		vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "incrementando pages")
		pages++
	}

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "chamando make e atribuindo resultado a out")
	out := make([]map[string]any, 0, len(historicos))
	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "iniciando loop sobre historicos")
	for _, h := range historicos {
		// senha_hash_anterior não é exposto na resposta HTTP (ver nota em ListarTodos).
		item := map[string]any{
			"id":           h.ID,
			"usuario_id":   h.UsuarioID,
			"usuario_nome": h.UsuarioNome,
			"ip_origem":    h.IPOrigem,
			"user_agent":   h.UserAgent,
			"tipo_reset":   h.TipoReset,
			"created_at":   h.CreatedAt,
		}
		if h.ResetadoPorID.Valid {
			item["resetado_por_id"] = h.ResetadoPorID.Int64
		}
		if h.ResetadoPorNome.Valid {
			item["resetado_por_nome"] = h.ResetadoPorNome.String
		}
		out = append(out, item)
	}

	vlog.Printf("senha_historico_handler.go", "SenhaHistoricoHandler.ListarPorUsuario", "loop sobre historicos concluído: %d itens", len(historicos))
	writeJSONWithPagination(w, http.StatusOK, out, page, limit, total, pages)
}
