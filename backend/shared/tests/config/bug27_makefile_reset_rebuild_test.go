package config_test

// BUG-27 (Lote 35): db-reset/db-rebuild não rodam o db-seed (CREATE
// DATABASE + seed) quando o DROP é recusado, nem com "make -k", e com "-j"
// cada etapa só começa depois da anterior. As 3 guardas continuam
// pré-requisitos diretos; db-down, db-seed e os db-import-* viraram uma
// cadeia "$(MAKE) etapa && $(MAKE) etapa" na receita.
// Tudo numa cópia do Makefile em t.TempDir(), com sql/ mínimo, cliente
// mariadb e go falsos que só gravam a chamada num log único: nada conecta.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bug27Imports: ordem dos imports do db-rebuild (dependências entre tabelas).
var bug27Imports = []string{"clientes", "produtos", "pedidos", "pagamentos", "carteiras", "oportunidades", "visitas", "estoque"}

func TestBUG27_Makefile_ReceitaEmCadeia(t *testing.T) {
	mk := lerMakefile(t)
	guardas := []string{"db-guard-producao", "db-guard-sem-producao", "db-guard-drop"}
	for _, a := range []string{"db-reset", "db-rebuild"} {
		t.Run(a, func(t *testing.T) {
			r := mk.regras[a]
			require.NotNil(t, r)
			assert.Equal(t, guardas, r.prereqs, "%s: só as guardas como pré-requisitos (db-down/db-seed/imports na receita)", a)
			receita := strings.Join(r.receita, " ")
			etapas := []string{"db-down", "db-seed"}
			if a == "db-rebuild" {
				for _, i := range bug27Imports {
					etapas = append(etapas, "db-import-"+i)
				}
			}
			var cadeia []string
			for _, e := range etapas {
				cadeia = append(cadeia, "$(MAKE) RP_ENV_PAI='$(ENV_ARQUIVO_LIDO)' "+e) // BUG-29: sentinela RP_ENV_PAI
			}
			assert.Equal(t, strings.Join(cadeia, " && "), strings.Join(strings.Fields(receita), " "),
				"%s: um sub-make por etapa, encadeado por &&, na ordem", a)
			// SEC-46/DEPLOY-07: "no .env não vale"; repassar à mão viraria linha de comando.
			// BUG-29: só o nome do arquivo lido vai na sentinela (RP_ENV_PAI='$(ENV_ARQUIVO_LIDO)').
			semSentinela := strings.ReplaceAll(receita, "RP_ENV_PAI='$(ENV_ARQUIVO_LIDO)'", "")
			for _, v := range []string{"CONFIRMAR", "CONFIRMO_PRODUCAO", "ENV_ARQUIVO"} {
				assert.NotContains(t, semSentinela, v, "%s: %s não é repassado à mão", a, v)
			}
		})
	}
}

// bug27Dir monta a cópia do Makefile com sql/ mínimo e cliente mariadb e go
// falsos que gravam cada chamada (argv e, sem -e, a 1a linha do stdin) em
// chamadas.log. falharDrop faz o cliente falhar no DROP.
func bug27Dir(t *testing.T, falharDrop bool) (dir, pathFalso, cliente, log string) {
	t.Helper()
	dir = copiaDoMakefile(t)
	for _, d := range []string{"sql", "backend/shared", "bin-falso"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.FromSlash(d)), 0o755))
	}
	arquivos := map[string]string{
		"sql/ordem-ddl.txt":          "# ordem\n01_ddl_a.sql\n01_ddl_b.sql\n",
		"sql/01_ddl_a.sql":           "-- DDL-A\n",
		"sql/01_ddl_b.sql":           "-- DDL-B\n",
		"sql/02_seed_admin.sql":      "-- SEED-ADMIN\n",
		"sql/03_seed_vendedores.sql": "-- SEED-VENDEDORES\n",
	}
	for nome, conteudo := range arquivos {
		require.NoError(t, os.WriteFile(filepath.Join(dir, filepath.FromSlash(nome)), []byte(conteudo), 0o644))
	}
	log = filepath.Join(dir, "chamadas.log")
	falha := ""
	if falharDrop {
		falha = "case \"$*\" in *DROP*) echo \"mariadb $*\" >> '" + log + "'; exit 1 ;; esac\n"
	}
	cliente = filepath.Join(dir, "bin-falso", "mariadb-falso")
	scriptCliente := "#!/bin/sh\n[ \"$1\" = --version ] && { echo 'mariadb from 11.4.5-MariaDB, client 15.2'; exit 0; }\n" + falha +
		"case \"$*\" in *' -e '*) echo \"mariadb $*\" >> '" + log + "' ;; *) echo \"mariadb $(head -n 1)\" >> '" + log + "' ;; esac\n"
	require.NoError(t, os.WriteFile(cliente, []byte(scriptCliente), 0o755))
	scriptGo := "#!/bin/sh\necho \"go $*\" >> '" + log + "'\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bin-falso", "go"), []byte(scriptGo), 0o755))
	pathFalso = "PATH=" + filepath.Join(dir, "bin-falso") + string(os.PathListSeparator) + os.Getenv("PATH")
	return dir, pathFalso, cliente, log
}

// bug27Etapas reduz o log às marcas de etapa, na ordem em que rodaram.
func bug27Etapas(log string) []string {
	marcas := []struct{ trecho, etapa string }{
		{"DROP DATABASE", "DROP"}, {"CREATE DATABASE", "CREATE"}, {"DDL-A", "DDL-A"}, {"DDL-B", "DDL-B"},
		{"SEED-ADMIN", "SEED-ADMIN"}, {"SEED-VENDEDORES", "SEED-VENDEDORES"}, {"resetpassword -list", "RESET-LIST"},
		{"resetpassword -create-admin", "RESET-ADMIN"}, {"resetpassword -all-users", "RESET-ALL"},
	}
	for _, i := range bug27Imports {
		marcas = append(marcas, struct{ trecho, etapa string }{"./cmd/import" + i, "IMPORT-" + i})
	}
	var etapas []string
	for _, l := range strings.Split(strings.TrimSpace(log), "\n") {
		for _, m := range marcas {
			if strings.Contains(l, m.trecho) {
				etapas = append(etapas, m.etapa)
				break
			}
		}
	}
	return etapas
}

var bug27Seed = []string{"CREATE", "DDL-A", "DDL-B", "SEED-ADMIN", "SEED-VENDEDORES", "RESET-LIST", "RESET-ADMIN", "RESET-ALL"}

func bug27Esperado(alvo string) []string {
	e := append([]string{"DROP"}, bug27Seed...)
	if alvo == "db-rebuild" {
		for _, i := range bug27Imports {
			e = append(e, "IMPORT-"+i)
		}
	}
	return e
}

func bug27Args(cliente string, extra ...string) []string {
	return append([]string{"DB_NAME=rotaperfumes", "DB_TLS_REQUIRED=false", "DB_TLS_CA=",
		"DB_USUARIO=u-bug27", "DB_SENHA=s-bug27", "DB_CLIENTE_BIN=" + cliente}, extra...)
}

var casaos27 = []string{"DB_HOST=192.168.168.106", "DB_PORT=3307"}

func TestBUG27_Makefile_KDropRecusadoNaoSemeia(t *testing.T) {
	for _, alvo := range []string{"db-reset", "db-rebuild"} {
		for _, flags := range [][]string{{"-k"}, {"-k", "-j4"}} {
			t.Run(alvo+"/"+strings.Join(flags, " "), func(t *testing.T) {
				dir, path, cliente, log := bug27Dir(t, false)
				args := append(append([]string{}, flags...), bug27Args(cliente, casaos27...)...)
				out, err := makeAlvo(t, dir, alvo, []string{path}, args...)
				require.Error(t, err, out)
				assert.Contains(t, out, "SEC-46: DROP DATABASE rotaperfumes em 192.168.168.106:3307")
				assert.Empty(t, lerSeExiste(t, log), "nenhum DROP/CREATE/seed/import com o DROP recusado")
			})
		}
	}
	t.Run("CONFIRMAR só no ambiente não vale nos sub-makes", func(t *testing.T) {
		dir, path, cliente, log := bug27Dir(t, false)
		out, err := makeAlvo(t, dir, "db-reset", []string{path, "CONFIRMAR=rotaperfumes"}, append([]string{"-k"}, bug27Args(cliente, casaos27...)...)...)
		require.Error(t, err, out)
		assert.Empty(t, lerSeExiste(t, log))
	})
	t.Run("CONFIRMAR no .env não vale nos sub-makes", func(t *testing.T) {
		dir, path, cliente, log := bug27Dir(t, false)
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("CONFIRMAR=rotaperfumes\nCONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes\n"), 0o600))
		out, err := makeAlvo(t, dir, "db-rebuild", []string{path}, append([]string{"-k"}, bug27Args(cliente, casaos27...)...)...)
		require.Error(t, err, out)
		assert.Empty(t, lerSeExiste(t, log))
	})
	t.Run("produção recusada mesmo com CONFIRMO_PRODUCAO e CONFIRMAR", func(t *testing.T) {
		dir, path, cliente, log := bug27Dir(t, false)
		args := append([]string{"-k", "-j4"}, bug27Args(cliente, "DB_HOST=192.168.168.106", "DB_PORT=3306",
			"CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes", "CONFIRMAR=rotaperfumes")...)
		out, err := makeAlvo(t, dir, "db-rebuild", []string{path}, args...)
		require.Error(t, err, out)
		assert.Contains(t, out, "mesmo com CONFIRMO_PRODUCAO")
		assert.Empty(t, lerSeExiste(t, log))
	})
}

// Com o DROP aceito pela guarda mas falhando no cliente, o "&&" para a
// cadeia mesmo com -k herdado pelos sub-makes.
func TestBUG27_Makefile_KDropFalhouParaACadeia(t *testing.T) {
	for _, alvo := range []string{"db-reset", "db-rebuild"} {
		t.Run(alvo, func(t *testing.T) {
			dir, path, cliente, log := bug27Dir(t, true)
			args := append([]string{"-k", "-j4"}, bug27Args(cliente, "DB_HOST=127.0.0.1", "DB_PORT=3306")...)
			out, err := makeAlvo(t, dir, alvo, []string{path}, args...)
			require.Error(t, err, out)
			assert.Equal(t, []string{"DROP"}, bug27Etapas(lerSeExiste(t, log)), "nada depois do DROP que falhou")
		})
	}
}

func TestBUG27_Makefile_OrdemGarantida(t *testing.T) {
	casos := []struct {
		nome  string
		flags []string
		extra []string
	}{
		{"loopback", nil, []string{"DB_HOST=127.0.0.1", "DB_PORT=3306"}},
		{"loopback -j4", []string{"-j4"}, []string{"DB_HOST=127.0.0.1", "DB_PORT=3306"}},
		{"loopback -k -j8", []string{"-k", "-j8"}, []string{"DB_HOST=localhost", "DB_PORT=3306"}},
		// CONFIRMAR na linha de comando chega aos sub-makes pelo MAKEFLAGS.
		{"casaos com CONFIRMAR -j4", []string{"-j4"}, append([]string{"CONFIRMAR=rotaperfumes"}, casaos27...)},
	}
	for _, alvo := range []string{"db-reset", "db-rebuild"} {
		for _, c := range casos {
			t.Run(alvo+"/"+c.nome, func(t *testing.T) {
				dir, path, cliente, log := bug27Dir(t, false)
				args := append(append([]string{}, c.flags...), bug27Args(cliente, c.extra...)...)
				out, err := makeAlvo(t, dir, alvo, []string{path}, args...)
				require.NoError(t, err, out)
				texto := lerSeExiste(t, log)
				assert.Equal(t, bug27Esperado(alvo), bug27Etapas(texto), "ordem DROP -> CREATE -> DDL -> seed -> imports\n%s", texto)
				assert.NotContains(t, texto+out, "s-bug27", "senha só por MYSQL_PWD")
			})
		}
	}
}

// TestBrain: uma etapa do meio que falha (um import, ou o seed) para o resto
// da cadeia do db-rebuild, mesmo com -k -j4 herdados pelos sub-makes.
func TestBUG27_Makefile_KEtapaDoMeioFalhaParaACadeia(t *testing.T) {
	casos := []struct {
		nome, trecho string
		esperado     []string
	}{
		{"import de pedidos falha", "./cmd/importpedidos", append(append([]string{"DROP"}, bug27Seed...), "IMPORT-clientes", "IMPORT-produtos", "IMPORT-pedidos")},
		{"seed: resetpassword -create-admin falha", "resetpassword -create-admin", []string{"DROP", "CREATE", "DDL-A", "DDL-B", "SEED-ADMIN", "SEED-VENDEDORES", "RESET-LIST", "RESET-ADMIN"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir, path, cliente, log := bug27Dir(t, false)
			scriptGo := "#!/bin/sh\necho \"go $*\" >> '" + log + "'\ncase \"$*\" in *'" + c.trecho + "'*) exit 1 ;; esac\nexit 0\n"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "bin-falso", "go"), []byte(scriptGo), 0o755))
			args := append([]string{"-k", "-j4"}, bug27Args(cliente, "DB_HOST=127.0.0.1", "DB_PORT=3306")...)
			out, err := makeAlvo(t, dir, "db-rebuild", []string{path}, args...)
			require.Error(t, err, out)
			assert.Equal(t, c.esperado, bug27Etapas(lerSeExiste(t, log)), "nada depois da etapa que falhou\n%s", out)
		})
	}
}
