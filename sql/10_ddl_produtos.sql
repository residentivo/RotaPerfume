-- ============================================================
-- DDL: Produtos (Catálogo/ERP)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-13
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/erp/produtos.csv (292 registros, sem header).
-- Decisões de schema:
--   - `sku` do CSV (ex: SKU00001) já é único na origem e é usado como
--     chave de idempotência para upsert na importação (equivalente ao
--     `cliente_id_origem` usado no padrão de Clientes). Mantemos, porém,
--     a PK interna `id` (BIGINT AUTO_INCREMENT) desacoplada da origem,
--     seguindo o padrão do restante do banco (vendedores, usuarios,
--     clientes, pedidos). `sku` é UNIQUE para garantir o upsert idempotente.
--   - `ativo` vira TINYINT(1) (0/1), convertido de 'S'/'N' do CSV.
--     DEFAULT 1 (ativo) pois é o padrão exigido para itens importados e
--     para inserções feitas diretamente pelo Backend (exclusão lógica).
--   - `preco_tabela` e `custo_unitario` viram DECIMAL(10,2) para evitar
--     erros de arredondamento de ponto flutuante em valores monetários.
--   - `data_lancamento` vira DATE NULL, pois o CSV traz o campo vazio em
--     parte dos registros (produto sem data de lançamento cadastrada).
--   - `unidade` guarda a unidade/embalagem de venda como veio da origem
--     (ex: "UN", "KIT 5", "DISPLAY 24", "CX 12") sem normalização adicional.
--   - Índices para os filtros esperados pelo Backend/Frontend: `categoria`,
--     `marca`, `ativo`, além do UNIQUE em `sku` para busca e upsert.
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: produtos
-- Catálogo de produtos do ERP (importado de dados/erp/produtos.csv)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `produtos` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do produto',
    `sku` VARCHAR(40) NOT NULL COMMENT 'SKU original do produto no CSV de origem (dados/erp/produtos.csv)',
    `descricao` VARCHAR(255) NOT NULL COMMENT 'Descrição/nome do produto',
    `categoria` VARCHAR(80) NOT NULL COMMENT 'Categoria do produto (ex: Eau de Parfum)',
    `marca` VARCHAR(80) NOT NULL COMMENT 'Marca do produto',
    `nota_olfativa` VARCHAR(80) NULL COMMENT 'Nota olfativa predominante (ex: Cardamomo, Jasmim)',
    `preco_tabela` DECIMAL(10,2) NOT NULL COMMENT 'Preço de tabela (venda) do produto',
    `custo_unitario` DECIMAL(10,2) NOT NULL COMMENT 'Custo unitário do produto',
    `unidade` VARCHAR(40) NOT NULL COMMENT 'Unidade/embalagem de venda (ex: UN, KIT 5, DISPLAY 24)',
    `data_lancamento` DATE NULL COMMENT 'Data de lançamento do produto (pode ser nula na origem)',
    `ativo` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '0 = inativo, 1 = ativo (origem: N/S; padrão de importação: ativo)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_produtos_sku` (`sku`),
    KEY `idx_produtos_categoria` (`categoria`),
    KEY `idx_produtos_marca` (`marca`),
    KEY `idx_produtos_ativo` (`ativo`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de produtos do catálogo/ERP';

SET FOREIGN_KEY_CHECKS = 1;
