// Package middleware contém middlewares HTTP reutilizáveis.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

// ctxKey é o tipo das chaves de contexto para evitar colisões.
type ctxKey string

const (
	// Context keys — valores únicos para injeção no request context.
	keyUserID ctxKey = "userID"
	keyRole   ctxKey = "role"
)

// JWTMiddleware extrai e valida o token JWT do header Authorization: Bearer <token>.
// Se válido, injeta userID e role no contexto da requisição.
// Se a rota é protegida e o token falta/inválido, retorna 401.
// Se a rota requer role "admin" e o usuário não é admin, retorna 403.
// Valida expiração do access token (campo ExpiresAt do JWT) — retorna 401 com mensagem específica se expirado.
func JWTMiddleware(cfg *config.Config, protected bool, requireAdmin bool) func(http.Handler) http.Handler {
	auth := services.NewAuthService()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				if protected {
					writeError(w, http.StatusUnauthorized, "authorization header ausente")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				if protected {
					writeError(w, http.StatusUnauthorized, "authorization header mal formado")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			claims, err := auth.ValidateJWT(parts[1], cfg.JWTSecret)
			if err != nil {
				if protected {
					// Distingue token expirado de inválido para melhor UX no frontend.
					if errors.Is(err, jwt.ErrTokenExpired) {
						writeError(w, http.StatusUnauthorized, "access token expirado — use /api/auth/refresh")
						return
					}
					writeError(w, http.StatusUnauthorized, "token inválido ou expirado")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Validação defensiva adicional de expiração.
			if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(timeNow()) {
				if protected {
					writeError(w, http.StatusUnauthorized, "access token expirado — use /api/auth/refresh")
					return
				}
			}

			if requireAdmin && claims.Role != "admin" {
				writeError(w, http.StatusForbidden, "acesso restrito a administradores")
				return
			}

			// Injeta claims no contexto para os handlers downstream.
			ctx := context.WithValue(r.Context(), keyUserID, claims.UserID)
			ctx = context.WithValue(ctx, keyRole, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// timeNow é uma var para facilitar testes (sobrescrevível).
var timeNow = func() time.Time { return time.Now() }

// GetUserID extrai o userID injetado pelo middleware.
func GetUserID(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(keyUserID).(int64)
	return v, ok
}

// GetRole extrai o role injetado pelo middleware.
func GetRole(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyRole).(string)
	return v, ok
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"success":false,"error":"` + msg + `"}` + "\n"))
}
