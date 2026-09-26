package pedidos_test

// TST-03 (Lote 7): parsing, lookups, upserts e Run do importador de pedidos
// e itens de pedido, com CSVs em t.TempDir() e banco em sqlmock.

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
	"github.com/rotaperfumes/shared/importers/pedidos"
)

const (
	headerPedidos = "pedido_id,cliente_id,vendedor_id,data_pedido,canal,status,valor_total\n"
	headerItens   = "item_id,pedido_id,sku,quantidade,preco_praticado,desconto_pct,valor_bruto\n"

	// clientes.csv usado pela unificação NEG-01: 3001 é cópia do cliente 1.
	clientesCSVUnificacao = "cliente_id,cnpj\n1,11222333000181\n3001,11.222.333/0001-81\n"
)

func data(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// ---------------------------------------------------------------------------
// Parsing de pedidos.csv
// ---------------------------------------------------------------------------

func TestParsePedidoRow(t *testing.T) {
	casos := []struct {
		nome   string
		rec    []string
		want   pedidos.PedidoRow
		errSub string
	}{
		{
			nome: "ISO completo",
			rec:  []string{"10", "1", "7", "2024-03-05", "App", "Faturado", "150.50"},
			want: pedidos.PedidoRow{PedidoIDOrigem: 10, ClienteIDOrigem: 1, VendedorID: 7, DataPedido: data(2024, 3, 5), Canal: "App", Status: "Faturado", ValorTotal: 150.5},
		},
		{
			nome: "data BR e espaços aparados",
			rec:  []string{" 11 ", " 2 ", " 8 ", " 05/03/2024 ", " WhatsApp ", " Em separação ", " 99 "},
			want: pedidos.PedidoRow{PedidoIDOrigem: 11, ClienteIDOrigem: 2, VendedorID: 8, DataPedido: data(2024, 3, 5), Canal: "WhatsApp", Status: "Em separação", ValorTotal: 99},
		},
		{nome: "pedido_id inválido", rec: []string{"x", "1", "7", "2024-03-05", "App", "Faturado", "1"}, errSub: "pedido_id inválido"},
		{nome: "cliente_id inválido", rec: []string{"10", "", "7", "2024-03-05", "App", "Faturado", "1"}, errSub: "cliente_id inválido"},
		{nome: "vendedor_id inválido", rec: []string{"10", "1", "7a", "2024-03-05", "App", "Faturado", "1"}, errSub: "vendedor_id inválido"},
		{nome: "data em formato desconhecido", rec: []string{"10", "1", "7", "2024/03/05", "App", "Faturado", "1"}, errSub: "data_pedido inválida"},
		{nome: "canal fora do ENUM", rec: []string{"10", "1", "7", "2024-03-05", "Email", "Faturado", "1"}, errSub: "canal inválido"},
		{nome: "canal é case-sensitive", rec: []string{"10", "1", "7", "2024-03-05", "app", "Faturado", "1"}, errSub: "canal inválido"},
		{nome: "status fora do ENUM", rec: []string{"10", "1", "7", "2024-03-05", "App", "Pago", "1"}, errSub: "status inválido"},
		{nome: "valor_total inválido", rec: []string{"10", "1", "7", "2024-03-05", "App", "Entregue", "1,5"}, errSub: "valor_total inválido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := pedidos.ParsePedidoRow(c.rec)
			if c.errSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.errSub)
				assert.Equal(t, pedidos.PedidoRow{}, got)
				return
			}
			require.NoError(t, err)
			assert.True(t, c.want.DataPedido.Equal(got.DataPedido), "data: got %s want %s", got.DataPedido, c.want.DataPedido)
			got.DataPedido, c.want.DataPedido = time.Time{}, time.Time{}
			assert.Equal(t, c.want, got)
		})
	}
}

func TestIsValidCanalEStatus(t *testing.T) {
	canais := map[string]bool{"App": true, "Telefone": true, "Visita": true, "WhatsApp": true, "": false, "Loja": false, "APP": false}
	for v, want := range canais {
		assert.Equal(t, want, pedidos.IsValidCanal(v), "canal %q", v)
	}
	status := map[string]bool{"Cancelado": true, "Em separação": true, "Entregue": true, "Faturado": true, "": false, "Em separacao": false, "Aberto": false}
	for v, want := range status {
		assert.Equal(t, want, pedidos.IsValidStatus(v), "status %q", v)
	}
}

func TestReadPedidosCSV(t *testing.T) {
	casos := []struct {
		nome       string
		csv        string
		wantIDs    []int64
		wantErrs   int
		wantErrSub string
	}{
		{nome: "vazio: sem cabeçalho", csv: "", wantErrSub: "lendo cabeçalho"},
		{nome: "só cabeçalho", csv: headerPedidos},
		{
			nome: "válidas, colunas a menos e valor inválido",
			csv: headerPedidos +
				"1,1,7,2024-01-02,App,Faturado,10\n" +
				"2,1,7,2024-01-02,App\n" + // 5 colunas: erro de leitura CSV
				"3,1,7,2024-01-02,Fax,Faturado,10\n" + // canal inválido
				"4,2,8,02/01/2024,Visita,Entregue,20.5\n",
			wantIDs:  []int64{1, 4},
			wantErrs: 2,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			rows, parseErrs, err := pedidos.ReadPedidosCSV(strings.NewReader(c.csv))
			if c.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErrSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantErrs, parseErrs)
			var ids []int64
			for _, r := range rows {
				ids = append(ids, r.PedidoIDOrigem)
			}
			assert.Equal(t, c.wantIDs, ids)
		})
	}
}

func TestReadPedidosCSVFile(t *testing.T) {
	capturarLog(t)
	dir := t.TempDir()
	path := escreverArquivo(t, dir, "pedidos.csv", headerPedidos+"1,1,7,2024-01-02,Telefone,Cancelado,0\n")

	rows, parseErrs, err := pedidos.ReadPedidosCSVFile(path)
	require.NoError(t, err)
	assert.Zero(t, parseErrs)
	require.Len(t, rows, 1)
	assert.Equal(t, "Telefone", rows[0].Canal)

	_, _, err = pedidos.ReadPedidosCSVFile(filepath.Join(dir, "inexistente.csv"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abrindo arquivo")
}

// ---------------------------------------------------------------------------
// Parsing de itens_pedido.csv
// ---------------------------------------------------------------------------

func TestParseItemPedidoRow(t *testing.T) {
	casos := []struct {
		nome   string
		rec    []string
		want   pedidos.ItemPedidoRow
		errSub string
	}{
		{
			nome: "válido com espaços",
			rec:  []string{" 100 ", "10", " SKU-1 ", "3", "19.90", "5", "59.70"},
			want: pedidos.ItemPedidoRow{ItemIDOrigem: 100, PedidoIDOrigem: 10, SKU: "SKU-1", Quantidade: 3, PrecoPraticado: 19.9, DescontoPct: 5, ValorBruto: 59.7},
		},
		{nome: "item_id inválido", rec: []string{"a", "10", "S", "1", "1", "0", "1"}, errSub: "item_id inválido"},
		{nome: "pedido_id inválido", rec: []string{"1", "", "S", "1", "1", "0", "1"}, errSub: "pedido_id inválido"},
		{nome: "sku vazio", rec: []string{"1", "10", "  ", "1", "1", "0", "1"}, errSub: "sku vazio"},
		{nome: "quantidade decimal", rec: []string{"1", "10", "S", "1.5", "1", "0", "1"}, errSub: "quantidade inválida"},
		{nome: "preço inválido", rec: []string{"1", "10", "S", "1", "R$1", "0", "1"}, errSub: "preco_praticado inválido"},
		{nome: "desconto inválido", rec: []string{"1", "10", "S", "1", "1", "%", "1"}, errSub: "desconto_pct inválido"},
		{nome: "valor_bruto inválido", rec: []string{"1", "10", "S", "1", "1", "0", ""}, errSub: "valor_bruto inválido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := pedidos.ParseItemPedidoRow(c.rec)
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

func TestReadItensPedidoCSV(t *testing.T) {
	casos := []struct {
		nome       string
		csv        string
		wantIDs    []int64
		wantErrs   int
		wantErrSub string
	}{
		{nome: "vazio", csv: "", wantErrSub: "lendo cabeçalho"},
		{nome: "só cabeçalho", csv: headerItens},
		{
			nome: "mistura de válidas e inválidas",
			csv: headerItens +
				"100,10,SKU-1,1,10,0,10\n" +
				"101,10,SKU-1,1,10,0,10,extra\n" + // 8 colunas
				"102,10,,1,10,0,10\n" + // sku vazio
				"103,11,SKU-2,2,5,10,9\n",
			wantIDs:  []int64{100, 103},
			wantErrs: 2,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			rows, parseErrs, err := pedidos.ReadItensPedidoCSV(strings.NewReader(c.csv))
			if c.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErrSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantErrs, parseErrs)
			var ids []int64
			for _, r := range rows {
				ids = append(ids, r.ItemIDOrigem)
			}
			assert.Equal(t, c.wantIDs, ids)
		})
	}
}

func TestReadItensPedidoCSVFile_Inexistente(t *testing.T) {
	_, _, err := pedidos.ReadItensPedidoCSVFile(filepath.Join(t.TempDir(), "nada.csv"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abrindo arquivo")
}

// ---------------------------------------------------------------------------
// ResolveCSVPath
// ---------------------------------------------------------------------------

func TestResolveCSVPath(t *testing.T) {
	t.Run("flag tem prioridade sobre env", func(t *testing.T) {
		t.Setenv(pedidos.EnvPedidosCSVPath, "/env/pedidos.csv")
		got, err := pedidos.ResolveCSVPath("/flag/pedidos.csv", pedidos.EnvPedidosCSVPath, "pedidos.csv")
		require.NoError(t, err)
		assert.Equal(t, "/flag/pedidos.csv", got)
	})
	t.Run("env quando sem flag", func(t *testing.T) {
		t.Setenv(pedidos.EnvItensCSVPath, "/env/itens.csv")
		got, err := pedidos.ResolveCSVPath("", pedidos.EnvItensCSVPath, "itens_pedido.csv")
		require.NoError(t, err)
		assert.Equal(t, "/env/itens.csv", got)
	})
	t.Run("default em dados/erp da raiz do projeto", func(t *testing.T) {
		t.Setenv(pedidos.EnvPedidosCSVPath, "")
		got, err := pedidos.ResolveCSVPath("", pedidos.EnvPedidosCSVPath, "pedidos.csv")
		require.NoError(t, err)
		root, err := cmdutil.FindProjectRoot()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "dados", "erp", "pedidos.csv"), got)
	})
	t.Run("fora do repositório devolve erro", func(t *testing.T) {
		t.Setenv(pedidos.EnvPedidosCSVPath, "")
		chdir(t, t.TempDir())
		_, err := pedidos.ResolveCSVPath("", pedidos.EnvPedidosCSVPath, "pedidos.csv")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "raiz do projeto")
	})
}

// ---------------------------------------------------------------------------
// Lookups
// ---------------------------------------------------------------------------

func TestLoadLookups(t *testing.T) {
	t.Run("clientes e pedidos: identidade origem -> origem", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery("SELECT cliente_id_origem FROM clientes").
			WillReturnRows(sqlmock.NewRows([]string{"cliente_id_origem"}).AddRow(1).AddRow(5))
		mock.ExpectQuery("SELECT pedido_id_origem FROM pedidos").
			WillReturnRows(sqlmock.NewRows([]string{"pedido_id_origem"}).AddRow(10))

		cli, err := pedidos.LoadClienteIDsByOrigem(db)
		require.NoError(t, err)
		assert.Equal(t, map[int64]int64{1: 1, 5: 5}, cli)

		ped, err := pedidos.LoadPedidoIDsByOrigem(db)
		require.NoError(t, err)
		assert.Equal(t, map[int64]int64{10: 10}, ped)
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("produtos: sku -> id", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery("SELECT id, sku FROM produtos").
			WillReturnRows(sqlmock.NewRows([]string{"id", "sku"}).AddRow(3, "SKU-A").AddRow(4, "SKU-B"))
		got, err := pedidos.LoadProdutoIDsBySKU(db)
		require.NoError(t, err)
		assert.Equal(t, map[string]int64{"SKU-A": 3, "SKU-B": 4}, got)
	})

	type loader func(cmdutil.DB) error
	loaders := map[string]struct {
		query string
		cols  []string
		ruim  []driver.Value // linha que não converte no Scan
		fn    loader
	}{
		"clientes": {"SELECT cliente_id_origem FROM clientes", []string{"c"}, []driver.Value{"x"}, func(db cmdutil.DB) error { _, err := pedidos.LoadClienteIDsByOrigem(db); return err }},
		"pedidos":  {"SELECT pedido_id_origem FROM pedidos", []string{"p"}, []driver.Value{"x"}, func(db cmdutil.DB) error { _, err := pedidos.LoadPedidoIDsByOrigem(db); return err }},
		"produtos": {"SELECT id, sku FROM produtos", []string{"id", "sku"}, []driver.Value{"x", "S"}, func(db cmdutil.DB) error { _, err := pedidos.LoadProdutoIDsBySKU(db); return err }},
	}
	for nome, l := range loaders {
		t.Run(nome+": erro na query", func(t *testing.T) {
			db, mock := novoMock(t)
			mock.ExpectQuery(l.query).WillReturnError(errBanco)
			assert.ErrorIs(t, l.fn(db), errBanco)
		})
		t.Run(nome+": erro no scan", func(t *testing.T) {
			db, mock := novoMock(t)
			mock.ExpectQuery(l.query).WillReturnRows(sqlmock.NewRows(l.cols).AddRow(l.ruim...))
			assert.Error(t, l.fn(db))
		})
		t.Run(nome+": erro na iteração", func(t *testing.T) {
			db, mock := novoMock(t)
			linhaBoa := make([]driver.Value, len(l.cols))
			for i := range linhaBoa {
				linhaBoa[i] = 1
			}
			mock.ExpectQuery(l.query).WillReturnRows(sqlmock.NewRows(l.cols).AddRow(linhaBoa...).RowError(0, errBanco))
			assert.ErrorIs(t, l.fn(db), errBanco)
		})
	}
}

// ---------------------------------------------------------------------------
// Upserts
// ---------------------------------------------------------------------------

func TestUpsertPedidos(t *testing.T) {
	capturarLog(t)
	db, mock := novoMock(t)
	rows := []pedidos.PedidoRow{
		{PedidoIDOrigem: 1, ClienteIDOrigem: 1, VendedorID: 7, DataPedido: data(2024, 1, 2), Canal: "App", Status: "Faturado", ValorTotal: 10},
		{PedidoIDOrigem: 2, ClienteIDOrigem: 999, VendedorID: 7, DataPedido: data(2024, 1, 2), Canal: "App", Status: "Faturado", ValorTotal: 10},
		{PedidoIDOrigem: 3, ClienteIDOrigem: 1, VendedorID: 7, DataPedido: data(2024, 1, 3), Canal: "Visita", Status: "Entregue", ValorTotal: 20},
		{PedidoIDOrigem: 4, ClienteIDOrigem: 1, VendedorID: 99, DataPedido: data(2024, 1, 4), Canal: "App", Status: "Cancelado", ValorTotal: 0},
		{PedidoIDOrigem: 5, ClienteIDOrigem: 1, VendedorID: 7, DataPedido: data(2024, 1, 5), Canal: "App", Status: "Faturado", ValorTotal: 5},
	}
	mock.ExpectPrepare("INSERT INTO pedidos")
	mock.ExpectExec("INSERT INTO pedidos").WithArgs(int64(1), int64(1), int64(7), "2024-01-02", "App", "Faturado", 10.0).WillReturnResult(sqlmock.NewResult(0, 1))
	// pedido 2: cliente 999 não está no lookup -> nenhum Exec.
	mock.ExpectExec("INSERT INTO pedidos").WithArgs(int64(3), int64(1), int64(7), "2024-01-03", "Visita", "Entregue", 20.0).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO pedidos").WithArgs(int64(4), int64(1), int64(99), "2024-01-04", "App", "Cancelado", 0.0).WillReturnError(errBanco) // FK de vendedor
	mock.ExpectExec("INSERT INTO pedidos").WithArgs(int64(5), int64(1), int64(7), "2024-01-05", "App", "Faturado", 5.0).WillReturnResult(sqlmock.NewResult(0, 0))

	ins, upd, fail, err := pedidos.UpsertPedidos(db, rows, map[int64]int64{1: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, ins, "affected=1: inserido ou inalterado")
	assert.Equal(t, 2, upd, "affected=2 e affected=0 contam como atualizado")
	assert.Equal(t, 2, fail, "cliente ausente + erro de Exec")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertItensPedido(t *testing.T) {
	capturarLog(t)
	db, mock := novoMock(t)
	rows := []pedidos.ItemPedidoRow{
		{ItemIDOrigem: 100, PedidoIDOrigem: 10, SKU: "SKU-A", Quantidade: 2, PrecoPraticado: 10, DescontoPct: 0, ValorBruto: 20},
		{ItemIDOrigem: 101, PedidoIDOrigem: 99, SKU: "SKU-A", Quantidade: 1, PrecoPraticado: 10, ValorBruto: 10}, // pedido ausente
		{ItemIDOrigem: 102, PedidoIDOrigem: 10, SKU: "SKU-X", Quantidade: 1, PrecoPraticado: 10, ValorBruto: 10}, // sku ausente
		{ItemIDOrigem: 103, PedidoIDOrigem: 10, SKU: "SKU-B", Quantidade: 1, PrecoPraticado: 5, DescontoPct: 10, ValorBruto: 5},
		{ItemIDOrigem: 104, PedidoIDOrigem: 10, SKU: "SKU-B", Quantidade: 1, PrecoPraticado: 5, ValorBruto: 5},
		{ItemIDOrigem: 105, PedidoIDOrigem: 10, SKU: "SKU-B", Quantidade: 1, PrecoPraticado: 5, ValorBruto: 5},
	}
	mock.ExpectPrepare("INSERT INTO itens_pedido")
	mock.ExpectExec("INSERT INTO itens_pedido").WithArgs(int64(100), int64(10), int64(3), 2, 10.0, 0.0, 20.0).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO itens_pedido").WithArgs(int64(103), int64(10), int64(4), 1, 5.0, 10.0, 5.0).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO itens_pedido").WithArgs(int64(104), int64(10), int64(4), 1, 5.0, 0.0, 5.0).WillReturnError(errBanco)
	mock.ExpectExec("INSERT INTO itens_pedido").WithArgs(int64(105), int64(10), int64(4), 1, 5.0, 0.0, 5.0).WillReturnResult(sqlmock.NewResult(0, 0))

	ins, upd, fail, err := pedidos.UpsertItensPedido(db, rows, map[int64]int64{10: 10}, map[string]int64{"SKU-A": 3, "SKU-B": 4})
	require.NoError(t, err)
	assert.Equal(t, 1, ins)
	assert.Equal(t, 2, upd)
	assert.Equal(t, 3, fail, "pedido ausente + sku ausente + erro de Exec")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpserts_PrepareFalha(t *testing.T) {
	t.Run("pedidos", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectPrepare("INSERT INTO pedidos").WillReturnError(errBanco)
		_, _, _, err := pedidos.UpsertPedidos(db, nil, nil)
		require.ErrorIs(t, err, errBanco)
		assert.Contains(t, err.Error(), "prepare (pedidos)")
	})
	t.Run("itens", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectPrepare("INSERT INTO itens_pedido").WillReturnError(errBanco)
		_, _, _, err := pedidos.UpsertItensPedido(db, nil, nil, nil)
		require.ErrorIs(t, err, errBanco)
		assert.Contains(t, err.Error(), "prepare (itens_pedido)")
	})
}

// ---------------------------------------------------------------------------
// Run
// ---------------------------------------------------------------------------

// cenarioRun prepara os três CSVs (pedidos, itens e clientes para a
// unificação NEG-01) num diretório temporário.
func cenarioRun(t *testing.T) pedidos.Options {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(clientesdedup.EnvCSVPath, escreverArquivo(t, dir, "clientes.csv", clientesCSVUnificacao))
	return pedidos.Options{
		PedidosCSVFlag: escreverArquivo(t, dir, "pedidos.csv", headerPedidos+
			"10,1,7,2024-01-02,App,Faturado,100\n"+
			"11,3001,7,03/01/2024,Visita,Entregue,50\n"+ // cópia -> cliente 1
			"12,999,7,2024-01-04,App,Faturado,1\n"+ // cliente inexistente
			"13,1,7,data-ruim,App,Faturado,1\n"), // erro de parsing
		ItensCSVFlag: escreverArquivo(t, dir, "itens.csv", headerItens+
			"100,10,SKU-A,2,50,0,100\n"+
			"101,11,SKU-X,1,50,0,50\n"+ // sku inexistente
			"102,x,SKU-A,1,1,0,1\n"), // erro de parsing
	}
}

func TestRun_DryRunNaoAbreBanco(t *testing.T) {
	logs := capturarLog(t)
	opts := cenarioRun(t)
	opts.DryRun = true

	require.NoError(t, pedidos.Run(opts, openerProibido(t)))
	assert.Contains(t, logs.String(), "[fase 1/2] 3 linhas válidas lidas, 1 linhas com erro de parsing")
	assert.Contains(t, logs.String(), "[fase 2/2] 2 linhas válidas lidas, 1 linhas com erro de parsing")
	assert.Contains(t, logs.String(), "--dry-run informado")
}

func TestRun_ImportaAsDuasFasesComUnificacao(t *testing.T) {
	logs := capturarLog(t)
	opts := cenarioRun(t)
	db, mock := novoMock(t)

	mock.ExpectPing()
	mock.ExpectQuery("SELECT cliente_id_origem FROM clientes").
		WillReturnRows(sqlmock.NewRows([]string{"cliente_id_origem"}).AddRow(1))
	mock.ExpectPrepare("INSERT INTO pedidos")
	mock.ExpectExec("INSERT INTO pedidos").WithArgs(int64(10), int64(1), int64(7), "2024-01-02", "App", "Faturado", 100.0).WillReturnResult(sqlmock.NewResult(0, 1))
	// NEG-01: pedido do cliente 3001 (cópia) é gravado no sobrevivente 1.
	mock.ExpectExec("INSERT INTO pedidos").WithArgs(int64(11), int64(1), int64(7), "2024-01-03", "Visita", "Entregue", 50.0).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery("SELECT pedido_id_origem FROM pedidos").
		WillReturnRows(sqlmock.NewRows([]string{"pedido_id_origem"}).AddRow(10).AddRow(11))
	mock.ExpectQuery("SELECT id, sku FROM produtos").
		WillReturnRows(sqlmock.NewRows([]string{"id", "sku"}).AddRow(3, "SKU-A"))
	mock.ExpectPrepare("INSERT INTO itens_pedido")
	mock.ExpectExec("INSERT INTO itens_pedido").WithArgs(int64(100), int64(10), int64(3), 2, 50.0, 0.0, 100.0).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectClose()

	aberturas := 0
	require.NoError(t, pedidos.Run(opts, openerDe(db, &aberturas)))
	assert.Equal(t, 1, aberturas)
	require.NoError(t, mock.ExpectationsWereMet())

	saida := logs.String()
	assert.Contains(t, saida, "1 cliente_id(s) de CNPJ duplicado redirecionados")
	assert.Contains(t, saida, "[fase 1/2] OK — lidos=3 inseridos_ou_inalterados=1 atualizados=1 erros_upsert=1 erros_parsing=1")
	assert.Contains(t, saida, "[fase 2/2] OK — lidos=2 inseridos_ou_inalterados=1 atualizados=0 erros_upsert=1 erros_parsing=1")
}

func TestRun_Erros(t *testing.T) {
	casos := []struct {
		nome   string
		ajuste func(t *testing.T, o *pedidos.Options)
		mock   func(m sqlmock.Sqlmock)
		abrir  error
		errSub string
	}{
		{
			nome:   "CSV de pedidos inexistente",
			ajuste: func(t *testing.T, o *pedidos.Options) { o.PedidosCSVFlag = filepath.Join(t.TempDir(), "x.csv") },
			errSub: "falha ao ler CSV de pedidos",
		},
		{
			nome: "CSV de itens inexistente no dry-run",
			ajuste: func(t *testing.T, o *pedidos.Options) {
				o.ItensCSVFlag = filepath.Join(t.TempDir(), "x.csv")
				o.DryRun = true
			},
			errSub: "falha ao ler CSV de itens de pedido",
		},
		{
			nome:   "falha ao abrir o banco",
			abrir:  errBanco,
			errSub: errBanco.Error(),
		},
		{
			nome:   "ping falha",
			mock:   func(m sqlmock.Sqlmock) { m.ExpectPing().WillReturnError(errBanco) },
			errSub: "ping no banco falhou",
		},
		{
			nome: "lookup de clientes falha",
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery("FROM clientes").WillReturnError(errBanco)
			},
			errSub: "falha ao carregar clientes",
		},
		{
			nome: "prepare de pedidos falha",
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery("FROM clientes").WillReturnRows(sqlmock.NewRows([]string{"c"}))
				m.ExpectPrepare("INSERT INTO pedidos").WillReturnError(errBanco)
			},
			errSub: "prepare (pedidos) falhou",
		},
		{
			nome:   "CSV de itens some antes da fase 2",
			ajuste: func(t *testing.T, o *pedidos.Options) { o.ItensCSVFlag = filepath.Join(t.TempDir(), "x.csv") },
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery("FROM clientes").WillReturnRows(sqlmock.NewRows([]string{"c"}))
				m.ExpectPrepare("INSERT INTO pedidos")
			},
			errSub: "falha ao ler CSV de itens de pedido",
		},
		{
			nome: "lookup de pedidos falha",
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery("FROM clientes").WillReturnRows(sqlmock.NewRows([]string{"c"}))
				m.ExpectPrepare("INSERT INTO pedidos")
				m.ExpectQuery("FROM pedidos").WillReturnError(errBanco)
			},
			errSub: "falha ao carregar pedidos",
		},
		{
			nome: "lookup de produtos falha",
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery("FROM clientes").WillReturnRows(sqlmock.NewRows([]string{"c"}))
				m.ExpectPrepare("INSERT INTO pedidos")
				m.ExpectQuery("FROM pedidos").WillReturnRows(sqlmock.NewRows([]string{"p"}))
				m.ExpectQuery("FROM produtos").WillReturnError(errBanco)
			},
			errSub: "falha ao carregar produtos",
		},
		{
			nome: "prepare de itens falha",
			mock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectQuery("FROM clientes").WillReturnRows(sqlmock.NewRows([]string{"c"}))
				m.ExpectPrepare("INSERT INTO pedidos")
				m.ExpectQuery("FROM pedidos").WillReturnRows(sqlmock.NewRows([]string{"p"}))
				m.ExpectQuery("FROM produtos").WillReturnRows(sqlmock.NewRows([]string{"id", "sku"}))
				m.ExpectPrepare("INSERT INTO itens_pedido").WillReturnError(errBanco)
			},
			errSub: "prepare (itens_pedido) falhou",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			dir := t.TempDir()
			t.Setenv(clientesdedup.EnvCSVPath, filepath.Join(dir, "sem-clientes.csv"))
			// Sem linhas: o foco é o caminho de erro, não o upsert.
			opts := pedidos.Options{
				PedidosCSVFlag: escreverArquivo(t, dir, "pedidos.csv", headerPedidos),
				ItensCSVFlag:   escreverArquivo(t, dir, "itens.csv", headerItens),
			}
			if c.ajuste != nil {
				c.ajuste(t, &opts)
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
			err := pedidos.Run(opts, open)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errSub)
			assert.NotContains(t, err.Error(), pedidos.Tag, "o prefixo Tag fica a cargo do main")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRun_ResolveCSVPathFalha(t *testing.T) {
	t.Setenv(pedidos.EnvPedidosCSVPath, "")
	t.Setenv(pedidos.EnvItensCSVPath, "")
	chdir(t, t.TempDir())

	err := pedidos.Run(pedidos.Options{DryRun: true}, openerProibido(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")

	err = pedidos.Run(pedidos.Options{PedidosCSVFlag: "p.csv", DryRun: true}, openerProibido(t))
	require.Error(t, err, "itens sem flag nem env também exige a raiz")
	assert.Contains(t, err.Error(), "raiz do projeto")
}
