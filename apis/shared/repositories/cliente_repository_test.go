// Package repositories_test contém testes de integração com banco de dados mockado.
// Cada função é stateless — o sqlmock substitui o driver real.
package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

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
	mock.ExpectQuery(`SELECT ` + clienteColunasRegexp + ` FROM clientes ` + whereRegexp + ` ORDER BY cliente_id_origem ASC LIMIT \? OFFSET \?`).
		WithArgs(int64(7), 10, 0).
		WillReturnRows(sqlmock.NewRows(clienteRepoColumns).
			AddRow(int64(1), "12345678000199", "Empresa Teste", "varejo", "SP", "SP", "Centro", now, true, now, now))

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	clientes, total, err := repo.List(ctx, db, 1, 10, repositories.ClienteFiltro{VendedorID: 7})

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
	mock.ExpectQuery(`SELECT ` + clienteColunasRegexp + ` FROM clientes ORDER BY cliente_id_origem ASC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(clienteRepoColumns))

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, total, err := repo.List(ctx, db, 1, 10, repositories.ClienteFiltro{})

	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.NoError(t, mock.ExpectationsWereMet())
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
