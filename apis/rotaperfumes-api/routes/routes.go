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
//	GET  /api/usuarios                  — admin only (JWT + role=admin) — listagem paginada (order_by/order_dir opcionais)
//	POST /api/usuarios                  — admin only — criar usuário
//	PUT  /api/usuarios/{id}             — admin only — atualizar usuário
//	PATCH /api/usuarios/{id}/inativar   — admin only — ativar/inativar
//	POST /api/admin/reset-password      — admin only — resetar senha de outro usuário
//	GET  /api/vendedores                — acesso comum — lista vendedores ativos (sem paginação)
//	POST /api/vendedores                — acesso comum — criar vendedor
//	GET  /api/vendedores/{id}           — acesso comum — detalhe de um vendedor (com clientes vinculados)
//	PUT  /api/vendedores/{id}           — acesso comum — atualizar vendedor
//	DELETE /api/vendedores/{id}         — acesso comum — inativar vendedor (soft-delete via data_desligamento)
//	POST   /api/vendedores/{id}/reativar — acesso comum — reativar vendedor (limpa data_desligamento)
//	POST   /api/vendedores/{id}/clientes — acesso comum — vincula cliente à carteira do vendedor (transfere automaticamente se já vinculado a outro vendedor)
//	GET    /api/vendedores/{id}/clientes — acesso comum — lista clientes vinculados (carteira ativa) ao vendedor
//	DELETE /api/vendedores/{id}/clientes/{clienteId} — acesso comum — encerra vínculo ativo entre vendedor e cliente
//	GET  /api/senha-historico           — admin only — todo histórico de senhas (paginado, order_by/order_dir opcionais)
//	GET  /api/senha-historico/{user_id} — admin only — histórico de um usuário (paginado, order_by/order_dir opcionais)
//	GET  /api/dashboard/metrics         — acesso comum — métricas gerais (vendas, pedidos, ticket medio)
//	GET  /api/dashboard/vendas          — acesso comum — serie temporal de vendas (ultimos N dias)
//	GET  /api/dashboard/vendedores      — acesso comum — ranking de vendedores com meta
//	GET  /api/dashboard/clientes        — acesso comum — métricas da base de clientes (totais, novos, por segmento, por uf)
//	GET  /api/clientes                  — acesso comum — lista clientes (paginado, filtros uf/segmento/ativo/q, order_by/order_dir opcionais)
//	POST /api/clientes                  — acesso comum — criar cliente
//	GET  /api/clientes/{id}             — acesso comum — detalhe de um cliente
//	PUT  /api/clientes/{id}             — acesso comum — atualizar cliente
//	PATCH /api/clientes/{id}/inativar   — acesso comum — ativar/inativar cliente
//	GET  /api/produtos                  — acesso comum — lista produtos (paginado, filtros categoria/marca/ativo/q, order_by/order_dir opcionais)
//	POST /api/produtos                  — acesso comum — criar produto
//	GET  /api/produtos/{id}             — acesso comum — detalhe de um produto
//	PUT  /api/produtos/{id}             — acesso comum — atualizar produto
//	PATCH /api/produtos/{id}/inativar   — acesso comum — ativar/inativar produto
//	GET  /api/pedidos                   — acesso comum — lista pedidos (paginado, filtros status/canal/cliente_id/vendedor_id/data_inicio/data_fim/q, order_by/order_dir opcionais)
//	POST /api/pedidos                   — acesso comum — cria pedido com itens (calcula valor_bruto/valor_total)
//	GET  /api/pedidos/{id}               — acesso comum — detalhe de um pedido (com itens)
//	PUT  /api/pedidos/{id}               — acesso comum — atualiza pedido e substitui a lista de itens
//	GET  /api/pagamentos                — acesso comum (qualquer usuário autenticado) — lista pagamentos (paginado, filtros status_pagamento/forma_pagamento/pedido_id/vencimento_de/vencimento_ate, order_by/order_dir opcionais)
//	POST /api/pagamentos                — acesso comum — cria pagamento
//	GET  /api/pagamentos/{id}            — acesso comum — detalhe de um pagamento
//	PUT  /api/pagamentos/{id}            — acesso comum — atualiza pagamento
//	GET  /api/oportunidades             — admin only — lista oportunidades (paginado, filtros cliente_id/vendedor_id/etapa/origem/data_abertura_de/data_abertura_ate/q, order_by/order_dir opcionais)
//	POST /api/oportunidades             — admin only — cria oportunidade
//	GET  /api/oportunidades/{id}        — admin only — detalhe de uma oportunidade
//	PUT  /api/oportunidades/{id}        — admin only — atualiza oportunidade
//	GET  /health                        — público
func NewMux(cfg *config.Config, authH *handlers.AuthHandler, userH *handlers.UsuarioHandler, dashboardH *handlers.DashboardHandler, senhaH *handlers.SenhaHistoricoHandler, vendedorH *handlers.VendedorHandler, clienteH *handlers.ClienteHandler, produtoH *handlers.ProdutoHandler, pedidoH *handlers.PedidoHandler, pagamentoH *handlers.PagamentoHandler, oportunidadeH *handlers.OportunidadeHandler) http.Handler {
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

	// Dashboard metrics: acesso comum (qualquer usuário autenticado).
	metricsChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(dashboardH.GetMetrics))
	mux.Handle("/api/dashboard/metrics", metricsChain)

	// Dashboard vendas (serie temporal): acesso comum.
	vendasChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(dashboardH.GetVendas))
	mux.Handle("/api/dashboard/vendas", vendasChain)

	// Dashboard vendedores (ranking): acesso comum.
	vendedoresChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(dashboardH.GetVendedores))
	mux.Handle("/api/dashboard/vendedores", vendedoresChain)

	// Lista de vendedores (para popular selects): acesso comum.
	listVendedoresChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(vendedorH.ListVendedores))
	mux.Handle("GET /api/vendedores", listVendedoresChain)

	// Cria vendedor: acesso comum.
	createVendedorChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(vendedorH.CreateVendedor))
	mux.Handle("POST /api/vendedores", createVendedorChain)

	// Detalhe de vendedor (com clientes vinculados): acesso comum.
	getVendedorChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(vendedorH.GetVendedor))
	mux.Handle("GET /api/vendedores/{id}", getVendedorChain)

	// Atualiza vendedor: acesso comum.
	updateVendedorChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(vendedorH.UpdateVendedor))
	mux.Handle("PUT /api/vendedores/{id}", updateVendedorChain)

	// Inativa vendedor (soft-delete via data_desligamento): acesso comum.
	deleteVendedorChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(vendedorH.DeleteVendedor))
	mux.Handle("DELETE /api/vendedores/{id}", deleteVendedorChain)

	// Reativa vendedor (limpa data_desligamento): acesso comum.
	reativarVendedorChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(vendedorH.ReativarVendedor))
	mux.Handle("POST /api/vendedores/{id}/reativar", reativarVendedorChain)

	// Vincula cliente à carteira do vendedor (transfere automaticamente se já
	// vinculado a outro vendedor): acesso comum.
	vincularClienteChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(vendedorH.VincularCliente))
	mux.Handle("POST /api/vendedores/{id}/clientes", vincularClienteChain)

	// Desvincula cliente da carteira do vendedor (encerra vínculo ativo): acesso comum.
	desvincularClienteChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(vendedorH.DesvincularCliente))
	mux.Handle("DELETE /api/vendedores/{id}/clientes/{clienteId}", desvincularClienteChain)

	// Lista clientes vinculados (carteira ativa) a um vendedor: acesso comum
	// (usado pelo dropdown em cascata do frontend em Oportunidades).
	listClientesDoVendedorChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(vendedorH.ListClientesDoVendedor))
	mux.Handle("GET /api/vendedores/{id}/clientes", listClientesDoVendedorChain)

	// Dashboard clientes (métricas da base de clientes): acesso comum.
	dashboardClientesChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(dashboardH.GetClientes))
	mux.Handle("/api/dashboard/clientes", dashboardClientesChain)

	// Lista de clientes: acesso comum.
	listClientesChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(clienteH.ListClientes))
	mux.Handle("GET /api/clientes", listClientesChain)

	// Cria cliente: acesso comum.
	createClienteChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(clienteH.CreateCliente))
	mux.Handle("POST /api/clientes", createClienteChain)

	// Detalhe de cliente: acesso comum.
	getClienteChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(clienteH.GetCliente))
	mux.Handle("GET /api/clientes/{id}", getClienteChain)

	// Atualiza cliente: acesso comum.
	updateClienteChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(clienteH.UpdateCliente))
	mux.Handle("PUT /api/clientes/{id}", updateClienteChain)

	// Toggle ativo/inativo de cliente: acesso comum.
	toggleAtivoClienteChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(clienteH.ToggleAtivoCliente))
	mux.Handle("PATCH /api/clientes/{id}/inativar", toggleAtivoClienteChain)

	// Lista de produtos: acesso comum.
	listProdutosChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(produtoH.ListProdutos))
	mux.Handle("GET /api/produtos", listProdutosChain)

	// Cria produto: acesso comum.
	createProdutoChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(produtoH.CreateProduto))
	mux.Handle("POST /api/produtos", createProdutoChain)

	// Detalhe de produto: acesso comum.
	getProdutoChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(produtoH.GetProduto))
	mux.Handle("GET /api/produtos/{id}", getProdutoChain)

	// Atualiza produto: acesso comum.
	updateProdutoChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(produtoH.UpdateProduto))
	mux.Handle("PUT /api/produtos/{id}", updateProdutoChain)

	// Toggle ativo/inativo de produto: acesso comum.
	toggleAtivoProdutoChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(produtoH.ToggleAtivoProduto))
	mux.Handle("PATCH /api/produtos/{id}/inativar", toggleAtivoProdutoChain)

	// Lista de pedidos: acesso comum.
	listPedidosChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(pedidoH.ListPedidos))
	mux.Handle("GET /api/pedidos", listPedidosChain)

	// Cria pedido (com itens): acesso comum.
	createPedidoChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(pedidoH.CreatePedido))
	mux.Handle("POST /api/pedidos", createPedidoChain)

	// Detalhe de pedido (com itens): acesso comum.
	getPedidoChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(pedidoH.GetPedido))
	mux.Handle("GET /api/pedidos/{id}", getPedidoChain)

	// Atualiza pedido (substitui itens): acesso comum.
	updatePedidoChain := middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(pedidoH.UpdatePedido))
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

	// Lista de oportunidades: admin only.
	listOportunidadesChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(oportunidadeH.ListOportunidades))
	mux.Handle("GET /api/oportunidades", listOportunidadesChain)

	// Cria oportunidade: admin only.
	createOportunidadeChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(oportunidadeH.CreateOportunidade))
	mux.Handle("POST /api/oportunidades", createOportunidadeChain)

	// Detalhe de oportunidade: admin only.
	getOportunidadeChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(oportunidadeH.GetOportunidade))
	mux.Handle("GET /api/oportunidades/{id}", getOportunidadeChain)

	// Atualiza oportunidade: admin only.
	updateOportunidadeChain := middleware.JWTMiddleware(cfg, true, true)(http.HandlerFunc(oportunidadeH.UpdateOportunidade))
	mux.Handle("PUT /api/oportunidades/{id}", updateOportunidadeChain)

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
