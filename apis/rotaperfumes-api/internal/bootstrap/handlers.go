// Package bootstrap é a composition root da API (ARQ-01, Facade): o ÚNICO
// lugar que conhece config, *sql.DB, os adapters e os singletons por
// processo. DI manual, sem framework (docs/arquitetura-ddd.md §5.11).
package bootstrap

import (
	"database/sql"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/internal/catalogohttp"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/config"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

// Handlers agrupa os handlers HTTP. Os contextos ainda não migrados
// (ARQ-03..06, strangler fig) usam os handlers LEGADOS; o campo de cada
// contexto troca de tipo no lote dele (Catálogo: ARQ-02).
type Handlers struct {
	Auth           *handlers.AuthHandler
	Usuario        *handlers.UsuarioHandler
	Dashboard      *handlers.DashboardHandler
	SenhaHistorico *handlers.SenhaHistoricoHandler
	Vendedor       *handlers.VendedorHandler
	Cliente        *handlers.ClienteHandler
	Produto        *catalogohttp.ProdutoHandler
	Pedido         *handlers.PedidoHandler
	Pagamento      *handlers.PagamentoHandler
	Oportunidade   *handlers.OportunidadeHandler
	Visita         *handlers.VisitaHandler
	Estoque        *catalogohttp.EstoqueHandler
}

// NovosHandlers cria os handlers sobre o pool. Cada handler é criado uma vez
// por processo; os rate limiters e o Turnstile do AuthHandler nascem aqui
// dentro do construtor legado e passam a singletons explícitos no ARQ-03.
func NovosHandlers(pool *sql.DB, cfg *config.Config, email sharedsvc.EmailService) Handlers {
	produto, estoque := NovoCatalogoHTTP(pool)
	return Handlers{
		Auth:           handlers.NewAuthHandler(pool, cfg),
		Usuario:        handlers.NewUsuarioHandler(pool, cfg, email),
		Dashboard:      handlers.NewDashboardHandler(pool, cfg),
		SenhaHistorico: handlers.NewSenhaHistoricoHandler(pool),
		Vendedor:       handlers.NewVendedorHandler(pool, cfg),
		Cliente:        handlers.NewClienteHandler(pool, cfg),
		Produto:        produto,
		Pedido:         handlers.NewPedidoHandler(pool, cfg),
		Pagamento:      handlers.NewPagamentoHandler(pool, cfg),
		Oportunidade:   handlers.NewOportunidadeHandler(pool, cfg),
		Visita:         handlers.NewVisitaHandler(pool, cfg),
		Estoque:        estoque,
	}
}

// NovoCatalogoHTTP liga o contexto Catálogo (ARQ-02): adapters MySQL sobre o
// pool → casos de uso → handlers HTTP.
func NovoCatalogoHTTP(pool *sql.DB) (*catalogohttp.ProdutoHandler, *catalogohttp.EstoqueHandler) {
	casos := catalogoapp.Novo(catalogomysql.NovoProdutos(pool), catalogomysql.NovoEstoques(pool))
	return catalogohttp.NovoProdutoHandler(casos), catalogohttp.NovoEstoqueHandler(casos)
}
