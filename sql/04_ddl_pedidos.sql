-- ============================================================
-- DDL: Pedidos (ERP)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-13
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/erp/pedidos.csv (28.729 registros, com header).
-- Colunas de origem: pedido_id,cliente_id,vendedor_id,data_pedido,canal,status,valor_total
--
-- Decisões de schema:
--   - `pedido_id` do CSV é sequencial e único, mas seguindo o padrão já
--     adotado em Clientes/Produtos, a PK interna `id` (BIGINT AUTO_INCREMENT)
--     fica desacoplada da origem. O valor original é preservado em
--     `pedido_id_origem` (UNIQUE) para rastreabilidade e upsert idempotente.
--   - `cliente_id` do CSV NÃO pode ser usado direto como FK: a tabela
--     `clientes` tem PK interna própria. O importador resolve via lookup
--     `SELECT id FROM clientes WHERE cliente_id_origem = ?` antes do insert.
--     Aqui a coluna `cliente_id` já armazena o `id` interno de `clientes`.
--   - `vendedor_id` do CSV, ao contrário, JÁ corresponde 1:1 ao `id` de
--     `vendedores` (seed com IDs explícitos 1..42) — usado direto como FK,
--     sem necessidade de lookup.
--   - `data_pedido` vira DATE (o CSV traz apenas data, sem horário).
--   - `canal` e `status` viram ENUM com os valores reais observados no CSV
--     (análise prévia dos 28.729 registros), evitando strings livres:
--       canal:  App, Telefone, Visita, WhatsApp
--       status: Cancelado, Em separação, Entregue, Faturado
--   - `valor_total` vira DECIMAL(15,2) para evitar erros de arredondamento
--     de ponto flutuante em valores monetários.
--   - Índices para os filtros esperados pelo Backend/Frontend (tela de
--     Pedidos master-detail): `status`, `canal`, `data_pedido`, `cliente_id`,
--     `vendedor_id`.
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: pedidos
-- Pedidos do ERP (importados de dados/erp/pedidos.csv)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `pedidos` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do pedido',
    `pedido_id_origem` BIGINT NOT NULL COMMENT 'ID original do pedido no CSV de origem (dados/erp/pedidos.csv)',
    `cliente_id` BIGINT NOT NULL COMMENT 'FK para clientes.id (resolvido via cliente_id_origem na importação)',
    `vendedor_id` BIGINT NOT NULL COMMENT 'FK para vendedores.id (vendedor_id do CSV já corresponde 1:1)',
    `data_pedido` DATE NOT NULL COMMENT 'Data do pedido',
    `canal` ENUM('App','Telefone','Visita','WhatsApp') NOT NULL COMMENT 'Canal de venda do pedido',
    `status` ENUM('Cancelado','Em separação','Entregue','Faturado') NOT NULL COMMENT 'Status do pedido',
    `valor_total` DECIMAL(15,2) NOT NULL DEFAULT 0.00 COMMENT 'Valor total do pedido em R$',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_pedidos_pedido_id_origem` (`pedido_id_origem`),
    KEY `idx_pedidos_status` (`status`),
    KEY `idx_pedidos_canal` (`canal`),
    KEY `idx_pedidos_data_pedido` (`data_pedido`),
    KEY `idx_pedidos_cliente_id` (`cliente_id`),
    KEY `idx_pedidos_vendedor_id` (`vendedor_id`),
    CONSTRAINT `fk_pedidos_cliente` FOREIGN KEY (`cliente_id`)
        REFERENCES `clientes`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    CONSTRAINT `fk_pedidos_vendedor` FOREIGN KEY (`vendedor_id`)
        REFERENCES `vendedores`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de pedidos do ERP';

SET FOREIGN_KEY_CHECKS = 1;
