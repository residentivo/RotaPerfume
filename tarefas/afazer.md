# A Fazer

> Cards registrados durante o Lote 5 (2026-09-25). Aguardam a priorização do usuário.

---

## BUG-08: `POST /api/clientes` devolve `created_at` e `updated_at` zerados — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Backend
**Origem:** 🔴 TestBrain, execução do roteiro do Lote 5 (2026-09-25). É anterior ao lote.

**Descrição:** A resposta 201 traz `0001-01-01T00:00:00Z` nesses campos. O motivo: `apis/rotaperfumes-api/services/cliente_service.go:216-228` e `:238-271` devolvem o objeto em memória sem reler do banco.

**Ação esperada:**
- 🟡 BackBrain: reler o registro depois do INSERT e do UPDATE.
- 🔴 TestBrain: cobrir.

---

## FE-09: listagem de clientes mostra "Nenhum cliente cadastrado." quando há erro de rede — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Frontend
**Origem:** 🔴 TestBrain, execução do roteiro do Lote 5 (2026-09-25). É anterior ao lote.

**Descrição:** Com erro de rede, `frontend/src/app/admin/clientes/page.tsx:152-162` zera a lista. A tela passa a mostrar "Nenhum cliente cadastrado." ao lado do alerta de conexão.

**Ação esperada:**
- 🟢 FrontBrain: manter a última lista ou ocultar o estado vazio quando houver erro, e conferir se outras listagens repetem o padrão.

---

## DOC-03: outros documentos e o Postman ainda usam `henrique.rodrigues` (inativo) — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Documentação
**Origem:** 🔵 SubBrain, DOC-01 (2026-09-25).

**Descrição:** O usuário id 2 (`henrique.rodrigues`) está com `ativo=0` e não faz login, mas ainda aparece em:
- `docs/roteiro-teste-manual-clientes.md`, linha 30;
- `docs/roteiro-teste-manual-dashboard.md`, linha 21;
- `postman/README.md`, linhas 37 e 60;
- `postman/collection.json`: requisição "Login — Vendedor" e o exemplo de resposta.

**Ação esperada:**
- 🔵 SubBrain: trocar por um usuário ativo, seguindo o mesmo critério do DOC-01.

---

## SEC-06: sessão de usuário inativado continua válida até o access token expirar — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Backend (+ Segurança)
**Origem:** 🔵 SubBrain, DOC-01 (2026-09-25). Análise pelo código, ainda não validada no navegador.

**Descrição:**
- O middleware de autenticação não confere `usuarios.ativo`.
- Quando um vendedor é desligado pela tela, os usuários dele são inativados, mas a sessão aberta continua até o access token expirar.
- Só no refresh a API responde `401 "usuário inativo"` e o front faz logout.

**Ação esperada:**
- 🟣 SecBrain: avaliar se a janela é aceitável ou se é preciso revogar os refresh tokens ao inativar e/ou checar `ativo` no middleware.
- 🟡 BackBrain aplicar.
- 🔴 TestBrain cobrir.

---

## SEC-07: revogar a família de refresh tokens em caso de reuso fora da janela — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Backend (+ Segurança)
**Origem:** 🟣 SecBrain, SEC-04 (2026-09-25).

**Descrição:** Com o SEC-04, o reuso de um refresh token revogado fora da janela de graça só gera log `[seguranca]`. Revogar todos os tokens do usuário (`RevokeAllUserTokens`) protege melhor contra roubo de token. Por outro lado, pode deslogar usuários legítimos cujo cookie novo se perdeu.

**Complemento (🔴 TestBrain, regressão do Lote 5):**
- Um token revogado por logout ou por `RevokeAllByUser` e reusado por uma aba antiga depois de 30s dispara o alerta `[auth][seguranca]` de "possível roubo". É um falso positivo.
- O log não traz o `user_id`.

**Ação esperada:**
- 🟣 SecBrain: detalhar o trade-off.
- Usuário: decidir.

---

## DOC-04: manual completo da base de dados — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Documentação
**Origem:** 🔵 SubBrain, DOC-02 (2026-09-25).

**Descrição:** O `docs/manual-base-de-dados.md` foi criado no DOC-02 com o detalhe da tabela `clientes`, do `uq_clientes_cnpj` e da migração 19. As demais tabelas aparecem só no índice.

**Ação esperada:**
- 🔵 SubBrain: detalhar as demais tabelas, se o usuário quiser o manual completo.

---

## FE-07: assimetria de logout entre falha de rede no refresh e falha de rede no retry — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Frontend (+ Segurança)
**Origem:** 🔴 TestBrain, regressão do Lote 5 (2026-09-25).

**Descrição:**
- Quando o próprio `POST /api/auth/refresh` falha por rede ou timeout, o front faz logout.
- Quando é o retry após o refresh que falha por rede (FE-06), o front não faz logout.
- Esse comportamento vem de antes do lote e agora está coberto por teste.

**Ação esperada:**
- 🟣 SecBrain e 🟢 FrontBrain: avaliar se uma falha de rede no refresh deveria preservar a sessão.

---

## TEST-01: nome de subteste e comentário de CNPJ desatualizados — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Testes
**Origem:** 🔴 TestBrain, regressão do Lote 5 (2026-09-25).

**Descrição:** Em `apis/rotaperfumes-api/handlers/lote4_http_integration_test.go`, o subteste ainda se chama "CNPJ mascarado é gravado só com dígitos". Com o NEG-02, o nome não descreve mais a regra. Do mesmo jeito, o comentário de `TestCruzado_Divergencias` em `apis/shared/cnpj/cruzado_test.go` ainda cita `it.fails` no front, que foi removido na correção NEG-02-A/B.

**Ação esperada:**
- 🔴 TestBrain: renomear o subteste e ajustar o comentário.
