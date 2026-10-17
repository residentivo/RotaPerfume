package handlers_test

// ARQ-06 (Analytics, CQRS leve): o Dashboard saiu do handler/service/
// repository legados (e da ponte handlers/scope.go) para analyticshttp →
// analyticsapp → analyticsmysql. Estes testes provam:
//   - o contrato JSON byte a byte contra o map[string]any + structs legadas
//     congeladas, serializados pelo envelope legado (map + Encoder);
//   - D-01/O1: SemAcesso, escopo zero value e desligado respondem zerado
//     SEM tocar no banco (nenhuma expectativa no sqlmock);
//   - P1/O7: os níveis do vlog sem SQL nem nomes.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/analyticshttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/kernel"
)

// Structs legadas congeladas (repositories.VendedorRanking, MetaVendedor,
// SegmentoContagem, UFContagem), na ordem de campos original.
type (
	legadoVendedorRanking struct {
		ID               int64   `json:"id"`
		Nome             string  `json:"nome"`
		TotalVendas      float64 `json:"total_vendas"`
		Meta             float64 `json:"meta"`
		PercentualMeta   float64 `json:"percentual_meta"`
		QuantidadeVendas int     `json:"quantidade_vendas"`
	}
	legadoMetaVendedor struct {
		ID               int64   `json:"id"`
		Nome             string  `json:"nome"`
		Regiao           string  `json:"regiao"`
		UF               string  `json:"uf"`
		Meta             float64 `json:"meta"`
		Realizado        float64 `json:"realizado"`
		Percentual       float64 `json:"percentual"`
		QuantidadeVendas int     `json:"quantidade_vendas"`
	}
	legadoSegmento struct {
		Segmento string `json:"segmento"`
		Total    int    `json:"total"`
	}
	legadoUF struct {
		UF    string `json:"uf"`
		Total int    `json:"total"`
	}
)

// legadoMetricas é o metricsResponse do dashboard_service.go legado.
func legadoMetricas(periodo string, total float64, qtd, pedidos int, ticket float64, top []legadoVendedorRanking, metas []legadoMetaVendedor, metaMes float64, desligado bool) map[string]any {
	return map[string]any{
		"periodo": periodo, "total_vendas": total, "total_vendas_qtd": qtd, "total_pedidos": pedidos,
		"ticket_medio": ticket, "top_vendedores": top, "metas_vendedores": metas, "meta_mes": metaMes,
		"vendedor_desligado": desligado,
	}
}

// legadoClientes é o clienteMetricsResponse legado.
func legadoClientes(periodo string, total, ativos, inativos, novos int, seg []legadoSegmento, uf []legadoUF) map[string]any {
	return map[string]any{
		"periodo": periodo, "total_clientes": total, "total_ativos": ativos, "total_inativos": inativos,
		"novos_no_periodo": novos, "por_segmento": seg, "por_uf": uf,
	}
}

func legadoPonto(dia string, total float64, pedidos int) map[string]any {
	return map[string]any{"dia": dia, "total_vendas": total, "total_pedidos": pedidos}
}

func legadoPaginado(data any, page, limit, total, pages int) map[string]any {
	return map[string]any{"success": true, "data": data,
		"pagination": map[string]any{"page": page, "limit": limit, "total": total, "pages": pages}}
}

func TestARQ06_ContratoJSON_ByteAByte(t *testing.T) {
	percentual := (250.25 / 1000.0) * 100

	t.Run("GET /api/dashboard/metrics?periodo=week (admin)", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(reVendasTotais).WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(1234.5, 3))
		mock.ExpectQuery(reTotalPedidos).WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(3))
		mock.ExpectQuery(reTopVendedores).WithArgs(10).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}).AddRow(int64(7), "Ana", 1000.0))
		mock.ExpectQuery(reEnrichVendas).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}).AddRow(int64(7), 250.25, 2))
		mock.ExpectQuery(reMetasVendedores).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}).AddRow(int64(7), "Ana", "Sul", "PR", 1000.0))
		mock.ExpectQuery(reEnrichVendas).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}).AddRow(int64(7), 250.25, 2))
		mock.ExpectQuery(reMetaMensalTotal).WillReturnRows(sqlmock.NewRows([]string{"m"}).AddRow(1000.0))

		status, body := getComToken(t, server.URL+"/api/dashboard/metrics?periodo=week", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": legadoMetricas("week", 1234.5, 3, 3, 1234.5/3,
			[]legadoVendedorRanking{{ID: 7, Nome: "Ana", TotalVendas: 250.25, Meta: 1000, PercentualMeta: percentual, QuantidadeVendas: 2}},
			[]legadoMetaVendedor{{ID: 7, Nome: "Ana", Regiao: "Sul", UF: "PR", Meta: 1000, Realizado: 250.25, Percentual: percentual, QuantidadeVendas: 2}},
			1000, false)}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/dashboard/vendas?dias=2 (admin, dia sem venda zerado)", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(reSerieVendas).WithArgs(2).WillReturnRows(sqlmock.NewRows([]string{"d", "v", "q"}).AddRow("2026-10-14", 10.5, 1))
		mock.ExpectQuery(reSerieDias).WithArgs(2).WillReturnRows(sqlmock.NewRows([]string{"dia"}).AddRow("2026-10-14").AddRow("2026-10-15"))

		status, body := getComToken(t, server.URL+"/api/dashboard/vendas?dias=2", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": map[string]any{"dias": 2,
			"pontos": []map[string]any{legadoPonto("2026-10-14", 10.5, 1), legadoPonto("2026-10-15", 0.0, 0)}}}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/dashboard/vendedores?page=1&limit=2 (admin)", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(reRankingCount).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(3))
		mock.ExpectQuery(reRankingLista).WithArgs(2, 0).WillReturnRows(sqlmock.NewRows(rankingCols).
			AddRow(int64(7), "Ana", "Sul", "PR", 1000.0, 333.333, 3, 33.3333))

		status, body := getComToken(t, server.URL+"/api/dashboard/vendedores?page=1&limit=2", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, legadoPaginado([]map[string]any{{
			"vendedor_id": int64(7), "vendedor_nome": "Ana", "regiao": "Sul", "uf": "PR", "total_vendas": 333.333,
			"total_pedidos": 3, "ticket_medio": 111.11, "meta": 1000.0, "atingimento_meta": 33.33,
		}}, 1, 2, 3, 2)), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/dashboard/vendedores sem linhas: data [] (nunca null)", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(reRankingCount).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
		mock.ExpectQuery(reRankingLista).WithArgs(20, 0).WillReturnRows(sqlmock.NewRows(rankingCols))

		status, body := getComToken(t, server.URL+"/api/dashboard/vendedores", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, legadoPaginado([]map[string]any{}, 1, 20, 0, 0)), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/dashboard/clientes?periodo=today (admin)", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes$`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(9))
		mock.ExpectQuery(`FROM clientes WHERE ativo = \?$`).WithArgs(true).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(7))
		mock.ExpectQuery(`FROM clientes WHERE ativo = \?$`).WithArgs(false).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(2))
		mock.ExpectQuery(`FROM clientes WHERE data_cadastro = CURDATE\(\)$`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
		mock.ExpectQuery(`SELECT segmento`).WillReturnRows(sqlmock.NewRows([]string{"s", "t"}).AddRow("varejo", 6).AddRow("atacado", 3))
		mock.ExpectQuery(`SELECT uf`).WillReturnRows(sqlmock.NewRows([]string{"u", "t"}).AddRow("PR", 9))

		status, body := getComToken(t, server.URL+"/api/dashboard/clientes?periodo=today", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": legadoClientes("today", 9, 7, 2, 1,
			[]legadoSegmento{{"varejo", 6}, {"atacado", 3}}, []legadoUF{{"PR", 9}})}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("sem acesso: zerado com listas [] (EmptyMetrics/EmptyClienteMetrics legados)", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
		status, body := getComToken(t, server.URL+"/api/dashboard/metrics", 5, "normal")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": legadoMetricas("month", 0, 0, 0, 0,
			[]legadoVendedorRanking{}, []legadoMetaVendedor{}, 0, false)}), body)

		mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
		status, body = getComToken(t, server.URL+"/api/dashboard/clientes?periodo=week", 5, "normal")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": legadoClientes("week", 0, 0, 0, 0,
			[]legadoSegmento{}, []legadoUF{})}), body)

		mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
		status, body = getComToken(t, server.URL+"/api/dashboard/vendedores?page=3&limit=5", 5, "normal")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, legadoPaginado([]map[string]any{}, 3, 5, 0, 0)), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("400 de periodo e 500 de banco: envelope de erro legado", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		status, body := getComToken(t, server.URL+"/api/dashboard/clientes?periodo=ano", 1, "admin")
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": false, "error": "periodo deve ser 'today', 'week' ou 'month'"}), body)

		for _, path := range dashboardPaths {
			switch path {
			case "/api/dashboard/metrics":
				mock.ExpectQuery(reVendasTotais).WillReturnError(sqlmock.ErrCancelled)
			case "/api/dashboard/vendas":
				mock.ExpectQuery(reSerieVendas).WillReturnError(sqlmock.ErrCancelled)
			case "/api/dashboard/vendedores":
				mock.ExpectQuery(reRankingCount).WillReturnError(sqlmock.ErrCancelled)
			default:
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).WillReturnError(sqlmock.ErrCancelled)
			}
			status, body = getComToken(t, server.URL+path, 1, "admin")
			assert.Equal(t, http.StatusInternalServerError, status, path)
			assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": false, "error": "erro interno"}), body, path)
		}
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// escopoFixoAnalytics é a porta EscopoAnalytics com um escopo fixo, SEM SQL
// (para provar o que o caso de uso faz com cada escopo).
type escopoFixoAnalytics struct {
	e         kernel.Escopo
	desligado bool
}

func (f escopoFixoAnalytics) ResolverParaAnalytics(context.Context, kernel.Principal) (kernel.Escopo, bool, error) {
	return f.e, f.desligado, nil
}

func dashboardComEscopo(t *testing.T, e kernel.Escopo, desligado bool) (*analyticshttp.DashboardHandler, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return bootstrap.NovoAnalyticsHTTP(bootstrap.NovoAnalytics(db, escopoFixoAnalytics{e: e, desligado: desligado})), mock
}

func getDashboard(h http.HandlerFunc, path string) (*httptest.ResponseRecorder, map[string]any) {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

// TestARQ04_N1_PonteEscopo_ZeroValueEhSemAcesso (nome do ARQ-04 mantido,
// R1): a ponte handlers/scope.go saiu no ARQ-06, e o teste interno dela
// virou este teste externo sobre o código novo. A garantia é a mesma: o
// escopo zero value nunca é irrestrito (resposta zerada, sem SQL); só o
// Irrestrito sai sem filtro; o de vendedor filtra pelo id dele.
func TestARQ04_N1_PonteEscopo_ZeroValueEhSemAcesso(t *testing.T) {
	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	irrestrito, _ := kernel.EscopoIrrestrito(admin)
	doVendedor, _ := kernel.EscopoDoVendedor(10)
	casos := []struct {
		nome   string
		e      kernel.Escopo
		expect func(sqlmock.Sqlmock)
		total  float64
	}{
		{"irrestrito", irrestrito, func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reRankingCount + `$`).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(2))
			m.ExpectQuery(reRankingLista).WithArgs(20, 0).WillReturnRows(sqlmock.NewRows(rankingCols))
		}, 2},
		{"vendedor", doVendedor, func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reRankingCount + ` AND id = \?$`).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
			m.ExpectQuery(reRankingLista).WithArgs(int64(10), 20, 0).WillReturnRows(sqlmock.NewRows(rankingCols))
		}, 1},
		{"sem acesso", kernel.EscopoSemAcesso(), func(sqlmock.Sqlmock) {}, 0},
		{"zero value do Escopo", kernel.Escopo{}, func(sqlmock.Sqlmock) {}, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			h, mock := dashboardComEscopo(t, c.e, false)
			c.expect(mock)
			rec, body := getDashboard(h.GetVendedores, "/api/dashboard/vendedores")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, c.total, body["pagination"].(map[string]any)["total"])
			assert.NoError(t, mock.ExpectationsWereMet(), "zero value e SemAcesso não consultam o banco")
		})
	}
}

// TestARQ06_O1_SemAcessoEZeroValueSemSQL: nos 4 endpoints, SemAcesso, o
// zero value do Escopo e o desligado respondem 200 zerado SEM nenhuma
// expectativa no sqlmock (D-01, O1); só /metrics leva o aviso de
// desligamento. O zero value ainda gera o log operacional [escopo] invalido.
func TestARQ06_O1_SemAcessoEZeroValueSemSQL(t *testing.T) {
	escopos := []struct {
		nome      string
		e         kernel.Escopo
		desligado bool
	}{
		{"sem acesso", kernel.EscopoSemAcesso(), false},
		{"zero value", kernel.Escopo{}, false},
		{"desligado", kernel.EscopoSemAcesso(), true},
	}
	for _, esc := range escopos {
		t.Run(esc.nome, func(t *testing.T) {
			saida := capturarLog(t)
			h, mock := dashboardComEscopo(t, esc.e, esc.desligado)

			_, body := getDashboard(h.GetMetrics, "/api/dashboard/metrics?periodo=today")
			data := body["data"].(map[string]any)
			assert.Equal(t, "today", data["periodo"])
			assert.Equal(t, esc.desligado, data["vendedor_desligado"])
			assert.Equal(t, 0.0, data["total_vendas"])
			assert.Equal(t, []any{}, data["top_vendedores"])

			_, body = getDashboard(h.GetVendas, "/api/dashboard/vendas?dias=3")
			assert.Len(t, body["data"].(map[string]any)["pontos"], 3)

			_, body = getDashboard(h.GetVendedores, "/api/dashboard/vendedores")
			assert.Equal(t, []any{}, body["data"])

			_, body = getDashboard(h.GetClientes, "/api/dashboard/clientes")
			assert.Equal(t, []any{}, body["data"].(map[string]any)["por_uf"])

			assert.NoError(t, mock.ExpectationsWereMet())
			if esc.nome == "zero value" {
				assert.Contains(t, saida.String(), "[escopo] invalido [dashboard] Metricas")
			} else {
				assert.NotContains(t, saida.String(), "[escopo] invalido")
			}
		})
	}
}

func TestARQ06_Vlog_NiveisSemValores(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	out := capturarVlogLigado(t)

	expectEscopoVendedor(mock, escopoVendedorB)
	mock.ExpectQuery(reRankingCount).WithArgs(escopoVendedorB).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectQuery(reRankingLista).WithArgs(escopoVendedorB, 20, 0).WillReturnRows(sqlmock.NewRows(rankingCols).
		AddRow(escopoVendedorB, "NomeSecreto", "Sul", "PR", 100.0, 50.0, 1, 50.0))
	status, _ := getComToken2(t, server.URL+"/api/dashboard/vendedores", generateToken(t, testCfg(), escopoUsuarioNormal, "normal"))
	require.Equal(t, http.StatusOK, status)

	mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(30)))
	expectVendedorDesligado(mock, 30, true)
	status, _ = getComToken2(t, server.URL+"/api/dashboard/metrics", generateToken(t, testCfg(), 9, "normal"))
	require.Equal(t, http.StatusOK, status)
	require.NoError(t, mock.ExpectationsWereMet())

	saida := out.String()
	for _, nivel := range []string{
		"[dashboard_handler.go] [DashboardHandler.GetVendedores] início GET",               // 1. rota
		"[consultas.go] [Analytics.Ranking] início page=1 limit=20",                        // 2. caso de uso
		"[consultas.go] [Analytics.Ranking] decisão escopo restrito=true vendedor_id=20",   // 3. decisão
		"[dashboard.go] [Dashboard.Ranking] SELECT vendedores ranking linhas=1 total=1",    // 4. query
		"[consultas.go] [Analytics.Metricas] decisão sem_acesso motivo=vendedor_desligado", // D-01
		"[consultas.go] [Analytics.Metricas] início periodo=month",
	} {
		assert.Contains(t, saida, nivel)
	}
	for _, valor := range []string{"NomeSecreto", "SELECT COUNT", "LEFT JOIN", "data_desligamento IS NULL"} {
		assert.NotContains(t, saida, valor, "o log não leva nomes nem texto de SQL")
	}
	assert.False(t, strings.Contains(saida, "[escopo] invalido"))
}
