package vendas_test

// SEC-26 (Lote 27): TOCTOU de posse no PUT/DELETE de pedido. A posse é
// conferida de novo DENTRO da tx, sob o lock da linha do pedido
// (TravarParaEscrita: SELECT status, vendedor_id … FOR UPDATE). No DELETE,
// existência, posse, pagamentos e status são lidos só dentro da tx, na
// mesma ordem de erros do legado (404 → 404 → 409s).

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

const (
	sec26PagamentosRe   = `SELECT 1 FROM pagamentos WHERE pedido_id = \? LIMIT 1`
	sec26DeleteItensRe  = `DELETE FROM itens_pedido WHERE pedido_id = \?`
	sec26DeletePedidoRe = `DELETE FROM pedidos WHERE pedido_id_origem = \?`
)

// sec26Restrito: usuário normal com escopo do vendedor 2 (o dono dos
// fixtures bug16LinhaPedido).
func sec26Restrito(t *testing.T) (kernel.Principal, kernel.Escopo) {
	t.Helper()
	p, err := kernel.NovoPrincipal(7, kernel.RoleNormal)
	require.NoError(t, err)
	e, err := kernel.EscopoDoVendedor(2)
	require.NoError(t, err)
	return p, e
}

func sec26Lock(m sqlmock.Sqlmock, id int64, status string, vendedorID int64) {
	m.ExpectBegin()
	m.ExpectQuery(bug16StatusLockRe).WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}).AddRow(status, vendedorID))
}

// sec26PreCheckPut é a checagem de posse antes do decode (E-05: 404 antes
// do 400), mantida no escopo restrito.
func sec26PreCheckPut(m sqlmock.Sqlmock, id int64) {
	m.ExpectQuery(arq05PedidoPorIDRe).WithArgs(id).WillReturnRows(bug16LinhaPedido(id, "Faturado"))
	m.ExpectQuery(arq05ItensRe).WithArgs(id).WillReturnRows(sqlmock.NewRows(nil))
}

// sec26PutSemMudarItens: pedido Faturado com os mesmos itens só regrava o
// cabeçalho (sem troca de itens, sem baixa).
func sec26PutSemMudarItens(m sqlmock.Sqlmock, id int64) {
	sec26Lock(m, id, "Faturado", 2)
	m.ExpectQuery(bug16ItensAtuaRe).WithArgs(id).WillReturnRows(bug16ItensAtuais())
	m.ExpectExec(`UPDATE pedidos`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	bug16Releitura(m, id, "Faturado")
}

func TestSEC26_PUT_PosseMudouEntreOPreCheckEOLock_404Rollback(t *testing.T) {
	out := bug16CapturarVlog(t)
	p, e := sec26Restrito(t)
	pool, mock := novoMock(t)
	sec26PreCheckPut(mock, 1)
	sec26Lock(mock, 1, "Pendente", 99) // transferido para o vendedor 99 nesse meio-tempo
	mock.ExpectRollback()

	ped, err := novosCasos(pool, e).AtualizarPedido(context.Background(), p, 1, ler(pedidoValido()))
	assert.Nil(t, ped)
	assert.Same(t, vendas.ErrPedidoNaoEncontrado, err)
	assert.NoError(t, mock.ExpectationsWereMet(), "sem leitura de itens, sem UPDATE")
	assert.Contains(t, out.String(), "[TxPedido.Atualizar] decisão fora_da_carteira id=1")
}

func TestSEC26_PUT_InexistenteNoLock_404Rollback(t *testing.T) {
	pool, mock := novoMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(bug16StatusLockRe).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}))
	mock.ExpectRollback()

	_, err := novosCasos(pool, escopoIrrestrito(t)).AtualizarPedido(context.Background(), admin(t), 1, ler(pedidoValido()))
	assert.Same(t, vendas.ErrPedidoNaoEncontrado, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC26_PUT_DonoSobOLock_Commit(t *testing.T) {
	p, e := sec26Restrito(t)
	pool, mock := novoMock(t)
	sec26PreCheckPut(mock, 1)
	sec26PutSemMudarItens(mock, 1)

	ped, err := novosCasos(pool, e).AtualizarPedido(context.Background(), p, 1, ler(pedidoValido()))
	require.NoError(t, err)
	assert.Equal(t, int64(1), ped.PedidoIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Ordem exata do DELETE: Begin → lock → pagamentos → DELETE itens →
// DELETE pedido → Commit; nenhuma leitura fora da tx.
func TestSEC26_DELETE_OrdemNaTx(t *testing.T) {
	p, e := sec26Restrito(t)
	pool, mock := novoMock(t)
	sec26Lock(mock, 1, "Em separação", 2)
	mock.ExpectQuery(sec26PagamentosRe).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectExec(sec26DeleteItensRe).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(sec26DeletePedidoRe).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, novosCasos(pool, e).ExcluirPedido(context.Background(), p, 1))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC26_DELETE_Recusas_Rollback(t *testing.T) {
	casos := []struct {
		nome    string
		prepara func(m sqlmock.Sqlmock)
		want    error
	}{
		{"inexistente no lock → 404", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(bug16StatusLockRe).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}))
		}, vendas.ErrPedidoNaoEncontrado},
		{"de outro vendedor sob o lock → 404, sem ler pagamentos", func(m sqlmock.Sqlmock) {
			sec26Lock(m, 1, "Em separação", 99)
		}, vendas.ErrPedidoNaoEncontrado},
		{"com pagamento → 409", func(m sqlmock.Sqlmock) {
			sec26Lock(m, 1, "Em separação", 2)
			m.ExpectQuery(sec26PagamentosRe).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		}, vendas.ErrPedidoPossuiPagamentosVinculados},
		{"Faturado → 409", func(m sqlmock.Sqlmock) {
			sec26Lock(m, 1, "Faturado", 2)
			m.ExpectQuery(sec26PagamentosRe).WithArgs(int64(1)).WillReturnError(sql.ErrNoRows)
		}, vendas.ErrPedidoFaturadoNaoPodeSerExcluido},
		{"Entregue → 409", func(m sqlmock.Sqlmock) {
			sec26Lock(m, 1, "Entregue", 2)
			m.ExpectQuery(sec26PagamentosRe).WithArgs(int64(1)).WillReturnError(sql.ErrNoRows)
		}, vendas.ErrPedidoEntregueNaoPodeSerExcluido},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p, e := sec26Restrito(t)
			pool, mock := novoMock(t)
			c.prepara(mock)
			mock.ExpectRollback()

			err := novosCasos(pool, e).ExcluirPedido(context.Background(), p, 1)
			assert.Same(t, c.want, err)
			assert.NoError(t, mock.ExpectationsWereMet(), "rollback, sem DELETE e sem commit")
		})
	}
}

func TestSEC26_DELETE_FalhaNaLeituraDePagamentos_500Rollback(t *testing.T) {
	errBanco := errors.New("db down")
	pool, mock := novoMock(t)
	sec26Lock(mock, 1, "Em separação", 2)
	mock.ExpectQuery(sec26PagamentosRe).WillReturnError(errBanco)
	mock.ExpectRollback()

	err := novosCasos(pool, escopoIrrestrito(t)).ExcluirPedido(context.Background(), admin(t), 1)
	assert.ErrorIs(t, err, errBanco)
	var de *kernel.Erro
	assert.False(t, errors.As(err, &de))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Prova de que a tx usa só os repositórios dela: com uma conexão só no
// pool, qualquer consulta pelo pool dentro da fn esperaria a conexão presa
// na tx até o deadline curto.
func TestSEC26_TudoNaMesmaConexao(t *testing.T) {
	t.Run("PUT", func(t *testing.T) {
		p, e := sec26Restrito(t)
		pool, mock := novoMock(t)
		pool.SetMaxOpenConns(1)
		sec26PreCheckPut(mock, 1)
		sec26PutSemMudarItens(mock, 1)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := novosCasos(pool, e).AtualizarPedido(ctx, p, 1, ler(pedidoValido()))
		require.NoError(t, err, "uso do pool dentro da tx travaria até o deadline")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("DELETE", func(t *testing.T) {
		p, e := sec26Restrito(t)
		pool, mock := novoMock(t)
		pool.SetMaxOpenConns(1)
		sec26Lock(mock, 1, "Em separação", 2)
		mock.ExpectQuery(sec26PagamentosRe).WillReturnRows(sqlmock.NewRows([]string{"1"}))
		mock.ExpectExec(sec26DeleteItensRe).WillReturnResult(sqlmock.NewResult(0, 2))
		mock.ExpectExec(sec26DeletePedidoRe).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, novosCasos(pool, e).ExcluirPedido(ctx, p, 1), "uso do pool dentro da tx travaria até o deadline")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// Adapter: TravarParaEscrita preenche id, status e vendedor.
func TestSEC26_TravarParaEscrita_PreencheStatusEVendedor(t *testing.T) {
	pool, mock := novoMock(t)
	mock.ExpectQuery(bug16StatusLockRe).WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}).AddRow("Pendente", int64(3)))
	got, err := vendasmysql.NovoPedidos(pool).TravarParaEscrita(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, &vendas.Pedido{PedidoIDOrigem: 5, Status: "Pendente", VendedorID: 3}, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}
