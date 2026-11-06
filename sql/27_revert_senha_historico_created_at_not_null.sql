-- ============================================================
-- REVERT: desfaz sql/27_alter_senha_historico_created_at_not_null.sql (DB-09)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-11-05
-- Agente: DataBrain (🌸)
--
-- Volta senha_historico.created_at a aceitar NULL. Nenhum dado é alterado: se o
-- remédio manual do 27 foi usado, as linhas continuam com a sentinela
-- 1970-01-01 00:00:01 (o NULL original não é restaurado).
--
-- ATENÇÃO: o sql/07_ddl_senha_historico.sql já nasce com NOT NULL. Este revert
-- também vale para bancos criados pelo 07 novo.
--
-- Idempotente: MODIFY COLUMN com a definição já aplicada não muda nada.
--
-- Uso:
--   make db-revert-historico-created-at
--   ou
--   MYSQL_PWD="$DB_SENHA" mariadb -u $DB_USUARIO -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/27_revert_senha_historico_created_at_not_null.sql
-- ============================================================

SET NAMES utf8mb4;

ALTER TABLE senha_historico MODIFY COLUMN
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP;
