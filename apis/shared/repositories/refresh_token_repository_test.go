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

var refreshTokenColumns = []string{
	"id", "usuario_id", "token_hash", "expires_at", "revoked_at", "ip_origem", "user_agent",
}

func refreshTokenRow(id, usuarioID int64, tokenHash string, expiresAt time.Time, revokedAt *time.Time, ip, userAgent string) []driver.Value {
	var ra driver.Value = nil
	if revokedAt != nil {
		ra = *revokedAt
	}
	return []driver.Value{id, usuarioID, tokenHash, expiresAt, ra, ip, userAgent}
}

func TestRefreshTokenCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rt := &repositories.RefreshToken{
		UsuarioID: 1,
		TokenHash: "hash-abc",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		IPOrigem:  "127.0.0.1",
		UserAgent: "go-test",
	}

	mock.ExpectExec("INSERT INTO refresh_tokens").
		WithArgs(rt.UsuarioID, rt.TokenHash, rt.ExpiresAt, rt.IPOrigem, rt.UserAgent).
		WillReturnResult(sqlmock.NewResult(5, 1))

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, rt)

	require.NoError(t, err)
	assert.Equal(t, int64(5), rt.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenCreate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rt := &repositories.RefreshToken{}
	mock.ExpectExec("INSERT INTO refresh_tokens").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, rt)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByTokenHash_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(refreshTokenColumns).
		AddRow(refreshTokenRow(1, 1, "hash-abc", now.Add(24*time.Hour), nil, "127.0.0.1", "go-test")...)

	mock.ExpectQuery(`SELECT .+ FROM refresh_tokens WHERE token_hash = \? LIMIT 1`).
		WithArgs("hash-abc").
		WillReturnRows(rows)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	rt, err := repo.FindByTokenHash(ctx, db, "hash-abc")

	require.NoError(t, err)
	assert.Equal(t, int64(1), rt.ID)
	assert.False(t, rt.RevokedAt.Valid)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByTokenHash_ComRevoked(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	revoked := now.Add(-1 * time.Hour)
	rows := sqlmock.NewRows(refreshTokenColumns).
		AddRow(refreshTokenRow(2, 1, "hash-def", now.Add(24*time.Hour), &revoked, "127.0.0.1", "go-test")...)

	mock.ExpectQuery(`SELECT .+ FROM refresh_tokens WHERE token_hash = \? LIMIT 1`).
		WithArgs("hash-def").
		WillReturnRows(rows)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	rt, err := repo.FindByTokenHash(ctx, db, "hash-def")

	require.NoError(t, err)
	assert.True(t, rt.RevokedAt.Valid)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByTokenHash_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ FROM refresh_tokens WHERE token_hash = \? LIMIT 1`).
		WithArgs("naoexiste").
		WillReturnError(sql.ErrNoRows)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	rt, err := repo.FindByTokenHash(ctx, db, "naoexiste")

	assert.Nil(t, rt)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByTokenHash_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ FROM refresh_tokens WHERE token_hash = \? LIMIT 1`).
		WithArgs("hash").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	rt, err := repo.FindByTokenHash(ctx, db, "hash")

	assert.Nil(t, rt)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenRevoke_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	err := repo.Revoke(ctx, db, 1)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenRevoke_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	err := repo.Revoke(ctx, db, 999)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenRevoke_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	err := repo.Revoke(ctx, db, 1)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenRevokeAllByUser_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 3))

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	err := repo.RevokeAllByUser(ctx, db, 1)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenRevokeAllByUser_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	err := repo.RevokeAllByUser(ctx, db, 1)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenDeleteExpired_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM refresh_tokens`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 4))

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	n, err := repo.DeleteExpired(ctx, db)

	require.NoError(t, err)
	assert.Equal(t, int64(4), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenDeleteExpired_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM refresh_tokens`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	_, err := repo.DeleteExpired(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByUsuario_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM refresh_tokens WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(refreshTokenColumns).
		AddRow(refreshTokenRow(2, 1, "hash-2", now, nil, "ip", "ua")...).
		AddRow(refreshTokenRow(1, 1, "hash-1", now, nil, "ip", "ua")...)

	mock.ExpectQuery(`SELECT .+ FROM refresh_tokens WHERE usuario_id = \? ORDER BY id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(1), 10, 0).
		WillReturnRows(rows)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	tokens, total, err := repo.FindByUsuario(ctx, db, 1, 1, 10)

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, tokens, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByUsuario_Defaults(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM refresh_tokens WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ FROM refresh_tokens WHERE usuario_id = \? ORDER BY id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(1), 20, 0).
		WillReturnRows(sqlmock.NewRows(refreshTokenColumns))

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	_, _, err := repo.FindByUsuario(ctx, db, 1, 0, 0)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByUsuario_LimitCap(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM refresh_tokens WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ FROM refresh_tokens WHERE usuario_id = \? ORDER BY id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(1), 100, 0).
		WillReturnRows(sqlmock.NewRows(refreshTokenColumns))

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	_, _, err := repo.FindByUsuario(ctx, db, 1, 1, 200)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByUsuario_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM refresh_tokens WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	_, _, err := repo.FindByUsuario(ctx, db, 1, 1, 10)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByUsuario_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM refresh_tokens WHERE usuario_id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ FROM refresh_tokens WHERE usuario_id = \? ORDER BY id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(1), 10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	_, _, err := repo.FindByUsuario(ctx, db, 1, 1, 10)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
