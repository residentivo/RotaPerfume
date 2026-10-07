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
	OrderBy   string
	OrderDir  string
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
	vlog.Printf("produto_service.go", "ProdutoService.ListProdutos", "montando literal repositories.ProdutoFiltro e declarando repoFiltro")
	repoFiltro := repositories.ProdutoFiltro{
		Categoria: filtro.Categoria,
		Marca:     filtro.Marca,
		Ativo:     filtro.Ativo,
		Q:         filtro.Q,
		OrderBy:   filtro.OrderBy,
		OrderDir:  filtro.OrderDir,
	}
	return s.repo.List(ctx, db, page, limit, repoFiltro)
}

// GetProdutoByID busca um produto por id. Retorna ErrProdutoNaoEncontrado se não existir.
func (s *ProdutoService) GetProdutoByID(ctx context.Context, db *sql.DB, id int64) (*models.Produto, error) {
	vlog.Printf("produto_service.go", "ProdutoService.GetProdutoByID", "chamando s.repo.GetByID e declarando p, err")
	p, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("produto_service.go", "ProdutoService.GetProdutoByID", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("produto_service.go", "ProdutoService.GetProdutoByID", "verificando condição errors.Is(err, repositories.ErrNotFound)")
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
	vlog.Printf("produto_service.go", "ProdutoService.ToggleAtivoProduto", "chamando s.repo.GetByID e declarando p, err")
	p, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("produto_service.go", "ProdutoService.ToggleAtivoProduto", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("produto_service.go", "ProdutoService.ToggleAtivoProduto", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrProdutoNaoEncontrado
		}
		return nil, err
	}
	vlog.Printf("produto_service.go", "ProdutoService.ToggleAtivoProduto", "declarando newAtivo com !p.Ativo")
	newAtivo := !p.Ativo
	vlog.Printf("produto_service.go", "ProdutoService.ToggleAtivoProduto", "verificando condição ativo != nil")
	if ativo != nil {
		vlog.Printf("produto_service.go", "ProdutoService.ToggleAtivoProduto", "atribuindo *ativo a newAtivo")
		newAtivo = *ativo
	}
	vlog.Printf("produto_service.go", "ProdutoService.ToggleAtivoProduto", "chamando s.repo.SetAtivo e declarando err e verificando condição err != nil")
	if err := s.repo.SetAtivo(ctx, db, id, newAtivo); err != nil {
		vlog.Printf("produto_service.go", "ProdutoService.ToggleAtivoProduto", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrProdutoNaoEncontrado
		}
		return nil, err
	}
	vlog.Printf("produto_service.go", "ProdutoService.ToggleAtivoProduto", "atribuindo newAtivo a p.Ativo")
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
	vlog.Printf("produto_service.go", "validarProdutoInput", "chamando strings.TrimSpace e atribuindo a sku")
	sku = strings.TrimSpace(input.SKU)
	vlog.Printf("produto_service.go", "validarProdutoInput", "chamando strings.TrimSpace e atribuindo a descricao")
	descricao = strings.TrimSpace(input.Descricao)
	vlog.Printf("produto_service.go", "validarProdutoInput", "chamando strings.TrimSpace e atribuindo a categoria")
	categoria = strings.TrimSpace(input.Categoria)
	vlog.Printf("produto_service.go", "validarProdutoInput", "chamando strings.TrimSpace e atribuindo a marca")
	marca = strings.TrimSpace(input.Marca)
	vlog.Printf("produto_service.go", "validarProdutoInput", "chamando strings.TrimSpace e atribuindo a notaOlfativa")
	notaOlfativa = strings.TrimSpace(input.NotaOlfativa)
	vlog.Printf("produto_service.go", "validarProdutoInput", "chamando strings.TrimSpace e atribuindo a unidade")
	unidade = strings.TrimSpace(input.Unidade)
	vlog.Printf("produto_service.go", "validarProdutoInput", "chamando strings.TrimSpace e declarando dataLancamentoStr")
	dataLancamentoStr := strings.TrimSpace(input.DataLancamento)

	vlog.Printf("produto_service.go", "validarProdutoInput", "verificando condição requireSKU && sku == \"\"")
	if requireSKU && sku == "" {
		vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo ErrSKUObrigatorio a err")
		err = ErrSKUObrigatorio
		return
	}
	vlog.Printf("produto_service.go", "validarProdutoInput", "verificando condição descricao == \"\"")
	if descricao == "" {
		vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo ErrDescricaoObrigatoria a err")
		err = ErrDescricaoObrigatoria
		return
	}
	vlog.Printf("produto_service.go", "validarProdutoInput", "verificando condição categoria == \"\"")
	if categoria == "" {
		vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo ErrCategoriaObrigatoria a err")
		err = ErrCategoriaObrigatoria
		return
	}
	vlog.Printf("produto_service.go", "validarProdutoInput", "verificando condição marca == \"\"")
	if marca == "" {
		vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo ErrMarcaObrigatoria a err")
		err = ErrMarcaObrigatoria
		return
	}
	vlog.Printf("produto_service.go", "validarProdutoInput", "verificando condição unidade == \"\"")
	if unidade == "" {
		vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo ErrUnidadeObrigatoria a err")
		err = ErrUnidadeObrigatoria
		return
	}
	vlog.Printf("produto_service.go", "validarProdutoInput", "verificando condição input.PrecoTabela < 0")
	if input.PrecoTabela < 0 {
		vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo ErrPrecoTabelaInvalido a err")
		err = ErrPrecoTabelaInvalido
		return
	}
	vlog.Printf("produto_service.go", "validarProdutoInput", "verificando condição input.CustoUnitario < 0")
	if input.CustoUnitario < 0 {
		vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo ErrCustoUnitarioInvalido a err")
		err = ErrCustoUnitarioInvalido
		return
	}
	vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo input.PrecoTabela a precoTabela")
	precoTabela = input.PrecoTabela
	vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo input.CustoUnitario a custoUnitario")
	custoUnitario = input.CustoUnitario

	vlog.Printf("produto_service.go", "validarProdutoInput", "verificando condição dataLancamentoStr != \"\"")
	if dataLancamentoStr != "" {
		vlog.Printf("produto_service.go", "validarProdutoInput", "chamando time.ParseInLocation e declarando t, parseErr")
		t, parseErr := time.ParseInLocation(dataLancamentoLayout, dataLancamentoStr, time.Local)
		vlog.Printf("produto_service.go", "validarProdutoInput", "verificando condição parseErr != nil")
		if parseErr != nil {
			vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo ErrDataLancamentoInvalida a err")
			err = ErrDataLancamentoInvalida
			return
		}
		vlog.Printf("produto_service.go", "validarProdutoInput", "atribuindo &t a dataLancamento")
		dataLancamento = &t
	}
	return
}

// CreateProduto cria um novo produto, validando os campos obrigatórios.
// Ativo é sempre TRUE por padrão na criação (mesma regra dos itens
// importados via CSV).
func (s *ProdutoService) CreateProduto(ctx context.Context, db *sql.DB, input ProdutoInput) (*models.Produto, error) {
	vlog.Printf("produto_service.go", "ProdutoService.CreateProduto", "chamando validarProdutoInput e declarando sku, descricao, categoria, marca, notaOlfativa, unidade, precoTabela, custoUnitario, dataLancamento, err")
	sku, descricao, categoria, marca, notaOlfativa, unidade, precoTabela, custoUnitario, dataLancamento, err := validarProdutoInput(input, true)
	vlog.Printf("produto_service.go", "ProdutoService.CreateProduto", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("produto_service.go", "ProdutoService.CreateProduto", "montando &models.Produto e declarando p")
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
	vlog.Printf("produto_service.go", "ProdutoService.CreateProduto", "chamando s.repo.Create e declarando err e verificando condição err != nil")
	if err := s.repo.Create(ctx, db, p); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[produtos] criado: id=%d sku=%s descricao=%s", p.ID, p.SKU, p.Descricao)
	}
	return s.relerProdutoCriado(ctx, db, p), nil
}

// relerProdutoCriado relê do banco o produto recém-gravado (BUG-09), para
// devolver created_at/updated_at preenchidos pelo MySQL. O INSERT já foi
// confirmado: se a releitura falhar, loga e devolve o objeto em memória
// (timestamps zerados) em vez de responder erro para uma gravação que deu
// certo — um retry esbarraria no SKU duplicado.
func (s *ProdutoService) relerProdutoCriado(ctx context.Context, db *sql.DB, p *models.Produto) *models.Produto {
	vlog.Printf("produto_service.go", "ProdutoService.relerProdutoCriado", "chamando s.repo.GetByID e declarando gravado, err")
	gravado, err := s.repo.GetByID(ctx, db, p.ID)
	vlog.Printf("produto_service.go", "ProdutoService.relerProdutoCriado", "verificando condição err != nil")
	if err != nil {
		log.Printf("[produtos] criado, mas falhou a releitura: id=%d: %v", p.ID, err)
		return p
	}
	return gravado
}

// UpdateProduto atualiza os campos editáveis de um produto existente
// (sku e ativo não são alterados por aqui).
// Retorna ErrProdutoNaoEncontrado se não existir.
func (s *ProdutoService) UpdateProduto(ctx context.Context, db *sql.DB, id int64, input ProdutoInput) (*models.Produto, error) {
	vlog.Printf("produto_service.go", "ProdutoService.UpdateProduto", "chamando validarProdutoInput e declarando _, descricao, categoria, marca, notaOlfativa, unidade, precoTabela, custoUnitario, dataLancamento, err")
	_, descricao, categoria, marca, notaOlfativa, unidade, precoTabela, custoUnitario, dataLancamento, err := validarProdutoInput(input, false)
	vlog.Printf("produto_service.go", "ProdutoService.UpdateProduto", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("produto_service.go", "ProdutoService.UpdateProduto", "montando &models.Produto e declarando p")
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
	vlog.Printf("produto_service.go", "ProdutoService.UpdateProduto", "chamando s.repo.Update e declarando err e verificando condição err != nil")
	if err := s.repo.Update(ctx, db, id, p); err != nil {
		vlog.Printf("produto_service.go", "ProdutoService.UpdateProduto", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrProdutoNaoEncontrado
		}
		return nil, err
	}

	vlog.Printf("produto_service.go", "ProdutoService.UpdateProduto", "chamando s.repo.GetByID e declarando atualizado, err")
	atualizado, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("produto_service.go", "ProdutoService.UpdateProduto", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[produtos] atualizado: id=%d descricao=%s", id, descricao)
	}
	return atualizado, nil
}
