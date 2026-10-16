package repositories_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/vendas"
)

var pedidoColumns = []string{
	"pedido_id_origem", "cliente_id", "vendedor_id", "data_pedido", "canal", "status",
	"valor_total", "created_at", "updated_at", "cliente_nome", "vendedor_nome",
}

func pedidoRow(idOrigem, clienteID, vendedorID int64, dataPedido time.Time, canal, status string, valorTotal float64, created, updated time.Time, clienteNome, vendedorNome string) []driver.Value {
	return []driver.Value{idOrigem, clienteID, vendedorID, dataPedido, canal, status, valorTotal, created, updated, clienteNome, vendedorNome}
}

var itemPedidoColumns = []string{
	"item_id_origem", "pedido_id", "produto_id", "quantidade", "preco_praticado",
	"desconto_pct", "valor_bruto", "created_at", "updated_at", "produto_sku", "produto_descricao",
}

func itemPedidoRow(idOrigem, pedidoID, produtoID int64, quantidade int, preco, desconto, valorBruto float64, created, updated time.Time, sku, descricao string) []driver.Value {
	return []driver.Value{idOrigem, pedidoID, produtoID, quantidade, preco, desconto, valorBruto, created, updated, sku, descricao}
}

const pedidoFromRegexp = `FROM pedidos p JOIN clientes c ON c\.cliente_id_origem = p\.cliente_id JOIN vendedores v ON v\.id = p\.vendedor_id`

func TestPedidoList_SemFiltro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pedidoFromRegexp).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(pedidoColumns).
		AddRow(pedidoRow(202, 1, 1, now, "online", "aprovado", 300.0, now, now, "Cliente A", "Vendedor A")...).
		AddRow(pedidoRow(201, 1, 1, now, "online", "aprovado", 100.0, now, now, "Cliente A", "Vendedor A")...)

	mock.ExpectQuery(`SELECT .+ `+pedidoFromRegexp+` ORDER BY p\.pedido_id_origem DESC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnRows(rows)

	ctx := context.Background()
	pedidos, total, err := itensETotal(vendasmysql.NovoPedidos(db).Listar(ctx, escopoAdmin(), vendas.FiltroPedido{Ordenacao: vendas.OrdenacaoPedidoPadrao()}, kernel.NovaPagina(1, 10)))

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, pedidos, 2)
	assert.Equal(t, "Cliente A", pedidos[0].ClienteNome)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoList_ComFiltros(t *testing.T) {
	testes := []struct {
		nome        string
		filtro      vendas.FiltroPedido
		whereRegexp string
		args        []driver.Value
	}{
		{
			nome:        "filtro por status",
			filtro:      vendas.FiltroPedido{Status: "aprovado", Ordenacao: vendas.OrdenacaoPedidoPadrao()},
			whereRegexp: `WHERE p\.status = \?`,
			args:        []driver.Value{"aprovado"},
		},
		{
			nome:        "filtro por canal",
			filtro:      vendas.FiltroPedido{Canal: "online", Ordenacao: vendas.OrdenacaoPedidoPadrao()},
			whereRegexp: `WHERE p\.canal = \?`,
			args:        []driver.Value{"online"},
		},
		{
			nome:        "filtro por cliente",
			filtro:      vendas.FiltroPedido{ClienteID: 5, Ordenacao: vendas.OrdenacaoPedidoPadrao()},
			whereRegexp: `WHERE p\.cliente_id = \?`,
			args:        []driver.Value{int64(5)},
		},
		{
			nome:        "filtro por vendedor",
			filtro:      vendas.FiltroPedido{VendedorID: 3, Ordenacao: vendas.OrdenacaoPedidoPadrao()},
			whereRegexp: `WHERE p\.vendedor_id = \?`,
			args:        []driver.Value{int64(3)},
		},
		{
			nome:        "filtro por data inicio e fim",
			filtro:      vendas.FiltroPedido{DataInicio: "2024-01-01", DataFim: "2024-01-31", Ordenacao: vendas.OrdenacaoPedidoPadrao()},
			whereRegexp: `WHERE p\.data_pedido >= \? AND p\.data_pedido <= \?`,
			args:        []driver.Value{"2024-01-01", "2024-01-31"},
		},
		{
			nome:        "filtro por busca textual",
			filtro:      vendas.FiltroPedido{Busca: "Empresa", Ordenacao: vendas.OrdenacaoPedidoPadrao()},
			whereRegexp: `WHERE c\.razao_social LIKE \?`,
			args:        []driver.Value{"%Empresa%"},
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pedidoFromRegexp + ` ` + tt.whereRegexp).
				WithArgs(tt.args...).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

			allArgs := append(append([]driver.Value{}, tt.args...), int64(10), int64(0))
			mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp + ` ` + tt.whereRegexp + ` ORDER BY p\.pedido_id_origem DESC LIMIT \? OFFSET \?`).
				WithArgs(allArgs...).
				WillReturnRows(sqlmock.NewRows(pedidoColumns))

			ctx := context.Background()
			_, _, err := itensETotal(vendasmysql.NovoPedidos(db).Listar(ctx, escopoAdmin(), tt.filtro, kernel.NovaPagina(1, 10)))

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPedidoList_OrderBy(t *testing.T) {
	testes := []struct {
		nome        string
		orderBy     string
		orderDir    string
		orderRegexp string
	}{
		{
			nome:        "order_by válido asc",
			orderBy:     "valor_total",
			orderDir:    "asc",
			orderRegexp: `ORDER BY p\.valor_total ASC`,
		},
		{
			nome:        "order_by com alias de outra tabela (cliente_nome)",
			orderBy:     "cliente_nome",
			orderDir:    "asc",
			orderRegexp: `ORDER BY c\.razao_social ASC`,
		},
		{
			nome:        "order_by com alias de outra tabela (vendedor_nome)",
			orderBy:     "vendedor_nome",
			orderDir:    "desc",
			orderRegexp: `ORDER BY v\.nome DESC`,
		},
		{
			nome:        "order_by fora da whitelist cai no default",
			orderBy:     "1; DROP TABLE pedidos;--",
			orderDir:    "desc",
			orderRegexp: `ORDER BY p\.pedido_id_origem DESC`,
		},
		{
			nome:        "order_dir inválido cai no default (desc)",
			orderBy:     "status",
			orderDir:    "sideways",
			orderRegexp: `ORDER BY p\.status DESC`,
		},
		{
			nome:        "order_by e order_dir vazios caem no default",
			orderBy:     "",
			orderDir:    "",
			orderRegexp: `ORDER BY p\.pedido_id_origem DESC`,
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pedidoFromRegexp).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery(`SELECT .+ `+pedidoFromRegexp+` `+tt.orderRegexp+` LIMIT \? OFFSET \?`).
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows(pedidoColumns))

			ctx := context.Background()
			_, _, err := itensETotal(vendasmysql.NovoPedidos(db).Listar(ctx, escopoAdmin(), vendas.FiltroPedido{Ordenacao: kernel.ParseOrdenacao(tt.orderBy, tt.orderDir, vendas.CamposOrdenacaoPedido(), vendas.OrdenacaoPedidoPadrao())}, kernel.NovaPagina(1, 10)))

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPedidoList_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pedidoFromRegexp).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, _, err := itensETotal(vendasmysql.NovoPedidos(db).Listar(ctx, escopoAdmin(), vendas.FiltroPedido{Ordenacao: vendas.OrdenacaoPedidoPadrao()}, kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoList_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pedidoFromRegexp).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ `+pedidoFromRegexp).
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, _, err := itensETotal(vendasmysql.NovoPedidos(db).Listar(ctx, escopoAdmin(), vendas.FiltroPedido{Ordenacao: vendas.OrdenacaoPedidoPadrao()}, kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pedidoFromRegexp).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	now := time.Now()
	mock.ExpectQuery(`SELECT .+ `+pedidoFromRegexp).
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(pedidoColumns).
			AddRow(pedidoRow(201, 1, 1, now, "online", "aprovado", 100.0, now, now, "Cliente A", "Vendedor A")...).
			RowError(0, sql.ErrConnDone))

	ctx := context.Background()
	_, _, err := itensETotal(vendasmysql.NovoPedidos(db).Listar(ctx, escopoAdmin(), vendas.FiltroPedido{Ordenacao: vendas.OrdenacaoPedidoPadrao()}, kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(pedidoColumns).
		AddRow(pedidoRow(201, 1, 1, now, "online", "aprovado", 100.0, now, now, "Cliente A", "Vendedor A")...)

	mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	ctx := context.Background()
	p, err := vendasmysql.NovoPedidos(db).PorID(ctx, 1)

	require.NoError(t, err)
	assert.Equal(t, int64(201), p.PedidoIDOrigem)
	assert.Equal(t, "Cliente A", p.ClienteNome)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	ctx := context.Background()
	p, err := vendasmysql.NovoPedidos(db).PorID(ctx, 999)

	assert.Nil(t, p)
	assert.ErrorIs(t, err, vendas.ErrPedidoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	p, err := vendasmysql.NovoPedidos(db).PorID(ctx, 1)

	assert.Nil(t, p)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, vendas.ErrPedidoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoListItensByPedidoID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(itemPedidoColumns).
		AddRow(itemPedidoRow(301, 1, 1, 2, 50.0, 0, 100.0, now, now, "SKU1", "Perfume A")...)

	mock.ExpectQuery(`SELECT .+ FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	ctx := context.Background()
	itens, err := vendasmysql.NovoPedidos(db).Itens(ctx, 1)

	require.NoError(t, err)
	require.Len(t, itens, 1)
	assert.Equal(t, "SKU1", itens[0].ProdutoSKU)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoListItensByPedidoID_Vazio(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(itemPedidoColumns))

	ctx := context.Background()
	itens, err := vendasmysql.NovoPedidos(db).Itens(ctx, 1)

	require.NoError(t, err)
	assert.Len(t, itens, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoListItensByPedidoID_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, err := vendasmysql.NovoPedidos(db).Itens(ctx, 1)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoListItensByPedidoID_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(itemPedidoColumns).
		AddRow(itemPedidoRow(301, 1, 1, 2, 50.0, 0, 100.0, now, now, "SKU1", "Perfume A")...).
		RowError(0, sql.ErrConnDone)

	mock.ExpectQuery(`SELECT .+ FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	ctx := context.Background()
	_, err := vendasmysql.NovoPedidos(db).Itens(ctx, 1)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoExistsByID(t *testing.T) {
	testes := []struct {
		nome     string
		id       int64
		mockErr  error
		expected bool
	}{
		{nome: "pedido existe", id: 1, expected: true},
		{nome: "pedido nao existe", id: 999, mockErr: sql.ErrNoRows, expected: false},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			q := mock.ExpectQuery(`SELECT 1 FROM pedidos WHERE pedido_id_origem = \? LIMIT 1`).WithArgs(tt.id)
			if tt.mockErr != nil {
				q.WillReturnError(tt.mockErr)
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			}

			ctx := context.Background()
			exists, err := vendasmysql.NovoPedidos(db).Existe(ctx, tt.id)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, exists)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPedidoExistsByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT 1 FROM pedidos WHERE pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	exists, err := vendasmysql.NovoPedidos(db).Existe(ctx, 1)

	assert.False(t, exists)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestPedidoCreateComItens_Success valida que pedido_id_origem e
// item_id_origem são obtidos via LastInsertId() do sql.Result (não mais via
// SELECT MAX(...) + 1 manual, comportamento removido do repositório).
func TestPedidoCreateComItens_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	p := &vendas.Pedido{
		ClienteID:  1,
		VendedorID: 1,
		DataPedido: now,
		Canal:      "online",
		Status:     "aprovado",
		ValorTotal: 150.0,
	}
	itens := []vendas.ItemPedido{
		{ProdutoID: 1, Quantidade: 3, PrecoPraticado: 50.0, DescontoPct: 0, ValorBruto: 150.0},
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO pedidos").
		WithArgs(p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal).
		WillReturnResult(sqlmock.NewResult(10, 1))
	mock.ExpectExec("INSERT INTO itens_pedido").
		WithArgs(int64(10), int64(1), 3, 50.0, 0.0, 150.0).
		WillReturnResult(sqlmock.NewResult(20, 1))
	mock.ExpectCommit()

	ctx := context.Background()
	err := inserirPedido(ctx, db, p, itens)

	require.NoError(t, err)
	assert.Equal(t, int64(10), p.PedidoIDOrigem)
	assert.Equal(t, int64(10), itens[0].PedidoID)
	assert.Equal(t, int64(20), itens[0].ItemIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoCreateComItens_SemItens(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	p := &vendas.Pedido{ClienteID: 1, VendedorID: 1, DataPedido: now, Canal: "online", Status: "aprovado", ValorTotal: 0}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO pedidos").
		WithArgs(p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal).
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectCommit()

	ctx := context.Background()
	err := inserirPedido(ctx, db, p, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(11), p.PedidoIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoCreateComItens_ErroInsertPedido(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &vendas.Pedido{}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO pedidos").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	ctx := context.Background()
	err := inserirPedido(ctx, db, p, nil)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoCreateComItens_ErroInsertItem(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &vendas.Pedido{}
	itens := []vendas.ItemPedido{{ProdutoID: 1, Quantidade: 1, PrecoPraticado: 10.0}}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO pedidos").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO itens_pedido").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	ctx := context.Background()
	err := inserirPedido(ctx, db, p, itens)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// itensAtuaisColumns/Regexp seguem o SELECT usado por listItensByPedidoIDTx
// (chamado dentro da tx de UpdateComItens para comparar itens atuais x payload).
var itensAtuaisColumns = []string{"produto_id", "quantidade", "preco_praticado", "desconto_pct"}

const selectStatusForUpdateRegexp = `SELECT status FROM pedidos WHERE pedido_id_origem = \? FOR UPDATE`
const selectItensAtuaisTxRegexp = `SELECT produto_id, quantidade, preco_praticado, desconto_pct FROM itens_pedido WHERE pedido_id = \? ORDER BY item_id_origem ASC`

func TestPedidoUpdateComItens_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	p := &vendas.Pedido{ClienteID: 1, VendedorID: 1, DataPedido: now, Canal: "online", Status: "aprovado", ValorTotal: 200.0}
	itens := []vendas.ItemPedido{
		{ProdutoID: 2, Quantidade: 4, PrecoPraticado: 50.0, DescontoPct: 0, ValorBruto: 200.0},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("aprovado"))
	mock.ExpectQuery(selectItensAtuaisTxRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(itensAtuaisColumns))
	mock.ExpectExec("UPDATE pedidos").
		WithArgs(p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO itens_pedido").
		WithArgs(int64(1), int64(2), 4, 50.0, 0.0, 200.0).
		WillReturnResult(sqlmock.NewResult(30, 1))
	mock.ExpectCommit()

	ctx := context.Background()
	err := atualizarPedido(ctx, db, 1, p, itens)

	require.NoError(t, err)
	assert.Equal(t, int64(30), itens[0].ItemIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &vendas.Pedido{}

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexp).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	ctx := context.Background()
	err := atualizarPedido(ctx, db, 999, p, nil)

	assert.ErrorIs(t, err, vendas.ErrPedidoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_ErroUpdate(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &vendas.Pedido{}

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("aprovado"))
	mock.ExpectQuery(selectItensAtuaisTxRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(itensAtuaisColumns))
	mock.ExpectExec("UPDATE pedidos").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	ctx := context.Background()
	err := atualizarPedido(ctx, db, 1, p, nil)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_ErroDelete(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &vendas.Pedido{}

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("aprovado"))
	mock.ExpectQuery(selectItensAtuaisTxRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(itensAtuaisColumns))
	mock.ExpectExec("UPDATE pedidos").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	ctx := context.Background()
	err := atualizarPedido(ctx, db, 1, p, nil)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_TransicaoParaFaturado_BaixaEstoque(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	p := &vendas.Pedido{ClienteID: 1, VendedorID: 1, DataPedido: now, Canal: "online", Status: "Faturado", ValorTotal: 200.0}
	itens := []vendas.ItemPedido{
		{ProdutoID: 2, Quantidade: 4, PrecoPraticado: 50.0, DescontoPct: 0, ValorBruto: 200.0},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("aprovado"))
	mock.ExpectQuery(selectItensAtuaisTxRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(itensAtuaisColumns))
	mock.ExpectExec("UPDATE pedidos").
		WithArgs(p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO itens_pedido").
		WithArgs(int64(1), int64(2), 4, 50.0, 0.0, 200.0).
		WillReturnResult(sqlmock.NewResult(30, 1))
	// baixa de estoque: resolve sku pelo produto_id, depois ajusta saldo.
	mock.ExpectQuery(`SELECT sku FROM produtos WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"sku"}).AddRow("SKU2"))
	mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO estoque").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	ctx := context.Background()
	err := atualizarPedido(ctx, db, 1, p, itens)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_ReenvioFaturadoIdempotente_NaoBaixaEstoqueDeNovo(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	itens := []vendas.ItemPedido{
		{ProdutoID: 2, Quantidade: 4, PrecoPraticado: 50.0, DescontoPct: 0, ValorBruto: 200.0},
	}
	// payload igual aos itens já persistidos (mesmo produto/qtd/preco/desconto).
	p := &vendas.Pedido{ClienteID: 1, VendedorID: 1, DataPedido: now, Canal: "online", Status: "Faturado", ValorTotal: 200.0}

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Faturado"))
	mock.ExpectQuery(selectItensAtuaisTxRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(itensAtuaisColumns).
			AddRow(int64(2), 4, 50.0, 0.0))
	mock.ExpectExec("UPDATE pedidos").
		WithArgs(p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// pedido já estava Faturado: não reescreve itens_pedido (sem DELETE/INSERT)
	// nem dispara baixa de estoque novamente (statusAtual já era Faturado).
	mock.ExpectCommit()

	ctx := context.Background()
	err := atualizarPedido(ctx, db, 1, p, itens)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_FaturadoTentaAlterarItens_Erro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	// payload com item DIFERENTE do persistido (quantidade mudou).
	itens := []vendas.ItemPedido{
		{ProdutoID: 2, Quantidade: 99, PrecoPraticado: 50.0, DescontoPct: 0, ValorBruto: 200.0},
	}
	p := &vendas.Pedido{ClienteID: 1, VendedorID: 1, DataPedido: now, Canal: "online", Status: "Faturado", ValorTotal: 200.0}

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Faturado"))
	mock.ExpectQuery(selectItensAtuaisTxRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(itensAtuaisColumns).
			AddRow(int64(2), 4, 50.0, 0.0))
	mock.ExpectRollback()

	ctx := context.Background()
	err := atualizarPedido(ctx, db, 1, p, itens)

	assert.ErrorIs(t, err, vendas.ErrPedidoJaFaturadoNaoPodeAlterarItens)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_ErroBaixaEstoque_RollbackTudo(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	p := &vendas.Pedido{ClienteID: 1, VendedorID: 1, DataPedido: now, Canal: "online", Status: "Faturado", ValorTotal: 200.0}
	itens := []vendas.ItemPedido{
		{ProdutoID: 2, Quantidade: 4, PrecoPraticado: 50.0, DescontoPct: 0, ValorBruto: 200.0},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("aprovado"))
	mock.ExpectQuery(selectItensAtuaisTxRegexp).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(itensAtuaisColumns))
	mock.ExpectExec("UPDATE pedidos").
		WithArgs(p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO itens_pedido").
		WithArgs(int64(1), int64(2), 4, 50.0, 0.0, 200.0).
		WillReturnResult(sqlmock.NewResult(30, 1))
	mock.ExpectQuery(`SELECT sku FROM produtos WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	ctx := context.Background()
	err := atualizarPedido(ctx, db, 1, p, itens)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
