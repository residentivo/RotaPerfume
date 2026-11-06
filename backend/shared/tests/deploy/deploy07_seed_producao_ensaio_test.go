package deploy_test

// DEPLOY-07 (Lote 33, TestBrain): ensaio do deploy/seed-producao.sh REAL com
// docker FALSO (stub no PATH), num user namespace (EUID 0) com binds de
// /root e /etc/mysql para um TempDir. Nada conecta em banco nem sobe
// container. Confere:
//   - recusa (fail closed) com a tabela usuarios preenchida, sem DDL/seed;
//   - ordem: DDL de sql/ordem-ddl.txt -> 02 -> 03 -> UPDATE ativo=0 ->
//     compilação -> -create-admin (nunca -all-users);
//   - todo cliente mariadb com --network rotaperfumes-db --ip 172.30.42.20,
//     --ssl-verify-server-cert e MYSQL_PWD só pelo ambiente;
//   - senha do banco, pepper e senha do admin nunca no argv nem na saída;
//   - env-file 600 com as variáveis certas, apagado no fim (sucesso e erro);
//   - senha do admin gravada 600 no destino, sem sobrescrever arquivo.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubDockerSeed: cada chamada vai para $FAKE/log (argv numa linha) e
// $FAKE/calls/N.{argv,stdin}. Respostas das consultas pelo texto do SQL.
const stubDockerSeed = `#!/bin/bash
set -u
C="$FAKE/calls"; mkdir -p "$C"
n=$(( $(cat "$C/n" 2>/dev/null || echo 0) + 1 )); echo "$n" >"$C/n"
printf '%s\n' "$*" >>"$FAKE/log"
printf '%s\n' "$@" >"$C/$n.argv"
[[ -n "${MYSQL_PWD:-}" ]] && printf '%s' "$MYSQL_PWD" >"$C/$n.mysql_pwd"
case "${1:-}" in
  info) exit 0 ;;
  inspect)
    # TST-17: estado do container e as redes ligadas a ele (chaves de
    # .NetworkSettings.Networks), pelo formato pedido.
    case "$*" in
      *State.Running*) cat "$FAKE/state/running" 2>/dev/null || echo true ;;
      *NetworkSettings.Networks*) cat "$FAKE/state/redes" 2>/dev/null || echo rotaperfumes-db ;;
      *) echo true ;;
    esac
    exit 0 ;;
  network)
    [[ -f "$FAKE/state/sem_rede" ]] && exit 1
    if [[ "$*" == *IPAM.Config* ]]; then cat "$FAKE/state/subnets" 2>/dev/null || echo "172.30.42.0/24 "; fi
    exit 0 ;;
  run) ;;
  *) echo "docker stub: $*" >&2; exit 2 ;;
esac
args=("$@")
fonte_de() { # fonte_de TARGET: source do --mount com esse target
  local i
  for ((i=0; i<${#args[@]}; i++)); do
    if [[ "${args[i]}" == --mount && "${args[i+1]}" == *",target=$1"* ]]; then
      local m=${args[i+1]#*source=}; printf '%s' "${m%%,*}"; return 0
    fi
  done
  return 1
}
if [[ " $* " == *" --entrypoint mariadb "* ]]; then
  q=""
  for ((i=0; i<${#args[@]}; i++)); do
    [[ "${args[i]}" == -e && "${args[i+1]}" != MYSQL_PWD ]] && q=${args[i+1]}
  done
  if [[ -z "$q" ]]; then cat >"$C/$n.stdin"; q=$(<"$C/$n.stdin"); else printf '%s' "$q" >"$C/$n.stdin"; fi
  case "$q" in
    "SELECT 1") echo 1 ;;
    *"TABLE_NAME = 'usuarios'"*) cat "$FAKE/state/tem_usuarios" 2>/dev/null || echo 0 ;;
    "SELECT COUNT(*) FROM usuarios") cat "$FAKE/state/linhas" ;;
    *"role <> 'admin' AND ativo = 0"*) echo 42 ;;
    *"role <> 'admin' AND ativo <> 0"*) echo 0 ;;
    *"information_schema.TABLES"*) echo 13 ;;
    *argon2id*) echo "1 1 1" ;;
  esac
  [[ -f "$FAKE/state/falha_sql_$n" ]] && exit 1
  exit 0
fi
if [[ " $* " == *" sh -ec "* ]]; then
  cat >/dev/null
  out=$(fonte_de /out) || exit 3
  printf '#!/bin/sh\n' >"$out/resetpassword"; chmod 755 "$out/resetpassword"
  exit 0
fi
if [[ " $* " == *" -create-admin "* ]]; then
  for ((i=0; i<${#args[@]}; i++)); do
    if [[ "${args[i]}" == --env-file ]]; then
      env=${args[i+1]}
      printf '%s' "$env" >"$FAKE/state/env_path"
      stat -c '%a' "$env" >"$FAKE/state/env_modo"
      cp "$env" "$FAKE/state/env_conteudo"
    fi
  done
  [[ -f "$FAKE/state/falha_create_admin" ]] && exit 1
  saida=$(fonte_de /saida) || exit 3
  printf 'SenhaAdminGeradaXyz123456789\n' >"$saida/admin.senha"
  exit 0
fi
echo "docker stub: run inesperado: $*" >&2
exit 2
`

const stubInstallSeed = `#!/bin/bash
args=()
while [[ $# -gt 0 ]]; do case "$1" in -o|-g) shift 2 ;; *) args+=("$1"); shift ;; esac; done
exec /usr/bin/install "${args[@]}"
`

// stubMktempSeed registra cada mktemp (TST-17: nenhuma recusa chega nele; sem
// opções, o destino é o /root).
const stubMktempSeed = `#!/bin/bash
printf '%s\n' "$*" >>"$FAKE/mktemp.log"
exec /usr/bin/mktemp "$@"
`

// runnerSeed roda o script real num user namespace (EUID 0) + mount
// namespace. TST-17 (Lote 37): o seed confere o caminho real e os
// ancestrais (root, sem escrita de grupo/outros) do --segredos-dir,
// --trabalho-dir e --ssl-dir. No namespace o "/" e o "/etc" reais aparecem
// com o overflow uid (65534) e o /tmp é 1777; por isso, como o runnerEnsaio
// do TST-15/16, ele faz chroot numa raiz tmpfs (dona: o root do namespace,
// 755) com rbind dos diretórios de topo. O /etc é recriado na tmpfs com bind
// de cada entrada (o /etc/mysql vem do $FAKE/etcmysql) e o topo do $FAKE
// (ex.: /tmp) é recriado vazio (755) com só o $FAKE e o deploy/ expostos.
const runnerSeed = `#!/bin/bash
set -euo pipefail
umask 022
NR="$FAKE/.raiz"
mkdir -p "$NR"
mount -t tmpfs -o mode=755 tmpfs "$NR"
topo=${FAKE#/}; topo=/${topo%%/*}
for x in /*; do
  [[ "$x" == "$topo" ]] && continue
  if [[ -L "$x" ]]; then ln -s "$(readlink "$x")" "$NR$x"
  elif [[ "$x" == /etc ]]; then
    mkdir "$NR/etc"
    for y in /etc/*; do
      [[ "$y" == /etc/mysql ]] && continue
      if [[ -L "$y" ]]; then ln -s "$(readlink "$y")" "$NR$y"
      elif [[ -d "$y" ]]; then mkdir "$NR$y"; mount --rbind "$y" "$NR$y" 2>/dev/null || true
      elif [[ -e "$y" ]]; then : >"$NR$y"; mount --bind "$y" "$NR$y" 2>/dev/null || true
      fi
    done
  elif [[ -d "$x" ]]; then mkdir "$NR$x"; mount --rbind "$x" "$NR$x"
  fi
done
mkdir -p "$NR$topo" "$NR/etc/mysql"
expor() { [[ "$1" == "$topo"/* ]] || return 0; mkdir -p "$NR$1"; mount --bind "$1" "$NR$1"; }
expor "$FAKE"
expor "$(dirname "$SCRIPT")"
mount --bind "$FAKE/root" "$NR/root"
mount --bind "$FAKE/etcmysql" "$NR/etc/mysql"
cd /
export PATH="$FAKE/bin:$PATH"
umask 077
exec chroot "$NR" /bin/bash -c 'cd / && exec bash "$0" "$@"' "$SCRIPT" "$@"
`

const (
	senhaDBSeed  = "SenhaDoBancoAdminAbc123456789XYZ"
	pepperSeed   = "pepper-de-teste-com-pelo-menos-32-bytes-ok"
	senhaAdmSeed = "SenhaAdminGeradaXyz123456789"
)

type ensaioSeed struct {
	t   *testing.T
	dir string
}

func novoEnsaioSeed(t *testing.T) *ensaioSeed {
	t.Helper()
	precisaUnshare(t)
	for _, c := range []string{"shred", "tar", "stat", "mktemp", "realpath", "chroot"} {
		if _, err := exec.LookPath(c); err != nil {
			t.Skip(c + " não está no PATH")
		}
	}
	if _, err := os.Stat("/usr/bin/mktemp"); err != nil {
		t.Skip("/usr/bin/mktemp não existe (stub do mktemp)")
	}
	e := &ensaioSeed{t: t, dir: t.TempDir()}
	// TST-17: o t.TempDir() cria o 00N com 0777 &^ umask (775 com umask 002),
	// e o seed recusa ancestral com escrita para o grupo.
	require.NoError(t, os.Chmod(e.dir, 0o700))
	for _, d := range []string{"root/rotaperfumes-mariadb", "etcmysql/ssl", "state", "bin"} {
		require.NoError(t, os.MkdirAll(filepath.Join(e.dir, d), 0o755))
	}
	for _, d := range []string{"root", "etcmysql", "etcmysql/ssl"} {
		require.NoError(t, os.Chmod(filepath.Join(e.dir, d), 0o755))
	}
	// TST-17: /root e o --segredos-dir precisam ser 700 (conferir_dir_privado).
	require.NoError(t, os.Chmod(filepath.Join(e.dir, "root"), 0o700))
	require.NoError(t, os.Chmod(filepath.Join(e.dir, "root", "rotaperfumes-mariadb"), 0o700))
	escrever(t, filepath.Join(e.dir, "bin", "docker"), stubDockerSeed, 0o755)
	escrever(t, filepath.Join(e.dir, "bin", "install"), stubInstallSeed, 0o755)
	escrever(t, filepath.Join(e.dir, "bin", "mktemp"), stubMktempSeed, 0o755)
	escrever(t, filepath.Join(e.dir, "runner.sh"), runnerSeed, 0o755)
	escrever(t, filepath.Join(e.dir, "etcmysql", "ssl", "ca.pem"), "-----BEGIN CERTIFICATE-----\nx\n-----END CERTIFICATE-----\n", 0o644)
	escrever(t, filepath.Join(e.dir, "root", "rotaperfumes-mariadb", "rotaperfumes_admin.senha"), senhaDBSeed+"\n", 0o600)
	escrever(t, filepath.Join(e.dir, "root", "pepper.txt"), pepperSeed+"\n", 0o600)
	e.state("tem_usuarios", "1\n")
	e.state("linhas", "0\n")
	return e
}

func (e *ensaioSeed) state(nome, v string) {
	escrever(e.t, filepath.Join(e.dir, "state", nome), v, 0o644)
}

func (e *ensaioSeed) ler(rel string) string {
	b, err := os.ReadFile(filepath.Join(e.dir, rel))
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(e.t, err)
	return string(b)
}

func (e *ensaioSeed) rodar(args ...string) (string, error) {
	e.t.Helper()
	out, err := e.comando(args...).CombinedOutput()
	return string(out), err
}

// comando monta a execução do script real no ensaio (BUG-26: o caso sem
// terminal ajusta o SysProcAttr antes de rodar).
func (e *ensaioSeed) comando(args ...string) *exec.Cmd {
	e.t.Helper()
	cmd := exec.Command("unshare", append([]string{"-rm", "bash", filepath.Join(e.dir, "runner.sh")}, args...)...)
	cmd.Env = append(ambienteLimpo(), "FAKE="+e.dir, "SCRIPT="+caminhoRaiz(e.t, "deploy", "seed-producao.sh"))
	return cmd
}

type chamadaDocker struct {
	argv  []string
	stdin string
	pwd   string
}

func (e *ensaioSeed) chamadas() []chamadaDocker {
	e.t.Helper()
	nStr := strings.TrimSpace(e.ler("calls/n"))
	if nStr == "" {
		return nil
	}
	n, err := strconv.Atoi(nStr)
	require.NoError(e.t, err)
	out := make([]chamadaDocker, 0, n)
	for i := 1; i <= n; i++ {
		p := "calls/" + strconv.Itoa(i)
		out = append(out, chamadaDocker{
			argv:  strings.Split(strings.TrimSuffix(e.ler(p+".argv"), "\n"), "\n"),
			stdin: e.ler(p + ".stdin"),
			pwd:   e.ler(p + ".mysql_pwd"),
		})
	}
	return out
}

func (c chamadaDocker) tem(s string) bool {
	for _, a := range c.argv {
		if a == s {
			return true
		}
	}
	return false
}

func (c chamadaDocker) mariadb() bool {
	return c.tem("mariadb") && c.tem("--entrypoint") && c.argv[0] == "run"
}

// rotuloSQL identifica o conteúdo enviado ao cliente mariadb: nome do
// arquivo de sql/ (conteúdo idêntico) ou o próprio texto.
func rotuloSQL(stdin string, arquivos map[string]string) string {
	for nome, conteudo := range arquivos {
		if stdin == conteudo {
			return nome
		}
	}
	return stdin
}

func arquivosSQL(t *testing.T) (ddl []string, conteudos map[string]string) {
	t.Helper()
	conteudos = map[string]string{}
	for _, l := range linhasUteis(lerRaiz(t, "sql", "ordem-ddl.txt")) {
		ddl = append(ddl, l)
		conteudos[l] = lerRaiz(t, "sql", l)
	}
	for _, s := range []string{"02_seed_admin.sql", "03_seed_vendedores.sql"} {
		conteudos[s] = lerRaiz(t, "sql", s)
	}
	return ddl, conteudos
}

func TestDEPLOY07_SeedProducao_Ensaio_Sucesso(t *testing.T) {
	e := novoEnsaioSeed(t)
	out, err := e.rodar("--pepper-file", "/root/pepper.txt")
	require.NoError(t, err, out)
	assert.Contains(t, out, "=== [7/7] Conferencia ===")
	assert.Contains(t, out, "Admin ativo, Argon2id, troca obrigatoria no 1o login: OK.")

	ddl, conteudos := arquivosSQL(t)
	cs := e.chamadas()
	require.NotEmpty(t, cs)

	// Sequência de efeitos: DDL na ordem do arquivo, 02, 03, UPDATE, build, -create-admin.
	var efeitos []string
	for _, c := range cs {
		switch {
		case c.mariadb():
			r := rotuloSQL(c.stdin, conteudos)
			if _, ok := conteudos[r]; ok || strings.HasPrefix(r, "UPDATE") {
				efeitos = append(efeitos, r)
			}
		case c.tem("sh") && c.tem("-ec"):
			efeitos = append(efeitos, "build")
		case c.tem("-create-admin"):
			efeitos = append(efeitos, "create-admin")
		}
	}
	esperado := append(append([]string{}, ddl...), "02_seed_admin.sql", "03_seed_vendedores.sql",
		"UPDATE usuarios SET ativo = 0 WHERE role <> 'admin'", "build", "create-admin")
	assert.Equal(t, esperado, efeitos)

	log := e.ler("log")
	assert.NotContains(t, log, "-all-users", "nunca -all-users")
	assert.NotContains(t, out, "-all-users")

	for i, c := range cs {
		if !c.mariadb() {
			continue
		}
		t.Run("cliente mariadb "+strconv.Itoa(i+1), func(t *testing.T) {
			argv := strings.Join(c.argv, " ")
			for _, s := range []string{"--network rotaperfumes-db --ip 172.30.42.20", "--cap-drop ALL",
				"--security-opt no-new-privileges:true", "--ssl-verify-server-cert", "--ssl-ca=/etc/mysql/ssl/ca.pem",
				"--local-infile=0", "-h rotaperfumes-mariadb", "-u rotaperfumes_admin", "-e MYSQL_PWD",
				"type=bind,source=/etc/mysql/ssl/ca.pem,target=/etc/mysql/ssl/ca.pem,readonly"} {
				assert.Contains(t, argv, s)
			}
			assert.Equal(t, senhaDBSeed, c.pwd, "senha do banco só via MYSQL_PWD no ambiente")
		})
	}

	// create-admin: rede/IP do admin, read-only, env-file, -password-out.
	var adm *chamadaDocker
	for i := range cs {
		if cs[i].tem("-create-admin") {
			adm = &cs[i]
		}
	}
	require.NotNil(t, adm)
	argv := strings.Join(adm.argv, " ")
	for _, s := range []string{"--network rotaperfumes-db --ip 172.30.42.20", "--cap-drop ALL", "--read-only",
		"--env-file", "-create-admin -password-out /saida/admin.senha", "target=/usr/local/bin/resetpassword,readonly"} {
		assert.Contains(t, argv, s)
	}
	assert.Empty(t, adm.pwd, "resetpassword não recebe MYSQL_PWD")

	// Segredos nunca no argv nem na saída.
	for nome, v := range map[string]string{"senha do banco": senhaDBSeed, "pepper": pepperSeed, "senha do admin": senhaAdmSeed} {
		assert.NotContains(t, log, v, "%s no argv do docker", nome)
		assert.NotContains(t, out, v, "%s na saída", nome)
	}

	// env-file: 600, variáveis certas, apagado no fim (e a área de trabalho também).
	assert.Equal(t, "600", strings.TrimSpace(e.ler("state/env_modo")))
	env := e.ler("state/env_conteudo")
	for _, s := range []string{"DB_HOST=rotaperfumes-mariadb\n", "DB_PORT=3306\n", "DB_USUARIO=rotaperfumes_admin\n",
		"DB_SENHA=" + senhaDBSeed + "\n", "DB_NAME=rotaperfumes\n", "DB_TLS_CA=/etc/mysql/ssl/ca.pem\n",
		"DB_TLS_SERVER_NAME=rotaperfumes-mariadb\n", "PASSWORD_PEPPER=" + pepperSeed + "\n"} {
		assert.Contains(t, env, s)
	}
	envPath := e.ler("state/env_path")
	require.True(t, strings.HasPrefix(envPath, "/root/rotaperfumes-seed."), envPath)
	trab, _ := filepath.Glob(filepath.Join(e.dir, "root", "rotaperfumes-seed.*"))
	assert.Empty(t, trab, "área de trabalho (env-file, binário, senha) removida")

	// Senha do admin: 600 no destino padrão, só o caminho na saída.
	dest := filepath.Join(e.dir, "root", "rotaperfumes-mariadb", "admin-app.senha")
	st, err := os.Stat(dest)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	assert.Equal(t, senhaAdmSeed+"\n", e.ler("root/rotaperfumes-mariadb/admin-app.senha"))
	assert.Contains(t, out, "/root/rotaperfumes-mariadb/admin-app.senha (600, so root)")

	// Segunda execução: destino já existe = recusa antes de qualquer efeito.
	require.NoError(t, os.Remove(filepath.Join(e.dir, "calls", "n")))
	out2, err := e.rodar("--pepper-file", "/root/pepper.txt")
	require.Error(t, err, out2)
	assert.Contains(t, out2, "admin-app.senha ja existe")
	assert.NotContains(t, out2, "=== [2/7]")
	assert.Equal(t, senhaAdmSeed+"\n", e.ler("root/rotaperfumes-mariadb/admin-app.senha"), "não sobrescreve")
}

// Fail closed: a tabela usuarios com linhas recusa sem --forcar, antes de
// qualquer DDL, seed, UPDATE ou resetpassword.
func TestDEPLOY07_SeedProducao_Ensaio_UsuariosPreenchidaRecusa(t *testing.T) {
	e := novoEnsaioSeed(t)
	e.state("linhas", "5\n")
	out, err := e.rodar("--pepper-file", "/root/pepper.txt")
	require.Error(t, err, out)
	assert.Contains(t, out, "a tabela usuarios ja tem 5 linha(s)")
	assert.Contains(t, out, "Nada foi alterado")
	assert.NotContains(t, out, "=== [3/7]")
	for _, c := range e.chamadas() {
		if c.mariadb() {
			assert.Regexp(t, regexp.MustCompile(`^SELECT `), c.stdin, "só consultas")
		}
		assert.False(t, c.tem("-create-admin"))
		assert.False(t, c.tem("-ec"), "nem compila")
	}
	assert.NoFileExists(t, filepath.Join(e.dir, "root", "rotaperfumes-mariadb", "admin-app.senha"))
	assert.NotContains(t, out, senhaDBSeed)
}

// Erro no -create-admin: o trap apaga o env-file (com pepper e senha do
// banco) e a área de trabalho; nenhuma senha do admin no destino.
func TestDEPLOY07_SeedProducao_Ensaio_FalhaCreateAdminLimpaEnvFile(t *testing.T) {
	e := novoEnsaioSeed(t)
	e.state("falha_create_admin", "")
	out, err := e.rodar("--pepper-file", "/root/pepper.txt")
	require.Error(t, err, out)
	assert.Contains(t, out, "o resetpassword -create-admin falhou")
	assert.Equal(t, "600", strings.TrimSpace(e.ler("state/env_modo")))
	trab, _ := filepath.Glob(filepath.Join(e.dir, "root", "rotaperfumes-seed.*"))
	assert.Empty(t, trab, "env-file e área de trabalho apagados também em erro")
	assert.NoFileExists(t, filepath.Join(e.dir, "root", "rotaperfumes-mariadb", "admin-app.senha"))
	for _, v := range []string{senhaDBSeed, pepperSeed} {
		assert.NotContains(t, out, v)
		assert.NotContains(t, e.ler("log"), v)
	}
}

// Pré-requisitos que falham antes de qualquer chamada ao banco.
func TestDEPLOY07_SeedProducao_Ensaio_PreRequisitos(t *testing.T) {
	casos := []struct {
		nome string
		prep func(e *ensaioSeed)
		args []string
		msg  string
	}{
		{"senha do banco com modo 644", func(e *ensaioSeed) {
			require.NoError(t, os.Chmod(filepath.Join(e.dir, "root", "rotaperfumes-mariadb", "rotaperfumes_admin.senha"), 0o644))
		}, nil, "ausente ou com dono/modo errado"},
		{"pepper com modo 644", func(e *ensaioSeed) {
			require.NoError(t, os.Chmod(filepath.Join(e.dir, "root", "pepper.txt"), 0o644))
		}, nil, "precisa ser arquivo regular do root com modo 600 ou 400"},
		{"pepper curto", func(e *ensaioSeed) {
			escrever(t, filepath.Join(e.dir, "root", "pepper.txt"), "curto\n", 0o600)
		}, nil, "PASSWORD_PEPPER com menos de 32 caracteres"},
		{"CA ausente", func(e *ensaioSeed) {
			require.NoError(t, os.Remove(filepath.Join(e.dir, "etcmysql", "ssl", "ca.pem")))
		}, nil, "CA do banco nao encontrada em /etc/mysql/ssl/ca.pem"},
		{"destino relativo", func(*ensaioSeed) {}, []string{"--admin-senha-arquivo", "rel.senha"}, "precisa de caminho absoluto"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioSeed(t)
			c.prep(e)
			out, err := e.rodar(append([]string{"--pepper-file", "/root/pepper.txt"}, c.args...)...)
			require.Error(t, err, out)
			assert.Contains(t, out, c.msg)
			assert.NotContains(t, out, "=== [2/7]")
			for _, ch := range e.chamadas() {
				assert.False(t, ch.mariadb(), "nenhum cliente mariadb antes dos pré-requisitos")
			}
			assert.NotContains(t, out, pepperSeed)
		})
	}
}
