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

var clienteResumoColumns = []string{
	"cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "carteira_id_origem", "data_inicio", "data_fim",
}

func clienteResumoRow(id int64, cnpj, razaoSocial, segmento, cidade, uf string, carteiraID int64, dataInicio time.Time, dataFim *time.Time) []driver.Value {
	return []driver.Value{id, cnpj, razaoSocial, segmento, cidade, uf, carteiraID, dataInicio, dataFim}
}

const clienteResumoFromRegexp = `FROM carteiras ca JOIN clientes c ON c\.cliente_id_origem = ca\.cliente_id`

func TestCarteiraListClientesByVendedorID_ComClientes(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(clienteResumoColumns).
		AddRow(clienteResumoRow(1, "11.111.111/0001-11", "Cliente A", "Varejo", "São Paulo", "SP", 10, now, nil)...).
		AddRow(clienteResumoRow(2, "22.222.222/0001-22", "Cliente B", "Atacado", "Campinas", "SP", 11, now, nil)...)

	mock.ExpectQuery(`SELECT .+ ` + clienteResumoFromRegexp + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	clientes, err := repo.ListClientesByVendedorID(ctx, db, 1)

	require.NoError(t, err)
	require.Len(t, clientes, 2)
	assert.Equal(t, "Cliente A", clientes[0].RazaoSocial)
	assert.Equal(t, int64(10), clientes[0].CarteiraID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraListClientesByVendedorID_Vazio(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ ` + clienteResumoFromRegexp + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows(clienteResumoColumns))

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	clientes, err := repo.ListClientesByVendedorID(ctx, db, 99)

	require.NoError(t, err)
	assert.Len(t, clientes, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraListClientesByVendedorID_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .+ ` + clienteResumoFromRegexp + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	clientes, err := repo.ListClientesByVendedorID(ctx, db, 1)

	assert.Nil(t, clientes)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraListClientesByVendedorID_ScanError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	// Faltando coluna força erro de scan (quantidade de colunas divergente).
	rows := sqlmock.NewRows([]string{"id", "cnpj", "razao_social"}).
		AddRow(1, "11.111.111/0001-11", "Cliente A")

	mock.ExpectQuery(`SELECT .+ ` + clienteResumoFromRegexp + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	clientes, err := repo.ListClientesByVendedorID(ctx, db, 1)

	assert.Nil(t, clientes)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraListClientesByVendedorID_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(clienteResumoColumns).
		AddRow(clienteResumoRow(1, "11.111.111/0001-11", "Cliente A", "Varejo", "São Paulo", "SP", 10, now, nil)...).
		RowError(0, sql.ErrConnDone)

	mock.ExpectQuery(`SELECT .+ ` + clienteResumoFromRegexp + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	clientes, err := repo.ListClientesByVendedorID(ctx, db, 1)

	assert.Nil(t, clientes)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

var carteiraColumns = []string{
	"carteira_id_origem", "cliente_id", "vendedor_id", "data_inicio", "data_fim", "created_at", "updated_at",
}

func TestCarteiraGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(carteiraColumns).
		AddRow(500, 10, 1, now, nil, now, now)

	mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE carteira_id_origem = \?\s+LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	c, err := repo.GetByID(ctx, db, 1)

	require.NoError(t, err)
	assert.Equal(t, int64(500), c.CarteiraIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE carteira_id_origem = \?\s+LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	c, err := repo.GetByID(ctx, db, 999)

	assert.Nil(t, c)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE carteira_id_origem = \?\s+LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	c, err := repo.GetByID(ctx, db, 1)

	assert.Nil(t, c)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetVinculoAtivoByClienteID
// ---------------------------------------------------------------------------

func TestCarteiraGetVinculoAtivoByClienteID(t *testing.T) {
	now := time.Now()

	testes := []struct {
		nome      string
		clienteID int64
		mock      func(mock sqlmock.Sqlmock)
		wantErr   error
		wantID    int64
	}{
		{
			nome:      "vínculo ativo encontrado",
			clienteID: 10,
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(carteiraColumns).AddRow(500, 10, 1, now, nil, now, now)
				mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
					WithArgs(int64(10)).
					WillReturnRows(rows)
			},
			wantID: 500,
		},
		{
			nome:      "não encontrado",
			clienteID: 99,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
					WithArgs(int64(99)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: repositories.ErrNotFound,
		},
		{
			nome:      "erro de query",
			clienteID: 1,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: nil,
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			tt.mock(mock)

			repo := repositories.NewCarteiraRepository()
			ctx := context.Background()
			c, err := repo.GetVinculoAtivoByClienteID(ctx, db, tt.clienteID)

			if tt.wantID != 0 {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, c.CarteiraIDOrigem)
			} else {
				assert.Nil(t, c)
				assert.Error(t, err)
				if tt.wantErr != nil {
					assert.ErrorIs(t, err, tt.wantErr)
				} else {
					assert.NotErrorIs(t, err, repositories.ErrNotFound)
				}
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetVinculoAtivo
// ---------------------------------------------------------------------------

func TestCarteiraGetVinculoAtivo(t *testing.T) {
	now := time.Now()

	testes := []struct {
		nome       string
		vendedorID int64
		clienteID  int64
		mock       func(mock sqlmock.Sqlmock)
		wantErr    error
		wantID     int64
	}{
		{
			nome:       "vínculo ativo encontrado",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(carteiraColumns).AddRow(500, 10, 1, now, nil, now, now)
				mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
					WithArgs(int64(1), int64(10)).
					WillReturnRows(rows)
			},
			wantID: 500,
		},
		{
			nome:       "não encontrado",
			vendedorID: 2,
			clienteID:  99,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
					WithArgs(int64(2), int64(99)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: repositories.ErrNotFound,
		},
		{
			nome:       "erro de query",
			vendedorID: 1,
			clienteID:  1,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
					WithArgs(int64(1), int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: nil,
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			tt.mock(mock)

			repo := repositories.NewCarteiraRepository()
			ctx := context.Background()
			c, err := repo.GetVinculoAtivo(ctx, db, tt.vendedorID, tt.clienteID)

			if tt.wantID != 0 {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, c.CarteiraIDOrigem)
			} else {
				assert.Nil(t, c)
				assert.Error(t, err)
				if tt.wantErr != nil {
					assert.ErrorIs(t, err, tt.wantErr)
				} else {
					assert.NotErrorIs(t, err, repositories.ErrNotFound)
				}
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestCarteiraCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	c := &models.Carteira{
		CarteiraIDOrigem: 500,
		ClienteID:        10,
		VendedorID:       1,
		DataInicio:       now,
	}

	mock.ExpectExec(`INSERT INTO carteiras \(cliente_id, vendedor_id, data_inicio, data_fim\)\s+VALUES \(\?, \?, \?, \?\)`).
		WithArgs(c.ClienteID, c.VendedorID, c.DataInicio, c.DataFim).
		WillReturnResult(sqlmock.NewResult(7, 1))

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, c)

	require.NoError(t, err)
	assert.Equal(t, int64(7), c.CarteiraIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraCreate_ExecError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	c := &models.Carteira{ClienteID: 10, VendedorID: 1, DataInicio: time.Now()}

	mock.ExpectExec(`INSERT INTO carteiras \(cliente_id, vendedor_id, data_inicio, data_fim\)\s+VALUES \(\?, \?, \?, \?\)`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, c)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraCreate_LastInsertIdError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	c := &models.Carteira{ClienteID: 10, VendedorID: 1, DataInicio: time.Now()}

	mock.ExpectExec(`INSERT INTO carteiras \(cliente_id, vendedor_id, data_inicio, data_fim\)\s+VALUES \(\?, \?, \?, \?\)`).
		WillReturnResult(sqlmock.NewErrorResult(sql.ErrConnDone))

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, c)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// EncerrarVinculo
// ---------------------------------------------------------------------------

func TestCarteiraEncerrarVinculo_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	dataFim := time.Now()

	mock.ExpectExec(`UPDATE carteiras SET data_fim = \? WHERE carteira_id_origem = \?`).
		WithArgs(dataFim, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.EncerrarVinculo(ctx, db, 1, dataFim)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraEncerrarVinculo_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	dataFim := time.Now()

	mock.ExpectExec(`UPDATE carteiras SET data_fim = \? WHERE carteira_id_origem = \?`).
		WithArgs(dataFim, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.EncerrarVinculo(ctx, db, 999, dataFim)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraEncerrarVinculo_ExecError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE carteiras SET data_fim = \? WHERE carteira_id_origem = \?`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.EncerrarVinculo(ctx, db, 1, time.Now())

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraEncerrarVinculo_RowsAffectedError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE carteiras SET data_fim = \? WHERE carteira_id_origem = \?`).
		WillReturnResult(sqlmock.NewErrorResult(sql.ErrConnDone))

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.EncerrarVinculo(ctx, db, 1, time.Now())

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestCarteiraDelete_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM carteiras WHERE carteira_id_origem = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.Delete(ctx, db, 1)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraDelete_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM carteiras WHERE carteira_id_origem = \?`).
		WithArgs(int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.Delete(ctx, db, 999)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraDelete_ExecError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM carteiras WHERE carteira_id_origem = \?`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.Delete(ctx, db, 1)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCarteiraDelete_RowsAffectedError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM carteiras WHERE carteira_id_origem = \?`).
		WillReturnResult(sqlmock.NewErrorResult(sql.ErrConnDone))

	repo := repositories.NewCarteiraRepository()
	ctx := context.Background()
	err := repo.Delete(ctx, db, 1)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// NextCarteiraIDOrigem foi removida: carteira_id_origem agora é a PK
// AUTO_INCREMENT da tabela carteiras, e o valor é obtido via LastInsertId()
// no repositório Create (ver TestCarteiraCreate_Success acima).
