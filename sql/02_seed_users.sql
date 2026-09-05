-- =============================================================================
-- Seed: Usuários (item 6 do prompt)
-- Senha padrão: Temp@2026! (criptografada com bcrypt cost 12)
-- Para regenerar: cd apis/shared && go run ./cmd/seedusers
-- =============================================================================

-- Hash bcrypt para "Temp@2026!" com cost 12
SET @SEED_PASSWORD_HASH = '$2a$12$rvvRlVsHu5OBJ4cJmuOnoe/LvriER4mwoQtfZDiJUTvjhnCD556lu';

-- Usuário admin RH (sem gerente)
INSERT INTO usuarios (login, email, password_hash, role, ativo, gerente_id)
VALUES ('admin', 'admin@rotaperfumes.com.br', @SEED_PASSWORD_HASH, 'RH', TRUE, NULL)
ON DUPLICATE KEY UPDATE login = login;

-- Usuário gerente exemplo
INSERT INTO usuarios (login, email, password_hash, role, ativo, gerente_id)
VALUES ('gerente.curitiba', 'gerente.curitiba@rotaperfumes.com.br', @SEED_PASSWORD_HASH, 'GERENTE', TRUE, 1)
ON DUPLICATE KEY UPDATE login = login;

-- Usuário vendedor exemplo
INSERT INTO usuarios (login, email, password_hash, role, ativo, gerente_id)
VALUES ('vendedor.teste', 'vendedor.teste@rotaperfumes.com.br', @SEED_PASSWORD_HASH, 'VENDEDOR', TRUE, 2)
ON DUPLICATE KEY UPDATE login = login;
