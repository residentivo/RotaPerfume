package config_test

// Lote 37 (BackBrain; o TestBrain completa a cobertura):
//   - BUG-29: MAKEFLAGS/MAKEOVERRIDES/GNUMAKEFLAGS na linha de comando,
//     ENV_ARQUIVO do ambiente, sentinela RP_ENV_PAI e variáveis do make no
//     .env são recusados no parse, antes de qualquer guarda ou cliente;
//   - BUG-28: o db-up roda na receita do db-seed (com -k/-j não roda junto
//     de uma guarda que falhou);
//   - SEC-56/SEC-57: IPv4 numérico não canônico e porta com espaço são
//     recusados sem válvula, com a mensagem própria (Makefile e Go).
// Tudo em cópias do Makefile com cliente mariadb e go falsos (bug27Dir):
// nada conecta em banco.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/testkit/integracao"
)

// bug29Dir: a cópia do bug27Dir com a.env (127.0.0.1/rp_a) e .env
// (127.0.0.2/rp_b). Os argumentos de bug29Args não trazem DB_HOST/DB_NAME:
// quem decide é o arquivo lido.
func bug29Dir(t *testing.T) (dir, path, cliente, log string) {
	t.Helper()
	dir, path, cliente, log = bug27Dir(t, false)
	escrever := func(nome, host, banco string) {
		conteudo := "DB_HOST=" + host + "\nDB_PORT=3306\nDB_NAME=" + banco + "\nDB_USUARIO=u-bug29\nDB_SENHA=s-bug29\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, nome), []byte(conteudo), 0o600))
	}
	escrever("a.env", "127.0.0.1", "rp_a")
	escrever(".env", "127.0.0.2", "rp_b")
	return dir, path, cliente, log
}

func bug29Args(cliente string, extra ...string) []string {
	return append([]string{"DB_TLS_CA=", "DB_CLIENTE_BIN=" + cliente}, extra...)
}

func TestBUG29_Makefile_RecusaNoParse(t *testing.T) {
	casos := []struct {
		nome, alvo string
		env, args  []string
		msg        string
	}{
		{"db-reset MAKEFLAGS=-k", "db-reset", nil, []string{"MAKEFLAGS=-k", "ENV_ARQUIVO=a.env"}, "MAKEFLAGS/MAKEOVERRIDES/GNUMAKEFLAGS na linha de comando"},
		{"db-reset -j2 MAKEFLAGS=-k", "db-reset", nil, []string{"-j2", "MAKEFLAGS=-k", "ENV_ARQUIVO=a.env"}, "MAKEFLAGS/MAKEOVERRIDES/GNUMAKEFLAGS na linha de comando"},
		{"test-integration MAKEFLAGS=-k", "test-integration", nil, []string{"MAKEFLAGS=-k", "ENV_ARQUIVO=a.env"}, "MAKEFLAGS/MAKEOVERRIDES/GNUMAKEFLAGS na linha de comando"},
		{"db-rebuild MAKEOVERRIDES=", "db-rebuild", nil, []string{"MAKEOVERRIDES=", "ENV_ARQUIVO=a.env"}, "MAKEFLAGS/MAKEOVERRIDES/GNUMAKEFLAGS na linha de comando"},
		{"db-reset GNUMAKEFLAGS=-k", "db-reset", nil, []string{"GNUMAKEFLAGS=-k", "ENV_ARQUIVO=a.env"}, "MAKEFLAGS/MAKEOVERRIDES/GNUMAKEFLAGS na linha de comando"},
		{"ENV_ARQUIVO no ambiente", "db-down", []string{"ENV_ARQUIVO=a.env"}, nil, "ENV_ARQUIVO veio do ambiente"},
		{"ENV_ARQUIVO no ambiente com make -e", "db-down", []string{"ENV_ARQUIVO=a.env"}, []string{"-e"}, "ENV_ARQUIVO veio do ambiente"},
		{"ENV_ARQUIVO no ambiente com MAKEFLAGS=e", "db-down", []string{"ENV_ARQUIVO=a.env", "MAKEFLAGS=e"}, nil, "ENV_ARQUIVO veio do ambiente"},
		{"RP_ENV_PAI na linha de comando sem ENV_ARQUIVO", "db-down", nil, []string{"RP_ENV_PAI=a.env"}, "o make pai leu a.env e este sub-make leria .env"},
		{"RP_ENV_PAI no ambiente", "db-down", []string{"RP_ENV_PAI=a.env"}, nil, "o make pai leu a.env e este sub-make leria .env"},
		{"ENV_ARQUIVO com aspas", "db-down", nil, []string{"ENV_ARQUIVO=a'b.env"}, "invalido: um caminho so com"},
		{"ENV_ARQUIVO com espaço", "db-down", nil, []string{"ENV_ARQUIVO=a b.env"}, "invalido: um caminho so com"},
		{"ENV_ARQUIVO com vírgula", "db-down", nil, []string{"ENV_ARQUIVO=a,b.env"}, "invalido: um caminho so com"},
		{"ENV_ARQUIVO vazio", "db-down", nil, []string{"ENV_ARQUIVO="}, "invalido: um caminho so com"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			out, err := makeAlvo(t, dir, c.alvo, append([]string{path}, c.env...), bug29Args(cliente, c.args...)...)
			require.Error(t, err, out)
			assert.Contains(t, out, "BUG-29: ")
			assert.Contains(t, out, c.msg)
			assert.Empty(t, lerSeExiste(t, log), "nenhum cliente/go test antes da recusa")
		})
	}
}

func TestBUG29_Makefile_VariaveisDoMakeNoEnvRecusadas(t *testing.T) {
	for _, linha := range []string{"MAKEFLAGS=-k", "export MAKEOVERRIDES=", "GNUMAKEFLAGS += -k", "MAKEFILES=x.mk", "MAKELEVEL=0", "MAKE=/bin/true", "RP_ENV_PAI=.env", "ENV_ARQUIVO_LIDO=a.env"} {
		t.Run(linha, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			f, err := os.OpenFile(filepath.Join(dir, ".env"), os.O_APPEND|os.O_WRONLY, 0o600)
			require.NoError(t, err)
			_, err = f.WriteString(linha + "\n")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			out, err := makeAlvo(t, dir, "db-down", []string{path}, bug29Args(cliente)...)
			require.Error(t, err, out)
			assert.Contains(t, out, "BUG-29: .env declara")
			assert.Empty(t, lerSeExiste(t, log))
		})
	}
}

// Positivos: -k direto ou MAKEFLAGS no ambiente; todos os sub-makes leem o
// a.env (127.0.0.1/rp_a), nunca o .env (127.0.0.2/rp_b).
func TestBUG29_Makefile_SubMakesLeemOMesmoArquivo(t *testing.T) {
	casos := []struct {
		nome, alvo string
		env, args  []string
	}{
		{"make -k db-reset", "db-reset", nil, []string{"-k"}},
		{"MAKEFLAGS=-k no ambiente", "db-reset", []string{"MAKEFLAGS=-k"}, nil},
		{"make -k -j4 db-rebuild", "db-rebuild", nil, []string{"-k", "-j4"}},
		{"db-seed", "db-seed", nil, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			out, err := makeAlvo(t, dir, c.alvo, append([]string{path}, c.env...), bug29Args(cliente, append(c.args, "ENV_ARQUIVO=a.env")...)...)
			require.NoError(t, err, out)
			texto := lerSeExiste(t, log)
			require.NotEmpty(t, texto)
			for _, l := range strings.Split(strings.TrimSpace(texto), "\n") {
				if strings.Contains(l, " -e ") { // com stdin, o cliente falso grava só a 1a linha do SQL

					assert.Contains(t, l, "-h 127.0.0.1 ", l)
				}
			}
			assert.NotContains(t, texto, "127.0.0.2")
			assert.NotContains(t, texto, "rp_b")
			assert.Contains(t, texto, "rp_a")
		})
	}
	t.Run("test-integration ENV_ARQUIVO=a.env", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "backend", "rotaperfumes-api"), 0o755))
		out, err := makeAlvo(t, dir, "test-integration", []string{path}, bug29Args(cliente, "-k", "ENV_ARQUIVO=a.env")...)
		require.NoError(t, err, out)
		assert.Contains(t, lerSeExiste(t, log), "go test ./...")
	})
	// BUG-29 (achado do TestBrain): as variáveis só de linha de comando não
	// chegam ao ambiente dos filhos (unexport); o sub-make as recebe pelo MAKEFLAGS.
	t.Run("filhos do test-integration não veem ENV_ARQUIVO/RP_ENV_PAI/CONFIRMAR/CONFIRMO_PRODUCAO", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "backend", "rotaperfumes-api"), 0o755))
		envLog := filepath.Join(dir, "env.log")
		scriptGo := "#!/bin/sh\necho \"go $*\" >> '" + log + "'\nenv >> '" + envLog + "'\nexit 0\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "bin-falso", "go"), []byte(scriptGo), 0o755))
		out, err := makeAlvo(t, dir, "test-integration", []string{path}, bug29Args(cliente, "-k", "ENV_ARQUIVO=a.env",
			"CONFIRMAR=rotaperfumes", "CONFIRMO_PRODUCAO=127.0.0.1/rp_a")...)
		require.NoError(t, err, out)
		assert.Contains(t, lerSeExiste(t, log), "go test ./...")
		ambiente := lerSeExiste(t, envLog)
		require.NotEmpty(t, ambiente)
		assert.Contains(t, ambiente, "DB_NAME=rp_a", "o sub-make leu o a.env")
		for _, v := range []string{"ENV_ARQUIVO", "RP_ENV_PAI", "CONFIRMAR", "CONFIRMO_PRODUCAO"} {
			assert.NotRegexp(t, `(?m)^`+v+`=`, ambiente, "%s vazou para o ambiente do go", v)
		}
	})
	t.Run("CONFIRMAR só no ambiente continua recusado no CasaOS", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		out, err := makeAlvo(t, dir, "db-down", []string{path, "CONFIRMAR=rotaperfumes"},
			bug29Args(cliente, "DB_HOST=192.168.168.106", "DB_PORT=3307", "DB_NAME=rotaperfumes", "DB_TLS_REQUIRED=false")...)
		require.Error(t, err, out)
		assert.Contains(t, out, "(fora do loopback): repita com CONFIRMAR=")
		assert.Empty(t, lerSeExiste(t, log))
	})
}

// Todo "$(MAKE)" do Makefile é literal e leva a sentinela.
func TestBUG29_Makefile_SubMakesComSentinela(t *testing.T) {
	mk := lerMakefile(t)
	total := 0
	for _, a := range mk.ordem {
		for _, l := range mk.regras[a].receita {
			n := strings.Count(l, "$(MAKE)")
			total += n
			assert.Equal(t, n, strings.Count(l, "$(MAKE) RP_ENV_PAI='$(ENV_ARQUIVO_LIDO)' "), "%s: sub-make sem sentinela: %s", a, l)
		}
	}
	assert.GreaterOrEqual(t, total, 12, "db-reset (2), db-rebuild (10), db-seed (1), test-integration (1)")
}

func TestBUG28_Makefile_DbUpNaReceitaDoSeed(t *testing.T) {
	mk := lerMakefile(t)
	assert.Equal(t, []string{"db-guard-producao", "db-guard-sem-producao", "db-check-env"}, mk.regras["db-seed"].prereqs)
	assert.Equal(t, "$(MAKE) RP_ENV_PAI='$(ENV_ARQUIVO_LIDO)' db-up", mk.regras["db-seed"].receita[0])
	// Catraca: quem depende do db-guard-sem-producao só tem guardas e db-check-env.
	achou := 0
	for _, a := range mk.ordem {
		r := mk.regras[a]
		if !contemStr(r.prereqs, "db-guard-sem-producao") {
			continue
		}
		achou++
		for _, p := range r.prereqs {
			assert.True(t, strings.HasPrefix(p, "db-guard-") || p == "db-check-env", "%s: pré-requisito %s roda junto das guardas com -k/-j", a, p)
		}
	}
	assert.GreaterOrEqual(t, achou, 4)
}

func contemStr(lista []string, v string) bool {
	for _, i := range lista {
		if i == v {
			return true
		}
	}
	return false
}

func TestBUG28_Makefile_SeedNaProducaoComKNaoCriaNada(t *testing.T) {
	for _, flags := range [][]string{{"-k"}, {"-k", "-j4"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			dir, path, cliente, log := bug27Dir(t, false)
			args := append(append([]string{}, flags...), bug27Args(cliente, "DB_HOST=192.168.168.106", "DB_PORT=3306",
				"CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes")...)
			out, err := makeAlvo(t, dir, "db-seed", []string{path}, args...)
			require.Error(t, err, out)
			assert.Contains(t, out, "SEC-46:")
			assert.Empty(t, lerSeExiste(t, log), "nenhum CREATE/DDL/seed na produção")
		})
	}
	t.Run("loopback: CREATE -> DDL -> seeds", func(t *testing.T) {
		dir, path, cliente, log := bug27Dir(t, false)
		out, err := makeAlvo(t, dir, "db-seed", []string{path}, append([]string{"-k", "-j4"}, bug27Args(cliente, "DB_HOST=127.0.0.1", "DB_PORT=3306")...)...)
		require.NoError(t, err, out)
		assert.Equal(t, bug27Seed, bug27Etapas(lerSeExiste(t, log)))
	})
}

// SEC-56/SEC-57: mensagem própria no db-guard-producao, sem válvula.
func TestSEC56_SEC57_Makefile_MensagemSemValvula(t *testing.T) {
	casos := []struct {
		host, porta, msg string
	}{
		{"0xc0a8a86a", "3307", "SEC-56: DB_HOST=0xc0a8a86a e IPv4 em forma numerica nao canonica"},
		{"127.1", "3306", "SEC-56: DB_HOST=127.1 e IPv4"},
		{"127.0.0.1", "3307 ", "SEC-57: DB_PORT='3307 ' invalida"},
		{"192.168.168.106", "3307 ", "SEC-57: DB_PORT='3307 ' invalida"},
		{"127.0.0.1", "abc", "SEC-57: DB_PORT='abc' invalida"},
		{"localhost", "33 07", "SEC-57: DB_PORT='33 07' invalida"},
	}
	for _, c := range casos {
		for _, confirma := range []bool{false, true} {
			t.Run(c.host+":"+c.porta+map[bool]string{true: "/confirmado"}[confirma], func(t *testing.T) {
				args := []string{"DB_HOST=" + c.host, "DB_PORT=" + c.porta, "DB_NAME=rotaperfumes"}
				if confirma {
					args = append(args, "CONFIRMO_PRODUCAO="+c.host+"/rotaperfumes")
				}
				out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", nil, args...)
				require.Error(t, err, out)
				assert.Contains(t, out, c.msg)
				assert.NotContains(t, out, "e o banco de PRODUCAO")
				assert.NotContains(t, out, "AVISO (DEPLOY-07)")
			})
		}
	}
	t.Run("db-guard-sem-producao e db-guard-integracao mostram SEC-57 e mantêm a recusa", func(t *testing.T) {
		args := []string{"DB_HOST=127.0.0.1", "DB_PORT=3307 "}
		out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-sem-producao", nil, args...)
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-57:")
		assert.Contains(t, out, "SEC-46:")
		out, err = makeAlvo(t, copiaDoMakefile(t), "db-guard-integracao", nil, args...)
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-57:")
		assert.Contains(t, out, "SEC-54: DB_HOST=")
	})
}

func TestSEC56_SEC57_Go(t *testing.T) {
	for _, p := range []string{"3307 ", " 3307", "33 07", "\t3307", "  ", "abc", "+"} {
		err := integracao.ValidarNaoProducao("127.0.0.1", p)
		require.ErrorIs(t, err, integracao.ErrPortaInvalida, "%q", p)
		assert.NotErrorIs(t, err, integracao.ErrBancoProducao)
		assert.Contains(t, err.Error(), "inválida: não é numérico (SEC-57: só dígitos, '+' opcional, sem espaços)")
		errInt := integracao.ValidarPermitidoIntegracao("127.0.0.1", p)
		assert.ErrorIs(t, errInt, integracao.ErrPortaInvalida, "%q", p)
		assert.ErrorIs(t, errInt, integracao.ErrHostNaoPermitido, "%q", p)
	}
	for _, h := range []string{"3232278634", "0XC0A8A86A", "127.000.000.001", "[::ffff:010.1.1.1]"} {
		err := integracao.ValidarNaoProducao(h, "3307")
		require.ErrorIs(t, err, integracao.ErrBancoProducao, h)
		assert.Contains(t, err.Error(), "SEC-56: DB_HOST=")
		assert.Contains(t, err.Error(), "IPv4 em forma numerica nao canonica (decimal/hex/octal/curta): use a.b.c.d")
	}
	assert.ErrorIs(t, integracao.ValidarPermitidoIntegracao("127.000.000.001", "3307"), integracao.ErrHostNaoPermitido,
		"127.000.000.001 não é loopback canônico")
	assert.NoError(t, integracao.ValidarNaoProducao("127.0.0.1", ""))
	assert.NoError(t, integracao.ValidarNaoProducao("127.0.0.1", "+03307"))
}
