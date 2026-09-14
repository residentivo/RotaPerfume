package services_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

func dashboardTestCfg(verbose bool) *config.Config {
	return &config.Config{Verbose: verbose}
}

func newDashboardTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

// expectMetricsHappyPath configura as expectativas de mock necessárias para
// um GetMetrics bem-sucedido, na ordem em que o service as executa.
func expectMetricsHappyPath(mock sqlmock.Sqlmock) {
	// GetVendasTotais
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
		WillReturnRows(sqlmock.NewRows([]string{"valor_total", "quantidade"}).AddRow(1000.0, 10))
	// GetTotalPedidos
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos\s+WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(10))
	// GetTopVendedores
	mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}))
	// GetMetasVendedores
	mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.regiao,\s+v\.uf,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}))
}

// ---------------------------------------------------------------------------
// GetMetrics
// ---------------------------------------------------------------------------

func TestDashboardService_GetMetrics(t *testing.T) {
	t.Run("sucesso - periodo today", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		expectMetricsHappyPath(mock)

		svc := services.NewDashboardService(db, dashboardTestCfg(true))
		metrics, err := svc.GetMetrics(context.Background(), db, "today")
		require.NoError(t, err)
		assert.Equal(t, "today", metrics["periodo"])
		assert.Equal(t, 1000.0, metrics["total_vendas_valor"])
		assert.Equal(t, 10, metrics["total_vendas_qtd"])
		assert.Equal(t, 10, metrics["total_pedidos"])
		assert.Equal(t, 100.0, metrics["ticket_medio"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("sucesso - periodo month, ticket medio zero quando sem vendas", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
			WillReturnRows(sqlmock.NewRows([]string{"valor_total", "quantidade"}).AddRow(0.0, 0))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos\s+WHERE`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
		mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
			WithArgs(10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}))
		mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.regiao,\s+v\.uf,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}))

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		metrics, err := svc.GetMetrics(context.Background(), db, "month")
		require.NoError(t, err)
		assert.Equal(t, 0.0, metrics["ticket_medio"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no GetVendasTotais é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		metrics, err := svc.GetMetrics(context.Background(), db, "today")
		assert.Nil(t, metrics)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no GetTotalPedidos é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
			WillReturnRows(sqlmock.NewRows([]string{"valor_total", "quantidade"}).AddRow(1000.0, 10))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos\s+WHERE`).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		metrics, err := svc.GetMetrics(context.Background(), db, "today")
		assert.Nil(t, metrics)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no GetTopVendedores é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
			WillReturnRows(sqlmock.NewRows([]string{"valor_total", "quantidade"}).AddRow(1000.0, 10))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos\s+WHERE`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(10))
		mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
			WithArgs(10).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		metrics, err := svc.GetMetrics(context.Background(), db, "today")
		assert.Nil(t, metrics)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no GetMetasVendedores é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
			WillReturnRows(sqlmock.NewRows([]string{"valor_total", "quantidade"}).AddRow(1000.0, 10))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos\s+WHERE`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(10))
		mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
			WithArgs(10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}))
		mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.regiao,\s+v\.uf,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		metrics, err := svc.GetMetrics(context.Background(), db, "today")
		assert.Nil(t, metrics)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// GetVendasSeries
// ---------------------------------------------------------------------------

func TestDashboardService_GetVendasSeries(t *testing.T) {
	t.Run("tabela pedidos inexistente retorna serie vazia com zeros", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT DATE\(data_pedido\) AS data`).
			WithArgs(7).
			WillReturnError(sql.ErrNoRows) // não contém texto "doesn't exist" -> repo retorna erro real

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		series, err := svc.GetVendasSeries(context.Background(), db, 7)
		// Como sql.ErrNoRows não é reconhecido como "tabela não existe" pelo
		// repo, o erro é propagado.
		assert.Error(t, err)
		assert.Nil(t, series)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro tipo tabela nao existe retorna serie vazia sem erro", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT DATE\(data_pedido\) AS data`).
			WithArgs(3).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		// sql.ErrConnDone também não contém "doesn't exist" -> erro propagado.
		series, err := svc.GetVendasSeries(context.Background(), db, 3)
		assert.Error(t, err)
		assert.Nil(t, series)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// GetVendedoresRanking
// ---------------------------------------------------------------------------

func TestDashboardService_GetVendedoresRanking(t *testing.T) {
	t.Run("sucesso - sem vendedores", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
		mock.ExpectQuery(`SELECT id, nome, regiao, uf, meta_mensal\s+FROM vendedores\s+WHERE data_desligamento IS NULL\s+ORDER BY meta_mensal DESC\s+LIMIT \? OFFSET \?`).
			WithArgs(20, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal"}))

		svc := services.NewDashboardService(db, dashboardTestCfg(true))
		ranking, total, err := svc.GetVendedoresRanking(context.Background(), db, 1, 20)
		require.NoError(t, err)
		assert.Len(t, ranking, 0)
		assert.Equal(t, 0, total)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("sucesso - com vendedores e vendas", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
		mock.ExpectQuery(`SELECT id, nome, regiao, uf, meta_mensal\s+FROM vendedores\s+WHERE data_desligamento IS NULL\s+ORDER BY meta_mensal DESC\s+LIMIT \? OFFSET \?`).
			WithArgs(20, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal"}).
				AddRow(int64(1), "João", "Sudeste", "SP", 10000.0))
		mock.ExpectQuery(`SELECT id_vendedor,\s+COALESCE\(SUM\(valor_total\), 0\),\s+COUNT\(\*\)\s+FROM pedidos`).
			WithArgs(int64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id_vendedor", "total", "qtd"}).AddRow(int64(1), 5000.0, 5))

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		ranking, total, err := svc.GetVendedoresRanking(context.Background(), db, 1, 20)
		require.NoError(t, err)
		require.Len(t, ranking, 1)
		assert.Equal(t, 1, total)
		assert.Equal(t, int64(1), ranking[0]["vendedor_id"])
		assert.Equal(t, 5000.0, ranking[0]["total_vendas"])
		assert.Equal(t, 50.0, ranking[0]["atingimento_meta"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no count é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL`).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		ranking, total, err := svc.GetVendedoresRanking(context.Background(), db, 1, 20)
		assert.Nil(t, ranking)
		assert.Equal(t, 0, total)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// GetClienteMetrics
// ---------------------------------------------------------------------------

func TestDashboardService_GetClienteMetrics(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(100))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \?`).
			WithArgs(true).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(80))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \?`).
			WithArgs(false).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(20))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(5))
		mock.ExpectQuery(`SELECT segmento, COUNT\(\*\) AS total\s+FROM clientes\s+GROUP BY segmento\s+ORDER BY total DESC`).
			WillReturnRows(sqlmock.NewRows([]string{"segmento", "total"}).AddRow("varejo", 60))
		mock.ExpectQuery(`SELECT uf, COUNT\(\*\) AS total\s+FROM clientes\s+GROUP BY uf\s+ORDER BY total DESC`).
			WillReturnRows(sqlmock.NewRows([]string{"uf", "total"}).AddRow("SP", 40))

		svc := services.NewDashboardService(db, dashboardTestCfg(true))
		metrics, err := svc.GetClienteMetrics(context.Background(), db, "month")
		require.NoError(t, err)
		assert.Equal(t, "month", metrics["periodo"])
		assert.Equal(t, 100, metrics["total_clientes"])
		assert.Equal(t, 80, metrics["total_ativos"])
		assert.Equal(t, 20, metrics["total_inativos"])
		assert.Equal(t, 5, metrics["novos_no_periodo"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no CountTotal é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		metrics, err := svc.GetClienteMetrics(context.Background(), db, "today")
		assert.Nil(t, metrics)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no CountPorSegmento é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(100))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \?`).
			WithArgs(true).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(80))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \?`).
			WithArgs(false).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(20))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(5))
		mock.ExpectQuery(`SELECT segmento, COUNT\(\*\) AS total\s+FROM clientes\s+GROUP BY segmento\s+ORDER BY total DESC`).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		metrics, err := svc.GetClienteMetrics(context.Background(), db, "today")
		assert.Nil(t, metrics)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
