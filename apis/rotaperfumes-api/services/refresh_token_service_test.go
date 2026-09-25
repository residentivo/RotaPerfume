package services_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

func newRefreshTokenTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func refreshTokenColunas() []string {
	return []string{"id", "usuario_id", "token_hash", "expires_at", "revoked_at", "ip_origem", "user_agent"}
}

func hashOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// GenerateRefreshToken
// ---------------------------------------------------------------------------

func TestRefreshTokenService_GenerateRefreshToken(t *testing.T) {
	t.Run("sucesso - cria token e retorna texto puro", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		mock.ExpectExec(`INSERT INTO refresh_tokens \(usuario_id, token_hash, expires_at, ip_origem, user_agent\)`).
			WithArgs(int64(1), sqlmock.AnyArg(), sqlmock.AnyArg(), "127.0.0.1", "curl/8.0").
			WillReturnResult(sqlmock.NewResult(1, 1))

		svc := services.NewRefreshTokenService()
		token, err := svc.GenerateRefreshToken(context.Background(), db, 1, "127.0.0.1", "curl/8.0")
		require.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.Len(t, token, services.RefreshTokenBytes*2) // hex-encoded
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no repo é propagado", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		mock.ExpectExec(`INSERT INTO refresh_tokens \(usuario_id, token_hash, expires_at, ip_origem, user_agent\)`).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewRefreshTokenService()
		token, err := svc.GenerateRefreshToken(context.Background(), db, 1, "127.0.0.1", "curl/8.0")
		assert.Error(t, err)
		assert.Empty(t, token)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// ValidateRefreshToken
// ---------------------------------------------------------------------------

func TestRefreshTokenService_ValidateRefreshToken(t *testing.T) {
	token := "token-em-texto-puro"

	testCases := []struct {
		nome    string
		mock    func(mock sqlmock.Sqlmock)
		wantErr error
	}{
		{
			nome: "sucesso - token válido não expirado nem revogado",
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(1), int64(10), hashOf(token), time.Now().Add(1*time.Hour), nil, "127.0.0.1", "curl/8.0")
				mock.ExpectQuery(`SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`).
					WithArgs(hashOf(token)).
					WillReturnRows(rows)
			},
		},
		{
			nome: "não encontrado retorna ErrRefreshTokenNotFound",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`).
					WithArgs(hashOf(token)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: services.ErrRefreshTokenNotFound,
		},
		{
			nome: "expirado retorna ErrRefreshTokenExpired",
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(1), int64(10), hashOf(token), time.Now().Add(-1*time.Hour), nil, "127.0.0.1", "curl/8.0")
				mock.ExpectQuery(`SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`).
					WithArgs(hashOf(token)).
					WillReturnRows(rows)
			},
			wantErr: services.ErrRefreshTokenExpired,
		},
		{
			nome: "revogado retorna ErrRefreshTokenRevoked",
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(1), int64(10), hashOf(token), time.Now().Add(1*time.Hour), time.Now(), "127.0.0.1", "curl/8.0")
				mock.ExpectQuery(`SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`).
					WithArgs(hashOf(token)).
					WillReturnRows(rows)
			},
			wantErr: services.ErrRefreshTokenRevoked,
		},
		{
			nome: "erro genérico do repo é propagado",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`).
					WithArgs(hashOf(token)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: nil, // erro genérico, checado via assert.Error apenas
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			tc.mock(mock)

			svc := services.NewRefreshTokenService()
			rt, err := svc.ValidateRefreshToken(context.Background(), db, token)

			if tc.nome == "sucesso - token válido não expirado nem revogado" {
				require.NoError(t, err)
				require.NotNil(t, rt)
				assert.Equal(t, int64(1), rt.ID)
			} else if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, rt)
			} else {
				assert.Error(t, err)
				assert.Nil(t, rt)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// RevokeToken
// ---------------------------------------------------------------------------

func TestRefreshTokenService_RevokeToken(t *testing.T) {
	token := "token-em-texto-puro"

	t.Run("sucesso", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		rows := sqlmock.NewRows(refreshTokenColunas()).
			AddRow(int64(1), int64(10), hashOf(token), time.Now().Add(1*time.Hour), nil, "127.0.0.1", "curl/8.0")
		mock.ExpectQuery(`SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`).
			WithArgs(hashOf(token)).
			WillReturnRows(rows)
		mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE id = \? AND revoked_at IS NULL`).
			WithArgs(sqlmock.AnyArg(), int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		svc := services.NewRefreshTokenService()
		err := svc.RevokeToken(context.Background(), db, token)
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("token não encontrado retorna ErrRefreshTokenNotFound", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		mock.ExpectQuery(`SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`).
			WithArgs(hashOf(token)).
			WillReturnError(sql.ErrNoRows)

		svc := services.NewRefreshTokenService()
		err := svc.RevokeToken(context.Background(), db, token)
		assert.ErrorIs(t, err, services.ErrRefreshTokenNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no find é propagado", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		mock.ExpectQuery(`SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`).
			WithArgs(hashOf(token)).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewRefreshTokenService()
		err := svc.RevokeToken(context.Background(), db, token)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("token revogado entre find e revoke (corrida) retorna ErrRefreshTokenRevoked", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		rows := sqlmock.NewRows(refreshTokenColunas()).
			AddRow(int64(1), int64(10), hashOf(token), time.Now().Add(1*time.Hour), nil, "127.0.0.1", "curl/8.0")
		mock.ExpectQuery(`SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`).
			WithArgs(hashOf(token)).
			WillReturnRows(rows)
		mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE id = \? AND revoked_at IS NULL`).
			WithArgs(sqlmock.AnyArg(), int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		svc := services.NewRefreshTokenService()
		err := svc.RevokeToken(context.Background(), db, token)
		assert.ErrorIs(t, err, services.ErrRefreshTokenRevoked)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// RevokeAllUserTokens
// ---------------------------------------------------------------------------

func TestRefreshTokenService_RevokeAllUserTokens(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
			WithArgs(sqlmock.AnyArg(), int64(10)).
			WillReturnResult(sqlmock.NewResult(0, 3))

		svc := services.NewRefreshTokenService()
		err := svc.RevokeAllUserTokens(context.Background(), db, 10)
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro do repo é propagado", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewRefreshTokenService()
		err := svc.RevokeAllUserTokens(context.Background(), db, 10)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// CleanupExpired
// ---------------------------------------------------------------------------

func TestRefreshTokenService_CleanupExpired(t *testing.T) {
	testCases := []struct {
		nome    string
		mock    func(mock sqlmock.Sqlmock)
		wantN   int64
		wantErr bool
	}{
		{
			nome: "sucesso - remove tokens expirados",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM refresh_tokens`).
					WillReturnResult(sqlmock.NewResult(0, 5))
			},
			wantN: 5,
		},
		{
			nome: "sucesso - nada para remover",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM refresh_tokens`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantN: 0,
		},
		{
			nome: "erro do repo é propagado",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM refresh_tokens`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			tc.mock(mock)

			svc := services.NewRefreshTokenService()
			n, err := svc.CleanupExpired(context.Background(), db)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantN, n)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
