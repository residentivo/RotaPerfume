-- ============================================================
-- ALTER: adiciona refresh_tokens.revoked_reason (bancos já existentes)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-26
-- Card: SEC-07 (Lote 6), opção B do SecBrain (🟣)
-- Agente: DataBrain (🌸)
--
-- Contexto: hoje `revoked_at` só diz QUE o token foi revogado, não POR QUÊ.
-- O backend precisa distinguir rotação normal (refresh) de logout e de
-- revogação em massa (troca de senha, inativação do usuário etc.), por
-- exemplo para detectar reuso de refresh token já rotacionado.
--
-- Coluna nova:
--   `revoked_reason` ENUM('rotacao','logout','revogacao_massa','senha','inativacao') NULL
--     - NULL  = token ativo (revoked_at IS NULL) OU revogado antes desta
--               migração (legado, motivo desconhecido). O backend trata NULL
--               com revoked_at preenchido de forma conservadora.
--     - valor = motivo gravado junto com revoked_at.
--   Posição: logo após `revoked_at`.
--
-- Dados: nenhuma linha é alterada. Registros legados já revogados ficam NULL
-- de propósito (não há como saber o motivo real).
--
-- Índice: NÃO criado. As buscas de refresh_tokens são por `token_hash`
-- (UNIQUE) e por `usuario_id` (idx_refresh_usuario_id); o motivo só é lido
-- depois de localizar a linha. Coluna de baixa cardinalidade, sem ganho.
--
-- Bancos novos: sql/06_ddl_refresh_tokens.sql já cria a coluna. Como o 06 usa
-- CREATE TABLE IF NOT EXISTS, db-up/db-reset não conflitam com esta migração.
-- Esta migração não faz parte do db-up/db-seed/db-reset (mesmo padrão da
-- 08/13/19/20): é correção avulsa para bancos criados antes do SEC-07.
--
-- Idempotente: só adiciona a coluna se ela ainda não existir (evita 1060
-- Duplicate column name ao rodar duas vezes).
-- Reversão: sql/21_revert_refresh_tokens_revoked_reason.sql
--           (make db-revert-revoked-reason)
--
-- Uso:
--   mysql --local-infile=1 -u $DB_USUARIO -p$DB_SENHA -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/21_alter_refresh_tokens_revoked_reason.sql
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
    @coluna_existe = 0,
    'ALTER TABLE `refresh_tokens` ADD COLUMN `revoked_reason` ENUM(\'rotacao\',\'logout\',\'revogacao_massa\',\'senha\',\'inativacao\') NULL DEFAULT NULL COMMENT \'Motivo da revogação (NULL = ativo ou revogado antes do SEC-07/legado)\' AFTER `revoked_at`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
