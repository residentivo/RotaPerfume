// Package cmdutil reúne os utilitários comuns aos comandos de linha de
// comando de backend/shared/cmd (importadores e ferramentas): localizar a raiz
// do repositório e carregar o .env.
package cmdutil

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/rotaperfumes/shared/projroot"
)

// maxNiveis limita quantos diretórios acima do cwd são inspecionados.
const maxNiveis = 6

// FindProjectRoot sobe a árvore de diretórios a partir do cwd até achar a
// raiz do repositório (identificada por backend/shared/go.mod). Delegado a
// projroot.Find, que o pacote config também usa (SEC-29).
func FindProjectRoot() (string, error) {
	return projroot.Find()
}

// LoadEnvFromCwd tenta carregar o .env da raiz do projeto subindo diretórios
// a partir do working directory. Não retorna erro — se não achar, segue sem .env.
func LoadEnvFromCwd() {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	dir := wd
	for i := 0; i < maxNiveis; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
			_ = godotenv.Load(filepath.Join(dir, ".env"))
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}
