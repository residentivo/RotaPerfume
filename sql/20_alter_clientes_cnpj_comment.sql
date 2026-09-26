-- ============================================================
-- ALTER: atualiza o COMMENT da coluna clientes.cnpj (bancos já existentes)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-25
-- Card: DB-02 (Lote 5)
-- Agente: DataBrain (🌸)
--
-- Contexto: desde o NEG-02 o `cnpj` aceita o CNPJ alfanumérico da Receita:
-- 14 caracteres, sem máscara, em MAIÚSCULAS, 12 primeiras posições em
-- [0-9A-Z] e os 2 DVs numéricos. O COMMENT antigo ainda dizia
-- "somente dígitos". Esta migração troca SÓ o texto do COMMENT.
--
-- O que NÃO muda (preservado exatamente como está):
--   - tipo CHAR(14), NOT NULL, sem DEFAULT;
--   - CHARACTER SET utf8mb4 / COLLATE utf8mb4_unicode_ci;
--   - índice UNIQUE `uq_clientes_cnpj` (não é tocado);
--   - dados (nenhuma linha é alterada).
-- ALGORITHM=INPLACE, LOCK=NONE: troca de COMMENT é só metadado. Se o MySQL
-- precisar copiar a tabela (ou seja, se algo além do COMMENT mudasse), o
-- ALTER falha em vez de reconstruir a tabela.
--
-- Bancos novos: sql/09_ddl_clientes.sql já cria a coluna com este COMMENT.
-- Esta migração não faz parte do db-up/db-seed/db-reset (mesmo padrão da 19):
-- é correção avulsa para bancos criados antes do DB-02.
--
-- A tabela de backup `clientes_merge_backup_20260925` NÃO é alterada: ela é
-- um retrato das 40 cópias removidas no NEG-01 (todas com CNPJ numérico),
-- herdou o COMMENT via CREATE ... AS SELECT e só é lida pelo revert da 19,
-- que usa lista explícita de colunas. Mudar o texto dela não tem efeito
-- prático e alteraria um artefato de auditoria.
--
-- Idempotente: rodar de novo reaplica o mesmo COMMENT.
-- Reversão: sql/20_revert_clientes_cnpj_comment.sql (make db-revert-cnpj-comment)
--
-- Uso:
--   mysql --local-infile=1 -u $DB_USUARIO -p$DB_SENHA -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/20_alter_clientes_cnpj_comment.sql
-- ============================================================

SET NAMES utf8mb4;

ALTER TABLE `clientes`
    MODIFY `cnpj` CHAR(14) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL
        COMMENT 'CNPJ normalizado: 14 caracteres, sem máscara, em maiúsculas; 12 primeiras posições em [0-9A-Z] e 2 DVs numéricos (NEG-02)',
    ALGORITHM=INPLACE, LOCK=NONE;
