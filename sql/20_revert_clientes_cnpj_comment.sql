-- ============================================================
-- REVERT: desfaz sql/20_alter_clientes_cnpj_comment.sql (DB-02)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-25
-- Agente: DataBrain (🌸)
--
-- Volta o COMMENT de clientes.cnpj ao texto antigo ("somente dígitos").
-- Tipo, NOT NULL, charset/collation, índice `uq_clientes_cnpj` e dados não
-- mudam. Idempotente.
-- Obs.: o texto antigo está desatualizado desde o NEG-02 (CNPJ
-- alfanumérico). Use só se precisar desfazer a migração 20.
--
-- Uso:
--   mysql --local-infile=1 -u $DB_USUARIO -p$DB_SENHA -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/20_revert_clientes_cnpj_comment.sql
-- ============================================================

SET NAMES utf8mb4;

ALTER TABLE `clientes`
    MODIFY `cnpj` CHAR(14) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL
        COMMENT 'CNPJ normalizado (somente dígitos, sem máscara)',
    ALGORITHM=INPLACE, LOCK=NONE;
