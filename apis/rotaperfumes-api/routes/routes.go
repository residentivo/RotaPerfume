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
// userChecker (obrigatório, não nulo) verifica ativo/role do usuário no banco
// a cada request protegido — em produção middleware.NewDBUserStatusChecker(db).
//
// Rotas:
//
//	POST /api/auth/login                — público (requer captchaToken no body — Cloudflare Turnstile)
//	POST /api/auth/refresh              — público (usa refresh_token no body)
//	POST /api/auth/logout               — público (revoga refresh_token)
//	POST /api/auth/reset-password       — protegido (JWT, qualquer role autenticado; requer captchaToken no body e tem rate limiting próprio por IP+usuário)
//	GET  /api/auth/me                   — protegido (JWT)
//	GET  /api/usuarios                  — admin only (JWT + role=admin) — listagem paginada (order_by/order_dir opcionais)
//	POST /api/usuarios                  — admin only — criar usuário
//	PUT  /api/usuarios/{id}             — admin only — atualizar usuário
//	PATCH /api/usuarios/{id}/inativar   — admin only — ativar/inativar
//	POST /api/admin/reset-password      — admin only — resetar senha de outro usuário
//	GET  /api/vendedores                — acesso comum — lista vendedores ativos (sem paginação)
//	POST /api/vendedores                — admin only — criar vendedor
//	GET  /api/vendedores/{id}           — admin only — detalhe de um vendedor (com clientes vinculados)
//	PUT  /api/vendedores/{id}           — admin only — atualizar vendedor
//	DELETE /api/vendedores/{id}         — admin only — inativar vendedor (soft-delete via data_desligamento)
//	POST   /api/vendedores/{id}/reativar — admin only — reativar vendedor (limpa data_desligamento)
//	POST   /api/vendedores/{id}/clientes — admin only — vincula cliente à carteira do vendedor (transfere automaticamente se já vinculado a outro vendedor)
//	GET    /api/vendedores/{id}/clientes — acesso comum — lista clientes vinculados (carteira ativa) ao vendedor
//	DELETE /api/vendedores/{id}/clientes/{clienteId} — admin only — encerra vínculo ativo entre vendedor e cliente
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
//	POST /api/produtos                  — admin only — criar produto
//	GET  /api/produtos/{id}             — acesso comum — detalhe de um produto
//	PUT  /api/produtos/{id}             — admin only — atualizar produto
//	PATCH /api/produtos/{id}/inativar   — admin only — ativar/inativar produto
//	GET  /api/pedidos                   — acesso comum — lista pedidos (paginado, filtros status/canal/cliente_id/vendedor_id/data_inicio/data_fim/q, order_by/order_dir opcionais)
//	POST /api/pedidos                   — acesso comum (escopo por carteira) — cria pedido com itens (calcula valor_bruto/valor_total)
//	GET  /api/pedidos/{id}               — acesso comum (escopo por carteira) — detalhe de um pedido (com itens)
//	PUT  /api/pedidos/{id}               — acesso comum (escopo por carteira) — atualiza pedido e substitui a lista de itens
//	DELETE /api/pedidos/{id}             — acesso comum — exclui pedido e seus itens (hard delete); bloqueado (409) se houver pagamentos vinculados ou status = Faturado
//	GET  /api/pagamentos                — acesso comum (qualquer usuário autenticado) — lista pagamentos (paginado, filtros status_pagamento/forma_pagamento/pedido_id/vencimento_de/vencimento_ate, order_by/order_dir opcionais)
//	POST /api/pagamentos                — acesso comum (escopo por carteira) — cria pagamento
//	GET  /api/pagamentos/{id}            — acesso comum (escopo por carteira) — detalhe de um pagamento
//	PUT  /api/pagamentos/{id}            — acesso comum (escopo por carteira) — atualiza pagamento
//	DELETE /api/pagamentos/{id}          — acesso comum — exclui pagamento (hard delete); bloqueado (409) se status_pagamento = Pago ou Pago com atraso
//	GET  /api/oportunidades             — acesso comum (escopo por carteira) — lista oportunidades (paginado, filtros cliente_id/vendedor_id/etapa/origem/data_abertura_de/data_abertura_ate/q, order_by/order_dir opcionais)
//	POST /api/oportunidades             — acesso comum (escopo por carteira) — cria oportunidade
//	GET  /api/oportunidades/{id}        — acesso comum (escopo por carteira) — detalhe de uma oportunidade
//	PUT  /api/oportunidades/{id}        — acesso comum (escopo por carteira) — atualiza oportunidade
//	DELETE /api/oportunidades/{id}      — acesso comum (escopo por carteira) — exclui oportunidade (hard delete)
//	GET  /api/visitas                   — acesso comum (escopo por carteira) — lista visitas (paginado, filtros cliente_id/vendedor_id/resultado/data_visita_de/data_visita_ate/q, order_by/order_dir opcionais)
//	POST /api/visitas                   — acesso comum (escopo por carteira) — cria visita
//	GET  /api/visitas/{id}               — acesso comum (escopo por carteira) — detalhe de uma visita
//	PUT  /api/visitas/{id}               — acesso comum (escopo por carteira) — atualiza visita
//	DELETE /api/visitas/{id}             — acesso comum (escopo por carteira) — exclui visita (hard delete)
//	GET  /api/estoque                   — admin only — lista estoque (paginado; por padrão última posição por sku; filtros sku/data_de/data_ate/ruptura, order_by/order_dir/historico opcionais)
//	POST /api/estoque                   — admin only — cria ajuste manual de estoque
//	GET  /api/estoque/{id}               — admin only — detalhe de um registro de estoque
//	PUT  /api/estoque/{id}               — admin only — atualiza (ajuste manual) saldo de um registro de estoque
//	GET  /health                        — público
func NewMux(cfg *config.Config, userChecker middleware.UserStatusChecker, authH *handlers.AuthHandler, userH *handlers.UsuarioHandler, dashboardH *handlers.DashboardHandler, senhaH *handlers.SenhaHistoricoHandler, vendedorH *handlers.VendedorHandler, clienteH *handlers.ClienteHandler, produtoH *handlers.ProdutoHandler, pedidoH *handlers.PedidoHandler, pagamentoH *handlers.PagamentoHandler, oportunidadeH *handlers.OportunidadeHandler, visitaH *handlers.VisitaHandler, estoqueH *handlers.EstoqueHandler) http.Handler {
	mux := http.NewServeMux()

	// jwt monta o middleware de autenticação de cada rota. Rotas protegidas
	// consultam o usuário no banco a cada request (SEC-06): inativo/inexistente
	// → 401 e o role do banco prevalece sobre o do token. Rotas públicas
	// (protected=false) não consultam o banco.
	jwt := func(protected, requireAdmin bool) func(http.Handler) http.Handler {
		return middleware.JWTMiddlewareWithUserCheck(cfg, userChecker, protected, requireAdmin)
	}

	// Login: middleware "não-protegido" (não exige token). Mas usamos um middleware
	// neutro para manter a cadeia única.
	loginChain := jwt(false, false)(http.HandlerFunc(authH.Login))
	mux.Handle("/api/auth/login", loginChain)

	// Refresh token: público (envia refresh_token no body).
	refreshChain := jwt(false, false)(http.HandlerFunc(authH.Refresh))
	mux.Handle("/api/auth/refresh", refreshChain)

	// Logout: público (envia refresh_token no body).
	logoutChain := jwt(false, false)(http.HandlerFunc(authH.Logout))
	mux.Handle("/api/auth/logout", logoutChain)

	// Reset password: exige JWT, qualquer role (usuário troca a própria senha).
	resetChain := jwt(true, false)(http.HandlerFunc(authH.ResetPassword))
	mux.Handle("/api/auth/reset-password", resetChain)

	// Me: exige JWT.
	meChain := jwt(true, false)(http.HandlerFunc(authH.Me))
	mux.Handle("/api/auth/me", meChain)

	// Lista de usuários: admin only.
	listUsuariosChain := jwt(true, true)(http.HandlerFunc(userH.ListUsuarios))
	mux.Handle("GET /api/usuarios", listUsuariosChain)

	// Cria usuário: admin only.
	createUsuarioChain := jwt(true, true)(http.HandlerFunc(userH.CreateUsuario))
	mux.Handle("POST /api/usuarios", createUsuarioChain)

	// Atualiza usuário: admin only.
	updateUsuarioChain := jwt(true, true)(http.HandlerFunc(userH.UpdateUsuario))
	mux.Handle("PUT /api/usuarios/{id}", updateUsuarioChain)

	// Toggle ativo/inativo: admin only.
	toggleAtivoChain := jwt(true, true)(http.HandlerFunc(userH.ToggleAtivoUsuario))
	mux.Handle("PATCH /api/usuarios/{id}/inativar", toggleAtivoChain)

	// Admin reset password: admin only.
	adminResetChain := jwt(true, true)(http.HandlerFunc(userH.AdminResetPassword))
	mux.Handle("POST /api/admin/reset-password", adminResetChain)

	// Senha histórico: admin only.
	// Importante: a rota /api/senha-historico/{user_id} deve ser registrada
	// ANTES da rota sem path param para que o matching funcione corretamente.
	senhaPorUsuarioChain := jwt(true, true)(http.HandlerFunc(senhaH.ListarPorUsuario))
	mux.Handle("/api/senha-historico/", senhaPorUsuarioChain)

	senhaTodosChain := jwt(true, true)(http.HandlerFunc(senhaH.ListarTodos))
	mux.Handle("/api/senha-historico", senhaTodosChain)

	// Dashboard metrics: acesso comum (qualquer usuário autenticado)
	// (escopo por vendedor: normal vê só os próprios números).
	metricsChain := jwt(true, false)(http.HandlerFunc(dashboardH.GetMetrics))
	mux.Handle("/api/dashboard/metrics", metricsChain)

	// Dashboard vendas (serie temporal): acesso comum
	// (escopo por vendedor: normal vê só os próprios números).
	vendasChain := jwt(true, false)(http.HandlerFunc(dashboardH.GetVendas))
	mux.Handle("/api/dashboard/vendas", vendasChain)

	// Dashboard vendedores (ranking): acesso comum
	// (escopo por vendedor: normal vê só os próprios números).
	vendedoresChain := jwt(true, false)(http.HandlerFunc(dashboardH.GetVendedores))
	mux.Handle("/api/dashboard/vendedores", vendedoresChain)

	// Lista de vendedores (para popular selects): acesso comum.
	listVendedoresChain := jwt(true, false)(http.HandlerFunc(vendedorH.ListVendedores))
	mux.Handle("GET /api/vendedores", listVendedoresChain)

	// Cria vendedor: admin only (gestão de vendedores/carteiras é restrita a
	// administradores — parecer SecBrain).
	createVendedorChain := jwt(true, true)(http.HandlerFunc(vendedorH.CreateVendedor))
	mux.Handle("POST /api/vendedores", createVendedorChain)

	// Detalhe de vendedor (com clientes vinculados): admin only.
	getVendedorChain := jwt(true, true)(http.HandlerFunc(vendedorH.GetVendedor))
	mux.Handle("GET /api/vendedores/{id}", getVendedorChain)

	// Atualiza vendedor: admin only.
	updateVendedorChain := jwt(true, true)(http.HandlerFunc(vendedorH.UpdateVendedor))
	mux.Handle("PUT /api/vendedores/{id}", updateVendedorChain)

	// Inativa vendedor (soft-delete via data_desligamento): admin only.
	deleteVendedorChain := jwt(true, true)(http.HandlerFunc(vendedorH.DeleteVendedor))
	mux.Handle("DELETE /api/vendedores/{id}", deleteVendedorChain)

	// Reativa vendedor (limpa data_desligamento): admin only.
	reativarVendedorChain := jwt(true, true)(http.HandlerFunc(vendedorH.ReativarVendedor))
	mux.Handle("POST /api/vendedores/{id}/reativar", reativarVendedorChain)

	// Vincula cliente à carteira do vendedor (transfere automaticamente se já
	// vinculado a outro vendedor): admin only.
	vincularClienteChain := jwt(true, true)(http.HandlerFunc(vendedorH.VincularCliente))
	mux.Handle("POST /api/vendedores/{id}/clientes", vincularClienteChain)

	// Desvincula cliente da carteira do vendedor (encerra vínculo ativo): admin only.
	desvincularClienteChain := jwt(true, true)(http.HandlerFunc(vendedorH.DesvincularCliente))
	mux.Handle("DELETE /api/vendedores/{id}/clientes/{clienteId}", desvincularClienteChain)

	// Lista clientes vinculados (carteira ativa) a um vendedor: acesso comum
	// (usado pelo dropdown em cascata do frontend em Oportunidades).
	listClientesDoVendedorChain := jwt(true, false)(http.HandlerFunc(vendedorH.ListClientesDoVendedor))
	mux.Handle("GET /api/vendedores/{id}/clientes", listClientesDoVendedorChain)

	// Dashboard clientes (métricas da base de clientes): acesso comum.
	dashboardClientesChain := jwt(true, false)(http.HandlerFunc(dashboardH.GetClientes))
	mux.Handle("/api/dashboard/clientes", dashboardClientesChain)

	// Lista de clientes: acesso comum.
	listClientesChain := jwt(true, false)(http.HandlerFunc(clienteH.ListClientes))
	mux.Handle("GET /api/clientes", listClientesChain)

	// Cria cliente: acesso comum.
	createClienteChain := jwt(true, false)(http.HandlerFunc(clienteH.CreateCliente))
	mux.Handle("POST /api/clientes", createClienteChain)

	// Detalhe de cliente: acesso comum.
	getClienteChain := jwt(true, false)(http.HandlerFunc(clienteH.GetCliente))
	mux.Handle("GET /api/clientes/{id}", getClienteChain)

	// Atualiza cliente: acesso comum.
	updateClienteChain := jwt(true, false)(http.HandlerFunc(clienteH.UpdateCliente))
	mux.Handle("PUT /api/clientes/{id}", updateClienteChain)

	// Toggle ativo/inativo de cliente: acesso comum.
	toggleAtivoClienteChain := jwt(true, false)(http.HandlerFunc(clienteH.ToggleAtivoCliente))
	mux.Handle("PATCH /api/clientes/{id}/inativar", toggleAtivoClienteChain)

	// Lista de produtos: acesso comum.
	listProdutosChain := jwt(true, false)(http.HandlerFunc(produtoH.ListProdutos))
	mux.Handle("GET /api/produtos", listProdutosChain)

	// Cria produto: admin only (gestão de catálogo movida para Administração;
	// leitura continua liberada para o vendedor montar pedidos).
	createProdutoChain := jwt(true, true)(http.HandlerFunc(produtoH.CreateProduto))
	mux.Handle("POST /api/produtos", createProdutoChain)

	// Detalhe de produto: acesso comum.
	getProdutoChain := jwt(true, false)(http.HandlerFunc(produtoH.GetProduto))
	mux.Handle("GET /api/produtos/{id}", getProdutoChain)

	// Atualiza produto: admin only.
	updateProdutoChain := jwt(true, true)(http.HandlerFunc(produtoH.UpdateProduto))
	mux.Handle("PUT /api/produtos/{id}", updateProdutoChain)

	// Toggle ativo/inativo de produto: admin only.
	toggleAtivoProdutoChain := jwt(true, true)(http.HandlerFunc(produtoH.ToggleAtivoProduto))
	mux.Handle("PATCH /api/produtos/{id}/inativar", toggleAtivoProdutoChain)

	// Lista de pedidos: acesso comum.
	listPedidosChain := jwt(true, false)(http.HandlerFunc(pedidoH.ListPedidos))
	mux.Handle("GET /api/pedidos", listPedidosChain)

	// Cria pedido (com itens): acesso comum (escopo por carteira aplicado no handler).
	createPedidoChain := jwt(true, false)(http.HandlerFunc(pedidoH.CreatePedido))
	mux.Handle("POST /api/pedidos", createPedidoChain)

	// Detalhe de pedido (com itens): acesso comum.
	getPedidoChain := jwt(true, false)(http.HandlerFunc(pedidoH.GetPedido))
	mux.Handle("GET /api/pedidos/{id}", getPedidoChain)

	// Atualiza pedido (substitui itens): acesso comum (escopo por carteira aplicado no handler).
	updatePedidoChain := jwt(true, false)(http.HandlerFunc(pedidoH.UpdatePedido))
	mux.Handle("PUT /api/pedidos/{id}", updatePedidoChain)

	// Exclui pedido (hard delete, com itens): acesso comum.
	deletePedidoChain := jwt(true, false)(http.HandlerFunc(pedidoH.DeletePedido))
	mux.Handle("DELETE /api/pedidos/{id}", deletePedidoChain)

	// Lista de pagamentos: acesso comum (qualquer usuário autenticado, sem
	// exigir admin) — requireAuth=true, requireAdmin=false.
	listPagamentosChain := jwt(true, false)(http.HandlerFunc(pagamentoH.ListPagamentos))
	mux.Handle("GET /api/pagamentos", listPagamentosChain)

	// Cria pagamento: acesso comum (escopo por carteira aplicado no handler).
	createPagamentoChain := jwt(true, false)(http.HandlerFunc(pagamentoH.CreatePagamento))
	mux.Handle("POST /api/pagamentos", createPagamentoChain)

	// Detalhe de pagamento: acesso comum.
	getPagamentoChain := jwt(true, false)(http.HandlerFunc(pagamentoH.GetPagamento))
	mux.Handle("GET /api/pagamentos/{id}", getPagamentoChain)

	// Atualiza pagamento: acesso comum (escopo por carteira aplicado no handler).
	updatePagamentoChain := jwt(true, false)(http.HandlerFunc(pagamentoH.UpdatePagamento))
	mux.Handle("PUT /api/pagamentos/{id}", updatePagamentoChain)

	// Exclui pagamento (hard delete): acesso comum.
	deletePagamentoChain := jwt(true, false)(http.HandlerFunc(pagamentoH.DeletePagamento))
	mux.Handle("DELETE /api/pagamentos/{id}", deletePagamentoChain)

	// Lista de oportunidades: acesso comum (escopo por carteira aplicado no handler).
	listOportunidadesChain := jwt(true, false)(http.HandlerFunc(oportunidadeH.ListOportunidades))
	mux.Handle("GET /api/oportunidades", listOportunidadesChain)

	// Cria oportunidade: acesso comum (escopo por carteira aplicado no handler).
	createOportunidadeChain := jwt(true, false)(http.HandlerFunc(oportunidadeH.CreateOportunidade))
	mux.Handle("POST /api/oportunidades", createOportunidadeChain)

	// Detalhe de oportunidade: acesso comum (escopo por carteira aplicado no handler).
	getOportunidadeChain := jwt(true, false)(http.HandlerFunc(oportunidadeH.GetOportunidade))
	mux.Handle("GET /api/oportunidades/{id}", getOportunidadeChain)

	// Atualiza oportunidade: acesso comum (escopo por carteira aplicado no handler).
	updateOportunidadeChain := jwt(true, false)(http.HandlerFunc(oportunidadeH.UpdateOportunidade))
	mux.Handle("PUT /api/oportunidades/{id}", updateOportunidadeChain)

	// Exclui oportunidade (hard delete): acesso comum (escopo por carteira aplicado no handler).
	deleteOportunidadeChain := jwt(true, false)(http.HandlerFunc(oportunidadeH.DeleteOportunidade))
	mux.Handle("DELETE /api/oportunidades/{id}", deleteOportunidadeChain)

	// Lista de visitas: acesso comum (escopo por carteira aplicado no handler).
	listVisitasChain := jwt(true, false)(http.HandlerFunc(visitaH.ListVisitas))
	mux.Handle("GET /api/visitas", listVisitasChain)

	// Cria visita: acesso comum (escopo por carteira aplicado no handler).
	createVisitaChain := jwt(true, false)(http.HandlerFunc(visitaH.CreateVisita))
	mux.Handle("POST /api/visitas", createVisitaChain)

	// Detalhe de visita: acesso comum (escopo por carteira aplicado no handler).
	getVisitaChain := jwt(true, false)(http.HandlerFunc(visitaH.GetVisita))
	mux.Handle("GET /api/visitas/{id}", getVisitaChain)

	// Atualiza visita: acesso comum (escopo por carteira aplicado no handler).
	updateVisitaChain := jwt(true, false)(http.HandlerFunc(visitaH.UpdateVisita))
	mux.Handle("PUT /api/visitas/{id}", updateVisitaChain)

	// Exclui visita (hard delete): acesso comum (escopo por carteira aplicado no handler).
	deleteVisitaChain := jwt(true, false)(http.HandlerFunc(visitaH.DeleteVisita))
	mux.Handle("DELETE /api/visitas/{id}", deleteVisitaChain)

	// Lista de estoque: admin only (página de Estoque é restrita a administradores).
	listEstoqueChain := jwt(true, true)(http.HandlerFunc(estoqueH.ListEstoque))
	mux.Handle("GET /api/estoque", listEstoqueChain)

	// Cria ajuste manual de estoque: admin only.
	createEstoqueChain := jwt(true, true)(http.HandlerFunc(estoqueH.CreateEstoque))
	mux.Handle("POST /api/estoque", createEstoqueChain)

	// Detalhe de um registro de estoque: admin only.
	getEstoqueChain := jwt(true, true)(http.HandlerFunc(estoqueH.GetEstoque))
	mux.Handle("GET /api/estoque/{id}", getEstoqueChain)

	// Atualiza (ajuste manual) saldo de um registro de estoque: admin only.
	updateEstoqueChain := jwt(true, true)(http.HandlerFunc(estoqueH.UpdateEstoque))
	mux.Handle("PUT /api/estoque/{id}", updateEstoqueChain)

	// Healthcheck.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true,"data":{"status":"ok"}}`))
	})

	// Security headers — aplicado em TODAS as rotas.
	securityChain := middleware.SecurityHeadersMiddleware()

	// CORS — aplicado em TODAS as rotas (deve ser o middleware mais externo).
	// Origins permitidas: lista explícita (comparação exata) em
	// cfg.CORSAllowedOrigins, que já inclui as origens de dev local padrão
	// mais o que vier em CORS_ALLOWED_ORIGINS.
	corsChain := middleware.CORSMiddleware(cfg.CORSAllowedOrigins)
	return corsChain(securityChain(mux))
}
