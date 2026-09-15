-- ============================================================
-- DDL: Pagamentos (ERP)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-14
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/erp/pagamentos.csv (27.772 registros, com header).
-- Colunas de origem: pagamento_id,pedido_id,forma_pagamento,parcelas,valor,
--                     taxa_pct,valor_liquido,data_vencimento,data_pagamento,
--                     status_pagamento
--
-- Decisões de schema:
--   - DESVIO DELIBERADO DO PADRÃO USUAL (instrução explícita do usuário):
--     nas tabelas anteriores (clientes, produtos, pedidos, itens_pedido) a
--     PK interna é sempre `id` BIGINT AUTO_INCREMENT desacoplada da origem,
--     com a coluna original preservada como `xxx_id_origem` UNIQUE. Para
--     Pagamentos, a PK é literalmente `pagamento_id` BIGINT AUTO_INCREMENT,
--     pois o `pagamento_id` do CSV já é sequencial e único (1..27772) — NÃO
--     há coluna `id` separada nem `pagamento_id_origem`. O importador insere
--     o valor de `pagamento_id` explicitamente (não deixa o AUTO_INCREMENT
--     gerar), garantindo idempotência via upsert direto na PK.
--   - ATUALIZAÇÃO (2026-09-15): `pedido_id` do CSV referencia `pedido_id` de
--     dados/erp/pedidos.csv, que agora é a própria PK `pedido_id_origem` de
--     `pedidos` (ver sql/04_ddl_pedidos.sql) — o importador resolve via
--     lookup `SELECT pedido_id_origem FROM pedidos WHERE pedido_id_origem =
--     ?` (mesmo padrão de `itens_pedido.pedido_id`, ver
--     sql/11_ddl_itens_pedido.sql). A coluna `pedido_id` aqui já armazena o
--     `pedido_id_origem` de `pedidos`.
--   - `ON DELETE RESTRICT` em `pedido_id`: diferente de `itens_pedido`
--     (CASCADE, pois item é composição do pedido), um pagamento não deve
--     ficar órfão silenciosamente ao excluir o pedido — a exclusão do pedido
--     deve ser bloqueada enquanto houver pagamentos vinculados, exigindo
--     tratamento explícito.
--   - `forma_pagamento` vira ENUM com os valores reais observados no CSV
--     (análise prévia dos 27.772 registros):
--       Boleto 14 dias, Boleto 28 dias, Cartão de crédito, Cartão de débito,
--       Cheque a prazo, Dinheiro, PIX
--   - `status_pagamento` vira ENUM com os valores reais observados no CSV:
--       Em aberto, Inadimplente, Pago, Pago com atraso
--   - `parcelas` vira TINYINT UNSIGNED (valores observados 1 a 6).
--   - `valor` e `valor_liquido` viram DECIMAL(15,2) (mesmo padrão monetário
--     de `pedidos.valor_total`/`itens_pedido.valor_bruto`), `taxa_pct` vira
--     DECIMAL(5,2) (percentual, ex: 3.20 = 3,20%), mesmo padrão de
--     `itens_pedido.desconto_pct`.
--   - `data_vencimento` vira DATE NOT NULL (100% preenchida no CSV).
--   - `data_pagamento` vira DATE NULL: 1.865 linhas do CSV têm o campo vazio,
--     todas com `status_pagamento` em ('Em aberto', 'Inadimplente') — ou
--     seja, pagamento ainda não ocorreu. NULL representa fielmente "ainda
--     não pago", em vez de forçar uma data fictícia.
--   - Índices para os filtros esperados pelo Backend/Frontend (tela de
--     Pagamentos, acesso comum a qualquer usuário autenticado): `pedido_id`,
--     `status_pagamento`, `data_vencimento`.
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: pagamentos
-- Pagamentos do ERP (importados de dados/erp/pagamentos.csv)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `pagamentos` (
    `pagamento_id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao pagamento_id do CSV de origem (dados/erp/pagamentos.csv), sem desacoplamento',
    `pedido_id` BIGINT NOT NULL COMMENT 'FK para pedidos.pedido_id_origem',
    `forma_pagamento` ENUM('Boleto 14 dias','Boleto 28 dias','Cartão de crédito','Cartão de débito','Cheque a prazo','Dinheiro','PIX') NOT NULL COMMENT 'Forma de pagamento utilizada',
    `parcelas` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT 'Número de parcelas',
    `valor` DECIMAL(15,2) NOT NULL COMMENT 'Valor bruto do pagamento em R$',
    `taxa_pct` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT 'Percentual de taxa aplicado (ex: 3.20 = 3,20%)',
    `valor_liquido` DECIMAL(15,2) NOT NULL COMMENT 'Valor líquido do pagamento em R$ (valor descontada a taxa)',
    `data_vencimento` DATE NOT NULL COMMENT 'Data de vencimento do pagamento',
    `data_pagamento` DATE NULL COMMENT 'Data em que o pagamento efetivamente ocorreu (NULL quando ainda não pago — status Em aberto/Inadimplente)',
    `status_pagamento` ENUM('Em aberto','Inadimplente','Pago','Pago com atraso') NOT NULL COMMENT 'Status atual do pagamento',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`pagamento_id`),
    KEY `idx_pagamentos_pedido_id` (`pedido_id`),
    KEY `idx_pagamentos_status_pagamento` (`status_pagamento`),
    KEY `idx_pagamentos_data_vencimento` (`data_vencimento`),
    CONSTRAINT `fk_pagamentos_pedido` FOREIGN KEY (`pedido_id`)
        REFERENCES `pedidos`(`pedido_id_origem`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de pagamentos do ERP';

SET FOREIGN_KEY_CHECKS = 1;
