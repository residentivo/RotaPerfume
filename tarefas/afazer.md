# A Fazer

> **Lote 4 concluído em 2026-09-25.** SEC-02, SEC-03, NEG-01, BUG-06, FE-04 e FE-05, mais os derivados SEC-05, NEG-03, NEG-04 e DB-01, estão todos em `feito.md` (aceite do usuário em 2026-09-25). Nenhuma divergência do roteiro do Lote 4 ficou pendente.
>
> **Follow-ups do Lote 4 (2026-09-25):** os cards abaixo foram registrados pelo 🔴 TestBrain durante a regressão do Lote 4. Eles continuam no backlog e aguardam a priorização do usuário.

---

## FE-06: mensagem enganosa quando o retry após o refresh falha por rede — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Frontend
**Origem:** 🔴 TestBrain, Lote 4 (2026-09-25).

**Descrição:** Em `frontend/src/lib/apiClient.ts`, quando o retry após o refresh com `401` falha por erro de rede, as requisições da fila são rejeitadas com "Sessão expirada. Faça login novamente.", mas o usuário não é deslogado. A mensagem é enganosa.

**Ação esperada:**
- 🟢 FrontBrain: usar uma mensagem de erro de rede nesse caso.
- 🔴 TestBrain cobrir.

---

## SEC-04: corrida legítima no refresh conta no rate limit por IP — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Backend (+ Segurança)
**Origem:** 🔴 TestBrain, Lote 4 (2026-09-25).

**Descrição:**
- Cada `401` de revogação concorrente no refresh conta no `refreshLimiter` por IP. Com 10 falhas, a API responde `429`.
- No front, um `429` no refresh leva ao logout.
- Várias abas, ou vários usuários atrás do mesmo IP (NAT), podem cair nisso.

**Ação esperada:**
- 🟣 SecBrain: avaliar a solução (por exemplo, não contar a corrida legítima ou usar uma chave por token/usuário).
- 🟡 BackBrain aplicar.
- 🔴 TestBrain cobrir.

---

## DOC-01: roteiro de vendedor desligado usa um usuário que não faz mais login — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Documentação
**Origem:** 🔴 TestBrain, Lote 4 (2026-09-25).

**Descrição:** `docs/roteiro-teste-manual-vendedor-desligado.md` usa `henrique.rodrigues` (usuário id 2), que está com `ativo=0` (efeito do BUG-05) e não faz mais login.

**Ação esperada:**
- 🔴 TestBrain / 🔵 SubBrain: trocar para `thiago.silva` (usuário id 4, vendedor 3 desligado) ou descrever o novo comportamento.

---

## NEG-02: aceitar o CNPJ alfanumérico da Receita (DECISÃO DE NEGÓCIO) — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Backend / Database / Frontend
**Origem:** 🔴 TestBrain, Lote 4 (2026-09-25).

**Descrição:** O CNPJ alfanumérico da Receita está vigente desde julho de 2026. Hoje a API aceita só dígitos, e o schema usa `CHAR(14)` com dígito verificador módulo 11 numérico.

**Ação esperada:**
- 🤍 MegaBrain: levar a decisão ao usuário.

---

## DOC-02: manual da base de dados com o índice `uq_clientes_cnpj` — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Documentação
**Origem:** 🔴 TestBrain, Lote 4 (2026-09-25).

**Descrição:** Atualizar o manual da base de dados com o índice `uq_clientes_cnpj` e a unificação dos CNPJs duplicados. Depende da aplicação da migração 19 (`make db-fix-cnpj-unique`).

**Atualização (2026-09-25):** a migração 19 está aplicada no banco local (confirmado na execução do roteiro do Lote 4). O card está desbloqueado. O DB-01 foi fechado (log de vínculos vazio é o esperado; ver `feito.md`).

**Ação esperada:**
- 🔵 SubBrain: atualizar o manual depois que a migração 19 for aplicada.
