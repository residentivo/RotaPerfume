// Package bootstrap é a composition root da API (ARQ-01, Facade): o ÚNICO
// lugar que conhece config, *sql.DB, os adapters e os singletons por
// processo. DI manual, sem framework (docs/arquitetura-ddd.md §5.11).
package bootstrap

import (
	"database/sql"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/shared/config"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

// Handlers agrupa os handlers HTTP. Até cada contexto ser migrado
// (ARQ-02..06, strangler fig), são os handlers LEGADOS; o campo de cada
// contexto troca de tipo no lote dele.
type Handlers struct {
	Auth           *handlers.AuthHandler
	Usuario        *handlers.UsuarioHandler
	Dashboard      *handlers.DashboardHandler
	SenhaHistorico *handlers.SenhaHistoricoHandler
	Vendedor       *handlers.VendedorHandler
	Cliente        *handlers.ClienteHandler
	Produto        *handlers.ProdutoHandler
	Pedido         *handlers.PedidoHandler
	Pagamento      *handlers.PagamentoHandler
	Oportunidade   *handlers.OportunidadeHandler
	Visita         *handlers.VisitaHandler
	Estoque        *handlers.EstoqueHandler
}

// NovosHandlers cria os handlers sobre o pool. Cada handler é criado uma vez
// por processo; os rate limiters e o Turnstile do AuthHandler nascem aqui
// dentro do construtor legado e passam a singletons explícitos no ARQ-03.
func NovosHandlers(pool *sql.DB, cfg *config.Config, email sharedsvc.EmailService) Handlers {
	return Handlers{
		Auth:           handlers.NewAuthHandler(pool, cfg),
		Usuario:        handlers.NewUsuarioHandler(pool, cfg, email),
		Dashboard:      handlers.NewDashboardHandler(pool, cfg),
		SenhaHistorico: handlers.NewSenhaHistoricoHandler(pool),
		Vendedor:       handlers.NewVendedorHandler(pool, cfg),
		Cliente:        handlers.NewClienteHandler(pool, cfg),
		Produto:        handlers.NewProdutoHandler(pool, cfg),
		Pedido:         handlers.NewPedidoHandler(pool, cfg),
		Pagamento:      handlers.NewPagamentoHandler(pool, cfg),
		Oportunidade:   handlers.NewOportunidadeHandler(pool, cfg),
		Visita:         handlers.NewVisitaHandler(pool, cfg),
		Estoque:        handlers.NewEstoqueHandler(pool, cfg),
	}
}
