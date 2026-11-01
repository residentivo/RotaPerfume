package handlers_test

// SEC-35 (Lote 28): criação e exclusão de pagamento rodam na tx do pedido
// (TxPedido), com a ordem de lock pedido → pagamento. Estes helpers montam
// a sequência de SQL esperada nos testes HTTP.

import (
	"github.com/DATA-DOG/go-sqlmock"
)

const (
	// lockPagamentoRegexH é o lock do pagamento na exclusão.
	lockPagamentoRegexH = `SELECT pedido_id, status_pagamento FROM pagamentos WHERE pagamento_id = \? FOR UPDATE`
	// deletePagamentoCondRegexH é o DELETE condicionado a não quitado.
	deletePagamentoCondRegexH = `DELETE FROM pagamentos WHERE pagamento_id = \? AND status_pagamento NOT IN \(\?, \?\)`
)

// expectLockPedidoH abre a tx e trava o pedido, que pertence a vendedorID.
func expectLockPedidoH(m sqlmock.Sqlmock, pedidoID, vendedorID int64) {
	m.ExpectBegin()
	m.ExpectQuery(selectStatusForUpdateRegexH).WithArgs(pedidoID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}).AddRow("Pendente", vendedorID))
}

// expectLockPedidoErroH abre a tx e o lock do pedido falha com err
// (sql.ErrNoRows = pedido inexistente).
func expectLockPedidoErroH(m sqlmock.Sqlmock, pedidoID int64, err error) {
	m.ExpectBegin()
	m.ExpectQuery(selectStatusForUpdateRegexH).WithArgs(pedidoID).WillReturnError(err)
}

// expectLockPagamentoH trava o pagamento (pedido e status lidos sob lock).
func expectLockPagamentoH(m sqlmock.Sqlmock, id, pedidoID int64, status string) {
	m.ExpectQuery(lockPagamentoRegexH).WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"pedido_id", "status_pagamento"}).AddRow(pedidoID, status))
}

// expectDeletePagamentoH é o DELETE condicionado, com linhas afetadas.
func expectDeletePagamentoH(m sqlmock.Sqlmock, id, linhas int64) {
	m.ExpectExec(deletePagamentoCondRegexH).WithArgs(id, "Pago", "Pago com atraso").
		WillReturnResult(sqlmock.NewResult(0, linhas))
}
