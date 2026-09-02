package repository

import (
	"context"
	"backend/erp/internal/models"
)

// ProdutoRepository interface
type ProdutoRepository interface {
	ListarTodos(ctx context.Context) ([]models.Produto, error)
	BuscarPorSKU(ctx context.Context, sku string) (*models.Produto, error)
	Criar(ctx context.Context, p models.Produto) error
	Atualizar(ctx context.Context, sku string, p models.Produto) error
}

// PedidoRepository interface
type PedidoRepository interface {
	ListarTodos(ctx context.Context) ([]models.Pedido, error)
	BuscarPorID(ctx context.Context, id int64) (*models.Pedido, error)
	Criar(ctx context.Context, p models.Pedido, itens []models.ItemPedido) (int64, error)
}

// ItemPedidoRepository interface
type ItemPedidoRepository interface {
	ListarPorPedido(ctx context.Context, pedidoID int64) ([]models.ItemPedido, error)
}

// PagamentoRepository interface
type PagamentoRepository interface {
	ListarTodos(ctx context.Context) ([]models.Pagamento, error)
	BuscarPorID(ctx context.Context, id int64) (*models.Pagamento, error)
	Criar(ctx context.Context, p models.Pagamento) (int64, error)
	Atualizar(ctx context.Context, id int64, p models.Pagamento) error
}

// EstoqueRepository interface
type EstoqueRepository interface {
	ListarPorSKU(ctx context.Context, sku string) ([]models.Estoque, error)
	UltimoSnapshot(ctx context.Context, sku string) (*models.Estoque, error)
	ListarEmRuptura(ctx context.Context) ([]models.DashboardRuptura, error)
}

// UsuarioRepository interface
type UsuarioRepository interface {
	BuscarPorLogin(ctx context.Context, login string) (*models.Usuario, error)
}

// DashboardRepository interface
type DashboardRepository interface {
	ObterTotaisPorFormaPagamento(ctx context.Context) ([]models.DashboardFinanceiroTotais, error)
}
