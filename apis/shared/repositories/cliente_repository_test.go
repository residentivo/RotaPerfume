// Package repositories_test contém testes de integração com banco de dados mockado.
// Cada função é stateless — o sqlmock substitui o driver real.
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

// boolPtr é um helper de teste usado por vários arquivos deste pacote
// (produto_repository_test.go, etc.) para obter um *bool a partir de um
// literal, útil ao montar filtros com campo *bool opcional.
func boolPtr(b bool) *bool {
	return &b
}

// TestClienteCountNovosNoPeriodo_Success cobre os três valores de periodo
// aceitos ("today", "week", "month") garantindo que cada um monta a query
// esperada e retorna o total corretamente.
func TestClienteCountNovosNoPeriodo_Success(t *testing.T) {
	testes := []struct {
		nome        string
		periodo     string
		queryRegexp string
	}{
		{
			nome:        "periodo today",
			periodo:     "today",
			queryRegexp: `SELECT COUNT\(\*\) FROM clientes WHERE data_cadastro = CURDATE\(\)`,
		},
		{
			nome:        "periodo week",
			periodo:     "week",
			queryRegexp: `SELECT COUNT\(\*\) FROM clientes WHERE data_cadastro >= DATE_SUB\(CURDATE\(\), INTERVAL 6 DAY\)`,
		},
		{
			nome:        "periodo month",
			periodo:     "month",
			queryRegexp: `SELECT COUNT\(\*\) FROM clientes WHERE YEAR\(data_cadastro\) = YEAR\(CURDATE\(\)\) AND MONTH\(data_cadastro\) = MONTH\(CURDATE\(\)\)`,
		},
		{
			nome:        "periodo desconhecido cai no default (month)",
			periodo:     "ano",
			queryRegexp: `SELECT COUNT\(\*\) FROM clientes WHERE YEAR\(data_cadastro\) = YEAR\(CURDATE\(\)\) AND MONTH\(data_cadastro\) = MONTH\(CURDATE\(\)\)`,
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery(tt.queryRegexp).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(3))

			repo := repositories.NewClienteRepository()
			ctx := context.Background()
			total, err := repo.CountNovosNoPeriodo(ctx, db, tt.periodo)

			require.NoError(t, err)
			assert.Equal(t, 3, total)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestClienteCountNovosNoPeriodo_DBError garante que erros do banco são
// propagados (sem fallback silencioso, diferente do dashboard_repository).
func TestClienteCountNovosNoPeriodo_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, err := repo.CountNovosNoPeriodo(ctx, db, "week")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
