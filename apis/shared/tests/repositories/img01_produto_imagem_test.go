package repositories_test

// IMG-01 (Lote 26, TestBrain): catalogomysql.Produtos.ImagemParaAtualizar
// (SELECT … FOR UPDATE parametrizado; NULL → ""; sem linha → 404) e
// DefinirImagem ("" grava NULL; 0 linhas → 404; erro do driver embrulhado).
// Leitura do produto com a coluna imagem (COALESCE).

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
)

const img01Nome = "3f2b8c1e-9a4d-4c7b-8e2f-1a2b3c4d5e6f.png"

func img01Mock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func TestIMG01_Repo_ImagemParaAtualizar(t *testing.T) {
	const q = `SELECT imagem FROM produtos WHERE id = ? FOR UPDATE`
	casos := []struct {
		nome  string
		mock  func(m sqlmock.Sqlmock)
		want  string
		errIs error
		errS  string
	}{
		{"com imagem", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(img01Nome))
		}, img01Nome, nil, ""},
		{"NULL → vazio", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(nil))
		}, "", nil, ""},
		{"sem linha → produto não encontrado", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"imagem"}))
		}, "", catalogo.ErrProdutoNaoEncontrado, ""},
		{"erro do driver", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q).WithArgs(int64(5)).WillReturnError(errors.New("lock wait timeout"))
		}, "", nil, "catalogomysql: lock imagem do produto: lock wait timeout"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := img01Mock(t)
			c.mock(mock)
			got, err := catalogomysql.NovoProdutos(db).ImagemParaAtualizar(context.Background(), 5)
			switch {
			case c.errIs != nil:
				require.ErrorIs(t, err, c.errIs)
			case c.errS != "":
				require.EqualError(t, err, c.errS)
			default:
				require.NoError(t, err)
			}
			assert.Equal(t, c.want, got)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestIMG01_Repo_DefinirImagem(t *testing.T) {
	const q = `UPDATE produtos SET imagem = ? WHERE id = ?`
	casos := []struct {
		nome  string
		valor string
		arg   driver.Value
		res   driver.Result
		err   error
		errIs error
		errS  string
	}{
		{"grava o nome", img01Nome, img01Nome, sqlmock.NewResult(0, 1), nil, nil, ""},
		{"vazio grava NULL", "", nil, sqlmock.NewResult(0, 1), nil, nil, ""},
		{"0 linhas → produto não encontrado", img01Nome, img01Nome, sqlmock.NewResult(0, 0), nil, catalogo.ErrProdutoNaoEncontrado, ""},
		{"erro do driver", img01Nome, img01Nome, nil, errors.New("check constraint"), nil, "catalogomysql: definir imagem produto: check constraint"},
		{"erro no RowsAffected", img01Nome, img01Nome, sqlmock.NewErrorResult(errors.New("ra")), nil, nil, "catalogomysql: definir imagem produto rowsAffected: ra"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := img01Mock(t)
			e := mock.ExpectExec(q).WithArgs(c.arg, int64(5))
			if c.err != nil {
				e.WillReturnError(c.err)
			} else {
				e.WillReturnResult(c.res)
			}
			err := catalogomysql.NovoProdutos(db).DefinirImagem(context.Background(), 5, c.valor)
			switch {
			case c.errIs != nil:
				require.ErrorIs(t, err, c.errIs)
			case c.errS != "":
				require.EqualError(t, err, c.errS)
			default:
				require.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestIMG01_Repo_PorID_LeImagem(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	agora := time.Now()
	cols := []string{"id", "sku", "descricao", "categoria", "marca", "nota_olfativa", "preco_tabela", "custo_unitario",
		"unidade", "data_lancamento", "ativo", "imagem", "created_at", "updated_at"}
	mock.ExpectQuery(`COALESCE\(imagem, ''\).+FROM produtos WHERE id = \?`).WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(5), "SKU-1", "D", "C", "M", "", 1.0, 1.0, "UN", nil, true, img01Nome, agora, agora))
	p, err := catalogomysql.NovoProdutos(db).PorID(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, img01Nome, p.Imagem)
	assert.NoError(t, mock.ExpectationsWereMet())
}
