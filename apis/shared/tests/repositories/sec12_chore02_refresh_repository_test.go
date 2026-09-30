package repositories_test

// SEC-12 / CHORE-02 (Lote 12): RefreshTokenRepository.
//   - MarcarReusoDetectado: UPDATE condicional (revoked_reason='rotacao' e
//     reuso_detectado_em NULL ou <= limite), args (agora, id, limite);
//     RowsAffected > 0 → true.
//   - DesfazerReusoDetectado: WHERE id = ? AND reuso_detectado_em = ?, args
//     (id, marca).
//   - DeleteExpired: DELETE ... WHERE expires_at < ? ORDER BY id LIMIT ?,
//     args exatamente (corte, lote).
// O SQL é comparado por igualdade (espaços normalizados), não por regex.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

var reEspacos = regexp.MustCompile(`\s+`)

func normalizarSQL(s string) string { return strings.TrimSpace(reEspacos.ReplaceAllString(s, " ")) }

// newMockSQLExato: sqlmock que exige o SQL idêntico (espaços normalizados).
func newMockSQLExato(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	exato := sqlmock.QueryMatcherFunc(func(esperado, atual string) error {
		if normalizarSQL(esperado) != normalizarSQL(atual) {
			return fmt.Errorf("SQL diferente:\n esperado: %s\n atual:    %s", normalizarSQL(esperado), normalizarSQL(atual))
		}
		return nil
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(exato))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

const (
	sqlMarcarReuso = `UPDATE refresh_tokens SET reuso_detectado_em = ?
		WHERE id = ? AND revoked_reason = 'rotacao'
		  AND (reuso_detectado_em IS NULL OR reuso_detectado_em <= ?)`
	sqlDesfazerReuso = `UPDATE refresh_tokens SET reuso_detectado_em = NULL WHERE id = ? AND reuso_detectado_em = ?`
	sqlDeleteExpired = `DELETE FROM refresh_tokens WHERE expires_at < ? ORDER BY id LIMIT ?`
)

var errBancoSEC12 = errors.New("banco fora do ar")

func TestSEC12_MarcarReusoDetectado(t *testing.T) {
	agora := time.Date(2026, 9, 27, 10, 30, 0, 0, time.Local)
	limite := agora.Add(-30 * time.Minute)

	casos := []struct {
		nome     string
		setup    func(e *sqlmock.ExpectedExec)
		want     bool
		wantErr  string
		wantWrap error
	}{
		{"1 linha → marcou (corta)", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1)) }, true, "", nil},
		{"0 linhas → suprimido", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 0)) }, false, "", nil},
		{"erro no Exec é propagado (envolvido)", func(e *sqlmock.ExpectedExec) { e.WillReturnError(errBancoSEC12) }, false, "repositories: marcar reuso refresh_token", errBancoSEC12},
		{"erro no RowsAffected é propagado", func(e *sqlmock.ExpectedExec) {
			e.WillReturnResult(sqlmock.NewErrorResult(errBancoSEC12))
		}, false, "marcar reuso refresh_token rows affected", errBancoSEC12},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMockSQLExato(t)
			c.setup(mock.ExpectExec(sqlMarcarReuso).WithArgs(agora, int64(10), limite))

			ok, err := repositories.NewRefreshTokenRepository().MarcarReusoDetectado(context.Background(), db, 10, agora, limite)
			assert.Equal(t, c.want, ok)
			if c.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErr)
				assert.ErrorIs(t, err, c.wantWrap)
			} else {
				require.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Aceita *sql.Tx (Execer).
func TestSEC12_MarcarReusoDetectado_EmTransacao(t *testing.T) {
	db, mock := newMockSQLExato(t)
	agora := time.Date(2026, 9, 27, 10, 30, 0, 0, time.Local)
	mock.ExpectBegin()
	mock.ExpectExec(sqlMarcarReuso).WithArgs(agora, int64(7), agora.Add(-time.Minute)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx, err := db.Begin()
	require.NoError(t, err)
	ok, err := repositories.NewRefreshTokenRepository().MarcarReusoDetectado(context.Background(), tx, 7, agora, agora.Add(-time.Minute))
	require.NoError(t, err)
	assert.True(t, ok)
	require.NoError(t, tx.Commit())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC12_DesfazerReusoDetectado(t *testing.T) {
	marca := time.Date(2026, 9, 27, 10, 30, 0, 0, time.Local)
	casos := []struct {
		nome    string
		setup   func(e *sqlmock.ExpectedExec)
		wantErr bool
	}{
		{"limpa a marca (1 linha)", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1)) }, false},
		{"marca já trocada por outro reuso (0 linhas) não é erro", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 0)) }, false},
		{"erro propagado", func(e *sqlmock.ExpectedExec) { e.WillReturnError(errBancoSEC12) }, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMockSQLExato(t)
			c.setup(mock.ExpectExec(sqlDesfazerReuso).WithArgs(int64(10), marca))

			err := repositories.NewRefreshTokenRepository().DesfazerReusoDetectado(context.Background(), db, 10, marca)
			if c.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, errBancoSEC12)
				assert.Contains(t, err.Error(), "repositories: desfazer reuso refresh_token")
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCHORE02_DeleteExpired(t *testing.T) {
	corte := time.Date(2026, 8, 28, 12, 0, 0, 0, time.Local)
	casos := []struct {
		nome    string
		lote    int
		setup   func(e *sqlmock.ExpectedExec)
		wantN   int64
		wantErr string
	}{
		{"lote cheio (1000)", 1000, func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1000)) }, 1000, ""},
		{"lote parcial", 1000, func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 3)) }, 3, ""},
		{"nada a apagar", 1000, func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 0)) }, 0, ""},
		{"lote 1", 1, func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1)) }, 1, ""},
		{"erro no Exec", 1000, func(e *sqlmock.ExpectedExec) { e.WillReturnError(errBancoSEC12) }, 0, "repositories: delete expired refresh_tokens"},
		{"erro no RowsAffected", 1000, func(e *sqlmock.ExpectedExec) {
			e.WillReturnResult(sqlmock.NewErrorResult(errBancoSEC12))
		}, 0, "delete expired refresh_tokens rows affected"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMockSQLExato(t)
			c.setup(mock.ExpectExec(sqlDeleteExpired).WithArgs(corte, c.lote))

			n, err := repositories.NewRefreshTokenRepository().DeleteExpired(context.Background(), db, corte, c.lote)
			assert.Equal(t, c.wantN, n)
			if c.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErr)
				assert.ErrorIs(t, err, errBancoSEC12)
			} else {
				require.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
