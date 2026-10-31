-- ============================================================
-- REVERT: desfaz sql/26_alter_produtos_imagem.sql (IMG-01)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-10-31
-- Agente: DataBrain (🌸)
--
-- Remove o CHECK chk_produtos_imagem, o UNIQUE uk_produtos_imagem e a coluna
-- produtos.imagem (nessa ordem: o MariaDB não deixa remover uma coluna citada
-- por um CHECK de tabela).
--
-- ATENÇÃO:
--   - Os vínculos produto -> imagem são perdidos. Os arquivos em
--     PRODUTO_IMAGENS_DIR NÃO são apagados por este script (viram órfãos).
--   - Antes de reverter, volte o backend para uma versão que não lê/grava
--     `imagem` (senão List/Get/upload de produto falham).
--   - O sql/10_ddl_produtos.sql cria os mesmos objetos inline. Este revert
--     também os remove em bancos criados pelo 10 novo.
--
-- Idempotente: cada passo só roda se o objeto existir.
--
-- Uso:
--   make db-revert-produtos-imagem
--   ou
--   MYSQL_PWD="$DB_SENHA" mariadb -u $DB_USUARIO -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/26_revert_produtos_imagem.sql
-- ============================================================

SET NAMES utf8mb4;

-- 1) CHECK
SET @check_existe = (
    SELECT COUNT(*)
    FROM information_schema.CHECK_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE()
      AND TABLE_NAME = 'produtos'
      AND CONSTRAINT_NAME = 'chk_produtos_imagem'
);

SET @ddl = IF(
    @check_existe = 1,
    'ALTER TABLE `produtos` DROP CONSTRAINT `chk_produtos_imagem`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 2) UNIQUE
SET @indice_existe = (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'produtos'
      AND INDEX_NAME = 'uk_produtos_imagem'
);

SET @ddl = IF(
    @indice_existe > 0,
    'ALTER TABLE `produtos` DROP INDEX `uk_produtos_imagem`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 3) Coluna
SET @coluna_existe = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'produtos'
      AND COLUMN_NAME = 'imagem'
);

SET @ddl = IF(
    @coluna_existe = 1,
    'ALTER TABLE `produtos` DROP COLUMN `imagem`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
