-- ============================================================
-- REVERT: desfaz sql/19_alter_clientes_cnpj_unique.sql (NEG-01)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-25
-- Agente: DataBrain (🌸)
--
-- Usa as tabelas de backup criadas pelo script 19:
--   - clientes_merge_backup_20260925          (cópias removidas)
--   - clientes_merge_backup_20260925_vinculos (filhos transferidos/descartados)
-- Passos:
--   1. UNIQUE uq_clientes_cnpj -> índice simples idx_clientes_cnpj
--   2. Reinsere as cópias em `clientes` (mesmo cliente_id_origem)
--   3. Reinsere as carteiras descartadas (a partir do JSON)
--   4. Devolve os filhos transferidos ao cliente de origem
-- As tabelas de backup NÃO são removidas (apague manualmente depois de
-- conferir). Idempotente.
--
-- Uso:
--   mysql --local-infile=1 -u $DB_USUARIO -p$DB_SENHA -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/19_revert_clientes_cnpj_unique.sql
-- ============================================================

SET NAMES utf8mb4;

-- 1) Índice (o ADD vem antes do DROP para a coluna nunca ficar sem índice)
SET @idx_existe = (
    SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND INDEX_NAME = 'idx_clientes_cnpj'
);
SET @ddl = IF(@idx_existe = 0, 'ALTER TABLE `clientes` ADD KEY `idx_clientes_cnpj` (`cnpj`)', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @uq_existe = (
    SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND INDEX_NAME = 'uq_clientes_cnpj'
);
SET @ddl = IF(@uq_existe > 0, 'ALTER TABLE `clientes` DROP INDEX `uq_clientes_cnpj`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

START TRANSACTION;

-- 2) Cópias de volta
INSERT IGNORE INTO `clientes`
    (`cliente_id_origem`, `cnpj`, `razao_social`, `segmento`, `cidade`, `uf`, `bairro`,
     `data_cadastro`, `ativo`, `created_at`, `updated_at`)
SELECT `cliente_id_origem`, `cnpj`, `razao_social`, `segmento`, `cidade`, `uf`, `bairro`,
       `data_cadastro`, `ativo`, `created_at`, `updated_at`
FROM `clientes_merge_backup_20260925`;

-- 3) Carteiras descartadas de volta
INSERT IGNORE INTO `carteiras`
    (`carteira_id_origem`, `cliente_id`, `vendedor_id`, `data_inicio`, `data_fim`, `created_at`, `updated_at`)
SELECT
    l.`registro_id`,
    l.`cliente_id_antigo`,
    JSON_EXTRACT(l.`dados`, '$.vendedor_id'),
    JSON_UNQUOTE(JSON_EXTRACT(l.`dados`, '$.data_inicio')),
    IF(JSON_TYPE(JSON_EXTRACT(l.`dados`, '$.data_fim')) = 'NULL', NULL,
       JSON_UNQUOTE(JSON_EXTRACT(l.`dados`, '$.data_fim'))),
    LEFT(JSON_UNQUOTE(JSON_EXTRACT(l.`dados`, '$.created_at')), 19),
    LEFT(JSON_UNQUOTE(JSON_EXTRACT(l.`dados`, '$.updated_at')), 19)
FROM `clientes_merge_backup_20260925_vinculos` l
WHERE l.`tabela` = 'carteiras' AND l.`acao` = 'descartado';

-- 4) Filhos transferidos de volta
UPDATE `pedidos` x
JOIN `clientes_merge_backup_20260925_vinculos` l
  ON l.`tabela` = 'pedidos' AND l.`acao` = 'transferido' AND l.`registro_id` = x.`pedido_id_origem`
SET x.`cliente_id` = l.`cliente_id_antigo`;

UPDATE `oportunidades` x
JOIN `clientes_merge_backup_20260925_vinculos` l
  ON l.`tabela` = 'oportunidades' AND l.`acao` = 'transferido' AND l.`registro_id` = x.`oportunidade_id`
SET x.`cliente_id` = l.`cliente_id_antigo`;

UPDATE `visitas` x
JOIN `clientes_merge_backup_20260925_vinculos` l
  ON l.`tabela` = 'visitas' AND l.`acao` = 'transferido' AND l.`registro_id` = x.`visita_id`
SET x.`cliente_id` = l.`cliente_id_antigo`;

UPDATE `carteiras` x
JOIN `clientes_merge_backup_20260925_vinculos` l
  ON l.`tabela` = 'carteiras' AND l.`acao` = 'transferido' AND l.`registro_id` = x.`carteira_id_origem`
SET x.`cliente_id` = l.`cliente_id_antigo`;

COMMIT;
