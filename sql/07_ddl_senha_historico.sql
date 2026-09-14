-- =====================================================
-- Tabela: senha_historico
-- Purpose: Auditoria de alterações de senha
-- Created: 2026-09-07
-- =====================================================

CREATE TABLE IF NOT EXISTS senha_historico (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    usuario_id BIGINT NOT NULL,
    resetado_por_id BIGINT NULL,
    senha_hash_anterior VARCHAR(255) NOT NULL,
    ip_origem VARCHAR(45),
    user_agent TEXT,
    tipo_reset ENUM('usuario','admin','primeiro_acesso','esquecimento') NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (usuario_id) REFERENCES usuarios(id) ON DELETE CASCADE,
    FOREIGN KEY (resetado_por_id) REFERENCES usuarios(id) ON DELETE SET NULL,
    INDEX idx_usuario (usuario_id),
    INDEX idx_data (created_at)
);
