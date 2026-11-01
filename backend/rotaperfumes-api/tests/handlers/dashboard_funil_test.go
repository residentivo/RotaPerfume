package handlers_test

// DASH-01: GET /api/dashboard/funil?janela=30d|90d|365d|all. Prova o
// contrato JSON (snake_case, ordem dos campos), o 400 da janela ANTES de
// resolver o escopo, o escopo vindo só do Principal (?vendedor_id= é
// ignorado), o zerado sem SQL (desligado, sem vínculo, zero value), o 500
// genérico e os 4 níveis do vlog sem SQL.

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
)

const reFunil = `SELECT etapa, COUNT\(\*\), COALESCE\(SUM\(valor_estimado\), 0\)\s+FROM oportunidades\s+WHERE 1=1`

var funilColunas = []string{"etapa", "quantidade", "valor"}

// funilLinhas: 2 em prospecção, 1 ganha, 1 perdida e 1 etapa desconhecida.
func funilLinhas() *sqlmock.Rows {
	return sqlmock.NewRows(funilColunas).
		AddRow("Prospecção", 2, 200.0).
		AddRow("Fechado ganho", 1, 150.5).
		AddRow("Fechado perdido", 1, 80.0).
		AddRow("Arquivada", 3, 999.0)
}

// envelopeFunil decodifica o envelope guardando data cru (ordem dos campos).
type envelopeFunil struct {
	Success bool            `json:"success"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

func decodeFunil(t *testing.T, body string) (envelopeFunil, map[string]any) {
	t.Helper()
	var env envelopeFunil
	require.NoError(t, json.Unmarshal([]byte(body), &env), body)
	var data map[string]any
	if len(env.Data) > 0 && string(env.Data) != "null" {
		require.NoError(t, json.Unmarshal(env.Data, &data))
	}
	return env, data
}

func chaves(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestDASH01_Funil_ContratoJSON(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	mock.ExpectQuery(reFunil + `\s+AND data_abertura >= DATE_SUB\(CURDATE\(\), INTERVAL 29 DAY\)\s+GROUP BY etapa$`).
		WithoutArgs().WillReturnRows(funilLinhas())

	status, body := getComToken(t, server.URL+"/api/dashboard/funil?janela=30d", 1, "admin")
	require.Equal(t, http.StatusOK, status, body)
	env, data := decodeFunil(t, body)
	assert.True(t, env.Success)
	assert.Empty(t, env.Error)
	// Byte a byte: snake_case, campos na ordem do contrato, etapas na ordem do funil.
	assert.JSONEq(t, `{"janela":"30d","total_oportunidades":7,"etapas":[
		{"etapa":"Prospecção","quantidade":3,"valor_estimado":350.5,"conversao_anterior_pct":100,"conversao_topo_pct":100},
		{"etapa":"Qualificação","quantidade":1,"valor_estimado":150.5,"conversao_anterior_pct":33.33,"conversao_topo_pct":33.33},
		{"etapa":"Proposta enviada","quantidade":1,"valor_estimado":150.5,"conversao_anterior_pct":100,"conversao_topo_pct":33.33},
		{"etapa":"Negociação","quantidade":1,"valor_estimado":150.5,"conversao_anterior_pct":100,"conversao_topo_pct":33.33},
		{"etapa":"Fechado ganho","quantidade":1,"valor_estimado":150.5,"conversao_anterior_pct":100,"conversao_topo_pct":33.33}],
		"perdidas":{"quantidade":1,"valor_estimado":80},"taxa_ganho_pct":50,"vendedor_desligado":false}`, string(env.Data))
	assert.Equal(t, `{"janela":"30d","total_oportunidades":7,"etapas":[{"etapa":"Prospecção","quantidade":3,"valor_estimado":350.5,`+
		`"conversao_anterior_pct":100,"conversao_topo_pct":100},{"etapa":"Qualificação","quantidade":1,"valor_estimado":150.5,`+
		`"conversao_anterior_pct":33.33,"conversao_topo_pct":33.33},{"etapa":"Proposta enviada","quantidade":1,"valor_estimado":150.5,`+
		`"conversao_anterior_pct":100,"conversao_topo_pct":33.33},{"etapa":"Negociação","quantidade":1,"valor_estimado":150.5,`+
		`"conversao_anterior_pct":100,"conversao_topo_pct":33.33},{"etapa":"Fechado ganho","quantidade":1,"valor_estimado":150.5,`+
		`"conversao_anterior_pct":100,"conversao_topo_pct":33.33}],"perdidas":{"quantidade":1,"valor_estimado":80},`+
		`"taxa_ganho_pct":50,"vendedor_desligado":false}`, string(env.Data), "ordem dos campos do contrato")
	assert.ElementsMatch(t, []string{"janela", "total_oportunidades", "etapas", "perdidas", "taxa_ganho_pct", "vendedor_desligado"}, chaves(data))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDASH01_Funil_JanelasValidas(t *testing.T) {
	casos := []struct {
		query, janela, clausula string
	}{
		{"", "all", ``},
		{"?janela=", "all", ``},
		{"?janela=all", "all", ``},
		{"?janela=30d", "30d", `\s+AND data_abertura >= DATE_SUB\(CURDATE\(\), INTERVAL 29 DAY\)`},
		{"?janela=90d", "90d", `\s+AND data_abertura >= DATE_SUB\(CURDATE\(\), INTERVAL 89 DAY\)`},
		{"?janela=365d", "365d", `\s+AND data_abertura >= DATE_SUB\(CURDATE\(\), INTERVAL 364 DAY\)`},
	}
	for _, c := range casos {
		t.Run(c.janela+c.query, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			mock.ExpectQuery(reFunil + c.clausula + `\s+GROUP BY etapa$`).WithoutArgs().WillReturnRows(sqlmock.NewRows(funilColunas))
			status, body := getComToken(t, server.URL+"/api/dashboard/funil"+c.query, 1, "admin")
			require.Equal(t, http.StatusOK, status, body)
			_, data := decodeFunil(t, body)
			assert.Equal(t, c.janela, data["janela"])
			assert.Len(t, data["etapas"], 5, "sem linhas: as 5 etapas zeradas")
			assert.Equal(t, 0.0, data["total_oportunidades"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Janela inválida → 400 com a mensagem do contrato, SEM resolver o escopo
// (usuário normal: nenhuma consulta a usuarios/vendedores).
func TestDASH01_Funil_JanelaInvalida400SemEscopo(t *testing.T) {
	for _, janela := range []string{"7d", "ALL", "30D", "abc", "30", "365d%20", "30d%27%20OR%201=1"} {
		t.Run(janela, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			status, body := getComToken(t, server.URL+"/api/dashboard/funil?janela="+janela, escopoUsuarioNormal, "normal")
			assert.Equal(t, http.StatusBadRequest, status)
			env, _ := decodeFunil(t, body)
			assert.False(t, env.Success)
			assert.Equal(t, "janela deve ser '30d', '90d', '365d' ou 'all'", env.Error)
			assert.NoError(t, mock.ExpectationsWereMet(), "o 400 sai antes de qualquer SQL")
		})
	}
}

// O escopo vem só do Principal: ?vendedor_id= não muda o filtro.
func TestDASH01_Funil_VendedorFiltraPeloPrincipal(t *testing.T) {
	for _, query := range []string{"?janela=90d", "?janela=90d&vendedor_id=99", "?vendedor_id=1&janela=90d&vendedor_id=2"} {
		t.Run(query, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			expectEscopoVendedor(mock, escopoVendedorB)
			mock.ExpectQuery(reFunil + `\s+AND data_abertura >= DATE_SUB\(CURDATE\(\), INTERVAL 89 DAY\)\s+AND vendedor_id = \?\s+GROUP BY etapa$`).
				WithArgs(escopoVendedorB).WillReturnRows(funilLinhas())
			status, body := getComToken(t, server.URL+"/api/dashboard/funil"+query, escopoUsuarioNormal, "normal")
			require.Equal(t, http.StatusOK, status, body)
			_, data := decodeFunil(t, body)
			assert.Equal(t, "90d", data["janela"])
			assert.Equal(t, 7.0, data["total_oportunidades"])
			assert.Equal(t, false, data["vendedor_desligado"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// D-01: desligado e sem vínculo → 200 zerado (5 etapas) SEM a query do funil.
func TestDASH01_Funil_SemAcessoZeradoSemSQL(t *testing.T) {
	casos := []struct {
		nome      string
		preparo   func(sqlmock.Sqlmock)
		desligado bool
	}{
		{"vendedor desligado", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reUsuarioVendedor).WithArgs(escopoUsuarioNormal).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(30)))
			expectVendedorDesligado(m, 30, true)
		}, true},
		{"usuário sem vínculo", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reUsuarioVendedor).WithArgs(escopoUsuarioNormal).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
		}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			c.preparo(mock)
			status, body := getComToken(t, server.URL+"/api/dashboard/funil?janela=365d&vendedor_id=1", escopoUsuarioNormal, "normal")
			require.Equal(t, http.StatusOK, status, body)
			env, _ := decodeFunil(t, body)
			zerada := func(nome string) string {
				return `{"etapa":"` + nome + `","quantidade":0,"valor_estimado":0,"conversao_anterior_pct":0,"conversao_topo_pct":0}`
			}
			desl := "false"
			if c.desligado {
				desl = "true"
			}
			assert.Equal(t, `{"janela":"365d","total_oportunidades":0,"etapas":[`+zerada("Prospecção")+`,`+zerada("Qualificação")+`,`+
				zerada("Proposta enviada")+`,`+zerada("Negociação")+`,`+zerada("Fechado ganho")+`],`+
				`"perdidas":{"quantidade":0,"valor_estimado":0},"taxa_ganho_pct":0,"vendedor_desligado":`+desl+`}`, string(env.Data))
			assert.NoError(t, mock.ExpectationsWereMet(), "nenhuma query em oportunidades")
		})
	}
}

// Escopo zero value e SemAcesso direto na porta (sem SQL algum).
func TestDASH01_Funil_EscopoFixoSemSQL(t *testing.T) {
	for nome, e := range map[string]kernel.Escopo{"zero value": {}, "sem acesso": kernel.EscopoSemAcesso()} {
		t.Run(nome, func(t *testing.T) {
			h, mock := dashboardComEscopo(t, e, false)
			rec, body := getDashboard(h.GetFunil, "/api/dashboard/funil?janela=90d")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			data := body["data"].(map[string]any)
			assert.Equal(t, "90d", data["janela"])
			assert.Equal(t, false, data["vendedor_desligado"])
			assert.Len(t, data["etapas"], 5)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 500 genérico: nem texto de SQL nem do driver vazam na resposta.
func TestDASH01_Funil_Erro500Generico(t *testing.T) {
	errDriver := errors.New("Error 1146 (42S02): Table 'rota.oportunidades' doesn't exist near SELECT etapa")
	casos := []struct {
		nome    string
		uid     int64
		role    string
		preparo func(sqlmock.Sqlmock)
	}{
		{"erro na query do funil (admin)", 1, "admin", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reFunil).WillReturnError(errDriver)
		}},
		{"erro de scan (admin)", 1, "admin", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reFunil).WillReturnRows(sqlmock.NewRows(funilColunas).AddRow("Prospecção", "nao-numero", 1.0))
		}},
		{"erro ao resolver o escopo (normal)", escopoUsuarioNormal, "normal", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reUsuarioVendedor).WithArgs(escopoUsuarioNormal).WillReturnError(errDriver)
		}},
		{"erro na query do funil (vendedor)", escopoUsuarioNormal, "normal", func(m sqlmock.Sqlmock) {
			expectEscopoVendedor(m, escopoVendedorB)
			m.ExpectQuery(reFunil).WithArgs(escopoVendedorB).WillReturnError(errDriver)
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			c.preparo(mock)
			status, body := getComToken(t, server.URL+"/api/dashboard/funil", c.uid, c.role)
			assert.Equal(t, http.StatusInternalServerError, status)
			env, _ := decodeFunil(t, body)
			assert.False(t, env.Success)
			assert.Equal(t, "erro interno", env.Error)
			for _, vazado := range []string{"SELECT", "oportunidades", "1146", "Table", "scan", "analyticsmysql"} {
				assert.NotContains(t, body, vazado)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDASH01_Funil_SemTokenNaoAutenticado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	resp, err := http.Get(server.URL + "/api/dashboard/funil")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Os 4 níveis do vlog (rota, caso de uso, decisão, query) sem SQL nem
// valores de negócio.
func TestDASH01_Funil_VlogNiveis(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	out := capturarVlogLigado(t)

	expectEscopoVendedor(mock, escopoVendedorB)
	mock.ExpectQuery(reFunil).WithArgs(escopoVendedorB).WillReturnRows(funilLinhas())
	status, _ := getComToken(t, server.URL+"/api/dashboard/funil?janela=30d", escopoUsuarioNormal, "normal")
	require.Equal(t, http.StatusOK, status)

	mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(30)))
	expectVendedorDesligado(mock, 30, true)
	status, _ = getComToken(t, server.URL+"/api/dashboard/funil", 9, "normal")
	require.Equal(t, http.StatusOK, status)
	require.NoError(t, mock.ExpectationsWereMet())

	saida := out.String()
	for _, nivel := range []string{
		"[dashboard_handler.go] [DashboardHandler.GetFunil] início GET",                    // 1. rota
		"[consultas.go] [Analytics.Funil] início janela=30d",                               // 2. caso de uso
		"[consultas.go] [Analytics.Funil] decisão escopo restrito=true vendedor_id=20",     // 3. decisão
		"[oportunidades.go] [Oportunidades.Funil] SELECT oportunidades por etapa linhas=4", // 4. query
		"[consultas.go] [Analytics.Funil] decisão sem_acesso motivo=vendedor_desligado",    // D-01
		"[consultas.go] [Analytics.Funil] início janela=all",
	} {
		assert.Contains(t, saida, nivel)
	}
	for _, valor := range []string{"Prospecção", "Arquivada", "SELECT etapa", "COUNT(", "data_abertura", "INTERVAL"} {
		assert.NotContains(t, saida, valor, "o log não leva valores nem texto de SQL")
	}
}
