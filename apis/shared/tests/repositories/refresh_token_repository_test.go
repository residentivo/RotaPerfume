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

var refreshTokenColumns = []string{
	"id", "usuario_id", "token_hash", "expires_at", "revoked_at", "ip_origem", "user_agent", "revoked_reason",
}

func refreshTokenRow(id, usuarioID int64, tokenHash string, expiresAt time.Time, revokedAt *time.Time, ip, userAgent string) []driver.Value {
	var ra driver.Value = nil
	if revokedAt != nil {
		ra = *revokedAt
	}
	return []driver.Value{id, usuarioID, tokenHash, expiresAt, ra, ip, userAgent, nil}
}

func TestRefreshTokenCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rt := &identidade.RefreshToken{
		UsuarioID: 1,
		HashToken: "hash-abc",
		ExpiraEm:  time.Now().Add(24 * time.Hour),
		IP:        "127.0.0.1",
		UserAgent: "go-test",
	}

	mock.ExpectExec("INSERT INTO refresh_tokens").
		WithArgs(rt.UsuarioID, rt.HashToken, rt.ExpiraEm, rt.IP, rt.UserAgent).
		WillReturnResult(sqlmock.NewResult(5, 1))

	ctx := context.Background()
	err := identidademysql.NovoRefreshTokens(db).Inserir(ctx, rt)

	require.NoError(t, err)
	assert.Equal(t, int64(5), rt.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenCreate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rt := &identidade.RefreshToken{}
	mock.ExpectExec("INSERT INTO refresh_tokens").
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	err := identidademysql.NovoRefreshTokens(db).Inserir(ctx, rt)

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

	ctx := context.Background()
	rt, err := identidademysql.NovoRefreshTokens(db).PorHash(ctx, "hash-abc")

	require.NoError(t, err)
	assert.Equal(t, int64(1), rt.ID)
	assert.Nil(t, rt.RevogadoEm)
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

	ctx := context.Background()
	rt, err := identidademysql.NovoRefreshTokens(db).PorHash(ctx, "hash-def")

	require.NoError(t, err)
	assert.NotNil(t, rt.RevogadoEm)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByTokenHash_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ FROM refresh_tokens WHERE token_hash = \? LIMIT 1`).
		WithArgs("naoexiste").
		WillReturnError(sql.ErrNoRows)

	ctx := context.Background()
	rt, err := identidademysql.NovoRefreshTokens(db).PorHash(ctx, "naoexiste")

	assert.Nil(t, rt)
	assert.ErrorIs(t, err, identidade.ErrRefreshInvalido)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByTokenHash_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ FROM refresh_tokens WHERE token_hash = \? LIMIT 1`).
		WithArgs("hash").
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	rt, err := identidademysql.NovoRefreshTokens(db).PorHash(ctx, "hash")

	assert.Nil(t, rt)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, identidade.ErrRefreshInvalido)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenRevoke_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE id = \? AND revoked_at IS NULL`).
		WithArgs(sqlmock.AnyArg(), "rotacao", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	err := identidademysql.NovoRefreshTokens(db).Revogar(ctx, 1, identidade.MotivoRotacao)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenRevoke_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE id = \? AND revoked_at IS NULL`).
		WithArgs(sqlmock.AnyArg(), "rotacao", int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	ctx := context.Background()
	err := identidademysql.NovoRefreshTokens(db).Revogar(ctx, 999, identidade.MotivoRotacao)

	assert.ErrorIs(t, err, identidade.ErrRefreshRevogado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenRevoke_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE id = \? AND revoked_at IS NULL`).
		WithArgs(sqlmock.AnyArg(), "rotacao", int64(1)).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	err := identidademysql.NovoRefreshTokens(db).Revogar(ctx, 1, identidade.MotivoRotacao)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenRevokeAllByUser_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
		WithArgs(sqlmock.AnyArg(), "revogacao_massa", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 3))

	ctx := context.Background()
	err := identidademysql.NovoRefreshTokens(db).RevogarTodosDoUsuario(ctx, 1, identidade.MotivoRevogacaoMassa)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenRevokeAllByUser_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
		WithArgs(sqlmock.AnyArg(), "revogacao_massa", int64(1)).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	err := identidademysql.NovoRefreshTokens(db).RevogarTodosDoUsuario(ctx, 1, identidade.MotivoRevogacaoMassa)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenDeleteExpired_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	corte := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec(`DELETE FROM refresh_tokens WHERE expires_at < \? ORDER BY id LIMIT \?`).
		WithArgs(corte, 1000).
		WillReturnResult(sqlmock.NewResult(0, 4))

	ctx := context.Background()
	n, err := identidademysql.NovoRefreshTokens(db).ApagarExpirados(ctx, corte, 1000)

	require.NoError(t, err)
	assert.Equal(t, int64(4), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenDeleteExpired_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM refresh_tokens`).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, err := identidademysql.NovoRefreshTokens(db).ApagarExpirados(ctx, time.Now(), 1000)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ARQ-07: RefreshTokenRepository.FindByUsuario (listagem paginada por usuário,
// sem chamador de produção desde o ARQ-03) saiu com o legado. Os testes, com
// os mesmos nomes, exercitam a listagem paginada por usuário que existe em
// Identidade, HistoricoSenhas.DoUsuario: o mesmo desenho (COUNT por
// usuario_id + página com LIMIT/OFFSET), os mesmos padrões de página (page
// < 1 → 1, limit < 1 → 20, limit > 100 → 100, kernel.NovaPagina) e os mesmos
// ramos de erro.
const (
	reContaHistoricoUsuario = `SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`
	reListaHistoricoUsuario = `SELECT .+ FROM senha_historico sh .+ WHERE sh\.usuario_id = \? ORDER BY sh\.id DESC LIMIT \? OFFSET \?`
)

func TestRefreshTokenFindByUsuario_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(reContaHistoricoUsuario).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(senhaHistoricoColumns).
		AddRow(senhaHistoricoRow(2, 1, nil, "hash-2", "ip", "ua", "usuario", now)...).
		AddRow(senhaHistoricoRow(1, 1, nil, "hash-1", "ip", "ua", "usuario", now)...)

	mock.ExpectQuery(reListaHistoricoUsuario).
		WithArgs(int64(1), 10, 0).
		WillReturnRows(rows)

	ctx := context.Background()
	tokens, total, err := itensETotal(identidademysql.NovoHistoricoSenhas(db).DoUsuario(ctx, 1, ordemHistorico("", ""), kernel.NovaPagina(1, 10)))

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, tokens, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByUsuario_Defaults(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(reContaHistoricoUsuario).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(reListaHistoricoUsuario).
		WithArgs(int64(1), 20, 0).
		WillReturnRows(sqlmock.NewRows(senhaHistoricoColumns))

	ctx := context.Background()
	_, _, err := itensETotal(identidademysql.NovoHistoricoSenhas(db).DoUsuario(ctx, 1, ordemHistorico("", ""), kernel.NovaPagina(0, 0)))

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByUsuario_LimitCap(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(reContaHistoricoUsuario).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(reListaHistoricoUsuario).
		WithArgs(int64(1), 100, 0).
		WillReturnRows(sqlmock.NewRows(senhaHistoricoColumns))

	ctx := context.Background()
	_, _, err := itensETotal(identidademysql.NovoHistoricoSenhas(db).DoUsuario(ctx, 1, ordemHistorico("", ""), kernel.NovaPagina(1, 200)))

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByUsuario_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(reContaHistoricoUsuario).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, _, err := itensETotal(identidademysql.NovoHistoricoSenhas(db).DoUsuario(ctx, 1, ordemHistorico("", ""), kernel.NovaPagina(1, 10)))

	assert.ErrorIs(t, err, sql.ErrConnDone)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenFindByUsuario_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(reContaHistoricoUsuario).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(reListaHistoricoUsuario).
		WithArgs(int64(1), 10, 0).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, _, err := itensETotal(identidademysql.NovoHistoricoSenhas(db).DoUsuario(ctx, 1, ordemHistorico("", ""), kernel.NovaPagina(1, 10)))

	assert.ErrorIs(t, err, sql.ErrConnDone)
	assert.NoError(t, mock.ExpectationsWereMet())
}
