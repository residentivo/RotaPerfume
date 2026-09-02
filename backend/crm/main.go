package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"backend/crm/internal/config"
	"backend/crm/internal/handlers"
	"backend/crm/internal/middleware"
	mysqlrepo "backend/crm/internal/repository/mysql"
	"backend/crm/internal/services"
)

func main() {
	// Carrega configuração
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Erro ao carregar configuração: %v", err)
	}

	// Conecta ao banco de dados
	db, err := mysqlrepo.NewDB(cfg.DBURL)
	if err != nil {
		log.Fatalf("Erro ao conectar ao banco: %v", err)
	}
	defer db.Close()

	// Inicializa repositories
	clienteRepo := mysqlrepo.NewClienteRepository(db)
	vendedorRepo := mysqlrepo.NewVendedorRepository(db)
	carteiraRepo := mysqlrepo.NewCarteiraRepository(db)
	visitaRepo := mysqlrepo.NewVisitaRepository(db)
	oportunidadeRepo := mysqlrepo.NewOportunidadeRepository(db)
	usuarioRepo := mysqlrepo.NewUsuarioRepository(db)
	dashboardRepo := mysqlrepo.NewDashboardRepository(db)

	// Inicializa services
	clienteService := services.NewClienteService(clienteRepo)
	vendedorService := services.NewVendedorService(vendedorRepo)
	carteiraService := services.NewCarteiraService(carteiraRepo)
	visitaService := services.NewVisitaService(visitaRepo)
	oportunidadeService := services.NewOportunidadeService(oportunidadeRepo)
	authService := services.NewAuthService(usuarioRepo)
	dashboardService := services.NewDashboardService(dashboardRepo)

	// Inicializa handlers
	h := handlers.NewHandler(
		clienteService,
		vendedorService,
		carteiraService,
		visitaService,
		oportunidadeService,
		dashboardService,
	)
	authHandler := handlers.NewAuthHandler(authService, cfg.JWTSecret)

	// Configura o router
	r := mux.NewRouter()

	// Health check
	r.HandleFunc("/health", h.HealthCheck).Methods("GET")

	// Rota pública: login
	r.HandleFunc("/auth/login", authHandler.Login).Methods("POST")

	// Middleware JWT para rotas protegidas
	api := r.PathPrefix("/api").Subrouter()
	api.Use(middleware.JWTMiddleware(cfg.JWTSecret))

	// Rotas de clientes
	api.HandleFunc("/clientes", h.ListarClientes).Methods("GET")
	api.HandleFunc("/clientes/{id}", h.BuscarCliente).Methods("GET")
	api.HandleFunc("/clientes", h.CriarCliente).Methods("POST")
	api.HandleFunc("/clientes/{id}", h.AtualizarCliente).Methods("PUT")

	// Rotas de vendedores
	api.HandleFunc("/vendedores", h.ListarVendedores).Methods("GET")
	api.HandleFunc("/vendedores/{id}", h.BuscarVendedor).Methods("GET")

	// Rotas de carteira
	api.HandleFunc("/carteira", h.ListarCarteira).Methods("GET")
	api.HandleFunc("/carteira", h.CriarCarteira).Methods("POST")

	// Rotas de visitas
	api.HandleFunc("/visitas", h.ListarVisitas).Methods("GET")
	api.HandleFunc("/visitas", h.CriarVisita).Methods("POST")

	// Rotas de oportunidades
	api.HandleFunc("/oportunidades", h.ListarOportunidades).Methods("GET")
	api.HandleFunc("/oportunidades/{id}", h.BuscarOportunidade).Methods("GET")
	api.HandleFunc("/oportunidades", h.CriarOportunidade).Methods("POST")
	api.HandleFunc("/oportunidades/{id}", h.AtualizarOportunidade).Methods("PUT")

	// Rotas de dashboard
	api.HandleFunc("/dashboard/vendas", h.RankingVendas).Methods("GET")
	api.HandleFunc("/dashboard/carteira", h.DistribuicaoCarteira).Methods("GET")

	// CORS middleware - aplicado como wrapper externo
	// Garante que TODAS as requisições (incluindo OPTIONS preflight) passem por aqui
	handler := corsMiddleware(r)

	// Inicia o servidor
	addr := ":" + cfg.Port
	fmt.Printf("CRM API rodando na porta %s\n", cfg.Port)
	fmt.Printf("Health check: http://localhost%s/health\n", addr)
	fmt.Printf("Login: POST http://localhost%s/auth/login\n", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

// origens permitidas em desenvolvimento
var allowedOrigins = map[string]bool{
	"http://localhost:3000":  true, // CRM
	"http://localhost:3001":  true, // ERP
	"http://localhost:3002":  true, // RH
	"http://localhost:8081":  true, // API direta
	"http://localhost:8082":  true, // API direta
	"http://127.0.0.1:3000":  true,
	"http://127.0.0.1:3001":  true,
	"http://127.0.0.1:3002":  true,
	"http://127.0.0.1:8081":  true,
	"http://127.0.0.1:8082":  true,
}

// corsMiddleware adiciona headers CORS às respostas
// É um wrapper que intercepta TODAS as requisições antes de chegar ao router
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Log para debug
		log.Printf("[CORS] %s %s - Origin: %s", r.Method, r.URL.Path, origin)

		// Permite a origem se estiver na lista, senão usa "*" para dev
		if origin != "" && allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else if origin != "" {
			// Origem não está na lista, mas responde com a origem mesmo assim (dev)
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "3600")
		w.Header().Set("Vary", "Origin, Access-Control-Request-Headers, Access-Control-Request-Method")

		// Responde a preflight OPTIONS direto
		if r.Method == "OPTIONS" {
			log.Printf("[CORS] Preflight OPTIONS respondido para %s", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
