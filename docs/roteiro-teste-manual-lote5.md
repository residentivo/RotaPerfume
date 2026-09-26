# Roteiro de teste manual: Lote 5 (SEC-04, FE-06, FE-08, NEG-02, DOC-01, DOC-02, DB-02)

**Cards:** SEC-04 (a corrida legítima no refresh não conta no rate limit), FE-06 (mensagem de erro de rede na fila de refresh), FE-08 (mensagem amigável para falha de rede e timeout no apiClient), NEG-02 (CNPJ alfanumérico da Receita), DOC-01 (roteiro do vendedor desligado), DOC-02 (manual da base de dados) e DB-02 (COMMENT de `clientes.cnpj`). Detalhes em `tarefas/fazendo.md`.
**Migração:** só a 20 (`sql/20_alter_clientes_cnpj_comment.sql`, DB-02), que troca **apenas o COMMENT** de `clientes.cnpj`. O tipo continua `CHAR(14)` `utf8mb4_unicode_ci`, com o índice `uq_clientes_cnpj`, e nenhum dado é alterado. Conferência no item 4.4.
**Tempo estimado:** 35 a 45 minutos.
**Autor:** TestBrain (2026-09-25)

Marque cada checkbox depois de conferir. Nas partes de navegador, deixe o DevTools aberto na aba **Network**. Se algo divergir, anote o caso e encaminhe ao SubBrain.

---

## 0. Preparação

1. **Reinicie a API** com o código do Lote 5 (`make dev-api`) e suba o frontend (`make dev-frontend`). Se a API estiver no debugger do VS Code, reinicie a sessão de debug: o binário antigo conta a corrida do refresh no rate limit e recusa CNPJ com letras.
2. Usuários (confira as senhas que você usa hoje):

   | Perfil | Usuário (e-mail) | id | Vendedor |
   | --- | --- | --- | --- |
   | admin | `admin@rotaperfumes.com.br` | 1 | - |
   | normal com vendedor ativo | `rafael.carvalho@rotaperfumes.com.br` | 5 | 4 - Rafael Carvalho |
   | normal com vendedor **desligado** | `thiago.silva@rotaperfumes.com.br` | 4 | 3 - Thiago Silva (`data_desligamento` = 2025-07-26) |

3. CNPJs de teste (nenhum existe na base: os 3000 clientes atuais são numéricos):

   | Uso | CNPJ | Com máscara |
   | --- | --- | --- |
   | Alfanumérico válido (exemplo oficial da Receita) | `12ABC34501DE35` | `12.ABC.345/01DE-35` |
   | Alfanumérico válido 2 | `RP5LOTE0000141` | `RP.5LO.TE0/0001-41` |
   | Alfanumérico válido 3 (edição) | `RP5LOTE0000222` | `RP.5LO.TE0/0002-22` |
   | Alfanumérico com DV errado | `12ABC34501DE36` | `12.ABC.345/01DE-36` |
   | Numérico válido | `11222333000181` | `11.222.333/0001-81` |

   Antes de começar, confirme que eles não existem: `SELECT cliente_id_origem, cnpj FROM clientes WHERE cnpj IN ('12ABC34501DE35','RP5LOTE0000141','RP5LOTE0000222');` deve voltar **0 linhas**. O `11222333000181` pode já existir se sobrou de um teste anterior. Nesse caso, use-o só no item 3.14.

4. **Rate limit do refresh:** são 10 falhas por IP em 1 minuto, e o bloqueio dura **5 minutos**. Na seção 1c o bloqueio é provocado de propósito. Como o navegador e o terminal usam o mesmo IP (`127.0.0.1`/`::1`), faça a seção 1c **por último na seção 1** e, depois dela, **reinicie a API** (o limiter fica em memória) antes de seguir para a seção 2.

---

## 1. SEC-04: corrida no refresh não conta no rate limit

### 1a. Terminal: 15 refresh em paralelo com o mesmo token

1. Faça login no navegador com o `rafael.carvalho@...` e copie o valor do cookie `refresh_token` (**DevTools > Application > Cookies > http://localhost:3000**).
2. No Git Bash:

   ```bash
   RT='<cole o refresh_token aqui>'
   for i in $(seq 1 15); do
     curl -s -o /dev/null -w "req $i -> %{http_code}\n" -X POST http://localhost:8080/api/auth/refresh \
       -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$RT\"}" &
   done; wait
   ```

   - [ ] 1.1 Exatamente **um** `200` e **catorze** `401`. **Nenhum** `429`.
   - [ ] 1.2 No log da API aparecem `[auth] refresh OK: user_id=5` uma vez e, para os outros, `token revogado recentemente (corrida entre abas), não conta no rate limit` e/ou `token já revogado por requisição concorrente (corrida no BeginRotation), não conta no rate limit`. **Não** aparece `[auth][seguranca]`.
   - [ ] 1.3 Só um novo par foi emitido: `SELECT COUNT(*) FROM refresh_tokens WHERE usuario_id = 5 AND revoked_at IS NULL AND created_at > NOW() - INTERVAL 2 MINUTE;` volta **1**.

### 1b. Terminal: aba atrasada dentro da janela de graça (30 s)

Logo depois do 1a (em menos de 30 s), repita **o mesmo token** 12 vezes, em sequência:

```bash
for i in $(seq 1 12); do
  curl -s -w " <- req $i %{http_code}\n" -X POST http://localhost:8080/api/auth/refresh \
    -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$RT\"}"
done
```

- [ ] 1.4 As 12 respondem **401** `{"success":false,"error":"refresh token revogado"}` e **nenhuma** responde 429, mesmo passando de 10 falhas.
- [ ] 1.5 Nenhuma resposta traz `Set-Cookie` (confira com `curl -i` em uma delas). Nenhum token é emitido.

> Se passar de 30 s entre o 1a e o 1b, as tentativas passam a contar (seção 1c). Nesse caso, faça login de novo e refaça o 1a e o 1b.

### 1c. Terminal: reuso fora da janela conta (possível roubo)

> Faça por último na seção 1: este item bloqueia o refresh do seu IP por 5 minutos.

1. Espere **pelo menos 35 segundos** depois do último refresh do 1b.
2. Rode de novo o laço do 1b, com o mesmo `$RT`, mas com **11** tentativas (`seq 1 11`).

   - [ ] 1.6 As **10** primeiras respondem **401** com **exatamente o mesmo corpo** do 1.4 (`"refresh token revogado"`). O cliente não consegue distinguir "dentro" de "fora" da janela.
   - [ ] 1.7 A **11ª** responde **429**, com o header `Retry-After` (`curl -i`).
   - [ ] 1.8 No log aparece, para cada uma das 10, `[auth][seguranca] refresh: reuso de refresh token revogado fora da janela de graça (possível roubo de token): ip=... ua=curl/...`, e depois `IP bloqueado por rate limit`.
3. **Reinicie a API** para zerar o bloqueio (ou espere 5 minutos).

### 1d. Navegador: várias abas

1. Faça login com o `rafael.carvalho@...` e abra **cinco abas**: `/admin/pedidos`, `/admin/clientes`, `/pagamentos`, `/admin/oportunidades` e `/admin/visitas`.
2. Em **Application > Cookies**, apague só o cookie `access_token` (mantenha o `refresh_token`).
3. Em até 2 segundos, clique em ">" (próxima página) nas cinco abas, uma depois da outra.

   - [ ] 1.9 Nenhuma aba vai para `/login`, e todas mostram a página seguinte.
   - [ ] 1.10 Na Network, nenhum `POST /api/auth/refresh` volta **429**, e nenhum `POST /api/auth/logout` é disparado. Um refresh que volte 401 (outra aba renovou antes) é seguido do `GET` repetido com 200.
   - [ ] 1.11 Repita os passos 2 e 3 **três vezes seguidas** (mais de 10 refresh em 1 minuto). Continua sem 429 e sem logout.

   > Com Web Locks (Chrome, Edge e Firefox atuais), as abas do mesmo navegador renovam em fila, e os refresh costumam voltar todos 200. A corrida de verdade, que antes do SEC-04 somava falhas até o 429, é a do item 1a (terminal) ou a de dois navegadores diferentes com o mesmo cookie.

---

## 2. FE-06: erro de rede na fila de refresh não vira "Sessão expirada"

### 2a. Simulação da falha de rede (Console do DevTools)

Não dá para derrubar a rede no momento exato do retry pela interface, então a simulação troca o `fetch` da página. O Dashboard dispara **4** requisições em paralelo, o que forma a fila do refresh.

1. Faça login com o `rafael.carvalho@...` e abra `/dashboard`.
2. Cole no Console:

   ```js
   (() => {
     const orig = window.fetch.bind(window);
     const vistos = new Set();
     let falhou = false;
     const json = (status, body) =>
       new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
     window.fetch = async (input, init) => {
       const u = String(input instanceof Request ? input.url : input);
       if (u.includes("/api/auth/refresh")) {
         await new Promise((r) => setTimeout(r, 1500)); // segura o refresh: as outras entram na fila
         return json(401, { success: false, error: "refresh token revogado" }); // "outra aba já renovou"
       }
       if (u.includes("/api/auth/")) return orig(input, init);
       if (!vistos.has(u)) { vistos.add(u); return json(401, { success: false, error: "token expirado" }); }
       if (!falhou) { falhou = true; throw new TypeError("Failed to fetch"); } // retry cai por rede
       return orig(input, init);
     };
     window.__restaurarFetch = () => { window.fetch = orig; };
   })();
   ```

3. Clique no botão de período **"Semana"** do Dashboard (o padrão é "Mes") para disparar as 4 requisições.

   - [ ] 2.1 O Dashboard mostra o erro de rede amigável do FE-08, **"Não foi possível conectar ao servidor. Verifique sua conexão com a internet e tente novamente."**, e **não** "Sessão expirada. Faça login novamente.". O texto cru "Failed to fetch" **não** aparece mais: o `TypeError` do retry vira `NetworkError` (`kind` "conexao") no apiClient.
   - [ ] 2.2 A tela **não** vai para `/login`, o menu continua com o nome do usuário, e o `auth_user` continua no **Local Storage**.
   - [ ] 2.3 Não aparece `POST /api/auth/logout` na Network. As chamadas `/api/auth/*`, exceto o refresh, passam pelo `fetch` real e apareceriam lá. As respostas simuladas (401 e refresh) **não** aparecem na Network, o que é esperado.
4. Restaure o `fetch` no Console: `__restaurarFetch()`. Troque o período de novo: o Dashboard carrega normalmente, sem novo login.

   - [ ] 2.4 Depois do `__restaurarFetch()`, o Dashboard volta a carregar (o estado do refresh foi liberado; nada fica "preso").

### 2b. Controle: 401 de verdade continua levando a logout

1. Ainda no `/dashboard`, em **Application > Cookies**, apague o `access_token` e **edite** o valor do `refresh_token` para `invalido` (duplo clique no valor).
2. Troque o período do Dashboard.

   - [ ] 2.5 Na Network: os `GET` voltam 401, **um** `POST /api/auth/refresh` volta **401**, o `GET` original é repetido uma vez (401), e o front chama `POST /api/auth/logout`.
   - [ ] 2.6 A tela vai para `/login`. Se a mensagem aparecer antes do redirect, ela é "Sessão expirada. Faça login novamente.".
   - [ ] 2.7 O `auth_user` some do Local Storage.

---

## 2A. FE-08: mensagem amigável para falha de rede e timeout

O apiClient converte a rejeição do `fetch` em `NetworkError`:

| Situação | `kind` | Mensagem na tela |
| --- | --- | --- |
| Rede caída, API fora do ar, CORS etc. ("Failed to fetch") | `conexao` | "Não foi possível conectar ao servidor. Verifique sua conexão com a internet e tente novamente." |
| `TimeoutError` (ex.: `AbortSignal.timeout` do chamador) | `timeout` | "O servidor demorou para responder. Tente novamente." |
| Abort intencional (`AbortError`, signal do chamador) | - | sem conversão (a tela ignora) |

> O 2b e o 2A.5 terminam em logout. Faça esta seção **antes** do 2b, ou faça login de novo. Continue logado com o `rafael.carvalho@...`.

- [ ] 2A.1 **Listagem com a API inacessível:** abra `/admin/clientes`. No DevTools, em **Network**, clique com o botão direito numa requisição `GET /api/clientes` e escolha **Block request URL** (ou, no Playwright, `context.route('http://localhost:8080/api/clientes**', r => r.abort('failed'))`). Clique em ">" (próxima página). O alerta vermelho mostra a mensagem de **conexão** da tabela. A tela **não** vai para `/login`, o `auth_user` continua no Local Storage, e não sai `POST /api/auth/logout`. Na Network, o `GET` aparece como `(blocked)`/`net::ERR_FAILED`.
- [ ] 2A.2 Desbloqueie a URL e digite algo em **Buscar**: a lista volta a carregar, sem novo login.
- [ ] 2A.3 **Offline total:** em `/dashboard`, ligue **Network > No throttling > Offline** (ou `context.setOffline(true)`) e clique em "Semana". As 4 requisições falham (`net::ERR_INTERNET_DISCONNECTED`), e o Dashboard mostra a mensagem de **conexão**, sem logout. Volte para "No throttling" (`setOffline(false)`), clique em "Mes": o Dashboard carrega.
- [ ] 2A.4 **Timeout:** nenhuma tela usa `AbortSignal.timeout` hoje, então o timeout é simulado no Console do `/dashboard`:

   ```js
   (() => {
     const orig = window.fetch.bind(window);
     window.fetch = async (input, init) => {
       const u = String(input instanceof Request ? input.url : input);
       if (u.includes("/api/dashboard")) throw new DOMException("signal timed out", "TimeoutError");
       return orig(input, init);
     };
     window.__restaurarFetch = () => { window.fetch = orig; };
   })();
   ```

   Clique em "Semana": o Dashboard mostra **"O servidor demorou para responder. Tente novamente."**, sem logout. Depois rode `__restaurarFetch()`.
- [ ] 2A.5 **Retry do FE-06:** é o item 2.1 (a fila recebe a mensagem de **conexão**, sem logout).
- [ ] 2A.6 **Refresh que falha por rede continua levando a logout:** em `/dashboard`, bloqueie a URL `http://localhost:8080/api/auth/refresh` (Block request URL, ou `route.abort('failed')`), apague o cookie `access_token` e clique em "Semana". Os 4 `GET` voltam 401, o `POST /api/auth/refresh` falha (`net::ERR_FAILED`), **sem** retry do `GET`, e o front chama `POST /api/auth/logout`. Antes do redirect aparece "Sessão expirada. Faça login novamente." (e **não** a mensagem de conexão). A tela vai para `/login?redirect=...`, e o `auth_user` some. Desbloqueie a URL e faça login de novo.

---

## 3. NEG-02: CNPJ alfanumérico

Logado como `rafael.carvalho@...`, em `/admin/clientes`, use **"+ Novo Cliente"**. Anote a razão social de cada cliente criado para excluir no fim (sugestão: prefixo `QA-LOTE5`).

### 3a. Cadastro pela tela

- [ ] 3.1 O campo CNPJ mostra o placeholder `Ex: 12.ABC.345/01DE-35` e a ajuda "Aceita letras e numeros; os 2 ultimos caracteres (DV) sao numericos.". No celular, o teclado é de texto (não numérico).
- [ ] 3.2 Digite `12abc34501de35`. O campo mostra `12.ABC.345/01DE-35` enquanto você digita (maiúsculas e máscara automáticas).
- [ ] 3.3 Preencha os outros campos e clique em **"Criar cliente"**. Salva. Na aba Network, o body do `POST /api/clientes` tem `"cnpj":"12ABC34501DE35"` (sem máscara, em maiúsculas), e a resposta **201** devolve o mesmo valor.
- [ ] 3.4 Na tabela, o cliente aparece com o CNPJ `12.ABC.345/01DE-35`.
- [ ] 3.5 Tente digitar uma letra nas **duas últimas** posições (ex.: `12ABC34501DEA5`). A letra do DV é **ignorada**: o campo fica com `12.ABC.345/01DE-5` (incompleto). Ao salvar, aparece "CNPJ invalido." e **nenhuma** requisição sai (confira a Network).
- [ ] 3.6 Digite `12ABC34501DE36` (DV errado). Ao salvar: "CNPJ invalido." no modal, **sem** requisição.
- [ ] 3.7 Cole `12.abc.345/01de-35` (Ctrl+V) no campo. Ele vira `12.ABC.345/01DE-35`. Caracteres fora de `[0-9A-Z]` (ex.: `#`, `_`, acentos) são descartados ao digitar ou colar.
- [ ] 3.8 Com o `12ABC34501DE35` já cadastrado (3.3), crie outro cliente com `12.abc.345/01de-35`. Resultado: **409** "cnpj já cadastrado" no modal, sem gravar. A resposta não mostra id, razão social nem vendedor do cliente existente.

### 3b. Busca com máscara

No campo **Buscar** de `/admin/clientes` (como admin, para não depender da carteira):

| # | Termo digitado | Esperado |
| --- | --- | --- |
| 3.9 | `12ABC34501DE35` | O cliente do 3.3 aparece. |
| 3.10 | `12abc34501de35` | Aparece (a busca ignora maiúsculas/minúsculas). |
| 3.11 | `12.ABC.345/01DE-35` e `12.abc.345/01de-35` | Aparece. Na Network, o `q` vai como digitado; a API tira a máscara. |
| 3.12 | `12.abc.345` (prefixo com máscara) | Aparece. |

- [ ] 3.9 a 3.12 conferidos.
- [ ] 3.13 Um termo de razão social com hífen (ex.: parte do nome de um cliente que tenha `-`) continua encontrando o cliente pela razão social.

### 3c. Edição

- [ ] 3.14 Edite o cliente do 3.3. O modal abre com `12.ABC.345/01DE-35`. Troque o CNPJ para `rp.5lo.te0/0002-22` e salve: **200**, e a tabela mostra `RP.5LO.TE0/0002-22`.
- [ ] 3.15 Edite um cliente com CNPJ **numérico** de DV válido (ex.: um criado com `11222333000181` ou `11.444.777/0001-61`), mudando só a razão social: **200**. CNPJ numérico continua válido.
- [ ] 3.16 Edite o cliente **9** (CNPJ legado com DV inválido, `29.401.965/5698-16`) mudando só a razão social. Como admin, porque o cliente 9 pode não estar na carteira do Rafael. Com o NEG-02, a **tela** valida o DV antes de enviar: o modal mostra "CNPJ invalido." e **nenhuma** requisição sai. A regra do NEG-04 na API continua valendo: o `PUT /api/clientes/9` direto (item 3.17b) responde **400** "cnpj inválido". Anote o CNPJ original antes, por garantia.

### 3d. API direta (Console, logado como admin)

```js
const criar = (cnpj) => fetch("http://localhost:8080/api/clientes", {
  method: "POST", credentials: "include", headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ cnpj, razao_social: "QA-LOTE5-API " + cnpj, segmento: "Teste", cidade: "Curitiba", uf: "PR", bairro: "Centro" }),
}).then(async (r) => console.log(cnpj, r.status, await r.json()));
await criar("rp.5lo.te0/0001-41");   // 201, cnpj "RP5LOTE0000141"
await criar("RP5LOTE0000141");       // 409 "cnpj já cadastrado"
await criar("12ABC34501DE36");       // 400 "cnpj inválido" (DV)
await criar("12ABC34501DEA5");       // 400 (letra no DV)
await criar("12ABC34501DE3#");       // 400 (caractere inválido)
await criar("12ÁBC34501DE35");       // 400 (acento)
```

- [ ] 3.17 Os status batem com os comentários. No 201, `data.cnpj` = `RP5LOTE0000141` (sem máscara, maiúsculas).
- [ ] 3.17b PUT direto no cliente 9 mantendo o CNPJ legado (a tela não deixa enviar, ver 3.16):

   ```js
   const c9 = (await (await fetch("http://localhost:8080/api/clientes/9", { credentials: "include" })).json()).data;
   const r9 = await fetch("http://localhost:8080/api/clientes/9", {
     method: "PUT", credentials: "include", headers: { "Content-Type": "application/json" },
     body: JSON.stringify({ ...c9, razao_social: c9.razao_social + " QA", data_cadastro: c9.data_cadastro.slice(0, 10) }),
   });
   console.log(r9.status, await r9.json());   // 400 "cnpj inválido"; o banco não muda (confira updated_at)
   ```
- [ ] 3.18 No banco, os valores estão em **maiúsculas** de verdade (a collation `_ci` esconderia a diferença numa comparação comum):

   ```sql
   SELECT cliente_id_origem, BINARY cnpj AS cnpj, razao_social
     FROM clientes WHERE razao_social LIKE 'QA-LOTE5%';
   SELECT COUNT(*) AS minusculas FROM clientes WHERE BINARY cnpj <> UPPER(cnpj);   -- esperado: 0
   ```

### 3e. Exibição em outras telas

- [ ] 3.19 Como admin, em **Vendedores**, abra o vendedor 4 (Rafael) e a lista de clientes da carteira. Um cliente alfanumérico da carteira dele (o do 3.3/3.14, criado pelo Rafael) aparece com a máscara `RP.5LO.TE0/0002-22`.

> **Importador (`importclientes`/`clientesdedup`):** não rode o importador contra o banco local neste roteiro. A regra nova (letras aceitas, caractere inválido recusa a linha em vez de ser removido) é coberta pelos testes automatizados. Conferido pelo TestBrain em 2026-09-25: as 3040 linhas do `dados/crm/clientes.csv` passam na regra nova e na antiga, sem diferença.

---

## 4. DOC-01 e DOC-02: conferência dos documentos

- [ ] 4.1 **DOC-01:** `docs/roteiro-teste-manual-vendedor-desligado.md` usa o `thiago.silva` (id 4, vendedor 3). Confirme:

   ```sql
   SELECT u.id, u.email, u.ativo, u.id_vendedor, v.data_desligamento
     FROM usuarios u JOIN vendedores v ON v.id = u.id_vendedor
    WHERE u.id IN (2, 4, 5);
   ```

   Esperado: id 2 (`henrique.rodrigues`) `ativo` = 0, vendedor 1 desligado em 2025-09-07; id 4 `ativo` = 1, vendedor 3 desligado em 2025-07-26; id 5 `ativo` = 1, vendedor 4 sem desligamento.
- [ ] 4.2 Faça login com o `thiago.silva@...`: o login funciona e as telas da carteira mostram o aviso de vendedor desligado (seções 1 e 2 daquele roteiro).
- [ ] 4.3 **DOC-02:** as consultas da seção "Verificação" de `docs/manual-base-de-dados.md` batem com o banco: 3000 clientes, 0 grupos duplicados, só o `uq_clientes_cnpj` (`Non_unique` = 0), 40 linhas em `clientes_merge_backup_20260925` e 0 em `clientes_merge_backup_20260925_vinculos`.
- [ ] 4.4 **DB-02 (migração 20):** confira o COMMENT e os atributos da coluna:

   ```sql
   SHOW FULL COLUMNS FROM clientes LIKE 'cnpj';
   SHOW INDEX FROM clientes WHERE Column_name = 'cnpj';
   SELECT COUNT(*) FROM clientes;
   ```

   Esperado: `Type` = `char(14)`, `Collation` = `utf8mb4_unicode_ci`, `Null` = `NO`, `Key` = `UNI`, e o `Comment` = "CNPJ normalizado: 14 caracteres, sem máscara, em maiúsculas; 12 primeiras posições em [0-9A-Z] e 2 DVs numéricos (NEG-02)". O COMMENT cita o NEG-02 e descreve o formato alfanumérico pelo conjunto `[0-9A-Z]`, sem usar a palavra "alfanumérico". O índice é só o `uq_clientes_cnpj`, com `Non_unique` = 0, e a contagem continua **3000**. O mesmo texto está em `sql/09_ddl_clientes.sql` (bancos novos).

---

## 5. Limpeza

- [ ] 5.1 Excluir os clientes de teste:

   ```sql
   SELECT cliente_id_origem, cnpj, razao_social FROM clientes
    WHERE razao_social LIKE 'QA-LOTE5%' OR cnpj IN ('12ABC34501DE35','RP5LOTE0000141','RP5LOTE0000222');
   -- confira a lista e então:
   DELETE FROM clientes
    WHERE razao_social LIKE 'QA-LOTE5%' OR cnpj IN ('12ABC34501DE35','RP5LOTE0000141','RP5LOTE0000222');
   SELECT COUNT(*) FROM clientes;   -- 3000 (ou 3000 + os clientes de teste que você decidiu manter)
   SELECT COUNT(*) FROM carteiras;  -- 3637: os vínculos criados pelo Rafael (3.3/3.15) saem por ON DELETE CASCADE
   ```

   O `DELETE` pelo prefixo `QA-LOTE5` também remove o cliente numérico do 3.15. O `AUTO_INCREMENT` de `clientes` não volta, o que é esperado.

- [ ] 5.2 Cookies do navegador de volta ao normal: faça logout e login de novo (o `refresh_token` do 1a/1b foi revogado e o do 2b foi editado).
- [ ] 5.3 `__restaurarFetch()` executado (2a e 2A.4), ou a aba recarregada (F5). URLs bloqueadas no DevTools (2A.1/2A.6) desbloqueadas, e o throttling de volta para "No throttling" (2A.3).
- [ ] 5.4 Cliente 9 com o CNPJ original (não deve ter mudado no 3.16).

---

## Testes automatizados

Rodam com `make test` (Go), `make test-frontend` (Vitest) e, com MySQL local, `INTEGRATION=1`:

```bash
cd apis/rotaperfumes-api && INTEGRATION=1 go test ./handlers/ ./services/ -run Integracao -count=1 -v
```

| Arquivo | O que cobre |
| --- | --- |
| `apis/rotaperfumes-api/services/refresh_token_service_test.go` | SEC-04: janela de graça com relógio injetado (5 s, exatamente 30 s, 31 s, 1 min, ±1 s/31 s no futuro), `Recently` envolve `Revoked`, expiração tem precedência. |
| `apis/rotaperfumes-api/handlers/auth_refresh_logout_test.go` | SEC-04: corrida no `BeginRotation` não conta; 15 revogados na janela sem 429; fora da janela → 429; resposta idêntica dentro e fora; forjados intercalados com revogados recentes ainda levam a 429. |
| `apis/rotaperfumes-api/handlers/sec04_rate_limit_matriz_test.go` | SEC-04 (TestBrain): matriz "conta / não conta" (não encontrado, expirado, expirado + revogado recente, revogado há 31 s/1 h, `revoked_at` 2 min no futuro contam; revogado agora, há 25 s e 10 s no futuro não contam), sempre sem emitir token. |
| `apis/rotaperfumes-api/handlers/lote4_http_integration_test.go` | SEC-02/SEC-04 com MySQL: 3 rodadas de 15 refresh paralelos (1×200, 14×401, nenhum 429, nenhum token nos 401, a aba vencedora segue). NEG-01/NEG-04 inalterados. |
| `apis/rotaperfumes-api/handlers/lote5_http_integration_test.go` | TestBrain, com MySQL. **SEC-04:** reuso sequencial do token recém-rotacionado (`revoked_at` gravado pela API), revogado há 5 s e 25 s pelo relógio do banco (sem 429), fora da janela (2 min) com o mesmo corpo e 429 na 11ª. **NEG-02:** criar com máscara em minúsculas (201, grava maiúsculas, conferido com `BINARY`), busca `?q=` sem máscara, minúsculas, com máscara, prefixo e trecho; 409 em outra caixa/máscara; 400 para DV errado, letra no DV e símbolo; PUT em minúsculas mantém maiúsculas. |
| `apis/shared/cnpj/cnpj_test.go` | Pacote central: normalização, formato, DV, exemplo oficial. |
| `apis/shared/cnpj/testdata/casos_cruzados.json` | **Massa comum back x front** (147 casos + 4 divergências), gerada por uma 3ª implementação de referência do DV. Inclui `12ABC34501DE35` e `11222333000181`. |
| `apis/shared/cnpj/cruzado_test.go`, `apis/rotaperfumes-api/services/cnpj_cruzado_internal_test.go` | A massa comum no pacote Go e na camada de serviço (fluxo de Create/Update) e o `termoBuscaCNPJ` (busca com máscara, minúsculas e prefixo). |
| `frontend/src/lib/cnpj.cruzado.test.ts` | A mesma massa no util TS: validade, valor enviado, fluxo da tela (digitar → máscara → validar → enviar) e `formatCnpj`. As 4 divergências estão como `it.fails` (ver "Divergências" abaixo). |
| `apis/shared/cmd/importclientes/cnpj_cruzado_test.go` | Importador com a massa comum: válido → mesmo valor da API; formato certo com DV errado é importado; formato inválido e divergências recusados; variantes de máscara/caixa unificam no `clientesdedup`. |
| `apis/shared/cmd/importclientes/main_test.go`, `cmd/internal/clientesdedup/clientesdedup_test.go`, `repositories/cliente_repository_test.go`, `services/cliente_service_test.go` | NEG-02: `parseRow` alfanumérico, dedup ignorando máscara e caixa, `QCNPJ` no LIKE de `cnpj`. |
| `frontend/src/lib/cnpj.test.ts`, `components/admin/ClienteModal.cnpj.test.tsx`, `app/admin/clientes/page.test.tsx`, `components/admin/FormModais.test.tsx` | NEG-02 na tela: máscara progressiva, letra no DV ignorada, DV errado sem requisição, envio normalizado, 409, exibição na tabela e busca enviada como digitada. |
| `frontend/src/lib/apiClient.test.ts` | FE-06: falha de rede no retry → a fila recebe o **mesmo** erro, sem logout (inclusive rejeição não-`Error`); 401 → retry 401 → logout e "Sessão expirada". TestBrain: fila com 3 pendentes recebendo o mesmo objeto de erro; refresh 401+401, 429, 500, timeout e erro de rede → logout e "Sessão expirada" na fila toda; depois da falha de rede, um novo 401 dispara um novo refresh. |

---

## Execução 2026-09-25 (TestBrain), só automatizada

A execução no navegador está na seção "Execução 2026-09-25 (TestBrain, Playwright)", abaixo. Resultado das suítes na regressão do lote:

| Suíte | Resultado |
| --- | --- |
| Go `apis/shared` | `go build` e `go vet` sem erros; `go test ./...` ok; cobertura total 49,5% (os `cmd/import*` puxam para baixo; `cnpj` 97,4%, `clientesdedup` 100%, `repositories` 84,5%). |
| Go `apis/rotaperfumes-api` | `go build` e `go vet` sem erros; `go test ./...` ok; cobertura total 88,5% (`handlers` 87,9%, `services` 94,7%). |
| Integração (`INTEGRATION=1`, MySQL local) | Todos os testes `Integracao*` de `handlers` e `services` passam, incluindo o Lote 5. |
| Frontend | `tsc --noEmit` e `eslint` sem erros; `vitest run --coverage` com todos os testes passando (4 `expected fail` das divergências NEG-02); cobertura de linhas acima de 80%. |

**Divergências conhecidas (NEG-02, reportadas ao time):**

- **D1:** o util TS remove qualquer espaço em branco (`\s`: tab, quebra de linha, NBSP, espaço em), e o pacote Go remove só o espaço comum. Um CNPJ com tab ou NBSP **no meio** é válido no front e inválido na API. Na tela isso não chega à API, porque o input sanitiza o valor.
- **D2:** `toUpperCase()` no TS transforma `ı` (i sem ponto, U+0131) em `I`, então o front aceita `12ABı34501DE42`, e a API recusa.

---

## Execução 2026-09-25 (TestBrain, Playwright)

Executado contra `localhost:3000` (`next dev`) e `localhost:8080` (`__debug_bin`, iniciado às 20:16 com o código do lote), das 20:22 às 20:45 (UTC-3). Foi usado o Chrome real com `--remote-debugging-port` (9321 admin, 9322 `rafael.carvalho`, 9323 `thiago.silva`), com login manual por causa do Turnstile e automação por `connectOverCDP`. Os logouts provocados de propósito (2b e 2A.6) foram interceptados com `route.fulfill`, para não revogar a sessão real; o `POST /api/auth/logout` foi disparado pelo front e aparece na Network. Depois, os cookies e o `auth_user` foram restaurados. A API ainda roda o código novo: o 1b (12 reusos depois de 14 falhas no 1a) não deu 429, e o 3.3 aceitou CNPJ com letras.

**Nenhum defeito funcional do Lote 5.** Um bug **fora do escopo** foi encontrado na tela de Vendedores (B1, abaixo).

| Seção | Resultado |
| --- | --- |
| 0. Preparação | OK. Os 3 CNPJs alfanuméricos não existiam, e o `11222333000181` também não. Base: 3000 clientes, 3637 carteiras, 0 CNPJ em minúsculas. |
| 1a. 15 refresh em paralelo | 1.1 OK: 1×200, 14×401 (`"refresh token revogado"`, sem `Set-Cookie`), nenhum 429. 1.3 OK: 1 par novo. 1.2 **NÃO EXECUTADO**: API no debugger do VS Code, sem acesso ao log pelo shell. |
| 1b. Reuso na janela de graça | 1.4 e 1.5 OK: 12×401 com o mesmo corpo, nenhum 429, nenhum `Set-Cookie`, nenhum `Retry-After`. |
| 1d. Cinco abas | 1.9 a 1.11 OK. Quatro rodadas com o `access_token` apagado e os 5 cliques disparados em menos de 25 ms: 18 `POST /api/auth/refresh`, todos 200 (Web Locks serializando), nenhum 429, nenhum logout, e todas as abas avançaram de página (a de clientes do Rafael só tem 3 páginas, então ficou fora das rodadas 3 e 4). |
| 1c. Reuso fora da janela | Feito depois do 1d, 115 s após o 1b. 1.6 OK: 10×401 com o corpo idêntico ao do 1.4. 1.7 OK: a 11ª deu 429 com `Retry-After: 299`. 1.8 **NÃO EXECUTADO** (log, mesmo motivo do 1.2). A API **não** foi reiniciada: esperei os 5 minutos do bloqueio antes da seção 2. |
| 2a. FE-06 | 2.1 OK, já com a mensagem do FE-08 ("Não foi possível conectar ao servidor..."). 2.2 OK: continuou em `/dashboard`, com o `auth_user` presente. 2.3 OK: nenhum `/api/auth/*` real. 2.4 OK: depois do `__restaurarFetch()`, 4×200. |
| 2b. Controle | 2.5 OK: 4 `GET` 401, 1 refresh 401, 1 `GET` repetido 401 e `POST /api/auth/logout`. 2.6 OK: foi para `/login?redirect=...`; a mensagem não chegou a aparecer antes do redirect, o que o item permite. 2.7 OK: `auth_user` removido. |
| 2A. FE-08 | 2A.1 OK (`route.abort('failed')`: `net::ERR_FAILED` e mensagem de conexão, sem logout; ver O2). 2A.2 OK. 2A.3 OK (`context.setOffline(true)`: 4× `ERR_INTERNET_DISCONNECTED`, mensagem de conexão; de volta online, 4×200). 2A.4 OK ("O servidor demorou para responder. Tente novamente."). 2A.5 = 2.1. 2A.6 OK (refresh `ERR_FAILED`, sem retry do `GET`, "Sessão expirada. Faça login novamente." antes do redirect, `/login`, `auth_user` removido). |
| 3a. Cadastro pela tela | 3.1 a 3.8 OK. Placeholder e ajuda corretos, `inputmode="text"`, `autocapitalize="characters"`. A máscara progressiva vai de `1` a `12.ABC.345/01DE-35`. O POST enviou `"cnpj":"12ABC34501DE35"`, e o 201 devolveu o mesmo valor. Letra no DV: `12.ABC.345/01DE-5`, "CNPJ invalido.", 0 requisições. DV errado: 0 requisições. Colar pela área de transferência resultou em `12.ABC.345/01DE-35`, e `1#2_Áé.a b` digitado virou `12.AB`. Duplicado em minúsculas com máscara: 409 `"cnpj já cadastrado"`, sem dados do existente. |
| 3b. Busca (admin) | 3.9 a 3.12 OK: o `q` saiu como digitado, e a API devolveu 1 resultado em todas as variantes. 3.13 OK com `QA-LOTE5` e `A-LOTE5 Tela`: a base não tem outra razão social com hífen, então foi usado o cliente de teste. |
| 3c. Edição | 3.14 OK: o modal abriu com `12.ABC.345/01DE-35`, o PUT enviou `RP5LOTE0000222` e voltou 200, e a tabela mostrou `RP.5LO.TE0/0002-22`. 3.15 OK: cliente numérico `11222333000181` criado e editado só na razão social, 200. 3.16 **divergência D1**: a tela barra antes (sem requisição); o 400 foi conferido na API (3.17b). |
| 3d. API direta | 3.17 OK: 201 (`RP5LOTE0000141`), 409, e 400 para DV errado, letra no DV, `#` e acento. 3.17b OK: `PUT /api/clientes/9` = 400 `"cnpj inválido"`, com `updated_at` inalterado. 3.18 OK: `BINARY cnpj` em maiúsculas, 0 CNPJ em minúsculas. Observação O1 sobre o corpo do 201. |
| 3e. Outras telas | 3.19 OK: no modal do vendedor 4, `#3216 QA-LOTE5 Tela 1` aparece com `RP.5LO.TE0/0002-22`, e o numérico com `11.222.333/0001-81`. Ao abrir o modal, foi encontrado o **B1**. |
| 4. DOC-01/DOC-02 | 4.1 OK (ids 2, 4 e 5 como esperado). 4.2 OK: `thiago.silva` logou, o `/me` trouxe `vendedor_desligado: true`, o Dashboard mostrou o aviso, as 5 URLs da carteira mostraram o aviso e o link "Ir para o Dashboard", sem tabela e sem refresh/logout, só `GET /api/auth/me`. 4.3 OK: 3000 clientes, 0 duplicados, só `uq_clientes_cnpj` com `Non_unique` = 0, 40 linhas no backup e 0 nos vínculos. |
| 4.4 DB-02 | OK: `char(14)`, `utf8mb4_unicode_ci`, `NOT NULL`, `UNI`; o COMMENT é o da migração 20 e cita o NEG-02 e `[0-9A-Z]`; só `uq_clientes_cnpj`; 3000 clientes. Observação O3: a palavra "alfanumérico" não aparece no COMMENT. |
| 5. Limpeza | 5.1 OK: 3 clientes apagados (3216, 3218 e 3219; o 3217 foi o id consumido pelo INSERT recusado no 409). Voltou a 3000 clientes e 3637 carteiras (CASCADE), com 0 resíduos (`QA-%`, CNPJ com letras e `11222333000181`). Pedidos 28732, oportunidades 5980 e visitas 37936, iguais ao Lote 4. 5.2 OK: logout nas 3 sessões, 0 refresh token ativo das últimas 2 h, janelas fechadas. 5.3 OK. 5.4 OK: cliente 9 com `29401965569816`, "Perfumaria Sublime S/A", `updated_at` 18:23:40 inalterado. |

**Divergências do roteiro (texto já ajustado acima):**

- **D1 (3.16):** o roteiro esperava 400 pela tela. Com o NEG-02, o `ClienteModal` valida o DV antes de enviar (`validateCnpj`), então a tela mostra "CNPJ invalido." e **não** faz o `PUT`. O 400 do NEG-04 continua na API (3.17b). O resultado final é correto (o legado não é regravado).
- **D2 (2.1):** o texto "Failed to fetch" foi trocado pela mensagem do FE-08.
- **D3 (1c):** a API não foi reiniciada (ela roda no debugger do VS Code); o bloqueio de 5 minutos foi esperado. O Node usou `::1`, e o bloqueio vale por IP (`refresh-ip:<ip>`).

**Bug fora do escopo do lote (encaminhar ao SubBrain):**

- **B1 (tela de Vendedores, grave: perda de dado ao salvar):** o modal "Editar Vendedor" abre com **Data de admissao = hoje** e **Meta mensal = 0**, e não com os valores do banco (vendedor 4: `2023-06-22` e `55000.00`). Clicar em "Salvar alteracoes", mesmo sem mudar nada, envia `{"nome":"Rafael Carvalho","regiao":"Manaus","uf":"AM","data_admissao":"2026-09-25","meta_mensal":0}`. O corpo foi capturado com o `PUT` interceptado, e nada foi gravado. Causa: `frontend/src/app/admin/vendedores/page.tsx:196-211` passa `data_admissao: ""` e `meta_mensal: 0` como fallback, porque o `GET /api/vendedores` (projeção do SEC-03) não traz esses campos. O comentário diz que o modal busca o detalhe para preenchê-los, mas `frontend/src/components/admin/VendedorModal.tsx:143-152` inicializa o formulário só a partir desse fallback (`todayISO()` / `0`), e o efeito de `:174-183` usa o `GET /api/vendedores/{id}` só para `detalhe.clientes`. Reproduzir: como admin, em Vendedores, "Editar" em qualquer vendedor. Pré-existente: não está no diff do Lote 5, que só trocou o `fmtCnpj` por `formatCnpj` nesse arquivo.

**Observações (sem bloqueio):**

- **O1:** o 201 do `POST /api/clientes` devolve `created_at`/`updated_at` = `0001-01-01T00:00:00Z` e, sem `data_cadastro` no body, `data_cadastro` com nanossegundos (`2026-09-25T20:29:12.7350372-03:00`). `CreateCliente` e `CreateClienteNaCarteira` (`apis/rotaperfumes-api/services/cliente_service.go:216-228` e `:238-271`) devolvem o struct em memória, sem reler do banco. O PUT devolve os valores certos. Pré-existente.
- **O2:** com erro de rede em `/admin/clientes`, `aplicarErroClientes` (`frontend/src/app/admin/clientes/page.tsx:152-162`) zera a lista, e a tela mostra "0 clientes / Nenhum cliente cadastrado." junto com o alerta de conexão. A mensagem é a certa, mas o vazio sugere que não há clientes. Pré-existente, é UX.
- **O3:** o COMMENT do DB-02 descreve o formato alfanumérico por `[0-9A-Z]`, sem a palavra "alfanumérico". Aceitável se o card não exigir o termo literal.

**Evidências:** screenshots em `C:\Users\ivoce\AppData\Local\Temp\claude\c--Users-ivoce-OneDrive-Documentos-golang-SistemaCompleto\1baaf4ea-771e-4d6f-b13e-4f4e89264130\scratchpad\shots\` (temporários da sessão).
