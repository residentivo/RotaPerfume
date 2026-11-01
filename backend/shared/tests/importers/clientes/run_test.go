package clientes_test

// TST-03 (Lote 7): leitura do CSV, carga dos donos de CNPJ e Run do
// importador de clientes, com CSV em t.TempDir() e banco em sqlmock.

import (
	"database/sql/driver"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/clientes"
)

const headerClientes = "cliente_id,cnpj,razao_social,segmento,cidade,uf,bairro,data_cadastro,ativo\n"

func TestReadCSV(t *testing.T) {
	casos := []struct {
		nome       string
		csv        string
		wantIDs    []int64
		wantErrs   int
		wantErrSub string
	}{
		{nome: "vazio", csv: "", wantErrSub: "lendo cabeçalho"},
		{nome: "só cabeçalho", csv: headerClientes},
		{
			nome: "válidas e inválidas",
			csv: headerClientes +
				"1,11.222.333/0001-81,Loja A,Varejo,Curitiba,pr,Centro,2023-01-02,S\n" +
				"2,11222333000181,Loja B\n" + // colunas a menos
				"3,123,Loja C,Varejo,Curitiba,PR,Centro,2023-01-02,S\n" + // CNPJ fora do formato
				"4,11444777000161,Loja D,Atacado,Londrina,PR,,02/01/2023,n\n",
			wantIDs:  []int64{1, 4},
			wantErrs: 2,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			rows, parseErrs, err := clientes.ReadCSV(strings.NewReader(c.csv))
			if c.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErrSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantErrs, parseErrs)
			var ids []int64
			for _, r := range rows {
				ids = append(ids, r.ClienteIDOrigem)
			}
			assert.Equal(t, c.wantIDs, ids)
			if len(rows) > 0 {
				assert.Equal(t, "11222333000181", rows[0].CNPJ, "máscara removida")
				assert.Equal(t, "PR", rows[0].UF, "UF em maiúsculas")
				assert.False(t, rows[1].Ativo, "'n' minúsculo = inativo")
			}
		})
	}
}

func TestReadCSVFile_Inexistente(t *testing.T) {
	_, _, err := clientes.ReadCSVFile(filepath.Join(t.TempDir(), "nada.csv"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abrindo arquivo")
}

func TestResolveCSVPath_DefaultEForaDoRepositorio(t *testing.T) {
	t.Setenv(clientes.EnvCSVPath, "")
	got, err := clientes.ResolveCSVPath("")
	require.NoError(t, err)
	root, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "dados", "crm", "clientes.csv"), got)

	chdir(t, t.TempDir())
	_, err = clientes.ResolveCSVPath("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}

func TestLoadDonosCNPJ(t *testing.T) {
	t.Run("carrega cnpj -> cliente", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(reDonosCNPJ).WillReturnRows(sqlmock.NewRows([]string{"id", "cnpj"}).AddRow(1, "A").AddRow(2, "B"))
		d, err := clientes.LoadDonosCNPJ(db)
		require.NoError(t, err)
		id, ok := d.Dono("B")
		assert.True(t, ok)
		assert.Equal(t, int64(2), id)
		_, ok = d.Dono("C")
		assert.False(t, ok)
	})
	casos := []struct {
		nome string
		prep func(m sqlmock.Sqlmock)
	}{
		{"erro na query", func(m sqlmock.Sqlmock) { m.ExpectQuery(reDonosCNPJ).WillReturnError(errBanco) }},
		{"erro no scan", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reDonosCNPJ).WillReturnRows(sqlmock.NewRows([]string{"id", "cnpj"}).AddRow(driver.Value("x"), "A"))
		}},
		{"erro na iteração", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reDonosCNPJ).WillReturnRows(sqlmock.NewRows([]string{"id", "cnpj"}).AddRow(1, "A").RowError(0, errBanco))
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := novoMock(t)
			c.prep(mock)
			_, err := clientes.LoadDonosCNPJ(db)
			assert.Error(t, err)
		})
	}
}

func TestUpsertAll_ErrosFatais(t *testing.T) {
	t.Run("donos de CNPJ não carregam", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(reDonosCNPJ).WillReturnError(errBanco)
		_, err := clientes.UpsertAll(db, nil)
		require.ErrorIs(t, err, errBanco)
		assert.Contains(t, err.Error(), "falha ao carregar CNPJs existentes")
	})
	t.Run("prepare falha", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(reDonosCNPJ).WillReturnRows(sqlmock.NewRows([]string{"id", "cnpj"}))
		mock.ExpectPrepare(reUpsertCliente).WillReturnError(errBanco)
		_, err := clientes.UpsertAll(db, nil)
		require.ErrorIs(t, err, errBanco)
		assert.Contains(t, err.Error(), "prepare falhou")
	})
}

// csvRun: 3001 repete o CNPJ do cliente 1 (com máscara) e é unificado; a
// linha 5 tem data inválida.
const csvRun = headerClientes +
	"1,11222333000181,Loja A,Varejo,Curitiba,PR,Centro,2023-01-02,S\n" +
	"2,11444777000161,Loja B,Varejo,Curitiba,PR,Centro,2023-01-03,N\n" +
	"3001,11.222.333/0001-81,Loja A filial,Varejo,Curitiba,PR,Centro,2023-02-01,S\n" +
	"5,22333444000155,Loja E,Varejo,Curitiba,PR,Centro,ontem,S\n"

func TestRun_DryRun(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "clientes.csv", csvRun)
	require.NoError(t, clientes.Run(clientes.Options{CSVFlag: path, DryRun: true}, openerProibido(t)))
	assert.Contains(t, logs.String(), "3 linhas válidas lidas, 1 linhas com erro de parsing")
	assert.Contains(t, logs.String(), "1 linhas unificadas por CNPJ duplicado")
	assert.Contains(t, logs.String(), "2 clientes a gravar")
	assert.Contains(t, logs.String(), "--dry-run informado")
}

func TestRun_ImportaSemGravarCopias(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "clientes.csv", csvRun)
	db, mock := novoMock(t)
	mock.ExpectPing()
	mock.ExpectQuery(reDonosCNPJ).WillReturnRows(sqlmock.NewRows([]string{"id", "cnpj"}))
	mock.ExpectPrepare(reUpsertCliente)
	mock.ExpectExec(reUpsertCliente).
		WithArgs(int64(1), "11222333000181", "Loja A", "Varejo", "Curitiba", "PR", "Centro", "2023-01-02", true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(reUpsertCliente).
		WithArgs(int64(2), "11444777000161", "Loja B", "Varejo", "Curitiba", "PR", "Centro", "2023-01-03", false).
		WillReturnResult(sqlmock.NewResult(0, 2))
	// 3001 (cópia do CNPJ do cliente 1) nunca é enviada ao banco.
	mock.ExpectClose()

	n := 0
	require.NoError(t, clientes.Run(clientes.Options{CSVFlag: path}, openerDe(db, &n)))
	assert.Equal(t, 1, n)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, logs.String(), "OK — lidos=3 unificados=1 inseridos_ou_inalterados=1 atualizados=1 conflitos_cnpj=0 erros_upsert=0 erros_parsing=1")
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
			nome: "upsert falha ao carregar donos",
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery(reDonosCNPJ).WillReturnError(errBanco)
			},
			errSub: "falha ao carregar CNPJs existentes",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "inexistente.csv")
			if !c.semCSV {
				path = escreverArquivo(t, dir, "clientes.csv", headerClientes)
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
			err := clientes.Run(clientes.Options{CSVFlag: path}, open)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errSub)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRun_ResolveCSVPathFalha(t *testing.T) {
	t.Setenv(clientes.EnvCSVPath, "")
	chdir(t, t.TempDir())
	err := clientes.Run(clientes.Options{DryRun: true}, openerProibido(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}
