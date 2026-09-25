# A Fazer

---

## SEC-02: corrida no refresh token gera dois pares de tokens — prioridade MÉDIA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Backend (+ Frontend)
**Origem:** Lote 3 de 2026-09-24.

**Descrição:** `apis/rotaperfumes-api/handlers/auth_handler.go` (~l.311-357) valida e revoga o refresh token em passos separados e ignora o erro da revogação. Duas chamadas de refresh simultâneas com o mesmo token geram dois pares de tokens válidos.

**Ação esperada:**
- 🟡 BackBrain:
  - `Revoke` com `WHERE id = ? AND revoked_at IS NULL`.
  - Se `n == 0`, devolver `ErrRefreshTokenRevoked` e responder `401` antes de gerar os novos tokens.
  - Registrar a falha no `refreshLimiter`.
- 🟢 FrontBrain (multi-aba): repetir a requisição original uma vez antes de deslogar.
- 🔴 TestBrain: dois refresh em paralelo com o mesmo token → um `200` e um `401`.
- 🔵 SubBrain atualizar o Postman.

---

## SEC-03: `GET /api/vendedores` sem escopo — prioridade BAIXA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Backend
**Origem:** Lote 3 de 2026-09-24.

**Descrição:** Em `apis/rotaperfumes-api/handlers/vendedor_handler.go` (~l.41), o usuário `normal` recebe todos os vendedores, e o bloqueio de vendedor desligado não é aplicado.

**Ação esperada:**
- 🟣 SecBrain / 🟡 BackBrain:
  - Devolver ao usuário `normal` só o próprio vendedor e aplicar o bloqueio de vendedor desligado.
  - Avaliar também o bloqueio de desligado em `GET /api/produtos` e `GET /api/produtos/{id}`.
- 🟢 FrontBrain: conferir os selects que usam a listagem.
- 🔴 TestBrain cobrir.
- 🔵 SubBrain atualizar o Postman.

---

## NEG-01: validação e duplicidade de CNPJ (DECISÃO DE NEGÓCIO) — prioridade BAIXA

**Status:** não iniciado. Aguarda a priorização e a decisão do usuário.
**Camada:** Backend / Database
**Origem:** Lote 3 de 2026-09-24.

**Descrição:** O índice `idx_clientes_cnpj` não é `UNIQUE`, e não há validação de 14 dígitos no CNPJ.

**Ação esperada:**
- 🤍 MegaBrain / 🔵 SubBrain levar a decisão ao usuário.
- Proposta:
  - `400` "cnpj inválido" para CNPJ fora do formato.
  - Política de duplicidade com `409` genérico, sem revelar a qual vendedor o cliente pertence.
- Depois da decisão: 🌸 DataBrain (índice), 🟡 BackBrain, 🔴 TestBrain e 🔵 SubBrain (Postman).

---

## BUG-06: `PATCH /inativar` com body inválido inverte o estado — prioridade MÉDIA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Backend
**Origem:** Lote 3 de 2026-09-24.

**Descrição:** Três handlers ignoram o erro de decode do body:
- `cliente_handler.go` (~l.222)
- `produto_handler.go` (~l.113)
- `usuario_handler.go` (~l.303)

Um body inválido, como `{"ativo":"false"}` (string), é tratado como omitido e **alterna** o estado.

**Ação esperada:**
- 🟡 BackBrain responder `400` quando o body vier preenchido e inválido. O body vazio continua fazendo toggle.
- 🔴 TestBrain cobrir.
- 🔵 SubBrain atualizar o Postman.

---

## FE-04: resposta obsoleta sobrescreve a mais nova nas listagens — prioridade MÉDIA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Frontend
**Origem:** Lote 3 de 2026-09-24 (documentado pelo 🔴 TestBrain durante o FE-03). O problema já existia antes do FE-03.

**Descrição:** O problema aparece de três formas:
1. **Paginação rápida:** a resposta de uma página antiga sobrescreve a da página atual. Efeitos afetados:
   - `admin/pedidos:181`, `pagamentos:181`, `admin/clientes:181`
   - `oportunidades:281`, `visitas:249`
   - `estoque:125`, `produtos:136`
   - provavelmente também `usuarios:131` e `senha-historico:165`
2. **Debounce dos filtros disparado na montagem** (`setTimeout` em `pedidos:187` etc.) com `page=1`:
   - A listagem faz duas buscas ao abrir.
   - A tabela pode voltar à página 1 enquanto o paginador mostra a página 2.
3. **Linha excluída reaparece** em pedidos, pagamentos, oportunidades e visitas.

**Ação esperada:**
- 🟢 FrontBrain:
  - Aplicar o padrão `cancelado`/chave (como no Dashboard e nos modais).
  - Não disparar o debounce na montagem.
- 🔴 TestBrain: os 18 testes `it.fails` já existem. Ao corrigir, trocar `it.fails` por `it`.

---

## FE-05: listagens chamam `fetch` direto, sem refresh automático no 401 — prioridade BAIXA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Frontend
**Origem:** Lote 3 de 2026-09-24.

**Descrição:**
- As funções `apiList*`, `apiDashboardVendedores` e `apiListSenhaHistorico` (`frontend/src/lib/api.ts`) chamam `fetch` direto. Por isso, um `401` não dispara o refresh automático do `fetchWithAuth`.
- Incluir na mesma correção: em `frontend/src/app/admin/clientes/page.tsx` (~l.96), enquanto o `/me` carrega, o motivo do botão desabilitado diz "Usuario sem vendedor vinculado". O problema é só de texto.

**Ação esperada:**
- 🟢 FrontBrain migrar essas funções para `fetchWithAuth` e ajustar o texto durante o carregamento.
- 🔴 TestBrain cobrir.
