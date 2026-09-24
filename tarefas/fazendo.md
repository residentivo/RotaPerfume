# Fazendo

> Lote iniciado em **2026-09-24**. O 🤍 MegaBrain executou os 10 cards que vieram de `afazer.md`. 8 foram concluídos e movidos para `feito.md`. Os 2 cards abaixo aguardam a validação do usuário no navegador.

---

## Teste manual/e2e do `PedidoModal` no navegador (usuário normal e admin)

**Início:** 2026-09-24
**Passo atual:** Validação via API concluída pelo 🔴 TestBrain (ver roteiro em `docs/roteiro-teste-manual-pedidomodal.md`); aguardando o usuário executar o roteiro no navegador.

**Origem:** 🟢 FrontBrain, durante o card "Scope check em Create/Update de Pedidos" (2026-09-23).

**Descrição:** As mudanças em `frontend/src/components/admin/PedidoModal.tsx` foram validadas só com typecheck. Não houve teste no navegador. As mudanças são:
- Vendedor travado para usuário `normal`.
- Aviso e botão Salvar bloqueado quando não há vendedor vinculado.
- Cascata vendedor → cliente via `apiListClientesDoVendedor`.
- Cliente "(fora da carteira)" mantido na edição.
- Mensagem amigável para o `400` de carteira.

**DECISÃO DO USUÁRIO (2026-09-24):**
- 🔴 TestBrain valida via API com os perfis `admin`, `normal` com vendedor e `normal` sem vendedor.
- 🔴 TestBrain escreve em `docs/` um roteiro de checagem manual para o usuário executar no navegador.
- **O card permanece em `fazendo.md` até o usuário validar.**

**Andamento (2026-09-24):**
- 🔴 TestBrain validou via API todos os fluxos (admin, normal com vendedor, normal sem vendedor, normal com vendedor desligado) e registrou as evidências em `docs/roteiro-teste-manual-pedidomodal.md`.
- Neste lote foi registrado em `afazer.md` o **BUG-01** (`data_pedido` gravada com um dia a menos). O bug afeta este fluxo: cada re-salvamento no `PedidoModal` pode recuar mais um dia. Leve isso em conta ao executar o roteiro.

**Ação esperada:** validar os fluxos abaixo:
- Criar e editar pedido como `admin`.
- Criar e editar pedido como `normal` com vendedor vinculado.
- Usuário `normal` sem vendedor vinculado.
- Edição de pedido com cliente fora da carteira.

Registrar evidências e abrir bugs, se houver.

---

## Teste manual do Dashboard no navegador (normal com vendedor, normal sem vendedor, admin)

**Início:** 2026-09-24
**Passo atual:** Validação via API concluída pelo 🔴 TestBrain (ver roteiro em `docs/roteiro-teste-manual-dashboard.md`); aguardando o usuário executar o roteiro no navegador.

**Origem:** 🟢 FrontBrain, durante o card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23).

**Descrição:** As mudanças em `frontend/src/app/dashboard/page.tsx` não foram validadas no navegador. As mudanças são:
- Rótulos "Minhas Vendas", "Meus Pedidos" e "Minha Meta".
- Card "Meu Desempenho" no lugar do ranking.
- Aviso para usuário sem vendedor vinculado.
- Tolerância a `null` e `[]`.
- (2026-09-24) Aviso de vendedor desligado, baseado no campo `vendedor_desligado` da API.
- (2026-09-24) Rótulos "Meus Clientes" e "Clientes da minha carteira" para o usuário `normal`.
- (2026-09-24) Valores monetários com 2 casas decimais (`fmtCurrency`).

**DECISÃO DO USUÁRIO (2026-09-24):**
- 🔴 TestBrain valida via API com os perfis.
- 🔴 TestBrain escreve em `docs/` um roteiro de checagem manual para o usuário executar no navegador.
- **O card permanece em `fazendo.md` até o usuário validar.**
- O roteiro deve incluir também os novos casos deste lote: vendedor desligado e `/api/dashboard/clientes` com escopo da carteira.

**Andamento (2026-09-24):**
- 🔴 TestBrain validou via API os perfis admin, normal com vendedor, normal sem vendedor e normal com vendedor desligado, incluindo `/api/dashboard/clientes` com escopo da carteira. As evidências estão em `docs/roteiro-teste-manual-dashboard.md`.
- Divergências registradas em `afazer.md`:
  - **UI-01:** título do gráfico fixo em "Vendas nos Ultimos 30 Dias".
  - **UI-02:** para normal sem vendedor e com vendedor desligado, a tela mostra só o aviso, sem os números zerados e o gráfico esperados abaixo. É preciso confirmar com o usuário qual layout é o desejado.

**Ação esperada:** validar com os perfis abaixo:
- `normal` com vendedor: só os próprios números e a própria meta.
- `normal` sem vendedor: aviso, números zerados e gráfico com dias zerados (ver UI-02).
- `normal` com vendedor desligado: aviso de vendedor desligado e números zerados (ver UI-02).
- `admin`: todos os números e o ranking completo.

Registrar evidências e abrir bugs, se houver.
