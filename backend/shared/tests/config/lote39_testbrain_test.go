package config_test

// Lote 39 (TestBrain): lacunas dos testes do BUG-31, BUG-32, SEC-59 e DB-09.
// Makefile em cópias com cliente mariadb e go falsos (bug29Dir): nada conecta.
// Os literais de relógio são montados por concatenação (catraca TST-23).

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lote39TBEscreverEnv grava o .env inteiro: a linha ruim fica no número esperado.
func lote39TBEscreverEnv(t *testing.T, dir, conteudo string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte(conteudo), 0o600))
}

func TestBUG31_Makefile_PortaEfetiva(t *testing.T) {
	casos := []struct {
		nome, env string
		args      []string
		esperado  string
	}{
		{"só espaços na linha de comando", "", []string{"DB_PORT=   "}, "-P 3306 --default-character-set=utf8mb4"},
		{"CR no .env", "DB_PORT=\r\n", nil, "-P 3306 --default-character-set=utf8mb4"},
		{"só espaços no .env", "DB_PORT=   \n", nil, "-P 3306 --default-character-set=utf8mb4"},
		{"sem a linha no .env", "#\n", nil, "-P 3306 --default-character-set=utf8mb4"},
		{"porta explícita é mantida", "DB_PORT=33306\n", nil, "-P 33306 --default-character-set=utf8mb4"},
		{"linha de comando vence o .env", "DB_PORT=33306\n", []string{"DB_PORT=33307"}, "-P 33307 --default-character-set=utf8mb4"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			lote39TBEscreverEnv(t, dir, "DB_HOST=127.0.0.1\nDB_NAME=rp_a\nDB_USUARIO=u-l39\nDB_SENHA=s-l39\n"+c.env)
			out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente, c.args...)...)
			require.NoError(t, err, out)
			assert.Contains(t, lerSeExiste(t, log), c.esperado)
		})
	}
}

// Os casos do TestBUG32_Makefile_PortaComCaracteresDeShell não têm aspa simples,
// a que delimita o valor na mensagem, nem barra invertida ou ";".
func TestBUG32_Makefile_PortaComOutrosCaracteres(t *testing.T) {
	casos := []struct{ nome, porta, msg string }{
		{"aspa simples", "33'06", "SEC-57: DB_PORT='33'06'"},
		{"aspa simples com comando", "3306'; touch pwn-l39; '", "SEC-57: DB_PORT='3306'; touch pwn-l39; ''"},
		{"barra invertida", `33\06`, `SEC-57: DB_PORT='33\06'`},
		{"ponto e vírgula", "3306; touch pwn-l39", "SEC-57: DB_PORT='3306; touch pwn-l39'"},
		{"tab", "33\t06", "SEC-57: DB_PORT='33?06'"},
		{"cr", "33\r06", "SEC-57: DB_PORT='33?06'"},
	}
	for _, alvo := range []string{"db-guard-producao", "db-guard-sem-producao", "db-guard-integracao", "db-check-env"} {
		for _, c := range casos {
			t.Run(alvo+"/"+c.nome, func(t *testing.T) {
				dir, path, cliente, log := bug29Dir(t)
				out, err := makeAlvo(t, dir, alvo, []string{path}, bug29Args(cliente, "DB_PORT="+c.porta)...)
				require.Error(t, err, out)
				assert.Contains(t, out, c.msg)
				assert.NoFileExists(t, filepath.Join(dir, "pwn-l39"), "o valor não pode executar no shell da guarda")
				assert.Empty(t, lerSeExiste(t, log))
			})
		}
	}
}

func TestSEC59_Makefile_OutrasLinhasRecusadas(t *testing.T) {
	linhas := []struct{ nome, linha string }{
		{"vpath", "vpath % /tmp"},
		{"load", "load x.so"},
		{"ifdef", "ifdef X"},
		{"else", "else"},
		{"endif", "endif"},
		{"endef", "endef"},
		{"phony", ".PHONY: x"},
		{"regra de padrão", "%: ; @echo PWN"},
		{"export sem valor", "export"},
		{"dois nomes", "export X Y=1"},
		{"nome com ponto", "A.B=1"},
		{"nome com hífen", "A-B=1"},
		{"nome começando por dígito", "1X=1"},
		{"dólar depois de # no valor", "X=1 # $(shell touch pwn-l39)"},
		{"dólar no fim", "X=a$"},
		{"barra no fim com CR", "X=a \\\r"},
		{"barra e espaços no fim", "X=a \\   "},
	}
	for _, c := range linhas {
		t.Run(c.nome, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			lote39AnexarEnv(t, dir, c.linha+"\n")
			out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente)...)
			require.Error(t, err, out)
			assert.Contains(t, out, "SEC-59: .env:6 nao e NOME=valor")
			assert.NoFileExists(t, filepath.Join(dir, "pwn-l39"))
			assert.Empty(t, lerSeExiste(t, log))
		})
	}
	t.Run("última linha sem quebra de linha também é conferida", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		lote39AnexarEnv(t, dir, "override X=1")
		out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente)...)
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-59: .env:6 nao e NOME=valor")
		assert.Empty(t, lerSeExiste(t, log))
	})
}

// A allowlist aceita NOME=valor de qualquer nome. O que importa é que um nome
// interno do Makefile, escrito sem "$" nem override, não desvie a guarda: o .env
// aponta para a produção e o cliente falso não pode ser chamado.
func TestSEC59_Makefile_VariaveisInternasNaoDesviamGuarda(t *testing.T) {
	const producao = "DB_HOST=192.168.168.106\nDB_PORT=3306\nDB_NAME=rp_a\nDB_USUARIO=u-l39\nDB_SENHA=s-l39\nDB_TLS_REQUIRED=false\n"
	linhas := []string{
		"PRODUCAO_HOSTS_LAN=",
		"PRODUCAO_HOSTS_LAN := 127.0.0.9",
		"PRODUCAO_HOSTS_REDE=",
		"PRODUCAO_DETECTA=prod=false",
		"NORMALIZA_HOST=h=x",
		"PORTA_DETECTA_INVALIDA=portainvalida=false",
		"MSG_SEC57=true",
		"CONFIRMO_PRODUCAO=192.168.168.106/rp_a",
		"export CONFIRMO_PRODUCAO=192.168.168.106/rp_a",
		"CONFIRMAR=rp_a",
		"GUARDA_INTEGRACAO=",
	}
	for _, linha := range linhas {
		t.Run(linha, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			lote39TBEscreverEnv(t, dir, producao+linha+"\n")
			out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente)...)
			require.Error(t, err, out)
			assert.Contains(t, out, "e o banco de PRODUCAO")
			assert.Empty(t, lerSeExiste(t, log), "nenhum cliente chamado")
		})
	}
}

// O SEM_SEGREDOS não pode ser esvaziado por linhas NOME=valor no .env.
func TestSEC59_Makefile_VariaveisInternasNaoEsvaziamSemSegredos(t *testing.T) {
	for _, linha := range []string{"SEM_SEGREDOS=", "SEGREDOS_FIXOS=", "VARS_DB_INTEGRACAO=", "INTEGRATION=1", "INTEGRACAO_PELO_MAKE=1"} {
		t.Run(linha, func(t *testing.T) {
			dir, path, _, log := bug29Dir(t)
			lote39AnexarEnv(t, dir, linha+"\n")
			scriptEnv := "#!/bin/sh\necho \"env $*\" >> '" + log + "'\n"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "bin-falso", "env"), []byte(scriptEnv), 0o755))
			out, err := makeAlvo(t, dir, "build-shared", []string{path})
			require.NoError(t, err, out)
			chamadas := lerSeExiste(t, log)
			for _, segredo := range []string{"-u DB_SENHA ", "-u MYSQL_PWD ", "-u DB_USUARIO "} {
				assert.Contains(t, chamadas, segredo)
			}
		})
	}
}

// A allowlist lê o arquivo com o erro em /dev/null: o que ela não consegue ler
// não pode virar "arquivo sem linha ruim" seguido de sucesso.
func TestSEC59_Makefile_ArquivoIlegivelNaoPassa(t *testing.T) {
	casos := []struct {
		nome string
		prep func(t *testing.T, dir string)
	}{
		{"arquivo inexistente", func(*testing.T, string) {}},
		{"diretório", func(t *testing.T, dir string) { require.NoError(t, os.Mkdir(filepath.Join(dir, "l39.env"), 0o700)) }},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			c.prep(t, dir)
			out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente, "ENV_ARQUIVO=l39.env")...)
			require.Error(t, err, out)
			assert.Empty(t, lerSeExiste(t, log), "nenhum cliente chamado")
		})
	}
	t.Run("sem permissão de leitura", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root lê arquivo com modo 000")
		}
		dir, path, cliente, log := bug29Dir(t)
		arq := filepath.Join(dir, "l39.env")
		require.NoError(t, os.WriteFile(arq, []byte("DB_HOST=127.0.0.1\n"), 0o000))
		out, err := makeAlvo(t, dir, "db-create", []string{path}, bug29Args(cliente, "ENV_ARQUIVO=l39.env")...)
		require.Error(t, err, out)
		assert.Empty(t, lerSeExiste(t, log), "nenhum cliente chamado")
	})
}

// O TestIntegracaoDB09_Migracao27_ForceNaoPreencheNull só roda com INTEGRATION=1.
// Esta conferência estática pega a mesma regressão no "make test" e no CI.
func TestDB09_SQL27_AlterDentroDoBlocoDepoisDoSignal(t *testing.T) {
	semComentarios := func(sql string) string {
		var ls []string
		for _, l := range strings.Split(sql, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(l), "--") {
				ls = append(ls, l)
			}
		}
		return strings.Join(ls, "\n")
	}
	ler := func(nome string) string {
		b, err := os.ReadFile(filepath.Join(raizProjeto(t), "sql", nome))
		require.NoError(t, err)
		return string(b)
	}
	relogio := "CURRENT_" + "TIMESTAMP"
	definicao := regexp.MustCompile(`created_at\s+TIMESTAMP\s+NOT\s+NULL\s+DEFAULT\s+` + relogio + `\b`)

	codigo := semComentarios(ler("27_alter_senha_historico_created_at_not_null.sql"))
	t.Run("ALTER único, dentro do bloco e depois do SIGNAL", func(t *testing.T) {
		iBloco := strings.Index(codigo, "BEGIN NOT ATOMIC")
		iSignal := strings.Index(codigo, "SIGNAL SQLSTATE '45000'")
		iAlter := strings.Index(codigo, "ALTER TABLE")
		iFim := strings.Index(codigo, "END$$")
		require.GreaterOrEqual(t, iBloco, 0)
		require.GreaterOrEqual(t, iSignal, 0)
		require.GreaterOrEqual(t, iAlter, 0)
		require.GreaterOrEqual(t, iFim, 0)
		assert.Less(t, iBloco, iSignal)
		assert.Less(t, iSignal, iAlter, "ALTER depois do SIGNAL")
		assert.Less(t, iAlter, iFim, "fora do bloco, um cliente com --force chegaria ao ALTER")
		assert.Equal(t, 1, strings.Count(codigo, "ALTER TABLE"))
	})
	t.Run("não altera dados", func(t *testing.T) {
		assert.NotRegexp(t, `(?i)\b(UPDATE|DELETE|TRUNCATE|DROP)\b`, codigo)
	})
	t.Run("mesma definição do DDL de bancos novos", func(t *testing.T) {
		assert.Regexp(t, definicao, codigo)
		assert.Regexp(t, definicao, semComentarios(ler("07_ddl_senha_historico.sql")))
	})
	t.Run("avulsa: fora do sql/ordem-ddl.txt", func(t *testing.T) {
		assert.NotContains(t, ler("ordem-ddl.txt"), "27_")
	})
	t.Run("revert: um único ALTER, sem mexer em dados", func(t *testing.T) {
		volta := semComentarios(ler("27_revert_senha_historico_created_at_not_null.sql"))
		assert.Equal(t, 1, strings.Count(volta, "ALTER TABLE"))
		assert.Regexp(t, `created_at\s+TIMESTAMP\s+NULL\s+DEFAULT\s+`+relogio+`\b`, volta)
		assert.NotRegexp(t, `(?i)\b(UPDATE|DELETE|TRUNCATE|DROP)\b`, volta)
	})
}
