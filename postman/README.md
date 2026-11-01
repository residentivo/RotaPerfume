# RotaPerfumes API — Postman Collection

> Coleção Postman para testes e consumo dos endpoints de autenticação do SistemaCompleto.

## Arquivos

| Arquivo | Descrição |
|---------|-----------|
| `collection.json` | Coleção Postman v2.1 com todos os endpoints |
| `README.md` | Este manual |

## Como usar

### 1. Importar a coleção

1. Abra o **Postman**.
2. Clique em **Import** → **Upload Files** e selecione `collection.json`.
3. A coleção **"RotaPerfumes API — Autenticação"** aparecerá na sua biblioteca.

### 2. Configurar variáveis de ambiente

A coleção inclui variáveis pré-definidas. Ajuste `{{base_url}}` se sua API estiver em outra porta:

| Variável | Valor padrão | Descrição |
|----------|-------------|-----------|
| `base_url` | `http://localhost:8080` | URL base da API |
| `token` | (vazio) | JWT do usuário logado (preenchido automaticamente após login) |
| `admin_token` | (vazio) | JWT do admin (preenchido automaticamente após login como admin) |
| `vendedor_token` | (vazio) | JWT de um vendedor (preenchido pelo teste "Login Vendedor") |
| `refresh_token` | (vazio) | Refresh token (preenchido automaticamente após login) |
| `senha_vendedor` | (vazio) | Senha dos usuários de seed, usada no "Login — Vendedor". **Preencha à mão** com o `SEED_USER_PASSWORD` do `.env` ou a senha impressa pelo `make db-seed` (DOC-03, 2026-09-26) |
| `imagem_arquivo` | (vazio) | Nome do arquivo da imagem do produto (`<uuid-v4>.png` etc.). Preenchido pelo teste de "Enviar/Trocar Imagem do Produto" e usado na leitura pública (IMG-01, Lote 26, 2026-10-31) |

### 3. Autenticar

1. Abra a requisição **Login** (`POST /api/auth/login`) — usa `admin@rotaperfumes.com.br` por padrão.
2. Clique em **Send**.
3. Os tokens serão salvos automaticamente nas variáveis `token`, `admin_token` e `refresh_token` via script de testes.
4. Para logar como vendedor, use a requisição **Login — Vendedor** (`POST /api/auth/login` com `rafael.carvalho@rotaperfumes.com.br`, usuário id 5, vinculado ao vendedor 4 ativo). Antes, preencha a variável `senha_vendedor` com a senha de seed (ver a nota abaixo). A requisição salva o token em `vendedor_token` e confere só que `trocar_senha` é booleano: o valor depende de `usuarios.deve_trocar_senha` (`true` se o usuário ainda não trocou a senha; `false` depois da troca, que é o estado do id 5 no banco local em 2026-09-26). O `henrique.rodrigues@...` (id 2), usado antes, está com `ativo = 0` e recebe `401 "credenciais inválidas"` no login (DOC-03, 2026-09-26; até o Lote 24 a mensagem era `"usuário inativo"`, ver SEC-22).

### 4. Acessar endpoints protegidos

As requisições protegidas usam o token Bearer salvo. Basta executá-las — o Postman injeta o header `Authorization: Bearer {{token}}` automaticamente.

Para testar endpoints que exigem **role admin**, alterne a variável `token` para `admin_token` no cabeçalho de autorização da requisição.

## Credenciais padrão

### Admin

| Campo | Valor |
|-------|-------|
| Email | `admin@rotaperfumes.com.br` |
| Senha | `Admin@123` |
| Role | `admin` |

### Vendedor (42 vendedores, dados de seed/dev)

Formato de e-mail: `<slug-nome>@rotaperfumes.com.br`

Exemplos (todos com a senha de seed, ver a nota abaixo):
- `rafael.carvalho@rotaperfumes.com.br` (id 5, vendedor 4 ativo; usado na requisição "Login — Vendedor")
- `carla.carvalho@rotaperfumes.com.br`
- `thiago.silva@rotaperfumes.com.br` (id 4, vendedor 3 **desligado**: faz login, mas as rotas da carteira respondem `403`)

> **Senha de seed (DOC-03, 2026-09-26):** não existe senha fixa de vendedor. O `make db-seed` grava nos usuários de seed o valor de `SEED_USER_PASSWORD` do `.env` ou, se ela não estiver definida, uma senha aleatória de 16 caracteres impressa no console (`USER_PASSWORD=...`), a mesma para todos (ver o cabeçalho de `sql/03_seed_vendedores.sql`). Na collection, informe essa senha na variável `senha_vendedor`. O nome `@HASH_MUDAR_123` no SQL é legado, e `Mudar@123` **não** confere com o banco local.
>
> **`trocar_senha`:** vem da coluna `usuarios.deve_trocar_senha`. É `true` enquanto o usuário não troca a senha (depois do seed, da criação ou do reset pelo admin), e o frontend redireciona para a tela de troca de senha. É `false` depois da troca (`POST /api/auth/reset-password`). Os dois casos são válidos: no banco local de 2026-09-26, o id 5 está com `deve_trocar_senha = 0`.
>
> **Troca obrigatória imposta pelo backend (SEC-23, Lote 28, 2026-10-31):** com `trocar_senha = true`, só `GET /api/auth/me` e `POST /api/auth/reset-password` funcionam; as demais rotas protegidas respondem `403` `"troca de senha obrigatória"`. Os usuários recém-semeados (`make db-seed`) estão nesse estado: troque a senha antes de usar a collection com eles.
>
> **Fluxo real de senha (produção/novos usuários):** a partir da criação de usuário (`POST /api/usuarios`) ou reset de senha pelo admin (`POST /api/admin/reset-password`), a API **não usa mais senha fixa**. Ela gera uma senha aleatória segura (`crypto/rand`, mínimo 12 caracteres, com maiúscula/minúscula/dígito/símbolo garantidos) e a envia por email (SMTP) ao endereço cadastrado do usuário. A senha gerada nunca é exposta pela API — nem em logs, nem na resposta HTTP. Ambas as respostas (criação e reset) incluem um campo `email_enviado: boolean` informando se o envio de fato ocorreu (será `false` se o SMTP não estiver configurado, caso em que a API usa um serviço "noop" que apenas loga o envio, sem quebrar a operação de criação/reset).

## Mudanças de contrato do Lote 4 (2026-09-25)

> Resumo das mudanças. O detalhe está em cada endpoint abaixo e nos exemplos marcados com `(SEC-02 | SEC-03 | BUG-06 | NEG-01, 2026-09-25)` na collection. Os cards continuam em `tarefas/fazendo.md` até a validação do usuário.
>
> **Envelope de erro:** `{"success": false, "error": "..."}`. O envelope (na época, `writeJSON` em `handlers/response.go`; desde o Lote 21, `backend/rotaperfumes-api/internal/httpx/respond.go`) omite `data` quando é nulo, então as respostas de erro **não** trazem `"data": null`.

| Card | Endpoint | Mudança |
|------|----------|---------|
| SEC-02 | `POST /api/auth/refresh` | Novo `401` `"refresh token revogado"` quando uma requisição concorrente já usou o token (contava no rate limit; deixou de contar no SEC-04, Lote 5). Falha ao gravar o novo refresh token → `500` `"erro interno"`, sem cookie (antes: `200` sem refresh). Erro de banco na revogação → `500`. |
| SEC-03 | `GET /api/vendedores` | Admin: todos (sem mudança). `normal` com vínculo: `data` só com o próprio vendedor (`id, nome, regiao, uf, data_desligamento`). Sem vínculo: `data: []`. Vendedor desligado: `403` `"acesso bloqueado: vendedor desligado"`. |
| BUG-06 | `PATCH /api/clientes\|produtos\|usuarios/{id}/inativar` | Body preenchido e inválido (JSON malformado, `{"ativo":"x"}`, `{"ativo":1}`, `[true]`) → `400` `"body JSON inválido"`. Vazio, `null`, `{}` ou `{"ativo":null}` → toggle. `{"ativo":true\|false}` → define o valor. Em clientes, sem acesso ao registro continua `404` antes do `400`. |
| NEG-01 | `POST`/`PUT /api/clientes` | Novo `400` `"cnpj inválido"` (formato, 14 dígitos ou dígito verificador). Novo `409` `"cnpj já cadastrado"` (genérico). Aceita máscara; grava e devolve só dígitos (no Lote 5, o NEG-02 passou a aceitar o CNPJ alfanumérico). CNPJ válido usado nos exemplos: `11222333000181`. |
| NEG-04 | `PUT /api/clientes/{id}` | O dígito verificador é exigido em **toda** gravação, mesmo quando o CNPJ enviado é igual ao atual (antes só era exigido se o CNPJ mudasse). DV inválido → `400` `"cnpj inválido"` sem consultar o banco, inclusive para id inexistente (antes: `404`). Clientes legados com DV inválido só podem ser editados depois de corrigir o CNPJ. `PATCH /inativar` não valida o CNPJ. |

## Mudanças de contrato do Lote 5 (2026-09-25)

> Resumo das mudanças. O detalhe está em cada endpoint abaixo e nos exemplos marcados com `(NEG-02, 2026-09-25)` na collection. Roteiro manual: `docs/roteiro-teste-manual-lote5.md`. Os cards continuam em `tarefas/fazendo.md` até a validação do usuário.

| Card | Endpoint | Mudança |
|------|----------|---------|
| NEG-02 | `POST`/`PUT /api/clientes`, `GET /api/clientes?q=` | Aceita o **CNPJ alfanumérico** da Receita (vigente desde julho de 2026). As 12 primeiras posições são `[0-9A-Z]` e os 2 DVs são numéricos. Envio com ou sem máscara, em maiúsculas ou minúsculas. Retorno sempre com 14 caracteres, sem máscara e em maiúsculas. A busca `?q=` aceita máscara e não diferencia maiúsculas de minúsculas. Os erros `400`/`409` não mudam. Exemplo oficial: `12ABC34501DE35` (`12.ABC.345/01DE-35`). CNPJs numéricos continuam válidos. |
| SEC-04 | `POST /api/auth/refresh` | Um token revogado há no máximo 30 s (janela de graça) e a corrida no `BeginRotation` **não contam mais** no rate limit por IP. O reuso fora da janela continua contando e gera log `[auth][seguranca]`. A resposta HTTP não muda. |
| FE-06 | (frontend, `apiClient.ts`) | Falha de rede no retry após o refresh não faz logout: a fila recebe o mesmo erro de rede, e não mais "Sessão expirada. Faça login novamente.". Sem mudança de contrato na API. |

## Mudanças de contrato do Lote 6 (2026-09-26)

> Resumo das mudanças. O detalhe está nos endpoints abaixo e nas descrições marcadas com `(SEC-06 | SEC-07 | BUG-08, Lote 6, 2026-09-26)` na collection. Roteiro manual: `docs/roteiro-teste-manual-lote6.md`. Os cards continuam em `tarefas/fazendo.md` até a validação do usuário.

| Card | Endpoint | Mudança |
|------|----------|---------|
| SEC-06 | **Todas as rotas protegidas** (Bearer) | O middleware JWT (`JWTMiddlewareWithUserCheck` + `DBUserStatusChecker`) consulta `usuarios.ativo` e `usuarios.role` a cada requisição, sem cache. Usuário inativo ou inexistente → `401` `{"success":false,"error":"usuário inativo"}`, mesmo com o access token dentro da validade. Erro de banco na checagem → `500` `"erro interno"` (fail-closed). A role usada é a do banco: rebaixar `admin` → `normal` vale na próxima requisição, sem novo login (rota admin → `403`). |
| SEC-06 | `PATCH /api/usuarios/{id}/inativar`, `DELETE /api/vendedores/{id}` | Inativar o usuário revoga os refresh tokens dele. Desligar o vendedor revoga os refresh tokens dos usuários vinculados, na mesma transação que grava o desligamento (`RevokeAllByVendedorID`). Motivo gravado: `inativacao`. |
| SEC-07 | `POST /api/auth/refresh` | Toda revogação grava `refresh_tokens.revoked_reason` (migração 21). Reuso, fora da janela de 30 s, de token revogado por `rotacao` → alerta `[auth][seguranca]` com `user_id`, `token_id`, IP e UA, e revogação de **todos** os refresh tokens do usuário (`revogacao_massa`). Outros motivos e `NULL` (legado) → só log informativo. Resposta HTTP sem mudança (`401` `"refresh token revogado"`). |
| BUG-08 | `POST /api/clientes` | O `201` traz `created_at` e `updated_at` preenchidos (o cliente é relido do banco depois do INSERT, com ou sem carteira). Antes: `0001-01-01T00:00:00Z`. O `PUT` já relia. |
| FE-07 | (frontend, `apiClient.ts`) | Falha de rede, timeout ou falha do Web Lock no **próprio** refresh → `NetworkError`, sem logout e sem chamar `/api/auth/logout`; a fila recebe o mesmo erro. `401`, `429` e `500` no refresh continuam deslogando. Sem mudança de contrato na API. |
| FE-09 | (frontend, listagens) | Com erro na carga, as 10 listagens mostram só o alerta, sem "Nenhum ... cadastrado". Sem mudança de contrato na API. |

> **Limitação conhecida (card SEC-08):** o SEC-06 revoga só os refresh tokens. Se o usuário for reativado antes de o access token antigo expirar (TTL de 24h), esse token volta a valer. **Resolvida no Lote 8 (SEC-08, ver abaixo).**

## Mudanças de contrato do Lote 8 (2026-09-26)

> Resumo das mudanças. O detalhe está nas descrições marcadas com `(SEC-08 | BUG-09, Lote 8, 2026-09-26)` na collection. Roteiro manual: `docs/roteiro-teste-manual-lote8.md`.

| Card | Endpoint | Mudança |
|------|----------|---------|
| SEC-08 | **Todas as rotas protegidas** (Bearer) | Corte de sessão por usuário (`usuarios.tokens_validos_desde`, migração 22). Access token com `iat <= tokens_validos_desde`, ou sem `iat` quando há corte → `401` `{"success":false,"error":"sessão encerrada — faça login novamente"}`, mesmo dentro das 24h. Log: `[auth] acesso negado: user_id=... token anterior ao corte de sessão iat=... corte=...`. Sem corte (`NULL`), nada muda. |
| SEC-08 | `PATCH /api/usuarios/{id}/inativar` (inativar), `DELETE /api/vendedores/{id}`, `POST /api/auth/reset-password`, `POST /api/admin/reset-password`, reuso de refresh token do SEC-07, CLI `resetpassword` | Gravam o corte (hora do Go truncada no segundo). Efeito: token antigo não volta a valer depois de reativar; troca ou reset de senha derruba todas as sessões abertas do usuário (inclusive a que trocou a senha). Reativação e logout **não** gravam corte. Um login no mesmo segundo de um corte gera token recusado (aceito pelo 🤍 MegaBrain). |
| BUG-09 | `POST /api/vendedores`, `/api/produtos`, `/api/pagamentos`, `/api/oportunidades`, `/api/visitas`, `/api/estoque` | O `201` traz `created_at`/`updated_at` reais e os campos preenchidos pelo banco (ex.: `produto_descricao` do estoque, via JOIN), porque o registro é relido depois do INSERT. Antes podiam vir `0001-01-01T00:00:00Z`. Mesmo envelope. Se só a releitura falhar: log e objeto em memória. `POST /api/usuarios` já relia. |
| BUG-10 | (CLI `seedusers`) | `FindProjectRoot` acha a raiz pela pasta com `backend/shared/go.mod` (até 6 níveis acima) e devolve erro se não achar. Sem mudança na API. |
| FE-10, FE-11, FE-12 | (frontend) | Paginador e contador coerentes quando a carga falha; mensagens temporárias sem timer pendente no unmount; `sortValue` removido das colunas (ordenação é server-side). Sem mudança na API. |

## Mudanças de contrato do Lote 9 (2026-09-26)

> Resumo das mudanças. O detalhe está nas descrições marcadas com `(FE-13 | BUG-11, Lote 9, 2026-09-26)` na collection. Teste manual do FE-13: anexo de `docs/roteiro-teste-manual-lote8.md`.

| Card | Endpoint | Mudança |
|------|----------|---------|
| FE-13 | `GET /api/senha-historico`, `GET /api/senha-historico/{usuario_id}` | `order_by` aceita também `usuario_nome`, `resetado_por_nome` e `ip_origem`. Quando a coluna não é `id`, desempate por `id DESC` (vale também para as chaves antigas; paginação sem repetir nem pular). `resetado_por_nome` nulo (troca pelo próprio usuário) vem primeiro em `asc` e por último em `desc`. Chave fora da lista → `id` na direção pedida, sem `400`. A tela `admin/senha-historico` envia as três chaves novas. |
| BUG-11 | `POST /api/usuarios` | Ordem: INSERT → e-mail com a senha inicial → releitura. Se só a releitura falhar: `201` (antes: erro, com o usuário gravado e sem e-mail), com `email_enviado` correto, `created_at`/`updated_at` zerados e `vendedor_nome` vazio; log `[usuarios] criado, mas falhou a releitura: id=...`. Falha no INSERT continua com erro e sem e-mail. |
| CHORE-01 | (repositório) | `*.tsbuildinfo` no `.gitignore`; `frontend/tsconfig.tsbuildinfo` deixou de ser rastreado. Sem mudança na API. |

## Mudanças de contrato do Lote 10 (2026-09-26)

> Não muda o formato da resposta, só a robustez. Detalhe nas descrições marcadas com `(BUG-12, Lote 10, 2026-09-26)` na collection.

| Card | Endpoint | Mudança |
|------|----------|---------|
| BUG-12 | `GET /api/senha-historico`, `GET /api/senha-historico/{usuario_id}` | Registro com NULL no banco não derruba mais a listagem (antes: `500` "converting NULL to string is unsupported"). `ip_origem`/`user_agent` NULL saem `""`; `created_at` NULL sai `"0001-01-01T00:00:00Z"`; `usuario_nome` de usuário inexistente sai `""`. A tela `admin/senha-historico` exibe `-` para a data zero. |

## Mudanças do Lote 11 (2026-09-26)

> **Sem mudança de contrato HTTP** (nenhum endpoint, corpo ou status mudou; a collection não foi alterada). Teste manual: `docs/roteiro-teste-manual-lote11.md`.

| Card | Onde | Mudança |
|------|------|---------|
| SEC-09 | `POST /api/auth/refresh` | O reuso de um refresh token já rotacionado, fora da janela de 30 s, continua respondendo `401` `"refresh token revogado"`, mas agora envia um e-mail de alerta ao usuário e uma cópia aos admins listados em `SECURITY_ALERT_EMAILS` (nova env, separada por vírgula; ver `.env.example`). Dedup de 30 min por usuário, teto de 20 e-mails/h, envio assíncrono. Sem SMTP, o alerta fica só no log. |
| SEC-10 | CLIs e SMTP | `resetpassword`: novas flags `-password-prompt` (sem eco) e `-password-stdin`; `-password` depreciado. `resetpassword` e `seedusers` exigem `DB_USUARIO`/`DB_SENHA` no `.env`. `make gen-hash` não altera mais `sql/` (usa cópias temporárias em `tmp/seed`). SMTP exige TLS 1.2+ com validação de certificado. |
| FE-14 | (frontend) | Formatador de data único; datas `AAAA-MM-DD` não aparecem mais um dia antes; vazio e data zero do Go aparecem como `-`. Sem mudança na API. |

## Mudanças do Lote 12 (2026-09-27)

> **Sem mudança de contrato HTTP** (nenhum endpoint, corpo ou status mudou; a collection não foi alterada). Teste manual: `docs/roteiro-teste-manual-lote12.md`. Detalhes do banco: `docs/manual-base-de-dados.md` (seções 7.1, 7.2, 17 e 23).

| Card | Onde | Mudança |
|------|------|---------|
| SEC-12 | `POST /api/auth/refresh` | Reenviar um refresh token já rotacionado derruba as sessões do usuário **uma vez** por token: novos reusos do **mesmo** token dentro de `REFRESH_REUSE_SUPPRESS_WINDOW` (padrão 30 min) não revogam as sessões de novo nem enviam outro e-mail, e o re-login da vítima sobrevive. A resposta continua **idêntica** nos dois casos (`401` `{"success":false,"error":"refresh token revogado"}`, sem `Set-Cookie`, contando no rate limit): o cliente não distingue o reuso suprimido. Coluna nova `refresh_tokens.reuso_detectado_em` (migração 23, `make db-fix-reuso-detectado`). |
| CHORE-02 | (API, tarefa de fundo) | Limpeza periódica de `refresh_tokens`: na partida e a cada `REFRESH_CLEANUP_INTERVAL` (padrão `6h`; `0` desativa), apaga em lotes de 1000 os tokens com `expires_at` há mais de `REFRESH_TOKEN_RETENCAO` (padrão `720h`). Token ainda não expirado nunca é apagado. Logs `[refresh] cleanup: ...`. |
| SEC-11 | `Makefile`, `config.Load()` | `DB_USUARIO`/`DB_SENHA` obrigatórios no `.env`, sem o default `golang/golang`: a API e os importadores não sobem sem eles, e os alvos `db-*` param no `db-check-env`. O `make` passa a senha ao cliente do banco (`mysql` no Lote 12; `mariadb` desde o DB-03) por `MYSQL_PWD` (não mais `-p...` no argv) e lê o `.env` com `-include .env` (ver "Subindo o ambiente"). |
| DB-01 | `make db-up` | Passa a criar a tabela `estoque` (`sql/17_ddl_estoque.sql`, já sem `origem`), depois de `visitas`. Bancos antigos com `estoque.origem`: `make db-fix-estoque-origem` (idempotente). |

## Mudanças do Lote 21 (2026-10-13 a 2026-10-16): reestruturação DDD da API

> Refatoração interna (DDD + Clean Code + Design Patterns; ver `docs/arquitetura-ddd.md` e `docs/adr-001-reestruturacao-ddd.md`). Rotas, payloads, status e mensagens **não mudaram**, com uma única exceção aprovada pelo usuário (X1, aplicada no ARQ-01). Nenhuma requisição desta collection dependia do comportamento antigo: todas já usam o método documentado e nenhuma chama `/api/senha-historico/{id}` com subcaminho, `0` ou negativo. A collection não foi alterada, salvo a referência ao arquivo do handler de clientes na descrição.

| Card | Endpoint | Mudança |
|------|----------|---------|
| X1 | `POST /api/auth/login`, `POST /api/auth/refresh`, `POST /api/auth/logout`, `POST /api/auth/reset-password`, `GET /api/auth/me`, `GET /api/senha-historico`, `GET /api/senha-historico/{user_id...}`, `GET /api/dashboard/metrics`, `GET /api/dashboard/vendas`, `GET /api/dashboard/vendedores`, `GET /api/dashboard/clientes`, `GET /health` | Essas rotas aceitavam qualquer método. Agora só aceitam o método indicado: outro método recebe **`405`** com o header `Allow` (corpo em texto do `ServeMux`, sem o envelope JSON), antes da autenticação e sem consultar o banco. `HEAD` continua aceito nas rotas `GET` (`HEAD /health` → `200`; `HEAD /api/auth/me` sem token → `401`). `OPTIONS` continua `204` no CORS. |
| X1 | `GET /api/senha-historico/{user_id...}` | Parse estrito do id: só dígitos e maior que zero. `/api/senha-historico/2/extra` (antes `200` com o histórico do usuário 2), `/0` e `/-1` passam a **`400`** `{"success":false,"error":"usuario_id inválido"}`. `/api/senha-historico/` (vazio) continua `400` `"usuario_id é obrigatório na URL"`. |
| ARQ-01 | Respostas de erro | A ordem das chaves no JSON de erro pode mudar; os valores são os mesmos. |

## Mudanças do Lote 22 (2026-10-29)

> **Endpoint novo**, sem mudança nos contratos existentes. A API passa de 60 para **61 rotas** (4 Public, 36 Authenticated, 21 AdminOnly). Na collection, pasta **"Dashboard — Funil de vendas (DASH-01, Lote 22)"** (default, `30d`, `90d`, `365d` e `400`). Teste manual: seção 7 de `docs/roteiro-teste-manual-dashboard.md`.

| Card | Endpoint | Mudança |
|------|----------|---------|
| DASH-01 | `GET /api/dashboard/funil?janela=30d\|90d\|365d\|all` | Funil de vendas das oportunidades por etapa (cumulativo), com conversão em relação à etapa anterior e ao topo, perdidas, taxa de ganho e total. Acesso autenticado com escopo por `oportunidades.vendedor_id`; sem acesso ou vendedor desligado → `200` zerado. Janela fora do enum → `400`. Detalhe em "GET /api/dashboard/funil" abaixo. No frontend, seção "Funil de vendas" do Dashboard. |

## Mudanças do Lote 24 (2026-10-29): GAPs de segurança ALTA da auditoria ARQ-00

> Muda **uma** mensagem de erro (login de usuário inativo) e o comportamento do refresh de usuário inativo. Rotas e payloads de sucesso não mudaram. Na collection: o exemplo "401 Unauthorized — Usuário inativo" do Login passou a mostrar `"credenciais inválidas"`, e o Refresh Token ganhou o exemplo "401 Unauthorized — Refresh de usuário inativo". Corpo de erro real: as chaves saem em ordem alfabética (`{"error":"...","success":false}`); os exemplos mostram o mesmo conteúdo.

| Card | Onde | Mudança |
|------|------|---------|
| SEC-22 | `POST /api/auth/login` | Usuário inativo responde `401` `"credenciais inválidas"`, com o mesmo corpo e o mesmo custo de hash de usuário inexistente e de senha errada (antes: `401` `"usuário inativo"`). A resposta não revela que a conta existe nem que está inativa. |
| SEC-22 | `POST /api/auth/refresh` | Refresh token ainda ativo de usuário inativo é revogado com motivo `inativacao` e a resposta é `401` `"refresh token revogado"`. |
| SEC-22 | Rotas autenticadas (Bearer) | Sem mudança: access token válido de usuário inativo continua recebendo `401` `"usuário inativo"` do middleware (SEC-06). |
| SEC-18 | Access token e `config.Load()` | O JWT precisa ter `iss` igual a `JWT_ISSUER` (aparado; vazio vira `rotaperfumes`), `exp` e algoritmo HS256; `iat` no futuro é recusado (`401`). A API não sobe com `JWT_SECRET` menor que 32 bytes ou igual ao placeholder (erro `config: JWT_SECRET é obrigatório (mínimo de 32 bytes)`) nem com `JWT_TTL <= 0`. |
| SEC-20 | Rate limit, logs e alertas | O IP do cliente é o item **mais à direita** do `X-Forwarded-For`, validado e normalizado; valor inválido cai no `RemoteAddr`. O `X-Real-IP` deixou de ser lido. Um `X-Forwarded-For` forjado no Postman não troca mais o IP usado pelo rate limit atrás do Caddy. Sem env nova. |
| SEC-21 | Cookies de sessão (login, refresh, logout) | O `Secure` vem da env `COOKIE_SECURE` (default `true`), não mais do `Host` da requisição. `make dev-api` e `make run-api` usam `COOKIE_SECURE=false` (HTTP local); o `deploy/setup-servidor.sh` força `true`. A limpeza dos cookies no logout usa os mesmos atributos da criação. |

## Mudanças de contrato do Lote 25 (2026-10-30): status de pedido, pagamento quitado, ids e duplicidade

> Mudanças aprovadas pelo usuário em 2026-10-29 (CHORE-03 e CHORE-06 mudam o contrato HTTP; BUG-16, BUG-20 e SEC-27 trazem respostas de erro novas). Algumas recusas são **extensões do 🤍 MegaBrain** para impedir que a baixa de estoque seja burlada (marcadas abaixo). Na collection, cada pasta com rota `{id}` ganhou o exemplo "400 Bad Request — id inválido (CHORE-03, Lote 25)"; Pedidos, Pagamentos, Produtos e Estoque ganharam os exemplos `400`/`409` novos; a pasta Vendedores ganhou "Vincular Cliente ao Vendedor" e "Encerrar Vínculo Cliente-Vendedor". O exemplo principal de "Editar Pedido" passou a levar o pedido `100001` de Faturado para Entregue com os mesmos itens (o anterior mudava os itens, o que já dava `409`).

| Card | Endpoint | Mudança |
|------|----------|---------|
| CHORE-03 | Todas as 29 rotas com `{id}` | Id de path `<= 0` ou não numérico (`0`, `-1`, `+1`, `abc`, `1.5`, `" 1"`, overflow) → **`400`** `"id inválido"`, sem consultar o banco. Antes, `0` e `-1` chegavam ao repositório e davam `404`. Em `DELETE /api/vendedores/{id}/clientes/{clienteId}`, o `{clienteId}` inválido → `400` `"cliente id inválido"`. O `httpx.IDDoPathPermissivo` foi removido; toda rota usa o `httpx.IDDoPath` estrito. O frontend não muda (os ids vêm da API). |
| CHORE-06 | `POST /api/produtos` | SKU já usado → **`409`** `"sku já cadastrado"` (antes `500`). |
| CHORE-06 | `POST /api/estoque` | Par `(data_snapshot, sku)` já existente → **`409`** `"registro de estoque já cadastrado para este sku nesta data"` (antes `500`; esta documentação dizia `400` por engano). A tela de Estoque acrescenta "Edite o registro existente.". |
| CHORE-06 | `POST /api/vendedores/{id}/clientes` | Vínculo do mesmo par cliente/vendedor na mesma data → **`409`** `"vínculo entre cliente e vendedor já cadastrado nesta data"` (antes `500`). |
| CHORE-06 | `POST`/`PUT /api/clientes`, `POST /api/usuarios` | Sem mudança: CNPJ (`"cnpj já cadastrado"`) e e-mail (`"email já cadastrado"`) já eram `409`. Em todos os casos o log registra só o índice, sem o valor duplicado. |
| BUG-16 | `PUT /api/pedidos/{id}` | Máquina de status (`400`, checada antes dos itens, sob o lock do pedido): Faturado → só Entregue (`"pedido faturado só pode ter o status alterado para Entregue"`); Entregue é terminal (`"pedido entregue não pode ter o status alterado"`); Em separação ou Cancelado → Entregue recusado (`"pedido só pode ser marcado como Entregue depois de Faturado"`, **extensão do MegaBrain**). Reenviar o mesmo status continua idempotente. Itens diferentes em pedido Faturado **ou Entregue** → `409` com o texto antigo `"pedido já faturado: não é possível alterar os itens, apenas o status"` (card CHORE-07). Detalhe em "PUT /api/pedidos/{id}". |
| BUG-16 | `DELETE /api/pedidos/{id}` | Pedido Entregue → **`409`** `"pedido entregue não pode ser excluído"` (**extensão do MegaBrain**). Ordem: pagamentos vinculados primeiro, depois Faturado/Entregue. |
| BUG-20 | `POST /api/pedidos` | Pedido criado já como Faturado **baixa o estoque** na mesma transação do INSERT (falha na baixa → nada é gravado). Criado como Entregue → **`400`** `"pedido só pode ser marcado como Entregue depois de Faturado"` (**extensão do MegaBrain**). |
| SEC-27 | `POST`/`PUT /api/pagamentos` | `taxa_pct` fora de 0 a 30 (limites inclusivos) → **`400`** `"taxa_pct deve estar entre 0 e 30"`, para admin e normal. No banco, CHECK `chk_pagamentos_taxa_pct` (migração 24, `make db-fix-taxa-pct-check`; revert `make db-revert-taxa-pct-check`). |
| SEC-27 | `PUT /api/pagamentos/{id}` | Pagamento quitado (`Pago`/`Pago com atraso`) editado pelo usuário `normal` → **`409`** `"pagamento já quitado não pode ser editado"` (UPDATE condicional; a corrida também dá `409`). Admin edita normalmente (`200`). Inexistente continua `404`. |
| BUG-17 | `PATCH /api/produtos/{id}/inativar` | Sem body, o toggle é um `UPDATE … SET ativo = NOT ativo` atômico. Contrato igual; o `updated_at` da resposta passa a ser o real. |
| BUG-17 (nota) | `PUT /api/estoque/{id}` | Sem mudança, só documentado: **PUT de estoque é sobrescrita absoluta** do saldo, inclusive por cima de baixas de faturamento concorrentes. |
| BUG-19, BUG-21 | Dashboard e baixa de estoque | Sem mudança de contrato. `arredondar2` usa `math.Round` (negativos corretos, nunca `-0`); a ruptura do upsert de faturamento é calculada sobre o saldo já atualizado. |
| Doc | `DELETE /api/pedidos/{id}` | Correção: o `409` de pagamentos vinculados é `"pedido possui pagamentos vinculados"` (a documentação e o exemplo mostravam um sufixo que a API não envia). |

## Mudanças do Lote 26 (2026-10-31): imagem do produto (IMG-01)

> **3 rotas novas** e um campo novo no JSON do produto; nenhum contrato existente mudou. A API passa de 61 para **64 rotas** (5 Public, 36 Authenticated, 23 AdminOnly). Na collection, 3 requests no fim da pasta **"Produtos"**: "Enviar/Trocar Imagem do Produto", "Imagem do Produto — leitura pública" e "Remover Imagem do Produto", nessa ordem (rodar a remoção antes da leitura dá `404`). Os exemplos de produto anteriores ao Lote 26 não mostram `imagem_url`. Teste manual: `docs/roteiro-teste-manual-lote26.md`.

| Card | Endpoint | Mudança |
|------|----------|---------|
| IMG-01 | `POST /api/produtos/{id}/imagem` | **Novo, admin.** `multipart/form-data` com uma única parte `imagem` (JPEG, PNG ou WebP, até 1 MB). Header `X-Requested-With: XMLHttpRequest` obrigatório. `200` com o produto. Detalhe em "POST /api/produtos/{id}/imagem" abaixo. |
| IMG-01 | `DELETE /api/produtos/{id}/imagem` | **Novo, admin.** `200` com `imagem_url: null`; idempotente; `404` se o produto não existe. |
| IMG-01 | `GET /api/public/produtos/imagens/{arquivo}` | **Novo, público** (sem token, sem cookie). Serve o arquivo com `Content-Type` fixo pela extensão e cache imutável de 1 ano. Aceita `HEAD` e `Range`. `404` `"imagem não encontrada"`. |
| IMG-01 | `GET /api/produtos`, `GET /api/produtos/{id}`, `POST`/`PUT`/`PATCH` de produtos | Campo novo `imagem_url`: `"/api/public/produtos/imagens/<uuid>.<jpg\|png\|webp>"` ou `null`. O diretório no servidor nunca é exposto. |
| IMG-01 | Config e deploy | Env nova `PRODUTO_IMAGENS_DIR` (padrão `./data/produtos-imagens`; em dev, `backend/rotaperfumes-api/data/produtos-imagens/`). Vazio, `/` ou dentro de `frontend/public` impede a subida, assim como diretório sem permissão de escrita. No deploy, volume `produto_imagens` montado em `/data/produtos-imagens`. |
| IMG-01 | Banco | Coluna `produtos.imagem` (migração 26, `make db-fix-produtos-imagem`; revert `make db-revert-produtos-imagem`). Ver `docs/manual-base-de-dados.md`, seção 13. |
| IMG-01 | `make db-gerar-imagens-produtos` | Gera um PNG 600x600 com o nome do produto na diagonal para cada produto sem imagem, pelo mesmo caso de uso do upload. Já rodado no banco local: 292 imagens. Ver "Imagens de produto" em "Subindo o ambiente". |

## Mudanças do Lote 27 (2026-10-31): limites de entrada, CORS, busca e transações

> **Nenhuma rota nova**: a API continua com **64 rotas** (5 Public, 36 Authenticated, 23 AdminOnly). As mudanças são respostas novas para entradas anormais (corpo, headers, `page`, `q`, itens do pedido), uma regra de configuração (CORS) e o `500` do BUG-15. O caminho de sucesso das rotas não muda. Na collection, os exemplos de listagem usam `page=1` e `q` curto, então nenhum exemplo precisou mudar; a descrição ganhou a seção "Mudanças do Lote 27".

| Card | Onde | Mudança |
|------|------|---------|
| SEC-14 | Todas as rotas com corpo | Corpo acima de **256 KiB** → **`413`** `"corpo da requisição excede o limite de 256 KB"` (`Content-Length` acima do limite é recusado sem chamar o handler; corpo chunked é cortado pelo `MaxBytesReader`). 401/403 da policy continuam vindo antes do 413. Única exceção: `POST /api/produtos/{id}/imagem`, que mantém o seu limite (1 MB + 64 KB) e o `413` `"imagem excede 1 MB"`. |
| SEC-14 | Servidor HTTP | `ReadHeaderTimeout` de 5 s (contra slowloris) e `MaxHeaderBytes` de 64 KiB: headers acima disso recebem **`431`** automático do Go. |
| SEC-14 | `POST /api/pedidos`, `PUT /api/pedidos/{id}` | Pedido com mais de **200 itens** → **`400`** `"o pedido deve ter no máximo 200 itens"`. O maior pedido do banco local tem 16 itens. |
| SEC-16 | Todas as listagens paginadas | `page` acima de **10000** é limitado a 10000 (clamp, como o `limit` acima de 100 vira 100); `pagination.page` mostra o valor depois do clamp. `page` gigante (ex.: `9223372036854775807`), que estourava o OFFSET e dava `500`, agora dá `200`; `page` fora da faixa de `int` (ex.: `99999999999999999999`) conta como ausente (página 1). |
| SEC-25 | Buscas `q` (produtos, clientes, visitas, oportunidades, pedidos) | `q` com mais de **100 caracteres** (contados depois de tirar os espaços das pontas) → **`400`** `"o parâmetro q deve ter no máximo 100 caracteres"`, sem consultar o banco. `%` e `_` digitados passam a ser **literais** (antes funcionavam como curinga: `q=%` devolvia tudo). No SQL: `LIKE ? ESCAPE '!'`. |
| SEC-17 | Config (`CORS_ALLOWED_ORIGINS`) | Entrada `*` ou `null` (sem diferenciar maiúsculas) **impede a API de subir** (`config: CORS_ALLOWED_ORIGINS não aceita "*" nem "null" (os cookies de sessão exigem origens explícitas)`). O middleware também não aceita mais curinga. **Antes do deploy:** conferir que o Secret file do Jenkins não tem `*`/`null`. Documentado em `.env.example` e `deploy/api.env.example`. |
| BUG-15 | `POST /api/auth/reset-password`, reset pelo admin (`POST /api/admin/reset-password`) e reset em massa | Histórico, nova senha e revogação das sessões gravados numa única transação. **Mudança intencional:** falha ao gravar o histórico ou ao revogar as sessões, que antes respondia `200` (com o erro só no log), passa a **`500`**, sem trocar a senha. No reset pelo admin, o e-mail só sai depois do commit. Usuário inexistente continua `404`. |
| SEC-15, SEC-24, BUG-14, SEC-26 | — | Sem mudança de contrato: DSN mascarado também com `@` na senha (SEC-15); testes de injeção de `order_by` em todas as listagens (SEC-24, `order_by` inválido continua caindo no padrão com `200`); `POST /api/vendedores/{id}/clientes` numa transação com lock do cliente (BUG-14); posse e regras de exclusão do pedido checadas sob lock no PUT e no DELETE (SEC-26, mesmos status e mensagens). |

## Mudanças do Lote 28 (2026-10-31): troca de senha obrigatória, Turnstile e pagamentos sob lock

> **Nenhuma rota nova**: a API continua com **64 rotas** (5 Public, 36 Authenticated, 23 AdminOnly). Muda o acesso de quem tem troca de senha pendente (SEC-23), a validação do captcha (SEC-19) e um `500` que vira `404` na criação de pagamento (SEC-35). Na collection, o `GET /api/auth/me` ganhou `trocar_senha` no exemplo, e a descrição ganhou a seção "Mudanças do Lote 28".
>
> **Atenção ao usar a collection com os usuários do seed:** os 42 usuários de `sql/03_seed_vendedores.sql` nascem com `deve_trocar_senha = 1`. Enquanto não trocarem a senha, **todas** as rotas protegidas respondem `403` `"troca de senha obrigatória"`, menos `GET /api/auth/me` e `POST /api/auth/reset-password`. Antes de usar as demais requisições com um desses usuários (inclusive o "Login — Vendedor"), faça login, rode "Reset Password (própria senha)" (`POST /api/auth/reset-password`, que derruba a sessão, SEC-08) e faça login de novo. O admin também é bloqueado se estiver com a flag ligada.

| Card | Onde | Mudança |
|------|------|---------|
| SEC-23 | Todas as rotas não públicas, exceto `GET /api/auth/me` e `POST /api/auth/reset-password` | Usuário com `usuarios.deve_trocar_senha = 1` (qualquer papel, inclusive admin) recebe **`403`** `{"success":false,"error":"troca de senha obrigatória"}`. Ordem no middleware: os `401` (token, inexistente, inativo, sessão encerrada) e o `500` vêm antes; depois o `403` de troca; por último o `403` de admin. O identificador é a mensagem exata + `403` (sem campo `code`). Log: `[auth] acesso negado: user_id=... troca de senha pendente <método> <rota saneada>`. `login`, `refresh`, `logout`, `/health` e a imagem pública são Public e não mudam. No frontend, esse `403` leva a `/trocar-senha`. |
| SEC-23 | `GET /api/auth/me` | Campo novo **`trocar_senha`** (bool, entre `role` e `ultimo_login_at`), com o valor de `usuarios.deve_trocar_senha`. Mudança aditiva. O frontend usa o campo para redirecionar para `/trocar-senha` antes de renderizar a tela. |
| SEC-19 | `POST /api/auth/login`, `POST /api/auth/reset-password` | O token Turnstile passa a ser conferido também pelo **`hostname`** (sem diferenciar maiúsculas, contra a lista aceita) e pela **`action`** (exata): `login` no login e `reset-senha` na troca de senha. Token de outro site ou do outro fluxo é recusado com a resposta de sempre, **`400`** `"captcha inválido ou ausente"`, ainda antes do rate limit. Com uma secret de teste da Cloudflare (`1x…AA`, `2x…AA`, `3x…AA`), hostname e action não são conferidos (aviso no startup). |
| SEC-19 | Config (`TURNSTILE_HOSTNAMES`) | **Env nova, opcional:** hostnames aceitos, separados por vírgula, sem esquema, porta, caminho ou curinga (ex.: `ivo-inspiron-15-3530,192.168.168.106`). Vazia ou não definida: a lista é derivada dos hosts de `CORS_ALLOWED_ORIGINS`. Item inválido (`://`, `/`, `:`, `*`, espaço interno, item vazio) impede a subida com `TURNSTILE_HOSTNAMES inválida`, sem o valor; `:` é recusado, então IPv6 não é aceito. Lista final vazia recusa todos os tokens (fail-closed). O startup registra o modo (teste, lista vazia ou `hostnames aceitos=N` com a origem). Documentada em `.env.example` e `deploy/api.env.example`. **No deploy:** o hostname da página do frontend precisa estar no CORS ou nesta env, e backend e frontend precisam subir juntos (uma aba com o bundle antigo recebe `400` no login até recarregar). |
| SEC-35 | `POST /api/pagamentos` | Criação numa transação com lock do pedido. Pedido apagado no meio da criação (erro de FK `1452` em `fk_pagamentos_pedido`) passa de `500` para **`404`** `"pedido não encontrado"`, sem o texto do driver no log. A ordem de erros `403` → `400` → `404` → `400` não muda. |
| SEC-35 | `DELETE /api/pagamentos/{id}` | Exclusão numa transação (lock do pedido no escopo restrito, depois lock do pagamento) e `DELETE` condicional a "não quitado": pagamento quitado no meio da exclusão dá `409` `"pagamento já quitado não pode ser excluído"` em vez de ser apagado. Status e mensagens iguais no caminho normal; na exclusão pelo admin, erro de banco no lock do pagamento passa a `500`. |

## Endpoints

### Healthcheck

#### GET /health
- **Auth:** nenhuma
- **Descrição:** Verifica se a API está no ar

### Autenticação (`/api/auth/*`)

#### POST /api/auth/login
- **Auth:** nenhuma
- **Body:** `{ "email": "...", "password": "...", "captchaToken": "..." }`
- **Resposta:** `{ "success": true, "data": { "access_token", "refresh_token", "token_type", "expires_in", "user": {...}, "trocar_senha" } }`
- **Nota:** `trocar_senha` reflete diretamente a coluna `usuarios.deve_trocar_senha` (não é mais calculado comparando a senha digitada com uma constante fixa). Fica `true` quando o usuário foi criado ou teve a senha resetada pelo admin e ainda não trocou a senha voluntariamente; volta a `false` após `POST /api/auth/reset-password`.
- **CAPTCHA:** `captchaToken` (string, **obrigatório**) é o token do widget Cloudflare Turnstile, validado server-side contra `https://challenges.cloudflare.com/turnstile/v0/siteverify` (fail-closed, timeout 4s). Ver seção "CAPTCHA (Cloudflare Turnstile)" abaixo.
- **Anti-enumeração (SEC-22, Lote 24, 2026-10-29):** usuário inexistente, usuário inativo e senha errada respondem igual: `401` `"credenciais inválidas"`, com o mesmo custo de hash. Antes, o inativo recebia `401` `"usuário inativo"`.

#### POST /api/auth/refresh
- **Auth:** nenhuma (usa `refresh_token` no body)
- **Body:** `{ "refresh_token": "..." }`
- **Resposta:** Novo par `{ "access_token", "refresh_token", "token_type", "expires_in" }`
- **Nota:** O refresh token antigo é revogado (single-use)
- **Corrida entre requisições (SEC-02, 2026-09-25):** a revogação agora é condicional e a rotação é transacional (`BeginRotation`/`Issue`). Se duas requisições usam o mesmo refresh token ao mesmo tempo, só uma recebe o novo par; a outra recebe `401` `{"success":false,"error":"refresh token revogado"}`. Na versão do SEC-02, essa falha contava no rate limit de refresh; o SEC-04 mudou isso (ver abaixo).
- **Rate limit e reuso de token revogado (SEC-04, 2026-09-25):** o `refreshLimiter` por IP (10 falhas em 1 minuto, bloqueio de 5 minutos → `429` com `Retry-After`) passou a distinguir a corrida legítima do reuso suspeito:
  - **Janela de graça de 30 s** (`RefreshRevokeGraceWindow`): um token revogado há no máximo 30 s (outra aba ou outro usuário atrás do mesmo IP/NAT acabou de rotacioná-lo) responde `401` e **não conta** no rate limit. Log: `token revogado recentemente (corrida entre abas), não conta no rate limit`.
  - **Corrida na rotação:** quando a requisição perde a corrida no `BeginRotation` (outra requisição concorrente revogou o token), responde `401` e **não conta**. Log: `token já revogado por requisição concorrente (corrida no BeginRotation), não conta no rate limit`.
  - **Reuso fora da janela** (token revogado há mais de 30 s): **continua contando** e gera o log `[auth][seguranca] refresh: reuso de refresh token revogado fora da janela de graça (possível roubo de token): ip=... ua=...`.
  - **Continuam contando:** token não encontrado e token expirado.
  - **Resposta HTTP sem mudança:** dentro e fora da janela o corpo é idêntico (`401` `{"success":false,"error":"refresh token revogado"}`), sem `Set-Cookie` e sem emitir token. O cliente não consegue distinguir os dois casos. A rotação continua de uso único e o frontend não mudou.
- **Falha de banco (SEC-02):** erro ao revogar o token antigo ou ao gravar o novo refresh token → `500` `"erro interno"`, **sem cookie e sem novo refresh token**. Antes, a falha ao gravar o novo refresh token respondia `200` só com o access token.
- **Erros:** `400` `"refresh_token é obrigatório"`; `401` inválido/expirado/revogado; `429` rate limit; `500` erro interno.
- **Frontend (SEC-02):** `apiClient.ts` serializa o refresh entre abas com `navigator.locks` (lock `"rp-auth-refresh"`) e, se o refresh responder `401`, repete a requisição original uma única vez.
- **Frontend (FE-06, 2026-09-25):** se essa repetição (retry após o refresh) falhar por **erro de rede**, o `apiClient.ts` **não** faz logout e propaga o mesmo erro de rede (o erro original do `fetch`, ex.: `Failed to fetch`) para todas as requisições que estavam na fila do refresh (`onRefreshComplete(false, erro)`). Antes, a fila recebia "Sessão expirada. Faça login novamente." sem que o usuário fosse deslogado. "Sessão expirada" só aparece quando há logout (refresh `401`/`429`/`500`, ou retry `401`). Depois da falha de rede, o estado do refresh é liberado e um novo `401` dispara um novo refresh.
- **Frontend (FE-07, Lote 6, 2026-09-26):** falha de rede, timeout ou falha do Web Lock no **próprio** `POST /api/auth/refresh` também deixou de fazer logout: o `apiClient.ts` propaga `NetworkError` para a requisição e para a fila, sem chamar `/api/auth/logout` (o refresh token continua válido no cookie). Antes, esses casos deslogavam.
- **Motivo da revogação e reuso (SEC-07, Lote 6, 2026-09-26):** toda revogação grava `refresh_tokens.revoked_reason` (migração 21): `rotacao` (refresh), `logout`, `revogacao_massa`, `senha` (troca ou reset de senha) e `inativacao` (SEC-06). No reuso fora da janela de 30 s:
  - token revogado por `rotacao` → log `[auth][seguranca] refresh: reuso de refresh token já rotacionado fora da janela de graça (possível roubo de token) — revogando todas as sessões: user_id=... token_id=... revoked_at=... ip=... ua=...` e `RevokeAllUserTokens` com motivo `revogacao_massa` (todas as sessões do usuário perdem o refresh);
  - outro motivo ou `NULL` (tokens de antes da migração) → só log informativo `token revogado apresentado fora da janela de graça: user_id=... token_id=... motivo=...`, sem revogação em massa. Isso elimina o falso positivo de "possível roubo" para abas antigas depois de um logout.
  - A resposta HTTP continua idêntica (`401` `"refresh token revogado"`) e o reuso fora da janela continua contando no rate limit.
- **Reuso repetido do mesmo token (SEC-12, Lote 12, 2026-09-27):** só o primeiro reuso de um token `rotacao` revoga todas as sessões e envia o alerta. Novos reusos do **mesmo** token dentro de `REFRESH_REUSE_SUPPRESS_WINDOW` (padrão 30 min) só geram o log `[auth][seguranca] refresh: reuso repetido de token rotacionado dentro da janela de supressão — sessões NÃO revogadas de novo`, sem revogar nem enviar e-mail; depois da janela, o reuso volta a cortar. A supressão é por token: reuso de **outro** token roubado continua cortando. Resposta idêntica (`401` `"refresh token revogado"`), contando no rate limit.
- **Usuário inativado (SEC-06):** como os refresh tokens são revogados na inativação, o refresh desse usuário responde `401` `"refresh token revogado"`. `401` `"usuário inativo"` continua valendo para um token ainda válido de usuário inativo.
- **Refresh de usuário inativo com token ainda ativo (SEC-22, Lote 24, 2026-10-29):** se sobrou um refresh token não revogado (por exemplo, `ativo = 0` gravado direto no banco), o refresh revoga esse token com motivo `inativacao` e responde `401` `"refresh token revogado"`, sem cookie e sem novo par.

#### POST /api/auth/logout
- **Auth:** nenhuma (usa `refresh_token` no body)
- **Body (opcional):** `{ "refresh_token": "..." }`
- **Resposta:** `{ "success": true, "data": { "mensagem": "logout realizado" } }`

#### GET /api/auth/me
- **Auth:** Bearer Token
- **Resposta:** `{ id, nome, email, role, ativo, id_vendedor, trocar_senha, created_at, ultimo_login_at, vendedor_desligado }`
- **`trocar_senha` (bool, SEC-23, Lote 28, 2026-10-31):** valor de `usuarios.deve_trocar_senha`. Enquanto for `true`, o `/me` e `POST /api/auth/reset-password` são as únicas rotas protegidas liberadas; as demais respondem `403` `"troca de senha obrigatória"`.
- **`vendedor_desligado` (bool, 2026-09-24):** `true` só para usuário `normal` vinculado a vendedor com `data_desligamento` preenchida. É `false` para admin, para usuário sem vínculo e para vínculo órfão (vendedor inexistente). O `/me` continua acessível para o vendedor desligado (não retorna `403`). O frontend usa este campo e o `id_vendedor` atualizado para revalidar a sessão (em vez de confiar só no `localStorage`) e exibir o aviso.
- **Erros:** `401` não autenticado; `401` `"usuário inativo"` (SEC-06); `404` usuário não encontrado; `500` erro interno (inclui falha ao checar se o vendedor está desligado ou o status do usuário).
- **Usuário inativo em rotas protegidas (SEC-06, Lote 6, 2026-09-26):** vale para o `/me` e para todas as rotas com Bearer. O middleware lê `ativo` e `role` do usuário no banco a cada requisição, sem cache. Inativo ou inexistente → `401` `"usuário inativo"`; o front faz logout. Erro de banco → `500`. A role do banco substitui a do token, então um rebaixamento vale sem novo login.
- **Sessão encerrada (SEC-08, Lote 8, 2026-09-26):** vale para o `/me` e para todas as rotas com Bearer. Token com `iat` até o segundo de `usuarios.tokens_validos_desde` (ou sem `iat` quando há corte) → `401` `"sessão encerrada — faça login novamente"`. O corte é gravado na inativação, no desligamento do vendedor, na troca e no reset de senha, na revogação em massa do SEC-07 e na CLI `resetpassword`.

#### POST /api/auth/reset-password
- **Auth:** Bearer Token (qualquer role autenticado)
- **Body:** `{ "senha_atual": "...", "nova_senha": "...", "captchaToken": "..." }`
- **Descrição:** Usuário troca a própria senha (precisa da senha atual)
- **CAPTCHA:** `captchaToken` (string, **obrigatório**) — mesma validação Turnstile do login (fail-closed). Endpoint também ganhou rate limiting dedicado: 10 falhas em 5 minutos bloqueiam por 10 minutos (lacuna identificada pelo SecBrain — antes não existia rate limiting aqui).
- **Efeito nas sessões (SEC-08, Lote 8, 2026-09-26):** a troca grava `usuarios.tokens_validos_desde`. Todos os access tokens emitidos até esse segundo, inclusive o usado na troca, passam a receber `401` `"sessão encerrada — faça login novamente"`. O frontend leva ao login depois da troca.

### Usuários (`/api/usuarios`) — admin only

#### GET /api/usuarios
- **Auth:** Bearer Token (admin)
- **Query:** `?page=1&limit=20&order_by=nome&order_dir=asc`
- **Descrição:** Lista usuários paginada (total + pages)
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, nome, email, role, ativo, created_at, updated_at, ultimo_login_at` (default: `id`); `order_dir` aceita `asc`|`desc` case-insensitive (default: `asc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/usuarios
- **Auth:** Bearer Token (admin)
- **Body:** `{ "nome", "email", "role": "admin"|"normal" }`
- **Descrição:** Cria um novo usuário. A senha inicial é gerada aleatoriamente e enviada por email para o endereço cadastrado (nunca retornada pela API). `deve_trocar_senha` é setada para `true` no banco. Resposta inclui `email_enviado: boolean`.
- **Releitura (BUG-11, Lote 9, 2026-09-26):** o usuário é relido no service depois do envio do e-mail. Se só a releitura falhar, a resposta é `201` com `email_enviado` correto, timestamps zerados e `vendedor_nome` vazio (antes: erro, sem e-mail, e nova tentativa batia em e-mail duplicado).

#### PUT /api/usuarios/{id}
- **Auth:** Bearer Token (admin)
- **Body:** `{ "nome", "role": "admin"|"normal" }`
- **Descrição:** Atualiza nome e/ou role

#### PATCH /api/usuarios/{id}/inativar
- **Auth:** Bearer Token (admin)
- **Body (opcional):** `{ "ativo": true|false }` — omitido = toggle
- **Descrição:** Ativa ou inativa o usuário
- **Body inválido (BUG-06, 2026-09-25):** vazio, `null`, `{}` ou `{"ativo":null}` → toggle; `{"ativo":true|false}` → define o valor; body preenchido e inválido (JSON malformado, `{"ativo":"x"}`, `{"ativo":1}`, `[true]`) → `400` `"body JSON inválido"`, sem alterar o usuário. Antes esses bodies caíam no toggle.
- **Efeito na sessão (SEC-06, Lote 6, 2026-09-26):** inativar revoga os refresh tokens do usuário (`revoked_reason = 'inativacao'`), e a próxima requisição dele em qualquer rota protegida recebe `401` `"usuário inativo"`. ~~Limitação (card SEC-08): reativado antes de o access token antigo expirar (24h), esse token volta a valer.~~ Resolvida no Lote 8: inativar grava o corte `usuarios.tokens_validos_desde`, e o token antigo recebe `401` `"sessão encerrada — faça login novamente"` mesmo depois de reativar.

### Admin (`/api/admin/*`) — admin only

#### POST /api/admin/reset-password
- **Auth:** Bearer Token (admin)
- **Body:** `{ "usuario_id": <int64> }`
- **Descrição:** Reseta a senha de um usuário para uma senha aleatória gerada pela API, enviada por email ao endereço cadastrado (revoga refresh tokens; `deve_trocar_senha` volta a `true`). Resposta inclui `email_enviado: boolean`.
- **Efeito nas sessões (SEC-08, Lote 8, 2026-09-26):** grava o corte `usuarios.tokens_validos_desde` do usuário resetado. Os access tokens dele emitidos até esse segundo recebem `401` `"sessão encerrada — faça login novamente"` (antes seguiam válidos por até 24h). O token do admin não é afetado.

### Bloqueio de vendedor desligado nas rotas da carteira (2026-09-24)

> **Decisão do usuário (2026-09-24):** o usuário `normal` vinculado a um vendedor desligado (`vendedores.data_desligamento` preenchida) fica bloqueado em **todas as rotas da carteira**, na leitura e na escrita. Contrato definido pelo 🟣 SecBrain e aplicado pelo 🟡 BackBrain em `backend/rotaperfumes-api/handlers/scope.go` (`resolverVendedorScope(r, db)` + `responderErroEscopo`). Desde o Lote 21 (ARQ-04 a ARQ-06), a regra está no `AutorizadorEscopo` (`backend/shared/comercial/comercialapp/escopo.go`), usado por Comercial, Vendas e Analytics; o pacote `handlers` não existe mais.
>
> **Resposta de bloqueio:** `403` com corpo
> ```json
> {"success":false,"error":"acesso bloqueado: vendedor desligado"}
> ```
>
> | Recurso | Rotas bloqueadas para o vendedor desligado |
> |---------|--------------------------------------------|
> | Clientes | `GET /api/clientes`, `GET /api/clientes/{id}`, `POST /api/clientes`, `PUT /api/clientes/{id}`, `PATCH /api/clientes/{id}/inativar` |
> | Pedidos | `GET /api/pedidos`, `GET /api/pedidos/{id}`, `POST /api/pedidos`, `PUT /api/pedidos/{id}`, `DELETE /api/pedidos/{id}` |
> | Pagamentos | `GET /api/pagamentos`, `GET /api/pagamentos/{id}`, `POST /api/pagamentos`, `PUT /api/pagamentos/{id}`, `DELETE /api/pagamentos/{id}` |
> | Oportunidades | `GET /api/oportunidades`, `GET /api/oportunidades/{id}`, `POST /api/oportunidades`, `PUT /api/oportunidades/{id}`, `DELETE /api/oportunidades/{id}` |
> | Visitas | `GET /api/visitas`, `GET /api/visitas/{id}`, `POST /api/visitas`, `PUT /api/visitas/{id}`, `DELETE /api/visitas/{id}` |
> | Carteira do vendedor | `GET /api/vendedores/{id}/clientes` |
> | Lista de vendedores (SEC-03, 2026-09-25) | `GET /api/vendedores` |
>
> - **Continuam acessíveis:** login, `POST /api/auth/refresh`, `GET /api/auth/me` (com `vendedor_desligado: true`) e o Dashboard (`/api/dashboard/*` responde `200` zerado, e `/metrics` traz `vendedor_desligado: true`; ver seção abaixo, **sem mudança** neste lote).
> - **Checagem a cada requisição:** sem cache e sem depender de novo JWT. O desligamento bloqueia imediatamente, e a reativação libera imediatamente.
> - **Admin** não é afetado. **Usuário `normal` sem vendedor vinculado** mantém o comportamento anterior. **Vínculo órfão** (vendedor inexistente) é tratado como "sem vendedor".
> - **Falha ao checar o vendedor no banco:** `500` `"erro interno"` (fail-closed).
> - Na collection, cada pasta (Clientes, Pedidos, Pagamentos, Oportunidades, Visitas) tem uma nota na descrição e o exemplo `403 Forbidden — Vendedor desligado (usuário normal)` na listagem e na criação, e também em "Listar Clientes do Vendedor".
>
> **Datas (BUG-01, 2026-09-24):** o formato das datas nas respostas **não mudou**. A correção (`time.ParseInLocation(..., time.Local)`) só eliminou o deslocamento de um dia na gravação: a data enviada como `AAAA-MM-DD` é gravada e lida no mesmo dia, inclusive em re-salvamentos.
>
> **Fuso fixo (RISCO-01, 2026-09-24):** o processo da API (e o `resetpassword`) passou a usar `time.Local` = fuso fixo `-03:00`, sem horário de verão (pacote `backend/shared/tz`). O formato das datas nas respostas **não mudou**. Datas históricas de início de horário de verão (ex.: `2018-11-04`) não recuam mais um dia em Linux/Docker com `TZ=America/Sao_Paulo`.
>
> **Salvar sem alterações (BUG-04, 2026-09-24):** qualquer `PUT`/`PATCH` que envie os mesmos valores já gravados responde `200` com o registro. Antes respondia `404` "não encontrado", porque o banco (MySQL na época; hoje MariaDB, mesmo comportamento) devolvia `RowsAffected=0`. O DSN agora usa `clientFoundRows=true`. Id inexistente continua `404`. `DELETE /api/vendedores/{id}` em vendedor já desligado responde `200` e preserva a `data_desligamento` original.

### Dashboard (`/api/dashboard/*`) — acesso comum com escopo por vendedor

> **Escopo por vendedor (2026-09-23):** `/api/dashboard/metrics`, `/api/dashboard/vendas` e `/api/dashboard/vendedores` aceitam qualquer usuário autenticado e filtram os números pelo vendedor vinculado ao usuário (`pedidos.vendedor_id`). O filtro é resolvido no handler (`resolverEscopo`):
> - **admin:** vê todos os números, sem filtro.
> - **normal com vendedor vinculado:** vê só os próprios números. `meta_mes` é a meta do próprio vendedor. `top_vendedores`, `metas_vendedores` e o ranking de `/vendedores` trazem no máximo a linha dele.
> - **normal sem vendedor vinculado:** recebe `200` com tudo zerado ou vazio (não é erro). `/metrics` volta zerado, com `top_vendedores: []` e `metas_vendedores: []`. `/vendas` volta com os N dias, todos zerados. `/vendedores` volta com `data: []` e `total=0`, `pages=0`.
> - **normal com vendedor desligado ou inexistente (2026-09-24, decisão do usuário: bloquear o acesso):** se o vendedor vinculado tem `vendedores.data_desligamento` preenchida (ou não existe mais), os quatro endpoints (`/metrics`, `/vendas`, `/vendedores`, `/clientes`) respondem como no caso "sem vendedor vinculado", zerados ou vazios. `/metrics` traz `vendedor_desligado: true` para o frontend mostrar o aviso. Se a checagem do vendedor falhar no banco, a resposta é `500`.
> - **Campo `vendedor_desligado` (bool, só em `/metrics`):** `true` apenas para o usuário normal com vendedor desligado. É `false` em todos os outros casos, inclusive admin, normal com vendedor ativo e normal sem vendedor.
> - **`/api/dashboard/funil` (DASH-01, Lote 22, 2026-10-29):** mesmo esquema de escopo, mas sobre `oportunidades.vendedor_id` (igual a `/api/oportunidades`). Sem vendedor, vendedor desligado ou inexistente → `200` com o funil zerado, sem executar SQL; traz `vendedor_desligado: true` só para o desligado. Ver "GET /api/dashboard/funil" abaixo.
> - **`/api/dashboard/clientes` com escopo da carteira (2026-09-24):** o admin continua com a visão global. O normal vê só os clientes da carteira ativa do próprio vendedor (`carteiras.data_fim IS NULL`). Ver a seção "Clientes" abaixo.
> - **Contrato das listas e dos valores (2026-09-24):** `top_vendedores`, `metas_vendedores`, `por_segmento` e `por_uf` vêm sempre como `[]` quando vazios, nunca `null`. `total_vendas` (no topo, em `top_vendedores[]` e no ranking) e `metas_vendedores[].realizado` são `float64` com centavos, e os percentuais são calculados a partir desse valor.
>
> **Regra de negócio: pedidos de cliente transferido (decisão do usuário, 2026-09-23).**
> - O pedido pertence ao vendedor que o registrou (`pedidos.vendedor_id`), e não ao vendedor que tem o cliente na carteira hoje.
> - Depois que o cliente é transferido, o novo vendedor **não vê** os pedidos antigos desse cliente. Isso vale para pedidos, pagamentos e Dashboard. Só o admin vê todos.
> - O vendedor original continua vendo os pedidos que registrou, mas não consegue editá-los. `PUT /api/pedidos/{id}` retorna `400` "cliente não pertence à carteira deste vendedor", porque a edição valida a carteira ativa.

#### GET /api/dashboard/metrics
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo por vendedor para o normal)
- **Query:** `?periodo=today|week|month&ano=2026&mes=9`
- **Descrição:** Métricas gerais: `total_vendas`, `total_vendas_qtd`, `total_pedidos`, `ticket_medio`, `top_vendedores`, `metas_vendedores` e `meta_mes`.
- **Período `week`:** considera os últimos 7 dias corridos (`DATE_SUB(CURDATE(), INTERVAL 6 DAY)` até hoje), não a semana civil.
- **Campo `meta_mes`:** para o admin, é a soma real de `vendedores.meta_mensal` dos vendedores ativos (`GetMetaMensalTotal`), sem fallback calculado. Para o normal, é a meta do próprio vendedor.
- **Normal sem vendedor:** `200` com todos os valores `0` e as listas `[]` (`EmptyMetrics`), `vendedor_desligado: false`.
- **Normal com vendedor desligado (2026-09-24):** `200` zerado (mesmo `EmptyMetrics`) com `vendedor_desligado: true`.
- **Exemplo (admin, com centavos):** `{ "periodo": "month", "total_vendas": 152300.57, "total_vendas_qtd": 234, "total_pedidos": 234, "ticket_medio": 650.86, "top_vendedores": [{ "id": 7, "nome": "Débora Ribeiro", "total_vendas": 9500.57, "meta": 10000.00, "percentual_meta": 95.0057, "quantidade_vendas": 14 }], "metas_vendedores": [{ "id": 7, ..., "realizado": 9500.57, "percentual": 95.0057 }], "meta_mes": 180000.00, "vendedor_desligado": false }`.

#### GET /api/dashboard/vendas
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo por vendedor para o normal)
- **Query:** `?dias=30` (padrão 30, máx 365)
- **Descrição:** Série temporal de vendas dos últimos N dias. Para o normal, considera só os pedidos do próprio vendedor.
- **Resposta:** `data: { dias: number, pontos: [{ dia: "YYYY-MM-DD", total_vendas: number, total_pedidos: number }] }`. Há um ponto para cada dia do intervalo. Dias sem venda vêm com `total_vendas`/`total_pedidos` zerados, nunca omitidos, para que o gráfico "Vendas nos Últimos 30 Dias" não fique vazio.
- **Normal sem vendedor ou com vendedor desligado:** `200` com os N dias, todos zerados (`EmptyVendasSeries`).

#### GET /api/dashboard/vendedores
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo por vendedor para o normal)
- **Query:** `?page=1&limit=20`
- **Descrição:** Ranking de vendedores (total de vendas, meta, percentual). O normal recebe no máximo a própria linha.
- **Ordenação:** por `meta_mensal DESC`, com desempate por `atingimento_meta DESC` (calculado no SQL, antes da paginação).
- **Normal sem vendedor ou com vendedor desligado:** `200` com `data: []` e paginação `total=0`, `pages=0`.

#### GET /api/dashboard/funil
- **Desde:** DASH-01, Lote 22 (2026-10-29). Handler `DashboardHandler.GetFunil` (`backend/rotaperfumes-api/internal/analyticshttp/dashboard_handler.go`), caso de uso `Consultas.Funil` (`backend/shared/analytics/analyticsapp/consultas.go`), domínio `backend/shared/analytics/{janela,funil}.go`, SQL em `backend/shared/analytics/analyticsmysql/oportunidades.go`.
- **Auth:** Bearer Token (qualquer usuário autenticado, Policy `Authenticated`; escopo por vendedor para o normal).
- **Query:** `?janela=30d|90d|365d|all` (padrão: `all`; vazio também vale `all`).
  - Filtra por `oportunidades.data_abertura` relativa a `CURDATE()`, incluindo hoje: `30d` = `INTERVAL 29 DAY`, `90d` = `INTERVAL 89 DAY`, `365d` = `INTERVAL 364 DAY`; `all` não tem cláusula de data.
  - Qualquer outro valor (inclusive maiúsculas, como `30D`) → `400` `{"success":false,"error":"janela deve ser '30d', '90d', '365d' ou 'all'"}`, sem consultar o banco.
  - A janela é um enum tipado traduzido para constantes SQL; o texto do usuário nunca entra no SQL. Não reusa o `periodo` dos outros endpoints porque a base de oportunidades vai até 2026-08-31 (`today`/`week`/`month` ficariam vazios).
- **Escopo:**
  - **admin:** todas as oportunidades.
  - **normal com vendedor ativo:** só as oportunidades do próprio vendedor (`oportunidades.vendedor_id`), resolvido pelo `AutorizadorEscopo` e traduzido só por `Escopo.Filtro()`. Não há `vendedor_id` na query.
  - **normal sem vendedor, com vendedor desligado ou inexistente:** `200` com o funil zerado (5 etapas com zero), **sem executar SQL**. `vendedor_desligado: true` só para o desligado.
  - Erro ao resolver o escopo → `500` `"erro interno"` (fail-closed). Sem cache: o escopo é resolvido a cada requisição.
- **Resposta:** `data: { janela, total_oportunidades, etapas: [{ etapa, quantidade, valor_estimado, conversao_anterior_pct, conversao_topo_pct }], perdidas: { quantidade, valor_estimado }, taxa_ganho_pct, vendedor_desligado }`.
  - `etapas`: sempre 5, na ordem Prospecção → Qualificação → Proposta enviada → Negociação → Fechado ganho. Funil **cumulativo**: cada etapa conta as oportunidades cuja etapa atual está nela ou adiante (Fechado perdido não entra).
  - `conversao_anterior_pct` e `conversao_topo_pct`: na primeira etapa, `100` com dados e `0` sem dados. Base zero → `0`. Duas casas decimais.
  - `perdidas`: oportunidades em Fechado perdido.
  - `taxa_ganho_pct` = ganhas / (ganhas + perdidas) × 100; `0` sem oportunidades fechadas.
  - `total_oportunidades`: todas as oportunidades da janela, inclusive perdidas e etapas desconhecidas. Etapas desconhecidas só entram no total.
- **Exemplo (admin, `janela=90d`):** `{"data":{"janela":"90d","total_oportunidades":112,"etapas":[{"etapa":"Prospecção","quantidade":97,"valor_estimado":325000.75,"conversao_anterior_pct":100,"conversao_topo_pct":100},{"etapa":"Qualificação","quantidade":57,"valor_estimado":225000.25,"conversao_anterior_pct":58.76,"conversao_topo_pct":58.76},{"etapa":"Proposta enviada","quantidade":32,"valor_estimado":145000,"conversao_anterior_pct":56.14,"conversao_topo_pct":32.99},{"etapa":"Negociação","quantidade":17,"valor_estimado":85000,"conversao_anterior_pct":53.13,"conversao_topo_pct":17.53},{"etapa":"Fechado ganho","quantidade":7,"valor_estimado":35000,"conversao_anterior_pct":41.18,"conversao_topo_pct":7.22}],"perdidas":{"quantidade":13,"valor_estimado":42000.1},"taxa_ganho_pct":35,"vendedor_desligado":false},"success":true}`. Aqui, 112 = 97 no funil + 13 perdidas + 2 em etapa desconhecida.
- **Erros:** `400` janela inválida; `401` não autenticado; `405` outro método (com `Allow`); `500` erro interno (mensagem genérica).

### Histórico de Senhas (`/api/senha-historico/*`) — admin only

#### GET /api/senha-historico
- **Auth:** Bearer Token (admin)
- **Query:** `?page=1&limit=20&order_by=created_at&order_dir=desc`
- **Descrição:** Lista global paginada de alterações de senha (com tipo, IP, user agent)
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, usuario_id, tipo_reset, created_at, usuario_nome, resetado_por_nome, ip_origem` (default: `id`; as três últimas desde o FE-13, Lote 9); `order_dir` aceita `asc`|`desc` case-insensitive (default: `desc`). Valor inválido/ausente cai silenciosamente em `id` (sem erro 400). Quando a coluna não é `id`, desempate por `id DESC`. `resetado_por_nome` nulo vem primeiro em `asc` e por último em `desc`.
- **Campos NULL no banco (BUG-12, Lote 10):** `ip_origem`/`user_agent` podem vir `""` e `created_at` pode vir `"0001-01-01T00:00:00Z"` (antes a listagem dava `500`). Vale também para a rota por usuário.

#### GET /api/senha-historico/{usuario_id}
- **Auth:** Bearer Token (admin)
- **Query:** `?page=1&limit=20&order_by=created_at&order_dir=desc`
- **Descrição:** Histórico de senhas de um usuário específico
- **Ordenação:** mesmas regras de `GET /api/senha-historico` acima (`order_by`/`order_dir`).

### Clientes (`/api/clientes/*` e `/api/dashboard/clientes`) — **acesso comum, com escopo por carteira**

> Base de clientes importada de `dados/crm/clientes.csv` para a tabela `clientes` (ver seção "Importação de clientes (CRM)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Clientes"** da collection.
>
> **Vendedor desligado (2026-09-24):** as 5 rotas `/api/clientes*` respondem `403` `"acesso bloqueado: vendedor desligado"` para o usuário `normal` com vendedor desligado. Ver a seção "Bloqueio de vendedor desligado nas rotas da carteira".
>
> **Acesso comum com escopo por carteira:** todas as rotas `/api/clientes*` têm a política `Authenticated` (qualquer usuário autenticado; tabela de rotas em `backend/rotaperfumes-api/internal/bootstrap/rotas.go`). O escopo é resolvido pelo `AutorizadorEscopo` (`backend/shared/comercial/comercialapp/escopo.go`) e aplicado nos casos de uso de `comercialapp`; o handler fica em `backend/rotaperfumes-api/internal/comercialhttp/cliente_handler.go` (até o Lote 21, `backend/rotaperfumes-api/handlers/cliente_handler.go`). O admin não sofre restrições.
>
> **Escopo nas escritas (SEC-01, 2026-09-24):** antes, qualquer usuário `normal` conseguia editar ou inativar qualquer cliente pelo id. Agora (helper `autorizarEscritaCliente`, checado antes de ler o body):
>
> | Rota | `normal` com vendedor | `normal` sem vendedor | Admin |
> |------|-----------------------|-----------------------|-------|
> | `POST /api/clientes` | `201`; o cliente é vinculado automaticamente à carteira do vendedor (`data_inicio` = hoje), na mesma transação (`ClienteService.CreateClienteNaCarteira`) | `403` `"usuário sem vendedor vinculado"` | `201`, sem vínculo de carteira (sem mudança) |
> | `PUT /api/clientes/{id}` | `404` `{"success":false,"error":"cliente não encontrado"}` fora da carteira ativa | `404` `"cliente não encontrado"` | sem restrição |
> | `PATCH /api/clientes/{id}/inativar` | `404` `"cliente não encontrado"` fora da carteira ativa | `404` `"cliente não encontrado"` | sem restrição |
>
> - O `404` fora da carteira usa a mesma mensagem do cliente inexistente, para não revelar clientes de outras carteiras.
> - Falha ao checar a carteira no banco: `500` `"erro interno"`.
> - Vendedor desligado: `403` `"acesso bloqueado: vendedor desligado"`, como antes.
> - **Leitura:** `GET /api/clientes` devolve só a carteira ativa do vendedor (lista vazia para usuário sem vendedor). `GET /api/clientes/{id}` fora da carteira devolve `404`.
>
> **Salvar sem alterações (BUG-04, 2026-09-24):** `PUT`/`PATCH` com os mesmos valores já gravados responde `200`. Antes respondia `404` "não encontrado".
>
> **CNPJ validado e único (NEG-01, 2026-09-25):** `POST`/`PUT` validam o CNPJ e o banco tem índice UNIQUE `uq_clientes_cnpj` (migração `sql/19_alter_clientes_cnpj_unique.sql`). Os clientes duplicados foram unificados no de menor id, com os registros vinculados transferidos (decisão do usuário, 2026-09-25). A migração está aplicada no banco local desde 2026-09-25 (índice `uq_clientes_cnpj` ativo), então o `409` é disparado pelo banco. Em um banco novo, aplique com `make db-fix-cnpj-unique`; sem ela, o `409` não acontece.
>
> **Body do PATCH inativar (BUG-06, 2026-09-25):** body preenchido e inválido → `400` `"body JSON inválido"`, mas só depois da checagem de escopo: sem acesso ao registro continua `404`.

#### GET /api/clientes
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo pela carteira para o `normal`)
- **Query (todos opcionais):** `?page=1&limit=20&uf=SP&segmento=Varejo&ativo=true&q=perfumaria&order_by=razao_social&order_dir=asc`
- **Descrição:** Lista clientes paginada (total + pages), com filtros exatos por `uf`/`segmento`, filtro por status (`ativo=true|false`) e busca livre (`q`) em `razao_social` OU `cnpj`
- **Busca por CNPJ (NEG-02, 2026-09-25):** o `q` pode vir com máscara e em maiúsculas ou minúsculas. Quando o `q` tem caracteres de máscara (`.`, `/`, `-`) e só caracteres de CNPJ, a API tira a máscara e converte para maiúsculas antes de buscar em `cnpj` (`ClienteFiltro.QCNPJ`); a busca em `razao_social` continua usando o `q` como veio (um nome com hífen continua sendo encontrado). Sem máscara, o `q` é usado direto, e a collation `utf8mb4_unicode_ci` ignora a caixa. A busca é por trecho (`LIKE`), então um prefixo como `12.abc.345` também encontra. Ex.: `?q=12ABC34501DE35`, `?q=12abc34501de35`, `?q=12.ABC.345/01DE-35` e `?q=12.abc.345` encontram o mesmo cliente.
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, razao_social, cnpj, segmento, cidade, uf, data_cadastro, ativo, created_at, updated_at` (default: `id`; `id` é um alias de coluna aceito pela API, mapeado para `cliente_id_origem` — não existe mais campo `id` na resposta); `order_dir` aceita `asc`|`desc` case-insensitive (default: `asc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/clientes
- **Auth:** Bearer Token (qualquer usuário autenticado; `normal` precisa ter vendedor vinculado)
- **Escopo (SEC-01):** usuário `normal` sem vendedor → `403` `"usuário sem vendedor vinculado"`. Com vendedor → `201`, e o cliente é vinculado automaticamente à carteira do vendedor. Admin: sem mudança.
- **Body:** `{ "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro", "data_cadastro" (opcional, "AAAA-MM-DD", default hoje) }`
- **Descrição:** Cria um novo cliente. `cliente_id_origem` é a PK `BIGINT AUTO_INCREMENT` da tabela, gerada nativamente pelo MariaDB (não é aceita no body), e `ativo` é sempre `true` na criação. Campos obrigatórios: `razao_social`, `cnpj`, `segmento`, `cidade`, `uf` (2 letras). Retorna `201` com o cliente criado; `400` em caso de validação. Não existe mais campo `id` — `cliente_id_origem` é o único identificador.
- **Timestamps (BUG-08, Lote 6, 2026-09-26):** o `201` traz `created_at` e `updated_at` preenchidos, porque o cliente é relido do banco depois do INSERT (`relerClienteCriado`, com ou sem vínculo de carteira). Antes vinham `0001-01-01T00:00:00Z`. Os demais creates (vendedores, produtos, pagamentos, oportunidades, visitas e estoque) passaram a reler no Lote 8 (BUG-09); o de usuários já relia no repositório.
- **CNPJ (NEG-01 + NEG-02, 2026-09-25):** aceita o CNPJ numérico e o **alfanumérico** da Receita (vigente desde julho de 2026).
  - **Envio:** com ou sem máscara (`.`, `/`, `-` e espaço), em maiúsculas ou minúsculas. Ex.: `12ABC34501DE35`, `12.ABC.345/01DE-35`, `12.abc.345/01de-35`, `11222333000181` ou `11.222.333/0001-81`.
  - **Formato:** 14 caracteres; as 12 primeiras posições são `[0-9A-Z]` e as 2 últimas (DVs) são numéricas.
  - **DV:** módulo 11 com os pesos do CNPJ numérico (5,4,3,2,9,8,7,6,5,4,3,2 para o 1º DV e 6,5,4,3,2,9,8,7,6,5,4,3,2 para o 2º). Cada caractere vale o código ASCII − 48 (`0`–`9` → 0–9, `A` → 17, …, `Z` → 42). Resto < 2 → DV 0; senão DV = 11 − resto. Para CNPJ só com dígitos, o cálculo é o mesmo de antes.
  - **Retorno e gravação:** sempre 14 caracteres, sem máscara e em **maiúsculas** (ex.: enviar `12.abc.345/01de-35` grava e devolve `12ABC34501DE35`).
  - **Erros (sem mudança):** `400` `"cnpj inválido"` para caractere fora de `[0-9A-Za-z]` e da máscara (ex.: `#`, `_`, acentos), letra nas posições do DV, tamanho diferente de 14, todos os caracteres iguais ou DV errado. `409` `"cnpj já cadastrado"` se outro cliente já usa o CNPJ (a comparação ignora máscara e caixa, porque o valor é normalizado antes); a mensagem é genérica e não revela qual cliente.
  - **Exemplos:** `12ABC34501DE35` → `201`; `12ABC34501DE36` (DV errado) → `400`; `12ABC34501DEA5` (letra no DV) → `400`; `12ABC34501DE3#` → `400`.
- **Nota histórica (resolvida):** versões anteriores geravam `cliente_id_origem` via `MAX(cliente_id_origem) + 1` sem transação/lock explícito, com risco teórico de colisão em criações concorrentes. Esse débito técnico foi eliminado na tarefa "Promover colunas \*_id_origem a PK autoincremento" (2026-09-15) — a geração agora é feita nativamente pelo banco (MariaDB desde o DB-03) via `AUTO_INCREMENT`.

#### GET /api/clientes/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado; `404` fora da carteira para o `normal`)
- **Descrição:** Retorna o detalhe de um cliente pelo `cliente_id_origem` (PK da tabela; o path param continua se chamando `id` na rota, mas não existe mais campo `id` separado na resposta)

#### PUT /api/clientes/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo pela carteira ativa para o `normal`)
- **Escopo (SEC-01):** usuário `normal` fora da carteira ativa, ou sem vendedor → `404` `"cliente não encontrado"`. Falha ao checar a carteira → `500`. Vendedor desligado → `403`.
- **Sem alterações (BUG-04):** `200`.
- **Body:** `{ "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro", "data_cadastro" ("AAAA-MM-DD") }`
- **Descrição:** Atualiza os dados de um cliente existente. `cliente_id_origem` e `ativo` **não** são editáveis por esta rota (use `PATCH /api/clientes/{id}/inativar` para alterar `ativo`). Retorna `200` com o cliente atualizado, `404` se não existir, `400` se o payload for inválido.
- **CNPJ (NEG-01 + NEG-02, 2026-09-25):** mesma regra e normalização do `POST`: numérico ou alfanumérico, máscara e minúsculas aceitas, grava e devolve 14 caracteres sem máscara e em maiúsculas. **Toda gravação exige CNPJ válido (NEG-04, 2026-09-25):** formato (14 caracteres, 12 primeiras `[0-9A-Z]` e 2 DVs numéricos) e dígito verificador, mesmo quando o CNPJ enviado é igual ao atual. DV inválido → `400` `"cnpj inválido"`, sem consultar o banco e sem gravar; por isso, id inexistente com DV inválido responde `400`, e não `404` (o `404` vem do UPDATE com 0 linhas, sem SELECT prévio). Clientes legados com DV inválido só podem ser editados depois de corrigir o CNPJ (antes do NEG-04, o DV só era exigido se o CNPJ mudasse). `PATCH /api/clientes/{id}/inativar` não valida o CNPJ. `409` `"cnpj já cadastrado"` se o novo CNPJ pertence a outro cliente. O escopo é checado antes: sem acesso, `404` mesmo com CNPJ inválido.

#### PATCH /api/clientes/{id}/inativar
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo pela carteira ativa para o `normal`)
- **Body (opcional):** `{ "ativo": true|false }` — omitido = toggle
- **Descrição:** Ativa ou inativa o cliente
- **Escopo (SEC-01):** usuário `normal` fora da carteira ativa, ou sem vendedor → `404` `"cliente não encontrado"`. Falha ao checar a carteira → `500`. Vendedor desligado → `403`.
- **Mesmo valor (BUG-04):** enviar `ativo` igual ao estado atual responde `200`.
- **Body inválido (BUG-06, 2026-09-25):** vazio, `null`, `{}` ou `{"ativo":null}` → toggle; `{"ativo":true|false}` → define o valor; body preenchido e inválido (JSON malformado, `{"ativo":"x"}`, `{"ativo":1}`, `[true]`) → `400` `"body JSON inválido"`. O body só é lido **depois** da checagem de escopo: sem acesso ao registro, a resposta continua `404` (SEC-01).

#### GET /api/dashboard/clientes
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo pela carteira do vendedor para o normal). A rota tem a política `Authenticated` (antes do Lote 21, `JWTMiddleware(cfg, true, false)`), então não há `403`.
- **Query:** `?periodo=today|week|month` (padrão: `month`)
- **Descrição:** Métricas agregadas da base de clientes: `total_clientes`, `total_ativos`, `total_inativos`, `novos_no_periodo`, `por_segmento` (array `{segmento, total}`) e `por_uf` (array `{uf, total}`)
- **Escopo (decisão do usuário, 2026-09-24):**
  - **admin:** visão global da base de clientes.
  - **normal com vendedor ativo:** só os clientes da carteira ativa do próprio vendedor (`carteiras.data_fim IS NULL`).
  - **normal sem vendedor, com vendedor desligado ou com vendedor inexistente:** `200` zerado: `{"periodo":"month","total_clientes":0,"total_ativos":0,"total_inativos":0,"novos_no_periodo":0,"por_segmento":[],"por_uf":[]}`.
- **Listas:** `por_segmento` e `por_uf` vêm sempre como `[]` quando vazias, nunca `null`.
- **Período `week`:** considera os últimos 7 dias corridos (`DATE_SUB(CURDATE(), INTERVAL 6 DAY)` até hoje), não a semana civil.

### Produtos (`/api/produtos/*`) — GET acesso comum, POST/PUT/PATCH admin only

> Base de produtos importada de `dados/erp/produtos.csv` (293 linhas) para a tabela `produtos` (ver seção "Importação de produtos (ERP)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Produtos"** da collection. A exclusão de produtos é sempre **lógica** (flag `ativo`) — não existe endpoint de `DELETE`.
>
> **Assimetria de acesso (2026-09-22):** `GET /api/produtos` e `GET /api/produtos/{id}` são **acesso comum** (qualquer usuário autenticado — admin ou normal), mesma cadeia de middleware usada em Pagamentos (`cfg, true, false`) — mantidos abertos porque o vendedor precisa consultar o catálogo para montar um pedido (`PedidoModal.tsx`). `POST /api/produtos`, `PUT /api/produtos/{id}` e `PATCH /api/produtos/{id}/inativar` continuam **admin only** (`cfg, true, true`) — só admin pode criar, editar ou ativar/inativar produtos. (Estoque, diferente de Produtos, teve **todas** as rotas — incluindo GET — restringidas a admin only na mesma data; ver seção "Estoque" abaixo.)

#### GET /api/produtos
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Query (todos opcionais):** `?page=1&limit=20&categoria=Masculino&marca=Rota&ativo=true&q=intense&order_by=descricao&order_dir=asc`
- **Descrição:** Lista produtos paginada (total + pages), com filtros exatos por `categoria`/`marca`, filtro por status (`ativo=true|false`) e busca livre (`q`) em `descricao` OU `sku`
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, sku, descricao, categoria, marca, preco_tabela, custo_unitario, data_lancamento, ativo, created_at, updated_at` (default: `id`); `order_dir` aceita `asc`|`desc` case-insensitive (default: `asc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/produtos
- **Auth:** Bearer Token (admin) — usuário `normal` recebe `403`
- **Body:** `{ "sku", "descricao", "categoria", "marca", "nota_olfativa", "preco_tabela", "custo_unitario", "unidade", "data_lancamento" (opcional, "AAAA-MM-DD") }`
- **Descrição:** Cria um novo produto. `ativo` é sempre `true` na criação — não é aceito no body. Campos obrigatórios: `sku`, `descricao`, `categoria`, `marca`, `unidade`; `preco_tabela`/`custo_unitario` devem ser `>= 0`. Retorna `201` com o produto criado; `400` em caso de validação; `409` `"sku já cadastrado"` se o SKU já existir (CHORE-06, Lote 25; antes `500`).

#### GET /api/produtos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Descrição:** Retorna o detalhe de um produto pelo `id` interno

#### PUT /api/produtos/{id}
- **Auth:** Bearer Token (admin) — usuário `normal` recebe `403`
- **Body:** `{ "descricao", "categoria", "marca", "nota_olfativa", "preco_tabela", "custo_unitario", "unidade", "data_lancamento" (opcional, "AAAA-MM-DD") }`
- **Descrição:** Atualiza os dados de um produto existente. `sku` e `ativo` **não** são editáveis por esta rota (use `PATCH /api/produtos/{id}/inativar` para alterar `ativo`). Retorna `200` com o produto atualizado, `404` se não existir, `400` se o payload for inválido.

#### PATCH /api/produtos/{id}/inativar
- **Auth:** Bearer Token (admin) — usuário `normal` recebe `403`
- **Body (opcional):** `{ "ativo": true|false }` — omitido = toggle
- **Descrição:** Ativa ou inativa o produto (exclusão lógica)
- **Body inválido (BUG-06, 2026-09-25):** vazio, `null`, `{}` ou `{"ativo":null}` → toggle; `{"ativo":true|false}` → define o valor; body preenchido e inválido (JSON malformado, `{"ativo":"x"}`, `{"ativo":1}`, `[true]`) → `400` `"body JSON inválido"`, sem alterar o produto.
- **Toggle atômico (BUG-17, Lote 25, 2026-10-30):** sem valor explícito, a API grava `UPDATE produtos SET ativo = NOT ativo WHERE id = ?` (sem ler antes), então dois toggles concorrentes não se anulam. Com valor explícito, grava `SET ativo = ?`. Nos dois casos o produto é relido para a resposta (o `updated_at` é o real). `404` se não existir. Na tela, o botão fica bloqueado por 1,5 s depois de cada clique. Clientes e usuários ainda leem e invertem (card BUG-22).

> **Campo `imagem_url` (IMG-01, Lote 26, 2026-10-31):** todas as respostas com produto trazem `imagem_url`, que é `"/api/public/produtos/imagens/<uuid-v4>.<jpg|png|webp>"` (caminho relativo à base da API) ou `null` quando o produto não tem imagem. O frontend só exibe a imagem se o valor começar com esse prefixo.

#### POST /api/produtos/{id}/imagem
- **Auth:** Bearer Token (admin); usuário `normal` recebe `403`
- **Headers:** `X-Requested-With: XMLHttpRequest` (**obrigatório**, anti-CSRF). Se houver `Origin`, ela precisa ser a do próprio host ou uma das origens CORS permitidas.
- **Body:** `multipart/form-data` com **exatamente uma** parte, `imagem`. Outra parte, segunda parte, outro nome de campo, corpo vazio ou corpo que não é multipart → `400`.
- **Ordem das checagens:** policy (`401`/`403`) → id (`400` `"id inválido"`) → anti-CSRF (`403` `"requisição de upload recusada"`) → corpo (`413`/`400`) → produto (`404`).
- **Validação pelo conteúdo** (o nome, a extensão e o `Content-Type` enviados pelo cliente são ignorados): magic bytes (`http.DetectContentType`) → cabeçalho da imagem (`DecodeConfig`) → limites (1 a 4096 px por lado, largura × altura ≤ 16.000.000) → decodificação completa. JPEG e PNG são **reencodados** (EXIF e bytes anexados saem); WebP é gravado como veio (card SEC de backlog).
- **Gravação:** nome `<uuid-v4>.<ext>` (UUID via `crypto/rand`, extensão do tipo detectado), escrita atômica (arquivo temporário + rename) em `PRODUTO_IMAGENS_DIR`. Banco em transação com `FOR UPDATE`; a imagem antiga só é apagada depois do commit.
- **Resposta:** `200` `{"success":true,"data":<produto>}`, com `imagem_url` preenchido.
- **Erros:** `400` `"id inválido"`; `400` `"arquivo de imagem inválido (use JPEG, PNG ou WebP)"`; `401`; `403` `"acesso restrito a administradores"`; `403` `"requisição de upload recusada"`; `404` `"produto não encontrado"` (o arquivo gravado é apagado); `413` `"imagem excede 1 MB"` (limite de 1.048.576 bytes; `Content-Length` acima de 1 MB + 64 KB é recusado sem ler o corpo; alguns envios chunked grandes recebem `400`, aceito pelo 🤍 MegaBrain).

#### DELETE /api/produtos/{id}/imagem
- **Auth:** Bearer Token (admin); usuário `normal` recebe `403`
- **Descrição:** grava `imagem = NULL` (transação com `FOR UPDATE`) e apaga o arquivo depois do commit. **Idempotente:** produto sem imagem também responde `200`.
- **Resposta:** `200` com o produto e `"imagem_url": null`.
- **Erros:** `400` `"id inválido"`; `401`; `403`; `404` `"produto não encontrado"`.

#### GET /api/public/produtos/imagens/{arquivo}
- **Auth:** nenhuma (rota pública; não lê token nem cookie e não envia `Set-Cookie`)
- **Path:** `arquivo` = `<uuid-v4>.jpg|.png|.webp` em minúsculas. Nome fora do padrão → `404` sem tocar o disco.
- **Resposta:** `200` com os bytes. `Content-Type` de mapa fixo (`image/jpeg`, `image/png`, `image/webp`), `Cache-Control: public, max-age=31536000, immutable`, `Cross-Origin-Resource-Policy: same-site`, `Content-Disposition: inline`, mais `X-Content-Type-Options: nosniff`, CSP e `X-Frame-Options` globais. `HEAD` → `200` sem corpo; `Range` → `206`. Outros métodos → `405`.
- **Erros:** `404` `"imagem não encontrada"`. `..` literal no caminho recebe `307` do roteador para fora da rota (nada vaza; aceito).
- **Dev:** use o mesmo host (`localhost`) no frontend e na API, por causa do `Cross-Origin-Resource-Policy: same-site`.

### Pedidos (`/api/pedidos/*`) — **acesso comum, com escopo por carteira** (atualizado em 2026-09-23)

> Tela master-detail: lista de pedidos + sub-lista de itens (produtos) de cada pedido. Base importada de `dados/erp/pedidos.csv` (28.729 linhas) e `dados/erp/itens_pedido.csv` (197.724 linhas) para as tabelas `pedidos`/`itens_pedido` (ver seção "Importação de pedidos (ERP)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Pedidos"** da collection.
>
> **Escopo por carteira em Create/Update (2026-09-23):** todas as rotas de Pedidos têm a política `Authenticated` (qualquer usuário autenticado). A restrição de dono é aplicada pelo escopo de carteira (na época, no handler `backend/rotaperfumes-api/handlers/pedido_handler.go`; desde o Lote 21, ARQ-05, nos casos de uso de `backend/shared/vendas/vendasapp`, com o handler em `backend/rotaperfumes-api/internal/vendashttp/pedido_handler.go`; mesma regra de Oportunidades/Visitas). Para usuário `normal`:
> - **`POST /api/pedidos`:** `403` (`"usuário sem vendedor vinculado"`) se o usuário não tiver vendedor vinculado; `vendedor_id` do payload é **ignorado** e forçado ao vendedor do usuário logado; `cliente_id` precisa pertencer à carteira ativa (`data_fim IS NULL`) desse vendedor, senão `400` (`"cliente não pertence à carteira deste vendedor"`).
> - **`PUT /api/pedidos/{id}`:** `404` (`"pedido não encontrado"`, corpo idêntico ao de pedido inexistente) se o usuário não tiver vendedor vinculado ou se o pedido pertencer a outro vendedor — evita enumeração de IDs; `vendedor_id` do payload é forçado ao vendedor do usuário (não é possível reatribuir o pedido); `cliente_id` fora da carteira → `400`.
> - **Efeito colateral aceito:** se o cliente de um pedido antigo tiver sido transferido para outro vendedor, o usuário `normal` não consegue mais editar esse pedido (`400` de carteira). Admin não é afetado.
> - Usuário `admin` não sofre nenhuma dessas restrições.
>
> **Botão de exclusão (2026-09-22):** adicionado endpoint `DELETE /api/pedidos/{id}` — hard delete (sem exclusão lógica). Ver detalhes abaixo.

#### GET /api/pedidos
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Query (todos opcionais):** `?page=1&limit=20&status=Faturado&canal=App&cliente_id=1&vendedor_id=1&data_inicio=2026-01-01&data_fim=2026-12-31&q=perfumaria&order_by=data_pedido&order_dir=desc`
- **Descrição:** Lista pedidos paginada (total + pages), com filtros exatos por `status` (`Cancelado`|`Em separação`|`Entregue`|`Faturado`), `canal` (`App`|`Telefone`|`Visita`|`WhatsApp`), `cliente_id`, `vendedor_id`, intervalo `data_inicio`/`data_fim` (`AAAA-MM-DD`) e busca livre (`q`) pela razão social do cliente. Cada item traz o cabeçalho do pedido enriquecido com `cliente_nome`/`vendedor_nome`, sem os itens.
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, data_pedido, canal, status, valor_total, created_at, updated_at, cliente_nome, vendedor_nome` (default: `id`; `id` é um alias de coluna aceito pela API, mapeado para `pedido_id_origem` — não existe mais campo `id` na resposta); `order_dir` aceita `asc`|`desc` case-insensitive (default: `desc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/pedidos
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Body:** `{ "cliente_id", "vendedor_id", "data_pedido" ("AAAA-MM-DD"), "canal", "status", "itens": [{ "produto_id", "quantidade", "preco_praticado", "desconto_pct" }] }`
- **Descrição:** Cria um novo pedido com seus itens. `valor_bruto` de cada item e `valor_total` do pedido são calculados no backend (não aceitos no body). `pedido_id_origem` é a PK `BIGINT AUTO_INCREMENT` gerada nativamente pelo MariaDB. Exige ao menos um item. Retorna `201` com o pedido criado (incluindo itens); `400` em caso de validação. Não existe mais campo `id` — `pedido_id_origem`/`item_id_origem` são os identificadores.
- **Escopo (usuário `normal`, 2026-09-23):** `403` `"usuário sem vendedor vinculado"` se não houver vendedor vinculado; `vendedor_id` do body é ignorado e forçado ao vendedor do usuário; `400` `"cliente não pertence à carteira deste vendedor"` se `cliente_id` estiver fora da carteira ativa.
- **Status na criação (BUG-20, Lote 25, 2026-10-30):** criado já como `Faturado`, o pedido baixa o estoque de cada item na mesma transação do INSERT (a mesma baixa da transição no `PUT`); se a baixa falhar, nada é gravado. Criado como `Entregue` → `400` `"pedido só pode ser marcado como Entregue depois de Faturado"`, sem gravar nada (extensão do 🤍 MegaBrain: criar direto em Entregue pularia a baixa). `Em separação` e `Cancelado` não baixam. Na tela, a criação não oferece Entregue.

#### GET /api/pedidos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira; `404` para pedido de outro vendedor)
- **Descrição:** Retorna o detalhe de um pedido pelo `pedido_id_origem` (PK; o path param continua se chamando `id` na rota), incluindo a lista de itens (`itens`, cada um enriquecido com `produto_sku`/`produto_descricao` via JOIN) — usado na tela master-detail.

#### PUT /api/pedidos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Body:** mesmo formato do `POST /api/pedidos`
- **Descrição:** Atualiza os dados de um pedido existente e substitui integralmente a lista de itens (delete + insert). `valor_bruto`/`valor_total` são recalculados no backend. Retorna `200` com o pedido atualizado (incluindo itens), `404` se não existir, `400` se o payload for inválido.
- **Escopo (usuário `normal`, 2026-09-23):** `404` `"pedido não encontrado"` (mesmo corpo de inexistente) se o usuário não tiver vendedor vinculado ou o pedido for de outro vendedor; `vendedor_id` do body é forçado ao vendedor do usuário; `400` `"cliente não pertence à carteira deste vendedor"` se `cliente_id` estiver fora da carteira ativa.
- **Máquina de status (BUG-16, Lote 25, 2026-10-30):** checada dentro da transação, sob `SELECT status … FOR UPDATE`, **antes** da regra de itens. A recusa não grava nada.

  | Status gravado | Destinos aceitos | Recusa (`400`) |
  |---|---|---|
  | Em separação / Cancelado | Em separação, Cancelado, Faturado (baixa o estoque) | Entregue → `"pedido só pode ser marcado como Entregue depois de Faturado"` (extensão do 🤍 MegaBrain) |
  | Faturado | Faturado (idempotente), Entregue (sem nova baixa) | qualquer outro → `"pedido faturado só pode ter o status alterado para Entregue"` |
  | Entregue (terminal) | Entregue (idempotente) | qualquer outro → `"pedido entregue não pode ter o status alterado"` |

  Nenhuma transição volta para Faturado, então o estoque é baixado uma vez só (antes, Faturado → outro → Faturado baixava duas vezes). Em pedido Faturado ou Entregue, os itens enviados precisam ser iguais aos gravados; itens diferentes → `409` `"pedido já faturado: não é possível alterar os itens, apenas o status"` (texto mantido; revisão no card CHORE-07). Na tela, o select de Status oferece só as transições permitidas, os itens ficam travados e o botão Excluir fica desabilitado em Faturado e Entregue.

#### DELETE /api/pedidos/{id} (adicionado em 2026-09-22)
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Descrição:** Exclui definitivamente um pedido e seus itens (hard delete, sem soft-delete). Bloqueado com `409` se o pedido tiver **pagamentos vinculados** (`"pedido possui pagamentos vinculados"`; corrigido no Lote 25: a documentação mostrava um sufixo que a API não envia), se já estiver com `status = "Faturado"` (`"pedido faturado não pode ser excluído, apenas ter o status alterado"` — pelo BUG-16, a única alteração permitida é para Entregue, via `PUT`) ou se estiver `Entregue` (`"pedido entregue não pode ser excluído"`; BUG-16, Lote 25, extensão do 🤍 MegaBrain). A checagem de pagamentos vem primeiro. Escopo idêntico ao `GET`/`PUT`: `404` (não `403`) se o pedido pertencer a outro vendedor. Retorna `204` sem corpo em caso de sucesso.

### Pagamentos (`/api/pagamentos/*`) — **acesso comum, com escopo por carteira**

> **Pagamentos é liberado a qualquer usuário autenticado — `admin` ou `normal`**, como Clientes, Pedidos, Oportunidades e Visitas. A política da rota é `Authenticated` (antes do Lote 21, `middleware.JWTMiddleware(cfg, true, false)`); as rotas admin only (Usuários, Estoque, escrita de Produtos, gestão de Vendedores) usam `AdminOnly` (antes, `(cfg, true, true)`). Tabela de rotas: `backend/rotaperfumes-api/internal/bootstrap/rotas.go`. Base importada de `dados/erp/pagamentos.csv` (~27,7 mil linhas) para a tabela `pagamentos` (ver seção "Importação de pagamentos (ERP)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Pagamentos"** da collection e usam `{{vendedor_token}}` (usuário `normal`) nos exemplos, em vez de `{{admin_token}}`, para deixar explícito que o acesso é comum.
>
> **Botão de exclusão (2026-09-22):** adicionado endpoint `DELETE /api/pagamentos/{id}` — hard delete (sem exclusão lógica). Ver detalhes abaixo.
>
> **Escopo por carteira em Create/Update (2026-09-23):** o mesmo bypass corrigido em Pedidos existia em Pagamentos (apontado pelo 🟣 SecBrain). Agora, para usuário `normal`, o handler (`pagamento_handler.go`, helper `pedidoNoEscopo`) valida que o pedido do pagamento pertence ao vendedor do usuário logado:
> - **`POST /api/pagamentos`:** `403` (`"usuário sem vendedor vinculado"`) se o usuário não tiver vendedor vinculado; `404` (`"pedido não encontrado"`) se `pedido_id` for de outro vendedor **ou** não existir.
> - **`PUT /api/pagamentos/{id}`:** `404` (`"pagamento não encontrado"`) se o usuário não tiver vendedor vinculado ou se o pagamento pertencer a pedido de outro vendedor.
> - **Status padronizado (2026-09-24):** `POST` com `pedido_id` inexistente retorna `404` `"pedido não encontrado"` para `admin` e `normal`. Antes o admin recebia `400`. O corpo é idêntico ao de pedido de outro vendedor, então não permite enumeração (parecer do 🟣 SecBrain). `pedido_id` ausente ou `<= 0` continua `400` `"pedido_id é obrigatório"`.
> - Usuário `admin` não sofre essas restrições.
>
> **Nota de schema:** a chave primária da tabela é literalmente `pagamento_id` (BIGINT AUTO_INCREMENT), não o padrão `id` desacoplado usado nas demais tabelas — decisão explícita do usuário, alinhada 1:1 ao `pagamento_id` do CSV de origem.

#### GET /api/pagamentos
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Query (todos opcionais):** `?page=1&limit=20&status_pagamento=Em aberto&forma_pagamento=PIX&pedido_id=1&vencimento_de=2026-01-01&vencimento_ate=2026-12-31&order_by=data_vencimento&order_dir=asc`
- **Descrição:** Lista pagamentos paginada (total + pages), com filtros exatos por `status_pagamento` (`Em aberto`|`Inadimplente`|`Pago`|`Pago com atraso`), `forma_pagamento` (`Boleto 14 dias`|`Boleto 28 dias`|`Cartão de crédito`|`Cartão de débito`|`Cheque a prazo`|`Dinheiro`|`PIX`), `pedido_id` e intervalo `vencimento_de`/`vencimento_ate` (`AAAA-MM-DD`).
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `pagamento_id, pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento, created_at, updated_at` (default: `pagamento_id`); `order_dir` aceita `asc`|`desc` case-insensitive (default: `asc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/pagamentos
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Body:** `{ "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct", "valor_liquido", "data_vencimento" ("AAAA-MM-DD"), "data_pagamento" (opcional, "AAAA-MM-DD"), "status_pagamento" }`
- **Descrição:** Cria um novo pagamento. `pedido_id` deve existir em `pedidos`. `valor_liquido` é sempre exigido explicitamente no payload — não é calculado automaticamente a partir de `valor`/`taxa_pct` (o CSV de origem já traz o valor líquido calculado, às vezes com pequenas diferenças de arredondamento em relação ao cálculo direto). `pagamento_id` é gerado automaticamente (AUTO_INCREMENT). `taxa_pct` deve estar entre 0 e 30, limites inclusivos (SEC-27, Lote 25): fora disso, `400` `"taxa_pct deve estar entre 0 e 30"`, para admin e normal. Retorna `201` com o pagamento criado; `400` em caso de validação (inclusive `pedido_id` ausente ou `<= 0`); `404` `"pedido não encontrado"` se `pedido_id` não existir (admin e normal, desde 2026-09-24).
- **Escopo (usuário `normal`, 2026-09-23):** `403` `"usuário sem vendedor vinculado"` se não houver vendedor vinculado; `404` `"pedido não encontrado"` se `pedido_id` pertencer a outro vendedor ou não existir.

#### GET /api/pagamentos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Descrição:** Retorna o detalhe de um pagamento pelo `pagamento_id`.

#### PUT /api/pagamentos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Body:** mesmo formato do `POST /api/pagamentos`, exceto `pedido_id`
- **Descrição:** Atualiza os campos editáveis de um pagamento existente. `pagamento_id` e `pedido_id` **não** são editáveis por esta rota (o vínculo com o pedido de origem é definitivo — para reatribuir a outro pedido, o fluxo correto é excluir/recriar). Retorna `200` com o pagamento atualizado, `404` se não existir, `400` se o payload for inválido.
- **Escopo (usuário `normal`, 2026-09-23):** `404` `"pagamento não encontrado"` se o usuário não tiver vendedor vinculado ou o pagamento pertencer a pedido de outro vendedor.
- **Pagamento quitado (SEC-27, Lote 25, 2026-10-30):** pagamento `Pago` ou `Pago com atraso` só é editável por **admin** (`200`). Para o usuário `normal`, `409` `"pagamento já quitado não pode ser editado"`. O UPDATE do `normal` é condicional (`… WHERE pagamento_id = ? AND status_pagamento NOT IN ('Pago', 'Pago com atraso')`, com os valores vindos do domínio); com 0 linhas, a API relê: inexistente → `404`, quitado (inclusive quitado por outra requisição no meio) → `409`. `Em aberto` → `Pago` continua permitido. Ordem: posse → leitura do body → validação (`400`) → UPDATE (`409`/`404`).
- **Teto da taxa (SEC-27):** `taxa_pct` fora de 0 a 30 → `400` `"taxa_pct deve estar entre 0 e 30"` (admin e normal).
- **Na tela:** o usuário `normal` vê "Ver" em vez de "Editar" nos quitados; o modal abre em modo leitura (campos desabilitados, sem Salvar, aviso "Pagamento quitado nao pode ser editado."). O campo Taxa (%) tem `max=30` e a validação "Taxa (%) deve estar entre 0 e 30.".

#### DELETE /api/pagamentos/{id} (adicionado em 2026-09-22)
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Descrição:** Exclui definitivamente um pagamento (hard delete, sem soft-delete). Bloqueado com `409` se `status_pagamento` já for `"Pago"` ou `"Pago com atraso"` (`"pagamento já quitado não pode ser excluído"`) — preserva a trilha financeira de pagamentos já quitados. Escopo idêntico ao `GET`/`PUT`: `404` (não `403`) se o pagamento pertencer a pedido de outro vendedor. Retorna `204` sem corpo em caso de sucesso.

### Vendedores (`/api/vendedores/*`) — admin only, exceto 2 GETs (atualizado em 2026-09-23)

> Até 2026-09-22 todas as rotas `/api/vendedores*` usavam `JWTMiddleware(cfg, true, false)` (acesso comum) — a restrição a admin existia só no frontend. O 🟣 SecBrain identificou que um usuário `normal` podia gerenciar vendedores via API, ler a carteira de qualquer vendedor (`GET /api/vendedores/{id}`) e — crítico — transferir clientes de outros vendedores para a própria carteira (`POST /api/vendedores/{id}/clientes`). Desde 2026-09-23 (na época em `routes/routes.go`; desde o Lote 21, na tabela de rotas de `backend/rotaperfumes-api/internal/bootstrap/rotas.go`):

| Método | Rota | Acesso | Usuário `normal` |
|--------|------|--------|------------------|
| GET | `/api/vendedores` | acesso comum, com escopo (SEC-03, 2026-09-25) | `200` só com o próprio vendedor; `200` `data: []` sem vínculo; `403` se o vendedor estiver desligado |
| GET | `/api/vendedores/{id}/clientes` | acesso comum | `200` (dropdown em cascata Vendedor → Cliente) |
| POST | `/api/vendedores` | admin only | `403` |
| GET | `/api/vendedores/{id}` | admin only | `403` |
| PUT | `/api/vendedores/{id}` | admin only | `403` |
| DELETE | `/api/vendedores/{id}` | admin only (inativa via `data_desligamento`; em vendedor já desligado responde `200` e preserva a `data_desligamento` original — BUG-04, 2026-09-24) | `403` |
| POST | `/api/vendedores/{id}/reativar` | admin only | `403` |
| POST | `/api/vendedores/{id}/clientes` | admin only (vincula/transfere cliente; vínculo do mesmo par na mesma data → `409` `"vínculo entre cliente e vendedor já cadastrado nesta data"`, CHORE-06, Lote 25) | `403` |
| DELETE | `/api/vendedores/{id}/clientes/{clienteId}` | admin only (encerra vínculo; `{clienteId}` inválido → `400` `"cliente id inválido"`, CHORE-03, Lote 25) | `403` |

> O `403` retorna `{"success": false, "error": "acesso restrito a administradores"}`. 
>
> **`meta_mensal` fora da listagem (confirmado em 2026-09-24):** `GET /api/vendedores` devolve `VendedorResumo` (`id`, `nome`, `regiao`, `uf`, `data_desligamento`) para **todos** os perfis, sem `meta_mensal`. O usuário `normal` não recebe a meta de ninguém. O admin obtém `meta_mensal` pelo detalhe `GET /api/vendedores/{id}` (admin only). O código já estava assim; o 🔴 TestBrain adicionou um teste de regressão.
>
> **Escopo da listagem (SEC-03, 2026-09-25):** antes, qualquer usuário autenticado recebia a lista completa de vendedores. Agora `GET /api/vendedores` (`vendedor_handler.go`, `ListResumoByID`):
> - **admin:** todos os vendedores, ativos e desligados (sem mudança);
> - **`normal` com vendedor vinculado:** `200` com `data: [{id, nome, regiao, uf, data_desligamento}]` contendo só o próprio vendedor;
> - **`normal` sem vínculo (ou vínculo órfão):** `200` com `data: []`;
> - **`normal` com vendedor desligado:** `403` `{"success":false,"error":"acesso bloqueado: vendedor desligado"}`;
> - falha ao resolver o escopo: `500` `"erro interno"`.
> - `GET /api/produtos` **não** foi bloqueado (decisão do 🟣 SecBrain: é catálogo).
> - No frontend, `useVendedorTravado.ts` trava o campo vendedor nos modais de Oportunidade e Visita; o `PedidoModal` já estava correto.
> - Na collection: pasta **"Vendedores"** → "Listar Vendedores (escopo por usuário)", com os quatro exemplos.

### Oportunidades (`/api/oportunidades/*`) — **acesso comum, com escopo por carteira** (atualizado em 2026-09-22)

> Funil de vendas (CRM) importado de `dados/crm/oportunidades.csv` (colunas: `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda`). Tela com dois dropdowns em cascata (Vendedor → Cliente) e filtros de coluna na listagem. Requests desses endpoints estão agrupadas na pasta **"Oportunidades"** da collection.
>
> **Botão de exclusão (2026-09-22):** adicionado endpoint `DELETE /api/oportunidades/{id}` — hard delete (sem exclusão lógica), sem restrição extra de negócio. Ver detalhes abaixo.
>
> **Mudança de acesso (2026-09-22):** `GET/POST /api/oportunidades` e `GET/PUT /api/oportunidades/{id}` deixaram de ser `admin only` e passaram a **acesso comum com escopo por carteira** — mesma cadeia de middleware usada em Pagamentos (`cfg, true, false`), com a restrição de dono aplicada no handler via `vendedorScope`/`resolverVendedorScope` (`backend/rotaperfumes-api/handlers/scope.go`; desde o Lote 21, pelo `AutorizadorEscopo` de `backend/shared/comercial/comercialapp/escopo.go`), o mesmo padrão já usado em Pedidos/Clientes/Pagamentos:
> - **Listagem (`GET /api/oportunidades`):** usuário `admin` enxerga tudo; usuário `normal` só enxerga oportunidades da própria carteira — qualquer `vendedor_id` informado na query é **ignorado** e forçado ao vendedor vinculado ao usuário logado (nunca confia em input do cliente).
> - **Detalhe/edição por ID (`GET`/`PUT /api/oportunidades/{id}`):** usuário `normal` tentando acessar/editar uma oportunidade de outro vendedor recebe **`404`** (não `403`) — decisão deliberada para não expor, por enumeração de ID, a existência do registro a quem não tem acesso a ele.
> - **Criação/edição (`POST`/`PUT`):** o `vendedor_id` do payload é **ignorado** para usuário `normal` — é sempre forçado ao vendedor vinculado ao usuário logado, mesmo que o body envie outro valor (impede forjar/reatribuir o registro a outro vendedor). Além disso, `cliente_id` precisa pertencer à carteira ativa (`data_fim IS NULL`) desse vendedor — se não pertencer, retorna `400` (`"cliente não pertence à carteira deste vendedor"`).
> - Usuário `normal` sem vendedor vinculado (`id_vendedor` nulo) é tratado como carteira vazia: listagem retorna vazio, `GET`/`PUT` por ID retornam `404`, `POST` retorna `403` (`"usuário sem vendedor vinculado"`).
> - Usuário `admin` não sofre nenhuma dessas restrições (comportamento idêntico ao anterior).
>
> **Decisão de design:** `vendedor_id` é um campo próprio da oportunidade — **não depende** de a oportunidade ter um vínculo de carteira ativo entre aquele cliente e aquele vendedor (uma oportunidade pode existir mesmo que o cliente esteja hoje na carteira de outro vendedor, ou sem vínculo ativo algum) — essa regra vale para `admin`; para usuário `normal`, o `cliente_id` do `POST`/`PUT` é validado contra a própria carteira, como descrito acima. O endpoint `GET /api/vendedores/{id}/clientes` (usado só para alimentar o segundo dropdown do formulário) continua **acesso comum**, mesma cadeia de middleware (`cfg, true, false`).

#### GET /api/oportunidades
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Query (todos opcionais):** `?page=1&limit=20&cliente_id=1&vendedor_id=1&etapa=Proposta enviada&origem=Indicação&data_abertura_de=2026-01-01&data_abertura_ate=2026-12-31&q=indica&order_by=data_abertura&order_dir=desc`
- **Descrição:** Lista oportunidades paginada (total + pages), com filtros exatos por `cliente_id`, `vendedor_id`, `etapa`, `origem`, intervalo `data_abertura_de`/`data_abertura_ate` (`AAAA-MM-DD`) e busca livre (`q`) em `origem` OU `etapa`. Para usuário `normal`, `vendedor_id` da query é ignorado e forçado à própria carteira.
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, data_abertura, valor_estimado, probabilidade_pct, etapa, origem, created_at, updated_at` (default: `id`); `order_dir` aceita `asc`|`desc` case-insensitive (default: `asc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/oportunidades
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Body:** `{ "cliente_id", "vendedor_id", "origem", "data_abertura" (opcional, "AAAA-MM-DD", default hoje), "etapa", "probabilidade_pct", "valor_estimado", "data_fechamento" (opcional, "AAAA-MM-DD"), "ciclo_dias" (opcional), "motivo_perda" (obrigatório se etapa = "Fechado perdido") }`
- **Descrição:** Cria uma nova oportunidade. `cliente_id` deve existir em `clientes`; `vendedor_id` deve existir em `vendedores` (campo independente de vínculo de carteira, para `admin`). `probabilidade_pct` deve estar entre 0 e 100; `valor_estimado` deve ser `>= 0`. `oportunidade_id` é gerado automaticamente (AUTO_INCREMENT). Retorna `201` com a oportunidade criada; `400` em caso de validação. Para usuário `normal`: `vendedor_id` do payload é ignorado (forçado à própria carteira) e `cliente_id` precisa pertencer à carteira ativa desse vendedor (`400` se não pertencer).

#### GET /api/oportunidades/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Descrição:** Retorna o detalhe de uma oportunidade pelo `oportunidade_id`. Usuário `normal` tentando acessar oportunidade de outro vendedor recebe `404`.

#### PUT /api/oportunidades/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Body:** mesmo formato do `POST /api/oportunidades` (`data_abertura` obrigatório na edição)
- **Descrição:** Atualiza os dados de uma oportunidade existente. Retorna `200` com a oportunidade atualizada, `404` se não existir (ou pertencer a outro vendedor, para usuário `normal`), `400` se o payload for inválido (inclui `cliente_id` fora da carteira do vendedor, para usuário `normal`). `vendedor_id` do payload é ignorado para usuário `normal` (não é possível reatribuir a outro vendedor).

#### DELETE /api/oportunidades/{id} (adicionado em 2026-09-22)
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Descrição:** Exclui definitivamente uma oportunidade (hard delete, sem soft-delete). Sem restrição extra de negócio (diferente de Pedido/Pagamento). Usuário `normal` tentando excluir oportunidade de outro vendedor recebe `404` (não `403`). Retorna `204` sem corpo em caso de sucesso.

#### GET /api/vendedores/{id}/clientes
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Descrição:** Lista os clientes vinculados (carteira ativa, `data_fim IS NULL`) a um vendedor. Usado pelo dropdown em cascata da tela de Oportunidades (ao escolher o vendedor, filtra os clientes possíveis no segundo dropdown). `404` se o vendedor não existir.

### Visitas (`/api/visitas/*`) — **acesso comum, com escopo por carteira** (atualizado em 2026-09-22)

> Registro de visitas de vendedores a clientes (CRM), importado de `dados/crm/visitas.csv` (colunas: `visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min`). Tela com os mesmos dois dropdowns em cascata (Vendedor → Cliente) usados em Oportunidades — reaproveita o endpoint compartilhado `GET /api/vendedores/{id}/clientes` (acesso comum, documentado acima na seção de Oportunidades) para alimentar o segundo dropdown. Filtros de coluna na listagem. Requests desses endpoints estão agrupadas na pasta **"Visitas"** da collection.
>
> **Botão de exclusão (2026-09-22):** adicionado endpoint `DELETE /api/visitas/{id}` — hard delete (sem exclusão lógica), sem restrição extra de negócio. Ver detalhes abaixo.
>
> **Diferente de Oportunidades:** `data_visita` é **obrigatório e sem default** — no `POST`/`PUT` de Oportunidades, `data_abertura` vazio assume a data de hoje; em Visitas, `data_visita` vazio/ausente retorna `400` (`"data_visita inválida (use o formato AAAA-MM-DD)"`).
>
> **Mudança de acesso (2026-09-22):** exatamente a mesma mudança e as mesmas regras de escopo por carteira aplicadas a Oportunidades acima (mesmo padrão `vendedorScope`/`resolverVendedorScope`) — `GET/POST /api/visitas` e `GET/PUT /api/visitas/{id}` deixaram de ser `admin only` e passaram a **acesso comum**: usuário `normal` só lista/vê/cria/edita visitas da própria carteira (`vendedor_id` da query/payload é ignorado e forçado ao vendedor vinculado; `cliente_id` do `POST`/`PUT` precisa pertencer à carteira ativa desse vendedor, `400` se não pertencer); acesso a visita de outro vendedor por `GET`/`PUT` por ID retorna `404`; usuário `normal` sem vendedor vinculado é tratado como carteira vazia (listagem vazia, `404` em `GET`/`PUT` por ID, `403` em `POST`).

#### GET /api/visitas
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Query (todos opcionais):** `?page=1&limit=20&cliente_id=1&vendedor_id=1&resultado=Pedido fechado&data_visita_de=2026-01-01&data_visita_ate=2026-12-31&q=fechado&order_by=data_visita&order_dir=desc`
- **Descrição:** Lista visitas paginada (total + pages), com filtros exatos por `cliente_id`, `vendedor_id`, `resultado`, intervalo `data_visita_de`/`data_visita_ate` (`AAAA-MM-DD`) e busca livre (`q`) em `resultado`. Para usuário `normal`, `vendedor_id` da query é ignorado e forçado à própria carteira.
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, visita_id, data_visita, duracao_min, resultado, created_at, updated_at` (default: `id`); `order_dir` aceita `asc`|`desc` case-insensitive (default: `asc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/visitas
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Body:** `{ "cliente_id", "vendedor_id", "data_visita" ("AAAA-MM-DD", obrigatório, sem default), "resultado", "duracao_min" }`
- **Descrição:** Cria uma nova visita. `cliente_id` deve existir em `clientes`; `vendedor_id` deve existir em `vendedores`. `resultado` obrigatório (não vazio); `duracao_min` deve ser `>= 0`. `visita_id` é gerado automaticamente (AUTO_INCREMENT). Retorna `201` com a visita criada; `400` em caso de validação. Para usuário `normal`: `vendedor_id` do payload é ignorado (forçado à própria carteira) e `cliente_id` precisa pertencer à carteira ativa desse vendedor (`400` se não pertencer).

#### GET /api/visitas/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Descrição:** Retorna o detalhe de uma visita pelo `visita_id`. Usuário `normal` tentando acessar visita de outro vendedor recebe `404`.

#### PUT /api/visitas/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Body:** mesmo formato do `POST /api/visitas` (`data_visita` obrigatório também na edição, sem default)
- **Descrição:** Atualiza os dados de uma visita existente. Retorna `200` com a visita atualizada, `404` se não existir (ou pertencer a outro vendedor, para usuário `normal`), `400` se o payload for inválido (inclui `cliente_id` fora da carteira do vendedor, para usuário `normal`). `vendedor_id` do payload é ignorado para usuário `normal` (não é possível reatribuir a outro vendedor).

#### DELETE /api/visitas/{id} (adicionado em 2026-09-22)
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Descrição:** Exclui definitivamente uma visita (hard delete, sem soft-delete). Sem restrição extra de negócio (diferente de Pedido/Pagamento). Usuário `normal` tentando excluir visita de outro vendedor recebe `404` (não `403`). Retorna `204` sem corpo em caso de sucesso.

### Estoque (`/api/estoque/*`) — admin only

> Série temporal de snapshots diários de saldo por SKU, importada de `dados/erp/estoque.csv` para a tabela `estoque` (ver seção "Importação de estoque (ERP)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Estoque"** da collection. Não existe endpoint de `DELETE`.
>
> **Mudança de acesso (2026-09-22):** os quatro endpoints (`GET /api/estoque`, `GET /api/estoque/{id}`, `POST /api/estoque`, `PUT /api/estoque/{id}`) são **admin only** (`cfg, true, true`). Antes, `GET /api/estoque` e `GET /api/estoque/{id}` eram acesso comum (qualquer usuário autenticado); foram restringidos a admin junto com a mudança de "Estoque" no menu do frontend, que passou do dropdown "CRM" para "Administração" (visível só para admin).
>
> **Constraint de unicidade:** `UNIQUE (data_snapshot, sku)` — só existe um snapshot por SKU por dia. O importador e o hook de faturamento fazem *upsert* nessa chave.
>
> **Hook de faturamento:** ao atualizar um pedido (`PUT /api/pedidos/{id}`) com transição de status para `Faturado`, o sistema decrementa automaticamente o saldo de estoque de cada item do pedido (dentro da mesma transação de `UpdateComItens`, com `SELECT ... FOR UPDATE` para evitar condição de corrida). É idempotente — reenviar o update com status já `Faturado` não baixa estoque de novo — e bloqueia a edição dos itens de um pedido já faturado (a tentativa retorna `409 Conflict`: `{"success": false, "error": "pedido já faturado: não é possível alterar os itens, apenas o status"}`).
>
> **Lote 25 (2026-10-30):** pelo BUG-16, Faturado só vai para Entregue e nenhuma transição volta para Faturado, então não há mais baixa dupla no ciclo Faturado → outro → Faturado; os itens também ficam travados em Entregue. Pelo BUG-20, o `POST /api/pedidos` já como Faturado baixa o estoque na mesma transação. Pelo BUG-21, a `ruptura` do upsert é calculada sobre o saldo já atualizado (`ruptura = saldo <= 0`, na ordem de atribuição do MariaDB): saldo 10 com baixa de 6 grava saldo 4 e ruptura 0 (antes, ruptura 1).
>
> **Decisão consciente sobre a data do snapshot no faturamento:** a baixa de estoque usa a data/hora atual do servidor (`time.Now()`) como `data_snapshot` no momento do faturamento — **não** a data de criação do pedido (`data_pedido`). Ou seja, o snapshot de estoque reflete o dia em que o faturamento efetivamente ocorreu, e não a data em que o pedido foi originalmente registrado. Isso foi identificado e documentado pelo TestBrain como decisão de negócio (não é bug).

#### GET /api/estoque
- **Auth:** Bearer Token (admin)
- **Query (todos opcionais):** `?page=1&limit=20&sku=ROT-0001&data_de=2026-09-01&data_ate=2026-09-22&ruptura=false&order_by=data_snapshot&order_dir=desc&historico=true`
- **Descrição:** Lista snapshots de estoque paginados (total + pages), com filtro exato por `sku`, filtro por `ruptura` (`true`|`false`) e intervalo `data_de`/`data_ate` (`AAAA-MM-DD`). Cada registro vem com `produto_descricao` (via JOIN com `produtos`), para exibição ao lado do SKU.
- **Comportamento por padrão (sem `historico`):** retorna a **última posição de estoque de cada SKU** (um registro por SKU). Sem `data_ate`, é a última posição geral (snapshot mais recente); com `data_ate`, é a **última posição de cada SKU até (inclusive) aquela data** — permite consultar "quanto tinha em estoque até o dia X" para todos os produtos.
- **Com `historico=true`:** retorna a **série temporal completa** (todos os snapshots que casarem com `data_de`/`data_ate`, sem agregação por SKU).
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, sku, data_snapshot, saldo, ruptura, created_at, updated_at` (default: `data_snapshot`); `order_dir` aceita `asc`|`desc` case-insensitive (default: `desc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### GET /api/estoque/{id}
- **Auth:** Bearer Token (admin)
- **Descrição:** Retorna o detalhe de um snapshot de estoque pelo `id`.

#### POST /api/estoque
- **Auth:** Bearer Token (admin)
- **Body:** `{ "sku", "data_snapshot" ("AAAA-MM-DD"), "saldo" }`
- **Descrição:** Cria um snapshot de estoque manual. `sku` deve existir em `produtos`; `ruptura` é derivada automaticamente (`saldo <= 0`, não aceita no body). Respeita a constraint `UNIQUE (data_snapshot, sku)` — criar outro registro para o mesmo par retorna `409` `"registro de estoque já cadastrado para este sku nesta data"` (CHORE-06, Lote 25, 2026-10-30; antes era `500`, e esta documentação dizia `400` por engano). O registro do dia pode ter sido criado pelo faturamento; para ajustar, edite o registro existente (`PUT`). Na tela de Estoque, o 409 vem com a orientação "Edite o registro existente.". Retorna `201` com o snapshot criado.

#### PUT /api/estoque/{id}
- **Auth:** Bearer Token (admin)
- **Body:** `{ "saldo" }`
- **Descrição:** Atualiza o saldo de um snapshot existente. `sku` e `data_snapshot` **não** são editáveis por esta rota (imutáveis). `ruptura` é recalculada automaticamente. Retorna `200` com o snapshot atualizado, `404` se não existir, `400` se o payload for inválido.
- **PUT de estoque é sobrescrita absoluta (nota do BUG-17, Lote 25):** o saldo enviado (contagem física) substitui o gravado, com saldo e ruptura no mesmo UPDATE. Ele sobrescreve inclusive baixas de faturamento concorrentes no mesmo snapshot. Comportamento intencional, sem mudança.

## Testes automatizados (Postman)

A collection inclui scripts de teste em JavaScript em cada request. Os testes verificam:

### Login (admin)
- `Login bem-sucedido` (status 200)
- `Resposta contém user com role`
- `Contém access_token`
- `Contém refresh_token`
- Salva automaticamente os tokens nas variáveis da collection

### Login — Vendedor (primeiro acesso)
- `Login de vendedor bem-sucedido` (status 200)
- `Role é normal`
- `Flag trocar_senha é true (deve_trocar_senha=1 no banco)` — **teste crítico para validação da feature de primeiro acesso**
- `Contém access_token`
- `Contém refresh_token`
- `Contém dados do usuário`
- Salva o token em `vendedor_token`

### Refresh Token
- `Status 200`
- `Novo access_token gerado`
- `Novo refresh_token gerado`
- `Expires_in presente`
- Atualiza variáveis da collection

### Logout
- `Status 200`
- `Logout realizado com sucesso`

### Me
- `Status 200`
- `Resposta contém dados do usuário`
- `vendedor_desligado é booleano` (2026-09-24)

### Reset Password
- `Status 200 para usuário autenticado`
- `Confirmação de reset`

### Listar Usuários
- `Status 200`
- `Lista retornada`
- `Paginação presente`

### Criar Usuário
- `Status 201 Created`
- `Usuário criado com dados corretos`
- `Resposta informa se o email foi enviado` (`email_enviado` é booleano)

### Atualizar Usuário
- `Status 200 OK`
- `Usuário atualizado com dados corretos`

### Ativar/Inativar
- `Status 200 OK`
- `Usuário com campo ativo retornado`

### Reset Password Admin
- `Status 200 OK`
- `Senha resetada com sucesso`
- `Resposta informa se o email foi enviado` (`email_enviado` é booleano)

### Dashboard — Métricas
- `Status 200`
- `Dados de métricas presentes` (`total_vendas`, `total_pedidos`, `ticket_medio`, `periodo`, `meta_mes`; `vendedor_desligado` é booleano)
- `top_vendedores e metas_vendedores são arrays (nunca null)` (adicionado em 2026-09-24)

### Dashboard — Vendas
- `Status 200`
- `Série temporal retornada`
- `Cada item tem data e valor`

### Dashboard — Vendedores
- `Status 200`
- `Lista de vendedores retornada`
- `Paginação presente`
- `Cada vendedor tem métricas`

### Dashboard — Funil de vendas (DASH-01, Lote 22)
- **Default (sem janela), `30d`, `90d`, `365d`:** `Status 200`, `Funil com as 5 etapas, perdidas e taxa de ganho` e `Janela ecoada` (no default, `Janela default é all`). O default tem ainda `Etapas na ordem do funil`.
- **Janela inválida (`?janela=7d`):** `Status 400` e `Erro com a mensagem do contrato`.

> Os testes de Dashboard usam `{{admin_token}}`. Se forem executados com `{{vendedor_token}}` (normal), os números vêm restritos ao próprio vendedor, e o ranking tem no máximo 1 linha. Se o usuário normal não tiver vendedor vinculado, ou se o vendedor estiver desligado, os números vêm zerados e as listas vazias, e os testes continuam passando, porque a estrutura da resposta é a mesma. No caso do vendedor desligado, `/metrics` traz `vendedor_desligado: true`. O teste "Dashboard — Clientes" também passa com `{{vendedor_token}}` (desde 2026-09-24 a rota é de acesso comum, com escopo da carteira).

### Histórico de Senhas — Todos
- `Status 200`
- `Lista de histórico retornada`
- `Paginação presente`

### Histórico de Senhas — Por Usuário
- `Status 200`
- `Lista de histórico do usuário retornada`
- `Todos os itens são do usuário correto`

### Listar Clientes
- `Status 200`
- `Lista retornada`
- `Paginação presente`

### Criar Cliente
- `Status 201 Created`
- `Cliente criado com dados corretos` (`cliente_id_origem`, `razao_social`, `ativo === true`)

### Detalhe do Cliente
- `Status 200`
- `Dados do cliente presentes` (`cliente_id_origem`, `razao_social`, `cnpj`; antes verificava `id`, campo que não existe mais)

### Editar Cliente
- `Status 200 OK`
- `Cliente atualizado com dados corretos` (`cliente_id_origem` e `razao_social`; antes verificava `id`, campo que não existe mais)

### Ativar/Inativar Cliente
- `Status 200 OK`
- `Cliente com campo ativo retornado`

### Dashboard — Clientes
- `Status 200`
- `Métricas de clientes presentes` (`periodo`, `total_clientes`, `total_ativos`, `total_inativos`, `novos_no_periodo`, `por_segmento`, `por_uf`)
- `por_segmento e por_uf são arrays`

### Listar Produtos
- `Status 200`
- `Lista retornada`
- `Paginação presente`

### Criar Produto
- `Status 201 Created`
- `Produto criado com dados corretos` (`id`, `descricao`, `ativo === true`)

### Detalhe do Produto
- `Status 200`
- `Dados do produto presentes`

### Editar Produto
- `Status 200 OK`
- `Produto atualizado com dados corretos`

### Ativar/Inativar Produto
- `Status 200 OK`
- `Produto com campo ativo retornado`

### Enviar/Trocar Imagem do Produto (IMG-01, Lote 26)
- `Status 200 OK`
- `Produto com imagem_url pública (UUID v4 + extensão do tipo detectado)`
- Salva o último segmento de `imagem_url` em `imagem_arquivo`. Antes de enviar, escolha o arquivo no campo `imagem` da aba Body.

### Imagem do Produto — leitura pública (IMG-01, Lote 26)
- `Status 200 OK`
- `Content-Type de imagem (mapa fixo)`
- `Cabeçalhos de cache e segurança` (`Cache-Control`, `nosniff`, `Cross-Origin-Resource-Policy`, `Content-Disposition`)
- `Sem Set-Cookie`

### Remover Imagem do Produto (IMG-01, Lote 26)
- `Status 200 OK`
- `Produto retornado com imagem_url null`

### Listar Pedidos
- `Status 200`
- `Lista retornada`
- `Paginação presente`

### Criar Pedido
- `Status 201 Created`
- `Pedido criado com dados corretos` (`pedido_id_origem`, `itens` array não vazio)
- `valor_total e valor_bruto calculados pelo backend`

### Detalhe do Pedido
- `Status 200`
- `Dados do pedido presentes` (`pedido_id_origem`, `itens`)
- `Itens é um array`

### Editar Pedido
- `Status 200 OK`
- `Pedido atualizado com dados corretos` (`id`, `status`, `itens` array)

### Excluir Pedido
- `Status 204 No Content`

### Listar Pagamentos
- `Status 200`
- `Usuário NORMAL consegue acessar (não é 403)` — **valida explicitamente que o acesso é comum, não admin only**
- `Lista retornada`
- `Paginação presente`

### Criar Pagamento
- `Status 201 Created`
- `Usuário NORMAL consegue criar (não é 403)`
- `Pagamento criado com dados corretos` (`pagamento_id`, `pedido_id`, `status_pagamento`)

### Detalhe do Pagamento
- `Status 200`
- `Dados do pagamento presentes` (`pagamento_id`, `pedido_id`, `status_pagamento`)

### Editar Pagamento
- `Status 200 OK`
- `Usuário NORMAL consegue editar (não é 403)`
- `Pagamento atualizado com dados corretos` (`pagamento_id`, `status_pagamento`)

### Excluir Pagamento
- `Status 204 No Content`
- `Usuário NORMAL consegue excluir (não é 403)` — **valida explicitamente que o acesso é comum, não admin only**

### Listar Oportunidades
- `Status 200`
- `Lista retornada`
- `Paginação presente`

### Detalhe da Oportunidade
- `Status 200`
- `Dados da oportunidade presentes` (`oportunidade_id`, `cliente_id`, `vendedor_id`, `etapa`)

### Criar Oportunidade
- `Status 201 Created`
- `Oportunidade criada com dados corretos` (`oportunidade_id`, `cliente_id`, `vendedor_id`, `etapa`)

### Editar Oportunidade
- `Status 200 OK`
- `Oportunidade atualizada com dados corretos` (`oportunidade_id`, `etapa`)

### Excluir Oportunidade
- `Status 204 No Content`

### Listar Clientes do Vendedor
- `Status 200`
- `Usuário NORMAL consegue acessar (não é 403)` — **valida explicitamente que o acesso é comum, não admin only**
- `Lista de clientes retornada`

### Listar Visitas
- `Status 200`
- `Lista retornada`
- `Paginação presente`

### Detalhe da Visita
- `Status 200`
- `Dados da visita presentes` (`visita_id`, `cliente_id`, `vendedor_id`, `resultado`)

### Criar Visita
- `Status 201 Created`
- `Visita criada com dados corretos` (`visita_id`, `cliente_id`, `vendedor_id`, `resultado`)

### Editar Visita
- `Status 200 OK`
- `Visita atualizada com dados corretos` (`visita_id`, `resultado`)

### Excluir Visita
- `Status 204 No Content`

### Listar Estoque
- `Status 200`
- `Lista retornada`
- `Paginação presente`

### Detalhe do Estoque
- `Status 200`
- `Dados do estoque presentes` (`id`, `sku`, `saldo`)

### Criar Estoque
- `Status 201 Created`
- `Estoque criado com dados corretos` (`id`, `sku`)

### Editar Estoque
- `Status 200 OK`
- `Estoque atualizado com dados corretos` (`saldo`)

## Resumo de testes por endpoint

| Request | # Testes | Salva variáveis |
|---------|----------|-----------------|
| Healthcheck | 0 | — |
| Login (admin) | 4 | `token`, `admin_token`, `refresh_token` |
| Login — Vendedor | 6 | `vendedor_token`, `refresh_token` |
| Logout | 2 | — |
| Refresh Token | 4 | `token`, `refresh_token` |
| Me | 3 | — |
| Reset Password | 2 | — |
| Listar Usuários | 3 | — |
| Criar Usuário | 2 | — |
| Atualizar Usuário | 2 | — |
| Ativar/Inativar | 2 | — |
| Reset Password Admin | 2 | — |
| Dashboard — Métricas | 3 | — |
| Dashboard — Vendas | 3 | — |
| Dashboard — Vendedores | 4 | — |
| Funil de vendas — default | 4 | — |
| Funil de vendas — 30d | 3 | — |
| Funil de vendas — 90d | 3 | — |
| Funil de vendas — 365d | 3 | — |
| Funil de vendas — janela inválida | 2 | — |
| Histórico — Todos | 3 | — |
| Histórico — Por Usuário | 3 | — |
| Listar Clientes | 2 | — |
| Criar Cliente | 2 | — |
| Detalhe do Cliente | 1 | — |
| Editar Cliente | 2 | — |
| Ativar/Inativar Cliente | 1 | — |
| Dashboard — Clientes | 2 | — |
| Listar Produtos | 2 | — |
| Criar Produto | 2 | — |
| Detalhe do Produto | 1 | — |
| Editar Produto | 2 | — |
| Ativar/Inativar Produto | 2 | — |
| Enviar/Trocar Imagem do Produto (Lote 26) | 2 | `imagem_arquivo` |
| Imagem do Produto — leitura pública (Lote 26) | 4 | — |
| Remover Imagem do Produto (Lote 26) | 2 | — |
| Listar Pedidos | 2 | — |
| Criar Pedido | 2 | — |
| Detalhe do Pedido | 2 | — |
| Editar Pedido | 1 | — |
| Excluir Pedido | 1 | — |
| Listar Pagamentos | 4 | — |
| Criar Pagamento | 3 | — |
| Detalhe do Pagamento | 1 | — |
| Editar Pagamento | 3 | — |
| Excluir Pagamento | 2 | — |
| Listar Oportunidades | 2 | — |
| Detalhe da Oportunidade | 1 | — |
| Criar Oportunidade | 1 | — |
| Editar Oportunidade | 1 | — |
| Excluir Oportunidade | 1 | — |
| Listar Clientes do Vendedor | 2 | — |
| Vincular Cliente ao Vendedor (Lote 25) | 2 | — |
| Encerrar Vínculo Cliente-Vendedor (Lote 25) | 1 | — |
| Listar Visitas | 2 | — |
| Detalhe da Visita | 1 | — |
| Criar Visita | 1 | — |
| Editar Visita | 1 | — |
| Excluir Visita | 1 | — |
| Listar Estoque | 3 | — |
| Detalhe do Estoque | 1 | — |
| Criar Estoque | 1 | — |
| Editar Estoque | 1 | — |

## Códigos de erro comuns

| Código | Significado |
|--------|-------------|
| 400 | Body inválido ou campos obrigatórios ausentes; `"id inválido"` para id de path não numérico ou `<= 0` (CHORE-03, Lote 25); transição de status de pedido proibida (BUG-16); `q` acima de 100 caracteres e pedido acima de 200 itens (SEC-25 e SEC-14, Lote 27) |
| 401 | Não autenticado (token ausente/inválido) ou credenciais inválidas |
| 403 | Acesso restrito a administradores; usuário `normal` sem vendedor vinculado (escritas); ou `"acesso bloqueado: vendedor desligado"` nas rotas da carteira (2026-09-24) |
| 404 | Recurso não encontrado |
| 413 | `"imagem excede 1 MB"` no upload da imagem do produto (IMG-01, Lote 26); `"corpo da requisição excede o limite de 256 KB"` nas demais rotas (SEC-14, Lote 27) |
| 431 | Headers da requisição acima de 64 KiB (resposta automática do servidor Go; SEC-14, Lote 27) |
| 409 | Conflito: duplicidade (e-mail, CNPJ, SKU, estoque do SKU na data, vínculo de carteira na data; CHORE-06, Lote 25), pedido Faturado/Entregue (itens ou exclusão), pagamento quitado (exclusão; edição pelo `normal`, SEC-27) |
| 500 | Erro interno do servidor |

## Subindo o ambiente

**Banco (DB-03, 2026-10-29):** MariaDB 11.4 LTS. Os alvos `make db-*`, inclusive o `make db-seed` (o `seedusers` chama o binário `mariadb`, sem fallback), exigem o cliente `mariadb` (MariaDB ≥ 11.4) no `PATH`. A API usa o driver Go `go-sql-driver/mysql`, compatível com o MariaDB.

Antes, copie `.env.example` para `.env` na raiz e preencha **`DB_USUARIO` e `DB_SENHA`** (obrigatórios desde o SEC-11, Lote 12, sem default). Sem eles, os alvos `db-*` param com `SEC-11: defina DB_USUARIO/DB_SENHA no .env` e a API não sobe (`config: defina DB_USUARIO/DB_SENHA no .env`).

> **Formato do `.env` (SEC-11):** o `make` lê o arquivo inteiro (`-include .env`) e exporta todas as variáveis. Em **qualquer** valor, evite `$`, `#`, aspas, espaços nas pontas e começar com `/`, e salve o arquivo com final de linha LF. Precedência: `make DB_SENHA=x` na linha de comando > `.env` > variável do shell. A senha chega ao cliente `mariadb` por `MYSQL_PWD`, nunca por `-p` na linha de comando. Para rodar um script de `sql/` à mão: `MYSQL_PWD="$DB_SENHA" mariadb --local-infile=0 -u $DB_USUARIO -h $DB_HOST -P $DB_PORT --default-character-set=utf8mb4 $DB_NAME < sql/<script>.sql`.

```bash
# 1. Recriar o banco com seed (cria todas as tabelas, inclusive estoque)
make db-reset

# 2. Aplicar os seeds com hash gerado (sem alterar sql/; senhas de SEED_ADMIN_PASSWORD/SEED_USER_PASSWORD)
make gen-hash

# 3. Iniciar a API
make dev-api
```

### Envio de email (senha inicial / reset de senha)

Copie `.env.example` (raiz do projeto) para `.env` e preencha as variáveis `SMTP_*` (Gmail com "senha de app") para que `POST /api/usuarios` e `POST /api/admin/reset-password` realmente enviem a senha gerada por email. Se essas variáveis não forem preenchidas, a API sobe normalmente e usa um serviço de email "noop" (apenas loga que o envio foi pulado, sem nunca logar a senha em texto claro) — útil para dev local, mas nesse caso `email_enviado` retorna `false` e o usuário não recebe a nova senha por nenhum canal (o admin precisaria providenciá-la manualmente).

Desde o Lote 11 (SEC-09), o mesmo SMTP envia o alerta de reuso de refresh token ao usuário e aos endereços de `SECURITY_ALERT_EMAILS` (vírgula; vazio = só o usuário). Sem SMTP, a API loga "alertas de segurança só no log".

### CAPTCHA (Cloudflare Turnstile) em Login e Troca de Senha

`POST /api/auth/login` e `POST /api/auth/reset-password` exigem o campo `captchaToken` no body — um token gerado pelo widget Cloudflare Turnstile, validado server-side (fail-closed, timeout 4s) contra `https://challenges.cloudflare.com/turnstile/v0/siteverify`. O verificador fica em `backend/shared/adapters/turnstile/turnstile.go` (até o Lote 21, `backend/shared/services/turnstile_service.go`).

Para configurar:
1. Crie um site no [dashboard do Cloudflare Turnstile](https://dash.cloudflare.com/?to=/:account/turnstile) e copie a **Site Key** (pública) e a **Secret Key** (privada).
2. Backend: copie `.env.example` (raiz do projeto) para `.env` e preencha `TURNSTILE_SECRET_KEY` com a Secret Key. **Nunca** faça commit dessa chave.
3. Frontend: preencha `NEXT_PUBLIC_TURNSTILE_SITE_KEY` em `frontend/.env.local` com a Site Key (ver `frontend/.env.local.example`).

Sem `captchaToken` válido, ambos os endpoints respondem `400` `"captcha inválido ou ausente"` (este parágrafo dizia `401` por engano; corrigido no Lote 28).

**Hostname e action (SEC-19, Lote 28, 2026-10-31):** a resposta do siteverify também precisa trazer um `hostname` da lista aceita (`TURNSTILE_HOSTNAMES` ou, se vazia, os hosts de `CORS_ALLOWED_ORIGINS`) e a `action` do fluxo (`login` no login, `reset-senha` na troca de senha; o widget do frontend envia a action correta). Com uma secret de teste da Cloudflare, os dois não são conferidos. Ver "Mudanças do Lote 28". `POST /api/auth/reset-password` também ganhou rate limiting dedicado (10 falhas/5min, bloqueio de 10min), lacuna identificada pelo SecBrain durante a revisão desta feature.

### Importação de clientes (CRM)

`make db-seed`/`make db-reset` criam a tabela `clientes` (via `sql/09_ddl_clientes.sql`), mas **não** carregam os dados nela. Para popular a tabela `clientes` a partir de `dados/crm/clientes.csv` (~3040 registros), rode adicionalmente:

```bash
make db-up && make db-import-clientes
```

O importador (`backend/shared/cmd/importclientes`) é idempotente (upsert por `cliente_id_origem`) e pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET /api/clientes`, `GET /api/clientes/{id}`, `PATCH /api/clientes/{id}/inativar` e `GET /api/dashboard/clientes` funcionam normalmente, mas retornam base vazia/zerada.

### Importação de produtos (ERP)

`make db-up`/`make db-reset` criam a tabela `produtos` (via `sql/10_ddl_produtos.sql`), mas **não** carregam os dados nela. Para popular a tabela `produtos` a partir de `dados/erp/produtos.csv` (293 registros), rode adicionalmente:

```bash
make db-up && make db-import-produtos
```

O importador (`backend/shared/cmd/importprodutos`) é idempotente (upsert por `sku`) e pode ser executado quantas vezes for necessário sem duplicar registros. Itens do CSV são importados com `ativo = true` por padrão. Sem esse passo, os endpoints `GET /api/produtos`, `GET /api/produtos/{id}`, `POST/PUT /api/produtos` e `PATCH /api/produtos/{id}/inativar` funcionam normalmente, mas retornam/operam sobre base vazia.

### Imagens de produto (IMG-01, Lote 26, 2026-10-31)

As imagens ficam em disco, em `PRODUTO_IMAGENS_DIR` (padrão `./data/produtos-imagens`, relativo ao diretório da API; em dev, `backend/rotaperfumes-api/data/produtos-imagens/`, fora do git). O banco guarda só o nome do arquivo (`produtos.imagem`). Documentada em `.env.example` e `deploy/api.env.example`. A API não sobe se o valor for vazio, `/`, ficar dentro de `frontend/public` ou se o diretório não aceitar escrita. No deploy (`deploy/docker-compose.yml`), o volume nomeado `produto_imagens` é montado em `/data/produtos-imagens`; o `backend/Dockerfile` cria o diretório com dono 65532 (usuário nonroot).

Banco criado antes do Lote 26: aplique a coluna com `make db-fix-produtos-imagem` (idempotente; revert `make db-revert-produtos-imagem`). Bancos novos já nascem com ela (`sql/10_ddl_produtos.sql`).

Para gerar uma imagem de teste para cada produto (PNG 600x600 com o nome do produto na diagonal, gravado pelo mesmo caso de uso do upload, com a mesma validação e o mesmo nome UUID):

```bash
make db-gerar-imagens-produtos                                        # só produtos sem imagem
make db-gerar-imagens-produtos GERAR_IMAGENS_FLAGS=-sobrescrever      # troca todas
```

O comando (`backend/rotaperfumes-api/cmd/gerarimagens`) aceita `-confirmar` (obrigatório para gravar; o alvo do `make` já passa), `-sobrescrever` e `-dry-run` (lista sem gravar; funciona sem `-confirmar`). No banco local (2026-10-31): 292 imagens geradas, 292 produtos com `imagem` preenchido.

### Importação de pedidos (ERP)

`make db-up`/`make db-reset` criam as tabelas `pedidos` e `itens_pedido` (via `sql/04_ddl_pedidos.sql` e `sql/11_ddl_itens_pedido.sql`), mas **não** carregam os dados nelas. Para popular as tabelas a partir de `dados/erp/pedidos.csv` (28.729 linhas) e `dados/erp/itens_pedido.csv` (197.724 linhas), rode adicionalmente:

```bash
make db-up && make db-import-pedidos
```

O importador (`backend/shared/cmd/importpedidos`) é idempotente (upsert por `pedido_id_origem`/`item_id_origem`) e importa primeiro os pedidos e depois os itens (nessa ordem, por causa da FK `itens_pedido.pedido_id`). Pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET /api/pedidos`, `GET /api/pedidos/{id}`, `POST/PUT /api/pedidos` funcionam normalmente, mas retornam/operam sobre base vazia.

### Importação de pagamentos (ERP)

`make db-up`/`make db-reset` criam a tabela `pagamentos` (via `sql/12_ddl_pagamentos.sql`), mas **não** carregam os dados nela. Para popular a tabela a partir de `dados/erp/pagamentos.csv` (~27,7 mil linhas), rode adicionalmente:

```bash
make db-up && make db-import-pagamentos
```

O importador (`backend/shared/cmd/importpagamentos`) é idempotente (upsert por `pagamento_id`, a própria PK da tabela) e resolve `pedido_id` via lookup em `pedidos.pedido_id_origem` — pagamentos cujo `pedido_id` do CSV não corresponda a nenhum pedido importado são pulados com log de aviso. Pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET /api/pagamentos`, `GET /api/pagamentos/{id}`, `POST/PUT /api/pagamentos` funcionam normalmente (inclusive para usuários `normal`, já que o acesso é comum), mas retornam/operam sobre base vazia — exceto pagamentos criados manualmente via `POST`, que exigem que o `pedido_id` informado já exista (rode `make db-import-pedidos` antes, se necessário).

### Oportunidades (CRM)

`make db-up`/`make db-reset` criam a tabela `oportunidades` (via `sql/15_ddl_oportunidades.sql`, FKs `cliente_id → clientes.cliente_id_origem` e `vendedor_id → vendedores.id`), mas **não** carregam os dados nela. Para popular a tabela `oportunidades` a partir de `dados/crm/oportunidades.csv` (5979 registros; colunas `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda`), rode adicionalmente:

```bash
make db-up && make db-import-oportunidades
```

O importador (`backend/shared/cmd/importoportunidades`) é idempotente (upsert por `oportunidade_id`) e pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET/POST /api/oportunidades` e `GET/PUT /api/oportunidades/{id}` funcionam normalmente, mas retornam/operam sobre base vazia — exceto oportunidades criadas manualmente via `POST`, que exigem que `cliente_id`/`vendedor_id` informados já existam. O mesmo vale para `GET /api/dashboard/funil` (DASH-01, Lote 22): sem a importação, o funil vem zerado.

### Visitas (CRM)

`make db-up`/`make db-reset` criam a tabela `visitas` (via `sql/16_ddl_visitas.sql`, FKs `cliente_id → clientes.cliente_id_origem` e `vendedor_id → vendedores.id`), mas **não** carregam os dados nela. Para popular a tabela `visitas` a partir de `dados/crm/visitas.csv` (37936 registros; colunas `visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min`), rode adicionalmente:

```bash
make db-up && make db-import-visitas
```

O importador (`backend/shared/cmd/importvisitas`) é idempotente (upsert por `visita_id`) e pode ser executado quantas vezes for necessário sem duplicar registros. Já foi executado com sucesso contra o banco local (37936 linhas importadas, 0 erros; segunda execução confirmou idempotência: 0 inseridos, 37936 atualizados). Sem esse passo, os endpoints `GET/POST /api/visitas` e `GET/PUT /api/visitas/{id}` funcionam normalmente, mas retornam/operam sobre base vazia — exceto visitas criadas manualmente via `POST`, que exigem que `cliente_id`/`vendedor_id` informados já existam.

### Estoque (ERP)

`make db-up`/`make db-reset` criam a tabela `estoque` (via `sql/17_ddl_estoque.sql`, último passo do `db-up` desde o DB-01, Lote 12; FK `sku → produtos.sku`, `UNIQUE (data_snapshot, sku)`; sem a coluna `origem`), mas **não** carregam os dados nela. Banco criado antes do Lote 12 com `estoque.origem`: rode `make db-fix-estoque-origem` (idempotente; reversão `make db-revert-estoque-origem`). Para popular a tabela `estoque` a partir de `dados/erp/estoque.csv` (colunas `data_snapshot,sku,saldo,ruptura`), rode adicionalmente:

```bash
make db-up && make db-import-estoque
```

O importador (`backend/shared/cmd/importestoque`) é idempotente (upsert por `(data_snapshot, sku)`). Pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET/POST /api/estoque` e `GET/PUT /api/estoque/{id}` funcionam normalmente, mas retornam/operam sobre base vazia — exceto snapshots criados manualmente via `POST`, que exigem que o `sku` informado já exista em `produtos`. Além do importador, o saldo de estoque também é alimentado automaticamente pelo hook de faturamento de pedidos (ver seção "Estoque" em Endpoints acima). Desde a migração 18 não há mais registro de qual processo gravou cada snapshot (a coluna `origem` foi removida): import e faturamento no mesmo `(data_snapshot, sku)` se sobrescrevem.

Depois, em outro terminal:

```bash
# Iniciar o frontend
make dev-frontend
```

A API ficará disponível em `http://localhost:8080` e o frontend em `http://localhost:3000`.

### Frontend em disco sem suporte a symlink (registrado no DASH-01, Lote 22, 2026-10-29)

Em discos que não aceitam links simbólicos (ex.: pendrive/HD externo em exFAT/FAT32 ou NTFS montado sem suporte a symlink), `npm ci`/`npm install` falham ao criar `node_modules/.bin`. Nesse caso:

```bash
cd frontend
npm ci --no-bin-links
```

Sem `node_modules/.bin`, os scripts do `package.json` (`npm test`, `npm run lint`, `npm run dev`) e os alvos do `make` que os usam (`make test-frontend`, `make dev-frontend`) **não rodam direto**. Chame os pacotes pelo caminho do JS:

```bash
cd frontend
node node_modules/vitest/vitest.mjs run          # testes (equivale a npm test)
node node_modules/typescript/bin/tsc --noEmit    # checagem de tipos
node node_modules/eslint/bin/eslint.js .         # lint
node node_modules/next/dist/bin/next dev         # servidor de desenvolvimento
```

Alternativa: manter o clone do projeto num disco com sistema de arquivos Linux (ext4) e usar os comandos normais. Isso é só do ambiente local: o build em Docker (`frontend/Dockerfile`, `npm ci`) não é afetado.