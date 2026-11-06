package deploy_test

// CI-02 e SEC-62 (Lote 39): estágio "Testes Go" no GO_TEST_IMAGE (Debian, com
// bash e gpg, fixado por digest; sem instalar nada no estágio) e código dos
// estágios de teste vindo do git archive HEAD (só o que está rastreado).

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var reGoTestImage = regexp.MustCompile(`^golang:(\d+\.\d+)\.\d+-(trixie|bookworm)@sha256:[0-9a-f]{64}$`)

func imagemJenkins(t *testing.T, jf, nome string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^\s*` + nome + `\s*=\s*'([^']+)'`).FindStringSubmatch(jf)
	require.NotNil(t, m, "%s no environment do Jenkinsfile", nome)
	return m[1]
}

func TestCI02_GoTestImageFixadaNaVersaoDoGoMod(t *testing.T) {
	jf := lerRaiz(t, "Jenkinsfile")
	img := imagemJenkins(t, jf, "GO_TEST_IMAGE")
	m := reGoTestImage.FindStringSubmatch(img)
	require.NotNil(t, m, "golang:<x.y.z>-<trixie|bookworm>@sha256:<64 hex>: %q", img)

	for _, mod := range []string{"shared", "rotaperfumes-api"} {
		gm := regexp.MustCompile(`(?m)^go (\d+\.\d+)(\.\d+)?$`).FindStringSubmatch(lerRaiz(t, "backend", mod, "go.mod"))
		require.NotNil(t, gm, "diretiva go de %s", mod)
		assert.Equal(t, gm[1], m[1], "mesma linha do Go que backend/%s/go.mod", mod)
	}

	versao := func(ref string) string { return strings.SplitN(strings.SplitN(ref, ":", 2)[1], "-", 2)[0] }
	goImage := imagemJenkins(t, jf, "GO_IMAGE")
	assert.Equal(t, versao(goImage), versao(img), "mesma versão do GO_IMAGE")
	assert.Contains(t, goImage, "-alpine@", "GO_IMAGE continua alpine (deploy/seed-producao.sh)")

	refs := regexp.MustCompile(`[a-z0-9._/-]+:[A-Za-z0-9._-]+@sha256:[0-9a-f]{64}`).FindAllString(jf, -1)
	assert.Contains(t, refs, img, "visível para o deps-audit-imagens")
}

func TestCI02_EstagioTestesGoUsaGoTestImage(t *testing.T) {
	codigo := ci01CodigoTestesGo(t)
	run := regexp.MustCompile(`docker run [^\n]*`).FindString(codigo)
	require.NotEmpty(t, run)
	assert.Contains(t, run, `"$GO_TEST_IMAGE"`)
	assert.NotContains(t, codigo, `$GO_IMAGE`)
	assert.Equal(t, 1, strings.Count(lerRaiz(t, "Jenkinsfile"), `$GO_TEST_IMAGE`), "só o estágio \"Testes Go\" usa")

	for nome, re := range map[string]string{
		"apk":            `\bapk\b`,
		"apt":            `\bapt(-get)?\b`,
		"--privileged":   `--privileged`,
		"seccomp":        `seccomp`,
		"--cap-add":      `--cap-add`,
		"--security-opt": `--security-opt`,
	} {
		assert.NotRegexp(t, re, codigo, "sem %s no estágio", nome)
	}
}

const ci02VarFilho = "RP_CI02_FERRAMENTA_FILHO"

// TestCI02_FilhoFerramentaAusente só faz algo no subprocesso: o caminho do
// TestSEC58_ConferirChaveMariaDB até o gpg, com o PATH dado pelo caso.
func TestCI02_FilhoFerramentaAusente(t *testing.T) {
	if os.Getenv(ci02VarFilho) != "1" {
		t.Skip("executado só como subprocesso de TestCI02_FerramentaAusenteFalhaNoCI")
	}
	novoGpgSEC58(t)
	t.Log("CI02-NAO-PAROU")
}

// No CI, bash ou gpg ausente falha em vez de pular (subprocesso porque t.Fatalf
// e t.Skip não são observáveis de dentro do próprio teste).
func TestCI02_FerramentaAusenteFalhaNoCI(t *testing.T) {
	if os.Getenv(ci02VarFilho) == "1" {
		t.Skip("no subprocesso")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash não está no PATH (fora do CI)")
	}
	soBash := t.TempDir()
	require.NoError(t, os.Symlink(bash, soBash+"/bash"))

	casos := []struct {
		nome, path, ausente, ci string
		falha                   bool
	}{
		{"sem bash no CI", t.TempDir(), "bash", "true", true},
		{"sem gpg no CI", soBash, "gpg", "true", true},
		{"sem bash fora do CI", t.TempDir(), "bash", "", false},
		{"sem gpg fora do CI", soBash, "gpg", "", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestCI02_FilhoFerramentaAusente$", "-test.v", "-test.count=1")
			t.Setenv("CI", c.ci)
			t.Setenv(ci02VarFilho, "1")
			t.Setenv("PATH", c.path)
			out, err := cmd.CombinedOutput()
			saida := string(out)
			assert.NotContains(t, saida, "CI02-NAO-PAROU")
			if c.falha {
				require.Error(t, err, saida)
				assert.Contains(t, saida, c.ausente+" ausente no CI")
				return
			}
			require.NoError(t, err, saida)
			assert.Contains(t, saida, c.ausente+" não está no PATH")
		})
	}
}

// sec62Codigo devolve o corpo do estágio sem as linhas de comentário.
func sec62Codigo(t *testing.T, nome string) string {
	t.Helper()
	var ls []string
	for _, l := range strings.Split(estagioJenkins(t, lerRaiz(t, "Jenkinsfile"), nome), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "//") {
			ls = append(ls, l)
		}
	}
	return strings.Join(ls, "\n")
}

// sec62Caminhos devolve a lista de caminhos do git archive do estágio.
func sec62Caminhos(t *testing.T, codigo string) []string {
	t.Helper()
	m := regexp.MustCompile(`(?s)git archive --format=tar HEAD -- (.*?)\|`).FindStringSubmatch(codigo)
	require.NotNil(t, m, "git archive --format=tar HEAD -- <lista>")
	caminhos := strings.Fields(strings.ReplaceAll(m[1], "\\", " "))
	require.NotEmpty(t, caminhos)
	return caminhos
}

func TestSEC62_EstagiosUsamGitArchive(t *testing.T) {
	for _, nome := range []string{"Testes Go", "Testes Frontend"} {
		t.Run(nome, func(t *testing.T) {
			codigo := sec62Codigo(t, nome)
			iGuarda := strings.Index(codigo, `command -v git >/dev/null || { echo "git ausente no agente (SEC-62)" >&2; exit 1; }`)
			iArchive := strings.Index(codigo, "git archive --format=tar HEAD -- ")
			require.GreaterOrEqual(t, iGuarda, 0, "guarda do git")
			require.GreaterOrEqual(t, iArchive, 0, "git archive")
			assert.Less(t, iGuarda, iArchive)
			assert.NotContains(t, codigo, "tar -cf -", "sem tar sobre o workspace")
			assert.NotContains(t, codigo, "--exclude", "sem lista de exclusão")
			sec62Caminhos(t, codigo)
		})
	}
	assert.Regexp(t, `(?m)^//\s+- .*\bgit\b`, lerRaiz(t, "Jenkinsfile"), "git nos pré-requisitos do agente")
}

// O .gitattributes não vai no tar; com o .git ausente, não há o que conferir.
func TestSEC62_GitattributesSemExportIgnore(t *testing.T) {
	if _, err := os.Stat(caminhoRaiz(t, ".git")); err != nil {
		t.Skip(".git ausente (tar do git archive)")
	}
	assert.NotContains(t, lerRaiz(t, ".gitattributes"), "export-ignore", "tiraria arquivos do git archive em silêncio")
}

// Todo caminho das listas está rastreado (o git archive falha com pathspec
// não rastreado). Sem .git (o próprio tar do CI) não há o que consultar: lá
// o git archive do estágio já conferiu.
func TestSEC62_CaminhosRastreados(t *testing.T) {
	if _, err := os.Stat(caminhoRaiz(t, ".git")); err != nil {
		t.Skip(".git ausente (tar do git archive)")
	}
	ferramentaObrigatoria(t, "git")
	for _, nome := range []string{"Testes Go", "Testes Frontend"} {
		for _, c := range sec62Caminhos(t, sec62Codigo(t, nome)) {
			t.Run(nome+"/"+c, func(t *testing.T) {
				cmd := exec.Command("git", "ls-files", "--error-unmatch", "--", c)
				cmd.Dir = raiz(t)
				out, err := cmd.CombinedOutput()
				require.NoError(t, err, "%s", out)
			})
		}
	}
}
