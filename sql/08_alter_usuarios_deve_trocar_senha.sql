-- ============================================================
-- ALTER: adiciona deve_trocar_senha em usuarios (bancos já existentes)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-13
-- Agente: MegaBrain (🤍)
--
-- `sql/01_ddl_usuarios.sql` já define esta coluna para instalações novas
-- (via `make db-reset`, que recria o banco do zero). Este script é só
-- para quem já tinha o banco criado ANTES dessa coluna existir e não
-- quer/pode rodar `make db-reset` (que apaga todos os dados).
--
-- Uso:
--   mysql --local-infile=1 -u $DB_USUARIO -p$DB_SENHA -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/08_alter_usuarios_deve_trocar_senha.sql
-- ============================================================

SET NAMES utf8mb4;

-- Idempotente: só adiciona a coluna se ela ainda não existir, para o script
-- poder ser rodado mais de uma vez sem erro (ex: 1060 Duplicate column name).
SET @coluna_existe = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'usuarios'
      AND COLUMN_NAME = 'deve_trocar_senha'
);

SET @ddl = IF(
    @coluna_existe = 0,
    'ALTER TABLE `usuarios` ADD COLUMN `deve_trocar_senha` TINYINT(1) NOT NULL DEFAULT 0 COMMENT \'1 = usuário deve trocar a senha no próximo login (senha gerada pelo sistema)\' AFTER `ativo`',
    'SELECT 1'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
