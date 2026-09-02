package services

import (
	"context"
	"errors"
	"time"

	"backend/crm/internal/models"
	"backend/crm/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// ClienteService contém a lógica de negócio para clientes
type ClienteService struct {
	repo repository.ClienteRepository
}

// NewClienteService cria um novo serviço de clientes
func NewClienteService(repo repository.ClienteRepository) *ClienteService {
	return &ClienteService{repo: repo}
}

// ListarTodos retorna clientes com filtros
func (s *ClienteService) ListarTodos(ctx context.Context, filtro models.ClienteFiltro) ([]models.Cliente, error) {
	return s.repo.ListarTodos(ctx, filtro)
}

// BuscarPorID retorna um cliente pelo ID
func (s *ClienteService) BuscarPorID(ctx context.Context, id int64) (*models.Cliente, error) {
	cliente, err := s.repo.BuscarPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cliente == nil {
		return nil, errors.New("cliente não encontrado")
	}
	return cliente, nil
}

// Criar cria um novo cliente
func (s *ClienteService) Criar(ctx context.Context, input models.ClienteInput) (*models.Cliente, error) {
	// Validações
	if input.CNPJ == "" {
		return nil, errors.New("CNPJ é obrigatório")
	}
	if input.RazaoSocial == "" {
		return nil, errors.New("razão social é obrigatória")
	}
	return s.repo.Criar(ctx, input)
}

// Atualizar atualiza um cliente existente
func (s *ClienteService) Atualizar(ctx context.Context, id int64, input models.ClienteInput) (*models.Cliente, error) {
	existente, err := s.repo.BuscarPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existente == nil {
		return nil, errors.New("cliente não encontrado")
	}
	return s.repo.Atualizar(ctx, id, input)
}

// VendedorService contém a lógica de negócio para vendedores
type VendedorService struct {
	repo repository.VendedorRepository
}

// NewVendedorService cria um novo serviço de vendedores
func NewVendedorService(repo repository.VendedorRepository) *VendedorService {
	return &VendedorService{repo: repo}
}

// ListarTodos retorna vendedores com filtros
func (s *VendedorService) ListarTodos(ctx context.Context, filtro models.VendedorFiltro) ([]models.Vendedor, error) {
	return s.repo.ListarTodos(ctx, filtro)
}

// BuscarPorID retorna um vendedor pelo ID
func (s *VendedorService) BuscarPorID(ctx context.Context, id int64) (*models.Vendedor, error) {
	vendedor, err := s.repo.BuscarPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if vendedor == nil {
		return nil, errors.New("vendedor não encontrado")
	}
	return vendedor, nil
}

// CarteiraService contém a lógica de negócio para carteira
type CarteiraService struct {
	repo repository.CarteiraRepository
}

// NewCarteiraService cria um novo serviço de carteira
func NewCarteiraService(repo repository.CarteiraRepository) *CarteiraService {
	return &CarteiraService{repo: repo}
}

// ListarPorVendedor retorna a carteira de um vendedor
func (s *CarteiraService) ListarPorVendedor(ctx context.Context, vendedorID int64) ([]models.Carteira, error) {
	return s.repo.ListarPorVendedor(ctx, vendedorID)
}

// ListarTodos retorna todas as carteiras
func (s *CarteiraService) ListarTodos(ctx context.Context) ([]models.Carteira, error) {
	return s.repo.ListarTodos(ctx)
}

// Criar cria uma nova associação de carteira
func (s *CarteiraService) Criar(ctx context.Context, input models.CarteiraInput) (*models.Carteira, error) {
	if input.ClienteID <= 0 {
		return nil, errors.New("cliente_id é obrigatório")
	}
	if input.VendedorID <= 0 {
		return nil, errors.New("vendedor_id é obrigatório")
	}
	return s.repo.Criar(ctx, input)
}

// VisitaService contém a lógica de negócio para visitas
type VisitaService struct {
	repo repository.VisitaRepository
}

// NewVisitaService cria um novo serviço de visitas
func NewVisitaService(repo repository.VisitaRepository) *VisitaService {
	return &VisitaService{repo: repo}
}

// ListarPorVendedor retorna visitas de um vendedor
func (s *VisitaService) ListarPorVendedor(ctx context.Context, vendedorID int64) ([]models.Visita, error) {
	return s.repo.ListarPorVendedor(ctx, vendedorID)
}

// ListarPorCliente retorna visitas de um cliente
func (s *VisitaService) ListarPorCliente(ctx context.Context, clienteID int64) ([]models.Visita, error) {
	return s.repo.ListarPorCliente(ctx, clienteID)
}

// Criar cria uma nova visita
func (s *VisitaService) Criar(ctx context.Context, input models.VisitaInput) (*models.Visita, error) {
	if input.ClienteID <= 0 {
		return nil, errors.New("cliente_id é obrigatório")
	}
	if input.VendedorID <= 0 {
		return nil, errors.New("vendedor_id é obrigatório")
	}
	if input.Resultado == "" {
		return nil, errors.New("resultado é obrigatório")
	}
	return s.repo.Criar(ctx, input)
}

// OportunidadeService contém a lógica de negócio para oportunidades
type OportunidadeService struct {
	repo repository.OportunidadeRepository
}

// NewOportunidadeService cria um novo serviço de oportunidades
func NewOportunidadeService(repo repository.OportunidadeRepository) *OportunidadeService {
	return &OportunidadeService{repo: repo}
}

// ListarTodos retorna oportunidades com filtros
func (s *OportunidadeService) ListarTodos(ctx context.Context, filtro models.OportunidadeFiltro) ([]models.Oportunidade, error) {
	return s.repo.ListarTodos(ctx, filtro)
}

// BuscarPorID retorna uma oportunidade pelo ID
func (s *OportunidadeService) BuscarPorID(ctx context.Context, id int64) (*models.Oportunidade, error) {
	op, err := s.repo.BuscarPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if op == nil {
		return nil, errors.New("oportunidade não encontrada")
	}
	return op, nil
}

// Criar cria uma nova oportunidade
func (s *OportunidadeService) Criar(ctx context.Context, input models.OportunidadeInput) (*models.Oportunidade, error) {
	if input.ClienteID <= 0 {
		return nil, errors.New("cliente_id é obrigatório")
	}
	if input.VendedorID <= 0 {
		return nil, errors.New("vendedor_id é obrigatório")
	}
	if input.ValorEstimado <= 0 {
		return nil, errors.New("valor_estimado deve ser maior que zero")
	}
	return s.repo.Criar(ctx, input)
}

// Atualizar atualiza uma oportunidade existente
func (s *OportunidadeService) Atualizar(ctx context.Context, id int64, input models.OportunidadeInput) (*models.Oportunidade, error) {
	existente, err := s.repo.BuscarPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existente == nil {
		return nil, errors.New("oportunidade não encontrada")
	}
	return s.repo.Atualizar(ctx, id, input)
}

// AuthService contém a lógica de negócio para autenticação
type AuthService struct {
	repo repository.UsuarioRepository
}

// NewAuthService cria um novo serviço de autenticação
func NewAuthService(repo repository.UsuarioRepository) *AuthService {
	return &AuthService{repo: repo}
}

// Login autentica um usuário
func (s *AuthService) Login(ctx context.Context, login, senha string) (*models.Usuario, error) {
	if login == "" || senha == "" {
		return nil, errors.New("login e senha são obrigatórios")
	}

	usuario, err := s.repo.BuscarPorLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	if usuario == nil {
		return nil, errors.New("usuário ou senha incorretos")
	}
	if usuario.Ativo != "S" {
		return nil, errors.New("usuário inativo")
	}

	err = bcrypt.CompareHashAndPassword([]byte(usuario.SenhaHash), []byte(senha))
	if err != nil {
		return nil, errors.New("usuário ou senha incorretos")
	}

	return usuario, nil
}

// DashboardService contém a lógica de negócio para dashboard
type DashboardService struct {
	repo repository.DashboardRepository
}

// NewDashboardService cria um novo serviço de dashboard
func NewDashboardService(repo repository.DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

// RankingVendedores retorna ranking de vendedores por valor fechado
func (s *DashboardService) RankingVendedores(ctx context.Context) ([]models.RankingVendedor, error) {
	return s.repo.RankingVendedores(ctx)
}

// DistribuicaoCarteira retorna distribuição de clientes por vendedor
func (s *DashboardService) DistribuicaoCarteira(ctx context.Context) ([]models.DistribuicaoCarteira, error) {
	return s.repo.DistribuicaoCarteira(ctx)
}

// RelatorioService contém serviços de relatórios
type RelatorioService struct {
	clienteRepo      repository.ClienteRepository
	vendedorRepo     repository.VendedorRepository
	carteiraRepo     repository.CarteiraRepository
	visitaRepo       repository.VisitaRepository
	oportunidadeRepo repository.OportunidadeRepository
}

// NewRelatorioService cria um novo serviço de relatórios
func NewRelatorioService(
	clienteRepo repository.ClienteRepository,
	vendedorRepo repository.VendedorRepository,
	carteiraRepo repository.CarteiraRepository,
	visitaRepo repository.VisitaRepository,
	oportunidadeRepo repository.OportunidadeRepository,
) *RelatorioService {
	return &RelatorioService{
		clienteRepo:      clienteRepo,
		vendedorRepo:     vendedorRepo,
		carteiraRepo:     carteiraRepo,
		visitaRepo:       visitaRepo,
		oportunidadeRepo: oportunidadeRepo,
	}
}

// ResumoGeral retorna um resumo geral do CRM
type ResumoGeral struct {
	TotalClientes         int `json:"total_clientes"`
	ClientesAtivos        int `json:"clientes_ativos"`
	TotalVendedores       int `json:"total_vendedores"`
	TotalVisitasMes       int `json:"total_visitas_mes"`
	TotalOportunidades    int `json:"total_oportunidades"`
	OportunidadesAbertas  int `json:"oportunidades_abertas"`
	OportunidadesFechadas int `json:"oportunidades_fechadas"`
}

// ResumoGeral retorna um resumo geral do CRM
func (s *RelatorioService) ResumoGeral(ctx context.Context) (*ResumoGeral, error) {
	clientes, err := s.clienteRepo.ListarTodos(ctx, models.ClienteFiltro{})
	if err != nil {
		return nil, err
	}

	clientesAtivos := 0
	for _, c := range clientes {
		if c.Ativo == "S" {
			clientesAtivos++
		}
	}

	vendedores, err := s.vendedorRepo.ListarTodos(ctx, models.VendedorFiltro{})
	if err != nil {
		return nil, err
	}

	mesAtual := time.Now().Month()
	anoAtual := time.Now().Year()
	_ = mesAtual
	_ = anoAtual

	oportunidades, err := s.oportunidadeRepo.ListarTodos(ctx, models.OportunidadeFiltro{})
	if err != nil {
		return nil, err
	}

	abertas := 0
	fechadas := 0
	for _, o := range oportunidades {
		if o.Etapa == "Fechada" {
			fechadas++
		} else {
			abertas++
		}
	}

	return &ResumoGeral{
		TotalClientes:         len(clientes),
		ClientesAtivos:        clientesAtivos,
		TotalVendedores:       len(vendedores),
		TotalVisitasMes:       0,
		TotalOportunidades:    len(oportunidades),
		OportunidadesAbertas:  abertas,
		OportunidadesFechadas: fechadas,
	}, nil
}
