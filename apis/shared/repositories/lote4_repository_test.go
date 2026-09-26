package repositories_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

// Testes do Lote 4: NEG-01 (1062 em uq_clientes_cnpj), SEC-03
// (ListResumoByID) e SEC-02 (Revoke condicional dentro de tx).

func TestClienteRepository_CNPJDuplicado(t *testing.T) {
	cases := []struct {
		nome    string
		err     error
		wantDup bool
	}{
		{"1062 em uq_clientes_cnpj", &mysql.MySQLError{Number: 1062, Message: "Duplicate entry '1' for key 'clientes.uq_clientes_cnpj'"}, true},
		{"1062 em outro índice", &mysql.MySQLError{Number: 1062, Message: "Duplicate entry '1' for key 'clientes.PRIMARY'"}, false},
		{"outro código com o nome do índice", &mysql.MySQLError{Number: 1452, Message: "uq_clientes_cnpj"}, false},
		{"erro genérico", errors.New("db down"), false},
	}
	for _, tc := range cases {
		c := &models.Cliente{CNPJ: "11222333000181", DataCadastro: time.Now()}
		t.Run("create/"+tc.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			mock.ExpectExec(`INSERT INTO clientes`).WillReturnError(tc.err)

			err := repositories.NewClienteRepository().Create(context.Background(), db, c)
			require.Error(t, err)
			assert.Equal(t, tc.wantDup, errors.Is(err, repositories.ErrCNPJDuplicado))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
		t.Run("update/"+tc.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			mock.ExpectExec(`UPDATE clientes`).WillReturnError(tc.err)

			err := repositories.NewClienteRepository().Update(context.Background(), db, 1, c)
			require.Error(t, err)
			assert.Equal(t, tc.wantDup, errors.Is(err, repositories.ErrCNPJDuplicado))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVendedorRepository_ListResumoByID(t *testing.T) {
	const q = `SELECT id, nome, regiao, uf, data_desligamento\s+FROM vendedores\s+WHERE id = \?\s+LIMIT 1`
	cols := []string{"id", "nome", "regiao", "uf", "data_desligamento"}

	t.Run("encontrado", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(q).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(7), "Sete", "Sul", "PR", nil))

		out, err := repositories.NewVendedorRepository().ListResumoByID(context.Background(), db, 7)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, int64(7), out[0].ID)
		assert.Nil(t, out[0].DataDesligamento)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("inexistente devolve lista vazia não nula", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(q).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows(cols))

		out, err := repositories.NewVendedorRepository().ListResumoByID(context.Background(), db, 7)
		require.NoError(t, err)
		assert.NotNil(t, out)
		assert.Empty(t, out)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("erro de banco", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(q).WillReturnError(errors.New("db down"))

		_, err = repositories.NewVendedorRepository().ListResumoByID(context.Background(), db, 7)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("erro de scan", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(q).WillReturnRows(sqlmock.NewRows(cols).AddRow("x", "Sete", "Sul", "PR", nil))

		_, err = repositories.NewVendedorRepository().ListResumoByID(context.Background(), db, 7)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestRefreshTokenRevoke_Condicional_EmTx: o UPDATE só afeta token ainda não
// revogado e aceita *sql.Tx; 0 linhas → ErrNotFound (já revogado).
func TestRefreshTokenRevoke_Condicional_EmTx(t *testing.T) {
	cases := []struct {
		nome    string
		rows    int64
		wantErr error
	}{
		{"revoga", 1, nil},
		{"já revogado", 0, repositories.ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE id = \? AND revoked_at IS NULL`).
				WithArgs(sqlmock.AnyArg(), "rotacao", int64(3)).WillReturnResult(sqlmock.NewResult(0, tc.rows))
			mock.ExpectRollback()

			tx, err := db.Begin()
			require.NoError(t, err)
			err = repositories.NewRefreshTokenRepository().Revoke(context.Background(), tx, 3, repositories.RevokeReasonRotacao)
			_ = tx.Rollback()
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRefreshTokenRevoke_RowsAffectedErro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()
	mock.ExpectExec(`UPDATE refresh_tokens`).WillReturnResult(sqlmock.NewErrorResult(errors.New("sem rows affected")))

	err := repositories.NewRefreshTokenRepository().Revoke(context.Background(), db, 3, repositories.RevokeReasonRotacao)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}
