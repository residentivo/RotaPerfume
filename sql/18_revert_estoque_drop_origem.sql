-- ============================================================
-- REVERT: desfaz sql/18_alter_estoque_drop_origem.sql
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-27 (card DB-01, Lote 12)
-- Agente: DataBrain (🌸)
--
-- Recria a coluna estoque.origem com a mesma definição da versão antiga de
-- sql/17_ddl_estoque.sql, na mesma posição (logo após `ruptura`).
--
-- ATENÇÃO: o valor original de `origem` foi perdido quando a 18 rodou. Todas
-- as linhas existentes voltam com o DEFAULT 'import_csv', que NÃO reflete o
-- processo que de fato gravou cada snapshot. O backend atual não lê nem grava
-- `origem`; como a coluna tem DEFAULT, os INSERTs/upserts atuais continuam
-- funcionando depois do revert.
--
-- Idempotente: só adiciona a coluna se ela ainda não existir (evita o erro
-- 1060 Duplicate column name ao rodar duas vezes).
--
-- Uso:
--   make db-revert-estoque-origem
--   ou
--   MYSQL_PWD="$DB_SENHA" mysql --local-infile=1 -u $DB_USUARIO -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/18_revert_estoque_drop_origem.sql
-- ============================================================

SET NAMES utf8mb4;

SET @coluna_existe = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'estoque'
      AND COLUMN_NAME = 'origem'
);

SET @ddl = IF(
    @coluna_existe = 0,
    'ALTER TABLE `estoque` ADD COLUMN `origem` ENUM(\'import_csv\',\'faturamento\',\'manual\') NOT NULL DEFAULT \'import_csv\' COMMENT \'Processo que gravou/atualizou por último este snapshot (import_csv = dados/erp/estoque.csv, faturamento = baixa por pedido faturado, manual = ajuste direto via Backend)\' AFTER `ruptura`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
