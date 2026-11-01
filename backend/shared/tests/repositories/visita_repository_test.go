package repositories_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/kernel"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var visitaColumns = []string{
	"visita_id", "cliente_id", "vendedor_id", "data_visita", "resultado", "duracao_min",
	"created_at", "updated_at",
}

func visitaRow(id, clienteID, vendedorID int64, dataVisita time.Time, resultado string, duracaoMin int, created, updated time.Time) []driver.Value {
	return []driver.Value{id, clienteID, vendedorID, dataVisita, resultado, duracaoMin, created, updated}
}

func TestVisitaList_SemFiltro(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM visitas").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))

	now := time.Now()
	rows := sqlmock.NewRows(visitaColumns).
		AddRow(visitaRow(1, 100, 1, now, "Positiva", 30, now, now)...).
		AddRow(visitaRow(2, 101, 2, now, "Negativa", 15, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM visitas ORDER BY visita_id ASC LIMIT \\? OFFSET \\?").
		WithArgs(10, 0).
		WillReturnRows(rows)

	ctx := context.Background()
	visitas, total, err := itensETotal(comercialmysql.NovoVisitas(db).Listar(ctx, escopoAdmin(), comercial.FiltroVisita{Ordenacao: comercial.OrdenacaoVisitaPadrao()}, kernel.NovaPagina(1, 10)))

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, visitas, 2)
	assert.Equal(t, int64(1), visitas[0].VisitaID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaList_ComFiltros(t *testing.T) {
	testes := []struct {
		nome        string
		filtro      comercial.FiltroVisita
		whereRegexp string
		args        []driver.Value
	}{
		{
			nome:        "filtro por cliente_id",
			filtro:      comercial.FiltroVisita{ClienteID: 100, Ordenacao: comercial.OrdenacaoVisitaPadrao()},
			whereRegexp: "WHERE cliente_id = \\?",
			args:        []driver.Value{int64(100)},
		},
		{
			nome:        "filtro por vendedor_id",
			filtro:      comercial.FiltroVisita{VendedorID: 5, Ordenacao: comercial.OrdenacaoVisitaPadrao()},
			whereRegexp: "WHERE vendedor_id = \\?",
			args:        []driver.Value{int64(5)},
		},
		{
			nome:        "filtro por resultado",
			filtro:      comercial.FiltroVisita{Resultado: "Positiva", Ordenacao: comercial.OrdenacaoVisitaPadrao()},
			whereRegexp: "WHERE resultado = \\?",
			args:        []driver.Value{"Positiva"},
		},
		{
			nome:        "filtro por data_visita_de",
			filtro:      comercial.FiltroVisita{De: "2024-01-01", Ordenacao: comercial.OrdenacaoVisitaPadrao()},
			whereRegexp: "WHERE data_visita >= \\?",
			args:        []driver.Value{"2024-01-01"},
		},
		{
			nome:        "filtro por data_visita_ate",
			filtro:      comercial.FiltroVisita{Ate: "2024-12-31", Ordenacao: comercial.OrdenacaoVisitaPadrao()},
			whereRegexp: "WHERE data_visita <= \\?",
			args:        []driver.Value{"2024-12-31"},
		},
		{
			nome:        "filtro por busca textual",
			filtro:      comercial.FiltroVisita{Busca: "Positiva", Ordenacao: comercial.OrdenacaoVisitaPadrao()},
			whereRegexp: "WHERE resultado LIKE \\? ESCAPE '!'",
			args:        []driver.Value{"%Positiva%"},
		},
		{
			nome:        "filtros combinados",
			filtro:      comercial.FiltroVisita{ClienteID: 100, VendedorID: 5, Resultado: "Positiva", Ordenacao: comercial.OrdenacaoVisitaPadrao()},
			whereRegexp: "WHERE cliente_id = \\? AND vendedor_id = \\? AND resultado = \\?",
			args:        []driver.Value{int64(100), int64(5), "Positiva"},
		},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM visitas " + tt.whereRegexp).
				WithArgs(tt.args...).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

			allArgs := append(append([]driver.Value{}, tt.args...), int64(10), int64(0))
			mock.ExpectQuery("SELECT .+ FROM visitas " + tt.whereRegexp + " ORDER BY visita_id ASC LIMIT \\? OFFSET \\?").
				WithArgs(allArgs...).
				WillReturnRows(sqlmock.NewRows(visitaColumns))

			ctx := context.Background()
			_, _, err := itensETotal(comercialmysql.NovoVisitas(db).Listar(ctx, escopoAdmin(), tt.filtro, kernel.NovaPagina(1, 10)))

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVisitaList_OrderBy(t *testing.T) {
	testes := []struct {
		nome        string
		orderBy     string
		orderDir    string
		orderRegexp string
	}{
		{"order_by válido asc", "duracao_min", "asc", "ORDER BY duracao_min ASC"},
		{"order_by válido desc", "data_visita", "desc", "ORDER BY data_visita DESC"},
		{"order_by fora da whitelist cai no default", "1; DROP TABLE visitas;--", "asc", "ORDER BY visita_id ASC"},
		{"order_dir inválido cai no default (asc)", "resultado", "invalido", "ORDER BY resultado ASC"},
		{"tudo vazio cai no default", "", "", "ORDER BY visita_id ASC"},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()

			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM visitas").
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery("SELECT .+ FROM visitas "+tt.orderRegexp+" LIMIT \\? OFFSET \\?").
				WithArgs(10, 0).
				WillReturnRows(sqlmock.NewRows(visitaColumns))

			ctx := context.Background()
			_, _, err := itensETotal(comercialmysql.NovoVisitas(db).Listar(ctx, escopoAdmin(), comercial.FiltroVisita{Ordenacao: kernel.ParseOrdenacao(tt.orderBy, tt.orderDir, comercial.CamposOrdenacaoVisita(), comercial.OrdenacaoVisitaPadrao())}, kernel.NovaPagina(1, 10)))

			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVisitaList_CountError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM visitas").
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, _, err := itensETotal(comercialmysql.NovoVisitas(db).Listar(ctx, escopoAdmin(), comercial.FiltroVisita{Ordenacao: comercial.OrdenacaoVisitaPadrao()}, kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaList_QueryError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM visitas").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM visitas").
		WithArgs(10, 0).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	_, _, err := itensETotal(comercialmysql.NovoVisitas(db).Listar(ctx, escopoAdmin(), comercial.FiltroVisita{Ordenacao: comercial.OrdenacaoVisitaPadrao()}, kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaList_IterError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM visitas").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM visitas").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(visitaColumns).
			AddRow(visitaRow(1, 100, 1, now, "Positiva", 30, now, now)...).
			RowError(0, sql.ErrConnDone))

	ctx := context.Background()
	_, _, err := itensETotal(comercialmysql.NovoVisitas(db).Listar(ctx, escopoAdmin(), comercial.FiltroVisita{Ordenacao: comercial.OrdenacaoVisitaPadrao()}, kernel.NovaPagina(1, 10)))

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaGetByID_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows(visitaColumns).
		AddRow(visitaRow(1, 100, 1, now, "Positiva", 30, now, now)...)

	mock.ExpectQuery("SELECT .+ FROM visitas WHERE visita_id = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	ctx := context.Background()
	v, err := comercialmysql.NovoVisitas(db).PorID(ctx, 1)

	require.NoError(t, err)
	assert.Equal(t, int64(1), v.VisitaID)
	assert.Equal(t, "Positiva", v.Resultado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaGetByID_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM visitas WHERE visita_id = \\? LIMIT 1").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	ctx := context.Background()
	v, err := comercialmysql.NovoVisitas(db).PorID(ctx, 999)

	assert.Nil(t, v)
	assert.ErrorIs(t, err, comercial.ErrVisitaNaoEncontrada)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaGetByID_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	mock.ExpectQuery("SELECT .+ FROM visitas WHERE visita_id = \\? LIMIT 1").
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	v, err := comercialmysql.NovoVisitas(db).PorID(ctx, 1)

	assert.Nil(t, v)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, comercial.ErrVisitaNaoEncontrada)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaCreate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	v := &comercial.Visita{
		ClienteID:  100,
		VendedorID: 1,
		DataVisita: now,
		Resultado:  "Positiva",
		DuracaoMin: 30,
	}

	mock.ExpectExec("INSERT INTO visitas").
		WithArgs(v.ClienteID, v.VendedorID, v.DataVisita, v.Resultado, v.DuracaoMin).
		WillReturnResult(sqlmock.NewResult(9, 1))

	ctx := context.Background()
	err := comercialmysql.NovoVisitas(db).Inserir(ctx, v)

	require.NoError(t, err)
	assert.Equal(t, int64(9), v.VisitaID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaCreate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	v := &comercial.Visita{}
	mock.ExpectExec("INSERT INTO visitas").
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	err := comercialmysql.NovoVisitas(db).Inserir(ctx, v)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaUpdate_Success(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	now := time.Now()
	v := &comercial.Visita{
		ClienteID:  100,
		VendedorID: 1,
		DataVisita: now,
		Resultado:  "Negativa",
		DuracaoMin: 45,
	}

	mock.ExpectExec("UPDATE visitas").
		WithArgs(v.ClienteID, v.VendedorID, v.DataVisita, v.Resultado, v.DuracaoMin, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	err := comercialmysql.NovoVisitas(db).Atualizar(ctx, 1, v)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaUpdate_NotFound(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	v := &comercial.Visita{}
	mock.ExpectExec("UPDATE visitas").
		WillReturnResult(sqlmock.NewResult(0, 0))

	ctx := context.Background()
	err := comercialmysql.NovoVisitas(db).Atualizar(ctx, 999, v)

	assert.ErrorIs(t, err, comercial.ErrVisitaNaoEncontrada)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitaUpdate_DBError(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()

	v := &comercial.Visita{}
	mock.ExpectExec("UPDATE visitas").
		WillReturnError(sql.ErrConnDone)

	ctx := context.Background()
	err := comercialmysql.NovoVisitas(db).Atualizar(ctx, 1, v)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
