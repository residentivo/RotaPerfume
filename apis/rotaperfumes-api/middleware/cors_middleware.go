// Package middleware contém middlewares HTTP compartilhados (CORS, auth, log).
package middleware

import (
	"net/http"
)

// CORSMiddleware retorna um handler que adiciona headers CORS para o frontend.
//
// A origem da requisição só é aceita se estiver EXATAMENTE (sem prefix-match)
// na lista allowedOrigins — normalmente config.Config.CORSAllowedOrigins, que
// já inclui as origens de dev local padrão. "*" na lista libera qualquer
// origem (não recomendado com credentials=true; use apenas se necessário).
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	wildcard := false
	for _, o := range allowedOrigins {
		if o == "*" {
			wildcard = true
			continue
		}
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" && (wildcard || allowed[origin]) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers",
					"Authorization, Content-Type, X-Requested-With, Accept, Origin")
				w.Header().Set("Access-Control-Max-Age", "86400")
				// Expose cookies para o frontend via document.cookie (so os que nao sao HttpOnly).
				// Tokens HttpOnly nao sao acessiveis via JS, mas metadados podem ser uteis.
				w.Header().Set("Access-Control-Expose-Headers", "Set-Cookie")
			}

			// Vary: Origin — evita cache incorreto de respostas CORS entre origens distintas.
			w.Header().Add("Vary", "Origin")

			// Responde preflight OPTIONS imediatamente.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
