-- ============================================================
-- DDL: Oportunidades (CRM)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-15
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/crm/oportunidades.csv.
-- Colunas de origem: oportunidade_id,cliente_id,vendedor_id,origem,
-- data_abertura,etapa,probabilidade_pct,valor_estimado,data_fechamento,
-- ciclo_dias,motivo_perda
--
-- Decisões de schema:
--   - `oportunidade_id` é a própria PK BIGINT AUTO_INCREMENT (pedido
--     explícito do usuário) — diferente do padrão `*_id_origem` usado em
--     `clientes`/`carteiras`, mas equivalente ao padrão já adotado em
--     `pagamentos.pagamento_id`. O `oportunidade_id` do CSV é sequencial e
--     único, então nasce direto como AUTO_INCREMENT nativo (sem MAX+1
--     manual).
--   - `cliente_id` referencia `clientes.cliente_id_origem` (mesmo padrão de
--     `carteiras.cliente_id`), ON DELETE CASCADE — se o cliente for
--     removido, suas oportunidades também são.
--   - `vendedor_id` referencia `vendedores.id` diretamente (o CSV já usa o
--     mesmo id 1..42 do seed de vendedores, mesmo padrão de
--     `carteiras.vendedor_id`), ON DELETE RESTRICT — o vendedor é o "dono"
--     direto da oportunidade, independente do vínculo de carteira, e não
--     pode ser removido enquanto houver oportunidades associadas.
--   - `origem` e `etapa` vieram do CSV como texto livre em português, com
--     valores como "WhatsApp", "Indicação", "Inbound site", "Instagram",
--     "Feira de beleza", "Reativação", "Prospecção ativa" (origem) e
--     "Prospecção", "Qualificação", "Proposta enviada", "Negociação",
--     "Fechado ganho", "Fechado perdido" (etapa). Mantidos como VARCHAR
--     (80 e 40, respectivamente) em vez de ENUM para não travar a
--     importação caso surjam novos valores no futuro.
--   - `probabilidade_pct` no CSV vem sempre como inteiro (0,10,25,50,75,100)
--     mas mantido DECIMAL(5,2) conforme spec para suportar frações futuras.
--   - `valor_estimado` sempre com 2 casas decimais no CSV — DECIMAL(15,2)
--     conforme padrão de valores monetários do projeto.
--   - `data_fechamento`, `ciclo_dias` e `motivo_perda` ficam NULL enquanto a
--     oportunidade está aberta (equivalente aos campos vazios do CSV).
--   - `ciclo_dias` é armazenado tal como vem no CSV (não recalculado em
--     trigger), já que é um dado de importação histórica.
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: oportunidades
-- Oportunidades de venda (CRM), vinculadas a Cliente e Vendedor
-- (importado de dados/crm/oportunidades.csv)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `oportunidades` (
    `oportunidade_id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao oportunidade_id do CSV de origem (dados/crm/oportunidades.csv)',
    `cliente_id` BIGINT NOT NULL COMMENT 'FK para clientes.cliente_id_origem',
    `vendedor_id` BIGINT NOT NULL COMMENT 'FK para vendedores.id (dono direto da oportunidade)',
    `origem` VARCHAR(80) NOT NULL COMMENT 'Canal de origem da oportunidade (ex.: WhatsApp, Indicação, Inbound site, Instagram)',
    `data_abertura` DATE NOT NULL COMMENT 'Data de abertura da oportunidade',
    `etapa` VARCHAR(40) NOT NULL COMMENT 'Etapa atual do funil (ex.: Prospecção, Qualificação, Proposta enviada, Negociação, Fechado ganho, Fechado perdido)',
    `probabilidade_pct` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT 'Probabilidade de fechamento (0-100)',
    `valor_estimado` DECIMAL(15,2) NOT NULL DEFAULT 0.00 COMMENT 'Valor estimado da oportunidade',
    `data_fechamento` DATE NULL COMMENT 'Data de fechamento (NULL enquanto a oportunidade está aberta)',
    `ciclo_dias` INT NULL COMMENT 'Dias entre abertura e fechamento (NULL enquanto aberta)',
    `motivo_perda` VARCHAR(255) NULL COMMENT 'Motivo da perda, quando etapa = Fechado perdido',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`oportunidade_id`),
    KEY `idx_oportunidades_cliente_id` (`cliente_id`),
    KEY `idx_oportunidades_vendedor_id` (`vendedor_id`),
    KEY `idx_oportunidades_etapa` (`etapa`),
    KEY `idx_oportunidades_origem` (`origem`),
    KEY `idx_oportunidades_data_abertura` (`data_abertura`),
    CONSTRAINT `fk_oportunidades_cliente` FOREIGN KEY (`cliente_id`)
        REFERENCES `clientes`(`cliente_id_origem`)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    CONSTRAINT `fk_oportunidades_vendedor` FOREIGN KEY (`vendedor_id`)
        REFERENCES `vendedores`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Oportunidades de venda (CRM)';

SET FOREIGN_KEY_CHECKS = 1;
