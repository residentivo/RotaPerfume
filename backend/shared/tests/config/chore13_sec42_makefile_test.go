package config_test

// CHORE-13 / SEC-42 / SEC-31 no Makefile:
//   - sem o ramo DB_CLIENTE=mysql (nem --ssl-mode/VERIFY_IDENTITY); TLS só com
//     --ssl-ca + --ssl-verify-server-cert;
//   - db-check-env aplica o SEC-42 (DB_HOST não-loopback sem DB_TLS_CA falha,
//     salvo DB_TLS_REQUIRED=false; valor inválido falha) e confere que o
//     DB_CLIENTE_BIN é o cliente MariaDB. Comportamento testado com uma CÓPIA
//     do Makefile num diretório temporário (sem .env) e um cliente falso que
//     só responde a --version: nada conecta em banco;
//   - deps-audit-imagens existe, é informativo e entra no deps-audit.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCHORE13_Makefile_SoClienteMariaDB(t *testing.T) {
	mk := lerMakefile(t)
	assert.NotContains(t, mk.texto, "ssl-mode", "o cliente MariaDB não tem --ssl-mode")
	assert.NotContains(t, mk.texto, "VERIFY_IDENTITY")
	assert.NotRegexp(t, `(?m)^ifeq \(\$\(DB_CLIENTE\)`, mk.texto, "sem o ramo DB_CLIENTE=mysql")
	assert.NotRegexp(t, `(?m)^DB_CLIENTE\s*[:?]?=`, mk.texto, "DB_CLIENTE não é mais definido")
	assert.NotRegexp(t, `(?i)mysql 8|cliente oficial do mysql`, mk.texto)

	tlsOpts := mk.linhaDaVariavel(t, "MYSQL_TLS_OPTS")
	assert.Contains(t, tlsOpts, `--ssl-ca="$(DB_TLS_CA)" --ssl-verify-server-cert`)
	assert.Contains(t, mk.linhaDaVariavel(t, "MYSQL_OPTS"), "$(MYSQL_TLS_OPTS)")

	receita := strings.Join(mk.regras["db-check-env"].receita, "\n")
	assert.Contains(t, receita, "*MariaDB*", "db-check-env confere que o cliente é MariaDB")
	assert.Contains(t, receita, "DB_TLS_REQUIRED")
	assert.Contains(t, receita, "SEC-42")
}

func TestSEC31_Makefile_DepsAuditImagens(t *testing.T) {
	mk := lerMakefile(t)
	require.Contains(t, mk.regras, "deps-audit-imagens")
	assert.True(t, mk.phony["deps-audit-imagens"])
	assert.Contains(t, mk.regras["deps-audit"].prereqs, "deps-audit-imagens")
	assert.False(t, mk.dependeDe("deps-audit-imagens", "db-check-env"), "não exige credenciais")
	receita := strings.Join(mk.regras["deps-audit-imagens"].receita, "\n")
	for _, s := range []string{"$(IMAGENS_FIXADAS_EM)", "imagetools inspect", "registry-1.docker.io", "auth.docker.io", "exit 0"} {
		assert.Contains(t, receita, s)
	}
	assert.Contains(t, mk.texto, "IMAGENS_FIXADAS_EM = Jenkinsfile deploy/mariadb-docker.sh")
	assert.Contains(t, strings.ToLower(receita), "docker-content-digest")
	assert.NotContains(t, receita, "exit 1", "informativo: nunca falha")
}

// sec42Make roda "make db-check-env" numa cópia do Makefile, sem .env, com
// um cliente falso. Variáveis na linha de comando (ganham do ambiente).
func sec42Make(t *testing.T, versaoCliente string, vars map[string]string) (string, error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("usa sh e um cliente falso em shell")
	}
	makeBin, err := exec.LookPath("make")
	if err != nil {
		t.Skip("make não encontrado")
	}
	dir := t.TempDir()
	dados, err := os.ReadFile(filepath.Join(raizProjeto(t), "Makefile"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Makefile"), dados, 0o644))
	cliente := filepath.Join(dir, "cliente-falso")
	script := "#!/bin/sh\n[ \"$1\" = --version ] || { echo 'cliente falso: so --version' >&2; exit 99; }\necho '" + versaoCliente + "'\n"
	require.NoError(t, os.WriteFile(cliente, []byte(script), 0o755))
	ca := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(ca, []byte("publica"), 0o644))

	args := []string{"-s", "db-check-env",
		"DB_USUARIO=u", "DB_SENHA=segredo-sec42", "DB_CLIENTE_BIN=" + cliente, "DB_CLIENTE=",
		"DB_HOST=localhost", "DB_TLS_CA=", "DB_TLS_REQUIRED="}
	for k, v := range vars {
		if v == "<ca>" {
			v = ca
		}
		args = append(args, k+"="+v)
	}
	cmd := exec.Command(makeBin, args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "LC_ALL=C"}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

const sec42VersaoMariaDB = "mariadb from 11.4.13-MariaDB, client 15.2 for Linux (x86_64)"

func TestSEC42_Makefile_DbCheckEnv(t *testing.T) {
	casos := []struct {
		nome   string
		vars   map[string]string
		ok     bool
		trecho string
	}{
		{"localhost sem CA", map[string]string{"DB_HOST": "localhost"}, true, ""},
		{"127.0.0.1 sem CA", map[string]string{"DB_HOST": "127.0.0.1"}, true, ""},
		{"127.9.8.7 sem CA", map[string]string{"DB_HOST": "127.9.8.7"}, true, ""},
		{"::1 sem CA", map[string]string{"DB_HOST": "::1"}, true, ""},
		{"IP remoto sem CA", map[string]string{"DB_HOST": "192.168.168.106"}, false, "SEC-42: DB_HOST=192.168.168.106 nao e loopback"},
		{"container sem CA", map[string]string{"DB_HOST": "rotaperfumes-mariadb"}, false, "SEC-42"},
		{"127.0.0.1.nip.io sem CA", map[string]string{"DB_HOST": "127.0.0.1.nip.io"}, false, "SEC-42"},
		{"remoto sem CA, true explícito", map[string]string{"DB_HOST": "10.0.0.1", "DB_TLS_REQUIRED": "true"}, false, "SEC-42"},
		{"remoto com CA", map[string]string{"DB_HOST": "10.0.0.1", "DB_TLS_CA": "<ca>"}, true, ""},
		{"remoto sem CA, false explícito", map[string]string{"DB_HOST": "10.0.0.1", "DB_TLS_REQUIRED": "false"}, true, "AVISO (SEC-42)"},
		{"valor inválido", map[string]string{"DB_TLS_REQUIRED": "sim"}, false, "DB_TLS_REQUIRED=sim invalido"},
		{"CA inexistente", map[string]string{"DB_HOST": "10.0.0.1", "DB_TLS_CA": "/nao/existe/ca.pem"}, false, "nao encontrado"},
		{"DB_CLIENTE=mysql removido", map[string]string{"DB_CLIENTE": "mysql"}, false, "CHORE-13"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, err := sec42Make(t, sec42VersaoMariaDB, c.vars)
			if c.ok {
				assert.NoError(t, err, out)
			} else {
				assert.Error(t, err, out)
			}
			if c.trecho != "" {
				assert.Contains(t, out, c.trecho)
			}
			assert.NotContains(t, out, "segredo-sec42", "a senha nunca aparece")
			assert.NotContains(t, out, "cliente falso: so --version", "db-check-env só chama --version")
		})
	}
}

func TestCHORE13_Makefile_DbCheckEnvRecusaClienteMySQL(t *testing.T) {
	out, err := sec42Make(t, "mysql  Ver 8.0.40 for Linux on x86_64 (MySQL Community Server - GPL)", nil)
	require.Error(t, err, out)
	assert.Contains(t, out, "nao e o cliente MariaDB")

	out, err = sec42Make(t, sec42VersaoMariaDB, nil)
	assert.NoError(t, err, out)
}
