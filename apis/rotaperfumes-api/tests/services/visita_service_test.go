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

func visitaTestCfg(verbose bool) *config.Config {
	return &config.Config{Verbose: verbose}
}

func newVisitaTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

const visitaColunasRegex = `visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min, created_at, updated_at`

func visitaRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"visita_id", "cliente_id", "vendedor_id", "data_visita", "resultado", "duracao_min",
		"created_at", "updated_at",
	}).AddRow(int64(1), int64(100), int64(1), now, "Positiva", 30, now, now)
}

func visitaClienteExistsRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"1"}).AddRow(1)
}

func visitaVendedorExistsRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"1"}).AddRow(1)
}

// ---------------------------------------------------------------------------
// ListVisitas
// ---------------------------------------------------------------------------

func TestVisitaService_ListVisitas(t *testing.T) {
	testCases := []struct {
		nome    string
		filtro  comercial.FiltroVisita
		page    int
		limit   int
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
		wantTot int
	}{
		{
			nome:   "sem filtros - sucesso",
			filtro: comercial.FiltroVisita{Ordenacao: comercial.OrdenacaoVisitaPadrao()},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT `+visitaColunasRegex+` FROM visitas ORDER BY visita_id ASC LIMIT \? OFFSET \?`).
					WithArgs(20, 0).
					WillReturnRows(visitaRows())
			},
			wantLen: 1,
			wantTot: 1,
		},
		{
			nome:   "com filtros cliente/vendedor/resultado - repassa args ao repo",
			filtro: comercial.FiltroVisita{ClienteID: 100, VendedorID: 1, Resultado: "Positiva", Ordenacao: comercial.OrdenacaoVisitaPadrao()},
			page:   2,
			limit:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas WHERE cliente_id = \? AND vendedor_id = \? AND resultado = \?`).
					WithArgs(int64(100), int64(1), "Positiva").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
				mock.ExpectQuery(`SELECT `+visitaColunasRegex+` FROM visitas WHERE cliente_id = \? AND vendedor_id = \? AND resultado = \? ORDER BY visita_id ASC LIMIT \? OFFSET \?`).
					WithArgs(int64(100), int64(1), "Positiva", 10, 10).
					WillReturnRows(visitaRows())
			},
			wantLen: 1,
			wantTot: 5,
		},
		{
			nome:   "erro no count do repo é propagado",
			filtro: comercial.FiltroVisita{Ordenacao: comercial.OrdenacaoVisitaPadrao()},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVisitaTestDB(t)
			tc.mock(mock)

			casos := casosComercial(db, 0)
			visitas, total, err := itensETotal(casos.ListarVisitas(context.Background(), principalAdmin(), tc.filtro, kernel.NovaPagina(tc.page, tc.limit)))

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, visitas, tc.wantLen)
				assert.Equal(t, tc.wantTot, total)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetVisitaByID
// ---------------------------------------------------------------------------

func TestVisitaService_GetVisitaByID(t *testing.T) {
	t.Run("encontrada", func(t *testing.T) {
		db, mock := newVisitaTestDB(t)
		mock.ExpectQuery(`SELECT ` + visitaColunasRegex + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(visitaRows())

		casos := casosComercial(db, 0)
		v, err := casos.ObterVisita(context.Background(), principalAdmin(), 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), v.VisitaID)
		assert.Equal(t, "Positiva", v.Resultado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("não encontrada retorna ErrVisitaNaoEncontrada", func(t *testing.T) {
		db, mock := newVisitaTestDB(t)
		mock.ExpectQuery(`SELECT ` + visitaColunasRegex + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
			WithArgs(int64(999)).
			WillReturnError(sql.ErrNoRows)

		casos := casosComercial(db, 0)
		v, err := casos.ObterVisita(context.Background(), principalAdmin(), 999)
		assert.Nil(t, v)
		assert.ErrorIs(t, err, comercial.ErrVisitaNaoEncontrada)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro genérico do repo é propagado", func(t *testing.T) {
		db, mock := newVisitaTestDB(t)
		mock.ExpectQuery(`SELECT ` + visitaColunasRegex + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnError(sql.ErrConnDone)

		casos := casosComercial(db, 0)
		v, err := casos.ObterVisita(context.Background(), principalAdmin(), 1)
		assert.Nil(t, v)
		assert.Error(t, err)
		assert.False(t, err == comercial.ErrVisitaNaoEncontrada)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// CreateVisita
// ---------------------------------------------------------------------------

func validVisitaInput() comercial.DadosVisita {
	return comercial.DadosVisita{
		ClienteID:  100,
		VendedorID: 1,
		DataVisita: "2024-01-15",
		Resultado:  "Positiva",
		DuracaoMin: 30,
	}
}

func TestVisitaService_CreateVisita(t *testing.T) {
	testCases := []struct {
		nome      string
		input     func() comercial.DadosVisita
		mock      func(mock sqlmock.Sqlmock)
		wantErr   error
		wantErrIs bool
	}{
		{
			nome:  "sucesso",
			input: validVisitaInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(visitaClienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(visitaVendedorExistsRows())
				mock.ExpectExec(`INSERT INTO visitas`).
					WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30).
					WillReturnResult(sqlmock.NewResult(9, 1))
			},
		},
		{
			nome: "cliente_id ausente",
			input: func() comercial.DadosVisita {
				in := validVisitaInput()
				in.ClienteID = 0
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrClienteInvalido,
			wantErrIs: true,
		},
		{
			nome: "vendedor_id ausente",
			input: func() comercial.DadosVisita {
				in := validVisitaInput()
				in.VendedorID = 0
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrVendedorInvalido,
			wantErrIs: true,
		},
		{
			nome: "resultado vazio",
			input: func() comercial.DadosVisita {
				in := validVisitaInput()
				in.Resultado = "  "
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrResultadoObrigatorio,
			wantErrIs: true,
		},
		{
			nome: "duracao_min negativa",
			input: func() comercial.DadosVisita {
				in := validVisitaInput()
				in.DuracaoMin = -1
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrDuracaoInvalida,
			wantErrIs: true,
		},
		{
			nome: "duracao_min zero é válida - sucesso",
			input: func() comercial.DadosVisita {
				in := validVisitaInput()
				in.DuracaoMin = 0
				return in
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(visitaClienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(visitaVendedorExistsRows())
				mock.ExpectExec(`INSERT INTO visitas`).
					WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 0).
					WillReturnResult(sqlmock.NewResult(9, 1))
			},
		},
		{
			nome: "data_visita vazia é erro (sem default)",
			input: func() comercial.DadosVisita {
				in := validVisitaInput()
				in.DataVisita = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrDataVisitaInvalida,
			wantErrIs: true,
		},
		{
			nome: "data_visita em formato inválido",
			input: func() comercial.DadosVisita {
				in := validVisitaInput()
				in.DataVisita = "15/01/2024"
				return in
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(visitaClienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(visitaVendedorExistsRows())
			},
			wantErr:   comercial.ErrDataVisitaInvalida,
			wantErrIs: true,
		},
		{
			nome: "cliente inexistente",
			input: func() comercial.DadosVisita {
				return validVisitaInput()
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
			input: func() comercial.DadosVisita {
				return validVisitaInput()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(visitaClienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr:   comercial.ErrVendedorInvalido,
			wantErrIs: true,
		},
		{
			nome: "erro do repo ao checar cliente é propagado",
			input: func() comercial.DadosVisita {
				return validVisitaInput()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
		{
			nome:  "erro no Create é propagado",
			input: validVisitaInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(visitaClienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(visitaVendedorExistsRows())
				mock.ExpectExec(`INSERT INTO visitas`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVisitaTestDB(t)
			tc.mock(mock)

			casos := casosComercial(db, 0)
			v, err := casos.CriarVisita(context.Background(), principalAdmin(), fixo(tc.input()))

			if tc.wantErr != nil {
				assert.Nil(t, v)
				if tc.wantErrIs {
					assert.ErrorIs(t, err, tc.wantErr)
				} else {
					assert.Error(t, err)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, v)
				assert.Equal(t, int64(9), v.VisitaID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateVisita
// ---------------------------------------------------------------------------

func TestVisitaService_UpdateVisita(t *testing.T) {
	testCases := []struct {
		nome      string
		id        int64
		input     func() comercial.DadosVisita
		mock      func(mock sqlmock.Sqlmock)
		wantErr   error
		wantErrIs bool
	}{
		{
			nome:  "sucesso",
			id:    1,
			input: validVisitaInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(visitaClienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(visitaVendedorExistsRows())
				mock.ExpectExec(`UPDATE visitas`).
					WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30, int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(`SELECT ` + visitaColunasRegex + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(visitaRows())
			},
		},
		{
			nome: "data_visita vazia é erro na edição (sem default)",
			id:   1,
			input: func() comercial.DadosVisita {
				in := validVisitaInput()
				in.DataVisita = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   comercial.ErrDataVisitaInvalida,
			wantErrIs: true,
		},
		{
			nome: "id inexistente retorna ErrVisitaNaoEncontrada",
			id:   999,
			input: func() comercial.DadosVisita {
				return validVisitaInput()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(visitaClienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(visitaVendedorExistsRows())
				mock.ExpectExec(`UPDATE visitas`).
					WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30, int64(999)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr:   comercial.ErrVisitaNaoEncontrada,
			wantErrIs: true,
		},
		{
			nome:  "erro no Update é propagado",
			id:    1,
			input: validVisitaInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(visitaClienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(visitaVendedorExistsRows())
				mock.ExpectExec(`UPDATE visitas`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
		{
			nome:  "erro no GetByID pós-update é propagado",
			id:    1,
			input: validVisitaInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(visitaClienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(visitaVendedorExistsRows())
				mock.ExpectExec(`UPDATE visitas`).
					WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30, int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(`SELECT ` + visitaColunasRegex + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVisitaTestDB(t)
			esperarPosseVisita(mock, tc.id)
			tc.mock(mock)

			casos := casosComercial(db, 0)
			v, err := casos.AtualizarVisita(context.Background(), principalAdmin(), tc.id, fixo(tc.input()))

			if tc.wantErr != nil {
				assert.Nil(t, v)
				if tc.wantErrIs {
					assert.ErrorIs(t, err, tc.wantErr)
				} else {
					assert.Error(t, err)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, v)
				assert.Equal(t, int64(1), v.VisitaID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteVisita
// ---------------------------------------------------------------------------

const deleteVisitaRegex = `DELETE FROM visitas WHERE visita_id = \?`

func TestVisitaService_DeleteVisita_Sucesso(t *testing.T) {
	db, mock := newVisitaTestDB(t)
	esperarPosseVisita(mock, 1)
	mock.ExpectExec(deleteVisitaRegex).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	casos := casosComercial(db, 0)
	err := casos.ExcluirVisita(context.Background(), principalAdmin(), 1)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaService_DeleteVisita_NaoEncontrada(t *testing.T) {
	db, mock := newVisitaTestDB(t)
	esperarPosseVisita(mock, 999)
	mock.ExpectExec(deleteVisitaRegex).
		WithArgs(int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	casos := casosComercial(db, 0)
	err := casos.ExcluirVisita(context.Background(), principalAdmin(), 999)
	assert.ErrorIs(t, err, comercial.ErrVisitaNaoEncontrada)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaService_DeleteVisita_ErroRepo(t *testing.T) {
	db, mock := newVisitaTestDB(t)
	esperarPosseVisita(mock, 1)
	mock.ExpectExec(deleteVisitaRegex).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	casos := casosComercial(db, 0)
	err := casos.ExcluirVisita(context.Background(), principalAdmin(), 1)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
