# A Fazer

> Atualizado em 2026-11-04 (decisões do usuário após o Lote 34, repassadas pelo 🤍 MegaBrain; 🔵 SubBrain). Saíram desta lista para `feito.md` (bloco "Decisões do usuário de 2026-11-04 (após o Lote 34)"): **OPS-02** (SMTP de produção configurado pelo usuário, por ora via Gmail; o passo 10 do DEPLOY-08 fica com o pré-requisito atendido) e **CHORE-18** (hostname `ivo-Inspiron-15-3530` confirmado; a guarda já o cobre em minúsculas, sem alteração; a conferência do SAN virou item opcional do DEPLOY-08). O **DEPLOY-08** ganhou a nota "Andamento 2026-11-04" (passo 11 rodado no servidor por engano; na máquina do admin, `db-check-env` ok, mas a `192.168.168.106:3306` não responde e o fingerprint da `deploy/ca-producao.pem` difere do `deploy/db-ca.fingerprint`). O **TST-17** ganhou nota (Docker na máquina do admin sem acesso ao socket; falta `usermod -aG docker`; sem `shellcheck`). O **SEC-54** deixou de citar o CHORE-18 como pendente. O **DOC-09** continua aqui: fatos conferidos, mas o `CLAUDE.md` não foi alterado pelo agente (autorização repassada por agente não vale; o usuário aplica direto). **Seguem pendentes do usuário:** **DEPLOY-08** (ALTA; bloqueia o primeiro deploy da `main`), **DOC-09** e **TST-17**. O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-04 (fechamento do Lote 34, 🔵 SubBrain). O **Lote 34** (SEC-53, SEC-46, SEC-50, SEC-49, CHORE-15, CHORE-17 e BUG-26) foi concluído e está em `feito.md` (**não commitado**, pendente do usuário). Entraram 7 cards no bloco "Achados do Lote 34", logo abaixo: **BUG-27** (MÉDIA: `db-reset`/`db-rebuild` com `db-down` e `db-seed` como pré-requisitos irmãos) e **SEC-54**, **TST-19**, **SEC-55**, **TST-20**, **TST-21** e **TST-22** (BAIXA). O **DOC-09** ganhou o texto do SEC-46, do SEC-53 e do SEC-50 (continua aguardando a autorização do usuário). **Seguem pendentes do usuário:** **DEPLOY-08** (ALTA; bloqueia o primeiro deploy da `main`), **OPS-02** (SMTP de produção), **CHORE-18** (hostname do servidor), **DOC-09** (alteração do `CLAUDE.md`) e **TST-17** (precisa de uma máquina com Docker). O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-04 (abertura do Lote 34, 🔵 SubBrain). O usuário pediu "continue com as tarefas a fazer e as fazendo". O 🤍 MegaBrain abriu o **Lote 34 de 2026-11-04** e 7 cards saíram desta lista para `fazendo.md`: **SEC-50**, **SEC-53**, **CHORE-17** e **BUG-26** (bloco "Achados do Lote 33 (DEPLOY-07)") e **SEC-46**, **SEC-49** e **CHORE-15** (bloco "Achados dos Lotes 30 e 31"). Passo atual do lote: "🟣 SecBrain especificando → 🟡 BackBrain → 🔴 TestBrain → 🔵 SubBrain fechando". **Continuam aqui, pendentes do usuário:** **DEPLOY-08** (ALTA; bloqueia o primeiro deploy da `main`), **OPS-02** (SMTP de produção), **CHORE-18** (hostname do servidor), **DOC-09** (alteração do `CLAUDE.md`, exige autorização) e **TST-17** (precisa de uma máquina com Docker). O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-04 (🔴 TestBrain): o **TST-18** (estágio "Testes Frontend" no Jenkinsfile, pedido do usuário) foi criado e concluído no mesmo dia, direto em `feito.md`; o backlog abaixo não mudou.
>
> Antes, ainda em 2026-11-04 (fechamento do Lote 33, 🔵 SubBrain). O **DEPLOY-07** foi concluído no código e nas docs e está em `feito.md` (não commitado). O **DEPLOY-05** saiu desta lista (substituído pelo DEPLOY-07; registro em `feito.md`). Entraram 11 cards novos no bloco "Achados do Lote 33 (DEPLOY-07)", logo abaixo: **DEPLOY-08** (ALTA, pendente do usuário: executar a §11 de `docs/deploy-servidor.md` no servidor; **bloqueia o primeiro deploy da `main`**), **SEC-50**, **SEC-53**, **TST-17**, **OPS-02** e **CHORE-18** (MÉDIA) e **SEC-51**, **SEC-52** (risco aceito), **CHORE-17**, **BUG-26** e **DOC-09** (BAIXA). O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-04 (abertura do Lote 33, 🔵 SubBrain). O primeiro build da `main` falhou no estágio "CA do banco" (`bind source path does not exist: /etc/mysql/ssl/ca.pem`): o `deploy/mariadb-docker.sh` ainda não rodou no servidor. O usuário informou que **não existe MySQL no ambiente**; o **DEPLOY-05** foi **substituído pelo DEPLOY-07** (instalação limpa do MariaDB 11.4 de produção, CA nova de produção, CA de dev preservada, seed em produção), card novo que foi direto para o **Lote 33 de 2026-11-04** (`fazendo.md`). O DEPLOY-05 fica abaixo só como histórico, com a nota de substituição. O restante do backlog não mudou.
>
> Antes, em 2026-11-02 (fechamento dos Lotes 30 e 31, 🔵 SubBrain). Os Lotes 30 e 31 foram fechados (`feito.md`). Voltaram para esta lista:
> - **DEPLOY-05 (ALTA, pendente do usuário):** a revisão do `deploy/mariadb-docker.sh` foi concluída no Lote 30; falta a **execução no servidor** (passo a passo em `docs/deploy-servidor.md` §11). **Bloqueia o primeiro deploy da `main`:** sem a rede `rotaperfumes-db` e o container `rotaperfumes-mariadb`, o compose falha e o pipeline para.
> - **ARQ-09 (Lote 32):** **adiado** pelo 🤍 MegaBrain por recomendação do 🟣 SecBrain (não executar antes do primeiro teste em produção).
>
> Entraram 12 cards novos no bloco "Achados dos Lotes 30 e 31": SEC-46, BACKUP-01 (MÉDIA) e SEC-47, CHORE-14, SEC-48, DB-07, SEC-49, CHORE-15, BUG-25, FE-18, TST-16, CHORE-16 (BAIXA). O **TST-14** foi cancelado (`feito.md`). Continuam aqui o **TST-04** (bloqueado) e o **DEP-03** (acompanhamento; agora 16 moderadas do `minimatch` via `eslint`, só dev, 0 altas). **Em seguida:** merge de todas as branches na `main` pelo 🤍 MegaBrain; antes do primeiro deploy, o usuário executa o DEPLOY-05.
>
> Antes, ainda em 2026-11-02 (abertura dos Lotes 30, 31 e 32, 🔵 SubBrain). O usuário pediu para executar **todas** as pendências, aprovou antecipadamente as decisões de produto/contrato pendentes nos cards (inclusive os textos do **CHORE-07** e os nomes de cookies do **SEC-33**) e pediu, ao final, o merge de todas as branches na `main` e o commit da `main`, para testar o site em produção. Saíram desta lista para `fazendo.md`:
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

# Achados do Lote 34, 2026-11-04

> Pendências do Lote 34 (`feito.md`, "Lote 34 de 2026-11-04"), levantadas pelo 🟣 SecBrain e pelo 🔴 TestBrain. Nenhuma bloqueou o fechamento do lote. Caminhos: `SH` = `backend/shared`.

## BUG-27: `db-reset`/`db-rebuild` rodam o `db-seed` mesmo com o DROP recusado (`make -k`) e têm corrida com `-j` — prioridade MÉDIA

**Status:** pendente.
**Camadas:** Config (Makefile)
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🔴 TestBrain no Lote 34 (2026-11-04). O problema já existia antes do lote.

**Descrição:** `db-reset` e `db-rebuild` têm `db-down` e `db-seed` como **pré-requisitos irmãos**. Com `make -k`, o `db-seed` roda (`CREATE DATABASE` + seed) mesmo quando o `DROP` foi recusado pelo `db-guard-drop` (SEC-46); confirmado num tempdir. Com `-j`, os dois podem rodar em paralelo (corrida entre o DROP e o seed).

**Sugestão:** receitas sequenciais (`$(MAKE) db-down && $(MAKE) db-seed`, repassando `CONFIRMAR`/`ENV_ARQUIVO`) ou `.NOTPARALLEL` nesses alvos; garantir que a recusa de qualquer guarda pare o alvo inteiro mesmo com `-k`.

**Critério de aceite:** teste com `make -k db-reset` e `make -k db-rebuild` (DROP recusado → nenhum `CREATE DATABASE`/seed) e com `-j` (ordem garantida); os testes do SEC-46 continuam verdes.

---

## SEC-54: guarda de produção é blocklist; aliases do servidor passam — prioridade BAIXA

**Status:** pendente.
**Camadas:** Config (Makefile), Backend (testes)
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain no Lote 34 (2026-11-04).

**Descrição:** a detecção de produção (`PRODUCAO_DETECTA` no Makefile e `SH/testkit/integracao` no Go) compara o `DB_HOST` com uma lista fixa. Aliases do mesmo servidor passam nos dois: `ivo-inspiron-15-3530.local`, `::ffff:192.168.168.106`, outro IP da máquina.

**A avaliar:** allowlist para a integração (loopback e CasaOS `192.168.168.106:3307`) ou resolver o DNS antes de comparar (com cuidado para não depender de rede nos testes). Manter a paridade Makefile × Go.

**Relacionado:** CHORE-18 (hostname real do servidor na guarda): **concluído em 2026-11-04**, sem alteração (o hostname `ivo-Inspiron-15-3530` confirmado pelo usuário já está coberto, em minúsculas, no `PRODUCAO_HOSTS_LAN` e no `SH/testkit/integracao`; ver `feito.md`). Continuam valendo os aliases acima (`.local`, IPv4 mapeado em IPv6, outros IPs).

---

## TST-19: catraca para testes que abrem conexão com a config do ambiente sem o helper do SEC-50 — prioridade BAIXA

**Status:** pendente.
**Camadas:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** 🔴 TestBrain no Lote 34 (2026-11-04).

**Descrição:** a catraca de `SH/tests/config/sec50_integracao_guarda_test.go` só detecta leitura direta de `INTEGRATION` (`Getenv("INTEGRATION")`). Um `_test.go` novo que faça `config.Load()` + `db.Open` sem passar por `integracao.Config`/`integracao.Abrir` não é pego.

**Critério de aceite:** teste (AST ou varredura) que falha se algum `_test.go` abrir conexão com a config do ambiente sem chamar `integracao.Config`/`Abrir`; lista de exceções explícita, se houver.

---

## SEC-55: `go build` de `build-shared`/`build-api` recebe as `DB_*` no modo integração — prioridade BAIXA

**Status:** pendente.
**Camadas:** Config (Makefile)
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain no Lote 34 (2026-11-04).

**Descrição:** com `INTEGRATION=1` (caminho do `make test-integration`), os `go build` de `build-shared` e `build-api` também recebem as `DB_*`. Não há conexão ao banco no build (impacto mínimo), mas o segredo chega a um processo que não precisa dele.

**Ação esperada:** um `SEM_SEGREDOS` estrito para os alvos de build (sem a exceção do modo integração).

**Critério de aceite:** teste do Makefile (no padrão do `sec53_makefile_segredos_test.go`) mostrando que o `go build` nunca recebe `DB_*`/`MYSQL_PWD`, mesmo com `INTEGRATION=1`.

---

## TST-20: cobrir o ramo de erro de `integracao.Abrir` — prioridade BAIXA

**Status:** pendente.
**Camadas:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** 🔴 TestBrain no Lote 34 (2026-11-04).

**Descrição:** `SH/testkit/integracao` está com 72,7% de cobertura no `Abrir`: o ramo de erro (falha ao abrir/pingar o banco → `t.Fatal`) não é exercitado.

**Critério de aceite:** teste do ramo de erro (ex.: `testing.TB` falso e DSN inválido/porta fechada no loopback, sem tocar em banco real); cobertura do pacote sem queda.

---

## TST-21: teste com terminal real do `seed-producao.sh` (confirmação e pepper) — prioridade BAIXA

**Status:** pendente.
**Camadas:** Testes, Deploy
**Responsável:** 🔴 TestBrain
**Origem:** 🔴 TestBrain no Lote 34 (BUG-26, 2026-11-04).

**Descrição:** o BUG-26 cobriu só o caminho **sem** terminal (`bug26_seed_sem_terminal_linux_test.go`). O caminho com terminal real (digitar a confirmação do `--forcar` e o pepper sem eco) não tem teste automatizado.

**Sugestão:** pty (ex.: `script -qec`, ou um pty em Go) com `docker` falso, no padrão dos testes de ensaio do DEPLOY-07; pular quando o `script`/pty não estiver disponível.

---

## TST-22: rodar os testes de integração com o helper do SEC-50 contra um MariaDB descartável — prioridade BAIXA

**Status:** pendente (execução; exige um banco descartável: CasaOS `192.168.168.106:3307` ou loopback).
**Camadas:** Testes
**Responsável:** 🔴 TestBrain (com o arquivo de ambiente fornecido pelo usuário)
**Origem:** 🔴 TestBrain no Lote 34 (2026-11-04).

**Descrição:** no Lote 34, os 22 pontos de integração migrados para `SH/testkit/integracao` foram verificados só com `INTEGRATION` desligado (42 SKIP) e pela recusa da produção. O **caminho de sucesso** com o helper novo não rodou contra um banco real.

**A fazer:** `make test-integration ENV_ARQUIVO=integracao.env` com um `integracao.env` apontando para o CasaOS :3307 ou para o loopback (nunca o `.env` local, que aponta para a produção).

**Critério de aceite:** testes de integração verdes contra o banco descartável; nenhuma conexão à produção.

**Relacionado:** TST-17 (testes reais com Docker).

---

# Achados do Lote 33 (DEPLOY-07), 2026-11-04

> Pendências do DEPLOY-07 (`feito.md`, "Lote 33 de 2026-11-04"). O DEPLOY-08 vem primeiro: sem ele não há produção. A OPS-02 e a CHORE-18 foram concluídas em 2026-11-04 (`feito.md`, "Decisões do usuário de 2026-11-04 (após o Lote 34)").

## DEPLOY-08: executar a §11 de `docs/deploy-servidor.md` no servidor (instalação inicial da produção) — prioridade ALTA (pendente do usuário)

**Status:** pendente **do usuário**; **bloqueia o primeiro deploy da `main`**. Código e docs prontos no DEPLOY-07 (Lote 33, `feito.md`). Substitui a parte de execução do antigo DEPLOY-05.
**Camadas:** Deploy, Database
**Responsável:** usuário (servidor `192.168.168.106`, Jenkins e máquina do admin) → 🔴 TestBrain (conferência dos resultados) → 🔵 SubBrain (fechamento)
**Origem:** fechamento do DEPLOY-07 (2026-11-04).

**Por que bloqueia:** a `main` espera a rede Docker `rotaperfumes-db`, o container `rotaperfumes-mariadb`, a CA de produção em `deploy/db-ca.fingerprint` (hoje só o cabeçalho: o estágio "CA do banco" falha com `deploy/db-ca.fingerprint sem fingerprint: ...`), o banco semeado e as credenciais do Jenkins.

**Passo a passo:** `docs/deploy-servidor.md` §11 ("Instalação inicial da produção (DEPLOY-07)"), 12 passos:
1. checkout com o DEPLOY-07 em `/root/rotaperfumes`;
2. `sudo bash deploy/mariadb-docker.sh --nova-ca --dev-ip <IP do admin>` e anotar o `CA_FP`;
3. chave da CA para mídia offline e `shred`;
4. `CA_FP` como única linha útil de `deploy/db-ca.fingerprint`, `make test`, commit (sem push);
5. `PASSWORD_PEPPER` novo, Secret file `rotaperfumes-api-env` e credencial `rotaperfumes-db-admin`;
6. `sudo bash deploy/seed-producao.sh --pepper-file <arq>` (senha do admin em `/root/rotaperfumes-mariadb/admin-app.senha`);
7. conferências SQL (13 tabelas, admin ativo com `deve_trocar_senha = 1` e Argon2id, 42 vendedores, 0 não-admin ativos);
8. checklist pré-push (`IMPORTAR_DUMP` desmarcado);
9. push na `main`;
10. 1º login do admin com troca obrigatória e ativação dos vendedores (pré-requisito **OPS-02 atendido** em 2026-11-04: SMTP de produção configurado pelo usuário, por ora via Gmail);
11. `.env.producao` + `deploy/ca-producao.pem` na máquina do admin;
12. `TestTST15_MariadbDocker_Real` numa máquina com Docker (ver **TST-17**).

**Item opcional (vindo do CHORE-18):** conferir o SAN do certificado no servidor: `openssl x509 -in /etc/mysql/ssl/server-cert.pem -noout -ext subjectAltName` (deve conter `ivo-Inspiron-15-3530` e `192.168.168.106`).

**Andamento 2026-11-04:**
- O usuário rodou os comandos do passo 11 **no servidor, dentro de `deploy/`**: por isso `make: Sem regra para processar o alvo 'db-check-env'`, e `make 192.168.168.106 ...` porque o `<alvo>` do exemplo foi trocado pelo IP. O passo 11 é para a **máquina do admin** (`192.168.168.105`), **na raiz do repositório**.
- Conferido pelo 🤍 MegaBrain na máquina do admin: `make db-check-env ENV_ARQUIVO=.env.producao` → ok. **Porém:**
  - (a) a porta `192.168.168.106:3306` **não responde** desta máquina (timeout de TCP; a 3307 do CasaOS responde);
  - (b) o fingerprint de `deploy/ca-producao.pem` local (`f8dbaf59…aaf70`) **difere** da linha útil de `deploy/db-ca.fingerprint` (`f31ee820…a04c2`).
- **Próximo passo do usuário:** no servidor, conferir `docker port rotaperfumes-mariadb`, o firewall (`ufw status`/`iptables`) e se `192.168.168.105` está no `--dev-ip`; conferir o fingerprint de `/etc/mysql/ssl/ca.pem`; depois recopiar a CA (passo 11.1) e repetir o passo 11.3 na máquina do admin.

**Critério de aceite:** container `rotaperfumes-mariadb` 11.4 com a CA de produção; `deploy/db-ca.fingerprint` commitado só com o fingerprint dela; estágios "CA do banco", "Deploy" e "Smoke test" verdes; log da API com `tls=ligado (custom)` e `min=TLS1.3`; admin logado e com a senha trocada; `/root/rotaperfumes-ca/ca-key.pem`, `/root/pepper-producao.txt` e `/root/rotaperfumes-mariadb/admin-app.senha` apagados do servidor.

---

> **2026-11-04 (Lote 34):** SEC-50, SEC-53, CHORE-17 e BUG-26 foram para o **Lote 34** e estão concluídos (`feito.md`).

---

## TST-17: testes reais com Docker do `seed-producao.sh` e do `mariadb-docker.sh` (`TST15_DOCKER=1`) e `shellcheck` nos scripts — prioridade MÉDIA

**Status:** pendente (exige uma máquina com Docker que **não** seja o servidor; esta máquina não tem acesso ao socket do Docker).
**Camadas:** Testes, Deploy
**Responsável:** 🔴 TestBrain (usuário roda onde houver Docker)
**Origem:** DEPLOY-07 (Lote 33, 2026-11-04); continua a pendência do TST-15.

**Descrição:**
- O `deploy/seed-producao.sh` só tem testes estáticos e de ensaio (`deploy07_seed_producao_test.go`, `deploy07_seed_producao_ensaio_test.go`). Falta um teste real, no padrão do `TestTST15_MariadbDocker_Real`: container descartável criado pelo `mariadb-docker.sh`, seed, conferências (13 tabelas, admin `1 1 1`, 0 não-admin ativos), recusa da 2ª execução e `--forcar`.
- O `TestTST15_MariadbDocker_Real` (`TST15_DOCKER=1`) ainda não rodou (`docs/deploy-servidor.md` §11, passo 12).
- O `shellcheck` não está instalado no ambiente: incluir no `make setup`/`scripts/versoes.conf` (ou documentar) e rodar em `deploy/*.sh` e `scripts/*.sh`.

**Relacionado:** TST-16 (isolamento do teste real do `mariadb-docker.sh`).

**Nota 2026-11-04:** a máquina do admin (`192.168.168.105`, `residentivo-MS-7E28`) tem Docker, mas o usuário `residentivo` não está no grupo `docker` (`permission denied` no socket) e a sessão do agente não tem sudo. **Aguardando o usuário** rodar `sudo usermod -aG docker residentivo` e reabrir a sessão. O `shellcheck` também não está instalado nessa máquina.

---

> **2026-11-04 (decisões do usuário após o Lote 34):** **OPS-02** (SMTP de produção configurado, por ora via Gmail) e **CHORE-18** (hostname `ivo-Inspiron-15-3530` confirmado, sem alteração de código) foram concluídos e estão em `feito.md`.

---

## SEC-51: conta somente-leitura em produção para a máquina do admin — prioridade BAIXA

**Status:** pendente.
**Camadas:** Database, Deploy
**Responsável:** 🟣 SecBrain → 🌸 DataBrain/🟡 BackBrain (`mariadb-docker.sh`) → 🔴 TestBrain
**Origem:** DEPLOY-07 (Lote 33, 2026-11-04).

**Descrição:** com `--dev-ip`, a conta `casaos` da máquina do admin tem `ALL ON rotaperfumes.*` na produção. Para consultas do dia a dia, uma conta só com `SELECT` reduziria o estrago de um engano (a guarda do Makefile não cobre clientes externos, como o `mariadb` manual ou uma IDE). Avaliar uma conta `*_ro` por `--dev-ip`, com senha própria, e deixar o `ALL` só para migrações.

---

## SEC-52: build do `resetpassword` no `seed-producao.sh` sem `--cap-drop ALL` — prioridade BAIXA (risco aceito)

**Status:** **risco aceito** no DEPLOY-07 (2026-11-04); card para reavaliar.
**Camadas:** Deploy
**Responsável:** 🟣 SecBrain → 🟡 BackBrain
**Origem:** DEPLOY-07 (Lote 33, 2026-11-04).

**Descrição:** a etapa 5 do `deploy/seed-producao.sh` compila o `resetpassword` num container `GO_IMAGE` com `--security-opt no-new-privileges:true`, mas sem `--cap-drop ALL`/`--read-only` e com a rede padrão (baixa os módulos do proxy do Go). O container não tem rede do banco nem segredos, e o binário roda depois com `--cap-drop ALL` e `--read-only`. Avaliar endurecer (ex.: `--cap-drop ALL`, cache de módulos em `tmpfs`/volume, ou módulos pré-baixados) sem quebrar o build.

---

> **DOC-09** concluído em 2026-11-04 (autorização direta do usuário; ver `feito.md`, "Decisões do usuário de 2026-11-04 (após o Lote 34)").

---

## ARQ-09: valores monetários em centavos (`int64`) — prioridade BAIXA/MÉDIA (ADIADO)

**Status:** **adiado** (2026-11-02) pelo 🤍 MegaBrain, por recomendação do 🟣 SecBrain: refatoração ampla de dinheiro; **não executar antes do primeiro teste em produção** (validação dos Lotes 30/31). Voltou de `fazendo.md` (Lote 32, aberto e não executado). Antes: pendente (sai do Lote 21; executar após o ARQ-08, em lote próprio).
**Camadas:** Database, Backend (Frontend se o formato do JSON mudar)
**Responsável:** 🌸 DataBrain → 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** decisão P3 do 🤍 MegaBrain no ARQ-00 (2026-10-13).

**Descrição:** no ARQ-05, o value object `Dinheiro` permanece `float64` encapsulado. Este card migra o cálculo interno para centavos (`int64`).

**Invariantes do 🟣 SecBrain (2026-11-02), para quando o card for retomado:**
- **contrato JSON inalterado:** valores em **reais decimais** (sem passar a centavos no JSON);
- **schema inalterado:** colunas `DECIMAL(15,2)`, **sem migração**;
- entrada decodificada com **`json.Number`** (sem passar por `float64`);
- arredondamento **half away from zero**, explícito e testado;
- **overflow verificado** nas somas e multiplicações em `int64` (substitui a nota N-9 sobre `+Inf` no `kernel.Dinheiro.Somar`).

**Risco:** arredondamento em `valor_total` (itens, descontos e pagamentos); é preciso um teste de regressão comparando os totais antigos com os novos.

**Critério de aceite:** cálculos monetários sem `float64`; totais existentes reproduzidos sem diferença; contrato JSON e schema inalterados; testes verdes sem perda de cobertura.

---

## Achados dos Lotes 30 e 31

> Origem comum: especificação e revisão do 🟣 SecBrain, entregas dos devs e verificação do 🔴 TestBrain nos Lotes 30 e 31 (2026-11-02; ver `feito.md`). Nenhum deles bloqueou o fechamento dos lotes. Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário. Caminhos: `SH` = `backend/shared`.

---

> **2026-11-04 (Lote 34):** SEC-46, SEC-49 e CHORE-15 foram para o **Lote 34** e estão concluídos (`feito.md`).

---

## BACKUP-01: backup diário do MariaDB de produção — prioridade MÉDIA

**Status:** pendente.
**Camadas:** Database, Deploy
**Responsável:** 🌸 DataBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, Lote 30 (2026-11-02).

**Descrição:** hoje não existe backup do banco de produção. Um `--recriar` do `mariadb-docker.sh`, uma troca de digest (SEC-31) ou um erro da conta `casaos` (que tem escrita a partir da LAN) pode apagar os dados sem volta.

**Ação esperada:** rotina diária de dump (arquivo com permissão `600`, retenção configurável), documentada em `docs/deploy-servidor.md`; exigir/registrar um backup antes de qualquer `--recriar` ou upgrade de digest; procedimento de restauração testado.

**Critério de aceite:** backup diário gerado com `600` e retenção; restauração testada num banco descartável; docs atualizadas.

---

## SEC-47: fixar por digest as imagens base dos `Dockerfile` — prioridade BAIXA

**Status:** pendente.
**Camadas:** Infra (build)
**Responsável:** 🟡 BackBrain / 🟢 FrontBrain (🟣 SecBrain revisa)
**Origem:** SEC-31 (Lote 30, 2026-11-02).

**Descrição:** o SEC-31 fixou por digest as imagens do `Jenkinsfile` e do `mariadb-docker.sh`, mas `backend/Dockerfile`, `frontend/Dockerfile` e `deploy/Dockerfile.caddy` ainda usam tags mutáveis.

**Ação esperada:** fixar as bases por `tag@sha256:` e incluí-las no `make deps-audit-imagens`.

**Critério de aceite:** todas as bases com digest; `deps-audit-imagens` cobrindo os três arquivos; build verde.

---

## CHORE-14: remover a limpeza dos cookies sem prefixo (transição do SEC-33) — prioridade BAIXA

**Status:** pendente; executar **7 dias ou mais depois do deploy do SEC-33** em produção.
**Camadas:** Backend, Testes
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** SEC-33 (Lote 31, 2026-11-02).

**Descrição:** no modo seguro, o logout e a transição ainda limpam os cookies antigos (`access_token`/`refresh_token`, sem `__Host-`). Depois que os cookies antigos expirarem, esse código fica sem uso.

**Ação esperada:** remover a limpeza dos nomes antigos no modo seguro (o dev continua com os nomes sem prefixo); ajustar os testes.

**Critério de aceite:** modo seguro só emite/limpa `__Host-…`; testes verdes.

---

## SEC-48: reprocessar os `.webp` de produto já gravados — prioridade BAIXA

**Status:** pendente.
**Camadas:** Backend (shared/tools), Testes
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** SEC-34 e CHORE-09 (Lote 31, 2026-11-02).

**Descrição:** o SEC-34 converte o WebP novo (PNG/JPEG), mas os `.webp` gravados antes continuam com EXIF e eventuais bytes anexados.

**Ação esperada:** opção no comando `backend/shared/cmd/limpezaimagens` (com `-dry-run`) para reprocessar os `.webp` existentes pelo normalizador e atualizar `produtos.imagem` na mesma operação, sem deixar órfão nem referência quebrada.

**Critério de aceite:** testes com diretório temporário (convertido, referência atualizada, `-dry-run` sem efeito, falha sem perda do original); docs atualizadas.

---

## DB-07: ligar `ONLY_FULL_GROUP_BY` no `SQL_MODE` do `mariadb-docker.sh` — prioridade BAIXA

**Status:** pendente; depende de um MariaDB descartável para a integração.
**Camadas:** Database, Testes
**Responsável:** 🌸 DataBrain → 🔴 TestBrain
**Origem:** DB-04 + DB-06 (Lote 30, 2026-11-02): o modo ficou desligado.

**Descrição:** o `SQL_MODE` do `deploy/mariadb-docker.sh` ficou sem `ONLY_FULL_GROUP_BY`; um `GROUP BY` incompleto devolve valores arbitrários em vez de erro. O `sqlSerieVendas` já foi ajustado (`GROUP BY` da data).

**Ação esperada:** rodar a suíte de integração (`INTEGRATION=1`) num MariaDB descartável com o modo ligado; corrigir as consultas que falharem; incluir o modo no `SQL_MODE` e documentar.

**Critério de aceite:** integração verde com o modo ligado; `mariadb-docker.sh` e `docs/deploy-servidor.md` atualizados.

---

## BUG-25: usuário apagado no meio da troca da própria senha responde 500 — prioridade BAIXA (decisão)

**Status:** pendente; **decisão** do 🟣 SecBrain/🤍 MegaBrain.
**Camada:** Backend (shared)
**Responsável:** 🟣 SecBrain (decisão) → 🟡 BackBrain → 🔴 TestBrain
**Origem:** BUG-23/BUG-24 (Lote 31, 2026-11-02).

**Descrição:** se o usuário for apagado durante a troca da própria senha, a resposta é 500; no reset admin o mesmo caso é 404.

**Ação esperada:** decidir se a troca da própria senha passa a 404 (ou 401 de sessão inválida, como no refresh do BUG-23); aplicar e testar.

**Critério de aceite:** decisão registrada; teste do caso; contrato documentado.

---

## FE-18: bloqueio de 1500 ms nos botões de ativo de clientes e usuários — prioridade BAIXA

**Status:** pendente.
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain → 🔴 TestBrain
**Origem:** BUG-22 (Lote 31, 2026-11-02), complemento.

**Descrição:** o BUG-22 deixou o toggle atômico no backend, mas os botões de ativo das telas de clientes e usuários não têm o bloqueio de 1500 ms que o BUG-17 colocou em produtos.

**Ação esperada:** aplicar o mesmo bloqueio nas duas telas.

**Critério de aceite:** teste de frontend (clique duplo dentro de 1500 ms gera uma só chamada); `tsc` e `eslint` limpos.

---

## TST-16: isolamento do teste real do `mariadb-docker.sh` e concorrência real do BUG-22 — prioridade BAIXA

**Status:** pendente; depende de Docker/banco descartável.
**Camadas:** Deploy (script), Testes
**Responsável:** 🌸 DataBrain → 🔴 TestBrain
**Origem:** TST-15 (Lote 30) e BUG-22 (Lote 31), 2026-11-02.

**Descrição:** o `TestTST15_MariadbDocker_Real` usa os diretórios padrão de segredos e da CA do script, então não fica isolado de uma instalação real na mesma máquina. O BUG-22 não teve teste de concorrência contra banco real.

**Ação esperada:** opções `--segredos-dir` e `--ca-dir` no `deploy/mariadb-docker.sh`, usadas pelo teste real com diretórios temporários; teste de integração com toggles concorrentes (clientes e usuários) num banco descartável.

**Critério de aceite:** teste real sem tocar em `/root`; teste de concorrência verde; docs do script atualizadas.

---

## CHORE-16: renomear `seedusers.MySQLArgs`/`RunMySQL` e avaliar os pacotes `*mysql` — prioridade BAIXA

**Status:** pendente (cosmético).
**Camada:** Backend
**Responsável:** 🟡 BackBrain
**Origem:** CHORE-13/DOC-08 (Lote 30, 2026-11-02).

**Descrição:** com somente MariaDB, `seedusers.MySQLArgs`/`RunMySQL` e os pacotes de adapter `*mysql` (`catalogomysql`, `vendasmysql` etc.) mantêm o nome antigo.

**Ação esperada:** renomear as funções do `seedusers`; avaliar (custo × benefício) renomear os pacotes, sem mudança de comportamento.

**Critério de aceite:** build e testes verdes; decisão sobre os pacotes registrada.

---

## Achados do Lote 29 (sem cards pendentes)

> Origem comum: revisão do 🟣 SecBrain e entregas do 🟡 BackBrain e do 🔴 TestBrain no Lote 29 (SEC-29, TST-09, TST-08 e SETUP-01, 2026-11-01; ver `feito.md`). Nenhum deles bloqueou o fechamento desses cards. Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário. Caminhos: `SH` = `backend/shared`.
>
> **2026-11-02 (fechamento do Lote 30):** o **TST-14** foi **cancelado** (os drop-ins do `setup-servidor.sh` deixaram de existir; coberto pelo `TestTST15_MariadbDocker_Real`; card em `feito.md`). Este bloco fica sem cards pendentes.
>
> **2026-11-02 (abertura do Lote 30):** SEC-39, SEC-40, SEC-41, SEC-42, TST-12, DB-06 e DEPLOY-04 foram para o **Lote 30** (`fazendo.md`). DOC-07 (substituído pelo DOC-08) e DEPLOY-03 (sem MySQL) foram **cancelados** (`feito.md`). Aqui fica só o TST-14, bloqueado.
>
> **2026-11-01 (decisões do usuário):** TST-13 cancelado e SEC-43 fechado como "não corrigir" (ambos em `feito.md`). DOC-07 entrou aqui (divergência MariaDB 11.4 × MySQL 8.0 em produção). SEC-39 a SEC-42 e TST-12 devem considerar a produção MySQL 8.0 e o SEC-44 (concluído, em `feito.md`).
>
> **2026-11-01 (fechamento do Lote 29):** entraram DB-06, DEPLOY-03, DEPLOY-04 e TST-14 (achados do SEC-44 e do DEPLOY-02), logo após o DOC-07.

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

**Status:** pendente (acompanhar o upstream). **2026-11-02 (fechamento do Lote 31):** upstream conferido pelo 🟢 FrontBrain; as altas do `braces` saíram (**0 altas**), restam **16 moderadas** do `minimatch` via `eslint` (só dev). O card fica como acompanhamento. Antes: a conferência do upstream entrou no Lote 31.
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain
**Origem:** residual do DEP-01 (Lote 20, 2026-10-11; ver `feito.md`).

**Descrição:** após a migração para Tailwind 4, o `npm audit` completo caiu de 9 para 5 vulnerabilidades (5 altas); `npm audit --omit=dev` = 0. As 5 vêm da cadeia `braces` → `micromatch` → `fast-glob@3.3.1` → `@next/eslint-plugin-next` → `eslint-config-next@16.4.0`, que só é usada pela ferramenta de lint (dev). O `braces` não tem versão corrigida, e o `npm audit fix --force` rebaixaria o `eslint-config-next` para 14.2.35 (rejeitado).

**Ação esperada:** acompanhar o upstream (`eslint-config-next`/`@next/eslint-plugin-next`, `fast-glob`, `micromatch`, `braces`). Quando houver correção, atualizar ou usar `overrides` de `fast-glob`/`micromatch` no `package.json` e validar com `tsc`, `eslint`, vitest, `next build` e `npm audit`. Não usar `npm audit fix --force`.

---

> **2026-11-02:** CHORE-04 e CHORE-05 foram para o **Lote 31** (`fazendo.md`).
>
> **2026-10-29:** CHORE-03 (id de path → 400) e CHORE-06 (duplicidade → 409) receberam a aprovação do usuário para a mudança de contrato HTTP e foram para o **Lote 25** (cards em `fazendo.md`). Aqui ficam CHORE-04 e CHORE-05.
