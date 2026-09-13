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
- **Body:** `{ "email": "...", "password": "..." }`
- **Resposta:** `{ "success": true, "data": { "access_token", "refresh_token", "token_type", "expires_in", "user": {...}, "trocar_senha" } }`
- **Nota:** `trocar_senha` reflete diretamente a coluna `usuarios.deve_trocar_senha` (não é mais calculado comparando a senha digitada com uma constante fixa). Fica `true` quando o usuário foi criado ou teve a senha resetada pelo admin e ainda não trocou a senha voluntariamente; volta a `false` após `POST /api/auth/reset-password`.

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
- **Resposta:** `{ id, nome, email, role, ativo, id_vendedor, created_at, ultimo_login_at }`

#### POST /api/auth/reset-password
- **Auth:** Bearer Token (qualquer role autenticado)
- **Body:** `{ "senha_atual": "...", "nova_senha": "..." }`
- **Descrição:** Usuário troca a própria senha (precisa da senha atual)

### Usuários (`/api/usuarios`) — admin only

#### GET /api/usuarios
- **Auth:** Bearer Token (admin)
- **Query:** `?page=1&limit=20`
- **Descrição:** Lista usuários paginada (total + pages)

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

### Dashboard (`/api/dashboard/*`) — admin only

#### GET /api/dashboard/metrics
- **Auth:** Bearer Token (admin)
- **Query:** `?periodo=today|month&ano=2026&mes=9`
- **Descrição:** Métricas gerais (total de vendas, pedidos, ticket médio, vendedores ativos)

#### GET /api/dashboard/vendas
- **Auth:** Bearer Token (admin)
- **Query:** `?dias=30` (padrão 30, máx 365)
- **Descrição:** Série temporal de vendas dos últimos N dias

#### GET /api/dashboard/vendedores
- **Auth:** Bearer Token (admin)
- **Query:** `?page=1&limit=20`
- **Descrição:** Ranking de vendedores (total de vendas, meta, percentual)

### Histórico de Senhas (`/api/senha-historico/*`) — admin only

#### GET /api/senha-historico
- **Auth:** Bearer Token (admin)
- **Query:** `?page=1&limit=20`
- **Descrição:** Lista global paginada de alterações de senha (com tipo, IP, user agent)

#### GET /api/senha-historico/{usuario_id}
- **Auth:** Bearer Token (admin)
- **Query:** `?page=1&limit=20`
- **Descrição:** Histórico de senhas de um usuário específico

### Clientes (`/api/clientes/*` e `/api/dashboard/clientes`) — admin only

> Base de clientes importada de `dados/crm/clientes.csv` para a tabela `clientes` (ver seção "Importação de clientes (CRM)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Clientes"** da collection.

#### GET /api/clientes
- **Auth:** Bearer Token (admin)
- **Query (todos opcionais):** `?page=1&limit=20&uf=SP&segmento=Varejo&ativo=true&q=perfumaria`
- **Descrição:** Lista clientes paginada (total + pages), com filtros exatos por `uf`/`segmento`, filtro por status (`ativo=true|false`) e busca livre (`q`) em `razao_social` OU `cnpj`

#### POST /api/clientes
- **Auth:** Bearer Token (admin)
- **Body:** `{ "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro", "data_cadastro" (opcional, "AAAA-MM-DD", default hoje) }`
- **Descrição:** Cria um novo cliente. `cliente_id_origem` é gerado automaticamente pelo sistema (`MAX + 1`) e `ativo` é sempre `true` na criação — nenhum dos dois é aceito no body. Campos obrigatórios: `razao_social`, `cnpj`, `segmento`, `cidade`, `uf` (2 letras). Retorna `201` com o cliente criado; `400` em caso de validação.
- **Débito técnico conhecido:** o próximo `cliente_id_origem` é calculado via `MAX(cliente_id_origem) + 1` sem transação/lock explícito no banco. Em criações concorrentes simultâneas (ex.: dois admins criando clientes ao mesmo tempo) existe risco teórico de colisão. Risco considerado baixo dado o baixo volume de uso desta tela (admin only), mas registrado aqui como débito técnico conhecido para eventual revisão futura (ex.: usar transação com `SELECT ... FOR UPDATE` ou coluna `AUTO_INCREMENT` dedicada).

#### GET /api/clientes/{id}
- **Auth:** Bearer Token (admin)
- **Descrição:** Retorna o detalhe de um cliente pelo `id` interno (não confundir com `cliente_id_origem`, o ID do CSV de origem)

#### PUT /api/clientes/{id}
- **Auth:** Bearer Token (admin)
- **Body:** `{ "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro", "data_cadastro" ("AAAA-MM-DD") }`
- **Descrição:** Atualiza os dados de um cliente existente. `cliente_id_origem` e `ativo` **não** são editáveis por esta rota (use `PATCH /api/clientes/{id}/inativar` para alterar `ativo`). Retorna `200` com o cliente atualizado, `404` se não existir, `400` se o payload for inválido.

#### PATCH /api/clientes/{id}/inativar
- **Auth:** Bearer Token (admin)
- **Body (opcional):** `{ "ativo": true|false }` — omitido = toggle
- **Descrição:** Ativa ou inativa o cliente

#### GET /api/dashboard/clientes
- **Auth:** Bearer Token (admin)
- **Query:** `?periodo=today|month` (padrão: `month`)
- **Descrição:** Métricas agregadas da base de clientes: `total_clientes`, `total_ativos`, `total_inativos`, `novos_no_periodo`, `por_segmento` (array `{segmento, total}`) e `por_uf` (array `{uf, total}`)

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
- `Dados de métricas presentes` (`total_vendas`, `total_pedidos`, `ticket_medio`, `periodo`)

### Dashboard — Vendas
- `Status 200`
- `Série temporal retornada`
- `Cada item tem data e valor`

### Dashboard — Vendedores
- `Status 200`
- `Lista de vendedores retornada`
- `Paginação presente`
- `Cada vendedor tem métricas`

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
- `Cliente criado com dados corretos` (`id`, `cliente_id_origem`, `razao_social`, `ativo === true`)

### Detalhe do Cliente
- `Status 200`
- `Dados do cliente presentes`

### Editar Cliente
- `Status 200 OK`
- `Cliente atualizado com dados corretos`

### Ativar/Inativar Cliente
- `Status 200 OK`
- `Cliente com campo ativo retornado`

### Dashboard — Clientes
- `Status 200`
- `Métricas de clientes presentes` (`periodo`, `total_clientes`, `total_ativos`, `total_inativos`, `novos_no_periodo`, `por_segmento`, `por_uf`)
- `por_segmento e por_uf são arrays`

## Resumo de testes por endpoint

| Request | # Testes | Salva variáveis |
|---------|----------|-----------------|
| Healthcheck | 0 | — |
| Login (admin) | 4 | `token`, `admin_token`, `refresh_token` |
| Login — Vendedor | 6 | `vendedor_token`, `refresh_token` |
| Logout | 2 | — |
| Refresh Token | 4 | `token`, `refresh_token` |
| Me | 2 | — |
| Reset Password | 2 | — |
| Listar Usuários | 3 | — |
| Criar Usuário | 2 | — |
| Atualizar Usuário | 2 | — |
| Ativar/Inativar | 2 | — |
| Reset Password Admin | 2 | — |
| Dashboard — Métricas | 2 | — |
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

## Códigos de erro comuns

| Código | Significado |
|--------|-------------|
| 400 | Body inválido ou campos obrigatórios ausentes |
| 401 | Não autenticado (token ausente/inválido) ou credenciais inválidas |
| 403 | Acesso restrito a administradores |
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

### Importação de clientes (CRM)

`make db-seed`/`make db-reset` criam a tabela `clientes` (via `sql/09_ddl_clientes.sql`), mas **não** carregam os dados nela. Para popular a tabela `clientes` a partir de `dados/crm/clientes.csv` (~3040 registros), rode adicionalmente:

```bash
make db-up && make db-import-clientes
```

O importador (`apis/shared/cmd/importclientes`) é idempotente (upsert por `cliente_id_origem`) e pode ser executado quantas vezes for necessário sem duplicar registros. Sem esse passo, os endpoints `GET /api/clientes`, `GET /api/clientes/{id}`, `PATCH /api/clientes/{id}/inativar` e `GET /api/dashboard/clientes` funcionam normalmente, mas retornam base vazia/zerada.

Depois, em outro terminal:

```bash
# Iniciar o frontend
make dev-frontend
```

A API ficará disponível em `http://localhost:8080` e o frontend em `http://localhost:3000`.