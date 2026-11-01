package services_test

import (
	"context"
	"database/sql"
	"github.com/rotaperfumes/shared/kernel"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/vendas"
)

func pagamentoTestCfg(verbose bool) *config.Config {
	return &config.Config{Verbose: verbose}
}

func newPagamentoTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

// ---------------------------------------------------------------------------
// Regexes/colunas que espelham as constantes de
// backend/shared/vendas/vendasmysql (SQL do legado pagamento_repository.go).
// ---------------------------------------------------------------------------

const pagamentoColunasRegex = `pagamento_id, pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento, created_at, updated_at`
const pagamentoFromRegex = ` FROM pagamentos`

func pagamentoColunasHeader() []string {
	return []string{
		"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct",
		"valor_liquido", "data_vencimento", "data_pagamento", "status_pagamento", "created_at", "updated_at",
	}
}

func pagamentoRows(id, pedidoID int64, valor float64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(pagamentoColunasHeader()).
		AddRow(id, pedidoID, "PIX", uint8(1), valor, 0.0, valor, now, nil, "Em aberto", now, now)
}

func emptyPagamentoRows() *sqlmock.Rows {
	return sqlmock.NewRows(pagamentoColunasHeader())
}

// validPagamentoInput retorna um input válido para create/update.
func validPagamentoInput() vendas.DadosNovoPagamento {
	return vendas.DadosNovoPagamento{PedidoID: 1, DadosPagamento: vendas.DadosPagamento{
		FormaPagamento:  "PIX",
		Parcelas:        1,
		Valor:           100.0,
		TaxaPct:         0,
		ValorLiquido:    100.0,
		DataVencimento:  "2024-01-15",
		StatusPagamento: "Em aberto",
	}}
}

// ---------------------------------------------------------------------------
// ListPagamentos
// ---------------------------------------------------------------------------

func TestPagamentoService_ListPagamentos(t *testing.T) {
	testCases := []struct {
		nome    string
		filtro  vendas.FiltroPagamento
		page    int
		limit   int
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
		wantTot int
	}{
		{
			nome:   "sem filtros - sucesso",
			filtro: vendas.FiltroPagamento{Ordenacao: vendas.OrdenacaoPagamentoPadrao()},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\)` + pagamentoFromRegex).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT `+pagamentoColunasRegex+pagamentoFromRegex+` ORDER BY pagamento_id ASC LIMIT \? OFFSET \?`).
					WithArgs(20, 0).
					WillReturnRows(pagamentoRows(1, 1, 100.0))
			},
			wantLen: 1,
			wantTot: 1,
		},
		{
			nome: "com todos os filtros - repassa args ao repo",
			filtro: vendas.FiltroPagamento{
				StatusPagamento: "Pago",
				FormaPagamento:  "PIX",
				PedidoID:        5,
				VencimentoDe:    "2024-01-01",
				VencimentoAte:   "2024-01-31",
				Ordenacao:       vendas.OrdenacaoPagamentoPadrao(),
			},
			page:  2,
			limit: 10,
			mock: func(mock sqlmock.Sqlmock) {
				whereRegex := ` WHERE status_pagamento = \? AND forma_pagamento = \? AND pedido_id = \? AND data_vencimento >= \? AND data_vencimento <= \?`
				mock.ExpectQuery(`SELECT COUNT\(\*\)`+pagamentoFromRegex+whereRegex).
					WithArgs("Pago", "PIX", int64(5), "2024-01-01", "2024-01-31").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
				mock.ExpectQuery(`SELECT `+pagamentoColunasRegex+pagamentoFromRegex+whereRegex+` ORDER BY pagamento_id ASC LIMIT \? OFFSET \?`).
					WithArgs("Pago", "PIX", int64(5), "2024-01-01", "2024-01-31", 10, 10).
					WillReturnRows(pagamentoRows(1, 5, 100.0))
			},
			wantLen: 1,
			wantTot: 3,
		},
		{
			nome:   "erro no count do repo é propagado",
			filtro: vendas.FiltroPagamento{Ordenacao: vendas.OrdenacaoPagamentoPadrao()},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\)` + pagamentoFromRegex).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newPagamentoTestDB(t)
			tc.mock(mock)

			casos := casosVendas(db, 0)
			pagamentos, total, err := itensETotal(casos.ListarPagamentos(context.Background(), principalAdmin(), lerFixo(tc.filtro), kernel.NovaPagina(tc.page, tc.limit)))

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, pagamentos, tc.wantLen)
				assert.Equal(t, tc.wantTot, total)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetPagamentoByID
// ---------------------------------------------------------------------------

func TestPagamentoService_GetPagamentoByID(t *testing.T) {
	t.Run("encontrado", func(t *testing.T) {
		db, mock := newPagamentoTestDB(t)
		mock.ExpectQuery(`SELECT ` + pagamentoColunasRegex + pagamentoFromRegex + ` WHERE pagamento_id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(pagamentoRows(1, 1, 100.0))

		casos := casosVendas(db, 0)
		p, err := casos.ObterPagamento(context.Background(), principalAdmin(), 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), p.PagamentoID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("não encontrado retorna ErrPagamentoNaoEncontrado", func(t *testing.T) {
		db, mock := newPagamentoTestDB(t)
		mock.ExpectQuery(`SELECT ` + pagamentoColunasRegex + pagamentoFromRegex + ` WHERE pagamento_id = \? LIMIT 1`).
			WithArgs(int64(999)).
			WillReturnError(sql.ErrNoRows)

		casos := casosVendas(db, 0)
		p, err := casos.ObterPagamento(context.Background(), principalAdmin(), 999)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, vendas.ErrPagamentoNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro genérico é propagado", func(t *testing.T) {
		db, mock := newPagamentoTestDB(t)
		mock.ExpectQuery(`SELECT ` + pagamentoColunasRegex + pagamentoFromRegex + ` WHERE pagamento_id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnError(sql.ErrConnDone)

		casos := casosVendas(db, 0)
		p, err := casos.ObterPagamento(context.Background(), principalAdmin(), 1)
		assert.Nil(t, p)
		assert.Error(t, err)
		assert.False(t, err == vendas.ErrPagamentoNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	_ = emptyPagamentoRows // usado em outros testes de repositório equivalentes
}

// ---------------------------------------------------------------------------
// Validações comuns a CreatePagamento/UpdatePagamento
// ---------------------------------------------------------------------------

func pagamentoValidacaoTestCases() []struct {
	nome    string
	input   func() vendas.DadosNovoPagamento
	wantErr error
} {
	return []struct {
		nome    string
		input   func() vendas.DadosNovoPagamento
		wantErr error
	}{
		{
			nome: "forma_pagamento inválida",
			input: func() vendas.DadosNovoPagamento {
				in := validPagamentoInput()
				in.FormaPagamento = "Bitcoin"
				return in
			},
			wantErr: vendas.ErrFormaPagamentoInvalida,
		},
		{
			nome: "status_pagamento inválido",
			input: func() vendas.DadosNovoPagamento {
				in := validPagamentoInput()
				in.StatusPagamento = "Cancelado"
				return in
			},
			wantErr: vendas.ErrStatusPagamentoInvalido,
		},
		{
			nome: "parcelas menor que 1",
			input: func() vendas.DadosNovoPagamento {
				in := validPagamentoInput()
				in.Parcelas = 0
				return in
			},
			wantErr: vendas.ErrParcelasInvalidas,
		},
		{
			nome: "valor negativo",
			input: func() vendas.DadosNovoPagamento {
				in := validPagamentoInput()
				in.Valor = -0.01
				return in
			},
			wantErr: vendas.ErrValorInvalido,
		},
		{
			nome: "taxa_pct negativa",
			input: func() vendas.DadosNovoPagamento {
				in := validPagamentoInput()
				in.TaxaPct = -1
				return in
			},
			wantErr: vendas.ErrTaxaPctInvalida,
		},
		{
			nome: "valor_liquido negativo",
			input: func() vendas.DadosNovoPagamento {
				in := validPagamentoInput()
				in.ValorLiquido = -1
				return in
			},
			wantErr: vendas.ErrValorLiquidoInvalido,
		},
		{
			nome: "data_vencimento ausente",
			input: func() vendas.DadosNovoPagamento {
				in := validPagamentoInput()
				in.DataVencimento = ""
				return in
			},
			wantErr: vendas.ErrDataVencimentoObrigatoria,
		},
		{
			nome: "data_vencimento em formato inválido",
			input: func() vendas.DadosNovoPagamento {
				in := validPagamentoInput()
				in.DataVencimento = "15/01/2024"
				return in
			},
			wantErr: vendas.ErrDataVencimentoInvalida,
		},
		{
			nome: "data_pagamento em formato inválido",
			input: func() vendas.DadosNovoPagamento {
				in := validPagamentoInput()
				in.DataPagamento = "15/01/2024"
				return in
			},
			wantErr: vendas.ErrDataPagamentoInvalida,
		},
	}
}

// ---------------------------------------------------------------------------
// CreatePagamento
// ---------------------------------------------------------------------------

func TestPagamentoService_CreatePagamento_PedidoIDObrigatorio(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	// nenhuma query esperada: validação falha antes de tocar o repo.
	casos := casosVendas(db, 0)

	in := validPagamentoInput()
	in.PedidoID = 0
	p, err := casos.CriarPagamento(context.Background(), principalAdmin(), lerFixo(in))
	assert.Nil(t, p)
	assert.ErrorIs(t, err, vendas.ErrPedidoIDObrigatorio)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoService_CreatePagamento_PedidoNaoEncontrado(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegex).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	casos := casosVendas(db, 0)
	in := validPagamentoInput()
	in.PedidoID = 999
	p, err := casos.CriarPagamento(context.Background(), principalAdmin(), lerFixo(in))
	assert.Nil(t, p)
	assert.ErrorIs(t, err, vendas.ErrPedidoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoService_CreatePagamento_ErroExistsByID(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(selectStatusForUpdateRegex).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	casos := casosVendas(db, 0)
	p, err := casos.CriarPagamento(context.Background(), principalAdmin(), lerFixo(validPagamentoInput()))
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoService_CreatePagamento_Validacoes(t *testing.T) {
	for _, tc := range pagamentoValidacaoTestCases() {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newPagamentoTestDB(t)
			esperarLockPedido(mock, 1, 1)
			mock.ExpectRollback()

			casos := casosVendas(db, 0)
			p, err := casos.CriarPagamento(context.Background(), principalAdmin(), lerFixo(tc.input()))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPagamentoService_CreatePagamento_Sucesso(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	esperarLockPedido(mock, 1, 1)
	mock.ExpectExec(`INSERT INTO pagamentos \(pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento\)`).
		WithArgs(int64(1), "PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	casos := casosVendas(db, 0)
	p, err := casos.CriarPagamento(context.Background(), principalAdmin(), lerFixo(validPagamentoInput()))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, int64(1), p.PagamentoID)
	assert.Equal(t, int64(1), p.PedidoID)
	assert.Equal(t, "Em aberto", p.StatusPagamento)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoService_CreatePagamento_ErroCreateRepo(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	esperarLockPedido(mock, 1, 1)
	mock.ExpectExec(`INSERT INTO pagamentos`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	casos := casosVendas(db, 0)
	p, err := casos.CriarPagamento(context.Background(), principalAdmin(), lerFixo(validPagamentoInput()))
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UpdatePagamento
// ---------------------------------------------------------------------------

func TestPagamentoService_UpdatePagamento_Validacoes(t *testing.T) {
	for _, tc := range pagamentoValidacaoTestCases() {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newPagamentoTestDB(t)
			// nenhuma query esperada: validação falha antes de tocar o repo
			// (UpdatePagamento não valida pedido_id — não é editável).
			casos := casosVendas(db, 0)
			p, err := casos.AtualizarPagamento(context.Background(), principalAdmin(), 1, lerFixo(tc.input().DadosPagamento))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPagamentoService_UpdatePagamento_IgnoraPedidoIDEPagamentoIDDoInput(t *testing.T) {
	// input.PedidoID/PagamentoID não fazem parte de vendas.DadosNovoPagamento usado por
	// UpdatePagamento (o handler nem os aceita no DTO de update) — o teste
	// documenta que o Update usa exclusivamente o pagamentoID do path.
	db, mock := newPagamentoTestDB(t)
	mock.ExpectExec(`UPDATE pagamentos\s+SET forma_pagamento = \?, parcelas = \?, valor = \?, taxa_pct = \?, valor_liquido = \?, data_vencimento = \?, data_pagamento = \?, status_pagamento = \?\s+WHERE pagamento_id = \?`).
		WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + pagamentoColunasRegex + pagamentoFromRegex + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pagamentoRows(1, 1, 100.0))

	casos := casosVendas(db, 0)
	in := validPagamentoInput()
	in.PedidoID = 999 // ignorado: vendas.DadosNovoPagamento.PedidoID não é lido pelo Update
	p, err := casos.AtualizarPagamento(context.Background(), principalAdmin(), 1, lerFixo(in.DadosPagamento))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, int64(1), p.PedidoID) // permanece o do banco, não o do input
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoService_UpdatePagamento_NaoEncontrado(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	mock.ExpectExec(`UPDATE pagamentos`).
		WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto", int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	casos := casosVendas(db, 0)
	p, err := casos.AtualizarPagamento(context.Background(), principalAdmin(), 999, lerFixo(validPagamentoInput().DadosPagamento))
	assert.Nil(t, p)
	assert.ErrorIs(t, err, vendas.ErrPagamentoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoService_UpdatePagamento_ErroGenericoDoRepo(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	mock.ExpectExec(`UPDATE pagamentos`).
		WillReturnError(sql.ErrConnDone)

	casos := casosVendas(db, 0)
	p, err := casos.AtualizarPagamento(context.Background(), principalAdmin(), 1, lerFixo(validPagamentoInput().DadosPagamento))
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.False(t, err == vendas.ErrPagamentoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoService_UpdatePagamento_ErroGetByIDAposUpdate(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	mock.ExpectExec(`UPDATE pagamentos`).
		WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + pagamentoColunasRegex + pagamentoFromRegex + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	casos := casosVendas(db, 0)
	p, err := casos.AtualizarPagamento(context.Background(), principalAdmin(), 1, lerFixo(validPagamentoInput().DadosPagamento))
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoService_UpdatePagamento_ComDataPagamento(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	mock.ExpectExec(`UPDATE pagamentos`).
		WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), sqlmock.AnyArg(), "Pago", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + pagamentoColunasRegex + pagamentoFromRegex + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pagamentoRows(1, 1, 100.0))

	casos := casosVendas(db, 0)
	in := validPagamentoInput()
	in.StatusPagamento = "Pago"
	in.DataPagamento = "2024-01-20"
	p, err := casos.AtualizarPagamento(context.Background(), principalAdmin(), 1, lerFixo(in.DadosPagamento))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// DeletePagamento
// ---------------------------------------------------------------------------

// deletePagamentoRegex é o DELETE condicionado a não quitado (SEC-35).
const deletePagamentoRegex = `DELETE FROM pagamentos WHERE pagamento_id = \? AND status_pagamento NOT IN \(\?, \?\)`

// SEC-35: o admin não trava o pedido; o pagamento é travado na tx.
func TestPagamentoService_DeletePagamento_Sucesso(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	esperarPossePagamento(mock, 1)
	mock.ExpectBegin()
	esperarLockPagamento(mock, 1, 1, "Em aberto")
	mock.ExpectExec(deletePagamentoRegex).
		WithArgs(int64(1), "Pago", "Pago com atraso").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	casos := casosVendas(db, 0)
	err := casos.ExcluirPagamento(context.Background(), principalAdmin(), 1)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagamentoService_DeletePagamento_NaoEncontrado(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	esperarPossePagamento(mock, 999)
	mock.ExpectBegin()
	mock.ExpectQuery(selectPagamentoForUpdateRegex).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	casos := casosVendas(db, 0)
	err := casos.ExcluirPagamento(context.Background(), principalAdmin(), 999)
	assert.ErrorIs(t, err, vendas.ErrPagamentoNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestPagamentoService_DeletePagamento_JaQuitado_409 cobre a regra do
// SecBrain: pagamento com status_pagamento "Pago" ou "Pago com atraso" não
// pode ser excluído (preserva a trilha financeira).
func TestPagamentoService_DeletePagamento_JaQuitado_409(t *testing.T) {
	statusQuitados := []string{"Pago", "Pago com atraso"}
	for _, status := range statusQuitados {
		t.Run(status, func(t *testing.T) {
			db, mock := newPagamentoTestDB(t)
			esperarPossePagamento(mock, 1)
			mock.ExpectBegin()
			esperarLockPagamento(mock, 1, 1, status)
			mock.ExpectRollback()

			casos := casosVendas(db, 0)
			err := casos.ExcluirPagamento(context.Background(), principalAdmin(), 1)
			assert.ErrorIs(t, err, vendas.ErrPagamentoJaQuitadoNaoPodeSerExcluido)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPagamentoService_DeletePagamento_ErroDeleteRepo(t *testing.T) {
	db, mock := newPagamentoTestDB(t)
	esperarPossePagamento(mock, 1)
	mock.ExpectBegin()
	esperarLockPagamento(mock, 1, 1, "Em aberto")
	mock.ExpectExec(deletePagamentoRegex).
		WithArgs(int64(1), "Pago", "Pago com atraso").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	casos := casosVendas(db, 0)
	err := casos.ExcluirPagamento(context.Background(), principalAdmin(), 1)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
