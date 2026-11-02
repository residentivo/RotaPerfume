#!/usr/bin/env bash
# =============================================================================
# MariaDB do rotaperfumes em Docker, com TLS proprio. E o UNICO caminho
# oficial do banco em producao (DEPLOY-06): o setup-servidor.sh nao instala
# nem configura banco.
#
# Rodar como root NO SERVIDOR (192.168.168.106):
#   sudo bash deploy/mariadb-docker.sh --ca-key /media/pendrive/ca-key.pem
#       instala/atualiza, sem publicar porta (so a rede docker rotaperfumes-db)
#   sudo bash deploy/mariadb-docker.sh --dev-ip 192.168.168.50 --dev-ip 192.168.168.51
#       publica SITE_IP:3306 so para essas maquinas de dev (conta casaos)
#   sudo bash deploy/mariadb-docker.sh --nova-ca      # primeira instalacao (CA nova)
#   sudo bash deploy/mariadb-docker.sh --recriar      # recria o container (dados ficam)
#   sudo bash deploy/mariadb-docker.sh --novo-cert --ca-key ...  # reemite o certificado
#   sudo bash deploy/mariadb-docker.sh --help
#
# Etapas:
#   1. confere root, comandos, argumentos e o container atual (divergencia de
#      rede/porta/usuario exige --recriar; nada e mudado antes disso)
#   2. imagem rotaperfumes-mariadb:11.4-<digest> (FROM IMAGEM_BASE fixada por
#      digest, SEC-31, + cnf de TLS, ed25519 e hardening). A chave privada NAO
#      entra na imagem: os certificados ficam no host (SSL_DIR), montados ro.
#   3. certificados: CA propria (chave FORA do servidor: --ca-key; CA nova so
#      com --nova-ca) + certificado do servidor com SAN fixo
#   4. senhas (root e usuarios) em SEGREDOS_DIR (so root le)
#   5. rede docker rotaperfumes-db (172.30.42.0/24, IPs fixos) e, com
#      --dev-ip, a chain ROTAPERFUMES-DB em DOCKER-USER + unit systemd
#   6. container (so na rede rotaperfumes-db; --user mysql, --cap-drop ALL,
#      --read-only; root so local: MARIADB_ROOT_HOST=localhost)
#   7. banco rotaperfumes e contas (todas ed25519 + REQUIRE SSL), convergidas:
#        rotaperfumes_app   @ 172.30.42.10 (API)            -> so DML
#        rotaperfumes_admin @ 172.30.42.20 (import/migracao) -> ALL em rotaperfumes
#        casaos             @ cada --dev-ip (make db-*)      -> ALL em rotaperfumes e rotaperfumes\_%
#      contas do projeto com outro host sao removidas (DROP USER)
#   8. testa a conexao TLS verificada pela rede interna (e a recusa sem TLS);
#      com --dev-ip, tambem o certificado servido em SITE_IP:PORTA
#
# Rede (SEC-45): a API entra na rede externa rotaperfumes-db com
#   ipv4_address 172.30.42.10 e usa DB_HOST=rotaperfumes-mariadb, DB_PORT=3306,
#   DB_TLS_SERVER_NAME=rotaperfumes-mariadb. O import do Jenkins usa
#   docker run --network rotaperfumes-db --ip 172.30.42.20 ... -h rotaperfumes-mariadb.
#
# Atualizar a imagem base (SEC-31):
#   1. docker buildx imagetools inspect mariadb:11.4 --format '{{.Manifest.Digest}}'
#      (digest do INDICE multi-arch) e a tag do patch (ex.: 11.4.13)
#   2. trocar IMAGEM_BASE aqui e DB_CLIENT_IMAGE no Jenkinsfile (mesma string)
#   3. make test
#   4. backup do banco (mariadb-dump)
#   5. sudo bash deploy/mariadb-docker.sh --recriar [--dev-ip ...]
#
# Re-executavel: cada etapa confere o estado antes de mudar algo; as senhas
# ficam nos arquivos (reexecucao nao troca senha).
# Regras: nunca "set -x"; segredos nunca vao para argv, log ou tela.
# =============================================================================
set -euo pipefail
umask 077

# SEC-31: imagem base fixada pelo digest do indice multi-arch. Mesma string do
# DB_CLIENT_IMAGE do Jenkinsfile (teste do TST-15 confere).
IMAGEM_BASE="mariadb:11.4.13@sha256:d1891ede5a559f1086f55053bb8cc1569b0298338c4d72d71725a7f74f6ba620"
PORTA="3306"
SITE_IP="192.168.168.106"
SITE_HOST="$(hostname)"
CONTAINER="rotaperfumes-mariadb"
REDE_NOME="rotaperfumes-db"
REDE_DOCKER="172.30.42.0/24"
SSL_DIR="/etc/mysql/ssl"
CA_DIR="/root/rotaperfumes-ca"
CA_KEY=""
SEGREDOS_DIR="/root/rotaperfumes-mariadb"
DADOS_DIR="/var/lib/rotaperfumes/mariadb"
BUILD_DIR="/opt/rotaperfumes/mariadb-imagem"
RECRIAR=false
NOVO_CERT=false
NOVA_CA=false
READ_ONLY=true
CASAOS_NATIVE_MOTIVO=""
DEV_IPS=()

readonly CONTAINER_PADRAO="rotaperfumes-mariadb"
readonly DB_NOME="rotaperfumes"
readonly DB_APP="rotaperfumes_app"
readonly DB_ADMIN="rotaperfumes_admin"
readonly DB_MAKE="casaos"
# DB-04/DB-06: ONLY_FULL_GROUP_BY fica DESLIGADO (decisao de 2026-11-02,
# revisao estatica). O MariaDB nao reconhece dependencia funcional nem casa
# "SELECT expr ... GROUP BY expr" (a mesma expressao) com a lista do SELECT:
# a serie de vendas do dashboard (backend/shared/analytics/analyticsmysql/
# sql.go, sqlSerieVendas: SELECT DATE_FORMAT(data_pedido, ...) ... GROUP BY
# DATE_FORMAT(data_pedido, ...)) seria recusada com ER_WRONG_FIELD_WITH_GROUP.
# Ligar depois de o backend agrupar pelo alias (GROUP BY data) e de um teste
# de integracao em banco DESCARTAVEL; o mesmo sql_mode vale para dev (make).
readonly SQL_MODE="STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION"
readonly COLLATION="utf8mb4_unicode_ci"
readonly MIN_CHARSET_COLLATIONS="11.2"
readonly CERT_DIAS=730
readonly CA_DIAS=3650
readonly CA_CN="RotaPerfumes_MariaDB_CA"
# SEC-45: firewall da porta publicada (so com --dev-ip).
readonly FW_CHAIN="ROTAPERFUMES-DB"
readonly FW_SCRIPT="/usr/local/sbin/rotaperfumes-db-firewall"
readonly FW_UNIT="rotaperfumes-db-firewall.service"
readonly FW_UNIT_ARQ="/etc/systemd/system/$FW_UNIT"
# Dentro do container
readonly C_SSL="/etc/mysql/ssl"
readonly C_SEGREDOS="/run/rotaperfumes"

usage() {
  cat <<EOF
Uso: sudo bash $0 [opcoes]

  --dev-ip IP         maquina de dev autorizada (repetivel): publica
                      $SITE_IP:$PORTA, libera so esses IPs na chain $FW_CHAIN
                      (DOCKER-USER) e cria casaos@IP. Sem --dev-ip: nenhuma
                      porta publicada e nenhuma conta casaos.
  --ca-key ARQ        chave da CA existente (mantida FORA do servidor, ex.:
                      pendrive). Lida de la, nunca copiada. Exigida quando o
                      certificado do servidor precisa ser (re)emitido.
  --nova-ca           gera uma CA NOVA (primeira instalacao ou troca de CA).
                      Implica --novo-cert. Atualize deploy/db-ca.fingerprint.
  --novo-cert         reemite o certificado do servidor (exige a chave da CA)
  --recriar           remove e recria o container (os dados ficam em --dados)
  --imagem-base REF   imagem base, mariadb:<versao>@sha256:<64 hex>
                      (padrao $IMAGEM_BASE)
  --porta P           porta publicada no host com --dev-ip (padrao $PORTA)
  --ip IP             IP do servidor: SAN, porta publicada e teste (padrao $SITE_IP)
  --host-nome N       nome DNS extra no SAN (padrao: hostname = $SITE_HOST)
  --rede-docker R     subnet /24 da rede $REDE_NOME (padrao $REDE_DOCKER;
                      .2 = MariaDB, .10 = API, .20 = admin/import)
  --rede-nome N       nome da rede docker (padrao $REDE_NOME; so para testes)
  --container N       nome do container e DNS no SAN (padrao $CONTAINER;
                      outro nome nao aceita --dev-ip)
  --dados DIR         datadir no host (padrao $DADOS_DIR)
  --ssl-dir DIR       certificados no host (padrao $SSL_DIR, lido pelo Jenkins)
  --sem-read-only     cria o container sem --read-only (so se nao subir com ele)
  --casaos-native-motivo TEXTO
                      SAIDA DE EMERGENCIA (SEC-30): casaos com
                      mysql_native_password (cliente sem ed25519). O motivo
                      fica registrado em $SEGREDOS_DIR/casaos-native.motivo.
  -h, --help          esta ajuda
EOF
}

die()   { printf 'ERRO: %s\n' "$*" >&2; exit 1; }

exigir_valor() { [[ $# -ge 2 && -n "$2" ]] || die "a opcao $1 exige um valor (veja --help)"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dev-ip) exigir_valor "$@"; DEV_IPS+=("$2"); shift 2 ;;
    --ca-key) exigir_valor "$@"; CA_KEY="$2"; shift 2 ;;
    --nova-ca) NOVA_CA=true; NOVO_CERT=true; shift ;;
    --novo-cert) NOVO_CERT=true; shift ;;
    --recriar) RECRIAR=true; shift ;;
    --imagem-base) exigir_valor "$@"; IMAGEM_BASE="$2"; shift 2 ;;
    --porta) exigir_valor "$@"; PORTA="$2"; shift 2 ;;
    --ip) exigir_valor "$@"; SITE_IP="$2"; shift 2 ;;
    --host-nome) exigir_valor "$@"; SITE_HOST="$2"; shift 2 ;;
    --rede-docker) exigir_valor "$@"; REDE_DOCKER="$2"; shift 2 ;;
    --rede-nome) exigir_valor "$@"; REDE_NOME="$2"; shift 2 ;;
    --container) exigir_valor "$@"; CONTAINER="$2"; shift 2 ;;
    --dados) exigir_valor "$@"; DADOS_DIR="$2"; shift 2 ;;
    --ssl-dir) exigir_valor "$@"; SSL_DIR="$2"; shift 2 ;;
    --sem-read-only) READ_ONLY=false; shift ;;
    --casaos-native-motivo) exigir_valor "$@"; CASAOS_NATIVE_MOTIVO="$2"; shift 2 ;;
    --rede-lan) die "--rede-lan saiu (SEC-45): use --dev-ip IP, uma vez por maquina de dev" ;;
    --versao) die "--versao saiu (SEC-31): use --imagem-base mariadb:<versao>@sha256:<digest>" ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Opcao desconhecida: $1" >&2; usage >&2; exit 2 ;;
  esac
done

ETAPA=0
readonly TOTAL_ETAPAS=8
etapa() { ETAPA=$((ETAPA + 1)); printf '\n=== [%d/%d] %s ===\n' "$ETAPA" "$TOTAL_ETAPAS" "$1"; }
info()  { printf '  %s\n' "$*"; }
aviso() { printf '  AVISO: %s\n' "$*" >&2; }

# versao_ge A B -> sucesso se A >= B (ordem de versao).
versao_ge() { [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]; }

# IPv4 literal com octetos 0-255.
ipv4_valido() {
  local ip=$1 o
  [[ "$ip" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] || return 1
  IFS=. read -r -a _oct <<<"$ip"
  for o in "${_oct[@]}"; do (( 10#$o <= 255 )) || return 1; done
}

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

[[ "$IMAGEM_BASE" =~ ^mariadb:[0-9.]+@sha256:[0-9a-f]{64}$ ]] \
  || die "--imagem-base invalida: $IMAGEM_BASE (formato mariadb:<versao>@sha256:<64 hex minusculo>)"
readonly IMAGEM_BASE
VERSAO_BASE=${IMAGEM_BASE#mariadb:}; VERSAO_BASE=${VERSAO_BASE%%@*}
DIGEST_BASE=${IMAGEM_BASE##*@sha256:}
[[ "$VERSAO_BASE" =~ ^([0-9]+\.[0-9]+) ]] || die "versao da --imagem-base sem major.minor: $VERSAO_BASE"
VERSAO_MM=${BASH_REMATCH[1]}
[[ "$PORTA" =~ ^[0-9]+$ ]] && (( PORTA >= 1 && PORTA <= 65535 )) || die "--porta invalida: $PORTA"
ipv4_valido "$SITE_IP" || die "--ip invalido: $SITE_IP"
[[ "$SITE_IP" != 0.0.0.0 && "$SITE_IP" != 127.* ]] || die "--ip precisa ser o IP da LAN (nao 0.0.0.0/loopback): $SITE_IP"
[[ "$SITE_HOST" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ ]] || die "--host-nome invalido: $SITE_HOST"
[[ "$CONTAINER" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ ]] || die "--container invalido: $CONTAINER (minusculas, numeros e '-': vira DNS no SAN)"
[[ "$REDE_NOME" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || die "--rede-nome invalido: $REDE_NOME"
# (ipv4_valido usa =~: guardar o prefixo antes de chama-la)
REDE_PREFIXO=""
[[ "$REDE_DOCKER" =~ ^([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})\.0/24$ ]] && REDE_PREFIXO=${BASH_REMATCH[1]}
[[ -n "$REDE_PREFIXO" ]] && ipv4_valido "$REDE_PREFIXO.0" || die "--rede-docker invalida: $REDE_DOCKER (use A.B.C.0/24)"
for d in "$SSL_DIR" "$DADOS_DIR"; do
  [[ "$d" =~ ^/[A-Za-z0-9._/-]+$ && "$d" != / && "$d" != *..* ]] || die "diretorio invalido: '$d' (caminho absoluto, sem espacos/virgulas/..)"
done
readonly IP_DB="$REDE_PREFIXO.2" IP_API="$REDE_PREFIXO.10" IP_ADMIN="$REDE_PREFIXO.20"

# --dev-ip: IPv4 de maquina (sem faixa), sem repeticao.
declare -A _vistos=()
DEV_IPS_OK=()
for ip in "${DEV_IPS[@]+"${DEV_IPS[@]}"}"; do
  ipv4_valido "$ip" || die "--dev-ip invalido: $ip (um IPv4 por maquina, sem mascara)"
  [[ "$ip" != 0.0.0.0 && "$ip" != 127.* && "$ip" != 255.255.255.255 ]] || die "--dev-ip nao pode ser $ip"
  [[ -n "${_vistos[$ip]:-}" ]] && continue
  _vistos[$ip]=1
  DEV_IPS_OK+=("$ip")
done
DEV_IPS=("${DEV_IPS_OK[@]+"${DEV_IPS_OK[@]}"}")
PUBLICAR=false
(( ${#DEV_IPS[@]} > 0 )) && PUBLICAR=true
if $PUBLICAR && [[ "$CONTAINER" != "$CONTAINER_PADRAO" ]]; then
  die "--dev-ip so com o container padrao ($CONTAINER_PADRAO): a chain $FW_CHAIN e a unit $FW_UNIT sao unicas no host"
fi
if [[ -n "$CA_KEY" ]]; then
  [[ -f "$CA_KEY" && -r "$CA_KEY" ]] || die "--ca-key nao encontrada/legivel: $CA_KEY"
fi
if [[ -n "$CASAOS_NATIVE_MOTIVO" ]]; then
  [[ "$CASAOS_NATIVE_MOTIVO" =~ ^[[:print:]]{10,300}$ ]] || die "--casaos-native-motivo precisa de 10 a 300 caracteres imprimiveis"
fi

# Firewall (SEC-45, fail closed): com --dev-ip, sem iptables/DOCKER-USER aborta.
if $PUBLICAR; then
  for cmd in iptables iptables-restore systemctl; do
    command -v "$cmd" >/dev/null || die "--dev-ip exige '$cmd' (chain $FW_CHAIN em DOCKER-USER); sem ele a porta ficaria aberta para a LAN"
  done
  iptables -w -S DOCKER-USER >/dev/null 2>&1 \
    || die "chain DOCKER-USER nao encontrada (Docker com firewall nftables/sem iptables?): sem ela a porta ficaria aberta para a LAN. Rode sem --dev-ip."
fi

IMAGEM="rotaperfumes-mariadb:${VERSAO_MM}-${DIGEST_BASE:0:12}"
CONTAINER_ID=$(docker ps -aq --filter "name=^${CONTAINER}\$")
CONTAINER_RODANDO=false
if [[ -n "$CONTAINER_ID" ]] && [[ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER")" == "true" ]]; then
  CONTAINER_RODANDO=true
fi

# Portas publicadas esperadas (formato do docker inspect).
if $PUBLICAR; then
  PORTAS_ESPERADAS="{\"3306/tcp\":[{\"HostIp\":\"$SITE_IP\",\"HostPort\":\"$PORTA\"}]}"
else
  PORTAS_ESPERADAS="{}"
fi

# Container existente com configuracao diferente da desejada: so com --recriar
# (nada foi mudado ate aqui; evita reiniciar o banco de producao sem querer).
if [[ -n "$CONTAINER_ID" ]] && ! $RECRIAR; then
  DIVERGE=()
  ip_atual=$(docker inspect -f "{{with index .NetworkSettings.Networks \"$REDE_NOME\"}}{{if .IPAMConfig}}{{.IPAMConfig.IPv4Address}}{{end}}{{end}}" "$CONTAINER")
  [[ "$ip_atual" == "$IP_DB" ]] || DIVERGE+=("rede $REDE_NOME com IP fixo $IP_DB (atual: '${ip_atual:-nenhum}')")
  n_redes=$(docker inspect -f '{{len .NetworkSettings.Networks}}' "$CONTAINER")
  [[ "$n_redes" == 1 ]] || DIVERGE+=("so a rede $REDE_NOME (atual: $n_redes redes)")
  portas_atual=$(docker inspect -f '{{json .HostConfig.PortBindings}}' "$CONTAINER")
  [[ "$portas_atual" == "null" ]] && portas_atual="{}"
  [[ "$portas_atual" == "$PORTAS_ESPERADAS" ]] || DIVERGE+=("portas publicadas $PORTAS_ESPERADAS (atual: $portas_atual)")
  [[ "$(docker inspect -f '{{.HostConfig.ReadonlyRootfs}}' "$CONTAINER")" == "$READ_ONLY" ]] \
    || DIVERGE+=("--read-only=$READ_ONLY")
  [[ -n "$(docker inspect -f '{{.Config.User}}' "$CONTAINER")" ]] || DIVERGE+=("--user do mysql (atual: root)")
  if (( ${#DIVERGE[@]} > 0 )); then
    printf '  Container %s difere do desejado:\n' "$CONTAINER" >&2
    printf '    - %s\n' "${DIVERGE[@]}" >&2
    die "rode de novo com --recriar (os dados ficam em $DADOS_DIR; o banco fica fora do ar alguns segundos)"
  fi
fi

# A porta (so com --dev-ip) pode estar ocupada pelo proprio container; por
# qualquer outro, aborta.
if $PUBLICAR && [[ -n "$(ss -Hltn "sport = :$PORTA")" ]]; then
  if ! $CONTAINER_RODANDO || ! docker port "$CONTAINER" 3306/tcp 2>/dev/null | grep -q ":$PORTA\$"; then
    info "Containers publicando a porta $PORTA:"
    docker ps --filter "publish=$PORTA" --format '    {{.Names}}  {{.Image}}  {{.Ports}}' >&2 || true
    die "porta $PORTA ocupada por outro servico. Pare o banco antigo (ex.: docker stop <container_mysql>), use --porta ou rode sem --dev-ip"
  fi
fi
if $PUBLICAR; then
  info "Docker ok; porta $SITE_IP:$PORTA livre (ou ja do $CONTAINER); dev: ${DEV_IPS[*]}."
else
  info "Docker ok; sem --dev-ip: nenhuma porta publicada (so a rede $REDE_NOME)."
fi

# ---------------------------------------------------------------------------
etapa "Imagem $IMAGEM"
install -d -m 700 "$BUILD_DIR"
CHARSET_COLLATIONS=""
if versao_ge "$VERSAO_BASE" "$MIN_CHARSET_COLLATIONS"; then
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
# SEC-30: contas com IDENTIFIED VIA ed25519.
plugin_load_add = auth_ed25519
# DEPLOY-02: hardening. DB-04/DB-06: sem ONLY_FULL_GROUP_BY (ver o script).
local_infile = 0
sql_mode = $SQL_MODE
character_set_server = utf8mb4
collation_server = $COLLATION
$CHARSET_COLLATIONS
# As contas usam IP: sem DNS reverso.
skip_name_resolve = ON
EOF
cat > "$BUILD_DIR/Dockerfile" <<'EOF'
ARG IMAGEM_BASE
FROM ${IMAGEM_BASE}
COPY zz-rotaperfumes.cnf /etc/mysql/conf.d/zz-rotaperfumes.cnf
RUN chmod 644 /etc/mysql/conf.d/zz-rotaperfumes.cnf
LABEL org.opencontainers.image.title="rotaperfumes-mariadb" \
      org.opencontainers.image.description="MariaDB do rotaperfumes com TLS e hardening (deploy/mariadb-docker.sh)"
EOF
docker build --pull -q --build-arg "IMAGEM_BASE=$IMAGEM_BASE" \
  --label "rotaperfumes.imagem-base=$IMAGEM_BASE" -t "$IMAGEM" "$BUILD_DIR" >/dev/null
info "Imagem $IMAGEM pronta ($(docker run --rm --entrypoint mariadbd "$IMAGEM" --version | awk '{print $3}'))."
MYSQL_UID=$(docker run --rm --entrypoint id "$IMAGEM" -u mysql)
MYSQL_GID=$(docker run --rm --entrypoint id "$IMAGEM" -g mysql)
[[ "$MYSQL_UID" =~ ^[0-9]+$ && "$MYSQL_GID" =~ ^[0-9]+$ && "$MYSQL_UID" != 0 ]] || die "uid/gid do mysql da imagem inesperado: $MYSQL_UID:$MYSQL_GID"
info "Usuario mysql da imagem: uid $MYSQL_UID, gid $MYSQL_GID."
if HOST_USR=$(getent passwd "$MYSQL_UID"); then
  aviso "o uid $MYSQL_UID existe no host (${HOST_USR%%:*}): esse usuario le a chave do servidor e o datadir. Confira se e esperado."
fi

# ---------------------------------------------------------------------------
etapa "Certificados em $SSL_DIR"
install -d -m 755 "$SSL_DIR"
install -d -m 700 "$CA_DIR"
# SEC-45: sem host.docker.internal nem gateway do docker0 (a API usa a rede
# rotaperfumes-db e o nome do container).
SAN="IP:$SITE_IP, DNS:$CONTAINER, DNS:localhost, IP:127.0.0.1, DNS:$SITE_HOST"

mesma_chave() { # cert key
  [[ "$(openssl x509 -in "$1" -noout -pubkey 2>/dev/null)" == "$(openssl pkey -in "$2" -pubout 2>/dev/null)" ]]
}
# Lista "TIPO:valor" ordenada (IP Address: vira IP:).
san_normalizado() {
  tr ',' '\n' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/^IP Address:/IP:/' \
    | sed '/^$/d' | sort -u
}
san_do_cert() {
  openssl x509 -in "$1" -noout -ext subjectAltName 2>/dev/null | tail -n +2 | san_normalizado
}
cert_servidor_valido() {
  [[ -s "$SSL_DIR/ca.pem" && -s "$SSL_DIR/server-cert.pem" && -s "$SSL_DIR/server-key.pem" ]] || return 1
  openssl verify -CAfile "$SSL_DIR/ca.pem" "$SSL_DIR/server-cert.pem" >/dev/null 2>&1 || return 1
  openssl x509 -in "$SSL_DIR/server-cert.pem" -noout -checkend $((30 * 86400)) >/dev/null || return 1
  mesma_chave "$SSL_DIR/server-cert.pem" "$SSL_DIR/server-key.pem" || return 1
  # SEC-45: o SAN tem de ser EXATAMENTE o desejado (todas as entradas).
  [[ "$(san_do_cert "$SSL_DIR/server-cert.pem")" == "$(san_normalizado <<<"$SAN")" ]]
}
fingerprint_sha256() { # cert -> hex minusculo sem ':' (formato do deploy/db-ca.fingerprint)
  openssl x509 -in "$1" -noout -fingerprint -sha256 | cut -d= -f2 | tr -d ':' | tr 'A-F' 'a-f'
}

# Chave da CA: --ca-key (lida de la, nao copiada) ou, se ainda existir no
# servidor, $CA_DIR/ca-key.pem (aviso no fim para tira-la daqui).
CA_KEY_USADA=""
if [[ -n "$CA_KEY" ]]; then
  CA_KEY_USADA="$CA_KEY"
elif [[ -s "$CA_DIR/ca-key.pem" ]]; then
  CA_KEY_USADA="$CA_DIR/ca-key.pem"
fi

CERT_MUDOU=false
CA_MUDOU=false
if ! $NOVO_CERT && cert_servidor_valido; then
  info "Certificado existente valido (CA confere, SAN exato, vale mais de 30 dias): reaproveitado."
else
  if $NOVO_CERT; then info "Reemissao pedida (--novo-cert/--nova-ca)."; else info "Certificado ausente, invalido, vencendo ou com SAN diferente de: $SAN"; fi
  TMP=$(mktemp -d /root/rotaperfumes-ssl.XXXXXX)
  trap 'rm -rf "$TMP"' EXIT
  if $NOVA_CA; then
    if [[ -s "$SSL_DIR/ca.pem" ]]; then
      BACKUP="$SSL_DIR.bak-$(date +%Y%m%d-%H%M%S)"
      cp -a "$SSL_DIR" "$BACKUP"
      aviso "--nova-ca: a CA anterior foi trocada (backup em $BACKUP). Os clientes precisam da CA nova."
    fi
    openssl genrsa -out "$TMP/ca-key.pem" 3072 2>/dev/null
    openssl req -x509 -new -key "$TMP/ca-key.pem" -sha256 -days "$CA_DIAS" \
      -subj "/CN=$CA_CN" \
      -addext "basicConstraints=critical,CA:TRUE" \
      -addext "keyUsage=critical,keyCertSign,cRLSign" \
      -out "$TMP/ca.pem"
    install -m 600 -o root -g root "$TMP/ca-key.pem" "$CA_DIR/ca-key.pem"
    install -m 644 -o root -g root "$TMP/ca.pem" "$SSL_DIR/ca.pem"
    CA_KEY_USADA="$CA_DIR/ca-key.pem"
    CA_MUDOU=true
    info "CA nova: $SSL_DIR/ca.pem (chave em $CA_DIR/ca-key.pem: tire do servidor, aviso no fim)."
  else
    [[ -s "$SSL_DIR/ca.pem" ]] || die "nao ha CA em $SSL_DIR/ca.pem. Primeira instalacao: --nova-ca (ou copie o ca.pem da CA atual e use --ca-key)"
    [[ -n "$CA_KEY_USADA" ]] || die "o certificado do servidor precisa ser (re)emitido e a chave da CA nao esta no servidor: informe --ca-key /caminho/ca-key.pem (pendrive; lida de la, nao copiada) ou troque a CA com --nova-ca"
    mesma_chave "$SSL_DIR/ca.pem" "$CA_KEY_USADA" || die "a chave $CA_KEY_USADA nao e a da CA $SSL_DIR/ca.pem"
    openssl x509 -in "$SSL_DIR/ca.pem" -noout -checkend $((CERT_DIAS * 86400)) >/dev/null \
      || die "a CA $SSL_DIR/ca.pem vence antes de $CERT_DIAS dias: troque a CA com --nova-ca"
    info "Reaproveitando a CA ($SSL_DIR/ca.pem, chave $CA_KEY_USADA)."
  fi
  openssl genrsa -out "$TMP/server-key.pem" 3072 2>/dev/null
  openssl req -new -key "$TMP/server-key.pem" -subj "/CN=RotaPerfumes MariaDB Server" -out "$TMP/server.csr"
  cat > "$TMP/server-ext.cnf" <<EOF
basicConstraints = critical, CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = $SAN
EOF
  # Serial aleatorio: sem arquivo .srl ao lado da chave da CA.
  openssl x509 -req -in "$TMP/server.csr" -CA "$SSL_DIR/ca.pem" -CAkey "$CA_KEY_USADA" \
    -set_serial "0x$(openssl rand -hex 16)" -sha256 -days "$CERT_DIAS" \
    -extfile "$TMP/server-ext.cnf" -out "$TMP/server-cert.pem" 2>/dev/null
  openssl verify -CAfile "$SSL_DIR/ca.pem" "$TMP/server-cert.pem" >/dev/null || die "certificado gerado nao valida contra a CA"
  [[ "$(san_do_cert "$TMP/server-cert.pem")" == "$(san_normalizado <<<"$SAN")" ]] || die "SAN do certificado gerado difere do esperado"
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
CA_FP=$(fingerprint_sha256 "$SSL_DIR/ca.pem")

# ---------------------------------------------------------------------------
etapa "Senhas em $SEGREDOS_DIR"
install -d -m 700 -o root -g root "$SEGREDOS_DIR"
# Subpasta montada no container: o entrypoint (como mysql) le a senha do root.
install -d -m 700 -o "$MYSQL_UID" -g "$MYSQL_GID" "$SEGREDOS_DIR/container"
install -d -m 700 -o "$MYSQL_UID" -g "$MYSQL_GID" "$DADOS_DIR"
# --user mysql: o datadir inteiro precisa ser do uid da imagem.
if [[ -n "$(find "$DADOS_DIR" \( ! -uid "$MYSQL_UID" -o ! -gid "$MYSQL_GID" \) -print -quit)" ]]; then
  chown -R "$MYSQL_UID:$MYSQL_GID" "$DADOS_DIR"
  info "Dono do datadir ajustado para $MYSQL_UID:$MYSQL_GID (--user do container)."
fi

DADOS_INICIALIZADOS=false
[[ -d "$DADOS_DIR/mysql" ]] && DADOS_INICIALIZADOS=true

senha_arquivo() { # arquivo -> cria se faltar (reexecucao nao troca senha)
  if [[ ! -s "$1" ]]; then
    gerar_senha > "$1"
    info "Senha nova: $1"
  fi
  chown root:root "$1"
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
# SEC-45 item 4: root.cnf montado read-only no container (aceito).
printf '[client]\nuser=root\npassword=%s\nprotocol=socket\n' "$ROOT_SENHA" > "$SEGREDOS_DIR/root.cnf.tmp"
install -m 400 -o "$MYSQL_UID" -g "$MYSQL_GID" "$SEGREDOS_DIR/root.cnf.tmp" "$SEGREDOS_DIR/container/root.cnf"
rm -f "$SEGREDOS_DIR/root.cnf.tmp"
unset ROOT_SENHA
info "Senhas prontas (so root le). Nada foi impresso."

# ---------------------------------------------------------------------------
etapa "Rede $REDE_NOME e firewall"
subnet_atual=$(docker network inspect -f '{{range .IPAM.Config}}{{.Subnet}} {{end}}' "$REDE_NOME" 2>/dev/null || true)
subnet_atual=${subnet_atual% }
if [[ -z "$subnet_atual" ]] && ! docker network inspect "$REDE_NOME" >/dev/null 2>&1; then
  # IPs dinamicos so em .128-.255: .2/.10/.20 ficam livres para os fixos.
  docker network create --driver bridge --subnet "$REDE_DOCKER" --gateway "$REDE_PREFIXO.1" \
    --ip-range "$REDE_PREFIXO.128/25" --label rotaperfumes.rede=db "$REDE_NOME" >/dev/null
  info "Rede $REDE_NOME criada ($REDE_DOCKER; dinamicos em $REDE_PREFIXO.128/25)."
elif [[ "$subnet_atual" != "$REDE_DOCKER" ]]; then
  die "a rede $REDE_NOME ja existe com outra subnet ('$subnet_atual', esperado $REDE_DOCKER). Confira (docker network inspect $REDE_NOME) ou use --rede-docker"
else
  info "Rede $REDE_NOME ja existe ($REDE_DOCKER)."
  faixa=$(docker network inspect -f '{{range .IPAM.Config}}{{.IPRange}}{{end}}' "$REDE_NOME" 2>/dev/null || true)
  [[ "$faixa" == "$REDE_PREFIXO.128/25" ]] \
    || aviso "a rede $REDE_NOME nao tem --ip-range $REDE_PREFIXO.128/25 (atual: '${faixa:-nenhum}'): um container com IP dinamico pode ocupar $IP_API/$IP_ADMIN. Recrie a rede quando puder."
fi
info "IPs fixos: MariaDB $IP_DB, API $IP_API, admin/import $IP_ADMIN."

# Script persistente do firewall (SEC-45). iptables-restore --noflush: a
# declaracao da chain a esvazia e a recarga e atomica (sem janela aberta).
gerar_script_firewall() {
  cat <<EOF
#!/bin/bash
# Gerado por deploy/mariadb-docker.sh (SEC-45). Nao edite: o script regrava.
# Restringe a porta $PORTA publicada pelo Docker em $SITE_IP as maquinas de dev.
set -euo pipefail
CHAIN=$FW_CHAIN
SITE_IP=$SITE_IP
PORTA=$PORTA
DEV_IPS=(${DEV_IPS[*]+${DEV_IPS[*]}})
for i in \$(seq 1 60); do
  iptables -w -S DOCKER-USER >/dev/null 2>&1 && break
  [[ \$i -eq 60 ]] && { echo "rotaperfumes-db-firewall: chain DOCKER-USER nao existe" >&2; exit 1; }
  sleep 1
done
{
  echo '*filter'
  echo ":\$CHAIN - [0:0]"
  for ip in "\${DEV_IPS[@]+"\${DEV_IPS[@]}"}"; do echo "-A \$CHAIN -s \$ip/32 -j RETURN"; done
  echo "-A \$CHAIN -j DROP"
  echo 'COMMIT'
} | iptables-restore -w --noflush
REGRA=(-p tcp -m conntrack --ctorigdst "\$SITE_IP" --ctorigdstport "\$PORTA" -j "\$CHAIN")
iptables -w -C DOCKER-USER "\${REGRA[@]}" 2>/dev/null || iptables -w -I DOCKER-USER 1 "\${REGRA[@]}"
# Saltos antigos (outro IP/porta) saem depois que o novo esta no lugar.
while read -r -a partes; do
  linha="\${partes[*]}"
  [[ "\$linha" == *"--ctorigdst \$SITE_IP/32 "* && "\$linha" == *"--ctorigdstport \$PORTA "* ]] && continue
  partes[0]=-D
  iptables -w "\${partes[@]}"
done < <(iptables -w -S DOCKER-USER | grep -E -- "-j \$CHAIN( |\\\$)" || true)
EOF
}
gerar_unit_firewall() {
  cat <<EOF
# Gerado por deploy/mariadb-docker.sh (SEC-45). Nao edite: o script regrava.
[Unit]
Description=RotaPerfumes: restringe a porta do MariaDB (chain $FW_CHAIN em DOCKER-USER)
After=docker.service
PartOf=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=$FW_SCRIPT

[Install]
WantedBy=multi-user.target
EOF
}

# Com --dev-ip: instala/atualiza e aplica. Sem --dev-ip mas com a unit de uma
# execucao anterior: recarrega com a lista vazia (so DROP; fail closed).
# Container de teste (--container diferente): nunca mexe no firewall de producao.
if $PUBLICAR || { [[ -f "$FW_UNIT_ARQ" ]] && [[ "$CONTAINER" == "$CONTAINER_PADRAO" ]]; }; then
  for cmd in iptables iptables-restore systemctl; do
    command -v "$cmd" >/dev/null || die "comando '$cmd' nao encontrado (firewall $FW_CHAIN)"
  done
  gerar_script_firewall > "$BUILD_DIR/firewall.tmp"
  bash -n "$BUILD_DIR/firewall.tmp" || die "script de firewall gerado invalido"
  install -m 700 -o root -g root "$BUILD_DIR/firewall.tmp" "$FW_SCRIPT"
  gerar_unit_firewall > "$BUILD_DIR/unit.tmp"
  install -m 644 -o root -g root "$BUILD_DIR/unit.tmp" "$FW_UNIT_ARQ"
  rm -f "$BUILD_DIR/firewall.tmp" "$BUILD_DIR/unit.tmp"
  systemctl daemon-reload
  systemctl enable "$FW_UNIT" >/dev/null 2>&1 || die "systemctl enable $FW_UNIT falhou"
  systemctl restart "$FW_UNIT" || die "a unit $FW_UNIT falhou (journalctl -u $FW_UNIT); porta NAO publicada"
  iptables -w -C DOCKER-USER -p tcp -m conntrack --ctorigdst "$SITE_IP" --ctorigdstport "$PORTA" -j "$FW_CHAIN" 2>/dev/null \
    || die "salto para $FW_CHAIN nao esta em DOCKER-USER apos a unit; porta NAO publicada"
  iptables -w -S "$FW_CHAIN" | grep -qx -- "-A $FW_CHAIN -j DROP" || die "chain $FW_CHAIN sem o DROP final"
  if $PUBLICAR; then
    info "Firewall: $SITE_IP:$PORTA so para ${DEV_IPS[*]} (chain $FW_CHAIN, unit $FW_UNIT habilitada)."
  else
    info "Firewall: sem --dev-ip; chain $FW_CHAIN recarregada so com DROP (unit $FW_UNIT mantida)."
  fi
else
  info "Sem --dev-ip: nada publicado, firewall nao necessario."
fi

# ---------------------------------------------------------------------------
etapa "Container $CONTAINER"
if [[ -n "$CONTAINER_ID" ]] && $RECRIAR; then
  info "Removendo o container antigo (--recriar; dados ficam em $DADOS_DIR)."
  docker rm -f "$CONTAINER" >/dev/null
  CONTAINER_ID=""
  CONTAINER_RODANDO=false
fi
if [[ -z "$CONTAINER_ID" ]]; then
  RUN_ARGS=(
    -d --name "$CONTAINER"
    --restart unless-stopped
    --network "$REDE_NOME" --ip "$IP_DB"
    --user "$MYSQL_UID:$MYSQL_GID"
    --cap-drop ALL
    --security-opt no-new-privileges:true
    --tmpfs "/run/mysqld:rw,nosuid,nodev,noexec,mode=0755,uid=$MYSQL_UID,gid=$MYSQL_GID"
    --tmpfs "/tmp:rw,nosuid,nodev,noexec,mode=1777"
    -e MARIADB_ROOT_PASSWORD_FILE="$C_SEGREDOS/root.senha"
    -e MARIADB_ROOT_HOST=localhost
    -e MARIADB_AUTO_UPGRADE=1
    -v "$DADOS_DIR:/var/lib/mysql"
    -v "$SSL_DIR:$C_SSL:ro"
    -v "$SEGREDOS_DIR/container:$C_SEGREDOS:ro"
    --label "rotaperfumes.cert-sha256=$CERT_SHA"
    --label "rotaperfumes.imagem-base=$IMAGEM_BASE"
    --log-opt max-size=10m --log-opt max-file=5
  )
  $READ_ONLY && RUN_ARGS+=(--read-only)
  # SEC-45: nunca 0.0.0.0; so o IP da LAN e so com --dev-ip (firewall ja ativo).
  $PUBLICAR && RUN_ARGS+=(-p "$SITE_IP:$PORTA:3306")
  docker run "${RUN_ARGS[@]}" "$IMAGEM" >/dev/null
  info "Container criado (imagem $IMAGEM, $REDE_NOME/$IP_DB, read-only=$READ_ONLY$($PUBLICAR && printf ', publicado em %s:%s' "$SITE_IP" "$PORTA"))."
elif ! $CONTAINER_RODANDO; then
  docker start "$CONTAINER" >/dev/null
  info "Container existente iniciado."
else
  info "Container ja em execucao."
fi
if [[ "$(docker inspect -f '{{.Image}}' "$CONTAINER")" != "$(docker image inspect -f '{{.Id}}' "$IMAGEM")" ]]; then
  aviso "o container usa uma imagem diferente de $IMAGEM: rode com --recriar para aplicar (faca backup antes)."
fi

root_sql() { docker exec -i "$CONTAINER" mariadb --defaults-extra-file="$C_SEGREDOS/root.cnf" -N -B "$@"; }
info "Aguardando o servidor definitivo (a inicializacao roda um servidor temporario sem rede)..."
for i in $(seq 1 90); do
  if [[ "$(root_sql -e 'SELECT @@skip_networking' 2>/dev/null)" == "0" ]]; then break; fi
  if [[ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null)" != "true" ]] && (( i > 5 )); then
    docker logs --tail 40 "$CONTAINER" >&2 || true
    if $READ_ONLY; then
      die "o container parou. Se o log mostrar escrita negada fora de /var/lib/mysql, /run/mysqld ou /tmp, rode de novo com --recriar --sem-read-only"
    fi
    die "o container parou (veja o log acima)"
  fi
  [[ $i -eq 90 ]] && { docker logs --tail 40 "$CONTAINER" >&2; die "o MariaDB nao ficou pronto em 180 s"; }
  sleep 2
done
if $CERT_MUDOU && [[ -n "$CONTAINER_ID" ]]; then
  root_sql -e 'FLUSH SSL' && info "Certificado novo recarregado (FLUSH SSL)."
fi
info "MariaDB $(root_sql -e 'SELECT VERSION()') pronto; TLS: $(root_sql -e "SHOW GLOBAL VARIABLES LIKE 'have_ssl'" | awk '{print $2}')."
[[ "$(root_sql -e "SELECT COUNT(*) FROM information_schema.PLUGINS WHERE PLUGIN_NAME = 'ed25519' AND PLUGIN_STATUS = 'ACTIVE'")" == 1 ]] \
  || die "plugin ed25519 nao carregado (plugin_load_add = auth_ed25519 no cnf)"

# ---------------------------------------------------------------------------
etapa "Banco $DB_NOME e contas"
SENHA_APP=$(<"$SEGREDOS_DIR/$DB_APP.senha")
SENHA_ADMIN=$(<"$SEGREDOS_DIR/$DB_ADMIN.senha")
SENHA_MAKE=$(<"$SEGREDOS_DIR/$DB_MAKE.senha")
for s in "$SENHA_APP" "$SENHA_ADMIN" "$SENHA_MAKE"; do
  [[ "$s" =~ ^[A-Za-z0-9]{20,}$ ]] || die "senha em $SEGREDOS_DIR fora do formato [A-Za-z0-9]{20,} (gerada fora do script?)"
done

# Contas desejadas (user@host); o resto das contas do projeto sai (convergencia).
DESEJADAS=("$DB_APP@$IP_API" "$DB_ADMIN@$IP_ADMIN")
for ip in "${DEV_IPS[@]+"${DEV_IPS[@]}"}"; do DESEJADAS+=("$DB_MAKE@$ip"); done

# SQL pelo stdin (nunca no argv). As senhas sao [A-Za-z0-9]: sem escape.
# ALTER USER a cada execucao: converge plugin (ed25519), senha e REQUIRE SSL.
# REVOKE + GRANT: converge os privilegios (some o que nao for o desejado).
sql_contas() {
  local ip
  cat <<EOF
CREATE DATABASE IF NOT EXISTS \`$DB_NOME\` CHARACTER SET utf8mb4 COLLATE $COLLATION;

-- API (rede $REDE_NOME, $IP_API): somente DML.
CREATE USER IF NOT EXISTS '$DB_APP'@'$IP_API' IDENTIFIED VIA ed25519 USING PASSWORD('$SENHA_APP') REQUIRE SSL;
ALTER USER '$DB_APP'@'$IP_API' IDENTIFIED VIA ed25519 USING PASSWORD('$SENHA_APP') REQUIRE SSL;
REVOKE ALL PRIVILEGES, GRANT OPTION FROM '$DB_APP'@'$IP_API';
GRANT SELECT, INSERT, UPDATE, DELETE ON \`$DB_NOME\`.* TO '$DB_APP'@'$IP_API';

-- Import do Jenkins (credencial rotaperfumes-db-admin) e migracoes ($IP_ADMIN).
CREATE USER IF NOT EXISTS '$DB_ADMIN'@'$IP_ADMIN' IDENTIFIED VIA ed25519 USING PASSWORD('$SENHA_ADMIN') REQUIRE SSL;
ALTER USER '$DB_ADMIN'@'$IP_ADMIN' IDENTIFIED VIA ed25519 USING PASSWORD('$SENHA_ADMIN') REQUIRE SSL;
REVOKE ALL PRIVILEGES, GRANT OPTION FROM '$DB_ADMIN'@'$IP_ADMIN';
GRANT ALL PRIVILEGES ON \`$DB_NOME\`.* TO '$DB_ADMIN'@'$IP_ADMIN';
EOF
  for ip in "${DEV_IPS[@]+"${DEV_IPS[@]}"}"; do
    cat <<EOF

-- make db-* da maquina de dev $ip (db-down/db-create precisam de DROP/CREATE;
-- rotaperfumes\\_% = bancos de teste).
CREATE USER IF NOT EXISTS '$DB_MAKE'@'$ip' IDENTIFIED VIA ed25519 USING PASSWORD('$SENHA_MAKE') REQUIRE SSL;
ALTER USER '$DB_MAKE'@'$ip' IDENTIFIED VIA ed25519 USING PASSWORD('$SENHA_MAKE') REQUIRE SSL;
REVOKE ALL PRIVILEGES, GRANT OPTION FROM '$DB_MAKE'@'$ip';
GRANT ALL PRIVILEGES ON \`$DB_NOME\`.* TO '$DB_MAKE'@'$ip';
GRANT ALL PRIVILEGES ON \`rotaperfumes\\_%\`.* TO '$DB_MAKE'@'$ip';
EOF
    if [[ -n "$CASAOS_NATIVE_MOTIVO" ]]; then
      # SEC-30, saida de emergencia (so casaos): cliente sem ed25519.
      printf "ALTER USER '%s'@'%s' IDENTIFIED VIA mysql_native_password USING PASSWORD('%s') REQUIRE SSL;\n" \
        "$DB_MAKE" "$ip" "$SENHA_MAKE"
    fi
  done
  cat <<EOF

-- Higiene: sem contas anonimas e sem o banco test.
DELETE FROM mysql.global_priv WHERE User = '';
DROP DATABASE IF EXISTS test;
FLUSH PRIVILEGES;
EOF
}
sql_contas | root_sql
unset SENHA_APP SENHA_MAKE

if [[ -n "$CASAOS_NATIVE_MOTIVO" ]]; then
  printf '%s %s\n' "$(date -Is)" "$CASAOS_NATIVE_MOTIVO" >> "$SEGREDOS_DIR/casaos-native.motivo"
  chmod 600 "$SEGREDOS_DIR/casaos-native.motivo"
  aviso "SEC-30: casaos com mysql_native_password (saida de emergencia). Motivo registrado em $SEGREDOS_DIR/casaos-native.motivo. Volte ao ed25519 assim que possivel (rode sem --casaos-native-motivo)."
fi

# Convergencia (SEC-45 item 2): contas do projeto com host fora do desejado saem.
eh_desejada() { local c; for c in "${DESEJADAS[@]}"; do [[ "$c" == "$1" ]] && return 0; done; return 1; }
REMOVER_SQL=""
while IFS=$'\t' read -r u h; do
  [[ -n "$u" ]] || continue
  eh_desejada "$u@$h" && continue
  if [[ ! "$h" =~ ^[A-Za-z0-9.:/%_-]*$ ]]; then
    aviso "conta $u com host fora do padrao NAO removida automaticamente; confira: SELECT User, Host FROM mysql.global_priv"
    continue
  fi
  REMOVER_SQL+="DROP USER IF EXISTS '$u'@'$h';"$'\n'
  info "Removendo conta fora do desejado: $u@$h"
done < <(root_sql -e "SELECT User, Host FROM mysql.global_priv WHERE User IN ('$DB_APP', '$DB_ADMIN', '$DB_MAKE') ORDER BY User, Host")
if [[ -n "$REMOVER_SQL" ]]; then
  printf '%s' "$REMOVER_SQL" | root_sql
fi

info "Contas:"
root_sql -e "SELECT CONCAT('    ', User, '@', Host, '  ', COALESCE(JSON_VALUE(Priv, '$.plugin'), '?')) FROM mysql.global_priv WHERE User NOT IN ('mariadb.sys', 'PUBLIC') AND User <> '' ORDER BY User, Host"
OUTRAS=$(root_sql -e "SELECT CONCAT(User, '@', Host) FROM mysql.global_priv WHERE NOT (User = 'root' AND Host = 'localhost') AND User NOT IN ('mariadb.sys', 'PUBLIC', '$DB_APP', '$DB_ADMIN', '$DB_MAKE') ORDER BY User, Host")
if [[ -n "$OUTRAS" ]]; then
  aviso "contas fora do esperado (root@localhost, mariadb.sys e as do projeto): $(tr '\n' ' ' <<<"$OUTRAS")- confira e remova se nao forem suas."
fi
FALTANDO=$(root_sql -e "SELECT COUNT(*) FROM mysql.global_priv WHERE User IN ('$DB_APP', '$DB_ADMIN', '$DB_MAKE') AND COALESCE(JSON_VALUE(Priv, '$.ssl_type'), '0') <> '1'")
[[ "$FALTANDO" == 0 ]] || die "ha conta do projeto sem REQUIRE SSL (ssl_type <> ANY)"

# ---------------------------------------------------------------------------
etapa "Teste de conexao TLS"
# Cliente descartavel na rede interna, com o IP do admin/import ($IP_ADMIN).
cliente() {
  MYSQL_PWD="$SENHA_ADMIN" docker run --rm -i -e MYSQL_PWD \
    --network "$REDE_NOME" --ip "$IP_ADMIN" \
    --cap-drop ALL --security-opt no-new-privileges:true \
    --mount "type=bind,source=$SSL_DIR/ca.pem,target=/ca.pem,readonly" \
    --entrypoint mariadb "$IMAGEM" -h "$CONTAINER" -P 3306 -u "$DB_ADMIN" -N -B "$@"
}
if cliente --ssl-ca=/ca.pem --ssl-verify-server-cert \
     -e "SELECT CONCAT('  conectado como ', CURRENT_USER(), ', ', VERSION(), ', ', VARIABLE_VALUE) FROM information_schema.SESSION_STATUS WHERE VARIABLE_NAME = 'Ssl_version'"; then
  info "TLS com verificacao do certificado ($CONTAINER:3306, rede $REDE_NOME): OK."
else
  die "falhou a conexao TLS verificada em $CONTAINER:3306 pela rede $REDE_NOME (confira o SAN, a conta $DB_ADMIN@$IP_ADMIN e se o IP $IP_ADMIN esta livre)"
fi
if cliente --skip-ssl -e 'SELECT 1' >/dev/null 2>&1; then
  die "conexao SEM TLS foi aceita (require_secure_transport/REQUIRE SSL nao estao valendo)"
fi
info "Conexao sem TLS recusada: OK."
unset SENHA_ADMIN
if $PUBLICAR; then
  if SAIDA=$(timeout 15 openssl s_client -starttls mysql -connect "$SITE_IP:$PORTA" \
               -CAfile "$SSL_DIR/ca.pem" -verify_return_error -verify_ip "$SITE_IP" </dev/null 2>&1); then
    info "Porta publicada $SITE_IP:$PORTA: certificado verificado (s_client -verify_ip): OK."
  else
    printf '%s\n' "$SAIDA" | grep -iE 'verify error|verify return code|errno|connect' | head -5 >&2 || true
    die "openssl s_client nao validou o certificado servido em $SITE_IP:$PORTA"
  fi
fi

cat <<EOF

Pronto. MariaDB no container $CONTAINER (imagem $IMAGEM, base $IMAGEM_BASE),
rede $REDE_NOME ($IP_DB:3306)$($PUBLICAR && printf ', publicado em %s:%s para %s' "$SITE_IP" "$PORTA" "${DEV_IPS[*]}").

CA do banco ($SSL_DIR/ca.pem), fingerprint SHA-256 (formato do deploy/db-ca.fingerprint):
  $CA_FP

Senhas (so root le; nao foram impressas):
  $SEGREDOS_DIR/root.senha              root@localhost (docker exec -it $CONTAINER mariadb -uroot -p)
  $SEGREDOS_DIR/$DB_APP.senha   API: DB_SENHA do Secret file rotaperfumes-api-env
  $SEGREDOS_DIR/$DB_ADMIN.senha credencial Jenkins rotaperfumes-db-admin (import)
  $SEGREDOS_DIR/$DB_MAKE.senha             .env das maquinas de dev (make db-*)

Proximos passos:
  1. api.env (Secret file rotaperfumes-api-env): DB_HOST=$CONTAINER DB_PORT=3306
     DB_USUARIO=$DB_APP DB_TLS_SERVER_NAME=$CONTAINER; a API entra na rede
     $REDE_NOME com ipv4_address $IP_API (deploy/docker-compose.yml).
  2. deploy/db-ca.fingerprint precisa conter $CA_FP
     (senao o estagio "CA do banco" do Jenkins recusa a CA).
  3. Maquinas de dev (so as de --dev-ip): copie $SSL_DIR/ca.pem para
     deploy/ca-cert.pem e no .env use
       DB_HOST=$SITE_IP  DB_PORT=$PORTA  DB_USUARIO=$DB_MAKE  DB_SENHA=<$DB_MAKE.senha>
       DB_TLS_CA=deploy/ca-cert.pem
     (cliente mariadb com o plugin client_ed25519).
EOF
if $CA_MUDOU; then
  aviso "CA NOVA: atualize deploy/db-ca.fingerprint e deploy/ca-cert.pem no repositorio e nas maquinas de dev."
fi
if [[ -s "$CA_DIR/ca-key.pem" ]]; then
  aviso "a chave da CA esta NO SERVIDOR ($CA_DIR/ca-key.pem). Copie para midia offline (pendrive cifrado) e apague daqui: shred -u $CA_DIR/ca-key.pem. Depois, use --ca-key <midia>/ca-key.pem para reemitir."
fi
