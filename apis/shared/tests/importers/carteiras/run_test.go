package carteiras_test

// TST-03 (Lote 7): leitura do CSV, lookup, upsert e Run do importador de
// carteiras, com CSV em t.TempDir() e banco em sqlmock.

import (
	"database/sql/driver"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/carteiras"
	"github.com/rotaperfumes/shared/importers/clientesdedup"
)

const (
	headerCarteira = "carteira_id,cliente_id,vendedor_id,data_inicio,data_fim\n"
	// 3001 é cópia (mesmo CNPJ) do cliente 1.
	clientesCSVUnificacao = "cliente_id,cnpj\n1,11222333000181\n3001,11222333000181\n"
)

func TestReadCSV(t *testing.T) {
	casos := []struct {
		nome       string
		csv        string
		wantIDs    []int64
		wantErrs   int
		wantErrSub string
	}{
		{nome: "vazio", csv: "", wantErrSub: "lendo cabeçalho"},
		{nome: "só cabeçalho", csv: headerCarteira, wantIDs: []int64{}},
		{
			nome: "válidas e inválidas",
			csv: headerCarteira +
				"1,5,7,2024-01-02,\n" +
				"2,5,7\n" + // colunas a menos
				"3,5,7,2024-01-02,fim\n" + // data_fim inválida
				"4,6,8,02/01/2024,30/06/2024\n",
			wantIDs:  []int64{1, 4},
			wantErrs: 2,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			rows, parseErrs, err := carteiras.ReadCSV(strings.NewReader(c.csv))
			if c.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErrSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantErrs, parseErrs)
			assert.Equal(t, c.wantIDs, idsCarteira(rows))
		})
	}
}

func TestReadCSVFile(t *testing.T) {
	capturarLog(t)
	dir := t.TempDir()
	rows, _, err := carteiras.ReadCSVFile(escreverArquivo(t, dir, "c.csv", headerCarteira+"1,5,7,2024-01-02,2024-02-01\n"))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].DataFim)
	assert.Equal(t, "2024-02-01", rows[0].DataFim.Format("2006-01-02"))

	_, _, err = carteiras.ReadCSVFile(filepath.Join(dir, "nada.csv"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abrindo arquivo")
}

func TestResolveCSVPath_DefaultEForaDoRepositorio(t *testing.T) {
	t.Setenv(carteiras.EnvCSVPath, "")
	got, err := carteiras.ResolveCSVPath("")
	require.NoError(t, err)
	root, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "dados", "crm", "carteira.csv"), got)

	chdir(t, t.TempDir())
	_, err = carteiras.ResolveCSVPath("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}

func TestLoadClienteIDsByOrigem(t *testing.T) {
	casos := []struct {
		nome   string
		rows   *sqlmock.Rows
		qErr   error
		want   map[int64]int64
		falhar bool
	}{
		{nome: "ok", rows: sqlmock.NewRows([]string{"c"}).AddRow(1).AddRow(2), want: map[int64]int64{1: 1, 2: 2}},
		{nome: "erro na query", qErr: errBanco, falhar: true},
		{nome: "erro no scan", rows: sqlmock.NewRows([]string{"c"}).AddRow(driver.Value("x")), falhar: true},
		{nome: "erro na iteração", rows: sqlmock.NewRows([]string{"c"}).AddRow(1).RowError(0, errBanco), falhar: true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := novoMock(t)
			e := mock.ExpectQuery("SELECT cliente_id_origem FROM clientes")
			if c.qErr != nil {
				e.WillReturnError(c.qErr)
			} else {
				e.WillReturnRows(c.rows)
			}
			got, err := carteiras.LoadClienteIDsByOrigem(db)
			if c.falhar {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestUpsertAll(t *testing.T) {
	capturarLog(t)
	db, mock := novoMock(t)
	fim := time.Date(2024, 6, 30, 0, 0, 0, 0, time.Local)
	r1 := vinculo(1, 5, 7, "2024-01-02")
	r1.DataFim = &fim
	rows := []carteiras.Row{
		r1,
		vinculo(2, 999, 7, "2024-01-02"), // cliente fora do lookup
		vinculo(3, 5, 8, "2024-01-03"),
		vinculo(4, 5, 404, "2024-01-03"),
		vinculo(5, 5, 9, "2024-01-03"),
	}
	mock.ExpectPrepare("INSERT INTO carteiras")
	mock.ExpectExec("INSERT INTO carteiras").WithArgs(int64(1), int64(5), int64(7), "2024-01-02", "2024-06-30").WillReturnResult(sqlmock.NewResult(0, 1))
	// data_fim nil vai como NULL (vínculo ativo).
	mock.ExpectExec("INSERT INTO carteiras").WithArgs(int64(3), int64(5), int64(8), "2024-01-03", nil).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO carteiras").WithArgs(int64(4), int64(5), int64(404), "2024-01-03", nil).WillReturnError(errBanco)
	mock.ExpectExec("INSERT INTO carteiras").WithArgs(int64(5), int64(5), int64(9), "2024-01-03", nil).WillReturnResult(sqlmock.NewResult(0, 0))

	ins, upd, fail, err := carteiras.UpsertAll(db, rows, map[int64]int64{5: 5})
	require.NoError(t, err)
	assert.Equal(t, 1, ins)
	assert.Equal(t, 2, upd)
	assert.Equal(t, 2, fail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertAll_PrepareFalha(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectPrepare("INSERT INTO carteiras").WillReturnError(errBanco)
	_, _, _, err := carteiras.UpsertAll(db, nil, nil)
	require.ErrorIs(t, err, errBanco)
	assert.Contains(t, err.Error(), "prepare falhou")
}

const csvRun = headerCarteira +
	"1,1,7,2024-01-02,\n" + // sobrevivente
	"2,3001,7,2024-01-02,2024-03-01\n" + // cópia: vínculo equivalente ao 1 -> descartado
	"3,3001,8,2024-01-05,2024-06-01\n" + // cópia com outro vendedor -> redirecionado para 1
	"4,999,7,2024-01-02,\n" + // cliente inexistente
	"5,x,7,2024-01-02,\n" // erro de parsing

func TestRun_DryRun(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "c.csv", csvRun)
	require.NoError(t, carteiras.Run(carteiras.Options{CSVFlag: path, DryRun: true}, openerProibido(t)))
	assert.Contains(t, logs.String(), "4 linhas válidas lidas, 1 linhas com erro de parsing")
	assert.Contains(t, logs.String(), "--dry-run informado")
}

func TestRun_ImportaComUnificacaoEDescarte(t *testing.T) {
	logs := capturarLog(t)
	dir := t.TempDir()
	t.Setenv(clientesdedup.EnvCSVPath, escreverArquivo(t, dir, "clientes.csv", clientesCSVUnificacao))
	path := escreverArquivo(t, dir, "c.csv", csvRun)

	db, mock := novoMock(t)
	mock.ExpectPing()
	mock.ExpectQuery("SELECT cliente_id_origem FROM clientes").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectPrepare("INSERT INTO carteiras")
	mock.ExpectExec("INSERT INTO carteiras").WithArgs(int64(1), int64(1), int64(7), "2024-01-02", nil).WillReturnResult(sqlmock.NewResult(0, 1))
	// A carteira 2 não chega ao banco: sobrescreveria o data_fim do vínculo 1.
	mock.ExpectExec("INSERT INTO carteiras").WithArgs(int64(3), int64(1), int64(8), "2024-01-05", "2024-06-01").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectClose()

	n := 0
	require.NoError(t, carteiras.Run(carteiras.Options{CSVFlag: path}, openerDe(db, &n)))
	assert.Equal(t, 1, n)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, logs.String(), "carteira_id_origem=2 descartada")
	assert.Contains(t, logs.String(), "OK — lidos=4 descartados_unificacao=1 inseridos_ou_inalterados=2 atualizados=0 erros_upsert=1 erros_parsing=1")
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
			nome: "lookup falha",
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery("FROM clientes").WillReturnError(errBanco)
			},
			errSub: "falha ao carregar lookup de clientes",
		},
		{
			nome: "prepare falha",
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery("FROM clientes").WillReturnRows(sqlmock.NewRows([]string{"c"}))
				m.ExpectPrepare("INSERT INTO carteiras").WillReturnError(errBanco)
			},
			errSub: "prepare falhou",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			dir := t.TempDir()
			t.Setenv(clientesdedup.EnvCSVPath, filepath.Join(dir, "sem-clientes.csv"))
			path := filepath.Join(dir, "inexistente.csv")
			if !c.semCSV {
				path = escreverArquivo(t, dir, "c.csv", headerCarteira)
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
			err := carteiras.Run(carteiras.Options{CSVFlag: path}, open)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errSub)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRun_ResolveCSVPathFalha(t *testing.T) {
	t.Setenv(carteiras.EnvCSVPath, "")
	chdir(t, t.TempDir())
	err := carteiras.Run(carteiras.Options{DryRun: true}, openerProibido(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}
