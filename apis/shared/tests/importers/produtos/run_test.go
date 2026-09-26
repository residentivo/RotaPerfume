package produtos_test

// TST-03 (Lote 7): leitura do CSV, upsert e Run do importador de produtos,
// com CSV em t.TempDir() e banco em sqlmock.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/produtos"
)

const headerProdutos = "sku,descricao,categoria,marca,nota_olfativa,preco_tabela,custo_unitario,unidade,ativo,data_lancamento\n"

func TestReadCSV(t *testing.T) {
	casos := []struct {
		nome       string
		csv        string
		wantSKUs   []string
		wantErrs   int
		wantErrSub string
	}{
		{nome: "vazio", csv: "", wantErrSub: "lendo cabeçalho"},
		{nome: "só cabeçalho", csv: headerProdutos},
		{
			nome: "válidas e inválidas",
			csv: headerProdutos +
				"SKU-1,Perfume A,Perfume,Marca,Floral,100,40,UN,S,2024-01-02\n" +
				"SKU-2,Perfume B\n" + // colunas a menos
				"SKU-3,Perfume C,Perfume,Marca,,abc,40,UN,S,\n" + // preço inválido
				"SKU-4,Perfume D,Perfume,Marca,,50,20,UN,,\n",
			wantSKUs: []string{"SKU-1", "SKU-4"},
			wantErrs: 2,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			rows, parseErrs, err := produtos.ReadCSV(strings.NewReader(c.csv))
			if c.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErrSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantErrs, parseErrs)
			var skus []string
			for _, r := range rows {
				skus = append(skus, r.SKU)
			}
			assert.Equal(t, c.wantSKUs, skus)
		})
	}
}

func TestReadCSVFile_Inexistente(t *testing.T) {
	_, _, err := produtos.ReadCSVFile(filepath.Join(t.TempDir(), "nada.csv"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abrindo arquivo")
}

func TestResolveCSVPath_DefaultEForaDoRepositorio(t *testing.T) {
	t.Setenv(produtos.EnvCSVPath, "")
	got, err := produtos.ResolveCSVPath("")
	require.NoError(t, err)
	root, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "dados", "erp", "produtos.csv"), got)

	chdir(t, t.TempDir())
	_, err = produtos.ResolveCSVPath("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}

func TestUpsertAll(t *testing.T) {
	capturarLog(t)
	db, mock := novoMock(t)
	lanc := time.Date(2024, 1, 2, 0, 0, 0, 0, time.Local)
	rows := []produtos.Row{
		{SKU: "SKU-1", Descricao: "A", Categoria: "Perfume", Marca: "M", NotaOlfativa: "Floral", PrecoTabela: 100, CustoUnitario: 40, Unidade: "UN", Ativo: true, DataLancamento: &lanc},
		{SKU: "SKU-2", Descricao: "B", Categoria: "Perfume", Marca: "M", PrecoTabela: 50, CustoUnitario: 20, Unidade: "UN", Ativo: false},
		{SKU: "SKU-3", Descricao: "C", Categoria: "Perfume", Marca: "M", PrecoTabela: 1, CustoUnitario: 1, Unidade: "UN", Ativo: true},
		{SKU: "SKU-4", Descricao: "D", Categoria: "Perfume", Marca: "M", PrecoTabela: 1, CustoUnitario: 1, Unidade: "UN", Ativo: true},
	}
	mock.ExpectPrepare("INSERT INTO produtos")
	mock.ExpectExec("INSERT INTO produtos").
		WithArgs("SKU-1", "A", "Perfume", "M", "Floral", 100.0, 40.0, "UN", "2024-01-02", true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// nota olfativa vazia e data_lancamento ausente vão como NULL.
	mock.ExpectExec("INSERT INTO produtos").
		WithArgs("SKU-2", "B", "Perfume", "M", nil, 50.0, 20.0, "UN", nil, false).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO produtos").WithArgs("SKU-3", "C", "Perfume", "M", nil, 1.0, 1.0, "UN", nil, true).WillReturnError(errBanco)
	mock.ExpectExec("INSERT INTO produtos").WithArgs("SKU-4", "D", "Perfume", "M", nil, 1.0, 1.0, "UN", nil, true).WillReturnResult(sqlmock.NewResult(0, 0))

	ins, upd, fail, err := produtos.UpsertAll(db, rows)
	require.NoError(t, err)
	assert.Equal(t, 1, ins)
	assert.Equal(t, 2, upd)
	assert.Equal(t, 1, fail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertAll_PrepareFalha(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectPrepare("INSERT INTO produtos").WillReturnError(errBanco)
	_, _, _, err := produtos.UpsertAll(db, nil)
	require.ErrorIs(t, err, errBanco)
	assert.Contains(t, err.Error(), "prepare falhou")
}

const csvRun = headerProdutos +
	"SKU-1,Perfume A,Perfume,Marca,Floral,100,40,UN,N,2024-01-02\n" +
	"SKU-2,Perfume B,Perfume,Marca,,50,20,UN,x,\n" + // ativo inválido = ativo
	"SKU-3,,Perfume,Marca,,50,20,UN,S,\n" // descrição vazia

func TestRun_DryRun(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "produtos.csv", csvRun)
	require.NoError(t, produtos.Run(produtos.Options{CSVFlag: path, DryRun: true}, openerProibido(t)))
	assert.Contains(t, logs.String(), "2 linhas válidas lidas, 1 linhas com erro de parsing")
	assert.Contains(t, logs.String(), "--dry-run informado")
}

func TestRun_Importa(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "produtos.csv", csvRun)
	db, mock := novoMock(t)
	mock.ExpectPing()
	mock.ExpectPrepare("INSERT INTO produtos")
	mock.ExpectExec("INSERT INTO produtos").
		WithArgs("SKU-1", "Perfume A", "Perfume", "Marca", "Floral", 100.0, 40.0, "UN", "2024-01-02", false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO produtos").
		WithArgs("SKU-2", "Perfume B", "Perfume", "Marca", nil, 50.0, 20.0, "UN", nil, true).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectClose()

	n := 0
	require.NoError(t, produtos.Run(produtos.Options{CSVFlag: path}, openerDe(db, &n)))
	assert.Equal(t, 1, n)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, logs.String(), "OK — lidos=2 inseridos_ou_inalterados=1 atualizados=1 erros_upsert=0 erros_parsing=1")
}

func TestRun_Erros(t *testing.T) {
	casos := []struct {
		nome   string
		semCSV bool
		mock   func(m sqlmock.Sqlmock)
		abrir  error
		errSub string
	}{
		{nome: "CSV inexistente", semCSV: true, errSub: "falha ao ler CSV"},
		{nome: "abrir falha", abrir: errBanco, errSub: errBanco.Error()},
		{nome: "ping falha", mock: func(m sqlmock.Sqlmock) { m.ExpectPing().WillReturnError(errBanco) }, errSub: "ping no banco falhou"},
		{
			nome: "prepare falha",
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectPrepare("INSERT INTO produtos").WillReturnError(errBanco)
			},
			errSub: "prepare falhou",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "inexistente.csv")
			if !c.semCSV {
				path = escreverArquivo(t, dir, "produtos.csv", headerProdutos)
			}
			db, mock := novoMock(t)
			if c.mock != nil {
				c.mock(mock)
			}
			open := func() (cmdutil.Conn, error) {
				if c.abrir != nil {
					return nil, c.abrir
				}
				return db, nil
			}
			err := produtos.Run(produtos.Options{CSVFlag: path}, open)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errSub)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRun_ResolveCSVPathFalha(t *testing.T) {
	t.Setenv(produtos.EnvCSVPath, "")
	chdir(t, t.TempDir())
	err := produtos.Run(produtos.Options{DryRun: true}, openerProibido(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}
