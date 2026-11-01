package routes_test

// ARQ-01 (TestBrain, validação por mutação): fixa a ORDEM da cadeia real
// CORS → SecurityHeaders → mux devolvida por bootstrap.HandlerHTTP.
//
// Na linha de base (cd12d47, routes.NewMux) a cadeia era
// corsChain(securityChain(mux)): o preflight OPTIONS termina no CORS (204) sem
// passar pelo SecurityHeaders. Inverter a ordem continuava devolvendo 204 e
// passava em todos os testes existentes, mas mudava o contrato HTTP (o
// preflight passava a levar os security headers). Este teste prova a ordem
// pelo comportamento observável:
//   - preflight: sem security headers (o CORS é o mais externo e responde);
//   - request comum: CORS e security headers presentes (ambos na cadeia).
//
// Se a mudança de contrato for desejada (headers também no preflight), ela
// exige decisão do SecBrain e atualização explícita deste teste.

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var arq01HeadersDeSeguranca = []string{
	"X-Content-Type-Options",
	"X-Frame-Options",
	"Referrer-Policy",
	"Content-Security-Policy",
	"Strict-Transport-Security",
}

func TestARQ01_Ordem_CORSMaisExternoQueSecurityHeaders(t *testing.T) {
	casos := []struct {
		nome          string
		metodo        string
		path          string
		origin        string
		status        int
		comACAO       bool
		comSegHeaders bool
	}{
		{"preflight origem permitida", http.MethodOptions, "/api/produtos", arq00OrigemPermitida, http.StatusNoContent, true, false},
		{"preflight origem negada", http.MethodOptions, "/api/produtos", arq00OrigemNegada, http.StatusNoContent, false, false},
		{"preflight sem origin", http.MethodOptions, "/api/auth/login", "", http.StatusNoContent, false, false},
		{"preflight rota admin", http.MethodOptions, "/api/usuarios", arq00OrigemPermitida, http.StatusNoContent, true, false},
		{"GET publico origem permitida", http.MethodGet, "/health", arq00OrigemPermitida, http.StatusOK, true, true},
		{"GET protegido sem token origem permitida", http.MethodGet, "/api/produtos", arq00OrigemPermitida, http.StatusUnauthorized, true, true},
		{"metodo errado 405 origem permitida", http.MethodGet, "/api/auth/login", arq00OrigemPermitida, http.StatusMethodNotAllowed, true, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			h, _ := arq00Router(t, fakeUserStatusChecker{})
			w := arq00Serve(h, c.metodo, c.path, "", c.origin)

			assert.Equal(t, c.status, w.Code)
			if c.comACAO {
				assert.Equal(t, c.origin, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
			if c.comSegHeaders {
				assertSecurityHeaders(t, w)
				return
			}
			for _, hdr := range arq01HeadersDeSeguranca {
				assert.Empty(t, w.Header().Get(hdr), "preflight não passa pelo SecurityHeaders (CORS é o mais externo): %s", hdr)
			}
		})
	}
}
