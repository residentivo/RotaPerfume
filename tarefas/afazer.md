# A Fazer

> Atualizado em 2026-10-31 (fechamento do Lote 27, 🔵 SubBrain). O **Lote 27** (SEC-14, SEC-15, SEC-16, SEC-17, SEC-24, SEC-25, BUG-14, BUG-15 e SEC-26) foi concluído e está em `feito.md` (não commitado, pendente do usuário). Dele saíram cinco cards, no bloco "Achados do Lote 27", logo após o SEC-34: **SEC-35** (`ExcluirPagamento`/`CriarPagamento` checam fora de lock, MÉDIA), **SEC-36** (origens de dev sempre na allowlist de CORS, BAIXA), **BUG-24** (corrida entre troca e reset de senha, BAIXA), **CHORE-10** (`ErrImagemMuitoGrande` com `KindMuitoGrande` e `maxLength={100}` nas buscas do frontend, BAIXA) e **TST-11** (teste instável do IMG-01, BAIXA). SEC-19 e SEC-23 continuam no bloco "GAPs de segurança da auditoria ARQ-00".
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
> - **P1, logs (altera a regra LOG-02):** `vlog` em 4 níveis: rota automática no router, caso de uso via `defer`, pontos de decisão e 1 linha por query (~600 chamadas no lugar de ~3.726). Mesmo formato de log; TST-05 deve continuar verde. Nesses cards, o critério "logs `vlog` preservados" significa formato e TST-05 preservados e cobertura pelos 4 níveis, não a manutenção de cada chamada atual. Regra no código: comentário de `apis/shared/vlog/vlog.go`; documentação: `docs/logs-verbose.md` (atualizada no ARQ-08).
> - **X1, contrato HTTP:** a única exceção ao "contrato HTTP inalterado" é a do ARQ-01 (métodos das 13 rotas, `/api/senha-historico/2/extra` e, pelo parse estrito de `{user_id...}` (R-f), `/api/senha-historico/0` e `/-1` passando a 400).
>
> Histórico anterior (2026-10-11): o 🔴 TestBrain executou `docs/roteiro-teste-manual-lote12.md` (24 aprovados, 1 falhou, 13 não verificáveis de 38; ver a seção "Execução (2026-10-07, 🔴 TestBrain)" no fim do roteiro). Daí saíram os cards BUG-13, LOG-01, DOC-05 e TST-04. BUG-13, LOG-01 e DOC-05 foram concluídos no Lote 16 (2026-10-07; ver `feito.md`). O LOG-02 (Lote 17) gerou o TST-05, e o DEP-00 (Lote 19) gerou o DEP-01 e o DEP-02. No Lote 20 (2026-10-11; ver `feito.md`) foram concluídos DEP-01, DEP-02 e TST-05; o TST-04 avançou parcialmente (só faltam 1.1, 1.2 e 1.7, que dependem do usuário rodar o `make`) e do DEP-01 saiu o card DEP-03 (BAIXA). Em 2026-10-16 o DEPLOY-01 (Lote 13) foi concluído e o MAIL-01 foi cancelado (sem servidor de e-mail; a API usa o envio Noop). Os dois estão em `feito.md`.

---

## ARQ-09: valores monetários em centavos (`int64`) — prioridade BAIXA/MÉDIA

**Status:** pendente (sai do Lote 21; executar após o ARQ-08, em lote próprio)
**Camadas:** Database, Backend (Frontend se o formato do JSON mudar)
**Responsável:** 🌸 DataBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** decisão P3 do 🤍 MegaBrain no ARQ-00 (2026-10-13).

**Descrição:** no ARQ-05, o value object `Dinheiro` permanece `float64` encapsulado. Este card migra o cálculo interno para centavos (`int64`) e avalia o schema (colunas DECIMAL) e o contrato JSON.

**Risco:** arredondamento em `valor_total` (soma de itens, descontos e pagamentos). Os valores calculados podem divergir em centavos dos já gravados; é preciso um teste de regressão comparando os totais antigos com os novos e uma regra de arredondamento explícita.

**Nota N-9 (🟣 SecBrain, ARQ-01, 2026-10-13):** `kernel.Dinheiro.Somar` pode resultar em `+Inf` em overflow do `float64`. Revalidar neste card (com `int64`, checar overflow na soma).

**Critério de aceite:** cálculos monetários sem `float64`; totais existentes reproduzidos sem diferença; contrato JSON inalterado (ou mudança aprovada pelo usuário); testes verdes sem perda de cobertura.

---

## Achados do ARQ-05 (TST-06; BUG-16, SEC-27 e BUG-17 foram para o Lote 25; SEC-26 foi para o Lote 27)

> Origem comum: revisão do ARQ-05 (Vendas) pelo 🟣 SecBrain e pelo 🔴 TestBrain, Lote 21 (2026-10-15). **Fora do escopo do Lote 21:** não corrigir dentro dos cards ARQ. Executar após o ARQ-07 ou em lote próprio, por decisão do 🤍 MegaBrain/usuário. Durante a refatoração, não piorar estes pontos. Caminhos: `SH` = `apis/shared`, `API` = `apis/rotaperfumes-api`.
>
> **2026-10-29:** BUG-16, SEC-27 e BUG-17 receberam a decisão de produto do usuário e foram para o **Lote 25** (cards em `fazendo.md`, na fila até o fechamento do Lote 24).
>
> **2026-10-31:** o SEC-26 foi para o **Lote 27** e foi concluído nele (card em `feito.md`). Aqui fica só o TST-06.

---

## TST-06: asserções de tipo sem checagem nos testes de handler — prioridade BAIXA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** 🔴 TestBrain, revisão do ARQ-05 (Lote 21, 2026-10-15).

**Descrição:** os testes de handler usam asserções de tipo sem checagem, como `body["data"].(map[string]any)`. Se a resposta vier em outro formato, o teste dá panic, aborta o pacote inteiro e esconde as falhas dos testes seguintes.

**Ação esperada:** trocar por `require.IsType` ou pela forma `v, ok := …` com `require.True(t, ok)`, em todas as suítes de handler. Sem enfraquecer nenhuma asserção.

**Critério de aceite:** nenhuma asserção de tipo sem checagem nos testes de handler (conferir por busca); mesma quantidade de testes, todos verdes.

---

## BUG-18: Analytics, IN sem limite nas metas e detecção de "tabela não existe" por texto amplo — prioridade BAIXA

**Status:** pendente (fora do escopo do Lote 21)
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

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Testes
**Responsável:** 🔴 TestBrain (🟡 BackBrain se algum adapter precisar de ajuste)
**Origem:** sugestão do 🔴 TestBrain, Lote 21 ARQ-07 (2026-10-16).

**Descrição:** `apis/rotaperfumes-api/tests/arquitetura/arq07_sql_concat_test.go` detecta concatenação com `+` em textos SQL passados a `QueryContext`/`QueryRowContext`/`ExecContext`, mas hoje varre só o `analyticsmysql`.

**Ação esperada:** estender a varredura aos demais adapters MySQL (`catalogomysql`, `comercialmysql`, `vendasmysql`, identidade etc.), com allowlist dos helpers que recebem SQL pronto do chamador: `apis/shared/db/linhas.go`, `apis/shared/db/consulta.go`, `paginar` em `comercialmysql/linhas.go` e em `vendasmysql/linhas.go`, e `vinculo` em `comercialmysql/carteiras.go`. Manter um controle negativo (caso que o teste precisa reprovar).

**Critério de aceite:** todos os adapters MySQL cobertos; allowlist explícita e mínima; controle negativo reprovando; testes verdes.

---

## Achados do DASH-01 (DOC-06, FE-15; BUG-19 foi para o Lote 25)

> Origem comum: revisão do DASH-01 (Funil de vendas no Dashboard) pelo 🟣 SecBrain e pelo 🔴 TestBrain, Lote 22 (2026-10-29). Nenhum deles bloqueou o fechamento do DASH-01 (em `feito.md`). Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário.
>
> **2026-10-29:** o BUG-19 recebeu a decisão do usuário (corrigir o arredondamento) e foi para o **Lote 25** (card em `fazendo.md`). Aqui ficam DOC-06 e FE-15.

---

## DOC-06: comentário "escopo por carteira" das rotas de Oportunidades em `rotas.go` — prioridade BAIXA

**Status:** pendente
**Camada:** Backend (só comentário)
**Responsável:** 🟡 BackBrain
**Origem:** 🟣 SecBrain, revisão do DASH-01 (Lote 22, 2026-10-29).

**Descrição:** em `apis/rotaperfumes-api/internal/bootstrap/rotas.go:130`, o comentário do bloco de Oportunidades diz "acesso comum, escopo por carteira", mas o código filtra por vendedor (`oportunidades.vendedor_id`), não pela carteira ativa do cliente. O DASH-01 usou o mesmo filtro por vendedor. O comentário engana quem for revisar o escopo.

**Ação esperada:** corrigir o comentário para "escopo por vendedor (`oportunidades.vendedor_id`)". Aproveitar e conferir se os comentários equivalentes das linhas 116 (Pedidos) e 137 (Visitas) descrevem o filtro real; corrigir só o texto, sem mudar código.

**Critério de aceite:** comentários de `rotas.go` batem com o filtro aplicado em cada contexto; build e testes verdes (nenhuma mudança de comportamento).

---

## FE-15: `FunilVendas`, aviso de desligado sem `forcarZeros` e seleção da janela após erro — prioridade BAIXA

**Status:** pendente
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain → 🔴 TestBrain
**Origem:** 🔴 TestBrain, testes do DASH-01 (Lote 22, 2026-10-29).

**Descrição:** dois pontos em `frontend/src/components/dashboard/FunilVendas.tsx`:
1. **Aviso de desligado (linha ~252):** o aviso próprio do componente ("Vendedor desligado. Nao ha oportunidades no funil...") também dispara pela sessão (`useVendedorDesligado`) quando o componente é usado **sem** `forcarZeros`. No Dashboard isso não acontece, porque a página passa `forcarZeros` nesses casos e mostra o aviso dela. Mas, se o componente for reutilizado em outra tela, o aviso pode aparecer duplicado ou fora de contexto.
2. **Erro na troca de janela:** depois de um erro ao trocar a janela, o botão novo fica marcado (`aria-pressed`), mas as barras continuam com os dados da janela anterior, sem indicar isso.

**Ação esperada:** (1) definir a regra do aviso quando o componente não recebe `forcarZeros` (ex.: só pelo `vendedor_desligado` do payload, ou um prop explícito); (2) depois do erro, voltar a seleção para a janela dos dados exibidos, ou indicar de qual janela são os dados. Testes para os dois casos.

**Critério de aceite:** comportamento definido e testado nos dois pontos; nenhuma regressão no Dashboard (testes DASH-01 e FE-02 verdes); `tsc` e `eslint` limpos.

---

## Achados do DB-03 (SEC-29, SEC-30, SEC-31, DB-04, DEPLOY-02)

> Origem comum: avaliação de riscos do 🟣 SecBrain no DB-03 (troca de MySQL para MariaDB 11.4 LTS, Lote 23, 2026-10-29). Nenhum deles bloqueia o fechamento do DB-03. Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário.

---

## SEC-29: TLS na conexão Go ↔ MariaDB — prioridade MÉDIA

**Status:** pendente
**Camadas:** Backend (config/DSN), Database (servidor), Testes
**Responsável:** 🌸 DataBrain (certificados no servidor) → 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, DB-03 (Lote 23, 2026-10-29).

**Descrição:** a API conecta no MariaDB do host (`host.docker.internal:3306`) sem TLS: credenciais e dados trafegam em claro pela bridge docker. O driver `go-sql-driver/mysql` suporta TLS com CA própria via `mysql.RegisterTLSConfig` e `tls=custom` no DSN.

**Ação esperada:** habilitar TLS no servidor MariaDB (`ssl_cert`/`ssl_key`/`ssl_ca`; opcionalmente `require_secure_transport=ON` ou `REQUIRE SSL` nos usuários `rotaperfumes_app`/`rotaperfumes_admin`); na API, nova variável (ex.: `DB_TLS_CA`) lida em `apis/shared/config/config.go`, registrar a config com `mysql.RegisterTLSConfig("custom", ...)` e acrescentar `tls=custom` ao DSN quando definida; o import do `Jenkinsfile` (`mariadb --ssl`) também. Ajustar os testes que comparam o DSN literal (ex.: `apis/shared/tests/config` e `apis/shared/tests/db/mask_dsn_test.go`) e documentar em `docs/deploy-servidor.md` e `deploy/api.env.example`.

**Critério de aceite:** com a variável definida, a conexão usa TLS e falha se o certificado não bater com a CA; sem ela, comportamento atual (dev local); testes de DSN ajustados e verdes; `maskDSN` sem vazar nada novo; docs atualizadas.

---

## SEC-30: autenticação ed25519 nas contas do banco — prioridade BAIXA

**Status:** pendente
**Camadas:** Database (deploy), Testes
**Responsável:** 🌸 DataBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, DB-03 (Lote 23, 2026-10-29).

**Descrição:** `deploy/mariadb-setup.sql` e `deploy/setup-servidor.sh` criam `rotaperfumes_app` e `rotaperfumes_admin` com o plugin padrão (`mysql_native_password`, hash SHA1). O MariaDB oferece o plugin `ed25519`, mais forte.

**Ação esperada:** avaliar `INSTALL SONAME 'auth_ed25519'` e `IDENTIFIED VIA ed25519 USING PASSWORD('...')` nas duas contas; confirmar que o `go-sql-driver/mysql` (versão do `go.mod`) e o cliente `mariadb:11.4` do Jenkins autenticam com ed25519; manter a idempotência do setup (usuário já existente) e documentar a migração das contas atuais.

**Critério de aceite:** contas da API e do admin com ed25519 no servidor; API e estágio "Importar dump" funcionando; procedimento documentado em `docs/deploy-servidor.md`.

---

## SEC-31: fixar a imagem `mariadb:11.4` do Jenkins por digest — prioridade BAIXA

**Status:** pendente
**Camadas:** Infra (CI/CD)
**Responsável:** 🌸 DataBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, DB-03 (Lote 23, 2026-10-29).

**Descrição:** o `Jenkinsfile` usa `DB_CLIENT_IMAGE = 'mariadb:11.4'` no estágio "Importar dump". A tag é mutável: uma imagem trocada no registry rodaria com a senha do `rotaperfumes_admin` no ambiente.

**Ação esperada:** fixar a imagem por digest (`mariadb:11.4@sha256:...`), registrar no comentário do `Jenkinsfile` como atualizar o digest e incluir essa atualização na rotina do `make deps-audit` (mensal e antes de deploy). Avaliar fazer o mesmo com `golang:1.26-alpine`.

**Critério de aceite:** imagem referenciada por digest; procedimento de atualização documentado; pipeline verde.

---

## DB-04: avaliar `ONLY_FULL_GROUP_BY` no `sql_mode` — prioridade BAIXA

**Status:** pendente
**Camadas:** Database, Backend (SQL dos adapters), Testes
**Responsável:** 🌸 DataBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, DB-03 (Lote 23, 2026-10-29).

**Descrição:** o drop-in do `deploy/setup-servidor.sh` grava `sql_mode = STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION`, sem `ONLY_FULL_GROUP_BY` (que o MySQL 8 tinha por padrão). Sem ele, um `GROUP BY` incompleto devolve valores arbitrários em vez de erro.

**Ação esperada:** revisar as consultas com `GROUP BY` (principalmente `analyticsmysql` e as métricas de clientes), rodar a suíte de integração com `ONLY_FULL_GROUP_BY` ligado e, se tudo passar, incluí-lo no drop-in e na documentação. Se alguma consulta falhar, corrigi-la antes.

**Critério de aceite:** decisão registrada (incluir ou não, com motivo); se incluído, consultas compatíveis, testes de integração verdes e `docs/deploy-servidor.md` atualizado.

---

## DEPLOY-02: aplicar `local_infile`/`sql_mode`/collation também em servidores já abertos — prioridade MÉDIA

**Status:** pendente
**Camadas:** Infra (deploy)
**Responsável:** 🌸 DataBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, DB-03 (Lote 23, 2026-10-29).

**Descrição:** em `deploy/setup-servidor.sh`, o drop-in `zz-rotaperfumes.cnf` (`gravar_dropin_bind`) só é gravado quando o banco escuta em loopback. Num servidor cujo `bind-address` já está aberto, `local_infile = 0`, o `sql_mode` estrito e `utf8mb4_unicode_ci` nunca são aplicados, e o servidor pode seguir com `local_infile` ligado e a collation padrão do MariaDB 11.4 (`utf8mb4_uca1400_ai_ci`).

**Ação esperada:** separar o drop-in de hardening (local_infile, sql_mode, charset/collation) do ajuste de `bind-address`: gravar o de hardening sempre (idempotente, reiniciando o serviço só se o conteúdo mudar) e o de bind só quando necessário. Conferir os valores efetivos com `SHOW GLOBAL VARIABLES` ao final e avisar se divergirem.

**Critério de aceite:** em servidor com bind já aberto, as três configurações ficam aplicadas após o script; reexecução sem restart desnecessário; `docs/deploy-servidor.md` (etapa 3 da seção 0) atualizado.

---

## Achados da especificação do Lote 25 (UX-02, CHORE-07, BUG-22)

> **2026-10-30:** o Lote 25 foi concluído (`feito.md`); estes três cards continuam pendentes (UX-02 com avanço parcial).
>
> Origem comum: 🟣 SecBrain, especificação do Lote 25 (2026-10-29). Por decisão do usuário (adendo do 🤍 MegaBrain, item 5), R4 e R6 ficam **só no backlog** (não implementar no Lote 25). Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário. Caminhos: `SH` = `apis/shared`.

---

## UX-02: orientar o ajuste manual de estoque depois do snapshot do faturamento (409 → usar o PUT) — prioridade BAIXA

**Status:** pendente
**Camadas:** Frontend (Docs/Postman)
**Responsável:** 🟢 FrontBrain → 🔴 TestBrain → 🔵 SubBrain
**Origem:** 🟣 SecBrain, especificação do Lote 25 (risco R4, 2026-10-29).

**Descrição:** o upsert do faturamento cria o snapshot de estoque do dia para o SKU. Depois disso, um ajuste manual (POST de estoque) para o mesmo SKU e dia bate no índice `uk_estoque_data_sku`. Com o CHORE-06 (Lote 25), essa resposta passa de 500 para 409 "registro de estoque já cadastrado para este sku nesta data", mas o usuário não entende por que o registro "já existe" se ele não o criou.

**Ação esperada:** em `frontend/src/app/admin/estoque/page.tsx` (modal de ajuste), no 409 orientar a editar o registro existente (PUT), por exemplo com a mensagem "já existe registro deste SKU nesta data (pode ter sido criado pelo faturamento); edite o registro existente", e, se possível, um atalho para abrir o registro. Documentar o comportamento no `postman/README.md` e no manual da tela de estoque.

**Critério de aceite:** o 409 do POST de estoque mostra a orientação e não a mensagem genérica; teste de frontend do caso; docs atualizadas. Depende do CHORE-06 concluído.

**Andamento (2026-10-30, Lote 25):** o CHORE-06 foi concluído, e a tela de estoque já acrescenta "Edite o registro existente." à mensagem do 409; o `postman/README.md` documenta o caso (inclusive o registro criado pelo faturamento). Falta: o atalho para abrir o registro existente, o texto mencionando o faturamento na tela e o manual da tela de estoque.

---

## CHORE-07: mensagens "apenas o status" / "apenas ter o status alterado" imprecisas depois do BUG-16 — prioridade BAIXA

**Status:** pendente (mudança de texto = mudança de contrato; exige aprovação do usuário)
**Camadas:** Backend, Frontend (mapeamento de mensagens), Testes, Docs/Postman
**Responsável:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → 🔵 SubBrain
**Origem:** 🟣 SecBrain, especificação do Lote 25 (risco R6, 2026-10-29).

**Descrição:** as mensagens existentes em `SH/vendas/erros.go:38` ("…apenas o status") e `erros.go:42` ("…apenas ter o status alterado") dizem que um pedido faturado pode ter o status alterado. Depois do BUG-16 (Faturado só vai para Entregue; Entregue é terminal), isso fica enganoso. O BUG-16 **não** muda esses textos, porque mudar a mensagem é mudança de contrato HTTP não aprovada.

**Ação esperada:** propor ao usuário os textos novos, alinhados à regra do BUG-16; com a aprovação, trocar os textos, atualizar o `friendlyPedidoError` do frontend, os testes que conferem a mensagem exata, o Postman (exemplo "…apenas ter o status alterado") e o `postman/README.md`.

**Critério de aceite:** textos aprovados pelo usuário; mensagens coerentes com a regra de status; testes, frontend, Postman e docs atualizados.

---

## BUG-22: toggle de ativo de clientes e usuários lê e inverte (mesmo padrão do BUG-17) — prioridade BAIXA

**Status:** pendente
**Camadas:** Backend, Frontend, Testes
**Responsável:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, especificação do Lote 25 (BUG-17, 2026-10-29).

**Descrição:** o toggle de ativo de clientes (contexto Comercial) e de usuários (contexto Identidade) usa o mesmo padrão do produto antes do BUG-17: lê o registro, inverte o valor em memória e grava. Dois toggles concorrentes leem o mesmo valor e se anulam (lost update). O BUG-17 corrigiu só o produto.

**Ação esperada:** aplicar o mesmo padrão do BUG-17: `UPDATE … SET ativo = NOT ativo WHERE id = ?` atômico quando não houver valor explícito no body, `DefinirAtivo` quando houver, e releitura para a resposta; bloqueio de 1500 ms no botão das telas, se aplicável. Atenção às regras próprias desses toggles (escopo/IDOR de clientes do SEC-01; revogação de sessão do usuário inativado do SEC-06/SEC-08; BUG-05) para não perder nenhuma delas. O toggle de usuário está no contexto Identidade, mexido no Lote 24: executar depois do merge dele.

**Critério de aceite:** toggles concorrentes não se anulam (teste de integração); regras de escopo e de revogação de sessão preservadas; contrato HTTP inalterado; testes verdes.

---

## Achados do Lote 24 (SEC-32, SEC-33, BUG-23, TST-08)

> Origem comum: achados registrados durante o Lote 24 (SEC-18, SEC-20, SEC-21 e SEC-22, 2026-10-29), fora do escopo dele. Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário, depois do merge do Lote 24. Caminhos: `SH` = `apis/shared`.

---

## SEC-32: hash vazio ou malformado no banco permite enumerar contas pelo tempo de resposta — prioridade BAIXA

**Status:** pendente
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** Lote 24 (2026-10-29), achado do SEC-22.

**Descrição:** em `SH/identidade/identidadecrypto/hasher.go:89-96`, quando o hash gravado no banco está vazio ou malformado (formato não reconhecido), a verificação retorna sem rodar o Argon2. A resposta continua 401 "credenciais inválidas", mas sai mais rápido, o que permite enumerar essas contas pelo tempo de resposta (quebra parcial da anti-enumeração do SEC-22).

**Ação esperada:** quando o hash não for reconhecido, rodar o hash descartável (mesmo custo do caminho normal) antes de responder; log de decisão sem o hash nem o e-mail.

**Critério de aceite:** com hash vazio ou malformado, o hash descartável roda e a resposta é idêntica (mensagem e status) à de senha errada; teste que confirma a execução do hash nesse caminho; testes do SEC-22 verdes.

---

## SEC-33: prefixo `__Host-` nos nomes dos cookies de sessão — prioridade BAIXA

**Status:** pendente (muda os nomes dos cookies; avaliar impacto com o usuário)
**Camadas:** Backend, Frontend (se ler os nomes), Testes, Docs/Postman
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain → 🔵 SubBrain
**Origem:** Lote 24 (2026-10-29), achado do SEC-21.

**Descrição:** os cookies de sessão (access e refresh) não usam o prefixo `__Host-`. O prefixo faz o navegador exigir `Secure`, `Path=/` e ausência de `Domain`, o que impede que um subdomínio ou uma conexão sem HTTPS plante ou sobrescreva o cookie (cookie tossing).

**Ação esperada:** avaliar a troca dos nomes para `__Host-…`. Impactos: muda os nomes (todas as sessões ativas caem no deploy; conferir o frontend, o Caddy, o Postman e os testes que citam os nomes) e exige `Secure` **sempre**, inclusive em dev (o `COOKIE_SECURE=false` do SEC-21 deixaria de funcionar com o prefixo; definir como fica o ambiente local). Conferir o `Path` do cookie de refresh (o `__Host-` exige `Path=/`).

**Critério de aceite:** decisão registrada; se aprovada, cookies com `__Host-`, `Secure`, `Path=/` e sem `Domain`; logout limpa os cookies novos (e os antigos, na transição); testes, Postman e docs atualizados.

---

## BUG-23: refresh de usuário apagado responde 500 — prioridade BAIXA

**Status:** pendente
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 24 (2026-10-29), achado do SEC-22.

**Descrição:** em `SH/identidade/identidadeapp/sessao.go:37-41`, se o usuário dono do refresh token não existir mais (apagado do banco), a releitura do usuário falha e a resposta é 500, em vez da resposta genérica de sessão inválida usada para inativo (SEC-22).

**Ação esperada:** tratar "usuário não encontrado" no refresh como sessão inválida: 401 genérico, revogar o refresh token e limpar os cookies, no mesmo padrão do inativo; log de decisão só com o id.

**Critério de aceite:** refresh de usuário apagado → 401 genérico (sem 500); token revogado; teste do caso; testes do SEC-22 verdes.

---

## TST-08: `make test` falha com `go: no such tool "covdata"` — prioridade MÉDIA

**Status:** pendente
**Camadas:** Ambiente (Makefile/toolchain), Testes
**Responsável:** 🔴 TestBrain (🟡 BackBrain, se o ajuste for no `Makefile`)
**Origem:** Lote 24 (2026-10-29), execução dos testes.

**Descrição:** `make test` falha com `go: no such tool "covdata"`. O toolchain `go1.26.0` é baixado automaticamente (diretiva `toolchain`/`GOTOOLCHAIN`) e não traz a ferramenta `covdata`, que o `go test` usa quando os alvos `test-shared`/`test-api` passam `-coverpkg`. Os testes em si passam com `go test ./...`. Sem o alvo funcionando, a medição de cobertura pelo `make` fica quebrada.

**Ação esperada:** identificar a causa (toolchain baixado automaticamente versus Go instalado localmente) e corrigir de forma reprodutível: alinhar a versão do Go instalada com a do `go.mod`, fixar `GOTOOLCHAIN` nos alvos ou ajustar o uso de `-coverpkg`, sem perder a medição de cobertura. Documentar o pré-requisito no `CLAUDE.md`/`Makefile` (`make help`).

**Critério de aceite:** `make test` roda de ponta a ponta com cobertura em ambiente limpo; causa e pré-requisito documentados.

---

## Achados do Lote 25 (TST-09, TST-10, CHORE-08, FE-16)

> Origem comum: fechamento do Lote 25 (BUG-16, SEC-27, BUG-17, BUG-19, CHORE-03, CHORE-06, BUG-20 e BUG-21, 2026-10-30; ver `feito.md`). Nenhum deles bloqueou o fechamento. Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário. Caminhos: `SH` = `apis/shared`, `API` = `apis/rotaperfumes-api`.

---

## TST-09: rodar os 3 testes de integração do Lote 25 e conferir o CHECK `chk_pagamentos_taxa_pct` — prioridade MÉDIA

**Status:** pendente (bloqueado: **depende do usuário** corrigir a credencial do banco)
**Camadas:** Testes, Database
**Responsável:** usuário (credencial) → 🔴 TestBrain (testes) e 🌸 DataBrain (conferência do CHECK)
**Origem:** Lote 25 (2026-10-30).

**Descrição:** o banco do `.env` (MariaDB 11.8.8) passou a recusar o login (`Access denied for user 'casaos'`) durante o Lote 25. Por isso: (1) os 3 testes de integração do lote não rodaram: o de dois toggles concorrentes do BUG-17 e os de upsert real do BUG-21 (saldo 10, baixa 6 → saldo 4, ruptura 0; saldo 5, baixa 6 → saldo −1, ruptura 1), em `SH/tests/catalogo/bug17_bug21_integration_test.go`; (2) a conferência final do CHECK `chk_pagamentos_taxa_pct`, que o 🌸 DataBrain aplicou com `make db-fix-taxa-pct-check` em 2026-10-29 (tabela com 0 linhas), não foi feita.

**Ação esperada:** depois de o usuário corrigir a credencial no `.env` (ou no servidor), rodar os testes com `INTEGRATION=1`, as variáveis `DB_*` no ambiente e `-p 1` (ver `docs/arquitetura-ddd.md` §10.9); conferir no `information_schema.CHECK_CONSTRAINTS` que o CHECK existe com `taxa_pct >= 0 and taxa_pct <= 30`, e que um INSERT com `taxa_pct = 30.01` falha com o erro 4025. Se algum teste falhar, abrir card de bug para o dev responsável.

**Critério de aceite:** os 3 testes de integração rodam (sem `t.Skip`) e passam; CHECK conferido no banco; resultado registrado no card do Lote 25 em `feito.md`.

---

## TST-10: `make test-frontend` e `make frontend-deps` não funcionam no disco exFAT com o Node 18 do sistema — prioridade MÉDIA

**Status:** pendente
**Camadas:** Ambiente (Makefile/toolchain), Frontend, Testes
**Responsável:** 🔴 TestBrain (🟢 FrontBrain, se o ajuste for no `package.json`/`Makefile`)
**Origem:** Lote 25 (2026-10-30), execução dos testes de frontend.

**Descrição:** com o repositório no disco exFAT (sem symlinks), o `npm ci` não cria `node_modules/.bin`, então os alvos `make frontend-deps` e `make test-frontend` não encontram os binários. Além disso, o Node do sistema é o 18, e o vitest 4 exige Node >= 20. No Lote 25, os 1630 testes, o `tsc` e o `eslint` só rodaram num espelho com Node 22. Card separado do TST-08 (que é do toolchain Go e do `covdata`), porque a causa e a correção são outras.

**Ação esperada:** documentar o pré-requisito de Node >= 20 (por exemplo, `.nvmrc` ou `engines` no `package.json`, e o `CLAUDE.md`/`make help`); fazer os alvos funcionarem sem `node_modules/.bin` (ex.: chamar `node node_modules/vitest/vitest.mjs` ou `npx --no-install`, ou usar `npm ci --no-bin-links`, citado em `postman/README.md`, "Subindo o ambiente") ou documentar o espelho em disco com symlinks como caminho oficial.

**Critério de aceite:** `make frontend-deps` e `make test-frontend` rodam de ponta a ponta no ambiente do usuário (ou falham com mensagem clara de pré-requisito); pré-requisitos documentados.

---

## CHORE-08: observações do 🔴 TestBrain no Lote 25 (log, vlog, NaN e 1062 de carteiras) — prioridade BAIXA

**Status:** pendente
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

**Status:** pendente
**Camadas:** Frontend
**Responsável:** 🟢 FrontBrain (ou 🔴 TestBrain)
**Origem:** Lote 25 (2026-10-30).

**Descrição:** no Lote 25 o frontend passou em testes, `tsc` e `eslint` (espelho com Node 22), mas o `next build` não foi rodado. Mudaram `PedidoModal.tsx`, `PagamentoModal.tsx`, `ui/Select.tsx` (prop `helperText`), `app/admin/pedidos/page.tsx`, `app/pagamentos/page.tsx`, `app/admin/produtos/page.tsx` e `app/admin/estoque/page.tsx`.

**Ação esperada:** rodar `next build` (no ambiente com Node >= 20; ver TST-10) e corrigir o que aparecer; registrar o resultado.

**Critério de aceite:** `next build` sem erros; resultado registrado no card do Lote 25 em `feito.md`.

---

## Achados do IMG-01 (CHORE-09, SEC-34)

> Origem comum: fechamento do IMG-01 (imagem do produto, Lote 26, 2026-10-31; ver `feito.md`). Nenhum deles bloqueou o fechamento. Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário. Caminhos: `SH` = `apis/shared`, `API` = `apis/rotaperfumes-api`.

---

## CHORE-09: job de limpeza de imagens órfãs de produto — prioridade BAIXA

**Status:** pendente
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

**Status:** pendente
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

## Achados do Lote 27 (SEC-35, SEC-36, BUG-24, CHORE-10, TST-11)

> Origem comum: especificação do 🟣 SecBrain e entregas do 🟡 BackBrain e do 🔴 TestBrain no Lote 27 (SEC-14, SEC-15, SEC-16, SEC-17, SEC-24, SEC-25, BUG-14, BUG-15 e SEC-26, 2026-10-31; ver `feito.md`). Nenhum deles bloqueou o fechamento. Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário. Caminhos: `SH` = `apis/shared`, `API` = `apis/rotaperfumes-api`.

---

## SEC-35: `ExcluirPagamento` e `CriarPagamento` checam fora de lock (TOCTOU) — prioridade MÉDIA

**Status:** pendente
**Camada:** Backend (shared)
**Responsável:** 🟣 SecBrain (escolha da correção) → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, especificação do Lote 27 (2026-10-31), achado fora do escopo nº 1 (mesmo padrão do SEC-26).

**Descrição:**
1. `ExcluirPagamento` (`SH/vendas/vendasapp/pagamentos.go:121-145`) checa "nunca quitado" e a posse fora de lock e depois executa um DELETE sem condição. Se o pagamento for quitado entre a checagem e o DELETE, um pagamento quitado é apagado.
2. `CriarPagamento` checa a posse do pedido fora de lock. Se o pedido for apagado no meio, o INSERT cai no erro de FK 1452 e a API responde 500.

**Ação esperada:**
- Exclusão: DELETE condicional `... AND status_pagamento NOT IN ('Pago','Pago com atraso')` (como o UPDATE do SEC-27, usando `vendas.StatusPago`/`StatusPagoComAtraso`), com releitura para decidir 404/409 quando o DELETE afetar 0 linhas; ou tx com `SELECT ... FOR UPDATE` e as checagens de posse e status dentro dela.
- Criação: mapear o erro 1452 de `fk_pagamentos_pedido` para `ErrPedidoNaoEncontrado` (404), sem o valor no log; ou checar a posse dentro de uma tx com lock do pedido.

**Critério de aceite:** pagamento quitado no meio da exclusão não é apagado (409); pedido apagado no meio da criação → 404, não 500; testes sqlmock da ordem do SQL e do caminho de corrida; mensagens e status atuais preservados no caminho normal.

---

## SEC-36: origens de dev (`localhost:3000`) sempre na allowlist de CORS, inclusive em produção — prioridade BAIXA

**Status:** pendente
**Camada:** Backend (config/middleware) + Deploy
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, especificação do Lote 27 (2026-10-31), achado fora do escopo nº 2 (levantado durante o SEC-17).

**Descrição:** `defaultDevOrigins` (`http://localhost:3000` e `http://127.0.0.1:3000`, `SH/config/config.go:496`) entra sempre na allowlist de CORS com `Access-Control-Allow-Credentials: true`, também em produção, e vale para o `uploadPermitido` do IMG-01. Um processo local na máquina do usuário na porta 3000 faria requisições autenticadas com os cookies de produção. Além disso, o middleware envia `Access-Control-Expose-Headers: Set-Cookie`, que não tem efeito (o navegador nunca expõe `Set-Cookie` ao JS).

**Ação esperada:** incluir `defaultDevOrigins` só em dev (ex.: quando `COOKIE_SECURE=false`, critério a confirmar pelo 🟣 SecBrain); remover `Set-Cookie` do `Access-Control-Expose-Headers`; documentar em `.env.example` e `deploy/api.env.example`.

**Critério de aceite:** em configuração de produção, `Origin: http://localhost:3000` não recebe `Access-Control-Allow-Origin` nem passa no `uploadPermitido`; em dev continua funcionando; o header `Expose-Headers` não cita `Set-Cookie`; testes de config e de middleware.

---

## BUG-24: troca de senha concorrente com reset admin sobrescreve uma à outra — prioridade BAIXA

**Status:** pendente
**Camada:** Backend (shared)
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** 🟣 SecBrain, especificação do Lote 27 (2026-10-31), achado fora do escopo nº 3 (levantado durante o BUG-15).

**Descrição:** depois do BUG-15 cada fluxo de senha é atômico, mas dois fluxos simultâneos para o mesmo usuário (troca da própria senha e reset admin) leem o hash atual fora da tx e gravam sem condição: a última gravação vence e o histórico pode registrar o `HashAnterior` errado (o hash lido antes da outra troca).

**Ação esperada:** `AtualizarSenha` condicionado ao hash lido (`... WHERE id = ? AND senha_hash = ?`), como já faz o `RegravarHash`; 0 linhas afetadas → erro de conflito (409 ou nova tentativa, decisão do 🟣 SecBrain), com rollback do histórico e da revogação na mesma tx.

**Critério de aceite:** teste com duas trocas concorrentes (ou sqlmock com 0 linhas afetadas) mostrando que só uma vence e que o histórico grava o `HashAnterior` correto; caminho normal sem mudança de contrato.

---

## CHORE-10: `ErrImagemMuitoGrande` com `KindMuitoGrande` e `maxLength={100}` nas buscas do frontend — prioridade BAIXA

**Status:** pendente
**Camadas:** Backend (shared + API), Frontend, Testes
**Responsável:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, especificação do Lote 27 (2026-10-31): achado fora do escopo nº 4 (opcional) e item "Frontend (opcional)" do SEC-25.

**Descrição:**
1. O SEC-14 criou `kernel.KindMuitoGrande` (→ 413), mas `catalogo.ErrImagemMuitoGrande` continua com outro `Kind` e o handler de imagem trata o 413 num caso especial de `responderErroImagem` (`API/internal/catalogohttp/imagem_handler.go`).
2. O SEC-25 limitou `q` a 100 caracteres (acima disso, 400), mas os campos de busca do frontend não têm limite, e o usuário só descobre o limite pelo erro.

**Ação esperada:** passar `catalogo.ErrImagemMuitoGrande` para `kernel.KindMuitoGrande` e remover o caso especial de `responderErroImagem`, mantendo a mensagem "imagem excede 1 MB"; colocar `maxLength={100}` nos campos de busca das telas de listagem (produtos, estoque, clientes, visitas, oportunidades, pedidos).

**Critério de aceite:** upload acima de 1 MB continua 413 "imagem excede 1 MB" (`img01_imagem_handler_test.go` verde); campos de busca não aceitam mais de 100 caracteres (teste de frontend); contrato HTTP inalterado.

---

## TST-11: teste instável `TestIMG01_Upload_FilenameHostilNaoVaza/CRLF_por_RFC_2231` — prioridade BAIXA

**Status:** pendente
**Camada:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** entregas do Lote 27 (2026-10-31), observado durante a verificação final.

**Descrição:** o subteste `CRLF_por_RFC_2231` de `TestIMG01_Upload_FilenameHostilNaoVaza` (suíte do IMG-01, `API/tests/handlers/`) falha quando o UUID aleatório gerado para o arquivo termina em "b" ("…b.png"): a asserção de que o fragmento hostil do filename não aparece na resposta casa por coincidência com o nome gerado. Falha intermitente, sem relação com o código de produção.

**Ação esperada:** tornar a asserção determinística (procurar o fragmento hostil completo, ou comparar contra o `imagem_url` devolvido excluindo o nome gerado, ou injetar um gerador de UUID fixo no teste).

**Critério de aceite:** o subteste passa em `go test -count=200 -run TestIMG01_Upload_FilenameHostilNaoVaza` sem falhas; a cobertura do caso hostil é mantida.

---

## GAPs de segurança da auditoria ARQ-00 (SEC-14 a SEC-25)

> Origem comum: 🟣 SecBrain, auditoria ARQ-00, Lote 21 (2026-10-13). Relatório completo em `docs/arq-00-invariantes-seguranca.md` (seção "GAPs"; itens 1 a 11). O item 12 (teste de matriz de rotas) não virou card: é feito pelo 🔴 TestBrain dentro do ARQ-00.
>
> **Todos os cards abaixo estão fora do escopo do Lote 21: não corrigir dentro dos cards ARQ. Executar após o ARQ-07 ou em lote próprio, por decisão do 🤍 MegaBrain/usuário.** Durante a refatoração, a regra é não piorar nenhum destes pontos.
>
> Caminhos: `API` = `apis/rotaperfumes-api`, `SH` = `apis/shared`. As linhas citadas são do código antes da refatoração DDD; depois do ARQ-07 o dev deve localizar o ponto equivalente na nova estrutura.
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
> - SEC-19: `SH/adapters/turnstile/turnstile.go`.
> - SEC-20: **concluído no Lote 24 (card em `feito.md`)**; `API/internal/httpx/clientip.go` (`IPDoCliente`).
> - SEC-21: **concluído no Lote 24 (card em `feito.md`)**; `API/internal/httpx/cookies.go` (`DefinirCookiesSessao`, `LimparCookiesSessao`).
> - SEC-22: **concluído no Lote 24 (card em `feito.md`)**; `SH/identidade/identidadeapp/login.go` (`Login`) e `sessao.go` (refresh).
> - SEC-23: `deve_trocar_senha` em `SH/identidade/usuario.go` e `SH/identidade/identidademysql/usuarios.go`; autenticação por request em `API/internal/httpx/auth.go`.
> - SEC-24: **concluído no Lote 27 (card em `feito.md`)**; enums de ordenação em `SH/catalogo/ordenacao.go`, `SH/comercial/ordenacao.go`, `SH/vendas/filtros.go`, `SH/identidade/usuario.go` e `SH/identidade/historico.go`, traduzidos por `switch` nos adapters `*mysql`; testes em `SH/tests/repositories/sort_test.go`.
> - SEC-25: **concluído no Lote 27 (card em `feito.md`)**; buscas `LIKE ?` em `SH/catalogo/catalogomysql/produtos.go`, `SH/comercial/comercialmysql/{clientes,oportunidades,visitas}.go` e `SH/vendas/vendasmysql/pedidos.go`.

---

## SEC-19: Turnstile não valida `hostname` nem `action` — prioridade MÉDIA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend (+ Frontend para o `action`)
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 6; invariante I-13).

**Descrição:** a verificação do Turnstile em login e reset de senha é fail-closed e vem antes do rate limit, mas só confere `success`; não confere se o `hostname` da resposta é o do sistema nem se o `action` corresponde ao fluxo (login vs. reset). Um token obtido em outro site com a mesma site key, ou em outro fluxo, é aceito.

**Ação esperada:** configurar os hostnames esperados (ex. `TURNSTILE_HOSTNAMES`) e conferir `hostname` e `action` na resposta do siteverify; o 🟢 FrontBrain passa `action` distinto em cada widget (`login`, `reset-senha`). Resposta inválida → 400 com a mensagem atual. Testes com resposta simulada de hostname/action errados.

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

---

## SEC-23: `deve_trocar_senha` não é imposto pelo backend — prioridade MÉDIA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend + Frontend
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 10).

**Descrição:** a flag `deve_trocar_senha` (`SH/models/usuario.go:21`; gravada em `SH/repositories/usuario_repository.go:276` e `:432`) só é devolvida no login (`API/handlers/auth_handler.go:322-324`) para o frontend redirecionar. O backend aceita todas as rotas protegidas com um token de usuário que ainda deveria trocar a senha gerada pelo sistema, então a obrigação depende só da tela.

**Ação esperada:** no middleware de autenticação (que já consulta o usuário a cada request, SEC-06), bloquear com 403 de mensagem fixa as rotas protegidas quando `deve_trocar_senha = true`, exceto a troca de senha, logout, refresh e `me`; o 🟢 FrontBrain trata o 403 redirecionando para a troca. Testes na matriz de rotas.

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

---

## TST-04: itens do roteiro do Lote 12 que testam o próprio `make` (1.1, 1.2 e 1.7) — prioridade MÉDIA

**Status:** pendente, **bloqueado** (depende do usuário); avançou parcialmente no Lote 20 (2026-10-11)
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

**Status:** pendente (acompanhar o upstream)
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain
**Origem:** residual do DEP-01 (Lote 20, 2026-10-11; ver `feito.md`).

**Descrição:** após a migração para Tailwind 4, o `npm audit` completo caiu de 9 para 5 vulnerabilidades (5 altas); `npm audit --omit=dev` = 0. As 5 vêm da cadeia `braces` → `micromatch` → `fast-glob@3.3.1` → `@next/eslint-plugin-next` → `eslint-config-next@16.4.0`, que só é usada pela ferramenta de lint (dev). O `braces` não tem versão corrigida, e o `npm audit fix --force` rebaixaria o `eslint-config-next` para 14.2.35 (rejeitado).

**Ação esperada:** acompanhar o upstream (`eslint-config-next`/`@next/eslint-plugin-next`, `fast-glob`, `micromatch`, `braces`). Quando houver correção, atualizar ou usar `overrides` de `fast-glob`/`micromatch` no `package.json` e validar com `tsc`, `eslint`, vitest, `next build` e `npm audit`. Não usar `npm audit fix --force`.

---

## CHORE-04: normalizar formatação (gofmt/CRLF) de 3 testes SEC12 de integração — prioridade BAIXA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** piloto ARQ-02 (Lote 21, 2026-10-14), achado do 🟡 BackBrain.

**Descrição:** os arquivos abaixo não passam no `gofmt` (formatação/finais de linha CRLF):
- `apis/shared/tests/repositories/sec12_chore02_refresh_integration_test.go`
- `apis/rotaperfumes-api/tests/handlers/sec12_http_integration_test.go`
- `apis/rotaperfumes-api/tests/services/sec12_chore02_integration_test.go`

**Ação esperada:** rodar `gofmt -w` e normalizar para LF (conferir `.gitattributes`), sem alterar a lógica, em commit separado.

**Critério de aceite:** `gofmt -l` vazio nos 3 arquivos; mesmos testes passando.

---

## CHORE-05: logs de auditoria de escrita registram só o role, não o id do usuário — prioridade BAIXA

**Status:** pendente (fora do escopo do Lote 21; executar **após o ARQ-03**, quando o Principal estiver disponível em todos os contextos)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** NB-4 do 🟣 SecBrain, revisão do ARQ-02 (Lote 21, 2026-10-14).

**Descrição:** os logs de auditoria de escrita registram só o role de quem fez a ação, por exemplo `[produtos] criado: id=%d por usuario role=%s`. Sem o id do usuário, não dá para saber quem alterou o registro.

**Ação esperada:** incluir o id do Principal em todos os logs de auditoria de escrita (create, update, delete e toggle) de todos os contextos, mantendo o role. Não registrar PII (e-mail ou nome).

**Pontos acrescentados no ARQ-03 (2026-10-14):** `apis/shared/identidade/identidadeapp/senha.go:169` e `apis/rotaperfumes-api/internal/identidadehttp/usuario_handler.go:59,82,106` registram `por admin=<role>` em vez do id.

**Critério de aceite:** todos os logs de escrita trazem o id e o role do usuário; testes de log por contexto; contrato HTTP inalterado.

---

> **2026-10-29:** CHORE-03 (id de path → 400) e CHORE-06 (duplicidade → 409) receberam a aprovação do usuário para a mudança de contrato HTTP e foram para o **Lote 25** (cards em `fazendo.md`). Aqui ficam CHORE-04 e CHORE-05.
