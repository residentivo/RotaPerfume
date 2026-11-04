#!/usr/bin/env bash
# =============================================================================
# DEPLOY-07: seed INICIAL do banco de PRODUCAO (container rotaperfumes-mariadb,
# criado pelo deploy/mariadb-docker.sh). Substitui o "make db-seed", que NUNCA
# roda contra este banco (senha conhecida, -all-users, DROP via db-reset).
#
# Rodar como root NO SERVIDOR (192.168.168.106), a partir do checkout do repo:
#   sudo bash deploy/seed-producao.sh                         # pepper pedido sem eco
#   sudo bash deploy/seed-producao.sh --pepper-file /root/pepper.txt
#   sudo bash deploy/seed-producao.sh --admin-senha-prompt    # admin digita a senha
#   sudo bash deploy/seed-producao.sh --help
#
# Etapas:
#   1. confere root, comandos, container/rede, CA, senha do rotaperfumes_admin,
#      arquivos de sql/, pepper e o destino da senha do admin (nada muda antes)
#   2. estado do banco: tabela usuarios com linhas = recusa (fail closed),
#      salvo --forcar, que exige digitar o nome do banco
#   3. DDL na ordem de sql/ordem-ddl.txt (a mesma do make db-up)
#   4. sql/02_seed_admin.sql e sql/03_seed_vendedores.sql (INSERT IGNORE) e,
#      depois, todos os usuarios nao-admin INATIVOS (os 42 vendedores sao
#      reais; o admin os ativa pelo fluxo de reset de senha da aplicacao)
#   5. compila o backend/shared/cmd/resetpassword num container GO_IMAGE
#      (sem acesso ao banco nem a segredos)
#   6. roda o resetpassword -create-admin num container GO_IMAGE na rede
#      rotaperfumes-db (IP do admin), com --env-file temporario 600 (shred no
#      fim): senha gerada em arquivo (-password-out) ou digitada
#      (-password-prompt). deve_trocar_senha = 1. NAO roda -all-users: os
#      vendedores ficam com o hash placeholder (ninguem loga com ele).
#   7. confere: tabelas, admin ativo com Argon2id e troca obrigatoria, nenhum
#      nao-admin ativo
#
# Clientes (mesmo padrao do mariadb_cli do Jenkinsfile, estagio "Importar
# dump"): docker run --rm -i --network rotaperfumes-db --ip 172.30.42.20
# --cap-drop ALL, MYSQL_PWD pelo ambiente (-e MYSQL_PWD sem valor), CA do host
# montada read-only, --ssl-verify-server-cert, --local-infile=0.
#
# Imagens fixadas por digest (SEC-31): DB_CLIENT_IMAGE = IMAGEM_BASE do
# mariadb-docker.sh = DB_CLIENT_IMAGE do Jenkinsfile; GO_IMAGE = GO_IMAGE do
# Jenkinsfile (mesmas strings; o teste do TST-15/DEPLOY-07 confere).
#
# Regras: nunca "set -x"; segredos (senha do banco, pepper, senha do admin)
# nunca vao para argv, log ou tela.
# =============================================================================
set -euo pipefail
umask 077

DB_CLIENT_IMAGE="mariadb:11.4.13@sha256:d1891ede5a559f1086f55053bb8cc1569b0298338c4d72d71725a7f74f6ba620"
GO_IMAGE="golang:1.26.10-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308"

readonly CONTAINER="rotaperfumes-mariadb"
readonly REDE_NOME="rotaperfumes-db"
readonly IP_ADMIN="172.30.42.20"
readonly DB_NOME="rotaperfumes"
readonly DB_ADMIN="rotaperfumes_admin"
readonly SEGREDOS_DIR="/root/rotaperfumes-mariadb"
readonly SENHA_DB_ARQ="$SEGREDOS_DIR/$DB_ADMIN.senha"
readonly CA_HOST="/etc/mysql/ssl/ca.pem"
readonly C_CA="/etc/mysql/ssl/ca.pem"
readonly ADMIN_EMAIL="admin@rotaperfumes.com.br"
readonly PEPPER_MIN_BYTES=32

FORCAR=false
PEPPER_FILE=""
ADMIN_PROMPT=false
ADMIN_SENHA_ARQ="$SEGREDOS_DIR/admin-app.senha"

RAIZ=$(cd "$(dirname "$0")/.." && pwd -P)
readonly RAIZ
readonly SQL_DIR="$RAIZ/sql"
readonly ORDEM_DDL="$SQL_DIR/ordem-ddl.txt"
readonly SEEDS=(02_seed_admin.sql 03_seed_vendedores.sql)

usage() {
  cat <<EOF
Uso: sudo bash $0 [opcoes]

Seed INICIAL do banco de producao ($DB_NOME no container $CONTAINER, rede
$REDE_NOME). Rode depois do deploy/mariadb-docker.sh e antes do primeiro
deploy da API. Nunca use "make db-seed" contra este banco.

  --pepper-file ARQ     le o PASSWORD_PEPPER da 1a linha de ARQ (arquivo
                        regular do root, modo 600 ou 400). Sem a opcao, o
                        pepper e pedido no terminal, sem eco e com
                        confirmacao. TEM de ser o mesmo PASSWORD_PEPPER do
                        Secret file rotaperfumes-api-env (deploy/api.env), sem
                        aspas; senao o admin nao consegue logar.
  --admin-senha-prompt  o admin digita a senha inicial (resetpassword
                        -password-prompt, docker run -it; exige terminal).
                        Sem a opcao, a senha e GERADA e gravada em
                        --admin-senha-arquivo (nunca impressa).
  --admin-senha-arquivo ARQ
                        destino da senha gerada do admin (padrao
                        $ADMIN_SENHA_ARQ). Nao pode existir (o script recusa
                        antes de mudar qualquer coisa); fica com modo 600,
                        dono root. Depois do 1o login (troca obrigatoria),
                        apague com shred -u.
  --forcar              aceita rodar com a tabela usuarios ja com linhas
                        (reaplica a DDL e os seeds, troca a senha do admin e
                        INATIVA todos os usuarios nao-admin). Exige digitar o
                        nome do banco ($DB_NOME) no terminal.
  -h, --help            esta ajuda

O que o script faz (nesta ordem):
  1. confere root, docker, container $CONTAINER rodando, rede $REDE_NOME,
     CA em $CA_HOST, senha em $SENHA_DB_ARQ, sql/ordem-ddl.txt e os seeds,
     pepper ($PEPPER_MIN_BYTES bytes ou mais) e o destino da senha do admin
  2. recusa se usuarios ja tem linhas (salvo --forcar)
  3. aplica a DDL na ordem de sql/ordem-ddl.txt (a mesma do make db-up)
  4. aplica sql/02_seed_admin.sql e sql/03_seed_vendedores.sql e roda
     UPDATE usuarios SET ativo = 0 WHERE role <> 'admin'
     (os 42 vendedores sao reais e entram INATIVOS, com hash placeholder;
     o admin os ativa pelo fluxo de reset de senha da aplicacao)
  5. compila o resetpassword (backend/shared) num container $GO_IMAGE
  6. resetpassword -create-admin num container na rede $REDE_NOME
     ($IP_ADMIN, conta $DB_ADMIN, TLS verificado): admin ativo, Argon2id com
     o pepper e deve_trocar_senha = 1. NAO roda -all-users.
  7. confere o resultado

Clientes do banco: docker run --rm -i --network $REDE_NOME --ip $IP_ADMIN
--cap-drop ALL --security-opt no-new-privileges:true, MYSQL_PWD pelo ambiente,
CA montada read-only, --ssl-verify-server-cert -h $CONTAINER --local-infile=0.
O resetpassword recebe DB_* e PASSWORD_PEPPER por --env-file temporario (600,
apagado com shred no fim, inclusive em erro). Docs: docs/deploy-servidor.md §11.
EOF
}

die()   { printf 'ERRO: %s\n' "$*" >&2; exit 1; }
info()  { printf '  %s\n' "$*"; }
aviso() { printf '  AVISO: %s\n' "$*" >&2; }
exigir_valor() { [[ $# -ge 2 && -n "$2" ]] || die "a opcao $1 exige um valor (veja --help)"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --pepper-file) exigir_valor "$@"; PEPPER_FILE="$2"; shift 2 ;;
    --admin-senha-prompt) ADMIN_PROMPT=true; shift ;;
    --admin-senha-arquivo) exigir_valor "$@"; ADMIN_SENHA_ARQ="$2"; shift 2 ;;
    --forcar) FORCAR=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Opcao desconhecida: $1" >&2; usage >&2; exit 2 ;;
  esac
done

ETAPA=0
readonly TOTAL_ETAPAS=7
etapa() { ETAPA=$((ETAPA + 1)); printf '\n=== [%d/%d] %s ===\n' "$ETAPA" "$TOTAL_ETAPAS" "$1"; }

# Area de trabalho (so root) e limpeza: env-file e senha com shred, sempre.
TRAB=""
ENV_ARQ=""
limpar() {
  [[ -n "$ENV_ARQ" && -f "$ENV_ARQ" ]] && shred -u "$ENV_ARQ" 2>/dev/null
  if [[ -n "$TRAB" && -d "$TRAB" ]]; then
    [[ -f "$TRAB/saida/admin.senha" ]] && shred -u "$TRAB/saida/admin.senha" 2>/dev/null
    rm -rf "$TRAB"
  fi
  return 0
}
trap limpar EXIT

# tem_tty (BUG-26): ha terminal de controle? [[ -r /dev/tty ]] usa access()
# e /dev/tty e 0666, entao passa mesmo sem terminal; so o open() responde
# (ENXIO sem terminal de controle).
tem_tty() { { : </dev/tty; } 2>/dev/null; }

# arquivo_secreto_ok ARQ: regular, dono root, modo 600 ou 400.
arquivo_secreto_ok() {
  [[ -f "$1" && ! -L "$1" ]] || return 1
  [[ "$(stat -c '%u' "$1")" == 0 ]] || return 1
  case "$(stat -c '%a' "$1")" in 600|400) return 0 ;; *) return 1 ;; esac
}

# ---------------------------------------------------------------------------
etapa "Pre-requisitos"
[[ $EUID -eq 0 ]] || die "rode como root: sudo bash $0"
# BUG-26: sem terminal, falha ja aqui (antes do Docker e do banco) quando a
# execucao vai precisar dele (pepper sem --pepper-file; confirmacao do --forcar).
if [[ -z "$PEPPER_FILE" ]]; then
  tem_tty || die "sem terminal para pedir o PASSWORD_PEPPER: use --pepper-file ARQ"
fi
if $FORCAR; then
  tem_tty || die "--forcar exige um terminal para a confirmacao"
fi
for cmd in docker tar shred stat install mktemp grep; do
  command -v "$cmd" >/dev/null || die "comando '$cmd' nao encontrado"
done
docker info >/dev/null 2>&1 || die "o daemon do Docker nao responde"
[[ "$DB_CLIENT_IMAGE" =~ ^mariadb:[0-9.]+@sha256:[0-9a-f]{64}$ ]] || die "DB_CLIENT_IMAGE invalida: $DB_CLIENT_IMAGE"
[[ "$GO_IMAGE" =~ ^golang:[0-9.]+-alpine@sha256:[0-9a-f]{64}$ ]] || die "GO_IMAGE invalida: $GO_IMAGE"

[[ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null || true)" == "true" ]] \
  || die "o container $CONTAINER nao esta rodando: rode antes sudo bash deploy/mariadb-docker.sh"
docker network inspect "$REDE_NOME" >/dev/null 2>&1 || die "a rede docker $REDE_NOME nao existe (deploy/mariadb-docker.sh)"
[[ -r "$CA_HOST" && -s "$CA_HOST" ]] || die "CA do banco nao encontrada em $CA_HOST (deploy/mariadb-docker.sh)"
arquivo_secreto_ok "$SENHA_DB_ARQ" || die "$SENHA_DB_ARQ ausente ou com dono/modo errado (esperado root, 600)"
SENHA_DB=$(<"$SENHA_DB_ARQ")
[[ "$SENHA_DB" =~ ^[A-Za-z0-9]{20,}$ ]] || die "senha em $SENHA_DB_ARQ fora do formato [A-Za-z0-9]{20,}"

# sql/ordem-ddl.txt: so nomes de DDL de sql/, todos existentes.
[[ -r "$ORDEM_DDL" ]] || die "$ORDEM_DDL nao encontrado (rode a partir do checkout do repositorio)"
DDLS=()
while IFS= read -r linha || [[ -n "$linha" ]]; do
  linha=${linha%$'\r'}
  linha=${linha#"${linha%%[![:space:]]*}"}
  linha=${linha%"${linha##*[![:space:]]}"}
  [[ -z "$linha" || "$linha" == \#* ]] && continue
  [[ "$linha" =~ ^[0-9]{2}_ddl_[a-z_]+\.sql$ ]] || die "linha invalida em $ORDEM_DDL: '$linha' (esperado NN_ddl_nome.sql)"
  [[ -r "$SQL_DIR/$linha" ]] || die "$SQL_DIR/$linha (listado em $ORDEM_DDL) nao encontrado"
  DDLS+=("$linha")
done < "$ORDEM_DDL"
(( ${#DDLS[@]} > 0 )) || die "$ORDEM_DDL sem nenhum arquivo"
for f in "${SEEDS[@]}"; do
  [[ -r "$SQL_DIR/$f" ]] || die "$SQL_DIR/$f nao encontrado"
done
[[ -r "$RAIZ/backend/shared/go.mod" ]] || die "$RAIZ/backend/shared/go.mod nao encontrado"

# Destino/forma da senha do admin.
if $ADMIN_PROMPT; then
  [[ -t 0 && -t 1 ]] || die "--admin-senha-prompt exige um terminal interativo"
else
  [[ "$ADMIN_SENHA_ARQ" == /* ]] || die "--admin-senha-arquivo precisa de caminho absoluto: $ADMIN_SENHA_ARQ"
  [[ ! -e "$ADMIN_SENHA_ARQ" && ! -L "$ADMIN_SENHA_ARQ" ]] \
    || die "$ADMIN_SENHA_ARQ ja existe: confira se a senha anterior ainda e necessaria, apague com shred -u ou use --admin-senha-arquivo"
  [[ -d "$(dirname "$ADMIN_SENHA_ARQ")" ]] || die "diretorio de $ADMIN_SENHA_ARQ nao existe"
fi

# PASSWORD_PEPPER: arquivo (600/400 do root) ou terminal sem eco.
PEPPER=""
if [[ -n "$PEPPER_FILE" ]]; then
  arquivo_secreto_ok "$PEPPER_FILE" || die "--pepper-file $PEPPER_FILE: precisa ser arquivo regular do root com modo 600 ou 400"
  IFS= read -r PEPPER < "$PEPPER_FILE" || [[ -n "$PEPPER" ]] || die "--pepper-file $PEPPER_FILE vazio"
else
  tem_tty || die "sem terminal para pedir o PASSWORD_PEPPER: use --pepper-file ARQ"
  IFS= read -rs -p "PASSWORD_PEPPER (o mesmo do rotaperfumes-api-env; nao aparece na tela): " PEPPER < /dev/tty \
    || die "leitura do PASSWORD_PEPPER no terminal falhou (use --pepper-file ARQ); nada foi alterado"
  printf '\n' >&2
  IFS= read -rs -p "Confirme o PASSWORD_PEPPER: " PEPPER2 < /dev/tty \
    || die "leitura da confirmacao do PASSWORD_PEPPER falhou; nada foi alterado"
  printf '\n' >&2
  [[ "$PEPPER" == "$PEPPER2" ]] || die "os dois PASSWORD_PEPPER digitados nao conferem"
  unset PEPPER2
fi
PEPPER=${PEPPER%$'\r'}
(( ${#PEPPER} >= PEPPER_MIN_BYTES )) || die "PASSWORD_PEPPER com menos de $PEPPER_MIN_BYTES caracteres (a API recusaria)"
[[ "$PEPPER" != *$'\n'* ]] || die "PASSWORD_PEPPER com quebra de linha"
case "$PEPPER" in
  \"*\"|\'*\') aviso "o PASSWORD_PEPPER informado comeca e termina com aspas: confira se elas fazem parte do valor no rotaperfumes-api-env." ;;
esac
info "Pre-requisitos OK (container $CONTAINER, rede $REDE_NOME, ${#DDLS[@]} DDL, pepper lido; nada foi impresso)."

TRAB=$(mktemp -d /root/rotaperfumes-seed.XXXXXX)
install -d -m 700 -o root -g root "$TRAB/bin" "$TRAB/saida"

# Cliente mariadb descartavel na rede interna (padrao do mariadb_cli do Jenkinsfile).
mariadb_cli() {
  MYSQL_PWD="$SENHA_DB" docker run --rm -i -e MYSQL_PWD \
    --network "$REDE_NOME" --ip "$IP_ADMIN" \
    --cap-drop ALL --security-opt no-new-privileges:true \
    --mount "type=bind,source=$CA_HOST,target=$C_CA,readonly" \
    --entrypoint mariadb "$DB_CLIENT_IMAGE" \
    --local-infile=0 -h "$CONTAINER" -P 3306 -u "$DB_ADMIN" \
    --ssl-ca="$C_CA" --ssl-verify-server-cert \
    --default-character-set=utf8mb4 "$@"
}
consulta() { mariadb_cli -N -B "$DB_NOME" -e "$1"; }

# ---------------------------------------------------------------------------
etapa "Estado do banco $DB_NOME"
consulta 'SELECT 1' >/dev/null \
  || die "sem conexao TLS verificada em $CONTAINER:3306 como $DB_ADMIN@$IP_ADMIN (confira o IP $IP_ADMIN livre e o deploy/mariadb-docker.sh)"
TEM_USUARIOS=$(consulta "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = '$DB_NOME' AND TABLE_NAME = 'usuarios'")
LINHAS=0
if [[ "$TEM_USUARIOS" == 1 ]]; then
  LINHAS=$(consulta 'SELECT COUNT(*) FROM usuarios')
fi
[[ "$LINHAS" =~ ^[0-9]+$ ]] || die "resposta inesperada ao contar usuarios: '$LINHAS'"
if (( LINHAS > 0 )); then
  $FORCAR || die "a tabela usuarios ja tem $LINHAS linha(s): este banco ja foi semeado. Nada foi alterado. (--forcar reaplica e INATIVA todos os nao-admin)"
  tem_tty || die "--forcar exige um terminal para a confirmacao"
  aviso "--forcar: usuarios tem $LINHAS linha(s). A DDL e os seeds serao reaplicados, a senha do admin trocada e TODOS os usuarios nao-admin INATIVADOS."
  IFS= read -r -p "Digite o nome do banco ($DB_NOME) para confirmar: " RESPOSTA < /dev/tty \
    || die "--forcar: leitura da confirmacao falhou; nada foi alterado"
  [[ "$RESPOSTA" == "$DB_NOME" ]] || die "confirmacao nao confere; nada foi alterado"
else
  info "Banco sem usuarios (instalacao nova): seguindo."
fi

# ---------------------------------------------------------------------------
etapa "DDL (sql/ordem-ddl.txt)"
for f in "${DDLS[@]}"; do
  info "Aplicando sql/$f"
  mariadb_cli "$DB_NOME" < "$SQL_DIR/$f" >/dev/null || die "falhou a DDL sql/$f"
done

# ---------------------------------------------------------------------------
etapa "Seeds (admin e vendedores) e vendedores inativos"
for f in "${SEEDS[@]}"; do
  info "Aplicando sql/$f"
  mariadb_cli "$DB_NOME" < "$SQL_DIR/$f" >/dev/null || die "falhou o seed sql/$f"
done
# Os 42 vendedores sao reais: entram INATIVOS (o admin ativa pelo reset).
mariadb_cli "$DB_NOME" -e "UPDATE usuarios SET ativo = 0 WHERE role <> 'admin'" \
  || die "falhou a inativacao dos usuarios nao-admin"
info "Usuarios nao-admin inativos: $(consulta "SELECT COUNT(*) FROM usuarios WHERE role <> 'admin' AND ativo = 0")."

# ---------------------------------------------------------------------------
etapa "Compilando o resetpassword ($GO_IMAGE)"
# Sem rede do banco e sem segredos: so o codigo de backend/shared por stdin
# (sem .env). Rede padrao (baixa os modulos do proxy do Go).
tar -C "$RAIZ" -cf - --exclude='.env' --exclude='.env.*' --exclude='*.env' \
    --exclude='coverage*.txt' --exclude='coverage.html' backend/shared \
  | docker run --rm -i --security-opt no-new-privileges:true \
      -e CGO_ENABLED=0 -e GOFLAGS=-mod=readonly \
      --mount "type=bind,source=$TRAB/bin,target=/out" \
      "$GO_IMAGE" sh -ec '
        mkdir -p /src && tar -xf - -C /src
        cd /src/backend/shared && go build -trimpath -o /out/resetpassword ./cmd/resetpassword
      ' || die "falhou a compilacao do resetpassword"
[[ -x "$TRAB/bin/resetpassword" ]] || die "binario do resetpassword nao foi gerado"
info "resetpassword compilado."

# ---------------------------------------------------------------------------
etapa "Admin ($ADMIN_EMAIL)"
ENV_ARQ="$TRAB/resetpassword.env"
{
  printf 'DB_HOST=%s\n' "$CONTAINER"
  printf 'DB_PORT=3306\n'
  printf 'DB_USUARIO=%s\n' "$DB_ADMIN"
  printf 'DB_SENHA=%s\n' "$SENHA_DB"
  printf 'DB_NAME=%s\n' "$DB_NOME"
  printf 'DB_TLS_CA=%s\n' "$C_CA"
  printf 'DB_TLS_SERVER_NAME=%s\n' "$CONTAINER"
  printf 'PASSWORD_PEPPER=%s\n' "$PEPPER"
} > "$ENV_ARQ"
chmod 600 "$ENV_ARQ"
unset PEPPER

RP_ARGS=(
  --rm --network "$REDE_NOME" --ip "$IP_ADMIN"
  --cap-drop ALL --security-opt no-new-privileges:true --read-only
  --env-file "$ENV_ARQ"
  --mount "type=bind,source=$CA_HOST,target=$C_CA,readonly"
  --mount "type=bind,source=$TRAB/bin/resetpassword,target=/usr/local/bin/resetpassword,readonly"
  -w / --entrypoint /usr/local/bin/resetpassword
)
if $ADMIN_PROMPT; then
  docker run -it "${RP_ARGS[@]}" "$GO_IMAGE" -create-admin -password-prompt \
    || die "o resetpassword -create-admin falhou"
else
  docker run -i "${RP_ARGS[@]}" \
    --mount "type=bind,source=$TRAB/saida,target=/saida" \
    "$GO_IMAGE" -create-admin -password-out /saida/admin.senha </dev/null \
    || die "o resetpassword -create-admin falhou"
  [[ -s "$TRAB/saida/admin.senha" ]] || die "o resetpassword nao gravou a senha do admin"
  [[ ! -e "$ADMIN_SENHA_ARQ" ]] || die "$ADMIN_SENHA_ARQ apareceu durante a execucao; a senha nova ficou so no banco: rode de novo com --forcar"
  install -m 600 -o root -g root "$TRAB/saida/admin.senha" "$ADMIN_SENHA_ARQ"
fi
shred -u "$ENV_ARQ"
ENV_ARQ=""

# ---------------------------------------------------------------------------
etapa "Conferencia"
N_TABELAS=$(consulta "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = '$DB_NOME'")
ADMIN_OK=$(mariadb_cli -N -B "$DB_NOME" <<'SQL'
SELECT CONCAT(ativo, ' ', deve_trocar_senha, ' ', password_hash LIKE CONCAT(CHAR(36), 'argon2id', CHAR(36), '%'))
FROM usuarios WHERE email = 'admin@rotaperfumes.com.br';
SQL
)
ATIVOS_NAO_ADMIN=$(consulta "SELECT COUNT(*) FROM usuarios WHERE role <> 'admin' AND ativo <> 0")
[[ "$ADMIN_OK" == "1 1 1" ]] || die "admin fora do esperado (ativo, deve_trocar_senha, argon2id) = '$ADMIN_OK'"
[[ "$ATIVOS_NAO_ADMIN" == 0 ]] || die "$ATIVOS_NAO_ADMIN usuario(s) nao-admin ativo(s); esperado 0"
info "Tabelas em $DB_NOME: $N_TABELAS. Admin ativo, Argon2id, troca obrigatoria no 1o login: OK."
info "Usuarios nao-admin ativos: 0 (o admin ativa cada vendedor pelo reset de senha)."

cat <<EOF

Pronto. Seed inicial aplicado em $DB_NOME ($CONTAINER).
EOF
if $ADMIN_PROMPT; then
  info "Senha inicial do admin: a digitada (troca obrigatoria no 1o login)."
else
  cat <<EOF
Senha inicial do admin ($ADMIN_EMAIL), gerada e NAO impressa:
  $ADMIN_SENHA_ARQ (600, so root): sudo cat $ADMIN_SENHA_ARQ
  Faca o 1o login (troca obrigatoria) e depois: sudo shred -u $ADMIN_SENHA_ARQ
EOF
fi
cat <<EOF
Proximos passos: confira o deploy/db-ca.fingerprint (so a CA de producao),
o Secret file rotaperfumes-api-env (mesmo PASSWORD_PEPPER) e rode o pipeline
da main (docs/deploy-servidor.md §11).
EOF
