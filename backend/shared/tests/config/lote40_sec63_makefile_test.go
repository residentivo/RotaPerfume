package config_test

// SEC-63 (Lote 40, BackBrain; o TestBrain completa): DB_HOST, DB_NAME,
// DB_USUARIO, DB_TLS_CA, DB_CLIENTE_BIN, CONFIRMAR e CONFIRMO_PRODUCAO têm charset
// conferido no parse, e as guardas leem do ambiente. Antes, um valor com espaço
// acrescentava -h ao argv do cliente depois de a guarda aprovar o loopback, e um
// valor com aspas executava no shell da guarda. Tudo em cópias do Makefile com
// cliente falso: nada conecta.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/testkit/integracao"
)

const sec63Cred = "DB_USUARIO=u\nDB_SENHA=s\n"

// sec63Dir: cópia do Makefile com cliente que grava o argv ([a][b]) e go falso.
func sec63Dir(t *testing.T, env string) (dir, path, cliente, log string) {
	t.Helper()
	dir, path, _, log = bug29Dir(t)
	cliente = filepath.Join(dir, "bin-falso", "mariadb-falso")
	script := "#!/bin/sh\n[ \"$1\" = --version ] && { echo 'mariadb 11.4-MariaDB'; exit 0; }\n" +
		"{ printf '[%s]' \"$@\"; echo; } >> '" + log + "'\n"
	require.NoError(t, os.WriteFile(cliente, []byte(script), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600))
	return dir, path, cliente, log
}

func sec63Recusa(t *testing.T, alvo, env, msg string, args ...string) {
	t.Helper()
	dir, path, cliente, log := sec63Dir(t, env)
	out, err := makeAlvo(t, dir, alvo, []string{path}, append([]string{"DB_CLIENTE_BIN=" + cliente}, args...)...)
	require.Error(t, err, out)
	assert.Contains(t, out, msg)
	assert.NotContains(t, out, "192.168.168.106", "a mensagem não repete o valor")
	assert.NoFileExists(t, filepath.Join(dir, "pwn"), "o valor não pode executar")
	assert.Empty(t, lerSeExiste(t, log), "nenhum cliente chamado")
}

// PoCs A a L, por .env e por linha de comando.
func TestSEC63_Makefile_RecusaNoParse(t *testing.T) {
	casos := []struct {
		nome, variavel, valor string
	}{
		{"A dois -h", "DB_HOST", "127.0.0.1 -h 192.168.168.106"},
		{"host com aspas", "DB_HOST", `127.0.0.1"; touch pwn; "`},
		{"host com crase", "DB_HOST", "127.0.0.1`touch pwn`"},
		{"host -h inicial", "DB_HOST", "-h192.168.168.106"},
		{"C nome com -h", "DB_NAME", "rotaperfumes -h 192.168.168.106 -P 3306"},
		{"J nome com aspas", "DB_NAME", `x"; touch pwn; "`},
		{"J nome com ponto e vírgula", "DB_NAME", "rotaperfumes;DROP"},
		{"nome com hífen", "DB_NAME", "rota-perfumes"},
		{"E usuário --tee", "DB_USUARIO", "u --tee=/tmp/x"},
		{"E usuário --init-command", "DB_USUARIO", "u --init-command=SET@a=1"},
		{"usuário - inicial", "DB_USUARIO", "--tee=/tmp/x"},
		{"I CA com aspas", "DB_TLS_CA", `x"; touch pwn; exit 0; "`},
		{"CA com espaço", "DB_TLS_CA", "meu dir/ca.pem"},
		{"CA - inicial", "DB_TLS_CA", "-ca.pem"},
		{"L cliente com parêntese", "DB_CLIENTE_BIN", "/tmp/a(b/mariadb"},
		{"cliente com aspas", "DB_CLIENTE_BIN", `x"; touch pwn; "`},
		{"cliente - inicial", "DB_CLIENTE_BIN", "-mariadb"},
		{"F confirmo com aspas", "CONFIRMO_PRODUCAO", `x"; exit 0; "`},
		{"G confirmar com aspas", "CONFIRMAR", `x"; exit 0; "`},
	}
	for _, alvo := range []string{"db-create", "db-guard-producao", "db-check-env"} {
		for _, c := range casos {
			msg := "SEC-63: " + c.variavel + " invalido"
			// DB_CLIENTE_BIN no .env tem teste próprio: o harness o passa na linha
			// de comando, que ganha do arquivo.
			if c.variavel != "DB_CLIENTE_BIN" {
				t.Run(alvo+"/env/"+c.nome, func(t *testing.T) {
					sec63Recusa(t, alvo, sec63Cred+c.variavel+"="+c.valor+"\n", msg)
				})
			}
			t.Run(alvo+"/cmd/"+c.nome, func(t *testing.T) {
				sec63Recusa(t, alvo, sec63Cred, msg, c.variavel+"="+c.valor)
			})
		}
	}
	t.Run("DB_CLIENTE_BIN no .env", func(t *testing.T) {
		dir, path, _, log := sec63Dir(t, sec63Cred+"DB_CLIENTE_BIN=/tmp/a(b/mariadb\n")
		out, err := makeAlvo(t, dir, "db-check-env", []string{path})
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-63: DB_CLIENTE_BIN invalido")
		assert.NotContains(t, out, "Syntax error")
		assert.Empty(t, lerSeExiste(t, log))
	})
	for nome, args := range map[string][]string{
		"sobra só de espaços":  {"DB_HOST=127.1  "},
		"espaços nas pontas":   {"DB_HOST=  127.1  "},
		"quebra de linha host": {"DB_HOST=127.0.0.1\n-h\n192.168.168.106"},
		"quebra de linha nome": {"DB_NAME=rp\n"},
		"DB_NAME vazio":        {"DB_NAME="},
	} {
		t.Run(nome, func(t *testing.T) {
			sec63Recusa(t, "db-create", sec63Cred, "SEC-63: ", args...)
		})
	}
}

// SEC-46: os alvos destrutivos não chegam ao cliente com dois -h.
func TestSEC63_Makefile_AlvosDestrutivos(t *testing.T) {
	hostDuplo := []string{"DB_HOST=127.0.0.1 -h 192.168.168.106", "DB_PORT=3306", "DB_TLS_REQUIRED=false"}
	for _, alvo := range []string{"db-reset", "db-rebuild", "db-down", "db-seed"} {
		t.Run(alvo, func(t *testing.T) {
			sec63Recusa(t, alvo, sec63Cred, "SEC-63: DB_HOST invalido",
				append([]string{"DB_NAME=rotaperfumes", "CONFIRMAR=rotaperfumes"}, hostDuplo...)...)
		})
	}
	mk := lerMakefile(t)
	total := 0
	for _, a := range mk.ordem {
		if strings.HasPrefix(a, "db-fix-") || strings.HasPrefix(a, "db-revert-") {
			total++
			t.Run(a, func(t *testing.T) {
				sec63Recusa(t, a, sec63Cred, "SEC-63: DB_NAME invalido", "DB_HOST=127.0.0.1", "DB_NAME=rotaperfumes -h 192.168.168.106 -P 3306")
			})
		}
	}
	assert.GreaterOrEqual(t, total, 20)
}

func TestSEC63_Makefile_ConfirmacoesEmQualquerOrigem(t *testing.T) {
	prod := "DB_HOST=192.168.168.106\nDB_PORT=3306\nDB_TLS_REQUIRED=false\nDB_NAME=rotaperfumes\n"
	casa := "DB_HOST=192.168.168.106\nDB_PORT=3307\nDB_TLS_REQUIRED=false\nDB_NAME=rotaperfumes\n"
	for _, c := range []struct{ variavel, base, alvo string }{
		{"CONFIRMO_PRODUCAO", prod, "db-create"},
		{"CONFIRMAR", casa, "db-down"},
	} {
		invalido := c.variavel + `=x"; exit 0; "`
		msg := "SEC-63: " + c.variavel + " invalido"
		t.Run(c.variavel+" no .env", func(t *testing.T) {
			sec63Recusa(t, c.alvo, sec63Cred+c.base+invalido+"\n", msg)
		})
		t.Run(c.variavel+" na linha de comando", func(t *testing.T) {
			sec63Recusa(t, c.alvo, sec63Cred+c.base, msg, invalido)
		})
		t.Run(c.variavel+" no ambiente", func(t *testing.T) {
			dir, path, cliente, log := sec63Dir(t, sec63Cred+c.base)
			out, err := makeAlvo(t, dir, c.alvo, []string{path, invalido}, "DB_CLIENTE_BIN="+cliente)
			require.Error(t, err, out)
			assert.Contains(t, out, msg)
			assert.Empty(t, lerSeExiste(t, log))
		})
	}
	// Válidos só no arquivo não abrem a válvula, inclusive com CRLF.
	for nome, fim := range map[string]string{"LF": "\n", "CRLF": "\r\n"} {
		t.Run("válido no .env não confirma/"+nome, func(t *testing.T) {
			dir, path, cliente, log := sec63Dir(t, sec63Cred+prod+"CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes"+fim)
			out, err := makeAlvo(t, dir, "db-create", []string{path}, "DB_CLIENTE_BIN="+cliente)
			require.Error(t, err, out)
			assert.Contains(t, out, "e o banco de PRODUCAO")
			assert.NotContains(t, out, "SEC-63")
			assert.Empty(t, lerSeExiste(t, log))
		})
		t.Run("válido no .env não confirma DROP/"+nome, func(t *testing.T) {
			dir, path, cliente, log := sec63Dir(t, sec63Cred+casa+"CONFIRMAR=rotaperfumes"+fim)
			out, err := makeAlvo(t, dir, "db-down", []string{path}, "DB_CLIENTE_BIN="+cliente)
			require.Error(t, err, out)
			assert.Contains(t, out, "repita com CONFIRMAR=rotaperfumes")
			assert.NotContains(t, out, "SEC-63")
			assert.Empty(t, lerSeExiste(t, log))
		})
	}
	t.Run("CONFIRMO_PRODUCAO válido na linha de comando confirma", func(t *testing.T) {
		dir, path, cliente, log := sec63Dir(t, sec63Cred+prod)
		out, err := makeAlvo(t, dir, "db-create", []string{path}, "DB_CLIENTE_BIN="+cliente, "CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes")
		require.NoError(t, err, out)
		assert.Contains(t, out, "AVISO (DEPLOY-07)")
		assert.Contains(t, lerSeExiste(t, log), "CREATE DATABASE")
	})
}

func TestSEC63_Makefile_Aceitos(t *testing.T) {
	argvDe := func(t *testing.T, env string, args ...string) string {
		t.Helper()
		dir, path, cliente, log := sec63Dir(t, env)
		out, err := makeAlvo(t, dir, "db-create", []string{path}, append([]string{"DB_CLIENTE_BIN=" + cliente}, args...)...)
		require.NoError(t, err, out)
		return lerSeExiste(t, log)
	}
	t.Run("M: DB_HOST vazio não engole o -P", func(t *testing.T) {
		assert.Contains(t, argvDe(t, sec63Cred+"DB_HOST=\nDB_NAME=rp_x\nDB_TLS_REQUIRED=false\n"), "[-h][localhost][-P][3306]")
	})
	t.Run("host IPv6 entre colchetes vai inteiro (sem glob)", func(t *testing.T) {
		assert.Contains(t, argvDe(t, sec63Cred+"DB_HOST=[::1]\nDB_NAME=rp_x\n"), "[-h][[::1]][-P]")
	})
	t.Run("DB_NAME em maiúsculas", func(t *testing.T) {
		assert.Contains(t, argvDe(t, sec63Cred+"DB_HOST=127.0.0.1\nDB_NAME=ROTAPERFUMES\n"), "EXISTS ROTAPERFUMES ")
	})
	t.Run("todo o charset de host passa do parse", func(t *testing.T) {
		dir, path, cliente, _ := sec63Dir(t, sec63Cred)
		out, _ := makeAlvo(t, dir, "db-check-env", []string{path},
			"DB_CLIENTE_BIN="+cliente, "DB_HOST=Az09._-:[]")
		assert.NotContains(t, out, "SEC-63")
	})
	t.Run("CA relativa e absoluta", func(t *testing.T) {
		dir, path, cliente, log := sec63Dir(t, sec63Cred+"DB_HOST=10.0.0.1\nDB_NAME=rp_x\nDB_TLS_CA=ca.pem\n")
		ca := filepath.Join(dir, "ca.pem")
		require.NoError(t, os.WriteFile(ca, []byte("x"), 0o644))
		out, err := makeAlvo(t, dir, "db-create", []string{path}, "DB_CLIENTE_BIN="+cliente)
		require.NoError(t, err, out)
		assert.Contains(t, lerSeExiste(t, log), "[--ssl-ca=ca.pem]")
		out, err = makeAlvo(t, dir, "db-create", []string{path}, "DB_CLIENTE_BIN="+cliente, "DB_TLS_CA="+ca)
		require.NoError(t, err, out)
		assert.Contains(t, lerSeExiste(t, log), "[--ssl-ca="+ca+"]")
	})
	t.Run("C:/x/ca.pem passa do parse", func(t *testing.T) {
		dir, path, cliente, _ := sec63Dir(t, sec63Cred)
		out, err := makeAlvo(t, dir, "db-check-env", []string{path}, "DB_CLIENTE_BIN="+cliente, "DB_TLS_CA=C:/x/ca.pem")
		require.Error(t, err, out)
		assert.NotContains(t, out, "SEC-63")
		assert.Contains(t, out, "SEC-29: DB_TLS_CA=C:/x/ca.pem nao encontrado")
	})
	t.Run("DB_TLS_REQUIRED com aspas: recusado pelo case, sem executar", func(t *testing.T) {
		dir, path, cliente, log := sec63Dir(t, sec63Cred+`DB_TLS_REQUIRED=x"; touch pwn; "`+"\n")
		out, err := makeAlvo(t, dir, "db-check-env", []string{path}, "DB_CLIENTE_BIN="+cliente)
		require.Error(t, err, out)
		assert.Contains(t, out, "invalido: use true ou false")
		assert.NoFileExists(t, filepath.Join(dir, "pwn"))
		assert.Empty(t, lerSeExiste(t, log))
	})
}

func TestSEC63_Makefile_RpTiraCharsProibidaNoEnv(t *testing.T) {
	sec63Recusa(t, "db-create", sec63Cred+"DB_HOST=127.0.0.1 -h 192.168.168.106\nRP_TIRA_CHARS=\n", "BUG-29: .env declara RP_TIRA_CHARS")
}

func TestSEC63_Makefile_Gate(t *testing.T) {
	ruim := sec63Cred + "DB_HOST=a b\n"
	for _, c := range []struct {
		nome string
		args []string
	}{
		{"help", []string{"help"}},
		{"test", []string{"-n", "test"}},
		{"lint", []string{"-n", "lint"}},
		{"build-shared", []string{"-n", "build-shared"}},
		{"sem goal", nil},
	} {
		t.Run("passa/"+c.nome, func(t *testing.T) {
			dir, path, _, _ := sec63Dir(t, ruim)
			out, err := sec63MakeDireto(t, dir, path, c.args...)
			require.NoError(t, err, out)
			assert.NotContains(t, out, "SEC-63")
		})
	}
	for _, c := range []struct {
		nome string
		args []string
	}{
		{"db-guard-producao", []string{"db-guard-producao"}},
		{"dev-api", []string{"-n", "dev-api"}},
		{"db-export", []string{"-n", "db-export"}},
		{"test INTEGRATION=1", []string{"-n", "test", "INTEGRATION=1"}},
		{"test db-reset", []string{"-n", "test", "db-reset"}},
		{"alvo desconhecido", []string{"alvo-novo-sec63"}},
	} {
		t.Run("recusa/"+c.nome, func(t *testing.T) {
			dir, path, _, log := sec63Dir(t, ruim)
			out, err := sec63MakeDireto(t, dir, path, c.args...)
			require.Error(t, err, out)
			assert.Contains(t, out, "SEC-63: DB_HOST invalido")
			assert.Empty(t, lerSeExiste(t, log))
		})
	}
}

// sec63MakeDireto: sem args, roda o make sem goal (makeAlvo sempre passa um alvo).
func sec63MakeDireto(t *testing.T, dir, path string, args ...string) (string, error) {
	t.Helper()
	if len(args) > 0 {
		return makeAlvo(t, dir, args[0], []string{path}, args[1:]...)
	}
	cmd := exec.Command("make", "-s")
	cmd.Dir = dir
	cmd.Env = []string{path, "HOME=" + dir, "LC_ALL=C"}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Catraca: alvo com banco não entra na lista dos sem banco.
func TestSEC63_Makefile_CatracaDoGate(t *testing.T) {
	mk := lerMakefile(t)
	linha := ""
	for _, l := range mk.linhas {
		if strings.HasPrefix(l, "override RP_ALVOS_SEM_BANCO :=") {
			linha = strings.TrimPrefix(l, "override RP_ALVOS_SEM_BANCO :=")
		}
	}
	semBanco := strings.Fields(linha)
	require.NotEmpty(t, semBanco)
	// Fecho dos pré-requisitos: um alvo da lista não alcança o banco nem por dependência.
	proibidasNaReceita := []string{"$(DB_CLI)", "$(MYSQL_OPTS)", "go run", "$(DB_NAME)", "$(MAKE)"}
	var conferir func(raiz, a string, visto map[string]bool)
	conferir = func(raiz, a string, visto map[string]bool) {
		if visto[a] {
			return
		}
		visto[a] = true
		r := mk.regras[a]
		require.NotNil(t, r, "%s (via %s) não existe no Makefile", a, raiz)
		for _, l := range r.receita {
			for _, p := range proibidasNaReceita {
				assert.NotContains(t, l, p, "%s (via %s) usa %s", a, raiz, p)
			}
		}
		for _, p := range r.prereqs {
			assert.False(t, strings.HasPrefix(p, "db-"), "%s (via %s) depende de %s", a, raiz, p)
			if !strings.HasPrefix(p, "$(") {
				conferir(raiz, p, visto)
			}
		}
	}
	for _, a := range semBanco {
		conferir(a, a, map[string]bool{})
	}
	for _, a := range []string{"db-export", "dev-api", "run-api", "test-integration", "reset-senhas-simular"} {
		assert.NotContains(t, semBanco, a, "%s usa o banco", a)
	}
}

// As variáveis do próprio gate são redefinidas depois do include: nenhuma delas, em
// nenhuma forma de atribuição, desliga a validação a partir do .env.
func TestSEC63_Makefile_GateNaoDesligaPeloEnv(t *testing.T) {
	larga := "A B C D E F G H I J K L M N O P Q R S T U V W X Y Z a b c d e f g h i j k l m n o p q r s t u v w x y z 0 1 2 3 4 5 6 7 8 9 . _ - : [ ] ( )"
	formas := map[string]func(nome, valor string) string{
		"=":      func(n, v string) string { return n + "=" + v + "\n" },
		":=":     func(n, v string) string { return n + " := " + v + "\n" },
		"+=":     func(n, v string) string { return n + " += " + v + "\n" },
		"export": func(n, v string) string { return "export " + n + "=" + v + "\n" },
	}
	vars := []struct{ nome, valor string }{
		{"RP_VALIDAR_BANCO", ""},
		{"RP_ALVOS_SEM_BANCO", "db-create"},
		{"RP_CHARS_HOST", larga},
		{"INTEGRACAO_PELO_MAKE", ""},
		{"CR", ""},
	}
	// Com espaço (sobra que nenhuma lista tira) e com "(" (que a lista larga tiraria).
	hosts := map[string]string{"espaço": "127.0.0.1 -h 192.168.168.106", "parêntese": "127.0.0.1(b"}
	goals := map[string][]string{"db-create": {"db-create"}, "test INTEGRATION=1": {"-n", "test", "INTEGRATION=1"}}
	rodar := func(t *testing.T, env string, goal []string) {
		t.Helper()
		dir, path, cliente, log := sec63Dir(t, env)
		out, err := makeAlvo(t, dir, goal[0], []string{path}, append(goal[1:], "DB_CLIENTE_BIN="+cliente)...)
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-63: DB_HOST invalido")
		assert.Empty(t, lerSeExiste(t, log))
	}
	for nomeHost, host := range hosts {
		for nomeGoal, goal := range goals {
			for forma, montar := range formas {
				todas := ""
				for _, v := range vars {
					linha := montar(v.nome, v.valor)
					todas += linha
					t.Run(nomeHost+"/"+nomeGoal+"/"+forma+"/"+v.nome, func(t *testing.T) {
						rodar(t, sec63Cred+"DB_HOST="+host+"\n"+linha, goal)
					})
				}
				t.Run(nomeHost+"/"+nomeGoal+"/"+forma+"/todas juntas", func(t *testing.T) {
					rodar(t, sec63Cred+todas+"DB_HOST="+host+"\n", goal)
				})
			}
		}
	}
}

// Catraca: guardas e macros leem do ambiente; ninguém tira as variáveis dele.
func TestSEC63_Makefile_CatracaSemInterpolacao(t *testing.T) {
	mk := lerMakefile(t)
	proibidas := []string{"$(DB_HOST)", "$(DB_NAME)", "$(DB_TLS_CA)", "$(DB_TLS_REQUIRED)", "$(DB_CLIENTE)", "$(DB_CLIENTE_BIN)"}
	verificar := func(onde, l string) {
		for _, p := range proibidas {
			assert.NotContains(t, l, p, "%s interpola %s", onde, p)
		}
	}
	guardas := 0
	for _, a := range mk.ordem {
		if !strings.HasPrefix(a, "db-guard-") && a != "db-check-env" {
			continue
		}
		guardas++
		for _, l := range mk.regras[a].receita {
			verificar(a, l)
		}
	}
	assert.Equal(t, 5, guardas)
	for _, macro := range []string{"NORMALIZA_HOST", "MSG_SEC56", "PRODUCAO_DETECTA"} {
		achou := false
		for _, l := range mk.linhas {
			if strings.HasPrefix(l, macro+" =") {
				achou = true
				verificar(macro, l)
			}
		}
		assert.True(t, achou, macro)
	}
	re := regexp.MustCompile(`(?m)^\s*unexport\b.*\b(DB_HOST|DB_NAME|DB_TLS_CA|DB_TLS_REQUIRED|DB_CLIENTE|DB_CLIENTE_BIN)\b`)
	assert.False(t, re.MatchString(mk.texto), "unexport deixaria a guarda sem o valor")
}

func sec63AmbienteGo(t *testing.T, vars map[string]string) {
	t.Helper()
	ambienteIntegracao(t, credenciais(vars))
}

func TestSEC63_Go_ConfigDoAmbienteRecusa(t *testing.T) {
	casos := []struct{ variavel, valor string }{
		{"DB_NAME", "x@tcp(192.168.168.106:3306)/rotaperfumes"},
		{"DB_NAME", "rotaperfumes?tls=false&x="},
		{"DB_NAME", "rota perfumes"},
		{"DB_NAME", "a/b"},
		{"DB_NAME", "a-b"},
		{"DB_HOST", "127.0.0.1)/x@tcp(192.168.168.106"},
		{"DB_HOST", "a@b"},
		{"DB_HOST", "a/b"},
		{"DB_HOST", "a?b"},
		{"DB_HOST", "a(b"},
		{"DB_HOST", "a b"},
		{"DB_HOST", "-h"},
		{"DB_HOST", "127.0.0.1\n"},
		{"DB_USUARIO", "u:x@tcp(192.168.168.106:3306)/"},
		{"DB_USUARIO", "a@b"},
		{"DB_USUARIO", "a/b"},
		{"DB_USUARIO", "a?b"},
		{"DB_USUARIO", "a(b"},
		{"DB_USUARIO", "a b"},
		{"DB_USUARIO", "-u"},
	}
	for i, c := range casos {
		t.Run(c.variavel+"/"+string(rune('a'+i)), func(t *testing.T) {
			vars := map[string]string{"DB_HOST": "127.0.0.1", "DB_PORT": "3307", "DB_TLS_CA": "/inexistente-sec63.pem"}
			vars[c.variavel] = c.valor
			sec63AmbienteGo(t, vars)
			_, err := integracao.ConfigDoAmbiente()
			require.ErrorIs(t, err, integracao.ErrDBValorInvalido, "antes da CA")
			assert.NotErrorIs(t, err, config.ErrDBTLSCA)
			assert.Contains(t, err.Error(), c.variavel)
			assert.NotContains(t, err.Error(), c.valor, "a mensagem não repete o valor")
		})
	}
}

func TestSEC63_Go_DSNNaoTrocaEndereco(t *testing.T) {
	vars := map[string]string{"DB_HOST": "127.0.0.1", "DB_PORT": "3307", "DB_NAME": "ROTA_perfumes", "DB_USUARIO": "u.a-b_c"}
	sec63AmbienteGo(t, vars)
	cfg, err := integracao.ConfigDoAmbiente()
	require.NoError(t, err)
	parsed, err := mysql.ParseDSN(cfg.DSN())
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:3307", parsed.Addr)
	assert.Equal(t, "ROTA_perfumes", parsed.DBName)
	assert.Equal(t, "u.a-b_c", parsed.User)

	// Por que o charset existe: o mesmo DSN com DB_NAME malicioso troca o Addr.
	malicioso := config.Config{DBHost: "127.0.0.1", DBPort: "3307", DBUsuario: "u", DBSenha: "s", DBName: "x@tcp(192.0.2.1:3306)/rotaperfumes"}
	parsed, err = mysql.ParseDSN(malicioso.DSN())
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.1:3306", parsed.Addr, "sem o ValidarFormaDB o endereço real mudaria")
}

// Paridade: o que o Makefile aceita no parse, o Go aceita, e vice-versa.
func TestSEC63_Paridade_MakefileGo(t *testing.T) {
	validos := func(variavel, valor string) (bool, bool) {
		dir, path, cliente, _ := sec63Dir(t, sec63Cred)
		out, _ := makeAlvo(t, dir, "db-guard-sem-producao", []string{path}, "DB_CLIENTE_BIN="+cliente, variavel+"="+valor)
		host, nome, usuario := "h", "n", "u"
		switch variavel {
		case "DB_HOST":
			host = valor
		case "DB_NAME":
			nome = valor
		case "DB_USUARIO":
			usuario = valor
		}
		return !strings.Contains(out, "SEC-63"), integracao.ValidarFormaDB(host, nome, usuario) == nil
	}
	amostras := map[string][]string{
		"DB_HOST":    {"localhost", "127.0.0.1", "[::1]", "[::ffff:1.2.3.4]", "a_b-c.d:1", "Az09", "a-", "a b", "-a", "a@b", "a/b", "a?b", "a(b", "a%b", "a,b", "a+b", `a"b`, "é", "aéb"},
		"DB_NAME":    {"rotaperfumes", "ROTA_1", "a-b", "a.b", "a b", "a@b", "a/b", "a?b", "a;b", `a"b`, "é", "aéb"},
		"DB_USUARIO": {"root", "u.a-b_c", "a-", "-a", "a b", "a@b", "a/b", "a:b", "a?b", `a"b`, "é", "aéb"},
	}
	for variavel, valores := range amostras {
		for _, v := range valores {
			mk, goOk := validos(variavel, v)
			assert.Equal(t, mk, goOk, "%s=%q: Makefile aceita=%t, Go aceita=%t", variavel, v, mk, goOk)
		}
	}
}
