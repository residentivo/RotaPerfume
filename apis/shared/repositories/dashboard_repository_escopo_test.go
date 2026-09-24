package repositories_test

// Testes do escopo por vendedor do DashboardRepository: com vendedorID > 0
// (usuário normal) toda query recebe o filtro "AND <coluna> = ?" e o
// argumento na posição correta (antes de LIMIT/OFFSET); com vendedorID = 0
// (admin) a query sai sem filtro e sem argumento extra.

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

// newCapturingMock cria um sqlmock com matcher regexp que também registra o
// SQL efetivamente executado, permitindo asserções sobre o texto da query.
func newCapturingMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *[]string) {
	t.Helper()
	captured := []string{}
	matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		captured = append(captured, actualSQL)
		return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock, &captured
}

// normalizeSQL colapsa espaços em branco para facilitar asserções textuais.
func normalizeSQL(q string) string {
	return strings.Join(strings.Fields(q), " ")
}

// assertFiltro verifica presença/ausência do filtro de vendedor no SQL e, se
// informado, que ele aparece antes do marcador (ORDER BY / LIMIT / GROUP BY).
func assertFiltro(t *testing.T, sqlText, filtro string, esperaFiltro bool, antesDe string) {
	t.Helper()
	q := normalizeSQL(sqlText)
	if !esperaFiltro {
		assert.NotContains(t, q, filtro, "admin não deve receber filtro de vendedor")
		assert.NotContains(t, q, "vendedor_id = ?")
		assert.NotContains(t, q, " id = ?")
		return
	}
	idx := strings.Index(q, filtro)
	require.GreaterOrEqual(t, idx, 0, "filtro %q ausente em: %s", filtro, q)
	assert.Equal(t, 1, strings.Count(q, filtro), "filtro deve aparecer uma única vez")
	if antesDe != "" {
		posMarcador := strings.Index(q, antesDe)
		require.GreaterOrEqual(t, posMarcador, 0)
		assert.Less(t, idx, posMarcador, "filtro deve vir antes de %q", antesDe)
	}
}

var escopoCasos = []struct {
	nome       string
	vendedorID int64
	filtra     bool
}{
	{"admin (vendedorID=0) sem filtro", 0, false},
	{"normal (vendedorID=7) com filtro", 7, true},
	{"vendedorID negativo tratado como admin", -1, false},
}

func TestDashboardEscopo_GetVendasTotais(t *testing.T) {
	for _, tt := range escopoCasos {
		for _, periodo := range []string{"today", "week", "month"} {
			t.Run(tt.nome+"/"+periodo, func(t *testing.T) {
				db, mock, captured := newCapturingMock(t)
				exp := mock.ExpectQuery(`SELECT COALESCE\(SUM\(valor_total\), 0\), COUNT\(\*\)\s+FROM pedidos`)
				if tt.filtra {
					exp.WithArgs(tt.vendedorID)
				} else {
					exp.WithoutArgs()
				}
				exp.WillReturnRows(sqlmock.NewRows([]string{"v", "q"}).AddRow(50.0, 1))

				valor, qtd, err := repositories.NewDashboardRepository().GetVendasTotais(context.Background(), db, periodo, tt.vendedorID)
				require.NoError(t, err)
				assert.Equal(t, 50.0, valor)
				assert.Equal(t, 1, qtd)
				require.NotEmpty(t, *captured)
				assertFiltro(t, (*captured)[0], "AND status NOT IN ('cancelado', 'devolvido') AND vendedor_id = ?", tt.filtra, "")
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

func TestDashboardEscopo_GetTotalPedidos(t *testing.T) {
	for _, tt := range escopoCasos {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock, captured := newCapturingMock(t)
			exp := mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos\s+WHERE`)
			if tt.filtra {
				exp.WithArgs(tt.vendedorID)
			} else {
				exp.WithoutArgs()
			}
			exp.WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(3))

			total, err := repositories.NewDashboardRepository().GetTotalPedidos(context.Background(), db, "month", tt.vendedorID)
			require.NoError(t, err)
			assert.Equal(t, 3, total)
			assertFiltro(t, (*captured)[0], "AND vendedor_id = ?", tt.filtra, "")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardEscopo_GetMetaMensalTotal(t *testing.T) {
	for _, tt := range escopoCasos {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock, captured := newCapturingMock(t)
			exp := mock.ExpectQuery(`SELECT COALESCE\(SUM\(meta_mensal\), 0\)\s+FROM vendedores`)
			if tt.filtra {
				exp.WithArgs(tt.vendedorID)
			} else {
				exp.WithoutArgs()
			}
			exp.WillReturnRows(sqlmock.NewRows([]string{"m"}).AddRow(1000.0))

			meta, err := repositories.NewDashboardRepository().GetMetaMensalTotal(context.Background(), db, tt.vendedorID)
			require.NoError(t, err)
			assert.Equal(t, 1000.0, meta)
			assertFiltro(t, (*captured)[0], "WHERE data_desligamento IS NULL AND id = ?", tt.filtra, "")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardEscopo_GetTopVendedores(t *testing.T) {
	for _, tt := range escopoCasos {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock, captured := newCapturingMock(t)
			exp := mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.meta_mensal AS meta\s+FROM vendedores v`)
			if tt.filtra {
				// Filtro antes do LIMIT: (vendedorID, limit).
				exp.WithArgs(tt.vendedorID, 10)
			} else {
				exp.WithArgs(10)
			}
			exp.WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "meta"}))

			res, err := repositories.NewDashboardRepository().GetTopVendedores(context.Background(), db, 10, tt.vendedorID)
			require.NoError(t, err)
			assert.Empty(t, res)
			assertFiltro(t, (*captured)[0], "WHERE v.data_desligamento IS NULL AND v.id = ?", tt.filtra, "ORDER BY")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardEscopo_GetMetasVendedores(t *testing.T) {
	for _, tt := range escopoCasos {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock, captured := newCapturingMock(t)
			exp := mock.ExpectQuery(`SELECT\s+v\.id,\s+v\.nome,\s+v\.regiao,\s+v\.uf,\s+v\.meta_mensal AS meta\s+FROM vendedores v`)
			if tt.filtra {
				exp.WithArgs(tt.vendedorID)
			} else {
				exp.WithoutArgs()
			}
			exp.WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "meta"}))

			res, err := repositories.NewDashboardRepository().GetMetasVendedores(context.Background(), db, tt.vendedorID)
			require.NoError(t, err)
			assert.Empty(t, res)
			assertFiltro(t, (*captured)[0], "WHERE v.data_desligamento IS NULL AND v.id = ?", tt.filtra, "ORDER BY")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardEscopo_GetVendasSeries(t *testing.T) {
	for _, tt := range escopoCasos {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock, captured := newCapturingMock(t)
			exp := mock.ExpectQuery(`SELECT DATE_FORMAT\(data_pedido, '%Y-%m-%d'\) AS data`)
			if tt.filtra {
				// dias primeiro (INTERVAL ? DAY), depois o vendedor.
				exp.WithArgs(15, tt.vendedorID)
			} else {
				exp.WithArgs(15)
			}
			exp.WillReturnRows(sqlmock.NewRows([]string{"data", "valor", "quantidade"}))

			serie, err := repositories.NewDashboardRepository().GetVendasSeries(context.Background(), db, 15, tt.vendedorID)
			require.NoError(t, err)
			assert.Len(t, serie, 15)
			assertFiltro(t, (*captured)[0], "AND status NOT IN ('cancelado', 'devolvido') AND vendedor_id = ?", tt.filtra, "GROUP BY")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardEscopo_GetVendedoresRanking(t *testing.T) {
	casos := []struct {
		nome       string
		vendedorID int64
		filtra     bool
		page       int
		limit      int
		offset     int
	}{
		{"admin página 1", 0, false, 1, 20, 0},
		{"admin página 3", 0, false, 3, 10, 20},
		{"normal página 1", 7, true, 1, 20, 0},
		{"normal página 2", 7, true, 2, 5, 5},
	}
	cols := []string{"id", "nome", "regiao", "uf", "meta_mensal", "total_vendas", "total_pedidos", "atingimento_meta"}

	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock, captured := newCapturingMock(t)
			count := mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL`)
			lista := mock.ExpectQuery(`(?s)SELECT.*FROM vendedores v.*LIMIT \? OFFSET \?`)
			rows := sqlmock.NewRows(cols)
			total := 3
			if tt.filtra {
				count.WithArgs(tt.vendedorID)
				// Filtro antes de LIMIT/OFFSET.
				lista.WithArgs(tt.vendedorID, tt.limit, tt.offset)
				total = 1
				rows.AddRow(tt.vendedorID, "B", "Sul", "PR", 100.0, 50.0, 1, 50.0)
			} else {
				count.WithoutArgs()
				lista.WithArgs(tt.limit, tt.offset)
			}
			count.WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(total))
			lista.WillReturnRows(rows)

			res, gotTotal, err := repositories.NewDashboardRepository().GetVendedoresRanking(context.Background(), db, tt.page, tt.limit, tt.vendedorID)
			require.NoError(t, err)
			assert.Equal(t, total, gotTotal)
			require.Len(t, *captured, 2)
			assertFiltro(t, (*captured)[0], "WHERE data_desligamento IS NULL AND id = ?", tt.filtra, "")
			assertFiltro(t, (*captured)[1], "WHERE v.data_desligamento IS NULL AND v.id = ?", tt.filtra, "ORDER BY")
			if tt.filtra {
				require.Len(t, res, 1)
				assert.Equal(t, tt.vendedorID, res[0]["vendedor_id"])
			} else {
				assert.Empty(t, res)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDashboardEscopo_GetVendedoresRanking_CountError_ComFiltro(t *testing.T) {
	db, mock, _ := newCapturingMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM vendedores WHERE data_desligamento IS NULL AND id = \?`).
		WithArgs(int64(7)).
		WillReturnError(sql.ErrConnDone)

	_, _, err := repositories.NewDashboardRepository().GetVendedoresRanking(context.Background(), db, 1, 20, 7)
	require.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrConnDone)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDashboardEmptySeries(t *testing.T) {
	isoDate := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	casos := []struct {
		nome string
		dias int
	}{
		{"zero dias", 0},
		{"um dia", 1},
		{"sete dias", 7},
		{"trinta dias", 30},
		{"365 dias", 365},
	}
	repo := repositories.NewDashboardRepository()
	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			serie := repo.EmptySeries(tt.dias)
			require.NotNil(t, serie)
			require.Len(t, serie, tt.dias)
			for i, p := range serie {
				assert.Len(t, p, 3)
				assert.Regexp(t, isoDate, p["dia"])
				assert.Equal(t, 0.0, p["total_vendas"])
				assert.Equal(t, 0, p["total_pedidos"])
				if i > 0 {
					assert.Less(t, serie[i-1]["dia"].(string), p["dia"].(string), "ordem crescente")
				}
			}
			if tt.dias > 0 {
				assert.Equal(t, time.Now().Format("2006-01-02"), serie[tt.dias-1]["dia"], "último ponto é hoje")
			}
		})
	}
}
