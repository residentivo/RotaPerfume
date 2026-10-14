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

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/kernel"
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

			// ARQ-02: order_by/order_dir viram o enum CampoProduto no parse
			// (kernel.ParseOrdenacao, como o catalogohttp) e a coluna sai do
			// switch do adapter. A asserção sobre o SQL é a mesma.
			_, _, err := listarProdutos(catalogomysql.NovoProdutos(db), context.Background(),
				catalogo.FiltroProduto{Ordenacao: ordenacaoProduto(tt.orderBy, tt.orderDir)})
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
		{"campo mapeado para alias", "usuario_id", "asc", "ORDER BY sh.usuario_id ASC, sh.id DESC"},
		{"campo mapeado com dir default", "created_at", "", "ORDER BY sh.created_at DESC, sh.id DESC"},
		{"campo fora da whitelist", "senha_hash_anterior", "asc", "ORDER BY sh.id ASC"},
		{"dir inválida cai no default DESC", "tipo_reset", "up", "ORDER BY sh.tipo_reset DESC, sh.id DESC"},
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

			_, _, err := novoHistoricoRepo().FindAll(context.Background(), db, 1, 20, tt.orderBy, tt.orderDir)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())

			lista := (*captured)[len(*captured)-1]
			assert.Equal(t, tt.want, orderByDe(t, lista))
			assert.NotContains(t, lista, "DELETE")
		})
	}
}

// casosOrdenacaoEstoque: as mesmas classes de entrada do produto (default,
// válidos, maiúsculas, espaços, fora da whitelist e injection) contra a
// whitelist de estoque. col é a coluna SEM alias; o default é
// "data_snapshot DESC".
var casosOrdenacaoEstoque = []struct {
	nome     string
	orderBy  string
	orderDir string
	col, dir string
}{
	{"tudo vazio usa default", "", "", "data_snapshot", "DESC"},
	{"campo válido asc", "saldo", "asc", "saldo", "ASC"},
	{"campo válido desc", "sku", "desc", "sku", "DESC"},
	{"id", "id", "asc", "id", "ASC"},
	{"ruptura", "ruptura", "", "ruptura", "DESC"},
	{"created_at", "created_at", "asc", "created_at", "ASC"},
	{"updated_at", "UPDATED_AT", "asc", "updated_at", "ASC"},
	{"espaços em volta", "  data_snapshot  ", "  asc  ", "data_snapshot", "ASC"},
	{"campo de produtos não vale em estoque", "preco_tabela", "asc", "data_snapshot", "ASC"},
	{"coluna com alias não é aceita", "e.saldo", "asc", "data_snapshot", "ASC"},
	{"dir inválida cai no default", "saldo", "sideways", "saldo", "DESC"},
	{"SQL injection em order_by", "1; DROP TABLE estoque;--", "asc", "data_snapshot", "ASC"},
	{"subquery em order_by", "(SELECT 1)", "", "data_snapshot", "DESC"},
	{"SQL injection em order_dir", "saldo", "asc; DROP TABLE x;--", "saldo", "DESC"},
	{"injection nos dois", "1); DROP TABLE x;--", "desc; DROP TABLE x;--", "data_snapshot", "DESC"},
}

// TestOrdenacao_EstoqueHistorico: whitelist da série histórica (colunas com
// o alias "e." da tabela estoque, C-02).
func TestOrdenacao_EstoqueHistorico(t *testing.T) {
	for _, tt := range casosOrdenacaoEstoque {
		t.Run(tt.nome, func(t *testing.T) {
			lista := capturarListagemEstoque(t, true, ordenacaoEstoque(tt.orderBy, tt.orderDir))
			assert.Equal(t, "ORDER BY e."+tt.col+" "+tt.dir, orderByDe(t, lista))
			assert.NotContains(t, lista, "DROP")
			assert.NotContains(t, lista, "SELECT 1")
		})
	}
}

// TestOrdenacao_EstoqueUltimaPosicao: segunda whitelist, sobre a tabela
// derivada "ranked" (colunas sem alias). O ORDER BY interno da window
// function é fixo e não depende do order_by.
func TestOrdenacao_EstoqueUltimaPosicao(t *testing.T) {
	for _, tt := range casosOrdenacaoEstoque {
		t.Run(tt.nome, func(t *testing.T) {
			lista := capturarListagemEstoque(t, false, ordenacaoEstoque(tt.orderBy, tt.orderDir))
			fim := strings.Index(lista, "ranked WHERE rn = 1 ")
			require.GreaterOrEqual(t, fim, 0, lista)
			assert.Equal(t, "ORDER BY "+tt.col+" "+tt.dir, orderByDe(t, lista[fim:]))
			assert.Contains(t, lista, "PARTITION BY e.sku ORDER BY e.data_snapshot DESC, e.id DESC")
			assert.NotContains(t, lista, "DROP")
			assert.NotContains(t, lista, "SELECT 1")
		})
	}
}

// TestOrdenacao_EnumForaDaWhitelist: um valor do enum fabricado fora do parse
// (conversão direta de string) e a ordenação zero value não chegam ao SQL:
// o switch do adapter cai no default de cada entidade (defesa em
// profundidade da C-02).
func TestOrdenacao_EnumForaDaWhitelist(t *testing.T) {
	t.Run("produto", func(t *testing.T) {
		for _, c := range []struct {
			o    kernel.Ordenacao[catalogo.CampoProduto]
			want string
		}{
			{kernel.NovaOrdenacao(catalogo.CampoProduto("id; DROP TABLE produtos"), kernel.Desc), "ORDER BY id DESC"},
			{kernel.Ordenacao[catalogo.CampoProduto]{}, "ORDER BY id ASC"}, // zero value: padrão da entidade
		} {
			db, mock, captured := newCapturingMock(t)
			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
			mock.ExpectQuery(`FROM produtos`).WithArgs(10, 0).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			_, _, err := listarProdutos(catalogomysql.NovoProdutos(db), context.Background(), catalogo.FiltroProduto{Ordenacao: c.o})
			require.NoError(t, err)
			lista := (*captured)[len(*captured)-1]
			assert.Equal(t, c.want, orderByDe(t, lista))
			assert.NotContains(t, lista, "DROP")
		}
	})
	t.Run("estoque", func(t *testing.T) {
		o := kernel.NovaOrdenacao(catalogo.CampoEstoque("saldo; DROP TABLE estoque"), kernel.Asc)
		assert.Equal(t, "ORDER BY e.data_snapshot ASC", orderByDe(t, capturarListagemEstoque(t, true, o)))
		assert.Equal(t, "ORDER BY e.data_snapshot DESC", orderByDe(t, capturarListagemEstoque(t, true, kernel.Ordenacao[catalogo.CampoEstoque]{})))
	})
}

// capturarListagemEstoque roda Historico (historico=true) ou UltimaPosicao e
// devolve o SQL da listagem enviado ao banco.
func capturarListagemEstoque(t *testing.T, historico bool, o kernel.Ordenacao[catalogo.CampoEstoque]) string {
	t.Helper()
	db, mock, captured := newCapturingMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\)`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	mock.ExpectQuery(`FROM`).WithArgs(10, 0).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	repo := catalogomysql.NovoEstoques(db)
	listar := repo.UltimaPosicao
	if historico {
		listar = repo.Historico
	}
	_, _, err := listarEstoque(listar, context.Background(), catalogo.FiltroEstoque{Ordenacao: o})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	return (*captured)[len(*captured)-1]
}
