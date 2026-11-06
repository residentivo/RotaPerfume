package deploy_test

// Lote 39 (TestBrain): lacunas dos testes do CI-02, SEC-60, SEC-61 e SEC-62.

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// O regex é o do alvo, lido do Makefile: um teste com cópia própria continuaria
// verde se o alvo deixasse de enxergar o GO_TEST_IMAGE.
func TestCI02_GoTestImageVistaPeloDepsAuditImagens(t *testing.T) {
	mk := lerRaiz(t, "Makefile")
	fontes := regexp.MustCompile(`(?m)^IMAGENS_FIXADAS_EM\s*:?=\s*(.+)$`).FindStringSubmatch(mk)
	require.NotNil(t, fontes, "IMAGENS_FIXADAS_EM no Makefile")
	assert.Contains(t, strings.Fields(fontes[1]), "Jenkinsfile")

	grep := regexp.MustCompile(`grep -hoE '([^']+)' \$\(IMAGENS_FIXADAS_EM\)`).FindStringSubmatch(mk)
	require.NotNil(t, grep, "grep do deps-audit-imagens")
	refs := regexp.MustCompile(grep[1]).FindAllString(lerRaiz(t, "Jenkinsfile"), -1)
	for _, nome := range []string{"GO_IMAGE", "GO_TEST_IMAGE", "NODE_IMAGE", "DB_CLIENT_IMAGE"} {
		assert.Contains(t, refs, imagemJenkins(t, lerRaiz(t, "Jenkinsfile"), nome), nome)
	}
}

// Sem pipefail, uma falha do git archive viraria tar vazio no container; com
// bind mount ou credencial, o tar deixaria de ser a única entrada do estágio.
func TestSEC62_EstagiosSoRecebemOTar(t *testing.T) {
	for _, nome := range []string{"Testes Go", "Testes Frontend"} {
		t.Run(nome, func(t *testing.T) {
			codigo := sec62Codigo(t, nome)
			iPipefail := strings.Index(codigo, "set -euo pipefail")
			require.GreaterOrEqual(t, iPipefail, 0)
			assert.Less(t, iPipefail, strings.Index(codigo, "git archive"))
			assert.Regexp(t, `(?s)git archive --format=tar HEAD -- .*?\|\s*docker run --rm -i `, codigo, "o tar entra pelo stdin")
			for _, proibido := range []string{"withCredentials", "--mount", "--volume", "--env-file", "--network", "--user root"} {
				assert.NotContains(t, codigo, proibido)
			}
			assert.NotRegexp(t, `docker run[^\n]* -v `, codigo, "sem bind mount")
			assert.NotRegexp(t, `-e (DB_|MYSQL_|JWT_|PASSWORD_|SMTP_)`, codigo, "sem segredo no container")
		})
	}
}

func TestSEC62_ListaSemCaminhoAmplo(t *testing.T) {
	for _, nome := range []string{"Testes Go", "Testes Frontend"} {
		for _, c := range sec62Caminhos(t, sec62Codigo(t, nome)) {
			t.Run(nome+"/"+c, func(t *testing.T) {
				assert.NotContains(t, []string{".", "./", "*", "backend", ":/"}, c)
				assert.False(t, strings.HasPrefix(c, "/") || strings.HasPrefix(c, "..") || strings.HasPrefix(c, ":"), "caminho fora do repositório ou pathspec mágico")
				assert.NotContains(t, c, "*", "sem curinga")
			})
		}
	}
}

// sec62NomeSensivel: nome que não pode estar em um tar de teste. Só o nome é
// conferido, nunca o conteúdo.
func sec62NomeSensivel(caminho string) bool {
	base := path.Base(caminho)
	if strings.HasSuffix(base, ".example") {
		return false
	}
	if base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".env") {
		return true
	}
	for _, sufixo := range []string{".pem", ".key", ".p12", ".pfx", ".sql.gz", ".dump", ".senha"} {
		if strings.HasSuffix(base, sufixo) {
			return true
		}
	}
	return strings.HasPrefix(base, "id_rsa") || strings.HasPrefix(base, "id_ed25519")
}

func TestSEC62_NomeSensivel(t *testing.T) {
	casos := map[string]bool{
		".env": true, "deploy/api.env": true, ".env.producao": true, "frontend/.env.local": true,
		"deploy/ca-cert.pem": true, "a/b.key": true, "deploy/dumps/x.sql.gz": true, "root/x.dump": true,
		"admin-app.senha": true, "home/id_rsa": true, "x/id_ed25519.pub": true,
		".env.example": false, "deploy/api.env.example": false, "frontend/.env.local.example": false,
		"sql/07_ddl_senha_historico.sql": false, "backend/shared/env.go": false,
	}
	for caminho, sensivel := range casos {
		t.Run(caminho, func(t *testing.T) { assert.Equal(t, sensivel, sec62NomeSensivel(caminho)) })
	}
}

// O git archive só protege do que está fora do índice: um .env rastreado iria
// no tar. Sem .git (o próprio tar do CI) não há o que consultar.
func TestSEC62_ListasSemArquivoSensivelRastreado(t *testing.T) {
	if _, err := os.Stat(caminhoRaiz(t, ".git")); err != nil {
		t.Skip(".git ausente (tar do git archive)")
	}
	ferramentaObrigatoria(t, "git")
	for _, nome := range []string{"Testes Go", "Testes Frontend"} {
		t.Run(nome, func(t *testing.T) {
			cmd := exec.Command("git", append([]string{"ls-files", "--"}, sec62Caminhos(t, sec62Codigo(t, nome))...)...)
			cmd.Dir = raiz(t)
			out, err := cmd.Output()
			require.NoError(t, err)
			arquivos := linhas(string(out))
			require.NotEmpty(t, arquivos)
			for _, a := range arquivos {
				assert.False(t, sec62NomeSensivel(a), "rastreado e na lista do estágio: %s", a)
			}
		})
	}
}

func TestSEC62_GitignoreCobreSegredos(t *testing.T) {
	gi := linhas(lerRaiz(t, ".gitignore"))
	for _, padrao := range []string{".env", "*.env", ".env.producao", "deploy/api.env", "deploy/dumps/", "*.sql.gz", "deploy/ca-cert.pem", "deploy/ca-producao.pem"} {
		assert.Contains(t, gi, padrao)
	}
}

func TestSEC60_Ensaio_OutrosDiretoriosRecusados(t *testing.T) {
	modo := func(perm os.FileMode) func(t *testing.T, base string) string {
		return func(t *testing.T, base string) string {
			d := filepath.Join(base, "d")
			require.NoError(t, os.Mkdir(d, 0o700))
			require.NoError(t, os.Chmod(d, perm))
			return filepath.Join(d, "admin.senha")
		}
	}
	casos := []struct {
		nome string
		prep func(t *testing.T, base string) string
		msg  string
	}{
		{"modo 770", modo(0o770), "precisa ser do root com modo 700"},
		{"modo 750", modo(0o750), "precisa ser do root com modo 700"},
		{"modo 701", modo(0o701), "precisa ser do root com modo 700"},
		{"modo 500", modo(0o500), "precisa ser do root com modo 700"},
		{"/tmp, o exemplo do card", func(*testing.T, string) string { return "/tmp/admin.senha" }, "precisa ser do root"},
		{"raiz", func(*testing.T, string) string { return "/admin.senha" }, "precisa ser do root"},
		{"o pai é um arquivo", func(t *testing.T, base string) string {
			p := filepath.Join(base, "arquivo")
			require.NoError(t, os.WriteFile(p, nil, 0o600))
			return filepath.Join(p, "admin.senha")
		}, "nao existe"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioSeed(t)
			out, err := e.rodar("--pepper-file", "/root/pepper.txt", "--admin-senha-arquivo", c.prep(t, e.dir))
			e.recusaSemEfeito(t, out, err, c.msg)
			assert.Contains(t, out, "--admin-senha-arquivo")
			assert.Empty(t, e.chamadasDeBanco(), "nenhum cliente mariadb")
		})
	}
}

// No diretório privado, o próprio arquivo não pode existir nem como symlink:
// o install gravaria a senha no alvo do link.
func TestSEC60_Ensaio_ArquivoJaExistente(t *testing.T) {
	casos := []struct {
		nome string
		prep func(t *testing.T, arq, alvo string)
	}{
		{"arquivo comum", func(t *testing.T, arq, _ string) { require.NoError(t, os.WriteFile(arq, []byte("anterior"), 0o600)) }},
		{"symlink para outro arquivo", func(t *testing.T, arq, alvo string) { require.NoError(t, os.Symlink(alvo, arq)) }},
		{"symlink quebrado", func(t *testing.T, arq, alvo string) { require.NoError(t, os.Symlink(alvo+".nao-existe", arq)) }},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioSeed(t)
			d := filepath.Join(e.dir, "privado")
			require.NoError(t, os.Mkdir(d, 0o700))
			require.NoError(t, os.Chmod(d, 0o700))
			alvo := filepath.Join(e.dir, "alvo-do-link")
			require.NoError(t, os.WriteFile(alvo, []byte("intacto"), 0o600))
			arq := filepath.Join(d, "admin.senha")
			c.prep(t, arq, alvo)

			out, err := e.rodar("--pepper-file", "/root/pepper.txt", "--admin-senha-arquivo", arq)

			e.recusaSemEfeito(t, out, err, "ja existe")
			assert.NotContains(t, out, senhaAdmSeed)
			b, errLer := os.ReadFile(alvo)
			require.NoError(t, errLer)
			assert.Equal(t, "intacto", string(b), "o alvo do link não pode ser sobrescrito")
		})
	}
}

// Formas de keyring divergente que o TestSEC61_PreflightKeyring não exercita.
func TestSEC61_PreflightKeyringInconfiavel(t *testing.T) {
	g := novoGpgSEC58(t)
	fprA := g.gerar("SEC61 E <e@sec61.invalid>")
	fprB := g.gerar("SEC61 F <f@sec61.invalid>")
	fprRevogada := g.gerar("SEC61 G <g@sec61.invalid>")
	duasChaves := g.exportar("duas.pgp", fprA, fprB)
	certa := g.exportar("e.pgp", fprA)
	g.revogar(fprRevogada)
	revogada := g.exportar("g.pgp", fprRevogada)

	dir := t.TempDir()
	corrompido := filepath.Join(dir, "corrompido.pgp")
	require.NoError(t, os.WriteFile(corrompido, []byte("não é uma chave"), 0o600))
	vazio := filepath.Join(dir, "vazio.pgp")
	require.NoError(t, os.WriteFile(vazio, nil, 0o600))
	linkQuebrado := filepath.Join(dir, "quebrado.pgp")
	require.NoError(t, os.Symlink(filepath.Join(dir, "nao-existe"), linkQuebrado))
	linkCerto := filepath.Join(dir, "link-certo.pgp")
	require.NoError(t, os.Symlink(certa, linkCerto))

	inconfiaveis := []struct{ nome, k, fpr string }{
		{"certa mais uma extra", duasChaves, fprA},
		{"revogada", revogada, fprRevogada},
		{"corrompido", corrompido, fprA},
		{"vazio", vazio, fprA},
		{"symlink quebrado", linkQuebrado, fprA},
	}
	for _, c := range inconfiaveis {
		t.Run("instalar aborta/"+c.nome, func(t *testing.T) {
			out, err := rodarPreflightSEC61(t, ambienteLimpo(), "instalar", "1", c.k, c.fpr)
			assert.Equal(t, 1, codigoSaida(t, err), out)
			assert.NotContains(t, out, marcaDepoisPreflightSEC61)
			assert.Contains(t, out, "SEC-61: "+c.k+" não confere")
		})
		t.Run("check registra FALTA/"+c.nome, func(t *testing.T) {
			out, err := rodarPreflightSEC61(t, ambienteLimpo(), "check", "1", c.k, c.fpr)
			assert.Equal(t, 0, codigoSaida(t, err), out)
			assert.Contains(t, out, "REGISTRAR mariadb-key|FALTA|"+c.k+" não confere")
		})
	}
	t.Run("FPR com espaços e minúsculas é o mesmo", func(t *testing.T) {
		grupos := regexp.MustCompile(`.{1,4}`).FindAllString(strings.ToLower(fprA), -1)
		out, err := rodarPreflightSEC61(t, ambienteLimpo(), "instalar", "1", certa, strings.Join(grupos, " "))
		assert.Equal(t, 0, codigoSaida(t, err), out)
		assert.Contains(t, out, marcaDepoisPreflightSEC61)
	})
	t.Run("symlink para a chave certa", func(t *testing.T) {
		out, err := rodarPreflightSEC61(t, ambienteLimpo(), "instalar", "1", linkCerto, fprA)
		assert.Equal(t, 0, codigoSaida(t, err), out)
		assert.Contains(t, out, marcaDepoisPreflightSEC61)
	})
}
