-- ============================================================
-- ALTER: corrige ENUM de senha_historico.tipo_reset (bancos já existentes)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-14
-- Agente: MegaBrain (🤍)
--
-- Bug: `sql/07_ddl_senha_historico.sql` criava a coluna `tipo_reset` com
-- ENUM('proprio','admin','primeiro_login'), mas o código Go
-- (services/senha_historico_service.go) sempre grava os valores
-- "usuario", "admin", "primeiro_acesso" ou "esquecimento". Qualquer valor
-- fora do ENUM causa erro 1265 (Data truncated for column 'tipo_reset')
-- ao registrar o histórico, o que travava a tela de troca de senha
-- obrigatória (ex: primeiro acesso de vendedor).
--
-- `sql/07_ddl_senha_historico.sql` já foi corrigido para instalações
-- novas (via `make db-reset`). Este script é para quem já tinha o banco
-- criado ANTES dessa correção e não quer/pode rodar `make db-reset`
-- (que apaga todos os dados).
--
-- Uso:
--   mysql --local-infile=1 -u $DB_USUARIO -p$DB_SENHA -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/13_alter_senha_historico_tipo_reset.sql
-- ============================================================

SET NAMES utf8mb4;

ALTER TABLE `senha_historico`
    MODIFY COLUMN `tipo_reset` ENUM('usuario','admin','primeiro_acesso','esquecimento') NOT NULL;
