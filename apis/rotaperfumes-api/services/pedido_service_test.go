package services_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

func pedidoTestCfg(verbose bool) *config.Config {
	return &config.Config{Verbose: verbose}
}

func newPedidoTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

// ---------------------------------------------------------------------------
// Regexes/colunas que espelham as constantes do repositório.
// ---------------------------------------------------------------------------

const pedidoColunasRegex = `p\.id, p\.pedido_id_origem, p\.cliente_id, p\.vendedor_id, p\.data_pedido, p\.canal, p\.status, p\.valor_total, p\.created_at, p\.updated_at, c\.razao_social, v\.nome`
const pedidoFromRegex = ` FROM pedidos p JOIN clientes c ON c\.id = p\.cliente_id JOIN vendedores v ON v\.id = p\.vendedor_id`
const itemPedidoColunasRegex = `i\.id, i\.item_id_origem, i\.pedido_id, i\.produto_id, i\.quantidade, i\.preco_praticado, i\.desconto_pct, i\.valor_bruto, i\.created_at, i\.updated_at, pr\.sku, pr\.descricao`
const itemPedidoFromRegex = ` FROM itens_pedido i JOIN produtos pr ON pr\.id = i\.produto_id`

func pedidoColunasHeader() []string {
	return []string{
		"id", "pedido_id_origem", "cliente_id", "vendedor_id", "data_pedido",
		"canal", "status", "valor_total", "created_at", "updated_at", "cliente_nome", "vendedor_nome",
	}
}

func pedidoRows(id int64, valorTotal float64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(pedidoColunasHeader()).
		AddRow(id, int64(100), int64(1), int64(2), now, "App", "Faturado", valorTotal, now, now, "Cliente Teste", "Vendedor Teste")
}

func emptyPedidoRows() *sqlmock.Rows {
	return sqlmock.NewRows(pedidoColunasHeader())
}

func itemPedidoColunasHeader() []string {
	return []string{
		"id", "item_id_origem", "pedido_id", "produto_id", "quantidade",
		"preco_praticado", "desconto_pct", "valor_bruto", "created_at", "updated_at", "produto_sku", "produto_descricao",
	}
}

func itemPedidoRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(itemPedidoColunasHeader()).
		AddRow(int64(1), int64(1), int64(1), int64(10), 2, 100.0, 10.0, 180.0, now, now, "SKU-010", "Produto A").
		AddRow(int64(2), int64(2), int64(1), int64(11), 1, 50.0, 0.0, 50.0, now, now, "SKU-011", "Produto B")
}

// validPedidoInput retorna um input válido com dois itens (usado para
// verificar o cálculo de valor_bruto por item e valor_total do pedido:
// item 1: 2 * 100.0 * (1 - 10/100) = 180.0
// item 2: 1 * 50.0  * (1 - 0/100)  = 50.0
// valor_total = 230.0
func validPedidoInput() services.PedidoInput {
	return services.PedidoInput{
		ClienteID:  1,
		VendedorID: 2,
		DataPedido: "2024-01-15",
		Canal:      "App",
		Status:     "Faturado",
		Itens: []services.ItemPedidoInput{
			{ProdutoID: 10, Quantidade: 2, PrecoPraticado: 100.0, DescontoPct: 10},
			{ProdutoID: 11, Quantidade: 1, PrecoPraticado: 50.0, DescontoPct: 0},
		},
	}
}

// ---------------------------------------------------------------------------
// ListPedidos
// ---------------------------------------------------------------------------

func TestPedidoService_ListPedidos(t *testing.T) {
	testCases := []struct {
		nome    string
		filtro  services.PedidoFiltro
		page    int
		limit   int
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
		wantTot int
	}{
		{
			nome:   "sem filtros - sucesso",
			filtro: services.PedidoFiltro{},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\)` + pedidoFromRegex).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT ` + pedidoColunasRegex + pedidoFromRegex + ` ORDER BY p\.id DESC LIMIT \? OFFSET \?`).
					WithArgs(20, 0).
					WillReturnRows(pedidoRows(1, 230.0))
			},
			wantLen: 1,
			wantTot: 1,
		},
		{
			nome: "com todos os filtros - repassa args ao repo",
			filtro: services.PedidoFiltro{
				Status: "Faturado", Canal: "App", ClienteID: 1, VendedorID: 2,
				DataInicio: "2024-01-01", DataFim: "2024-01-31", Q: "Teste",
			},
			page:  2,
			limit: 10,
			mock: func(mock sqlmock.Sqlmock) {
				whereRegex := ` WHERE p\.status = \? AND p\.canal = \? AND p\.cliente_id = \? AND p\.vendedor_id = \? AND p\.data_pedido >= \? AND p\.data_pedido <= \? AND c\.razao_social LIKE \?`
				mock.ExpectQuery(`SELECT COUNT\(\*\)` + pedidoFromRegex + whereRegex).
					WithArgs("Faturado", "App", int64(1), int64(2), "2024-01-01", "2024-01-31", "%Teste%").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
				mock.ExpectQuery(`SELECT ` + pedidoColunasRegex + pedidoFromRegex + whereRegex + ` ORDER BY p\.id DESC LIMIT \? OFFSET \?`).
					WithArgs("Faturado", "App", int64(1), int64(2), "2024-01-01", "2024-01-31", "%Teste%", 10, 10).
					WillReturnRows(pedidoRows(1, 230.0))
			},
			wantLen: 1,
			wantTot: 3,
		},
		{
			nome:   "erro no count do repo é propagado",
			filtro: services.PedidoFiltro{},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\)` + pedidoFromRegex).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newPedidoTestDB(t)
			tc.mock(mock)

			svc := services.NewPedidoService(db, pedidoTestCfg(true))
			pedidos, total, err := svc.ListPedidos(context.Background(), db, tc.page, tc.limit, tc.filtro)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, pedidos, tc.wantLen)
				assert.Equal(t, tc.wantTot, total)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetPedidoDetalhe
// ---------------------------------------------------------------------------

func TestPedidoService_GetPedidoDetalhe(t *testing.T) {
	t.Run("encontrado - com itens", func(t *testing.T) {
		db, mock := newPedidoTestDB(t)
		mock.ExpectQuery(`SELECT ` + pedidoColunasRegex + pedidoFromRegex + ` WHERE p\.id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(pedidoRows(1, 230.0))
		mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegex + itemPedidoFromRegex + ` WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
			WithArgs(int64(1)).
			WillReturnRows(itemPedidoRows())

		svc := services.NewPedidoService(db, pedidoTestCfg(false))
		p, err := svc.GetPedidoDetalhe(context.Background(), db, 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), p.ID)
		assert.Equal(t, 230.0, p.ValorTotal)
		assert.Len(t, p.Itens, 2)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("não encontrado retorna ErrPedidoNaoEncontrado", func(t *testing.T) {
		db, mock := newPedidoTestDB(t)
		mock.ExpectQuery(`SELECT ` + pedidoColunasRegex + pedidoFromRegex + ` WHERE p\.id = \? LIMIT 1`).
			WithArgs(int64(999)).
			WillReturnRows(emptyPedidoRows())

		svc := services.NewPedidoService(db, pedidoTestCfg(false))
		p, err := svc.GetPedidoDetalhe(context.Background(), db, 999)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, services.ErrPedidoNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro genérico no GetByID é propagado", func(t *testing.T) {
		db, mock := newPedidoTestDB(t)
		mock.ExpectQuery(`SELECT ` + pedidoColunasRegex + pedidoFromRegex + ` WHERE p\.id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewPedidoService(db, pedidoTestCfg(false))
		p, err := svc.GetPedidoDetalhe(context.Background(), db, 1)
		assert.Nil(t, p)
		assert.Error(t, err)
		assert.False(t, err == services.ErrPedidoNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro genérico no ListItensByPedidoID é propagado", func(t *testing.T) {
		db, mock := newPedidoTestDB(t)
		mock.ExpectQuery(`SELECT ` + pedidoColunasRegex + pedidoFromRegex + ` WHERE p\.id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(pedidoRows(1, 230.0))
		mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegex + itemPedidoFromRegex + ` WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
			WithArgs(int64(1)).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewPedidoService(db, pedidoTestCfg(false))
		p, err := svc.GetPedidoDetalhe(context.Background(), db, 1)
		assert.Nil(t, p)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// Validações comuns a CreatePedido/UpdatePedido (via validarPedidoInput)
// ---------------------------------------------------------------------------

func pedidoValidacaoTestCases() []struct {
	nome    string
	input   func() services.PedidoInput
	wantErr error
} {
	return []struct {
		nome    string
		input   func() services.PedidoInput
		wantErr error
	}{
		{
			nome: "cliente_id zero",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.ClienteID = 0
				return in
			},
			wantErr: services.ErrClienteIDObrigatorio,
		},
		{
			nome: "vendedor_id zero",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.VendedorID = 0
				return in
			},
			wantErr: services.ErrVendedorIDObrigatorio,
		},
		{
			nome: "data_pedido vazia",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.DataPedido = ""
				return in
			},
			wantErr: services.ErrDataPedidoInvalida,
		},
		{
			nome: "data_pedido em formato inválido",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.DataPedido = "15/01/2024"
				return in
			},
			wantErr: services.ErrDataPedidoInvalida,
		},
		{
			nome: "canal inválido",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.Canal = "Email"
				return in
			},
			wantErr: services.ErrCanalInvalido,
		},
		{
			nome: "status inválido",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.Status = "Pendente"
				return in
			},
			wantErr: services.ErrStatusInvalido,
		},
		{
			nome: "lista de itens vazia",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.Itens = nil
				return in
			},
			wantErr: services.ErrItensObrigatorios,
		},
		{
			nome: "produto_id ausente em um item",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.Itens[0].ProdutoID = 0
				return in
			},
			wantErr: services.ErrProdutoIDObrigatorio,
		},
		{
			nome: "quantidade inválida em um item",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.Itens[0].Quantidade = 0
				return in
			},
			wantErr: services.ErrQuantidadeInvalida,
		},
		{
			nome: "preco_praticado negativo em um item",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.Itens[0].PrecoPraticado = -0.01
				return in
			},
			wantErr: services.ErrPrecoPraticadoInvalido,
		},
		{
			nome: "desconto_pct negativo em um item",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.Itens[0].DescontoPct = -1
				return in
			},
			wantErr: services.ErrDescontoPctInvalido,
		},
		{
			nome: "desconto_pct acima de 100 em um item",
			input: func() services.PedidoInput {
				in := validPedidoInput()
				in.Itens[0].DescontoPct = 101
				return in
			},
			wantErr: services.ErrDescontoPctInvalido,
		},
	}
}

// ---------------------------------------------------------------------------
// CreatePedido
// ---------------------------------------------------------------------------

func TestPedidoService_CreatePedido_Validacoes(t *testing.T) {
	for _, tc := range pedidoValidacaoTestCases() {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newPedidoTestDB(t)
			// nenhuma query é esperada: validação falha antes de tocar o repo.
			svc := services.NewPedidoService(db, pedidoTestCfg(true))
			p, err := svc.CreatePedido(context.Background(), db, tc.input())
			assert.Nil(t, p)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// expectCreateComItensSuccess registra os mocks para uma criação bem-sucedida
// de pedido (id=1, pedido_id_origem=100) a partir de validPedidoInput(),
// calculando valor_bruto=180.0/50.0 e valor_total=230.0.
func expectCreateComItensSuccess(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(pedido_id_origem\), 0\) \+ 1 FROM pedidos`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(100)))

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO pedidos \(pedido_id_origem, cliente_id, vendedor_id, data_pedido, canal, status, valor_total\)`).
		WithArgs(int64(100), int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(item_id_origem\), 0\) \+ 1 FROM itens_pedido`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(1)))
	mock.ExpectExec(`INSERT INTO itens_pedido \(item_id_origem, pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(1), int64(1), int64(10), 2, 100.0, 10.0, 180.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido \(item_id_origem, pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(2), int64(1), int64(11), 1, 50.0, 0.0, 50.0).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegex + pedidoFromRegex + ` WHERE p\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRows(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegex + itemPedidoFromRegex + ` WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRows())
}

func TestPedidoService_CreatePedido_Sucesso(t *testing.T) {
	db, mock := newPedidoTestDB(t)
	expectCreateComItensSuccess(mock)

	svc := services.NewPedidoService(db, pedidoTestCfg(true))
	p, err := svc.CreatePedido(context.Background(), db, validPedidoInput())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, int64(1), p.ID)
	assert.Equal(t, 230.0, p.ValorTotal)
	assert.Len(t, p.Itens, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoService_CreatePedido_ErroNextPedidoIDOrigem(t *testing.T) {
	db, mock := newPedidoTestDB(t)
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(pedido_id_origem\), 0\) \+ 1 FROM pedidos`).
		WillReturnError(sql.ErrConnDone)

	svc := services.NewPedidoService(db, pedidoTestCfg(false))
	p, err := svc.CreatePedido(context.Background(), db, validPedidoInput())
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoService_CreatePedido_ErroCreateComItens(t *testing.T) {
	db, mock := newPedidoTestDB(t)
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(pedido_id_origem\), 0\) \+ 1 FROM pedidos`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(100)))
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO pedidos \(pedido_id_origem, cliente_id, vendedor_id, data_pedido, canal, status, valor_total\)`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	svc := services.NewPedidoService(db, pedidoTestCfg(false))
	p, err := svc.CreatePedido(context.Background(), db, validPedidoInput())
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UpdatePedido
// ---------------------------------------------------------------------------

func TestPedidoService_UpdatePedido_Validacoes(t *testing.T) {
	for _, tc := range pedidoValidacaoTestCases() {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newPedidoTestDB(t)
			svc := services.NewPedidoService(db, pedidoTestCfg(true))
			p, err := svc.UpdatePedido(context.Background(), db, 1, tc.input())
			assert.Nil(t, p)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPedidoService_UpdatePedido_Sucesso(t *testing.T) {
	db, mock := newPedidoTestDB(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE pedidos\s+SET cliente_id = \?, vendedor_id = \?, data_pedido = \?, canal = \?, status = \?, valor_total = \?\s+WHERE id = \?`).
		WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido WHERE pedido_id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(item_id_origem\), 0\) \+ 1 FROM itens_pedido`).
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(1)))
	mock.ExpectExec(`INSERT INTO itens_pedido \(item_id_origem, pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(1), int64(1), int64(10), 2, 100.0, 10.0, 180.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido \(item_id_origem, pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto\)`).
		WithArgs(int64(2), int64(1), int64(11), 1, 50.0, 0.0, 50.0).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT ` + pedidoColunasRegex + pedidoFromRegex + ` WHERE p\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pedidoRows(1, 230.0))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegex + itemPedidoFromRegex + ` WHERE i\.pedido_id = \? ORDER BY i\.id ASC`).
		WithArgs(int64(1)).
		WillReturnRows(itemPedidoRows())

	svc := services.NewPedidoService(db, pedidoTestCfg(true))
	p, err := svc.UpdatePedido(context.Background(), db, 1, validPedidoInput())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, int64(1), p.ID)
	assert.Equal(t, 230.0, p.ValorTotal)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoService_UpdatePedido_NaoEncontrado(t *testing.T) {
	db, mock := newPedidoTestDB(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE pedidos\s+SET cliente_id = \?, vendedor_id = \?, data_pedido = \?, canal = \?, status = \?, valor_total = \?\s+WHERE id = \?`).
		WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	svc := services.NewPedidoService(db, pedidoTestCfg(false))
	p, err := svc.UpdatePedido(context.Background(), db, 999, validPedidoInput())
	assert.Nil(t, p)
	assert.ErrorIs(t, err, services.ErrPedidoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPedidoService_UpdatePedido_ErroGenericoDoRepo(t *testing.T) {
	db, mock := newPedidoTestDB(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE pedidos\s+SET cliente_id = \?, vendedor_id = \?, data_pedido = \?, canal = \?, status = \?, valor_total = \?\s+WHERE id = \?`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	svc := services.NewPedidoService(db, pedidoTestCfg(false))
	p, err := svc.UpdatePedido(context.Background(), db, 1, validPedidoInput())
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.False(t, err == services.ErrPedidoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}
