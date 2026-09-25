-- ============================================================
-- ALTER: clientes.cnpj passa a ser UNIQUE (uq_clientes_cnpj) + unificação
--        dos clientes duplicados por CNPJ (bancos já existentes)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-25
-- Card: NEG-01 (Lote 4)
-- Agente: DataBrain (🌸)
--
-- Contexto: decisão do usuário (2026-09-25) de bloquear CNPJ duplicado em
-- `clientes`. A base importada de dados/crm/clientes.csv tinha 40 CNPJs
-- duplicados (80 linhas): o original (id < 3001) e uma cópia (id 3001..3040)
-- com a razão social em MAIÚSCULAS.
--
-- Regra de unificação (decisão do usuário):
--   1. Para cada CNPJ duplicado, o SOBREVIVENTE é o MIN(cliente_id_origem)
--      do grupo. As demais linhas são CÓPIAS.
--   2. Registros vinculados às cópias em TODAS as tabelas com FK para
--      clientes (pedidos, carteiras, oportunidades, visitas — conferido no
--      INFORMATION_SCHEMA em 2026-09-25) são transferidos para o
--      sobrevivente.
--   3. carteiras tem UNIQUE (cliente_id, vendedor_id, data_inicio). Uma linha
--      de carteira da cópia só é DESCARTADA quando o vínculo equivalente já
--      existe no grupo, ou seja, há outra linha com o mesmo vendedor_id e a
--      mesma data_inicio no sobrevivente (ou, se ele não tiver, em outra
--      cópia do mesmo grupo com carteira_id_origem menor, que é a mantida).
--      Qualquer outro vínculo (outro vendedor, ou mesmo vendedor com outra
--      data_inicio, inclusive um 2º vínculo ativo com data_fim NULL) é
--      TRANSFERIDO, preservando o histórico. A linha descartada é salva
--      inteira (JSON) no log abaixo, para poder ser revertida.
--      Obs.: não há UNIQUE de "vínculo ativo" no schema. Se depois da
--      unificação o sobrevivente ficar com 2 vínculos ativos, isso é um dado
--      de negócio, e o script não resolve isso automaticamente.
--   4. As cópias são excluídas. O `ativo` do sobrevivente NÃO é alterado.
--   5. `idx_clientes_cnpj` é substituído por UNIQUE KEY `uq_clientes_cnpj`.
--
-- ATENÇÃO: as FKs de carteiras/oportunidades/visitas são ON DELETE CASCADE.
-- Por isso a transferência acontece ANTES do DELETE, e o DELETE só remove a
-- cópia que não tem mais nenhum filho (guarda contra perda silenciosa).
-- Se sobrar alguma cópia, o ADD UNIQUE falha com 1062 e nada é perdido.
--
-- Backup para reversão (criado por este script, não apagar):
--   - clientes_merge_backup_20260925          -> linhas das cópias + sobrevivente
--   - clientes_merge_backup_20260925_vinculos -> log de cada filho transferido
--     (cliente_id antigo/novo) e de cada carteira descartada (linha em JSON)
-- Reversão: sql/19_revert_clientes_cnpj_unique.sql (make db-revert-cnpj-unique)
--
-- Idempotente: pode ser rodado de novo. Se não houver duplicados, só
-- garante o índice UNIQUE. Os DDLs (CREATE TABLE/ALTER) fazem commit
-- implícito no MySQL, por isso a parte DML (log + transferência + delete)
-- fica isolada em uma única transação.
--
-- Uso:
--   mysql --local-infile=1 -u $DB_USUARIO -p$DB_SENHA -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/19_alter_clientes_cnpj_unique.sql
-- ============================================================

SET NAMES utf8mb4;

-- ------------------------------------------------------------
-- 1) Mapa de grupos: cada cliente de CNPJ duplicado -> sobrevivente.
--    Inclui o próprio sobrevivente (cliente_id = sobrevivente_id).
--    Duas cópias idênticas porque o MySQL não permite referenciar a mesma
--    TEMPORARY TABLE duas vezes na mesma query.
-- ------------------------------------------------------------
DROP TEMPORARY TABLE IF EXISTS `tmp_merge_grupo`;
CREATE TEMPORARY TABLE `tmp_merge_grupo` (
    `cliente_id` BIGINT NOT NULL,
    `sobrevivente_id` BIGINT NOT NULL,
    PRIMARY KEY (`cliente_id`),
    KEY (`sobrevivente_id`)
) ENGINE=InnoDB;

INSERT INTO `tmp_merge_grupo` (`cliente_id`, `sobrevivente_id`)
SELECT c.`cliente_id_origem`, d.`sobrevivente_id`
FROM `clientes` c
JOIN (
    SELECT `cnpj`, MIN(`cliente_id_origem`) AS `sobrevivente_id`
    FROM `clientes`
    GROUP BY `cnpj`
    HAVING COUNT(*) > 1
) d ON d.`cnpj` = c.`cnpj`;

DROP TEMPORARY TABLE IF EXISTS `tmp_merge_grupo2`;
CREATE TEMPORARY TABLE `tmp_merge_grupo2` LIKE `tmp_merge_grupo`;
INSERT INTO `tmp_merge_grupo2` SELECT * FROM `tmp_merge_grupo`;

-- Carteiras das cópias que serão descartadas (regra 3 do cabeçalho).
DROP TEMPORARY TABLE IF EXISTS `tmp_merge_carteiras_descartar`;
CREATE TEMPORARY TABLE `tmp_merge_carteiras_descartar` (
    `carteira_id_origem` BIGINT NOT NULL,
    PRIMARY KEY (`carteira_id_origem`)
) ENGINE=InnoDB;

INSERT IGNORE INTO `tmp_merge_carteiras_descartar` (`carteira_id_origem`)
SELECT cc.`carteira_id_origem`
FROM `carteiras` cc
JOIN `tmp_merge_grupo` g1
  ON g1.`cliente_id` = cc.`cliente_id`
 AND g1.`cliente_id` <> g1.`sobrevivente_id`          -- linha pertence a uma cópia
JOIN `carteiras` ck
  ON ck.`vendedor_id` = cc.`vendedor_id`
 AND ck.`data_inicio` = cc.`data_inicio`
 AND ck.`carteira_id_origem` <> cc.`carteira_id_origem`
JOIN `tmp_merge_grupo2` g2
  ON g2.`cliente_id` = ck.`cliente_id`
 AND g2.`sobrevivente_id` = g1.`sobrevivente_id`      -- mesmo grupo
WHERE g2.`cliente_id` = g2.`sobrevivente_id`          -- equivalente já está no sobrevivente
   OR ck.`carteira_id_origem` < cc.`carteira_id_origem`; -- ou em outra cópia, que é a mantida

-- ------------------------------------------------------------
-- 2) Backup das cópias (para reversão). O CREATE ... AS SELECT só roda na
--    1ª execução. O INSERT IGNORE cobre execuções seguintes, caso surjam
--    novos duplicados.
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `clientes_merge_backup_20260925` (
    PRIMARY KEY (`cliente_id_origem`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='NEG-01: backup das cópias de clientes unificadas por CNPJ (reversão via sql/19_revert_clientes_cnpj_unique.sql)'
AS
SELECT c.*, g.`sobrevivente_id` AS `merged_into_cliente_id`
FROM `clientes` c
JOIN `tmp_merge_grupo` g
  ON g.`cliente_id` = c.`cliente_id_origem`
 AND g.`cliente_id` <> g.`sobrevivente_id`;

INSERT IGNORE INTO `clientes_merge_backup_20260925`
SELECT c.*, g.`sobrevivente_id`
FROM `clientes` c
JOIN `tmp_merge_grupo` g
  ON g.`cliente_id` = c.`cliente_id_origem`
 AND g.`cliente_id` <> g.`sobrevivente_id`;

CREATE TABLE IF NOT EXISTS `clientes_merge_backup_20260925_vinculos` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `tabela` VARCHAR(64) NOT NULL COMMENT 'Tabela filha (pedidos, carteiras, oportunidades, visitas)',
    `registro_id` BIGINT NOT NULL COMMENT 'PK do registro na tabela filha',
    `cliente_id_antigo` BIGINT NOT NULL COMMENT 'Cópia à qual o registro pertencia',
    `cliente_id_novo` BIGINT NOT NULL COMMENT 'Sobrevivente',
    `acao` ENUM('transferido','descartado') NOT NULL COMMENT 'descartado = carteira equivalente já existia no grupo',
    `dados` JSON NULL COMMENT 'Linha completa (só para descartado)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uq_merge_vinculos_tabela_registro` (`tabela`, `registro_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='NEG-01: log dos filhos transferidos/descartados na unificação de clientes por CNPJ';

-- ------------------------------------------------------------
-- 3) Unificação (DML em uma única transação)
-- ------------------------------------------------------------
START TRANSACTION;

-- 3a) Log dos registros que serão transferidos/descartados
INSERT IGNORE INTO `clientes_merge_backup_20260925_vinculos`
    (`tabela`, `registro_id`, `cliente_id_antigo`, `cliente_id_novo`, `acao`)
SELECT 'pedidos', p.`pedido_id_origem`, p.`cliente_id`, g.`sobrevivente_id`, 'transferido'
FROM `pedidos` p
JOIN `tmp_merge_grupo` g ON g.`cliente_id` = p.`cliente_id` AND g.`cliente_id` <> g.`sobrevivente_id`;

INSERT IGNORE INTO `clientes_merge_backup_20260925_vinculos`
    (`tabela`, `registro_id`, `cliente_id_antigo`, `cliente_id_novo`, `acao`)
SELECT 'oportunidades', o.`oportunidade_id`, o.`cliente_id`, g.`sobrevivente_id`, 'transferido'
FROM `oportunidades` o
JOIN `tmp_merge_grupo` g ON g.`cliente_id` = o.`cliente_id` AND g.`cliente_id` <> g.`sobrevivente_id`;

INSERT IGNORE INTO `clientes_merge_backup_20260925_vinculos`
    (`tabela`, `registro_id`, `cliente_id_antigo`, `cliente_id_novo`, `acao`)
SELECT 'visitas', v.`visita_id`, v.`cliente_id`, g.`sobrevivente_id`, 'transferido'
FROM `visitas` v
JOIN `tmp_merge_grupo` g ON g.`cliente_id` = v.`cliente_id` AND g.`cliente_id` <> g.`sobrevivente_id`;

INSERT IGNORE INTO `clientes_merge_backup_20260925_vinculos`
    (`tabela`, `registro_id`, `cliente_id_antigo`, `cliente_id_novo`, `acao`, `dados`)
SELECT 'carteiras', ca.`carteira_id_origem`, ca.`cliente_id`, g.`sobrevivente_id`,
       IF(d.`carteira_id_origem` IS NULL, 'transferido', 'descartado'),
       IF(d.`carteira_id_origem` IS NULL, NULL, JSON_OBJECT(
           'carteira_id_origem', ca.`carteira_id_origem`,
           'cliente_id', ca.`cliente_id`,
           'vendedor_id', ca.`vendedor_id`,
           'data_inicio', ca.`data_inicio`,
           'data_fim', ca.`data_fim`,
           'created_at', ca.`created_at`,
           'updated_at', ca.`updated_at`))
FROM `carteiras` ca
JOIN `tmp_merge_grupo` g ON g.`cliente_id` = ca.`cliente_id` AND g.`cliente_id` <> g.`sobrevivente_id`
LEFT JOIN `tmp_merge_carteiras_descartar` d ON d.`carteira_id_origem` = ca.`carteira_id_origem`;

-- 3b) Carteiras: descarta só as equivalentes já existentes no grupo
DELETE ca
FROM `carteiras` ca
JOIN `tmp_merge_carteiras_descartar` d ON d.`carteira_id_origem` = ca.`carteira_id_origem`;

-- 3c) Transferência para o sobrevivente
UPDATE `pedidos` p
JOIN `tmp_merge_grupo` g ON g.`cliente_id` = p.`cliente_id` AND g.`cliente_id` <> g.`sobrevivente_id`
SET p.`cliente_id` = g.`sobrevivente_id`;

UPDATE `oportunidades` o
JOIN `tmp_merge_grupo` g ON g.`cliente_id` = o.`cliente_id` AND g.`cliente_id` <> g.`sobrevivente_id`
SET o.`cliente_id` = g.`sobrevivente_id`;

UPDATE `visitas` v
JOIN `tmp_merge_grupo` g ON g.`cliente_id` = v.`cliente_id` AND g.`cliente_id` <> g.`sobrevivente_id`
SET v.`cliente_id` = g.`sobrevivente_id`;

UPDATE `carteiras` ca
JOIN `tmp_merge_grupo` g ON g.`cliente_id` = ca.`cliente_id` AND g.`cliente_id` <> g.`sobrevivente_id`
SET ca.`cliente_id` = g.`sobrevivente_id`;

-- 3d) Remove as cópias. Guarda: só remove a cópia sem nenhum filho restante
--     (evita o ON DELETE CASCADE apagar dados silenciosamente).
DELETE c
FROM `clientes` c
JOIN `tmp_merge_grupo` g ON g.`cliente_id` = c.`cliente_id_origem` AND g.`cliente_id` <> g.`sobrevivente_id`
WHERE NOT EXISTS (SELECT 1 FROM `pedidos` x WHERE x.`cliente_id` = c.`cliente_id_origem`)
  AND NOT EXISTS (SELECT 1 FROM `carteiras` x WHERE x.`cliente_id` = c.`cliente_id_origem`)
  AND NOT EXISTS (SELECT 1 FROM `oportunidades` x WHERE x.`cliente_id` = c.`cliente_id_origem`)
  AND NOT EXISTS (SELECT 1 FROM `visitas` x WHERE x.`cliente_id` = c.`cliente_id_origem`);

COMMIT;

DROP TEMPORARY TABLE IF EXISTS `tmp_merge_grupo`;
DROP TEMPORARY TABLE IF EXISTS `tmp_merge_grupo2`;
DROP TEMPORARY TABLE IF EXISTS `tmp_merge_carteiras_descartar`;

-- ------------------------------------------------------------
-- 4) Índice: idx_clientes_cnpj -> UNIQUE uq_clientes_cnpj (idempotente)
--    O ADD vem antes do DROP para a coluna nunca ficar sem índice.
-- ------------------------------------------------------------
SET @uq_existe = (
    SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND INDEX_NAME = 'uq_clientes_cnpj'
);
SET @ddl = IF(@uq_existe = 0,
    'ALTER TABLE `clientes` ADD UNIQUE KEY `uq_clientes_cnpj` (`cnpj`)',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @idx_existe = (
    SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND INDEX_NAME = 'idx_clientes_cnpj'
);
SET @ddl = IF(@idx_existe > 0,
    'ALTER TABLE `clientes` DROP INDEX `idx_clientes_cnpj`',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
