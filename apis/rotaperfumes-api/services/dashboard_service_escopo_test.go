package services_test

// Testes do escopo por vendedor no DashboardService: EmptyMetrics /
// EmptyVendasSeries devem ter o mesmo formato (mesmas chaves JSON) da
// resposta normal, e vendedorID deve chegar às queries do repositório.

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

// jsonKeys serializa v em JSON e devolve as chaves do objeto (ordenadas).
func jsonKeys(t *testing.T, v any) ([]string, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, m
}

func TestDashboardService_EmptyMetrics_MesmoFormato(t *testing.T) {
	for _, periodo := range []string{"today", "week", "month"} {
		for _, verbose := range []bool{false, true} {
			nome := periodo
			if verbose {
				nome += "/verbose"
			}
			t.Run(nome, func(t *testing.T) {
				db, mock := newDashboardTestDB(t)
				expectMetricsHappyPath(mock)
				svc := services.NewDashboardService(db, dashboardTestCfg(verbose))

				normal, err := svc.GetMetrics(context.Background(), db, periodo, 0)
				require.NoError(t, err)
				require.NoError(t, mock.ExpectationsWereMet())

				vazio := svc.EmptyMetrics(periodo)

				keysNormal, _ := jsonKeys(t, normal)
				keysVazio, m := jsonKeys(t, vazio)
				assert.Equal(t, keysNormal, keysVazio)
				assert.ElementsMatch(t, []string{
					"periodo", "total_vendas", "total_vendas_qtd", "total_pedidos",
					"ticket_medio", "top_vendedores", "metas_vendedores", "meta_mes",
				}, keysVazio)

				assert.Equal(t, periodo, m["periodo"])
				for _, k := range []string{"total_vendas", "total_vendas_qtd", "total_pedidos", "ticket_medio", "meta_mes"} {
					assert.Equal(t, 0.0, m[k], k)
				}
				// Listas vazias serializam como [] (nunca null).
				assert.Equal(t, []any{}, m["top_vendedores"])
				assert.Equal(t, []any{}, m["metas_vendedores"])
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
			svc := services.NewDashboardService(db, dashboardTestCfg(tt.verbose))

			normal, err := svc.GetVendasSeries(context.Background(), db, tt.dias, 0)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())

			vazio := svc.EmptyVendasSeries(tt.dias)

			keysNormal, mNormal := jsonKeys(t, normal)
			keysVazio, mVazio := jsonKeys(t, vazio)
			assert.Equal(t, keysNormal, keysVazio)
			assert.Equal(t, []string{"dias", "pontos"}, keysVazio)
			assert.Equal(t, float64(tt.dias), mVazio["dias"])

			pontoNormal := mNormal["pontos"].([]any)[0].(map[string]any)
			pontos := mVazio["pontos"].([]any)
			require.Len(t, pontos, tt.dias)
			for _, p := range pontos {
				pm := p.(map[string]any)
				kN, _ := jsonKeys(t, pontoNormal)
				kV, _ := jsonKeys(t, pm)
				assert.Equal(t, kN, kV, "pontos com as mesmas chaves")
				assert.Equal(t, 0.0, pm["total_vendas"])
				assert.Equal(t, 0.0, pm["total_pedidos"])
			}
			assert.Equal(t, hoje, pontos[len(pontos)-1].(map[string]any)["dia"])
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

		svc := services.NewDashboardService(db, dashboardTestCfg(true))
		m, err := svc.GetMetrics(context.Background(), db, "month", vid)
		require.NoError(t, err)
		assert.Equal(t, 50.0, m["ticket_medio"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetVendasSeries", func(t *testing.T) {
		db, mock := newDashboardTestDB(t)
		mock.ExpectQuery(`SELECT DATE_FORMAT\(data_pedido, '%Y-%m-%d'\) AS data.*AND vendedor_id = \?`).
			WithArgs(7, vid).
			WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))

		svc := services.NewDashboardService(db, dashboardTestCfg(false))
		s, err := svc.GetVendasSeries(context.Background(), db, 7, vid)
		require.NoError(t, err)
		assert.Len(t, s["pontos"], 7)
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

		svc := services.NewDashboardService(db, dashboardTestCfg(true))
		lista, total, err := svc.GetVendedoresRanking(context.Background(), db, 1, 20, vid)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, lista, 1)
		assert.Equal(t, vid, lista[0]["vendedor_id"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
