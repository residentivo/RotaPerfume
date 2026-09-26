package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/repositories"
)

// BUG-04: desligar preserva a data original via COALESCE.
const reMarcarDesligamento = `UPDATE vendedores SET data_desligamento = COALESCE\(data_desligamento, \?\) WHERE id = \?`

func TestVendedorMarcarDesligamento(t *testing.T) {
	agora := time.Now()
	casos := []struct {
		nome    string
		result  func(m sqlmock.Sqlmock)
		wantErr error
		algum   bool
	}{
		{"linha encontrada (nova ou já desligada)", func(m sqlmock.Sqlmock) {
			m.ExpectExec(reMarcarDesligamento).WithArgs(agora, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
		}, nil, false},
		{"id inexistente", func(m sqlmock.Sqlmock) {
			m.ExpectExec(reMarcarDesligamento).WithArgs(agora, int64(1)).WillReturnResult(sqlmock.NewResult(0, 0))
		}, repositories.ErrNotFound, false},
		{"erro no exec", func(m sqlmock.Sqlmock) {
			m.ExpectExec(reMarcarDesligamento).WillReturnError(sql.ErrConnDone)
		}, nil, true},
		{"erro no rowsAffected", func(m sqlmock.Sqlmock) {
			m.ExpectExec(reMarcarDesligamento).WillReturnResult(sqlmock.NewErrorResult(sql.ErrConnDone))
		}, nil, true},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			tc.result(mock)

			err := repositories.NewVendedorRepository().MarcarDesligamento(context.Background(), db, 1, agora)

			switch {
			case tc.wantErr != nil:
				assert.ErrorIs(t, err, tc.wantErr)
			case tc.algum:
				assert.Error(t, err)
			default:
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
