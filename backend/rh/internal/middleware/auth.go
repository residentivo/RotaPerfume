package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"backend/rh/internal/services"
)

// ContextKey é o tipo para chaves do context
type ContextKey string

const (
	// UserClaimsKey é a chave para as claims do usuário no context
	UserClaimsKey ContextKey = "user_claims"
)

// AuthMiddleware cria um middleware de autenticação JWT
func AuthMiddleware(usuarioService *services.UsuarioService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				escreverErro(w, http.StatusUnauthorized, "token de autenticação não fornecido")
				return
			}

			partes := strings.SplitN(authHeader, " ", 2)
			if len(partes) != 2 || partes[0] != "Bearer" {
				escreverErro(w, http.StatusUnauthorized, "formato de token inválido. Use: Bearer <token>")
				return
			}

			tokenString := partes[1]
			claims, err := usuarioService.ValidarToken(tokenString)
			if err != nil {
				log.Printf("Erro ao validar token: %v", err)
				escreverErro(w, http.StatusUnauthorized, "token inválido ou expirado")
				return
			}

			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole retorna um middleware que valida se o usuário possui um dos tipos permitidos
func RequireRole(usuarioService *services.UsuarioService, tiposPermitidos ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(UserClaimsKey).(*services.JWTClaims)
			if !ok {
				escreverErro(w, http.StatusUnauthorized, "não autenticado")
				return
			}

			for _, tipo := range tiposPermitidos {
				if claims.TipoUsuario == tipo {
					next.ServeHTTP(w, r)
					return
				}
			}

			escreverErro(w, http.StatusForbidden, "acesso negado: permissão insuficiente")
		})
	}
}

// GetClaimsFromContext extrai as claims do usuário do request
func GetClaimsFromContext(r *http.Request) (*services.JWTClaims, bool) {
	claims, ok := r.Context().Value(UserClaimsKey).(*services.JWTClaims)
	return claims, ok
}

func escreverErro(w http.ResponseWriter, status int, mensagem string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": mensagem})
}
