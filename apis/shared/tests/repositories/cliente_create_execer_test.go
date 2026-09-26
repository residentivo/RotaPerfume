package repositories_test

// SEC-01: ClienteRepository.Create passou a aceitar Execer (*sql.DB ou
// *sql.Tx) para participar da transação cliente + carteira.

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

const reInsertClienteRepo = `INSERT INTO clientes \(cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo\)\s+VALUES \(\?, \?, \?, \?, \?, \?, \?, \?\)`

func TestClienteRepository_Create(t *testing.T) {
	errDB := errors.New("falha simulada")
	dataCad := time.Date(2024, 2, 10, 0, 0, 0, 0, time.Local)

	casos := []struct {
		nome    string
		emTx    bool
		result  func(e *sqlmock.ExpectedExec)
		wantID  int64
		wantErr error
	}{
		{"sucesso com *sql.DB", false, func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(321, 1)) }, 321, nil},
		{"sucesso com *sql.Tx", true, func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(322, 1)) }, 322, nil},
		{"erro no exec", false, func(e *sqlmock.ExpectedExec) { e.WillReturnError(errDB) }, 0, errDB},
		{"erro no exec dentro da tx", true, func(e *sqlmock.ExpectedExec) { e.WillReturnError(errDB) }, 0, errDB},
		{"erro no lastInsertId", false, func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewErrorResult(errDB)) }, 0, errDB},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			defer db.Close()
			ctx := context.Background()

			var exec repositories.Execer = db
			var tx *sql.Tx
			if tc.emTx {
				mock.ExpectBegin()
				tx, err = db.BeginTx(ctx, nil)
				require.NoError(t, err)
				exec = tx
			}
			tc.result(mock.ExpectExec(reInsertClienteRepo).
				WithArgs("12345678000199", "Empresa", "varejo", "Curitiba", "PR", "Centro", dataCad, true))
			if tc.emTx {
				mock.ExpectRollback()
			}

			c := &models.Cliente{
				CNPJ: "12345678000199", RazaoSocial: "Empresa", Segmento: "varejo", Cidade: "Curitiba",
				UF: "PR", Bairro: "Centro", DataCadastro: dataCad, Ativo: true,
			}
			err = repositories.NewClienteRepository().Create(ctx, exec, c)
			if tx != nil {
				require.NoError(t, tx.Rollback())
			}

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Zero(t, c.ClienteIDOrigem)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, c.ClienteIDOrigem)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
