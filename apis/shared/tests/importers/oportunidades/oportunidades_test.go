package oportunidades_test

// TST-03 (Lote 7): parsing, lookup, upsert e Run do importador de
// oportunidades, com CSV em t.TempDir() e banco em sqlmock.

import (
	"database/sql"
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
	"github.com/rotaperfumes/shared/importers/oportunidades"
)

const (
	headerOport = "oportunidade_id,cliente_id,vendedor_id,origem,data_abertura,etapa,probabilidade_pct,valor_estimado,data_fechamento,ciclo_dias,motivo_perda\n"
	// 3001 é cópia (mesmo CNPJ) do cliente 1.
	clientesCSVUnificacao = "cliente_id,cnpj\n1,11222333000181\n3001,11222333000181\n"
)

func dia(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.Local) }

func TestParseRow(t *testing.T) {
	fech := dia(2024, 2, 1)
	casos := []struct {
		nome   string
		rec    []string
		want   oportunidades.Row
		errSub string
	}{
		{
			nome: "fechada: todos os opcionais preenchidos",
			rec:  []string{"1", "5", "7", "Indicação", "2024-01-02", "Perdida", "0", "1500.5", "01/02/2024", "30", "Preço"},
			want: oportunidades.Row{OportunidadeID: 1, ClienteIDOrigem: 5, VendedorID: 7, Origem: "Indicação", DataAbertura: dia(2024, 1, 2), Etapa: "Perdida",
				ProbabilidadePct: 0, ValorEstimado: 1500.5, DataFechamento: &fech, CicloDias: sql.NullInt64{Int64: 30, Valid: true}, MotivoPerda: sql.NullString{String: "Preço", Valid: true}},
		},
		{
			nome: "aberta: opcionais vazios viram NULL",
			rec:  []string{" 2 ", " 5 ", " 7 ", " Site ", " 02/01/2024 ", " Proposta ", " 60 ", " 900 ", "", " ", ""},
			want: oportunidades.Row{OportunidadeID: 2, ClienteIDOrigem: 5, VendedorID: 7, Origem: "Site", DataAbertura: dia(2024, 1, 2), Etapa: "Proposta", ProbabilidadePct: 60, ValorEstimado: 900},
		},
		{nome: "id inválido", rec: []string{"x", "5", "7", "S", "2024-01-02", "E", "0", "0", "", "", ""}, errSub: "oportunidade_id inválido"},
		{nome: "cliente inválido", rec: []string{"1", "x", "7", "S", "2024-01-02", "E", "0", "0", "", "", ""}, errSub: "cliente_id inválido"},
		{nome: "vendedor inválido", rec: []string{"1", "5", "x", "S", "2024-01-02", "E", "0", "0", "", "", ""}, errSub: "vendedor_id inválido"},
		{nome: "origem vazia", rec: []string{"1", "5", "7", " ", "2024-01-02", "E", "0", "0", "", "", ""}, errSub: "origem vazia"},
		{nome: "abertura inválida", rec: []string{"1", "5", "7", "S", "2024.01.02", "E", "0", "0", "", "", ""}, errSub: "data_abertura inválida"},
		{nome: "etapa vazia", rec: []string{"1", "5", "7", "S", "2024-01-02", "", "0", "0", "", "", ""}, errSub: "etapa vazia"},
		{nome: "probabilidade inválida", rec: []string{"1", "5", "7", "S", "2024-01-02", "E", "%", "0", "", "", ""}, errSub: "probabilidade_pct inválido"},
		{nome: "valor inválido", rec: []string{"1", "5", "7", "S", "2024-01-02", "E", "0", "R$", "", "", ""}, errSub: "valor_estimado inválido"},
		{nome: "fechamento inválido", rec: []string{"1", "5", "7", "S", "2024-01-02", "E", "0", "0", "31/02/2024", "", ""}, errSub: "data_fechamento inválida"},
		{nome: "ciclo inválido", rec: []string{"1", "5", "7", "S", "2024-01-02", "E", "0", "0", "", "3.5", ""}, errSub: "ciclo_dias inválido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := oportunidades.ParseRow(c.rec)
			if c.errSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.errSub)
				return
			}
			require.NoError(t, err)
			assert.True(t, c.want.DataAbertura.Equal(got.DataAbertura))
			if c.want.DataFechamento == nil {
				assert.Nil(t, got.DataFechamento)
			} else {
				require.NotNil(t, got.DataFechamento)
				assert.True(t, c.want.DataFechamento.Equal(*got.DataFechamento))
			}
			got.DataAbertura, c.want.DataAbertura = time.Time{}, time.Time{}
			got.DataFechamento, c.want.DataFechamento = nil, nil
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
		{nome: "só cabeçalho", csv: headerOport},
		{
			nome: "válidas e inválidas",
			csv: headerOport +
				"1,5,7,Site,2024-01-02,Proposta,60,900,,,\n" +
				"2,5,7,Site,2024-01-02\n" + // colunas a menos
				"3,5,7,,2024-01-02,Proposta,60,900,,,\n" + // origem vazia
				"4,6,8,Feira,02/01/2024,Ganha,100,50,10/01/2024,8,\n",
			wantIDs:  []int64{1, 4},
			wantErrs: 2,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			rows, parseErrs, err := oportunidades.ReadCSV(strings.NewReader(c.csv))
			if c.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErrSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantErrs, parseErrs)
			var ids []int64
			for _, r := range rows {
				ids = append(ids, r.OportunidadeID)
			}
			assert.Equal(t, c.wantIDs, ids)
		})
	}
}

func TestReadCSVFile_Inexistente(t *testing.T) {
	_, _, err := oportunidades.ReadCSVFile(filepath.Join(t.TempDir(), "nada.csv"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abrindo arquivo")
}

func TestResolveCSVPath(t *testing.T) {
	t.Run("flag", func(t *testing.T) {
		t.Setenv(oportunidades.EnvCSVPath, "/env/o.csv")
		got, err := oportunidades.ResolveCSVPath("/flag/o.csv")
		require.NoError(t, err)
		assert.Equal(t, "/flag/o.csv", got)
	})
	t.Run("env", func(t *testing.T) {
		t.Setenv(oportunidades.EnvCSVPath, "/env/o.csv")
		got, err := oportunidades.ResolveCSVPath("")
		require.NoError(t, err)
		assert.Equal(t, "/env/o.csv", got)
	})
	t.Run("default em dados/crm", func(t *testing.T) {
		t.Setenv(oportunidades.EnvCSVPath, "")
		got, err := oportunidades.ResolveCSVPath("")
		require.NoError(t, err)
		root, err := cmdutil.FindProjectRoot()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "dados", "crm", "oportunidades.csv"), got)
	})
	t.Run("fora do repositório", func(t *testing.T) {
		t.Setenv(oportunidades.EnvCSVPath, "")
		chdir(t, t.TempDir())
		_, err := oportunidades.ResolveCSVPath("")
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
			got, err := oportunidades.LoadClienteIDsByOrigem(db)
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
	fech := dia(2024, 2, 1)
	rows := []oportunidades.Row{
		{OportunidadeID: 1, ClienteIDOrigem: 5, VendedorID: 7, Origem: "Site", DataAbertura: dia(2024, 1, 2), Etapa: "Perdida", ProbabilidadePct: 0, ValorEstimado: 10,
			DataFechamento: &fech, CicloDias: sql.NullInt64{Int64: 30, Valid: true}, MotivoPerda: sql.NullString{String: "Preço", Valid: true}},
		{OportunidadeID: 2, ClienteIDOrigem: 999, VendedorID: 7, Origem: "Site", DataAbertura: dia(2024, 1, 2), Etapa: "Proposta"},
		{OportunidadeID: 3, ClienteIDOrigem: 5, VendedorID: 7, Origem: "Site", DataAbertura: dia(2024, 1, 3), Etapa: "Proposta", ProbabilidadePct: 50, ValorEstimado: 20},
		{OportunidadeID: 4, ClienteIDOrigem: 5, VendedorID: 404, Origem: "Site", DataAbertura: dia(2024, 1, 3), Etapa: "Proposta"},
		{OportunidadeID: 5, ClienteIDOrigem: 5, VendedorID: 7, Origem: "Site", DataAbertura: dia(2024, 1, 3), Etapa: "Proposta"},
	}
	mock.ExpectPrepare("INSERT INTO oportunidades")
	mock.ExpectExec("INSERT INTO oportunidades").
		WithArgs(int64(1), int64(5), int64(7), "Site", "2024-01-02", "Perdida", 0.0, 10.0, "2024-02-01", int64(30), "Preço").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// oportunidade 3: aberta, opcionais vão como NULL.
	mock.ExpectExec("INSERT INTO oportunidades").
		WithArgs(int64(3), int64(5), int64(7), "Site", "2024-01-03", "Proposta", 50.0, 20.0, nil, nil, nil).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO oportunidades").WithArgs(int64(4), int64(5), int64(404), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), nil, nil, nil).
		WillReturnError(errBanco) // FK de vendedor
	mock.ExpectExec("INSERT INTO oportunidades").WithArgs(int64(5), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), nil, nil, nil).
		WillReturnResult(sqlmock.NewResult(0, 0))

	ins, upd, fail, err := oportunidades.UpsertAll(db, rows, map[int64]int64{5: 5})
	require.NoError(t, err)
	assert.Equal(t, 1, ins)
	assert.Equal(t, 2, upd)
	assert.Equal(t, 2, fail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertAll_PrepareFalha(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectPrepare("INSERT INTO oportunidades").WillReturnError(errBanco)
	_, _, _, err := oportunidades.UpsertAll(db, nil, nil)
	require.ErrorIs(t, err, errBanco)
	assert.Contains(t, err.Error(), "prepare falhou")
}

const csvRun = headerOport +
	"1,1,7,Site,2024-01-02,Proposta,60,900,,,\n" +
	"2,3001,7,Site,2024-01-03,Proposta,60,900,,,\n" + // cópia do cliente 1
	"3,999,7,Site,2024-01-04,Proposta,60,900,,,\n" + // cliente inexistente
	"4,1,7,Site,2024-01-02,Proposta,60,900,,x,\n" // ciclo inválido

func TestRun_DryRun(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "o.csv", csvRun)
	require.NoError(t, oportunidades.Run(oportunidades.Options{CSVFlag: path, DryRun: true}, openerProibido(t)))
	assert.Contains(t, logs.String(), "3 linhas válidas lidas, 1 linhas com erro de parsing")
	assert.Contains(t, logs.String(), "--dry-run informado")
}

func TestRun_ImportaComUnificacao(t *testing.T) {
	logs := capturarLog(t)
	dir := t.TempDir()
	t.Setenv(clientesdedup.EnvCSVPath, escreverArquivo(t, dir, "clientes.csv", clientesCSVUnificacao))
	path := escreverArquivo(t, dir, "o.csv", csvRun)

	db, mock := novoMock(t)
	mock.ExpectPing()
	mock.ExpectQuery("SELECT cliente_id_origem FROM clientes").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectPrepare("INSERT INTO oportunidades")
	mock.ExpectExec("INSERT INTO oportunidades").WithArgs(int64(1), int64(1), int64(7), "Site", "2024-01-02", "Proposta", 60.0, 900.0, nil, nil, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// NEG-01: a oportunidade do cliente 3001 (cópia) vai para o sobrevivente 1.
	mock.ExpectExec("INSERT INTO oportunidades").WithArgs(int64(2), int64(1), int64(7), "Site", "2024-01-03", "Proposta", 60.0, 900.0, nil, nil, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectClose()

	n := 0
	require.NoError(t, oportunidades.Run(oportunidades.Options{CSVFlag: path}, openerDe(db, &n)))
	assert.Equal(t, 1, n)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, logs.String(), "1 cliente_id(s) de CNPJ duplicado redirecionados")
	assert.Contains(t, logs.String(), "OK — lidos=3 inseridos_ou_inalterados=2 atualizados=0 erros_upsert=1 erros_parsing=1")
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
				m.ExpectPrepare("INSERT INTO oportunidades").WillReturnError(errBanco)
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
				path = escreverArquivo(t, dir, "o.csv", headerOport)
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
			err := oportunidades.Run(oportunidades.Options{CSVFlag: path}, open)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errSub)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRun_ResolveCSVPathFalha(t *testing.T) {
	t.Setenv(oportunidades.EnvCSVPath, "")
	chdir(t, t.TempDir())
	err := oportunidades.Run(oportunidades.Options{DryRun: true}, openerProibido(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}
