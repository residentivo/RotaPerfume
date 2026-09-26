package repositories_test

// Filtro de escopo por vendedor do DashboardRepository (helper interno
// vendedorFilter) testado pela API pública, cobrindo as três formas de coluna
// usadas: "vendedor_id" (GetVendasTotais), "id" (GetMetaMensalTotal) e "v.id"
// (GetTopVendedores). O ID do vendedor nunca é interpolado no SQL: vai só
// como argumento de placeholder.

import (
	"context"
	"database/sql"
	"strconv"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

func TestDashboardVendedorFilter(t *testing.T) {
	repo := repositories.NewDashboardRepository()
	ctx := context.Background()

	consultas := []struct {
		nome   string
		filtro string
		// executa registra a expectativa no mock (com ou sem o argumento do
		// vendedor) e chama o método público correspondente.
		executa func(t *testing.T, db *sql.DB, mock sqlmock.Sqlmock, vendedorID int64, comFiltro bool)
	}{
		{
			nome:   "vendedor_id (GetVendasTotais)",
			filtro: "AND vendedor_id = ?",
			executa: func(t *testing.T, db *sql.DB, mock sqlmock.Sqlmock, vendedorID int64, comFiltro bool) {
				exp := mock.ExpectQuery(`FROM pedidos`)
				if comFiltro {
					exp.WithArgs(vendedorID)
				} else {
					exp.WithArgs()
				}
				exp.WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(10.5, 2))
				v, q, err := repo.GetVendasTotais(ctx, db, "month", vendedorID)
				require.NoError(t, err)
				assert.Equal(t, 10.5, v)
				assert.Equal(t, 2, q)
			},
		},
		{
			nome:   "id (GetMetaMensalTotal)",
			filtro: "AND id = ?",
			executa: func(t *testing.T, db *sql.DB, mock sqlmock.Sqlmock, vendedorID int64, comFiltro bool) {
				exp := mock.ExpectQuery(`FROM vendedores`)
				if comFiltro {
					exp.WithArgs(vendedorID)
				} else {
					exp.WithArgs()
				}
				exp.WillReturnRows(sqlmock.NewRows([]string{"m"}).AddRow(1000.0))
				m, err := repo.GetMetaMensalTotal(ctx, db, vendedorID)
				require.NoError(t, err)
				assert.Equal(t, 1000.0, m)
			},
		},
		{
			nome:   "v.id (GetTopVendedores)",
			filtro: "AND v.id = ?",
			executa: func(t *testing.T, db *sql.DB, mock sqlmock.Sqlmock, vendedorID int64, comFiltro bool) {
				exp := mock.ExpectQuery(`FROM vendedores v`)
				if comFiltro {
					exp.WithArgs(vendedorID, 5)
				} else {
					exp.WithArgs(5)
				}
				// Sem linhas: não dispara o enriquecimento com vendas.
				exp.WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}))
				out, err := repo.GetTopVendedores(ctx, db, 5, vendedorID)
				require.NoError(t, err)
				assert.Empty(t, out)
			},
		},
	}

	ids := []struct {
		nome       string
		vendedorID int64
		comFiltro  bool
	}{
		{"admin (0) sem filtro", 0, false},
		{"negativo sem filtro", -5, false},
		{"vendedor 7", 7, true},
		{"vendedor 42", 42, true},
		{"vendedor 987654", 987654, true},
	}

	for _, c := range consultas {
		for _, id := range ids {
			t.Run(c.nome+"/"+id.nome, func(t *testing.T) {
				db, mock, captured := newCapturingMock(t)
				c.executa(t, db, mock, id.vendedorID, id.comFiltro)
				require.NoError(t, mock.ExpectationsWereMet())

				q := normalizeSQL((*captured)[len(*captured)-1])
				if id.comFiltro {
					assert.Contains(t, q, c.filtro)
					// O valor nunca é interpolado no SQL, só vai por placeholder.
					assert.NotContains(t, q, "= "+strconv.FormatInt(id.vendedorID, 10))
					assert.NotContains(t, q, strconv.FormatInt(id.vendedorID, 10)+" ")
				} else {
					assert.NotContains(t, q, c.filtro)
				}
			})
		}
	}
}
