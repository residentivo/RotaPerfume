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

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/routes"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("[server] iniciando rotaperfumes-api")

	// Carrega .env da raiz do projeto. Tenta o diretório de execução e
	// alguns caminhos-pai para funcionar tanto em `go run` quanto em binário compilado.
	loadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[server] config: %v", err)
	}
	log.Printf("[server] config OK: db=%s@%s jwt_issuer=%s verbose=%v",
		cfg.DBName, cfg.DBHost, cfg.JWTIssuer, cfg.Verbose)

	// Pool de conexão único para toda a aplicação.
	conn, err := db.Open(cfg.DSN())
	if err != nil {
		log.Fatalf("[server] db: %v", err)
	}
	if err := conn.Ping(); err != nil {
		log.Fatalf("[server] db ping: %v", err)
	}
	log.Printf("[server] db ping OK")

	// EmailService: usa SMTP real se as credenciais estiverem configuradas,
	// caso contrário cai no fallback noop (log-only) — permite `make dev-api`
	// funcionar sem SMTP configurado.
	emailSvc := newEmailService(cfg)

	// Handler + rotas (injetam o pool de conexão).
	authHandler := handlers.NewAuthHandler(conn, cfg)
	userHandler := handlers.NewUsuarioHandler(conn, cfg, emailSvc)
	dashboardHandler := handlers.NewDashboardHandler(conn, cfg)
	senhaHandler := handlers.NewSenhaHistoricoHandler(conn)
	vendedorHandler := handlers.NewVendedorHandler(conn, cfg)
	clienteHandler := handlers.NewClienteHandler(conn, cfg)
	produtoHandler := handlers.NewProdutoHandler(conn, cfg)
	pedidoHandler := handlers.NewPedidoHandler(conn, cfg)
	pagamentoHandler := handlers.NewPagamentoHandler(conn, cfg)
	oportunidadeHandler := handlers.NewOportunidadeHandler(conn, cfg)
	visitaHandler := handlers.NewVisitaHandler(conn, cfg)
	estoqueHandler := handlers.NewEstoqueHandler(conn, cfg)
	mux := routes.NewMux(cfg, authHandler, userHandler, dashboardHandler, senhaHandler, vendedorHandler, clienteHandler, produtoHandler, pedidoHandler, pagamentoHandler, oportunidadeHandler, visitaHandler, estoqueHandler)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      withLogging(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	go func() {
		log.Printf("[server] escutando em :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[server] ListenAndServe: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Printf("[server] shutdown solicitado")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[server] shutdown erro: %v", err)
	}
	_ = conn.Close()
	log.Printf("[server] bye")
}

// newEmailService escolhe a implementação de EmailService com base na config:
// SMTP real quando SMTP_USER/SMTP_PASSWORD/SMTP_FROM estão presentes, ou o
// fallback noop (log-only) caso contrário — assim `make dev-api` funciona
// mesmo sem SMTP configurado.
func newEmailService(cfg *config.Config) sharedsvc.EmailService {
	svc, err := sharedsvc.NewSMTPEmailService(cfg)
	if err != nil {
		log.Printf("[server] SMTP não configurado — emails de senha inicial/reset serão apenas logados (%v)", err)
		return sharedsvc.NewNoopEmailService()
	}
	log.Printf("[server] EmailService SMTP configurado: host=%s port=%s from=%s", cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)
	return svc
}

// withLogging envolve o mux com log mínimo de cada request.
func withLogging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(ww, r)
		log.Printf("[http] %s %s -> %d (%s)", r.Method, r.URL.Path, ww.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// loadEnvFromCwd tenta carregar o .env da raiz do projeto subindo diretórios
// a partir do working directory. Funciona tanto em `go run` (cwd = raiz) quanto
// em binário compilado. Não retorna erro — se não achar, segue sem .env.
func loadEnvFromCwd() {
	if wd, err := os.Getwd(); err == nil {
		dir := wd
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
	}
}
