# Fazendo

## Lote 13 (2026-09-30): DEPLOY-01

> Aberto pelo 🤍 MegaBrain a partir do pedido "implementar o projeto no servidor ivo-inspiron-15-3530 com Docker". Vindo de `afazer.md`.

**Fluxo:** 🟣 SecBrain (spec de segurança do deploy) → 🟡 BackBrain (infra: API, compose, Caddy, Jenkins, dump) ∥ 🟢 FrontBrain (Dockerfile do frontend) → 🔵 SubBrain (manual e Kanban) → **usuário** (commit/push, servidor, primeira execução) → 🔵 SubBrain (fechamento em `feito.md`).

**Passo atual:** código enviado ao GitHub (commit 87d79af) e `deploy/setup-servidor.sh` criado (🟡 BackBrain); aguardando o usuário rodar o script no servidor, cadastrar as 2 credenciais no Jenkins e acompanhar a 1ª execução do job.

---

## DEPLOY-01: implantar o sistema no servidor `ivo-inspiron-15-3530` via Docker + Jenkins — prioridade ALTA

**Status:** em execução (Lote 13, 2026-09-30)
**Passo atual:** ver linha 9.
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
