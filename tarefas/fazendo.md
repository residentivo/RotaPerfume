# Fazendo

## Lote 21 (2026-10-13): reestruturação DDD + Clean Code + Design Patterns da API

> Aberto pelo 🤍 MegaBrain a partir do pedido "revise todo o código da API e adicione Design Patterns em todos os locais possíveis para simplificar o código. Use o padrão DDD e Clean Code para reestruturar o código da API".

**Decisões do usuário:** estratégia **incremental por bounded context**: cada lote termina com build e testes verdes e um commit na branch `refactor/ddd`. Escopo: API (`apis/rotaperfumes-api`) + módulo `apis/shared` (repositories, models, services, importers e CLIs).

**Fluxo:** ARQ-00 (🟣 SecBrain → 🟡 BackBrain: blueprint) → ARQ-01 (fundação/shared kernel) → ARQ-02 (Catálogo, piloto) → ARQ-03 (Identidade e Acesso) → ARQ-04 (Comercial) → ARQ-05 (Vendas) → ARQ-06 (Analytics) → ARQ-07 (Notificação, importadores, CLIs e remoção do legado) → ARQ-08 (🔵 SubBrain: documentação e fechamento). Em cada lote de código: 🟡 BackBrain → 🔴 TestBrain (no ARQ-03, o 🟣 SecBrain revisa depois, junto com o 🔴 TestBrain; ver decisão no card). ARQ-00 (commit `cd12d47`), ARQ-01 (commit `0e7e757`) e ARQ-02 (commit `17502da`) concluídos, ver `feito.md`; ARQ-04 a ARQ-08 estão em `afazer.md`, na ordem de execução.

**Passo atual:** ARQ-03 (Identidade e Acesso): implementado e aprovado com ressalvas pelo 🟣 SecBrain e pelo 🔴 TestBrain; 🟡 BackBrain aplicando os ajustes finais, depois vem o commit. Ver card abaixo.

**Pendência opcional (não é card):** o race detector (`go test -race`) está indisponível nesta máquina porque falta gcc/MinGW (cgo). Sugestão ao usuário: instalar o MinGW-w64 localmente ou rodar `go test -race ./...` no Jenkins (Linux).

---

## ARQ-03: contexto Identidade e Acesso — prioridade ALTA (sensível a segurança)

**Status:** em execução (Lote 21, desde 2026-10-14; ARQ-02 concluído em `17502da`)
**Camadas:** Segurança, Backend
**Responsável:** 🟡 BackBrain → 🟣 SecBrain (revisão aprofundada) + 🔴 TestBrain
**Origem:** Lote 21 (2026-10-13), reestruturação DDD da API. Referências: `docs/arquitetura-ddd.md` e `docs/arq-00-invariantes-seguranca.md` (I-01..I-16).

**Descrição:** migrar:
- Auth/JWT e Usuário;
- RefreshToken com rotação e detecção de reuso;
- SenhaHistórico;
- hash Argon2, política de senha e gerador de senha;
- Turnstile e alerta de segurança;
- middlewares auth, user_status e ratelimit.

**Decisão do 🤍 MegaBrain (2026-10-14):** a especificação prévia do 🟣 SecBrain foi **dispensada**, porque `docs/arq-00-invariantes-seguranca.md` (I-01..I-16) já a cobre. Em compensação, o 🟣 SecBrain fará **revisão aprofundada pós-implementação**, junto com o 🔴 TestBrain.

**Regras herdadas do piloto:** métrica "código novo ≤ lógica legada sem vlog; handlers ≤ 30 linhas; código novo 100% coberto"; vlog P1 via `defer vlog.Trace`.

**Implementação (🟡 BackBrain, 2026-10-14):** Identidade e Acesso migrado para `apis/shared/identidade{,app,mysql,crypto}` + `apis/rotaperfumes-api/internal/identidadehttp`: Autenticador nativo (role lida do banco, substituindo o adapter de transição do ARQ-01), singletons de rate limit/notifier/Turnstile, fronteiras transacionais idênticas às atuais e testes SEC portados. Legado removido: `handlers/{auth,senha_historico,usuario}_handler.go`, `middleware/{auth_middleware,ratelimit_middleware,user_status}.go` e `services/{refresh_cleanup,refresh_token_service,reset_senhas_ativos_service,senha_historico_service,usuario_service}.go`. Ficaram pontes para o ARQ-04/ARQ-07 (ver notas nesses cards).

**Histórico de validação:**
- 2026-10-14, 🟣 SecBrain: **APROVADO COM RESSALVAS**, sem bloqueantes. I-01..I-16 e O4/O5/O7 conferidos; GAPs SEC-18, SEC-20, SEC-21 e SEC-22 preservados (nem piorados nem corrigidos). Ressalva #2 (restringir quem chama `ResetarSenhasAtivos`, `RegistrarReuso` e `DesfazerReuso`) vai para o ajuste final do 🟡 BackBrain; NB-1, NB-6 e NB-7 anotados no ARQ-07.
- 2026-10-14, 🤍 MegaBrain: confirmou por `git show` que três comportamentos são idênticos ao legado: login de usuário inativo conta no rate limit, Bearer comparado com EqualFold e, no reset, campos obrigatórios validados antes do captcha.
- 2026-10-14, 🔴 TestBrain: **APROVADO COM RESSALVAS**. R1 ok (testes API 793→806, shared 652→687); cobertura combinada 89,07%→89,35% (linhas descobertas 1.248→1.110). Das 18 mutações de segurança, 6 sobreviviam e passaram a ser mortas pelos testes `TestARQ03_*` que ele adicionou. As 285 linhas de asserção alteradas foram classificadas, quase todas troca de identificador. Ressalvas: a asserção do `bug11` não-verbose ficou vazia e a de `HashPassword_Erros` ficou genérica.

**Decisões:**
- especificação prévia do 🟣 SecBrain dispensada, com revisão aprofundada pós-implementação (ver acima);
- fronteiras transacionais conforme a seção abaixo (troca de senha fora da UoW → BUG-15);
- GAPs de segurança pré-existentes não são corrigidos neste card (regra "não piorar");
- pontes de transição (shims de teste, `middleware/principal_legado.go`, legado encolhido de usuário/refresh token, adapter `vendedoresLegado`) ficam até o ARQ-04/ARQ-07;
- logs de auditoria com `por admin=<role>` seguem para o CHORE-05.

**Passo atual:** 🟡 BackBrain aplicando os ajustes finais: (1) corrigir as 2 asserções (`bug11` não-verbose e `HashPassword_Erros`); (2) teste de arquitetura restringindo os chamadores de `ResetarSenhasAtivos`, `RegistrarReuso` e `DesfazerReuso` (ressalva #2 do 🟣 SecBrain); (3) registro no blueprint. Depois: build e testes verdes e commit do ARQ-03 na branch `refactor/ddd`.

**Fronteiras transacionais (🤍 MegaBrain, 2026-10-14):**
- **Troca de senha** (histórico + hash + corte + revogação): fica **fora** da Unit of Work neste card e vai para o BUG-15 (O8-iii).
- **Rotação de refresh token** (UPDATE condicional + insert na mesma tx, I-06): **continua transacional** no ARQ-03, via `db.Transactor`.
- **Detecção de reuso:** mantém a fronteira atual, com `RegistrarReuso` atômico + `DesfazerReuso` compensatório (O8-iv).

**Critério de aceite:**
- contrato HTTP inalterado (rotas, payloads, status codes e mensagens de erro);
- `go build`, `go vet` e `go test ./...` verdes em `apis/rotaperfumes-api` e `apis/shared`;
- sem perda de cobertura;
- logs `vlog` preservados;
- invariantes I-01..I-16 preservadas (revisão do 🟣 SecBrain);
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
