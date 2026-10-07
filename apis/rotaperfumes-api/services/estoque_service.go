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
	vlog.Printf("estoque_service.go", "parseFiltroData", "chamando strings.TrimSpace e atribuindo a s")
	s = strings.TrimSpace(s)
	vlog.Printf("estoque_service.go", "parseFiltroData", "verificando condição s == \"\"")
	if s == "" {
		return nil, nil
	}
	vlog.Printf("estoque_service.go", "parseFiltroData", "chamando time.ParseInLocation e declarando t, err")
	t, err := time.ParseInLocation(estoqueDataLayout, s, time.Local)
	vlog.Printf("estoque_service.go", "parseFiltroData", "verificando condição err != nil")
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
	vlog.Printf("estoque_service.go", "EstoqueService.ListEstoque", "chamando parseFiltroData e declarando dataDe, err")
	dataDe, err := parseFiltroData(filtro.DataDe)
	vlog.Printf("estoque_service.go", "EstoqueService.ListEstoque", "verificando condição err != nil")
	if err != nil {
		return nil, 0, err
	}
	vlog.Printf("estoque_service.go", "EstoqueService.ListEstoque", "chamando parseFiltroData e declarando dataAte, err")
	dataAte, err := parseFiltroData(filtro.DataAte)
	vlog.Printf("estoque_service.go", "EstoqueService.ListEstoque", "verificando condição err != nil")
	if err != nil {
		return nil, 0, err
	}

	if s.Cfg.Verbose {
		log.Printf("[estoque] list page=%d limit=%d sku=%q data_de=%q data_ate=%q ruptura=%v historico=%t",
			page, limit, filtro.SKU, filtro.DataDe, filtro.DataAte, filtro.Ruptura, filtro.Historico)
	}

	vlog.Printf("estoque_service.go", "EstoqueService.ListEstoque", "montando literal repositories.EstoqueFiltro e declarando repoFiltro")
	repoFiltro := repositories.EstoqueFiltro{
		SKU:      filtro.SKU,
		DataDe:   dataDe,
		DataAte:  dataAte,
		Ruptura:  filtro.Ruptura,
		OrderBy:  filtro.OrderBy,
		OrderDir: filtro.OrderDir,
	}

	vlog.Printf("estoque_service.go", "EstoqueService.ListEstoque", "verificando condição filtro.Historico")
	if filtro.Historico {
		return s.repo.List(ctx, db, page, limit, repoFiltro)
	}
	return s.repo.UltimaPosicaoPorSku(ctx, db, page, limit, repoFiltro)
}

// GetEstoqueByID busca um registro de estoque por id. Retorna
// ErrEstoqueNaoEncontrado se não existir.
func (s *EstoqueService) GetEstoqueByID(ctx context.Context, db *sql.DB, id int64) (*models.Estoque, error) {
	vlog.Printf("estoque_service.go", "EstoqueService.GetEstoqueByID", "chamando s.repo.GetByID e declarando e, err")
	e, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("estoque_service.go", "EstoqueService.GetEstoqueByID", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("estoque_service.go", "EstoqueService.GetEstoqueByID", "verificando condição errors.Is(err, repositories.ErrNotFound)")
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
	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "chamando strings.TrimSpace e atribuindo a sku")
	sku = strings.TrimSpace(input.SKU)
	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "verificando condição sku == \"\"")
	if sku == "" {
		vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "atribuindo ErrEstoqueSKUObrigatorio a err")
		err = ErrEstoqueSKUObrigatorio
		return
	}

	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "chamando strings.TrimSpace e declarando dataStr")
	dataStr := strings.TrimSpace(input.DataSnapshot)
	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "verificando condição dataStr == \"\"")
	if dataStr == "" {
		vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "atribuindo ErrEstoqueDataObrigatoria a err")
		err = ErrEstoqueDataObrigatoria
		return
	}
	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "chamando time.ParseInLocation e declarando parsedData, parseErr")
	parsedData, parseErr := time.ParseInLocation(estoqueDataLayout, dataStr, time.Local)
	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "verificando condição parseErr != nil")
	if parseErr != nil {
		vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "atribuindo ErrEstoqueDataInvalida a err")
		err = ErrEstoqueDataInvalida
		return
	}
	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "verificando condição parsedData.After(time.Now())")
	if parsedData.After(time.Now()) {
		vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "atribuindo ErrEstoqueDataFutura a err")
		err = ErrEstoqueDataFutura
		return
	}
	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "atribuindo parsedData a dataSnapshot")
	dataSnapshot = parsedData

	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "verificando condição input.Saldo < 0")
	if input.Saldo < 0 {
		vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "atribuindo ErrEstoqueSaldoInvalido a err")
		err = ErrEstoqueSaldoInvalido
		return
	}
	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "atribuindo input.Saldo a saldo")
	saldo = input.Saldo

	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "chamando s.produtoRepo.ExistsBySKU e declarando existe, existeErr")
	existe, existeErr := s.produtoRepo.ExistsBySKU(ctx, db, sku)
	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "verificando condição existeErr != nil")
	if existeErr != nil {
		vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "atribuindo existeErr a err")
		err = existeErr
		return
	}
	vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "verificando condição !existe")
	if !existe {
		vlog.Printf("estoque_service.go", "EstoqueService.validarEstoqueInput", "atribuindo ErrEstoqueSKUInexistente a err")
		err = ErrEstoqueSKUInexistente
		return
	}
	return
}

// CreateEstoque cria um ajuste manual de estoque, validando sku (deve
// existir em produtos), saldo (>= 0) e data_snapshot (não futura). ruptura é
// sempre derivada de saldo <= 0.
func (s *EstoqueService) CreateEstoque(ctx context.Context, db *sql.DB, input EstoqueInput) (*models.Estoque, error) {
	vlog.Printf("estoque_service.go", "EstoqueService.CreateEstoque", "chamando s.validarEstoqueInput e declarando sku, dataSnapshot, saldo, err")
	sku, dataSnapshot, saldo, err := s.validarEstoqueInput(ctx, db, input)
	vlog.Printf("estoque_service.go", "EstoqueService.CreateEstoque", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("estoque_service.go", "EstoqueService.CreateEstoque", "montando &models.Estoque e declarando e")
	e := &models.Estoque{
		SKU:          sku,
		DataSnapshot: dataSnapshot,
		Saldo:        saldo,
		Ruptura:      saldo <= 0,
	}
	vlog.Printf("estoque_service.go", "EstoqueService.CreateEstoque", "chamando s.repo.Create e declarando err e verificando condição err != nil")
	if err := s.repo.Create(ctx, db, e); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[estoque] criado manualmente: id=%d sku=%s data_snapshot=%s saldo=%d ruptura=%t",
			e.ID, e.SKU, dataSnapshot.Format(estoqueDataLayout), e.Saldo, e.Ruptura)
	}
	return s.relerEstoqueCriado(ctx, db, e), nil
}

// relerEstoqueCriado relê do banco o registro de estoque recém-gravado
// (BUG-09), para devolver created_at/updated_at (e demais campos com default)
// preenchidos pelo MySQL. O INSERT já foi confirmado: se a releitura falhar,
// loga e devolve o objeto em memória (timestamps zerados) em vez de responder
// erro para uma gravação que deu certo.
func (s *EstoqueService) relerEstoqueCriado(ctx context.Context, db *sql.DB, e *models.Estoque) *models.Estoque {
	vlog.Printf("estoque_service.go", "EstoqueService.relerEstoqueCriado", "chamando s.repo.GetByID e declarando gravado, err")
	gravado, err := s.repo.GetByID(ctx, db, e.ID)
	vlog.Printf("estoque_service.go", "EstoqueService.relerEstoqueCriado", "verificando condição err != nil")
	if err != nil {
		log.Printf("[estoque] criado, mas falhou a releitura: id=%d: %v", e.ID, err)
		return e
	}
	return gravado
}

// UpdateEstoque atualiza um registro de estoque existente via ajuste MANUAL
// (sku e data_snapshot não são alterados por aqui — para mudar a chave de
// negócio, crie um novo registro). ruptura é sempre derivada de saldo <= 0.
// Retorna ErrEstoqueNaoEncontrado se não existir.
func (s *EstoqueService) UpdateEstoque(ctx context.Context, db *sql.DB, id int64, input EstoqueInput) (*models.Estoque, error) {
	vlog.Printf("estoque_service.go", "EstoqueService.UpdateEstoque", "chamando s.repo.GetByID e declarando atual, err")
	atual, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("estoque_service.go", "EstoqueService.UpdateEstoque", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("estoque_service.go", "EstoqueService.UpdateEstoque", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrEstoqueNaoEncontrado
		}
		return nil, err
	}

	// sku e data_snapshot não são editáveis: valida usando os valores atuais
	// do registro, mas ainda assim exige saldo válido no payload.
	vlog.Printf("estoque_service.go", "EstoqueService.UpdateEstoque", "verificando condição input.Saldo < 0")
	if input.Saldo < 0 {
		return nil, ErrEstoqueSaldoInvalido
	}

	vlog.Printf("estoque_service.go", "EstoqueService.UpdateEstoque", "montando &models.Estoque e declarando e")
	e := &models.Estoque{
		Saldo:   input.Saldo,
		Ruptura: input.Saldo <= 0,
	}
	vlog.Printf("estoque_service.go", "EstoqueService.UpdateEstoque", "chamando s.repo.Update e declarando err e verificando condição err != nil")
	if err := s.repo.Update(ctx, db, id, e); err != nil {
		vlog.Printf("estoque_service.go", "EstoqueService.UpdateEstoque", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrEstoqueNaoEncontrado
		}
		return nil, err
	}

	vlog.Printf("estoque_service.go", "EstoqueService.UpdateEstoque", "chamando s.repo.GetByID e declarando atualizado, err")
	atualizado, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("estoque_service.go", "EstoqueService.UpdateEstoque", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[estoque] atualizado manualmente: id=%d sku=%s saldo=%d ruptura=%t",
			id, atual.SKU, atualizado.Saldo, atualizado.Ruptura)
	}
	return atualizado, nil
}
