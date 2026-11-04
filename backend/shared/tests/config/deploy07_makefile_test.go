package config_test

// DEPLOY-07 (Lote 33): Makefile.
//   - sql/ordem-ddl.txt: só DDL existente, todo NN_ddl_*.sql uma vez, FKs na
//     ordem; o db-up a lê (DDL_ORDEM);
//   - db-guard-producao: dependência dos alvos que escrevem no banco; recusa
//     produção por host+porta (o dev do CasaOS, mesmo host na 3307, passa) salvo CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME> na linha
//     de comando (no ambiente não vale);
//   - nenhuma senha fixa (Admin@123) e fix-hash/fix-admin com -password-prompt.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lerOrdemDDL(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, l := range strings.Split(lerTexto(t, filepath.Join(raizProjeto(t), "sql", "ordem-ddl.txt")), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
	}
	return out
}

func TestDEPLOY07_OrdemDDL(t *testing.T) {
	root := raizProjeto(t)
	ordem := lerOrdemDDL(t)
	require.NotEmpty(t, ordem)
	pos := map[string]int{}
	for i, f := range ordem {
		assert.Regexp(t, `^[0-9]{2}_ddl_[a-z_]+\.sql$`, f)
		assert.FileExists(t, filepath.Join(root, "sql", f))
		_, dup := pos[f]
		assert.False(t, dup, "%s repetido", f)
		pos[f] = i
	}
	todos, err := filepath.Glob(filepath.Join(root, "sql", "[0-9][0-9]_ddl_*.sql"))
	require.NoError(t, err)
	for _, a := range todos {
		assert.Contains(t, pos, filepath.Base(a), "toda DDL entra em sql/ordem-ddl.txt")
	}
	assert.Equal(t, "01_ddl_usuarios.sql", ordem[0], "vendedores/usuarios primeiro (FKs)")
	antes := [][2]string{
		{"09_ddl_clientes.sql", "04_ddl_pedidos.sql"},
		{"04_ddl_pedidos.sql", "11_ddl_itens_pedido.sql"},
		{"10_ddl_produtos.sql", "11_ddl_itens_pedido.sql"},
		{"04_ddl_pedidos.sql", "12_ddl_pagamentos.sql"},
		{"09_ddl_clientes.sql", "14_ddl_carteiras.sql"},
		{"09_ddl_clientes.sql", "15_ddl_oportunidades.sql"},
		{"09_ddl_clientes.sql", "16_ddl_visitas.sql"},
		{"10_ddl_produtos.sql", "17_ddl_estoque.sql"},
	}
	for _, p := range antes {
		assert.Less(t, pos[p[0]], pos[p[1]], "%s antes de %s", p[0], p[1])
	}

	mk := lerMakefile(t)
	assert.Contains(t, mk.texto, "DDL_ORDEM := $(shell grep -Ev '^[[:space:]]*(\\#|$$)' sql/ordem-ddl.txt")
	receita := strings.Join(mk.regras["db-up"].receita, "\n")
	assert.Contains(t, receita, "for f in $(DDL_ORDEM)")
	assert.Contains(t, receita, `$(DB_CLI) $(DB_NAME) < "sql/$$f" || exit 1`)
}

func TestDEPLOY07_Makefile_GuardaDeProducaoNosAlvos(t *testing.T) {
	mk := lerMakefile(t)
	require.Contains(t, mk.regras, "db-guard-producao")
	assert.True(t, mk.phony["db-guard-producao"])
	assert.False(t, mk.dependeDe("db-check-env", "db-guard-producao"), "db-check-env continua só conferindo o ambiente")

	alvos := []string{"db-down", "db-up", "db-create", "db-seed", "db-reset", "db-rebuild", "db-seed-mes-atual",
		"test-integration", "gen-hash", "reset-senhas-ativos", "db-gerar-imagens-produtos"}
	for _, a := range mk.ordem {
		if strings.HasPrefix(a, "db-import-") || strings.HasPrefix(a, "db-fix-") || strings.HasPrefix(a, "db-revert-") || strings.HasPrefix(a, "fix-") {
			alvos = append(alvos, a)
		}
	}
	require.GreaterOrEqual(t, len(alvos), 35)
	for _, a := range alvos {
		t.Run(a, func(t *testing.T) {
			require.Contains(t, mk.regras, a)
			assert.Contains(t, mk.regras[a].prereqs, "db-guard-producao", "%s depende direto da guarda", a)
		})
	}
	for _, a := range []string{"help", "build", "test", "test-shared", "test-api", "lint", "db-check-env", "db-export", "reset-senhas-simular"} {
		assert.False(t, mk.dependeDe(a, "db-guard-producao"), "%s não precisa da guarda", a)
	}
	lan := strings.Fields(strings.SplitN(mk.linhaDaVariavel(t, "PRODUCAO_HOSTS_LAN"), "=", 2)[1])
	rede := strings.Fields(strings.SplitN(mk.linhaDaVariavel(t, "PRODUCAO_HOSTS_REDE"), "=", 2)[1])
	assert.Contains(t, lan, "192.168.168.106")
	assert.Contains(t, rede, "rotaperfumes-mariadb")
	assert.Contains(t, rede, "172.30.42.2")
}

func TestDEPLOY07_Makefile_SemSenhaFixa(t *testing.T) {
	mk := lerMakefile(t)
	assert.NotContains(t, mk.texto, "Admin@123")
	assert.NotRegexp(t, `resetpassword[^\n]*-password=`, mk.texto)
	for _, a := range []string{"fix-hash", "fix-admin"} {
		receita := strings.Join(mk.regras[a].receita, "\n")
		assert.Contains(t, receita, "-create-admin -password-prompt", a)
		assert.NotContains(t, receita, "@echo.", "%s: echo. não existe no sh", a)
	}
}

// deploy07Make roda "make db-guard-producao" numa cópia do Makefile, sem .env.
func deploy07Make(t *testing.T, env []string, args ...string) (string, error) {
	t.Helper()
	return makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", env, args...)
}

// copiaDoMakefile copia o Makefile do projeto para um diretório temporário
// (sem .env, sem sql/, sem backend/): nada ali conecta em banco.
func copiaDoMakefile(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("usa sh")
	}
	dir := t.TempDir()
	dados, err := os.ReadFile(filepath.Join(raizProjeto(t), "Makefile"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Makefile"), dados, 0o644))
	return dir
}

// makeAlvo roda "make -s <alvo> args..." em dir com ambiente mínimo (PATH,
// HOME=dir, LC_ALL=C) mais env; nada do ambiente do go test (MAKEFLAGS,
// DB_*, .env real) vaza para o make.
func makeAlvo(t *testing.T, dir, alvo string, env []string, args ...string) (string, error) {
	t.Helper()
	makeBin, err := exec.LookPath("make")
	if err != nil {
		t.Skip("make não encontrado")
	}
	cmd := exec.Command(makeBin, append([]string{"-s", alvo}, args...)...)
	cmd.Dir = dir
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "LC_ALL=C"}, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// casoGuardaProducao: produção = host do servidor na porta 3306 (ou sem
// porta) ou o nome/IP do container na rede docker (qualquer porta). O dev do
// CasaOS é o mesmo host na porta 3307 e passa. A mesma tabela roda contra o
// db-guard-producao do Makefile (DEPLOY-07) e contra o
// integracao.ValidarNaoProducao dos testes Go (SEC-50).
type casoGuardaProducao struct {
	host, porta string // porta "" = DB_PORT vazio na linha de comando
	recusa      bool
}

var casosGuardaProducao = []casoGuardaProducao{
	{"192.168.168.106", "3306", true},
	{"192.168.168.106", "", true},
	{"192.168.168.106", "3307", false},
	{"192.168.168.106", "03306", true}, // porta como número (o cliente conecta na 3306)
	{"192.168.168.106", "+3306", true},
	{"192.168.168.106", "03307", false},
	{"IVO-INSPIRON-15-3530", "3306", true},
	{"ivo-inspiron-15-3530", "3307", false},
	{"rotaperfumes-mariadb", "3306", true},
	{"rotaperfumes-mariadb", "3307", true},
	{"ROTAPERFUMES-MARIADB.", "3306", true},
	{"172.30.42.2", "3306", true},
	{"172.30.42.2", "13306", true},
	{"127.0.0.1", "3306", false},
	{"localhost", "3306", false},
	{"192.168.168.107", "3306", false},
}

func TestDEPLOY07_Makefile_GuardaRecusaProducao(t *testing.T) {
	casos := casosGuardaProducao
	for _, c := range casos {
		t.Run(c.host+":"+c.porta, func(t *testing.T) {
			out, err := deploy07Make(t, nil, "DB_HOST="+c.host, "DB_PORT="+c.porta)
			if c.recusa {
				require.Error(t, err, out)
				assert.Contains(t, out, "e o banco de PRODUCAO")
				assert.Contains(t, out, "deploy/seed-producao.sh")
			} else {
				assert.NoError(t, err, out)
				assert.NotContains(t, out, "PRODUCAO")
			}
		})
	}
	t.Run("confirmado na linha de comando", func(t *testing.T) {
		out, err := deploy07Make(t, nil, "DB_HOST=192.168.168.106", "DB_PORT=3306", "DB_NAME=rotaperfumes", "CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes")
		assert.NoError(t, err, out)
		assert.Contains(t, out, "AVISO (DEPLOY-07)")
	})
	t.Run("confirmação errada", func(t *testing.T) {
		out, err := deploy07Make(t, nil, "DB_HOST=192.168.168.106", "DB_PORT=3306", "DB_NAME=rotaperfumes", "CONFIRMO_PRODUCAO=192.168.168.106/outro")
		assert.Error(t, err, out)
	})
	t.Run("confirmação pelo ambiente não vale", func(t *testing.T) {
		out, err := deploy07Make(t, []string{"CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes"}, "DB_HOST=192.168.168.106", "DB_PORT=3306", "DB_NAME=rotaperfumes")
		assert.Error(t, err, out)
	})
}

// ENV_ARQUIVO (só pela linha de comando) troca o .env por outro arquivo.
func TestDEPLOY07_Makefile_EnvArquivo(t *testing.T) {
	mk := lerMakefile(t)
	assert.Regexp(t, regexp.MustCompile(`(?m)^ifeq \(\$\(origin ENV_ARQUIVO\),command line\)\n(#.*\n)*include \$\(ENV_ARQUIVO\)\nelse\n-include \.env\nendif$`), mk.texto)
}
