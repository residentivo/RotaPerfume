package bootstrap_test

// IMG-01 (Lote 26, TestBrain): composition root das imagens de produto.
//   - PRODUTO_IMAGENS_DIR inutilizável (não cria, não abre, não grava) →
//     log.Fatalf na subida (fail-fast), conferido num subprocesso;
//   - diretório válido → adapter aberto (cria o dir); Config sem diretório
//     (montada à mão) → imagens desligadas.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/config"
)

const envFailFast = "IMG01_FAILFAST_DIR"

func TestIMG01_ArmazenamentoDoConfig_FailFast(t *testing.T) {
	if dir, ok := os.LookupEnv(envFailFast); ok {
		bootstrap.ArmazenamentoDoConfig(&config.Config{ProdutoImagensDir: dir})
		os.Exit(0) // não deveria chegar aqui
	}
	base := t.TempDir()
	arquivo := filepath.Join(base, "arquivo")
	require.NoError(t, os.WriteFile(arquivo, []byte("x"), 0o600))
	casos := map[string]func(t *testing.T) string{
		"caminho é arquivo": func(*testing.T) string { return arquivo },
		"pai é arquivo":     func(*testing.T) string { return filepath.Join(arquivo, "sub") },
		"sem escrita": func(t *testing.T) string {
			if os.Geteuid() == 0 {
				t.Skip("root ignora permissões")
			}
			d := filepath.Join(base, "ro")
			require.NoError(t, os.Mkdir(d, 0o500))
			t.Cleanup(func() { _ = os.Chmod(d, 0o700) })
			return d
		},
	}
	for nome, dir := range casos {
		t.Run(nome, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestIMG01_ArmazenamentoDoConfig_FailFast$")
			cmd.Env = append(os.Environ(), envFailFast+"="+dir(t))
			out, err := cmd.CombinedOutput()
			var saida *exec.ExitError
			require.ErrorAs(t, err, &saida, "a subida tem de falhar: %s", out)
			assert.Equal(t, 1, saida.ExitCode())
			assert.Contains(t, string(out), "[server] imagens de produto: catalogofs:")
		})
	}
}

func TestIMG01_ArmazenamentoDoConfig(t *testing.T) {
	assert.Nil(t, bootstrap.ArmazenamentoDoConfig(&config.Config{}), "sem diretório: imagens desligadas")

	dir := filepath.Join(t.TempDir(), "novo", "produtos-imagens")
	a := bootstrap.ArmazenamentoDoConfig(&config.Config{ProdutoImagensDir: dir})
	require.NotNil(t, a)
	defer a.Close()
	assert.DirExists(t, dir)
	ents, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, ents, "o teste de escrita não deixa arquivo")
}
