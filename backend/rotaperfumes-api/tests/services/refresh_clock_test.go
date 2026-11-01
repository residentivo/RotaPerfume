package services_test

import (
	"github.com/rotaperfumes/shared/identidade"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{"relógio depois da expiração", func() time.Time { return expira.Add(time.Minute) }, identidade.ErrRefreshExpirado},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			mock.ExpectQuery(findSQL).WithArgs(hashOf(token)).
				WillReturnRows(sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(1), int64(10), hashOf(token), expira, nil, "ip", "ua", nil))

			relogio := c.now
			if relogio == nil {
				relogio = time.Now // ARQ-07: a montagem de produção liga time.Now (Relogio nil é panic)
			}
			rt, estado, err := buscarEAvaliar(db, token, relogio())
			require.NoError(t, err)
			if c.wantErr != nil {
				assert.Equal(t, identidade.RefreshExpirado, estado, "o relógio injetado decide a expiração")
			} else {
				assert.Equal(t, identidade.RefreshValido, estado)
				require.NoError(t, err)
				require.NotNil(t, rt)
				assert.Equal(t, int64(10), rt.UsuarioID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
