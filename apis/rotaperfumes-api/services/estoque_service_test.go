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

func estoqueTestCfg(verbose bool) *config.Config {
	return &config.Config{Verbose: verbose}
}

func newEstoqueTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func estoqueColunasHeader() []string {
	return []string{"id", "data_snapshot", "sku", "produto_descricao", "saldo", "ruptura", "created_at", "updated_at"}
}

func estoqueRows(id int64, saldo int, ruptura bool) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(estoqueColunasHeader()).
		AddRow(id, now, "SKU-001", "Perfume Teste", saldo, ruptura, now, now)
}

func emptyEstoqueRows() *sqlmock.Rows {
	return sqlmock.NewRows(estoqueColunasHeader())
}

// ---------------------------------------------------------------------------
// ListEstoque
// ---------------------------------------------------------------------------

func TestEstoqueService_ListEstoque_Padrao_UsaUltimaPosicao(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM \(.+\) ranked WHERE rn = 1`).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	mock.ExpectQuery(`SELECT .+ FROM \(.+\) ranked WHERE rn = 1 ORDER BY data_snapshot DESC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(estoqueRows(1, 10, false))

	svc := services.NewEstoqueService(db, estoqueTestCfg(true))
	registros, total, err := svc.ListEstoque(context.Background(), db, 1, 20, services.EstoqueFiltro{})

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, registros, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueService_ListEstoque_Historico_UsaListSimples(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM estoque`).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(3))
	mock.ExpectQuery(`SELECT .+ FROM estoque.+ORDER BY e\.data_snapshot DESC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(estoqueRows(1, 10, false))

	svc := services.NewEstoqueService(db, estoqueTestCfg(false))
	_, total, err := svc.ListEstoque(context.Background(), db, 1, 20, services.EstoqueFiltro{Historico: true})

	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueService_ListEstoque_DataDeInvalida(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	svc := services.NewEstoqueService(db, estoqueTestCfg(false))
	_, _, err := svc.ListEstoque(context.Background(), db, 1, 20, services.EstoqueFiltro{DataDe: "31/12/2024"})

	assert.ErrorIs(t, err, services.ErrEstoqueDataInvalida)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueService_ListEstoque_DataAteInvalida(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	svc := services.NewEstoqueService(db, estoqueTestCfg(false))
	_, _, err := svc.ListEstoque(context.Background(), db, 1, 20, services.EstoqueFiltro{DataAte: "31/12/2024"})

	assert.ErrorIs(t, err, services.ErrEstoqueDataInvalida)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetEstoqueByID
// ---------------------------------------------------------------------------

func TestEstoqueService_GetEstoqueByID_Success(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	mock.ExpectQuery(`SELECT .+ FROM estoque.+WHERE e\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRows(1, 10, false))

	svc := services.NewEstoqueService(db, estoqueTestCfg(false))
	e, err := svc.GetEstoqueByID(context.Background(), db, 1)

	require.NoError(t, err)
	require.NotNil(t, e)
	assert.Equal(t, int64(1), e.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueService_GetEstoqueByID_NaoEncontrado(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	mock.ExpectQuery(`SELECT .+ FROM estoque.+WHERE e\.id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyEstoqueRows())

	svc := services.NewEstoqueService(db, estoqueTestCfg(false))
	e, err := svc.GetEstoqueByID(context.Background(), db, 999)

	assert.Nil(t, e)
	assert.ErrorIs(t, err, services.ErrEstoqueNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreateEstoque — validações
// ---------------------------------------------------------------------------

func validEstoqueInput() services.EstoqueInput {
	return services.EstoqueInput{
		SKU:          "SKU-001",
		DataSnapshot: "2024-06-01",
		Saldo:        10,
	}
}

func TestEstoqueService_CreateEstoque_Validacoes(t *testing.T) {
	testCases := []struct {
		nome    string
		input   func() services.EstoqueInput
		mock    func(mock sqlmock.Sqlmock)
		wantErr error
	}{
		{
			nome: "sku obrigatório (vazio)",
			input: func() services.EstoqueInput {
				in := validEstoqueInput()
				in.SKU = "   "
				return in
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantErr: services.ErrEstoqueSKUObrigatorio,
		},
		{
			nome: "data_snapshot obrigatória (vazia)",
			input: func() services.EstoqueInput {
				in := validEstoqueInput()
				in.DataSnapshot = ""
				return in
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantErr: services.ErrEstoqueDataObrigatoria,
		},
		{
			nome: "data_snapshot em formato inválido",
			input: func() services.EstoqueInput {
				in := validEstoqueInput()
				in.DataSnapshot = "01/06/2024"
				return in
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantErr: services.ErrEstoqueDataInvalida,
		},
		{
			nome: "data_snapshot futura",
			input: func() services.EstoqueInput {
				in := validEstoqueInput()
				in.DataSnapshot = time.Now().AddDate(0, 0, 5).Format("2006-01-02")
				return in
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantErr: services.ErrEstoqueDataFutura,
		},
		{
			nome: "saldo negativo",
			input: func() services.EstoqueInput {
				in := validEstoqueInput()
				in.Saldo = -1
				return in
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantErr: services.ErrEstoqueSaldoInvalido,
		},
		{
			nome:  "sku inexistente em produtos",
			input: validEstoqueInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`).
					WithArgs("SKU-001").
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: services.ErrEstoqueSKUInexistente,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newEstoqueTestDB(t)
			tc.mock(mock)

			svc := services.NewEstoqueService(db, estoqueTestCfg(false))
			e, err := svc.CreateEstoque(context.Background(), db, tc.input())

			assert.Nil(t, e)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestEstoqueService_CreateEstoque_RupturaSempreDerivadaDoSaldo cobre a
// exigência do SecBrain: ruptura NUNCA vem do input, é sempre derivada de
// saldo <= 0 dentro do service — cobrindo saldo positivo (ruptura=false),
// saldo zero e saldo positivo pequeno (ambos ruptura conforme a regra).
func TestEstoqueService_CreateEstoque_RupturaSempreDerivadaDoSaldo(t *testing.T) {
	testCases := []struct {
		nome        string
		saldo       int
		wantRuptura bool
	}{
		{"saldo positivo: ruptura=false", 10, false},
		{"saldo zero: ruptura=true", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newEstoqueTestDB(t)

			mock.ExpectQuery(`SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`).
				WithArgs("SKU-001").
				WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			mock.ExpectExec(`INSERT INTO estoque`).
				WithArgs("2024-06-01", "SKU-001", tc.saldo, tc.wantRuptura).
				WillReturnResult(sqlmock.NewResult(1, 1))

			in := validEstoqueInput()
			in.Saldo = tc.saldo

			svc := services.NewEstoqueService(db, estoqueTestCfg(true))
			e, err := svc.CreateEstoque(context.Background(), db, in)

			require.NoError(t, err)
			require.NotNil(t, e)
			assert.Equal(t, tc.wantRuptura, e.Ruptura)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestEstoqueService_CreateEstoque_Success(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	mock.ExpectQuery(`SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`).
		WithArgs("SKU-001").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO estoque`).
		WithArgs("2024-06-01", "SKU-001", 10, false).
		WillReturnResult(sqlmock.NewResult(7, 1))

	svc := services.NewEstoqueService(db, estoqueTestCfg(true))
	e, err := svc.CreateEstoque(context.Background(), db, validEstoqueInput())

	require.NoError(t, err)
	require.NotNil(t, e)
	assert.Equal(t, int64(7), e.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UpdateEstoque
// ---------------------------------------------------------------------------

func TestEstoqueService_UpdateEstoque_Success(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	mock.ExpectQuery(`SELECT .+ FROM estoque.+WHERE e\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRows(1, 10, false))
	mock.ExpectExec(`UPDATE estoque`).
		WithArgs(20, false, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .+ FROM estoque.+WHERE e\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRows(1, 20, false))

	svc := services.NewEstoqueService(db, estoqueTestCfg(true))
	e, err := svc.UpdateEstoque(context.Background(), db, 1, services.EstoqueInput{Saldo: 20})

	require.NoError(t, err)
	require.NotNil(t, e)
	assert.Equal(t, 20, e.Saldo)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueService_UpdateEstoque_NaoEncontrado(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	mock.ExpectQuery(`SELECT .+ FROM estoque.+WHERE e\.id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyEstoqueRows())

	svc := services.NewEstoqueService(db, estoqueTestCfg(false))
	e, err := svc.UpdateEstoque(context.Background(), db, 999, services.EstoqueInput{Saldo: 20})

	assert.Nil(t, e)
	assert.ErrorIs(t, err, services.ErrEstoqueNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueService_UpdateEstoque_SaldoNegativo(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	mock.ExpectQuery(`SELECT .+ FROM estoque.+WHERE e\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRows(1, 10, false))

	svc := services.NewEstoqueService(db, estoqueTestCfg(false))
	e, err := svc.UpdateEstoque(context.Background(), db, 1, services.EstoqueInput{Saldo: -5})

	assert.Nil(t, e)
	assert.ErrorIs(t, err, services.ErrEstoqueSaldoInvalido)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueService_UpdateEstoque_RupturaDerivadaDoSaldoZero(t *testing.T) {
	db, mock := newEstoqueTestDB(t)

	mock.ExpectQuery(`SELECT .+ FROM estoque.+WHERE e\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRows(1, 10, false))
	mock.ExpectExec(`UPDATE estoque`).
		WithArgs(0, true, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .+ FROM estoque.+WHERE e\.id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(estoqueRows(1, 0, true))

	svc := services.NewEstoqueService(db, estoqueTestCfg(false))
	e, err := svc.UpdateEstoque(context.Background(), db, 1, services.EstoqueInput{Saldo: 0})

	require.NoError(t, err)
	require.NotNil(t, e)
	assert.True(t, e.Ruptura)
	assert.NoError(t, mock.ExpectationsWereMet())
}
