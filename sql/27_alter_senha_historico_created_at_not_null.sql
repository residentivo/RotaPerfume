-- ============================================================
-- ALTER: senha_historico.created_at passa a NOT NULL (bancos já existentes)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-11-05
-- Card: DB-09 (Lote 39), especificado pelo SecBrain (🟣)
-- Agente: DataBrain (🌸)
--
-- Contexto: o sql/07 antigo declarava `created_at` sem NOT NULL e, com
-- explicit_defaults_for_timestamp=ON, a coluna aceitava NULL. A API nunca envia
-- created_at (identidademysql/historico.go), então o valor vem do DEFAULT.
--
-- Dados: NENHUMA linha é alterada. Antes do ALTER, o script conta as linhas com
-- created_at NULL. Se houver alguma, ele ABORTA com SIGNAL SQLSTATE '45000'
-- (mensagem com a quantidade), sem preencher nem apagar nada.
-- Essas linhas NÃO devem ser apagadas: o histórico alimenta o bloqueio de reuso
-- de senha. Levantamento:
--   SELECT COUNT(*) FROM senha_historico WHERE created_at IS NULL;
-- Remédio manual (decisão do responsável pelo banco; a data real da troca não é
-- recuperável, então o valor é uma sentinela e fica fixo depois disso). Rode o
-- SET na mesma sessão: a API lê em -03:00 e, sem ele, numa sessão UTC, a tela
-- mostraria 1969-12-31 21:00:01:
--   SET time_zone = '-03:00';
--   UPDATE senha_historico SET created_at = '1970-01-01 00:00:01' WHERE created_at IS NULL;
-- Depois rode esta migração de novo.
--
-- O ALTER fica DENTRO do bloco, depois do SIGNAL, de propósito: testado no
-- MariaDB 11.4, o MODIFY para NOT NULL não recusa as linhas NULL, mesmo em modo
-- estrito; ele as preenche em silêncio com a hora da migração, o que falsificaria
-- a auditoria. Fora do bloco, um cliente com --force seguiria até o ALTER.
--
-- Bancos novos: sql/07_ddl_senha_historico.sql já nasce com NOT NULL. Esta
-- migração não faz parte do db-up/db-seed/db-reset nem do sql/ordem-ddl.txt
-- (mesmo padrão da 08/13/18 a 24/26): é correção avulsa para bancos criados
-- antes do DB-09. Num banco criado pelo 07 novo, não muda nada.
--
-- Risco residual: o IMPORTAR_DUMP do Jenkinsfile recria as tabelas pelo DDL do
-- dump, então um dump antigo volta a coluna para anulável. Por isso o scan com
-- sql.NullTime em identidademysql/historico.go fica como defesa; rode esta
-- migração de novo depois de importar um dump assim.
--
-- Requer o cliente `mariadb` (usa DELIMITER e BEGIN NOT ATOMIC, MariaDB
-- >= 10.1; o projeto usa MariaDB 11.4 LTS).
--
-- Idempotente: MODIFY COLUMN com a definição já aplicada não muda nada.
-- Reversão: sql/27_revert_senha_historico_created_at_not_null.sql
--           (make db-revert-historico-created-at)
--
-- Uso:
--   make db-fix-historico-created-at
--   ou
--   MYSQL_PWD="$DB_SENHA" mariadb -u $DB_USUARIO -h $DB_HOST -P $DB_PORT \
--     --default-character-set=utf8mb4 $DB_NAME < sql/27_alter_senha_historico_created_at_not_null.sql
-- ============================================================

SET NAMES utf8mb4;

-- ------------------------------------------------------------
-- Checagem de dados (aborta se houver created_at NULL) e, só depois, o ALTER.
-- ------------------------------------------------------------
DELIMITER $$
BEGIN NOT ATOMIC
    DECLARE v_qtd BIGINT DEFAULT 0;
    DECLARE v_msg VARCHAR(255);

    SELECT COUNT(*)
      INTO v_qtd
      FROM `senha_historico`
     WHERE `created_at` IS NULL;

    IF v_qtd > 0 THEN
        SET v_msg = CONCAT(
            'DB-09: migracao 27 abortada, ', v_qtd,
            ' linha(s) de senha_historico com created_at NULL. Nenhum dado foi alterado; veja o remedio no cabecalho do sql/27.'
        );
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = v_msg;
    END IF;

    ALTER TABLE senha_historico MODIFY COLUMN
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;
END$$
DELIMITER ;
