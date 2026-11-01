package bootstrap_test

// ARQ-05 (§5.2, C9): o UoW do pedido montado pelo bootstrap precisa rodar
// TODOS os passos no MESMO *sql.Tx: cabeçalho e itens na criação; e, no PUT
// com transição para Faturado, o lock do status, os itens, o UPDATE e a
// baixa de estoque do Catálogo (sku + SELECT … FOR UPDATE + upsert). Com o
// pool limitado a UMA conexão, um passo executado no pool em vez da tx
// fica esperando a conexão presa pela tx e estoura o prazo do contexto
// (padrão de poolDeUmaConexao, arq04_mutantes_test.go). Mata os mutantes
// "adapter montado sobre o pool em vez do q" no bind do TxPedido.

import (
	"time"

	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
)

var colunasPedido = []string{"pedido_id_origem", "cliente_id", "vendedor_id", "data_pedido", "canal", "status",
	"valor_total", "created_at", "updated_at", "cliente_nome", "vendedor_nome"}

func pedidoFaturado() vendasapp.Leitor[vendas.DadosPedido] {
	return func() (vendas.DadosPedido, error) {
		return vendas.DadosPedido{ClienteID: 1, VendedorID: 2, DataPedido: "2024-01-15", Canal: "App", Status: "Faturado",
			Itens: []vendas.DadosItem{{ProdutoID: 10, Quantidade: 3, PrecoPraticado: 10}}}, nil
	}
}

func adminARQ05(t *testing.T) kernel.Principal {
	t.Helper()
	p, err := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	require.NoError(t, err)
	return p
}

func TestARQ05_Bootstrap_AtualizarPedidoComBaixaNaMesmaTx(t *testing.T) {
	db, mock, ctx := poolDeUmaConexao(t)
	agora := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status, vendedor_id FROM pedidos WHERE pedido_id_origem = \? FOR UPDATE`).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}).AddRow("Em separação", int64(2)))
	mock.ExpectQuery(`SELECT produto_id, quantidade, preco_praticado, desconto_pct FROM itens_pedido`).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"produto_id", "quantidade", "preco_praticado", "desconto_pct"}))
	mock.ExpectExec(`UPDATE pedidos`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido`).WithArgs(int64(1), int64(10), 3, 10.0, 0.0, 30.0).WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectQuery(`SELECT sku FROM produtos WHERE id = \? LIMIT 1`).WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"sku"}).AddRow("SKU-10"))
	mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(77)))
	mock.ExpectExec(`INSERT INTO estoque`).WithArgs(sqlmock.AnyArg(), "SKU-10", -3, true).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	mock.ExpectQuery(`FROM pedidos p .+ WHERE p\.pedido_id_origem = \? LIMIT 1`).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(colunasPedido).AddRow(int64(1), int64(1), int64(2), agora, "App", "Faturado", 30.0, agora, agora, "C", "V"))
	mock.ExpectQuery(`FROM itens_pedido i`).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows(nil))

	casos := bootstrap.NovoVendas(db, bootstrap.NovoComercial(db).Escopo())
	p, err := casos.AtualizarPedido(ctx, adminARQ05(t), 1, pedidoFaturado())

	require.NoError(t, err, "lock, itens, UPDATE e baixa de estoque devem usar a conexão da tx (§5.2)")
	require.NotNil(t, p)
	assert.Equal(t, "Faturado", p.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ05_Bootstrap_CriarPedidoCabecalhoEItensNaMesmaTx(t *testing.T) {
	db, mock, ctx := poolDeUmaConexao(t)
	agora := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO pedidos`).WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 30.0).
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido`).WithArgs(int64(9), int64(10), 3, 10.0, 0.0, 30.0).WillReturnResult(sqlmock.NewResult(5, 1))
	// BUG-20: criado como Faturado, baixa o estoque na MESMA tx.
	mock.ExpectQuery(`SELECT sku FROM produtos WHERE id = \? LIMIT 1`).WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"sku"}).AddRow("SKU-10"))
	mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(77)))
	mock.ExpectExec(`INSERT INTO estoque`).WithArgs(sqlmock.AnyArg(), "SKU-10", -3, true).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	mock.ExpectQuery(`FROM pedidos p .+ WHERE p\.pedido_id_origem = \? LIMIT 1`).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows(colunasPedido).AddRow(int64(9), int64(1), int64(2), agora, "App", "Faturado", 30.0, agora, agora, "C", "V"))
	mock.ExpectQuery(`FROM itens_pedido i`).WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows(nil))

	casos := bootstrap.NovoVendas(db, bootstrap.NovoComercial(db).Escopo())
	p, err := casos.CriarPedido(ctx, adminARQ05(t), pedidoFaturado())

	require.NoError(t, err, "cabeçalho, itens e baixa de estoque devem usar a conexão da tx (C9, BUG-20)")
	require.NotNil(t, p)
	assert.Equal(t, int64(9), p.PedidoIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}
