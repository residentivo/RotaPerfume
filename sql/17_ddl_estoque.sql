-- ============================================================
-- DDL: Estoque (ERP)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-22
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/erp/estoque.csv (snapshot diário por SKU, sem
-- relação 1:1 com produtos). Colunas de origem: data_snapshot,sku,saldo,ruptura
--
-- Decisões de schema:
--   - Diferente de `produtos` (uma linha por SKU), `estoque` é uma série
--     temporal: o CSV traz múltiplas linhas por SKU ao longo do tempo (um
--     snapshot por dia). Por isso a PK é uma `id` BIGINT AUTO_INCREMENT
--     própria e desacoplada, seguindo o padrão de vendedores/usuarios/
--     clientes/produtos — a identidade de negócio do registro é o par
--     (data_snapshot, sku), garantido por UNIQUE KEY separada.
--   - `sku` referencia `produtos.sku` via FK (`fk_estoque_sku`), já que
--     `produtos` possui `sku VARCHAR(40) UNIQUE` (uk_produtos_sku, ver
--     sql/10_ddl_produtos.sql). ON UPDATE CASCADE segue o mesmo padrão de
--     FKs adotado em sql/04_ddl_pedidos.sql (fk_pedidos_cliente,
--     fk_pedidos_vendedor). ON DELETE RESTRICT evita apagar produto com
--     histórico de estoque associado.
--   - `data_snapshot` vira DATE (o CSV traz apenas data, sem horário),
--     igual ao padrão usado em `pedidos.data_pedido`.
--   - `saldo` vira INT — quantidade em unidades, não há indício de frações
--     no CSV de origem.
--   - `ruptura` vira TINYINT(1) (0/1), convertido de 'S'/'N' do CSV, mesmo
--     padrão booleano usado em `produtos.ativo`. DEFAULT 0 (sem ruptura),
--     pois é o estado mais comum e o padrão seguro para inserções feitas
--     diretamente pelo Backend quando o campo não for informado.
--   - UNIQUE KEY `uk_estoque_data_sku` (data_snapshot, sku) é a chave de
--     idempotência usada pelo importador (upsert) e também pelo upsert de
--     saldo por faturamento de pedidos (UpsertPorDataSku no repository).
--   - Índices adicionais em `sku` e `data_snapshot` isolados (além do
--     UNIQUE composto) para os filtros esperados pelo Backend/Frontend:
--     listagem por SKU (histórico de um produto) e listagem por intervalo
--     de datas (posição de estoque num dia).
--   - ATUALIZAÇÃO (2026-09-22, revisão SecBrain 🟣): adicionada coluna
--     `origem` ENUM('import_csv','faturamento','manual') NOT NULL DEFAULT
--     'import_csv', para rastrear quem gerou/alterou por último cada
--     snapshot (data_snapshot, sku). Motivo: sem essa coluna, o import
--     diário do CSV do ERP e a baixa de estoque do faturamento de pedidos
--     do dia corrente poderiam se sobrescrever silenciosamente ao colidir
--     na mesma UNIQUE KEY uk_estoque_data_sku, sem nenhum rastro de qual
--     processo gravou o valor vigente.
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: estoque
-- Snapshots diários de saldo/ruptura por SKU (importados de
-- dados/erp/estoque.csv)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `estoque` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do registro de estoque',
    `data_snapshot` DATE NOT NULL COMMENT 'Data do snapshot de estoque',
    `sku` VARCHAR(40) NOT NULL COMMENT 'SKU do produto (FK para produtos.sku)',
    `saldo` INT NOT NULL COMMENT 'Saldo em estoque na data do snapshot',
    `ruptura` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '0 = sem ruptura, 1 = em ruptura (origem: N/S)',
    `origem` ENUM('import_csv','faturamento','manual') NOT NULL DEFAULT 'import_csv' COMMENT 'Processo que gravou/atualizou por último este snapshot (import_csv = dados/erp/estoque.csv, faturamento = baixa por pedido faturado, manual = ajuste direto via Backend)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_estoque_data_sku` (`data_snapshot`, `sku`),
    KEY `idx_estoque_sku` (`sku`),
    KEY `idx_estoque_data_snapshot` (`data_snapshot`),
    KEY `idx_estoque_ruptura` (`ruptura`),
    CONSTRAINT `fk_estoque_sku` FOREIGN KEY (`sku`)
        REFERENCES `produtos`(`sku`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Snapshots diários de saldo/ruptura de estoque por SKU';

SET FOREIGN_KEY_CHECKS = 1;
