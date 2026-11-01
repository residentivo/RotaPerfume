package db_test

// db.Transactor (ARQ-01, TestBrain): o rollback é garantido pelo defer, então
// vale também quando a fn entra em pânico; e uma falha no próprio rollback
// nunca mascara o erro de negócio devolvido pela fn.

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestTransactor_WithTx_RollbackGarantido(t *testing.T) {
	errNegocio := errors.New("regra de negócio violada")
	errRollback := errors.New("driver: rollback falhou")

	casos := []struct {
		nome       string
		comSQL     bool
		rollback   error
		fn         func(context.Context, txTeste) error
		wantPanico any
		wantErr    error
	}{
		{
			nome:       "panic na fn faz rollback e repropaga o panic",
			fn:         func(context.Context, txTeste) error { panic("bug na fn") },
			wantPanico: "bug na fn",
		},
		{
			nome:       "panic após SQL na fn faz rollback (nada é commitado)",
			comSQL:     true,
			fn:         func(ctx context.Context, r txTeste) error { _ = r.Coisas.marcar(ctx, 1); panic("depois do SQL") },
			wantPanico: "depois do SQL",
		},
		{
			nome:     "falha no rollback não mascara o erro da fn",
			rollback: errRollback,
			fn:       func(context.Context, txTeste) error { return errNegocio },
			wantErr:  errNegocio,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			tr, mock := novoTransactor(t)
			mock.ExpectBegin()
			if c.comSQL {
				mock.ExpectExec(`UPDATE coisas`).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			rb := mock.ExpectRollback()
			if c.rollback != nil {
				rb.WillReturnError(c.rollback)
			}
			ctx := context.Background()
			chamar := func() error { return tr.WithTx(ctx, func(r txTeste) error { return c.fn(ctx, r) }) }

			if c.wantPanico != nil {
				assert.PanicsWithValue(t, c.wantPanico, func() { _ = chamar() })
			} else {
				err := chamar()
				assert.ErrorIs(t, err, c.wantErr)
				assert.NotErrorIs(t, err, errRollback)
				assert.Equal(t, c.wantErr.Error(), err.Error(), "erro da fn volta sem embrulho")
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "begin+rollback, nunca commit")
		})
	}
}
