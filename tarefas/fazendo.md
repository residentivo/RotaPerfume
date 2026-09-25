# Fazendo

> **Lote 1 (2026-09-24):** o 🤍 MegaBrain executou os 10 cards que vieram de `afazer.md`. 8 foram concluídos e movidos para `feito.md`. Os cards BUG-02 e "Teste manual do Dashboard" aguardam a validação do usuário no navegador. O card "Teste manual/e2e do `PedidoModal`" foi validado pelo usuário no navegador em 2026-09-25 e movido para `feito.md`.
>
> **UI-04 (2026-09-25):** relato do usuário durante a validação do `PedidoModal` (accordion de itens do pedido abre no fim da tabela). Correção aplicada pelo 🟢 FrontBrain; aguardando a validação do usuário no navegador.
>
> **Lote 2 (2026-09-24):** o 🤍 MegaBrain executou os 6 cards que estavam em `afazer.md`. **BUG-01** e **Tooling do frontend** foram concluídos e movidos para `feito.md`. Os 4 cards abaixo (UI-01, UI-02, Segurança do vendedor desligado e UX do `id_vendedor`) foram implementados e cobertos por testes automatizados e aguardam a validação do usuário no navegador, pelos roteiros indicados em cada card. As decisões do usuário de 2026-09-24 sobre UI-02 e sobre o vendedor desligado estão registradas nos respectivos cards. Os follow-ups do lote (SEC-01, BUG-04, RISCO-01, FE-01, FE-02 e FE-03) viraram o Lote 3.
>
> **BUG-05 (2026-09-24):** relato do usuário (inativar vendedor deve inativar o usuário vinculado). Em execução pelo 🟡 BackBrain.
>
> **Lote 3 de 2026-09-24:** a pedido do usuário, o 🤍 MegaBrain executou os 6 cards que estavam em `afazer.md`. **BUG-04**, **RISCO-01**, **FE-01** e **FE-02** foram concluídos e movidos para `feito.md`. **SEC-01** e **FE-03** foram validados pelo usuário no navegador em 2026-09-25 (roteiros `docs/roteiro-teste-manual-clientes.md` e `docs/roteiro-smoke-pos-fe03.md`) e também foram movidos para `feito.md`. O Lote 3 está concluído (6 de 6). Os follow-ups do lote (SEC-02, SEC-03, NEG-01, BUG-06, FE-04 e FE-05) estão em `afazer.md`, aguardando priorização.

---

## BUG-02: botão "Alterar Senha" nunca habilita em `/trocar-senha`

**Início:** 2026-09-24
**Passo atual:** Correção aplicada pelo 🟢 FrontBrain (typecheck OK); aguardando o usuário validar no navegador.

**Causa:** o usuário chega em `/trocar-senha` por navegação client-side (login → `router.replace` ou link da Navbar), com o script do Turnstile já carregado na tela de login. O `next/script` deduplica pelo `src` e não dispara `onLoad` de novo; o widget não era renderizado, nenhum token chegava e o botão (`disabled={isTurnstileEnabled && !captchaToken}`) ficava travado.

**Correção:** `frontend/src/components/ui/Turnstile.tsx` troca `onLoad` por `onReady` e inicia `scriptLoaded` como true quando `window.turnstile` já existe. O captcha continua obrigatório.

**BUG-03 (achado na validação, 2026-09-24):** com a senha atual errada, a tela mostrava "Nao foi possivel validar o captcha". A API respondia `401 senha atual incorreta`; o `fetchWithAuth` tratava como token expirado, fazia refresh e reenviava o mesmo `captchaToken` (uso único), que o Cloudflare recusava. 🟡 BackBrain mudou para `400` em `auth_handler.go` (`ResetPassword`), ajustou os testes e criou `TestResetPassword_SenhaAtualIncorreta_Retorna400` (`go vet`/`go test` OK). Postman atualizado.

**Ação esperada:** logar com usuário que precisa trocar a senha → o widget do captcha aparece em `/trocar-senha` → após o desafio, o botão habilita e a troca funciona. Repetir entrando pelo link da Navbar.

---

## UI-04: accordion de itens do pedido abre no fim da tabela

**Início:** 2026-09-25
**Passo atual:** Correção aplicada pelo 🟢 FrontBrain em 2026-09-25; aguardando o usuário validar no navegador.

**Andamento (2026-09-25):**
- Novo prop opcional `renderExpanded` em `frontend/src/components/ui/Table.tsx` (Fragment por linha e `<tr>` extra com `colSpan`).
- Novo componente `frontend/src/components/admin/PedidoItensDetalhe.tsx`.
- `frontend/src/app/admin/pedidos/page.tsx`: removido o bloco do fim da tabela e criado o helper `fecharItens()`, que também limpa o erro antigo.
- Novo teste em `frontend/src/app/crudPaginas.test.tsx`.
- Resultados: tsc OK; vitest com 19/19 arquivos (635 passaram, 18 falhas esperadas, 2 pulados); eslint OK.
- Não existe layout mobile separado. Em telas estreitas, o detalhe fica dentro do scroll horizontal da tabela.

**Camada:** Frontend
**Origem:** relato do usuário durante a validação do `PedidoModal` (2026-09-25).

**Descrição:** Em `frontend/src/app/admin/pedidos/page.tsx`, ao clicar em "Itens" numa linha, o detalhe dos itens aparece abaixo da tabela inteira, num bloco separado (por volta da linha 600). O detalhe deveria abrir como linha expandida logo após a linha clicada.

**Ação esperada:** o usuário clica em "Itens" em qualquer linha, inclusive no meio da página e no layout mobile (cards), e o detalhe aparece logo abaixo dessa linha.

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

---

## UI-01: título do gráfico do Dashboard (admin) fixo em "Vendas nos Ultimos 30 Dias"

**Início:** 2026-09-24
**Passo atual:** Implementado e coberto por testes automatizados; aguardando o usuário validar no navegador pelo roteiro `docs/roteiro-teste-manual-dashboard.md` (item 4.8).

**Andamento (2026-09-24):** 🟢 FrontBrain montou o título a partir dos dias selecionados. 🔴 TestBrain cobriu com teste automatizado (`frontend/src/app/dashboard/page.test.tsx`, 7/14/30/60 dias) e atualizou o item 4.8 do roteiro.

**Camada:** Frontend (menor)
**Origem:** lote de 2026-09-24, durante a validação do card "Teste manual do Dashboard" (em `fazendo.md`).

**Descrição:** Em `frontend/src/app/dashboard/page.tsx`, o título do gráfico do admin continua "Vendas nos Ultimos 30 Dias" mesmo quando o select de dias muda para outro valor.

**Ação esperada:** 🟢 FrontBrain montar o título a partir do número de dias selecionado (ex.: "Vendas nos Últimos 7 Dias").

---

## UI-02: layout do Dashboard para normal sem vendedor / com vendedor desligado (DECIDIDO: opção b)

**Início:** 2026-09-24
**Passo atual:** Implementado e coberto por testes automatizados; aguardando o usuário validar no navegador pelo roteiro `docs/roteiro-teste-manual-dashboard.md` (itens 2.2-2.6 e 3.3-3.8).

**Andamento (2026-09-24):** 🟢 FrontBrain implementou a opção (b) em `frontend/src/app/dashboard/page.tsx`. 🔴 TestBrain cobriu com testes automatizados (`frontend/src/app/dashboard/page.test.tsx`) e atualizou o roteiro. Ver também o follow-up **FE-02** em `afazer.md`: o Dashboard depende da API para zerar os dados.

**Camada:** Frontend
**Origem:** lote de 2026-09-24, durante a validação do card "Teste manual do Dashboard" (em `fazendo.md`).

**Descrição:** Para o usuário `normal` sem vendedor vinculado e para o `normal` com vendedor desligado, `frontend/src/app/dashboard/page.tsx` mostra **só o aviso**. Não aparecem os KPIs zerados nem o gráfico com os dias zerados. O card original de teste manual esperava "aviso, números zerados e gráfico com dias zerados". A API devolve os dados zerados corretamente nos dois casos.

**DECISÃO DO USUÁRIO (2026-09-24):** opção **(b)** — aviso + KPIs zerados + gráfico com os dias zerados.
- 🟢 FrontBrain ajusta `frontend/src/app/dashboard/page.tsx`.
- 🔴 TestBrain atualiza `docs/roteiro-teste-manual-dashboard.md`.

**Ação esperada (original):**
- 🔵 SubBrain / 🤍 MegaBrain confirmar com o usuário o layout desejado: (a) só o aviso, como hoje, ou (b) aviso + KPIs zerados + gráfico zerado. *(Concluído: opção b.)*
- Se for (b), 🟢 FrontBrain ajustar `page.tsx` e 🔴 TestBrain atualizar `docs/roteiro-teste-manual-dashboard.md`.
- Se for (a), ajustar só a expectativa do roteiro e do card de teste manual. *(Não se aplica.)*

---

## Segurança: vendedor desligado ainda pode criar/editar pedidos, pagamentos, oportunidades e visitas (DECIDIDO: bloquear)

**Início:** 2026-09-24
**Passo atual:** Implementado e coberto por testes automatizados; aguardando o usuário validar no navegador pelo roteiro `docs/roteiro-teste-manual-vendedor-desligado.md`.

**Camada:** Backend (com impacto em Frontend, Testes e Postman)
**Origem:** 🟣 SecBrain, lote de 2026-09-24 (relacionado ao card "Dashboard de vendedor desligado", em `feito.md`).

**Descrição:** A decisão do usuário de 2026-09-24 ("bloquear o acesso" do vendedor desligado) cobriu **só o Dashboard**. `resolverVendedorScope` (`apis/rotaperfumes-api/handlers/scope.go`) ignora `vendedores.data_desligamento`. Com isso, um usuário `normal` vinculado a um vendedor desligado continua podendo listar, criar e editar pedidos, pagamentos, oportunidades e visitas da própria carteira.

**DECISÃO DO USUÁRIO (2026-09-24):**
- Bloquear o usuário `normal` vinculado a vendedor desligado em **todas as rotas da carteira**, tanto de **leitura** quanto de **escrita**.
- **Login** e **Dashboard** continuam acessíveis. O Dashboard mostra o aviso de vendedor desligado.
- Responsáveis:
  - 🟣 SecBrain define o contrato (ex.: `403` "vendedor desligado").
  - 🟡 BackBrain aplica.
  - 🟢 FrontBrain exibe o aviso nas telas afetadas.
  - 🔴 TestBrain cobre com testes.
  - 🔵 SubBrain atualiza o Postman.

**Andamento (2026-09-24):**
- **🟣 SecBrain:** contrato `403` `{"success":false,"error":"acesso bloqueado: vendedor desligado"}` em todas as rotas da carteira (leitura e escrita), fail-closed (`500` em falha de banco). Login, `/api/auth/me` e Dashboard continuam acessíveis.
- **🟡 BackBrain:** `apis/rotaperfumes-api/handlers/scope.go`:
  - `resolverVendedorScope(r, db)` passou a checar `data_desligamento` a cada requisição (sem cache).
  - `resolverVendedorScopeBase` (sem a checagem de desligamento) fica restrito ao Dashboard, que continua `200` zerado com `vendedor_desligado: true`.
  - Erro sentinela `errVendedorDesligado` e `responderErroEscopo`, que responde `403` para desligado e `500` para qualquer outro erro.
  - Aplicado nos handlers de clientes, pedidos, pagamentos, oportunidades, visitas e em `GET /api/vendedores/{id}/clientes`.
  - `GET /api/auth/me` passou a devolver `vendedor_desligado` (bool).
- **🟢 FrontBrain:**
  - `ApiError`.
  - `frontend/src/lib/vendedorDesligado.ts`.
  - `frontend/src/lib/session.ts` (revalidação da sessão via `/me`).
  - `frontend/src/components/layout/CarteiraGuard.tsx` (aviso nas telas da carteira).
  - Navbar oculta os itens da carteira para o vendedor desligado.
- **🔴 TestBrain:**
  - Backend: `apis/rotaperfumes-api/handlers/escopo_desligado_cobertura_test.go` e `scope_internal_test.go`. Cobertura de `handlers`: 87,3%.
  - Frontend: testes Vitest da camada de sessão e layout (ex.: `vendedorDesligado.test.ts`, `session.test.ts`, `CarteiraGuard.test.tsx`, `Navbar.test.tsx`).
  - Roteiro manual `docs/roteiro-teste-manual-vendedor-desligado.md`.
- **🔵 SubBrain:** Postman atualizado (`postman/collection.json` e `postman/README.md`). Cada pasta da carteira ganhou uma nota e exemplos `403`, e o `/me` ganhou `vendedor_desligado`.
- **Follow-ups:** **FE-01** (corrida em `refreshSessionUser`) em `afazer.md`. O card **SEC-01** (IDOR em clientes) foi aberto durante a definição do contrato.
- **Observação (2026-09-24, BUG-05):** por decisão do 🤍 MegaBrain, inativar o vendedor passa a inativar também os usuários vinculados (`ativo = 0`). Com isso, o login do usuário de vendedor desligado **deixa de ficar acessível**, o que altera a premissa deste card ("Login e Dashboard continuam acessíveis"). Ver card **BUG-05** abaixo.

**Ação esperada (original):**
- 🤍 MegaBrain / 🔵 SubBrain levar a decisão ao usuário: bloquear também as escritas (e talvez as leituras) do vendedor desligado, ou manter como está. *(Concluído: bloquear leitura e escrita.)*
- Se bloquear: 🟣 SecBrain definir os status (ex.: `403` "vendedor desligado"), 🟡 BackBrain aplicar em `resolverVendedorScope`, 🟢 FrontBrain exibir o aviso nas telas afetadas, 🔴 TestBrain cobrir e 🔵 SubBrain atualizar o Postman.

---

## UX: `id_vendedor` do localStorage desatualizado após o admin mudar o vínculo do usuário

**Início:** 2026-09-24
**Passo atual:** Implementado e coberto por testes automatizados; aguardando o usuário validar no navegador pelos roteiros `docs/roteiro-teste-manual-vendedor-desligado.md` (cenário de troca de vínculo, seção 6) e `docs/roteiro-teste-manual-pedidomodal.md` (seção 5b).

**Andamento (2026-09-24):** 🟢 FrontBrain passou a revalidar os dados do usuário via `GET /api/auth/me` (`frontend/src/lib/session.ts`), em vez de confiar só no `localStorage`. 🔴 TestBrain cobriu com testes (`session.test.ts`, `ProtectedRoute.test.tsx` e `PedidoModal.test.tsx`) e escreveu o roteiro manual.

**Camada:** Frontend
**Origem:** 🟣 SecBrain (2026-09-24).

**Descrição:** O frontend guarda `id_vendedor` no `localStorage` no login. Se o admin mudar o vínculo usuário → vendedor, as telas `PedidoModal`, Oportunidades e Visitas continuam travando o vendedor antigo até um novo login. Não há vazamento de dados: o backend é a fonte da verdade e aplica o vendedor correto. É só um problema de experiência.

**Ação esperada:** 🟢 FrontBrain avaliar atualizar os dados do usuário via `GET /api/auth/me` (que já devolve `id_vendedor`) ao carregar a aplicação ou ao abrir esses formulários, em vez de confiar só no `localStorage`. 🔴 TestBrain incluir o cenário no roteiro manual.

---

## BUG-05: inativar vendedor não inativa o usuário vinculado — prioridade ALTA

**Início:** 2026-09-24
**Passo atual:** 🟡 BackBrain implementando.

**Camada:** Backend
**Origem:** relato do usuário, 2026-09-24.

**Descrição:** Ao inativar um vendedor (`DeleteVendedor` em `apis/rotaperfumes-api/services/vendedor_service.go`), só `vendedores.data_desligamento` é gravado. O usuário vinculado continua com `ativo = 1` e consegue fazer login. A regra pedida pelo usuário é inativar também o usuário vinculado.

**Decisão do 🤍 MegaBrain (2026-09-24):**
- Inativar os usuários vinculados na mesma operação (atômica).
- Reativar o vendedor **não** reativa os usuários automaticamente. A reativação é manual, na tela de usuários.
- Isso altera o comportamento do card "Segurança: vendedor desligado", em que o login do usuário continuava acessível. A observação foi registrada no "Andamento" daquele card.

**Ação esperada:**
- 🟡 BackBrain implementa e cobre com testes.
- 🔵 SubBrain atualiza a documentação e o Postman se a resposta da API mudar.
