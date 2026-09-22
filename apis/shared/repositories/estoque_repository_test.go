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

var estoqueColumns = []string{
	"id", "data_snapshot", "sku", "produto_descricao", "saldo", "ruptura", "created_at", "updated_at",
}

func estoqueRow(id int64, dataSnapshot time.Time, sku string, descricao string, saldo int, ruptura bool, created, updated time.Time) []driver.Value {
	return []driver.Value{id, dataSnapshot, sku, descricao, saldo, ruptura, created, updated}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestEstoqueList_SemFiltro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM estoque").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(estoqueColumns).
		AddRow(estoqueRow(1, now, "SKU1", "Perfume 1", 10, false, now, now)...).
		AddRow(estoqueRow(2, now.AddDate(0, 0, -1), "SKU1", "Perfume 1", 5, false, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM estoque e LEFT JOIN produtos p ON p\\.sku = e\\.sku ORDER BY e\\.data_snapshot DESC LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnRows(rows)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	registros, total, err := repo.List(ctx, db, 1, 10, repositories.EstoqueFiltro{})

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, registros, 2)
	assert.Equal(t, "SKU1", registros[0].SKU)
	assert.Equal(t, "Perfume 1", registros[0].ProdutoDescricao)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueList_ComFiltros(t *testing.T) {
	dataDe := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	dataAte := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	testes := []struct {
		nome        string
		filtro      repositories.EstoqueFiltro
		whereRegexp string
		args        []driver.Value
	}{
		{
			nome:        "filtro por sku",
			filtro:      repositories.EstoqueFiltro{SKU: "SKU1"},
			whereRegexp: "WHERE e\\.sku = \\?",
			args:        []driver.Value{"SKU1"},
		},
		{
			nome:        "filtro por data_de",
			filtro:      repositories.EstoqueFiltro{DataDe: &dataDe},
			whereRegexp: "WHERE e\\.data_snapshot >= \\?",
			args:        []driver.Value{"2024-01-01"},
		},
		{
			nome:        "filtro por data_ate",
			filtro:      repositories.EstoqueFiltro{DataAte: &dataAte},
			whereRegexp: "WHERE e\\.data_snapshot <= \\?",
			args:        []driver.Value{"2024-01-31"},
		},
		{
			nome:        "filtro por ruptura",
			filtro:      repositories.EstoqueFiltro{Ruptura: boolPtr(true)},
			whereRegexp: "WHERE e\\.ruptura = \\?",
			args:        []driver.Value{true},
		},
		{
			nome:        "filtro por sku + intervalo de datas",
			filtro:      repositories.EstoqueFiltro{SKU: "SKU1", DataDe: &dataDe, DataAte: &dataAte},
			whereRegexp: "WHERE e\\.sku = \\? AND e\\.data_snapshot >= \\? AND e\\.data_snapshot <= \\?",
			args:        []driver.Value{"SKU1", "2024-01-01", "2024-01-31"},
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM estoque.+" + tt.whereRegexp).
				WithArgs(tt.args...).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

			allArgs := append(append([]driver.Value{}, tt.args...), int64(10), int64(0))
			mock.ExpectQuery("SELECT .+ FROM estoque.+" + tt.whereRegexp + " ORDER BY e\\.data_snapshot DESC LIMIT \\? OFFSET \\?").
				WithArgs(allArgs...).
				WillReturnRows(sqlmock.NewRows(estoqueColumns))

			repo := repositories.NewEstoqueRepository()
			ctx := context.Background()
			_, _, err := repo.List(ctx, db, 1, 10, tt.filtro)

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestEstoqueList_OrderBy(t *testing.T) {
	testes := []struct {
		nome        string
		orderBy     string
		orderDir    string
		orderRegexp string
	}{
		{
			nome:        "order_by válido asc",
			orderBy:     "saldo",
			orderDir:    "asc",
			orderRegexp: "ORDER BY e\\.saldo ASC",
		},
		{
			nome:        "order_by válido desc (default)",
			orderBy:     "",
			orderDir:    "",
			orderRegexp: "ORDER BY e\\.data_snapshot DESC",
		},
		{
			nome:        "order_by case-insensitive",
			orderBy:     "SKU",
			orderDir:    "ASC",
			orderRegexp: "ORDER BY e\\.sku ASC",
		},
		{
			nome:        "order_by fora da whitelist cai no default",
			orderBy:     "1; DROP TABLE estoque;--",
			orderDir:    "asc",
			orderRegexp: "ORDER BY e\\.data_snapshot ASC",
		},
		{
			nome:        "order_dir inválido cai no default (desc)",
			orderBy:     "saldo",
			orderDir:    "sideways",
			orderRegexp: "ORDER BY e\\.saldo DESC",
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM estoque").
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery("SELECT .+ FROM estoque.+" + tt.orderRegexp + " LIMIT \\? OFFSET \\?").
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows(estoqueColumns))

			repo := repositories.NewEstoqueRepository()
			ctx := context.Background()
			_, _, err := repo.List(ctx, db, 1, 10, repositories.EstoqueFiltro{OrderBy: tt.orderBy, OrderDir: tt.orderDir})

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestEstoqueList_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM estoque").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.EstoqueFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueList_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM estoque").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM estoque").
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.EstoqueFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM estoque").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM estoque").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(estoqueColumns).
			AddRow(estoqueRow(1, now, "SKU1", "Perfume 1", 10, false, now, now)...).
			RowError(0, sql.ErrConnDone))

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.EstoqueFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UltimaPosicaoPorSku
// ---------------------------------------------------------------------------

func TestEstoqueUltimaPosicaoPorSku_RetornaApenasUltimoPorSku(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM \\(.+ROW_NUMBER\\(\\) OVER \\(PARTITION BY e\\.sku ORDER BY e\\.data_snapshot DESC, e\\.id DESC\\).+FROM estoque e LEFT JOIN produtos.+\\) ranked WHERE rn = 1").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))

	now := time.Now()
	// Só o registro mais recente (rn=1) é retornado pela query real (a
	// window function já filtra no SQL); aqui simulamos a linha resultante.
	rows := sqlmock.NewRows(estoqueColumns).
		AddRow(estoqueRow(2, now, "SKU1", "Perfume 1", 8, false, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM \\(.+ROW_NUMBER\\(\\) OVER \\(PARTITION BY e\\.sku ORDER BY e\\.data_snapshot DESC, e\\.id DESC\\).+\\) ranked WHERE rn = 1 ORDER BY data_snapshot DESC LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnRows(rows)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	registros, total, err := repo.UltimaPosicaoPorSku(ctx, db, 1, 10, repositories.EstoqueFiltro{})

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, registros, 1)
	assert.Equal(t, int64(2), registros[0].ID)
	assert.Equal(t, 8, registros[0].Saldo)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueUltimaPosicaoPorSku_RespeitaFiltroDeData(t *testing.T) {
	dataDe := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	dataAte := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	db, mock := newMock(t)
	defer db.Close()

	whereRegexp := "WHERE e\\.data_snapshot >= \\? AND e\\.data_snapshot <= \\?"

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM \\(.+" + whereRegexp + "\\) ranked WHERE rn = 1").
		WithArgs("2024-01-01", "2024-01-31").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))

	now := time.Now()
	rows := sqlmock.NewRows(estoqueColumns).
		AddRow(estoqueRow(5, dataAte, "SKU1", "Perfume 1", 3, false, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM \\(.+" + whereRegexp + "\\) ranked WHERE rn = 1 ORDER BY data_snapshot DESC LIMIT \\? OFFSET \\?").
		WithArgs("2024-01-01", "2024-01-31", 10, 0).
		WillReturnRows(rows)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	registros, total, err := repo.UltimaPosicaoPorSku(ctx, db, 1, 10, repositories.EstoqueFiltro{DataDe: &dataDe, DataAte: &dataAte})

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, registros, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueUltimaPosicaoPorSku_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM \\(.+\\) ranked WHERE rn = 1").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	_, _, err := repo.UltimaPosicaoPorSku(ctx, db, 1, 10, repositories.EstoqueFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueUltimaPosicaoPorSku_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM \\(.+\\) ranked WHERE rn = 1").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM \\(.+\\) ranked WHERE rn = 1").
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	_, _, err := repo.UltimaPosicaoPorSku(ctx, db, 1, 10, repositories.EstoqueFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func TestEstoqueGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(estoqueColumns).
		AddRow(estoqueRow(1, now, "SKU1", "Perfume 1", 10, false, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM estoque.+WHERE e\\.id = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	e, err := repo.GetByID(ctx, db, 1)

	require.NoError(t, err)
	assert.Equal(t, "SKU1", e.SKU)
	assert.Equal(t, "Perfume 1", e.ProdutoDescricao)
	assert.Equal(t, 10, e.Saldo)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM estoque.+WHERE e\\.id = \\? LIMIT 1").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	e, err := repo.GetByID(ctx, db, 999)

	assert.Nil(t, e)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM estoque.+WHERE e\\.id = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	e, err := repo.GetByID(ctx, db, 1)

	assert.Nil(t, e)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestEstoqueCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	data := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	e := &models.Estoque{
		DataSnapshot: data,
		SKU:          "SKU1",
		Saldo:        10,
		Ruptura:      false,
	}

	mock.ExpectExec("INSERT INTO estoque").
		WithArgs("2024-06-01", "SKU1", 10, false).
		WillReturnResult(sqlmock.NewResult(7, 1))

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, e)

	require.NoError(t, err)
	assert.Equal(t, int64(7), e.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueCreate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	e := &models.Estoque{}
	mock.ExpectExec("INSERT INTO estoque").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, e)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestEstoqueUpdate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	e := &models.Estoque{Saldo: 20, Ruptura: false}

	mock.ExpectExec("UPDATE estoque").
		WithArgs(20, false, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, e)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueUpdate_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	e := &models.Estoque{}
	mock.ExpectExec("UPDATE estoque").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 999, e)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueUpdate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	e := &models.Estoque{}
	mock.ExpectExec("UPDATE estoque").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, e)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UpsertPorDataSku
// ---------------------------------------------------------------------------

func TestEstoqueUpsertPorDataSku_InsertNovo(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	data := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectExec("INSERT INTO estoque .+ ON DUPLICATE KEY UPDATE").
		WithArgs("2024-06-01", "SKU1", 15, false).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.UpsertPorDataSku(ctx, db, "SKU1", data, 15, false)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueUpsertPorDataSku_UpdateExistente(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	data := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	// ON DUPLICATE KEY UPDATE: mesmo INSERT, mas RowsAffected=2 é o
	// comportamento típico do MySQL para upsert que atualiza (em vez de
	// inserir) — não afeta o retorno do método (void em caso de sucesso).
	mock.ExpectExec("INSERT INTO estoque .+ ON DUPLICATE KEY UPDATE").
		WithArgs("2024-06-01", "SKU1", 0, true).
		WillReturnResult(sqlmock.NewResult(1, 2))

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.UpsertPorDataSku(ctx, db, "SKU1", data, 0, true)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueUpsertPorDataSku_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	data := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectExec("INSERT INTO estoque").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.UpsertPorDataSku(ctx, db, "SKU1", data, 15, false)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// AjustarSaldoPorFaturamento
// ---------------------------------------------------------------------------

// newMockTx cria um sqlmock configurado para transações (usado pelos testes
// de AjustarSaldoPorFaturamento, que recebe um *sql.Tx).
func newMockTx(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *sql.Tx) {
	db, mock := newMock(t)
	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)
	return db, mock, tx
}

func TestEstoqueAjustarSaldoPorFaturamento_CriaRegistroQuandoNaoExiste(t *testing.T) {
	db, mock, tx := newMockTx(t)
	defer db.Close()

	data := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
		WithArgs("2024-06-01", "SKU1").
		WillReturnError(sql.ErrNoRows)
	// delta=5 (baixa): contribuicao = -5, ruptura = (-5 <= 0) = true.
	mock.ExpectExec(`INSERT INTO estoque .+ ON DUPLICATE KEY UPDATE`).
		WithArgs("2024-06-01", "SKU1", -5, true).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.AjustarSaldoPorFaturamento(ctx, tx, "SKU1", data, 5)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueAjustarSaldoPorFaturamento_DecrementaSaldoExistente(t *testing.T) {
	db, mock, tx := newMockTx(t)
	defer db.Close()

	data := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
		WithArgs("2024-06-01", "SKU1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
	mock.ExpectExec(`INSERT INTO estoque .+ ON DUPLICATE KEY UPDATE`).
		WithArgs("2024-06-01", "SKU1", -3, true).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.AjustarSaldoPorFaturamento(ctx, tx, "SKU1", data, 3)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueAjustarSaldoPorFaturamento_RupturaRecalculada(t *testing.T) {
	// Cobre a derivação de ruptura a partir do delta/contribuição aplicada:
	// delta positivo grande o suficiente para deixar o saldo <= 0.
	testes := []struct {
		nome            string
		delta           int
		wantContrib     int
		wantRupturaFlag bool
	}{
		{"delta pequeno não gera ruptura (contribuição positiva)", -2, 2, false},
		{"delta gera ruptura (contribuição negativa/zero)", 10, -10, true},
		{"delta exatamente zero de contribuição gera ruptura (<=0)", 0, 0, true},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock, tx := newMockTx(t)
			defer db.Close()

			data := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

			mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
				WithArgs("2024-06-01", "SKU1").
				WillReturnError(sql.ErrNoRows)
			mock.ExpectExec(`INSERT INTO estoque .+ ON DUPLICATE KEY UPDATE`).
				WithArgs("2024-06-01", "SKU1", tt.wantContrib, tt.wantRupturaFlag).
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectCommit()

			repo := repositories.NewEstoqueRepository()
			ctx := context.Background()
			err := repo.AjustarSaldoPorFaturamento(ctx, tx, "SKU1", data, tt.delta)
			require.NoError(t, err)
			require.NoError(t, tx.Commit())

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestEstoqueAjustarSaldoPorFaturamento_ErroNoLock(t *testing.T) {
	db, mock, tx := newMockTx(t)
	defer db.Close()

	data := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
		WithArgs("2024-06-01", "SKU1").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.AjustarSaldoPorFaturamento(ctx, tx, "SKU1", data, 5)
	assert.Error(t, err)
	require.NoError(t, tx.Rollback())

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEstoqueAjustarSaldoPorFaturamento_ErroNoUpsert(t *testing.T) {
	db, mock, tx := newMockTx(t)
	defer db.Close()

	data := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
		WithArgs("2024-06-01", "SKU1").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO estoque .+ ON DUPLICATE KEY UPDATE`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	repo := repositories.NewEstoqueRepository()
	ctx := context.Background()
	err := repo.AjustarSaldoPorFaturamento(ctx, tx, "SKU1", data, 5)
	assert.Error(t, err)
	require.NoError(t, tx.Rollback())

	assert.NoError(t, mock.ExpectationsWereMet())
}
