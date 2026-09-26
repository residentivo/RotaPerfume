-- ============================================================
-- REVERT: desfaz sql/21_alter_refresh_tokens_revoked_reason.sql (SEC-07)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-26
-- Agente: DataBrain (🌸)
--
-- Remove a coluna refresh_tokens.revoked_reason. ATENÇÃO: os motivos de
-- revogação gravados depois da migração 21 são perdidos (revoked_at e os
-- demais dados continuam intactos). Antes de reverter, volte o backend para
-- uma versão que não lê/grava `revoked_reason`.
--
-- Idempotente: só remove a coluna se ela existir.
--
-- Uso:
--   mysql --local-infile=1 -u $DB_USUARIO -p$DB_SENHA -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/21_revert_refresh_tokens_revoked_reason.sql
-- ============================================================

SET NAMES utf8mb4;

SET @coluna_existe = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'refresh_tokens'
      AND COLUMN_NAME = 'revoked_reason'
);

SET @ddl = IF(
    @coluna_existe = 1,
    'ALTER TABLE `refresh_tokens` DROP COLUMN `revoked_reason`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
