# Relatório de Falhas de Segurança — SistemaCompleto

Auditoria realizada por 🟣 SecBrain, cobrindo API Go (`apis/rotaperfumes-api`, `apis/shared`), Frontend Next.js (`frontend/`), Banco de Dados (`sql/`, `dados/`) e segredos no repositório.

## Tabela-Resumo

| # | Falha | Severidade | Arquivo | Status |
|---|--------|-----------|---------|--------|
| 1 | Credenciais em texto puro commitadas no repositório | Crítica | `itenslocais.txt` (raiz) | Corrigido |
| 2 | Password hash (bcrypt) exposto via API de auditoria | Média | `apis/rotaperfumes-api/handlers/senha_historico_handler.go:73,148` | Corrigido |
| 3 | Sem rate limiting / anti-bruteforce no login | Alta | `apis/rotaperfumes-api/routes/routes.go:74`, `handlers/auth_handler.go` (Login) | Corrigido |
| 4 | Ausência de headers de segurança HTTP (CSP, HSTS, X-Content-Type-Options, X-Frame-Options, Referrer-Policy) | Média | `apis/rotaperfumes-api/cmd/server/main.go`, `middleware/` | Corrigido |
| 5 | CORS aceita automaticamente qualquer origem `localhost`/`127.0.0.1` independente da porta | Baixa/Média | `apis/rotaperfumes-api/middleware/cors_middleware.go:17-21` | Corrigido |
| 6 | `getClientIP` confia cegamente em `X-Forwarded-For`/`X-Real-IP` (spoofable) | Baixa | `apis/rotaperfumes-api/handlers/auth_handler.go:436-444` | Corrigido |
| 7 | Log de tentativa de login expõe e-mail em texto claro | Baixa | `apis/rotaperfumes-api/handlers/auth_handler.go:132` | Corrigido |
| 8 | `seedusers` imprime senhas em stdout e passa senha do DB via argv (`-p senha`) | Baixa | `apis/shared/cmd/seedusers/main.go:96-101,239` | Corrigido |
| 9 | Senha padrão idêntica para os 42 vendedores seed, sem forçar troca | Média | `sql/03_seed_vendedores.sql:75-117` | Corrigido |
| 10 | `.env` local continha segredos reais (JWT_SECRET, senha de app Gmail) | Baixa (mitigada) | `.env` (raiz) | Corrigido |
| 11 | `ProtectedRoute` do frontend confia em `localStorage` para decisão de UI de admin | Baixa | `frontend/src/components/layout/ProtectedRoute.tsx:18-35`, `frontend/src/lib/auth.ts:27-49` | Corrigido |
| 12 | Rotas de negócio sem escopo por vendedor (possível excesso de exposição de dados) | Média (a validar) | `apis/rotaperfumes-api/routes/routes.go:176-246` | Corrigido |
| 13 | Dependências recentes sem política de atualização documentada | Baixa (a validar) | `frontend/package.json`, `apis/rotaperfumes-api/go.mod`, `apis/shared/go.mod` | Corrigido |

---

## Segredos no Repositório

### 1. CRÍTICA — Credenciais em texto puro no repositório — ✅ CORRIGIDO
**Arquivo:** `itenslocais.txt` (raiz), linhas 1-3 — continha usuário + senha em texto puro para 2 contas (`gerente_admin`, `rh_admin`); valores redigidos deste relatório propositalmente.
Estava versionado em todo o histórico do git (não coberto por `.gitignore`).

**Correção aplicada:** arquivo removido de todos os commits do histórico via `git filter-branch --index-filter` + `.gitignore` atualizado para bloquear reintrodução. **Ação pendente do usuário:** rotacionar as senhas das contas `gerente_admin`/`rh_admin` no banco (essas senhas ficaram expostas publicamente no GitHub até esta correção).

### 10. BAIXA (mitigada) — `.env` local com segredos reais — ✅ CORRIGIDO
**Arquivo:** `.env` (raiz) — continha `JWT_SECRET` e `SMTP_PASSWORD` (senha de app Gmail) reais, e estava commitado em todo o histórico do git (apesar de coberto por `.gitignore` atualmente).

**Correção aplicada:** arquivo removido de todos os commits do histórico via `git filter-branch`; `JWT_SECRET` regenerado (todas as sessões JWT existentes serão invalidadas — os usuários precisarão logar novamente). **Ação pendente do usuário:** gerar uma nova senha de app Gmail em myaccount.google.com/apppasswords e preencher `SMTP_USER`/`SMTP_PASSWORD` no `.env` local (a antiga foi invalidada/removida e não pôde ser recuperada). **Force-push necessário** para publicar a história reescrita no `origin` — ainda pendente de confirmação do usuário.

---

## API (Go)

**Pontos positivos:** JWT HS256 com segredo obrigatório, bcrypt para senhas, cookies `HttpOnly`/`SameSite`/`Secure`, refresh tokens com hash SHA-256 e rotação, `order_by`/`order_dir` validados por whitelist (sem SQLi), `password_hash` nunca serializado, mensagens de erro de login genéricas.

### 3. ALTA — Ausência de rate limiting / proteção contra brute force
**Arquivos:** `apis/rotaperfumes-api/routes/routes.go:73-90`, `handlers/auth_handler.go` (Login).
Nenhum middleware de rate limiting, CAPTCHA ou bloqueio por tentativas falhas.

**Cenário:** brute force / credential stuffing automatizado contra `/api/auth/login`, agravado pela senha padrão conhecida (item 9).

**Recomendação:** rate limiting por IP e por conta, bloqueio temporário após N tentativas falhas.

### 4. MÉDIA — Ausência de headers de segurança HTTP
**Arquivos:** `apis/rotaperfumes-api/cmd/server/main.go`, `middleware/`.
Faltam `Strict-Transport-Security`, `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy`, `Referrer-Policy`.

**Recomendação:** middleware de security headers aplicado a todas as respostas.

### 5. BAIXA/MÉDIA — CORS aceita qualquer origem localhost/127.0.0.1
**Arquivo:** `apis/rotaperfumes-api/middleware/cors_middleware.go:17-21`.
Prefix-match genérico permite qualquer porta em `localhost`/`127.0.0.1` com credentials=true.

**Recomendação:** restringir a lista explícita de origens via env; remover/guardar o prefix-match de dev por checagem de ambiente.

### 6. BAIXA — `X-Forwarded-For`/`X-Real-IP` confiados sem validação
**Arquivo:** `apis/rotaperfumes-api/handlers/auth_handler.go:436-444` (`getClientIP`).

**Recomendação:** só confiar nesses headers atrás de proxy conhecido; usar `r.RemoteAddr` como fallback padrão.

### 7. BAIXA — Log de e-mail em tentativa de login
**Arquivo:** `apis/rotaperfumes-api/handlers/auth_handler.go:132`.

**Recomendação:** mascarar parcialmente o e-mail no log.

### 8. BAIXA — `seedusers`: senha em stdout + argv
**Arquivo:** `apis/shared/cmd/seedusers/main.go:96-101,239`.

**Recomendação:** usar `MYSQL_PWD` ou `--defaults-extra-file`; evitar imprimir senha em claro.

### 2. MÉDIA — Exposição de hash de senha via API de auditoria
**Arquivo:** `apis/rotaperfumes-api/handlers/senha_historico_handler.go:73,148` — campo `senha_hash_anterior`.

**Recomendação:** remover `senha_hash_anterior` da resposta HTTP.

### 12. MÉDIA — Rotas de negócio sem escopo por vendedor — ✅ CORRIGIDO
**Arquivo:** `apis/rotaperfumes-api/routes/routes.go:176-246`.
`/api/pagamentos`, `/api/clientes`, `/api/pedidos`, `/api/vendedores/{id}/clientes` eram acessíveis a qualquer usuário autenticado sem filtro por carteira — confirmado pelo usuário como não intencional.

**Correção aplicada (🟡 BackBrain):** filtro server-side por `id_vendedor` para usuários `role=normal`, sempre resolvido no servidor a partir de `usuarios.id_vendedor` (nunca a partir de parâmetro vindo do cliente, o que evita bypass via query string/URL). `role=admin` continua sem restrição.
- `GET /api/clientes`: filtra por clientes com vínculo ativo (`data_fim IS NULL`) na tabela `carteiras` para o vendedor do usuário; `GET /api/clientes/{id}` retorna 404 se o cliente não pertencer à carteira do usuário.
- `GET /api/pedidos`: força `vendedor_id` do usuário no filtro (ignorando `vendedor_id` da query string); `GET /api/pedidos/{id}` retorna 404 se o pedido for de outro vendedor.
- `GET /api/pagamentos`: filtra pagamentos cujo `pedido_id` pertence a um pedido do vendedor do usuário (pagamentos não têm `vendedor_id` direto — vínculo via `pedidos.vendedor_id`); `GET /api/pagamentos/{id}` idem.
- `GET /api/vendedores/{id}/clientes`: usuário `role=normal` só pode consultar a própria carteira (`{id}` precisa ser igual ao seu `id_vendedor`), senão 404.
- Usuário `role=normal` sem `id_vendedor` vinculado nunca recebe "sem filtro" (retorna lista vazia/404 em vez de vazar todos os registros).
- Endpoints de escrita (POST/PUT) não foram alterados nesta correção (fora do escopo reportado, que trata de exposição de leitura).

---

## Banco de Dados

Sem uso de usuário root ou grants excessivos nos DDLs/seeds. Usuário dedicado `golang` (senha fraca apenas em dev). Constraints adequadas (UNIQUE em email, FKs com `ON DELETE SET NULL`).

### 9. MÉDIA — Senha padrão idêntica para os 42 vendedores seed
**Arquivo:** `sql/03_seed_vendedores.sql:75-117`.
Todos os 42 usuários "normal" usam a mesma senha padrão (valor redigido deste relatório) sem `deve_trocar_senha = 1`.

**Recomendação:** setar `deve_trocar_senha = 1` no INSERT; garantir rotação de senha antes de qualquer promoção a produção.

**Observação:** `MYSQL_LOCAL_INFILE=1` habilitado em `.env` — necessário para import de CSV; risco baixo/controlado pois não há `LOAD DATA LOCAL INFILE` com dado não confiável nos handlers.
**Atualização (DB-03, 2026-10-29):** o banco passou a ser MariaDB 11.4 LTS. O `Makefile` e o `Jenkinsfile` chamam o cliente `mariadb` com `--local-infile=0` (SEC-28), e o drop-in do `deploy/setup-servidor.sh` grava `local_infile = 0` no servidor MariaDB.

---

## Frontend (Next.js)

- Tokens de sessão não ficam em `localStorage`/`sessionStorage` (boa prática).
- `NEXT_PUBLIC_API_URL` é a única env pública — nenhum segredo exposto ao client.
- Sem `dangerouslySetInnerHTML`, `innerHTML` ou `eval(` em `frontend/src` — sem vetor de XSS óbvio.
- CSRF mitigado razoavelmente via `SameSite=Strict/Lax` + `HttpOnly` nos cookies de auth.

### 11. BAIXA — `ProtectedRoute` confia em `localStorage` para decisão de UI de admin
**Arquivos:** `frontend/src/components/layout/ProtectedRoute.tsx:18-38`, `frontend/src/lib/auth.ts:27-58`.
`isAdmin()`/`isAuthenticated()` leem só o `localStorage`, sem revalidar contra `/api/auth/me`.

**Cenário:** edição manual do `localStorage.auth_user.role` via DevTools exibe telas administrativas na UI (chamadas de API subjacentes continuam protegidas pelo JWT real, mas há vazamento de informação sobre funcionalidades admin).

**Recomendação:** revalidar `role` contra `/api/auth/me` ao montar rotas protegidas.

---

## Dependências

### 13. BAIXA (a validar) — Dependências recentes sem política de atualização documentada — ✅ CORRIGIDO
**Arquivos:** `frontend/package.json`, `apis/rotaperfumes-api/go.mod`, `apis/shared/go.mod`.

**Recomendação:** rodar `npm audit` / `go list -m -u` periodicamente; documentar processo no CLAUDE.md ou Makefile.

**Correção aplicada:**
- **Processo (🟡 BackBrain):** `Makefile` ganhou os alvos `deps-audit` (roda os dois abaixo), `deps-audit-go` (`go list -m -u all` filtrado + `govulncheck ./...` do PATH; se o govulncheck não estiver instalado, imprime aviso apontando para o `CLAUDE.md` e falha) e `deps-audit-frontend` (`npm outdated` informativo + `npm audit --omit=dev --audit-level=high`, falha em high ou acima). O comando de instalação do govulncheck fica no `CLAUDE.md` (o `Makefile` não pode conter o texto "golang" — teste SEC-11). Política documentada na seção "Auditoria de dependências" do `CLAUDE.md` (mensal + antes de cada deploy; achados viram card no Kanban; `npm audit fix --force` só com card próprio; `make -k deps-audit` para rodar ambos mesmo se um falhar).
- **Go (🟡 BackBrain):** a primeira execução encontrou GO-2025-3553 em `github.com/golang-jwt/jwt/v5` v5.2.1 → atualizado para v5.3.1 em `apis/shared/go.mod` e `apis/rotaperfumes-api/go.mod` (`x/crypto` passou a dependência direta no rotaperfumes-api, mesma versão). 15 vulnerabilidades da stdlib (go1.26.4 local), corrigidas em go1.26.9 → `apis/Dockerfile` agora usa `ARG GO_VERSION=1.26.9`. Testes: shared 2226 pass / 0 fail / 8 skip; rotaperfumes-api 2412 pass / 0 fail / 28 skip; build e vet ok.
- **Frontend (🟢 FrontBrain):** `npm audit fix` (sem `--force`) → sharp 0.35.4→0.35.5, source-map-js 1.2.1→1.2.2, eslint-config-next / @next/eslint-plugin-next 16.3.5→16.4.0 (apenas `package-lock.json`). `npm audit --omit=dev`: **0 vulnerabilidades em produção**. tsc e lint ok; vitest 1495/1495 (36 arquivos); `next build` ok.
- **Residual aceito:** 9 vulnerabilidades apenas em dependências de desenvolvimento (7 altas: `braces`, sem versão corrigida, e dependentes incluindo tailwindcss 3; 2 moderadas: `postcss-selector-parser`/`postcss-nested` via tailwind 3). Só corrigíveis com upgrade major Tailwind 3→4 — registrado no backlog como card DEP-01. Atualização dos módulos Go desatualizados (sem CVE) registrada como DEP-02.

---

## Priorização Sugerida

1. ~~**Crítica** — remover `itenslocais.txt` do repo/histórico e rotacionar credenciais.~~ ✅ Removido do histórico; rotação de senha das contas pendente do usuário.
2. ~~**Alta** — implementar rate limiting no login/refresh.~~ ✅ Corrigido (🟡 BackBrain).
3. ~~**Média** — remover `senha_hash_anterior` da resposta; adicionar security headers; validar escopo de `/api/pagamentos`, `/api/clientes`, `/api/pedidos`; ajustar seed de vendedores.~~ ✅ Corrigido (🟡 BackBrain / 🌸 DataBrain).
4. ~~**Baixa** — revalidar sessão no frontend via `/api/auth/me`; restringir CORS; não confiar em `X-Forwarded-For` sem proxy; mascarar e-mail em logs; corrigir exposição de senha em `seedusers`.~~ ✅ Corrigido (🟡 BackBrain / 🟢 FrontBrain).
5. ~~**A validar** — CVEs de dependências e política de atualização (item 13).~~ ✅ Corrigido (🟡 BackBrain / 🟢 FrontBrain): alvos `make deps-audit*`, política no `CLAUDE.md`, jwt/v5 → v5.3.1, imagem Go → 1.26.9, `npm audit fix` (0 vulns em produção).

Todos os 13 itens identificados nesta auditoria (1-13) foram corrigidos. Os itens 1-12 foram validados por 🔴 TestBrain; o item 13 foi validado pela execução do próprio `make deps-audit` e pelas suítes de testes Go e frontend após as atualizações. A auditoria de dependências passa a ser uma rotina recorrente (mensal e antes de cada deploy), conforme o `CLAUDE.md`.

---

## Pendências Operacionais (fora do escopo dos agentes)

As ações abaixo não são falhas de código — são passos que dependem de decisão/execução direta do usuário (dono de credenciais, contas de e-mail e repositórios) e não podem ser realizadas pelos agentes:

1. **Rotacionar senhas das contas `gerente_admin`/`rh_admin` no banco.** Essas credenciais ficaram expostas em texto puro no GitHub (arquivo `itenslocais.txt`) até a limpeza do histórico do git (item 1). Mesmo com o arquivo removido do histórico, os valores antigos devem ser considerados comprometidos e as senhas trocadas diretamente no banco de dados.
2. **Gerar nova senha de app Gmail e atualizar o `.env` local.** A senha de app anterior (usada em `SMTP_USER`/`SMTP_PASSWORD`) foi destruída durante a limpeza do histórico do git (item 10) e não pôde ser recuperada. É necessário gerar uma nova em [myaccount.google.com/apppasswords](https://myaccount.google.com/apppasswords) e preencher as variáveis no `.env` local.
3. **Tratar forks do repositório GitHub (`residentivo/RotaPerfume`), se existirem.** Caso haja forks, eles ainda contêm o histórico antigo com os segredos (`itenslocais.txt`, `.env`) mesmo após a reescrita do histórico no repositório principal. Cada fork precisaria do mesmo tratamento (filter-branch + force-push), o que está fora do alcance desta sessão — requer verificação manual do usuário em github.com/residentivo/RotaPerfume/network/members (ou equivalente).
4. ~~**Configurar suíte de testes automatizados no frontend.**~~ ✅ **RESOLVIDO** — o frontend agora tem vitest configurado (`make test-frontend`) e a revalidação de sessão do `ProtectedRoute.tsx` (item 11) está coberta por `frontend/tests/components/layout/ProtectedRoute.test.tsx`.
5. ~~**Adotar rotina periódica de auditoria de dependências (item 13).**~~ ✅ **RESOLVIDO** — política documentada na seção "Auditoria de dependências" do `CLAUDE.md` e automatizada em `make deps-audit` (rodar mensalmente e antes de cada deploy).
6. **Atualizar o Go local para ≥ 1.26.9 e reconstruir a imagem Docker da API.** As 15 vulnerabilidades da stdlib apontadas pelo govulncheck (item 13) só desaparecem localmente com o toolchain atualizado (hoje go1.26.4). O `apis/Dockerfile` já usa `GO_VERSION=1.26.9`, mas a imagem precisa ser reconstruída para que o binário em execução receba as correções.
7. **Instalar o govulncheck na máquina local.** Pré-requisito do `make deps-audit-go`: `go install golang.org/x/vuln/cmd/govulncheck@latest`, garantindo que `$(go env GOPATH)/bin` esteja no `PATH`.
