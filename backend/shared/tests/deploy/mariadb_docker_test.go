package deploy_test

// TST-15 / SEC-45 / SEC-39 / SEC-31 / SEC-30 (Lote 30): deploy/mariadb-docker.sh.
//   - estáticos: bash -n, shellcheck (se houver), todo CREATE USER em deploy/
//     com VIA ed25519 e REQUIRE SSL, nenhum -p do Docker sem IP, IMAGEM_BASE
//     igual ao DB_CLIENT_IMAGE do Jenkinsfile (ambos @sha256), SAN sem
//     host.docker.internal, opções removidas (--versao, --rede-lan) dão erro;
//   - ensaio com docker/iptables/systemctl FALSOS (stubs no PATH) rodando o
//     script REAL num user namespace (EUID 0) com binds dos caminhos fixos
//     (/root, /opt, /usr/local/sbin, /etc/systemd/system) para um TempDir:
//     rede, contas por IP, convergência, firewall (--dev-ip), CA (--ca-key,
//     --nova-ca, --novo-cert com FLUSH SSL), reexecução;
//   - Docker real: só com root + Docker acessível + opt-in (TST15_DOCKER=1),
//     senão skip (pendência registrada no TST-15).

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mariadbDocker(t *testing.T) string {
	t.Helper()
	precisaBash(t)
	return caminhoRaiz(t, "deploy", "mariadb-docker.sh")
}

// ---------------------------------------------------------------------------
// Estáticos
// ---------------------------------------------------------------------------

func scriptsDeploy(t *testing.T) []string {
	t.Helper()
	sh, err := filepath.Glob(caminhoRaiz(t, "deploy", "*.sh"))
	require.NoError(t, err)
	require.NotEmpty(t, sh)
	return sh
}

func TestTST15_ScriptsDeploy_BashN(t *testing.T) {
	precisaBash(t)
	for _, s := range scriptsDeploy(t) {
		t.Run(filepath.Base(s), func(t *testing.T) {
			out, err := exec.Command("bash", "-n", s).CombinedOutput()
			require.NoError(t, err, "%s", out)
		})
	}
}

func TestTST15_ScriptsDeploy_Shellcheck(t *testing.T) {
	sc, err := exec.LookPath("shellcheck")
	if err != nil {
		t.Skip("shellcheck não está no PATH (instale para rodar este teste)")
	}
	for _, s := range scriptsDeploy(t) {
		t.Run(filepath.Base(s), func(t *testing.T) {
			out, err := exec.Command(sc, "-S", "error", s).CombinedOutput()
			require.NoError(t, err, "%s", out)
		})
	}
}

// arquivosDeploy: texto de deploy/ (sem dumps nem binários).
func arquivosDeploy(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	base := caminhoRaiz(t, "deploy")
	require.NoError(t, filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "dumps" {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".gz", ".pem", ".crt", ".key", ".png", ".jpg":
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(base, p)
		out[rel] = string(b)
		return nil
	}))
	return out
}

// SEC-39/SEC-30: todo CREATE USER em deploy/ é ed25519 + REQUIRE SSL; os
// ALTER USER ... IDENTIFIED também mantêm o REQUIRE SSL.
func TestSEC39_CreateUserEmDeploy_Ed25519ERequireSSL(t *testing.T) {
	reCreate := regexp.MustCompile(`(?i)CREATE\s+USER\b[^;]*;?`)
	reAlter := regexp.MustCompile(`(?i)ALTER\s+USER\b[^;]*IDENTIFIED[^;]*;?`)
	total := 0
	for nome, texto := range arquivosDeploy(t) {
		for _, l := range strings.Split(texto, "\n") {
			for _, m := range reCreate.FindAllString(l, -1) {
				total++
				assert.Contains(t, m, "IDENTIFIED VIA ed25519", "%s: %s", nome, m)
				assert.Contains(t, m, "REQUIRE SSL", "%s: %s", nome, m)
			}
			for _, m := range reAlter.FindAllString(l, -1) {
				assert.Contains(t, m, "REQUIRE SSL", "%s: %s", nome, m)
			}
		}
	}
	assert.GreaterOrEqual(t, total, 3, "app, admin e casaos (o teste não pode ficar verde vazio)")
	_, err := os.Stat(caminhoRaiz(t, "deploy", "mariadb-setup.sql"))
	assert.True(t, os.IsNotExist(err), "deploy/mariadb-setup.sql saiu (SEC-39)")
}

// SEC-45: nenhum -p/--publish do Docker sem IP (nunca 0.0.0.0); no
// mariadb-docker.sh, uma única publicação, só com --dev-ip.
func TestSEC45_DockerPublishSempreComIP(t *testing.T) {
	rePub := regexp.MustCompile(`(?:^|[\s(])(?:-p|--publish)(?:=|\s+)"?([^\s")]+)`)
	alvos := map[string]string{"Jenkinsfile": lerRaiz(t, "Jenkinsfile")}
	for nome, texto := range arquivosDeploy(t) {
		if strings.HasSuffix(nome, ".sh") || strings.HasSuffix(nome, ".ps1") {
			alvos["deploy/"+nome] = texto
		}
	}
	for nome, texto := range alvos {
		// (-P maiúsculo é a porta do cliente mariadb; só --publish-all importa)
		assert.False(t, regexp.MustCompile(`(^|\s)--publish-all(\s|=|$)`).MatchString(texto), "%s usa --publish-all", nome)
		for _, m := range rePub.FindAllStringSubmatch(texto, -1) {
			v := m[1]
			if !strings.Contains(v, ":") {
				continue // mkdir -p, mariadb -p, ...
			}
			partes := strings.Split(v, ":")
			assert.Len(t, partes, 3, "%s: -p %s precisa de IP:porta:porta", nome, v)
			assert.NotContains(t, []string{"", "0.0.0.0", "*"}, partes[0], "%s: -p %s", nome, v)
		}
	}
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	pubs := rePub.FindAllStringSubmatch(md, -1)
	var mapeamentos []string
	for _, m := range pubs {
		if strings.Contains(m[1], ":") {
			mapeamentos = append(mapeamentos, m[1])
		}
	}
	assert.Equal(t, []string{"$SITE_IP:$PORTA:3306"}, mapeamentos)
	assert.Contains(t, md, `$PUBLICAR && RUN_ARGS+=(-p "$SITE_IP:$PORTA:3306")`, "só publica com --dev-ip")
}

var reImagemMariaDB = regexp.MustCompile(`^mariadb:11\.4\.\d+@sha256:[0-9a-f]{64}$`)

// SEC-31: a imagem base do script e o cliente do Jenkins são a mesma string,
// fixada por digest.
func TestSEC31_ImagemBaseIgualDbClientImage(t *testing.T) {
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	jf := lerRaiz(t, "Jenkinsfile")
	mBase := regexp.MustCompile(`(?m)^IMAGEM_BASE="([^"]+)"$`).FindStringSubmatch(md)
	require.NotNil(t, mBase, "IMAGEM_BASE no mariadb-docker.sh")
	mCli := regexp.MustCompile(`(?m)^\s*DB_CLIENT_IMAGE\s*=\s*'([^']+)'`).FindStringSubmatch(jf)
	require.NotNil(t, mCli, "DB_CLIENT_IMAGE no Jenkinsfile")
	assert.Regexp(t, reImagemMariaDB, mBase[1])
	assert.Regexp(t, reImagemMariaDB, mCli[1])
	assert.Equal(t, mBase[1], mCli[1], "trocar nos dois (comentário do script, SEC-31)")
	mGo := regexp.MustCompile(`(?m)^\s*GO_IMAGE\s*=\s*'([^']+)'`).FindStringSubmatch(jf)
	require.NotNil(t, mGo)
	assert.Regexp(t, `^golang:1\.\d+\.\d+-alpine@sha256:[0-9a-f]{64}$`, mGo[1])
	assert.NotRegexp(t, `(?m)^\s*FROM\s+mariadb:`, md, "FROM só via ARG IMAGEM_BASE")
	assert.Contains(t, md, "FROM ${IMAGEM_BASE}")
	assert.Contains(t, md, `--build-arg "IMAGEM_BASE=$IMAGEM_BASE"`)
}

func TestSEC45_SanSemHostDockerInternal(t *testing.T) {
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	m := regexp.MustCompile(`(?m)^SAN="([^"]+)"$`).FindStringSubmatch(md)
	require.NotNil(t, m)
	assert.Equal(t, "IP:$SITE_IP, DNS:$CONTAINER, DNS:localhost, IP:127.0.0.1, DNS:$SITE_HOST", m[1])
	var codigo []string
	for _, l := range strings.Split(md, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "#") {
			codigo = append(codigo, l)
		}
	}
	for _, s := range []string{"host.docker.internal", "DOCKER_GW", "0.0.0.0:"} {
		assert.NotContains(t, strings.Join(codigo, "\n"), s, "fora de comentário")
	}
	assert.NotRegexp(t, `(?m)^[^#]*-p "?0\.0\.0\.0`, md)
}

func TestSEC45_FingerprintVersionadoConfereComCA(t *testing.T) {
	precisaOpenSSL(t)
	fps := linhasUteis(lerRaiz(t, "deploy", "db-ca.fingerprint"))
	require.NotEmpty(t, fps)
	assert.LessOrEqual(t, len(fps), 2, "até dois na rotação")
	for _, fp := range fps {
		assert.Regexp(t, `^[0-9a-f]{64}$`, fp)
	}
	ca := caminhoRaiz(t, "deploy", "ca-cert.pem")
	if _, err := os.Stat(ca); err != nil {
		t.Skip("deploy/ca-cert.pem ausente nesta máquina")
	}
	assert.Contains(t, fps, fingerprint(t, ca), "deploy/ca-cert.pem precisa estar no deploy/db-ca.fingerprint")
}

func linhasUteis(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
	}
	return out
}

// Opções e validações que falham ANTES de qualquer efeito (sem root).
func TestSEC45_MariadbDocker_OpcoesRemovidasEInvalidas(t *testing.T) {
	sc := mariadbDocker(t)
	casos := []struct {
		nome string
		args []string
		rc   int
		msg  string
	}{
		{"--versao saiu", []string{"--versao", "11.4"}, 1, "--versao saiu (SEC-31): use --imagem-base"},
		{"--rede-lan saiu", []string{"--rede-lan", "192.168.168.0/24"}, 1, "--rede-lan saiu (SEC-45): use --dev-ip"},
		{"--rede-lan depois de outra opcao", []string{"--dev-ip", "192.168.168.50", "--rede-lan", "x"}, 1, "--rede-lan saiu"},
		{"opcao desconhecida", []string{"--skip-mysql"}, 2, "Opcao desconhecida: --skip-mysql"},
		{"--dev-ip sem valor", []string{"--dev-ip"}, 1, "a opcao --dev-ip exige um valor"},
		{"--ca-key vazia", []string{"--ca-key", ""}, 1, "a opcao --ca-key exige um valor"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			cmd := exec.Command("bash", append([]string{sc}, c.args...)...)
			cmd.Env = ambienteLimpo()
			out, err := cmd.CombinedOutput()
			require.Error(t, err, "%s", out)
			var ee *exec.ExitError
			require.ErrorAs(t, err, &ee)
			assert.Equal(t, c.rc, ee.ExitCode(), "%s", out)
			assert.Contains(t, string(out), c.msg)
			assert.NotContains(t, string(out), "=== [1/", "falha antes da primeira etapa")
		})
	}
	t.Run("--help", func(t *testing.T) {
		cmd := exec.Command("bash", sc, "--help")
		cmd.Env = ambienteLimpo()
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
		for _, s := range []string{"--dev-ip", "--ca-key", "--nova-ca", "--imagem-base", "--rede-docker"} {
			assert.Contains(t, string(out), s)
		}
		assert.NotContains(t, string(out), "--rede-lan")
		assert.NotContains(t, string(out), "--versao")
	})
	t.Run("sem root", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("rodando como root")
		}
		cmd := exec.Command("bash", sc)
		cmd.Env = ambienteLimpo()
		out, err := cmd.CombinedOutput()
		require.Error(t, err)
		assert.Contains(t, string(out), "rode como root")
	})
}

// ---------------------------------------------------------------------------
// Ensaio com stubs
// ---------------------------------------------------------------------------

type ensaio struct {
	t   *testing.T
	dir string
}

const (
	siteIP    = "192.168.168.106"
	saltoFW   = "-A DOCKER-USER -p tcp -m conntrack --ctorigdst 192.168.168.106/32 --ctorigdstport 3306 -j ROTAPERFUMES-DB"
	segredos  = "root/rotaperfumes-mariadb"
	hostTeste = "srv-teste"
)

func precisaUnshare(t *testing.T) {
	t.Helper()
	precisaBash(t)
	precisaOpenSSL(t)
	for _, d := range []string{"/opt", "/usr/local/sbin", "/etc/systemd/system", "/usr/bin/install"} {
		if _, err := os.Stat(d); err != nil {
			t.Skip(d + " não existe nesta máquina")
		}
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("unshare não está no PATH")
	}
	d := t.TempDir()
	if out, err := exec.Command("unshare", "-rm", "mount", "--bind", d, "/opt").CombinedOutput(); err != nil {
		t.Skipf("user namespace indisponível (unshare -rm): %v %s", err, out)
	}
}

func novoEnsaio(t *testing.T) *ensaio {
	t.Helper()
	precisaUnshare(t)
	e := &ensaio{t: t, dir: t.TempDir()}
	for _, d := range []string{"root", "opt", "sbin", "systemd", "state", "bin"} {
		require.NoError(t, os.MkdirAll(filepath.Join(e.dir, d), 0o755))
	}
	bins := map[string]string{
		"docker": stubDocker, "iptables": stubIptables, "iptables-restore": stubIptablesRestore,
		"systemctl": stubSystemctl, "install": stubInstall, "chown": stubChown, "getent": stubGetent,
		"ss": stubSs, "timeout": stubTimeout,
	}
	for nome, conteudo := range bins {
		escrever(t, filepath.Join(e.dir, "bin", nome), conteudo, 0o755)
	}
	escrever(t, filepath.Join(e.dir, "runner.sh"), runnerEnsaio, 0o755)
	e.setState("chain_DOCKER-USER", "-A DOCKER-USER -j RETURN\n")
	return e
}

func (e *ensaio) setState(nome, conteudo string) {
	escrever(e.t, filepath.Join(e.dir, "state", nome), conteudo, 0o644)
}

func (e *ensaio) state(nome string) string {
	b, err := os.ReadFile(filepath.Join(e.dir, "state", nome))
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(e.t, err)
	return string(b)
}

func (e *ensaio) arq(rel string) string {
	b, err := os.ReadFile(filepath.Join(e.dir, rel))
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(e.t, err)
	return string(b)
}

func (e *ensaio) existe(rel string) bool {
	_, err := os.Stat(filepath.Join(e.dir, rel))
	return err == nil
}

func (e *ensaio) log() string { return e.arq("log") }

type saidaEnsaio struct {
	out string
	err error
}

// rodar executa o script real; --ssl-dir/--dados/--host-nome apontam para o
// TempDir (o resto vem dos binds do runner).
func (e *ensaio) rodar(args ...string) saidaEnsaio {
	e.t.Helper()
	_ = os.Remove(filepath.Join(e.dir, "log"))
	base := []string{"-rm", "bash", filepath.Join(e.dir, "runner.sh"),
		"--ssl-dir", filepath.Join(e.dir, "ssl"), "--dados", filepath.Join(e.dir, "dados"), "--host-nome", hostTeste}
	cmd := exec.Command("unshare", append(base, args...)...)
	cmd.Env = append(ambienteLimpo(), "FAKE="+e.dir, "SCRIPT="+mariadbDocker(e.t))
	out, err := cmd.CombinedOutput()
	return saidaEnsaio{out: string(out), err: err}
}

func (e *ensaio) senhas() map[string]string {
	out := map[string]string{}
	for _, u := range []string{"root", "rotaperfumes_app", "rotaperfumes_admin", "casaos"} {
		out[u] = strings.TrimSpace(e.arq(filepath.Join(segredos, u+".senha")))
	}
	return out
}

// gerarCA cria ca.pem/ca-key.pem (RSA 2048) em dir.
func gerarCA(t *testing.T, dir, cn string) (ca, key string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	ca, key = filepath.Join(dir, "ca.pem"), filepath.Join(dir, "ca-key.pem")
	out, err := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", key,
		"-out", ca, "-days", "3650", "-subj", "/CN="+cn,
		"-addext", "basicConstraints=critical,CA:TRUE", "-addext", "keyUsage=critical,keyCertSign,cRLSign").CombinedOutput()
	require.NoError(t, err, "%s", out)
	return ca, key
}

func fingerprint(t *testing.T, cert string) string {
	t.Helper()
	out, err := exec.Command("openssl", "x509", "-in", cert, "-noout", "-fingerprint", "-sha256").Output()
	require.NoError(t, err)
	v := strings.TrimSpace(strings.SplitN(string(out), "=", 2)[1])
	return strings.ToLower(strings.ReplaceAll(v, ":", ""))
}

func sanDoCert(t *testing.T, cert string) []string {
	t.Helper()
	out, err := exec.Command("openssl", "x509", "-in", cert, "-noout", "-ext", "subjectAltName").Output()
	require.NoError(t, err)
	var san []string
	for _, l := range linhas(string(out))[1:] {
		for _, p := range strings.Split(l, ",") {
			p = strings.TrimSpace(p)
			p = strings.Replace(p, "IP Address:", "IP:", 1)
			if p != "" {
				san = append(san, p)
			}
		}
	}
	sort.Strings(san)
	return san
}

func contaLinhas(s, sub string) int {
	n := 0
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			n++
		}
	}
	return n
}

// Primeira instalação, sem --dev-ip: rede, container sem porta, contas por
// IP fixo, CA nova com SAN exato, nenhum firewall.
func TestTST15_Ensaio_PrimeiraInstalacaoSemDevIP(t *testing.T) {
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)
	log := e.log()

	// Rede dedicada com a subnet e a faixa dinâmica fora dos IPs fixos.
	assert.Contains(t, log, "docker network create --driver bridge --subnet 172.30.42.0/24 --gateway 172.30.42.1 --ip-range 172.30.42.128/25 --label rotaperfumes.rede=db rotaperfumes-db")
	assert.Contains(t, log, "docker build --pull -q --build-arg IMAGEM_BASE=mariadb:11.4.")

	// Container: só na rede, IP fixo, sem porta publicada.
	run := e.state("run_args")
	require.NotEmpty(t, run, r.out)
	for _, s := range []string{"--network rotaperfumes-db --ip 172.30.42.2", "--user 999:999", "--cap-drop ALL",
		"--security-opt no-new-privileges:true", "--read-only", "MARIADB_ROOT_HOST=localhost", "-e MARIADB_ROOT_PASSWORD_FILE=/run/rotaperfumes/root.senha"} {
		assert.Contains(t, run, s)
	}
	assert.NotRegexp(t, `(^|\s)-p\s`, run, "sem --dev-ip nenhuma porta publicada")
	assert.Contains(t, r.out, "sem --dev-ip: nenhuma porta publicada")

	// Sem firewall nem unit.
	assert.NotContains(t, log, "iptables")
	assert.NotContains(t, log, "systemctl")
	assert.False(t, e.existe("sbin/rotaperfumes-db-firewall"))
	assert.False(t, e.existe("systemd/rotaperfumes-db-firewall.service"))

	// Contas: app e admin por IP fixo, ed25519 + REQUIRE SSL; sem casaos.
	sql := e.state("sql")
	s := e.senhas()
	assert.Contains(t, sql, "CREATE USER IF NOT EXISTS 'rotaperfumes_app'@'172.30.42.10' IDENTIFIED VIA ed25519 USING PASSWORD('"+s["rotaperfumes_app"]+"') REQUIRE SSL;")
	assert.Contains(t, sql, "GRANT SELECT, INSERT, UPDATE, DELETE ON `rotaperfumes`.* TO 'rotaperfumes_app'@'172.30.42.10';")
	assert.Contains(t, sql, "CREATE USER IF NOT EXISTS 'rotaperfumes_admin'@'172.30.42.20' IDENTIFIED VIA ed25519 USING PASSWORD('"+s["rotaperfumes_admin"]+"') REQUIRE SSL;")
	assert.Contains(t, sql, "GRANT ALL PRIVILEGES ON `rotaperfumes`.* TO 'rotaperfumes_admin'@'172.30.42.20';")
	assert.NotContains(t, sql, "'casaos'@")
	assert.NotContains(t, sql, "FLUSH SSL", "container novo não precisa de FLUSH SSL")
	assert.NotContains(t, sql, "172.16.0.0")

	// Senhas: 32 alfanuméricos, 600, nunca na tela nem no argv do docker.
	for u, v := range s {
		assert.Regexp(t, `^[A-Za-z0-9]{32}$`, v, u)
		st, err := os.Stat(filepath.Join(e.dir, segredos, u+".senha"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm(), u)
		assert.NotContains(t, r.out, v, "senha de %s na saída", u)
		assert.NotContains(t, log, v, "senha de %s no argv do docker", u)
	}
	assert.Contains(t, e.arq(filepath.Join(segredos, "container", "root.cnf")), "password="+s["root"])

	// CA nova + certificado com SAN exato (SEC-45).
	ca := filepath.Join(e.dir, "ssl", "ca.pem")
	cert := filepath.Join(e.dir, "ssl", "server-cert.pem")
	out, err := exec.Command("openssl", "verify", "-CAfile", ca, cert).CombinedOutput()
	require.NoError(t, err, "%s", out)
	assert.Equal(t, []string{"DNS:localhost", "DNS:rotaperfumes-mariadb", "DNS:" + hostTeste, "IP:127.0.0.1", "IP:" + siteIP}, sanDoCert(t, cert))
	subj, _ := exec.Command("openssl", "x509", "-in", ca, "-noout", "-subject").Output()
	assert.Contains(t, string(subj), "RotaPerfumes_MariaDB_CA")
	assert.Contains(t, r.out, fingerprint(t, ca), "fingerprint no formato do deploy/db-ca.fingerprint")
	assert.True(t, e.existe("root/rotaperfumes-ca/ca-key.pem"))
	assert.Contains(t, r.out, "a chave da CA esta NO SERVIDOR")
	assert.Contains(t, r.out, "CA NOVA: atualize deploy/db-ca.fingerprint")
	assert.Contains(t, r.out, "Conexao sem TLS recusada: OK.")
	assert.NotContains(t, log, "timeout ", "sem --dev-ip não há s_client na porta publicada")
	assert.NotContains(t, r.out, "BEGIN PRIVATE KEY")
}

// --dev-ip: chain com RETURN por IP + DROP, salto em DOCKER-USER (salto
// antigo removido), unit systemd, porta só no IP da LAN, casaos por IP.
func TestTST15_Ensaio_DevIPFirewallEContas(t *testing.T) {
	e := novoEnsaio(t)
	// Salto antigo (outra porta) de uma execução anterior.
	e.setState("chain_DOCKER-USER", "-A DOCKER-USER -p tcp -m conntrack --ctorigdst 192.168.168.106/32 --ctorigdstport 3307 -j ROTAPERFUMES-DB\n-A DOCKER-USER -j RETURN\n")
	r := e.rodar("--nova-ca", "--dev-ip", "192.168.168.50", "--dev-ip", "192.168.168.51", "--dev-ip", "192.168.168.50")
	require.NoError(t, r.err, r.out)

	assert.Equal(t, "-A ROTAPERFUMES-DB -s 192.168.168.50/32 -j RETURN\n-A ROTAPERFUMES-DB -s 192.168.168.51/32 -j RETURN\n-A ROTAPERFUMES-DB -j DROP\n",
		e.state("chain_ROTAPERFUMES-DB"), "IPs repetidos entram uma vez; DROP no fim")
	assert.Equal(t, saltoFW+"\n-A DOCKER-USER -j RETURN\n", e.state("chain_DOCKER-USER"), "salto novo no topo; antigo (3307) removido")

	fw := e.arq("sbin/rotaperfumes-db-firewall")
	assert.Contains(t, fw, "DEV_IPS=(192.168.168.50 192.168.168.51)")
	st, err := os.Stat(filepath.Join(e.dir, "sbin", "rotaperfumes-db-firewall"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), st.Mode().Perm())
	unit := e.arq("systemd/rotaperfumes-db-firewall.service")
	for _, s := range []string{"After=docker.service", "PartOf=docker.service", "WantedBy=multi-user.target", "ExecStart=/usr/local/sbin/rotaperfumes-db-firewall", "Type=oneshot"} {
		assert.Contains(t, unit, s)
	}
	log := e.log()
	assert.Contains(t, log, "systemctl enable rotaperfumes-db-firewall.service")
	iFW := strings.Index(log, "systemctl restart rotaperfumes-db-firewall.service")
	iRun := strings.Index(log, "docker run -d ")
	require.GreaterOrEqual(t, iFW, 0)
	require.GreaterOrEqual(t, iRun, 0)
	assert.Less(t, iFW, iRun, "firewall ativo ANTES de publicar a porta")

	assert.Contains(t, e.state("run_args"), "-p "+siteIP+":3306:3306")
	assert.Contains(t, log, "timeout 15 openssl s_client -starttls mysql -connect "+siteIP+":3306")
	assert.Contains(t, log, "-verify_ip "+siteIP)

	sql := e.state("sql")
	s := e.senhas()
	for _, ip := range []string{"192.168.168.50", "192.168.168.51"} {
		assert.Contains(t, sql, "CREATE USER IF NOT EXISTS 'casaos'@'"+ip+"' IDENTIFIED VIA ed25519 USING PASSWORD('"+s["casaos"]+"') REQUIRE SSL;")
		assert.Contains(t, sql, "GRANT ALL PRIVILEGES ON `rotaperfumes`.* TO 'casaos'@'"+ip+"';")
		assert.Contains(t, sql, "GRANT ALL PRIVILEGES ON `rotaperfumes\\_%`.* TO 'casaos'@'"+ip+"';")
	}
	assert.Equal(t, 1, strings.Count(sql, "CREATE USER IF NOT EXISTS 'casaos'@'192.168.168.50'"))
	assert.NotContains(t, sql, "mysql_native_password")

	// Reexecução: salto não duplica, chain igual, senhas iguais.
	antes := e.senhas()
	r2 := e.rodar("--dev-ip", "192.168.168.50", "--dev-ip", "192.168.168.51")
	require.NoError(t, r2.err, r2.out)
	assert.Equal(t, 1, contaLinhas(e.state("chain_DOCKER-USER"), "-j ROTAPERFUMES-DB"))
	assert.Equal(t, 3, len(linhas(e.state("chain_ROTAPERFUMES-DB"))))
	assert.Equal(t, antes, e.senhas(), "reexecução não troca senha")
	assert.Contains(t, r2.out, "Container ja em execucao.")
}

// Convergência (SEC-45 item 2): contas do projeto com host fora do desejado
// saem; host fora do padrão só gera aviso; contas estranhas geram aviso.
func TestTST15_Ensaio_ConvergenciaDeContas(t *testing.T) {
	casos := []struct {
		nome      string
		args      []string
		removidas []string
		mantidas  []string
	}{
		{"com --dev-ip", []string{"--dev-ip", "192.168.168.50"},
			[]string{"'casaos'@'192.168.168.0/255.255.255.0'", "'rotaperfumes_app'@'172.16.0.0/255.240.0.0'", "'rotaperfumes_admin'@'%'"},
			[]string{"'casaos'@'192.168.168.50'", "'rotaperfumes_app'@'172.30.42.10'", "'rotaperfumes_admin'@'172.30.42.20'"}},
		{"sem --dev-ip remove casaos", nil,
			[]string{"'casaos'@'192.168.168.0/255.255.255.0'", "'casaos'@'192.168.168.50'", "'rotaperfumes_app'@'172.16.0.0/255.240.0.0'", "'rotaperfumes_admin'@'%'"},
			[]string{"'rotaperfumes_app'@'172.30.42.10'", "'rotaperfumes_admin'@'172.30.42.20'"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaio(t)
			e.setState("contas", strings.Join([]string{
				"casaos\t192.168.168.0/255.255.255.0", "casaos\t192.168.168.50", "casaos\tx'y",
				"rotaperfumes_admin\t%", "rotaperfumes_admin\t172.30.42.20",
				"rotaperfumes_app\t172.16.0.0/255.240.0.0", "rotaperfumes_app\t172.30.42.10",
			}, "\n")+"\n")
			e.setState("outras", "intruso@%\n")
			r := e.rodar(append([]string{"--nova-ca"}, c.args...)...)
			require.NoError(t, r.err, r.out)
			sql := e.state("sql")
			for _, u := range c.removidas {
				assert.Contains(t, sql, "DROP USER IF EXISTS "+u+";")
			}
			for _, u := range c.mantidas {
				assert.NotContains(t, sql, "DROP USER IF EXISTS "+u+";")
			}
			assert.NotContains(t, sql, "x'y", "host fora do padrão não vai para o SQL")
			assert.Contains(t, r.out, "conta casaos com host fora do padrao NAO removida")
			assert.Contains(t, r.out, "contas fora do esperado (root@localhost, mariadb.sys e as do projeto): intruso@%")
			iCreate := strings.Index(sql, "CREATE USER")
			iDrop := strings.Index(sql, "DROP USER")
			assert.Less(t, iCreate, iDrop, "cria as desejadas antes de remover as antigas")
		})
	}
}

// CA (SEC-45 item 3) e certificado.
func TestTST15_Ensaio_CA(t *testing.T) {
	t.Run("CA existente sem chave aborta", func(t *testing.T) {
		e := novoEnsaio(t)
		ca, _ := gerarCA(t, filepath.Join(e.dir, "offline"), "RotaPerfumes_MariaDB_CA")
		escrever(t, filepath.Join(e.dir, "ssl", "ca.pem"), mustRead(t, ca), 0o644)
		r := e.rodar()
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "informe --ca-key /caminho/ca-key.pem")
		assert.Empty(t, e.state("run_args"), "nada sobe")
		assert.False(t, e.existe("ssl/server-cert.pem"))
	})
	t.Run("sem CA e sem --nova-ca aborta", func(t *testing.T) {
		e := novoEnsaio(t)
		r := e.rodar()
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "Primeira instalacao: --nova-ca")
		assert.Empty(t, e.state("run_args"))
	})
	t.Run("--ca-key de outra CA aborta", func(t *testing.T) {
		e := novoEnsaio(t)
		ca, _ := gerarCA(t, filepath.Join(e.dir, "offline"), "RotaPerfumes_MariaDB_CA")
		_, outraKey := gerarCA(t, filepath.Join(e.dir, "outra"), "Outra")
		escrever(t, filepath.Join(e.dir, "ssl", "ca.pem"), mustRead(t, ca), 0o644)
		r := e.rodar("--ca-key", outraKey)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "nao e a da CA")
	})
	t.Run("--ca-key offline emite sem copiar a chave", func(t *testing.T) {
		e := novoEnsaio(t)
		ca, key := gerarCA(t, filepath.Join(e.dir, "offline"), "RotaPerfumes_MariaDB_CA")
		escrever(t, filepath.Join(e.dir, "ssl", "ca.pem"), mustRead(t, ca), 0o644)
		r := e.rodar("--ca-key", key)
		require.NoError(t, r.err, r.out)
		out, err := exec.Command("openssl", "verify", "-CAfile", ca, filepath.Join(e.dir, "ssl", "server-cert.pem")).CombinedOutput()
		require.NoError(t, err, "%s", out)
		assert.Equal(t, mustRead(t, ca), e.arq("ssl/ca.pem"), "CA mantida")
		assert.False(t, e.existe("root/rotaperfumes-ca/ca-key.pem"), "chave lida de lá, não copiada")
		assert.NotContains(t, r.out, "a chave da CA esta NO SERVIDOR")
		assert.NotContains(t, r.out, "CA NOVA")
		st, err := os.Stat(filepath.Join(e.dir, "ssl", "server-key.pem"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	})
	t.Run("--nova-ca troca a CA com backup", func(t *testing.T) {
		e := novoEnsaio(t)
		ca, _ := gerarCA(t, filepath.Join(e.dir, "offline"), "Antiga")
		escrever(t, filepath.Join(e.dir, "ssl", "ca.pem"), mustRead(t, ca), 0o644)
		r := e.rodar("--nova-ca")
		require.NoError(t, r.err, r.out)
		assert.NotEqual(t, mustRead(t, ca), e.arq("ssl/ca.pem"))
		assert.Contains(t, r.out, "--nova-ca: a CA anterior foi trocada (backup em")
		bkp, _ := filepath.Glob(filepath.Join(e.dir, "ssl.bak-*", "ca.pem"))
		require.Len(t, bkp, 1)
		assert.Equal(t, mustRead(t, ca), mustRead(t, bkp[0]))
	})
	t.Run("reexecucao reaproveita; --novo-cert faz FLUSH SSL", func(t *testing.T) {
		e := novoEnsaio(t)
		require.NoError(t, e.rodar("--nova-ca").err)
		cert1 := e.arq("ssl/server-cert.pem")
		senhas := e.senhas()

		r := e.rodar()
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "Certificado existente valido (CA confere, SAN exato")
		assert.Equal(t, cert1, e.arq("ssl/server-cert.pem"))
		assert.Equal(t, senhas, e.senhas())
		assert.Contains(t, r.out, "Rede rotaperfumes-db ja existe (172.30.42.0/24).")
		assert.NotContains(t, e.log(), "docker network create")
		assert.NotContains(t, e.state("sql"), "FLUSH SSL")

		r = e.rodar("--novo-cert")
		require.NoError(t, r.err, r.out)
		assert.NotEqual(t, cert1, e.arq("ssl/server-cert.pem"))
		assert.Contains(t, e.state("sql"), "FLUSH SSL", "certificado novo com o container já rodando")
		assert.Equal(t, senhas, e.senhas())
	})
	t.Run("SAN diferente com outro host-nome reemite", func(t *testing.T) {
		e := novoEnsaio(t)
		r0 := e.rodar("--nova-ca")
		require.NoError(t, r0.err, r0.out)
		r := e.rodar("--host-nome", "outro-host")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "SAN diferente de")
		assert.Contains(t, sanDoCert(t, filepath.Join(e.dir, "ssl", "server-cert.pem")), "DNS:outro-host")
	})
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(b)
}

// Falhas fechadas e validações (como root no namespace).
func TestTST15_Ensaio_FalhasFechadas(t *testing.T) {
	casos := []struct {
		nome  string
		prep  func(e *ensaio)
		args  []string
		msg   string
		subiu bool
	}{
		{"--dev-ip sem DOCKER-USER", func(e *ensaio) { e.setState("sem_docker_user", "") }, []string{"--nova-ca", "--dev-ip", "192.168.168.50"}, "chain DOCKER-USER nao encontrada", false},
		{"--dev-ip com faixa", nil, []string{"--nova-ca", "--dev-ip", "192.168.168.0/24"}, "--dev-ip invalido", false},
		{"--dev-ip loopback", nil, []string{"--nova-ca", "--dev-ip", "127.0.0.1"}, "--dev-ip nao pode ser 127.0.0.1", false},
		{"--dev-ip com outro container", nil, []string{"--nova-ca", "--dev-ip", "192.168.168.50", "--container", "teste-db"}, "--dev-ip so com o container padrao", false},
		{"--imagem-base sem digest", nil, []string{"--nova-ca", "--imagem-base", "mariadb:11.4"}, "--imagem-base invalida", false},
		{"--imagem-base com digest maiusculo", nil, []string{"--nova-ca", "--imagem-base", "mariadb:11.4.13@sha256:" + strings.Repeat("A", 64)}, "--imagem-base invalida", false},
		{"--ip loopback", nil, []string{"--nova-ca", "--ip", "127.0.0.1"}, "--ip precisa ser o IP da LAN", false},
		{"--rede-docker /16", nil, []string{"--nova-ca", "--rede-docker", "172.30.0.0/16"}, "--rede-docker invalida", false},
		{"rede existente com outra subnet", func(e *ensaio) { e.setState("rede", "172.30.99.0/24\n") }, []string{"--nova-ca"}, "ja existe com outra subnet ('172.30.99.0/24', esperado 172.30.42.0/24)", false},
		{"porta ocupada por outro", func(e *ensaio) { e.setState("porta_ocupada", "") }, []string{"--nova-ca", "--dev-ip", "192.168.168.50"}, "porta 3306 ocupada por outro servico", false},
		{"conexao sem TLS aceita", func(e *ensaio) { e.setState("aceita_sem_tls", "") }, []string{"--nova-ca"}, "conexao SEM TLS foi aceita", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaio(t)
			if c.prep != nil {
				c.prep(e)
			}
			r := e.rodar(c.args...)
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, c.msg)
			assert.Equal(t, c.subiu, e.state("run_args") != "", "container criado? %s", r.out)
			if !c.subiu {
				assert.NotContains(t, e.log(), "iptables -w -I", "nenhuma regra mexida")
			}
		})
	}
}

// Container existente sem a porta e pedido de --dev-ip: só com --recriar.
func TestTST15_Ensaio_DivergenciaExigeRecriar(t *testing.T) {
	e := novoEnsaio(t)
	require.NoError(t, e.rodar("--nova-ca").err)
	r := e.rodar("--dev-ip", "192.168.168.50")
	require.Error(t, r.err, r.out)
	assert.Contains(t, r.out, "portas publicadas")
	assert.Contains(t, r.out, "rode de novo com --recriar")
	assert.NotContains(t, e.log(), "iptables-restore", "nada mudado antes do --recriar")

	r = e.rodar("--dev-ip", "192.168.168.50", "--recriar")
	require.NoError(t, r.err, r.out)
	assert.Contains(t, e.log(), "docker rm -f rotaperfumes-mariadb")
	assert.Contains(t, e.state("run_args"), "-p "+siteIP+":3306:3306")
}

// Sem --dev-ip, mas com a unit de uma execução anterior: chain recarregada só
// com DROP (fail closed); com --container de teste, nunca mexe no firewall.
func TestTST15_Ensaio_SemDevIPComUnitAnterior(t *testing.T) {
	e := novoEnsaio(t)
	require.NoError(t, e.rodar("--nova-ca", "--dev-ip", "192.168.168.50", "--recriar").err)
	require.True(t, e.existe("systemd/rotaperfumes-db-firewall.service"))

	r := e.rodar("--recriar")
	require.NoError(t, r.err, r.out)
	assert.Equal(t, "-A ROTAPERFUMES-DB -j DROP\n", e.state("chain_ROTAPERFUMES-DB"))
	assert.Contains(t, r.out, "chain ROTAPERFUMES-DB recarregada so com DROP")
	assert.NotRegexp(t, `(^|\s)-p\s`, e.state("run_args"))

	t.Run("container de teste nao toca o firewall", func(t *testing.T) {
		e2 := novoEnsaio(t)
		escrever(t, filepath.Join(e2.dir, "systemd", "rotaperfumes-db-firewall.service"), "[Unit]\n", 0o644)
		r := e2.rodar("--nova-ca", "--container", "teste-db", "--rede-nome", "teste-rede", "--rede-docker", "172.30.99.0/24")
		require.NoError(t, r.err, r.out)
		assert.NotContains(t, e2.log(), "iptables")
		assert.NotContains(t, e2.log(), "systemctl")
		assert.Contains(t, e2.state("run_args"), "--network teste-rede --ip 172.30.99.2")
		assert.Contains(t, sanDoCert(t, filepath.Join(e2.dir, "ssl", "server-cert.pem")), "DNS:teste-db")
		assert.Contains(t, e2.state("sql"), "'rotaperfumes_app'@'172.30.99.10'")
	})
}

// SEC-30: saída de emergência só para casaos, com motivo registrado.
func TestTST15_Ensaio_CasaosNativeMotivo(t *testing.T) {
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca", "--dev-ip", "192.168.168.50", "--casaos-native-motivo", "cliente do Windows sem ed25519")
	require.NoError(t, r.err, r.out)
	sql := e.state("sql")
	assert.Contains(t, sql, "ALTER USER 'casaos'@'192.168.168.50' IDENTIFIED VIA mysql_native_password USING PASSWORD('")
	assert.Equal(t, 1, strings.Count(sql, "mysql_native_password"), "só casaos")
	assert.Contains(t, e.arq(filepath.Join(segredos, "casaos-native.motivo")), "cliente do Windows sem ed25519")
	assert.Contains(t, r.out, "SEC-30: casaos com mysql_native_password")

	r = e.rodar("--dev-ip", "192.168.168.50", "--casaos-native-motivo", "curto")
	require.Error(t, r.err)
	assert.Contains(t, r.out, "precisa de 10 a 300 caracteres")
}

// ---------------------------------------------------------------------------
// Docker real (pendência: sem acesso ao socket nesta máquina)
// ---------------------------------------------------------------------------

// TestTST15_MariadbDocker_Real roda o script de verdade com container, rede,
// datadir e ssl-dir de teste, sem --dev-ip. Exige root (o script grava em
// /root/rotaperfumes-ca e /root/rotaperfumes-mariadb, caminhos fixos) e por
// isso só roda com TST15_DOCKER=1 e se esses diretórios NÃO existirem (não
// roda no servidor de produção). Ex.: sudo -E env TST15_DOCKER=1 go test
// -count=1 -run TestTST15_MariadbDocker_Real ./tests/deploy/
func TestTST15_MariadbDocker_Real(t *testing.T) {
	sc := mariadbDocker(t)
	if os.Getenv("TST15_DOCKER") != "1" {
		t.Skip("opt-in: TST15_DOCKER=1 (como root, com Docker; pendência do TST-15)")
	}
	if os.Geteuid() != 0 {
		t.Skip("precisa de root (caminhos fixos em /root)")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker inacessível: " + err.Error())
	}
	for _, d := range []string{"/root/rotaperfumes-ca", "/root/rotaperfumes-mariadb"} {
		if _, err := os.Stat(d); err == nil {
			t.Skip(d + " já existe (servidor real?): o teste não sobrescreve")
		}
	}
	criadoBuild := false
	if _, err := os.Stat("/opt/rotaperfumes/mariadb-imagem"); os.IsNotExist(err) {
		criadoBuild = true
	}
	const nome, rede, subnet = "tst15-mariadb", "tst15-db", "172.30.243.0/24"
	dir := t.TempDir()
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", nome).Run()
		_ = exec.Command("docker", "network", "rm", rede).Run()
		_ = os.RemoveAll("/root/rotaperfumes-ca")
		_ = os.RemoveAll("/root/rotaperfumes-mariadb")
		if criadoBuild {
			_ = os.RemoveAll("/opt/rotaperfumes/mariadb-imagem")
		}
		_ = exec.Command("chown", "-R", "0:0", dir).Run()
	})
	rodar := func(args ...string) (string, error) {
		base := []string{sc, "--container", nome, "--rede-nome", rede, "--rede-docker", subnet,
			"--dados", filepath.Join(dir, "dados"), "--ssl-dir", filepath.Join(dir, "ssl"), "--host-nome", "tst15"}
		cmd := exec.Command("bash", append(base, args...)...)
		cmd.Env = ambienteLimpo()
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	rootSQL := func(q string) string {
		out, err := exec.Command("docker", "exec", nome, "mariadb", "--defaults-extra-file=/run/rotaperfumes/root.cnf", "-N", "-B", "-e", q).CombinedOutput()
		require.NoError(t, err, "%s", out)
		return strings.TrimSpace(string(out))
	}

	out, err := rodar("--nova-ca")
	require.NoError(t, err, out)
	assert.Contains(t, out, "TLS com verificacao do certificado")
	assert.Contains(t, out, "Conexao sem TLS recusada: OK.")
	assert.Equal(t, "rotaperfumes_admin@172.30.243.20 ed25519\nrotaperfumes_app@172.30.243.10 ed25519",
		rootSQL("SELECT CONCAT(User,'@',Host,' ',JSON_VALUE(Priv,'$.plugin')) FROM mysql.global_priv WHERE User LIKE 'rotaperfumes%' OR User='casaos' ORDER BY User"))
	assert.Equal(t, "0", rootSQL("SELECT COUNT(*) FROM mysql.global_priv WHERE User='root' AND Host<>'localhost'"), "root sem host remoto")
	assert.Equal(t, "0", rootSQL("SELECT COUNT(*) FROM mysql.global_priv WHERE User LIKE 'rotaperfumes%' AND JSON_VALUE(Priv,'$.ssl_type')<>'1'"))
	portas, _ := exec.Command("docker", "inspect", "-f", "{{json .HostConfig.PortBindings}}", nome).Output()
	assert.Contains(t, []string{"{}", "null"}, strings.TrimSpace(string(portas)), "sem --dev-ip nenhuma porta")

	senha1, err := os.ReadFile("/root/rotaperfumes-mariadb/rotaperfumes_app.senha")
	require.NoError(t, err)
	out, err = rodar()
	require.NoError(t, err, out)
	senha2, _ := os.ReadFile("/root/rotaperfumes-mariadb/rotaperfumes_app.senha")
	assert.Equal(t, senha1, senha2, "reexecução não troca senha")
	assert.Contains(t, out, "Certificado existente valido")

	out, err = rodar("--novo-cert")
	require.NoError(t, err, out)
	assert.Contains(t, out, "Certificado novo recarregado (FLUSH SSL).")
}
