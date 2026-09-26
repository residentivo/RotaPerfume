package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

const revokeCondicionalSQL = `UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE id = \? AND revoked_at IS NULL`

// TestRefreshTokenService_BeginRotation cobre a abertura da rotação (SEC-02):
// begin + UPDATE condicional. 0 linhas → ErrRefreshTokenRevoked (rollback).
func TestRefreshTokenService_BeginRotation(t *testing.T) {
	cases := []struct {
		nome    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr error
		wantAny bool
	}{
		{
			nome: "revoga e mantém a tx aberta",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(revokeCondicionalSQL).WithArgs(sqlmock.AnyArg(), "rotacao", int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectRollback() // Rollback explícito do teste
			},
		},
		{
			nome: "já revogado (0 linhas) retorna ErrRefreshTokenRevoked",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(revokeCondicionalSQL).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
			},
			wantErr: services.ErrRefreshTokenRevoked,
		},
		{
			nome: "erro de banco no UPDATE é propagado",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(revokeCondicionalSQL).WillReturnError(errors.New("db down"))
				mock.ExpectRollback()
			},
			wantAny: true,
		},
		{
			nome: "erro no begin é propagado",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("sem conexão"))
			},
			wantAny: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			tc.setup(mock)

			rot, err := services.NewRefreshTokenService().BeginRotation(context.Background(), db, 7)
			switch {
			case tc.wantErr != nil:
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, rot)
			case tc.wantAny:
				assert.Error(t, err)
				assert.NotErrorIs(t, err, services.ErrRefreshTokenRevoked)
				assert.Nil(t, rot)
			default:
				require.NoError(t, err)
				rot.Rollback()
				rot.Rollback() // idempotente
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRefreshRotation_Issue cobre a emissão do novo token na mesma tx.
func TestRefreshRotation_Issue(t *testing.T) {
	cases := []struct {
		nome    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			nome: "insere e confirma",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WithArgs(int64(3), sqlmock.AnyArg(), sqlmock.AnyArg(), "ip", "ua").
					WillReturnResult(sqlmock.NewResult(11, 1))
				mock.ExpectCommit()
			},
		},
		{
			nome: "falha no insert desfaz a revogação",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnError(errors.New("falha insert"))
				mock.ExpectRollback()
			},
			wantErr: true,
		},
		{
			nome: "falha no commit",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnResult(sqlmock.NewResult(11, 1))
				mock.ExpectCommit().WillReturnError(errors.New("falha commit"))
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			mock.ExpectBegin()
			mock.ExpectExec(revokeCondicionalSQL).WillReturnResult(sqlmock.NewResult(0, 1))
			tc.setup(mock)

			rot, err := services.NewRefreshTokenService().BeginRotation(context.Background(), db, 7)
			require.NoError(t, err)
			tok, err := rot.Issue(context.Background(), 3, "ip", "ua")
			rot.Rollback() // no-op após Issue
			if tc.wantErr {
				assert.Error(t, err)
				assert.Empty(t, tok)
			} else {
				require.NoError(t, err)
				assert.Len(t, tok, services.RefreshTokenBytes*2)
			}

			_, err = rot.Issue(context.Background(), 3, "ip", "ua")
			assert.Error(t, err, "rotação finalizada não pode emitir de novo")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
