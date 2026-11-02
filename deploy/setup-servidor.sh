#!/usr/bin/env bash
# =============================================================================
# Instalacao automatica do rotaperfumes no servidor da LAN (card DEPLOY-01).
#
# Prepara o servidor (Docker/Jenkins/banco MySQL 8 ou MariaDB/ufw), cria o
# banco e os usuarios,
# gera o api.env de producao, cria/atualiza o job Jenkins "rotaperfumes-deploy"
# e dispara o primeiro build.
#
# NAO cria, altera nem apaga credenciais no Jenkins: o cadastro das 2
# credenciais e MANUAL (o script pausa, explica e depois so VERIFICA via GET).
#
# Uso rapido (no servidor):
#   curl -fsSLO https://raw.githubusercontent.com/residentivo/RotaPerfume/main/deploy/setup-servidor.sh
#   sudo bash setup-servidor.sh --env /tmp/.env --dump /tmp/rotaperfumes-AAAAMMDD-HHMMSS.sql.gz
# So o hardening/bind-address do banco (servidor ja instalado, DEPLOY-02):
#   sudo bash setup-servidor.sh --apenas-banco
#
# Re-executavel: cada etapa confere o estado antes de mudar algo.
# Regras: nunca "set -x"; segredos nunca vao para argv, log ou tela.
# =============================================================================
set -euo pipefail
umask 077

readonly TOTAL_ETAPAS=11
readonly JOB_NOME="rotaperfumes-deploy"
readonly REPO_URL="https://github.com/residentivo/RotaPerfume.git"
readonly DB_NOME="rotaperfumes"
readonly DB_APP="rotaperfumes_app"
readonly DB_ADMIN="rotaperfumes_admin"
readonly DB_HOST_DOCKER="172.16.0.0/255.240.0.0"
readonly FAIXA_DOCKER="172.16.0.0/12"
readonly DUMP_DIR="/opt/rotaperfumes/dumps"
readonly DUMP_DESTINO="$DUMP_DIR/rotaperfumes.sql.gz"
readonly CRED_API_ENV="rotaperfumes-api-env"
readonly CRED_DB_ADMIN="rotaperfumes-db-admin"
readonly COMPOSE_MINIMO="2.17.0"
readonly PLUGINS_EXIGIDOS=(workflow-job workflow-cps git credentials plain-credentials credentials-binding)
# Chaves do api.env que o script sempre define (as do --env sao descartadas).
readonly CHAVES_SOBRESCRITAS=(DB_HOST DB_PORT DB_NAME DB_USUARIO DB_SENHA JWT_SECRET CORS_ALLOWED_ORIGINS TRUST_PROXY_HEADERS COOKIE_SECURE)
# Drop-ins [mysqld] do banco (DEPLOY-02): hardening sempre; bind so se loopback.
readonly DROPIN_HARDENING="zz-rotaperfumes-hardening.cnf"
readonly DROPIN_BIND="zz-rotaperfumes-bind.cnf"
readonly DROPIN_LEGADO="zz-rotaperfumes.cnf"
readonly COLLATION_PROJETO="utf8mb4_unicode_ci"
readonly SQL_MODE_MARIADB="STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION"
# MySQL 8: mantem ONLY_FULL_GROUP_BY (padrao do 8.0). NO_AUTO_CREATE_USER nao existe no 8.0.
readonly SQL_MODE_MYSQL="ONLY_FULL_GROUP_BY,$SQL_MODE_MARIADB"
# character_set_collations existe no MariaDB >= 11.2 (no 11.2+ ele faz o
# "CHARACTER SET utf8mb4" sem COLLATE cair em utf8mb4_uca1400_ai_ci).
readonly MARIADB_MIN_CHARSET_COLLATIONS="11.2"

# Opcoes (preenchidas por ler_argumentos)
ARQ_ENV=""
ARQ_DUMP=""
TURNSTILE_SITE_KEY=""
SITE_HOST="ivo-inspiron-15-3530"
SITE_IP="192.168.168.106"
HTTPS_PORT="8443"
JENKINS_URL="http://localhost:8888"
PULAR_MYSQL=false
PULAR_BUILD=false
REMOVER_ENTRADAS=false
APENAS_BANCO=false

# Estado
USUARIO=""
USUARIO_HOME=""
USUARIO_GRUPO=""
MYSQL_BIN=""
MYSQL_ROOT_SENHA=""
MYSQL_SABOR=""
MYSQL_VERSAO=""
MYSQL_SERVICO=""
DB_PORTA=""
SENHA_APP=""
SENHA_ADMIN=""
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
etapa() { printf '\n==> [%s/%s] %s\n' "$1" "$TOTAL_ETAPAS" "$2"; }
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
     sudo bash setup-servidor.sh --apenas-banco

Obrigatorio (exceto com --apenas-banco):
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
  --apenas-banco              So aplica o hardening (local_infile, sql_mode,
                              charset/collation) e o bind-address do banco
                              (MySQL 8 ou MariaDB) e sai. Nao usa --env,
                              Docker, compose nem Jenkins. Para servidor
                              ja instalado.
  --skip-mysql                Nao cria banco/usuarios no MySQL/MariaDB (pede a
                              senha atual do rotaperfumes_app para o api.env).
  --skip-build                Nao dispara o build no final.
  --remove-inputs             Apaga o --env e o --dump originais no final.
  -h, --help                  Mostra esta ajuda.

Fluxo:
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

# Sucesso se TODOS os enderecos do bind-address forem loopback.
bind_so_loopback() {
  local valor=${1// /}
  [[ -n "$valor" ]] || return 1
  local -a partes
  IFS=, read -ra partes <<<"$valor"
  local p
  for p in "${partes[@]}"; do
    case "$p" in
      127.* | localhost | ::1) ;;
      *) return 1 ;;
    esac
  done
  return 0
}

# Escapa um literal SQL entre aspas simples (barra invertida e aspa simples).
sql_escape() {
  local s=${1//\\/\\\\}
  s=${s//\'/\'\'}
  printf '%s' "$s"
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
linhas_api_env() {
  local senha jwt
  senha=$(valor_env "$2") || return 1
  jwt=$(valor_env "$3") || return 1
  printf '\n# --- Definido por deploy/setup-servidor.sh ---\n'
  printf 'DB_HOST=host.docker.internal\n'
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

# SQL de banco/usuarios. Args: senha_app senha_admin ('' = manter a atual).
gerar_sql() {
  local host="'$DB_HOST_DOCKER'"
  # sql_mode vazio so nesta sessao: garante que "\\" seja escape (sem NO_BACKSLASH_ESCAPES).
  printf "SET SESSION sql_mode = '';\n"
  printf 'CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n' "$DB_NOME"
  sql_usuario "$DB_APP" "$1"
  printf 'GRANT SELECT, INSERT, UPDATE, DELETE ON %s.* TO %s@%s;\n' "$DB_NOME" "'$DB_APP'" "$host"
  sql_usuario "$DB_ADMIN" "$2"
  printf 'GRANT ALL PRIVILEGES ON %s.* TO %s@%s;\n' "$DB_NOME" "'$DB_ADMIN'" "$host"
  printf 'FLUSH PRIVILEGES;\n'
}

sql_usuario() {
  [[ -n "$2" ]] || return 0
  local conta senha
  conta="'$1'@'$DB_HOST_DOCKER'"
  senha=$(sql_escape "$2")
  printf "CREATE USER IF NOT EXISTS %s IDENTIFIED BY '%s';\n" "$conta" "$senha"
  printf "ALTER USER %s IDENTIFIED BY '%s';\n" "$conta" "$senha"
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

# --- Hardening do banco (DEPLOY-02) ------------------------------------------

# Nome legivel do sabor para mensagens. Arg: SABOR (mysql|mariadb; padrao $MYSQL_SABOR).
nome_sabor() {
  case "${1:-$MYSQL_SABOR}" in
    mysql) printf 'MySQL' ;;
    mariadb) printf 'MariaDB' ;;
    *) printf 'banco' ;;
  esac
}

# sql_mode_esperado SABOR -> lista do drop-in de hardening.
sql_mode_esperado() {
  case "$1" in
    mysql) printf '%s' "$SQL_MODE_MYSQL" ;;
    mariadb) printf '%s' "$SQL_MODE_MARIADB" ;;
    *) return 1 ;;
  esac
}

# Sucesso se o sabor/versao aceita character_set_collations (MariaDB >= 11.2).
usa_character_set_collations() {
  [[ "$1" == mariadb ]] || return 1
  versao_ge "$(normalizar_versao "$2")" "$MARIADB_MIN_CHARSET_COLLATIONS"
}

# "a, B ,c" -> "A,B,C" ordenado e sem repeticoes (para comparar como conjunto).
sql_mode_normalizado() {
  local s=${1^^}
  s=${s//[[:space:]]/}
  local -a itens
  IFS=, read -ra itens <<<"$s"
  ((${#itens[@]} == 0)) && return 0
  printf '%s\n' "${itens[@]}" | sed '/^$/d' | sort -u | paste -sd, -
}

# sql_mode_igual A B -> sucesso se os dois sql_mode tem os mesmos modos.
sql_mode_igual() {
  [[ "$(sql_mode_normalizado "$1")" == "$(sql_mode_normalizado "$2")" ]]
}

# conteudo_dropin_hardening SABOR VERSAO -> stdout. Sem bind-address e sem
# opcoes exclusivas do outro sabor (nada de NO_AUTO_CREATE_USER nem
# default_collation_for_utf8mb4).
conteudo_dropin_hardening() {
  local modo
  modo=$(sql_mode_esperado "$1") || return 1
  printf '# Gerado por deploy/setup-servidor.sh (rotaperfumes, DEPLOY-02): hardening.\n'
  printf '# Nao edite: o script regrava este arquivo. O bind fica em %s.\n' "$DROPIN_BIND"
  printf '[mysqld]\n'
  printf 'local_infile = 0\n'
  printf 'sql_mode = %s\n' "$modo"
  printf 'character_set_server = utf8mb4\n'
  printf 'collation_server = %s\n' "$COLLATION_PROJETO"
  if usa_character_set_collations "$1" "$2"; then
    printf 'character_set_collations = utf8mb4=%s\n' "$COLLATION_PROJETO"
  fi
}

# conteudo_dropin_bind [ENDERECO] -> stdout (so o bind-address; padrao
# 0.0.0.0, o ENDERECO so vem da migracao do legado).
conteudo_dropin_bind() {
  printf '# Gerado por deploy/setup-servidor.sh (rotaperfumes, DEPLOY-02): bind-address.\n'
  printf '# Gravado so porque o banco escutava apenas em loopback (containers nao alcancam).\n'
  printf '[mysqld]\n'
  printf 'bind-address = %s\n' "${1:-0.0.0.0}"
}

# SET GLOBAL (nao PERSIST: o drop-in e a fonte da verdade no proximo restart).
sql_set_global_hardening() {
  local modo
  modo=$(sql_mode_esperado "$1") || return 1
  printf "SET GLOBAL local_infile = 0, sql_mode = '%s', character_set_server = 'utf8mb4', collation_server = '%s'" \
    "$modo" "$COLLATION_PROJETO"
  if usa_character_set_collations "$1" "$2"; then
    printf ", character_set_collations = 'utf8mb4=%s'" "$COLLATION_PROJETO"
  fi
  printf '\n'
}

# Consulta das variaveis conferidas (saida "nome<TAB>valor").
sql_variaveis_hardening() {
  printf "SHOW GLOBAL VARIABLES WHERE Variable_name IN ('local_infile','sql_mode','character_set_server','collation_server','character_set_collations')"
}

# valores_divergentes SABOR VERSAO < "nome<TAB>valor" -> nomes que divergem
# do esperado (um por linha). Variavel esperada ausente tambem diverge.
valores_divergentes() {
  local sabor=$1 versao=$2 modo nome valor
  modo=$(sql_mode_esperado "$sabor") || return 1
  local -A vistos=()
  while IFS=$'\t' read -r nome valor || [[ -n "$nome" ]]; do
    nome=${nome,,}
    valor=${valor%$'\r'}
    [[ -n "$nome" ]] || continue
    vistos[$nome]=1
    case "$nome" in
      local_infile) [[ "${valor^^}" == OFF || "$valor" == 0 ]] || printf 'local_infile\n' ;;
      sql_mode) sql_mode_igual "$valor" "$modo" || printf 'sql_mode\n' ;;
      character_set_server) [[ "${valor,,}" == utf8mb4 ]] || printf 'character_set_server\n' ;;
      collation_server) [[ "${valor,,}" == "$COLLATION_PROJETO" ]] || printf 'collation_server\n' ;;
      character_set_collations)
        usa_character_set_collations "$sabor" "$versao" || continue
        valor=${valor,,}
        [[ ",${valor//[[:space:]]/}," == *",utf8mb4=$COLLATION_PROJETO,"* ]] || printf 'character_set_collations\n'
        ;;
    esac
  done
  local esperadas=(local_infile sql_mode character_set_server collation_server)
  usa_character_set_collations "$sabor" "$versao" && esperadas+=(character_set_collations)
  for nome in "${esperadas[@]}"; do
    [[ -n "${vistos[$nome]:-}" ]] || printf '%s\n' "$nome"
  done
}

# gravar_se_mudou TMP DESTINO -> 0 = gravou (conteudo novo), 1 = ja estava igual.
gravar_se_mudou() {
  if [[ -f "$2" ]] && cmp -s -- "$1" "$2"; then
    return 1
  fi
  install -m 644 -o root -g root "$1" "$2"
  return 0
}

# Sucesso se o arquivo .env traz uma secret de TESTE do Turnstile (Cloudflare).
env_tem_turnstile_teste() {
  grep -Eq '^[[:space:]]*(export[[:space:]]+)?TURNSTILE_SECRET_KEY[[:space:]]*=[[:space:]]*["'"'"']?[123]x0{31}AA["'"'"']?[[:space:]]*$' "$1"
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
      --skip-mysql) PULAR_MYSQL=true; shift ;;
      --skip-build) PULAR_BUILD=true; shift ;;
      --remove-inputs) REMOVER_ENTRADAS=true; shift ;;
      --apenas-banco) APENAS_BANCO=true; shift ;;
      -h | --help) uso; exit 0 ;;
      *) falhar "Opcao desconhecida: $1 (veja --help)." ;;
    esac
  done
  validar_argumentos
}

validar_argumentos() {
  $APENAS_BANCO || [[ -n "$ARQ_ENV" ]] || falhar "Informe --env ARQUIVO (veja --help)."
  [[ "$SITE_HOST" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ ]] || falhar "--site-host invalido."
  [[ "$SITE_IP" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || falhar "--site-ip invalido (IPv4)."
  [[ "$HTTPS_PORT" =~ ^[0-9]{1,5}$ ]] && ((HTTPS_PORT >= 1 && HTTPS_PORT <= 65535)) ||
    falhar "--https-port invalida."
  [[ "$TURNSTILE_SITE_KEY" =~ ^[A-Za-z0-9_-]*$ ]] || falhar "--turnstile-site-key invalida."
  [[ "$JENKINS_URL" =~ ^https?://[^[:space:]]+$ ]] || falhar "--jenkins-url invalida."
}

# -----------------------------------------------------------------------------
# [1] Pre-checagens
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
  for cmd in docker curl openssl gzip install runuser systemctl getent; do
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

checar_jenkins() {
  if systemctl cat jenkins >/dev/null 2>&1; then
    id jenkins >/dev/null 2>&1 || falhar "Servico jenkins existe, mas o usuario 'jenkins' nao."
    ok "Jenkins nativo (servico systemd 'jenkins')"
    return 0
  fi
  local imagens
  imagens=$(docker ps --format '{{.Image}}' 2>/dev/null || true)
  if [[ "$imagens" =~ (^|[[:space:]/])jenkins(/|:|[[:space:]]|$) ]]; then
    falhar "Jenkins roda em CONTAINER. Este script so suporta Jenkins nativo (systemd):
       o pipeline precisa do CLI docker + compose e do /var/run/docker.sock dentro
       do container do Jenkins. Monte o socket, instale o docker-ce-cli e o compose
       plugin na imagem e siga o passo a passo manual de docs/deploy-servidor.md."
  fi
  falhar "Jenkins nao encontrado como servico systemd 'jenkins'."
}

# Producao: MySQL 8.0 (cliente "mysql"). MariaDB: cliente "mariadb" (a partir
# do 11.x o pacote pode nao trazer mais o symlink "mysql").
detectar_mysql_bin() {
  if command -v mariadb >/dev/null 2>&1; then
    MYSQL_BIN=mariadb
  elif command -v mysql >/dev/null 2>&1; then
    MYSQL_BIN=mysql
  else
    falhar "Cliente mysql (ou mariadb) nao encontrado no servidor. O servidor do banco esta instalado?"
  fi
}

# Executa o cliente como root; a senha (se houver) vai so no ambiente do comando.
mysql_root() {
  if [[ -n "$MYSQL_ROOT_SENHA" ]]; then
    MYSQL_PWD=$MYSQL_ROOT_SENHA "$MYSQL_BIN" -uroot "$@"
  else
    "$MYSQL_BIN" -uroot "$@"
  fi
}

mysql_consulta() { mysql_root -N -B -e "$1"; }

checar_acesso_mysql() {
  if mysql_root -e 'SELECT 1' >/dev/null 2>&1; then
    ok "acesso root ao banco via socket"
    return 0
  fi
  info "Acesso root via socket falhou."
  IFS= read -rsp "    Senha do root do banco (MySQL/MariaDB): " MYSQL_ROOT_SENHA
  echo >&2
  mysql_root -e 'SELECT 1' >/dev/null 2>&1 || falhar "Nao foi possivel acessar o banco como root."
  ok "acesso root ao banco com senha"
}

detectar_sabor_e_servico() {
  local versao candidatos s
  versao=$(mysql_consulta 'SELECT VERSION()')
  MYSQL_VERSAO=$versao
  if [[ "$versao" == *MariaDB* ]]; then
    MYSQL_SABOR=mariadb
    candidatos=(mariadb mysql mysqld)
  else
    MYSQL_SABOR=mysql
    candidatos=(mysql mysqld mariadb)
  fi
  for s in "${candidatos[@]}"; do
    if systemctl cat "$s" >/dev/null 2>&1; then
      MYSQL_SERVICO=$s
      break
    fi
  done
  [[ -n "$MYSQL_SERVICO" ]] || falhar "Servico systemd do banco nao encontrado (mariadb/mysql/mysqld)."
  ok "$(nome_sabor) $versao (servico $MYSQL_SERVICO)"
}

etapa_prechecagens() {
  etapa 1 "Pre-checagens"
  checar_root_e_usuario
  checar_comandos
  checar_compose
  checar_entradas
  checar_jenkins
  detectar_mysql_bin
  checar_acesso_mysql
  detectar_sabor_e_servico
}

# --apenas-banco: so o necessario para a etapa 3 (sem --env, Docker, compose
# nem Jenkins).
etapa_prechecagens_banco() {
  etapa 1 "Pre-checagens (--apenas-banco)"
  checar_root
  detectar_mysql_bin
  checar_acesso_mysql
  detectar_sabor_e_servico
}

# -----------------------------------------------------------------------------
# [2] Jenkins no grupo docker
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

etapa_jenkins_docker() {
  etapa 2 "Usuario jenkins no grupo docker"
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

# -----------------------------------------------------------------------------
# [3] Hardening e bind-address do banco (DEPLOY-02)
#
# Dois drop-ins [mysqld] (secao lida pelo MySQL 8 e pelo MariaDB):
#   - zz-rotaperfumes-hardening.cnf: SEMPRE gravado (local_infile = 0, sql_mode
#     estrito, utf8mb4/utf8mb4_unicode_ci). Nunca reinicia o banco: se o valor
#     em execucao divergir, aplica com SET GLOBAL (nao PERSIST).
#   - zz-rotaperfumes-bind.cnf: so bind-address = 0.0.0.0, gravado so quando o
#     banco escuta apenas em loopback. E o UNICO que reinicia o servico.
# O legado zz-rotaperfumes.cnf (bind + hardening juntos) e migrado: o -bind.cnf
# e gravado ANTES de remover o legado, sem restart.
# -----------------------------------------------------------------------------
LEGADO_BACKUP=""

bind_address_atual() {
  mysql_consulta "SHOW GLOBAL VARIABLES LIKE 'bind_address'" | cut -f2
}

diretorio_dropin() {
  local d
  if [[ "$MYSQL_SABOR" == mariadb ]]; then d=/etc/mysql/mariadb.conf.d; else d=/etc/mysql/mysql.conf.d; fi
  for d in "$d" /etc/mysql/conf.d /etc/my.cnf.d; do
    if [[ -d "$d" ]]; then
      printf '%s' "$d"
      return 0
    fi
  done
  return 1
}

# Sucesso se o banco responder em ate 90s.
aguardar_mysql() {
  local limite=$((SECONDS + 90))
  while ((SECONDS < limite)); do
    mysql_root -e 'SELECT 1' >/dev/null 2>&1 && return 0
    sleep 2
  done
  return 1
}

# salvar_backup DESTINO BACKUP: copia o arquivo atual (se existir).
salvar_backup() {
  rm -f -- "$2"
  if [[ -f "$1" ]]; then cp -p -- "$1" "$2"; fi
  return 0
}

# restaurar_backup DESTINO BACKUP: volta o arquivo anterior, ou remove o
# DESTINO se ele nao existia antes.
restaurar_backup() {
  if [[ -f "$2" ]]; then
    install -m 644 -o root -g root "$2" "$1"
  else
    rm -f -- "$1"
  fi
}

# Valida a configuracao COMPLETA do servidor sem reiniciar.
#   MySQL 8:  mysqld --validate-config
#   MariaDB:  mariadbd --help --verbose (sai com erro em opcao desconhecida)
validar_config_banco() {
  local err="$TMP_DIR/validacao.err" bin rc=0 linha
  if [[ "$MYSQL_SABOR" == mysql ]]; then
    bin=$(command -v mysqld 2>/dev/null || printf '/usr/sbin/mysqld')
    "$bin" --validate-config --user=mysql >/dev/null 2>"$err" || rc=$?
  else
    bin=$(command -v mariadbd 2>/dev/null || command -v mysqld 2>/dev/null || printf '/usr/sbin/mariadbd')
    "$bin" --help --verbose --user=mysql >/dev/null 2>"$err" || rc=$?
  fi
  ((rc == 0)) && return 0
  aviso "A validacao da configuracao do $(nome_sabor) falhou (codigo $rc):"
  if [[ -s "$err" ]]; then
    while IFS= read -r linha; do aviso "  $linha"; done < <(sed -n '1,15p' "$err")
  fi
  return 1
}

# Se o legado foi removido nesta execucao, devolve-o (rollback do hardening).
restaurar_legado() {
  [[ -n "$LEGADO_BACKUP" && -f "$LEGADO_BACKUP" ]] || return 0
  install -m 644 -o root -g root "$LEGADO_BACKUP" "$1/$DROPIN_LEGADO"
  aviso "Drop-in legado $1/$DROPIN_LEGADO restaurado."
}

# migrar_dropin_legado DIR: grava o -bind.cnf (com o endereco do legado) e so
# depois remove o legado. Sem restart: o banco em execucao nao muda.
migrar_dropin_legado() {
  local dir=$1 legado="$1/$DROPIN_LEGADO" bind="$1/$DROPIN_BIND" tmp="$TMP_DIR/bind-legado.cnf" endereco
  [[ -f "$legado" ]] || return 0
  info "Drop-in legado encontrado: $legado. Migrando para $DROPIN_HARDENING + $DROPIN_BIND (sem restart)..."
  endereco=$(sed -nE 's/^[[:space:]]*bind[-_]address[[:space:]]*=[[:space:]]*([^[:space:]#]+).*/\1/p' "$legado" | tail -n 1)
  if [[ -n "$endereco" ]]; then
    conteudo_dropin_bind "$endereco" >"$tmp"
    if gravar_se_mudou "$tmp" "$bind"; then
      ok "gravado $bind (bind-address = $endereco, vindo do legado)"
    else
      ok "$bind ja estava igual"
    fi
  fi
  LEGADO_BACKUP="$TMP_DIR/legado.cnf.bkp"
  cp -p -- "$legado" "$LEGADO_BACKUP"
  rm -f -- "$legado"
  ok "removido $legado"
}

# aplicar_hardening DIR: grava o drop-in de hardening (se mudou), valida e
# aplica com SET GLOBAL o que divergir em execucao. Nunca reinicia.
aplicar_hardening() {
  local dir=$1 arq="$1/$DROPIN_HARDENING" tmp="$TMP_DIR/hardening.cnf" bkp="$TMP_DIR/hardening.cnf.bkp" vars div
  conteudo_dropin_hardening "$MYSQL_SABOR" "$MYSQL_VERSAO" >"$tmp" ||
    falhar "Sabor do banco desconhecido: '$MYSQL_SABOR'."
  salvar_backup "$arq" "$bkp"
  if gravar_se_mudou "$tmp" "$arq"; then
    if ! validar_config_banco; then
      restaurar_backup "$arq" "$bkp"
      restaurar_legado "$dir"
      falhar "Configuracao do $(nome_sabor) invalida com $arq. Arquivo anterior restaurado; $MYSQL_SERVICO NAO foi reiniciado."
    fi
    ok "gravado $arq (configuracao validada; sem restart)"
  else
    ok "$arq ja estava atualizado"
  fi
  vars=$(mysql_consulta "$(sql_variaveis_hardening)") || vars=""
  div=$(valores_divergentes "$MYSQL_SABOR" "$MYSQL_VERSAO" <<<"$vars" | tr '\n' ' ')
  div=${div% }
  if [[ -z "$div" ]]; then
    ok "valores em execucao ja conferem com o drop-in"
    return 0
  fi
  info "Em execucao diverge(m): $div. Aplicando com SET GLOBAL (sem restart; vale para conexoes novas)..."
  mysql_consulta "$(sql_set_global_hardening "$MYSQL_SABOR" "$MYSQL_VERSAO")" >/dev/null ||
    falhar "SET GLOBAL falhou. O drop-in $arq passa a valer no proximo restart do $MYSQL_SERVICO."
  ok "SET GLOBAL aplicado"
}

# gravar_dropin_bind DIR: so bind-address = 0.0.0.0. Reinicia o banco apenas
# quando o arquivo mudou; restaura e reinicia de novo se o restart falhar.
gravar_dropin_bind() {
  local arq="$1/$DROPIN_BIND" tmp="$TMP_DIR/bind.cnf" bkp="$TMP_DIR/bind.cnf.bkp"
  conteudo_dropin_bind >"$tmp"
  salvar_backup "$arq" "$bkp"
  if ! gravar_se_mudou "$tmp" "$arq"; then
    info "$arq ja estava gravado; sem restart."
    return 0
  fi
  if ! validar_config_banco; then
    restaurar_backup "$arq" "$bkp"
    falhar "Configuracao do $(nome_sabor) invalida com $arq. Arquivo anterior restaurado; $MYSQL_SERVICO NAO foi reiniciado."
  fi
  info "Gravado $arq; reiniciando $MYSQL_SERVICO..."
  if systemctl restart "$MYSQL_SERVICO" && aguardar_mysql; then
    return 0
  fi
  aviso "O $MYSQL_SERVICO nao voltou apos o restart. Restaurando $arq e reiniciando de novo..."
  restaurar_backup "$arq" "$bkp"
  systemctl restart "$MYSQL_SERVICO" || aviso "O segundo restart do $MYSQL_SERVICO tambem falhou."
  aguardar_mysql || aviso "O $(nome_sabor) ainda nao responde."
  falhar "Restart do $MYSQL_SERVICO falhou com $arq (arquivo restaurado). Veja: journalctl -u $MYSQL_SERVICO"
}

# Conferencia final: so AVISO (nada e alterado aqui).
conferir_hardening() {
  local vars div persistidas schema tabelas n
  vars=$(mysql_consulta "$(sql_variaveis_hardening)") || vars=""
  div=$(valores_divergentes "$MYSQL_SABOR" "$MYSQL_VERSAO" <<<"$vars" | tr '\n' ' ')
  div=${div% }
  if [[ -n "$div" ]]; then
    aviso "SHOW GLOBAL VARIABLES ainda diverge do drop-in em: $div."
    aviso "  Outro arquivo pode sobrescrever: grep -rnE 'local_infile|sql_mode|character_set|collation' /etc/mysql"
  else
    ok "conferido: local_infile=OFF, sql_mode estrito, utf8mb4/$COLLATION_PROJETO"
  fi
  if [[ "$MYSQL_SABOR" == mysql ]]; then
    persistidas=$(mysql_consulta "SELECT VARIABLE_NAME FROM performance_schema.persisted_variables WHERE VARIABLE_NAME IN ('local_infile','sql_mode','character_set_server','collation_server') ORDER BY VARIABLE_NAME" | tr '\n' ' ') ||
      persistidas=""
    persistidas=${persistidas% }
    if [[ -n "$persistidas" ]]; then
      aviso "SET PERSIST ativo para: $persistidas (mysqld-auto.cnf sobrescreve o drop-in no restart)."
      aviso "  Para o drop-in valer: RESET PERSIST <variavel>; (uma por vez)"
    fi
  fi
  schema=$(mysql_consulta "SELECT DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = '$DB_NOME'") || schema=""
  if [[ -z "$schema" ]]; then
    info "Schema $DB_NOME ainda nao existe (e criado com $COLLATION_PROJETO na etapa 5)."
    return 0
  fi
  if [[ "$schema" != "$COLLATION_PROJETO" ]]; then
    aviso "Schema $DB_NOME com collation $schema (esperado $COLLATION_PROJETO). O script NAO faz ALTER; abra um card."
  fi
  tabelas=$(mysql_consulta "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = '$DB_NOME' AND TABLE_TYPE = 'BASE TABLE' AND TABLE_COLLATION <> '$COLLATION_PROJETO' ORDER BY TABLE_NAME") ||
    tabelas=""
  if [[ -n "$tabelas" ]]; then
    n=$(grep -c . <<<"$tabelas")
    aviso "$n tabela(s) de $DB_NOME fora de $COLLATION_PROJETO: $(head -n 10 <<<"$tabelas" | paste -sd' ' -)$( ((n > 10)) && printf ' ...')"
    aviso "  O script NAO faz ALTER TABLE; abra um card para a conversao."
  fi
}

etapa_bind_address() {
  etapa 3 "Hardening e bind-address do $(nome_sabor)"
  local skip bind dir
  skip=$(mysql_consulta 'SELECT @@skip_networking')
  [[ "$skip" == "0" || "$skip" == "OFF" ]] || falhar "skip_networking esta ativo: o banco nao aceita TCP."
  dir=$(diretorio_dropin) || falhar "Diretorio de configuracao do banco nao encontrado."
  migrar_dropin_legado "$dir"
  aplicar_hardening "$dir"
  bind=$(bind_address_atual)
  if bind_so_loopback "$bind"; then
    info "bind-address atual: $bind (containers nao alcancam)."
    gravar_dropin_bind "$dir"
    bind=$(bind_address_atual)
    bind_so_loopback "$bind" &&
      falhar "bind-address continua '$bind'. Outro arquivo sobrescreve: grep -rn bind-address /etc/mysql"
  fi
  ok "bind-address: ${bind:-(todas as interfaces)}"
  DB_PORTA=$(mysql_consulta 'SELECT @@port')
  [[ "$DB_PORTA" =~ ^[0-9]+$ && "$DB_PORTA" != 0 ]] || falhar "Porta do banco invalida: $DB_PORTA"
  ok "porta do banco: $DB_PORTA"
  conferir_hardening
}

# -----------------------------------------------------------------------------
# [4] Firewall
# -----------------------------------------------------------------------------
ufw_ativo() {
  command -v ufw >/dev/null 2>&1 || return 1
  local status
  status=$(ufw status 2>/dev/null || true)
  [[ "${status,,}" == *"status: active"* ]]
}

# Remove e reinsere no topo: o allow da faixa docker precisa vir antes do deny.
ufw_liberar_docker() {
  local regra=(allow from "$FAIXA_DOCKER" to any port "$DB_PORTA" proto tcp)
  ufw delete "${regra[@]}" >/dev/null 2>&1 || true
  ufw insert 1 "${regra[@]}" >/dev/null 2>&1 || ufw "${regra[@]}" >/dev/null
}

etapa_firewall() {
  etapa 4 "Firewall (ufw)"
  if ufw_ativo; then
    ufw_liberar_docker
    ufw deny "$DB_PORTA/tcp" >/dev/null
    ufw allow "$HTTPS_PORT/tcp" >/dev/null
    ok "ufw: $DB_PORTA liberada so para $FAIXA_DOCKER; $HTTPS_PORT/tcp liberada"
    return 0
  fi
  aviso_destacado \
    "ufw INATIVO: com bind-address 0.0.0.0 a porta $DB_PORTA do banco fica aberta na LAN." \
    "O script NAO ativa o firewall. Sugestao (confira antes o acesso SSH):" \
    "  sudo ufw allow OpenSSH" \
    "  sudo ufw allow from $FAIXA_DOCKER to any port $DB_PORTA proto tcp" \
    "  sudo ufw deny $DB_PORTA/tcp" \
    "  sudo ufw allow $HTTPS_PORT/tcp" \
    "  sudo ufw allow 8888/tcp      # Jenkins, se acessado de outra maquina" \
    "  sudo ufw enable"
}

# -----------------------------------------------------------------------------
# [5] Banco e usuarios
# -----------------------------------------------------------------------------
usuario_db_existe() {
  local n
  n=$(mysql_consulta "SELECT COUNT(*) FROM mysql.user WHERE User='$1' AND Host='$DB_HOST_DOCKER'")
  [[ "$n" != "0" ]]
}

pedir_senha_admin() {
  local s1 s2
  info "Escolha a senha do usuario $DB_ADMIN do $(nome_sabor) (voce a digitara no Jenkins depois)."
  while true; do
    IFS= read -rsp "    Senha (min. 12 caracteres): " s1
    echo >&2
    if ((${#s1} < 12)); then
      aviso "Senha curta demais."
      continue
    fi
    IFS= read -rsp "    Repita a senha: " s2
    echo >&2
    [[ "$s1" == "$s2" ]] && break
    aviso "As senhas nao conferem. Tente de novo."
  done
  SENHA_ADMIN=$s1
}

pedir_senha_app_existente() {
  IFS= read -rsp "    Senha ATUAL do usuario $DB_APP do $(nome_sabor) (vai para o api.env): " SENHA_APP
  echo >&2
  [[ -n "$SENHA_APP" ]] || falhar "Senha vazia."
}

# Decide se as senhas serao (re)definidas. Retorno em SENHA_APP/SENHA_ADMIN.
decidir_senhas() {
  local resp=s admin_existe=false
  usuario_db_existe "$DB_ADMIN" && admin_existe=true
  if usuario_db_existe "$DB_APP" || $admin_existe; then
    info "Usuarios do rotaperfumes ja existem no banco."
    info "Redefinir as senhas exige ATUALIZAR as 2 credenciais no Jenkins (etapa 9)."
    read -rp "    Redefinir as senhas? [s/N] " resp
  fi
  if [[ "$resp" =~ ^[sS] ]]; then
    SENHA_APP=$(openssl rand -hex 24)
    pedir_senha_admin
    return 0
  fi
  if usuario_db_existe "$DB_APP"; then pedir_senha_app_existente; else SENHA_APP=$(openssl rand -hex 24); fi
  $admin_existe || pedir_senha_admin
}

etapa_banco() {
  etapa 5 "Banco $DB_NOME e usuarios"
  if $PULAR_MYSQL; then
    info "--skip-mysql: banco/usuarios nao serao alterados."
    pedir_senha_app_existente
    return 0
  fi
  decidir_senhas
  gerar_sql "$SENHA_APP" "$SENHA_ADMIN" | mysql_root
  ok "banco $DB_NOME (utf8mb4) e usuarios $DB_APP / $DB_ADMIN @'$DB_HOST_DOCKER'"
}

# -----------------------------------------------------------------------------
# [6] Dump
# -----------------------------------------------------------------------------
etapa_dump() {
  etapa 6 "Dump do banco"
  if [[ -z "$ARQ_DUMP" ]]; then
    info "Sem --dump: nada a instalar."
    return 0
  fi
  install -d -m 750 -o root -g jenkins "$DUMP_DIR"
  install -m 640 -o root -g jenkins "$ARQ_DUMP" "$DUMP_DESTINO"
  runuser -u jenkins -- test -r "$DUMP_DESTINO" || falhar "jenkins nao consegue ler $DUMP_DESTINO."
  ok "dump em $DUMP_DESTINO (root:jenkins 640)"
}

# -----------------------------------------------------------------------------
# [7] api.env
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

etapa_api_env() {
  etapa 7 "Gerando o api.env de producao"
  local tmp="$TMP_DIR/api.env" jwt
  jwt=$(openssl rand -base64 48 | tr -d '\n')
  avisar_chaves_com_dolar
  avisar_turnstile_vazio
  avisar_turnstile_teste
  {
    filtrar_env <"$ARQ_ENV"
    linhas_api_env "$DB_PORTA" "$SENHA_APP" "$jwt" "$SITE_HOST" "$SITE_IP" "$HTTPS_PORT"
  } >"$tmp" || falhar "Senha do app contem aspas simples; nao cabe no env_file."
  API_ENV_SAIDA="$USUARIO_HOME/rotaperfumes-api.env"
  ARQUIVOS_SECRETOS+=("$API_ENV_SAIDA")
  install -m 600 -o "$USUARIO" -g "$USUARIO_GRUPO" "$tmp" "$API_ENV_SAIDA"
  apagar_seguro "$tmp"
  ok "gravado $API_ENV_SAIDA (600, $USUARIO); JWT_SECRET novo"
}

# -----------------------------------------------------------------------------
# [8] Jenkins REST (job + leitura)
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
  etapa 8 "Job Jenkins $JOB_NOME"
  aguardar_jenkins
  pedir_token_jenkins
  validar_login_jenkins
  checar_plugins
  publicar_job
}

# -----------------------------------------------------------------------------
# [9] Pausa para as credenciais MANUAIS
# -----------------------------------------------------------------------------
credencial_existe() {
  jenkins_req GET "/credentials/store/system/domain/_/credential/$1/api/json?tree=id"
  [[ "$HTTP_CODE" == 200 ]]
}

instrucoes_credenciais() {
  local senha_admin="a senha que voce escolheu na etapa 5"
  $PULAR_MYSQL && senha_admin="a senha ATUAL do $DB_ADMIN"
  [[ -z "$SENHA_ADMIN" ]] && ! $PULAR_MYSQL && senha_admin="a senha ATUAL do $DB_ADMIN (nao foi alterada)"
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
         Password: $senha_admin | ID: $CRED_DB_ADMIN
    d) Apague o rotaperfumes-api.env do Windows:
         Remove-Item .\\rotaperfumes-api.env
    ---------------------------------------------------------------------
EOF
}

etapa_credenciais() {
  etapa 9 "Credenciais no Jenkins (cadastro MANUAL)"
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
# [10] Build
# -----------------------------------------------------------------------------
disparar_build() {
  local importar=false local_fila
  [[ -n "$ARQ_DUMP" ]] && importar=true
  jenkins_req POST "/job/$JOB_NOME/buildWithParameters" \
    --data-urlencode "IMPORTAR_DUMP=$importar" \
    --data-urlencode "DUMP_PATH=$DUMP_DESTINO" \
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
  etapa 10 "Primeiro build"
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
# [11] Resumo
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
  etapa 11 "Resumo"
  info "Acesso: https://$SITE_HOST:$HTTPS_PORT  (se o PC nao resolver o nome, adicione"
  info "  '$SITE_IP  $SITE_HOST' em C:\\Windows\\System32\\drivers\\etc\\hosts)"
  instrucoes_ca
  info "Turnstile: cadastre o hostname '$SITE_HOST' no site do Turnstile (Cloudflare)."
  [[ -z "$TURNSTILE_SITE_KEY" ]] && aviso "Sem --turnstile-site-key: o widget nao aparece; rode o job com TURNSTILE_SITE_KEY."
  [[ -n "$ARQ_DUMP" ]] && info "Depois de conferir os dados, apague o dump: sudo rm $DUMP_DESTINO"
  info "Proximos deploys: Jenkins > $JOB_NOME > Construir com parametros (IMPORTAR_DUMP desmarcado)."
  tratar_entradas
}

# -----------------------------------------------------------------------------
main() {
  ler_argumentos "$@"
  preparar_temporarios
  trap limpar EXIT
  trap 'exit 130' INT TERM
  if $APENAS_BANCO; then
    etapa_prechecagens_banco
    etapa_bind_address
    printf '\nConcluido (--apenas-banco).\n'
    return 0
  fi
  etapa_prechecagens
  etapa_jenkins_docker
  etapa_bind_address
  etapa_firewall
  etapa_banco
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
