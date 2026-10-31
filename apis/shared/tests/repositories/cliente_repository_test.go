// Package repositories_test contém testes de integração com banco de dados mockado.
// Cada função é stateless — o sqlmock substitui o driver real.
package repositories_test

import (
	"context"
	"database/sql"
	"github.com/rotaperfumes/shared/analytics/analyticsmysql"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/kernel"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

			ctx := context.Background()
			total, err := analyticsmysql.NovoClientes(db).NovosNoPeriodo(ctx, escopoDe(0), periodoDe(tt.periodo))

			require.NoError(t, err)
			assert.Equal(t, 3, total)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

const clienteColunasRegexp = `cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, COALESCE\(bairro, ''\), data_cadastro, ativo, created_at, updated_at`

var clienteRepoColumns = []string{
	"cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
	"data_cadastro", "ativo", "created_at", "updated_at",
}

// TestClienteList_FiltroVendedorID garante que VendedorID > 0 restringe a
// listagem aos clientes na carteira ativa desse vendedor (ver item 12 do
// relatório de segurança — filtro server-side por carteira).
func TestClienteList_FiltroVendedorID(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	whereRegexp := `WHERE cliente_id_origem IN \(SELECT cliente_id FROM carteiras WHERE vendedor_id = \? AND data_fim IS NULL\)`

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes ` + whereRegexp).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))

	now := time.Now()
	mock.ExpectQuery(`SELECT `+clienteColunasRegexp+` FROM clientes `+whereRegexp+` ORDER BY cliente_id_origem ASC LIMIT \? OFFSET \?`).
		WithArgs(int64(7), 10, 0).
		WillReturnRows(sqlmock.NewRows(clienteRepoColumns).
			AddRow(int64(1), "12345678000199", "Empresa Teste", "varejo", "SP", "SP", "Centro", now, true, now, now))

	ctx := context.Background()
	clientes, total, err := itensETotal(comercialmysql.NovoClientes(db).Listar(ctx, escopoDe(7), comercial.FiltroCliente{Ordenacao: comercial.OrdenacaoClientePadrao()}, kernel.NovaPagina(1, 10)))

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, clientes, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestClienteList_SemFiltroVendedorID garante que VendedorID=0 (não
// informado) não adiciona restrição de carteira à query.
func TestClienteList_SemFiltroVendedorID(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes$`).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT `+clienteColunasRegexp+` FROM clientes ORDER BY cliente_id_origem ASC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(clienteRepoColumns))

	ctx := context.Background()
	_, total, err := itensETotal(comercialmysql.NovoClientes(db).Listar(ctx, escopoAdmin(), comercial.FiltroCliente{Ordenacao: comercial.OrdenacaoClientePadrao()}, kernel.NovaPagina(1, 10)))

	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestClienteList_BuscaQCNPJ: NEG-02 — QCNPJ substitui Q apenas no LIKE de
// cnpj; vazio, o LIKE de cnpj usa o próprio Q.
func TestClienteList_BuscaQCNPJ(t *testing.T) {
	cases := []struct {
		nome        string
		filtro      comercial.FiltroCliente
		wantLikeRS  string
		wantLikeDoc string
	}{
		{"sem QCNPJ usa Q", comercial.FiltroCliente{Busca: "Teste", Ordenacao: comercial.OrdenacaoClientePadrao()}, "%Teste%", "%Teste%"},
		{"com QCNPJ", comercial.FiltroCliente{Busca: "12.abc.345", Ordenacao: comercial.OrdenacaoClientePadrao()}, "%12.abc.345%", "%12ABC345%"},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			where := `WHERE \(razao_social LIKE \? ESCAPE '!' OR cnpj LIKE \? ESCAPE '!'\)`
			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes `+where).
				WithArgs(tc.wantLikeRS, tc.wantLikeDoc).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery(`SELECT `+clienteColunasRegexp+` FROM clientes `+where+` ORDER BY cliente_id_origem ASC LIMIT \? OFFSET \?`).
				WithArgs(tc.wantLikeRS, tc.wantLikeDoc, 10, 0).
				WillReturnRows(sqlmock.NewRows(clienteRepoColumns))

			_, _, err := itensETotal(comercialmysql.NovoClientes(db).Listar(context.Background(), escopoAdmin(), tc.filtro, kernel.NovaPagina(1, 10)))

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

const carteiraAtivaWhereRegexp = `cliente_id_origem IN \(SELECT cliente_id FROM carteiras WHERE vendedor_id = \? AND data_fim IS NULL\)`

// TestClienteContagens_EscopoCarteira garante que vendedorID > 0 aplica o
// mesmo predicado de carteira ativa da listagem em todas as contagens do
// dashboard (combinado com AND onde já existe WHERE), sempre por
// placeholder; e que vendedorID = 0 (admin) não adiciona restrição.
func TestClienteContagens_EscopoCarteira(t *testing.T) {
	ctx := context.Background()

	t.Run("CountTotal", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ` + carteiraAtivaWhereRegexp + `$`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(4))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes$`).
			WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(40))

		total, err := analyticsmysql.NovoClientes(db).Total(ctx, escopoDe(7))
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		total, err = analyticsmysql.NovoClientes(db).Total(ctx, escopoDe(0))
		require.NoError(t, err)
		assert.Equal(t, 40, total)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CountPorAtivo", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \? AND `+carteiraAtivaWhereRegexp+`$`).
			WithArgs(true, int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(3))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE ativo = \?$`).
			WithArgs(false).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(9))

		total, err := analyticsmysql.NovoClientes(db).PorAtivo(ctx, escopoDe(7), true)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		total, err = analyticsmysql.NovoClientes(db).PorAtivo(ctx, escopoDe(0), false)
		require.NoError(t, err)
		assert.Equal(t, 9, total)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CountNovosNoPeriodo", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE data_cadastro = CURDATE\(\) AND ` + carteiraAtivaWhereRegexp + `$`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))

		total, err := analyticsmysql.NovoClientes(db).NovosNoPeriodo(ctx, escopoDe(7), periodoDe("today"))
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CountPorSegmento", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectQuery(`SELECT segmento, COUNT\(\*\) AS total FROM clientes WHERE ` + carteiraAtivaWhereRegexp + ` GROUP BY segmento ORDER BY total DESC`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"segmento", "total"}).AddRow("varejo", 2))
		mock.ExpectQuery(`SELECT segmento, COUNT\(\*\) AS total FROM clientes GROUP BY segmento ORDER BY total DESC`).
			WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"segmento", "total"}))

		out, err := analyticsmysql.NovoClientes(db).PorSegmento(ctx, escopoDe(7))
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, "varejo", out[0].Segmento)

		vazio, err := analyticsmysql.NovoClientes(db).PorSegmento(ctx, escopoDe(0))
		require.NoError(t, err)
		require.NotNil(t, vazio, "sem linhas deve devolver slice vazio, nunca nil")
		assert.Len(t, vazio, 0)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CountPorUF", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectQuery(`SELECT uf, COUNT\(\*\) AS total FROM clientes WHERE ` + carteiraAtivaWhereRegexp + ` GROUP BY uf ORDER BY total DESC`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"uf", "total"}))
		mock.ExpectQuery(`SELECT uf, COUNT\(\*\) AS total FROM clientes GROUP BY uf ORDER BY total DESC`).
			WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"uf", "total"}).AddRow("SP", 5))

		vazio, err := analyticsmysql.NovoClientes(db).PorUF(ctx, escopoDe(7))
		require.NoError(t, err)
		require.NotNil(t, vazio, "sem linhas deve devolver slice vazio, nunca nil")
		assert.Len(t, vazio, 0)

		out, err := analyticsmysql.NovoClientes(db).PorUF(ctx, escopoDe(0))
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, "SP", out[0].UF)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestClienteCountNovosNoPeriodo_DBError garante que erros do banco são
// propagados (sem fallback silencioso, diferente do dashboard_repository).
func TestClienteCountNovosNoPeriodo_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE`).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, err := analyticsmysql.NovoClientes(db).NovosNoPeriodo(ctx, escopoDe(0), periodoDe("week"))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
