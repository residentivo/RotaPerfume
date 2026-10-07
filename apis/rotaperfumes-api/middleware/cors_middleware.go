// Package middleware contém middlewares HTTP compartilhados (CORS, auth, log).
package middleware

import (
	"net/http"

	"github.com/rotaperfumes/shared/vlog"
)

// CORSMiddleware retorna um handler que adiciona headers CORS para o frontend.
//
// A origem da requisição só é aceita se estiver EXATAMENTE (sem prefix-match)
// na lista allowedOrigins — normalmente config.Config.CORSAllowedOrigins, que
// já inclui as origens de dev local padrão. "*" na lista libera qualquer
// origem (não recomendado com credentials=true; use apenas se necessário).
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	vlog.Printf("cors_middleware.go", "CORSMiddleware", "chamando make e declarando allowed")
	allowed := make(map[string]bool, len(allowedOrigins))
	vlog.Printf("cors_middleware.go", "CORSMiddleware", "declarando wildcard com false")
	wildcard := false
	vlog.Printf("cors_middleware.go", "CORSMiddleware", "iniciando loop range sobre allowedOrigins")
	for _, o := range allowedOrigins {
		if o == "*" {
			wildcard = true
			continue
		}
		allowed[o] = true
	}
	vlog.Printf("cors_middleware.go", "CORSMiddleware", "loop range concluído sobre allowedOrigins: %d itens", len(allowedOrigins))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "chamando r.Header.Get e declarando origin")
			origin := r.Header.Get("Origin")

			vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "verificando condição origin != \"\" && (wildcard || allowed[origin])")
			if origin != "" && (wildcard || allowed[origin]) {
				vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "chamando w.Header().Set")
				w.Header().Set("Access-Control-Allow-Origin", origin)
				vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "chamando w.Header().Set")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "chamando w.Header().Set")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
				vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "chamando w.Header().Set")
				w.Header().Set("Access-Control-Allow-Headers",
					"Authorization, Content-Type, X-Requested-With, Accept, Origin")
				vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "chamando w.Header().Set")
				w.Header().Set("Access-Control-Max-Age", "86400")
				// Expose cookies para o frontend via document.cookie (so os que nao sao HttpOnly).
				// Tokens HttpOnly nao sao acessiveis via JS, mas metadados podem ser uteis.
				vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "chamando w.Header().Set")
				w.Header().Set("Access-Control-Expose-Headers", "Set-Cookie")
			}

			// Vary: Origin — evita cache incorreto de respostas CORS entre origens distintas.
			vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "chamando w.Header().Add")
			w.Header().Add("Vary", "Origin")

			// Responde preflight OPTIONS imediatamente.
			vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "verificando condição r.Method == http.MethodOptions")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			vlog.Printf("cors_middleware.go", "CORSMiddleware.func.func", "chamando next.ServeHTTP")
			next.ServeHTTP(w, r)
		})
	}
}
