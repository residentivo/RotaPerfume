-- ============================================================
-- REVERT: desfaz sql/24_alter_pagamentos_taxa_pct_check.sql (SEC-27)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-10-29
-- Agente: DataBrain (🌸)
--
-- Remove a constraint pagamentos.chk_pagamentos_taxa_pct. Nenhum dado é
-- alterado. Depois disso, o banco volta a aceitar taxa_pct de -999.99 a
-- 999.99; o teto de 30 passa a depender só da validação do domínio
-- (vendas.TaxaPctMaxima) e os importadores deixam de ser barrados no banco.
--
-- ATENÇÃO: o sql/12_ddl_pagamentos.sql cria o mesmo CHECK inline. Este
-- revert também o remove em bancos criados pelo 12 novo.
--
-- Idempotente: só remove a constraint se ela existir.
--
-- Uso:
--   make db-revert-taxa-pct-check
--   ou
--   MYSQL_PWD="$DB_SENHA" mariadb -u $DB_USUARIO -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/24_revert_pagamentos_taxa_pct_check.sql
-- ============================================================

SET NAMES utf8mb4;

SET @check_existe = (
    SELECT COUNT(*)
    FROM information_schema.CHECK_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE()
      AND TABLE_NAME = 'pagamentos'
      AND CONSTRAINT_NAME = 'chk_pagamentos_taxa_pct'
);

SET @ddl = IF(
    @check_existe = 1,
    'ALTER TABLE `pagamentos` DROP CONSTRAINT `chk_pagamentos_taxa_pct`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
