package httpx

import (
	"net"
	"net/http"
)

// SecurityHeaders adiciona os headers de segurança a toda resposta
// (ex-middleware.SecurityHeadersMiddleware, F-03). A API só serve JSON, então
// a CSP é restritiva. HSTS só fora de localhost/loopback (dev em HTTP puro).
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if !hostLocal(r.Host) {
			// max-age de 180 dias; includeSubDomains por boa prática.
			h.Set("Strict-Transport-Security", "max-age=15552000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// hostLocal reconhece os hosts de desenvolvimento (com ou sem porta, IPv4 ou
// IPv6) pelo valor EXATO, sem prefix-match.
func hostLocal(host string) bool {
	switch semPorta(host) {
	case "localhost", "127.0.0.1", "0.0.0.0", "::1", "0:0:0:0:0:0:0:1":
		return true
	default:
		return false
	}
}

// semPorta remove a porta de "host:porta" e "[::1]:porta"; "::1" (IPv6 sem
// colchetes e sem porta) volta como está.
func semPorta(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
