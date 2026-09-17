// Package repositories_test contém testes de integração com banco de dados mockado.
// Cada função é stateless — o sqlmock substitui o driver real.
package repositories_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	u, err := repo.GetByEmail(ctx, db, email)

	require.NoError(t, err)
	assert.NotNil(t, u)
	assert.Equal(t, int64(1), u.ID)
	assert.Equal(t, email, u.Email)
	assert.Equal(t, "admin", u.Role)
	assert.True(t, u.Ativo)
	assert.NoError(t, mock.ExpectationsWereMet(), "todas expectativas do mock devem ser satisfeitas")
}

func TestGetByEmail_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM usuarios u LEFT JOIN vendedores v ON v.id = u.id_vendedor WHERE u.email = ?").
		WithArgs("naoexiste@test.com").
		WillReturnError(sql.ErrNoRows)

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	u, err := repo.GetByEmail(ctx, db, "naoexiste@test.com")

	assert.Nil(t, u)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	u, err := repo.GetByID(ctx, db, id)

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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	u, err := repo.GetByID(ctx, db, 99999)

	assert.Nil(t, u)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetIDVendedorByUsuarioID_ComVinculo(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	idVendedor, err := repo.GetIDVendedorByUsuarioID(ctx, db, 5)

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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	idVendedor, err := repo.GetIDVendedorByUsuarioID(ctx, db, 5)

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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	idVendedor, err := repo.GetIDVendedorByUsuarioID(ctx, db, 999)

	assert.Nil(t, idVendedor)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetIDVendedorByUsuarioID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	idVendedor, err := repo.GetIDVendedorByUsuarioID(ctx, db, 5)

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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	usuarios, total, err := repo.List(ctx, db, 1, 10, "", "")

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

	repo2 := repositories.NewUsuarioRepository()
	usuarios2, total2, err2 := repo2.List(context.Background(), db2, 2, 10, "", "")
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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 0, 0, "", "") // valores inválidos devem ser corrigidos

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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 200, "", "")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePasswordHash_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
		WithArgs("novo-hash-bcrypt", false, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	err := repo.UpdatePasswordHash(ctx, db, 7, "novo-hash-bcrypt", false)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePasswordHash_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
		WithArgs("hash", false, int64(99999)).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 linhas afetadas

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	err := repo.UpdatePasswordHash(ctx, db, 99999, "hash", false)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePasswordHash_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
		WithArgs("hash", false, int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	err := repo.UpdatePasswordHash(ctx, db, 1, "hash", false)

	assert.Error(t, err)
	assert.NotErrorIs(t, err, repositories.ErrNotFound, "erro de DB não é ErrNotFound")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUltimoLogin_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	mock.ExpectExec(`UPDATE usuarios SET ultimo_login_at = \? WHERE id = \?`).
		WithArgs(now, int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	err := repo.UpdateUltimoLogin(ctx, db, 3, now)

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
			mock.ExpectQuery(`SELECT .+ FROM usuarios .+ ` + tt.orderRegexp + ` LIMIT \? OFFSET \?`).
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows(baseColumns))

			repo := repositories.NewUsuarioRepository()
			ctx := context.Background()
			_, _, err := repo.List(ctx, db, 1, 10, tt.orderBy, tt.orderDir)

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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, "", "")

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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, "", "")

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

	repo := repositories.NewUsuarioRepository()
	ctx := context.Background()
	u, err := repo.GetByEmail(ctx, db, "user@test.com")

	require.NoError(t, err)
	assert.NotNil(t, u.UltimoLoginAt)
	assert.Equal(t, ultimoLogin.Unix(), u.UltimoLoginAt.Unix())
	assert.NoError(t, mock.ExpectationsWereMet())
}
