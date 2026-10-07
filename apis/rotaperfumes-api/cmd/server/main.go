// Command server inicia a API rotaperfumes-api na porta 8080.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
	sharedsvc "github.com/rotaperfumes/shared/services"
	"github.com/rotaperfumes/shared/vlog"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/rotaperfumes-api/routes"
	"github.com/rotaperfumes/rotaperfumes-api/services"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("[server] iniciando rotaperfumes-api")

	// Carrega .env da raiz do projeto. Tenta o diretório de execução e
	// alguns caminhos-pai para funcionar tanto em `go run` quanto em binário compilado.
	vlog.Printf("main.go", "main", "chamando loadEnvFromCwd")
	loadEnvFromCwd()

	vlog.Printf("main.go", "main", "chamando config.Load e declarando cfg, err")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[server] config: %v", err)
	}
	vlog.SetEnabled(cfg.Verbose)
	if vlog.Enabled() {
		log.Printf("[server] WARN: VERBOSE ativo — logs de rastreamento linha a linha ligados; não use em produção")
	}
	log.Printf("[server] config OK: db=%s@%s jwt_issuer=%s verbose=%v",
		cfg.DBName, cfg.DBHost, cfg.JWTIssuer, cfg.Verbose)

	// Pool de conexão único para toda a aplicação.
	vlog.Printf("main.go", "main", "chamando db.Open e declarando conn, err")
	conn, err := db.Open(cfg.DSN())
	if err != nil {
		log.Fatalf("[server] db: %v", err)
	}
	vlog.Printf("main.go", "main", "chamando conn.Ping e declarando err e verificando condição err != nil")
	if err := conn.Ping(); err != nil {
		log.Fatalf("[server] db ping: %v", err)
	}
	log.Printf("[server] db ping OK")

	// CHORE-02: limpeza periódica de refresh_tokens (roda já no início e a
	// cada REFRESH_CLEANUP_INTERVAL; "0" desativa). Encerrada no shutdown,
	// antes de fechar o pool.
	vlog.Printf("main.go", "main", "chamando context.WithCancel e declarando bgCtx, bgCancel")
	bgCtx, bgCancel := context.WithCancel(context.Background())
	vlog.Printf("main.go", "main", "agendando defer: bgCancel")
	defer bgCancel()
	vlog.Printf("main.go", "main", "chamando services.NewRefreshTokenService e declarando limpezaSvc")
	limpezaSvc := services.NewRefreshTokenService()
	vlog.Printf("main.go", "main", "chamando limpezaSvc.SetRetencao")
	limpezaSvc.SetRetencao(cfg.RefreshTokenRetencao)
	vlog.Printf("main.go", "main", "chamando services.IniciarLimpezaRefreshTokens e declarando limpezaDone")
	limpezaDone := services.IniciarLimpezaRefreshTokens(bgCtx, conn, limpezaSvc, cfg.RefreshCleanupInterval)

	// EmailService: usa SMTP real se as credenciais estiverem configuradas,
	// caso contrário cai no fallback noop (log-only) — permite `make dev-api`
	// funcionar sem SMTP configurado.
	vlog.Printf("main.go", "main", "chamando newEmailService e declarando emailSvc, alertaSender")
	emailSvc, alertaSender := newEmailService(cfg)

	// Handler + rotas (injetam o pool de conexão).
	vlog.Printf("main.go", "main", "chamando handlers.NewAuthHandler e declarando authHandler")
	authHandler := handlers.NewAuthHandler(conn, cfg)
	// SEC-09: alerta por e-mail (usuário + SECURITY_ALERT_EMAILS) no reuso de
	// refresh token já rotacionado.
	vlog.Printf("main.go", "main", "chamando authHandler.SetAlertaSeguranca")
	authHandler.SetAlertaSeguranca(services.NewAlertaSegurancaNotifier(conn, alertaSender, cfg.SecurityAlertEmails))
	log.Printf("[server] alertas de segurança: admins=%d", len(cfg.SecurityAlertEmails))
	vlog.Printf("main.go", "main", "chamando handlers.NewUsuarioHandler e declarando userHandler")
	userHandler := handlers.NewUsuarioHandler(conn, cfg, emailSvc)
	vlog.Printf("main.go", "main", "chamando handlers.NewDashboardHandler e declarando dashboardHandler")
	dashboardHandler := handlers.NewDashboardHandler(conn, cfg)
	vlog.Printf("main.go", "main", "chamando handlers.NewSenhaHistoricoHandler e declarando senhaHandler")
	senhaHandler := handlers.NewSenhaHistoricoHandler(conn)
	vlog.Printf("main.go", "main", "chamando handlers.NewVendedorHandler e declarando vendedorHandler")
	vendedorHandler := handlers.NewVendedorHandler(conn, cfg)
	vlog.Printf("main.go", "main", "chamando handlers.NewClienteHandler e declarando clienteHandler")
	clienteHandler := handlers.NewClienteHandler(conn, cfg)
	vlog.Printf("main.go", "main", "chamando handlers.NewProdutoHandler e declarando produtoHandler")
	produtoHandler := handlers.NewProdutoHandler(conn, cfg)
	vlog.Printf("main.go", "main", "chamando handlers.NewPedidoHandler e declarando pedidoHandler")
	pedidoHandler := handlers.NewPedidoHandler(conn, cfg)
	vlog.Printf("main.go", "main", "chamando handlers.NewPagamentoHandler e declarando pagamentoHandler")
	pagamentoHandler := handlers.NewPagamentoHandler(conn, cfg)
	vlog.Printf("main.go", "main", "chamando handlers.NewOportunidadeHandler e declarando oportunidadeHandler")
	oportunidadeHandler := handlers.NewOportunidadeHandler(conn, cfg)
	vlog.Printf("main.go", "main", "chamando handlers.NewVisitaHandler e declarando visitaHandler")
	visitaHandler := handlers.NewVisitaHandler(conn, cfg)
	vlog.Printf("main.go", "main", "chamando handlers.NewEstoqueHandler e declarando estoqueHandler")
	estoqueHandler := handlers.NewEstoqueHandler(conn, cfg)
	// SEC-06: rotas protegidas conferem ativo/role do usuário no banco a cada request.
	vlog.Printf("main.go", "main", "chamando middleware.NewDBUserStatusChecker e declarando userChecker")
	userChecker := middleware.NewDBUserStatusChecker(conn)
	vlog.Printf("main.go", "main", "chamando routes.NewMux e declarando mux")
	mux := routes.NewMux(cfg, userChecker, authHandler, userHandler, dashboardHandler, senhaHandler, vendedorHandler, clienteHandler, produtoHandler, pedidoHandler, pagamentoHandler, oportunidadeHandler, visitaHandler, estoqueHandler)

	vlog.Printf("main.go", "main", "montando &http.Server e declarando srv")
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      withLogging(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	vlog.Printf("main.go", "main", "disparando goroutine: função anônima")
	go func() {
		log.Printf("[server] escutando em :8080")
		vlog.Printf("main.go", "main.func", "chamando srv.ListenAndServe e declarando err e verificando condição err != nil && err != http.ErrServerClosed")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[server] ListenAndServe: %v", err)
		}
	}()

	vlog.Printf("main.go", "main", "chamando make e declarando stop")
	stop := make(chan os.Signal, 1)
	vlog.Printf("main.go", "main", "chamando signal.Notify")
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	vlog.Printf("main.go", "main", "<-stop")
	<-stop
	log.Printf("[server] shutdown solicitado")

	vlog.Printf("main.go", "main", "chamando context.WithTimeout e declarando ctx, cancel")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	vlog.Printf("main.go", "main", "agendando defer: cancel")
	defer cancel()
	vlog.Printf("main.go", "main", "chamando srv.Shutdown e declarando err e verificando condição err != nil")
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[server] shutdown erro: %v", err)
	}
	vlog.Printf("main.go", "main", "chamando pararTarefasDeFundo")
	pararTarefasDeFundo(bgCancel, limpezaDone, 10*time.Second)
	vlog.Printf("main.go", "main", "chamando conn.Close e atribuindo a _")
	_ = conn.Close()
	log.Printf("[server] bye")
}

// pararTarefasDeFundo cancela as tarefas em segundo plano (CHORE-02) e espera
// o término por até timeout, para não fechar o pool no meio de um DELETE.
func pararTarefasDeFundo(cancel context.CancelFunc, done <-chan struct{}, timeout time.Duration) {
	vlog.Printf("main.go", "pararTarefasDeFundo", "chamando cancel")
	cancel()
	vlog.Printf("main.go", "pararTarefasDeFundo", "aguardando select entre canais")
	select {
	case <-done:
		log.Printf("[server] tarefas de fundo encerradas")
	case <-time.After(timeout):
		log.Printf("[server] tarefas de fundo não encerraram em %s — seguindo com o shutdown", timeout)
	}
}

// newEmailService escolhe a implementação de EmailService com base na config:
// SMTP real quando SMTP_USER/SMTP_PASSWORD/SMTP_FROM estão presentes, ou o
// fallback noop (log-only) caso contrário — assim `make dev-api` funciona
// mesmo sem SMTP configurado.
//
// Devolve também o AlertaSegurancaSender (SEC-09) — a mesma instância
// implementa as duas interfaces.
func newEmailService(cfg *config.Config) (sharedsvc.EmailService, sharedsvc.AlertaSegurancaSender) {
	vlog.Printf("main.go", "newEmailService", "chamando sharedsvc.NewSMTPEmailService e declarando svc, err")
	svc, err := sharedsvc.NewSMTPEmailService(cfg)
	vlog.Printf("main.go", "newEmailService", "verificando condição err != nil")
	if err != nil {
		log.Printf("[server] SMTP não configurado — emails de senha inicial/reset serão apenas logados (%v)", err)
		log.Printf("[server] SMTP não configurado — alertas de segurança só no log")
		vlog.Printf("main.go", "newEmailService", "chamando sharedsvc.NewNoopEmailService e declarando noop")
		noop := sharedsvc.NewNoopEmailService()
		return noop, noop
	}
	log.Printf("[server] EmailService SMTP configurado: host=%s port=%s from=%s", cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)
	return svc, svc
}

// withLogging envolve o mux com log mínimo de cada request.
func withLogging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vlog.Printf("main.go", "withLogging.func", "chamando time.Now e declarando start")
		start := time.Now()
		vlog.Printf("main.go", "withLogging.func", "montando &statusRecorder e declarando ww")
		ww := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		vlog.Printf("main.go", "withLogging.func", "chamando h.ServeHTTP")
		h.ServeHTTP(ww, r)
		log.Printf("[http] %s %s -> %d (%s)", r.Method, r.URL.Path, ww.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	vlog.Printf("main.go", "statusRecorder.WriteHeader", "atribuindo code a s.status")
	s.status = code
	vlog.Printf("main.go", "statusRecorder.WriteHeader", "chamando s.ResponseWriter.WriteHeader")
	s.ResponseWriter.WriteHeader(code)
}

// loadEnvFromCwd tenta carregar o .env da raiz do projeto subindo diretórios
// a partir do working directory. Funciona tanto em `go run` (cwd = raiz) quanto
// em binário compilado. Não retorna erro — se não achar, segue sem .env.
func loadEnvFromCwd() {
	vlog.Printf("main.go", "loadEnvFromCwd", "chamando os.Getwd e declarando wd, err e verificando condição err == nil")
	if wd, err := os.Getwd(); err == nil {
		vlog.Printf("main.go", "loadEnvFromCwd", "declarando dir com wd")
		dir := wd
		vlog.Printf("main.go", "loadEnvFromCwd", "iniciando loop enquanto i < 6")
		for i := 0; i < 6; i++ { // até 6 níveis acima
			if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
				_ = godotenv.Load(filepath.Join(dir, ".env"))
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
		vlog.Printf("main.go", "loadEnvFromCwd", "loop concluído (enquanto i < 6)")
	}
}
