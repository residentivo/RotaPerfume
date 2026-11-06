package deploy_test

// SEC-60 (Lote 39): o diretório do --admin-senha-arquivo do seed-producao.sh
// precisa ser privado (conferir_dir_privado), conferido antes de ler a senha
// do banco e o pepper; o caminho passa pelo mesmo tipo de regex das opções de
// diretório.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSEC60_OrdemDasConferencias(t *testing.T) {
	sp := lerRaiz(t, "deploy", "seed-producao.sh")
	iDir := strings.Index(sp, `conferir_dir_privado --admin-senha-arquivo "$(dirname -- "$ADMIN_SENHA_ARQ")" existe`)
	require.GreaterOrEqual(t, iDir, 0)
	for nome, trecho := range map[string]string{
		"senha do banco": `SENHA_DB=$(<"$SENHA_DB_ARQ")`,
		"pepper":         `PEPPER=""`,
		"gravação":       `install -m 600 -o root -g root "$TRAB/saida/admin.senha" "$ADMIN_SENHA_ARQ"`,
	} {
		i := strings.Index(sp, trecho)
		require.GreaterOrEqual(t, i, 0, nome)
		assert.Less(t, iDir, i, "diretório conferido antes de: %s", nome)
	}
}

// A sintaxe é conferida antes do teste de root: roda sem namespace, e o
// --admin-senha-prompt não usa o arquivo.
func TestSEC60_CaminhoInvalidoRecusado(t *testing.T) {
	precisaBash(t)
	sc := caminhoRaiz(t, "deploy", "seed-producao.sh")
	rodar := func(args ...string) string {
		cmd := exec.Command("bash", append([]string{sc}, args...)...)
		cmd.Env = ambienteLimpo()
		out, err := cmd.CombinedOutput()
		require.Error(t, err, "%s", out)
		return string(out)
	}
	casos := []struct{ nome, valor, msg string }{
		{"relativo", "rel/admin.senha", "precisa de caminho absoluto"},
		{"espaço", "/root/a b", "--admin-senha-arquivo invalido"},
		{"vírgula", "/root/a,b", "--admin-senha-arquivo invalido"},
		{"quebra de linha", "/root/a\nb", "--admin-senha-arquivo invalido"},
		{"ponto-ponto", "/root/../a", "--admin-senha-arquivo invalido"},
		{"barras repetidas", "/root//a", "--admin-senha-arquivo invalido"},
		{"barra no fim", "/root/a/", "--admin-senha-arquivo invalido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			assert.Contains(t, rodar("--admin-senha-arquivo", c.valor), c.msg)
		})
	}
	t.Run("--admin-senha-prompt ignora o arquivo", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("como root o script segue além da sintaxe")
		}
		out := rodar("--admin-senha-prompt", "--admin-senha-arquivo", "/root/a b")
		assert.NotContains(t, out, "--admin-senha-arquivo invalido")
		assert.Contains(t, out, "rode como root")
	})
}

func TestSEC60_Ensaio_DiretorioNaoPrivadoRecusado(t *testing.T) {
	casos := []struct {
		nome string
		prep func(t *testing.T, base string) string
		msg  string
	}{
		{"modo 1777", func(t *testing.T, base string) string {
			d := filepath.Join(base, "publico")
			require.NoError(t, os.Mkdir(d, 0o700))
			require.NoError(t, os.Chmod(d, os.ModeSticky|0o777))
			return filepath.Join(d, "admin.senha")
		}, "precisa ser do root com modo 700"},
		{"modo 755", func(t *testing.T, base string) string {
			d := filepath.Join(base, "aberto")
			require.NoError(t, os.Mkdir(d, 0o700))
			require.NoError(t, os.Chmod(d, 0o755))
			return filepath.Join(d, "admin.senha")
		}, "precisa ser do root com modo 700"},
		{"o próprio diretório é symlink", func(t *testing.T, base string) string {
			d := filepath.Join(base, "real")
			require.NoError(t, os.Mkdir(d, 0o700))
			require.NoError(t, os.Symlink(d, filepath.Join(base, "lnk")))
			return filepath.Join(base, "lnk", "admin.senha")
		}, "e um link simbolico"},
		{"componente symlink", func(t *testing.T, base string) string {
			d := filepath.Join(base, "real", "sub")
			require.NoError(t, os.MkdirAll(d, 0o700))
			require.NoError(t, os.Symlink(filepath.Join(base, "real"), filepath.Join(base, "lnk")))
			return filepath.Join(base, "lnk", "sub", "admin.senha")
		}, "nao e o caminho real"},
		{"ancestral com escrita de outros", func(t *testing.T, base string) string {
			pai := filepath.Join(base, "pai")
			d := filepath.Join(pai, "sub")
			require.NoError(t, os.MkdirAll(d, 0o700))
			require.NoError(t, os.Chmod(pai, 0o777))
			return filepath.Join(d, "admin.senha")
		}, "tem escrita para grupo/outros"},
		{"diretório inexistente", func(t *testing.T, base string) string {
			return filepath.Join(base, "nao-existe", "admin.senha")
		}, "nao existe"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioSeed(t)
			arq := c.prep(t, e.dir)
			out, err := e.rodar("--pepper-file", "/root/pepper.txt", "--admin-senha-arquivo", arq)
			e.recusaSemEfeito(t, out, err, c.msg)
			assert.Contains(t, out, "--admin-senha-arquivo")
			assert.Empty(t, e.chamadasDeBanco(), "nenhum cliente mariadb")
		})
	}
}

func TestSEC60_Ensaio_DiretorioPrivadoAceito(t *testing.T) {
	e := novoEnsaioSeed(t)
	d := filepath.Join(e.dir, "privado")
	require.NoError(t, os.Mkdir(d, 0o700))
	require.NoError(t, os.Chmod(d, 0o700))
	arq := filepath.Join(d, "admin.senha")
	out, err := e.rodar("--pepper-file", "/root/pepper.txt", "--admin-senha-arquivo", arq)
	require.NoError(t, err, out)
	st, err := os.Stat(arq)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	assert.NotContains(t, out, senhaAdmSeed)
}

func (e *ensaioSeed) chamadasDeBanco() []chamadaDocker {
	e.t.Helper()
	var out []chamadaDocker
	for _, c := range e.chamadas() {
		if c.mariadb() {
			out = append(out, c)
		}
	}
	return out
}
