-- ============================================================
-- ALTER: adiciona produtos.imagem (bancos já existentes)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-10-31
-- Card: IMG-01 imagem do produto (Lote 26)
-- Agente: DataBrain (🌸)
--
-- Contexto: o produto passa a ter uma imagem opcional. O arquivo fica no
-- disco (PRODUTO_IMAGENS_DIR, adapter catalogofs); o banco guarda SÓ o nome
-- gerado pelo backend (UUID v4 minúsculo + extensão do tipo detectado).
-- Nunca guarda caminho, diretório, URL nem o filename enviado pelo cliente.
--
-- Coluna nova:
--   `imagem` VARCHAR(41) NULL DEFAULT NULL
--     - NULL  = produto sem imagem.
--     - valor = <uuid-v4>.jpg | .png | .webp (36 + 5 = 41 caracteres no máximo).
--   Posição: logo após `ativo`.
--
-- Constraints novas:
--   `chk_produtos_imagem` CHECK (imagem IS NULL OR imagem REGEXP
--     '(?-i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\.(jpg|png|webp)$')
--     O prefixo PCRE `(?-i)` é obrigatório: no MariaDB o REGEXP segue a
--     collation da coluna (utf8mb4_unicode_ci, case-insensitive), e sem ele
--     nomes com hexadecimal ou extensão MAIÚSCULOS passariam. É o mesmo
--     regex que o adapter aplica em Remover/Abrir (defesa em profundidade).
--   `uk_produtos_imagem` UNIQUE (imagem): dois produtos nunca apontam para o
--     mesmo arquivo (a troca apaga o antigo). NULLs não conflitam no UNIQUE.
--
-- Dados: nenhuma linha é alterada. Linhas existentes ficam NULL (sem imagem),
-- então o CHECK e o UNIQUE são criados sem risco de falhar.
--
-- Bancos novos: sql/10_ddl_produtos.sql já cria a coluna, o CHECK e o UNIQUE.
-- Como o 10 usa CREATE TABLE IF NOT EXISTS, db-up/db-reset não conflitam com
-- esta migração. Esta migração não faz parte do db-up/db-seed/db-reset (mesmo
-- padrão da 08/13/18 a 24): é correção avulsa para bancos criados antes do
-- IMG-01. Se for rodada num banco criado pelo 10 novo, não faz nada.
--
-- Idempotente: cada passo só roda se o objeto ainda não existir
-- (evita 1060 Duplicate column, 1061 Duplicate key name e 1826 Duplicate
-- CHECK constraint name ao rodar duas vezes).
-- Reversão: sql/26_revert_produtos_imagem.sql (make db-revert-produtos-imagem)
--
-- Uso:
--   make db-fix-produtos-imagem
--   ou
--   MYSQL_PWD="$DB_SENHA" mariadb -u $DB_USUARIO -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/26_alter_produtos_imagem.sql
-- ============================================================

SET NAMES utf8mb4;

-- ------------------------------------------------------------
-- 1) Coluna `imagem`.
-- ------------------------------------------------------------
SET @coluna_existe = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'produtos'
      AND COLUMN_NAME = 'imagem'
);

SET @ddl = IF(
    @coluna_existe = 0,
    'ALTER TABLE `produtos` ADD COLUMN `imagem` VARCHAR(41) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL COMMENT \'Nome do arquivo (UUIDv4 + .jpg/.png/.webp); NULL = sem imagem\' AFTER `ativo`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- ------------------------------------------------------------
-- 2) UNIQUE `uk_produtos_imagem`.
-- ------------------------------------------------------------
SET @indice_existe = (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'produtos'
      AND INDEX_NAME = 'uk_produtos_imagem'
);

SET @ddl = IF(
    @indice_existe = 0,
    'ALTER TABLE `produtos` ADD UNIQUE KEY `uk_produtos_imagem` (`imagem`)',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- ------------------------------------------------------------
-- 3) CHECK `chk_produtos_imagem`.
--    Dentro do literal do PREPARE o "\." do regex vira "\\\\.".
-- ------------------------------------------------------------
SET @check_existe = (
    SELECT COUNT(*)
    FROM information_schema.CHECK_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE()
      AND TABLE_NAME = 'produtos'
      AND CONSTRAINT_NAME = 'chk_produtos_imagem'
);

SET @ddl = IF(
    @check_existe = 0,
    'ALTER TABLE `produtos` ADD CONSTRAINT `chk_produtos_imagem` CHECK (`imagem` IS NULL OR `imagem` REGEXP \'(?-i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\\\\.(jpg|png|webp)$\')',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
