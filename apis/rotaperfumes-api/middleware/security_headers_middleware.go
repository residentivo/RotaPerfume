// Package middleware contém middlewares HTTP compartilhados (CORS, auth, log).
package middleware

import (
	"net"
	"net/http"

	"github.com/rotaperfumes/shared/vlog"
)

// SecurityHeadersMiddleware adiciona headers de segurança HTTP padrão a todas
// as respostas. Como a API só serve JSON (nunca HTML), a CSP é restritiva
// (default-src 'none'). Strict-Transport-Security só é enviado quando a
// requisição não é para localhost/127.0.0.1/0.0.0.0 — em dev local a conexão
// costuma ser HTTP puro e HSTS quebraria o acesso.
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vlog.Printf("security_headers_middleware.go", "SecurityHeadersMiddleware.func.func", "chamando w.Header e declarando h")
			h := w.Header()
			vlog.Printf("security_headers_middleware.go", "SecurityHeadersMiddleware.func.func", "chamando h.Set")
			h.Set("X-Content-Type-Options", "nosniff")
			vlog.Printf("security_headers_middleware.go", "SecurityHeadersMiddleware.func.func", "chamando h.Set")
			h.Set("X-Frame-Options", "DENY")
			vlog.Printf("security_headers_middleware.go", "SecurityHeadersMiddleware.func.func", "chamando h.Set")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			vlog.Printf("security_headers_middleware.go", "SecurityHeadersMiddleware.func.func", "chamando h.Set")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

			vlog.Printf("security_headers_middleware.go", "SecurityHeadersMiddleware.func.func", "verificando condição !isLocalHost(r.Host)")
			if !isLocalHost(r.Host) {
				// max-age de 180 dias; includeSubDomains por padrão de boa prática.
				vlog.Printf("security_headers_middleware.go", "SecurityHeadersMiddleware.func.func", "chamando h.Set")
				h.Set("Strict-Transport-Security", "max-age=15552000; includeSubDomains")
			}

			vlog.Printf("security_headers_middleware.go", "SecurityHeadersMiddleware.func.func", "chamando next.ServeHTTP")
			next.ServeHTTP(w, r)
		})
	}
}

// isLocalHost verifica se o Host da requisição é um endereço de desenvolvimento
// local (com ou sem porta, IPv4 ou IPv6), para não enviar HSTS em ambiente de
// dev sem HTTPS.
func isLocalHost(host string) bool {
	vlog.Printf("security_headers_middleware.go", "isLocalHost", "chamando stripHostPort e declarando h")
	h := stripHostPort(host)
	vlog.Printf("security_headers_middleware.go", "isLocalHost", "avaliando switch sobre h")
	switch h {
	case "localhost", "127.0.0.1", "0.0.0.0", "::1", "0:0:0:0:0:0:0:1":
		return true
	default:
		return false
	}
}

// stripHostPort remove a porta de um Host HTTP, suportando tanto endereços
// IPv4/hostname ("host:porta") quanto IPv6 com porta ("[::1]:porta") e IPv6
// sem porta ("::1", que não deve ser confundido com host:porta por
// net.SplitHostPort). Se não houver porta identificável, retorna o valor
// original.
func stripHostPort(host string) string {
	vlog.Printf("security_headers_middleware.go", "stripHostPort", "chamando net.SplitHostPort e declarando h, _, err e verificando condição err == nil")
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	// net.SplitHostPort falha para "::1" (sem colchetes, sem porta) — nesse
	// caso o host já é o próprio endereço IPv6, sem porta a remover.
	return host
}
