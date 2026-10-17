package httpx

import "net/http"

// CORS devolve o middleware de CORS (ex-middleware.CORSMiddleware).
//
// F-02: a origem só é refletida se estiver EXATAMENTE (sem prefix-match) na
// lista — normalmente config.Config.CORSAllowedOrigins. "*" na lista libera
// qualquer origem (GAP SEC-17, preservado).
//
// Invariante O3: TODO request OPTIONS recebe 204 aqui, antes do mux, com ou
// sem Origin e com origem permitida ou não. Um preflight nunca chega ao mux,
// nunca vira 405 e nunca consulta o checker de autenticação.
func CORS(origensPermitidas []string) Middleware {
	permitidas := make(map[string]bool, len(origensPermitidas))
	curinga := false
	for _, o := range origensPermitidas {
		if o == "*" {
			curinga = true
			continue
		}
		permitidas[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origem := r.Header.Get("Origin")
			if origem != "" && (curinga || permitidas[origem]) {
				escreverHeadersCORS(w.Header(), origem)
			}
			// Vary: Origin — evita cache incorreto de respostas entre origens.
			w.Header().Add("Vary", "Origin")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func escreverHeadersCORS(h http.Header, origem string) {
	h.Set("Access-Control-Allow-Origin", origem)
	h.Set("Access-Control-Allow-Credentials", "true")
	h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, Accept, Origin")
	h.Set("Access-Control-Max-Age", "86400")
	h.Set("Access-Control-Expose-Headers", "Set-Cookie")
}
