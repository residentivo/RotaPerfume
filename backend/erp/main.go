package main

import (
	"log"
	"net/http"

	"backend/erp/internal/config"
	"backend/erp/internal/handlers"
	"backend/erp/internal/middleware"

	"github.com/gorilla/mux"
)

func main() {
	// Carregar configuracoes
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Erro ao carregar configuracoes: %v", err)
	}

	// Conectar ao banco de dados
	if err := config.ConnectDB(); err != nil {
		log.Fatalf("Erro ao conectar ao banco de dados: %v", err)
	}
	defer config.CloseDB()


	// Criar router
	router := mux.NewRouter()

	// Instanciar handlers
	authHandler := handlers.NewAuthHandler()
	produtoHandler := handlers.NewProdutoHandler()
	pedidoHandler := handlers.NewPedidoHandler()
	itemPedidoHandler := handlers.NewItemPedidoHandler()
	pagamentoHandler := handlers.NewPagamentoHandler()
	estoqueHandler := handlers.NewEstoqueHandler()
	dashboardHandler := handlers.NewDashboardHandler()

	// Rotas publicas
	router.HandleFunc("/auth/login", authHandler.Login).Methods("POST")

	// Rotas protegidas
	api := router.PathPrefix("/api").Subrouter()
	api.Use(middleware.AuthMiddleware)

	// Produtos
	api.HandleFunc("/produtos", produtoHandler.ListarTodos).Methods("GET")
	api.HandleFunc("/produtos/{sku}", produtoHandler.BuscarPorSKU).Methods("GET")
	api.HandleFunc("/produtos", produtoHandler.Criar).Methods("POST")
	api.HandleFunc("/produtos/{sku}", produtoHandler.Atualizar).Methods("PUT")

	// Pedidos
	api.HandleFunc("/pedidos", pedidoHandler.ListarTodos).Methods("GET")
	api.HandleFunc("/pedidos/{id}", pedidoHandler.BuscarPorID).Methods("GET")
	api.HandleFunc("/pedidos", pedidoHandler.Criar).Methods("POST")
	api.HandleFunc("/pedidos/{id}/itens", itemPedidoHandler.ListarPorPedido).Methods("GET")

	// Pagamentos
	api.HandleFunc("/pagamentos", pagamentoHandler.ListarTodos).Methods("GET")
	api.HandleFunc("/pagamentos", pagamentoHandler.Criar).Methods("POST")
	api.HandleFunc("/pagamentos/{id}", pagamentoHandler.Atualizar).Methods("PUT")

	// Estoque
	api.HandleFunc("/estoque", estoqueHandler.ListarPorSKU).Methods("GET")

	// Dashboard
	api.HandleFunc("/dashboard/financeiro", dashboardHandler.ObterTotaisFinanceiro).Methods("GET")
	api.HandleFunc("/dashboard/estoque", dashboardHandler.ObterRupturas).Methods("GET")

	// CORS middleware - aplicado como wrapper externo
	// Garante que TODAS as requisições (incluindo OPTIONS preflight) passem por aqui
	handler := corsMiddleware(router)

	// Iniciar servidor
	addr := ":" + config.AppConfig.Port
	log.Printf("Servidor ERP iniciando na porta %s", config.AppConfig.Port)
	log.Printf("URL: http://localhost:%s", config.AppConfig.Port)

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
