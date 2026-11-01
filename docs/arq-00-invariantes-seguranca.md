> **Origem:** 🟣 SecBrain, card ARQ-00, Lote 21 (2026-10-13). Versionado pelo 🔵 SubBrain.
>
> **Referência obrigatória** dos cards ARQ-01 a ARQ-07 (reestruturação DDD da API, branch `refactor/ddd`). Cada lote de código deve ser conferido contra as regras globais (R1 a R4) e contra as invariantes da seção correspondente. Os itens marcados **[GAP]** viraram os cards SEC-14 a SEC-25 em `tarefas/afazer.md` e ficam fora do escopo do Lote 21. O GAP 12 (teste de matriz de rotas) é executado pelo 🔴 TestBrain dentro do próprio ARQ-00.
>
> **Nota do ARQ-08 (2026-10-16, 🔵 SubBrain):** este documento é o registro da auditoria e foi mantido como estava. Os caminhos e linhas de código de produção citados (`API/handlers/...`, `API/middleware/...`, `API/services/...`, `API/routes/routes.go`, `SH/services/...`, `SH/repositories/...`, `SH/models/...`) são do código **anterior** ao Lote 21; esses pacotes foram removidos no ARQ-07. Para achar o ponto equivalente na estrutura atual, use a seção 10 ("Estado final") de `docs/arquitetura-ddd.md`. As pastas de teste (`AT/...`, `ST/...`) continuam com os mesmos nomes.

---

# 🟣 SecBrain — ARQ-00: invariantes de segurança e riscos da refatoração DDD (2026-10-13)

Raízes: `API` = apis/rotaperfumes-api, `SH` = apis/shared, `AT` = API/tests, `ST` = SH/tests.
**[GAP]** = falha pré-existente; NÃO corrigir de carona na refatoração (vira card próprio), mas também não piorar.

## 0. Regras globais (ARQ-01..07)
- **R1 Paridade de testes.** Testes são pacotes `*_test` externos que importam handlers/middleware/services/repositories pelo caminho. Portar os testes no mesmo lote da mudança. Proibido apagar teste ou usar `t.Skip`. A contagem de `go test -list . ./...` por suíte não pode diminuir. Testes `SEC*`, `TST05*`, `Escopo*`, `MAIL01*`, `SEC10_TLS*` passam SEM alterar asserções.
- **R2 Teste de matriz de rotas ANTES do ARQ-01.** Tabela com os ~60 pares (método, rota) de `API/routes/routes.go:80-464` e a política (pública/autenticado/admin): sem token → 401; usuário normal em rota admin → 403; rota pública não consulta o checker.
- **R3 Fail-closed.** Erro de infra → 500 "erro interno"; nunca "liberar"/"sem filtro".
- **R4 Sem cache de usuário/escopo/vendedor entre requests** (SEC-06/SEC-08). Nada de decorator de cache em repositório.

## ARQ-01 Fundação
- F-01 Ordem: `withLogging(CORS(SecurityHeaders(mux → JWT por rota → handler)))` (`API/cmd/server/main.go:119`, `routes.go:466-476`). CORS mais externo; OPTIONS → 204 antes de auth (`cors_middleware.go:61-64`). Testes: `AT/middleware/cors_middleware_test.go`, `security_headers_middleware_test.go`.
- F-02 CORS allowlist exata; reflete só origem permitida; `Allow-Credentials: true` e `Vary: Origin` (`cors_middleware.go:16-69`). [GAP] `"*"` reflete qualquer origem com credentials.
  - **Resolvido no Lote 27 (SEC-17, GAP 4, 2026-10-31):** `CORS_ALLOWED_ORIGINS` com `*` ou `null` (sem diferenciar maiúsculas, com ou sem espaços) impede a API de subir (`ErrCORSOrigemInvalida`, `SH/config/config.go`); o middleware (`API/internal/httpx/cors.go`) não tem mais curinga e descarta `*`/`null` com um log único na montagem. Resíduo aberto: origens de dev (`localhost:3000`) sempre na allowlist, inclusive em produção (card SEC-36).
- F-03 Headers: nosniff, X-Frame-Options DENY, Referrer-Policy, CSP `default-src 'none'; frame-ancestors 'none'`; HSTS exceto host exato localhost/loopback (`security_headers_middleware.go:22-56`).
- F-04 Timeouts: HTTP Read/Write 15s, Idle 60s, shutdown 10s (`main.go:117-144`); Turnstile 4s; SMTP 10s/30s; DB ping 5s, pool 25/5, lifetime 5m (`SH/db/db.go:32-41`). [GAP] sem MaxBytesReader.
  - **Resolvido no Lote 27 (SEC-14, GAP 1, 2026-10-31):** limite de corpo por rota no router (`API/internal/httpx/router.go`, `MaxCorpoPadrao` = 256 KiB; só o upload de imagem usa `MaxCorpoUploadBytes`), 413 tipado (`kernel.KindMuitoGrande`), com 401/403 antes do 413; `NovoServidor` (`API/internal/bootstrap/server.go`) com `ReadHeaderTimeout` 5s e `MaxHeaderBytes` 64 KiB; pedido com no máximo 200 itens (`vendas.MaxItensPedido`, 400).
- F-05 Envelope de erro: `writeJSON` (`API/handlers/response.go:28-41`), `writeError` do middleware (`auth_middleware.go:280-285`). Erro reconhecido → mensagem fixa; desconhecido → log + 500 "erro interno". `writeError` concatena sem escape (seguro só porque mensagens são constantes).
- F-06 Segredos: JWT_SECRET obrigatório, PASSWORD_PEPPER ≥32 bytes, DB_USUARIO/DB_SENHA obrigatórios, faixas de Argon2/refresh validadas (`SH/config/config.go`); mensagens nunca incluem valores.
  - **Resolvido no Lote 24 (SEC-18, 2026-10-29):** `JWT_SECRET` exige ≥32 bytes e o placeholder é recusado (erro `config: JWT_SECRET é obrigatório (mínimo de 32 bytes)`); `JWT_ISSUER` aparado, vazio → `rotaperfumes`; `JWT_TTL <= 0` → erro. Nova env `COOKIE_SECURE` (SEC-21, default true).
- F-07 DSN `clientFoundRows=true`; condição no WHERE, nunca deduzir por RowsAffected (Revoke `WHERE revoked_at IS NULL` `refresh_token_repository.go:105`; Rehash `AND password_hash = ?` `usuario_repository.go:297`).
- F-08 DSN nunca logado (`maskDSN`, `SH/db/db.go:51-84`). [GAP] senha com `@` vaza parcialmente.
  - **Resolvido no Lote 27 (SEC-15, GAP 2, 2026-10-31):** o corte segue o parse do go-sql-driver (último `/`, último `@` antes dele, primeiro `:`); se a senha ainda aparecer no texto, o log sai só `****` (`SH/db/db.go`).
- F-09 vlog desligado por padrão (VERBOSE/LOG_LEVEL=debug); regras LOG-02 em `SH/vlog/vlog.go:7-15`; e-mail via `MaskEmail`; `withLogging` não loga query string (`main.go:210`). Testes: `AT/handlers/tst05_vlog_runtime_auth_test.go`.
- F-10 Paginação: limit default 20, max 100; page ≥1 (`API/services/usuario_service.go:403-424`); dashboard `dias` 1..365. [GAP] page sem teto.
  - **Resolvido no Lote 27 (SEC-16, GAP 3, 2026-10-31):** clamp em `kernel.PaginaMaxima` = 10.000 (`SH/kernel/pagina.go`; maior OFFSET 999.900); erro de faixa do `Atoi` em `API/internal/httpx/query.go` conta como ausente; `page` gigante, que dava 500, agora dá 200 com `pagination.page` = 10000.
- Riscos: builder de cadeia com ordem invertida (exigir teste: OPTIONS sem token em rota protegida → 204 com ACAO; 401 contém X-Content-Type-Options). Trocar `jwt(protected, requireAdmin)` booleanos por política tipada `Public | Authenticated | AdminOnly` obrigatória, sem default público. Prefixo `/api/senha-historico/` continua admin. Mapper de erros: só sentinelas via `errors.Is`; nunca `err.Error()` de erro embrulhado; `ValidarForcaSenha` (`auth_handler.go:717`) vira erro de domínio com texto fixo; não usar `map[error]` para decidir (ordem aleatória).
- Onde mora: interfaces/http (CORS, headers, logging, AuthN/AuthZ, envelope, error mapping); infrastructure (config, DSN, pool, maskDSN); `Pagina{page,limit}` VO com clamp no shared kernel.

## ARQ-02 Catálogo
- C-01 RBAC: produto leitura comum, escrita admin; estoque tudo admin (`routes.go:283-453`).
- C-02 Whitelist order_by (`SH/repositories/sort.go:29-46`), fallback para default; ASC/DESC. Produto `produto_repository.go:60`; Estoque duas whitelists `estoque_repository.go:77,83`. Teste `ST/repositories/sort_test.go`.
  - **Resolvido no Lote 27 (SEC-24, GAP 11, 2026-10-31):** `SH/tests/repositories/sec24_ordenacao_injecao_test.go` cobre os enums de ordenação de todas as entidades (6 entidades × 54 cenários + completude dos enums) e `API/tests/handlers/sec24_order_by_http_test.go` cobre as 9 listagens HTTP; nenhuma injeção encontrada.
- C-03 LIKE parametrizado. [GAP baixo] `%`/`_` não escapados.
  - **Resolvido no Lote 27 (SEC-25, 2026-10-31):** helper `db.TermoContem` (`SH/db/like.go`, escape `!`) e `LIKE ? ESCAPE '!'` em produtos, clientes, visitas, oportunidades e pedidos; `q` limitado a 100 caracteres (`httpx.BuscaDaQuery`, 400). Índice FULLTEXT ficou fora.
- Recomendação: enum tipado de ordenação no domínio/application; parse em interfaces/http; repositório mapeia enum→coluna via switch; assinatura nunca recebe string livre.

## ARQ-03 Identidade e Acesso
- I-01 JWT só HS256 (`SH/services/auth_service.go:99`), exp verificado + checagem defensiva; iat/nbf/exp emitidos.
  - **Resolvido no Lote 24 (SEC-18, GAP 5, 2026-10-29):** a validação exige `iss` (= `JWT_ISSUER`), `exp` e HS256; `iat` no futuro é recusado (`SH/identidade/identidadecrypto/jwt.go`).
- I-02 Bearer tem precedência; header malformado em rota protegida → 401 sem fallback para cookie; cookie `access_token` só sem header (`auth_middleware.go:72-93`).
- I-03 SEC-06: usuário consultado no banco a cada request protegido, sem cache; inexistente/inativo → 401 mesma mensagem; erro → 500 sem chamar handler; role do banco prevalece; JWT inválido → 401 sem consultar banco; rota pública não consulta; checker nil → panic na montagem.
- I-04 SEC-08: `iat <= tokens_validos_desde` → 401; com corte, token sem iat → 401. Corte gravado por UpdatePasswordHash, SetAtivo(false), InativarByVendedorID, InvalidarSessoes; RehashPassword NÃO mexe no corte.
- I-05 RBAC usa role do banco; handlers de usuário repetem checagem (defesa em profundidade); roles `admin|normal`; escopo trata role != "admin" como restrito (`scope.go:69`).
  - **GAP 10 resolvido no Lote 28 (SEC-23, 2026-10-31):** o `Autenticador` (`API/internal/httpx/auth.go`) impõe `usuarios.deve_trocar_senha` (lido na mesma query do SEC-06, `StatusAcesso.DeveTrocarSenha`): ordem 401/500 (token, inexistente, inativo, corte) → 403 "troca de senha obrigatória" → 403 de admin; vale para todos os papéis. Só `POST /api/auth/reset-password` e `GET /api/auth/me` têm `Rota.LiberaTrocaSenhaPendente` (`bootstrap/rotas.go`; `Router.Handle` dá panic se o campo for usado numa rota Public). O `/me` devolve `trocar_senha`. Teste de matriz: `API/tests/routes/sec23_troca_senha_matriz_test.go`.
- I-06 Refresh: 32 bytes crypto/rand, banco guarda SHA-256, TTL 7d; rotação atômica (UPDATE condicional + insert na mesma tx); concorrência → 401; expiração checada antes de revogação; janela de graça 30s → 401 sem contar no rate limit; refresh reconsulta ativo e usa role do banco.
- I-07 Reuso SEC-07/12: motivo `rotacao` fora da janela → `RegistrarReuso` atômico; cortar → revogação em massa + InvalidarSessoes + alerta; erro ao marcar → corta (fail-closed); corte incompleto → DesfazerReuso (`auth_handler.go:528-607`); resposta HTTP idêntica; checar `ErrRefreshTokenRevokedRecently` ANTES de `ErrRefreshTokenRevoked`.
- I-08 Alerta SEC-09: assíncrono, dedup 30min/usuário, teto 20/h, 4 concorrentes, timeout 30s; lista de admins saneada CR/LF.
- I-09 Revogações: logout idempotente + limpa cookies; troca de senha: corte + revogação total + limpa cookies; reset admin: revogação total + corte; inativação: revogação total; desligamento de vendedor: tx única (data_desligamento + inativa usuários + revoga tokens) (`vendedor_service.go:343-389`).
- I-10 Cleanup CHORE-02: retenção mínima 24h, lotes de 1000, cancelável, parada antes de fechar o pool.
- I-11 Hash SEC-13: Argon2id sobre HMAC-SHA256(pepper, senha), salt 16B, comparação tempo constante, nunca sem pepper; bcrypt legado só verifica e marca rehash; rehash best-effort condicional; hash descartável para e-mail inexistente.
- I-12 Política: min 8 runes, max 72 bytes, 3 de 4 classes. Ordem do reset: captcha → rate limit → senha atual (conta) → nova==atual → força (não conta) → histórico (atual + 2 últimas, SEM short-circuit) → grava histórico/senha → revogação. `password_hash` `json:"-"`; `SenhaHashAnterior` sem tag JSON (só não vaza porque o handler monta resposta à mão).
- I-13 Turnstile em login e reset; fail-closed → 400; antes do rate limit; token/secret nunca logados.
  - **Resolvido no Lote 28 (SEC-19, GAP 6, 2026-10-31):** o adapter (`SH/adapters/turnstile/turnstile.go`) confere, depois de `success`, o `hostname` (sem diferenciar maiúsculas, contra `cfg.TurnstileHostnames`: env `TURNSTILE_HOSTNAMES` ou, se vazia, os hosts de `CORS_ALLOWED_ORIGINS`; lista vazia recusa) e a `action` exata (`identidade.AcaoCaptchaLogin` = `login`, `AcaoCaptchaResetSenha` = `reset-senha`, em `SH/identidade/captcha.go`); os erros embrulham `ErrCaptchaInvalido` sem os valores recebidos. Contrato inalterado (400 "captcha inválido ou ausente", antes do rate limit). Env inválida impede a subida (`ErrTurnstileHostnames`, sem o valor). Com secret de teste da Cloudflare, hostname/action não são conferidos (aviso no startup). Resíduos abertos: a secret de teste é aceita em produção (card SEC-37) e o log de recusa não diz o motivo (card LOG-03).
- I-14 Rate limit: login `ip:<ip>` e `acct:<ip>|<email>` 5/1min bloqueio 5min; refresh `refresh-ip:<ip>` 10/1min bloqueio 5min; reset `reset:<ip>|<uid>` 10/5min bloqueio 10min; 429 + Retry-After ≥1s; sucesso zera. Em memória, uma instância por processo. IP: XFF/X-Real-IP só com TRUST_PROXY_HEADERS=true; senão RemoteAddr sem porta.
  - **Resolvido no Lote 24 (SEC-20, GAP 7, 2026-10-29):** com TRUST_PROXY_HEADERS=true, o IP é o item **mais à direita** do XFF, validado e normalizado; inválido → RemoteAddr. O `X-Real-IP` não é mais lido (`API/internal/httpx/clientip.go`). Sem env nova.
- I-15 Cookies: access_token HttpOnly SameSite=Strict MaxAge=TTL; refresh_token HttpOnly SameSite=Lax 7d; tokens nunca no corpo JSON.
  - **Resolvido no Lote 24 (SEC-21, GAP 8, 2026-10-29):** `Secure` vem da env `COOKIE_SECURE` (default true; `make dev-api`/`run-api` usam false; `deploy/setup-servidor.sh` força true), não mais de `r.Host`; a limpeza dos cookies usa os mesmos atributos da criação (`API/internal/httpx/cookies.go`).
- I-16 Anti-enumeração no login: inexistente e senha errada → mesmo 401, inexistente roda hash descartável.
  - **Resolvido no Lote 24 (SEC-22, GAP 9, 2026-10-29):** inativo também → 401 "credenciais inválidas", com o mesmo corpo e o mesmo custo de hash; `ErrUsuarioInativo` removido. Refresh de inativo revoga o token (motivo `inativacao`) e responde 401 "refresh token revogado". O 401 "usuário inativo" do middleware (I-03) continua. Resíduo aberto: hash vazio/malformado ainda retorna sem rodar o Argon2 (card SEC-32).
- Riscos: cache em Authenticate quebra SEC-06/08; AuthZ deve usar role do UserStatus do banco; `JWTMiddleware` sem checker e `RequireAdmin()` só usados em teste (isolar/remover no ARQ-07); chaves de contexto de tipo não exportado → `Principal{UserID, Role}` setado só pelo middleware; rate limiters e notifier são singletons na composition root (não construir por request); UoW do refresh: revogação+insert na mesma tx; comparação de histórico sem short-circuit (não usar slices.ContainsFunc/any/Specification com early return); VO Senha/Token com `String()` = "***" e sem serialização JSON.
- Onde mora: domain/identidade (Usuario, Role, PasswordPolicy, regra últimas N senhas, RefreshToken com regras); application (casos de uso Login/Refresh/Logout/TrocarSenha/AdminReset/Inativar, ordem captcha→limite→credencial, UoW, revogações, alertas); infrastructure (Argon2 e JWT via portas PasswordHasher/TokenService, Turnstile, rate limiter, MariaDB via driver go-sql-driver/mysql, SMTP); interfaces/http (extração token header/cookie, política por rota, Set-Cookie, ClientIPResolver, 401/403/429).

## ARQ-04 Comercial
- E-01 Escopo resolvido a cada request (`API/handlers/scope.go:26-155`): admin irrestrito; outro role restrito; sem vínculo/órfão → VendedorID=0 → SemAcesso; desligado → 403 "acesso bloqueado: vendedor desligado"; erro → 500.
- E-02 SENTINELA PERIGOSA: `VendedorID <= 0` = sem filtro (global) em cliente/pedido/pagamento/oportunidade/visita/dashboard repositories. Handler sempre barra SemAcesso antes.
- E-03 Listagens: restrito → filtro sobrescrito com scope.VendedorID ignorando query; SemAcesso → 200 lista vazia.
- E-04 Detalhe fora da carteira → 404 com a mesma mensagem de inexistente.
- E-05 Escrita: create força VendedorID do escopo e exige cliente na carteira ativa (`clienteNaCarteiraDoVendedor`, `scope.go:164`); update/delete checam posse do registro atual; update/toggle de cliente chamam `autorizarEscritaCliente` ANTES de decodificar o body; usuário normal cria cliente + vínculo em tx única; sem vendedor → 403.
- E-06 Vendedor: normal lista só o próprio; clientes de outro vendedor → 404; gestão admin.
- E-07 CNPJ duplicado → 409 genérico sem expor id/vendedor.
- Riscos: Specification de escopo com zero value. Usar tipo soma `Escopo{Irrestrito | Vendedor(id>0) | SemAcesso}` sem zero value útil; Irrestrito só a partir de Principal admin; SemAcesso responde vazio sem tocar o banco; restrito com id<=0 → erro; repositório recebe `Escopo`, nunca int64. Todo caso de uso recebe `Principal`. "Não é seu" = mesmo erro de "não existe" (nunca ErrForbidden). Comando não carrega VendedorID do payload para restrito. Manter "escopo antes do decode" em cliente.
- Onde mora: domain/comercial (Carteira, Vendedor desligado, Escopo, Specification puros); application (AutorizadorEscopo resolvendo por chamada via portas); infrastructure (Escopo → SQL com placeholders).

## ARQ-05 Vendas
- V-01 Escopo de pedido segue E-03..E-05; exclusão 409 com pagamentos ou status Faturado.
- V-02 Escopo de pagamento herdado de pedido.vendedor_id; `pedido_id` não editável no update; pedido alheio → 404; falha ao buscar vendedor do pedido em Get/Delete → 404 (não 500).
- V-03 valor_bruto/valor_total calculados no backend.
- Riscos: comando AtualizarPagamento não aceita PedidoID; manter checagem de posse via pedido; TOCTOU pré-existente (não piorar).
  - **TOCTOU de pedido e pagamento fechado nos Lotes 27 e 28:** pedido (PUT/DELETE) sob lock no SEC-26 (Lote 27); `CriarPagamento` e `ExcluirPagamento` em tx com lock pedido → pagamento e `DELETE` condicional a "não quitado" no SEC-35 (Lote 28, `TxPedido.CriarPagamento`/`ExcluirPagamento` em `SH/vendas/vendasapp/uow.go`); FK 1452 na criação → 404 "pedido não encontrado" (`db.ViolacaoFK`, `SH/db/fk.go`). V-02 mantido: erro no lock do pedido na exclusão restrita → 404 (resíduo: erro de infraestrutura escondido como 404, card SEC-38).

## ARQ-06 Dashboard
- D-01 Escopo próprio (`resolverVendedorScopeBase`): desligado/inexistente/sem vendedor → 200 zerado com `vendedor_desligado`; nunca zerar VendedorID e seguir.
- D-02 `fmt.Sprintf` no SQL só com constantes; período de enum validado; valores por placeholder.
- D-03 dias 1..365.
- Riscos: obrigatório usar tipo Escopo (0 = global aqui); período tipado antes do builder de SQL.

## ARQ-07 SMTP, importers, CLIs, legado
- M-01 SMTP: TLS implícito/STARTTLS (aborta se não anunciado); AUTH obrigatório só sobre TLS; InsecureSkipVerify=false, TLS ≥1.2; config inválida → log.Fatal; destinatário validado CR/LF.
- M-02 Senha gerada nunca em resposta/log; noop loga e-mail mascarado.
- M-03 CLIs: LoadSemCredenciaisDB só dry-run; resetsenhas simula por padrão (-executar), grava histórico e revoga tokens.
- M-04 Importers: placeholders; CNPJ nunca logado.
- Legado: remover/isolar JWTMiddleware sem checker e RequireAdmin(); teste R2 passa contra router novo.

## GAPs (cards próprios, fora do escopo)
1 MaxBytesReader/ReadHeaderTimeout · 2 maskDSN com `@` · 3 page sem teto · 4 `*` no CORS com credentials · 5 JWT sem issuer/exp obrigatório; JWT_SECRET sem tamanho mínimo · 6 Turnstile sem hostname/action · 7 XFF mais à esquerda · 8 cookie Secure por r.Host e clear sem atributos · 9 login revela usuário inativo (timing) · 10 deve_trocar_senha não imposto · 11 testes de injection de order_by só Produto/SenhaHistorico · 12 teste de matriz de rotas (pré-requisito ARQ-01, será feito no Lote 21).

> Mapeamento GAP → card (🔵 SubBrain): 1 → SEC-14 (**resolvido no Lote 27**) · 2 → SEC-15 (**resolvido no Lote 27**) · 3 → SEC-16 (**resolvido no Lote 27**) · 4 → SEC-17 (**resolvido no Lote 27**) · 5 → SEC-18 (**resolvido no Lote 24**) · 6 → SEC-19 (**resolvido no Lote 28**) · 7 → SEC-20 (**resolvido no Lote 24**) · 8 → SEC-21 (**resolvido no Lote 24**) · 9 → SEC-22 (**resolvido no Lote 24**) · 10 → SEC-23 (**resolvido no Lote 28**) · 11 → SEC-24 (**resolvido no Lote 27**) · 12 → executado dentro do ARQ-00 (🔴 TestBrain) · C-03 (LIKE sem escape de `%`/`_`, GAP baixo) → SEC-25 (**resolvido no Lote 27**).
