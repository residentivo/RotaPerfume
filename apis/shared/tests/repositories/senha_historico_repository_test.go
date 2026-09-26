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

var senhaHistoricoColumns = []string{
	"id", "usuario_id", "resetado_por_id", "senha_hash_anterior", "ip_origem", "user_agent", "tipo_reset", "created_at",
	"usuario_nome", "resetado_por_nome",
}

func senhaHistoricoRow(id, usuarioID int64, resetadoPorID *int64, hashAnterior, ip, userAgent, tipoReset string, created time.Time) []driver.Value {
	return senhaHistoricoRowComNomes(id, usuarioID, resetadoPorID, hashAnterior, ip, userAgent, tipoReset, created, "Usuario Teste", nil)
}

// senhaHistoricoRowComNomes monta uma linha completa, incluindo os nomes
// vindos dos JOINs com a tabela usuarios (usuario_nome sempre presente;
// resetado_por_nome apenas quando resetadoPorID/resetadoPorNome != nil).
func senhaHistoricoRowComNomes(id, usuarioID int64, resetadoPorID *int64, hashAnterior, ip, userAgent, tipoReset string, created time.Time, usuarioNome string, resetadoPorNome *string) []driver.Value {
	var rp driver.Value = nil
	if resetadoPorID != nil {
		rp = *resetadoPorID
	}
	var rpNome driver.Value = nil
	if resetadoPorNome != nil {
		rpNome = *resetadoPorNome
	}
	return []driver.Value{id, usuarioID, rp, hashAnterior, ip, userAgent, tipoReset, created, usuarioNome, rpNome}
}

func TestSenhaHistoricoCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	h := &repositories.SenhaHistorico{
		UsuarioID:         1,
		SenhaHashAnterior: "hash-antiga",
		IPOrigem:          "127.0.0.1",
		UserAgent:         "go-test",
		TipoReset:         "usuario",
	}

	mock.ExpectExec("INSERT INTO senha_historico").
		WithArgs(h.UsuarioID, nil, h.SenhaHashAnterior, h.IPOrigem, h.UserAgent, h.TipoReset).
		WillReturnResult(sqlmock.NewResult(3, 1))

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, h)

	require.NoError(t, err)
	assert.Equal(t, int64(3), h.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoCreate_ComResetadoPor(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	h := &repositories.SenhaHistorico{
		UsuarioID:         1,
		ResetadoPorID:     sql.NullInt64{Int64: 2, Valid: true},
		SenhaHashAnterior: "hash-antiga",
		IPOrigem:          "127.0.0.1",
		UserAgent:         "go-test",
		TipoReset:         "admin",
	}

	mock.ExpectExec("INSERT INTO senha_historico").
		WithArgs(h.UsuarioID, int64(2), h.SenhaHashAnterior, h.IPOrigem, h.UserAgent, h.TipoReset).
		WillReturnResult(sqlmock.NewResult(4, 1))

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, h)

	require.NoError(t, err)
	assert.Equal(t, int64(4), h.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoCreate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	h := &repositories.SenhaHistorico{}
	mock.ExpectExec("INSERT INTO senha_historico").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, h)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindByUsuario_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))

	now := time.Now()
	rows := sqlmock.NewRows(senhaHistoricoColumns).
		AddRow(senhaHistoricoRow(1, 1, nil, "hash", "ip", "ua", "usuario", now)...)

	mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id WHERE sh\.usuario_id = \? ORDER BY sh\.id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(1), 10, 0).
		WillReturnRows(rows)

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	historico, total, err := repo.FindByUsuario(ctx, db, 1, 1, 10, "", "")

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, historico, 1)
	assert.False(t, historico[0].ResetadoPorID.Valid)
	assert.False(t, historico[0].ResetadoPorNome.Valid, "resetado_por_nome deve ser NULL quando resetado_por_id é NULL")
	assert.Equal(t, "Usuario Teste", historico[0].UsuarioNome, "usuario_nome deve vir preenchido pelo JOIN mesmo sem reset por terceiros")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindByUsuario_ComResetadoPor(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))

	now := time.Now()
	resetadoPor := int64(2)
	resetadoPorNome := "Admin Teste"
	rows := sqlmock.NewRows(senhaHistoricoColumns).
		AddRow(senhaHistoricoRowComNomes(1, 1, &resetadoPor, "hash", "ip", "ua", "admin", now, "Usuario Teste", &resetadoPorNome)...)

	mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id WHERE sh\.usuario_id = \? ORDER BY sh\.id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(1), 10, 0).
		WillReturnRows(rows)

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	historico, _, err := repo.FindByUsuario(ctx, db, 1, 1, 10, "", "")

	require.NoError(t, err)
	require.Len(t, historico, 1)
	assert.True(t, historico[0].ResetadoPorID.Valid)
	assert.Equal(t, int64(2), historico[0].ResetadoPorID.Int64)
	assert.Equal(t, "Usuario Teste", historico[0].UsuarioNome)
	require.True(t, historico[0].ResetadoPorNome.Valid, "resetado_por_nome deve ser preenchido quando resetado_por_id existe")
	assert.Equal(t, "Admin Teste", historico[0].ResetadoPorNome.String)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindByUsuario_OrderBy(t *testing.T) {
	testes := []struct {
		nome        string
		orderBy     string
		orderDir    string
		orderRegexp string
	}{
		{"order_by válido asc", "tipo_reset", "asc", `ORDER BY sh\.tipo_reset ASC`},
		{"order_by válido desc", "usuario_id", "desc", `ORDER BY sh\.usuario_id DESC`},
		{"order_by fora da whitelist cai no default", "1; DROP TABLE senha_historico;--", "asc", `ORDER BY sh\.id ASC`},
		{"order_dir inválido cai no default (desc)", "tipo_reset", "invalido", `ORDER BY sh\.tipo_reset DESC`},
		{"tudo vazio cai no default", "", "", `ORDER BY sh\.id DESC`},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
				WithArgs(int64(1)).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id WHERE sh\.usuario_id = \? `+tt.orderRegexp+` LIMIT \? OFFSET \?`).
				WithArgs(int64(1), 10, 0).
				WillReturnRows(sqlmock.NewRows(senhaHistoricoColumns))

			repo := repositories.NewSenhaHistoricoRepository()
			ctx := context.Background()
			_, _, err := repo.FindByUsuario(ctx, db, 1, 1, 10, tt.orderBy, tt.orderDir)

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSenhaHistoricoFindByUsuario_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	_, _, err := repo.FindByUsuario(ctx, db, 1, 1, 10, "", "")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindByUsuario_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id WHERE sh\.usuario_id = \? ORDER BY sh\.id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(1), 10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	_, _, err := repo.FindByUsuario(ctx, db, 1, 1, 10, "", "")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindByUsuario_Defaults(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id WHERE sh\.usuario_id = \? ORDER BY sh\.id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(1), 20, 0).
		WillReturnRows(sqlmock.NewRows(senhaHistoricoColumns))

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	_, _, err := repo.FindByUsuario(ctx, db, 1, 0, 0, "", "")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindByUsuario_LimitCap(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id WHERE sh\.usuario_id = \? ORDER BY sh\.id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(1), 100, 0).
		WillReturnRows(sqlmock.NewRows(senhaHistoricoColumns))

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	_, _, err := repo.FindByUsuario(ctx, db, 1, 1, 200, "", "")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindAll_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico$`).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(senhaHistoricoColumns).
		AddRow(senhaHistoricoRow(2, 1, nil, "hash2", "ip", "ua", "usuario", now)...).
		AddRow(senhaHistoricoRow(1, 1, nil, "hash1", "ip", "ua", "usuario", now)...)

	mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id ORDER BY sh\.id DESC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnRows(rows)

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	historico, total, err := repo.FindAll(ctx, db, 1, 10, "", "")

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, historico, 2)
	for _, h := range historico {
		assert.Equal(t, "Usuario Teste", h.UsuarioNome)
		assert.False(t, h.ResetadoPorNome.Valid)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindAll_ComResetadoPor(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico$`).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))

	now := time.Now()
	resetadoPor := int64(2)
	resetadoPorNome := "Admin Teste"
	rows := sqlmock.NewRows(senhaHistoricoColumns).
		AddRow(senhaHistoricoRowComNomes(1, 1, &resetadoPor, "hash", "ip", "ua", "admin", now, "Usuario Teste", &resetadoPorNome)...)

	mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id ORDER BY sh\.id DESC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnRows(rows)

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	historico, _, err := repo.FindAll(ctx, db, 1, 10, "", "")

	require.NoError(t, err)
	require.Len(t, historico, 1)
	assert.Equal(t, "Usuario Teste", historico[0].UsuarioNome)
	require.True(t, historico[0].ResetadoPorNome.Valid)
	assert.Equal(t, "Admin Teste", historico[0].ResetadoPorNome.String)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindAll_OrderBy(t *testing.T) {
	testes := []struct {
		nome        string
		orderBy     string
		orderDir    string
		orderRegexp string
	}{
		{"order_by válido asc", "usuario_id", "asc", `ORDER BY sh\.usuario_id ASC`},
		{"order_by fora da whitelist cai no default", "1; DROP TABLE senha_historico;--", "asc", `ORDER BY sh\.id ASC`},
		{"order_dir inválido cai no default (desc)", "id", "invalido", `ORDER BY sh\.id DESC`},
		{"tudo vazio cai no default", "", "", `ORDER BY sh\.id DESC`},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico$`).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id `+tt.orderRegexp+` LIMIT \? OFFSET \?`).
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows(senhaHistoricoColumns))

			repo := repositories.NewSenhaHistoricoRepository()
			ctx := context.Background()
			_, _, err := repo.FindAll(ctx, db, 1, 10, tt.orderBy, tt.orderDir)

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSenhaHistoricoFindAll_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico$`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	_, _, err := repo.FindAll(ctx, db, 1, 10, "", "")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindAll_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico$`).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id ORDER BY sh\.id DESC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	_, _, err := repo.FindAll(ctx, db, 1, 10, "", "")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSenhaHistoricoFindAll_Defaults(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico$`).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ FROM senha_historico sh LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id ORDER BY sh\.id DESC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows(senhaHistoricoColumns))

	repo := repositories.NewSenhaHistoricoRepository()
	ctx := context.Background()
	_, _, err := repo.FindAll(ctx, db, 0, 0, "", "")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
