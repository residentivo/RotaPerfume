-- ============================================================
-- SEED: Admin principal
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-06
-- Agente: DataBrain (🌸)
--
-- ATENÇÃO: o hash abaixo é um PLACEHOLDER e DEVE continuar placeholder
-- neste arquivo (nunca commite um hash real aqui).
-- Placeholder usado porque não temos bcrypt em SQL puro:
--   $2a$12$XXXXPLACEHOLDER_ADMIN_PRECISA_SER_GERADO_PELO_GOXXXX
--
-- Para aplicar o seed com hash real (bcrypt, cost BCRYPT_COST):
--   make gen-hash   (= cd apis/shared && go run ./cmd/seedusers)
-- A senha vem de SEED_ADMIN_PASSWORD do .env (ou é aleatória; veja
-- -show-password). O comando grava uma cópia com o hash em tmp/seed/,
-- executa essa cópia e a apaga; este arquivo NÃO é alterado.
-- ============================================================

SET NAMES utf8mb4;

-- UPSERT: insere se não existe (id=1), ou atualiza hash se já existe.
INSERT INTO `usuarios` (
    `id`,
    `nome`,
    `email`,
    `password_hash`,
    `role`,
    `id_vendedor`,
    `ativo`
) VALUES (
    1,
    'Administrador Principal',
    'admin@rotaperfumes.com.br',
    '$2a$12$XXXXPLACEHOLDER_ADMIN_PRECISA_SER_GERADO_PELO_GOXXXX',
    'admin',
    NULL,
    1
)
ON DUPLICATE KEY UPDATE
    `password_hash` = VALUES(`password_hash`),
    `nome` = VALUES(`nome`),
    `email` = VALUES(`email`),
    `ativo` = VALUES(`ativo`);

-- Confirmação
SELECT
    id,
    nome,
    email,
    role,
    ativo,
    created_at,
    CASE
        WHEN password_hash LIKE '%PLACEHOLDER%' THEN '[PLACEHOLDER - rodar make gen-hash]'
        ELSE '[HASH OK]'
    END AS hash_status
FROM `usuarios`
WHERE email = 'admin@rotaperfumes.com.br';
