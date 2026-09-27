package handlers_test

// Lote 9:
//   - BUG-11: POST /api/usuarios responde 201 com a linha relida do banco
//     (timestamps/vendedor_nome) e, se só a releitura falhar, 201 com o objeto
//     em memória e email_enviado correto (nunca 500 para INSERT confirmado).
//   - FE-13: GET /api/senha-historico[/{id}] repassa order_by/order_dir ao
//     repositório com as novas chaves (usuario_nome, resetado_por_nome,
//     ip_origem) e o desempate sh.id DESC.

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBUG11_HTTP_CreateUsuario_Releitura(t *testing.T) {
	criado := time.Date(2026, 9, 26, 10, 30, 0, 0, time.UTC)
	casos := []struct {
		nome     string
		falha    error // nil = releitura OK
		idVend   any
		vendNome any
		payload  map[string]any
	}{
		{"releitura ok sem vendedor", nil, nil, nil, validCreateUsuarioPayload()},
		{"releitura ok com vendedor", nil, int64(3), "Vendedor Do Banco", map[string]any{"nome": "Novo Usuário", "email": "novo@test.com", "role": "normal", "id_vendedor": 3}},
		{"releitura falha: db down", errors.New("db down"), nil, nil, validCreateUsuarioPayload()},
		{"releitura falha: ErrNoRows", sql.ErrNoRows, nil, nil, validCreateUsuarioPayload()},
	}

	for _, tc := range casos {
		tc := tc
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			adminToken := generateToken(t, testCfg(), 1, "admin")

			if tc.idVend != nil {
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(tc.idVend).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			}
			mock.ExpectExec(`INSERT INTO usuarios \(nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha\)`).
				WithArgs("Novo Usuário", "novo@test.com", sqlmock.AnyArg(), "normal", tc.idVend, true, true).
				WillReturnResult(sqlmock.NewResult(9, 1))
			sel := mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).WithArgs(int64(9))
			if tc.falha != nil {
				sel.WillReturnError(tc.falha)
			} else {
				sel.WillReturnRows(sqlmock.NewRows([]string{
					"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo",
					"deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome",
				}).AddRow(int64(9), "Novo Usuário", "novo@test.com", "hash", "normal", tc.idVend, true, true, criado, criado, nil, tc.vendNome))
			}

			req, _ := http.NewRequest("POST", server.URL+"/api/usuarios", makeJSON(tc.payload))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusCreated, resp.StatusCode, "INSERT confirmado responde 201 mesmo se a releitura falhar")
			body := decodeResponse(t, readBody(t, resp))
			assert.True(t, body["success"].(bool))
			data := body["data"].(map[string]any)
			assert.Equal(t, float64(9), data["id"])
			assert.Equal(t, "novo@test.com", data["email"])
			assert.Equal(t, true, data["email_enviado"])
			_, temHash := data["password_hash"]
			assert.False(t, temHash)
			if tc.falha == nil {
				assert.Equal(t, criado.Format(time.RFC3339), data["created_at"])
				assert.Equal(t, criado.Format(time.RFC3339), data["updated_at"])
				if tc.vendNome != nil {
					assert.Equal(t, tc.vendNome, data["vendedor_nome"])
				}
			} else {
				assert.Equal(t, "0001-01-01T00:00:00Z", data["created_at"], "fallback: objeto em memória")
				assert.Equal(t, "", data["vendedor_nome"], "fallback sem JOIN: vendedor_nome vazio")
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFE13_HTTP_SenhaHistorico_OrderBy(t *testing.T) {
	type rota struct {
		nome  string
		path  string
		where string
		args  []any
	}
	rotas := []rota{
		{"ListarTodos", "/api/senha-historico", ``, []any{20, 0}},
		{"ListarPorUsuario", "/api/senha-historico/5", `WHERE sh\.usuario_id = \?\s+`, []any{int64(5), 20, 0}},
	}
	casos := []struct {
		nome    string
		orderBy string
		dir     string
		order   string
	}{
		{"usuario_nome asc", "usuario_nome", "asc", `ORDER BY u1\.nome ASC, sh\.id DESC`},
		{"usuario_nome desc", "usuario_nome", "desc", `ORDER BY u1\.nome DESC, sh\.id DESC`},
		{"resetado_por_nome asc", "resetado_por_nome", "asc", `ORDER BY u2\.nome ASC, sh\.id DESC`},
		{"resetado_por_nome desc", "resetado_por_nome", "desc", `ORDER BY u2\.nome DESC, sh\.id DESC`},
		{"ip_origem asc", "ip_origem", "asc", `ORDER BY sh\.ip_origem ASC, sh\.id DESC`},
		{"ip_origem desc", "ip_origem", "desc", `ORDER BY sh\.ip_origem DESC, sh\.id DESC`},
		{"ip_origem ASC maiúsculo", "ip_origem", "ASC", `ORDER BY sh\.ip_origem ASC, sh\.id DESC`},
		{"dir ausente = desc", "resetado_por_nome", "", `ORDER BY u2\.nome DESC, sh\.id DESC`},
		{"id asc sem desempate", "id", "asc", `ORDER BY sh\.id ASC`},
		{"sem order_by", "", "", `ORDER BY sh\.id DESC`},
		{"chave inválida", "senha_hash_anterior", "desc", `ORDER BY sh\.id DESC`},
		{"injeção", "u2.nome; DROP TABLE usuarios;--", "desc", `ORDER BY sh\.id DESC`},
		{"injeção em order_dir", "usuario_nome", "desc; DROP TABLE x", `ORDER BY u1\.nome DESC, sh\.id DESC`},
	}

	for _, r := range rotas {
		for _, tc := range casos {
			r, tc := r, tc
			t.Run(r.nome+"/"+tc.nome, func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()
				adminToken := generateToken(t, testCfg(), 1, "admin")

				count := mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico`)
				if len(r.args) == 3 {
					count.WithArgs(int64(5))
				}
				count.WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				// \s+LIMIT exige que nada venha depois do ORDER BY esperado
				// (sem desempate duplicado nem resto de injeção).
				args := make([]driver.Value, 0, len(r.args))
				for _, a := range r.args {
					args = append(args, a)
				}
				mock.ExpectQuery(`SELECT ` + senhaHistoricoColunas + `\s+` + senhaHistoricoJoins + `\s+` + r.where + tc.order + `\s+LIMIT \? OFFSET \?$`).
					WithArgs(args...).
					WillReturnRows(senhaHistoricoRowsSemResetadoPor())

				q := url.Values{}
				if tc.orderBy != "" {
					q.Set("order_by", tc.orderBy)
				}
				if tc.dir != "" {
					q.Set("order_dir", tc.dir)
				}
				req, _ := http.NewRequest("GET", server.URL+r.path+"?"+q.Encode(), nil)
				req.Header.Set("Authorization", "Bearer "+adminToken)
				resp, err := (&http.Client{}).Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				body := decodeResponse(t, readBody(t, resp))
				require.Len(t, body["data"].([]any), 1)
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}
