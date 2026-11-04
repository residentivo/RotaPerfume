#!/bin/bash
# SEC-29: valida a CA do banco antes do deploy/import.
#
# Roda DENTRO de um container descartavel (imagem DB_CLIENT_IMAGE, que traz
# bash e openssl) com o arquivo do HOST montado read-only em $CA, como
# uid 65532 (o mesmo da API distroless): se esse uid nao conseguir ler, a API
# tambem nao conseguiria. Assim funciona com o Jenkins em container (o
# arquivo nunca passa pelo workspace) e com o Jenkins nativo.
#
#   CA_FINGERPRINTS="$(cat deploy/db-ca.fingerprint)" \
#   docker run --rm -i --user 65532:65532 --network rotaperfumes-db --ip 172.30.42.20 \
#     --mount type=bind,source=<caminho no host>,target=/etc/rotaperfumes/db-ca.pem,readonly \
#     -e DB_ALVO=rotaperfumes-mariadb -e DB_HOST_PORT=3306 -e TLS_NOME=rotaperfumes-mariadb \
#     -e CA_FINGERPRINTS --entrypoint bash "$DB_CLIENT_IMAGE" -s < deploy/validar-ca-banco.sh
#
# Entradas (ambiente):
#   CA               arquivo da CA (padrao /etc/rotaperfumes/db-ca.pem)
#   CA_FINGERPRINTS  SEC-40, OBRIGATORIO: conteudo do deploy/db-ca.fingerprint
#                    (versionado): SHA-256 do DER de cada certificado aceito,
#                    hex minusculo sem ':', um por linha (ate dois na rotacao);
#                    linhas vazias e "#..." sao ignoradas. Calcular com
#                      openssl x509 -in ca.pem -noout -fingerprint -sha256 \
#                        | cut -d= -f2 | tr -d : | tr A-F a-f
#   DB_ALVO          host/IP onde o s_client conecta (padrao: SITE_IP). Na
#                    rede rotaperfumes-db: rotaperfumes-mariadb (SEC-45)
#   SITE_IP          legado: alvo quando DB_ALVO esta vazio
#   DB_HOST_PORT     porta do banco no alvo (3306)
#   TLS_NOME         nome conferido no certificado (IP ou DNS do SAN)
# Saida: mensagens no stderr (inclui o fingerprint de cada certificado); no
# stdout, so o sha256 do arquivo (label do compose, B1). Codigo != 0 = nao
# fazer deploy nem import.
#
# Regras: so blocos CERTIFICATE (chave privada = recusa); cada certificado e
# validado (vencido = falha; aviso de 30 dias e de CA:TRUE) e precisa estar
# em CA_FINGERPRINTS (SEC-40: um certificado fora da lista = falha, mesmo
# que o resto passe); por fim openssl s_client -starttls mysql confere que a
# CA valida o certificado que o servidor serve HOJE para TLS_NOME.
# DEPLOY-08: exige handshake real (certificado do peer e "Cipher is" diferente
# de "(NONE)"), alem do rc 0 e do "Verify return code: 0 (ok)": sem handshake
# o s_client tambem imprime "Verify return code: 0 (ok)". O container precisa
# de um IP com conta no MariaDB (--ip 172.30.42.20); de outro IP o mariadbd
# manda o erro 1130 antes do greeting.
set -euo pipefail

CA="${CA:-/etc/rotaperfumes/db-ca.pem}"
DB_ALVO="${DB_ALVO:-${SITE_IP:-}}"
: "${DB_ALVO:?DB_ALVO (ou SITE_IP) obrigatorio}" "${DB_HOST_PORT:?DB_HOST_PORT obrigatorio}" "${TLS_NOME:?TLS_NOME obrigatorio}"

TRAB=$(mktemp -d)
trap 'rm -rf "$TRAB"' EXIT
falhar() { echo "CA do banco: $*" >&2; exit 1; }
msg() { echo "$*" >&2; }

if ! [[ "$TLS_NOME" =~ ^[A-Za-z0-9.-]+$ ]] || [ "$TLS_NOME" = "0.0.0.0" ]; then
  falhar "DB_TLS_SERVER_NAME invalido: '$TLS_NOME' (hostname ou IPv4 do SAN)"
fi
if ! [[ "$DB_ALVO" =~ ^[A-Za-z0-9.-]+$ ]] || [ "$DB_ALVO" = "0.0.0.0" ]; then
  falhar "DB_ALVO invalido: '$DB_ALVO' (hostname ou IPv4)"
fi
[[ "$DB_HOST_PORT" =~ ^[0-9]{1,5}$ ]] || falhar "DB_HOST_PORT invalida: '$DB_HOST_PORT'"

# SEC-40: lista de fingerprints aceitos (vazia = falha: fail closed).
declare -A ACEITOS=()
while IFS= read -r linha || [ -n "$linha" ]; do
  linha=${linha%$'\r'}
  linha=${linha#"${linha%%[![:space:]]*}"}
  linha=${linha%"${linha##*[![:space:]]}"}
  [ -z "$linha" ] && continue
  [[ "$linha" == \#* ]] && continue
  [[ "$linha" =~ ^[0-9a-f]{64}$ ]] || falhar "CA_FINGERPRINTS com linha invalida ('$linha'): esperado SHA-256 em hex minusculo sem ':' (deploy/db-ca.fingerprint)"
  ACEITOS[$linha]=1
done <<< "${CA_FINGERPRINTS:-}"
[ "${#ACEITOS[@]}" -gt 0 ] || falhar "CA_FINGERPRINTS vazio: passe o conteudo do deploy/db-ca.fingerprint (-e CA_FINGERPRINTS)"
[ "${#ACEITOS[@]}" -le 2 ] || msg "AVISO: ${#ACEITOS[@]} fingerprints aceitos; o esperado e 1 (ou 2 durante a rotacao)."

[ -r "$CA" ] || falhar "arquivo da CA nao legivel pelo uid $(id -u) (o da API): confira DB_TLS_CA_HOST_PATH e o chmod 644 NO HOST (docs/Instal_SSL.md, passo 5)"

PEM="$TRAB/ca.pem"
tr -d '\015' < "$CA" > "$PEM"

# So certificados: qualquer outro bloco PEM e recusado (uma chave privada
# nao pode ser montada na API).
OUTROS=$(grep -- '-----BEGIN' "$PEM" | grep -vx -- '-----BEGIN CERTIFICATE-----' || true)
[ -z "$OUTROS" ] || falhar "o arquivo tem bloco PEM que nao e certificado ($(printf '%s' "$OUTROS" | head -1)); deploy abortado"

awk -v d="$TRAB" '
  /^-----BEGIN CERTIFICATE-----$/ { n++; f = sprintf("%s/cert-%03d.pem", d, n) }
  f { print > f }
  /^-----END CERTIFICATE-----$/ { if (f) close(f); f = "" }
' "$PEM"
CERTS=("$TRAB"/cert-*.pem)
[ -e "${CERTS[0]}" ] || falhar "nenhum certificado PEM no arquivo"
TOTAL=${#CERTS[@]}
i=0
FORA=0
for c in "${CERTS[@]}"; do
  i=$((i + 1))
  openssl x509 -in "$c" -noout 2>/dev/null || falhar "certificado $i/$TOTAL invalido"
  msg "Certificado $i/$TOTAL:"
  openssl x509 -in "$c" -noout -subject -enddate -fingerprint -sha256 | sed 's/^/  /' >&2
  if ! openssl x509 -in "$c" -noout -checkend 0 >/dev/null; then
    falhar "certificado $i/$TOTAL VENCIDO ($(openssl x509 -in "$c" -noout -subject))"
  fi
  if ! openssl x509 -in "$c" -noout -checkend 2592000 >/dev/null; then
    msg "  AVISO: vence em menos de 30 dias - planeje a rotacao (docs/Instal_SSL.md, passo 9)."
  fi
  if ! openssl x509 -in "$c" -noout -ext basicConstraints 2>/dev/null | grep -q 'CA:TRUE'; then
    msg "  AVISO: sem CA:TRUE - confira se o arquivo e a CA (ca.pem) e nao o certificado do servidor."
  fi
  FP=$(openssl x509 -in "$c" -noout -fingerprint -sha256 | cut -d= -f2 | tr -d ':' | tr 'A-F' 'a-f')
  if [ -n "${ACEITOS[$FP]:-}" ]; then
    msg "  fingerprint $FP: na lista (deploy/db-ca.fingerprint)"
  else
    msg "  fingerprint $FP: NAO esta em deploy/db-ca.fingerprint"
    FORA=$((FORA + 1))
  fi
done
# SEC-40: todo certificado do arquivo do host precisa estar na lista.
[ "$FORA" -eq 0 ] || falhar "$FORA de $TOTAL certificado(s) do arquivo do host fora de deploy/db-ca.fingerprint (CA trocada no servidor sem commit do fingerprint, ou arquivo adulterado). Deploy abortado; a API e o banco nao foram tocados."

# N1: a CA valida o certificado que o servidor serve hoje, para TLS_NOME?
if [[ "$TLS_NOME" =~ ^[0-9.]+$ ]]; then
  VERIF=(-verify_ip "$TLS_NOME")
else
  VERIF=(-verify_hostname "$TLS_NOME" -servername "$TLS_NOME")
fi
RC=0
SAIDA=$(timeout 15 openssl s_client -starttls mysql -connect "$DB_ALVO:$DB_HOST_PORT" \
          -CAfile "$PEM" -verify_return_error "${VERIF[@]}" </dev/null 2>&1) || RC=$?
falha_verificacao() {
  printf '%s\n' "$SAIDA" | grep -iE 'verify error|verify return code|errno|connect|didn.t' | head -5 >&2 || true
  falhar "a CA NAO valida o certificado servido em $DB_ALVO:$DB_HOST_PORT para o nome $TLS_NOME (CA desatualizada, nome fora do SAN ou TLS desligado no servidor). A API e o banco nao foram tocados."
}
# O s_client -starttls mysql nao mostra o texto do erro: diante de um pacote
# de erro (0xff) no lugar do greeting ele imprime "Only MySQL protocol
# version 10 is supported".
if grep -qE 'is not allowed to connect|Only MySQL protocol version 10' <<<"$SAIDA"; then
  falhar "o MariaDB respondeu com erro no lugar do greeting em $DB_ALVO:$DB_HOST_PORT (provavel erro 1130, IP de origem sem conta): o container de validacao precisa de --ip 172.30.42.20 (conta rotaperfumes_admin). A API e o banco nao foram tocados."
fi
grep -qi 'verify error' <<<"$SAIDA" && falha_verificacao
CIFRA=$(grep -m1 -oE 'Cipher is [^ ]+' <<<"$SAIDA" | cut -d' ' -f3 || true)
SEM_PEER=false
grep -q 'no peer certificate available' <<<"$SAIDA" && SEM_PEER=true
if $SEM_PEER || [[ -z "$CIFRA" || "$CIFRA" == "(NONE)" ]]; then
  printf '%s\n' "$SAIDA" | grep -iE 'verify error|errno|connect|didn.t|Cipher is|peer certificate' | head -5 >&2 || true
  falhar "sem handshake TLS com $DB_ALVO:$DB_HOST_PORT (sem certificado do peer ou \"Cipher is ${CIFRA:-?}\"): TLS desligado, IP de origem sem conta (erro 1130; o container precisa de --ip 172.30.42.20) ou porta errada. A API e o banco nao foram tocados."
fi
if [[ "$RC" -ne 0 ]] || ! grep -q 'Verify return code: 0 (ok)' <<<"$SAIDA"; then
  falha_verificacao
fi
printf '%s\n' "$SAIDA" | grep -m1 'Verify return code' | sed 's/^ */  s_client: /' >&2
msg "  s_client: Cipher is $CIFRA"

msg "CA do banco OK ($TOTAL certificado(s))"
sha256sum "$CA" | cut -d' ' -f1
