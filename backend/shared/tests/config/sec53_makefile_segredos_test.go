package config_test

// SEC-53 (Lote 34): o "export" global do Makefile continua (db-* e CLIs Go
// precisam), mas as receitas de go test/build/vet/get e npm rodam com
// $(SEM_SEGREDOS) (env -u das variáveis do arquivo lido e dos segredos
// fixos, menos NEXT_PUBLIC_*). Só o "make test-integration" (sub-make com
// INTEGRATION=1 na linha de comando) deixa as DB_* chegarem ao go test.
// Testes dinâmicos numa cópia do Makefile em t.TempDir(), com .env FALSO e
// go/npm falsos no PATH que só gravam o ambiente: nada conecta em banco e o
// .env real não é lido.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// alvosSemSegredos: toda chamada a go/npm nas receitas destes alvos passa
// por $(SEM_SEGREDOS).
var alvosSemSegredos = []string{"test-shared", "test-api", "cover-shared", "cover-api", "cover-combinada", "arch-check",
	"lint", "build-shared", "build-api", "deps", "deps-audit-go", "frontend-deps", "dev-frontend", "test-frontend", "deps-audit-frontend"}

var reChamadaGoNpm = regexp.MustCompile(`(^|[\s;&|(])(go|npm|govulncheck)\s`)

func TestSEC53_Makefile_ReceitasUsamSemSegredos(t *testing.T) {
	mk := lerMakefile(t)
	for _, a := range alvosSemSegredos {
		t.Run(a, func(t *testing.T) {
			require.Contains(t, mk.regras, a)
			chamadas := 0
			for _, l := range mk.regras[a].receita {
				for _, m := range reChamadaGoNpm.FindAllStringSubmatchIndex(l, -1) {
					ini := m[4] // início do nome do comando
					antes := l[:ini]
					// Ignora texto entre aspas (echo "... npm outdated ===") e "command -v".
					if strings.Count(antes, `"`)%2 == 1 || strings.HasSuffix(antes, "command -v ") {
						continue
					}
					chamadas++
					assert.True(t, strings.HasSuffix(antes, "$(SEM_SEGREDOS) "), "%s: chamada sem $(SEM_SEGREDOS): %s", a, l)
				}
			}
			assert.Greater(t, chamadas, 0, "%s: nenhuma chamada a go/npm encontrada", a)
		})
	}
	// As CLIs que precisam do ambiente (db-*, fix-*, reset-senhas-*, gen-hash,
	// dev-api, run-api) não passam pelo filtro.
	for _, a := range mk.ordem {
		if strings.HasPrefix(a, "db-") || strings.HasPrefix(a, "fix-") || strings.HasPrefix(a, "reset-senhas-") ||
			a == "gen-hash" || a == "dev-api" || a == "run-api" {
			assert.NotContains(t, strings.Join(mk.regras[a].receita, "\n"), "SEM_SEGREDOS", a)
		}
	}
}

func TestSEC53_Makefile_ListaDeSegredos(t *testing.T) {
	mk := lerMakefile(t)
	fixos := strings.Fields(strings.SplitN(mk.linhaDaVariavel(t, "SEGREDOS_FIXOS"), "=", 2)[1])
	require.NotEmpty(t, fixos)
	for _, v := range []string{"$(VARS_DB_INTEGRACAO)", "MYSQL_PWD", "PASSWORD_PEPPER", "JWT_SECRET", "SMTP_PASSWORD", "SMTP_USER", "TURNSTILE_SECRET_KEY", "SEED_ADMIN_PASSWORD"} {
		assert.Contains(t, fixos, v)
	}
	db := strings.Fields(strings.SplitN(mk.linhaDaVariavel(t, "VARS_DB_INTEGRACAO"), "=", 2)[1])
	for _, v := range []string{"DB_HOST", "DB_PORT", "DB_NAME", "DB_USUARIO", "DB_SENHA", "DB_TLS_CA"} {
		assert.Contains(t, db, v)
	}
	assert.NotContains(t, db, "MYSQL_PWD", "o MYSQL_PWD nunca vai para o go test")
	sem := mk.linhaDaVariavel(t, "SEM_SEGREDOS")
	assert.Contains(t, sem, "filter-out NEXT_PUBLIC_%")
	assert.Contains(t, sem, "$(INTEGRACAO_PELO_MAKE)")
	assert.Contains(t, mk.linhaDaVariavel(t, "INTEGRACAO_PELO_MAKE"), "$(origin INTEGRATION)")

	// O sub-make recebe INTEGRATION=1 como variável de linha de comando.
	receita := strings.Join(mk.regras["test-integration"].receita, "\n")
	assert.Contains(t, receita, `$(MAKE) test INTEGRATION=1 GO_TEST_FLAGS="-p 1 -count=1"`)
	assert.NotContains(t, receita, "INTEGRATION=1 $(MAKE)")
	// Todo alvo que roda go test com $(SEM_SEGREDOS) (DB_* liberadas com
	// INTEGRATION=1 na linha de comando) passa pela guarda de produção.
	for _, a := range []string{"test-shared", "test-api", "cover-shared", "cover-api", "cover-combinada", "arch-check"} {
		assert.Contains(t, mk.regras[a].prereqs, "$(GUARDA_INTEGRACAO)", a)
	}
}

// sec53Dir monta a cópia: Makefile, .env falso, backend/{shared,
// rotaperfumes-api} e frontend vazios e go/npm falsos que gravam o ambiente
// (env) de cada chamada em env.log.
func sec53Dir(t *testing.T) (dir, pathFalso, envLog string) {
	t.Helper()
	dir = copiaDoMakefile(t)
	for _, d := range []string{"backend/shared", "backend/rotaperfumes-api", "frontend", "bin-falso"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.FromSlash(d)), 0o755))
	}
	envFalso := strings.Join([]string{
		"DB_HOST=127.0.0.1",
		"DB_USUARIO=usuario-sec53",
		"DB_SENHA=segredoX-sec53",
		"export PASSWORD_PEPPER=pepper-sec53",
		"JWT_SECRET = jwt-sec53",
		"SMTP_PASSWORD?=smtp-sec53",
		"CUSTOM_SEGREDO:=custom-sec53",
		"NEXT_PUBLIC_API_URL=http://api-sec53",
		"# COMENTADO_SEC53=nao-e-variavel",
		"",
	}, "\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte(envFalso), 0o600))
	envLog = filepath.Join(dir, "env.log")
	script := "#!/bin/sh\n{ echo \"=== $(basename \"$0\") $*\"; env; } >> '" + envLog + "'\nexit 0\n"
	for _, b := range []string{"go", "npm"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "bin-falso", b), []byte(script), 0o755))
	}
	pathFalso = "PATH=" + filepath.Join(dir, "bin-falso") + string(os.PathListSeparator) + os.Getenv("PATH")
	return dir, pathFalso, envLog
}

// varsDoLog devolve os nomes de variáveis presentes no env.log.
func varsDoLog(t *testing.T, envLog string) (map[string]bool, string) {
	t.Helper()
	texto := lerSeExiste(t, envLog)
	require.NotEmpty(t, texto, "go/npm falso não foi chamado")
	nomes := map[string]bool{}
	for _, l := range strings.Split(texto, "\n") {
		if i := strings.Index(l, "="); i > 0 && !strings.HasPrefix(l, "===") {
			nomes[l[:i]] = true
		}
	}
	return nomes, texto
}

var sec53Valores = []string{"segredoX-sec53", "pepper-sec53", "jwt-sec53", "smtp-sec53", "custom-sec53", "usuario-sec53"}

func TestSEC53_Makefile_TestSemSegredos(t *testing.T) {
	dir, path, envLog := sec53Dir(t)
	out, err := makeAlvo(t, dir, "test", []string{path, "MYSQL_PWD=do-shell-sec53", "DB_PORT=3307"})
	require.NoError(t, err, out)
	nomes, texto := varsDoLog(t, envLog)
	assert.Contains(t, texto, "=== go test ./...", "test-shared e test-api chamaram o go test")
	for _, v := range []string{"DB_HOST", "DB_PORT", "DB_NAME", "DB_USUARIO", "DB_SENHA", "MYSQL_PWD", "PASSWORD_PEPPER",
		"JWT_SECRET", "SMTP_PASSWORD", "CUSTOM_SEGREDO", "MYSQL_OPTS", "DB_CLI", "MSYS2_ENV_CONV_EXCL"} {
		assert.False(t, nomes[v], "%s chegou ao go test", v)
	}
	for _, v := range append(sec53Valores, "do-shell-sec53") {
		assert.NotContains(t, texto, v)
	}
	assert.NotContains(t, out, "segredoX-sec53")
}

func TestSEC53_Makefile_TestIntegrationSoComDB(t *testing.T) {
	dir, path, envLog := sec53Dir(t)
	out, err := makeAlvo(t, dir, "test-integration", []string{path})
	require.NoError(t, err, out)
	nomes, texto := varsDoLog(t, envLog)
	for _, v := range []string{"DB_HOST", "DB_USUARIO", "DB_SENHA", "INTEGRATION"} {
		assert.True(t, nomes[v], "%s precisa chegar ao go test de integração", v)
	}
	assert.Contains(t, texto, "DB_SENHA=segredoX-sec53")
	assert.Contains(t, texto, "INTEGRATION=1")
	assert.Contains(t, texto, "-p 1 -count=1", "os pacotes de integração rodam um por vez")
	for _, v := range []string{"MYSQL_PWD", "PASSWORD_PEPPER", "JWT_SECRET", "SMTP_PASSWORD", "CUSTOM_SEGREDO", "MYSQL_OPTS"} {
		assert.False(t, nomes[v], "%s não pode chegar ao go test de integração", v)
	}
	for _, v := range []string{"pepper-sec53", "jwt-sec53", "smtp-sec53", "custom-sec53"} {
		assert.NotContains(t, texto, v)
	}
}

func TestSEC53_Makefile_IntegrationSoNoAmbienteFicaSemDB(t *testing.T) {
	dir, path, envLog := sec53Dir(t)
	out, err := makeAlvo(t, dir, "test", []string{path, "INTEGRATION=1"})
	require.NoError(t, err, out)
	nomes, texto := varsDoLog(t, envLog)
	assert.True(t, nomes["INTEGRATION"], "o INTEGRATION do shell continua no ambiente")
	for _, v := range []string{"DB_HOST", "DB_USUARIO", "DB_SENHA", "MYSQL_PWD"} {
		assert.False(t, nomes[v], "%s chegou ao go test sem o test-integration", v)
	}
	assert.NotContains(t, texto, "segredoX-sec53")
}

func TestSEC53_Makefile_IntegrationNaLinhaDeComandoPassaPelaGuarda(t *testing.T) {
	dir, path, envLog := sec53Dir(t)
	out, err := makeAlvo(t, dir, "test-shared", []string{path}, "INTEGRATION=1", "DB_HOST=192.168.168.106", "DB_PORT=3306")
	require.Error(t, err, out)
	assert.Contains(t, out, "e o banco de PRODUCAO")
	assert.Empty(t, lerSeExiste(t, envLog), "o go test não roda contra a produção")
}

func TestSEC53_Makefile_NpmSemSegredosComNextPublic(t *testing.T) {
	dir, path, envLog := sec53Dir(t)
	out, err := makeAlvo(t, dir, "frontend-deps", []string{path})
	require.NoError(t, err, out)
	nomes, texto := varsDoLog(t, envLog)
	assert.Contains(t, texto, "=== npm install")
	assert.True(t, nomes["NEXT_PUBLIC_API_URL"], "NEXT_PUBLIC_* chega ao npm")
	for _, v := range []string{"DB_SENHA", "MYSQL_PWD", "PASSWORD_PEPPER", "JWT_SECRET", "SMTP_PASSWORD", "CUSTOM_SEGREDO"} {
		assert.False(t, nomes[v], "%s chegou ao npm", v)
	}
	for _, v := range sec53Valores {
		assert.NotContains(t, texto, v)
	}
}

func TestSEC53_Makefile_EnvArquivoTrocaALista(t *testing.T) {
	dir, path, envLog := sec53Dir(t)
	outro := "OUTRO_SEGREDO=outro-sec53\nDB_SENHA=senha-outro-sec53\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.outro"), []byte(outro), 0o600))
	out, err := makeAlvo(t, dir, "lint", []string{path}, "ENV_ARQUIVO=.env.outro")
	require.NoError(t, err, out)
	nomes, texto := varsDoLog(t, envLog)
	assert.Contains(t, texto, "=== go vet ./...")
	assert.False(t, nomes["OUTRO_SEGREDO"], "a lista vem do ENV_ARQUIVO")
	assert.False(t, nomes["DB_SENHA"])
	assert.NotContains(t, texto, "outro-sec53")
	assert.False(t, nomes["CUSTOM_SEGREDO"], "o .env não é lido com ENV_ARQUIVO")
}
