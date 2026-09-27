# Roteiro de teste manual: Lote 11 (SEC-09, SEC-10, FE-14)

**Cards:** SEC-09 (e-mail de alerta no reuso de refresh token rotacionado), SEC-10 (endurecimentos: TLS SMTP, senhas sem viés, `resetpassword` sem senha na linha de comando, `seedusers` sem senhas fixas e sem gravar em `sql/`, corte de sessão truncado) e FE-14 (formatador de data único). DOC-04 não tem teste manual. Detalhes em `tarefas/feito.md` (Lote 11).
**Migração:** nenhuma.
**Tempo estimado:** 25 a 35 minutos.
**Autor:** SubBrain (2026-09-26)

Marque cada checkbox depois de conferir. Se algo divergir, anote o caso e encaminhe ao SubBrain.

---

## 0. Preparação

1. No `.env` da raiz, confira `DB_USUARIO` e `DB_SENHA` preenchidos (o `resetpassword` e o `seedusers` não usam mais o default `golang/golang`).
2. Preencha `SMTP_USER`, `SMTP_PASSWORD` e `SMTP_FROM` (veja `.env.example`) e `SECURITY_ALERT_EMAILS` com 1 ou 2 caixas de admin que você consiga ler (separadas por vírgula).
3. Crie um usuário de teste com um **e-mail real** que você consiga ler (por exemplo, um alias `+qa11` do seu Gmail), na pasta `apis/shared`:

   ```bash
   winpty go run ./cmd/resetpassword -email '<seu-email+qa11@gmail.com>' -password-prompt -role normal -nome "QA Lote 11"
   ```

   Digite `QaLote11@2026` duas vezes. Anote o **id** (`<QA>` abaixo). Se cair em `/trocar-senha` no primeiro login, troque e use a nova senha.
4. Reinicie a API (`make dev-api`) e suba o frontend (`make dev-frontend`).

   - [ ] 0.1 O log de subida **não** mostra "alertas de segurança só no log" (com SMTP configurado). Sem SMTP essa mensagem aparece e os alertas ficam só no log.

## 1. SEC-09: e-mail no reuso de refresh token

1. Faça o reuso como na seção 5c de `docs/roteiro-teste-manual-lote6.md` (passos 1 e 2), com o usuário `<QA>`: login, copiar o `refresh_token` como `RT1`, um refresh com `RT1` (200), esperar **35 s** e reusar o `RT1`.

   - [ ] 1.1 Resposta **401** `{"success":false,"error":"refresh token revogado"}` (inalterada) e, no log, o `[auth][seguranca]` do item 5.6 do roteiro do Lote 6.
   - [ ] 1.2 A caixa do `<QA>` recebe **1** e-mail de alerta, com horário de Brasília, IP e navegador (UA com até 120 caracteres), **sem** `user_id` nem `token_id`.
   - [ ] 1.3 Cada caixa de `SECURITY_ALERT_EMAILS` recebe **1** cópia, **com** `user_id` e `token_id`.
   - [ ] 1.4 No log, as linhas do alerta mostram `[email-usuario]` no lugar do e-mail do usuário (o endereço não aparece em nenhuma linha).

2. **Dedup de 30 min:** faça login de novo, repita o passo 1 com um `RT2` (rotacionar, esperar 35 s, reusar) **antes de 30 minutos** do primeiro alerta.

   - [ ] 1.5 Resposta **401** e alerta `[auth][seguranca]` no log de novo, mas **nenhum** e-mail novo (usuário nem admins).

3. **Controle:** reuso de token de logout (passo 3 da seção 5c do roteiro do Lote 6).

   - [ ] 1.6 Nenhum e-mail é enviado.

> Os reusos contam no rate limit do refresh (10 falhas por IP em 1 minuto). Se receber **429**, espere 5 minutos ou reinicie a API.

## 2. SEC-10: `resetpassword`, `seedusers` e `gen-hash`

Na pasta `apis/shared`:

1. **Prompt sem eco:** `winpty go run ./cmd/resetpassword -email '<e-mail do QA>' -password-prompt`.
   - [ ] 2.1 A senha não aparece ao digitar; pede confirmação; confirmação diferente → erro e nada é gravado.
2. **stdin:** `read -rs P && printf '%s' "$P" | go run ./cmd/resetpassword -email '<e-mail do QA>' -password-stdin`.
   - [ ] 2.2 Senha trocada; o login com ela funciona.
3. **Depreciado:** `go run ./cmd/resetpassword -email '<e-mail do QA>' -password 'QaLote11@2026'`.
   - [ ] 2.3 Funciona, mas o stderr mostra o aviso de depreciação.
   - [ ] 2.4 Duas fontes juntas (ex.: `-password ... -password-stdin`) → erro, nada é gravado.
4. **Credenciais obrigatórias:** comente `DB_SENHA` no `.env` e rode qualquer comando do `resetpassword`.
   - [ ] 2.5 Erro pedindo `DB_USUARIO`/`DB_SENHA` (sem tentar `golang/golang`). Restaure o `.env`.
5. **Seeds sem tocar em `sql/`:** na raiz, rode `make gen-hash`.
   - [ ] 2.6 `git status sql/` não mostra mudanças: `sql/02_seed_admin.sql` e `sql/03_seed_vendedores.sql` continuam só com placeholders.
   - [ ] 2.7 Não sobra a pasta `tmp/seed` (as cópias com hash são apagadas ao final).
   - [ ] 2.8 As senhas vêm de `SEED_ADMIN_PASSWORD`/`SEED_USER_PASSWORD` do `.env`; sem elas, o log avisa "não definido — gerando senha aleatória (modo dev)" (para ver a gerada, rode `go run ./cmd/seedusers -show-password` em `apis/shared`). Não existe mais senha padrão fixa. **Atenção:** `make gen-hash` reaplica os seeds; se as variáveis não estiverem no `.env`, as senhas do admin e dos vendedores mudam.
   - [ ] 2.9 (Opcional) Com `-no-exec` (`go run ./cmd/seedusers -no-exec`), as cópias ficam em `tmp/seed` com aviso no terminal. Apague a pasta depois.

> O TLS mínimo 1.2 do SMTP, as senhas sem viés e o truncamento do corte de sessão são cobertos só pelos testes automatizados. O item 1.2 já confirma que o envio SMTP com validação de certificado funciona.

## 3. FE-14: datas nas telas

Com o admin logado, confira as colunas de data em:

- [ ] 3.1 `/pagamentos` e o modal de pagamento: datas sem hora (vencimento, pagamento) iguais às do banco, **sem** aparecer um dia antes.
- [ ] 3.2 `/admin/clientes` e o modal de vendedor (`/admin/vendedores`, datas de admissão/desligamento): mesmo critério do 3.1.
- [ ] 3.3 `/admin/estoque`, `/admin/pedidos`, `/admin/visitas`, `/admin/oportunidades`: datas no formato `dd/mm/aaaa` (e hora onde houver), iguais às do banco.
- [ ] 3.4 `/admin/senha-historico`: data/hora corretas; registro com `created_at` NULL mostra `-`.
- [ ] 3.5 Campo de data vazio aparece como `-` (nunca "31/12/1" nem "Invalid Date").

Para conferir o 3.1, compare com o banco, por exemplo: `SELECT id, data_vencimento, data_pagamento FROM pagamentos ORDER BY id DESC LIMIT 5;`.

## 4. Limpeza

- [ ] 4.1 Apague o usuário de teste (ou inative pela tela de Usuários) e limpe `SECURITY_ALERT_EMAILS` se não quiser receber alertas em dev.
