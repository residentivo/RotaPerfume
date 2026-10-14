package services_test

import (
	"context"
	"database/sql"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/kernel"
)

// ARQ-02 (§7.2-3): os testes de ProdutoService/EstoqueService exercitam os
// casos de uso reais (catalogoapp) sobre os adapters reais (catalogomysql)
// ligados ao db do sqlmock. O SQL dos adapters é o do legado, então as
// expectativas (regex) continuam valendo; os nomes de teste são mantidos.
func novosCasosCatalogo(db *sql.DB) *catalogoapp.Casos {
	return catalogoapp.Novo(catalogomysql.NovoProdutos(db), catalogomysql.NovoEstoques(db))
}

// listarProdutos chama ListarProdutos e devolve no formato do legado.
func listarProdutos(c *catalogoapp.Casos, page, limit int, f catalogo.FiltroProduto) ([]catalogo.Produto, int, error) {
	res, err := c.ListarProdutos(context.Background(), f, kernel.NovaPagina(page, limit))
	return res.Itens, res.Total, err
}

// listarEstoque chama ListarEstoque e devolve no formato do legado.
func listarEstoque(c *catalogoapp.Casos, page, limit int, q catalogoapp.ConsultaEstoque) ([]catalogo.RegistroEstoque, int, error) {
	res, err := c.ListarEstoque(context.Background(), q, kernel.NovaPagina(page, limit))
	return res.Itens, res.Total, err
}
