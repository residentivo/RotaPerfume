package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

func TestVendedorList_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "nome", "regiao", "uf"}).
		AddRow(1, "Vendedor A", "Sudeste", "SP").
		AddRow(2, "Vendedor B", "Sul", "RS")

	mock.ExpectQuery(`SELECT .+ FROM vendedores WHERE data_desligamento IS NULL ORDER BY nome ASC`).
		WillReturnRows(rows)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	vendedores, err := repo.List(ctx, db)

	require.NoError(t, err)
	assert.Len(t, vendedores, 2)
	assert.Equal(t, "Vendedor A", vendedores[0].Nome)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorList_Vazio(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ FROM vendedores WHERE data_desligamento IS NULL ORDER BY nome ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf"}))

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	vendedores, err := repo.List(ctx, db)

	require.NoError(t, err)
	assert.Len(t, vendedores, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorList_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ FROM vendedores WHERE data_desligamento IS NULL ORDER BY nome ASC`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	_, err := repo.List(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorList_ScanError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	// Faltando coluna força erro de scan (quantidade de colunas divergente).
	rows := sqlmock.NewRows([]string{"id", "nome", "regiao"}).
		AddRow(1, "Vendedor A", "Sudeste")

	mock.ExpectQuery(`SELECT .+ FROM vendedores WHERE data_desligamento IS NULL ORDER BY nome ASC`).
		WillReturnRows(rows)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	_, err := repo.List(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "nome", "regiao", "uf"}).
		AddRow(1, "Vendedor A", "Sudeste", "SP").
		RowError(0, sql.ErrConnDone)

	mock.ExpectQuery(`SELECT .+ FROM vendedores WHERE data_desligamento IS NULL ORDER BY nome ASC`).
		WillReturnRows(rows)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	_, err := repo.List(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorExistsByID(t *testing.T) {
	testes := []struct {
		nome     string
		id       int64
		mockRow  bool
		mockErr  error
		expected bool
		wantErr  bool
	}{
		{nome: "vendedor existe", id: 1, mockRow: true, expected: true},
		{nome: "vendedor nao existe", id: 999, mockErr: sql.ErrNoRows, expected: false},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			q := mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).WithArgs(tt.id)
			if tt.mockErr != nil {
				q.WillReturnError(tt.mockErr)
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			}

			repo := repositories.NewVendedorRepository()
			ctx := context.Background()
			exists, err := repo.ExistsByID(ctx, db, tt.id)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, exists)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVendedorExistsByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	exists, err := repo.ExistsByID(ctx, db, 1)

	assert.False(t, exists)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
