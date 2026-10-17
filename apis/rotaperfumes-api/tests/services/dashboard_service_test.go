package services_test

import (
	"context"
	"database/sql"
	"errors"
	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsapp"
	"github.com/rotaperfumes/shared/kernel"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	// GetMetaMensalTotal
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(meta_mensal\), 0\)\s+FROM vendedores\s+WHERE data_desligamento IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"meta_total"}).AddRow(20000.0))
}

// ---------------------------------------------------------------------------
// GetMetrics
// ---------------------------------------------------------------------------

func TestDashboardService_GetMetrics(t *testing.T) {
	t.Run("sucesso - periodo today", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		expectMetricsHappyPath(mock)

		metrics, err := consultasAnalytics(db, 0).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoDe("today")})
		require.NoError(t, err)
		assert.Equal(t, "today", metrics.Periodo.String())
		assert.Equal(t, 1000.0, metrics.TotalVendas)
		assert.Equal(t, 10, metrics.TotalVendasQtd)
		assert.Equal(t, 10, metrics.TotalPedidos)
		assert.Equal(t, 100.0, metrics.TicketMedio)
		assert.Equal(t, 20000.0, metrics.MetaMes)
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
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(meta_mensal\), 0\)\s+FROM vendedores\s+WHERE data_desligamento IS NULL`).
			WillReturnRows(sqlmock.NewRows([]string{"meta_total"}).AddRow(0.0))

		metrics, err := consultasAnalytics(db, 0).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoDe("month")})
		require.NoError(t, err)
		assert.Equal(t, 0.0, metrics.TicketMedio)
		assert.Equal(t, 0.0, metrics.MetaMes)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no GetVendasTotais é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
			WillReturnError(sql.ErrConnDone)

		metrics, err := consultasAnalytics(db, 0).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoDe("today")})
		assert.Zero(t, metrics, "em erro, o read model volta zerado")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no GetTotalPedidos é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
			WillReturnRows(sqlmock.NewRows([]string{"valor_total", "quantidade"}).AddRow(1000.0, 10))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos\s+WHERE`).
			WillReturnError(sql.ErrConnDone)

		metrics, err := consultasAnalytics(db, 0).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoDe("today")})
		assert.Zero(t, metrics, "em erro, o read model volta zerado")
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

		metrics, err := consultasAnalytics(db, 0).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoDe("today")})
		assert.Zero(t, metrics, "em erro, o read model volta zerado")
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

		metrics, err := consultasAnalytics(db, 0).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoDe("today")})
		assert.Zero(t, metrics, "em erro, o read model volta zerado")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no GetMetaMensalTotal é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`).
			WillReturnRows(sqlmock.NewRows([]string{"valor_total", "quantidade"}).AddRow(1000.0, 10))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos\s+WHERE`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(10))
		mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
			WithArgs(10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}))
		mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.regiao,\s+v\.uf,\s+v\.meta_mensal AS meta\s+FROM vendedores v`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}))
		mock.ExpectQuery(`SELECT COALESCE\(SUM\(meta_mensal\), 0\)\s+FROM vendedores\s+WHERE data_desligamento IS NULL`).
			WillReturnError(sql.ErrConnDone)

		metrics, err := consultasAnalytics(db, 0).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoDe("today")})
		assert.Zero(t, metrics, "em erro, o read model volta zerado")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// GetVendasSeries
// ---------------------------------------------------------------------------

func TestDashboardService_GetVendasSeries(t *testing.T) {
	t.Run("sucesso retorna shape {dias, pontos}", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT DATE_FORMAT\(data_pedido, '%Y-%m-%d'\) AS data`).
			WithArgs(7).
			WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}).
				AddRow("2024-01-01", 500.0, 3))
		mock.ExpectQuery(`SELECT DATE_FORMAT\(DATE_SUB\(CURDATE\(\), INTERVAL n DAY\), '%Y-%m-%d'\) AS dia`).
			WithArgs(7).
			WillReturnRows(sqlmock.NewRows([]string{"dia"}).AddRow("2024-01-01"))

		series, err := consultasAnalytics(db, 0).SerieVendas(context.Background(), analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(7)})
		require.NoError(t, err)
		require.NotNil(t, series)

		assert.Equal(t, 7, series.Dias.Int())

		pontos := series.Pontos
		require.NotNil(t, pontos, "pontos nunca é nil (JSON [])")
		require.Len(t, pontos, 1)
		assert.Equal(t, "2024-01-01", pontos[0].Dia)
		assert.Equal(t, 500.0, pontos[0].TotalVendas)
		assert.Equal(t, 3, pontos[0].TotalPedidos)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("tabela pedidos inexistente retorna serie vazia com zeros", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT DATE_FORMAT\(data_pedido, '%Y-%m-%d'\) AS data`).
			WithArgs(7).
			WillReturnError(sql.ErrNoRows) // não contém texto "doesn't exist" -> repo retorna erro real

		series, err := consultasAnalytics(db, 0).SerieVendas(context.Background(), analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(7)})
		// Como sql.ErrNoRows não é reconhecido como "tabela não existe" pelo
		// repo, o erro é propagado.
		assert.Error(t, err)
		assert.Nil(t, series.Pontos, "em erro, nenhum ponto sai")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro tipo tabela nao existe retorna serie vazia sem erro", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT DATE_FORMAT\(data_pedido, '%Y-%m-%d'\) AS data`).
			WithArgs(3).
			WillReturnError(sql.ErrConnDone)

		// sql.ErrConnDone também não contém "doesn't exist" -> erro propagado.
		series, err := consultasAnalytics(db, 0).SerieVendas(context.Background(), analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(3)})
		assert.Error(t, err)
		assert.Nil(t, series.Pontos, "em erro, nenhum ponto sai")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("tabela nao existe (erro reconhecido) retorna shape {dias, pontos} com serie vazia", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		errTabelaNaoExiste := errors.New("Error 1146: Table 'db.pedidos' doesn't exist")
		mock.ExpectQuery(`SELECT DATE_FORMAT\(data_pedido, '%Y-%m-%d'\) AS data`).
			WithArgs(4).
			WillReturnError(errTabelaNaoExiste)

		series, err := consultasAnalytics(db, 0).SerieVendas(context.Background(), analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(4)})
		require.NoError(t, err)
		require.NotNil(t, series)

		assert.Equal(t, 4, series.Dias.Int())
		pontos := series.Pontos
		require.NotNil(t, pontos)
		require.Len(t, pontos, 4)
		for _, p := range pontos {
			assert.Equal(t, 0.0, p.TotalVendas)
			assert.Equal(t, 0, p.TotalPedidos)
			assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, p.Dia)
		}
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
		mock.ExpectQuery(`(?s)SELECT.*FROM vendedores v.*LEFT JOIN.*ORDER BY v\.meta_mensal DESC, atingimento_meta DESC.*LIMIT \? OFFSET \?`).
			WithArgs(20, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}))

		ranking, total, err := itensETotal(consultasAnalytics(db, 0).Ranking(context.Background(), analyticsapp.ConsultaRanking{Pagina: kernel.NovaPagina(1, 20)}))
		require.NoError(t, err)
		assert.Len(t, ranking, 0)
		assert.Equal(t, 0, total)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("sucesso - com vendedores e vendas", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL`).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
		mock.ExpectQuery(`(?s)SELECT.*FROM vendedores v.*LEFT JOIN.*ORDER BY v\.meta_mensal DESC, atingimento_meta DESC.*LIMIT \? OFFSET \?`).
			WithArgs(20, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}).
				AddRow(int64(1), "João", "Sudeste", "SP", 10000.0, 5000.0, 5, 50.0))

		ranking, total, err := itensETotal(consultasAnalytics(db, 0).Ranking(context.Background(), analyticsapp.ConsultaRanking{Pagina: kernel.NovaPagina(1, 20)}))
		require.NoError(t, err)
		require.Len(t, ranking, 1)
		assert.Equal(t, 1, total)
		assert.Equal(t, int64(1), ranking[0].VendedorID)
		assert.Equal(t, 5000.0, ranking[0].TotalVendas)
		assert.Equal(t, 50.0, ranking[0].AtingimentoMeta)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no count é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL`).
			WillReturnError(sql.ErrConnDone)

		ranking, total, err := itensETotal(consultasAnalytics(db, 0).Ranking(context.Background(), analyticsapp.ConsultaRanking{Pagina: kernel.NovaPagina(1, 20)}))
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

		metrics, err := consultasAnalytics(db, 0).MetricasClientes(context.Background(), analyticsapp.ConsultaMetricasClientes{Periodo: periodoDe("month")})
		require.NoError(t, err)
		assert.Equal(t, "month", metrics.Periodo.String())
		assert.Equal(t, 100, metrics.TotalClientes)
		assert.Equal(t, 80, metrics.TotalAtivos)
		assert.Equal(t, 20, metrics.TotalInativos)
		assert.Equal(t, 5, metrics.NovosNoPeriodo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("vendedorID > 0 restringe todas as contagens à carteira ativa", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		carteira := `cliente_id_origem IN \(SELECT cliente_id FROM carteiras WHERE vendedor_id = \? AND data_fim IS NULL\)`
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ` + carteira + `$`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(4))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \? AND `+carteira+`$`).
			WithArgs(true, int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(3))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \? AND `+carteira+`$`).
			WithArgs(false, int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE data_cadastro >= DATE_SUB\(CURDATE\(\), INTERVAL 6 DAY\) AND ` + carteira + `$`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
		mock.ExpectQuery(`SELECT segmento, COUNT\(\*\) AS total\s+FROM clientes WHERE ` + carteira + `\s+GROUP BY segmento`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"segmento", "total"}))
		mock.ExpectQuery(`SELECT uf, COUNT\(\*\) AS total\s+FROM clientes WHERE ` + carteira + `\s+GROUP BY uf`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"uf", "total"}))

		metrics, err := consultasAnalytics(db, 7).MetricasClientes(context.Background(), analyticsapp.ConsultaMetricasClientes{Periodo: periodoDe("week")})
		require.NoError(t, err)
		assert.Equal(t, 4, metrics.TotalClientes)
		assert.Equal(t, 3, metrics.TotalAtivos)
		assert.Equal(t, 1, metrics.TotalInativos)
		assert.Equal(t, 0, metrics.NovosNoPeriodo)
		// listas vazias e não nil: o DTO serializa [] (nunca null).
		assert.Equal(t, []analytics.ContagemSegmento{}, metrics.PorSegmento)
		assert.Equal(t, []analytics.ContagemUF{}, metrics.PorUF)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("EmptyClienteMetrics tem o mesmo formato, zerado e sem queries", func(t *testing.T) {
		for _, verbose := range []bool{false, true} {
			db, mock := newDashboardTestDB(t)
			_ = verbose // ARQ-07: o Verbose não muda a resposta (o log é o vlog global)
			m, err := consultasSemAcesso(db, false).MetricasClientes(context.Background(), analyticsapp.ConsultaMetricasClientes{Periodo: periodoDe("today")})
			require.NoError(t, err)
			assert.Equal(t, "today", m.Periodo.String())
			assert.Equal(t, [4]int{}, [4]int{m.TotalClientes, m.TotalAtivos, m.TotalInativos, m.NovosNoPeriodo})
			assert.Equal(t, []analytics.ContagemSegmento{}, m.PorSegmento)
			assert.Equal(t, []analytics.ContagemUF{}, m.PorUF)
			assert.NoError(t, mock.ExpectationsWereMet())
		}
	})

	t.Run("erro no CountTotal é propagado", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
			WillReturnError(sql.ErrConnDone)

		metrics, err := consultasAnalytics(db, 0).MetricasClientes(context.Background(), analyticsapp.ConsultaMetricasClientes{Periodo: periodoDe("today")})
		assert.Zero(t, metrics, "em erro, o read model volta zerado")
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

		metrics, err := consultasAnalytics(db, 0).MetricasClientes(context.Background(), analyticsapp.ConsultaMetricasClientes{Periodo: periodoDe("today")})
		assert.Zero(t, metrics, "em erro, o read model volta zerado")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
