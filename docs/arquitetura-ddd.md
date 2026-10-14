# Arquitetura DDD da API (blueprint ARQ-00)

> **Origem:** 🟡 BackBrain, card ARQ-00, Lote 21 (2026-10-13), branch `refactor/ddd`.
> **Escopo:** `apis/rotaperfumes-api` (handlers, services, middleware, routes, cmd) e `apis/shared` (repositories, models, services, config, db, cnpj, tz, vlog, importers, cmd, tools, cmdutil).
> **Referências obrigatórias:**
> - invariantes de segurança do 🟣 SecBrain: `docs/arq-00-invariantes-seguranca.md` (IDs R-, F-, C-, I-, E-, V-, D-, M-);
> - matriz de rotas do 🔴 TestBrain: `apis/rotaperfumes-api/tests/routes/arq00_matriz_rotas_test.go` (60 padrões: 4 Public, 35 Authenticated, 21 AdminOnly; corrigido no ARQ-01, N-6);
> - linha de base de testes e cobertura (2026-10-13): API **720** testes de topo, cobertura **91,5%**; shared **593** testes de topo, cobertura **85,6%** (ver §7.1).
>
> Este documento é **só desenho**: nenhum código foi movido no ARQ-00.
>
> **Status (2026-10-13):** revisado pelo 🟣 SecBrain, **aprovado com ressalvas**. Os obrigatórios O1 a O9 e os recomendados R-a a R-g já estão incorporados (ver §0.2). Decisões P1 a P4 tomadas (ver §0.1).

---

## 0. Resumo das decisões

| # | Decisão | Onde está detalhado |
|---|---|---|
| D1 | **Dois módulos continuam** (`shared` e `rotaperfumes-api`, com `replace ../shared`). Dockerfile, Makefile e Jenkinsfile não mudam. | §2.3 |
| D2 | **Domínio, aplicação e infraestrutura de cada contexto vão para `apis/shared/<contexto>/`**, em três pacotes: `<ctx>` (domínio), `<ctx>app` (casos de uso + portas) e `<ctx>mysql` (adapters). Tudo **público**, nada em `shared/internal`, porque a API e os CLIs de `shared` consomem os mesmos casos de uso. | §2.1, §2.2 |
| D3 | **A camada HTTP fica no módulo da API, em `internal/`**: `internal/httpx` (kit transversal), `internal/<ctx>http` (handlers por contexto) e `internal/bootstrap` (composition root). | §2.1 |
| D4 | **Shared kernel `apis/shared/kernel`**: `Principal`, `Role`, `Escopo` (tipo soma sem zero value útil), `Pagina`/`Resultado[T]`, `Ordenacao[C]`, `Data`, `Email`, erros de domínio tipados (`Kind`). | §3.1 |
| D5 | Os pacotes técnicos que já estão bem (`config`, `db`, `vlog`, `tz`, `cnpj`, `cmdutil`) **ficam onde estão**. Movê-los só geraria churn de imports sem ganho de design. | §2.1 |
| D6 | **Strangler fig por contexto.** Em cada lote, as rotas do contexto passam para os handlers novos, o código legado do contexto é apagado e os testes do contexto são portados **no mesmo lote** (R1). Os pacotes legados encolhem a cada lote e somem no ARQ-07. | §7 |
| D7 | **Router com política tipada obrigatória** (`Public`, `Authenticated`, `AdminOnly`) e **método HTTP obrigatório**. Esta é a **única mudança de contrato** do Lote 21 (exceção X1). | §5.9, §7.3 |
| D8 | **vlog em quatro níveis** (rota, caso de uso, decisão, query), no lugar do "linha a linha". Cai de ~3.700 para ~600 chamadas. **Aprovado pelo usuário (P1).** | §6 |
| D9 | **Unit of Work genérico** (`db.Transactor[R]`) no lugar de `BeginTx` espalhado por services e repositories. | §5.2 |
| D10 | **Mapper de erros por `Kind`** com mensagem fixa no sentinela. Acabam os ~12 `xxxErroParaStatus` e os `if errors.Is(...) {writeJSON(404)}` inline. | §5.17 |

### 0.1 Decisões tomadas (2026-10-13)

| # | Tema | Decisão | Quem decidiu |
|---|---|---|---|
| P1 | Densidade do vlog (LOG-02) | **Aprovado:** vlog em 4 níveis (§6), com as proibições do O7. O `docs/logs-verbose.md` é atualizado no ARQ-08. | usuário |
| P2 | Exceção X1 | **Aprovada:** método fixo nas 13 rotas sem método (método errado → 405); `GET /api/senha-historico/{user_id...}`: caminho vazio continua 400 "usuario_id é obrigatório na URL" e `/2/extra` passa de 200 para 400 "usuario_id inválido" (§7.3). | usuário |
| P3 | `Dinheiro` | **`float64` encapsulado no VO** no ARQ-05, com comportamento idêntico (mesma fórmula de `pedido_service.go:261`). A troca para centavos fica no card **ARQ-09**. | 🤍 MegaBrain |
| P4 | `VincularCliente` sem transação (`vendedor_service.go:427-529`) | **Card BUG-14.** O ARQ-04 **preserva** o comportamento atual (mesma sequência de chamadas, sem `Transactor`). | 🤍 MegaBrain |
| R-a | Role desconhecido no banco | **Preservar** o comportamento atual: tudo que não é `admin` é tratado como restrito (como `role != "admin"` hoje), com log operacional `[auth] role desconhecido user_id=<id>`. Nenhum 401/500 novo. | 🤍 MegaBrain |

Cards novos derivados deste blueprint (o 🔵 SubBrain cria): **ARQ-09** (Dinheiro em centavos), **BUG-14** (VincularCliente atômico) e o card da **troca de senha não transacional** (O8-iii, §5.2).

### 0.2 Revisão do 🟣 SecBrain (aplicada)

| Item | Resumo | Onde |
|---|---|---|
| O1 | Tradução Escopo→SQL por `switch` exaustivo via `Escopo.Filtro()`; SemAcesso/inválido nunca vira consulta global | §3.2, §5.5, §8 ARQ-04/05/06 |
| O2 | `Router.Handle` e `Autenticador.Exigir` com `panic` para política/método/padrão inválidos | §5.9, §8 ARQ-01 |
| O3 | Invariante: o `httpx.CORS` responde 204 a **todo** OPTIONS antes do mux | §5.8, §7.3, §8 ARQ-01 |
| O4 | Sem "ponte" exportada: o middleware legado é o único a gravar as chaves antigas; o Principal é derivado internamente | §7.5, §8 ARQ-01 |
| O5 | Allowlist de chamadores de `NovoPrincipal` e `Escopo*`; Principal com zero value inválido; sem "Principal de sistema" | §3.2, §4.3, §8 |
| O6 | Mapper por `switch` com `default` 500; `KindBloqueado` alinhado; AST de `NovoErro`; wrap sem valores | §3.2, §5.17, §9 |
| O7 | Proibições de conteúdo por nível de vlog + teste de AST | §6.2, §8 |
| O8 | I-09 no checklist ARQ-04; regra do `WithTx`; troca de senha fora do UoW; fronteira do I-07 mantida | §5.2, §8 |
| O9 | Ordem efetiva `auth(Trace(handler))` | §5.8, §5.9, §8 ARQ-01 |
| R-a..R-g | Role desconhecido (preservar), HEAD em `/me`, `errors.Join`, isolamento da tx, tag `[escopo] invalido`, parse estrito de `{user_id...}`, allowlists cobrindo os dois módulos | §0.1, §4.3, §5.2, §5.17, §7.3, §7.4, §8 |

---

## 1. Diagnóstico atual

Caminhos relativos a `apis/`. `API` = `rotaperfumes-api`, `SH` = `shared`.

### 1.1 Acoplamento e injeção de dependência

| # | Problema | Evidência | Efeito |
|---|---|---|---|
| A1 | **`*sql.DB` passado por parâmetro em todo método** de handler, service e repository; repositórios são `struct{}` vazias. | `API/services/produto_service.go:61` `ListProdutos(ctx, db *sql.DB, …)`; `API/handlers/produto_handler.go:59` `h.svc.ListProdutos(r.Context(), h.db, …)`; `SH/repositories/produto_repository.go:17-22` `type ProdutoRepository struct{}` | Toda assinatura carrega infraestrutura. O service não esconde o banco, e o handler precisa conhecer o pool. Não dá para trocar o repositório por um fake. |
| A2 | **Construtores que ignoram dependências.** | `API/services/produto_service.go:52` `NewProdutoService(db *sql.DB, cfg)` não usa `db`; o mesmo padrão se repete nos 12 services | API enganosa: parece injetar, mas não injeta. |
| A3 | **Handlers constroem os próprios services e repositórios**, e o `main` só repassa `(conn, cfg)`. | `API/cmd/server/main.go:81-109`; `API/handlers/auth_handler.go:77-100` (cria 3 rate limiters, Turnstile, repos e services dentro do handler) | Não existe composition root. Os singletons (rate limiter, notifier) ficam escondidos dentro de handlers. Os testes precisam de setters. |
| A4 | **Injeção por setter / acoplamento temporal.** | `auth_handler.go:71` `SetAlertaSeguranca`, `:942` `SetCaptchaVerifier`; `refresh_token_service.go:120,133` `SetReuseSuppressWindow`/`SetRetencao`; `alerta_seguranca_notifier.go:77-89` `SetNow`/`SetRun`/`SetUsuarioLookup` | Um objeto pode existir "meio configurado" (ex.: sem notifier = só log). |
| A5 | **A camada HTTP fala direto com o repositório**, pulando o service. | `API/handlers/scope.go:83,127,166` (`repositories.NewUsuarioRepository()…` dentro de handlers); `cliente_handler.go:167` `h.carteiraRepo.GetVinculoAtivo`; `dashboard_handler.go:253` `h.vendedorRepo.IsDesligado` | A regra de escopo (segurança) fica no handler, longe do caso de uso. |
| A6 | **Dois pacotes chamados `services`** (`SH/services`: auth, hash, email, turnstile; `API/services`: casos de uso), importados com alias `sharedsvc`. | `API/cmd/server/main.go:18`; `auth_handler.go` | A mesma palavra designa camadas diferentes. |
| A7 | **`models` anêmicos com tags JSON** são usados como entidade, DTO de resposta e linha de banco ao mesmo tempo. | `SH/models/usuario.go:9-26` (`PasswordHash json:"-"`); `SH/models/pedido.go` | O contrato HTTP fica acoplado ao schema. `json:"-"` vira a única barreira contra vazamento (I-12). |
| A8 | **`Execer` cobre só `ExecContext`**; parte dos métodos aceita `Execer` e parte `*sql.DB`. | `SH/repositories/execer.go:9-11`; `usuario_repository.go:112` (Execer) vs `:275` (`*sql.DB`) | Uma leitura dentro de uma transação é impossível sem mudar a assinatura. |

### 1.2 Duplicação

| # | Problema | Evidência |
|---|---|---|
| B1 | **Filtro declarado três vezes** (query → `services.XFiltro` → `repositories.XFiltro`), com cópia campo a campo. | `produto_handler.go:49-56` → `produto_service.go:36-43` → `:67-74` → `produto_repository.go:32-39`; o mesmo em cliente, pedido, pagamento, oportunidade, visita e estoque |
| B2 | **Paginação normalizada em duas camadas** e cálculo de `pages` copiado em 10 handlers. | `usuario_service.go:403-424` `ParsePagination` (helper HTTP morando em service de usuário); `produto_repository.go:111-125` repete o clamp; `pages := total / limit` 2x em cada um de 10 handlers (`produto_handler.go:67-73`, …) |
| B3 | **Mapeamento erro→HTTP por contexto** (`produtoErroParaStatus`, `pedidoErroParaStatus`, …), mais `if errors.Is(err, ErrXNaoEncontrado) { 404 }` inline. | `produto_handler.go:186-210`, `:96-99`, `:139-142`; `pedido_handler.go:221`; `pagamento_handler.go:219`; `vendedor_handler.go:186` |
| B4 | **Duas variantes de escopo** com regras parecidas. | `handlers/scope.go:26-46` `vendedorScope`; `dashboard_handler.go:222-277` `dashboardScope` (copia a consulta de desligado) |
| B5 | **Validação devolvendo 6 a 8 valores**, em vez de construir a entidade. | `cliente_service.go:149` `validarClienteInput(...) (razaoSocial, cnpj, segmento, cidade, uf, bairro string, dataCadastro time.Time, err error)`; `vendedor_service.go:172` |
| B6 | **Parse de data `2006-01-02` repetido** com erros diferentes. | `produto_service.go:33`, `vendedor_service.go:34`, `cliente_service.go`, `pedido_service.go` |
| B7 | **Wrapper de CNPJ duplicando o pacote `cnpj`.** | `API/services/cnpj.go:1-61` sobre `SH/cnpj/cnpj.go` |
| B8 | **Montagem de cadeia repetida 60 vezes** em `routes.go`: duas linhas de vlog + `jwt(bool,bool)(http.HandlerFunc(h.X))` + `mux.Handle`. | `API/routes/routes.go:96-455`; `NewMux` com 14 parâmetros (`:80`) |
| B9 | **Duas formas de log verbose**: `vlog.Printf` e `if s.Cfg.Verbose { log.Printf }`. | `produto_service.go:62,124`; 53 ocorrências de `Cfg.Verbose` em `API/services`; `dashboard_handler.go:270` |

### 1.3 Clean Code

| # | Problema | Evidência |
|---|---|---|
| C1 | **vlog antes de cada statement**: 3.726 chamadas, a maioria descrevendo sintaxe ("declarando variável req", "verificando se err != nil"). Quase metade das linhas de produção é log. | `produto_handler.go:42-75` (34 linhas, 13 de vlog); `produto_repository.go:66-106`; `main.go:33-153` |
| C2 | **Funções longas.** | `auth_handler.go:193-356` `Login` (~160 linhas), `:357-526` `Refresh` (~170), `:621-803` `ResetPassword` (~180); `pedido_handler.go:351-467` `UpdatePedido`; `vendedor_service.go:427-529` `VincularCliente` |
| C3 | **God object**: `AuthHandler`, com 1.005 linhas e 13 campos, faz HTTP, orquestra casos de uso, aplica rate limit, captcha, rotação, detecção de reuso e alerta. | `auth_handler.go:46-61` |
| C4 | **Booleanos posicionais** decidem a segurança da rota: `jwt(true, false)`. Um `false, false` por engano deixa a rota pública. | `routes.go:89-91`; `auth_middleware.go:37,62` |
| C5 | **Strings mágicas de papel**: `"admin"` literal. | `auth_middleware.go:152` `role != "admin"`; `scope.go:69` |
| C6 | **Sentinela numérica de escopo**: `vendedorID <= 0` significa "sem filtro" (global). | `SH/repositories/cliente_repository.go:41`, `oportunidade_repository.go:76`, `dashboard_repository.go:41-50` (E-02) |
| C7 | **Parâmetro anônimo** em método público. | `usuario_service.go:249` `CreateUsuario(ctx, db, input struct{…})` |
| C8 | **Resposta como `map[string]any`** (envelope, dashboard, usuário). | `handlers/response.go:28-66`; `dashboard_service.go:34,96,131`; `usuario_handler.go:144` `usuarioToMap` |
| C9 | **Repositório abrindo transação** (decisão de negócio na infraestrutura). | `pedido_repository.go:305` (`CreateComItens`), `:364` (`UpdateComItens`) |
| C10 | **Variável global para teste**. | `auth_middleware.go:236` `var timeNow = …` |
| C11 | **Código só de teste no pacote de produção**. | `auth_middleware.go:37` `JWTMiddleware` (sem checker), `:254` `RequireAdmin()` |
| C12 | **`writeError` concatena JSON à mão.** | `auth_middleware.go:280-285` (F-05) |
| C13 | **Erro de domínio declarado em arquivo de outro agregado.** | `vendedor_service.go:20-22` reaproveita `ErrVendedorNaoEncontrado` de `usuario_service.go` |
| C14 | **Rotas sem método** (13 padrões) e um padrão prefixo (`/api/senha-historico/`) aceitam qualquer método e qualquer subcaminho. | `routes.go:98-122,160,165,172-186,249,459` |

---

## 2. Estrutura-alvo

### 2.1 Árvore

```
apis/
├── Dockerfile                         (inalterado)
├── shared/                            módulo github.com/rotaperfumes/shared
│   ├── kernel/                        SHARED KERNEL (puro: sem sql, net/http, vlog)
│   │   ├── principal.go               Principal, Role
│   │   ├── escopo.go                  Escopo (tipo soma)
│   │   ├── pagina.go                  Pagina, Resultado[T]
│   │   ├── ordenacao.go               Direcao, Ordenacao[C]
│   │   ├── data.go                    Data (AAAA-MM-DD)
│   │   ├── email.go                   Email
│   │   ├── dinheiro.go                Dinheiro (ver P3)
│   │   └── erros.go                   Kind, Erro, sentinelas genéricas
│   │
│   ├── catalogo/                      DOMÍNIO   package catalogo
│   │   ├── produto.go  estoque.go  erros.go  ordenacao.go
│   │   ├── catalogoapp/               APLICAÇÃO package catalogoapp (casos de uso + portas)
│   │   └── catalogomysql/             INFRA     package catalogomysql (implementa as portas)
│   ├── identidade/                    Usuario, Senha, PoliticaSenha, RefreshToken, HistoricoSenha
│   │   ├── identidadeapp/             Login, Refresh, Logout, TrocarSenha, AdminReset, CRUD usuário, cleanup
│   │   ├── identidademysql/
│   │   └── identidadecrypto/          Argon2id/bcrypt (Strategy), JWT HS256, gerador de token/senha
│   ├── comercial/                     Vendedor, Carteira, Cliente, CNPJ, Visita, Oportunidade
│   │   ├── comercialapp/              casos de uso + AutorizadorEscopo
│   │   └── comercialmysql/
│   ├── vendas/                        Pedido (aggregate root) + ItemPedido, Pagamento
│   │   ├── vendasapp/
│   │   └── vendasmysql/
│   ├── analytics/                     Periodo, JanelaDias (read side)
│   │   ├── analyticsapp/              queries (CQRS leve)
│   │   └── analyticsmysql/
│   ├── notificacao/                   AlertaReuso, Mensagem, ModoTLS
│   │   ├── notificacaoapp/            AlertaSegurancaNotifier (dedup, teto, concorrência)
│   │   └── notificacaosmtp/           SMTP (Adapter + Strategy TLS) e Noop (Null Object)
│   ├── adapters/                      adapters técnicos sem contexto próprio
│   │   ├── turnstile/                 CaptchaVerifier (Adapter)
│   │   └── ratelimit/                 limitador em memória
│   │
│   ├── config/  db/  vlog/  tz/  cnpj/  cmdutil/     (FICAM; db ganha Querier/Transactor)
│   ├── importers/                     ficam; ganham importers/importkit (Template Method), ARQ-07
│   ├── tools/  cmd/                   ficam; passam a usar <ctx>app quando houver regra (ARQ-07)
│   ├── tests/                         ficam (+ tests/arquitetura)
│   └── (legado, removido no ARQ-07)   models/ repositories/ services/
│
└── rotaperfumes-api/                  módulo github.com/rotaperfumes/rotaperfumes-api
    ├── cmd/server/main.go             ~30 linhas: carrega env/config e chama bootstrap.Run
    ├── cmd/resetsenhas/main.go        usa identidadeapp (ARQ-07)
    ├── internal/
    │   ├── bootstrap/                 COMPOSITION ROOT (Facade): wire.go, server.go, email.go, background.go
    │   ├── httpx/                     kit HTTP transversal
    │   │   ├── router.go              Builder de rotas + Policy (D7)
    │   │   ├── chain.go               Chain of Responsibility (CORS → headers → mux → auth)
    │   │   ├── respond.go             envelope (success/data/error/pagination)
    │   │   ├── errmap.go              Kind → status + mensagem fixa
    │   │   ├── query.go               PaginaDaQuery, OrdenacaoDaQuery, BoolDaQuery, IDDoPath
    │   │   ├── principal.go           chave de contexto não exportada + PrincipalFrom
    │   │   ├── auth.go                AuthN/AuthZ (ex-auth_middleware + user_status), ARQ-03
    │   │   ├── cors.go  secheaders.go  logging.go  clientip.go  cookies.go
    │   ├── catalogohttp/              produto_handler.go, estoque_handler.go, dto.go, routes.go
    │   ├── identidadehttp/            auth_handler.go (nome mantido: TST05), usuario_handler.go, senha_historico_handler.go
    │   ├── comercialhttp/
    │   ├── vendashttp/
    │   └── analyticshttp/
    ├── tests/                         ficam (mesmas pastas; ver §7.2)
    └── (legado, removido no ARQ-07)   handlers/ services/ middleware/ routes/
```

### 2.2 Por que essa forma

1. **Por que a aplicação mora em `shared` e não na API.** Os CLIs de `shared` (`tools/resetpassword`, `tools/seedusers`) e o `cmd/resetsenhas` da API aplicam regras de Identidade (hash, histórico, revogação, M-03). Se os casos de uso morassem na API, os CLIs de `shared` não poderiam importá-los (`shared` não importa a API) e a regra continuaria duplicada em SQL cru. Em `shared`, o caso de uso é um só para a API e para os CLIs.
2. **Por que não `shared/internal/...`.** O Go só deixa importar `x/internal/...` a partir de pacotes com raiz em `x/`. O módulo da API (`rotaperfumes-api/...`) **não poderia** importar `shared/internal/catalogo`. Para continuar com dois módulos, as camadas compartilhadas precisam ser públicas. A disciplina de camadas fica com o teste de arquitetura (§4.2), não com `internal/`.
3. **Por que a camada HTTP vai para `rotaperfumes-api/internal/`.** Ninguém fora da API deve importar handlers. Os testes em `rotaperfumes-api/tests/...` estão sob a raiz do módulo e **podem** importar `rotaperfumes-api/internal/...` (a regra do `internal` permite isso), então a mudança não quebra os testes.
4. **Por que três pacotes por contexto, com nome sufixado (`catalogo`, `catalogoapp`, `catalogomysql`).** Um pacote `domain` em cada contexto obrigaria alias em todo arquivo que importa dois contextos (`catalogodomain "…/catalogo/domain"`). Nomes únicos seguem a convenção da stdlib (`net/http` → `httptest`, `httputil`), dispensam alias e deixam o `grep` óbvio. O pacote de domínio leva o nome do contexto porque é o que mais se lê: `catalogo.Produto`, `catalogo.ErrProdutoNaoEncontrado`.
5. **Por que manter `config`, `db`, `vlog`, `tz`, `cnpj` e `cmdutil`.** São pequenos, coesos, cobertos (config 100%, cnpj 100%) e importados por 37+ testes. `cnpj` vira o algoritmo usado pelo VO `comercial.CNPJ`.
6. **Por que não fundir os módulos.** Seria uma refatoração ortogonal ao DDD: mexeria em Dockerfile (contexto `apis/`), Makefile (`build-shared`), Jenkinsfile (`cd shared && go test`, `cd rotaperfumes-api && go test`) e na contagem de testes "por suíte" (R1). Fica como opção futura, com card próprio.

### 2.3 Impacto em build, Docker e CI

- **Dockerfile** (`apis/Dockerfile`): nada muda. O contexto já é `apis/`, ele já copia `shared/` e `rotaperfumes-api/` e compila `./cmd/server`.
- **`replace github.com/rotaperfumes/shared => ../shared`**: nada muda. Pacotes novos de `shared` entram automaticamente.
- **Makefile**: `build-api`, `dev-api`, `test` e `db-import-*` ficam iguais (`go run ./cmd/import…` continua). Opcional no ARQ-01: alvo `arch-check` (`go test ./tests/arquitetura/...`).
- **Jenkinsfile** (`go vet ./... && go test ./...` nos dois módulos): nada muda. O teste de arquitetura roda junto.
- **go.mod**: nenhuma dependência nova (sem framework de DI nem de logging).

---

## 3. Bounded contexts e shared kernel

### 3.1 Mapa de contextos

```
                 ┌───────────────── kernel (shared kernel) ─────────────────┐
                 │ Principal Role Escopo Pagina Resultado Ordenacao Data   │
                 │ Email Dinheiro Erro/Kind                                  │
                 └──────────────────────────────────────────────────────────┘
   Identidade ──(Principal, StatusUsuario)──► httpx.auth (AuthN/AuthZ)
   Comercial  ──(AutorizadorEscopo: Principal → Escopo)──► Vendas, Analytics   [Customer/Supplier]
   Catálogo   ──(ProdutoExiste/Preço, via porta)──► Vendas                      [Customer/Supplier]
   Identidade ──(evento ReusoDeRefreshDetectado)──► Notificação                [Published event]
   Comercial  ──(DesligarVendedor: inativa usuários + revoga tokens na mesma tx)──► Identidade (porta)
```

Regras entre contextos:
- **Um contexto não importa `<outro>mysql`.** Quem precisa de dado de outro contexto declara uma porta pequena no próprio `<ctx>app`, e o bootstrap liga a porta à implementação do outro contexto.
- **O domínio pode importar `kernel`** e nada mais do projeto.
- Desligar um vendedor (I-09) toca três tabelas de dois contextos **em uma transação**. O caso de uso fica em `comercialapp`, e a escrita em Identidade passa por uma porta `IdentidadeTx` amarrada ao mesmo `*sql.Tx` (UoW, §5.2).

### 3.2 Shared kernel (`apis/shared/kernel`)

| Tipo | Regra | Substitui |
|---|---|---|
| `Role` (`RoleAdmin`, `RoleNormal`) + `RoleDoBanco(s)` | Guarda o valor bruto do banco (o `/me` continua devolvendo a string do banco). `EhAdmin()` só para `"admin"` exato. **R-a:** um valor desconhecido **não** gera erro: é tratado como restrito, igual ao `role != "admin"` de hoje, e o `httpx.Autenticador` loga `[auth] role desconhecido user_id=<id>` (sem 401/500 novo). `Conhecido()` permite o log. | literais `"admin"` (`auth_middleware.go:152`, `scope.go:69`), `models.RoleAdmin` |
| `Principal{userID int64; role Role}` | Campos privados. `NovoPrincipal(id>0, role não vazio)`. **Zero value inválido**: `Valido()` é falso, e todo caso de uso que recebe Principal chama `p.Valido()` e falha fechado (`ErrPrincipalInvalido`, Kind interno → 500). **Só `rotaperfumes-api/internal/httpx` chama `NovoPrincipal`** (O5, verificado por teste, §4.3). Não existe "Principal de sistema": casos de uso de CLI (reset de senhas, seed, resetpassword) **não recebem Principal**. Expõe `UserID()`, `Role()`, `EhAdmin()`, `Valido()`. | `middleware.GetUserID/GetRole` (`auth_middleware.go:239-250`) |
| `Escopo` | Tipo soma `Irrestrito | Vendedor(id>0) | SemAcesso`. **Zero value inválido** (E-02, D-01). Os construtores `Escopo*` **só podem ser chamados** por `shared/comercial/comercialapp` (AutorizadorEscopo), pelo próprio `kernel` e por testes (O5). A tradução para SQL é **só** por `Filtro()` (O1). | `vendedorScope`, `dashboardScope`, `int64` sentinela |
| `Pagina`, `Resultado[T]` | page ≥ 1; limit < 1 vira 20; limit > 100 vira 100 (F-10). `Offset()`, `Paginas()`. | `ParsePagination` (`usuario_service.go:403`), clamps nos repositórios, `pages := …` x10 |
| `Ordenacao[C ~string]` + `Direcao` | Enum por entidade; o valor inválido cai no default (C-02). | `OrderBy/OrderDir string` em filtros |
| `Data` | Layout `2006-01-02`; `ParseData` devolve o erro do campo. | B6 |
| `Email` | Normaliza (trim/lower); valida o formato básico; recusa CR/LF (M-01). `Mascarado()` **reimplementa** o algoritmo de `vlog.MaskEmail` (o kernel não importa vlog, §4.1); a equivalência é coberta por `TestEmail_Mascarado_IgualAoVlog` (ARQ-01). | strings soltas |
| `Dinheiro` | ≥ 0; aritmética centralizada (P3). | `float64` soltos |
| `Kind` + `Erro` | `KindInterno`, `KindInvalido`, `KindNaoEncontrado`, `KindConflito`, `KindProibido`, `KindNaoAutenticado`, `KindBloqueado` (a mesma lista do enum do §5.17). Mensagem fixa e literal. | erros `errors.New` + mapas por handler |

```go
// kernel/escopo.go
package kernel

type tipoEscopo uint8

const (
	escopoInvalido tipoEscopo = iota // zero value: NUNCA utilizável
	escopoIrrestrito
	escopoVendedor
	escopoSemAcesso
)

// Escopo é a restrição de carteira de uma requisição. Não existe "0 = global":
// o zero value é inválido e todo consumidor chama Validar (fail-closed, E-02).
type Escopo struct {
	tipo       tipoEscopo
	vendedorID int64
}

var ErrEscopoInvalido = NovoErro(KindInterno, "erro interno")

// EscopoIrrestrito só nasce de um Principal admin (E-01).
func EscopoIrrestrito(p Principal) (Escopo, error) {
	if !p.EhAdmin() {
		return Escopo{}, ErrEscopoInvalido
	}
	return Escopo{tipo: escopoIrrestrito}, nil
}

func EscopoDoVendedor(id int64) (Escopo, error) {
	if id <= 0 {
		return Escopo{}, ErrEscopoInvalido
	}
	return Escopo{tipo: escopoVendedor, vendedorID: id}, nil
}

func EscopoSemAcesso() Escopo { return Escopo{tipo: escopoSemAcesso} }

func (e Escopo) Validar() error {
	if e.tipo == escopoInvalido || (e.tipo == escopoVendedor && e.vendedorID <= 0) {
		return ErrEscopoInvalido
	}
	return nil
}
func (e Escopo) SemAcesso() bool  { return e.tipo == escopoSemAcesso }
func (e Escopo) Irrestrito() bool { return e.tipo == escopoIrrestrito }

// ErrEscopoSemAcesso sinaliza ao adapter que NÃO deve executar SQL. O caso de
// uso responde vazio (E-03) ou 404 (E-04) antes de chegar aqui; se chegar, o
// adapter devolve vazio/erro sem consultar o banco.
var ErrEscopoSemAcesso = NovoErro(KindInterno, "erro interno")

// Filtro é a ÚNICA forma de traduzir Escopo em SQL (O1). switch exaustivo:
//   - Irrestrito      → filtra=false (sem filtro de vendedor);
//   - Vendedor(id>0)  → filtra=true, id;
//   - SemAcesso       → ErrEscopoSemAcesso (não executar SQL);
//   - inválido/zero   → ErrEscopoInvalido  (não executar SQL).
// O adapter nunca deduz "global" de um booleano falso: só de err == nil && !filtra.
func (e Escopo) Filtro() (vendedorID int64, filtra bool, err error) {
	switch e.tipo {
	case escopoIrrestrito:
		return 0, false, nil
	case escopoVendedor:
		if e.vendedorID <= 0 {
			return 0, false, ErrEscopoInvalido
		}
		return e.vendedorID, true, nil
	case escopoSemAcesso:
		return 0, false, ErrEscopoSemAcesso
	default:
		return 0, false, ErrEscopoInvalido
	}
}

// VendedorID devolve o id só no escopo de vendedor; 0 em qualquer outro caso.
// Serve para log e para forçar o VendedorID em comandos (E-05). NÃO serve para
// decidir filtro SQL (use Filtro).
func (e Escopo) VendedorID() int64 {
	if e.tipo == escopoVendedor {
		return e.vendedorID
	}
	return 0
}

// Permite reporta se um registro do vendedorID pode ser visto (E-04).
func (e Escopo) Permite(vendedorID int64) bool {
	switch e.tipo {
	case escopoIrrestrito:
		return true
	case escopoVendedor:
		return vendedorID == e.vendedorID
	default:
		return false
	}
}
```

```go
// kernel/principal.go
type Principal struct {
	userID int64
	role   Role
}

var ErrPrincipalInvalido = NovoErro(KindInterno, "erro interno")

// NovoPrincipal: chamador permitido SOMENTE rotaperfumes-api/internal/httpx
// (teste de allowlist, §4.3). Recebe userID e role JÁ CONFERIDOS no banco.
func NovoPrincipal(userID int64, role Role) (Principal, error) {
	if userID <= 0 || role.String() == "" {
		return Principal{}, ErrPrincipalInvalido
	}
	return Principal{userID: userID, role: role}, nil
}

func (p Principal) Valido() bool  { return p.userID > 0 && p.role.String() != "" } // zero value: false
func (p Principal) EhAdmin() bool { return p.Valido() && p.role.EhAdmin() }

// Uso obrigatório na 1ª linha de todo caso de uso que recebe Principal:
//	if !p.Valido() { return …, kernel.ErrPrincipalInvalido }
```

### 3.3 Catálogo (ARQ-02, piloto)

| Elemento | Conteúdo | Origem atual |
|---|---|---|
| Entidades | `Produto` (id, SKU, descrição, categoria, marca, nota olfativa, preço tabela, custo, unidade, lançamento, ativo); `RegistroEstoque` | `SH/models/produto.go`, `estoque.go` |
| VOs | `SKU` (obrigatório, imutável no update); `Dinheiro` (≥ 0, preço/custo); `kernel.Data`; `OrdenacaoProduto`, `OrdenacaoEstoque` (2 enums, C-02) | `produto_service.go:19-29`, `produto_repository.go:43-55`, `estoque_repository.go:77,83` |
| Fábricas | `NovoProduto(DadosProduto) (*Produto, error)`; `(*Produto).Atualizar(DadosProduto) error`; `(*Produto).DefinirAtivo(*bool)` (toggle) | `produto_service.go:130-…`, `:96-128` |
| Erros | `ErrProdutoNaoEncontrado` (NaoEncontrado), `ErrSKUObrigatorio`, …, `ErrDataLancamentoInvalida` (Invalido) | `produto_service.go:19-29`, `estoque_service.go` |
| Portas (`catalogoapp`) | `Produtos{Listar(ctx, FiltroProduto, Pagina) (Resultado[Produto], error); PorID; Inserir; Atualizar; DefinirAtivo}`; `Estoques{…}` | `ProdutoRepository`, `EstoqueRepository` |
| Casos de uso | `ListarProdutos`, `ObterProduto`, `CriarProduto`, `AtualizarProduto`, `AlternarAtivoProduto`; `ListarEstoque` (última posição ou histórico), `ObterEstoque`, `AjustarEstoque`, `AtualizarSaldo` | `produto_service.go`, `estoque_service.go` |
| HTTP | `catalogohttp`: leitura de produto Authenticated; escrita de produto e tudo de estoque AdminOnly (C-01) | `produto_handler.go`, `estoque_handler.go`, `routes.go:283-310,435-455` |

### 3.4 Identidade e Acesso (ARQ-03)

| Elemento | Conteúdo | Origem atual |
|---|---|---|
| Aggregates | `Usuario` (id, nome, email, role, ativo, deve trocar senha, id_vendedor, último login, **corte de sessão** `tokensValidosDesde`); `RefreshToken` (hash, expira, revogado em/motivo, reuso detectado) | `models/usuario.go`, `repositories/refresh_token_repository.go:39` |
| VOs | `Senha` (`String()`="***", sem `MarshalJSON`/`json`); `HashSenha` (opaco, sem JSON); `PoliticaSenha` (I-12: 8 runes, 72 bytes, 3 de 4 classes); `MotivoRevogacao` (enum: rotacao, logout, revogacao_massa, senha, inativacao); `TokenOpaco` (32B, `String()`="***") | `SH/services/password_policy.go`, `repositories.RevokeReason` |
| Regras de domínio | `HistoricoSenha.Reutilizada(candidata, atual+2 últimas, verificador) bool` **sem short-circuit** (avalia todas); `RefreshToken.Avaliar(agora, janelaGraca)` devolve `Valido | Expirado | RevogadoRecente | Revogado(motivo)`, checando expiração antes de revogação (I-06); `Usuario.TokenAnteriorAoCorte(iat)` (I-04) | `auth_handler.go:717`, `refresh_token_service.go:192-236`, `auth_middleware.go:214-224` |
| Erros | `ErrCredenciaisInvalidas`, `ErrUsuarioInativo` (mesma msg de inexistente), `ErrSessaoEncerrada`, `ErrSenhaFraca` (texto fixo), `ErrSenhaReutilizada`, `ErrSenhaAtualIncorreta`, `ErrRefreshInvalido/Expirado/Revogado/RevogadoRecente`, `ErrCaptchaInvalido`, `ErrMuitasTentativas` (KindBloqueado; o Retry-After vai em `*kernel.Bloqueio`, §5.17) | `services/refresh_token_service.go:43-56`, `usuario_service.go:34-42` |
| Portas | `Usuarios`, `RefreshTokens`, `HistoricoSenhas`, `VerificadorSenha`/`GeradorHash` (Strategy), `EmissorToken` (JWT), `VerificadorCaptcha`, `Limitador`, `Publicador` (eventos), `Relogio`, `Transactor[TxIdentidade]` | — |
| Casos de uso | `Login`, `RenovarSessao` (refresh com rotação + reuso), `Logout`, `TrocarSenha`, `AdminResetarSenha`, `CriarUsuario`, `AtualizarUsuario`, `AlternarAtivoUsuario`, `ListarUsuarios`, `ObterSessao` (`/me`), `ListarHistoricoSenha`, `StatusParaAutenticacao` (SEC-06), `LimparRefreshExpirados` (CHORE-02), `ResetarSenhasAtivos` (CLI) | `auth_handler.go`, `usuario_service.go`, `refresh_token_service.go`, `senha_historico_service.go`, `refresh_cleanup.go`, `reset_senhas_ativos_service.go` |
| Infra | `identidademysql`; `identidadecrypto` (Argon2id+pepper / bcrypt legado, JWT HS256, gerador de token e de senha); `adapters/turnstile`; `adapters/ratelimit` | `SH/services/password_hash.go`, `auth_service.go`, `password_generator.go`, `turnstile_service.go`, `API/middleware/ratelimit_middleware.go` |
| HTTP | `identidadehttp` (`auth_handler.go` **mantém o nome do arquivo e do tipo `AuthHandler`**, exigência do TST05); `httpx/auth.go` (Bearer/cookie, Principal, política); `httpx/clientip.go`; `httpx/cookies.go` | `auth_middleware.go`, `user_status.go`, `auth_handler.go:126-190,908-940` |

### 3.5 Comercial (ARQ-04)

| Elemento | Conteúdo | Origem atual |
|---|---|---|
| Aggregates | `Vendedor` (desligado = data_desligamento; `Desligar(agora)` idempotente via COALESCE, BUG-04); `Carteira` (vínculo cliente↔vendedor, `Ativo()` = data_fim nula); `Cliente`; `Visita`; `Oportunidade` | `models/vendedor.go`, `carteira.go`, `cliente.go`, `visita.go`, `oportunidade.go` |
| VOs | `CNPJ` (usa `shared/cnpj`; `String()` sem máscara; **nunca logado**, M-04); `UF`; `Segmento`; `EtapaOportunidade`, `ResultadoVisita` (enums); `kernel.Data` | `API/services/cnpj.go`, `cliente_service.go:149` |
| Serviço de domínio | `PoliticaEscrita`: dado um `Escopo` e um `Cliente`, diz se a escrita é permitida (E-05). Função pura. | `scope.go:157-176`, `cliente_handler.go:199` |
| Erros | `ErrClienteNaoEncontrado` (também para "fora da carteira", E-04), `ErrCNPJInvalido`, `ErrCNPJDuplicado` (409 genérico, E-07), `ErrVendedorDesligado` (403 com msg fixa "acesso bloqueado: vendedor desligado"), `ErrSemVendedor` (403), … | `scope.go:50-54`, `cnpj.go`, `vendedor_service.go:23-30` |
| Portas | `Vendedores`, `Carteiras`, `Clientes`, `Visitas`, `Oportunidades`, `VinculoUsuario` (usuarios.id_vendedor, porta para Identidade), `IdentidadeTx` (inativar usuários + revogar tokens na tx do desligamento), `Transactor[TxComercial]` | — |
| Casos de uso | `AutorizadorEscopo.Resolver(ctx, Principal) (Escopo, error)` e `.ResolverParaAnalytics(ctx, Principal) (Escopo, Desligamento, error)` (D-01); CRUD de Cliente (com `CriarClienteNaCarteira` em tx), Vendedor (incluindo `DesligarVendedor` em tx única, I-09), Carteira (vincular/desvincular), Visita, Oportunidade | `scope.go`, `cliente_service.go`, `vendedor_service.go`, `visita_service.go`, `oportunidade_service.go` |
| HTTP | `comercialhttp`; **o update/toggle de cliente chama `AutorizarEscritaCliente` antes de decodificar o body** (E-05) | `cliente_handler.go`, `vendedor_handler.go`, `visita_handler.go`, `oportunidade_handler.go` |

### 3.6 Vendas (ARQ-05)

| Elemento | Conteúdo | Origem atual |
|---|---|---|
| Aggregate root | `Pedido` com `[]ItemPedido`: `NovoPedido(dados, itens)` calcula `ValorBruto` por item e `ValorTotal` (V-03); `SubstituirItens`; `PodeExcluir(temPagamentos) error` (409 com pagamentos ou status Faturado, V-01) | `pedido_service.go:229-263`, `models/pedido.go`, `item_pedido.go` |
| Entidade | `Pagamento`: `PedidoID` **imutável** (não existe setter; o comando de update não tem o campo, V-02); `PodeExcluir()` (409 se Pago ou Pago com atraso) | `pagamento_service.go` |
| VOs | `StatusPedido`, `Canal`, `StatusPagamento`, `FormaPagamento` (enums); `DescontoPct` (0..100); `Dinheiro` | `pedido_service.go:30` |
| Portas | `Pedidos` (`Inserir(ctx, *Pedido)` grava cabeçalho e itens; a transação vem do UoW, não do repositório, C9), `Pagamentos`, `CarteiraConsulta` (cliente na carteira do vendedor, porta para Comercial), `EscopoResolver` (porta para Comercial), `Transactor[TxVendas]` | `pedido_repository.go:305,364` |
| Casos de uso | `ListarPedidos`, `ObterPedido`, `CriarPedido`, `AtualizarPedido`, `ExcluirPedido`; `ListarPagamentos`, `ObterPagamento`, `CriarPagamento`, `AtualizarPagamento`, `ExcluirPagamento` (escopo herdado de `pedido.vendedor_id`; falha ao buscar pedido em Get/Delete vira 404, V-02) | `pedido_service.go`, `pagamento_service.go`, `pagamento_handler.go:260` |

### 3.7 Analytics / Dashboard (ARQ-06)

| Elemento | Conteúdo | Origem atual |
|---|---|---|
| VOs | `Periodo` (enum `Hoje | Semana | Mes`, com parse que cai em `Mes` como hoje), `JanelaDias` (1..365, D-03) | `dashboard_repository.go:26-37`, `dashboard_handler.go:279` |
| Read models | `Metricas`, `SerieVendas`, `RankingVendedor`, `MetricasClientes`: structs tipadas no lugar de `map[string]any`, com as tags JSON idênticas às chaves atuais | `dashboard_service.go:34-248` |
| Queries | `ConsultarMetricas`, `ConsultarSerieVendas`, `ConsultarRanking` (paginada), `ConsultarMetricasClientes`. Recebem `Principal` e resolvem `Escopo` via porta. Desligado, inexistente ou sem vendedor devolvem o resultado zerado com `vendedor_desligado` (D-01). | `dashboard_handler.go:39-277` |
| Infra | `analyticsmysql`: SQL com `fmt.Sprintf` **só de constantes** (cláusula escolhida por `switch` sobre `Periodo`, D-02); o filtro de vendedor vem de `Escopo` via placeholder | `dashboard_repository.go` |

### 3.8 Notificação (ARQ-07)

| Elemento | Conteúdo | Origem atual |
|---|---|---|
| Domínio | `AlertaReuso`, `Destinatario` (Email sem CR/LF), `ModoTLS` (`Implicito | STARTTLS`) | `SH/services/email_service.go:39`, `email_smtp.go:51` |
| Aplicação | `AlertaSegurancaNotifier` (assinante do evento `identidade.ReusoDeRefreshDetectado`; dedup 30 min, teto 20/h, 4 concorrentes, timeout 30 s, I-08); portas `EnviadorSenhaInicial`, `EnviadorAlerta` | `API/services/alerta_seguranca_notifier.go` |
| Infra | `notificacaosmtp.SMTP` (Adapter; Strategy de TLS; AUTH só sobre TLS, M-01); `notificacaosmtp.Noop` (Null Object; loga e-mail mascarado, M-02) | `email_service.go`, `email_smtp.go` |

### 3.9 Importers, tools e CLIs (ARQ-07)

- `importers/importkit`: esqueleto genérico (Template Method, §5.13). Cada importer vira um `importkit.Spec[Row]` com `Parse`/`Upsert` e mantém o SQL de upsert (performance e idempotência). **Não** passam a usar repositórios de agregado: carga em lote não é caso de uso transacional por entidade, e forçar isso seria overengineering.
- Os importers que validam CNPJ usam o VO `comercial.CNPJ` (M-04: CNPJ nunca logado).
- `tools/resetpassword`, `tools/seedusers` e `cmd/resetsenhas` passam a usar `identidadeapp` e `identidadecrypto` (hash, histórico e revogação num lugar só, M-03). `LoadSemCredenciaisDB` continua só no dry-run.
- `tools/testeemail` usa `notificacaosmtp`. **Atenção:** `tools/testeemail/testeemail_test.go` é teste interno ao pacote (8 testes de topo). Se o arquivo for movido para `tests/tools/testeemail`, a contagem migra de pacote e o total do módulo `shared` não pode cair (§7.2).

---

## 4. Regras de dependência

### 4.1 Regras

| Pacote | Pode importar | Não pode importar |
|---|---|---|
| `kernel` | stdlib (sem `database/sql`, `net/http`, `log`) | qualquer pacote do projeto, incluindo `vlog` |
| `<ctx>` (domínio) | stdlib pura, `kernel`, `cnpj`, `tz` | `database/sql`, `net/http`, `encoding/json`*, `config`, `vlog`, `<ctx>app`, `<ctx>mysql`, outros contextos |
| `<ctx>app` | `kernel`, `<ctx>`, `vlog`, `context`, `time` | `database/sql`, `net/http`, `config`**, qualquer `*mysql`, `adapters/*`, `rotaperfumes-api/...` |
| `<ctx>mysql`, `identidadecrypto`, `notificacaosmtp`, `adapters/*` | `kernel`, `<ctx>`, `db`, `vlog`, `config`, drivers | `<ctx>app`*** , `net/http` (exceto o Turnstile), `rotaperfumes-api/...` |
| `rotaperfumes-api/internal/<ctx>http` | `httpx`, `<ctx>app`, `<ctx>`, `kernel`, `vlog` | `<ctx>mysql`, `database/sql`, `repositories`, `config` |
| `rotaperfumes-api/internal/httpx` | `kernel`, `vlog`, `net/http` e o domínio `shared/identidade` (tipos das portas do `Autenticador`, ARQ-03). Desde o ARQ-03 **não** importa `shared/config` nem `rotaperfumes-api/middleware` (é o `middleware` que lê o Principal do `httpx`); `handlers`, `services` e `repositories` continuam proibidos | `<ctx>mysql`, `database/sql` |
| `rotaperfumes-api/internal/bootstrap` | tudo (é a composition root) | — |

\* O domínio não carrega tags JSON; o DTO HTTP é que serializa (§5.16). Exceção: os VOs sensíveis (`Senha`, `TokenOpaco`, `HashSenha`) **implementam** `MarshalJSON` devolvendo erro, como defesa extra (I-12).
\*\* Parâmetros de configuração entram como struct de opções da própria aplicação (ex.: `identidadeapp.Opcoes{TTLAccess, JanelaGraca, Retencao}`), preenchida pelo bootstrap a partir de `config.Config`.
\*\*\* A infraestrutura satisfaz as interfaces de `<ctx>app` **implicitamente** (Go structural typing), sem importá-las. Se um tipo de retorno precisar ser compartilhado, ele mora no domínio.

### 4.2 Verificação automática

**Implementação (ARQ-01):** em vez de `exec go list`, os testes leem os pacotes com `go/build` (`build.ImportDir`: respeita build tags e ignora `_test.go`) e `go/parser`, sem depender do binário `go` no PATH. Cada regra tem também um teste com pacote/fonte sintética provando que detecta a violação. O código abaixo fica como ilustração da regra.

Um teste de arquitetura em cada módulo, criado no ARQ-01 e válido desde o primeiro lote (os pacotes que ainda não existem são simplesmente ignorados):

```go
// apis/shared/tests/arquitetura/dependencias_test.go
package arquitetura_test

import (
	"os/exec"
	"strings"
	"testing"
)

type regra struct {
	pacote    string   // prefixo do pacote analisado
	proibidos []string // prefixos de import proibidos
}

const mod = "github.com/rotaperfumes/shared/"

var regras = []regra{
	{mod + "kernel", []string{"database/sql", "net/http", "log", mod}},
	// domínio de cada contexto: sem infra nem transporte
	{mod + "catalogo", []string{"database/sql", "net/http", "encoding/json", mod + "config", mod + "vlog", mod + "repositories", mod + "models"}},
	// …identidade, comercial, vendas, analytics, notificacao
}

func TestARQ_Dependencias(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{join .Imports ","}}`, "../../...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, linha := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pkg, imps, _ := strings.Cut(linha, "|")
		for _, r := range regras {
			if !dentroDe(pkg, r.pacote) {
				continue
			}
			for _, imp := range strings.Split(imps, ",") {
				for _, p := range r.proibidos {
					if imp != "" && strings.HasPrefix(imp, p) && !dentroDe(imp, r.pacote) {
						t.Errorf("%s não pode importar %s", pkg, imp)
					}
				}
			}
		}
	}
}

// dentroDe trata "catalogo" como o pacote de domínio exato (os subpacotes
// catalogoapp/catalogomysql têm regras próprias).
func dentroDe(pkg, prefixo string) bool { return pkg == prefixo }
```

- Regras adicionais no mesmo arquivo: `*app` não importa `*mysql`, `database/sql` nem `net/http`; nenhum pacote novo importa `models`, `repositories` ou `services` legados (impede regressão ao legado).
- No módulo da API, `tests/arquitetura/dependencias_test.go` verifica que `internal/<ctx>http` não importa `database/sql`, `*mysql` nem `shared/repositories`, e que **só** `internal/bootstrap` importa `*mysql`.
- Os testes usam `go list` (já no PATH do CI, sem dependência nova) e contam como testes de topo novos (a contagem só sobe).

### 4.3 Allowlists de chamadores e testes de AST (O4, O5, O6, O7, R-c, R-g)

Ficam no **módulo da API**, em `rotaperfumes-api/tests/arquitetura/`, porque só de lá dá para varrer **os dois módulos**: `go list -f '{{.Dir}}' ./... github.com/rotaperfumes/shared/...` (R-g). O `shared` é resolvido pelo `replace ../shared`. Cada arquivo `.go` é lido com `go/parser` e cada regra procura chamadas `pkg.Func` (resolvendo o alias do import). Os arquivos `_test.go` ficam fora das allowlists de chamador.

| Teste | Regra | Origem |
|---|---|---|
| `TestARQ_NovoPrincipal_SoHttpx` | `kernel.NovoPrincipal` só pode ser chamado em `rotaperfumes-api/internal/httpx`. O `httpx` **não exporta** setter de Principal nem de chaves de contexto: os testes HTTP obtêm Principal passando pelo `Autenticador` com checker fake. | O5 |
| `TestARQ_Escopo_SoAutorizador` | `kernel.EscopoIrrestrito`, `EscopoDoVendedor` e `EscopoSemAcesso` só podem ser chamados em `shared/comercial/comercialapp` e no próprio `kernel`. | O5 |
| `TestARQ_Filtro_SoAdapters` | Nenhum adapter `*mysql` usa `Escopo.VendedorID()` nem `Escopo.Irrestrito()` para decidir SQL; a tradução é só via `Escopo.Filtro()`. | O1 |
| `TestARQ_ChavesLegadas_SoMiddlewareLegado` | `context.WithValue` com as chaves `keyUserID`/`keyRole` só existe em `middleware/auth_middleware.go`, até ele ser removido no ARQ-07. Se `ComPrincipalLegado` vier a existir (§7.5), o único chamador permitido é `internal/httpx/auth.go`. | O4 |
| `TestARQ_NovoErro_SentinelaLiteral` | Toda chamada a `kernel.NovoErro` (1) está dentro de uma declaração `var` de nível de pacote, (2) tem como 1º argumento uma constante `kernel.Kind*` e (3) tem como 2º argumento um **literal string** (`*ast.BasicLit`): nada de variável, concatenação ou `fmt.Sprintf`. | O6-iii |
| `TestARQ_SemErrorsJoinDeDominio` | `errors.Join` proibido nos pacotes `<ctx>`, `<ctx>app` e `<ctx>mysql`. Juntar erro de infraestrutura com sentinela de domínio faria o mapper responder com a mensagem do sentinela e engolir a falha de infra. A regra é: **ou** o sentinela (decisão de negócio) **ou** `fmt.Errorf("op: %w", errInfra)`. | R-c |
| `TestARQ_VlogSemConteudoSensivel` | Em toda chamada a `vlog.Printf` e `vlog.Trace`: (1) o formato (literal) não contém `%v`, `%+v` nem `%#v`; (2) nenhum argumento é um identificador ou seletor que case com `(?i)senha|password|token|secret|cnpj|email|hash|captcha|chave|key`, a menos que esteja envolvido em `vlog.MaskEmail(...)` ou `.Mascarado()`; (3) nenhum argumento é `r.URL`, `r.Header`, `r.Body`, `cmd`, `req`, `filtro` ou struct de entidade (heurística: tipo não básico via `go/types`, quando disponível; senão, a lista de nomes). | O7 |
| `TestARQ_SprintfSQLSoConstantes` | Em `analyticsmysql`, os argumentos de `fmt.Sprintf` usados para montar SQL são constantes ou resultados de funções do próprio pacote que fazem `switch` sobre enum. | D-02 |

Os testes de allowlist só **somam** à contagem (R1). Pacotes que ainda não existem são ignorados, então as regras valem desde o ARQ-01.

---

## 5. Catálogo de Design Patterns

Cada item diz **onde** aplicar, **o que simplifica** e traz um exemplo curto. A regra geral é usar interfaces pequenas, declaradas pelo consumidor, e construtores explícitos. Nada de framework.

### 5.1 Repository

- **Onde:** uma porta por agregado em `<ctx>app` e a implementação em `<ctx>mysql`, que guarda o `db.Querier` no struct.
- **Simplifica:** some o `db *sql.DB` de todas as assinaturas (A1); o repositório passa a receber `Escopo` em vez de `int64` (E-02) e o enum de ordenação em vez de string (C-02); o clamp de página sai do repositório (B2).

```go
// catalogoapp/portas.go — interface declarada pelo consumidor
type Produtos interface {
	Listar(ctx context.Context, f catalogo.FiltroProduto, p kernel.Pagina) (kernel.Resultado[catalogo.Produto], error)
	PorID(ctx context.Context, id int64) (*catalogo.Produto, error) // ErrProdutoNaoEncontrado
	Inserir(ctx context.Context, p *catalogo.Produto) error
	Atualizar(ctx context.Context, p *catalogo.Produto) error
}

// catalogomysql/produtos.go
type Produtos struct{ q db.Querier }

func NovoProdutos(q db.Querier) *Produtos { return &Produtos{q: q} }
```

**Não aplicar:** em repositório genérico `Repository[T]` com CRUD, porque cada agregado tem consultas próprias e o genérico vira vazamento de SQL. Também não usar decorator de cache (R4).

### 5.2 Unit of Work (Transactor)

- **Onde (só as fronteiras que JÁ são transacionais hoje; nenhuma nova):**
  - rotação do refresh (revogação condicional + insert, I-06; `refresh_token_service.go:282-340`);
  - desligar vendedor (I-09; `vendedor_service.go:343-389`): `data_desligamento` via COALESCE + inativação dos usuários gravando o corte `tokens_validos_desde` + revogação dos refresh tokens, numa tx única; falha em qualquer passo dá rollback total;
  - criar cliente na carteira (E-05; `cliente_service.go:303-349`);
  - pedido com itens (C9; `pedido_repository.go:305,364`, a tx sobe do repositório para a aplicação, mesma fronteira).
- **Fora do UoW (O8-iii, O8-iv, P4):**
  - **troca de senha** (histórico + hash + corte + revogação) **não** é transacional hoje e continua assim no ARQ-03, com a mesma sequência de chamadas. Tornar atômica é o card próprio que o 🔵 SubBrain cria;
  - **detecção de reuso (I-07)** mantém **exatamente** a fronteira atual: `RegistrarReuso` é um UPDATE condicional atômico isolado; se a marcação falhar, o caso de uso **corta assim mesmo** (fail-closed); se o corte ficar incompleto, `DesfazerReuso` é a **compensação** explícita (`auth_handler.go:528-607`). É proibido envolver marcação + corte numa tx em que um rollback desfaça a marcação **sem** cortar;
  - **VincularCliente** continua sem tx no ARQ-04 (card BUG-14).
- **Simplifica:** acaba com os 5 `BeginTx`/`defer Rollback`/`Commit` espalhados (`vendedor_service.go:345`, `cliente_service.go:312`, `refresh_token_service.go:284`, `pedido_repository.go:305,364`) e com `RefreshRotation` (`refresh_token_service.go:270-340`).
- **Regras do `WithTx` (O8-ii, R-d):**
  1. dentro da `fn`, use **só** os repositórios de `R` (ligados à tx). É proibido usar o pool, outro caso de uso ou um repositório capturado de fora da closure. O tipo `R` só contém o que a operação precisa, e a revisão confere que a closure não referencia campos `uc.repo*`;
  2. `BeginTx(ctx, nil)` usa o isolamento padrão do MySQL/InnoDB (**REPEATABLE READ**), o mesmo de hoje. Nenhum lote muda o nível de isolamento;
  3. a tx é **cancelada pelo `ctx`** da requisição: se o cliente desconectar ou o timeout de 15 s estourar, o `database/sql` faz rollback. É o comportamento atual dos `BeginTx(ctx, nil)` existentes;
  4. nada de efeito externo (e-mail, evento, cookie) dentro da `fn`: eles acontecem depois do commit.

```go
// shared/db/tx.go — infraestrutura técnica
type Querier interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// Transactor[R] abre uma tx, entrega a fn os repositórios R já ligados a ela,
// faz commit se fn devolver nil e rollback caso contrário.
type Transactor[R any] struct {
	pool *sql.DB
	bind func(Querier) R
}

func NewTransactor[R any](pool *sql.DB, bind func(Querier) R) *Transactor[R] {
	return &Transactor[R]{pool: pool, bind: bind}
}

func (t *Transactor[R]) WithTx(ctx context.Context, fn func(R) error) error {
	tx, err := t.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op após Commit
	if err := fn(t.bind(tx)); err != nil {
		return err
	}
	return tx.Commit()
}
```

```go
// identidadeapp/renovar_sessao.go — porta declarada pela aplicação
type TxRotacao struct{ Tokens RefreshTokens }
type Transactor[R any] interface{ WithTx(context.Context, func(R) error) error }

err := uc.tx.WithTx(ctx, func(r TxRotacao) error {
	if err := r.Tokens.RevogarSeAtivo(ctx, rt.ID, identidade.MotivoRotacao); err != nil {
		return err // ErrRefreshRevogado: corrida concorrente → 401, sem contar no rate limit
	}
	novo, err = uc.tokens.Novo(usuario.ID(), origem) // 32 bytes crypto/rand
	if err != nil {
		return err
	}
	return r.Tokens.Inserir(ctx, novo)
})
```

A condição continua no `WHERE` (`revoked_at IS NULL`), nunca deduzida por `RowsAffected` (F-07). A ordem atual é preservada: revogação na tx → emissão do JWT → insert do novo refresh → commit. Qualquer erro dá rollback e nenhum token é entregue.

### 5.3 Factory / construtor com validação

- **Onde:** `catalogo.NovoProduto`, `comercial.NovoCliente`, `comercial.NovoVendedor`, `vendas.NovoPedido`, `identidade.NovoUsuario`, `kernel.NovaPagina`, `kernel.EscopoDoVendedor`, e os construtores de caso de uso com **todas** as dependências obrigatórias (fim dos setters, A4).
- **Simplifica:** some o `validarClienteInput` com 8 retornos (B5): a entidade só existe válida.

```go
// comercial/cliente.go
type DadosCliente struct {
	RazaoSocial, CNPJ, Segmento, Cidade, UF, Bairro string
	DataCadastro                                     string // AAAA-MM-DD, opcional
}

func NovoCliente(d DadosCliente, hoje time.Time) (*Cliente, error) {
	cnpj, err := NovoCNPJ(d.CNPJ)
	if err != nil {
		return nil, err // ErrCNPJInvalido
	}
	uf, err := NovaUF(d.UF)
	if err != nil {
		return nil, err
	}
	data, err := kernel.ParseDataOuHoje(d.DataCadastro, hoje, ErrDataCadastroInvalida)
	if err != nil {
		return nil, err
	}
	razao := strings.TrimSpace(d.RazaoSocial)
	if razao == "" {
		return nil, ErrRazaoSocialObrigatoria
	}
	return &Cliente{razaoSocial: razao, cnpj: cnpj, uf: uf, dataCadastro: data, ativo: true /* … */}, nil
}
```

Os construtores de caso de uso fazem `panic` com dependência `nil` **na montagem** (como já acontece com `JWTMiddlewareWithUserCheck`, I-03), nunca em runtime.

### 5.4 Value Object

- **Onde:** `CNPJ`, `Email`, `Senha`, `HashSenha`, `TokenOpaco`, `Dinheiro`, `Data`, `Pagina`, `Ordenacao`, `Escopo`, `Role`, `Periodo`, `JanelaDias`, `UF`, `DescontoPct`, `MotivoRevogacao`, `ModoTLS`.
- **Simplifica:** a validação acontece uma vez, na borda, e o resto do código confia no tipo. Os segredos não vazam por `%v` nem por JSON.

```go
// identidade/senha.go
type Senha struct{ valor string }

func NovaSenha(s string) Senha                 { return Senha{valor: s} }
func (Senha) String() string                   { return "***" }
func (Senha) GoString() string                 { return "***" }
func (Senha) MarshalJSON() ([]byte, error)     { return nil, errSerializarSegredo }
func (s Senha) Bytes() []byte                  { return []byte(s.valor) } // só para o hasher
```

### 5.5 Specification (com as ressalvas do SecBrain)

- **Onde:** filtros de listagem expressos no domínio (`catalogo.FiltroProduto`, `vendas.FiltroPedido`). São **dados** que o repositório traduz para SQL parametrizado.
- **Ressalvas obrigatórias:**
  1. O escopo **não** é uma Specification opcional. É o parâmetro `kernel.Escopo` obrigatório de todo método de listagem e detalhe, traduzido para SQL só por `Escopo.Filtro()` (switch exaustivo; SemAcesso e inválido retornam erro antes do SQL, O1, E-02).
  2. A regra "últimas N senhas" **não** usa Specification, `slices.ContainsFunc` nem `any` com early return: o loop avalia todas as entradas (I-12, tempo constante por quantidade).
  3. O valor de busca vai sempre por placeholder (C-03); a coluna sai de `switch` sobre enum.

```go
// vendas/filtro.go: Specification como dado imutável
type FiltroPedido struct {
	Status      *StatusPedido
	Canal       *Canal
	ClienteID   int64 // 0 = sem filtro (filtro de negócio, NÃO de segurança)
	VendedorID  int64 // ignorado para Escopo restrito: o caso de uso sobrescreve (E-03)
	De, Ate     *kernel.Data
	Busca       string
	Ordenacao   kernel.Ordenacao[OrdemPedido]
}

// vendasmysql/pedidos.go
func (r *Pedidos) Listar(ctx context.Context, e kernel.Escopo, f vendas.FiltroPedido, p kernel.Pagina) (kernel.Resultado[vendas.Pedido], error) {
	// O1: Filtro() é switch exaustivo. SemAcesso e inválido voltam como erro
	// ANTES de qualquer SQL; "sem filtro" só existe quando err == nil && !filtra,
	// o que só acontece para Irrestrito.
	vendedorID, filtra, err := e.Filtro()
	if err != nil {
		return kernel.Resultado[vendas.Pedido]{}, err // nenhuma query executada
	}
	w := novoWhere()
	if filtra {
		w.e("p.vendedor_id = ?", vendedorID)
	}
	// …
}
```

- **Proibido** no adapter: `if id, ok := …; ok { filtra }` ou qualquer forma que transforme "não é vendedor" em "global". Também proibido usar `VendedorID()`/`Irrestrito()` para decidir SQL (`TestARQ_Filtro_SoAdapters`, §4.3).
- **O caso de uso** trata `SemAcesso` antes de chamar o repositório: listagem → `Resultado` vazio com 200 (E-03); detalhe → `ErrXNaoEncontrado` (E-04); escrita → 403 (E-05). O `ErrEscopoSemAcesso` no adapter é a segunda barreira.
- **Teste obrigatório por repositório com escopo** (ARQ-04, ARQ-05, ARQ-06): chamar `Listar`/`PorID`/consultas do dashboard com `kernel.EscopoSemAcesso()` e com `kernel.Escopo{}` e verificar que volta erro (ou vazio) **sem nenhuma expectativa registrada no sqlmock**. `mock.ExpectationsWereMet()` passa, e qualquer query inesperada faz o sqlmock falhar.

**Não aplicar:** em `And/Or/Not` combináveis em árvore. Nenhuma tela precisa disso, e o custo de leitura é alto.

### 5.6 Strategy

| Onde | Estratégias | Simplifica |
|---|---|---|
| Hash de senha (`identidadecrypto`) | `argon2idPepper` (gera + verifica) e `bcryptLegado` (só verifica, sinaliza rehash) | `if ehBcrypt(hash)` em `password_hash.go:79-127` |
| TLS do SMTP (`notificacaosmtp`) | `tlsImplicito` (porta 465) e `startTLS` (aborta se não anunciado) | `ResolverModoTLSSMTP` + ramificações em `email_smtp.go:51-165` |
| Ordenação | enum `OrdemProduto` + `switch` coluna no adapter | mapas `produtoOrderWhitelist` (`produto_repository.go:43`) com string livre |
| Período do dashboard | `Periodo` + `switch` cláusula constante | `periodoWhereClause(string)` (`dashboard_repository.go:26`) |

```go
// identidadecrypto/hasher.go
type verificador interface {
	Reconhece(hash string) bool
	Verificar(hash string, s identidade.Senha) (ok, precisaRehash bool)
}

type Hasher struct {
	atual  *argon2idPepper // única estratégia que gera hash
	todas  []verificador   // ordem fixa: argon2id, bcrypt
	falsoH string          // hash descartável p/ e-mail inexistente (I-11, I-16)
}

func (h *Hasher) Verificar(hash string, s identidade.Senha) (ok, rehash bool) {
	for _, v := range h.todas { // slice ordenado, não map
		if v.Reconhece(hash) {
			return v.Verificar(hash, s)
		}
	}
	return false, false
}
```

```go
// catalogomysql/ordenacao.go: o enum vira coluna, sem string livre na assinatura
func colunaProduto(o catalogo.OrdemProduto) string {
	switch o {
	case catalogo.OrdemSKU:
		return "sku"
	case catalogo.OrdemPreco:
		return "preco_tabela"
	// …
	default:
		return "id"
	}
}
```

O parse `order_by` → enum fica em `catalogohttp`, com fallback para o default. Não existe teste que precise mudar: `sort_test.go` passa a testar `kernel.ParseOrdenacao` e os `switch` (o nome do teste é mantido).

### 5.7 Decorator

- **Onde:**
  - `httpx.Logging` (ex-`withLogging`, `main.go:202-212`): log `[http] método caminho -> status (duração)`, **sem** query string (F-09);
  - `httpx.Trace`: em cada rota, vlog de entrada e saída com o rótulo do handler (§6);
  - `notificacaosmtp.ComLogMascarado`: envolve o `EnviadorAlerta` e loga destinatário mascarado e resultado.
- **Simplifica:** tira o log de dentro dos handlers.
- **Proibido:** decorator de cache em repositório, checker ou escopo (R4, I-03, E-01).

```go
// httpx/trace.go
func Trace(arquivo, funcao string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !vlog.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		vlog.Printf(arquivo, funcao, "início %s", r.Method)
		next.ServeHTTP(rec, r)
		vlog.Printf(arquivo, funcao, "fim status=%d", rec.status)
	})
}
```

### 5.8 Chain of Responsibility (middlewares)

- **Onde:** `httpx.Chain`, com ordem **fixa e testada** (F-01). A ordem efetiva, de fora para dentro, é:

  `Logging( CORS( SecurityHeaders( mux → Exigir(policy)( Trace(arquivo, funcao)( handler ) ) ) ) )`

  - Por rota, a autenticação fica **por fora** do `Trace` (O9). Um 401 ou 403 é escrito pelo `Autenticador` e **não** emite o rótulo vlog do handler, porque o handler não foi alcançado. Em rota `Public` não há `Exigir`: fica só `Trace(handler)`.
  - **Invariante do `httpx.CORS` (O3):** **todo** request `OPTIONS` é respondido com **204** pelo CORS, **antes** do mux, com ou sem `Origin` e com ou sem `Origin` permitida (hoje é assim: `cors_middleware.go:61-64`). Os headers `Access-Control-Allow-*` só são escritos para origem da allowlist (F-02). Consequência: um preflight **nunca** chega ao mux, nunca vira 405 da exceção X1 e nunca consulta o checker.
- **Simplifica:** some a montagem manual `corsChain(securityChain(mux))` (`routes.go:466-476`) e o `withLogging(mux)` separado no `main.go`.

```go
// httpx/chain.go
type Middleware func(http.Handler) http.Handler

// Chain aplica do mais externo para o mais interno: Chain(h, A, B) == A(B(h)).
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// bootstrap/server.go
handler := httpx.Chain(router.Mux(),
	httpx.Logging,                        // mais externo
	httpx.CORS(cfg.CORSAllowedOrigins),   // OPTIONS → 204 antes de auth e antes do 405
	httpx.SecurityHeaders,
)
```

Testes exigidos no ARQ-01 (`tests/routes`, todos novos, a contagem só sobe):

| Cenário | Esperado |
|---|---|
| `OPTIONS /api/auth/me` sem token, `Origin` permitida | 204 com `Access-Control-Allow-Origin` = origem; checker não chamado |
| `OPTIONS /api/auth/login` **sem** `Origin` (rota de método fixo) | 204, sem `Access-Control-Allow-Origin`, nunca 405 (O3) |
| `OPTIONS /api/produtos` com `Origin` **fora** da allowlist | 204 **sem** `Access-Control-Allow-Origin` (O3) |
| `GET /api/auth/me` sem token | 401 com `X-Content-Type-Options` e sem o rótulo `[auth_handler.go] [AuthHandler.Me]` no vlog (O9) |
| `DELETE /api/auth/me` (método errado em rota protegida), com e sem token | 405 com `Allow` e `X-Content-Type-Options`; **checker não consultado** (O3, R-b) |
| `HEAD /api/auth/me` sem token | 401 (o padrão `GET` cobre `HEAD`; HEAD não fura a autenticação) (R-b) |
| `HEAD /health` | 200 |

### 5.9 Builder de rotas com política tipada

- **Onde:** `httpx.Router` e um `routes.go` por contexto (`catalogohttp.Registrar(r *httpx.Router, h *Handlers)`).
- **Simplifica:** `routes.go` cai de 477 linhas para ~60 linhas declarativas por contexto. `NewMux` deixa de ter 14 parâmetros (B8). A política deixa de ser um par de booleanos (C4).

```go
// httpx/router.go
type Policy uint8

const (
	policyInvalida Policy = iota // zero value: panic na montagem
	Public
	Authenticated
	AdminOnly
)

type Rota struct {
	Metodo, Padrao string // método OBRIGATÓRIO (exceção X1)
	Policy         Policy
	Arquivo        string // rótulo vlog: "produto_handler.go"
	Funcao         string // rótulo vlog: "ProdutoHandler.List"
	Handler        http.HandlerFunc
}

type Router struct {
	mux   *http.ServeMux
	auth  *Autenticador // checker obrigatório (I-03)
	rotas []Rota
}

var metodosValidos = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}

// Handle registra a rota. Qualquer dado inválido é erro de programação:
// panic na montagem (no startup e nos testes), nunca em runtime.
func (r *Router) Handle(rt Rota) {
	switch rt.Policy { // O2: só as três políticas válidas
	case Public, Authenticated, AdminOnly:
	default:
		panic("httpx: rota " + rt.Padrao + " com política inválida")
	}
	if !metodosValidos[rt.Metodo] {
		panic("httpx: rota " + rt.Padrao + " com método inválido: " + rt.Metodo)
	}
	if !strings.HasPrefix(rt.Padrao, "/") || strings.ContainsAny(rt.Padrao, " \t") {
		panic("httpx: padrão inválido: " + rt.Padrao)
	}
	if rt.Handler == nil || rt.Arquivo == "" || rt.Funcao == "" {
		panic("httpx: rota " + rt.Padrao + " sem handler ou rótulo vlog")
	}

	// O9: ordem efetiva auth(Trace(handler)), com a autenticação por fora.
	var h http.Handler = Trace(rt.Arquivo, rt.Funcao, rt.Handler)
	if rt.Policy != Public {
		h = r.auth.Exigir(rt.Policy, h) // AuthN + AuthZ (role do banco)
	}
	r.mux.Handle(rt.Metodo+" "+rt.Padrao, h)
	r.rotas = append(r.rotas, rt)
}

// Rotas expõe a tabela registrada (introspecção para a matriz ARQ-00).
func (r *Router) Rotas() []Rota { return slices.Clone(r.rotas) }

// httpx/auth.go
func (a *Autenticador) Exigir(p Policy, next http.Handler) http.Handler {
	switch p {
	case Authenticated:
		return a.autenticar(next, false)
	case AdminOnly:
		return a.autenticar(next, true)
	default: // Public não passa por aqui; qualquer outro valor é bug
		panic("httpx: Exigir com política inválida")
	}
}
```

Testes de montagem (ARQ-01, novos): `Handle` dá panic para `Policy(0)`, `Policy(99)`, método `""`, `"OPTIONS"`, `"get "`, padrão sem `/` inicial, padrão com espaço, handler nil e rótulo vazio; `Exigir(Public, h)` e `Exigir(Policy(99), h)` dão panic.

```go
// catalogohttp/routes.go
func Registrar(r *httpx.Router, h *Handlers) {
	r.Handle(httpx.Rota{Metodo: "GET", Padrao: "/api/produtos", Policy: httpx.Authenticated,
		Arquivo: "produto_handler.go", Funcao: "ProdutoHandler.List", Handler: h.Produto.List})
	r.Handle(httpx.Rota{Metodo: "POST", Padrao: "/api/produtos", Policy: httpx.AdminOnly,
		Arquivo: "produto_handler.go", Funcao: "ProdutoHandler.Create", Handler: h.Produto.Create})
	// …
}
```

A rota pública **não** passa pelo `Autenticador`, logo não consulta o checker (I-03). O cookie de access token e o Bearer são lidos só em rotas protegidas (o refresh e o logout continuam lendo o **cookie de refresh** no próprio handler, como hoje).

**Mudança interna registrada (O4):** hoje, numa rota pública com token válido, o `newJWTMiddleware` grava no contexto o `userID` e o **role do claim**, sem consultar o banco (`auth_middleware.go:138-161`). No desenho novo, a rota `Public` **não tem** `Principal` nem as chaves antigas no contexto. É uma mudança para melhor (nenhum dado não conferido no banco circula) e sem efeito observável: nenhum handler público lê o contexto. Só `ResetPassword` e `Me`, ambos `Authenticated`, chamam `GetUserID`. O comportamento das rotas públicas é validado pelos testes `auth_refresh_logout_test.go` e `auth_handler_test.go`, sem mudar asserção.

### 5.10 Adapter

| Porta (consumidor) | Adapter | Origem |
|---|---|---|
| `identidadeapp.VerificadorCaptcha` | `adapters/turnstile` (HTTP 4 s, fail-closed → `ErrCaptchaInvalido`, I-13) | `SH/services/turnstile_service.go` |
| `identidadeapp.EmissorToken` | `identidadecrypto.JWT` (HS256 apenas; iat/nbf/exp, I-01) | `SH/services/auth_service.go:99` |
| `notificacaoapp.EnviadorAlerta`, `identidadeapp.EnviadorSenhaInicial` | `notificacaosmtp.SMTP` | `SH/services/email_service.go`, `email_smtp.go` |
| `identidadeapp.Limitador` | `adapters/ratelimit.Memoria` | `API/middleware/ratelimit_middleware.go` |
| `httpx.StatusChecker` | função sobre `identidadeapp.StatusParaAutenticacao` | `API/middleware/user_status.go` |

```go
// identidadeapp/portas.go
type VerificadorCaptcha interface {
	Verificar(ctx context.Context, token, ip, acao string) error // ErrCaptchaInvalido
}
```

### 5.11 Facade / Composition Root

- **Onde:** `rotaperfumes-api/internal/bootstrap`. É o **único** lugar que conhece `config`, `*sql.DB`, os adapters MySQL e os singletons (3 limitadores, notifier, hasher, emissor JWT, Turnstile), criados **uma vez por processo** (I-14, risco do SecBrain).
- **Simplifica:** o `main.go` cai de 248 para ~30 linhas. Somem a injeção por setter (A4) e os construtores que ignoram dependências (A2).

```go
// bootstrap/wire.go
type App struct {
	Handler  http.Handler
	Cleanup  *identidadeapp.LimparRefreshExpirados
	pool     *sql.DB
}

func Montar(cfg *config.Config, pool *sql.DB) *App {
	clock := identidadeapp.RelogioSistema{}
	// singletons por processo
	emailSvc, alertaSender := novoEmail(cfg) // SMTP ou Noop (Null Object)
	hasher := identidadecrypto.NovoHasher(cfg.HashSenha)
	jwt := identidadecrypto.NovoJWT(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	limites := identidadeapp.Limites{
		Login:   ratelimit.Nova(5, time.Minute, 5*time.Minute),
		Refresh: ratelimit.Nova(10, time.Minute, 5*time.Minute),
		Reset:   ratelimit.Nova(10, 5*time.Minute, 10*time.Minute),
	}

	// Catálogo
	produtos := catalogomysql.NovoProdutos(pool)
	catalogoUC := catalogoapp.Novo(produtos, catalogomysql.NovoEstoques(pool), clock)

	// … identidade, comercial, vendas, analytics

	router := httpx.NovoRouter(httpx.NovoAutenticador(jwt, identidadeUC.StatusParaAutenticacao, clock))
	catalogohttp.Registrar(router, catalogohttp.NovosHandlers(catalogoUC))
	// …
	return &App{Handler: httpx.Chain(router.Mux(), httpx.Logging, httpx.CORS(cfg.CORSAllowedOrigins), httpx.SecurityHeaders), pool: pool}
}
```

A ordem do shutdown fica igual (F-04, I-10): `srv.Shutdown(10s)` → cancelar o cleanup e esperar até 10 s → `pool.Close()`.

### 5.12 Observer / Domain Events

- **Onde:** `identidade.ReusoDeRefreshDetectado{UserID, TokenID, IP, UA}`. O caso de uso `RenovarSessao` publica o evento depois do corte de sessões, e o `notificacaoapp.AlertaSegurancaNotifier` assina.
- **Simplifica:** `AuthHandler` deixa de conhecer o notificador (`auth_handler.go:62-74`). Um assinante novo (ex.: auditoria) não mexe em Identidade.

```go
// identidadeapp/eventos.go
type Publicador interface{ Publicar(ctx context.Context, e identidade.Evento) }

// bootstrap: despacho síncrono em processo; o notifier já é assíncrono (I-08)
type publicadorLocal struct{ assinantes []func(context.Context, identidade.Evento) }

func (p publicadorLocal) Publicar(ctx context.Context, e identidade.Evento) {
	for _, a := range p.assinantes {
		a(ctx, e)
	}
}
```

As regras se mantêm: a resposta HTTP é idêntica com ou sem alerta; a falha ao marcar reuso corta mesmo assim (fail-closed); um corte incompleto aciona `DesfazerReuso` (I-07). **Não aplicar:** barramento genérico, fila ou eventos persistidos (outbox). Não há segundo processo consumidor.

### 5.13 Template Method (importers)

- **Onde:** `importers/importkit`. Os 8 importers repetem `ResolveCSVPath → ReadCSVFile → ReadCSV → ParseRow → (dry-run?) → abrir DB → UpsertAll → relatório` (`importers/produtos/produtos.go:84-291`).
- **Simplifica:** cada importer fica só com `Parse` e `Upsert`.

```go
// importers/importkit/importkit.go
type Spec[R any] struct {
	Tag         string // "[importprodutos]"
	CSVPadrao   string // "dados/erp/produtos.csv"
	Cabecalho   []string
	Parse       func(record []string) (R, error) // sem log por linha (LOG-02)
	Upsert      func(db cmdutil.DB, rows []R) (ins, upd, falhas int, err error)
}

func Run[R any](s Spec[R], opts Options, open cmdutil.Opener) error {
	path, err := resolverCSV(opts.CSVFlag, s.CSVPadrao)
	if err != nil {
		return err
	}
	rows, parseErrs, err := lerCSV(path, s.Cabecalho, s.Parse)
	if err != nil {
		return err
	}
	vlog.Printf("importkit.go", "Run", "%s linhas=%d invalidas=%d", s.Tag, len(rows), parseErrs)
	if opts.DryRun {
		return nil
	}
	// … abrir, upsert, relatório (contagens apenas)
}
```

As funções exportadas que os testes atuais chamam (`produtos.ParseRow`, `produtos.ReadCSV`, `produtos.UpsertAll`, `produtos.Run`) continuam existindo como fachada fina sobre a `Spec`, para os 123 testes de importers seguirem sem mudar asserção.

### 5.14 Command / Query (CQRS leve)

- **Onde:** Analytics. São só queries, com read models próprios e SQL agregador em `analyticsmysql`. Os casos de uso de escrita dos outros contextos recebem um **comando** (struct de entrada) e devolvem a entidade.
- **Simplifica:** acaba o `map[string]any` do dashboard (`dashboard_service.go:96-248`). Os read models não passam pelos agregados (sem N+1).

```go
// analyticsapp/metricas.go
type ConsultaMetricas struct {
	Principal kernel.Principal
	Periodo   analytics.Periodo
}

type Metricas struct { // tags JSON = chaves atuais (contrato inalterado)
	Periodo           string  `json:"periodo"`
	TicketMedio       float64 `json:"ticket_medio"`
	TotalPedidos      int     `json:"total_pedidos"`
	VendedorDesligado bool    `json:"vendedor_desligado"`
	// …
}

func (q *Consultas) Metricas(ctx context.Context, c ConsultaMetricas) (Metricas, error)
```

**Não aplicar:** em modelos de leitura separados nem em projeções para os demais contextos. Lá, o CRUD com o mesmo modelo basta.

### 5.15 Null Object

- **Onde:** `notificacaosmtp.Noop` (SMTP não configurado; loga e-mail mascarado, nunca a senha, M-02) e `identidadeapp.SemPublicador{}` em testes.
- **Simplifica:** some o `if h.alerta != nil` (`auth_handler.go`). Todo caso de uso recebe uma dependência real.

### 5.16 Mapper / DTO (HTTP ↔ aplicação ↔ domínio)

- **Onde:** `<ctx>http/dto.go` (request/response com tags JSON) e `<ctx>mysql/linhas.go` (scan ↔ entidade).
- **Simplifica:** o domínio fica sem tags (A7). A resposta de usuário nunca carrega hash (I-12): o DTO não tem o campo. Some o `usuarioToMap` (`usuario_handler.go:144`).

```go
// catalogohttp/dto.go
type produtoResposta struct {
	ID            int64   `json:"id"`
	SKU           string  `json:"sku"`
	PrecoTabela   float64 `json:"preco_tabela"`
	// … mesmas chaves e mesmos omitempty de models.Produto
}

func produtoParaResposta(p *catalogo.Produto) produtoResposta { /* … */ }
func (r criarProdutoRequest) paraDados() catalogo.DadosProduto  { /* … */ }
```

A regra de compatibilidade é: **mesmas chaves, mesmos `omitempty`, mesmos tipos numéricos** do model atual. A ordem das chaves pode mudar (map vira struct): os testes que comparam JSON usam `assert.JSONEq` (`sec08_corte_sessao_test.go:99`, `arq00_matriz_rotas_test.go:478`) ou `Contains`, e o frontend faz parse.

### 5.17 Error mapper

- **Onde:** `httpx/errmap.go`, com uma função única para toda a API.
- **Regras (F-05, SecBrain):** decisão só por sentinela (`errors.As` para `*kernel.Erro`, ou `errors.Is`). A mensagem pública é **a constante do sentinela**, nunca `err.Error()` da cadeia embrulhada. Um erro desconhecido vira `log.Printf` com tag + 500 "erro interno" (R3). Nada de `map[error]…` para decidir. Exceções pontuais em `[]regra` **ordenada**.

```go
// kernel/erros.go
type Kind uint8

const (
	KindInterno Kind = iota
	KindInvalido
	KindNaoEncontrado
	KindConflito
	KindProibido
	KindNaoAutenticado
	KindBloqueado // 429; o Retry-After vem de *kernel.Bloqueio (ver abaixo)
)

// Erro é um erro de domínio com mensagem pública FIXA. Só pode ser criado em
// declaração `var` de pacote, com Kind constante e mensagem LITERAL string
// (TestARQ_NovoErro_SentinelaLiteral, §4.3).
type Erro struct {
	kind Kind
	msg  string
}

func NovoErro(k Kind, msg string) *Erro { return &Erro{kind: k, msg: msg} }
func (e *Erro) Error() string           { return e.msg }
func (e *Erro) Kind() Kind              { return e.kind }

// Bloqueio carrega o tempo de espera do rate limit sem criar mensagem
// dinâmica: Unwrap devolve o sentinela (ex.: identidade.ErrMuitasTentativas).
type Bloqueio struct {
	Sentinela  *Erro
	RetryAfter time.Duration
}

func (b *Bloqueio) Error() string { return b.Sentinela.Error() }
func (b *Bloqueio) Unwrap() error { return b.Sentinela }
```

```go
// httpx/errmap.go
// ResponderErro escreve o envelope de erro. tag identifica o handler no log.
// switch explícito com default fail-closed (O6-ii): não se indexa array por
// Kind, então um Kind novo esquecido cai em 500, nunca em status 0 ou panic.
func ResponderErro(w http.ResponseWriter, tag string, err error) {
	var de *kernel.Erro
	if !errors.As(err, &de) {
		interno(w, tag, err)
		return
	}
	switch de.Kind() {
	case kernel.KindInvalido:
		JSON(w, http.StatusBadRequest, nil, de.Error()) // mensagem constante do sentinela
	case kernel.KindNaoEncontrado:
		JSON(w, http.StatusNotFound, nil, de.Error())
	case kernel.KindConflito:
		JSON(w, http.StatusConflict, nil, de.Error())
	case kernel.KindProibido:
		JSON(w, http.StatusForbidden, nil, de.Error())
	case kernel.KindNaoAutenticado:
		JSON(w, http.StatusUnauthorized, nil, de.Error())
	case kernel.KindBloqueado:
		ResponderBloqueado(w, err) // 429 + Retry-After >= 1s (I-14)
	default: // KindInterno, ErrEscopoInvalido, ErrPrincipalInvalido, Kind desconhecido
		interno(w, tag, err)
	}
}

func interno(w http.ResponseWriter, tag string, err error) {
	switch {
	case errors.Is(err, kernel.ErrEscopoInvalido), errors.Is(err, kernel.ErrEscopoSemAcesso):
		log.Printf("[escopo] invalido %s", tag) // R-e: tag fixa para alertas; sem detalhes
	case errors.Is(err, kernel.ErrPrincipalInvalido):
		log.Printf("[auth] principal invalido %s", tag)
	default:
		log.Printf("%s: %v", tag, err) // como hoje: só log, nunca resposta
	}
	JSON(w, http.StatusInternalServerError, nil, "erro interno")
}
```

- **Teste novo (ARQ-01):** `TestErrMap_TodosOsKinds` percorre `kernel.Kind(0)` até `kernel.KindBloqueado + 5` e confere o status esperado; os valores fora do enum devolvem 500 "erro interno" e geram log. Um teste de tabela confere que a resposta usa a mensagem do sentinela e **não** o texto do wrap (`fmt.Errorf("catalogomysql: x: %w", ErrProdutoNaoEncontrado)` responde "produto não encontrado").
- **Decisões de fluxo fora do mapper (I-07):** `ErrRefreshRevogadoRecente` é checado **antes** de `ErrRefreshRevogado`. A forma recomendada é `RefreshToken.Avaliar` devolver um enum de estado, e não uma cadeia de erros.
- **Wrap e driver (O6-iv):** o wrap de infraestrutura leva **só o nome da operação** (`fmt.Errorf("comercialmysql: inserir cliente: %w", err)`), nunca valores (CNPJ, e-mail, id de terceiros, filtro). O erro de chave duplicada do MySQL (`Error 1062: Duplicate entry '<cnpj/email>' for key …`) é **traduzido no adapter** para o sentinela (`comercial.ErrCNPJDuplicado`, `identidade.ErrEmailDuplicado`) **sem embrulhar** o erro do driver (`return comercial.ErrCNPJDuplicado`, e não `fmt.Errorf("%w: %v", ErrCNPJDuplicado, err)`). Assim o valor duplicado não chega nem ao log. Hoje o cliente já faz isso (`cliente_repository.go:434-435`, via `isDuplicateKey`). O usuário detecta por `strings.Contains(err.Error(), "Duplicate entry")` (`usuario_repository.go:337-338`); em `identidademysql`, a detecção passa a `errors.As(*mysql.MySQLError)` + número 1062, com o mesmo resultado (`ErrEmailDuplicado`).
- **`errors.Join`** de erro de infra com sentinela é proibido (R-c, §4.3).

### 5.18 Outros padrões pontuais

- **Envelope genérico:** `httpx.JSON(w, status, data, errMsg)` e `httpx.JSONPaginado[T](w, kernel.Resultado[T])` substituem `writeJSON`, `writeJSONWithPagination` e `writeError` (que concatena a string, C12). O `writeError` do middleware passa a usar o encoder JSON sobre um map (ARQ-01): as chaves saem em ordem alfabética, `{"error":"…","success":false}` (antes `success` vinha primeiro); os testes usam `JSONEq`/`Contains`. O envelope do `httpx` é uma struct com campos em ordem alfabética para sair byte a byte igual ao `map[string]any` legado.
- **Clock como dependência** (`identidadeapp.Relogio`) no lugar de `var timeNow` (C10) e de `SetNow`.

### 5.19 Onde NÃO aplicar (overengineering)

| Não fazer | Por quê |
|---|---|
| Container de DI / reflection (wire, fx, dig) | O bootstrap explícito tem ~200 linhas e é legível. |
| `Repository[T]` genérico, `BaseEntity`, `AggregateRoot` com eventos embutidos | Não há ganho; Go não tem herança. |
| Interfaces "para tudo" do lado do produtor (`IProdutoService`) | Interface pequena no consumidor; o handler depende de `catalogoapp.Casos`, uma struct concreta, e os testes HTTP usam sqlmock como hoje. |
| Decorator de cache | Proibido (R4). |
| Specification combinável, Visitor | Nenhum filtro precisa disso. |
| Mediator / command bus | Chamar o método do caso de uso direto é mais claro. |
| Event sourcing, outbox, sagas | Há um único processo e um único banco. |
| Agregados para importers | Carga em lote idempotente não precisa de invariantes por entidade. |
| Value Object para todo campo `string` (Bairro, Cidade, Nome) | Só vira VO o que tem regra (CNPJ, UF, Email, Data, Dinheiro). |
| Separar `Visita` e `Oportunidade` em contextos próprios | São CRUD simples na mesma linguagem do Comercial. |

---

## 6. Estratégia de logging

### 6.1 Requisitos que não podem cair

- **F-09 / LOG-02:** vlog desligado por padrão; formato `[arquivo] [Funcao] descrição`; e-mail só via `MaskEmail`; nada de valores (seção 5 de `docs/logs-verbose.md`); nada de log por iteração.
- **TST05** (`tests/handlers/tst05_vlog_runtime_auth_test.go:320`): com vlog ligado, a saída **tem de conter** `[auth_handler.go] [AuthHandler.Login]` e `[auth_handler.go] [AuthHandler.ResetPassword]`, e **nenhum** segredo. Por isso `identidadehttp/auth_handler.go` mantém o nome do arquivo e o tipo `AuthHandler`, com os métodos `Login` e `ResetPassword`.
- **`shared/tests/vlog`:** o formato da linha não muda (`vlog.Printf` fica igual).
- **Logs operacionais (`log.Printf`, sempre ligados):** `[http] …`, `[auth] acesso negado …`, `[escopo] bloqueado vendedor_desligado …`, `[server] …` continuam iguais. São contrato de operação, e os testes de SEC procuram parte deles.

### 6.2 Os quatro níveis (aprovado pelo usuário, P1)

| Nível | Quem emite | Exemplo |
|---|---|---|
| 1. Rota | `httpx.Trace`, automático pelo builder (§5.7) | `[produto_handler.go] [ProdutoHandler.List] início GET` / `fim status=200` |
| 2. Caso de uso | `defer vlog.Trace(...)(&err)` na 1ª linha de cada método de `<ctx>app` | `[produtos.go] [Catalogo.ListarProdutos] início page=1 limit=20` / `fim ok total=37 dur=3ms` |
| 3. Decisão | linha explícita em cada ramo com significado de negócio ou segurança | `escopo=vendedor id=7`, `rate limit bloqueado retry=42s`, `captcha recusado`, `reuso detectado token_id=9 cortar=true` |
| 4. Query | 1 linha por operação de repositório: nome + contagem | `[produtos.go] [Produtos.Listar] SELECT produtos linhas=20 total=37` |

```go
// shared/vlog/trace.go: helper novo, compatível com o formato atual
// Trace emite "início" agora e "fim" no defer. Erro só pelo Kind (nunca err.Error()).
func Trace(arquivo, funcao, format string, args ...any) func(errp *error) {
	if !Enabled() {
		return func(*error) {}
	}
	inicio := time.Now()
	Printf(arquivo, funcao, "início "+format, args...)
	return func(errp *error) {
		if errp != nil && *errp != nil {
			Printf(arquivo, funcao, "fim erro=%s dur=%s", classificar(*errp), time.Since(inicio))
			return
		}
		Printf(arquivo, funcao, "fim ok dur=%s", time.Since(inicio))
	}
}
```

`classificar` devolve o `Kind` do `kernel.Erro` (ex.: `nao_encontrado`) ou `"interno"`. Assim o `vlog` não depende de `kernel`: a interface `interface{ KindNome() string }` é declarada no próprio `vlog`.

### 6.3 O que cada nível pode e não pode logar (O7)

| Nível | Pode | Nunca |
|---|---|---|
| 1. Rota | método HTTP, status, rótulo `[arquivo] [Funcao]` | path com query string, headers, cookies, corpo |
| 2. Caso de uso (`vlog.Trace`) | IDs numéricos, contagens, `page`/`limit`, enums (`periodo=mes`, `ordem=sku`), booleanos, `Kind` do erro | `%v`/`%+v`/`%#v` de comando, entidade, DTO ou filtro; textos digitados (nome, busca, razão social) no máximo como `len()` ou `busca=sim/não` |
| 3. Decisão | IDs numéricos, papel, booleanos, `retry` em segundos, `token_id` (id da linha, não o token), e-mail **só** via `Email.Mascarado()`/`vlog.MaskEmail` | a **chave** do rate limit (`acct:<ip>|<email>`, `reset:<ip>|<uid>`); token Turnstile, refresh token, hash, JWT, senha, **nem mascarados**; CNPJ; IP e User-Agent |
| 4. Query | **só** o nome da operação (`SELECT produtos`, `UPDATE refresh_tokens revogar`) e contagens (`linhas=20 total=37 afetadas=1`) | texto do SQL, args, placeholders, valores de filtro (no máximo `busca=sim/não`) |

**Quantidades numéricas (ARQ-02, NB-2 do 🟣 SecBrain):** uma quantidade numérica não sensível digitada pelo usuário (ex.: `saldo=%d` no `AjustarEstoque`/`AtualizarSaldo`) conta como **contagem** e pode ir para o nível 2. Valores de texto livre (SKU, busca, nomes) continuam proibidos. No log operacional (`log.Printf`) eles vão com `%q`, para que um CR/LF não forje linhas de log.

**Verificação automática (ARQ-01):** `TestARQ_VlogSemConteudoSensivel` (§4.3) roda sobre todas as chamadas a `vlog.Printf` e `vlog.Trace` nos dois módulos, exceto dentro do próprio pacote `vlog`. Ele rejeita `%v`/`%+v`/`%#v` no formato e argumentos cujo identificador case com `(?i)senha|password|token|secret|cnpj|email|hash|captcha|chave|key` sem `MaskEmail`/`Mascarado()`. Ids como `tokenID`/`token_id` (inteiro) passam por uma allowlist explícita no teste, por nome **e** tipo inteiro. O TST05 continua sendo a prova em runtime (nenhum segredo na saída com vlog ligado).

### 6.4 Regras gerais

- **Fica proibido** daqui para frente: vlog de sintaxe ("declarando variável", "verificando se err != nil"), `if cfg.Verbose { log.Printf }` (B9; vira nível 3) e vlog em `kernel`/domínio (o domínio é puro; quem loga é a aplicação).
- **Estimativa:** de 3.726 para ~600 chamadas, e de ~45% para <10% das linhas de produção.
- **Por que não decorator de interface por caso de uso:** exigiria uma interface e um wrapper por caso de uso (~70). O `defer vlog.Trace` é uma linha e tem o mesmo efeito.
- **Documentação:** `docs/logs-verbose.md` §2, §6 e §7 é atualizado no ARQ-08 com os níveis e a tabela do §6.3. Até lá, o código novo segue esta seção.

---

## 7. Plano de migração

### 7.1 Linha de base e portões (todos os lotes)

- **Linha de base** (🔴 TestBrain, 2026-10-13, arquivos em `scratchpad/baseline-testes/`; o SubBrain versiona em `docs/arq-baseline-testes.md` no ARQ-01):
  - API: 720 testes de topo (`tests/handlers` 439, `tests/middleware` 52, `tests/routes` 25, `tests/services` 204), 3.312 execuções PASS, cobertura 91,5%;
  - shared: 593 testes de topo, 2.252 execuções PASS, cobertura 85,6%.
- **Portão de cada lote** (critério de aceite, além do contrato HTTP inalterado):
  1. `go build ./... && go vet ./... && go test ./...` nos dois módulos;
  2. **R1:** `go test -list . ./...` por módulo ≥ linha de base, **nenhum nome de teste some** (diff de nomes, não só da contagem), sem `t.Skip` novo;
  3. as suítes `SEC*`, `TST05*`, `Escopo*`, `MAIL01*` e `SEC10_TLS*` passam **sem diff de asserção** (`git diff` dos arquivos mostra só imports e construtores);
  4. **cobertura — portão oficial (decisão 🤍 MegaBrain, ARQ-01): a cobertura combinada API+shared não cai E o número absoluto de statements descobertos não sobe.** O percentual por pacote **não** é critério quando o código só mudou de lugar (no ARQ-01, `middleware` foi de 89,7% para 86,7% com os mesmos 30 statements descobertos, porque CORS e headers, cobertos, foram para o `httpx`). Alvo: `make cover-combinada`. Como o código vai migrar da API para `shared`, a medição passa a ser `go test ./... -coverpkg=./...,github.com/rotaperfumes/shared/...` no módulo da API, somada à do módulo shared (blocos mesclados). Sem isso, a cobertura de shared cairia artificialmente (o código migrado é exercitado pelos testes da API, em outro módulo). O script do TestBrain ganha esse modo no ARQ-01;
  5. testes de arquitetura e allowlists (§4.2, §4.3) verdes;
  6. matriz de rotas ARQ-00 verde;
  7. o 🟣 SecBrain confere o checklist do lote (§8);
  8. um commit por lote em `refactor/ddd`.

### 7.2 Regras para portar testes (R1)

1. **O teste fica no mesmo módulo e na mesma pasta** (`rotaperfumes-api/tests/services/produto_service_test.go` continua lá), mesmo quando o código testado vai para `shared`. Mudam só imports e construtores. Assim a contagem "por suíte" não se move.
2. **O nome da função `TestXxx` não muda.** Se o nome citar um tipo antigo (`TestProdutoService_ListProdutos…`), ele é mantido como identificador histórico, com um comentário `// ARQ-02: exercita catalogoapp.ListarProdutos`.
3. **Testes de service com sqlmock** passam a montar o caso de uso real com o adapter MySQL real sobre o `db` do sqlmock: `catalogoapp.Novo(catalogomysql.NovoProdutos(db), …)`. As expectativas de SQL (regex) continuam válidas porque **o SQL dos adapters é copiado do repositório antigo sem alteração** (regra explícita: zero mudança de SQL em ARQ-01..07, salvo a adição de `BEGIN`/`COMMIT` onde o UoW passar a envolver o que antes era um repositório abrindo tx, que tem o mesmo efeito no sqlmock: `ExpectBegin`/`ExpectCommit`).
4. **Testes de handler** passam a usar um helper por contexto que monta o handler novo sobre sqlmock (`tests/handlers/helpers_catalogo_test.go: novoProdutoHandler(db, cfg)`). Hoje eles chamam `handlers.NewProdutoHandler(db, cfg)`; a assinatura do helper é igual, então o corpo dos testes não muda.
5. **Testes de repositório** (`shared/tests/repositories`, 314) mudam o construtor (`repositories.NewProdutoRepository()` vira `catalogomysql.NovoProdutos(db)`) e a assinatura (sem `db`; `Escopo` no lugar de `int64`). Onde o teste passava `vendedorID=0` querendo "global", passa `kernel.EscopoIrrestrito(admin)`. Onde passava `>0`, passa `EscopoDoVendedor(id)`. As asserções de resultado e de SQL continuam iguais.
6. **Testes que exercitam código removido** (ex.: `middleware.JWTMiddleware` sem checker, `RequireAdmin()`, C11) são reapontados para o equivalente novo (`httpx.Autenticador` com política) no ARQ-03, com o mesmo nome e a mesma asserção de status e mensagem. Nenhum teste é apagado.
7. **Testes novos** (Escopo, Pagina, Policy, Chain, arquitetura, AST de erros) só **somam**.
8. **`tools/testeemail/testeemail_test.go`** (8 testes internos ao pacote): se o ARQ-07 movê-lo para `tests/tools/testeemail`, o diff de nomes por módulo continua igual (o pacote muda, o nome não). O relatório R1 compara pelo par (nome do teste, módulo).

### 7.3 Exceção X1 ao "contrato inalterado" (aprovada pelo usuário, P2)

O builder de rotas exige método. As 13 rotas registradas hoje sem método passam a ter o método documentado no comentário de `NewMux` (`routes.go:20-79`):

| Padrão atual | Novo padrão | Política (inalterada) |
|---|---|---|
| `/api/auth/login` | `POST /api/auth/login` | Public |
| `/api/auth/refresh` | `POST /api/auth/refresh` | Public |
| `/api/auth/logout` | `POST /api/auth/logout` | Public |
| `/api/auth/reset-password` | `POST /api/auth/reset-password` | Authenticated |
| `/api/auth/me` | `GET /api/auth/me` | Authenticated |
| `/api/senha-historico` | `GET /api/senha-historico` | AdminOnly |
| `/api/senha-historico/` (prefixo) | `GET /api/senha-historico/{user_id...}` | AdminOnly |
| `/api/dashboard/metrics`, `/vendas`, `/vendedores`, `/clientes` | `GET …` | Authenticated |
| `/health` | `GET /health` | Public |

- **Efeito:** um método diferente passa a receber **405** do `ServeMux`, com o header `Allow`. Hoje o handler executa com qualquer método. Pelo `ServeMux` do Go 1.22+, `GET` também casa com `HEAD`, então o healthcheck com HEAD continua 200.
- **`/api/senha-historico/{user_id...}`** (wildcard de resto, um único padrão, AdminOnly). A escolha por `{user_id...}` em vez de `{user_id}` é proposital:
  - `/api/senha-historico/` (id vazio) **continua** chegando ao handler e respondendo 400 "usuario_id é obrigatório na URL". Com `{user_id}`, viraria 404 e quebraria `tests/handlers/senha_historico_handler_test.go:300-309`;
  - `/api/senha-historico/2/extra` deixa de devolver o histórico do usuário 2 (o loop atual em `senha_historico_handler.go:126-135` pega o segmento após `senha-historico` e ignora o resto) e passa a **400 "usuario_id inválido"**, porque `PathValue` devolve `2/extra`. Essa é a parte da exceção X1 para este padrão;
  - o handler deixa de cortar o path à mão e passa a usar `r.PathValue("user_id")`;
  - **parse estrito (R-f):** vazio → 400 "usuario_id é obrigatório na URL" (como hoje); depois, **só dígitos ASCII** (sem sinal, espaço ou `/`; decisão 🤍 MegaBrain no ARQ-01: `+2` e `%202` também viram 400), `strconv.ParseInt(v, 10, 64)` e resultado `> 0` (`httpx.ParseID`). Qualquer falha → 400 "usuario_id inválido". Novidade dentro da X1: `0` e negativos, que hoje passam do `ParseInt`, também viram 400. Testes novos: `/api/senha-historico/2/extra`, `/api/senha-historico/0`, `/api/senha-historico/-1`, `/api/senha-historico/..%2F2` e `/api/senha-historico/2%2F3` → 400 (ou 404, se o próprio `ServeMux` recusar ou limpar o caminho antes do handler; o teste aceita só esses dois status e verifica que o checker de escopo/repositório **não** foi chamado);
  - a matriz ARQ-00 já exercita `/2`, `/` e `/2/extra` (`arq00_matriz_rotas_test.go:94`); as asserções de política (401/403) não mudam.
- **Frontend conferido** (`frontend/src/lib/api.ts`, `apiClient.ts`): login `POST` (api.ts:146), refresh `POST` (api.ts:160, apiClient.ts:175), me `GET` (api.ts:168), reset-password `POST` (api.ts:188,199), logout `POST` (apiClient.ts:679), dashboard `GET` (api.ts:285,296,318,541 via `listarPaginado`/GET), senha-historico `GET /api/senha-historico[/id]` (api.ts:346). `/health` é chamado pelo Caddy (`deploy/Caddyfile:40`, rewrite de `/api/health`) e pelo smoke do Jenkins (`Jenkinsfile:167`) com GET. **Nenhuma chamada usa outro método.**
- **Interação com CORS e com a cadeia:**
  - **invariante do `httpx.CORS` (O3):** todo `OPTIONS` (com ou sem `Origin`, com origem permitida ou não) recebe 204 **antes** do mux. É o comportamento atual (`cors_middleware.go:61-64`) e passa a ser invariante documentada e testada. Assim o preflight nunca chega ao mux e nunca vira 405;
  - o 405 é gerado pelo mux, que fica **dentro** de `SecurityHeaders`, então leva os headers de segurança;
  - o 405 sai **antes** da autenticação por rota: um método errado numa rota protegida dá 405 com ou sem token (e não 401) e **não consulta o checker**. Isso não vaza nada além do `Allow`, que já está documentado;
  - o corpo do 405 é o texto padrão do `ServeMux` (não é o envelope JSON); manter assim é aceitável e evita lógica extra;
  - os testes do ARQ-01 estão na tabela do §5.8 (OPTIONS sem `Origin`, OPTIONS com origem fora da allowlist, 405 sem consultar o checker, `HEAD /api/auth/me` → 401, `HEAD /health` → 200), mais `DELETE /api/auth/login` e `GET /api/auth/login` → 405 com `X-Content-Type-Options`.
- **Matriz ARQ-00:** os 13 padrões da tabela `matrizRotas` mudam de string (recebem o método). Para padrões sem método, a matriz exercita todos os métodos (`todosMetodos`). Com o método fixo, a matriz passa a exercitar o método da política e acrescenta a verificação "outros métodos → 405". **As asserções de política (401/403/checker não consultado) ficam iguais.**

### 7.4 Adaptação da matriz de rotas ARQ-00 (ARQ-01)

Hoje `TestARQ00_Matriz_CobreTodasAsRotasDeRoutesGo` lê `routes/routes.go` com a regex `mux\.Handle(?:Func)?\("…"` (`arq00_matriz_rotas_test.go:259`). Com o builder, o registro deixa de ser `mux.Handle` literal. A troca, **sem enfraquecer** o teste:

1. o router expõe `Rotas() []httpx.Rota` (§5.9), e o bootstrap expõe (ARQ-01): `bootstrap.NovoRouter(cfg, checker, Handlers) *httpx.Router` (fonte de `Rotas()`); `bootstrap.HandlerHTTP(cfg, checker, Handlers)` (CORS → SecurityHeaders → mux, **sem** Logging: substitui o `routes.NewMux` nos testes, para não injetar linhas `[http]` em quem captura o log); `bootstrap.Montar(cfg, pool)` (= `HandlerHTTP` + `httpx.Logging`, produção); `bootstrap.NovosHandlers(pool, cfg, email)` cria os handlers legados;
2. o teste de cobertura passa a comparar `matrizRotas` com a **tabela registrada em runtime** (padrão `Metodo+" "+Padrao` **e** `Policy`). Isso é mais forte que a regex, porque também confere que a política registrada é a da matriz, e não só que a rota existe;
3. continua a falha bidirecional: rota registrada fora da matriz e rota na matriz fora do registro;
4. continua a detecção de duplicidade (o próprio `ServeMux` dá panic em padrão duplicado; o teste também verifica a lista);
5. o nome `TestARQ00_Matriz_CobreTodasAsRotasDeRoutesGo` é mantido (R1) com um comentário `// ARQ-01: fonte = router.Rotas()`. Os testes de política (sem token → 401; normal em admin → 403; rota pública não consulta o checker) passam a chamar `bootstrap.HandlerHTTP` em vez de `routes.NewMux`, sem mudar asserção;
6. durante a transição (ARQ-01 a ARQ-06), **todas** as rotas já são registradas pelo builder novo, inclusive as dos contextos ainda legados (§7.5), então a matriz tem uma única fonte desde o ARQ-01;
7. **R-b:** a matriz ganha dois casos por rota protegida: `HEAD` sem token → 401 (só nas rotas `GET`, já que o `ServeMux` casa HEAD com GET) e um método não registrado → 405 **sem consultar o checker** (o checker fake conta as chamadas e o teste exige zero).

### 7.5 Como a API continua funcionando durante a transição

- **No ARQ-01, o router novo assume todas as 60 rotas de uma vez**, apontando para os handlers **legados** dos contextos ainda não migrados: `httpx.Rota{…, Handler: legado.Produto.ListProdutos}`. `routes/routes.go` é apagado no ARQ-01 e o `cmd/server` passa a chamar `bootstrap`.
- **Principal na transição (O4), sem ponte exportada:**
  1. **ARQ-01 e ARQ-02:** o `httpx.Autenticador` **delega** ao `middleware.JWTMiddlewareWithUserCheck` atual, com a política traduzida (`Authenticated` → `(true,false)`, `AdminOnly` → `(true,true)`). As rotas `Public` não passam pelo `Autenticador` (§5.9). O middleware legado continua sendo o **único** código que grava as chaves antigas (`keyUserID`, `keyRole`), e só depois de validar o JWT e conferir ativo, corte de sessão e role **no banco** (I-03, I-04).
  2. Dentro do `Autenticador` existe um invólucro **não exportado** (`derivarPrincipal`), encaixado entre o middleware legado e o próximo handler. Ele **só roda se o legado aprovou** (o legado chama `next` apenas nesse caso), lê `userID` e `role` com `middleware.GetUserID/GetRole` (o role é o do banco, já gravado pelo legado), chama `kernel.NovoPrincipal` e guarda o Principal com a chave não exportada do `httpx`. Se a leitura falhar (`!ok`) ou o Principal for inválido, responde 500 "erro interno" sem chamar o handler (fail-closed).
  3. **Nenhuma função exportada** grava Principal ou chaves antigas a partir de dados arbitrários. Os handlers legados continuam lendo `middleware.GetUserID/GetRole`, gravados pelo próprio legado; os novos leem `httpx.PrincipalFrom(r) (kernel.Principal, bool)`. Em handler protegido, `!ok` vira 500 "erro interno".
  4. **ARQ-03:** o AuthN/AuthZ é reescrito em `httpx/auth.go`, junto com o SecBrain, e o middleware legado é apagado. Os handlers legados que ainda restarem (Comercial, Vendas, Analytics, até ARQ-04..06) precisam continuar achando as chaves antigas. Se for indispensável, cria-se `middleware.ComPrincipalLegado(ctx, p)`, que recebe **só** um `kernel.Principal` já validado. O único chamador permitido é `internal/httpx/auth.go` (`TestARQ_ChavesLegadas_SoMiddlewareLegado`, §4.3), e a função é removida no ARQ-07, quando o último handler legado sai. A alternativa preferida é migrar `GetUserID/GetRole` para ler o Principal do `httpx`, sem setter algum.
  5. **Rota Public:** sem Principal e sem chaves antigas no contexto (mudança interna registrada no §5.9).
  6. **Role desconhecido (R-a):** preservado como restrito, com log `[auth] role desconhecido user_id=<id>`.
- **Por contexto (ARQ-02..06):** o lote cria `<ctx>`, `<ctx>app`, `<ctx>mysql` e `<ctx>http`, troca os `Handler:` das rotas do contexto no seu `Registrar`, apaga os arquivos legados do contexto (handler, service, repository e model, quando ninguém mais os usa) e porta os testes. Os pacotes legados continuam compilando com o que resta.
- **Dependência cruzada com legado:** quando um contexto novo precisa de algo ainda legado (ex.: Catálogo no ARQ-02 só precisa de `Principal`, que já existe; Vendas no ARQ-05 precisa do escopo, já migrado no ARQ-04), a ordem dos lotes já cobre. Onde não cobre, o bootstrap implementa a porta nova com uma função que chama o legado (Adapter temporário marcado `// TODO(ARQ-0X): remover`).
- **Helpers compartilhados do legado** (`ParsePagination`, `parseAtivoQuery`, `writeJSON`, `resolverVendedorScope`): no lote em que o equivalente novo nasce, o legado passa a delegar a ele (`func ParsePagination(p, l string) (int, int) { pg := httpx.PaginaDaQueryStrings(p, l); return pg.Numero(), pg.Limite() }`). Assim só existe uma regra.

### 7.6 Lotes

#### ARQ-01: Fundação / shared kernel

- **Entra:** `shared/kernel` (todos os tipos do §3.2, com testes unitários); `shared/db` (`Querier`, `Transactor[R]`); `shared/vlog/trace.go`; `rotaperfumes-api/internal/httpx` (router + Policy + método obrigatório, chain, respond, errmap, query, principal, cors, secheaders, logging, trace; o `Autenticador` delegando ao middleware atual); `internal/bootstrap` (composition root com handlers legados); `cmd/server/main.go` enxuto; testes de arquitetura nos dois módulos; teste de AST dos erros; matriz ARQ-00 adaptada (§7.4); exceção X1 (§7.3).
- **Substitui ou apaga:** `routes/routes.go` (apagado); `main.go` (`withLogging`, `statusRecorder`, `newEmailService` e o wiring vão para o bootstrap); `cors_middleware.go` e `security_headers_middleware.go` passam a `httpx` (os testes em `tests/middleware/cors_middleware_test.go` e `security_headers_middleware_test.go` trocam o import); `handlers/response.go` passa a delegar a `httpx`.
- **Testes:** `tests/routes/*` passam a usar `bootstrap`; os de CORS e headers trocam o import; mais os novos (Chain, Policy, X1, kernel, arquitetura).
- **Aceite:** portões do §7.1; matriz verde com fonte `Rotas()`; `HEAD /health` 200; preflight 204.
- **Riscos:** ordem da cadeia invertida (mitigado pelos testes F-01); rota esquecida (impossível com a matriz bidirecional); 405 inesperado (frontend conferido, §7.3).

- **Registro do ARQ-01 (o que foi feito diferente do desenho, revisado pelo 🤍 MegaBrain):**
  - **`Email.Mascarado`** reimplementa o algoritmo de `vlog.MaskEmail`, porque o kernel não importa vlog (§4.1). A equivalência é coberta por `TestEmail_Mascarado_IgualAoVlog`.
  - **Testes de arquitetura** usam `go/build` + `go/parser`, sem `exec go list` (§4.2). Cada regra tem um teste com fonte sintética que prova a detecção.
  - **API do bootstrap:** `NovoRouter`, `HandlerHTTP` (sem Logging, usado pelos testes) e `Montar` (com Logging, produção), no lugar de `bootstrap.Rotas(cfg, checker)` (§7.4).
  - **`httpx` importa `shared/config` e o `middleware` legado** até o ARQ-03 (§4.1). `handlers`, `services` e `repositories` continuam proibidos e verificados.
  - **`writeError` do middleware:** usa o encoder; as chaves saem em ordem alfabética (`{"error":…,"success":false}`).
  - **`derivarPrincipal` falha fechado:** se o legado aprovou mas o Principal é inválido (role vazio no banco ou `user_id <= 0` no token), responde 500 "erro interno" sem chamar o handler. Antes o handler rodava; o schema atual torna o caso irreal.
  - **Zero value de `kernel.Pagina`** equivale a `NovaPagina(0, 0)`: página 1, limite 20.
  - **`kernel.Data`** guarda a data de calendário à meia-noite UTC. `DataDe(t)` usa a data no fuso de `t`. `ParseData(s, errCampo)` devolve o sentinela do campo em qualquer falha, inclusive vazio.
  - **Contrato de erro do `db.Transactor.WithTx`:** o erro da `fn` volta **sem embrulho** (o mapper decide pelo sentinela); falhas de begin/commit voltam como `db: begin: %w` / `db: commit: %w`. `NewTransactor` dá panic com pool ou bind nil.
  - **`TestARQ_SprintfSQLSoConstantes`** foi adiado para o ARQ-06, quando nasce o `analyticsmysql`.
  - **Regra O7 (`TestARQ_VlogSemConteudoSensivel`)** é aplicada aos **dois módulos inteiros**, não só aos pacotes novos. Tem 6 exceções revisadas do legado, todas benignas: nome de variável de ambiente em `config.go`, constantes numéricas da política de senha e `%v` de erro/booleano em `email_service.go` e `resetpassword.go`. O teste falha se uma exceção ficar obsoleta, e os pacotes novos não têm exceção.
  - **`make` indisponível** na máquina de desenvolvimento: os alvos novos `arch-check` e `cover-combinada` foram validados rodando os mesmos comandos à mão. A mescla de perfis usa `awk`, com o máximo por bloco.
  - **Ressalvas da revisão do 🟣 SecBrain** (aprovado sem bloqueantes):
    - **N-1:** role **vazio** → 500 no `derivarPrincipal` é uma **exceção consciente a R-a**. Role vazio é diferente de role desconhecido: o desconhecido continua restrito, sem 401/500 novo. O caso é inalcançável com `usuarios.role` ENUM NOT NULL.
    - **N-2:** `kernel.ParseData`/`ParseDataOuHoje` dão panic com `errCampo == nil` (erro de programação que engoliria data inválida).
    - **N-4/N-5:** as allowlists de AST marcam qualquer referência qualificada, chamada ou não (`f := kernel.NovoPrincipal`). `middleware.JWTMiddleware`, `JWTMiddlewareWithUserCheck` e `RequireAdmin` só podem ser referenciados em `internal/httpx/auth.go` (fora os `_test.go`) — `TestARQ_MiddlewareAuthLegado_SoAutenticador`.
    - **N-6:** a matriz confere a contagem por política: 4 Public, 35 Authenticated, 21 AdminOnly. A soma 4/31/25 que constava no topo deste documento estava errada e foi corrigida.
    - **N-7:** checker nil **tipado** (interface não nula guardando ponteiro nil) não dá panic na montagem, igual ao legado `JWTMiddlewareWithUserCheck`. A proteção é o bootstrap montar sempre o checker real.
    - **N-8:** `httpx.interno` loga `%v` do erro, que pode conter o valor cru de um 1062 (`Duplicate entry '<valor>'`). Isso se resolve no ARQ-04 pela regra O6-iv: o adapter traduz o 1062 para o sentinela sem embrulhar o erro do driver.
    - **N-9:** `Dinheiro.Somar` não verifica overflow nem infinito. Revalidar no ARQ-05 (uso real) e no ARQ-09 (centavos).
  - **Race detector indisponível** nesta máquina (sem gcc para `-race`). Os testes rodaram sem `-race`; rodar com `-race` no CI (Linux) quando disponível.

#### ARQ-02: Catálogo (piloto)

- **Entra:** `catalogo`, `catalogoapp`, `catalogomysql` e `catalogohttp`; enums `OrdemProduto`/`OrdemEstoque` (C-02); `Dinheiro` e `Data` em uso real; o padrão de teste de service com adapter real + sqlmock (§7.2-3) vira referência para os demais lotes.
- **Apaga:** `handlers/produto_handler.go`, `estoque_handler.go`; `services/produto_service.go`, `estoque_service.go`; `repositories/produto_repository.go`, `estoque_repository.go`; `models/produto.go`, `estoque.go` (se nenhum importer ou tool os usar; hoje nenhum usa); a parte de produto e estoque de `repositories/sort.go`.
- **Testes:** `tests/handlers/produto_handler_test.go` (750 linhas), `estoque_handler_test.go`; `tests/services/produto_service_test.go`, `estoque_service_test.go`; `shared/tests/repositories/produto_repository_test.go`, `estoque_repository_test.go`, `sort_test.go` (os casos de injection de order_by viram casos de parse de enum, com o mesmo nome).
- **Aceite:** §7.1; medir a redução de linhas (meta: −40% nos arquivos do contexto); revisar o blueprint com o aprendizado do piloto, antes do ARQ-03 (atualização deste documento).
- **Riscos:** o piloto revela atrito no padrão de teste. Mitigação: ajustar o blueprint antes de seguir.

- **Registro do ARQ-02 (o que foi feito, o que divergiu do desenho e as lições que ajustam ARQ-03..07; 🟡 BackBrain, 2026-10-14):**
  - **Entregue.**
    - `shared/catalogo`: `Produto` (com `CamposProduto` embutido), `RegistroEstoque`, o VO `SKU`, `DadosProduto`/`DadosEstoque` (comandos), `FiltroProduto`/`FiltroEstoque` (query objects), os enums `CampoProduto`/`CampoEstoque` com as whitelists e as ordenações padrão, e 15 sentinelas `kernel.NovoErro` com as mensagens do legado.
    - `shared/catalogo/catalogoapp`: as portas `Produtos`/`Estoques` e os casos de uso `ListarProdutos`, `ObterProduto`, `CriarProduto`, `AtualizarProduto`, `AlternarAtivoProduto`, `ListarEstoque`, `ObterEstoque`, `AjustarEstoque` e `AtualizarSaldo`.
    - `shared/catalogo/catalogomysql`: `Produtos` e `Estoques`, com o SQL copiado sem alteração.
    - `rotaperfumes-api/internal/catalogohttp`: `ProdutoHandler`, `EstoqueHandler` e `dto.go`.
    - `httpx/body.go`: `DecodificarJSON`, `LerAtivoOpcional` (o `handlers/ativo_body.go` legado passou a delegar), `IDDoPathPermissivo` e `JSONPaginadoCom`.
    - `kernel.Data.MeiaNoiteEm(loc)`.
  - **Portões.** gofmt, build, vet e test verdes nos dois módulos. R1: nenhum nome some (API 781 → 793; shared 626 → 652). A cobertura combinada foi de **89,00% para 89,07%**, e os statements descobertos caíram de **1297 para 1248**. O código novo está 100% coberto.
  - **Divergências do desenho:**
    - **(a) Edição de produto sem `(*Produto).Atualizar`.** O legado faz `UPDATE` e depois relê, sem leitura prévia. Carregar o agregado antes acrescentaria uma query e quebraria as expectativas do sqlmock. Por isso a edição valida `NovosCamposProduto(d)` e a porta recebe `Atualizar(ctx, id, CamposProduto)`.
    - **(b) Entidades com campos exportados**, criadas por fábrica, e não campos privados com getters. Os testes portados acessam `p.ID` e `p.Descricao` sem mudança. O custo aparece nos literais dos testes de repositório, que precisam de `CamposProduto: catalogo.CamposProduto{…}`.
    - **(c) `Unidade` não virou VO.** A única regra é ser obrigatória, a mesma de descrição, categoria e marca (§5.19).
    - **(d) `Dinheiro` só valida**, via `kernel.NovoDinheiro` na fábrica. Os campos continuam `float64`: com `Dinheiro` como tipo de campo, a leitura de uma linha legada negativa viraria 500. Decidir no ARQ-05/09, junto com uma restrição no banco (🌸 DataBrain).
    - **(e) Os casos de uso do Catálogo não recebem `Principal`.** A única regra de acesso é a política da rota (C-01). Os handlers de escrita leem o `Principal` só para o log de auditoria (`[produtos] criado … role=`) e falham fechado com 500 se ele faltar (§9).
    - **(f) A tabela de rotas continua central** em `bootstrap/rotas.go`, sem `catalogohttp.Registrar` (§5.9). Ela é a fonte única da matriz ARQ-00, e os nomes de tipo e método dos handlers foram mantidos porque são o rótulo do vlog de rota.
    - **(g) Não houve decorator de trace.** O nível 2 é o `defer vlog.Trace(...)(&err)` de cada caso de uso, como no §6.4.
  - **Remoção do legado.**
    - **Apagados:** `handlers/produto_handler.go`, `handlers/estoque_handler.go`, `services/produto_service.go`, `services/estoque_service.go`, `repositories/produto_repository.go`, `models/produto.go` e `models/estoque.go`. Nenhum importer, tool ou CLI os usava.
    - **`repositories/estoque_repository.go` foi encolhido para 126 linhas e não apagado.** O `PedidoRepository.UpdateComItens` chama `AjustarSaldoPorFaturamento` dentro da tx do pedido. Mover essa chamada para o adapter novo agora faria o legado de Vendas importar `catalogomysql` e mudaria a fronteira transacional antes do ARQ-05. Ficaram só `AjustarSaldoPorFaturamento` e `UpsertPorDataSku`, que não tem chamador de produção e saiu junto para não espalhar o legado.
    - **`sort.go` fica**, porque os outros repositórios ainda o usam. As whitelists de produto e estoque saíram com os arquivos.
    - **`nullTimeFrom` foi para `pagamento_repository.go`**, seu único usuário.
  - **Contrato HTTP.** Três armadilhas que valem para todo lote:
    1. **Parse de id.** O legado usa `strconv.ParseInt`: `/api/produtos/-1` e `/0` respondem **404**. O `httpx.IDDoPath` estrito (R-f) responderia 400, o que é mudança de contrato. Daí o `httpx.IDDoPathPermissivo`. **ARQ-03..06 usam o permissivo**, salvo exceção aprovada como a X1 (sugestão: card SEC para adotar o parse estrito em toda a API de uma vez).
    2. **DTO de update só com os campos editáveis.** O legado tinha structs separadas para create e update. Com um DTO único, `{"sku": 5}` num PUT viraria 400 por tipo errado. A regra é usar um DTO por operação, como no legado.
    3. **Releitura após escrita.** No create, uma falha na releitura devolve o objeto em memória (BUG-09). No update, **qualquer** falha na releitura é 500, inclusive "não encontrado", porque o legado devolvia o erro cru do repositório. O `relerAtualizado` embrulha com `%v` (sem `%w`) de propósito, para o sentinela não virar 404.
    - **Teste novo de contrato (padrão para os demais lotes):** `TestARQ02_ContratoJSON_ByteAByte` compara a resposta com structs **congeladas** dos models legados apagados, serializadas pelo envelope legado (map + Encoder).
  - **Datas (BUG-01).** O `kernel.Data` guarda a data à meia-noite UTC, mas o banco (DSN `loc=Local`) e o JSON precisam da **meia-noite em `time.Local`**. Sem isso, o driver grava o dia anterior. A entidade guarda `time.Time` local (`Data.MeiaNoiteEm(time.Local)`), e o `Data` fica no parse e nos filtros (`FiltroEstoque.De/Ate` → `String()` no SQL). **ARQ-04/05 têm 6 campos de data**: promover o `dataLocal` do Catálogo para o kernel (`kernel.ParseDataLocal(s, errCampo)`).
  - **Lições que ajustam ARQ-03..07:**
    1. **Meta de linhas.** Saiu **−21% físico** (1953 → 1547) e **−24% sem comentários e linhas em branco** (1555 → 1181), contra a meta de −40%. Quase todo o ganho veio das 318 linhas de vlog sintático. As camadas novas (enum + switch C-02, DTO separado do domínio, portas, comandos, query objects) custam o mesmo que a lógica legada sem vlog (≈1237 linhas). **Proposta:** trocar a meta por "≤ lógica legada sem vlog, handlers ≤ 30 linhas e código novo 100% coberto". Os contextos com mais regras (Identidade, Vendas) tendem a ganhar mais com fábricas e VOs.
    2. **Portão de cobertura.** Apagar legado denso em vlog **derruba o percentual** mesmo quando os descobertos caem: sai muito statement coberto. Na primeira medição, os descobertos já tinham caído de 1297 para 1273, mas o percentual ficou em 88,85%. Para passar, o código novo precisa nascer **100% coberto**, e os ramos de erro de infra (LastInsertId, RowsAffected, scan) precisam de teste dedicado. **Planejar esses testes junto com o código**, não no fim.
    3. **Padrão de porte de teste.** O que funcionou: um helper por contexto (`novosCasosCatalogo(db)`, `listarProdutos`/`listarEstoque`, que devolvem a tupla legada `(itens, total, err)`), `novoXHandler(db, cfg)` com a assinatura do construtor legado e `sed` para trocar as chamadas. Os testes de handler HTTP **não mudaram uma linha**.
    4. **Promover para `shared/db`** no ARQ-03/04 o que todo `*mysql` vai repetir: `consultar[T]` (lê as linhas e devolve nil sem linhas, que vira `data: null`), `paginada` e `where`. Para o pacote de aplicação, o mesmo com `relerCriado`/`relerAtualizado` (BUG-08/09 valem para vendedor, cliente, pagamento, oportunidade e visita).
    5. **Rótulos de vlog colidem por arquivo** (`produtos.go` existe na aplicação e no adapter). A convenção que desambigua: `Catalogo.<CasoDeUso>` na aplicação e `<Repo>.<Metodo>` no adapter. O nível 4 loga só o rótulo da operação e as contagens (`SELECT produtos linhas=1 total=1`); o teste de runtime `TestARQ02_Vlog_QuatroNiveisSemValores` prova que busca, categoria, marca, SKU e texto de SQL não aparecem no log.
    6. **Relógio.** O "data futura" usa `Casos.agora` (`time.Now`), sem setter. O ARQ-03 introduz o `Relogio` como dependência do construtor (§5.18).
    7. **gofmt.** Três arquivos SEC12 (`sec12_*integration_test.go`, CRLF) já falham no `gofmt -l` em `HEAD`. Não foram tocados (R1, sem diff em SEC*). Sugestão de card próprio.

#### ARQ-03: Identidade e Acesso

- **Entra:** `identidade`, `identidadeapp` (Login, RenovarSessao, Logout, TrocarSenha, AdminResetarSenha, CRUD de usuário, ObterSessao, HistoricoSenha, StatusParaAutenticacao, LimparRefreshExpirados), `identidademysql`, `identidadecrypto`, `adapters/turnstile`, `adapters/ratelimit`, `identidadehttp` (`auth_handler.go`, `usuario_handler.go`, `senha_historico_handler.go`); `httpx/auth.go` reescrito (extração Bearer/cookie, I-02; Principal; política com o role do banco, I-03/I-05; corte de sessão, I-04); `httpx/clientip.go` (I-14); `httpx/cookies.go` (I-15); o evento `ReusoDeRefreshDetectado` (o notifier atual vira assinante, ainda em `API/services` até o ARQ-07).
- **Apaga:** `handlers/auth_handler.go`, `usuario_handler.go`, `senha_historico_handler.go`; `middleware/auth_middleware.go`, `user_status.go`, `ratelimit_middleware.go`; `services/usuario_service.go`, `refresh_token_service.go`, `senha_historico_service.go`, `refresh_cleanup.go`; `repositories/usuario_repository.go`, `refresh_token_repository.go`, `senha_historico_repository.go`; `SH/services/auth_service.go`, `password_hash.go`, `password_policy.go`, `password_generator.go`, `turnstile_service.go`. O `middleware.ComPrincipalLegado` continua até o ARQ-07, para os handlers legados de Comercial, Vendas e Analytics.
- **Testes:** `tests/handlers/auth_*`, `sec07_*`, `sec08_*`, `sec12_*`, `tst05_*`, `usuario_handler_test.go`, `senha_historico_handler_test.go`; `tests/middleware/*` (52); `tests/services/usuario_*`, `refresh_*`, `senha_historico_*`, `chore02_*`, `sec09_notifier_test.go`, `sec12_chore02_*`, `bug11_*`, `ops01_*`; `shared/tests/services/auth_service_test.go`, `turnstile_service_test.go`, hash e policy; `shared/tests/repositories/usuario_*`, `refresh_*`, `senha_historico_*`, `sec08_*`, `sec12_*`, `bug12_*`, `fe13_*`. `JWTMiddleware` sem checker e `RequireAdmin()` (C11) têm os testes reapontados para `httpx.Autenticador`.
- **Aceite:** §7.1 e checklist ARQ-03 (§8) assinado pelo SecBrain; o `AuthHandler` fica com handlers ≤ 40 linhas cada (só HTTP: decode, cookies, status).
- **Riscos:** é o lote mais sensível. Ordem captcha → limite → credencial; reuso de refresh; cookies. Mitigação: os casos de uso de Identidade são escritos **portando a sequência exata** dos handlers atuais (comentários com o ID de cada invariante no ponto de decisão) e os testes SEC rodam sem diff de asserção.

- **Registro do ARQ-03 (o que foi feito, o que divergiu do desenho e as lições para ARQ-04..07; 🟡 BackBrain, 2026-10-14):**
  - **Entregue.**
    - `shared/identidade` (domínio puro): `Usuario` (campos exportados; o hash é VO), `Senha`/`HashSenha`/`TokenOpaco` (`String`/`GoString` = `***`, `MarshalJSON`/`MarshalText` com erro), `ValidarForca` (política I-12), `ValidarCriacao`/`ValidarEdicao`/`NovoUsuario`, `StatusAcesso.TokenAnteriorAoCorte` (I-04), `ClaimsAcesso`, `CorteDeSessao`, `RefreshToken.Avaliar` (enum `EstadoRefresh`: expiração antes de revogação; `RevogadoRecente` antes de `Revogado`) e `ReusoDeRotacao`, `SenhaReutilizada` (sem short-circuit), `MotivoRevogacao`, `TipoReset`, enums de ordenação, o evento `ReusoDeRefreshDetectado` e 30 sentinelas com as mensagens do legado.
    - `identidadeapp`: portas e os casos `Login`, `RenovarSessao`, `Logout`, `TrocarSenha`, `AdminResetarSenha`, `ResetarSenhasAtivos` (CLI, sem Principal), `ListarUsuarios`, `CriarUsuario`, `AtualizarUsuario`, `AlternarAtivoUsuario`, `ObterSessao`, `StatusParaAutenticacao`, `ListarHistorico[DoUsuario]`, `LimparRefreshExpirados` + `IniciarLimpezaPeriodica`, `RegistrarReuso`/`DesfazerReuso`; `PublicadorLocal` (Observer) e `SemPublicador` (Null Object).
    - `identidademysql` (SQL copiado byte a byte), `identidadecrypto` (Hasher Strategy bcrypt/Argon2id+pepper, JWT HS256, geradores), `adapters/turnstile`, `adapters/ratelimit`.
    - `internal/identidadehttp` (`auth_handler.go`/`AuthHandler`, `usuario_handler.go`, `senha_historico_handler.go`, `dto.go`); `httpx/auth.go` reescrito, `httpx/cookies.go`, `httpx/clientip.go`, `httpx.PrincipalDoContexto`; `bootstrap/identidade.go` (`NovaIdentidade`, `NovoIdentidadeHTTP`, `NovoVerificadorStatus`, `ExtIdentidade`); `db/consulta.go` (`Consultar[T]`, `Paginada`, `Where`, `DirecaoSQL`, promovidos do catalogomysql, lição 4 do ARQ-02).
  - **Portões.** gofmt (fora os 3 arquivos SEC12 com CRLF, já sujos em `HEAD`), build, vet e test verdes nos dois módulos. R1: nenhum nome some (API 793 → 805; shared 652 → 683). Cobertura combinada **89,07% → 89,35%**; descobertos **1248 → 1110**. O código novo está 100% coberto, exceto 7 ramos de erro inalcançáveis de `crypto/rand` (`aleatorio.go`, os mesmos do legado) e o laço do ticker do rate limit.
  - **Divergências do desenho:**
    - **(a) O middleware JWT legado saiu já no ARQ-03.** `JWTMiddleware`, `JWTMiddlewareWithUserCheck`, `RequireAdmin`, `UserStatusChecker` e `LoginRateLimiter` foram apagados. Do pacote `middleware` restou só `principal_legado.go`: `GetUserID`/`GetRole` **leem** o Principal do `httpx`, sem setter (a alternativa preferida do §7.5). `ComPrincipalLegado` não foi necessário. Os testes de `tests/middleware` passam pelo `httpx.Autenticador`, com um helper que traduz o par de booleanos para a `Policy` (a variante "sem checker" usa o role do claim, como antes).
    - **(b) O `httpx` importa o domínio `shared/identidade`** (`StatusAcesso`, `ClaimsAcesso`, `ErrUsuarioNaoEncontrado`) para declarar as portas `ValidadorAcesso` e `ConsultaStatus`, e deixou de importar `config` e o `middleware` (regra nova no teste de arquitetura). O §4.1 foi atualizado.
    - **(c) O JWT entra pelo construtor** (`identidadecrypto.NovoJWT(cfg)`). O checker do router é o próprio caso de uso `StatusParaAutenticacao` em produção, ou `bootstrap.NovoVerificadorStatus(db)` (adapter MySQL direto) nos testes. `bootstrap.HandlerHTTP(cfg, checker, h)` manteve a assinatura: checker nil continua dando panic (SEC-06).
    - **(d) O notifier de alerta continua em `API/services`** (ARQ-07) e passou a **assinar** o evento (Observer) pelo `PublicadorLocal` montado no `Montar`. Por isso `repositories.UsuarioRepository.GetByID` e `models.Usuario` ficam até o ARQ-07.
    - **(e) Legado encolhido, não apagado:** `usuario_repository.go` (`GetByID` do notifier, `GetIDVendedorByUsuarioID` do escopo, `InativarByVendedorID` da tx do desligamento) e `refresh_token_repository.go` (`RevokeAllByVendedorID` da mesma tx, I-09; `FindByUsuario`, sem chamador, mantido com os 5 testes e removido junto com o arquivo). `ErrNotFound`/`rowScanner` foram para `repositories/erros.go`. Apagados: `SetDeveTrocarSenha` de usuário (sem chamador; o subcaso de `crud_mutacoes` saiu) e o `senha_historico_repository.go` inteiro.
    - **(f) A sequência de SQL do reset administrativo foi preservada:** o caso de uso lê o alvo duas vezes, como handler + service legados, porque o passo comum `redefinirSenhaGerada` é compartilhado com o reset em massa.
    - **(g) Os rótulos de erro do adapter** usam o nome de operação do legado (`identidademysql: set ativo`, `identidademysql: desfazer reuso refresh_token`), para que as suítes SEC que conferem a mensagem do wrap mudem só o prefixo do pacote.
    - **(h) Captcha:** a porta recebe `(ctx, token, ip)`. O log "ausente" ou "falha" é decidido pelo token vazio (o adapter devolve `ErrCaptchaTokenAusente` exatamente nesse caso). Qualquer erro recusa (I-13).
    - **(i) O log de criação de usuário** (`if cfg.Verbose { log.Printf(... email=...) }`, B9) virou vlog de decisão com e-mail mascarado; a asserção do `bug11` (suíte não estrita) foi ajustada. O log operacional de falha de e-mail também passou a mascarar o destinatário.
    - **(j) CLIs:** `cmd/resetsenhas` usa `identidadeapp.ResetarSenhasAtivos`, montado por `bootstrap.NovaIdentidade` (caso de uso sem Principal, O5). `tools/resetpassword` e `tools/seedusers` **mantêm o SQL próprio** e só passam a gerar o hash pelo `identidadecrypto`. Motivo: o upsert por e-mail e o placeholder do seed não existem como caso de uso, e os testes sqlmock dessas CLIs não mudam. A migração completa fica no ARQ-07 (M-03).
  - **Revisão (🟣 SecBrain e 🔴 TestBrain, aprovado com ressalvas, sem bloqueantes):**
    - **Conferidos idênticos ao legado:** (a) login de usuário **inativo conta** no rate limit (IP e IP+conta), como antes; (b) o esquema do header é comparado com `strings.EqualFold(…, "Bearer")` (`bearer x` vale); (c) no reset de senha, os obrigatórios (`senha_atual`, `nova_senha`) são checados **antes** do captcha.
    - **Role vazio no banco → 500 "erro interno"** sem chamar o handler (log `[auth] principal invalido`): é a exceção N-1 já aprovada no ARQ-01; role desconhecido não vazio continua restrito (R-a).
    - **Allowlist nova** (`tests/arquitetura/arq03_allowlist_test.go`, varre os dois módulos, pega referência e não só chamada, com controle negativo): `Casos.ResetarSenhasAtivos` só em `cmd/resetsenhas`; `RegistrarReuso`/`DesfazerReuso` só dentro de `identidadeapp`. Os dois últimos ficaram **exportados de propósito**: a suíte SEC-12 exercita janela e relógio por eles, e torná-los não exportados obrigaria a reescrever a suíte estrita sobre `RenovarSessao`.
    - **Mutantes:** os 5 sobreviventes do 🔴 TestBrain foram mortos por `rotaperfumes-api/tests/httpx/arq03_mutantes_test.go` e `shared/tests/identidade/arq03_mutantes_test.go` (manter).
    - **Dívida de teste para o ARQ-07** (reescrever sobre a API nova, sem os shims `helpers_*_test.go`): com o helper que traduz `protected=false` em "não monta o Autenticador", `TestUserCheck_RotaPublica_*` e `TestSEC08_CorteDeSessao_RotaPublicaIgnoraCorte` ficaram tautológicos (a garantia real de rota pública está na matriz ARQ-00 e em `TestSEC06_Router_RotaPublica_NaoConsultaChecker`); e `refresh_rotation_test.go` monta a transação no próprio teste (o UoW real é coberto por `TestRefresh_FalhasNaRotacao`).

  - **Porte de testes.** Nas suítes estritas o diff é de import, construtor e helper. Dentro de linhas de asserção só houve troca de identificador pelo equivalente novo (sentinela ou tipo movido: `repositories.ErrNotFound` e `middleware.ErrUserNotFound` → `identidade.ErrUsuarioNaoEncontrado`, `repositories.UsuarioStatus` → `identidade.StatusAcesso`, o prefixo `repositories:` → `identidademysql:` em 3 mensagens, `st.Role` → `st.Role.String()` e `services.X` → `X` local). **Nenhum valor esperado (status, mensagem, SQL, args, contagem) mudou.** Nos pacotes `tests/services` (API) e `tests/repositories`/`tests/services` (shared), arquivos `helpers_identidade_test.go` repassam os nomes antigos ao código novo, sem regra própria; saem no ARQ-07. Testes novos: `shared/tests/identidade` (domínio, casos de uso com fakes, infra), `tests/httpx/arq03_auth_test.go`, `tests/handlers/identidade_arq03_test.go` (contrato JSON byte a byte de `/me` e `/api/usuarios` contra o map legado) e a prova do Observer em produção (`TestMontar_ReusoDeRefresh_PublicaParaONotificador`).
  - **Linhas.** Contexto legado removido ou encolhido: 5268 físicas, 3160 de lógica sem vlog. Código novo + legado restante: 4336 físicas, 3245 de lógica (+2,7%, pelos DTOs separados, portas, enums e o `consulta.go` genérico, que tirou 55 linhas do catalogomysql). Handlers com no máximo 21 linhas.
  - **Lições para ARQ-04..07:**
    1. **Fakes de porta em `shared/tests/<ctx>`** cobrem barato os ramos de infra e de defesa (admin, Principal inválido) que o sqlmock não alcança. Somados aos testes HTTP, deixam o código novo em 100% sem testes artificiais.
    2. **Testar o contrato JSON byte a byte** contra `map[string]any` + `Encoder` (o envelope legado) confirma que o DTO com campos em ordem alfabética reproduz o legado. Listas que respondiam `[]` precisam de conversão com `make(..., 0)`: o `JSONPaginadoCom` devolve `null`.
    3. **Mover um sentinela muda a linha de asserção.** Para as suítes `Escopo*` (ARQ-04..06), declarar no domínio sentinelas com o mesmo papel dos antigos, trocar só o qualificador e registrar a troca no lote.
    4. **Ponte de leitura em vez de setter:** `middleware.GetUserID/GetRole` lendo o Principal eliminou o `ComPrincipalLegado`; os handlers legados de ARQ-04..06 não mudaram.
    5. **Adapter temporário no bootstrap** (`vendedoresLegado`, `TODO(ARQ-04)`) para a porta de um contexto ainda legado; o ARQ-04 troca pela implementação de `comercialmysql`.
    6. **`gopls imports -w` com o `go` no PATH** resolve os imports em lote depois de um `sed` (não há goimports nesta máquina).

#### ARQ-04: Comercial

- **Entra:** `comercial` (Vendedor, Carteira, Cliente, CNPJ, Visita, Oportunidade, PoliticaEscrita), `comercialapp` (AutorizadorEscopo + CRUDs; `DesligarVendedor` com `Transactor` atravessando a porta `IdentidadeTx`; `CriarClienteNaCarteira` com tx; `VincularCliente` **sem** tx, preservando o comportamento atual (BUG-14)), `comercialmysql` (recebe `kernel.Escopo` e traduz só via `Filtro()`), `comercialhttp`. Os handlers legados de Vendas e Analytics passam a obter o escopo via `comercialapp.AutorizadorEscopo` (o `handlers/scope.go` vira um adapter fino que converte `Escopo` em `vendedorScope` até ARQ-05/06).
- **Apaga:** `handlers/cliente_handler.go`, `vendedor_handler.go`, `visita_handler.go`, `oportunidade_handler.go`, `ativo_body.go` (vai para `httpx`); `services/cliente_service.go`, `cnpj.go`, `vendedor_service.go`, `visita_service.go`, `oportunidade_service.go`; `repositories/cliente_*`, `carteira_*`, `vendedor_*`, `visita_*`, `oportunidade_*`, `mysql_errors.go` (vai para `comercialmysql`); os models correspondentes, se os importers não os usarem (hoje não usam).
- **Testes:** `tests/handlers/cliente_*`, `scope_test.go`, `escopo_desligado_cobertura_test.go`, `vendedor_*`, `visita_*`, `oportunidade_*`, `lote4/5/6_http_integration_test.go`, `sec01_bug04_risco01_*`; `tests/services/cliente_*`, `cnpj_validacao_test.go`, `vendedor_*`, `visita_*`, `oportunidade_*`, `bug09_*`, `data_local_*`; `shared/tests/repositories/cliente_*`, `carteira_*`, `vendedor_*`, `visita_*`, `oportunidade_*`, `crud_mutacoes_test.go`.
- **Aceite:** §7.1; nenhum método de repositório recebe `vendedorID int64` de escopo (verificável por grep e pelo teste de arquitetura com a regra "assinatura com `Escopo`").
- **Riscos:** regressão de escopo. Mitigação: as suítes `Escopo*`/`SEC01*` sem diff; o zero value de `Escopo` falha fechado (teste dedicado: repositório com `Escopo{}` devolve erro e não executa SQL).

#### ARQ-05: Vendas

- **Entra:** `vendas` (Pedido + ItemPedido, Pagamento), `vendasapp`, `vendasmysql` (sem `BeginTx` interno; o UoW vem da aplicação), `vendashttp`; `Dinheiro` conforme a decisão P3.
- **Apaga:** `handlers/pedido_handler.go`, `pagamento_handler.go`; `services/pedido_service.go`, `pagamento_service.go`; `repositories/pedido_repository.go`, `pagamento_repository.go`; `models/pedido.go`, `item_pedido.go`, `pagamento.go`; `handlers/scope.go` (se Analytics já não depender; senão, no ARQ-06).
- **Testes:** `tests/handlers/pedido_*`, `pagamento_*`, `escopo_escrita_pedido_pagamento_test.go`; `tests/services/pedido_*`, `pagamento_*`; `shared/tests/repositories/pedido_*`, `pagamento_*`.
- **Aceite:** §7.1; `UpdatePedido` com handler ≤ 40 linhas.
- **Riscos:** arredondamento (P3); TOCTOU pré-existente (V, não piorar): a checagem de posse continua imediatamente antes da escrita, na mesma ordem.

#### ARQ-06: Analytics

- **Entra:** `analytics` (Periodo, JanelaDias), `analyticsapp` (4 queries tipadas), `analyticsmysql` (SQL atual, com período por enum e escopo por `Escopo`), `analyticshttp`.
- **Apaga:** `handlers/dashboard_handler.go`, `services/dashboard_service.go`, `repositories/dashboard_repository.go`; as métricas de cliente em `cliente_repository.go:256-420`, que migraram para `comercialmysql` no ARQ-04 e agora vão para `analyticsmysql`; `handlers/scope.go` (se ainda existir).
- **Testes:** `tests/handlers/dashboard_*`; `tests/services/dashboard_service_test.go`; `shared/tests/repositories/dashboard_*`.
- **Aceite:** §7.1; JSON igual por `JSONEq` contra respostas capturadas dos testes atuais; `fmt.Sprintf` no SQL só com constantes (teste de AST simples: o argumento de `Sprintf` em `analyticsmysql` não pode vir de parâmetro).
- **Riscos:** tipos numéricos no JSON (int vs float) ao trocar `map[string]any` por struct. Mitigação: manter os tipos exatos do map atual em cada campo.

#### ARQ-07: Notificação, importers, CLIs e remoção do legado

- **Entra:** `notificacao`, `notificacaoapp` (notifier), `notificacaosmtp` (SMTP + Noop); `importers/importkit` e os 8 importers como `Spec` (com fachadas das funções exportadas atuais); `tools/resetpassword`, `seedusers`, `testeemail` e `cmd/resetsenhas` sobre `identidadeapp`/`notificacaosmtp`; remoção de `middleware.ComPrincipalLegado`, `handlers/`, `services/` (API), `middleware/`, `SH/services/`, `SH/repositories/`, `SH/models/` (os testes de `shared/tests/models`, 2, são reapontados para os VOs equivalentes).
- **Testes:** `shared/tests/services/email_*`, `mail01_*`, `sec10_tls_test.go`, `sec09_alerta_email_test.go`; `tests/services/sec09_notifier_test.go`; `shared/tests/importers/*` (123); `shared/tests/tools/*`; `tools/testeemail/testeemail_test.go` (§7.2-8).
- **Aceite:** §7.1; `grep -r "shared/models\|shared/repositories\|rotaperfumes-api/handlers" apis` vazio; o teste de arquitetura sem exceções temporárias.
- **Riscos:** os CLIs de banco (`make db-seed` usa `resetpassword`). Mitigação: rodar o `make db-reset` local antes do commit (pedir ao usuário, como no TST-04).

#### ARQ-08: Documentação (🔵 SubBrain)

ADR, revisão deste documento com o que mudou nos lotes, `docs/logs-verbose.md` (níveis do §6), `.claude/agents/backend.md` (estrutura `apis/shared/<ctx>`, `internal/<ctx>http`), Postman (só a exceção X1).

---

## 8. Checklist de invariantes de segurança por lote

Referência: `docs/arq-00-invariantes-seguranca.md`. Marcar no PR de cada lote. "Teste" indica a suíte que tem de continuar verde sem diff de asserção.

### Todos os lotes

- [ ] **R1** contagem e nomes por módulo ≥ linha de base; nenhum `t.Skip` novo; suítes SEC*/TST05*/Escopo*/MAIL01*/SEC10_TLS* sem diff de asserção.
- [ ] **R2** matriz de rotas verde (60 padrões, mesma política).
- [ ] **R3** todo erro de infra vira 500 "erro interno"; nenhum caminho "libera" ou "sem filtro".
- [ ] **R4** nenhum cache de usuário, escopo ou vendedor entre requests; nenhum decorator de cache.
- [ ] Nenhum GAP corrigido de carona; nenhum GAP piorado.
- [ ] vlog: sem segredo, sem CNPJ, e-mail mascarado; sem log por iteração (F-09); tabela do §6.3 respeitada; `TestARQ_VlogSemConteudoSensivel` verde (O7).
- [ ] Allowlists e ASTs do §4.3 verdes nos dois módulos (`NovoPrincipal` só em `httpx`; `Escopo*` só em `comercialapp`; `NovoErro` literal em `var`; sem `errors.Join` de domínio; chaves legadas só no middleware legado) (O4, O5, O6, R-c, R-g).
- [ ] Casos de uso que recebem `Principal` chamam `p.Valido()` e falham fechado; nenhum "Principal de sistema"; CLIs não recebem Principal (O5).
- [ ] Wrap de infra só com o nome da operação; erro 1062 traduzido no adapter sem embrulhar o driver (O6-iv).

### ARQ-01

- [ ] **F-01** ordem efetiva `Logging(CORS(SecurityHeaders(mux → Exigir(policy)(Trace(handler)))))`, com a autenticação **por fora** do `Trace`: 401/403 não emitem o rótulo vlog do handler (O9). Todo `OPTIONS` → 204 no CORS antes do mux, com ou sem `Origin` e com origem fora da allowlist sem ACAO (O3). 401 e 405 com `X-Content-Type-Options`. Método errado em rota protegida → 405 sem consultar o checker. `HEAD /api/auth/me` sem token → 401 (R-b). Testes da tabela do §5.8.
- [ ] **F-02** allowlist exata, `Allow-Credentials: true`, `Vary: Origin` (`tests/middleware/cors_middleware_test.go`).
- [ ] **F-03** nosniff, DENY, Referrer-Policy, CSP, HSTS exceto localhost (`security_headers_middleware_test.go`).
- [ ] **F-04** HTTP 15/15/60 s, shutdown 10 s, pool 25/5/5 min, ping 5 s (os valores vão para o bootstrap sem mudar).
- [ ] **F-05** envelope único; mapper por `switch` sobre `Kind` com `default` → 500 "erro interno" + log (nunca índice de array); `TestErrMap_TodosOsKinds`; mensagem do sentinela, nunca do wrap; `writeError` sem concatenação; AST: `kernel.NovoErro` só em `var` de pacote, com Kind constante e mensagem literal (O6). `ErrEscopoInvalido` loga `[escopo] invalido <tag>` (R-e).
- [ ] **F-06** `config.Load` intacto (segredos obrigatórios, mensagens sem valor).
- [ ] **F-07** `Transactor` não deduz nada de `RowsAffected`; condições continuam no WHERE.
- [ ] **F-08** DSN só via `maskDSN`; o bootstrap não loga `cfg`.
- [ ] **F-09** vlog desligado por padrão; `withLogging` sem query string.
- [ ] **F-10** `kernel.NovaPagina`: default 20, máximo 100, page ≥ 1 (teste de tabela).
- [ ] Policy (O2): `Router.Handle` dá panic para política fora de {Public, Authenticated, AdminOnly}, método fora de {GET, POST, PUT, PATCH, DELETE}, padrão sem `/` inicial ou com espaço, handler nil e rótulo vazio; `Autenticador.Exigir` com `default: panic`; testes de panic.
- [ ] Exceção X1 documentada e testada; `/api/senha-historico/{user_id...}` AdminOnly, com parse estrito (`ParseInt`, `> 0`, sem `/`; teste com `..%2F`) (R-f).
- [ ] Principal na transição (O4): o middleware legado é o único que grava as chaves antigas; `derivarPrincipal` é não exportado e só roda depois da aprovação do legado, usando o role do banco; rota Public sem Principal nem chaves; `PrincipalFrom` devolve `(p, ok)` e `!ok` em rota protegida → 500.
- [ ] Role desconhecido no banco = restrito, com log `[auth] role desconhecido` (R-a); nenhum 401/500 novo.

### ARQ-02

- [ ] **C-01** produto: leitura Authenticated, escrita AdminOnly; estoque todo AdminOnly.
- [ ] **C-02** enum de ordenação; parse no HTTP com fallback; `switch` coluna no adapter; nenhuma assinatura recebe string livre de coluna (`sort_test.go`, injection de `order_by`).
- [ ] **C-03** LIKE por placeholder (o GAP de `%`/`_` continua GAP).

### ARQ-03

- [ ] **I-01** só HS256; exp verificado + checagem defensiva; iat/nbf/exp emitidos.
- [ ] **I-02** Bearer tem precedência; header malformado em rota protegida → 401 sem fallback para cookie; cookie só sem header.
- [ ] **I-03** consulta ao banco a cada request protegido; inexistente ou inativo → 401 com a mesma mensagem; erro → 500 sem chamar o handler; role do banco; JWT inválido não consulta o banco; rota pública não consulta; checker nil dá panic na montagem.
- [ ] **I-04** `iat <= corte` → 401; com corte, token sem iat → 401; corte gravado por UpdatePasswordHash, SetAtivo(false), InativarByVendedorID e InvalidarSessoes; Rehash não mexe no corte.
- [ ] **I-05** AuthZ pela role do banco; handlers de usuário repetem a checagem admin; `EhAdmin()` só para `"admin"` exato; qualquer outro valor (incluindo desconhecido) = restrito (R-a).
- [ ] **I-06** refresh de 32 B crypto/rand, SHA-256 no banco, TTL 7 d; rotação no `Transactor` (revogação condicional + insert); concorrência → 401 sem contar no rate limit; expiração antes de revogação; graça de 30 s → 401 sem contar; refresh reconsulta ativo e role.
- [ ] **I-07** motivo `rotacao` fora da janela → `RegistrarReuso` atômico; corte → revogação em massa + InvalidarSessoes + evento; erro ao marcar → corta; corte incompleto → DesfazerReuso; HTTP idêntico; `RevogadoRecente` antes de `Revogado` (enum de estado). **Fronteira exatamente igual à atual** (O8-iv): nenhuma tx envolvendo marcação + corte cujo rollback desfaça a marcação sem cortar.
- [ ] Troca de senha **sem** `Transactor` no ARQ-03, com a mesma sequência de chamadas de hoje (O8-iii; atomicidade fica em card próprio).
- [ ] `WithTx`: a `fn` usa só os repositórios de `R`; nenhum efeito externo dentro da tx; isolamento padrão (REPEATABLE READ) e cancelamento pelo `ctx` (O8-ii, R-d).
- [ ] **I-08** notifier assíncrono, dedup 30 min, 20/h, 4 concorrentes, 30 s; admins saneados de CR/LF (o assinante continua o mesmo objeto, singleton).
- [ ] **I-09** logout idempotente + limpa cookies; troca de senha: corte + revogação + limpa cookies; reset admin: revogação + corte; inativação: revogação.
- [ ] **I-10** cleanup: retenção ≥ 24 h, lotes de 1000, cancelável, para antes de `pool.Close()`.
- [ ] **I-11** Argon2id sobre HMAC(pepper); salt de 16 B; tempo constante; nunca sem pepper; bcrypt só verifica e marca rehash; rehash condicional best-effort; hash descartável para inexistente.
- [ ] **I-12** política de 8 runes, 72 bytes e 3/4 classes; ordem do reset: captcha → limite → senha atual → nova==atual → força → histórico (sem short-circuit) → grava → revoga; `ErrSenhaFraca` com texto fixo; `Senha`/`HashSenha` sem JSON e com `String()`="***"; DTO de usuário sem campo de hash.
- [ ] **I-13** Turnstile em login e reset; fail-closed → 400; antes do rate limit; token e secret nunca logados.
- [ ] **I-14** chaves `ip:`, `acct:`, `refresh-ip:`, `reset:` com limites idênticos; 429 + `Retry-After ≥ 1s`; sucesso zera; **3 limitadores singleton no bootstrap**; IP por XFF só com `TRUST_PROXY_HEADERS`.
- [ ] **I-15** cookies `access_token` (HttpOnly, Strict, MaxAge=TTL) e `refresh_token` (HttpOnly, Lax, 7 d); tokens nunca no corpo.
- [ ] **I-16** login: inexistente e senha errada → mesmo 401; inexistente roda hash descartável.
- [ ] `Principal` só criado pelo `httpx.Autenticador`; chave de contexto de tipo não exportado; o `httpx` não exporta setter (os testes obtêm Principal pelo `Autenticador` com checker fake); `JWTMiddleware` sem checker e `RequireAdmin()` removidos com os testes reapontados; `ComPrincipalLegado`, se existir, só chamado por `internal/httpx/auth.go` (O4).
- [ ] TST05: `[auth_handler.go] [AuthHandler.Login]` e `[AuthHandler.ResetPassword]` presentes; nenhum segredo.

### ARQ-04

- [ ] **E-01** escopo resolvido a cada request (sem cache): admin → Irrestrito; outro role → restrito; sem vínculo ou órfão → SemAcesso; desligado → 403 com mensagem exata; erro → 500.
- [ ] **E-02 / O1** `Escopo` sem zero value útil; repositório recebe `Escopo`, nunca `int64`; tradução **só** por `Escopo.Filtro()` (switch exaustivo: sem filtro só para Irrestrito; SemAcesso e inválido → erro antes do SQL); nenhum adapter deduz "global" de `ok == false`. **Teste por repositório com escopo**: `EscopoSemAcesso()` e `Escopo{}` → erro ou vazio **sem nenhuma expectativa no sqlmock**. Irrestrito só a partir de admin; construtores `Escopo*` só em `comercialapp` (O5).
- [ ] **I-09 desligamento** (O8-i): `DesligarVendedor` numa tx única (`data_desligamento` via COALESCE + inativação dos usuários gravando o corte `tokens_validos_desde` + revogação dos refresh tokens); falha em qualquer passo → rollback total; testes com `ExpectRollback` para falha em cada um dos três passos.
- [ ] **P4/BUG-14:** `VincularCliente` mantém a sequência atual, sem tx (nenhuma mudança de comportamento no ARQ-04).
- [ ] **E-03** restrito: filtro sobrescrito pelo escopo ignorando a query; SemAcesso → 200 vazio sem tocar no banco.
- [ ] **E-04** fora da carteira → 404 com a mesma mensagem de inexistente (nunca `KindProibido`).
- [ ] **E-05** create força VendedorID do escopo e exige cliente na carteira; update/delete checam a posse atual; update/toggle de cliente autorizam **antes** do decode; normal cria cliente + vínculo em tx; sem vendedor → 403; comando sem VendedorID do payload para restrito.
- [ ] **E-06** normal lista só o próprio vendedor; clientes de outro vendedor → 404; gestão AdminOnly.
- [ ] **E-07** CNPJ duplicado → 409 genérico sem id nem vendedor; CNPJ nunca em log.

### ARQ-05

- [ ] **V-01** escopo de pedido igual a E-03..E-05; exclusão 409 com pagamentos ou Faturado.
- [ ] **V-02** escopo de pagamento via `pedido.vendedor_id`; o comando de update não tem `PedidoID`; pedido alheio → 404; falha ao buscar pedido em Get/Delete → 404.
- [ ] **V-03** `valor_bruto`/`valor_total` calculados no domínio (`NovoPedido`), nunca aceitos do payload; `Dinheiro` com `float64` encapsulado e a mesma fórmula de hoje (P3; centavos no ARQ-09).
- [ ] **O1** em `vendasmysql`: pedidos e pagamentos com `EscopoSemAcesso()` e `Escopo{}` → erro ou vazio sem nenhuma expectativa no sqlmock.
- [ ] TOCTOU: a ordem "checar posse → escrever" não fica mais longa.

### ARQ-06

- [ ] **D-01** desligado, inexistente ou sem vendedor → 200 zerado com `vendedor_desligado`; nunca "zerar o VendedorID e seguir".
- [ ] **D-02** `Sprintf` só com constantes; `Periodo` tipado antes do SQL; valores por placeholder.
- [ ] **D-03** `JanelaDias` de 1 a 365.
- [ ] `Escopo` obrigatório (o 0 = global some daqui também); a tradução via `Filtro()` e o teste O1 (SemAcesso e `Escopo{}` sem SQL) valem para as 4 consultas do dashboard. O caso de uso responde zerado com `vendedor_desligado` **antes** de chamar o repositório (D-01).

### ARQ-07

- [ ] **M-01** SMTP: TLS implícito ou STARTTLS (aborta se não anunciado); AUTH só sobre TLS; verificação ligada; TLS ≥ 1.2; config inválida → `log.Fatal` no bootstrap; destinatário sem CR/LF.
- [ ] **M-02** senha gerada nunca em resposta nem log; Noop loga e-mail mascarado.
- [ ] **M-03** `LoadSemCredenciaisDB` só no dry-run; `resetsenhas` simula por padrão (`-executar`), grava histórico e revoga tokens via `identidadeapp`.
- [ ] **M-04** importers com placeholders; CNPJ nunca logado; `importkit` sem log por linha.
- [ ] Legado removido; matriz R2 verde contra o router final; nenhum adaptador `// TODO(ARQ-0X)` restante.

---

## 9. Convenções de código (resumo para os lotes)

- **Construtores:** `Novo<Tipo>(deps…)` com todas as dependências obrigatórias; `panic` na montagem se alguma for nil; nada de setters.
- **Casos de uso:** um struct por contexto (`catalogoapp.Casos`), com um método por caso de uso: `func (c *Casos) CriarProduto(ctx, p kernel.Principal, cmd CriarProduto) (*catalogo.Produto, error)`. Primeira linha: `defer vlog.Trace(...)(&err)`. Até ~40 linhas por método; acima disso, extrair os passos com nomes de negócio.
- **Handlers:** só decodificar, chamar o caso de uso e responder. Até ~30 linhas. Erros sempre via `httpx.ResponderErro(w, tag, err)`.
- **Erros:**
  - sentinela `var ErrX = kernel.NovoErro(kernel.KindY, "mensagem pública")` no domínio, com Kind constante e mensagem literal (§4.3);
  - a infra embrulha com `fmt.Errorf("catalogomysql: listar produtos: %w", err)`: o texto do wrap leva **só o nome da operação**, nunca valores (CNPJ, e-mail, busca, ids de terceiros) (O6-iv);
  - erro de chave duplicada do MySQL (1062, `Duplicate entry '<valor>'`) é traduzido **no adapter** para o sentinela, devolvido sem embrulhar o erro do driver;
  - `errors.Join` de infra com sentinela é proibido (R-c);
  - ninguém compara strings de erro.
- **Contexto:** `ctx` é sempre o 1º parâmetro; o `Principal` vai **explícito** como parâmetro para a aplicação (nunca lido do `ctx` fora de `httpx`). No handler: `p, ok := httpx.PrincipalFrom(r)`; em rota protegida, `!ok` → 500 "erro interno" (O5).
- **SQL:** constantes no adapter; placeholders para valores; identificadores só de `switch` sobre enum.
- **Arquivos:** um agregado por arquivo no domínio; `portas.go` em cada `<ctx>app`; `dto.go` e `routes.go` em cada `<ctx>http`.
- **Nomes:** em português para conceitos de negócio (já é o padrão do projeto); a sigla em inglês só para termos técnicos estabelecidos (`Querier`, `Transactor`, `Policy`).
