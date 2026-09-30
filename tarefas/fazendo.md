# Fazendo

## Lote 12 (2026-09-27): SEC-11, DB-01, SEC-12, CHORE-02

> Aberto pelo 🤍 MegaBrain a partir do pedido "faça os cards a fazer da lista de tarefas". INFO-01 fica em `afazer.md` (informativo).

**Fluxo:** 🟣 SecBrain (especifica SEC-11, SEC-12, CHORE-02) ∥ 🌸 DataBrain (DB-01) → 🟡 BackBrain (SEC-11, SEC-12, CHORE-02) → 🔴 TestBrain → 🔵 SubBrain (documentação, Postman, manuais e fechamento em `feito.md`).

**Passo atual:** 🟣 SecBrain especificando SEC-11/SEC-12/CHORE-02; 🌸 DataBrain executando DB-01.

> Último lote fechado: **Lote 11 de 2026-09-26** (FE-14, SEC-10, SEC-09, DOC-04; ver `feito.md`).

---

## SEC-11: credenciais do banco no Makefile e no `config.Load()` — prioridade MÉDIA

**Status:** em execução (Lote 12)
**Passo atual:** 🟣 SecBrain especificando SEC-11/SEC-12/CHORE-02; 🌸 DataBrain executando DB-01.
**Camada:** Segurança (+ Repositório/Makefile + Backend shared)
**Responsável:** 🟣 SecBrain (avaliação) → 🟡 BackBrain (aplicação) → 🔴 TestBrain
**Origem:** 🟣 SecBrain, SEC-10 (Lote 11, 2026-09-26).

**Descrição:**
- O Makefile exporta `DB_USUARIO ?= golang` / `DB_SENHA ?= golang`, e o `godotenv` não sobrescreve variáveis já existentes: sob `make`, as credenciais do `.env` são ignoradas.
- `MYSQL_OPTS` passa `-p$(DB_SENHA)` na linha de comando (visível na lista de processos).
- `config.Load()` também tem default `golang/golang` (o SEC-10 só removeu o fallback de `seedusers` e `resetpassword`).

**Ação esperada (proposta):** `-include .env` no Makefile; remover os defaults; usar `MYSQL_PWD` em vez de `-p`; `config.Load()` falhar sem `DB_USUARIO`/`DB_SENHA`.

---

## DB-01: tabela `estoque` fora do `make db-up`/`db-reset` — prioridade MÉDIA

**Status:** em execução (Lote 12)
**Passo atual:** 🟣 SecBrain especificando SEC-11/SEC-12/CHORE-02; 🌸 DataBrain executando DB-01.
**Camada:** Database (+ Makefile + Documentação)
**Responsável:** 🌸 DataBrain (documentação: 🔵 SubBrain)
**Origem:** 🔵 SubBrain, DOC-04 (Lote 11, 2026-09-26): itens 1–4 da seção 22 de `docs/manual-base-de-dados.md`.

**Descrição:**
1. O `make db-up` não roda `sql/17_ddl_estoque.sql` nem a `18`: `db-reset` não cria `estoque`, e o `db-rebuild` falharia no `db-import-estoque`.
2. O `17_ddl_estoque.sql` ainda cria a coluna `origem`, removida pela migração 18.
3. A migração 18 não é idempotente, não tem alvo no Makefile nem revert.
4. `postman/README.md` diz que o `db-up` cria `estoque`.

**Ação esperada:** 🌸 DataBrain alinha o DDL 17 ao schema atual, inclui `estoque` no `db-up`, torna a 18 idempotente (com alvo e revert); 🔵 SubBrain corrige o README e o manual.

---

## SEC-12: reenvio de refresh rotacionado derruba as sessões da vítima repetidamente — prioridade BAIXA

**Status:** em execução (Lote 12)
**Passo atual:** 🟣 SecBrain especificando SEC-11/SEC-12/CHORE-02; 🌸 DataBrain executando DB-01.
**Camada:** Segurança (+ Backend)
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, SEC-09 (Lote 11, 2026-09-26).

**Descrição:** Quem tem um refresh já rotacionado e não expirado pode reenviá-lo e derrubar as sessões da vítima a cada vez (cada reuso regrava o corte do SEC-08). Limites atuais: rate limit de 10/min por IP e a expiração do token. O e-mail do SEC-09 tem dedup de 30 min, mas a revogação não.

**Ação esperada (possível):** não regravar o corte se o último reuso desse token foi há menos de N minutos.

---

## CHORE-02: `refresh_tokens` sem limpeza automática — prioridade BAIXA

**Status:** em execução (Lote 12)
**Passo atual:** 🟣 SecBrain especificando SEC-11/SEC-12/CHORE-02; 🌸 DataBrain executando DB-01.
**Camada:** Backend (+ Database)
**Responsável:** 🟡 BackBrain (testes: 🔴 TestBrain)
**Origem:** 🔵 SubBrain, DOC-04 (Lote 11, 2026-09-26): item 11 da seção 22 do manual.

**Descrição:** `CleanupExpired`/`DeleteExpired` existem, mas só são chamados em testes; a tabela cresce sem limite.

**Ação esperada:** agendar a limpeza (rotina periódica na API ou alvo no Makefile), preservando os tokens revogados pelo tempo necessário à detecção de reuso.
