package exportdados_test

// TST-03 (Lote 7): export das tabelas para CSV (tools/exportdados) com o
// banco em sqlmock e a pasta de destino em t.TempDir().

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/exportdados"
)

var errBanco = errors.New("falha simulada no banco")

func novoMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp),
		sqlmock.MonitorPingsOption(true),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func silenciarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	saida, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(saida); log.SetFlags(flags) })
	return &buf
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	antigo, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(antigo) })
}

func lerArquivo(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// TestExports_ContratoComOsCSVsDeOrigem garante que cada export grava em
// crm/ ou erp/ com o mesmo cabeçalho do CSV lido pelo importador
// correspondente.
func TestExports_ContratoComOsCSVsDeOrigem(t *testing.T) {
	want := map[string]string{
		filepath.Join("crm", "clientes.csv"):      "cliente_id,cnpj,razao_social,segmento,cidade,uf,bairro,data_cadastro,ativo",
		filepath.Join("crm", "vendedores.csv"):    "vendedor_id,nome,regiao,uf,data_admissao,data_desligamento,meta_mensal",
		filepath.Join("crm", "carteira.csv"):      "carteira_id,cliente_id,vendedor_id,data_inicio,data_fim",
		filepath.Join("crm", "oportunidades.csv"): "oportunidade_id,cliente_id,vendedor_id,origem,data_abertura,etapa,probabilidade_pct,valor_estimado,data_fechamento,ciclo_dias,motivo_perda",
		filepath.Join("crm", "visitas.csv"):       "visita_id,cliente_id,vendedor_id,data_visita,resultado,duracao_min",
		filepath.Join("erp", "produtos.csv"):      "sku,descricao,categoria,marca,nota_olfativa,preco_tabela,custo_unitario,unidade,ativo,data_lancamento",
		filepath.Join("erp", "pedidos.csv"):       "pedido_id,cliente_id,vendedor_id,data_pedido,canal,status,valor_total",
		filepath.Join("erp", "itens_pedido.csv"):  "item_id,pedido_id,sku,quantidade,preco_praticado,desconto_pct,valor_bruto",
		filepath.Join("erp", "pagamentos.csv"):    "pagamento_id,pedido_id,forma_pagamento,parcelas,valor,taxa_pct,valor_liquido,data_vencimento,data_pagamento,status_pagamento",
	}
	require.Len(t, exportdados.Exports, len(want))
	for _, exp := range exportdados.Exports {
		header, ok := want[exp.RelPath]
		require.True(t, ok, "export inesperado: %s", exp.RelPath)
		assert.Equal(t, header, strings.Join(exp.Header, ","), exp.RelPath)
		assert.Contains(t, exp.Query, "SELECT")
		assert.Contains(t, exp.Query, "ORDER BY", "export determinístico: %s", exp.RelPath)
	}
}

func TestWriteCSV(t *testing.T) {
	t.Run("NULL vira vazio e campos com vírgula são citados", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"a", "b", "c"}).
			AddRow(1, "Loja, Centro", nil).
			AddRow(2, "Simples", "2024-01-02"))
		rows, err := db.Query("SELECT a, b, c")
		require.NoError(t, err)
		defer rows.Close()

		var buf bytes.Buffer
		n, err := exportdados.WriteCSV(&buf, []string{"a", "b", "c"}, rows, 3)
		require.NoError(t, err)
		assert.Equal(t, 2, n)
		assert.Equal(t, "a,b,c\n1,\"Loja, Centro\",\n2,Simples,2024-01-02\n", buf.String())
	})
	t.Run("scan com número de colunas errado", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"a", "b"}).AddRow(1, 2))
		rows, err := db.Query("SELECT a, b")
		require.NoError(t, err)
		defer rows.Close()
		_, err = exportdados.WriteCSV(io.Discard, []string{"a"}, rows, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scan")
	})
	t.Run("erro na iteração", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow(1).AddRow(2).RowError(1, errBanco))
		rows, err := db.Query("SELECT a")
		require.NoError(t, err)
		defer rows.Close()
		n, err := exportdados.WriteCSV(io.Discard, []string{"a"}, rows, 1)
		require.ErrorIs(t, err, errBanco)
		assert.Equal(t, 1, n, "a linha anterior ao erro foi escrita")
	})
	t.Run("destino que falha na escrita", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow(1))
		rows, err := db.Query("SELECT a")
		require.NoError(t, err)
		defer rows.Close()
		_, err = exportdados.WriteCSV(escritorQuebrado{}, []string{"a"}, rows, 1)
		assert.ErrorIs(t, err, errDisco)
	})
}

var errDisco = errors.New("disco cheio")

type escritorQuebrado struct{}

func (escritorQuebrado) Write([]byte) (int, error) { return 0, errDisco }

func TestExportTable(t *testing.T) {
	exp := exportdados.TableExport{RelPath: "x.csv", Header: []string{"id", "nome"}, Query: "SELECT id, nome FROM t"}

	t.Run("cria diretórios intermediários e grava o CSV", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(regexp.QuoteMeta(exp.Query)).WillReturnRows(sqlmock.NewRows([]string{"id", "nome"}).AddRow(1, "Ana"))
		dest := filepath.Join(t.TempDir(), "a", "b", "x.csv")
		n, err := exportdados.ExportTable(db, exp, dest)
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, "id,nome\n1,Ana\n", lerArquivo(t, dest))
	})
	t.Run("diretório de destino é um arquivo", func(t *testing.T) {
		db, _ := novoMock(t)
		dir := t.TempDir()
		arquivo := filepath.Join(dir, "ocupado")
		require.NoError(t, os.WriteFile(arquivo, nil, 0o600))
		_, err := exportdados.ExportTable(db, exp, filepath.Join(arquivo, "x.csv"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "criando diretório")
	})
	t.Run("query falha", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery("SELECT").WillReturnError(errBanco)
		_, err := exportdados.ExportTable(db, exp, filepath.Join(t.TempDir(), "x.csv"))
		require.ErrorIs(t, err, errBanco)
		assert.Contains(t, err.Error(), "query")
	})
	t.Run("destino é um diretório", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "nome"}))
		dest := t.TempDir() // já existe como diretório
		_, err := exportdados.ExportTable(db, exp, dest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "criando arquivo")
	})
}

// esperarTodosOsExports registra uma query por export, na ordem de Exports,
// cada uma devolvendo uma linha com o nome do arquivo na 1ª coluna.
func esperarTodosOsExports(mock sqlmock.Sqlmock) {
	for _, exp := range exportdados.Exports {
		vals := make([]driver.Value, len(exp.Header)) // demais colunas NULL
		vals[0] = filepath.Base(exp.RelPath)
		mock.ExpectQuery(regexp.QuoteMeta(exp.Query)).WillReturnRows(sqlmock.NewRows(exp.Header).AddRow(vals...))
	}
}

func TestExportAll(t *testing.T) {
	t.Run("exporta todas as tabelas na ordem", func(t *testing.T) {
		logs := silenciarLog(t)
		db, mock := novoMock(t)
		esperarTodosOsExports(mock)
		out := t.TempDir()
		require.NoError(t, exportdados.ExportAll(db, out, exportdados.Exports))
		require.NoError(t, mock.ExpectationsWereMet())
		for _, exp := range exportdados.Exports {
			conteudo := lerArquivo(t, filepath.Join(out, exp.RelPath))
			linhas := strings.Split(strings.TrimSuffix(conteudo, "\n"), "\n")
			require.Len(t, linhas, 2, exp.RelPath)
			assert.Equal(t, strings.Join(exp.Header, ","), linhas[0])
			assert.True(t, strings.HasPrefix(linhas[1], filepath.Base(exp.RelPath)+","), linhas[1])
		}
		assert.Contains(t, logs.String(), "OK — 9 arquivos exportados")
	})
	t.Run("para no primeiro erro", func(t *testing.T) {
		silenciarLog(t)
		db, mock := novoMock(t)
		exps := exportdados.Exports[:3]
		mock.ExpectQuery(regexp.QuoteMeta(exps[0].Query)).WillReturnRows(sqlmock.NewRows(exps[0].Header))
		mock.ExpectQuery(regexp.QuoteMeta(exps[1].Query)).WillReturnError(errBanco)
		out := t.TempDir()
		err := exportdados.ExportAll(db, out, exps)
		require.ErrorIs(t, err, errBanco)
		assert.Contains(t, err.Error(), filepath.Join(out, exps[1].RelPath))
		assert.NoFileExists(t, filepath.Join(out, exps[2].RelPath), "o terceiro export não roda")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestResolveOutDir(t *testing.T) {
	got, err := exportdados.ResolveOutDir("/tmp/saida")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/saida", got)

	got, err = exportdados.ResolveOutDir("")
	require.NoError(t, err)
	root, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "export"), got)

	chdir(t, t.TempDir())
	_, err = exportdados.ResolveOutDir("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raiz do projeto")
}

func TestRun(t *testing.T) {
	t.Run("exporta com o banco aberto pelo Opener", func(t *testing.T) {
		silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectPing()
		esperarTodosOsExports(mock)
		mock.ExpectClose()
		out := t.TempDir()
		err := exportdados.Run(exportdados.Options{OutFlag: out}, func() (cmdutil.Conn, error) { return db, nil })
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		assert.FileExists(t, filepath.Join(out, "erp", "pagamentos.csv"))
	})

	casos := []struct {
		nome   string
		abrir  error
		mock   func(m sqlmock.Sqlmock)
		errSub string
	}{
		{nome: "abrir falha", abrir: errBanco, errSub: errBanco.Error()},
		{nome: "ping falha", mock: func(m sqlmock.Sqlmock) { m.ExpectPing().WillReturnError(errBanco) }, errSub: "ping no banco falhou"},
		{nome: "query falha", mock: func(m sqlmock.Sqlmock) {
			m.ExpectPing()
			m.ExpectQuery("SELECT").WillReturnError(errBanco)
		}, errSub: "falha exportando"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			silenciarLog(t)
			db, mock := novoMock(t)
			if c.mock != nil {
				c.mock(mock)
			}
			err := exportdados.Run(exportdados.Options{OutFlag: t.TempDir()}, func() (cmdutil.Conn, error) {
				if c.abrir != nil {
					return nil, c.abrir
				}
				return db, nil
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errSub)
		})
	}

	t.Run("sem raiz do projeto não abre o banco", func(t *testing.T) {
		chdir(t, t.TempDir())
		err := exportdados.Run(exportdados.Options{}, func() (cmdutil.Conn, error) {
			t.Fatal("não deveria abrir o banco")
			return nil, nil
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "raiz do projeto")
	})
}
