-- ============================================================
-- ALTER: Estoque - remove coluna `origem`
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-22
-- Agente: DataBrain (🌸)
--
-- Contexto: `origem` (ver sql/17_ddl_estoque.sql) rastreava qual processo
-- gravou por último cada snapshot (data_snapshot, sku), para evitar que o
-- import diário do CSV do ERP e a baixa de estoque por faturamento de
-- pedidos se sobrescrevessem silenciosamente no mesmo dia. Removida a
-- pedido do usuário: o dado não é considerado relevante para a tela de
-- estoque. ATENÇÃO: a partir desta migration essa proteção de rastreio
-- deixa de existir — import do CSV e faturamento voltam a poder se
-- sobrescrever silenciosamente se rodarem no mesmo (data_snapshot, sku).
-- ============================================================

SET NAMES utf8mb4;

ALTER TABLE `estoque` DROP COLUMN `origem`;
