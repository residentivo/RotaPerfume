# A Fazer

---

## BUG-01: `data_pedido` gravada com um dia a menos (fuso UTC vs `loc=Local`) — prioridade ALTA

**Camada:** Backend
**Origem:** lote de 2026-09-24, durante a validação do card "Teste manual/e2e do `PedidoModal`" (em `fazendo.md`).

**Descrição:** Um pedido enviado com `data_pedido: "2026-09-24"` é gravado como `2026-09-23`.
- **Causa provável:** `time.Parse("2006-01-02", ...)` gera meia-noite em UTC, e o DSN do MySQL usa `loc=Local` (UTC-3). Na conversão, a data recua para 21:00 do dia anterior.
- **Ponto confirmado:** `apis/rotaperfumes-api/services/pedido_service.go:163`.
- **Mesmo padrão (não reproduzido):** services de cliente, estoque, oportunidade, pagamento, produto, vendedor e visita.
- **Impacto:** cada re-salvamento de um pedido no `PedidoModal` pode recuar mais um dia, porque o frontend reenvia a data já deslocada.

**Ação esperada:**
- 🟡 BackBrain corrigir a causa: `time.ParseInLocation` com o mesmo fuso do DSN, ou ajuste do DSN. Revisar todos os services que usam o mesmo padrão.
- 🔴 TestBrain cobrir com testes: gravação e leitura da mesma data, e re-salvamento sem deslocamento.
- 🔵 SubBrain atualizar o Postman, se o formato das datas na resposta mudar.

---

## UI-01: título do gráfico do Dashboard (admin) fixo em "Vendas nos Ultimos 30 Dias"

**Camada:** Frontend (menor)
**Origem:** lote de 2026-09-24, durante a validação do card "Teste manual do Dashboard" (em `fazendo.md`).

**Descrição:** Em `frontend/src/app/dashboard/page.tsx`, o título do gráfico do admin continua "Vendas nos Ultimos 30 Dias" mesmo quando o select de dias muda para outro valor.

**Ação esperada:** 🟢 FrontBrain montar o título a partir do número de dias selecionado (ex.: "Vendas nos Últimos 7 Dias").

---

## UI-02: layout do Dashboard para normal sem vendedor / com vendedor desligado (CONFIRMAR COM O USUÁRIO)

**Camada:** Frontend (aguardando decisão)
**Origem:** lote de 2026-09-24, durante a validação do card "Teste manual do Dashboard" (em `fazendo.md`).

**Descrição:** Para o usuário `normal` sem vendedor vinculado e para o `normal` com vendedor desligado, `frontend/src/app/dashboard/page.tsx` mostra **só o aviso**. Não aparecem os KPIs zerados nem o gráfico com os dias zerados. O card original de teste manual esperava "aviso, números zerados e gráfico com dias zerados". A API devolve os dados zerados corretamente nos dois casos.

**Ação esperada:**
- 🔵 SubBrain / 🤍 MegaBrain confirmar com o usuário o layout desejado: (a) só o aviso, como hoje, ou (b) aviso + KPIs zerados + gráfico zerado.
- Se for (b), 🟢 FrontBrain ajustar `page.tsx` e 🔴 TestBrain atualizar `docs/roteiro-teste-manual-dashboard.md`.
- Se for (a), ajustar só a expectativa do roteiro e do card de teste manual.

---

## Segurança: vendedor desligado ainda pode criar/editar pedidos, pagamentos, oportunidades e visitas (DECISÃO DE NEGÓCIO)

**Camada:** Backend (aguardando decisão)
**Origem:** 🟣 SecBrain, lote de 2026-09-24 (relacionado ao card "Dashboard de vendedor desligado", em `feito.md`).

**Descrição:** A decisão do usuário de 2026-09-24 ("bloquear o acesso" do vendedor desligado) cobriu **só o Dashboard**. `resolverVendedorScope` (`apis/rotaperfumes-api/handlers/scope.go`) ignora `vendedores.data_desligamento`. Com isso, um usuário `normal` vinculado a um vendedor desligado continua podendo listar, criar e editar pedidos, pagamentos, oportunidades e visitas da própria carteira.

**Ação esperada:**
- 🤍 MegaBrain / 🔵 SubBrain levar a decisão ao usuário: bloquear também as escritas (e talvez as leituras) do vendedor desligado, ou manter como está.
- Se bloquear: 🟣 SecBrain definir os status (ex.: `403` "vendedor desligado"), 🟡 BackBrain aplicar em `resolverVendedorScope`, 🟢 FrontBrain exibir o aviso nas telas afetadas, 🔴 TestBrain cobrir e 🔵 SubBrain atualizar o Postman.

---

## UX: `id_vendedor` do localStorage desatualizado após o admin mudar o vínculo do usuário

**Camada:** Frontend
**Origem:** 🟣 SecBrain (2026-09-24).

**Descrição:** O frontend guarda `id_vendedor` no `localStorage` no login. Se o admin mudar o vínculo usuário → vendedor, as telas `PedidoModal`, Oportunidades e Visitas continuam travando o vendedor antigo até um novo login. Não há vazamento de dados: o backend é a fonte da verdade e aplica o vendedor correto. É só um problema de experiência.

**Ação esperada:** 🟢 FrontBrain avaliar atualizar os dados do usuário via `GET /api/auth/me` (que já devolve `id_vendedor`) ao carregar a aplicação ou ao abrir esses formulários, em vez de confiar só no `localStorage`. 🔴 TestBrain incluir o cenário no roteiro manual.

---

## Tooling frontend: `npm run lint` quebrado e ausência de runner de testes

**Camada:** Frontend (tooling)
**Origem:** 🟢 FrontBrain (2026-09-24).

**Descrição:**
- `npm run lint` falha, porque `next lint` foi removido no Next 16 e o projeto não tem ESLint configurado.
- O frontend não tem runner de testes. `make test-frontend` termina com `|| true` e, por isso, nunca falha.

Hoje a única verificação automática do frontend é o typecheck.

**Ação esperada:**
- 🟢 FrontBrain configurar o ESLint (flat config, `eslint.config.mjs`) e ajustar o script `lint`.
- 🔴 TestBrain escolher e configurar um runner (ex.: Vitest + Testing Library), remover o `|| true` do `Makefile` e criar os primeiros testes (ex.: `dashboard/page.tsx`, `PedidoModal.tsx`).
