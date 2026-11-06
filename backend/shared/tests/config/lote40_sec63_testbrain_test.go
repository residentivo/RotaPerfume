package config_test

// SEC-63 (Lote 40, TestBrain): completa a seção 6 do spec e procura desvios que o
// lote40_sec63_makefile_test.go não cobre: PoC D e os dois K, os três caminhos de
// origem (.env, linha de comando, ambiente), bytes de controle e UTF-8, "%" (curinga
// do $(filter)), make -k/-j, o gate em TODOS os alvos e a paridade Makefile × Go byte
// a byte. Tudo em cópias do Makefile com cliente falso: nada conecta.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/testkit/integracao"
)

// sec63Origem: as três origens de um valor. Em "cmd", o cliente falso vai junto; no
// ambiente, DB_CLIENTE_BIN inválido não pode ser sobrescrito pelo harness.
type sec63Origem struct {
	nome   string
	rodar  func(t *testing.T, alvo, variavel, valor, base string) (out string, err error, dir, log string)
	aceita func(valor string) bool // false = o valor não cabe nesta origem
}

func sec63Origens() []sec63Origem {
	semQuebra := func(v string) bool { return !strings.Contains(v, "\n") }
	sempre := func(string) bool { return true }
	clienteArg := func(variavel, cliente string) []string {
		if variavel == "DB_CLIENTE_BIN" {
			return nil
		}
		return []string{"DB_CLIENTE_BIN=" + cliente}
	}
	return []sec63Origem{
		{"env", func(t *testing.T, alvo, variavel, valor, base string) (string, error, string, string) {
			dir, path, cliente, log := sec63Dir(t, sec63Cred+base+variavel+"="+valor+"\n")
			out, err := makeAlvo(t, dir, alvo, []string{path}, clienteArg(variavel, cliente)...)
			return out, err, dir, log
		}, semQuebra},
		{"cmd", func(t *testing.T, alvo, variavel, valor, base string) (string, error, string, string) {
			dir, path, cliente, log := sec63Dir(t, sec63Cred+base)
			out, err := makeAlvo(t, dir, alvo, []string{path}, append(clienteArg(variavel, cliente), variavel+"="+valor)...)
			return out, err, dir, log
		}, sempre},
		{"ambiente", func(t *testing.T, alvo, variavel, valor, base string) (string, error, string, string) {
			// O valor do .env ganha do ambiente: a variável testada sai do arquivo.
			var arquivo strings.Builder
			for _, l := range strings.SplitAfter(sec63Cred+base, "\n") {
				if !strings.HasPrefix(l, variavel+"=") {
					arquivo.WriteString(l)
				}
			}
			dir, path, cliente, log := sec63Dir(t, arquivo.String())
			out, err := makeAlvo(t, dir, alvo, []string{path, variavel + "=" + valor}, clienteArg(variavel, cliente)...)
			return out, err, dir, log
		}, sempre},
	}
}

// sec63RecusaEm confere a recusa nas três origens: mensagem, sentinela e cliente.
func sec63RecusaEm(t *testing.T, alvo, variavel, valor, base, msg string) {
	t.Helper()
	for _, o := range sec63Origens() {
		if !o.aceita(valor) {
			continue
		}
		t.Run(o.nome, func(t *testing.T) {
			out, err, dir, log := o.rodar(t, alvo, variavel, valor, base)
			require.Error(t, err, out)
			assert.Contains(t, out, msg)
			assert.NoFileExists(t, filepath.Join(dir, "pwn"), "o valor não pode executar")
			assert.Empty(t, lerSeExiste(t, log), "nenhum cliente nem go chamado")
		})
	}
}

// PoCs do spec que faltavam, com o alvo exato de cada linha da tabela.
func TestSEC63TB_PoCs(t *testing.T) {
	prod := "DB_HOST=192.168.168.106\nDB_PORT=3306\nDB_TLS_REQUIRED=false\nDB_NAME=rotaperfumes\n"
	casa := "DB_HOST=192.168.168.106\nDB_PORT=3307\nDB_TLS_REQUIRED=false\nDB_NAME=rotaperfumes\n"
	loop := "DB_HOST=127.0.0.1\n"
	casos := []struct {
		poc, alvo, variavel, valor, base, msg string
	}{
		// D divergiu do previsto: o -h fica DENTRO do SQL do -e (aspas), não no argv.
		// Com o parse, nem chega ao cliente.
		{"D", "db-down", "DB_NAME", "rotaperfumes -h 192.168.168.106 -P 3306", loop, "SEC-63: DB_NAME invalido"},
		{"D", "db-create", "DB_NAME", "rotaperfumes -h 192.168.168.106 -P 3306", loop, "SEC-63: DB_NAME invalido"},
		{"E", "db-create", "DB_USUARIO", "u -h 192.168.168.106", loop, "SEC-63: DB_USUARIO invalido"},
		{"F", "db-guard-producao", "CONFIRMO_PRODUCAO", `x"; touch pwn; exit 0; "`, prod, "SEC-63: CONFIRMO_PRODUCAO invalido"},
		{"G", "db-guard-drop", "CONFIRMAR", `x"; touch pwn; exit 0; "`, casa, "SEC-63: CONFIRMAR invalido"},
		{"G", "db-down", "CONFIRMAR", `x"; touch pwn; exit 0; "`, casa, "SEC-63: CONFIRMAR invalido"},
		{"H", "db-guard-sem-producao", "DB_HOST", `127.0.0.1"; touch pwn; "`, "", "SEC-63: DB_HOST invalido"},
		{"H", "db-guard-integracao", "DB_HOST", "127.0.0.1`touch pwn`", "", "SEC-63: DB_HOST invalido"},
		{"J", "db-guard-drop", "DB_NAME", `x"; touch pwn; "`, loop, "SEC-63: DB_NAME invalido"},
		{"J", "db-create", "DB_NAME", "rotaperfumes;DROP", loop, "SEC-63: DB_NAME invalido"},
		{"L", "db-check-env", "DB_CLIENTE_BIN", "/tmp/a(b/mariadb", loop, "SEC-63: DB_CLIENTE_BIN invalido"},
		// Os dois K não têm charset no parse: o shell só os lê do ambiente.
		{"K1", "db-check-env", "DB_TLS_REQUIRED", `x"; touch pwn; "`, loop + "DB_NAME=rp_x\n", "invalido: use true ou false"},
		{"K1", "db-check-env", "DB_TLS_REQUIRED", "x`touch pwn`", loop + "DB_NAME=rp_x\n", "invalido: use true ou false"},
		{"K2", "db-check-env", "DB_CLIENTE", `x"; touch pwn; "`, loop + "DB_NAME=rp_x\n", "CHORE-13: DB_CLIENTE nao existe mais"},
		{"K2", "db-check-env", "DB_CLIENTE", "x`touch pwn`", loop + "DB_NAME=rp_x\n", "CHORE-13: DB_CLIENTE nao existe mais"},
		{"K2", "db-create", "DB_CLIENTE", `x"; touch pwn; "`, loop + "DB_NAME=rp_x\n", "CHORE-13: DB_CLIENTE nao existe mais"},
	}
	for _, c := range casos {
		t.Run(c.poc+"/"+c.alvo+"/"+c.variavel, func(t *testing.T) {
			sec63RecusaEm(t, c.alvo, c.variavel, c.valor, c.base, c.msg)
		})
	}
}

// Controle, Unicode e "%": o RP_TIRA_CHARS opera por byte e nada sobra de fora.
func TestSEC63TB_BytesForaDoCharset(t *testing.T) {
	valores := []struct{ nome, valor string }{
		{"tab no meio", "a\tb"},
		{"tab com -h", "a\t-h\tb"},
		{"VT no meio", "a\vb"},
		{"FF no meio", "a\fb"},
		{"quebra de linha", "a\nb"},
		{"quebra com -h", "a\n-h\nb"},
		{"acento", "ação"},
		{"NBSP", "a b"},
		{"dígito largo", "１"},
		{"byte UTF-8 inválido", "a\xffb"},
		{"continuação solta", "a\xa1"},
		{"porcento", "a%b"},
		{"porcento só", "%"},
		{"hífen e porcento", "-%"},
		{"cerquilha escapada", `a\#b`},
		{"vírgula", "a,b"},
		{"parêntese de fechar", "a)b"},
	}
	for _, variavel := range []string{"DB_HOST", "DB_NAME", "DB_USUARIO", "DB_TLS_CA", "DB_CLIENTE_BIN", "CONFIRMAR", "CONFIRMO_PRODUCAO"} {
		for _, v := range valores {
			t.Run(variavel+"/"+v.nome, func(t *testing.T) {
				sec63RecusaEm(t, "db-create", variavel, v.valor, "", "SEC-63: "+variavel+" invalido")
			})
		}
	}
	t.Run("CR na linha de comando não passa pela limpeza do .env", func(t *testing.T) {
		for _, variavel := range []string{"DB_HOST", "DB_NAME", "DB_USUARIO", "DB_TLS_CA"} {
			sec63Recusa(t, "db-create", sec63Cred, "SEC-63: "+variavel+" invalido", variavel+"=a\r")
		}
	})
	t.Run("locale UTF-8 não muda o resultado", func(t *testing.T) {
		dir, path, cliente, log := sec63Dir(t, sec63Cred)
		out, err := makeAlvo(t, dir, "db-create", []string{path, "LC_ALL=C.UTF-8"}, "DB_CLIENTE_BIN="+cliente, "DB_NAME=ação")
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-63: DB_NAME invalido")
		assert.Empty(t, lerSeExiste(t, log))
	})
	for _, variavel := range []string{"DB_HOST", "DB_USUARIO", "DB_TLS_CA", "DB_CLIENTE_BIN"} {
		t.Run("hífen sozinho/"+variavel, func(t *testing.T) {
			sec63RecusaEm(t, "db-create", variavel, "-", "", "SEC-63: "+variavel+" invalido")
		})
	}
}

// O que o $(strip) e a limpeza de CR do .env tiram antes da validação não vira opção.
func TestSEC63TB_LimpezaDoEnv(t *testing.T) {
	argv := func(t *testing.T, env string) string {
		t.Helper()
		dir, path, cliente, log := sec63Dir(t, sec63Cred+env)
		out, err := makeAlvo(t, dir, "db-create", []string{path}, "DB_CLIENTE_BIN="+cliente)
		require.NoError(t, err, out)
		return lerSeExiste(t, log)
	}
	for nome, fim := range map[string]string{"tab": "\t", "espaço": "  ", "VT": "\v", "FF": "\f", "CR": "\r"} {
		t.Run(nome+" no fim do DB_HOST some", func(t *testing.T) {
			assert.Contains(t, argv(t, "DB_NAME=rp_x\nDB_HOST=127.0.0.1"+fim+"\n"), "[-h][127.0.0.1][-P]")
		})
	}
	t.Run("CR no meio não separa argumento", func(t *testing.T) {
		assert.Contains(t, argv(t, "DB_NAME=rp_x\nDB_TLS_REQUIRED=false\nDB_HOST=127.0.0.1\r-h\n"), "[-h][127.0.0.1-h][-P]")
	})
	t.Run("CR seguido de espaço e -h recusado", func(t *testing.T) {
		sec63Recusa(t, "db-create", sec63Cred+"DB_HOST=127.0.0.1\r -h 192.168.168.106\n", "SEC-63: DB_HOST invalido")
	})
	t.Run(".env inteiro com CRLF continua aceito", func(t *testing.T) {
		dir, path, cliente, log := sec63Dir(t, "DB_USUARIO=u\r\nDB_SENHA=s\r\nDB_HOST=127.0.0.1\r\nDB_PORT=3307\r\nDB_NAME=rp_x\r\nDB_TLS_CA=ca.pem\r\nDB_TLS_REQUIRED=true\r\nCONFIRMAR=rp_x\r\nCONFIRMO_PRODUCAO=127.0.0.1/rp_x\r\n")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ca.pem"), []byte("x"), 0o644))
		out, err := makeAlvo(t, dir, "db-create", []string{path}, "DB_CLIENTE_BIN="+cliente)
		require.NoError(t, err, out)
		chamada := lerSeExiste(t, log)
		assert.Contains(t, chamada, "[-u][u][-h][127.0.0.1][-P][3307]")
		assert.Contains(t, chamada, "[--ssl-ca=ca.pem]")
		assert.NotContains(t, chamada, "\r")
	})
	t.Run("DB_NAME só de espaços no .env vira vazio", func(t *testing.T) {
		sec63Recusa(t, "db-create", sec63Cred+"DB_NAME=   \n", "SEC-63: DB_NAME vazio")
	})
	t.Run("DB_USUARIO só de espaços no .env cai no SEC-11", func(t *testing.T) {
		sec63Recusa(t, "db-create", "DB_USUARIO=   \nDB_SENHA=s\nDB_NAME=rp_x\nDB_HOST=127.0.0.1\n", "SEC-11:")
	})
	// Desvio registrado do BackBrain: só de espaços no .env vira vazio, e não SEC-63.
	t.Run("DB_HOST só de espaços no .env cai no SEC-42", func(t *testing.T) {
		sec63Recusa(t, "db-create", sec63Cred+"DB_HOST=   \nDB_NAME=rp_x\n", "SEC-42:")
	})
	t.Run("DB_HOST só de espaços com TLS opcional usa localhost", func(t *testing.T) {
		assert.Contains(t, argv(t, "DB_HOST=   \nDB_NAME=rp_x\nDB_TLS_REQUIRED=false\n"), "[-h][localhost][-P]")
	})
	t.Run("DB_HOST vazio não conta como loopback no DROP", func(t *testing.T) {
		sec63Recusa(t, "db-down", sec63Cred+"DB_HOST=\nDB_NAME=rotaperfumes\nDB_TLS_REQUIRED=false\n", "repita com CONFIRMAR=rotaperfumes")
	})
	// O make descarta o espaço depois do "=" da linha de comando: fica vazio, como no .env.
	t.Run("DB_HOST só de espaços na linha de comando cai no SEC-42", func(t *testing.T) {
		sec63Recusa(t, "db-create", sec63Cred+"DB_NAME=rp_x\n", "SEC-42:", "DB_HOST=   ")
	})
	t.Run("DB_CLIENTE_BIN vazio no .env usa o mariadb do PATH", func(t *testing.T) {
		dir, path, cliente, log := sec63Dir(t, sec63Cred+"DB_CLIENTE_BIN=\nDB_NAME=rp_x\nDB_HOST=127.0.0.1\n")
		require.NoError(t, os.Rename(cliente, filepath.Join(filepath.Dir(cliente), "mariadb")))
		out, err := makeAlvo(t, dir, "db-create", []string{path})
		require.NoError(t, err, out)
		assert.Contains(t, lerSeExiste(t, log), "CREATE DATABASE")
	})
	t.Run("CONFIRMO_PRODUCAO com CR na linha de comando não confirma", func(t *testing.T) {
		dir, path, cliente, log := sec63Dir(t, sec63Cred+"DB_HOST=192.168.168.106\nDB_PORT=3306\nDB_TLS_REQUIRED=false\nDB_NAME=rotaperfumes\n")
		out, err := makeAlvo(t, dir, "db-create", []string{path}, "DB_CLIENTE_BIN="+cliente, "CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes\r")
		require.Error(t, err, out)
		assert.Contains(t, out, "e o banco de PRODUCAO")
		assert.Empty(t, lerSeExiste(t, log))
	})
}

// DB_PORT não tem charset do SEC-63: o SEC-57 recusa antes de o -P receber o valor.
func TestSEC63TB_PortaContinuaNoSEC57(t *testing.T) {
	for nome, valor := range map[string]string{
		"com -h":   "3306 -h 192.168.168.106",
		"aspas":    `"; touch pwn; "`,
		"crase":    "`touch pwn`",
		"tab":      "33\t06",
		"porcento": "33%",
	} {
		for _, alvo := range []string{"db-create", "db-check-env"} {
			t.Run(alvo+"/"+nome, func(t *testing.T) {
				dir, path, cliente, log := sec63Dir(t, sec63Cred+"DB_HOST=127.0.0.1\nDB_NAME=rp_x\nDB_PORT="+valor+"\n")
				out, err := makeAlvo(t, dir, alvo, []string{path}, "DB_CLIENTE_BIN="+cliente)
				require.Error(t, err, out)
				assert.Contains(t, out, "SEC-57:")
				assert.NoFileExists(t, filepath.Join(dir, "pwn"))
				assert.Empty(t, lerSeExiste(t, log))
			})
		}
	}
}

// O $(error) é no parse: -k e -j não rodam nenhum pré-requisito nem outro goal.
func TestSEC63TB_KeJNaoExecutamNada(t *testing.T) {
	casos := []struct {
		nome string
		args []string
		env  []string
	}{
		{"-k db-create", []string{"-k", "db-create"}, nil},
		{"-j4 db-create", []string{"-j4", "db-create"}, nil},
		{"-k -j4 vários", []string{"-k", "-j4", "db-create", "db-down", "db-seed", "db-reset", "db-rebuild"}, nil},
		{"-k -j4 goal sem banco antes", []string{"-k", "-j4", "test", "lint", "db-reset"}, nil},
		{"-k -j4 goal sem banco depois", []string{"-k", "-j4", "db-reset", "test", "lint"}, nil},
		{"-k -j4 test INTEGRATION=1", []string{"-k", "-j4", "test", "INTEGRATION=1"}, nil},
		{"-k test-integration", []string{"-k", "test-integration"}, nil},
		{"-i db-create", []string{"-i", "db-create"}, nil},
		{"MAKEFLAGS no ambiente", []string{"db-create", "lint"}, []string{"MAKEFLAGS=-k -j4"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir, path, cliente, log := sec63Dir(t, sec63Cred+`DB_HOST=127.0.0.1"; touch pwn; "`+"\n")
			out, err := makeAlvo(t, dir, c.args[0], append([]string{path}, c.env...), append(c.args[1:], "DB_CLIENTE_BIN="+cliente)...)
			require.Error(t, err, out)
			assert.Contains(t, out, "SEC-63: DB_HOST invalido")
			assert.NoFileExists(t, filepath.Join(dir, "pwn"))
			assert.Empty(t, lerSeExiste(t, log), "nem cliente nem go")
		})
	}
}

func sec63ListaSemBanco(t *testing.T, mk *makefile) []string {
	t.Helper()
	for _, l := range mk.linhas {
		if resto, ok := strings.CutPrefix(l, "override RP_ALVOS_SEM_BANCO :="); ok {
			lista := strings.Fields(resto)
			require.Greater(t, len(lista), 20, "a continuação de linha foi lida")
			return lista
		}
	}
	require.Fail(t, "RP_ALVOS_SEM_BANCO não encontrada")
	return nil
}

// Gate em todos os alvos reais: os da lista passam com o .env inválido (-n, nada roda)
// e todo o resto recusa. Alvo novo cai aqui sem ninguém lembrar de acrescentar.
func TestSEC63TB_GateEmTodosOsAlvos(t *testing.T) {
	mk := lerMakefile(t)
	lista := sec63ListaSemBanco(t, mk)
	semBanco := map[string]bool{}
	for _, a := range lista {
		semBanco[a] = true
	}
	ruim := sec63Cred + "DB_HOST=a b\nDB_NAME=c d\n"
	recusados := 0
	for _, a := range mk.ordem {
		if semBanco[a] {
			t.Run("passa/"+a, func(t *testing.T) {
				dir, path, _, log := sec63Dir(t, ruim)
				out, err := makeAlvo(t, dir, "-n", []string{path}, a)
				require.NoError(t, err, out)
				assert.NotContains(t, out, "SEC-63")
				assert.Empty(t, lerSeExiste(t, log), "-n não executa nada")
			})
			continue
		}
		recusados++
		t.Run("recusa/"+a, func(t *testing.T) {
			dir, path, _, log := sec63Dir(t, ruim)
			out, err := makeAlvo(t, dir, "-n", []string{path}, a)
			require.Error(t, err, out)
			assert.Contains(t, out, "SEC-63: DB_HOST invalido")
			assert.Empty(t, lerSeExiste(t, log))
		})
	}
	assert.Greater(t, recusados, 40)

	t.Run("recusa/INTEGRATION=1 em cada alvo de teste", func(t *testing.T) {
		for _, a := range []string{"test", "test-shared", "test-api", "cover-shared", "cover-api", "cover-combinada", "arch-check"} {
			dir, path, _, log := sec63Dir(t, ruim)
			out, err := makeAlvo(t, dir, "-n", []string{path}, a, "INTEGRATION=1")
			require.Error(t, err, a+": "+out)
			assert.Contains(t, out, "SEC-63: DB_HOST invalido", a)
			assert.Empty(t, lerSeExiste(t, log))
		}
	})
	t.Run("passa/INTEGRATION só no ambiente ou diferente de 1", func(t *testing.T) {
		dir, path, _, _ := sec63Dir(t, ruim)
		out, err := makeAlvo(t, dir, "-n", []string{path, "INTEGRATION=1"}, "test")
		require.NoError(t, err, out)
		out, err = makeAlvo(t, dir, "-n", []string{path}, "test", "INTEGRATION=0")
		require.NoError(t, err, out)
	})
	t.Run("recusa/caminho alternativo do goal", func(t *testing.T) {
		dir, path, _, _ := sec63Dir(t, ruim)
		out, err := makeAlvo(t, dir, "-n", []string{path}, "./db-create")
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-63")
	})
	for nome, args := range map[string][]string{"recusa/ENV_ARQUIVO no db-create": {"db-create"}, "recusa/ENV_ARQUIVO no test-integration": {"test-integration"}} {
		t.Run(nome, func(t *testing.T) {
			dir, path, _, log := sec63Dir(t, sec63Cred+"DB_HOST=127.0.0.1\nDB_NAME=rp_x\n")
			require.NoError(t, os.WriteFile(filepath.Join(dir, "ruim.env"), []byte(ruim), 0o600))
			out, err := makeAlvo(t, dir, args[0], []string{path}, "ENV_ARQUIVO=ruim.env")
			require.Error(t, err, out)
			assert.Contains(t, out, "SEC-63: DB_HOST invalido")
			assert.Empty(t, lerSeExiste(t, log))
		})
	}
	t.Run("passa/ENV_ARQUIVO inválido em alvo sem banco", func(t *testing.T) {
		dir, path, _, _ := sec63Dir(t, sec63Cred)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ruim.env"), []byte(ruim), 0o600))
		out, err := makeAlvo(t, dir, "-n", []string{path}, "test", "ENV_ARQUIVO=ruim.env")
		require.NoError(t, err, out)
	})
}

// Quem usa o cliente precisa estar fora da lista E atrás das guardas, mesmo que o
// nome não comece com db-.
func TestSEC63TB_ClienteSoEmAlvoGuardado(t *testing.T) {
	mk := lerMakefile(t)
	semBanco := map[string]bool{}
	for _, a := range sec63ListaSemBanco(t, mk) {
		semBanco[a] = true
	}
	var fecho func(a string, visto map[string]bool)
	fecho = func(a string, visto map[string]bool) {
		if visto[a] {
			return
		}
		visto[a] = true
		if r := mk.regras[a]; r != nil {
			for _, p := range r.prereqs {
				fecho(p, visto)
			}
		}
	}
	comCliente, comBinario := 0, 0
	for _, a := range mk.ordem {
		r := mk.regras[a]
		for _, l := range r.receita {
			if strings.Contains(l, "$(DB_CLI)") || strings.Contains(l, "$(MYSQL_OPTS)") {
				comCliente++
				visto := map[string]bool{}
				fecho(a, visto)
				assert.False(t, semBanco[a], "%s chama o cliente", a)
				assert.True(t, visto["db-check-env"], "%s chama o cliente sem db-check-env", a)
				assert.True(t, visto["db-guard-producao"], "%s chama o cliente sem db-guard-producao", a)
			}
			if strings.Contains(l, "go run") || strings.Contains(l, "bin/rotaperfumes-api") && !strings.Contains(l, "go build") {
				comBinario++
				assert.False(t, semBanco[a], "%s roda código que conecta", a)
			}
		}
	}
	assert.Greater(t, comCliente, 20)
	assert.Greater(t, comBinario, 5)
}

// Paridade byte a byte: Makefile, ValidarFormaDB e a lista do spec dizem o mesmo sobre
// cada byte (a amostra do lote40_sec63_makefile_test.go não varre tudo).
func TestSEC63TB_ParidadeByteAByte(t *testing.T) {
	alfanum := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	permitidos := map[string]string{
		"DB_HOST":    alfanum + "._-:[]",
		"DB_NAME":    alfanum + "_",
		"DB_USUARIO": alfanum + "_.-",
	}
	dir, path, cliente, _ := sec63Dir(t, sec63Cred)
	makeAceita := func(variavel, valor string) bool {
		// Na linha de comando, "$" é expansão do make: "$$" é o "$" literal.
		arg := variavel + "=" + strings.ReplaceAll(valor, "$", "$$")
		out, _ := makeAlvo(t, dir, "db-guard-sem-producao", []string{path}, "DB_CLIENTE_BIN="+cliente, arg)
		return !strings.Contains(out, "SEC-63")
	}
	goAceita := func(variavel, valor string) bool {
		a := map[string]string{"DB_HOST": "h", "DB_NAME": "n", "DB_USUARIO": "u"}
		a[variavel] = valor
		return integracao.ValidarFormaDB(a["DB_HOST"], a["DB_NAME"], a["DB_USUARIO"]) == nil
	}
	type caso struct {
		variavel, valor string
		esperado        bool
	}
	var casos []caso
	for variavel, ok := range permitidos {
		for b := 1; b <= 255; b++ {
			s := string([]byte{byte(b)})
			casos = append(casos, caso{variavel, "a" + s + "b", strings.Contains(ok, s)})
		}
		for _, s := range []string{"-", ".", "_", ":", "[", "]"} {
			inicialOk := strings.Contains(ok, s) && s != "-"
			casos = append(casos, caso{variavel, s + "a", inicialOk})
		}
		casos = append(casos, caso{variavel, "a-", strings.Contains(ok, "-")})
	}
	var (
		mu         sync.Mutex
		divergente []string
		wg         sync.WaitGroup
	)
	vagas := make(chan struct{}, 8)
	for _, c := range casos {
		wg.Add(1)
		vagas <- struct{}{}
		go func() {
			defer func() { <-vagas; wg.Done() }()
			m, g := makeAceita(c.variavel, c.valor), goAceita(c.variavel, c.valor)
			if m != c.esperado || g != c.esperado {
				mu.Lock()
				defer mu.Unlock()
				divergente = append(divergente, c.variavel+"="+strings.ToValidUTF8(strings.ReplaceAll(c.valor, "\n", `\n`), "?"))
			}
		}()
	}
	wg.Wait()
	assert.Empty(t, divergente)
}

func TestSEC63TB_Go_ConfigDoAmbiente(t *testing.T) {
	base := func(extra map[string]string) {
		vars := map[string]string{"DB_HOST": "127.0.0.1", "DB_PORT": "3307", "DB_NAME": "rp_x"}
		for k, v := range extra {
			vars[k] = v
		}
		sec63AmbienteGo(t, vars)
	}
	for _, variavel := range []string{"DB_HOST", "DB_NAME", "DB_USUARIO"} {
		for nome, valor := range map[string]string{
			"tab": "a\tb", "tab final": "a\t", "CR final": "a\r", "LF final": "a\n", "LF com -h": "a\n-h\nb",
			"acento": "ação", "NBSP": "a b", "UTF-8 inválido": "a\xffb", "porcento": "a%b", "espaço inicial": " zq9",
		} {
			t.Run(variavel+"/"+nome, func(t *testing.T) {
				base(map[string]string{variavel: valor})
				_, err := integracao.ConfigDoAmbiente()
				require.ErrorIs(t, err, integracao.ErrDBValorInvalido)
				assert.Contains(t, err.Error(), variavel)
				assert.NotContains(t, err.Error(), valor)
			})
		}
	}
	t.Run("a forma vem antes da guarda de produção", func(t *testing.T) {
		base(map[string]string{"DB_HOST": " 192.168.168.106 ", "DB_PORT": "3306"})
		_, err := integracao.ConfigDoAmbiente()
		require.ErrorIs(t, err, integracao.ErrDBValorInvalido)
		assert.NotErrorIs(t, err, integracao.ErrBancoProducao)
	})
	t.Run("sem credenciais continua pulando, mesmo com valor ruim", func(t *testing.T) {
		ambienteIntegracao(t, map[string]string{"INTEGRATION": "1", "DB_HOST": "a b"})
		_, err := integracao.ConfigDoAmbiente()
		require.ErrorIs(t, err, integracao.ErrSemCredenciais)
	})
	t.Run("vazio no ambiente usa o padrão antes de validar", func(t *testing.T) {
		base(map[string]string{"DB_HOST": "", "DB_NAME": ""})
		cfg, err := integracao.ConfigDoAmbiente()
		require.NoError(t, err)
		assert.Equal(t, "localhost", cfg.DBHost)
		assert.Equal(t, "rotaperfumes", cfg.DBName)
	})
	t.Run("host com porta embutida não engana a guarda", func(t *testing.T) {
		base(map[string]string{"DB_HOST": "192.168.168.106:3306", "DB_PORT": "3307"})
		_, err := integracao.ConfigDoAmbiente()
		require.ErrorIs(t, err, integracao.ErrBancoProducao)
	})
	t.Run("Config: valor inválido é fatal, não pula", func(t *testing.T) {
		base(map[string]string{"DB_NAME": "x@tcp(192.0.2.1:3306)/y"})
		falso := &tbFalso{TB: t}
		assert.Nil(t, integracao.Config(falso))
		assert.False(t, falso.pulou)
		assert.Contains(t, falso.msg, "DB_NAME")
		assert.NotContains(t, falso.msg, "192.0.2.1")
	})
}

func TestSEC63TB_Go_ValidarFormaDB(t *testing.T) {
	casos := []struct {
		nome, host, banco, usuario, erroEm string
	}{
		{"todos válidos", "localhost", "rp", "u", ""},
		{"IPv6", "[::1]", "RP_1", "u.a-b_c", ""},
		{"host vazio", "", "rp", "u", "DB_HOST"},
		{"nome vazio", "h", "", "u", "DB_NAME"},
		{"usuário vazio", "h", "rp", "", "DB_USUARIO"},
		{"ordem: host primeiro", "a b", "c d", "e f", "DB_HOST"},
		{"ordem: nome antes do usuário", "h", "c d", "e f", "DB_NAME"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			err := integracao.ValidarFormaDB(c.host, c.banco, c.usuario)
			if c.erroEm == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, integracao.ErrDBValorInvalido)
			assert.Contains(t, err.Error(), c.erroEm)
		})
	}
}

// A senha fica antes do endereço no DSN: nem com "@tcp(...)/" ela troca o Addr (por isso
// ela não precisa de charset no ValidarFormaDB).
func TestSEC63TB_Go_SenhaNaoTrocaEndereco(t *testing.T) {
	for _, senha := range []string{"x@tcp(192.0.2.1:3306)/y", "a/b@c", "a?tls=false&x=", "a:b"} {
		t.Run(senha, func(t *testing.T) {
			sec63AmbienteGo(t, map[string]string{"DB_HOST": "127.0.0.1", "DB_PORT": "3307", "DB_NAME": "rp_x", "DB_SENHA": senha})
			cfg, err := integracao.ConfigDoAmbiente()
			require.NoError(t, err)
			parsed, err := mysql.ParseDSN(cfg.DSN())
			require.NoError(t, err)
			assert.Equal(t, "127.0.0.1:3307", parsed.Addr)
			assert.Equal(t, "rp_x", parsed.DBName)
		})
	}
}

// sec63Desligou roda db-up e devolve a saída e o que chegou ao cliente falso.
func sec63Desligou(t *testing.T, env []string, args []string, base string) (saida, chamadas string) {
	t.Helper()
	dir, path, cliente, log := sec63Dir(t, sec63Cred+base)
	out, _ := makeAlvo(t, dir, "db-up", append([]string{path}, env...), append(args, "DB_CLIENTE_BIN="+cliente)...)
	return out, lerSeExiste(t, log)
}

// Achado do TestBrain, corrigido: com MAKECMDGOALS ou .DEFAULT_GOAL de fora, ou as
// variáveis do gate do SEC-63 na linha de comando, o cliente não pode ser chamado.
// É só isso que o teste cobre. O limite é o do Makefile: as guardas e o gate protegem
// contra engano e contra o conteúdo do .env, não contra o operador; MAKEFILES no
// ambiente, -f, -o, --eval, SHELL= e outras variáveis internas digitadas na linha de
// comando (ou no ambiente com -e) ficam fora do modelo.
func TestSEC63TB_GateNaoDesligaDeFora(t *testing.T) {
	base := "DB_HOST=127.0.0.1\nDB_NAME=rotaperfumes -h 192.168.168.106 -P 3306\n"
	const msgGoals = "SEC-63: MAKECMDGOALS veio de fora do make"
	const msgNome = "SEC-63: DB_NAME invalido"
	casos := []struct {
		nome, msg string
		env, args []string
	}{
		{"MAKECMDGOALS no ambiente", msgGoals, []string{"MAKECMDGOALS=help"}, nil},
		{"MAKECMDGOALS no ambiente com -e", msgGoals, []string{"MAKECMDGOALS=help"}, []string{"-e"}},
		{"MAKECMDGOALS na linha de comando", msgGoals, nil, []string{"MAKECMDGOALS=help"}},
		{"MAKECMDGOALS pelo MAKEFLAGS do ambiente", msgGoals, []string{"MAKEFLAGS=MAKECMDGOALS=help"}, nil},
		{"MAKECMDGOALS pelo --eval", msgGoals, nil, []string{"--eval=MAKECMDGOALS=help"}},
		{"RP_VALIDAR_BANCO na linha de comando", msgNome, nil, []string{"RP_VALIDAR_BANCO="}},
		{"RP_ALVOS_SEM_BANCO na linha de comando", msgNome, nil, []string{"RP_ALVOS_SEM_BANCO=db-up db-create db-guard-producao db-check-env"}},
		{"RP_CHARS_NOME na linha de comando", msgNome, nil, []string{"RP_CHARS_NOME=r o t a p e f u m s - h 1 9 2 . 6 8 P 3 0"}},
		{"RP_CHARS_ALFANUM na linha de comando", msgNome, nil, []string{"RP_CHARS_ALFANUM="}},
		{"RP_TIRA_CHARS na linha de comando", msgNome, nil, []string{"RP_TIRA_CHARS="}},
		{"INTEGRACAO_PELO_MAKE na linha de comando", msgNome, nil, []string{"INTEGRACAO_PELO_MAKE="}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, chamadas := sec63Desligou(t, c.env, c.args, base)
			assert.Empty(t, chamadas, "gate desligado: o cliente foi chamado")
			assert.Contains(t, out, c.msg)
		})
	}
	// O pior caso: sem o charset, as aspas do .env passam pelo DEPLOY-07 (PoC F).
	t.Run("MAKECMDGOALS no ambiente e CONFIRMO_PRODUCAO com aspas no .env", func(t *testing.T) {
		dir, path, cliente, log := sec63Dir(t, sec63Cred+"DB_HOST=192.168.168.106\nDB_PORT=3306\nDB_NAME=rotaperfumes\nDB_TLS_REQUIRED=false\nCONFIRMO_PRODUCAO=x\"; exit 0; \"\n")
		out, err := makeAlvo(t, dir, "db-create", []string{path, "MAKECMDGOALS=help"}, "DB_CLIENTE_BIN="+cliente)
		require.Error(t, err, out)
		assert.Contains(t, out, msgGoals)
		assert.Empty(t, lerSeExiste(t, log), "a guarda de produção foi desviada")
	})
	t.Run("INTEGRATION=1 na linha de comando continua validando", func(t *testing.T) {
		dir, path, _, log := sec63Dir(t, sec63Cred+base)
		out, err := makeAlvo(t, dir, "test", []string{path}, "INTEGRATION=1", "INTEGRACAO_PELO_MAKE=")
		require.Error(t, err, out)
		assert.Contains(t, out, msgNome)
		assert.Empty(t, lerSeExiste(t, log), "nenhum go test nem cliente")
	})
	// Sem goal o gate não valida nada: .DEFAULT_GOAL de fora escolheria o alvo.
	semGoal := func(t *testing.T, env []string, args ...string) (string, string, error) {
		t.Helper()
		dir, path, cliente, log := sec63Dir(t, sec63Cred+base)
		cmd := exec.Command("make", append([]string{"-s", "DB_CLIENTE_BIN=" + cliente}, args...)...)
		cmd.Dir = dir
		cmd.Env = append([]string{path, "HOME=" + dir, "LC_ALL=C"}, env...)
		out, err := cmd.CombinedOutput()
		return string(out), lerSeExiste(t, log), err
	}
	for _, c := range []struct {
		nome string
		env  []string
		args []string
	}{
		{"na linha de comando", nil, []string{".DEFAULT_GOAL=db-create"}},
		{"no ambiente com -e", []string{".DEFAULT_GOAL=db-create"}, []string{"-e"}},
		{"pelo --eval", nil, []string{"--eval=.DEFAULT_GOAL=db-create"}},
		{"pelo MAKEFLAGS do ambiente", []string{"MAKEFLAGS=.DEFAULT_GOAL=db-create"}, nil},
	} {
		t.Run(".DEFAULT_GOAL "+c.nome, func(t *testing.T) {
			out, chamadas, err := semGoal(t, c.env, c.args...)
			require.Error(t, err, out)
			assert.Contains(t, out, "SEC-63: .DEFAULT_GOAL veio de fora do make")
			assert.Empty(t, chamadas)
		})
	}
	t.Run(".DEFAULT_GOAL só no ambiente, sem -e, não vale: cai no help", func(t *testing.T) {
		out, chamadas, err := semGoal(t, []string{".DEFAULT_GOAL=db-create"})
		require.NoError(t, err, out)
		assert.Contains(t, out, "db-check-env")
		assert.Empty(t, chamadas)
	})
	t.Run("sem goal e sem nada de fora: help", func(t *testing.T) {
		out, chamadas, err := semGoal(t, nil)
		require.NoError(t, err, out)
		assert.Contains(t, out, "db-check-env")
		assert.Empty(t, chamadas)
	})
	t.Run(".DEFAULT_GOAL com goal explícito não escapa do gate", func(t *testing.T) {
		out, chamadas, err := semGoal(t, nil, ".DEFAULT_GOAL=help", "db-create")
		require.Error(t, err, out)
		assert.Contains(t, out, msgNome)
		assert.Empty(t, chamadas)
	})
	// As recusas não pegam o caminho normal: sub-makes reais e make dentro de make.
	for _, alvo := range []string{"db-seed", "db-reset", "db-rebuild"} {
		t.Run("sub-make real em "+alvo, func(t *testing.T) {
			dir, path, cliente, log := sec63Dir(t, sec63Cred+"DB_HOST=127.0.0.1\nDB_NAME=rp_x\n")
			out, err := makeAlvo(t, dir, alvo, []string{path}, "DB_CLIENTE_BIN="+cliente)
			require.NoError(t, err, out)
			assert.NotContains(t, out, "SEC-63")
			assert.Contains(t, lerSeExiste(t, log), "CREATE DATABASE")
		})
	}
	t.Run("sub-make real em test-integration", func(t *testing.T) {
		dir, path, cliente, log := sec63Dir(t, sec63Cred+"DB_HOST=127.0.0.1\nDB_NAME=rp_x\n")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "backend", "rotaperfumes-api"), 0o755))
		out, err := makeAlvo(t, dir, "test-integration", []string{path}, "-k", "DB_CLIENTE_BIN="+cliente)
		require.NoError(t, err, out)
		assert.NotContains(t, out, "SEC-63")
		assert.Contains(t, lerSeExiste(t, log), "go test ./...")
	})
	t.Run("MAKECMDGOALS não chega aos filhos, e make dentro de make passa", func(t *testing.T) {
		dir, path, _, _ := sec63Dir(t, sec63Cred+"DB_HOST=127.0.0.1\nDB_NAME=rp_x\n")
		envLog := filepath.Join(dir, "env.log")
		scriptGo := "#!/bin/sh\nenv >> '" + envLog + "'\nenv -u MAKEFLAGS -u MAKELEVEL make -s -C '" + dir + "' help >/dev/null || { echo FILHO-RECUSADO >> '" + envLog + "'; exit 1; }\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "bin-falso", "go"), []byte(scriptGo), 0o755))
		out, err := makeAlvo(t, dir, "build-shared", []string{path})
		require.NoError(t, err, out)
		ambiente := lerSeExiste(t, envLog)
		require.NotEmpty(t, ambiente)
		assert.NotRegexp(t, `(?m)^MAKECMDGOALS=`, ambiente)
		assert.NotRegexp(t, `(?m)^\.DEFAULT_GOAL=`, ambiente)
		assert.NotContains(t, ambiente, "FILHO-RECUSADO")
	})
	// O .PHONY não anuncia alvo sem regra: make test-all saía rc=0 sem rodar nada.
	t.Run("todo alvo do .PHONY tem regra", func(t *testing.T) {
		mk := lerMakefile(t)
		for a := range mk.phony {
			assert.Contains(t, mk.regras, a, ".PHONY sem regra")
		}
	})
	t.Run("variáveis do gate no ambiente não desligam", func(t *testing.T) {
		for _, env := range []string{"RP_VALIDAR_BANCO=", "RP_ALVOS_SEM_BANCO=db-up", "RP_TIRA_CHARS=", "INTEGRACAO_PELO_MAKE="} {
			out, chamadas := sec63Desligou(t, []string{env}, nil, base)
			assert.Contains(t, out, "SEC-63: DB_NAME invalido", env)
			assert.Empty(t, chamadas, env)
		}
	})
}
