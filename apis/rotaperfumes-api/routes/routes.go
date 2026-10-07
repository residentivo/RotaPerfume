// Package routes registra as rotas HTTP da API.
package routes

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/vlog"
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
	vlog.Printf("routes.go", "NewMux", "chamando http.NewServeMux e declarando mux")
	mux := http.NewServeMux()

	// jwt monta o middleware de autenticação de cada rota. Rotas protegidas
	// consultam o usuário no banco a cada request (SEC-06): inativo/inexistente
	// → 401 e o role do banco prevalece sobre o do token. Rotas públicas
	// (protected=false) não consultam o banco.
	vlog.Printf("routes.go", "NewMux", "definindo função anônima e declarando jwt")
	jwt := func(protected, requireAdmin bool) func(http.Handler) http.Handler {
		return middleware.JWTMiddlewareWithUserCheck(cfg, userChecker, protected, requireAdmin)
	}

	// Login: middleware "não-protegido" (não exige token). Mas usamos um middleware
	// neutro para manter a cadeia única.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(false, false) e declarando loginChain")
	loginChain := jwt(false, false)(http.HandlerFunc(authH.Login))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/auth/login", loginChain)

	// Refresh token: público (envia refresh_token no body).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(false, false) e declarando refreshChain")
	refreshChain := jwt(false, false)(http.HandlerFunc(authH.Refresh))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/auth/refresh", refreshChain)

	// Logout: público (envia refresh_token no body).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(false, false) e declarando logoutChain")
	logoutChain := jwt(false, false)(http.HandlerFunc(authH.Logout))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/auth/logout", logoutChain)

	// Reset password: exige JWT, qualquer role (usuário troca a própria senha).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando resetChain")
	resetChain := jwt(true, false)(http.HandlerFunc(authH.ResetPassword))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/auth/reset-password", resetChain)

	// Me: exige JWT.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando meChain")
	meChain := jwt(true, false)(http.HandlerFunc(authH.Me))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/auth/me", meChain)

	// Lista de usuários: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando listUsuariosChain")
	listUsuariosChain := jwt(true, true)(http.HandlerFunc(userH.ListUsuarios))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/usuarios", listUsuariosChain)

	// Cria usuário: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando createUsuarioChain")
	createUsuarioChain := jwt(true, true)(http.HandlerFunc(userH.CreateUsuario))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/usuarios", createUsuarioChain)

	// Atualiza usuário: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando updateUsuarioChain")
	updateUsuarioChain := jwt(true, true)(http.HandlerFunc(userH.UpdateUsuario))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PUT /api/usuarios/{id}", updateUsuarioChain)

	// Toggle ativo/inativo: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando toggleAtivoChain")
	toggleAtivoChain := jwt(true, true)(http.HandlerFunc(userH.ToggleAtivoUsuario))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PATCH /api/usuarios/{id}/inativar", toggleAtivoChain)

	// Admin reset password: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando adminResetChain")
	adminResetChain := jwt(true, true)(http.HandlerFunc(userH.AdminResetPassword))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/admin/reset-password", adminResetChain)

	// Senha histórico: admin only.
	// Importante: a rota /api/senha-historico/{user_id} deve ser registrada
	// ANTES da rota sem path param para que o matching funcione corretamente.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando senhaPorUsuarioChain")
	senhaPorUsuarioChain := jwt(true, true)(http.HandlerFunc(senhaH.ListarPorUsuario))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/senha-historico/", senhaPorUsuarioChain)

	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando senhaTodosChain")
	senhaTodosChain := jwt(true, true)(http.HandlerFunc(senhaH.ListarTodos))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/senha-historico", senhaTodosChain)

	// Dashboard metrics: acesso comum (qualquer usuário autenticado)
	// (escopo por vendedor: normal vê só os próprios números).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando metricsChain")
	metricsChain := jwt(true, false)(http.HandlerFunc(dashboardH.GetMetrics))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/dashboard/metrics", metricsChain)

	// Dashboard vendas (serie temporal): acesso comum
	// (escopo por vendedor: normal vê só os próprios números).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando vendasChain")
	vendasChain := jwt(true, false)(http.HandlerFunc(dashboardH.GetVendas))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/dashboard/vendas", vendasChain)

	// Dashboard vendedores (ranking): acesso comum
	// (escopo por vendedor: normal vê só os próprios números).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando vendedoresChain")
	vendedoresChain := jwt(true, false)(http.HandlerFunc(dashboardH.GetVendedores))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/dashboard/vendedores", vendedoresChain)

	// Lista de vendedores (para popular selects): acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando listVendedoresChain")
	listVendedoresChain := jwt(true, false)(http.HandlerFunc(vendedorH.ListVendedores))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/vendedores", listVendedoresChain)

	// Cria vendedor: admin only (gestão de vendedores/carteiras é restrita a
	// administradores — parecer SecBrain).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando createVendedorChain")
	createVendedorChain := jwt(true, true)(http.HandlerFunc(vendedorH.CreateVendedor))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/vendedores", createVendedorChain)

	// Detalhe de vendedor (com clientes vinculados): admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando getVendedorChain")
	getVendedorChain := jwt(true, true)(http.HandlerFunc(vendedorH.GetVendedor))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/vendedores/{id}", getVendedorChain)

	// Atualiza vendedor: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando updateVendedorChain")
	updateVendedorChain := jwt(true, true)(http.HandlerFunc(vendedorH.UpdateVendedor))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PUT /api/vendedores/{id}", updateVendedorChain)

	// Inativa vendedor (soft-delete via data_desligamento): admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando deleteVendedorChain")
	deleteVendedorChain := jwt(true, true)(http.HandlerFunc(vendedorH.DeleteVendedor))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("DELETE /api/vendedores/{id}", deleteVendedorChain)

	// Reativa vendedor (limpa data_desligamento): admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando reativarVendedorChain")
	reativarVendedorChain := jwt(true, true)(http.HandlerFunc(vendedorH.ReativarVendedor))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/vendedores/{id}/reativar", reativarVendedorChain)

	// Vincula cliente à carteira do vendedor (transfere automaticamente se já
	// vinculado a outro vendedor): admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando vincularClienteChain")
	vincularClienteChain := jwt(true, true)(http.HandlerFunc(vendedorH.VincularCliente))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/vendedores/{id}/clientes", vincularClienteChain)

	// Desvincula cliente da carteira do vendedor (encerra vínculo ativo): admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando desvincularClienteChain")
	desvincularClienteChain := jwt(true, true)(http.HandlerFunc(vendedorH.DesvincularCliente))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("DELETE /api/vendedores/{id}/clientes/{clienteId}", desvincularClienteChain)

	// Lista clientes vinculados (carteira ativa) a um vendedor: acesso comum
	// (usado pelo dropdown em cascata do frontend em Oportunidades).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando listClientesDoVendedorChain")
	listClientesDoVendedorChain := jwt(true, false)(http.HandlerFunc(vendedorH.ListClientesDoVendedor))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/vendedores/{id}/clientes", listClientesDoVendedorChain)

	// Dashboard clientes (métricas da base de clientes): acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando dashboardClientesChain")
	dashboardClientesChain := jwt(true, false)(http.HandlerFunc(dashboardH.GetClientes))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("/api/dashboard/clientes", dashboardClientesChain)

	// Lista de clientes: acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando listClientesChain")
	listClientesChain := jwt(true, false)(http.HandlerFunc(clienteH.ListClientes))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/clientes", listClientesChain)

	// Cria cliente: acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando createClienteChain")
	createClienteChain := jwt(true, false)(http.HandlerFunc(clienteH.CreateCliente))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/clientes", createClienteChain)

	// Detalhe de cliente: acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando getClienteChain")
	getClienteChain := jwt(true, false)(http.HandlerFunc(clienteH.GetCliente))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/clientes/{id}", getClienteChain)

	// Atualiza cliente: acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando updateClienteChain")
	updateClienteChain := jwt(true, false)(http.HandlerFunc(clienteH.UpdateCliente))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PUT /api/clientes/{id}", updateClienteChain)

	// Toggle ativo/inativo de cliente: acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando toggleAtivoClienteChain")
	toggleAtivoClienteChain := jwt(true, false)(http.HandlerFunc(clienteH.ToggleAtivoCliente))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PATCH /api/clientes/{id}/inativar", toggleAtivoClienteChain)

	// Lista de produtos: acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando listProdutosChain")
	listProdutosChain := jwt(true, false)(http.HandlerFunc(produtoH.ListProdutos))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/produtos", listProdutosChain)

	// Cria produto: admin only (gestão de catálogo movida para Administração;
	// leitura continua liberada para o vendedor montar pedidos).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando createProdutoChain")
	createProdutoChain := jwt(true, true)(http.HandlerFunc(produtoH.CreateProduto))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/produtos", createProdutoChain)

	// Detalhe de produto: acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando getProdutoChain")
	getProdutoChain := jwt(true, false)(http.HandlerFunc(produtoH.GetProduto))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/produtos/{id}", getProdutoChain)

	// Atualiza produto: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando updateProdutoChain")
	updateProdutoChain := jwt(true, true)(http.HandlerFunc(produtoH.UpdateProduto))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PUT /api/produtos/{id}", updateProdutoChain)

	// Toggle ativo/inativo de produto: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando toggleAtivoProdutoChain")
	toggleAtivoProdutoChain := jwt(true, true)(http.HandlerFunc(produtoH.ToggleAtivoProduto))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PATCH /api/produtos/{id}/inativar", toggleAtivoProdutoChain)

	// Lista de pedidos: acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando listPedidosChain")
	listPedidosChain := jwt(true, false)(http.HandlerFunc(pedidoH.ListPedidos))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/pedidos", listPedidosChain)

	// Cria pedido (com itens): acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando createPedidoChain")
	createPedidoChain := jwt(true, false)(http.HandlerFunc(pedidoH.CreatePedido))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/pedidos", createPedidoChain)

	// Detalhe de pedido (com itens): acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando getPedidoChain")
	getPedidoChain := jwt(true, false)(http.HandlerFunc(pedidoH.GetPedido))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/pedidos/{id}", getPedidoChain)

	// Atualiza pedido (substitui itens): acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando updatePedidoChain")
	updatePedidoChain := jwt(true, false)(http.HandlerFunc(pedidoH.UpdatePedido))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PUT /api/pedidos/{id}", updatePedidoChain)

	// Exclui pedido (hard delete, com itens): acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando deletePedidoChain")
	deletePedidoChain := jwt(true, false)(http.HandlerFunc(pedidoH.DeletePedido))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("DELETE /api/pedidos/{id}", deletePedidoChain)

	// Lista de pagamentos: acesso comum (qualquer usuário autenticado, sem
	// exigir admin) — requireAuth=true, requireAdmin=false.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando listPagamentosChain")
	listPagamentosChain := jwt(true, false)(http.HandlerFunc(pagamentoH.ListPagamentos))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/pagamentos", listPagamentosChain)

	// Cria pagamento: acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando createPagamentoChain")
	createPagamentoChain := jwt(true, false)(http.HandlerFunc(pagamentoH.CreatePagamento))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/pagamentos", createPagamentoChain)

	// Detalhe de pagamento: acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando getPagamentoChain")
	getPagamentoChain := jwt(true, false)(http.HandlerFunc(pagamentoH.GetPagamento))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/pagamentos/{id}", getPagamentoChain)

	// Atualiza pagamento: acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando updatePagamentoChain")
	updatePagamentoChain := jwt(true, false)(http.HandlerFunc(pagamentoH.UpdatePagamento))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PUT /api/pagamentos/{id}", updatePagamentoChain)

	// Exclui pagamento (hard delete): acesso comum.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando deletePagamentoChain")
	deletePagamentoChain := jwt(true, false)(http.HandlerFunc(pagamentoH.DeletePagamento))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("DELETE /api/pagamentos/{id}", deletePagamentoChain)

	// Lista de oportunidades: acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando listOportunidadesChain")
	listOportunidadesChain := jwt(true, false)(http.HandlerFunc(oportunidadeH.ListOportunidades))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/oportunidades", listOportunidadesChain)

	// Cria oportunidade: acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando createOportunidadeChain")
	createOportunidadeChain := jwt(true, false)(http.HandlerFunc(oportunidadeH.CreateOportunidade))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/oportunidades", createOportunidadeChain)

	// Detalhe de oportunidade: acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando getOportunidadeChain")
	getOportunidadeChain := jwt(true, false)(http.HandlerFunc(oportunidadeH.GetOportunidade))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/oportunidades/{id}", getOportunidadeChain)

	// Atualiza oportunidade: acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando updateOportunidadeChain")
	updateOportunidadeChain := jwt(true, false)(http.HandlerFunc(oportunidadeH.UpdateOportunidade))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PUT /api/oportunidades/{id}", updateOportunidadeChain)

	// Exclui oportunidade (hard delete): acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando deleteOportunidadeChain")
	deleteOportunidadeChain := jwt(true, false)(http.HandlerFunc(oportunidadeH.DeleteOportunidade))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("DELETE /api/oportunidades/{id}", deleteOportunidadeChain)

	// Lista de visitas: acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando listVisitasChain")
	listVisitasChain := jwt(true, false)(http.HandlerFunc(visitaH.ListVisitas))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/visitas", listVisitasChain)

	// Cria visita: acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando createVisitaChain")
	createVisitaChain := jwt(true, false)(http.HandlerFunc(visitaH.CreateVisita))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/visitas", createVisitaChain)

	// Detalhe de visita: acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando getVisitaChain")
	getVisitaChain := jwt(true, false)(http.HandlerFunc(visitaH.GetVisita))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/visitas/{id}", getVisitaChain)

	// Atualiza visita: acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando updateVisitaChain")
	updateVisitaChain := jwt(true, false)(http.HandlerFunc(visitaH.UpdateVisita))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PUT /api/visitas/{id}", updateVisitaChain)

	// Exclui visita (hard delete): acesso comum (escopo por carteira aplicado no handler).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, false) e declarando deleteVisitaChain")
	deleteVisitaChain := jwt(true, false)(http.HandlerFunc(visitaH.DeleteVisita))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("DELETE /api/visitas/{id}", deleteVisitaChain)

	// Lista de estoque: admin only (página de Estoque é restrita a administradores).
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando listEstoqueChain")
	listEstoqueChain := jwt(true, true)(http.HandlerFunc(estoqueH.ListEstoque))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/estoque", listEstoqueChain)

	// Cria ajuste manual de estoque: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando createEstoqueChain")
	createEstoqueChain := jwt(true, true)(http.HandlerFunc(estoqueH.CreateEstoque))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("POST /api/estoque", createEstoqueChain)

	// Detalhe de um registro de estoque: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando getEstoqueChain")
	getEstoqueChain := jwt(true, true)(http.HandlerFunc(estoqueH.GetEstoque))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("GET /api/estoque/{id}", getEstoqueChain)

	// Atualiza (ajuste manual) saldo de um registro de estoque: admin only.
	vlog.Printf("routes.go", "NewMux", "chamando jwt(true, true) e declarando updateEstoqueChain")
	updateEstoqueChain := jwt(true, true)(http.HandlerFunc(estoqueH.UpdateEstoque))
	vlog.Printf("routes.go", "NewMux", "chamando mux.Handle")
	mux.Handle("PUT /api/estoque/{id}", updateEstoqueChain)

	// Healthcheck.
	vlog.Printf("routes.go", "NewMux", "chamando mux.HandleFunc")
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		vlog.Printf("routes.go", "NewMux.func", "chamando w.Header().Set")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true,"data":{"status":"ok"}}`))
	})

	// Security headers — aplicado em TODAS as rotas.
	vlog.Printf("routes.go", "NewMux", "chamando middleware.SecurityHeadersMiddleware e declarando securityChain")
	securityChain := middleware.SecurityHeadersMiddleware()

	// CORS — aplicado em TODAS as rotas (deve ser o middleware mais externo).
	// Origins permitidas: lista explícita (comparação exata) em
	// cfg.CORSAllowedOrigins, que já inclui as origens de dev local padrão
	// mais o que vier em CORS_ALLOWED_ORIGINS.
	vlog.Printf("routes.go", "NewMux", "chamando middleware.CORSMiddleware e declarando corsChain")
	corsChain := middleware.CORSMiddleware(cfg.CORSAllowedOrigins)
	return corsChain(securityChain(mux))
}
