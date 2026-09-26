package repositories_test

// SEC-06: UsuarioRepository.GetStatusByID (checagem por request no
// middleware) e RefreshTokenRepository.RevokeAllByVendedorID (revogação na
// transação de desligamento do vendedor).

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

const (
	reGetStatusUsuario  = `SELECT ativo, role FROM usuarios WHERE id = \? LIMIT 1`
	reRevokePorVendedor = `UPDATE refresh_tokens rt\s+JOIN usuarios u ON u\.id = rt\.usuario_id\s+SET rt\.revoked_at = \?, rt\.revoked_reason = \?\s+WHERE u\.id_vendedor = \? AND rt\.revoked_at IS NULL`
)

func TestUsuarioGetStatusByID(t *testing.T) {
	casos := []struct {
		nome    string
		expect  func(m sqlmock.Sqlmock)
		want    *repositories.UsuarioStatus
		wantErr error
	}{
		{
			nome: "ativo admin",
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reGetStatusUsuario).WithArgs(int64(4)).
					WillReturnRows(sqlmock.NewRows([]string{"ativo", "role"}).AddRow(true, "admin"))
			},
			want: &repositories.UsuarioStatus{Ativo: true, Role: "admin"},
		},
		{
			nome: "inativo normal",
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reGetStatusUsuario).WithArgs(int64(4)).
					WillReturnRows(sqlmock.NewRows([]string{"ativo", "role"}).AddRow(false, "normal"))
			},
			want: &repositories.UsuarioStatus{Ativo: false, Role: "normal"},
		},
		{
			nome: "inexistente → ErrNotFound",
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reGetStatusUsuario).WithArgs(int64(4)).
					WillReturnRows(sqlmock.NewRows([]string{"ativo", "role"}))
			},
			wantErr: repositories.ErrNotFound,
		},
		{
			nome: "erro de banco repassado",
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reGetStatusUsuario).WithArgs(int64(4)).WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			c.expect(mock)

			st, err := repositories.NewUsuarioRepository().GetStatusByID(context.Background(), db, 4)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				assert.Nil(t, st)
			} else {
				require.NoError(t, err)
				assert.Equal(t, c.want, st)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRefreshTokenRevokeAllByVendedorID(t *testing.T) {
	t.Run("revoga e devolve quantidade", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectExec(reRevokePorVendedor).WithArgs(sqlmock.AnyArg(), "inativacao", int64(9)).
			WillReturnResult(sqlmock.NewResult(0, 3))

		n, err := repositories.NewRefreshTokenRepository().RevokeAllByVendedorID(context.Background(), db, 9, repositories.RevokeReasonInativacao)

		require.NoError(t, err)
		assert.Equal(t, int64(3), n)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("idempotente: 0 linhas não é erro", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectExec(reRevokePorVendedor).WithArgs(sqlmock.AnyArg(), "inativacao", int64(9)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		n, err := repositories.NewRefreshTokenRepository().RevokeAllByVendedorID(context.Background(), db, 9, repositories.RevokeReasonInativacao)

		require.NoError(t, err)
		assert.Equal(t, int64(0), n)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("dentro de transação (Execer = *sql.Tx)", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectBegin()
		mock.ExpectExec(reRevokePorVendedor).WithArgs(sqlmock.AnyArg(), "inativacao", int64(9)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		tx, err := db.Begin()
		require.NoError(t, err)
		_, err = repositories.NewRefreshTokenRepository().RevokeAllByVendedorID(context.Background(), tx, 9, repositories.RevokeReasonInativacao)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro de banco repassado", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		errDB := errors.New("falha")
		mock.ExpectExec(reRevokePorVendedor).WithArgs(sqlmock.AnyArg(), "inativacao", int64(9)).WillReturnError(errDB)

		_, err := repositories.NewRefreshTokenRepository().RevokeAllByVendedorID(context.Background(), db, 9, repositories.RevokeReasonInativacao)

		assert.ErrorIs(t, err, errDB)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro em RowsAffected repassado", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		errRows := errors.New("rows affected indisponível")
		mock.ExpectExec(reRevokePorVendedor).WithArgs(sqlmock.AnyArg(), "inativacao", int64(9)).
			WillReturnResult(sqlmock.NewErrorResult(errRows))

		n, err := repositories.NewRefreshTokenRepository().RevokeAllByVendedorID(context.Background(), db, 9, repositories.RevokeReasonInativacao)

		assert.ErrorIs(t, err, errRows)
		assert.Zero(t, n)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// SEC-07: FindByTokenHash lê revoked_reason; NULL vira RevokeReasonDesconhecido.
func TestRefreshTokenFindByTokenHash_LeMotivoDaRevogacao(t *testing.T) {
	casos := []struct {
		nome   string
		motivo any
		want   repositories.RevokeReason
	}{
		{"rotacao", "rotacao", repositories.RevokeReasonRotacao},
		{"logout", "logout", repositories.RevokeReasonLogout},
		{"inativacao", "inativacao", repositories.RevokeReasonInativacao},
		{"NULL", nil, repositories.RevokeReasonDesconhecido},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			agora := time.Now()
			mock.ExpectQuery(`SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent, revoked_reason\s+FROM refresh_tokens\s+WHERE token_hash = \?`).
				WithArgs("h").
				WillReturnRows(sqlmock.NewRows(refreshTokenColumns).
					AddRow(int64(1), int64(2), "h", agora.Add(time.Hour), agora, "ip", "ua", c.motivo))

			rt, err := repositories.NewRefreshTokenRepository().FindByTokenHash(context.Background(), db, "h")

			require.NoError(t, err)
			assert.Equal(t, c.want, rt.RevokedReason)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
