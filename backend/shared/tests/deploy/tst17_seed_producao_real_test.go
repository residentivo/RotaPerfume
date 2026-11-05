//go:build linux

package deploy_test

// TST-17 (Lote 36): deploy/seed-producao.sh contra um MariaDB REAL, no padrão
// do TestTST15_MariadbDocker_Real: container, rede e diretórios descartáveis
// (nomes tst17-*, TempDir), criados pelo deploy/mariadb-docker.sh com as
// opções do TST-16 (--segredos-dir/--ca-dir). Cobre: seed numa instalação
// nova (13 tabelas, admin "1 1 1" = ativo, troca obrigatória e Argon2id; 0
// não-admin ativos; senha do admin em arquivo 600), recusa da 2ª execução sem
// alterar nada e --forcar (nome do banco digitado num pty real do `script`),
// que troca a senha do admin e reinativa os não-admin.
//
// Pré-requisitos (senão skip, com o motivo):
//   - TST17_DOCKER=1, root (os dois scripts recusam outro EUID) e Docker;
//   - `script` do util-linux (o --forcar exige terminal);
//   - o seed-producao.sh com as opções de isolamento abaixo
//     (opcoesIsolamentoTST17). Hoje o script fixa o container
//     rotaperfumes-mariadb, a rede rotaperfumes-db, o IP 172.30.42.20,
//     /root/rotaperfumes-mariadb, /etc/mysql/ssl/ca.pem e o mktemp em /root:
//     sem elas o teste mexeria na instalação real e por isso é pulado
//     (pendência do TST-17, encaminhada ao 🔵 SubBrain).
// Ex.: sudo -E env TST17_DOCKER=1 go test -count=1 -run
// TestTST17_SeedProducao_Real ./tests/deploy/
//
// NUNCA no servidor de produção (Lote 36, ressalva do SecBrain): o teste
// divide com a produção o /opt/rotaperfumes/mariadb-imagem (contexto do
// build, removido no fim se o teste o criou) e a tag da imagem do MariaDB
// (rebuild/retag pelo mariadb-docker.sh). Por isso, com TST17_DOCKER=1, ele
// FALHA (não só pula) se o hostname for o do servidor de produção
// (hostProducaoTST17) ou se existir o container de produção
// (containerProducaoTST17). Rode-o numa máquina/VM descartável.
//
// Diretórios: a base é um diretório 700 do root criado em /root
// (os.MkdirTemp("/root", "tst17-")), e não o t.TempDir() em /tmp: o
// mariadb-docker.sh recusa caminho com symlink ou com ancestral gravável por
// outros (DataBrain, Lote 36). A base é apagada no fim.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	hostProducaoTST17      = "ivo-inspiron-15-3530"
	containerProducaoTST17 = "rotaperfumes-mariadb"
)

// recusaProducaoTST17 devolve o motivo para não rodar o TST-17 nesta
// máquina ("" = pode rodar).
func recusaProducaoTST17(hostname string, containerProducaoExiste bool) string {
	switch {
	case strings.EqualFold(strings.TrimSpace(hostname), hostProducaoTST17):
		return "hostname " + hostProducaoTST17 + " é o servidor de produção"
	case containerProducaoExiste:
		return "existe o container " + containerProducaoTST17 + " (instalação de produção nesta máquina)"
	}
	return ""
}

func TestTST17_RecusaProducao(t *testing.T) {
	casos := []struct {
		nome      string
		host      string
		container bool
		recusa    bool
	}{
		{"máquina descartável", "vm-teste", false, false},
		{"hostname de produção", hostProducaoTST17, false, true},
		{"hostname de produção em maiúsculas e com espaço", " IVO-Inspiron-15-3530\n", false, true},
		{"container de produção presente", "vm-teste", true, true},
		{"os dois", hostProducaoTST17, true, true},
		{"hostname parecido não é produção", "ivo-inspiron-15-3530-vm", false, false},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			m := recusaProducaoTST17(tc.host, tc.container)
			assert.Equal(t, tc.recusa, m != "", "motivo=%q", m)
		})
	}
}

// opcoesIsolamentoTST17: opções que o seed-producao.sh precisa aceitar para o
// teste real não tocar a instalação de produção. --ssl-dir e --segredos-dir
// com o mesmo significado do mariadb-docker.sh (CA em <ssl-dir>/ca.pem; senha
// do rotaperfumes_admin em <segredos-dir>/rotaperfumes_admin.senha).
var opcoesIsolamentoTST17 = []string{
	"--container", "--rede-nome", "--ip-admin", "--segredos-dir", "--ssl-dir", "--trabalho-dir",
}

func TestTST17_SeedProducao_Real(t *testing.T) {
	precisaBash(t)
	md := caminhoRaiz(t, "deploy", "mariadb-docker.sh")
	sp := caminhoRaiz(t, "deploy", "seed-producao.sh")
	if os.Getenv("TST17_DOCKER") != "1" {
		t.Skip("opt-in: TST17_DOCKER=1 (como root, com Docker; pendência do TST-17)")
	}
	if os.Geteuid() != 0 {
		t.Skip("precisa de root (os scripts recusam outro EUID)")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker inacessível: " + err.Error())
	}
	host, err := os.Hostname()
	require.NoError(t, err)
	existe := exec.Command("docker", "container", "inspect", containerProducaoTST17).Run() == nil
	if m := recusaProducaoTST17(host, existe); m != "" {
		t.Fatalf("TST-17 recusado: %s; o teste divide /opt/rotaperfumes/mariadb-imagem e a tag da imagem com a produção", m)
	}
	precisaScript(t)
	ajuda, err := exec.Command("bash", sp, "--help").CombinedOutput()
	require.NoError(t, err, "%s", ajuda)
	for _, o := range opcoesIsolamentoTST17 {
		if !strings.Contains(string(ajuda), o) {
			t.Skipf("pendente: seed-producao.sh sem a opção %s (isolamento do TST-17)", o)
		}
	}

	criadoBuild := false
	if _, err := os.Stat("/opt/rotaperfumes/mariadb-imagem"); os.IsNotExist(err) {
		criadoBuild = true
	}
	const nome, rede, subnet, ipAdmin = "tst17-mariadb", "tst17-db", "172.30.244.0/24", "172.30.244.20"
	dir, err := os.MkdirTemp("/root", "tst17-")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(dir, 0o700))
	seg, ca, ssl := filepath.Join(dir, "segredos"), filepath.Join(dir, "ca"), filepath.Join(dir, "ssl")
	trab := filepath.Join(dir, "trab")
	require.NoError(t, os.MkdirAll(trab, 0o700))
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", nome).Run()
		_ = exec.Command("docker", "network", "rm", rede).Run()
		if criadoBuild {
			_ = os.RemoveAll("/opt/rotaperfumes/mariadb-imagem")
		}
		_ = os.RemoveAll(dir)
	})

	out, err := exec.Command("bash", md, "--nova-ca", "--container", nome, "--rede-nome", rede,
		"--rede-docker", subnet, "--dados", filepath.Join(dir, "dados"), "--ssl-dir", ssl,
		"--segredos-dir", seg, "--ca-dir", ca, "--host-nome", "tst17").CombinedOutput()
	require.NoError(t, err, "mariadb-docker.sh: %s", out)

	pepper := filepath.Join(dir, "pepper.txt")
	escrever(t, pepper, strings.Repeat("p", 48)+"\n", 0o600)

	base := []string{sp, "--container", nome, "--rede-nome", rede, "--ip-admin", ipAdmin,
		"--segredos-dir", seg, "--ssl-dir", ssl, "--trabalho-dir", trab, "--pepper-file", pepper}
	seed := func(args ...string) (string, error) {
		cmd := exec.Command("bash", append(append([]string{}, base...), args...)...)
		cmd.Env = ambienteLimpo()
		o, err := cmd.CombinedOutput()
		return string(o), err
	}
	rootSQL := func(q string) string {
		o, err := exec.Command("docker", "exec", nome, "mariadb", "--defaults-extra-file=/run/rotaperfumes/root.cnf",
			"-N", "-B", "rotaperfumes", "-e", q).CombinedOutput()
		require.NoError(t, err, "%s", o)
		return strings.TrimSpace(string(o))
	}
	const qAdmin = "SELECT CONCAT(ativo,' ',deve_trocar_senha,' ',password_hash LIKE CONCAT(CHAR(36),'argon2id',CHAR(36),'%')) FROM usuarios WHERE email='admin@rotaperfumes.com.br'"
	const qHash = "SELECT password_hash FROM usuarios WHERE email='admin@rotaperfumes.com.br'"
	const qNaoAdminAtivos = "SELECT COUNT(*) FROM usuarios WHERE role <> 'admin' AND ativo <> 0"

	senha1 := filepath.Join(seg, "admin-app.senha")
	t.Run("instalação nova", func(t *testing.T) {
		out, err := seed("--admin-senha-arquivo", senha1)
		require.NoError(t, err, out)
		assert.Contains(t, out, "Admin ativo, Argon2id, troca obrigatoria no 1o login: OK.")
		assert.Equal(t, "13", rootSQL("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='rotaperfumes'"))
		assert.Equal(t, "1 1 1", rootSQL(qAdmin))
		assert.Equal(t, "0", rootSQL(qNaoAdminAtivos))
		assert.Equal(t, "42", rootSQL("SELECT COUNT(*) FROM usuarios WHERE role <> 'admin'"))
		st, err := os.Stat(senha1)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
		s, _ := os.ReadFile(senha1)
		assert.NotContains(t, out, strings.TrimSpace(string(s)), "senha do admin nunca na saída")
		ents, _ := os.ReadDir(trab)
		assert.Empty(t, ents, "área de trabalho removida no fim")
	})

	t.Run("2ª execução recusada sem alterar nada", func(t *testing.T) {
		antes := rootSQL(qHash)
		senha2 := filepath.Join(seg, "admin-app-2.senha")
		out, err := seed("--admin-senha-arquivo", senha2)
		require.Error(t, err, out)
		assert.Contains(t, out, "este banco ja foi semeado. Nada foi alterado.")
		assert.Equal(t, antes, rootSQL(qHash))
		assert.NoFileExists(t, senha2)
	})

	t.Run("--forcar com o nome do banco num terminal real", func(t *testing.T) {
		antes := rootSQL(qHash)
		rootSQL("UPDATE usuarios SET ativo = 1 WHERE role <> 'admin' LIMIT 1")
		senha3 := filepath.Join(seg, "admin-app-3.senha")
		linha := "bash"
		for _, a := range append(append([]string{}, base...), "--admin-senha-arquivo", senha3, "--forcar") {
			linha += " '" + a + "'"
		}
		cmd := exec.Command("script", "-qec", linha, "/dev/null")
		cmd.Env = ambienteLimpo()
		cmd.Stdin = strings.NewReader("rotaperfumes\n")
		o, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", o)
		assert.Contains(t, string(o), "Admin ativo, Argon2id, troca obrigatoria no 1o login: OK.")
		assert.NotEqual(t, antes, rootSQL(qHash), "--forcar troca a senha do admin")
		assert.Equal(t, "1 1 1", rootSQL(qAdmin))
		assert.Equal(t, "0", rootSQL(qNaoAdminAtivos), "--forcar reinativa os não-admin")
		assert.FileExists(t, senha3)
	})
}
