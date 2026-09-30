package config_test

// SEC-11 (Lote 12): testes de texto do Makefile e dos sql/*.sql (raiz via
// cmdutil.FindProjectRoot).
//   - MYSQL_OPTS sem -p (senha só por MYSQL_PWD no ambiente); sem default
//     DB_USUARIO?=/DB_SENHA?= nem "golang"; "-include .env" antes do export.
//   - Todo alvo que chama o mysql depende (direta ou transitivamente) de
//     db-check-env; db-create/db-down/db-fix-*/db-revert-* diretamente.
//   - help/build/test/lint/dev-frontend não exigem credenciais; help continua
//     sendo o alvo padrão (db-check-env vem depois dele).
//   - Nenhum sql/*.sql ensina -p$DB_SENHA.
// SEC-12 (DDL): coluna reuso_detectado_em no 06 e migração 23 idempotente.
// DB-01: o 17 não tem mais a coluna origem e o db-up aplica o 17.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
)

type makeRegra struct {
	prereqs []string
	receita []string
	linha   int // ordem de declaração
}

type makefile struct {
	texto  string
	linhas []string // continuações "\" já unidas
	regras map[string]*makeRegra
	phony  map[string]bool
	ordem  []string // alvos na ordem de declaração
}

func raizProjeto(t *testing.T) string {
	t.Helper()
	root, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)
	return root
}

func lerTexto(t *testing.T, caminho string) string {
	t.Helper()
	b, err := os.ReadFile(caminho)
	require.NoError(t, err)
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

var reRegra = regexp.MustCompile(`^([A-Za-z0-9_.%-]+(?:[ \t]+[A-Za-z0-9_.%-]+)*)[ \t]*:([^=].*|)$`)

func lerMakefile(t *testing.T) *makefile {
	t.Helper()
	texto := lerTexto(t, filepath.Join(raizProjeto(t), "Makefile"))
	var linhas []string
	acc := ""
	for _, l := range strings.Split(texto, "\n") {
		if strings.HasSuffix(l, "\\") {
			acc += strings.TrimSuffix(l, "\\") + " "
			continue
		}
		linhas = append(linhas, acc+l)
		acc = ""
	}
	mk := &makefile{texto: texto, linhas: linhas, regras: map[string]*makeRegra{}, phony: map[string]bool{}}
	var atual []*makeRegra
	for i, l := range linhas {
		if strings.HasPrefix(l, "\t") {
			for _, r := range atual {
				r.receita = append(r.receita, strings.TrimSpace(l))
			}
			continue
		}
		if strings.TrimSpace(l) == "" || strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		m := reRegra.FindStringSubmatch(l)
		if m == nil {
			atual = nil
			continue
		}
		deps := m[2]
		if idx := strings.Index(deps, "##"); idx >= 0 {
			deps = deps[:idx]
		}
		alvos := strings.Fields(m[1])
		if len(alvos) == 1 && alvos[0] == ".PHONY" {
			for _, a := range strings.Fields(deps) {
				mk.phony[a] = true
			}
			atual = nil
			continue
		}
		atual = nil
		for _, a := range alvos {
			r := &makeRegra{prereqs: strings.Fields(deps), linha: i}
			mk.regras[a] = r
			mk.ordem = append(mk.ordem, a)
			atual = append(atual, r)
		}
	}
	return mk
}

// dependeDe informa se alvo depende de dep, direta ou transitivamente.
func (mk *makefile) dependeDe(alvo, dep string) bool {
	visto := map[string]bool{}
	var busca func(a string) bool
	busca = func(a string) bool {
		if visto[a] {
			return false
		}
		visto[a] = true
		r, ok := mk.regras[a]
		if !ok {
			return false
		}
		for _, p := range r.prereqs {
			if p == dep || busca(p) {
				return true
			}
		}
		return false
	}
	return busca(alvo)
}

func (mk *makefile) linhaDaVariavel(t *testing.T, nome string) string {
	t.Helper()
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(nome) + `\s*[:?]?=`)
	for _, l := range mk.linhas {
		if re.MatchString(l) {
			return l
		}
	}
	t.Fatalf("variável %s não encontrada no Makefile", nome)
	return ""
}

func (mk *makefile) indiceLinha(re *regexp.Regexp) int {
	for i, l := range mk.linhas {
		if re.MatchString(l) {
			return i
		}
	}
	return -1
}

func TestSEC11_Makefile_MysqlOptsSemSenha(t *testing.T) {
	mk := lerMakefile(t)
	opts := mk.linhaDaVariavel(t, "MYSQL_OPTS")
	assert.NotRegexp(t, `(^|\s)-p`, strings.SplitN(opts, "=", 2)[1], "MYSQL_OPTS não pode levar -p (senha no argv)")
	assert.NotContains(t, opts, "DB_SENHA")
	assert.Contains(t, opts, "-u $(DB_USUARIO)")
	assert.Contains(t, opts, "--local-infile=1")

	casos := []struct{ nome, re string }{
		{"-p$(DB_SENHA)", `-p\$\(DB_SENHA\)`},
		{"-p${DB_SENHA}", `-p\$\{DB_SENHA\}`},
		{"-p$DB_SENHA", `-p\$DB_SENHA`},
		{"--password=", `--password=`},
	}
	for _, c := range casos {
		t.Run("sem "+c.nome, func(t *testing.T) {
			assert.NotRegexp(t, c.re, mk.texto)
		})
	}
}

func TestSEC11_Makefile_SemDefaultDeCredenciais(t *testing.T) {
	mk := lerMakefile(t)
	casos := []struct{ nome, re string }{
		{"DB_USUARIO?=", `(?m)^\s*DB_USUARIO\s*\?=`},
		{"DB_SENHA?=", `(?m)^\s*DB_SENHA\s*\?=`},
		{"DB_USUARIO=valor fixo", `(?m)^\s*DB_USUARIO\s*=`},
		{"DB_SENHA=valor fixo", `(?m)^\s*DB_SENHA\s*=`},
		{"golang", `(?i)golang`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			assert.NotRegexp(t, c.re, mk.texto)
		})
	}
	// As únicas atribuições são as de limpeza de CR (:= a partir dela mesma).
	for _, v := range []string{"DB_USUARIO", "DB_SENHA"} {
		l := mk.linhaDaVariavel(t, v)
		assert.Regexp(t, `^`+v+`\s*:=\s*.*\$\(subst \$\(CR\),,\$\(`+v+`\)\)`, l)
	}
}

func TestSEC11_Makefile_IncludeEnvEMysqlPwd(t *testing.T) {
	mk := lerMakefile(t)
	iInclude := mk.indiceLinha(regexp.MustCompile(`^-include \.env\s*$`))
	iSubst := mk.indiceLinha(regexp.MustCompile(`^DB_SENHA\s*:=`))
	iPwd := mk.indiceLinha(regexp.MustCompile(`^MYSQL_PWD\s*=\s*\$\(DB_SENHA\)\s*$`))
	iMsys := mk.indiceLinha(regexp.MustCompile(`^MSYS2_ENV_CONV_EXCL\s*=\s*MYSQL_PWD\s*$`))
	iExport := mk.indiceLinha(regexp.MustCompile(`^export\s*$`))
	iOpts := mk.indiceLinha(regexp.MustCompile(`^MYSQL_OPTS\s*=`))

	for nome, i := range map[string]int{"-include .env": iInclude, "DB_SENHA :=": iSubst, "MYSQL_PWD = $(DB_SENHA)": iPwd,
		"MSYS2_ENV_CONV_EXCL = MYSQL_PWD": iMsys, "export": iExport, "MYSQL_OPTS": iOpts} {
		require.GreaterOrEqual(t, i, 0, "linha %q ausente", nome)
	}
	assert.Less(t, iInclude, iSubst, "o .env é incluído antes da limpeza de CR")
	assert.Less(t, iSubst, iPwd)
	assert.Less(t, iPwd, iExport, "MYSQL_PWD precisa ser exportado")
	assert.Less(t, iMsys, iExport)
	assert.Contains(t, mk.linhaDaVariavel(t, "CR"), `$(shell printf '\r')`)
	assert.NotContains(t, mk.texto, "$(error", "nada de $(error) no parse: help/build/test não exigem credenciais")
}

func TestSEC11_Makefile_AlvosDeBancoDependemDeDbCheckEnv(t *testing.T) {
	mk := lerMakefile(t)
	require.Contains(t, mk.regras, "db-check-env")
	assert.True(t, mk.phony["db-check-env"], "db-check-env no .PHONY")

	// A checagem lê as variáveis exportadas pelo shell ($$VAR), sem expandir
	// a senha no texto do comando (argv do sh).
	receita := strings.Join(mk.regras["db-check-env"].receita, "\n")
	assert.Contains(t, receita, `test -n "$$DB_USUARIO" -a -n "$$MYSQL_PWD"`)
	assert.NotContains(t, receita, "$(DB_SENHA)")
	assert.Contains(t, receita, "exit 1")
	assert.Contains(t, receita, "SEC-11: defina DB_USUARIO/DB_SENHA no .env")

	// Diretos: db-create, db-down e todos os db-fix-*/db-revert-*.
	var diretos []string
	for a := range mk.regras {
		if a == "db-create" || a == "db-down" || strings.HasPrefix(a, "db-fix-") || strings.HasPrefix(a, "db-revert-") {
			diretos = append(diretos, a)
		}
	}
	sort.Strings(diretos)
	require.GreaterOrEqual(t, len(diretos), 16, "esperados db-create, db-down e os db-fix/db-revert: %v", diretos)
	for _, a := range []string{"db-fix-reuso-detectado", "db-revert-reuso-detectado", "db-fix-estoque-origem", "db-revert-estoque-origem"} {
		assert.Contains(t, diretos, a)
	}
	for _, a := range diretos {
		t.Run("direto/"+a, func(t *testing.T) {
			assert.Contains(t, mk.regras[a].prereqs, "db-check-env")
			assert.True(t, mk.phony[a], "%s no .PHONY", a)
		})
	}

	// Transitivos: db-up/db-seed/db-reset/db-rebuild herdam via db-create.
	for _, a := range []string{"db-up", "db-seed", "db-reset", "db-rebuild"} {
		t.Run("transitivo/"+a, func(t *testing.T) {
			assert.True(t, mk.dependeDe(a, "db-check-env"))
		})
	}

	// Regra geral: toda receita que chama o cliente mysql exige db-check-env.
	reMysql := regexp.MustCompile(`(^|[\s;&|(])mysql(\.exe)?\s`)
	for _, a := range mk.ordem {
		for _, l := range mk.regras[a].receita {
			if reMysql.MatchString(l) {
				assert.True(t, mk.dependeDe(a, "db-check-env"), "alvo %s chama o mysql sem db-check-env: %s", a, l)
				break
			}
		}
	}
}

func TestSEC11_Makefile_AlvosSemCredenciais(t *testing.T) {
	mk := lerMakefile(t)
	for _, a := range []string{"help", "build", "build-api", "test", "test-shared", "test-api", "lint", "dev-frontend", "cover-api", "cover-shared"} {
		t.Run(a, func(t *testing.T) {
			require.Contains(t, mk.regras, a)
			assert.False(t, mk.dependeDe(a, "db-check-env"), "%s não pode exigir credenciais do banco", a)
		})
	}
}

func TestSEC11_Makefile_HelpContinuaAlvoPadrao(t *testing.T) {
	mk := lerMakefile(t)
	require.NotEmpty(t, mk.ordem)
	assert.Equal(t, "help", mk.ordem[0], "o primeiro alvo (padrão do make) deve ser help")
	assert.Greater(t, mk.regras["db-check-env"].linha, mk.regras["help"].linha, "db-check-env fica depois do help")
}

// Nenhuma receita expande a senha no texto do comando ($(DB_SENHA) ou
// $(MYSQL_PWD)); só o db-check-env consulta $$MYSQL_PWD (variável do
// ambiente do shell, fora do argv).
func TestSEC11_Makefile_SenhaNuncaNoTextoDasReceitas(t *testing.T) {
	mk := lerMakefile(t)
	for _, a := range mk.ordem {
		for _, l := range mk.regras[a].receita {
			assert.NotContains(t, l, "$(DB_SENHA)", "alvo %s", a)
			assert.NotContains(t, l, "${DB_SENHA}", "alvo %s", a)
			assert.NotContains(t, l, "$(MYSQL_PWD)", "alvo %s", a)
			if a != "db-check-env" {
				assert.NotContains(t, l, "DB_SENHA", "alvo %s", a)
				assert.NotContains(t, l, "MYSQL_PWD", "alvo %s", a)
			}
		}
	}
}

// Todo sql/*.sql referenciado pelo Makefile existe.
func TestSEC11_Makefile_SqlReferenciadosExistem(t *testing.T) {
	mk := lerMakefile(t)
	root := raizProjeto(t)
	refs := regexp.MustCompile(`sql/[0-9A-Za-z_]+\.sql`).FindAllString(mk.texto, -1)
	require.NotEmpty(t, refs)
	for _, r := range refs {
		assert.FileExists(t, filepath.Join(root, filepath.FromSlash(r)))
	}
}

func TestSEC11_SQL_NenhumEnsinaSenhaNoArgv(t *testing.T) {
	root := raizProjeto(t)
	arquivos, err := filepath.Glob(filepath.Join(root, "sql", "*.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, arquivos)
	proibidos := []*regexp.Regexp{
		regexp.MustCompile(`-p\$DB_SENHA`),
		regexp.MustCompile(`-p"\$DB_SENHA"`),
		regexp.MustCompile(`-p\$\{DB_SENHA\}`),
		regexp.MustCompile(`-p\$\(DB_SENHA\)`),
		regexp.MustCompile(`--password=\$`),
	}
	for _, arq := range arquivos {
		t.Run(filepath.Base(arq), func(t *testing.T) {
			texto := lerTexto(t, arq)
			for _, re := range proibidos {
				assert.NotRegexp(t, re, texto)
			}
		})
	}
}

// Os cabeçalhos que ensinam a rodar o arquivo pelo mysql usam MYSQL_PWD.
func TestSEC11_SQL_CabecalhosUsamMysqlPwd(t *testing.T) {
	root := raizProjeto(t)
	for _, nome := range []string{
		"08_alter_usuarios_deve_trocar_senha.sql",
		"13_alter_senha_historico_tipo_reset.sql",
		"19_alter_clientes_cnpj_unique.sql",
		"19_revert_clientes_cnpj_unique.sql",
		"20_alter_clientes_cnpj_comment.sql",
		"20_revert_clientes_cnpj_comment.sql",
		"21_alter_refresh_tokens_revoked_reason.sql",
		"21_revert_refresh_tokens_revoked_reason.sql",
		"22_alter_usuarios_tokens_validos_desde.sql",
		"22_revert_usuarios_tokens_validos_desde.sql",
		"23_alter_refresh_tokens_reuso_detectado_em.sql",
		"23_revert_refresh_tokens_reuso_detectado_em.sql",
	} {
		t.Run(nome, func(t *testing.T) {
			texto := lerTexto(t, filepath.Join(root, "sql", nome))
			assert.Contains(t, texto, `MYSQL_PWD="$DB_SENHA" mysql`)
		})
	}
}

// SEC-12: DDL da coluna reuso_detectado_em.
func TestSEC12_SQL_ReusoDetectadoEm(t *testing.T) {
	root := raizProjeto(t)
	mk := lerMakefile(t)

	ddl := lerTexto(t, filepath.Join(root, "sql", "06_ddl_refresh_tokens.sql"))
	assert.Regexp(t, "`?reuso_detectado_em`?\\s+DATETIME\\s+NULL\\s+DEFAULT\\s+NULL", ddl)

	alter := lerTexto(t, filepath.Join(root, "sql", "23_alter_refresh_tokens_reuso_detectado_em.sql"))
	for _, s := range []string{"information_schema.COLUMNS", "PREPARE", "ADD COLUMN", "reuso_detectado_em", "DATETIME NULL DEFAULT NULL", "AFTER `revoked_reason`"} {
		assert.Contains(t, alter, s, "migração 23")
	}
	revert := lerTexto(t, filepath.Join(root, "sql", "23_revert_refresh_tokens_reuso_detectado_em.sql"))
	for _, s := range []string{"information_schema.COLUMNS", "PREPARE", "DROP COLUMN", "reuso_detectado_em"} {
		assert.Contains(t, revert, s, "revert 23")
	}

	casos := []struct{ alvo, arquivo string }{
		{"db-fix-reuso-detectado", "sql/23_alter_refresh_tokens_reuso_detectado_em.sql"},
		{"db-revert-reuso-detectado", "sql/23_revert_refresh_tokens_reuso_detectado_em.sql"},
	}
	for _, c := range casos {
		t.Run(c.alvo, func(t *testing.T) {
			require.Contains(t, mk.regras, c.alvo)
			assert.Contains(t, strings.Join(mk.regras[c.alvo].receita, "\n"), c.arquivo)
		})
	}
}

// DB-01: o 17 não cria mais estoque.origem e o db-up aplica o 17 depois do
// DDL de produtos (FK de estoque.sku).
func TestDB01_Estoque17SemOrigemEAplicadoNoDbUp(t *testing.T) {
	root := raizProjeto(t)
	ddl := lerTexto(t, filepath.Join(root, "sql", "17_ddl_estoque.sql"))

	ini := strings.Index(ddl, "CREATE TABLE")
	require.GreaterOrEqual(t, ini, 0)
	fim := strings.Index(ddl[ini:], ";")
	require.Greater(t, fim, 0)
	create := ddl[ini : ini+fim]
	assert.NotContains(t, create, "`origem`", "a coluna origem saiu do schema (migração 18)")
	assert.NotRegexp(t, `(?m)^\s*origem\s`, create)
	assert.NotContains(t, create, "import_csv")

	mk := lerMakefile(t)
	receita := strings.Join(mk.regras["db-up"].receita, "\n")
	i17 := strings.Index(receita, "sql/17_ddl_estoque.sql")
	iProd := strings.Index(receita, "sql/10_ddl_produtos.sql")
	require.GreaterOrEqual(t, i17, 0, "db-up precisa aplicar o 17")
	if iProd >= 0 {
		assert.Greater(t, i17, iProd, "estoque depois de produtos (FK)")
	}

	casos := []struct{ alvo, arquivo string }{
		{"db-fix-estoque-origem", "sql/18_alter_estoque_drop_origem.sql"},
		{"db-revert-estoque-origem", "sql/18_revert_estoque_drop_origem.sql"},
	}
	for _, c := range casos {
		t.Run(c.alvo, func(t *testing.T) {
			require.Contains(t, mk.regras, c.alvo)
			assert.Contains(t, strings.Join(mk.regras[c.alvo].receita, "\n"), c.arquivo)
		})
	}
}
