package httpx

import (
	"log"
	"net/http"
	"strings"
)

// CORS devolve o middleware de CORS (ex-middleware.CORSMiddleware).
//
// F-02: a origem só é refletida se estiver EXATAMENTE (sem prefix-match) na
// lista — normalmente config.Config.CORSAllowedOrigins.
//
// SEC-17: não há curinga. A resposta leva Allow-Credentials, então "*" ou
// "null" na lista liberaria os cookies de sessão para qualquer site. A
// config já recusa essas entradas (config.ErrCORSOrigemInvalida); aqui, por
// defesa em profundidade, elas são descartadas com um único log na montagem.
//
// Invariante O3: TODO request OPTIONS recebe 204 aqui, antes do mux, com ou
// sem Origin e com origem permitida ou não. Um preflight nunca chega ao mux,
// nunca vira 405 e nunca consulta o checker de autenticação.
func CORS(origensPermitidas []string) Middleware {
	permitidas := make(map[string]bool, len(origensPermitidas))
	ignorou := false
	for _, o := range origensPermitidas {
		if origemCuringa(o) {
			ignorou = true
			continue
		}
		permitidas[o] = true
	}
	if ignorou {
		log.Printf("[cors] entrada curinga/null ignorada na allowlist")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origem := r.Header.Get("Origin")
			if origem != "" && permitidas[origem] {
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

// origemCuringa: "*" e "null" (com trim, sem diferenciar maiúsculas) nunca
// entram na allowlist com credentials (mesma regra da config).
func origemCuringa(o string) bool {
	o = strings.TrimSpace(o)
	return o == "*" || strings.EqualFold(o, "null")
}

func escreverHeadersCORS(h http.Header, origem string) {
	h.Set("Access-Control-Allow-Origin", origem)
	h.Set("Access-Control-Allow-Credentials", "true")
	h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, Accept, Origin")
	h.Set("Access-Control-Max-Age", "86400")
	h.Set("Access-Control-Expose-Headers", "Set-Cookie")
}
