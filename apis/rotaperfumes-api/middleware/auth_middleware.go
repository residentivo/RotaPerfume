// Package middleware contém middlewares HTTP reutilizáveis.
package middleware

import (
	"context"
	"errors"
	"log"
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
//
// Esta variante confia apenas no JWT (não consulta o banco). O router da API
// usa JWTMiddlewareWithUserCheck (SEC-06); esta assinatura é mantida para
// rotas públicas e testes unitários que exercitam só a validação do token.
func JWTMiddleware(cfg *config.Config, protected bool, requireAdmin bool) func(http.Handler) http.Handler {
	return newJWTMiddleware(cfg, nil, protected, requireAdmin)
}

// JWTMiddlewareWithUserCheck é o JWTMiddleware com checagem do usuário no
// banco a cada request protegido (SEC-06). Depois de validar o JWT, consulta
// ativo/role via checker (sem cache) e aplica regras fail-closed:
//   - JWT ausente/inválido/expirado → 401, SEM consultar o banco;
//   - usuário inexistente ou inativo → 401 "usuário inativo";
//   - erro ao consultar → 500, sem chamar o handler;
//   - o role do BANCO (não o do token) vai para o contexto e para o
//     requireAdmin — rebaixar admin→normal vale imediatamente.
//
// Em rotas não protegidas o checker não é chamado. checker nil é erro de
// programação (panic na montagem das rotas, nunca em runtime).
func JWTMiddlewareWithUserCheck(cfg *config.Config, checker UserStatusChecker, protected bool, requireAdmin bool) func(http.Handler) http.Handler {
	if checker == nil {
		panic("middleware: JWTMiddlewareWithUserCheck exige um UserStatusChecker não nulo")
	}
	return newJWTMiddleware(cfg, checker, protected, requireAdmin)
}

// newJWTMiddleware monta o middleware; checker nil = confia no role do JWT.
func newJWTMiddleware(cfg *config.Config, checker UserStatusChecker, protected bool, requireAdmin bool) func(http.Handler) http.Handler {
	auth := services.NewAuthService()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := ""
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
					if protected {
						writeError(w, http.StatusUnauthorized, "authorization header mal formado")
						return
					}
					next.ServeHTTP(w, r)
					return
				}
				tokenString = parts[1]
			} else if c, err := r.Cookie("access_token"); err == nil && c.Value != "" {
				// Sem header Authorization: usa o cookie HttpOnly access_token
				// (o frontend não consegue ler esse cookie via JS para montar o header).
				tokenString = c.Value
			}

			if tokenString == "" {
				if protected {
					writeError(w, http.StatusUnauthorized, "authorization header ausente")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			claims, err := auth.ValidateJWT(tokenString, cfg.JWTSecret)
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

			role := claims.Role
			if checker != nil && protected {
				dbRole, ok := checkUserStatus(w, r, checker, claims.UserID, claims.Role)
				if !ok {
					return
				}
				role = dbRole
			}

			if requireAdmin && role != "admin" {
				writeError(w, http.StatusForbidden, "acesso restrito a administradores")
				return
			}

			// Injeta claims no contexto para os handlers downstream.
			ctx := context.WithValue(r.Context(), keyUserID, claims.UserID)
			ctx = context.WithValue(ctx, keyRole, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// checkUserStatus consulta o usuário via checker e, se ele puder seguir,
// devolve o role vigente no banco. Em qualquer outro caso já escreve a
// resposta de erro (401/500) e devolve ok=false — o handler não é chamado.
func checkUserStatus(w http.ResponseWriter, r *http.Request, checker UserStatusChecker, userID int64, tokenRole string) (string, bool) {
	st, err := checker.CheckUserStatus(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			log.Printf("[auth] acesso negado: user_id=%d inexistente (token válido) %s %s", userID, r.Method, r.URL.Path)
			writeError(w, http.StatusUnauthorized, msgUsuarioInativo)
			return "", false
		}
		log.Printf("[auth] erro ao verificar usuário user_id=%d: %v", userID, err)
		writeError(w, http.StatusInternalServerError, "erro interno")
		return "", false
	}
	if !st.Ativo {
		log.Printf("[auth] acesso negado: user_id=%d inativo %s %s", userID, r.Method, r.URL.Path)
		writeError(w, http.StatusUnauthorized, msgUsuarioInativo)
		return "", false
	}
	if st.Role != tokenRole {
		log.Printf("[auth] role do token divergente do banco: user_id=%d token=%s banco=%s (usando banco)", userID, tokenRole, st.Role)
	}
	return st.Role, true
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

// RequireAdmin returns a middleware that checks if the user has admin role.
// This is for use with the existing JWTMiddleware pattern in this codebase.
func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get role from context (set by JWTMiddleware)
			roleCtx := r.Context().Value(keyRole)
			if roleCtx == nil {
				writeError(w, http.StatusUnauthorized, "Não autenticado")
				return
			}

			role, ok := roleCtx.(string)
			if !ok || role != "admin" {
				writeError(w, http.StatusForbidden, "Acesso negado. Requer permissão admin.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"success":false,"error":"` + msg + `"}` + "\n"))
}
