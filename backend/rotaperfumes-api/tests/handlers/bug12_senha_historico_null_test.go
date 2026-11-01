package handlers_test

// BUG-12: GET /api/senha-historico e /api/senha-historico/{usuario_id} não
// podem responder 500 quando ip_origem, user_agent, created_at ou
// usuario_nome (LEFT JOIN) vêm NULL. O contrato JSON se mantém: strings
// vazias e created_at "0001-01-01T00:00:00Z".

import (
	"database/sql/driver"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bug12Colunas = []string{
	"id", "usuario_id", "resetado_por_id", "senha_hash_anterior", "ip_origem", "user_agent", "tipo_reset", "created_at",
	"usuario_nome", "resetado_por_nome",
}

func TestBUG12_SenhaHistorico_Handler_LinhaNull(t *testing.T) {
	criado := time.Date(2026, 9, 20, 14, 30, 5, 0, time.UTC)
	linhaNull := []driver.Value{int64(1), int64(5), nil, "hash", nil, nil, "usuario", nil, nil, nil}
	linhaCheia := []driver.Value{int64(2), int64(5), int64(9), "hash", "10.0.0.2", "Mozilla/5.0", "admin", criado, "Fulano", "Admin"}

	rotas := []struct {
		nome   string
		path   string
		count  string
		lista  string
		args   []driver.Value
		cntArg []driver.Value
	}{
		{
			nome:  "ListarTodos",
			path:  "/api/senha-historico",
			count: `SELECT COUNT\(\*\) FROM senha_historico`,
			lista: `SELECT ` + senhaHistoricoColunas + `\s+` + senhaHistoricoJoins + `\s+ORDER BY sh\.id DESC\s+LIMIT \? OFFSET \?`,
			args:  []driver.Value{20, 0},
		},
		{
			nome:   "ListarPorUsuario",
			path:   "/api/senha-historico/5",
			count:  `SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`,
			lista:  `SELECT ` + senhaHistoricoColunas + `\s+` + senhaHistoricoJoins + `\s+WHERE sh\.usuario_id = \?\s+ORDER BY sh\.id DESC\s+LIMIT \? OFFSET \?`,
			args:   []driver.Value{int64(5), 20, 0},
			cntArg: []driver.Value{int64(5)},
		},
	}
	cenarios := []struct {
		nome   string
		linhas [][]driver.Value
	}{
		{"só linha NULL", [][]driver.Value{linhaNull}},
		{"linha NULL depois de preenchida", [][]driver.Value{linhaCheia, linhaNull}},
		{"linha NULL antes de preenchida", [][]driver.Value{linhaNull, linhaCheia}},
	}

	for _, r := range rotas {
		for _, c := range cenarios {
			r, c := r, c
			t.Run(r.nome+"/"+c.nome, func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()
				adminToken := generateToken(t, testCfg(), 1, "admin")

				cnt := mock.ExpectQuery(r.count)
				if r.cntArg != nil {
					cnt.WithArgs(r.cntArg...)
				}
				cnt.WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(len(c.linhas)))
				rows := sqlmock.NewRows(bug12Colunas)
				for _, l := range c.linhas {
					rows.AddRow(l...)
				}
				mock.ExpectQuery(r.lista).WithArgs(r.args...).WillReturnRows(rows)

				req, _ := http.NewRequest("GET", server.URL+r.path, nil)
				req.Header.Set("Authorization", "Bearer "+adminToken)
				resp, err := (&http.Client{}).Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				require.Equal(t, http.StatusOK, resp.StatusCode, "linha com NULL não pode virar 500")
				body := decodeResponse(t, readBody(t, resp))
				assert.True(t, body["success"].(bool))
				data := body["data"].([]any)
				require.Len(t, data, len(c.linhas))
				assert.Equal(t, float64(len(c.linhas)), body["pagination"].(map[string]any)["total"])

				for _, it := range data {
					item := it.(map[string]any)
					switch item["id"] {
					case float64(1):
						assert.Equal(t, "", item["ip_origem"])
						assert.Equal(t, "", item["user_agent"])
						assert.Equal(t, "0001-01-01T00:00:00Z", item["created_at"])
						assert.Equal(t, "", item["usuario_nome"])
						assert.Equal(t, "usuario", item["tipo_reset"])
						_, temRP := item["resetado_por_id"]
						_, temRPNome := item["resetado_por_nome"]
						assert.False(t, temRP)
						assert.False(t, temRPNome)
					case float64(2):
						assert.Equal(t, "10.0.0.2", item["ip_origem"])
						assert.Equal(t, "Mozilla/5.0", item["user_agent"])
						assert.Equal(t, "2026-09-20T14:30:05Z", item["created_at"])
						assert.Equal(t, "Fulano", item["usuario_nome"])
						assert.Equal(t, float64(9), item["resetado_por_id"])
						assert.Equal(t, "Admin", item["resetado_por_nome"])
					default:
						t.Fatalf("id inesperado: %v", item["id"])
					}
					_, temHash := item["senha_hash_anterior"]
					assert.False(t, temHash, "senha_hash_anterior não pode ser serializado")
				}
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}
