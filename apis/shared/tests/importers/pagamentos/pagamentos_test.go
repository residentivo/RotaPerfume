package pagamentos_test

// TST-03 (Lote 7): parsing, lookup, upsert e Run do importador de
// pagamentos, com CSV em t.TempDir() e banco em sqlmock.

import (
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/pagamentos"
)

const headerPagamentos = "pagamento_id,pedido_id,forma_pagamento,parcelas,valor,taxa_pct,valor_liquido,data_vencimento,data_pagamento,status_pagamento\n"

func linha(campos ...string) []string { return campos }

func TestParsePagamentoRow(t *testing.T) {
	casos := []struct {
		nome   string
		rec    []string
		want   pagamentos.Row
		errSub string
	}{
		{
			nome: "pago, com data de pagamento",
			rec:  linha("1", "10", "PIX", "1", "100", "0", "100", "2024-01-10", "2024-01-09", "Pago"),
			want: pagamentos.Row{PagamentoID: 1, PedidoIDOrigem: 10, FormaPagamento: "PIX", Parcelas: 1, Valor: 100, TaxaPct: 0, ValorLiquido: 100,
				DataVencimento: "2024-01-10", DataPagamento: sql.NullString{String: "2024-01-09", Valid: true}, StatusPagamento: "Pago"},
		},
		{
			nome: "em aberto: data_pagamento vazia vira NULL",
			rec:  linha(" 2 ", " 11 ", " Boleto 28 dias ", " 3 ", " 300.5 ", " 2.5 ", " 293 ", " 2024-02-28 ", "  ", " Em aberto "),
			want: pagamentos.Row{PagamentoID: 2, PedidoIDOrigem: 11, FormaPagamento: "Boleto 28 dias", Parcelas: 3, Valor: 300.5, TaxaPct: 2.5, ValorLiquido: 293,
				DataVencimento: "2024-02-28", StatusPagamento: "Em aberto"},
		},
		{nome: "pagamento_id inválido", rec: linha("", "10", "PIX", "1", "1", "0", "1", "2024-01-10", "", "Pago"), errSub: "pagamento_id inválido"},
		{nome: "pedido_id inválido", rec: linha("1", "p", "PIX", "1", "1", "0", "1", "2024-01-10", "", "Pago"), errSub: "pedido_id inválido"},
		{nome: "forma fora do ENUM", rec: linha("1", "10", "Pix", "1", "1", "0", "1", "2024-01-10", "", "Pago"), errSub: "forma_pagamento inválida"},
		{nome: "parcelas inválido", rec: linha("1", "10", "PIX", "um", "1", "0", "1", "2024-01-10", "", "Pago"), errSub: "parcelas inválido"},
		{nome: "valor inválido", rec: linha("1", "10", "PIX", "1", "x", "0", "1", "2024-01-10", "", "Pago"), errSub: "valor inválido"},
		{nome: "taxa inválida", rec: linha("1", "10", "PIX", "1", "1", "x", "1", "2024-01-10", "", "Pago"), errSub: "taxa_pct inválido"},
		{nome: "valor líquido inválido", rec: linha("1", "10", "PIX", "1", "1", "0", "x", "2024-01-10", "", "Pago"), errSub: "valor_liquido inválido"},
		{nome: "vencimento no formato BR não é aceito", rec: linha("1", "10", "PIX", "1", "1", "0", "1", "10/01/2024", "", "Pago"), errSub: "data_vencimento inválida"},
		{nome: "data_pagamento inválida", rec: linha("1", "10", "PIX", "1", "1", "0", "1", "2024-01-10", "2024-13-01", "Pago"), errSub: "data_pagamento inválida"},
		{nome: "status fora do ENUM", rec: linha("1", "10", "PIX", "1", "1", "0", "1", "2024-01-10", "", "Quitado"), errSub: "status_pagamento inválido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := pagamentos.ParsePagamentoRow(c.rec)
			if c.errSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.errSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestEnumsDePagamento(t *testing.T) {
	formas := []string{"Boleto 14 dias", "Boleto 28 dias", "Cartão de crédito", "Cartão de débito", "Cheque a prazo", "Dinheiro", "PIX"}
	for _, f := range formas {
		assert.True(t, pagamentos.IsValidFormaPagamento(f), f)
	}
	for _, f := range []string{"", "pix", "Boleto", "Cartao de credito"} {
		assert.False(t, pagamentos.IsValidFormaPagamento(f), f)
	}
	for _, s := range []string{"Em aberto", "Inadimplente", "Pago", "Pago com atraso"} {
		assert.True(t, pagamentos.IsValidStatusPagamento(s), s)
	}
	for _, s := range []string{"", "pago", "Cancelado"} {
		assert.False(t, pagamentos.IsValidStatusPagamento(s), s)
	}
}

func TestReadPagamentosCSV(t *testing.T) {
	casos := []struct {
		nome       string
		csv        string
		wantIDs    []int64
		wantErrs   int
		wantErrSub string
	}{
		{nome: "vazio", csv: "", wantErrSub: "lendo cabeçalho"},
		{nome: "só cabeçalho", csv: headerPagamentos},
		{
			nome: "válidas e inválidas",
			csv: headerPagamentos +
				"1,10,PIX,1,100,0,100,2024-01-10,2024-01-10,Pago\n" +
				"2,10,PIX,1,100,0,100,2024-01-10\n" + // colunas a menos
				"3,10,Fiado,1,100,0,100,2024-01-10,,Em aberto\n" + // forma inválida
				"4,11,Dinheiro,1,50,0,50,2024-01-11,,Inadimplente\n",
			wantIDs:  []int64{1, 4},
			wantErrs: 2,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			rows, parseErrs, err := pagamentos.ReadPagamentosCSV(strings.NewReader(c.csv))
			if c.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErrSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantErrs, parseErrs)
			var ids []int64
			for _, r := range rows {
				ids = append(ids, r.PagamentoID)
			}
			assert.Equal(t, c.wantIDs, ids)
		})
	}
}

func TestReadPagamentosCSVFile_Inexistente(t *testing.T) {
	_, _, err := pagamentos.ReadPagamentosCSVFile(filepath.Join(t.TempDir(), "nada.csv"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abrindo arquivo")
}

func TestResolveCSVPath(t *testing.T) {
	t.Run("flag", func(t *testing.T) {
		t.Setenv(pagamentos.EnvCSVPath, "/env/p.csv")
		got, err := pagamentos.ResolveCSVPath("/flag/p.csv", pagamentos.EnvCSVPath, "pagamentos.csv")
		require.NoError(t, err)
		assert.Equal(t, "/flag/p.csv", got)
	})
	t.Run("env", func(t *testing.T) {
		t.Setenv(pagamentos.EnvCSVPath, "/env/p.csv")
		got, err := pagamentos.ResolveCSVPath("", pagamentos.EnvCSVPath, "pagamentos.csv")
		require.NoError(t, err)
		assert.Equal(t, "/env/p.csv", got)
	})
	t.Run("default", func(t *testing.T) {
		t.Setenv(pagamentos.EnvCSVPath, "")
		got, err := pagamentos.ResolveCSVPath("", pagamentos.EnvCSVPath, "pagamentos.csv")
		require.NoError(t, err)
		root, err := cmdutil.FindProjectRoot()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "dados", "erp", "pagamentos.csv"), got)
	})
	t.Run("fora do repositório", func(t *testing.T) {
		t.Setenv(pagamentos.EnvCSVPath, "")
		chdir(t, t.TempDir())
		_, err := pagamentos.ResolveCSVPath("", pagamentos.EnvCSVPath, "pagamentos.csv")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "raiz do projeto")
	})
}

func TestLoadPedidoIDsByOrigem(t *testing.T) {
	const q = "SELECT pedido_id_origem FROM pedidos"
	casos := []struct {
		nome   string
		rows   *sqlmock.Rows
		qErr   error
		want   map[int64]int64
		falhar bool
	}{
		{nome: "ok", rows: sqlmock.NewRows([]string{"p"}).AddRow(10).AddRow(11), want: map[int64]int64{10: 10, 11: 11}},
		{nome: "vazio", rows: sqlmock.NewRows([]string{"p"}), want: map[int64]int64{}},
		{nome: "erro na query", qErr: errBanco, falhar: true},
		{nome: "erro no scan", rows: sqlmock.NewRows([]string{"p"}).AddRow(driver.Value("x")), falhar: true},
		{nome: "erro na iteração", rows: sqlmock.NewRows([]string{"p"}).AddRow(1).RowError(0, errBanco), falhar: true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := novoMock(t)
			e := mock.ExpectQuery(q)
			if c.qErr != nil {
				e.WillReturnError(c.qErr)
			} else {
				e.WillReturnRows(c.rows)
			}
			got, err := pagamentos.LoadPedidoIDsByOrigem(db)
			if c.falhar {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestUpsertPagamentos(t *testing.T) {
	capturarLog(t)
	db, mock := novoMock(t)
	rows := []pagamentos.Row{
		{PagamentoID: 1, PedidoIDOrigem: 10, FormaPagamento: "PIX", Parcelas: 1, Valor: 100, ValorLiquido: 100, DataVencimento: "2024-01-10",
			DataPagamento: sql.NullString{String: "2024-01-09", Valid: true}, StatusPagamento: "Pago"},
		{PagamentoID: 2, PedidoIDOrigem: 99, FormaPagamento: "PIX", Parcelas: 1, Valor: 1, ValorLiquido: 1, DataVencimento: "2024-01-10", StatusPagamento: "Pago"},
		{PagamentoID: 3, PedidoIDOrigem: 10, FormaPagamento: "Dinheiro", Parcelas: 1, Valor: 5, ValorLiquido: 5, DataVencimento: "2024-01-11", StatusPagamento: "Em aberto"},
		{PagamentoID: 4, PedidoIDOrigem: 10, FormaPagamento: "PIX", Parcelas: 1, Valor: 5, ValorLiquido: 5, DataVencimento: "2024-01-11", StatusPagamento: "Em aberto"},
		{PagamentoID: 5, PedidoIDOrigem: 10, FormaPagamento: "PIX", Parcelas: 1, Valor: 5, ValorLiquido: 5, DataVencimento: "2024-01-11", StatusPagamento: "Em aberto"},
	}
	mock.ExpectPrepare("INSERT INTO pagamentos")
	mock.ExpectExec("INSERT INTO pagamentos").
		WithArgs(int64(1), int64(10), "PIX", 1, 100.0, 0.0, 100.0, "2024-01-10", "2024-01-09", "Pago").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// pagamento 2: pedido 99 fora do lookup -> sem Exec. pagamento 3: data_pagamento NULL.
	mock.ExpectExec("INSERT INTO pagamentos").
		WithArgs(int64(3), int64(10), "Dinheiro", 1, 5.0, 0.0, 5.0, "2024-01-11", nil, "Em aberto").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO pagamentos").WithArgs(int64(4), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errBanco)
	mock.ExpectExec("INSERT INTO pagamentos").WithArgs(int64(5), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	ins, upd, fail, err := pagamentos.UpsertPagamentos(db, rows, map[int64]int64{10: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, ins)
	assert.Equal(t, 2, upd, "affected 2 e 0")
	assert.Equal(t, 2, fail, "pedido ausente + erro de Exec")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertPagamentos_PrepareFalha(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectPrepare("INSERT INTO pagamentos").WillReturnError(errBanco)
	_, _, _, err := pagamentos.UpsertPagamentos(db, nil, nil)
	require.ErrorIs(t, err, errBanco)
	assert.Contains(t, err.Error(), "prepare (pagamentos) falhou")
}

const csvRun = headerPagamentos +
	"1,10,PIX,1,100,0,100,2024-01-10,2024-01-09,Pago\n" +
	"2,77,PIX,1,100,0,100,2024-01-10,,Em aberto\n" + // pedido inexistente
	"3,10,PIX,1,100,0,100,2024-01-10,,Nada\n" // status inválido

func TestRun_DryRun(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "pagamentos.csv", csvRun)
	require.NoError(t, pagamentos.Run(pagamentos.Options{PagamentosCSVFlag: path, DryRun: true}, openerProibido(t)))
	assert.Contains(t, logs.String(), "2 linhas válidas lidas, 1 linhas com erro de parsing")
	assert.Contains(t, logs.String(), "--dry-run informado")
}

func TestRun_Importa(t *testing.T) {
	logs := capturarLog(t)
	path := escreverArquivo(t, t.TempDir(), "pagamentos.csv", csvRun)
	db, mock := novoMock(t)
	mock.ExpectPing()
	mock.ExpectQuery("SELECT pedido_id_origem FROM pedidos").WillReturnRows(sqlmock.NewRows([]string{"p"}).AddRow(10))
	mock.ExpectPrepare("INSERT INTO pagamentos")
	mock.ExpectExec("INSERT INTO pagamentos").
		WithArgs(int64(1), int64(10), "PIX", 1, 100.0, 0.0, 100.0, "2024-01-10", "2024-01-09", "Pago").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectClose()

	n := 0
	require.NoError(t, pagamentos.Run(pagamentos.Options{PagamentosCSVFlag: path}, openerDe(db, &n)))
	assert.Equal(t, 1, n)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, logs.String(), "concluído — lidos=2 inseridos_ou_inalterados=1 atualizados=0 erros_upsert=1 erros_parsing=1")
}

func TestRun_Erros(t *testing.T) {
	casos := []struct {
		nome   string
		csv    string // vazio = sem arquivo
		mock   func(m sqlmock.Sqlmock)
		abrir  error
		errSub string
	}{
		{nome: "CSV inexistente", errSub: "falha ao ler CSV de pagamentos"},
		{nome: "CSV sem cabeçalho", csv: "", errSub: "lendo cabeçalho"},
		{nome: "abrir falha", csv: headerPagamentos, abrir: errBanco, errSub: errBanco.Error()},
		{nome: "ping falha", csv: headerPagamentos, mock: func(m sqlmock.Sqlmock) { m.ExpectPing().WillReturnError(errBanco) }, errSub: "ping no banco falhou"},
		{
			nome: "lookup falha", csv: headerPagamentos,
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery("FROM pedidos").WillReturnError(errBanco)
			},
			errSub: "falha ao carregar pedidos",
		},
		{
			nome: "prepare falha", csv: headerPagamentos,
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery("FROM pedidos").WillReturnRows(sqlmock.NewRows([]string{"p"}))
				m.ExpectPrepare("INSERT INTO pagamentos").WillReturnError(errBanco)
			},
			errSub: "prepare (pagamentos) falhou",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "inexistente.csv")
			if c.nome != "CSV inexistente" {
				path = escreverArquivo(t, dir, "pagamentos.csv", c.csv)
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
			err := pagamentos.Run(pagamentos.Options{PagamentosCSVFlag: path}, open)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errSub)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRun_ResolveCSVPathFalha(t *testing.T) {
	t.Setenv(pagamentos.EnvCSVPath, "")
	chdir(t, t.TempDir())
	err := pagamentos.Run(pagamentos.Options{DryRun: true}, openerProibido(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}
