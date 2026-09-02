package services

import (
	"context"
	"errors"
	"time"

	"backend/erp/internal/models"
	"backend/erp/internal/repository/mysql"

	"golang.org/x/crypto/bcrypt"
)

// ===================== PRODUTO SERVICES =====================

type ProdutoService struct {
	repo *mysql.ProdutoRepository
}

func NewProdutoService() *ProdutoService {
	return &ProdutoService{repo: mysql.NewProdutoRepository()}
}

func (s *ProdutoService) ListarTodos(ctx context.Context) ([]models.Produto, error) {
	return s.repo.ListarTodos(ctx)
}

func (s *ProdutoService) BuscarPorSKU(ctx context.Context, sku string) (*models.Produto, error) {
	return s.repo.BuscarPorSKU(ctx, sku)
}

func (s *ProdutoService) Criar(ctx context.Context, p models.Produto) error {
	if p.SKU == "" {
		return errors.New("SKU é obrigatório")
	}
	return s.repo.Criar(ctx, p)
}

func (s *ProdutoService) Atualizar(ctx context.Context, sku string, p models.Produto) error {
	return s.repo.Atualizar(ctx, sku, p)
}

// ===================== PEDIDO SERVICES =====================

type PedidoService struct {
	repo *mysql.PedidoRepository
}

func NewPedidoService() *PedidoService {
	return &PedidoService{repo: mysql.NewPedidoRepository()}
}

func (s *PedidoService) ListarTodos(ctx context.Context) ([]models.Pedido, error) {
	return s.repo.ListarTodos(ctx)
}

func (s *PedidoService) BuscarPorID(ctx context.Context, id int64) (*models.Pedido, error) {
	return s.repo.BuscarPorID(ctx, id)
}

func (s *PedidoService) Criar(ctx context.Context, req models.PedidoInput) (int64, error) {
	if req.ClienteID == 0 {
		return 0, errors.New("cliente_id é obrigatório")
	}
	if req.VendedorID == 0 {
		return 0, errors.New("vendedor_id é obrigatório")
	}
	if len(req.Itens) == 0 {
		return 0, errors.New("pedido deve ter ao menos um item")
	}

	status := req.Status
	if status == "" {
		status = "pendente"
	}
	canal := req.Canal
	if canal == "" {
		canal = "direto"
	}

	p := models.Pedido{
		ClienteID:  req.ClienteID,
		VendedorID: req.VendedorID,
		DataPedido: time.Now(),
		Canal:      canal,
		Status:     status,
	}
	return s.repo.Criar(ctx, p, req.Itens)
}

// ===================== ITEM PEDIDO SERVICES =====================

type ItemPedidoService struct {
	repo *mysql.ItemPedidoRepository
}

func NewItemPedidoService() *ItemPedidoService {
	return &ItemPedidoService{repo: mysql.NewItemPedidoRepository()}
}

func (s *ItemPedidoService) ListarPorPedido(ctx context.Context, pedidoID int64) ([]models.ItemPedido, error) {
	return s.repo.ListarPorPedido(ctx, pedidoID)
}

// ===================== PAGAMENTO SERVICES =====================

type PagamentoService struct {
	repo *mysql.PagamentoRepository
}

func NewPagamentoService() *PagamentoService {
	return &PagamentoService{repo: mysql.NewPagamentoRepository()}
}

func (s *PagamentoService) ListarTodos(ctx context.Context) ([]models.Pagamento, error) {
	return s.repo.ListarTodos(ctx)
}

func (s *PagamentoService) BuscarPorID(ctx context.Context, id int64) (*models.Pagamento, error) {
	return s.repo.BuscarPorID(ctx, id)
}

func (s *PagamentoService) Criar(ctx context.Context, p models.Pagamento) (int64, error) {
	if p.PedidoID == 0 {
		return 0, errors.New("pedido_id é obrigatório")
	}
	return s.repo.Criar(ctx, p)
}

func (s *PagamentoService) Atualizar(ctx context.Context, id int64, p models.Pagamento) error {
	return s.repo.Atualizar(ctx, id, p)
}

// ===================== ESTOQUE SERVICES =====================

type EstoqueService struct {
	repo *mysql.EstoqueRepository
}

func NewEstoqueService() *EstoqueService {
	return &EstoqueService{repo: mysql.NewEstoqueRepository()}
}

func (s *EstoqueService) ListarPorSKU(ctx context.Context, sku string) ([]models.Estoque, error) {
	return s.repo.ListarPorSKU(ctx, sku)
}

func (s *EstoqueService) UltimoSnapshot(ctx context.Context, sku string) (*models.Estoque, error) {
	return s.repo.UltimoSnapshot(ctx, sku)
}

func (s *EstoqueService) ListarEmRuptura(ctx context.Context) ([]models.DashboardRuptura, error) {
	return s.repo.ListarEmRuptura(ctx)
}

// ===================== AUTH SERVICES =====================

type AuthService struct {
	repo *mysql.UsuarioRepository
}

func NewAuthService() *AuthService {
	return &AuthService{repo: mysql.NewUsuarioRepository()}
}

func (s *AuthService) Login(ctx context.Context, login, senha string) (*models.Usuario, error) {
	if login == "" || senha == "" {
		return nil, errors.New("login e senha são obrigatórios")
	}

	user, err := s.repo.BuscarPorLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("usuário ou senha incorretos")
	}

	if user.Ativo != "S" {
		return nil, errors.New("usuário inativo")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.SenhaHash), []byte(senha))
	if err != nil {
		return nil, errors.New("usuário ou senha incorretos")
	}

	return user, nil
}

// ===================== DASHBOARD SERVICES =====================

type DashboardService struct {
	repo *mysql.DashboardRepository
}

func NewDashboardService() *DashboardService {
	return &DashboardService{repo: mysql.NewDashboardRepository()}
}

func (s *DashboardService) ObterTotaisPorFormaPagamento(ctx context.Context) ([]models.DashboardFinanceiroTotais, error) {
	return s.repo.ObterTotaisPorFormaPagamento(ctx)
}

func (s *DashboardService) ListarEmRuptura(ctx context.Context) ([]models.DashboardRuptura, error) {
	estoqueRepo := mysql.NewEstoqueRepository()
	return estoqueRepo.ListarEmRuptura(ctx)
}
