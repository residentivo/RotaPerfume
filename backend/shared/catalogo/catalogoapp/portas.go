// Package catalogoapp tem os casos de uso do Catálogo (ARQ-02) e as portas
// que eles consomem. As portas são declaradas aqui, pelo consumidor; o
// adapter catalogomysql as satisfaz implicitamente, sem importar este pacote
// (docs/arquitetura-ddd.md §4.1).
package catalogoapp

import (
	"context"
	"io"
	"time"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/kernel"
)

// Produtos é o repositório de produtos.
type Produtos interface {
	Listar(ctx context.Context, f catalogo.FiltroProduto, p kernel.Pagina) (kernel.Resultado[catalogo.Produto], error)
	// PorID devolve catalogo.ErrProdutoNaoEncontrado se o id não existir.
	PorID(ctx context.Context, id int64) (*catalogo.Produto, error)
	ExisteSKU(ctx context.Context, sku catalogo.SKU) (bool, error)
	// Inserir grava o produto e preenche p.ID; catalogo.ErrSKUDuplicado se
	// o SKU já existir (uk_produtos_sku).
	Inserir(ctx context.Context, p *catalogo.Produto) error
	// Atualizar e DefinirAtivo devolvem catalogo.ErrProdutoNaoEncontrado se
	// nenhuma linha casar com o id.
	Atualizar(ctx context.Context, id int64, c catalogo.CamposProduto) error
	DefinirAtivo(ctx context.Context, id int64, ativo bool) error
	// AlternarAtivo inverte o status no próprio UPDATE (BUG-17: atômico sob
	// concorrência); catalogo.ErrProdutoNaoEncontrado sem linha.
	AlternarAtivo(ctx context.Context, id int64) error
}

// Estoques é o repositório de registros de estoque.
type Estoques interface {
	// Historico devolve a série temporal completa que casa com o filtro.
	Historico(ctx context.Context, f catalogo.FiltroEstoque, p kernel.Pagina) (kernel.Resultado[catalogo.RegistroEstoque], error)
	// UltimaPosicao devolve só o registro mais recente de cada SKU.
	UltimaPosicao(ctx context.Context, f catalogo.FiltroEstoque, p kernel.Pagina) (kernel.Resultado[catalogo.RegistroEstoque], error)
	// PorID devolve catalogo.ErrEstoqueNaoEncontrado se o id não existir.
	PorID(ctx context.Context, id int64) (*catalogo.RegistroEstoque, error)
	// Inserir grava o registro e preenche e.ID; catalogo.ErrEstoqueDuplicado
	// se já houver registro do sku na data (uk_estoque_data_sku).
	Inserir(ctx context.Context, e *catalogo.RegistroEstoque) error
	// AtualizarSaldo devolve catalogo.ErrEstoqueNaoEncontrado se nenhuma
	// linha casar com o id.
	AtualizarSaldo(ctx context.Context, id int64, saldo int, ruptura bool) error
}

// ArmazenamentoImagens guarda os arquivos de imagem de produto (IMG-01;
// adapter catalogofs, ligado só no bootstrap). O adapter gera o nome
// (<uuid-v4><extensao>) e recusa, sem tocar o disco, nomes fora do padrão
// em Remover/Abrir.
type ArmazenamentoImagens interface {
	// Gravar escreve de forma atômica e devolve o nome gerado.
	Gravar(ctx context.Context, extensao string, dados []byte) (string, error)
	// Remover apaga o arquivo; inexistente não é erro.
	Remover(ctx context.Context, nome string) error
	// Abrir devolve o arquivo e a data de modificação;
	// catalogo.ErrImagemNaoEncontrada se não houver arquivo regular.
	Abrir(nome string) (io.ReadSeekCloser, time.Time, error)
}

// NormalizadorImagens valida o conteúdo (magic bytes, dimensões, decode
// completo) e reencoda JPEG/PNG (adapter catalogoimagem). Devolve
// catalogo.ErrImagemInvalida ou catalogo.ErrImagemMuitoGrande.
type NormalizadorImagens interface {
	Normalizar(dados []byte) (catalogo.ImagemNormalizada, error)
}

// TxImagemProduto é o repositório ligado à tx da troca/remoção de imagem.
type TxImagemProduto struct {
	Produtos interface {
		// ImagemParaAtualizar lê a imagem atual com SELECT … FOR UPDATE
		// ("" = sem imagem); catalogo.ErrProdutoNaoEncontrado sem linha.
		ImagemParaAtualizar(ctx context.Context, id int64) (string, error)
		// DefinirImagem grava o nome ("" grava NULL).
		DefinirImagem(ctx context.Context, id int64, nome string) error
	}
}

// Transactor é o Unit of Work (db.Transactor satisfaz).
type Transactor[R any] interface {
	WithTx(ctx context.Context, fn func(R) error) error
}

// Imagens são as dependências dos casos de uso de imagem de produto.
type Imagens struct {
	Armazenamento ArmazenamentoImagens
	Normalizador  NormalizadorImagens
	Tx            Transactor[TxImagemProduto]
}
