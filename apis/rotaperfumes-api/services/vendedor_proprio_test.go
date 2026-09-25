package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

// SEC-03: listagem de vendedores restrita ao próprio vendedor (filtro no SQL).
func TestVendedorService_ListVendedorProprio(t *testing.T) {
	const q = `SELECT id, nome, regiao, uf, data_desligamento\s+FROM vendedores\s+WHERE id = \?\s+LIMIT 1`
	cols := []string{"id", "nome", "regiao", "uf", "data_desligamento"}
	cases := []struct {
		nome    string
		verbose bool
		setup   func(m sqlmock.Sqlmock)
		wantLen int
		wantErr bool
	}{
		{"encontrado", true, func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(7), "Sete", "Sul", "PR", nil))
		}, 1, false},
		{"inexistente", false, func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows(cols))
		}, 0, false},
		{"erro de banco", false, func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q).WillReturnError(errors.New("db down"))
		}, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVendedorTestDB(t)
			tc.setup(mock)

			out, err := services.NewVendedorService(db, vendedorTestCfg(tc.verbose)).ListVendedorProprio(context.Background(), db, 7)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, out)
				assert.Len(t, out, tc.wantLen)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
