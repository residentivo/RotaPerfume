-- =============================================================================
-- DDL: Tabela de Usuários (item 5 do prompt)
-- Roles: VENDEDOR, GERENTE, RH
-- Auto-relacionamento: gerente_id para hierarquia de subordinação
-- =============================================================================

CREATE TABLE IF NOT EXISTS usuarios (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    login VARCHAR(100) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role ENUM('VENDEDOR', 'GERENTE', 'RH') NOT NULL DEFAULT 'VENDEDOR',
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    gerente_id BIGINT NULL,
    data_criacao DATETIME DEFAULT CURRENT_TIMESTAMP,
    data_atualizacao DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_usuarios_login (login),
    INDEX idx_usuarios_role (role),
    INDEX idx_usuarios_ativo (ativo),
    INDEX idx_usuarios_gerente_id (gerente_id),

    CONSTRAINT fk_usuarios_gerente
        FOREIGN KEY (gerente_id) REFERENCES usuarios(id)
        ON DELETE SET NULL
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
