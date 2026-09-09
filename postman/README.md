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
4. Para logar como vendedor, use a requisição **Login — Vendedor** (`POST /api/auth/login` com `v1.henrique_rodrigues@rotaperfumes.com.br` / `Mudar@123`). Isso salva em `vendedor_token` e verifica a flag `trocar_senha: true`.

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

### Vendedor (42 vendedores)

Formato de e-mail: `v<ID>.<slug-nome>@rotaperfumes.com.br`

Exemplos:
- `v1.henrique_rodrigues@rotaperfumes.com.br` / `Mudar@123`
- `v2.carla_carvalho@rotaperfumes.com.br` / `Mudar@123`
- `v3.thiago_silva@rotaperfumes.com.br` / `Mudar@123`

> **Importante:** A senha padrão de todos os vendedores é `Mudar@123`. Ao logar com ela, a API retorna `trocar_senha: true` indicando que o frontend deve redirecionar para a tela de troca de senha.

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
- **Nota:** Vendedores com senha padrão recebem `trocar_senha: true`

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
- **Descrição:** Cria um novo usuário (senha padrão `Mudar@123`)

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
- **Descrição:** Reseta a senha de um usuário para `Mudar@123` (revoga refresh tokens)

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
- `Flag trocar_senha é true (senha padrão)` — **teste crítico para validação da feature de primeiro acesso**
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

### Atualizar Usuário
- `Status 200 OK`
- `Usuário atualizado com dados corretos`

### Ativar/Inativar
- `Status 200 OK`
- `Usuário com campo ativo retornado`

### Reset Password Admin
- `Status 200 OK`
- `Senha resetada com sucesso`

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

Depois, em outro terminal:

```bash
# Iniciar o frontend
make dev-frontend
```

A API ficará disponível em `http://localhost:8080` e o frontend em `http://localhost:3000`.