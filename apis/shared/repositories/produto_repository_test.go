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

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

var produtoColumns = []string{
	"id", "sku", "descricao", "categoria", "marca", "nota_olfativa",
	"preco_tabela", "custo_unitario", "unidade", "data_lancamento", "ativo", "created_at", "updated_at",
}

func produtoRow(id int64, sku, descricao, categoria, marca, notaOlfativa string, precoTabela, custoUnitario float64, unidade string, dataLancamento *time.Time, ativo bool, created, updated time.Time) []driver.Value {
	var dl driver.Value = nil
	if dataLancamento != nil {
		dl = *dataLancamento
	}
	return []driver.Value{id, sku, descricao, categoria, marca, notaOlfativa, precoTabela, custoUnitario, unidade, dl, ativo, created, updated}
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

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	produtos, total, err := repo.List(ctx, db, 1, 10, repositories.ProdutoFiltro{})

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, produtos, 2)
	assert.Equal(t, "SKU1", produtos[0].SKU)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoList_ComFiltros(t *testing.T) {
	testes := []struct {
		nome        string
		filtro      repositories.ProdutoFiltro
		whereRegexp string
		args        []driver.Value
	}{
		{
			nome:        "filtro por categoria",
			filtro:      repositories.ProdutoFiltro{Categoria: "Perfumaria"},
			whereRegexp: "WHERE categoria = \\?",
			args:        []driver.Value{"Perfumaria"},
		},
		{
			nome:        "filtro por marca",
			filtro:      repositories.ProdutoFiltro{Marca: "Marca A"},
			whereRegexp: "WHERE marca = \\?",
			args:        []driver.Value{"Marca A"},
		},
		{
			nome:        "filtro por ativo",
			filtro:      repositories.ProdutoFiltro{Ativo: boolPtr(false)},
			whereRegexp: "WHERE ativo = \\?",
			args:        []driver.Value{false},
		},
		{
			nome:        "filtro por busca textual",
			filtro:      repositories.ProdutoFiltro{Q: "Perfume"},
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

			repo := repositories.NewProdutoRepository()
			ctx := context.Background()
			_, _, err := repo.List(ctx, db, 1, 10, tt.filtro)

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

			repo := repositories.NewProdutoRepository()
			ctx := context.Background()
			_, _, err := repo.List(ctx, db, 1, 10, repositories.ProdutoFiltro{OrderBy: tt.orderBy, OrderDir: tt.orderDir})

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

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.ProdutoFiltro{})

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

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.ProdutoFiltro{})

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

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.ProdutoFiltro{})

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

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	p, err := repo.GetByID(ctx, db, 1)

	require.NoError(t, err)
	assert.Equal(t, "SKU1", p.SKU)
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

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	p, err := repo.GetByID(ctx, db, 999)

	assert.Nil(t, p)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM produtos WHERE id = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	p, err := repo.GetByID(ctx, db, 1)

	assert.Nil(t, p)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoSetAtivo_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE produtos SET ativo = \? WHERE id = \?`).
		WithArgs(false, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	err := repo.SetAtivo(ctx, db, 1, false)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoSetAtivo_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE produtos SET ativo = \? WHERE id = \?`).
		WithArgs(true, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	err := repo.SetAtivo(ctx, db, 999, true)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoSetAtivo_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE produtos SET ativo = \? WHERE id = \?`).
		WithArgs(true, int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	err := repo.SetAtivo(ctx, db, 1, true)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCountTotal(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos$").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(15))

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	total, err := repo.CountTotal(ctx, db)

	require.NoError(t, err)
	assert.Equal(t, 15, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCountTotal_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM produtos$").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	_, err := repo.CountTotal(ctx, db)

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

			repo := repositories.NewProdutoRepository()
			ctx := context.Background()
			total, err := repo.CountPorAtivo(ctx, db, tt.ativo)

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

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	_, err := repo.CountPorAtivo(ctx, db, true)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Produto{
		SKU:           "SKU1",
		Descricao:     "Perfume A",
		Categoria:     "Perfumaria",
		Marca:         "Marca A",
		NotaOlfativa:  "Floral",
		PrecoTabela:   100.0,
		CustoUnitario: 50.0,
		Unidade:       "un",
		Ativo:         true,
	}

	mock.ExpectExec("INSERT INTO produtos").
		WithArgs(p.SKU, p.Descricao, p.Categoria, p.Marca, "Floral", p.PrecoTabela, p.CustoUnitario, p.Unidade, nil, p.Ativo).
		WillReturnResult(sqlmock.NewResult(7, 1))

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, p)

	require.NoError(t, err)
	assert.Equal(t, int64(7), p.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCreate_NotaOlfativaVazia(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Produto{SKU: "SKU2", Descricao: "Perfume B"}

	mock.ExpectExec("INSERT INTO produtos").
		WithArgs(p.SKU, p.Descricao, p.Categoria, p.Marca, nil, p.PrecoTabela, p.CustoUnitario, p.Unidade, nil, p.Ativo).
		WillReturnResult(sqlmock.NewResult(8, 1))

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, p)

	require.NoError(t, err)
	assert.Equal(t, int64(8), p.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoCreate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Produto{}
	mock.ExpectExec("INSERT INTO produtos").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, p)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoUpdate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Produto{
		Descricao:     "Perfume A Atualizado",
		Categoria:     "Perfumaria",
		Marca:         "Marca A",
		NotaOlfativa:  "Cítrico",
		PrecoTabela:   150.0,
		CustoUnitario: 60.0,
		Unidade:       "un",
	}

	mock.ExpectExec("UPDATE produtos").
		WithArgs(p.Descricao, p.Categoria, p.Marca, "Cítrico", p.PrecoTabela, p.CustoUnitario, p.Unidade, nil, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, p)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoUpdate_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Produto{}
	mock.ExpectExec("UPDATE produtos").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 999, p)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProdutoUpdate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Produto{}
	mock.ExpectExec("UPDATE produtos").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewProdutoRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, p)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
