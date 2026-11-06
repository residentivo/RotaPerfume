package deploy_test

// SEC-58 (Lote 37, BackBrain): scripts/setup-linux.sh só instala a chave do
// repositório oficial do MariaDB (/etc/apt/keyrings/mariadb-keyring.pgp) se
// o arquivo baixado tiver exatamente uma chave primária, válida, com o
// fingerprint MARIADB_GPG_FPR do scripts/versoes.conf. A função
// conferir_chave_mariadb é extraída do script (autocontida) e roda com
// chaves descartáveis geradas num GNUPGHOME temporário; nada é baixado nem
// instalado. O estágio "Testes Go" do Jenkinsfile copia scripts/ e passa
// CI=true (CI-01): lá arquivo, bash ou gpg ausente falha (CI-02: o
// GO_TEST_IMAGE traz os dois); fora do CI, pula.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fprMariaDBSEC58 = "177F4010FE56CA3336300305F1656F24C74CD1D8"

func setupLinuxSEC58(t *testing.T) string {
	t.Helper()
	return arquivoObrigatorio(t, "scripts", "setup-linux.sh")
}

func TestSEC58_VersoesConfFingerprint(t *testing.T) {
	setupLinuxSEC58(t)
	vc := lerRaiz(t, "scripts", "versoes.conf")
	assert.Regexp(t, `(?m)^MARIADB_GPG_FPR=`+fprMariaDBSEC58+`$`, vc)
}

// Ressalva do SecBrain (Lote 37): validade r (revogada), e (expirada) e
// i (inválida) da chave primária recusam.
func TestSEC58_ValidadeRecusada(t *testing.T) {
	sl := setupLinuxSEC58(t)
	fn := regexp.MustCompile(`(?ms)^conferir_chave_mariadb\(\) \{.*?^\}$`).FindString(sl)
	require.NotEmpty(t, fn)
	assert.Contains(t, fn, `$1=="pub"{p=1; if ($2 ~ /[rei]/) bad=1; next}`)
}

func TestSEC58_OrdemBaixarConferirInstalar(t *testing.T) {
	sl := setupLinuxSEC58(t)
	corpo := regexp.MustCompile(`(?ms)^configurar_repo_mariadb\(\) \{.*?^\}$`).FindString(sl)
	require.NotEmpty(t, corpo)
	iBaixar := strings.Index(corpo, `baixar "https://mariadb.org/mariadb_release_signing_key.pgp" "$tmp"`)
	iConferir := strings.Index(corpo, `conferir_chave_mariadb "$tmp" "${MARIADB_GPG_FPR:-}" || { rm -f "$tmp"; return 1; }`)
	iInstalar := strings.Index(corpo, `install -m 0644 "$tmp" /etc/apt/keyrings/mariadb-keyring.pgp`)
	require.GreaterOrEqual(t, iBaixar, 0)
	require.GreaterOrEqual(t, iConferir, 0)
	require.GreaterOrEqual(t, iInstalar, 0)
	assert.Less(t, iBaixar, iConferir)
	assert.Less(t, iConferir, iInstalar, "confere antes de instalar")
	assert.Equal(t, 1, strings.Count(sl, `"$tmp" /etc/apt/keyrings/mariadb-keyring.pgp`), "um único ponto de instalação da chave")
}

// gpgSEC58 gera chaves descartáveis num GNUPGHOME próprio.
type gpgSEC58 struct {
	t    *testing.T
	home string
}

func novoGpgSEC58(t *testing.T) *gpgSEC58 {
	t.Helper()
	precisaBash(t)
	ferramentaObrigatoria(t, "gpg")
	h, err := os.MkdirTemp("", "sec58-gnupg-")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = exec.Command("gpgconf", "--homedir", h, "--kill", "all").Run()
		_ = os.RemoveAll(h)
	})
	return &gpgSEC58{t: t, home: h}
}

func (g *gpgSEC58) gpg(args ...string) string {
	g.t.Helper()
	cmd := exec.Command("gpg", append([]string{"--homedir", g.home, "--batch", "--no-tty", "--quiet"}, args...)...)
	cmd.Env = append(ambienteLimpo(), "LC_ALL=C")
	out, err := cmd.Output()
	require.NoError(g.t, err, "gpg %v", args)
	return string(out)
}

// gerar cria uma chave (só certificação, ed25519) e devolve o fingerprint.
func (g *gpgSEC58) gerar(uid string) string {
	g.t.Helper()
	g.gpg("--pinentry-mode", "loopback", "--passphrase", "", "--quick-gen-key", uid, "ed25519", "cert", "never")
	for _, l := range strings.Split(g.gpg("--with-colons", "--list-keys", uid), "\n") {
		if strings.HasPrefix(l, "fpr:") {
			return strings.Split(l, ":")[9]
		}
	}
	g.t.Fatalf("sem fingerprint para %s", uid)
	return ""
}

func (g *gpgSEC58) exportar(arq string, fprs ...string) string {
	g.t.Helper()
	p := filepath.Join(filepath.Dir(g.home), filepath.Base(g.home)+"-"+arq)
	g.t.Cleanup(func() { _ = os.Remove(p) })
	require.NoError(g.t, os.WriteFile(p, []byte(g.gpg(append([]string{"--export"}, fprs...)...)), 0o600))
	return p
}

// revogar importa o certificado de revogação que o gpg gera com a chave.
func (g *gpgSEC58) revogar(fpr string) {
	g.t.Helper()
	b, err := os.ReadFile(filepath.Join(g.home, "openpgp-revocs.d", fpr+".rev"))
	require.NoError(g.t, err)
	rev := strings.Replace(string(b), ":-----BEGIN PGP PUBLIC KEY BLOCK-----", "-----BEGIN PGP PUBLIC KEY BLOCK-----", 1)
	p := filepath.Join(g.home, "rev.asc")
	require.NoError(g.t, os.WriteFile(p, []byte(rev), 0o600))
	g.gpg("--import", p)
}

// rodarConferirSEC58 roda a função extraída do setup-linux.sh.
func rodarConferirSEC58(t *testing.T, env []string, arq, fpr string) (string, error) {
	t.Helper()
	fn := regexp.MustCompile(`(?ms)^conferir_chave_mariadb\(\) \{.*?^\}$`).FindString(setupLinuxSEC58(t))
	require.NotEmpty(t, fn)
	sc := "aviso() { printf 'AVISO: %s\\n' \"$*\"; }\ntem() { command -v \"$1\" >/dev/null 2>&1; }\n" +
		fn + "\nconferir_chave_mariadb \"$1\" \"$2\"\n"
	cmd := exec.Command("bash", "-c", sc, "sec58", arq, fpr)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestSEC58_ConferirChaveMariaDB(t *testing.T) {
	g := novoGpgSEC58(t)
	fprA := g.gerar("SEC58 A <a@sec58.invalid>")
	fprB := g.gerar("SEC58 B <b@sec58.invalid>")
	fprR := g.gerar("SEC58 R <r@sec58.invalid>")
	g.revogar(fprR)
	soA := g.exportar("a.pgp", fprA)
	duas := g.exportar("ab.pgp", fprA, fprB)
	revogada := g.exportar("r.pgp", fprR)
	vazio := filepath.Join(t.TempDir(), "vazio.pgp")
	escrever(t, vazio, "", 0o600)
	lixo := filepath.Join(t.TempDir(), "lixo.pgp")
	escrever(t, lixo, "isto nao e uma chave\n", 0o600)

	// Fingerprint com espaços e minúsculas, como na documentação.
	var espacado []string
	for i := 0; i < len(fprA); i += 4 {
		espacado = append(espacado, strings.ToLower(fprA[i:i+4]))
	}

	env := ambienteLimpo()
	casos := []struct {
		nome, arq, fpr string
		ok             bool
		msg            string
	}{
		{"fingerprint certo", soA, fprA, true, ""},
		{"fingerprint com espaços e minúsculas", soA, strings.Join(espacado, " "), true, ""},
		{"fingerprint diferente", soA, fprB, false, "fingerprint inesperado (" + fprA + "); esperado " + fprB},
		{"duas chaves primárias (uma certa)", duas, fprA, false, "fingerprint inesperado"},
		{"chave revogada", revogada, fprR, false, "REVOGADA_OU_EXPIRADA"},
		{"arquivo vazio", vazio, fprA, false, "fingerprint inesperado ()"},
		{"arquivo lixo", lixo, fprA, false, "fingerprint inesperado ()"},
		{"MARIADB_GPG_FPR vazio", soA, "", false, "MARIADB_GPG_FPR ausente/invalido"},
		{"MARIADB_GPG_FPR curto", soA, fprA[:39], false, "MARIADB_GPG_FPR ausente/invalido"},
		{"MARIADB_GPG_FPR não hex", soA, "Z" + fprA[1:], false, "MARIADB_GPG_FPR ausente/invalido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, err := rodarConferirSEC58(t, env, c.arq, c.fpr)
			if c.ok {
				require.NoError(t, err, out)
				assert.Empty(t, strings.TrimSpace(out))
				return
			}
			require.Error(t, err, out)
			assert.Contains(t, out, c.msg)
		})
	}

	t.Run("PATH sem gpg", func(t *testing.T) {
		bin := t.TempDir()
		for _, c := range []string{"tr", "mktemp", "awk", "rm", "grep"} {
			p, err := exec.LookPath(c)
			require.NoError(t, err)
			require.NoError(t, os.Symlink(p, filepath.Join(bin, c)))
		}
		var e []string
		for _, kv := range env {
			if !strings.HasPrefix(kv, "PATH=") {
				e = append(e, kv)
			}
		}
		out, err := rodarConferirSEC58(t, append(e, "PATH="+bin), soA, fprA)
		require.Error(t, err, out)
		assert.Contains(t, out, "gpg ausente")
	})

	t.Run("não cria ~/.gnupg", func(t *testing.T) {
		home := t.TempDir()
		var e []string
		for _, kv := range env {
			if !strings.HasPrefix(kv, "HOME=") && !strings.HasPrefix(kv, "GNUPGHOME=") {
				e = append(e, kv)
			}
		}
		out, err := rodarConferirSEC58(t, append(e, "HOME="+home), soA, fprA)
		require.NoError(t, err, out)
		assert.NoDirExists(t, filepath.Join(home, ".gnupg"))
	})
}
