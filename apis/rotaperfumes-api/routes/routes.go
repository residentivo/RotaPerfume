// Package routes registra as rotas HTTP da API.
package routes

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/shared/config"
)

// NewMux monta o *http.ServeMux com todas as rotas e middlewares aplicados.
//
// Rotas:
//
//	POST /api/auth/login                — público
//	POST /api/auth/refresh              — público (usa refresh_token no body)
//	POST /api/auth/logout               — público (revoga refresh_token)
//	POST /api/auth/reset-password       — protegido (JWT, qualquer role autenticado)
//	GET  /api/auth/me                   — protegido (JWT)
//	GET  /api/usuarios                  — admin only (JWT + role=admin) — listagem paginada
//	POST /api/usuarios                  — admin only — criar usuário
//	PUT  /api/usuarios/{id}             — admin only — atualizar usuário
//	PATCH /api/usuarios/{id}/inativar   — admin only — ativar/inativar
//	POST /api/admin/reset-password      — admin only — resetar senha de outro usuário
//	GET  /api/vendedores                — admin only — lista vendedores ativos (sem paginação)
//	GET  /api/senha-historico           — admin only — todo histórico de senhas (paginado)
//	GET  /api/senha-historico/{user_id} — admin only — histórico de um usuário (paginado)
//	GET  /api/dashboard/metrics         — admin only — métricas gerais (vendas, pedidos, ticket medio)
//	GET  /api/dashboard/vendas          — admin only — serie temporal de vendas (ultimos N dias)
//	GET  /api/dashboard/vendedores      — admin only — ranking de vendedores com meta
//	GET  /api/dashboard/clientes        — admin only — métricas da base de clientes (totais, novos, por segmento, por uf)
//	GET  /api/clientes                  — admin only — lista clientes (paginado, filtros uf/segmento/ativo/q)
//	POST /api/clientes                  — admin only — criar cliente
//	GET  /api/clientes/{id}             — admin only — detalhe de um cliente
//	PUT  /api/clientes/{id}             — admin only — atualizar cliente
//	PATCH /api/clientes/{id}/inativar   — admin only — ativar/inativar cliente
//	GET  /api/produtos                  — admin only — lista produtos (paginado, filtros categoria/marca/ativo/q)
//	POST /api/produtos                  — admin only — criar produto
//	GET  /api/produtos/{id}             — admin only — detalhe de um produto
//	PUT  /api/produtos/{id}             — admin only — atualizar produto
//	PATCH /api/produtos/{id}/inativar   — admin only — ativar/inativar produto
//	GET  /api/pedidos                   — admin only — lista pedidos (paginado, filtros status/canal/cliente_id/vendedor_id/data_inicio/data_fim/q)
//	POST /api/pedidos                   — admin only — cria pedido com itens (calcula valor_bruto/valor_total)
//	GET  /api/pedidos/{id}               — admin only — detalhe de um pedido (com itens)
//	PUT  /api/pedidos/{id}               — admin only — atualiza pedido e substitui a lista de itens
//	GET  /api/pagamentos                — acesso comum (qualquer usuário autenticado) — lista pagamentos (paginado, filtros status_pagamento/forma_pagamento/pedido_id/vencimento_de/vencimento_ate)
//	POST /api/pagamentos                — acesso comum — cria pagamento
//	GET  /api/pagamentos/{id}            — acesso comum — detalhe de um pagamento
//	PUT  /api/pagamentos/{id}            — acesso comum — atualiza pagamento
//	GET  /health                        — público
func NewMux(cfg *config.Config, authH *handlers.AuthHandler, userH *handlers.UsuarioHandler, dashboardH *handlers.DashboardHandler, senhaH *handlers.SenhaHistoricoHandler, vendedorH *handlers.VendedorHandler, clienteH *handlers.ClienteHandler, produtoH *handlers.ProdutoHandler, pedidoH *handlers.PedidoHandler, pagamentoH *handlers.PagamentoHandler) http.Handler {
	mux := http.NewServeMux()

	// Login: middleware "não-protegido" (não exige token). Mas usamos um middleware
	// neutro para manter a cadeia única.
	loginChain := middleware.JWTMiddleware(cfg, false, false)(http.HandlerFunc(authH.Login))
	mux.Handle("/api/auth/login", loginChain)

	// Refresh token: público (envia refresh_token no body).
	refreshChain := middleware.JWTMiddleware(cfg, false, false)(http.HandlerFunc(authH.Refresh))
	mux.Handle("/api/auth/refresh", refreshChain)

	// Logout: público (envia refresh_token no body).
	logoutChain := middleware.JWTMiddleware(cfg, false, false)(http.HandlerFunc(authH.Logout))
	mux.Handle("/api/auth/logout", logoutChain)

	// Reset password: exige JWT, qualquer role (usuário troca a própria senha).
	resetChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(authH.ResetPassword))
	mux.Handle("/api/auth/reset-password", resetChain)

	// Me: exige JWT.
	meChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(authH.Me))
	mux.Handle("/api/auth/me", meChain)

	// Lista de usuários: admin only.
	listUsuariosChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(userH.ListUsuarios))
	mux.Handle("GET /api/usuarios", listUsuariosChain)

	// Cria usuário: admin only.
	createUsuarioChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(userH.CreateUsuario))
	mux.Handle("POST /api/usuarios", createUsuarioChain)

	// Atualiza usuário: admin only.
	updateUsuarioChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(userH.UpdateUsuario))
	mux.Handle("PUT /api/usuarios/{id}", updateUsuarioChain)

	// Toggle ativo/inativo: admin only.
	toggleAtivoChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(userH.ToggleAtivoUsuario))
	mux.Handle("PATCH /api/usuarios/{id}/inativar", toggleAtivoChain)

	// Admin reset password: admin only.
	adminResetChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(userH.AdminResetPassword))
	mux.Handle("POST /api/admin/reset-password", adminResetChain)

	// Senha histórico: admin only.
	// Importante: a rota /api/senha-historico/{user_id} deve ser registrada
	// ANTES da rota sem path param para que o matching funcione corretamente.
	senhaPorUsuarioChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(senhaH.ListarPorUsuario))
	mux.Handle("/api/senha-historico/", senhaPorUsuarioChain)

	senhaTodosChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(senhaH.ListarTodos))
	mux.Handle("/api/senha-historico", senhaTodosChain)

	// Dashboard metrics: admin only.
	metricsChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(dashboardH.GetMetrics))
	mux.Handle("/api/dashboard/metrics", metricsChain)

	// Dashboard vendas (serie temporal): admin only.
	vendasChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(dashboardH.GetVendas))
	mux.Handle("/api/dashboard/vendas", vendasChain)

	// Dashboard vendedores (ranking): admin only.
	vendedoresChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(dashboardH.GetVendedores))
	mux.Handle("/api/dashboard/vendedores", vendedoresChain)

	// Lista de vendedores (para popular selects no admin de usuários): admin only.
	listVendedoresChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(vendedorH.ListVendedores))
	mux.Handle("GET /api/vendedores", listVendedoresChain)

	// Dashboard clientes (métricas da base de clientes): admin only.
	dashboardClientesChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(dashboardH.GetClientes))
	mux.Handle("/api/dashboard/clientes", dashboardClientesChain)

	// Lista de clientes: admin only.
	listClientesChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(clienteH.ListClientes))
	mux.Handle("GET /api/clientes", listClientesChain)

	// Cria cliente: admin only.
	createClienteChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(clienteH.CreateCliente))
	mux.Handle("POST /api/clientes", createClienteChain)

	// Detalhe de cliente: admin only.
	getClienteChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(clienteH.GetCliente))
	mux.Handle("GET /api/clientes/{id}", getClienteChain)

	// Atualiza cliente: admin only.
	updateClienteChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(clienteH.UpdateCliente))
	mux.Handle("PUT /api/clientes/{id}", updateClienteChain)

	// Toggle ativo/inativo de cliente: admin only.
	toggleAtivoClienteChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(clienteH.ToggleAtivoCliente))
	mux.Handle("PATCH /api/clientes/{id}/inativar", toggleAtivoClienteChain)

	// Lista de produtos: admin only.
	listProdutosChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(produtoH.ListProdutos))
	mux.Handle("GET /api/produtos", listProdutosChain)

	// Cria produto: admin only.
	createProdutoChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(produtoH.CreateProduto))
	mux.Handle("POST /api/produtos", createProdutoChain)

	// Detalhe de produto: admin only.
	getProdutoChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(produtoH.GetProduto))
	mux.Handle("GET /api/produtos/{id}", getProdutoChain)

	// Atualiza produto: admin only.
	updateProdutoChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(produtoH.UpdateProduto))
	mux.Handle("PUT /api/produtos/{id}", updateProdutoChain)

	// Toggle ativo/inativo de produto: admin only.
	toggleAtivoProdutoChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(produtoH.ToggleAtivoProduto))
	mux.Handle("PATCH /api/produtos/{id}/inativar", toggleAtivoProdutoChain)

	// Lista de pedidos: admin only.
	listPedidosChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(pedidoH.ListPedidos))
	mux.Handle("GET /api/pedidos", listPedidosChain)

	// Cria pedido (com itens): admin only.
	createPedidoChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(pedidoH.CreatePedido))
	mux.Handle("POST /api/pedidos", createPedidoChain)

	// Detalhe de pedido (com itens): admin only.
	getPedidoChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(pedidoH.GetPedido))
	mux.Handle("GET /api/pedidos/{id}", getPedidoChain)

	// Atualiza pedido (substitui itens): admin only.
	updatePedidoChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(pedidoH.UpdatePedido))
	mux.Handle("PUT /api/pedidos/{id}", updatePedidoChain)

	// Lista de pagamentos: acesso comum (qualquer usuário autenticado, sem
	// exigir admin) — requireAuth=true, requireAdmin=false.
	listPagamentosChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(pagamentoH.ListPagamentos))
	mux.Handle("GET /api/pagamentos", listPagamentosChain)

	// Cria pagamento: acesso comum.
	createPagamentoChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(pagamentoH.CreatePagamento))
	mux.Handle("POST /api/pagamentos", createPagamentoChain)

	// Detalhe de pagamento: acesso comum.
	getPagamentoChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(pagamentoH.GetPagamento))
	mux.Handle("GET /api/pagamentos/{id}", getPagamentoChain)

	// Atualiza pagamento: acesso comum.
	updatePagamentoChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(pagamentoH.UpdatePagamento))
	mux.Handle("PUT /api/pagamentos/{id}", updatePagamentoChain)

	// Healthcheck.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true,"data":{"status":"ok"}}`))
	})

	// CORS — aplicado em TODAS as rotas (deve ser o middleware mais externo).
	// Origins permitidas: http://localhost:3000 (Next.js dev) + o que vier em CORS_ALLOWED_ORIGINS.
	corsChain := middleware.CORSMiddleware("http://localhost:3000")
	return corsChain(mux)
}
