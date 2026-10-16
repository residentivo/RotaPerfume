package services_test

// Testes do escopo por vendedor no Dashboard: a resposta zerada (escopo
// SemAcesso, D-01) deve ter o mesmo formato da resposta normal, e o vendedor
// do escopo deve chegar às queries do adapter. ARQ-07 (pendência m): os
// testes asserem sobre os read models de analytics (antes, maps legados
// serializados em JSON); o mesmo formato vem do tipo, e o contrato JSON
// (chaves, [] × null) é provado byte a byte no HTTP por
// TestARQ06_ContratoJSON_ByteAByte.

import (
	"context"
	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsapp"
	"github.com/rotaperfumes/shared/kernel"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardService_EmptyMetrics_MesmoFormato(t *testing.T) {
	for _, periodo := range []string{"today", "week", "month"} {
		for _, desligado := range []bool{false, true} {
			nome := periodo
			if desligado {
				nome += "/verbose"
			}
			t.Run(nome, func(t *testing.T) {
				db, mock := newDashboardTestDB(t)
				expectMetricsHappyPath(mock)

				normal, err := consultasAnalytics(db, 0).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoDe(periodo)})
				require.NoError(t, err)
				require.NoError(t, mock.ExpectationsWereMet())

				vazio, err := consultasSemAcesso(db, desligado).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoDe(periodo)})
				require.NoError(t, err)
				require.NoError(t, mock.ExpectationsWereMet(), "a resposta zerada não consulta o banco")

				// A consulta normal nunca sinaliza desligamento; a zerada repassa o flag.
				assert.False(t, normal.VendedorDesligado)
				assert.Equal(t, desligado, vazio.VendedorDesligado)

				assert.Equal(t, periodo, vazio.Periodo.String())
				assert.Equal(t, [3]float64{}, [3]float64{vazio.TotalVendas, vazio.TicketMedio, vazio.MetaMes})
				assert.Equal(t, [2]int{}, [2]int{vazio.TotalVendasQtd, vazio.TotalPedidos})
				// Listas vazias e não nil (o DTO serializa [], nunca null).
				assert.Equal(t, []analytics.VendedorRanking{}, vazio.TopVendedores)
				assert.Equal(t, []analytics.MetaVendedor{}, vazio.MetasVendedores)
				assert.NotNil(t, normal.TopVendedores)
				assert.NotNil(t, normal.MetasVendedores)
			})
		}
	}
}

func TestDashboardService_EmptyVendasSeries_MesmoFormato(t *testing.T) {
	casos := []struct {
		nome    string
		dias    int
		verbose bool
	}{
		{"1 dia", 1, false},
		{"7 dias verbose", 7, true},
		{"30 dias", 30, false},
	}
	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newDashboardTestDB(t)
			hoje := time.Now().Format("2006-01-02")
			mock.ExpectQuery(`SELECT DATE_FORMAT\(data_pedido, '%Y-%m-%d'\) AS data`).
				WithArgs(tt.dias).
				WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}).AddRow(hoje, 10.0, 1))
			mock.ExpectQuery(`SELECT DATE_FORMAT\(DATE_SUB\(CURDATE\(\), INTERVAL n DAY\), '%Y-%m-%d'\) AS dia`).
				WithArgs(tt.dias).
				WillReturnRows(sqlmock.NewRows([]string{"dia"}).AddRow(hoje))

			normal, err := consultasAnalytics(db, 0).SerieVendas(context.Background(), analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(tt.dias)})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())

			vazio, err := consultasSemAcesso(db, false).SerieVendas(context.Background(), analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(tt.dias)})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet(), "a série zerada não consulta o banco")

			assert.Equal(t, tt.dias, vazio.Dias.Int())
			require.NotEmpty(t, normal.Pontos)
			require.Len(t, vazio.Pontos, tt.dias)
			for _, p := range vazio.Pontos {
				assert.Equal(t, 0.0, p.TotalVendas)
				assert.Equal(t, 0, p.TotalPedidos)
				assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, p.Dia)
			}
			assert.Equal(t, hoje, vazio.Pontos[len(vazio.Pontos)-1].Dia)
		})
	}
}

func TestDashboardService_EscopoVendedor_RepassaVendedorID(t *testing.T) {
	const vid = int64(20)

	t.Run("GetMetrics", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`FROM pedidos.*AND vendedor_id = \?`).WithArgs(vid).
			WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(50.0, 1))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos.*AND vendedor_id = \?`).WithArgs(vid).
			WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(1))
		mock.ExpectQuery(`v\.meta_mensal AS meta\s+FROM vendedores v\s+WHERE v\.data_desligamento IS NULL AND v\.id = \?`).
			WithArgs(vid, 10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}))
		mock.ExpectQuery(`v\.uf,\s+v\.meta_mensal AS meta\s+FROM vendedores v\s+WHERE v\.data_desligamento IS NULL AND v\.id = \?`).
			WithArgs(vid).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}))
		mock.ExpectQuery(`SUM\(meta_mensal\).*AND id = \?`).WithArgs(vid).
			WillReturnRows(sqlmock.NewRows([]string{"m"}).AddRow(100.0))

		m, err := consultasAnalytics(db, vid).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoDe("month")})
		require.NoError(t, err)
		assert.Equal(t, 50.0, m.TicketMedio)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetVendasSeries", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT DATE_FORMAT\(data_pedido, '%Y-%m-%d'\) AS data.*AND vendedor_id = \?`).
			WithArgs(7, vid).
			WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))

		s, err := consultasAnalytics(db, vid).SerieVendas(context.Background(), analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(7)})
		require.NoError(t, err)
		assert.Len(t, s.Pontos, 7)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetVendedoresRanking", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL AND id = \?`).
			WithArgs(vid).
			WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
		mock.ExpectQuery(`(?s)AND v\.id = \?.*LIMIT \? OFFSET \?`).
			WithArgs(vid, 20, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}).
				AddRow(vid, "B", "Sul", "PR", 100.0, 50.0, 1, 50.0))

		lista, total, err := itensETotal(consultasAnalytics(db, vid).Ranking(context.Background(), analyticsapp.ConsultaRanking{Pagina: kernel.NovaPagina(1, 20)}))
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, lista, 1)
		assert.Equal(t, vid, lista[0].VendedorID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
