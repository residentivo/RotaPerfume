// Package bootstrap é a composition root da API (ARQ-01, Facade): o ÚNICO
// lugar que conhece config, *sql.DB, os adapters e os singletons por
// processo. DI manual, sem framework (docs/arquitetura-ddd.md §5.11).
package bootstrap

import (
	"database/sql"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/internal/catalogohttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/comercialhttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/identidadehttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/vendashttp"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

// Handlers agrupa os handlers HTTP. O contexto ainda não migrado
// (Analytics, ARQ-06, strangler fig) usa o handler LEGADO; o campo de cada
// contexto troca de tipo no lote dele (Catálogo: ARQ-02; Identidade: ARQ-03;
// Comercial: ARQ-04; Vendas: ARQ-05).
type Handlers struct {
	Auth           *identidadehttp.AuthHandler
	Usuario        *identidadehttp.UsuarioHandler
	Dashboard      *handlers.DashboardHandler
	SenhaHistorico *identidadehttp.SenhaHistoricoHandler
	Vendedor       *comercialhttp.VendedorHandler
	Cliente        *comercialhttp.ClienteHandler
	Produto        *catalogohttp.ProdutoHandler
	Pedido         *vendashttp.PedidoHandler
	Pagamento      *vendashttp.PagamentoHandler
	Oportunidade   *comercialhttp.OportunidadeHandler
	Visita         *comercialhttp.VisitaHandler
	Estoque        *catalogohttp.EstoqueHandler
}

// NovosHandlers cria os handlers sobre o pool, com a Identidade padrão
// (Turnstile real, sem assinante de evento) e o e-mail informado.
func NovosHandlers(pool *sql.DB, cfg *config.Config, email sharedsvc.EmailService) Handlers {
	return NovosHandlersCom(pool, cfg, NovaIdentidade(pool, cfg, extPadrao(cfg, email)))
}

// NovosHandlersCom cria os handlers com os casos de uso de Identidade já
// montados (produção: Montar; testes: captcha e publicador falsos).
func NovosHandlersCom(pool *sql.DB, cfg *config.Config, identidade *identidadeapp.Casos) Handlers {
	produto, estoque := NovoCatalogoHTTP(pool)
	comercialCasos := NovoComercial(pool)
	com := NovoComercialHTTP(comercialCasos)
	escopo := comercialCasos.Escopo() // a mesma regra de escopo para Vendas e para o Dashboard legado
	vendas := NovoVendasHTTP(NovoVendas(pool, escopo))
	ident := NovoIdentidadeHTTP(identidade, cfg)
	return Handlers{
		Auth:           ident.Auth,
		Usuario:        ident.Usuario,
		Dashboard:      handlers.NewDashboardHandler(pool, cfg, escopo),
		SenhaHistorico: ident.SenhaHistorico,
		Vendedor:       com.Vendedor,
		Cliente:        com.Cliente,
		Produto:        produto,
		Pedido:         vendas.Pedido,
		Pagamento:      vendas.Pagamento,
		Oportunidade:   com.Oportunidade,
		Visita:         com.Visita,
		Estoque:        estoque,
	}
}

// NovoCatalogoHTTP liga o contexto Catálogo (ARQ-02): adapters MySQL sobre o
// pool → casos de uso → handlers HTTP.
func NovoCatalogoHTTP(pool *sql.DB) (*catalogohttp.ProdutoHandler, *catalogohttp.EstoqueHandler) {
	casos := catalogoapp.Novo(catalogomysql.NovoProdutos(pool), catalogomysql.NovoEstoques(pool))
	return catalogohttp.NovoProdutoHandler(casos), catalogohttp.NovoEstoqueHandler(casos)
}
