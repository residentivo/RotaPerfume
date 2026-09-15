-- ============================================================
-- DDL: Clientes (CRM)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-13
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/crm/clientes.csv (3040 registros).
-- Decisões de schema:
--   - ATUALIZAÇÃO (2026-09-15): `cliente_id_origem` deixou de ser uma coluna
--     auxiliar desacoplada e passou a ser a própria PK BIGINT AUTO_INCREMENT
--     da tabela (mesmo padrão adotado em `pagamentos.pagamento_id`), pois o
--     `cliente_id` do CSV já é sequencial (1..3040) e único. A antiga PK
--     interna `id` foi removida — não há mais desacoplamento nem
--     necessidade de UNIQUE separada (a PK já garante unicidade). Coluna
--     padronizada para BIGINT (era INT).
--   - `cnpj` é normalizado (somente dígitos, 14 chars) na importação, pois
--     o CSV traz formatos mistos (com/sem máscara, com espaços). NÃO é
--     UNIQUE: a análise do CSV encontrou ~40 CNPJs duplicados associados a
--     `cliente_id` distintos (provavelmente filiais/registros duplicados na
--     origem) — index normal para busca/filtro, sem constraint de unicidade,
--     para não quebrar a importação de dados reais.
--   - `ativo` vira TINYINT(1) (0/1), convertido de 'S'/'N' do CSV.
--   - `data_cadastro` vira DATE. O CSV mistura formatos YYYY-MM-DD e
--     DD/MM/YYYY — o importador Go normaliza ambos antes do INSERT.
--   - Índices para os filtros esperados pelo Backend: `uf`, `segmento`,
--     `ativo`, e busca textual por `razao_social`/`cnpj`.
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: clientes
-- Base de clientes do CRM (importada de dados/crm/clientes.csv)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `clientes` (
    `cliente_id_origem` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao cliente_id do CSV de origem (dados/crm/clientes.csv), sem desacoplamento',
    `cnpj` CHAR(14) NOT NULL COMMENT 'CNPJ normalizado (somente dígitos, sem máscara)',
    `razao_social` VARCHAR(255) NOT NULL COMMENT 'Razão social do cliente',
    `segmento` VARCHAR(80) NOT NULL COMMENT 'Segmento de atuação (ex: Perfumaria, E-commerce)',
    `cidade` VARCHAR(120) NOT NULL COMMENT 'Cidade do cliente',
    `uf` CHAR(2) NOT NULL COMMENT 'UF do estado',
    `bairro` VARCHAR(120) NULL COMMENT 'Bairro do cliente',
    `data_cadastro` DATE NOT NULL COMMENT 'Data de cadastro do cliente no CRM de origem',
    `ativo` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '0 = inativo, 1 = ativo (origem: N/S)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`cliente_id_origem`),
    KEY `idx_clientes_cnpj` (`cnpj`),
    KEY `idx_clientes_uf` (`uf`),
    KEY `idx_clientes_segmento` (`segmento`),
    KEY `idx_clientes_ativo` (`ativo`),
    KEY `idx_clientes_razao_social` (`razao_social`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de clientes do CRM';

SET FOREIGN_KEY_CHECKS = 1;
