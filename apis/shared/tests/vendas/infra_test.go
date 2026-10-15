package vendas_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

func novoMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func irrestrito(t *testing.T) kernel.Escopo {
	t.Helper()
	admin, err := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	require.NoError(t, err)
	e, err := kernel.EscopoIrrestrito(admin)
	require.NoError(t, err)
	return e
}

func doVendedor(t *testing.T, id int64) kernel.Escopo {
	t.Helper()
	e, err := kernel.EscopoDoVendedor(id)
	require.NoError(t, err)
	return e
}

// O1 (§5.5, checklist ARQ-05): os dois repositórios com escopo devolvem
// erro com SemAcesso e com o zero value SEM nenhuma expectativa no sqlmock
// (qualquer SQL faria o mock falhar).
func TestARQ05_O1_SemAcessoEZeroValueSemSQL(t *testing.T) {
	ctx := context.Background()
	pg := kernel.NovaPagina(1, 20)
	escopos := map[string]struct {
		e    kernel.Escopo
		want error
	}{
		"SemAcesso":  {kernel.EscopoSemAcesso(), kernel.ErrEscopoSemAcesso},
		"zero value": {kernel.Escopo{}, kernel.ErrEscopoInvalido},
	}
	repos := map[string]func(db *sql.DB, e kernel.Escopo) error{
		"Pedidos.Listar": func(db *sql.DB, e kernel.Escopo) error {
			r, err := vendasmysql.NovoPedidos(db).Listar(ctx, e, vendas.FiltroPedido{VendedorID: 3}, pg)
			assert.Nil(t, r.Itens)
			return err
		},
		"Pagamentos.Listar": func(db *sql.DB, e kernel.Escopo) error {
			r, err := vendasmysql.NovoPagamentos(db).Listar(ctx, e, vendas.FiltroPagamento{PedidoID: 3}, pg)
			assert.Nil(t, r.Itens)
			return err
		},
	}
	for nomeRepo, chamar := range repos {
		for nomeEscopo, c := range escopos {
			t.Run(nomeRepo+"/"+nomeEscopo, func(t *testing.T) {
				db, mock := novoMock(t) // nenhuma expectativa: SQL nenhum
				err := chamar(db, c.e)
				assert.ErrorIs(t, err, c.want)
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

// E-03: no escopo de vendedor, o vendedor_id da query é SOBRESCRITO pelo do
// escopo (pedido); no pagamento, o escopo filtra pelos pedidos do vendedor
// (V-02). No irrestrito, o vendedor_id da query é filtro de negócio.
func TestARQ05_ListagemRestrita_SobrescreveVendedorDaQuery(t *testing.T) {
	ctx := context.Background()
	pg := kernel.NovaPagina(1, 20)

	db, mock := novoMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos p .+ WHERE p\.vendedor_id = \?`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
	mock.ExpectQuery(`WHERE p\.vendedor_id = \? ORDER BY`).WithArgs(int64(7), 20, 0).WillReturnRows(sqlmock.NewRows(nil))
	_, err := vendasmysql.NovoPedidos(db).Listar(ctx, doVendedor(t, 7), vendas.FiltroPedido{VendedorID: 99}, pg)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())

	db, mock = novoMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pedidos p .+ WHERE p\.vendedor_id = \?`).WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
	mock.ExpectQuery(`WHERE p\.vendedor_id = \? ORDER BY`).WithArgs(int64(99), 20, 0).WillReturnRows(sqlmock.NewRows(nil))
	_, err = vendasmysql.NovoPedidos(db).Listar(ctx, irrestrito(t), vendas.FiltroPedido{VendedorID: 99}, pg)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())

	db, mock = novoMock(t)
	sub := `pedido_id IN \(SELECT pedido_id_origem FROM pedidos WHERE vendedor_id = \?\)`
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pagamentos WHERE ` + sub).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
	mock.ExpectQuery(sub+` ORDER BY pagamento_id ASC`).WithArgs(int64(7), 20, 0).WillReturnRows(sqlmock.NewRows(nil))
	_, err = vendasmysql.NovoPagamentos(db).Listar(ctx, doVendedor(t, 7), vendas.FiltroPagamento{}, pg)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ05_Adapters_PanicSemQuerier(t *testing.T) {
	assert.Panics(t, func() { vendasmysql.NovoPedidos(nil) })
	assert.Panics(t, func() { vendasmysql.NovoPagamentos(nil) })
}

// C-02: cada valor do enum vira a coluna da whitelist do legado; fora do
// enum, o padrão.
func TestARQ05_Ordenacao_TodasAsColunas(t *testing.T) {
	ctx := context.Background()
	pedidos := map[vendas.CampoPedido]string{
		vendas.OrdemPedidoID: "p.pedido_id_origem", vendas.OrdemPedidoData: "p.data_pedido", vendas.OrdemPedidoCanal: "p.canal",
		vendas.OrdemPedidoStatus: "p.status", vendas.OrdemPedidoValorTotal: "p.valor_total", vendas.OrdemPedidoCriadoEm: "p.created_at",
		vendas.OrdemPedidoAtualizadoEm: "p.updated_at", vendas.OrdemPedidoClienteNome: "c.razao_social",
		vendas.OrdemPedidoVendedorNome: "v.nome", vendas.CampoPedido("x"): "p.pedido_id_origem",
	}
	for campo, coluna := range pedidos {
		db, mock := novoMock(t)
		mock.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
		mock.ExpectQuery(`ORDER BY ` + coluna + ` ASC LIMIT`).WillReturnRows(sqlmock.NewRows(nil))
		f := vendas.FiltroPedido{Ordenacao: kernel.NovaOrdenacao(campo, kernel.Asc)}
		_, err := vendasmysql.NovoPedidos(db).Listar(ctx, irrestrito(t), f, kernel.NovaPagina(1, 20))
		require.NoError(t, err, campo)
		assert.NoError(t, mock.ExpectationsWereMet(), campo)
	}
	pagamentos := map[vendas.CampoPagamento]string{
		vendas.OrdemPagamentoID: "pagamento_id", vendas.OrdemPagamentoPedidoID: "pedido_id", vendas.OrdemPagamentoForma: "forma_pagamento",
		vendas.OrdemPagamentoParcelas: "parcelas", vendas.OrdemPagamentoValor: "valor", vendas.OrdemPagamentoTaxaPct: "taxa_pct",
		vendas.OrdemPagamentoValorLiquido: "valor_liquido", vendas.OrdemPagamentoDataVencimento: "data_vencimento",
		vendas.OrdemPagamentoDataPagamento: "data_pagamento", vendas.OrdemPagamentoStatus: "status_pagamento",
		vendas.OrdemPagamentoCriadoEm: "created_at", vendas.OrdemPagamentoAtualizadoEm: "updated_at",
		vendas.CampoPagamento("x"): "pagamento_id",
	}
	for campo, coluna := range pagamentos {
		db, mock := novoMock(t)
		mock.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
		mock.ExpectQuery(`ORDER BY ` + coluna + ` DESC LIMIT`).WillReturnRows(sqlmock.NewRows(nil))
		f := vendas.FiltroPagamento{Ordenacao: kernel.NovaOrdenacao(campo, kernel.Desc)}
		_, err := vendasmysql.NovoPagamentos(db).Listar(ctx, irrestrito(t), f, kernel.NovaPagina(1, 20))
		require.NoError(t, err, campo)
		assert.NoError(t, mock.ExpectationsWereMet(), campo)
	}
}

// Ramos de infra que o fluxo HTTP não alcança: erro de banco no lock do
// PUT, na leitura dos itens atuais (query e scan) e no sku da baixa.
func TestARQ05_ErrosDeInfraNaTxDoPedido(t *testing.T) {
	ctx := context.Background()
	errBanco := errors.New("db down")

	db, mock := novoMock(t)
	mock.ExpectQuery(`SELECT status FROM pedidos WHERE pedido_id_origem = \? FOR UPDATE`).WillReturnError(errBanco)
	_, err := vendasmysql.NovoPedidos(db).StatusParaAtualizar(ctx, 1)
	assert.ErrorIs(t, err, errBanco)
	assert.NotErrorIs(t, err, vendas.ErrPedidoNaoEncontrado)

	db, mock = novoMock(t)
	mock.ExpectQuery(`SELECT produto_id, quantidade`).WillReturnError(errBanco)
	_, err = vendasmysql.NovoPedidos(db).ItensAtuais(ctx, 1)
	assert.ErrorIs(t, err, errBanco)

	db, mock = novoMock(t)
	mock.ExpectQuery(`SELECT produto_id, quantidade`).
		WillReturnRows(sqlmock.NewRows([]string{"produto_id", "quantidade", "preco_praticado", "desconto_pct"}).AddRow("x", 1, 1.0, 0.0))
	_, err = vendasmysql.NovoPedidos(db).ItensAtuais(ctx, 1)
	assert.ErrorContains(t, err, "vendasmysql: scan itens_pedido (tx)")

	db, mock = novoMock(t)
	mock.ExpectQuery(`SELECT sku FROM produtos WHERE id = \? LIMIT 1`).WillReturnError(errBanco)
	_, err = catalogomysql.NovoEstoques(db).SKUDoProduto(ctx, 1)
	assert.ErrorIs(t, err, errBanco)

	db, mock = novoMock(t)
	mock.ExpectQuery(`SELECT sku FROM produtos WHERE id = \? LIMIT 1`).WillReturnError(sql.ErrNoRows)
	_, err = catalogomysql.NovoEstoques(db).SKUDoProduto(ctx, 1)
	require.Error(t, err)
	var e *kernel.Erro
	assert.False(t, errors.As(err, &e), "produto inexistente na baixa é 500, nunca sentinela (404)")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// O6-iv / N2: um 1062 do driver chega ao log sem o valor duplicado.
func TestARQ05_ChaveDuplicadaSemValorDoDriver(t *testing.T) {
	ctx := context.Background()
	dup := &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'segredo-123' for key 'pagamentos.PRIMARY'"}

	db, mock := novoMock(t)
	mock.ExpectExec(`INSERT INTO pagamentos`).WillReturnError(dup)
	err := vendasmysql.NovoPagamentos(db).Inserir(ctx, &vendas.Pagamento{PedidoID: 1, DataVencimento: time.Now()})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "segredo-123")
	var me *mysql.MySQLError
	require.ErrorAs(t, err, &me)
	assert.Equal(t, uint16(1062), me.Number)
	assert.Contains(t, me.Message, "pagamentos.PRIMARY")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// NB-3 (SecBrain, movida do ARQ-04): o 1062 de SKU (produto) e de
// (data_snapshot, sku) (estoque) no Catálogo continua 500 (CHORE-06), mas o
// valor do driver não chega ao log.
func TestARQ05_NB3_DuplicadaDoCatalogoSemValor(t *testing.T) {
	ctx := context.Background()
	casos := map[string]struct {
		re     string
		dup    *mysql.MySQLError
		gravar func(db *sql.DB) error
	}{
		"SKU": {`INSERT INTO produtos`, &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'SKU-SECRETO' for key 'produtos.uk_produtos_sku'"},
			func(db *sql.DB) error { return catalogomysql.NovoProdutos(db).Inserir(ctx, novoProdutoMinimo(t)) }},
		"(data_snapshot, sku)": {`INSERT INTO estoque`, &mysql.MySQLError{Number: 1062, Message: "Duplicate entry '2024-01-01-SKU-SECRETO' for key 'estoque.uk_estoque_data_sku'"},
			func(db *sql.DB) error { return catalogomysql.NovoEstoques(db).Inserir(ctx, novoEstoqueMinimo(t)) }},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			db, mock := novoMock(t)
			mock.ExpectExec(c.re).WillReturnError(c.dup)
			err := c.gravar(db)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "SKU-SECRETO")
			var e *kernel.Erro
			assert.False(t, errors.As(err, &e), "status HTTP preservado: 500 (CHORE-06)")
			var me *mysql.MySQLError
			require.ErrorAs(t, err, &me)
			assert.Equal(t, uint16(1062), me.Number)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func novoProdutoMinimo(t *testing.T) *catalogo.Produto {
	t.Helper()
	return &catalogo.Produto{SKU: catalogo.SKU("SKU-SECRETO")}
}

func novoEstoqueMinimo(t *testing.T) *catalogo.RegistroEstoque {
	t.Helper()
	return &catalogo.RegistroEstoque{SKU: catalogo.SKU("SKU-SECRETO"), DataSnapshot: time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local)}
}
