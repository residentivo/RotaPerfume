-- ============================================================
-- DDL: Autenticação e Usuários
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-06
-- Agente: DataBrain (🌸)
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: vendedores
-- Já existia no CRM, recriada aqui para suportar FK de usuarios
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `vendedores` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do vendedor',
    `nome` VARCHAR(120) NOT NULL COMMENT 'Nome completo do vendedor',
    `regiao` VARCHAR(80) NOT NULL COMMENT 'Região de atuação',
    `uf` CHAR(2) NOT NULL COMMENT 'UF do estado',
    `data_admissao` DATE NOT NULL COMMENT 'Data de admissão',
    `data_desligamento` DATE NULL COMMENT 'Data de desligamento (NULL = ativo)',
    `meta_mensal` DECIMAL(15,2) NOT NULL DEFAULT 0.00 COMMENT 'Meta mensal em R$',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de vendedores (CRM)';

-- ------------------------------------------------------------
-- Tabela: usuarios
-- Sistema de autenticação com roles e vínculo opcional a vendedor
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `usuarios` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do usuário',
    `nome` VARCHAR(120) NOT NULL COMMENT 'Nome completo do usuário',
    `email` VARCHAR(120) NOT NULL COMMENT 'E-mail de login (único)',
    `password_hash` VARCHAR(255) NOT NULL COMMENT 'Hash bcrypt (cost 12) da senha',
    `role` ENUM('admin','normal') NOT NULL DEFAULT 'normal' COMMENT 'Papel do usuário no sistema',
    `id_vendedor` BIGINT NULL COMMENT 'FK opcional para vendedor vinculado',
    `ativo` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '0 = inativo, 1 = ativo',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    `ultimo_login_at` TIMESTAMP NULL COMMENT 'Data do último login (nullable)',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_usuarios_email` (`email`),
    KEY `idx_usuarios_role` (`role`),
    KEY `idx_usuarios_ativo` (`ativo`),
    KEY `idx_usuarios_id_vendedor` (`id_vendedor`),
    CONSTRAINT `fk_usuarios_vendedor` FOREIGN KEY (`id_vendedor`)
        REFERENCES `vendedores`(`id`)
        ON DELETE SET NULL
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de usuários para autenticação do sistema';

SET FOREIGN_KEY_CHECKS = 1;
