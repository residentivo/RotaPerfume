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
	ErrEstoqueNaoEncontrado   = errors.New("registro de estoque não encontrado")
	ErrEstoqueSKUObrigatorio  = errors.New("sku é obrigatório")
	ErrEstoqueSKUInexistente  = errors.New("sku não corresponde a um produto cadastrado")
	ErrEstoqueSaldoInvalido   = errors.New("saldo deve ser maior ou igual a zero")
	ErrEstoqueDataObrigatoria = errors.New("data_snapshot é obrigatória")
	ErrEstoqueDataInvalida    = errors.New("data_snapshot inválida (use o formato AAAA-MM-DD)")
	ErrEstoqueDataFutura      = errors.New("data_snapshot não pode ser uma data futura")
)

// estoqueDataLayout é o formato aceito para o campo data_snapshot no payload
// de criação/edição manual de estoque (mesmo formato de DATE do MySQL).
const estoqueDataLayout = "2006-01-02"

// EstoqueFiltro agrupa os filtros opcionais aceitos por ListEstoque.
type EstoqueFiltro struct {
	SKU      string
	DataDe   string // formato AAAA-MM-DD (inclusive)
	DataAte  string // formato AAAA-MM-DD (inclusive)
	Ruptura  *bool
	OrderBy  string
	OrderDir string
	// Historico, quando true, retorna a série temporal completa (via
	// EstoqueRepository.List) em vez do padrão (última posição por SKU, via
	// EstoqueRepository.UltimaPosicaoPorSku).
	Historico bool
}

// EstoqueService agrega regras de negócio sobre estoque.
type EstoqueService struct {
	repo        *repositories.EstoqueRepository
	produtoRepo *repositories.ProdutoRepository
	Cfg         *config.Config
}

// NewEstoqueService cria um EstoqueService com pool de conexão injetado.
func NewEstoqueService(db *sql.DB, cfg *config.Config) *EstoqueService {
	return &EstoqueService{
		repo:        repositories.NewEstoqueRepository(),
		produtoRepo: repositories.NewProdutoRepository(),
		Cfg:         cfg,
	}
}

// parseFiltroData converte uma string AAAA-MM-DD opcional em *time.Time.
// Retorna nil se vazia. Retorna ErrEstoqueDataInvalida se malformada.
func parseFiltroData(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(estoqueDataLayout, s)
	if err != nil {
		return nil, ErrEstoqueDataInvalida
	}
	return &t, nil
}

// ListEstoque pagina registros de estoque aplicando os filtros informados.
// Por padrão retorna apenas a última posição de cada SKU (respeitando o
// filtro de data, quando informado) — comportamento padrão da tela de
// estoque. Quando filtro.Historico=true, retorna a série temporal completa.
// page/limit são validados (limit max 100) no repositório.
func (s *EstoqueService) ListEstoque(ctx context.Context, db *sql.DB, page, limit int, filtro EstoqueFiltro) ([]models.Estoque, int, error) {
	dataDe, err := parseFiltroData(filtro.DataDe)
	if err != nil {
		return nil, 0, err
	}
	dataAte, err := parseFiltroData(filtro.DataAte)
	if err != nil {
		return nil, 0, err
	}

	if s.Cfg.Verbose {
		log.Printf("[estoque] list page=%d limit=%d sku=%q data_de=%q data_ate=%q ruptura=%v historico=%t",
			page, limit, filtro.SKU, filtro.DataDe, filtro.DataAte, filtro.Ruptura, filtro.Historico)
	}

	repoFiltro := repositories.EstoqueFiltro{
		SKU:      filtro.SKU,
		DataDe:   dataDe,
		DataAte:  dataAte,
		Ruptura:  filtro.Ruptura,
		OrderBy:  filtro.OrderBy,
		OrderDir: filtro.OrderDir,
	}

	if filtro.Historico {
		return s.repo.List(ctx, db, page, limit, repoFiltro)
	}
	return s.repo.UltimaPosicaoPorSku(ctx, db, page, limit, repoFiltro)
}

// GetEstoqueByID busca um registro de estoque por id. Retorna
// ErrEstoqueNaoEncontrado se não existir.
func (s *EstoqueService) GetEstoqueByID(ctx context.Context, db *sql.DB, id int64) (*models.Estoque, error) {
	e, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrEstoqueNaoEncontrado
		}
		return nil, err
	}
	return e, nil
}

// EstoqueInput agrupa os campos editáveis de um ajuste MANUAL de estoque,
// usados tanto na criação quanto na edição. ruptura NÃO é aceita como input
// — é sempre derivada de saldo <= 0 dentro do service (exigência do
// SecBrain 🟣).
type EstoqueInput struct {
	SKU          string
	DataSnapshot string // formato AAAA-MM-DD
	Saldo        int
}

// validarEstoqueInput aplica as validações comuns a criação e edição de
// ajustes manuais de estoque, normaliza os campos e resolve data_snapshot.
// Confere que o sku existe em produtos (rejeita sku inexistente) e que
// data_snapshot não é uma data futura.
func (s *EstoqueService) validarEstoqueInput(ctx context.Context, db *sql.DB, input EstoqueInput) (sku string, dataSnapshot time.Time, saldo int, err error) {
	sku = strings.TrimSpace(input.SKU)
	if sku == "" {
		err = ErrEstoqueSKUObrigatorio
		return
	}

	dataStr := strings.TrimSpace(input.DataSnapshot)
	if dataStr == "" {
		err = ErrEstoqueDataObrigatoria
		return
	}
	parsedData, parseErr := time.Parse(estoqueDataLayout, dataStr)
	if parseErr != nil {
		err = ErrEstoqueDataInvalida
		return
	}
	if parsedData.After(time.Now()) {
		err = ErrEstoqueDataFutura
		return
	}
	dataSnapshot = parsedData

	if input.Saldo < 0 {
		err = ErrEstoqueSaldoInvalido
		return
	}
	saldo = input.Saldo

	existe, existeErr := s.produtoRepo.ExistsBySKU(ctx, db, sku)
	if existeErr != nil {
		err = existeErr
		return
	}
	if !existe {
		err = ErrEstoqueSKUInexistente
		return
	}
	return
}

// CreateEstoque cria um ajuste manual de estoque, validando sku (deve
// existir em produtos), saldo (>= 0) e data_snapshot (não futura). ruptura é
// sempre derivada de saldo <= 0. origem é sempre gravada como
// models.EstoqueOrigemManual.
func (s *EstoqueService) CreateEstoque(ctx context.Context, db *sql.DB, input EstoqueInput) (*models.Estoque, error) {
	sku, dataSnapshot, saldo, err := s.validarEstoqueInput(ctx, db, input)
	if err != nil {
		return nil, err
	}

	e := &models.Estoque{
		SKU:          sku,
		DataSnapshot: dataSnapshot,
		Saldo:        saldo,
		Ruptura:      saldo <= 0,
		Origem:       models.EstoqueOrigemManual,
	}
	if err := s.repo.Create(ctx, db, e); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[estoque] criado manualmente: id=%d sku=%s data_snapshot=%s saldo=%d ruptura=%t",
			e.ID, e.SKU, dataSnapshot.Format(estoqueDataLayout), e.Saldo, e.Ruptura)
	}
	return e, nil
}

// UpdateEstoque atualiza um registro de estoque existente via ajuste MANUAL
// (sku e data_snapshot não são alterados por aqui — para mudar a chave de
// negócio, crie um novo registro). ruptura é sempre derivada de saldo <= 0.
// origem é sempre gravada como models.EstoqueOrigemManual. Retorna
// ErrEstoqueNaoEncontrado se não existir.
func (s *EstoqueService) UpdateEstoque(ctx context.Context, db *sql.DB, id int64, input EstoqueInput) (*models.Estoque, error) {
	atual, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrEstoqueNaoEncontrado
		}
		return nil, err
	}

	// sku e data_snapshot não são editáveis: valida usando os valores atuais
	// do registro, mas ainda assim exige saldo válido no payload.
	if input.Saldo < 0 {
		return nil, ErrEstoqueSaldoInvalido
	}

	e := &models.Estoque{
		Saldo:   input.Saldo,
		Ruptura: input.Saldo <= 0,
		Origem:  models.EstoqueOrigemManual,
	}
	if err := s.repo.Update(ctx, db, id, e); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrEstoqueNaoEncontrado
		}
		return nil, err
	}

	atualizado, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[estoque] atualizado manualmente: id=%d sku=%s saldo=%d ruptura=%t",
			id, atual.SKU, atualizado.Saldo, atualizado.Ruptura)
	}
	return atualizado, nil
}
