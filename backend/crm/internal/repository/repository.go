package repository

import (
	"context"
	"backend/crm/internal/models"
)

// ClienteRepository define a interface para operações de clientes
type ClienteRepository interface {
	ListarTodos(ctx context.Context, filtro models.ClienteFiltro) ([]models.Cliente, error)
	BuscarPorID(ctx context.Context, id int64) (*models.Cliente, error)
	Criar(ctx context.Context, input models.ClienteInput) (*models.Cliente, error)
	Atualizar(ctx context.Context, id int64, input models.ClienteInput) (*models.Cliente, error)
}

// VendedorRepository define a interface para operações de vendedores
type VendedorRepository interface {
	ListarTodos(ctx context.Context, filtro models.VendedorFiltro) ([]models.Vendedor, error)
	BuscarPorID(ctx context.Context, id int64) (*models.Vendedor, error)
}

// CarteiraRepository define a interface para operações de carteira
type CarteiraRepository interface {
	ListarPorVendedor(ctx context.Context, vendedorID int64) ([]models.Carteira, error)
	ListarTodos(ctx context.Context) ([]models.Carteira, error)
	Criar(ctx context.Context, input models.CarteiraInput) (*models.Carteira, error)
}

// VisitaRepository define a interface para operações de visitas
type VisitaRepository interface {
	ListarPorVendedor(ctx context.Context, vendedorID int64) ([]models.Visita, error)
	ListarPorCliente(ctx context.Context, clienteID int64) ([]models.Visita, error)
	Criar(ctx context.Context, input models.VisitaInput) (*models.Visita, error)
}

// OportunidadeRepository define a interface para operações de oportunidades
type OportunidadeRepository interface {
	ListarTodos(ctx context.Context, filtro models.OportunidadeFiltro) ([]models.Oportunidade, error)
	BuscarPorID(ctx context.Context, id int64) (*models.Oportunidade, error)
	Criar(ctx context.Context, input models.OportunidadeInput) (*models.Oportunidade, error)
	Atualizar(ctx context.Context, id int64, input models.OportunidadeInput) (*models.Oportunidade, error)
}

// UsuarioRepository define a interface para operações de usuários
type UsuarioRepository interface {
	BuscarPorLogin(ctx context.Context, login string) (*models.Usuario, error)
}

// DashboardRepository define a interface para operações de dashboard
type DashboardRepository interface {
	RankingVendedores(ctx context.Context) ([]models.RankingVendedor, error)
	DistribuicaoCarteira(ctx context.Context) ([]models.DistribuicaoCarteira, error)
}
