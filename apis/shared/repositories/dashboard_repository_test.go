package repositories_test

import (
	"database/sql"
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

// errTabelaNaoExiste simula o erro do driver MySQL quando a tabela não existe
// (usado para testar o fallback de isTableNotFound).
var errTabelaNaoExiste = errors.New("Error 1146: Table 'db.pedidos' doesn't exist")

func TestDashboardGetVendasTotais_Success(t *testing.T) {
	testes := []struct {
		nome    string
		periodo string
	}{
		{"periodo today", "today"},
		{"periodo month", "month"},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			rows := sqlmock.NewRows([]string{"valor_total", "quantidade"}).AddRow(1500.50, 5)
			mock.ExpectQuery("SELECT COALESCE\\(SUM\\(valor_total\\), 0\\), COUNT\\(\\*\\) FROM pedidos WHERE .+").
				WillReturnRows(rows)

			repo := repositories.NewDashboardRepository()
			ctx := context.Background()
			valor, qtd, err := repo.GetVendasTotais(ctx, db, tt.periodo)

			require.NoError(t, err)
			assert.Equal(t, 1500.50, valor)
			assert.Equal(t, 5, qtd)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardGetVendasTotais_TabelaNaoExiste(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(valor_total\\), 0\\), COUNT\\(\\*\\) FROM pedidos WHERE .+").
		WillReturnError(errTabelaNaoExiste)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	valor, qtd, err := repo.GetVendasTotais(ctx, db, "today")

	require.NoError(t, err)
	assert.Equal(t, 0.0, valor)
	assert.Equal(t, 0, qtd)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendasTotais_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(valor_total\\), 0\\), COUNT\\(\\*\\) FROM pedidos WHERE .+").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, _, err := repo.GetVendasTotais(ctx, db, "today")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetTotalPedidos_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM pedidos WHERE .+").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(7))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	total, err := repo.GetTotalPedidos(ctx, db, "month")

	require.NoError(t, err)
	assert.Equal(t, 7, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetTotalPedidos_TabelaNaoExiste(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM pedidos WHERE .+").
		WillReturnError(errTabelaNaoExiste)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	total, err := repo.GetTotalPedidos(ctx, db, "today")

	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetTotalPedidos_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM pedidos WHERE .+").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, err := repo.GetTotalPedidos(ctx, db, "today")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetTopVendedores_ComEnrich(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT v\\.id, v\\.nome, v\\.meta_mensal AS meta FROM vendedores v WHERE v\\.data_desligamento IS NULL ORDER BY v\\.meta_mensal DESC LIMIT \\?").
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}).
			AddRow(1, "Vendedor A", 10000.0))

	mock.ExpectQuery("SELECT id_vendedor, .+ FROM pedidos WHERE id_vendedor IN \\(\\?\\) AND YEAR\\(data_pedido\\).+").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor", "total", "qtd"}).
			AddRow(1, 5000.0, 3))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	result, err := repo.GetTopVendedores(ctx, db, 5)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Vendedor A", result[0].Nome)
	assert.Equal(t, 5000.0, result[0].TotalVendas)
	assert.Equal(t, 3, result[0].QuantidadeVendas)
	assert.Equal(t, 50.0, result[0].PercentualMeta)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetTopVendedores_Vazio(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT v\\.id, v\\.nome, v\\.meta_mensal AS meta FROM vendedores v WHERE v\\.data_desligamento IS NULL ORDER BY v\\.meta_mensal DESC LIMIT \\?").
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	result, err := repo.GetTopVendedores(ctx, db, 5)

	require.NoError(t, err)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetTopVendedores_TabelaNaoExiste(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT v\\.id, v\\.nome, v\\.meta_mensal AS meta FROM vendedores v WHERE v\\.data_desligamento IS NULL ORDER BY v\\.meta_mensal DESC LIMIT \\?").
		WithArgs(5).
		WillReturnError(errTabelaNaoExiste)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	result, err := repo.GetTopVendedores(ctx, db, 5)

	require.NoError(t, err)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetTopVendedores_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT v\\.id, v\\.nome, v\\.meta_mensal AS meta FROM vendedores v WHERE v\\.data_desligamento IS NULL ORDER BY v\\.meta_mensal DESC LIMIT \\?").
		WithArgs(5).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, err := repo.GetTopVendedores(ctx, db, 5)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetMetasVendedores_ComEnrich(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT v\\.id, v\\.nome, v\\.regiao, v\\.uf, v\\.meta_mensal AS meta FROM vendedores v WHERE v\\.data_desligamento IS NULL ORDER BY v\\.meta_mensal DESC").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}).
			AddRow(1, "Vendedor A", "Sudeste", "SP", 10000.0))

	mock.ExpectQuery("SELECT id_vendedor, .+ FROM pedidos WHERE id_vendedor IN \\(\\?\\) AND YEAR\\(data_pedido\\).+").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor", "total", "qtd"}).
			AddRow(1, 2500.0, 2))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	result, err := repo.GetMetasVendedores(ctx, db)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, 2500.0, result[0].Realizado)
	assert.Equal(t, 2, result[0].QuantidadeVendas)
	assert.Equal(t, 25.0, result[0].Percentual)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetMetasVendedores_Vazio(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT v\\.id, v\\.nome, v\\.regiao, v\\.uf, v\\.meta_mensal AS meta FROM vendedores v WHERE v\\.data_desligamento IS NULL ORDER BY v\\.meta_mensal DESC").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	result, err := repo.GetMetasVendedores(ctx, db)

	require.NoError(t, err)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetMetasVendedores_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT v\\.id, v\\.nome, v\\.regiao, v\\.uf, v\\.meta_mensal AS meta FROM vendedores v WHERE v\\.data_desligamento IS NULL ORDER BY v\\.meta_mensal DESC").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, err := repo.GetMetasVendedores(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendasSeries_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT DATE\\(data_pedido\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}).
			AddRow("2024-01-01", 100.0, 2))

	mock.ExpectQuery("SELECT DATE_SUB\\(CURDATE\\(\\), INTERVAL n DAY\\) AS dia .+").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"dia"}).AddRow("2024-01-01"))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	series, err := repo.GetVendasSeries(ctx, db, 1)

	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Equal(t, "2024-01-01", series[0]["data"])
	assert.Equal(t, 100.0, series[0]["valor"])
	assert.Equal(t, 2, series[0]["quantidade"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendasSeries_TabelaNaoExiste(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT DATE\\(data_pedido\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
		WithArgs(3).
		WillReturnError(errTabelaNaoExiste)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	series, err := repo.GetVendasSeries(ctx, db, 3)

	require.NoError(t, err)
	assert.Len(t, series, 3)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendasSeries_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT DATE\\(data_pedido\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
		WithArgs(3).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, err := repo.GetVendasSeries(ctx, db, 3)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendasSeries_SemRegistros(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT DATE\\(data_pedido\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	series, err := repo.GetVendasSeries(ctx, db, 2)

	require.NoError(t, err)
	assert.Len(t, series, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendasSeries_BuildFullSeriesErro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT DATE\\(data_pedido\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}).
			AddRow("2024-01-01", 100.0, 2))

	mock.ExpectQuery("SELECT DATE_SUB\\(CURDATE\\(\\), INTERVAL n DAY\\) AS dia .+").
		WithArgs(1).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	series, err := repo.GetVendasSeries(ctx, db, 1)

	require.NoError(t, err)
	// fallback: emptySeries quando a query de datas falha.
	assert.Len(t, series, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendedoresRanking_ComVendas(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM vendedores WHERE data_desligamento IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))

	mock.ExpectQuery("SELECT id, nome, regiao, uf, meta_mensal FROM vendedores WHERE data_desligamento IS NULL ORDER BY meta_mensal DESC LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal"}).
			AddRow(1, "Vendedor A", "Sudeste", "SP", 10000.0))

	mock.ExpectQuery("SELECT id_vendedor, .+ FROM pedidos WHERE id_vendedor IN \\(\\?\\) AND YEAR\\(data_pedido\\).+").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor", "total", "qtd"}).
			AddRow(1, 4000.0, 4))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	result, total, err := repo.GetVendedoresRanking(ctx, db, 1, 10)

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, result, 1)
	assert.Equal(t, "Vendedor A", result[0]["vendedor_nome"])
	assert.Equal(t, 4000.0, result[0]["total_vendas"])
	assert.Equal(t, 4, result[0]["total_pedidos"])
	assert.Equal(t, 1000.0, result[0]["ticket_medio"])
	assert.Equal(t, 40.0, result[0]["atingimento_meta"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendedoresRanking_Vazio(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM vendedores WHERE data_desligamento IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

	mock.ExpectQuery("SELECT id, nome, regiao, uf, meta_mensal FROM vendedores WHERE data_desligamento IS NULL ORDER BY meta_mensal DESC LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal"}))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	result, total, err := repo.GetVendedoresRanking(ctx, db, 1, 10)

	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendedoresRanking_Defaults(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM vendedores WHERE data_desligamento IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

	mock.ExpectQuery("SELECT id, nome, regiao, uf, meta_mensal FROM vendedores WHERE data_desligamento IS NULL ORDER BY meta_mensal DESC LIMIT \\? OFFSET \\?").
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal"}))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, _, err := repo.GetVendedoresRanking(ctx, db, 0, 0)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendedoresRanking_LimitCap(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM vendedores WHERE data_desligamento IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

	mock.ExpectQuery("SELECT id, nome, regiao, uf, meta_mensal FROM vendedores WHERE data_desligamento IS NULL ORDER BY meta_mensal DESC LIMIT \\? OFFSET \\?").
		WithArgs(100, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal"}))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, _, err := repo.GetVendedoresRanking(ctx, db, 1, 200)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendedoresRanking_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM vendedores WHERE data_desligamento IS NULL").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, _, err := repo.GetVendedoresRanking(ctx, db, 1, 10)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendedoresRanking_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM vendedores WHERE data_desligamento IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))

	mock.ExpectQuery("SELECT id, nome, regiao, uf, meta_mensal FROM vendedores WHERE data_desligamento IS NULL ORDER BY meta_mensal DESC LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, _, err := repo.GetVendedoresRanking(ctx, db, 1, 10)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
