.PHONY: help reset-senhas-simular reset-senhas-ativos db-check-env db-up db-down db-seed db-reset db-create db-fix-deve-trocar-senha db-fix-tipo-reset db-fix-cnpj-unique db-revert-cnpj-unique db-fix-cnpj-comment db-revert-cnpj-comment db-fix-revoked-reason db-revert-revoked-reason db-fix-tokens-validos-desde db-revert-tokens-validos-desde db-fix-reuso-detectado db-revert-reuso-detectado db-fix-estoque-origem db-revert-estoque-origem db-rebuild db-export db-import-clientes db-import-produtos db-import-pedidos db-import-pagamentos db-import-carteiras db-import-oportunidades db-import-visitas db-import-estoque test test-all lint \
	build build-api run-api dev-api stop-api \
	test-api cover-api test-shared cover-shared gen-hash fix-hash \
	frontend-deps \
	audit

# =============================================================================
# Helpers
# =============================================================================
# SEC-11: as credenciais do banco vêm do .env (sem default).
# Precedência: com "-include .env", o valor do .env ganha da variável do
# shell; "make DB_SENHA=x" na linha de comando ganha de tudo.
# Formato do .env: DB_SENHA sem "$", "#", aspas, espaços nas pontas e sem
# começar com "/" (o make e o Git Bash interpretariam esses caracteres).
-include .env

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

# Senha só pelo ambiente (lida nativamente pelo mysql/mysql.exe), nunca no argv.
MYSQL_PWD = $(DB_SENHA)
# Git Bash/MSYS: impede a conversão de caminho POSIX no valor da senha.
MSYS2_ENV_CONV_EXCL = MYSQL_PWD
export

MYSQL_OPTS = --local-infile=1 -u $(DB_USUARIO) -h $(DB_HOST) -P $(DB_PORT) --default-character-set=utf8mb4

help: ## Mostra esta ajuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# =============================================================================
# Banco de dados
# =============================================================================
# SEC-11: pré-requisito de todo alvo que chama o mysql (db-create, db-down,
# db-fix-*, db-revert-*; db-up/db-seed herdam via db-create). Fica depois do
# "help" para não virar o alvo padrão do make.
db-check-env: ## Confere se DB_USUARIO/DB_SENHA estão definidos no .env (SEC-11)
	@test -n "$$DB_USUARIO" -a -n "$$MYSQL_PWD" || { echo "SEC-11: defina DB_USUARIO/DB_SENHA no .env"; exit 1; }

db-create: db-check-env ## Cria o banco de dados se não existir
	mysql $(MYSQL_OPTS) -e "CREATE DATABASE IF NOT EXISTS $(DB_NAME) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

db-up: db-create ## Cria o schema (tabelas vazias)
	@echo "=== Aplicando DDL de usuarios (vendedores + usuarios) ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/01_ddl_usuarios.sql
	@echo "=== Aplicando DDL de refresh_tokens ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/06_ddl_refresh_tokens.sql
	@echo "=== Aplicando DDL de senha_historico ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/07_ddl_senha_historico.sql
	@echo "=== Aplicando DDL de clientes ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/09_ddl_clientes.sql
	@echo "=== Aplicando DDL de produtos ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/10_ddl_produtos.sql
	@echo "=== Aplicando DDL de pedidos (depende de clientes + vendedores) ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/04_ddl_pedidos.sql
	@echo "=== Aplicando DDL de itens_pedido (depende de pedidos + produtos) ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/11_ddl_itens_pedido.sql
	@echo "=== Aplicando DDL de pagamentos (depende de pedidos) ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/12_ddl_pagamentos.sql
	@echo "=== Aplicando DDL de carteiras (depende de clientes + vendedores) ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/14_ddl_carteiras.sql
	@echo "=== Aplicando DDL de oportunidades (depende de clientes + vendedores) ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/15_ddl_oportunidades.sql
	@echo "=== Aplicando DDL de visitas (depende de clientes + vendedores) ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/16_ddl_visitas.sql
	@echo "=== Aplicando DDL de estoque (depende de produtos; ja sem a coluna origem da migracao 18) ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/17_ddl_estoque.sql

db-seed: db-up ## Cria o schema, carrega dados e corrige hashes
	@echo "=== Seed: admin principal ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/02_seed_admin.sql
	@echo "=== Seed: usuarios dos 42 vendedores ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/03_seed_vendedores.sql
	@echo ""
	@echo "=== Corrigindo hashes (placeholder -> argon2id real) + criando admin ==="
	cd apis/shared && go run ./cmd/resetpassword -list
	cd apis/shared && go run ./cmd/resetpassword -create-admin
	cd apis/shared && go run ./cmd/resetpassword -all-users
	@echo ""
	@echo "=== Seed completo com hashes validos! ==="
	@echo "=== Admin: admin@rotaperfumes.com.br / Admin@123 ==="

db-down: db-check-env ## Dropa o banco de dados (CUIDADO!)
	mysql $(MYSQL_OPTS) -e "DROP DATABASE IF EXISTS $(DB_NAME);"

db-reset: db-down db-seed ## Recria o banco do zero com hashes validos

db-fix-deve-trocar-senha: db-check-env ## Adiciona a coluna deve_trocar_senha em bancos existentes (nao destrutivo, sem apagar dados)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/08_alter_usuarios_deve_trocar_senha.sql

db-fix-tipo-reset: db-check-env ## Corrige o ENUM de senha_historico.tipo_reset em bancos existentes (nao destrutivo, sem apagar dados)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/13_alter_senha_historico_tipo_reset.sql

db-fix-estoque-origem: db-check-env ## Remove estoque.origem (migracao 18) em bancos criados com o 17 antigo (idempotente; perde o rastreio de origem)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/18_alter_estoque_drop_origem.sql

db-revert-estoque-origem: db-check-env ## Reverte db-fix-estoque-origem (recria estoque.origem; linhas existentes voltam como import_csv)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/18_revert_estoque_drop_origem.sql

db-fix-cnpj-unique: db-check-env ## Unifica clientes com CNPJ duplicado no menor id e cria UNIQUE uq_clientes_cnpj (APAGA as copias; backup em clientes_merge_backup_20260925*)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/19_alter_clientes_cnpj_unique.sql

db-revert-cnpj-unique: db-check-env ## Reverte db-fix-cnpj-unique a partir das tabelas clientes_merge_backup_20260925*
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/19_revert_clientes_cnpj_unique.sql

db-fix-cnpj-comment: db-check-env ## Atualiza o COMMENT de clientes.cnpj para o formato alfanumerico (so metadado, sem apagar dados)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/20_alter_clientes_cnpj_comment.sql

db-revert-cnpj-comment: db-check-env ## Reverte db-fix-cnpj-comment (volta o COMMENT antigo "somente digitos")
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/20_revert_clientes_cnpj_comment.sql

db-fix-revoked-reason: db-check-env ## Adiciona refresh_tokens.revoked_reason (SEC-07) em bancos existentes (nao destrutivo, legado fica NULL)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/21_alter_refresh_tokens_revoked_reason.sql

db-revert-revoked-reason: db-check-env ## Reverte db-fix-revoked-reason (remove a coluna; perde os motivos gravados)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/21_revert_refresh_tokens_revoked_reason.sql

db-fix-tokens-validos-desde: db-check-env ## Adiciona usuarios.tokens_validos_desde (SEC-08) em bancos existentes (nao destrutivo, linhas existentes ficam NULL)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/22_alter_usuarios_tokens_validos_desde.sql

db-revert-tokens-validos-desde: db-check-env ## Reverte db-fix-tokens-validos-desde (remove a coluna; perde os cortes de token gravados)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/22_revert_usuarios_tokens_validos_desde.sql

db-fix-reuso-detectado: db-check-env ## Adiciona refresh_tokens.reuso_detectado_em (SEC-12) em bancos existentes (nao destrutivo, linhas existentes ficam NULL)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/23_alter_refresh_tokens_reuso_detectado_em.sql

db-revert-reuso-detectado: db-check-env ## Reverte db-fix-reuso-detectado (remove a coluna; perde as marcas de reuso gravadas)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/23_revert_refresh_tokens_reuso_detectado_em.sql

db-import-clientes: ## Importa dados/crm/clientes.csv para a tabela clientes (upsert idempotente)
	cd apis/shared && go run ./cmd/importclientes

db-import-produtos: ## Importa dados/erp/produtos.csv para a tabela produtos (upsert idempotente)
	cd apis/shared && go run ./cmd/importprodutos

db-import-pedidos: ## Importa dados/erp/pedidos.csv e itens_pedido.csv (upsert idempotente, nesta ordem)
	cd apis/shared && go run ./cmd/importpedidos

db-import-pagamentos: ## Importa dados/erp/pagamentos.csv para a tabela pagamentos (upsert idempotente, depende de pedidos já importados)
	cd apis/shared && go run ./cmd/importpagamentos

db-import-carteiras: ## Importa dados/crm/carteira.csv para a tabela carteiras (upsert idempotente, depende de clientes e vendedores já importados)
	cd apis/shared && go run ./cmd/importcarteiras

db-import-oportunidades: ## Importa dados/crm/oportunidades.csv para a tabela oportunidades (upsert idempotente, depende de clientes e vendedores já importados)
	cd apis/shared && go run ./cmd/importoportunidades

db-import-visitas: ## Importa dados/crm/visitas.csv para a tabela visitas (upsert idempotente, depende de clientes e vendedores já importados)
	cd apis/shared && go run ./cmd/importvisitas

db-import-estoque: ## Importa dados/erp/estoque.csv para a tabela estoque (upsert idempotente, depende de produtos já importados)
	cd apis/shared && go run ./cmd/importestoque

db-rebuild: db-down db-seed db-import-clientes db-import-produtos db-import-pedidos db-import-pagamentos db-import-carteiras db-import-oportunidades db-import-visitas db-import-estoque ## Recria o banco do zero e importa todos os dados

db-export: ## Exporta as tabelas do banco para CSV em export/crm e export/erp (mesmo formato de dados/)
	cd apis/shared && go run ./cmd/exportdados

# =============================================================================
# Build
# =============================================================================
build: build-api ## Build API única

build-shared:
	cd apis/shared && go build ./...

build-api: build-shared ## Compila a API unificada
	cd apis/rotaperfumes-api && go build -o ../../bin/rotaperfumes-api ./cmd/server

build-all: build

# =============================================================================
# Run (binários compilados)
# =============================================================================
run-api: build-api ## Sobe a API na porta 8080
	./bin/rotaperfumes-api

# =============================================================================
# Dev (go run direto)
# =============================================================================
dev-api: ## go run API (modo desenvolvimento)
	cd apis/rotaperfumes-api && go run ./cmd/server

# =============================================================================
# Frontend dev
# =============================================================================
dev-frontend: frontend-deps ## npm run dev frontend
	cd frontend && npm run dev

frontend-deps: ## Instala dependências npm do frontend (roda uma vez)
	cd frontend && npm install

# =============================================================================
# Testes
# =============================================================================
test: test-shared test-api ## Roda todos os testes Go

test-shared: ## Testes do pacote compartilhado (em tests/, cobertura medida com -coverpkg=./...)
	cd apis/shared && go test ./... -v -coverpkg=./... -coverprofile=coverage.txt

cover-shared: ## Cobertura do shared: total + relatório HTML (apis/shared/coverage.html)
	cd apis/shared && go test ./... -coverpkg=./... -coverprofile=coverage.txt
	cd apis/shared && go tool cover -func=coverage.txt | tail -1
	cd apis/shared && go tool cover -html=coverage.txt -o coverage.html

test-api: build-shared ## Testes da API (em tests/, cobertura medida com -coverpkg=./...)
	cd apis/rotaperfumes-api && go test ./... -v -coverpkg=./... -coverprofile=coverage.txt

cover-api: build-shared ## Cobertura da API: total + relatório HTML (apis/rotaperfumes-api/coverage.html)
	cd apis/rotaperfumes-api && go test ./... -coverpkg=./... -coverprofile=coverage.txt
	cd apis/rotaperfumes-api && go tool cover -func=coverage.txt | tail -1
	cd apis/rotaperfumes-api && go tool cover -html=coverage.txt -o coverage.html

test-integration: ## Roda testes de integração (requer DB)
	INTEGRATION=1 $(MAKE) test

test-frontend:
	cd frontend && npm test

lint: ## Vet em todos os módulos
	cd apis/shared && go vet ./...
	cd apis/rotaperfumes-api && go vet ./...

# =============================================================================
# Dependências
# =============================================================================
deps: ## Instala dependências Go
	cd apis/shared && go get ./... && go mod tidy
	cd apis/rotaperfumes-api && go get ./... && go mod tidy

# =============================================================================
# Seed de usuários (gera hash argon2id com PASSWORD_PEPPER)
# =============================================================================
gen-hash: ## aplica os seeds com hash gerado (sem alterar sql/)
	cd apis/shared && go run ./cmd/seedusers

reset-senhas-simular: ## OPS-01: lista os usuários ativos que o reset em massa atingiria (não grava)
	cd apis/rotaperfumes-api && go run ./cmd/resetsenhas

reset-senhas-ativos: ## OPS-01: nova senha Argon2id + e-mail para TODOS os usuários ativos (exige SMTP)
	cd apis/rotaperfumes-api && go run ./cmd/resetsenhas -executar

fix-hash: ## Lista usuários e corrige TODOS os PLACEHOLDER + cria admin (se faltar)
	cd apis/shared && go run ./cmd/resetpassword -list
	@echo.
	@echo "=== Corrigindo todos os PLACEHOLDER + criando admin ==="
	cd apis/shared && go run ./cmd/resetpassword -all-users -password=Admin@123
	cd apis/shared && go run ./cmd/resetpassword -create-admin -password=Admin@123
	@echo.
	@echo "=== CORRIGIDO! Credenciais: admin@rotaperfumes.com.br / Admin@123 ==="

fix-admin: ## Garante que admin existe (cria se não existir) com senha Admin@123
	cd apis/shared && go run ./cmd/resetpassword -create-admin -password=Admin@123

# =============================================================================
# Audit
# =============================================================================
audit: ## Gera resumo de linhas de código
	@echo "=== Linhas de código Go ==="
	@find apis -name "*.go" -exec wc -l {} + | tail -1
	@echo "=== Linhas de SQL ==="
	@find sql -name "*.sql" -exec wc -l {} + | tail -1 || true

# =============================================================================
# Limpeza
# =============================================================================
stop-api: ## Para o processo da API (Windows)
	-taskkill /F /IM rotaperfumes-api.exe 2>nul || true
