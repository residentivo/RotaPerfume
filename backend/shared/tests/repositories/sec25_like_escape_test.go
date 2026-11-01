package repositories_test

// SEC-25 (Lote 27): toda busca "contém" usa `LIKE ? ESCAPE '!'` com o termo
// de db.TermoContem: '%', '_' e '!' digitados são literais. Um adapter por
// caso; o SQL e os argumentos são capturados das duas consultas (COUNT e
// SELECT paginado).

import (
	"context"
	"database/sql/driver"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

// sec25Captura guarda o SQL e os argumentos string de cada consulta.
type sec25Captura struct {
	sqls []string
	args []string
}

type sec25Arg struct{ c *sec25Captura }

func (a sec25Arg) Match(v driver.Value) bool {
	if s, ok := v.(string); ok {
		a.c.args = append(a.c.args, s)
	}
	return true
}

// sec25Mock aceita as duas consultas da listagem (COUNT → 0, SELECT vazio)
// e captura o texto do SQL e os argumentos (nargs por consulta).
func sec25Mock(t *testing.T, nargsCount int) (*sec25Captura, sqlmock.Sqlmock, func() db.Querier) {
	t.Helper()
	c := &sec25Captura{}
	pool, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_, actual string) error {
		c.sqls = append(c.sqls, actual)
		return nil
	})))
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	args := func(n int) []driver.Value {
		out := make([]driver.Value, n)
		for i := range out {
			out[i] = sec25Arg{c}
		}
		return out
	}
	mock.ExpectQuery("count").WithArgs(args(nargsCount)...).WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery("select").WithArgs(args(nargsCount + 2)...).WillReturnRows(sqlmock.NewRows([]string{"x"}))
	return c, mock, func() db.Querier { return pool }
}

var sec25LikeSemEscape = regexp.MustCompile(`LIKE \?($|[^ ]| [^E])`)

func TestSEC25_Adapters_LikeComEscape(t *testing.T) {
	const termo = "%"          // q=% vira o argumento %!%%
	const argEsperado = "%!%%" // db.TermoContem("%")
	ctx := context.Background()
	pg := kernel.NovaPagina(1, 10)

	casos := []struct {
		nome   string
		nLikes int // LIKEs por consulta
		listar func(q db.Querier) error
	}{
		{"produtos (descricao/sku)", 2, func(q db.Querier) error {
			_, err := catalogomysql.NovoProdutos(q).Listar(ctx, catalogo.FiltroProduto{Busca: termo, Ordenacao: catalogo.OrdenacaoProdutoPadrao()}, pg)
			return err
		}},
		{"clientes (razao_social/cnpj)", 2, func(q db.Querier) error {
			_, err := comercialmysql.NovoClientes(q).Listar(ctx, escopoAdmin(), comercial.FiltroCliente{Busca: termo, Ordenacao: comercial.OrdenacaoClientePadrao()}, pg)
			return err
		}},
		{"visitas (resultado)", 1, func(q db.Querier) error {
			_, err := comercialmysql.NovoVisitas(q).Listar(ctx, escopoAdmin(), comercial.FiltroVisita{Busca: termo, Ordenacao: comercial.OrdenacaoVisitaPadrao()}, pg)
			return err
		}},
		{"oportunidades (origem/etapa)", 2, func(q db.Querier) error {
			_, err := comercialmysql.NovoOportunidades(q).Listar(ctx, escopoAdmin(), comercial.FiltroOportunidade{Busca: termo, Ordenacao: comercial.OrdenacaoOportunidadePadrao()}, pg)
			return err
		}},
		{"pedidos (razao_social do cliente)", 1, func(q db.Querier) error {
			_, err := vendasmysql.NovoPedidos(q).Listar(ctx, escopoAdmin(), vendas.FiltroPedido{Busca: termo, Ordenacao: vendas.OrdenacaoPedidoPadrao()}, pg)
			return err
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capt, mock, pool := sec25Mock(t, c.nLikes)
			require.NoError(t, c.listar(pool()))
			require.NoError(t, mock.ExpectationsWereMet())

			require.Len(t, capt.sqls, 2, "COUNT e SELECT")
			for _, s := range capt.sqls {
				assert.Equal(t, c.nLikes, strings.Count(s, "LIKE ? ESCAPE '!'"), "sql=%s", s)
				assert.False(t, sec25LikeSemEscape.MatchString(s), "LIKE sem ESCAPE: %s", s)
			}
			want := make([]string, 0, 2*c.nLikes)
			for i := 0; i < 2*c.nLikes; i++ {
				want = append(want, argEsperado)
			}
			assert.Equal(t, want, capt.args, "o curinga digitado vira literal")
		})
	}
}

// Clientes: o termo de CNPJ (busca mascarada sem a máscara) também é
// escapado; a razão social recebe a busca original.
func TestSEC25_Clientes_TermoCNPJTambemEscapado(t *testing.T) {
	capt, mock, pool := sec25Mock(t, 2)
	_, err := comercialmysql.NovoClientes(pool()).Listar(context.Background(), escopoAdmin(),
		comercial.FiltroCliente{Busca: "a_b%", Ordenacao: comercial.OrdenacaoClientePadrao()}, kernel.NovaPagina(1, 10))
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, []string{"%a!_b!%%", "%a!_b!%%", "%a!_b!%%", "%a!_b!%%"}, capt.args)
}
