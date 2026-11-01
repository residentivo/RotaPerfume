package repositories_test

// ARQ-02: caminhos de erro e whitelists completas dos adapters do Catálogo
// (catalogomysql), que os testes portados do legado não isolavam.

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/kernel"
)

func TestCatalogomysql_QuerierObrigatorio(t *testing.T) {
	assert.Panics(t, func() { catalogomysql.NovoProdutos(nil) })
	assert.Panics(t, func() { catalogomysql.NovoEstoques(nil) })
}

// TestCatalogomysql_ColunaProduto_TodosOsCampos: cada valor do enum vira a
// sua coluna (whitelist completa, C-02), nas duas direções.
func TestCatalogomysql_ColunaProduto_TodosOsCampos(t *testing.T) {
	colunas := map[catalogo.CampoProduto]string{
		catalogo.OrdemProdutoID: "id", catalogo.OrdemProdutoSKU: "sku", catalogo.OrdemProdutoDescricao: "descricao",
		catalogo.OrdemProdutoCategoria: "categoria", catalogo.OrdemProdutoMarca: "marca",
		catalogo.OrdemProdutoPrecoTabela: "preco_tabela", catalogo.OrdemProdutoCustoUnitario: "custo_unitario",
		catalogo.OrdemProdutoDataLancamento: "data_lancamento", catalogo.OrdemProdutoAtivo: "ativo",
		catalogo.OrdemProdutoCriadoEm: "created_at", catalogo.OrdemProdutoAtualizadoEm: "updated_at",
	}
	require.Len(t, colunas, len(catalogo.CamposOrdenacaoProduto()))
	for campo, col := range colunas {
		for _, d := range []kernel.Direcao{kernel.Asc, kernel.Desc} {
			t.Run(string(campo)+"/"+d.String(), func(t *testing.T) {
				db, mock, captured := newCapturingMock(t)
				mock.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
				mock.ExpectQuery(`FROM produtos`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
				_, _, err := listarProdutos(catalogomysql.NovoProdutos(db), context.Background(),
					catalogo.FiltroProduto{Ordenacao: kernel.NovaOrdenacao(campo, d)})
				require.NoError(t, err)
				want := "ORDER BY " + col + " ASC"
				if d == kernel.Desc {
					want = "ORDER BY " + col + " DESC"
				}
				assert.Equal(t, want, orderByDe(t, (*captured)[len(*captured)-1]))
			})
		}
	}
}

// TestCatalogomysql_ErrosDeEscrita: falhas de LastInsertId/RowsAffected
// viram erro de infra (500), nunca "não encontrado".
func TestCatalogomysql_ErrosDeEscrita(t *testing.T) {
	errDriver := errors.New("driver")
	ctx := context.Background()
	casos := []struct {
		nome  string
		sqlRe string
		chama func(db *sql.DB) error
	}{
		{"produto inserir lastInsertId", `INSERT INTO produtos`, func(db *sql.DB) error {
			return catalogomysql.NovoProdutos(db).Inserir(ctx, &catalogo.Produto{SKU: "S"})
		}},
		{"produto atualizar rowsAffected", `UPDATE produtos`, func(db *sql.DB) error {
			return catalogomysql.NovoProdutos(db).Atualizar(ctx, 1, catalogo.CamposProduto{})
		}},
		{"produto ativo rowsAffected", `UPDATE produtos SET ativo`, func(db *sql.DB) error {
			return catalogomysql.NovoProdutos(db).DefinirAtivo(ctx, 1, true)
		}},
		{"estoque inserir lastInsertId", `INSERT INTO estoque`, func(db *sql.DB) error {
			return catalogomysql.NovoEstoques(db).Inserir(ctx, &catalogo.RegistroEstoque{SKU: "S"})
		}},
		{"estoque saldo rowsAffected", `UPDATE estoque`, func(db *sql.DB) error {
			return catalogomysql.NovoEstoques(db).AtualizarSaldo(ctx, 1, 2, false)
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			mock.ExpectExec(c.sqlRe).WillReturnResult(sqlmock.NewErrorResult(errDriver))

			err := c.chama(db)
			assert.ErrorIs(t, err, errDriver)
			var de *kernel.Erro
			assert.False(t, errors.As(err, &de), "falha de driver não é erro de domínio")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestCatalogomysql_ErroDeScanNaListagem: uma linha que não converte aborta
// a listagem com erro (sem devolver página parcial).
func TestCatalogomysql_ErroDeScanNaListagem(t *testing.T) {
	ctx := context.Background()
	t.Run("produtos", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
		mock.ExpectQuery(`FROM produtos`).WillReturnRows(sqlmock.NewRows(produtoColumns).
			AddRow("não-numérico", "S", "D", "C", "M", "", 1.0, 1.0, "UN", nil, true, "", nil, nil))
		itens, _, err := listarProdutos(catalogomysql.NovoProdutos(db), ctx, catalogo.FiltroProduto{})
		assert.Error(t, err)
		assert.Nil(t, itens)
	})
	t.Run("estoque", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
		mock.ExpectQuery(`FROM estoque`).WillReturnRows(sqlmock.NewRows(estoqueColumns).
			AddRow(int64(1), "não-é-data", "S", "D", 1, false, nil, nil))
		itens, _, err := listarEstoque(catalogomysql.NovoEstoques(db).Historico, ctx, catalogo.FiltroEstoque{})
		assert.Error(t, err)
		assert.Nil(t, itens)
	})
}

// TestCatalogomysql_FiltrosCombinadosEPagina: todos os filtros de produto
// juntos, na ordem do legado, com LIMIT/OFFSET da página (page 3, limit 5).
func TestCatalogomysql_FiltrosCombinadosEPagina(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()
	nao := false
	where := `WHERE categoria = \? AND marca = \? AND ativo = \? AND \(descricao LIKE \? ESCAPE '!' OR sku LIKE \? ESCAPE '!'\)`
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos `+where).
		WithArgs("C", "M", false, "%b%", "%b%").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(11))
	mock.ExpectQuery(`FROM produtos `+where+` ORDER BY id ASC LIMIT \? OFFSET \?`).
		WithArgs("C", "M", false, "%b%", "%b%", 5, 10).WillReturnRows(sqlmock.NewRows(produtoColumns))

	res, err := catalogomysql.NovoProdutos(db).Listar(context.Background(),
		catalogo.FiltroProduto{Categoria: "C", Marca: "M", Ativo: &nao, Busca: "b"}, kernel.NovaPagina(3, 5))
	require.NoError(t, err)
	assert.Equal(t, 11, res.Total)
	assert.Equal(t, 3, res.Paginas())
	assert.Nil(t, res.Itens, "sem linhas: nil (JSON data: null, como no legado)")
	assert.NoError(t, mock.ExpectationsWereMet())
}
