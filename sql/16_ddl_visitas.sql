-- ============================================================
-- DDL: Visitas (CRM)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-15
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/crm/visitas.csv.
-- Colunas de origem: visita_id,cliente_id,vendedor_id,data_visita,
-- resultado,duracao_min
--
-- Decisões de schema:
--   - `visita_id` é a própria PK BIGINT AUTO_INCREMENT (mesmo padrão de
--     `oportunidades.oportunidade_id` — ver sql/15_ddl_oportunidades.sql).
--     O `visita_id` do CSV é sequencial e único, então nasce direto como
--     AUTO_INCREMENT nativo (sem MAX+1 manual).
--   - `cliente_id` referencia `clientes.cliente_id_origem` (mesmo padrão de
--     `oportunidades.cliente_id`), ON DELETE CASCADE — se o cliente for
--     removido, suas visitas também são.
--   - `vendedor_id` referencia `vendedores.id` diretamente (o CSV já usa o
--     mesmo id 1..42 do seed de vendedores, mesmo padrão de
--     `oportunidades.vendedor_id`), ON DELETE RESTRICT — o vendedor
--     responsável pela visita não pode ser removido enquanto houver
--     visitas associadas.
--   - `resultado` vem do CSV como texto livre em português, com apenas 5
--     valores observados no dataset atual ("Sem pedido", "Pedido
--     realizado", "Reagendada", "Cliente ausente", "Apenas
--     relacionamento"). Mantido como VARCHAR(40) em vez de ENUM, seguindo
--     o mesmo critério já adotado para `oportunidades.etapa` (não travar a
--     importação caso surjam novos valores no futuro).
--   - `duracao_min` é a duração da visita em minutos, sempre inteiro no CSV
--     — INT.
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: visitas
-- Visitas de vendedores a clientes (CRM), vinculadas a Cliente e Vendedor
-- (importado de dados/crm/visitas.csv)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `visitas` (
    `visita_id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao visita_id do CSV de origem (dados/crm/visitas.csv)',
    `cliente_id` BIGINT NOT NULL COMMENT 'FK para clientes.cliente_id_origem',
    `vendedor_id` BIGINT NOT NULL COMMENT 'FK para vendedores.id (responsável pela visita)',
    `data_visita` DATE NOT NULL COMMENT 'Data em que a visita ocorreu',
    `resultado` VARCHAR(40) NOT NULL COMMENT 'Resultado da visita (ex.: Sem pedido, Pedido realizado, Reagendada, Cliente ausente, Apenas relacionamento)',
    `duracao_min` INT NOT NULL DEFAULT 0 COMMENT 'Duração da visita em minutos',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`visita_id`),
    KEY `idx_visitas_cliente_id` (`cliente_id`),
    KEY `idx_visitas_vendedor_id` (`vendedor_id`),
    KEY `idx_visitas_data_visita` (`data_visita`),
    KEY `idx_visitas_resultado` (`resultado`),
    CONSTRAINT `fk_visitas_cliente` FOREIGN KEY (`cliente_id`)
        REFERENCES `clientes`(`cliente_id_origem`)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    CONSTRAINT `fk_visitas_vendedor` FOREIGN KEY (`vendedor_id`)
        REFERENCES `vendedores`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Visitas de vendedores a clientes (CRM)';

SET FOREIGN_KEY_CHECKS = 1;
