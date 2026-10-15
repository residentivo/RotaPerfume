package repositories_test

// ARQ-05: os repositórios de pedido e pagamento migraram para
// vendas/vendasmysql (SQL idêntico) e as escritas de estoque que restavam
// no legado (baixa por faturamento e upsert absoluto) para
// catalogo/catalogomysql. Os testes mantêm os nomes e exercitam os adapters
// novos por estes repassadores finos, que só traduzem a assinatura antiga
// (db por parâmetro, page/limit e order_by em string, vendedorID int64 de
// escopo). A transação que o PedidoRepository legado abria subiu para o UoW
// da aplicação (§5.2, C9): os métodos *ComItens rodam no db.Transactor com
// o MESMO bind do bootstrap (pedidos + estoque no *sql.Tx) e o roteiro do
// PUT é o vendasapp.TxPedido.Atualizar (a regra mora lá e no domínio, não
// aqui). Erros não são traduzidos: as asserções usam os sentinelas de
// shared/vendas. Sai no ARQ-07.

import (
	"context"
	"database/sql"
	"time"

	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

// Filtros legados (o VendedorID de pedido era filtro de negócio; o de
// pagamento, o escopo).
type (
	PedidoFiltro struct {
		Status, Canal         string
		ClienteID, VendedorID int64
		DataInicio, DataFim   string
		Q, OrderBy, OrderDir  string
	}
	PagamentoFiltro struct {
		StatusPagamento, FormaPagamento string
		PedidoID                        int64
		VencimentoDe, VencimentoAte     string
		VendedorID                      int64
		OrderBy, OrderDir               string
	}
)

// txPedido é o UoW do pedido montado como no bootstrap.
func txPedido(pool *sql.DB) *db.Transactor[vendasapp.TxPedido] {
	return db.NewTransactor(pool, func(q db.Querier) vendasapp.TxPedido {
		return vendasapp.TxPedido{Pedidos: vendasmysql.NovoPedidos(q), Estoque: catalogomysql.NovoEstoques(q)}
	})
}

// ---------------------------------------------------------------------------
// pedidos
// ---------------------------------------------------------------------------

type pedidoRepo struct{}

// NewPedidoRepository substitui repositories.NewPedidoRepository.
func NewPedidoRepository() pedidoRepo { return pedidoRepo{} }

func (pedidoRepo) List(ctx context.Context, pool *sql.DB, page, limit int, f PedidoFiltro) ([]vendas.Pedido, int, error) {
	filtro := vendas.FiltroPedido{Status: f.Status, Canal: f.Canal, ClienteID: f.ClienteID, VendedorID: f.VendedorID,
		DataInicio: f.DataInicio, DataFim: f.DataFim, Busca: f.Q,
		Ordenacao: kernel.ParseOrdenacao(f.OrderBy, f.OrderDir, vendas.CamposOrdenacaoPedido(), vendas.OrdenacaoPedidoPadrao())}
	r, err := vendasmysql.NovoPedidos(pool).Listar(ctx, escopoLegado(0), filtro, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (pedidoRepo) GetByID(ctx context.Context, pool *sql.DB, id int64) (*vendas.Pedido, error) {
	return vendasmysql.NovoPedidos(pool).PorID(ctx, id)
}

func (pedidoRepo) ListItensByPedidoID(ctx context.Context, pool *sql.DB, id int64) ([]vendas.ItemPedido, error) {
	return vendasmysql.NovoPedidos(pool).Itens(ctx, id)
}

func (pedidoRepo) ExistsByID(ctx context.Context, pool *sql.DB, id int64) (bool, error) {
	return vendasmysql.NovoPedidos(pool).Existe(ctx, id)
}

// CreateComItens: cabeçalho e itens na tx do UoW (os ids voltam no próprio
// slice itens, como no legado).
func (pedidoRepo) CreateComItens(ctx context.Context, pool *sql.DB, p *vendas.Pedido, itens []vendas.ItemPedido) error {
	p.Itens = itens
	return txPedido(pool).WithTx(ctx, func(r vendasapp.TxPedido) error { return r.Pedidos.Inserir(ctx, p) })
}

// UpdateComItens: o roteiro do PUT (vendasapp.TxPedido.Atualizar) na tx.
func (pedidoRepo) UpdateComItens(ctx context.Context, pool *sql.DB, id int64, p *vendas.Pedido, itens []vendas.ItemPedido) error {
	p.Itens = itens
	return txPedido(pool).WithTx(ctx, func(r vendasapp.TxPedido) error { return r.Atualizar(ctx, id, p, time.Now()) })
}

func (pedidoRepo) DeleteComItens(ctx context.Context, pool *sql.DB, id int64) error {
	return txPedido(pool).WithTx(ctx, func(r vendasapp.TxPedido) error { return r.Pedidos.Excluir(ctx, id) })
}

// ---------------------------------------------------------------------------
// pagamentos
// ---------------------------------------------------------------------------

type pagamentoRepo struct{}

// NewPagamentoRepository substitui repositories.NewPagamentoRepository.
func NewPagamentoRepository() pagamentoRepo { return pagamentoRepo{} }

func (pagamentoRepo) List(ctx context.Context, pool *sql.DB, page, limit int, f PagamentoFiltro) ([]vendas.Pagamento, int, error) {
	filtro := vendas.FiltroPagamento{StatusPagamento: f.StatusPagamento, FormaPagamento: f.FormaPagamento,
		PedidoID: f.PedidoID, VencimentoDe: f.VencimentoDe, VencimentoAte: f.VencimentoAte,
		Ordenacao: kernel.ParseOrdenacao(f.OrderBy, f.OrderDir, vendas.CamposOrdenacaoPagamento(), vendas.OrdenacaoPagamentoPadrao())}
	r, err := vendasmysql.NovoPagamentos(pool).Listar(ctx, escopoLegado(f.VendedorID), filtro, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (pagamentoRepo) GetByID(ctx context.Context, pool *sql.DB, id int64) (*vendas.Pagamento, error) {
	return vendasmysql.NovoPagamentos(pool).PorID(ctx, id)
}

func (pagamentoRepo) Create(ctx context.Context, pool *sql.DB, p *vendas.Pagamento) error {
	return vendasmysql.NovoPagamentos(pool).Inserir(ctx, p)
}

func (pagamentoRepo) Update(ctx context.Context, pool *sql.DB, id int64, p *vendas.Pagamento) error {
	return vendasmysql.NovoPagamentos(pool).Atualizar(ctx, id, p)
}

func (pagamentoRepo) ExistsByPedidoID(ctx context.Context, pool *sql.DB, id int64) (bool, error) {
	return vendasmysql.NovoPagamentos(pool).ExistePorPedido(ctx, id)
}

func (pagamentoRepo) Delete(ctx context.Context, pool *sql.DB, id int64) error {
	return vendasmysql.NovoPagamentos(pool).Excluir(ctx, id)
}

// ---------------------------------------------------------------------------
// estoque (as escritas que eram do repositories.EstoqueRepository encolhido)
// ---------------------------------------------------------------------------

type estoqueRepo struct{}

// NewEstoqueRepository substitui repositories.NewEstoqueRepository.
func NewEstoqueRepository() estoqueRepo { return estoqueRepo{} }

func (estoqueRepo) UpsertPorDataSku(ctx context.Context, q db.Querier, sku string, data time.Time, saldo int, ruptura bool) error {
	return catalogomysql.NovoEstoques(q).UpsertPorDataSku(ctx, sku, data, saldo, ruptura)
}

// AjustarSaldoPorFaturamento recebe a tx (como o legado): o adapter ligado
// a ela é o mesmo que o bootstrap liga ao UoW do pedido.
func (estoqueRepo) AjustarSaldoPorFaturamento(ctx context.Context, tx *sql.Tx, sku string, data time.Time, delta int) error {
	return catalogomysql.NovoEstoques(tx).AjustarSaldoPorFaturamento(ctx, sku, data, delta)
}
