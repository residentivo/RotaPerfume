package visitas_test

// TST-03 (Lote 7): parsing, lookup, upsert e Run do importador de visitas,
// com CSV em t.TempDir() e banco em sqlmock.

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
	"github.com/rotaperfumes/shared/importers/clientesdedup"
	"github.com/rotaperfumes/shared/importers/visitas"
)

const (
	headerVisitas = "visita_id,cliente_id,vendedor_id,data_visita,resultado,duracao_min\n"
	// 3001 é cópia (mesmo CNPJ, com máscara) do cliente 1.
	clientesCSVUnificacao = "cliente_id,cnpj\n1,11222333000181\n3001,11.222.333/0001-81\n"
)

func TestParseRow(t *testing.T) {
	casos := []struct {
		nome   string
		rec    []string
		want   visitas.Row
		errSub string
	}{
		{
			nome: "ISO",
			rec:  []string{"1", "5", "7", "2024-01-02", "Pedido fechado", "45"},
			want: visitas.Row{VisitaID: 1, ClienteIDOrigem: 5, VendedorID: 7, DataVisita: time.Date(2024, 1, 2, 0, 0, 0, 0, time.Local), Resultado: "Pedido fechado", DuracaoMin: 45},
		},
		{
			nome: "BR com espaços",
			rec:  []string{" 2 ", " 5 ", " 7 ", " 03/01/2024 ", " Sem interesse ", " 10 "},
			want: visitas.Row{VisitaID: 2, ClienteIDOrigem: 5, VendedorID: 7, DataVisita: time.Date(2024, 1, 3, 0, 0, 0, 0, time.Local), Resultado: "Sem interesse", DuracaoMin: 10},
		},
		{nome: "visita_id inválido", rec: []string{"a", "5", "7", "2024-01-02", "R", "1"}, errSub: "visita_id inválido"},
		{nome: "cliente inválido", rec: []string{"1", "", "7", "2024-01-02", "R", "1"}, errSub: "cliente_id inválido"},
		{nome: "vendedor inválido", rec: []string{"1", "5", "-", "2024-01-02", "R", "1"}, errSub: "vendedor_id inválido"},
		{nome: "data inválida", rec: []string{"1", "5", "7", "02-01-2024", "R", "1"}, errSub: "data_visita inválida"},
		{nome: "resultado vazio", rec: []string{"1", "5", "7", "2024-01-02", "  ", "1"}, errSub: "resultado vazio"},
		{nome: "duração inválida", rec: []string{"1", "5", "7", "2024-01-02", "R", "1h"}, errSub: "duracao_min inválido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := visitas.ParseRow(c.rec)
			if c.errSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.errSub)
				return
			}
			require.NoError(t, err)
			assert.True(t, c.want.DataVisita.Equal(got.DataVisita))
			got.DataVisita, c.want.DataVisita = time.Time{}, time.Time{}
			assert.Equal(t, c.want, got)
		})
	}
}

func TestReadCSV(t *testing.T) {
	casos := []struct {
		nome       string
		csv        string
		wantIDs    []int64
		wantErrs   int
		wantErrSub string
	}{
		{nome: "vazio", csv: "", wantErrSub: "lendo cabeçalho"},
		{nome: "só cabeçalho", csv: headerVisitas},
		{
			nome: "válidas e inválidas",
			csv: headerVisitas +
				"1,5,7,2024-01-02,Pedido,30\n" +
				"2,5,7,2024-01-02,Pedido\n" + // colunas a menos
				"3,5,7,2024-01-02,,30\n" + // resultado vazio
				"4,6,8,04/01/2024,Retorno,15\n",
			wantIDs:  []int64{1, 4},
			wantErrs: 2,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			rows, parseErrs, err := visitas.ReadCSV(strings.NewReader(c.csv))
			if c.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErrSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantErrs, parseErrs)
			var ids []int64
			for _, r := range rows {
				ids = append(ids, r.VisitaID)
			}
			assert.Equal(t, c.wantIDs, ids)
		})
	}
}

func TestReadCSVFile_Inexistente(t *testing.T) {
	_, _, err := visitas.ReadCSVFile(filepath.Join(t.TempDir(), "nada.csv"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abrindo arquivo")
}

func TestResolveCSVPath(t *testing.T) {
	t.Run("flag", func(t *testing.T) {
		t.Setenv(visitas.EnvCSVPath, "/env/v.csv")
		got, err := visitas.ResolveCSVPath("/flag/v.csv")
		require.NoError(t, err)
		assert.Equal(t, "/flag/v.csv", got)
	})
	t.Run("env", func(t *testing.T) {
		t.Setenv(visitas.EnvCSVPath, "/env/v.csv")
		got, err := visitas.ResolveCSVPath("")
		require.NoError(t, err)
		assert.Equal(t, "/env/v.csv", got)
	})
	t.Run("default em dados/crm", func(t *testing.T) {
		t.Setenv(visitas.EnvCSVPath, "")
		got, err := visitas.ResolveCSVPath("")
		require.NoError(t, err)
		root, err := cmdutil.FindProjectRoot()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "dados", "crm", "visitas.csv"), got)
	})
	t.Run("fora do repositório", func(t *testing.T) {
		t.Setenv(visitas.EnvCSVPath, "")
		chdir(t, t.TempDir())
		_, err := visitas.ResolveCSVPath("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "raiz do projeto")
	})
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
			got, err := visitas.LoadClienteIDsByOrigem(db)
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
	d := time.Date(2024, 1, 2, 0, 0, 0, 0, time.Local)
	rows := []visitas.Row{
		{VisitaID: 1, ClienteIDOrigem: 5, VendedorID: 7, DataVisita: d, Resultado: "Pedido", DuracaoMin: 30},
		{VisitaID: 2, ClienteIDOrigem: 999, VendedorID: 7, DataVisita: d, Resultado: "Pedido", DuracaoMin: 30},
		{VisitaID: 3, ClienteIDOrigem: 5, VendedorID: 7, DataVisita: d, Resultado: "Retorno", DuracaoMin: 5},
		{VisitaID: 4, ClienteIDOrigem: 5, VendedorID: 404, DataVisita: d, Resultado: "Retorno", DuracaoMin: 5},
		{VisitaID: 5, ClienteIDOrigem: 5, VendedorID: 7, DataVisita: d, Resultado: "Retorno", DuracaoMin: 5},
	}
	mock.ExpectPrepare("INSERT INTO visitas")
	mock.ExpectExec("INSERT INTO visitas").WithArgs(int64(1), int64(5), int64(7), "2024-01-02", "Pedido", int64(30)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO visitas").WithArgs(int64(3), int64(5), int64(7), "2024-01-02", "Retorno", int64(5)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO visitas").WithArgs(int64(4), int64(5), int64(404), "2024-01-02", "Retorno", int64(5)).WillReturnError(errBanco)
	mock.ExpectExec("INSERT INTO visitas").WithArgs(int64(5), int64(5), int64(7), "2024-01-02", "Retorno", int64(5)).WillReturnResult(sqlmock.NewResult(0, 0))

	ins, upd, fail, err := visitas.UpsertAll(db, rows, map[int64]int64{5: 5})
	require.NoError(t, err)
	assert.Equal(t, 1, ins)
	assert.Equal(t, 2, upd)
	assert.Equal(t, 2, fail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertAll_PrepareFalha(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectPrepare("INSERT INTO visitas").WillReturnError(errBanco)
	_, _, _, err := visitas.UpsertAll(db, nil, nil)
	require.ErrorIs(t, err, errBanco)
}

const csvRun = headerVisitas +
	"1,1,7,2024-01-02,Pedido,30\n" +
	"2,3001,7,2024-01-03,Pedido,20\n" + // cópia do cliente 1
	"3,999,7,2024-01-04,Pedido,10\n" + // cliente inexistente
	"4,1,7,2024-01-05,Pedido,x\n" // duração inválida

func TestRun_DryRun(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "v.csv", csvRun)
	require.NoError(t, visitas.Run(visitas.Options{CSVFlag: path, DryRun: true}, openerProibido(t)))
	assert.Contains(t, logs.String(), "3 linhas válidas lidas, 1 linhas com erro de parsing")
	assert.Contains(t, logs.String(), "--dry-run informado")
}

func TestRun_ImportaComUnificacao(t *testing.T) {
	logs := capturarLog(t)
	dir := t.TempDir()
	t.Setenv(clientesdedup.EnvCSVPath, escreverArquivo(t, dir, "clientes.csv", clientesCSVUnificacao))
	path := escreverArquivo(t, dir, "v.csv", csvRun)

	db, mock := novoMock(t)
	mock.ExpectPing()
	mock.ExpectQuery("SELECT cliente_id_origem FROM clientes").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectPrepare("INSERT INTO visitas")
	mock.ExpectExec("INSERT INTO visitas").WithArgs(int64(1), int64(1), int64(7), "2024-01-02", "Pedido", int64(30)).WillReturnResult(sqlmock.NewResult(0, 1))
	// NEG-01: a visita do cliente 3001 (cópia) vai para o sobrevivente 1.
	mock.ExpectExec("INSERT INTO visitas").WithArgs(int64(2), int64(1), int64(7), "2024-01-03", "Pedido", int64(20)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectClose()

	n := 0
	require.NoError(t, visitas.Run(visitas.Options{CSVFlag: path}, openerDe(db, &n)))
	assert.Equal(t, 1, n)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, logs.String(), "OK — lidos=3 inseridos_ou_inalterados=1 atualizados=1 erros_upsert=1 erros_parsing=1")
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
				m.ExpectPrepare("INSERT INTO visitas").WillReturnError(errBanco)
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
				path = escreverArquivo(t, dir, "v.csv", headerVisitas)
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
			err := visitas.Run(visitas.Options{CSVFlag: path}, open)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errSub)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRun_ResolveCSVPathFalha(t *testing.T) {
	t.Setenv(visitas.EnvCSVPath, "")
	chdir(t, t.TempDir())
	err := visitas.Run(visitas.Options{DryRun: true}, openerProibido(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}
