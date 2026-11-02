package deploy_test

// DEPLOY-02 (Lote 29, DataBrain): hardening do banco em deploy/setup-servidor.sh.
//   - Drop-in zz-rotaperfumes-hardening.cnf sempre gravado (local_infile = 0,
//     sql_mode estrito, utf8mb4/utf8mb4_unicode_ci), sem restart: o que diverge
//     em execução vai por SET GLOBAL.
//   - Drop-in zz-rotaperfumes-bind.cnf só quando o bind é loopback; é o único
//     que reinicia o serviço.
//   - Migração do legado zz-rotaperfumes.cnf, validação antes de aplicar,
//     rollback, conferência final (só AVISO) e o modo --apenas-banco.
// O script é carregado com "source" (o main só roda quando executado
// diretamente) e mysql_consulta/mysql_root/systemctl/install/mysqld/mariadbd
// viram stubs que registram as chamadas em $FAKE/log.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sqlModeMariaDB = "STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION"
	sqlModeMySQL   = "ONLY_FULL_GROUP_BY," + sqlModeMariaDB
	versaoMySQL    = "8.0.46-0ubuntu0.24.04.1"
	versaoMariaDB  = "11.8.8-MariaDB-ubu2404"
	dropinHard     = "zz-rotaperfumes-hardening.cnf"
	dropinBind     = "zz-rotaperfumes-bind.cnf"
	dropinLegado   = "zz-rotaperfumes.cnf"
)

func script(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash não está no PATH")
	}
	root, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)
	return filepath.Join(root, "deploy", "setup-servidor.sh")
}

// stubs: carregados depois do "source". $FAKE é o diretório do cenário.
const stubs = `
TMP_DIR=$FAKE/tmp
DIR=$FAKE/etc
mkdir -p "$TMP_DIR" "$DIR"
reg() { printf '%s\n' "$*" >>"$FAKE/log"; }
diretorio_dropin() { printf '%s' "$DIR"; }
install() {
  local n=$#
  local src=${@:n-1:1} dst=${@:n:1}
  reg "install $(basename "$dst")"
  cp -- "$src" "$dst"
}
systemctl() {
  reg "systemctl $*"
  [[ "$1" == restart ]] || return 0
  local n
  n=$(( $(cat "$FAKE/restarts" 2>/dev/null || echo 0) + 1 ))
  echo "$n" >"$FAKE/restarts"
  if [[ "${FAKE_FALHA_RESTART:-}" == 1 && "$n" == 1 ]]; then return 1; fi
  if [[ -f "$DIR/zz-rotaperfumes-bind.cnf" ]]; then echo 0.0.0.0 >"$FAKE/bind"; else echo 127.0.0.1 >"$FAKE/bind"; fi
  return 0
}
mysqld() { reg "validar mysqld $*"; return "${FAKE_VALIDA_RC:-0}"; }
mariadbd() { reg "validar mariadbd $*"; return "${FAKE_VALIDA_RC:-0}"; }
mysql_root() { return 0; }
mysql_consulta() {
  reg "sql $1"
  case "$1" in
    "SHOW GLOBAL VARIABLES LIKE 'bind_address'") printf 'bind_address\t%s\n' "$(cat "$FAKE/bind")" ;;
    "SHOW GLOBAL VARIABLES WHERE"*) cat "$FAKE/vars" ;;
    "SET GLOBAL"*) [[ -f "$FAKE/vars_apos_set" ]] && cp "$FAKE/vars_apos_set" "$FAKE/vars"; return 0 ;;
    *persisted_variables*) cat "$FAKE/persistidas" 2>/dev/null || true ;;
    *SCHEMATA*) cat "$FAKE/schema" 2>/dev/null || true ;;
    *information_schema.TABLES*) cat "$FAKE/tabelas" 2>/dev/null || true ;;
    "SELECT @@skip_networking") echo 0 ;;
    "SELECT @@port") echo 3306 ;;
    "SELECT VERSION()") echo "$FAKE_VERSAO" ;;
  esac
}
`

type cenario struct {
	sabor, versao string
	bind          string            // bind_address inicial
	vars          string            // SHOW GLOBAL VARIABLES inicial
	varsAposSet   string            // vars depois do SET GLOBAL ("" = não muda)
	arquivos      map[string]string // drop-ins pré-existentes
	extras        map[string]string // persistidas, schema, tabelas
	env           []string
	comando       string // padrão: etapa_bind_address
}

type resultado struct {
	dir, saida string
	err        error
	log        []string
}

func (r resultado) conta(prefixo string) int {
	n := 0
	for _, l := range r.log {
		if strings.HasPrefix(l, prefixo) {
			n++
		}
	}
	return n
}

func (r resultado) indice(prefixo string) int {
	for i, l := range r.log {
		if strings.HasPrefix(l, prefixo) {
			return i
		}
	}
	return -1
}

func (r resultado) arquivo(t *testing.T, nome string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r.dir, "etc", nome))
	if os.IsNotExist(err) {
		return "", false
	}
	require.NoError(t, err)
	return string(b), true
}

func varsTSV(sabor string, localInfile, sqlMode, collation string, csc string) string {
	s := "character_set_server\tutf8mb4\ncollation_server\t" + collation + "\nlocal_infile\t" + localInfile + "\nsql_mode\t" + sqlMode + "\n"
	if csc != "" {
		s += "character_set_collations\t" + csc + "\n"
	}
	return s
}

func varsBons(sabor string) string {
	if sabor == "mysql" {
		// MySQL devolve o sql_mode na ordem dele; a comparação é por conjunto.
		return varsTSV(sabor, "OFF", "STRICT_TRANS_TABLES,ONLY_FULL_GROUP_BY,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION", "utf8mb4_unicode_ci", "")
	}
	return varsTSV(sabor, "OFF", sqlModeMariaDB, "utf8mb4_unicode_ci", "utf8mb4=utf8mb4_unicode_ci")
}

func varsRuins(sabor string) string {
	if sabor == "mysql" {
		return varsTSV(sabor, "ON", sqlModeMySQL, "utf8mb4_0900_ai_ci", "")
	}
	return varsTSV(sabor, "ON", sqlModeMariaDB, "utf8mb4_uca1400_ai_ci", "utf8mb4=utf8mb4_uca1400_ai_ci")
}

// ambienteLimpo é o os.Environ() sem o que mudaria o comportamento dos stubs
// ou do bash: FAKE_* herdados do shell (ex.: FAKE_VALIDA_RC=1 quebrava os
// cenários) e BASH_ENV/ENV (arquivo carregado pelo bash não interativo).
func ambienteLimpo() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "FAKE") || strings.HasPrefix(kv, "BASH_ENV=") || strings.HasPrefix(kv, "ENV=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func rodar(t *testing.T, c cenario, dir string) resultado {
	t.Helper()
	sc := script(t)
	if dir == "" {
		dir = t.TempDir()
	}
	escrever := func(nome, conteudo string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, nome)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, nome), []byte(conteudo), 0o644))
	}
	_ = os.Remove(filepath.Join(dir, "log"))
	_ = os.Remove(filepath.Join(dir, "restarts"))
	if c.bind != "" {
		escrever("bind", c.bind+"\n")
	}
	if c.vars != "" {
		escrever("vars", c.vars)
	}
	if c.varsAposSet != "" {
		escrever("vars_apos_set", c.varsAposSet)
	}
	for nome, conteudo := range c.arquivos {
		escrever(filepath.Join("etc", nome), conteudo)
	}
	for nome, conteudo := range c.extras {
		escrever(nome, conteudo)
	}
	cmdTxt := c.comando
	if cmdTxt == "" {
		cmdTxt = "etapa_bind_address"
	}
	corpo := `source "$1"` + "\n" + stubs + "\n" +
		`MYSQL_SABOR=$FAKE_SABOR; MYSQL_VERSAO=$FAKE_VERSAO; MYSQL_SERVICO=$FAKE_SABOR` + "\n" + cmdTxt + "\n"
	cmd := exec.Command("bash", "-c", corpo, "deploy02", sc)
	cmd.Env = append(ambienteLimpo(), "FAKE="+dir, "FAKE_SABOR="+c.sabor, "FAKE_VERSAO="+c.versao)
	cmd.Env = append(cmd.Env, c.env...)
	out, err := cmd.CombinedOutput()
	r := resultado{dir: dir, saida: string(out), err: err}
	if b, e := os.ReadFile(filepath.Join(dir, "log")); e == nil {
		r.log = strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	}
	return r
}

func bash(t *testing.T, corpo string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "-c", `source "$1"`+"\n"+corpo, "deploy02", script(t))
	cmd.Env = ambienteLimpo()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ---------------------------------------------------------------------------
// Conteúdo dos drop-ins por sabor
// ---------------------------------------------------------------------------

func TestDEPLOY02_ConteudoHardening_MySQL8(t *testing.T) {
	out, err := bash(t, `conteudo_dropin_hardening mysql `+versaoMySQL)
	require.NoError(t, err, out)
	assert.Contains(t, out, "[mysqld]\n")
	assert.Contains(t, out, "local_infile = 0\n")
	assert.Contains(t, out, "sql_mode = "+sqlModeMySQL+"\n", "MySQL 8 mantém ONLY_FULL_GROUP_BY")
	assert.Contains(t, out, "character_set_server = utf8mb4\n")
	assert.Contains(t, out, "collation_server = utf8mb4_unicode_ci\n")
	assert.NotContains(t, out, "NO_AUTO_CREATE_USER", "não existe no MySQL 8.0")
	assert.NotContains(t, out, "character_set_collations", "opção só do MariaDB")
	assert.NotContains(t, out, "default_collation_for_utf8mb4")
	assert.NotContains(t, out, "bind-address", "bind fica no outro drop-in")
	assert.NotContains(t, out, "bind_address")
}

func TestDEPLOY02_ConteudoHardening_MariaDB(t *testing.T) {
	casos := []struct {
		versao string
		csc    bool
	}{
		{versaoMariaDB, true},
		{"11.4.4-MariaDB-ubu2404", true},
		{"11.2.1-MariaDB", true},
		{"11.1.6-MariaDB", false},
		{"10.11.6-MariaDB-0ubuntu0.24.04.1", false},
	}
	for _, c := range casos {
		t.Run(c.versao, func(t *testing.T) {
			out, err := bash(t, `conteudo_dropin_hardening mariadb "`+c.versao+`"`)
			require.NoError(t, err, out)
			assert.Contains(t, out, "local_infile = 0\n")
			assert.Contains(t, out, "sql_mode = "+sqlModeMariaDB+"\n")
			assert.NotContains(t, out, "ONLY_FULL_GROUP_BY")
			assert.Contains(t, out, "collation_server = utf8mb4_unicode_ci\n")
			assert.NotContains(t, out, "NO_AUTO_CREATE_USER")
			assert.NotContains(t, out, "default_collation_for_utf8mb4")
			assert.NotContains(t, out, "bind-address")
			if c.csc {
				assert.Contains(t, out, "character_set_collations = utf8mb4=utf8mb4_unicode_ci\n")
			} else {
				assert.NotContains(t, out, "character_set_collations")
			}
		})
	}
}

func TestDEPLOY02_ConteudoBind_SoBindAddress(t *testing.T) {
	out, err := bash(t, `conteudo_dropin_bind`)
	require.NoError(t, err, out)
	var efetivas []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" && !strings.HasPrefix(l, "#") {
			efetivas = append(efetivas, l)
		}
	}
	assert.Equal(t, []string{"[mysqld]", "bind-address = 0.0.0.0"}, efetivas)
}

func TestDEPLOY02_SqlModeIgual_ComoConjunto(t *testing.T) {
	out, err := bash(t, `
sql_mode_igual "STRICT_TRANS_TABLES,ONLY_FULL_GROUP_BY" "only_full_group_by, STRICT_TRANS_TABLES" && echo A
sql_mode_igual "$(sql_mode_esperado mysql)" "NO_ENGINE_SUBSTITUTION,ERROR_FOR_DIVISION_BY_ZERO,NO_ZERO_DATE,NO_ZERO_IN_DATE,STRICT_TRANS_TABLES,ONLY_FULL_GROUP_BY" && echo B
sql_mode_igual "STRICT_TRANS_TABLES" "STRICT_TRANS_TABLES,ONLY_FULL_GROUP_BY" || echo C
sql_mode_igual "" "STRICT_TRANS_TABLES" || echo D
sql_mode_igual "" "" && echo E
`)
	require.NoError(t, err, out)
	assert.Equal(t, "A\nB\nC\nD\nE\n", out)
}

func TestDEPLOY02_GravarSeMudou(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("bash", "-c", `source "$1"
install() { local n=$#; cp -- "${@:n-1:1}" "${@:n:1}"; }
printf 'x\n' >"$D/tmp"
# set -e vem do script: o retorno 1 só pode ser lido dentro de um if.
r() { if gravar_se_mudou "$D/tmp" "$D/dst"; then echo "$1:0"; else echo "$1:1"; fi; }
r 1
r 2
printf 'y\n' >"$D/tmp"
r 3
cat "$D/dst"`, "deploy02", script(t))
	cmd.Env = append(ambienteLimpo(), "D="+dir)
	b, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", b)
	assert.Equal(t, "1:0\n2:1\n3:0\ny\n", string(b))
}

// ---------------------------------------------------------------------------
// etapa_bind_address com stubs
// ---------------------------------------------------------------------------

func TestDEPLOY02_BindAberto_SemRestart_ComSetGlobal(t *testing.T) {
	for _, sabor := range []struct{ nome, versao string }{{"mysql", versaoMySQL}, {"mariadb", versaoMariaDB}} {
		t.Run(sabor.nome, func(t *testing.T) {
			r := rodar(t, cenario{
				sabor: sabor.nome, versao: sabor.versao, bind: "0.0.0.0",
				vars: varsRuins(sabor.nome), varsAposSet: varsBons(sabor.nome),
			}, "")
			require.NoError(t, r.err, r.saida)
			assert.Equal(t, 0, r.conta("systemctl restart"), "bind aberto não reinicia: %v", r.log)
			assert.Equal(t, 1, r.conta("install "+dropinHard), "hardening gravado mesmo com bind aberto")
			assert.Equal(t, 0, r.conta("install "+dropinBind), "bind não é gravado")
			assert.Equal(t, 1, r.conta("validar "), "configuração validada antes de aplicar")
			require.Equal(t, 1, r.conta("sql SET GLOBAL"), "valores divergentes aplicados em execução")
			set := r.log[r.indice("sql SET GLOBAL")]
			assert.Contains(t, set, "local_infile = 0")
			assert.Contains(t, set, "collation_server = 'utf8mb4_unicode_ci'")
			assert.NotContains(t, set, "PERSIST")
			if sabor.nome == "mysql" {
				assert.Contains(t, set, "sql_mode = '"+sqlModeMySQL+"'")
				assert.NotContains(t, set, "character_set_collations")
				assert.Contains(t, r.log[r.indice("validar ")], "validar mysqld --validate-config --user=mysql")
			} else {
				assert.Contains(t, set, "character_set_collations = 'utf8mb4=utf8mb4_unicode_ci'")
				assert.Contains(t, r.log[r.indice("validar ")], "validar mariadbd --help --verbose --user=mysql")
			}
			_, temBind := r.arquivo(t, dropinBind)
			assert.False(t, temBind)
			assert.NotContains(t, r.saida, "AVISO: SHOW GLOBAL VARIABLES ainda diverge", "conferência ok depois do SET GLOBAL")
		})
	}
}

func TestDEPLOY02_BindLoopback_UmRestart(t *testing.T) {
	r := rodar(t, cenario{
		sabor: "mysql", versao: versaoMySQL, bind: "127.0.0.1",
		vars: varsBons("mysql"),
	}, "")
	require.NoError(t, r.err, r.saida)
	assert.Equal(t, 1, r.conta("systemctl restart mysql"), "%v", r.log)
	assert.Equal(t, 1, r.conta("install "+dropinBind))
	assert.Equal(t, 1, r.conta("install "+dropinHard))
	assert.Equal(t, 0, r.conta("sql SET GLOBAL"), "valores já conferem")
	assert.Less(t, r.indice("validar "), r.indice("systemctl restart"), "valida antes do restart")
	bind, ok := r.arquivo(t, dropinBind)
	require.True(t, ok)
	assert.Contains(t, bind, "bind-address = 0.0.0.0\n")
	assert.Contains(t, r.saida, "bind-address: 0.0.0.0")
}

func TestDEPLOY02_Reexecucao_SemGravacaoNemRestart(t *testing.T) {
	dir := t.TempDir()
	c := cenario{sabor: "mysql", versao: versaoMySQL, bind: "127.0.0.1", vars: varsRuins("mysql"), varsAposSet: varsBons("mysql")}
	r1 := rodar(t, c, dir)
	require.NoError(t, r1.err, r1.saida)
	require.Equal(t, 1, r1.conta("systemctl restart"))

	// Segunda execução: estado do servidor depois da primeira (bind já aberto
	// pelo restart do stub, vars corrigidas pelo SET GLOBAL).
	r2 := rodar(t, cenario{sabor: "mysql", versao: versaoMySQL}, dir)
	require.NoError(t, r2.err, r2.saida)
	assert.Equal(t, 0, r2.conta("install "), "nenhuma gravação: %v", r2.log)
	assert.Equal(t, 0, r2.conta("systemctl restart"))
	assert.Equal(t, 0, r2.conta("sql SET GLOBAL"))
	assert.Equal(t, 0, r2.conta("validar "), "sem mudança, sem validação")
	assert.Contains(t, r2.saida, "ja estava atualizado")
}

func TestDEPLOY02_MigracaoLegado_BindAntesDeRemover(t *testing.T) {
	legado := "# Gerado por deploy/setup-servidor.sh (rotaperfumes)\n[mysqld]\nbind-address = 0.0.0.0\nlocal_infile = 0\n" +
		"sql_mode = " + sqlModeMariaDB + "\ncharacter_set_server = utf8mb4\ncollation_server = utf8mb4_unicode_ci\n"
	// O stub de rm registra a remoção do legado para conferir a ordem.
	r := rodar(t, cenario{
		sabor: "mysql", versao: versaoMySQL, bind: "0.0.0.0",
		vars: varsTSV("mysql", "OFF", sqlModeMariaDB, "utf8mb4_unicode_ci", ""), varsAposSet: varsBons("mysql"),
		arquivos: map[string]string{dropinLegado: legado},
		comando: `rm() { local a; for a in "$@"; do [[ "$a" == */zz-rotaperfumes.cnf ]] && reg "rm legado"; done; command rm "$@"; }
etapa_bind_address`,
	}, "")
	require.NoError(t, r.err, r.saida)
	_, temLegado := r.arquivo(t, dropinLegado)
	assert.False(t, temLegado, "legado removido")
	bind, ok := r.arquivo(t, dropinBind)
	require.True(t, ok, "bind preservado no arquivo novo")
	assert.Contains(t, bind, "bind-address = 0.0.0.0\n")
	iBind, iRm := r.indice("install "+dropinBind), r.indice("rm legado")
	require.GreaterOrEqual(t, iBind, 0, "%v", r.log)
	require.GreaterOrEqual(t, iRm, 0, "%v", r.log)
	assert.Less(t, iBind, iRm, "o -bind.cnf é gravado ANTES de remover o legado")
	assert.Less(t, iRm, r.indice("install "+dropinHard))
	assert.Equal(t, 0, r.conta("systemctl restart"), "migração sem restart")
	assert.Equal(t, 1, r.conta("sql SET GLOBAL"), "MySQL sem ONLY_FULL_GROUP_BY em execução: SET GLOBAL")
	hard, ok := r.arquivo(t, dropinHard)
	require.True(t, ok)
	assert.NotContains(t, hard, "bind-address")
}

func TestDEPLOY02_FalhaNaValidacao_RestauraSemRestart(t *testing.T) {
	anterior := "[mysqld]\nlocal_infile = 0\n"
	r := rodar(t, cenario{
		sabor: "mysql", versao: versaoMySQL, bind: "127.0.0.1", vars: varsRuins("mysql"),
		arquivos: map[string]string{dropinHard: anterior},
		env:      []string{"FAKE_VALIDA_RC=1"},
	}, "")
	require.Error(t, r.err, r.saida)
	assert.Contains(t, r.saida, "ERRO: Configuracao do MySQL invalida")
	assert.Equal(t, 0, r.conta("systemctl restart"), "%v", r.log)
	assert.Equal(t, 0, r.conta("sql SET GLOBAL"))
	assert.Equal(t, 0, r.conta("install "+dropinBind))
	hard, ok := r.arquivo(t, dropinHard)
	require.True(t, ok)
	assert.Equal(t, anterior, hard, "arquivo anterior restaurado")
}

func TestDEPLOY02_FalhaNaValidacao_RestauraLegado(t *testing.T) {
	legado := "[mysqld]\nbind-address = 0.0.0.0\nlocal_infile = 0\n"
	r := rodar(t, cenario{
		sabor: "mysql", versao: versaoMySQL, bind: "0.0.0.0", vars: varsRuins("mysql"),
		arquivos: map[string]string{dropinLegado: legado},
		env:      []string{"FAKE_VALIDA_RC=1"},
	}, "")
	require.Error(t, r.err, r.saida)
	assert.Equal(t, 0, r.conta("systemctl restart"))
	_, temHard := r.arquivo(t, dropinHard)
	assert.False(t, temHard, "hardening novo removido (não existia antes)")
	got, ok := r.arquivo(t, dropinLegado)
	require.True(t, ok, "legado devolvido no rollback")
	assert.Equal(t, legado, got)
}

func TestDEPLOY02_FalhaNoRestart_RestauraEReinicia(t *testing.T) {
	r := rodar(t, cenario{
		sabor: "mariadb", versao: versaoMariaDB, bind: "127.0.0.1", vars: varsBons("mariadb"),
		env: []string{"FAKE_FALHA_RESTART=1"},
	}, "")
	require.Error(t, r.err, r.saida)
	assert.Contains(t, r.saida, "ERRO: Restart do mariadb falhou")
	assert.Equal(t, 2, r.conta("systemctl restart mariadb"), "restaura e reinicia de novo: %v", r.log)
	_, temBind := r.arquivo(t, dropinBind)
	assert.False(t, temBind, "bind novo removido (não existia antes)")
	_, temHard := r.arquivo(t, dropinHard)
	assert.True(t, temHard, "hardening (validado) continua")
}

func TestDEPLOY02_Avisos(t *testing.T) {
	t.Run("divergencia persistente", func(t *testing.T) {
		// SET GLOBAL "não pega" (vars não mudam): a conferência avisa.
		r := rodar(t, cenario{sabor: "mysql", versao: versaoMySQL, bind: "0.0.0.0", vars: varsRuins("mysql")}, "")
		require.NoError(t, r.err, r.saida, "conferência só avisa")
		assert.Contains(t, r.saida, "AVISO: SHOW GLOBAL VARIABLES ainda diverge do drop-in em: ")
		assert.Regexp(t, `ainda diverge do drop-in em: [a-z_ ]*local_infile`, r.saida)
		assert.Regexp(t, `ainda diverge do drop-in em: [a-z_ ]*collation_server`, r.saida)
		assert.NotRegexp(t, `ainda diverge do drop-in em: [a-z_ ]*sql_mode`, r.saida, "sql_mode do MySQL confere")
	})
	t.Run("persisted_variables no MySQL", func(t *testing.T) {
		r := rodar(t, cenario{
			sabor: "mysql", versao: versaoMySQL, bind: "0.0.0.0", vars: varsBons("mysql"),
			extras: map[string]string{"persistidas": "local_infile\nsql_mode\n"},
		}, "")
		require.NoError(t, r.err, r.saida)
		assert.Contains(t, r.saida, "AVISO: SET PERSIST ativo para: local_infile sql_mode")
		assert.Contains(t, r.saida, "RESET PERSIST")
	})
	t.Run("persisted_variables nao consultada no MariaDB", func(t *testing.T) {
		r := rodar(t, cenario{sabor: "mariadb", versao: versaoMariaDB, bind: "0.0.0.0", vars: varsBons("mariadb")}, "")
		require.NoError(t, r.err, r.saida)
		assert.Equal(t, 0, r.conta("sql SELECT VARIABLE_NAME FROM performance_schema.persisted_variables"))
	})
	t.Run("collation do schema e das tabelas", func(t *testing.T) {
		r := rodar(t, cenario{
			sabor: "mysql", versao: versaoMySQL, bind: "0.0.0.0", vars: varsBons("mysql"),
			extras: map[string]string{"schema": "utf8mb4_0900_ai_ci\n", "tabelas": "clientes\nprodutos\n"},
		}, "")
		require.NoError(t, r.err, r.saida)
		assert.Contains(t, r.saida, "AVISO: Schema rotaperfumes com collation utf8mb4_0900_ai_ci")
		assert.Contains(t, r.saida, "AVISO: 2 tabela(s) de rotaperfumes fora de utf8mb4_unicode_ci: clientes produtos")
		for _, l := range r.log {
			assert.NotContains(t, strings.ToUpper(l), "ALTER ", "conferência não faz ALTER")
		}
	})
	t.Run("tudo certo sem avisos", func(t *testing.T) {
		r := rodar(t, cenario{
			sabor: "mysql", versao: versaoMySQL, bind: "0.0.0.0", vars: varsBons("mysql"),
			extras: map[string]string{"schema": "utf8mb4_unicode_ci\n"},
		}, "")
		require.NoError(t, r.err, r.saida)
		assert.NotContains(t, r.saida, "AVISO")
		assert.Contains(t, r.saida, "conferido: local_infile=OFF")
	})
}

func TestDEPLOY02_ApenasBanco_SemEnvSemJenkinsSemDocker(t *testing.T) {
	r := rodar(t, cenario{
		sabor: "mysql", versao: versaoMySQL, bind: "0.0.0.0", vars: varsRuins("mysql"), varsAposSet: varsBons("mysql"),
		comando: `checar_root() { reg "checar_root"; }
detectar_mysql_bin() { MYSQL_BIN=mysql; reg "detectar_mysql_bin"; }
preparar_temporarios() { :; }
checar_jenkins() { reg "PROIBIDO checar_jenkins"; }
checar_compose() { reg "PROIBIDO checar_compose"; }
checar_entradas() { reg "PROIBIDO checar_entradas"; }
checar_root_e_usuario() { reg "PROIBIDO checar_root_e_usuario"; }
docker() { reg "PROIBIDO docker $*"; }
etapa_jenkins_docker() { reg "PROIBIDO etapa_jenkins_docker"; }
MYSQL_SABOR=""; MYSQL_VERSAO=""; MYSQL_SERVICO=""
main --apenas-banco`,
	}, "")
	require.NoError(t, r.err, r.saida)
	assert.Equal(t, 0, r.conta("PROIBIDO"), "%v", r.log)
	assert.Equal(t, 1, r.conta("checar_root"))
	assert.Equal(t, 1, r.conta("sql SELECT VERSION()"), "detectar_sabor_e_servico rodou")
	assert.Equal(t, 1, r.conta("install "+dropinHard))
	assert.Equal(t, 1, r.conta("sql SET GLOBAL"))
	assert.Equal(t, 0, r.conta("systemctl restart"))
	assert.Contains(t, r.saida, "Concluido (--apenas-banco).")
	assert.NotContains(t, r.saida, "[4/")
}

func TestDEPLOY02_ValidarArgumentos(t *testing.T) {
	out, err := bash(t, `ler_argumentos --apenas-banco && echo OK`)
	require.NoError(t, err, out)
	assert.Equal(t, "OK\n", out)

	out, err = bash(t, `ler_argumentos --skip-build`)
	require.Error(t, err)
	assert.Contains(t, out, "Informe --env ARQUIVO")

	out, err = bash(t, `uso`)
	require.NoError(t, err)
	assert.Contains(t, out, "--apenas-banco")
}

func TestDEPLOY02_AvisoTurnstileTeste_SemImprimirValor(t *testing.T) {
	casos := map[string]bool{
		"TURNSTILE_SECRET_KEY=1x0000000000000000000000000000000AA\n":            true,
		"export TURNSTILE_SECRET_KEY='2x0000000000000000000000000000000AA'\r\n": true,
		"TURNSTILE_SECRET_KEY = \"3x0000000000000000000000000000000AA\"\n":      true,
		"TURNSTILE_SECRET_KEY=0x4AAAAAAAreal\n":                                 false,
		"# TURNSTILE_SECRET_KEY=1x0000000000000000000000000000000AA\n":          false,
	}
	for conteudo, teste := range casos {
		t.Run(strings.TrimSpace(conteudo), func(t *testing.T) {
			arq := filepath.Join(t.TempDir(), "origem.env")
			require.NoError(t, os.WriteFile(arq, []byte(conteudo), 0o600))
			cmd := exec.Command("bash", "-c", `source "$1"; ARQ_ENV=$2; avisar_turnstile_teste`, "deploy02", script(t), arq)
			cmd.Env = ambienteLimpo()
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", out)
			if teste {
				assert.Contains(t, string(out), "secret de TESTE")
			} else {
				assert.Empty(t, string(out))
			}
			assert.NotContains(t, string(out), "0000000000", "não imprime o valor")
		})
	}
}

// Valida os drop-ins num mysqld de verdade (contêiner descartável), se houver
// Docker acessível.
func TestDEPLOY02_ValidateConfig_Docker(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker não está no PATH")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker inacessível: " + err.Error())
	}
	casos := []struct {
		imagem, sabor, versao string
		args                  []string
	}{
		{"mysql:8.0", "mysql", versaoMySQL, []string{"mysqld", "--validate-config", "--user=mysql"}},
		{"mariadb:11.8", "mariadb", versaoMariaDB, []string{"mariadbd", "--help", "--verbose", "--user=mysql"}},
	}
	for _, c := range casos {
		t.Run(c.imagem, func(t *testing.T) {
			if err := exec.Command("docker", "image", "inspect", c.imagem).Run(); err != nil {
				t.Skip("imagem " + c.imagem + " não está no cache local")
			}
			dir := t.TempDir()
			hard, err := bash(t, `conteudo_dropin_hardening `+c.sabor+` `+c.versao)
			require.NoError(t, err)
			bind, err := bash(t, `conteudo_dropin_bind`)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, dropinHard), []byte(hard), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(dir, dropinBind), []byte(bind), 0o644))
			args := []string{"run", "--rm",
				"-v", filepath.Join(dir, dropinHard) + ":/etc/mysql/conf.d/" + dropinHard + ":ro",
				"-v", filepath.Join(dir, dropinBind) + ":/etc/mysql/conf.d/" + dropinBind + ":ro",
				c.imagem}
			out, err := exec.Command("docker", append(args, c.args...)...).CombinedOutput()
			require.NoError(t, err, "%s", out)
		})
	}
}
