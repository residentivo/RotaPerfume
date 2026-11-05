package deploy_test

// Stubs do ensaio do deploy/mariadb-docker.sh (TST-15). Ficam em $FAKE/bin,
// no começo do PATH; o estado "do servidor" fica em $FAKE/state e as chamadas
// em $FAKE/log. Nada fala com Docker, iptables ou systemd de verdade.

// docker: rede, container, imagem, exec (SQL do root) e run (clientes).
const stubDocker = `#!/bin/bash
S="$FAKE/state"
echo "docker $*" >>"$FAKE/log"
case "$1" in
  info|build|pull) exit 0 ;;
  ps)
    if [[ "$*" == *-aq* ]]; then [[ -f "$S/criado" ]] && echo cid123; fi
    exit 0 ;;
  network)
    case "$2" in
      create)
        shift 2
        while [[ $# -gt 0 ]]; do
          case "$1" in
            --subnet) echo "$2" >"$S/rede"; shift 2 ;;
            --ip-range) echo "$2" >"$S/iprange"; shift 2 ;;
            --gateway|--driver|--label) shift 2 ;;
            *) shift ;;
          esac
        done
        exit 0 ;;
      inspect)
        [[ -f "$S/rede" ]] || exit 1
        case "$*" in
          *.Subnet*) echo "$(cat "$S/rede") " ;;
          *.IPRange*) cat "$S/iprange" 2>/dev/null ;;
        esac
        exit 0 ;;
    esac
    exit 0 ;;
  inspect)
    [[ -f "$S/criado" ]] || exit 1
    case "$*" in
      *State.Running*) cat "$S/rodando" 2>/dev/null || echo true ;;
      *IPAMConfig*) cat "$S/ip" ;;
      *"len .NetworkSettings"*) echo 1 ;;
      *PortBindings*) cat "$S/portas" ;;
      *ReadonlyRootfs*) cat "$S/readonly" ;;
      *Config.User*) cat "$S/user" ;;
      *"{{.Image}}"*) echo sha256:img ;;
    esac
    exit 0 ;;
  image) echo sha256:img; exit 0 ;;
  port) [[ -f "$S/portas" ]] && grep -q HostPort "$S/portas" && echo "0.0.0.0:3306"; exit 0 ;;
  rm) rm -f "$S/criado"; exit 0 ;;
  start|stop|logs) exit 0 ;;
  run)
    if [[ " $* " == *" -d "* ]]; then
      echo "$*" >"$S/run_args"
      touch "$S/criado"
      ip=""; portas="{}"; ro=false; user=""
      shift
      while [[ $# -gt 0 ]]; do
        case "$1" in
          --ip) ip=$2; shift 2 ;;
          -p) IFS=: read -r h p _ <<<"$2"; portas="{\"3306/tcp\":[{\"HostIp\":\"$h\",\"HostPort\":\"$p\"}]}"; shift 2 ;;
          --read-only) ro=true; shift ;;
          --user) user=$2; shift 2 ;;
          *) shift ;;
        esac
      done
      echo "$ip" >"$S/ip"; echo "$portas" >"$S/portas"; echo "$ro" >"$S/readonly"; echo "$user" >"$S/user"
      exit 0
    fi
    case "$*" in
      *"--entrypoint mariadbd"*) echo "mariadbd  Ver 11.4.13-MariaDB for debian-linux-gnu" ;;
      *"--entrypoint id"*"-u mysql"*) echo 999 ;;
      *"--entrypoint id"*"-g mysql"*) echo 999 ;;
      *--skip-ssl*) [[ -f "$S/aceita_sem_tls" ]] && exit 0; exit 1 ;;
      *Ssl_server_not_after*)
        # DEPLOY-08: validade do certificado em uso = a do arquivo montado
        # (ou $S/cert_em_uso, simulando um mariadbd com outro certificado).
        if [[ -f "$S/cert_em_uso" ]]; then cat "$S/cert_em_uso"; exit 0; fi
        src=$(grep -oE 'source=[^,]*/ca\.pem' <<<"$*" | head -n1); d=${src#source=}; d=${d%/ca.pem}
        fim=$(openssl x509 -in "$d/server-cert.pem" -noout -enddate | cut -d= -f2)
        ini=$(openssl x509 -in "$d/server-cert.pem" -noout -startdate | cut -d= -f2)
        echo "$fim|$ini" ;;
      *--ssl-verify-server-cert*) echo "  conectado como rotaperfumes_admin@172.30.42.20, 11.4.13-MariaDB, TLSv1.3" ;;
    esac
    exit 0 ;;
  exec)
    a="$*"
    if [[ "$a" != *" -e "* ]]; then echo "-- stdin" >>"$S/sql"; cat >>"$S/sql"; exit 0; fi
    case "$a" in
      *skip_networking*) echo 0 ;;
      *"FLUSH SSL"*) echo "FLUSH SSL" >>"$S/sql" ;;
      *"SELECT VERSION()"*) echo 11.4.13-MariaDB ;;
      *have_ssl*) printf 'have_ssl\tYES\n' ;;
      *PLUGINS*) echo 1 ;;
      *"SELECT User, Host"*) cat "$S/contas" 2>/dev/null ;;
      *"CONCAT('    '"*) echo "    (contas)" ;;
      *"mysql.global_priv hc"*) echo "$a" >>"$S/sql_healthcheck"; cat "$S/healthcheck_alem_usage" 2>/dev/null ;;
      *"CONCAT(User, '@', Host)"*) echo "$a" >>"$S/sql_outras"; cat "$S/outras" 2>/dev/null ;;
      *ssl_type*) echo 0 ;;
    esac
    exit 0 ;;
esac
exit 0
`

// iptables com estado: uma chain por arquivo ($S/chain_<nome>), com o IP
// normalizado para /32. Como no iptables real (libxtables), o "-S" omite o
// /32 do --ctorigdst (o -s mantém); com $S/ctorigdst_com_32 o "-S" o mantém
// (DEPLOY-08: o script deve aceitar as duas formas).
const stubIptables = `#!/bin/bash
S="$FAKE/state"
echo "iptables $*" >>"$FAKE/log"
args=("$@")
[[ "${args[0]}" == -w ]] && args=("${args[@]:1}")
op=${args[0]}; chain=${args[1]}; rest=("${args[@]:2}")
if [[ -f "$S/sem_docker_user" && "$chain" == DOCKER-USER ]]; then
  echo "iptables: No chain/target/match by that name." >&2; exit 1
fi
norm() {
  local -a v=("$@") out=(); local i=0 x
  while ((i < ${#v[@]})); do
    out+=("${v[i]}")
    if [[ "${v[i]}" == --ctorigdst || "${v[i]}" == -s ]]; then
      i=$((i + 1)); x=${v[i]}; [[ "$x" == */* ]] || x=$x/32; out+=("$x")
    fi
    i=$((i + 1))
  done
  echo "${out[*]}"
}
f="$S/chain_$chain"
touch "$f"
case "$op" in
  -S) echo "-N $chain"
      if [[ -f "$S/ctorigdst_com_32" ]]; then cat "$f"
      else sed -E 's#(--ctorigdst [0-9.]+)/32( |$)#\1\2#' "$f"; fi ;;
  -C) grep -qxF -- "-A $chain $(norm "${rest[@]}")" "$f" ;;
  -I) r=$(norm "${rest[@]:1}"); { echo "-A $chain $r"; cat "$f"; } >"$f.tmp"; mv "$f.tmp" "$f" ;;
  -D) r=$(norm "${rest[@]}"); { grep -vxF -- "-A $chain $r" "$f" || true; } >"$f.tmp"; mv "$f.tmp" "$f" ;;
  -A) echo "-A $chain $(norm "${rest[@]}")" >>"$f" ;;
  *) echo "iptables stub: op $op" >&2; exit 2 ;;
esac
`

// iptables-restore --noflush: ":CHAIN" esvazia a chain; "-A" acrescenta.
const stubIptablesRestore = `#!/bin/bash
S="$FAKE/state"
echo "iptables-restore $*" >>"$FAKE/log"
while IFS= read -r l; do
  echo "$l" >>"$S/restore_in"
  case "$l" in
    :*) c=${l#:}; c=${c%% *}; : >"$S/chain_$c" ;;
    "-A "*) c=$(awk '{print $2}' <<<"$l"); echo "$l" >>"$S/chain_$c" ;;
  esac
done
`

// systemctl: restart da unit roda o script de firewall gerado (com o
// iptables falso), como o systemd faria.
const stubSystemctl = `#!/bin/bash
echo "systemctl $*" >>"$FAKE/log"
if [[ "$1" == restart && "$2" == rotaperfumes-db-firewall.service ]]; then
  bash /usr/local/sbin/rotaperfumes-db-firewall
  exit $?
fi
exit 0
`

// install/chown sem -o/-g: no user namespace só o uid 0 está mapeado.
// Lote 36 (TOCTOU): com $S/install_troca = caminho, depois do install
// desse caminho o stub o troca por um symlink para $FAKE/desvio (simula a
// troca entre o install -d e a gravacao).
const stubInstall = `#!/bin/bash
args=()
while [[ $# -gt 0 ]]; do case "$1" in -o|-g) shift 2 ;; *) args+=("$1"); shift ;; esac; done
/usr/bin/install "${args[@]}" || exit
if [[ -f "$FAKE/state/install_troca" && "${args[-1]}" == "$(<"$FAKE/state/install_troca")" ]]; then
  mkdir -p "$FAKE/desvio"
  rm -rf "${args[-1]}"
  ln -s "$FAKE/desvio" "${args[-1]}"
fi
`

const stubChown = "#!/bin/bash\nexit 0\n"

const stubGetent = `#!/bin/bash
[[ "$1" == passwd ]] && exit 2
exec /usr/bin/getent "$@"
`

const stubSs = `#!/bin/bash
[[ -f "$FAKE/state/porta_ocupada" ]] && echo "LISTEN 0 4096 0.0.0.0:3306 0.0.0.0:*"
exit 0
`

// timeout só envolve a leitura da porta publicada (DEPLOY-08: bash -c com
// /dev/tcp). Responde com os bytes de $S/porta_resposta (padrão do ensaio: o
// erro 1130 que o servidor recebe de verdade); sem o arquivo, simula o
// timeout (sai 124 sem bytes).
const stubTimeout = `#!/bin/bash
echo "timeout $*" >>"$FAKE/log"
[[ -f "$FAKE/state/porta_resposta" ]] || exit 124
cat "$FAKE/state/porta_resposta"
`

// Roda o script real, sem cópia/sed: user namespace (EUID 0) + mount
// namespace com binds dos caminhos fixos do script para o $FAKE.
// Lote 36 (TOCTOU): o script exige que os ancestrais de --segredos-dir e
// --ca-dir sejam do root e sem escrita de grupo/outros. No user namespace o
// "/" real aparece com o overflow uid (65534) e o /tmp é 1777, então o runner
// faz um chroot numa raiz tmpfs (dona: o root do namespace, 755) com rbind
// dos diretórios de topo; o topo do $FAKE (ex.: /tmp) é recriado vazio (755)
// e só o $FAKE e o deploy/ do script são expostos nele, nos mesmos caminhos.
const runnerEnsaio = `#!/bin/bash
set -euo pipefail
umask 022
NR="$FAKE/.raiz"
mkdir -p "$NR"
mount -t tmpfs -o mode=755 tmpfs "$NR"
topo=${FAKE#/}; topo=/${topo%%/*}
for x in /*; do
  [[ "$x" == "$topo" ]] && continue
  if [[ -L "$x" ]]; then ln -s "$(readlink "$x")" "$NR$x"
  elif [[ -d "$x" ]]; then mkdir "$NR$x"; mount --rbind "$x" "$NR$x"
  fi
done
mkdir -p "$NR$topo"
expor() { [[ "$1" == "$topo"/* ]] || return 0; mkdir -p "$NR$1"; mount --bind "$1" "$NR$1"; }
expor "$FAKE"
expor "$(dirname "$SCRIPT")"
mount --bind "$FAKE/root" "$NR/root"
mount --bind "$FAKE/opt" "$NR/opt"
mount --bind "$FAKE/sbin" "$NR/usr/local/sbin"
mount --bind "$FAKE/systemd" "$NR/etc/systemd/system"
cd /
export PATH="$FAKE/bin:$PATH"
umask 077
exec chroot "$NR" /bin/bash -c 'cd / && exec bash "$0" "$@"' "$SCRIPT" "$@"
`
