package vendas_test

// SEC-35 (Lote 28): criação e exclusão de pagamento rodam no TxPedido, com
// a ordem de lock pedido → pagamento (a do TxPedido.Excluir, SEC-26).
//   - Exclusão: fora da tx só a leitura do pagamento (para o pedido_id);
//     na tx, o lock do pedido (só no escopo restrito), o lock do pagamento,
//     a regra de quitado (409) e o DELETE condicionado a não quitado.
//   - Criação: na tx, o lock do pedido (404 também para o admin), a posse,
//     a validação dos campos e o INSERT; o 1452 da FK vira 404 sem o texto
//     do driver.
// O sqlmock confere a ordem exata do SQL (MatchExpectationsInOrder).

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

const (
	sec35LockPagamentoRe = `SELECT pedido_id, status_pagamento FROM pagamentos WHERE pagamento_id = \? FOR UPDATE`
	sec35DeleteCondRe    = `DELETE FROM pagamentos WHERE pagamento_id = \? AND status_pagamento NOT IN \(\?, \?\)`
	sec35InsertRe        = `INSERT INTO pagamentos \(pedido_id, forma_pagamento`
)

// sec35TextoDriver é o trecho da mensagem do 1452 que nunca vai ao log.
const sec35TextoDriver = "Cannot add or update a child row"

func sec35Erro1452() error {
	return &mysql.MySQLError{Number: 1452, Message: sec35TextoDriver + ": a foreign key constraint fails (`rotaperfumes`.`pagamentos`, CONSTRAINT `fk_pagamentos_pedido` FOREIGN KEY (`pedido_id`) REFERENCES `pedidos` (`pedido_id_origem`))"}
}

func sec35LockPagamento(m sqlmock.Sqlmock, id, pedidoID int64, status string) {
	m.ExpectQuery(sec35LockPagamentoRe).WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"pedido_id", "status_pagamento"}).AddRow(pedidoID, status))
}

func sec35Delete(m sqlmock.Sqlmock, id int64, linhas int64) {
	m.ExpectExec(sec35DeleteCondRe).WithArgs(id, "Pago", "Pago com atraso").WillReturnResult(sqlmock.NewResult(0, linhas))
}

func sec35NovoPagamento() vendas.DadosNovoPagamento {
	return vendas.DadosNovoPagamento{PedidoID: 1, DadosPagamento: pagamentoValido()}
}

// --- Exclusão ---------------------------------------------------------------

func TestSEC35_Excluir_Restrito_OrdemDoSQL(t *testing.T) {
	out := bug16CapturarVlog(t)
	p, e := sec26Restrito(t)
	pool, mock := novoMock(t)
	mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
	sec26Lock(mock, 1, "Pendente", 2)
	sec35LockPagamento(mock, 5, 1, "Em aberto")
	sec35Delete(mock, 5, 1)
	mock.ExpectCommit()

	require.NoError(t, novosCasos(pool, e).ExcluirPagamento(context.Background(), p, 5))
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, out.String(), "[pagamentos] excluído: pagamento_id=5")
}

func TestSEC35_Excluir_Admin_SemLockDoPedido(t *testing.T) {
	pool, mock := novoMock(t)
	mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
	mock.ExpectBegin()
	sec35LockPagamento(mock, 5, 1, "Inadimplente")
	sec35Delete(mock, 5, 1)
	mock.ExpectCommit()

	require.NoError(t, novosCasos(pool, escopoIrrestrito(t)).ExcluirPagamento(context.Background(), admin(t), 5))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC35_Excluir_QuitadoSobLock_409Rollback(t *testing.T) {
	for _, status := range []string{"Pago", "Pago com atraso"} {
		t.Run(status, func(t *testing.T) {
			out := bug16CapturarVlog(t)
			p, e := sec26Restrito(t)
			pool, mock := novoMock(t)
			// Fora da tx ainda "Em aberto": quitado entre a leitura e o lock.
			mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
			sec26Lock(mock, 1, "Pendente", 2)
			sec35LockPagamento(mock, 5, 1, status)
			mock.ExpectRollback()

			err := novosCasos(pool, e).ExcluirPagamento(context.Background(), p, 5)
			assert.Same(t, vendas.ErrPagamentoJaQuitadoNaoPodeSerExcluido, err)
			assert.NoError(t, mock.ExpectationsWereMet(), "sem DELETE")
			assert.NotContains(t, out.String(), "excluído")
		})
	}
}

func TestSEC35_Excluir_DeleteSemLinha_404Rollback(t *testing.T) {
	pool, mock := novoMock(t)
	mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
	mock.ExpectBegin()
	sec35LockPagamento(mock, 5, 1, "Em aberto")
	sec35Delete(mock, 5, 0)
	mock.ExpectRollback()

	err := novosCasos(pool, escopoIrrestrito(t)).ExcluirPagamento(context.Background(), admin(t), 5)
	assert.Same(t, vendas.ErrPagamentoNaoEncontrado, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC35_Excluir_PosseSobLock_404Rollback(t *testing.T) {
	casos := []struct {
		nome    string
		prepara func(m sqlmock.Sqlmock)
		log     string
	}{
		{"outro vendedor sob o lock", func(m sqlmock.Sqlmock) {
			sec26Lock(m, 1, "Pendente", 99)
		}, "[TxPedido.posseDoPagamentoTravada] decisão fora_da_carteira pedido_id=1"},
		{"pedido sumiu no lock", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(bug16StatusLockRe).WithArgs(int64(1)).WillReturnError(sql.ErrNoRows)
		}, "[pagamentos] DeletePagamento vendedor do pedido:"},
		{"erro de banco no lock do pedido (V-02)", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(bug16StatusLockRe).WithArgs(int64(1)).WillReturnError(errors.New("conexão caiu"))
		}, "[pagamentos] DeletePagamento vendedor do pedido:"},
		{"pagamento sumiu no lock", func(m sqlmock.Sqlmock) {
			sec26Lock(m, 1, "Pendente", 2)
			m.ExpectQuery(sec35LockPagamentoRe).WithArgs(int64(5)).WillReturnError(sql.ErrNoRows)
		}, "[Pagamentos.TravarParaExcluir] SELECT pagamentos FOR UPDATE linhas=0"},
		{"pedido do pagamento divergente", func(m sqlmock.Sqlmock) {
			sec26Lock(m, 1, "Pendente", 2)
			sec35LockPagamento(m, 5, 77, "Em aberto")
		}, "[TxPedido.ExcluirPagamento] decisão pedido_divergente id=5"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out := bug16CapturarVlog(t)
			p, e := sec26Restrito(t)
			pool, mock := novoMock(t)
			mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
			c.prepara(mock)
			mock.ExpectRollback()

			err := novosCasos(pool, e).ExcluirPagamento(context.Background(), p, 5)
			assert.Same(t, vendas.ErrPagamentoNaoEncontrado, err)
			assert.NoError(t, mock.ExpectationsWereMet(), "sem DELETE")
			assert.Contains(t, out.String(), c.log)
		})
	}
}

func TestSEC35_Excluir_Admin_ErroNoLockDoPagamento_500(t *testing.T) {
	errDB := errors.New("conexão caiu")
	pool, mock := novoMock(t)
	mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
	mock.ExpectBegin()
	mock.ExpectQuery(sec35LockPagamentoRe).WithArgs(int64(5)).WillReturnError(errDB)
	mock.ExpectRollback()

	err := novosCasos(pool, escopoIrrestrito(t)).ExcluirPagamento(context.Background(), admin(t), 5)
	require.ErrorIs(t, err, errDB)
	assert.NotErrorIs(t, err, vendas.ErrPagamentoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC35_Excluir_InexistenteForaDaTx_404SemTx(t *testing.T) {
	pool, mock := novoMock(t)
	mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnError(sql.ErrNoRows)

	err := novosCasos(pool, escopoIrrestrito(t)).ExcluirPagamento(context.Background(), admin(t), 5)
	assert.ErrorIs(t, err, vendas.ErrPagamentoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet(), "sem BEGIN")
}

// --- Criação ----------------------------------------------------------------

func TestSEC35_Criar_OrdemDoSQL(t *testing.T) {
	restritoP, restritoE := sec26Restrito(t)
	casos := []struct {
		nome   string
		chamar func(pool *sql.DB) (*vendas.Pagamento, error)
	}{
		{"admin", func(pool *sql.DB) (*vendas.Pagamento, error) {
			return novosCasos(pool, escopoIrrestrito(t)).CriarPagamento(context.Background(), admin(t), ler(sec35NovoPagamento()))
		}},
		{"restrito na carteira", func(pool *sql.DB) (*vendas.Pagamento, error) {
			return novosCasos(pool, restritoE).CriarPagamento(context.Background(), restritoP, ler(sec35NovoPagamento()))
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pool, mock := novoMock(t)
			sec26Lock(mock, 1, "Pendente", 2)
			mock.ExpectExec(sec35InsertRe).WillReturnResult(sqlmock.NewResult(42, 1))
			mock.ExpectCommit()
			mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(42)).WillReturnRows(arq05LinhaPagamento(42, 1))

			pag, err := c.chamar(pool)
			require.NoError(t, err)
			assert.Equal(t, int64(42), pag.PagamentoID)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSEC35_Criar_PedidoSumiuNoLock_404(t *testing.T) {
	restritoP, restritoE := sec26Restrito(t)
	casos := []struct {
		nome   string
		chamar func(pool *sql.DB) (*vendas.Pagamento, error)
	}{
		{"admin", func(pool *sql.DB) (*vendas.Pagamento, error) {
			return novosCasos(pool, escopoIrrestrito(t)).CriarPagamento(context.Background(), admin(t), ler(sec35NovoPagamento()))
		}},
		{"restrito", func(pool *sql.DB) (*vendas.Pagamento, error) {
			return novosCasos(pool, restritoE).CriarPagamento(context.Background(), restritoP, ler(sec35NovoPagamento()))
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pool, mock := novoMock(t)
			mock.ExpectBegin()
			mock.ExpectQuery(bug16StatusLockRe).WithArgs(int64(1)).WillReturnError(sql.ErrNoRows)
			mock.ExpectRollback()

			pag, err := c.chamar(pool)
			assert.Nil(t, pag)
			assert.Same(t, vendas.ErrPedidoNaoEncontrado, err)
			assert.Equal(t, "pedido não encontrado", err.Error())
			assert.NoError(t, mock.ExpectationsWereMet(), "sem INSERT")
		})
	}
}

func TestSEC35_Criar_OutroVendedorSobLock_404Rollback(t *testing.T) {
	out := bug16CapturarVlog(t)
	p, e := sec26Restrito(t)
	pool, mock := novoMock(t)
	sec26Lock(mock, 1, "Pendente", 99)
	mock.ExpectRollback()

	pag, err := novosCasos(pool, e).CriarPagamento(context.Background(), p, ler(sec35NovoPagamento()))
	assert.Nil(t, pag)
	assert.Same(t, vendas.ErrPedidoNaoEncontrado, err)
	assert.NoError(t, mock.ExpectationsWereMet(), "sem INSERT")
	assert.Contains(t, out.String(), "[TxPedido.CriarPagamento] decisão fora_da_carteira id=1")
}

func TestSEC35_Criar_ErroDeBancoNoLock_500(t *testing.T) {
	errDB := errors.New("conexão caiu")
	pool, mock := novoMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(bug16StatusLockRe).WithArgs(int64(1)).WillReturnError(errDB)
	mock.ExpectRollback()

	_, err := novosCasos(pool, escopoIrrestrito(t)).CriarPagamento(context.Background(), admin(t), ler(sec35NovoPagamento()))
	require.ErrorIs(t, err, errDB)
	assert.NotErrorIs(t, err, vendas.ErrPedidoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC35_Criar_Insert1452_404SemTextoDoDriver(t *testing.T) {
	out := bug16CapturarVlog(t)
	pool, mock := novoMock(t)
	sec26Lock(mock, 1, "Pendente", 2)
	mock.ExpectExec(sec35InsertRe).WillReturnError(sec35Erro1452())
	mock.ExpectRollback()

	pag, err := novosCasos(pool, escopoIrrestrito(t)).CriarPagamento(context.Background(), admin(t), ler(sec35NovoPagamento()))
	assert.Nil(t, pag)
	assert.Same(t, vendas.ErrPedidoNaoEncontrado, err, "sentinela sem embrulhar o erro do driver")
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NotContains(t, out.String(), sec35TextoDriver)
	assert.NotContains(t, out.String(), "fk_pagamentos_pedido")
}

func TestSEC35_Criar_PedidoIDObrigatorio_400SemTx(t *testing.T) {
	pool, mock := novoMock(t)
	_, err := novosCasos(pool, escopoIrrestrito(t)).CriarPagamento(context.Background(), admin(t),
		ler(vendas.DadosNovoPagamento{DadosPagamento: pagamentoValido()}))
	assert.ErrorIs(t, err, vendas.ErrPedidoIDObrigatorio)
	assert.NoError(t, mock.ExpectationsWereMet(), "sem BEGIN")
}

// --- Adapter e helper -------------------------------------------------------

func TestSEC35_ViolacaoFK(t *testing.T) {
	casos := []struct {
		nome       string
		err        error
		constraint string
		want       bool
	}{
		{"1452 da FK", sec35Erro1452(), "fk_pagamentos_pedido", true},
		{"1452 embrulhado", errors.Join(errors.New("ctx"), sec35Erro1452()), "fk_pagamentos_pedido", true},
		{"1452 sem filtro de constraint", sec35Erro1452(), "", true},
		{"1452 de outra FK", sec35Erro1452(), "fk_outra", false},
		{"outro código com o nome da FK", &mysql.MySQLError{Number: 1062, Message: "fk_pagamentos_pedido"}, "fk_pagamentos_pedido", false},
		{"erro comum", errors.New("fk_pagamentos_pedido"), "fk_pagamentos_pedido", false},
		{"nil", nil, "fk_pagamentos_pedido", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			assert.Equal(t, c.want, db.ViolacaoFK(c.err, c.constraint))
		})
	}
}

func TestSEC35_Adapter_Inserir_OutroErroContinuaEmbrulhado(t *testing.T) {
	pool, mock := novoMock(t)
	outra := &mysql.MySQLError{Number: 1452, Message: "a foreign key constraint fails (CONSTRAINT `fk_outra`)"}
	mock.ExpectExec(sec35InsertRe).WillReturnError(outra)

	pag, err := vendas.DadosNovoPagamento{PedidoID: 1, DadosPagamento: pagamentoValido()}.Pagamento(1)
	require.NoError(t, err)
	err = vendasmysql.NovoPagamentos(pool).Inserir(context.Background(), pag)
	assert.ErrorIs(t, err, outra)
	assert.NotErrorIs(t, err, vendas.ErrPedidoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC35_Adapter_TravarParaExcluir(t *testing.T) {
	t.Run("lê pedido e status", func(t *testing.T) {
		pool, mock := novoMock(t)
		sec35LockPagamento(mock, 5, 1, "Pago")
		pag, err := vendasmysql.NovoPagamentos(pool).TravarParaExcluir(context.Background(), 5)
		require.NoError(t, err)
		assert.Equal(t, vendas.Pagamento{PagamentoID: 5, PedidoID: 1, StatusPagamento: "Pago"}, *pag)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("sem linha → 404", func(t *testing.T) {
		pool, mock := novoMock(t)
		mock.ExpectQuery(sec35LockPagamentoRe).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"pedido_id", "status_pagamento"}))
		_, err := vendasmysql.NovoPagamentos(pool).TravarParaExcluir(context.Background(), 5)
		assert.Same(t, vendas.ErrPagamentoNaoEncontrado, err)
	})
	t.Run("erro de banco embrulhado", func(t *testing.T) {
		pool, mock := novoMock(t)
		errDB := errors.New("falha")
		mock.ExpectQuery(sec35LockPagamentoRe).WithArgs(int64(5)).WillReturnError(errDB)
		_, err := vendasmysql.NovoPagamentos(pool).TravarParaExcluir(context.Background(), 5)
		assert.ErrorIs(t, err, errDB)
		assert.NotErrorIs(t, err, vendas.ErrPagamentoNaoEncontrado)
	})
}
