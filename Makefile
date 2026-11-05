.PHONY: help db-guard-producao db-guard-sem-producao db-guard-drop db-guard-integracao fix-admin test-integration reset-senhas-simular reset-senhas-ativos db-gerar-imagens-produtos db-check-env db-up db-down db-seed db-reset db-create db-fix-deve-trocar-senha db-fix-tipo-reset db-fix-cnpj-unique db-revert-cnpj-unique db-fix-cnpj-comment db-revert-cnpj-comment db-fix-revoked-reason db-revert-revoked-reason db-fix-tokens-validos-desde db-revert-tokens-validos-desde db-fix-reuso-detectado db-revert-reuso-detectado db-fix-taxa-pct-check db-revert-taxa-pct-check db-fix-produtos-imagem db-revert-produtos-imagem db-fix-estoque-origem db-revert-estoque-origem db-rebuild db-export db-import-clientes db-import-produtos db-import-pedidos db-import-pagamentos db-import-carteiras db-import-oportunidades db-import-visitas db-import-estoque db-seed-mes-atual test test-all lint \
	build build-api run-api dev-api stop-api \
	test-api cover-api test-shared cover-shared arch-check cover-combinada gen-hash fix-hash \
	frontend-deps \
	audit deps-audit deps-audit-go deps-audit-frontend deps-audit-imagens \
	setup setup-check

# =============================================================================
# Helpers
# =============================================================================
# SEC-11: as credenciais do banco vêm do .env (sem default).
# Precedência: com "-include .env", o valor do .env ganha da variável do
# shell; "make DB_SENHA=x" na linha de comando ganha de tudo.
# Formato do .env: DB_SENHA sem "$", "#", aspas, espaços nas pontas e sem
# começar com "/" (o make e o Git Bash interpretariam esses caracteres).
ifeq ($(origin ENV_ARQUIVO),command line)
# DEPLOY-07: outro arquivo NO LUGAR do .env (ex.: .env.producao da maquina do
# admin); o .env NAO e lido. So pela linha de comando:
#   make db-up ENV_ARQUIVO=.env.producao CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>
include $(ENV_ARQUIVO)
else
-include .env
endif

DB_NAME ?= rotaperfumes
DB_HOST ?= localhost
DB_PORT ?= 3306
# SEC-11: sem default para DB_USUARIO/DB_SENHA (obrigatórios no .env).

# .env salvo com CRLF no Windows deixaria "\r" no fim do valor.
CR := $(shell printf '\r')
DB_USUARIO := $(strip $(subst $(CR),,$(DB_USUARIO)))
DB_SENHA   := $(subst $(CR),,$(DB_SENHA))
DB_HOST    := $(strip $(subst $(CR),,$(DB_HOST)))
DB_PORT    := $(strip $(subst $(CR),,$(DB_PORT)))
DB_NAME    := $(strip $(subst $(CR),,$(DB_NAME)))
DB_TLS_CA  := $(strip $(subst $(CR),,$(DB_TLS_CA)))
DB_TLS_REQUIRED := $(strip $(subst $(CR),,$(DB_TLS_REQUIRED)))

# Senha só pelo ambiente (lida nativamente pelo cliente mariadb), nunca no argv.
MYSQL_PWD = $(DB_SENHA)
# Git Bash/MSYS: impede a conversão de caminho POSIX no valor da senha.
MSYS2_ENV_CONV_EXCL = MYSQL_PWD
export

# SEC-53: segredos do .env fora dos processos de teste, build, lint e npm.
# O "export" acima continua global (o cliente mariadb dos db-* precisa do
# MYSQL_PWD e as CLIs Go de db-import-*/fix-*/reset-senhas-* leem o ambiente),
# mas as receitas de go test/build/vet e npm rodam com $(SEM_SEGREDOS), que
# tira (env -u) toda variavel declarada no arquivo lido (.env ou ENV_ARQUIVO)
# e os segredos fixos abaixo, menos NEXT_PUBLIC_* (publicas, o build do
# frontend precisa). Excecao (so no $(SEM_SEGREDOS_TESTE), SEC-55): "make
# test-integration" chama o sub-make com INTEGRATION=1 NA LINHA DE COMANDO
# (passando pelo db-guard-producao e pelo db-guard-integracao); so ai as DB_*
# chegam ao go test. "INTEGRATION=1 make test" do shell fica sem
# credenciais: os testes de integracao sao pulados.
ENV_ARQUIVO_LIDO := $(if $(filter command line,$(origin ENV_ARQUIVO)),$(ENV_ARQUIVO),.env)
VARS_DO_ENV := $(shell sed -n 's/^[[:space:]]*\(export[[:space:]][[:space:]]*\)\{0,1\}\([A-Za-z_][A-Za-z0-9_]*\)[[:space:]]*[:?+]\{0,1\}=.*/\2/p' $(ENV_ARQUIVO_LIDO) 2>/dev/null)
VARS_DB_INTEGRACAO := DB_HOST DB_PORT DB_NAME DB_USUARIO DB_SENHA DB_TLS_CA DB_TLS_SERVER_NAME DB_TLS_REQUIRED
SEGREDOS_FIXOS := $(VARS_DB_INTEGRACAO) MYSQL_PWD MSYS2_ENV_CONV_EXCL MYSQL_OPTS MYSQL_TLS_OPTS DB_CLI PASSWORD_PEPPER JWT_SECRET SMTP_USER SMTP_PASSWORD TURNSTILE_SECRET_KEY SEED_ADMIN_PASSWORD
INTEGRACAO_PELO_MAKE := $(and $(filter command line,$(origin INTEGRATION)),$(filter 1,$(INTEGRATION)))
# SEC-55: SEM_SEGREDOS e estrito (sem excecao): go build, go tool cover,
# go vet, go get/mod, govulncheck e npm nunca recebem DB_*/MYSQL_PWD, nem com
# INTEGRATION=1. SEM_SEGREDOS_TESTE (so nas linhas "go test" de test-shared,
# test-api, cover-shared, cover-api e cover-combinada) libera as DB_* quando
# INTEGRATION=1 veio da linha de comando (test-integration).
SEM_SEGREDOS = env $(addprefix -u ,$(sort $(filter-out NEXT_PUBLIC_%,$(VARS_DO_ENV) $(SEGREDOS_FIXOS))))
SEM_SEGREDOS_TESTE = env $(addprefix -u ,$(sort $(filter-out NEXT_PUBLIC_% $(if $(INTEGRACAO_PELO_MAKE),$(VARS_DB_INTEGRACAO)),$(VARS_DO_ENV) $(SEGREDOS_FIXOS))))
# Com as DB_* liberadas (INTEGRATION=1 na linha de comando), test-shared,
# test-api, cover-*, arch-check e cover-combinada passam tambem pela guarda de
# producao e pela allowlist da integracao (SEC-54; vale para
# "make test INTEGRATION=1" chamado direto, sem o test-integration).
GUARDA_INTEGRACAO := $(if $(INTEGRACAO_PELO_MAKE),db-guard-producao db-guard-integracao)

# SEC-29/CHORE-13: com DB_TLS_CA no .env, o cliente mariadb conecta por TLS
# conferindo a CA e o nome do servidor (--ssl-ca + --ssl-verify-server-cert;
# DB_HOST precisa estar no SAN do certificado: o cliente não tem equivalente ao
# DB_TLS_SERVER_NAME da API). Caminho relativo vale a partir da raiz do projeto
# (onde o make roda). SEC-42: sem DB_TLS_CA, só com DB_HOST loopback
# (localhost, 127.x.x.x, ::1) ou DB_TLS_REQUIRED=false (conferido no db-check-env).
# CHORE-13: o projeto usa só o cliente MariaDB. DB_CLIENTE_BIN troca o
# executável (padrão mariadb), que o db-check-env confere ser o cliente MariaDB:
#   make db-up DB_CLIENTE_BIN=/opt/mariadb/bin/mariadb
DB_CLIENTE_BIN := $(or $(strip $(subst $(CR),,$(DB_CLIENTE_BIN))),mariadb)
MYSQL_TLS_OPTS = $(if $(DB_TLS_CA),--ssl-ca="$(DB_TLS_CA)" --ssl-verify-server-cert)
MYSQL_OPTS = --local-infile=0 -u $(DB_USUARIO) -h $(DB_HOST) -P $(DB_PORT) --default-character-set=utf8mb4 $(MYSQL_TLS_OPTS)
DB_CLI = $(DB_CLIENTE_BIN) $(MYSQL_OPTS)

help: ## Mostra esta ajuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(firstword $(MAKEFILE_LIST)) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# =============================================================================
# Ambiente de desenvolvimento (SETUP-01)
# =============================================================================
# Versões: Go = diretiva "go" dos go.mod; Node = frontend/.nvmrc; demais em
# scripts/versoes.conf. No Windows (Git Bash) chama o scripts/setup-windows.ps1.
# Opções extras: make setup SETUP_ARGS="--deps --go-sistema" (ver --help).
ifeq ($(OS),Windows_NT)
SETUP_CMD = powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/setup-windows.ps1
SETUP_CHECK_FLAG = -Check
else
SETUP_CMD = bash scripts/setup-linux.sh
SETUP_CHECK_FLAG = --check
endif

setup: ## SETUP-01: instala/ajusta as ferramentas (Go do go.mod, Node do .nvmrc, govulncheck, mariadb, Postman CLI)
	$(SETUP_CMD) $(SETUP_ARGS)

setup-check: ## SETUP-01: só confere versões (inclui covdata e GOTOOLCHAIN, pré-requisito do make test - TST-08)
	$(SETUP_CMD) $(SETUP_CHECK_FLAG) $(SETUP_ARGS)

# =============================================================================
# Banco de dados
# =============================================================================
# SEC-11: pré-requisito de todo alvo que chama o cliente do banco (db-create, db-down,
# db-fix-*, db-revert-*; db-up/db-seed herdam via db-create). Fica depois do
# "help" para não virar o alvo padrão do make.
db-check-env: ## Confere DB_USUARIO/DB_SENHA (SEC-11), o TLS exigido para host remoto (SEC-42) e o cliente MariaDB (DB_CLIENTE_BIN)
	@test -n "$$DB_USUARIO" -a -n "$$MYSQL_PWD" || { echo "SEC-11: defina DB_USUARIO/DB_SENHA no .env"; exit 1; }
	@case "$(DB_TLS_REQUIRED)" in ""|true|false) ;; *) echo "SEC-42: DB_TLS_REQUIRED=$(DB_TLS_REQUIRED) invalido: use true ou false"; exit 1 ;; esac; \
	$(NORMALIZA_HOST); \
	if $(HOST_LOOPBACK); then :; \
	elif [ -n "$(DB_TLS_CA)" ]; then :; \
	elif [ "$(DB_TLS_REQUIRED)" = false ]; then echo "AVISO (SEC-42): DB_TLS_REQUIRED=false: conexao SEM TLS com DB_HOST=$(DB_HOST) (fora do loopback); usuario e senha em texto claro"; \
	else echo "SEC-42: DB_HOST=$(DB_HOST) nao e loopback (localhost, 127.x.x.x, ::1) e DB_TLS_CA esta vazio: defina DB_TLS_CA no .env (ou, so sabendo do risco, DB_TLS_REQUIRED=false)"; exit 1; fi
	@test -z "$(DB_TLS_CA)" -o -r "$(DB_TLS_CA)" || { echo "SEC-29: DB_TLS_CA=$(DB_TLS_CA) nao encontrado/legivel (caminho relativo vale a partir da raiz do projeto)"; exit 1; }
	@test -z "$(DB_CLIENTE)" || { echo "CHORE-13: DB_CLIENTE nao existe mais (o projeto usa so o cliente MariaDB): remova do .env; para outro executavel use DB_CLIENTE_BIN"; exit 1; }
	@v=$$($(DB_CLIENTE_BIN) --version 2>&1) || { echo "DB_CLIENTE_BIN=$(DB_CLIENTE_BIN) nao encontrado no PATH (instale o cliente MariaDB: make setup)"; exit 1; }; \
	case "$$v" in *MariaDB*) ;; *) echo "DB_CLIENTE_BIN=$(DB_CLIENTE_BIN) nao e o cliente MariaDB ($$v)"; exit 1 ;; esac

# DEPLOY-07: o banco de PRODUCAO (container rotaperfumes-mariadb) nunca recebe
# make db-*/fix-*/import por engano. Todo alvo que escreve no banco depende
# desta guarda. Producao =
#   DB_HOST em PRODUCAO_HOSTS_LAN (IP/hostname do servidor) com DB_PORT vazio
#   ou 3306 (a porta publicada com --dev-ip), OU
#   DB_HOST em PRODUCAO_HOSTS_REDE (nome/IP na rede docker), qualquer porta.
# O MariaDB de DEV do CasaOS roda no MESMO host, porta 3307: passa.
# Producao aborta, salvo CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME> NA LINHA DE
# COMANDO (no .env ou no ambiente nao vale, para nao virar confirmacao
# permanente). O seed inicial de producao e o deploy/seed-producao.sh.
PRODUCAO_HOSTS_LAN := 192.168.168.106 ivo-inspiron-15-3530
PRODUCAO_HOSTS_REDE := rotaperfumes-mariadb 172.30.42.2
# SEC-54: normalizacao textual do DB_HOST (sem DNS), a mesma do
# normalizarHost do backend/shared/testkit/integracao (paridade testada):
# espacos nas pontas, minusculas, "[" e "]" das pontas, todos os "." finais,
# um sufixo ".local" ($$eralocal=true) e IPv4 mapeado em IPv6
# (::ffff:a.b.c.d e 0:0:0:0:0:ffff:a.b.c.d viram a.b.c.d). Define $$h.
NORMALIZA_HOST = h=$$(printf '%s' "$(DB_HOST)" | tr 'A-Z' 'a-z' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$$//' -e 's/^\[//' -e 's/\]$$//' -e 's/\.*$$//'); \
	eralocal=false; case "$$h" in *.local) h=$${h%.local}; eralocal=true ;; esac; \
	h=$$(printf '%s' "$$h" | sed -e 's/^::ffff:\([0-9]\{1,3\}\(\.[0-9]\{1,3\}\)\{3\}\)$$/\1/' -e 's/^0:0:0:0:0:ffff:\([0-9]\{1,3\}\(\.[0-9]\{1,3\}\)\{3\}\)$$/\1/')
# Loopback (localhost, ::1, 127.x.x.x) depois do NORMALIZA_HOST; nome com
# ".local" (mDNS, pode ser qualquer maquina) nunca conta como loopback.
HOST_LOOPBACK = { ! $$eralocal && { [ "$$h" = localhost ] || [ "$$h" = ::1 ] || printf '%s' "$$h" | grep -Eq '^127\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$$'; }; }
# SEC-46: deteccao reutilizada pelas guardas (define $$h, $$porta e $$prod no
# shell da receita). A porta e normalizada como numero ("+"/zeros a esquerda:
# 03306 e 3306 para o cliente mariadb e para o Go), igual ao SEC-50. O SEC-50 aplica a mesma regra nos testes Go
# (backend/shared/testkit/integracao; paridade testada). SEC-54: porta nao
# numerica e literal com ":" que nao seja ::1 (IPv6) contam como PRODUCAO
# (fail-closed).
PRODUCAO_DETECTA = $(NORMALIZA_HOST); \
	porta=$$(printf '%s' "$(DB_PORT)" | sed 's/^+//; s/^0*\([0-9]\)/\1/'); prod=false; \
	pnum="$(DB_PORT)"; case "$$pnum" in +) prod=true ;; esac; case "$${pnum\#+}" in *[!0-9]*) prod=true ;; esac; \
	case "$$h" in ::1) ;; *:*) prod=true ;; esac; \
	for p in $(PRODUCAO_HOSTS_REDE); do [ "$$h" = "$$p" ] && prod=true; done; \
	for p in $(PRODUCAO_HOSTS_LAN); do \
	  if [ "$$h" = "$$p" ] && { [ -z "$$porta" ] || [ "$$porta" = 3306 ]; }; then prod=true; fi; \
	done
db-guard-producao: ## DEPLOY-07: recusa o banco de producao (salvo CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME> na linha de comando)
	@$(PRODUCAO_DETECTA); \
	$$prod || exit 0; \
	if [ "$(origin CONFIRMO_PRODUCAO)" = "command line" ] && [ "$(CONFIRMO_PRODUCAO)" = "$(DB_HOST)/$(DB_NAME)" ]; then \
	  echo "AVISO (DEPLOY-07): $(DB_HOST):$${porta:-3306} e PRODUCAO; confirmado por CONFIRMO_PRODUCAO=$(DB_HOST)/$(DB_NAME)."; exit 0; \
	fi; \
	echo "DEPLOY-07: DB_HOST=$(DB_HOST) DB_PORT=$${porta:-3306} e o banco de PRODUCAO: alvo recusado (make db-* apaga/altera dados reais)."; \
	echo "  O dev (CasaOS) e a porta 3307. Seed inicial de producao: sudo bash deploy/seed-producao.sh no servidor."; \
	echo "  Se for mesmo producao, repita com CONFIRMO_PRODUCAO=$(DB_HOST)/$(DB_NAME) na linha de comando."; \
	exit 1

# SEC-46: db-down/db-reset/db-rebuild (DROP DATABASE) e db-seed (sobrescreve
# usuarios/senhas) NUNCA rodam na producao, nem com CONFIRMO_PRODUCAO (sem
# valvula). Producao se recria no servidor: backup, mariadb-docker.sh
# --recriar e seed-producao.sh. Pre-requisito direto (vale com make -j).
db-guard-sem-producao: ## SEC-46: recusa db-down/db-reset/db-rebuild/db-seed na producao, mesmo com CONFIRMO_PRODUCAO
	@$(PRODUCAO_DETECTA); \
	$$prod || exit 0; \
	echo "SEC-46: DB_HOST=$(DB_HOST) DB_PORT=$${porta:-3306} e PRODUCAO: db-down/db-reset/db-rebuild/db-seed sao recusados mesmo com CONFIRMO_PRODUCAO (apagam ou sobrescrevem dados reais). Recriar producao: backup (BACKUP-01), sudo bash deploy/mariadb-docker.sh --recriar e sudo bash deploy/seed-producao.sh no servidor."; \
	exit 1

# SEC-46: DROP DATABASE rotaperfumes (o nome real, sem diferenciar maiusculas)
# fora do loopback (mesmo teste do db-check-env: localhost, ::1, [::1],
# 127.x.x.x), como o dev do CasaOS (:3307) ou um alias da producao fora da
# lista, exige CONFIRMAR=<DB_NAME> NA LINHA DE COMANDO (no .env ou no
# ambiente nao vale). Loopback ou outro nome de banco: sem pergunta.
db-guard-drop: ## SEC-46: DROP DATABASE rotaperfumes fora do loopback exige CONFIRMAR=<DB_NAME> na linha de comando
	@$(NORMALIZA_HOST); \
	if $(HOST_LOOPBACK); then exit 0; fi; \
	[ "$$(printf '%s' "$(DB_NAME)" | tr 'A-Z' 'a-z')" = rotaperfumes ] || exit 0; \
	porta="$(DB_PORT)"; \
	if [ "$(origin CONFIRMAR)" = "command line" ] && [ "$(CONFIRMAR)" = "$(DB_NAME)" ]; then \
	  echo "AVISO (SEC-46): DROP DATABASE $(DB_NAME) em $(DB_HOST):$${porta:-3306} confirmado por CONFIRMAR=$(DB_NAME)."; exit 0; \
	fi; \
	echo "SEC-46: DROP DATABASE $(DB_NAME) em $(DB_HOST):$${porta:-3306} (fora do loopback): repita com CONFIRMAR=$(DB_NAME) na linha de comando (no .env ou no ambiente nao vale)."; \
	exit 1

# SEC-54: allowlist dos testes de integracao (test-integration e GUARDA_INTEGRACAO),
# a mesma de integracao.Permitidos no Go (paridade testada). "host" vale em
# qualquer porta; "host:porta", so nessa porta (DB_PORT vazio = 3306);
# 127.0.0.0/8 e o loopback IPv4 (textual). O CasaOS de dev so na 3307. Nome
# com ".local" nunca passa. Os alvos db-* continuam com a blocklist do
# db-guard-producao (outro IP da maquina: risco residual aceito).
INTEGRACAO_PERMITIDOS := localhost 127.0.0.0/8 ::1 192.168.168.106:3307
db-guard-integracao: ## SEC-54: testes de integracao so no loopback ou no CasaOS de dev (192.168.168.106:3307)
	@$(PRODUCAO_DETECTA); \
	ok=false; $$prod || $$eralocal || for e in $(INTEGRACAO_PERMITIDOS); do \
	  case "$$e" in \
	    127.0.0.0/8) printf '%s' "$$h" | grep -Eq '^127\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$$' && ok=true ;; \
	    *:*:*) [ "$$h" = "$$e" ] && ok=true ;; \
	    *:*) [ "$$h:$${porta:-3306}" = "$$e" ] && ok=true ;; \
	    *) [ "$$h" = "$$e" ] && ok=true ;; \
	  esac; \
	done; \
	$$ok && exit 0; \
	echo "SEC-54: DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) fora da allowlist da integracao ($(INTEGRACAO_PERMITIDOS)): use um MariaDB descartavel no loopback ou o dev do CasaOS (porta 3307)."; \
	exit 1

db-create: db-guard-producao db-check-env ## Cria o banco de dados se não existir
	$(DB_CLI) -e "CREATE DATABASE IF NOT EXISTS $(DB_NAME) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

# DEPLOY-07: a ordem da DDL fica em sql/ordem-ddl.txt (a mesma usada pelo
# deploy/seed-producao.sh). Linhas vazias e "#..." sao ignoradas.
DDL_ORDEM := $(shell grep -Ev '^[[:space:]]*(\#|$$)' sql/ordem-ddl.txt 2>/dev/null)

db-up: db-guard-producao db-create ## Cria o schema (tabelas vazias) na ordem de sql/ordem-ddl.txt
	@test -n "$(DDL_ORDEM)" || { echo "DEPLOY-07: sql/ordem-ddl.txt ausente ou vazio"; exit 1; }
	@for f in $(DDL_ORDEM); do \
	  echo "=== Aplicando DDL sql/$$f ==="; \
	  $(DB_CLI) $(DB_NAME) < "sql/$$f" || exit 1; \
	done

db-seed: db-guard-producao db-guard-sem-producao db-up ## Cria o schema, carrega dados e corrige hashes (DEV; producao: deploy/seed-producao.sh)
	@echo "=== Seed: admin principal ==="
	$(DB_CLI) $(DB_NAME) < sql/02_seed_admin.sql
	@echo "=== Seed: usuarios dos 42 vendedores ==="
	$(DB_CLI) $(DB_NAME) < sql/03_seed_vendedores.sql
	@echo ""
	@echo "=== Corrigindo hashes (placeholder -> argon2id real) + criando admin ==="
	cd backend/shared && go run ./cmd/resetpassword -list
	cd backend/shared && go run ./cmd/resetpassword -create-admin
	cd backend/shared && go run ./cmd/resetpassword -all-users
	@echo ""
	@echo "=== Seed completo com hashes validos! ==="
	@echo "=== Admin: admin@rotaperfumes.com.br (senha: SEED_ADMIN_PASSWORD do .env ou a ADMIN_PASSWORD impressa acima; troca obrigatoria no 1o login) ==="

db-down: db-guard-producao db-guard-sem-producao db-guard-drop db-check-env ## Dropa o banco de dados (CUIDADO!)
	$(DB_CLI) -e "DROP DATABASE IF EXISTS $(DB_NAME);"

# BUG-27: as 3 guardas sao pre-requisitos diretos; db-down, db-seed e os
# db-import-* rodam na receita, um sub-make por etapa encadeado por "&&". Com
# make -k a receita nem roda se uma guarda falhar e o "&&" para na primeira
# etapa que falhar (um sub-make so com varios goals continuaria com o -k
# herdado); com -j cada etapa so comeca depois da anterior. CONFIRMAR,
# CONFIRMO_PRODUCAO e ENV_ARQUIVO NAO sao repassados a mao: os da linha de
# comando ja chegam pelo MAKEFLAGS (origin = command line); repassar
# transformaria um valor do .env/ambiente em linha de comando (SEC-46).
# As guardas rodam de novo em cada sub-make (o AVISO pode repetir).
db-reset: db-guard-producao db-guard-sem-producao db-guard-drop ## Recria o banco do zero com hashes validos (db-down && db-seed em sequencia; para no 1o erro, mesmo com -k/-j: BUG-27)
	$(MAKE) db-down && $(MAKE) db-seed

db-fix-deve-trocar-senha: db-guard-producao db-check-env ## Adiciona a coluna deve_trocar_senha em bancos existentes (nao destrutivo, sem apagar dados)
	$(DB_CLI) $(DB_NAME) < sql/08_alter_usuarios_deve_trocar_senha.sql

db-fix-tipo-reset: db-guard-producao db-check-env ## Corrige o ENUM de senha_historico.tipo_reset em bancos existentes (nao destrutivo, sem apagar dados)
	$(DB_CLI) $(DB_NAME) < sql/13_alter_senha_historico_tipo_reset.sql

db-fix-estoque-origem: db-guard-producao db-check-env ## Remove estoque.origem (migracao 18) em bancos criados com o 17 antigo (idempotente; perde o rastreio de origem)
	$(DB_CLI) $(DB_NAME) < sql/18_alter_estoque_drop_origem.sql

db-revert-estoque-origem: db-guard-producao db-check-env ## Reverte db-fix-estoque-origem (recria estoque.origem; linhas existentes voltam como import_csv)
	$(DB_CLI) $(DB_NAME) < sql/18_revert_estoque_drop_origem.sql

db-fix-cnpj-unique: db-guard-producao db-check-env ## Unifica clientes com CNPJ duplicado no menor id e cria UNIQUE uq_clientes_cnpj (APAGA as copias; backup em clientes_merge_backup_20260925*)
	$(DB_CLI) $(DB_NAME) < sql/19_alter_clientes_cnpj_unique.sql

db-revert-cnpj-unique: db-guard-producao db-check-env ## Reverte db-fix-cnpj-unique a partir das tabelas clientes_merge_backup_20260925*
	$(DB_CLI) $(DB_NAME) < sql/19_revert_clientes_cnpj_unique.sql

db-fix-cnpj-comment: db-guard-producao db-check-env ## Atualiza o COMMENT de clientes.cnpj para o formato alfanumerico (so metadado, sem apagar dados)
	$(DB_CLI) $(DB_NAME) < sql/20_alter_clientes_cnpj_comment.sql

db-revert-cnpj-comment: db-guard-producao db-check-env ## Reverte db-fix-cnpj-comment (volta o COMMENT antigo "somente digitos")
	$(DB_CLI) $(DB_NAME) < sql/20_revert_clientes_cnpj_comment.sql

db-fix-revoked-reason: db-guard-producao db-check-env ## Adiciona refresh_tokens.revoked_reason (SEC-07) em bancos existentes (nao destrutivo, legado fica NULL)
	$(DB_CLI) $(DB_NAME) < sql/21_alter_refresh_tokens_revoked_reason.sql

db-revert-revoked-reason: db-guard-producao db-check-env ## Reverte db-fix-revoked-reason (remove a coluna; perde os motivos gravados)
	$(DB_CLI) $(DB_NAME) < sql/21_revert_refresh_tokens_revoked_reason.sql

db-fix-tokens-validos-desde: db-guard-producao db-check-env ## Adiciona usuarios.tokens_validos_desde (SEC-08) em bancos existentes (nao destrutivo, linhas existentes ficam NULL)
	$(DB_CLI) $(DB_NAME) < sql/22_alter_usuarios_tokens_validos_desde.sql

db-revert-tokens-validos-desde: db-guard-producao db-check-env ## Reverte db-fix-tokens-validos-desde (remove a coluna; perde os cortes de token gravados)
	$(DB_CLI) $(DB_NAME) < sql/22_revert_usuarios_tokens_validos_desde.sql

db-fix-reuso-detectado: db-guard-producao db-check-env ## Adiciona refresh_tokens.reuso_detectado_em (SEC-12) em bancos existentes (nao destrutivo, linhas existentes ficam NULL)
	$(DB_CLI) $(DB_NAME) < sql/23_alter_refresh_tokens_reuso_detectado_em.sql

db-revert-reuso-detectado: db-guard-producao db-check-env ## Reverte db-fix-reuso-detectado (remove a coluna; perde as marcas de reuso gravadas)
	$(DB_CLI) $(DB_NAME) < sql/23_revert_refresh_tokens_reuso_detectado_em.sql

db-fix-taxa-pct-check: db-guard-producao db-check-env ## Cria o CHECK chk_pagamentos_taxa_pct (0 a 30) em bancos existentes (SEC-27; aborta sem alterar dados se houver taxa_pct fora da faixa)
	$(DB_CLI) $(DB_NAME) < sql/24_alter_pagamentos_taxa_pct_check.sql

db-revert-taxa-pct-check: db-guard-producao db-check-env ## Reverte db-fix-taxa-pct-check (remove o CHECK; o teto de taxa_pct fica so no dominio)
	$(DB_CLI) $(DB_NAME) < sql/24_revert_pagamentos_taxa_pct_check.sql

db-fix-produtos-imagem: db-guard-producao db-check-env ## Adiciona produtos.imagem + CHECK chk_produtos_imagem + UNIQUE uk_produtos_imagem (IMG-01) em bancos existentes (nao destrutivo, linhas existentes ficam NULL)
	$(DB_CLI) $(DB_NAME) < sql/26_alter_produtos_imagem.sql

db-revert-produtos-imagem: db-guard-producao db-check-env ## Reverte db-fix-produtos-imagem (remove CHECK, UNIQUE e coluna; perde os vinculos produto->imagem, arquivos ficam orfaos)
	$(DB_CLI) $(DB_NAME) < sql/26_revert_produtos_imagem.sql

# IMG-01: gera um PNG por produto (nome na diagonal) pelo mesmo caso de uso do
# upload, no PRODUTO_IMAGENS_DIR (padrão backend/rotaperfumes-api/data/produtos-imagens).
# Flags extras: make db-gerar-imagens-produtos GERAR_IMAGENS_FLAGS=-sobrescrever
# (ou -dry-run para só simular).
GERAR_IMAGENS_FLAGS ?=
db-gerar-imagens-produtos: db-guard-producao db-check-env ## IMG-01: gera e grava a imagem PNG (nome na diagonal) dos produtos sem imagem (GERAR_IMAGENS_FLAGS=-sobrescrever troca todas)
	cd backend/rotaperfumes-api && go run ./cmd/gerarimagens -confirmar $(GERAR_IMAGENS_FLAGS)

db-seed-mes-atual: db-guard-producao db-check-env ## Gera movimentacoes de exemplo no mes corrente (pedidos/itens/pagamentos/oportunidades/visitas/estoque/carteiras; idempotente, refaz so as linhas do seed do mes)
	$(DB_CLI) $(DB_NAME) < sql/25_seed_movimentacoes_mes_atual.sql

db-import-clientes: db-guard-producao ## Importa dados/crm/clientes.csv para a tabela clientes (upsert idempotente)
	cd backend/shared && go run ./cmd/importclientes

db-import-produtos: db-guard-producao ## Importa dados/erp/produtos.csv para a tabela produtos (upsert idempotente)
	cd backend/shared && go run ./cmd/importprodutos

db-import-pedidos: db-guard-producao ## Importa dados/erp/pedidos.csv e itens_pedido.csv (upsert idempotente, nesta ordem)
	cd backend/shared && go run ./cmd/importpedidos

db-import-pagamentos: db-guard-producao ## Importa dados/erp/pagamentos.csv para a tabela pagamentos (upsert idempotente, depende de pedidos já importados)
	cd backend/shared && go run ./cmd/importpagamentos

db-import-carteiras: db-guard-producao ## Importa dados/crm/carteira.csv para a tabela carteiras (upsert idempotente, depende de clientes e vendedores já importados)
	cd backend/shared && go run ./cmd/importcarteiras

db-import-oportunidades: db-guard-producao ## Importa dados/crm/oportunidades.csv para a tabela oportunidades (upsert idempotente, depende de clientes e vendedores já importados)
	cd backend/shared && go run ./cmd/importoportunidades

db-import-visitas: db-guard-producao ## Importa dados/crm/visitas.csv para a tabela visitas (upsert idempotente, depende de clientes e vendedores já importados)
	cd backend/shared && go run ./cmd/importvisitas

db-import-estoque: db-guard-producao ## Importa dados/erp/estoque.csv para a tabela estoque (upsert idempotente, depende de produtos já importados)
	cd backend/shared && go run ./cmd/importestoque

# BUG-27: mesma cadeia do db-reset, seguida dos imports na ordem das
# dependencias, um sub-make por etapa.
db-rebuild: db-guard-producao db-guard-sem-producao db-guard-drop ## Recria o banco do zero e importa todos os dados (db-down && db-seed && db-import-*, em sequencia: BUG-27)
	$(MAKE) db-down && $(MAKE) db-seed && \
	$(MAKE) db-import-clientes && $(MAKE) db-import-produtos && \
	$(MAKE) db-import-pedidos && $(MAKE) db-import-pagamentos && \
	$(MAKE) db-import-carteiras && $(MAKE) db-import-oportunidades && \
	$(MAKE) db-import-visitas && $(MAKE) db-import-estoque

db-export: ## Exporta as tabelas do banco para CSV em export/crm e export/erp (mesmo formato de dados/)
	cd backend/shared && go run ./cmd/exportdados

# =============================================================================
# Build
# =============================================================================
build: build-api ## Build API única

build-shared:
	cd backend/shared && $(SEM_SEGREDOS) go build ./...

build-api: build-shared ## Compila a API unificada
	cd backend/rotaperfumes-api && $(SEM_SEGREDOS) go build -o ../../bin/rotaperfumes-api ./cmd/server

build-all: build

# =============================================================================
# Run (binários compilados)
# =============================================================================
# SEC-21: dev roda em HTTP, então só run-api e dev-api desligam o Secure dos
# cookies de sessão. "?=": o valor do .env ou do shell continua ganhando.
# Sem o make (go run ./cmd/server, binário, Docker) o padrão é true.
dev-api run-api: export COOKIE_SECURE ?= false

run-api: build-api ## Sobe a API na porta 8080 (COOKIE_SECURE=false se não definido)
	./bin/rotaperfumes-api

# =============================================================================
# Dev (go run direto)
# =============================================================================
dev-api: ## go run API (modo desenvolvimento; COOKIE_SECURE=false se não definido)
	cd backend/rotaperfumes-api && go run ./cmd/server

# =============================================================================
# Frontend dev
# =============================================================================
dev-frontend: frontend-deps ## npm run dev frontend
	cd frontend && $(SEM_SEGREDOS) npm run dev

frontend-deps: ## Instala dependências npm do frontend (roda uma vez)
	cd frontend && $(SEM_SEGREDOS) npm install

# =============================================================================
# Testes
# =============================================================================
test: test-shared test-api ## Roda todos os testes Go (requer Go local na versão do go.mod, com covdata: ver make setup-check / TST-08)

test-shared: $(GUARDA_INTEGRACAO) ## Testes do pacote compartilhado (em tests/, cobertura medida com -coverpkg=./...)
	cd backend/shared && $(SEM_SEGREDOS_TESTE) go test ./... $(GO_TEST_FLAGS) -v -coverpkg=./... -coverprofile=coverage.txt

cover-shared: $(GUARDA_INTEGRACAO) ## Cobertura do shared: total + relatório HTML (backend/shared/coverage.html)
	cd backend/shared && $(SEM_SEGREDOS_TESTE) go test ./... -coverpkg=./... -coverprofile=coverage.txt
	cd backend/shared && $(SEM_SEGREDOS) go tool cover -func=coverage.txt | tail -1
	cd backend/shared && $(SEM_SEGREDOS) go tool cover -html=coverage.txt -o coverage.html

test-api: $(GUARDA_INTEGRACAO) build-shared ## Testes da API (em tests/, cobertura medida com -coverpkg=./...)
	cd backend/rotaperfumes-api && $(SEM_SEGREDOS_TESTE) go test ./... $(GO_TEST_FLAGS) -v -coverpkg=./... -coverprofile=coverage.txt

cover-api: $(GUARDA_INTEGRACAO) build-shared ## Cobertura da API: total + relatório HTML (backend/rotaperfumes-api/coverage.html)
	cd backend/rotaperfumes-api && $(SEM_SEGREDOS_TESTE) go test ./... -coverpkg=./... -coverprofile=coverage.txt
	cd backend/rotaperfumes-api && $(SEM_SEGREDOS) go tool cover -func=coverage.txt | tail -1
	cd backend/rotaperfumes-api && $(SEM_SEGREDOS) go tool cover -html=coverage.txt -o coverage.html

arch-check: $(GUARDA_INTEGRACAO) ## ARQ-01: testes de arquitetura (dependências entre camadas, allowlists e AST, nos dois módulos)
	cd backend/shared && $(SEM_SEGREDOS) go test ./tests/arquitetura/...
	cd backend/rotaperfumes-api && $(SEM_SEGREDOS) go test ./tests/arquitetura/...

# ARQ-01 (§7.1, portão 4): o código migra da API para shared, então a
# cobertura é medida com os testes da API cobrindo também os pacotes de
# shared, somada à do módulo shared (blocos mesclados: um bloco conta como
# coberto se algum dos dois perfis o cobrir).
cover-combinada: $(GUARDA_INTEGRACAO) build-shared ## ARQ-01: cobertura combinada API+shared (-coverpkg cobrindo os dois módulos, blocos mesclados)
	cd backend/shared && $(SEM_SEGREDOS_TESTE) go test ./... -coverpkg=./... -coverprofile=coverage-combinada-shared.txt
	cd backend/rotaperfumes-api && $(SEM_SEGREDOS_TESTE) go test ./... -coverpkg=./...,github.com/rotaperfumes/shared/... -coverprofile=coverage-combinada-api.txt
	awk 'NR == 1 { print; next } FNR == 1 { next } { chave = $$1 " " $$2; if (!(chave in n) || $$3 > n[chave]) n[chave] = $$3 } END { for (c in n) print c, n[c] }' \
		backend/rotaperfumes-api/coverage-combinada-api.txt backend/shared/coverage-combinada-shared.txt > backend/rotaperfumes-api/coverage-combinada.txt
	cd backend/rotaperfumes-api && $(SEM_SEGREDOS) go tool cover -func=coverage-combinada.txt | tail -1

# Integração: além de INTEGRATION=1, os testes precisam das DB_* no ambiente
# (SEC-11). SEC-53: o sub-make recebe INTEGRATION=1 NA LINHA DE COMANDO, e só
# assim o $(SEM_SEGREDOS_TESTE) deixa as DB_* chegarem ao go test (MYSQL_PWD,
# PASSWORD_PEPPER, JWT_SECRET e SMTP_* nunca; go build/cover nunca, SEC-55).
# SEC-50: os testes recusam a produção antes de conectar
# (backend/shared/testkit/integracao), sem válvula. SEC-54: só loopback ou
# o CasaOS :3307 (db-guard-integracao / integracao.Permitidos). Os pacotes usam o mesmo banco: GO_TEST_FLAGS="-p 1 -count=1"
# (um pacote por vez, sem cache). Banco descartável fora do .env:
#   make test-integration ENV_ARQUIVO=integracao.env
# (ver docs/arquitetura-ddd.md §10.9).
test-integration: db-guard-producao db-guard-integracao ## Roda testes de integração com -p 1 (requer MariaDB descartável e DB_*; só loopback ou CasaOS :3307, SEC-54; ver docs/arquitetura-ddd.md §10.9)
	$(MAKE) test INTEGRATION=1 GO_TEST_FLAGS="-p 1 -count=1"

test-frontend:
	cd frontend && $(SEM_SEGREDOS) npm test

lint: ## Vet em todos os módulos
	cd backend/shared && $(SEM_SEGREDOS) go vet ./...
	cd backend/rotaperfumes-api && $(SEM_SEGREDOS) go vet ./...

# =============================================================================
# Dependências
# =============================================================================
deps: ## Instala dependências Go
	cd backend/shared && $(SEM_SEGREDOS) go get ./... && $(SEM_SEGREDOS) go mod tidy
	cd backend/rotaperfumes-api && $(SEM_SEGREDOS) go get ./... && $(SEM_SEGREDOS) go mod tidy

# SEC-13: política de atualização — rodar "make deps-audit" periodicamente
# (ex.: mensalmente e antes de cada deploy). Só reporta; não atualiza nada.
GO_MODULES = backend/shared backend/rotaperfumes-api

deps-audit: deps-audit-go deps-audit-frontend deps-audit-imagens ## SEC-13: audita dependências Go + npm (vulnerabilidades e desatualizadas) e os digests das imagens (SEC-31)

deps-audit-go: ## SEC-13: lista módulos Go desatualizados e roda govulncheck (falha se houver vulnerabilidade alcançável)
	@status=0; for m in $(GO_MODULES); do \
		echo "=== $$m: módulos com atualização disponível ==="; \
		(cd $$m && $(SEM_SEGREDOS) go list -m -u all | grep '\[' || echo "(nenhum)"); \
		echo "=== $$m: govulncheck ==="; \
		if command -v govulncheck >/dev/null 2>&1; then \
			(cd $$m && $(SEM_SEGREDOS) govulncheck ./...) || status=1; \
		else \
			echo "govulncheck não encontrado: veja a seção de auditoria de dependências no CLAUDE.md"; \
			status=1; \
		fi; \
	done; exit $$status

deps-audit-frontend: ## SEC-13: npm audit (falha em severidade high+) e npm outdated (só informativo)
	@echo "=== frontend: npm outdated ==="
	cd frontend && $(SEM_SEGREDOS) npm outdated || true
	@echo "=== frontend: npm audit --omit=dev --audit-level=high ==="
	cd frontend && $(SEM_SEGREDOS) npm audit --omit=dev --audit-level=high

# SEC-31: imagens fixadas por digest (tag@sha256:...) no Jenkinsfile e no
# deploy/mariadb-docker.sh. Só informa: para cada referência, compara o digest
# fixado com o digest atual da mesma tag e o da tag sem o patch (ex.: 11.4,
# 1.26-alpine) no Docker Hub (docker buildx imagetools ou, sem docker, curl ao
# registry com token anônimo). Nunca falha. Para atualizar: trocar a string
# nos dois arquivos (mesmo digest do mariadb), make test, backup e
# sudo bash deploy/mariadb-docker.sh --recriar (ver comentário no Jenkinsfile).
IMAGENS_FIXADAS_EM = Jenkinsfile deploy/mariadb-docker.sh deploy/seed-producao.sh
IMAGEM_ACCEPT = application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json

deps-audit-imagens: ## SEC-31: compara os digests fixados (Jenkinsfile: go, node e mariadb; mariadb-docker.sh) com os das tags atuais no Docker Hub (informativo, nunca falha)
	@digest_remoto() { \
	  if command -v docker >/dev/null 2>&1 && docker buildx version >/dev/null 2>&1; then \
	    docker buildx imagetools inspect "$$1:$$2" --format '{{.Manifest.Digest}}' 2>/dev/null && return; \
	  fi; \
	  command -v curl >/dev/null 2>&1 || return 1; \
	  case "$$1" in */*) repo="$$1" ;; *) repo="library/$$1" ;; esac; \
	  tok=$$(curl -fsS --max-time 15 "https://auth.docker.io/token?service=registry.docker.io&scope=repository:$$repo:pull" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p'); \
	  [ -n "$$tok" ] || return 1; \
	  curl -fsSI --max-time 15 -H "Authorization: Bearer $$tok" -H "Accept: $(IMAGEM_ACCEPT)" \
	    "https://registry-1.docker.io/v2/$$repo/manifests/$$2" | tr -d '\r' | awk -F': ' 'tolower($$1)=="docker-content-digest"{print $$2}' | grep -E '^sha256:[0-9a-f]{64}$$'; \
	}; \
	refs=$$(grep -hoE '[a-z0-9._/-]+:[A-Za-z0-9._-]+@sha256:[0-9a-f]{64}' $(IMAGENS_FIXADAS_EM) 2>/dev/null | sort -u); \
	[ -n "$$refs" ] || { echo "AVISO (SEC-31): nenhuma imagem fixada por digest em $(IMAGENS_FIXADAS_EM)"; exit 0; }; \
	command -v docker >/dev/null 2>&1 || echo "AVISO (SEC-31): docker nao encontrado; consultando o registry por curl"; \
	for ref in $$refs; do \
	  img=$${ref%%:*}; resto=$${ref#*:}; tag=$${resto%%@*}; fixado=$${ref#*@}; \
	  flutuante=$$(printf '%s' "$$tag" | sed -E 's/^([0-9]+\.[0-9]+)\.[0-9]+/\1/'); \
	  echo "=== $$img:$$tag (fixado $$fixado) ==="; \
	  for t in $$tag $$flutuante; do \
	    atual=$$(digest_remoto "$$img" "$$t") || atual=""; \
	    if [ -z "$$atual" ]; then echo "  $$img:$$t: AVISO: nao foi possivel consultar o digest (sem docker/curl ou sem rede)"; \
	    elif [ "$$atual" = "$$fixado" ]; then echo "  $$img:$$t: igual ao fixado"; \
	    else echo "  $$img:$$t: DIFERENTE: atual $$atual (atualize: ver SEC-31 no Jenkinsfile)"; fi; \
	    [ "$$t" = "$$flutuante" ] && break; \
	  done; \
	done; exit 0

# =============================================================================
# Seed de usuários (gera hash argon2id com PASSWORD_PEPPER)
# =============================================================================
gen-hash: db-guard-producao ## aplica os seeds com hash gerado (sem alterar sql/)
	cd backend/shared && go run ./cmd/seedusers

reset-senhas-simular: ## OPS-01: lista os usuários ativos que o reset em massa atingiria (não grava)
	cd backend/rotaperfumes-api && go run ./cmd/resetsenhas

reset-senhas-ativos: db-guard-producao ## OPS-01: nova senha Argon2id + e-mail para TODOS os usuários ativos (exige SMTP)
	cd backend/rotaperfumes-api && go run ./cmd/resetsenhas -executar

# DEPLOY-07: sem senha fixa. A senha e pedida no terminal (sem eco, com
# confirmacao); no Git Bash rode o comando via winpty. O admin fica com
# deve_trocar_senha = 1 (troca obrigatoria no 1o login).
fix-hash: db-guard-producao ## Lista usuários e corrige TODOS os PLACEHOLDER + cria admin (se faltar); senha pedida no terminal
	cd backend/shared && go run ./cmd/resetpassword -list
	@echo ""
	@echo "=== Corrigindo todos os PLACEHOLDER (senha dos usuarios) ==="
	cd backend/shared && go run ./cmd/resetpassword -all-users -password-prompt
	@echo "=== Criando/atualizando o admin (senha do admin) ==="
	cd backend/shared && go run ./cmd/resetpassword -create-admin -password-prompt
	@echo ""
	@echo "=== CORRIGIDO! Admin: admin@rotaperfumes.com.br (senha digitada acima; troca obrigatoria no 1o login) ==="

fix-admin: db-guard-producao ## Garante que admin existe (cria se não existir); senha pedida no terminal, troca obrigatoria no 1o login
	cd backend/shared && go run ./cmd/resetpassword -create-admin -password-prompt

# =============================================================================
# Audit
# =============================================================================
audit: ## Gera resumo de linhas de código
	@echo "=== Linhas de código Go ==="
	@find backend -name "*.go" -exec wc -l {} + | tail -1
	@echo "=== Linhas de SQL ==="
	@find sql -name "*.sql" -exec wc -l {} + | tail -1 || true

# =============================================================================
# Limpeza
# =============================================================================
stop-api: ## Para o processo da API (Windows)
	-taskkill /F /IM rotaperfumes-api.exe 2>nul || true
