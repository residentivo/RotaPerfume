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
- [ ] 1.5 O gráfico se chama "Minhas Vendas nos Ultimos N Dias". Trocar 7/14/30/60 dias recarrega a série com só as vendas dele. Dias sem venda aparecem como barra zerada.
- [ ] 1.6 O card de metas se chama "Minha Meta" e mostra só a linha do Rafael Carvalho.
- [ ] 1.7 No lugar do "Ranking de Vendedores" aparece o card **"Meu Desempenho"** ("Seus indicadores no ranking de vendas"), com os indicadores dele: total, pedidos, ticket, meta e atingimento.
- [ ] 1.8 A seção de clientes tem o título **"Clientes da minha carteira"** e o subtítulo "Somente clientes vinculados ao seu vendedor". "Meus Clientes" = **60**, ativos = **54**, inativos = **6** (em 2026-09-24), batendo com a query de referência e **não** com o total da base (3.040).
- [ ] 1.9 "Meus Clientes por Segmento" e "Meus Clientes por UF" somam 60, e não milhares.
- [ ] 1.10 Os filtros Hoje / Semana / Mês recarregam os números. Em "Hoje", números baixos ou zerados são normais.
- [ ] 1.11 Não aparece nenhum aviso amarelo (sem vendedor ou desligado).

## 2. Normal sem vendedor

- [ ] 2.1 Aparece o aviso amarelo: "Usuario sem vendedor vinculado. As metricas de vendas sao exibidas apenas para usuarios vinculados a um vendedor; solicite o vinculo a um administrador."
- [ ] 2.2 Nenhum número da empresa aparece: vendas, pedidos, ticket e meta ficam zerados ou ocultos, e o mesmo vale para os clientes (0 e listas vazias).
- [ ] 2.3 Gráfico: se estiver visível, mostra os N dias com **barras zeradas** e total R$ 0,00. A API devolve os dias preenchidos com zero.
- [ ] 2.4 Não aparece nenhuma mensagem de erro vermelha. Todas as chamadas voltam 200 com valores zerados.

> **Observação para o FrontBrain/SubBrain:** na versão de `page.tsx` lida em 2026-09-24, quando há aviso (sem vendedor ou desligado) a tela renderiza **só o aviso** e esconde KPIs, gráfico e clientes. O card pede "aviso, números zerados e gráfico com dias zerados". Confirme qual comportamento vale e ajuste o item 2.2/2.3 se o layout final esconder os cards.

## 3. Normal com vendedor desligado (Henrique Rodrigues, vendedor 1)

- [ ] 3.1 Aparece o aviso amarelo: "Vendedor desligado. O vendedor vinculado ao seu usuario possui data de desligamento, por isso nao ha metricas de vendas nem clientes na sua carteira; procure um administrador."
- [ ] 3.2 O aviso de "sem vendedor" **não** aparece. É o aviso específico de desligado.
- [ ] 3.3 Vendas, pedidos, ticket e meta ficam zerados (ou ocultos). Mesmo que o vendedor 1 tenha vendas históricas, **nada** é exibido.
- [ ] 3.4 Os clientes ficam zerados (ou ocultos), embora o vendedor 1 ainda tenha 82 clientes em carteira ativa.
- [ ] 3.5 Ranking e "Meu Desempenho" vêm vazios.
- [ ] 3.6 Bloqueio imediato (opcional): como admin, preencha `data_desligamento` de um vendedor ativo e recarregue o Dashboard do usuário dele, sem novo login. O aviso de desligado aparece. **Desfaça** depois (`UPDATE vendedores SET data_desligamento = NULL WHERE id = <id>;`).

## 4. Admin

- [ ] 4.1 O cabeçalho mostra "Visao geral das metricas de vendas".
- [ ] 4.2 Os KPIs se chamam "Total de Vendas", "Pedidos", "Ticket Medio" e "Ranking Vendedores" (valor do líder + "Lider: <nome>").
- [ ] 4.3 No período "Mês", o Total de Vendas é o da empresa inteira (em 2026-09-24: **R$ 3.761.832,81**, 1.186 pedidos) e a Meta Mensal é a soma das metas dos vendedores ativos (**R$ 2.505.000,00**).
- [ ] 4.4 O card "Acompanhamento de Metas" lista vários vendedores.
- [ ] 4.5 O "Ranking de Vendedores" (Top 10 por volume) mostra 10 linhas. Em 2026-09-24 o líder era Débora Ribeiro, com R$ 158.830,30 e 158,83% da meta. A API informa `total: 36` vendedores com vendas no ranking.
- [ ] 4.6 A seção de clientes usa os rótulos globais ("Total de Clientes" etc.): **3.040** clientes, 2.823 ativos, 217 inativos. O título "Clientes da minha carteira" **não** aparece.
- [ ] 4.7 Não aparece nenhum aviso amarelo.
- [ ] 4.8 O título do gráfico é "Vendas nos Ultimos 30 Dias" mesmo quando se escolhe 7/14/60 dias no select. **Achado menor de UI:** para o admin o título não acompanha o select (no normal ele acompanha). Veja a seção de achados.

## 5. Valores com centavos (sem truncar)

- [ ] 5.1 Admin: no Ranking e em "Acompanhamento de Metas", os valores têm centavos (ex.: Débora Ribeiro **R$ 158.830,30**, Queila Lima **R$ 74.558,44**, João Soares **R$ 135.441,13**), não valores redondos.
- [ ] 5.2 O percentual de meta é calculado sobre o valor com centavos (ex.: Queila Lima 74.558,44 / 100.000 = **74,56%**).
- [ ] 5.3 Normal (vendedor 4): Minhas Vendas R$ 75.148,43, e "Meu Desempenho" e "Minha Meta" mostram o mesmo valor com centavos (136,63%).
- [ ] 5.4 Sem dados (usuário sem vendedor / desligado / período sem vendas), nenhuma tela quebra nem mostra "null"/"NaN". A API devolve `[]` nas listas.

## 6. Limpeza

- [ ] 6.1 Usuário temporário removido.
- [ ] 6.2 `data_desligamento` restaurada se o item 3.6 foi executado.

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
- **UI-01 (frontend, menor):** para o admin, o título do gráfico é fixo "Vendas nos Ultimos 30 Dias" e não acompanha o select de 7/14/60 dias.
- **UI-02 (frontend, a confirmar):** para usuário sem vendedor ou com vendedor desligado, a tela exibe só o aviso e esconde KPIs e gráfico. O card de teste esperava "números zerados e gráfico com dias zerados". Confirmar a intenção com o FrontBrain.
