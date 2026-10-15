package vendas_test

// ARQ-05: casos de uso de Vendas com os adapters reais sobre sqlmock e um
// escopo fixo (fake da porta para Comercial). Cobre os ramos de defesa que
// o fluxo HTTP não alcança: escopo zero value (fail-closed), falhas de
// releitura e falhas de cada passo da tx do pedido (rollback total).

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

// escopoFixo é o fake da porta vendasapp.EscopoCarteira.
type escopoFixo struct{ e kernel.Escopo }

func (f escopoFixo) Resolver(context.Context, kernel.Principal) (kernel.Escopo, error) {
	return f.e, nil
}

func (escopoFixo) ClienteNaCarteira(context.Context, kernel.Escopo, int64) (bool, error) {
	return true, nil
}

func txPedido(pool *sql.DB) *db.Transactor[vendasapp.TxPedido] {
	return db.NewTransactor(pool, func(q db.Querier) vendasapp.TxPedido {
		return vendasapp.TxPedido{Pedidos: vendasmysql.NovoPedidos(q), Estoque: catalogomysql.NovoEstoques(q)}
	})
}

func novosCasos(pool *sql.DB, e kernel.Escopo) *vendasapp.Casos {
	return vendasapp.Novo(vendasapp.Deps{
		Pedidos: vendasmysql.NovoPedidos(pool), Pagamentos: vendasmysql.NovoPagamentos(pool),
		Escopo: escopoFixo{e: e}, TxPedido: txPedido(pool), Relogio: time.Now,
	})
}

func admin(t *testing.T) kernel.Principal {
	t.Helper()
	p, err := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	require.NoError(t, err)
	return p
}

func ler[T any](v T) vendasapp.Leitor[T] { return func() (T, error) { return v, nil } }

func TestVendasapp_Novo_PanicSemDependencia(t *testing.T) {
	pool, _ := novoMock(t)
	completo := func() vendasapp.Deps {
		return vendasapp.Deps{Pedidos: vendasmysql.NovoPedidos(pool), Pagamentos: vendasmysql.NovoPagamentos(pool),
			Escopo: escopoFixo{}, TxPedido: txPedido(pool), Relogio: time.Now}
	}
	assert.NotPanics(t, func() { vendasapp.Novo(completo()) })
	faltas := map[string]func(d *vendasapp.Deps){
		"pedidos":    func(d *vendasapp.Deps) { d.Pedidos = nil },
		"pagamentos": func(d *vendasapp.Deps) { d.Pagamentos = nil },
		"escopo":     func(d *vendasapp.Deps) { d.Escopo = nil },
		"tx":         func(d *vendasapp.Deps) { d.TxPedido = nil },
		"relógio":    func(d *vendasapp.Deps) { d.Relogio = nil },
	}
	for nome, tirar := range faltas {
		d := completo()
		tirar(&d)
		assert.Panics(t, func() { vendasapp.Novo(d) }, nome)
	}
}

// Escopo zero value vindo da porta (autorizador defeituoso): nunca vira
// "global". Listagem e escrita falham fechado (500, ErrEscopoInvalido) sem
// SQL; o detalhe trata o zero value como restrito e esconde o registro.
func TestVendasapp_EscopoZeroValue_FalhaFechado(t *testing.T) {
	ctx := context.Background()
	pool, mock := novoMock(t)
	c := novosCasos(pool, kernel.Escopo{})

	_, err := c.ListarPedidos(ctx, admin(t), vendas.FiltroPedido{}, kernel.NovaPagina(1, 20))
	assert.ErrorIs(t, err, kernel.ErrEscopoInvalido)
	_, err = c.ListarPagamentos(ctx, admin(t), ler(vendas.FiltroPagamento{}), kernel.NovaPagina(1, 20))
	assert.ErrorIs(t, err, kernel.ErrEscopoInvalido)
	_, err = c.CriarPedido(ctx, admin(t), ler(pedidoValido()))
	assert.ErrorIs(t, err, kernel.ErrEscopoInvalido)
	assert.NoError(t, mock.ExpectationsWereMet(), "nenhum SQL com escopo zero value")

	mock.ExpectQuery(`FROM pedidos p .+ WHERE p\.pedido_id_origem = \? LIMIT 1`).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"pedido_id_origem", "cliente_id", "vendedor_id", "data_pedido", "canal", "status",
			"valor_total", "created_at", "updated_at", "c", "v"}).
			AddRow(int64(1), int64(1), int64(2), time.Now(), "App", "Faturado", 1.0, time.Now(), time.Now(), "C", "V"))
	mock.ExpectQuery(`FROM itens_pedido i`).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows(nil))
	_, err = c.ObterPedido(ctx, admin(t), 1)
	assert.ErrorIs(t, err, vendas.ErrPedidoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func escopoIrrestrito(t *testing.T) kernel.Escopo {
	t.Helper()
	e, err := kernel.EscopoIrrestrito(admin(t))
	require.NoError(t, err)
	return e
}

func esperarGravacaoDoPedido(m sqlmock.Sqlmock) {
	m.ExpectBegin()
	m.ExpectExec(`INSERT INTO pedidos`).WillReturnResult(sqlmock.NewResult(100, 1))
	m.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(2, 1))
	m.ExpectCommit()
}

// A releitura depois do commit falhando é 500 (o legado devolvia o erro
// do GetPedidoDetalhe; "não encontrado" seria 404, como no legado).
func TestVendasapp_Pedido_FalhaNaReleitura(t *testing.T) {
	ctx := context.Background()
	errBanco := errors.New("db down")

	pool, mock := novoMock(t)
	esperarGravacaoDoPedido(mock)
	mock.ExpectQuery(`FROM pedidos p`).WillReturnError(errBanco)
	p, err := novosCasos(pool, escopoIrrestrito(t)).CriarPedido(ctx, admin(t), ler(pedidoValido()))
	assert.Nil(t, p)
	assert.ErrorIs(t, err, errBanco)
	assert.NoError(t, mock.ExpectationsWereMet())

	pool, mock = novoMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM pedidos`).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Faturado"))
	mock.ExpectQuery(`SELECT produto_id, quantidade`).WillReturnRows(sqlmock.NewRows([]string{"produto_id", "quantidade", "preco_praticado", "desconto_pct"}).
		AddRow(int64(10), 2, 100.0, 10.0).AddRow(int64(11), 1, 50.0, 0.0))
	mock.ExpectExec(`UPDATE pedidos`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`FROM pedidos p`).WillReturnError(errBanco)
	p, err = novosCasos(pool, escopoIrrestrito(t)).AtualizarPedido(ctx, admin(t), 1, ler(pedidoValido()))
	assert.Nil(t, p)
	assert.ErrorIs(t, err, errBanco)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// UoW do PUT (§5.2): falha na leitura dos itens atuais ou na baixa de
// estoque desfaz TUDO (rollback, inclusive o UPDATE do faturamento).
func TestVendasapp_TxPedido_FalhaEmCadaPassoFazRollback(t *testing.T) {
	ctx := context.Background()
	errBanco := errors.New("db down")

	pool, mock := novoMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM pedidos`).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Pendente"))
	mock.ExpectQuery(`SELECT produto_id, quantidade`).WillReturnError(errBanco)
	mock.ExpectRollback()
	_, err := novosCasos(pool, escopoIrrestrito(t)).AtualizarPedido(ctx, admin(t), 1, ler(pedidoValido()))
	assert.ErrorIs(t, err, errBanco)
	assert.NoError(t, mock.ExpectationsWereMet())

	pool, mock = novoMock(t)
	d := pedidoValido()
	d.Itens = d.Itens[:1]
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM pedidos`).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Pendente"))
	mock.ExpectQuery(`SELECT produto_id, quantidade`).WillReturnRows(sqlmock.NewRows(nil))
	mock.ExpectExec(`UPDATE pedidos`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT sku FROM produtos`).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"sku"}).AddRow("SKU-10"))
	mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
		WithArgs(time.Now().Format(kernel.LayoutData), "SKU-10").WillReturnError(errBanco)
	mock.ExpectRollback()
	_, err = novosCasos(pool, escopoIrrestrito(t)).AtualizarPedido(ctx, admin(t), 1, ler(d))
	assert.ErrorIs(t, err, errBanco)
	assert.ErrorContains(t, err, "vendasapp: baixa de estoque no faturamento do pedido")
	assert.NoError(t, mock.ExpectationsWereMet())
}
