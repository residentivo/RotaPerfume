// Package projroot localiza a raiz do repositório (a pasta que contém
// backend/shared/go.mod). É um pacote folha, sem dependências do módulo, para
// que tanto config quanto cmdutil possam usá-lo sem ciclo de import.
package projroot

import (
	"fmt"
	"os"
	"path/filepath"
)

// maxNiveis limita quantos diretórios acima do cwd são inspecionados.
const maxNiveis = 6

// Find sobe a árvore de diretórios a partir do cwd até achar a raiz do
// repositório (identificada por backend/shared/go.mod).
func Find() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < maxNiveis; i++ {
		if _, err := os.Stat(filepath.Join(dir, "backend", "shared", "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("não foi possível localizar backend/shared/go.mod a partir de %s", dir)
}
