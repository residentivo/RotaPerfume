package estoque_test

// TST-03 (Lote 7): leitura do CSV, caminho do CSV, prepare e Run do
// importador de estoque, com CSV em t.TempDir() e banco em sqlmock.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/estoque"
)

const headerEstoque = "data_snapshot,sku,saldo,ruptura\n"

func TestReadCSV(t *testing.T) {
	casos := []struct {
		nome       string
		csv        string
		wantSKUs   []string
		wantErrs   int
		wantErrSub string
	}{
		{nome: "vazio", csv: "", wantErrSub: "lendo cabeçalho"},
		{nome: "só cabeçalho", csv: headerEstoque},
		{
			nome: "válidas e inválidas",
			csv: headerEstoque +
				"2024-06-01,SKU-1,10,S\n" +
				"2024-06-01,SKU-2\n" + // colunas a menos
				"01/06/2024,SKU-3,1,N\n" + // formato BR não é aceito no estoque
				"2024-06-01,SKU-4,0,\n",
			wantSKUs: []string{"SKU-1", "SKU-4"},
			wantErrs: 2,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			rows, parseErrs, err := estoque.ReadCSV(strings.NewReader(c.csv))
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
			if len(rows) == 2 {
				assert.True(t, rows[0].Ruptura)
				assert.False(t, rows[1].Ruptura, "ruptura vazia = sem ruptura")
			}
		})
	}
}

func TestReadCSVFile_Inexistente(t *testing.T) {
	_, _, err := estoque.ReadCSVFile(filepath.Join(t.TempDir(), "nada.csv"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abrindo arquivo")
}

func TestResolveCSVPath_DefaultEForaDoRepositorio(t *testing.T) {
	t.Setenv(estoque.EnvCSVPath, "")
	got, err := estoque.ResolveCSVPath("")
	require.NoError(t, err)
	root, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "dados", "erp", "estoque.csv"), got)

	chdir(t, t.TempDir())
	_, err = estoque.ResolveCSVPath("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}

func TestUpsertAll_PrepareFalha(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectPrepare("INSERT INTO estoque").WillReturnError(errBanco)
	_, _, _, err := estoque.UpsertAll(db, nil)
	require.ErrorIs(t, err, errBanco)
	assert.Contains(t, err.Error(), "prepare falhou")
}

const csvRun = headerEstoque +
	"2024-06-01,SKU-1,10,S\n" +
	"2024-06-01,SKU-2,5,n\n" +
	"2024-06-01,,5,N\n" // sku vazio

func TestRun_DryRun(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "estoque.csv", csvRun)
	require.NoError(t, estoque.Run(estoque.Options{CSVFlag: path, DryRun: true}, openerProibido(t)))
	assert.Contains(t, logs.String(), "2 linhas válidas lidas, 1 linhas com erro de parsing")
	assert.Contains(t, logs.String(), "--dry-run informado")
}

func TestRun_Importa(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "estoque.csv", csvRun)
	db, mock := novoMock(t)
	mock.ExpectPing()
	mock.ExpectPrepare("INSERT INTO estoque")
	mock.ExpectExec("INSERT INTO estoque").WithArgs("2024-06-01", "SKU-1", 10, true).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO estoque").WithArgs("2024-06-01", "SKU-2", 5, false).WillReturnError(errBanco) // FK de sku
	mock.ExpectClose()

	n := 0
	require.NoError(t, estoque.Run(estoque.Options{CSVFlag: path}, openerDe(db, &n)))
	assert.Equal(t, 1, n)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, logs.String(), "OK — lidos=2 inseridos_ou_inalterados=1 atualizados=0 erros_upsert=1 erros_parsing=1")
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
				m.ExpectPrepare("INSERT INTO estoque").WillReturnError(errBanco)
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
				path = escreverArquivo(t, dir, "estoque.csv", headerEstoque)
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
			err := estoque.Run(estoque.Options{CSVFlag: path}, open)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errSub)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRun_ResolveCSVPathFalha(t *testing.T) {
	t.Setenv(estoque.EnvCSVPath, "")
	chdir(t, t.TempDir())
	err := estoque.Run(estoque.Options{DryRun: true}, openerProibido(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}
