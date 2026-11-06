package analytics_test

// BUG-30 (Lote 38): as datas do analytics (hoje, semana, mês, janelas do
// funil e série) são calculadas no Go, no fuso fixo tz.Local, a partir do
// relógio injetado nos adapters, e vão por placeholder. O SQL não usa o
// relógio do servidor (decisão TST-23): com a imagem MariaDB em UTC, entre
// 21h e 24h -03 o dia do servidor já é o seguinte.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsapp"
	"github.com/rotaperfumes/shared/analytics/analyticsmysql"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/tz"
)

// relogioFixo é o relógio dos espelhos sqlmock: hojeFixo (2026-10-15 09:00
// UTC = 06:00 -03), então hoje = "2026-10-15" e o mês = (2026, 10).
func relogioFixo() time.Time { return hojeFixo }

// relogioEm devolve um relógio parado no instante informado.
func relogioEm(instante time.Time) func() time.Time {
	return func() time.Time { return instante }
}

// idsDeAte devolve os ids de..ate (int64, como o adapter) como args do mock.
func idsDeAte(de, ate int) []driver.Value {
	ids := make([]driver.Value, 0, ate-de+1)
	for i := de; i <= ate; i++ {
		ids = append(ids, int64(i))
	}
	return ids
}

// viradaDoDia: 22:30 de 05/11/2026 em -03 (o servidor em UTC já está em 06/11).
var viradaDoDia = time.Date(2026, 11, 6, 1, 30, 0, 0, time.UTC)

// consultaGravada é uma consulta vista pelo querierGravador.
type consultaGravada struct {
	sql  string
	args []any
}

// querierGravador repassa ao *sql.DB (sqlmock) e grava cada SQL com os args.
type querierGravador struct {
	db        *sql.DB
	mu        sync.Mutex
	consultas []consultaGravada
}

func (q *querierGravador) gravar(query string, args []any) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.consultas = append(q.consultas, consultaGravada{sql: query, args: append([]any(nil), args...)})
}

func (q *querierGravador) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	q.gravar(query, args)
	return q.db.ExecContext(ctx, query, args...)
}

func (q *querierGravador) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	q.gravar(query, args)
	return q.db.QueryContext(ctx, query, args...)
}

func (q *querierGravador) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	q.gravar(query, args)
	return q.db.QueryRowContext(ctx, query, args...)
}

// escoposBUG30 são os dois escopos que executam SQL (com e sem filtro).
func escoposBUG30(t *testing.T) map[string]kernel.Escopo {
	t.Helper()
	doVendedor, err := kernel.EscopoDoVendedor(7)
	require.NoError(t, err)
	return map[string]kernel.Escopo{"irrestrito": irrestrito(t), "vendedor 7": doVendedor}
}

func TestBUG30_ViradaDoDia_DatasNoFusoDoGo(t *testing.T) {
	ctx := context.Background()
	doVendedor, err := kernel.EscopoDoVendedor(7)
	require.NoError(t, err)
	for nome, instante := range map[string]time.Time{
		"instante em UTC":       viradaDoDia,
		"mesmo instante em -03": viradaDoDia.In(tz.Local),
	} {
		t.Run(nome, func(t *testing.T) {
			relogio := relogioEm(instante)
			db, mock := novoMock(t)
			d := analyticsmysql.NovoDashboard(db, relogio)

			mock.ExpectQuery(`WHERE DATE\(data_pedido\) = \? AND`).WithArgs("2026-11-05", int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(1.0, 1))
			_, err := d.VendasTotais(ctx, doVendedor, analytics.PeriodoHoje)
			require.NoError(t, err)

			mock.ExpectQuery(`WHERE data_pedido >= \? AND`).WithArgs("2026-10-30", int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(1.0, 1))
			_, err = d.VendasTotais(ctx, doVendedor, analytics.PeriodoSemana)
			require.NoError(t, err)

			mock.ExpectQuery(`WHERE YEAR\(data_pedido\) = \? AND MONTH\(data_pedido\) = \? AND`).WithArgs(2026, 11, int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(1))
			_, err = d.TotalPedidos(ctx, doVendedor, analytics.PeriodoMes)
			require.NoError(t, err)

			mock.ExpectQuery(`AS data`).WithArgs("2026-10-29", int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"d", "v", "q"}).AddRow("2026-11-05", 9.0, 2))
			serie, err := d.SerieVendas(ctx, doVendedor, analytics.DiasDe(7))
			require.NoError(t, err)
			dias := make([]string, 0, len(serie))
			for _, p := range serie {
				dias = append(dias, p.Dia)
			}
			assert.Equal(t, []string{"2026-10-30", "2026-10-31", "2026-11-01", "2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05"}, dias)
			assert.Equal(t, analytics.PontoVenda{Dia: "2026-11-05", TotalVendas: 9, TotalPedidos: 2}, serie[6], "a venda de hoje (-03) cai no último ponto")

			mock.ExpectQuery(`AND data_abertura >= \? AND vendedor_id = \?`).WithArgs("2026-10-07", int64(7)).
				WillReturnRows(sqlmock.NewRows(funilCols))
			_, err = analyticsmysql.NovoOportunidades(db, relogio).Funil(ctx, doVendedor, analytics.Janela30d)
			require.NoError(t, err)

			mock.ExpectQuery(`FROM clientes WHERE data_cadastro = \? AND cliente_id_origem IN`).WithArgs("2026-11-05", int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(3))
			_, err = analyticsmysql.NovoClientes(db, relogio).NovosNoPeriodo(ctx, doVendedor, analytics.PeriodoHoje)
			require.NoError(t, err)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBUG30_ViradaDeMesEAno(t *testing.T) {
	ctx := context.Background()
	doVendedor, err := kernel.EscopoDoVendedor(7)
	require.NoError(t, err)
	casos := []struct {
		nome      string
		instante  time.Time
		hoje      string
		ano, mes  int
		inicioSem string
	}{
		{"01/12 02:00 UTC = 30/11 -03", time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC), "2026-11-30", 2026, 11, "2026-11-24"},
		{"01/01 01:00 UTC = 31/12 -03", time.Date(2027, 1, 1, 1, 0, 0, 0, time.UTC), "2026-12-31", 2026, 12, "2026-12-25"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := novoMock(t)
			d := analyticsmysql.NovoDashboard(db, relogioEm(c.instante))
			mock.ExpectQuery(`WHERE YEAR\(data_pedido\) = \? AND MONTH\(data_pedido\) = \? AND`).WithArgs(c.ano, c.mes, int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(1.0, 1))
			_, err := d.VendasTotais(ctx, doVendedor, analytics.PeriodoMes)
			require.NoError(t, err)
			mock.ExpectQuery(`WHERE DATE\(data_pedido\) = \? AND`).WithArgs(c.hoje, int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(1))
			_, err = d.TotalPedidos(ctx, doVendedor, analytics.PeriodoHoje)
			require.NoError(t, err)
			mock.ExpectQuery(`FROM clientes WHERE data_cadastro >= \? AND`).WithArgs(c.inicioSem, int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
			_, err = analyticsmysql.NovoClientes(db, relogioEm(c.instante)).NovosNoPeriodo(ctx, doVendedor, analytics.PeriodoSemana)
			require.NoError(t, err)
			mock.ExpectQuery(`SELECT COUNT`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
			mock.ExpectQuery(`LIMIT \? OFFSET \?`).WithArgs(c.ano, c.mes, int64(7), 20, 0).
				WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}))
			_, err = d.Ranking(ctx, doVendedor, kernel.NovaPagina(1, 20))
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Para todo valor do enum (inclusive fora dele), o SQL executado tem tantos
// placeholders quantos argumentos: um desalinhamento trocaria a data pelo
// vendedor (ou vice-versa).
func TestBUG30_PlaceholdersBatemComArgs(t *testing.T) {
	ctx := context.Background()
	periodos := []analytics.Periodo{analytics.PeriodoHoje, analytics.PeriodoSemana, analytics.PeriodoMes, analytics.Periodo(99)}
	janelas := []analytics.Janela{analytics.JanelaTudo, analytics.Janela30d, analytics.Janela90d, analytics.Janela365d, analytics.Janela(99)}
	for nomeEscopo, e := range escoposBUG30(t) {
		t.Run(nomeEscopo, func(t *testing.T) {
			db, mock := novoMock(t)
			q := &querierGravador{db: db}
			d := analyticsmysql.NovoDashboard(q, relogioFixo)
			c := analyticsmysql.NovoClientes(q, relogioFixo)
			o := analyticsmysql.NovoOportunidades(q, relogioFixo)
			for _, p := range periodos {
				mock.ExpectQuery(`FROM pedidos`).WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(1.0, 1))
				_, err := d.VendasTotais(ctx, e, p)
				require.NoError(t, err)
				mock.ExpectQuery(`FROM pedidos`).WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(1))
				_, err = d.TotalPedidos(ctx, e, p)
				require.NoError(t, err)
				mock.ExpectQuery(`FROM clientes`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
				_, err = c.NovosNoPeriodo(ctx, e, p)
				require.NoError(t, err)
			}
			for _, j := range janelas {
				mock.ExpectQuery(`FROM oportunidades`).WillReturnRows(sqlmock.NewRows(funilCols))
				_, err := o.Funil(ctx, e, j)
				require.NoError(t, err)
			}
			mock.ExpectQuery(`AS data`).WillReturnRows(sqlmock.NewRows([]string{"d", "v", "q"}).AddRow("2026-10-15", 1.0, 1))
			_, err := d.SerieVendas(ctx, e, analytics.DiasDe(7))
			require.NoError(t, err)
			mock.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
			mock.ExpectQuery(`LIMIT \? OFFSET \?`).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}))
			_, err = d.Ranking(ctx, e, kernel.NovaPagina(1, 20))
			require.NoError(t, err)
			mock.ExpectQuery(`FROM vendedores v`).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}).AddRow(int64(7), "A", "S", "PR", 1.0))
			mock.ExpectQuery(`WHERE vendedor_id IN`).WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}))
			_, err = d.MetasVendedores(ctx, e)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())

			require.Len(t, q.consultas, len(periodos)*3+len(janelas)+1+2+2)
			for _, g := range q.consultas {
				assert.Equal(t, strings.Count(g.sql, "?"), len(g.args), "placeholders x args: %s %v", strings.Join(strings.Fields(g.sql), " "), g.args)
			}
		})
	}
}

// vendasDoMes com mais de um lote: o 2º lote recebe os SEUS ids (append
// sobre a fatia do 1º lote sobrescreveria os ids 101 e 102 com ano e mês).
func TestBUG30_VendasDoMes_LotesSemAliasing(t *testing.T) {
	ctx := context.Background()
	const total = analyticsmysql.LoteVendasDoMes + 50
	db, mock := novoMock(t)
	linhas := sqlmock.NewRows([]string{"id", "nome", "meta"})
	for i := 1; i <= total; i++ {
		linhas.AddRow(int64(i), "V", 100.0)
	}
	mock.ExpectQuery(`FROM vendedores v`).WithArgs(total).WillReturnRows(linhas)
	mock.ExpectQuery(`WHERE vendedor_id IN`).WithArgs(append(idsDeAte(1, analyticsmysql.LoteVendasDoMes), 2026, 11)...).
		WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}).AddRow(int64(1), 10.0, 1))
	mock.ExpectQuery(`WHERE vendedor_id IN`).WithArgs(append(idsDeAte(analyticsmysql.LoteVendasDoMes+1, total), 2026, 11)...).
		WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}).AddRow(int64(analyticsmysql.LoteVendasDoMes+1), 20.0, 2))

	top, err := analyticsmysql.NovoDashboard(db, relogioEm(viradaDoDia)).TopVendedores(ctx, irrestrito(t), total)
	require.NoError(t, err)
	require.Len(t, top, total)
	assert.Equal(t, 10.0, top[0].TotalVendas)
	assert.Equal(t, 20.0, top[analyticsmysql.LoteVendasDoMes].TotalVendas)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A série faz UMA consulta (a sequência de dias é do Go) e devolve os mesmos
// dias da série zerada, com as vendas nos dias certos.
func TestBUG30_SerieUmaConsultaSequenciaDoGo(t *testing.T) {
	ctx := context.Background()
	db, mock := novoMock(t)
	mock.ExpectQuery(`AS data`).WithArgs("2026-10-12").
		WillReturnRows(sqlmock.NewRows([]string{"d", "v", "q"}).AddRow("2026-10-12", 1.0, 1).AddRow("2026-10-14", 5.0, 2))
	pontos, err := analyticsmysql.NovoDashboard(db, relogioFixo).SerieVendas(ctx, irrestrito(t), analytics.DiasDe(3))
	require.NoError(t, err)
	esperado := analytics.SerieZerada(3, hojeFixo)
	esperado[1] = analytics.PontoVenda{Dia: "2026-10-14", TotalVendas: 5, TotalPedidos: 2}
	assert.Equal(t, esperado, pontos, "o dia extra (2026-10-12) fica fora")
	assert.NoError(t, mock.ExpectationsWereMet(), "uma consulta só")
}

// A série zerada do caso de uso (sem acesso) lê o relógio no fuso tz.Local:
// 01:30 UTC de 06/11 é 22:30 de 05/11 em -03, então o último dia é 05/11,
// mesmo com o relógio devolvendo o instante em UTC.
func TestBUG30_SerieZeradaDoCasoDeUsoNoFusoLocal(t *testing.T) {
	q := analyticsapp.Novo(analyticsapp.Deps{
		Vendas: &vendasFake{}, Clientes: &clientesFake{}, Funil: &vendasFake{},
		Escopo: escopoFake{e: kernel.EscopoSemAcesso()}, Relogio: relogioEm(viradaDoDia),
	})
	s, err := q.SerieVendas(context.Background(), analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(2)})
	require.NoError(t, err)
	assert.Equal(t, []analytics.PontoVenda{{Dia: "2026-11-04"}, {Dia: "2026-11-05"}}, s.Pontos)
}
