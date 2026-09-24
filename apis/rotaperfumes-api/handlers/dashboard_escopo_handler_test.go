package handlers_test

// Testes do escopo por vendedor do Dashboard (/metrics, /vendas,
// /vendedores). Regra: o pedido pertence a pedidos.vendedor_id; usuário
// normal vê só os próprios números; admin vê tudo; normal sem vendedor
// vinculado recebe 200 com payload zerado/vazio sem consultar pedidos/
// vendedores.

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/middleware"
)

const (
	escopoUsuarioNormal = int64(5)
	escopoVendedorB     = int64(20)
)

const (
	reUsuarioVendedor = `SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`
	reVendasTotais    = `SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`
	reTotalPedidos    = `SELECT COUNT\(\*\) FROM pedidos\s+WHERE`
	reTopVendedores   = `SELECT\s+v\.id,\s+v\.nome,\s+v\.meta_mensal AS meta\s+FROM vendedores v`
	reMetasVendedores = `SELECT\s+v\.id,\s+v\.nome,\s+v\.regiao,\s+v\.uf,\s+v\.meta_mensal AS meta\s+FROM vendedores v`
	reMetaMensalTotal = `SELECT COALESCE\(SUM\(meta_mensal\), 0\)\s+FROM vendedores\s+WHERE data_desligamento IS NULL`
	reEnrichVendas    = `SELECT vendedor_id,\s+COALESCE\(SUM\(valor_total\), 0\),\s+COUNT\(\*\)\s+FROM pedidos\s+WHERE vendedor_id IN \(\?\)`
	reSerieVendas     = `SELECT DATE_FORMAT\(data_pedido, '%Y-%m-%d'\) AS data`
	reSerieDias       = `SELECT DATE_FORMAT\(DATE_SUB\(CURDATE\(\), INTERVAL n DAY\), '%Y-%m-%d'\) AS dia`
	reRankingCount    = `SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL`
	reRankingLista    = `(?s)SELECT.*FROM vendedores v.*LEFT JOIN.*LIMIT \? OFFSET \?`
)

var rankingCols = []string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}

func doDashboardGet(t *testing.T, server *httptest.Server, path, token string) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("GET", server.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp, decodeResponse(t, readBody(t, resp))
}

func expectEscopoVendedor(mock sqlmock.Sqlmock, vendedorID int64) {
	mock.ExpectQuery(reUsuarioVendedor).
		WithArgs(escopoUsuarioNormal).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(vendedorID))
}

// ---------------------------------------------------------------------------
// Normal com vendedor B
// ---------------------------------------------------------------------------

func TestDashboardEscopo_Metrics_NormalVeSoVendedorB(t *testing.T) {
	for _, periodo := range []string{"today", "week", "month"} {
		t.Run(periodo, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			token := generateToken(t, testCfg(), escopoUsuarioNormal, "normal")

			expectEscopoVendedor(mock, escopoVendedorB)
			mock.ExpectQuery(reVendasTotais + `.*AND vendedor_id = \?`).
				WithArgs(escopoVendedorB).
				WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(50.0, 1))
			mock.ExpectQuery(reTotalPedidos + `.*AND vendedor_id = \?`).
				WithArgs(escopoVendedorB).
				WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(1))
			mock.ExpectQuery(reTopVendedores + `\s+WHERE v\.data_desligamento IS NULL AND v\.id = \?`).
				WithArgs(escopoVendedorB, 10).
				WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}).AddRow(escopoVendedorB, "Vendedor B", 100.0))
			mock.ExpectQuery(reEnrichVendas).
				WithArgs(escopoVendedorB).
				WillReturnRows(sqlmock.NewRows([]string{"vid", "t", "q"}).AddRow(escopoVendedorB, 50.0, 1))
			mock.ExpectQuery(reMetasVendedores + `\s+WHERE v\.data_desligamento IS NULL AND v\.id = \?`).
				WithArgs(escopoVendedorB).
				WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}).AddRow(escopoVendedorB, "Vendedor B", "Sul", "PR", 100.0))
			mock.ExpectQuery(reEnrichVendas).
				WithArgs(escopoVendedorB).
				WillReturnRows(sqlmock.NewRows([]string{"vid", "t", "q"}).AddRow(escopoVendedorB, 50.0, 1))
			mock.ExpectQuery(reMetaMensalTotal + ` AND id = \?`).
				WithArgs(escopoVendedorB).
				WillReturnRows(sqlmock.NewRows([]string{"m"}).AddRow(100.0))

			resp, body := doDashboardGet(t, server, "/api/dashboard/metrics?periodo="+periodo, token)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			data := body["data"].(map[string]any)
			assert.Equal(t, periodo, data["periodo"])
			assert.Equal(t, 50.0, data["total_vendas"])
			assert.Equal(t, 1.0, data["total_vendas_qtd"])
			assert.Equal(t, 1.0, data["total_pedidos"])
			assert.Equal(t, 50.0, data["ticket_medio"])
			assert.Equal(t, 100.0, data["meta_mes"])

			for _, chave := range []string{"top_vendedores", "metas_vendedores"} {
				lista := data[chave].([]any)
				require.LessOrEqual(t, len(lista), 1, chave)
				for _, item := range lista {
					assert.Equal(t, float64(escopoVendedorB), item.(map[string]any)["id"], chave)
				}
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardEscopo_Vendas_NormalFiltraVendedorB(t *testing.T) {
	casos := []struct {
		nome string
		qs   string
		dias int
	}{
		{"default 30 dias", "", 30},
		{"7 dias", "?dias=7", 7},
	}
	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			token := generateToken(t, testCfg(), escopoUsuarioNormal, "normal")

			hoje := time.Now().Format("2006-01-02")
			expectEscopoVendedor(mock, escopoVendedorB)
			mock.ExpectQuery(reSerieVendas + `.*AND vendedor_id = \?\s+GROUP BY`).
				WithArgs(tt.dias, escopoVendedorB).
				WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}).AddRow(hoje, 50.0, 1))
			mock.ExpectQuery(reSerieDias).
				WithArgs(tt.dias).
				WillReturnRows(sqlmock.NewRows([]string{"dia"}).AddRow(hoje))

			resp, body := doDashboardGet(t, server, "/api/dashboard/vendas"+tt.qs, token)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			data := body["data"].(map[string]any)
			assert.Equal(t, float64(tt.dias), data["dias"])
			pontos := data["pontos"].([]any)
			require.Len(t, pontos, 1)
			p := pontos[0].(map[string]any)
			assert.Equal(t, hoje, p["dia"])
			assert.Equal(t, 50.0, p["total_vendas"])
			assert.Equal(t, 1.0, p["total_pedidos"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardEscopo_Vendedores_NormalSoLinhaB(t *testing.T) {
	casos := []struct {
		nome      string
		total     int
		comLinha  bool
		wantPages float64
	}{
		{"vendedor B ativo (total 1)", 1, true, 1},
		{"vendedor B desligado (total 0)", 0, false, 0},
	}
	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			token := generateToken(t, testCfg(), escopoUsuarioNormal, "normal")

			expectEscopoVendedor(mock, escopoVendedorB)
			mock.ExpectQuery(reRankingCount + ` AND id = \?`).
				WithArgs(escopoVendedorB).
				WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(tt.total))
			rows := sqlmock.NewRows(rankingCols)
			if tt.comLinha {
				rows.AddRow(escopoVendedorB, "Vendedor B", "Sul", "PR", 100.0, 50.0, 1, 50.0)
			}
			mock.ExpectQuery(`(?s)WHERE v\.data_desligamento IS NULL AND v\.id = \?.*LIMIT \? OFFSET \?`).
				WithArgs(escopoVendedorB, 20, 0).
				WillReturnRows(rows)

			resp, body := doDashboardGet(t, server, "/api/dashboard/vendedores", token)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			pag := body["pagination"].(map[string]any)
			assert.Equal(t, float64(tt.total), pag["total"])
			assert.Equal(t, tt.wantPages, pag["pages"])
			lista := body["data"].([]any)
			if tt.comLinha {
				require.Len(t, lista, 1)
				linha := lista[0].(map[string]any)
				assert.Equal(t, float64(escopoVendedorB), linha["vendedor_id"])
				assert.Equal(t, 50.0, linha["ticket_medio"])
			} else {
				assert.Empty(t, lista)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Normal sem vendedor vinculado (id_vendedor NULL / usuário inexistente)
// ---------------------------------------------------------------------------

func TestDashboardEscopo_SemVendedor_RespostaZerada(t *testing.T) {
	semVendedor := []struct {
		nome   string
		expect func(sqlmock.Sqlmock)
	}{
		{"id_vendedor NULL", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reUsuarioVendedor).WithArgs(escopoUsuarioNormal).
				WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
		}},
		{"usuário inexistente", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reUsuarioVendedor).WithArgs(escopoUsuarioNormal).
				WillReturnError(sql.ErrNoRows)
		}},
	}
	endpoints := []struct {
		nome   string
		path   string
		verify func(t *testing.T, body map[string]any)
	}{
		{"metrics", "/api/dashboard/metrics?periodo=week", func(t *testing.T, body map[string]any) {
			data := body["data"].(map[string]any)
			assert.Len(t, data, 8)
			assert.Equal(t, "week", data["periodo"])
			for _, k := range []string{"total_vendas", "total_vendas_qtd", "total_pedidos", "ticket_medio", "meta_mes"} {
				assert.Equal(t, 0.0, data[k], k)
			}
			assert.Equal(t, []any{}, data["top_vendedores"])
			assert.Equal(t, []any{}, data["metas_vendedores"])
		}},
		{"vendas default", "/api/dashboard/vendas", func(t *testing.T, body map[string]any) {
			data := body["data"].(map[string]any)
			assert.Equal(t, 30.0, data["dias"])
			pontos := data["pontos"].([]any)
			require.Len(t, pontos, 30)
			for _, p := range pontos {
				pm := p.(map[string]any)
				assert.Equal(t, 0.0, pm["total_vendas"])
				assert.Equal(t, 0.0, pm["total_pedidos"])
				assert.NotEmpty(t, pm["dia"])
			}
		}},
		{"vendas 7 dias", "/api/dashboard/vendas?dias=7", func(t *testing.T, body map[string]any) {
			data := body["data"].(map[string]any)
			assert.Equal(t, 7.0, data["dias"])
			assert.Len(t, data["pontos"].([]any), 7)
		}},
		{"vendedores", "/api/dashboard/vendedores?page=2&limit=10", func(t *testing.T, body map[string]any) {
			assert.Equal(t, []any{}, body["data"])
			pag := body["pagination"].(map[string]any)
			assert.Equal(t, 2.0, pag["page"])
			assert.Equal(t, 10.0, pag["limit"])
			assert.Equal(t, 0.0, pag["total"])
			assert.Equal(t, 0.0, pag["pages"])
		}},
	}

	for _, sv := range semVendedor {
		for _, ep := range endpoints {
			t.Run(sv.nome+"/"+ep.nome, func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()
				token := generateToken(t, testCfg(), escopoUsuarioNormal, "normal")

				// Só a query de escopo é esperada: qualquer query a pedidos/
				// vendedores falharia no sqlmock (resposta 500).
				sv.expect(mock)

				resp, body := doDashboardGet(t, server, ep.path, token)
				require.Equal(t, http.StatusOK, resp.StatusCode)
				assert.True(t, body["success"].(bool))
				ep.verify(t, body)
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Erro ao resolver escopo -> 500
// ---------------------------------------------------------------------------

func TestDashboardEscopo_ErroResolverEscopo_500(t *testing.T) {
	for _, path := range []string{"/api/dashboard/metrics", "/api/dashboard/vendas", "/api/dashboard/vendedores"} {
		t.Run(path, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			token := generateToken(t, testCfg(), escopoUsuarioNormal, "normal")

			mock.ExpectQuery(reUsuarioVendedor).WithArgs(escopoUsuarioNormal).
				WillReturnError(sql.ErrConnDone)

			resp, body := doDashboardGet(t, server, path, token)
			assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
			assert.Equal(t, "erro interno", body["error"])
			assert.Nil(t, body["data"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Admin -> sem filtro (regressão)
// ---------------------------------------------------------------------------

func TestDashboardEscopo_Admin_SemFiltro(t *testing.T) {
	casos := []struct {
		nome   string
		path   string
		expect func(sqlmock.Sqlmock)
	}{
		{"metrics", "/api/dashboard/metrics", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reVendasTotais).WithoutArgs().
				WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(300.0, 3))
			m.ExpectQuery(reTotalPedidos).WithoutArgs().
				WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(3))
			m.ExpectQuery(reTopVendedores + `\s+WHERE v\.data_desligamento IS NULL\s+ORDER BY`).WithArgs(10).
				WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}))
			m.ExpectQuery(reMetasVendedores + `\s+WHERE v\.data_desligamento IS NULL\s+ORDER BY`).WithoutArgs().
				WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}))
			m.ExpectQuery(reMetaMensalTotal + `$`).WithoutArgs().
				WillReturnRows(sqlmock.NewRows([]string{"m"}).AddRow(900.0))
		}},
		{"vendas", "/api/dashboard/vendas?dias=5", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reSerieVendas + `.*AND status NOT IN \('cancelado', 'devolvido'\)\s+GROUP BY`).WithArgs(5).
				WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))
		}},
		{"vendedores", "/api/dashboard/vendedores", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reRankingCount + `$`).WithoutArgs().
				WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(2))
			m.ExpectQuery(`(?s)WHERE v\.data_desligamento IS NULL\s+ORDER BY.*LIMIT \? OFFSET \?`).WithArgs(20, 0).
				WillReturnRows(sqlmock.NewRows(rankingCols).
					AddRow(int64(1), "A", "Sul", "PR", 100.0, 10.0, 1, 10.0).
					AddRow(int64(2), "B", "Sul", "SC", 50.0, 0.0, 0, 0.0))
		}},
	}
	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			token := generateToken(t, testCfg(), 1, "admin")

			// Admin não consulta usuarios.id_vendedor.
			tt.expect(mock)

			resp, _ := doDashboardGet(t, server, tt.path, token)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// /metrics com periodo inválido -> 400 antes de resolver o escopo
// ---------------------------------------------------------------------------

func TestDashboardEscopo_Metrics_PeriodoInvalido_NaoResolveEscopo(t *testing.T) {
	for _, periodo := range []string{"ano", "MONTH", "x"} {
		t.Run(periodo, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			token := generateToken(t, testCfg(), escopoUsuarioNormal, "normal")

			// Armadilha: se o escopo fosse resolvido, esta expectativa seria
			// consumida. Ela deve permanecer pendente.
			expectEscopoVendedor(mock, escopoVendedorB)

			resp, body := doDashboardGet(t, server, "/api/dashboard/metrics?periodo="+periodo, token)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Equal(t, "periodo deve ser 'today', 'week' ou 'month'", body["error"])
			assert.Error(t, mock.ExpectationsWereMet(), "query de escopo não deveria ter sido executada")
		})
	}
}

// ---------------------------------------------------------------------------
// Verbose: log de escopo em resolverEscopo
// ---------------------------------------------------------------------------

func TestDashboardEscopo_Verbose_SemVendedor(t *testing.T) {
	cfg := testCfg()
	cfg.Verbose = true
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	h := handlers.NewDashboardHandler(db, cfg)
	chains := []struct {
		nome string
		h    http.HandlerFunc
		path string
	}{
		{"metrics", h.GetMetrics, "/api/dashboard/metrics"},
		{"vendas", h.GetVendas, "/api/dashboard/vendas"},
		{"vendedores", h.GetVendedores, "/api/dashboard/vendedores"},
	}
	token := generateToken(t, cfg, escopoUsuarioNormal, "normal")
	for _, c := range chains {
		t.Run(c.nome, func(t *testing.T) {
			mock.ExpectQuery(reUsuarioVendedor).WithArgs(escopoUsuarioNormal).
				WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))

			handler := middleware.JWTMiddleware(cfg, true, false)(c.h)
			req := httptest.NewRequest("GET", c.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
