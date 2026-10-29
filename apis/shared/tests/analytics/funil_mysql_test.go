package analytics_test

// DASH-01 (adapter MySQL): o SQL do funil é a concatenação de constantes
// (janela + filtro de vendedor) e o vendedor vai só por placeholder.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsmysql"
	"github.com/rotaperfumes/shared/kernel"
)

// novoMockExato compara o SQL inteiro (espaços normalizados pelo sqlmock).
func novoMockExato(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

const (
	sqlFunilBase = "SELECT etapa, COUNT(*), COALESCE(SUM(valor_estimado), 0) FROM oportunidades WHERE 1=1"
	sqlFunilFim  = " GROUP BY etapa"
)

var funilCols = []string{"etapa", "quantidade", "valor"}

func TestDASH01_Oportunidades_ExigeQuerier(t *testing.T) {
	assert.PanicsWithValue(t, "analyticsmysql: NovoOportunidades exige Querier", func() { analyticsmysql.NovoOportunidades(nil) })
}

func TestDASH01_Oportunidades_Funil_SQLPorEscopoEJanela(t *testing.T) {
	doVendedor, err := kernel.EscopoDoVendedor(42)
	require.NoError(t, err)
	janelas := []struct {
		j        analytics.Janela
		clausula string
	}{
		{analytics.JanelaTudo, ""},
		{analytics.Janela30d, " AND data_abertura >= DATE_SUB(CURDATE(), INTERVAL 29 DAY)"},
		{analytics.Janela90d, " AND data_abertura >= DATE_SUB(CURDATE(), INTERVAL 89 DAY)"},
		{analytics.Janela365d, " AND data_abertura >= DATE_SUB(CURDATE(), INTERVAL 364 DAY)"},
		{analytics.Janela(99), ""}, // fora do enum: default all, sem cláusula
	}
	escopos := []struct {
		nome   string
		e      kernel.Escopo
		filtro string
		args   []driver.Value
	}{
		{"admin", irrestrito(t), "", nil},
		{"vendedor", doVendedor, " AND vendedor_id = ?", []driver.Value{int64(42)}},
	}
	for _, esc := range escopos {
		for _, jan := range janelas {
			t.Run(esc.nome+"/"+jan.j.String(), func(t *testing.T) {
				db, mock := novoMockExato(t)
				exp := mock.ExpectQuery(sqlFunilBase + jan.clausula + esc.filtro + sqlFunilFim)
				if esc.args == nil {
					exp = exp.WithoutArgs()
				} else {
					exp = exp.WithArgs(esc.args...)
				}
				exp.WillReturnRows(sqlmock.NewRows(funilCols).
					AddRow(analytics.EtapaProspeccao, 3, 300.5).
					AddRow(analytics.EtapaPerdido, 1, nil)) // SUM NULL → 0

				got, err := analyticsmysql.NovoOportunidades(db).Funil(context.Background(), esc.e, jan.j)
				require.NoError(t, err)
				assert.Equal(t, []analytics.ContagemEtapa{
					{Etapa: analytics.EtapaProspeccao, Quantidade: 3, Valor: 300.5},
					{Etapa: analytics.EtapaPerdido, Quantidade: 1, Valor: 0},
				}, got)
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

func TestDASH01_Oportunidades_Funil_SemLinhasListaVazia(t *testing.T) {
	db, mock := novoMockExato(t)
	mock.ExpectQuery(sqlFunilBase + sqlFunilFim).WillReturnRows(sqlmock.NewRows(funilCols))
	got, err := analyticsmysql.NovoOportunidades(db).Funil(context.Background(), irrestrito(t), analytics.JanelaTudo)
	require.NoError(t, err)
	assert.NotNil(t, got, "lista vazia, nunca nil")
	assert.Empty(t, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// O1: SemAcesso e zero value → o erro do Filtro, com ZERO queries.
func TestDASH01_Oportunidades_Funil_SemAcessoSemSQL(t *testing.T) {
	casos := []struct {
		nome string
		e    kernel.Escopo
		want error
	}{
		{"sem acesso", kernel.EscopoSemAcesso(), kernel.ErrEscopoSemAcesso},
		{"zero value", kernel.Escopo{}, kernel.ErrEscopoInvalido},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := novoMockExato(t)
			got, err := analyticsmysql.NovoOportunidades(db).Funil(context.Background(), c.e, analytics.Janela30d)
			assert.ErrorIs(t, err, c.want)
			assert.Nil(t, got)
			assert.NoError(t, mock.ExpectationsWereMet(), "nenhuma query")
		})
	}
}

func TestDASH01_Oportunidades_Funil_ErrosDeInfra(t *testing.T) {
	errBanco := errors.New("Error 1146: Table 'rota.oportunidades' doesn't exist")
	casos := []struct {
		nome    string
		preparo func(sqlmock.Sqlmock)
		msg     string
		causa   error
	}{
		{"erro de query", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(sqlFunilBase + sqlFunilFim).WillReturnError(errBanco)
		}, "analyticsmysql: funil:", errBanco},
		{"erro de scan (quantidade não numérica)", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(sqlFunilBase + sqlFunilFim).WillReturnRows(sqlmock.NewRows(funilCols).AddRow(analytics.EtapaGanho, "x", 1.0))
		}, "analyticsmysql: funil scan:", nil},
		{"erro de rows.Err", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(sqlFunilBase + sqlFunilFim).WillReturnRows(sqlmock.NewRows(funilCols).
				AddRow(analytics.EtapaGanho, 1, 1.0).AddRow(analytics.EtapaPerdido, 1, 1.0).RowError(1, errBanco))
		}, "analyticsmysql: funil rows:", errBanco},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := novoMockExato(t)
			c.preparo(mock)
			got, err := analyticsmysql.NovoOportunidades(db).Funil(context.Background(), irrestrito(t), analytics.JanelaTudo)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.msg)
			if c.causa != nil {
				assert.ErrorIs(t, err, c.causa)
			}
			assert.Nil(t, got, "sem resposta parcial")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
