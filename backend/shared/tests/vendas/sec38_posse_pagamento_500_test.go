package vendas_test

// SEC-38 (Lote 31): a posse do pagamento (V-02) deixa de esconder erro de
// banco como 404.
//   - posseDoPagamento (Get, sem lock): pedido inexistente ou fora da
//     carteira → 404 "pagamento não encontrado"; outro erro → 500 com log
//     (pedido_id e erro).
//   - posseDoPagamentoTravada (Delete) e travarNoEscopo (PUT/DELETE de
//     pedido, POST de pagamento), no escopo restrito: (1) leitura sem lock
//     → 404/500; (2) FOR UPDATE → sumiu 404, outro 500; (3) posse sob o
//     lock → 404. O admin só trava (sem mudança).

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

var errSEC38 = errors.New("conexão caiu")

// sec38Erro500: erro de infraestrutura, nunca um sentinela de domínio.
func sec38Erro500(t *testing.T, err error) {
	t.Helper()
	require.ErrorIs(t, err, errSEC38)
	var de *kernel.Erro
	assert.False(t, errors.As(err, &de), "500, não um 404 de domínio")
}

func TestSEC38_ObterPagamento_ErroAoLerPedido_500ComLog(t *testing.T) {
	out := bug16CapturarVlog(t)
	p, e := sec26Restrito(t)
	pool, mock := novoMock(t)
	mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
	mock.ExpectQuery(arq05PedidoPorIDRe).WithArgs(int64(1)).WillReturnError(errSEC38)

	_, err := novosCasos(pool, e).ObterPagamento(context.Background(), p, 5)

	sec38Erro500(t, err)
	assert.Contains(t, out.String(), "[pagamentos] GetPagamento vendedor do pedido: pedido_id=1:")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC38_ObterPagamento_PedidoInexistenteOuAlheio_404(t *testing.T) {
	p, e := sec26Restrito(t)
	t.Run("inexistente", func(t *testing.T) {
		pool, mock := novoMock(t)
		mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
		mock.ExpectQuery(arq05PedidoPorIDRe).WithArgs(int64(1)).WillReturnError(sql.ErrNoRows)

		_, err := novosCasos(pool, e).ObterPagamento(context.Background(), p, 5)
		assert.Same(t, vendas.ErrPagamentoNaoEncontrado, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("de outro vendedor", func(t *testing.T) {
		pool, mock := novoMock(t)
		mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
		mock.ExpectQuery(arq05PedidoPorIDRe).WithArgs(int64(1)).WillReturnRows(arq05LinhaPedido(1, 99))

		_, err := novosCasos(pool, e).ObterPagamento(context.Background(), p, 5)
		assert.Same(t, vendas.ErrPagamentoNaoEncontrado, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSEC38_ExcluirPagamento_Restrito_ErroDeBanco_500Rollback(t *testing.T) {
	casos := []struct {
		nome    string
		prepara func(m sqlmock.Sqlmock)
	}{
		{"na leitura sem lock", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(sec38LeituraRe).WithArgs(int64(1)).WillReturnError(errSEC38)
		}},
		{"no FOR UPDATE do pedido", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			sec38Leitura(m, 1, 2)
			m.ExpectQuery(bug16StatusLockRe).WithArgs(int64(1)).WillReturnError(errSEC38)
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p, e := sec26Restrito(t)
			pool, mock := novoMock(t)
			mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
			c.prepara(mock)
			mock.ExpectRollback()

			err := novosCasos(pool, e).ExcluirPagamento(context.Background(), p, 5)
			sec38Erro500(t, err)
			assert.NoError(t, mock.ExpectationsWereMet(), "sem lock do pagamento, sem DELETE")
		})
	}
}

func TestSEC38_ExcluirPedido_Restrito_ErroNaLeituraSemLock_500(t *testing.T) {
	p, e := sec26Restrito(t)
	pool, mock := novoMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(sec38LeituraRe).WithArgs(int64(1)).WillReturnError(errSEC38)
	mock.ExpectRollback()

	err := novosCasos(pool, e).ExcluirPedido(context.Background(), p, 1)
	sec38Erro500(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC38_CriarPagamento_Restrito_AlheioNaLeitura_404SemLock(t *testing.T) {
	p, e := sec26Restrito(t)
	pool, mock := novoMock(t)
	mock.ExpectBegin()
	sec38Leitura(mock, 1, 99)
	mock.ExpectRollback()

	_, err := novosCasos(pool, e).CriarPagamento(context.Background(), p, ler(sec35NovoPagamento()))
	assert.Same(t, vendas.ErrPedidoNaoEncontrado, err)
	assert.NoError(t, mock.ExpectationsWereMet(), "o restrito não trava pedido de outro vendedor")
}

func TestSEC38_Admin_SemLeituraSemLock(t *testing.T) {
	pool, mock := novoMock(t)
	sec26Lock(mock, 1, "Em separação", 2)
	mock.ExpectQuery(sec26PagamentosRe).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectExec(sec26DeleteItensRe).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(sec26DeletePedidoRe).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, novosCasos(pool, escopoIrrestrito(t)).ExcluirPedido(context.Background(), admin(t), 1))
	assert.NoError(t, mock.ExpectationsWereMet())
}
