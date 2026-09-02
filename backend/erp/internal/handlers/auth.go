package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"backend/erp/internal/config"
	"backend/erp/internal/models"
	"backend/erp/internal/services"

	"github.com/golang-jwt/jwt/v5"
)

type AuthHandler struct {
	service *services.AuthService
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{service: services.NewAuthService()}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"erro": "Dados invalidos"}`, http.StatusBadRequest)
		return
	}

	usuario, err := h.service.Login(r.Context(), req.Login, req.Senha)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusUnauthorized)
		return
	}

	claims := jwt.MapClaims{
		"id":           usuario.ID,
		"login":        usuario.Login,
		"tipo_usuario": usuario.TipoUsuario,
		"exp":          time.Now().Add(24 * time.Hour).Unix(),
		"iat":          time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.AppConfig.JWTSecret))
	if err != nil {
		http.Error(w, `{"erro": "erro ao gerar token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.LoginResponse{
		Token:       tokenString,
		TipoUsuario: usuario.TipoUsuario,
		Usuario:     usuario,
	})
}
