package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

// TestClienteContagens_ErrosPropagados cobre os caminhos de erro (query,
// scan e iteração) das contagens do dashboard de clientes, com e sem o
// filtro de carteira (vendedorID > 0 / = 0). Os erros devem ser propagados
// e o resultado nunca deve ser um valor "parcial".
func TestClienteContagens_ErrosPropagados(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewClienteRepository()
	errDB := errors.New("db down")

	casos := []struct {
		nome  string
		setup func(mock sqlmock.Sqlmock)
		call  func(db *sql.DB) (any, error)
	}{
		{
			nome: "CountTotal erro de query (carteira)",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ` + carteiraAtivaWhereRegexp).
					WithArgs(int64(7)).WillReturnError(errDB)
			},
			call: func(db *sql.DB) (any, error) { return repo.CountTotal(ctx, db, 7) },
		},
		{
			nome: "CountPorAtivo erro de query (admin)",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \?$`).
					WithArgs(true).WillReturnError(errDB)
			},
			call: func(db *sql.DB) (any, error) { return repo.CountPorAtivo(ctx, db, true, 0) },
		},
		{
			nome: "CountPorSegmento erro de query",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT segmento`).WithArgs(int64(7)).WillReturnError(errDB)
			},
			call: func(db *sql.DB) (any, error) { return repo.CountPorSegmento(ctx, db, 7) },
		},
		{
			nome: "CountPorSegmento erro de scan",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT segmento`).
					WillReturnRows(sqlmock.NewRows([]string{"segmento", "total"}).AddRow("varejo", "nao-numero"))
			},
			call: func(db *sql.DB) (any, error) { return repo.CountPorSegmento(ctx, db, 0) },
		},
		{
			nome: "CountPorSegmento erro de iteração",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT segmento`).
					WillReturnRows(sqlmock.NewRows([]string{"segmento", "total"}).AddRow("varejo", 1).RowError(0, errDB))
			},
			call: func(db *sql.DB) (any, error) { return repo.CountPorSegmento(ctx, db, 0) },
		},
		{
			nome: "CountPorUF erro de query",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT uf`).WithArgs(int64(7)).WillReturnError(errDB)
			},
			call: func(db *sql.DB) (any, error) { return repo.CountPorUF(ctx, db, 7) },
		},
		{
			nome: "CountPorUF erro de scan",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT uf`).
					WillReturnRows(sqlmock.NewRows([]string{"uf", "total"}).AddRow("SP", "nao-numero"))
			},
			call: func(db *sql.DB) (any, error) { return repo.CountPorUF(ctx, db, 0) },
		},
		{
			nome: "CountPorUF erro de iteração",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT uf`).
					WillReturnRows(sqlmock.NewRows([]string{"uf", "total"}).AddRow("SP", 1).RowError(0, errDB))
			},
			call: func(db *sql.DB) (any, error) { return repo.CountPorUF(ctx, db, 0) },
		},
	}

	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			tc.setup(mock)

			out, err := tc.call(db)
			require.Error(t, err)
			switch v := out.(type) {
			case int:
				assert.Equal(t, 0, v)
			case []repositories.SegmentoContagem:
				assert.Nil(t, v)
			case []repositories.UFContagem:
				assert.Nil(t, v)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
