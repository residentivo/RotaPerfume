-- ============================================================
-- DDL: Itens de Pedido (ERP)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-13
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/erp/itens_pedido.csv (197.724 registros, com header).
-- Colunas de origem: item_id,pedido_id,sku,quantidade,preco_praticado,desconto_pct,valor_bruto
--
-- Decisões de schema:
--   - `item_id` do CSV é preservado em `item_id_origem` (UNIQUE) como chave
--     de idempotência para upsert, seguindo o mesmo padrão de `pedidos` e
--     `clientes`. A PK interna `id` (BIGINT AUTO_INCREMENT) fica desacoplada.
--   - `pedido_id` do CSV referencia `pedido_id` de dados/erp/pedidos.csv, e
--     NÃO o `id` interno de `pedidos` — o importador resolve via lookup
--     `SELECT id FROM pedidos WHERE pedido_id_origem = ?`. A coluna
--     `pedido_id` aqui já armazena o `id` interno de `pedidos`.
--   - `sku` do CSV referencia `produtos.sku` (chave natural/UNIQUE já
--     existente) — o importador resolve via lookup
--     `SELECT id FROM produtos WHERE sku = ?` e armazena o `id` interno de
--     `produtos` em `produto_id`, mantendo a PK interna desacoplada da
--     origem (mesmo padrão de `pedidos.cliente_id`).
--   - `ON DELETE CASCADE` em `pedido_id`: excluir um pedido remove seus
--     itens (relação de composição, item não existe sem o pedido pai).
--   - `quantidade` vira INT, `preco_praticado`/`valor_bruto` viram DECIMAL
--     para evitar erro de arredondamento monetário, `desconto_pct` vira
--     DECIMAL(5,2) (percentual, ex: 5.00 = 5%).
--   - Índice em `pedido_id` para a consulta master-detail (itens de um
--     pedido) esperada pelo Backend/Frontend.
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: itens_pedido
-- Itens (linhas) de cada pedido, importados de dados/erp/itens_pedido.csv
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `itens_pedido` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do item de pedido',
    `item_id_origem` BIGINT NOT NULL COMMENT 'ID original do item no CSV de origem (dados/erp/itens_pedido.csv)',
    `pedido_id` BIGINT NOT NULL COMMENT 'FK para pedidos.id (resolvido via pedido_id_origem na importação)',
    `produto_id` BIGINT NOT NULL COMMENT 'FK para produtos.id (resolvido via sku na importação)',
    `quantidade` INT NOT NULL COMMENT 'Quantidade de unidades do item',
    `preco_praticado` DECIMAL(10,2) NOT NULL COMMENT 'Preço unitário praticado no item',
    `desconto_pct` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT 'Percentual de desconto aplicado ao item (ex: 5.00 = 5%)',
    `valor_bruto` DECIMAL(15,2) NOT NULL COMMENT 'Valor bruto do item (quantidade x preco_praticado, já com desconto conforme origem)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_itens_pedido_item_id_origem` (`item_id_origem`),
    KEY `idx_itens_pedido_pedido_id` (`pedido_id`),
    KEY `idx_itens_pedido_produto_id` (`produto_id`),
    CONSTRAINT `fk_itens_pedido_pedido` FOREIGN KEY (`pedido_id`)
        REFERENCES `pedidos`(`id`)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    CONSTRAINT `fk_itens_pedido_produto` FOREIGN KEY (`produto_id`)
        REFERENCES `produtos`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de itens de pedido do ERP';

SET FOREIGN_KEY_CHECKS = 1;
