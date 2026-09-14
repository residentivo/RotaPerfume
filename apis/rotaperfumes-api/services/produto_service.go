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
	ErrProdutoNaoEncontrado   = errors.New("produto não encontrado")
	ErrSKUObrigatorio         = errors.New("sku é obrigatório")
	ErrDescricaoObrigatoria   = errors.New("descrição é obrigatória")
	ErrCategoriaObrigatoria   = errors.New("categoria é obrigatória")
	ErrMarcaObrigatoria       = errors.New("marca é obrigatória")
	ErrUnidadeObrigatoria     = errors.New("unidade é obrigatória")
	ErrPrecoTabelaInvalido    = errors.New("preco_tabela deve ser maior ou igual a zero")
	ErrCustoUnitarioInvalido  = errors.New("custo_unitario deve ser maior ou igual a zero")
	ErrDataLancamentoInvalida = errors.New("data_lancamento inválida (use o formato AAAA-MM-DD)")
)

// dataLancamentoLayout é o formato aceito para o campo data_lancamento no
// payload de criação/edição de produtos (mesmo formato de DATE do MySQL).
const dataLancamentoLayout = "2006-01-02"

// ProdutoFiltro agrupa os filtros opcionais aceitos por ListProdutos.
type ProdutoFiltro struct {
	Categoria string
	Marca     string
	Ativo     *bool
	Q         string
}

// ProdutoService agrega regras de negócio sobre produtos.
type ProdutoService struct {
	repo *repositories.ProdutoRepository
	Cfg  *config.Config
}

// NewProdutoService cria um ProdutoService com pool de conexão injetado.
func NewProdutoService(db *sql.DB, cfg *config.Config) *ProdutoService {
	return &ProdutoService{
		repo: repositories.NewProdutoRepository(),
		Cfg:  cfg,
	}
}

// ListProdutos pagina produtos aplicando os filtros informados.
// page/limit são validados (limit max 100) no repositório.
func (s *ProdutoService) ListProdutos(ctx context.Context, db *sql.DB, page, limit int, filtro ProdutoFiltro) ([]models.Produto, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[produtos] list page=%d limit=%d categoria=%q marca=%q ativo=%v q=%q",
			page, limit, filtro.Categoria, filtro.Marca, filtro.Ativo, filtro.Q)
	}
	repoFiltro := repositories.ProdutoFiltro{
		Categoria: filtro.Categoria,
		Marca:     filtro.Marca,
		Ativo:     filtro.Ativo,
		Q:         filtro.Q,
	}
	return s.repo.List(ctx, db, page, limit, repoFiltro)
}

// GetProdutoByID busca um produto por id. Retorna ErrProdutoNaoEncontrado se não existir.
func (s *ProdutoService) GetProdutoByID(ctx context.Context, db *sql.DB, id int64) (*models.Produto, error) {
	p, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrProdutoNaoEncontrado
		}
		return nil, err
	}
	return p, nil
}

// ToggleAtivoProduto ativa/inativa um produto (exclusão lógica). Se ativo
// for nil, inverte o status atual (toggle). Retorna ErrProdutoNaoEncontrado
// se não existir.
func (s *ProdutoService) ToggleAtivoProduto(ctx context.Context, db *sql.DB, id int64, ativo *bool) (*models.Produto, error) {
	p, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrProdutoNaoEncontrado
		}
		return nil, err
	}
	newAtivo := !p.Ativo
	if ativo != nil {
		newAtivo = *ativo
	}
	if err := s.repo.SetAtivo(ctx, db, id, newAtivo); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrProdutoNaoEncontrado
		}
		return nil, err
	}
	p.Ativo = newAtivo
	if s.Cfg.Verbose {
		log.Printf("[produtos] ativo toggle: id=%d ativo=%t", id, newAtivo)
	}
	return p, nil
}

// ProdutoInput agrupa os campos editáveis de um produto, usados tanto na
// criação quanto na edição.
type ProdutoInput struct {
	SKU            string
	Descricao      string
	Categoria      string
	Marca          string
	NotaOlfativa   string
	PrecoTabela    float64
	CustoUnitario  float64
	Unidade        string
	DataLancamento string // formato AAAA-MM-DD; vazio = NULL (opcional)
}

// validarProdutoInput aplica as validações comuns a criação e edição,
// normaliza os campos (trim) e resolve data_lancamento (opcional).
// requireSKU controla se o campo sku é validado como obrigatório: na
// criação sim; na edição o sku não é editável (não vem no payload), então
// a validação é pulada.
func validarProdutoInput(input ProdutoInput, requireSKU bool) (sku, descricao, categoria, marca, notaOlfativa, unidade string, precoTabela, custoUnitario float64, dataLancamento *time.Time, err error) {
	sku = strings.TrimSpace(input.SKU)
	descricao = strings.TrimSpace(input.Descricao)
	categoria = strings.TrimSpace(input.Categoria)
	marca = strings.TrimSpace(input.Marca)
	notaOlfativa = strings.TrimSpace(input.NotaOlfativa)
	unidade = strings.TrimSpace(input.Unidade)
	dataLancamentoStr := strings.TrimSpace(input.DataLancamento)

	if requireSKU && sku == "" {
		err = ErrSKUObrigatorio
		return
	}
	if descricao == "" {
		err = ErrDescricaoObrigatoria
		return
	}
	if categoria == "" {
		err = ErrCategoriaObrigatoria
		return
	}
	if marca == "" {
		err = ErrMarcaObrigatoria
		return
	}
	if unidade == "" {
		err = ErrUnidadeObrigatoria
		return
	}
	if input.PrecoTabela < 0 {
		err = ErrPrecoTabelaInvalido
		return
	}
	if input.CustoUnitario < 0 {
		err = ErrCustoUnitarioInvalido
		return
	}
	precoTabela = input.PrecoTabela
	custoUnitario = input.CustoUnitario

	if dataLancamentoStr != "" {
		t, parseErr := time.Parse(dataLancamentoLayout, dataLancamentoStr)
		if parseErr != nil {
			err = ErrDataLancamentoInvalida
			return
		}
		dataLancamento = &t
	}
	return
}

// CreateProduto cria um novo produto, validando os campos obrigatórios.
// Ativo é sempre TRUE por padrão na criação (mesma regra dos itens
// importados via CSV).
func (s *ProdutoService) CreateProduto(ctx context.Context, db *sql.DB, input ProdutoInput) (*models.Produto, error) {
	sku, descricao, categoria, marca, notaOlfativa, unidade, precoTabela, custoUnitario, dataLancamento, err := validarProdutoInput(input, true)
	if err != nil {
		return nil, err
	}

	p := &models.Produto{
		SKU:            sku,
		Descricao:      descricao,
		Categoria:      categoria,
		Marca:          marca,
		NotaOlfativa:   notaOlfativa,
		PrecoTabela:    precoTabela,
		CustoUnitario:  custoUnitario,
		Unidade:        unidade,
		DataLancamento: dataLancamento,
		Ativo:          true,
	}
	if err := s.repo.Create(ctx, db, p); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[produtos] criado: id=%d sku=%s descricao=%s", p.ID, p.SKU, p.Descricao)
	}
	return p, nil
}

// UpdateProduto atualiza os campos editáveis de um produto existente
// (sku e ativo não são alterados por aqui).
// Retorna ErrProdutoNaoEncontrado se não existir.
func (s *ProdutoService) UpdateProduto(ctx context.Context, db *sql.DB, id int64, input ProdutoInput) (*models.Produto, error) {
	_, descricao, categoria, marca, notaOlfativa, unidade, precoTabela, custoUnitario, dataLancamento, err := validarProdutoInput(input, false)
	if err != nil {
		return nil, err
	}

	p := &models.Produto{
		Descricao:      descricao,
		Categoria:      categoria,
		Marca:          marca,
		NotaOlfativa:   notaOlfativa,
		PrecoTabela:    precoTabela,
		CustoUnitario:  custoUnitario,
		Unidade:        unidade,
		DataLancamento: dataLancamento,
	}
	if err := s.repo.Update(ctx, db, id, p); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrProdutoNaoEncontrado
		}
		return nil, err
	}

	atualizado, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[produtos] atualizado: id=%d descricao=%s", id, descricao)
	}
	return atualizado, nil
}
