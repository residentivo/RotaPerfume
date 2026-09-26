# Manual da base de dados

**Banco:** MySQL, schema `rotaperfumes` (padrão do `Makefile`: `DB_NAME?=rotaperfumes`), charset `utf8mb4`, collation `utf8mb4_unicode_ci`.
**Fonte da verdade:** os scripts em `sql/`. Em caso de divergência entre este manual e um script, vale o script.
**Autor:** SubBrain (2026-09-25, card DOC-02; atualizado no fechamento do Lote 5 com o NEG-02, CNPJ alfanumérico, e no Lote 6, 2026-09-26, com a migração 21 e a coluna `refresh_tokens.revoked_reason`).

> **Escopo desta versão:** o card DOC-02 cobre a tabela `clientes`, o índice único `uq_clientes_cnpj` e a migração 19 (unificação dos CNPJs duplicados). O Lote 6 acrescentou a tabela `refresh_tokens` e a migração 21 (seção 7). As outras tabelas aparecem só no índice da seção 1, com o script de origem e os relacionamentos com `clientes`. O detalhamento campo a campo delas ainda não foi feito.

---

## 1. Tabelas e scripts

A ordem abaixo é a do `make db-up`, que cria o schema vazio. O `make db-seed` roda o `db-up` e depois os seeds; o `make db-reset` apaga o banco e roda o `db-seed`.

| Tabela | Script | Relação com `clientes` |
| --- | --- | --- |
| `vendedores`, `usuarios` | `sql/01_ddl_usuarios.sql` (+ `08_alter_usuarios_deve_trocar_senha.sql`) | - |
| `refresh_tokens` | `sql/06_ddl_refresh_tokens.sql` (+ `21_alter_refresh_tokens_revoked_reason.sql` em bancos existentes; seção 7) | - (ligada a `usuarios`) |
| `senha_historico` | `sql/07_ddl_senha_historico.sql` (+ `13_alter_senha_historico_tipo_reset.sql`) | - |
| `clientes` | `sql/09_ddl_clientes.sql` (+ `19_alter_clientes_cnpj_unique.sql` e `20_alter_clientes_cnpj_comment.sql` em bancos existentes) | tabela principal |
| `produtos` | `sql/10_ddl_produtos.sql` | - |
| `pedidos` | `sql/04_ddl_pedidos.sql` | `fk_pedidos_cliente` (`cliente_id`), `ON DELETE RESTRICT` |
| `itens_pedido` | `sql/11_ddl_itens_pedido.sql` | - (ligada a `pedidos`) |
| `pagamentos` | `sql/12_ddl_pagamentos.sql` | - (ligada a `pedidos`) |
| `carteiras` | `sql/14_ddl_carteiras.sql` | `fk_carteiras_cliente` (`cliente_id`), `ON DELETE CASCADE` |
| `oportunidades` | `sql/15_ddl_oportunidades.sql` | `fk_oportunidades_cliente` (`cliente_id`), `ON DELETE CASCADE` |
| `visitas` | `sql/16_ddl_visitas.sql` | `fk_visitas_cliente` (`cliente_id`), `ON DELETE CASCADE` |
| `estoque` | `sql/17_ddl_estoque.sql` (+ `18_alter_estoque_drop_origem.sql`) | - |
| `clientes_merge_backup_20260925` | criada pela migração 19 | backup das cópias removidas (seção 4) |
| `clientes_merge_backup_20260925_vinculos` | criada pela migração 19 | log dos filhos das cópias (seção 4) |

Todas as FKs para `clientes` apontam para `clientes.cliente_id_origem`.

---

## 2. Tabela `clientes`

Base de clientes do CRM, importada de `dados/crm/clientes.csv` (`make db-import-clientes`).

| Campo | Tipo | Nulo | Descrição |
| --- | --- | --- | --- |
| `cliente_id_origem` | BIGINT AUTO_INCREMENT | não | PK. Corresponde 1:1 ao `cliente_id` do CSV. |
| `cnpj` | CHAR(14) | não | CNPJ sem máscara, em maiúsculas, numérico ou alfanumérico (NEG-02; formato na seção "CNPJ alfanumérico" abaixo). **Único** (`uq_clientes_cnpj`). |
| `razao_social` | VARCHAR(255) | não | Razão social. |
| `segmento` | VARCHAR(80) | não | Segmento de atuação (ex.: Perfumaria, E-commerce). |
| `cidade` | VARCHAR(120) | não | Cidade. |
| `uf` | CHAR(2) | não | UF. |
| `bairro` | VARCHAR(120) | sim | Bairro. |
| `data_cadastro` | DATE | não | Data de cadastro no CRM de origem. |
| `ativo` | TINYINT(1) | não | 1 = ativo (padrão), 0 = inativo. No CSV vem como S/N. |
| `created_at` | TIMESTAMP | não | Criação do registro. |
| `updated_at` | TIMESTAMP | não | Última atualização (`ON UPDATE CURRENT_TIMESTAMP`). |

### Índices

| Índice | Tipo | Colunas |
| --- | --- | --- |
| `PRIMARY` | PK | `cliente_id_origem` |
| `uq_clientes_cnpj` | **UNIQUE** | `cnpj` |
| `idx_clientes_uf` | simples | `uf` |
| `idx_clientes_segmento` | simples | `segmento` |
| `idx_clientes_ativo` | simples | `ativo` |
| `idx_clientes_razao_social` | simples | `razao_social` |

### Índice único `uq_clientes_cnpj` (NEG-01, 2026-09-25)

- Substitui o antigo índice simples `idx_clientes_cnpj`, que não impedia CNPJ repetido. Depois da migração, o `idx_clientes_cnpj` não existe mais.
- Decisão do usuário (NEG-01): bloquear CNPJ duplicado no banco.
- Em bancos novos, o `sql/09_ddl_clientes.sql` já cria a tabela com o `uq_clientes_cnpj`. Em bancos que já existiam, quem cria o índice é a migração 19 (seção 3).
- Um INSERT/UPDATE com CNPJ repetido falha com o erro MySQL **1062** (`ER_DUP_ENTRY`). A API converte esse erro em `409` genérico "cnpj já cadastrado", sem revelar o vendedor dono do cliente. Um CNPJ com formato ou DV inválido é recusado antes, com `400` "cnpj inválido" (NEG-03 e NEG-04), então a checagem de duplicidade só se aplica a CNPJ válido.
- **Importação:** como o CSV tem CNPJs repetidos, os importadores unificam os duplicados antes de gravar (`apis/shared/importers/clientesdedup`; até o Lote 6 ficava em `cmd/internal/clientesdedup`). Fica a primeira ocorrência de cada CNPJ, que é o menor `cliente_id`, igual à regra da migração. O `importclientes` não grava as cópias, e os importadores de carteiras, pedidos, oportunidades e visitas redirecionam o `cliente_id` da cópia para o sobrevivente. Sem isso, a importação falharia com 1062.
- **Collation:** `cnpj` herda `utf8mb4_unicode_ci`, que ignora maiúsculas e minúsculas. No índice único, valores que só diferem na caixa colidem. O Backend grava em maiúsculas. Detalhes no cabeçalho de `sql/09_ddl_clientes.sql` (nota do NEG-02, CNPJ alfanumérico).

### CNPJ alfanumérico (NEG-02, 2026-09-25)

A coluna `cnpj` aceita o CNPJ alfanumérico da Receita Federal (vigente desde julho de 2026). **Não houve migração:** `CHAR(14)` com `utf8mb4_unicode_ci` já comporta letras, e não há CHECK, trigger nem view sobre a coluna.

- **Formato gravado:** 14 caracteres, sem máscara e em **maiúsculas**. As 12 primeiras posições são `[0-9A-Z]` e as 2 últimas (dígitos verificadores) são `[0-9]`. Ex.: `12ABC34501DE35` (exibido como `12.ABC.345/01DE-35`). Os CNPJs numéricos continuam válidos, e os 3000 clientes atuais são todos numéricos.
- **Dígito verificador:** módulo 11 com os pesos do CNPJ numérico. Cada caractere vale o código ASCII − 48 (`0`–`9` → 0–9, `A` → 17, …, `Z` → 42). O DV é validado pela API e pelo frontend, **não pelo banco**.
- **Regra central:** `apis/shared/cnpj` (`Normalizar`, `FormatoValido`, `Valido`, `DigitosVerificadores`), usada pela API (`services/cnpj.go`) e pelos importadores. O frontend espelha a regra em `frontend/src/lib/cnpj.ts`.
- **Caixa:** a API normaliza para maiúsculas antes de gravar. Como a collation é `_ci`, um `SELECT ... WHERE cnpj = '12abc34501de35'` também encontra o registro, e o índice único trata `12abc...` e `12ABC...` como o mesmo valor. Para conferir a caixa gravada de verdade, use `BINARY cnpj`. Ex.: `SELECT COUNT(*) FROM clientes WHERE BINARY cnpj <> UPPER(cnpj);` deve voltar 0.
- **Comentário da coluna (DB-02, 2026-09-25):** o `COMMENT` de `cnpj` diz "CNPJ normalizado: 14 caracteres, sem máscara, em maiúsculas; 12 primeiras posições em [0-9A-Z] e 2 DVs numéricos (NEG-02)". Bancos novos já recebem esse texto pelo `sql/09_ddl_clientes.sql`. Bancos existentes recebem pela migração 20 (`sql/20_alter_clientes_cnpj_comment.sql`, `make db-fix-cnpj-comment`; reversão: `make db-revert-cnpj-comment`). Ela troca só o texto: tipo, collation, índice `uq_clientes_cnpj` e dados não mudam. Como a 19, a migração 20 não faz parte do `db-up`/`db-seed`. Foi aplicada no banco local em 2026-09-25. A tabela `clientes_merge_backup_20260925` mantém o comentário antigo de propósito, porque guarda o retrato das cópias removidas no NEG-01.

### Importador `importclientes` e o CNPJ (NEG-02, 2026-09-25)

O importador de clientes (`apis/shared/importers/clientes`, chamado pelo `cmd/importclientes`) e o `importers/clientesdedup` usam a mesma normalização da API (`clientesdedup.NormalizarCNPJ` → `shared/cnpj`):

- remove a máscara (`.`, `/`, `-` e espaços) e converte letras para maiúsculas;
- exige 14 caracteres, com as 12 primeiras posições em `[0-9A-Z]` e as 2 últimas numéricas.

**Mudança de comportamento:** uma linha do CSV com caractere fora da máscara e de `[0-9A-Za-z]` (ex.: `#`, `_`, acento), ou com letra nas posições do DV, agora é **recusada**. Ela é contada em `erros_parsing` no log, com a mensagem `cnpj inválido (..., esperado 14 caracteres: 12 em [0-9A-Z] + 2 DVs numéricos, máscara opcional)`. Antes, o importador **removia** o caractere inválido e seguia com o que sobrasse.

- **O DV não é validado na importação:** um CNPJ com o formato certo e DV errado é importado (os dados do CSV são fictícios; o DV só é exigido pela API). Na primeira edição pela API, esse cliente recebe `400` "cnpj inválido" até o CNPJ ser corrigido (regra do NEG-04).
- **Unificação:** como a normalização vem antes da deduplicação, variantes do mesmo CNPJ com outra máscara ou outra caixa são unificadas na primeira ocorrência.
- **Impacto no CSV atual:** nenhum. O TestBrain conferiu em 2026-09-25 que as 3040 linhas de `dados/crm/clientes.csv` passam na regra nova e na antiga, sem diferença.

---

## 3. Migração 19: unificação dos CNPJs duplicados

**Script:** `sql/19_alter_clientes_cnpj_unique.sql`
**Comando:** `make db-fix-cnpj-unique` (roda `mysql $(MYSQL_OPTS) $(DB_NAME) < sql/19_alter_clientes_cnpj_unique.sql`)
**Reversão:** `make db-revert-cnpj-unique` (`sql/19_revert_clientes_cnpj_unique.sql`)
**Situação:** aplicada no banco local em 2026-09-25. Ela não faz parte do `db-up`/`db-seed`: é para bancos que já existiam antes do NEG-01.

### Situação de origem

O `dados/crm/clientes.csv` tinha **40 CNPJs duplicados (80 linhas)**: o original (id até 3000) e uma cópia (ids 3001 a 3040) com a razão social em maiúsculas. Antes da migração, o banco tinha 3040 clientes.

### Regra de unificação

1. **Sobrevivente:** em cada grupo de CNPJ repetido, fica o cliente de **menor `cliente_id_origem`**. As outras linhas do grupo são as **cópias**.
2. **Transferência:** os registros ligados às cópias nas quatro tabelas com FK para `clientes` (`pedidos`, `carteiras`, `oportunidades`, `visitas`) passam a apontar para o sobrevivente.
3. **Carteiras equivalentes:** `carteiras` tem o UNIQUE `uk_carteiras_cliente_vendedor_inicio` (`cliente_id`, `vendedor_id`, `data_inicio`). Uma carteira da cópia só é **descartada** quando o vínculo equivalente (mesmo `vendedor_id` e mesma `data_inicio`) já existe no grupo: no sobrevivente ou em outra cópia com `carteira_id_origem` menor, que é a mantida. Qualquer outro vínculo é **transferido**, e o histórico é preservado. A linha descartada fica salva inteira (JSON) no log.
   - Não existe UNIQUE de "vínculo ativo". Se o sobrevivente ficar com dois vínculos ativos (`data_fim` NULL), isso é dado de negócio: o script não resolve sozinho.
4. **Exclusão das cópias:** as cópias são apagadas. O `ativo` do sobrevivente não muda.
5. **Índice:** cria o `uq_clientes_cnpj` e só depois apaga o `idx_clientes_cnpj`, para a coluna nunca ficar sem índice.

### Ordem de execução e proteções

1. Monta o mapa cópia → sobrevivente (tabelas temporárias) e a lista de carteiras a descartar.
2. Grava o backup das cópias em `clientes_merge_backup_20260925`.
3. Numa **única transação**:
   1. grava o log em `clientes_merge_backup_20260925_vinculos` (antes de mexer em qualquer dado);
   2. apaga as carteiras equivalentes;
   3. transfere pedidos, oportunidades, visitas e carteiras para o sobrevivente;
   4. apaga as cópias, **só as que não têm mais nenhum filho**. As FKs de `carteiras`, `oportunidades` e `visitas` são `ON DELETE CASCADE`, e essa guarda impede que um filho seja apagado sem aviso.
4. Troca o índice (DDL fora da transação, porque DDL faz commit implícito no MySQL).

Se sobrar alguma cópia, o `ADD UNIQUE` falha com **1062** e nenhum dado é perdido. O script é **idempotente**: rodar de novo sem duplicados só garante o índice.

### Resultado no banco local (NEG-01, 2026-09-25)

- `clientes` = **3000**, grupos de CNPJ duplicado = **0**.
- `SHOW INDEX FROM clientes WHERE Column_name = 'cnpj'` mostra só o `uq_clientes_cnpj` (`Non_unique` = 0).
- `pedidos` = 28732, `carteiras` = 3637, `oportunidades` = 5980, `visitas` = 37936 (contagens do roteiro do Lote 4, item 1.5).
- `clientes_merge_backup_20260925` = **40** linhas.
- `clientes_merge_backup_20260925_vinculos` = **0** linhas (DB-01, abaixo).

---

## 4. Tabelas de backup da migração 19

Criadas pela migração 19 e **não apagadas** pela reversão. Só apague manualmente depois de conferir que a reversão não será mais necessária.

### `clientes_merge_backup_20260925`

- Uma linha por **cópia removida**: as mesmas colunas de `clientes` (PK `cliente_id_origem`), mais `merged_into_cliente_id` (o sobrevivente em que a cópia foi unificada).
- Criada com `CREATE TABLE ... AS SELECT` na primeira execução. Em execuções seguintes, um `INSERT IGNORE` acrescenta cópias novas, se aparecerem.

### `clientes_merge_backup_20260925_vinculos` (log de vínculos)

| Campo | Tipo | Descrição |
| --- | --- | --- |
| `id` | BIGINT AUTO_INCREMENT | PK. |
| `tabela` | VARCHAR(64) | Tabela filha: `pedidos`, `carteiras`, `oportunidades` ou `visitas`. |
| `registro_id` | BIGINT | PK do registro na tabela filha. |
| `cliente_id_antigo` | BIGINT | Cópia à qual o registro pertencia. |
| `cliente_id_novo` | BIGINT | Sobrevivente. |
| `acao` | ENUM('transferido','descartado') | `descartado` = carteira equivalente já existia no grupo. |
| `dados` | JSON (nulo) | Linha completa da carteira, só quando `acao = 'descartado'`. |
| `created_at`, `updated_at` | TIMESTAMP | Controle. |

Índice único: `uq_merge_vinculos_tabela_registro` (`tabela`, `registro_id`).

**Log vazio é o esperado (DB-01, fechado em 2026-09-25).** Com os dados atuais, o log fica com **0 linhas**. As 40 cópias (ids 3001 a 3040) nunca tiveram filhos: os CSVs de `dados/crm` e `dados/erp` não têm `cliente_id` acima de 3000, então não houve registro transferido nem descartado. O script grava o log antes do DELETE e dos UPDATEs, na mesma transação e com o mesmo filtro. O log só teria linhas se as cópias tivessem filhos. Conferência:

```sql
SELECT (SELECT COUNT(*) FROM pedidos       WHERE cliente_id > 3000) AS pedidos,
       (SELECT COUNT(*) FROM carteiras     WHERE cliente_id > 3000) AS carteiras,
       (SELECT COUNT(*) FROM oportunidades WHERE cliente_id > 3000) AS oportunidades,
       (SELECT COUNT(*) FROM visitas       WHERE cliente_id > 3000) AS visitas;
-- esperado: 0 em todas
```

---

## 5. Reversão (`make db-revert-cnpj-unique`)

`sql/19_revert_clientes_cnpj_unique.sql` usa as duas tabelas de backup:

1. Recria o índice simples `idx_clientes_cnpj` e depois apaga o `uq_clientes_cnpj`.
2. Reinsere as cópias em `clientes`, com o mesmo `cliente_id_origem`.
3. Reinsere as carteiras descartadas a partir do JSON do log.
4. Devolve os filhos transferidos ao cliente de origem, também a partir do log.

É idempotente. Com o log vazio (DB-01), a reversão continua funcionando: ela reinsere as 40 cópias, e os passos 3 e 4 não alteram nada.

---

## 6. Consultas de verificação

```sql
SELECT COUNT(*) AS clientes FROM clientes;                                   -- 3000
SELECT COUNT(*) AS grupos_duplicados
  FROM (SELECT cnpj FROM clientes GROUP BY cnpj HAVING COUNT(*) > 1) d;      -- 0
SHOW INDEX FROM clientes WHERE Column_name = 'cnpj';                         -- só uq_clientes_cnpj, Non_unique = 0
SELECT COUNT(*) AS copias_no_backup FROM clientes_merge_backup_20260925;     -- 40
SELECT tabela, acao, COUNT(*) FROM clientes_merge_backup_20260925_vinculos
 GROUP BY tabela, acao;                                                      -- nenhuma linha (DB-01)
```

Referências: cards NEG-01, NEG-03, NEG-04 e DB-01 em `tarefas/feito.md`; roteiro `docs/roteiro-teste-manual-lote4.md`, seção 1.

---

## 7. Tabela `refresh_tokens` e migração 21 (SEC-07, Lote 6, 2026-09-26)

Guarda os refresh tokens (hash SHA-256) emitidos no login e no refresh. Cada refresh é de uso único: o token antigo é revogado e um novo é gravado na mesma transação.

| Campo | Tipo | Nulo | Descrição |
| --- | --- | --- | --- |
| `id` | BIGINT AUTO_INCREMENT | não | PK. É o `token_id` dos logs de `[auth]`. |
| `usuario_id` | BIGINT | não | FK `fk_refresh_token_usuario` → `usuarios.id` (`ON DELETE CASCADE`, `ON UPDATE CASCADE`). |
| `token_hash` | VARCHAR(255) | não | SHA-256 do refresh token. **Único** (`uk_refresh_token_hash`). O token em texto puro nunca é gravado. |
| `expires_at` | DATETIME | não | Expiração. |
| `revoked_at` | DATETIME | sim | Data/hora da revogação. `NULL` = ativo. |
| `revoked_reason` | ENUM('rotacao','logout','revogacao_massa','senha','inativacao') | sim | **Novo (migração 21).** Motivo da revogação, gravado junto com `revoked_at`. `NULL` = token ativo ou revogado antes da migração (legado). |
| `ip_origem` | VARCHAR(45) | sim | IP que pediu o token (IPv4/IPv6). |
| `user_agent` | TEXT | sim | User-Agent no momento da criação. |
| `created_at`, `updated_at` | TIMESTAMP | não | Controle. |

**Índices:** `PRIMARY` (`id`), `uk_refresh_token_hash` (UNIQUE, `token_hash`), `idx_refresh_usuario_id`, `idx_refresh_expires_at` e `idx_refresh_revoked_at`. A migração 21 **não** cria índice para `revoked_reason`: as buscas são por `token_hash` e `usuario_id`, e o motivo só é lido depois de achar a linha.

### Valores de `revoked_reason`

| Valor | Quando é gravado | Onde no código |
| --- | --- | --- |
| `rotacao` | `POST /api/auth/refresh`: o token antigo é trocado por um novo. | `RefreshTokenService.BeginRotation` |
| `logout` | `POST /api/auth/logout`. | `auth_handler.go` (Logout) |
| `revogacao_massa` | Reuso, fora da janela de 30 s, de um token com motivo `rotacao` (possível roubo): todos os tokens ativos do usuário são revogados. | `auth_handler.go` (`tratarTokenRevogadoForaDaJanela`) |
| `senha` | Troca de senha pelo usuário (`POST /api/auth/reset-password`) ou reset pelo admin (`POST /api/admin/reset-password`). | `auth_handler.go`, `usuario_handler.go` |
| `inativacao` | Inativação do usuário (`PATCH /api/usuarios/{id}/inativar`) ou desligamento do vendedor dele (`DELETE /api/vendedores/{id}`, na mesma transação, via `RevokeAllByVendedorID`). SEC-06. | `usuario_handler.go`, `vendedor_service.go` |

**Regra de uso (SEC-07, opção B):** só o reuso de um token com motivo `rotacao` fora da janela de graça gera o alerta `[auth][seguranca]` e a revogação em massa. Os demais motivos e o `NULL` legado geram só log informativo. A resposta HTTP é sempre `401` `"refresh token revogado"`.

### Migração 21

**Script:** `sql/21_alter_refresh_tokens_revoked_reason.sql`
**Comando:** `make db-fix-revoked-reason`
**Reversão:** `make db-revert-revoked-reason` (`sql/21_revert_refresh_tokens_revoked_reason.sql`; remove a coluna e perde os motivos gravados)
**Situação:** aplicada no banco local em 2026-09-26 (107 tokens na tabela; os já revogados antes da migração ficaram com `NULL`).

- Adiciona `revoked_reason` logo depois de `revoked_at`, com `DEFAULT NULL` e COMMENT "Motivo da revogação (NULL = ativo ou revogado antes do SEC-07/legado)".
- Não altera nenhuma linha: os tokens legados ficam `NULL` de propósito, porque não há como saber o motivo real.
- **Idempotente:** consulta `information_schema.COLUMNS` e só roda o `ALTER TABLE` se a coluna ainda não existir.
- Como a 19 e a 20, não faz parte do `db-up`/`db-seed`/`db-reset`. Bancos novos já recebem a coluna pelo `sql/06_ddl_refresh_tokens.sql` (atualizado no mesmo card).

### Consultas de verificação

```sql
SHOW FULL COLUMNS FROM refresh_tokens LIKE 'revoked_reason';
-- Type = enum('rotacao','logout','revogacao_massa','senha','inativacao'), Null = YES, Default = NULL

SELECT revoked_reason, COUNT(*) AS tokens,
       SUM(revoked_at IS NULL) AS ativos
  FROM refresh_tokens
 GROUP BY revoked_reason;
-- ativos só aparecem na linha revoked_reason = NULL

SELECT COUNT(*) FROM refresh_tokens
 WHERE revoked_at IS NULL AND revoked_reason IS NOT NULL;   -- 0 (ativo nunca tem motivo)
```

Referências: cards SEC-06 e SEC-07 em `tarefas/fazendo.md`; roteiro `docs/roteiro-teste-manual-lote6.md`, seção 5.
