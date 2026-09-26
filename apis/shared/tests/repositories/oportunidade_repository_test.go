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

var oportunidadeColumns = []string{
	"oportunidade_id", "cliente_id", "vendedor_id", "origem", "data_abertura", "etapa",
	"probabilidade_pct", "valor_estimado", "data_fechamento", "ciclo_dias", "motivo_perda",
	"created_at", "updated_at",
}

func oportunidadeRow(id, clienteID, vendedorID int64, origem string, dataAbertura time.Time, etapa string, prob, valor float64, dataFechamento any, cicloDias any, motivoPerda any, created, updated time.Time) []driver.Value {
	return []driver.Value{id, clienteID, vendedorID, origem, dataAbertura, etapa, prob, valor, dataFechamento, cicloDias, motivoPerda, created, updated}
}

func TestOportunidadeList_SemFiltro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM oportunidades").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(oportunidadeColumns).
		AddRow(oportunidadeRow(1, 100, 1, "Site", now, "Prospeccao", 10, 1000, nil, nil, nil, now, now)...).
		AddRow(oportunidadeRow(2, 101, 2, "Indicacao", now, "Negociacao", 50, 2000, nil, nil, nil, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM oportunidades ORDER BY oportunidade_id ASC LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnRows(rows)

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	oportunidades, total, err := repo.List(ctx, db, 1, 10, repositories.OportunidadeFiltro{})

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, oportunidades, 2)
	assert.Equal(t, int64(1), oportunidades[0].OportunidadeID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeList_ComFiltros(t *testing.T) {
	testes := []struct {
		nome        string
		filtro      repositories.OportunidadeFiltro
		whereRegexp string
		args        []driver.Value
	}{
		{
			nome:        "filtro por cliente_id",
			filtro:      repositories.OportunidadeFiltro{ClienteID: 100},
			whereRegexp: "WHERE cliente_id = \\?",
			args:        []driver.Value{int64(100)},
		},
		{
			nome:        "filtro por vendedor_id",
			filtro:      repositories.OportunidadeFiltro{VendedorID: 5},
			whereRegexp: "WHERE vendedor_id = \\?",
			args:        []driver.Value{int64(5)},
		},
		{
			nome:        "filtro por etapa",
			filtro:      repositories.OportunidadeFiltro{Etapa: "Fechado ganho"},
			whereRegexp: "WHERE etapa = \\?",
			args:        []driver.Value{"Fechado ganho"},
		},
		{
			nome:        "filtro por origem",
			filtro:      repositories.OportunidadeFiltro{Origem: "Site"},
			whereRegexp: "WHERE origem = \\?",
			args:        []driver.Value{"Site"},
		},
		{
			nome:        "filtro por data_abertura_de",
			filtro:      repositories.OportunidadeFiltro{DataAberturaDe: "2024-01-01"},
			whereRegexp: "WHERE data_abertura >= \\?",
			args:        []driver.Value{"2024-01-01"},
		},
		{
			nome:        "filtro por data_abertura_ate",
			filtro:      repositories.OportunidadeFiltro{DataAberturaAte: "2024-12-31"},
			whereRegexp: "WHERE data_abertura <= \\?",
			args:        []driver.Value{"2024-12-31"},
		},
		{
			nome:        "filtro por busca textual",
			filtro:      repositories.OportunidadeFiltro{Q: "Site"},
			whereRegexp: "WHERE \\(origem LIKE \\? OR etapa LIKE \\?\\)",
			args:        []driver.Value{"%Site%", "%Site%"},
		},
		{
			nome:        "filtros combinados",
			filtro:      repositories.OportunidadeFiltro{ClienteID: 100, VendedorID: 5, Etapa: "Negociacao"},
			whereRegexp: "WHERE cliente_id = \\? AND vendedor_id = \\? AND etapa = \\?",
			args:        []driver.Value{int64(100), int64(5), "Negociacao"},
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM oportunidades " + tt.whereRegexp).
				WithArgs(tt.args...).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

			allArgs := append(append([]driver.Value{}, tt.args...), int64(10), int64(0))
			mock.ExpectQuery("SELECT .+ FROM oportunidades " + tt.whereRegexp + " ORDER BY oportunidade_id ASC LIMIT \\? OFFSET \\?").
				WithArgs(allArgs...).
				WillReturnRows(sqlmock.NewRows(oportunidadeColumns))

			repo := repositories.NewOportunidadeRepository()
			ctx := context.Background()
			_, _, err := repo.List(ctx, db, 1, 10, tt.filtro)

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestOportunidadeList_OrderBy(t *testing.T) {
	testes := []struct {
		nome        string
		orderBy     string
		orderDir    string
		orderRegexp string
	}{
		{"order_by válido asc", "valor_estimado", "asc", "ORDER BY valor_estimado ASC"},
		{"order_by válido desc", "data_abertura", "desc", "ORDER BY data_abertura DESC"},
		{"order_by fora da whitelist cai no default", "1; DROP TABLE oportunidades;--", "asc", "ORDER BY oportunidade_id ASC"},
		{"order_dir inválido cai no default (asc)", "etapa", "invalido", "ORDER BY etapa ASC"},
		{"tudo vazio cai no default", "", "", "ORDER BY oportunidade_id ASC"},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM oportunidades").
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery("SELECT .+ FROM oportunidades "+tt.orderRegexp+" LIMIT \\? OFFSET \\?").
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows(oportunidadeColumns))

			repo := repositories.NewOportunidadeRepository()
			ctx := context.Background()
			_, _, err := repo.List(ctx, db, 1, 10, repositories.OportunidadeFiltro{OrderBy: tt.orderBy, OrderDir: tt.orderDir})

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestOportunidadeList_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM oportunidades").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.OportunidadeFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeList_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM oportunidades").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM oportunidades").
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.OportunidadeFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM oportunidades").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM oportunidades").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(oportunidadeColumns).
			AddRow(oportunidadeRow(1, 100, 1, "Site", now, "Prospeccao", 10, 1000, nil, nil, nil, now, now)...).
			RowError(0, sql.ErrConnDone))

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	_, _, err := repo.List(ctx, db, 1, 10, repositories.OportunidadeFiltro{})

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(oportunidadeColumns).
		AddRow(oportunidadeRow(1, 100, 1, "Site", now, "Prospeccao", 10, 1000, nil, nil, nil, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM oportunidades WHERE oportunidade_id = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	o, err := repo.GetByID(ctx, db, 1)

	require.NoError(t, err)
	assert.Equal(t, int64(1), o.OportunidadeID)
	assert.Equal(t, "Prospeccao", o.Etapa)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM oportunidades WHERE oportunidade_id = \\? LIMIT 1").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	o, err := repo.GetByID(ctx, db, 999)

	assert.Nil(t, o)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM oportunidades WHERE oportunidade_id = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	o, err := repo.GetByID(ctx, db, 1)

	assert.Nil(t, o)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	o := &models.Oportunidade{
		ClienteID:        100,
		VendedorID:       1,
		Origem:           "Site",
		DataAbertura:     now,
		Etapa:            "Prospeccao",
		ProbabilidadePct: 10,
		ValorEstimado:    1000,
	}

	mock.ExpectExec("INSERT INTO oportunidades").
		WithArgs(o.ClienteID, o.VendedorID, o.Origem, o.DataAbertura, o.Etapa, o.ProbabilidadePct, o.ValorEstimado, o.DataFechamento, o.CicloDias, o.MotivoPerda).
		WillReturnResult(sqlmock.NewResult(9, 1))

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, o)

	require.NoError(t, err)
	assert.Equal(t, int64(9), o.OportunidadeID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeCreate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	o := &models.Oportunidade{}
	mock.ExpectExec("INSERT INTO oportunidades").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	err := repo.Create(ctx, db, o)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeUpdate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	o := &models.Oportunidade{
		ClienteID:        100,
		VendedorID:       1,
		Origem:           "Site",
		DataAbertura:     now,
		Etapa:            "Negociacao",
		ProbabilidadePct: 50,
		ValorEstimado:    2000,
	}

	mock.ExpectExec("UPDATE oportunidades").
		WithArgs(o.ClienteID, o.VendedorID, o.Origem, o.DataAbertura, o.Etapa, o.ProbabilidadePct, o.ValorEstimado, o.DataFechamento, o.CicloDias, o.MotivoPerda, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, o)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeUpdate_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	o := &models.Oportunidade{}
	mock.ExpectExec("UPDATE oportunidades").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 999, o)

	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOportunidadeUpdate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	o := &models.Oportunidade{}
	mock.ExpectExec("UPDATE oportunidades").
		WillReturnError(sql.ErrConnDone)

	repo := repositories.NewOportunidadeRepository()
	ctx := context.Background()
	err := repo.Update(ctx, db, 1, o)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
