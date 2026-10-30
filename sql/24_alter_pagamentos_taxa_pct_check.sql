-- ============================================================
-- ALTER: CHECK de faixa em pagamentos.taxa_pct (bancos já existentes)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-10-29
-- Card: SEC-27 (Lote 25), especificado pelo SecBrain (🟣)
-- Agente: DataBrain (🌸)
--
-- Contexto: `taxa_pct` é DECIMAL(5,2) e aceitava qualquer valor até 999.99
-- (ou negativo). A regra principal (0 <= taxa_pct <= 30, constante
-- vendas.TaxaPctMaxima) fica no domínio; este CHECK é defesa em profundidade.
--
-- Constraint nova:
--   `chk_pagamentos_taxa_pct` CHECK (`taxa_pct` >= 0 AND `taxa_pct` <= 30)
--   Os limites são inclusivos: 0.00 e 30.00 são aceitos.
--
-- Dados: NENHUMA linha é alterada. Antes do ALTER, o script conta as linhas
-- com taxa_pct < 0 OR taxa_pct > 30. Se houver alguma, ele ABORTA com
-- SIGNAL SQLSTATE '45000' (mensagem com quantidade, mínimo e máximo), sem
-- corrigir nada: abra um card para o usuário decidir o que fazer com essas
-- linhas e rode a migração de novo depois. Para listar as linhas:
--   SELECT pagamento_id, pedido_id, taxa_pct, status_pagamento
--     FROM pagamentos WHERE taxa_pct < 0 OR taxa_pct > 30;
-- (Sem a checagem, o ADD CONSTRAINT falharia de qualquer forma com o erro
-- 4025 "CONSTRAINT ... failed", só que com mensagem menos clara.)
--
-- Bancos novos: sql/12_ddl_pagamentos.sql já cria o CHECK inline. Como o 12
-- usa CREATE TABLE IF NOT EXISTS, db-up/db-reset não conflitam com esta
-- migração. Esta migração não faz parte do db-up/db-seed/db-reset (mesmo
-- padrão da 08/13/18 a 23): é correção avulsa para bancos criados antes do
-- SEC-27. Se for rodada num banco criado pelo 12 novo, não faz nada.
--
-- Requer o cliente `mariadb` (usa DELIMITER e BEGIN NOT ATOMIC, MariaDB
-- >= 10.1; o projeto usa MariaDB 11.4 LTS).
--
-- Idempotente: só adiciona a constraint se ela ainda não existir em
-- information_schema.CHECK_CONSTRAINTS (evita o erro 1826 Duplicate CHECK
-- constraint name ao rodar duas vezes).
-- Reversão: sql/24_revert_pagamentos_taxa_pct_check.sql
--           (make db-revert-taxa-pct-check)
--
-- Uso:
--   make db-fix-taxa-pct-check
--   ou
--   MYSQL_PWD="$DB_SENHA" mariadb -u $DB_USUARIO -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/24_alter_pagamentos_taxa_pct_check.sql
-- ============================================================

SET NAMES utf8mb4;

-- ------------------------------------------------------------
-- 1) Checagem de dados: aborta se houver linha fora da faixa.
-- ------------------------------------------------------------
DELIMITER $$
BEGIN NOT ATOMIC
    DECLARE v_qtd BIGINT DEFAULT 0;
    DECLARE v_min DECIMAL(5,2) DEFAULT NULL;
    DECLARE v_max DECIMAL(5,2) DEFAULT NULL;
    DECLARE v_msg VARCHAR(255);

    SELECT COUNT(*), MIN(`taxa_pct`), MAX(`taxa_pct`)
      INTO v_qtd, v_min, v_max
      FROM `pagamentos`
     WHERE `taxa_pct` < 0 OR `taxa_pct` > 30;

    IF v_qtd > 0 THEN
        SET v_msg = CONCAT(
            'SEC-27: migracao 24 abortada, ', v_qtd,
            ' linha(s) de pagamentos com taxa_pct fora de 0..30 (min ', v_min,
            ', max ', v_max, '). Nenhum dado foi alterado; corrija as linhas e rode de novo.'
        );
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = v_msg;
    END IF;
END$$
DELIMITER ;

-- ------------------------------------------------------------
-- 2) Adiciona o CHECK se ainda não existir.
-- ------------------------------------------------------------
SET @check_existe = (
    SELECT COUNT(*)
    FROM information_schema.CHECK_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE()
      AND TABLE_NAME = 'pagamentos'
      AND CONSTRAINT_NAME = 'chk_pagamentos_taxa_pct'
);

SET @ddl = IF(
    @check_existe = 0,
    'ALTER TABLE `pagamentos` ADD CONSTRAINT `chk_pagamentos_taxa_pct` CHECK (`taxa_pct` >= 0 AND `taxa_pct` <= 30)',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
