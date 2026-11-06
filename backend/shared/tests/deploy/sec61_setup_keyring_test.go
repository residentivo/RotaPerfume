package deploy_test

// SEC-61 (Lote 39): keyring do MariaDB já instalado que não confere com o
// MARIADB_GPG_FPR. O mariadb.sources confia nele, então o preflight roda em
// main() antes de qualquer apt: no modo instalar aborta; no --check registra
// FALTA (divergente) ou AVISO (não conferível). As funções são extraídas do
// scripts/setup-linux.sh e rodam com chaves descartáveis; nada é instalado.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// marcaDepoisPreflightSEC61 no lugar de etapa_basicas: só aparece se o
// preflight deixou o setup seguir para o apt.
const marcaDepoisPreflightSEC61 = "SEGUIU-PARA-O-APT"

func funcaoSetupSEC61(t *testing.T, nome string) string {
	t.Helper()
	fn := regexp.MustCompile(`(?ms)^` + nome + `\(\) \{.*?^\}$`).FindString(setupLinuxSEC58(t))
	require.NotEmpty(t, fn, nome)
	return fn
}

func rodarPreflightSEC61(t *testing.T, env []string, modo, apt, k, fpr string) (string, error) {
	t.Helper()
	sc := "aviso() { printf 'AVISO: %s\\n' \"$*\"; }\ninfo() { printf 'INFO: %s\\n' \"$*\"; }\n" +
		"registrar() { printf 'REGISTRAR %s|%s|%s\\n' \"$1\" \"$2\" \"$3\"; }\n" +
		"tem() { command -v \"$1\" >/dev/null 2>&1; }\n" +
		funcaoSetupSEC61(t, "conferir_chave_mariadb") + "\n" +
		funcaoSetupSEC61(t, "conferir_keyring_mariadb_instalado") + "\n" +
		funcaoSetupSEC61(t, "preflight_keyring_mariadb") + "\n" +
		"MODO=$1; APT=$2; MARIADB_GPG_FPR=$4\npreflight_keyring_mariadb \"$3\"\necho " + marcaDepoisPreflightSEC61 + "\n"
	cmd := exec.Command("bash", "-c", sc, "sec61", modo, apt, k, fpr)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ambienteSemGpgSEC61: PATH só com o que a função usa além do gpg.
func ambienteSemGpgSEC61(t *testing.T) []string {
	t.Helper()
	bin := t.TempDir()
	for _, c := range []string{"tr", "mktemp", "awk", "rm", "grep"} {
		p, err := exec.LookPath(c)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(p, filepath.Join(bin, c)))
	}
	var env []string
	for _, kv := range ambienteLimpo() {
		if !strings.HasPrefix(kv, "PATH=") {
			env = append(env, kv)
		}
	}
	return append(env, "PATH="+bin)
}

func codigoSaida(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	require.True(t, errors.As(err, &ee), "%v", err)
	return ee.ExitCode()
}

func TestSEC61_ConferirChave_CodigosDeRetorno(t *testing.T) {
	g := novoGpgSEC58(t)
	fprA := g.gerar("SEC61 A <a@sec61.invalid>")
	fprB := g.gerar("SEC61 B <b@sec61.invalid>")
	soA := g.exportar("a.pgp", fprA)

	casos := []struct {
		nome, fpr string
		env       []string
		rc        int
	}{
		{"certa", fprA, ambienteLimpo(), 0},
		{"divergente", fprB, ambienteLimpo(), 1},
		{"FPR inválido", "curto", ambienteLimpo(), 2},
		{"sem gpg", fprA, ambienteSemGpgSEC61(t), 2},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, err := rodarConferirSEC58(t, c.env, soA, c.fpr)
			assert.Equal(t, c.rc, codigoSaida(t, err), out)
		})
	}
}

func TestSEC61_PreflightKeyring(t *testing.T) {
	g := novoGpgSEC58(t)
	fprA := g.gerar("SEC61 C <c@sec61.invalid>")
	fprB := g.gerar("SEC61 D <d@sec61.invalid>")
	k := g.exportar("c.pgp", fprA)
	ausente := filepath.Join(t.TempDir(), "nao-existe.pgp")
	remedio := "sudo rm " + k + " /etc/apt/sources.list.d/mariadb.sources && make setup"

	casos := []struct {
		nome, modo, apt, k, fpr string
		env                     []string
		rc                      int
		seguiu                  bool
		msg                     string
	}{
		{"instalar: certo segue", "instalar", "1", k, fprA, ambienteLimpo(), 0, true, "confere com MARIADB_GPG_FPR"},
		{"instalar: ausente segue", "instalar", "1", ausente, fprA, ambienteLimpo(), 0, true, ""},
		{"instalar: divergente aborta", "instalar", "1", k, fprB, ambienteLimpo(), 1, false, "SEC-61: " + k + " não confere com MARIADB_GPG_FPR"},
		{"instalar: sem gpg aborta", "instalar", "1", k, fprA, ambienteSemGpgSEC61(t), 1, false, "não foi possível conferir " + k + ": gpg ausente"},
		{"instalar: FPR inválido aborta", "instalar", "1", k, "curto", ambienteLimpo(), 1, false, "MARIADB_GPG_FPR inválido"},
		{"instalar sem apt: só registra", "instalar", "0", k, fprB, ambienteLimpo(), 0, true, "REGISTRAR mariadb-key|FALTA|" + k + " não confere"},
		{"check: divergente = FALTA", "check", "1", k, fprB, ambienteLimpo(), 0, true, "REGISTRAR mariadb-key|FALTA|" + k + " não confere"},
		{"check: sem gpg = AVISO", "check", "1", k, fprA, ambienteSemGpgSEC61(t), 0, true, "REGISTRAR mariadb-key|AVISO|não foi possível conferir " + k + ": gpg ausente"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, err := rodarPreflightSEC61(t, c.env, c.modo, c.apt, c.k, c.fpr)
			assert.Equal(t, c.rc, codigoSaida(t, err), out)
			assert.Contains(t, out, c.msg)
			if c.seguiu {
				assert.Contains(t, out, marcaDepoisPreflightSEC61)
			} else {
				assert.NotContains(t, out, marcaDepoisPreflightSEC61, "aborta antes do apt")
				assert.Contains(t, out, remedio)
			}
		})
	}
}

// O preflight fica em main() depois de detectar_distro (precisa do APT) e
// antes de etapa_basicas (primeiro apt-get), nos dois modos.
func TestSEC61_PreflightAntesDoApt(t *testing.T) {
	principal := funcaoSetupSEC61(t, "main")
	iDistro := strings.Index(principal, "\tdetectar_distro\n")
	const chamada = "\tpreflight_keyring_mariadb /etc/apt/keyrings/mariadb-keyring.pgp\n"
	iPre := strings.Index(principal, chamada)
	iBasicas := strings.Index(principal, "\tetapa_basicas\n")
	require.GreaterOrEqual(t, iDistro, 0)
	require.GreaterOrEqual(t, iPre, 0)
	require.GreaterOrEqual(t, iBasicas, 0)
	assert.Less(t, iDistro, iPre)
	assert.Less(t, iPre, iBasicas)
	assert.Equal(t, 1, strings.Count(setupLinuxSEC58(t), chamada), "uma única chamada, sem condição de modo")
}
