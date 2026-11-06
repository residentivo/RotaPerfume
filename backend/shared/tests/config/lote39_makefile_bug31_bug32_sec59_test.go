package config_test

// Lote 39 (BackBrain-A), em cópias do Makefile com cliente mariadb e go falsos
// (bug27Dir/bug29Dir): nada conecta em banco.
//   - BUG-31: DB_PORT vazio não faz o -P engolir a opção seguinte;
//   - SEC-59: o arquivo lido só aceita linhas NOME=valor (diretivas do make,
//     SHELL e PATH são recusados no parse);
//   - BUG-32: o DB_PORT é lido do ambiente nas guardas, nunca interpolado.
//   - DB-09: alvos db-fix/db-revert-historico-created-at.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lote39AnexarEnv(t *testing.T, dir, conteudo string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, ".env"), os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(conteudo)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

// lote39EnvSemPorta grava um .env de loopback em que DB_PORT é vazio.
func lote39EnvSemPorta(t *testing.T, dir string) {
	t.Helper()
	conteudo := "DB_HOST=127.0.0.1\nDB_PORT=\nDB_NAME=rp_a\nDB_USUARIO=u-l39\nDB_SENHA=s-l39\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte(conteudo), 0o600))
}

func TestBUG31_Makefile_PortaVaziaNaoEngoleOpcao(t *testing.T) {
	casos := []struct {
		nome string
		args []string
	}{
		{"DB_PORT= no .env", nil},
		{"DB_PORT= na linha de comando", []string{"DB_PORT="}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			lote39EnvSemPorta(t, dir)
			out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente, c.args...)...)
			require.NoError(t, err, out)
			assert.Contains(t, lerSeExiste(t, log), "-P 3306 --default-character-set=utf8mb4")
		})
	}
	t.Run("192.168.168.106 com porta vazia continua produção", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		out, err := makeAlvo(t, dir, "db-create", []string{path},
			bug29Args(cliente, "DB_HOST=192.168.168.106", "DB_PORT=", "DB_TLS_REQUIRED=false")...)
		require.Error(t, err, out)
		assert.Contains(t, out, "DEPLOY-07: DB_HOST=192.168.168.106 DB_PORT=3306 e o banco de PRODUCAO")
		assert.Empty(t, lerSeExiste(t, log))
	})
	t.Run("db-check-env recusa porta inválida sozinho", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		out, err := makeAlvo(t, dir, "db-check-env", []string{path}, bug29Args(cliente, "DB_PORT=abc")...)
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-57: DB_PORT='abc' invalida")
		assert.Empty(t, lerSeExiste(t, log))
	})
}

func TestSEC59_Makefile_DiretivasNoEnvRecusadas(t *testing.T) {
	linhas := []string{
		"override MAKEFLAGS=-k",
		"override X=segredo-l39",
		"private X=1",
		"define X",
		"X != echo 1",
		"X ::= 1",
		"X :::= 1",
		"include a.env",
		"-include a.env",
		"unexport DB_PORT",
		"undefine DB_HOST",
		"export X",
		"alvo: ; @echo PWN",
		"alvo: Y=1",
		".SHELLFLAGS=-c",
		".DEFAULT_GOAL=x",
		"$(shell echo PWN)",
		"X=a \\",
		"# comentario \\",
	}
	for _, linha := range linhas {
		t.Run(linha, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			lote39AnexarEnv(t, dir, linha+"\n")
			out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente)...)
			require.Error(t, err, out)
			assert.Contains(t, out, "SEC-59: .env:6 nao e NOME=valor", "só o número da linha")
			assert.NotContains(t, out, "segredo-l39", "o conteúdo da linha pode ter segredo")
			assert.NotContains(t, out, "PWN")
			assert.Empty(t, lerSeExiste(t, log), "nenhum cliente/go antes da recusa")
		})
	}
	t.Run("ENV_ARQUIVO também passa pela allowlist", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		f := filepath.Join(dir, "a.env")
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(f, append(b, []byte("override X=1\n")...), 0o600))
		out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente, "ENV_ARQUIVO=a.env")...)
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-59: a.env:6 nao e NOME=valor")
		assert.Empty(t, lerSeExiste(t, log))
	})
	t.Run("várias linhas ruins são listadas", func(t *testing.T) {
		dir, path, cliente, _ := bug29Dir(t)
		lote39AnexarEnv(t, dir, "override A=1\nB=2\ndefine C\n")
		out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente)...)
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-59: .env:6 8 nao e NOME=valor")
	})
}

// Ressalva do SecBrain: "$" no valor expandiria no parse ou na exportação
// ($(eval), $(shell), $(file)) e desviaria as guardas. Os argumentos apontam para a
// produção, mas o cliente é falso: nada conecta.
func TestSEC59_Makefile_DolarNoValorRecusado(t *testing.T) {
	casos := []struct{ nome, linha string }{
		{"shell imediato", "X := $(shell echo PWN > pwn-l39)"},
		{"shell adiado", "X = $(shell echo PWN > pwn-l39)"},
		{"shell com chaves", "X=${shell echo PWN > pwn-l39}"},
		{"eval hosts de producao", "X := $(eval override PRODUCAO_HOSTS_LAN :=)"},
		{"eval shell", "X := $(eval override SHELL := /bin/true)"},
		{"file", "X=$(file >pwn-l39,x)"},
		{"referencia simples", "X=$Y"},
		{"dolar dobrado", "X=a$$b"},
		{"com export e tab", "\texport X := $(shell echo PWN > pwn-l39)"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			lote39AnexarEnv(t, dir, c.linha+"\n")
			out, err := makeAlvo(t, dir, "db-create", []string{path},
				bug29Args(cliente, "DB_HOST=192.168.168.106", "DB_PORT=3306", "DB_TLS_REQUIRED=false")...)
			require.Error(t, err, out)
			assert.Contains(t, out, "SEC-59: .env:6 nao e NOME=valor", "só o número da linha")
			assert.NotContains(t, out, "pwn-l39", "o conteúdo da linha pode ter segredo")
			assert.NoFileExists(t, filepath.Join(dir, "pwn-l39"), "a linha não pode expandir")
			assert.Empty(t, lerSeExiste(t, log), "nenhum cliente chamado")
		})
	}
	t.Run("comentário com dólar é aceito e não expande", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		lote39AnexarEnv(t, dir, "# custo $(shell echo PWN > pwn-l39)\n   # $X\n\t# ${Y}\n")
		out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente)...)
		require.NoError(t, err, out)
		assert.NoFileExists(t, filepath.Join(dir, "pwn-l39"))
		assert.Contains(t, lerSeExiste(t, log), "CREATE DATABASE")
	})
}

// Ressalva do SecBrain: "-foo" passa no charset, mas grep e sed o tomariam como
// opção; com o erro em /dev/null, a allowlist ficaria vazia e o include leria o
// arquivo sem conferência.
func TestSEC59_Makefile_EnvArquivoComHifenInicial(t *testing.T) {
	escrever := func(t *testing.T, dir string) {
		t.Helper()
		conteudo := "DB_HOST=127.0.0.1\nDB_NAME=rp_hifen\nDB_USUARIO=u-l39\nDB_SENHA=s-l39\noverride X=1\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "-foo"), []byte(conteudo), 0o600))
	}
	t.Run("-foo é recusado", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		escrever(t, dir)
		out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente, "ENV_ARQUIVO=-foo")...)
		require.Error(t, err, out)
		assert.Contains(t, out, "BUG-29: ENV_ARQUIVO='-foo' invalido")
		assert.Empty(t, lerSeExiste(t, log))
	})
	t.Run("./-foo passa pela allowlist", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		escrever(t, dir)
		out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente, "ENV_ARQUIVO=./-foo")...)
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-59: ./-foo:5 nao e NOME=valor")
		assert.Empty(t, lerSeExiste(t, log))
	})
	t.Run("leituras do arquivo usam --", func(t *testing.T) {
		mk := lerMakefile(t)
		for _, v := range []string{"RP_ENV_LINHAS_RUINS", "VARS_DO_ENV"} {
			l := mk.linhaDaVariavel(t, v)
			assert.Equal(t, strings.Count(l, "$(ENV_ARQUIVO_LIDO)"), strings.Count(l, " -- "), "%s: um -- por leitura: %s", v, l)
		}
	})
}

// SHELL/PATH passam na allowlist e caem na lista de proibidas. A conferência
// precisa vir antes do include: depois dele, o próprio $(shell sed ...) rodaria
// com o SHELL do arquivo e a recusa nunca dispararia.
func TestSEC59_Makefile_ShellEPathNoEnvRecusados(t *testing.T) {
	for _, linha := range []string{"SHELL=/bin/true", "export SHELL := /bin/true", "MAKESHELL=/bin/true", "MAKECMDGOALS=x",
		"CURDIR=/tmp", "VPATH=/tmp", "PATH=/tmp", "VARS_DO_ENV=", "GNUMAKEFLAGS += -k"} {
		t.Run(linha, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			lote39AnexarEnv(t, dir, linha+"\n")
			out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente)...)
			require.Error(t, err, out)
			assert.Contains(t, out, "BUG-29: .env declara")
			assert.Contains(t, out, "SEC-59")
			assert.Empty(t, lerSeExiste(t, log))
		})
	}
}

func TestSEC59_Makefile_FormasAceitas(t *testing.T) {
	t.Run("arquivo com comentários, vazias e CRLF passa", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		lote39AnexarEnv(t, dir, "\n# comentario\n   \n\t# outro\r\n\r\nX_CRLF=1\r\n")
		out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente)...)
		require.NoError(t, err, out)
		assert.Contains(t, lerSeExiste(t, log), "CREATE DATABASE")
	})
	t.Run("sem .env segue (-include)", func(t *testing.T) {
		out, err := makeAlvo(t, copiaDoMakefile(t), "help", nil)
		require.NoError(t, err, out)
		assert.Contains(t, out, "db-fix-historico-created-at")
	})
	// Paridade: toda forma aceita pela allowlist entra no VARS_DO_ENV e, por ele,
	// no env -u do SEM_SEGREDOS.
	for _, linha := range []string{"X_L39=1", "export X_L39=1", "X_L39 := 1", "X_L39 ?= 1", "X_L39 += 1", "\tX_L39=1", "  export\tX_L39 = 1"} {
		t.Run("SEM_SEGREDOS/"+linha, func(t *testing.T) {
			dir, path, _, log := bug29Dir(t)
			lote39AnexarEnv(t, dir, linha+"\n")
			scriptEnv := "#!/bin/sh\necho \"env $*\" >> '" + log + "'\n"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "bin-falso", "env"), []byte(scriptEnv), 0o755))
			out, err := makeAlvo(t, dir, "build-shared", []string{path})
			require.NoError(t, err, out)
			assert.Contains(t, lerSeExiste(t, log), "-u X_L39 ")
		})
	}
}

func TestBUG32_Makefile_PortaComCaracteresDeShell(t *testing.T) {
	guardas := []string{"db-guard-producao", "db-guard-sem-producao", "db-guard-integracao", "db-check-env", "db-create"}
	// "$$" na linha de comando do make vira "$" no valor.
	casos := []struct{ nome, porta, msg string }{
		{"quebra no fim", "3306\n", "SEC-57: DB_PORT='3306?'"},
		{"quebra no meio", "33\n06", "SEC-57: DB_PORT='33?06'"},
		{"aspas", `3306"; echo PWN; "`, `SEC-57: DB_PORT='3306"; echo PWN; "'`},
		{"substituicao", "$$(echo PWN)", "SEC-57: DB_PORT='$(echo PWN"},
		{"crase", "`echo PWN`", "SEC-57: DB_PORT='`echo PWN"},
	}
	for _, alvo := range guardas {
		for _, c := range casos {
			t.Run(alvo+"/"+c.nome, func(t *testing.T) {
				dir, path, cliente, log := bug29Dir(t)
				sentinela := filepath.Join(dir, "pwn-l39")
				porta := strings.ReplaceAll(c.porta, "echo PWN", "echo PWN >"+sentinela+"; echo")
				msg := strings.ReplaceAll(c.msg, "echo PWN", "echo PWN >"+sentinela+"; echo")
				out, err := makeAlvo(t, dir, alvo, []string{path}, bug29Args(cliente, "DB_PORT="+porta)...)
				require.Error(t, err, out)
				assert.Contains(t, out, msg)
				assert.NoFileExists(t, sentinela, "o valor não pode executar no shell da guarda")
				assert.Empty(t, lerSeExiste(t, log), "nenhum cliente executado")
			})
		}
	}
}

// Catraca: as guardas não interpolam o DB_PORT e ninguém o tira do ambiente.
func TestBUG32_Makefile_CatracaSemInterpolacao(t *testing.T) {
	mk := lerMakefile(t)
	for _, a := range mk.ordem {
		if !strings.HasPrefix(a, "db-guard-") && a != "db-check-env" {
			continue
		}
		for _, l := range mk.regras[a].receita {
			assert.NotContains(t, l, "$(DB_PORT)", "%s interpola o DB_PORT", a)
		}
	}
	for _, macro := range []string{"PRODUCAO_DETECTA", "PORTA_DETECTA_INVALIDA", "PORTA_EXIBIVEL", "MSG_SEC57"} {
		achou := false
		for _, l := range mk.linhas {
			if strings.HasPrefix(l, macro+" =") {
				achou = true
				assert.NotContains(t, l, "$(DB_PORT)", macro)
			}
		}
		assert.True(t, achou, "macro %s não encontrada", macro)
	}
	reUnexport := regexp.MustCompile(`(?m)^\s*unexport\b.*\bDB_PORT\b`)
	assert.False(t, reUnexport.MatchString(mk.texto), "unexport DB_PORT deixaria a guarda sem a porta (vazio = 3306)")
}

func TestDB09_Makefile_AlvosHistoricoCreatedAt(t *testing.T) {
	mk := lerMakefile(t)
	alvos := map[string]string{
		"db-fix-historico-created-at":    "27_alter_senha_historico_created_at_not_null.sql",
		"db-revert-historico-created-at": "27_revert_senha_historico_created_at_not_null.sql",
	}
	for alvo, sql := range alvos {
		t.Run(alvo, func(t *testing.T) {
			r := mk.regras[alvo]
			require.NotNil(t, r)
			assert.True(t, mk.phony[alvo], ".PHONY")
			assert.Equal(t, []string{"db-guard-producao", "db-check-env"}, r.prereqs)
			assert.Equal(t, []string{"$(DB_CLI) $(DB_NAME) < sql/" + sql}, r.receita)
			assert.FileExists(t, filepath.Join(raizProjeto(t), "sql", sql))

			dir, path, cliente, log := bug29Dir(t)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "sql", sql), []byte("-- ARQ-"+alvo+"\n"), 0o644))
			out, err := makeAlvo(t, dir, alvo, []string{path}, bug29Args(cliente)...)
			require.NoError(t, err, out)
			assert.Contains(t, lerSeExiste(t, log), "ARQ-"+alvo)

			dir, path, cliente, log = bug29Dir(t)
			out, err = makeAlvo(t, dir, alvo, []string{path},
				bug29Args(cliente, "DB_HOST=192.168.168.106", "DB_PORT=3306", "DB_TLS_REQUIRED=false")...)
			require.Error(t, err, out)
			assert.Contains(t, out, "e o banco de PRODUCAO")
			assert.Empty(t, lerSeExiste(t, log))
		})
	}
}
