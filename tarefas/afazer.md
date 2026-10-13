# A Fazer

> Atualizado em 2026-10-13. O Lote 21 (2026-10-13) abriu a reestruturação da API em DDD + Clean Code + Design Patterns: o ARQ-00 (blueprint) está em `fazendo.md` e os cards ARQ-01 a ARQ-08 estão abaixo, na ordem de execução (sequencial, um commit por lote na branch `refactor/ddd`). Logo depois deles estão os cards SEC-14 a SEC-25, GAPs de segurança levantados pelo 🟣 SecBrain na auditoria do ARQ-00 (`docs/arq-00-invariantes-seguranca.md`), fora do escopo do Lote 21. Também saíram do ARQ-00 os cards ARQ-09 (Dinheiro em centavos), BUG-14 (VincularCliente sem transação) e BUG-15 (troca de senha sem transação), logo após o ARQ-08.
>
> **Regras do Lote 21 válidas para ARQ-01 a ARQ-07 (decisões de 2026-10-13, ver ARQ-00 em `fazendo.md`):**
> - **P1, logs (altera a regra LOG-02):** `vlog` em 4 níveis: rota automática no router, caso de uso via `defer`, pontos de decisão e 1 linha por query (~600 chamadas no lugar de ~3.726). Mesmo formato de log; TST-05 deve continuar verde. Nesses cards, o critério "logs `vlog` preservados" significa formato e TST-05 preservados e cobertura pelos 4 níveis, não a manutenção de cada chamada atual. O 🟡 BackBrain atualiza a regra no código (comentário de `apis/shared/vlog/vlog.go`); o 🔵 SubBrain atualiza a documentação no ARQ-08.
> - **X1, contrato HTTP:** a única exceção ao "contrato HTTP inalterado" é a do ARQ-01 (métodos das 13 rotas e `/api/senha-historico/2/extra`).
>
> Histórico anterior (2026-10-11): o 🔴 TestBrain executou `docs/roteiro-teste-manual-lote12.md` (24 aprovados, 1 falhou, 13 não verificáveis de 38; ver a seção "Execução (2026-10-07, 🔴 TestBrain)" no fim do roteiro). Daí saíram os cards BUG-13, LOG-01, DOC-05 e TST-04. BUG-13, LOG-01 e DOC-05 foram concluídos no Lote 16 (2026-10-07; ver `feito.md`). O LOG-02 (Lote 17) gerou o TST-05, e o DEP-00 (Lote 19) gerou o DEP-01 e o DEP-02. No Lote 20 (2026-10-11; ver `feito.md`) foram concluídos DEP-01, DEP-02 e TST-05; o TST-04 avançou parcialmente (só faltam 1.1, 1.2 e 1.7, que dependem do usuário rodar o `make`) e do DEP-01 saiu o card DEP-03 (BAIXA). O MAIL-01 e o DEPLOY-01 seguem em `fazendo.md`, aguardando o usuário.

---

## ARQ-01: fundação / shared kernel — prioridade ALTA

**Status:** pendente (Lote 21; depende do ARQ-00)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 21 (2026-10-13), reestruturação DDD da API.

**Descrição:** criar a base comum a todos os contextos:
- paginação e ordenação;
- erros de domínio e o mapeamento deles para HTTP;
- envelope de resposta;
- interfaces de repositório;
- Unit of Work / transação;
- composition root em `cmd/server/main.go`;
- builder de rotas e cadeia de middlewares.

**Mudança de contrato aprovada (X1, usuário, 2026-10-13):** as 13 rotas hoje registradas sem método passam a aceitar só o método documentado (405 para os demais), e `/api/senha-historico/2/extra` passa de 200 para 400 (rota `{user_id...}`). Atualizar a matriz de rotas do ARQ-00 (`tests/routes/arq00_matriz_rotas_test.go`) para refletir isso.

**Logs (P1):** o router passa a gerar o log automático de rota (nível 1 da nova regra LOG-02; ver bloco do lote no topo).

**Critério de aceite:**
- contrato HTTP inalterado (rotas, payloads, status codes e mensagens de erro), exceto a mudança X1 acima;
- `go build`, `go vet` e `go test ./...` verdes em `apis/rotaperfumes-api` e `apis/shared`;
- sem perda de cobertura;
- logs `vlog` preservados;
- commit do lote na branch `refactor/ddd`.

---

## ARQ-02: contexto Catálogo (Produto, Estoque), piloto — prioridade ALTA

**Status:** pendente (Lote 21; depende do ARQ-01)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 21 (2026-10-13), reestruturação DDD da API.

**Descrição:** migrar Produto e Estoque para a estrutura `domain`/`application`/`infrastructure`/`interfaces`. É o piloto que valida o blueprint e o shared kernel antes dos demais contextos.

**Critério de aceite:**
- contrato HTTP inalterado (rotas, payloads, status codes e mensagens de erro);
- `go build`, `go vet` e `go test ./...` verdes em `apis/rotaperfumes-api` e `apis/shared`;
- sem perda de cobertura;
- logs `vlog` preservados;
- commit do lote na branch `refactor/ddd`.

---

## ARQ-03: contexto Identidade e Acesso — prioridade ALTA (sensível a segurança)

**Status:** pendente (Lote 21; depende do ARQ-02)
**Camadas:** Segurança, Backend
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 21 (2026-10-13), reestruturação DDD da API.

**Descrição:** migrar:
- Auth/JWT e Usuário;
- RefreshToken com rotação e detecção de reuso;
- SenhaHistórico;
- hash Argon2, política de senha e gerador de senha;
- Turnstile e alerta de segurança;
- middlewares auth, user_status e ratelimit.

O 🟣 SecBrain revisa antes e depois, contra as invariantes de segurança do ARQ-00.

**Critério de aceite:**
- contrato HTTP inalterado (rotas, payloads, status codes e mensagens de erro);
- `go build`, `go vet` e `go test ./...` verdes em `apis/rotaperfumes-api` e `apis/shared`;
- sem perda de cobertura;
- logs `vlog` preservados;
- commit do lote na branch `refactor/ddd`.

---

## ARQ-04: contexto Comercial — prioridade ALTA

**Status:** pendente (Lote 21; depende do ARQ-03)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 21 (2026-10-13), reestruturação DDD da API.

**Descrição:** migrar:
- Vendedor e Carteira;
- Cliente, com CNPJ como value object;
- Visita e Oportunidade;
- escopo de acesso por carteira.

**Critério de aceite:**
- contrato HTTP inalterado (rotas, payloads, status codes e mensagens de erro);
- `go build`, `go vet` e `go test ./...` verdes em `apis/rotaperfumes-api` e `apis/shared`;
- sem perda de cobertura;
- logs `vlog` preservados;
- commit do lote na branch `refactor/ddd`.

---

## ARQ-05: contexto Vendas — prioridade ALTA

**Status:** pendente (Lote 21; depende do ARQ-04)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 21 (2026-10-13), reestruturação DDD da API.

**Descrição:** migrar Pedido como aggregate root (com ItemPedido) e Pagamento.

**Critério de aceite:**
- contrato HTTP inalterado (rotas, payloads, status codes e mensagens de erro);
- `go build`, `go vet` e `go test ./...` verdes em `apis/rotaperfumes-api` e `apis/shared`;
- sem perda de cobertura;
- logs `vlog` preservados;
- commit do lote na branch `refactor/ddd`.

---

## ARQ-06: contexto Analytics (Dashboard) — prioridade MÉDIA

**Status:** pendente (Lote 21; depende do ARQ-05)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 21 (2026-10-13), reestruturação DDD da API.

**Descrição:** migrar o Dashboard como read model (lado de consulta, no estilo CQRS).

**Critério de aceite:**
- contrato HTTP inalterado (rotas, payloads, status codes e mensagens de erro);
- `go build`, `go vet` e `go test ./...` verdes em `apis/rotaperfumes-api` e `apis/shared`;
- sem perda de cobertura;
- logs `vlog` preservados;
- commit do lote na branch `refactor/ddd`.

---

## ARQ-07: Notificação/SMTP, importadores, CLIs e remoção do legado — prioridade MÉDIA

**Status:** pendente (Lote 21; depende do ARQ-06)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 21 (2026-10-13), reestruturação DDD da API.

**Descrição:**
- migrar Notificação/SMTP para a nova estrutura, preservando o comportamento do MAIL-01;
- migrar os importadores e CLIs (`apis/shared/cmd`, `importers` e `tools`);
- remover os pacotes legados (`handlers`, `services`, `repositories` e os `models` antigos).

**Achado do 🔴 TestBrain (ARQ-00):** `apis/shared/tools/testeemail/testeemail_test.go` é teste interno ao pacote de produção (8 testes), fora de `tests/`. Decidir na migração: mover para a suíte de testes ou manter, sem perder os 8 testes da linha de base.

**Critério de aceite:**
- contrato HTTP inalterado (rotas, payloads, status codes e mensagens de erro);
- `go build`, `go vet` e `go test ./...` verdes em `apis/rotaperfumes-api` e `apis/shared`;
- sem perda de cobertura;
- logs `vlog` preservados;
- commit do lote na branch `refactor/ddd`.

---

## ARQ-08: documentação e fechamento do Lote 21 — prioridade MÉDIA

**Status:** pendente (Lote 21; depende do ARQ-07)
**Camada:** Documentação
**Responsável:** 🔵 SubBrain
**Origem:** Lote 21 (2026-10-13), reestruturação DDD da API.

**Descrição:**
- escrever o ADR da reestruturação;
- revisar `docs/arquitetura-ddd.md`;
- atualizar `.claude/agents/backend.md` e o `CLAUDE.md`, se aplicável;
- Postman (`postman/`): conferir; a única mudança de contrato é a X1 (405 nas 13 rotas que passam a ter método fixo e 400 em `/api/senha-historico/2/extra`). Ajustar requests e exemplos que dependam do comportamento antigo;
- atualizar a documentação da regra LOG-02 para o modelo de 4 níveis (P1): `docs/logs-verbose.md`, nota no histórico do `feito.md` e conferir se o comentário de `apis/shared/vlog/vlog.go` já foi atualizado pelo 🟡 BackBrain;
- fechar o Kanban e mover ARQ-00 a ARQ-08 para `feito.md`.

**Critério de aceite:**
- contrato HTTP inalterado (rotas, payloads, status codes e mensagens de erro), exceto a mudança X1 do ARQ-01;
- `go build`, `go vet` e `go test ./...` verdes em `apis/rotaperfumes-api` e `apis/shared`;
- sem perda de cobertura;
- logs `vlog` preservados;
- commit do lote na branch `refactor/ddd`.

---

## ARQ-09: valores monetários em centavos (`int64`) — prioridade BAIXA/MÉDIA

**Status:** pendente (sai do Lote 21; executar após o ARQ-08, em lote próprio)
**Camadas:** Database, Backend (Frontend se o formato do JSON mudar)
**Responsável:** 🌸 DataBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** decisão P3 do 🤍 MegaBrain no ARQ-00 (2026-10-13).

**Descrição:** no ARQ-05, o value object `Dinheiro` permanece `float64` encapsulado. Este card migra o cálculo interno para centavos (`int64`) e avalia o schema (colunas DECIMAL) e o contrato JSON.

**Risco:** arredondamento em `valor_total` (soma de itens, descontos e pagamentos). Os valores calculados podem divergir em centavos dos já gravados; é preciso um teste de regressão comparando os totais antigos com os novos e uma regra de arredondamento explícita.

**Critério de aceite:** cálculos monetários sem `float64`; totais existentes reproduzidos sem diferença; contrato JSON inalterado (ou mudança aprovada pelo usuário); testes verdes sem perda de cobertura.

---

## BUG-14: `VendedorService.VincularCliente` sem transação — prioridade MÉDIA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** decisão P4 do 🤍 MegaBrain no ARQ-00 (2026-10-13).

**Descrição:** `apis/rotaperfumes-api/services/vendedor_service.go:427-529` (linhas de antes da refatoração) encerra o vínculo antigo do cliente e cria o novo **sem transação**. Uma falha entre as duas operações deixa o cliente sem vendedor ou com vínculo inconsistente.

**Ação esperada:** executar as duas operações numa única transação (Unit of Work do ARQ-01, se já existir); teste que simula falha na criação do novo vínculo e confirma o rollback do encerramento. Durante a refatoração (ARQ-04), preservar o comportamento atual e não piorar o ponto.

**Critério de aceite:** vínculo antigo e novo atômicos; teste de falha/rollback; contrato HTTP inalterado.

---

## BUG-15: troca de senha não é transacional — prioridade MÉDIA

**Status:** pendente (fora do escopo do Lote 21; executar após o ARQ-03)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** revisão do blueprint pelo 🟣 SecBrain no ARQ-00 (2026-10-13), registrada pelo 🤍 MegaBrain.

**Descrição:** `ResetPassword` (troca da própria senha, `apis/rotaperfumes-api/handlers/auth_handler.go`) e o reset feito pelo admin gravam o histórico de senha, o novo hash, o corte `tokens_validos_desde` e a revogação dos refresh tokens em chamadas separadas, **sem transação**. Uma falha no meio deixa estado parcial (ex.: senha trocada sem revogar sessões, ou histórico gravado sem a troca).

**Ação esperada:** envolver as operações numa Unit of Work (do ARQ-01) quando o contexto Identidade e Acesso estiver migrado (após o ARQ-03). As expectativas de sqlmock das suítes SEC precisarão mudar (Begin/Commit/Rollback): fazer isso neste card, não dentro dos cards ARQ. Teste de falha em cada etapa confirmando o rollback completo.

**Critério de aceite:** troca e reset de senha atômicos; testes de falha/rollback; suítes SEC ajustadas e verdes; contrato HTTP inalterado.

---

## GAPs de segurança da auditoria ARQ-00 (SEC-14 a SEC-25)

> Origem comum: 🟣 SecBrain, auditoria ARQ-00, Lote 21 (2026-10-13). Relatório completo em `docs/arq-00-invariantes-seguranca.md` (seção "GAPs"; itens 1 a 11). O item 12 (teste de matriz de rotas) não virou card: é feito pelo 🔴 TestBrain dentro do ARQ-00.
>
> **Todos os cards abaixo estão fora do escopo do Lote 21: não corrigir dentro dos cards ARQ. Executar após o ARQ-07 ou em lote próprio, por decisão do 🤍 MegaBrain/usuário.** Durante a refatoração, a regra é não piorar nenhum destes pontos.
>
> Caminhos: `API` = `apis/rotaperfumes-api`, `SH` = `apis/shared`. As linhas citadas são do código antes da refatoração DDD; depois do ARQ-07 o dev deve localizar o ponto equivalente na nova estrutura.

---

## SEC-14: limite de tamanho do corpo (MaxBytesReader) e ReadHeaderTimeout — prioridade MÉDIA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend
**Responsável:** 🟣 SecBrain (spec dos limites) → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 1; invariante F-04).

**Descrição:** o servidor tem Read/Write 15s e Idle 60s (`API/cmd/server/main.go:117-144`), mas não limita o tamanho do corpo das requisições (nenhum `http.MaxBytesReader`) nem define `ReadHeaderTimeout`. Um cliente pode enviar corpos JSON grandes ou segurar headers lentamente (slowloris), consumindo memória e conexões.

**Ação esperada:** aplicar `http.MaxBytesReader` (limite global, ex. 1 MiB, com exceção explícita se alguma rota precisar de mais) na cadeia de middlewares ou no decode de JSON, respondendo 413 com mensagem fixa; definir `ReadHeaderTimeout` (ex. 5s) no `http.Server`. Testes: corpo acima do limite → 413; corpo normal inalterado.

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

---

## SEC-15: `maskDSN` vaza parte da senha quando ela contém `@` — prioridade MÉDIA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend (infra/banco)
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 2; invariante F-08).

**Descrição:** `maskDSN` (`SH/db/db.go:51-84`) mascara a senha do DSN antes de logar, mas quando a senha contém `@` o corte é feito no ponto errado e um trecho da senha aparece no log.

**Ação esperada:** mascarar a partir do **último** `@` antes do endereço (ou montar a string de log a partir da `mysql.Config` já parseada, sem nunca incluir `Passwd`). Testes de tabela com senhas contendo `@`, `:`, `/` e `%`, assegurando que nenhum caractere da senha aparece na saída.

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

---

## SEC-16: parâmetro `page` sem teto na paginação — prioridade BAIXA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 3; invariante F-10).

**Descrição:** a paginação limita `limit` (default 20, máx. 100) e exige `page ≥ 1` (`API/services/usuario_service.go:403-424`), mas não impõe teto a `page`. Valores enormes geram `OFFSET` gigantes (consulta cara no MySQL) e podem estourar a multiplicação `(page-1)*limit`.

**Ação esperada:** definir teto para `page` (ou para o OFFSET resultante) no VO `Pagina` do shared kernel criado no ARQ-01, com clamp ou 400 de mensagem fixa (decisão do 🟣 SecBrain, mantendo o contrato documentado); teste com `page` muito grande e com overflow.

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

---

## SEC-17: CORS com `"*"` reflete qualquer origem com credentials — prioridade MÉDIA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend (middleware) + Deploy
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 4; invariante F-02).

**Descrição:** o middleware de CORS (`API/middleware/cors_middleware.go:16-69`) usa allowlist exata, mas se a configuração contiver `"*"` ele reflete qualquer `Origin` junto com `Access-Control-Allow-Credentials: true`, o que permite que qualquer site faça requisições autenticadas com os cookies do usuário.

**Ação esperada:** rejeitar `"*"` quando credentials está ativo (falhar na validação da config, impedindo a API de subir, ou ignorar o curinga com log de aviso); documentar no `.env.example`/`deploy/api.env.example`. Teste: config com `"*"` → erro de config (ou origem arbitrária sem `Access-Control-Allow-Origin`).

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

---

## SEC-18: JWT sem validação de issuer/`exp` obrigatório e `JWT_SECRET` sem tamanho mínimo — prioridade ALTA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend (shared)
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 5; invariante I-01, F-06).

**Descrição:** o token é emitido com `Issuer: cfg.JWTIssuer` (`SH/services/auth_service.go:82`) e a validação aceita só HS256 (`SH/services/auth_service.go:99`), mas o parser não exige o issuer nem a presença de `exp` (há só uma checagem defensiva). Além disso, `JWT_SECRET` é obrigatório (`SH/config/config.go`), mas sem tamanho mínimo: um segredo curto torna o HS256 vulnerável a força bruta offline.

**Ação esperada:** no parse, usar `jwt.WithIssuer(cfg.JWTIssuer)`, `jwt.WithExpirationRequired()` e `jwt.WithValidMethods([]string{"HS256"})`; exigir `JWT_SECRET` com pelo menos 32 bytes na carga da config (mesma linha do `PASSWORD_PEPPER`), sem incluir o valor na mensagem. Atualizar `.env.example`/`deploy/api.env.example`. Testes: token com issuer errado → 401; token sem `exp` → 401; segredo curto → erro de config. Avaliar impacto nos tokens em circulação (todos já são emitidos com issuer e exp).

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

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

## SEC-20: IP do cliente pega o valor mais à esquerda do `X-Forwarded-For` — prioridade ALTA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend + Deploy
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 7; invariante I-14).

**Descrição:** com `TRUST_PROXY_HEADERS=true`, `getClientIP` (`API/handlers/auth_handler.go:908-923`, em especial 912-915) usa o primeiro item do `X-Forwarded-For`, que é controlado pelo cliente quando o proxy **acrescenta** em vez de sobrescrever. Isso permite contornar o rate limit de login/refresh/reset trocando o IP forjado a cada tentativa. No deploy atual o Caddy sobrescreve o header (DEPLOY-01), mas o código não deveria depender disso.

**Ação esperada:** usar o item mais à direita (o IP visto pelo proxy confiável) ou ler o header a partir da direita descontando N proxies confiáveis configurados; validar que o valor é um IP (`net.ParseIP`), com fallback para `RemoteAddr`. Testes com XFF de vários saltos e valores inválidos.

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

---

## SEC-21: flag `Secure` do cookie decidida por `r.Host` e limpeza de cookies sem atributos — prioridade ALTA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 8; invariante I-15).

**Descrição:** `setTokenCookies` (`API/handlers/auth_handler.go:126-159`) desliga `Secure` quando `r.Host` começa com `localhost`, `127.0.0.1` ou `0.0.0.0` (linhas 128-130). O `Host` vem do cliente, então um header forjado (ou um host como `localhost.atacante.com`, que passa no `HasPrefix`) faz o servidor emitir os tokens sem `Secure`. Além disso, `clearTokenCookies` (linhas 162-177) apaga os cookies sem `HttpOnly`, `Secure` e `SameSite`, divergindo dos atributos de criação.

**Ação esperada:** decidir `Secure` por configuração explícita (ex. `COOKIE_SECURE`, default true; false só em dev) em vez de `r.Host`; `clearTokenCookies` com os mesmos atributos da criação. Testes: Host forjado não desliga `Secure`; Set-Cookie de logout com os atributos corretos.

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

---

## SEC-22: login revela que o usuário existe e está inativo (mensagem e timing) — prioridade ALTA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain (+ 🟢 FrontBrain se a tela tratar a mensagem)
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 9; invariante I-16).

**Descrição:** no `Login` (`API/handlers/auth_handler.go:266-275`), o usuário inativo é checado **antes** da verificação de senha e recebe 401 "usuário inativo", sem rodar o hash. Isso quebra a anti-enumeração: pela mensagem e pelo tempo de resposta (sem custo do Argon2) dá para saber que a conta existe e está inativa, mesmo sem saber a senha.

**Ação esperada:** verificar a senha (ou rodar o hash descartável) antes de qualquer decisão e responder ao inativo com o mesmo 401 "credenciais inválidas" e o mesmo custo. Se o produto quiser informar "conta inativa", só depois de a senha estar correta (decisão do 🤍 MegaBrain/usuário). Conferir também o refresh (linhas 447-448). Testes de mensagem idêntica e de que o hash roda nos três casos (inexistente, inativo, senha errada).

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

## SEC-24: testes de injeção via `order_by` só cobrem Produto e SenhaHistorico — prioridade MÉDIA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Testes
**Responsável:** 🔴 TestBrain (com 🟣 SecBrain definindo os payloads)
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (GAP 11; invariante C-02).

**Descrição:** a whitelist de ordenação (`SH/repositories/sort.go:29-46`) é usada por vários repositórios (Produto `produto_repository.go:60`; Estoque com duas whitelists em `estoque_repository.go:77,83`; demais listagens), mas os testes de injeção (`ST/repositories/sort_test.go` e afins) só exercitam Produto e SenhaHistorico.

**Ação esperada:** teste de tabela cobrindo todo repositório/endpoint que aceita `order_by`/direção, com payloads maliciosos (`id; DROP`, `(SELECT ...)`, `1,2`, direção inválida), assegurando fallback para o default e nenhum texto livre no SQL. Após o ARQ-02, apontar para o enum tipado de ordenação.

**Nota:** fora do escopo do Lote 21 — não corrigir dentro dos cards ARQ; executar após o ARQ-07 ou em lote próprio, decisão do 🤍 MegaBrain/usuário.

---

## SEC-25: buscas com LIKE não escapam `%` e `_` — prioridade BAIXA

**Status:** pendente (fora do escopo do Lote 21)
**Camada:** Backend (shared)
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** 🟣 SecBrain, auditoria ARQ-00, Lote 21 (invariante C-03, "GAP baixo"; card criado por decisão do 🤍 MegaBrain).

**Descrição:** as buscas por texto (`q`) usam LIKE parametrizado, então não há injeção de SQL, mas os curingas `%` e `_` digitados pelo usuário não são escapados. Exemplo: `SH/repositories/produto_repository.go:95`; o mesmo vale para as demais buscas `q` com LIKE nos repositórios. Com isso, uma busca por `%` ou `_` devolve tudo ou dá resultado inesperado, e padrões como `%a%b%c%` deixam a consulta mais cara.

**Ação esperada:** criar um helper único de escape (`\` → `\\`, `%` → `\%`, `_` → `\_`) e usá-lo em todas as buscas LIKE, com `ESCAPE '\\'` explícito no SQL. Depois do ARQ-02, colocar o helper na infraestrutura compartilhada. Testes: busca por `%` e `_` encontra só os registros que têm esses caracteres literalmente.

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
