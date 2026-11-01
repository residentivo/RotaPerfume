package arquitetura_test

// Varredura dos pacotes de produção dos DOIS módulos (R-g): a API (raiz
// ../..) e o shared (../../../shared, o mesmo diretório do replace). Usa
// go/build (respeita build tags, ignora _test.go) e go/parser, sem depender
// do binário `go` no PATH.

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	modAPI    = "github.com/rotaperfumes/rotaperfumes-api"
	modShared = "github.com/rotaperfumes/shared"
	pkgKernel = modShared + "/kernel"
	pkgVlog   = modShared + "/vlog"
	pkgHttpx  = modAPI + "/internal/httpx"
)

var (
	raizAPI    = filepath.Join("..", "..")
	raizShared = filepath.Join("..", "..", "..", "shared")
)

// pacote é um pacote de produção com os arquivos já parseados.
type pacote struct {
	importPath string
	imports    []string
	arquivos   []arquivo
}

type arquivo struct {
	pkg     string // import path do pacote
	caminho string // relativo à raiz do módulo, com "/"
	fset    *token.FileSet
	f       *ast.File
}

func (a arquivo) pos(p token.Pos) string {
	return a.pkg + " " + path.Base(a.caminho) + ":" + strconv.Itoa(a.fset.Position(p).Line)
}

var (
	cacheOnce    sync.Once
	cachePacotes []pacote
	cacheErr     error
)

// pacotesDosDoisModulos varre e parseia uma única vez por execução.
func pacotesDosDoisModulos(t *testing.T) []pacote {
	t.Helper()
	cacheOnce.Do(func() {
		for _, m := range []struct{ raiz, mod string }{{raizAPI, modAPI}, {raizShared, modShared}} {
			ps, err := varrerModulo(m.raiz, m.mod)
			if err != nil {
				cacheErr = err
				return
			}
			cachePacotes = append(cachePacotes, ps...)
		}
	})
	if cacheErr != nil {
		t.Fatalf("varredura: %v", cacheErr)
	}
	if len(cachePacotes) == 0 {
		t.Fatal("nenhum pacote encontrado — a varredura quebrou?")
	}
	return cachePacotes
}

func varrerModulo(raiz, modulo string) ([]pacote, error) {
	var out []pacote
	err := filepath.WalkDir(raiz, func(dir string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		nome := d.Name()
		if dir != raiz && (strings.HasPrefix(nome, ".") || strings.HasPrefix(nome, "_") || nome == "testdata" || nome == "vendor" || nome == "node_modules") {
			return filepath.SkipDir
		}
		bp, err := build.ImportDir(dir, 0)
		if err != nil {
			return nil // sem arquivos Go de produção
		}
		rel, err := filepath.Rel(raiz, dir)
		if err != nil {
			return err
		}
		ip := modulo
		if rel != "." {
			ip = modulo + "/" + filepath.ToSlash(rel)
		}
		p := pacote{importPath: ip, imports: bp.Imports}
		fset := token.NewFileSet()
		for _, nomeArq := range bp.GoFiles {
			f, err := parser.ParseFile(fset, filepath.Join(dir, nomeArq), nil, 0)
			if err != nil {
				return err
			}
			p.arquivos = append(p.arquivos, arquivo{pkg: ip, caminho: filepath.ToSlash(filepath.Join(rel, nomeArq)), fset: fset, f: f})
		}
		out = append(out, p)
		return nil
	})
	return out, err
}

// fonteSintetica parseia um arquivo em memória (para provar que cada regra
// detecta a violação).
func fonteSintetica(t *testing.T, pkg, nome, src string) arquivo {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, nome, src, 0)
	if err != nil {
		t.Fatalf("fonte sintética %s: %v", nome, err)
	}
	return arquivo{pkg: pkg, caminho: nome, fset: fset, f: f}
}

// nomesDoImport devolve como o arquivo referencia importPath ("" se não importa).
func nomesDoImport(f *ast.File, importPath string) string {
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p != importPath {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				return ""
			}
			return imp.Name.Name
		}
		return path.Base(p)
	}
	return ""
}

// chamadaDe reconhece `alias.Func(...)` (ou `Func(...)` dentro do próprio
// pacote, quando noProprioPacote) e devolve o nome da função.
func chamadaDe(call *ast.CallExpr, alias string, noProprioPacote bool) (string, bool) {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok && alias != "" && x.Name == alias {
			return fn.Sel.Name, true
		}
	case *ast.Ident:
		if noProprioPacote {
			return fn.Name, true
		}
	}
	return "", false
}

// chamadasA percorre as chamadas de funções do pacote alvo em um arquivo.
func chamadasA(a arquivo, alvo string, visitar func(nome string, call *ast.CallExpr)) {
	alias := nomesDoImport(a.f, alvo)
	proprio := a.pkg == alvo
	if alias == "" && !proprio {
		return
	}
	ast.Inspect(a.f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if nome, ok := chamadaDe(call, alias, proprio); ok {
			visitar(nome, call)
		}
		return true
	})
}

func dentroDe(ip, prefixo string) bool {
	return ip == prefixo || strings.HasPrefix(ip, prefixo+"/")
}

// contextos previstos no blueprint (§2.1).
var contextos = []string{"catalogo", "identidade", "comercial", "vendas", "analytics", "notificacao"}

// pacoteDeContexto: shared/<ctx>, shared/<ctx>/<ctx>app, shared/<ctx>/<ctx>mysql…
func pacoteDeContexto(ip string) bool {
	for _, c := range contextos {
		if dentroDe(ip, modShared+"/"+c) {
			return true
		}
	}
	return false
}

// pacoteNovo: código escrito na arquitetura nova (as regras de conteúdo do
// vlog valem integralmente aqui; o legado é coberto pelo TST05 em runtime
// até cada contexto ser migrado).
func pacoteNovo(ip string) bool {
	return dentroDe(ip, pkgKernel) || dentroDe(ip, modAPI+"/internal") || ip == modShared+"/db" ||
		dentroDe(ip, modShared+"/adapters") || pacoteDeContexto(ip)
}
