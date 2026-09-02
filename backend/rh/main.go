package main

import (
	"fmt"
	"log"
	"net/http"

	"backend/rh/internal/config"
	"backend/rh/internal/handlers"
	"backend/rh/internal/middleware"
	mysqlrepo "backend/rh/internal/repository/mysql"
	"backend/rh/internal/services"

	"github.com/gorilla/mux"
)

func main() {
	// Carrega configurações
	cfg := config.Load()
	log.Printf("Iniciando servidor na porta %s", cfg.Port)

	// Inicializa conexão com o banco
	db, err := mysqlrepo.NewConnection(cfg.DBURL)
	if err != nil {
		log.Fatalf("Erro ao conectar ao banco de dados: %v", err)
	}
	defer db.Close()
	log.Println("Conectado ao banco de dados MySQL")

	// Inicializa repositórios
	usuarioRepo := mysqlrepo.NewUsuarioRepository(db)
	vendedorRepo := mysqlrepo.NewVendedorRepository(db)

	// Inicializa serviços
	usuarioService := services.NewUsuarioService(usuarioRepo, cfg.JWTSecret)
	vendedorService := services.NewVendedorService(vendedorRepo)

	// Inicializa handlers
	authHandler := handlers.NewAuthHandler(usuarioService)
	vendedorHandler := handlers.NewVendedorHandler(vendedorService)
	usuarioHandler := handlers.NewUsuarioHandler(usuarioService)

	// Cria router
	r := mux.NewRouter()

	// Middleware global: log de requisições
	r.Use(loggingMiddleware)

	// Rotas públicas
	r.HandleFunc("/health", handlers.HealthHandler).Methods("GET")
	r.HandleFunc("/auth/login", authHandler.Login).Methods("POST")

	// Rotas autenticadas (com middleware JWT)
	auth := middleware.AuthMiddleware(usuarioService)
	api := r.PathPrefix("").Subrouter()
	api.Use(auth)

	// Troca de senha (qualquer usuário autenticado)
	api.HandleFunc("/auth/trocar-senha", authHandler.TrocarSenha).Methods("POST")

	// Vendedores
	api.HandleFunc("/vendedores", vendedorHandler.Listar).Methods("GET")
	api.HandleFunc("/vendedores/{id:[0-9]+}", vendedorHandler.BuscarPorID).Methods("GET")

	// Vendedores (criação/atualização requer rh ou gerente)
	rhOrGerente := middleware.RequireRole(usuarioService, "rh", "gerente")
	api.Handle("/vendedores", rhOrGerente(http.HandlerFunc(vendedorHandler.Criar))).Methods("POST")
	api.Handle("/vendedores/{id:[0-9]+}", rhOrGerente(http.HandlerFunc(vendedorHandler.Atualizar))).Methods("PUT")

	// Usuários (somente rh pode criar/atualizar/resetar)
	rhOnly := middleware.RequireRole(usuarioService, "rh")
	api.HandleFunc("/usuarios", usuarioHandler.Listar).Methods("GET")
	api.HandleFunc("/usuarios/{id:[0-9]+}", usuarioHandler.BuscarPorID).Methods("GET")
	api.Handle("/usuarios", rhOnly(http.HandlerFunc(usuarioHandler.Criar))).Methods("POST")
	api.Handle("/usuarios/{id:[0-9]+}", rhOnly(http.HandlerFunc(usuarioHandler.Atualizar))).Methods("PUT")
	api.Handle("/usuarios/{id:[0-9]+}/reset-senha", rhOnly(http.HandlerFunc(usuarioHandler.ResetSenha))).Methods("POST")
	api.HandleFunc("/usuarios/{id:[0-9]+}/vendedor", usuarioHandler.BuscarVendedor).Methods("GET")

	// CORS middleware - aplicado como wrapper externo
	// Garante que TODAS as requisições (incluindo OPTIONS preflight) passem por aqui
	handler := corsMiddleware(r)

	// Inicia servidor
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Servidor rodando em http://localhost%s", addr)
	log.Printf("Endpoints disponíveis:")
	log.Printf("  POST   /auth/login")
	log.Printf("  POST   /auth/trocar-senha (autenticado)")
	log.Printf("  GET    /api/vendedores (autenticado)")
	log.Printf("  GET    /api/vendedores/{id} (autenticado)")
	log.Printf("  POST   /api/vendedores (autenticado, rh/gerente)")
	log.Printf("  PUT    /api/vendedores/{id} (autenticado, rh/gerente)")
	log.Printf("  GET    /api/usuarios (autenticado)")
	log.Printf("  POST   /api/usuarios (autenticado, rh)")
	log.Printf("  PUT    /api/usuarios/{id} (autenticado, rh)")
	log.Printf("  POST   /api/usuarios/{id}/reset-senha (autenticado, rh)")
	log.Printf("  GET    /api/usuarios/{id}/vendedor (autenticado)")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
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

// loggingMiddleware loga todas as requisições HTTP
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}
