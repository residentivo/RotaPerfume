package comercial_test

// BUG-14 (Lote 27): VincularCliente roda encerrar/reabrir/inserir numa tx
// aberta com o lock da linha do cliente (FOR UPDATE). Qualquer falha desfaz
// tudo (rollback, sem commit) e o log "cliente vinculado" só sai depois do
// commit.

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
)

const (
	reEncerrar = `UPDATE carteiras SET data_fim = \? WHERE carteira_id_origem = \?`
	reReabrir  = `UPDATE carteiras SET data_fim = NULL WHERE carteira_id_origem = \?`
	reInserir  = `INSERT INTO carteiras`
)

// capturarLog redireciona o log padrão para um buffer durante o teste.
func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	prev := log.Writer()
	log.SetOutput(out)
	t.Cleanup(func() { log.SetOutput(prev) })
	return out
}

// vinculoDeOutroVendedor: cliente 2 ativo na carteira do vendedor 5.
func vinculoDeOutroVendedor() *sqlmock.Rows {
	return sqlmock.NewRows(colsCarteira).AddRow(int64(9), int64(2), int64(5), agoraFixo, nil, agoraFixo, agoraFixo)
}

func assertErroInterno(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var de *kernel.Erro
	assert.False(t, errors.As(err, &de), "erro interno (500), não um sentinela de domínio")
}

func TestBUG14_VincularCliente_FalhaDesfazTudo(t *testing.T) {
	ctx := context.Background()
	fim := agoraFixo

	t.Run("falha no INSERT depois do Encerrar → rollback, 500", func(t *testing.T) {
		out := capturarLog(t)
		pool, mock := novoMock(t)
		esperarVincularAte(mock)
		mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(vinculoDeOutroVendedor())
		mock.ExpectExec(reEncerrar).WithArgs(agoraFixo, int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(reDoDia).WillReturnRows(sqlmock.NewRows(colsCarteira))
		mock.ExpectExec(reInserir).WillReturnError(errInfra)
		mock.ExpectRollback()

		r, err := novosCasos(pool, vinculoFake{}).VincularCliente(ctx, principal(t, kernel.RoleAdmin), 1, 2)
		assert.Nil(t, r)
		assert.ErrorIs(t, err, errInfra)
		assertErroInterno(t, err)
		assert.NoError(t, mock.ExpectationsWereMet(), "rollback e nenhum commit")
		assert.NotContains(t, out.String(), "cliente vinculado")
	})
	t.Run("falha no Reabrir → rollback, 500", func(t *testing.T) {
		pool, mock := novoMock(t)
		esperarVincularAte(mock)
		mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(sqlmock.NewRows(colsCarteira))
		mock.ExpectQuery(reDoDia).WillReturnRows(sqlmock.NewRows(colsCarteira).AddRow(int64(4), int64(2), int64(1), agoraFixo, &fim, agoraFixo, agoraFixo))
		mock.ExpectExec(reReabrir).WithArgs(int64(4)).WillReturnError(errInfra)
		mock.ExpectRollback()

		_, err := novosCasos(pool, vinculoFake{}).VincularCliente(ctx, principal(t, kernel.RoleAdmin), 1, 2)
		assertErroInterno(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("duplicado no INSERT → 409, rollback", func(t *testing.T) {
		pool, mock := novoMock(t)
		esperarVincularAte(mock)
		mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(vinculoDeOutroVendedor())
		mock.ExpectExec(reEncerrar).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(reDoDia).WillReturnRows(sqlmock.NewRows(colsCarteira))
		mock.ExpectExec(reInserir).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
		mock.ExpectRollback()

		_, err := novosCasos(pool, vinculoFake{}).VincularCliente(ctx, principal(t, kernel.RoleAdmin), 1, 2)
		assert.Same(t, comercial.ErrVinculoDuplicado, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("cliente some antes do lock → 404, rollback", func(t *testing.T) {
		pool, mock := novoMock(t)
		mock.ExpectQuery(reExisteVendedor).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		mock.ExpectQuery(reClientePorID).WithArgs(int64(2)).
			WillReturnRows(sqlmock.NewRows(colsCliente).AddRow(int64(2), "c", "r", "s", "ci", "PR", "", agoraFixo, true, agoraFixo, agoraFixo))
		mock.ExpectBegin()
		mock.ExpectQuery(reTravarCliente).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"cliente_id_origem"}))
		mock.ExpectRollback()

		_, err := novosCasos(pool, vinculoFake{}).VincularCliente(ctx, principal(t, kernel.RoleAdmin), 1, 2)
		assert.Same(t, comercial.ErrClienteNaoEncontrado, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("falha no commit → 500", func(t *testing.T) {
		out := capturarLog(t)
		pool, mock := novoMock(t)
		esperarVincularAte(mock)
		mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(sqlmock.NewRows(colsCarteira))
		mock.ExpectQuery(reDoDia).WillReturnRows(sqlmock.NewRows(colsCarteira))
		mock.ExpectExec(reInserir).WillReturnResult(sqlmock.NewResult(30, 1))
		mock.ExpectCommit().WillReturnError(errInfra)

		_, err := novosCasos(pool, vinculoFake{}).VincularCliente(ctx, principal(t, kernel.RoleAdmin), 1, 2)
		assertErroInterno(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
		assert.NotContains(t, out.String(), "cliente vinculado")
	})
}

// Ordem exata da transferência: Begin → lock → ativo → UPDATE → do dia →
// INSERT → Commit; o log da vinculação sai depois do commit.
func TestBUG14_VincularCliente_TransferenciaNaTx(t *testing.T) {
	out := capturarLog(t)
	pool, mock := novoMock(t)
	esperarVincularAte(mock)
	mock.ExpectQuery(reAtivoDoCliente).WithArgs(int64(2)).WillReturnRows(vinculoDeOutroVendedor())
	mock.ExpectExec(reEncerrar).WithArgs(agoraFixo, int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(reDoDia).WithArgs(int64(2), int64(1), "2026-05-06").WillReturnRows(sqlmock.NewRows(colsCarteira))
	mock.ExpectExec(reInserir).WithArgs(int64(2), int64(1), agoraFixo, nil).WillReturnResult(sqlmock.NewResult(30, 1))
	mock.ExpectCommit()

	r, err := novosCasos(pool, vinculoFake{}).VincularCliente(context.Background(), principal(t, kernel.RoleAdmin), 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(30), r.CarteiraID)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, out.String(), "[vendedores] cliente vinculado: vendedor_id=1 cliente_id=2")
}

// Mesmo vendedor: nada a escrever, commit da tx só com o lock e a leitura.
func TestBUG14_VincularCliente_MesmoVendedorSemEscrita(t *testing.T) {
	pool, mock := novoMock(t)
	esperarVincularAte(mock)
	mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(sqlmock.NewRows(colsCarteira).AddRow(int64(9), int64(2), int64(1), agoraFixo, nil, agoraFixo, agoraFixo))
	mock.ExpectCommit()

	r, err := novosCasos(pool, vinculoFake{}).VincularCliente(context.Background(), principal(t, kernel.RoleAdmin), 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(9), r.CarteiraID)
	assert.NoError(t, mock.ExpectationsWereMet(), "nenhum UPDATE/INSERT")
}

// Prova de que a fn usa só a tx: com uma única conexão no pool, qualquer
// consulta pelo pool dentro da fn esperaria a conexão presa na tx e o
// contexto curto estouraria.
func TestBUG14_VincularCliente_TudoNaMesmaConexao(t *testing.T) {
	pool, mock := novoMock(t)
	pool.SetMaxOpenConns(1)
	esperarVincularAte(mock)
	mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(vinculoDeOutroVendedor())
	mock.ExpectExec(reEncerrar).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(reDoDia).WillReturnRows(sqlmock.NewRows(colsCarteira))
	mock.ExpectExec(reInserir).WillReturnResult(sqlmock.NewResult(30, 1))
	mock.ExpectCommit()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := novosCasos(pool, vinculoFake{}).VincularCliente(ctx, principal(t, kernel.RoleAdmin), 1, 2)
	require.NoError(t, err, "uso do pool dentro da tx travaria até o deadline")
	assert.NoError(t, mock.ExpectationsWereMet())
}
