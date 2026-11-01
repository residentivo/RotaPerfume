package analytics_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsmysql"
	"github.com/rotaperfumes/shared/kernel"
)

func novoMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func TestAnalyticsmysql_ExigemQuerier(t *testing.T) {
	assert.Panics(t, func() { analyticsmysql.NovoDashboard(nil, time.Now) })
	db, _ := novoMock(t)
	assert.Panics(t, func() { analyticsmysql.NovoDashboard(db, nil) })
	assert.Panics(t, func() { analyticsmysql.NovoClientes(nil) })
}

// O1 (E-02, D-01): as 12 consultas com EscopoSemAcesso() e com o zero value
// devolvem o erro do Filtro SEM nenhuma expectativa no sqlmock (nenhum SQL).
func TestARQ06_O1_AdaptersSemAcessoEZeroValueSemSQL(t *testing.T) {
	ctx := context.Background()
	for nome, e := range map[string]kernel.Escopo{"sem acesso": kernel.EscopoSemAcesso(), "zero value": {}} {
		db, mock := novoMock(t)
		d, c := analyticsmysql.NovoDashboard(db, time.Now), analyticsmysql.NovoClientes(db)
		erros := []error{}
		_, err := d.VendasTotais(ctx, e, analytics.PeriodoMes)
		erros = append(erros, err)
		_, err = d.TotalPedidos(ctx, e, analytics.PeriodoMes)
		erros = append(erros, err)
		_, err = d.MetaMensalTotal(ctx, e)
		erros = append(erros, err)
		_, err = d.TopVendedores(ctx, e, 10)
		erros = append(erros, err)
		_, err = d.MetasVendedores(ctx, e)
		erros = append(erros, err)
		_, err = d.SerieVendas(ctx, e, analytics.DiasDe(7))
		erros = append(erros, err)
		_, err = d.Ranking(ctx, e, kernel.NovaPagina(1, 20))
		erros = append(erros, err)
		_, err = c.Total(ctx, e)
		erros = append(erros, err)
		_, err = c.PorAtivo(ctx, e, true)
		erros = append(erros, err)
		_, err = c.NovosNoPeriodo(ctx, e, analytics.PeriodoHoje)
		erros = append(erros, err)
		_, err = c.PorSegmento(ctx, e)
		erros = append(erros, err)
		_, err = c.PorUF(ctx, e)
		erros = append(erros, err)

		esperado := kernel.ErrEscopoSemAcesso
		if nome == "zero value" {
			esperado = kernel.ErrEscopoInvalido
		}
		require.Len(t, erros, 12)
		for i, err := range erros {
			assert.ErrorIs(t, err, esperado, "%s consulta %d", nome, i)
		}
		assert.NoError(t, mock.ExpectationsWereMet(), nome)
	}
}

// Ramos de infraestrutura que o legado tratava em silêncio: o enriquecimento
// e a série ignoram falha de query e linha que não escaneia (sem 500).
func TestAnalyticsmysql_FalhasSilenciosasDoLegado(t *testing.T) {
	ctx := context.Background()
	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	e, _ := kernel.EscopoIrrestrito(admin)

	t.Run("enriquecimento: erro de query mantém as metas sem vendas", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(`FROM vendedores v`).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}).AddRow(int64(1), "A", "S", "PR", 100.0))
		mock.ExpectQuery(`WHERE vendedor_id IN \(\?\)`).WithArgs(int64(1)).WillReturnError(errors.New("x"))
		metas, err := analyticsmysql.NovoDashboard(db, time.Now).MetasVendedores(ctx, e)
		require.NoError(t, err)
		assert.Equal(t, []analytics.MetaVendedor{{ID: 1, Nome: "A", Regiao: "S", UF: "PR", Meta: 100}}, metas)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("enriquecimento: linha que não escaneia é ignorada (IN com 2 ids)", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(`FROM vendedores v`).WithArgs(5).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}).
			AddRow(int64(1), "A", 100.0).AddRow(int64(2), "B", 0.0))
		mock.ExpectQuery(`WHERE vendedor_id IN \(\?,\?\)`).WithArgs(int64(1), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}).
			AddRow("nao-numero", 1.0, 1).AddRow(int64(2), 30.0, 3))
		top, err := analyticsmysql.NovoDashboard(db, time.Now).TopVendedores(ctx, e, 5)
		require.NoError(t, err)
		assert.Equal(t, []analytics.VendedorRanking{{ID: 1, Nome: "A", Meta: 100}, {ID: 2, Nome: "B", TotalVendas: 30, QuantidadeVendas: 3}}, top)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("top e metas: erro de scan é erro", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(`FROM vendedores v`).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}).AddRow("x", "A", 1.0))
		_, err := analyticsmysql.NovoDashboard(db, time.Now).TopVendedores(ctx, e, 10)
		assert.ErrorContains(t, err, "analyticsmysql: top vendedores scan")
		mock.ExpectQuery(`FROM vendedores v`).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}).AddRow("x", "A", "S", "PR", 1.0))
		_, err = analyticsmysql.NovoDashboard(db, time.Now).MetasVendedores(ctx, e)
		assert.ErrorContains(t, err, "analyticsmysql: metas vendedores scan")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("série: linha de venda e dia que não escaneiam são ignorados", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(`AS data`).WithArgs(2).WillReturnRows(sqlmock.NewRows([]string{"d", "v", "q"}).
			AddRow("2026-10-14", "nao-numero", 1).AddRow("2026-10-15", 5.0, 1))
		mock.ExpectQuery(`AS dia`).WithArgs(2).WillReturnRows(sqlmock.NewRows([]string{"dia"}).AddRow(nil).AddRow("2026-10-15"))
		pontos, err := analyticsmysql.NovoDashboard(db, time.Now).SerieVendas(ctx, e, analytics.DiasDe(2))
		require.NoError(t, err)
		assert.Equal(t, []analytics.PontoVenda{{Dia: "2026-10-15", TotalVendas: 5, TotalPedidos: 1}}, pontos)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("série: só linhas inválidas = série zerada de hoje", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(`AS data`).WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"d", "v", "q"}).AddRow(nil, 1.0, 1))
		pontos, err := analyticsmysql.NovoDashboard(db, time.Now).SerieVendas(ctx, e, analytics.DiasDe(1))
		require.NoError(t, err)
		assert.Equal(t, analytics.SerieZerada(1, time.Now()), pontos)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAnalyticsmysql_ErrosDeInfra(t *testing.T) {
	ctx := context.Background()
	doVendedor, _ := kernel.EscopoDoVendedor(7)
	cols := []string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}

	t.Run("ranking: scan e iteração", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(`SELECT COUNT`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
		mock.ExpectQuery(`LIMIT \? OFFSET \?`).WithArgs(int64(7), 20, 0).WillReturnRows(sqlmock.NewRows(cols).AddRow("x", "A", "S", "PR", 1.0, 1.0, 1, 1.0))
		_, err := analyticsmysql.NovoDashboard(db, time.Now).Ranking(ctx, doVendedor, kernel.NovaPagina(1, 20))
		assert.ErrorContains(t, err, "analyticsmysql: ranking scan")

		mock.ExpectQuery(`SELECT COUNT`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
		mock.ExpectQuery(`LIMIT \? OFFSET \?`).WithArgs(int64(7), 20, 0).WillReturnRows(sqlmock.NewRows(cols).
			AddRow(int64(7), "A", "S", "PR", 1.0, 1.0, 1, 1.0).RowError(0, errInfra))
		_, err = analyticsmysql.NovoDashboard(db, time.Now).Ranking(ctx, doVendedor, kernel.NovaPagina(1, 20))
		assert.ErrorIs(t, err, errInfra)
		assert.ErrorContains(t, err, "analyticsmysql: ranking rows")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("clientes: novos no período por enum e erro embrulhado só com a operação", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(`FROM clientes WHERE data_cadastro >= DATE_SUB\(CURDATE\(\), INTERVAL 6 DAY\) AND cliente_id_origem IN`).
			WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(2))
		n, err := analyticsmysql.NovoClientes(db).NovosNoPeriodo(ctx, doVendedor, analytics.PeriodoSemana)
		require.NoError(t, err)
		assert.Equal(t, 2, n)
		mock.ExpectQuery(`FROM clientes WHERE data_cadastro = CURDATE\(\) AND`).WithArgs(int64(7)).WillReturnError(errInfra)
		_, err = analyticsmysql.NovoClientes(db).NovosNoPeriodo(ctx, doVendedor, analytics.PeriodoHoje)
		assert.EqualError(t, err, "analyticsmysql: count novos clientes: infra")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("pedidos: período do dia e mês fora do enum", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(`WHERE DATE\(data_pedido\) = CURDATE\(\) AND status NOT IN \('cancelado', 'devolvido'\) AND vendedor_id = \?$`).
			WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(nil))
		n, err := analyticsmysql.NovoDashboard(db, time.Now).TotalPedidos(ctx, doVendedor, analytics.PeriodoHoje)
		require.NoError(t, err)
		assert.Zero(t, n, "NULL vira 0")
		mock.ExpectQuery(`WHERE YEAR\(data_pedido\) = YEAR\(CURDATE\(\)\) AND MONTH`).WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(nil, nil))
		tot, err := analyticsmysql.NovoDashboard(db, time.Now).VendasTotais(ctx, doVendedor, analytics.Periodo(99))
		require.NoError(t, err)
		assert.Equal(t, analytics.TotaisVendas{}, tot)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// SecBrain #6: a série zerada do adapter usa o relógio injetado (o mesmo das
// Deps do caso de uso), não o time.Now do pacote.
func TestAnalyticsmysql_SerieZeradaUsaORelogioInjetado(t *testing.T) {
	ctx := context.Background()
	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	e, _ := kernel.EscopoIrrestrito(admin)
	relogio := func() time.Time { return hojeFixo }
	db, mock := novoMock(t)
	d := analyticsmysql.NovoDashboard(db, relogio)

	mock.ExpectQuery(`AS data`).WithArgs(2).WillReturnError(errors.New("Error 1146: Table 'x.pedidos' doesn't exist"))
	pontos, err := d.SerieVendas(ctx, e, analytics.DiasDe(2))
	require.NoError(t, err)
	assert.Equal(t, analytics.SerieZerada(2, hojeFixo), pontos, "sem a tabela")

	mock.ExpectQuery(`AS data`).WithArgs(2).WillReturnRows(sqlmock.NewRows([]string{"d", "v", "q"}))
	pontos, err = d.SerieVendas(ctx, e, analytics.DiasDe(2))
	require.NoError(t, err)
	assert.Equal(t, analytics.SerieZerada(2, hojeFixo), pontos, "sem vendas")

	mock.ExpectQuery(`AS data`).WithArgs(2).WillReturnRows(sqlmock.NewRows([]string{"d", "v", "q"}).AddRow("2026-10-15", 1.0, 1))
	mock.ExpectQuery(`AS dia`).WithArgs(2).WillReturnError(errInfra)
	pontos, err = d.SerieVendas(ctx, e, analytics.DiasDe(2))
	require.NoError(t, err)
	assert.Equal(t, analytics.SerieZerada(2, hojeFixo), pontos, "falha na sequência de dias")
	assert.NoError(t, mock.ExpectationsWereMet())
}
