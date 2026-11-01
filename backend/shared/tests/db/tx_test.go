package db_test

// db.Transactor (Unit of Work, ARQ-01): commit quando a fn devolve nil,
// rollback em qualquer erro, erro da fn sem embrulho e os repositórios R
// ligados à tx (o SQL passa pela transação, não pelo pool).

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/db"
)

// repoTeste é um "repositório" mínimo ligado a um Querier.
type repoTeste struct{ q db.Querier }

func (r repoTeste) marcar(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, "UPDATE coisas SET marcada = 1 WHERE id = ? AND marcada = 0", id)
	return err
}

type txTeste struct{ Coisas repoTeste }

func novoTransactor(t *testing.T) (*db.Transactor[txTeste], sqlmock.Sqlmock) {
	t.Helper()
	pool, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return db.NewTransactor(pool, func(q db.Querier) txTeste { return txTeste{Coisas: repoTeste{q: q}} }), mock
}

func TestTransactor_WithTx(t *testing.T) {
	errNegocio := errors.New("regra de negócio violada")
	errDriver := errors.New("driver: conexão perdida")

	casos := []struct {
		nome    string
		preparo func(sqlmock.Sqlmock)
		fn      func(context.Context, txTeste) error
		wantErr error
		wantMsg string
	}{
		{
			nome: "sucesso faz commit",
			preparo: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(`UPDATE coisas`).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			},
			fn: func(ctx context.Context, r txTeste) error { return r.Coisas.marcar(ctx, 1) },
		},
		{
			nome: "erro da fn faz rollback e volta sem embrulho",
			preparo: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectRollback()
			},
			fn:      func(context.Context, txTeste) error { return errNegocio },
			wantErr: errNegocio,
			wantMsg: errNegocio.Error(),
		},
		{
			nome: "erro de SQL no meio faz rollback",
			preparo: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(`UPDATE coisas`).WillReturnError(errDriver)
				m.ExpectRollback()
			},
			fn:      func(ctx context.Context, r txTeste) error { return r.Coisas.marcar(ctx, 1) },
			wantErr: errDriver,
		},
		{
			nome:    "falha no begin não chama a fn",
			preparo: func(m sqlmock.Sqlmock) { m.ExpectBegin().WillReturnError(errDriver) },
			fn: func(context.Context, txTeste) error {
				panic("fn não pode rodar sem transação")
			},
			wantErr: errDriver,
			wantMsg: "db: begin: " + errDriver.Error(),
		},
		{
			nome: "falha no commit volta com o nome da operação",
			preparo: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectCommit().WillReturnError(errDriver)
			},
			fn:      func(context.Context, txTeste) error { return nil },
			wantErr: errDriver,
			wantMsg: "db: commit: " + errDriver.Error(),
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			tr, mock := novoTransactor(t)
			c.preparo(mock)
			ctx := context.Background()
			err := tr.WithTx(ctx, func(r txTeste) error { return c.fn(ctx, r) })
			if c.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, c.wantErr)
			}
			if c.wantMsg != "" {
				assert.EqualError(t, err, c.wantMsg)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// A fn recebe um Querier que é a própria transação.
func TestTransactor_BindRecebeTx(t *testing.T) {
	pool, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	mock.ExpectBegin()
	mock.ExpectCommit()

	var recebido db.Querier
	tr := db.NewTransactor(pool, func(q db.Querier) db.Querier { recebido = q; return q })
	require.NoError(t, tr.WithTx(context.Background(), func(db.Querier) error { return nil }))
	_, ehTx := recebido.(*sql.Tx)
	assert.True(t, ehTx, "bind deve receber *sql.Tx, nunca o pool")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNewTransactor_DependenciaNilPanica(t *testing.T) {
	pool, _, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	bind := func(q db.Querier) db.Querier { return q }

	assert.Panics(t, func() { db.NewTransactor[db.Querier](nil, bind) })
	assert.Panics(t, func() { db.NewTransactor[db.Querier](pool, nil) })
	assert.NotPanics(t, func() { db.NewTransactor(pool, bind) })
}
