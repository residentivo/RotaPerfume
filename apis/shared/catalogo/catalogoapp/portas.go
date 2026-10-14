// Package catalogoapp tem os casos de uso do Catálogo (ARQ-02) e as portas
// que eles consomem. As portas são declaradas aqui, pelo consumidor; o
// adapter catalogomysql as satisfaz implicitamente, sem importar este pacote
// (docs/arquitetura-ddd.md §4.1).
package catalogoapp

import (
	"context"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/kernel"
)

// Produtos é o repositório de produtos.
type Produtos interface {
	Listar(ctx context.Context, f catalogo.FiltroProduto, p kernel.Pagina) (kernel.Resultado[catalogo.Produto], error)
	// PorID devolve catalogo.ErrProdutoNaoEncontrado se o id não existir.
	PorID(ctx context.Context, id int64) (*catalogo.Produto, error)
	ExisteSKU(ctx context.Context, sku catalogo.SKU) (bool, error)
	// Inserir grava o produto e preenche p.ID.
	Inserir(ctx context.Context, p *catalogo.Produto) error
	// Atualizar e DefinirAtivo devolvem catalogo.ErrProdutoNaoEncontrado se
	// nenhuma linha casar com o id.
	Atualizar(ctx context.Context, id int64, c catalogo.CamposProduto) error
	DefinirAtivo(ctx context.Context, id int64, ativo bool) error
}

// Estoques é o repositório de registros de estoque.
type Estoques interface {
	// Historico devolve a série temporal completa que casa com o filtro.
	Historico(ctx context.Context, f catalogo.FiltroEstoque, p kernel.Pagina) (kernel.Resultado[catalogo.RegistroEstoque], error)
	// UltimaPosicao devolve só o registro mais recente de cada SKU.
	UltimaPosicao(ctx context.Context, f catalogo.FiltroEstoque, p kernel.Pagina) (kernel.Resultado[catalogo.RegistroEstoque], error)
	// PorID devolve catalogo.ErrEstoqueNaoEncontrado se o id não existir.
	PorID(ctx context.Context, id int64) (*catalogo.RegistroEstoque, error)
	// Inserir grava o registro e preenche e.ID.
	Inserir(ctx context.Context, e *catalogo.RegistroEstoque) error
	// AtualizarSaldo devolve catalogo.ErrEstoqueNaoEncontrado se nenhuma
	// linha casar com o id.
	AtualizarSaldo(ctx context.Context, id int64, saldo int, ruptura bool) error
}
