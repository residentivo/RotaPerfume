-- ============================================================
-- SEED: Usuários para os 42 vendedores
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-06
-- Agente: DataBrain (🌸)
--
-- Cada vendedor (ativo ou não) recebe um usuário com:
--   role = 'normal'
--   email = v<ID>.<slug-nome>@rotaperfumes.com.br
--   senha placeholder (todos usam "Mudar@123" hashed)
--
-- ATENÇÃO: hash bcrypt placeholders — substituir via:
--   cd apis/shared && go run ./cmd/seedusers
-- ============================================================

SET NAMES utf8mb4;

-- Placeholder genérico para "Mudar@123" (bcrypt cost 12)
-- O hash real será gerado pelo Go quando BackBrain executar make gen-hash
SET @HASH_MUDAR_123 = '$2a$12$XXXXPLACEHOLDER_MUDAR123_SERA_SUBSTITUIDO_PELO_GOXXXX';

-- ------------------------------------------------------------
-- Primeiro, inserir os vendedores (se ainda não existirem)
-- ------------------------------------------------------------
INSERT IGNORE INTO `vendedores` (`id`, `nome`, `regiao`, `uf`, `data_admissao`, `data_desligamento`, `meta_mensal`) VALUES
(1, 'Henrique Rodrigues', 'Curitiba', 'PR', '2021-01-01', '2025-09-07', 40000.00),
(2, 'Carla Carvalho', 'Brasília', 'DF', '2020-10-17', NULL, 40000.00),
(3, 'Thiago Silva', 'Brasília', 'DF', '2024-06-02', '2025-07-26', 100000.00),
(4, 'Rafael Carvalho', 'Manaus', 'AM', '2023-06-22', NULL, 55000.00),
(5, 'Zeca Ferreira', 'São Paulo', 'SP', '2022-01-26', NULL, 85000.00),
(6, 'Zeca Costa', 'Curitiba', 'PR', '2022-09-06', NULL, 40000.00),
(7, 'Débora Ribeiro', 'Salvador', 'BA', '2024-01-26', NULL, 100000.00),
(8, 'Olívia Fernandes', 'Manaus', 'AM', '2023-02-08', NULL, 40000.00),
(9, 'Thiago Ribeiro', 'Campinas', 'SP', '2022-06-19', '2025-09-26', 100000.00),
(10, 'Vinícius Pereira', 'São Paulo', 'SP', '2023-07-06', NULL, 70000.00),
(11, 'Mariana Lima', 'Rio de Janeiro', 'RJ', '2024-02-21', NULL, 85000.00),
(12, 'Lucas Ribeiro', 'Belo Horizonte', 'MG', '2021-01-09', NULL, 55000.00),
(13, 'Ursula Oliveira', 'Campinas', 'SP', '2020-10-30', NULL, 100000.00),
(14, 'Henrique Ferreira', 'Manaus', 'AM', '2021-01-10', NULL, 85000.00),
(15, 'Wagner Fernandes', 'Campinas', 'SP', '2022-06-17', NULL, 55000.00),
(16, 'Henrique Santos', 'São Paulo', 'SP', '2020-09-30', NULL, 70000.00),
(17, 'Fernanda Costa', 'Curitiba', 'PR', '2022-05-04', NULL, 55000.00),
(18, 'Olívia Rodrigues', 'Campinas', 'SP', '2020-11-29', NULL, 70000.00),
(19, 'Rafael Lima', 'Brasília', 'DF', '2023-10-22', NULL, 100000.00),
(20, 'Lucas Pereira', 'Recife', 'PE', '2022-03-09', NULL, 55000.00),
(21, 'Débora Souza', 'São Paulo', 'SP', '2021-09-24', NULL, 55000.00),
(22, 'Nathan Oliveira', 'Campinas', 'SP', '2021-01-25', NULL, 85000.00),
(23, 'Queila Lima', 'Fortaleza', 'CE', '2022-06-13', NULL, 100000.00),
(24, 'Vinícius Fernandes', 'Rio de Janeiro', 'RJ', '2024-07-10', NULL, 70000.00),
(25, 'Nathan Ferreira', 'Porto Alegre', 'RS', '2020-12-29', NULL, 85000.00),
(26, 'Isabela Soares', 'Manaus', 'AM', '2024-07-27', NULL, 55000.00),
(27, 'João Soares', 'Campinas', 'SP', '2021-09-28', NULL, 100000.00),
(28, 'Rafael Soares', 'Belo Horizonte', 'MG', '2023-06-22', NULL, 40000.00),
(29, 'Diego Ribeiro', 'São Paulo', 'SP', '2021-03-25', NULL, 70000.00),
(30, 'Carla Lopes', 'Rio de Janeiro', 'RJ', '2023-03-31', '2026-07-03', 40000.00),
(31, 'Vinícius Lopes', 'Belo Horizonte', 'MG', '2021-08-08', NULL, 100000.00),
(32, 'Nathan Alves', 'Goiânia', 'GO', '2023-08-30', NULL, 100000.00),
(33, 'Mariana Ribeiro', 'Porto Alegre', 'RS', '2020-09-20', NULL, 85000.00),
(34, 'Henrique Oliveira', 'Curitiba', 'PR', '2021-09-08', NULL, 70000.00),
(35, 'Sabrina Pereira', 'Curitiba', 'PR', '2024-06-20', NULL, 40000.00),
(36, 'Henrique Oliveira', 'São Paulo', 'SP', '2024-03-10', NULL, 40000.00),
(37, 'Vinícius Lopes', 'Porto Alegre', 'RS', '2022-09-26', '2025-07-31', 55000.00),
(38, 'Paulo Pereira', 'Goiânia', 'GO', '2021-07-25', '2026-07-07', 85000.00),
(39, 'Vinícius Carvalho', 'Rio de Janeiro', 'RJ', '2022-04-22', NULL, 70000.00),
(40, 'Bruno Souza', 'Manaus', 'AM', '2022-03-19', NULL, 40000.00),
(41, 'Henrique Alves', 'Rio de Janeiro', 'RJ', '2022-05-01', NULL, 55000.00),
(42, 'Felipe Lima', 'Recife', 'PE', '2021-07-31', NULL, 85000.00);

-- ------------------------------------------------------------
-- Usuários dos vendedores (INSERT IGNORE para idempotência)
-- Formato de email: v<ID>.<slug-nome>@rotaperfumes.com.br
-- Slug: nome lowercase, espaços → underscores, acentos → normalizados
-- ------------------------------------------------------------

REPLACE INTO `usuarios` (`nome`, `email`, `password_hash`, `role`, `id_vendedor`, `ativo`) VALUES
('Henrique Rodrigues', 'henrique.rodrigues@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 1, 1),
('Carla Carvalho', 'carla.carvalho@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 2, 1),
('Thiago Silva', 'thiago.silva@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 3, 1),
('Rafael Carvalho', 'rafael.carvalho@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 4, 1),
('Zeca Ferreira', 'zeca.ferreira@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 5, 1),
('Zeca Costa', 'zeca.costa@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 6, 1),
('Débora Ribeiro', 'debora.ribeiro@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 7, 1),
('Olívia Fernandes', 'olivia.fernandes@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 8, 1),
('Thiago Ribeiro', 'thiago.ribeiro@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 9, 1),
('Vinícius Pereira', 'vinicius.pereira@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 10, 1),
('Mariana Lima', 'mariana.lima@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 11, 1),
('Lucas Ribeiro', 'lucas.ribeiro@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 12, 1),
('Ursula Oliveira', 'ursula.oliveira@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 13, 1),
('Henrique Ferreira', 'henrique.ferreira@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 14, 1),
('Wagner Fernandes', 'wagner.fernandes@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 15, 1),
('Henrique Santos', 'henrique.santos@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 16, 1),
('Fernanda Costa', 'fernanda.costa@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 17, 1),
('Olívia Rodrigues', 'olivia.rodrigues@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 18, 1),
('Rafael Lima', 'rafael.lima@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 19, 1),
('Lucas Pereira', 'lucas.pereira@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 20, 1),
('Débora Souza', 'debora.souza@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 21, 1),
('Nathan Oliveira', 'nathan.oliveira@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 22, 1),
('Queila Lima', 'queila.lima@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 23, 1),
('Vinícius Fernandes', 'vinicius.fernandes@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 24, 1),
('Nathan Ferreira', 'nathan.ferreira@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 25, 1),
('Isabela Soares', 'isabela.soares@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 26, 1),
('João Soares', 'joao.soares@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 27, 1),
('Rafael Soares', 'rafael.soares@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 28, 1),
('Diego Ribeiro', 'diego.ribeiro@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 29, 1),
('Carla Lopes', 'carla.lopes@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 30, 1),
('Vinícius Lopes', 'vinicius.lopes.mg@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 31, 1),
('Nathan Alves', 'nathan.alves@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 32, 1),
('Mariana Ribeiro', 'mariana.ribeiro@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 33, 1),
('Henrique Oliveira', 'henrique.oliveira.pr@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 34, 1),
('Sabrina Pereira', 'sabrina.pereira@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 35, 1),
('Henrique Oliveira', 'henrique.oliveira.sp@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 36, 1),
('Vinícius Lopes', 'vinicius.lopes.rs@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 37, 1),
('Paulo Pereira', 'paulo.pereira@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 38, 1),
('Vinícius Carvalho', 'vinicius.carvalho@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 39, 1),
('Bruno Souza', 'bruno.souza@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 40, 1),
('Henrique Alves', 'henrique.alves@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 41, 1),
('Felipe Lima', 'felipe.lima@rotaperfumes.com.br', @HASH_MUDAR_123, 'normal', 42, 1);

-- ------------------------------------------------------------
-- Confirmação
-- ------------------------------------------------------------
SELECT
    u.id,
    u.nome,
    u.email,
    u.role,
    u.ativo,
    v.regiao AS regiao_vendedor,
    CASE WHEN v.data_desligamento IS NULL THEN 'ativo' ELSE 'desligado' END AS status_vendedor,
    CASE WHEN u.password_hash LIKE '%PLACEHOLDER%' THEN '[PLACEHOLDER]' ELSE '[OK]' END AS hash_status
FROM `usuarios` u
LEFT JOIN `vendedores` v ON v.id = u.id_vendedor
WHERE u.role = 'normal'
ORDER BY u.id;
