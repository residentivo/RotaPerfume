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

var clienteColumns = []string{
	"cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
	"data_cadastro", "ativo", "created_at", "updated_at",
}

func clienteRow(idOrigem int64, cnpj, razao, segmento, cidade, uf, bairro string, dataCadastro time.Time, ativo bool, created, updated time.Time) []driver.Value {
	return []driver.Value{idOrigem, cnpj, razao, segmento, cidade, uf, bairro, dataCadastro, ativo, created, updated}
}

func TestClienteList_SemFiltro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(clienteColumns).
		AddRow(clienteRow(100, "11111111000100", "Empresa A", "Perfumaria", "SP", "SP", "Centro", now, true, now, now)...).
		AddRow(clienteRow(101, "22222222000100", "Empresa B", "Cosmeticos", "RJ", "RJ", "Zona Sul", now, true, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM clientes ORDER BY cliente_id_origem ASC LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnRows(rows)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	clientes, total, err := repo.List(ctx, db, 1, 10, repositories.ClienteFiltro{})

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, clientes, 2)
	assert.Equal(t, int64(100), clientes[0].ClienteIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteList_ComFiltros(t *testing.T) {
	testes := []struct {
		nome        string
		filtro      repositories.ClienteFiltro
		whereRegexp string
		args        []driver.Value
	}{
		{
			nome:        "filtro por UF",
			filtro:      repositories.ClienteFiltro{UF: "SP"},
			whereRegexp: "WHERE uf = \\?",
			args:        []driver.Value{"SP"},
		},
		{
			nome:        "filtro por segmento",
			filtro:      repositories.ClienteFiltro{Segmento: "Perfumaria"},
			whereRegexp: "WHERE segmento = \\?",
			args:        []driver.Value{"Perfumaria"},
		},
		{
			nome:        "filtro por ativo",
			filtro:      repositories.ClienteFiltro{Ativo: boolPtr(true)},
			whereRegexp: "WHERE ativo = \\?",
			args:        []driver.Value{true},
		},
		{
			nome:        "filtro por busca textual",
			filtro:      repositories.ClienteFiltro{Q: "Perfumes"},
			whereRegexp: "WHERE \\(razao_social LIKE \\? OR cnpj LIKE \\?\\)",
			args:        []driver.Value{"%Perfumes%", "%Perfumes%"},
		},
		{
			nome:        "filtros combinados",
			filtro:      repositories.ClienteFiltro{UF: "SP", Segmento: "Perfumaria", Ativo: boolPtr(true)},
			whereRegexp: "WHERE uf = \\? AND segmento = \\? AND ativo = \\?",
			args:        []driver.Value{"SP", "Perfumaria", true},
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes " + tt.whereRegexp).
				WithArgs(tt.args...).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

			allArgs := append(append([]driver.Value{}, tt.args...), int64(10), int64(0))
			mock.ExpectQuery("SELECT .+ FROM clientes " + tt.whereRegexp + " ORDER BY cliente_id_origem ASC LIMIT \\? OFFSET \\?").
				WithArgs(allArgs...).
				WillReturnRows(sqlmock.NewRows(clienteColumns))

			repo := repositories.NewClienteRepository()
			ctx := context.Background()
			_, _, err := repo.List(ctx, db, 1, 10, tt.filtro)

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestClienteList_OrderBy(t *testing.T) {
	testes := []struct {
		nome        string
		orderBy     string
		orderDir    string
		orderRegexp string
	}{
		{"order_by válido asc", "razao_social", "asc", "ORDER BY razao_social ASC"},
		{"order_by válido desc", "data_cadastro", "desc", "ORDER BY data_cadastro DESC"},
		{"order_by fora da whitelist cai no default", "1; DROP TABLE clientes;--", "asc", "ORDER BY cliente_id_origem ASC"},
		{"order_dir inválido cai no default (asc)", "cnpj", "invalido", "ORDER BY cnpj ASC"},
		{"tudo vazio cai no default", "", "", "ORDER BY cliente_id_origem ASC"},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes").
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery("SELECT .+ FROM clientes " + tt.orderRegexp + " LIMIT \\? OFFSET \\?").
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows(clienteColumns))

			repo := repositories.NewClienteRepository()
			ctx := context.Background()
			_, _, err := repo.List(ctx, db, 1, 10, repositories.ClienteFiltro{OrderBy: tt.orderBy, OrderDir: tt.orderDir})

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestClienteList_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.ClienteFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteList_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM clientes").
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.ClienteFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM clientes").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(clienteColumns).
			AddRow(clienteRow(100, "cnpj", "razao", "seg", "cidade", "uf", "bairro", now, true, now, now)...).
			RowError(0, sql.ErrConnDone))

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.ClienteFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteExistsByID(t *testing.T) {
	testes := []struct {
		nome     string
		id       int64
		mockErr  error
		expected bool
	}{
		{nome: "cliente existe", id: 1, expected: true},
		{nome: "cliente nao existe", id: 999, mockErr: sql.ErrNoRows, expected: false},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			q := mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).WithArgs(tt.id)
			if tt.mockErr != nil {
				q.WillReturnError(tt.mockErr)
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			}

			repo := repositories.NewClienteRepository()
			ctx := context.Background()
			exists, err := repo.ExistsByID(ctx, db, tt.id)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, exists)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestClienteExistsByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	exists, err := repo.ExistsByID(ctx, db, 1)

	assert.False(t, exists)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(clienteColumns).
		AddRow(clienteRow(100, "11111111000100", "Empresa A", "Perfumaria", "SP", "SP", "Centro", now, true, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM clientes WHERE cliente_id_origem = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	c, err := repo.GetByID(ctx, db, 1)

	require.NoError(t, err)
	assert.Equal(t, int64(100), c.ClienteIDOrigem)
	assert.Equal(t, "Empresa A", c.RazaoSocial)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM clientes WHERE cliente_id_origem = \\? LIMIT 1").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	c, err := repo.GetByID(ctx, db, 999)

	assert.Nil(t, c)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM clientes WHERE cliente_id_origem = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	c, err := repo.GetByID(ctx, db, 1)

	assert.Nil(t, c)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteSetAtivo_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE cliente_id_origem = \?`).
		WithArgs(false, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	err := repo.SetAtivo(ctx, db, 1, false)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteSetAtivo_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE cliente_id_origem = \?`).
		WithArgs(true, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	err := repo.SetAtivo(ctx, db, 999, true)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteSetAtivo_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE cliente_id_origem = \?`).
		WithArgs(true, int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	err := repo.SetAtivo(ctx, db, 1, true)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCountTotal(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes$").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(42))

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	total, err := repo.CountTotal(ctx, db)

	require.NoError(t, err)
	assert.Equal(t, 42, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCountTotal_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes$").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, err := repo.CountTotal(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCountPorAtivo(t *testing.T) {
	testes := []struct {
		nome  string
		ativo bool
		total int
	}{
		{"clientes ativos", true, 30},
		{"clientes inativos", false, 5},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes WHERE ativo = \\?").
				WithArgs(tt.ativo).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(tt.total))

			repo := repositories.NewClienteRepository()
			ctx := context.Background()
			total, err := repo.CountPorAtivo(ctx, db, tt.ativo)

			require.NoError(t, err)
			assert.Equal(t, tt.total, total)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestClienteCountPorAtivo_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes WHERE ativo = \\?").
		WithArgs(true).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, err := repo.CountPorAtivo(ctx, db, true)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCountNovosNoPeriodo(t *testing.T) {
	testes := []struct {
		nome    string
		periodo string
	}{
		{"periodo today", "today"},
		{"periodo month", "month"},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes WHERE .+").
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(3))

			repo := repositories.NewClienteRepository()
			ctx := context.Background()
			total, err := repo.CountNovosNoPeriodo(ctx, db, tt.periodo)

			require.NoError(t, err)
			assert.Equal(t, 3, total)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestClienteCountNovosNoPeriodo_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM clientes WHERE .+").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, err := repo.CountNovosNoPeriodo(ctx, db, "today")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCountPorSegmento_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"segmento", "total"}).
		AddRow("Perfumaria", 10).
		AddRow("Cosmeticos", 5)

	mock.ExpectQuery("SELECT segmento, COUNT\\(\\*\\) AS total").
		WillReturnRows(rows)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	result, err := repo.CountPorSegmento(ctx, db)

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "Perfumaria", result[0].Segmento)
	assert.Equal(t, 10, result[0].Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCountPorSegmento_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT segmento, COUNT\\(\\*\\) AS total").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, err := repo.CountPorSegmento(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCountPorSegmento_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"segmento", "total"}).
		AddRow("Perfumaria", 10).
		RowError(0, sql.ErrConnDone)

	mock.ExpectQuery("SELECT segmento, COUNT\\(\\*\\) AS total").
		WillReturnRows(rows)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, err := repo.CountPorSegmento(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCountPorUF_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"uf", "total"}).
		AddRow("SP", 20).
		AddRow("RJ", 8)

	mock.ExpectQuery("SELECT uf, COUNT\\(\\*\\) AS total").
		WillReturnRows(rows)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	result, err := repo.CountPorUF(ctx, db)

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "SP", result[0].UF)
	assert.Equal(t, 20, result[0].Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCountPorUF_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT uf, COUNT\\(\\*\\) AS total").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, err := repo.CountPorUF(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCountPorUF_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"uf", "total"}).
		AddRow("SP", 20).
		RowError(0, sql.ErrConnDone)

	mock.ExpectQuery("SELECT uf, COUNT\\(\\*\\) AS total").
		WillReturnRows(rows)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	_, err := repo.CountPorUF(ctx, db)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// NextClienteIDOrigem foi removida: cliente_id_origem agora é a PK
// AUTO_INCREMENT da tabela clientes, e o valor é obtido via LastInsertId()
// no repositório Create (ver TestClienteCreate_Success abaixo).

func TestClienteCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	c := &models.Cliente{
		CNPJ:         "11111111000100",
		RazaoSocial:  "Empresa A",
		Segmento:     "Perfumaria",
		Cidade:       "SP",
		UF:           "SP",
		Bairro:       "Centro",
		DataCadastro: now,
		Ativo:        true,
	}

	mock.ExpectExec("INSERT INTO clientes").
		WithArgs(c.CNPJ, c.RazaoSocial, c.Segmento, c.Cidade, c.UF, c.Bairro, c.DataCadastro, c.Ativo).
		WillReturnResult(sqlmock.NewResult(9, 1))

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, c)

	require.NoError(t, err)
	assert.Equal(t, int64(9), c.ClienteIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteCreate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	c := &models.Cliente{}
	mock.ExpectExec("INSERT INTO clientes").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, c)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteUpdate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	c := &models.Cliente{
		CNPJ:         "11111111000100",
		RazaoSocial:  "Empresa A Atualizada",
		Segmento:     "Perfumaria",
		Cidade:       "SP",
		UF:           "SP",
		Bairro:       "Centro",
		DataCadastro: now,
	}

	mock.ExpectExec("UPDATE clientes").
		WithArgs(c.CNPJ, c.RazaoSocial, c.Segmento, c.Cidade, c.UF, c.Bairro, c.DataCadastro, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, c)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteUpdate_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	c := &models.Cliente{}
	mock.ExpectExec("UPDATE clientes").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 999, c)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteUpdate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	c := &models.Cliente{}
	mock.ExpectExec("UPDATE clientes").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewClienteRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, c)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func boolPtr(b bool) *bool {
	return &b
}
