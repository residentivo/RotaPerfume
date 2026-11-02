package deploy_test

// DEPLOY-06 / DEPLOY-04 (Lote 30, TST-15): deploy/setup-servidor.sh depois de
// sair o banco nativo (substitui os testes do DEPLOY-02, que testavam o
// hardening/sabor MySQL/--apenas-banco removidos).
//   - o script não instala nem configura banco: só confere o container e a
//     rede do deploy/mariadb-docker.sh (checar_banco_docker) e lê a senha
//     gerada por ele;
//   - --apenas-banco e --skip-mysql saíram e dão erro apontando para o
//     mariadb-docker.sh;
//   - Jenkins nativo (systemd) ou em container (checar_jenkins);
//   - numeração das etapas contínua [1/9]..[9/9] nos dois modos.
// O script é carregado com "source" (o main só roda quando executado
// diretamente) e docker/systemctl/id/... viram funções stub.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupServidor(t *testing.T) string {
	t.Helper()
	precisaBash(t)
	return caminhoRaiz(t, "deploy", "setup-servidor.sh")
}

// bashSetup faz "source" do setup-servidor.sh e roda corpo. FAKE = dir.
func bashSetup(t *testing.T, dir, corpo string, env ...string) (string, error) {
	t.Helper()
	sc := setupServidor(t)
	if dir == "" {
		dir = t.TempDir()
	}
	cmd := exec.Command("bash", "-c", `source "$1"`+"\n"+corpo, "deploy06", sc)
	cmd.Env = append(append(ambienteLimpo(), "FAKE="+dir), env...)
	cmd.Stdin = strings.NewReader("\n")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ---------------------------------------------------------------------------
// Argumentos
// ---------------------------------------------------------------------------

func TestDEPLOY06_SetupServidor_OpcoesAntigasDaoErro(t *testing.T) {
	for _, op := range []string{"--apenas-banco", "--skip-mysql"} {
		for _, prefixo := range []string{"", "--env /tmp/x.env "} {
			t.Run(prefixo+op, func(t *testing.T) {
				out, err := bashSetup(t, "", `ler_argumentos `+prefixo+op+`; echo NAO_DEVIA_CHEGAR`)
				require.Error(t, err, out)
				assert.Contains(t, out, "ERRO: A opcao "+op+" saiu (DEPLOY-06)")
				assert.Contains(t, out, "mariadb-docker.sh")
				assert.NotContains(t, out, "NAO_DEVIA_CHEGAR")
			})
		}
	}
}

func TestDEPLOY06_SetupServidor_ValidarArgumentos(t *testing.T) {
	casos := []struct {
		nome, args, erro string
	}{
		{"minimo", "--env /tmp/x.env", ""},
		{"todas as opcoes", "--env /tmp/x.env --dump /tmp/d.sql.gz --turnstile-site-key 0x4AAA_b-c --site-host srv.lan --site-ip 10.0.0.5 --https-port 443 --jenkins-url http://localhost:8080/ --jenkins-container jenkins --db-container rotaperfumes-mariadb --db-segredos /root/x/ --skip-build --remove-inputs", ""},
		{"sem --env", "--skip-build", "Informe --env ARQUIVO"},
		{"opcao desconhecida", "--env /tmp/x.env --sabor mysql", "Opcao desconhecida: --sabor"},
		{"valor ausente", "--env", "A opcao --env exige um valor"},
		{"site-ip invalido", "--env /tmp/x.env --site-ip 10.0.0", "--site-ip invalido"},
		{"https-port fora da faixa", "--env /tmp/x.env --https-port 70000", "--https-port invalida"},
		{"jenkins-url invalida", "--env /tmp/x.env --jenkins-url ftp://x", "--jenkins-url invalida"},
		{"jenkins-container invalido", "--env /tmp/x.env --jenkins-container -x", "--jenkins-container invalido"},
		{"db-container com maiuscula", "--env /tmp/x.env --db-container Banco", "--db-container invalido"},
		{"db-segredos relativo", "--env /tmp/x.env --db-segredos segredos", "--db-segredos precisa ser caminho absoluto"},
		{"turnstile invalida", "--env /tmp/x.env --turnstile-site-key a.b", "--turnstile-site-key invalida"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, err := bashSetup(t, "", `ler_argumentos `+c.args+` && echo "OK db=$DB_CONTAINER seg=$DB_SEGREDOS_DIR url=$JENKINS_URL"`)
			if c.erro == "" {
				require.NoError(t, err, out)
				assert.Contains(t, out, "OK db=rotaperfumes-mariadb")
				if strings.Contains(c.args, "--db-segredos") {
					assert.Contains(t, out, "seg=/root/x url=http://localhost:8080\n", "barra final removida")
				}
				return
			}
			require.Error(t, err, out)
			assert.Contains(t, out, c.erro)
		})
	}
}

func TestDEPLOY06_SetupServidor_UsoApontaParaMariadbDocker(t *testing.T) {
	out, err := bashSetup(t, "", `uso`)
	require.NoError(t, err, out)
	assert.Contains(t, out, "deploy/mariadb-docker.sh")
	assert.Contains(t, out, "--db-container")
	assert.Contains(t, out, "--jenkins-container")
	for _, s := range []string{"--apenas-banco", "--skip-mysql", "MySQL", "mysql"} {
		assert.NotContains(t, out, s)
	}
}

// O script não pode voltar a mexer no banco nativo (DEPLOY-06).
func TestDEPLOY06_SetupServidor_SemBancoNativo(t *testing.T) {
	texto := lerRaiz(t, "deploy", "setup-servidor.sh")
	for _, proibido := range []string{
		"zz-rotaperfumes-hardening", "zz-rotaperfumes-bind", "mysql_root", "mysql_consulta",
		"MYSQL_SABOR", "MYSQL_SERVICO", "CREATE USER", "SET GLOBAL", "sql_mode", "bind-address",
		"mariadb-setup.sql", "systemctl restart mysql", "systemctl restart mariadb",
		"apt-get install", "mariadb-server", "detectar_sabor",
	} {
		assert.NotContains(t, texto, proibido)
	}
	assert.NotRegexp(t, `(?m)^\s*ufw allow .*3306`, texto, "porta do banco é do mariadb-docker.sh/DOCKER-USER")
	_, err := os.Stat(caminhoRaiz(t, "deploy", "mariadb-setup.sql"))
	assert.True(t, os.IsNotExist(err), "deploy/mariadb-setup.sql saiu do repositório (SEC-39)")
}

// ---------------------------------------------------------------------------
// checar_banco_docker
// ---------------------------------------------------------------------------

const stubDockerBanco = `
docker() {
  case "$*" in
    *State.Running*) printf '%s\n' "${FAKE_RODANDO:-}"; [[ -n "${FAKE_RODANDO:-}" ]] ;;
    "network inspect rotaperfumes-db") [[ "${FAKE_REDE:-}" == 1 ]] ;;
    *NetworkSettings.Networks*) printf '%s\n' "${FAKE_IP:-}" ;;
    *) echo "docker inesperado: $*" >&2; return 99 ;;
  esac
}
DB_SEGREDOS_DIR=$FAKE/seg
`

func TestDEPLOY06_ChecarBancoDocker(t *testing.T) {
	casos := []struct {
		nome      string
		env       []string
		senhas    []string
		erro, msg string
	}{
		{"ok", []string{"FAKE_RODANDO=true", "FAKE_REDE=1", "FAKE_IP=172.30.42.2"}, []string{"rotaperfumes_app", "rotaperfumes_admin"}, "",
			"OK: MariaDB do mariadb-docker.sh: container rotaperfumes-mariadb na rede rotaperfumes-db (172.30.42.2)"},
		{"container inexistente", nil, nil, "Container do banco 'rotaperfumes-mariadb' nao esta rodando. Rode antes: sudo bash deploy/mariadb-docker.sh", ""},
		{"container parado", []string{"FAKE_RODANDO=false", "FAKE_REDE=1", "FAKE_IP=172.30.42.2"}, nil, "nao esta rodando", ""},
		{"sem a rede", []string{"FAKE_RODANDO=true"}, nil, "Rede docker 'rotaperfumes-db' nao existe", ""},
		{"fora da rede", []string{"FAKE_RODANDO=true", "FAKE_REDE=1"}, nil, "nao esta na rede 'rotaperfumes-db'. Rode: sudo bash deploy/mariadb-docker.sh --recriar", ""},
		{"sem senha do app", []string{"FAKE_RODANDO=true", "FAKE_REDE=1", "FAKE_IP=172.30.42.2"}, []string{"rotaperfumes_admin"}, "/seg/rotaperfumes_app.senha", ""},
		{"sem senha do admin", []string{"FAKE_RODANDO=true", "FAKE_REDE=1", "FAKE_IP=172.30.42.2"}, []string{"rotaperfumes_app"}, "/seg/rotaperfumes_admin.senha", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir := t.TempDir()
			for _, s := range c.senhas {
				escrever(t, filepath.Join(dir, "seg", s+".senha"), "Abc123def456ghi789jkl012", 0o600)
			}
			out, err := bashSetup(t, dir, stubDockerBanco+`checar_banco_docker; echo FIM`, c.env...)
			assert.NotContains(t, out, "docker inesperado")
			if c.erro != "" {
				require.Error(t, err, out)
				assert.Contains(t, out, c.erro)
				assert.NotContains(t, out, "FIM")
				return
			}
			require.NoError(t, err, out)
			assert.Contains(t, out, c.msg)
			assert.NotContains(t, out, "Abc123", "senha nunca vai para a tela")
		})
	}
}

func TestDEPLOY06_LerSenhaApp(t *testing.T) {
	casos := map[string]bool{
		"Abc123def456ghi789jkl012\n": true,
		"Abc123def456ghi789jkl012":   true,
		"com'aspas":                  false,
		"com espaco":                 false,
		"":                           false,
	}
	for conteudo, valido := range casos {
		t.Run(strconv.Quote(conteudo), func(t *testing.T) {
			dir := t.TempDir()
			escrever(t, filepath.Join(dir, "rotaperfumes_app.senha"), conteudo, 0o600)
			out, err := bashSetup(t, dir, `DB_SEGREDOS_DIR=$FAKE; ler_senha_app; [[ -n "$SENHA_APP" ]] && echo LIDA`)
			if valido {
				require.NoError(t, err, out)
				assert.Equal(t, "LIDA\n", out, "não imprime a senha")
				return
			}
			require.Error(t, err, out)
			assert.Contains(t, out, "fora do formato esperado")
			if conteudo != "" {
				assert.NotContains(t, out, conteudo)
			}
		})
	}
}

func TestDEPLOY06_LinhasApiEnv_UsaContainerDoBanco(t *testing.T) {
	out, err := bashSetup(t, "", `linhas_api_env "$DB_PORTA_INTERNA" Senha123 jwt srv 10.0.0.5 8443
DB_CONTAINER=outro-banco; linhas_api_env 3306 S j h 1.2.3.4 443 | grep '^DB_HOST='`)
	require.NoError(t, err, out)
	assert.Contains(t, out, "DB_HOST=rotaperfumes-mariadb\nDB_PORT=3306\nDB_NAME=rotaperfumes\nDB_USUARIO=rotaperfumes_app\nDB_SENHA=Senha123\n")
	assert.Contains(t, out, "COOKIE_SECURE=true\n")
	assert.NotContains(t, out, "host.docker.internal")
	assert.True(t, strings.HasSuffix(out, "DB_HOST=outro-banco\n"), "--db-container vira o DB_HOST: %q", out)
}

// ---------------------------------------------------------------------------
// checar_jenkins (DEPLOY-04)
// ---------------------------------------------------------------------------

const stubJenkins = `
systemctl() { [[ "$1 $2" == "cat jenkins" && "${FAKE_SYSTEMD:-}" == 1 ]]; }
id() { [[ "${FAKE_ID:-1}" == 1 ]]; }
runuser() { :; }
docker() {
  case "$1" in
    ps) printf '%b' "${FAKE_PS:-}" ;;
    inspect) local n=${!#}; [[ " ${FAKE_RODANDO:-} " == *" $n "* ]] && echo true || echo false ;;
    *) echo "docker inesperado: $*" >&2; return 99 ;;
  esac
}
`

func TestDEPLOY04_ChecarJenkins(t *testing.T) {
	casos := []struct {
		nome       string
		pre        string
		env        []string
		modo, cont string
		erro       string
	}{
		{"nativo", "", []string{"FAKE_SYSTEMD=1"}, "nativo", "", ""},
		{"nativo sem usuario jenkins", "", []string{"FAKE_SYSTEMD=1", "FAKE_ID=0"}, "", "", "Servico jenkins existe, mas o usuario 'jenkins' nao"},
		{"container detectado", "", []string{`FAKE_PS=rotaperfumes-caddy-1 caddy:2\njk jenkins/jenkins:lts-jdk21\n`, "FAKE_RODANDO=jk"}, "container", "jk", ""},
		{"container detectado por digest", "", []string{`FAKE_PS=meu-jk registry.lan/jenkins@sha256:abc\n`, "FAKE_RODANDO=meu-jk"}, "container", "meu-jk", ""},
		{"container parado", "", []string{`FAKE_PS=jk jenkins/jenkins:lts\n`}, "", "", "Container do Jenkins 'jk' nao esta rodando"},
		{"dois containers", "", []string{`FAKE_PS=a jenkins/jenkins:lts\nb jenkins:2\n`, "FAKE_RODANDO=a b"}, "", "", "Mais de um container de Jenkins (a b)"},
		{"nenhum", "", []string{`FAKE_PS=x nginx:1\njenkinsfile-lint alpine:3\n`}, "", "", "Jenkins nao encontrado"},
		{"--jenkins-container ganha do systemd", "JENKINS_CONTAINER=ci", []string{"FAKE_SYSTEMD=1", "FAKE_RODANDO=ci"}, "container", "ci", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, err := bashSetup(t, "", stubJenkins+c.pre+"\nchecar_jenkins; echo \"MODO=$JENKINS_MODO C=$JENKINS_CONTAINER\"", c.env...)
			assert.NotContains(t, out, "docker inesperado")
			if c.erro != "" {
				require.Error(t, err, out)
				assert.Contains(t, out, c.erro)
				return
			}
			require.NoError(t, err, out)
			assert.Contains(t, out, "MODO="+c.modo+" C="+c.cont+"\n")
		})
	}
}

func TestDEPLOY04_EhImagemJenkins(t *testing.T) {
	casos := map[string]bool{
		"jenkins/jenkins:lts":           true,
		"jenkins/jenkins":               true,
		"jenkins:2.440":                 true,
		"registry.lan/jenkins:lts":      true,
		"registry.lan/jenkins@sha256:a": true,
		"jenkins/inbound-agent:latest":  true,
		"myjenkins:1":                   false,
		"jenkinsci-lint:1":              false,
		"nginx:1":                       false,
		"caddy:2":                       false,
	}
	for img, esperado := range casos {
		t.Run(img, func(t *testing.T) {
			out, err := bashSetup(t, "", `if eh_imagem_jenkins '`+img+`'; then echo SIM; else echo NAO; fi`)
			require.NoError(t, err, out)
			assert.Equal(t, map[bool]string{true: "SIM\n", false: "NAO\n"}[esperado], out)
		})
	}
}

func TestDEPLOY04_CaminhoNoContainer(t *testing.T) {
	mounts := "/var/lib/docker/volumes/jh/_data|/var/jenkins_home\n/opt/rotaperfumes/|/opt/rotaperfumes\n/srv|/dados/\n"
	casos := []struct{ alvo, esperado string }{
		{"/opt/rotaperfumes/dumps/rotaperfumes.sql.gz", "/opt/rotaperfumes/dumps/rotaperfumes.sql.gz"},
		{"/srv/a/b", "/dados/a/b"},
		{"/srv", "/dados"},
		{"/srvx/a", ""},
		{"/opt/outro", ""},
	}
	for _, c := range casos {
		t.Run(c.alvo, func(t *testing.T) {
			out, err := bashSetup(t, "", `caminho_no_container "$M" '`+c.alvo+`' || echo NENHUM`, "M="+mounts)
			require.NoError(t, err, out)
			if c.esperado == "" {
				assert.Equal(t, "NENHUM\n", out)
			} else {
				assert.Equal(t, c.esperado, out)
			}
		})
	}
}

// Jenkins em container sem bind do /opt/rotaperfumes: dump vai por docker cp
// para o JENKINS_HOME, com dono do Jenkins e 600.
func TestDEPLOY04_InstalarDumpContainer(t *testing.T) {
	casos := []struct {
		nome, mounts, destino string
		cp                    bool
	}{
		{"sem bind: docker cp", "/var/lib/docker/volumes/jh/_data|/var/jenkins_home", "/var/jenkins_home/rotaperfumes-dumps/rotaperfumes.sql.gz", true},
		{"com bind do /opt/rotaperfumes", "/opt/rotaperfumes|/opt/rotaperfumes", "/opt/rotaperfumes/dumps/rotaperfumes.sql.gz", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir := t.TempDir()
			out, err := bashSetup(t, dir, `
reg() { printf '%s\n' "$*" >>"$FAKE/log"; }
docker() {
  case "$1 $2" in
    "inspect -f") printf '%s\n' "$FAKE_MOUNTS" ;;
    "exec jk") case "$3" in id) [[ "$4" == -u ]] && echo 1000 || echo 1000 ;; test) reg "docker $*" ;; esac ;;
    *) reg "docker $*" ;;
  esac
}
install() { reg "install $*"; }
JENKINS_CONTAINER=jk; JENKINS_MODO=container; ARQ_DUMP=/tmp/d.sql.gz
etapa_dump; echo "JOB=$DUMP_PATH_JOB"`, "FAKE_MOUNTS="+c.mounts)
			require.NoError(t, err, out)
			assert.Contains(t, out, "JOB="+c.destino+"\n")
			log, _ := os.ReadFile(filepath.Join(dir, "log"))
			if c.cp {
				assert.Contains(t, string(log), "docker cp /tmp/d.sql.gz jk:"+c.destino)
				assert.Contains(t, string(log), "docker exec -u 0 jk chmod 600 "+c.destino)
				assert.Contains(t, string(log), "docker exec -u 0 jk chown 1000:1000 "+c.destino)
				assert.NotContains(t, string(log), "\ninstall ")
			} else {
				assert.Contains(t, string(log), "install -m 640 -o root -g 1000 /tmp/d.sql.gz /opt/rotaperfumes/dumps/rotaperfumes.sql.gz")
				assert.NotContains(t, string(log), "docker cp")
			}
			assert.Contains(t, string(log), "docker exec jk test -r "+c.destino)
		})
	}
}

// ---------------------------------------------------------------------------
// Numeração contínua (DEPLOY-04)
// ---------------------------------------------------------------------------

func TestDEPLOY04_EtapasDeclaradasIguaisAoTotal(t *testing.T) {
	texto := lerRaiz(t, "deploy", "setup-servidor.sh")
	m := regexp.MustCompile(`(?m)^readonly TOTAL_ETAPAS=(\d+)$`).FindStringSubmatch(texto)
	require.NotNil(t, m)
	total, _ := strconv.Atoi(m[1])
	assert.Equal(t, 9, total)
	chamadas := regexp.MustCompile(`(?m)^\s+etapa "`).FindAllString(texto, -1)
	assert.Len(t, chamadas, total, "cada etapa_* chama etapa uma vez; o total bate com TOTAL_ETAPAS")
}

func TestDEPLOY04_NumeracaoContinua(t *testing.T) {
	precisaOpenSSL(t)
	for _, modo := range []string{"nativo", "container"} {
		t.Run(modo, func(t *testing.T) {
			dir := t.TempDir()
			escrever(t, filepath.Join(dir, "seg", "rotaperfumes_app.senha"), "Abc123def456ghi789jkl012\n", 0o600)
			escrever(t, filepath.Join(dir, "seg", "rotaperfumes_admin.senha"), "Xyz123def456ghi789jkl012\n", 0o600)
			escrever(t, filepath.Join(dir, "origem.env"), "SMTP_HOST=smtp.exemplo\nTURNSTILE_SECRET_KEY=0x4AAAreal\n", 0o600)
			out, err := bashSetup(t, dir, `
checar_root_e_usuario() { USUARIO=u; USUARIO_HOME=$FAKE; USUARIO_GRUPO=g; }
checar_comandos() { :; }; checar_compose() { :; }; checar_entradas() { :; }
checar_jenkins() { JENKINS_MODO=$FAKE_MODO; JENKINS_CONTAINER=jk; }
checar_banco_docker() { echo "SUB banco"; }
etapa_jenkins_docker_nativo() { echo "SUB nativo"; }
etapa_jenkins_docker_container() { echo "SUB container"; }
ufw_ativo() { return 0; }; ufw() { :; }
install() { cp -- "${@: -2:1}" "${@: -1}"; }
aguardar_jenkins() { :; }; pedir_token_jenkins() { :; }; validar_login_jenkins() { :; }
checar_plugins() { :; }; publicar_job() { :; }; credencial_existe() { return 0; }
exportar_ca_caddy() { return 1; }
main --env "$FAKE/origem.env" --skip-build --db-segredos "$FAKE/seg"`, "FAKE_MODO="+modo)
			require.NoError(t, err, out)
			re := regexp.MustCompile(`(?m)^==> \[(\d+)/(\d+)\] (.*)$`)
			achadas := re.FindAllStringSubmatch(out, -1)
			require.Len(t, achadas, 9, out)
			for i, a := range achadas {
				assert.Equal(t, strconv.Itoa(i+1), a[1], "numeração contínua: %v", a[0])
				assert.Equal(t, "9", a[2])
			}
			assert.Equal(t, "Jenkins com acesso ao Docker ("+modo+")", achadas[1][3])
			assert.Contains(t, out, "SUB "+modo)
			assert.Contains(t, out, "SUB banco", "pré-checagem do banco do mariadb-docker.sh")
			assert.Contains(t, out, "DB_HOST=rotaperfumes-mariadb")
			assert.NotContains(t, out, "Abc123", "senha do app nunca na tela")
			assert.Contains(t, out, "\nConcluido.\n")
			_, errEnv := os.Stat(filepath.Join(dir, "rotaperfumes-api.env"))
			assert.True(t, os.IsNotExist(errEnv), "api.env apagado depois das credenciais")
		})
	}
}

// ---------------------------------------------------------------------------
// Turnstile (SEC-37/DEPLOY-02), mantido
// ---------------------------------------------------------------------------

func TestDEPLOY02_AvisoTurnstileTeste_SemImprimirValor(t *testing.T) {
	casos := map[string]bool{
		"TURNSTILE_SECRET_KEY=1x0000000000000000000000000000000AA\n":            true,
		"export TURNSTILE_SECRET_KEY='2x0000000000000000000000000000000AA'\r\n": true,
		"TURNSTILE_SECRET_KEY = \"3x0000000000000000000000000000000AA\"\n":      true,
		"TURNSTILE_SECRET_KEY=0x4AAAAAAAreal\n":                                 false,
		"# TURNSTILE_SECRET_KEY=1x0000000000000000000000000000000AA\n":          false,
	}
	for conteudo, teste := range casos {
		t.Run(strings.TrimSpace(conteudo), func(t *testing.T) {
			dir := t.TempDir()
			escrever(t, filepath.Join(dir, "origem.env"), conteudo, 0o600)
			out, err := bashSetup(t, dir, `ARQ_ENV=$FAKE/origem.env; avisar_turnstile_teste`)
			require.NoError(t, err, out)
			if teste {
				assert.Contains(t, out, "secret de TESTE")
			} else {
				assert.Empty(t, out)
			}
			assert.NotContains(t, out, "0000000000", "não imprime o valor")
		})
	}
}
