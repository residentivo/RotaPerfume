-- ============================================================
-- DDL: Carteiras (CRM) — vínculo Cliente ↔ Vendedor
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-14
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/crm/carteira.csv.
-- Colunas de origem: carteira_id,cliente_id,vendedor_id,data_inicio,data_fim
--
-- Decisões de schema:
--   - `carteira_id` do CSV é sequencial e único, mas seguindo o padrão já
--     adotado em Clientes/Pedidos, a PK interna `id` (BIGINT AUTO_INCREMENT)
--     fica desacoplada da origem. O valor original é preservado em
--     `carteira_id_origem` (UNIQUE) para rastreabilidade e upsert
--     idempotente na importação.
--   - `cliente_id` do CSV NÃO pode ser usado direto como FK: a tabela
--     `clientes` tem PK interna própria. O importador resolve via lookup
--     `SELECT id FROM clientes WHERE cliente_id_origem = ?` antes do insert
--     (mesmo padrão de `pedidos`). Aqui a coluna `cliente_id` já armazena o
--     `id` interno de `clientes`.
--   - `vendedor_id` do CSV, ao contrário, JÁ corresponde 1:1 ao `id` de
--     `vendedores` (seed com IDs explícitos 1..42) — usado direto como FK,
--     sem necessidade de lookup (mesmo padrão de `pedidos`).
--   - O CSV traz HISTÓRICO de vínculos: um mesmo `cliente_id` pode aparecer
--     mais de uma vez com `vendedor_id` diferentes em períodos distintos
--     (ex: cliente 2 passa do vendedor 15 para o vendedor 32 em
--     2026-04-14). Por isso NÃO usamos UNIQUE(cliente_id) nem
--     UNIQUE(cliente_id, vendedor_id) — isso quebraria a reimportação do
--     histórico e impediria um cliente de voltar a um vendedor anterior no
--     futuro. Optamos por UNIQUE(cliente_id, vendedor_id, data_inicio),
--     suficiente para idempotência da importação (mesma tripla não se
--     repete no CSV) e para permitir múltiplos vendedores ao longo do tempo.
--   - `data_fim` é NULL enquanto o vínculo está ativo (equivalente ao campo
--     vazio do CSV). O vínculo "atual" de um cliente é a linha com
--     `data_fim IS NULL` (índice dedicado para essa busca).
--   - `data_inicio`/`data_fim` viram DATE (o CSV traz apenas data, sem
--     horário).
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: carteiras
-- Vínculo histórico Cliente ↔ Vendedor (importado de
-- dados/crm/carteira.csv)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `carteiras` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno da carteira',
    `carteira_id_origem` BIGINT NOT NULL COMMENT 'ID original da carteira no CSV de origem (dados/crm/carteira.csv)',
    `cliente_id` BIGINT NOT NULL COMMENT 'FK para clientes.id (resolvido via cliente_id_origem na importação)',
    `vendedor_id` BIGINT NOT NULL COMMENT 'FK para vendedores.id (vendedor_id do CSV já corresponde 1:1)',
    `data_inicio` DATE NOT NULL COMMENT 'Data de início do vínculo cliente-vendedor',
    `data_fim` DATE NULL COMMENT 'Data de fim do vínculo (NULL = vínculo ativo)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_carteiras_carteira_id_origem` (`carteira_id_origem`),
    UNIQUE KEY `uk_carteiras_cliente_vendedor_inicio` (`cliente_id`, `vendedor_id`, `data_inicio`),
    KEY `idx_carteiras_cliente_id` (`cliente_id`),
    KEY `idx_carteiras_vendedor_id` (`vendedor_id`),
    KEY `idx_carteiras_data_fim` (`data_fim`),
    CONSTRAINT `fk_carteiras_cliente` FOREIGN KEY (`cliente_id`)
        REFERENCES `clientes`(`id`)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    CONSTRAINT `fk_carteiras_vendedor` FOREIGN KEY (`vendedor_id`)
        REFERENCES `vendedores`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Vínculo histórico Cliente ↔ Vendedor (carteira de clientes)';

SET FOREIGN_KEY_CHECKS = 1;
