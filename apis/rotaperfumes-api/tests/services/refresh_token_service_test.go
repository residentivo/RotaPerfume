package services_test

// Refresh tokens (o RefreshTokenService legado). ARQ-07 (pendência a): o
// repassador que reimplementava a classificação do token (RevokedTokenError)
// saiu. Os testes, com os mesmos nomes, exercitam o código de produção:
//   - emissão: o caso de uso RenovarSessao (gerador opaco + adapter na tx);
//   - validação: identidademysql.RefreshTokens.PorHash + o domínio
//     identidade.RefreshToken.Avaliar (a regra que o caso de uso aplica);
//   - revogação: o passo do Logout (PorHash + Revogar condicional) e
//     RevogarTodosDoUsuario;
//   - limpeza: o caso de uso LimparRefreshExpirados.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
)

func newRefreshTokenTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func refreshTokenColunas() []string {
	return []string{"id", "usuario_id", "token_hash", "expires_at", "revoked_at", "ip_origem", "user_agent", "revoked_reason"}
}

func hashOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

const findRefreshSQL = `SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent, revoked_reason\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`

// buscarEAvaliar é a validação que o RenovarSessao faz: busca pelo hash do
// token e classifica com o relógio informado.
func buscarEAvaliar(db *sql.DB, token string, agora time.Time) (*identidade.RefreshToken, identidade.EstadoRefresh, error) {
	rt, err := identidademysql.NovoRefreshTokens(db).PorHash(context.Background(), hashOf(token))
	if err != nil {
		return nil, 0, err
	}
	return rt, rt.Avaliar(agora), nil
}

// revogarPorToken é o passo do Logout: busca pelo hash e revoga
// condicionalmente com o motivo logout.
func revogarPorToken(db *sql.DB, token string) error {
	tokens := identidademysql.NovoRefreshTokens(db)
	rt, err := tokens.PorHash(context.Background(), hashOf(token))
	if err != nil {
		return err
	}
	return tokens.Revogar(context.Background(), rt.ID, identidade.MotivoLogout)
}

// ---------------------------------------------------------------------------
// GenerateRefreshToken
// ---------------------------------------------------------------------------

func TestRefreshTokenService_GenerateRefreshToken(t *testing.T) {
	t.Run("sucesso - cria token e retorna texto puro", func(t *testing.T) {
		s, err, mock := renovar(t, func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(revokeCondicionalSQL).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`INSERT INTO refresh_tokens \(usuario_id, token_hash, expires_at, ip_origem, user_agent\)`).
				WithArgs(int64(3), sqlmock.AnyArg(), sqlmock.AnyArg(), "ip", "ua").
				WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectCommit()
		})
		require.NoError(t, err)
		token := s.Refresh.Revelar()
		assert.NotEmpty(t, token)
		assert.Len(t, token, identidadecrypto.BytesRefresh*2) // hex-encoded
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no repo é propagado", func(t *testing.T) {
		s, err, mock := renovar(t, func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(revokeCondicionalSQL).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`INSERT INTO refresh_tokens \(usuario_id, token_hash, expires_at, ip_origem, user_agent\)`).
				WillReturnError(sql.ErrConnDone)
			m.ExpectRollback()
		})
		assert.Error(t, err)
		assert.Nil(t, s, "nenhum token sai quando a gravação falha")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// ValidateRefreshToken
// ---------------------------------------------------------------------------

func TestRefreshTokenService_ValidateRefreshToken(t *testing.T) {
	token := "token-em-texto-puro"

	testCases := []struct {
		nome       string
		mock       func(mock sqlmock.Sqlmock)
		wantErr    error
		wantEstado identidade.EstadoRefresh
	}{
		{
			nome: "sucesso - token válido não expirado nem revogado",
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(1), int64(10), hashOf(token), time.Now().Add(1*time.Hour), nil, "127.0.0.1", "curl/8.0", nil)
				mock.ExpectQuery(findRefreshSQL).WithArgs(hashOf(token)).WillReturnRows(rows)
			},
			wantEstado: identidade.RefreshValido,
		},
		{
			nome: "não encontrado retorna ErrRefreshTokenNotFound",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(findRefreshSQL).WithArgs(hashOf(token)).WillReturnError(sql.ErrNoRows)
			},
			wantErr: identidade.ErrRefreshInvalido,
		},
		{
			nome: "expirado retorna ErrRefreshTokenExpired",
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(1), int64(10), hashOf(token), time.Now().Add(-1*time.Hour), nil, "127.0.0.1", "curl/8.0", nil)
				mock.ExpectQuery(findRefreshSQL).WithArgs(hashOf(token)).WillReturnRows(rows)
			},
			wantEstado: identidade.RefreshExpirado,
		},
		{
			nome: "revogado retorna ErrRefreshTokenRevoked",
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(1), int64(10), hashOf(token), time.Now().Add(1*time.Hour), time.Now(), "127.0.0.1", "curl/8.0", nil)
				mock.ExpectQuery(findRefreshSQL).WithArgs(hashOf(token)).WillReturnRows(rows)
			},
			wantEstado: identidade.RefreshRevogadoRecente, // revogado agora: dentro da janela de graça
		},
		{
			nome: "erro genérico do repo é propagado",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(findRefreshSQL).WithArgs(hashOf(token)).WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			tc.mock(mock)

			rt, estado, err := buscarEAvaliar(db, token, time.Now())

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, rt)
			} else {
				require.NoError(t, err)
				require.NotNil(t, rt)
				assert.Equal(t, int64(1), rt.ID)
				assert.Equal(t, tc.wantEstado, estado)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRefreshTokenService_ValidateRefreshToken_JanelaDeGraca (SEC-04): com
// relógio injetado, um token revogado dentro de identidade.JanelaGracaRevogacao
// (em qualquer direção) é RevogadoRecente (não conta no rate limit); fora
// dela, Revogado. A expiração tem precedência sobre a revogação.
func TestRefreshTokenService_ValidateRefreshToken_JanelaDeGraca(t *testing.T) {
	const token = "token-janela-graca"
	agora := time.Date(2026, 9, 25, 12, 0, 0, 0, time.Local)

	cases := []struct {
		nome      string
		expiresAt time.Time
		revokedAt time.Time
		want      identidade.EstadoRefresh
	}{
		{"revogado há 5s -> Recently", agora.Add(time.Hour), agora.Add(-5 * time.Second), identidade.RefreshRevogadoRecente},
		{"revogado há exatamente 30s -> Recently", agora.Add(time.Hour), agora.Add(-identidade.JanelaGracaRevogacao), identidade.RefreshRevogadoRecente},
		{"revogado há 31s -> Revoked", agora.Add(time.Hour), agora.Add(-31 * time.Second), identidade.RefreshRevogado},
		{"revogado há 1min -> Revoked", agora.Add(time.Hour), agora.Add(-time.Minute), identidade.RefreshRevogado},
		{"revoked_at 1s no futuro -> Recently", agora.Add(time.Hour), agora.Add(time.Second), identidade.RefreshRevogadoRecente},
		{"revoked_at 31s no futuro -> Revoked", agora.Add(time.Hour), agora.Add(31 * time.Second), identidade.RefreshRevogado},
		{"expirado e revogado recentemente -> Expired", agora.Add(-time.Second), agora.Add(-5 * time.Second), identidade.RefreshExpirado},
	}

	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			mock.ExpectQuery(findRefreshSQL).WithArgs(hashOf(token)).
				WillReturnRows(sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(1), int64(10), hashOf(token), tc.expiresAt, tc.revokedAt, "127.0.0.1", "curl/8.0", nil))

			_, estado, err := buscarEAvaliar(db, token, agora)

			require.NoError(t, err)
			assert.Equal(t, tc.want, estado)
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
			AddRow(int64(1), int64(10), hashOf(token), time.Now().Add(1*time.Hour), nil, "127.0.0.1", "curl/8.0", nil)
		mock.ExpectQuery(findRefreshSQL).WithArgs(hashOf(token)).WillReturnRows(rows)
		mock.ExpectExec(revokeCondicionalSQL).
			WithArgs(sqlmock.AnyArg(), "logout", int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, revogarPorToken(db, token))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("token não encontrado retorna ErrRefreshTokenNotFound", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		mock.ExpectQuery(findRefreshSQL).WithArgs(hashOf(token)).WillReturnError(sql.ErrNoRows)

		assert.ErrorIs(t, revogarPorToken(db, token), identidade.ErrRefreshInvalido)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no find é propagado", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		mock.ExpectQuery(findRefreshSQL).WithArgs(hashOf(token)).WillReturnError(sql.ErrConnDone)

		assert.Error(t, revogarPorToken(db, token))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("token revogado entre find e revoke (corrida) retorna ErrRefreshTokenRevoked", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		rows := sqlmock.NewRows(refreshTokenColunas()).
			AddRow(int64(1), int64(10), hashOf(token), time.Now().Add(1*time.Hour), nil, "127.0.0.1", "curl/8.0", nil)
		mock.ExpectQuery(findRefreshSQL).WithArgs(hashOf(token)).WillReturnRows(rows)
		mock.ExpectExec(revokeCondicionalSQL).
			WithArgs(sqlmock.AnyArg(), "logout", int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		assert.ErrorIs(t, revogarPorToken(db, token), identidade.ErrRefreshRevogado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// RevokeAllUserTokens
// ---------------------------------------------------------------------------

func TestRefreshTokenService_RevokeAllUserTokens(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
			WithArgs(sqlmock.AnyArg(), "revogacao_massa", int64(10)).
			WillReturnResult(sqlmock.NewResult(0, 3))

		err := identidademysql.NovoRefreshTokens(db).RevogarTodosDoUsuario(context.Background(), 10, identidade.MotivoRevogacaoMassa)
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro do repo é propagado", func(t *testing.T) {
		db, mock := newRefreshTokenTestDB(t)
		mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
			WillReturnError(sql.ErrConnDone)

		err := identidademysql.NovoRefreshTokens(db).RevogarTodosDoUsuario(context.Background(), 10, identidade.MotivoRevogacaoMassa)
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

			n, err := novosCasos(db, nil, nil, time.Now, identidadeapp.Opcoes{}).LimparRefreshExpirados(context.Background())

			if tc.wantErr {
				assert.Error(t, err)
				assert.False(t, errors.Is(err, identidade.ErrRefreshInvalido))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantN, n)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
