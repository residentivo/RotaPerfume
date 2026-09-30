-- ============================================================
-- REVERT: desfaz sql/23_alter_refresh_tokens_reuso_detectado_em.sql (SEC-12)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-27
-- Agente: DataBrain (🌸)
--
-- Remove a coluna refresh_tokens.reuso_detectado_em. ATENÇÃO: as marcas de
-- reuso gravadas são perdidas, e a supressão de corte repetido por token
-- deixa de existir. Antes de reverter, volte o backend para uma versão que
-- não lê/grava `reuso_detectado_em` (senão o refresh com reuso falha).
--
-- Idempotente: só remove a coluna se ela existir.
--
-- Uso:
--   make db-revert-reuso-detectado
--   ou
--   MYSQL_PWD="$DB_SENHA" mysql --local-infile=1 -u $DB_USUARIO -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/23_revert_refresh_tokens_reuso_detectado_em.sql
-- ============================================================

SET NAMES utf8mb4;

SET @coluna_existe = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'refresh_tokens'
      AND COLUMN_NAME = 'reuso_detectado_em'
);

SET @ddl = IF(
    @coluna_existe = 1,
    'ALTER TABLE `refresh_tokens` DROP COLUMN `reuso_detectado_em`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
