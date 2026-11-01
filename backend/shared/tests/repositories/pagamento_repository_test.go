// Package repositories_test contém testes de integração com banco de dados mockado.
// Cada função é stateless — o sqlmock substitui o driver real.
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

var pagamentoColumns = []string{
	"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct",
	"valor_liquido", "data_vencimento", "data_pagamento", "status_pagamento", "created_at", "updated_at",
}

func pagamentoRow(pagamentoID, pedidoID int64, forma string, parcelas uint8, valor, taxaPct, valorLiquido float64, dataVencimento time.Time, dataPagamento *time.Time, status string, created, updated time.Time) []driver.Value {
	var dp driver.Value
	if dataPagamento != nil {
		dp = *dataPagamento
	}
	return []driver.Value{pagamentoID, pedidoID, forma, parcelas, valor, taxaPct, valorLiquido, dataVencimento, dp, status, created, updated}
}

const pagamentoFromRegexp = `FROM pagamentos`

func TestPagamentoList_SemFiltro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pagamentoFromRegexp).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(pagamentoColumns).
		AddRow(pagamentoRow(2, 20, "PIX", 1, 100.0, 0, 100.0, now, nil, "Em aberto", now, now)...).
		AddRow(pagamentoRow(1, 10, "Boleto 14 dias", 1, 200.0, 2.5, 195.0, now, nil, "Pago", now, now)...)

	mock.ExpectQuery(`SELECT .+ `+pagamentoFromRegexp+` ORDER BY pagamento_id ASC LIMIT \? OFFSET \?`).
		WithArgs(10, 0).
		WillReturnRows(rows)

	ctx := context.Background()
	pagamentos, total, err := itensETotal(vendasmysql.NovoPagamentos(db).Listar(ctx, escopoAdmin(), vendas.FiltroPagamento{Ordenacao: vendas.OrdenacaoPagamentoPadrao()}, kernel.NovaPagina(1, 10)))

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, pagamentos, 2)
	assert.Equal(t, "PIX", pagamentos[0].FormaPagamento)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoList_ComFiltros(t *testing.T) {
	testes := []struct {
		nome        string
		filtro      vendas.FiltroPagamento
		vendedor    int64 // escopo (0 = admin)
		whereRegexp string
		args        []driver.Value
	}{
		{
			nome:        "filtro por status_pagamento",
			filtro:      vendas.FiltroPagamento{StatusPagamento: "Pago"},
			whereRegexp: `WHERE status_pagamento = \?`,
			args:        []driver.Value{"Pago"},
		},
		{
			nome:        "filtro por forma_pagamento",
			filtro:      vendas.FiltroPagamento{FormaPagamento: "PIX"},
			whereRegexp: `WHERE forma_pagamento = \?`,
			args:        []driver.Value{"PIX"},
		},
		{
			nome:        "filtro por pedido_id",
			filtro:      vendas.FiltroPagamento{PedidoID: 5},
			whereRegexp: `WHERE pedido_id = \?`,
			args:        []driver.Value{int64(5)},
		},
		{
			nome:        "filtro por vencimento_de e vencimento_ate",
			filtro:      vendas.FiltroPagamento{VencimentoDe: "2024-01-01", VencimentoAte: "2024-01-31"},
			whereRegexp: `WHERE data_vencimento >= \? AND data_vencimento <= \?`,
			args:        []driver.Value{"2024-01-01", "2024-01-31"},
		},
		{
			nome:        "filtro por vendedor_id (restrição de carteira)",
			filtro:      vendas.FiltroPagamento{},
			vendedor:    7,
			whereRegexp: `WHERE pedido_id IN \(SELECT pedido_id_origem FROM pedidos WHERE vendedor_id = \?\)`,
			args:        []driver.Value{int64(7)},
		},
		{
			nome: "filtro combinado (todos os campos)",
			filtro: vendas.FiltroPagamento{
				StatusPagamento: "Pago",
				FormaPagamento:  "PIX",
				PedidoID:        5,
				VencimentoDe:    "2024-01-01",
				VencimentoAte:   "2024-01-31",
			},
			vendedor:    7,
			whereRegexp: `WHERE status_pagamento = \? AND forma_pagamento = \? AND pedido_id = \? AND data_vencimento >= \? AND data_vencimento <= \? AND pedido_id IN \(SELECT pedido_id_origem FROM pedidos WHERE vendedor_id = \?\)`,
			args:        []driver.Value{"Pago", "PIX", int64(5), "2024-01-01", "2024-01-31", int64(7)},
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pagamentoFromRegexp + ` ` + tt.whereRegexp).
				WithArgs(tt.args...).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

			allArgs := append(append([]driver.Value{}, tt.args...), int64(10), int64(0))
			mock.ExpectQuery(`SELECT .+ ` + pagamentoFromRegexp + ` ` + tt.whereRegexp + ` ORDER BY pagamento_id ASC LIMIT \? OFFSET \?`).
				WithArgs(allArgs...).
				WillReturnRows(sqlmock.NewRows(pagamentoColumns))

			ctx := context.Background()
			f := tt.filtro
			f.Ordenacao = vendas.OrdenacaoPagamentoPadrao() // o parse do HTTP com order_by vazio
			_, _, err := itensETotal(vendasmysql.NovoPagamentos(db).Listar(ctx, escopoDe(tt.vendedor), f, kernel.NovaPagina(1, 10)))

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPagamentoList_OrderBy(t *testing.T) {
	testes := []struct {
		nome        string
		orderBy     string
		orderDir    string
		orderRegexp string
	}{
		{"order_by válido asc", "valor", "asc", `ORDER BY valor ASC`},
		{"order_by válido desc", "data_vencimento", "desc", `ORDER BY data_vencimento DESC`},
		{"order_by fora da whitelist cai no default", "1; DROP TABLE pagamentos;--", "asc", `ORDER BY pagamento_id ASC`},
		{"order_dir inválido cai no default (asc)", "status_pagamento", "invalido", `ORDER BY status_pagamento ASC`},
		{"tudo vazio cai no default", "", "", `ORDER BY pagamento_id ASC`},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pagamentoFromRegexp).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery(`SELECT .+ `+pagamentoFromRegexp+` `+tt.orderRegexp+` LIMIT \? OFFSET \?`).
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows(pagamentoColumns))

			ctx := context.Background()
			_, _, err := itensETotal(vendasmysql.NovoPagamentos(db).Listar(ctx, escopoAdmin(), vendas.FiltroPagamento{Ordenacao: kernel.ParseOrdenacao(tt.orderBy, tt.orderDir, vendas.CamposOrdenacaoPagamento(), vendas.OrdenacaoPagamentoPadrao())}, kernel.NovaPagina(1, 10)))

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPagamentoList_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pagamentoFromRegexp).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, _, err := itensETotal(vendasmysql.NovoPagamentos(db).Listar(ctx, escopoAdmin(), vendas.FiltroPagamento{Ordenacao: vendas.OrdenacaoPagamentoPadrao()}, kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoList_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pagamentoFromRegexp).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT .+ `+pagamentoFromRegexp).
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, _, err := itensETotal(vendasmysql.NovoPagamentos(db).Listar(ctx, escopoAdmin(), vendas.FiltroPagamento{Ordenacao: vendas.OrdenacaoPagamentoPadrao()}, kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) ` + pagamentoFromRegexp).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	now := time.Now()
	mock.ExpectQuery(`SELECT .+ `+pagamentoFromRegexp).
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(pagamentoColumns).
			AddRow(pagamentoRow(1, 10, "PIX", 1, 100.0, 0, 100.0, now, nil, "Em aberto", now, now)...).
			RowError(0, sql.ErrConnDone))

	ctx := context.Background()
	_, _, err := itensETotal(vendasmysql.NovoPagamentos(db).Listar(ctx, escopoAdmin(), vendas.FiltroPagamento{Ordenacao: vendas.OrdenacaoPagamentoPadrao()}, kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	dataPagamento := now
	rows := sqlmock.NewRows(pagamentoColumns).
		AddRow(pagamentoRow(1, 10, "PIX", 1, 100.0, 0, 100.0, now, &dataPagamento, "Pago", now, now)...)

	mock.ExpectQuery(`SELECT .+ ` + pagamentoFromRegexp + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	ctx := context.Background()
	p, err := vendasmysql.NovoPagamentos(db).PorID(ctx, 1)

	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, int64(1), p.PagamentoID)
	assert.Equal(t, int64(10), p.PedidoID)
	require.NotNil(t, p.DataPagamento)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoGetByID_SemDataPagamento(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(pagamentoColumns).
		AddRow(pagamentoRow(1, 10, "PIX", 1, 100.0, 0, 100.0, now, nil, "Em aberto", now, now)...)

	mock.ExpectQuery(`SELECT .+ ` + pagamentoFromRegexp + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	ctx := context.Background()
	p, err := vendasmysql.NovoPagamentos(db).PorID(ctx, 1)

	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Nil(t, p.DataPagamento)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ ` + pagamentoFromRegexp + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	ctx := context.Background()
	p, err := vendasmysql.NovoPagamentos(db).PorID(ctx, 999)

	assert.Nil(t, p)
	assert.ErrorIs(t, err, vendas.ErrPagamentoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ ` + pagamentoFromRegexp + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	p, err := vendasmysql.NovoPagamentos(db).PorID(ctx, 1)

	assert.Nil(t, p)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, vendas.ErrPagamentoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	p := &vendas.Pagamento{
		PedidoID:        10,
		FormaPagamento:  "PIX",
		Parcelas:        1,
		Valor:           100.0,
		TaxaPct:         0,
		ValorLiquido:    100.0,
		DataVencimento:  now,
		StatusPagamento: "Em aberto",
	}

	mock.ExpectExec(`INSERT INTO pagamentos \(pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento\)`).
		WithArgs(p.PedidoID, p.FormaPagamento, p.Parcelas, p.Valor, p.TaxaPct, p.ValorLiquido, p.DataVencimento, nil, p.StatusPagamento).
		WillReturnResult(sqlmock.NewResult(42, 1))

	ctx := context.Background()
	err := vendasmysql.NovoPagamentos(db).Inserir(ctx, p)

	require.NoError(t, err)
	assert.Equal(t, int64(42), p.PagamentoID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoCreate_ComDataPagamento(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	dataPagamento := now
	p := &vendas.Pagamento{
		PedidoID:        10,
		FormaPagamento:  "PIX",
		Parcelas:        1,
		Valor:           100.0,
		ValorLiquido:    100.0,
		DataVencimento:  now,
		DataPagamento:   &dataPagamento,
		StatusPagamento: "Pago",
	}

	mock.ExpectExec(`INSERT INTO pagamentos`).
		WithArgs(p.PedidoID, p.FormaPagamento, p.Parcelas, p.Valor, p.TaxaPct, p.ValorLiquido, p.DataVencimento, dataPagamento, p.StatusPagamento).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	err := vendasmysql.NovoPagamentos(db).Inserir(ctx, p)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoCreate_ErroInsert(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &vendas.Pagamento{}
	mock.ExpectExec(`INSERT INTO pagamentos`).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	err := vendasmysql.NovoPagamentos(db).Inserir(ctx, p)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoUpdate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	p := &vendas.Pagamento{
		FormaPagamento:  "Boleto 14 dias",
		Parcelas:        2,
		Valor:           200.0,
		TaxaPct:         1.5,
		ValorLiquido:    197.0,
		DataVencimento:  now,
		StatusPagamento: "Pago com atraso",
	}

	mock.ExpectExec(`UPDATE pagamentos\s+SET forma_pagamento = \?, parcelas = \?, valor = \?, taxa_pct = \?, valor_liquido = \?, data_vencimento = \?, data_pagamento = \?, status_pagamento = \?\s+WHERE pagamento_id = \?`).
		WithArgs(p.FormaPagamento, p.Parcelas, p.Valor, p.TaxaPct, p.ValorLiquido, p.DataVencimento, nil, p.StatusPagamento, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	err := vendasmysql.NovoPagamentos(db).Atualizar(ctx, 1, p)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoUpdate_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &vendas.Pagamento{}
	mock.ExpectExec(`UPDATE pagamentos`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	ctx := context.Background()
	err := vendasmysql.NovoPagamentos(db).Atualizar(ctx, 999, p)

	assert.ErrorIs(t, err, vendas.ErrPagamentoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoUpdate_ErroExec(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	p := &vendas.Pagamento{}
	mock.ExpectExec(`UPDATE pagamentos`).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	err := vendasmysql.NovoPagamentos(db).Atualizar(ctx, 1, p)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
