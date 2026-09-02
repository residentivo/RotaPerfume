package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"backend/crm/internal/models"
	"backend/crm/internal/services"
)

// AuthHandler gerencia as requisições de autenticação
type AuthHandler struct {
	authService *services.AuthService
	jwtSecret  string
}

// NewAuthHandler cria um novo handler de autenticação
func NewAuthHandler(authService *services.AuthService, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		jwtSecret:   jwtSecret,
	}
}

// Login processa a requisição de login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"erro": "requisição inválida"}`, http.StatusBadRequest)
		return
	}

	usuario, err := h.authService.Login(r.Context(), req.Login, req.Senha)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusUnauthorized)
		return
	}

	// Gera o token JWT
	claims := jwt.MapClaims{
		"id":           usuario.ID,
		"login":        usuario.Login,
		"tipo_usuario": usuario.TipoUsuario,
		"exp":          time.Now().Add(24 * time.Hour).Unix(),
		"iat":          time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		http.Error(w, `{"erro": "erro ao gerar token"}`, http.StatusInternalServerError)
		return
	}

	response := models.LoginResponse{
		Token:   tokenString,
		Usuario: usuario,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
