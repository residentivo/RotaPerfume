package repositories_test

// SEC-24: injeção via order_by/order_dir em TODOS os enums de ordenação que
// ainda não tinham tabela própria (Cliente, Visita, Oportunidade, Pedido,
// Pagamento, Usuário). Produto, Estoque e SenhaHistorico estão em
// sort_test.go. O caminho testado é o mesmo do HTTP: kernel.ParseOrdenacao
// contra CamposOrdenacaoX() e o switch do adapter. A asserção é sobre o
// ORDER BY efetivamente enviado ao banco (SQL capturado pelo sqlmock).
//
// Fora de escopo de propósito: o SQL da busca LIKE e o formato de "q"
// (SEC-25, em mudança); os testes não usam filtro de busca.

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

// entidadeSEC24 descreve uma listagem ordenável: como listar com uma
// ordenação já resolvida, a whitelist, o padrão e o resultado esperado.
type entidadeSEC24[C ~string] struct {
	validos []C
	padrao  kernel.Ordenacao[C]
	// colPadrao/dirPadrao: "ORDER BY <colPadrao> <dirPadrao>".
	colPadrao, dirPadrao string
	// campoValido/colValida: um campo da whitelist (não padrão) e a coluna
	// que ele deve gerar; usado nos casos de order_dir e de maiúsculas.
	campoValido, colValida string
	// mesmaColunaQuePadrao: valores do enum que, por contrato, geram a
	// coluna padrão (ex.: "visita_id" é sinônimo de "id" em visitas).
	mesmaColunaQuePadrao []C
	listar               func(*sql.DB, kernel.Ordenacao[C]) error
}

// payloadsOrderBySEC24: nenhum deve sair do padrão da entidade (dir vazia =
// direção padrão).
var payloadsOrderBySEC24 = []struct{ nome, orderBy string }{
	{"DROP com comentário", "id; DROP TABLE x;--"},
	{"DROP sem espaço", "id;DROP"},
	{"subquery", "(SELECT 1)"},
	{"lista de colunas 1,2", "1,2"},
	{"coluna mais subquery SLEEP", "id,(SELECT SLEEP(5))"},
	{"SLEEP direto", "SLEEP(5)"},
	{"crase sobrando", "razao_social`"},
	{"crase envolvendo", "`id`"},
	{"alias p.", "p.razao_social"},
	{"alias c.", "c.razao_social"},
	{"alias u.", "u.id"},
	{"byte nulo", "id\x00"},
	{"byte nulo no meio", "id\x00; DROP TABLE x"},
	{"só espaços", "     "},
	{"tab e quebra de linha", "\t\n"},
	{"comentário inline", "id/**/"},
	{"posição numérica", "1"},
	{"CASE WHEN", "CASE WHEN 1=1 THEN id END"},
	{"IF/SLEEP", "IF(1=1,SLEEP(5),id)"},
	{"vazio", ""},
}

// payloadsOrderDirSEC24: com campo válido, a direção cai na padrão.
var payloadsOrderDirSEC24 = []struct{ nome, orderDir string }{
	{"DROP em dir", "asc; DROP TABLE x;--"},
	{"ASC,1", "ASC,1"},
	{"desc limit 1", "desc limit 1"},
	{"SLEEP em dir", "desc,SLEEP(5)"},
	{"inválido", "sideways"},
	{"vazio", ""},
	{"byte nulo", "asc\x00"},
}

// assertSQLLimpo: nada do payload chega ao texto do SQL.
func assertSQLLimpo(t *testing.T, lista string) {
	t.Helper()
	up := strings.ToUpper(lista)
	for _, proibido := range []string{"DROP", "SLEEP", "SELECT 1", ",2", "--", "/*", "`", "\x00", "CASE WHEN", ";"} {
		assert.NotContains(t, up, proibido, "SQL contém %q: %s", proibido, lista)
	}
}

var colunaSegura = regexp.MustCompile(`^ORDER BY [a-z_]+(\.[a-z_]+)? (ASC|DESC)$`)

// capturarOrderBy roda a listagem e devolve o SQL e o ORDER BY capturados.
func capturarOrderBy[C ~string](t *testing.T, e entidadeSEC24[C], o kernel.Ordenacao[C]) (string, string) {
	t.Helper()
	db, mock, captured := newCapturingMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\)`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	mock.ExpectQuery(`ORDER BY .* LIMIT \? OFFSET \?`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	require.NoError(t, e.listar(db, o))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, *captured, 2)
	lista := (*captured)[1]
	ob := orderByDe(t, lista)
	assert.Regexp(t, colunaSegura, ob)
	return lista, ob
}

// rodarSEC24 aplica as tabelas de payload, os enums fabricados e a
// completude a uma entidade.
func rodarSEC24[C ~string](t *testing.T, e entidadeSEC24[C]) {
	padrao := "ORDER BY " + e.colPadrao + " " + e.dirPadrao
	parse := func(campo, dir string) kernel.Ordenacao[C] {
		return kernel.ParseOrdenacao(campo, dir, e.validos, e.padrao)
	}

	t.Run("order_by", func(t *testing.T) {
		for _, p := range payloadsOrderBySEC24 {
			t.Run(p.nome, func(t *testing.T) {
				lista, ob := capturarOrderBy(t, e, parse(p.orderBy, ""))
				assert.Equal(t, padrao, ob)
				assertSQLLimpo(t, lista)
			})
		}
	})

	t.Run("order_by com order_dir injetado", func(t *testing.T) {
		for _, p := range payloadsOrderBySEC24 {
			t.Run(p.nome, func(t *testing.T) {
				lista, ob := capturarOrderBy(t, e, parse(p.orderBy, "asc; DROP TABLE x;--"))
				assert.Equal(t, padrao, ob)
				assertSQLLimpo(t, lista)
			})
		}
	})

	t.Run("order_dir", func(t *testing.T) {
		for _, p := range payloadsOrderDirSEC24 {
			t.Run(p.nome, func(t *testing.T) {
				lista, ob := capturarOrderBy(t, e, parse(e.campoValido, p.orderDir))
				assert.Equal(t, "ORDER BY "+e.colValida+" "+e.dirPadrao, ob)
				assertSQLLimpo(t, lista)
			})
		}
	})

	t.Run("campo válido em maiúsculas resolve", func(t *testing.T) {
		for _, c := range []struct{ campo, dir, want string }{
			{strings.ToUpper(e.campoValido), "DESC", "ORDER BY " + e.colValida + " DESC"},
			{"  " + strings.ToUpper(e.campoValido) + "  ", " asc ", "ORDER BY " + e.colValida + " ASC"},
		} {
			_, ob := capturarOrderBy(t, e, parse(c.campo, c.dir))
			assert.Equal(t, c.want, ob)
		}
	})

	t.Run("enum fabricado fora do parse e zero value", func(t *testing.T) {
		for _, c := range []struct {
			nome string
			o    kernel.Ordenacao[C]
			want string
		}{
			{"DROP fabricado", kernel.NovaOrdenacao(C("id; DROP TABLE x;--"), kernel.Desc), "ORDER BY " + e.colPadrao + " DESC"},
			{"SLEEP fabricado", kernel.NovaOrdenacao(C("id,(SELECT SLEEP(5))"), kernel.Asc), "ORDER BY " + e.colPadrao + " ASC"},
			{"1,2 fabricado", kernel.NovaOrdenacao(C("1,2"), kernel.Asc), "ORDER BY " + e.colPadrao + " ASC"},
			{"maiúsculas fabricadas não casam", kernel.NovaOrdenacao(C(strings.ToUpper(e.campoValido)), kernel.Asc), "ORDER BY " + e.colPadrao + " ASC"},
			{"zero value", kernel.Ordenacao[C]{}, padrao},
		} {
			t.Run(c.nome, func(t *testing.T) {
				lista, ob := capturarOrderBy(t, e, c.o)
				assert.Equal(t, c.want, ob)
				assertSQLLimpo(t, lista)
			})
		}
	})

	t.Run("completude do enum", func(t *testing.T) {
		require.NotEmpty(t, e.validos)
		vistos := map[string]C{}
		for _, v := range e.validos {
			_, ob := capturarOrderBy(t, e, kernel.NovaOrdenacao(v, kernel.Asc))
			col := strings.TrimSuffix(strings.TrimPrefix(ob, "ORDER BY "), " ASC")
			if v == e.padrao.Campo() || contem(e.mesmaColunaQuePadrao, v) {
				assert.Equal(t, e.colPadrao, col, "campo %q deveria gerar a coluna padrão", v)
				continue
			}
			assert.NotEqual(t, e.colPadrao, col, "campo %q da whitelist sem case no switch do adapter (caiu no padrão)", v)
			if outro, dup := vistos[col]; dup {
				t.Errorf("campos %q e %q geram a mesma coluna %q", outro, v, col)
			}
			vistos[col] = v
		}
		require.Contains(t, e.validos, e.padrao.Campo(), "o campo padrão precisa estar na whitelist")
		require.Contains(t, e.validos, C(e.campoValido))
	})
}

func contem[C comparable](s []C, v C) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func pagina() kernel.Pagina { return kernel.NovaPagina(1, 10) }

func TestSEC24_OrdenacaoInjecao_Cliente(t *testing.T) {
	rodarSEC24(t, entidadeSEC24[comercial.CampoCliente]{
		validos: comercial.CamposOrdenacaoCliente(), padrao: comercial.OrdenacaoClientePadrao(),
		colPadrao: "cliente_id_origem", dirPadrao: "ASC",
		campoValido: "razao_social", colValida: "razao_social",
		listar: func(db *sql.DB, o kernel.Ordenacao[comercial.CampoCliente]) error {
			_, err := comercialmysql.NovoClientes(db).Listar(context.Background(), escopoAdmin(), comercial.FiltroCliente{Ordenacao: o}, pagina())
			return err
		},
	})
}

func TestSEC24_OrdenacaoInjecao_Visita(t *testing.T) {
	rodarSEC24(t, entidadeSEC24[comercial.CampoVisita]{
		validos: comercial.CamposOrdenacaoVisita(), padrao: comercial.OrdenacaoVisitaPadrao(),
		colPadrao: "visita_id", dirPadrao: "ASC",
		campoValido: "data_visita", colValida: "data_visita",
		mesmaColunaQuePadrao: []comercial.CampoVisita{comercial.OrdemVisitaVisitaID},
		listar: func(db *sql.DB, o kernel.Ordenacao[comercial.CampoVisita]) error {
			_, err := comercialmysql.NovoVisitas(db).Listar(context.Background(), escopoAdmin(), comercial.FiltroVisita{Ordenacao: o}, pagina())
			return err
		},
	})
}

func TestSEC24_OrdenacaoInjecao_Oportunidade(t *testing.T) {
	rodarSEC24(t, entidadeSEC24[comercial.CampoOportunidade]{
		validos: comercial.CamposOrdenacaoOportunidade(), padrao: comercial.OrdenacaoOportunidadePadrao(),
		colPadrao: "oportunidade_id", dirPadrao: "ASC",
		campoValido: "probabilidade_pct", colValida: "probabilidade_pct",
		listar: func(db *sql.DB, o kernel.Ordenacao[comercial.CampoOportunidade]) error {
			_, err := comercialmysql.NovoOportunidades(db).Listar(context.Background(), escopoAdmin(), comercial.FiltroOportunidade{Ordenacao: o}, pagina())
			return err
		},
	})
}

func TestSEC24_OrdenacaoInjecao_Pedido(t *testing.T) {
	rodarSEC24(t, entidadeSEC24[vendas.CampoPedido]{
		validos: vendas.CamposOrdenacaoPedido(), padrao: vendas.OrdenacaoPedidoPadrao(),
		colPadrao: "p.pedido_id_origem", dirPadrao: "DESC",
		campoValido: "cliente_nome", colValida: "c.razao_social",
		listar: func(db *sql.DB, o kernel.Ordenacao[vendas.CampoPedido]) error {
			_, err := vendasmysql.NovoPedidos(db).Listar(context.Background(), escopoAdmin(), vendas.FiltroPedido{Ordenacao: o}, pagina())
			return err
		},
	})
}

func TestSEC24_OrdenacaoInjecao_Pagamento(t *testing.T) {
	rodarSEC24(t, entidadeSEC24[vendas.CampoPagamento]{
		validos: vendas.CamposOrdenacaoPagamento(), padrao: vendas.OrdenacaoPagamentoPadrao(),
		colPadrao: "pagamento_id", dirPadrao: "ASC",
		campoValido: "valor_liquido", colValida: "valor_liquido",
		listar: func(db *sql.DB, o kernel.Ordenacao[vendas.CampoPagamento]) error {
			_, err := vendasmysql.NovoPagamentos(db).Listar(context.Background(), escopoAdmin(), vendas.FiltroPagamento{Ordenacao: o}, pagina())
			return err
		},
	})
}

func TestSEC24_OrdenacaoInjecao_Usuario(t *testing.T) {
	rodarSEC24(t, entidadeSEC24[identidade.CampoUsuario]{
		validos: identidade.CamposOrdenacaoUsuario(), padrao: identidade.OrdenacaoUsuarioPadrao(),
		colPadrao: "u.id", dirPadrao: "ASC",
		campoValido: "ultimo_login_at", colValida: "u.ultimo_login_at",
		listar: func(db *sql.DB, o kernel.Ordenacao[identidade.CampoUsuario]) error {
			_, err := identidademysql.NovoUsuarios(db).Listar(context.Background(), o, pagina())
			return err
		},
	})
}
