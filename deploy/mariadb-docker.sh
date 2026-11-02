#!/usr/bin/env bash
# =============================================================================
# MariaDB do rotaperfumes em Docker, com TLS proprio (substitui o MySQL).
#
# Rodar como root NO SERVIDOR (192.168.168.106):
#   sudo bash deploy/mariadb-docker.sh                 # instala/atualiza (porta 3306)
#   sudo bash deploy/mariadb-docker.sh --porta 3308    # lado a lado com outro banco
#   sudo bash deploy/mariadb-docker.sh --recriar       # recria o container (dados ficam)
#   sudo bash deploy/mariadb-docker.sh --novo-cert     # reemite o certificado do servidor
#   sudo bash deploy/mariadb-docker.sh --help
#
# Etapas:
#   1. confere root, docker, openssl e a porta
#   2. imagem rotaperfumes-mariadb:<versao> (FROM mariadb:<versao> + cnf de
#      TLS e hardening). A chave privada NAO entra na imagem: os certificados
#      ficam no host (SSL_DIR) e sao montados read-only.
#   3. certificados: CA propria + certificado do servidor com SAN (reaproveita
#      os existentes se forem validos, mesmo padrao do docs/Instal_SSL.md)
#   4. senhas (root e usuarios) geradas em SEGREDOS_DIR (so root le)
#   5. container (root so local: MARIADB_ROOT_HOST=localhost, via docker exec)
#   6. banco rotaperfumes e usuarios (todos REQUIRE SSL):
#        rotaperfumes_app   @ rede docker  -> API (so DML)
#        rotaperfumes_admin @ rede docker  -> dump do Jenkins / migracoes
#        casaos             @ rede da LAN  -> alvos make db-* das maquinas de dev
#   7. testa a conexao TLS com verificacao do certificado (e a recusa sem TLS)
#
# Re-executavel: cada etapa confere o estado antes de mudar algo.
# Regras: nunca "set -x"; segredos nunca vao para argv, log ou tela.
# =============================================================================
set -euo pipefail
umask 077

VERSAO="11.4"
PORTA="3306"
SITE_IP="192.168.168.106"
SITE_HOST="$(hostname)"
CONTAINER="rotaperfumes-mariadb"
REDE_LAN="192.168.168.0/255.255.255.0"
SSL_DIR="/etc/mysql/ssl"
CA_DIR="/root/rotaperfumes-ca"
SEGREDOS_DIR="/root/rotaperfumes-mariadb"
DADOS_DIR="/var/lib/rotaperfumes/mariadb"
BUILD_DIR="/opt/rotaperfumes/mariadb-imagem"
RECRIAR=false
NOVO_CERT=false

readonly DB_NOME="rotaperfumes"
readonly DB_APP="rotaperfumes_app"
readonly DB_ADMIN="rotaperfumes_admin"
readonly DB_MAKE="casaos"
readonly DB_HOST_DOCKER="172.16.0.0/255.240.0.0"
readonly SQL_MODE="STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION"
readonly COLLATION="utf8mb4_unicode_ci"
readonly MIN_CHARSET_COLLATIONS="11.2"
readonly CERT_DIAS=730
readonly CA_DIAS=3650
# Dentro do container
readonly C_SSL="/etc/mysql/ssl"
readonly C_SEGREDOS="/run/rotaperfumes"

usage() {
  cat <<EOF
Uso: sudo bash $0 [opcoes]

  --versao V        tag da imagem oficial mariadb (padrao $VERSAO)
  --porta P         porta publicada no host (padrao $PORTA)
  --ip IP           IP do servidor no SAN e no teste (padrao $SITE_IP)
  --host-nome N     nome DNS extra no SAN (padrao: hostname = $SITE_HOST)
  --rede-lan R      origem do usuario $DB_MAKE, formato IP/MASCARA
                    (padrao $REDE_LAN)
  --container N     nome do container (padrao $CONTAINER)
  --dados DIR       datadir no host (padrao $DADOS_DIR)
  --ssl-dir DIR     certificados no host (padrao $SSL_DIR = DB_TLS_CA_HOST_PATH do Jenkins)
  --recriar         remove e recria o container (os dados ficam em --dados)
  --novo-cert       reemite o certificado do servidor (e a CA, se a chave dela sumiu)
  -h, --help        esta ajuda
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --versao) VERSAO="${2:?}"; shift 2 ;;
    --porta) PORTA="${2:?}"; shift 2 ;;
    --ip) SITE_IP="${2:?}"; shift 2 ;;
    --host-nome) SITE_HOST="${2:?}"; shift 2 ;;
    --rede-lan) REDE_LAN="${2:?}"; shift 2 ;;
    --container) CONTAINER="${2:?}"; shift 2 ;;
    --dados) DADOS_DIR="${2:?}"; shift 2 ;;
    --ssl-dir) SSL_DIR="${2:?}"; shift 2 ;;
    --recriar) RECRIAR=true; shift ;;
    --novo-cert) NOVO_CERT=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Opcao desconhecida: $1" >&2; usage >&2; exit 2 ;;
  esac
done

ETAPA=0
readonly TOTAL_ETAPAS=7
etapa() { ETAPA=$((ETAPA + 1)); printf '\n=== [%d/%d] %s ===\n' "$ETAPA" "$TOTAL_ETAPAS" "$1"; }
info()  { printf '  %s\n' "$*"; }
aviso() { printf '  AVISO: %s\n' "$*" >&2; }
die()   { printf 'ERRO: %s\n' "$*" >&2; exit 1; }

# versao_ge A B -> sucesso se A >= B (ordem de versao).
versao_ge() { [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]; }

# Senha so com [A-Za-z0-9] (vale no .env, no make e no SQL sem escapar).
gerar_senha() {
  local s
  s=$(openssl rand -base64 64 | tr -dc 'A-Za-z0-9')
  printf '%s' "${s:0:32}"
}

# ---------------------------------------------------------------------------
etapa "Pre-requisitos"
[[ $EUID -eq 0 ]] || die "rode como root: sudo bash $0"
for cmd in docker openssl ss sha256sum; do
  command -v "$cmd" >/dev/null || die "comando '$cmd' nao encontrado"
done
docker info >/dev/null 2>&1 || die "o daemon do Docker nao responde"
[[ "$VERSAO" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]] || die "--versao invalida: $VERSAO"
[[ "$PORTA" =~ ^[0-9]+$ ]] && (( PORTA >= 1 && PORTA <= 65535 )) || die "--porta invalida: $PORTA"
[[ "$SITE_IP" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] || die "--ip invalido: $SITE_IP"
[[ "$SITE_HOST" =~ ^[A-Za-z0-9.-]+$ ]] || die "--host-nome invalido: $SITE_HOST"
[[ "$REDE_LAN" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}/[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] \
  || die "--rede-lan invalida: $REDE_LAN (use IP/MASCARA, ex.: 192.168.168.0/255.255.255.0)"
[[ "$CONTAINER" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || die "--container invalido: $CONTAINER"

IMAGEM="rotaperfumes-mariadb:$VERSAO"
CONTAINER_ID=$(docker ps -aq --filter "name=^${CONTAINER}\$")
CONTAINER_RODANDO=false
if [[ -n "$CONTAINER_ID" ]] && [[ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER")" == "true" ]]; then
  CONTAINER_RODANDO=true
fi

# A porta pode estar ocupada pelo proprio container; por qualquer outro, aborta.
if [[ -n "$(ss -Hltn "sport = :$PORTA")" ]]; then
  if ! $CONTAINER_RODANDO || ! docker port "$CONTAINER" 3306/tcp 2>/dev/null | grep -q ":$PORTA\$"; then
    info "Containers publicando a porta $PORTA:"
    docker ps --filter "publish=$PORTA" --format '    {{.Names}}  {{.Image}}  {{.Ports}}' >&2 || true
    die "porta $PORTA ocupada por outro servico. Pare o banco antigo (ex.: docker stop <container_mysql>) ou use --porta"
  fi
fi
info "Docker ok; porta $PORTA livre (ou ja do $CONTAINER)."

# ---------------------------------------------------------------------------
etapa "Imagem $IMAGEM"
install -d -m 700 "$BUILD_DIR"
CHARSET_COLLATIONS=""
if versao_ge "$VERSAO" "$MIN_CHARSET_COLLATIONS"; then
  CHARSET_COLLATIONS="character_set_collations = utf8mb4=$COLLATION"
fi
cat > "$BUILD_DIR/zz-rotaperfumes.cnf" <<EOF
# Gerado por deploy/mariadb-docker.sh. Nao edite: o script regrava.
[mariadb]
# SEC-29: TLS obrigatorio com o certificado proprio (montado de $SSL_DIR).
ssl_ca   = $C_SSL/ca.pem
ssl_cert = $C_SSL/server-cert.pem
ssl_key  = $C_SSL/server-key.pem
require_secure_transport = ON
tls_version = TLSv1.2,TLSv1.3
# DEPLOY-02: hardening.
local_infile = 0
sql_mode = $SQL_MODE
character_set_server = utf8mb4
collation_server = $COLLATION
$CHARSET_COLLATIONS
# As contas usam IP/mascara: sem DNS reverso.
skip_name_resolve = ON
EOF
cat > "$BUILD_DIR/Dockerfile" <<'EOF'
ARG VERSAO
FROM mariadb:${VERSAO}
COPY zz-rotaperfumes.cnf /etc/mysql/conf.d/zz-rotaperfumes.cnf
RUN chmod 644 /etc/mysql/conf.d/zz-rotaperfumes.cnf
LABEL org.opencontainers.image.title="rotaperfumes-mariadb" \
      org.opencontainers.image.description="MariaDB do rotaperfumes com TLS e hardening (deploy/mariadb-docker.sh)"
EOF
docker build --pull -q --build-arg "VERSAO=$VERSAO" -t "$IMAGEM" "$BUILD_DIR" >/dev/null
info "Imagem $IMAGEM pronta ($(docker run --rm --entrypoint mariadbd "$IMAGEM" --version | awk '{print $3}'))."
MYSQL_UID=$(docker run --rm --entrypoint id "$IMAGEM" -u mysql)
MYSQL_GID=$(docker run --rm --entrypoint id "$IMAGEM" -g mysql)
info "Usuario mysql da imagem: uid $MYSQL_UID, gid $MYSQL_GID."

# ---------------------------------------------------------------------------
etapa "Certificados em $SSL_DIR"
install -d -m 755 "$SSL_DIR"
install -d -m 700 "$CA_DIR"
DOCKER_GW=$(ip -4 -o addr show docker0 2>/dev/null | awk '{split($4, a, "/"); print a[1]; exit}')
DOCKER_GW="${DOCKER_GW:-172.17.0.1}"
SAN="IP:$SITE_IP, IP:127.0.0.1, IP:$DOCKER_GW, DNS:localhost, DNS:host.docker.internal, DNS:$SITE_HOST"

mesma_chave() { # cert key
  [[ "$(openssl x509 -in "$1" -noout -pubkey 2>/dev/null)" == "$(openssl pkey -in "$2" -pubout 2>/dev/null)" ]]
}
cert_servidor_valido() {
  [[ -s "$SSL_DIR/ca.pem" && -s "$SSL_DIR/server-cert.pem" && -s "$SSL_DIR/server-key.pem" ]] || return 1
  openssl verify -CAfile "$SSL_DIR/ca.pem" "$SSL_DIR/server-cert.pem" >/dev/null 2>&1 || return 1
  openssl x509 -in "$SSL_DIR/server-cert.pem" -noout -checkend $((30 * 86400)) >/dev/null || return 1
  mesma_chave "$SSL_DIR/server-cert.pem" "$SSL_DIR/server-key.pem" || return 1
  openssl x509 -in "$SSL_DIR/server-cert.pem" -noout -ext subjectAltName 2>/dev/null \
    | grep -qE "IP Address:${SITE_IP//./\\.}(,|\$| )"
}
ca_utilizavel() {
  [[ -s "$SSL_DIR/ca.pem" && -s "$CA_DIR/ca-key.pem" ]] || return 1
  openssl x509 -in "$SSL_DIR/ca.pem" -noout -checkend $((CERT_DIAS * 86400)) >/dev/null || return 1
  mesma_chave "$SSL_DIR/ca.pem" "$CA_DIR/ca-key.pem"
}

CERT_MUDOU=false
if ! $NOVO_CERT && cert_servidor_valido; then
  info "Certificado existente valido (CA confere, SAN tem $SITE_IP, vale mais de 30 dias): reaproveitado."
else
  TMP=$(mktemp -d /root/rotaperfumes-ssl.XXXXXX)
  trap 'rm -rf "$TMP"' EXIT
  if ca_utilizavel; then
    info "Reaproveitando a CA ($SSL_DIR/ca.pem + $CA_DIR/ca-key.pem)."
  else
    if [[ -s "$SSL_DIR/ca.pem" ]]; then
      BACKUP="$SSL_DIR.bak-$(date +%Y%m%d-%H%M%S)"
      cp -a "$SSL_DIR" "$BACKUP"
      aviso "CA anterior sem a chave (ou vencendo): gerando CA NOVA. Backup em $BACKUP."
      aviso "Os clientes precisam da CA nova (deploy/ca-cert.pem das maquinas de dev)."
    fi
    openssl genrsa -out "$TMP/ca-key.pem" 3072 2>/dev/null
    openssl req -x509 -new -key "$TMP/ca-key.pem" -sha256 -days "$CA_DIAS" \
      -subj "/CN=RotaPerfumes_MariaDB_CA" \
      -addext "basicConstraints=critical,CA:TRUE" \
      -addext "keyUsage=critical,keyCertSign,cRLSign" \
      -out "$TMP/ca.pem"
    install -m 600 -o root -g root "$TMP/ca-key.pem" "$CA_DIR/ca-key.pem"
    install -m 644 -o root -g root "$TMP/ca.pem" "$SSL_DIR/ca.pem"
    info "CA nova: $SSL_DIR/ca.pem (chave em $CA_DIR/ca-key.pem)."
  fi
  openssl genrsa -out "$TMP/server-key.pem" 3072 2>/dev/null
  openssl req -new -key "$TMP/server-key.pem" -subj "/CN=RotaPerfumes MariaDB Server" -out "$TMP/server.csr"
  cat > "$TMP/server-ext.cnf" <<EOF
basicConstraints = critical, CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = $SAN
EOF
  openssl x509 -req -in "$TMP/server.csr" -CA "$SSL_DIR/ca.pem" -CAkey "$CA_DIR/ca-key.pem" \
    -CAserial "$CA_DIR/ca.srl" -CAcreateserial -sha256 -days "$CERT_DIAS" \
    -extfile "$TMP/server-ext.cnf" -out "$TMP/server-cert.pem" 2>/dev/null
  openssl verify -CAfile "$SSL_DIR/ca.pem" "$TMP/server-cert.pem" >/dev/null || die "certificado gerado nao valida contra a CA"
  install -m 644 -o root -g root "$TMP/server-cert.pem" "$SSL_DIR/server-cert.pem"
  install -m 644 -o root -g root "$TMP/server-ext.cnf" "$CA_DIR/server-ext.cnf"
  install -m 600 -o "$MYSQL_UID" -g "$MYSQL_GID" "$TMP/server-key.pem" "$SSL_DIR/server-key.pem"
  CERT_MUDOU=true
  info "Certificado do servidor emitido ($CERT_DIAS dias). SAN: $SAN"
fi
# O mariadbd (uid do mysql) le a chave; ca.pem e o certificado sao publicos.
chown "$MYSQL_UID:$MYSQL_GID" "$SSL_DIR/server-key.pem"
chmod 600 "$SSL_DIR/server-key.pem"
chmod 644 "$SSL_DIR/ca.pem" "$SSL_DIR/server-cert.pem"
CERT_SHA=$(sha256sum "$SSL_DIR/server-cert.pem" | awk '{print $1}')

# ---------------------------------------------------------------------------
etapa "Senhas em $SEGREDOS_DIR"
install -d -m 700 -o root -g root "$SEGREDOS_DIR"
# Subpasta montada no container: o entrypoint relê a senha do root ja como mysql.
install -d -m 700 -o "$MYSQL_UID" -g "$MYSQL_GID" "$SEGREDOS_DIR/container"
install -d -m 700 -o "$MYSQL_UID" -g "$MYSQL_GID" "$DADOS_DIR"

DADOS_INICIALIZADOS=false
[[ -d "$DADOS_DIR/mysql" ]] && DADOS_INICIALIZADOS=true

senha_arquivo() { # arquivo -> cria se faltar
  if [[ ! -s "$1" ]]; then
    gerar_senha > "$1"
    info "Senha nova: $1"
  fi
  chmod 600 "$1"
}
if [[ ! -s "$SEGREDOS_DIR/root.senha" ]] && $DADOS_INICIALIZADOS; then
  die "$DADOS_DIR ja tem dados, mas $SEGREDOS_DIR/root.senha nao existe (senha do root desconhecida). Restaure o arquivo ou use outro --dados."
fi
for u in root "$DB_APP" "$DB_ADMIN" "$DB_MAKE"; do
  senha_arquivo "$SEGREDOS_DIR/$u.senha"
done
ROOT_SENHA=$(<"$SEGREDOS_DIR/root.senha")
install -m 400 -o "$MYSQL_UID" -g "$MYSQL_GID" "$SEGREDOS_DIR/root.senha" "$SEGREDOS_DIR/container/root.senha"
printf '[client]\nuser=root\npassword=%s\nprotocol=socket\n' "$ROOT_SENHA" > "$SEGREDOS_DIR/container/root.cnf"
chown "$MYSQL_UID:$MYSQL_GID" "$SEGREDOS_DIR/container/root.cnf"
chmod 400 "$SEGREDOS_DIR/container/root.cnf"
unset ROOT_SENHA
info "Senhas prontas (so root le). Nada foi impresso."

# ---------------------------------------------------------------------------
etapa "Container $CONTAINER"
if [[ -n "$CONTAINER_ID" ]] && $RECRIAR; then
  info "Removendo o container antigo (--recriar; dados ficam em $DADOS_DIR)."
  docker rm -f "$CONTAINER" >/dev/null
  CONTAINER_ID=""
  CONTAINER_RODANDO=false
fi
if [[ -z "$CONTAINER_ID" ]]; then
  docker run -d --name "$CONTAINER" \
    --restart unless-stopped \
    --security-opt no-new-privileges:true \
    -p "$PORTA:3306" \
    -e MARIADB_ROOT_PASSWORD_FILE="$C_SEGREDOS/root.senha" \
    -e MARIADB_ROOT_HOST=localhost \
    -e MARIADB_AUTO_UPGRADE=1 \
    -v "$DADOS_DIR:/var/lib/mysql" \
    -v "$SSL_DIR:$C_SSL:ro" \
    -v "$SEGREDOS_DIR/container:$C_SEGREDOS:ro" \
    --label "rotaperfumes.cert-sha256=$CERT_SHA" \
    --log-opt max-size=10m --log-opt max-file=5 \
    "$IMAGEM" >/dev/null
  info "Container criado (porta $PORTA, imagem $IMAGEM)."
elif ! $CONTAINER_RODANDO; then
  docker start "$CONTAINER" >/dev/null
  info "Container existente iniciado."
else
  info "Container ja em execucao."
  if [[ "$(docker inspect -f '{{.Image}}' "$CONTAINER")" != "$(docker image inspect -f '{{.Id}}' "$IMAGEM")" ]]; then
    aviso "o container usa uma imagem anterior a $IMAGEM: rode com --recriar para aplicar."
  fi
fi

root_sql() { docker exec -i "$CONTAINER" mariadb --defaults-extra-file="$C_SEGREDOS/root.cnf" -N -B "$@"; }
info "Aguardando o servidor definitivo (a inicializacao roda um servidor temporario sem rede)..."
for i in $(seq 1 90); do
  if [[ "$(root_sql -e 'SELECT @@skip_networking' 2>/dev/null)" == "0" ]]; then break; fi
  [[ $i -eq 90 ]] && { docker logs --tail 40 "$CONTAINER" >&2; die "o MariaDB nao ficou pronto em 180 s"; }
  sleep 2
done
if $CERT_MUDOU && [[ -n "$CONTAINER_ID" ]]; then
  root_sql -e 'FLUSH SSL' && info "Certificado novo recarregado (FLUSH SSL)."
fi
info "MariaDB $(root_sql -e 'SELECT VERSION()') pronto; TLS: $(root_sql -e "SHOW GLOBAL VARIABLES LIKE 'have_ssl'" | awk '{print $2}')."

# ---------------------------------------------------------------------------
etapa "Banco $DB_NOME e usuarios"
SENHA_APP=$(<"$SEGREDOS_DIR/$DB_APP.senha")
SENHA_ADMIN=$(<"$SEGREDOS_DIR/$DB_ADMIN.senha")
SENHA_MAKE=$(<"$SEGREDOS_DIR/$DB_MAKE.senha")
# SQL pelo stdin (nunca no argv). As senhas sao [A-Za-z0-9]: sem escape.
root_sql <<EOF
CREATE DATABASE IF NOT EXISTS \`$DB_NOME\` CHARACTER SET utf8mb4 COLLATE $COLLATION;

-- API: somente DML (mesmo escopo do deploy/mariadb-setup.sql).
CREATE USER IF NOT EXISTS '$DB_APP'@'$DB_HOST_DOCKER' IDENTIFIED BY '$SENHA_APP' REQUIRE SSL;
ALTER USER '$DB_APP'@'$DB_HOST_DOCKER' IDENTIFIED BY '$SENHA_APP' REQUIRE SSL;
GRANT SELECT, INSERT, UPDATE, DELETE ON \`$DB_NOME\`.* TO '$DB_APP'@'$DB_HOST_DOCKER';

-- Dump do Jenkins (credencial rotaperfumes-db-admin) e migracoes: DDL no banco.
CREATE USER IF NOT EXISTS '$DB_ADMIN'@'$DB_HOST_DOCKER' IDENTIFIED BY '$SENHA_ADMIN' REQUIRE SSL;
ALTER USER '$DB_ADMIN'@'$DB_HOST_DOCKER' IDENTIFIED BY '$SENHA_ADMIN' REQUIRE SSL;
GRANT ALL PRIVILEGES ON \`$DB_NOME\`.* TO '$DB_ADMIN'@'$DB_HOST_DOCKER';

-- Alvos make db-* das maquinas da LAN (db-down/db-create precisam de DROP/CREATE).
CREATE USER IF NOT EXISTS '$DB_MAKE'@'$REDE_LAN' IDENTIFIED BY '$SENHA_MAKE' REQUIRE SSL;
ALTER USER '$DB_MAKE'@'$REDE_LAN' IDENTIFIED BY '$SENHA_MAKE' REQUIRE SSL;
GRANT ALL PRIVILEGES ON \`$DB_NOME\`.* TO '$DB_MAKE'@'$REDE_LAN';

-- Higiene: root so local, sem contas anonimas e sem o banco test.
DELETE FROM mysql.global_priv WHERE User = '';
DROP DATABASE IF EXISTS test;
FLUSH PRIVILEGES;
EOF
unset SENHA_APP SENHA_MAKE
info "Contas:"
root_sql -e "SELECT CONCAT('    ', User, '@', Host) FROM mysql.global_priv WHERE User NOT IN ('mariadb.sys', 'healthcheck') ORDER BY User, Host"
REMOTOS=$(root_sql -e "SELECT COUNT(*) FROM mysql.global_priv WHERE User = 'root' AND Host NOT IN ('localhost', '127.0.0.1', '::1')")
[[ "$REMOTOS" == "0" ]] || aviso "existe root com host remoto; confira (o esperado e so root@localhost)."

# ---------------------------------------------------------------------------
etapa "Teste de conexao TLS"
# Cliente descartavel saindo pela rede docker: chega como 172.x (conta do admin).
cliente() {
  MYSQL_PWD="$SENHA_ADMIN" docker run --rm -i -e MYSQL_PWD \
    --mount "type=bind,source=$SSL_DIR/ca.pem,target=/ca.pem,readonly" \
    --entrypoint mariadb "$IMAGEM" -h "$SITE_IP" -P "$PORTA" -u "$DB_ADMIN" -N -B "$@"
}
if cliente --ssl-ca=/ca.pem --ssl-verify-server-cert \
     -e "SELECT CONCAT('  conectado como ', CURRENT_USER(), ', ', VERSION(), ', ', VARIABLE_VALUE) FROM information_schema.SESSION_STATUS WHERE VARIABLE_NAME = 'Ssl_version'"; then
  info "TLS com verificacao do certificado: OK."
else
  die "falhou a conexao TLS verificada em $SITE_IP:$PORTA (confira o SAN e o firewall)"
fi
if cliente --skip-ssl -e 'SELECT 1' >/dev/null 2>&1; then
  aviso "conexao SEM TLS foi aceita (require_secure_transport nao esta valendo)."
else
  info "Conexao sem TLS recusada: OK."
fi
unset SENHA_ADMIN

cat <<EOF

Pronto. MariaDB em $SITE_IP:$PORTA (container $CONTAINER, imagem $IMAGEM).

Senhas (so root le; nao foram impressas):
  $SEGREDOS_DIR/root.senha              root@localhost (docker exec $CONTAINER mariadb -uroot -p)
  $SEGREDOS_DIR/$DB_APP.senha   API: DB_USUARIO/DB_SENHA do Secret file rotaperfumes-api-env
  $SEGREDOS_DIR/$DB_ADMIN.senha credencial Jenkins rotaperfumes-db-admin (dump)
  $SEGREDOS_DIR/$DB_MAKE.senha             .env das maquinas de dev (make db-*)

Proximos passos:
  1. CA publica para os clientes: copie $SSL_DIR/ca.pem para deploy/ca-cert.pem
     das maquinas de dev (o Jenkins ja le $SSL_DIR/ca.pem).
  2. .env das maquinas de dev:
       DB_HOST=$SITE_IP  DB_PORT=$PORTA  DB_USUARIO=$DB_MAKE  DB_SENHA=<$DB_MAKE.senha>
       DB_TLS_CA=deploy/ca-cert.pem  DB_CLIENTE=mariadb
  3. Guarde $CA_DIR/ca-key.pem fora do servidor (so e usada para reemitir o
     certificado: --novo-cert).
  4. Porta publicada pelo Docker NAO passa pelo ufw: para restringir a LAN, use
     a chain DOCKER-USER do iptables.
EOF
