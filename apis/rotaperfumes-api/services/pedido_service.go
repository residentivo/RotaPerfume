// Package services (da API) orquestra regras de negócio dos endpoints.
package services

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
	"github.com/rotaperfumes/shared/vlog"
)

// Erros exportados para uso em handlers.
var (
	ErrPedidoNaoEncontrado    = errors.New("pedido não encontrado")
	ErrClienteIDObrigatorio   = errors.New("cliente_id é obrigatório")
	ErrVendedorIDObrigatorio  = errors.New("vendedor_id é obrigatório")
	ErrDataPedidoInvalida     = errors.New("data_pedido inválida (use o formato AAAA-MM-DD)")
	ErrCanalInvalido          = errors.New("canal inválido (use: App, Telefone, Visita, WhatsApp)")
	ErrStatusInvalido         = errors.New("status inválido (use: Cancelado, Em separação, Entregue, Faturado)")
	ErrItensObrigatorios      = errors.New("o pedido deve ter ao menos um item")
	ErrProdutoIDObrigatorio   = errors.New("produto_id é obrigatório em todos os itens")
	ErrQuantidadeInvalida     = errors.New("quantidade deve ser maior que zero em todos os itens")
	ErrPrecoPraticadoInvalido = errors.New("preco_praticado deve ser maior ou igual a zero em todos os itens")
	ErrDescontoPctInvalido    = errors.New("desconto_pct deve estar entre 0 e 100 em todos os itens")
	// ErrPedidoJaFaturadoNaoPodeAlterarItens espelha
	// repositories.ErrPedidoJaFaturadoNaoPodeAlterarItens para uso em
	// handlers, sem expor o pacote repositories diretamente.
	ErrPedidoJaFaturadoNaoPodeAlterarItens = repositories.ErrPedidoJaFaturadoNaoPodeAlterarItens
	// ErrPedidoPossuiPagamentosVinculados é retornado por DeletePedido quando
	// existe ao menos um pagamento com pedido_id apontando para o pedido —
	// exigência do SecBrain para preservar a integridade da trilha financeira.
	ErrPedidoPossuiPagamentosVinculados = errors.New("pedido possui pagamentos vinculados: remova-os antes de excluir o pedido")
	// ErrPedidoFaturadoNaoPodeSerExcluido é retornado por DeletePedido quando
	// o pedido está com status "Faturado" — exigência do SecBrain: um pedido
	// faturado só pode ter seu status alterado (ex: para "Entregue"), nunca
	// ser excluído, pois já baixou estoque e pode ter registros financeiros
	// associados.
	ErrPedidoFaturadoNaoPodeSerExcluido = errors.New("pedido faturado não pode ser excluído, apenas ter o status alterado")
)

// dataPedidoLayout é o formato aceito para o campo data_pedido no payload de
// criação/edição de pedidos (mesmo formato de DATE do MySQL).
const dataPedidoLayout = "2006-01-02"

// canaisValidos e statusValidos espelham os ENUMs de sql/04_ddl_pedidos.sql.
var canaisValidos = map[string]bool{"App": true, "Telefone": true, "Visita": true, "WhatsApp": true}
var statusValidos = map[string]bool{"Cancelado": true, "Em separação": true, "Entregue": true, "Faturado": true}

// PedidoFiltro agrupa os filtros opcionais aceitos por ListPedidos.
type PedidoFiltro struct {
	Status     string
	Canal      string
	ClienteID  int64
	VendedorID int64
	DataInicio string
	DataFim    string
	Q          string
	OrderBy    string
	OrderDir   string
}

// PedidoService agrega regras de negócio sobre pedidos e seus itens.
type PedidoService struct {
	repo          *repositories.PedidoRepository
	pagamentoRepo *repositories.PagamentoRepository
	Cfg           *config.Config
}

// NewPedidoService cria um PedidoService com pool de conexão injetado.
func NewPedidoService(db *sql.DB, cfg *config.Config) *PedidoService {
	return &PedidoService{
		repo:          repositories.NewPedidoRepository(),
		pagamentoRepo: repositories.NewPagamentoRepository(),
		Cfg:           cfg,
	}
}

// ListPedidos pagina pedidos (sem itens) aplicando os filtros informados.
// page/limit são validados (limit max 100) no repositório.
func (s *PedidoService) ListPedidos(ctx context.Context, db *sql.DB, page, limit int, filtro PedidoFiltro) ([]repositories.PedidoListagem, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[pedidos] list page=%d limit=%d status=%q canal=%q cliente_id=%d vendedor_id=%d data_inicio=%q data_fim=%q q=%q",
			page, limit, filtro.Status, filtro.Canal, filtro.ClienteID, filtro.VendedorID, filtro.DataInicio, filtro.DataFim, filtro.Q)
	}
	vlog.Printf("pedido_service.go", "PedidoService.ListPedidos", "montando literal repositories.PedidoFiltro e declarando repoFiltro")
	repoFiltro := repositories.PedidoFiltro{
		Status:     filtro.Status,
		Canal:      filtro.Canal,
		ClienteID:  filtro.ClienteID,
		VendedorID: filtro.VendedorID,
		DataInicio: filtro.DataInicio,
		DataFim:    filtro.DataFim,
		Q:          filtro.Q,
		OrderBy:    filtro.OrderBy,
		OrderDir:   filtro.OrderDir,
	}
	return s.repo.List(ctx, db, page, limit, repoFiltro)
}

// GetPedidoDetalhe busca o cabeçalho de um pedido e seus itens (master-detail).
// Retorna ErrPedidoNaoEncontrado se não existir.
func (s *PedidoService) GetPedidoDetalhe(ctx context.Context, db *sql.DB, id int64) (*repositories.PedidoDetalhe, error) {
	vlog.Printf("pedido_service.go", "PedidoService.GetPedidoDetalhe", "chamando s.repo.GetByID e declarando p, err")
	p, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("pedido_service.go", "PedidoService.GetPedidoDetalhe", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("pedido_service.go", "PedidoService.GetPedidoDetalhe", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrPedidoNaoEncontrado
		}
		return nil, err
	}

	vlog.Printf("pedido_service.go", "PedidoService.GetPedidoDetalhe", "chamando s.repo.ListItensByPedidoID e declarando itens, err")
	itens, err := s.repo.ListItensByPedidoID(ctx, db, id)
	vlog.Printf("pedido_service.go", "PedidoService.GetPedidoDetalhe", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	return &repositories.PedidoDetalhe{
		PedidoListagem: *p,
		Itens:          itens,
	}, nil
}

// ItemPedidoInput representa um item do payload de criação/edição de pedido.
type ItemPedidoInput struct {
	ProdutoID      int64
	Quantidade     int
	PrecoPraticado float64
	DescontoPct    float64
}

// PedidoInput agrupa os campos editáveis de um pedido (cabeçalho + itens),
// usados tanto na criação quanto na edição.
type PedidoInput struct {
	ClienteID  int64
	VendedorID int64
	DataPedido string // formato AAAA-MM-DD
	Canal      string
	Status     string
	Itens      []ItemPedidoInput
}

// validarPedidoInput aplica as validações comuns a criação e edição,
// normaliza os campos e calcula o valor_bruto de cada item e o valor_total
// do pedido (quantidade * preco_praticado * (1 - desconto_pct/100)).
func validarPedidoInput(input PedidoInput) (pedido models.Pedido, itens []models.ItemPedido, err error) {
	vlog.Printf("pedido_service.go", "validarPedidoInput", "verificando condição input.ClienteID <= 0")
	if input.ClienteID <= 0 {
		vlog.Printf("pedido_service.go", "validarPedidoInput", "atribuindo ErrClienteIDObrigatorio a err")
		err = ErrClienteIDObrigatorio
		return
	}
	vlog.Printf("pedido_service.go", "validarPedidoInput", "verificando condição input.VendedorID <= 0")
	if input.VendedorID <= 0 {
		vlog.Printf("pedido_service.go", "validarPedidoInput", "atribuindo ErrVendedorIDObrigatorio a err")
		err = ErrVendedorIDObrigatorio
		return
	}

	vlog.Printf("pedido_service.go", "validarPedidoInput", "chamando strings.TrimSpace e declarando dataPedidoStr")
	dataPedidoStr := strings.TrimSpace(input.DataPedido)
	vlog.Printf("pedido_service.go", "validarPedidoInput", "verificando condição dataPedidoStr == \"\"")
	if dataPedidoStr == "" {
		vlog.Printf("pedido_service.go", "validarPedidoInput", "atribuindo ErrDataPedidoInvalida a err")
		err = ErrDataPedidoInvalida
		return
	}
	vlog.Printf("pedido_service.go", "validarPedidoInput", "chamando time.ParseInLocation e declarando dataPedido, parseErr")
	dataPedido, parseErr := time.ParseInLocation(dataPedidoLayout, dataPedidoStr, time.Local)
	vlog.Printf("pedido_service.go", "validarPedidoInput", "verificando condição parseErr != nil")
	if parseErr != nil {
		vlog.Printf("pedido_service.go", "validarPedidoInput", "atribuindo ErrDataPedidoInvalida a err")
		err = ErrDataPedidoInvalida
		return
	}

	vlog.Printf("pedido_service.go", "validarPedidoInput", "chamando strings.TrimSpace e declarando canal")
	canal := strings.TrimSpace(input.Canal)
	vlog.Printf("pedido_service.go", "validarPedidoInput", "verificando condição !canaisValidos[canal]")
	if !canaisValidos[canal] {
		vlog.Printf("pedido_service.go", "validarPedidoInput", "atribuindo ErrCanalInvalido a err")
		err = ErrCanalInvalido
		return
	}

	vlog.Printf("pedido_service.go", "validarPedidoInput", "chamando strings.TrimSpace e declarando status")
	status := strings.TrimSpace(input.Status)
	vlog.Printf("pedido_service.go", "validarPedidoInput", "verificando condição !statusValidos[status]")
	if !statusValidos[status] {
		vlog.Printf("pedido_service.go", "validarPedidoInput", "atribuindo ErrStatusInvalido a err")
		err = ErrStatusInvalido
		return
	}

	vlog.Printf("pedido_service.go", "validarPedidoInput", "verificando condição len(input.Itens) == 0")
	if len(input.Itens) == 0 {
		vlog.Printf("pedido_service.go", "validarPedidoInput", "atribuindo ErrItensObrigatorios a err")
		err = ErrItensObrigatorios
		return
	}

	vlog.Printf("pedido_service.go", "validarPedidoInput", "declarando valorTotal")
	var valorTotal float64
	vlog.Printf("pedido_service.go", "validarPedidoInput", "chamando make e atribuindo a itens")
	itens = make([]models.ItemPedido, 0, len(input.Itens))
	vlog.Printf("pedido_service.go", "validarPedidoInput", "iniciando loop range sobre input.Itens")
	for _, itemInput := range input.Itens {
		if itemInput.ProdutoID <= 0 {
			err = ErrProdutoIDObrigatorio
			return
		}
		if itemInput.Quantidade <= 0 {
			err = ErrQuantidadeInvalida
			return
		}
		if itemInput.PrecoPraticado < 0 {
			err = ErrPrecoPraticadoInvalido
			return
		}
		if itemInput.DescontoPct < 0 || itemInput.DescontoPct > 100 {
			err = ErrDescontoPctInvalido
			return
		}

		valorBruto := calcularValorBruto(itemInput.Quantidade, itemInput.PrecoPraticado, itemInput.DescontoPct)
		valorTotal += valorBruto

		itens = append(itens, models.ItemPedido{
			ProdutoID:      itemInput.ProdutoID,
			Quantidade:     itemInput.Quantidade,
			PrecoPraticado: itemInput.PrecoPraticado,
			DescontoPct:    itemInput.DescontoPct,
			ValorBruto:     valorBruto,
		})
	}
	vlog.Printf("pedido_service.go", "validarPedidoInput", "loop range concluído sobre input.Itens: %d itens", len(input.Itens))

	vlog.Printf("pedido_service.go", "validarPedidoInput", "montando literal models.Pedido e atribuindo a pedido")
	pedido = models.Pedido{
		ClienteID:  input.ClienteID,
		VendedorID: input.VendedorID,
		DataPedido: dataPedido,
		Canal:      canal,
		Status:     status,
		ValorTotal: valorTotal,
	}
	return
}

// calcularValorBruto calcula o valor bruto de um item de pedido:
// quantidade * preco_praticado * (1 - desconto_pct/100).
func calcularValorBruto(quantidade int, precoPraticado, descontoPct float64) float64 {
	return float64(quantidade) * precoPraticado * (1 - descontoPct/100)
}

// CreatePedido cria um novo pedido com seus itens, calculando valor_bruto de
// cada item e valor_total do pedido no backend. pedido_id_origem é gerado
// nativamente pelo AUTO_INCREMENT do MySQL.
func (s *PedidoService) CreatePedido(ctx context.Context, db *sql.DB, input PedidoInput) (*repositories.PedidoDetalhe, error) {
	vlog.Printf("pedido_service.go", "PedidoService.CreatePedido", "chamando validarPedidoInput e declarando pedido, itens, err")
	pedido, itens, err := validarPedidoInput(input)
	vlog.Printf("pedido_service.go", "PedidoService.CreatePedido", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("pedido_service.go", "PedidoService.CreatePedido", "chamando s.repo.CreateComItens e declarando err e verificando condição err != nil")
	if err := s.repo.CreateComItens(ctx, db, &pedido, itens); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[pedidos] criado: pedido_id_origem=%d cliente_id=%d valor_total=%.2f itens=%d",
			pedido.PedidoIDOrigem, pedido.ClienteID, pedido.ValorTotal, len(itens))
	}

	return s.GetPedidoDetalhe(ctx, db, pedido.PedidoIDOrigem)
}

// UpdatePedido atualiza o cabeçalho de um pedido existente e substitui
// integralmente a lista de itens, recalculando valor_bruto/valor_total.
// Retorna ErrPedidoNaoEncontrado se não existir.
func (s *PedidoService) UpdatePedido(ctx context.Context, db *sql.DB, id int64, input PedidoInput) (*repositories.PedidoDetalhe, error) {
	vlog.Printf("pedido_service.go", "PedidoService.UpdatePedido", "chamando validarPedidoInput e declarando pedido, itens, err")
	pedido, itens, err := validarPedidoInput(input)
	vlog.Printf("pedido_service.go", "PedidoService.UpdatePedido", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("pedido_service.go", "PedidoService.UpdatePedido", "chamando s.repo.UpdateComItens e declarando err e verificando condição err != nil")
	if err := s.repo.UpdateComItens(ctx, db, id, &pedido, itens); err != nil {
		vlog.Printf("pedido_service.go", "PedidoService.UpdatePedido", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrPedidoNaoEncontrado
		}
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[pedidos] atualizado: id=%d valor_total=%.2f itens=%d", id, pedido.ValorTotal, len(itens))
	}

	return s.GetPedidoDetalhe(ctx, db, id)
}

// DeletePedido remove um pedido e seus itens (hard delete), aplicando as
// regras de negócio exigidas pelo SecBrain:
//   - bloqueia a exclusão (ErrPedidoPossuiPagamentosVinculados) se existir
//     qualquer pagamento vinculado ao pedido;
//   - bloqueia a exclusão (ErrPedidoFaturadoNaoPodeSerExcluido) se o pedido
//     estiver com status "Faturado" (só é alterável via mudança de status).
//
// Retorna ErrPedidoNaoEncontrado se o pedido não existir. O scope check por
// carteira (vendedor) é responsabilidade do handler chamador, feito antes de
// invocar este método.
func (s *PedidoService) DeletePedido(ctx context.Context, db *sql.DB, id int64) error {
	vlog.Printf("pedido_service.go", "PedidoService.DeletePedido", "chamando s.repo.GetByID e declarando pedido, err")
	pedido, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("pedido_service.go", "PedidoService.DeletePedido", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("pedido_service.go", "PedidoService.DeletePedido", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrPedidoNaoEncontrado
		}
		return err
	}

	vlog.Printf("pedido_service.go", "PedidoService.DeletePedido", "chamando s.pagamentoRepo.ExistsByPedidoID e declarando possuiPagamentos, err")
	possuiPagamentos, err := s.pagamentoRepo.ExistsByPedidoID(ctx, db, id)
	vlog.Printf("pedido_service.go", "PedidoService.DeletePedido", "verificando condição err != nil")
	if err != nil {
		return err
	}
	vlog.Printf("pedido_service.go", "PedidoService.DeletePedido", "verificando condição possuiPagamentos")
	if possuiPagamentos {
		return ErrPedidoPossuiPagamentosVinculados
	}

	vlog.Printf("pedido_service.go", "PedidoService.DeletePedido", "verificando condição pedido.Status == \"Faturado\"")
	if pedido.Status == "Faturado" {
		return ErrPedidoFaturadoNaoPodeSerExcluido
	}

	vlog.Printf("pedido_service.go", "PedidoService.DeletePedido", "chamando s.repo.DeleteComItens e declarando err e verificando condição err != nil")
	if err := s.repo.DeleteComItens(ctx, db, id); err != nil {
		vlog.Printf("pedido_service.go", "PedidoService.DeletePedido", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrPedidoNaoEncontrado
		}
		return err
	}

	if s.Cfg.Verbose {
		log.Printf("[pedidos] excluído: id=%d", id)
	}
	return nil
}
