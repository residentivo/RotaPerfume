package deploy_test

// TST-16 (Lote 36): --segredos-dir e --ca-dir do deploy/mariadb-docker.sh.
// Ensaio com os stubs do TST-15 (user namespace, EUID 0): com as opções, as
// senhas, o root.cnf, a chave da CA e o temporário dos certificados ficam nos
// diretórios dados, e nada é gravado no /root (bind para o TempDir). As
// validações (absoluto, sem "..", disjuntos pelo caminho real, sem symlink
// em nenhum componente, ancestrais do root sem escrita de grupo/outros, dono
// root e modo 700 quando já existe) falham na etapa 1, antes de qualquer
// efeito; o Lote 36 (TOCTOU) repete a checagem depois do install -d.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTST16_Ensaio_SegredosDirECaDir(t *testing.T) {
	e := novoEnsaio(t)
	seg, ca := filepath.Join(e.dir, "segredos"), filepath.Join(e.dir, "ca")
	r := e.rodar("--nova-ca", "--segredos-dir", seg, "--ca-dir", ca)
	require.NoError(t, r.err, r.out)

	for _, u := range []string{"root", "rotaperfumes_app", "rotaperfumes_admin"} {
		st, err := os.Stat(filepath.Join(seg, u+".senha"))
		require.NoError(t, err, u)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm(), u)
	}
	assert.FileExists(t, filepath.Join(seg, "container", "root.cnf"))
	assert.FileExists(t, filepath.Join(ca, "ca-key.pem"))
	assert.FileExists(t, filepath.Join(ca, "server-ext.cnf"))
	for _, d := range []string{seg, ca} {
		st, err := os.Stat(d)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), st.Mode().Perm(), d)
	}
	// Temporário dos certificados removido; nada no /root (TST-16).
	tmps, _ := filepath.Glob(filepath.Join(ca, ".ssl-tmp.*"))
	assert.Empty(t, tmps, "temporário removido no EXIT")
	noRoot, err := os.ReadDir(filepath.Join(e.dir, "root"))
	require.NoError(t, err)
	assert.Empty(t, noRoot, "com --segredos-dir/--ca-dir nada vai para /root")

	assert.Contains(t, e.state("run_args"), "-v "+seg+"/container:/run/rotaperfumes:ro")
	assert.Contains(t, r.out, seg+"/rotaperfumes_app.senha")
	assert.Contains(t, r.out, "a chave da CA esta NO SERVIDOR ("+ca+"/ca-key.pem)")

	// Reexecução com os mesmos diretórios (já 700): reaproveita senha e CA.
	antes := e.arq("segredos/rotaperfumes_app.senha")
	r = e.rodar("--segredos-dir", seg, "--ca-dir", ca)
	require.NoError(t, r.err, r.out)
	assert.Equal(t, antes, e.arq("segredos/rotaperfumes_app.senha"), "reexecução não troca senha")
	assert.Contains(t, r.out, "Certificado existente valido")
}

// Sem as opções o comportamento é o de antes: tudo em /root (bind do ensaio).
func TestTST16_Ensaio_PadraoContinuaEmRoot(t *testing.T) {
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)
	assert.True(t, e.existe(segredos+"/root.senha"))
	assert.True(t, e.existe("root/rotaperfumes-ca/ca-key.pem"))
	tmps, _ := filepath.Glob(filepath.Join(e.dir, "root", "rotaperfumes-ssl.*"))
	assert.Empty(t, tmps, "o temporário não fica mais solto em /root")
	tmps, _ = filepath.Glob(filepath.Join(e.dir, "root", "rotaperfumes-ca", ".ssl-tmp.*"))
	assert.Empty(t, tmps)
}

func TestTST16_Ensaio_ValidacaoDosDiretorios(t *testing.T) {
	e := novoEnsaio(t)
	aberto := filepath.Join(e.dir, "aberto")
	require.NoError(t, os.MkdirAll(aberto, 0o755))
	require.NoError(t, os.Chmod(aberto, 0o755))
	real := filepath.Join(e.dir, "real")
	require.NoError(t, os.MkdirAll(real, 0o700))
	link := filepath.Join(e.dir, "link")
	require.NoError(t, os.Symlink(real, link))
	arquivo := filepath.Join(e.dir, "arquivo")
	escrever(t, arquivo, "x", 0o600)
	ssl := filepath.Join(e.dir, "ssl")
	// Lote 36 (SecBrain): symlink num ancestral, ancestral 777/1777 (sticky
	// não basta), caminho não canônico e disjunção pelo caminho real.
	linkPai := filepath.Join(e.dir, "linkpai")
	require.NoError(t, os.Symlink(real, linkPai))
	aberto777 := filepath.Join(e.dir, "aberto777")
	require.NoError(t, os.MkdirAll(aberto777, 0o700))
	require.NoError(t, os.Chmod(aberto777, 0o777))
	sticky := filepath.Join(e.dir, "sticky")
	require.NoError(t, os.MkdirAll(sticky, 0o700))
	require.NoError(t, os.Chmod(sticky, 0o1777|os.ModeSticky))
	grupo := filepath.Join(e.dir, "grupo")
	require.NoError(t, os.MkdirAll(grupo, 0o700))
	require.NoError(t, os.Chmod(grupo, 0o770))
	sslReal := filepath.Join(e.dir, "sslreal")
	require.NoError(t, os.MkdirAll(sslReal, 0o755))
	sslLink := filepath.Join(e.dir, "ssllink")
	require.NoError(t, os.Symlink(sslReal, sslLink))
	dadosLink := filepath.Join(e.dir, "dadoslink")
	require.NoError(t, os.Symlink(real, dadosLink))

	casos := []struct {
		nome string
		args []string
		msg  string
	}{
		{"segredos relativo", []string{"--segredos-dir", "segredos"}, "diretorio invalido: 'segredos'"},
		{"ca relativo", []string{"--ca-dir", "./ca"}, "diretorio invalido: './ca'"},
		{"segredos com ..", []string{"--segredos-dir", e.dir + "/x/../segredos"}, "sem espacos/virgulas/.."},
		{"ca com espaco", []string{"--ca-dir", e.dir + "/c a"}, "diretorio invalido"},
		{"segredos raiz", []string{"--segredos-dir", "/"}, "diretorio invalido: '/'"},
		{"segredos existente 755", []string{"--segredos-dir", aberto}, "--segredos-dir " + aberto + " precisa ser do root com modo 700 (atual: dono 0, modo 755)"},
		{"ca existente 755", []string{"--ca-dir", aberto}, "--ca-dir " + aberto + " precisa ser do root com modo 700"},
		{"ca symlink", []string{"--ca-dir", link}, "--ca-dir " + link + " e um link simbolico"},
		{"segredos arquivo", []string{"--segredos-dir", arquivo}, "existe e nao e diretorio"},
		{"ca dentro do ssl-dir", []string{"--ca-dir", ssl + "/ca"}, "--ca-dir (" + ssl + "/ca) nao pode ficar dentro de --ssl-dir"},
		{"segredos igual a ca", []string{"--segredos-dir", real, "--ca-dir", real}, "nao pode ficar dentro de"},
		{"segredos dentro da ca", []string{"--ca-dir", real, "--segredos-dir", real + "/seg"}, "--segredos-dir (" + real + "/seg) nao pode ficar dentro de --ca-dir"},
		{"dados dentro dos segredos", []string{"--segredos-dir", real, "--dados", real + "/dados"}, "nao pode ficar dentro de"},
		{"barra final igual", []string{"--segredos-dir", real + "/", "--ca-dir", real}, "nao pode ficar dentro de"},
		{"symlink em ancestral (segredos)", []string{"--segredos-dir", linkPai + "/seg"}, "--segredos-dir " + linkPai + "/seg nao e o caminho real (" + real + "/seg)"},
		{"symlink em ancestral (ca)", []string{"--ca-dir", linkPai + "/a/ca"}, "nenhum componente pode ser link simbolico"},
		{"ancestral 777", []string{"--ca-dir", aberto777 + "/ca"}, "--ca-dir " + aberto777 + "/ca: o ancestral " + aberto777 + " tem escrita para grupo/outros (modo 777"},
		{"ancestral 777 distante", []string{"--segredos-dir", aberto777 + "/x/y/seg"}, "o ancestral " + aberto777 + " tem escrita"},
		{"ancestral sticky 1777", []string{"--segredos-dir", sticky + "/seg"}, "(modo 1777; o sticky bit nao basta)"},
		{"ancestral 770", []string{"--segredos-dir", grupo + "/seg"}, "o ancestral " + grupo + " tem escrita para grupo/outros (modo 770"},
		{"realpath diferente (/./)", []string{"--segredos-dir", e.dir + "/./seg"}, "nao e o caminho real (" + e.dir + "/seg)"},
		{"realpath diferente (//)", []string{"--ca-dir", e.dir + "//ca"}, "nao e o caminho real (" + e.dir + "/ca)"},
		{"disjuncao via symlink (ca no ssl)", []string{"--ssl-dir", sslLink, "--ca-dir", sslReal + "/ca"}, "--ca-dir (" + sslReal + "/ca) nao pode ficar dentro de --ssl-dir (" + sslLink + ") nem ser igual a ele (caminhos reais: " + sslReal + "/ca e " + sslReal + ")"},
		{"disjuncao via symlink (dados = segredos)", []string{"--segredos-dir", real, "--dados", dadosLink}, "caminhos reais: " + real + " e " + real},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := e.rodar(c.args...)
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, c.msg)
			assert.NotContains(t, r.out, "=== [2/", "falha na etapa 1, antes de qualquer efeito")
			assert.NotContains(t, e.log(), "docker build", "nenhum efeito")
			assert.NotContains(t, e.log(), "network create", "nenhum efeito")
		})
	}
	st, err := os.Stat(aberto)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), st.Mode().Perm(), "o script não corrige o modo (falha fechada)")
	noRoot, err := os.ReadDir(filepath.Join(e.dir, "root"))
	require.NoError(t, err)
	assert.Empty(t, noRoot)
}

// Lote 36 (TOCTOU): a checagem roda de novo depois do install -d. O stub do
// install troca o diretório recém-criado por um symlink (simula a troca entre
// a etapa 1 e a gravação): o script para antes de gravar senha ou chave da
// CA, e nada vai para o destino do symlink.
func TestTST16_Ensaio_RechecagemDepoisDoInstall(t *testing.T) {
	for _, c := range []struct{ nome, op, etapa string }{
		{"ca", "--ca-dir", "=== [3/8] Certificados"},
		{"segredos", "--segredos-dir", "=== [4/8] Senhas"},
	} {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaio(t)
			seg, ca := filepath.Join(e.dir, "segredos"), filepath.Join(e.dir, "ca")
			alvo := map[string]string{"--ca-dir": ca, "--segredos-dir": seg}[c.op]
			e.setState("install_troca", alvo)
			r := e.rodar("--nova-ca", "--segredos-dir", seg, "--ca-dir", ca)
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, c.etapa)
			assert.Contains(t, r.out, c.op+" "+alvo+" e um link simbolico")
			desvio, err := os.ReadDir(filepath.Join(e.dir, "desvio"))
			require.NoError(t, err)
			assert.Empty(t, desvio, "nada gravado pelo symlink")
			assert.Empty(t, e.state("run_args"), "o container não sobe")
			assert.NotContains(t, e.log(), "network create", "para antes da rede")
		})
	}
}

// --help lista as opções novas.
func TestTST16_Help_ListaOpcoes(t *testing.T) {
	cmd := exec.Command("bash", mariadbDocker(t), "--help")
	cmd.Env = ambienteLimpo()
	b, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", b)
	out := string(b)
	for _, s := range []string{"--segredos-dir DIR", "--ca-dir DIR", "modo 700"} {
		assert.Contains(t, out, s)
	}
	assert.True(t, strings.Contains(out, "seed-producao.sh so le o padrao"))
}
