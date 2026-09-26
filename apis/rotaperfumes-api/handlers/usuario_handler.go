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
	"github.com/rotaperfumes/shared/repositories"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

// UsuarioHandler trata as rotas /api/usuarios/*.
type UsuarioHandler struct {
	db         *sql.DB
	cfg        *config.Config
	svc        *services.UsuarioService
	senhaSvc   *services.SenhaHistoricoService
	refreshSvc *services.RefreshTokenService
}

// NewUsuarioHandler cria um UsuarioHandler com pool de conexão injetado.
// emailSvc é o serviço usado para enviar a senha inicial/reset por email
// (injete sharedsvc.NewNoopEmailService() quando SMTP não estiver configurado).
func NewUsuarioHandler(db *sql.DB, cfg *config.Config, emailSvc sharedsvc.EmailService) *UsuarioHandler {
	return &UsuarioHandler{
		db:         db,
		cfg:        cfg,
		svc:        services.NewUsuarioService(db, cfg, emailSvc),
		senhaSvc:   services.NewSenhaHistoricoService(),
		refreshSvc: services.NewRefreshTokenService(),
	}
}

// ListUsuarios GET /api/usuarios
//
// Query params: page (default 1), limit (default 20, max 100),
// order_by (id|nome|email|role|ativo|created_at|updated_at|ultimo_login_at;
// default id), order_dir (asc|desc; default asc).
// Response: {success, data: [{id, id_vendedor, vendedor_nome, email, role, ativo, nome, created_at, ultimo_login_at}], error, pagination: {page, limit, total, pages}}
//
// Exclui password_hash de todas as respostas.
func (h *UsuarioHandler) ListUsuarios(w http.ResponseWriter, r *http.Request) {
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

	usuarios, total, err := h.svc.ListUsuarios(r.Context(), h.db, page, limit, orderBy, orderDir)
	if err != nil {
		log.Printf("[usuarios] ListUsuarios: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	// Transforma para output, omitindo password_hash.
	out := make([]map[string]any, 0, len(usuarios))
	for _, u := range usuarios {
		out = append(out, map[string]any{
			"id":              u.ID,
			"id_vendedor":     u.IDVendedor,
			"vendedor_nome":   vendedorNomeOuVazio(u.VendedorNome),
			"email":           u.Email,
			"role":            u.Role,
			"ativo":           u.Ativo,
			"nome":            u.Nome,
			"created_at":      u.CreatedAt,
			"ultimo_login_at": u.UltimoLoginAt,
		})
	}

	writeJSONWithPagination(w, http.StatusOK, out, page, limit, total, pages)
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CreateUsuarioRequest body do POST /api/usuarios.
type CreateUsuarioRequest struct {
	Nome       string `json:"nome"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	IDVendedor *int64 `json:"id_vendedor"` // opcional: null/omitido = sem vendedor
}

// UpdateUsuarioRequest body do PUT /api/usuarios/{id}.
type UpdateUsuarioRequest struct {
	Nome       string `json:"nome"`
	Role       string `json:"role"`
	IDVendedor *int64 `json:"id_vendedor"` // opcional: null/omitido = sem vendedor
}

// SetAtivoRequest body do PATCH /api/usuarios/{id}/inativar.
type SetAtivoRequest struct {
	Ativo *bool `json:"ativo"` // omitido = toggle
}

// AdminResetPasswordRequest body do POST /api/admin/reset-password.
type AdminResetPasswordRequest struct {
	UsuarioID int64 `json:"usuario_id"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// usuarioToMap transforma um Usuario em map de output (sem password_hash).
func usuarioToMap(u *models.Usuario) map[string]any {
	return map[string]any{
		"id":              u.ID,
		"id_vendedor":     u.IDVendedor,
		"vendedor_nome":   vendedorNomeOuVazio(u.VendedorNome),
		"email":           u.Email,
		"role":            u.Role,
		"ativo":           u.Ativo,
		"nome":            u.Nome,
		"created_at":      u.CreatedAt,
		"updated_at":      u.UpdatedAt,
		"ultimo_login_at": u.UltimoLoginAt,
	}
}

// vendedorNomeOuVazio retorna o nome do vendedor vinculado ou string vazia
// quando o usuário não tem vendedor associado.
func vendedorNomeOuVazio(nome *string) string {
	if nome == nil {
		return ""
	}
	return *nome
}

// CreateUsuario POST /api/usuarios
//
// Body: { "nome": string, "email": string, "role": "admin"|"normal", "id_vendedor": int|null }
// A senha inicial é gerada aleatoriamente e enviada por email ao endereço
// cadastrado — nunca é retornada nesta resposta.
// Retorna: { id, id_vendedor, vendedor_nome, email, role, ativo, nome, created_at, updated_at, ultimo_login_at, email_enviado }
func (h *UsuarioHandler) CreateUsuario(w http.ResponseWriter, r *http.Request) {
	role, ok := middleware.GetRole(r.Context())
	if !ok || role != models.RoleAdmin {
		writeJSON(w, http.StatusForbidden, nil, "acesso restrito a administradores")
		return
	}

	var req CreateUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}
	req.Nome = strings.TrimSpace(req.Nome)
	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.TrimSpace(req.Role)

	if req.Nome == "" {
		writeJSON(w, http.StatusBadRequest, nil, "nome é obrigatório")
		return
	}
	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, nil, "email é obrigatório")
		return
	}
	if req.Role != models.RoleAdmin && req.Role != models.RoleNormal {
		writeJSON(w, http.StatusBadRequest, nil, "role deve ser 'admin' ou 'normal'")
		return
	}

	ctx := r.Context()
	u, emailEnviado, err := h.svc.CreateUsuario(ctx, h.db, struct {
		Nome       string
		Email      string
		Role       string
		IDVendedor *int64
	}{req.Nome, req.Email, req.Role, req.IDVendedor})
	if err != nil {
		if errors.Is(err, services.ErrEmailDuplicado) {
			writeJSON(w, http.StatusConflict, nil, "email já cadastrado")
			return
		}
		if errors.Is(err, services.ErrRoleInvalido) {
			writeJSON(w, http.StatusBadRequest, nil, "role deve ser 'admin' ou 'normal'")
			return
		}
		if errors.Is(err, services.ErrEmailInvalido) {
			writeJSON(w, http.StatusBadRequest, nil, "email inválido")
			return
		}
		if errors.Is(err, services.ErrVendedorNaoEncontrado) {
			writeJSON(w, http.StatusBadRequest, nil, "vendedor não encontrado")
			return
		}
		log.Printf("[usuarios] CreateUsuario: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	if !emailEnviado {
		log.Printf("[usuarios] ATENÇÃO: usuário id=%d criado, mas email com senha inicial NÃO foi enviado", u.ID)
	}

	log.Printf("[usuarios] criado: id=%d por admin=%s email_enviado=%t", u.ID, role, emailEnviado)
	out := usuarioToMap(u)
	out["email_enviado"] = emailEnviado
	writeJSON(w, http.StatusCreated, out, "")
}

// UpdateUsuario PUT /api/usuarios/{id}
//
// Body: { "nome": string, "role": "admin"|"normal", "id_vendedor": int|null }
// Retorna: usuário atualizado
func (h *UsuarioHandler) UpdateUsuario(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}
	req.Nome = strings.TrimSpace(req.Nome)
	req.Role = strings.TrimSpace(req.Role)

	if req.Nome == "" {
		writeJSON(w, http.StatusBadRequest, nil, "nome é obrigatório")
		return
	}
	if req.Role != models.RoleAdmin && req.Role != models.RoleNormal {
		writeJSON(w, http.StatusBadRequest, nil, "role deve ser 'admin' ou 'normal'")
		return
	}

	ctx := r.Context()
	u, err := h.svc.UpdateUsuario(ctx, h.db, id, req.Nome, req.Role, req.IDVendedor)
	if err != nil {
		if errors.Is(err, services.ErrUsuarioNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "usuário não encontrado")
			return
		}
		if errors.Is(err, services.ErrRoleInvalido) {
			writeJSON(w, http.StatusBadRequest, nil, "role deve ser 'admin' ou 'normal'")
			return
		}
		if errors.Is(err, services.ErrVendedorNaoEncontrado) {
			writeJSON(w, http.StatusBadRequest, nil, "vendedor não encontrado")
			return
		}
		log.Printf("[usuarios] UpdateUsuario: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[usuarios] atualizado: id=%d por admin=%s", id, role)
	writeJSON(w, http.StatusOK, usuarioToMap(u), "")
}

// ToggleAtivoUsuario PATCH /api/usuarios/{id}/inativar
//
// Body opcional: { "ativo": bool } — omitido = toggle
// Retorna: usuário atualizado
func (h *UsuarioHandler) ToggleAtivoUsuario(w http.ResponseWriter, r *http.Request) {
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

	ativo, ok := lerAtivoOpcional(w, r) // vazio/null/{} = toggle; inválido = 400
	if !ok {
		return
	}

	ctx := r.Context()
	u, err := h.svc.ToggleAtivoUsuario(ctx, h.db, id, ativo)
	if err != nil {
		if errors.Is(err, services.ErrUsuarioNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "usuário não encontrado")
			return
		}
		log.Printf("[usuarios] ToggleAtivoUsuario: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	// SEC-06: usuário inativado perde as sessões de refresh na hora (ativar
	// não revoga). Falha na revogação não desfaz a inativação — o middleware
	// já bloqueia o usuário inativo em todo request protegido —, mas é logada.
	if !u.Ativo {
		if err := h.refreshSvc.RevokeAllUserTokens(ctx, h.db, id, repositories.RevokeReasonInativacao); err != nil {
			log.Printf("[usuarios] ToggleAtivoUsuario: falha ao revogar refresh tokens do usuario_id=%d: %v", id, err)
		}
	}

	log.Printf("[usuarios] ativo=%t: id=%d por admin=%s", u.Ativo, id, role)
	writeJSON(w, http.StatusOK, usuarioToMap(u), "")
}

// AdminResetPassword POST /api/admin/reset-password
//
// Body: { "usuario_id": int }
// Nova senha: gerada aleatoriamente e enviada por email ao endereço
// cadastrado do usuário — nunca é retornada nesta resposta.
// Retorna: { sucesso: true, mensagem: "...", email_enviado: bool }
func (h *UsuarioHandler) AdminResetPassword(w http.ResponseWriter, r *http.Request) {
	role, ok := middleware.GetRole(r.Context())
	if !ok || role != models.RoleAdmin {
		writeJSON(w, http.StatusForbidden, nil, "acesso restrito a administradores")
		return
	}

	var req AdminResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}
	if req.UsuarioID <= 0 {
		writeJSON(w, http.StatusBadRequest, nil, "usuario_id é obrigatório e deve ser > 0")
		return
	}

	ctx := r.Context()

	// Busca usuário atual para capturar o hash anterior (auditoria).
	targetUser, err := h.svc.GetUsuarioByID(ctx, h.db, req.UsuarioID)
	if err != nil {
		if errors.Is(err, services.ErrUsuarioNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "usuário não encontrado")
			return
		}
		log.Printf("[usuarios] AdminResetPassword: GetByID: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	// Captura adminID do contexto.
	adminID, _ := middleware.GetUserID(r.Context())
	ipOrigem := getClientIP(r, h.cfg.TrustProxyHeaders)
	userAgent := r.UserAgent()

	emailEnviado, err := h.svc.AdminResetPassword(ctx, h.db, req.UsuarioID, targetUser.Email, targetUser.Nome)
	if err != nil {
		if errors.Is(err, services.ErrUsuarioNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "usuário não encontrado")
			return
		}
		log.Printf("[usuarios] AdminResetPassword: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	// Registra no histórico de senhas (tipo "admin").
	if err := h.senhaSvc.Registrar(ctx, h.db, req.UsuarioID, &adminID, targetUser.PasswordHash, ipOrigem, userAgent, "admin"); err != nil {
		log.Printf("[usuarios] AdminResetPassword: falha ao registrar histórico de senha do usuario_id=%d: %v", req.UsuarioID, err)
	}

	// Revoga todos os refresh tokens do usuário após reset (security best practice).
	if err := h.refreshSvc.RevokeAllUserTokens(ctx, h.db, req.UsuarioID, repositories.RevokeReasonSenha); err != nil {
		log.Printf("[usuarios] AdminResetPassword: falha ao revogar refresh tokens do usuario_id=%d: %v", req.UsuarioID, err)
	}

	if !emailEnviado {
		log.Printf("[usuarios] ATENÇÃO: senha de usuario_id=%d resetada, mas email NÃO foi enviado", req.UsuarioID)
	}

	log.Printf("[usuarios] admin resetou senha: usuario_id=%d por admin=%s email_enviado=%t", req.UsuarioID, role, emailEnviado)
	writeJSON(w, http.StatusOK, map[string]any{
		"sucesso":       true,
		"mensagem":      "Senha resetada e enviada por email ao usuário",
		"email_enviado": emailEnviado,
	}, "")
}
