package services_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

// TestDashboardService_GetClienteMetrics_ErroEmCadaEtapa injeta erro em cada
// uma das seis consultas de GetClienteMetrics (escopo de carteira,
// vendedorID=7) e garante que o erro é propagado sem resposta parcial.
func TestDashboardService_GetClienteMetrics_ErroEmCadaEtapa(t *testing.T) {
	carteira := `cliente_id_origem IN \(SELECT cliente_id FROM carteiras WHERE vendedor_id = \? AND data_fim IS NULL\)`
	etapas := []struct {
		nome   string
		expect func(mock sqlmock.Sqlmock) *sqlmock.ExpectedQuery
	}{
		{"CountTotal", func(m sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
			return m.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ` + carteira + `$`).WithArgs(int64(7))
		}},
		{"CountPorAtivo(true)", func(m sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
			return m.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \? AND `+carteira).WithArgs(true, int64(7))
		}},
		{"CountPorAtivo(false)", func(m sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
			return m.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \? AND `+carteira).WithArgs(false, int64(7))
		}},
		{"CountNovosNoPeriodo", func(m sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
			return m.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE YEAR\(data_cadastro\)`).WithArgs(int64(7))
		}},
		{"CountPorSegmento", func(m sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
			return m.ExpectQuery(`SELECT segmento`).WithArgs(int64(7))
		}},
		{"CountPorUF", func(m sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
			return m.ExpectQuery(`SELECT uf`).WithArgs(int64(7))
		}},
	}

	for falha := range etapas {
		t.Run("erro em "+etapas[falha].nome, func(t *testing.T) {
			db, mock := newDashboardTestDB(t)
			for i := 0; i <= falha; i++ {
				q := etapas[i].expect(mock)
				switch {
				case i == falha:
					q.WillReturnError(sql.ErrConnDone)
				case i >= 4:
					q.WillReturnRows(sqlmock.NewRows([]string{"k", "total"}).AddRow("x", 1))
				default:
					q.WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
				}
			}

			svc := services.NewDashboardService(db, dashboardTestCfg(false))
			metrics, err := svc.GetClienteMetrics(context.Background(), db, "month", 7)
			assert.Nil(t, metrics)
			assert.ErrorIs(t, err, sql.ErrConnDone)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
