package repositories_test

import (
	"database/sql"
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

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
		{"periodo week", "week"},
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
	testes := []struct {
		nome    string
		periodo string
	}{
		{"periodo today", "today"},
		{"periodo week", "week"},
		{"periodo month", "month"},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM pedidos WHERE .+").
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(7))

			repo := repositories.NewDashboardRepository()
			ctx := context.Background()
			total, err := repo.GetTotalPedidos(ctx, db, tt.periodo)

			require.NoError(t, err)
			assert.Equal(t, 7, total)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestDashboardGetVendasTotais_PeriodoWeek_UsaDateSub garante que o periodo
// "week" monta a clausula com DATE_SUB(CURDATE(), INTERVAL 6 DAY), cobrindo
// os ultimos 7 dias (incluindo hoje).
func TestDashboardGetVendasTotais_PeriodoWeek_UsaDateSub(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"valor_total", "quantidade"}).AddRow(999.0, 3)
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos\s+WHERE data_pedido >= DATE_SUB\(CURDATE\(\), INTERVAL 6 DAY\) AND status NOT IN`).
		WillReturnRows(rows)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	valor, qtd, err := repo.GetVendasTotais(ctx, db, "week")

	require.NoError(t, err)
	assert.Equal(t, 999.0, valor)
	assert.Equal(t, 3, qtd)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestDashboardGetTotalPedidos_PeriodoWeek_UsaDateSub garante o mesmo
// comportamento de clausula para GetTotalPedidos.
func TestDashboardGetTotalPedidos_PeriodoWeek_UsaDateSub(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos\s+WHERE data_pedido >= DATE_SUB\(CURDATE\(\), INTERVAL 6 DAY\) AND status NOT IN`).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(4))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	total, err := repo.GetTotalPedidos(ctx, db, "week")

	require.NoError(t, err)
	assert.Equal(t, 4, total)
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

func TestDashboardGetMetaMensalTotal_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(meta_mensal\\), 0\\)\\s+FROM vendedores\\s+WHERE data_desligamento IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"meta_total"}).AddRow(35000.75))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	meta, err := repo.GetMetaMensalTotal(ctx, db)

	require.NoError(t, err)
	assert.Equal(t, 35000.75, meta)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetMetaMensalTotal_TabelaVazia(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	// COALESCE garante 0 quando não há vendedores ativos (SUM de nenhuma linha).
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(meta_mensal\\), 0\\)\\s+FROM vendedores\\s+WHERE data_desligamento IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"meta_total"}).AddRow(0.0))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	meta, err := repo.GetMetaMensalTotal(ctx, db)

	require.NoError(t, err)
	assert.Equal(t, 0.0, meta)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetMetaMensalTotal_TabelaNaoExiste(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(meta_mensal\\), 0\\)\\s+FROM vendedores\\s+WHERE data_desligamento IS NULL").
		WillReturnError(errTabelaNaoExiste)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	meta, err := repo.GetMetaMensalTotal(ctx, db)

	require.NoError(t, err)
	assert.Equal(t, 0.0, meta)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetMetaMensalTotal_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(meta_mensal\\), 0\\)\\s+FROM vendedores\\s+WHERE data_desligamento IS NULL").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, err := repo.GetMetaMensalTotal(ctx, db)

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

	mock.ExpectQuery("SELECT vendedor_id, .+ FROM pedidos WHERE vendedor_id IN \\(\\?\\) AND YEAR\\(data_pedido\\).+").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"vendedor_id", "total", "qtd"}).
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

	mock.ExpectQuery("SELECT vendedor_id, .+ FROM pedidos WHERE vendedor_id IN \\(\\?\\) AND YEAR\\(data_pedido\\).+").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"vendedor_id", "total", "qtd"}).
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

	mock.ExpectQuery("SELECT DATE_FORMAT\\(data_pedido, '%Y-%m-%d'\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}).
			AddRow("2024-01-01", 100.0, 2))

	mock.ExpectQuery("SELECT DATE_FORMAT\\(DATE_SUB\\(CURDATE\\(\\), INTERVAL n DAY\\), '%Y-%m-%d'\\) AS dia .+").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"dia"}).AddRow("2024-01-01"))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	series, err := repo.GetVendasSeries(ctx, db, 1)

	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Equal(t, "2024-01-01", series[0]["dia"])
	assert.Equal(t, 100.0, series[0]["total_vendas"])
	assert.Equal(t, 2, series[0]["total_pedidos"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// dataFormatoISORegex valida que uma string de data esta exatamente no
// formato YYYY-MM-DD, sem hora nem timezone embutidos (regressao do bug
// "NaN/09" causado por parseTime=true convertendo DATE em time.Time e sendo
// escaneado como string com sufixo " 00:00:00 -0300 -03").
var dataFormatoISORegex = `^\d{4}-\d{2}-\d{2}$`

// TestDashboardGetVendasSeries_FormatoDataISO_BuildFullSeries garante que,
// quando GetVendasSeries usa o caminho de buildFullSeries (query
// DATE_FORMAT(DATE_SUB(CURDATE(), INTERVAL n DAY), '%Y-%m-%d')), cada valor de
// "dia" retornado no resultado final tem exatamente o formato YYYY-MM-DD,
// tanto para dias com venda (presentes no seriesMap vindo da query de
// pedidos) quanto para dias sem venda (preenchidos com zero pelo fallback).
func TestDashboardGetVendasSeries_FormatoDataISO_BuildFullSeries(t *testing.T) {
	testes := []struct {
		nome          string
		dias          int
		diasComVenda  []string // subconjunto de dias que a query de pedidos retorna
		diasDaSerieBD []string // dias que a query de buildFullSeries retorna (simulando DATE_FORMAT)
	}{
		{
			nome:          "todos os dias tem venda",
			dias:          2,
			diasComVenda:  []string{"2026-09-15", "2026-09-16"},
			diasDaSerieBD: []string{"2026-09-15", "2026-09-16"},
		},
		{
			nome:          "nenhum dia da serie tem venda (buildFullSeries preenche zero)",
			dias:          3,
			diasComVenda:  []string{"2026-09-14"}, // dia fora da serie retornada por buildFullSeries
			diasDaSerieBD: []string{"2026-09-15", "2026-09-16", "2026-09-17"},
		},
		{
			nome:          "mix de dias com e sem venda",
			dias:          3,
			diasComVenda:  []string{"2026-09-16"},
			diasDaSerieBD: []string{"2026-09-15", "2026-09-16", "2026-09-17"},
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			pedidosRows := sqlmock.NewRows([]string{"data", "valor", "quantidade"})
			for _, d := range tt.diasComVenda {
				pedidosRows.AddRow(d, 100.0, 2)
			}
			mock.ExpectQuery("SELECT DATE_FORMAT\\(data_pedido, '%Y-%m-%d'\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
				WithArgs(tt.dias).
				WillReturnRows(pedidosRows)

			serieRows := sqlmock.NewRows([]string{"dia"})
			for _, d := range tt.diasDaSerieBD {
				serieRows.AddRow(d)
			}
			mock.ExpectQuery("SELECT DATE_FORMAT\\(DATE_SUB\\(CURDATE\\(\\), INTERVAL n DAY\\), '%Y-%m-%d'\\) AS dia .+").
				WithArgs(tt.dias).
				WillReturnRows(serieRows)

			repo := repositories.NewDashboardRepository()
			ctx := context.Background()
			series, err := repo.GetVendasSeries(ctx, db, tt.dias)

			require.NoError(t, err)
			require.Len(t, series, len(tt.diasDaSerieBD))

			vendaSet := make(map[string]bool)
			for _, d := range tt.diasComVenda {
				vendaSet[d] = true
			}

			for i, ponto := range series {
				dia, ok := ponto["dia"].(string)
				require.True(t, ok, "campo 'dia' deve ser string")

				// Deve bater exatamente com YYYY-MM-DD - nada de hora/timezone
				// embutidos (ex: "2026-09-16 00:00:00 -0300 -03").
				assert.Regexp(t, dataFormatoISORegex, dia, "dia %q nao esta no formato YYYY-MM-DD", dia)
				assert.Equal(t, tt.diasDaSerieBD[i], dia)

				if vendaSet[dia] {
					assert.Equal(t, 100.0, ponto["total_vendas"])
					assert.Equal(t, 2, ponto["total_pedidos"])
				} else {
					assert.Equal(t, 0.0, ponto["total_vendas"])
					assert.Equal(t, 0, ponto["total_pedidos"])
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardGetVendasSeries_TabelaNaoExiste(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT DATE_FORMAT\\(data_pedido, '%Y-%m-%d'\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
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

	mock.ExpectQuery("SELECT DATE_FORMAT\\(data_pedido, '%Y-%m-%d'\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
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

	mock.ExpectQuery("SELECT DATE_FORMAT\\(data_pedido, '%Y-%m-%d'\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	series, err := repo.GetVendasSeries(ctx, db, 2)

	require.NoError(t, err)
	assert.Len(t, series, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestDashboardGetVendasSeries_EmptySeries_FormatoDataReal garante que, quando
// a serie e vazia (tabela nao existe OU sem registros no periodo), cada ponto
// tem uma data real no formato YYYY-MM-DD calculada a partir de time.Now(),
// e nao mais o texto legado "N dias atras".
func TestDashboardGetVendasSeries_EmptySeries_FormatoDataReal(t *testing.T) {
	testes := []struct {
		nome          string
		dias          int
		simulaErroSQL bool
	}{
		{"tabela nao existe", 5, true},
		{"sem registros no periodo", 4, false},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			query := mock.ExpectQuery("SELECT DATE_FORMAT\\(data_pedido, '%Y-%m-%d'\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
				WithArgs(tt.dias)
			if tt.simulaErroSQL {
				query.WillReturnError(errTabelaNaoExiste)
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))
			}

			repo := repositories.NewDashboardRepository()
			ctx := context.Background()
			series, err := repo.GetVendasSeries(ctx, db, tt.dias)

			require.NoError(t, err)
			require.Len(t, series, tt.dias)

			hoje := time.Now()
			for i, ponto := range series {
				diasAtras := tt.dias - 1 - i
				dataEsperada := hoje.AddDate(0, 0, -diasAtras).Format("2006-01-02")

				dia, ok := ponto["dia"].(string)
				require.True(t, ok, "campo 'dia' deve ser string")

				// Nao pode mais ser o texto legado "N dias atras".
				assert.NotContains(t, dia, "dias atras")
				assert.NotContains(t, dia, "dias atrás")

				// Deve bater com o formato YYYY-MM-DD real.
				assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, dia)
				assert.Equal(t, dataEsperada, dia)

				assert.Equal(t, 0.0, ponto["total_vendas"])
				assert.Equal(t, 0, ponto["total_pedidos"])
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardGetVendasSeries_BuildFullSeriesErro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT DATE_FORMAT\\(data_pedido, '%Y-%m-%d'\\) AS data, .+ FROM pedidos WHERE data_pedido >= DATE_SUB\\(CURDATE\\(\\), INTERVAL \\? DAY\\).+").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}).
			AddRow("2024-01-01", 100.0, 2))

	mock.ExpectQuery("SELECT DATE_FORMAT\\(DATE_SUB\\(CURDATE\\(\\), INTERVAL n DAY\\), '%Y-%m-%d'\\) AS dia .+").
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

	mock.ExpectQuery("SELECT(.|\\n)*FROM vendedores v(.|\\n)*LEFT JOIN(.|\\n)*ORDER BY v.meta_mensal DESC, atingimento_meta DESC(.|\\n)*LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}).
			AddRow(1, "Vendedor A", "Sudeste", "SP", 10000.0, 4000.0, 4, 40.0))

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

	mock.ExpectQuery("SELECT(.|\\n)*FROM vendedores v(.|\\n)*LEFT JOIN(.|\\n)*ORDER BY v.meta_mensal DESC, atingimento_meta DESC(.|\\n)*LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}))

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

	mock.ExpectQuery("SELECT(.|\\n)*FROM vendedores v(.|\\n)*LEFT JOIN(.|\\n)*ORDER BY v.meta_mensal DESC, atingimento_meta DESC(.|\\n)*LIMIT \\? OFFSET \\?").
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}))

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

	mock.ExpectQuery("SELECT(.|\\n)*FROM vendedores v(.|\\n)*LEFT JOIN(.|\\n)*ORDER BY v.meta_mensal DESC, atingimento_meta DESC(.|\\n)*LIMIT \\? OFFSET \\?").
		WithArgs(100, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}))

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

// TestDashboardGetVendedoresRanking_EmpateMeta_OrdenaPorAtingimento garante
// que, quando dois ou mais vendedores tem a mesma meta_mensal (empate), a
// query gerada pelo repositorio contem a clausula de desempate
// "ORDER BY v.meta_mensal DESC, atingimento_meta DESC" e que o slice
// resultante em Go preserva exatamente a ordem devolvida pelo mock (ou seja,
// o codigo Go nao reordena nada — a ordenacao e responsabilidade do SQL).
func TestDashboardGetVendedoresRanking_EmpateMeta_OrdenaPorAtingimento(t *testing.T) {
	// orderByRegex e tolerante a espacos/quebras de linha entre as colunas da
	// clausula ORDER BY, refletindo a formatacao multi-linha da query real.
	orderByRegex := regexp.MustCompile(`(?is)ORDER\s+BY\s+v\.meta_mensal\s+DESC\s*,\s*atingimento_meta\s+DESC`)

	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM vendedores WHERE data_desligamento IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(3))

	// Simula o que o banco faria: mesma meta_mensal (10000.0) para os 3
	// vendedores, ja devolvidos na ordem correta de desempate
	// (atingimento_meta decrescente: 80.0, 50.0, 20.0).
	rankingRows := sqlmock.NewRows([]string{
		"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta",
	}).
		AddRow(1, "Vendedor Alto Atingimento", "Sudeste", "SP", 10000.0, 8000.0, 10, 80.0).
		AddRow(2, "Vendedor Medio Atingimento", "Sul", "PR", 10000.0, 5000.0, 6, 50.0).
		AddRow(3, "Vendedor Baixo Atingimento", "Nordeste", "BA", 10000.0, 2000.0, 3, 20.0)

	mock.ExpectQuery(orderByRegex.String()).
		WithArgs(10, 0).
		WillReturnRows(rankingRows)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	result, total, err := repo.GetVendedoresRanking(ctx, db, 1, 10)

	require.NoError(t, err)
	assert.Equal(t, 3, total)
	require.Len(t, result, 3)

	// A ordem do slice deve ser exatamente a mesma ordem em que o mock
	// devolveu as linhas — nenhuma reordenacao adicional em Go.
	assert.Equal(t, "Vendedor Alto Atingimento", result[0]["vendedor_nome"])
	assert.Equal(t, 80.0, result[0]["atingimento_meta"])

	assert.Equal(t, "Vendedor Medio Atingimento", result[1]["vendedor_nome"])
	assert.Equal(t, 50.0, result[1]["atingimento_meta"])

	assert.Equal(t, "Vendedor Baixo Atingimento", result[2]["vendedor_nome"])
	assert.Equal(t, 20.0, result[2]["atingimento_meta"])

	// Todos os vendedores tem a mesma meta_mensal, confirmando que o empate
	// foi desfeito apenas pelo atingimento_meta (nao houve reordenacao por
	// meta, ja que ela e identica para os tres).
	assert.Equal(t, 10000.0, result[0]["meta"])
	assert.Equal(t, 10000.0, result[1]["meta"])
	assert.Equal(t, 10000.0, result[2]["meta"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardGetVendedoresRanking_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM vendedores WHERE data_desligamento IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))

	mock.ExpectQuery("SELECT(.|\\n)*FROM vendedores v(.|\\n)*LEFT JOIN(.|\\n)*ORDER BY v.meta_mensal DESC, atingimento_meta DESC(.|\\n)*LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewDashboardRepository()
	ctx := context.Background()
	_, _, err := repo.GetVendedoresRanking(ctx, db, 1, 10)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
