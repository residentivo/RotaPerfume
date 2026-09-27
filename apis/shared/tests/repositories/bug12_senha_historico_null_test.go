package repositories_test

// BUG-12: colunas anuláveis da listagem de histórico de senhas
// (sh.ip_origem, sh.user_agent, sh.created_at e u1.nome, este vindo de
// LEFT JOIN) derrubavam FindAll/FindByUsuario inteiros com erro de Scan
// quando uma única linha vinha NULL. Agora scanSenhaHistorico lê em
// sql.Null* e converte para o valor zero ("" / time.Time{}).
//
// Os testes sqlmock cobrem FindAll e FindByUsuario (parametrizados) com
// linhas totalmente NULL, parcialmente NULL, preenchidas e páginas mistas;
// o teste de integração (INTEGRATION=1) grava NULLs reais no MySQL.

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

// bug12Linha descreve uma linha do SELECT da listagem. Ponteiros nil
// representam NULL no banco.
type bug12Linha struct {
	id          int64
	resetadoPor *int64
	ip          *string
	userAgent   *string
	createdAt   *time.Time
	usuarioNome *string
	resetNome   *string
}

func bug12Str(s string) *string      { return &s }
func bug12Int(i int64) *int64        { return &i }
func bug12Tm(t time.Time) *time.Time { return &t }

func bug12Val[T any](p *T) driver.Value {
	if p == nil {
		return nil
	}
	return *p
}

func (l bug12Linha) valores(usuarioID int64) []driver.Value {
	return []driver.Value{
		l.id, usuarioID, bug12Val(l.resetadoPor), "hash", bug12Val(l.ip), bug12Val(l.userAgent),
		"usuario", bug12Val(l.createdAt), bug12Val(l.usuarioNome), bug12Val(l.resetNome),
	}
}

// conferir valida o registro lido contra a linha de origem: NULL vira o
// valor zero; valor preenchido chega intacto.
func (l bug12Linha) conferir(t *testing.T, h repositories.SenhaHistorico) {
	t.Helper()
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	assert.Equal(t, l.id, h.ID)
	assert.Equal(t, deref(l.ip), h.IPOrigem, "ip_origem")
	assert.Equal(t, deref(l.userAgent), h.UserAgent, "user_agent")
	assert.Equal(t, deref(l.usuarioNome), h.UsuarioNome, "usuario_nome")
	if l.createdAt == nil {
		assert.True(t, h.CreatedAt.IsZero(), "created_at NULL deve virar time.Time{}")
	} else {
		assert.True(t, l.createdAt.Equal(h.CreatedAt), "created_at preenchido deve chegar intacto")
	}
	assert.Equal(t, l.resetadoPor != nil, h.ResetadoPorID.Valid)
	if l.resetadoPor != nil {
		assert.Equal(t, *l.resetadoPor, h.ResetadoPorID.Int64)
	}
	assert.Equal(t, l.resetNome != nil, h.ResetadoPorNome.Valid)
	assert.Equal(t, deref(l.resetNome), h.ResetadoPorNome.String)
	assert.Equal(t, "hash", h.SenhaHashAnterior)
	assert.Equal(t, "usuario", h.TipoReset)
}

type bug12Metodo struct {
	nome   string
	count  string
	lista  string
	args   []driver.Value
	chamar func(db *sql.DB) ([]repositories.SenhaHistorico, int, error)
}

const bug12UsuarioID = int64(7)

func bug12Metodos() []bug12Metodo {
	repo := repositories.NewSenhaHistoricoRepository()
	return []bug12Metodo{
		{
			nome:  "FindAll",
			count: `SELECT COUNT\(\*\) FROM senha_historico$`,
			lista: `FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id ORDER BY sh\.id DESC LIMIT \? OFFSET \?`,
			args:  []driver.Value{20, 0},
			chamar: func(db *sql.DB) ([]repositories.SenhaHistorico, int, error) {
				return repo.FindAll(context.Background(), db, 1, 20, "", "")
			},
		},
		{
			nome:  "FindByUsuario",
			count: `SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`,
			lista: `FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id WHERE sh\.usuario_id = \? ORDER BY sh\.id DESC LIMIT \? OFFSET \?`,
			args:  []driver.Value{bug12UsuarioID, 20, 0},
			chamar: func(db *sql.DB) ([]repositories.SenhaHistorico, int, error) {
				return repo.FindByUsuario(context.Background(), db, bug12UsuarioID, 1, 20, "", "")
			},
		},
	}
}

func bug12Preparar(t *testing.T, m bug12Metodo, rows *sqlmock.Rows, total int) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock := newMock(t)
	t.Cleanup(func() { db.Close() })
	c := mock.ExpectQuery(m.count)
	if m.nome == "FindByUsuario" {
		c.WithArgs(bug12UsuarioID)
	}
	c.WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(total))
	mock.ExpectQuery(m.lista).WithArgs(m.args...).WillReturnRows(rows)
	return db, mock
}

func TestBUG12_SenhaHistorico_ColunasNull(t *testing.T) {
	criado := time.Date(2026, 9, 20, 14, 30, 5, 0, time.UTC)
	preenchida := bug12Linha{
		id: 10, resetadoPor: bug12Int(1), ip: bug12Str("192.168.0.10"), userAgent: bug12Str("Mozilla/5.0"),
		createdAt: bug12Tm(criado), usuarioNome: bug12Str("Fulano"), resetNome: bug12Str("Admin"),
	}
	casos := []struct {
		nome   string
		linhas []bug12Linha
	}{
		{"tudo NULL", []bug12Linha{{id: 1}}},
		{"só ip_origem NULL", []bug12Linha{{id: 2, userAgent: bug12Str("ua"), createdAt: bug12Tm(criado), usuarioNome: bug12Str("Fulano")}}},
		{"só user_agent NULL", []bug12Linha{{id: 3, ip: bug12Str("10.0.0.1"), createdAt: bug12Tm(criado), usuarioNome: bug12Str("Fulano")}}},
		{"só created_at NULL", []bug12Linha{{id: 4, ip: bug12Str("10.0.0.1"), userAgent: bug12Str("ua"), usuarioNome: bug12Str("Fulano")}}},
		{"só usuario_nome NULL (LEFT JOIN órfão)", []bug12Linha{{id: 5, ip: bug12Str("10.0.0.1"), userAgent: bug12Str("ua"), createdAt: bug12Tm(criado)}}},
		{"NULLs com resetado_por preenchido", []bug12Linha{{id: 6, resetadoPor: bug12Int(3), resetNome: bug12Str("Admin B")}}},
		{"strings vazias (não NULL) continuam vazias", []bug12Linha{{id: 7, ip: bug12Str(""), userAgent: bug12Str(""), createdAt: bug12Tm(criado), usuarioNome: bug12Str("")}}},
		{"tudo preenchido intacto", []bug12Linha{preenchida}},
		{"página mista NULL e preenchidas", []bug12Linha{
			preenchida,
			{id: 9},
			{id: 8, ip: bug12Str("10.0.0.8"), usuarioNome: bug12Str("Beltrano")},
			{id: 7, userAgent: bug12Str("curl/8"), createdAt: bug12Tm(criado.Add(-time.Hour)), resetadoPor: bug12Int(2), resetNome: bug12Str("Admin A")},
			{id: 6, ip: bug12Str("10.0.0.6"), userAgent: bug12Str("ua6"), createdAt: bug12Tm(criado.Add(-2 * time.Hour)), usuarioNome: bug12Str("Ciclano")},
		}},
		{"NULL no fim da página não descarta as anteriores", []bug12Linha{preenchida, {id: 2}}},
	}

	for _, m := range bug12Metodos() {
		for _, tc := range casos {
			m, tc := m, tc
			t.Run(m.nome+"/"+tc.nome, func(t *testing.T) {
				rows := sqlmock.NewRows(senhaHistoricoColumns)
				for _, l := range tc.linhas {
					rows.AddRow(l.valores(bug12UsuarioID)...)
				}
				db, mock := bug12Preparar(t, m, rows, len(tc.linhas))

				lista, total, err := m.chamar(db)

				require.NoError(t, err, "NULL em coluna anulável não pode derrubar a listagem")
				require.NoError(t, mock.ExpectationsWereMet())
				assert.Equal(t, len(tc.linhas), total)
				require.Len(t, lista, len(tc.linhas))
				for i, l := range tc.linhas {
					assert.Equal(t, bug12UsuarioID, lista[i].UsuarioID)
					l.conferir(t, lista[i])
				}
			})
		}
	}
}

// Erros de Scan que não são NULL (tipo incompatível) continuam propagando
// com o prefixo do repositório, sem retornar lista parcial.
func TestBUG12_SenhaHistorico_ScanTipoInvalido(t *testing.T) {
	criado := time.Date(2026, 9, 20, 14, 30, 5, 0, time.UTC)
	casos := []struct {
		nome  string
		mudar func(v []driver.Value)
	}{
		{"created_at não é data", func(v []driver.Value) { v[7] = "nao-e-data" }},
		{"id não numérico", func(v []driver.Value) { v[0] = "abc" }},
		{"usuario_id NULL (NOT NULL no schema)", func(v []driver.Value) { v[1] = nil }},
		{"senha_hash_anterior NULL (NOT NULL no schema)", func(v []driver.Value) { v[3] = nil }},
		{"resetado_por_id não numérico", func(v []driver.Value) { v[2] = "x" }},
	}
	for _, m := range bug12Metodos() {
		for _, tc := range casos {
			m, tc := m, tc
			t.Run(m.nome+"/"+tc.nome, func(t *testing.T) {
				ok := bug12Linha{id: 2, ip: bug12Str("10.0.0.1"), createdAt: bug12Tm(criado), usuarioNome: bug12Str("Fulano")}
				ruim := bug12Linha{id: 1}.valores(bug12UsuarioID)
				ruim[7] = criado
				tc.mudar(ruim)
				rows := sqlmock.NewRows(senhaHistoricoColumns).
					AddRow(ok.valores(bug12UsuarioID)...).
					AddRow(ruim...)
				db, _ := bug12Preparar(t, m, rows, 2)

				lista, total, err := m.chamar(db)

				require.Error(t, err)
				assert.Contains(t, err.Error(), "repositories: scan senha_historico")
				assert.NotErrorIs(t, err, repositories.ErrNotFound)
				assert.Nil(t, lista)
				assert.Equal(t, 0, total)
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Integração com o MySQL local (INTEGRATION=1)
// ---------------------------------------------------------------------------

// TestIntegracaoBUG12_NullReal grava, para dois usuários temporários
// (zz-test-bug12-*@teste.local), registros com ip_origem/user_agent/
// created_at NULL misturados a registros preenchidos, e um registro órfão
// (usuario_id sem linha em usuarios, gravado com FOREIGN_KEY_CHECKS=0 na
// sessão) para produzir u1.nome NULL no LEFT JOIN. Tudo é apagado no Cleanup.
func TestIntegracaoBUG12_NullReal(t *testing.T) {
	db := abrirDBSEC08(t)
	ctx := context.Background()
	repo := repositories.NewSenhaHistoricoRepository()
	sufixo := time.Now().UnixNano()

	criarUsuario := func(tag string) int64 {
		t.Helper()
		res, err := db.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, ativo) VALUES (?, ?, 'x', 'normal', 1)`,
			fmt.Sprintf("ZZ-BUG12-%s-%d", tag, sufixo), fmt.Sprintf("zz-test-bug12-%s-%d@teste.local", strings.ToLower(tag), sufixo))
		require.NoError(t, err)
		id, err := res.LastInsertId()
		require.NoError(t, err)
		t.Cleanup(func() {
			// ON DELETE CASCADE leva o histórico junto.
			_, err := db.Exec(`DELETE FROM usuarios WHERE id = ?`, id)
			assert.NoError(t, err)
		})
		return id
	}
	admin := criarUsuario("ADMIN")
	alvo := criarUsuario("ALVO")

	type reg struct {
		ip, ua   any // nil => NULL
		created  any
		resetPor any
	}
	criado := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	regs := []reg{
		{nil, nil, nil, nil},                  // tudo NULL
		{"10.0.0.2", "ua-bug12", criado, nil}, // preenchido
		{nil, "ua-bug12", criado, admin},      // só ip NULL, reset por admin
		{"10.0.0.1", nil, nil, nil},           // user_agent e created_at NULL
		{"10.0.0.3", "ua-bug12", nil, admin},  // só created_at NULL
	}
	ids := make([]int64, len(regs))
	for i, r := range regs {
		tipo := "usuario"
		if r.resetPor != nil {
			tipo = "admin"
		}
		res, err := db.Exec(`INSERT INTO senha_historico (usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset, created_at)
			VALUES (?, ?, 'x', ?, ?, ?, ?)`, alvo, r.resetPor, r.ip, r.ua, tipo, r.created)
		require.NoError(t, err)
		ids[i], err = res.LastInsertId()
		require.NoError(t, err)
	}

	// Garante que os NULLs foram realmente gravados (explicit_defaults_for_timestamp).
	var nullCreated int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM senha_historico WHERE usuario_id = ? AND created_at IS NULL`, alvo).Scan(&nullCreated))
	require.Equal(t, 3, nullCreated, "o MySQL deve gravar created_at NULL explícito")

	// Registro órfão: usuario_id inexistente => u1.nome NULL no LEFT JOIN.
	orfaoUsuario := int64(9_000_000_000) + sufixo%1_000_000
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `SET SESSION foreign_key_checks = 0`)
	require.NoError(t, err)
	res, err := conn.ExecContext(ctx, `INSERT INTO senha_historico (usuario_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset, created_at)
		VALUES (?, 'x', NULL, NULL, 'usuario', NULL)`, orfaoUsuario)
	_, errFK := conn.ExecContext(ctx, `SET SESSION foreign_key_checks = 1`)
	require.NoError(t, errFK)
	require.NoError(t, conn.Close())
	require.NoError(t, err)
	orfaoID, err := res.LastInsertId()
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec(`DELETE FROM senha_historico WHERE id = ?`, orfaoID)
		assert.NoError(t, err)
	})

	esperado := map[int64]struct {
		ip, ua, nome string
		createdZero  bool
		resetado     bool
	}{
		ids[0]:  {"", "", "ZZ-BUG12-ALVO-" + fmt.Sprint(sufixo), true, false},
		ids[1]:  {"10.0.0.2", "ua-bug12", "ZZ-BUG12-ALVO-" + fmt.Sprint(sufixo), false, false},
		ids[2]:  {"", "ua-bug12", "ZZ-BUG12-ALVO-" + fmt.Sprint(sufixo), false, true},
		ids[3]:  {"10.0.0.1", "", "ZZ-BUG12-ALVO-" + fmt.Sprint(sufixo), true, false},
		ids[4]:  {"10.0.0.3", "ua-bug12", "ZZ-BUG12-ALVO-" + fmt.Sprint(sufixo), true, true},
		orfaoID: {"", "", "", true, false},
	}
	conferir := func(t *testing.T, h repositories.SenhaHistorico) {
		t.Helper()
		e := esperado[h.ID]
		assert.Equal(t, e.ip, h.IPOrigem, "ip_origem id=%d", h.ID)
		assert.Equal(t, e.ua, h.UserAgent, "user_agent id=%d", h.ID)
		assert.Equal(t, e.nome, h.UsuarioNome, "usuario_nome id=%d", h.ID)
		assert.Equal(t, e.createdZero, h.CreatedAt.IsZero(), "created_at id=%d", h.ID)
		if !e.createdZero {
			assert.True(t, criado.Equal(h.CreatedAt), "created_at id=%d: %v", h.ID, h.CreatedAt)
		}
		assert.Equal(t, e.resetado, h.ResetadoPorID.Valid, "resetado_por_id id=%d", h.ID)
		assert.Equal(t, e.resetado, h.ResetadoPorNome.Valid, "resetado_por_nome id=%d", h.ID)
	}

	ordenacoes := []struct{ orderBy, orderDir string }{
		{"ip_origem", "asc"},
		{"ip_origem", "desc"},
		{"created_at", "asc"},
		{"usuario_nome", "asc"},
		{"", ""},
	}

	for _, o := range ordenacoes {
		o := o
		t.Run("FindAll/"+o.orderBy+"/"+o.orderDir, func(t *testing.T) {
			vistos := map[int64]bool{}
			var subIPs []string
			for page := 1; ; page++ {
				hs, total, err := repo.FindAll(ctx, db, page, 100, o.orderBy, o.orderDir)
				require.NoError(t, err, "FindAll não pode falhar com NULLs no banco")
				for _, h := range hs {
					if _, nosso := esperado[h.ID]; nosso {
						vistos[h.ID] = true
						conferir(t, h)
						subIPs = append(subIPs, h.IPOrigem)
					}
				}
				if page*100 >= total || len(hs) == 0 {
					break
				}
			}
			assert.Len(t, vistos, len(esperado), "todos os registros com NULL devem aparecer")
			if o.orderBy == "ip_origem" && o.orderDir == "asc" {
				// NULL (vazio) primeiro no ASC do MySQL, depois IPs em ordem.
				assert.Equal(t, []string{"", "", "", "10.0.0.1", "10.0.0.2", "10.0.0.3"}, subIPs)
			}
			if o.orderBy == "ip_origem" && o.orderDir == "desc" {
				assert.Equal(t, []string{"10.0.0.3", "10.0.0.2", "10.0.0.1", "", "", ""}, subIPs)
			}
		})
	}

	t.Run("FindAll/ip_origem/asc/paginado", func(t *testing.T) {
		// Páginas pequenas: um NULL em qualquer página não pode derrubar a listagem.
		_, total, err := repo.FindAll(ctx, db, 1, 1, "ip_origem", "asc")
		require.NoError(t, err)
		for page := 1; (page-1)*5 < total; page++ {
			_, _, err := repo.FindAll(ctx, db, page, 5, "ip_origem", "asc")
			require.NoError(t, err, "página %d", page)
		}
	})

	for _, o := range ordenacoes {
		o := o
		t.Run("FindByUsuario/"+o.orderBy+"/"+o.orderDir, func(t *testing.T) {
			hs, total, err := repo.FindByUsuario(ctx, db, alvo, 1, 20, o.orderBy, o.orderDir)
			require.NoError(t, err)
			assert.Equal(t, len(regs), total)
			require.Len(t, hs, len(regs))
			for _, h := range hs {
				conferir(t, h)
			}
		})
	}

	t.Run("FindByUsuario/orfao", func(t *testing.T) {
		hs, total, err := repo.FindByUsuario(ctx, db, orfaoUsuario, 1, 20, "", "")
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, hs, 1)
		conferir(t, hs[0])
	})
}
