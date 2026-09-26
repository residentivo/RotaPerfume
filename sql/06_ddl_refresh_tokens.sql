-- ============================================================
-- DDL: Refresh Tokens (JWT)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-07
-- Agente: DataBrain (🌸)
-- Tarefa: Refresh de token JWT
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: refresh_tokens
-- Armazena refresh tokens ativos e revogados para suporte
-- ao sistema de renovação de JWT (access token).
-- Cada refresh token é armazenado como hash SHA-256.
-- Revogação: revoked_at = NULL (ativo) | DATETIME (revogado)
-- Motivo: revoked_reason (SEC-07, 2026-09-26). Bancos criados antes
-- recebem a coluna pela migração 21 (make db-fix-revoked-reason).
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `refresh_tokens` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do refresh token',
    `usuario_id` BIGINT NOT NULL COMMENT 'FK para usuario dono do token',
    `token_hash` VARCHAR(255) NOT NULL COMMENT 'SHA-256 do refresh token (único)',
    `expires_at` DATETIME NOT NULL COMMENT 'Data/hora de expiração do token',
    `revoked_at` DATETIME NULL COMMENT 'Data/hora de revogação (NULL = ativo)',
    `revoked_reason` ENUM('rotacao','logout','revogacao_massa','senha','inativacao') NULL DEFAULT NULL COMMENT 'Motivo da revogação (NULL = ativo ou revogado antes do SEC-07/legado)',
    `ip_origem` VARCHAR(45) NULL COMMENT 'IP de origem que solicitou o token (IPv4/IPv6)',
    `user_agent` TEXT NULL COMMENT 'User-Agent do navegador/cliente no momento da criação',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_refresh_token_hash` (`token_hash`),
    KEY `idx_refresh_usuario_id` (`usuario_id`),
    KEY `idx_refresh_expires_at` (`expires_at`),
    KEY `idx_refresh_revoked_at` (`revoked_at`),
    CONSTRAINT `fk_refresh_token_usuario`
        FOREIGN KEY (`usuario_id`)
        REFERENCES `usuarios`(`id`)
        ON DELETE CASCADE
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
COMMENT='Tabela de refresh tokens para renovação automática de JWT';

SET FOREIGN_KEY_CHECKS = 1;
