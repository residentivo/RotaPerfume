package cmdutil_test

// TST-03 (Lote 7): utilitários dos comandos (raiz do projeto, .env e
// Opener do MySQL). Não usa t.Parallel: altera cwd e variáveis de ambiente.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	antigo, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(antigo) })
}

// arvore cria base/<niveis...> e devolve o caminho mais fundo.
func arvore(t *testing.T, base string, niveis ...string) string {
	t.Helper()
	p := filepath.Join(append([]string{base}, niveis...)...)
	require.NoError(t, os.MkdirAll(p, 0o755))
	return p
}

func mesmoDir(t *testing.T, want, got string) {
	t.Helper()
	w, err := filepath.EvalSymlinks(want)
	require.NoError(t, err)
	g, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)
	assert.Equal(t, w, g)
}

func TestFindProjectRoot(t *testing.T) {
	t.Run("no repositório real", func(t *testing.T) {
		root, err := cmdutil.FindProjectRoot()
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(root, "apis", "shared", "go.mod"))
	})

	casos := []struct {
		nome string
		cwd  []string // caminho, a partir da raiz falsa, de onde o comando roda
		acha bool
	}{
		{nome: "na própria raiz", cwd: nil, acha: true},
		{nome: "em apis/shared (make db-import-*)", cwd: []string{"apis", "shared"}, acha: true},
		{nome: "em apis/shared/cmd/importclientes", cwd: []string{"apis", "shared", "cmd", "importclientes"}, acha: true},
		{nome: "6 níveis abaixo: fora do alcance", cwd: []string{"a", "b", "c", "d", "e", "f"}, acha: false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			root := t.TempDir()
			shared := arvore(t, root, "apis", "shared")
			require.NoError(t, os.WriteFile(filepath.Join(shared, "go.mod"), []byte("module x\n"), 0o600))
			chdir(t, arvore(t, root, c.cwd...))

			got, err := cmdutil.FindProjectRoot()
			if !c.acha {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "apis/shared/go.mod")
				return
			}
			require.NoError(t, err)
			mesmoDir(t, root, got)
		})
	}

	t.Run("fora de qualquer repositório", func(t *testing.T) {
		chdir(t, t.TempDir())
		_, err := cmdutil.FindProjectRoot()
		require.Error(t, err)
	})
}

func TestLoadEnvFromCwd(t *testing.T) {
	const chave = "CMDUTIL_TESTE_VAR"

	t.Run("carrega o .env de um diretório acima", func(t *testing.T) {
		t.Setenv(chave, "")
		require.NoError(t, os.Unsetenv(chave))
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte(chave+"=do_env\n"), 0o600))
		chdir(t, arvore(t, root, "apis", "shared"))

		cmdutil.LoadEnvFromCwd()
		assert.Equal(t, "do_env", os.Getenv(chave))
	})
	t.Run("não sobrescreve variável já definida", func(t *testing.T) {
		t.Setenv(chave, "do_shell")
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte(chave+"=do_env\n"), 0o600))
		chdir(t, root)

		cmdutil.LoadEnvFromCwd()
		assert.Equal(t, "do_shell", os.Getenv(chave))
	})
	t.Run("sem .env segue sem erro", func(t *testing.T) {
		t.Setenv(chave, "")
		require.NoError(t, os.Unsetenv(chave))
		chdir(t, arvore(t, t.TempDir(), "a", "b", "c", "d", "e", "f", "g"))

		assert.NotPanics(t, cmdutil.LoadEnvFromCwd)
		_, definida := os.LookupEnv(chave)
		assert.False(t, definida)
	})
}

func TestOpenMySQL(t *testing.T) {
	t.Run("DSN válido abre sem conectar (Ping fica com o Run)", func(t *testing.T) {
		open := cmdutil.OpenMySQL("u:p@tcp(127.0.0.1:1)/db?parseTime=true")
		conn, err := open()
		require.NoError(t, err)
		require.NotNil(t, conn)
		assert.NoError(t, conn.Close())
	})
	t.Run("DSN inválido falha no sql.Open", func(t *testing.T) {
		_, err := cmdutil.OpenMySQL("isto não é um DSN")()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sql.Open")
	})
}

// TestRaizDoDisco cobre a parada da busca ao chegar na raiz do volume
// (filepath.Dir(dir) == dir) antes de esgotar os níveis.
func TestRaizDoDisco(t *testing.T) {
	const chave = "CMDUTIL_TESTE_RAIZ"
	raiz := filepath.VolumeName(os.TempDir()) + string(filepath.Separator)
	if _, err := os.Stat(filepath.Join(raiz, "apis", "shared", "go.mod")); err == nil {
		t.Skip("a raiz do disco é, ela mesma, um repositório")
	}
	chdir(t, raiz)

	_, err := cmdutil.FindProjectRoot()
	assert.Error(t, err)

	t.Setenv(chave, "")
	require.NoError(t, os.Unsetenv(chave))
	assert.NotPanics(t, cmdutil.LoadEnvFromCwd)
}
