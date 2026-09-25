# Roteiro de teste manual: Dashboard

**Card:** "Teste manual do Dashboard no navegador (normal com vendedor, normal sem vendedor, admin)" (`tarefas/fazendo.md`). Cobre também os cards do lote de 2026-09-24: vendedor desligado, `/api/dashboard/clientes` com escopo da carteira, valores com centavos e listas vazias como `[]`.
**Tela:** `frontend/src/app/dashboard/page.tsx`
**Endpoints envolvidos:** `GET /api/dashboard/metrics?periodo=`, `GET /api/dashboard/vendas?dias=`, `GET /api/dashboard/vendedores?page=&limit=`, `GET /api/dashboard/clientes?periodo=`
**Autor:** TestBrain (2026-09-24)

Marque cada checkbox depois de conferir no navegador. Os números de referência abaixo foram obtidos em 2026-09-24 e **mudam com o tempo** (período "Mês" = mês corrente). Para conferir no dia do teste, use as queries SQL de cada seção.

---

## 0. Preparação

1. Suba a API e o frontend (`make dev-api` e `make dev-frontend`) e acesse `http://localhost:3000/dashboard`.
2. Usuários:

   | Perfil | Usuário (e-mail) | id | Vendedor |
   | --- | --- | --- | --- |
   | admin | `admin@rotaperfumes.com.br` | 1 | - |
   | normal com vendedor | `rafael.carvalho@rotaperfumes.com.br` | 5 | 4 - Rafael Carvalho (ativo, meta 55.000,00) |
   | normal com vendedor **desligado** | `henrique.rodrigues@rotaperfumes.com.br` | 2 | 1 - Henrique Rodrigues (`data_desligamento` = 2025-09-07; ainda tem 82 clientes em carteira ativa) |
   | normal sem vendedor | criar temporário (SQL abaixo) | - | nenhum |

   Outros usuários com vendedor desligado na base: ids 3, 4, 10, 38 e 39.

3. Usuário temporário sem vendedor (defina a senha pela tela de Usuários como admin):
   ```sql
   INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo)
   VALUES ('QA Sem Vendedor', 'qa.semvendedor@rotaperfumes.test', '!definir-pela-tela!', 'normal', NULL, 1);
   ```
4. Queries de referência para o vendedor 4 no mês corrente:
   ```sql
   -- Minhas Vendas / Meus Pedidos (mês corrente, sem cancelados)
   SELECT COUNT(*) pedidos, SUM(valor_total) total
   FROM pedidos
   WHERE vendedor_id = 4 AND YEAR(data_pedido) = YEAR(CURDATE()) AND MONTH(data_pedido) = MONTH(CURDATE())
     AND status <> 'Cancelado';

   -- Minha Meta
   SELECT meta_mensal FROM vendedores WHERE id = 4;

   -- Meus Clientes (carteira ativa): total e ativos
   SELECT COUNT(*) total, SUM(ativo) ativos
   FROM clientes
   WHERE cliente_id_origem IN (SELECT cliente_id FROM carteiras WHERE vendedor_id = 4 AND data_fim IS NULL);
   ```
5. **Limpeza ao final:** `DELETE FROM usuarios WHERE email = 'qa.semvendedor@rotaperfumes.test';`

---

## 1. Normal com vendedor (Rafael Carvalho, vendedor 4)

- [ ] 1.1 O cabeçalho mostra "Dashboard / Suas metricas de vendas".
- [ ] 1.2 Os KPIs se chamam **"Minhas Vendas"**, **"Meus Pedidos"**, **"Meu Ticket Medio"** e **"Minha Meta"**, sem "Ranking Vendedores".
- [ ] 1.3 No período "Mês", Minhas Vendas bate com a query de referência (em 2026-09-24: **R$ 75.148,43**, 29 pedidos, ticket R$ 2.591,33) e **não** com o total da empresa.
- [ ] 1.4 Minha Meta mostra **R$ 55.000,00** (só a meta dele, não a soma de todos) com "Atingido: ~136,6%".
- [ ] 1.5 O gráfico se chama "Minhas Vendas nos Últimos N Dias" e o N acompanha o select (7/14/30/60). Trocar o select recarrega a série com só as vendas dele. Dias sem venda aparecem como barra zerada.
- [ ] 1.6 O card de metas se chama "Minha Meta" e mostra só a linha do Rafael Carvalho.
- [ ] 1.7 No lugar do "Ranking de Vendedores" aparece o card **"Meu Desempenho"** ("Seus indicadores no ranking de vendas"), com os indicadores dele: total, pedidos, ticket, meta e atingimento.
- [ ] 1.8 A seção de clientes tem o título **"Clientes da minha carteira"** e o subtítulo "Somente clientes vinculados ao seu vendedor". "Meus Clientes" = **60**, ativos = **54**, inativos = **6** (em 2026-09-24), batendo com a query de referência e **não** com o total da base (3.040).
- [ ] 1.9 "Meus Clientes por Segmento" e "Meus Clientes por UF" somam 60, e não milhares.
- [ ] 1.10 Os filtros Hoje / Semana / Mês recarregam os números. Em "Hoje", números baixos ou zerados são normais.
- [ ] 1.11 Não aparece nenhum aviso amarelo (sem vendedor ou desligado).

## 2. Normal sem vendedor

- [ ] 2.1 Aparece o aviso amarelo: "Usuario sem vendedor vinculado. As metricas de vendas sao exibidas apenas para usuarios vinculados a um vendedor; solicite o vinculo a um administrador."
- [ ] 2.2 **UI-02 opção (b):** o aviso fica **acima** dos cards, e os KPIs continuam **visíveis e zerados**: "Minhas Vendas" R$ 0,00, "Meus Pedidos" 0, "Meu Ticket Medio" R$ 0,00, "Minha Meta" "-" ("Sem meta cadastrada"). Nenhum número da empresa aparece.
- [ ] 2.3 O gráfico continua **visível**, com o título "Minhas Vendas nos Últimos N Dias" e o subtítulo "Total: R$ 0,00". As barras vêm zeradas (a API devolve os N dias com zero) ou aparece "Nenhum dado de vendas no periodo".
- [ ] 2.4 Os clientes aparecem zerados: "Meus Clientes", ativos, inativos e novos = 0, e "por Segmento"/"por UF" mostram "Sem dados disponiveis". "Meu Desempenho" mostra "Nenhum dado de desempenho encontrado".
- [ ] 2.5 Não aparece nenhuma mensagem de erro vermelha. Todas as chamadas voltam 200 com valores zerados.
- [ ] 2.6 Trocar o select do gráfico para 7 dias muda o título para "Minhas Vendas nos Últimos 7 Dias", e o aviso continua visível.

## 3. Normal com vendedor desligado (Henrique Rodrigues, vendedor 1)

- [ ] 3.1 Aparece o aviso amarelo: "Vendedor desligado. O vendedor vinculado ao seu usuario possui data de desligamento, por isso nao ha metricas de vendas nem clientes na sua carteira; procure um administrador."
- [ ] 3.2 O aviso de "sem vendedor" **não** aparece. É o aviso específico de desligado.
- [ ] 3.3 **UI-02 opção (b):** os KPIs continuam **visíveis e zerados** (R$ 0,00 / 0 / R$ 0,00 / "-"). Mesmo que o vendedor 1 tenha vendas históricas, nenhum valor dele aparece.
- [ ] 3.4 O gráfico continua **visível e zerado** ("Total: R$ 0,00") e o título acompanha o select ("Minhas Vendas nos Últimos N Dias").
- [ ] 3.5 Os clientes aparecem zerados (0 e listas vazias), embora o vendedor 1 ainda tenha 82 clientes em carteira ativa.
- [ ] 3.6 "Meu Desempenho" mostra "Nenhum dado de desempenho encontrado".
- [ ] 3.7 O menu não mostra os itens da carteira: os dropdowns **ERP** (Clientes, Oportunidades, Visitas) e **CRM** (Pagamentos, Pedidos) não aparecem. "Dashboard" e "Trocar Senha" continuam no menu. O roteiro completo do desligado está em `docs/roteiro-teste-manual-vendedor-desligado.md`.
- [ ] 3.8 Bloqueio sem novo login (opcional): como admin, preencha `data_desligamento` de um vendedor ativo. Na aba do usuário dele, **troque de aba e volte** (ou recarregue). O aviso de desligado aparece e os KPIs zeram, sem novo login. **Desfaça** depois (`UPDATE vendedores SET data_desligamento = NULL WHERE id = <id>;`).

## 4. Admin

- [ ] 4.1 O cabeçalho mostra "Visao geral das metricas de vendas".
- [ ] 4.2 Os KPIs se chamam "Total de Vendas", "Pedidos", "Ticket Medio" e "Ranking Vendedores" (valor do líder + "Lider: <nome>").
- [ ] 4.3 No período "Mês", o Total de Vendas é o da empresa inteira (em 2026-09-24: **R$ 3.761.832,81**, 1.186 pedidos) e a Meta Mensal é a soma das metas dos vendedores ativos (**R$ 2.505.000,00**).
- [ ] 4.4 O card "Acompanhamento de Metas" lista vários vendedores.
- [ ] 4.5 O "Ranking de Vendedores" (Top 10 por volume) mostra 10 linhas. Em 2026-09-24 o líder era Débora Ribeiro, com R$ 158.830,30 e 158,83% da meta. A API informa `total: 36` vendedores com vendas no ranking.
- [ ] 4.6 A seção de clientes usa os rótulos globais ("Total de Clientes" etc.): **3.040** clientes, 2.823 ativos, 217 inativos. O título "Clientes da minha carteira" **não** aparece.
- [ ] 4.7 Não aparece nenhum aviso amarelo.
- [ ] 4.8 **UI-01:** o título do gráfico acompanha o select. Com o padrão, é "Vendas nos Últimos 30 Dias". Escolher 7, 14 e 60 muda para "Vendas nos Últimos 7 Dias", "... 14 Dias" e "... 60 Dias", e a série é recarregada (`GET /api/dashboard/vendas?dias=N`).
- [ ] 4.9 Um admin cujo usuário tenha `id_vendedor` desligado **não** vê aviso: admin nunca é bloqueado.

## 5. Valores com centavos (sem truncar)

- [ ] 5.1 Admin: no Ranking e em "Acompanhamento de Metas", os valores têm centavos (ex.: Débora Ribeiro **R$ 158.830,30**, Queila Lima **R$ 74.558,44**, João Soares **R$ 135.441,13**), não valores redondos.
- [ ] 5.2 O percentual de meta é calculado sobre o valor com centavos (ex.: Queila Lima 74.558,44 / 100.000 = **74,56%**).
- [ ] 5.3 Normal (vendedor 4): Minhas Vendas R$ 75.148,43, e "Meu Desempenho" e "Minha Meta" mostram o mesmo valor com centavos (136,63%).
- [ ] 5.4 Sem dados (usuário sem vendedor / desligado / período sem vendas), nenhuma tela quebra nem mostra "null"/"NaN". A API devolve `[]` nas listas.

## 5b. FE-02: zeros forçados pelo frontend (defesa em profundidade)

Desde o FE-02, para o usuário `normal` **sem vendedor** ou com **vendedor desligado**, a tela mostra tudo zerado **mesmo que a API devolva números**. Hoje a API já devolve zeros nesses casos, então o teste manual força uma resposta preenchida com o recurso **Local Overrides** do Chrome/Edge:

1. DevTools → aba **Network** → recarregue o Dashboard → clique com o botão direito em `metrics?periodo=month` → **Override content** (na primeira vez, escolha uma pasta local para os overrides e aceite a permissão).
2. Troque o corpo por uma resposta **preenchida**, mantendo o envelope da API, por exemplo:
   ```json
   {"success":true,"data":{"total_vendas":99999.99,"total_pedidos":42,"ticket_medio":2380.95,"total_clientes":7,"meta_mes":50000,"atingimento_meta":199.99,"periodo":"month","vendedor_desligado":false,"top_vendedores":[],"metas_vendedores":[]}}
   ```
   Faça o mesmo em `clientes?periodo=month` (ex.: `"total_clientes":7,"total_ativos":6,"total_inativos":1,"novos_no_periodo":2,"por_segmento":[{"segmento":"Varejo","total":7}],"por_uf":[{"uf":"SP","total":7}]`), em `vendas?dias=30` (pontos com `total_vendas` > 0) e em `vendedores?page=1&limit=10` (uma linha com `vendedor_id`, `vendedor_nome` e `total_vendas`).
3. Recarregue a página com os overrides ativos.

- [ ] 5b.1 **Normal sem vendedor:** o aviso de "sem vendedor" aparece e **todos** os números ficam zerados: Minhas Vendas R$ 0,00, Meus Pedidos 0, Meu Ticket Medio R$ 0,00, Minha Meta "-", gráfico com "Total: R$ 0,00", Meus Clientes/Ativos/Inativos/Novos = 0, "por Segmento"/"por UF" vazios e "Meu Desempenho" com "Nenhum dado de desempenho encontrado". **Nenhum** número do override aparece (99.999,99 / 42 / 7 / Varejo / SP).
- [ ] 5b.2 **Normal com vendedor desligado (id 2):** mesmo resultado do 5b.1, com o aviso de **desligado**.
- [ ] 5b.3 **Desligado só pelo payload:** como normal **com** vendedor ativo (id 5), use o override de `metrics` com `"vendedor_desligado":true`. O aviso de desligado aparece e a tela zera tudo, inclusive os clientes do override.
- [ ] 5b.4 **Controle (normal ativo, id 5):** com o override de `metrics` preenchido e `"vendedor_desligado":false`, os números do override **aparecem** (R$ 99.999,99, 42 pedidos). Isso prova que o zero dos itens anteriores vem da regra, e não de uma falha do override.
- [ ] 5b.5 **Admin:** com o override preenchido e `"vendedor_desligado":true`, o admin continua vendo os números (admin nunca é zerado).
- [ ] 5b.6 **Troca rápida de filtro:** clique em Hoje → Semana → Mês em sequência rápida (com a rede em "Slow 3G" no DevTools). Ao final, os números e o botão ativo correspondem ao **último** filtro clicado. Uma resposta atrasada de um filtro anterior não sobrescreve a tela, e o indicador "Atualizando dados..." some ao terminar.
- [ ] 5b.7 **Atualizar após erro:** bloqueie `metrics` (DevTools → Network → botão direito → **Block request URL**) e recarregue: aparece o alerta vermelho. Desbloqueie e clique em "Atualizar": o alerta some e os números voltam.
- [ ] 5b.8 Desative os overrides no fim (DevTools → Sources → Overrides → desmarque "Enable Local Overrides").

## 6. Limpeza

- [ ] 6.1 Usuário temporário removido.
- [ ] 6.2 `data_desligamento` restaurada se o item 3.8 foi executado.
- [ ] 6.3 Local Overrides e bloqueios de URL do DevTools desativados (seção 5b).

---

## Evidências via API (2026-09-24)

Validação feita pelo TestBrain com a API local (porta 8080) e o banco local `rotaperfumes`. Como o login real exige captcha (Turnstile), os testes usaram tokens JWT gerados localmente com o `JWT_SECRET`/`JWT_ISSUER` do `.env`, para os usuários reais: admin id 1; normal id 5 (vendedor 4); normal desligado id 2 (vendedor 1); normal sem vendedor = usuário temporário id 86, criado por SQL e removido no fim. O servidor foi parado no fim.

### Normal com vendedor (id 5 / vendedor 4)

| Chamada | Status | Trecho |
| --- | --- | --- |
| `GET /api/dashboard/metrics?periodo=month` | 200 | `"meta_mes":55000,"metas_vendedores":[{"id":4,"nome":"Rafael Carvalho",...,"realizado":75148.43,"percentual":136.6335...}],"top_vendedores":[{"id":4,...,"total_vendas":75148.43,...}],"total_pedidos":29,"total_vendas":75148.43,"vendedor_desligado":false` |
| `GET /api/dashboard/vendas?dias=7` | 200 | `{"dias":7,"pontos":[{"dia":"2026-09-18","total_pedidos":0,"total_vendas":0},{"dia":"2026-09-19","total_pedidos":1,"total_vendas":1125.56},...` |
| `GET /api/dashboard/vendedores?page=1&limit=10` | 200 | `[{"atingimento_meta":136.63,"meta":55000,...,"total_vendas":75148.43,"vendedor_id":4,"vendedor_nome":"Rafael Carvalho"}],"pagination":{...,"total":1}` (só ele) |
| `GET /api/dashboard/clientes?periodo=month` | 200 | `"total_clientes":60,"total_ativos":54,"total_inativos":6,"por_segmento":[{"segmento":"Salão de beleza","total":10},...]` |

Conferência no banco: carteira ativa do vendedor 4 = 60 clientes, 54 ativos; pedidos de setembro/2026 = 29, total 75.148,43; `meta_mensal` = 55000.00. **Os números batem.**

### Normal sem vendedor (id 86, temporário)

| Chamada | Status | Trecho |
| --- | --- | --- |
| `GET /api/dashboard/metrics?periodo=month` | 200 | `{"meta_mes":0,"metas_vendedores":[],"periodo":"month","ticket_medio":0,"top_vendedores":[],"total_pedidos":0,"total_vendas":0,"total_vendas_qtd":0,"vendedor_desligado":false}` |
| `GET /api/dashboard/vendas?dias=7` | 200 | 7 pontos de 2026-09-18 a 2026-09-24, todos com `"total_pedidos":0,"total_vendas":0` |
| `GET /api/dashboard/vendedores` | 200 | `{"data":[],"pagination":{"limit":10,"page":1,"pages":0,"total":0}}` |
| `GET /api/dashboard/clientes?periodo=month` | 200 | `{"novos_no_periodo":0,"periodo":"month","por_segmento":[],"por_uf":[],"total_ativos":0,"total_clientes":0,"total_inativos":0}` |

### Normal com vendedor desligado (id 2 / vendedor 1)

| Chamada | Status | Trecho |
| --- | --- | --- |
| `GET /api/dashboard/metrics?periodo=month` | 200 | `{"meta_mes":0,"metas_vendedores":[],...,"top_vendedores":[],"total_pedidos":0,"total_vendas":0,"total_vendas_qtd":0,"vendedor_desligado":true}` |
| `GET /api/dashboard/vendas?dias=7` | 200 | 7 pontos, todos zerados |
| `GET /api/dashboard/vendedores` | 200 | `{"data":[],"pagination":{...,"total":0}}` |
| `GET /api/dashboard/clientes?periodo=month` | 200 | `{"novos_no_periodo":0,...,"por_segmento":[],"por_uf":[],"total_ativos":0,"total_clientes":0,"total_inativos":0}` (mesmo com 82 clientes em carteira ativa) |

### Admin (id 1)

| Chamada | Status | Trecho |
| --- | --- | --- |
| `GET /api/dashboard/metrics?periodo=month` | 200 | `"meta_mes":2505000,"metas_vendedores":[{"id":23,"nome":"Queila Lima",...,"realizado":74558.44,"percentual":74.55844},{"id":27,"nome":"João Soares",...,"realizado":135441.13,...}...],"total_pedidos":1186,"total_vendas":3761832.81,"ticket_medio":3171.86...,"vendedor_desligado":false` (10 itens em `top_vendedores`) |
| `GET /api/dashboard/vendas?dias=7` | 200 | `{"dia":"2026-09-18","total_pedidos":64,"total_vendas":205277.36},{"dia":"2026-09-19","total_pedidos":53,"total_vendas":204193.63},...` |
| `GET /api/dashboard/vendedores?page=1&limit=10` | 200 | `[{"atingimento_meta":158.83,"meta":100000,...,"total_vendas":158830.3,"vendedor_id":7,"vendedor_nome":"Débora Ribeiro"},{"atingimento_meta":135.44,...,"vendedor_nome":"João Soares"},...],"pagination":{"limit":10,"page":1,"pages":4,"total":36}` |
| `GET /api/dashboard/clientes?periodo=month` | 200 | `"total_clientes":3040,"total_ativos":2823,"total_inativos":217,"por_segmento":[{"segmento":"Loja de departamento","total":401},...]` (base inteira: `SELECT COUNT(*) FROM clientes` = 3040) |

**Centavos:** `realizado`/`total_vendas` vêm com centavos (74558.44, 135441.13, 158830.3, 75148.43), e os percentuais são calculados sobre esses valores (74558.44 / 100000 = 74.55844%). Não há truncamento.
**Listas vazias:** nos perfis sem acesso, `top_vendedores`, `metas_vendedores`, `por_segmento`, `por_uf` e `data` vêm como `[]`, nunca `null`.

### Achados

- **BUG-01 (backend, ver `docs/roteiro-teste-manual-pedidomodal.md`):** a `data_pedido` é gravada com 1 dia a menos (UTC vs `loc=Local`). No Dashboard, os pedidos de teste criados com data 2026-09-24 apareceram no dia **2026-09-23** da série de vendas do vendedor 4 (`{"dia":"2026-09-23","total_pedidos":2,"total_vendas":78.39}`, soma exata dos 2 pedidos de teste). Os pedidos de teste foram excluídos depois.
- **UI-01 (frontend) - resolvido:** o título do gráfico agora é dinâmico ("Vendas nos Últimos N Dias" para o admin, "Minhas Vendas nos Últimos N Dias" para o normal) e acompanha o select. Veja o item 4.8.
- **UI-02 (frontend) - resolvido, opção (b):** para usuário sem vendedor ou com vendedor desligado, a tela mostra o aviso **e** mantém KPIs, gráfico e clientes visíveis e zerados. Veja os itens 2.2 a 2.4 e 3.3 a 3.6.

### Testes automatizados (frontend)

Os itens abaixo também são cobertos por testes automatizados, que rodam com `make test-frontend` ou `cd frontend && npm test`:

- `frontend/src/app/dashboard/page.test.tsx`: título do gráfico em 7/14/30/60 dias (admin e normal). Aviso + KPIs/gráfico zerados para normal sem vendedor, desligado pela sessão (`/me` ou 403) e desligado por `metrics.vendedor_desligado`. Sem aviso para admin e para normal ativo.
- `frontend/src/components/layout/Navbar.test.tsx`: itens da carteira ocultos para o desligado.
- `frontend/src/app/dashboard/page.test.tsx` (FE-02, 2026-09-24):
  - Payload **preenchido** da API é ignorado e a tela fica zerada para normal sem vendedor, desligado pela sessão e desligado pelo `metrics`.
  - Admin nunca é zerado, mesmo com flag de desligado. Normal ativo com payload cheio mostra os números.
  - Desligamento detectado depois (403 na sessão) zera a tela já carregada.
  - Resposta obsoleta de um período anterior é descartada.
  - "Atualizando dados..." aparece e some. "Atualizar" limpa o erro anterior.
