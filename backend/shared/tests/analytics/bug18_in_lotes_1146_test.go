package analytics_test

// BUG-18 (Lote 31): o IN das vendas do mês é fatiado em lotes de
// LoteVendasDoMes ids; só o código 1146 do driver é "tabela ausente" (a
// série degrada para zero); qualquer outro erro, inclusive com texto
// parecido ("not found", "doesn't exist"), é propagado.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsmysql"
	"github.com/rotaperfumes/shared/kernel"
)

func TestBUG18_MetasVendedores_INEmLotes(t *testing.T) {
	ctx := context.Background()
	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	e, _ := kernel.EscopoIrrestrito(admin)
	const total = analyticsmysql.LoteVendasDoMes + 50

	db, mock := novoMock(t)
	linhas := sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"})
	for i := 1; i <= total; i++ {
		linhas.AddRow(int64(i), "V", "S", "PR", 100.0)
	}
	mock.ExpectQuery(`FROM vendedores v`).WillReturnRows(linhas)
	in := func(n int) string {
		return `WHERE vendedor_id IN \(` + strings.TrimSuffix(strings.Repeat(`\?,`, n), ",") + `\)`
	}
	mock.ExpectQuery(in(analyticsmysql.LoteVendasDoMes)).WithArgs(append(idsDeAte(1, analyticsmysql.LoteVendasDoMes), 2026, 10)...).WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}).AddRow(int64(1), 10.0, 1))
	mock.ExpectQuery(in(50)).WithArgs(append(idsDeAte(analyticsmysql.LoteVendasDoMes+1, total), 2026, 10)...).WillReturnRows(sqlmock.NewRows([]string{"v", "t", "q"}).AddRow(int64(total), 20.0, 2))

	metas, err := analyticsmysql.NovoDashboard(db, relogioFixo).MetasVendedores(ctx, e)
	require.NoError(t, err)
	require.Len(t, metas, total)
	assert.Equal(t, 10.0, metas[0].Realizado)
	assert.Equal(t, 20.0, metas[total-1].Realizado)
	assert.NoError(t, mock.ExpectationsWereMet(), "dois SELECTs: 100 + 50 placeholders")
}

func TestBUG18_TabelaAusente_SoCodigo1146(t *testing.T) {
	ctx := context.Background()
	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	e, _ := kernel.EscopoIrrestrito(admin)
	hoje := relogioFixo

	t.Run("1146 degrada para série zerada", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(`AS data`).WithArgs("2026-10-13").WillReturnError(&mysql.MySQLError{Number: 1146, Message: "Table 'x.pedidos' doesn't exist"})
		pontos, err := analyticsmysql.NovoDashboard(db, hoje).SerieVendas(ctx, e, analytics.DiasDe(2))
		require.NoError(t, err)
		assert.Equal(t, analytics.SerieZerada(2, hojeFixo), pontos)
	})
	for nome, errBanco := range map[string]error{
		"texto 'not found' sem código":     errors.New("column not found"),
		"texto 'doesn't exist' sem código": errors.New("Error 1146: Table 'x.pedidos' doesn't exist"),
		"outro código do driver":           &mysql.MySQLError{Number: 1054, Message: "Unknown column 'x' not found"},
	} {
		t.Run(nome+" é propagado", func(t *testing.T) {
			db, mock := novoMock(t)
			mock.ExpectQuery(`AS data`).WithArgs("2026-10-13").WillReturnError(errBanco)
			_, err := analyticsmysql.NovoDashboard(db, hoje).SerieVendas(ctx, e, analytics.DiasDe(2))
			assert.Error(t, err)
		})
	}
}
