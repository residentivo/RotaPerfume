-- ============================================================
-- DDL: Clientes (CRM)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-13
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/crm/clientes.csv (3040 registros).
-- Decisões de schema:
--   - ATUALIZAÇÃO (2026-09-15): `cliente_id_origem` deixou de ser uma coluna
--     auxiliar desacoplada e passou a ser a própria PK BIGINT AUTO_INCREMENT
--     da tabela (mesmo padrão adotado em `pagamentos.pagamento_id`), pois o
--     `cliente_id` do CSV já é sequencial (1..3040) e único. A antiga PK
--     interna `id` foi removida — não há mais desacoplamento nem
--     necessidade de UNIQUE separada (a PK já garante unicidade). Coluna
--     padronizada para BIGINT (era INT).
--   - `cnpj` é normalizado (14 chars, sem máscara) na importação, pois
--     o CSV traz formatos mistos (com/sem máscara, com espaços). Até o
--     NEG-02 eram somente dígitos; ver a atualização NEG-02 abaixo.
--   - ATUALIZAÇÃO (2026-09-25, NEG-01): `cnpj` passou a ser UNIQUE
--     (`uq_clientes_cnpj`, substitui o antigo índice simples
--     `idx_clientes_cnpj`) por decisão do usuário de bloquear CNPJ
--     duplicado. O CSV de origem traz 40 CNPJs duplicados (cópias com
--     cliente_id 3001..3040 e razão social em MAIÚSCULAS). Em bancos
--     existentes, eles foram unificados no menor cliente_id por
--     sql/19_alter_clientes_cnpj_unique.sql (make db-fix-cnpj-unique).
--     O importador (apis/shared/cmd/importclientes) precisa descartar ou
--     unificar os duplicados do CSV, senão falha com erro 1062
--     (ER_DUP_ENTRY) no `uq_clientes_cnpj`.
--   - ATUALIZAÇÃO (2026-09-25, NEG-02): CNPJ alfanumérico da Receita.
--     Regra: 14 posições, as 12 primeiras em [0-9A-Z] e os 2 DVs numéricos,
--     gravado SEM máscara e em MAIÚSCULAS. Nenhuma mudança de DDL foi
--     necessária: `cnpj` é CHAR(14) utf8mb4 (texto, não numérico) e não há
--     CHECK/REGEXP/trigger/procedure/view no schema que exija só dígitos.
--     A validação do formato (e dos DVs) é responsabilidade do Backend.
--   - ATUALIZAÇÃO (2026-09-25, DB-02): o COMMENT da coluna `cnpj` deixou de
--     dizer "somente dígitos" e passou a descrever o formato alfanumérico
--     acima. Bancos novos já nascem com o texto novo por este DDL. Bancos
--     existentes recebem o mesmo texto por
--     sql/20_alter_clientes_cnpj_comment.sql (make db-fix-cnpj-comment),
--     que só troca o COMMENT (tipo, collation e `uq_clientes_cnpj` intactos).
--     COLLATION: `cnpj` herda utf8mb4_unicode_ci (case- e accent-insensitive).
--     Logo, no `uq_clientes_cnpj`, 'ab12cd34ef5601' e 'AB12CD34EF5601'
--     colidem (ER_DUP_ENTRY 1062), assim como buscas por igualdade ignoram
--     caixa. Isso é aceitável e intencional: o Backend normaliza para
--     MAIÚSCULAS antes de gravar/consultar, então o banco nunca deve conter
--     minúsculas. Não trocar para utf8mb4_bin sem rever essa premissa.
--   - `ativo` vira TINYINT(1) (0/1), convertido de 'S'/'N' do CSV.
--   - `data_cadastro` vira DATE. O CSV mistura formatos YYYY-MM-DD e
--     DD/MM/YYYY — o importador Go normaliza ambos antes do INSERT.
--   - Índices para os filtros esperados pelo Backend: `uf`, `segmento`,
--     `ativo`, e busca textual por `razao_social`/`cnpj`.
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: clientes
-- Base de clientes do CRM (importada de dados/crm/clientes.csv)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `clientes` (
    `cliente_id_origem` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao cliente_id do CSV de origem (dados/crm/clientes.csv), sem desacoplamento',
    `cnpj` CHAR(14) NOT NULL COMMENT 'CNPJ normalizado: 14 caracteres, sem máscara, em maiúsculas; 12 primeiras posições em [0-9A-Z] e 2 DVs numéricos (NEG-02)',
    `razao_social` VARCHAR(255) NOT NULL COMMENT 'Razão social do cliente',
    `segmento` VARCHAR(80) NOT NULL COMMENT 'Segmento de atuação (ex: Perfumaria, E-commerce)',
    `cidade` VARCHAR(120) NOT NULL COMMENT 'Cidade do cliente',
    `uf` CHAR(2) NOT NULL COMMENT 'UF do estado',
    `bairro` VARCHAR(120) NULL COMMENT 'Bairro do cliente',
    `data_cadastro` DATE NOT NULL COMMENT 'Data de cadastro do cliente no CRM de origem',
    `ativo` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '0 = inativo, 1 = ativo (origem: N/S)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`cliente_id_origem`),
    UNIQUE KEY `uq_clientes_cnpj` (`cnpj`),
    KEY `idx_clientes_uf` (`uf`),
    KEY `idx_clientes_segmento` (`segmento`),
    KEY `idx_clientes_ativo` (`ativo`),
    KEY `idx_clientes_razao_social` (`razao_social`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de clientes do CRM';

SET FOREIGN_KEY_CHECKS = 1;
