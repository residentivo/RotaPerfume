package httpx

import (
	"log"
	"net/http"
	"time"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

// Logging é o log de acesso sempre ligado (ex-withLogging do main.go):
// "[http] MÉTODO caminho -> status (duração)". Usa r.URL.Path: a query
// string nunca vai ao log (F-09).
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		rec := novoStatusRecorder(w)
		next.ServeHTTP(rec, r)
		// ARQ-07 NB-C: path decodificado saneado (sem quebra de linha no log).
		log.Printf("[http] %s %s -> %d (%s)", r.Method, kernel.SanearRota(r.URL.Path), rec.status, time.Since(inicio))
	})
}

// Trace é o log de nível 1 (rota), aplicado pelo Router a todo handler:
// "[arquivo] [Funcao] início MÉTODO" e "fim status=N". Só método e status:
// nada de path, query, headers, cookies ou corpo (O7). Fica por DENTRO da
// autenticação (O9): 401/403 não emitem o rótulo do handler.
func Trace(arquivo, funcao string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !vlog.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		rec := novoStatusRecorder(w)
		vlog.Printf(arquivo, funcao, "início %s", r.Method)
		next.ServeHTTP(rec, r)
		vlog.Printf(arquivo, funcao, "fim status=%d", rec.status)
	})
}

// statusRecorder guarda o status escrito (200 se o handler só escrever corpo).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func novoStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap permite ao http.ResponseController alcançar o writer original.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
