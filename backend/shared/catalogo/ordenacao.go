package catalogo

import "github.com/rotaperfumes/shared/kernel"

// CampoProduto é o enum de campos de ordenação de produtos (C-02). O valor é
// o nome aceito em ?order_by=; a coluna SQL sai de um switch no adapter.
type CampoProduto string

// Campos de ordenação de produtos.
const (
	OrdemProdutoID             CampoProduto = "id"
	OrdemProdutoSKU            CampoProduto = "sku"
	OrdemProdutoDescricao      CampoProduto = "descricao"
	OrdemProdutoCategoria      CampoProduto = "categoria"
	OrdemProdutoMarca          CampoProduto = "marca"
	OrdemProdutoPrecoTabela    CampoProduto = "preco_tabela"
	OrdemProdutoCustoUnitario  CampoProduto = "custo_unitario"
	OrdemProdutoDataLancamento CampoProduto = "data_lancamento"
	OrdemProdutoAtivo          CampoProduto = "ativo"
	OrdemProdutoCriadoEm       CampoProduto = "created_at"
	OrdemProdutoAtualizadoEm   CampoProduto = "updated_at"
)

// CamposOrdenacaoProduto devolve a whitelist de campos de produto.
func CamposOrdenacaoProduto() []CampoProduto {
	return []CampoProduto{
		OrdemProdutoID, OrdemProdutoSKU, OrdemProdutoDescricao, OrdemProdutoCategoria,
		OrdemProdutoMarca, OrdemProdutoPrecoTabela, OrdemProdutoCustoUnitario,
		OrdemProdutoDataLancamento, OrdemProdutoAtivo, OrdemProdutoCriadoEm, OrdemProdutoAtualizadoEm,
	}
}

// OrdenacaoProdutoPadrao é "id ASC" (comportamento de sempre).
func OrdenacaoProdutoPadrao() kernel.Ordenacao[CampoProduto] {
	return kernel.NovaOrdenacao(OrdemProdutoID, kernel.Asc)
}

// CampoEstoque é o enum de campos de ordenação de estoque (C-02). O adapter
// tem duas traduções (com alias "e." na série histórica e sem alias sobre a
// tabela derivada da última posição), ambas por switch.
type CampoEstoque string

// Campos de ordenação de estoque.
const (
	OrdemEstoqueID           CampoEstoque = "id"
	OrdemEstoqueDataSnapshot CampoEstoque = "data_snapshot"
	OrdemEstoqueSKU          CampoEstoque = "sku"
	OrdemEstoqueSaldo        CampoEstoque = "saldo"
	OrdemEstoqueRuptura      CampoEstoque = "ruptura"
	OrdemEstoqueCriadoEm     CampoEstoque = "created_at"
	OrdemEstoqueAtualizadoEm CampoEstoque = "updated_at"
)

// CamposOrdenacaoEstoque devolve a whitelist de campos de estoque.
func CamposOrdenacaoEstoque() []CampoEstoque {
	return []CampoEstoque{
		OrdemEstoqueID, OrdemEstoqueDataSnapshot, OrdemEstoqueSKU, OrdemEstoqueSaldo,
		OrdemEstoqueRuptura, OrdemEstoqueCriadoEm, OrdemEstoqueAtualizadoEm,
	}
}

// OrdenacaoEstoquePadrao é "data_snapshot DESC" (última posição primeiro).
func OrdenacaoEstoquePadrao() kernel.Ordenacao[CampoEstoque] {
	return kernel.NovaOrdenacao(OrdemEstoqueDataSnapshot, kernel.Desc)
}
