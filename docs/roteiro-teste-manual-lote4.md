# Roteiro de teste manual: Lote 4 (SEC-02, SEC-03, NEG-01, BUG-06, FE-04, FE-05)

**Cards:** SEC-02 (corrida no refresh token), SEC-03 (escopo de `GET /api/vendedores`), NEG-01 (validação e duplicidade de CNPJ), BUG-06 (`PATCH /inativar` com body inválido), FE-04 (resposta obsoleta nas listagens) e FE-05 (listagens sem refresh automático). Detalhes em `tarefas/fazendo.md`.
**Migração:** `sql/19_alter_clientes_cnpj_unique.sql` (`make db-fix-cnpj-unique`). Ela unifica os 40 CNPJs duplicados e cria o índice UNIQUE `uq_clientes_cnpj`. Reversão: `make db-revert-cnpj-unique`.
**Tempo estimado:** 40 a 50 minutos.
**Autor:** TestBrain (2026-09-25)

Marque cada checkbox depois de conferir. Use o DevTools aberto na aba **Network** nas partes de navegador. Se algo divergir, anote o caso e encaminhe ao SubBrain.

---

## 0. Preparação

1. Suba a API e o frontend (`make dev-api` e `make dev-frontend`).
2. Usuários (confira as senhas que você usa hoje):

   | Perfil | Usuário (e-mail) | id | Vendedor |
   | --- | --- | --- | --- |
   | admin | `admin@rotaperfumes.com.br` | 1 | - |
   | normal com vendedor ativo | `rafael.carvalho@rotaperfumes.com.br` | 5 | 4 - Rafael Carvalho |
   | normal com vendedor **desligado** | `thiago.silva@rotaperfumes.com.br` | 4 | 3 - Thiago Silva (`data_desligamento` = 2025-07-26) |
   | normal **sem** vendedor | `qa.semvendedor@rotaperfumes.test` | 87 | - |

   O `henrique.rodrigues@...` (id 2), usado em roteiros anteriores como desligado, hoje está com `ativo = 0` e não consegue fazer login.

3. Para as chamadas pelo terminal/Postman você precisa do access token. Faça login no navegador e copie o valor do cookie `access_token` em **DevTools > Application > Cookies > http://localhost:3000**. O login pela API exige o CAPTCHA (Turnstile), então o caminho mais simples é copiar o cookie.

---

## 1. Migração NEG-01 (terminal)

> A API pode continuar no ar. O script é idempotente, faz backup das cópias e roda a parte DML em uma transação.

1. **Antes**, anote o estado atual:

   ```sql
   SELECT COUNT(*) AS clientes FROM clientes;                                   -- esperado: 3040
   SELECT COUNT(*) AS grupos_duplicados
     FROM (SELECT cnpj FROM clientes GROUP BY cnpj HAVING COUNT(*) > 1) d;      -- esperado: 40
   SHOW INDEX FROM clientes WHERE Column_name = 'cnpj';                         -- idx_clientes_cnpj, Non_unique = 1
   ```

2. Aplique a migração:

   ```bash
   make db-fix-cnpj-unique
   ```

   - [ ] 1.1 O comando termina sem erro. Se aparecer `ERROR 1062 ... uq_clientes_cnpj`, **pare** e reporte: sobrou alguma cópia e o índice não foi criado (nenhum dado foi perdido).

3. Verificação (cole no `mysql` ou no Workbench):

   ```sql
   SELECT COUNT(*) AS clientes FROM clientes;
   SELECT COUNT(*) AS grupos_duplicados
     FROM (SELECT cnpj FROM clientes GROUP BY cnpj HAVING COUNT(*) > 1) d;
   SHOW INDEX FROM clientes WHERE Column_name = 'cnpj';
   SELECT (SELECT COUNT(*) FROM pedidos)       AS pedidos,
          (SELECT COUNT(*) FROM carteiras)     AS carteiras,
          (SELECT COUNT(*) FROM oportunidades) AS oportunidades,
          (SELECT COUNT(*) FROM visitas)       AS visitas;
   SELECT COUNT(*) AS copias_no_backup FROM clientes_merge_backup_20260925;
   SELECT tabela, acao, COUNT(*) FROM clientes_merge_backup_20260925_vinculos GROUP BY tabela, acao;
   ```

   - [ ] 1.2 `clientes` = **3000**.
   - [ ] 1.3 `grupos_duplicados` = **0**.
   - [ ] 1.4 `SHOW INDEX` mostra **só** `uq_clientes_cnpj` com `Non_unique` = **0**. O `idx_clientes_cnpj` não existe mais.
   - [ ] 1.5 `pedidos` = **28732**, `carteiras` = **3637**, `oportunidades` = **5980**, `visitas` = **37936**. Os filhos das cópias foram transferidos, e não apagados.
   - [ ] 1.6 `copias_no_backup` = **40**. O log `clientes_merge_backup_20260925_vinculos` fica **vazio (0 linhas)** com os dados atuais. Isso é o esperado: nos CSVs de `dados/`, nenhum pedido, carteira, oportunidade ou visita aponta para as cópias 3001 a 3040, então não houve filho transferido nem descartado. Ele só teria linhas se as cópias tivessem filhos. Para conferir: `SELECT COUNT(*) FROM pedidos WHERE cliente_id > 3000` (e o mesmo em carteiras, oportunidades e visitas) = 0.
   - [ ] 1.7 Rode `make db-fix-cnpj-unique` **de novo**. Termina sem erro, e os números de 1.2 a 1.5 não mudam (idempotente).

---

## 2. SEC-02: refresh concorrente

### 2a. Terminal: dois refresh em paralelo com o mesmo token

1. Faça login no navegador com o `rafael.carvalho@...` e copie o valor do cookie `refresh_token`.
2. No Git Bash, dispare os dois refresh ao mesmo tempo:

   ```bash
   RT='<cole o refresh_token aqui>'
   for i in 1 2; do
     curl -s -o /dev/null -w "req $i -> %{http_code}\n" -X POST http://localhost:8080/api/auth/refresh \
       -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$RT\"}" &
   done; wait
   ```

   - [ ] 2.1 Uma requisição responde **200** e a outra **401** (a ordem varia).
   - [ ] 2.2 Repita o comando com o **mesmo** token: as duas respondem **401** `"refresh token revogado"`, porque o token já foi usado.
   - [ ] 2.3 No log da API aparecem `[auth] refresh OK` uma vez e `[auth] refresh: token já revogado por requisição concorrente` uma vez. Se a API estiver rodando no debugger do VS Code, o log fica no **Debug Console**, e não no terminal.
   - [ ] 2.4 Consulta: `SELECT COUNT(*) FROM refresh_tokens WHERE usuario_id = 5 AND revoked_at IS NULL AND created_at > NOW() - INTERVAL 5 MINUTE;` volta **1**. Só um novo par foi emitido.

   > Depois deste teste o `refresh_token` do navegador está revogado. Na próxima renovação, a aba volta para `/login`, o que é esperado. Faça login de novo antes da seção 2b.

### 2b. Navegador: duas abas

1. Faça login com o `rafael.carvalho@...` e abra **duas abas**: `/admin/pedidos` e `/admin/clientes`.
2. Em **DevTools > Application > Cookies**, apague só o cookie `access_token` (mantenha o `refresh_token`). Os cookies valem para as duas abas.
3. Em até 1 ou 2 segundos, clique em ">" (próxima página) na aba 1 e depois na aba 2.

   - [ ] 2.5 **Nenhuma** das abas vai para `/login`, e as duas mostram a página seguinte.
   - [ ] 2.6 Na aba Network de cada aba: o `GET` da lista volta 401, depois vem `POST /api/auth/refresh`, e o `GET` é repetido com **200**. Se a outra aba renovou primeiro, o refresh desta aba pode voltar **401**. Mesmo assim, o `GET` é repetido **uma vez** e volta 200, sem logout.
   - [ ] 2.7 Não aparece nenhum `POST /api/auth/logout`.
   - [ ] 2.8 Controle negativo: apague **os dois** cookies (`access_token` e `refresh_token`) e clique em ">". Sequência real: o `GET` volta 401; o `POST /api/auth/refresh` volta **400** `"refresh_token é obrigatório (body ou cookie)"` (sem cookie não há token para validar); o frontend chama `POST /api/auth/logout` e a tela vai para `/login`, **sem** repetir o `GET`. O que importa é o resultado final: a tela vai para `/login` e nenhum dado é exibido.

   > **Comportamento aceito (SEC-05, decidido em 2026-09-25):** o usuário decidiu manter o `400` sem cookie, sem mudança de código. Esta é a sequência esperada.

---

## 3. SEC-03: `GET /api/vendedores` com escopo

### 3a. Modais e filtros (navegador)

| Usuário | Tela | Esperado |
| --- | --- | --- |
| admin | `/admin/oportunidades` e `/admin/visitas` | Filtro "Vendedor" e select do modal "Nova ..." com **todos** os vendedores (ativos e desligados). |
| `rafael.carvalho` (vend. 4) | `/admin/oportunidades` > "Nova Oportunidade" | Select "Vendedor" **pré-selecionado e travado** em "Rafael Carvalho". A lista de clientes é a carteira dele (`GET /api/vendedores/4/clientes`). |
| `rafael.carvalho` | `/admin/visitas` > "Nova Visita" | Mesmo comportamento do item acima. |
| `qa.semvendedor` | `/admin/oportunidades` e `/admin/visitas` > "Nova ..." | Aviso de usuário sem vendedor, select mostrando "Sem vendedor vinculado" e botão de salvar **desabilitado**. |
| `thiago.silva` (desligado) | qualquer tela da carteira | Aviso "Seu vendedor foi desligado..." (CarteiraGuard). O modal nem abre. |

- [ ] 3.1 admin vê todos os vendedores nos selects.
- [ ] 3.2 normal com vínculo vê só o próprio vendedor, travado, nos dois modais.
- [ ] 3.3 normal sem vínculo: aviso e salvar desabilitado, sem erro vermelho.
- [ ] 3.4 desligado: bloqueado pela tela, sem loop de requisições.

### 3b. API (Console do DevTools, logado com cada usuário)

```js
const r = await fetch("http://localhost:8080/api/vendedores", { credentials: "include" });
console.log(r.status, await r.json());
```

- [ ] 3.5 admin: **200** com todos os vendedores. O total bate com `SELECT COUNT(*) FROM vendedores;`.
- [ ] 3.6 `rafael.carvalho`: **200** com **1** item (`id` 4), com os campos `id, nome, regiao, uf, data_desligamento` e sem `meta_mensal`.
- [ ] 3.7 `qa.semvendedor`: **200** com `"data": []` (array vazio, nunca `null`).
- [ ] 3.8 `thiago.silva`: **403** `{"success":false,"error":"acesso bloqueado: vendedor desligado"}`.

---

## 4. NEG-01: CNPJ no cadastro de clientes (navegador)

Logado como `rafael.carvalho@...`, abra `/admin/clientes` e use **"+ Novo Cliente"**. Para gerar CNPJs válidos de teste, use um gerador de CNPJ (ex.: 4devs). Anote a razão social de cada cliente criado para excluir depois.

| # | CNPJ digitado | Esperado |
| --- | --- | --- |
| 4.1 | válido **com máscara** (ex.: `11.444.777/0001-61`, se ainda não existir na base) | Salva. Na lista e no `SELECT cnpj FROM clientes WHERE razao_social = '...'`, o CNPJ aparece **só com dígitos** (`11444777000161`). |
| 4.2 | `123` | Erro "cnpj inválido" no modal, e nada é gravado. |
| 4.3 | 14 dígitos com DV errado (ex.: `11.444.777/0001-62`) | "cnpj inválido". |
| 4.4 | `00.000.000/0000-00` ou `11111111111111` | "cnpj inválido". |
| 4.5 | CNPJ de um cliente que **já existe**, é de **outro vendedor** e tem **DV válido** (ex.: `23124329212779`, cliente 52, vendedor 33; ou use a consulta abaixo) | **409**, com a mensagem "cnpj já cadastrado". A resposta **não** mostra id, razão social nem vendedor do cliente existente (confira o corpo na aba Network). |
| 4.6 | O mesmo do 4.5, digitado **com máscara** | Também 409 (a comparação é feita sobre os dígitos). |

> **Use um CNPJ com DV válido nos itens 4.5, 4.6, 4.10 e 4.11.** Os CNPJs importados são fictícios: só **28 dos 3000** têm DV válido. A API valida o formato e o DV **antes** de checar a duplicidade, então um CNPJ legado com DV inválido (ex.: `17810801326773`) responde **400** "cnpj inválido", e não 409. Esse é o comportamento aceito (NEG-03, decidido em 2026-09-25): um CNPJ com DV inválido é recusado e não é gravado, a checagem de duplicidade só vale para CNPJ válido, e o índice UNIQUE continua protegendo contra o duplicado.
>
> Consulta que devolve só CNPJs de outro vendedor com DV válido (módulo 11). Deve trazer, entre outros, o `23124329212779`:
>
> ```sql
> SELECT cliente_id_origem, cnpj, vendedor_id FROM (
>   SELECT c.cliente_id_origem, c.cnpj, k.vendedor_id,
>          SUBSTRING(c.cnpj,1,1)*5 + SUBSTRING(c.cnpj,2,1)*4 + SUBSTRING(c.cnpj,3,1)*3 + SUBSTRING(c.cnpj,4,1)*2
>        + SUBSTRING(c.cnpj,5,1)*9 + SUBSTRING(c.cnpj,6,1)*8 + SUBSTRING(c.cnpj,7,1)*7 + SUBSTRING(c.cnpj,8,1)*6
>        + SUBSTRING(c.cnpj,9,1)*5 + SUBSTRING(c.cnpj,10,1)*4 + SUBSTRING(c.cnpj,11,1)*3 + SUBSTRING(c.cnpj,12,1)*2 AS s1,
>          SUBSTRING(c.cnpj,1,1)*6 + SUBSTRING(c.cnpj,2,1)*5 + SUBSTRING(c.cnpj,3,1)*4 + SUBSTRING(c.cnpj,4,1)*3
>        + SUBSTRING(c.cnpj,5,1)*2 + SUBSTRING(c.cnpj,6,1)*9 + SUBSTRING(c.cnpj,7,1)*8 + SUBSTRING(c.cnpj,8,1)*7
>        + SUBSTRING(c.cnpj,9,1)*6 + SUBSTRING(c.cnpj,10,1)*5 + SUBSTRING(c.cnpj,11,1)*4 + SUBSTRING(c.cnpj,12,1)*3
>        + SUBSTRING(c.cnpj,13,1)*2 AS s2
>     FROM clientes c
>     JOIN carteiras k ON k.cliente_id = c.cliente_id_origem AND k.data_fim IS NULL
>    WHERE k.vendedor_id <> 4
>      AND c.cnpj REGEXP '^[0-9]{14}$'
>      AND c.cnpj <> REPEAT(LEFT(c.cnpj,1), 14)
> ) t
> WHERE SUBSTRING(cnpj,13,1) = IF(MOD(s1,11) < 2, 0, 11 - MOD(s1,11))
>   AND SUBSTRING(cnpj,14,1) = IF(MOD(s2,11) < 2, 0, 11 - MOD(s2,11))
> LIMIT 5;
> ```
>
> Alternativa sem SQL: use o CNPJ do cliente criado no 4.1 (DV válido) e tente cadastrá-lo de novo como **admin** ou com outro usuário com vendedor.

Edição (`Editar` em um cliente da carteira do Rafael, ex.: id 9 "Perfumaria Sublime S/A"):

**Antes do 4.8**, anote o CNPJ original do cliente: `SELECT cnpj FROM clientes WHERE cliente_id_origem = 9;`.

> **Regra do NEG-04 (2026-09-25):** toda gravação exige CNPJ com DV válido, mesmo quando o `PUT` mantém o CNPJ atual. Os CNPJs importados são fictícios, e cerca de 2.970 clientes legados têm DV inválido: eles só podem ser editados depois de corrigir o CNPJ (impacto aceito pelo usuário). **Reinicie a API antes da seção 4.** Se ela ainda estiver com o binário anterior ao NEG-04 (por exemplo, no debugger do VS Code), o 4.7 volta 200.

- [ ] 4.7 No cliente 9 (CNPJ legado com DV inválido), altere só a razão social, **mantendo o CNPJ**, e salve. Resultado: **400** "cnpj inválido" no modal, e **nada** é gravado (confira com `SELECT razao_social, cnpj FROM clientes WHERE cliente_id_origem = 9;`).
- [ ] 4.7b Para testar um PUT **200** mantendo o CNPJ, use um cliente com DV válido (ex.: o criado no 4.1). Alterar só a razão social salva com 200, e o CNPJ continua o mesmo, só com dígitos.
- [ ] 4.8 Trocar o CNPJ do cliente 9 por um válido e **inédito** salva com 200 e grava só os dígitos. **Desfaça** depois **por SQL**: a tela e a API recusam voltar ao CNPJ legado com DV inválido (400 "cnpj inválido"). Use:

   ```sql
   UPDATE clientes SET cnpj = '<CNPJ original anotado>' WHERE cliente_id_origem = 9;
   ```

   > **Decidido e implementado (NEG-04, 2026-09-25): toda gravação exige DV válido.** Voltar a um CNPJ legado com DV inválido só é possível por SQL. Esse é o comportamento esperado.
- [ ] 4.9 Trocar o CNPJ por `123` dá "cnpj inválido".
- [ ] 4.10 Trocar o CNPJ pelo de **outro** cliente **com DV válido** (ex.: `23124329212779`) dá 409 "cnpj já cadastrado", sem vazar dados. Com o CNPJ de um cliente legado com DV inválido, a resposta é 400 "cnpj inválido" (ver nota do 4.5).
- [ ] 4.11 Repita 4.5 como **admin** (sem carteira), com o mesmo CNPJ de DV válido: também 409 genérico.
- [ ] 4.12 **Sem** a migração da seção 1, o item 4.5 **grava** o duplicado. Se isso acontecer, a migração não foi aplicada: exclua o duplicado e volte à seção 1.
- [ ] 4.13 **Inativar cliente legado (NEG-04):** no cliente 9, com o CNPJ legado de DV inválido (depois do `UPDATE` do 4.8), use "Inativar" (`PATCH /api/clientes/9/inativar`). Resultado: **200**, porque o `/inativar` não valida o CNPJ. Clique de novo para reativar (200) e deixar o estado original.

---

## 5. BUG-06: `PATCH .../inativar` pelo Postman

Coleção `postman/collection.json`. Cole o access token (passo 0.3) do **admin** na variável `admin_token` e use-o no header `Authorization` das três pastas: **Ativar/Inativar Cliente**, **Ativar/Inativar Produto (admin only)** e **Ativar/Inativar Usuário (admin only)**. Use registros de teste, ou anote o estado original para restaurar.

Para cada um dos três endpoints (`/api/clientes/{id}/inativar`, `/api/produtos/{id}/inativar`, `/api/usuarios/{id}/inativar`):

| # | Body (raw, JSON) | Esperado |
| --- | --- | --- |
| 5.1 | `{"ativo":"false"}` (string) | **400** `"body JSON inválido"`. O estado **não muda** (confira com `GET` ou `SELECT ativo`). |
| 5.2 | `{"ativo":1}` | 400, estado inalterado. |
| 5.3 | `{"ativo":` (malformado) | 400, estado inalterado. |
| 5.4 | `{"ativo":true} lixo` | 400, estado inalterado. |
| 5.5 | sem body (aba Body = none) | **200** e **alterna** o estado. |
| 5.6 | `null` | 200, alterna. |
| 5.7 | `{}` | 200, alterna. |
| 5.8 | `{"ativo":false}` duas vezes | 200 e `ativo = false` nas duas (define, não alterna). |
| 5.9 | `{"ativo":true}` | 200, `ativo = true`. Estado restaurado. |

- [ ] 5.10 Cliente fora da carteira, com token de usuário **normal** e body inválido: **404** (o escopo é checado antes do body; não pode vazar 400).

---

## 6. FE-04: listagens sem resposta obsoleta (navegador)

Logado como **admin**. No DevTools, em **Network**, ative o throttling **"Slow 3G"** nos itens 6.1 e 6.2.

Telas: `/admin/pedidos`, `/pagamentos`, `/admin/clientes`, `/admin/oportunidades`, `/admin/visitas`, `/admin/estoque`, `/admin/produtos`, `/admin/usuarios`, `/admin/senha-historico`.

> **Telas com poucas páginas:** `/admin/usuarios` tem só 3 páginas e `/admin/senha-historico` tem 1. Os itens 6.1 e 6.2 não se aplicam a essas duas telas (na execução de 2026-09-25 foram marcados como "não se aplica").

- [ ] 6.1 **Paginação rápida:** clique ">" três vezes seguidas, rápido. No fim, a tabela mostra os dados da **mesma** página que o paginador ("Pagina 4 de N"), e não de uma página intermediária. Compare o primeiro ID da tabela com a resposta da **última** requisição na aba Network.
- [ ] 6.2 **Ida e volta:** clique ">" e logo em seguida "<". A tabela termina na página 1, com os dados da página 1.
- [ ] 6.3 **Uma busca ao abrir** (sem throttling): recarregue a tela com a aba Network limpa. Aparece **um único** `GET` da listagem (ex.: `GET /api/pedidos?page=1...`), e não dois. Espere 1 segundo: nenhuma requisição extra é disparada pelo debounce dos filtros.

   > **Atenção (modo dev):** com `make dev-frontend` (`next dev`) e `reactStrictMode: true` em `frontend/next.config.js`, o React roda os efeitos **duas vezes** na montagem, e toda listagem mostra **2 GETs idênticos** ao abrir. Isso é do modo de desenvolvimento, e não do FE-04. Para validar o 6.3, suba o frontend em modo produção (`cd frontend && npx next build && npx next start`) e confira que sai **um** GET. Em `next dev`, o critério é: no máximo 2 GETs **idênticos** (mesma URL, `page=1`) e nenhum GET extra depois de 1 segundo.
   >
   > **Confirmado em produção (2026-09-25, TestBrain):** com `next build --webpack` numa cópia do frontend, servida por `next start -p 3001` (o navegador acessou `localhost:3000`, com HTML e JS vindos do build), as **9/9** listagens fizeram **1 único GET** ao abrir, sem GET extra do debounce em 1,5 s.
- [ ] 6.4 **Filtro na página 2:** vá para a página 2 e digite em um filtro. Depois da pausa, sai **uma** requisição com `page=1`, e o paginador mostra a página 1.
- [ ] 6.5 **Clique rápido logo após abrir:** recarregue e clique ">" em menos de meio segundo. A tela fica na página 2, e não volta para a página 1.
- [ ] 6.6 **Exclusão** (pedidos, pagamentos, oportunidades e visitas): crie um registro de teste, exclua-o e, **imediatamente**, pagine para frente e para trás (com Slow 3G). A linha excluída **não reaparece**, e o total diminui 1.

   > **Pré-condições do registro de teste:** um pagamento quitado ("Pago" ou "Pago com atraso") **não** pode ser excluído (409, regra do SecBrain), e um pedido com pagamento vinculado também **não** (409). Use um pagamento **"Em aberto"** e um pedido **sem** pagamento.

---

## 7. FE-05: refresh automático nas listagens (navegador)

Logado com `rafael.carvalho@...`.

1. Abra `/admin/pedidos`.
2. Em **Application > Cookies**, apague só o `access_token` (o equivalente a deixar o token expirar).
3. Clique ">" (ou "Buscar").

   - [ ] 7.1 Na Network: `GET /api/pedidos...` volta **401**, depois `POST /api/auth/refresh` volta **200**, e o `GET /api/pedidos...` é repetido com **200**. A tabela mostra a página, com o paginador correto (a paginação do envelope é preservada).
   - [ ] 7.2 O cookie `access_token` volta a existir, e **não** há redirect para `/login`.
   - [ ] 7.3 Repita 1 a 3 em `/pagamentos`, `/admin/clientes`, `/admin/oportunidades` e `/admin/visitas` com o Rafael. Como **admin**, repita em `/admin/estoque`, `/admin/produtos` (telas só de admin: com o Rafael elas redirecionam para `/pagamentos`), no **Dashboard** (ranking de vendedores), em `/admin/usuarios` e em `/admin/senha-historico`. O comportamento é o mesmo em todas.

      > Em `/admin/senha-historico` há só 1 página e a busca filtra só pelo cliente, sem nova requisição. O gatilho do passo 3 nessa tela é trocar o select **"Itens por pagina"**.
   - [ ] 7.4 Apague os **dois** cookies e clique ">". Mesmo comportamento do 2.8: o `GET` volta 401, o refresh volta **400** "refresh_token é obrigatório (body ou cookie)", o frontend chama `POST /api/auth/logout` e a tela vai para `/login`, **sem** repetir o `GET` (comportamento aceito: SEC-05, decidido em 2026-09-25).
   - [ ] 7.5 **Texto durante o carregamento** (`/admin/clientes`): com o throttling "Slow 3G", recarregue a tela. Enquanto o `/api/auth/me` carrega, a tela inteira mostra só o spinner **"Verificando autenticacao..."** (ProtectedRoute), e o botão "+ Novo Cliente" ainda não aparece. Por isso o texto "Carregando dados do usuario..." normalmente **não** é visto. O que se confere: em nenhum momento aparece o motivo "Usuario sem vendedor vinculado" para o Rafael. Depois do `/me`, o botão já aparece no estado final: **habilitado** para o Rafael; com o `qa.semvendedor`, **desabilitado** com o motivo real ("Usuario sem vendedor vinculado: solicite o vinculo a um administrador.").

---

## 8. Limpeza

- [ ] 8.1 Clientes de teste (seção 4) excluídos, e o cliente editado em 4.8 com o CNPJ original de volta (pelo `UPDATE` do 4.8).
- [ ] 8.2 Estados `ativo` alterados na seção 5 restaurados.
- [ ] 8.3 Registros criados em 6.6 excluídos.
- [ ] 8.4 Throttling do DevTools de volta a "No throttling".
- [ ] 8.5 (Só se precisar desfazer a migração) `make db-revert-cnpj-unique`, e depois confira `clientes` = 3040.

---

## Testes automatizados

Rodam com `make test` (Go), `make test-frontend` (Vitest) e, com MySQL local, `INTEGRATION=1`:

```bash
cd apis/rotaperfumes-api && INTEGRATION=1 go test ./handlers/ ./services/ -run Integracao -count=1 -v
```

| Arquivo | O que cobre |
| --- | --- |
| `apis/rotaperfumes-api/handlers/lote4_http_integration_test.go` | Integração com o router real e o MySQL local (INTEGRATION=1, dados `ZZ-TEST-HTTP-*` apagados no fim). **SEC-02:** 8 rodadas de 2 refresh paralelos, cada uma com exatamente um 200 e um 401, o token antigo revogado e só um novo emitido. **SEC-03:** admin, normal com vínculo, sem vínculo e desligado. **BUG-06:** 6 bodies inválidos (400, estado inalterado no banco), toggle com vazio, `null`, `{}` e `{"ativo":null}`, e o valor explícito, nos 3 endpoints. **NEG-01:** 400 (formato, letras, todos iguais, DV), máscara gravada só com dígitos, PUT com o mesmo CNPJ válido e 409 genérico sem vazamento (pulado até a migração 19 ser aplicada). **NEG-04:** `TestIntegracaoHTTP_NEG04_CNPJLegadoDVInvalido` (PUT de cliente legado mantendo o CNPJ com DV inválido → 400 sem gravar; `/inativar` continua 200). |
| `apis/rotaperfumes-api/handlers/auth_refresh_logout_test.go` | SEC-02 com sqlmock: corrida (um 200 e um 401, só um par emitido), falhas na rotação (500 sem cookie), 401 concorrente contando no rate limit. |
| `apis/rotaperfumes-api/services/refresh_rotation_test.go`, `apis/shared/repositories/lote4_repository_test.go` | `BeginRotation`/`Issue`/`Rollback` e o `UPDATE ... AND revoked_at IS NULL`. |
| `apis/rotaperfumes-api/handlers/vendedor_handler_test.go`, `services/vendedor_proprio_test.go` | SEC-03 com a tabela de escopo (vínculo, sem vínculo, órfão, desligado e erros). |
| `apis/rotaperfumes-api/handlers/inativar_body_test.go` | BUG-06: 8 bodies aceitos e 9 inválidos × 3 endpoints, sem chamar o service no 400. |
| `apis/rotaperfumes-api/handlers/cliente_cnpj_test.go`, `services/cliente_cnpj_test.go`, `services/cnpj_internal_test.go` | NEG-01: máscara, DV (módulo 11) e 409 genérico (Create, Create na carteira com rollback, Update). NEG-04 (junto com `services/cliente_service_test.go`, `handlers/cliente_handler_test.go` e `handlers/cliente_sec01_cobertura_test.go`): Update com o mesmo CNPJ legado de DV inválido → 400, com e sem máscara (`TestUpdateCliente_CNPJ`); DV inválido tem precedência sobre o 404 (`TestClienteService_UpdateCliente_DVInvalidoTemPrecedenciaSobre404`); `/inativar` de legado com DV inválido (`TestToggleAtivoCliente_CNPJLegadoDVInvalido`). |
| `apis/shared/cmd/internal/clientesdedup/`, `cmd/importclientes/unificacao_test.go`, `cmd/importcarteiras/unificacao_test.go` | Dedup dos importadores (a 1ª ocorrência do CNPJ fica, e as cópias são redirecionadas). |
| `frontend/src/lib/apiClient.test.ts` | SEC-02 multi-aba: retry único após refresh 401 (OK, erro não-401, 401 leva a logout, falha de rede, fila liberada), sem retry em 429, 500, timeout, erro de rede e falha do Web Lock, e o Web Lock `rp-auth-refresh`. |
| `frontend/src/lib/apiListagensRefresh.test.ts` | FE-05: listagens pelo `fetchWithAuth`/`fetchEnvelopeWithAuth` (refresh, fila única, logout, 403 de desligado, envelope preservado). |
| `frontend/src/lib/useListaSegura.test.ts`, `app/listasPaginadas.test.tsx`, `app/admin/listasAdmin.test.tsx`, `app/crudPaginas.test.tsx`, `app/admin/crudAdmin.test.tsx` | FE-04: última resposta vence, sem debounce na montagem, e exclusão sem a linha reaparecer. Não há mais nenhum `it.fails`. |
| `frontend/src/components/admin/VendedorTravado.test.tsx`, `app/admin/clientes/page.test.tsx` | SEC-03 nos modais (travado, sem vínculo, admin, 403) e o texto de carregamento do FE-05. |

---

## Execução 2026-09-25 (TestBrain)

Executado contra `localhost:3000` e `localhost:8080`. **Nenhum defeito funcional bloqueante.** As divergências abaixo já foram incorporadas ao texto deste roteiro.

| Seção | Resultado |
| --- | --- |
| 1. Migração 19 | OK. Já estava aplicada; reaplicada sem erro (idempotente): 3000 clientes, 0 duplicados, só `uq_clientes_cnpj` com `Non_unique` = 0; pedidos 28732, carteiras 3637, oportunidades 5980, visitas 37936; 40 cópias no backup. `clientes_merge_backup_20260925_vinculos` com 0 linhas: log vazio é o esperado (DB-01, fechado). |
| 2a. Refresh em paralelo | 2.1, 2.2 e 2.4 OK. 2.3 não verificado (API no debugger do VS Code, log não acessível). |
| 2b. Duas abas | 2.5, 2.6 e 2.7 OK (corrida real, Web Lock funcionando). 2.8: divergência D1. |
| 3. SEC-03 | 3.1 a 3.8 OK. |
| 4. NEG-01 | Backend OK: máscara gravada só com dígitos, 400 "cnpj inválido", 409 genérico "cnpj já cadastrado" sem vazar dados. Na execução, o PUT com CNPJ legado mantido deu 200 (regra anterior); com o **NEG-04, implementado depois deste roteiro**, esse caso passou a dar 400, e os itens 4.7 e 4.8 foram reescritos (e criados o 4.7b e o 4.13). Divergências D2 e D3. |
| 5. BUG-06 | 5.1 a 5.10 OK nos 3 endpoints. |
| 6. FE-04 | 6.1, 6.2, 6.4, 6.5 e 6.6 OK em todas as telas aplicáveis (respostas forçadas fora de ordem). 6.3 falha em `next dev` (D4) e foi **confirmado em produção**: 9/9 telas com 1 GET ao abrir. |
| 7. FE-05 | 7.1 a 7.3 OK nas 10 telas. 7.4 igual ao 2.8 (D1). 7.5: D5. |
| 8. Limpeza | Feita; contagens de volta aos valores de referência. Migração **não** revertida. |

**Divergências:**

- **D1 (2.8/7.4):** sem cookie, o refresh responde 400 (e não 401), e o front faz logout e vai para `/login` sem repetir o `GET`. Resultado final correto. Roteiro ajustado. Comportamento aceito (**SEC-05**, decidido em 2026-09-25: manter o 400, sem mudança de código).
- **D2 (4.5/4.6/4.10/4.11):** a consulta antiga trazia CNPJs legados com DV inválido, que respondem 400 antes da checagem de duplicidade (só 28 de 3000 têm DV válido). Com CNPJ válido de outro vendedor (`23124329212779`), o 409 funciona. Roteiro ajustado. Comportamento aceito (**NEG-03**, decidido em 2026-09-25: manter o 400, sem mudança de código).
- **D3 (4.8):** voltar ao CNPJ legado original pela tela/API dá 400; o "desfazer" é por SQL. **Decidido e implementado (NEG-04, 2026-09-25): toda gravação exige DV válido**, inclusive o PUT que mantém o CNPJ legado (antes, 200). O NEG-04 foi implementado **depois** desta execução. Os itens 4.7 e 4.8 mudaram, e foram criados o 4.7b (PUT 200 com cliente de DV válido) e o 4.13 (`/inativar` de legado = 200). Esses itens foram reexecutados com sucesso com a API reiniciada (ver "Revalidação do NEG-04" abaixo).
- **D4 (6.3):** em `next dev` com `reactStrictMode: true`, toda listagem faz 2 GETs idênticos ao abrir. Roteiro ajustado (validar com `next build && next start`). **Resolvido como comportamento esperado do modo dev:** em produção, 9/9 telas fizeram 1 único GET (TestBrain, 2026-09-25).
- **D5 (7.5):** "Carregando dados do usuario..." não aparece, porque o ProtectedRoute mostra só "Verificando autenticacao..." até o `/me` responder. O objetivo (nunca mostrar o motivo errado) é atendido. Roteiro ajustado.
- **D6 (escrita do roteiro):** estoque/produtos no 7.3 são telas só de admin; pré-condições de exclusão no 6.6; gatilho "Itens por pagina" em senha-historico; 6.1/6.2 não se aplicam a usuarios (3 páginas) e senha-historico (1 página). Roteiro ajustado.

**Revalidação do NEG-04 (TestBrain, 2026-09-25, API reiniciada às 18:22 com o código do NEG-04):** os itens 4.7, 4.7b, 4.8 e 4.13 foram reexecutados com sucesso.

- **4.7:** pela tela, o PUT no cliente 9 mantendo o CNPJ legado deu 400 "cnpj inválido". A mensagem apareceu no modal, e o banco ficou inalterado (inclusive `updated_at`).
- **4.7b:** PUT mantendo o CNPJ no cliente de teste com CNPJ válido = 200.
- **4.8:** a troca por um CNPJ válido e inédito deu 200 e gravou só os dígitos. A volta ao legado pela API deu 400, e foi feita por SQL.
- **4.13:** inativar e reativar o cliente 9 com o CNPJ legado deu 200 nos dois.
- **Limpeza:** cliente de teste apagado, cliente 9 restaurado, 3000 clientes e 3637 carteiras.
- **Postman:** `12345678000199` foi trocado por `11222333000181` (JSON validado, 0 ocorrências restantes), e o README registra a migração 19 como aplicada.

**Lote 4 fechado em 2026-09-25.** Os cards do lote (SEC-02, SEC-03, NEG-01, BUG-06, FE-04 e FE-05) e os derivados (SEC-05, NEG-03, NEG-04 e DB-01) estão em `tarefas/feito.md`.
