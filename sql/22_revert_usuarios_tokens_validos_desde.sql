-- ============================================================
-- REVERT: desfaz sql/22_alter_usuarios_tokens_validos_desde.sql (SEC-08)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-26
-- Agente: DataBrain (🌸)
--
-- Remove a coluna usuarios.tokens_validos_desde. ATENÇÃO: os cortes de
-- access token gravados são perdidos, ou seja, access tokens emitidos antes
-- de uma troca de senha/inativação voltam a ser aceitos até expirarem. Antes
-- de reverter, volte o backend para uma versão que não lê/grava
-- `tokens_validos_desde`.
--
-- Idempotente: só remove a coluna se ela existir.
--
-- Uso:
--   mysql --local-infile=1 -u $DB_USUARIO -p$DB_SENHA -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/22_revert_usuarios_tokens_validos_desde.sql
-- ============================================================

SET NAMES utf8mb4;

SET @coluna_existe = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'usuarios'
      AND COLUMN_NAME = 'tokens_validos_desde'
);

SET @ddl = IF(
    @coluna_existe = 1,
    'ALTER TABLE `usuarios` DROP COLUMN `tokens_validos_desde`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
