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
		filtro  services.VisitaFiltro
		page    int
		limit   int
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
		wantTot int
	}{
		{
			nome:   "sem filtros - sucesso",
			filtro: services.VisitaFiltro{},
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
			filtro: services.VisitaFiltro{ClienteID: 100, VendedorID: 1, Resultado: "Positiva"},
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
			filtro: services.VisitaFiltro{},
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

			svc := services.NewVisitaService(db, visitaTestCfg(true))
			visitas, total, err := svc.ListVisitas(context.Background(), db, tc.page, tc.limit, tc.filtro)

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

		svc := services.NewVisitaService(db, visitaTestCfg(false))
		v, err := svc.GetVisitaByID(context.Background(), db, 1)
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

		svc := services.NewVisitaService(db, visitaTestCfg(false))
		v, err := svc.GetVisitaByID(context.Background(), db, 999)
		assert.Nil(t, v)
		assert.ErrorIs(t, err, services.ErrVisitaNaoEncontrada)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro genérico do repo é propagado", func(t *testing.T) {
		db, mock := newVisitaTestDB(t)
		mock.ExpectQuery(`SELECT ` + visitaColunasRegex + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewVisitaService(db, visitaTestCfg(false))
		v, err := svc.GetVisitaByID(context.Background(), db, 1)
		assert.Nil(t, v)
		assert.Error(t, err)
		assert.False(t, err == services.ErrVisitaNaoEncontrada)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// CreateVisita
// ---------------------------------------------------------------------------

func validVisitaInput() services.VisitaInput {
	return services.VisitaInput{
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
		input     func() services.VisitaInput
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
			input: func() services.VisitaInput {
				in := validVisitaInput()
				in.ClienteID = 0
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrVisitaClienteInvalido,
			wantErrIs: true,
		},
		{
			nome: "vendedor_id ausente",
			input: func() services.VisitaInput {
				in := validVisitaInput()
				in.VendedorID = 0
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrVisitaVendedorInvalido,
			wantErrIs: true,
		},
		{
			nome: "resultado vazio",
			input: func() services.VisitaInput {
				in := validVisitaInput()
				in.Resultado = "  "
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrVisitaResultadoInvalido,
			wantErrIs: true,
		},
		{
			nome: "duracao_min negativa",
			input: func() services.VisitaInput {
				in := validVisitaInput()
				in.DuracaoMin = -1
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrVisitaDuracaoInvalida,
			wantErrIs: true,
		},
		{
			nome: "duracao_min zero é válida - sucesso",
			input: func() services.VisitaInput {
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
			input: func() services.VisitaInput {
				in := validVisitaInput()
				in.DataVisita = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrVisitaDataInvalida,
			wantErrIs: true,
		},
		{
			nome: "data_visita em formato inválido",
			input: func() services.VisitaInput {
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
			wantErr:   services.ErrVisitaDataInvalida,
			wantErrIs: true,
		},
		{
			nome: "cliente inexistente",
			input: func() services.VisitaInput {
				return validVisitaInput()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr:   services.ErrVisitaClienteInvalido,
			wantErrIs: true,
		},
		{
			nome: "vendedor inexistente",
			input: func() services.VisitaInput {
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
			wantErr:   services.ErrVisitaVendedorInvalido,
			wantErrIs: true,
		},
		{
			nome: "erro do repo ao checar cliente é propagado",
			input: func() services.VisitaInput {
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

			svc := services.NewVisitaService(db, visitaTestCfg(true))
			v, err := svc.CreateVisita(context.Background(), db, tc.input())

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
		input     func() services.VisitaInput
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
			input: func() services.VisitaInput {
				in := validVisitaInput()
				in.DataVisita = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrVisitaDataInvalida,
			wantErrIs: true,
		},
		{
			nome: "id inexistente retorna ErrVisitaNaoEncontrada",
			id:   999,
			input: func() services.VisitaInput {
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
			wantErr:   services.ErrVisitaNaoEncontrada,
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
			tc.mock(mock)

			svc := services.NewVisitaService(db, visitaTestCfg(true))
			v, err := svc.UpdateVisita(context.Background(), db, tc.id, tc.input())

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
