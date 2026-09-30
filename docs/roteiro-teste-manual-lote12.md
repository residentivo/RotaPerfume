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
   - [ ] 1.3 `make help` e `make lint` funcionam normalmente sem as credenciais.

   Restaure o `.env`.

2. **Senha fora do eco e do argv:** use uma senha fictícia só na linha de comando (tem precedência sobre o `.env`), para não expor a real:

   ```bash
   make -n db-up DB_SENHA=SENHA_FICTICIA_L12
   make -n db-up DB_SENHA=SENHA_FICTICIA_L12 | grep -c SENHA_FICTICIA_L12
   ```

   - [ ] 1.4 As linhas `mysql ...` mostram só `--local-infile=1 -u <usuario> -h localhost -P 3306 --default-character-set=utf8mb4`, **sem** `-p` e sem a senha.
   - [ ] 1.5 A contagem do `grep -c` é **0** (a senha não aparece em nenhuma linha). **Atenção:** a linha `test -n "..." -a -n "..."` do `db-check-env` expande `$(DB_SENHA)`; se a contagem for 1 e a senha aparecer nessa linha, marque como **falha** e anote (achado registrado no manual da base, seção 23.4, item 6, para o 🟣 SecBrain/🟡 BackBrain).
   - [ ] 1.6 `grep -n -- '-p\$DB_SENHA' sql/*.sql` não encontra nada (os cabeçalhos ensinam `MYSQL_PWD="$DB_SENHA" mysql ...`).

3. **Precedência:** com o `.env` correto, rode `DB_SENHA=errada make db-create`.

   - [ ] 1.7 Funciona (o `.env` ganha da variável do shell). Já `make db-create DB_SENHA=errada` falha com `Access denied` (a linha de comando ganha de tudo).

4. **API sem credenciais:** comente `DB_SENHA` no `.env` e rode `make dev-api`.

   - [ ] 1.8 A API não sobe: `[server] config: config: defina DB_USUARIO/DB_SENHA no .env`, sem mostrar valores.
   - [ ] 1.9 O mesmo comentando só `DB_USUARIO`. Restaure o `.env`.
   - [ ] 1.10 (Opcional) `make db-import-produtos` sem `DB_SENHA` também recusa com a mesma mensagem.

## 2. CHORE-02: limpeza de `refresh_tokens`

1. **Intervalo inválido:** no `.env`, `REFRESH_CLEANUP_INTERVAL=30s`; rode `make dev-api`.

   - [ ] 2.1 A API não sobe: `REFRESH_CLEANUP_INTERVAL inválido ("30s"): esperado 0 (desativa) ou >= 1m0s`.
   - [ ] 2.2 (Opcional) `REFRESH_TOKEN_RETENCAO=1h` também impede a subida (faixa `24h` a `8760h`).

   Restaure o `.env` (`REFRESH_CLEANUP_INTERVAL=6h`, `REFRESH_TOKEN_RETENCAO=720h`).

2. **Massa para a limpeza:** escolha dois tokens **já revogados** de um usuário de teste e ajuste a expiração:

   ```sql
   SELECT id, usuario_id, expires_at, revoked_reason FROM refresh_tokens
    WHERE revoked_at IS NOT NULL ORDER BY id DESC LIMIT 5;
   UPDATE refresh_tokens SET expires_at = NOW() - INTERVAL 31 DAY WHERE id = <ANTIGO>;
   UPDATE refresh_tokens SET expires_at = NOW() - INTERVAL 1 HOUR  WHERE id = <RECENTE>;
   ```

3. **Partida:** rode `make dev-api`.

   - [ ] 2.3 Logo depois de `[server] db ping OK`: `[refresh] cleanup: limpeza periódica iniciada (intervalo=6h0m0s)` e `[refresh] cleanup: N tokens removidos (corte=...)`, com `N >= 1` e `corte` cerca de 30 dias atrás.
   - [ ] 2.4 `SELECT id FROM refresh_tokens WHERE id IN (<ANTIGO>, <RECENTE>);` devolve só o `<RECENTE>` (expirado há 1h continua; expirado há 31 dias foi apagado). Nenhum token ativo sumiu.

4. **Shutdown limpo:** no terminal da API, `Ctrl+C`.

   - [ ] 2.5 Aparecem, nesta ordem, `[server] shutdown solicitado`, `[refresh] cleanup: limpeza periódica encerrada`, `[server] tarefas de fundo encerradas` e `[server] bye`, sem `panic` e sem `tarefas de fundo não encerraram`.

5. **Desativada:** `REFRESH_CLEANUP_INTERVAL=0` no `.env`, suba a API.

   - [ ] 2.6 Log `[refresh] cleanup: limpeza periódica desativada (REFRESH_CLEANUP_INTERVAL=0)` e nenhum `tokens removidos`. Restaure o `.env` e suba a API de novo.

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

   - [ ] 5.1 Resposta **401** `{"success":false,"error":"refresh token revogado"}`. Anote o corpo exato.
   - [ ] 5.2 Log `[auth][seguranca] refresh: reuso de refresh token já rotacionado fora da janela de graça (possível roubo de token) — revogando todas as sessões: user_id=<QA> token_id=<T1> ...`.
   - [ ] 5.3 `SELECT reuso_detectado_em FROM refresh_tokens WHERE id = <T1>;` está preenchido; `SELECT tokens_validos_desde FROM usuarios WHERE id = <QA>;` foi gravado (anote o valor `<CORTE1>`); nenhum token do `<QA>` com `revoked_at` NULL.
   - [ ] 5.4 Com SMTP configurado: **1** e-mail de alerta para o `<QA>` (e 1 cópia por endereço de `SECURITY_ALERT_EMAILS`).

3. **Re-login da vítima:** feche a janela anônima, abra outra e faça login com o `<QA>` (espere 2 s depois do passo 2, para não cair no mesmo segundo do corte). Navegue por uma tela.

4. **Segundo reuso, dentro de 30 min:** rode o mesmo `curl` do passo 2 com o `RT1`.

   - [ ] 5.5 Resposta **401** com o corpo **idêntico** ao do 5.1, sem `Set-Cookie` (confira com `curl -s -i ...`).
   - [ ] 5.6 Log `[auth][seguranca] refresh: reuso repetido de token rotacionado dentro da janela de supressão — sessões NÃO revogadas de novo: user_id=<QA> token_id=<T1> ...`, **sem** a linha `revogando todas as sessões`.
   - [ ] 5.7 `tokens_validos_desde` do `<QA>` continua igual a `<CORTE1>`, e o token do re-login continua com `revoked_at` NULL.
   - [ ] 5.8 A sessão do re-login continua funcionando (navegar pelas telas e, depois de apagar o cookie `access_token`, o refresh ainda dá 200). **Nenhum** e-mail novo.

5. (Opcional) **Janela expirada:** com `REFRESH_REUSE_SUPPRESS_WINDOW=1m` no `.env` (reinicie a API), repita os passos 1 a 4 com um `RT2` e, depois, espere **mais de 1 minuto** e reuse o `RT2` de novo.

   - [ ] 5.9 Esse terceiro reuso volta a cortar (log `revogando todas as sessões`, novo `tokens_validos_desde`). Restaure `REFRESH_REUSE_SUPPRESS_WINDOW=30m`.

> Os reusos contam no rate limit do refresh (10 falhas por IP em 1 minuto). Se receber **429**, espere 5 minutos ou reinicie a API. O dedup de e-mail do SEC-09 (30 min por usuário) também evitaria o segundo e-mail; o que comprova o SEC-12 são os itens 5.6 a 5.8.

## 6. Limpeza

- [ ] 6.1 Restaure o `.env` (`cp .env.bak .env`) e apague o `.env.bak`.
- [ ] 6.2 Confira `REFRESH_REUSE_SUPPRESS_WINDOW=30m`, `REFRESH_CLEANUP_INTERVAL=6h` e `REFRESH_TOKEN_RETENCAO=720h` (ou ausentes, que usam o padrão).
- [ ] 6.3 Inative o usuário de teste pela tela de Usuários, se não for mais usar.
