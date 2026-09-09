// Package middleware contém middlewares HTTP compartilhados (CORS, auth, log).
package middleware

import (
	"net/http"
	"strings"
)

// CORSMiddleware retorna um handler que adiciona headers CORS para frontend Next.js.
func CORSMiddleware(allowedOrigins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Permite qualquer origin localhost (dev: Next.js em :3000).
			// Em produção, troque allowedOrigins pelo domínio do frontend.
			if origin != "" && (strings.HasPrefix(origin, "http://localhost") ||
				strings.HasPrefix(origin, "http://127.0.0.1") ||
				allowedOrigins == "*" ||
				strings.Contains(allowedOrigins, origin) ||
				allowedOrigins == "") {

				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers",
					"Authorization, Content-Type, X-Requested-With, Accept, Origin")
				// Necessario para frontend ler cookies em cross-origin requests.
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")
				// Expose cookies para o frontend via document.cookie (so os que nao sao HttpOnly).
				// Tokens HttpOnly nao sao acessiveis via JS, mas metadados podem ser uteis.
				w.Header().Set("Access-Control-Expose-Headers", "Set-Cookie")
			}

			// Responde preflight OPTIONS imediatamente.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
