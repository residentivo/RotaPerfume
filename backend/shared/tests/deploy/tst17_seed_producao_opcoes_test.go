package deploy_test

// TST-17 item 1 (Lote 37, BackBrain): opções de isolamento do
// deploy/seed-producao.sh (--container, --rede-nome, --ip-admin,
// --segredos-dir, --ssl-dir, --trabalho-dir). Ensaio do DEPLOY-07 (docker e
// mktemp falsos, user namespace + chroot). Toda recusa acontece na etapa 1:
// o docker falso só vê info/inspect, nenhum docker run e nenhum mktemp.
// Cobertura básica; o 🔴 TestBrain completa.

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

// execBash roda o script sem privilégio (só parse e validação de sintaxe).
func execBash(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", args...)
	cmd.Env = ambienteLimpo()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var opcoesTST17 = []string{"--container", "--rede-nome", "--ip-admin", "--segredos-dir", "--ssl-dir", "--trabalho-dir"}

func TestTST17_SeedProducao_AjudaEValorObrigatorio(t *testing.T) {
	precisaBash(t)
	sc := caminhoRaiz(t, "deploy", "seed-producao.sh")
	out, err := execBash(t, sc, "--help")
	require.NoError(t, err, out)
	for _, o := range opcoesTST17 {
		assert.Contains(t, out, o)
	}
	for _, p := range []string{"rotaperfumes-mariadb", "rotaperfumes-db", "172.30.42.20", "/root/rotaperfumes-mariadb", "/etc/mysql/ssl", "(padrao /root;"} {
		assert.Contains(t, out, p, "padrão na ajuda")
	}
	for _, o := range opcoesTST17 {
		t.Run(o, func(t *testing.T) {
			out, err := execBash(t, sc, o)
			require.Error(t, err)
			assert.Contains(t, out, "a opcao "+o+" exige um valor")
			out, err = execBash(t, sc, o, "")
			require.Error(t, err)
			assert.Contains(t, out, "a opcao "+o+" exige um valor")
		})
	}
}

// Paridade: o conferir_dir_privado do seed é cópia byte a byte do
// mariadb-docker.sh.
func TestTST17_SeedProducao_ConferirDirPrivadoParidade(t *testing.T) {
	re := regexp.MustCompile(`(?ms)^conferir_dir_privado\(\) \{.*?^\}$`)
	md := re.FindString(lerRaiz(t, "deploy", "mariadb-docker.sh"))
	sp := re.FindString(lerRaiz(t, "deploy", "seed-producao.sh"))
	require.NotEmpty(t, md)
	assert.Equal(t, md, sp)
}

// recusaSemEfeito: falhou com a mensagem, nada além de info/inspect no
// docker e nenhum mktemp.
func (e *ensaioSeed) recusaSemEfeito(t *testing.T, out string, err error, msg string) {
	t.Helper()
	require.Error(t, err, out)
	assert.Contains(t, out, msg)
	assert.NotContains(t, out, "=== [2/7]")
	for _, c := range e.chamadas() {
		require.NotEmpty(t, c.argv)
		assert.Contains(t, []string{"info", "inspect", "network"}, c.argv[0], "só consultas: %v", c.argv)
	}
	assert.Empty(t, e.ler("mktemp.log"), "nenhum mktemp")
	assert.NotContains(t, out, pepperSeed)
	assert.NotContains(t, out, senhaDBSeed)
}

func TestTST17_SeedProducao_Ensaio_Recusas(t *testing.T) {
	type caso struct {
		nome string
		prep func(e *ensaioSeed) []string
		msg  string
	}
	fixo := func(args ...string) func(*ensaioSeed) []string {
		return func(*ensaioSeed) []string { return args }
	}
	// dirs cria em e.dir os diretórios do teste: seg 700 com a senha, ssl
	// 755 com ca.pem 644, trab 700.
	dirs := func(e *ensaioSeed) (seg, ssl, trab string) {
		seg, ssl, trab = filepath.Join(e.dir, "seg"), filepath.Join(e.dir, "ssl"), filepath.Join(e.dir, "trab")
		criarDirsTST17(t, seg, ssl, trab)
		return seg, ssl, trab
	}
	casos := []caso{
		{"container com maiúscula e _", fixo("--container", "Rota_X"), "--container invalido: Rota_X"},
		{"container começando com -", fixo("--container", "-x"), "--container invalido: -x"},
		{"container com 64 caracteres", fixo("--container", strings.Repeat("a", 64)), "--container invalido"},
		{"rede começando com -", fixo("--rede-nome", "-x"), "--rede-nome invalido: -x"},
		{"rede com espaço", fixo("--rede-nome", "a b"), "--rede-nome invalido: a b"},
		{"ip com zero à esquerda", fixo("--ip-admin", "172.30.42.020"), "--ip-admin invalido: 172.30.42.020"},
		{"ip com octeto 256", fixo("--ip-admin", "172.30.42.256"), "--ip-admin invalido"},
		{"ip curto", fixo("--ip-admin", "1.2.3"), "--ip-admin invalido"},
		{"ip .21 na subnet .0/24", fixo("--ip-admin", "172.30.42.21"), "so existe em 172.30.42.20"},
		{"subnet não /24", func(e *ensaioSeed) []string { e.state("subnets", "172.30.42.0/16 "); return nil }, "nao e A.B.C.0/24"},
		{"duas subnets IPv4", func(e *ensaioSeed) []string {
			e.state("subnets", "172.30.42.0/24 172.30.43.0/24 ")
			return nil
		}, "exatamente uma subnet IPv4 (tem 2"},
		{"container fora da rede", func(e *ensaioSeed) []string { e.state("redes", "bridge\nrotaperfumes-db2\n"); return nil },
			"nao esta ligado a rede rotaperfumes-db"},
		{"container parado", func(e *ensaioSeed) []string { e.state("running", "false\n"); return nil }, "nao esta rodando"},
		{"rede inexistente", func(e *ensaioSeed) []string { e.state("sem_rede", ""); return nil }, "a rede docker rotaperfumes-db nao existe"},
		{"segredos relativo", fixo("--segredos-dir", "seg"), "diretorio invalido: 'seg'"},
		{"ssl é /", fixo("--ssl-dir", "/"), "diretorio invalido: '/'"},
		{"ssl é //", fixo("--ssl-dir", "//"), "diretorio invalido: '//' (barras repetidas"},
		{"segredos com // no meio", fixo("--segredos-dir", "/root//rotaperfumes-mariadb"), "barras repetidas '//'"},
		{"trabalho com // no fim", fixo("--trabalho-dir", "/root//"), "diretorio invalido: '/root//' (barras repetidas"},
		{"trabalho com ..", fixo("--trabalho-dir", "/root/../tmp"), "diretorio invalido"},
		{"ssl com vírgula", fixo("--ssl-dir", "/root/a,target=/x"), "diretorio invalido: '/root/a,target=/x'"},
		{"segredos inexistente", func(e *ensaioSeed) []string {
			return []string{"--segredos-dir", filepath.Join(e.dir, "nao-existe")}
		}, "nao-existe nao existe"},
		{"segredos 755", func(e *ensaioSeed) []string {
			seg, _, _ := dirs(e)
			require.NoError(t, os.Chmod(seg, 0o755))
			return []string{"--segredos-dir", seg}
		}, "precisa ser do root com modo 700"},
		{"trabalho 755", func(e *ensaioSeed) []string {
			_, _, trab := dirs(e)
			require.NoError(t, os.Chmod(trab, 0o755))
			return []string{"--trabalho-dir", trab}
		}, "precisa ser do root com modo 700"},
		{"/root 755 (padrão do --trabalho-dir)", func(e *ensaioSeed) []string {
			require.NoError(t, os.Chmod(filepath.Join(e.dir, "root"), 0o755))
			return nil
		}, "--trabalho-dir /root precisa ser do root com modo 700"},
		{"symlink num componente", func(e *ensaioSeed) []string {
			seg, _, _ := dirs(e)
			require.NoError(t, os.Symlink(e.dir, filepath.Join(e.dir, "lnk")))
			return []string{"--segredos-dir", filepath.Join(e.dir, "lnk", filepath.Base(seg))}
		}, "nao e o caminho real"},
		{"ancestral 1777", func(e *ensaioSeed) []string {
			ab := filepath.Join(e.dir, "aberto")
			require.NoError(t, os.MkdirAll(filepath.Join(ab, "seg"), 0o700))
			require.NoError(t, os.Chmod(filepath.Join(ab, "seg"), 0o700))
			require.NoError(t, os.Chmod(ab, os.ModeSticky|0o777))
			escrever(t, filepath.Join(ab, "seg", "rotaperfumes_admin.senha"), senhaDBSeed+"\n", 0o600)
			return []string{"--segredos-dir", filepath.Join(ab, "seg")}
		}, "tem escrita para grupo/outros"},
		{"ssl com escrita de grupo", func(e *ensaioSeed) []string {
			_, ssl, _ := dirs(e)
			require.NoError(t, os.Chmod(ssl, 0o775))
			return []string{"--ssl-dir", ssl}
		}, "precisa ser do root sem escrita para grupo/outros"},
		{"ssl inexistente", func(e *ensaioSeed) []string {
			return []string{"--ssl-dir", filepath.Join(e.dir, "sem-ssl")}
		}, "sem-ssl nao existe ou nao e diretorio"},
		{"ca.pem symlink", func(e *ensaioSeed) []string {
			_, ssl, _ := dirs(e)
			ca := filepath.Join(ssl, "ca.pem")
			require.NoError(t, os.Rename(ca, filepath.Join(ssl, "real.pem")))
			require.NoError(t, os.Symlink("real.pem", ca))
			return []string{"--ssl-dir", ssl}
		}, "CA do banco nao encontrada"},
		{"ca.pem com escrita de grupo", func(e *ensaioSeed) []string {
			_, ssl, _ := dirs(e)
			require.NoError(t, os.Chmod(filepath.Join(ssl, "ca.pem"), 0o664))
			return []string{"--ssl-dir", ssl}
		}, "ca.pem precisa ser do root sem escrita para grupo/outros"},
		{"ca.pem ausente", func(e *ensaioSeed) []string {
			_, ssl, _ := dirs(e)
			require.NoError(t, os.Remove(filepath.Join(ssl, "ca.pem")))
			return []string{"--ssl-dir", ssl}
		}, "CA do banco nao encontrada em " + "/"},
		{"ssl dentro de segredos", func(e *ensaioSeed) []string {
			seg, _, _ := dirs(e)
			return []string{"--segredos-dir", seg, "--ssl-dir", filepath.Join(seg, "ssl")}
		}, "precisam ser disjuntos"},
		{"segredos dentro de ssl", func(e *ensaioSeed) []string {
			_, ssl, _ := dirs(e)
			return []string{"--ssl-dir", ssl, "--segredos-dir", filepath.Join(ssl, "seg")}
		}, "precisam ser disjuntos"},
		{"ssl igual a trabalho", func(e *ensaioSeed) []string {
			_, ssl, _ := dirs(e)
			return []string{"--ssl-dir", ssl, "--trabalho-dir", ssl}
		}, "--ssl-dir (" + "/"},
		{"trabalho igual a segredos", func(e *ensaioSeed) []string {
			seg, _, _ := dirs(e)
			return []string{"--segredos-dir", seg, "--trabalho-dir", seg}
		}, "nao pode ser --segredos-dir ("},
		{"trabalho dentro de segredos", func(e *ensaioSeed) []string {
			seg, _, _ := dirs(e)
			return []string{"--segredos-dir", seg, "--trabalho-dir", filepath.Join(seg, "t")}
		}, "nao pode ser --segredos-dir ("},
		{"ssl sobreposto a segredos por symlink", func(e *ensaioSeed) []string {
			seg, _, _ := dirs(e)
			require.NoError(t, os.Symlink(seg, filepath.Join(e.dir, "ssl-lnk")))
			return []string{"--segredos-dir", seg, "--ssl-dir", filepath.Join(e.dir, "ssl-lnk")}
		}, "precisam ser disjuntos"},
		{"senha do admin no ssl-dir", func(e *ensaioSeed) []string {
			_, ssl, _ := dirs(e)
			return []string{"--ssl-dir", ssl, "--admin-senha-arquivo", filepath.Join(ssl, "admin.senha")}
		}, "nao pode ficar no --ssl-dir"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioSeed(t)
			args := append([]string{"--pepper-file", "/root/pepper.txt"}, c.prep(e)...)
			out, err := e.rodar(args...)
			e.recusaSemEfeito(t, out, err, c.msg)
		})
	}
}

// criarDirsTST17: seg 700 com a senha do banco, ssl 755 com ca.pem 644 e
// trab 700 (modos fixados com Chmod: o umask não interfere).
func criarDirsTST17(t *testing.T, seg, ssl, trab string) {
	t.Helper()
	for d, m := range map[string]os.FileMode{seg: 0o700, ssl: 0o755, trab: 0o700} {
		require.NoError(t, os.MkdirAll(d, m))
		require.NoError(t, os.Chmod(d, m))
	}
	escrever(t, filepath.Join(seg, "rotaperfumes_admin.senha"), senhaDBSeed+"\n", 0o600)
	escrever(t, filepath.Join(ssl, "ca.pem"), "-----BEGIN CERTIFICATE-----\nx\n-----END CERTIFICATE-----\n", 0o644)
	require.NoError(t, os.Chmod(filepath.Join(ssl, "ca.pem"), 0o644))
}

// Sem opções: os padrões no argv do docker e o mktemp em /root.
func TestTST17_SeedProducao_Ensaio_PadraoNoArgv(t *testing.T) {
	e := novoEnsaioSeed(t)
	out, err := e.rodar("--pepper-file", "/root/pepper.txt")
	require.NoError(t, err, out)
	assert.Equal(t, "-d /root/rotaperfumes-seed.XXXXXX\n", e.ler("mktemp.log"))
	log := e.ler("log")
	assert.Contains(t, log, "inspect -f {{.State.Running}} rotaperfumes-mariadb")
	assert.Contains(t, log, "network inspect -f {{range .IPAM.Config}}{{.Subnet}} {{end}} rotaperfumes-db")
	assert.Contains(t, log, "--network rotaperfumes-db --ip 172.30.42.20")
	assert.Contains(t, log, "type=bind,source=/etc/mysql/ssl/ca.pem,target=/etc/mysql/ssl/ca.pem,readonly")
}

// Com as seis opções: argv, env-file, área de trabalho e senha do admin nos
// valores dados (nada em /root além do pepper do ensaio).
func TestTST17_SeedProducao_Ensaio_OpcoesNoArgv(t *testing.T) {
	e := novoEnsaioSeed(t)
	seg, ssl, trab := filepath.Join(e.dir, "seg"), filepath.Join(e.dir, "ssl"), filepath.Join(e.dir, "trab")
	criarDirsTST17(t, seg, ssl, trab)
	require.NoError(t, os.Remove(filepath.Join(e.dir, "root", "rotaperfumes-mariadb", "rotaperfumes_admin.senha")))
	e.state("redes", "bridge\ntst17-db\n")
	e.state("subnets", "172.30.244.0/24 fd00:17::/64 ")

	out, err := e.rodar("--pepper-file", "/root/pepper.txt", "--container", "tst17-mariadb", "--rede-nome", "tst17-db",
		"--ip-admin", "172.30.244.20", "--segredos-dir", seg+"/", "--ssl-dir", ssl, "--trabalho-dir", trab)
	require.NoError(t, err, out)
	assert.Contains(t, out, "=== [7/7] Conferencia ===")
	assert.Equal(t, "-d "+trab+"/rotaperfumes-seed.XXXXXX\n", e.ler("mktemp.log"))
	log := e.ler("log")
	assert.NotContains(t, log, "rotaperfumes-db ")
	assert.NotContains(t, log, "172.30.42.20")
	for _, c := range e.chamadas() {
		if !c.mariadb() && !c.tem("-create-admin") {
			continue
		}
		argv := strings.Join(c.argv, " ")
		assert.Contains(t, argv, "--network tst17-db --ip 172.30.244.20")
		assert.Contains(t, argv, "type=bind,source="+ssl+"/ca.pem,target=/etc/mysql/ssl/ca.pem,readonly")
		if c.mariadb() {
			assert.Contains(t, argv, "-h tst17-mariadb")
		}
	}
	env := e.ler("state/env_conteudo")
	assert.Contains(t, env, "DB_HOST=tst17-mariadb\n")
	assert.Contains(t, env, "DB_TLS_SERVER_NAME=tst17-mariadb\n")
	assert.True(t, strings.HasPrefix(e.ler("state/env_path"), trab+"/rotaperfumes-seed."))
	ents, err := os.ReadDir(trab)
	require.NoError(t, err)
	assert.Empty(t, ents, "área de trabalho removida")
	st, err := os.Stat(filepath.Join(seg, "admin-app.senha"))
	require.NoError(t, err, "senha do admin no --segredos-dir (padrão do --admin-senha-arquivo)")
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	assert.Contains(t, out, seg+"/admin-app.senha (600, so root)")
}
