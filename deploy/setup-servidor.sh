#!/usr/bin/env bash
# =============================================================================
# Instalacao automatica do rotaperfumes no servidor da LAN (card DEPLOY-01).
#
# Prepara o servidor (Docker/Jenkins/ufw), gera o api.env de producao,
# cria/atualiza o job Jenkins "rotaperfumes-deploy" e dispara o primeiro build.
#
# BANCO (DEPLOY-06): este script NAO instala nem configura banco. O unico
# caminho oficial do banco em producao e o deploy/mariadb-docker.sh (MariaDB
# 11.4 em Docker, rede rotaperfumes-db, TLS e contas ed25519). Rode-o ANTES:
#   sudo bash deploy/mariadb-docker.sh --ca-key <midia>/ca-key.pem [--dev-ip IP ...]
# Aqui so se confere que o container esta no ar e se le a senha do
# rotaperfumes_app gerada por ele (para o api.env).
#
# Jenkins (DEPLOY-04): nativo (servico systemd "jenkins") ou em container
# (com o socket do Docker do host e o CLI docker + compose dentro dele).
#
# NAO cria, altera nem apaga credenciais no Jenkins: o cadastro das 2
# credenciais e MANUAL (o script pausa, explica e depois so VERIFICA via GET).
#
# Uso rapido (no servidor):
#   curl -fsSLO https://raw.githubusercontent.com/residentivo/RotaPerfume/main/deploy/setup-servidor.sh
#   sudo bash setup-servidor.sh --env /tmp/.env --dump /tmp/rotaperfumes-AAAAMMDD-HHMMSS.sql.gz
#
# Re-executavel: cada etapa confere o estado antes de mudar algo.
# Regras: nunca "set -x"; segredos nunca vao para argv, log ou tela.
# =============================================================================
set -euo pipefail
umask 077

readonly JOB_NOME="rotaperfumes-deploy"
readonly REPO_URL="https://github.com/residentivo/RotaPerfume.git"
readonly DB_NOME="rotaperfumes"
readonly DB_APP="rotaperfumes_app"
readonly DB_ADMIN="rotaperfumes_admin"
# Banco: container e rede criados pelo deploy/mariadb-docker.sh (SEC-45).
readonly DB_REDE="rotaperfumes-db"
readonly DB_PORTA_INTERNA="3306"
readonly DUMP_DIR="/opt/rotaperfumes/dumps"
readonly DUMP_DESTINO="$DUMP_DIR/rotaperfumes.sql.gz"
# Jenkins em container sem bind de $DUMP_DIR: o dump vai para o JENKINS_HOME.
readonly DUMP_DIR_JENKINS_CONTAINER="/var/jenkins_home/rotaperfumes-dumps"
readonly CRED_API_ENV="rotaperfumes-api-env"
readonly CRED_DB_ADMIN="rotaperfumes-db-admin"
readonly COMPOSE_MINIMO="2.17.0"
readonly PLUGINS_EXIGIDOS=(workflow-job workflow-cps git credentials plain-credentials credentials-binding)
# Chaves do api.env que o script sempre define (as do --env sao descartadas).
readonly CHAVES_SOBRESCRITAS=(DB_HOST DB_PORT DB_NAME DB_USUARIO DB_SENHA JWT_SECRET CORS_ALLOWED_ORIGINS TRUST_PROXY_HEADERS COOKIE_SECURE)
# Etapas (numeracao continua, igual com Jenkins nativo ou em container).
readonly TOTAL_ETAPAS=9

# Opcoes (preenchidas por ler_argumentos)
ARQ_ENV=""
ARQ_DUMP=""
TURNSTILE_SITE_KEY=""
SITE_HOST="ivo-inspiron-15-3530"
SITE_IP="192.168.168.106"
HTTPS_PORT="8443"
JENKINS_URL="http://localhost:8888"
JENKINS_CONTAINER=""
DB_CONTAINER="rotaperfumes-mariadb"
DB_SEGREDOS_DIR="/root/rotaperfumes-mariadb"
PULAR_BUILD=false
REMOVER_ENTRADAS=false

# Estado
ETAPA_ATUAL=0
USUARIO=""
USUARIO_HOME=""
USUARIO_GRUPO=""
JENKINS_MODO=""   # nativo | container
DUMP_PATH_JOB="$DUMP_DESTINO"
SENHA_APP=""
API_ENV_SAIDA=""
TMP_DIR=""
CURL_CFG=""
COOKIE_JAR=""
RESP_BODY=""
RESP_HEADERS=""
HTTP_CODE=""
BUILD_RESULTADO=""
ARQUIVOS_SECRETOS=()

# -----------------------------------------------------------------------------
# Mensagens
# -----------------------------------------------------------------------------
etapa() {
  ETAPA_ATUAL=$((ETAPA_ATUAL + 1))
  printf '\n==> [%s/%s] %s\n' "$ETAPA_ATUAL" "$TOTAL_ETAPAS" "$1"
}
info() { printf '    %s\n' "$*"; }
ok() { printf '    OK: %s\n' "$*"; }
aviso() { printf '    AVISO: %s\n' "$*" >&2; }
falhar() {
  printf '\nERRO: %s\n' "$*" >&2
  exit 1
}

aviso_destacado() {
  printf '\n    %s\n' "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!" >&2
  local linha
  for linha in "$@"; do printf '    !! %s\n' "$linha" >&2; done
  printf '    %s\n\n' "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!" >&2
}

uso() {
  cat <<'EOF'
Uso: sudo bash setup-servidor.sh --env ARQUIVO [opcoes]

Antes (banco, DEPLOY-06): sudo bash deploy/mariadb-docker.sh [--ca-key ...] [--dev-ip ...]
Este script nao instala nem configura banco.

Obrigatorio:
  --env ARQUIVO               .env de origem (ex. /tmp/.env copiado do Windows).
                              DB_*, JWT_SECRET, CORS, TRUST_PROXY_HEADERS e
                              COOKIE_SECURE (sempre true) sao substituidos; o resto (SMTP, Turnstile...) e mantido.
Opcoes:
  --dump ARQUIVO              Dump .sql.gz a importar no primeiro build
                              (instalado em /opt/rotaperfumes/dumps).
  --turnstile-site-key KEY    Site key PUBLICA do Cloudflare Turnstile.
  --site-host HOST            Hostname de acesso   (padrao ivo-inspiron-15-3530)
  --site-ip IP                IP do servidor na LAN (padrao 192.168.168.106)
  --https-port PORTA          Porta HTTPS do Caddy (padrao 8443)
  --jenkins-url URL           URL do Jenkins vista do servidor
                              (padrao http://localhost:8888)
  --jenkins-container NOME    Container do Jenkins (padrao: detectado pela
                              imagem jenkins/*; sem container = Jenkins nativo)
  --db-container NOME         Container do MariaDB (padrao rotaperfumes-mariadb)
  --db-segredos DIR           Senhas do mariadb-docker.sh
                              (padrao /root/rotaperfumes-mariadb)
  --skip-build                Nao dispara o build no final.
  --remove-inputs             Apaga o --env e o --dump originais no final.
  -h, --help                  Mostra esta ajuda.

Fluxo:
  Servidor: sudo bash deploy/mariadb-docker.sh ...   (banco, uma vez)
  Windows:  scp deploy/dumps/<arquivo>.sql.gz .env USUARIO@ivo-inspiron-15-3530:/tmp/
  Servidor: curl -fsSLO https://raw.githubusercontent.com/residentivo/RotaPerfume/main/deploy/setup-servidor.sh
            sudo bash setup-servidor.sh --env /tmp/.env --dump /tmp/<arquivo>.sql.gz

O script pausa para voce cadastrar MANUALMENTE as credenciais
"rotaperfumes-api-env" e "rotaperfumes-db-admin" na UI do Jenkins.
EOF
}

# -----------------------------------------------------------------------------
# Limpeza de temporarios com segredo
# -----------------------------------------------------------------------------

apagar_seguro() {
  local arq=$1
  [[ -e "$arq" ]] || return 0
  if command -v shred >/dev/null 2>&1; then
    shred -u -- "$arq" 2>/dev/null || rm -f -- "$arq"
  else
    rm -f -- "$arq"
  fi
}

limpar() {
  stty echo 2>/dev/null || true
  local arq
  for arq in "${ARQUIVOS_SECRETOS[@]+"${ARQUIVOS_SECRETOS[@]}"}"; do
    apagar_seguro "$arq"
  done
  if [[ -n "$TMP_DIR" && -d "$TMP_DIR" ]]; then
    for arq in "$TMP_DIR"/*; do apagar_seguro "$arq"; done
    rm -rf -- "$TMP_DIR"
  fi
}

preparar_temporarios() {
  TMP_DIR=$(mktemp -d /tmp/rotaperfumes-setup.XXXXXX)
  CURL_CFG="$TMP_DIR/curl.cfg"
  COOKIE_JAR="$TMP_DIR/cookies.txt"
  RESP_BODY="$TMP_DIR/resp.body"
  RESP_HEADERS="$TMP_DIR/resp.headers"
  : >"$COOKIE_JAR"
}

# -----------------------------------------------------------------------------
# Funcoes puras (testaveis isoladamente)
# -----------------------------------------------------------------------------

# "v2.29.1-desktop.1" -> "2.29.1"
normalizar_versao() {
  local v=${1#v}
  v=${v%%[-+]*}
  printf '%s' "$v"
}

# versao_ge A B -> sucesso se A >= B (componentes numericos, ate 3).
versao_ge() {
  local -a a b
  IFS=. read -ra a <<<"$1"
  IFS=. read -ra b <<<"$2"
  local i x y
  for i in 0 1 2; do
    x=${a[i]:-0}
    y=${b[i]:-0}
    x=${x%%[!0-9]*}
    y=${y%%[!0-9]*}
    x=$((10#${x:-0}))
    y=$((10#${y:-0}))
    if ((x > y)); then return 0; fi
    if ((x < y)); then return 1; fi
  done
  return 0
}

# Substituicoes entre aspas: no bash >= 5.2 (patsub_replacement) um "&" sem
# aspas na substituicao vira o texto casado.
xml_escape() {
  local s=${1//&/"&amp;"}
  s=${s//</"&lt;"}
  s=${s//>/"&gt;"}
  s=${s//\"/"&quot;"}
  s=${s//\'/"&apos;"}
  printf '%s' "$s"
}

# Escapa valor entre aspas duplas no arquivo de config do curl (-K).
curl_cfg_escape() {
  local s=${1//\\/\\\\}
  s=${s//\"/\\\"}
  printf '%s' "$s"
}

chave_sobrescrita() {
  local c
  for c in "${CHAVES_SOBRESCRITAS[@]}"; do
    [[ "$1" == "$c" ]] && return 0
  done
  return 1
}

readonly RE_LINHA_ENV='^[[:space:]]*(export[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=(.*)$'

# stdin -> stdout: normaliza CRLF, tira "export " e remove as chaves sobrescritas.
filtrar_env() {
  local linha
  while IFS= read -r linha || [[ -n "$linha" ]]; do
    linha=${linha%$'\r'}
    if [[ "$linha" =~ $RE_LINHA_ENV ]]; then
      chave_sobrescrita "${BASH_REMATCH[2]}" && continue
      if [[ -n "${BASH_REMATCH[1]}" ]]; then
        linha="${BASH_REMATCH[2]}=${BASH_REMATCH[3]}"
      fi
    fi
    printf '%s\n' "$linha"
  done
}

# stdin -> stdout: nomes das chaves com "$" fora de aspas simples.
chaves_com_dolar() {
  local linha chave valor
  while IFS= read -r linha || [[ -n "$linha" ]]; do
    linha=${linha%$'\r'}
    [[ "$linha" =~ $RE_LINHA_ENV ]] || continue
    chave=${BASH_REMATCH[2]}
    valor=${BASH_REMATCH[3]}
    valor=${valor#"${valor%%[![:space:]]*}"}
    valor=${valor%"${valor##*[![:space:]]}"}
    [[ "$valor" == *'$'* ]] || continue
    [[ "$valor" =~ ^\'.*\'$ ]] && continue
    printf '%s\n' "$chave"
  done
}

# Formata um valor para env_file do compose: aspas simples se preciso.
valor_env() {
  local v=$1
  if [[ "$v" =~ ^[A-Za-z0-9._~+/=:@%^,-]*$ ]]; then
    printf '%s' "$v"
  elif [[ "$v" != *"'"* ]]; then
    printf "'%s'" "$v"
  else
    return 1
  fi
}

# Linhas acrescentadas ao api.env. Args: porta senha_app jwt host ip https_port
# DB_HOST = container do mariadb-docker.sh na rede rotaperfumes-db (SEC-45);
# a porta e a interna do container (3306).
linhas_api_env() {
  local senha jwt
  senha=$(valor_env "$2") || return 1
  jwt=$(valor_env "$3") || return 1
  printf '\n# --- Definido por deploy/setup-servidor.sh ---\n'
  printf 'DB_HOST=%s\n' "$DB_CONTAINER"
  printf 'DB_PORT=%s\n' "$1"
  printf 'DB_NAME=%s\n' "$DB_NOME"
  printf 'DB_USUARIO=%s\n' "$DB_APP"
  printf 'DB_SENHA=%s\n' "$senha"
  printf 'JWT_SECRET=%s\n' "$jwt"
  printf 'CORS_ALLOWED_ORIGINS=https://%s:%s,https://%s:%s\n' "$4" "$6" "$5" "$6"
  printf 'TRUST_PROXY_HEADERS=true\n'
  printf 'COOKIE_SECURE=true\n'
}

# stdin (JSON do pluginManager) -> stdout: plugins exigidos ausentes/inativos.
plugins_faltantes() {
  local json nome obj
  json=$(cat)
  for nome in "$@"; do
    obj=$({ grep -o '{[^{}]*}' <<<"$json" || true; } | { grep -F "\"shortName\":\"$nome\"" || true; })
    [[ "$obj" == *'"active":true'* ]] || printf '%s\n' "$nome"
  done
}

xml_param_string() {
  printf '        <hudson.model.StringParameterDefinition>\n'
  printf '          <name>%s</name>\n' "$(xml_escape "$1")"
  printf '          <description>%s</description>\n' "$(xml_escape "$2")"
  printf '          <defaultValue>%s</defaultValue>\n' "$(xml_escape "$3")"
  printf '          <trim>true</trim>\n'
  printf '        </hudson.model.StringParameterDefinition>\n'
}

xml_param_bool() {
  printf '        <hudson.model.BooleanParameterDefinition>\n'
  printf '          <name>%s</name>\n' "$(xml_escape "$1")"
  printf '          <description>%s</description>\n' "$(xml_escape "$2")"
  printf '          <defaultValue>%s</defaultValue>\n' "$3"
  printf '        </hudson.model.BooleanParameterDefinition>\n'
}

# config.xml do job (mesmos parametros do Jenkinsfile).
gerar_config_xml() {
  cat <<EOF
<?xml version='1.0' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <description>Deploy do rotaperfumes (Docker compose + Caddy). Criado por deploy/setup-servidor.sh.</description>
  <keepDependencies>false</keepDependencies>
  <properties>
    <hudson.model.ParametersDefinitionProperty>
      <parameterDefinitions>
EOF
  xml_param_string SITE_HOST 'Hostname de acesso na LAN' "$SITE_HOST"
  xml_param_string SITE_IP 'IP do servidor na LAN (tambem aceito pelo Caddy)' "$SITE_IP"
  xml_param_string HTTPS_PORT 'Porta HTTPS publicada pelo Caddy' "$HTTPS_PORT"
  xml_param_string TURNSTILE_SITE_KEY 'Site key PUBLICA do Cloudflare Turnstile (embutida no bundle do frontend)' "$TURNSTILE_SITE_KEY"
  xml_param_bool IMPORTAR_DUMP 'Importa o dump (.sql.gz) no MariaDB do servidor - SOBRESCREVE as tabelas' false
  xml_param_string DUMP_PATH 'Caminho do dump no servidor (visivel para o agente Jenkins)' "$DUMP_DESTINO"
  cat <<EOF
      </parameterDefinitions>
    </hudson.model.ParametersDefinitionProperty>
  </properties>
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsScmFlowDefinition" plugin="workflow-cps">
    <scm class="hudson.plugins.git.GitSCM" plugin="git">
      <configVersion>2</configVersion>
      <userRemoteConfigs>
        <hudson.plugins.git.UserRemoteConfig>
          <url>$(xml_escape "$REPO_URL")</url>
        </hudson.plugins.git.UserRemoteConfig>
      </userRemoteConfigs>
      <branches>
        <hudson.plugins.git.BranchSpec>
          <name>*/main</name>
        </hudson.plugins.git.BranchSpec>
      </branches>
      <extensions/>
    </scm>
    <scriptPath>Jenkinsfile</scriptPath>
    <lightweight>true</lightweight>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>
EOF
}

# Sucesso se o arquivo .env traz uma secret de TESTE do Turnstile (Cloudflare).
env_tem_turnstile_teste() {
  grep -Eq '^[[:space:]]*(export[[:space:]]+)?TURNSTILE_SECRET_KEY[[:space:]]*=[[:space:]]*["'"'"']?[123]x0{31}AA["'"'"']?[[:space:]]*$' "$1"
}

# Imagem de Jenkins na saida do "docker ps" (nome[/...]:tag).
eh_imagem_jenkins() {
  [[ "$1" =~ (^|/)jenkins(/[^:[:space:]]+)?(:|@|$) ]]
}

# caminho_no_container MOUNTS CAMINHO_HOST -> caminho visto dentro do
# container, se algum bind (linhas "origem|destino") cobrir o caminho.
caminho_no_container() {
  local mounts=$1 alvo=$2 origem destino
  while IFS='|' read -r origem destino; do
    [[ -n "$origem" && -n "$destino" ]] || continue
    origem=${origem%/}
    if [[ "$alvo" == "$origem" || "$alvo" == "$origem/"* ]]; then
      printf '%s%s' "${destino%/}" "${alvo#"$origem"}"
      return 0
    fi
  done <<<"$mounts"
  return 1
}

# -----------------------------------------------------------------------------
# Argumentos
# -----------------------------------------------------------------------------
exigir_valor() {
  [[ $# -ge 2 && -n "$2" ]] || falhar "A opcao $1 exige um valor (veja --help)."
}

ler_argumentos() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --env) exigir_valor "$@"; ARQ_ENV=$2; shift 2 ;;
      --dump) exigir_valor "$@"; ARQ_DUMP=$2; shift 2 ;;
      --turnstile-site-key) exigir_valor "$@"; TURNSTILE_SITE_KEY=$2; shift 2 ;;
      --site-host) exigir_valor "$@"; SITE_HOST=$2; shift 2 ;;
      --site-ip) exigir_valor "$@"; SITE_IP=$2; shift 2 ;;
      --https-port) exigir_valor "$@"; HTTPS_PORT=$2; shift 2 ;;
      --jenkins-url) exigir_valor "$@"; JENKINS_URL=${2%/}; shift 2 ;;
      --jenkins-container) exigir_valor "$@"; JENKINS_CONTAINER=$2; shift 2 ;;
      --db-container) exigir_valor "$@"; DB_CONTAINER=$2; shift 2 ;;
      --db-segredos) exigir_valor "$@"; DB_SEGREDOS_DIR=${2%/}; shift 2 ;;
      --skip-build) PULAR_BUILD=true; shift ;;
      --remove-inputs) REMOVER_ENTRADAS=true; shift ;;
      --apenas-banco | --skip-mysql)
        falhar "A opcao $1 saiu (DEPLOY-06): o banco e so o deploy/mariadb-docker.sh (sudo bash deploy/mariadb-docker.sh --help)." ;;
      -h | --help) uso; exit 0 ;;
      *) falhar "Opcao desconhecida: $1 (veja --help)." ;;
    esac
  done
  validar_argumentos
}

validar_argumentos() {
  [[ -n "$ARQ_ENV" ]] || falhar "Informe --env ARQUIVO (veja --help)."
  [[ "$SITE_HOST" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ ]] || falhar "--site-host invalido."
  [[ "$SITE_IP" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || falhar "--site-ip invalido (IPv4)."
  # shellcheck disable=SC2015 # o "|| C" deve rodar se A OU B falhar (pretendido)
  [[ "$HTTPS_PORT" =~ ^[0-9]{1,5}$ ]] && ((HTTPS_PORT >= 1 && HTTPS_PORT <= 65535)) ||
    falhar "--https-port invalida."
  [[ "$TURNSTILE_SITE_KEY" =~ ^[A-Za-z0-9_-]*$ ]] || falhar "--turnstile-site-key invalida."
  [[ "$JENKINS_URL" =~ ^https?://[^[:space:]]+$ ]] || falhar "--jenkins-url invalida."
  [[ -z "$JENKINS_CONTAINER" || "$JENKINS_CONTAINER" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || falhar "--jenkins-container invalido."
  [[ "$DB_CONTAINER" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ ]] || falhar "--db-container invalido."
  [[ "$DB_SEGREDOS_DIR" =~ ^/[A-Za-z0-9._/-]+$ ]] || falhar "--db-segredos precisa ser caminho absoluto."
}

# -----------------------------------------------------------------------------
# Pre-checagens
# -----------------------------------------------------------------------------
checar_root() {
  [[ $EUID -eq 0 ]] || falhar "Rode como root: sudo bash $0 ..."
}

checar_root_e_usuario() {
  checar_root
  [[ -n "${SUDO_USER:-}" && "$SUDO_USER" != "root" ]] ||
    falhar "Rode com sudo a partir do SEU usuario (SUDO_USER vazio): o api.env sera gravado na home dele."
  USUARIO=$SUDO_USER
  USUARIO_HOME=$(getent passwd "$USUARIO" | cut -d: -f6)
  USUARIO_GRUPO=$(id -gn "$USUARIO")
  [[ -d "$USUARIO_HOME" ]] || falhar "Home de $USUARIO nao encontrada."
  ok "root via sudo; usuario $USUARIO ($USUARIO_HOME)"
}

checar_comandos() {
  local cmd faltando=()
  for cmd in docker curl openssl gzip install getent; do
    command -v "$cmd" >/dev/null 2>&1 || faltando+=("$cmd")
  done
  ((${#faltando[@]} == 0)) || falhar "Comandos ausentes: ${faltando[*]}"
  ok "comandos: docker curl openssl gzip"
}

checar_compose() {
  local v
  v=$(docker compose version --short 2>/dev/null) || falhar "Plugin 'docker compose' nao encontrado (instale docker-compose-plugin)."
  v=$(normalizar_versao "$v")
  versao_ge "$v" "$COMPOSE_MINIMO" ||
    falhar "docker compose $v < $COMPOSE_MINIMO . Atualize o docker-compose-plugin."
  ok "docker compose $v"
}

checar_entradas() {
  [[ -f "$ARQ_ENV" && -r "$ARQ_ENV" ]] || falhar "--env nao encontrado/legivel: $ARQ_ENV"
  if [[ -n "$ARQ_DUMP" ]]; then
    [[ -f "$ARQ_DUMP" ]] || falhar "--dump nao encontrado: $ARQ_DUMP"
    gzip -t "$ARQ_DUMP" 2>/dev/null || falhar "--dump nao e um gzip valido: $ARQ_DUMP"
    ok "dump integro (gzip -t)"
  else
    info "Sem --dump: o build NAO importara dados."
  fi
}

# DEPLOY-04: Jenkins nativo (systemd) ou em container. Resultado em
# JENKINS_MODO (nativo|container) e JENKINS_CONTAINER.
checar_jenkins() {
  if [[ -z "$JENKINS_CONTAINER" ]] && systemctl cat jenkins >/dev/null 2>&1; then
    id jenkins >/dev/null 2>&1 || falhar "Servico jenkins existe, mas o usuario 'jenkins' nao."
    command -v runuser >/dev/null 2>&1 || falhar "Comando 'runuser' ausente (Jenkins nativo)."
    JENKINS_MODO=nativo
    ok "Jenkins nativo (servico systemd 'jenkins')"
    return 0
  fi
  if [[ -z "$JENKINS_CONTAINER" ]]; then
    local nome imagem achados=()
    while read -r nome imagem; do
      eh_imagem_jenkins "$imagem" && achados+=("$nome")
    done < <(docker ps --format '{{.Names}} {{.Image}}' 2>/dev/null || true)
    ((${#achados[@]} > 0)) || falhar "Jenkins nao encontrado (nem servico systemd 'jenkins', nem container com imagem jenkins). Use --jenkins-container NOME."
    ((${#achados[@]} == 1)) || falhar "Mais de um container de Jenkins (${achados[*]}): escolha com --jenkins-container NOME."
    JENKINS_CONTAINER=${achados[0]}
  fi
  [[ "$(docker inspect -f '{{.State.Running}}' "$JENKINS_CONTAINER" 2>/dev/null)" == true ]] ||
    falhar "Container do Jenkins '$JENKINS_CONTAINER' nao esta rodando."
  JENKINS_MODO=container
  ok "Jenkins em container ($JENKINS_CONTAINER)"
}

# DEPLOY-06: o banco e do deploy/mariadb-docker.sh. Aqui so se confere.
checar_banco_docker() {
  local rodando ip
  rodando=$(docker inspect -f '{{.State.Running}}' "$DB_CONTAINER" 2>/dev/null || true)
  [[ "$rodando" == true ]] ||
    falhar "Container do banco '$DB_CONTAINER' nao esta rodando. Rode antes: sudo bash deploy/mariadb-docker.sh (ver --help; este script nao instala banco)."
  docker network inspect "$DB_REDE" >/dev/null 2>&1 ||
    falhar "Rede docker '$DB_REDE' nao existe. Rode antes: sudo bash deploy/mariadb-docker.sh"
  ip=$(docker inspect -f "{{with index .NetworkSettings.Networks \"$DB_REDE\"}}{{.IPAddress}}{{end}}" "$DB_CONTAINER")
  [[ -n "$ip" ]] || falhar "O container '$DB_CONTAINER' nao esta na rede '$DB_REDE'. Rode: sudo bash deploy/mariadb-docker.sh --recriar"
  local arq
  for arq in "$DB_SEGREDOS_DIR/$DB_APP.senha" "$DB_SEGREDOS_DIR/$DB_ADMIN.senha"; do
    [[ -s "$arq" ]] || falhar "Senha nao encontrada: $arq (gerada pelo deploy/mariadb-docker.sh; use --db-segredos)."
  done
  ok "MariaDB do mariadb-docker.sh: container $DB_CONTAINER na rede $DB_REDE ($ip)"
}

etapa_prechecagens() {
  etapa "Pre-checagens"
  checar_root_e_usuario
  checar_comandos
  checar_compose
  checar_entradas
  checar_jenkins
  checar_banco_docker
}

# -----------------------------------------------------------------------------
# Jenkins com acesso ao Docker
# -----------------------------------------------------------------------------

aguardar_jenkins() {
  local limite=$((SECONDS + 180)) codigo
  info "Aguardando $JENKINS_URL/login responder 200 (ate 180s)..."
  while ((SECONDS < limite)); do
    codigo=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$JENKINS_URL/login" || true)
    if [[ "$codigo" == "200" ]]; then
      ok "Jenkins no ar"
      return 0
    fi
    sleep 3
  done
  falhar "Jenkins nao respondeu 200 em $JENKINS_URL/login (confira --jenkins-url)."
}

etapa_jenkins_docker_nativo() {
  getent group docker >/dev/null || falhar "Grupo 'docker' nao existe."
  if [[ " $(id -nG jenkins) " == *" docker "* ]]; then
    ok "jenkins ja esta no grupo docker"
  else
    usermod -aG docker jenkins
    info "jenkins adicionado ao grupo docker; reiniciando o Jenkins..."
    systemctl restart jenkins
  fi
  aguardar_jenkins
  runuser -u jenkins -- docker ps >/dev/null 2>&1 || falhar "O usuario jenkins ainda nao acessa o Docker."
  ok "jenkins acessa o Docker"
}

# Container: o pipeline precisa do CLI docker + compose e do socket do host
# dentro do container. So confere (nao altera o container do Jenkins).
etapa_jenkins_docker_container() {
  aguardar_jenkins
  docker exec "$JENKINS_CONTAINER" docker ps >/dev/null 2>&1 ||
    falhar "O Jenkins em container ($JENKINS_CONTAINER) nao acessa o Docker do host: monte /var/run/docker.sock, instale o docker-ce-cli na imagem e de ao usuario do Jenkins acesso ao socket (docs/deploy-servidor.md)."
  local v
  v=$(docker exec "$JENKINS_CONTAINER" docker compose version --short 2>/dev/null) ||
    falhar "Sem o plugin 'docker compose' dentro do container $JENKINS_CONTAINER (instale o docker-compose-plugin na imagem)."
  v=$(normalizar_versao "$v")
  versao_ge "$v" "$COMPOSE_MINIMO" || falhar "docker compose $v < $COMPOSE_MINIMO dentro do container $JENKINS_CONTAINER."
  ok "Jenkins em container acessa o Docker do host (compose $v)"
}

etapa_jenkins_docker() {
  etapa "Jenkins com acesso ao Docker ($JENKINS_MODO)"
  if [[ "$JENKINS_MODO" == container ]]; then
    etapa_jenkins_docker_container
  else
    etapa_jenkins_docker_nativo
  fi
}

# -----------------------------------------------------------------------------
# Firewall (so a porta HTTPS; a do banco e do mariadb-docker.sh/DOCKER-USER)
# -----------------------------------------------------------------------------

ufw_ativo() {
  command -v ufw >/dev/null 2>&1 || return 1
  local status
  status=$(ufw status 2>/dev/null || true)
  [[ "${status,,}" == *"status: active"* ]]
}

etapa_firewall() {
  etapa "Firewall (ufw)"
  info "Banco: nada a fazer aqui. A porta do MariaDB so e publicada com --dev-ip do"
  info "  deploy/mariadb-docker.sh e e restringida na chain DOCKER-USER (o ufw nao ve portas do Docker)."
  if ufw_ativo; then
    ufw allow "$HTTPS_PORT/tcp" >/dev/null
    ok "ufw: $HTTPS_PORT/tcp liberada"
    return 0
  fi
  aviso_destacado \
    "ufw INATIVO. O script NAO ativa o firewall. Sugestao (confira antes o acesso SSH):" \
    "  sudo ufw allow OpenSSH" \
    "  sudo ufw allow $HTTPS_PORT/tcp" \
    "  sudo ufw allow 8888/tcp      # Jenkins, se acessado de outra maquina" \
    "  sudo ufw enable"
}

# -----------------------------------------------------------------------------
# Dump
# -----------------------------------------------------------------------------
instalar_dump_nativo() {
  install -d -m 750 -o root -g jenkins "$DUMP_DIR"
  install -m 640 -o root -g jenkins "$ARQ_DUMP" "$DUMP_DESTINO"
  runuser -u jenkins -- test -r "$DUMP_DESTINO" || falhar "jenkins nao consegue ler $DUMP_DESTINO."
  DUMP_PATH_JOB=$DUMP_DESTINO
  ok "dump em $DUMP_DESTINO (root:jenkins 640)"
}

# Container: se $DUMP_DIR estiver num bind do Jenkins, usa o caminho visto
# por ele; senao copia para o JENKINS_HOME (docker cp) com dono do Jenkins.
instalar_dump_container() {
  local mounts destino uid gid
  mounts=$(docker inspect -f '{{range .Mounts}}{{.Source}}|{{.Destination}}{{println}}{{end}}' "$JENKINS_CONTAINER")
  uid=$(docker exec "$JENKINS_CONTAINER" id -u)
  gid=$(docker exec "$JENKINS_CONTAINER" id -g)
  [[ "$uid" =~ ^[0-9]+$ && "$gid" =~ ^[0-9]+$ ]] || falhar "uid/gid do Jenkins no container inesperado."
  if destino=$(caminho_no_container "$mounts" "$DUMP_DESTINO"); then
    install -d -m 750 -o root -g "$gid" "$DUMP_DIR"
    install -m 640 -o root -g "$gid" "$ARQ_DUMP" "$DUMP_DESTINO"
    info "dump em $DUMP_DESTINO (bind do Jenkins: $destino)"
  else
    destino="$DUMP_DIR_JENKINS_CONTAINER/rotaperfumes.sql.gz"
    docker exec -u 0 "$JENKINS_CONTAINER" install -d -m 700 -o "$uid" -g "$gid" "$DUMP_DIR_JENKINS_CONTAINER"
    docker cp "$ARQ_DUMP" "$JENKINS_CONTAINER:$destino" >/dev/null
    docker exec -u 0 "$JENKINS_CONTAINER" chown "$uid:$gid" "$destino"
    docker exec -u 0 "$JENKINS_CONTAINER" chmod 600 "$destino"
    info "dump copiado para $JENKINS_CONTAINER:$destino (sem bind de $DUMP_DIR no container)"
  fi
  docker exec "$JENKINS_CONTAINER" test -r "$destino" || falhar "o Jenkins ($JENKINS_CONTAINER) nao consegue ler $destino."
  DUMP_PATH_JOB=$destino
  ok "DUMP_PATH do job: $DUMP_PATH_JOB"
}

etapa_dump() {
  etapa "Dump do banco"
  if [[ -z "$ARQ_DUMP" ]]; then
    info "Sem --dump: nada a instalar."
    return 0
  fi
  if [[ "$JENKINS_MODO" == container ]]; then instalar_dump_container; else instalar_dump_nativo; fi
}

# -----------------------------------------------------------------------------
# api.env
# -----------------------------------------------------------------------------

avisar_chaves_com_dolar() {
  local chaves
  chaves=$(chaves_com_dolar <"$ARQ_ENV" | sort -u | tr '\n' ' ')
  [[ -z "$chaves" ]] && return 0
  aviso "Valores com '\$' sem aspas simples (o compose interpola): $chaves"
  aviso "Coloque esses valores entre aspas SIMPLES no .env e rode o script de novo."
}

# SEC-37/DEPLOY-02: secret de TESTE (1x/2x/3x...AA) aceita qualquer token.
# Avisa sem imprimir o valor.
avisar_turnstile_teste() {
  env_tem_turnstile_teste "$ARQ_ENV" || return 0
  aviso "TURNSTILE_SECRET_KEY do --env e uma secret de TESTE da Cloudflare: o captcha fica desligado."
  aviso "Desde o SEC-37 a API NAO sobe com ela (COOKIE_SECURE=true), salvo TURNSTILE_PERMITIR_SECRET_TESTE=true."
  aviso "Troque pela secret real do site no Turnstile (docs/deploy-servidor.md, secao 4, item 5)."
}

avisar_turnstile_vazio() {
  grep -Eq '^[[:space:]]*TURNSTILE_SECRET_KEY[[:space:]]*=[[:space:]]*[^[:space:]]' "$ARQ_ENV" && return 0
  aviso "TURNSTILE_SECRET_KEY vazia no --env: o login sera recusado (fail-closed)."
}

# Senha do rotaperfumes_app: lida do arquivo do mariadb-docker.sh (nunca impressa).
ler_senha_app() {
  SENHA_APP=$(<"$DB_SEGREDOS_DIR/$DB_APP.senha")
  [[ "$SENHA_APP" =~ ^[A-Za-z0-9]+$ ]] || falhar "Senha em $DB_SEGREDOS_DIR/$DB_APP.senha fora do formato esperado."
}

etapa_api_env() {
  etapa "Gerando o api.env de producao"
  local tmp="$TMP_DIR/api.env" jwt
  jwt=$(openssl rand -base64 48 | tr -d '\n')
  ler_senha_app
  avisar_chaves_com_dolar
  avisar_turnstile_vazio
  avisar_turnstile_teste
  {
    filtrar_env <"$ARQ_ENV"
    linhas_api_env "$DB_PORTA_INTERNA" "$SENHA_APP" "$jwt" "$SITE_HOST" "$SITE_IP" "$HTTPS_PORT"
  } >"$tmp" || falhar "Senha do app contem aspas simples; nao cabe no env_file."
  SENHA_APP=""
  API_ENV_SAIDA="$USUARIO_HOME/rotaperfumes-api.env"
  ARQUIVOS_SECRETOS+=("$API_ENV_SAIDA")
  install -m 600 -o "$USUARIO" -g "$USUARIO_GRUPO" "$tmp" "$API_ENV_SAIDA"
  apagar_seguro "$tmp"
  ok "gravado $API_ENV_SAIDA (600, $USUARIO); JWT_SECRET novo; DB_HOST=$DB_CONTAINER"
}

# -----------------------------------------------------------------------------
# Jenkins REST (job + leitura)
# -----------------------------------------------------------------------------

cabecalho() {
  { grep -i "^$1:" "$2" || true; } | tail -1 | cut -d: -f2- | tr -d '\r' | sed 's/^[[:space:]]*//'
}

# stdin JSON -> primeiro valor texto do campo (parser minimo, sem jq).
json_texto() {
  { grep -o "\"$1\":\"[^\"]*\"" || true; } | sed -n 1p | cut -d'"' -f4
}

obter_crumb() {
  local resp campo crumb
  resp=$(curl -sS -g -K "$CURL_CFG" -b "$COOKIE_JAR" -c "$COOKIE_JAR" --max-time 30 \
    "$JENKINS_URL/crumbIssuer/api/json") || return 1
  campo=$(json_texto crumbRequestField <<<"$resp")
  crumb=$(json_texto crumb <<<"$resp")
  [[ -n "$campo" && -n "$crumb" ]] || return 1
  printf 'header = "%s: %s"\n' "$(curl_cfg_escape "$campo")" "$(curl_cfg_escape "$crumb")" >>"$CURL_CFG"
}

# jenkins_req METODO CAMINHO [args curl]. Resultado: HTTP_CODE, RESP_BODY, RESP_HEADERS.
jenkins_req() {
  local metodo=$1 caminho=$2 tentativa
  shift 2
  for tentativa in 1 2; do
    : >"$RESP_BODY"
    : >"$RESP_HEADERS"
    HTTP_CODE=$(curl -sS -g -K "$CURL_CFG" -b "$COOKIE_JAR" -c "$COOKIE_JAR" --max-time 60 \
      -X "$metodo" -D "$RESP_HEADERS" -o "$RESP_BODY" -w '%{http_code}' \
      "$@" "$JENKINS_URL$caminho" 2>/dev/null) || HTTP_CODE=000
    if [[ "$HTTP_CODE" == 403 && $tentativa == 1 ]] && grep -qi crumb "$RESP_BODY" && obter_crumb; then
      continue
    fi
    return 0
  done
}

pedir_token_jenkins() {
  local usuario token
  info "Autenticacao no Jenkins ($JENKINS_URL). Gere um API token em:"
  info "  Jenkins > (seu usuario, canto superior direito) > Security > API Token > Add new Token"
  read -rp "    Usuario do Jenkins: " usuario
  IFS= read -rsp "    API token: " token
  echo >&2
  [[ -n "$usuario" && -n "$token" ]] || falhar "Usuario e token sao obrigatorios."
  printf 'user = "%s:%s"\n' "$(curl_cfg_escape "$usuario")" "$(curl_cfg_escape "$token")" >"$CURL_CFG"
  chmod 600 "$CURL_CFG"
}

validar_login_jenkins() {
  jenkins_req GET /whoAmI/api/json
  [[ "$HTTP_CODE" == 200 ]] || falhar "Login no Jenkins falhou (HTTP $HTTP_CODE). Confira usuario/token."
  grep -q '"authenticated":true' "$RESP_BODY" || falhar "Jenkins nao autenticou o usuario."
  local nome
  nome=$(json_texto name <"$RESP_BODY")
  [[ -n "$nome" && "$nome" != anonymous ]] || falhar "Jenkins respondeu como anonimo: token invalido."
  ok "autenticado no Jenkins como $nome"
}

checar_plugins() {
  jenkins_req GET '/pluginManager/api/json?depth=1&tree=plugins[shortName,active]'
  [[ "$HTTP_CODE" == 200 ]] || falhar "Nao foi possivel listar plugins (HTTP $HTTP_CODE; precisa ser admin)."
  local faltando
  faltando=$(plugins_faltantes "${PLUGINS_EXIGIDOS[@]}" <"$RESP_BODY" | tr '\n' ' ')
  [[ -z "$faltando" ]] ||
    falhar "Plugins ausentes/inativos: $faltando. Instale em Gerenciar Jenkins > Plugins e rode de novo."
  ok "plugins: ${PLUGINS_EXIGIDOS[*]}"
}

publicar_job() {
  local xml="$TMP_DIR/config.xml" args
  gerar_config_xml >"$xml"
  args=(-H 'Content-Type: application/xml' --data-binary "@$xml")
  jenkins_req GET "/job/$JOB_NOME/api/json?tree=name"
  if [[ "$HTTP_CODE" == 200 ]]; then
    jenkins_req POST "/job/$JOB_NOME/config.xml" "${args[@]}"
    [[ "$HTTP_CODE" == 200 ]] || falhar "Falha ao atualizar o job (HTTP $HTTP_CODE)."
    ok "job $JOB_NOME atualizado"
  else
    jenkins_req POST "/createItem?name=$JOB_NOME" "${args[@]}"
    [[ "$HTTP_CODE" == 200 ]] || falhar "Falha ao criar o job (HTTP $HTTP_CODE)."
    ok "job $JOB_NOME criado"
  fi
}

etapa_jenkins_job() {
  etapa "Job Jenkins $JOB_NOME"
  aguardar_jenkins
  pedir_token_jenkins
  validar_login_jenkins
  checar_plugins
  publicar_job
}

# -----------------------------------------------------------------------------
# Pausa para as credenciais MANUAIS
# -----------------------------------------------------------------------------

credencial_existe() {
  jenkins_req GET "/credentials/store/system/domain/_/credential/$1/api/json?tree=id"
  [[ "$HTTP_CODE" == 200 ]]
}

instrucoes_credenciais() {
  cat <<EOF

    ---------------------------------------------------------------------
    PAUSA: cadastre (ou ATUALIZE, se ja existirem) as 2 credenciais no
    Jenkins. O script NAO mexe em credenciais; ele so confere se existem.
    ---------------------------------------------------------------------
    a) No PC Windows (PowerShell), baixe o api.env gerado:
         scp $USUARIO@$SITE_HOST:rotaperfumes-api.env .
    b) Jenkins > Gerenciar Jenkins > Credentials > System >
       Global credentials (unrestricted) > Add Credentials:
         Kind: Secret file | File: rotaperfumes-api.env | ID: $CRED_API_ENV
       (se ja existir: abra a credencial > Update > envie o arquivo novo)
    c) Add Credentials:
         Kind: Username with password | Username: $DB_ADMIN
         Password: conteudo de $DB_SEGREDOS_DIR/$DB_ADMIN.senha
                   (gerada pelo mariadb-docker.sh; leia no servidor com
                    sudo cat, sem colar em chat/log) | ID: $CRED_DB_ADMIN
    d) Apague o rotaperfumes-api.env do Windows:
         Remove-Item .\\rotaperfumes-api.env
    ---------------------------------------------------------------------
EOF
}

etapa_credenciais() {
  etapa "Credenciais no Jenkins (cadastro MANUAL)"
  instrucoes_credenciais
  local resp faltando id
  while true; do
    read -rp "    Pressione ENTER quando terminar (q para sair)... " resp
    [[ "$resp" == q || "$resp" == Q ]] && falhar "Interrompido. Rode o script de novo quando quiser (o api.env foi apagado)."
    faltando=()
    for id in "$CRED_API_ENV" "$CRED_DB_ADMIN"; do
      credencial_existe "$id" || faltando+=("$id")
    done
    ((${#faltando[@]} == 0)) && break
    aviso "Credencial(is) nao encontrada(s) em System > Global: ${faltando[*]} (confira o ID exato)."
  done
  ok "credenciais $CRED_API_ENV e $CRED_DB_ADMIN encontradas"
  apagar_seguro "$API_ENV_SAIDA"
  ok "apagado $API_ENV_SAIDA do servidor"
}

# -----------------------------------------------------------------------------
# Build
# -----------------------------------------------------------------------------

disparar_build() {
  local importar=false local_fila
  [[ -n "$ARQ_DUMP" ]] && importar=true
  jenkins_req POST "/job/$JOB_NOME/buildWithParameters" \
    --data-urlencode "IMPORTAR_DUMP=$importar" \
    --data-urlencode "DUMP_PATH=$DUMP_PATH_JOB" \
    --data-urlencode "SITE_HOST=$SITE_HOST" \
    --data-urlencode "SITE_IP=$SITE_IP" \
    --data-urlencode "HTTPS_PORT=$HTTPS_PORT" \
    --data-urlencode "TURNSTILE_SITE_KEY=$TURNSTILE_SITE_KEY"
  [[ "$HTTP_CODE" == 201 ]] || falhar "Falha ao disparar o build (HTTP $HTTP_CODE)."
  local_fila=$(cabecalho Location "$RESP_HEADERS")
  local_fila=$(grep -o '/queue/item/[0-9]*/' <<<"$local_fila") || falhar "Location da fila ausente."
  info "Build enfileirado (IMPORTAR_DUMP=$importar); aguardando executor..." >&2
  printf '%s' "$local_fila"
}

aguardar_numero_build() {
  local fila=$1 limite=$((SECONDS + 600)) num
  while ((SECONDS < limite)); do
    jenkins_req GET "${fila}api/json"
    if [[ "$HTTP_CODE" == 200 ]]; then
      grep -q '"cancelled":true' "$RESP_BODY" && falhar "Build cancelado na fila."
      num=$({ grep -o '"executable":{[^}]*}' "$RESP_BODY" || true; } | { grep -o '"number":[0-9]*' || true; } | cut -d: -f2)
      if [[ -n "$num" ]]; then
        printf '%s' "$num"
        return 0
      fi
    fi
    sleep 3
  done
  falhar "O build nao saiu da fila em 10 min (executor ocupado/offline?)."
}

acompanhar_log() {
  local num=$1 pos=0 mais tam erros=0
  while true; do
    jenkins_req GET "/job/$JOB_NOME/$num/logText/progressiveText?start=$pos"
    if [[ "$HTTP_CODE" != 200 ]]; then
      ((++erros > 40)) && falhar "Log do build indisponivel (HTTP $HTTP_CODE)."
      sleep 3
      continue
    fi
    cat "$RESP_BODY"
    tam=$(cabecalho X-Text-Size "$RESP_HEADERS")
    mais=$(cabecalho X-More-Data "$RESP_HEADERS")
    [[ -n "$tam" ]] && pos=$tam
    [[ "$mais" == true ]] || return 0
    sleep 2
  done
}

resultado_build() {
  local num=$1 tentativa res
  for tentativa in $(seq 1 30); do
    jenkins_req GET "/job/$JOB_NOME/$num/api/json?tree=result,building"
    res=$(json_texto result <"$RESP_BODY" || true)
    if [[ -n "$res" ]]; then
      printf '%s' "$res"
      return 0
    fi
    sleep 2
  done
  printf 'DESCONHECIDO'
}

etapa_build() {
  etapa "Primeiro build"
  if $PULAR_BUILD; then
    info "--skip-build: rode o job $JOB_NOME manualmente (Construir com parametros)."
    return 0
  fi
  local fila num
  fila=$(disparar_build)
  num=$(aguardar_numero_build "$fila")
  info "Build #$num em execucao: $JENKINS_URL/job/$JOB_NOME/$num/console"
  printf '\n----- log do build #%s -----\n' "$num"
  acompanhar_log "$num"
  printf '\n----- fim do log -----\n'
  BUILD_RESULTADO=$(resultado_build "$num")
  if [[ "$BUILD_RESULTADO" == SUCCESS ]]; then ok "build #$num SUCCESS"; else aviso "build #$num terminou com $BUILD_RESULTADO"; fi
}

# -----------------------------------------------------------------------------
# Resumo
# -----------------------------------------------------------------------------

exportar_ca_caddy() {
  local destino="$USUARIO_HOME/rotaperfumes-caddy-root.crt" tmp="$TMP_DIR/root.crt"
  docker cp rotaperfumes-caddy-1:/data/caddy/pki/authorities/local/root.crt "$tmp" >/dev/null 2>&1 || return 1
  install -m 644 -o "$USUARIO" -g "$USUARIO_GRUPO" "$tmp" "$destino"
  printf '%s' "$destino"
}

instrucoes_ca() {
  local crt
  if crt=$(exportar_ca_caddy); then
    info "CA do Caddy exportada em $crt. No Windows:"
  else
    info "Exporte a CA do Caddy no servidor (depois de um build verde):"
    info "  docker cp rotaperfumes-caddy-1:/data/caddy/pki/authorities/local/root.crt ~/rotaperfumes-caddy-root.crt"
    info "No Windows:"
  fi
  info "  scp $USUARIO@$SITE_HOST:rotaperfumes-caddy-root.crt ."
  info "  duplo clique > Instalar Certificado > Autoridades de Certificacao Raiz Confiaveis"
}

tratar_entradas() {
  if $REMOVER_ENTRADAS; then
    apagar_seguro "$ARQ_ENV"
    [[ -n "$ARQ_DUMP" ]] && apagar_seguro "$ARQ_DUMP"
    ok "--env e --dump originais apagados"
    return 0
  fi
  aviso_destacado "Apague os arquivos de entrada (contem segredos/dados):" \
    "  shred -u $ARQ_ENV${ARQ_DUMP:+ $ARQ_DUMP}"
}

etapa_resumo() {
  etapa "Resumo"
  info "Acesso: https://$SITE_HOST:$HTTPS_PORT  (se o PC nao resolver o nome, adicione"
  info "  '$SITE_IP  $SITE_HOST' em C:\\Windows\\System32\\drivers\\etc\\hosts)"
  instrucoes_ca
  info "Turnstile: cadastre o hostname '$SITE_HOST' no site do Turnstile (Cloudflare)."
  [[ -z "$TURNSTILE_SITE_KEY" ]] && aviso "Sem --turnstile-site-key: o widget nao aparece; rode o job com TURNSTILE_SITE_KEY."
  if [[ -n "$ARQ_DUMP" ]]; then
    if [[ "$JENKINS_MODO" == container && "$DUMP_PATH_JOB" != "$DUMP_DESTINO" ]]; then
      info "Depois de conferir os dados, apague o dump: sudo docker exec $JENKINS_CONTAINER rm $DUMP_PATH_JOB"
    else
      info "Depois de conferir os dados, apague o dump: sudo rm $DUMP_DESTINO"
    fi
  fi
  info "Proximos deploys: Jenkins > $JOB_NOME > Construir com parametros (IMPORTAR_DUMP desmarcado)."
  tratar_entradas
}

# -----------------------------------------------------------------------------
main() {
  ler_argumentos "$@"
  preparar_temporarios
  trap limpar EXIT
  trap 'exit 130' INT TERM
  etapa_prechecagens
  etapa_jenkins_docker
  etapa_firewall
  etapa_dump
  etapa_api_env
  etapa_jenkins_job
  etapa_credenciais
  etapa_build
  etapa_resumo
  if [[ -n "$BUILD_RESULTADO" && "$BUILD_RESULTADO" != SUCCESS ]]; then
    falhar "O build nao terminou em SUCCESS ($BUILD_RESULTADO). Veja o log acima."
  fi
  printf '\nConcluido.\n'
}

if [[ "${BASH_SOURCE[0]:-$0}" == "$0" ]]; then
  main "$@"
fi
