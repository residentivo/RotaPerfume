package config_test

// SEC-21 (Lote 24, TestBrain): COOKIE_SECURE no Makefile e no deploy.
//   - Makefile: só dev-api e run-api definem `export COOKIE_SECURE ?= false`
//     (variável por alvo); nenhuma atribuição global. Comportamento conferido
//     com o make de verdade: um pré-requisito sonda (receita com "+", que roda
//     mesmo com -n) lê o ambiente que o alvo passa aos filhos; o valor do
//     shell continua ganhando.
//   - deploy/setup-servidor.sh: o api.env de produção sempre sai com
//     COOKIE_SECURE=true, mesmo que o .env de origem traga false (com
//     "export", CRLF ou espaços). O script é carregado com "source" (o main só
//     roda quando executado diretamente).

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

var reCookieSecureMake = regexp.MustCompile(`\bCOOKIE_SECURE\s*(\?|:|::|\+)?=`)

func TestSEC21_Makefile_SoDevApiERunApiDesligamSecure(t *testing.T) {
	mk := lerMakefile(t)
	var atribuicoes []string
	for _, l := range mk.linhas {
		if strings.HasPrefix(strings.TrimSpace(l), "#") || strings.HasPrefix(l, "\t") {
			continue
		}
		if i := strings.Index(l, "#"); i >= 0 {
			l = l[:i] // texto de help ("## ...") e comentários
		}
		if reCookieSecureMake.MatchString(l) {
			atribuicoes = append(atribuicoes, strings.TrimSpace(l))
		}
	}
	require.Len(t, atribuicoes, 1, "uma única atribuição de COOKIE_SECURE no Makefile: %v", atribuicoes)

	alvos, decl, ok := strings.Cut(atribuicoes[0], ":")
	require.True(t, ok, "a atribuição tem de ser por alvo: %q", atribuicoes[0])
	assert.ElementsMatch(t, []string{"dev-api", "run-api"}, strings.Fields(alvos))
	assert.Regexp(t, `^\s*export\s+COOKIE_SECURE\s*\?=\s*false\s*$`, decl,
		"export com ?= (o .env/shell ganham) e valor false")
}

// sondarCookieSecure roda `make -n <alvo>` com um pré-requisito extra que
// imprime o COOKIE_SECURE do ambiente da receita.
func sondarCookieSecure(t *testing.T, alvo string, envExtra ...string) string {
	t.Helper()
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make não está no PATH")
	}
	cmd := exec.Command("make", "-n", "--no-print-directory",
		"--eval", alvo+": sec21-sonda",
		"--eval", `sec21-sonda: ; +@echo "SEC21SONDA=[$$COOKIE_SECURE]"`,
		alvo)
	cmd.Dir = raizProjeto(t)
	env := []string{}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "COOKIE_SECURE=") && !strings.HasPrefix(kv, "MAKEFLAGS=") && !strings.HasPrefix(kv, "MAKELEVEL=") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, envExtra...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "make -n %s: %s", alvo, out)
	m := regexp.MustCompile(`(?m)^SEC21SONDA=\[(.*)\]$`).FindStringSubmatch(string(out))
	require.NotNil(t, m, "sonda não rodou para %s: %s", alvo, out)
	return m[1]
}

func TestSEC21_Makefile_ComportamentoDoMake(t *testing.T) {
	raiz := raizProjeto(t)
	if b, err := os.ReadFile(filepath.Join(raiz, ".env")); err == nil &&
		regexp.MustCompile(`(?m)^\s*(export\s+)?COOKIE_SECURE\s*=`).Match(b) {
		t.Skip(".env local define COOKIE_SECURE: o valor do .env ganha (comportamento esperado), sonda inconclusiva")
	}
	casos := []struct {
		alvo     string
		envExtra []string
		want     string
	}{
		{"dev-api", nil, "false"},
		{"run-api", nil, "false"},
		{"dev-api", []string{"COOKIE_SECURE=true"}, "true"},
		{"run-api", []string{"COOKIE_SECURE=true"}, "true"},
		{"build-api", nil, ""},
		{"build-shared", nil, ""},
		{"dev-frontend", nil, ""},
	}
	for _, c := range casos {
		nome := c.alvo
		if len(c.envExtra) > 0 {
			nome += "/" + strings.Join(c.envExtra, ",")
		}
		t.Run(nome, func(t *testing.T) {
			assert.Equal(t, c.want, sondarCookieSecure(t, c.alvo, c.envExtra...))
		})
	}
}

func TestSEC21_SetupServidor_CookieSecureSempreTrue(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash não está no PATH")
	}
	script := filepath.Join(raizProjeto(t), "deploy", "setup-servidor.sh")
	texto := lerTexto(t, script)
	linhaChaves := regexp.MustCompile(`(?m)^readonly CHAVES_SOBRESCRITAS=\((.*)\)$`).FindStringSubmatch(texto)
	require.NotNil(t, linhaChaves)
	assert.Contains(t, strings.Fields(linhaChaves[1]), "COOKIE_SECURE", "COOKIE_SECURE do .env é descartado")

	casos := []struct {
		nome   string
		envSrc string
	}{
		{"sem COOKIE_SECURE no .env", "SMTP_HOST=smtp.exemplo\nJWT_SECRET=velho\n"},
		{"COOKIE_SECURE=false", "SMTP_HOST=smtp.exemplo\nCOOKIE_SECURE=false\n"},
		{"export COOKIE_SECURE=false", "export COOKIE_SECURE=false\nSMTP_HOST=smtp.exemplo\n"},
		{"COOKIE_SECURE=false com CRLF", "COOKIE_SECURE=false\r\nSMTP_HOST=smtp.exemplo\r\n"},
		{"COOKIE_SECURE com espaços e aspas", "  COOKIE_SECURE = 'false'\nSMTP_HOST=smtp.exemplo\n"},
		{"COOKIE_SECURE=false na última linha sem quebra", "SMTP_HOST=smtp.exemplo\nCOOKIE_SECURE=false"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "origem.env")
			require.NoError(t, os.WriteFile(src, []byte(c.envSrc), 0o600))

			// Mesma composição de etapa_api_env: filtrar_env + linhas_api_env.
			cmd := exec.Command("bash", "-c",
				`source "$1" && { filtrar_env <"$2"; linhas_api_env 3306 senha-app jwt-novo site.local 192.168.0.10 8443; }`,
				"sec21", script, src)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", out)

			var valores []string
			for _, l := range strings.Split(string(out), "\n") {
				if m := regexp.MustCompile(`^\s*(export\s+)?COOKIE_SECURE\s*=(.*)$`).FindStringSubmatch(l); m != nil {
					valores = append(valores, m[2])
				}
			}
			assert.Equal(t, []string{"true"}, valores, "api.env gerado:\n%s", out)
			assert.Contains(t, string(out), "SMTP_HOST=smtp.exemplo", "as demais chaves são mantidas")
			assert.Contains(t, string(out), "TRUST_PROXY_HEADERS=true")
		})
	}
}
