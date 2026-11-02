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

func precisaBash(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("scripts bash de servidor Linux")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash não está no PATH")
	}
}

func precisaOpenSSL(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl não está no PATH")
	}
}

// ambienteLimpo é o os.Environ() sem o que mudaria o comportamento dos stubs
// ou do bash: FAKE* herdados do shell e BASH_ENV/ENV (arquivo carregado pelo
// bash não interativo).
func ambienteLimpo() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "FAKE") || strings.HasPrefix(kv, "BASH_ENV=") || strings.HasPrefix(kv, "ENV=") {
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
