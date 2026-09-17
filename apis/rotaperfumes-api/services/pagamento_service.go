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
)

// Erros exportados para uso em handlers.
var (
	ErrPagamentoNaoEncontrado    = errors.New("pagamento não encontrado")
	ErrPedidoIDObrigatorio       = errors.New("pedido_id é obrigatório")
	ErrFormaPagamentoInvalida    = errors.New("forma_pagamento inválida")
	ErrStatusPagamentoInvalido   = errors.New("status_pagamento inválido")
	ErrParcelasInvalidas         = errors.New("parcelas deve ser maior ou igual a 1")
	ErrValorInvalido             = errors.New("valor deve ser maior ou igual a zero")
	ErrTaxaPctInvalida           = errors.New("taxa_pct deve ser maior ou igual a zero")
	ErrValorLiquidoInvalido      = errors.New("valor_liquido deve ser maior ou igual a zero")
	ErrDataVencimentoObrigatoria = errors.New("data_vencimento é obrigatória (use o formato AAAA-MM-DD)")
	ErrDataVencimentoInvalida    = errors.New("data_vencimento inválida (use o formato AAAA-MM-DD)")
	ErrDataPagamentoInvalida     = errors.New("data_pagamento inválida (use o formato AAAA-MM-DD)")
)

// dataPagamentoLayout é o formato aceito para data_vencimento/data_pagamento
// no payload (mesmo formato de DATE do MySQL).
const dataPagamentoLayout = "2006-01-02"

// formasPagamentoValidas espelha o ENUM forma_pagamento de
// sql/12_ddl_pagamentos.sql.
var formasPagamentoValidas = map[string]bool{
	"Boleto 14 dias":    true,
	"Boleto 28 dias":    true,
	"Cartão de crédito": true,
	"Cartão de débito":  true,
	"Cheque a prazo":    true,
	"Dinheiro":          true,
	"PIX":               true,
}

// statusPagamentoValidos espelha o ENUM status_pagamento de
// sql/12_ddl_pagamentos.sql.
var statusPagamentoValidos = map[string]bool{
	"Em aberto":       true,
	"Inadimplente":    true,
	"Pago":            true,
	"Pago com atraso": true,
}

// PagamentoFiltro agrupa os filtros opcionais aceitos por ListPagamentos.
type PagamentoFiltro struct {
	StatusPagamento string
	FormaPagamento  string
	PedidoID        int64
	VencimentoDe    string
	VencimentoAte   string
	VendedorID      int64 // > 0 restringe aos pagamentos de pedidos desse vendedor
	OrderBy         string
	OrderDir        string
}

// PagamentoService agrega regras de negócio sobre pagamentos.
type PagamentoService struct {
	repo       *repositories.PagamentoRepository
	pedidoRepo *repositories.PedidoRepository
	Cfg        *config.Config
}

// NewPagamentoService cria um PagamentoService com pool de conexão injetado.
func NewPagamentoService(db *sql.DB, cfg *config.Config) *PagamentoService {
	return &PagamentoService{
		repo:       repositories.NewPagamentoRepository(),
		pedidoRepo: repositories.NewPedidoRepository(),
		Cfg:        cfg,
	}
}

// ListPagamentos pagina pagamentos aplicando os filtros informados.
// page/limit são validados (limit max 100) no repositório.
func (s *PagamentoService) ListPagamentos(ctx context.Context, db *sql.DB, page, limit int, filtro PagamentoFiltro) ([]models.Pagamento, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[pagamentos] list page=%d limit=%d status=%q forma=%q pedido_id=%d vencimento_de=%q vencimento_ate=%q vendedor_id=%d",
			page, limit, filtro.StatusPagamento, filtro.FormaPagamento, filtro.PedidoID, filtro.VencimentoDe, filtro.VencimentoAte, filtro.VendedorID)
	}
	repoFiltro := repositories.PagamentoFiltro{
		StatusPagamento: filtro.StatusPagamento,
		FormaPagamento:  filtro.FormaPagamento,
		PedidoID:        filtro.PedidoID,
		VencimentoDe:    filtro.VencimentoDe,
		VencimentoAte:   filtro.VencimentoAte,
		VendedorID:      filtro.VendedorID,
		OrderBy:         filtro.OrderBy,
		OrderDir:        filtro.OrderDir,
	}
	return s.repo.List(ctx, db, page, limit, repoFiltro)
}

// GetPagamentoByID busca um pagamento por pagamento_id. Retorna
// ErrPagamentoNaoEncontrado se não existir.
func (s *PagamentoService) GetPagamentoByID(ctx context.Context, db *sql.DB, pagamentoID int64) (*models.Pagamento, error) {
	p, err := s.repo.GetByID(ctx, db, pagamentoID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrPagamentoNaoEncontrado
		}
		return nil, err
	}
	return p, nil
}

// VendedorIDDoPedido retorna o vendedor_id do pedido informado. Usado para
// verificar se um pagamento pertence à carteira do usuário autenticado
// (role=normal), já que a tabela pagamentos não guarda vendedor_id
// diretamente — o vínculo é via pedidos.vendedor_id.
// Retorna ErrPedidoNaoEncontrado se o pedido não existir.
func (s *PagamentoService) VendedorIDDoPedido(ctx context.Context, db *sql.DB, pedidoID int64) (int64, error) {
	pedido, err := s.pedidoRepo.GetByID(ctx, db, pedidoID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return 0, ErrPedidoNaoEncontrado
		}
		return 0, err
	}
	return pedido.VendedorID, nil
}

// PagamentoInput agrupa os campos aceitos no payload de criação/edição de
// pagamentos.
//
// Decisão de negócio: valor_liquido é sempre exigido explicitamente no
// payload (não é calculado automaticamente a partir de valor/taxa_pct).
// Motivo: o CSV de origem traz valor_liquido já calculado e, em alguns
// registros, com pequenas diferenças de arredondamento em relação a
// valor * (1 - taxa_pct/100) — exigir o valor explícito evita divergência
// silenciosa entre o que o usuário informa e o que fica persistido.
type PagamentoInput struct {
	PedidoID        int64
	FormaPagamento  string
	Parcelas        uint8
	Valor           float64
	TaxaPct         float64
	ValorLiquido    float64
	DataVencimento  string // formato AAAA-MM-DD, obrigatório
	DataPagamento   string // formato AAAA-MM-DD, opcional (vazio = NULL)
	StatusPagamento string
}

// validarPagamentoInput aplica as validações comuns a criação e edição,
// normaliza os campos e resolve as datas.
func validarPagamentoInput(input PagamentoInput) (formaPagamento, statusPagamento string, dataVencimento time.Time, dataPagamento *time.Time, err error) {
	formaPagamento = strings.TrimSpace(input.FormaPagamento)
	statusPagamento = strings.TrimSpace(input.StatusPagamento)
	dataVencimentoStr := strings.TrimSpace(input.DataVencimento)
	dataPagamentoStr := strings.TrimSpace(input.DataPagamento)

	if !formasPagamentoValidas[formaPagamento] {
		err = ErrFormaPagamentoInvalida
		return
	}
	if !statusPagamentoValidos[statusPagamento] {
		err = ErrStatusPagamentoInvalido
		return
	}
	if input.Parcelas < 1 {
		err = ErrParcelasInvalidas
		return
	}
	if input.Valor < 0 {
		err = ErrValorInvalido
		return
	}
	if input.TaxaPct < 0 {
		err = ErrTaxaPctInvalida
		return
	}
	if input.ValorLiquido < 0 {
		err = ErrValorLiquidoInvalido
		return
	}
	if dataVencimentoStr == "" {
		err = ErrDataVencimentoObrigatoria
		return
	}
	t, parseErr := time.Parse(dataPagamentoLayout, dataVencimentoStr)
	if parseErr != nil {
		err = ErrDataVencimentoInvalida
		return
	}
	dataVencimento = t

	if dataPagamentoStr != "" {
		dp, parseErr := time.Parse(dataPagamentoLayout, dataPagamentoStr)
		if parseErr != nil {
			err = ErrDataPagamentoInvalida
			return
		}
		dataPagamento = &dp
	}
	return
}

// CreatePagamento cria um novo pagamento, validando pedido_id (deve
// existir) e os demais campos.
func (s *PagamentoService) CreatePagamento(ctx context.Context, db *sql.DB, input PagamentoInput) (*models.Pagamento, error) {
	if input.PedidoID <= 0 {
		return nil, ErrPedidoIDObrigatorio
	}
	existe, err := s.pedidoRepo.ExistsByID(ctx, db, input.PedidoID)
	if err != nil {
		return nil, err
	}
	if !existe {
		return nil, ErrPedidoNaoEncontrado
	}

	formaPagamento, statusPagamento, dataVencimento, dataPagamento, err := validarPagamentoInput(input)
	if err != nil {
		return nil, err
	}

	p := &models.Pagamento{
		PedidoID:        input.PedidoID,
		FormaPagamento:  formaPagamento,
		Parcelas:        input.Parcelas,
		Valor:           input.Valor,
		TaxaPct:         input.TaxaPct,
		ValorLiquido:    input.ValorLiquido,
		DataVencimento:  dataVencimento,
		DataPagamento:   dataPagamento,
		StatusPagamento: statusPagamento,
	}
	if err := s.repo.Create(ctx, db, p); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[pagamentos] criado: pagamento_id=%d pedido_id=%d valor=%.2f status=%s", p.PagamentoID, p.PedidoID, p.Valor, p.StatusPagamento)
	}
	return p, nil
}

// UpdatePagamento atualiza os campos editáveis de um pagamento existente.
// pagamento_id e pedido_id não são alteráveis após a criação: o vínculo com
// o pedido de origem é definitivo (para reatribuir um pagamento a outro
// pedido, o fluxo correto é excluir/recriar, garantindo trilha de auditoria
// clara em vez de um UPDATE silencioso na FK).
// Retorna ErrPagamentoNaoEncontrado se não existir.
func (s *PagamentoService) UpdatePagamento(ctx context.Context, db *sql.DB, pagamentoID int64, input PagamentoInput) (*models.Pagamento, error) {
	formaPagamento, statusPagamento, dataVencimento, dataPagamento, err := validarPagamentoInput(input)
	if err != nil {
		return nil, err
	}

	p := &models.Pagamento{
		FormaPagamento:  formaPagamento,
		Parcelas:        input.Parcelas,
		Valor:           input.Valor,
		TaxaPct:         input.TaxaPct,
		ValorLiquido:    input.ValorLiquido,
		DataVencimento:  dataVencimento,
		DataPagamento:   dataPagamento,
		StatusPagamento: statusPagamento,
	}
	if err := s.repo.Update(ctx, db, pagamentoID, p); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrPagamentoNaoEncontrado
		}
		return nil, err
	}

	atualizado, err := s.repo.GetByID(ctx, db, pagamentoID)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[pagamentos] atualizado: pagamento_id=%d status=%s", pagamentoID, statusPagamento)
	}
	return atualizado, nil
}
