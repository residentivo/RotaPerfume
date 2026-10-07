# Roteiro de teste manual: Lote 12 (SEC-11, SEC-12, CHORE-02, DB-01)

**Cards:** SEC-11 (credenciais do banco obrigatórias, sem default `golang/golang`, senha fora do argv via `MYSQL_PWD`), SEC-12 (reenvio de refresh rotacionado não derruba as sessões de novo dentro da janela de supressão), CHORE-02 (limpeza periódica de `refresh_tokens`) e DB-01 (`estoque` no `db-up`, migração 18 idempotente com reversão). Detalhes em `tarefas/feito.md` (Lote 12) e em `docs/manual-base-de-dados.md` (seções 7.1, 7.2, 17 e 23).
**Migrações:** 23 (`refresh_tokens.reuso_detectado_em`, já aplicada no banco de dev) e 18 (agora idempotente, só para bancos antigos).
**Sem mudança de contrato HTTP.**
**Tempo estimado:** 35 a 50 minutos (a seção 3 apaga e recria o banco de dev).
**Autor:** SubBrain (2026-09-27)

Marque cada checkbox depois de conferir. Se algo divergir, anote o caso e encaminhe ao SubBrain.

---

## 0. Preparação

1. Salve uma cópia do `.env` da raiz (ex.: `cp .env .env.bak`). Vários passos editam o arquivo; restaure com `cp .env.bak .env` ao final de cada um.
2. Confira que o `.env` tem `DB_USUARIO` e `DB_SENHA` preenchidos, sem `$`, `#`, aspas, espaços nas pontas nem valor começando com `/` (vale para **todos** os valores do `.env`, porque o `make` lê o arquivo inteiro), e que está salvo com final de linha **LF**.
3. Garanta que o shell não tem as variáveis do banco exportadas (senão elas mascaram o teste de "sem credenciais"): no Git Bash, `[ -z "$DB_USUARIO$DB_SENHA" ] && echo limpo`. Se não imprimir `limpo`, rode `unset DB_USUARIO DB_SENHA MYSQL_PWD`.
4. Para rodar SQL à mão, use o Workbench ou o `mysql` com a senha pelo ambiente (nunca `-p`):

   ```bash
   MYSQL_PWD="<senha>" mysql -u <usuario> -h localhost -P 3306 rotaperfumes -e "SELECT 1;"
   ```

## 1. SEC-11: credenciais e `make`

1. **Sem credenciais:** no `.env`, comente as linhas `DB_USUARIO` e `DB_SENHA` (`#DB_USUARIO=...`). Na raiz, rode `make db-create`.

   - [ ] 1.1 O `make` para com `SEC-11: defina DB_USUARIO/DB_SENHA no .env` (e `Error 1` no alvo `db-check-env`), **sem** chamar o `mysql` e sem tentar `golang/golang`.
   - [ ] 1.2 O mesmo para `make db-fix-reuso-detectado` e `make db-down` (nada é apagado).
   - [x] 1.3 `make help` e `make lint` funcionam normalmente sem as credenciais. (Passa desde o BUG-13, verificado em 2026-10-07 pelo 🔴 TestBrain no Lote 16.)

   Restaure o `.env`.

2. **Senha fora do eco e do argv:** use uma senha fictícia só na linha de comando (tem precedência sobre o `.env`), para não expor a real:

   ```bash
   make -n db-up DB_SENHA=SENHA_FICTICIA_L12
   make -n db-up DB_SENHA=SENHA_FICTICIA_L12 | grep -c SENHA_FICTICIA_L12
   ```

   - [x] 1.4 As linhas `mysql ...` mostram só `--local-infile=1 -u <usuario> -h localhost -P 3306 --default-character-set=utf8mb4`, **sem** `-p` e sem a senha.
   - [x] 1.5 A contagem do `grep -c` é **0** (a senha não aparece em nenhuma linha, nem na do `db-check-env`, que testa `$$DB_USUARIO`/`$$MYSQL_PWD` do ambiente). Qualquer valor diferente de 0 é **falha**.
   - [x] 1.6 `grep -n -- '-p\$DB_SENHA' sql/*.sql` não encontra nada (os cabeçalhos ensinam `MYSQL_PWD="$DB_SENHA" mysql ...`).

3. **Precedência:** com o `.env` correto, rode `DB_SENHA=errada make db-create`.

   - [ ] 1.7 Funciona (o `.env` ganha da variável do shell). Já `make db-create DB_SENHA=errada` falha com `Access denied` (a linha de comando ganha de tudo).

4. **API sem credenciais:** comente `DB_SENHA` no `.env` e rode `make dev-api`.

   - [x] 1.8 A API não sobe: `[server] config: config: defina DB_USUARIO/DB_SENHA no .env`, sem mostrar valores.
   - [x] 1.9 O mesmo comentando só `DB_USUARIO`. Restaure o `.env`.
   - [x] 1.10 (Opcional) `make db-import-produtos` sem `DB_SENHA` também recusa com a mesma mensagem.

## 2. CHORE-02: limpeza de `refresh_tokens`

1. **Intervalo inválido:** no `.env`, `REFRESH_CLEANUP_INTERVAL=30s`; rode `make dev-api`.

   - [x] 2.1 A API não sobe: `REFRESH_CLEANUP_INTERVAL inválido ("30s"): esperado 0 (desativa) ou >= 1m0s`.
   - [x] 2.2 (Opcional) `REFRESH_TOKEN_RETENCAO=1h` também impede a subida (faixa `24h` a `8760h`).

   Restaure o `.env` (`REFRESH_CLEANUP_INTERVAL=6h`, `REFRESH_TOKEN_RETENCAO=720h`).

2. **Massa para a limpeza:** escolha dois tokens **já revogados** de um usuário de teste e ajuste a expiração:

   ```sql
   SELECT id, usuario_id, expires_at, revoked_reason FROM refresh_tokens
    WHERE revoked_at IS NOT NULL ORDER BY id DESC LIMIT 5;
   UPDATE refresh_tokens SET expires_at = NOW() - INTERVAL 31 DAY WHERE id = <ANTIGO>;
   UPDATE refresh_tokens SET expires_at = NOW() - INTERVAL 1 HOUR  WHERE id = <RECENTE>;
   ```

3. **Partida:** rode `make dev-api`.

   - [x] 2.3 Logo depois de `[server] db ping OK`: `[refresh] cleanup: limpeza periódica iniciada (intervalo=6h0m0s)` e `[refresh] cleanup: N tokens removidos (corte=...)`, com `N >= 1` e `corte` cerca de 30 dias atrás.
   - [x] 2.4 `SELECT id FROM refresh_tokens WHERE id IN (<ANTIGO>, <RECENTE>);` devolve só o `<RECENTE>` (expirado há 1h continua; expirado há 31 dias foi apagado). Nenhum token ativo sumiu.

4. **Shutdown limpo:** no terminal da API, `Ctrl+C`.

   - [x] 2.5 Aparecem, nesta ordem, `[server] shutdown solicitado`, `[refresh] cleanup: limpeza periódica encerrada`, `[server] tarefas de fundo encerradas` e `[server] bye`, sem `panic` e sem `tarefas de fundo não encerraram`.

5. **Desativada:** `REFRESH_CLEANUP_INTERVAL=0` no `.env`, suba a API.

   - [x] 2.6 Log `[refresh] cleanup: limpeza periódica desativada (REFRESH_CLEANUP_INTERVAL=0)` e nenhum `tokens removidos`. Restaure o `.env` e suba a API de novo.

## 3. DB-01: `estoque` no `db-up` e migração 18

> **Atenção:** o `db-reset` apaga o banco de dev. Se precisar dos dados depois, rode `make db-rebuild` (recria e importa todos os CSVs) no lugar do `db-reset`.

1. Pare a API e rode `make db-reset` (ou `make db-rebuild`).

   - [ ] 3.1 O log do `make` mostra `=== Aplicando DDL de estoque ...` depois do de `visitas`, sem erro.
   - [ ] 3.2 `SHOW TABLES LIKE 'estoque';` devolve a tabela, e `SHOW COLUMNS FROM estoque LIKE 'origem';` vem **vazio**.
   - [ ] 3.3 `SHOW COLUMNS FROM refresh_tokens LIKE 'reuso_detectado_em';` existe (vem do `06_ddl_refresh_tokens.sql`).
   - [ ] 3.4 (Se usou `db-rebuild`) o `db-import-estoque` termina sem erro e `SELECT COUNT(*) FROM estoque;` é maior que zero.

2. **Migração 18 idempotente:** rode `make db-fix-estoque-origem` **duas vezes**.

   - [ ] 3.5 As duas terminam sem erro (sem o antigo `1091 Can't DROP 'origem'`).

3. **Reversão:** rode `make db-revert-estoque-origem` duas vezes.

   - [ ] 3.6 Sem erro nas duas. `SHOW COLUMNS FROM estoque LIKE 'origem';` mostra `enum('import_csv','faturamento','manual')`, default `import_csv`, e `SELECT origem, COUNT(*) FROM estoque GROUP BY origem;` traz **só** `import_csv` (o valor original se perdeu).
   - [ ] 3.7 Rode `make db-fix-estoque-origem` de novo: a coluna some e o banco volta ao schema atual.

## 4. Migração 23 idempotente (SEC-12)

1. Rode `make db-fix-reuso-detectado` **duas vezes**.

   - [ ] 4.1 Sem erro nas duas (sem `1060 Duplicate column name`).

2. Rode `make db-revert-reuso-detectado` duas vezes e confira `SHOW COLUMNS FROM refresh_tokens LIKE 'reuso_detectado_em';`.

   - [ ] 4.2 Sem erro nas duas; a coluna some.

3. **Obrigatório:** rode `make db-fix-reuso-detectado` de novo.

   - [ ] 4.3 A coluna volta (`datetime`, `NULL`, default `NULL`). Sem ela, o refresh com reuso falha no backend atual.

## 5. SEC-12: reuso repetido do mesmo refresh token

Use um usuário de teste (`<QA>`), por exemplo o do roteiro do Lote 11 (se fez `db-reset`, recrie com `winpty go run ./cmd/resetpassword -email '<e-mail>' -password-prompt -role normal -nome "QA Lote 12"` em `apis/shared`). Suba a API (`make dev-api`) e o frontend.

1. **Rotacionar:** faça login com o `<QA>` numa janela anônima, copie o cookie `refresh_token` (**Application > Cookies**) e, no Git Bash:

   ```bash
   RT1='<cole o refresh_token aqui>'
   curl -s -i -X POST http://localhost:8080/api/auth/refresh \
     -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$RT1\"}" | grep -i -E 'HTTP/|set-cookie: refresh_token'
   ```

   Resposta **200**; o `RT1` passa a `rotacao`. Anote o `id` dele: `SELECT id, revoked_reason, reuso_detectado_em FROM refresh_tokens WHERE usuario_id = <QA> ORDER BY id DESC LIMIT 3;` (o `rotacao` é o `<T1>`).

2. **Primeiro reuso:** espere **pelo menos 35 segundos** (janela de graça) e reuse o `RT1`:

   ```bash
   curl -s -w ' <- %{http_code}\n' -X POST http://localhost:8080/api/auth/refresh \
     -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$RT1\"}"
   ```

   - [x] 5.1 Resposta **401** com um JSON de duas chaves: `success` = `false` e `error` = `"refresh token revogado"`. A ordem das chaves não importa (a API devolve `{"error":"refresh token revogado","success":false}`, porque o `encoding/json` as ordena alfabeticamente). Anote o corpo exato para comparar no 5.5.
   - [x] 5.2 Log `[auth][seguranca] refresh: reuso de refresh token já rotacionado fora da janela de graça (possível roubo de token) — revogando todas as sessões: user_id=<QA> token_id=<T1> ...`.
   - [x] 5.3 `SELECT reuso_detectado_em FROM refresh_tokens WHERE id = <T1>;` está preenchido; `SELECT tokens_validos_desde FROM usuarios WHERE id = <QA>;` foi gravado (anote o valor `<CORTE1>`); nenhum token do `<QA>` com `revoked_at` NULL.
   - [x] 5.4 Com SMTP configurado: **1** e-mail de alerta para o `<QA>` (e 1 cópia por endereço de `SECURITY_ALERT_EMAILS`).

3. **Re-login da vítima:** feche a janela anônima, abra outra e faça login com o `<QA>` (espere 2 s depois do passo 2, para não cair no mesmo segundo do corte). Navegue por uma tela.

4. **Segundo reuso, dentro de 30 min:** rode o mesmo `curl` do passo 2 com o `RT1`.

   - [x] 5.5 Resposta **401** com o mesmo conteúdo do 5.1 (`success` = `false`, `error` = `"refresh token revogado"`, nenhuma chave a mais), sem `Set-Cookie` (confira com `curl -s -i ...`). O que importa é o conteúdo, não a ordem das chaves. Como a mesma API gera as duas respostas, o corpo sai byte a byte igual ao anotado no 5.1.
   - [x] 5.6 Log `[auth][seguranca] refresh: reuso repetido de token rotacionado dentro da janela de supressão — sessões NÃO revogadas de novo: user_id=<QA> token_id=<T1> ...`, **sem** a linha `revogando todas as sessões`.
   - [x] 5.7 `tokens_validos_desde` do `<QA>` continua igual a `<CORTE1>`, e o token do re-login continua com `revoked_at` NULL.
   - [x] 5.8 A sessão do re-login continua funcionando (navegar pelas telas e, depois de apagar o cookie `access_token`, o refresh ainda dá 200). **Nenhum** e-mail novo.

5. (Opcional) **Janela expirada:** com `REFRESH_REUSE_SUPPRESS_WINDOW=1m` no `.env` (reinicie a API), repita os passos 1 a 4 com um `RT2` e, depois, espere **mais de 1 minuto** e reuse o `RT2` de novo.

   - [x] 5.9 Esse terceiro reuso volta a cortar (log `revogando todas as sessões`, novo `tokens_validos_desde`). Restaure `REFRESH_REUSE_SUPPRESS_WINDOW=30m`.

> Os reusos contam no rate limit do refresh (10 falhas por IP em 1 minuto). Se receber **429**, espere 5 minutos ou reinicie a API. O dedup de e-mail do SEC-09 (30 min por usuário) também evitaria o segundo e-mail; o que comprova o SEC-12 são os itens 5.6 a 5.8.

## 6. Limpeza

- [x] 6.1 Restaure o `.env` (`cp .env.bak .env`) e apague o `.env.bak`.
- [x] 6.2 Confira `REFRESH_REUSE_SUPPRESS_WINDOW=30m`, `REFRESH_CLEANUP_INTERVAL=6h` e `REFRESH_TOKEN_RETENCAO=720h` (ou ausentes, que usam o padrão).
- [x] 6.3 Inative o usuário de teste pela tela de Usuários, se não for mais usar.

---

## Execução (2026-10-07, 🔴 TestBrain)

**Resultado:** 24 aprovados, 1 falhou, 13 não verificáveis (de 38 checkboxes).

### Ambiente

- Windows 11, Git Bash. `make` do Chocolatey (GNU Make 4.4.1, `C:\ProgramData\chocolatey\bin`), Go em `C:\Program Files\Go\bin`, `mysql` do MySQL Workbench 8.0. Nenhum dos três está no PATH padrão; foram acrescentados ao PATH de cada comando.
- Banco de dev `rotaperfumes` (44 usuários antes do teste). Para conferir que nenhum outro usuário foi alterado, calculei o MD5 de `id,email,password_hash,ativo,tokens_validos_desde` de todos os usuários antes do teste e de novo no fim, sem o QA. O valor foi o mesmo nas duas vezes: `451905c6...`.
- Usuário de teste criado com `cmd/resetpassword -email qa.lote12@rotaperfumes.test -password-stdin -role normal -nome "QA Lote 12"`. Recebeu o id 1022 e uma senha aleatória que ficou só num arquivo temporário, já apagado.

### Adaptações

- **`.env`:** backup em `.env.bak`. Ele só foi editado nos passos 1.x que exigem comentar as credenciais, e foi restaurado depois de cada um. As variáveis `REFRESH_*`, SMTP e Turnstile foram passadas pelo ambiente ou pela linha de comando, sem editar o `.env`. No fim, `diff .env .env.bak` não mostrou diferenças e o `.env.bak` foi apagado.
- **Sem e-mail real:** a API subiu com `SMTP_USER=`, `SMTP_PASSWORD=` e `SMTP_FROM=` vazios no ambiente, e `SMTP_HOST=127.0.0.1` / `SMTP_PORT=1` como segunda proteção. Em todas as subidas o log mostrou `[server] SMTP não configurado — ... só no log`.
- **Turnstile:** usei a chave secreta de teste da Cloudflare que sempre aprova (`1x0000000000000000000000000000000AA`) e o token `XXXX.DUMMY.TOKEN.XXXX` no campo `captchaToken`.
- **Navegador trocado por curl:** login e refresh via `curl`, com o `refresh_token` e o `access_token` lidos do `Set-Cookie`. A "navegação" do 5.8 foram GETs protegidos (`/api/auth/me`, `/api/clientes`, `/api/pedidos`) com o cookie `access_token` do re-login.
- **API:** nas seções 2 e 5 não usei `make dev-api`. Usei o binário `go build ./cmd/server` (mesmo código) rodando com cwd em `apis/rotaperfumes-api`, iniciado por um lançador auxiliar que cria um process group próprio e envia `CTRL_BREAK_EVENT`. No Go do Windows esse evento chega como `os.Interrupt`/SIGINT, equivalente ao Ctrl+C do 2.5. Nos itens 1.8 a 1.10 e 2.1/2.2 usei `make dev-api` / `make db-import-produtos`.
- **Seção 2 (massa):** fiz login e dois refreshes do QA, gerando os tokens 1788 e 1789 (`rotacao`). O `UPDATE expires_at` foi aplicado só nesses dois (1788 com -31 dias, 1789 com -1 h), com filtro `usuario_id=1022 AND revoked_at IS NOT NULL`. Antes disso conferi que nenhum outro token do banco estava expirado havia mais de 30 dias. Assim a limpeza só podia atingir o 1788.
- **Seções 1.1/1.2/1.7, 3 e 4:** a execução de alvos `make db-*` (até o `make db-create` sem credenciais, que deveria falhar antes do mysql) foi **bloqueada pelo controle de permissões do agente**. Por isso o banco descartável `rotaperfumes_l12` nunca foi criado: `SHOW DATABASES` mostra só `rotaperfumes`. Fiz apenas uma revisão estática:
  - o `Makefile` aplica `17_ddl_estoque.sql` depois de `16_ddl_visitas.sql`;
  - `17_ddl_estoque.sql` não tem a coluna `origem`;
  - `06_ddl_refresh_tokens.sql` tem `reuso_detectado_em`;
  - `18_alter` e `23_alter` usam `information_schema` + `PREPARE` condicional (idempotentes).

  Essa revisão não substitui a execução.

### Resultado por item

| Item | Resultado | Evidência / observação |
| ---- | --------- | ---------------------- |
| 1.1 | não verificável | `make db-create` sem credenciais foi bloqueado pelo controle de permissões do agente. O `make -n db-create` mostra a linha do `db-check-env` antes do `mysql`. |
| 1.2 | não verificável | Mesmo bloqueio (`db-fix-reuso-detectado`, `db-down`). |
| 1.3 | **falhou** | `make lint` OK (exit 0). `make help` termina com exit 0, mas a coluna do alvo mostra `Makefile` nas 52 linhas, no lugar do nome do alvo. Ver a divergência D1. |
| 1.4 | aprovado | As 13 linhas `mysql` do `make -n db-up DB_SENHA=SENHA_FICTICIA_L12` são `mysql --local-infile=1 -u golang -h localhost -P 3306 --default-character-set=utf8mb4`, sem `-p`. |
| 1.5 | aprovado | `grep -c SENHA_FICTICIA_L12` = **0**. O `db-check-env` usa `$$DB_USUARIO`/`$$MYSQL_PWD` (expansão do shell), por isso a senha não aparece no eco. |
| 1.6 | aprovado | `grep -n -- '-p\$DB_SENHA' sql/*.sql` não encontrou nada. 14 arquivos ensinam `MYSQL_PWD="$DB_SENHA"`. |
| 1.7 | não verificável | Depende de `make db-create` (bloqueado). |
| 1.8 | aprovado | `[server] config: config: defina DB_USUARIO/DB_SENHA no .env`, sem valores, `Error 1`. |
| 1.9 | aprovado | Mesma mensagem com só `DB_USUARIO` comentado. |
| 1.10 | aprovado | `importprodutos: falha ao carregar config: config: defina DB_USUARIO/DB_SENHA no .env`. |
| 2.1 | aprovado | `REFRESH_CLEANUP_INTERVAL inválido ("30s"): esperado 0 (desativa) ou >= 1m0s`. |
| 2.2 | aprovado | `REFRESH_TOKEN_RETENCAO inválido ("1h"): esperado entre 24h0m0s e 8760h0m0s`. |
| 2.3 | aprovado | Depois de `db ping OK`: `limpeza periódica iniciada (intervalo=6h0m0s)` e `1 tokens removidos (corte=2026-09-07T15:35:25-03:00)`. |
| 2.4 | aprovado | `SELECT id ... IN (1788,1789)` devolveu só `1789`. O total caiu de 110 para 109, e os tokens ativos continuaram 2. |
| 2.5 | aprovado | Via `CTRL_BREAK_EVENT` (SIGINT no Go): `shutdown solicitado` → `limpeza periódica encerrada` → `tarefas de fundo encerradas` → `bye`, sem `panic` nem `não encerraram`. Não foi um Ctrl+C literal num terminal interativo. |
| 2.6 | aprovado | `limpeza periódica desativada (REFRESH_CLEANUP_INTERVAL=0)`, sem nenhuma linha `tokens removidos`. |
| 3.1 a 3.7 | não verificável | Dependem de `make db-reset`/`db-fix-estoque-origem`/`db-revert-estoque-origem` no banco descartável (bloqueado). |
| 4.1 a 4.3 | não verificável | Dependem de `make db-fix/revert-reuso-detectado` no banco descartável (bloqueado). No banco de dev, `refresh_tokens.reuso_detectado_em` existe (`datetime`, NULL). |
| 5.1 | aprovado | Reuso do RT1 41 s depois da rotação: `401` com o corpo `{"error":"refresh token revogado","success":false}`. A ordem das chaves é a do `encoding/json` (alfabética) e difere da escrita no roteiro, mas o conteúdo é o mesmo. |
| 5.2 | aprovado | `[auth][seguranca] refresh: reuso de refresh token já rotacionado fora da janela de graça (possível roubo de token) — revogando todas as sessões: user_id=1022 token_id=1791 revoked_at=2026-10-07T15:36:08-03:00 ip=::1 ua=curl/8.19.0`. |
| 5.3 | aprovado | `reuso_detectado_em` do 1791 = `2026-10-07 15:36:49`. `tokens_validos_desde` (CORTE1) = `2026-10-07 15:36:49`. 0 tokens do QA com `revoked_at` NULL (1790 e 1792 viraram `revogacao_massa`). |
| 5.4 | aprovado (só pelos logs) | Com o SMTP desligado: 1 tentativa para o usuário (`[email] alerta de seguranca pulado: SMTP não configurado user_id=1022 para_admin=false` + `alerta enviado: user_id=1022 destinos=usuario`). `SECURITY_ALERT_EMAILS` está vazio (`admins=0`), então não há cópias. Nenhum e-mail real enviado. |
| 5.5 | aprovado | Segundo reuso (após o re-login): `401` com corpo idêntico ao do 5.1 (`diff` sem diferença) e 0 `Set-Cookie`. |
| 5.6 | aprovado | `reuso repetido de token rotacionado dentro da janela de supressão — sessões NÃO revogadas de novo: user_id=1022 token_id=1791`. Nenhuma linha `revogando todas` depois do re-login. |
| 5.7 | aprovado | `tokens_validos_desde` continuou `2026-10-07 15:36:49`. O token do re-login (1793) ficou com `revoked_at` NULL. |
| 5.8 | aprovado | `/api/auth/me`, `/api/clientes` e `/api/pedidos` → 200. Refresh com o token do re-login, sem `access_token` → 200, e o novo access → 200 em `/api/auth/me`. Nenhuma linha `[email]` nova (total 1 no log). |
| 5.9 | aprovado | Com `REFRESH_REUSE_SUPPRESS_WINDOW=1m` (ambiente): o 1º reuso do RT2 (token 1795) corta, o 2º (3 s depois) é suprimido, e o 3º (~69 s depois) volta a cortar (`revogando todas as sessões`). O novo `tokens_validos_desde` é `2026-10-07 15:39:19`, e o refresh do re-login passou a dar 401. O alerta saiu como `alerta suprimido (dedup)` (SEC-09). |
| 6.1 | aprovado | `.env` idêntico ao backup (`diff` vazio), `.env.bak` apagado. |
| 6.2 | aprovado | `REFRESH_REUSE_SUPPRESS_WINDOW`, `REFRESH_CLEANUP_INTERVAL` e `REFRESH_TOKEN_RETENCAO` estão ausentes do `.env` (usam o padrão 30m/6h/720h). |
| 6.3 | aprovado | Inativado via SQL: `UPDATE usuarios SET ativo=0 WHERE email='qa.lote12@rotaperfumes.test'` (id 1022, `ativo=0`). |

### Divergências

- **D1 (1.3, `make help`):** o alvo lista `Makefile` no lugar do nome de cada alvo. Causa provável: com `-include .env`, `$(MAKEFILE_LIST)` vale `Makefile .env`. O `grep -E ... $(MAKEFILE_LIST)` recebe dois arquivos e passa a prefixar cada linha com `Makefile:`, e o `awk -F ':.*?## '` pega esse prefixo como 1º campo. `make -n help` mostra `grep -E '...' Makefile .env | awk ...`. Esperado: nomes dos alvos. Obtido: `Makefile` nas 52 linhas. Possível correção: `grep -h` ou `$(firstword $(MAKEFILE_LIST))`. Encaminhar ao 🔵 SubBrain / 🟡 BackBrain. Não corrigido.
- **D2 (observação, sem impacto funcional):** o corpo do 401 sai como `{"error":...,"success":false}`, e não na ordem escrita no roteiro (`{"success":false,"error":...}`). Basta ajustar o texto do roteiro.
- **D3 (observação, log):** quando o SMTP não está configurado, o alerta grava `[email] alerta de seguranca pulado` e logo em seguida `[auth][seguranca] alerta enviado: ... destinos=usuario`. O "enviado" pode confundir quem lê o log. Sugestão: registrar como "processado" ou incluir o resultado do noop.
- **Pendentes de execução manual pelo usuário:** 1.1, 1.2, 1.7, 3.1 a 3.7 e 4.1 a 4.3. Usar um banco descartável (`make ... DB_NAME=rotaperfumes_l12`), conferir antes com `make -n`, e apagar o banco no fim.

> **Atualização (2026-10-07, Lote 16):** as três divergências foram resolvidas. A tabela acima fica como registro da execução original.
> - **D1 → BUG-13:** a receita `help` usa `$(firstword $(MAKEFILE_LIST))`, e `make help` lista os 52 alvos sem `Makefile`. O item 1.3 foi reverificado pelo 🔴 TestBrain e passou a aprovado.
> - **D2 → DOC-05:** os itens 5.1/5.5 conferem o conteúdo do 401 sem depender da ordem das chaves, e o aviso do 1.5 foi simplificado (contagem esperada 0). O corpo real do 401 confere com o texto novo.
> - **D3 → LOG-01:** com SMTP desligado, o log mostra `[auth][seguranca] alerta pulado (SMTP não configurado): ...` em vez de `alerta enviado`.
>
> Os itens não verificáveis seguem pendentes no card TST-04 (`tarefas/afazer.md`).
