package comercial_test

// ARQ-04: adapters comercialmysql. O1 (§5.5/§8): todo repositório que
// recebe Escopo devolve erro com EscopoSemAcesso() e com Escopo{} SEM
// nenhuma expectativa registrada no sqlmock (qualquer SQL faria o sqlmock
// falhar). N-8/O6-iv: o 1062 vira o sentinela sem o erro do driver.

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	shareddb "github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
)

func novoMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func irrestrito() kernel.Escopo {
	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	e, _ := kernel.EscopoIrrestrito(admin)
	return e
}

// TestARQ04_O1_SemAcessoEZeroValueSemSQL: os quatro repositórios com escopo.
func TestARQ04_O1_SemAcessoEZeroValueSemSQL(t *testing.T) {
	ctx := context.Background()
	pg := kernel.NovaPagina(1, 20)
	escopos := map[string]struct {
		e    kernel.Escopo
		want error
	}{
		"SemAcesso":  {kernel.EscopoSemAcesso(), kernel.ErrEscopoSemAcesso},
		"zero value": {kernel.Escopo{}, kernel.ErrEscopoInvalido},
	}
	repos := map[string]func(db *sql.DB, e kernel.Escopo) error{
		"Vendedores.ListarResumo": func(db *sql.DB, e kernel.Escopo) error {
			out, err := comercialmysql.NovoVendedores(db).ListarResumo(ctx, e)
			assert.Nil(t, out)
			return err
		},
		"Clientes.Listar": func(db *sql.DB, e kernel.Escopo) error {
			r, err := comercialmysql.NovoClientes(db).Listar(ctx, e, comercial.FiltroCliente{}, pg)
			assert.Nil(t, r.Itens)
			return err
		},
		"Visitas.Listar": func(db *sql.DB, e kernel.Escopo) error {
			r, err := comercialmysql.NovoVisitas(db).Listar(ctx, e, comercial.FiltroVisita{VendedorID: 3}, pg)
			assert.Nil(t, r.Itens)
			return err
		},
		"Oportunidades.Listar": func(db *sql.DB, e kernel.Escopo) error {
			r, err := comercialmysql.NovoOportunidades(db).Listar(ctx, e, comercial.FiltroOportunidade{VendedorID: 3}, pg)
			assert.Nil(t, r.Itens)
			return err
		},
	}
	for nomeRepo, chamar := range repos {
		for nomeEscopo, c := range escopos {
			t.Run(nomeRepo+"/"+nomeEscopo, func(t *testing.T) {
				db, mock := novoMock(t) // nenhuma expectativa: SQL nenhum
				err := chamar(db, c.e)
				assert.ErrorIs(t, err, c.want)
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

// E-03: no escopo de vendedor, o vendedor_id da query é SOBRESCRITO.
func TestARQ04_ListagemRestrita_SobrescreveVendedorDaQuery(t *testing.T) {
	doVendedor, _ := kernel.EscopoDoVendedor(10)
	db, mock := novoMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas WHERE vendedor_id = \?$`).WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
	mock.ExpectQuery(`FROM visitas WHERE vendedor_id = \? ORDER BY visita_id ASC LIMIT`).WithArgs(int64(10), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"visita_id"}))
	_, err := comercialmysql.NovoVisitas(db).Listar(context.Background(), doVendedor, comercial.FiltroVisita{VendedorID: 99}, kernel.NovaPagina(1, 20))
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ04_Adapters_PanicSemQuerier(t *testing.T) {
	assert.Panics(t, func() { comercialmysql.NovoVendedores(nil) })
	assert.Panics(t, func() { comercialmysql.NovoCarteiras(nil) })
	assert.Panics(t, func() { comercialmysql.NovoClientes(nil) })
	assert.Panics(t, func() { comercialmysql.NovoVisitas(nil) })
	assert.Panics(t, func() { comercialmysql.NovoOportunidades(nil) })
}

// N-8/O6-iv/E-07: o 1062 do CNPJ vira o sentinela (409) SEM o erro do driver
// (que traz o CNPJ). O 1062 de carteiras vira ErrVinculoDuplicado (409
// desde o CHORE-06; era 500), também sem o valor.
func TestARQ04_ChaveDuplicadaSemValorDoDriver(t *testing.T) {
	ctx := context.Background()
	dup := &mysql.MySQLError{Number: 1062, Message: "Duplicate entry '11222333000181' for key 'clientes.uq_clientes_cnpj'"}

	db, mock := novoMock(t)
	mock.ExpectExec(`INSERT INTO clientes`).WillReturnError(dup)
	mock.ExpectExec(`UPDATE clientes`).WillReturnError(dup)
	err := comercialmysql.NovoClientes(db).Inserir(ctx, &comercial.Cliente{})
	assert.Same(t, comercial.ErrCNPJDuplicado, err)
	err = comercialmysql.NovoClientes(db).Atualizar(ctx, 1, &comercial.Cliente{})
	assert.Same(t, comercial.ErrCNPJDuplicado, err)
	assert.NotContains(t, err.Error(), "11222333000181")

	mock.ExpectExec(`INSERT INTO carteiras`).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry '5-3-2026-01-01' for key 'carteiras.uq'"})
	err = comercialmysql.NovoCarteiras(db).Inserir(ctx, &comercial.Carteira{})
	assert.Same(t, comercial.ErrVinculoDuplicado, err)
	var me *mysql.MySQLError
	assert.False(t, errors.As(err, &me))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// C-02: cada valor do enum vira a sua coluna; fora do enum, o padrão.
func TestARQ04_Ordenacao_TodasAsColunas(t *testing.T) {
	ctx := context.Background()
	pg := kernel.NovaPagina(1, 20)
	clientes := map[comercial.CampoCliente]string{
		comercial.OrdemClienteID: "cliente_id_origem", comercial.OrdemClienteRazaoSocial: "razao_social", comercial.OrdemClienteCNPJ: "cnpj",
		comercial.OrdemClienteSegmento: "segmento", comercial.OrdemClienteCidade: "cidade", comercial.OrdemClienteUF: "uf",
		comercial.OrdemClienteDataCadastro: "data_cadastro", comercial.OrdemClienteAtivo: "ativo",
		comercial.OrdemClienteCriadoEm: "created_at", comercial.OrdemClienteAtualizadoEm: "updated_at",
	}
	for campo, coluna := range clientes {
		esperarListagem(t, "clientes", coluna, func(db *sql.DB) error {
			_, err := comercialmysql.NovoClientes(db).Listar(ctx, irrestrito(), comercial.FiltroCliente{Ordenacao: kernel.NovaOrdenacao(campo, kernel.Desc)}, pg)
			return err
		})
	}
	visitas := map[comercial.CampoVisita]string{
		comercial.OrdemVisitaID: "visita_id", comercial.OrdemVisitaVisitaID: "visita_id", comercial.OrdemVisitaData: "data_visita",
		comercial.OrdemVisitaDuracao: "duracao_min", comercial.OrdemVisitaResultado: "resultado",
		comercial.OrdemVisitaCriadoEm: "created_at", comercial.OrdemVisitaAtualizadoEm: "updated_at",
	}
	for campo, coluna := range visitas {
		esperarListagem(t, "visitas", coluna, func(db *sql.DB) error {
			_, err := comercialmysql.NovoVisitas(db).Listar(ctx, irrestrito(), comercial.FiltroVisita{Ordenacao: kernel.NovaOrdenacao(campo, kernel.Desc)}, pg)
			return err
		})
	}
	oportunidades := map[comercial.CampoOportunidade]string{
		comercial.OrdemOportunidadeID: "oportunidade_id", comercial.OrdemOportunidadeDataAbertura: "data_abertura",
		comercial.OrdemOportunidadeValorEstimado: "valor_estimado", comercial.OrdemOportunidadeProbabilidade: "probabilidade_pct",
		comercial.OrdemOportunidadeEtapa: "etapa", comercial.OrdemOportunidadeOrigem: "origem",
		comercial.OrdemOportunidadeCriadoEm: "created_at", comercial.OrdemOportunidadeAtualizadoEm: "updated_at",
	}
	for campo, coluna := range oportunidades {
		esperarListagem(t, "oportunidades", coluna, func(db *sql.DB) error {
			_, err := comercialmysql.NovoOportunidades(db).Listar(ctx, irrestrito(), comercial.FiltroOportunidade{Ordenacao: kernel.NovaOrdenacao(campo, kernel.Desc)}, pg)
			return err
		})
	}
}

func esperarListagem(t *testing.T, tabela, coluna string, listar func(*sql.DB) error) {
	t.Run(tabela+"/"+coluna, func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ` + tabela).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
		mock.ExpectQuery(`ORDER BY ` + regexp.QuoteMeta(coluna) + ` DESC LIMIT`).WillReturnRows(sqlmock.NewRows([]string{"x"}))
		require.NoError(t, listar(db))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// I-09 (adapter): MarcarDesligamento usa COALESCE e 0 linhas = inexistente.
func TestARQ04_MarcarDesligamento(t *testing.T) {
	db, mock := novoMock(t)
	agora := time.Now()
	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = COALESCE\(data_desligamento, \?\) WHERE id = \?`).
		WithArgs(agora, int64(4)).WillReturnResult(sqlmock.NewResult(0, 0))
	err := comercialmysql.NovoVendedores(db).MarcarDesligamento(context.Background(), 4, agora)
	assert.ErrorIs(t, err, comercial.ErrVendedorNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// N2: 1062 em OUTRO índice de clientes continua erro interno embrulhado,
// com o tipo e o índice do driver, mas SEM o valor duplicado.
func TestARQ04_N2_DuplicadaEmOutroIndiceSemValor(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectExec(`INSERT INTO clientes`).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'segredo-99' for key 'clientes.PRIMARY'"})
	err := comercialmysql.NovoClientes(db).Inserir(context.Background(), &comercial.Cliente{})
	require.Error(t, err)
	assert.NotErrorIs(t, err, comercial.ErrCNPJDuplicado)
	var me *mysql.MySQLError
	require.True(t, errors.As(err, &me), "erro tipado preservado")
	assert.Equal(t, uint16(1062), me.Number)
	assert.Contains(t, err.Error(), "clientes.PRIMARY")
	assert.NotContains(t, err.Error(), "segredo-99")
	assert.NoError(t, mock.ExpectationsWereMet())

	outro := errors.New("db down")
	assert.Same(t, outro, shareddb.SemValorDuplicado(outro))
	assert.Equal(t, "Duplicate entry (valor omitido)", shareddb.SemValorDuplicado(&mysql.MySQLError{Number: 1062, Message: "x"}).(*mysql.MySQLError).Message)
}
