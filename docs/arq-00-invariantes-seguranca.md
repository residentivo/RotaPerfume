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
- F-03 Headers: nosniff, X-Frame-Options DENY, Referrer-Policy, CSP `default-src 'none'; frame-ancestors 'none'`; HSTS exceto host exato localhost/loopback (`security_headers_middleware.go:22-56`).
- F-04 Timeouts: HTTP Read/Write 15s, Idle 60s, shutdown 10s (`main.go:117-144`); Turnstile 4s; SMTP 10s/30s; DB ping 5s, pool 25/5, lifetime 5m (`SH/db/db.go:32-41`). [GAP] sem MaxBytesReader.
- F-05 Envelope de erro: `writeJSON` (`API/handlers/response.go:28-41`), `writeError` do middleware (`auth_middleware.go:280-285`). Erro reconhecido → mensagem fixa; desconhecido → log + 500 "erro interno". `writeError` concatena sem escape (seguro só porque mensagens são constantes).
- F-06 Segredos: JWT_SECRET obrigatório, PASSWORD_PEPPER ≥32 bytes, DB_USUARIO/DB_SENHA obrigatórios, faixas de Argon2/refresh validadas (`SH/config/config.go`); mensagens nunca incluem valores.
- F-07 DSN `clientFoundRows=true`; condição no WHERE, nunca deduzir por RowsAffected (Revoke `WHERE revoked_at IS NULL` `refresh_token_repository.go:105`; Rehash `AND password_hash = ?` `usuario_repository.go:297`).
- F-08 DSN nunca logado (`maskDSN`, `SH/db/db.go:51-84`). [GAP] senha com `@` vaza parcialmente.
- F-09 vlog desligado por padrão (VERBOSE/LOG_LEVEL=debug); regras LOG-02 em `SH/vlog/vlog.go:7-15`; e-mail via `MaskEmail`; `withLogging` não loga query string (`main.go:210`). Testes: `AT/handlers/tst05_vlog_runtime_auth_test.go`.
- F-10 Paginação: limit default 20, max 100; page ≥1 (`API/services/usuario_service.go:403-424`); dashboard `dias` 1..365. [GAP] page sem teto.
- Riscos: builder de cadeia com ordem invertida (exigir teste: OPTIONS sem token em rota protegida → 204 com ACAO; 401 contém X-Content-Type-Options). Trocar `jwt(protected, requireAdmin)` booleanos por política tipada `Public | Authenticated | AdminOnly` obrigatória, sem default público. Prefixo `/api/senha-historico/` continua admin. Mapper de erros: só sentinelas via `errors.Is`; nunca `err.Error()` de erro embrulhado; `ValidarForcaSenha` (`auth_handler.go:717`) vira erro de domínio com texto fixo; não usar `map[error]` para decidir (ordem aleatória).
- Onde mora: interfaces/http (CORS, headers, logging, AuthN/AuthZ, envelope, error mapping); infrastructure (config, DSN, pool, maskDSN); `Pagina{page,limit}` VO com clamp no shared kernel.

## ARQ-02 Catálogo
- C-01 RBAC: produto leitura comum, escrita admin; estoque tudo admin (`routes.go:283-453`).
- C-02 Whitelist order_by (`SH/repositories/sort.go:29-46`), fallback para default; ASC/DESC. Produto `produto_repository.go:60`; Estoque duas whitelists `estoque_repository.go:77,83`. Teste `ST/repositories/sort_test.go`.
- C-03 LIKE parametrizado. [GAP baixo] `%`/`_` não escapados.
- Recomendação: enum tipado de ordenação no domínio/application; parse em interfaces/http; repositório mapeia enum→coluna via switch; assinatura nunca recebe string livre.

## ARQ-03 Identidade e Acesso
- I-01 JWT só HS256 (`SH/services/auth_service.go:99`), exp verificado + checagem defensiva; iat/nbf/exp emitidos.
- I-02 Bearer tem precedência; header malformado em rota protegida → 401 sem fallback para cookie; cookie `access_token` só sem header (`auth_middleware.go:72-93`).
- I-03 SEC-06: usuário consultado no banco a cada request protegido, sem cache; inexistente/inativo → 401 mesma mensagem; erro → 500 sem chamar handler; role do banco prevalece; JWT inválido → 401 sem consultar banco; rota pública não consulta; checker nil → panic na montagem.
- I-04 SEC-08: `iat <= tokens_validos_desde` → 401; com corte, token sem iat → 401. Corte gravado por UpdatePasswordHash, SetAtivo(false), InativarByVendedorID, InvalidarSessoes; RehashPassword NÃO mexe no corte.
- I-05 RBAC usa role do banco; handlers de usuário repetem checagem (defesa em profundidade); roles `admin|normal`; escopo trata role != "admin" como restrito (`scope.go:69`).
- I-06 Refresh: 32 bytes crypto/rand, banco guarda SHA-256, TTL 7d; rotação atômica (UPDATE condicional + insert na mesma tx); concorrência → 401; expiração checada antes de revogação; janela de graça 30s → 401 sem contar no rate limit; refresh reconsulta ativo e usa role do banco.
- I-07 Reuso SEC-07/12: motivo `rotacao` fora da janela → `RegistrarReuso` atômico; cortar → revogação em massa + InvalidarSessoes + alerta; erro ao marcar → corta (fail-closed); corte incompleto → DesfazerReuso (`auth_handler.go:528-607`); resposta HTTP idêntica; checar `ErrRefreshTokenRevokedRecently` ANTES de `ErrRefreshTokenRevoked`.
- I-08 Alerta SEC-09: assíncrono, dedup 30min/usuário, teto 20/h, 4 concorrentes, timeout 30s; lista de admins saneada CR/LF.
- I-09 Revogações: logout idempotente + limpa cookies; troca de senha: corte + revogação total + limpa cookies; reset admin: revogação total + corte; inativação: revogação total; desligamento de vendedor: tx única (data_desligamento + inativa usuários + revoga tokens) (`vendedor_service.go:343-389`).
- I-10 Cleanup CHORE-02: retenção mínima 24h, lotes de 1000, cancelável, parada antes de fechar o pool.
- I-11 Hash SEC-13: Argon2id sobre HMAC-SHA256(pepper, senha), salt 16B, comparação tempo constante, nunca sem pepper; bcrypt legado só verifica e marca rehash; rehash best-effort condicional; hash descartável para e-mail inexistente.
- I-12 Política: min 8 runes, max 72 bytes, 3 de 4 classes. Ordem do reset: captcha → rate limit → senha atual (conta) → nova==atual → força (não conta) → histórico (atual + 2 últimas, SEM short-circuit) → grava histórico/senha → revogação. `password_hash` `json:"-"`; `SenhaHashAnterior` sem tag JSON (só não vaza porque o handler monta resposta à mão).
- I-13 Turnstile em login e reset; fail-closed → 400; antes do rate limit; token/secret nunca logados.
- I-14 Rate limit: login `ip:<ip>` e `acct:<ip>|<email>` 5/1min bloqueio 5min; refresh `refresh-ip:<ip>` 10/1min bloqueio 5min; reset `reset:<ip>|<uid>` 10/5min bloqueio 10min; 429 + Retry-After ≥1s; sucesso zera. Em memória, uma instância por processo. IP: XFF/X-Real-IP só com TRUST_PROXY_HEADERS=true; senão RemoteAddr sem porta.
- I-15 Cookies: access_token HttpOnly SameSite=Strict MaxAge=TTL; refresh_token HttpOnly SameSite=Lax 7d; tokens nunca no corpo JSON.
- I-16 Anti-enumeração no login: inexistente e senha errada → mesmo 401, inexistente roda hash descartável.
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

> Mapeamento GAP → card (🔵 SubBrain): 1 → SEC-14 · 2 → SEC-15 · 3 → SEC-16 · 4 → SEC-17 · 5 → SEC-18 · 6 → SEC-19 · 7 → SEC-20 · 8 → SEC-21 · 9 → SEC-22 · 10 → SEC-23 · 11 → SEC-24 · 12 → executado dentro do ARQ-00 (🔴 TestBrain) · C-03 (LIKE sem escape de `%`/`_`, GAP baixo) → SEC-25.
