package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

var vendedorGetColumns = []string{
	"id", "nome", "regiao", "uf", "data_admissao", "data_desligamento", "meta_mensal", "created_at", "updated_at",
}

func TestVendedorList_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_desligamento"}).
		AddRow(1, "Vendedor A", "Sudeste", "SP", nil).
		AddRow(2, "Vendedor B", "Sul", "RS", nil)

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento FROM vendedores ORDER BY nome ASC`).
		WillReturnRows(rows)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	vendedores, err := repo.List(ctx, db)

	require.NoError(t, err)
	assert.Len(t, vendedores, 2)
	assert.Equal(t, "Vendedor A", vendedores[0].Nome)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestVendedorList_AtivosEInativos cobre o comportamento central da correção
// do bug de vínculo "sumido": List não deve mais filtrar por
// data_desligamento IS NULL — vendedores ativos e inativos devem vir juntos,
// e DataDesligamento deve vir populado (não-nil) para os inativos e nil para
// os ativos, permitindo ao chamador marcá-los (ex: "[inativo]").
func TestVendedorList_AtivosEInativos(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	desligadoEm := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_desligamento"}).
		AddRow(1, "Vendedor Ativo", "Sudeste", "SP", nil).
		AddRow(2, "Vendedor Inativo", "Sul", "RS", desligadoEm)

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento FROM vendedores ORDER BY nome ASC`).
		WillReturnRows(rows)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	vendedores, err := repo.List(ctx, db)

	require.NoError(t, err)
	require.Len(t, vendedores, 2)

	ativo := vendedores[0]
	assert.Equal(t, "Vendedor Ativo", ativo.Nome)
	assert.Nil(t, ativo.DataDesligamento)

	inativo := vendedores[1]
	assert.Equal(t, "Vendedor Inativo", inativo.Nome)
	require.NotNil(t, inativo.DataDesligamento)
	assert.True(t, desligadoEm.Equal(*inativo.DataDesligamento))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorList_Vazio(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento FROM vendedores ORDER BY nome ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_desligamento"}))

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	vendedores, err := repo.List(ctx, db)

	require.NoError(t, err)
	assert.Len(t, vendedores, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorList_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento FROM vendedores ORDER BY nome ASC`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	_, err := repo.List(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorList_ScanError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	// Faltando coluna força erro de scan (quantidade de colunas divergente).
	rows := sqlmock.NewRows([]string{"id", "nome", "regiao"}).
		AddRow(1, "Vendedor A", "Sudeste")

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento FROM vendedores ORDER BY nome ASC`).
		WillReturnRows(rows)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	_, err := repo.List(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_desligamento"}).
		AddRow(1, "Vendedor A", "Sudeste", "SP", nil).
		RowError(0, sql.ErrConnDone)

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento FROM vendedores ORDER BY nome ASC`).
		WillReturnRows(rows)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	_, err := repo.List(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorExistsByID(t *testing.T) {
	testes := []struct {
		nome     string
		id       int64
		mockRow  bool
		mockErr  error
		expected bool
		wantErr  bool
	}{
		{nome: "vendedor existe", id: 1, mockRow: true, expected: true},
		{nome: "vendedor nao existe", id: 999, mockErr: sql.ErrNoRows, expected: false},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			q := mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).WithArgs(tt.id)
			if tt.mockErr != nil {
				q.WillReturnError(tt.mockErr)
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			}

			repo := repositories.NewVendedorRepository()
			ctx := context.Background()
			exists, err := repo.ExistsByID(ctx, db, tt.id)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, exists)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVendedorExistsByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	exists, err := repo.ExistsByID(ctx, db, 1)

	assert.False(t, exists)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func TestVendedorGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(vendedorGetColumns).
		AddRow(1, "João Vendedor", "Sudeste", "SP", now, nil, 5000.0, now, now)

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_admissao, data_desligamento, meta_mensal, created_at, updated_at FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	v, err := repo.GetByID(ctx, db, 1)

	require.NoError(t, err)
	assert.Equal(t, int64(1), v.ID)
	assert.Equal(t, "João Vendedor", v.Nome)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_admissao, data_desligamento, meta_mensal, created_at, updated_at FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	v, err := repo.GetByID(ctx, db, 999)

	assert.Nil(t, v)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_admissao, data_desligamento, meta_mensal, created_at, updated_at FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	v, err := repo.GetByID(ctx, db, 1)

	assert.Nil(t, v)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestVendedorCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	v := &models.Vendedor{Nome: "Novo Vendedor", Regiao: "Sul", UF: "RS", DataAdmissao: now, MetaMensal: 3000.0}

	mock.ExpectExec(`INSERT INTO vendedores \(nome, regiao, uf, data_admissao, data_desligamento, meta_mensal\)\s+VALUES \(\?, \?, \?, \?, \?, \?\)`).
		WithArgs(v.Nome, v.Regiao, v.UF, v.DataAdmissao, v.DataDesligamento, v.MetaMensal).
		WillReturnResult(sqlmock.NewResult(9, 1))

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, v)

	require.NoError(t, err)
	assert.Equal(t, int64(9), v.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorCreate_ExecError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	v := &models.Vendedor{Nome: "Novo Vendedor", Regiao: "Sul", UF: "RS", DataAdmissao: time.Now(), MetaMensal: 3000.0}

	mock.ExpectExec(`INSERT INTO vendedores \(nome, regiao, uf, data_admissao, data_desligamento, meta_mensal\)\s+VALUES \(\?, \?, \?, \?, \?, \?\)`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, v)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorCreate_LastInsertIdError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	v := &models.Vendedor{Nome: "Novo Vendedor", Regiao: "Sul", UF: "RS", DataAdmissao: time.Now(), MetaMensal: 3000.0}

	mock.ExpectExec(`INSERT INTO vendedores \(nome, regiao, uf, data_admissao, data_desligamento, meta_mensal\)\s+VALUES \(\?, \?, \?, \?, \?, \?\)`).
		WillReturnResult(sqlmock.NewErrorResult(sql.ErrConnDone))

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, v)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestVendedorUpdate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	v := &models.Vendedor{Nome: "Vendedor Atualizado", Regiao: "Nordeste", UF: "BA", DataAdmissao: now, MetaMensal: 4000.0}

	mock.ExpectExec(`UPDATE vendedores\s+SET nome = \?, regiao = \?, uf = \?, data_admissao = \?, meta_mensal = \?\s+WHERE id = \?`).
		WithArgs(v.Nome, v.Regiao, v.UF, v.DataAdmissao, v.MetaMensal, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, v)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorUpdate_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	v := &models.Vendedor{Nome: "Vendedor X", Regiao: "Nordeste", UF: "BA", DataAdmissao: time.Now(), MetaMensal: 4000.0}

	mock.ExpectExec(`UPDATE vendedores\s+SET nome = \?, regiao = \?, uf = \?, data_admissao = \?, meta_mensal = \?\s+WHERE id = \?`).
		WithArgs(v.Nome, v.Regiao, v.UF, v.DataAdmissao, v.MetaMensal, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 999, v)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorUpdate_ExecError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	v := &models.Vendedor{Nome: "Vendedor X", Regiao: "Nordeste", UF: "BA", DataAdmissao: time.Now(), MetaMensal: 4000.0}

	mock.ExpectExec(`UPDATE vendedores\s+SET nome = \?, regiao = \?, uf = \?, data_admissao = \?, meta_mensal = \?\s+WHERE id = \?`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, v)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorUpdate_RowsAffectedError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	v := &models.Vendedor{Nome: "Vendedor X", Regiao: "Nordeste", UF: "BA", DataAdmissao: time.Now(), MetaMensal: 4000.0}

	mock.ExpectExec(`UPDATE vendedores\s+SET nome = \?, regiao = \?, uf = \?, data_admissao = \?, meta_mensal = \?\s+WHERE id = \?`).
		WillReturnResult(sqlmock.NewErrorResult(sql.ErrConnDone))

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, v)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// SetDataDesligamento
// ---------------------------------------------------------------------------

func TestVendedorSetDataDesligamento_Success(t *testing.T) {
	testes := []struct {
		nome             string
		dataDesligamento *sql.NullTime
	}{
		{
			nome:             "inativa vendedor (define data)",
			dataDesligamento: &sql.NullTime{Time: time.Now(), Valid: true},
		},
		{
			nome:             "reativa vendedor (limpa data)",
			dataDesligamento: &sql.NullTime{Valid: false},
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
				WithArgs(tt.dataDesligamento, int64(1)).
				WillReturnResult(sqlmock.NewResult(0, 1))

			repo := repositories.NewVendedorRepository()
			ctx := context.Background()
			err := repo.SetDataDesligamento(ctx, db, 1, tt.dataDesligamento)

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVendedorSetDataDesligamento_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	dataDesligamento := &sql.NullTime{Time: time.Now(), Valid: true}

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WithArgs(dataDesligamento, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	err := repo.SetDataDesligamento(ctx, db, 999, dataDesligamento)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorSetDataDesligamento_ExecError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	dataDesligamento := &sql.NullTime{Time: time.Now(), Valid: true}

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	err := repo.SetDataDesligamento(ctx, db, 1, dataDesligamento)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorSetDataDesligamento_RowsAffectedError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	dataDesligamento := &sql.NullTime{Time: time.Now(), Valid: true}

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WillReturnResult(sqlmock.NewErrorResult(sql.ErrConnDone))

	repo := repositories.NewVendedorRepository()
	ctx := context.Background()
	err := repo.SetDataDesligamento(ctx, db, 1, dataDesligamento)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
