// Package catalogo é o domínio do contexto Catálogo (ARQ-02): Produto,
// RegistroEstoque, os VOs SKU e as ordenações tipadas (C-02).
//
// O pacote é puro (docs/arquitetura-ddd.md §4.1): importa só a stdlib e o
// kernel. Não loga, não conhece SQL nem HTTP, e não tem tags JSON (o DTO HTTP
// é que serializa).
package catalogo

import "github.com/rotaperfumes/shared/kernel"

// Erros de domínio do Catálogo. A mensagem é a resposta HTTP pública: as
// mesmas do legado (produto_service.go/estoque_service.go), sem valores.
var (
	ErrProdutoNaoEncontrado   = kernel.NovoErro(kernel.KindNaoEncontrado, "produto não encontrado")
	ErrSKUObrigatorio         = kernel.NovoErro(kernel.KindInvalido, "sku é obrigatório")
	ErrDescricaoObrigatoria   = kernel.NovoErro(kernel.KindInvalido, "descrição é obrigatória")
	ErrCategoriaObrigatoria   = kernel.NovoErro(kernel.KindInvalido, "categoria é obrigatória")
	ErrMarcaObrigatoria       = kernel.NovoErro(kernel.KindInvalido, "marca é obrigatória")
	ErrUnidadeObrigatoria     = kernel.NovoErro(kernel.KindInvalido, "unidade é obrigatória")
	ErrPrecoTabelaInvalido    = kernel.NovoErro(kernel.KindInvalido, "preco_tabela deve ser maior ou igual a zero")
	ErrCustoUnitarioInvalido  = kernel.NovoErro(kernel.KindInvalido, "custo_unitario deve ser maior ou igual a zero")
	ErrDataLancamentoInvalida = kernel.NovoErro(kernel.KindInvalido, "data_lancamento inválida (use o formato AAAA-MM-DD)")
	// ErrSKUDuplicado (CHORE-06): 1062 em uk_produtos_sku no INSERT, 409.
	ErrSKUDuplicado = kernel.NovoErro(kernel.KindConflito, "sku já cadastrado")

	ErrEstoqueNaoEncontrado   = kernel.NovoErro(kernel.KindNaoEncontrado, "registro de estoque não encontrado")
	ErrEstoqueSKUInexistente  = kernel.NovoErro(kernel.KindInvalido, "sku não corresponde a um produto cadastrado")
	ErrEstoqueSaldoInvalido   = kernel.NovoErro(kernel.KindInvalido, "saldo deve ser maior ou igual a zero")
	ErrEstoqueDataObrigatoria = kernel.NovoErro(kernel.KindInvalido, "data_snapshot é obrigatória")
	ErrEstoqueDataInvalida    = kernel.NovoErro(kernel.KindInvalido, "data_snapshot inválida (use o formato AAAA-MM-DD)")
	ErrEstoqueDataFutura      = kernel.NovoErro(kernel.KindInvalido, "data_snapshot não pode ser uma data futura")
	// ErrEstoqueDuplicado (CHORE-06): 1062 em uk_estoque_data_sku no INSERT, 409.
	ErrEstoqueDuplicado = kernel.NovoErro(kernel.KindConflito, "registro de estoque já cadastrado para este sku nesta data")
)
