package repositories_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

var pedidoColumns = []string{
	"id", "pedido_id_origem", "cliente_id", "vendedor_id", "data_pedido", "canal", "status",
	"valor_total", "created_at", "updated_at", "cliente_nome", "vendedor_nome",
}

func pedidoRow(id, idOrigem, clienteID, vendedorID int64, dataPedido time.Time, canal, status string, valorTotal float64, created, updated time.Time, clienteNome, vendedorNome string) []driver.Value {
	return []driver.Value{id, idOrigem, clienteID, vendedorID, dataPedido, canal, status, valorTotal, created, updated, clienteNome, vendedorNome}
}

var itemPedidoColumns = []string{
	"id", "item_id_origem", "pedido_id", "produto_id", "quantidade", "preco_praticado",
	"desconto_pct", "valor_bruto", "created_at", "updated_at", "produto_sku", "produto_descricao",
}

func itemPedidoRow(id, idOrigem, pedidoID, produtoID int64, quantidade int, preco, desconto, valorBruto float64, created, updated time.Time, sku, descricao string) []driver.Value {
	return []driver.Value{id, idOrigem, pedidoID, produtoID, quantidade, preco, desconto, valorBruto, created, updated, sku, descricao}
}

const pedidoFromRegexp = `FROM pedidos p JOIN clientes c ON c.id = p.cliente_id JOIN vendedores v ON v.id = p.vendedor_id`

func TestPedidoList_SemFiltro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pedidoFromRegexp).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(pedidoColumns).
		AddRow(pedidoRow(2, 202, 1, 1, now, "online", "aprovado", 300.0, now, now, "Cliente A", "Vendedor A")...).
		AddRow(pedidoRow(1, 201, 1, 1, now, "online", "aprovado", 100.0, now, now, "Cliente A", "Vendedor A")...)

	mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp + ` ORDER BY p.id DESC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnRows(rows)

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	pedidos, total, err := repo.List(ctx, db, 1, 10, repositories.PedidoFiltro{})

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, pedidos, 2)
	assert.Equal(t, "Cliente A", pedidos[0].ClienteNome)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoList_ComFiltros(t *testing.T) {
	testes := []struct {
		nome        string
		filtro      repositories.PedidoFiltro
		whereRegexp string
		args        []driver.Value
	}{
		{
			nome:        "filtro por status",
			filtro:      repositories.PedidoFiltro{Status: "aprovado"},
			whereRegexp: `WHERE p\.status = \?`,
			args:        []driver.Value{"aprovado"},
		},
		{
			nome:        "filtro por canal",
			filtro:      repositories.PedidoFiltro{Canal: "online"},
			whereRegexp: `WHERE p\.canal = \?`,
			args:        []driver.Value{"online"},
		},
		{
			nome:        "filtro por cliente",
			filtro:      repositories.PedidoFiltro{ClienteID: 5},
			whereRegexp: `WHERE p\.cliente_id = \?`,
			args:        []driver.Value{int64(5)},
		},
		{
			nome:        "filtro por vendedor",
			filtro:      repositories.PedidoFiltro{VendedorID: 3},
			whereRegexp: `WHERE p\.vendedor_id = \?`,
			args:        []driver.Value{int64(3)},
		},
		{
			nome:        "filtro por data inicio e fim",
			filtro:      repositories.PedidoFiltro{DataInicio: "2024-01-01", DataFim: "2024-01-31"},
			whereRegexp: `WHERE p\.data_pedido >= \? AND p\.data_pedido <= \?`,
			args:        []driver.Value{"2024-01-01", "2024-01-31"},
		},
		{
			nome:        "filtro por busca textual",
			filtro:      repositories.PedidoFiltro{Q: "Empresa"},
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
			mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp + ` ` + tt.whereRegexp + ` ORDER BY p\.id DESC LIMIT \? OFFSET \?`).
				WithArgs(allArgs...).
				WillReturnRows(sqlmock.NewRows(pedidoColumns))

			repo := repositories.NewPedidoRepository()
			ctx := context.Background()
			_, _, err := repo.List(ctx, db, 1, 10, tt.filtro)

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

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.PedidoFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoList_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pedidoFromRegexp).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp).
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.PedidoFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pedidoFromRegexp).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	now := time.Now()
	mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp).
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(pedidoColumns).
			AddRow(pedidoRow(1, 201, 1, 1, now, "online", "aprovado", 100.0, now, now, "Cliente A", "Vendedor A")...).
			RowError(0, sql.ErrConnDone))

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.PedidoFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(pedidoColumns).
		AddRow(pedidoRow(1, 201, 1, 1, now, "online", "aprovado", 100.0, now, now, "Cliente A", "Vendedor A")...)

	mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp + ` WHERE p\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	p, err := repo.GetByID(ctx, db, 1)

	require.NoError(t, err)
	assert.Equal(t, int64(1), p.ID)
	assert.Equal(t, "Cliente A", p.ClienteNome)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp + ` WHERE p\.id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	p, err := repo.GetByID(ctx, db, 999)

	assert.Nil(t, p)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ ` + pedidoFromRegexp + ` WHERE p\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	p, err := repo.GetByID(ctx, db, 1)

	assert.Nil(t, p)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoListItensByPedidoID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(itemPedidoColumns).
		AddRow(itemPedidoRow(1, 301, 1, 1, 2, 50.0, 0, 100.0, now, now, "SKU1", "Perfume A")...)

	mock.ExpectQuery(`SELECT .+ FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	itens, err := repo.ListItensByPedidoID(ctx, db, 1)

	require.NoError(t, err)
	require.Len(t, itens, 1)
	assert.Equal(t, "SKU1", itens[0].ProdutoSKU)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoListItensByPedidoID_Vazio(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(itemPedidoColumns))

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	itens, err := repo.ListItensByPedidoID(ctx, db, 1)

	require.NoError(t, err)
	assert.Len(t, itens, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoListItensByPedidoID_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	_, err := repo.ListItensByPedidoID(ctx, db, 1)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoListItensByPedidoID_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(itemPedidoColumns).
		AddRow(itemPedidoRow(1, 301, 1, 1, 2, 50.0, 0, 100.0, now, now, "SKU1", "Perfume A")...).
		RowError(0, sql.ErrConnDone)

	mock.ExpectQuery(`SELECT .+ FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	_, err := repo.ListItensByPedidoID(ctx, db, 1)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoNextPedidoIDOrigem_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COALESCE\(MAX\(pedido_id_origem\), 0\) \+ 1 FROM pedidos`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(202))

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	next, err := repo.NextPedidoIDOrigem(ctx, db)

	require.NoError(t, err)
	assert.Equal(t, int64(202), next)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoNextPedidoIDOrigem_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COALESCE\(MAX\(pedido_id_origem\), 0\) \+ 1 FROM pedidos`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	_, err := repo.NextPedidoIDOrigem(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoCreateComItens_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	p := &models.Pedido{
		PedidoIDOrigem: 202,
		ClienteID:      1,
		VendedorID:     1,
		DataPedido:     now,
		Canal:          "online",
		Status:         "aprovado",
		ValorTotal:     150.0,
	}
	itens := []models.ItemPedido{
		{ProdutoID: 1, Quantidade: 3, PrecoPraticado: 50.0, DescontoPct: 0, ValorBruto: 150.0},
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO pedidos").
		WithArgs(p.PedidoIDOrigem, p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal).
		WillReturnResult(sqlmock.NewResult(10, 1))
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(item_id_origem\), 0\) \+ 1 FROM itens_pedido`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(401))
	mock.ExpectExec("INSERT INTO itens_pedido").
		WithArgs(int64(401), int64(10), int64(1), 3, 50.0, 0.0, 150.0).
		WillReturnResult(sqlmock.NewResult(20, 1))
	mock.ExpectCommit()

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	err := repo.CreateComItens(ctx, db, p, itens)

	require.NoError(t, err)
	assert.Equal(t, int64(10), p.ID)
	assert.Equal(t, int64(20), itens[0].ID)
	assert.Equal(t, int64(401), itens[0].ItemIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoCreateComItens_SemItens(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	p := &models.Pedido{PedidoIDOrigem: 202, ClienteID: 1, VendedorID: 1, DataPedido: now, Canal: "online", Status: "aprovado", ValorTotal: 0}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO pedidos").
		WithArgs(p.PedidoIDOrigem, p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal).
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectCommit()

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	err := repo.CreateComItens(ctx, db, p, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(11), p.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoCreateComItens_ErroInsertPedido(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Pedido{}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO pedidos").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	err := repo.CreateComItens(ctx, db, p, nil)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoCreateComItens_ErroInsertItem(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Pedido{}
	itens := []models.ItemPedido{{ProdutoID: 1, Quantidade: 1, PrecoPraticado: 10.0}}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO pedidos").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(item_id_origem\), 0\) \+ 1 FROM itens_pedido`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(1))
	mock.ExpectExec("INSERT INTO itens_pedido").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	err := repo.CreateComItens(ctx, db, p, itens)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	p := &models.Pedido{ClienteID: 1, VendedorID: 1, DataPedido: now, Canal: "online", Status: "aprovado", ValorTotal: 200.0}
	itens := []models.ItemPedido{
		{ProdutoID: 2, Quantidade: 4, PrecoPraticado: 50.0, DescontoPct: 0, ValorBruto: 200.0},
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE pedidos").
		WithArgs(p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(item_id_origem\), 0\) \+ 1 FROM itens_pedido`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(500))
	mock.ExpectExec("INSERT INTO itens_pedido").
		WithArgs(int64(500), int64(1), int64(2), 4, 50.0, 0.0, 200.0).
		WillReturnResult(sqlmock.NewResult(30, 1))
	mock.ExpectCommit()

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	err := repo.UpdateComItens(ctx, db, 1, p, itens)

	require.NoError(t, err)
	assert.Equal(t, int64(30), itens[0].ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Pedido{}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE pedidos").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	err := repo.UpdateComItens(ctx, db, 999, p, nil)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_ErroUpdate(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Pedido{}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE pedidos").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	err := repo.UpdateComItens(ctx, db, 1, p, nil)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoUpdateComItens_ErroDelete(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &models.Pedido{}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE pedidos").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	repo := repositories.NewPedidoRepository()
	ctx := context.Background()
	err := repo.UpdateComItens(ctx, db, 1, p, nil)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
