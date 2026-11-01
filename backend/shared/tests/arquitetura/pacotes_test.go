package arquitetura_test

import (
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pacote é um pacote de produção do módulo (só arquivos não-teste).
type pacote struct {
	importPath string
	imports    []string
}

const mod = "github.com/rotaperfumes/shared"

// raizModulo: os testes rodam com cwd = tests/arquitetura.
var raizModulo = filepath.Join("..", "..")

// pacotesDoModulo lê os imports de cada pacote com go/build (respeita build
// tags e ignora _test.go), sem depender do binário `go` no PATH.
func pacotesDoModulo(t *testing.T, raiz, modulo string) []pacote {
	t.Helper()
	var out []pacote
	err := filepath.WalkDir(raiz, func(dir string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		nome := d.Name()
		if dir != raiz && (strings.HasPrefix(nome, ".") || strings.HasPrefix(nome, "_") || nome == "testdata" || nome == "vendor") {
			return filepath.SkipDir
		}
		bp, err := build.ImportDir(dir, 0)
		if err != nil {
			return nil // sem arquivos Go de produção (ex.: só _test.go)
		}
		rel, err := filepath.Rel(raiz, dir)
		if err != nil {
			return err
		}
		ip := modulo
		if rel != "." {
			ip = modulo + "/" + filepath.ToSlash(rel)
		}
		out = append(out, pacote{importPath: ip, imports: bp.Imports})
		return nil
	})
	if err != nil {
		t.Fatalf("varrendo %s: %v", raiz, err)
	}
	if len(out) == 0 {
		t.Fatalf("nenhum pacote encontrado em %s — a varredura quebrou?", raiz)
	}
	return out
}

func existe(t *testing.T, caminho string) bool {
	t.Helper()
	_, err := os.Stat(caminho)
	return err == nil
}
