package bootstrap

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
)

// HandlerHTTP monta a cadeia CORS → SecurityHeaders → mux → Exigir(policy) →
// Trace → handler (substitui o routes.NewMux). O Logging de acesso fica de
// fora: é aplicado por Montar, como o withLogging antigo ficava no main.
//
// checker (obrigatório) consulta ativo/role/corte no banco a cada request
// protegido (SEC-06); nil dá panic na montagem.
func HandlerHTTP(cfg *config.Config, checker httpx.ConsultaStatus, h Handlers) http.Handler {
	return httpx.Chain(NovoRouter(cfg, checker, h).Mux(),
		httpx.CORS(cfg.CORSAllowedOrigins), // OPTIONS → 204 antes do mux (O3)
		httpx.SecurityHeaders,
	)
}

// NovoRouter registra as 61 rotas da API com método e política obrigatórios.
// Rotas() do router devolvido é a fonte da matriz ARQ-00.
func NovoRouter(cfg *config.Config, checker httpx.ConsultaStatus, h Handlers) *httpx.Router {
	r := httpx.NovoRouter(httpx.NovoAutenticador(identidadecrypto.NovoJWT(cfg), checker))
	for _, rt := range rotas(h) {
		r.Handle(rt)
	}
	return r
}

const (
	get   = http.MethodGet
	post  = http.MethodPost
	put   = http.MethodPut
	patch = http.MethodPatch
	del   = http.MethodDelete

	publica = httpx.Public
	autent  = httpx.Authenticated
	admin   = httpx.AdminOnly
)

// rotas é a tabela declarativa. Os 13 padrões que antes não fixavam método
// passam ao método documentado (exceção X1, §7.3).
func rotas(h Handlers) []httpx.Rota {
	const (
		fAuth   = "auth_handler.go"
		fUser   = "usuario_handler.go"
		fSenha  = "senha_historico_handler.go"
		fDash   = "dashboard_handler.go"
		fVend   = "vendedor_handler.go"
		fCli    = "cliente_handler.go"
		fProd   = "produto_handler.go"
		fPed    = "pedido_handler.go"
		fPag    = "pagamento_handler.go"
		fOport  = "oportunidade_handler.go"
		fVisita = "visita_handler.go"
		fEstq   = "estoque_handler.go"
	)
	return []httpx.Rota{
		// Autenticação (login/refresh/logout públicos; refresh e logout leem o
		// cookie de refresh no próprio handler).
		{Metodo: post, Padrao: "/api/auth/login", Policy: publica, Arquivo: fAuth, Funcao: "AuthHandler.Login", Handler: h.Auth.Login},
		{Metodo: post, Padrao: "/api/auth/refresh", Policy: publica, Arquivo: fAuth, Funcao: "AuthHandler.Refresh", Handler: h.Auth.Refresh},
		{Metodo: post, Padrao: "/api/auth/logout", Policy: publica, Arquivo: fAuth, Funcao: "AuthHandler.Logout", Handler: h.Auth.Logout},
		{Metodo: post, Padrao: "/api/auth/reset-password", Policy: autent, Arquivo: fAuth, Funcao: "AuthHandler.ResetPassword", Handler: h.Auth.ResetPassword},
		{Metodo: get, Padrao: "/api/auth/me", Policy: autent, Arquivo: fAuth, Funcao: "AuthHandler.Me", Handler: h.Auth.Me},

		// Usuários (admin).
		{Metodo: get, Padrao: "/api/usuarios", Policy: admin, Arquivo: fUser, Funcao: "UsuarioHandler.ListUsuarios", Handler: h.Usuario.ListUsuarios},
		{Metodo: post, Padrao: "/api/usuarios", Policy: admin, Arquivo: fUser, Funcao: "UsuarioHandler.CreateUsuario", Handler: h.Usuario.CreateUsuario},
		{Metodo: put, Padrao: "/api/usuarios/{id}", Policy: admin, Arquivo: fUser, Funcao: "UsuarioHandler.UpdateUsuario", Handler: h.Usuario.UpdateUsuario},
		{Metodo: patch, Padrao: "/api/usuarios/{id}/inativar", Policy: admin, Arquivo: fUser, Funcao: "UsuarioHandler.ToggleAtivoUsuario", Handler: h.Usuario.ToggleAtivoUsuario},
		{Metodo: post, Padrao: "/api/admin/reset-password", Policy: admin, Arquivo: fUser, Funcao: "UsuarioHandler.AdminResetPassword", Handler: h.Usuario.AdminResetPassword},

		// Histórico de senhas (admin). {user_id...}: id vazio ainda chega ao
		// handler (400 "obrigatório"); "/2/extra" vira 400 (X1, R-f).
		{Metodo: get, Padrao: "/api/senha-historico", Policy: admin, Arquivo: fSenha, Funcao: "SenhaHistoricoHandler.ListarTodos", Handler: h.SenhaHistorico.ListarTodos},
		{Metodo: get, Padrao: "/api/senha-historico/{user_id...}", Policy: admin, Arquivo: fSenha, Funcao: "SenhaHistoricoHandler.ListarPorUsuario", Handler: h.SenhaHistorico.ListarPorUsuario},

		// Dashboard (analyticshttp, ARQ-06; escopo por vendedor, D-01).
		{Metodo: get, Padrao: "/api/dashboard/metrics", Policy: autent, Arquivo: fDash, Funcao: "DashboardHandler.GetMetrics", Handler: h.Dashboard.GetMetrics},
		{Metodo: get, Padrao: "/api/dashboard/vendas", Policy: autent, Arquivo: fDash, Funcao: "DashboardHandler.GetVendas", Handler: h.Dashboard.GetVendas},
		{Metodo: get, Padrao: "/api/dashboard/vendedores", Policy: autent, Arquivo: fDash, Funcao: "DashboardHandler.GetVendedores", Handler: h.Dashboard.GetVendedores},
		{Metodo: get, Padrao: "/api/dashboard/clientes", Policy: autent, Arquivo: fDash, Funcao: "DashboardHandler.GetClientes", Handler: h.Dashboard.GetClientes},
		{Metodo: get, Padrao: "/api/dashboard/funil", Policy: autent, Arquivo: fDash, Funcao: "DashboardHandler.GetFunil", Handler: h.Dashboard.GetFunil},

		// Vendedores (leitura comum; gestão admin).
		{Metodo: get, Padrao: "/api/vendedores", Policy: autent, Arquivo: fVend, Funcao: "VendedorHandler.ListVendedores", Handler: h.Vendedor.ListVendedores},
		{Metodo: post, Padrao: "/api/vendedores", Policy: admin, Arquivo: fVend, Funcao: "VendedorHandler.CreateVendedor", Handler: h.Vendedor.CreateVendedor},
		{Metodo: get, Padrao: "/api/vendedores/{id}", Policy: admin, Arquivo: fVend, Funcao: "VendedorHandler.GetVendedor", Handler: h.Vendedor.GetVendedor},
		{Metodo: put, Padrao: "/api/vendedores/{id}", Policy: admin, Arquivo: fVend, Funcao: "VendedorHandler.UpdateVendedor", Handler: h.Vendedor.UpdateVendedor},
		{Metodo: del, Padrao: "/api/vendedores/{id}", Policy: admin, Arquivo: fVend, Funcao: "VendedorHandler.DeleteVendedor", Handler: h.Vendedor.DeleteVendedor},
		{Metodo: post, Padrao: "/api/vendedores/{id}/reativar", Policy: admin, Arquivo: fVend, Funcao: "VendedorHandler.ReativarVendedor", Handler: h.Vendedor.ReativarVendedor},
		{Metodo: post, Padrao: "/api/vendedores/{id}/clientes", Policy: admin, Arquivo: fVend, Funcao: "VendedorHandler.VincularCliente", Handler: h.Vendedor.VincularCliente},
		{Metodo: del, Padrao: "/api/vendedores/{id}/clientes/{clienteId}", Policy: admin, Arquivo: fVend, Funcao: "VendedorHandler.DesvincularCliente", Handler: h.Vendedor.DesvincularCliente},
		{Metodo: get, Padrao: "/api/vendedores/{id}/clientes", Policy: autent, Arquivo: fVend, Funcao: "VendedorHandler.ListClientesDoVendedor", Handler: h.Vendedor.ListClientesDoVendedor},

		// Clientes (acesso comum, escopo por carteira).
		{Metodo: get, Padrao: "/api/clientes", Policy: autent, Arquivo: fCli, Funcao: "ClienteHandler.ListClientes", Handler: h.Cliente.ListClientes},
		{Metodo: post, Padrao: "/api/clientes", Policy: autent, Arquivo: fCli, Funcao: "ClienteHandler.CreateCliente", Handler: h.Cliente.CreateCliente},
		{Metodo: get, Padrao: "/api/clientes/{id}", Policy: autent, Arquivo: fCli, Funcao: "ClienteHandler.GetCliente", Handler: h.Cliente.GetCliente},
		{Metodo: put, Padrao: "/api/clientes/{id}", Policy: autent, Arquivo: fCli, Funcao: "ClienteHandler.UpdateCliente", Handler: h.Cliente.UpdateCliente},
		{Metodo: patch, Padrao: "/api/clientes/{id}/inativar", Policy: autent, Arquivo: fCli, Funcao: "ClienteHandler.ToggleAtivoCliente", Handler: h.Cliente.ToggleAtivoCliente},

		// Produtos (catalogohttp, ARQ-02; leitura comum, escrita admin, C-01).
		{Metodo: get, Padrao: "/api/produtos", Policy: autent, Arquivo: fProd, Funcao: "ProdutoHandler.ListProdutos", Handler: h.Produto.ListProdutos},
		{Metodo: post, Padrao: "/api/produtos", Policy: admin, Arquivo: fProd, Funcao: "ProdutoHandler.CreateProduto", Handler: h.Produto.CreateProduto},
		{Metodo: get, Padrao: "/api/produtos/{id}", Policy: autent, Arquivo: fProd, Funcao: "ProdutoHandler.GetProduto", Handler: h.Produto.GetProduto},
		{Metodo: put, Padrao: "/api/produtos/{id}", Policy: admin, Arquivo: fProd, Funcao: "ProdutoHandler.UpdateProduto", Handler: h.Produto.UpdateProduto},
		{Metodo: patch, Padrao: "/api/produtos/{id}/inativar", Policy: admin, Arquivo: fProd, Funcao: "ProdutoHandler.ToggleAtivoProduto", Handler: h.Produto.ToggleAtivoProduto},

		// Pedidos (acesso comum, escopo por carteira).
		{Metodo: get, Padrao: "/api/pedidos", Policy: autent, Arquivo: fPed, Funcao: "PedidoHandler.ListPedidos", Handler: h.Pedido.ListPedidos},
		{Metodo: post, Padrao: "/api/pedidos", Policy: autent, Arquivo: fPed, Funcao: "PedidoHandler.CreatePedido", Handler: h.Pedido.CreatePedido},
		{Metodo: get, Padrao: "/api/pedidos/{id}", Policy: autent, Arquivo: fPed, Funcao: "PedidoHandler.GetPedido", Handler: h.Pedido.GetPedido},
		{Metodo: put, Padrao: "/api/pedidos/{id}", Policy: autent, Arquivo: fPed, Funcao: "PedidoHandler.UpdatePedido", Handler: h.Pedido.UpdatePedido},
		{Metodo: del, Padrao: "/api/pedidos/{id}", Policy: autent, Arquivo: fPed, Funcao: "PedidoHandler.DeletePedido", Handler: h.Pedido.DeletePedido},

		// Pagamentos (acesso comum, escopo herdado do pedido).
		{Metodo: get, Padrao: "/api/pagamentos", Policy: autent, Arquivo: fPag, Funcao: "PagamentoHandler.ListPagamentos", Handler: h.Pagamento.ListPagamentos},
		{Metodo: post, Padrao: "/api/pagamentos", Policy: autent, Arquivo: fPag, Funcao: "PagamentoHandler.CreatePagamento", Handler: h.Pagamento.CreatePagamento},
		{Metodo: get, Padrao: "/api/pagamentos/{id}", Policy: autent, Arquivo: fPag, Funcao: "PagamentoHandler.GetPagamento", Handler: h.Pagamento.GetPagamento},
		{Metodo: put, Padrao: "/api/pagamentos/{id}", Policy: autent, Arquivo: fPag, Funcao: "PagamentoHandler.UpdatePagamento", Handler: h.Pagamento.UpdatePagamento},
		{Metodo: del, Padrao: "/api/pagamentos/{id}", Policy: autent, Arquivo: fPag, Funcao: "PagamentoHandler.DeletePagamento", Handler: h.Pagamento.DeletePagamento},

		// Oportunidades (acesso comum, escopo por carteira).
		{Metodo: get, Padrao: "/api/oportunidades", Policy: autent, Arquivo: fOport, Funcao: "OportunidadeHandler.ListOportunidades", Handler: h.Oportunidade.ListOportunidades},
		{Metodo: post, Padrao: "/api/oportunidades", Policy: autent, Arquivo: fOport, Funcao: "OportunidadeHandler.CreateOportunidade", Handler: h.Oportunidade.CreateOportunidade},
		{Metodo: get, Padrao: "/api/oportunidades/{id}", Policy: autent, Arquivo: fOport, Funcao: "OportunidadeHandler.GetOportunidade", Handler: h.Oportunidade.GetOportunidade},
		{Metodo: put, Padrao: "/api/oportunidades/{id}", Policy: autent, Arquivo: fOport, Funcao: "OportunidadeHandler.UpdateOportunidade", Handler: h.Oportunidade.UpdateOportunidade},
		{Metodo: del, Padrao: "/api/oportunidades/{id}", Policy: autent, Arquivo: fOport, Funcao: "OportunidadeHandler.DeleteOportunidade", Handler: h.Oportunidade.DeleteOportunidade},

		// Visitas (acesso comum, escopo por carteira).
		{Metodo: get, Padrao: "/api/visitas", Policy: autent, Arquivo: fVisita, Funcao: "VisitaHandler.ListVisitas", Handler: h.Visita.ListVisitas},
		{Metodo: post, Padrao: "/api/visitas", Policy: autent, Arquivo: fVisita, Funcao: "VisitaHandler.CreateVisita", Handler: h.Visita.CreateVisita},
		{Metodo: get, Padrao: "/api/visitas/{id}", Policy: autent, Arquivo: fVisita, Funcao: "VisitaHandler.GetVisita", Handler: h.Visita.GetVisita},
		{Metodo: put, Padrao: "/api/visitas/{id}", Policy: autent, Arquivo: fVisita, Funcao: "VisitaHandler.UpdateVisita", Handler: h.Visita.UpdateVisita},
		{Metodo: del, Padrao: "/api/visitas/{id}", Policy: autent, Arquivo: fVisita, Funcao: "VisitaHandler.DeleteVisita", Handler: h.Visita.DeleteVisita},

		// Estoque (catalogohttp, ARQ-02; tudo admin, C-01).
		{Metodo: get, Padrao: "/api/estoque", Policy: admin, Arquivo: fEstq, Funcao: "EstoqueHandler.ListEstoque", Handler: h.Estoque.ListEstoque},
		{Metodo: post, Padrao: "/api/estoque", Policy: admin, Arquivo: fEstq, Funcao: "EstoqueHandler.CreateEstoque", Handler: h.Estoque.CreateEstoque},
		{Metodo: get, Padrao: "/api/estoque/{id}", Policy: admin, Arquivo: fEstq, Funcao: "EstoqueHandler.GetEstoque", Handler: h.Estoque.GetEstoque},
		{Metodo: put, Padrao: "/api/estoque/{id}", Policy: admin, Arquivo: fEstq, Funcao: "EstoqueHandler.UpdateEstoque", Handler: h.Estoque.UpdateEstoque},

		// Healthcheck (Caddy e smoke do Jenkins, GET; HEAD também casa).
		{Metodo: get, Padrao: "/health", Policy: publica, Arquivo: "server.go", Funcao: "Health", Handler: health},
	}
}

// health responde o healthcheck com o mesmo corpo de sempre.
func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"success":true,"data":{"status":"ok"}}`))
}
