# Roteiro de teste manual: Lote 6 (SEC-06, SEC-07, BUG-08, FE-07, FE-09, TEST-01, DOC-03)

**Cards:** SEC-06 (usuário inativado perde a sessão na hora; role vem do banco), SEC-07 (motivo da revogação do refresh token e revogação em massa no reuso de token rotacionado), BUG-08 (`POST /api/clientes` devolve os timestamps), FE-07 (falha de rede no refresh não desloga), FE-09 (listagem com erro não mostra "Nenhum ... cadastrado"), TEST-01 (nomes de teste) e DOC-03 (documentos e Postman com usuário ativo e senha de seed). Detalhes em `tarefas/fazendo.md`.
**Migração:** a 21 (`sql/21_alter_refresh_tokens_revoked_reason.sql`, `make db-fix-revoked-reason`), que cria `refresh_tokens.revoked_reason`. Já aplicada no banco local em 2026-09-26. Conferência no item 5.1.
**Tempo estimado:** 30 a 40 minutos.
**Autor:** SubBrain (2026-09-26)

Marque cada checkbox depois de conferir. Nas partes de navegador, deixe o DevTools aberto na aba **Network**. Se algo divergir, anote o caso e encaminhe ao SubBrain.

> **Mudança em relação ao Lote 5:** o item 2A.6 de `docs/roteiro-teste-manual-lote5.md` ("refresh que falha por rede continua levando a logout") deixou de valer com o FE-07. O comportamento novo está na seção 3 deste roteiro.

---

## 0. Preparação

1. **Reinicie a API** com o código do Lote 6 (`make dev-api`, ou reinicie a sessão de debug do VS Code) e suba o frontend (`make dev-frontend`). O binário antigo não confere `usuarios.ativo` a cada requisição e não grava `revoked_reason`.
2. Usuários:

   | Perfil | Usuário (e-mail) | id | Senha |
   | --- | --- | --- | --- |
   | admin | `admin@rotaperfumes.com.br` | 1 | a sua |
   | normal com vendedor ativo | `rafael.carvalho@rotaperfumes.com.br` | 5 | a de seed (`SEED_USER_PASSWORD` do `.env` ou a impressa pelo `make db-seed`), ou a que você definiu |
   | **usuário de teste** (SEC-06) | `qa.lote6@rotaperfumes.test` | criado abaixo | `QaLote6@2026` |

3. Crie o usuário de teste como **admin** (no terminal, na pasta `apis/shared`):

   ```bash
   go run ./cmd/resetpassword -email qa.lote6@rotaperfumes.test -password 'QaLote6@2026' -role admin -nome "QA Lote 6"
   ```

   Confira: `SELECT id, email, role, ativo, deve_trocar_senha FROM usuarios WHERE email = 'qa.lote6@rotaperfumes.test';` Anote o **id** (chamado de `<QA>` abaixo). Se `deve_trocar_senha` = 1, o primeiro login leva para `/trocar-senha`: troque a senha e anote a nova.

4. **Snapshot para a restauração (seção 6):**

   ```sql
   SELECT (SELECT COUNT(*) FROM clientes)  AS clientes,    -- esperado: 3000
          (SELECT COUNT(*) FROM carteiras) AS carteiras,   -- esperado: 3637
          (SELECT COUNT(*) FROM usuarios)  AS usuarios;
   ```

   Anote os três valores.

---

## 1. SEC-06: inativação e rebaixamento de role valem sem novo login

Use duas janelas: **A** (normal) com o admin e **B** (anônima) com o `qa.lote6@...`.

### 1a. Rebaixamento de role

1. Na janela B, faça login com o `qa.lote6@...` e abra `/admin/usuarios`. A lista carrega (ele é admin).
2. Na janela A, em **Usuários**, edite o "QA Lote 6" e troque a role para **normal**. Salve.
3. Na janela B, **sem sair**, clique em ">" (próxima página) ou recarregue `/admin/usuarios`.

   - [ ] 1.1 O `GET /api/usuarios` responde **403** na Network. O token da janela B ainda diz `admin`, mas a API usa a role do banco.
   - [ ] 1.2 A janela B **não** foi deslogada (a sessão continua; só o acesso de admin caiu). Anote o que a tela mostra.

4. Na janela A, volte a role do "QA Lote 6" para **admin**. Na janela B, repita o passo 3.

   - [ ] 1.3 A lista volta a carregar (**200**), também sem novo login.

### 1b. Inativação

1. Na janela A, em **Usuários**, **inative** o "QA Lote 6".
2. Na janela B, clique em qualquer ação que chame a API (ex.: ">" na lista, ou abrir `/dashboard`).

   - [ ] 1.4 Na Network: o `GET` responde **401** `{"success":false,"error":"usuário inativo"}`, o `POST /api/auth/refresh` responde **401** `"refresh token revogado"` (os refresh tokens foram revogados na inativação) e o front chama `POST /api/auth/logout`.
   - [ ] 1.5 A janela B vai para `/login`, e o `auth_user` some do Local Storage.
   - [ ] 1.6 Novo login com o `qa.lote6@...` é recusado com **"usuário inativo"**.
   - [ ] 1.7 No banco, os tokens do usuário foram revogados com o motivo certo:

     ```sql
     SELECT id, revoked_at, revoked_reason FROM refresh_tokens
      WHERE usuario_id = <QA> ORDER BY id DESC LIMIT 5;
     -- nenhum token com revoked_at NULL; o(s) que estava(m) ativo(s) com revoked_reason = 'inativacao'
     ```

3. Na janela A, **reative** o "QA Lote 6" (ele será usado de novo na seção 5 e apagado na 6).

> **Limitação conhecida (card SEC-08):** o access token antigo (TTL de 24h) volta a valer se o usuário for reativado antes de expirar. Não é defeito deste lote. O desligamento de vendedor (`DELETE /api/vendedores/{id}`, que inativa a sessão dos usuários dele pelo mesmo mecanismo) está coberto pelos testes de integração e fica fora deste roteiro para não mexer em vendedor real.

---

## 2. FE-09: listagem com erro não mostra "Nenhum ... cadastrado"

Logado como admin, pare a API (Ctrl+C no `make dev-api`, ou pare o debug) e abra cada tela pela URL (F5), para que a **primeira carga** falhe:

| # | Tela | Mensagem de vazio que **não** pode aparecer |
| --- | --- | --- |
| 2.1 | `/admin/clientes` | "Nenhum cliente cadastrado." |
| 2.2 | `/admin/vendedores` | estado vazio de vendedores |
| 2.3 | `/admin/usuarios` | estado vazio de usuários |
| 2.4 | `/admin/produtos` | estado vazio de produtos |
| 2.5 | `/admin/senha-historico` | estado vazio do histórico |
| 2.6 | `/admin/estoque` | estado vazio de estoque |
| 2.7 | `/admin/pedidos` | estado vazio de pedidos |
| 2.8 | `/admin/oportunidades` | estado vazio de oportunidades |
| 2.9 | `/admin/visitas` | estado vazio de visitas |
| 2.10 | `/pagamentos` | estado vazio de pagamentos |

- [ ] 2.1 a 2.10 Cada tela mostra **só** o alerta vermelho de conexão ("Não foi possível conectar ao servidor..."), sem a mensagem de vazio da tabela.
- [ ] 2.11 `/dashboard` com a API parada: aparece o erro, e os estados vazios ("Nenhum dado ...") ficam ocultos.
- [ ] 2.12 Suba a API de novo e recarregue uma das telas: a lista volta normalmente.

> **Comportamentos antigos, fora do card (card FE-10):** o contador do cabeçalho pode mostrar "0 clientes" com a primeira carga falhando; numa troca de página que falha, a tabela mantém as linhas da página anterior, mas o indicador mostra a página nova; em Vendedores, o alerta de erro pode aparecer junto com a mensagem de sucesso se a recarga depois de inativar falhar. Anote se vir, mas não conta como falha deste lote.

---

## 3. FE-07: falha de rede no refresh não desloga

1. Logado com o `rafael.carvalho@...`, abra `/dashboard`.
2. No DevTools, **Network**, bloqueie a URL `http://localhost:8080/api/auth/refresh` (botão direito numa requisição > **Block request URL**, ou em **More tools > Network request blocking**).
3. Em **Application > Cookies**, apague só o cookie `access_token` (mantenha o `refresh_token`).
4. Clique em "Semana".

   - [ ] 3.1 Os `GET` voltam **401**, o `POST /api/auth/refresh` aparece como `(blocked)`/`net::ERR_FAILED`, e **não** sai `POST /api/auth/logout`.
   - [ ] 3.2 A tela mostra a mensagem de **conexão** ("Não foi possível conectar ao servidor..."), e **não** "Sessão expirada. Faça login novamente.".
   - [ ] 3.3 A tela **não** vai para `/login`, e o `auth_user` continua no Local Storage.

5. Desbloqueie a URL e clique em "Mes".

   - [ ] 3.4 O refresh responde **200**, os `GET` são repetidos com 200, e o Dashboard carrega sem novo login (o refresh token não foi revogado).

6. **Controle:** apague o `access_token` de novo, **edite** o `refresh_token` para `invalido` e clique em "Semana".

   - [ ] 3.5 O refresh responde **401**, o front chama `POST /api/auth/logout` e vai para `/login`. `401`, `429` e `500` no refresh continuam deslogando. Faça login de novo.

---

## 4. BUG-08: `POST /api/clientes` traz os timestamps

1. No Postman, importe de novo o `postman/collection.json`. Preencha a variável `senha_vendedor` (DOC-03) e rode **Login** (admin).
2. Rode **Clientes > Criar Cliente** com a razão social `QA-LOTE6 Timestamps` e o CNPJ `11222333000181` (se ele já existir, `409`: use `11444777000161`).

   - [ ] 4.1 A resposta é **201**, e `created_at` e `updated_at` vêm com a data e a hora de agora, e **não** `0001-01-01T00:00:00Z`.
   - [ ] 4.2 Rode **Login — Vendedor** (`rafael.carvalho`, com `{{senha_vendedor}}`) e crie outro cliente com o `vendedor_token` (razão social `QA-LOTE6 Carteira`, CNPJ `RP5LOTE0000141`). Resposta **201** com os timestamps preenchidos (caminho `CreateClienteNaCarteira`).
   - [ ] 4.3 **DOC-03:** o "Login — Vendedor" passa com `trocar_senha` `true` **ou** `false` (o teste só confere que é booleano).

---

## 5. SEC-07: `revoked_reason` depois de login, refresh e logout

Use o `qa.lote6@...` (reativado no fim da seção 1). As queries usam `<QA>`.

### 5a. Coluna e dados legados

- [ ] 5.1 Migração 21 aplicada:

  ```sql
  SHOW FULL COLUMNS FROM refresh_tokens LIKE 'revoked_reason';
  -- Type = enum('rotacao','logout','revogacao_massa','senha','inativacao'), Null = YES, Default = NULL
  SELECT revoked_reason, COUNT(*) FROM refresh_tokens GROUP BY revoked_reason;
  -- os tokens de antes da migração ficam em NULL
  ```

### 5b. Login, refresh e logout pelo navegador

1. Faça login com o `qa.lote6@...` numa janela anônima.

   - [ ] 5.2 `SELECT id, revoked_at, revoked_reason FROM refresh_tokens WHERE usuario_id = <QA> ORDER BY id DESC LIMIT 3;` O token mais novo está com `revoked_at` e `revoked_reason` **NULL** (ativo).

2. Apague o cookie `access_token` e clique em qualquer ação (dispara um refresh 200).

   - [ ] 5.3 A mesma query: o token anterior ficou com `revoked_reason = 'rotacao'`, e há um novo ativo (NULL/NULL).

3. Clique em **Sair**.

   - [ ] 5.4 O token que estava ativo ficou com `revoked_reason = 'logout'`. Nenhum token do `<QA>` com `revoked_at` NULL.

### 5c. Reuso de token (terminal)

1. Faça login de novo com o `qa.lote6@...` e copie o cookie `refresh_token` (**Application > Cookies**). No Git Bash:

   ```bash
   RT1='<cole o refresh_token aqui>'
   curl -s -i -X POST http://localhost:8080/api/auth/refresh \
     -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$RT1\"}" | grep -i -E 'HTTP/|set-cookie: refresh_token'
   ```

   A resposta é **200** e o `RT1` passa a `rotacao`.

2. **Espere pelo menos 35 segundos** e reuse o `RT1`:

   ```bash
   curl -s -w ' <- %{http_code}\n' -X POST http://localhost:8080/api/auth/refresh \
     -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$RT1\"}"
   ```

   - [ ] 5.5 Resposta **401** `{"success":false,"error":"refresh token revogado"}`, o mesmo corpo de sempre.
   - [ ] 5.6 No log da API (terminal do `make dev-api` ou Debug Console): `[auth][seguranca] refresh: reuso de refresh token já rotacionado fora da janela de graça (possível roubo de token) — revogando todas as sessões: user_id=<QA> token_id=... ip=... ua=curl/...`.
   - [ ] 5.7 Todas as sessões do usuário foram revogadas: `SELECT revoked_reason, COUNT(*) FROM refresh_tokens WHERE usuario_id = <QA> GROUP BY revoked_reason;` mostra `revogacao_massa` (o token emitido no passo 1), e nenhum token do `<QA>` fica com `revoked_at` NULL. A janela do `qa.lote6` continua até o access token dela precisar de refresh; aí vai para `/login`.

3. **Controle (token de logout não alerta):** faça login de novo com o `qa.lote6@...`, copie o `refresh_token` como `RT2`, clique em **Sair** e, depois de **35 segundos**, rode o `curl` do passo 2 com o `RT2`.

   - [ ] 5.8 Resposta **401** com o mesmo corpo. No log aparece só `token revogado apresentado fora da janela de graça: user_id=<QA> token_id=... motivo=logout`, **sem** `[auth][seguranca]`, e nenhum token passa a `revogacao_massa`.

> Os reusos fora da janela contam no rate limit do refresh (10 falhas por IP em 1 minuto). Este roteiro faz só 2; se você repetir muitas vezes e receber **429**, espere 5 minutos ou reinicie a API.

---

## 6. Restauração do banco

- [ ] 6.1 Apagar os clientes de teste (as carteiras saem por `ON DELETE CASCADE`):

  ```sql
  SELECT cliente_id_origem, cnpj, razao_social FROM clientes WHERE razao_social LIKE 'QA-LOTE6%';
  -- confira a lista e então:
  DELETE FROM clientes WHERE razao_social LIKE 'QA-LOTE6%';
  ```

- [ ] 6.2 Apagar o usuário de teste (os refresh tokens e o `senha_historico` dele saem por `ON DELETE CASCADE`):

  ```sql
  DELETE FROM usuarios WHERE email = 'qa.lote6@rotaperfumes.test';
  ```

- [ ] 6.3 Contagens iguais às do snapshot do passo 0.4 (clientes 3000, carteiras 3637 e usuários como anotado). O `AUTO_INCREMENT` não volta, o que é esperado.
- [ ] 6.4 Role do "QA Lote 6" não importa mais (apagado). Confira que nenhum outro usuário teve a role ou o `ativo` alterado: `SELECT id, email, role, ativo FROM usuarios WHERE updated_at > NOW() - INTERVAL 2 HOUR;` volta vazio (ou só o que você mudou de propósito).
- [ ] 6.5 No DevTools, URLs desbloqueadas (seção 3) e cookies de volta ao normal: faça logout e login de novo com o `rafael.carvalho@...` (o `refresh_token` do 3.5 foi editado).

---

## Testes automatizados

Rodam com `make test` (Go), `make test-frontend` (Vitest) e, com MySQL local, `INTEGRATION=1`:

```bash
cd apis/rotaperfumes-api && INTEGRATION=1 go test ./handlers/ ./services/ -count=1
```

| Arquivo | O que cobre |
| --- | --- |
| `apis/rotaperfumes-api/handlers/lote6_http_integration_test.go` | Com MySQL. **SEC-06:** inativar usuário (401 "usuário inativo" nas rotas protegidas e tokens revogados com `inativacao`), desligar vendedor (revogação na mesma transação), rebaixamento de role e usuário inexistente. **SEC-07:** reuso de token com `rotacao` fora da janela (alerta e `revogacao_massa`) e de token de logout (sem revogação em massa). **BUG-08:** timestamps no 201, com e sem carteira. |
| `apis/rotaperfumes-api/middleware/user_status_test.go` e `user_status_cookie_test.go` | SEC-06: `JWTMiddlewareWithUserCheck` com cookie e Bearer; inativo/inexistente → 401; erro de banco → 500; role do banco substitui a do token. |
| `apis/rotaperfumes-api/services/refresh_token_service_test.go`, `refresh_rotation_test.go`, `apis/shared/repositories/refresh_token_repository_test.go` | SEC-07: `RevokedTokenError` com motivo, `IsRotationReuse`, gravação de `revoked_reason` em todas as revogações. |
| `frontend/src/lib/apiClient.lote6.test.ts` | FE-07: falha de rede, timeout e falha do Web Lock no refresh → `NetworkError`, sem logout e sem `/api/auth/logout`; a fila recebe o mesmo erro; 401/429/500 continuam deslogando. |
| `frontend/src/components/ui/Table.test.tsx`, `frontend/src/app/admin/listasAdmin.test.tsx` | FE-09: `erroCarga` oculta o estado vazio nas 10 listagens. |
| `apis/rotaperfumes-api/handlers/lote4_http_integration_test.go:254`, `apis/shared/cnpj/cruzado_test.go:75-79` | TEST-01: nome do subteste e comentário atualizados. |

**Regressão do 🔴 TestBrain (2026-09-26):** Go ok (unit e `INTEGRATION=1`: 1202 PASS em `apis/shared` e 2354 em `rotaperfumes-api`, 0 FAIL). Frontend: `tsc`, `eslint` e `vitest` 1188/1188. Cobertura: `rotaperfumes-api` 88,6%, `apiClient.ts` 94,81%, `Table.tsx` 100% das linhas. Banco restaurado e idêntico ao snapshot.
