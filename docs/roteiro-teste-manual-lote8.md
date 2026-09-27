# Roteiro de teste manual: Lote 8 (SEC-08, BUG-09, BUG-10, FE-10, FE-11, FE-12)

**Cards:** SEC-08 (corte de sessão: access token antigo recusado depois de inativação, troca ou reset de senha e revogação em massa), BUG-09 (creates devolvem os timestamps reais), BUG-10 (`seedusers` acha a raiz do projeto), FE-10 (paginador, contador e alerta coerentes quando a carga falha), FE-11 (timers cancelados ao sair da tela) e FE-12 (`sortValue` removido; sem teste manual próprio). Detalhes em `tarefas/feito.md` (Lote 8).
**Migração:** a 22 (`sql/22_alter_usuarios_tokens_validos_desde.sql`, `make db-fix-tokens-validos-desde`), que cria `usuarios.tokens_validos_desde`. Já aplicada no banco local em 2026-09-26. Conferência no item 1.1.
**Tempo estimado:** 35 a 45 minutos.
**Autor:** SubBrain (2026-09-26)

Marque cada checkbox depois de conferir. Nas partes de navegador, deixe o DevTools aberto na aba **Network**. Se algo divergir, anote o caso e encaminhe ao SubBrain.

> **Mudança em relação ao Lote 6:** a "Limitação conhecida (card SEC-08)" do fim da seção 1 de `docs/roteiro-teste-manual-lote6.md` deixou de valer. Depois de reativar o usuário, o access token antigo agora responde `401` `"sessão encerrada — faça login novamente"`.

---

## 0. Preparação

1. **Reinicie a API** com o código do Lote 8 (`make dev-api`, ou reinicie o debug do VS Code) e suba o frontend (`make dev-frontend`). O binário antigo não confere `tokens_validos_desde`.
2. Usuários:

   | Perfil | Usuário (e-mail) | Senha |
   | --- | --- | --- |
   | admin | `admin@rotaperfumes.com.br` | a sua |
   | **usuário de teste** (SEC-08) | `qa.lote8@rotaperfumes.test` | `QaLote8@2026` (criado abaixo) |

3. Crie o usuário de teste como **normal** (no terminal, na pasta `apis/shared`):

   ```bash
   go run ./cmd/resetpassword -email qa.lote8@rotaperfumes.test -password 'QaLote8@2026' -role normal -nome "QA Lote 8"
   ```

   Confira: `SELECT id, email, role, ativo, deve_trocar_senha, tokens_validos_desde FROM usuarios WHERE email = 'qa.lote8@rotaperfumes.test';` Anote o **id** (`<QA>` abaixo). O `tokens_validos_desde` já vem preenchido: a CLI também grava o corte (SEC-08). Se `deve_trocar_senha` = 1, o primeiro login leva a `/trocar-senha`: troque para `QaLote8@2027` e use essa daqui em diante.

4. **Snapshot para a restauração (seção 6):**

   ```sql
   SELECT (SELECT COUNT(*) FROM usuarios)      AS usuarios,
          (SELECT COUNT(*) FROM vendedores)    AS vendedores,
          (SELECT COUNT(*) FROM produtos)      AS produtos,
          (SELECT COUNT(*) FROM estoque)       AS estoque,
          (SELECT COUNT(*) FROM visitas)       AS visitas,
          (SELECT COUNT(*) FROM oportunidades) AS oportunidades,
          (SELECT COUNT(*) FROM pagamentos)    AS pagamentos;
   ```

   Anote os valores.

5. **Como pegar o access token no navegador:** DevTools > **Application > Cookies** > `http://localhost:3000` (ou a origem da API) > copie o valor de `access_token`. Nos `curl` abaixo (Git Bash):

   ```bash
   AT='<cole o access_token aqui>'
   curl -s -w ' <- %{http_code}\n' http://localhost:8080/api/auth/me -H "Authorization: Bearer $AT"
   ```

---

## 1. SEC-08: token antigo recusado depois do corte de sessão

- [ ] 1.1 Migração 22 aplicada:

  ```sql
  SHOW FULL COLUMNS FROM usuarios LIKE 'tokens_validos_desde';
  -- Type = datetime, Null = YES, Default = NULL
  ```

Use duas janelas: **A** (normal) com o admin e **B** (anônima) com o `qa.lote8@...`.

### 1a. Inativar e reativar

1. Na janela B, faça login com o `qa.lote8@...` e abra `/dashboard`. Copie o `access_token` como `AT1` e rode o `curl` do passo 0.5.

   - [ ] 1.2 Resposta **200** com os dados do QA.

2. Na janela A, em **Usuários**, **inative** o "QA Lote 8". Rode o `curl` com o `AT1`.

   - [ ] 1.3 Resposta **401** `"usuário inativo"` (a checagem de `ativo` vem antes da checagem do corte no middleware; confirmado pelo teste de precedência do 🔴 TestBrain).
   - [ ] 1.4 `SELECT tokens_validos_desde FROM usuarios WHERE id = <QA>;` mostra a hora da inativação (sem fração de segundo).

3. Na janela A, **reative** o "QA Lote 8". Rode o `curl` com o `AT1` de novo.

   - [ ] 1.5 Resposta **401** `{"success":false,"error":"sessão encerrada — faça login novamente"}`. **Antes do Lote 8, aqui voltava 200** (o defeito do SEC-08).
   - [ ] 1.6 No log da API: `[auth] acesso negado: user_id=<QA> token anterior ao corte de sessão iat=... corte=...`.
   - [ ] 1.7 O `tokens_validos_desde` **não** mudou com a reativação (mesmo valor do 1.4).
   - [ ] 1.8 Na janela B, clique em qualquer ação: a tela vai para `/login` (o refresh também foi revogado na inativação). Faça login de novo: entra normalmente, e o novo token passa no `curl` (**200**).

### 1b. Troca de senha pelo próprio usuário

1. Na janela B (logado como QA), copie o `access_token` como `AT2`. Abra uma **segunda aba** anônima na mesma sessão (será a "outra sessão").
2. Na primeira aba, vá a `/trocar-senha` e troque a senha (ex.: para `QaLote8@2028`).

   - [ ] 1.9 Aparece "Senha alterada com sucesso!" e, depois de ~1,5 s, a tela sai de `/trocar-senha`. Como o próprio token foi cortado, o destino final é `/login` (o `/dashboard` recebe `401 "sessão encerrada"` e o refresh falha). Anote se o fluxo for diferente.
   - [ ] 1.10 `curl` com o `AT2` → **401** `"sessão encerrada — faça login novamente"`.
   - [ ] 1.11 Login com a senha nova funciona, e o novo token passa no `curl` (**200**).

### 1c. Reset de senha pelo admin

1. Na janela B, faça login com o QA e copie o `access_token` como `AT3`. Na janela A, copie também o `access_token` do admin como `ATADM`.
2. Na janela A, em **Usuários**, use **Resetar senha** no "QA Lote 8".

   - [ ] 1.12 `curl` com o `AT3` → **401** `"sessão encerrada — faça login novamente"` (antes seguia válido por até 24h).
   - [ ] 1.13 `curl` com o `ATADM` → **200**: o token do admin não é afetado.
   - [ ] 1.14 Na janela B, a próxima ação leva a `/login`.

3. Volte a senha do QA para uma conhecida (a senha gerada vai por e-mail e não aparece em log):

   ```bash
   go run ./cmd/resetpassword -email qa.lote8@rotaperfumes.test -password 'QaLote8@2026' -role normal -nome "QA Lote 8"
   ```

> **Limite aceito:** pela regra `iat <= corte`, um login feito no **mesmo segundo** de um corte gera um token já recusado. Na prática só acontece se você logar menos de 1 s depois do corte; se vir um `401 "sessão encerrada"` logo após o login, espere 1 s e logue de novo. A revogação em massa do SEC-07 (reuso de refresh token) também grava o corte e está coberta pelos testes de integração.

---

## 2. BUG-09: os creates devolvem os timestamps reais

1. No Postman, importe de novo o `postman/collection.json` e rode **Login** (admin).
2. Rode as requisições abaixo e confira o `201` de cada uma. Anote os ids para a seção 6.

   | # | Requisição (Postman) | Ajuste no body | Campo a anotar |
   | --- | --- | --- | --- |
   | 2.1 | Produtos > **Criar Produto** | `sku` = `QA-LOTE8-01`, `descricao` = `QA-LOTE8 Produto` | `id` |
   | 2.2 | Estoque > **Criar Estoque** | `sku` = `QA-LOTE8-01` | `id` |
   | 2.3 | **Criar Visita** | `resultado` = `QA-LOTE8` | `visita_id` |
   | 2.4 | **Criar Oportunidade** | `origem` = `QA-LOTE8` | `oportunidade_id` |
   | 2.5 | **Criar Pagamento** | `forma_pagamento` = `PIX`, `valor` = `1`, `valor_liquido` = `1` | `pagamento_id` |

   - [ ] 2.1 a 2.5 Cada `201` traz `created_at` e `updated_at` com a data e a hora **de agora** (fuso `-03:00`), e **não** `0001-01-01T00:00:00Z`.
   - [ ] 2.6 No estoque (2.2), `produto_descricao` vem preenchido com `QA-LOTE8 Produto` (vem do JOIN com `produtos`).
   - [ ] 2.7 **Vendedor (sem requisição na collection):** em `/admin/vendedores`, crie o vendedor `QA-LOTE8 Vendedor`. Na Network, o `POST /api/vendedores` responde **201** com `created_at`/`updated_at` preenchidos. Anote o `id`.

---

## 3. FE-10: paginador, contador e alerta quando a carga falha

### 3a. Troca de página que falha

1. Logado como admin, abra `/admin/clientes` (página 1 carregada). Pare a API (Ctrl+C no `make dev-api`).
2. Clique em ">" (próxima página).

   - [ ] 3.1 Aparece o alerta de conexão. A tabela mantém as linhas da **página 1**, e o paginador e o contador continuam mostrando a **página 1** (antes mostravam a página 2 com as linhas da 1).

3. Suba a API e clique em ">" de novo.

   - [ ] 3.2 A página 2 carrega, e o paginador mostra 2. O clique repetiu a busca.

### 3b. Primeira carga que falha

1. Pare a API e abra `/admin/clientes` pela URL (F5).

   - [ ] 3.3 Só o alerta de conexão. O contador do cabeçalho **não** aparece (antes: "0 clientes"), e a tabela não mostra "Nenhum cliente cadastrado.".
   - [ ] 3.4 Repita em `/admin/produtos` e `/pagamentos`: mesmo comportamento.

2. Suba a API e recarregue: a lista e o contador voltam.

### 3c. Criar com a recarga falhando

1. Em `/admin/produtos`, no DevTools, bloqueie o padrão `localhost:8080/api/produtos?*` (**More tools > Network request blocking**). Ele bloqueia só a listagem (`GET` com query), não o `POST /api/produtos`.
2. Crie o produto `QA-LOTE8 Produto 2` (sku `QA-LOTE8-02`).

   - [ ] 3.5 O `POST` responde **201** e o `GET` da recarga aparece como `(blocked)`.
   - [ ] 3.6 Aparece **um único** alerta de erro com o texto "<mensagem de sucesso> Porem, nao foi possivel recarregar a lista (...). Os dados exibidos podem estar desatualizados.". Não aparece um alerta verde de sucesso separado.
   - [ ] 3.7 O alerta **não** some sozinho depois de 4 s.

3. Desbloqueie o padrão e recarregue a tela: o produto novo aparece na lista.

> Em Vendedores, o mesmo alerta único vale para a recarga depois de inativar. Está coberto pelos testes automatizados e fica fora do roteiro para não desligar um vendedor real.

---

## 4. FE-11: mensagens temporárias e timers

1. Em `/admin/produtos`, edite o `QA-LOTE8 Produto` (troque só a descrição para `QA-LOTE8 Produto editado`) e salve.

   - [ ] 4.1 A mensagem verde de sucesso aparece e **some sozinha em ~4 s**.

2. Salve de novo e, **antes de 4 s**, clique em outro item do menu (ex.: Clientes).

   - [ ] 4.2 A troca de tela acontece normalmente. No **Console** do DevTools, nenhum aviso de atualização de estado em componente desmontado nem erro.

3. Repita o passo 2 em `/trocar-senha` com o QA: troque a senha e, em menos de 1,5 s, clique em **Voltar** do navegador (ou digite outra URL).

   - [ ] 4.3 O redirecionamento atrasado **não** acontece depois que você saiu da tela (antes, ele rodava mesmo assim). Por causa do SEC-08, a sessão do QA termina na próxima ação; faça login com a senha nova.

> O `Paginador` comum (extraído de 10 telas) e a remoção do `sortValue` (FE-12) não mudam o comportamento visível: a ordenação continua pelo clique no cabeçalho, feita pela API. Em `/admin/senha-historico`, as colunas Usuario, Resetado Por e IP Origem parecem ordenáveis, mas não reordenam: é o card FE-13, anterior ao lote.

---

## 5. BUG-10: `seedusers` acha a raiz do projeto

O `seedusers` substitui placeholders nos arquivos `sql/02_seed_admin.sql` e `sql/03_seed_vendedores.sql`. Use `-no-exec` para **não** executar SQL no banco.

1. Antes, confira que os dois arquivos estão limpos: `git status sql/` não lista `02_seed_admin.sql` nem `03_seed_vendedores.sql`.
2. Na pasta `apis/shared`:

   ```bash
   go run ./cmd/seedusers -no-exec
   ```

   - [ ] 5.1 O log mostra `seedusers: 02_seed_admin.sql → N substituições` e `seedusers: 03_seed_vendedores.sql → N substituições` (N pode ser 0, se os arquivos já não tiverem placeholders), e `SQLs NÃO foram executados`. Antes do Lote 8, rodando de `apis/shared`, o comando procurava os arquivos na pasta **acima** do SistemaCompleto e falhava.

3. Desfaça as substituições, para não commitar hashes:

   ```bash
   git checkout -- sql/02_seed_admin.sql sql/03_seed_vendedores.sql
   ```

   - [ ] 5.2 `git status sql/` volta a não listar os dois arquivos.

> Fora de um checkout do projeto (sem `apis/shared/go.mod` até 6 níveis acima), o comando agora falha com erro, em vez de usar um caminho errado. Isso está coberto pelo `apis/shared/tests/tools/seedusers/seedusers_test.go`.

---

## 6. Restauração do banco

- [ ] 6.1 Apagar os registros de teste (use os ids anotados na seção 2):

  ```sql
  DELETE FROM pagamentos    WHERE pagamento_id    = <pagamento_id>;
  DELETE FROM oportunidades WHERE oportunidade_id = <oportunidade_id>;
  DELETE FROM visitas       WHERE visita_id       = <visita_id>;
  DELETE FROM estoque       WHERE sku LIKE 'QA-LOTE8%';
  DELETE FROM produtos      WHERE sku LIKE 'QA-LOTE8%';
  DELETE FROM vendedores    WHERE nome = 'QA-LOTE8 Vendedor';
  ```

- [ ] 6.2 Apagar o usuário de teste (os refresh tokens e o `senha_historico` dele saem por `ON DELETE CASCADE`):

  ```sql
  DELETE FROM usuarios WHERE email = 'qa.lote8@rotaperfumes.test';
  ```

- [ ] 6.3 Contagens iguais às do snapshot do passo 0.4. O `AUTO_INCREMENT` não volta, o que é esperado.
- [ ] 6.4 Nenhum outro usuário ganhou corte de sessão: `SELECT id, email, tokens_validos_desde FROM usuarios WHERE tokens_validos_desde > NOW() - INTERVAL 2 HOUR;` volta vazio (ou só o que você mudou de propósito).
- [ ] 6.5 No DevTools, padrões de bloqueio removidos (seção 3c).

---

## Testes automatizados

Rodam com `make test` (Go), `make test-frontend` (Vitest) e, com MySQL local, `INTEGRATION=1`:

```bash
cd apis/rotaperfumes-api && INTEGRATION=1 go test ./tests/handlers/ ./tests/middleware/ ./tests/services/ -coverpkg=./... -count=1
```

| Arquivo | O que cobre |
| --- | --- |
| `apis/rotaperfumes-api/tests/middleware/sec08_corte_sessao_test.go` | SEC-08: `iat <= corte` → 401 "sessão encerrada"; `iat` ausente com corte; sem corte (`NULL`) segue válido. |
| `apis/rotaperfumes-api/tests/handlers/sec08_invalidar_sessoes_test.go` | SEC-08: `InvalidarSessoes` chamado depois do `RevokeAllUserTokens` no reuso de refresh token. |
| `apis/rotaperfumes-api/tests/handlers/sec08_http_integration_test.go` | Com MySQL: inativar/reativar, troca de senha, reset pelo admin e revogação em massa → token antigo 401. |
| `apis/shared/tests/repositories/sec08_corte_sessao_test.go`, `apis/shared/tests/tools/resetpassword/sec08_corte_sessao_test.go` | SEC-08: gravação do corte em `SetAtivo(false)`, `InativarByVendedorID`, `UpdatePasswordHash`, `InvalidarSessoes` e na CLI `resetpassword`. |
| `apis/rotaperfumes-api/tests/handlers/lote6_http_integration_test.go`, `sec07_reuso_refresh_test.go` | Ajustados: o token antigo agora exige 401 depois de reativar; `ExpectExec` do `InvalidarSessoes`. |
| `apis/rotaperfumes-api/tests/services/bug09_creates_timestamps_test.go` | BUG-09: releitura depois do INSERT nos 6 creates e o caminho de falha da releitura (log e objeto em memória). |
| `apis/shared/tests/tools/seedusers/seedusers_test.go` | BUG-10: raiz achada a partir de `<raiz>`, `apis/shared`, `apis/shared/cmd/seedusers` e `sql`; erro sem `go.mod`. |
| `frontend/tests/lib/useMensagemTemporaria.test.ts`, `frontend/tests/components/ui/Paginador.test.tsx` | FE-11 e Paginador comum: timer cancelado no unmount; navegação e desabilitação. |
| `frontend/tests/app/admin/listasAdmin.test.tsx`, `listasPaginadas`, `crudPaginas`, `crudAdmin`, `clientes/page`, `trocar-senha/page`, `useListaSegura` | FE-10, FE-11 e FE-12 nas telas. |

**Regressão do 🔴 TestBrain (2026-09-26):** suítes Go 3× verdes com e sem `INTEGRATION=1`. Cobertura: `rotaperfumes-api` 89,7%, `shared` 92,8%, `tools/seedusers` 95,6%. Frontend: 35 arquivos, 1465 testes, cobertura 95,27 / 90,16 / 92,00 / 95,70 (stmts/branches/funcs/lines); `tsc` e `eslint` limpos.

---

## Anexo — Lote 9 (FE-13, 2026-09-26)

Logado como admin, abra `admin/senha-historico` (com pelo menos 3 registros, incluindo um de troca feita pelo próprio usuário, que não tem "Resetado Por").

- [ ] A.1 Clique em **Usuario**: a lista reordena pelo nome (A→Z); clique de novo: Z→A. No DevTools (Network), a requisição leva `order_by=usuario_nome`.
- [ ] A.2 Clique em **Resetado Por**: em `asc`, as linhas sem "Resetado Por" vêm primeiro; em `desc`, por último (`order_by=resetado_por_nome`).
- [ ] A.3 Clique em **IP Origem**: a lista reordena pelo IP nas duas direções (`order_by=ip_origem`).
- [ ] A.4 Com mais de uma página, troque de página em cada ordenação: nenhum registro repete nem some (desempate por `id DESC`).
- [ ] A.5 (BUG-11, opcional) Crie um usuário em `admin/usuarios`: resposta de sucesso e e-mail com a senha inicial, como antes. O caminho de falha só da releitura é coberto pelos testes automatizados (`tests/services/bug11_create_usuario_releitura_test.go`).
