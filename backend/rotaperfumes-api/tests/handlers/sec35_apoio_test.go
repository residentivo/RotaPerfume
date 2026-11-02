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

// reLeituraPedidoH é a leitura sem lock do SEC-38 (escopo restrito): a posse
// é conferida antes do FOR UPDATE, para o restrito não travar pedido alheio.
const reLeituraPedidoH = `^SELECT vendedor_id FROM pedidos WHERE pedido_id_origem = \?$`

// expectLeituraPedidoH é a leitura sem lock (SEC-38), dentro da tx.
func expectLeituraPedidoH(m sqlmock.Sqlmock, pedidoID, vendedorID int64) {
	m.ExpectQuery(reLeituraPedidoH).WithArgs(pedidoID).WillReturnRows(sqlmock.NewRows([]string{"vendedor_id"}).AddRow(vendedorID))
}

// expectLockPedidoRestritoH é o roteiro do SEC-38 no escopo restrito: Begin
// → leitura sem lock → FOR UPDATE, com o pedido do vendedorID.
func expectLockPedidoRestritoH(m sqlmock.Sqlmock, pedidoID, vendedorID int64) {
	m.ExpectBegin()
	expectLeituraPedidoH(m, pedidoID, vendedorID)
	m.ExpectQuery(selectStatusForUpdateRegexH).WithArgs(pedidoID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}).AddRow("Pendente", vendedorID))
}

// expectLeituraPedidoErroH abre a tx e a leitura sem lock (SEC-38) falha
// com err (sql.ErrNoRows = pedido inexistente → 404, sem lock).
func expectLeituraPedidoErroH(m sqlmock.Sqlmock, pedidoID int64, err error) {
	m.ExpectBegin()
	m.ExpectQuery(reLeituraPedidoH).WithArgs(pedidoID).WillReturnError(err)
}

// expectPosseAlheiaH abre a tx e a leitura sem lock acha o pedido de outro
// vendedor: 404 antes do FOR UPDATE (o restrito não trava pedido alheio).
func expectPosseAlheiaH(m sqlmock.Sqlmock, pedidoID, vendedorDoPedido int64) {
	m.ExpectBegin()
	expectLeituraPedidoH(m, pedidoID, vendedorDoPedido)
}
