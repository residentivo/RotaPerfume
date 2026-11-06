package deploy_test

// CI-01 (Lote 38): o estágio "Testes Go" do Jenkinsfile copia scripts/ no
// tar e roda o container com -e CI=true; com CI, arquivo do repositório
// ausente faz o teste falhar (arquivoObrigatorio) em vez de pular. Bash e gpg
// ausentes falham do mesmo jeito (CI-02). SEC-62: o tar é o git archive HEAD.

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ci01CodigoTestesGo é o corpo do estágio "Testes Go" sem as linhas de
// comentário (//), que citam scripts/ e CI=true no texto.
func ci01CodigoTestesGo(t *testing.T) string {
	t.Helper()
	corpo := estagioJenkins(t, lerRaiz(t, "Jenkinsfile"), "Testes Go")
	var linhas []string
	for _, l := range strings.Split(corpo, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "//") {
			continue
		}
		linhas = append(linhas, l)
	}
	return strings.Join(linhas, "\n")
}

// ci01ComandoTar devolve o comando que gera o tar (com as continuações "\")
// até o pipe.
func ci01ComandoTar(t *testing.T, codigo string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)git archive --format=tar HEAD -- .*?\|`).FindString(codigo)
	require.NotEmpty(t, m, "git archive do estágio \"Testes Go\"")
	return m
}

func TestCI01_TarDoTestesGoCopiaScripts(t *testing.T) {
	tar := ci01ComandoTar(t, ci01CodigoTestesGo(t))
	assert.Regexp(t, `\sscripts(\s|$)`, tar, "scripts/ no tar do \"Testes Go\"")
}

func TestCI01_DockerRunDoTestesGoPassaCI(t *testing.T) {
	codigo := ci01CodigoTestesGo(t)
	run := regexp.MustCompile(`docker run [^\n]*`).FindString(codigo)
	require.NotEmpty(t, run, "docker run do estágio \"Testes Go\"")
	assert.Regexp(t, `\s-e CI=true(\s|$)`, run)
}

func TestCI01_AcaoArquivoAusente(t *testing.T) {
	casos := []struct {
		ci   string
		acao string
	}{
		{"true", acaoFalhar},
		{"1", acaoFalhar},
		{"", acaoPular},
		{"false", acaoPular},
	}
	for _, c := range casos {
		t.Run("CI="+c.ci, func(t *testing.T) {
			t.Setenv("CI", c.ci)
			assert.Equal(t, c.acao, acaoArquivoAusente(emCI()))
		})
	}
}

// Os arquivos de scripts/ lidos pelos testes de deploy existem no
// repositório (no CI, a ausência falha em arquivoObrigatorio).
func TestCI01_ScriptsLidosExistem(t *testing.T) {
	for _, a := range [][]string{{"scripts", "setup-linux.sh"}, {"scripts", "versoes.conf"}} {
		t.Run(strings.Join(a, "/"), func(t *testing.T) {
			assert.NotEmpty(t, arquivoObrigatorio(t, a...))
		})
	}
}
