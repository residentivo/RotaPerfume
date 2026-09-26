package services_test

// SEC-07: ValidateRefreshToken devolve *RevokedTokenError (ID, UsuarioID,
// motivo, RevokedAt, Recent) para token revogado, preservando errors.Is com
// ErrRefreshTokenRevoked / ErrRefreshTokenRevokedRecently.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/repositories"
)

const findRefreshSEC07 = `SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent, revoked_reason\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`

func TestSEC07_ValidateRefreshToken_RevokedTokenError(t *testing.T) {
	agora := time.Date(2026, 9, 26, 12, 0, 0, 0, time.Local)
	casos := []struct {
		nome       string
		revokedAt  time.Time
		motivo     any
		wantReason repositories.RevokeReason
		wantRecent bool
		wantReuso  bool
	}{
		{"rotacao fora da janela = reuso", agora.Add(-time.Minute), "rotacao", repositories.RevokeReasonRotacao, false, true},
		{"rotacao dentro da janela = corrida", agora.Add(-5 * time.Second), "rotacao", repositories.RevokeReasonRotacao, true, false},
		{"logout fora da janela", agora.Add(-time.Minute), "logout", repositories.RevokeReasonLogout, false, false},
		{"inativacao fora da janela", agora.Add(-time.Minute), "inativacao", repositories.RevokeReasonInativacao, false, false},
		{"senha fora da janela", agora.Add(-time.Minute), "senha", repositories.RevokeReasonSenha, false, false},
		{"revogacao_massa fora da janela", agora.Add(-time.Minute), "revogacao_massa", repositories.RevokeReasonRevogacaoMassa, false, false},
		{"NULL legado fora da janela (conservador)", agora.Add(-time.Minute), nil, repositories.RevokeReasonDesconhecido, false, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			defer db.Close()
			token := "tok-sec07"
			mock.ExpectQuery(findRefreshSEC07).WithArgs(hashOf(token)).
				WillReturnRows(sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(44), int64(9), hashOf(token), agora.Add(time.Hour), c.revokedAt, "ip", "ua", c.motivo))

			svc := services.NewRefreshTokenService()
			services.SetRefreshClockForTest(svc, func() time.Time { return agora })
			rt, err := svc.ValidateRefreshToken(context.Background(), db, token)

			assert.Nil(t, rt)
			var rev *services.RevokedTokenError
			require.True(t, errors.As(err, &rev), "deve devolver *RevokedTokenError")
			assert.Equal(t, int64(44), rev.TokenID)
			assert.Equal(t, int64(9), rev.UsuarioID)
			assert.Equal(t, c.wantReason, rev.Reason)
			assert.True(t, rev.RevokedAt.Equal(c.revokedAt))
			assert.Equal(t, c.wantRecent, rev.Recent)
			assert.Equal(t, c.wantReuso, rev.IsRotationReuse())

			assert.ErrorIs(t, err, services.ErrRefreshTokenRevoked)
			if c.wantRecent {
				assert.ErrorIs(t, err, services.ErrRefreshTokenRevokedRecently)
			} else {
				assert.NotErrorIs(t, err, services.ErrRefreshTokenRevokedRecently)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// BeginRotation grava revoked_reason = "rotacao".
func TestSEC07_BeginRotation_GravaMotivoRotacao(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE id = \? AND revoked_at IS NULL`).
		WithArgs(sqlmock.AnyArg(), "rotacao", int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	rot, err := services.NewRefreshTokenService().BeginRotation(context.Background(), db, 7)
	require.NoError(t, err)
	rot.Rollback()
	assert.NoError(t, mock.ExpectationsWereMet())
}

// RevokeAllUserTokens grava exatamente o motivo informado pelo chamador.
func TestSEC07_RevokeAllUserTokens_GravaMotivoInformado(t *testing.T) {
	for _, motivo := range []repositories.RevokeReason{
		repositories.RevokeReasonSenha,
		repositories.RevokeReasonInativacao,
		repositories.RevokeReasonRevogacaoMassa,
	} {
		t.Run(string(motivo), func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
				WithArgs(sqlmock.AnyArg(), string(motivo), int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))

			err = services.NewRefreshTokenService().RevokeAllUserTokens(context.Background(), db, 3, motivo)
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
