# Fazendo

## Lote 21 (2026-10-13): reestruturação DDD + Clean Code + Design Patterns da API

> Aberto pelo 🤍 MegaBrain a partir do pedido "revise todo o código da API e adicione Design Patterns em todos os locais possíveis para simplificar o código. Use o padrão DDD e Clean Code para reestruturar o código da API".

**Decisões do usuário:** estratégia **incremental por bounded context**: cada lote termina com build e testes verdes e um commit na branch `refactor/ddd`. Escopo: API (`apis/rotaperfumes-api`) + módulo `apis/shared` (repositories, models, services, importers e CLIs).

**Fluxo:** ARQ-00 (🟣 SecBrain → 🟡 BackBrain: blueprint) → ARQ-01 (fundação/shared kernel) → ARQ-02 (Catálogo, piloto) → ARQ-03 (Identidade e Acesso) → ARQ-04 (Comercial) → ARQ-05 (Vendas) → ARQ-06 (Analytics) → ARQ-07 (Notificação, importadores, CLIs e remoção do legado) → ARQ-08 (🔵 SubBrain: documentação e fechamento). Em cada lote de código: 🟡 BackBrain → 🔴 TestBrain (no ARQ-03, o 🟣 SecBrain revisa depois, junto com o 🔴 TestBrain; ver decisão no card). ARQ-00 (commit `cd12d47`), ARQ-01 (commit `0e7e757`), ARQ-02 (commit `17502da`), ARQ-03 (commit `c412d10`), ARQ-04 (commit `fdaa7d9`) e ARQ-05 (commit `3e33f6f`) concluídos, ver `feito.md`; ARQ-07 e ARQ-08 estão em `afazer.md`, na ordem de execução.

**Passo atual:** ARQ-06 (Analytics/Dashboard): aprovado pelo 🟣 SecBrain e pelo 🔴 TestBrain; 🟡 BackBrain aplica os ajustes finais e depois vem o commit. Ver card abaixo.

**Pendência opcional (não é card):** o race detector (`go test -race`) está indisponível nesta máquina porque falta gcc/MinGW (cgo). Sugestão ao usuário: instalar o MinGW-w64 localmente ou rodar `go test -race ./...` no Jenkins (Linux).

---

## ARQ-06: contexto Analytics (Dashboard) — prioridade MÉDIA

**Status:** em execução (Lote 21, desde 2026-10-15; ARQ-05 concluído em `3e33f6f`)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain + 🟣 SecBrain
**Origem:** Lote 21 (2026-10-13), reestruturação DDD da API.

**Descrição:** migrar o Dashboard como read model (lado de consulta, no estilo CQRS leve) para `apis/shared/analytics{,app,mysql}` + `apis/rotaperfumes-api/internal/analyticshttp`.

**Itens do card:**
- read models de consulta (CQRS leve), sem passar pelos agregados de escrita;
- escopo por carteira via `Filtro()` (mesmo padrão de Comercial/Vendas; zero value = restrito);
- teste de arquitetura `TestARQ_SprintfSQLSoConstantes` (SQL montado com `Sprintf` só a partir de constantes);
- remoção da ponte `apis/rotaperfumes-api/handlers/scope.go` (último consumidor era o dashboard).

**Lições do ARQ-05 (2026-10-15):** zero value do escopo nunca pode ser fail-open (mutação sobrevivente no ARQ-05); filtros de vendedor vindos da query devem ser ignorados na listagem restrita (`TestARQ05_ListagemRestrita_IgnoraVendedorDaQuery`); preferir `require.IsType`/`, ok` nas asserções de tipo dos testes de handler (ver TST-06).

**Histórico (2026-10-15):**
- 🟡 BackBrain migrou Analytics/Dashboard (`shared/analytics{,app,mysql}` + `internal/analyticshttp`); removidos `handlers/{dashboard_handler,response,scope}.go`, `services/{dashboard_service,paginacao}.go` e `shared/repositories/{cliente,dashboard}_repository.go`.
- 🟣 SecBrain: **APROVADO**, 0 bloqueantes. D-01..D-03, O1 e O5 atendidos; a remoção da ponte `handlers/scope.go` não deixou caminho sem escopo; o ranking do perfil restrito é igual ao legado.
- 🔴 TestBrain: **APROVADO**. R1 ok (shared 740→761 testes, API 833→839); cobertura combinada 89,416%→89,418%, descobertos 881→852; 31/31 mutantes mortos pelos testes existentes.
- **Incidente:** houve um incidente no script de mutação do 🔴 TestBrain; foi revertido e a integridade foi verificada por md5 do perfil de cobertura.

**Decisões (🤍 MegaBrain):**
- aplicar agora as ressalvas #2 (erro ignorado no Ranking) e #6 (`Relogio` no lugar de `time.Now()` na série) do 🟣 SecBrain;
- demais ressalvas fora do lote: BUG-18 (IN sem limite nas metas e detecção de tabela ausente por texto) e nota no SEC-16 (overflow do offset no ranking);
- ARQ-07: cobertura mantida extraindo a lógica dos `main` de `cmd/*` para pacotes testáveis (sem afrouxar o portão) e pendências (k)–(n) registradas no card.

**Passo atual:** 🟡 BackBrain aplicando os ajustes finais (SecBrain #2 e #6); depois, build/testes verdes e commit do ARQ-06 na branch `refactor/ddd`.

**Critério de aceite:**
- contrato HTTP inalterado (rotas, payloads, status codes e mensagens de erro);
- `go build`, `go vet` e `go test ./...` verdes em `apis/rotaperfumes-api` e `apis/shared`;
- sem perda de cobertura;
- logs `vlog` preservados;
- commit do lote na branch `refactor/ddd`.

---

## MAIL-01: programa de diagnóstico de envio de e-mail (`cmd/testeemail`) — prioridade ALTA

**Status:** em execução (2026-10-09; avançou no Lote 20, 2026-10-11)
**Camada:** Backend (ferramenta CLI)
**Responsável:** 🟡 BackBrain
**Origem:** pedido do usuário: "o envio de email não está funcionando". Pediu um programa que só envie um e-mail de `henrique.rodrigues@rotaperfumes.com.br` para `admin@rotaperfumes.com.br`, com texto padrão, logando todos os passos e erros e usando o `.env` da raiz.

**Suspeitas levantadas (🤍 MegaBrain):** no `.env` da raiz, `SMTP_PORT=465` (TLS implícito/SMTPS). Mas `services/email_service.go` (`enviar`) abre TCP puro e exige STARTTLS, e na 465 o servidor espera o handshake TLS antes do banner, então trava ou falha. Além disso, `SMTP_HOST` é um IP (`192.168.168.106`) e o `ServerName` do TLS recebe o host, então a verificação do certificado falha se o certificado não tiver SAN de IP.

**Histórico:**
- 2026-10-09: 🟡 BackBrain entregou `apis/shared/cmd/testeemail` e `apis/shared/tools/testeemail` (build, vet e 6 testes OK).
- 2026-10-11 (Lote 20): o diagnóstico real rodou uma vez. A porta 465 em `192.168.168.106` está acessível (a 587 recusa). O certificado é autoassinado ("rcgen self signed cert", SAN DNS só `localhost`, sem SAN de IP), SHA-256 `C2:A9:4D:46:D2:D2:4C:92:0A:A2:B9:EC:59:D4:5E:66:47:9F:CA:23:BB:24:87:29:52:C1:1C:2A:17:10:10:E0`. Falhou no handshake TLS (x509 sem IP SAN) e nenhum e-mail foi enviado. Aviso: o remetente `henrique.rodrigues@` é diferente do `SMTP_USER` (`admin@`).
- 2026-10-11 (Lote 20): correção no código (🟡 BackBrain):
  - novo `apis/shared/services/email_smtp.go`: TLS implícito na 465 e STARTTLS nas outras portas; AUTH obrigatório (`ErrSMTPSemAuth`; PLAIN ou LOGIN); credenciais nunca trafegam sem TLS; timeouts de 10s/30s; verificação de certificado sempre ligada; TLS ≥ 1.2;
  - variáveis novas opcionais `SMTP_TLS_MODE`, `SMTP_TLS_SERVER_NAME` e `SMTP_TLS_CA_FILE` (em `.env.example` e `deploy/api.env.example`);
  - configuração SMTP inválida agora impede a API de subir;
  - `testeemail` ganhou `-tls-ca-file` e `-salvar-cert`;
  - testes: 9 `TestMAIL01_*` novos (`apis/shared/tests/services/mail01_smtp_tls_test.go`); shared 2252 e api 2429 aprovados, 0 falhas.

**Passo atual:** aguardando o usuário:
1. confirmar o fingerprint acima com o admin do servidor de e-mail, salvar o PEM **fora do repositório** e definir no `.env` `SMTP_TLS_SERVER_NAME=localhost` e `SMTP_TLS_CA_FILE=<caminho>`;
2. rodar de novo `go run ./cmd/testeemail` em `apis/shared` (usar `-from admin@rotaperfumes.com.br` se o servidor recusar o remetente);
3. solução definitiva recomendada: certificado real no servidor de e-mail;
4. no deploy: montar o PEM no `deploy/docker-compose.yml` (o container da API é `read_only`) e definir as variáveis no `api.env`.

**Fechamento:** só após um envio real bem-sucedido.

---

## Lote 13 (2026-09-30): DEPLOY-01

> Aberto pelo 🤍 MegaBrain a partir do pedido "implementar o projeto no servidor ivo-inspiron-15-3530 com Docker". Vindo de `afazer.md`.

**Fluxo:** 🟣 SecBrain (spec de segurança do deploy) → 🟡 BackBrain (infra: API, compose, Caddy, Jenkins, dump) ∥ 🟢 FrontBrain (Dockerfile do frontend) → 🔵 SubBrain (manual e Kanban) → **usuário** (commit/push, servidor, primeira execução) → 🔵 SubBrain (fechamento em `feito.md`).

**Passo atual:** código enviado ao GitHub (commit 87d79af) e `deploy/setup-servidor.sh` criado (🟡 BackBrain); aguardando o usuário rodar o script no servidor, cadastrar as 2 credenciais no Jenkins e acompanhar a 1ª execução do job.

---

## DEPLOY-01: implantar o sistema no servidor `ivo-inspiron-15-3530` via Docker + Jenkins — prioridade ALTA

**Status:** em execução (Lote 13, 2026-09-30)
**Passo atual:** ver o "Passo atual" do bloco do Lote 13, acima.
**Camadas:** Segurança, Backend (infra), Frontend (infra), Documentação
**Origem:** pedido do usuário, "implementar o projeto no servidor ivo-inspiron-15-3530 com Docker".

**Decisões do usuário:** deploy por **pipeline Jenkins** (Jenkins em `http://ivo-inspiron-15-3530:8888/`); **MySQL que já existe no servidor**; dados vindos de um **dump do banco local**.

**Fatos levantados (🤍 MegaBrain):** o servidor é `192.168.168.106`. A porta 22 (SSH) está aberta, mas sem chave autorizada. A porta 80 já está ocupada por outra aplicação. A 8888 é o Jenkins 2.541.

**Arquitetura adotada:** o Jenkins clona `residentivo/RotaPerfume` (branch `main`) e roda `docker compose -p rotaperfumes`. O Caddy (`tls internal`) é a **única** porta publicada (**8443**) e roteia `/api/*` → API Go (`api:8080`) e o resto → Next (`frontend:3000`). O MySQL do host é acessado via `host.docker.internal`. HTTPS é obrigatório porque os cookies de auth são `Secure`.

**Entregue:**
- 🟣 **SecBrain (spec):** segredos só em credenciais Jenkins (Secret file `rotaperfumes-api-env`, Username/password `rotaperfumes-db-admin`), sem `set -x`/eco de segredos e com `api.env` modo 600 apagado no `post`; usuário MySQL da API só com DML (`rotaperfumes_app@172.16.0.0/12`) e admin separado para o import; API e frontend sem porta publicada; `TRUST_PROXY_HEADERS=true` com o Caddy sobrescrevendo `X-Forwarded-For` e removendo `X-Real-IP`; CORS explícito (hostname e IP); containers com `cap_drop: ALL`, `no-new-privileges` e `read_only` (API/Caddy); `JWT_SECRET` novo; truncar `refresh_tokens` após o import; Turnstile com o hostname cadastrado.
- 🟡 **BackBrain (infra):** `apis/Dockerfile` (multi-stage, distroless nonroot, contexto `apis/` por causa do `replace ../shared`); `deploy/docker-compose.yml`; `deploy/Caddyfile`; `deploy/api.env.example`; `deploy/mysql-setup.sql`; `deploy/dump-local.ps1` (senha via `MYSQL_PWD`, saída em `deploy/dumps/`, que o git ignora); `Jenkinsfile` (Checkout → Testes Go → Build → Importar dump opcional → Deploy → Smoke test); `.dockerignore` (raiz e `apis/`); `.gitignore` com `deploy/api.env` e `deploy/dumps/`. Um dump local já foi gerado em `deploy/dumps/`.
- 🟢 **FrontBrain (Dockerfile):** `frontend/Dockerfile` (Next standalone, `NEXT_PUBLIC_API_URL`/`NEXT_PUBLIC_TURNSTILE_SITE_KEY` como build-args, usuário `node`); `frontend/.dockerignore`; `next.config.js` com `output: 'standalone'`; `src/lib/{api,apiClient,auth}.ts` removem a barra final do `API_BASE`.
- 🔵 **SubBrain:** manual `docs/deploy-servidor.md` (pré-requisitos, MySQL, Turnstile, credenciais, job, dump, CA do Caddy, operações, rollback e troubleshooting). Não há README nem índice de docs no repositório para linkar o manual.

**Validado localmente:** `go build`/`go test` da API, `next build` e os 1482 testes do frontend.

**NÃO validado (sem Docker local):** `docker build` das três imagens, `docker compose up`, o roteamento/TLS do Caddy, o pipeline Jenkins (credenciais, import do dump, smoke test) e o acesso real pelo navegador com cookie `Secure` e Turnstile no hostname da LAN. Isso será verificado na primeira execução no servidor.

**Pendências do usuário:**
1. Commit e push dos arquivos de deploy para o `main`.
2. Pré-requisitos do servidor: docker + compose ≥ 2.17, usuário `jenkins` no grupo `docker` e restart do Jenkins, MySQL (bind-address, `mysql-setup.sql`, ufw) e hostname cadastrado no Turnstile.
3. Credenciais `rotaperfumes-api-env` e `rotaperfumes-db-admin` no Jenkins, e o job Pipeline from SCM.
4. Copiar o dump para `/opt/rotaperfumes/dumps/rotaperfumes.sql.gz`, rodar a primeira execução com `IMPORTAR_DUMP=true` e depois apagar o dump.
5. Instalar a CA do Caddy nos clientes e confirmar o acesso em `https://ivo-inspiron-15-3530:8443`.

**Fechamento:** após a primeira execução verde e o login confirmado no navegador, o 🔵 SubBrain move o card para `feito.md`.
