package deploy_test

// Helpers comuns dos testes de deploy/ (TST-15, Lote 30).
//
// Os testes leem deploy/* e o Jenkinsfile da raiz do projeto. O estágio
// "Testes Go" do Jenkinsfile precisa copiar deploy/ e o Jenkinsfile para o
// container (tar), senão estes testes falham lá (de propósito: um skip
// silencioso esconderia a regressão dos digests/contas).

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/stretchr/testify/require"
)

func raiz(t *testing.T) string {
	t.Helper()
	r, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)
	return r
}

func caminhoRaiz(t *testing.T, partes ...string) string {
	t.Helper()
	return filepath.Join(append([]string{raiz(t)}, partes...)...)
}

func lerRaiz(t *testing.T, partes ...string) string {
	t.Helper()
	b, err := os.ReadFile(caminhoRaiz(t, partes...))
	require.NoError(t, err)
	return string(b)
}

// emCI: o teste roda no CI (o estágio "Testes Go" do Jenkinsfile passa
// -e CI=true ao container; CI-01). Só os.Getenv("CI") é lido.
func emCI() bool {
	v := os.Getenv("CI")
	return v == "true" || v == "1"
}

// Ações de acaoArquivoAusente.
const (
	acaoFalhar = "falhar"
	acaoPular  = "pular"
)

// acaoArquivoAusente decide o que fazer quando um arquivo do repositório
// lido pelo teste não existe: no CI falha (o tar do Jenkinsfile deixou de
// copiá-lo — regressão); fora dele pula (checkout parcial).
func acaoArquivoAusente(ci bool) string {
	if ci {
		return acaoFalhar
	}
	return acaoPular
}

// arquivoObrigatorio lê um arquivo da raiz do projeto. Ausente: t.Fatalf no
// CI, t.Skip fora (acaoArquivoAusente). Outro erro de leitura sempre falha.
func arquivoObrigatorio(t *testing.T, partes ...string) string {
	t.Helper()
	p := caminhoRaiz(t, partes...)
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		rel := filepath.ToSlash(filepath.Join(partes...))
		if acaoArquivoAusente(emCI()) == acaoFalhar {
			t.Fatalf("%s ausente no CI: o estágio \"Testes Go\" do Jenkinsfile precisa copiá-lo (CI-01)", rel)
		}
		t.Skipf("%s ausente (checkout parcial)", rel)
	}
	require.NoError(t, err)
	return string(b)
}

func precisaBash(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("scripts bash de servidor Linux")
	}
	ferramentaObrigatoria(t, "bash")
}

// ferramentaObrigatoria exige um executável que a imagem do CI traz (CI-02:
// GO_TEST_IMAGE). Ausente: t.Fatalf no CI, para o teste não pular calado;
// t.Skip fora (mesma decisão de arquivoObrigatorio).
func ferramentaObrigatoria(t *testing.T, nome string) {
	t.Helper()
	if _, err := exec.LookPath(nome); err == nil {
		return
	}
	if acaoArquivoAusente(emCI()) == acaoFalhar {
		t.Fatalf("%s ausente no CI: o GO_TEST_IMAGE do Jenkinsfile precisa trazê-lo (CI-02)", nome)
	}
	t.Skipf("%s não está no PATH", nome)
}

func precisaOpenSSL(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl não está no PATH")
	}
}

// ambienteLimpo é o os.Environ() sem o que mudaria o comportamento dos stubs
// ou do bash: FAKE* herdados do shell e BASH_ENV/ENV (arquivo carregado pelo
// bash não interativo). DEPLOY-07: também sem MYSQL_PWD, DB_* e
// PASSWORD_PEPPER (o Makefile exporta o .env local, que aponta para um banco
// real): nenhum segredo de verdade chega aos scripts e stubs.
func ambienteLimpo() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "FAKE") || strings.HasPrefix(kv, "BASH_ENV=") || strings.HasPrefix(kv, "ENV=") ||
			strings.HasPrefix(kv, "MYSQL_PWD=") || strings.HasPrefix(kv, "DB_") || strings.HasPrefix(kv, "PASSWORD_PEPPER=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func escrever(t *testing.T, caminho, conteudo string, modo os.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(caminho), 0o755))
	require.NoError(t, os.WriteFile(caminho, []byte(conteudo), modo))
}

func linhas(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
