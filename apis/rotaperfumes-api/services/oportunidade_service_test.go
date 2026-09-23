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

func oportunidadeTestCfg(verbose bool) *config.Config {
	return &config.Config{Verbose: verbose}
}

func newOportunidadeTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

const oportunidadeColunasRegex = `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda, created_at, updated_at`

func oportunidadeRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"oportunidade_id", "cliente_id", "vendedor_id", "origem", "data_abertura", "etapa",
		"probabilidade_pct", "valor_estimado", "data_fechamento", "ciclo_dias", "motivo_perda",
		"created_at", "updated_at",
	}).AddRow(int64(1), int64(100), int64(1), "Site", now, "Prospeccao", 10.0, 1000.0, nil, nil, nil, now, now)
}

func clienteExistsRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"1"}).AddRow(1)
}

func vendedorExistsRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"1"}).AddRow(1)
}

// ---------------------------------------------------------------------------
// ListOportunidades
// ---------------------------------------------------------------------------

func TestOportunidadeService_ListOportunidades(t *testing.T) {
	testCases := []struct {
		nome    string
		filtro  services.OportunidadeFiltro
		page    int
		limit   int
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
		wantTot int
	}{
		{
			nome:   "sem filtros - sucesso",
			filtro: services.OportunidadeFiltro{},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT `+oportunidadeColunasRegex+` FROM oportunidades ORDER BY oportunidade_id ASC LIMIT \? OFFSET \?`).
					WithArgs(20, 0).
					WillReturnRows(oportunidadeRows())
			},
			wantLen: 1,
			wantTot: 1,
		},
		{
			nome:   "com filtros cliente/vendedor/etapa - repassa args ao repo",
			filtro: services.OportunidadeFiltro{ClienteID: 100, VendedorID: 1, Etapa: "Prospeccao"},
			page:   2,
			limit:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades WHERE cliente_id = \? AND vendedor_id = \? AND etapa = \?`).
					WithArgs(int64(100), int64(1), "Prospeccao").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
				mock.ExpectQuery(`SELECT `+oportunidadeColunasRegex+` FROM oportunidades WHERE cliente_id = \? AND vendedor_id = \? AND etapa = \? ORDER BY oportunidade_id ASC LIMIT \? OFFSET \?`).
					WithArgs(int64(100), int64(1), "Prospeccao", 10, 10).
					WillReturnRows(oportunidadeRows())
			},
			wantLen: 1,
			wantTot: 5,
		},
		{
			nome:   "erro no count do repo é propagado",
			filtro: services.OportunidadeFiltro{},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newOportunidadeTestDB(t)
			tc.mock(mock)

			svc := services.NewOportunidadeService(db, oportunidadeTestCfg(true))
			oportunidades, total, err := svc.ListOportunidades(context.Background(), db, tc.page, tc.limit, tc.filtro)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, oportunidades, tc.wantLen)
				assert.Equal(t, tc.wantTot, total)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetOportunidadeByID
// ---------------------------------------------------------------------------

func TestOportunidadeService_GetOportunidadeByID(t *testing.T) {
	t.Run("encontrada", func(t *testing.T) {
		db, mock := newOportunidadeTestDB(t)
		mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegex + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(oportunidadeRows())

		svc := services.NewOportunidadeService(db, oportunidadeTestCfg(false))
		o, err := svc.GetOportunidadeByID(context.Background(), db, 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), o.OportunidadeID)
		assert.Equal(t, "Prospeccao", o.Etapa)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("não encontrada retorna ErrOportunidadeNaoEncontrada", func(t *testing.T) {
		db, mock := newOportunidadeTestDB(t)
		mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegex + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
			WithArgs(int64(999)).
			WillReturnError(sql.ErrNoRows)

		svc := services.NewOportunidadeService(db, oportunidadeTestCfg(false))
		o, err := svc.GetOportunidadeByID(context.Background(), db, 999)
		assert.Nil(t, o)
		assert.ErrorIs(t, err, services.ErrOportunidadeNaoEncontrada)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro genérico do repo é propagado", func(t *testing.T) {
		db, mock := newOportunidadeTestDB(t)
		mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegex + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewOportunidadeService(db, oportunidadeTestCfg(false))
		o, err := svc.GetOportunidadeByID(context.Background(), db, 1)
		assert.Nil(t, o)
		assert.Error(t, err)
		assert.False(t, err == services.ErrOportunidadeNaoEncontrada)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// CreateOportunidade
// ---------------------------------------------------------------------------

func validOportunidadeInput() services.OportunidadeInput {
	return services.OportunidadeInput{
		ClienteID:        100,
		VendedorID:       1,
		Origem:           "Site",
		DataAbertura:     "2024-01-15",
		Etapa:            "Prospeccao",
		ProbabilidadePct: 10,
		ValorEstimado:    1000,
	}
}

func TestOportunidadeService_CreateOportunidade(t *testing.T) {
	testCases := []struct {
		nome      string
		input     func() services.OportunidadeInput
		mock      func(mock sqlmock.Sqlmock)
		wantErr   error
		wantErrIs bool
	}{
		{
			nome:  "sucesso",
			input: validOportunidadeInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
				mock.ExpectExec(`INSERT INTO oportunidades`).
					WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(9, 1))
			},
		},
		{
			nome: "cliente_id ausente",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.ClienteID = 0
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrOportunidadeClienteInvalido,
			wantErrIs: true,
		},
		{
			nome: "vendedor_id ausente",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.VendedorID = 0
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrOportunidadeVendedorInvalido,
			wantErrIs: true,
		},
		{
			nome: "origem vazia",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.Origem = "  "
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrOportunidadeOrigemInvalida,
			wantErrIs: true,
		},
		{
			nome: "etapa vazia",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.Etapa = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrOportunidadeEtapaInvalida,
			wantErrIs: true,
		},
		{
			nome: "probabilidade abaixo de 0",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.ProbabilidadePct = -1
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrOportunidadeProbabilidadeInvalida,
			wantErrIs: true,
		},
		{
			nome: "probabilidade acima de 100",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.ProbabilidadePct = 101
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrOportunidadeProbabilidadeInvalida,
			wantErrIs: true,
		},
		{
			nome: "valor_estimado negativo",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.ValorEstimado = -100
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrOportunidadeValorEstimadoInvalido,
			wantErrIs: true,
		},
		{
			nome: "etapa Fechado perdido sem motivo_perda",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.Etapa = "Fechado perdido"
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrOportunidadeMotivoPerdaObrigatorio,
			wantErrIs: true,
		},
		{
			nome: "etapa Fechado perdido com motivo_perda - sucesso",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.Etapa = "Fechado perdido"
				in.MotivoPerda = "Preço"
				return in
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
				mock.ExpectExec(`INSERT INTO oportunidades`).
					WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Fechado perdido", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(9, 1))
			},
		},
		{
			nome: "cliente inexistente",
			input: func() services.OportunidadeInput {
				return validOportunidadeInput()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr:   services.ErrOportunidadeClienteInvalido,
			wantErrIs: true,
		},
		{
			nome: "vendedor inexistente",
			input: func() services.OportunidadeInput {
				return validOportunidadeInput()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr:   services.ErrOportunidadeVendedorInvalido,
			wantErrIs: true,
		},
		{
			nome: "data_abertura inválida",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.DataAbertura = "15/01/2024"
				return in
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
			},
			wantErr:   services.ErrOportunidadeDataAberturaInvalida,
			wantErrIs: true,
		},
		{
			nome: "data_abertura vazia usa hoje (sucesso na criação)",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.DataAbertura = ""
				return in
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
				mock.ExpectExec(`INSERT INTO oportunidades`).
					WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(9, 1))
			},
		},
		{
			nome: "data_fechamento inválida",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.DataFechamento = "31/12/2024"
				return in
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
			},
			wantErr:   services.ErrOportunidadeDataFechamentoInvalida,
			wantErrIs: true,
		},
		{
			nome:  "erro no Create é propagado",
			input: validOportunidadeInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
				mock.ExpectExec(`INSERT INTO oportunidades`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newOportunidadeTestDB(t)
			tc.mock(mock)

			svc := services.NewOportunidadeService(db, oportunidadeTestCfg(true))
			o, err := svc.CreateOportunidade(context.Background(), db, tc.input())

			if tc.wantErr != nil {
				assert.Nil(t, o)
				if tc.wantErrIs {
					assert.ErrorIs(t, err, tc.wantErr)
				} else {
					assert.Error(t, err)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, o)
				assert.Equal(t, int64(9), o.OportunidadeID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateOportunidade
// ---------------------------------------------------------------------------

func TestOportunidadeService_UpdateOportunidade(t *testing.T) {
	testCases := []struct {
		nome      string
		input     func() services.OportunidadeInput
		mock      func(mock sqlmock.Sqlmock)
		wantErr   error
		wantErrIs bool
	}{
		{
			nome:  "sucesso",
			input: validOportunidadeInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
				mock.ExpectExec(`UPDATE oportunidades`).
					WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegex + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(oportunidadeRows())
			},
		},
		{
			nome: "data_abertura vazia é erro na edição (defaultHoje=false)",
			input: func() services.OportunidadeInput {
				in := validOportunidadeInput()
				in.DataAbertura = ""
				return in
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
			},
			wantErr:   services.ErrOportunidadeDataAberturaInvalida,
			wantErrIs: true,
		},
		{
			nome: "id inexistente retorna ErrOportunidadeNaoEncontrada",
			input: func() services.OportunidadeInput {
				return validOportunidadeInput()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
				mock.ExpectExec(`UPDATE oportunidades`).
					WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(999)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr:   services.ErrOportunidadeNaoEncontrada,
			wantErrIs: true,
		},
		{
			nome:  "erro no Update é propagado",
			input: validOportunidadeInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
				mock.ExpectExec(`UPDATE oportunidades`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
		{
			nome:  "erro no GetByID pós-update é propagado",
			input: validOportunidadeInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
				mock.ExpectExec(`UPDATE oportunidades`).
					WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegex + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newOportunidadeTestDB(t)
			tc.mock(mock)

			id := int64(1)
			if tc.nome == "id inexistente retorna ErrOportunidadeNaoEncontrada" {
				id = 999
			}

			svc := services.NewOportunidadeService(db, oportunidadeTestCfg(true))
			o, err := svc.UpdateOportunidade(context.Background(), db, id, tc.input())

			if tc.wantErr != nil {
				assert.Nil(t, o)
				if tc.wantErrIs {
					assert.ErrorIs(t, err, tc.wantErr)
				} else {
					assert.Error(t, err)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, o)
				assert.Equal(t, int64(1), o.OportunidadeID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteOportunidade
// ---------------------------------------------------------------------------

const deleteOportunidadeRegex = `DELETE FROM oportunidades WHERE oportunidade_id = \?`

func TestOportunidadeService_DeleteOportunidade_Sucesso(t *testing.T) {
	db, mock := newOportunidadeTestDB(t)
	mock.ExpectExec(deleteOportunidadeRegex).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	svc := services.NewOportunidadeService(db, oportunidadeTestCfg(true))
	err := svc.DeleteOportunidade(context.Background(), db, 1)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeService_DeleteOportunidade_NaoEncontrada(t *testing.T) {
	db, mock := newOportunidadeTestDB(t)
	mock.ExpectExec(deleteOportunidadeRegex).
		WithArgs(int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	svc := services.NewOportunidadeService(db, oportunidadeTestCfg(false))
	err := svc.DeleteOportunidade(context.Background(), db, 999)
	assert.ErrorIs(t, err, services.ErrOportunidadeNaoEncontrada)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeService_DeleteOportunidade_ErroRepo(t *testing.T) {
	db, mock := newOportunidadeTestDB(t)
	mock.ExpectExec(deleteOportunidadeRegex).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	svc := services.NewOportunidadeService(db, oportunidadeTestCfg(false))
	err := svc.DeleteOportunidade(context.Background(), db, 1)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
