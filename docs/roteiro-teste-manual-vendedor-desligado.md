# Roteiro de teste manual: vendedor desligado e sessão revalidada (/me)

**Cards:** bloqueio de vendedor desligado na carteira e revalidação da sessão sem novo login (`tarefas/fazendo.md`)
**Telas/componentes:** `frontend/src/components/layout/Navbar.tsx`, `CarteiraGuard.tsx`, `ProtectedRoute.tsx`, `frontend/src/lib/session.ts`, `frontend/src/lib/apiClient.ts`, `frontend/src/app/dashboard/page.tsx`, `PedidoModal.tsx`, `OportunidadeModal.tsx`/`oportunidades/page.tsx`, `VisitaModal.tsx`/`visitas/page.tsx`
**Contrato (SecBrain):** para um usuário `normal` **ativo** vinculado a um vendedor com `data_desligamento`, as rotas da carteira respondem `403 {"success":false,"error":"acesso bloqueado: vendedor desligado"}`. Login, `/api/auth/me`, trocar senha e Dashboard continuam acessíveis.
**Mudança do BUG-05 (2026-09-25):** desligar um vendedor pela API (`DELETE /api/vendedores/{id}`, tela de Vendedores) também grava `ativo = 0` nos usuários vinculados, na mesma transação. Esses usuários deixam de fazer login (`401 "usuário inativo"`). Reativar o vendedor **não** reativa os usuários: a reativação é manual, na tela de Usuários. O contrato acima continua valendo para o usuário que segue ativo com vendedor desligado, como o `thiago.silva` do seed (o vendedor 3 já vem desligado do seed e não passou pelo `DELETE`).
**Autor:** TestBrain (2026-09-24). Atualizado pelo SubBrain em 2026-09-25 (DOC-01): o usuário desligado de teste passou de `henrique.rodrigues` (id 2) para `thiago.silva` (id 4).

Marque cada checkbox depois de conferir no navegador, com o DevTools aberto na aba **Network**. Se algo divergir, anote o caso e encaminhe ao SubBrain.

---

## 0. Preparação

1. Suba a API e o frontend (`make dev-api` e `make dev-frontend`).
2. Usuários (confira as senhas que você usa hoje):

   | Perfil | Usuário (e-mail) | id | Vendedor |
   | --- | --- | --- | --- |
   | admin | `admin@rotaperfumes.com.br` | 1 | - |
   | normal com vendedor **desligado** | `thiago.silva@rotaperfumes.com.br` | 4 | 3 - Thiago Silva (`data_desligamento` = 2025-07-26) |
   | normal com vendedor ativo | `rafael.carvalho@rotaperfumes.com.br` | 5 | 4 - Rafael Carvalho |

   O `henrique.rodrigues@...` (id 2, vendedor 1), usado antes como desligado, hoje está com `ativo = 0` (efeito do BUG-05) e não consegue fazer login. Não use esse usuário neste roteiro.

   **Senhas:** o seed (`sql/03_seed_vendedores.sql`) grava um hash placeholder. O `make db-seed` troca esse hash pela senha do `SEED_USER_PASSWORD` do `.env` ou, se ela não existir, por uma senha aleatória impressa no terminal. Use a senha que você tem hoje para cada usuário. Os usuários do seed vêm com `deve_trocar_senha = 1`: se ainda estiver assim, o primeiro login leva para `/trocar-senha` antes de liberar as outras telas.

3. Confira o usuário desligado antes de começar:

   ```sql
   SELECT u.id, u.email, u.ativo, u.id_vendedor, v.data_desligamento
     FROM usuarios u JOIN vendedores v ON v.id = u.id_vendedor
    WHERE u.email = 'thiago.silva@rotaperfumes.com.br';
   ```

   Esperado: `id` = 4, `ativo` = 1, `id_vendedor` = 3, `data_desligamento` = 2025-07-26. Se `ativo` = 0 (por exemplo, porque alguém desligou o vendedor 3 pela tela de Vendedores), o login responde `401 "usuário inativo"` e o roteiro não pode ser executado com esse usuário. Nesse caso, reative o usuário 4 na tela de Usuários (o vendedor 3 continua desligado) e anote isso para desfazer no fim.

4. Para os cenários de troca de vínculo (seção 5), use **dois navegadores** (ou uma janela anônima): um com o admin e outro com o usuário normal.
5. Anote o vínculo original do usuário 5 para desfazer no fim: `SELECT id_vendedor, ativo FROM usuarios WHERE id = 5;` (esperado: `id_vendedor` = 4, `ativo` = 1).

---

## 1. Menu (Navbar) do desligado

Faça login como `thiago.silva@...` (id 4). Depois do login, o usuário `normal` cai em `/pagamentos`, que para o desligado já mostra o aviso da seção 2 (ou em `/trocar-senha`, se o usuário ainda tiver `deve_trocar_senha = 1`).

- [ ] 1.1 Os dropdowns **ERP** (Clientes, Oportunidades, Visitas) e **CRM** (Pagamentos, Pedidos) **não** aparecem.
- [ ] 1.2 "Dashboard" e "Trocar Senha" continuam no menu, junto com o nome do usuário, o papel e o botão "Sair".
- [ ] 1.3 O dropdown "Administração" não aparece (usuário normal).
- [ ] 1.4 Comparação: faça login como `rafael.carvalho@...` (ativo). ERP e CRM aparecem com os 5 itens.

## 2. URL direta da carteira

Ainda como o desligado, digite cada URL na barra de endereço:

| # | URL | Título esperado |
| --- | --- | --- |
| 2.1 | `/admin/clientes` | Clientes |
| 2.2 | `/admin/pedidos` | Pedidos |
| 2.3 | `/pagamentos` | Pagamentos |
| 2.4 | `/admin/oportunidades` | Oportunidades |
| 2.5 | `/admin/visitas` | Visitas |

Em cada uma:

- [ ] A tela mostra **só** o título, o aviso amarelo "Seu vendedor foi desligado; o acesso aos dados da carteira está bloqueado." e o link **"Ir para o Dashboard"**. Não aparece tabela, filtro nem botão "Novo".
- [ ] O link "Ir para o Dashboard" leva para `/dashboard`.
- [ ] A tela **não** redireciona para `/login`, não pede novo login e não fica em loop de requisições. Na aba Network, `/api/auth/me` não se repete sem parar.
- [ ] Nenhuma chamada para `/api/auth/refresh` ou `/api/auth/logout` é disparada.

## 3. API responde 403 (bloqueio real no backend)

O menu e o guard são só UX: o bloqueio real é do backend. Com o desligado logado, rode no Console do DevTools (os cookies HttpOnly vão sozinhos):

```js
for (const p of ["/api/clientes", "/api/pedidos", "/api/pagamentos", "/api/oportunidades", "/api/visitas", "/api/vendedores/3/clientes"]) {
  const r = await fetch("http://localhost:8080" + p, { credentials: "include" });
  console.log(p, r.status, await r.text());
}
```

- [ ] 3.1 Todas respondem **403** com `{"success":false,"error":"acesso bloqueado: vendedor desligado"}`. A mensagem tem que ser exatamente essa, porque o frontend compara por igualdade.
- [ ] 3.2 `GET /api/auth/me` responde **200** e traz `"vendedor_desligado": true`.
- [ ] 3.3 O endpoint de Dashboard (`GET /api/dashboard/metrics?periodo=month`) responde **200** com os valores zerados e `"vendedor_desligado": true`.
- [ ] 3.4 No **Application > Local Storage**, a chave `auth_user` **não** contém `vendedor_desligado` (o flag só existe em memória).

## 4. Dashboard do desligado e rotas que continuam acessíveis

- [ ] 4.1 `/dashboard` abre normalmente e mostra o aviso "Vendedor desligado. O vendedor vinculado ao seu usuario possui data de desligamento...".
- [ ] 4.2 Os KPIs (Minhas Vendas R$ 0,00, Meus Pedidos 0, Meu Ticket Medio R$ 0,00, Minha Meta "-") e o gráfico ("Minhas Vendas nos Últimos N Dias", "Total: R$ 0,00") continuam **visíveis e zerados** (UI-02 opção b). Os clientes também aparecem zerados.
- [ ] 4.3 `/trocar-senha` abre e permite trocar a senha (`POST /api/auth/reset-password` responde 200). A troca apaga os cookies e exige novo login: entre com a nova senha e **restaure a senha original**.
- [ ] 4.4 "Sair" funciona e leva para `/login`. O login do `thiago.silva` continua permitido, porque o usuário está com `ativo = 1`. Um vendedor desligado **pela tela de Vendedores** depois do BUG-05 tem os usuários inativados, e esses usuários recebem `401 "usuário inativo"` no login (veja a nota da seção 5).

## 5. Desligamento sem novo login (bloqueio reativo)

1. Navegador A: faça login com o normal **ativo** (`rafael.carvalho@...`) e abra `/admin/clientes`. A lista aparece.
2. Preencha a data de desligamento do vendedor 4 **só por SQL**: `UPDATE vendedores SET data_desligamento = CURDATE() WHERE id = 4;`.

> **Não use a tela de Vendedores neste passo.** Depois do BUG-05, desligar o vendedor pela tela (`DELETE /api/vendedores/4`) também grava `ativo = 0` no usuário 5, e reativar o vendedor não reativa o usuário. Pelo código, o que acontece nesse caso é o seguinte (não foi validado no navegador): o access token que já foi emitido continua valendo até expirar, porque o middleware de autenticação não consulta `ativo`. Enquanto isso, o comportamento é o dos itens 5.1 a 5.3. Quando o token expira, o `POST /api/auth/refresh` responde `401 "usuário inativo"`, o front faz logout e vai para `/login`, e um novo login responde `401 "usuário inativo"`. Se isso acontecer por engano, reative o usuário 5 na tela de Usuários (item 8.2).

- [ ] 5.1 Navegador A: troque de aba e volte (evento de foco). O menu esconde ERP/CRM e a tela de Clientes passa a mostrar o aviso + o link para o Dashboard, **sem** novo login.
- [ ] 5.2 Navegador A, sem trocar de aba: clicar em "Buscar" ou mudar de página na lista dispara uma requisição que volta 403. A tela troca para o aviso na hora, sem ir para `/login` e sem chamar `/api/auth/refresh`.
- [ ] 5.3 Navegador A: o Dashboard passa a mostrar o aviso de desligado com os valores zerados.
- [ ] 5.4 **Desfaça:** `UPDATE vendedores SET data_desligamento = NULL WHERE id = 4;`. Volte o foco para a aba do Navegador A: o menu e as telas da carteira voltam ao normal, sem novo login.

## 6. Vínculo usuário -> vendedor alterado pelo admin com o usuário logado (UX)

1. Navegador A: faça login com `rafael.carvalho@...` (vendedor 4) e deixe a tela `/admin/pedidos` aberta.
2. Navegador B (admin): em **Usuários**, edite o usuário 5 e troque o vendedor vinculado para outro vendedor **ativo** (ex.: 7 - Débora Ribeiro). Salve.
3. Navegador A: vá para outra aba e **volte** (ou clique na janela). Não faça novo login.

- [ ] 6.1 **Pedidos > Novo Pedido (`PedidoModal`):** o select "Vendedor" (travado/desabilitado) mostra o **novo** vendedor (#7). A lista de clientes é a carteira do vendedor 7 (`GET /api/vendedores/7/clientes` na aba Network), e não a do 4.
- [ ] 6.2 Com o modal **já aberto** antes da troca: ao voltar o foco para a aba, o select "Vendedor" passa para o novo vendedor e os clientes são recarregados.
- [ ] 6.3 Criar um pedido: o `POST /api/pedidos` sai com `"vendedor_id": 7`. **Exclua** o pedido de teste depois.
- [ ] 6.4 **Oportunidades** (`/admin/oportunidades`): o filtro de vendedor travado e o vendedor do modal de nova oportunidade mostram o novo vendedor (7).
- [ ] 6.5 **Visitas** (`/admin/visitas`): o filtro de vendedor travado e o vendedor do modal de nova visita mostram o novo vendedor (7).
- [ ] 6.6 O `auth_user` do Local Storage passa a ter `"id_vendedor": 7` (o cache é atualizado pelo `/me`), mas a tela não depende dele: ela lê a sessão em memória.
- [ ] 6.7 Troque o vínculo para **nenhum vendedor**. Ao voltar o foco, o PedidoModal mostra "Seu usuario nao esta vinculado a um vendedor..." com o botão "Criar pedido" desabilitado, e o Dashboard mostra o aviso de sem vendedor com os KPIs zerados.
- [ ] 6.8 Troque o vínculo para um vendedor **desligado** (ex.: 3). Ao voltar o foco, o menu esconde ERP/CRM e as telas da carteira mostram o aviso da seção 2. Trocar o vínculo na tela de Usuários não mexe no `ativo` do usuário 5 (só o desligamento do vendedor pela API inativa usuários).
- [ ] 6.9 **Desfaça:** volte o vínculo do usuário 5 para o vendedor 4 (valor anotado no passo 0.5).

## 7. Admin nunca é bloqueado

- [ ] 7.1 Como admin, todas as telas da carteira e os itens ERP/CRM/Administração aparecem normalmente.
- [ ] 7.2 Se o usuário admin tiver `id_vendedor` apontando para um vendedor desligado (opcional, via SQL), o Dashboard **não** mostra aviso e a carteira não é bloqueada. Desfaça depois.

## 8. Limpeza

- [ ] 8.1 `data_desligamento` do vendedor 4 restaurada para `NULL`.
- [ ] 8.2 Usuário 5 com vínculo restaurado para o vendedor 4 e `ativo = 1` (`SELECT id_vendedor, ativo FROM usuarios WHERE id = 5;`).
- [ ] 8.3 Pedidos de teste excluídos.
- [ ] 8.4 Senha do `thiago.silva` (id 4) restaurada (item 4.3).
- [ ] 8.5 Se o usuário 4 foi reativado no passo 0.3, volte o `ativo` dele ao valor anotado.

---

## Testes automatizados (frontend)

Estes cenários também são cobertos por testes Vitest + Testing Library, que rodam com `make test-frontend` ou `cd frontend && npm test`:

| Arquivo | O que cobre |
| --- | --- |
| `frontend/src/lib/vendedorDesligado.test.ts` | `isVendedorDesligadoError`: só 403 + mensagem exata é `true`. Maiúsculas, prefixo, sufixo, outros status e `Error` comum são `false`. Também cobre subscribe/notify. |
| `frontend/src/lib/apiClient.test.ts` | O 403 de desligado notifica a sessão, sem `/api/auth/refresh`/logout e sem limpar o `auth_user`. O 401 continua disparando o refresh (com retry, lock compartilhado e logout se o refresh falhar). |
| `frontend/src/lib/auth.test.ts` | `saveUser` grava só a whitelist de `User`, sem `vendedor_desligado`. |
| `frontend/src/lib/session.test.ts` | A sessão via `/me` e o flag do 403 (que não é limpo pela rebusca do próprio 403). Admin nunca é bloqueado. O vínculo alterado aparece sem novo login. |
| `frontend/src/components/layout/CarteiraGuard.test.tsx` | Para o desligado, mostra aviso + link para o Dashboard e não mostra o conteúdo. |
| `frontend/src/components/layout/Navbar.test.tsx` | Esconde ERP/CRM para o desligado e mantém Dashboard e Trocar Senha. |
| `frontend/src/components/layout/ProtectedRoute.test.tsx` | Revalida `/me` no mount, no `focus` e no `visibilitychange`. Uma falha em segundo plano não derruba a sessão. |
| `frontend/src/components/admin/PedidoModal.test.tsx` | O vendedor travado vem de `/me`, não do localStorage desatualizado, e é atualizado com o modal aberto. O submit envia o `vendedor_id` da sessão. |
| `frontend/src/app/dashboard/page.test.tsx` | Aviso + KPIs/gráfico zerados (sem vendedor e desligado) e título dinâmico. |
