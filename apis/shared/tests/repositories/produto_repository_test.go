package repositories_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/kernel"
)

// ARQ-02: os testes de ProdutoRepository exercitam catalogomysql.Produtos (o
// SQL é o mesmo do legado); os nomes de teste são mantidos (R1).

// listarProdutos chama Listar na página 1 com limite 10 e devolve no formato
// do legado (itens, total, erro).
func listarProdutos(repo *catalogomysql.Produtos, ctx context.Context, f catalogo.FiltroProduto) ([]catalogo.Produto, int, error) {
	res, err := repo.Listar(ctx, f, kernel.NovaPagina(1, 10))
	return res.Itens, res.Total, err
}

// ordenacaoProduto faz o parse de order_by/order_dir como o catalogohttp.
func ordenacaoProduto(orderBy, orderDir string) kernel.Ordenacao[catalogo.CampoProduto] {
	return kernel.ParseOrdenacao(orderBy, orderDir, catalogo.CamposOrdenacaoProduto(), catalogo.OrdenacaoProdutoPadrao())
}

var produtoColumns = []string{
	"id", "sku", "descricao", "categoria", "marca", "nota_olfativa",
	"preco_tabela", "custo_unitario", "unidade", "data_lancamento", "ativo", "imagem", "created_at", "updated_at",
}

func produtoRow(id int64, sku, descricao, categoria, marca, notaOlfativa string, precoTabela, custoUnitario float64, unidade string, dataLancamento *time.Time, ativo bool, created, updated time.Time) []driver.Value {
	var dl driver.Value = nil
	if dataLancamento != nil {
		dl = *dataLancamento
	}
	return []driver.Value{id, sku, descricao, categoria, marca, notaOlfativa, precoTabela, custoUnitario, unidade, dl, ativo, "", created, updated}
}

func TestProdutoList_SemFiltro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(produtoColumns).
		AddRow(produtoRow(1, "SKU1", "Perfume A", "Perfumaria", "Marca A", "Floral", 100.0, 50.0, "un", nil, true, now, now)...).
		AddRow(produtoRow(2, "SKU2", "Perfume B", "Perfumaria", "Marca B", "Amadeirado", 200.0, 90.0, "un", nil, true, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM produtos ORDER BY id ASC LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnRows(rows)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	produtos, total, err := listarProdutos(repo, ctx, catalogo.FiltroProduto{})

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, produtos, 2)
	assert.Equal(t, catalogo.SKU("SKU1"), produtos[0].SKU)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoList_ComFiltros(t *testing.T) {
	testes := []struct {
		nome        string
		filtro      catalogo.FiltroProduto
		whereRegexp string
		args        []driver.Value
	}{
		{
			nome:        "filtro por categoria",
			filtro:      catalogo.FiltroProduto{Categoria: "Perfumaria"},
			whereRegexp: "WHERE categoria = \\?",
			args:        []driver.Value{"Perfumaria"},
		},
		{
			nome:        "filtro por marca",
			filtro:      catalogo.FiltroProduto{Marca: "Marca A"},
			whereRegexp: "WHERE marca = \\?",
			args:        []driver.Value{"Marca A"},
		},
		{
			nome:        "filtro por ativo",
			filtro:      catalogo.FiltroProduto{Ativo: boolPtr(false)},
			whereRegexp: "WHERE ativo = \\?",
			args:        []driver.Value{false},
		},
		{
			nome:        "filtro por busca textual",
			filtro:      catalogo.FiltroProduto{Busca: "Perfume"},
			whereRegexp: "WHERE \\(descricao LIKE \\? OR sku LIKE \\?\\)",
			args:        []driver.Value{"%Perfume%", "%Perfume%"},
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos " + tt.whereRegexp).
				WithArgs(tt.args...).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

			allArgs := append(append([]driver.Value{}, tt.args...), int64(10), int64(0))
			mock.ExpectQuery("SELECT .+ FROM produtos " + tt.whereRegexp + " ORDER BY id ASC LIMIT \\? OFFSET \\?").
				WithArgs(allArgs...).
				WillReturnRows(sqlmock.NewRows(produtoColumns))

			repo := catalogomysql.NovoProdutos(db)
			ctx := context.Background()
			_, _, err := listarProdutos(repo, ctx, tt.filtro)

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestProdutoList_OrderBy(t *testing.T) {
	testes := []struct {
		nome        string
		orderBy     string
		orderDir    string
		orderRegexp string
	}{
		{
			nome:        "order_by válido asc",
			orderBy:     "descricao",
			orderDir:    "asc",
			orderRegexp: "ORDER BY descricao ASC",
		},
		{
			nome:        "order_by válido desc",
			orderBy:     "preco_tabela",
			orderDir:    "desc",
			orderRegexp: "ORDER BY preco_tabela DESC",
		},
		{
			nome:        "order_by case-insensitive",
			orderBy:     "SKU",
			orderDir:    "DESC",
			orderRegexp: "ORDER BY sku DESC",
		},
		{
			nome:        "order_by fora da whitelist cai no default",
			orderBy:     "1; DROP TABLE produtos;--",
			orderDir:    "asc",
			orderRegexp: "ORDER BY id ASC",
		},
		{
			nome:        "order_dir inválido cai no default (asc)",
			orderBy:     "marca",
			orderDir:    "sideways",
			orderRegexp: "ORDER BY marca ASC",
		},
		{
			nome:        "order_by vazio cai no default",
			orderBy:     "",
			orderDir:    "desc",
			orderRegexp: "ORDER BY id DESC",
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos").
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery("SELECT .+ FROM produtos "+tt.orderRegexp+" LIMIT \\? OFFSET \\?").
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows(produtoColumns))

			repo := catalogomysql.NovoProdutos(db)
			ctx := context.Background()
			_, _, err := listarProdutos(repo, ctx, catalogo.FiltroProduto{Ordenacao: ordenacaoProduto(tt.orderBy, tt.orderDir)})

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestProdutoList_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos").
		WillReturnError(sql.ErrConnDone)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	_, _, err := listarProdutos(repo, ctx, catalogo.FiltroProduto{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoList_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM produtos").
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	_, _, err := listarProdutos(repo, ctx, catalogo.FiltroProduto{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM produtos").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(produtoColumns).
			AddRow(produtoRow(1, "SKU1", "Perfume A", "Perfumaria", "Marca A", "Floral", 100.0, 50.0, "un", nil, true, now, now)...).
			RowError(0, sql.ErrConnDone))

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	_, _, err := listarProdutos(repo, ctx, catalogo.FiltroProduto{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	lancamento := now.Add(-24 * time.Hour)
	rows := sqlmock.NewRows(produtoColumns).
		AddRow(produtoRow(1, "SKU1", "Perfume A", "Perfumaria", "Marca A", "Floral", 100.0, 50.0, "un", &lancamento, true, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM produtos WHERE id = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	p, err := repo.PorID(ctx, 1)

	require.NoError(t, err)
	assert.Equal(t, catalogo.SKU("SKU1"), p.SKU)
	require.NotNil(t, p.DataLancamento)
	assert.Equal(t, lancamento.Unix(), p.DataLancamento.Unix())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM produtos WHERE id = \\? LIMIT 1").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	p, err := repo.PorID(ctx, 999)

	assert.Nil(t, p)
	assert.ErrorIs(t, err, catalogo.ErrProdutoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM produtos WHERE id = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	p, err := repo.PorID(ctx, 1)

	assert.Nil(t, p)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, catalogo.ErrProdutoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoSetAtivo_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE produtos SET ativo = \? WHERE id = \?`).
		WithArgs(false, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	err := repo.DefinirAtivo(ctx, 1, false)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoSetAtivo_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE produtos SET ativo = \? WHERE id = \?`).
		WithArgs(true, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	err := repo.DefinirAtivo(ctx, 999, true)

	assert.ErrorIs(t, err, catalogo.ErrProdutoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoSetAtivo_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE produtos SET ativo = \? WHERE id = \?`).
		WithArgs(true, int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	err := repo.DefinirAtivo(ctx, 1, true)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCountTotal(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos$").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(15))

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	total, err := repo.ContarTotal(ctx)

	require.NoError(t, err)
	assert.Equal(t, 15, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCountTotal_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos$").
		WillReturnError(sql.ErrConnDone)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	_, err := repo.ContarTotal(ctx)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCountPorAtivo(t *testing.T) {
	testes := []struct {
		nome  string
		ativo bool
		total int
	}{
		{"produtos ativos", true, 12},
		{"produtos inativos", false, 3},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos WHERE ativo = \\?").
				WithArgs(tt.ativo).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(tt.total))

			repo := catalogomysql.NovoProdutos(db)
			ctx := context.Background()
			total, err := repo.ContarPorAtivo(ctx, tt.ativo)

			require.NoError(t, err)
			assert.Equal(t, tt.total, total)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestProdutoCountPorAtivo_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos WHERE ativo = \\?").
		WithArgs(true).
		WillReturnError(sql.ErrConnDone)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	_, err := repo.ContarPorAtivo(ctx, true)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &catalogo.Produto{
		SKU: "SKU1",
		CamposProduto: catalogo.CamposProduto{
			Descricao:     "Perfume A",
			Categoria:     "Perfumaria",
			Marca:         "Marca A",
			NotaOlfativa:  "Floral",
			PrecoTabela:   100.0,
			CustoUnitario: 50.0,
			Unidade:       "un",
		},
		Ativo: true,
	}

	mock.ExpectExec("INSERT INTO produtos").
		WithArgs(p.SKU.String(), p.Descricao, p.Categoria, p.Marca, "Floral", p.PrecoTabela, p.CustoUnitario, p.Unidade, nil, p.Ativo).
		WillReturnResult(sqlmock.NewResult(7, 1))

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	err := repo.Inserir(ctx, p)

	require.NoError(t, err)
	assert.Equal(t, int64(7), p.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCreate_NotaOlfativaVazia(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &catalogo.Produto{SKU: "SKU2", CamposProduto: catalogo.CamposProduto{Descricao: "Perfume B"}}

	mock.ExpectExec("INSERT INTO produtos").
		WithArgs(p.SKU.String(), p.Descricao, p.Categoria, p.Marca, nil, p.PrecoTabela, p.CustoUnitario, p.Unidade, nil, p.Ativo).
		WillReturnResult(sqlmock.NewResult(8, 1))

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	err := repo.Inserir(ctx, p)

	require.NoError(t, err)
	assert.Equal(t, int64(8), p.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCreate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &catalogo.Produto{}
	mock.ExpectExec("INSERT INTO produtos").
		WillReturnError(sql.ErrConnDone)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	err := repo.Inserir(ctx, p)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoUpdate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &catalogo.Produto{CamposProduto: catalogo.CamposProduto{
		Descricao:     "Perfume A Atualizado",
		Categoria:     "Perfumaria",
		Marca:         "Marca A",
		NotaOlfativa:  "Cítrico",
		PrecoTabela:   150.0,
		CustoUnitario: 60.0,
		Unidade:       "un",
	}}

	mock.ExpectExec("UPDATE produtos").
		WithArgs(p.Descricao, p.Categoria, p.Marca, "Cítrico", p.PrecoTabela, p.CustoUnitario, p.Unidade, nil, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	err := repo.Atualizar(ctx, 1, p.CamposProduto)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoUpdate_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &catalogo.Produto{}
	mock.ExpectExec("UPDATE produtos").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	err := repo.Atualizar(ctx, 999, p.CamposProduto)

	assert.ErrorIs(t, err, catalogo.ErrProdutoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoUpdate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &catalogo.Produto{}
	mock.ExpectExec("UPDATE produtos").
		WillReturnError(sql.ErrConnDone)

	repo := catalogomysql.NovoProdutos(db)
	ctx := context.Background()
	err := repo.Atualizar(ctx, 1, p.CamposProduto)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
