package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
)

// Timeouts do servidor e do shutdown (F-04, valores inalterados).
const (
	enderecoHTTP       = ":8080"
	timeoutLeitura     = 15 * time.Second
	timeoutEscrita     = 15 * time.Second
	timeoutOcioso      = 60 * time.Second
	timeoutShutdown    = 10 * time.Second
	timeoutTarefasFund = 10 * time.Second
)

// App é a API montada: o handler HTTP completo (com o log de acesso).
type App struct {
	Handler http.Handler
}

// Montar liga as dependências sobre o pool: e-mail (SMTP ou noop), notifier
// de alerta (SEC-09), handlers, checker de usuário (SEC-06) e a cadeia
// Logging → CORS → SecurityHeaders → mux. Singletons por processo.
func Montar(cfg *config.Config, pool *sql.DB) *App {
	emailSvc, alertaSender := novoEmail(cfg)

	hs := NovosHandlers(pool, cfg, emailSvc)
	hs.Auth.SetAlertaSeguranca(services.NewAlertaSegurancaNotifier(pool, alertaSender, cfg.SecurityAlertEmails))
	log.Printf("[server] alertas de segurança: admins=%d", len(cfg.SecurityAlertEmails))

	checker := middleware.NewDBUserStatusChecker(pool)
	return &App{Handler: httpx.Chain(HandlerHTTP(cfg, checker, hs), httpx.Logging)}
}

// Run abre o banco, sobe a limpeza de refresh tokens (CHORE-02) e o servidor,
// e no SIGINT/SIGTERM faz o shutdown na ordem F-04/I-10: servidor (10 s) →
// tarefas de fundo (até 10 s) → pool.
func Run(cfg *config.Config) {
	pool := abrirBanco(cfg)

	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	limpezaDone := iniciarLimpezaRefresh(bgCtx, pool, cfg)

	srv := &http.Server{
		Addr:         enderecoHTTP,
		Handler:      Montar(cfg, pool).Handler,
		ReadTimeout:  timeoutLeitura,
		WriteTimeout: timeoutEscrita,
		IdleTimeout:  timeoutOcioso,
	}
	go func() {
		log.Printf("[server] escutando em %s", enderecoHTTP)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[server] ListenAndServe: %v", err)
		}
	}()

	esperarSinalDeParada()
	log.Printf("[server] shutdown solicitado")

	ctx, cancel := context.WithTimeout(context.Background(), timeoutShutdown)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[server] shutdown erro: %v", err)
	}
	pararTarefasDeFundo(bgCancel, limpezaDone, timeoutTarefasFund)
	_ = pool.Close()
	log.Printf("[server] bye")
}

// abrirBanco abre o pool único (db.Open já faz ping de 5 s e loga o DSN
// mascarado, F-08) e confere a conexão.
func abrirBanco(cfg *config.Config) *sql.DB {
	pool, err := db.Open(cfg.DSN())
	if err != nil {
		log.Fatalf("[server] db: %v", err)
	}
	if err := pool.Ping(); err != nil {
		log.Fatalf("[server] db ping: %v", err)
	}
	log.Printf("[server] db ping OK")
	return pool
}

func esperarSinalDeParada() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}
