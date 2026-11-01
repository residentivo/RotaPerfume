// Package httpx é o kit HTTP transversal da API (ARQ-01): router com
// política tipada, cadeia de middlewares, CORS, headers de segurança, log de
// acesso, envelope JSON, mapper de erros, leitura de query/path e o
// Autenticador que produz o kernel.Principal.
//
// Regras (docs/arquitetura-ddd.md §4.1): importa kernel, vlog e net/http;
// nunca database/sql nem adapters *mysql. Até o ARQ-03 o Autenticador delega
// ao middleware legado (O4).
package httpx

import "net/http"

// Middleware envolve um handler.
type Middleware func(http.Handler) http.Handler

// Chain aplica do mais externo para o mais interno: Chain(h, A, B) == A(B(h)).
//
// Ordem da API (F-01, montada no bootstrap):
//
//	Logging( CORS( SecurityHeaders( mux → Exigir(policy)( Trace( handler ) ) ) ) )
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
