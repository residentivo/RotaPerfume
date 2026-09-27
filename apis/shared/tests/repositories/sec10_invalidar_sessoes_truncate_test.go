package repositories_test

// SEC-10: InvalidarSessoes grava t.Truncate(time.Second). Um DATETIME sem
// fração no MySQL arredondaria 12:00:00.700 para 12:00:01 e derrubaria um
// token legítimo emitido no segundo seguinte. Reaproveita newMock e
// reInvalidar dos demais testes do pacote.

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

func TestSEC10_InvalidarSessoes_TruncaEmSegundos(t *testing.T) {
	base := time.Date(2026, 9, 26, 12, 0, 0, 0, time.Local)
	utc := time.Date(2026, 9, 26, 15, 0, 0, 0, time.UTC)
	casos := []struct {
		nome string
		in   time.Time
		want time.Time
	}{
		{"12:00:00.700 -> 12:00:00", base.Add(700 * time.Millisecond), base},
		{"12:00:00.999999999 -> 12:00:00 (nunca arredonda para cima)", base.Add(999999999), base},
		{"12:00:00.000000001 -> 12:00:00", base.Add(1), base},
		{"12:00:00.500 -> 12:00:00", base.Add(500 * time.Millisecond), base},
		{"sem fração: inalterado", base, base},
		{"fuso preservado (UTC)", utc.Add(700 * time.Millisecond), utc},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			mock.ExpectExec(reInvalidar).WithArgs(c.want, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
			require.NoError(t, repositories.NewUsuarioRepository().InvalidarSessoes(context.Background(), db, 7, c.in))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
	t.Run("time.Now() com monotônico: grava sem fração", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		agora := time.Now()
		mock.ExpectExec(reInvalidar).WithArgs(semFracao{}, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repositories.NewUsuarioRepository().InvalidarSessoes(context.Background(), db, 7, agora))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// semFracao casa com time.Time sem nanossegundos.
type semFracao struct{}

func (semFracao) Match(v driver.Value) bool {
	tm, ok := v.(time.Time)
	return ok && tm.Nanosecond() == 0
}
