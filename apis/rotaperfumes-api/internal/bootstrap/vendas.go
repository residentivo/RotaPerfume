package bootstrap

import (
	"database/sql"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/vendashttp"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

// NovoVendas liga o contexto Vendas (ARQ-05) sobre o pool: adapters MySQL,
// o escopo de carteira de Comercial (o AutorizadorEscopo, a mesma regra de
// todo o sistema) e o UoW do pedido. Na tx do pedido, o cabeçalho, os itens
// e a baixa de estoque do faturamento (porta para Catálogo) usam o MESMO
// *sql.Tx (§5.2): todos os adapters do bind recebem o q da tx, nunca o pool.
func NovoVendas(pool *sql.DB, escopo vendasapp.EscopoCarteira) *vendasapp.Casos {
	return vendasapp.Novo(vendasapp.Deps{
		Pedidos:    vendasmysql.NovoPedidos(pool),
		Pagamentos: vendasmysql.NovoPagamentos(pool),
		Escopo:     escopo,
		TxPedido: db.NewTransactor(pool, func(q db.Querier) vendasapp.TxPedido {
			return vendasapp.TxPedido{Pedidos: vendasmysql.NovoPedidos(q), Estoque: catalogomysql.NovoEstoques(q)}
		}),
		Relogio: time.Now,
	})
}

// VendasHTTP são os handlers do contexto.
type VendasHTTP struct {
	Pedido    *vendashttp.PedidoHandler
	Pagamento *vendashttp.PagamentoHandler
}

// NovoVendasHTTP monta os handlers sobre os casos de uso.
func NovoVendasHTTP(c *vendasapp.Casos) VendasHTTP {
	return VendasHTTP{Pedido: vendashttp.NovoPedidoHandler(c), Pagamento: vendashttp.NovoPagamentoHandler(c)}
}
