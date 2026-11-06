package deploy_test

// DEPLOY-07 (Lote 33): deploy/seed-producao.sh (estáticos e opções) e a
// guarda do Jenkinsfile para deploy/db-ca.fingerprint sem fingerprint.
//   - imagens: DB_CLIENT_IMAGE = IMAGEM_BASE do mariadb-docker.sh =
//     DB_CLIENT_IMAGE do Jenkinsfile; GO_IMAGE = GO_IMAGE do Jenkinsfile;
//   - cliente do banco no padrão do mariadb_cli do Jenkinsfile (rede, IP do
//     admin, cap-drop, MYSQL_PWD pelo ambiente, TLS verificado);
//   - DDL de sql/ordem-ddl.txt, vendedores inativos, sem -all-users, senha do
//     admin por -password-out/-password-prompt, env-file com shred;
//   - opções e falhas antes da 1ª etapa.
// O ensaio com docker falso fica para o 🔴 TestBrain.

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedProducao(t *testing.T) string {
	t.Helper()
	return lerRaiz(t, "deploy", "seed-producao.sh")
}

// codigoSemComentarios devolve o script sem as linhas de comentário.
func codigoSemComentarios(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "#") {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func TestDEPLOY07_SeedProducao_ImagensIguaisAoJenkinsfile(t *testing.T) {
	sp := seedProducao(t)
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	jf := lerRaiz(t, "Jenkinsfile")
	var (
		cliSeed = regexp.MustCompile(`(?m)^DB_CLIENT_IMAGE="([^"]+)"$`).FindStringSubmatch(sp)
		goSeed  = regexp.MustCompile(`(?m)^GO_IMAGE="([^"]+)"$`).FindStringSubmatch(sp)
		base    = regexp.MustCompile(`(?m)^IMAGEM_BASE="([^"]+)"$`).FindStringSubmatch(md)
		cliJF   = regexp.MustCompile(`(?m)^\s*DB_CLIENT_IMAGE\s*=\s*'([^']+)'`).FindStringSubmatch(jf)
		goJF    = regexp.MustCompile(`(?m)^\s*GO_IMAGE\s*=\s*'([^']+)'`).FindStringSubmatch(jf)
	)
	for nome, m := range map[string][]string{"DB_CLIENT_IMAGE do seed": cliSeed, "GO_IMAGE do seed": goSeed,
		"IMAGEM_BASE": base, "DB_CLIENT_IMAGE do Jenkinsfile": cliJF, "GO_IMAGE do Jenkinsfile": goJF} {
		require.NotNil(t, m, nome)
	}
	assert.Regexp(t, reImagemMariaDB, cliSeed[1])
	assert.Equal(t, base[1], cliSeed[1], "mesma string do IMAGEM_BASE (SEC-31)")
	assert.Equal(t, cliJF[1], cliSeed[1], "mesma string do DB_CLIENT_IMAGE do Jenkinsfile")
	assert.Equal(t, goJF[1], goSeed[1], "mesma string do GO_IMAGE do Jenkinsfile")
	assert.Contains(t, lerRaiz(t, "Makefile"), "deploy/seed-producao.sh", "deps-audit-imagens confere o seed também")
}

func TestDEPLOY07_SeedProducao_Contrato(t *testing.T) {
	sp := seedProducao(t)
	codigo := codigoSemComentarios(sp)

	assert.NotRegexp(t, `(?m)^\s*set\s+-[a-z]*x`, codigo, "nunca set -x")
	assert.Contains(t, codigo, "set -euo pipefail")

	// Cliente do banco: mesmo padrão do mariadb_cli do Jenkinsfile.
	for _, s := range []string{
		`MYSQL_PWD="$SENHA_DB" docker run --rm -i -e MYSQL_PWD`,
		`--network "$REDE_NOME" --ip "$IP_ADMIN"`,
		"--cap-drop ALL --security-opt no-new-privileges:true",
		`--mount "type=bind,source=$CA_HOST,target=$C_CA,readonly"`,
		`--local-infile=0 -h "$CONTAINER"`,
		`--ssl-ca="$C_CA" --ssl-verify-server-cert`,
		"--default-character-set=utf8mb4",
	} {
		assert.Contains(t, codigo, s)
	}
	// TST-17 (Lote 37): os padrões viram *_PADRAO; os valores ficam readonly
	// depois de validados na etapa 1.
	for _, s := range []string{`readonly IP_ADMIN_PADRAO="172.30.42.20"`, `readonly REDE_NOME_PADRAO="rotaperfumes-db"`,
		`readonly CONTAINER_PADRAO="rotaperfumes-mariadb"`, `readonly SSL_DIR_PADRAO="/etc/mysql/ssl"`,
		`readonly SEGREDOS_DIR_PADRAO="/root/rotaperfumes-mariadb"`, `readonly TRABALHO_DIR_PADRAO="/root"`,
		`readonly CA_HOST="$SSL_DIR/ca.pem"`, `readonly C_CA="/etc/mysql/ssl/ca.pem"`,
		`readonly CONTAINER REDE_NOME IP_ADMIN SEGREDOS_DIR SSL_DIR TRABALHO_DIR ADMIN_SENHA_ARQ`,
		`readonly SENHA_DB_ARQ="$SEGREDOS_DIR/$DB_ADMIN.senha"`, `readonly DB_ADMIN="rotaperfumes_admin"`} {
		assert.Contains(t, codigo, s)
	}

	// DDL pela lista compartilhada com o make db-up; seeds 02 e 03.
	assert.Contains(t, codigo, `ORDEM_DDL="$SQL_DIR/ordem-ddl.txt"`)
	assert.Contains(t, codigo, "SEEDS=(02_seed_admin.sql 03_seed_vendedores.sql)")
	// Vendedores reais entram inativos; sem -all-users.
	assert.Contains(t, codigo, `UPDATE usuarios SET ativo = 0 WHERE role <> 'admin'`)
	// O resetpassword só roda com -create-admin (o -all-users só aparece no --help, como "NAO roda").
	chamadas := regexp.MustCompile(`"\$GO_IMAGE" -[a-z-]+`).FindAllString(codigo, -1)
	require.Len(t, chamadas, 2, "prompt e -password-out")
	for _, c := range chamadas {
		assert.Equal(t, `"$GO_IMAGE" -create-admin`, c)
	}
	assert.Equal(t, 1, strings.Count(codigo, "-all-users"), "só no texto do --help")
	assert.Contains(t, codigo, "NAO roda -all-users")
	assert.NotContains(t, codigo, "make db-seed\n")
	// Admin: senha gerada em arquivo ou digitada; nunca -password=.
	assert.Contains(t, codigo, "-create-admin -password-out /saida/admin.senha")
	assert.Contains(t, codigo, "-create-admin -password-prompt")
	assert.NotRegexp(t, `-password[= ]['"]?[A-Za-z0-9]`, strings.ReplaceAll(codigo, "-password-", ""))
	// env-file temporário 600 com shred; pepper sem eco.
	assert.Contains(t, codigo, `--env-file "$ENV_ARQ"`)
	assert.Contains(t, codigo, `chmod 600 "$ENV_ARQ"`)
	assert.Contains(t, codigo, `shred -u "$ENV_ARQ"`)
	assert.Contains(t, codigo, "trap limpar EXIT")
	assert.Contains(t, codigo, "read -rs")
	for _, v := range []string{"DB_HOST", "DB_PORT", "DB_USUARIO", "DB_SENHA", "DB_NAME", "DB_TLS_CA", "DB_TLS_SERVER_NAME", "PASSWORD_PEPPER"} {
		assert.Contains(t, codigo, "printf '"+v+"=", "env-file com %s", v)
	}
	// Segredos nunca como -e VAR=valor.
	assert.NotRegexp(t, `-e\s+(MYSQL_PWD|DB_SENHA|PASSWORD_PEPPER)=`, codigo)
	// Fail closed com usuarios preenchida, salvo --forcar com o nome do banco.
	assert.Contains(t, codigo, `$FORCAR || die "a tabela usuarios ja tem`)
	assert.Contains(t, codigo, `[[ "$RESPOSTA" == "$DB_NOME" ]]`)
}

func TestDEPLOY07_SeedProducao_Opcoes(t *testing.T) {
	precisaBash(t)
	sc := caminhoRaiz(t, "deploy", "seed-producao.sh")
	rodar := func(args ...string) (string, int) {
		cmd := exec.Command("bash", append([]string{sc}, args...)...)
		cmd.Env = ambienteLimpo()
		out, err := cmd.CombinedOutput()
		if ee, ok := err.(*exec.ExitError); ok {
			return string(out), ee.ExitCode()
		}
		require.NoError(t, err, "%s", out)
		return string(out), 0
	}

	out, rc := rodar("--help")
	require.Equal(t, 0, rc, out)
	for _, s := range []string{"--pepper-file", "--admin-senha-prompt", "--admin-senha-arquivo", "--forcar",
		"sql/ordem-ddl.txt", "ativo = 0 WHERE role <> 'admin'", "NAO roda -all-users", "make db-seed"} {
		assert.Contains(t, out, s)
	}

	casos := []struct {
		nome string
		args []string
		rc   int
		msg  string
	}{
		{"opcao desconhecida", []string{"--all-users"}, 2, "Opcao desconhecida: --all-users"},
		{"--pepper-file sem valor", []string{"--pepper-file"}, 1, "a opcao --pepper-file exige um valor"},
		{"--admin-senha-arquivo vazio", []string{"--admin-senha-arquivo", ""}, 1, "a opcao --admin-senha-arquivo exige um valor"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, rc := rodar(c.args...)
			assert.Equal(t, c.rc, rc, out)
			assert.Contains(t, out, c.msg)
			assert.NotContains(t, out, "=== [1/", "falha antes da primeira etapa")
		})
	}
	t.Run("sem root", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("rodando como root")
		}
		out, rc := rodar()
		assert.NotEqual(t, 0, rc)
		assert.Contains(t, out, "rode como root")
	})
}

// DEPLOY-07: deploy/db-ca.fingerprint só com comentários (instalação nova) faz
// o estágio "CA do banco" falhar com mensagem clara (o grep sem linhas não
// derruba o pipefail em silêncio), antes do container de validação.
func TestDEPLOY07_Jenkinsfile_FingerprintVazioFalhaClaro(t *testing.T) {
	jf := lerRaiz(t, "Jenkinsfile")
	iGuarda := strings.Index(jf, `FPS=$(grep -Ev '^[[:space:]]*(#|$)' deploy/db-ca.fingerprint || true)`)
	iMsg := strings.Index(jf, "deploy/db-ca.fingerprint sem fingerprint: rode mariadb-docker.sh --nova-ca e grave o CA_FP")
	iValidar := strings.Index(jf, "deploy/validar-ca-banco.sh)")
	require.GreaterOrEqual(t, iGuarda, 0)
	require.GreaterOrEqual(t, iMsg, 0)
	require.GreaterOrEqual(t, iValidar, 0)
	assert.Less(t, iGuarda, iMsg)
	assert.Less(t, iMsg, iValidar, "a guarda vem antes do container de validação")
	assert.NotContains(t, jf, "grep -Ev '^[[:space:]]*(#|$)' deploy/db-ca.fingerprint | sed", "grep sem || true sob pipefail")
}

// DEPLOY-07 / SecBrain: o fingerprint da CA de DEV (CasaOS) nunca está no
// deploy/db-ca.fingerprint, e o mariadb-docker.sh não manda copiar a CA de
// produção para deploy/ca-cert.pem.
func TestDEPLOY07_CADevNuncaEmProducao(t *testing.T) {
	const caDev = "ef31b6565518b18bbd5ababf94db6bca76de356f8e0f013491dbec7f5aedb37b"
	assert.NotContains(t, linhasUteis(lerRaiz(t, "deploy", "db-ca.fingerprint")), caDev)
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	assert.Contains(t, md, "NAO copie esta CA para deploy/ca-cert.pem")
	assert.NotRegexp(t, `copie \$SSL_DIR/ca\.pem para\s+deploy/ca-cert\.pem`, md)
	assert.Contains(t, md, "deploy/ca-producao.pem")
	assert.Contains(t, md, "sudo bash deploy/seed-producao.sh")
	assert.NotContains(t, codigoSemComentarios(md), "rotaperfumes\\\\_%", "sem GRANT em bancos de teste")
	gi := lerRaiz(t, ".gitignore")
	for _, s := range []string{"deploy/ca-producao.pem", ".env.producao", "deploy/ca-cert.pem"} {
		assert.Regexp(t, `(?m)^`+regexp.QuoteMeta(s)+`$`, gi)
	}
}
