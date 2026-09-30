-- ============================================================
-- ALTER: adiciona refresh_tokens.reuso_detectado_em (bancos já existentes)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-27
-- Card: SEC-12 (Lote 12), especificado pelo SecBrain (🟣)
-- Agente: DataBrain (🌸)
--
-- Contexto: quando um refresh token já rotacionado (revoked_reason =
-- 'rotacao') é reapresentado fora da janela de tolerância, o backend corta
-- todas as sessões do usuário. Como o token reusado continua com
-- revoked_reason = 'rotacao', cada novo replay repetia o corte e derrubava a
-- vítima de novo, mesmo depois do re-login. Esta coluna guarda, POR TOKEN, o
-- instante do último reuso detectado; o backend só corta de novo se ela for
-- NULL ou mais antiga que a janela de supressão (padrão 30 min), via UPDATE
-- condicional atômico pela PK.
--
-- Coluna nova:
--   `reuso_detectado_em` DATETIME NULL DEFAULT NULL
--     - NULL  = nenhum reuso detectado para este token.
--     - valor = instante do último reuso que gerou corte de sessões.
--   Posição: logo após `revoked_reason`.
--
-- Dados: nenhuma linha é alterada. Linhas existentes ficam NULL.
--
-- Índice: NÃO criado. O UPDATE é feito pela PK (`id`).
--
-- Bancos novos: sql/06_ddl_refresh_tokens.sql já cria a coluna. Como o 06 usa
-- CREATE TABLE IF NOT EXISTS, db-up/db-reset não conflitam com esta migração.
-- Esta migração não faz parte do db-up/db-seed/db-reset (mesmo padrão da
-- 08/13/18/19/20/21/22): é correção avulsa para bancos criados antes do SEC-12.
--
-- Idempotente: só adiciona a coluna se ela ainda não existir (evita 1060
-- Duplicate column name ao rodar duas vezes).
-- Reversão: sql/23_revert_refresh_tokens_reuso_detectado_em.sql
--           (make db-revert-reuso-detectado)
--
-- Uso:
--   make db-fix-reuso-detectado
--   ou
--   MYSQL_PWD="$DB_SENHA" mysql --local-infile=1 -u $DB_USUARIO -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/23_alter_refresh_tokens_reuso_detectado_em.sql
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
    @coluna_existe = 0,
    'ALTER TABLE `refresh_tokens` ADD COLUMN `reuso_detectado_em` DATETIME NULL DEFAULT NULL COMMENT \'Último reuso detectado deste token já rotacionado (SEC-12); NULL = nunca\' AFTER `revoked_reason`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
