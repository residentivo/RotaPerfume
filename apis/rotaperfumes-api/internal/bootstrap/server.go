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
	"github.com/rotaperfumes/shared/adapters/turnstile"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/notificacao/notificacaoapp"
)

// Timeouts do servidor e do shutdown (F-04); ReadHeaderTimeout e
// MaxHeaderBytes vieram com o SEC-14.
const (
	enderecoHTTP       = ":8080"
	timeoutCabecalho   = 5 * time.Second
	maxBytesCabecalho  = 64 << 10
	timeoutLeitura     = 15 * time.Second
	timeoutEscrita     = 15 * time.Second
	timeoutOcioso      = 60 * time.Second
	timeoutShutdown    = 10 * time.Second
	timeoutTarefasFund = 10 * time.Second
	timeoutAlertas     = 10 * time.Second
)

// App é a API montada: o handler HTTP completo (com o log de acesso), os
// casos de uso de Identidade (a limpeza de refresh tokens roda sobre eles) e
// o notificador de alertas (o shutdown espera os envios em andamento).
type App struct {
	Handler     http.Handler
	Identidade  *identidadeapp.Casos
	Notificador *notificacaoapp.AlertaSegurancaNotifier
}

// Montar liga as dependências sobre o pool: e-mail (SMTP ou Noop), notifier
// de alerta (SEC-09), handlers, checker de usuário (SEC-06) e a cadeia
// Logging → CORS → SecurityHeaders → mux. Singletons por processo.
func Montar(cfg *config.Config, pool *sql.DB) *App {
	email := NovoEmail(cfg)
	notifier := NovoNotificadorAlerta(pool, email, cfg.SecurityAlertEmails)
	log.Printf("[server] alertas de segurança: admins=%d", len(cfg.SecurityAlertEmails))

	identidade := NovaIdentidade(pool, cfg, ExtIdentidade{
		Captcha:    turnstile.Novo(cfg.TurnstileSecretKey),
		Publicador: identidadeapp.NovoPublicadorLocal(assinanteAlerta(notifier)),
		Email:      email,
	})
	hs := NovosHandlersCom(pool, cfg, identidade)
	// SEC-06: o checker é o próprio caso de uso (consulta o banco a cada request).
	return &App{Handler: httpx.Chain(HandlerHTTP(cfg, identidade, hs), httpx.Logging), Identidade: identidade, Notificador: notifier}
}

// Run abre o banco, sobe a limpeza de refresh tokens (CHORE-02) e o servidor,
// e no SIGINT/SIGTERM faz o shutdown na ordem F-04/I-10: servidor (10 s) →
// tarefas de fundo (até 10 s) → alertas de segurança em andamento (até
// 10 s, NB-6) → pool.
func Run(cfg *config.Config) {
	pool := abrirBanco(cfg)

	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	app := Montar(cfg, pool)
	limpezaDone := identidadeapp.IniciarLimpezaPeriodica(bgCtx, app.Identidade, cfg.RefreshCleanupInterval)

	srv := NovoServidor(app.Handler)
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
	aguardarAlertas(app.Notificador, timeoutAlertas)
	_ = pool.Close()
	log.Printf("[server] bye")
}

// NovoServidor monta o http.Server da API com os timeouts F-04 e, do SEC-14,
// ReadHeaderTimeout de 5 s (slowloris) e cabeçalhos de no máximo 64 KiB.
func NovoServidor(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              enderecoHTTP,
		Handler:           handler,
		ReadHeaderTimeout: timeoutCabecalho,
		ReadTimeout:       timeoutLeitura,
		WriteTimeout:      timeoutEscrita,
		IdleTimeout:       timeoutOcioso,
		MaxHeaderBytes:    maxBytesCabecalho,
	}
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
