package httpx

import (
	"net"
	"net/http"
	"strings"
)

// IPDoCliente devolve o IP de origem (chaves de rate limit e auditoria,
// I-14). X-Forwarded-For/X-Real-IP só valem com TRUST_PROXY_HEADERS=true
// (atrás de proxy confiável): por padrão são ignorados, porque o cliente os
// forja. Do XFF vale o primeiro da lista (SEC-20, GAP preservado). Sem
// proxy, RemoteAddr sem a porta (cada conexão nova tem porta efêmera).
func IPDoCliente(r *http.Request, confiarProxy bool) string {
	if confiarProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
		if real := r.Header.Get("X-Real-IP"); real != "" {
			return real
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
