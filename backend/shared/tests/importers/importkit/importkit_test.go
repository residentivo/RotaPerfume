package importkit_test

// ARQ-07: esqueleto comum dos importadores (Template Method, §5.13). Os
// importadores concretos são cobertos pelas suítes de cada pacote; aqui
// ficam a ordem fixa dos passos e os ganchos do kit.

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/clientes"
	"github.com/rotaperfumes/shared/importers/importkit"
)

var errTeste = errors.New("falhou")

func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(anterior) })
	return &buf
}

// passos registra a ordem em que o método-template chama os ganchos.
type passos struct {
	ordem                        []string
	errLer, errDryRun, errGravar error
}

func (p *passos) etapas(comDryRun bool) importkit.Etapas {
	e := importkit.Etapas{
		Tag:    "importteste",
		Ler:    func() error { p.ordem = append(p.ordem, "ler"); return p.errLer },
		Gravar: func(cmdutil.DB) error { p.ordem = append(p.ordem, "gravar"); return p.errGravar },
	}
	if comDryRun {
		e.LerNoDryRun = func() error { p.ordem = append(p.ordem, "dry-run"); return p.errDryRun }
	}
	return e
}

func abrirMock(t *testing.T, ping bool) (cmdutil.Opener, sqlmock.Sqlmock, *[]string) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	if ping {
		mock.ExpectPing()
	} else {
		mock.ExpectPing().WillReturnError(errTeste)
	}
	mock.ExpectClose()
	abertos := &[]string{}
	return func() (cmdutil.Conn, error) { *abertos = append(*abertos, "open"); return db, nil }, mock, abertos
}

func TestImportkit_Executar_OrdemDosPassos(t *testing.T) {
	logs := capturarLog(t)
	t.Run("carga real: ler → abrir → ping → gravar → fechar", func(t *testing.T) {
		p := &passos{}
		open, mock, abertos := abrirMock(t, true)
		require.NoError(t, importkit.Executar(p.etapas(true), false, open))
		assert.Equal(t, []string{"ler", "gravar"}, p.ordem, "LerNoDryRun não roda na carga real")
		assert.Equal(t, []string{"open"}, *abertos)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("dry-run: ler → ganchos do dry-run, sem abrir o banco", func(t *testing.T) {
		p := &passos{}
		nuncaAbre := func() (cmdutil.Conn, error) { t.Fatal("dry-run não abre o banco"); return nil, nil }
		require.NoError(t, importkit.Executar(p.etapas(true), true, nuncaAbre))
		assert.Equal(t, []string{"ler", "dry-run"}, p.ordem)
		assert.Contains(t, logs.String(), "importteste: --dry-run informado, nada foi gravado no banco")
		p2 := &passos{}
		require.NoError(t, importkit.Executar(p2.etapas(false), true, nuncaAbre))
		assert.Equal(t, []string{"ler"}, p2.ordem, "sem gancho de dry-run")
	})
	t.Run("falha ao ler para antes de tudo", func(t *testing.T) {
		p := &passos{errLer: errTeste}
		assert.ErrorIs(t, importkit.Executar(p.etapas(true), false, nil), errTeste)
		assert.Equal(t, []string{"ler"}, p.ordem)
	})
	t.Run("falha no gancho do dry-run", func(t *testing.T) {
		p := &passos{errDryRun: errTeste}
		assert.ErrorIs(t, importkit.Executar(p.etapas(true), true, nil), errTeste)
	})
	t.Run("falha ao abrir, no ping e ao gravar", func(t *testing.T) {
		p := &passos{}
		falhaAbrir := func() (cmdutil.Conn, error) { return nil, errTeste }
		assert.ErrorIs(t, importkit.Executar(p.etapas(false), false, falhaAbrir), errTeste)

		p = &passos{}
		open, mock, _ := abrirMock(t, false)
		err := importkit.Executar(p.etapas(false), false, open)
		require.ErrorIs(t, err, errTeste)
		assert.Contains(t, err.Error(), "ping no banco falhou")
		assert.Equal(t, []string{"ler"}, p.ordem, "sem ping não grava")
		assert.NoError(t, mock.ExpectationsWereMet(), "a conexão é fechada mesmo com o ping falhando")

		p = &passos{errGravar: errTeste}
		open, _, _ = abrirMock(t, true)
		assert.ErrorIs(t, importkit.Executar(p.etapas(false), false, open), errTeste)
	})
}

func TestImportkit_ResolverCSV(t *testing.T) {
	t.Setenv("IMPORTKIT_TESTE_CSV", "")
	c, err := importkit.ResolverCSV("/flag.csv", "IMPORTKIT_TESTE_CSV", "dados", "x.csv")
	require.NoError(t, err)
	assert.Equal(t, "/flag.csv", c, "a flag vence")

	t.Setenv("IMPORTKIT_TESTE_CSV", "/env.csv")
	c, err = importkit.ResolverCSV("", "IMPORTKIT_TESTE_CSV", "dados", "x.csv")
	require.NoError(t, err)
	assert.Equal(t, "/env.csv", c, "depois a variável de ambiente")

	t.Setenv("IMPORTKIT_TESTE_CSV", "")
	c, err = importkit.ResolverCSV("", "IMPORTKIT_TESTE_CSV", "dados", "erp", "x.csv")
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(filepath.ToSlash(c), "/dados/erp/x.csv"), c)

	vazio := t.TempDir() // criado antes: a limpeza dele roda depois da volta ao cwd
	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(wd) })
	require.NoError(t, os.Chdir(vazio))
	_, err = importkit.ResolverCSV("", "IMPORTKIT_TESTE_CSV", "x.csv")
	assert.ErrorContains(t, err, "não foi possível localizar a raiz do projeto")
}

type linha struct{ a, b string }

func parseLinha(r []string) (linha, error) {
	if r[0] == "ruim" {
		return linha{}, errors.New("campo a inválido")
	}
	return linha{r[0], r[1]}, nil
}

func TestImportkit_LerCSV(t *testing.T) {
	logs := capturarLog(t)
	f := importkit.Formato{Prefixo: "importteste: x.csv", Campos: 2}
	linhas, invalidas, err := importkit.LerCSV(strings.NewReader("a,b\n1,2\nruim,3\n1,2,3\n4,5\n"), f, parseLinha)
	require.NoError(t, err)
	assert.Equal(t, []linha{{"1", "2"}, {"4", "5"}}, linhas)
	assert.Equal(t, 2, invalidas)
	assert.Contains(t, logs.String(), "importteste: x.csv linha 3: campo a inválido")
	assert.Contains(t, logs.String(), "importteste: x.csv linha 4: erro de leitura CSV")

	_, _, err = importkit.LerCSV(strings.NewReader(""), f, parseLinha)
	assert.ErrorContains(t, err, "lendo cabeçalho")

	lerNada := func(io.Reader) ([]linha, int, error) { return nil, 0, nil }
	_, _, err = importkit.LerArquivo(filepath.Join(t.TempDir(), "nao-existe.csv"), lerNada)
	assert.ErrorContains(t, err, "abrindo arquivo")

	caminho := filepath.Join(t.TempDir(), "ok.csv")
	require.NoError(t, os.WriteFile(caminho, []byte("a,b\n7,8\n"), 0o600))
	linhas, _, err = importkit.LerArquivo(caminho, func(r io.Reader) ([]linha, int, error) { return importkit.LerCSV(r, f, parseLinha) })
	require.NoError(t, err)
	assert.Equal(t, []linha{{"7", "8"}}, linhas)
}

func TestImportkit_UpsertGravar(t *testing.T) {
	logs := capturarLog(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	u := importkit.Upsert[linha]{
		SQL:         "INSERT INTO t (a, b) VALUES (?, ?) ON DUPLICATE KEY UPDATE b = VALUES(b)",
		ErroPrepare: "prepare (t) falhou",
		Args: func(l linha) ([]any, bool) {
			if l.a == "sem-ref" {
				log.Printf("importteste: a=%s sem referência", l.a)
				return nil, false
			}
			return []any{l.a, l.b}, true
		},
		LogFalha: func(l linha, err error) { log.Printf("importteste: erro no upsert de a=%s: %v", l.a, err) },
	}

	prep := mock.ExpectPrepare(`INSERT INTO t`)
	prep.ExpectExec().WithArgs("1", "x").WillReturnResult(sqlmock.NewResult(1, 1)) // inserido
	prep.ExpectExec().WithArgs("2", "y").WillReturnResult(sqlmock.NewResult(0, 2)) // atualizado
	prep.ExpectExec().WithArgs("3", "z").WillReturnResult(sqlmock.NewResult(0, 0)) // 0 → atualizado (robustez)
	prep.ExpectExec().WithArgs("4", "w").WillReturnError(errTeste)
	prep.WillBeClosed()

	c, err := u.Gravar(db, []linha{{"1", "x"}, {"2", "y"}, {"sem-ref", "-"}, {"3", "z"}, {"4", "w"}})
	require.NoError(t, err)
	assert.Equal(t, importkit.Contagem{Inseridos: 1, Atualizados: 2, Falhas: 2}, c)
	assert.Contains(t, logs.String(), "importteste: a=sem-ref sem referência")
	assert.Contains(t, logs.String(), "importteste: erro no upsert de a=4: falhou")
	assert.NoError(t, mock.ExpectationsWereMet())

	mock.ExpectPrepare(`INSERT INTO t`).WillReturnError(errTeste)
	_, err = u.Gravar(db, []linha{{"1", "x"}})
	require.ErrorIs(t, err, errTeste)
	assert.Contains(t, err.Error(), "prepare (t) falhou")
}

func TestImportkit_Lookups(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT cliente_id_origem FROM clientes`).WillReturnRows(sqlmock.NewRows([]string{"o"}).AddRow(int64(7)).AddRow(int64(9)))
	m, err := importkit.IDsDeOrigem(db, "SELECT cliente_id_origem FROM clientes")
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{7: 7, 9: 9}, m)

	mock.ExpectQuery(`SELECT id, sku FROM produtos`).WillReturnRows(sqlmock.NewRows([]string{"id", "sku"}).AddRow(int64(1), "SKU-1"))
	p, err := importkit.IDsPorChave[string](db, "SELECT id, sku FROM produtos")
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{"SKU-1": 1}, p)

	mock.ExpectQuery(`SELECT`).WillReturnError(errTeste)
	_, err = importkit.IDsDeOrigem(db, "SELECT x FROM y")
	assert.ErrorIs(t, err, errTeste)

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{"o"}).AddRow("não-é-número"))
	_, err = importkit.IDsDeOrigem(db, "SELECT x FROM y")
	assert.Error(t, err, "falha de scan")

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{"id", "sku"}).AddRow("x", "y"))
	_, err = importkit.IDsPorChave[string](db, "SELECT id, sku FROM produtos")
	assert.Error(t, err, "falha de scan")

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{"o"}).AddRow(int64(1)).RowError(0, errTeste))
	_, err = importkit.IDsDeOrigem(db, "SELECT x FROM y")
	assert.ErrorIs(t, err, errTeste, "rows.Err")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestImportkit_ContagemRegistrar(t *testing.T) {
	var c importkit.Contagem
	for _, afetadas := range []int64{1, 2, 0, 1} {
		c.Registrar(sqlmock.NewResult(0, afetadas))
	}
	assert.Equal(t, importkit.Contagem{Inseridos: 2, Atualizados: 2}, c)
	var _ sql.Result = sqlmock.NewResult(0, 0)
}

// M-04: o CNPJ inválido nunca vai para o log do importador (antes do ARQ-07
// o erro de parsing citava o valor cru).
func TestARQ07_M04_CNPJInvalidoNaoVaiParaOLog(t *testing.T) {
	logs := capturarLog(t)
	const cnpj = "12.ABC.345/0001-99X"
	csv := "cliente_id,cnpj,razao_social,segmento,cidade,uf,bairro,data_cadastro,ativo\n" +
		"10," + cnpj + ",Loja,Varejo,SP,SP,Centro,2024-01-02,S\n"
	_, invalidas, err := clientes.ReadCSV(strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 1, invalidas)
	assert.Contains(t, logs.String(), "importclientes: linha 2: cnpj inválido")
	assert.NotContains(t, logs.String(), cnpj)
	assert.NotContains(t, logs.String(), "ABC")
}
