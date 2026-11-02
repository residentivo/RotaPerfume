# A Fazer

> Atualizado em 2026-11-02 (abertura dos Lotes 30, 31 e 32, 🔵 SubBrain). O usuário pediu para executar **todas** as pendências, aprovou antecipadamente as decisões de produto/contrato pendentes nos cards (inclusive os textos do **CHORE-07** e os nomes de cookies do **SEC-33**) e pediu, ao final, o merge de todas as branches na `main` e o commit da `main`, para testar o site em produção. Saíram desta lista para `fazendo.md`:
> - **Lote 30 de 2026-11-02 (somente MariaDB + infra/TLS):** DEPLOY-05, CHORE-13, DEPLOY-06, DEPLOY-04, DOC-08, TST-15, SEC-45, SEC-39, SEC-31, SEC-30, SEC-40, SEC-41, SEC-42, TST-12, DB-04 e DB-06 (DB-04 e DB-06 tratados juntos).
> - **Lote 31 de 2026-11-02 (backend/frontend, BAIXA):** TST-06, BUG-18, TST-07, DOC-06, FE-15, UX-02, CHORE-07, BUG-22, SEC-32, SEC-33, BUG-23, CHORE-08, FE-16, CHORE-09, SEC-34, SEC-36, BUG-24, CHORE-10, FE-17, CHORE-11, SEC-38, LOG-03, CHORE-04 e CHORE-05.
> - **Lote 32 de 2026-11-02:** ARQ-09 (valores monetários em centavos).
>
> **Triagem do 🤍 MegaBrain (somente MariaDB):** **DOC-07** cancelado (substituído pelo DOC-08) e **DEPLOY-03** cancelado (sem MySQL), ambos em `feito.md` com o motivo. **Ficam aqui:** **TST-14** (bloqueado: o usuário desta máquina não tem acesso ao socket do Docker), **TST-04** (bloqueado: o `.env` local aponta para o banco de produção, e os agentes não rodam `make db-*` nele) e **DEP-03** (acompanhamento do upstream; a conferência entra no Lote 31 e, sem correção publicada, o card continua aqui). Os blocos de origem que ficaram sem cards saíram desta lista; o histórico está nas notas abaixo e em cada card (`fazendo.md`).
>
> Antes, em 2026-11-02 (pedido do usuário, sem execução nesta rodada). **Somente MariaDB:** o MySQL deixa de ser usado. Entrou o bloco "Migração para somente MariaDB", logo antes dos GAPs da ARQ-00, com **DEPLOY-05** (subir o MariaDB em Docker com `deploy/mariadb-docker.sh` e aposentar o MySQL, ALTA), **CHORE-13** (`Makefile`/`.env.example`/`CLAUDE.md`), **DEPLOY-06** (`setup-servidor.sh`, `Jenkinsfile`, compose), **DOC-08** (docs; substitui o DOC-07), **TST-15** (testes) e **SEC-45** (revisão do script), todos MÉDIA. O 🤍 MegaBrain deve fazer a triagem de DOC-07, DB-06, DEPLOY-03, TST-14, SEC-39 e SEC-31 ao abrir o lote.
>
> Atualizado em 2026-11-01 (fechamento do Lote 29, 🔵 SubBrain). O **Lote 29** foi fechado (`feito.md`; não commitado, pendente do usuário). Entraram quatro cards no bloco "Achados do Lote 29", logo após o DOC-07: **DB-06** (`ONLY_FULL_GROUP_BY` diferente entre dev MariaDB e prod MySQL 8, MÉDIA), **DEPLOY-03** (dump do MariaDB 11.x recusado pelo MySQL 8, MÉDIA), **DEPLOY-04** (`setup-servidor.sh` completo recusa Jenkins em contêiner; numeração dos passos, MÉDIA) e **TST-14** (`TestDEPLOY02_ValidateConfig_Docker` com Docker, BAIXA). O **TST-11** saiu desta lista (concluído). O DOC-07 ganhou nota sobre o `IMPORTAR_DUMP` e o SEC-40 ganhou nota sobre o `DB_TLS_CA_HOST_PATH`.
>
> Antes, ainda em 2026-11-01 (novas decisões do usuário no Lote 29, 🔵 SubBrain). Saíram desta lista para `feito.md`: **TST-13** (cancelado pelo usuário: "não irei conferir esse script agora") e **SEC-43** (fechado como "não corrigir": ambiente de dev, risco aceito pelo usuário). Entrou o **DOC-07** (MÉDIA, bloco "Achados do Lote 29"): revisar docs, `CLAUDE.md` e `setup-servidor.sh` para a realidade (prod MySQL 8.0, dev MariaDB). O card novo **SEC-44** (TLS do SEC-29 no ambiente real) foi direto para o Lote 29 (`fazendo.md`).
>
> Antes, ainda em 2026-11-01 (fechamento parcial do Lote 29, 🔵 SubBrain). SEC-29, TST-09, TST-08 e SETUP-01 foram concluídos (`feito.md`); SEC-37 e DEPLOY-02 seguem no Lote 29 (`fazendo.md`). Dos concluídos saíram sete cards, no bloco "Achados do Lote 29", logo após o LOG-03: **SEC-39** (`REQUIRE SSL` nos scripts de setup, MÉDIA), **SEC-40** (confiança na CA do workspace/`DB_TLS_CA_PATH`, BAIXA), **SEC-41** (`deploy/dump-local.ps1` sem TLS, BAIXA), **SEC-42** (aviso com TLS desligado e host remoto, BAIXA), **SEC-43** (credencial fraca no `.env` local, MÉDIA, ação do usuário), **TST-12** (testes de TLS faltantes, BAIXA) e **TST-13** (validar os scripts de setup no Windows e o MariaDB 11.4 no Linux, MÉDIA). O TST-11 ganhou nota de andamento (flake corrigido no TST-09; falta conferir e fechar).
>
> Antes, ainda em 2026-11-01 (decisões do usuário no Lote 29, 🔵 SubBrain). Backlog sem cards novos nem removidos: o **TST-10** foi cancelado (em `feito.md`), o **SETUP-01** (scripts de instalação do ambiente) entrou direto no Lote 29 (`fazendo.md`) e o **TST-09** foi desbloqueado (credencial do `.env` voltou a funcionar). No FE-16, a referência ao TST-10 passa a valer para o SETUP-01 (Node 22 via nvm).
>
> Antes, ainda em 2026-11-01 (abertura do Lote 29, 🔵 SubBrain). O 🤍 MegaBrain abriu o **Lote 29**, a pedido do usuário, com os 6 cards de prioridade MÉDIA do backlog, que saíram desta lista para `fazendo.md`: **TST-08** (bloco "Achados do Lote 24"), **SEC-29** e **DEPLOY-02** (bloco "Achados do DB-03"), **TST-09** e **TST-10** (bloco "Achados do Lote 25") e **SEC-37** (bloco "Achados do Lote 28"). O TST-09 segue bloqueado pela credencial do banco (depende do usuário). O CHORE-12 (rename `apis/` → `backend/`) foi commitado em `c4c87af` (branch `lote-27`).
>
> Antes, em 2026-10-31 (fechamento do Lote 28, 🔵 SubBrain). O **Lote 28** (SEC-19, SEC-23 e SEC-35) foi concluído e está em `feito.md` (não commitado, pendente do usuário). Dele saíram cinco cards, no bloco "Achados do Lote 28", logo após o TST-11: **SEC-37** (secret de teste do Turnstile aceita em produção, MÉDIA), **FE-17** (depois da troca de senha o frontend vai para `/dashboard` com a sessão já cortada, BAIXA), **CHORE-11** (`apiResetPassword` morta no frontend, BAIXA), **SEC-38** (`posseDoPagamento`/`posseDoPagamentoTravada` escondem erro de banco como 404, BAIXA) e **LOG-03** (recusa do captcha logada sem o motivo, BAIXA). O bloco "GAPs de segurança da auditoria ARQ-00" fica todo concluído.
>
> Antes, ainda em 2026-10-31 (abertura do Lote 28, 🔵 SubBrain). O 🤍 MegaBrain abriu o **Lote 28** e 3 cards saíram desta lista para `fazendo.md`: **SEC-35** (bloco "Achados do Lote 27") e **SEC-19** e **SEC-23** (bloco "GAPs de segurança da auditoria ARQ-00", que fica sem cards pendentes). O **Lote 27** foi commitado pelo usuário (`a7e2d4a`, branch `lote-27`).
>
> Antes, ainda em 2026-10-31 (fechamento do Lote 27, 🔵 SubBrain). O **Lote 27** (SEC-14, SEC-15, SEC-16, SEC-17, SEC-24, SEC-25, BUG-14, BUG-15 e SEC-26) foi concluído e está em `feito.md` (não commitado, pendente do usuário). Dele saíram cinco cards, no bloco "Achados do Lote 27", logo após o SEC-34: **SEC-35** (`ExcluirPagamento`/`CriarPagamento` checam fora de lock, MÉDIA), **SEC-36** (origens de dev sempre na allowlist de CORS, BAIXA), **BUG-24** (corrida entre troca e reset de senha, BAIXA), **CHORE-10** (`ErrImagemMuitoGrande` com `KindMuitoGrande` e `maxLength={100}` nas buscas do frontend, BAIXA) e **TST-11** (teste instável do IMG-01, BAIXA). SEC-19 e SEC-23 continuam no bloco "GAPs de segurança da auditoria ARQ-00".
>
> Antes, ainda em 2026-10-31 (abertura do Lote 27, 🔵 SubBrain). O 🤍 MegaBrain abriu o **Lote 27** e 9 cards saíram desta lista para `fazendo.md`: **BUG-14** e **BUG-15** (topo da lista), **SEC-26** (bloco "Achados do ARQ-05") e **SEC-14, SEC-15, SEC-16, SEC-17, SEC-24 e SEC-25** (bloco "GAPs de segurança da auditoria ARQ-00"). Critério: cards de backend/testes de prioridade MÉDIA (mais SEC-16 e SEC-25, BAIXA, que tocam os mesmos pontos), sem decisão de produto pendente. SEC-19 e SEC-23 (backend + frontend + config) continuam aqui, para o lote seguinte.
>
> Antes, ainda em 2026-10-31 (fechamento do Lote 26, 🔵 SubBrain). O **IMG-01** (imagem do produto) foi concluído e está em `feito.md` (não commitado, pendente do usuário). Dele saíram dois cards BAIXA, no bloco "Achados do IMG-01", logo após o FE-16: **CHORE-09** (job de limpeza de imagens órfãs, inclusive `.tmp-*` antigos e o caso de commit ambíguo) e **SEC-34** (WebP não é reencodado; EXIF e bytes anexados ficam no arquivo).
>
> Antes, ainda em 2026-10-31 (🔵 SubBrain). O 🤍 MegaBrain abriu o **Lote 26** com o card **IMG-01** (imagem do produto), pedido direto do usuário que foi para `fazendo.md` sem passar por esta lista. O backlog abaixo não mudou.
>
> Antes, em 2026-10-30 (fechamento do Lote 25, 🔵 SubBrain). O **Lote 25** (BUG-16, SEC-27, BUG-17, BUG-19, CHORE-03, CHORE-06, BUG-20 e BUG-21) foi concluído e está em `feito.md` (não commitado, pendente do usuário). Dele saíram quatro cards, no bloco "Achados do Lote 25", logo após o TST-08: **TST-09** (3 testes de integração e conferência do CHECK `chk_pagamentos_taxa_pct`, bloqueados pela credencial do banco; depende do usuário, MÉDIA), **TST-10** (`make test-frontend`/`make frontend-deps` em disco exFAT com Node 18, MÉDIA; card próprio, separado do TST-08), **CHORE-08** (4 observações do 🔴 TestBrain, BAIXA) e **FE-16** (rodar o `next build`, BAIXA). O UX-02 avançou em parte (a orientação "Edite o registro existente." no 409 do estoque entrou com o CHORE-06).
>
> Antes, em 2026-10-29 (especificação do Lote 25). Da especificação do 🟣 SecBrain para o Lote 25 saíram três cards BAIXA, no bloco "Achados da especificação do Lote 25", logo após o DEPLOY-02: **UX-02** (R4, UX do ajuste manual de estoque após o snapshot do faturamento), **CHORE-07** (R6, mensagens imprecisas de `vendas/erros.go`) e **BUG-22** (toggle de ativo de clientes e usuários). Dos achados do Lote 24 saíram, no bloco seguinte ("Achados do Lote 24"), **SEC-32** (enumeração por tempo com hash vazio/malformado, BAIXA), **SEC-33** (prefixo `__Host-` nos cookies, BAIXA), **BUG-23** (refresh de usuário apagado → 500, BAIXA) e **TST-08** (`make test` falha com `covdata`, MÉDIA). Os riscos R2 e R3 viraram BUG-20 e BUG-21 dentro do Lote 25 (`fazendo.md`).
>
> Antes, ainda em 2026-10-29: o **Lote 25** (BUG-16, SEC-27, BUG-17, BUG-19, CHORE-03 e CHORE-06) foi aberto pelo 🤍 MegaBrain depois de o usuário tomar as decisões de produto pendentes nesses cards; os seis saíram desta lista para `fazendo.md`, onde ficam na fila até o fechamento do Lote 24 (decisões registradas em cada card). No bloco "Achados do ARQ-05" ficam SEC-26 e TST-06; no bloco "Achados do DASH-01" ficam DOC-06 e FE-15. O **Lote 23** foi commitado pelo usuário (`c87a30d` e `186b15f`).
>
> Ainda em 2026-10-29: o **Lote 24** (SEC-18, SEC-20, SEC-21 e SEC-22, os 4 GAPs de segurança de prioridade ALTA da auditoria ARQ-00) foi aberto e os quatro cards saíram desta lista para `fazendo.md`. No bloco "GAPs de segurança da auditoria ARQ-00" ficam SEC-14 a SEC-17, SEC-19 e SEC-23 a SEC-25.
>
> O **Lote 23** (DB-03, MySQL → MariaDB 11.4) foi concluído e está em `feito.md`. Da avaliação de riscos do DB-03 saíram cinco cards no bloco "Achados do DB-03", logo após o FE-15: **SEC-29** (TLS Go ↔ MariaDB, MÉDIA), **SEC-30** (ed25519, BAIXA), **SEC-31** (imagem do Jenkins por digest, BAIXA), **DB-04** (`ONLY_FULL_GROUP_BY`, BAIXA) e **DEPLOY-02** (hardening do servidor mesmo com bind já aberto, MÉDIA). O SEC-28 foi resolvido pelo DB-03 e saiu desta lista (está em `feito.md`, Lote 23).
>
> O **Lote 22** (DASH-01, funil de vendas no Dashboard) foi concluído e está em `feito.md`. Da revisão do DASH-01 saíram três cards BAIXA, no bloco "Achados do DASH-01", logo após o TST-07: **DOC-06** (comentário "escopo por carteira" em `rotas.go`, do 🟣 SecBrain), **BUG-19** (`arredondar2` com negativos, do 🔴 TestBrain) e **FE-15** (aviso de desligado e seleção da janela após erro no `FunilVendas`, do 🔴 TestBrain). A limitação do setup local em disco sem symlink (`npm ci --no-bin-links`) ficou só documentada em `postman/README.md` ("Subindo o ambiente"), sem card.
>
> Em 2026-10-16: o Lote 21 (2026-10-13 a 2026-10-16, reestruturação da API em DDD + Clean Code + Design Patterns) foi **concluído**: ARQ-00 a ARQ-08 estão em `feito.md` (estrutura final em `docs/arquitetura-ddd.md` §10; ADR em `docs/adr-001-reestruturacao-ddd.md`). Os cards abaixo saíram do lote e ficam fora do escopo dele. Os cards SEC-14 a SEC-25 são GAPs de segurança levantados pelo 🟣 SecBrain na auditoria do ARQ-00 (`docs/arq-00-invariantes-seguranca.md`), fora do escopo do Lote 21. Também saíram do ARQ-00 os cards ARQ-09 (Dinheiro em centavos), BUG-14 (VincularCliente sem transação) e BUG-15 (troca de senha sem transação), no topo da lista. Do ARQ-05 (2026-10-15) saíram BUG-16, SEC-26, SEC-27, TST-06 e BUG-17, no bloco "Achados do ARQ-05", logo após o BUG-15. Do ARQ-06 (2026-10-15) saiu o BUG-18, logo após o BUG-17. Do ARQ-07 (2026-10-16) saíram SEC-28 e TST-07, logo após o BUG-18.
>
> **Regras do Lote 21 (decisões de 2026-10-13, ver ARQ-00 em `feito.md`), que continuam valendo para o código daqui em diante:**
> - **P1, logs (altera a regra LOG-02):** `vlog` em 4 níveis: rota automática no router, caso de uso via `defer`, pontos de decisão e 1 linha por query (~600 chamadas no lugar de ~3.726). Mesmo formato de log; TST-05 deve continuar verde. Nesses cards, o critério "logs `vlog` preservados" significa formato e TST-05 preservados e cobertura pelos 4 níveis, não a manutenção de cada chamada atual. Regra no código: comentário de `backend/shared/vlog/vlog.go`; documentação: `docs/logs-verbose.md` (atualizada no ARQ-08).
> - **X1, contrato HTTP:** a única exceção ao "contrato HTTP inalterado" é a do ARQ-01 (métodos das 13 rotas, `/api/senha-historico/2/extra` e, pelo parse estrito de `{user_id...}` (R-f), `/api/senha-historico/0` e `/-1` passando a 400).
>
> Histórico anterior (2026-10-11): o 🔴 TestBrain executou `docs/roteiro-teste-manual-lote12.md` (24 aprovados, 1 falhou, 13 não verificáveis de 38; ver a seção "Execução (2026-10-07, 🔴 TestBrain)" no fim do roteiro). Daí saíram os cards BUG-13, LOG-01, DOC-05 e TST-04. BUG-13, LOG-01 e DOC-05 foram concluídos no Lote 16 (2026-10-07; ver `feito.md`). O LOG-02 (Lote 17) gerou o TST-05, e o DEP-00 (Lote 19) gerou o DEP-01 e o DEP-02. No Lote 20 (2026-10-11; ver `feito.md`) foram concluídos DEP-01, DEP-02 e TST-05; o TST-04 avançou parcialmente (só faltam 1.1, 1.2 e 1.7, que dependem do usuário rodar o `make`) e do DEP-01 saiu o card DEP-03 (BAIXA). Em 2026-10-16 o DEPLOY-01 (Lote 13) foi concluído e o MAIL-01 foi cancelado (sem servidor de e-mail; a API usa o envio Noop). Os dois estão em `feito.md`.

---

## Achados do Lote 29 (TST-14; os demais foram para o Lote 30 ou cancelados)

> Origem comum: revisão do 🟣 SecBrain e entregas do 🟡 BackBrain e do 🔴 TestBrain no Lote 29 (SEC-29, TST-09, TST-08 e SETUP-01, 2026-11-01; ver `feito.md`). Nenhum deles bloqueou o fechamento desses cards. Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário. Caminhos: `SH` = `backend/shared`.
>
> **2026-11-02 (abertura do Lote 30):** SEC-39, SEC-40, SEC-41, SEC-42, TST-12, DB-06 e DEPLOY-04 foram para o **Lote 30** (`fazendo.md`). DOC-07 (substituído pelo DOC-08) e DEPLOY-03 (sem MySQL) foram **cancelados** (`feito.md`). Aqui fica só o TST-14, bloqueado.
>
> **2026-11-01 (decisões do usuário):** TST-13 cancelado e SEC-43 fechado como "não corrigir" (ambos em `feito.md`). DOC-07 entrou aqui (divergência MariaDB 11.4 × MySQL 8.0 em produção). SEC-39 a SEC-42 e TST-12 devem considerar a produção MySQL 8.0 e o SEC-44 (concluído, em `feito.md`).
>
> **2026-11-01 (fechamento do Lote 29):** entraram DB-06, DEPLOY-03, DEPLOY-04 e TST-14 (achados do SEC-44 e do DEPLOY-02), logo após o DOC-07.

---

## TST-14: rodar `TestDEPLOY02_ValidateConfig_Docker` num ambiente com Docker — prioridade BAIXA

**Status:** pendente, **bloqueado** (2026-11-02): o usuário desta máquina não tem acesso ao socket do Docker (`/var/run/docker.sock`), então o teste continua com skip aqui. Depende do usuário (rodar numa máquina com Docker ou liberar o acesso ao socket).
**Camada:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** DEPLOY-02 (Lote 29, 2026-11-01): o teste foi pulado por falta de Docker.

**Descrição:** `TestDEPLOY02_ValidateConfig_Docker` (`tests/deploy/deploy02_hardening_test.go`) confere os drop-ins com `mysqld --validate-config`/`mariadbd --help --verbose` em contêineres reais, mas não rodou.

**Ação esperada:** rodar o teste num ambiente com Docker, contra `mysql:8.0` e `mariadb:11.8`; corrigir o que aparecer.

**Critério de aceite:** teste rodado sem skip e verde nas duas imagens; resultado registrado no card do DEPLOY-02 em `feito.md`.

**Triagem (2026-11-02, 🤍 MegaBrain, somente MariaDB):** rodar **só com `mariadb`** (a imagem `mysql:8.0` sai do teste junto com o TST-15/DEPLOY-06, no Lote 30). O critério de aceite passa a ser "verde na imagem `mariadb`". Quando o Docker estiver acessível, rodar junto com a parte Docker do TST-15 (teste do `deploy/mariadb-docker.sh`).

---

## GAPs de segurança da auditoria ARQ-00 (SEC-14 a SEC-25)

> Origem comum: 🟣 SecBrain, auditoria ARQ-00, Lote 21 (2026-10-13). Relatório completo em `docs/arq-00-invariantes-seguranca.md` (seção "GAPs"; itens 1 a 11). O item 12 (teste de matriz de rotas) não virou card: é feito pelo 🔴 TestBrain dentro do ARQ-00.
>
> **Todos os cards abaixo estão fora do escopo do Lote 21: não corrigir dentro dos cards ARQ. Executar após o ARQ-07 ou em lote próprio, por decisão do 🤍 MegaBrain/usuário.** Durante a refatoração, a regra é não piorar nenhum destes pontos.
>
> Caminhos: `API` = `backend/rotaperfumes-api`, `SH` = `backend/shared`. As linhas citadas são do código antes da refatoração DDD; depois do ARQ-07 o dev deve localizar o ponto equivalente na nova estrutura.
>
> **2026-10-31 (fechamento do Lote 28):** SEC-19 e SEC-23 foram **concluídos no Lote 28** (cards em `feito.md`). Todos os GAPs deste bloco estão resolvidos (Lotes 24, 27 e 28).
>
> **2026-10-31 (abertura do Lote 28):** SEC-19 e SEC-23 foram para o **Lote 28** (cards em `fazendo.md`). Com isso, nenhum card deste bloco fica pendente aqui: SEC-14 a SEC-17 e SEC-24/SEC-25 foram concluídos no Lote 27, SEC-18 e SEC-20 a SEC-22 no Lote 24.
>
> **2026-10-31 (fechamento do Lote 27):** SEC-14, SEC-15, SEC-16, SEC-17, SEC-24 e SEC-25 foram **concluídos no Lote 27** (cards em `feito.md`).
>
> **2026-10-31:** SEC-14, SEC-15, SEC-16, SEC-17, SEC-24 e SEC-25 foram para o **Lote 27** (cards em `fazendo.md`). Aqui ficam SEC-19 e SEC-23 (backend + frontend + config), para o lote seguinte. SEC-18 e SEC-20 a SEC-22 já foram concluídos no Lote 24.
>
> **Onde fica hoje (conferido no ARQ-08, 2026-10-16; mapa completo em `docs/arquitetura-ddd.md` §10):**
> - SEC-14: **concluído no Lote 27 (card em `feito.md`)**; `http.Server` em `API/internal/bootstrap/server.go`; decode de JSON em `API/internal/httpx/body.go` (`DecodificarJSON`); itens do pedido em `SH/vendas/pedido.go` (`NovoPedido`).
> - SEC-15: **concluído no Lote 27 (card em `feito.md`)**; `maskDSN` continua em `SH/db/db.go`.
> - SEC-16: **concluído no Lote 27 (card em `feito.md`)**; `SH/kernel/pagina.go` (`Pagina`, `Offset()`).
> - SEC-17: **concluído no Lote 27 (card em `feito.md`)**; `API/internal/httpx/cors.go`.
> - SEC-18: **concluído no Lote 24 (card em `feito.md`)**; `SH/identidade/identidadecrypto/jwt.go` e `SH/config/config.go`.
> - SEC-19: **concluído no Lote 28 (card em `feito.md`)**; `SH/adapters/turnstile/turnstile.go`.
> - SEC-20: **concluído no Lote 24 (card em `feito.md`)**; `API/internal/httpx/clientip.go` (`IPDoCliente`).
> - SEC-21: **concluído no Lote 24 (card em `feito.md`)**; `API/internal/httpx/cookies.go` (`DefinirCookiesSessao`, `LimparCookiesSessao`).
> - SEC-22: **concluído no Lote 24 (card em `feito.md`)**; `SH/identidade/identidadeapp/login.go` (`Login`) e `sessao.go` (refresh).
> - SEC-23: **concluído no Lote 28 (card em `feito.md`)**; `deve_trocar_senha` em `SH/identidade/usuario.go` e `SH/identidade/identidademysql/usuarios.go`; autenticação por request em `API/internal/httpx/auth.go`.
> - SEC-24: **concluído no Lote 27 (card em `feito.md`)**; enums de ordenação em `SH/catalogo/ordenacao.go`, `SH/comercial/ordenacao.go`, `SH/vendas/filtros.go`, `SH/identidade/usuario.go` e `SH/identidade/historico.go`, traduzidos por `switch` nos adapters `*mysql`; testes em `SH/tests/repositories/sort_test.go`.
> - SEC-25: **concluído no Lote 27 (card em `feito.md`)**; buscas `LIKE ?` em `SH/catalogo/catalogomysql/produtos.go`, `SH/comercial/comercialmysql/{clientes,oportunidades,visitas}.go` e `SH/vendas/vendasmysql/pedidos.go`.

---

## TST-04: itens do roteiro do Lote 12 que testam o próprio `make` (1.1, 1.2 e 1.7) — prioridade MÉDIA

**Status:** pendente, **bloqueado** (depende do usuário); avançou parcialmente no Lote 20 (2026-10-11). **Reconfirmado em 2026-11-02:** o `.env` local aponta para o banco de **produção** (`192.168.168.106:3306`), e os agentes não rodam `make db-*` nele. Continua bloqueado até o usuário rodar os itens (ou apontar o `.env` para um banco de dev/descartável).
**Camada:** Testes
**Responsável:** 🔴 TestBrain com o usuário
**Origem:** 🔴 TestBrain, roteiro manual do Lote 12 (2026-10-07).

**Já feito (Lote 20, 2026-10-11):** execução equivalente sem `make` no banco descartável `rotaperfumes_l12` (criado e depois apagado; o banco `rotaperfumes` ficou intacto, com o mesmo MD5 de schema e `estoque` com 8401 linhas). Aprovados: 3.1, 3.2, 3.3, 3.5, 3.6, 3.7, 4.1, 4.2 e 4.3. O 3.4 não se aplica (só com `db-rebuild`). Resultado em `docs/roteiro-teste-manual-lote12.md`, seção "Execução complementar do TST-04".

**Falta:** 1.1, 1.2 e 1.7 (SEC-11: `db-check-env` e precedência de variáveis). Eles testam o próprio `make`, então não há execução equivalente: continuam bloqueados até o usuário rodá-los com o `make` (ou liberar a permissão de `make db-*` para o 🔴 TestBrain).

**Regras:**
- usar **sempre** `DB_NAME=rotaperfumes_l12`, conferindo antes com `make -n` que o banco alvo é o descartável;
- **nunca** rodar no banco `rotaperfumes`;
- apagar o `rotaperfumes_l12` ao final.

Registrar o resultado na seção "Execução" do roteiro.

---

## DEP-03: vulnerabilidades altas de dev restantes (`braces` via `eslint-config-next`) — prioridade BAIXA

**Status:** pendente (acompanhar o upstream). **2026-11-02:** a conferência do upstream entra no **Lote 31** (🟢 FrontBrain); se não houver correção publicada, o card continua aqui como acompanhamento, com a data da conferência.
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain
**Origem:** residual do DEP-01 (Lote 20, 2026-10-11; ver `feito.md`).

**Descrição:** após a migração para Tailwind 4, o `npm audit` completo caiu de 9 para 5 vulnerabilidades (5 altas); `npm audit --omit=dev` = 0. As 5 vêm da cadeia `braces` → `micromatch` → `fast-glob@3.3.1` → `@next/eslint-plugin-next` → `eslint-config-next@16.4.0`, que só é usada pela ferramenta de lint (dev). O `braces` não tem versão corrigida, e o `npm audit fix --force` rebaixaria o `eslint-config-next` para 14.2.35 (rejeitado).

**Ação esperada:** acompanhar o upstream (`eslint-config-next`/`@next/eslint-plugin-next`, `fast-glob`, `micromatch`, `braces`). Quando houver correção, atualizar ou usar `overrides` de `fast-glob`/`micromatch` no `package.json` e validar com `tsc`, `eslint`, vitest, `next build` e `npm audit`. Não usar `npm audit fix --force`.

---

> **2026-11-02:** CHORE-04 e CHORE-05 foram para o **Lote 31** (`fazendo.md`).
>
> **2026-10-29:** CHORE-03 (id de path → 400) e CHORE-06 (duplicidade → 409) receberam a aprovação do usuário para a mudança de contrato HTTP e foram para o **Lote 25** (cards em `fazendo.md`). Aqui ficam CHORE-04 e CHORE-05.
