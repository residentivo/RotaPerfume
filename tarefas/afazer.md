# A Fazer

---

## SEC-01: IDOR em Create/Update/Toggle de clientes — prioridade ALTA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Backend
**Origem:** 🟣 SecBrain, 2026-09-24, durante a definição do contrato de bloqueio do vendedor desligado.

**Descrição:** Em `apis/rotaperfumes-api/handlers/cliente_handler.go`, três handlers não chamam `resolverVendedorScope`:
- `CreateCliente` (~l.251)
- `UpdateCliente` (~l.291)
- `ToggleAtivoCliente` (~l.169)

Com isso, qualquer usuário `normal` consegue editar ou inativar **qualquer** cliente pelo id. O lote atual só adiciona nesses handlers o bloqueio do vendedor desligado.

**Ação esperada:**
- 🟡 BackBrain restringir Update e Toggle à carteira ativa via `clienteNaCarteiraDoVendedor`, respondendo `404` "cliente não encontrado" para clientes fora da carteira. Definir também a regra do Create para o usuário `normal`.
- 🔴 TestBrain cobrir com testes.
- 🔵 SubBrain atualizar o Postman.

---

## BUG-04: salvar registro sem alterações retorna 404 "não encontrado" — prioridade MÉDIA/ALTA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Backend
**Origem:** 🔴 TestBrain, 2026-09-24.

**Descrição:** Salvar um registro sem nenhuma alteração retorna `404` "não encontrado".
- **Causa:** o DSN não tem `clientFoundRows=true` (`apis/shared/config/config.go:138` e `apis/shared/cmd/resetpassword/main.go:54`). O MySQL devolve `RowsAffected=0` em um `UPDATE` que não muda nada, e os repositórios tratam `n == 0` como `ErrNotFound`.
- **Confirmado em:** `vendedor_repository.go:152-157` e `produto_repository.go:242-247`.
- **Mesmo padrão em:**
  - `cliente:188`, `estoque:247`, `oportunidade:205`, `pagamento:201`, `pedido:329` e `visita:187`
  - `usuario:142/196/210/225`
  - carteira
- **Agravante:** ficou mais provável depois do BUG-01, porque re-salvar a mesma data não altera mais a linha.
- **Teste já existente (pulado):** `TestIntegracao_UpdateSemAlteracao`.

**Ação esperada:**
- 🟣 SecBrain avaliar o impacto de `clientFoundRows=true`, por exemplo nas checagens de revogação de token e de lockout que dependem de `RowsAffected`.
- 🟡 BackBrain corrigir.
- 🔴 TestBrain reativar o teste `TestIntegracao_UpdateSemAlteracao`.

---

## RISCO-01: datas antigas de início de horário de verão com `TZ=America/Sao_Paulo` (Linux/Docker) — prioridade BAIXA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Backend
**Origem:** lote de 2026-09-24, durante o BUG-01 (card em `feito.md`).

**Descrição:** Em datas de início de horário de verão, a meia-noite não existe (ex.: `2018-11-04`). Nesse caso, `time.ParseInLocation` devolve 23:00 do dia anterior. Não ocorre no Windows. O caso está documentado em `TestBUG01_HorarioDeVeraoHistorico` (skip).

**Ação esperada:** decidir entre duas opções:
- tratar as datas puras em UTC de ponta a ponta;
- fixar `loc`/`TZ` do container em um fuso sem horário de verão.

---

## FE-01: corrida em `refreshSessionUser` mantém o bloqueio de vendedor desligado após a reativação — prioridade BAIXA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Frontend
**Origem:** 2026-09-24, card "Segurança: vendedor desligado" (em `fazendo.md`).

**Descrição:** Em `frontend/src/lib/session.ts:68-75`, a revalidação de focus/mount reutiliza a promise em voo originada pelo `403` (`origem403: true`) e não limpa `bloqueado403`. Com isso, o bloqueio pode continuar na tela depois que o vendedor é reativado. Corrige sozinho no próximo focus.

**Ação esperada:** 🟢 FrontBrain corrigir a corrida. 🔴 TestBrain cobrir o cenário em `session.test.ts`.

---

## FE-02 (nota de design): Dashboard depende da API para zerar os dados de vendedor desligado / sem vendedor — prioridade BAIXA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Frontend
**Origem:** 2026-09-24, card UI-02 (em `fazendo.md`).

**Descrição:** `frontend/src/app/dashboard/page.tsx:603-616` não força zeros para o usuário `normal` sem vendedor ou com vendedor desligado. Hoje o backend já devolve os dados zerados, então a tela fica correta. Se a API mudar, a tela pode exibir números.

**Ação esperada:** 🟢 FrontBrain avaliar se o frontend deve forçar os zeros nesses dois casos.

---

## FE-03: 34 warnings `react-hooks/set-state-in-effect` — prioridade BAIXA

**Status:** não iniciado. Aguarda a priorização do usuário.
**Camada:** Frontend
**Origem:** 2026-09-24, card "Tooling frontend" (em `feito.md`).

**Descrição:** O ESLint configurado neste lote aponta 34 warnings `react-hooks/set-state-in-effect`. Por enquanto, a regra está como `warn`.

**Ação esperada:** 🟢 FrontBrain fazer um refactor dedicado para eliminar o `setState` dentro de `useEffect` e voltar a regra para `error`. 🔴 TestBrain garantir que os testes continuam verdes.
