package config_test

// TST-19 (Lote 35): catraca AST para conexões com a config do ambiente fora
// do helper do SEC-50 (shared/testkit/integracao).
//
// Varre TODOS os .go de backend/shared e backend/rotaperfumes-api (não só os
// _test.go), resolvendo as chamadas pelo CAMINHO do import (um alias como
// `banco "github.com/rotaperfumes/shared/db"` não escapa). Regras:
//   - _test.go: uma função (com as closures dentro dela) que abre conexão
//     (abridores abaixo) com a config do ambiente (config.Load*, DB_* via
//     os.Getenv/LookupEnv, os.Environ, cmdutil.AmbientePadrao ou um
//     argumento X.DSN()) precisa chamar integracao.Config/Abrir/
//     ConfigDoAmbiente ANTES da primeira abertura;
//   - código de produção: todo arquivo que chama um abridor (ou monta o
//     ambiente real) está em excecoesTST19, com a justificativa;
//   - dot import dos pacotes vigiados é proibido (não dá para resolver);
//   - catraca do SEC-50 ampliada: nenhum _test.go chama os.Environ() fora
//     de excecoesEnvironTST19, nem lê INTEGRATION por Getenv/LookupEnv.
// Fica fora só o shared/testkit/integracao (o próprio helper).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pkgConfig     = "github.com/rotaperfumes/shared/config"
	pkgDB         = "github.com/rotaperfumes/shared/db"
	pkgCmdutil    = "github.com/rotaperfumes/shared/cmdutil"
	pkgResetPw    = "github.com/rotaperfumes/shared/tools/resetpassword"
	pkgBootstrap  = "github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	pkgIntegracao = "github.com/rotaperfumes/shared/testkit/integracao"
)

type funcPkg struct{ pkg, nome string }

// Abridores: abrem (ou devolvem quem abre) um pool real.
var abridoresTST19 = map[funcPkg]bool{
	{pkgDB, "Open"}:                true,
	{"database/sql", "Open"}:       true,
	{"database/sql", "OpenDB"}:     true,
	{pkgCmdutil, "OpenMySQL"}:      true,
	{pkgCmdutil, "AmbientePadrao"}: true, // Config: config.Load + Abrir: OpenMySQL
	{pkgResetPw, "AbrirMySQL"}:     true,
	{pkgResetPw, "DepsPadrao"}:     true, // AbrirBanco: AbrirMySQL
	{pkgBootstrap, "Run"}:          true, // abrirBanco(cfg)
}

// Fontes de config do ambiente (além de DB_* lidas direto e de X.DSN()).
var fontesAmbienteTST19 = map[funcPkg]bool{
	{pkgConfig, "Load"}:                 true,
	{pkgConfig, "LoadDBTLS"}:            true,
	{pkgConfig, "LoadSemCredenciaisDB"}: true,
	{pkgCmdutil, "AmbientePadrao"}:      true,
	{"os", "Environ"}:                   true,
}

var guardasTST19 = map[funcPkg]bool{
	{pkgIntegracao, "Config"}:           true,
	{pkgIntegracao, "Abrir"}:            true,
	{pkgIntegracao, "ConfigDoAmbiente"}: true,
}

// vigiadosTST19: pacotes cujo dot import cegaria a resolução.
var vigiadosTST19 = map[string]bool{pkgConfig: true, pkgDB: true, pkgCmdutil: true, pkgResetPw: true,
	pkgBootstrap: true, pkgIntegracao: true, "database/sql": true}

// excecoesTST19: arquivos de PRODUÇÃO (caminho relativo a backend/) que
// chamam um abridor. Cada um com a justificativa; o teste falha se um
// arquivo novo abrir conexão ou se a exceção ficar obsoleta.
var excecoesTST19 = map[string]string{
	"shared/db/db.go":                               "é o db.Open (sql.Open + Ping + DSN mascarado), usado por todos",
	"shared/cmdutil/db.go":                          "OpenMySQL, o Opener dos CLIs (composition root dos cmd/*)",
	"shared/tools/resetpassword/cli.go":             "AbrirMySQL/DepsPadrao do CLI resetpassword (operador, não teste)",
	"rotaperfumes-api/internal/bootstrap/server.go": "abrirBanco do servidor da API (config já validada no main)",
}

// cmdMainTST19: os cmd/*/main.go (package main, composition root dos CLIs e
// da API; nenhum teste importa um package main) montam o ambiente real
// (cmdutil.AmbientePadrao, config.Load + bootstrap.Run). Exceção por regra,
// não por arquivo: um cmd novo nasce fino (ARQ-07) e a lógica testável fica
// num pacote importável, que continua vigiado.
func cmdMainTST19(rel string) bool {
	partes := strings.Split(rel, "/")
	n := len(partes)
	return n >= 4 && partes[n-3] == "cmd" && partes[n-1] == "main.go"
}

// excecoesTesteTST19: funções de _test.go ("arquivo:Funcao") que chamam um
// abridor com o ambiente sem a guarda, cada uma com a justificativa.
var excecoesTesteTST19 = map[string]string{
	"shared/tests/cmdutil/arq07_cli_test.go:TestARQ07_AmbientePadrao": "só confere que os campos do AmbientePadrao estão preenchidos; não chama Config nem Abrir",
}

// excecoesEnvironTST19: _test.go que repassam os.Environ() a um processo
// filho (nenhum abre banco; o filho não roda integração).
var excecoesEnvironTST19 = map[string]string{
	"shared/tests/deploy/helpers_test.go":                          "ambienteLimpo: os.Environ() filtrado para os scripts de deploy com stubs",
	"shared/tests/config/sec21_makefile_setup_test.go":             "ambiente do make filho nos testes do Makefile (sem banco)",
	"rotaperfumes-api/tests/bootstrap/img01_armazenamento_test.go": "re-exec do próprio binário de teste (fail-fast do IMG-01, sem banco)",
}

type achadoTST19 struct {
	arquivo, funcao, motivo string
}

// analisarTST19 aplica as regras a um arquivo já parseado. ehTeste diz se
// é _test.go. Devolve os achados e se o arquivo chama algum abridor (ou
// os.Environ, em teste).
func analisarTST19(fset *token.FileSet, f *ast.File, rel string, ehTeste bool) (achados []achadoTST19, abre, environ bool) {
	imports := map[string]string{} // nome local -> caminho
	for _, imp := range f.Imports {
		caminho, _ := strconv.Unquote(imp.Path.Value)
		nome := caminho[strings.LastIndex(caminho, "/")+1:]
		if imp.Name != nil {
			nome = imp.Name.Name
		}
		if nome == "." && vigiadosTST19[caminho] {
			achados = append(achados, achadoTST19{rel, "-", "dot import de " + caminho + " (resolução impossível)"})
		}
		if nome == "_" || nome == "." {
			continue
		}
		imports[nome] = caminho
	}
	resolver := func(call *ast.CallExpr) (funcPkg, bool) {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return funcPkg{}, false
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Obj != nil { // id.Obj != nil: variável local, não pacote
			return funcPkg{}, false
		}
		p, ok := imports[id.Name]
		return funcPkg{p, sel.Sel.Name}, ok
	}
	leDBDireto := func(call *ast.CallExpr, fp funcPkg) bool {
		if fp.pkg != "os" || (fp.nome != "Getenv" && fp.nome != "LookupEnv") || len(call.Args) != 1 {
			return false
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return false
		}
		v, _ := strconv.Unquote(lit.Value)
		return strings.HasPrefix(v, "DB_") || v == "MYSQL_PWD"
	}
	usaDSN := func(call *ast.CallExpr) bool {
		achou := false
		for _, a := range call.Args {
			ast.Inspect(a, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "DSN" {
						achou = true
					}
				}
				return !achou
			})
		}
		return achou
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		var primeiraAbertura, primeiraGuarda token.Pos
		ambiente := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fp, ok := resolver(call)
			if !ok {
				return true
			}
			if fp.pkg == "os" && fp.nome == "Environ" {
				environ = true
			}
			if guardasTST19[fp] && (primeiraGuarda == token.NoPos || call.Pos() < primeiraGuarda) {
				primeiraGuarda = call.Pos()
			}
			if fontesAmbienteTST19[fp] || leDBDireto(call, fp) {
				ambiente = true
			}
			if abridoresTST19[fp] {
				abre = true
				if usaDSN(call) {
					ambiente = true
				}
				if primeiraAbertura == token.NoPos || call.Pos() < primeiraAbertura {
					primeiraAbertura = call.Pos()
				}
			}
			if fp.pkg == "os" && (fp.nome == "Getenv" || fp.nome == "LookupEnv") && len(call.Args) == 1 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Value == `"INTEGRATION"` {
					achados = append(achados, achadoTST19{rel, fn.Name.Name, "lê INTEGRATION direto (use integracao.Config/Abrir)"})
				}
			}
			return true
		})
		if !ehTeste || primeiraAbertura == token.NoPos || !ambiente {
			continue
		}
		switch {
		case primeiraGuarda == token.NoPos:
			achados = append(achados, achadoTST19{rel, fn.Name.Name, "abre conexão com a config do ambiente sem integracao.Config/Abrir"})
		case primeiraGuarda > primeiraAbertura:
			achados = append(achados, achadoTST19{rel, fn.Name.Name, "integracao.Config/Abrir depois da abertura (linha " +
				strconv.Itoa(fset.Position(primeiraAbertura).Line) + ")"})
		}
	}
	return achados, abre, environ
}

func analisarFonteTST19(t *testing.T, fonte string, ehTeste bool) []achadoTST19 {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", fonte, 0)
	require.NoError(t, err)
	achados, _, _ := analisarTST19(fset, f, "x.go", ehTeste)
	return achados
}

// O analisador pega os desvios (alias, closure, ordem, DSN()) e não acusa
// os usos legítimos.
func TestTST19_Analisador(t *testing.T) {
	cab := "package x_test\nimport (\n\t\"os\"\n\t\"testing\"\n\t\"database/sql\"\n\tcfgpkg \"" + pkgConfig + "\"\n\tbanco \"" + pkgDB +
		"\"\n\t\"" + pkgCmdutil + "\"\n\t\"" + pkgIntegracao + "\"\n)\nvar _ = os.Getenv\nvar _ = sql.Open\nvar _ testing.T\nvar _ = cmdutil.OpenMySQL\nvar _ = integracao.Abrir\nvar _ = cfgpkg.Load\nvar _ = banco.Open\n"
	casos := []struct {
		nome   string
		corpo  string
		teste  bool
		motivo string // "" = sem achado
	}{
		{"alias: config.Load + db.Open sem guarda", "func TestX(t *testing.T) { c, _ := cfgpkg.Load(); banco.Open(c.DSN()) }", true, "sem integracao.Config/Abrir"},
		{"só DSN() do ambiente", "func abrir(c interface{ DSN() string }) { banco.Open(c.DSN()) }", true, "sem integracao.Config/Abrir"},
		{"sql.Open com DB_HOST", "func TestX(t *testing.T) { sql.Open(\"mysql\", os.Getenv(\"DB_HOST\")) }", true, "sem integracao.Config/Abrir"},
		{"closure dentro da função", "func TestX(t *testing.T) { f := func() { c, _ := cfgpkg.Load(); banco.Open(c.DSN()) }; f() }", true, "sem integracao.Config/Abrir"},
		{"AmbientePadrao", "func TestX(t *testing.T) { _ = cmdutil.AmbientePadrao() }", true, "sem integracao.Config/Abrir"},
		{"guarda depois da abertura", "func TestX(t *testing.T) { c, _ := cfgpkg.Load(); banco.Open(c.DSN()); integracao.Config(t) }", true, "depois da abertura"},
		{"INTEGRATION por LookupEnv", "func TestX(t *testing.T) { os.LookupEnv(\"INTEGRATION\") }", true, "lê INTEGRATION direto"},
		{"INTEGRATION por Getenv", "func TestX(t *testing.T) { _ = os.Getenv(\"INTEGRATION\") }", true, "lê INTEGRATION direto"},
		{"guarda antes: ok", "func TestX(t *testing.T) { integracao.Config(t); c, _ := cfgpkg.Load(); banco.Open(c.DSN()) }", true, ""},
		{"Abrir: ok", "func TestX(t *testing.T) { _ = integracao.Abrir(t) }", true, ""},
		{"DSN literal (sem ambiente): ok", "func TestX(t *testing.T) { banco.Open(\"u:p@tcp(127.0.0.1:1)/x\") }", true, ""},
		{"config.Load sem abrir: ok", "func TestX(t *testing.T) { _, _ = cfgpkg.Load() }", true, ""},
		{"variável local chamada banco não é o pacote", "func TestX(t *testing.T) { banco := struct{ Open func(string) }{}; c, _ := cfgpkg.Load(); banco.Open(c.DSN) }", true, ""},
		{"produção não segue a regra de função", "func abrir() { c, _ := cfgpkg.Load(); banco.Open(c.DSN()) }", false, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			achados := analisarFonteTST19(t, cab+c.corpo+"\n", c.teste)
			if c.motivo == "" {
				assert.Empty(t, achados)
				return
			}
			require.Len(t, achados, 1, "%v", achados)
			assert.Contains(t, achados[0].motivo, c.motivo)
		})
	}
	t.Run("dot import é proibido", func(t *testing.T) {
		achados := analisarFonteTST19(t, "package x_test\nimport . \""+pkgDB+"\"\nvar _ = Open\n", true)
		require.Len(t, achados, 1)
		assert.Contains(t, achados[0].motivo, "dot import")
	})
}

func TestTST19_CatracaConexaoComAmbiente(t *testing.T) {
	backend := filepath.Join(raizProjeto(t), "backend")
	fset := token.NewFileSet()
	var achados []achadoTST19
	abridores := map[string]bool{}
	environs := map[string]bool{}
	vistos := map[string]int{}
	for _, mod := range []string{"shared", "rotaperfumes-api"} {
		err := filepath.WalkDir(filepath.Join(backend, mod), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(p, backend+string(filepath.Separator)))
			if d.IsDir() {
				switch d.Name() {
				case "testdata", "node_modules", "vendor", ".git":
					return filepath.SkipDir
				}
				if rel == "shared/testkit/integracao" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") {
				return nil
			}
			f, err := parser.ParseFile(fset, p, nil, 0)
			if err != nil {
				return err
			}
			ehTeste := strings.HasSuffix(p, "_test.go")
			if ehTeste {
				vistos["teste"]++
			} else {
				vistos["producao"]++
			}
			a, abre, environ := analisarTST19(fset, f, rel, ehTeste)
			achados = append(achados, a...)
			if abre && !ehTeste && !cmdMainTST19(rel) {
				abridores[rel] = true
			}
			if environ && ehTeste {
				environs[rel] = true
			}
			return nil
		})
		require.NoError(t, err)
	}
	assert.Greater(t, vistos["teste"], 100, "a varredura achou os _test.go")
	assert.Greater(t, vistos["producao"], 100, "a varredura achou o código de produção")
	var restantes []achadoTST19
	usadas := map[string]bool{}
	for _, a := range achados {
		chave := a.arquivo + ":" + a.funcao
		if _, ok := excecoesTesteTST19[chave]; ok && strings.Contains(a.motivo, "sem integracao.Config/Abrir") {
			usadas[chave] = true
			continue
		}
		restantes = append(restantes, a)
	}
	assert.Empty(t, restantes, "TST-19: abra a conexão de teste por integracao.Config/Abrir (SEC-50)")
	for chave, j := range excecoesTesteTST19 {
		assert.NotEmpty(t, strings.TrimSpace(j), chave)
		assert.True(t, usadas[chave], "exceção de teste obsoleta (tire da lista): %s", chave)
	}

	compara := func(nome string, achou map[string]bool, excecoes map[string]string) {
		var novos, obsoletos []string
		for a := range achou {
			if _, ok := excecoes[a]; !ok {
				novos = append(novos, a)
			}
		}
		for a, j := range excecoes {
			assert.NotEmpty(t, strings.TrimSpace(j), "%s: exceção %s sem justificativa", nome, a)
			if !achou[a] {
				obsoletos = append(obsoletos, a)
			}
		}
		sort.Strings(novos)
		sort.Strings(obsoletos)
		assert.Empty(t, novos, "%s: arquivo novo fora da lista de exceções (justifique ou use o testkit/integracao)", nome)
		assert.Empty(t, obsoletos, "%s: exceção obsoleta (tire da lista)", nome)
	}
	compara("abridores de produção", abridores, excecoesTST19)
	compara("os.Environ() em _test.go", environs, excecoesEnvironTST19)

	// O próprio helper existe e é o que fica fora da varredura.
	_, err := os.Stat(filepath.Join(backend, "shared", "testkit", "integracao", "integracao.go"))
	require.NoError(t, err)
}
