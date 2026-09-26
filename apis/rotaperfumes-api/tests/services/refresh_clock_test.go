package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

// TestNewRefreshTokenServiceWithClock: o relógio injetado decide a
// expiração; now == nil cai no relógio do sistema.
func TestNewRefreshTokenServiceWithClock(t *testing.T) {
	const token = "tok-relogio"
	const findSQL = `SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent, revoked_reason\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`
	expira := time.Now().Add(time.Hour)

	casos := []struct {
		nome    string
		now     func() time.Time
		wantErr error
	}{
		{"nil usa time.Now (ainda válido)", nil, nil},
		{"relógio antes da expiração", func() time.Time { return expira.Add(-time.Minute) }, nil},
		{"relógio depois da expiração", func() time.Time { return expira.Add(time.Minute) }, services.ErrRefreshTokenExpired},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			mock.ExpectQuery(findSQL).WithArgs(hashOf(token)).
				WillReturnRows(sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(1), int64(10), hashOf(token), expira, nil, "ip", "ua", nil))

			rt, err := services.NewRefreshTokenServiceWithClock(c.now).ValidateRefreshToken(context.Background(), db, token)
			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				assert.Nil(t, rt)
			} else {
				require.NoError(t, err)
				require.NotNil(t, rt)
				assert.Equal(t, int64(10), rt.UsuarioID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
