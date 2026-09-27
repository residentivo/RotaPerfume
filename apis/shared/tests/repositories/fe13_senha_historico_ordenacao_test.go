package repositories_test

// FE-13: novas chaves de ordenação da listagem de histórico de senhas
// (usuario_nome -> u1.nome, resetado_por_nome -> u2.nome, ip_origem ->
// sh.ip_origem) com desempate estável sh.id DESC, em FindAll e FindByUsuario.
// Chave fora da whitelist / injeção caem em sh.id (sem desempate duplicado).
// O teste de integração (INTEGRATION=1) confere a ordem real no MySQL,
// inclusive NULL em resetado_por_nome (reset feito pelo próprio usuário).

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

type fe13Metodo struct {
	nome     string
	count    string
	listArgs []any
	chamar   func(db *sql.DB, orderBy, orderDir string) ([]repositories.SenhaHistorico, int, error)
}

func fe13Metodos() []fe13Metodo {
	repo := repositories.NewSenhaHistoricoRepository()
	return []fe13Metodo{
		{
			nome:     "FindAll",
			count:    `SELECT COUNT\(\*\) FROM senha_historico$`,
			listArgs: []any{20, 0},
			chamar: func(db *sql.DB, orderBy, orderDir string) ([]repositories.SenhaHistorico, int, error) {
				return repo.FindAll(context.Background(), db, 1, 20, orderBy, orderDir)
			},
		},
		{
			nome:     "FindByUsuario",
			count:    `SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`,
			listArgs: []any{int64(9), 20, 0},
			chamar: func(db *sql.DB, orderBy, orderDir string) ([]repositories.SenhaHistorico, int, error) {
				return repo.FindByUsuario(context.Background(), db, 9, 1, 20, orderBy, orderDir)
			},
		},
	}
}

func fe13Executar(t *testing.T, m fe13Metodo, orderBy, orderDir string) string {
	t.Helper()
	db, mock, captured := newCapturingMock(t)
	c := mock.ExpectQuery(m.count)
	if len(m.listArgs) == 3 {
		c.WithArgs(int64(9))
	}
	c.WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	args := make([]driver.Value, 0, len(m.listArgs))
	for _, a := range m.listArgs {
		args = append(args, a)
	}
	mock.ExpectQuery(`FROM senha_historico sh`).
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows(senhaHistoricoColumns).
			AddRow(senhaHistoricoRow(1, 9, nil, "hash", "10.0.0.1", "ua", "usuario", time.Now())...))

	lista, total, err := m.chamar(db, orderBy, orderDir)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, 1, total)
	require.Len(t, lista, 1)

	sqlLista := (*captured)[len(*captured)-1]
	assert.NotContains(t, sqlLista, "DROP")
	assert.NotContains(t, sqlLista, "DELETE")
	assert.Equal(t, 1, strings.Count(normalizeSQL(sqlLista), "ORDER BY"))
	return orderByDe(t, sqlLista)
}

func TestFE13_SenhaHistorico_NovasChaves(t *testing.T) {
	chaves := []struct {
		chave  string
		coluna string
	}{
		{"usuario_nome", "u1.nome"},
		{"resetado_por_nome", "u2.nome"},
		{"ip_origem", "sh.ip_origem"},
	}
	dirs := []struct {
		entrada string
		sql     string
	}{
		{"asc", "ASC"},
		{"desc", "DESC"},
		{"ASC", "ASC"},
		{"Desc", "DESC"},
		{"", "DESC"},
		{"lateral", "DESC"},
	}

	for _, m := range fe13Metodos() {
		for _, c := range chaves {
			for _, d := range dirs {
				m, c, d := m, c, d
				t.Run(m.nome+"/"+c.chave+"/"+d.entrada, func(t *testing.T) {
					got := fe13Executar(t, m, c.chave, d.entrada)
					assert.Equal(t, "ORDER BY "+c.coluna+" "+d.sql+", sh.id DESC", got)
				})
			}
			// Chave com espaços e caixa mista também é aceita.
			m, c := m, c
			t.Run(m.nome+"/"+c.chave+"/normalizada", func(t *testing.T) {
				got := fe13Executar(t, m, "  "+strings.ToUpper(c.chave)+" ", "asc")
				assert.Equal(t, "ORDER BY "+c.coluna+" ASC, sh.id DESC", got)
			})
		}
	}
}

func TestFE13_SenhaHistorico_ForaDaWhitelist_SemDesempateDuplicado(t *testing.T) {
	casos := []struct {
		nome     string
		orderBy  string
		orderDir string
		want     string
	}{
		{"vazio", "", "", "ORDER BY sh.id DESC"},
		{"id desc", "id", "desc", "ORDER BY sh.id DESC"},
		{"id asc", "id", "asc", "ORDER BY sh.id ASC"},
		{"coluna real mas não exposta", "user_agent", "desc", "ORDER BY sh.id DESC"},
		{"hash anterior", "senha_hash_anterior", "", "ORDER BY sh.id DESC"},
		{"alias SQL direto", "u2.nome", "desc", "ORDER BY sh.id DESC"},
		{"coluna SQL direta", "sh.ip_origem", "desc", "ORDER BY sh.id DESC"},
		{"injeção com ;", "resetado_por_nome; DROP TABLE usuarios;--", "desc", "ORDER BY sh.id DESC"},
		{"injeção com vírgula", "usuario_nome, (SELECT 1)", "desc", "ORDER BY sh.id DESC"},
		{"injeção com DELETE", "ip_origem DESC; DELETE FROM senha_historico", "", "ORDER BY sh.id DESC"},
		{"injeção em order_dir", "ip_origem", "desc; DROP TABLE x", "ORDER BY sh.ip_origem DESC, sh.id DESC"},
	}
	for _, m := range fe13Metodos() {
		for _, tc := range casos {
			m, tc := m, tc
			t.Run(m.nome+"/"+tc.nome, func(t *testing.T) {
				got := fe13Executar(t, m, tc.orderBy, tc.orderDir)
				assert.Equal(t, tc.want, got)
				assert.LessOrEqual(t, strings.Count(got, "sh.id"), 1, "desempate não pode duplicar sh.id")
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Integração com o MySQL local (INTEGRATION=1)
// ---------------------------------------------------------------------------

// TestIntegracaoFE13_OrdenacaoReal cria 3 usuários temporários (e-mails
// zz-test-fe13-*@teste.local, apagados no Cleanup) e 5 registros de histórico
// para o usuário alvo, com resetado_por NULL/A/B e IPs repetidos.
func TestIntegracaoFE13_OrdenacaoReal(t *testing.T) {
	db := abrirDBSEC08(t)
	ctx := context.Background()
	repo := repositories.NewSenhaHistoricoRepository()
	sufixo := time.Now().UnixNano()

	criarUsuario := func(tag string) int64 {
		t.Helper()
		res, err := db.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, ativo) VALUES (?, ?, 'x', 'normal', 1)`,
			fmt.Sprintf("ZZ-FE13-%s-%d", tag, sufixo), fmt.Sprintf("zz-test-fe13-%s-%d@teste.local", strings.ToLower(tag), sufixo))
		require.NoError(t, err)
		id, err := res.LastInsertId()
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := db.Exec(`DELETE FROM usuarios WHERE id = ?`, id)
			assert.NoError(t, err)
		})
		return id
	}
	// Cleanups rodam em ordem inversa: o alvo (criado por último) sai
	// primeiro e leva o histórico em cascata.
	adminA := criarUsuario("A")
	adminB := criarUsuario("B")
	alvo := criarUsuario("ALVO")

	type linha struct {
		resetadoPor *int64
		ip          string
	}
	linhas := []linha{
		{nil, "10.0.0.2"},     // r1
		{&adminB, "10.0.0.1"}, // r2
		{&adminA, "10.0.0.2"}, // r3
		{nil, "10.0.0.1"},     // r4
		{&adminA, "10.0.0.3"}, // r5
	}
	ids := make([]int64, len(linhas))
	for i, l := range linhas {
		h := &repositories.SenhaHistorico{UsuarioID: alvo, SenhaHashAnterior: "x", IPOrigem: l.ip, UserAgent: "fe13", TipoReset: "usuario"}
		if l.resetadoPor != nil {
			h.ResetadoPorID = sql.NullInt64{Int64: *l.resetadoPor, Valid: true}
			h.TipoReset = "admin"
		}
		require.NoError(t, repo.Create(ctx, db, h))
		ids[i] = h.ID
	}
	r1, r2, r3, r4, r5 := ids[0], ids[1], ids[2], ids[3], ids[4]

	casos := []struct {
		orderBy, orderDir string
		want              []int64
	}{
		// NULL primeiro no ASC (MySQL); empates em sh.id DESC.
		{"resetado_por_nome", "asc", []int64{r4, r1, r5, r3, r2}},
		{"resetado_por_nome", "desc", []int64{r2, r5, r3, r4, r1}},
		{"ip_origem", "asc", []int64{r4, r2, r3, r1, r5}},
		{"ip_origem", "desc", []int64{r5, r3, r1, r4, r2}},
		// Todos com o mesmo usuario_nome: só o desempate decide.
		{"usuario_nome", "asc", []int64{r5, r4, r3, r2, r1}},
		{"usuario_nome", "desc", []int64{r5, r4, r3, r2, r1}},
		{"id", "asc", []int64{r1, r2, r3, r4, r5}},
		{"invalida", "asc", []int64{r1, r2, r3, r4, r5}},
	}

	idsDe := func(hs []repositories.SenhaHistorico) []int64 {
		out := make([]int64, 0, len(hs))
		for _, h := range hs {
			out = append(out, h.ID)
		}
		return out
	}
	nossos := map[int64]bool{r1: true, r2: true, r3: true, r4: true, r5: true}

	for _, tc := range casos {
		tc := tc
		t.Run("FindByUsuario/"+tc.orderBy+"/"+tc.orderDir, func(t *testing.T) {
			hs, total, err := repo.FindByUsuario(ctx, db, alvo, 1, 20, tc.orderBy, tc.orderDir)
			require.NoError(t, err)
			assert.Equal(t, 5, total)
			assert.Equal(t, tc.want, idsDe(hs))
			for _, h := range hs {
				if h.ID == r1 || h.ID == r4 {
					assert.False(t, h.ResetadoPorNome.Valid, "reset pelo próprio usuário: resetado_por_nome NULL")
				} else {
					assert.True(t, h.ResetadoPorNome.Valid)
				}
			}
		})

		t.Run("FindByUsuario/paginado/"+tc.orderBy+"/"+tc.orderDir, func(t *testing.T) {
			// Páginas de 2: o desempate impede repetir/pular linhas.
			var got []int64
			for page := 1; page <= 3; page++ {
				hs, _, err := repo.FindByUsuario(ctx, db, alvo, page, 2, tc.orderBy, tc.orderDir)
				require.NoError(t, err)
				got = append(got, idsDe(hs)...)
			}
			assert.Equal(t, tc.want, got)
		})

		t.Run("FindAll/"+tc.orderBy+"/"+tc.orderDir, func(t *testing.T) {
			// Percorre todas as páginas e confere a subsequência dos nossos
			// registros (os demais dados do banco se intercalam). Os nomes
			// ZZ-FE13-* não empatam com terceiros, então a subsequência é a
			// mesma do FindByUsuario — exceto por usuario_nome, em que outros
			// usuários não interferem entre os nossos (mesmo u1.nome).
			var sub []int64
			for page := 1; ; page++ {
				hs, total, err := repo.FindAll(ctx, db, page, 100, tc.orderBy, tc.orderDir)
				require.NoError(t, err)
				for _, id := range idsDe(hs) {
					if nossos[id] {
						sub = append(sub, id)
					}
				}
				if page*100 >= total || len(hs) == 0 {
					break
				}
			}
			assert.Equal(t, tc.want, sub)
		})
	}
}
