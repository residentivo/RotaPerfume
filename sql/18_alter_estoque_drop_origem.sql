-- ============================================================
-- ALTER: Estoque - remove coluna `origem`
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-22 (idempotência e alvo no Makefile: 2026-09-27, card DB-01)
-- Agente: DataBrain (🌸)
--
-- Contexto: `origem` (versão antiga de sql/17_ddl_estoque.sql) rastreava qual
-- processo gravou por último cada snapshot (data_snapshot, sku), para evitar
-- que o import diário do CSV do ERP e a baixa de estoque por faturamento de
-- pedidos se sobrescrevessem silenciosamente no mesmo dia. Removida a
-- pedido do usuário: o dado não é considerado relevante para a tela de
-- estoque. ATENÇÃO: a partir desta migration essa proteção de rastreio
-- deixa de existir — import do CSV e faturamento voltam a poder se
-- sobrescrever silenciosamente se rodarem no mesmo (data_snapshot, sku).
--
-- Bancos novos: desde o DB-01 (Lote 12) o sql/17_ddl_estoque.sql já cria a
-- tabela SEM `origem`, e o db-up roda o 17. Esta migração não faz parte do
-- db-up/db-seed/db-reset (mesmo padrão da 08/13/19/20/21/22): é correção
-- avulsa para bancos criados com a versão antiga do 17.
--
-- Idempotente: só remove a coluna se ela existir (evita o erro 1091
-- "Can't DROP 'origem'; check that column/key exists" ao rodar duas vezes
-- ou num banco criado pelo 17 atual).
-- Reversão: sql/18_revert_estoque_drop_origem.sql
--           (make db-revert-estoque-origem)
--
-- Uso:
--   make db-fix-estoque-origem
--   ou
--   MYSQL_PWD="$DB_SENHA" mysql --local-infile=1 -u $DB_USUARIO -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/18_alter_estoque_drop_origem.sql
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
    @coluna_existe = 1,
    'ALTER TABLE `estoque` DROP COLUMN `origem`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
