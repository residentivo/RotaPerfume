# Fazendo

> Atualizado em 2026-11-02 (abertura dos Lotes 30, 31 e 32, 🔵 SubBrain): o usuário pediu para executar **todas** as pendências, aprovou antecipadamente as decisões de produto/contrato pendentes nos cards (inclusive os textos do **CHORE-07** e os nomes de cookies do **SEC-33**) e pediu, ao final, o **merge de todas as branches na `main` e o commit da `main`**, para testar o site em produção. O 🤍 MegaBrain abriu três lotes, com os cards vindos de `afazer.md` (blocos abaixo):
> - **Lote 30 de 2026-11-02 (somente MariaDB + infra/TLS), 16 cards:** DEPLOY-05, CHORE-13, DEPLOY-06, DEPLOY-04, DOC-08, TST-15, SEC-45, SEC-39, SEC-31, SEC-30, SEC-40, SEC-41, SEC-42, TST-12, DB-04 e DB-06.
> - **Lote 31 de 2026-11-02 (backend/frontend, BAIXA), 24 cards:** TST-06, BUG-18, TST-07, DOC-06, FE-15, UX-02, CHORE-07, BUG-22, SEC-32, SEC-33, BUG-23, CHORE-08, FE-16, CHORE-09, SEC-34, SEC-36, BUG-24, CHORE-10, FE-17, CHORE-11, SEC-38, LOG-03, CHORE-04 e CHORE-05; mais a conferência do upstream do **DEP-03** (o card fica em `afazer.md` como acompanhamento se não houver correção).
> - **Lote 32 de 2026-11-02, 1 card:** ARQ-09 (valores monetários em centavos).
>
> **Triagem do 🤍 MegaBrain:** DOC-07 e DEPLOY-03 cancelados (`feito.md`); TST-14 e TST-04 ficam em `afazer.md`, bloqueados (sem acesso ao socket do Docker; `.env` local aponta para o banco de produção `192.168.168.106:3306`, onde os agentes não rodam `make db-*`).
>
> **Passo atual (nos três lotes):** "🟣 SecBrain especificando → devs → 🔴 TestBrain → 🔵 SubBrain fechando". Depois do fechamento: merge das branches na `main` e commit da `main` (pedido do usuário).
>
> Antes, em 2026-11-01 (fechamento do Lote 29, 🔵 SubBrain): **nenhum card em execução.** O **Lote 29** foi concluído e está em `feito.md` ("Lote 29 de 2026-11-01"; **não commitado, pendente do usuário**): 7 cards concluídos (SEC-29, TST-09, TST-08, SETUP-01, SEC-44, SEC-37 e DEPLOY-02), 2 cancelados (TST-10 e TST-13) e 1 fechado sem correção (SEC-43). Dele saíram, em `afazer.md` (bloco "Achados do Lote 29"), SEC-39, SEC-40, SEC-41, SEC-42, TST-12, DOC-07, DB-06, DEPLOY-03, DEPLOY-04 e TST-14; o TST-11 foi fechado (flake corrigido no TST-09). **Atenção antes do merge na `main`:** o usuário precisa executar os passos 1 a 6 do `docs/Instal_SSL.md` no servidor (ver pendências do SEC-44 em `feito.md`). Passos anteriores do Lote 29: "🟣 SecBrain especificando"; "🟡 BackBrain (SEC-29) e 🔴 TestBrain (SETUP-01) → 🔴 TestBrain (integração TST-09/TST-08) e 🟣 SecBrain revisando SEC-29 → 🔵 SubBrain fechando"; "SEC-37 e DEPLOY-02: 🟣 SecBrain especificando"; "🟡 BackBrain (SEC-29b/Instal_SSL) e 🟣 SecBrain especificando SEC-37 e DEPLOY-02 → 🟡 BackBrain (SEC-37) e 🌸 DataBrain (DEPLOY-02) → 🔴 TestBrain → 🔵 SubBrain fechando" (2026-11-01).
>
> Antes, ainda em 2026-11-01 (novas decisões do usuário no Lote 29, 🔵 SubBrain): **TST-13 cancelado** e **SEC-43 fechado como "não corrigir"** (ambiente de dev; risco aceito pelo usuário), ambos em `feito.md`. Card novo **SEC-44** (TLS do SEC-29 no ambiente real, MÉDIA) entrou no Lote 29: a produção é MySQL 8.0.46 nativo com certificado sem SAN e o Jenkins roda em contêiner. Card novo **DOC-07** (docs MariaDB 11.4 × produção MySQL 8.0, MÉDIA) em `afazer.md`. O lote segue com 3 cards: SEC-37, DEPLOY-02 e SEC-44. Passo atual: "🟡 BackBrain (SEC-29b/Instal_SSL) e 🟣 SecBrain especificando SEC-37 e DEPLOY-02 → 🟡 BackBrain (SEC-37) e 🌸 DataBrain (DEPLOY-02) → 🔴 TestBrain → 🔵 SubBrain fechando" (SEC-29b = SEC-44).
>
> Antes, ainda em 2026-11-01 (fechamento parcial do Lote 29, 🔵 SubBrain): **SEC-29, TST-09, TST-08 e SETUP-01 concluídos** e movidos para `feito.md` ("Lote 29 de 2026-11-01: 4 cards concluídos, 1 cancelado"; não commitado, pendente do usuário). O lote **continua aberto** com **SEC-37** e **DEPLOY-02** (não trabalhados hoje). Passo atual: "SEC-37 e DEPLOY-02: 🟣 SecBrain especificando". Dos cards concluídos saíram SEC-39, SEC-40, SEC-41, SEC-42, SEC-43, TST-12 e TST-13 em `afazer.md` (bloco "Achados do Lote 29").
>
> Antes, ainda em 2026-11-01 (decisões do usuário no Lote 29, 🔵 SubBrain): **TST-10 cancelado** (o projeto foi migrado para um HD com Linux, sem exFAT, e com Node 22 via nvm; card em `feito.md`). **TST-08** resolvido no ambiente pelo usuário (Go da versão do `go.mod` instalado localmente; `make test` roda); falta a verificação do 🔴 TestBrain e a documentação do pré-requisito, vinculada ao card novo **SETUP-01** (scripts de instalação do ambiente de desenvolvimento em Linux e Windows, MÉDIA, entrou no Lote 29). **SEC-29:** a parte do servidor foi feita pelo usuário (MariaDB 11.8.8 com TLSv1.3, CA em `deploy/ca-cert.pem`, `DB_TLS_CA` nos `.env.example`); falta a parte da API, do Jenkinsfile e do docker-compose. **TST-09 desbloqueado** (credencial do `.env` voltou a funcionar). O lote segue com 6 cards: TST-08, SEC-37, SEC-29, DEPLOY-02, TST-09 e SETUP-01. Passo atual: "🟡 BackBrain (SEC-29) e 🔴 TestBrain (SETUP-01) → 🔴 TestBrain (integração TST-09/TST-08) e 🟣 SecBrain revisando SEC-29 → 🔵 SubBrain fechando".
>
> Antes, ainda em 2026-11-01 (abertura do Lote 29, 🔵 SubBrain): o 🤍 MegaBrain abriu o **Lote 29** com 6 cards de prioridade MÉDIA vindos de `afazer.md`: **TST-08, SEC-37, SEC-29, DEPLOY-02, TST-09 e TST-10** (bloco "Lote 29 de 2026-11-01" abaixo). Passo atual: "🟣 SecBrain especificando". O CHORE-12 (rename `apis/` → `backend/`) foi commitado em `c4c87af` (branch `lote-27`; registro em `feito.md`).
>
> Atualizado em 2026-10-31 (fechamento do Lote 28, 🔵 SubBrain): **nenhum card em execução.** O **Lote 28** (SEC-19, SEC-23 e SEC-35) foi concluído e está em `feito.md` ("Lote 28 de 2026-10-31: 3 cards concluídos"; não commitado, pendente do usuário). Dele saíram os cards SEC-37, FE-17, CHORE-11, SEC-38 e LOG-03 em `afazer.md` (bloco "Achados do Lote 28"). Passos anteriores do Lote 28: "🟣 SecBrain especificando → 🟡 BackBrain → 🟢 FrontBrain → verificação do 🤍 MegaBrain → 🔵 SubBrain fechando" (2026-10-31; o 🔴 TestBrain não rodou passo separado, os testes vieram com os devs).
>
> Antes, ainda em 2026-10-31 (abertura do Lote 28, 🔵 SubBrain): o 🤍 MegaBrain abriu o **Lote 28** com 3 cards vindos de `afazer.md`: **SEC-19** (Turnstile sem conferência de `hostname`/`action`) e **SEC-23** (`deve_trocar_senha` imposto pelo backend), ambos do bloco "GAPs de segurança da auditoria ARQ-00" e com backend + frontend + config, e **SEC-35** (`ExcluirPagamento`/`CriarPagamento` fora de lock, bloco "Achados do Lote 27"). Passo atual: "🟣 SecBrain especificando → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → 🔵 SubBrain fechando". O **Lote 27** foi commitado pelo usuário (`a7e2d4a` "Lote 27: limites HTTP, CORS, LIKE, paginação e transações", branch `lote-27`, criado a partir de `main`; registro em `feito.md`).
>
> Antes, ainda em 2026-10-31 (fechamento do Lote 27, 🔵 SubBrain): **nenhum card em execução.** O **Lote 27** (SEC-14, SEC-15, SEC-16, SEC-17, SEC-24, SEC-25, BUG-14, BUG-15 e SEC-26) foi concluído e está em `feito.md` ("Lote 27 de 2026-10-31: 9 cards concluídos"; no fechamento, não commitado). Dele saíram os cards SEC-35, SEC-36, BUG-24, CHORE-10 e TST-11 em `afazer.md` (bloco "Achados do Lote 27"). Passos anteriores do Lote 27: "🟣 SecBrain especificando → 🟡 BackBrain → 🔴 TestBrain → 🔵 SubBrain fechando" (2026-10-31).
>
> Antes, ainda em 2026-10-31 (abertura do Lote 27, 🔵 SubBrain): o 🤍 MegaBrain abriu o **Lote 27** com 9 cards de backend/testes vindos de `afazer.md`: **SEC-14, SEC-15, SEC-16, SEC-17, SEC-24, SEC-25** (GAPs de segurança da auditoria ARQ-00), **BUG-14, BUG-15** (transações em `VincularCliente` e na troca de senha) e **SEC-26** (achado do ARQ-05). Critério: cards de backend/testes de prioridade MÉDIA (mais SEC-16 e SEC-25, BAIXA, que tocam os mesmos pontos), sem decisão de produto pendente. SEC-19 e SEC-23 (backend + frontend + config) ficam para o lote seguinte.
>
> Antes, ainda em 2026-10-31 (fechamento do Lote 26, 🔵 SubBrain): **nenhum card em execução.** O **IMG-01** (imagem do produto) foi concluído e está em `feito.md` ("Lote 26 de 2026-10-31"; não commitado, pendente do usuário). Dele saíram os cards CHORE-09 e SEC-34 em `afazer.md` (bloco "Achados do IMG-01").
>
> Antes, ainda em 2026-10-31 (abertura do Lote 26, 🔵 SubBrain): o 🤍 MegaBrain abriu o **Lote 26** com um card, o **IMG-01** (imagem do produto), pedido direto do usuário (não passou por `afazer.md`). Passos: 🟣 SecBrain especificando → 🌸 DataBrain, 🟡 BackBrain, 🟢 FrontBrain e 🔴 TestBrain → 🔵 SubBrain fechando.
>
> Antes, ainda em 2026-10-31: nenhum card em execução. O DB-05 (`make db-seed-mes-atual`) foi concluído e está em `feito.md` (não commitado, pendente do usuário).
>
> Antes, em 2026-10-30: nenhum card em execução.
>
> Histórico: o **Lote 25** (8 cards: BUG-16, SEC-27, BUG-17, BUG-19, CHORE-03, CHORE-06, BUG-20 e BUG-21), aberto em 2026-10-29 depois das decisões de produto do usuário, foi concluído por 🌸 DataBrain, 🟡 BackBrain, 🟢 FrontBrain e 🔴 TestBrain e fechado pelo 🔵 SubBrain em 2026-10-30. Os 8 cards, com as decisões (originais e refinadas), as extensões do 🤍 MegaBrain, as entregas, os testes e as pendências, estão em `feito.md` ("Lote 25 de 2026-10-30: 8 cards concluídos"; não commitado, pendente do usuário). As pendências viraram os cards TST-09, TST-10, CHORE-08 e FE-16 em `afazer.md` (bloco "Achados do Lote 25"). Passos anteriores do Lote 25: "🌸 DataBrain e 🟡 BackBrain implementando" (2026-10-30), "aguarda o fechamento do Lote 24; especificação pronta" e "🟣 SecBrain especificando" (2026-10-29).
>
> Histórico: o **Lote 24** (SEC-18, SEC-20, SEC-21 e SEC-22, os 4 GAPs de segurança de prioridade ALTA da auditoria ARQ-00) foi fechado em 2026-10-30 e está em `feito.md` (não commitado, pendente do usuário).
>
> Atualização de 2026-10-29 (🔵 SubBrain): o 🟣 SecBrain concluiu a especificação do Lote 25 e o usuário tomou novas decisões a partir dos riscos R1 a R6. BUG-16 e SEC-27 tiveram a decisão refinada (a anterior ficou como histórico em cada card), o BUG-17 passou a incluir o UPDATE atômico no backend, e os riscos R2 e R3 viraram os cards **BUG-20** e **BUG-21** (no Lote 25). R4, R6 e os achados do Lote 24 viraram cards de backlog em `afazer.md` (UX-02, CHORE-07, BUG-22, SEC-32, SEC-33, BUG-23 e TST-08). Antes (2026-10-29): Lote 25 com 6 cards, passo "🟣 SecBrain especificando".
>
> Histórico: o **Lote 23** (DB-03, troca da conexão de MySQL para MariaDB 11.4, escopo "Infra + docs") foi concluído em 2026-10-29 e está em `feito.md` (commitado pelo usuário em `c87a30d` e `186b15f`), junto com o SEC-28 (resolvido pelo DB-03); os achados do 🟣 SecBrain viraram os cards SEC-29, SEC-30, SEC-31, DB-04 e DEPLOY-02 em `afazer.md`. O **Lote 22** (DASH-01, funil de vendas no Dashboard) foi concluído em 2026-10-29 e está em `feito.md`; os achados viraram os cards DOC-06, BUG-19 e FE-15 em `afazer.md`. Em 2026-10-16 o DEPLOY-01 (Lote 13) foi concluído e o MAIL-01 foi cancelado; os dois estão em `feito.md`.

---

## Lote 30 de 2026-11-02 (somente MariaDB + infra/TLS): 16 cards

> Aberto pelo 🤍 MegaBrain em 2026-11-02, com aprovação antecipada do usuário para as decisões pendentes. **Passo atual:** "🟣 SecBrain especificando → devs → 🔴 TestBrain → 🔵 SubBrain fechando".
>
> **Ordem sugerida:** DEPLOY-05 (revisão do script) → CHORE-13, DEPLOY-06, DEPLOY-04 e DOC-08 (código e docs) → TST-15 → SEC-45, SEC-39, SEC-31, SEC-30, SEC-40, SEC-41, SEC-42, TST-12 → DB-04 + DB-06.
>
> **Decisões da abertura (🤍 MegaBrain, 2026-11-02):**
> - **Banco oficial:** somente MariaDB. O caminho oficial do banco em produção é **só o `deploy/mariadb-docker.sh`**; o `deploy/setup-servidor.sh` deixa de cuidar do banco nativo (DEPLOY-06).
> - **DEPLOY-05, escopo neste lote:** revisão e ajustes do `deploy/mariadb-docker.sh` pelo 🌸 DataBrain. A **execução no servidor e a migração dos dados** (itens 2 a 5 da ação esperada) ficam como **pendência do usuário**.
> - **DB-04 e DB-06 tratados juntos:** com prod e dev em MariaDB, alinhar o `sql_mode` (incluindo `ONLY_FULL_GROUP_BY`) nos dois ambientes.
> - **SEC-39:** o `mariadb-docker.sh` já cria as contas com `REQUIRE SSL`; conferir e, com o DEPLOY-06, decidir o destino do `mariadb-setup.sql`. **SEC-31:** fixar por digest também a imagem base `mariadb:<versão>` do `mariadb-docker.sh`.
> - **Limitações do ambiente dos agentes:** sem acesso ao socket do Docker (a parte Docker do TST-15 fica como pendência, igual ao TST-14) e `.env` local apontando para o banco de produção `192.168.168.106:3306` (nenhum `make db-*` nele).
> - Cancelados na triagem: **DOC-07** (substituído pelo DOC-08) e **DEPLOY-03** (sem MySQL), em `feito.md`.

---

## DEPLOY-05: subir o MariaDB em Docker na produção (`deploy/mariadb-docker.sh`) e aposentar o MySQL — prioridade ALTA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camadas:** Database, Deploy
**Responsável:** 🌸 DataBrain (revisão do script e migração dos dados) → usuário (execução no servidor) → 🔴 TestBrain (conferência)
**Origem:** decisão do usuário em 2026-11-02 (somente MariaDB). O script foi escrito em 2026-11-02 e ainda **não rodou contra Docker real** (só `bash -n` e os comandos `openssl` foram testados).

**Escopo no Lote 30 (🤍 MegaBrain, 2026-11-02):** só o item 1 (revisão e ajustes do script pelo 🌸 DataBrain). Os itens 2 a 5 (dump/migração dos dados, troca do MySQL pelo MariaDB na 3306, credenciais do Jenkins/`.env` e deploy) ficam como **pendência do usuário**, registrada no fechamento.

**Descrição:** o `deploy/mariadb-docker.sh` (root, no servidor) gera a CA e o certificado com SAN, monta a imagem `rotaperfumes-mariadb:11.4` (TLS obrigatório e o hardening do DEPLOY-02), sobe o container com root só local e cria `rotaperfumes_app`, `rotaperfumes_admin` e `casaos`, todos `REQUIRE SSL`. A porta 3306 está ocupada pelo MySQL 26.10, então o script aborta até o MySQL parar (ou roda lado a lado com `--porta`).

**Ação esperada:**
1. 🌸 DataBrain revisa o script (contas, grants, cnf, ordem das etapas, reexecução).
2. Se houver dados no MySQL 26.10, gerar um dump e importar no MariaDB. Atenção à collation `utf8mb4_0900_ai_ci` e a outras sintaxes exclusivas do MySQL no dump (o inverso do DEPLOY-03).
3. Parar e remover o container do MySQL e rodar o script na 3306.
4. Atualizar a credencial Jenkins `rotaperfumes-db-admin`, o Secret file `rotaperfumes-api-env` (`rotaperfumes_app`), o `deploy/ca-cert.pem` das máquinas de dev (CA nova `RotaPerfumes_MariaDB_CA`, se o script gerou uma) e os `.env` de dev (`casaos`).
5. Rodar o deploy do Jenkins (estágios "CA do banco" e "Importar dump").

**Critério de aceite:** MariaDB respondendo na 3306 com o certificado da CA do projeto (`openssl s_client -starttls mysql` mostra o emissor `RotaPerfumes_MariaDB_CA` ou `RotaPerfumes_MySQL_CA`); API com `tls=ligado`; `make db-up` de uma máquina de dev com `casaos` passa; MySQL removido do servidor.

---

## CHORE-13: tirar o MySQL do `Makefile`, `.env.example` e `CLAUDE.md` — prioridade MÉDIA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camadas:** Backend (tooling)
**Responsável:** 🟡 BackBrain
**Origem:** decisão do usuário em 2026-11-02. O parâmetro `DB_CLIENTE` (`mariadb`|`mysql`) e o `DB_CLIENTE_BIN` foram criados no mesmo dia, antes da decisão.

**Descrição:** com só MariaDB, o ramo `DB_CLIENTE=mysql` (TLS com `--ssl-mode=VERIFY_IDENTITY` e a checagem de sabor no `db-check-env`) fica sem uso.

**Ação esperada:** remover o ramo `mysql`. Manter `DB_CLIENTE_BIN` (útil para um `mariadb` fora do `PATH` ou num container) e a checagem de que o executável é MariaDB. Ajustar `.env.example` e `CLAUDE.md` e revisar os comentários "MySQL 8" do `Makefile`.

**Critério de aceite:** `grep -i mysql Makefile .env.example CLAUDE.md` só encontra `MYSQL_PWD`/`MYSQL_OPTS` (nomes do protocolo/cliente) e o driver Go; `make -n db-up` igual ao de hoje com `mariadb`.

---

## DEPLOY-06: `setup-servidor.sh`, `mariadb-setup.sql`, `Jenkinsfile` e `docker-compose.yml` só com MariaDB — prioridade MÉDIA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camadas:** Deploy, Database
**Responsável:** 🌸 DataBrain → 🟡 BackBrain
**Origem:** decisão do usuário em 2026-11-02.

**Decisão (🤍 MegaBrain, 2026-11-02, com aprovação antecipada do usuário):** o caminho oficial do banco em produção é **só o `deploy/mariadb-docker.sh`**. O `setup-servidor.sh` **deixa de cuidar do banco nativo** (sem sabor MySQL nem MariaDB no host); o destino do `mariadb-setup.sql` (referência ou remoção) fica a cargo do 🌸 DataBrain, registrado no card.

**Descrição:** o `deploy/setup-servidor.sh` detecta o "sabor" (MySQL 8 ou MariaDB), tem `SQL_MODE_MYSQL`, `mysql.conf.d` e `systemctl ... mysql`. O `Jenkinsfile` e o `docker-compose.yml` comentam "produção = MySQL 8.0 do host". O `deploy/mariadb-setup.sql` supõe o banco no host. O `deploy/dump-local.ps1` também cita o MySQL.

**Ação esperada:** remover o sabor MySQL. Decidir com o usuário se o banco no host continua suportado ou se o caminho oficial é só o `mariadb-docker.sh` (se for, o `setup-servidor.sh` chama o `mariadb-docker.sh` ou deixa de cuidar do banco, e o `mariadb-setup.sql` vira referência ou sai). Atualizar os comentários do `Jenkinsfile`/compose (o banco agora é o container `rotaperfumes-mariadb`, ainda acessado por `host.docker.internal`/`SITE_IP`).

**Critério de aceite:** nenhum caminho de código de deploy trata MySQL; `bash -n` e os testes do `tests/deploy` verdes; decisão sobre o banco no host registrada.

---

## DEPLOY-04: modo completo do `setup-servidor.sh` recusa o Jenkins em contêiner; numeração dos passos no `--apenas-banco` — prioridade MÉDIA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camadas:** Infra (deploy)
**Responsável:** 🌸 DataBrain
**Origem:** DEPLOY-02 (Lote 29, 2026-11-01).

**Nota da abertura (2026-11-02):** executar junto com o DEPLOY-06. Como o `setup-servidor.sh` deixa de cuidar do banco nativo, o modo `--apenas-banco` perde o sentido atual; o 🌸 DataBrain define o que acontece com ele (remover, ou apontar para o `mariadb-docker.sh`) e numera os passos conforme os modos que restarem.

**Descrição:** no modo completo, o `checar_jenkins` do `deploy/setup-servidor.sh` recusa o Jenkins em contêiner, que é o da produção; hoje só o `--apenas-banco` serve no servidor real. Além disso, no `--apenas-banco` a numeração dos passos pula (`[1/11]`, `[3/11]`...).

**Ação esperada:** aceitar o Jenkins em contêiner no `checar_jenkins` (detectar o contêiner e ajustar o que depende do Jenkins nativo); numerar os passos conforme o modo.

**Critério de aceite:** modo completo roda com o Jenkins em contêiner; numeração contínua nos dois modos; `docs/deploy-servidor.md` atualizado.

---

## DOC-08: documentação só com MariaDB (substitui o DOC-07) — prioridade MÉDIA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente. O DOC-07 foi cancelado na abertura do lote (`feito.md`).
**Camada:** Documentação
**Responsável:** 🔵 SubBrain
**Origem:** decisão do usuário em 2026-11-02. O DOC-07 pedia o contrário (alinhar a documentação com a produção MySQL 8.0).

**Descrição:** citam MySQL: `docs/Instal_SSL.md` (passos para o MySQL 8.0 do host), `docs/deploy-servidor.md`, `docs/manual-base-de-dados.md`, `docs/arquitetura-ddd.md`, `docs/roteiro-teste-manual-*.md`, `postman/README.md`, `deploy/api.env.example`, `FalhasEncontradas.md` e os comentários dos `sql/*.sql`.

**Ação esperada:** reescrever o `Instal_SSL.md` em torno do `deploy/mariadb-docker.sh` (geração, renovação com `--novo-cert`, `FLUSH SSL`, rollback). Documentar o `casaos` e o `.env` de dev. Revisar as demais menções. Fechar o DOC-07.

**Critério de aceite:** `grep -ril "mysql 8\|mysql:8\|mysql 26"` em `docs/` e `deploy/` sem resultado (exceto histórico explícito); o `Instal_SSL.md` descreve o fluxo real.

---

## TST-15: testes sem MySQL e teste do `deploy/mariadb-docker.sh` — prioridade MÉDIA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camada:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** decisão do usuário em 2026-11-02.

**Nota da abertura (2026-11-02):** nesta máquina o usuário dos agentes não tem acesso ao socket do Docker (mesmo bloqueio do TST-14). A parte sem Docker (`bash -n`, `shellcheck`, remoção dos casos MySQL) roda no lote; a execução completa com Docker fica com skip e vira pendência do usuário (rodar junto com o TST-14).

**Descrição:** testes que tratam o sabor MySQL ou citam MySQL 8: `SH/tests/deploy/deploy02_hardening_test.go`, `SH/tests/db/sec29_tls_handshake_test.go`, `SH/tests/config/sec11_makefile_sql_test.go`, `SH/tests/db/mask_dsn_test.go`, `SH/tests/tools/seedusers/seedusers_test.go` e `SH/tests/analytics/mysql_test.go` (conferir se é só o nome do adapter). O `mariadb-docker.sh` não tem teste.

**Ação esperada:** remover os casos de MySQL junto com o CHORE-13/DEPLOY-06. Criar um teste do `mariadb-docker.sh`: `bash -n`, `shellcheck` e, com Docker (pular sem ele, como o TST-14), uma execução completa numa porta e num datadir temporários: contas criadas, TLS verificado aceito, conexão sem TLS recusada, root sem host remoto, reexecução sem mudar senhas e `--novo-cert` com `FLUSH SSL`.

**Critério de aceite:** `make test` verde; o teste com Docker rodado ao menos uma vez e registrado.

---

## SEC-45: revisão de segurança do `deploy/mariadb-docker.sh` — prioridade MÉDIA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente. Correções de risco já aprovadas antecipadamente pelo usuário; o que depender do servidor fica como pendência do usuário.
**Camada:** Segurança
**Responsável:** 🟣 SecBrain
**Origem:** script criado em 2026-11-02 (decisão de somente MariaDB).

**Descrição:** pontos a avaliar:
- a porta publicada pelo Docker passa por fora do `ufw`, e a 3306 fica aberta para toda a LAN;
- `casaos` tem `ALL PRIVILEGES` em `rotaperfumes.*` para a LAN inteira (`192.168.168.0/255.255.255.0`);
- a chave da CA fica em `/root/rotaperfumes-ca/ca-key.pem` no próprio servidor;
- o `root.cnf` (senha do root) é montado read-only no container;
- `rotaperfumes_app`/`rotaperfumes_admin` valem para toda a faixa `172.16.0.0/12` (qualquer container do host pode tentar);
- a imagem base não está fixada por digest (SEC-31);
- autenticação ed25519 (SEC-30).

**Ação esperada:** parecer com as correções por camada (ex.: regra na chain `DOCKER-USER` restringindo a 3306, `casaos` por IP ou com grants menores, CA offline).

**Critério de aceite:** parecer registrado e correções aplicadas ou recusadas pelo usuário.

---

## SEC-39: `REQUIRE SSL` nas contas do banco criadas pelos scripts de setup — prioridade MÉDIA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente (executar só depois do TLS ativo no servidor e do primeiro deploy com `tls=ligado`).
**Camadas:** Database (deploy)
**Responsável:** 🌸 DataBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, revisão do SEC-29 (Lote 29, 2026-11-01).

**Triagem (2026-11-02, 🤍 MegaBrain):** o `deploy/mariadb-docker.sh` já cria as contas com `REQUIRE SSL`. Com o DEPLOY-06 (o `setup-servidor.sh` deixa de cuidar do banco), o card se resume a conferir o `mariadb-docker.sh` (inclusive o `ALTER USER ... REQUIRE SSL` idempotente na reexecução) e alinhar o `mariadb-setup.sql`, se ele continuar como referência.

**Descrição:** o SEC-29 deixou a API e o pipeline conectando com TLS, mas `deploy/mariadb-setup.sql` e `deploy/setup-servidor.sh` criam `rotaperfumes_app` e `rotaperfumes_admin` sem `REQUIRE SSL`. Hoje o `REQUIRE SSL` fica como passo manual do usuário (`docs/deploy-servidor.md` §3.5).

**Ação esperada:** `CREATE USER ... REQUIRE SSL` nos dois usuários e `ALTER USER ... REQUIRE SSL` idempotente para os que já existem; rodar só quando o servidor tiver TLS (conferir antes e avisar se não tiver, para não trancar a API fora).

**Critério de aceite:** contas criadas ou reconfiguradas com `REQUIRE SSL`; reexecução sem erro; conexão sem TLS recusada para as duas contas; docs atualizadas.

---

## SEC-31: fixar a imagem `mariadb:11.4` do Jenkins por digest — prioridade BAIXA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camadas:** Infra (CI/CD)
**Responsável:** 🌸 DataBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, DB-03 (Lote 23, 2026-10-29).

**Triagem (2026-11-02, 🤍 MegaBrain):** fixar por digest também a imagem base `mariadb:<versão>` usada pelo `deploy/mariadb-docker.sh` (ponto do SEC-45).

**Descrição:** o `Jenkinsfile` usa `DB_CLIENT_IMAGE = 'mariadb:11.4'` no estágio "Importar dump". A tag é mutável: uma imagem trocada no registry rodaria com a senha do `rotaperfumes_admin` no ambiente.

**Ação esperada:** fixar a imagem por digest (`mariadb:11.4@sha256:...`), registrar no comentário do `Jenkinsfile` como atualizar o digest e incluir essa atualização na rotina do `make deps-audit` (mensal e antes de deploy). Avaliar fazer o mesmo com `golang:1.26-alpine`.

**Critério de aceite:** imagem referenciada por digest; procedimento de atualização documentado; pipeline verde.

---

## SEC-30: autenticação ed25519 nas contas do banco — prioridade BAIXA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camadas:** Database (deploy), Testes
**Responsável:** 🌸 DataBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, DB-03 (Lote 23, 2026-10-29).

**Nota da abertura (2026-11-02):** com somente MariaDB, o ed25519 vale para todos os ambientes. O ponto de aplicação passa a ser o `deploy/mariadb-docker.sh` (e o `mariadb-setup.sql`, se continuar como referência), não mais o `setup-servidor.sh` (DEPLOY-06). A migração das contas no servidor real é pendência do usuário (junto com o DEPLOY-05).

**Descrição:** `deploy/mariadb-setup.sql` e `deploy/setup-servidor.sh` criam `rotaperfumes_app` e `rotaperfumes_admin` com o plugin padrão (`mysql_native_password`, hash SHA1). O MariaDB oferece o plugin `ed25519`, mais forte.

**Ação esperada:** avaliar `INSTALL SONAME 'auth_ed25519'` e `IDENTIFIED VIA ed25519 USING PASSWORD('...')` nas duas contas; confirmar que o `go-sql-driver/mysql` (versão do `go.mod`) e o cliente `mariadb:11.4` do Jenkins autenticam com ed25519; manter a idempotência do setup (usuário já existente) e documentar a migração das contas atuais.

**Critério de aceite:** contas da API e do admin com ed25519 no servidor; API e estágio "Importar dump" funcionando; procedimento documentado em `docs/deploy-servidor.md`.

---

## SEC-40: confiança na CA do workspace e no parâmetro `DB_TLS_CA_PATH` do Jenkins — prioridade BAIXA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camadas:** Infra (CI/CD)
**Responsável:** 🟡 BackBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, revisão do SEC-29 (Lote 29, 2026-11-01).

**Descrição:** o estágio "CA do banco" do `Jenkinsfile` lê a CA do caminho do parâmetro `DB_TLS_CA_PATH`. Quem consegue mudar o parâmetro do job (ou o arquivo no workspace) pode apontar para outra CA, e uma troca de CA passa sem aviso.

**Ação esperada:** restringir o `realpath` do parâmetro a `/etc/mysql/ssl/` (ou tornar o caminho constante) e avisar quando o fingerprint da CA mudar em relação ao último build.

**Critério de aceite:** caminho fora de `/etc/mysql/ssl/` recusado (ou caminho fixo); troca de fingerprint gera aviso visível no build; procedimento de rotação em `docs/deploy-servidor.md` §3.5 ajustado.

**Nota (2026-11-01, SEC-44):** o `DB_TLS_CA_PATH` foi substituído por `DB_TLS_CA_HOST_PATH` (padrão `/etc/mysql/ssl/ca.pem`), e a CA agora é validada num contêiner descartável (`deploy/validar-ca-banco.sh`), com o sha256 em `deploy/db-ca.sha256`. Reavaliar o card sobre o novo parâmetro; o `db-ca.sha256` pode servir de base para o aviso de troca de fingerprint.

**Nota da abertura (2026-11-02):** conferir o diretório permitido conforme onde o `deploy/mariadb-docker.sh` publica a CA (pode deixar de ser `/etc/mysql/ssl/`); alinhar com o DEPLOY-06 e o DOC-08.

---

## SEC-41: `deploy/dump-local.ps1` conecta sem TLS — prioridade BAIXA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camadas:** Infra (deploy)
**Responsável:** 🌸 DataBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, revisão do SEC-29 (Lote 29, 2026-11-01).

**Descrição:** o `deploy/dump-local.ps1` não usa `DB_TLS_CA`; o dump trafega em claro e, com `REQUIRE SSL` (SEC-39), deixaria de funcionar.

**Ação esperada:** ler `DB_TLS_CA` (e `DB_TLS_SERVER_NAME`, se aplicável) e passar `--ssl-ca` com `--ssl-verify-server-cert` (ou `--ssl-mode=VERIFY_IDENTITY`, conforme o cliente) ao cliente do banco.

**Critério de aceite:** com `DB_TLS_CA` definida, o dump usa TLS com verificação; sem ela, comportamento atual com aviso; documentado.

**Nota da abertura (2026-11-02):** com somente MariaDB, usar a sintaxe do cliente `mariadb` (`--ssl-ca` + `--ssl-verify-server-cert`); tirar as menções a MySQL do script junto com o DEPLOY-06. O `pwsh` não está disponível nesta máquina (ver TST-13 cancelado); registrar a validação que for possível.

---

## SEC-42: aviso (ou `DB_TLS_REQUIRED=true`) quando o TLS está desligado com `DB_HOST` remoto — prioridade BAIXA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camada:** Backend (shared config)
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, revisão do SEC-29 (Lote 29, 2026-11-01).

**Descrição:** em `SH/config/db_tls.go`, sem `DB_TLS_CA` a conexão segue sem TLS em silêncio (só o log `tls=desligado`), mesmo com `DB_HOST` fora do loopback. Um `api.env` sem a variável faria a produção voltar a trafegar em claro.

**Ação esperada:** com `DB_HOST` não-loopback e TLS desligado, logar um aviso explícito no startup; avaliar a variável `DB_TLS_REQUIRED=true` para a API recusar subir nesse caso (decisão do 🟣 SecBrain).

**Critério de aceite:** aviso (ou recusa com `DB_TLS_REQUIRED=true`) testado; dev local com loopback sem mudança; documentado em `.env.example` e `deploy/api.env.example`.

---

## TST-12: testes de TLS que faltaram no SEC-29 — prioridade BAIXA

**Status:** em execução (Lote 30, 2026-11-02). Antes: pendente.
**Camada:** Testes
**Responsável:** 🔴 TestBrain (🟣 SecBrain decide o `MinVersion`)
**Origem:** 🟣 SecBrain e 🔴 TestBrain, SEC-29 (Lote 29, 2026-11-01).

**Descrição:** os testes do SEC-29 cobrem CA errada, CA vencida, nome fora do SAN e servidor sem TLS, mas não cobrem certificado do **servidor** vencido nem servidor que só aceita TLS 1.1.

**Ação esperada:** acrescentar os dois casos (servidor TLS falso no teste); avaliar subir o `MinVersion` de TLS 1.2 para TLS 1.3 (o servidor já negocia TLSv1.3).

**Critério de aceite:** os dois casos recusados com erro claro; decisão sobre o `MinVersion` registrada; testes verdes.

---

## DB-04: avaliar `ONLY_FULL_GROUP_BY` no `sql_mode` — prioridade BAIXA

**Status:** em execução (Lote 30, 2026-11-02), **junto com o DB-06**. Antes: pendente.
**Camadas:** Database, Backend (SQL dos adapters), Testes
**Responsável:** 🌸 DataBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, DB-03 (Lote 23, 2026-10-29).

**Decisão da abertura (🤍 MegaBrain, 2026-11-02):** DB-04 e DB-06 são tratados juntos. Com prod e dev em MariaDB, alinhar o `sql_mode` nos dois ambientes (drop-in da imagem do `deploy/mariadb-docker.sh` e MariaDB de dev), avaliando ligar `ONLY_FULL_GROUP_BY` nos dois. A suíte de integração não roda no banco de produção (o `.env` local aponta para ele); a conferência das consultas usa um banco de dev/descartável ou fica como pendência do usuário.

**Descrição:** o drop-in do `deploy/setup-servidor.sh` grava `sql_mode = STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION`, sem `ONLY_FULL_GROUP_BY` (que o MySQL 8 tinha por padrão). Sem ele, um `GROUP BY` incompleto devolve valores arbitrários em vez de erro.

**Ação esperada:** revisar as consultas com `GROUP BY` (principalmente `analyticsmysql` e as métricas de clientes), rodar a suíte de integração com `ONLY_FULL_GROUP_BY` ligado e, se tudo passar, incluí-lo no drop-in e na documentação. Se alguma consulta falhar, corrigi-la antes.

**Critério de aceite:** decisão registrada (incluir ou não, com motivo); se incluído, consultas compatíveis, testes de integração verdes e `docs/deploy-servidor.md` atualizado.

---

## DB-06: `ONLY_FULL_GROUP_BY` diferente entre dev (MariaDB, sem) e produção (MySQL 8, com) — prioridade MÉDIA

**Status:** em execução (Lote 30, 2026-11-02), **junto com o DB-04**. Antes: pendente.
**Camadas:** Database, Testes
**Responsável:** 🌸 DataBrain → 🔴 TestBrain
**Origem:** DEPLOY-02 (Lote 29, 2026-11-01): o drop-in de hardening mantém `ONLY_FULL_GROUP_BY` no MySQL 8 da produção, mas o dev (MariaDB 11.8.8) roda sem ele.

**Triagem (2026-11-02, 🤍 MegaBrain):** com somente MariaDB, a opção "rodar a integração contra um MySQL 8" sai. Fica o alinhamento do `sql_mode` entre prod e dev, ambos MariaDB (ver decisão no DB-04).

**Descrição:** a suíte de integração roda contra o dev, sem `ONLY_FULL_GROUP_BY`. Um `GROUP BY` incompleto passa nos testes e falha só na produção (MySQL 8). Relacionado ao DB-04 (que avaliava ligar o modo no MariaDB).

**Ação esperada:** alinhar o `sql_mode` do dev com o da produção (ligar `ONLY_FULL_GROUP_BY` no MariaDB de dev) ou rodar a integração também contra um MySQL 8 (ex.: contêiner `mysql:8.0`). Corrigir as consultas que falharem antes.

**Critério de aceite:** integração rodando com `ONLY_FULL_GROUP_BY` ligado e verde; decisão registrada (alinhar o dev ou rodar contra MySQL 8); DB-04 atualizado ou fechado junto.

---

## Lote 31 de 2026-11-02 (backend/frontend, BAIXA): 24 cards

> Aberto pelo 🤍 MegaBrain em 2026-11-02, com aprovação antecipada do usuário para as decisões de produto/contrato pendentes. **Passo atual:** "🟣 SecBrain especificando → devs → 🔴 TestBrain → 🔵 SubBrain fechando". Caminhos: `SH` = `backend/shared`, `API` = `backend/rotaperfumes-api`, `FE` = `frontend`.
>
> **Decisões da abertura (2026-11-02):**
> - **CHORE-07:** textos novos aprovados antecipadamente pelo usuário; o 🟣 SecBrain/🟡 BackBrain propõem e registram os textos finais no card.
> - **SEC-33:** troca dos nomes dos cookies para `__Host-…` aprovada antecipadamente (inclusive a queda das sessões ativas no deploy); definir no card como fica o dev local sem `Secure`.
> - **SEC-34, SEC-38, SEC-36, BUG-24 e CHORE-09:** as escolhas pendentes ficam com o 🟣 SecBrain/🤍 MegaBrain, já aprovadas pelo usuário; registrar a opção escolhida em cada card.
> - **DEP-03:** só a conferência do upstream entra neste lote; o card fica em `afazer.md` como acompanhamento se não houver correção.
> - Ordem de execução sugerida: backend/shared primeiro (BUG-18, SEC-32, BUG-23, BUG-22, BUG-24, CHORE-08, CHORE-09, SEC-34, SEC-36, SEC-38, LOG-03, CHORE-05, CHORE-07, SEC-33, CHORE-10, DOC-06), depois frontend (FE-15, UX-02, FE-17, CHORE-11, partes de frontend de CHORE-07/CHORE-10/BUG-22/SEC-33) e testes (TST-06, TST-07, CHORE-04), com o FE-16 (`next build`) por último.

---

## TST-06: asserções de tipo sem checagem nos testes de handler — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente (fora do escopo do Lote 21).
**Camada:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** 🔴 TestBrain, revisão do ARQ-05 (Lote 21, 2026-10-15).

**Descrição:** os testes de handler usam asserções de tipo sem checagem, como `body["data"].(map[string]any)`. Se a resposta vier em outro formato, o teste dá panic, aborta o pacote inteiro e esconde as falhas dos testes seguintes.

**Ação esperada:** trocar por `require.IsType` ou pela forma `v, ok := …` com `require.True(t, ok)`, em todas as suítes de handler. Sem enfraquecer nenhuma asserção.

**Critério de aceite:** nenhuma asserção de tipo sem checagem nos testes de handler (conferir por busca); mesma quantidade de testes, todos verdes.

---

## BUG-18: Analytics, IN sem limite nas metas e detecção de "tabela não existe" por texto amplo — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente (fora do escopo do Lote 21).
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** revisão do ARQ-06 (Analytics/Dashboard) pelo 🟣 SecBrain, Lote 21 (2026-10-15).

**Descrição:**
- consulta de metas monta `IN` sem limite (`marcadores(len(ids))`), com tamanho igual ao nº de vendedores ativos;
- a detecção de "tabela não existe" em `SH/analytics/analyticsmysql/filtros.go:~92-100` compara texto amplo da mensagem (inclui `not found`), podendo mascarar outros erros.

**Ação esperada:** limitar/fatiar o `IN` (ou trocar por subconsulta/JOIN); detectar a tabela ausente via `errors.As` em `*mysql.MySQLError` com código 1146. Testes para os dois pontos.

**Critério de aceite:** `IN` com tamanho limitado ou eliminado; só o código 1146 é tratado como tabela ausente; contrato HTTP inalterado; testes verdes.

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

---

## TST-07: estender o teste de AST de concatenação SQL aos demais adapters — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente (fora do escopo do Lote 21).
**Camada:** Testes
**Responsável:** 🔴 TestBrain (🟡 BackBrain se algum adapter precisar de ajuste)
**Origem:** sugestão do 🔴 TestBrain, Lote 21 ARQ-07 (2026-10-16).

**Descrição:** `backend/rotaperfumes-api/tests/arquitetura/arq07_sql_concat_test.go` detecta concatenação com `+` em textos SQL passados a `QueryContext`/`QueryRowContext`/`ExecContext`, mas hoje varre só o `analyticsmysql`.

**Ação esperada:** estender a varredura aos demais adapters MySQL (`catalogomysql`, `comercialmysql`, `vendasmysql`, identidade etc.), com allowlist dos helpers que recebem SQL pronto do chamador: `backend/shared/db/linhas.go`, `backend/shared/db/consulta.go`, `paginar` em `comercialmysql/linhas.go` e em `vendasmysql/linhas.go`, e `vinculo` em `comercialmysql/carteiras.go`. Manter um controle negativo (caso que o teste precisa reprovar).

**Critério de aceite:** todos os adapters MySQL cobertos; allowlist explícita e mínima; controle negativo reprovando; testes verdes.

---

## DOC-06: comentário "escopo por carteira" das rotas de Oportunidades em `rotas.go` — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camada:** Backend (só comentário)
**Responsável:** 🟡 BackBrain
**Origem:** 🟣 SecBrain, revisão do DASH-01 (Lote 22, 2026-10-29).

**Descrição:** em `backend/rotaperfumes-api/internal/bootstrap/rotas.go:130`, o comentário do bloco de Oportunidades diz "acesso comum, escopo por carteira", mas o código filtra por vendedor (`oportunidades.vendedor_id`), não pela carteira ativa do cliente. O DASH-01 usou o mesmo filtro por vendedor. O comentário engana quem for revisar o escopo.

**Ação esperada:** corrigir o comentário para "escopo por vendedor (`oportunidades.vendedor_id`)". Aproveitar e conferir se os comentários equivalentes das linhas 116 (Pedidos) e 137 (Visitas) descrevem o filtro real; corrigir só o texto, sem mudar código.

**Critério de aceite:** comentários de `rotas.go` batem com o filtro aplicado em cada contexto; build e testes verdes (nenhuma mudança de comportamento).

---

## FE-15: `FunilVendas`, aviso de desligado sem `forcarZeros` e seleção da janela após erro — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain → 🔴 TestBrain
**Origem:** 🔴 TestBrain, testes do DASH-01 (Lote 22, 2026-10-29).

**Descrição:** dois pontos em `frontend/src/components/dashboard/FunilVendas.tsx`:
1. **Aviso de desligado (linha ~252):** o aviso próprio do componente ("Vendedor desligado. Nao ha oportunidades no funil...") também dispara pela sessão (`useVendedorDesligado`) quando o componente é usado **sem** `forcarZeros`. No Dashboard isso não acontece, porque a página passa `forcarZeros` nesses casos e mostra o aviso dela. Mas, se o componente for reutilizado em outra tela, o aviso pode aparecer duplicado ou fora de contexto.
2. **Erro na troca de janela:** depois de um erro ao trocar a janela, o botão novo fica marcado (`aria-pressed`), mas as barras continuam com os dados da janela anterior, sem indicar isso.

**Ação esperada:** (1) definir a regra do aviso quando o componente não recebe `forcarZeros` (ex.: só pelo `vendedor_desligado` do payload, ou um prop explícito); (2) depois do erro, voltar a seleção para a janela dos dados exibidos, ou indicar de qual janela são os dados. Testes para os dois casos.

**Critério de aceite:** comportamento definido e testado nos dois pontos; nenhuma regressão no Dashboard (testes DASH-01 e FE-02 verdes); `tsc` e `eslint` limpos.

---

## UX-02: orientar o ajuste manual de estoque depois do snapshot do faturamento (409 → usar o PUT) — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente (avanço parcial no Lote 25).
**Camadas:** Frontend (Docs/Postman)
**Responsável:** 🟢 FrontBrain → 🔴 TestBrain → 🔵 SubBrain
**Origem:** 🟣 SecBrain, especificação do Lote 25 (risco R4, 2026-10-29).

**Descrição:** o upsert do faturamento cria o snapshot de estoque do dia para o SKU. Depois disso, um ajuste manual (POST de estoque) para o mesmo SKU e dia bate no índice `uk_estoque_data_sku`. Com o CHORE-06 (Lote 25), essa resposta passa de 500 para 409 "registro de estoque já cadastrado para este sku nesta data", mas o usuário não entende por que o registro "já existe" se ele não o criou.

**Ação esperada:** em `frontend/src/app/admin/estoque/page.tsx` (modal de ajuste), no 409 orientar a editar o registro existente (PUT), por exemplo com a mensagem "já existe registro deste SKU nesta data (pode ter sido criado pelo faturamento); edite o registro existente", e, se possível, um atalho para abrir o registro. Documentar o comportamento no `postman/README.md` e no manual da tela de estoque.

**Critério de aceite:** o 409 do POST de estoque mostra a orientação e não a mensagem genérica; teste de frontend do caso; docs atualizadas. Depende do CHORE-06 concluído.

**Andamento (2026-10-30, Lote 25):** o CHORE-06 foi concluído, e a tela de estoque já acrescenta "Edite o registro existente." à mensagem do 409; o `postman/README.md` documenta o caso (inclusive o registro criado pelo faturamento). Falta: o atalho para abrir o registro existente, o texto mencionando o faturamento na tela e o manual da tela de estoque.

---

## CHORE-07: mensagens "apenas o status" / "apenas ter o status alterado" imprecisas depois do BUG-16 — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente (mudança de texto = mudança de contrato; exige aprovação do usuário). **Aprovação:** o usuário aprovou antecipadamente a troca dos textos em 2026-11-02 ("estou aprovando elas antecipadamente"); registrar os textos finais neste card.
**Camadas:** Backend, Frontend (mapeamento de mensagens), Testes, Docs/Postman
**Responsável:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → 🔵 SubBrain
**Origem:** 🟣 SecBrain, especificação do Lote 25 (risco R6, 2026-10-29).

**Descrição:** as mensagens existentes em `SH/vendas/erros.go:38` ("…apenas o status") e `erros.go:42` ("…apenas ter o status alterado") dizem que um pedido faturado pode ter o status alterado. Depois do BUG-16 (Faturado só vai para Entregue; Entregue é terminal), isso fica enganoso. O BUG-16 **não** muda esses textos, porque mudar a mensagem é mudança de contrato HTTP não aprovada.

**Ação esperada:** propor ao usuário os textos novos, alinhados à regra do BUG-16; com a aprovação, trocar os textos, atualizar o `friendlyPedidoError` do frontend, os testes que conferem a mensagem exata, o Postman (exemplo "…apenas ter o status alterado") e o `postman/README.md`.

**Critério de aceite:** textos aprovados pelo usuário; mensagens coerentes com a regra de status; testes, frontend, Postman e docs atualizados.

---

## BUG-22: toggle de ativo de clientes e usuários lê e inverte (mesmo padrão do BUG-17) — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camadas:** Backend, Frontend, Testes
**Responsável:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, especificação do Lote 25 (BUG-17, 2026-10-29).

**Descrição:** o toggle de ativo de clientes (contexto Comercial) e de usuários (contexto Identidade) usa o mesmo padrão do produto antes do BUG-17: lê o registro, inverte o valor em memória e grava. Dois toggles concorrentes leem o mesmo valor e se anulam (lost update). O BUG-17 corrigiu só o produto.

**Ação esperada:** aplicar o mesmo padrão do BUG-17: `UPDATE … SET ativo = NOT ativo WHERE id = ?` atômico quando não houver valor explícito no body, `DefinirAtivo` quando houver, e releitura para a resposta; bloqueio de 1500 ms no botão das telas, se aplicável. Atenção às regras próprias desses toggles (escopo/IDOR de clientes do SEC-01; revogação de sessão do usuário inativado do SEC-06/SEC-08; BUG-05) para não perder nenhuma delas. O toggle de usuário está no contexto Identidade, mexido no Lote 24: executar depois do merge dele.

**Critério de aceite:** toggles concorrentes não se anulam (teste de integração); regras de escopo e de revogação de sessão preservadas; contrato HTTP inalterado; testes verdes.

---

## SEC-32: hash vazio ou malformado no banco permite enumerar contas pelo tempo de resposta — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** Lote 24 (2026-10-29), achado do SEC-22.

**Descrição:** em `SH/identidade/identidadecrypto/hasher.go:89-96`, quando o hash gravado no banco está vazio ou malformado (formato não reconhecido), a verificação retorna sem rodar o Argon2. A resposta continua 401 "credenciais inválidas", mas sai mais rápido, o que permite enumerar essas contas pelo tempo de resposta (quebra parcial da anti-enumeração do SEC-22).

**Ação esperada:** quando o hash não for reconhecido, rodar o hash descartável (mesmo custo do caminho normal) antes de responder; log de decisão sem o hash nem o e-mail.

**Critério de aceite:** com hash vazio ou malformado, o hash descartável roda e a resposta é idêntica (mensagem e status) à de senha errada; teste que confirma a execução do hash nesse caminho; testes do SEC-22 verdes.

---

## SEC-33: prefixo `__Host-` nos nomes dos cookies de sessão — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente (muda os nomes dos cookies; avaliar impacto com o usuário). **Aprovação:** o usuário aprovou antecipadamente a troca dos nomes dos cookies em 2026-11-02 (inclusive a queda das sessões ativas no deploy); registrar a decisão sobre o ambiente local neste card.
**Camadas:** Backend, Frontend (se ler os nomes), Testes, Docs/Postman
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain → 🔵 SubBrain
**Origem:** Lote 24 (2026-10-29), achado do SEC-21.

**Descrição:** os cookies de sessão (access e refresh) não usam o prefixo `__Host-`. O prefixo faz o navegador exigir `Secure`, `Path=/` e ausência de `Domain`, o que impede que um subdomínio ou uma conexão sem HTTPS plante ou sobrescreva o cookie (cookie tossing).

**Ação esperada:** avaliar a troca dos nomes para `__Host-…`. Impactos: muda os nomes (todas as sessões ativas caem no deploy; conferir o frontend, o Caddy, o Postman e os testes que citam os nomes) e exige `Secure` **sempre**, inclusive em dev (o `COOKIE_SECURE=false` do SEC-21 deixaria de funcionar com o prefixo; definir como fica o ambiente local). Conferir o `Path` do cookie de refresh (o `__Host-` exige `Path=/`).

**Critério de aceite:** decisão registrada; se aprovada, cookies com `__Host-`, `Secure`, `Path=/` e sem `Domain`; logout limpa os cookies novos (e os antigos, na transição); testes, Postman e docs atualizados.

---

## BUG-23: refresh de usuário apagado responde 500 — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 24 (2026-10-29), achado do SEC-22.

**Descrição:** em `SH/identidade/identidadeapp/sessao.go:37-41`, se o usuário dono do refresh token não existir mais (apagado do banco), a releitura do usuário falha e a resposta é 500, em vez da resposta genérica de sessão inválida usada para inativo (SEC-22).

**Ação esperada:** tratar "usuário não encontrado" no refresh como sessão inválida: 401 genérico, revogar o refresh token e limpar os cookies, no mesmo padrão do inativo; log de decisão só com o id.

**Critério de aceite:** refresh de usuário apagado → 401 genérico (sem 500); token revogado; teste do caso; testes do SEC-22 verdes.

---

## CHORE-08: observações do 🔴 TestBrain no Lote 25 (log, vlog, NaN e 1062 de carteiras) — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camadas:** Backend (shared), Testes
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa o item 4)
**Origem:** 🔴 TestBrain, testes do Lote 25 (2026-10-30).

**Descrição:** quatro pontos menores, nenhum com impacto no contrato atual:
1. **Log "quitado" impreciso na corrida** (`SH/vendas/vendasapp/pagamentos.go:164`): quando o UPDATE condicional afeta 0 linhas e a releitura decide o resultado, o log de bloqueio diz "quitado" mesmo em casos que não são quitação (conferir o caminho exato).
2. **Rótulo `status_faturado` do vlog ambíguo** (`SH/vendas/vendasapp/uow.go:67-72`): o mesmo rótulo é usado para recusas de naturezas diferentes da máquina de status (Faturado, Entregue, Entregue sem Faturado), o que dificulta ler o log.
3. **`TaxaPct` NaN aceito no domínio** (`SH/vendas/pagamento.go:147`): a checagem `TaxaPct < 0 || TaxaPct > TaxaPctMaxima` é falsa para NaN. Pelo JSON o NaN não chega, mas o domínio deveria recusar (`math.IsNaN`/`IsInf`).
4. **1062 em qualquer índice de carteiras vira 409** (`SH/comercial/comercialmysql/carteiras.go:28`, `qualquerIndice = ""`): inclusive um 1062 de PRIMARY, que deveria ser 500 como em produtos e estoque (CHORE-06).

**Ação esperada:** (1) e (2) ajustar o texto/rótulo do log para refletir a decisão real (só ids, sem dados sensíveis); (3) recusar NaN e ±Inf em `taxa_pct` com o mesmo erro 400; (4) detectar o índice pelo nome (`uk_carteiras_cliente_vendedor_inicio`) com `db.ChaveDuplicada`, deixando outros 1062 como 500.

**Critério de aceite:** testes de cada item (incluindo 1062 PRIMARY de carteiras → 500 e NaN → 400 no domínio); testes de vlog (TST-05) verdes; contrato HTTP inalterado nos casos normais.

---

## FE-16: rodar o `next build` do frontend depois do Lote 25 — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente. Rodar por último no lote, depois das mudanças de frontend do Lote 31 (Node 22 via nvm já disponível, SETUP-01).
**Camadas:** Frontend
**Responsável:** 🟢 FrontBrain (ou 🔴 TestBrain)
**Origem:** Lote 25 (2026-10-30).

**Descrição:** no Lote 25 o frontend passou em testes, `tsc` e `eslint` (espelho com Node 22), mas o `next build` não foi rodado. Mudaram `PedidoModal.tsx`, `PagamentoModal.tsx`, `ui/Select.tsx` (prop `helperText`), `app/admin/pedidos/page.tsx`, `app/pagamentos/page.tsx`, `app/admin/produtos/page.tsx` e `app/admin/estoque/page.tsx`.

**Ação esperada:** rodar `next build` (no ambiente com Node >= 20; ver TST-10) e corrigir o que aparecer; registrar o resultado.

**Critério de aceite:** `next build` sem erros; resultado registrado no card do Lote 25 em `feito.md`.

---

## CHORE-09: job de limpeza de imagens órfãs de produto — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camadas:** Backend (shared + API), Testes, Documentação
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** IMG-01 (Lote 26, 2026-10-31): decisão D5 do 🤍 MegaBrain na especificação e achado do 🔴 TestBrain sobre o commit ambíguo.

**Descrição:** o diretório `PRODUTO_IMAGENS_DIR` pode acumular arquivos sem referência em `produtos.imagem`:
1. **Remoção da imagem antiga que falha depois do commit** (é best-effort e só gera log).
2. **Commit com erro ambíguo:** o driver devolve erro, mas o commit foi aplicado. O caso de uso apaga o arquivo novo e o banco fica apontando para um arquivo que não existe (a leitura pública responde `404`). Achado do 🔴 TestBrain, convertido em card pelo 🤍 MegaBrain.
3. **`.tmp-*` antigos** de gravações interrompidas (queda do processo entre a criação do temporário e o rename), inclusive os `.tmp-probe-*` do teste de escrita da subida.
4. **Revert da migração 26** (`make db-revert-produtos-imagem`), que zera os vínculos e deixa todos os arquivos órfãos.

**Ação esperada:** tarefa periódica (no padrão de `identidadeapp.IniciarLimpezaPeriodica`/CHORE-02, com intervalo por env e `0` para desativar) ou comando `cmd/` com `-dry-run`, que:
- liste os arquivos do diretório pelo `*os.Root` do adapter `catalogofs`, sem seguir symlinks;
- apague só os nomes que batem com o regex `<uuid-v4>.(jpg|png|webp)` e não estão em `produtos.imagem`, e os `.tmp-*` com mais de N horas, sempre com uma idade mínima (para não apagar um arquivo cuja transação ainda está aberta);
- trate o caso 2 (banco apontando para arquivo inexistente): logar e, por decisão do 🤍 MegaBrain, zerar a coluna ou só reportar;
- logue totais, sem nomes do cliente (não existem) e sem caminhos absolutos.

**Critério de aceite:** testes com diretório temporário cobrindo órfão apagado, referenciado preservado, `.tmp-*` antigo apagado e recente preservado, nome fora do regex ignorado, symlink ignorado e `-dry-run` sem efeito; documentação em `docs/manual-base-de-dados.md` §13.1 e `postman/README.md` ("Imagens de produto").

---

## SEC-34: WebP de produto não é reencodado (EXIF e bytes anexados ficam no arquivo) — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente. A escolha entre as opções 1 a 3 fica com o 🟣 SecBrain (pré-aprovada pelo usuário em 2026-11-02); se mudar a extensão do `imagem_url`, registrar a mudança de contrato.
**Camadas:** Backend (shared), Testes
**Responsável:** 🟣 SecBrain (decisão) → 🟡 BackBrain → 🔴 TestBrain
**Origem:** IMG-01 (Lote 26, 2026-10-31): decisão D1 do 🤍 MegaBrain e achado do 🔴 TestBrain (WebP com HTML anexado é gravado inteiro).

**Descrição:** o adapter `SH/catalogo/catalogoimagem/normalizador.go` reencoda JPEG e PNG depois do decode (EXIF, GPS e bytes anexados são descartados), mas o WebP é gravado como veio, porque `golang.org/x/image` só decodifica WebP. Ficam no arquivo público: metadados EXIF/XMP (ex.: localização da foto) e qualquer payload anexado depois dos dados da imagem (achado do 🔴 TestBrain: WebP com HTML anexado aceito e servido inteiro). O risco de execução é mitigado por `X-Content-Type-Options: nosniff`, CSP `default-src 'none'`, `Content-Type` fixo `image/webp` e `Content-Disposition: inline`; o 🤍 MegaBrain aceitou o risco no IMG-01.

**Ação esperada (decidir uma):**
1. adotar um encoder WebP (avaliar dependência, CGO, licença e govulncheck) e reencodar como nos outros formatos;
2. converter o WebP recebido para PNG (sem dependência nova; o `imagem_url` passaria a terminar em `.png` e o arquivo pode crescer);
3. no mínimo, remover os chunks `EXIF`/`XMP ` do contêiner RIFF e truncar o arquivo no tamanho declarado no cabeçalho RIFF.

**Critério de aceite:** WebP com EXIF e com bytes anexados gravado sem eles (teste comparando os bytes servidos); limite de 1 MB e validação atuais preservados; contrato HTTP documentado se a extensão mudar.

---

## SEC-36: origens de dev (`localhost:3000`) sempre na allowlist de CORS, inclusive em produção — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camada:** Backend (config/middleware) + Deploy
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, especificação do Lote 27 (2026-10-31), achado fora do escopo nº 2 (levantado durante o SEC-17).

**Descrição:** `defaultDevOrigins` (`http://localhost:3000` e `http://127.0.0.1:3000`, `SH/config/config.go:496`) entra sempre na allowlist de CORS com `Access-Control-Allow-Credentials: true`, também em produção, e vale para o `uploadPermitido` do IMG-01. Um processo local na máquina do usuário na porta 3000 faria requisições autenticadas com os cookies de produção. Além disso, o middleware envia `Access-Control-Expose-Headers: Set-Cookie`, que não tem efeito (o navegador nunca expõe `Set-Cookie` ao JS).

**Ação esperada:** incluir `defaultDevOrigins` só em dev (ex.: quando `COOKIE_SECURE=false`, critério a confirmar pelo 🟣 SecBrain); remover `Set-Cookie` do `Access-Control-Expose-Headers`; documentar em `.env.example` e `deploy/api.env.example`.

**Critério de aceite:** em configuração de produção, `Origin: http://localhost:3000` não recebe `Access-Control-Allow-Origin` nem passa no `uploadPermitido`; em dev continua funcionando; o header `Expose-Headers` não cita `Set-Cookie`; testes de config e de middleware.

---

## BUG-24: troca de senha concorrente com reset admin sobrescreve uma à outra — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camada:** Backend (shared)
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, especificação do Lote 27 (2026-10-31), achado fora do escopo nº 3 (levantado durante o BUG-15).

**Descrição:** depois do BUG-15 cada fluxo de senha é atômico, mas dois fluxos simultâneos para o mesmo usuário (troca da própria senha e reset admin) leem o hash atual fora da tx e gravam sem condição: a última gravação vence e o histórico pode registrar o `HashAnterior` errado (o hash lido antes da outra troca).

**Ação esperada:** `AtualizarSenha` condicionado ao hash lido (`... WHERE id = ? AND senha_hash = ?`), como já faz o `RegravarHash`; 0 linhas afetadas → erro de conflito (409 ou nova tentativa, decisão do 🟣 SecBrain), com rollback do histórico e da revogação na mesma tx.

**Critério de aceite:** teste com duas trocas concorrentes (ou sqlmock com 0 linhas afetadas) mostrando que só uma vence e que o histórico grava o `HashAnterior` correto; caminho normal sem mudança de contrato.

---

## CHORE-10: `ErrImagemMuitoGrande` com `KindMuitoGrande` e `maxLength={100}` nas buscas do frontend — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camadas:** Backend (shared + API), Frontend, Testes
**Responsável:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, especificação do Lote 27 (2026-10-31): achado fora do escopo nº 4 (opcional) e item "Frontend (opcional)" do SEC-25.

**Descrição:**
1. O SEC-14 criou `kernel.KindMuitoGrande` (→ 413), mas `catalogo.ErrImagemMuitoGrande` continua com outro `Kind` e o handler de imagem trata o 413 num caso especial de `responderErroImagem` (`API/internal/catalogohttp/imagem_handler.go`).
2. O SEC-25 limitou `q` a 100 caracteres (acima disso, 400), mas os campos de busca do frontend não têm limite, e o usuário só descobre o limite pelo erro.

**Ação esperada:** passar `catalogo.ErrImagemMuitoGrande` para `kernel.KindMuitoGrande` e remover o caso especial de `responderErroImagem`, mantendo a mensagem "imagem excede 1 MB"; colocar `maxLength={100}` nos campos de busca das telas de listagem (produtos, estoque, clientes, visitas, oportunidades, pedidos).

**Critério de aceite:** upload acima de 1 MB continua 413 "imagem excede 1 MB" (`img01_imagem_handler_test.go` verde); campos de busca não aceitam mais de 100 caracteres (teste de frontend); contrato HTTP inalterado.

---

## FE-17: depois de trocar a senha, o frontend vai para `/dashboard` com a sessão já cortada — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, especificação do Lote 28 (2026-10-31), achado fora do escopo nº 2 (levantado durante o SEC-23).

**Descrição:** depois de uma troca bem-sucedida, `FE/src/app/trocar-senha/page.tsx` (~linha 133) redireciona para `/dashboard`, mas o backend já gravou o corte de sessão (SEC-08). Resultado: `401` → refresh falha → `/login?redirect=...`, com uma passagem pelo dashboard e a mensagem de sessão expirada.

**Ação esperada:** depois do sucesso, `clearTokens()` + `router.replace("/login")` com a mensagem "Senha alterada. Faça login com a nova senha.", sem chamar o dashboard.

**Critério de aceite:** teste da página de trocar senha: no sucesso, tokens limpos e redirecionamento direto para `/login` com a mensagem; nenhuma chamada autenticada depois da troca; fluxo SEC-23 (403 → `/trocar-senha`) continua funcionando.

---

## CHORE-11: remover `apiResetPassword` morta do frontend — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, especificação do Lote 28 (2026-10-31), achado fora do escopo nº 3.

**Descrição:** `apiResetPassword` (`FE/src/lib/api.ts`, ~linha 175) envia `usuario_id`, que o backend ignora (`POST /api/auth/reset-password` troca sempre a senha do próprio usuário). A função não é usada e engana quem lê o código; a troca real usa `apiChangePassword`.

**Ação esperada:** remover `apiResetPassword` (e testes/mocks que só existam por causa dela), mantendo `apiChangePassword`.

**Critério de aceite:** `grep apiResetPassword frontend/src` vazio; `tsc --noEmit`, `eslint` e `vitest` verdes; sem mudança de contrato HTTP.

---

## SEC-38: `posseDoPagamento`/`posseDoPagamentoTravada` escondem erro de banco como 404 — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camada:** Backend (shared)
**Responsável:** 🟣 SecBrain (decisão) → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, especificação do Lote 28 (2026-10-31), achado fora do escopo nº 4 (registro); entregas do 🟡 BackBrain (estendido à `posseDoPagamentoTravada` criada no SEC-35).

**Descrição:** pela invariante V-02 (`docs/arq-00-invariantes-seguranca.md`, ARQ-05), qualquer erro ao buscar o pedido de um pagamento vira `404` `ErrPagamentoNaoEncontrado`. Isso vale também para erros de infraestrutura (banco fora, timeout, deadlock no lock do SEC-35), que ficam invisíveis para o cliente e para o monitoramento.

**Ação esperada:** avaliar (🟣 SecBrain) responder `500` com log para erros que não sejam "não encontrado", como já faz `posseParaEditar`, mantendo `404` para inexistente e para fora da carteira. Se aprovado, atualizar a V-02.

**Critério de aceite:** decisão registrada; se mudar, testes com erro de banco em `posseDoPagamento` e `posseDoPagamentoTravada` → `500` com log, inexistente/fora da carteira → `404`; contrato documentado no `postman/README.md`.

---

## LOG-03: recusa do captcha logada sem o motivo — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente.
**Camada:** Backend (shared)
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** entregas do 🟡 BackBrain no Lote 28 (2026-10-31), decisão fora da spec nº 6 (SEC-19).

**Descrição:** depois do SEC-19, um token pode ser recusado por `success=false`, hostname inesperado ou action inesperada, mas o log de recusa (`identidadeapp/casos.go`, `verificarCaptcha`) é genérico. Em produção não dá para saber, por exemplo, que o hostname do frontend ficou fora de `TURNSTILE_HOSTNAMES`.

**Ação esperada:** logar só o motivo (`success`, `hostname` ou `action`), **sem** os valores recebidos, sem token e sem secret; a resposta HTTP continua `400` `"captcha inválido ou ausente"`.

**Critério de aceite:** testes mostrando o motivo no log para cada caso e a ausência do token, da secret, do hostname e da action recebidos; contrato HTTP inalterado.

---

## CHORE-04: normalizar formatação (gofmt/CRLF) de 3 testes SEC12 de integração — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente (fora do escopo do Lote 21).
**Camada:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** piloto ARQ-02 (Lote 21, 2026-10-14), achado do 🟡 BackBrain.

**Descrição:** os arquivos abaixo não passam no `gofmt` (formatação/finais de linha CRLF):
- `backend/shared/tests/repositories/sec12_chore02_refresh_integration_test.go`
- `backend/rotaperfumes-api/tests/handlers/sec12_http_integration_test.go`
- `backend/rotaperfumes-api/tests/services/sec12_chore02_integration_test.go`

**Ação esperada:** rodar `gofmt -w` e normalizar para LF (conferir `.gitattributes`), sem alterar a lógica, em commit separado.

**Critério de aceite:** `gofmt -l` vazio nos 3 arquivos; mesmos testes passando.

---

## CHORE-05: logs de auditoria de escrita registram só o role, não o id do usuário — prioridade BAIXA

**Status:** em execução (Lote 31, 2026-11-02). Antes: pendente (fora do escopo do Lote 21; executar **após o ARQ-03**, quando o Principal estiver disponível em todos os contextos). O ARQ-03 já foi concluído (Lote 21).
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** NB-4 do 🟣 SecBrain, revisão do ARQ-02 (Lote 21, 2026-10-14).

**Descrição:** os logs de auditoria de escrita registram só o role de quem fez a ação, por exemplo `[produtos] criado: id=%d por usuario role=%s`. Sem o id do usuário, não dá para saber quem alterou o registro.

**Ação esperada:** incluir o id do Principal em todos os logs de auditoria de escrita (create, update, delete e toggle) de todos os contextos, mantendo o role. Não registrar PII (e-mail ou nome).

**Pontos acrescentados no ARQ-03 (2026-10-14):** `backend/shared/identidade/identidadeapp/senha.go:169` e `backend/rotaperfumes-api/internal/identidadehttp/usuario_handler.go:59,82,106` registram `por admin=<role>` em vez do id.

**Critério de aceite:** todos os logs de escrita trazem o id e o role do usuário; testes de log por contexto; contrato HTTP inalterado.

---

## Lote 32 de 2026-11-02: 1 card

> Aberto pelo 🤍 MegaBrain em 2026-11-02, com aprovação antecipada do usuário para as decisões pendentes. **Passo atual:** "🟣 SecBrain especificando → devs → 🔴 TestBrain → 🔵 SubBrain fechando". Executar depois dos Lotes 30 e 31 (mexe no domínio de Vendas e possivelmente no schema).

---

## ARQ-09: valores monetários em centavos (`int64`) — prioridade BAIXA/MÉDIA

**Status:** em execução (Lote 32, 2026-11-02). Antes: pendente (sai do Lote 21; executar após o ARQ-08, em lote próprio). Uma eventual mudança do contrato JSON está coberta pela aprovação antecipada do usuário (2026-11-02), mas a preferência continua sendo o contrato inalterado; registrar a decisão no card.
**Camadas:** Database, Backend (Frontend se o formato do JSON mudar)
**Responsável:** 🌸 DataBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** decisão P3 do 🤍 MegaBrain no ARQ-00 (2026-10-13).

**Descrição:** no ARQ-05, o value object `Dinheiro` permanece `float64` encapsulado. Este card migra o cálculo interno para centavos (`int64`) e avalia o schema (colunas DECIMAL) e o contrato JSON.

**Risco:** arredondamento em `valor_total` (soma de itens, descontos e pagamentos). Os valores calculados podem divergir em centavos dos já gravados; é preciso um teste de regressão comparando os totais antigos com os novos e uma regra de arredondamento explícita.

**Nota N-9 (🟣 SecBrain, ARQ-01, 2026-10-13):** `kernel.Dinheiro.Somar` pode resultar em `+Inf` em overflow do `float64`. Revalidar neste card (com `int64`, checar overflow na soma).

**Nota da abertura (2026-11-02):** se houver migração de schema, o 🌸 DataBrain não a aplica no banco de produção a partir desta máquina (o `.env` local aponta para `192.168.168.106:3306`); a aplicação no servidor fica como pendência do usuário/deploy.

**Critério de aceite:** cálculos monetários sem `float64`; totais existentes reproduzidos sem diferença; contrato JSON inalterado (ou mudança aprovada pelo usuário); testes verdes sem perda de cobertura.
