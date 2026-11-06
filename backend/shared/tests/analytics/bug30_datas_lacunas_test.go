package analytics_test

// BUG-30 (Lote 38, TestBrain): lacunas da cobertura do spec. Argumentos
// exatos de toda janela do funil e do período fora do enum; ano/mês do
// enriquecimento (TopVendedores/MetasVendedores) na virada do mês; o mesmo
// instante em qualquer fuso dá o mesmo dia; e "hoje" lido UMA vez por método
// público (um relógio que avança no meio não pode misturar dias).

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsmysql"
	"github.com/rotaperfumes/shared/kernel"
)

// viradaDoMesBUG30: 01/12/2026 02:00 UTC = 30/11/2026 23:00 -03.
var viradaDoMesBUG30 = time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC)

func TestBUG30_FunilArgsExatosPorJanela(t *testing.T) {
	ctx := context.Background()
	doVendedor, err := kernel.EscopoDoVendedor(7)
	require.NoError(t, err)
	casos := []struct {
		nome   string
		janela analytics.Janela
		inicio string // "" = sem cláusula de data
	}{
		{"30d", analytics.Janela30d, "2026-10-07"},
		{"90d", analytics.Janela90d, "2026-08-08"},
		{"365d", analytics.Janela365d, "2025-11-06"},
		{"tudo", analytics.JanelaTudo, ""},
		{"fora do enum", analytics.Janela(99), ""},
	}
	for _, c := range casos {
		for nomeEscopo, e := range map[string]kernel.Escopo{"irrestrito": irrestrito(t), "vendedor 7": doVendedor} {
			t.Run(c.nome+"/"+nomeEscopo, func(t *testing.T) {
				db, mock := novoMock(t)
				var args []driver.Value
				if c.inicio != "" {
					args = append(args, c.inicio)
				}
				if nomeEscopo == "vendedor 7" {
					args = append(args, int64(7))
				}
				exp := mock.ExpectQuery(`FROM oportunidades`)
				if len(args) > 0 {
					exp = exp.WithArgs(args...)
				} else {
					exp = exp.WithoutArgs()
				}
				exp.WillReturnRows(sqlmock.NewRows(funilCols))
				_, err := analyticsmysql.NovoOportunidades(db, relogioEm(viradaDoDia)).Funil(ctx, e, c.janela)
				require.NoError(t, err)
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

// Período fora do enum cai no mês (o mesmo default da cláusula), com
// {ano, mês} de hoje em -03.
func TestBUG30_PeriodoForaDoEnumUsaMes(t *testing.T) {
	ctx := context.Background()
	doVendedor, err := kernel.EscopoDoVendedor(7)
	require.NoError(t, err)
	db, mock := novoMock(t)
	mock.ExpectQuery(`YEAR\(data_pedido\) = \? AND MONTH\(data_pedido\) = \?`).WithArgs(2026, 11, int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(1.0, 1))
	_, err = analyticsmysql.NovoDashboard(db, relogioEm(viradaDoMesBUG30)).VendasTotais(ctx, doVendedor, analytics.Periodo(99))
	require.NoError(t, err)
	mock.ExpectQuery(`YEAR\(data_cadastro\) = \? AND MONTH\(data_cadastro\) = \?`).WithArgs(2026, 11, int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	_, err = analyticsmysql.NovoClientes(db, relogioEm(viradaDoMesBUG30)).NovosNoPeriodo(ctx, doVendedor, analytics.Periodo(99))
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// O enriquecimento do mês (vendasDoMes) usa o mês de hoje em -03: às 23:00
// -03 de 30/11 (01/12 em UTC) ainda é novembro.
func TestBUG30_EnriquecimentoDoMesNaVirada(t *testing.T) {
	ctx := context.Background()
	casos := []struct {
		nome     string
		instante time.Time
		ano, mes int
	}{
		{"30/11 23:00 -03", viradaDoMesBUG30, 2026, 11},
		{"31/12 22:00 -03", time.Date(2027, 1, 1, 1, 0, 0, 0, time.UTC), 2026, 12},
		{"01/12 00:00 -03", time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC), 2026, 12},
	}
	for _, c := range casos {
		t.Run(c.nome+"/TopVendedores", func(t *testing.T) {
			db, mock := novoMock(t)
			mock.ExpectQuery(`FROM vendedores v`).WithArgs(5).
				WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}).AddRow(int64(3), "V", 100.0))
			mock.ExpectQuery(`WHERE vendedor_id IN`).WithArgs(int64(3), c.ano, c.mes).
				WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}).AddRow(int64(3), 10.0, 1))
			top, err := analyticsmysql.NovoDashboard(db, relogioEm(c.instante)).TopVendedores(ctx, irrestrito(t), 5)
			require.NoError(t, err)
			require.Len(t, top, 1)
			assert.Equal(t, 10.0, top[0].TotalVendas)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
		t.Run(c.nome+"/MetasVendedores", func(t *testing.T) {
			db, mock := novoMock(t)
			mock.ExpectQuery(`FROM vendedores v`).
				WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}).AddRow(int64(3), "A", "S", "PR", 1.0))
			mock.ExpectQuery(`WHERE vendedor_id IN`).WithArgs(int64(3), c.ano, c.mes).
				WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}))
			_, err := analyticsmysql.NovoDashboard(db, relogioEm(c.instante)).MetasVendedores(ctx, irrestrito(t))
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// O mesmo instante, visto de qualquer fuso, dá o mesmo "hoje" (o adapter
// converte para tz.Local; não depende de time.Local nem do fuso do relógio).
func TestBUG30_MesmoInstanteEmQualquerFuso(t *testing.T) {
	ctx := context.Background()
	for nome, loc := range map[string]*time.Location{
		"UTC":    time.UTC,
		"+09:00": time.FixedZone("T+9", 9*3600),
		"-10:00": time.FixedZone("T-10", -10*3600),
		"+14:00": time.FixedZone("T+14", 14*3600),
	} {
		t.Run(nome, func(t *testing.T) {
			db, mock := novoMock(t)
			relogio := relogioEm(viradaDoDia.In(loc))
			mock.ExpectQuery(`WHERE DATE\(data_pedido\) = \?`).WithArgs("2026-11-05").
				WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(1))
			_, err := analyticsmysql.NovoDashboard(db, relogio).TotalPedidos(ctx, irrestrito(t), analytics.PeriodoHoje)
			require.NoError(t, err)
			mock.ExpectQuery(`AS data`).WithArgs("2026-11-03").WillReturnError(&mysql.MySQLError{Number: 1146, Message: "Table 'x.pedidos' doesn't exist"})
			pontos, err := analyticsmysql.NovoDashboard(db, relogio).SerieVendas(ctx, irrestrito(t), analytics.DiasDe(2))
			require.NoError(t, err)
			assert.Equal(t, []analytics.PontoVenda{{Dia: "2026-11-04"}, {Dia: "2026-11-05"}}, pontos, "série zerada do adapter em -03")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// relogioContador conta as leituras e avança 1 h a cada uma, a partir de
// 23:30 -03: uma segunda leitura no mesmo método já seria o dia seguinte.
type relogioContador struct{ leituras int }

func (r *relogioContador) agora() time.Time {
	t := time.Date(2026, 11, 6, 2, 30, 0, 0, time.UTC).Add(time.Duration(r.leituras) * time.Hour)
	r.leituras++
	return t
}

func TestBUG30_HojeLidoUmaVezPorMetodo(t *testing.T) {
	ctx := context.Background()
	const total = analyticsmysql.LoteVendasDoMes + 1
	casos := map[string]func(t *testing.T, d *analyticsmysql.Dashboard, mock sqlmock.Sqlmock){
		"TopVendedores com 2 lotes": func(t *testing.T, d *analyticsmysql.Dashboard, mock sqlmock.Sqlmock) {
			linhas := sqlmock.NewRows([]string{"id", "nome", "meta"})
			for i := 1; i <= total; i++ {
				linhas.AddRow(int64(i), "V", 1.0)
			}
			mock.ExpectQuery(`FROM vendedores v`).WillReturnRows(linhas)
			mock.ExpectQuery(`WHERE vendedor_id IN`).WithArgs(append(idsDeAte(1, analyticsmysql.LoteVendasDoMes), 2026, 11)...).
				WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}))
			mock.ExpectQuery(`WHERE vendedor_id IN`).WithArgs(int64(total), 2026, 11).
				WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}))
			_, err := d.TopVendedores(ctx, irrestrito(t), total)
			require.NoError(t, err)
		},
		"SerieVendas": func(t *testing.T, d *analyticsmysql.Dashboard, mock sqlmock.Sqlmock) {
			mock.ExpectQuery(`AS data`).WithArgs("2026-11-02").
				WillReturnRows(sqlmock.NewRows([]string{"d", "v", "q"}).AddRow("2026-11-05", 1.0, 1))
			pontos, err := d.SerieVendas(ctx, irrestrito(t), analytics.DiasDe(3))
			require.NoError(t, err)
			require.Len(t, pontos, 3)
			assert.Equal(t, "2026-11-05", pontos[2].Dia)
		},
		"Ranking": func(t *testing.T, d *analyticsmysql.Dashboard, mock sqlmock.Sqlmock) {
			mock.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
			mock.ExpectQuery(`LIMIT \? OFFSET \?`).WithArgs(2026, 11, 20, 0).
				WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}))
			_, err := d.Ranking(ctx, irrestrito(t), kernel.NovaPagina(1, 20))
			require.NoError(t, err)
		},
	}
	for nome, rodar := range casos {
		t.Run(nome, func(t *testing.T) {
			db, mock := novoMock(t)
			r := &relogioContador{}
			rodar(t, analyticsmysql.NovoDashboard(db, r.agora), mock)
			assert.Equal(t, 1, r.leituras, "o relógio é lido uma vez por método público")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
