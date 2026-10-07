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
	// ErrPagamentoJaQuitadoNaoPodeSerExcluido é retornado por DeletePagamento
	// quando status_pagamento já é "Pago" ou "Pago com atraso" — exigência do
	// SecBrain para preservar a trilha financeira de pagamentos já quitados.
	ErrPagamentoJaQuitadoNaoPodeSerExcluido = errors.New("pagamento já quitado não pode ser excluído")
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
	vlog.Printf("pagamento_service.go", "PagamentoService.ListPagamentos", "montando literal repositories.PagamentoFiltro e declarando repoFiltro")
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
	vlog.Printf("pagamento_service.go", "PagamentoService.GetPagamentoByID", "chamando s.repo.GetByID e declarando p, err")
	p, err := s.repo.GetByID(ctx, db, pagamentoID)
	vlog.Printf("pagamento_service.go", "PagamentoService.GetPagamentoByID", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("pagamento_service.go", "PagamentoService.GetPagamentoByID", "verificando condição errors.Is(err, repositories.ErrNotFound)")
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
	vlog.Printf("pagamento_service.go", "PagamentoService.VendedorIDDoPedido", "chamando s.pedidoRepo.GetByID e declarando pedido, err")
	pedido, err := s.pedidoRepo.GetByID(ctx, db, pedidoID)
	vlog.Printf("pagamento_service.go", "PagamentoService.VendedorIDDoPedido", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("pagamento_service.go", "PagamentoService.VendedorIDDoPedido", "verificando condição errors.Is(err, repositories.ErrNotFound)")
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
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "chamando strings.TrimSpace e atribuindo a formaPagamento")
	formaPagamento = strings.TrimSpace(input.FormaPagamento)
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "chamando strings.TrimSpace e atribuindo a statusPagamento")
	statusPagamento = strings.TrimSpace(input.StatusPagamento)
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "chamando strings.TrimSpace e declarando dataVencimentoStr")
	dataVencimentoStr := strings.TrimSpace(input.DataVencimento)
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "chamando strings.TrimSpace e declarando dataPagamentoStr")
	dataPagamentoStr := strings.TrimSpace(input.DataPagamento)

	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "verificando condição !formasPagamentoValidas[formaPagamento]")
	if !formasPagamentoValidas[formaPagamento] {
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo ErrFormaPagamentoInvalida a err")
		err = ErrFormaPagamentoInvalida
		return
	}
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "verificando condição !statusPagamentoValidos[statusPagamento]")
	if !statusPagamentoValidos[statusPagamento] {
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo ErrStatusPagamentoInvalido a err")
		err = ErrStatusPagamentoInvalido
		return
	}
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "verificando condição input.Parcelas < 1")
	if input.Parcelas < 1 {
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo ErrParcelasInvalidas a err")
		err = ErrParcelasInvalidas
		return
	}
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "verificando condição input.Valor < 0")
	if input.Valor < 0 {
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo ErrValorInvalido a err")
		err = ErrValorInvalido
		return
	}
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "verificando condição input.TaxaPct < 0")
	if input.TaxaPct < 0 {
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo ErrTaxaPctInvalida a err")
		err = ErrTaxaPctInvalida
		return
	}
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "verificando condição input.ValorLiquido < 0")
	if input.ValorLiquido < 0 {
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo ErrValorLiquidoInvalido a err")
		err = ErrValorLiquidoInvalido
		return
	}
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "verificando condição dataVencimentoStr == \"\"")
	if dataVencimentoStr == "" {
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo ErrDataVencimentoObrigatoria a err")
		err = ErrDataVencimentoObrigatoria
		return
	}
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "chamando time.ParseInLocation e declarando t, parseErr")
	t, parseErr := time.ParseInLocation(dataPagamentoLayout, dataVencimentoStr, time.Local)
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "verificando condição parseErr != nil")
	if parseErr != nil {
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo ErrDataVencimentoInvalida a err")
		err = ErrDataVencimentoInvalida
		return
	}
	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo t a dataVencimento")
	dataVencimento = t

	vlog.Printf("pagamento_service.go", "validarPagamentoInput", "verificando condição dataPagamentoStr != \"\"")
	if dataPagamentoStr != "" {
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "chamando time.ParseInLocation e declarando dp, parseErr")
		dp, parseErr := time.ParseInLocation(dataPagamentoLayout, dataPagamentoStr, time.Local)
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "verificando condição parseErr != nil")
		if parseErr != nil {
			vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo ErrDataPagamentoInvalida a err")
			err = ErrDataPagamentoInvalida
			return
		}
		vlog.Printf("pagamento_service.go", "validarPagamentoInput", "atribuindo &dp a dataPagamento")
		dataPagamento = &dp
	}
	return
}

// CreatePagamento cria um novo pagamento, validando pedido_id (deve
// existir) e os demais campos.
func (s *PagamentoService) CreatePagamento(ctx context.Context, db *sql.DB, input PagamentoInput) (*models.Pagamento, error) {
	vlog.Printf("pagamento_service.go", "PagamentoService.CreatePagamento", "verificando condição input.PedidoID <= 0")
	if input.PedidoID <= 0 {
		return nil, ErrPedidoIDObrigatorio
	}
	vlog.Printf("pagamento_service.go", "PagamentoService.CreatePagamento", "chamando s.pedidoRepo.ExistsByID e declarando existe, err")
	existe, err := s.pedidoRepo.ExistsByID(ctx, db, input.PedidoID)
	vlog.Printf("pagamento_service.go", "PagamentoService.CreatePagamento", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("pagamento_service.go", "PagamentoService.CreatePagamento", "verificando condição !existe")
	if !existe {
		return nil, ErrPedidoNaoEncontrado
	}

	vlog.Printf("pagamento_service.go", "PagamentoService.CreatePagamento", "chamando validarPagamentoInput e declarando formaPagamento, statusPagamento, dataVencimento, dataPagamento, err")
	formaPagamento, statusPagamento, dataVencimento, dataPagamento, err := validarPagamentoInput(input)
	vlog.Printf("pagamento_service.go", "PagamentoService.CreatePagamento", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("pagamento_service.go", "PagamentoService.CreatePagamento", "montando &models.Pagamento e declarando p")
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
	vlog.Printf("pagamento_service.go", "PagamentoService.CreatePagamento", "chamando s.repo.Create e declarando err e verificando condição err != nil")
	if err := s.repo.Create(ctx, db, p); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[pagamentos] criado: pagamento_id=%d pedido_id=%d valor=%.2f status=%s", p.PagamentoID, p.PedidoID, p.Valor, p.StatusPagamento)
	}
	return s.relerPagamentoCriado(ctx, db, p), nil
}

// relerPagamentoCriado relê do banco o pagamento recém-gravado (BUG-09), para
// devolver created_at/updated_at preenchidos pelo MySQL. O INSERT já foi
// confirmado: se a releitura falhar, loga e devolve o objeto em memória
// (timestamps zerados) em vez de responder erro para uma gravação que deu
// certo — um retry duplicaria o pagamento.
func (s *PagamentoService) relerPagamentoCriado(ctx context.Context, db *sql.DB, p *models.Pagamento) *models.Pagamento {
	vlog.Printf("pagamento_service.go", "PagamentoService.relerPagamentoCriado", "chamando s.repo.GetByID e declarando gravado, err")
	gravado, err := s.repo.GetByID(ctx, db, p.PagamentoID)
	vlog.Printf("pagamento_service.go", "PagamentoService.relerPagamentoCriado", "verificando condição err != nil")
	if err != nil {
		log.Printf("[pagamentos] criado, mas falhou a releitura: pagamento_id=%d: %v", p.PagamentoID, err)
		return p
	}
	return gravado
}

// UpdatePagamento atualiza os campos editáveis de um pagamento existente.
// pagamento_id e pedido_id não são alteráveis após a criação: o vínculo com
// o pedido de origem é definitivo (para reatribuir um pagamento a outro
// pedido, o fluxo correto é excluir/recriar, garantindo trilha de auditoria
// clara em vez de um UPDATE silencioso na FK).
// Retorna ErrPagamentoNaoEncontrado se não existir.
func (s *PagamentoService) UpdatePagamento(ctx context.Context, db *sql.DB, pagamentoID int64, input PagamentoInput) (*models.Pagamento, error) {
	vlog.Printf("pagamento_service.go", "PagamentoService.UpdatePagamento", "chamando validarPagamentoInput e declarando formaPagamento, statusPagamento, dataVencimento, dataPagamento, err")
	formaPagamento, statusPagamento, dataVencimento, dataPagamento, err := validarPagamentoInput(input)
	vlog.Printf("pagamento_service.go", "PagamentoService.UpdatePagamento", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("pagamento_service.go", "PagamentoService.UpdatePagamento", "montando &models.Pagamento e declarando p")
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
	vlog.Printf("pagamento_service.go", "PagamentoService.UpdatePagamento", "chamando s.repo.Update e declarando err e verificando condição err != nil")
	if err := s.repo.Update(ctx, db, pagamentoID, p); err != nil {
		vlog.Printf("pagamento_service.go", "PagamentoService.UpdatePagamento", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrPagamentoNaoEncontrado
		}
		return nil, err
	}

	vlog.Printf("pagamento_service.go", "PagamentoService.UpdatePagamento", "chamando s.repo.GetByID e declarando atualizado, err")
	atualizado, err := s.repo.GetByID(ctx, db, pagamentoID)
	vlog.Printf("pagamento_service.go", "PagamentoService.UpdatePagamento", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[pagamentos] atualizado: pagamento_id=%d status=%s", pagamentoID, statusPagamento)
	}
	return atualizado, nil
}

// statusPagamentoQuitados são os status que impedem a exclusão de um
// pagamento (exigência do SecBrain): uma vez quitado, o registro deve ser
// preservado como trilha financeira.
var statusPagamentoQuitados = map[string]bool{
	"Pago":            true,
	"Pago com atraso": true,
}

// DeletePagamento remove um pagamento (hard delete). Retorna
// ErrPagamentoNaoEncontrado se não existir e
// ErrPagamentoJaQuitadoNaoPodeSerExcluido se status_pagamento já for "Pago"
// ou "Pago com atraso". O scope check por carteira (via VendedorIDDoPedido) é
// responsabilidade do handler chamador, feito antes de invocar este método.
func (s *PagamentoService) DeletePagamento(ctx context.Context, db *sql.DB, pagamentoID int64) error {
	vlog.Printf("pagamento_service.go", "PagamentoService.DeletePagamento", "chamando s.repo.GetByID e declarando pagamento, err")
	pagamento, err := s.repo.GetByID(ctx, db, pagamentoID)
	vlog.Printf("pagamento_service.go", "PagamentoService.DeletePagamento", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("pagamento_service.go", "PagamentoService.DeletePagamento", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrPagamentoNaoEncontrado
		}
		return err
	}

	vlog.Printf("pagamento_service.go", "PagamentoService.DeletePagamento", "verificando condição statusPagamentoQuitados[pagamento.StatusPagamento]")
	if statusPagamentoQuitados[pagamento.StatusPagamento] {
		return ErrPagamentoJaQuitadoNaoPodeSerExcluido
	}

	vlog.Printf("pagamento_service.go", "PagamentoService.DeletePagamento", "chamando s.repo.Delete e declarando err e verificando condição err != nil")
	if err := s.repo.Delete(ctx, db, pagamentoID); err != nil {
		vlog.Printf("pagamento_service.go", "PagamentoService.DeletePagamento", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrPagamentoNaoEncontrado
		}
		return err
	}

	if s.Cfg.Verbose {
		log.Printf("[pagamentos] excluído: pagamento_id=%d", pagamentoID)
	}
	return nil
}
