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

	"github.com/rotaperfumes/shared/catalogo"
)

// ARQ-02: TestProdutoService_* exercitam catalogoapp (ListarProdutos,
// ObterProduto, AlternarAtivoProduto, CriarProduto, AtualizarProduto) com o
// adapter real sobre sqlmock; os nomes são identificadores históricos (R1).

func newProdutoTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

var produtoColunasRegex = `id, sku, descricao, categoria, marca, COALESCE\(nota_olfativa, ''\), preco_tabela, custo_unitario, unidade, data_lancamento, ativo, COALESCE\(imagem, ''\), created_at, updated_at`

func produtoColunasHeader() []string {
	return []string{
		"id", "sku", "descricao", "categoria", "marca", "nota_olfativa",
		"preco_tabela", "custo_unitario", "unidade", "data_lancamento", "ativo", "imagem", "created_at", "updated_at",
	}
}

func produtoRows() *sqlmock.Rows {
	return produtoRowsAtivo(true)
}

// produtoRowsAtivo é o produto 1 com o ativo informado (releitura do toggle).
func produtoRowsAtivo(ativo bool) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(produtoColunasHeader()).
		AddRow(int64(1), "SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
			99.90, 45.00, "UN", nil, ativo, "", now, now)
}

func emptyProdutoRows() *sqlmock.Rows {
	return sqlmock.NewRows(produtoColunasHeader())
}

// ---------------------------------------------------------------------------
// ListProdutos
// ---------------------------------------------------------------------------

func TestProdutoService_ListProdutos(t *testing.T) {
	testCases := []struct {
		nome    string
		filtro  catalogo.FiltroProduto
		page    int
		limit   int
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
		wantTot int
	}{
		{
			nome:   "sem filtros - sucesso",
			filtro: catalogo.FiltroProduto{},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT `+produtoColunasRegex+` FROM produtos ORDER BY id ASC LIMIT \? OFFSET \?`).
					WithArgs(20, 0).
					WillReturnRows(produtoRows())
			},
			wantLen: 1,
			wantTot: 1,
		},
		{
			nome:   "com filtros categoria/marca/ativo/q - repassa args ao repo",
			filtro: catalogo.FiltroProduto{Categoria: "Perfumaria", Marca: "Marca X", Ativo: boolPtr(true), Busca: "Teste"},
			page:   2,
			limit:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos WHERE categoria = \? AND marca = \? AND ativo = \? AND \(descricao LIKE \? OR sku LIKE \?\)`).
					WithArgs("Perfumaria", "Marca X", true, "%Teste%", "%Teste%").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
				mock.ExpectQuery(`SELECT `+produtoColunasRegex+` FROM produtos WHERE categoria = \? AND marca = \? AND ativo = \? AND \(descricao LIKE \? OR sku LIKE \?\) ORDER BY id ASC LIMIT \? OFFSET \?`).
					WithArgs("Perfumaria", "Marca X", true, "%Teste%", "%Teste%", 10, 10).
					WillReturnRows(produtoRows())
			},
			wantLen: 1,
			wantTot: 5,
		},
		{
			nome:   "erro no count do repo é propagado",
			filtro: catalogo.FiltroProduto{},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newProdutoTestDB(t)
			tc.mock(mock)

			casos := novosCasosCatalogo(db)
			produtos, total, err := itensETotal(casos.ListarProdutos(context.Background(), tc.filtro, kernel.NovaPagina(tc.page, tc.limit)))

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, produtos, tc.wantLen)
				assert.Equal(t, tc.wantTot, total)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetProdutoByID
// ---------------------------------------------------------------------------

func TestProdutoService_GetProdutoByID(t *testing.T) {
	t.Run("encontrado", func(t *testing.T) {
		db, mock := newProdutoTestDB(t)
		mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(produtoRows())

		casos := novosCasosCatalogo(db)
		p, err := casos.ObterProduto(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), p.ID)
		assert.Equal(t, "Perfume Teste", p.Descricao)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("não encontrado retorna ErrProdutoNaoEncontrado", func(t *testing.T) {
		db, mock := newProdutoTestDB(t)
		mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
			WithArgs(int64(999)).
			WillReturnRows(emptyProdutoRows())

		casos := novosCasosCatalogo(db)
		p, err := casos.ObterProduto(context.Background(), 999)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, catalogo.ErrProdutoNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro genérico do repo é propagado", func(t *testing.T) {
		db, mock := newProdutoTestDB(t)
		mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnError(sql.ErrConnDone)

		casos := novosCasosCatalogo(db)
		p, err := casos.ObterProduto(context.Background(), 1)
		assert.Nil(t, p)
		assert.Error(t, err)
		assert.False(t, err == catalogo.ErrProdutoNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// ToggleAtivoProduto
// ---------------------------------------------------------------------------

func TestProdutoService_ToggleAtivoProduto(t *testing.T) {
	selectProduto := `SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`

	t.Run("ativo nil - inverte no próprio UPDATE e relê (BUG-17)", func(t *testing.T) {
		db, mock := newProdutoTestDB(t)
		mock.ExpectExec(`UPDATE produtos SET ativo = NOT ativo WHERE id = \?`).
			WithArgs(int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(selectProduto).WithArgs(int64(1)).WillReturnRows(produtoRowsAtivo(false))

		casos := novosCasosCatalogo(db)
		p, err := casos.AlternarAtivoProduto(context.Background(), 1, nil)
		require.NoError(t, err)
		assert.False(t, p.Ativo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ativo explícito - define valor informado", func(t *testing.T) {
		db, mock := newProdutoTestDB(t)
		mock.ExpectExec(`UPDATE produtos SET ativo = \? WHERE id = \?`).
			WithArgs(true, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(selectProduto).WithArgs(int64(1)).WillReturnRows(produtoRowsAtivo(true))

		casos := novosCasosCatalogo(db)
		ativo := true
		p, err := casos.AlternarAtivoProduto(context.Background(), 1, &ativo)
		require.NoError(t, err)
		assert.True(t, p.Ativo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("produto não encontrado - UPDATE sem linha", func(t *testing.T) {
		db, mock := newProdutoTestDB(t)
		mock.ExpectExec(`UPDATE produtos SET ativo = NOT ativo WHERE id = \?`).
			WithArgs(int64(999)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		casos := novosCasosCatalogo(db)
		p, err := casos.AlternarAtivoProduto(context.Background(), 999, nil)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, catalogo.ErrProdutoNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("produto some entre o UPDATE e a releitura (corrida) - 404", func(t *testing.T) {
		db, mock := newProdutoTestDB(t)
		mock.ExpectExec(`UPDATE produtos SET ativo = NOT ativo WHERE id = \?`).
			WithArgs(int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(selectProduto).WithArgs(int64(1)).WillReturnRows(emptyProdutoRows())

		casos := novosCasosCatalogo(db)
		p, err := casos.AlternarAtivoProduto(context.Background(), 1, nil)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, catalogo.ErrProdutoNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// CreateProduto
// ---------------------------------------------------------------------------

func validProdutoInput() catalogo.DadosProduto {
	return catalogo.DadosProduto{
		SKU:            "SKU-001",
		Descricao:      "Perfume Teste",
		Categoria:      "Perfumaria",
		Marca:          "Marca X",
		NotaOlfativa:   "Cítrico",
		PrecoTabela:    99.90,
		CustoUnitario:  45.00,
		Unidade:        "UN",
		DataLancamento: "2024-01-15",
	}
}

func TestProdutoService_CreateProduto(t *testing.T) {
	testCases := []struct {
		nome      string
		input     func() catalogo.DadosProduto
		mock      func(mock sqlmock.Sqlmock)
		wantErr   error
		wantErrIs bool
	}{
		{
			nome:  "sucesso",
			input: validProdutoInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO produtos \(sku, descricao, categoria, marca, nota_olfativa, preco_tabela, custo_unitario, unidade, data_lancamento, ativo\)`).
					WithArgs("SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico", 99.90, 45.00, "UN", sqlmock.AnyArg(), true).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nome: "sku vazio",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.SKU = "   "
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrSKUObrigatorio,
			wantErrIs: true,
		},
		{
			nome: "descricao vazia",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.Descricao = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrDescricaoObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "categoria vazia",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.Categoria = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrCategoriaObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "marca vazia",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.Marca = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrMarcaObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "unidade vazia",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.Unidade = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrUnidadeObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "preco_tabela negativo",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.PrecoTabela = -1
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrPrecoTabelaInvalido,
			wantErrIs: true,
		},
		{
			nome: "custo_unitario negativo",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.CustoUnitario = -1
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrCustoUnitarioInvalido,
			wantErrIs: true,
		},
		{
			nome: "data_lancamento inválida",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.DataLancamento = "15/01/2024"
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrDataLancamentoInvalida,
			wantErrIs: true,
		},
		{
			nome: "data_lancamento vazia é opcional (sucesso)",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.DataLancamento = ""
				return in
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO produtos \(sku, descricao, categoria, marca, nota_olfativa, preco_tabela, custo_unitario, unidade, data_lancamento, ativo\)`).
					WithArgs("SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico", 99.90, 45.00, "UN", sqlmock.AnyArg(), true).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nome:  "erro no Create é propagado",
			input: validProdutoInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO produtos \(sku, descricao, categoria, marca, nota_olfativa, preco_tabela, custo_unitario, unidade, data_lancamento, ativo\)`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newProdutoTestDB(t)
			tc.mock(mock)

			casos := novosCasosCatalogo(db)
			p, err := casos.CriarProduto(context.Background(), tc.input())

			if tc.wantErr != nil {
				assert.Nil(t, p)
				if tc.wantErrIs {
					assert.ErrorIs(t, err, tc.wantErr)
				} else {
					assert.Error(t, err)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, int64(1), p.ID)
				assert.True(t, p.Ativo)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateProduto
// ---------------------------------------------------------------------------

func TestProdutoService_UpdateProduto(t *testing.T) {
	testCases := []struct {
		nome      string
		input     func() catalogo.DadosProduto
		mock      func(mock sqlmock.Sqlmock)
		wantErr   error
		wantErrIs bool
	}{
		{
			nome:  "sucesso",
			input: validProdutoInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE produtos\s+SET descricao = \?, categoria = \?, marca = \?, nota_olfativa = \?, preco_tabela = \?, custo_unitario = \?, unidade = \?, data_lancamento = \?\s+WHERE id = \?`).
					WithArgs("Perfume Teste", "Perfumaria", "Marca X", "Cítrico", 99.90, 45.00, "UN", sqlmock.AnyArg(), int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(produtoRows())
			},
		},
		{
			nome: "descricao vazia",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.Descricao = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrDescricaoObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "categoria vazia",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.Categoria = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrCategoriaObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "marca vazia",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.Marca = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrMarcaObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "unidade vazia",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.Unidade = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrUnidadeObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "preco_tabela negativo",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.PrecoTabela = -0.01
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrPrecoTabelaInvalido,
			wantErrIs: true,
		},
		{
			nome: "custo_unitario negativo",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.CustoUnitario = -0.01
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrCustoUnitarioInvalido,
			wantErrIs: true,
		},
		{
			nome: "data_lancamento inválida",
			input: func() catalogo.DadosProduto {
				in := validProdutoInput()
				in.DataLancamento = "2024-31-01"
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   catalogo.ErrDataLancamentoInvalida,
			wantErrIs: true,
		},
		{
			nome:  "id inexistente retorna ErrProdutoNaoEncontrado",
			input: validProdutoInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE produtos\s+SET descricao = \?, categoria = \?, marca = \?, nota_olfativa = \?, preco_tabela = \?, custo_unitario = \?, unidade = \?, data_lancamento = \?\s+WHERE id = \?`).
					WithArgs("Perfume Teste", "Perfumaria", "Marca X", "Cítrico", 99.90, 45.00, "UN", sqlmock.AnyArg(), int64(999)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr:   catalogo.ErrProdutoNaoEncontrado,
			wantErrIs: true,
		},
		{
			nome:  "erro no Update é propagado",
			input: validProdutoInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE produtos\s+SET descricao = \?, categoria = \?, marca = \?, nota_olfativa = \?, preco_tabela = \?, custo_unitario = \?, unidade = \?, data_lancamento = \?\s+WHERE id = \?`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
		{
			nome:  "erro no GetByID pós-update é propagado",
			input: validProdutoInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE produtos\s+SET descricao = \?, categoria = \?, marca = \?, nota_olfativa = \?, preco_tabela = \?, custo_unitario = \?, unidade = \?, data_lancamento = \?\s+WHERE id = \?`).
					WithArgs("Perfume Teste", "Perfumaria", "Marca X", "Cítrico", 99.90, 45.00, "UN", sqlmock.AnyArg(), int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newProdutoTestDB(t)
			tc.mock(mock)

			id := int64(1)
			if tc.nome == "id inexistente retorna ErrProdutoNaoEncontrado" {
				id = 999
			}

			casos := novosCasosCatalogo(db)
			p, err := casos.AtualizarProduto(context.Background(), id, tc.input())

			if tc.wantErr != nil {
				assert.Nil(t, p)
				if tc.wantErrIs {
					assert.ErrorIs(t, err, tc.wantErr)
				} else {
					assert.Error(t, err)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, int64(1), p.ID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
