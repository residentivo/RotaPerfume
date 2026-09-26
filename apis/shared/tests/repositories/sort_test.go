package repositories_test

// Ordenação dinâmica (order_by / order_dir) testada pela API pública: o helper
// interno buildOrderByClause é exercitado via ProdutoRepository.List (default
// "id ASC", colunas iguais aos campos) e SenhaHistoricoRepository.FindAll
// (default "sh.id DESC", campos mapeados para colunas com alias). O que se
// verifica é a cláusula ORDER BY efetivamente enviada ao banco.

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

// orderByDe extrai o trecho "ORDER BY ... " (até antes de LIMIT) do SQL.
func orderByDe(t *testing.T, q string) string {
	t.Helper()
	q = normalizeSQL(q)
	ini := strings.Index(q, "ORDER BY ")
	require.GreaterOrEqual(t, ini, 0, "SQL sem ORDER BY: %s", q)
	fim := strings.Index(q[ini:], " LIMIT")
	require.GreaterOrEqual(t, fim, 0, "SQL sem LIMIT após ORDER BY: %s", q)
	return q[ini : ini+fim]
}

var casosOrdenacaoProduto = []struct {
	nome     string
	orderBy  string
	orderDir string
	want     string
}{
	{"tudo vazio usa default", "", "", "ORDER BY id ASC"},
	{"campo válido asc", "sku", "asc", "ORDER BY sku ASC"},
	{"campo válido desc", "descricao", "desc", "ORDER BY descricao DESC"},
	{"DESC maiúsculo", "marca", "DESC", "ORDER BY marca DESC"},
	{"case misto em dir (DeSc)", "marca", "DeSc", "ORDER BY marca DESC"},
	{"campo case-insensitive", "PRECO_TABELA", "asc", "ORDER BY preco_tabela ASC"},
	{"espaços em volta", "  categoria  ", "  desc  ", "ORDER BY categoria DESC"},
	{"campo fora da whitelist cai no default", "senha_hash", "desc", "ORDER BY id DESC"},
	{"dir inválida cai no default", "sku", "sideways", "ORDER BY sku ASC"},
	{"SQL injection em order_by", "1; DROP TABLE produtos;--", "asc", "ORDER BY id ASC"},
	{"subquery em order_by", "(SELECT 1)", "", "ORDER BY id ASC"},
	{"SQL injection em order_dir", "sku", "asc; DROP TABLE x;--", "ORDER BY sku ASC"},
	{"injection nos dois", "1); DROP TABLE x;--", "asc; DROP TABLE x;--", "ORDER BY id ASC"},
}

func TestOrdenacao_ProdutoList(t *testing.T) {
	for _, tt := range casosOrdenacaoProduto {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock, captured := newCapturingMock(t)
			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).
				WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
			mock.ExpectQuery(`FROM produtos`).
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows([]string{"id"}))

			_, _, err := repositories.NewProdutoRepository().List(context.Background(), db, 1, 10,
				repositories.ProdutoFiltro{OrderBy: tt.orderBy, OrderDir: tt.orderDir})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())

			lista := (*captured)[len(*captured)-1]
			got := orderByDe(t, lista)
			assert.Equal(t, tt.want, got)
			assert.NotContains(t, lista, "DROP")
			assert.NotContains(t, lista, "SELECT 1")
		})
	}
}

func TestOrdenacao_SenhaHistoricoFindAll(t *testing.T) {
	casos := []struct {
		nome     string
		orderBy  string
		orderDir string
		want     string
	}{
		{"tudo vazio usa default DESC", "", "", "ORDER BY sh.id DESC"},
		{"campo mapeado para alias", "usuario_id", "asc", "ORDER BY sh.usuario_id ASC"},
		{"campo mapeado com dir default", "created_at", "", "ORDER BY sh.created_at DESC"},
		{"campo fora da whitelist", "senha_hash_anterior", "asc", "ORDER BY sh.id ASC"},
		{"dir inválida cai no default DESC", "tipo_reset", "up", "ORDER BY sh.tipo_reset DESC"},
		{"injection", "id; DELETE FROM usuarios", "desc", "ORDER BY sh.id DESC"},
	}
	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock, captured := newCapturingMock(t)
			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico`).
				WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
			mock.ExpectQuery(`FROM senha_historico sh`).
				WithArgs(20, 0).
				WillReturnRows(sqlmock.NewRows([]string{"id"}))

			_, _, err := repositories.NewSenhaHistoricoRepository().FindAll(context.Background(), db, 1, 20, tt.orderBy, tt.orderDir)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())

			lista := (*captured)[len(*captured)-1]
			assert.Equal(t, tt.want, orderByDe(t, lista))
			assert.NotContains(t, lista, "DELETE")
		})
	}
}
