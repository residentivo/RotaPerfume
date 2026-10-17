// Package repositories_test contém testes de integração com banco de dados mockado.
// Cada função é stateless — o sqlmock substitui o driver real.
package repositories_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
)

func newMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New não deve falhar")
	return db, mock
}

// baseColumns são as colunas retornadas por todas as queries SELECT neste package.
// vendedor_nome vem do LEFT JOIN com a tabela vendedores.
var baseColumns = []string{
	"id", "nome", "email", "password_hash", "role",
	"id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome",
}

// baseRow cria uma []driver.Value na ordem de baseColumns.
// idVendedor e ultimoLogin são convertidos para driver.Value via sql.Null*.
// vendedorNome é sempre nil (não testado neste helper — ver testes específicos de JOIN).
func baseRow(id int64, nome, email, hash, role string, idVendedor *int64, ativo bool, created, updated time.Time, ultimoLogin *time.Time) []driver.Value {
	var iv driver.Value = nil
	if idVendedor != nil {
		iv = *idVendedor
	}
	var ul driver.Value = nil
	if ultimoLogin != nil {
		ul = *ultimoLogin
	}
	return []driver.Value{id, nome, email, hash, role, iv, ativo, false, created, updated, ul, nil}
}

func TestGetByEmail_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	email := "admin@test.com"
	rows := sqlmock.NewRows(baseColumns).
		AddRow(baseRow(1, "Admin Test", email, "hash123", "admin", nil, true, time.Now(), time.Now(), nil)...)

	mock.ExpectQuery("SELECT .+ FROM usuarios u LEFT JOIN vendedores v ON v.id = u.id_vendedor WHERE u.email = ?").
		WithArgs(email).
		WillReturnRows(rows)

	ctx := context.Background()
	u, err := identidademysql.NovoUsuarios(db).PorEmail(ctx, email)

	require.NoError(t, err)
	assert.NotNil(t, u)
	assert.Equal(t, int64(1), u.ID)
	assert.Equal(t, email, u.Email)
	assert.Equal(t, "admin", u.Role.String())
	assert.True(t, u.Ativo)
	assert.NoError(t, mock.ExpectationsWereMet(), "todas expectativas do mock devem ser satisfeitas")
}

func TestGetByEmail_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM usuarios u LEFT JOIN vendedores v ON v.id = u.id_vendedor WHERE u.email = ?").
		WithArgs("naoexiste@test.com").
		WillReturnError(sql.ErrNoRows)

	ctx := context.Background()
	u, err := identidademysql.NovoUsuarios(db).PorEmail(ctx, "naoexiste@test.com")

	assert.Nil(t, u)
	assert.ErrorIs(t, err, identidade.ErrUsuarioNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	id := int64(5)
	vID := int64(99)
	now := time.Now()
	rows := sqlmock.NewRows(baseColumns).
		AddRow(baseRow(id, "Vendedor Joe", "joe@test.com", "hash-bcrypt", "normal", &vID, true, now, now, nil)...)

	mock.ExpectQuery("SELECT .+ FROM usuarios u LEFT JOIN vendedores v ON v.id = u.id_vendedor WHERE u.id = ?").
		WithArgs(id).
		WillReturnRows(rows)

	ctx := context.Background()
	u, err := identidademysql.NovoUsuarios(db).PorID(ctx, id)

	require.NoError(t, err)
	assert.NotNil(t, u)
	assert.Equal(t, id, u.ID)
	assert.NotNil(t, u.IDVendedor)
	assert.Equal(t, vID, *u.IDVendedor)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM usuarios u LEFT JOIN vendedores v ON v.id = u.id_vendedor WHERE u.id = ?").
		WithArgs(int64(99999)).
		WillReturnError(sql.ErrNoRows)

	ctx := context.Background()
	u, err := identidademysql.NovoUsuarios(db).PorID(ctx, 99999)

	assert.Nil(t, u)
	assert.ErrorIs(t, err, identidade.ErrUsuarioNaoEncontrado) // ARQ-07: o sentinela do domínio substitui repositories.ErrNotFound
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetIDVendedorByUsuarioID_ComVinculo(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))

	ctx := context.Background()
	idVendedor, err := identidademysql.NovoUsuarios(db).VendedorDoUsuario(ctx, 5)

	require.NoError(t, err)
	require.NotNil(t, idVendedor)
	assert.Equal(t, int64(99), *idVendedor)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetIDVendedorByUsuarioID_SemVinculo(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))

	ctx := context.Background()
	idVendedor, err := identidademysql.NovoUsuarios(db).VendedorDoUsuario(ctx, 5)

	require.NoError(t, err)
	assert.Nil(t, idVendedor)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetIDVendedorByUsuarioID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	ctx := context.Background()
	idVendedor, err := identidademysql.NovoUsuarios(db).VendedorDoUsuario(ctx, 999)

	assert.Nil(t, idVendedor)
	assert.ErrorIs(t, err, identidade.ErrUsuarioNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetIDVendedorByUsuarioID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	idVendedor, err := identidademysql.NovoUsuarios(db).VendedorDoUsuario(ctx, 5)

	assert.Nil(t, idVendedor)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestList_Pagination(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	// COUNT(*) retornado antes da query paginada.
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM usuarios").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(15))

	// ROWS: página 1, limit 10 → offset 0 → 10 registros.
	rowsPage1 := sqlmock.NewRows(baseColumns)
	now := time.Now()
	for i := int64(1); i <= 10; i++ {
		rowsPage1.AddRow(baseRow(i, "User", "user@test.com", "hash", "normal", nil, true, now, now, nil)...)
	}
	mock.ExpectQuery("SELECT .+ FROM usuarios").
		WithArgs(10, 0).
		WillReturnRows(rowsPage1)

	ctx := context.Background()
	usuarios, total, err := itensETotal(identidademysql.NovoUsuarios(db).Listar(ctx, ordemUsuario("", ""), kernel.NovaPagina(1, 10)))

	require.NoError(t, err)
	assert.Equal(t, 15, total, "total vem do COUNT")
	assert.Len(t, usuarios, 10, "primeira página deve ter 10")
	assert.Equal(t, int64(1), usuarios[0].ID, "primeiro usuário deve ser ID=1")
	assert.Equal(t, int64(10), usuarios[9].ID, "décimo usuário deve ser ID=10")
	assert.NoError(t, mock.ExpectationsWereMet())

	// Página 2 (mesmo total=15, offset 10, limit 10 → 5 registros).
	db2, mock2 := newMock(t)
	defer db2.Close()

	mock2.ExpectQuery("SELECT COUNT\\(\\*\\) FROM usuarios").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(15))
	rowsPage2 := sqlmock.NewRows(baseColumns)
	for i := int64(11); i <= 15; i++ {
		rowsPage2.AddRow(baseRow(i, "User", "user@test.com", "hash", "normal", nil, true, now, now, nil)...)
	}
	mock2.ExpectQuery("SELECT .+ FROM usuarios").
		WithArgs(10, 10).
		WillReturnRows(rowsPage2)

	usuarios2, total2, err2 := itensETotal(identidademysql.NovoUsuarios(db2).Listar(context.Background(), ordemUsuario("", ""), kernel.NovaPagina(2, 10)))
	require.NoError(t, err2)
	assert.Equal(t, 15, total2)
	assert.Len(t, usuarios2, 5)
	assert.Equal(t, int64(11), usuarios2[0].ID)
	assert.NoError(t, mock2.ExpectationsWereMet())
}

func TestList_Defaults(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	// page=0 → deve corrigir para 1; limit=0 → deve corrigir para 20.
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM usuarios").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM usuarios").
		WithArgs(20, 0). // offset=0 quando page=1
		WillReturnRows(sqlmock.NewRows(baseColumns))

	ctx := context.Background()
	_, _, err := itensETotal(identidademysql.NovoUsuarios(db).Listar(ctx, ordemUsuario("", ""), kernel.NovaPagina(0, 0))) // valores inválidos devem ser corrigidos

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestList_LimitCap(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM usuarios").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	// limit=200 → deve ser capped para 100.
	mock.ExpectQuery("SELECT .+ FROM usuarios").
		WithArgs(100, 0).
		WillReturnRows(sqlmock.NewRows(baseColumns))

	ctx := context.Background()
	_, _, err := itensETotal(identidademysql.NovoUsuarios(db).Listar(ctx, ordemUsuario("", ""), kernel.NovaPagina(1, 200)))

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePasswordHash_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \?, tokens_validos_desde = \? WHERE id = \?`).
		WithArgs("novo-hash-bcrypt", false, sqlmock.AnyArg(), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	err := identidademysql.NovoUsuarios(db).AtualizarSenha(ctx, 7, identidade.NovoHashSenha("novo-hash-bcrypt"), false)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// SEC-13: o rehash só troca o hash se ainda for o antigo e não mexe em
// deve_trocar_senha nem em tokens_validos_desde.
func TestRehashPassword(t *testing.T) {
	const q = `UPDATE usuarios SET password_hash = \? WHERE id = \? AND password_hash = \?`
	casos := []struct {
		nome    string
		result  driver.Result
		err     error
		wantErr bool
	}{
		{"regravado", sqlmock.NewResult(0, 1), nil, false},
		{"hash mudou no meio do caminho (0 linhas) não é erro", sqlmock.NewResult(0, 0), nil, false},
		{"erro do banco", nil, sql.ErrConnDone, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			exp := mock.ExpectExec(q).WithArgs("$argon2id$novo", int64(7), "$2a$12$antigo")
			if c.err != nil {
				exp.WillReturnError(c.err)
			} else {
				exp.WillReturnResult(c.result)
			}

			err := identidademysql.NovoUsuarios(db).RegravarHash(context.Background(), 7, identidade.NovoHashSenha("$2a$12$antigo"), identidade.NovoHashSenha("$argon2id$novo"))
			if c.wantErr {
				assert.ErrorIs(t, err, sql.ErrConnDone)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpdatePasswordHash_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \?, tokens_validos_desde = \? WHERE id = \?`).
		WithArgs("hash", false, sqlmock.AnyArg(), int64(99999)).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 linhas afetadas

	ctx := context.Background()
	err := identidademysql.NovoUsuarios(db).AtualizarSenha(ctx, 99999, identidade.NovoHashSenha("hash"), false)

	assert.ErrorIs(t, err, identidade.ErrUsuarioNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePasswordHash_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \?, tokens_validos_desde = \? WHERE id = \?`).
		WithArgs("hash", false, sqlmock.AnyArg(), int64(1)).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	err := identidademysql.NovoUsuarios(db).AtualizarSenha(ctx, 1, identidade.NovoHashSenha("hash"), false)

	assert.Error(t, err)
	assert.NotErrorIs(t, err, identidade.ErrUsuarioNaoEncontrado, "erro de DB não é ErrNotFound")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUltimoLogin_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	mock.ExpectExec(`UPDATE usuarios SET ultimo_login_at = \? WHERE id = \?`).
		WithArgs(now, int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	err := identidademysql.NovoUsuarios(db).RegistrarLogin(ctx, 3, now)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestList_OrderBy(t *testing.T) {
	testes := []struct {
		nome        string
		orderBy     string
		orderDir    string
		orderRegexp string
	}{
		{"order_by válido asc", "nome", "asc", `ORDER BY u\.nome ASC`},
		{"order_by válido desc", "email", "desc", `ORDER BY u\.email DESC`},
		{"order_by fora da whitelist cai no default", "password_hash", "asc", `ORDER BY u\.id ASC`},
		{"order_by tentando SQL injection cai no default", "1; DROP TABLE usuarios;--", "asc", `ORDER BY u\.id ASC`},
		{"order_dir inválido cai no default (asc)", "role", "invalido", `ORDER BY u\.role ASC`},
		{"tudo vazio cai no default", "", "", `ORDER BY u\.id ASC`},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM usuarios").
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery(`SELECT .+ FROM usuarios .+ `+tt.orderRegexp+` LIMIT \? OFFSET \?`).
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows(baseColumns))

			ctx := context.Background()
			_, _, err := itensETotal(identidademysql.NovoUsuarios(db).Listar(ctx, ordemUsuario(tt.orderBy, tt.orderDir), kernel.NovaPagina(1, 10)))

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestList_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM usuarios").
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, _, err := itensETotal(identidademysql.NovoUsuarios(db).Listar(ctx, ordemUsuario("", ""), kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM usuarios").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))
	// Rows fecha prematuramente simulando erro na iteração.
	mock.ExpectQuery("SELECT .+ FROM usuarios").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(baseColumns).
			AddRow(baseRow(1, "U1", "u1@test.com", "h", "normal", nil, true, time.Now(), time.Now(), nil)...).
			RowError(0, sql.ErrConnDone))

	ctx := context.Background()
	_, _, err := itensETotal(identidademysql.NovoUsuarios(db).Listar(ctx, ordemUsuario("", ""), kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByEmail_ComUltimoLogin(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	ultimoLogin := time.Now().Add(-1 * time.Hour)
	rows := sqlmock.NewRows(baseColumns).
		AddRow(baseRow(2, "User", "user@test.com", "hash", "normal", nil, true, time.Now(), time.Now(), &ultimoLogin)...)

	mock.ExpectQuery("SELECT .+ FROM usuarios u LEFT JOIN vendedores v ON v.id = u.id_vendedor WHERE u.email = ?").
		WithArgs("user@test.com").
		WillReturnRows(rows)

	ctx := context.Background()
	u, err := identidademysql.NovoUsuarios(db).PorEmail(ctx, "user@test.com")

	require.NoError(t, err)
	assert.NotNil(t, u.UltimoLoginEm)
	assert.Equal(t, ultimoLogin.Unix(), u.UltimoLoginEm.Unix())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// InativarByVendedorID
// ---------------------------------------------------------------------------

const inativarPorVendedorRegex = `UPDATE usuarios SET ativo = 0, tokens_validos_desde = \? WHERE id_vendedor = \? AND ativo = 1`

func TestInativarByVendedorID_ComUsuarios(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(inativarPorVendedorRegex).
		WithArgs(sqlmock.AnyArg(), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	n, err := identidademysql.NovoUsuarios(db).InativarDoVendedor(context.Background(), 3)

	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInativarByVendedorID_SemUsuarios_NaoEhErro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(inativarPorVendedorRegex).
		WithArgs(sqlmock.AnyArg(), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	n, err := identidademysql.NovoUsuarios(db).InativarDoVendedor(context.Background(), 3)

	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInativarByVendedorID_ExecError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(inativarPorVendedorRegex).
		WithArgs(sqlmock.AnyArg(), int64(3)).
		WillReturnError(sql.ErrConnDone)

	n, err := identidademysql.NovoUsuarios(db).InativarDoVendedor(context.Background(), 3)

	assert.ErrorIs(t, err, sql.ErrConnDone)
	assert.Equal(t, int64(0), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInativarByVendedorID_RowsAffectedError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(inativarPorVendedorRegex).
		WithArgs(sqlmock.AnyArg(), int64(3)).
		WillReturnResult(sqlmock.NewErrorResult(sql.ErrConnDone))

	_, err := identidademysql.NovoUsuarios(db).InativarDoVendedor(context.Background(), 3)

	assert.ErrorIs(t, err, sql.ErrConnDone)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInativarByVendedorID_DentroDeTransacao(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(inativarPorVendedorRegex).
		WithArgs(sqlmock.AnyArg(), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	n, err := identidademysql.NovoUsuarios(tx).InativarDoVendedor(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	require.NoError(t, tx.Commit())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// OPS-01: ListAtivos filtra ativo = 1, ordena por id e propaga erros.
func TestListAtivos(t *testing.T) {
	const q = `WHERE u\.ativo = 1\s+ORDER BY u\.id ASC`
	ctx := context.Background()

	t.Run("sucesso", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		now := time.Now()
		mock.ExpectQuery(q).WillReturnRows(sqlmock.NewRows(baseColumns).
			AddRow(baseRow(1, "Admin", "admin@test.com", "h1", "admin", nil, true, now, now, nil)...).
			AddRow(baseRow(2, "Ana", "ana@test.com", "h2", "normal", nil, true, now, now, nil)...))

		us, err := identidademysql.NovoUsuarios(db).ListarAtivos(ctx)
		require.NoError(t, err)
		require.Len(t, us, 2)
		assert.Equal(t, "admin@test.com", us[0].Email)
		assert.Equal(t, "h2", us[1].Hash.Revelar())
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("vazio", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectQuery(q).WillReturnRows(sqlmock.NewRows(baseColumns))
		us, err := identidademysql.NovoUsuarios(db).ListarAtivos(ctx)
		require.NoError(t, err)
		assert.Empty(t, us)
	})

	t.Run("erro na query", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectQuery(q).WillReturnError(sql.ErrConnDone)
		_, err := identidademysql.NovoUsuarios(db).ListarAtivos(ctx)
		assert.ErrorIs(t, err, sql.ErrConnDone)
	})

	t.Run("erro no scan", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectQuery(q).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		_, err := identidademysql.NovoUsuarios(db).ListarAtivos(ctx)
		assert.Error(t, err)
	})

	t.Run("erro na iteração", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		now := time.Now()
		mock.ExpectQuery(q).WillReturnRows(sqlmock.NewRows(baseColumns).
			AddRow(baseRow(1, "Admin", "admin@test.com", "h1", "admin", nil, true, now, now, nil)...).
			RowError(0, sql.ErrConnDone))
		_, err := identidademysql.NovoUsuarios(db).ListarAtivos(ctx)
		assert.ErrorIs(t, err, sql.ErrConnDone)
	})
}
