-- ============================================================
-- DDL: Pedidos
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-07
-- Agente: DataBrain (🌸)
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: pedidos
-- Armazena pedidos realizados pelos vendedores
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `pedidos` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do pedido',
    `id_vendedor` BIGINT NOT NULL COMMENT 'FK para vendedor que criou o pedido',
    `data_pedido` DATETIME NOT NULL COMMENT 'Data e hora do pedido',
    `valor_total` DECIMAL(15,2) NOT NULL DEFAULT 0.00 COMMENT 'Valor total do pedido em R$',
    `quantidade_itens` INT NOT NULL DEFAULT 1 COMMENT 'Quantidade de itens no pedido',
    `status` ENUM('pendente','confirmado','cancelado','devolvido') NOT NULL DEFAULT 'confirmado' COMMENT 'Status do pedido',
    `cliente_nome` VARCHAR(255) NULL COMMENT 'Nome do cliente (opcional)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`),
    INDEX `idx_pedidos_vendedor_data` (`id_vendedor`, `data_pedido`),
    INDEX `idx_pedidos_data` (`data_pedido`),
    INDEX `idx_pedidos_status` (`status`),
    CONSTRAINT `fk_pedidos_vendedor` FOREIGN KEY (`id_vendedor`)
        REFERENCES `vendedores`(`id`)
        ON DELETE CASCADE
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de pedidos do sistema';

SET FOREIGN_KEY_CHECKS = 1;
