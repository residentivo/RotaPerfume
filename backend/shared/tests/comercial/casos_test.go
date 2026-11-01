package comercial_test

// ARQ-04: casos de uso do Comercial sobre os adapters reais (sqlmock) e
// fakes das portas de Identidade. Cobrem os ramos que os testes HTTP não
// alcançam: defesa em profundidade da gestão (admin), Principal inválido,
// corridas no vínculo e erros de infraestrutura.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialapp"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
)

var errInfra = errors.New("infra fora do ar")

// vinculoFake é o vínculo usuário→vendedor (porta para Identidade).
type vinculoFake struct {
	id  int64
	err error
}

func (v vinculoFake) VendedorDoUsuario(context.Context, int64) (int64, bool, error) {
	return v.id, v.id > 0, v.err
}

type identidadeTxFake struct{}

func (identidadeTxFake) InativarUsuariosDoVendedor(context.Context, int64) (int64, error) {
	return 0, nil
}
func (identidadeTxFake) RevogarSessoesDoVendedor(context.Context, int64) (int64, error) {
	return 0, nil
}

var agoraFixo = time.Date(2026, 5, 6, 10, 0, 0, 0, time.Local)

func novosCasos(pool *sql.DB, v comercialapp.VinculoUsuario) *comercialapp.Casos {
	return comercialapp.Novo(comercialapp.Deps{
		Vendedores: comercialmysql.NovoVendedores(pool), Carteiras: comercialmysql.NovoCarteiras(pool),
		Clientes: comercialmysql.NovoClientes(pool), Visitas: comercialmysql.NovoVisitas(pool),
		Oportunidades: comercialmysql.NovoOportunidades(pool), Vinculo: v,
		Desligamento: db.NewTransactor(pool, func(q db.Querier) comercialapp.TxDesligamento {
			return comercialapp.TxDesligamento{Vendedores: comercialmysql.NovoVendedores(q), Identidade: identidadeTxFake{}}
		}),
		ClienteNaCarteira: db.NewTransactor(pool, func(q db.Querier) comercialapp.TxClienteNaCarteira {
			return comercialapp.TxClienteNaCarteira{Clientes: comercialmysql.NovoClientes(q), Carteiras: comercialmysql.NovoCarteiras(q)}
		}),
		Vinculacao: db.NewTransactor(pool, func(q db.Querier) comercialapp.TxVinculo {
			return comercialapp.TxVinculo{Clientes: comercialmysql.NovoClientes(q), Carteiras: comercialmysql.NovoCarteiras(q)}
		}),
		Relogio: func() time.Time { return agoraFixo },
	})
}

func principal(t *testing.T, role kernel.Role) kernel.Principal {
	p, err := kernel.NovoPrincipal(7, role)
	require.NoError(t, err)
	return p
}

func TestComercialapp_Novo_PanicSemDependencia(t *testing.T) {
	pool, _ := novoMock(t)
	assert.Panics(t, func() { comercialapp.Novo(comercialapp.Deps{}) })
	assert.Panics(t, func() {
		d := comercialapp.Deps{
			Vendedores: comercialmysql.NovoVendedores(pool), Carteiras: comercialmysql.NovoCarteiras(pool),
			Clientes: comercialmysql.NovoClientes(pool), Visitas: comercialmysql.NovoVisitas(pool),
			Oportunidades: comercialmysql.NovoOportunidades(pool), Vinculo: vinculoFake{},
			Desligamento:      db.NewTransactor(pool, func(db.Querier) comercialapp.TxDesligamento { return comercialapp.TxDesligamento{} }),
			ClienteNaCarteira: db.NewTransactor(pool, func(db.Querier) comercialapp.TxClienteNaCarteira { return comercialapp.TxClienteNaCarteira{} }),
			Vinculacao:        db.NewTransactor(pool, func(db.Querier) comercialapp.TxVinculo { return comercialapp.TxVinculo{} }),
		}
		comercialapp.Novo(d) // sem relógio
	})
	assert.Panics(t, func() { comercialapp.NovoAutorizadorEscopo(nil, nil, nil) })
}

// Defesa em profundidade (a rota já é AdminOnly): Principal inválido → 500
// (ErrPrincipalInvalido); não admin → 403; nenhum SQL nos dois casos.
func TestComercialapp_GestaoDeVendedor_SoAdmin(t *testing.T) {
	ctx := context.Background()
	ops := map[string]func(c *comercialapp.Casos, p kernel.Principal) error{
		"ObterVendedor": func(c *comercialapp.Casos, p kernel.Principal) error {
			_, err := c.ObterVendedor(ctx, p, 1)
			return err
		},
		"CriarVendedor": func(c *comercialapp.Casos, p kernel.Principal) error {
			_, err := c.CriarVendedor(ctx, p, comercial.DadosVendedor{})
			return err
		},
		"AtualizarVendedor": func(c *comercialapp.Casos, p kernel.Principal) error {
			_, err := c.AtualizarVendedor(ctx, p, 1, comercial.DadosVendedor{})
			return err
		},
		"DesligarVendedor": func(c *comercialapp.Casos, p kernel.Principal) error {
			_, err := c.DesligarVendedor(ctx, p, 1)
			return err
		},
		"ReativarVendedor": func(c *comercialapp.Casos, p kernel.Principal) error {
			_, err := c.ReativarVendedor(ctx, p, 1)
			return err
		},
		"VincularCliente": func(c *comercialapp.Casos, p kernel.Principal) error {
			_, err := c.VincularCliente(ctx, p, 1, 2)
			return err
		},
		"DesvincularCliente": func(c *comercialapp.Casos, p kernel.Principal) error { return c.DesvincularCliente(ctx, p, 1, 2) },
	}
	for nome, op := range ops {
		t.Run(nome, func(t *testing.T) {
			pool, mock := novoMock(t)
			c := novosCasos(pool, vinculoFake{id: 3})
			assert.Same(t, kernel.ErrPrincipalInvalido, op(c, kernel.Principal{}))
			assert.Same(t, comercialapp.ErrAcessoRestritoAdmin, op(c, principal(t, kernel.RoleNormal)))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// E-01: Principal inválido → SemAcesso sem consulta (o legado tratava a
// falta de usuário como o escopo mais restritivo); erro do vínculo → 500.
func TestComercialapp_Autorizador(t *testing.T) {
	ctx := context.Background()
	pool, mock := novoMock(t)
	a := novosCasos(pool, vinculoFake{err: errInfra}).Escopo()

	e, err := a.Resolver(ctx, kernel.Principal{})
	require.NoError(t, err)
	assert.True(t, e.SemAcesso())

	_, err = a.Resolver(ctx, principal(t, kernel.RoleNormal))
	assert.ErrorIs(t, err, errInfra)
	_, _, err = a.ResolverParaAnalytics(ctx, principal(t, kernel.RoleNormal))
	assert.ErrorIs(t, err, errInfra)

	_, err = a.ClienteNaCarteira(ctx, kernel.EscopoSemAcesso(), 1)
	assert.ErrorIs(t, err, kernel.ErrEscopoSemAcesso, "O1: SemAcesso nunca vira consulta")
	ok, err := a.ClienteNaCarteira(ctx, irrestrito(), 1)
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// D-01: desligado não é erro para o Dashboard, vira SemAcesso + aviso.
func TestComercialapp_ResolverParaAnalytics_Desligado(t *testing.T) {
	pool, mock := novoMock(t)
	mock.ExpectQuery(`SELECT data_desligamento IS NOT NULL FROM vendedores`).WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(1))
	e, desligado, err := novosCasos(pool, vinculoFake{id: 3}).Escopo().ResolverParaAnalytics(context.Background(), principal(t, kernel.RoleNormal))
	require.NoError(t, err)
	assert.True(t, desligado)
	assert.True(t, e.SemAcesso())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComercialapp_CriarCliente_SemAcessoAntesDoDecode(t *testing.T) {
	pool, mock := novoMock(t)
	leu := false
	_, err := novosCasos(pool, vinculoFake{}).CriarCliente(context.Background(), principal(t, kernel.RoleNormal),
		func() (comercial.DadosCliente, error) { leu = true; return comercial.DadosCliente{}, nil })
	assert.Same(t, comercial.ErrSemVendedor, err)
	assert.False(t, leu, "E-05: o body não é lido sem escopo")
	assert.NoError(t, mock.ExpectationsWereMet())
}

const (
	reExisteVendedor = `SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`
	reExisteCliente  = `SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`
	reClientePorID   = `FROM clientes WHERE cliente_id_origem = \? LIMIT 1`
	reAtivoDoCliente = `WHERE cliente_id = \? AND data_fim IS NULL`
	reDoDia          = `WHERE cliente_id = \? AND vendedor_id = \? AND data_inicio = \?`
	reTravarCliente  = `SELECT cliente_id_origem FROM clientes WHERE cliente_id_origem = \? FOR UPDATE`
)

var (
	colsCliente  = []string{"cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro", "data_cadastro", "ativo", "created_at", "updated_at"}
	colsCarteira = []string{"carteira_id_origem", "cliente_id", "vendedor_id", "data_inicio", "data_fim", "created_at", "updated_at"}
)

func esperarVincularAte(m sqlmock.Sqlmock) {
	m.ExpectQuery(reExisteVendedor).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	m.ExpectQuery(reClientePorID).WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows(colsCliente).AddRow(int64(2), "c", "r", "s", "ci", "PR", "", agoraFixo, true, agoraFixo, agoraFixo))
	esperarTravaDoCliente(m)
}

// esperarTravaDoCliente: a tx do vínculo (BUG-14) abre travando o cliente.
func esperarTravaDoCliente(m sqlmock.Sqlmock) {
	m.ExpectBegin()
	m.ExpectQuery(reTravarCliente).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"cliente_id_origem"}).AddRow(int64(2)))
}

// BUG-14: a sequência roda numa tx. "Não encontrado" ao escrever um
// vínculo recém-lido é corrida: 500 com rollback, como no legado (nunca 404).
func TestComercialapp_VincularCliente_Corridas(t *testing.T) {
	ctx := context.Background()
	admin := principal(t, kernel.RoleAdmin)
	fim := agoraFixo

	t.Run("encerrar anterior sem linha → 500", func(t *testing.T) {
		pool, mock := novoMock(t)
		esperarVincularAte(mock)
		mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(sqlmock.NewRows(colsCarteira).AddRow(int64(9), int64(2), int64(5), agoraFixo, nil, agoraFixo, agoraFixo))
		mock.ExpectExec(`UPDATE carteiras SET data_fim = \?`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()
		_, err := novosCasos(pool, vinculoFake{}).VincularCliente(ctx, admin, 1, 2)
		require.Error(t, err)
		var de *kernel.Erro
		assert.False(t, errors.As(err, &de), "erro interno, não o sentinela 404")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("vínculo do dia já ativo é idempotente", func(t *testing.T) {
		pool, mock := novoMock(t)
		esperarVincularAte(mock)
		mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(sqlmock.NewRows(colsCarteira))
		mock.ExpectQuery(reDoDia).WillReturnRows(sqlmock.NewRows(colsCarteira).AddRow(int64(4), int64(2), int64(1), agoraFixo, nil, agoraFixo, agoraFixo))
		mock.ExpectCommit()
		r, err := novosCasos(pool, vinculoFake{}).VincularCliente(ctx, admin, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(4), r.CarteiraID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("reabrir sem linha → 500", func(t *testing.T) {
		pool, mock := novoMock(t)
		esperarVincularAte(mock)
		mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(sqlmock.NewRows(colsCarteira))
		mock.ExpectQuery(reDoDia).WillReturnRows(sqlmock.NewRows(colsCarteira).AddRow(int64(4), int64(2), int64(1), agoraFixo, &fim, agoraFixo, agoraFixo))
		mock.ExpectExec(`UPDATE carteiras SET data_fim = NULL`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()
		_, err := novosCasos(pool, vinculoFake{}).VincularCliente(ctx, admin, 1, 2)
		var de *kernel.Erro
		assert.False(t, errors.As(err, &de))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// Erros de infra na existência de cliente/vendedor (visita/oportunidade).
func TestComercialapp_ExistenciaComErroDeInfra(t *testing.T) {
	ctx := context.Background()
	admin := principal(t, kernel.RoleAdmin)
	dados := comercial.DadosVisita{ClienteID: 1, VendedorID: 2, DataVisita: "2024-01-01", Resultado: "ok"}

	pool, mock := novoMock(t)
	mock.ExpectQuery(reExisteCliente).WillReturnError(errInfra)
	_, err := novosCasos(pool, vinculoFake{}).CriarVisita(ctx, admin, func() (comercial.DadosVisita, error) { return dados, nil })
	assert.ErrorIs(t, err, errInfra)

	mock.ExpectQuery(reExisteCliente).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(reExisteVendedor).WillReturnError(errInfra)
	_, err = novosCasos(pool, vinculoFake{}).CriarVisita(ctx, admin, func() (comercial.DadosVisita, error) { return dados, nil })
	assert.ErrorIs(t, err, errInfra)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// E-05: erro de infra na checagem de carteira do restrito → 500.
func TestComercialapp_CarteiraComErroDeInfra(t *testing.T) {
	pool, mock := novoMock(t)
	mock.ExpectQuery(`SELECT data_desligamento IS NOT NULL FROM vendedores`).WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(0))
	mock.ExpectQuery(`WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL`).WillReturnError(errInfra)
	_, err := novosCasos(pool, vinculoFake{id: 3}).CriarOportunidade(context.Background(), principal(t, kernel.RoleNormal),
		func() (comercial.DadosOportunidade, error) { return comercial.DadosOportunidade{ClienteID: 1}, nil })
	assert.ErrorIs(t, err, errInfra)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Erro de infra ao conferir o vendedor da carteira pedida → 500.
func TestComercialapp_ClientesDoVendedor_ErroDeInfra(t *testing.T) {
	pool, mock := novoMock(t)
	mock.ExpectQuery(reExisteVendedor).WillReturnError(errInfra)
	_, err := novosCasos(pool, vinculoFake{}).ClientesDoVendedor(context.Background(), principal(t, kernel.RoleAdmin), 3)
	assert.ErrorIs(t, err, errInfra)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// N3: rota com escopo sem usuário loga a tag fixa "[escopo] principal
// invalido" (sem valores) e responde SemAcesso.
func TestComercialapp_N3_PrincipalInvalidoLogaTagFixa(t *testing.T) {
	var buf bytes.Buffer
	velho := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(velho) })
	pool, _ := novoMock(t)
	e, err := novosCasos(pool, vinculoFake{}).Escopo().Resolver(context.Background(), kernel.Principal{})
	require.NoError(t, err)
	assert.True(t, e.SemAcesso())
	assert.Contains(t, buf.String(), "[escopo] principal invalido")
}
