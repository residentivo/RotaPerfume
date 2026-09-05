.PHONY: help db-up db-down db-seed db-reset db-create test test-all lint \
	build build-api run-api dev-api stop-api \
	test-api gen-hash \
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
	@echo "=== Aplicando DDL Users ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/01_ddl_users.sql

db-seed: db-up ## Cria o schema e carrega todos os dados
	@echo "=== Carregando usuários ==="
	mysql $(MYSQL_OPTS) $(DB_NAME) < sql/02_seed_users.sql
	@echo "=== Seed completo! ==="

db-down: ## Dropa o banco de dados (CUIDADO!)
	mysql $(MYSQL_OPTS) -e "DROP DATABASE IF EXISTS $(DB_NAME);"

db-reset: db-down db-seed ## Recria o banco do zero

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
dev-frontend: ## npm run dev frontend (configurar conforme projeto)
	cd frontend && npm run dev

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
gen-hash: ## Gera hash bcrypt para o SEED_DEFAULT_PASSWORD
	cd apis/shared && go run ./cmd/seedusers

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
