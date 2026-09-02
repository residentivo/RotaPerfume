package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"backend/rh/internal/middleware"
	"backend/rh/internal/models"
	"backend/rh/internal/services"
)

// AuthHandler lida com as requisições de autenticação
type AuthHandler struct {
	usuarioService *services.UsuarioService
}

// NewAuthHandler cria uma nova instância do handler de autenticação
func NewAuthHandler(usuarioService *services.UsuarioService) *AuthHandler {
	return &AuthHandler{usuarioService: usuarioService}
}

// Login lida com a requisição de login
// POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "requisição inválida"})
		return
	}

	if req.Login == "" || req.Senha == "" {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "login e senha são obrigatórios"})
		return
	}

	resp, err := h.usuarioService.Login(req.Login, req.Senha)
	if err != nil {
		log.Printf("Erro no login: %v", err)
		escreverJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: err.Error()})
		return
	}

	escreverJSON(w, http.StatusOK, resp)
}

// TrocarSenha lida com a requisição de troca de senha
// POST /auth/trocar-senha
func (h *AuthHandler) TrocarSenha(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaimsFromContext(r)
	if !ok {
		escreverJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "não autenticado"})
		return
	}

	var req models.TrocarSenhaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "requisição inválida"})
		return
	}

	if req.SenhaAtual == "" || req.NovaSenha == "" {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "senha atual e nova senha são obrigatórias"})
		return
	}

	if len(req.NovaSenha) < 6 {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "nova senha deve ter pelo menos 6 caracteres"})
		return
	}

	if err := h.usuarioService.TrocarSenha(claims.UsuarioID, req.SenhaAtual, req.NovaSenha); err != nil {
		log.Printf("Erro ao trocar senha para usuário %d: %v", claims.UsuarioID, err)
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	escreverJSON(w, http.StatusOK, models.SuccessResponse{Message: "senha alterada com sucesso"})
}
