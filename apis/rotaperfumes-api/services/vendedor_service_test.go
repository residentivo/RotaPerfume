package services_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

func vendedorTestCfg(verbose bool) *config.Config {
	return &config.Config{Verbose: verbose}
}

func newVendedorTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

var vendedorColunasRegex = `id, nome, regiao, uf\s+FROM vendedores\s+WHERE data_desligamento IS NULL\s+ORDER BY nome ASC`

func vendedorRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "nome", "regiao", "uf"}).
		AddRow(int64(1), "João Vendedor", "Sudeste", "SP").
		AddRow(int64(2), "Maria Vendedora", "Sul", "RS")
}

// ---------------------------------------------------------------------------
// ListVendedores
// ---------------------------------------------------------------------------

func TestVendedorService_ListVendedores(t *testing.T) {
	testCases := []struct {
		nome    string
		verbose bool
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
	}{
		{
			nome:    "sucesso - retorna vendedores ativos ordenados por nome",
			verbose: true,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorColunasRegex).
					WillReturnRows(vendedorRows())
			},
			wantLen: 2,
		},
		{
			nome:    "sucesso - lista vazia",
			verbose: false,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorColunasRegex).
					WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf"}))
			},
			wantLen: 0,
		},
		{
			nome:    "erro do repo é propagado",
			verbose: false,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorColunasRegex).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVendedorTestDB(t)
			tc.mock(mock)

			svc := services.NewVendedorService(db, vendedorTestCfg(tc.verbose))
			vendedores, err := svc.ListVendedores(context.Background(), db)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, vendedores)
			} else {
				require.NoError(t, err)
				assert.Len(t, vendedores, tc.wantLen)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
