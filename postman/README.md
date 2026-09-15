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
- **Query:** `?page=1&limit=20&order_by=created_at&order_dir=desc`
- **Descrição:** Lista global paginada de alterações de senha (com tipo, IP, user agent)
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, usuario_id, tipo_reset, created_at` (default: `id`); `order_dir` aceita `asc`|`desc` case-insensitive (default: `desc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### GET /api/senha-historico/{usuario_id}
- **Auth:** Bearer Token (admin)
- **Query:** `?page=1&limit=20&order_by=created_at&order_dir=desc`
- **Descrição:** Histórico de senhas de um usuário específico
- **Ordenação:** mesmas regras de `GET /api/senha-historico` acima (`order_by`/`order_dir`).

### Clientes (`/api/clientes/*` e `/api/dashboard/clientes`) — admin only

> Base de clientes importada de `dados/crm/clientes.csv` para a tabela `clientes` (ver seção "Importação de clientes (CRM)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Clientes"** da collection.

#### GET /api/clientes
- **Auth:** Bearer Token (admin)
- **Query (todos opcionais):** `?page=1&limit=20&uf=SP&segmento=Varejo&ativo=true&q=perfumaria&order_by=razao_social&order_dir=asc`
- **Descrição:** Lista clientes paginada (total + pages), com filtros exatos por `uf`/`segmento`, filtro por status (`ativo=true|false`) e busca livre (`q`) em `razao_social` OU `cnpj`
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, razao_social, cnpj, segmento, cidade, uf, data_cadastro, ativo, created_at, updated_at` (default: `id`; `id` é um alias de coluna aceito pela API, mapeado para `cliente_id_origem` — não existe mais campo `id` na resposta); `order_dir` aceita `asc`|`desc` case-insensitive (default: `asc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/clientes
- **Auth:** Bearer Token (admin)
- **Body:** `{ "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro", "data_cadastro" (opcional, "AAAA-MM-DD", default hoje) }`
- **Descrição:** Cria um novo cliente. `cliente_id_origem` é a PK `BIGINT AUTO_INCREMENT` da tabela, gerada nativamente pelo MySQL (não é aceita no body), e `ativo` é sempre `true` na criação. Campos obrigatórios: `razao_social`, `cnpj`, `segmento`, `cidade`, `uf` (2 letras). Retorna `201` com o cliente criado; `400` em caso de validação. Não existe mais campo `id` — `cliente_id_origem` é o único identificador.
- **Nota histórica (resolvida):** versões anteriores geravam `cliente_id_origem` via `MAX(cliente_id_origem) + 1` sem transação/lock explícito, com risco teórico de colisão em criações concorrentes. Esse débito técnico foi eliminado na tarefa "Promover colunas \*_id_origem a PK autoincremento" (2026-09-15) — a geração agora é feita nativamente pelo MySQL via `AUTO_INCREMENT`.

#### GET /api/clientes/{id}
- **Auth:** Bearer Token (admin)
- **Descrição:** Retorna o detalhe de um cliente pelo `cliente_id_origem` (PK da tabela; o path param continua se chamando `id` na rota, mas não existe mais campo `id` separado na resposta)

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

### Produtos (`/api/produtos/*`) — admin only

> Base de produtos importada de `dados/erp/produtos.csv` (293 linhas) para a tabela `produtos` (ver seção "Importação de produtos (ERP)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Produtos"** da collection. A exclusão de produtos é sempre **lógica** (flag `ativo`) — não existe endpoint de `DELETE`.

#### GET /api/produtos
- **Auth:** Bearer Token (admin)
- **Query (todos opcionais):** `?page=1&limit=20&categoria=Masculino&marca=Rota&ativo=true&q=intense&order_by=descricao&order_dir=asc`
- **Descrição:** Lista produtos paginada (total + pages), com filtros exatos por `categoria`/`marca`, filtro por status (`ativo=true|false`) e busca livre (`q`) em `descricao` OU `sku`
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, sku, descricao, categoria, marca, preco_tabela, custo_unitario, data_lancamento, ativo, created_at, updated_at` (default: `id`); `order_dir` aceita `asc`|`desc` case-insensitive (default: `asc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/produtos
- **Auth:** Bearer Token (admin)
- **Body:** `{ "sku", "descricao", "categoria", "marca", "nota_olfativa", "preco_tabela", "custo_unitario", "unidade", "data_lancamento" (opcional, "AAAA-MM-DD") }`
- **Descrição:** Cria um novo produto. `ativo` é sempre `true` na criação — não é aceito no body. Campos obrigatórios: `sku`, `descricao`, `categoria`, `marca`, `unidade`; `preco_tabela`/`custo_unitario` devem ser `>= 0`. Retorna `201` com o produto criado; `400` em caso de validação.

#### GET /api/produtos/{id}
- **Auth:** Bearer Token (admin)
- **Descrição:** Retorna o detalhe de um produto pelo `id` interno

#### PUT /api/produtos/{id}
- **Auth:** Bearer Token (admin)
- **Body:** `{ "descricao", "categoria", "marca", "nota_olfativa", "preco_tabela", "custo_unitario", "unidade", "data_lancamento" (opcional, "AAAA-MM-DD") }`
- **Descrição:** Atualiza os dados de um produto existente. `sku` e `ativo` **não** são editáveis por esta rota (use `PATCH /api/produtos/{id}/inativar` para alterar `ativo`). Retorna `200` com o produto atualizado, `404` se não existir, `400` se o payload for inválido.

#### PATCH /api/produtos/{id}/inativar
- **Auth:** Bearer Token (admin)
- **Body (opcional):** `{ "ativo": true|false }` — omitido = toggle
- **Descrição:** Ativa ou inativa o produto (exclusão lógica)

### Pedidos (`/api/pedidos/*`) — admin only

> Tela master-detail: lista de pedidos + sub-lista de itens (produtos) de cada pedido. Base importada de `dados/erp/pedidos.csv` (28.729 linhas) e `dados/erp/itens_pedido.csv` (197.724 linhas) para as tabelas `pedidos`/`itens_pedido` (ver seção "Importação de pedidos (ERP)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Pedidos"** da collection. Não existe endpoint de `DELETE` nem de exclusão lógica para pedidos.

#### GET /api/pedidos
- **Auth:** Bearer Token (admin)
- **Query (todos opcionais):** `?page=1&limit=20&status=Faturado&canal=App&cliente_id=1&vendedor_id=1&data_inicio=2026-01-01&data_fim=2026-12-31&q=perfumaria&order_by=data_pedido&order_dir=desc`
- **Descrição:** Lista pedidos paginada (total + pages), com filtros exatos por `status` (`Cancelado`|`Em separação`|`Entregue`|`Faturado`), `canal` (`App`|`Telefone`|`Visita`|`WhatsApp`), `cliente_id`, `vendedor_id`, intervalo `data_inicio`/`data_fim` (`AAAA-MM-DD`) e busca livre (`q`) pela razão social do cliente. Cada item traz o cabeçalho do pedido enriquecido com `cliente_nome`/`vendedor_nome`, sem os itens.
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, data_pedido, canal, status, valor_total, created_at, updated_at, cliente_nome, vendedor_nome` (default: `id`; `id` é um alias de coluna aceito pela API, mapeado para `pedido_id_origem` — não existe mais campo `id` na resposta); `order_dir` aceita `asc`|`desc` case-insensitive (default: `desc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/pedidos
- **Auth:** Bearer Token (admin)
- **Body:** `{ "cliente_id", "vendedor_id", "data_pedido" ("AAAA-MM-DD"), "canal", "status", "itens": [{ "produto_id", "quantidade", "preco_praticado", "desconto_pct" }] }`
- **Descrição:** Cria um novo pedido com seus itens. `valor_bruto` de cada item e `valor_total` do pedido são calculados no backend (não aceitos no body). `pedido_id_origem` é a PK `BIGINT AUTO_INCREMENT` gerada nativamente pelo MySQL. Exige ao menos um item. Retorna `201` com o pedido criado (incluindo itens); `400` em caso de validação. Não existe mais campo `id` — `pedido_id_origem`/`item_id_origem` são os identificadores.

#### GET /api/pedidos/{id}
- **Auth:** Bearer Token (admin)
- **Descrição:** Retorna o detalhe de um pedido pelo `pedido_id_origem` (PK; o path param continua se chamando `id` na rota), incluindo a lista de itens (`itens`, cada um enriquecido com `produto_sku`/`produto_descricao` via JOIN) — usado na tela master-detail.

#### PUT /api/pedidos/{id}
- **Auth:** Bearer Token (admin)
- **Body:** mesmo formato do `POST /api/pedidos`
- **Descrição:** Atualiza os dados de um pedido existente e substitui integralmente a lista de itens (delete + insert). `valor_bruto`/`valor_total` são recalculados no backend. Retorna `200` com o pedido atualizado (incluindo itens), `404` se não existir, `400` se o payload for inválido.

### Pagamentos (`/api/pagamentos/*`) — **acesso comum (não é admin only)**

> **Diferente de Clientes/Produtos/Pedidos (todos admin only), Pagamentos é liberado a qualquer usuário autenticado — `admin` ou `normal`.** A cadeia de middleware usada é `middleware.JWTMiddleware(cfg, true, false)` (`requireAuth=true`, `requireAdmin=false`), enquanto as demais telas usam `(cfg, true, true)`. Base importada de `dados/erp/pagamentos.csv` (~27,7 mil linhas) para a tabela `pagamentos` (ver seção "Importação de pagamentos (ERP)" abaixo). Requests desses endpoints estão agrupadas na pasta **"Pagamentos"** da collection e usam `{{vendedor_token}}` (usuário `normal`) nos exemplos, em vez de `{{admin_token}}`, para deixar explícito que o acesso é comum. Não existe endpoint de `DELETE`.
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
- **Descrição:** Cria um novo pagamento. `pedido_id` deve existir em `pedidos`. `valor_liquido` é sempre exigido explicitamente no payload — não é calculado automaticamente a partir de `valor`/`taxa_pct` (o CSV de origem já traz o valor líquido calculado, às vezes com pequenas diferenças de arredondamento em relação ao cálculo direto). `pagamento_id` é gerado automaticamente (AUTO_INCREMENT). Retorna `201` com o pagamento criado; `400` em caso de validação (inclusive `pedido_id` inexistente).

#### GET /api/pagamentos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Descrição:** Retorna o detalhe de um pagamento pelo `pagamento_id`.

#### PUT /api/pagamentos/{id}
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Body:** mesmo formato do `POST /api/pagamentos`, exceto `pedido_id`
- **Descrição:** Atualiza os campos editáveis de um pagamento existente. `pagamento_id` e `pedido_id` **não** são editáveis por esta rota (o vínculo com o pedido de origem é definitivo — para reatribuir a outro pedido, o fluxo correto é excluir/recriar). Retorna `200` com o pagamento atualizado, `404` se não existir, `400` se o payload for inválido.

### Oportunidades (`/api/oportunidades/*`) — admin only, + `GET /api/vendedores/{id}/clientes` (acesso comum)

> Funil de vendas (CRM) importado de `dados/crm/oportunidades.csv` (colunas: `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda`). Tela com dois dropdowns em cascata (Vendedor → Cliente) e filtros de coluna na listagem. Requests desses endpoints estão agrupadas na pasta **"Oportunidades"** da collection. Não existe endpoint de `DELETE` nem exclusão lógica.
>
> **Decisão de design:** `vendedor_id` é um campo próprio da oportunidade — **não depende** de a oportunidade ter um vínculo de carteira ativo entre aquele cliente e aquele vendedor (uma oportunidade pode existir mesmo que o cliente esteja hoje na carteira de outro vendedor, ou sem vínculo ativo algum). O endpoint `GET /api/vendedores/{id}/clientes` (usado só para alimentar o segundo dropdown do formulário) é **acesso comum**, diferente do CRUD de Oportunidades (`admin only`) — mesma cadeia de middleware (`cfg, true, false`) usada em Pagamentos.

#### GET /api/oportunidades
- **Auth:** Bearer Token (admin)
- **Query (todos opcionais):** `?page=1&limit=20&cliente_id=1&vendedor_id=1&etapa=Proposta enviada&origem=Indicação&data_abertura_de=2026-01-01&data_abertura_ate=2026-12-31&q=indica&order_by=data_abertura&order_dir=desc`
- **Descrição:** Lista oportunidades paginada (total + pages), com filtros exatos por `cliente_id`, `vendedor_id`, `etapa`, `origem`, intervalo `data_abertura_de`/`data_abertura_ate` (`AAAA-MM-DD`) e busca livre (`q`) em `origem` OU `etapa`.
- **Ordenação (`order_by`/`order_dir`, opcionais):** `order_by` aceita `id, data_abertura, valor_estimado, probabilidade_pct, etapa, origem, created_at, updated_at` (default: `id`); `order_dir` aceita `asc`|`desc` case-insensitive (default: `asc`). Valor inválido/ausente cai silenciosamente no default (sem erro 400).

#### POST /api/oportunidades
- **Auth:** Bearer Token (admin)
- **Body:** `{ "cliente_id", "vendedor_id", "origem", "data_abertura" (opcional, "AAAA-MM-DD", default hoje), "etapa", "probabilidade_pct", "valor_estimado", "data_fechamento" (opcional, "AAAA-MM-DD"), "ciclo_dias" (opcional), "motivo_perda" (obrigatório se etapa = "Fechado perdido") }`
- **Descrição:** Cria uma nova oportunidade. `cliente_id` deve existir em `clientes`; `vendedor_id` deve existir em `vendedores` (campo independente de vínculo de carteira). `probabilidade_pct` deve estar entre 0 e 100; `valor_estimado` deve ser `>= 0`. `oportunidade_id` é gerado automaticamente (AUTO_INCREMENT). Retorna `201` com a oportunidade criada; `400` em caso de validação.

#### GET /api/oportunidades/{id}
- **Auth:** Bearer Token (admin)
- **Descrição:** Retorna o detalhe de uma oportunidade pelo `oportunidade_id`.

#### PUT /api/oportunidades/{id}
- **Auth:** Bearer Token (admin)
- **Body:** mesmo formato do `POST /api/oportunidades` (`data_abertura` obrigatório na edição)
- **Descrição:** Atualiza os dados de uma oportunidade existente. Retorna `200` com a oportunidade atualizada, `404` se não existir, `400` se o payload for inválido.

#### GET /api/vendedores/{id}/clientes
- **Auth:** Bearer Token (qualquer usuário autenticado — admin ou normal)
- **Descrição:** Lista os clientes vinculados (carteira ativa, `data_fim IS NULL`) a um vendedor. Usado pelo dropdown em cascata da tela de Oportunidades (ao escolher o vendedor, filtra os clientes possíveis no segundo dropdown). `404` se o vendedor não existir.

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
- `Cliente criado com dados corretos` (`cliente_id_origem`, `razao_social`, `ativo === true`)

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

### Listar Clientes do Vendedor
- `Status 200`
- `Usuário NORMAL consegue acessar (não é 403)` — **valida explicitamente que o acesso é comum, não admin only**
- `Lista de clientes retornada`

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
| Listar Produtos | 2 | — |
| Criar Produto | 2 | — |
| Detalhe do Produto | 1 | — |
| Editar Produto | 2 | — |
| Ativar/Inativar Produto | 2 | — |
| Listar Pedidos | 2 | — |
| Criar Pedido | 2 | — |
| Detalhe do Pedido | 2 | — |
| Editar Pedido | 1 | — |
| Listar Pagamentos | 4 | — |
| Criar Pagamento | 3 | — |
| Detalhe do Pagamento | 1 | — |
| Editar Pagamento | 3 | — |
| Listar Oportunidades | 2 | — |
| Detalhe da Oportunidade | 1 | — |
| Criar Oportunidade | 1 | — |
| Editar Oportunidade | 1 | — |
| Listar Clientes do Vendedor | 2 | — |

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

Depois, em outro terminal:

```bash
# Iniciar o frontend
make dev-frontend
```

A API ficará disponível em `http://localhost:8080` e o frontend em `http://localhost:3000`.