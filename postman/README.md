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

### 3. Autenticar

1. Abra a requisição **Login** (`POST /api/auth/login`) — usa `admin@rotaperfumes.com.br` por padrão.
2. Clique em **Send**.
3. Os tokens serão salvos automaticamente nas variáveis `token`, `admin_token` e `refresh_token` via script de testes.
4. Para logar como vendedor, use a requisição **Login — Vendedor** (`POST /api/auth/login` com `henrique.rodrigues@rotaperfumes.com.br` / `Mudar@123`, credenciais de seed). Isso salva em `vendedor_token` e verifica a flag `trocar_senha: true`.

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

Exemplos:
- `henrique.rodrigues@rotaperfumes.com.br` / `Mudar@123`
- `carla.carvalho@rotaperfumes.com.br` / `Mudar@123`
- `thiago.silva@rotaperfumes.com.br` / `Mudar@123`

> **Importante:** essas senhas fixas (`Mudar@123`) existem **apenas nos dados de seed** carregados por `make db-reset` (para permitir login de teste sem precisar ler email). Ao logar com um vendedor de seed, a API retorna `trocar_senha: true` — vindo da coluna `usuarios.deve_trocar_senha` — indicando que o frontend deve redirecionar para a tela de troca de senha.
>
> **Fluxo real de senha (produção/novos usuários):** a partir da criação de usuário (`POST /api/usuarios`) ou reset de senha pelo admin (`POST /api/admin/reset-password`), a API **não usa mais senha fixa**. Ela gera uma senha aleatória segura (`crypto/rand`, mínimo 12 caracteres, com maiúscula/minúscula/dígito/símbolo garantidos) e a envia por email (SMTP) ao endereço cadastrado do usuário. A senha gerada nunca é exposta pela API — nem em logs, nem na resposta HTTP. Ambas as respostas (criação e reset) incluem um campo `email_enviado: boolean` informando se o envio de fato ocorreu (será `false` se o SMTP não estiver configurado, caso em que a API usa um serviço "noop" que apenas loga o envio, sem quebrar a operação de criação/reset).

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

#### POST /api/auth/refresh
- **Auth:** nenhuma (usa `refresh_token` no body)
- **Body:** `{ "refresh_token": "..." }`
- **Resposta:** Novo par `{ "access_token", "refresh_token", "token_type", "expires_in" }`
- **Nota:** O refresh token antigo é revogado (single-use)

#### POST /api/auth/logout
- **Auth:** nenhuma (usa `refresh_token` no body)
- **Body (opcional):** `{ "refresh_token": "..." }`
- **Resposta:** `{ "success": true, "data": { "mensagem": "logout realizado" } }`

#### GET /api/auth/me
- **Auth:** Bearer Token
- **Resposta:** `{ id, nome, email, role, ativo, id_vendedor, created_at, ultimo_login_at, vendedor_desligado }`
- **`vendedor_desligado` (bool, 2026-09-24):** `true` só para usuário `normal` vinculado a vendedor com `data_desligamento` preenchida. É `false` para admin, para usuário sem vínculo e para vínculo órfão (vendedor inexistente). O `/me` continua acessível para o vendedor desligado (não retorna `403`). O frontend usa este campo e o `id_vendedor` atualizado para revalidar a sessão (em vez de confiar só no `localStorage`) e exibir o aviso.
- **Erros:** `401` não autenticado; `404` usuário não encontrado; `500` erro interno (inclui falha ao checar se o vendedor está desligado).

#### POST /api/auth/reset-password
- **Auth:** Bearer Token (qualquer role autenticado)
- **Body:** `{ "senha_atual": "...", "nova_senha": "...", "captchaToken": "..." }`
- **Descrição:** Usuário troca a própria senha (precisa da senha atual)
- **CAPTCHA:** `captchaToken` (string, **obrigatório**) — mesma validação Turnstile do login (fail-closed). Endpoint também ganhou rate limiting dedicado: 10 falhas em 5 minutos bloqueiam por 10 minutos (lacuna identificada pelo SecBrain — antes não existia rate limiting aqui).

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

#### PUT /api/usuarios/{id}
- **Auth:** Bearer Token (admin)
- **Body:** `{ "nome", "role": "admin"|"normal" }`
- **Descrição:** Atualiza nome e/ou role

#### PATCH /api/usuarios/{id}/inativar
- **Auth:** Bearer Token (admin)
- **Body (opcional):** `{ "ativo": true|false }` — omitido = toggle
- **Descrição:** Ativa ou inativa o usuário

### Admin (`/api/admin/*`) — admin only

#### POST /api/admin/reset-password
- **Auth:** Bearer Token (admin)
- **Body:** `{ "usuario_id": <int64> }`
- **Descrição:** Reseta a senha de um usuário para uma senha aleatória gerada pela API, enviada por email ao endereço cadastrado (revoga refresh tokens; `deve_trocar_senha` volta a `true`). Resposta inclui `email_enviado: boolean`.

### Bloqueio de vendedor desligado nas rotas da carteira (2026-09-24)

> **Decisão do usuário (2026-09-24):** o usuário `normal` vinculado a um vendedor desligado (`vendedores.data_desligamento` preenchida) fica bloqueado em **todas as rotas da carteira**, na leitura e na escrita. Contrato definido pelo 🟣 SecBrain e aplicado pelo 🟡 BackBrain em `apis/rotaperfumes-api/handlers/scope.go` (`resolverVendedorScope(r, db)` + `responderErroEscopo`).
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
>
> - **Continuam acessíveis:** login, `POST /api/auth/refresh`, `GET /api/auth/me` (com `vendedor_desligado: true`) e o Dashboard (`/api/dashboard/*` responde `200` zerado, e `/metrics` traz `vendedor_desligado: true`; ver seção abaixo, **sem mudança** neste lote).
> - **Checagem a cada requisição:** sem cache e sem depender de novo JWT. O desligamento bloqueia imediatamente, e a reativação libera imediatamente.
> - **Admin** não é afetado. **Usuário `normal` sem vendedor vinculado** mantém o comportamento anterior. **Vínculo órfão** (vendedor inexistente) é tratado como "sem vendedor".
> - **Falha ao checar o vendedor no banco:** `500` `"erro interno"` (fail-closed).
> - Na collection, cada pasta (Clientes, Pedidos, Pagamentos, Oportunidades, Visitas) tem uma nota na descrição e o exemplo `403 Forbidden — Vendedor desligado (usuário normal)` na listagem e na criação, e também em "Listar Clientes do Vendedor".
>
> **Datas (BUG-01, 2026-09-24):** o formato das datas nas respostas **não mudou**. A correção (`time.ParseInLocation(..., time.Local)`) só eliminou o deslocamento de um dia na gravação: a data enviada como `AAAA-MM-DD` é gravada e lida no mesmo dia, inclusive em re-salvamentos.
>
> **Fuso fixo (RISCO-01, 2026-09-24):** o processo da API (e o `resetpassword`) passou a usar `time.Local` = fuso fixo `-03:00`, sem horário de verão (pacote `apis/shared/tz`). O formato das datas nas respostas **não mudou**. Datas históricas de início de horário de verão (ex.: `2018-11-04`) não recuam mais um dia em Linux/Docker com `TZ=America/Sao_Paulo`.
>
> **Salvar sem alterações (BUG-04, 2026-09-24):** qualquer `PUT`/`PATCH` que envie os mesmos valores já gravados responde `200` com o registro. Antes respondia `404` "não encontrado", porque o MySQL devolvia `RowsAffected=0`. O DSN agora usa `clientFoundRows=true`. Id inexistente continua `404`. `DELETE /api/vendedores/{id}` em vendedor já desligado responde `200` e preserva a `data_desligamento` original.

### Dashboard (`/api/dashboard/*`) — acesso comum com escopo por vendedor

> **Escopo por vendedor (2026-09-23):** `/api/dashboard/metrics`, `/api/dashboard/vendas` e `/api/dashboard/vendedores` aceitam qualquer usuário autenticado e filtram os números pelo vendedor vinculado ao usuário (`pedidos.vendedor_id`). O filtro é resolvido no handler (`resolverEscopo`):
> - **admin:** vê todos os números, sem filtro.
> - **normal com vendedor vinculado:** vê só os próprios números. `meta_mes` é a meta do próprio vendedor. `top_vendedores`, `metas_vendedores` e o ranking de `/vendedores` trazem no máximo a linha dele.
> - **normal sem vendedor vinculado:** recebe `200` com tudo zerado ou vazio (não é erro). `/metrics` volta zerado, com `top_vendedores: []` e `metas_vendedores: []`. `/vendas` volta com os N dias, todos zerados. `/vendedores` volta com `data: []` e `total=0`, `pages=0`.
> - **normal com vendedor desligado ou inexistente (2026-09-24, decisão do usuário: bloquear o acesso):** se o vendedor vinculado tem `vendedores.data_desligamento` preenchida (ou não existe mais), os quatro endpoints (`/metrics`, `/vendas`, `/vendedores`, `/clientes`) respondem como no caso "sem vendedor vinculado", zerados ou vazios. `/metrics` traz `vendedor_desligado: true` para o frontend mostrar o aviso. Se a checagem do vendedor falhar no banco, a resposta é `500`.
> - **Campo `vendedor_desligado` (bool, só em `/metrics`):** `true` apenas para o usuário normal com vendedor desligado. É `false` em todos os outros casos, inclusive admin, normal com vendedor ativo e normal sem vendedor.
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

### Histórico de Senhas (`/api/senha-historico/*`) — admin only

#### GET /api/senha-historico
- **Auth:** Bearer Token (admin)
- **Query:** `?page=1&limit=20&order_by=created_at&order_dir=desc`
- **Descrição:** Lista global paginada de alterações de senha (com tipo, IP, user agent)
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, usuario_id, tipo_reset, created_at` (default: `id`); `order_dir` aceita `asc`|`desc` case-insensitive (default: `desc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

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
> **Acesso comum com escopo por carteira:** todas as rotas `/api/clientes*` usam `JWTMiddleware(cfg, true, false)` (qualquer usuário autenticado). O escopo é aplicado no handler (`apis/rotaperfumes-api/handlers/cliente_handler.go`). O admin não sofre restrições.
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

#### GET /api/clientes
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo pela carteira para o `normal`)
- **Query (todos opcionais):** `?page=1&limit=20&uf=SP&segmento=Varejo&ativo=true&q=perfumaria&order_by=razao_social&order_dir=asc`
- **Descrição:** Lista clientes paginada (total + pages), com filtros exatos por `uf`/`segmento`, filtro por status (`ativo=true|false`) e busca livre (`q`) em `razao_social` OU `cnpj`
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, razao_social, cnpj, segmento, cidade, uf, data_cadastro, ativo, created_at, updated_at` (default: `id`; `id` é um alias de coluna aceito pela API, mapeado para `cliente_id_origem` — não existe mais campo `id` na resposta); `order_dir` aceita `asc`|`desc` case-insensitive (default: `asc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/clientes
- **Auth:** Bearer Token (qualquer usuário autenticado; `normal` precisa ter vendedor vinculado)
- **Escopo (SEC-01):** usuário `normal` sem vendedor → `403` `"usuário sem vendedor vinculado"`. Com vendedor → `201`, e o cliente é vinculado automaticamente à carteira do vendedor. Admin: sem mudança.
- **Body:** `{ "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro", "data_cadastro" (opcional, "AAAA-MM-DD", default hoje) }`
- **Descrição:** Cria um novo cliente. `cliente_id_origem` é a PK `BIGINT AUTO_INCREMENT` da tabela, gerada nativamente pelo MySQL (não é aceita no body), e `ativo` é sempre `true` na criação. Campos obrigatórios: `razao_social`, `cnpj`, `segmento`, `cidade`, `uf` (2 letras). Retorna `201` com o cliente criado; `400` em caso de validação. Não existe mais campo `id` — `cliente_id_origem` é o único identificador.
- **Nota histórica (resolvida):** versões anteriores geravam `cliente_id_origem` via `MAX(cliente_id_origem) + 1` sem transação/lock explícito, com risco teórico de colisão em criações concorrentes. Esse débito técnico foi eliminado na tarefa "Promover colunas \*_id_origem a PK autoincremento" (2026-09-15) — a geração agora é feita nativamente pelo MySQL via `AUTO_INCREMENT`.

#### GET /api/clientes/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado; `404` fora da carteira para o `normal`)
- **Descrição:** Retorna o detalhe de um cliente pelo `cliente_id_origem` (PK da tabela; o path param continua se chamando `id` na rota, mas não existe mais campo `id` separado na resposta)

#### PUT /api/clientes/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo pela carteira ativa para o `normal`)
- **Escopo (SEC-01):** usuário `normal` fora da carteira ativa, ou sem vendedor → `404` `"cliente não encontrado"`. Falha ao checar a carteira → `500`. Vendedor desligado → `403`.
- **Sem alterações (BUG-04):** `200`.
- **Body:** `{ "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro", "data_cadastro" ("AAAA-MM-DD") }`
- **Descrição:** Atualiza os dados de um cliente existente. `cliente_id_origem` e `ativo` **não** são editáveis por esta rota (use `PATCH /api/clientes/{id}/inativar` para alterar `ativo`). Retorna `200` com o cliente atualizado, `404` se não existir, `400` se o payload for inválido.

#### PATCH /api/clientes/{id}/inativar
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo pela carteira ativa para o `normal`)
- **Body (opcional):** `{ "ativo": true|false }` — omitido = toggle
- **Descrição:** Ativa ou inativa o cliente
- **Escopo (SEC-01):** usuário `normal` fora da carteira ativa, ou sem vendedor → `404` `"cliente não encontrado"`. Falha ao checar a carteira → `500`. Vendedor desligado → `403`.
- **Mesmo valor (BUG-04):** enviar `ativo` igual ao estado atual responde `200`.

#### GET /api/dashboard/clientes
- **Auth:** Bearer Token (qualquer usuário autenticado; escopo pela carteira do vendedor para o normal). A rota usa `JWTMiddleware(cfg, true, false)`, então não há `403`.
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
- **Descrição:** Cria um novo produto. `ativo` é sempre `true` na criação — não é aceito no body. Campos obrigatórios: `sku`, `descricao`, `categoria`, `marca`, `unidade`; `preco_tabela`/`custo_unitario` devem ser `>= 0`. Retorna `201` com o produto criado; `400` em caso de validação.

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

### Pedidos (`/api/pedidos/*`) — **acesso comum, com escopo por carteira** (atualizado em 2026-09-23)

> Tela master-detail: lista de pedidos + sub-lista de itens (produtos) de cada pedido. Base importada de `dados/erp/pedidos.csv` (28.729 linhas) e `dados/erp/itens_pedido.csv` (197.724 linhas) para as tabelas `pedidos`/`itens_pedido` (ver seção "Importação de pedidos (ERP)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Pedidos"** da collection.
>
> **Escopo por carteira em Create/Update (2026-09-23):** todas as rotas de Pedidos usam a cadeia `middleware.JWTMiddleware(cfg, true, false)` (qualquer usuário autenticado). A restrição de dono é aplicada no handler (`apis/rotaperfumes-api/handlers/pedido_handler.go`, mesmo padrão `vendedorScope`/`resolverVendedorScope` de Oportunidades/Visitas). Para usuário `normal`:
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
- **Descrição:** Cria um novo pedido com seus itens. `valor_bruto` de cada item e `valor_total` do pedido são calculados no backend (não aceitos no body). `pedido_id_origem` é a PK `BIGINT AUTO_INCREMENT` gerada nativamente pelo MySQL. Exige ao menos um item. Retorna `201` com o pedido criado (incluindo itens); `400` em caso de validação. Não existe mais campo `id` — `pedido_id_origem`/`item_id_origem` são os identificadores.
- **Escopo (usuário `normal`, 2026-09-23):** `403` `"usuário sem vendedor vinculado"` se não houver vendedor vinculado; `vendedor_id` do body é ignorado e forçado ao vendedor do usuário; `400` `"cliente não pertence à carteira deste vendedor"` se `cliente_id` estiver fora da carteira ativa.

#### GET /api/pedidos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira; `404` para pedido de outro vendedor)
- **Descrição:** Retorna o detalhe de um pedido pelo `pedido_id_origem` (PK; o path param continua se chamando `id` na rota), incluindo a lista de itens (`itens`, cada um enriquecido com `produto_sku`/`produto_descricao` via JOIN) — usado na tela master-detail.

#### PUT /api/pedidos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Body:** mesmo formato do `POST /api/pedidos`
- **Descrição:** Atualiza os dados de um pedido existente e substitui integralmente a lista de itens (delete + insert). `valor_bruto`/`valor_total` são recalculados no backend. Retorna `200` com o pedido atualizado (incluindo itens), `404` se não existir, `400` se o payload for inválido.
- **Escopo (usuário `normal`, 2026-09-23):** `404` `"pedido não encontrado"` (mesmo corpo de inexistente) se o usuário não tiver vendedor vinculado ou o pedido for de outro vendedor; `vendedor_id` do body é forçado ao vendedor do usuário; `400` `"cliente não pertence à carteira deste vendedor"` se `cliente_id` estiver fora da carteira ativa.

#### DELETE /api/pedidos/{id} (adicionado em 2026-09-22)
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal, com escopo por carteira)
- **Descrição:** Exclui definitivamente um pedido e seus itens (hard delete, sem soft-delete). Bloqueado com `409` se o pedido tiver **pagamentos vinculados** (`"pedido possui pagamentos vinculados: remova-os antes de excluir o pedido"`) ou se já estiver com `status = "Faturado"` (`"pedido faturado não pode ser excluído, apenas ter o status alterado"` — nesse caso a única alteração permitida é a de status, via `PUT`). Escopo idêntico ao `GET`/`PUT`: `404` (não `403`) se o pedido pertencer a outro vendedor. Retorna `204` sem corpo em caso de sucesso.

### Pagamentos (`/api/pagamentos/*`) — **acesso comum, com escopo por carteira**

> **Pagamentos é liberado a qualquer usuário autenticado — `admin` ou `normal`**, como Clientes, Pedidos, Oportunidades e Visitas. A cadeia de middleware usada é `middleware.JWTMiddleware(cfg, true, false)` (`requireAuth=true`, `requireAdmin=false`); as rotas admin only (Usuários, Estoque, escrita de Produtos, gestão de Vendedores) usam `(cfg, true, true)`. Base importada de `dados/erp/pagamentos.csv` (~27,7 mil linhas) para a tabela `pagamentos` (ver seção "Importação de pagamentos (ERP)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Pagamentos"** da collection e usam `{{vendedor_token}}` (usuário `normal`) nos exemplos, em vez de `{{admin_token}}`, para deixar explícito que o acesso é comum.
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
- **Descrição:** Cria um novo pagamento. `pedido_id` deve existir em `pedidos`. `valor_liquido` é sempre exigido explicitamente no payload — não é calculado automaticamente a partir de `valor`/`taxa_pct` (o CSV de origem já traz o valor líquido calculado, às vezes com pequenas diferenças de arredondamento em relação ao cálculo direto). `pagamento_id` é gerado automaticamente (AUTO_INCREMENT). Retorna `201` com o pagamento criado; `400` em caso de validação (inclusive `pedido_id` ausente ou `<= 0`); `404` `"pedido não encontrado"` se `pedido_id` não existir (admin e normal, desde 2026-09-24).
- **Escopo (usuário `normal`, 2026-09-23):** `403` `"usuário sem vendedor vinculado"` se não houver vendedor vinculado; `404` `"pedido não encontrado"` se `pedido_id` pertencer a outro vendedor ou não existir.

#### GET /api/pagamentos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Descrição:** Retorna o detalhe de um pagamento pelo `pagamento_id`.

#### PUT /api/pagamentos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Body:** mesmo formato do `POST /api/pagamentos`, exceto `pedido_id`
- **Descrição:** Atualiza os campos editáveis de um pagamento existente. `pagamento_id` e `pedido_id` **não** são editáveis por esta rota (o vínculo com o pedido de origem é definitivo — para reatribuir a outro pedido, o fluxo correto é excluir/recriar). Retorna `200` com o pagamento atualizado, `404` se não existir, `400` se o payload for inválido.
- **Escopo (usuário `normal`, 2026-09-23):** `404` `"pagamento não encontrado"` se o usuário não tiver vendedor vinculado ou o pagamento pertencer a pedido de outro vendedor.

#### DELETE /api/pagamentos/{id} (adicionado em 2026-09-22)
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Descrição:** Exclui definitivamente um pagamento (hard delete, sem soft-delete). Bloqueado com `409` se `status_pagamento` já for `"Pago"` ou `"Pago com atraso"` (`"pagamento já quitado não pode ser excluído"`) — preserva a trilha financeira de pagamentos já quitados. Escopo idêntico ao `GET`/`PUT`: `404` (não `403`) se o pagamento pertencer a pedido de outro vendedor. Retorna `204` sem corpo em caso de sucesso.

### Vendedores (`/api/vendedores/*`) — admin only, exceto 2 GETs (atualizado em 2026-09-23)

> Até 2026-09-22 todas as rotas `/api/vendedores*` usavam `JWTMiddleware(cfg, true, false)` (acesso comum) — a restrição a admin existia só no frontend. O 🟣 SecBrain identificou que um usuário `normal` podia gerenciar vendedores via API, ler a carteira de qualquer vendedor (`GET /api/vendedores/{id}`) e — crítico — transferir clientes de outros vendedores para a própria carteira (`POST /api/vendedores/{id}/clientes`). Desde 2026-09-23 (`routes/routes.go`):

| Método | Rota | Acesso | Usuário `normal` |
|--------|------|--------|------------------|
| GET | `/api/vendedores` | acesso comum | `200` (lista para popular selects) |
| GET | `/api/vendedores/{id}/clientes` | acesso comum | `200` (dropdown em cascata Vendedor → Cliente) |
| POST | `/api/vendedores` | admin only | `403` |
| GET | `/api/vendedores/{id}` | admin only | `403` |
| PUT | `/api/vendedores/{id}` | admin only | `403` |
| DELETE | `/api/vendedores/{id}` | admin only (inativa via `data_desligamento`; em vendedor já desligado responde `200` e preserva a `data_desligamento` original — BUG-04, 2026-09-24) | `403` |
| POST | `/api/vendedores/{id}/reativar` | admin only | `403` |
| POST | `/api/vendedores/{id}/clientes` | admin only (vincula/transfere cliente) | `403` |
| DELETE | `/api/vendedores/{id}/clientes/{clienteId}` | admin only (encerra vínculo) | `403` |

> O `403` retorna `{"success": false, "error": "acesso restrito a administradores"}`. 
>
> **`meta_mensal` fora da listagem (confirmado em 2026-09-24):** `GET /api/vendedores` devolve `VendedorResumo` (`id`, `nome`, `regiao`, `uf`, `data_desligamento`) para **todos** os perfis, sem `meta_mensal`. O usuário `normal` não recebe a meta de ninguém. O admin obtém `meta_mensal` pelo detalhe `GET /api/vendedores/{id}` (admin only). O código já estava assim; o 🔴 TestBrain adicionou um teste de regressão.

### Oportunidades (`/api/oportunidades/*`) — **acesso comum, com escopo por carteira** (atualizado em 2026-09-22)

> Funil de vendas (CRM) importado de `dados/crm/oportunidades.csv` (colunas: `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda`). Tela com dois dropdowns em cascata (Vendedor → Cliente) e filtros de coluna na listagem. Requests desses endpoints estão agrupadas na pasta **"Oportunidades"** da collection.
>
> **Botão de exclusão (2026-09-22):** adicionado endpoint `DELETE /api/oportunidades/{id}` — hard delete (sem exclusão lógica), sem restrição extra de negócio. Ver detalhes abaixo.
>
> **Mudança de acesso (2026-09-22):** `GET/POST /api/oportunidades` e `GET/PUT /api/oportunidades/{id}` deixaram de ser `admin only` e passaram a **acesso comum com escopo por carteira** — mesma cadeia de middleware usada em Pagamentos (`cfg, true, false`), com a restrição de dono aplicada no handler via `vendedorScope`/`resolverVendedorScope` (`apis/rotaperfumes-api/handlers/scope.go`), o mesmo padrão já usado em Pedidos/Clientes/Pagamentos:
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
- **Descrição:** Cria um snapshot de estoque manual. `sku` deve existir em `produtos`; `ruptura` é derivada automaticamente (`saldo <= 0`, não aceita no body). Respeita a constraint `UNIQUE (data_snapshot, sku)` — criar outro registro para o mesmo par retorna `400`. Retorna `201` com o snapshot criado.

#### PUT /api/estoque/{id}
- **Auth:** Bearer Token (admin)
- **Body:** `{ "saldo" }`
- **Descrição:** Atualiza o saldo de um snapshot existente. `sku` e `data_snapshot` **não** são editáveis por esta rota (imutáveis). `ruptura` é recalculada automaticamente. Retorna `200` com o snapshot atualizado, `404` se não existir, `400` se o payload for inválido.

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
| 400 | Body inválido ou campos obrigatórios ausentes |
| 401 | Não autenticado (token ausente/inválido) ou credenciais inválidas |
| 403 | Acesso restrito a administradores; usuário `normal` sem vendedor vinculado (escritas); ou `"acesso bloqueado: vendedor desligado"` nas rotas da carteira (2026-09-24) |
| 404 | Recurso não encontrado |
| 409 | Conflito (ex.: e-mail duplicado) |
| 500 | Erro interno do servidor |

## Subindo o ambiente

```bash
# 1. Recriar o banco com seed
make db-reset

# 2. Gerar hashes bcrypt reais (substitui placeholders no seed)
make gen-hash

# 3. Iniciar a API
make dev-api
```

### Envio de email (senha inicial / reset de senha)

Copie `.env.example` (raiz do projeto) para `.env` e preencha as variáveis `SMTP_*` (Gmail com "senha de app") para que `POST /api/usuarios` e `POST /api/admin/reset-password` realmente enviem a senha gerada por email. Se essas variáveis não forem preenchidas, a API sobe normalmente e usa um serviço de email "noop" (apenas loga que o envio foi pulado, sem nunca logar a senha em texto claro) — útil para dev local, mas nesse caso `email_enviado` retorna `false` e o usuário não recebe a nova senha por nenhum canal (o admin precisaria providenciá-la manualmente).

### CAPTCHA (Cloudflare Turnstile) em Login e Troca de Senha

`POST /api/auth/login` e `POST /api/auth/reset-password` exigem o campo `captchaToken` no body — um token gerado pelo widget Cloudflare Turnstile, validado server-side (fail-closed, timeout 4s) contra `https://challenges.cloudflare.com/turnstile/v0/siteverify`. O serviço fica em `apis/shared/services/turnstile_service.go`.

Para configurar:
1. Crie um site no [dashboard do Cloudflare Turnstile](https://dash.cloudflare.com/?to=/:account/turnstile) e copie a **Site Key** (pública) e a **Secret Key** (privada).
2. Backend: copie `.env.example` (raiz do projeto) para `.env` e preencha `TURNSTILE_SECRET_KEY` com a Secret Key. **Nunca** faça commit dessa chave.
3. Frontend: preencha `NEXT_PUBLIC_TURNSTILE_SITE_KEY` em `frontend/.env.local` com a Site Key (ver `frontend/.env.local.example`).

Sem `captchaToken` válido, ambos os endpoints respondem `401`. `POST /api/auth/reset-password` também ganhou rate limiting dedicado (10 falhas/5min, bloqueio de 10min), lacuna identificada pelo SecBrain durante a revisão desta feature.

### Importação de clientes (CRM)

`make db-seed`/`make db-reset` criam a tabela `clientes` (via `sql/09_ddl_clientes.sql`), mas **não** carregam os dados nela. Para popular a tabela `clientes` a partir de `dados/crm/clientes.csv` (~3040 registros), rode adicionalmente:

```bash
make db-up && make db-import-clientes
```

O importador (`apis/shared/cmd/importclientes`) é idempotente (upsert por `cliente_id_origem`) e pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET /api/clientes`, `GET /api/clientes/{id}`, `PATCH /api/clientes/{id}/inativar` e `GET /api/dashboard/clientes` funcionam normalmente, mas retornam base vazia/zerada.

### Importação de produtos (ERP)

`make db-up`/`make db-reset` criam a tabela `produtos` (via `sql/10_ddl_produtos.sql`), mas **não** carregam os dados nela. Para popular a tabela `produtos` a partir de `dados/erp/produtos.csv` (293 registros), rode adicionalmente:

```bash
make db-up && make db-import-produtos
```

O importador (`apis/shared/cmd/importprodutos`) é idempotente (upsert por `sku`) e pode ser executado quantas vezes for necessário sem duplicar registros. Itens do CSV são importados com `ativo = true` por padrão. Sem esse passo, os endpoints `GET /api/produtos`, `GET /api/produtos/{id}`, `POST/PUT /api/produtos` e `PATCH /api/produtos/{id}/inativar` funcionam normalmente, mas retornam/operam sobre base vazia.

### Importação de pedidos (ERP)

`make db-up`/`make db-reset` criam as tabelas `pedidos` e `itens_pedido` (via `sql/04_ddl_pedidos.sql` e `sql/11_ddl_itens_pedido.sql`), mas **não** carregam os dados nelas. Para popular as tabelas a partir de `dados/erp/pedidos.csv` (28.729 linhas) e `dados/erp/itens_pedido.csv` (197.724 linhas), rode adicionalmente:

```bash
make db-up && make db-import-pedidos
```

O importador (`apis/shared/cmd/importpedidos`) é idempotente (upsert por `pedido_id_origem`/`item_id_origem`) e importa primeiro os pedidos e depois os itens (nessa ordem, por causa da FK `itens_pedido.pedido_id`). Pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET /api/pedidos`, `GET /api/pedidos/{id}`, `POST/PUT /api/pedidos` funcionam normalmente, mas retornam/operam sobre base vazia.

### Importação de pagamentos (ERP)

`make db-up`/`make db-reset` criam a tabela `pagamentos` (via `sql/12_ddl_pagamentos.sql`), mas **não** carregam os dados nela. Para popular a tabela a partir de `dados/erp/pagamentos.csv` (~27,7 mil linhas), rode adicionalmente:

```bash
make db-up && make db-import-pagamentos
```

O importador (`apis/shared/cmd/importpagamentos`) é idempotente (upsert por `pagamento_id`, a própria PK da tabela) e resolve `pedido_id` via lookup em `pedidos.pedido_id_origem` — pagamentos cujo `pedido_id` do CSV não corresponda a nenhum pedido importado são pulados com log de aviso. Pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET /api/pagamentos`, `GET /api/pagamentos/{id}`, `POST/PUT /api/pagamentos` funcionam normalmente (inclusive para usuários `normal`, já que o acesso é comum), mas retornam/operam sobre base vazia — exceto pagamentos criados manualmente via `POST`, que exigem que o `pedido_id` informado já exista (rode `make db-import-pedidos` antes, se necessário).

### Oportunidades (CRM)

`make db-up`/`make db-reset` criam a tabela `oportunidades` (via `sql/15_ddl_oportunidades.sql`, FKs `cliente_id → clientes.cliente_id_origem` e `vendedor_id → vendedores.id`), mas **não** carregam os dados nela. Para popular a tabela `oportunidades` a partir de `dados/crm/oportunidades.csv` (5979 registros; colunas `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda`), rode adicionalmente:

```bash
make db-up && make db-import-oportunidades
```

O importador (`apis/shared/cmd/importoportunidades`) é idempotente (upsert por `oportunidade_id`) e pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET/POST /api/oportunidades` e `GET/PUT /api/oportunidades/{id}` funcionam normalmente, mas retornam/operam sobre base vazia — exceto oportunidades criadas manualmente via `POST`, que exigem que `cliente_id`/`vendedor_id` informados já existam.

### Visitas (CRM)

`make db-up`/`make db-reset` criam a tabela `visitas` (via `sql/16_ddl_visitas.sql`, FKs `cliente_id → clientes.cliente_id_origem` e `vendedor_id → vendedores.id`), mas **não** carregam os dados nela. Para popular a tabela `visitas` a partir de `dados/crm/visitas.csv` (37936 registros; colunas `visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min`), rode adicionalmente:

```bash
make db-up && make db-import-visitas
```

O importador (`apis/shared/cmd/importvisitas`) é idempotente (upsert por `visita_id`) e pode ser executado quantas vezes for necessário sem duplicar registros. Já foi executado com sucesso contra o banco local (37936 linhas importadas, 0 erros; segunda execução confirmou idempotência: 0 inseridos, 37936 atualizados). Sem esse passo, os endpoints `GET/POST /api/visitas` e `GET/PUT /api/visitas/{id}` funcionam normalmente, mas retornam/operam sobre base vazia — exceto visitas criadas manualmente via `POST`, que exigem que `cliente_id`/`vendedor_id` informados já existam.

### Estoque (ERP)

`make db-up`/`make db-reset` criam a tabela `estoque` (via `sql/17_ddl_estoque.sql`, FK `sku → produtos.sku`, `UNIQUE (data_snapshot, sku)`), mas **não** carregam os dados nela. Para popular a tabela `estoque` a partir de `dados/erp/estoque.csv` (colunas `data_snapshot,sku,saldo,ruptura`), rode adicionalmente:

```bash
make db-up && make db-import-estoque
```

O importador (`apis/shared/cmd/importestoque`) é idempotente (upsert por `(data_snapshot, sku)`) e grava `origem=import_csv`. Pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET/POST /api/estoque` e `GET/PUT /api/estoque/{id}` funcionam normalmente, mas retornam/operam sobre base vazia — exceto snapshots criados manualmente via `POST`, que exigem que o `sku` informado já exista em `produtos`. Além do importador, o saldo de estoque também é alimentado automaticamente pelo hook de faturamento de pedidos (`origem=faturamento`, ver seção "Estoque" em Endpoints acima).

Depois, em outro terminal:

```bash
# Iniciar o frontend
make dev-frontend
```

A API ficará disponível em `http://localhost:8080` e o frontend em `http://localhost:3000`.