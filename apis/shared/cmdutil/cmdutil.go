// Package cmdutil reúne os utilitários comuns aos comandos de linha de
// comando de apis/shared/cmd (importadores e ferramentas): localizar a raiz
// do repositório e carregar o .env.
package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// maxNiveis limita quantos diretórios acima do cwd são inspecionados.
const maxNiveis = 6

// FindProjectRoot sobe a árvore de diretórios a partir do cwd até achar a
// raiz do repositório (identificada por apis/shared/go.mod).
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < maxNiveis; i++ {
		if _, err := os.Stat(filepath.Join(dir, "apis", "shared", "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("não foi possível localizar apis/shared/go.mod a partir de %s", dir)
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
