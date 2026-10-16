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

	"github.com/rotaperfumes/shared/comercial"
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
		filtro  comercial.FiltroOportunidade
		page    int
		limit   int
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
		wantTot int
	}{
		{
			nome:   "sem filtros - sucesso",
			filtro: comercial.FiltroOportunidade{Ordenacao: comercial.OrdenacaoOportunidadePadrao()},
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
			filtro: comercial.FiltroOportunidade{ClienteID: 100, VendedorID: 1, Etapa: "Prospeccao", Ordenacao: comercial.OrdenacaoOportunidadePadrao()},
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
			filtro: comercial.FiltroOportunidade{Ordenacao: comercial.OrdenacaoOportunidadePadrao()},
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

			casos := casosComercial(db, 0)
			oportunidades, total, err := itensETotal(casos.ListarOportunidades(context.Background(), principalAdmin(), tc.filtro, kernel.NovaPagina(tc.page, tc.limit)))

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

		casos := casosComercial(db, 0)
		o, err := casos.ObterOportunidade(context.Background(), principalAdmin(), 1)
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

		casos := casosComercial(db, 0)
		o, err := casos.ObterOportunidade(context.Background(), principalAdmin(), 999)
		assert.Nil(t, o)
		assert.ErrorIs(t, err, comercial.ErrOportunidadeNaoEncontrada)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro genérico do repo é propagado", func(t *testing.T) {
		db, mock := newOportunidadeTestDB(t)
		mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegex + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnError(sql.ErrConnDone)

		casos := casosComercial(db, 0)
		o, err := casos.ObterOportunidade(context.Background(), principalAdmin(), 1)
		assert.Nil(t, o)
		assert.Error(t, err)
		assert.False(t, err == comercial.ErrOportunidadeNaoEncontrada)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// CreateOportunidade
// ---------------------------------------------------------------------------

func validOportunidadeInput() comercial.DadosOportunidade {
	return comercial.DadosOportunidade{
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
		input     func() comercial.DadosOportunidade
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
			input: func() comercial.DadosOportunidade {
				in := validOportunidadeInput()
				in.ClienteID = 0
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrClienteInvalido,
			wantErrIs: true,
		},
		{
			nome: "vendedor_id ausente",
			input: func() comercial.DadosOportunidade {
				in := validOportunidadeInput()
				in.VendedorID = 0
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrVendedorInvalido,
			wantErrIs: true,
		},
		{
			nome: "origem vazia",
			input: func() comercial.DadosOportunidade {
				in := validOportunidadeInput()
				in.Origem = "  "
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrOrigemObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "etapa vazia",
			input: func() comercial.DadosOportunidade {
				in := validOportunidadeInput()
				in.Etapa = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrEtapaObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "probabilidade abaixo de 0",
			input: func() comercial.DadosOportunidade {
				in := validOportunidadeInput()
				in.ProbabilidadePct = -1
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrProbabilidadeInvalida,
			wantErrIs: true,
		},
		{
			nome: "probabilidade acima de 100",
			input: func() comercial.DadosOportunidade {
				in := validOportunidadeInput()
				in.ProbabilidadePct = 101
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrProbabilidadeInvalida,
			wantErrIs: true,
		},
		{
			nome: "valor_estimado negativo",
			input: func() comercial.DadosOportunidade {
				in := validOportunidadeInput()
				in.ValorEstimado = -100
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrValorEstimadoInvalido,
			wantErrIs: true,
		},
		{
			nome: "etapa Fechado perdido sem motivo_perda",
			input: func() comercial.DadosOportunidade {
				in := validOportunidadeInput()
				in.Etapa = "Fechado perdido"
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrMotivoPerdaObrigatorio,
			wantErrIs: true,
		},
		{
			nome: "etapa Fechado perdido com motivo_perda - sucesso",
			input: func() comercial.DadosOportunidade {
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
			input: func() comercial.DadosOportunidade {
				return validOportunidadeInput()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr:   comercial.ErrClienteInvalido,
			wantErrIs: true,
		},
		{
			nome: "vendedor inexistente",
			input: func() comercial.DadosOportunidade {
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
			wantErr:   comercial.ErrVendedorInvalido,
			wantErrIs: true,
		},
		{
			nome: "data_abertura inválida",
			input: func() comercial.DadosOportunidade {
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
			wantErr:   comercial.ErrDataAberturaInvalida,
			wantErrIs: true,
		},
		{
			nome: "data_abertura vazia usa hoje (sucesso na criação)",
			input: func() comercial.DadosOportunidade {
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
			input: func() comercial.DadosOportunidade {
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
			wantErr:   comercial.ErrDataFechamentoInvalida,
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

			casos := casosComercial(db, 0)
			o, err := casos.CriarOportunidade(context.Background(), principalAdmin(), fixo(tc.input()))

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
		input     func() comercial.DadosOportunidade
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
			input: func() comercial.DadosOportunidade {
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
			wantErr:   comercial.ErrDataAberturaInvalida,
			wantErrIs: true,
		},
		{
			nome: "id inexistente retorna ErrOportunidadeNaoEncontrada",
			input: func() comercial.DadosOportunidade {
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
			wantErr:   comercial.ErrOportunidadeNaoEncontrada,
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

			id := int64(1)
			if tc.nome == "id inexistente retorna ErrOportunidadeNaoEncontrada" {
				id = 999
			}
			esperarPosseOportunidade(mock, id)
			tc.mock(mock)

			casos := casosComercial(db, 0)
			o, err := casos.AtualizarOportunidade(context.Background(), principalAdmin(), id, fixo(tc.input()))

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
	esperarPosseOportunidade(mock, 1)
	mock.ExpectExec(deleteOportunidadeRegex).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	casos := casosComercial(db, 0)
	err := casos.ExcluirOportunidade(context.Background(), principalAdmin(), 1)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeService_DeleteOportunidade_NaoEncontrada(t *testing.T) {
	db, mock := newOportunidadeTestDB(t)
	esperarPosseOportunidade(mock, 999)
	mock.ExpectExec(deleteOportunidadeRegex).
		WithArgs(int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	casos := casosComercial(db, 0)
	err := casos.ExcluirOportunidade(context.Background(), principalAdmin(), 999)
	assert.ErrorIs(t, err, comercial.ErrOportunidadeNaoEncontrada)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeService_DeleteOportunidade_ErroRepo(t *testing.T) {
	db, mock := newOportunidadeTestDB(t)
	esperarPosseOportunidade(mock, 1)
	mock.ExpectExec(deleteOportunidadeRegex).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	casos := casosComercial(db, 0)
	err := casos.ExcluirOportunidade(context.Background(), principalAdmin(), 1)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
