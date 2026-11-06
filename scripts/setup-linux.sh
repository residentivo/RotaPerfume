#!/usr/bin/env bash
# =============================================================================
# SETUP-01: instala/confere as ferramentas do ambiente de desenvolvimento (Linux).
#
# Alvo: Debian/Ubuntu e derivados (apt). Em outras distros só confere e avisa
# o que falta (sem instalar pacotes do sistema).
#
# Versões: Go = diretiva "go" dos go.mod; Node = frontend/.nvmrc; demais em
# scripts/versoes.conf. Idempotente: pula o que já está na versão certa.
#
# Uso: scripts/setup-linux.sh [--check] [--deps] [opções]   (ver --help)
# O script não pede senha: só o "sudo" pede (a senha do seu usuário no sistema),
# e só nos passos que mexem no sistema (apt, /usr/local). Use --sem-sudo para pular.
# =============================================================================
set -uo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARQ_VERSOES="$RAIZ/scripts/versoes.conf"
ARQ_NVMRC="$RAIZ/frontend/.nvmrc"
GO_MODS=("$RAIZ/backend/rotaperfumes-api/go.mod" "$RAIZ/backend/shared/go.mod")

MODO=instalar
DEPS=0
SEM_SUDO=0
GO_DESTINO="$HOME/.local/go"
POSTMAN_VIA=auto
MARIADB_OFICIAL=1
AJUSTAR_GOTOOLCHAIN=1
AJUSTAR_PERFIL=1

MARCA_INI="# >>> RotaPerfume SETUP-01 (scripts/setup-linux.sh) >>>"
MARCA_FIM="# <<< RotaPerfume SETUP-01 <<<"

uso() {
	cat <<'EOF'
Uso: scripts/setup-linux.sh [opções]

  --check               Só confere as versões (não instala nem altera nada).
  --deps                No fim, baixa as dependências do projeto
                        (go mod download nos módulos e make frontend-deps).
  --go-dir DIR          Onde instalar o Go (padrão: $HOME/.local/go, sem sudo).
  --go-sistema          Instala o Go em /usr/local/go (usa sudo).
  --sem-sudo            Não usa sudo: passos de sistema (apt) viram FALTA.
  --mariadb-distro      Instala o mariadb-client do repositório da distro
                        (padrão: repositório oficial do MariaDB na série de versoes.conf).
  --postman-script      Instala o Postman CLI pelo script oficial (dl-cli.pstmn.io)
                        em vez do pacote npm oficial "postman-cli".
  --manter-gotoolchain  Não grava GOTOOLCHAIN=local no "go env".
  --sem-perfil          Não altera ~/.profile e ~/.bashrc (PATH do Go).
  -h, --help            Mostra esta ajuda.

Saída: 0 se não faltar nada; 1 se algum item ficou como FALTA.
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
	--check) MODO=check ;;
	--deps) DEPS=1 ;;
	--go-dir)
		[ $# -ge 2 ] || { echo "--go-dir precisa de um diretório" >&2; exit 2; }
		GO_DESTINO="$2"; shift ;;
	--go-sistema) GO_DESTINO=/usr/local/go ;;
	--sem-sudo) SEM_SUDO=1 ;;
	--mariadb-distro) MARIADB_OFICIAL=0 ;;
	--postman-script) POSTMAN_VIA=script ;;
	--manter-gotoolchain) AJUSTAR_GOTOOLCHAIN=0 ;;
	--sem-perfil) AJUSTAR_PERFIL=0 ;;
	-h | --help) uso; exit 0 ;;
	*) echo "Opção desconhecida: $1" >&2; uso >&2; exit 2 ;;
	esac
	shift
done

# ----------------------------------------------------------------------------
# Saída e resumo
# ----------------------------------------------------------------------------
if [ -t 1 ]; then
	C_OK=$'\033[32m'; C_AV=$'\033[33m'; C_FA=$'\033[31m'; C_TI=$'\033[36m'; C_NO=$'\033[0m'
else
	C_OK=; C_AV=; C_FA=; C_TI=; C_NO=
fi
titulo() { printf '\n%s== %s ==%s\n' "$C_TI" "$*" "$C_NO"; }
info() { printf '   %s\n' "$*"; }
aviso() { printf '   %sAVISO:%s %s\n' "$C_AV" "$C_NO" "$*"; }

RES_NOME=(); RES_STATUS=(); RES_DETALHE=()
registrar() { RES_NOME+=("$1"); RES_STATUS+=("$2"); RES_DETALHE+=("$3"); }

resumo() {
	titulo "Resumo ($MODO)"
	local i cor faltou=0
	for i in "${!RES_NOME[@]}"; do
		case "${RES_STATUS[$i]}" in
		OK) cor=$C_OK ;;
		AVISO) cor=$C_AV ;;
		*) cor=$C_FA; faltou=1 ;;
		esac
		printf '  %-14s %s%-5s%s  %s\n' "${RES_NOME[$i]}" "$cor" "${RES_STATUS[$i]}" "$C_NO" "${RES_DETALHE[$i]}"
	done
	echo
	if [ "$faltou" -eq 1 ]; then
		if [ "$MODO" = check ]; then
			echo "Há itens em FALTA. Rode sem --check para instalar (ou make setup)."
		else
			echo "Há itens em FALTA: veja as mensagens acima."
		fi
	fi
	if [ "$MODO" = instalar ] && [ "${PERFIL_ALTERADO:-0}" -eq 1 ]; then
		echo "O PATH foi alterado em ~/.profile e ~/.bashrc: abra um terminal novo (ou rode 'source ~/.bashrc')."
	fi
	return "$faltou"
}

# ----------------------------------------------------------------------------
# Utilitários
# ----------------------------------------------------------------------------
# ver_ge A B: verdadeiro se a versão A >= B.
ver_ge() { [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]; }
# Primeira versão numérica (x.y[.z]) do texto.
extrair_versao() { grep -oE '[0-9]+(\.[0-9]+)+' | head -n1; }
tem() { command -v "$1" >/dev/null 2>&1; }

carregar_versoes() {
	[ -f "$ARQ_VERSOES" ] || { echo "Não encontrei $ARQ_VERSOES" >&2; exit 2; }
	local linha chave valor
	while IFS= read -r linha || [ -n "$linha" ]; do
		linha="${linha%$'\r'}"
		case "$linha" in '' | '#'*) continue ;; esac
		chave="${linha%%=*}"; valor="${linha#*=}"
		[[ "$chave" =~ ^[A-Z_][A-Z0-9_]*$ ]] || continue
		printf -v "$chave" '%s' "$valor"
	done <"$ARQ_VERSOES"
}

SUDO_AVISADO=0
como_root() {
	if [ "$(id -u)" -eq 0 ]; then "$@"; return; fi
	if [ "$SEM_SUDO" -eq 1 ] || ! tem sudo; then
		aviso "precisa de root para: $*  (pulado: --sem-sudo ou sudo ausente)"
		return 1
	fi
	if [ "$SUDO_AVISADO" -eq 0 ]; then
		aviso "os próximos passos usam sudo; se ele pedir, é a senha do seu usuário no sistema (o script não lê nem guarda senha)."
		SUDO_AVISADO=1
	fi
	sudo "$@"
}

APT=0; APT_ATUALIZADO=0; DISTRO_ID=; DISTRO_LIKE=
detectar_distro() {
	if [ -r /etc/os-release ]; then
		# shellcheck disable=SC1091
		. /etc/os-release
		DISTRO_ID="${ID:-}"; DISTRO_LIKE="${ID_LIKE:-}"
		info "Distro: ${PRETTY_NAME:-$DISTRO_ID}"
	fi
	case " $DISTRO_ID $DISTRO_LIKE " in
	*" debian "* | *" ubuntu "*) tem apt-get && APT=1 ;;
	esac
	if [ "$APT" -eq 0 ]; then
		aviso "distro sem apt (ou não Debian/Ubuntu): os pacotes do sistema (git, make, curl, openssl, mariadb-client) não serão instalados; instale-os pelo gerenciador da distro. Go, Node (nvm), govulncheck e Postman CLI continuam funcionando."
	fi
}

apt_instalar() {
	[ "$APT" -eq 1 ] || return 1
	if [ "$APT_ATUALIZADO" -eq 0 ]; then
		como_root apt-get update -qq || return 1
		APT_ATUALIZADO=1
	fi
	como_root env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$@"
}

baixar() { curl -fsSL --proto '=https' --tlsv1.2 -o "$2" "$1"; }

ARQ_GO=
case "$(uname -m)" in
x86_64 | amd64) ARQ_GO=amd64 ;;
aarch64 | arm64) ARQ_GO=arm64 ;;
armv6l | armv7l) ARQ_GO=armv6l ;;
*) ARQ_GO= ;;
esac

# ----------------------------------------------------------------------------
# Ferramentas básicas (apt): curl, tar, git, make, openssl
# ----------------------------------------------------------------------------
ferramenta_basica() {
	local nome="$1" pacote="$2" minimo="$3" cmdver="$4" v
	if tem "$nome"; then
		v="$($cmdver 2>&1 | extrair_versao)"
		if [ -z "$minimo" ] || ver_ge "${v:-0}" "$minimo"; then
			registrar "$nome" OK "${v:-?}"
			return
		fi
		if [ "$MODO" = check ]; then
			registrar "$nome" AVISO "$v (mínimo $minimo)"; return
		fi
	elif [ "$MODO" = check ]; then
		registrar "$nome" FALTA "não encontrado (apt: $pacote)"; return
	fi
	info "Instalando $pacote..."
	if apt_instalar "$pacote" && tem "$nome"; then
		v="$($cmdver 2>&1 | extrair_versao)"
		registrar "$nome" OK "${v:-?} (instalado)"
	else
		registrar "$nome" FALTA "instale o pacote $pacote"
	fi
}

etapa_basicas() {
	titulo "Ferramentas básicas"
	ferramenta_basica curl curl "" "curl --version"
	ferramenta_basica tar tar "" "tar --version"
	ferramenta_basica git git "$GIT_MIN" "git --version"
	ferramenta_basica make make "$MAKE_MIN" "make --version"
	ferramenta_basica openssl openssl "$OPENSSL_MIN" "openssl version"
}

# ----------------------------------------------------------------------------
# Go (tarball oficial na versão do go.mod) + GOTOOLCHAIN + covdata
# ----------------------------------------------------------------------------
GO_VERSAO=
GO_BIN=
versao_go_mod() {
	local f v anterior=
	for f in "${GO_MODS[@]}"; do
		v="$(awk '$1 == "go" { print $2; exit }' "$f")"
		if [ -n "$anterior" ] && [ "$v" != "$anterior" ]; then
			aviso "os go.mod pedem versões diferentes ($anterior e $v): usando a maior."
			ver_ge "$anterior" "$v" && v="$anterior"
		fi
		anterior="$v"
	done
	GO_VERSAO="$anterior"
}

# Versão do Go de um binário, SEM troca automática de toolchain.
versao_go_de() { GOTOOLCHAIN=local "$1" env GOVERSION 2>/dev/null | sed 's/^go//'; }

go_aceitavel() {
	local v="$1"
	[ -n "$v" ] || return 1
	[ "$v" = "$GO_VERSAO" ] && return 0
	[ "${GO_ACEITAR_PATCH_MAIOR:-0}" = 1 ] || return 1
	# Mesma série (x.y) e patch >= o do go.mod.
	[ "${v%.*}" = "${GO_VERSAO%.*}" ] && ver_ge "$v" "$GO_VERSAO"
}

procurar_go() {
	local c v vistos=" "
	for c in "$GO_DESTINO/bin/go" /usr/local/go/bin/go "$HOME/.local/go/bin/go" "$HOME/sdk/go$GO_VERSAO/bin/go" "$(command -v go 2>/dev/null)"; do
		# shellcheck disable=SC2015 # o "|| C" deve rodar se A OU B falhar (é o pretendido)
		[ -n "$c" ] && [ -x "$c" ] || continue
		c="$(readlink -f "$c")"
		case "$vistos" in *" $c "*) continue ;; esac
		vistos+="$c "
		v="$(versao_go_de "$c")"
		info "Go local encontrado: $c ($v)"
		if go_aceitavel "$v"; then GO_BIN="$c"; return 0; fi
	done
	return 1
}

instalar_go() {
	[ -n "$ARQ_GO" ] || { aviso "arquitetura $(uname -m) sem Go oficial mapeado"; return 1; }
	local arquivo="go${GO_VERSAO}.linux-${ARQ_GO}.tar.gz" tmp esperado obtido pai
	tmp="$(mktemp -d)" || return 1
	info "Baixando $arquivo de https://go.dev/dl/ ..."
	if ! baixar "https://dl.google.com/go/$arquivo" "$tmp/$arquivo" ||
		! baixar "https://dl.google.com/go/$arquivo.sha256" "$tmp/sha256"; then
		aviso "falha no download do Go"; rm -rf "$tmp"; return 1
	fi
	esperado="$(tr -d ' \r\n' <"$tmp/sha256")"
	obtido="$(sha256sum "$tmp/$arquivo" | awk '{print $1}')"
	if [ -z "$esperado" ] || [ "$esperado" != "$obtido" ]; then
		aviso "SHA-256 do tarball não confere (esperado $esperado, obtido $obtido)"; rm -rf "$tmp"; return 1
	fi
	# shellcheck disable=SC2015 # o "|| C" deve rodar se A OU B falhar (é o pretendido)
	mkdir -p "$tmp/x" && tar -C "$tmp/x" -xzf "$tmp/$arquivo" || { rm -rf "$tmp"; return 1; }

	if [ -e "$GO_DESTINO" ] && ! grep -q '^go' "$GO_DESTINO/VERSION" 2>/dev/null; then
		aviso "$GO_DESTINO existe e não parece uma instalação do Go: não vou sobrescrever"; rm -rf "$tmp"; return 1
	fi
	pai="$(dirname "$GO_DESTINO")"
	local exec_=()
	if [ -w "$pai" ] || { [ ! -e "$pai" ] && mkdir -p "$pai" 2>/dev/null; }; then
		exec_=()
	else
		exec_=(como_root)
	fi
	# Recomendação oficial: remover a instalação anterior antes de extrair.
	# shellcheck disable=SC2015 # o "|| C" deve rodar se A OU B falhar (é o pretendido)
	"${exec_[@]}" rm -rf "$GO_DESTINO" &&
		"${exec_[@]}" mkdir -p "$pai" &&
		"${exec_[@]}" mv "$tmp/x/go" "$GO_DESTINO" || { rm -rf "$tmp"; return 1; }
	rm -rf "$tmp"
	GO_BIN="$GO_DESTINO/bin/go"
	info "Go $GO_VERSAO instalado em $GO_DESTINO"
}

PERFIL_ALTERADO=0
gravar_perfil() {
	# Bloco único e idempotente com o PATH do Go e do GOPATH/bin.
	local linha="$1" f tmp
	for f in "$HOME/.profile" "$HOME/.bashrc"; do
		[ -f "$f" ] || touch "$f"
		tmp="$(mktemp)"
		awk -v ini="$MARCA_INI" -v fim="$MARCA_FIM" '
			$0 == ini { pula = 1; next }
			$0 == fim { pula = 0; next }
			!pula { print }' "$f" >"$tmp"
		printf '%s\n%s\n%s\n' "$MARCA_INI" "$linha" "$MARCA_FIM" >>"$tmp"
		if ! cmp -s "$tmp" "$f"; then
			cat "$tmp" >"$f"
			PERFIL_ALTERADO=1
			info "PATH atualizado em $f"
		fi
		rm -f "$tmp"
	done
}

smoke_covdata() {
	# Exercita o covdata como o "make test": -coverpkg com um pacote sem testes.
	local go="$1" d saida
	d="$(mktemp -d)" || return 1
	mkdir -p "$d/a" "$d/t"
	printf 'module smoke\n\ngo %s\n' "$GO_VERSAO" >"$d/go.mod"
	printf 'package a\n\nfunc F() int { return 1 }\n' >"$d/a/a.go"
	printf 'package t_test\n\nimport (\n\t"testing"\n\n\t"smoke/a"\n)\n\nfunc TestF(t *testing.T) {\n\tif a.F() != 1 {\n\t\tt.Fatal()\n\t}\n}\n' >"$d/t/t_test.go"
	saida="$(cd "$d" && GOTOOLCHAIN=local GOFLAGS='' "$go" test -count=1 -coverpkg=./... ./... 2>&1)"
	local rc=$?
	rm -rf "$d"
	[ $rc -eq 0 ] || { printf '%s\n' "$saida" | tail -n 5 | sed 's/^/   /'; return 1; }
}

etapa_go() {
	titulo "Go (go.mod: $GO_VERSAO)"
	if ! procurar_go; then
		if [ "$MODO" = check ]; then
			registrar go FALTA "nenhum Go local $GO_VERSAO (instala em $GO_DESTINO)"
			if tem go; then
				local tc_atual efetivo
				tc_atual="$(go env GOTOOLCHAIN 2>/dev/null)"
				efetivo="$(cd "$RAIZ/backend/rotaperfumes-api" && go env GOVERSION 2>/dev/null)"
				registrar GOTOOLCHAIN AVISO "'$tc_atual': no repo o go troca para o toolchain baixado ${efetivo:-?} (módulo golang.org/toolchain), não para um Go local"
			fi
			registrar covdata FALTA "sem Go local $GO_VERSAO para conferir (TST-08)"
			info "Dica: com o go da distro mais antigo, o 'go' baixa o toolchain $GO_VERSAO sozinho (GOTOOLCHAIN=auto), mas esse download pode vir sem o covdata (TST-08)."
			return
		fi
		instalar_go || { registrar go FALTA "falha ao instalar Go $GO_VERSAO"; return; }
	fi
	local dir_go gopath atual v
	dir_go="$(dirname "$GO_BIN")"
	v="$(versao_go_de "$GO_BIN")"
	gopath="$(GOTOOLCHAIN=local "$GO_BIN" env GOPATH)"
	gopath="${gopath%%:*}"

	# PATH: o "go" do PATH precisa ser este (e não o da distro).
	atual="$(command -v go 2>/dev/null)"
	[ -n "$atual" ] && atual="$(readlink -f "$atual")"
	if [ "$atual" != "$GO_BIN" ]; then
		if [ "$MODO" = check ]; then
			registrar go AVISO "$v em $dir_go, mas o 'go' do PATH é ${atual:-nenhum}"
		elif [ "$AJUSTAR_PERFIL" -eq 1 ]; then
			gravar_perfil "export PATH=\"$dir_go:$gopath/bin:\$PATH\""
			registrar go OK "$v em $dir_go (PATH ajustado)"
		else
			registrar go AVISO "$v em $dir_go; ponha $dir_go no início do PATH"
		fi
		export PATH="$dir_go:$gopath/bin:$PATH"
	else
		registrar go OK "$v em $dir_go"
	fi
	GO_GOPATH="$gopath"

	# GOTOOLCHAIN: "local" evita a troca silenciosa por um toolchain baixado (TST-08).
	local tc tc_env
	tc="$("$GO_BIN" env GOTOOLCHAIN 2>/dev/null)"
	tc_env="${GOTOOLCHAIN:-}"
	if [ "$tc" = local ]; then
		registrar GOTOOLCHAIN OK "local"
	elif [ -n "$tc_env" ]; then
		registrar GOTOOLCHAIN AVISO "'$tc' vem da variável de ambiente GOTOOLCHAIN do shell; remova-a (o go env -w não a sobrepõe)"
	elif [ "$MODO" = instalar ] && [ "$AJUSTAR_GOTOOLCHAIN" -eq 1 ]; then
		if GOTOOLCHAIN=local "$GO_BIN" env -w GOTOOLCHAIN=local; then
			registrar GOTOOLCHAIN OK "local (antes: $tc; gravado em $(GOTOOLCHAIN=local "$GO_BIN" env GOENV))"
		else
			registrar GOTOOLCHAIN AVISO "não consegui gravar; rode: go env -w GOTOOLCHAIN=local"
		fi
	else
		registrar GOTOOLCHAIN AVISO "'$tc' (recomendado: go env -w GOTOOLCHAIN=local)"
	fi

	# covdata (TST-08)
	if [ -x "$(GOTOOLCHAIN=local "$GO_BIN" env GOTOOLDIR)/covdata" ] ||
		GOTOOLCHAIN=local "$GO_BIN" tool -n covdata >/dev/null 2>&1; then
		if smoke_covdata "$GO_BIN"; then
			registrar covdata OK "go tool -n covdata e go test -coverpkg funcionam"
		else
			registrar covdata FALTA "go test -coverpkg falhou no teste rápido (TST-08)"
		fi
	else
		registrar covdata FALTA "go tool -n covdata falhou (TST-08): reinstale o Go oficial"
	fi
}

# ----------------------------------------------------------------------------
# Node (nvm, versão do frontend/.nvmrc)
# ----------------------------------------------------------------------------
carregar_nvm() {
	export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
	if [ -s "$NVM_DIR/nvm.sh" ]; then
		# shellcheck disable=SC1091
		. "$NVM_DIR/nvm.sh" --no-use
		return 0
	fi
	return 1
}

etapa_node() {
	local alvo v nvm_ok=0
	alvo="$(tr -d ' \r\n' <"$ARQ_NVMRC")"; alvo="${alvo#v}"
	titulo "Node (frontend/.nvmrc: $alvo, mínimo $NODE_MIN)"
	carregar_nvm && nvm_ok=1
	if [ "$nvm_ok" -eq 1 ]; then
		registrar nvm OK "$(nvm --version 2>/dev/null)"
	elif [ "$MODO" = check ]; then
		registrar nvm FALTA "não encontrado em $NVM_DIR"
	else
		info "Instalando nvm v$NVM_VERSAO (script oficial do nvm-sh)..."
		local tmp; tmp="$(mktemp)"
		if baixar "https://raw.githubusercontent.com/nvm-sh/nvm/v$NVM_VERSAO/install.sh" "$tmp" &&
			bash "$tmp" && carregar_nvm; then
			nvm_ok=1; registrar nvm OK "$(nvm --version) (instalado)"
		else
			registrar nvm FALTA "falha ao instalar o nvm"
		fi
		rm -f "$tmp"
	fi

	if [ "$nvm_ok" -eq 1 ] && [ "$MODO" = instalar ]; then
		if ! nvm ls "$alvo" >/dev/null 2>&1; then
			info "nvm install $alvo"
			nvm install "$alvo" >/dev/null || aviso "nvm install $alvo falhou"
		fi
		nvm use "$alvo" >/dev/null 2>&1
	fi
	if [ "$nvm_ok" -eq 1 ] && [ "$MODO" = check ]; then
		# Na conferência usa o node que o terminal teria (alias default do nvm).
		nvm use default >/dev/null 2>&1 || true
	fi

	v="$(node --version 2>/dev/null | sed 's/^v//')"
	if [ "$v" = "$alvo" ]; then
		registrar node OK "$v ($(command -v node))"
	elif [ -n "$v" ] && [ "${v%%.*}" = "${alvo%%.*}" ] && ver_ge "$v" "$NODE_MIN"; then
		registrar node AVISO "$v (aceito: >= $NODE_MIN; o .nvmrc pede $alvo; rode 'nvm install' em frontend/)"
	elif [ -n "$v" ]; then
		registrar node FALTA "$v (precisa $alvo; rode 'nvm install' em frontend/)"
	else
		registrar node FALTA "não encontrado"
	fi
	if [ "$MODO" = instalar ] && [ "$nvm_ok" -eq 1 ] && [ "$v" = "$alvo" ]; then
		local def; def="$(nvm version default 2>/dev/null | sed 's/^v//')"
		if [ "$def" != "$alvo" ] && ! { [ "${def%%.*}" = "${alvo%%.*}" ] && ver_ge "$def" "$alvo"; }; then
			nvm alias default "$alvo" >/dev/null && info "nvm alias default -> $alvo (antes: ${def:-nenhum})"
		fi
	fi
	if tem npm; then registrar npm OK "$(npm --version 2>/dev/null)"; else registrar npm FALTA "vem com o Node"; fi
}

# ----------------------------------------------------------------------------
# govulncheck (go install) + GOPATH/bin no PATH
# ----------------------------------------------------------------------------
etapa_govulncheck() {
	titulo "govulncheck"
	local gobin exe v
	if [ -z "${GO_GOPATH:-}" ] && tem go; then GO_GOPATH="$(GOTOOLCHAIN=local go env GOPATH 2>/dev/null)"; GO_GOPATH="${GO_GOPATH%%:*}"; fi
	gobin="${GO_GOPATH:-$HOME/go}/bin"
	exe="$(command -v govulncheck 2>/dev/null)"
	[ -z "$exe" ] && [ -x "$gobin/govulncheck" ] && exe="$gobin/govulncheck"
	if [ -z "$exe" ] && [ "$MODO" = instalar ] && [ -n "$GO_BIN" ]; then
		info "go install golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSAO"
		GOTOOLCHAIN=local "$GO_BIN" install "golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSAO" && exe="$gobin/govulncheck"
	fi
	if [ -z "$exe" ] || [ ! -x "$exe" ]; then
		registrar govulncheck FALTA "go install golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSAO"; return
	fi
	v="$("$exe" -version 2>/dev/null | awk -F': ' '/^Scanner/ {print $2; exit}')"
	if tem govulncheck; then
		registrar govulncheck OK "${v:-instalado} ($exe)"
	elif [ "$MODO" = instalar ] && [ "$AJUSTAR_PERFIL" -eq 1 ]; then
		gravar_perfil "export PATH=\"$(dirname "${GO_BIN:-$gobin/go}"):$gobin:\$PATH\""
		registrar govulncheck OK "${v:-instalado} ($gobin no PATH)"
	else
		registrar govulncheck AVISO "${v:-instalado}, mas $gobin não está no PATH"
	fi
}

# ----------------------------------------------------------------------------
# Cliente MariaDB
# ----------------------------------------------------------------------------
versao_mariadb() { mariadb --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+-MariaDB' | head -n1 | sed 's/-MariaDB//'; }

# SEC-58 (Lote 37): a chave do repositório oficial só é instalada se o
# arquivo baixado tiver exatamente uma chave primária, não revogada nem
# expirada, com o fingerprint de MARIADB_GPG_FPR (scripts/versoes.conf).
# GNUPGHOME temporário: não cria nem lê o ~/.gnupg. Autocontida (o teste do
# SEC-58 a extrai do script); usa só aviso e tem.
# SEC-61: retorno 1 = chave recusada; 2 = não foi possível conferir (FPR
# inválido, gpg ausente ou mktemp falhou).
conferir_chave_mariadb() { # ARQ FPR_ESPERADO
	local arq=$1 esp gh fprs n
	esp=$(printf '%s' "${2:-}" | tr -d ' ' | tr 'a-f' 'A-F')
	[[ "$esp" =~ ^[0-9A-F]{40}$ ]] || { aviso "SEC-58: MARIADB_GPG_FPR ausente/invalido em versoes.conf"; return 2; }
	tem gpg || { aviso "SEC-58: gpg ausente: sem conferir a chave, o repositorio oficial do MariaDB nao e configurado"; return 2; }
	gh=$(mktemp -d) || return 2
	fprs=$(GNUPGHOME="$gh" LC_ALL=C gpg --batch --no-tty --quiet --with-colons --show-keys "$arq" 2>/dev/null |
		awk -F: '$1=="pub"{p=1; if ($2 ~ /[rei]/) bad=1; next} p&&$1=="fpr"{print $10; p=0} END{if (bad) print "REVOGADA_OU_EXPIRADA"}')
	rm -rf "$gh"
	n=$(printf '%s\n' "$fprs" | grep -c .)
	# shellcheck disable=SC2015 # o "|| C" deve rodar se A OU B falhar (é o pretendido)
	[ "$n" -eq 1 ] && [ "$fprs" = "$esp" ] || {
		aviso "SEC-58: chave do MariaDB com fingerprint inesperado (${fprs//$'\n'/ }); esperado $esp: nada instalado"
		return 1
	}
}

# SEC-58: a chave já instalada (de um setup anterior) é conferida também.
# Retorno: 0 = ausente ou confere; 1 e 2 como em conferir_chave_mariadb.
conferir_keyring_mariadb_instalado() { # ARQ
	local k=$1 rc=0
	[ -e "$k" ] || [ -L "$k" ] || return 0
	conferir_chave_mariadb "$k" "${MARIADB_GPG_FPR:-}" || rc=$?
	[ "$rc" -ne 0 ] || info "SEC-58: $k confere com MARIADB_GPG_FPR"
	return "$rc"
}

# SEC-61: o mariadb.sources confia nesse keyring, e o primeiro apt-get update
# já usaria a chave; por isso roda em main() antes de etapa_basicas. Instalar
# com apt: keyring divergente ou não conferível aborta antes de qualquer apt.
# Sem apt ou no --check: nada será instalado, só registra (divergente = FALTA;
# não conferível = AVISO).
preflight_keyring_mariadb() { # ARQ
	local k=$1 rc=0 problema status
	conferir_keyring_mariadb_instalado "$k" || rc=$?
	case "$rc" in
	0) return 0 ;;
	2)
		status=AVISO
		if tem gpg; then
			problema="não foi possível conferir $k: MARIADB_GPG_FPR inválido em scripts/versoes.conf ou mktemp falhou"
		else
			problema="não foi possível conferir $k: gpg ausente"
		fi
		;;
	*)
		status=FALTA
		problema="$k não confere com MARIADB_GPG_FPR (scripts/versoes.conf)"
		;;
	esac
	if [ "$MODO" = instalar ] && [ "$APT" -eq 1 ]; then
		printf 'SEC-61: %s. Nada foi instalado. Confira e reinstale: sudo rm %s /etc/apt/sources.list.d/mariadb.sources && make setup\n' "$problema" "$k" >&2
		exit 1
	fi
	registrar mariadb-key "$status" "$problema: confira e reinstale (sudo rm $k /etc/apt/sources.list.d/mariadb.sources && make setup)"
}

configurar_repo_mariadb() {
	local base suite
	case " $DISTRO_ID $DISTRO_LIKE " in
	*" ubuntu "*) base=ubuntu; suite="${UBUNTU_CODENAME:-${VERSION_CODENAME:-}}" ;;
	*" debian "*) base=debian; suite="${VERSION_CODENAME:-}" ;;
	esac
	# shellcheck disable=SC2015 # o "|| C" deve rodar se A OU B falhar (é o pretendido)
	[ -n "${base:-}" ] && [ -n "${suite:-}" ] || { aviso "não identifiquei a versão da distro para o repositório do MariaDB"; return 1; }
	info "Configurando o repositório oficial do MariaDB $MARIADB_SERIE ($base $suite)"
	tem gpg || apt_instalar gpg || true
	local tmp; tmp="$(mktemp)"
	baixar "https://mariadb.org/mariadb_release_signing_key.pgp" "$tmp" || { rm -f "$tmp"; return 1; }
	conferir_chave_mariadb "$tmp" "${MARIADB_GPG_FPR:-}" || { rm -f "$tmp"; return 1; }
	# shellcheck disable=SC2015 # o "|| C" deve rodar se A OU B falhar (é o pretendido)
	como_root install -d -m 0755 /etc/apt/keyrings &&
		como_root install -m 0644 "$tmp" /etc/apt/keyrings/mariadb-keyring.pgp || { rm -f "$tmp"; return 1; }
	printf 'X-Repolib-Name: MariaDB\nTypes: deb\nURIs: https://dlm.mariadb.com/repo/mariadb-server/%s/repo/%s\nSuites: %s\nComponents: main\nSigned-By: /etc/apt/keyrings/mariadb-keyring.pgp\n' \
		"$MARIADB_SERIE" "$base" "$suite" >"$tmp"
	como_root install -m 0644 "$tmp" /etc/apt/sources.list.d/mariadb.sources || { rm -f "$tmp"; return 1; }
	rm -f "$tmp"
	APT_ATUALIZADO=0
}

etapa_mariadb() {
	titulo "Cliente MariaDB (mínimo $MARIADB_CLIENTE_MIN)"
	local v; v="$(versao_mariadb)"
	if [ -n "$v" ] && ver_ge "$v" "$MARIADB_CLIENTE_MIN"; then
		registrar mariadb OK "$v"; return
	fi
	if [ "$MODO" = check ]; then
		if [ -n "$v" ]; then
			registrar mariadb AVISO "$v (abaixo do $MARIADB_CLIENTE_MIN documentado; o setup instala o do repositório oficial)"
		else
			registrar mariadb FALTA "cliente mariadb não encontrado"
		fi
		return
	fi
	if [ "$APT" -eq 0 ]; then
		registrar mariadb "$([ -n "$v" ] && echo AVISO || echo FALTA)" "${v:-ausente}: instale o cliente MariaDB >= $MARIADB_CLIENTE_MIN pela distro"
		return
	fi
	if [ "$MARIADB_OFICIAL" -eq 1 ]; then
		configurar_repo_mariadb || aviso "seguindo com o mariadb-client da distro"
	fi
	apt_instalar mariadb-client || true
	v="$(versao_mariadb)"
	if [ -n "$v" ] && ver_ge "$v" "$MARIADB_CLIENTE_MIN"; then
		registrar mariadb OK "$v (instalado)"
	elif [ -n "$v" ]; then
		registrar mariadb AVISO "$v (abaixo do $MARIADB_CLIENTE_MIN; funciona para os make db-*, mas não é o documentado)"
	else
		registrar mariadb FALTA "falha ao instalar mariadb-client"
	fi
}

# ----------------------------------------------------------------------------
# Postman CLI
# ----------------------------------------------------------------------------
versao_postman() {
	local s
	s="$(postman --version 2>/dev/null | head -n1)"
	[[ "$s" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+ ]] && echo "${s#v}"
}

etapa_postman() {
	titulo "Postman CLI"
	local v; v="$(versao_postman)"
	if [ -n "$v" ] && ver_ge "$v" "$POSTMAN_CLI_MIN"; then
		registrar postman OK "$v ($(command -v postman))"; return
	fi
	if tem postman && [ -z "$v" ]; then
		aviso "'$(command -v postman)' não responde a 'postman --version' (é o app desktop do Postman?): o CLI precisa vir antes dele no PATH."
	fi
	if [ "$MODO" = check ]; then registrar postman FALTA "Postman CLI não encontrado"; return; fi
	if [ "$POSTMAN_VIA" = auto ] && tem npm; then
		info "npm install -g postman-cli (pacote oficial do Postman)"
		npm install -g postman-cli >/dev/null 2>&1 || aviso "npm install -g postman-cli falhou"
	else
		info "Instalador oficial do Postman CLI (https://dl-cli.pstmn.io/install/linux64.sh); ele grava em /usr/local/bin e pode pedir sudo."
		local tmp; tmp="$(mktemp)"
		if baixar "https://dl-cli.pstmn.io/install/linux64.sh" "$tmp"; then
			if [ "$SEM_SUDO" -eq 1 ]; then aviso "--sem-sudo: pulando o instalador do Postman CLI"; else sh "$tmp" || aviso "instalador do Postman CLI falhou"; fi
		fi
		rm -f "$tmp"
	fi
	hash -r
	v="$(versao_postman)"
	if [ -n "$v" ]; then registrar postman OK "$v (instalado)"; else registrar postman FALTA "veja https://learning.postman.com/docs/postman-cli/postman-cli-installation/"; fi
}

# ----------------------------------------------------------------------------
# ShellCheck (TST-17, make shellcheck): opcional, nunca FALTA (não quebra o
# make setup-check); instalado pelo apt quando houver.
# ----------------------------------------------------------------------------
etapa_shellcheck() {
	titulo "shellcheck (opcional, make shellcheck)"
	local v
	if ! tem shellcheck && [ "$MODO" != check ] && [ "$APT" -eq 1 ]; then
		info "Instalando shellcheck..."
		apt_instalar shellcheck || true
	fi
	if ! tem shellcheck; then
		registrar shellcheck AVISO "não encontrado (opcional; apt: shellcheck)"; return
	fi
	v="$(shellcheck --version 2>/dev/null | sed -n 's/^version: *//p' | extrair_versao)"
	if ver_ge "${v:-0}" "$SHELLCHECK_MIN"; then
		registrar shellcheck OK "${v:-?}"
	else
		registrar shellcheck AVISO "${v:-?} (mínimo $SHELLCHECK_MIN)"
	fi
}

# ----------------------------------------------------------------------------
# Docker (opcional, só para deploy): só confere
# ----------------------------------------------------------------------------
etapa_docker() {
	titulo "Docker (opcional, deploy)"
	if ! tem docker; then registrar docker AVISO "não encontrado (opcional; ver docs/deploy-servidor.md)"; return; fi
	local v c
	v="$(docker --version 2>/dev/null | extrair_versao)"
	if c="$(docker compose version --short 2>/dev/null)"; then
		registrar docker OK "$v, compose $c"
	else
		registrar docker AVISO "$v, sem o plugin 'docker compose' (opcional)"
	fi
}

# ----------------------------------------------------------------------------
# Dependências do projeto (--deps)
# ----------------------------------------------------------------------------
etapa_deps() {
	titulo "Dependências do projeto (--deps)"
	if [ "$MODO" = check ]; then info "(--check: pulado)"; return; fi
	local m ok=1
	for m in backend/shared backend/rotaperfumes-api; do
		info "go mod download em $m"
		(cd "$RAIZ/$m" && GOTOOLCHAIN=local go mod download) || ok=0
	done
	info "make frontend-deps"
	make -C "$RAIZ" frontend-deps >/dev/null || ok=0
	if [ "$ok" -eq 1 ]; then registrar deps OK "go mod download + npm install"; else registrar deps FALTA "veja os erros acima"; fi
}

# ----------------------------------------------------------------------------
main() {
	carregar_versoes
	versao_go_mod
	printf 'RotaPerfume - setup do ambiente (Linux) - modo: %s\n' "$MODO"
	info "Repositório: $RAIZ"
	detectar_distro
	preflight_keyring_mariadb /etc/apt/keyrings/mariadb-keyring.pgp
	etapa_basicas
	etapa_go
	etapa_node
	etapa_govulncheck
	etapa_mariadb
	etapa_postman
	etapa_shellcheck
	etapa_docker
	[ "$DEPS" -eq 1 ] && etapa_deps
	resumo
}

main
