package catalogo_test

// BUG-17 (Lote 25): toggle de ativo atômico. Sem valor, um único
// "UPDATE produtos SET ativo = NOT ativo WHERE id = ?" (sem leitura antes);
// com valor, "SET ativo = ?"; depois, PorID para a resposta. 0 linhas → 404
// sem releitura.
//
// CHORE-06 (Lote 25): 1062 em uk_produtos_sku e uk_estoque_data_sku vira o
// sentinela 409 sem embrulhar o erro do driver; 1062 em outro índice
// continua 500 (sem o valor). O caso de uso loga a recusa sem o valor.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/kernel"
)

const (
	bug17ToggleRe  = `^UPDATE produtos SET ativo = NOT ativo WHERE id = \?$`
	bug17DefinirRe = `^UPDATE produtos SET ativo = \? WHERE id = \?$`
	bug17PorIDRe   = `^SELECT id, sku, .+ FROM produtos WHERE id = \? LIMIT 1$`
)

func bug17Mock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	pool, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool, mock
}

func bug17LinhaProduto(id int64, ativo bool) *sqlmock.Rows {
	agora := time.Now()
	return sqlmock.NewRows([]string{"id", "sku", "descricao", "categoria", "marca", "nota_olfativa", "preco_tabela",
		"custo_unitario", "unidade", "data_lancamento", "ativo", "created_at", "updated_at"}).
		AddRow(id, "SKU-1", "D", "C", "M", "", 1.0, 1.0, "UN", nil, ativo, agora, agora)
}

func bug17CapturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	prev := log.Writer()
	log.SetOutput(out)
	t.Cleanup(func() { log.SetOutput(prev) })
	return out
}

func TestBUG17_Adapter_AlternarAtivo(t *testing.T) {
	ctx := context.Background()
	errBanco := errors.New("db down")
	casos := []struct {
		nome string
		res  driverResult
		want error
	}{
		{"1 linha", driverResult{linhas: 1}, nil},
		{"0 linhas → ErrProdutoNaoEncontrado", driverResult{linhas: 0}, catalogo.ErrProdutoNaoEncontrado},
		{"erro do driver → infra", driverResult{err: errBanco}, errBanco},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pool, mock := bug17Mock(t)
			e := mock.ExpectExec(bug17ToggleRe).WithArgs(int64(4))
			if c.res.err != nil {
				e.WillReturnError(c.res.err)
			} else {
				e.WillReturnResult(sqlmock.NewResult(0, c.res.linhas))
			}
			err := catalogomysql.NovoProdutos(pool).AlternarAtivo(ctx, 4)
			if c.want == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, c.want)
			}
			if c.res.err != nil {
				var ke *kernel.Erro
				assert.False(t, errors.As(err, &ke), "falha de infra é 500")
				assert.ErrorContains(t, err, "catalogomysql: alternar ativo produto")
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

type driverResult struct {
	linhas int64
	err    error
}

// Caso de uso com o adapter real: a escrita vem PRIMEIRO (sqlmock em ordem
// estrita: um SELECT antes do UPDATE falharia) e a resposta é a releitura.
func TestBUG17_Casos_AlternarAtivoProduto_SQL(t *testing.T) {
	ctx := context.Background()
	verdadeiro, falso := true, false
	casos := []struct {
		nome    string
		ativo   *bool
		escrita func(m sqlmock.Sqlmock) *sqlmock.ExpectedExec
		relido  bool
		linhas  int64
		want    error
	}{
		{"sem body: NOT ativo", nil, func(m sqlmock.Sqlmock) *sqlmock.ExpectedExec { return m.ExpectExec(bug17ToggleRe).WithArgs(int64(1)) }, false, 1, nil},
		{"ativo=true: SET ativo = ?", &verdadeiro, func(m sqlmock.Sqlmock) *sqlmock.ExpectedExec {
			return m.ExpectExec(bug17DefinirRe).WithArgs(true, int64(1))
		}, true, 1, nil},
		{"ativo=false: SET ativo = ?", &falso, func(m sqlmock.Sqlmock) *sqlmock.ExpectedExec {
			return m.ExpectExec(bug17DefinirRe).WithArgs(false, int64(1))
		}, false, 1, nil},
		{"sem body, 0 linhas → 404 sem releitura", nil, func(m sqlmock.Sqlmock) *sqlmock.ExpectedExec { return m.ExpectExec(bug17ToggleRe).WithArgs(int64(1)) }, false, 0, catalogo.ErrProdutoNaoEncontrado},
		{"com body, 0 linhas → 404 sem releitura", &verdadeiro, func(m sqlmock.Sqlmock) *sqlmock.ExpectedExec {
			return m.ExpectExec(bug17DefinirRe).WithArgs(true, int64(1))
		}, false, 0, catalogo.ErrProdutoNaoEncontrado},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pool, mock := bug17Mock(t)
			c.escrita(mock).WillReturnResult(sqlmock.NewResult(0, c.linhas))
			if c.want == nil {
				mock.ExpectQuery(bug17PorIDRe).WithArgs(int64(1)).WillReturnRows(bug17LinhaProduto(1, c.relido))
			}
			casos := catalogoapp.Novo(catalogomysql.NovoProdutos(pool), catalogomysql.NovoEstoques(pool))
			p, err := casos.AlternarAtivoProduto(ctx, 1, c.ativo)
			if c.want != nil {
				assert.Nil(t, p)
				assert.ErrorIs(t, err, c.want)
			} else {
				require.NoError(t, err)
				assert.Equal(t, c.relido, p.Ativo, "a resposta é o valor relido do banco")
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Falha na releitura depois do toggle gravado: erro (o caso de uso não
// inventa o status em memória).
func TestBUG17_Casos_ReleituraFalha(t *testing.T) {
	pool, mock := bug17Mock(t)
	errBanco := errors.New("db down")
	mock.ExpectExec(bug17ToggleRe).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(bug17PorIDRe).WithArgs(int64(1)).WillReturnError(errBanco)
	p, err := catalogoapp.Novo(catalogomysql.NovoProdutos(pool), catalogomysql.NovoEstoques(pool)).AlternarAtivoProduto(context.Background(), 1, nil)
	assert.Nil(t, p)
	assert.ErrorIs(t, err, errBanco)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Dois toggles sobre o fake (que inverte o valor GRAVADO, como o UPDATE
// NOT ativo) voltam ao valor original; o valor explícito é idempotente.
func TestBUG17_Casos_ToggleDuploVoltaAoOriginal(t *testing.T) {
	ctx := context.Background()
	for _, original := range []bool{true, false} {
		f := &fakeProdutos{porID: map[int64]catalogo.Produto{1: {ID: 1, SKU: "S", Ativo: original}}}
		c := catalogoapp.Novo(f, &fakeEstoques{})
		p, err := c.AlternarAtivoProduto(ctx, 1, nil)
		require.NoError(t, err)
		assert.Equal(t, !original, p.Ativo)
		p, err = c.AlternarAtivoProduto(ctx, 1, nil)
		require.NoError(t, err)
		assert.Equal(t, original, p.Ativo)

		v := !original
		for i := 0; i < 2; i++ {
			p, err = c.AlternarAtivoProduto(ctx, 1, &v)
			require.NoError(t, err)
			assert.Equal(t, v, p.Ativo, "valor explícito é idempotente")
		}
	}
}

// CHORE-06 no adapter: só o índice certo vira o sentinela 409.
func TestCHORE06_Adapter_DuplicidadePorIndice(t *testing.T) {
	ctx := context.Background()
	casos := []struct {
		nome      string
		re        string
		msg       string
		gravar    func(pool *sql.DB) error
		sentinela error
	}{
		{"produto uk_produtos_sku → 409", `INSERT INTO produtos`, "Duplicate entry 'SKU-SECRETO' for key 'produtos.uk_produtos_sku'",
			func(pool *sql.DB) error {
				return catalogomysql.NovoProdutos(pool).Inserir(ctx, &catalogo.Produto{SKU: "SKU-SECRETO"})
			}, catalogo.ErrSKUDuplicado},
		{"produto PRIMARY → 500", `INSERT INTO produtos`, "Duplicate entry 'SKU-SECRETO' for key 'PRIMARY'",
			func(pool *sql.DB) error {
				return catalogomysql.NovoProdutos(pool).Inserir(ctx, &catalogo.Produto{SKU: "SKU-SECRETO"})
			}, nil},
		{"estoque uk_estoque_data_sku → 409", `INSERT INTO estoque`, "Duplicate entry '2024-01-01-SKU-SECRETO' for key 'estoque.uk_estoque_data_sku'",
			func(pool *sql.DB) error {
				return catalogomysql.NovoEstoques(pool).Inserir(ctx, &catalogo.RegistroEstoque{SKU: "SKU-SECRETO", DataSnapshot: time.Now()})
			}, catalogo.ErrEstoqueDuplicado},
		{"estoque PRIMARY → 500", `INSERT INTO estoque`, "Duplicate entry '2024-01-01-SKU-SECRETO' for key 'PRIMARY'",
			func(pool *sql.DB) error {
				return catalogomysql.NovoEstoques(pool).Inserir(ctx, &catalogo.RegistroEstoque{SKU: "SKU-SECRETO", DataSnapshot: time.Now()})
			}, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pool, mock := bug17Mock(t)
			mock.ExpectExec(c.re).WillReturnError(&mysql.MySQLError{Number: 1062, Message: c.msg})
			err := c.gravar(pool)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "SKU-SECRETO", "nunca o valor duplicado")
			var ke *kernel.Erro
			var me *mysql.MySQLError
			if c.sentinela != nil {
				assert.Same(t, c.sentinela, err)
				require.True(t, errors.As(err, &ke))
				assert.Equal(t, kernel.KindConflito, ke.Kind())
				assert.False(t, errors.As(err, &me), "sem embrulhar o driver")
			} else {
				assert.False(t, errors.As(err, &ke), "outro índice continua 500")
				require.True(t, errors.As(err, &me), "o 1062 segue identificável (sem o valor)")
				assert.Equal(t, uint16(1062), me.Number)
				assert.Contains(t, me.Message, "PRIMARY")
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCHORE06_Mensagens(t *testing.T) {
	assert.Equal(t, "sku já cadastrado", catalogo.ErrSKUDuplicado.Error())
	assert.Equal(t, "registro de estoque já cadastrado para este sku nesta data", catalogo.ErrEstoqueDuplicado.Error())
}

// O caso de uso loga a recusa (só o índice) e nunca o SKU nem a data.
func TestCHORE06_Casos_LogSemValor(t *testing.T) {
	ctx := context.Background()
	t.Run("produto", func(t *testing.T) {
		out := bug17CapturarLog(t)
		_, err := catalogoapp.Novo(&fakeProdutos{errInsert: catalogo.ErrSKUDuplicado}, &fakeEstoques{}).CriarProduto(ctx, dadosProdutoValidos())
		assert.Same(t, catalogo.ErrSKUDuplicado, err)
		assert.Contains(t, out.String(), "[produtos] sku duplicado recusado (uk_produtos_sku)")
		assert.NotContains(t, out.String(), "SKU-1")
	})
	t.Run("produto, outro erro não loga a recusa", func(t *testing.T) {
		out := bug17CapturarLog(t)
		_, err := catalogoapp.Novo(&fakeProdutos{errInsert: errInfra}, &fakeEstoques{}).CriarProduto(ctx, dadosProdutoValidos())
		assert.ErrorIs(t, err, errInfra)
		assert.NotContains(t, out.String(), "duplicado recusado")
	})
	t.Run("estoque", func(t *testing.T) {
		out := bug17CapturarLog(t)
		f := &fakeProdutos{existe: true}
		d := catalogo.DadosEstoque{SKU: "SKU-SECRETO", DataSnapshot: "2024-01-01", Saldo: 3}
		_, err := catalogoapp.Novo(f, &fakeEstoques{errInsert: catalogo.ErrEstoqueDuplicado}).AjustarEstoque(ctx, d)
		assert.Same(t, catalogo.ErrEstoqueDuplicado, err)
		assert.Contains(t, out.String(), "[estoque] registro duplicado recusado (uk_estoque_data_sku)")
		assert.NotContains(t, out.String(), "SKU-SECRETO")
		assert.NotContains(t, out.String(), "2024-01-01")
	})
}
