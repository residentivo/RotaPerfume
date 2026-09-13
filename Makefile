.PHONY: help db-up db-down db-seed db-reset db-create db-fix-deve-trocar-senha test test-all lint \
	build build-api run-api dev-api stop-api \
	test-api gen-hash fix-hash \
	frontend-deps \
	audit

# =============================================================================
# Helpers
# =============================================================================
MYSQL_OPTS=--local-infile=1 -u $(DB_USUARIO) -p$(DB_SENHA) -h $(DB_HOST) -P $(DB_PORT) --default-character-set=utf8mb4
DB_NAME?=rotaperfumes
DB_USUARIO?=golang
DB_SENHA?=golang
DB_HOST?=localhost
DB_PORT?=3306
export

help: ## Mostra esta ajuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# =============================================================================
# Banco de dados
# =============================================================================
db-create: ## Cria o banco de dados se não existir
	mysql $(MYSQL_OPTS) -e "CREATE DATABASE IF NOT EXISTS $(DB_NAME) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

db-up: db-create ## Cria o schema (tabelas vazias)
	@echo "=== Aplicando DDL de usuarios (vendedores + usuarios) ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/01_ddl_usuarios.sql
	@echo "=== Aplicando DDL de pedidos ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/04_ddl_pedidos.sql
	@echo "=== Aplicando DDL de refresh_tokens ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/06_ddl_refresh_tokens.sql
	@echo "=== Aplicando DDL de senha_historico ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/07_ddl_senha_historico.sql

db-seed: db-up ## Cria o schema, carrega dados e corrige hashes
	@echo "=== Seed: admin principal ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/02_seed_admin.sql
	@echo "=== Seed: usuarios dos 42 vendedores ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/03_seed_vendedores.sql
	@echo "=== Seed: pedidos ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/05_seed_pedidos.sql
	@echo ""
	@echo "=== Corrigindo hashes (placeholder -> bcrypt real) + criando admin ==="
	cd apis/shared && go run ./cmd/resetpassword -list
	cd apis/shared && go run ./cmd/resetpassword -create-admin
	cd apis/shared && go run ./cmd/resetpassword -all-users
	@echo ""
	@echo "=== Seed completo com hashes validos! ==="
	@echo "=== Admin: admin@rotaperfumes.com.br / Admin@123 ==="

db-down: ## Dropa o banco de dados (CUIDADO!)
	mysql $(MYSQL_OPTS) -e "DROP DATABASE IF EXISTS $(DB_NAME);"

db-reset: db-down db-seed ## Recria o banco do zero com hashes validos

db-fix-deve-trocar-senha: ## Adiciona a coluna deve_trocar_senha em bancos existentes (nao destrutivo, sem apagar dados)
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/08_alter_usuarios_deve_trocar_senha.sql

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

test-shared: ## Testes do pacote compartilhado
	cd apis/shared && go test ./... -v

test-api: build-shared ## Testes da API
	cd apis/rotaperfumes-api && go test ./... -v

test-integration: ## Roda testes de integração (requer DB)
	INTEGRATION=1 $(MAKE) test

test-frontend:
	cd frontend && npm test -- --passWithNoTests || true

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
# Seed de usuários (gera bcrypt hash)
# =============================================================================
gen-hash: ## Gera hash bcrypt e atualiza SQLs de seed (para db-reset limpo)
	cd apis/shared && go run ./cmd/seedusers

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
