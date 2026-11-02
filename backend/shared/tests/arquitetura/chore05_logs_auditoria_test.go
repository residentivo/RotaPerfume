package arquitetura_test

// CHORE-05 (Lote 31): todo log de auditoria de escrita ("por usuario"/"por
// admin") traz o id do Principal além do role (usuario_id=%d role=%s, ou
// admin_id=%d no reset administrativo), e nunca e-mail/nome. Varredura
// estática dos fontes (sem _test) do shared e da API.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	reLogAuditoria = regexp.MustCompile(`log\.Printf\("[^"]*\bpor (usuario|admin)[^"]*"`)
	reComID        = regexp.MustCompile(`por usuario_id=%d role=%s`)
	rePII          = regexp.MustCompile(`(?i)\b(email|nome|cnpj|telefone)=%`)
)

func TestCHORE05_LogsDeAuditoriaTemIDEEmPII(t *testing.T) {
	var achados, violacoes []string
	for _, raiz := range []string{"../..", "../../../rotaperfumes-api/internal"} {
		err := filepath.WalkDir(raiz, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "tests" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for _, m := range reLogAuditoria.FindAllString(string(src), -1) {
				achados = append(achados, m)
				if !reComID.MatchString(m) || rePII.MatchString(m) {
					violacoes = append(violacoes, p+": "+m)
				}
			}
			return nil
		})
		require.NoError(t, err)
	}
	assert.Greater(t, len(achados), 20, "a varredura encontrou os logs de auditoria")
	assert.Empty(t, violacoes)
}
