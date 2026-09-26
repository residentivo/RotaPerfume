-- ============================================================
-- ALTER: adiciona usuarios.tokens_validos_desde (bancos já existentes)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-26
-- Card: SEC-08 (Lote 8), aprovado pelo SecBrain (🟣)
-- Agente: DataBrain (🌸)
--
-- Contexto: revogar refresh tokens não invalida access tokens (JWT) já
-- emitidos, que continuam válidos até expirar. Esta coluna registra um
-- "corte" por usuário: o backend rejeita qualquer access token cujo `iat`
-- seja <= `tokens_validos_desde` (troca de senha, inativação, logout global
-- etc.).
--
-- Coluna nova:
--   `tokens_validos_desde` DATETIME NULL DEFAULT NULL
--     - NULL  = sem corte (todos os access tokens válidos pela assinatura/exp).
--     - valor = instante de corte; tokens com iat <= valor são rejeitados.
--   Posição: logo após `deve_trocar_senha`.
--
-- Dados: nenhuma linha é alterada. Linhas existentes ficam NULL.
--
-- Índice: NÃO criado. A coluna só é lida depois de localizar o usuário pela
-- PK (`id` do claim `sub`), sem ganho em indexá-la.
--
-- Bancos novos: sql/01_ddl_usuarios.sql já cria a coluna. Como o 01 usa
-- CREATE TABLE IF NOT EXISTS, db-up/db-reset não conflitam com esta migração.
-- Esta migração não faz parte do db-up/db-seed/db-reset (mesmo padrão da
-- 08/13/19/20/21): é correção avulsa para bancos criados antes do SEC-08.
--
-- Idempotente: só adiciona a coluna se ela ainda não existir (evita 1060
-- Duplicate column name ao rodar duas vezes).
-- Reversão: sql/22_revert_usuarios_tokens_validos_desde.sql
--           (make db-revert-tokens-validos-desde)
--
-- Uso:
--   mysql --local-infile=1 -u $DB_USUARIO -p$DB_SENHA -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/22_alter_usuarios_tokens_validos_desde.sql
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
    @coluna_existe = 0,
    'ALTER TABLE `usuarios` ADD COLUMN `tokens_validos_desde` DATETIME NULL DEFAULT NULL COMMENT \'Access tokens com iat <= este instante são rejeitados (SEC-08); NULL = sem corte\' AFTER `deve_trocar_senha`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
