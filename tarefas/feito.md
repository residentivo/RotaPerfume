# Concluídas ✅

> Histórico de tarefas finalizadas.

---

## Lote 6 de 2026-09-26: 7 cards concluídos (de 7)

> Lote aberto pelo 🤍 MegaBrain a partir de `afazer.md` e executado na ordem 🟣 SecBrain → 🌸 DataBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → 🔵 SubBrain, com os cards SEC-06, SEC-07 (opção B, decisão do usuário), FE-07 (opção A, decisão do usuário), BUG-08, FE-09, TEST-01 e DOC-03. **Aceite do usuário em 2026-09-26.**
>
> **Regressão 🔴 TestBrain (2026-09-26):**
> - Go: unit e `INTEGRATION=1` ok, com 1202 PASS em `apis/shared` e 2354 em `rotaperfumes-api`, 0 FAIL.
> - Frontend: `tsc`, `eslint` e `vitest` com 1188/1188.
> - Cobertura: `rotaperfumes-api` 88,6%, `apiClient.ts` 94,81% e `Table.tsx` 100% das linhas.
> - Banco restaurado e idêntico ao snapshot.
>
> **Documentação (🔵 SubBrain) do lote:** `postman/README.md` e `postman/collection.json` (seção "Mudanças de contrato do Lote 6": SEC-06, SEC-07, BUG-08, FE-07 e DOC-03), `docs/manual-base-de-dados.md` (seção 7: `refresh_tokens` e migração 21) e o roteiro `docs/roteiro-teste-manual-lote6.md`. O `postman/collection.json` foi validado com Node pelo 🤍 MegaBrain em 2026-09-26.
>
> **Cards derivados que ficaram em `afazer.md`:** SEC-08, BUG-09, FE-10, SEC-09 e DOC-04.

## SEC-06: sessão de usuário inativado continua válida até o access token expirar — 2026-09-26
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain → roteiro do Lote 6 (seção 1) → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-26.

**Camada:** Backend (+ Segurança)
**Origem:** 🔵 SubBrain, DOC-01 (2026-09-25). Prioridade subiu de BAIXA para MÉDIA.

**Descrição:**
- O middleware de autenticação não conferia `usuarios.ativo`.
- Quando um vendedor era desligado pela tela, os usuários dele eram inativados, mas a sessão aberta continuava até o access token (24h) expirar. O rebaixamento de role também só valia no próximo token.

**Decisão 🟣 SecBrain (2026-09-26):** checar `ativo, role` no banco a cada requisição protegida, sem cache e fail-closed, e revogar os refresh tokens ao inativar o usuário e ao desligar o vendedor.

**🟡 BackBrain (2026-09-26) — implementado:**
- `apis/rotaperfumes-api/middleware/user_status.go`: `UserStatusChecker` e `DBUserStatusChecker`.
- `apis/rotaperfumes-api/middleware/auth_middleware.go`: `JWTMiddlewareWithUserCheck`. Inativo ou inexistente → `401 "usuário inativo"`; erro de banco → `500`; a role vem do banco.
- `apis/rotaperfumes-api/routes/routes.go`: `NewMux` recebe o checker (helper `jwt`). `cmd/server/main.go` passa `NewDBUserStatusChecker(conn)`.
- `apis/shared/repositories/usuario_repository.go`: `GetStatusByID`.
- Revogação dos refresh tokens (`inativacao`) ao inativar o usuário (`handlers/usuario_handler.go`) e ao desligar o vendedor (`services/vendedor_service.go`, `RevokeAllByVendedorID` na mesma transação).

**🔴 TestBrain (2026-09-26) — testado:** `handlers/lote6_http_integration_test.go` (inativar usuário, desligar vendedor, rebaixamento de role, usuário inexistente) e `middleware/user_status_cookie_test.go`. Achado: o access token volta a valer se o usuário for reativado dentro do TTL (card SEC-08 em `afazer.md`).

---

## SEC-07: revogar a família de refresh tokens em caso de reuso fora da janela — 2026-09-26
**Agentes:** 🟣 SecBrain → decisão do usuário (opção B) → 🌸 DataBrain → 🟡 BackBrain → 🔴 TestBrain → roteiro do Lote 6 (seção 5) → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-26.

**Camada:** Backend (+ Segurança) + Database
**Origem:** 🟣 SecBrain, SEC-04 (2026-09-25).

**Descrição:** O reuso de refresh token revogado fora da janela de graça só gerava log `[seguranca]`, sem `user_id`, e dava falso positivo para tokens revogados por logout.

**Decisão do usuário (2026-09-26):** opção B do 🟣 SecBrain (coluna `revoked_reason`; só o reuso de token com motivo `rotacao` alerta e revoga todas as sessões).

**🌸 DataBrain (2026-09-26) — implementado:**
- Migração 21: `sql/21_alter_refresh_tokens_revoked_reason.sql` e `sql/21_revert_refresh_tokens_revoked_reason.sql`. Cria `refresh_tokens.revoked_reason` ENUM('rotacao','logout','revogacao_massa','senha','inativacao') NULL. Idempotente.
- DDL base `sql/06_ddl_refresh_tokens.sql` atualizado. `Makefile`: `db-fix-revoked-reason` e `db-revert-revoked-reason`.
- Aplicada no banco local (107 tokens; os legados ficaram com NULL).

**🟡 BackBrain (2026-09-26) — implementado:**
- `RevokeReason` gravado em todas as revogações (`apis/shared/repositories/refresh_token_repository.go`).
- `RevokedTokenError` tipado (`services/refresh_token_service.go`).
- `handlers/auth_handler.go`: reuso de token `rotacao` fora da janela de 30 s → alerta `[auth][seguranca]` com user_id, token_id, IP e UA, mais `RevokeAllUserTokens(revogacao_massa)`. Outros motivos e NULL → só log informativo. A resposta 401 continua idêntica.
- Os `_ =` que engoliam erro (revogação e `senhaSvc.Registrar`) viraram log.

**🔴 TestBrain (2026-09-26) — testado:** reuso em `handlers/lote6_http_integration_test.go`, mais os testes de service, rotação e repositório.

---

## FE-07: assimetria de logout entre falha de rede no refresh e falha de rede no retry — 2026-09-26
**Agentes:** 🟣 SecBrain → decisão do usuário (opção A) → 🟢 FrontBrain → 🔴 TestBrain → roteiro do Lote 6 (seção 3) → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-26.

**Camada:** Frontend (+ Segurança)
**Origem:** 🔴 TestBrain, regressão do Lote 5 (2026-09-25).

**Descrição:** A falha de rede ou timeout no próprio `POST /api/auth/refresh` (e a falha do Web Lock) fazia logout e chamava `/api/auth/logout`. A falha de rede no retry (FE-06) não fazia.

**Decisão do usuário (2026-09-26):** opção A do 🟣 SecBrain.

**🟢 FrontBrain (2026-09-26) — implementado:** `frontend/src/lib/apiClient.ts`. Falha de rede, timeout ou falha do Web Lock no refresh → `NetworkError`, sem logout e sem chamar `/api/auth/logout`; a fila recebe o mesmo erro. `401`, `429` e `500` continuam deslogando.

**🔴 TestBrain (2026-09-26) — testado:** `frontend/src/lib/apiClient.lote6.test.ts`. Cobertura do `apiClient.ts`: 94,81%.

---

## BUG-08: `POST /api/clientes` devolve `created_at` e `updated_at` zerados — 2026-09-26
**Agentes:** 🟡 BackBrain → 🔴 TestBrain → roteiro do Lote 6 (seção 4) → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-26.

**Camada:** Backend
**Origem:** 🔴 TestBrain, execução do roteiro do Lote 5 (2026-09-25). É anterior ao lote.

**Descrição:** A resposta 201 trazia `0001-01-01T00:00:00Z` nesses campos, porque o service devolvia o objeto em memória.

**🟡 BackBrain (2026-09-26) — implementado:** `apis/rotaperfumes-api/services/cliente_service.go`, `relerClienteCriado` depois de `CreateCliente` e de `CreateClienteNaCarteira`. O PUT já relia. O mesmo padrão nos outros creates virou o card BUG-09 (`afazer.md`).

**🔴 TestBrain (2026-09-26) — testado:** `handlers/lote6_http_integration_test.go` (com e sem carteira).

---

## FE-09: listagem de clientes mostra "Nenhum cliente cadastrado." quando há erro de rede — 2026-09-26
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → roteiro do Lote 6 (seção 2) → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-26.

**Camada:** Frontend
**Origem:** 🔴 TestBrain, execução do roteiro do Lote 5 (2026-09-25). É anterior ao lote.

**Descrição:** Com erro de rede, a listagem zerava a lista e mostrava "Nenhum cliente cadastrado." ao lado do alerta de conexão. Outras listagens repetiam o padrão.

**🟢 FrontBrain (2026-09-26) — implementado:**
- `frontend/src/components/ui/Table.tsx`: prop `erroCarga`, que oculta o estado vazio.
- Aplicada em 10 listagens: clientes, vendedores, usuários, produtos, senha-historico, estoque, pedidos, oportunidades, visitas e pagamentos.
- Dashboard: oculta os estados vazios quando a primeira carga dá erro.
- Comportamentos antigos observados e registrados no card FE-10 (`afazer.md`).

**🔴 TestBrain (2026-09-26) — testado:** `Table.test.tsx` e `app/admin/listasAdmin.test.tsx`. `Table.tsx` com 100% das linhas.

---

## TEST-01: nome de subteste e comentário de CNPJ desatualizados — 2026-09-26
**Agentes:** 🔴 TestBrain → regressão do Lote 6 (tabela "Testes automatizados" do roteiro) → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-26.

**Camada:** Testes
**Origem:** 🔴 TestBrain, regressão do Lote 5 (2026-09-25).

**Descrição:** O subteste "CNPJ mascarado é gravado só com dígitos" não descrevia mais a regra do NEG-02, e o comentário de `TestCruzado_Divergencias` ainda citava `it.fails` no front.

**🔴 TestBrain (2026-09-26) — feito:** subteste renomeado em `apis/rotaperfumes-api/handlers/lote4_http_integration_test.go:254` e comentário ajustado em `apis/shared/cnpj/cruzado_test.go:75-79`.

---

## DOC-03: outros documentos e o Postman ainda usam `henrique.rodrigues` (inativo) — 2026-09-26
**Agentes:** 🔵 SubBrain → conferência do 🔴 TestBrain → correção do 🔵 SubBrain → validação do JSON pelo 🤍 MegaBrain → roteiro do Lote 6 (item 4.3) → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-26.

**Camada:** Documentação
**Origem:** 🔵 SubBrain, DOC-01 (2026-09-25).

**Descrição:** O usuário id 2 (`henrique.rodrigues`) está com `ativo=0` e não faz login, mas ainda aparecia nos roteiros de clientes e do Dashboard e no Postman.

**🔵 SubBrain (2026-09-26) — primeira parte:**
- Roteiros de clientes e do Dashboard: perfil "normal com vendedor desligado" trocado para `thiago.silva` (id 4, vendedor 3).
- Postman ("Login — Vendedor"): trocado para `rafael.carvalho` (id 5, vendedor 4 ativo), com `captchaToken` no body.

**🔴 TestBrain (2026-09-26) — conferência:** encontrou três defeitos de documentação:
- a senha `Mudar@123` não confere com o banco para os ids 4 e 5;
- o id 5 tem `deve_trocar_senha=0`, e o teste da collection esperava `trocar_senha: true`;
- a lista de desligados do roteiro do Dashboard citava o id 3 (vendedor 2, ativo).

**🔵 SubBrain (2026-09-26) — correção:**
- **Senha:** `postman/README.md` (passo 4, exemplos e nota) e `postman/collection.json` (descrição da coleção, descrição do Login, body, descrição e exemplo do "Login — Vendedor") remetem a `SEED_USER_PASSWORD` ou à senha impressa pelo `make db-seed`. Nova variável `{{senha_vendedor}}` na collection. Os roteiros em `docs/` não citam `Mudar@123`. O admin `Admin@123` ficou.
- **`trocar_senha`:** o teste do "Login — Vendedor" confere só que o campo é booleano. A descrição e o README documentam os dois casos, e o exemplo 200 passou a `false`, o estado atual do id 5.
- **Desligados:** `docs/roteiro-teste-manual-dashboard.md` cita os ids 10, 38 e 39, e o item 3.5 cita 66 clientes em carteira ativa do vendedor 3, 62 ativos (conferido em 2026-09-26).

**🤍 MegaBrain (2026-09-26) — validação do JSON:** `postman/collection.json` validado com Node: JSON válido, com a variável `senha_vendedor` presente.

---

## BUG-07: modal "Editar Vendedor" sobrescreve a data de admissão e a meta mensal — prioridade ALTA — 2026-09-26
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain (2026-09-26) → aceite do usuário (2026-09-26) → fechamento por 🔵 SubBrain. 🟣 SecBrain dispensado: é um bug de UI sem superfície nova.

**Status:** concluído — aceite do usuário em 2026-09-26.

**Camada:** Frontend
**Origem:** 🔴 TestBrain, execução do roteiro do Lote 5 via Playwright (2026-09-25). É anterior ao lote.

**Descrição:** O modal abria com a data de admissão de hoje e meta mensal 0, e não com os valores do banco (vendedor 4: 2023-06-22 e 55000). Salvar sem mudar nada enviava `{"data_admissao":"2026-09-25","meta_mensal":0,...}`, e esses valores sobrescreviam os dados reais.
- `frontend/src/app/admin/vendedores/page.tsx:196-211` passava `data_admissao: ""` e `meta_mensal: 0` como valor provisório, porque `GET /api/vendedores` não traz esses campos (projeção do SEC-03).
- `frontend/src/components/admin/VendedorModal.tsx:143-152` preenchia o formulário só com esse valor provisório. O efeito de `:174-183` buscava o detalhe, mas usava apenas `detalhe.clientes`.

**🟢 FrontBrain (2026-09-26) — implementado:**
- `frontend/src/components/admin/VendedorModal.tsx`:
  - Novo helper `toDateInput()`, que normaliza a data para `YYYY-MM-DD`.
  - No modo edição, o modal ignora `data_admissao`/`meta_mensal` da listagem e preenche nome, regiao, uf, data_admissao e meta_mensal pelo `GET /api/vendedores/{id}`.
  - O salvar fica bloqueado enquanto o detalhe carrega ou se ele falhar: `fieldset disabled`, botão disabled com tooltip e guarda no `handleSubmit`.
  - Mostra um alerta "Carregando dados do vendedor..." e um alerta de erro.
  - O detalhe é descartado quando o modal troca de vendedor.
- `frontend/src/app/admin/vendedores/page.tsx`: só um comentário.

**🔴 TestBrain (2026-09-26) — testado:**
- `frontend/src/components/admin/FormModais.test.tsx`: 5 testes do FrontBrain e mais 28 do TestBrain.
- `vitest`: 1134/1134. `tsc` e `eslint` ok.
- Cobertura do `VendedorModal.tsx`: 99,4% das linhas e 88,73% dos branches.
- E2E no navegador não executado: o Turnstile exige Chrome + CDP, e o teste gravaria no banco.

**Verificação do banco (somente leitura, 2026-09-26):**
- 42 vendedores. Nenhum com `meta_mensal = 0`, nenhum com `data_admissao` em 2026-09 ou igual à data do `updated_at`. Nenhum vendedor foi sobrescrito.
- Vendedora id 30 (Carla Lopes): o usuário confirmou que a divergência de data_desligamento veio de um teste feito com ela. Não tem relação com o BUG-07. Dados não alterados.

**Para o aceite:** como admin, abra Vendedores, clique em "Editar" no vendedor 4 e confira 2023-06-22 e 55000.

---

## Lote 5 de 2026-09-25: 7 cards concluídos (de 7)

> Lote executado pelo 🤍 MegaBrain na ordem 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → 🔵 SubBrain, com os cards SEC-04, FE-06, DOC-01, DOC-02, NEG-02, DB-02 e FE-08. **Aceite do usuário em 2026-09-25.**
>
> **Regressão 🔴 TestBrain (2026-09-25):**
> - Go: `apis/shared` e `rotaperfumes-api` ok; integração com `INTEGRATION=1` ok.
> - Frontend: `tsc`, lint e `vitest` ok.
>
> **Roteiro executado via Playwright pelo 🔴 TestBrain (2026-09-25):**
> - Roteiro: `docs/roteiro-teste-manual-lote5.md`, seção "Execução 2026-09-25 (TestBrain, Playwright)".
> - Resultado: SEC-04, FE-06, FE-08, NEG-02, DOC-01, DOC-02 e DB-02 OK, sem defeitos nos cards do lote. O banco foi restaurado (3000 clientes, 0 resíduos).
> - Não executados: 1.2 e 1.8. Os logs `[auth]` e `[auth][seguranca]` só aparecem no Debug Console do VS Code.
>
> **Documentação (🔵 SubBrain) do lote:** `postman/README.md`, `postman/collection.json` (JSON validado com Node) e `docs/manual-base-de-dados.md` foram atualizados com NEG-02, SEC-04 e FE-06.
>
> **Cards derivados que ficaram em `afazer.md`:** BUG-08, FE-09, DOC-03, SEC-06, SEC-07, DOC-04, FE-07 e TEST-01. O BUG-07 (ALTA) foi concluído em 2026-09-26.

## SEC-04: corrida legítima no refresh conta no rate limit por IP — 2026-09-25
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain → roteiro do Lote 5 → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-25.

**Camada:** Backend (+ Segurança)
**Origem:** 🔴 TestBrain (2026-09-25), follow-up do SEC-02.

**Descrição:**
- Cada `401` de revogação concorrente no refresh contava no `refreshLimiter` por IP. Com 10 falhas, a API respondia `429`.
- No front, um `429` no refresh levava ao logout.
- Várias abas, ou vários usuários atrás do mesmo IP (NAT), podiam cair nisso.

**Decisão 🟣 SecBrain (2026-09-25):**
- **Janela de graça:** `RefreshRevokeGraceWindow = 30s`. Um token revogado há ≤ 30s devolve `ErrRefreshTokenRevokedRecently` e **não** conta no `refreshLimiter`.
- **Fora da janela:** continua contando e gera log `[seguranca]` de possível reuso.
- **Corrida no `BeginRotation`:** deixa de contar.
- **Continua contando:** `NotFound` e `Expired`.
- **Sem mudança:** a resposta HTTP é idêntica, a rotação continua de uso único e o front não muda.
- **Recomendação futura** (fora do card): revogar a família de tokens no reuso (card SEC-07 em `afazer.md`).

**🟡 BackBrain (2026-09-25) — implementado:**
- **Código:** `refresh_token_service.go` (janela de graça, relógio injetável) e `auth_handler.go` (`Recently` não conta; a corrida no `BeginRotation` não conta; log `[seguranca]` fora da janela).
- **Testes:** 7 casos de service, 5 testes de handler novos e a integração SEC-02 com 3 rodadas de 15 refreshes paralelos. Resultado: `go build`, `go vet` e `go test ./services/... ./handlers/...` ok; integração com `INTEGRATION=1` ok.

---

## FE-06: mensagem enganosa quando o retry após o refresh falha por rede — 2026-09-25
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → roteiro do Lote 5 → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-25.

**Camada:** Frontend
**Origem:** 🔴 TestBrain (2026-09-25), follow-up do SEC-02.

**Descrição:** Em `frontend/src/lib/apiClient.ts`, quando o retry após o refresh com `401` falhava por erro de rede, as requisições da fila eram rejeitadas com "Sessão expirada. Faça login novamente.", mas o usuário não era deslogado. A mensagem era enganosa.

**🟢 FrontBrain (2026-09-25) — implementado:**
- **Comportamento:** quando o retry falha por rede, as requisições da fila recebem o mesmo erro de rede da requisição original (`onRefreshComplete(false, erro)`), sem logout. "Sessão expirada" só aparece quando há logout.
- **Testes:** 3 testes novos ou ajustados em `apiClient.test.ts`. Resultado: `tsc` e `eslint` sem erros e `vitest` com 703/703 testes passando.
- **Observação:** o projeto não tinha uma mensagem amigável própria para erro de rede; isso virou o card FE-08 (abaixo).

---

## DOC-01: roteiro de vendedor desligado usa um usuário que não faz mais login — 2026-09-25
**Agentes:** 🔴 TestBrain → 🔵 SubBrain → conferência do 🔴 TestBrain → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-25.

**Camada:** Documentação
**Origem:** 🔴 TestBrain (2026-09-25).

**Descrição:** `docs/roteiro-teste-manual-vendedor-desligado.md` usava `henrique.rodrigues` (usuário id 2), que está com `ativo=0` (efeito do BUG-05) e não faz mais login.

**O que foi feito:**
- **🔵 SubBrain (2026-09-25):** roteiro atualizado para `thiago.silva` (usuário id 4, vendedor 3 desligado).
- **🔴 TestBrain (2026-09-25):** dados conferidos contra o seed e o banco local.
- Os demais documentos e o Postman que ainda citam `henrique.rodrigues` ficaram no card DOC-03 (`afazer.md`).

---

## DOC-02: manual da base de dados com o índice `uq_clientes_cnpj` — 2026-09-25
**Agentes:** 🔵 SubBrain → conferência do 🔴 TestBrain → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-25.

**Camada:** Documentação
**Origem:** 🔴 TestBrain (2026-09-25), a partir do NEG-01.

**Descrição:** Atualizar o manual da base de dados com o índice `uq_clientes_cnpj` e a unificação dos CNPJs duplicados. A migração 19 (`make db-fix-cnpj-unique`) já estava aplicada no banco local, e o DB-01 já tinha sido fechado (log de vínculos vazio é o esperado).

**O que foi feito:**
- **🔵 SubBrain (2026-09-25):** `docs/manual-base-de-dados.md` atualizado.
- **🔴 TestBrain (2026-09-25):** dados conferidos contra o seed e o banco local.
- O manual completo (todas as tabelas) ficou no card DOC-04 (`afazer.md`).

---

## NEG-02: aceitar o CNPJ alfanumérico da Receita — 2026-09-25
**Agentes:** 🌸 DataBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → 🔵 SubBrain → roteiro do Lote 5 → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-25.

**Camada:** Database / Backend / Frontend
**Origem:** 🔴 TestBrain (2026-09-25). **Decisão do usuário (2026-09-25): implementar agora.**

**Descrição:** O CNPJ alfanumérico da Receita está vigente desde julho de 2026. A API aceitava só dígitos, e o schema usa `CHAR(14)` com dígito verificador módulo 11 numérico.

**Regra (Receita):**
- 14 posições. As 12 primeiras são `[0-9A-Z]` e os 2 DVs são numéricos.
- DV por módulo 11 com os pesos atuais; cada caractere vale `ASCII − 48` (`0`–`9` → 0–9, `A` → 17, …, `Z` → 42).
- A entrada é normalizada para maiúsculas e a máscara (`.`, `/`, `-`) é removida.
- CNPJs numéricos existentes continuam válidos.

**🌸 DataBrain (2026-09-25) — sem migração:**
- **Schema:** `cnpj CHAR(14)` utf8mb4_unicode_ci. Não há CHECK, trigger nem view, e os 3000 clientes atuais são numéricos.
- **Collation:** case-insensitive, então o back grava em maiúsculas. Nota adicionada no cabeçalho de `sql/09_ddl_clientes.sql`.
- **Comentário da coluna:** corrigido pela migração 20 (DB-02, 2026-09-25).
- **Importador:** o `apis/shared/cmd/importclientes/main.go` também precisava aceitar letras (repassado ao BackBrain).

**🟡 BackBrain (2026-09-25) — implementado:**
- **Pacote central:** `apis/shared/cnpj` com `Normalizar`, `FormatoValido`, `Valido` e `DigitosVerificadores`, com 97,4% de cobertura. O exemplo oficial `12ABC34501DE35` confere.
- **API:** `services/cnpj.go` e `cliente_service.go` delegam ao pacote. A busca `?q=` com máscara usa o novo `ClienteFiltro.QCNPJ`.
- **Importadores:** `clientesdedup` e `importclientes` usam a mesma regra. Mudança de comportamento: uma linha com caractere inválido passa a ser recusada, em vez de ter o caractere removido.
- **Contrato:**
  - Envio: com ou sem máscara, em maiúsculas ou minúsculas.
  - Retorno: sempre 14 caracteres, sem máscara e em maiúsculas.
  - Erros: sem mudança (400/409).
- **Testes:** `go build`, `go vet` e `go test ./...` ok nos dois módulos. A integração com MySQL não rodou nessa etapa; rodou depois, na regressão do lote.

**🟢 FrontBrain (2026-09-25) — implementado:**
- **Util central:** `frontend/src/lib/cnpj.ts`, com normalização, validação com DV espelhando o back, máscara progressiva no input (letras só nas 12 primeiras posições) e máscara de exibição.
- **Telas:** `ClienteModal` valida e envia o CNPJ normalizado. A tabela de clientes e o `VendedorModal` usam o `formatCnpj` compartilhado.
- **Testes:** `tsc` e `eslint` sem erros e `vitest` com 774/774 testes passando.
- **Impacto:** um cliente legado com DV inválido não salva na edição até o CNPJ ser corrigido, igual ao back.
- **Correção NEG-02-A/B (2026-09-25):** o `cnpj.ts` agora espelha o back: `TrimSpace` com os espaços Unicode do Go nas bordas, remove só `[./- ]` e converte para maiúsculas só `a-z`. A massa cruzada passa nos dois lados e não sobrou nenhum `it.fails`. Resultado: `vitest` com 1090/1090 testes e `go test ./cnpj/...` ok.

**🔵 SubBrain (2026-09-25):** Postman (README e collection) e `docs/manual-base-de-dados.md` atualizados com a regra alfanumérica.

---

## DB-02: comentários imprecisos em SQL e no Makefile — 2026-09-25
**Agentes:** 🔵 SubBrain / 🌸 DataBrain (origem) → decisão do usuário → 🌸 DataBrain → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-25.

**Camada:** Database
**Origem:** 🔵 SubBrain e 🌸 DataBrain, DOC-02 e NEG-02 (2026-09-25). **Decisão do usuário:** corrigir a coluna (item 4, migração 20) e os demais comentários.

**Descrição:**
1. `sql/19_alter_clientes_cnpj_unique.sql:42`: o comentário dizia que o backup guarda "cópias + sobrevivente", mas o SQL grava só as cópias.
2. Makefile: a descrição do `db-fix-cnpj-unique` dizia "não destrutivo", mas a migração apaga as cópias (com backup).
3. `sql/03_seed_vendedores.sql`:
   - o comentário descrevia e-mails no formato `v<ID>.<slug>`, mas os reais são `nome.sobrenome@`;
   - o comentário dizia que a senha é `Mudar@123`, mas o `make db-seed` usa `SEED_USER_PASSWORD` ou uma senha aleatória.
4. O comentário da coluna `clientes.cnpj` dizia "somente dígitos", desatualizado com o NEG-02.

**🌸 DataBrain (2026-09-25):**
- **Migração 20:** `sql/20_alter_clientes_cnpj_comment.sql` + revert, com targets `make db-fix-cnpj-comment` e `db-revert-cnpj-comment`. Aplicada no banco local. Só o COMMENT mudou; tipo, collation, `uq_clientes_cnpj` e os 3000 clientes estão intactos. O DDL `09` também foi atualizado.
- **Comentários corrigidos:** cabeçalho da `19`, descrição do `db-fix-cnpj-unique` no Makefile e comentários de e-mail e senha em `03_seed_vendedores.sql`.
- **Tabela de backup:** mantida com o texto antigo, que é verdadeiro para os dados numéricos que ela guarda.

---

## FE-08: mensagem amigável para erro de rede — 2026-09-25
**Agentes:** decisão do usuário → 🟢 FrontBrain → 🔴 TestBrain → roteiro do Lote 5 → aceite do usuário → fechamento por 🔵 SubBrain

**Status:** concluído — aceite do usuário em 2026-09-25.

**Camada:** Frontend
**Origem:** decisão do usuário (2026-09-25), a partir do FE-06.

**Descrição:** O projeto não tinha mensagem própria para erro de rede. A tela mostrava o texto cru do navegador (ex.: "Failed to fetch").

**🟢 FrontBrain (2026-09-25):**
- **Erro novo:** `NetworkError` (com `kind` "conexao" ou "timeout" e `cause` = erro original), aplicado no `makeRequest` do `apiClient.ts`. Vale para a requisição original, os reenvios e a fila.
- **Mensagens:**
  - "Não foi possível conectar ao servidor. Verifique sua conexão com a internet e tente novamente."
  - "O servidor demorou para responder. Tente novamente."
- **Sem mudança:** abort do chamador, regras de logout e erros HTTP (`ApiError`).
- **Testes:** `tsc` e `eslint` sem erros e `vitest` com 1101/1101 testes passando.
- **Em aberto:** as requisições comuns não têm timeout padrão, só o refresh (10s). Criar um timeout global exige decisão.

---

## NEG-04: troca errada de CNPJ legado não pode ser desfeita pela tela/API (DECISÃO DE NEGÓCIO) — 2026-09-25
**Agentes:** 🔴 TestBrain → decisão do usuário → 🟡 BackBrain → 🔴 TestBrain → revalidação do roteiro → aceite do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** aceite após o roteiro manual lote 4 e a revalidação do NEG-04.

**Camada:** Backend
**Início:** 2026-09-25
**Origem:** 🔴 TestBrain, execução do roteiro do Lote 4 (2026-09-25), divergência D3 (item 4.8).

**Causa:** o DV só era exigido quando o CNPJ mudava. Um CNPJ legado inválido era aceito se mantido, mas recusado se reenviado depois de uma troca. Assim, uma troca errada não podia ser desfeita pela tela/API.

**Decisão do usuário (2026-09-25):** "validar o CNPJ em toda gravação".
- **Impacto aceito:** cerca de 2.970 clientes importados com DV inválido só podem ser editados depois que o CNPJ for corrigido.
- Voltar a um CNPJ legado só é possível por SQL.

**O que foi feito:**
- **🟡 BackBrain** (`apis/rotaperfumes-api/services/cliente_service.go`, `UpdateCliente`):
  - Todo PUT exige CNPJ com DV válido, mesmo quando é igual ao atual. Se não tiver, responde `400` "cnpj inválido" sem nenhuma query.
  - Saiu o SELECT prévio. O `404` vem do UPDATE com 0 linhas (`clientFoundRows=true`), e id inexistente com DV inválido dá `400`.
  - O `/inativar` continua sem exigir DV.
- **🔴 TestBrain:**
  - Testes novos: `TestClienteService_UpdateCliente_DVInvalidoTemPrecedenciaSobre404`, `TestUpdateCliente_CNPJ` (legado com e sem máscara), `TestToggleAtivoCliente_CNPJLegadoDVInvalido` e `TestIntegracaoHTTP_NEG04_CNPJLegadoDVInvalido`.
  - `go test ./...` OK nos dois módulos (handlers 87,9%, services 94,7%) e integração OK.
- **🔵 SubBrain:**
  - Postman (README e descrições da collection) com a regra nova.
  - CNPJ dos exemplos trocado de `12345678000199` para `11222333000181` (JSON validado, 0 ocorrências restantes).
  - O README registra a migração 19 como aplicada.
  - Roteiro com 4.7, 4.7b, 4.8 e 4.13 reescritos ou novos.
- **Revalidação (🔴 TestBrain, 2026-09-25, API reiniciada às 18:22 com o código do NEG-04):**
  - **4.7:** pela tela, o PUT no cliente 9 mantendo o CNPJ legado deu `400` "cnpj inválido". A mensagem apareceu no modal, e o banco ficou inalterado (inclusive `updated_at`).
  - **4.7b:** o PUT mantendo o CNPJ no cliente de teste com CNPJ válido deu `200`.
  - **4.8:** a troca por um CNPJ válido e inédito deu `200` e gravou só os dígitos. A volta ao legado pela API deu `400`, e foi feita por SQL.
  - **4.13:** inativar e reativar o cliente 9 com o CNPJ legado deu `200` nos dois.
  - **Limpeza:** cliente de teste apagado, cliente 9 restaurado, 3000 clientes e 3637 carteiras.

---

## NEG-01: validação e duplicidade de CNPJ (DECISÃO DE NEGÓCIO) — 2026-09-25
**Agentes:** 🌸 DataBrain → 🟡 BackBrain → 🔴 TestBrain → roteiro manual do Lote 4 → decisões NEG-03/NEG-04 e verificação DB-01 → aceite do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** aceite após o roteiro manual lote 4 e a revalidação do NEG-04.

**Camada:** Backend / Database
**Início:** 2026-09-25
**Origem:** Lote 3 de 2026-09-24.

**Causa:** o índice `idx_clientes_cnpj` não era `UNIQUE`, e não havia validação de 14 dígitos no CNPJ.

**Decisões do usuário (2026-09-25):**
- Bloquear duplicado com índice UNIQUE, `400` "cnpj inválido" e `409` genérico sem revelar o vendedor.
- Os duplicados existentes foram unificados no cliente de menor id, com os registros vinculados transferidos.

**O que foi feito:**
- **🌸 DataBrain:** migração `sql/19_alter_clientes_cnpj_unique.sql` (e o `19_revert`), backup `clientes_merge_backup_20260925`, targets `make db-fix-cnpj-unique`/`db-revert-cnpj-unique` e `09_ddl` atualizado. Aplicada no banco local: 3000 clientes, 0 duplicados, só `uq_clientes_cnpj`.
- **🟡 BackBrain:**
  - `services/cnpj.go`: máscara, 14 dígitos e DV módulo 11.
  - `409` via erro 1062 (`repositories/mysql_errors.go`).
  - Importadores com deduplicação (`shared/cmd/internal/clientesdedup`).
- **🔴 TestBrain:** cobertura unitária e de integração (`cliente_cnpj_test.go`, `cnpj_internal_test.go`, `lote4_http_integration_test.go`, dedup dos importadores).
- **🔵 SubBrain:** Postman atualizado (`201` com máscara, `400`, `409`). O manual da base de dados continua no card DOC-02 (`afazer.md`).
- **Roteiro manual (2026-09-25):** seção 1 e seção 4 OK. As divergências viraram cards derivados, todos em `feito.md`:
  - **DB-01:** log de vínculos vazio é o esperado.
  - **NEG-03:** DV inválido recebe `400` antes da checagem de duplicidade; comportamento mantido.
  - **NEG-04:** toda gravação exige DV válido; implementado.
- **Revalidação do NEG-04 (2026-09-25):** 4.7 (`400`, banco inalterado), 4.7b (`200`), 4.8 (`200` na troca; volta ao legado só por SQL) e 4.13 (`/inativar` `200`) OK. Limpeza feita (3000 clientes, 3637 carteiras).
- **Observação:** o CNPJ alfanumérico da Receita continua no card NEG-02 (`afazer.md`).

---

## NEG-03: CNPJ legado duplicado com DV inválido responde "cnpj inválido" em vez de "já cadastrado" (DECISÃO DE NEGÓCIO) — 2026-09-25
**Agentes:** 🔴 TestBrain → decisão do usuário → fechamento por 🔵 SubBrain

**Decidido pelo usuário em 2026-09-25:** MANTER o comportamento atual. **Sem mudança de código.**

**Camada:** Backend
**Origem:** 🔴 TestBrain, execução do roteiro do Lote 4 (2026-09-25), divergência D2 (itens 4.5, 4.6, 4.10 e 4.11).

**Comportamento aceito:**
- Um CNPJ com DV inválido é recusado com `400` "cnpj inválido" e não é gravado. Isso vale também para o CNPJ de um cliente legado que já existe (ex.: `17810801326773`).
- A verificação de duplicidade (`409` "cnpj já cadastrado") só vale para CNPJ válido (ex.: `23124329212779` → `409`).
- O índice `uq_clientes_cnpj` continua protegendo contra duplicados.
- Contexto: só 28 dos 3000 CNPJs da base têm DV válido (os importados são fictícios).

**O que foi feito:**
- 🔵 SubBrain: a nota do 4.5 e o D2 da seção "Execução 2026-09-25 (TestBrain)" em `docs/roteiro-teste-manual-lote4.md` registram o "comportamento aceito (NEG-03, decidido em 2026-09-25)".

---

## DB-01: log de vínculos da migração 19 (`clientes_merge_backup_20260925_vinculos`) está vazio — 2026-09-25
**Agentes:** 🔴 TestBrain → 🌸 DataBrain → decisão do usuário → fechamento por 🔵 SubBrain

**Veredito do 🌸 DataBrain em 2026-09-25:** log vazio é o esperado, sem defeito. **Sem mudança de SQL.** Fechamento decidido pelo usuário em 2026-09-25.

**Camada:** Database
**Origem:** 🔴 TestBrain, execução do roteiro do Lote 4 (2026-09-25), item 1.6.

**Situação:** com a migração 19 aplicada e reaplicada, as contagens estavam corretas, mas `clientes_merge_backup_20260925_vinculos` tinha 0 linhas.

**Evidências (🌸 DataBrain):**
- `sql/19_alter_clientes_cnpj_unique.sql` grava o log (linhas 154-186) **antes** do DELETE (189-191) e dos UPDATEs (194-208), na mesma transação e com o mesmo filtro.
- As 40 cópias (ids 3001 a 3040) nunca tiveram filhos: os CSVs de `dados/crm` e `dados/erp` não têm `cliente_id` acima de 3000, e no banco as quatro tabelas (pedidos, carteiras, oportunidades e visitas) dão 0 para `cliente_id > 3000`.
- A reversão `19_revert` funciona com o log vazio: ela reinsere as 40 cópias, e os passos 3 e 4 não mexem em nada.

**O que foi feito:**
- 🔵 SubBrain: item 1.6 de `docs/roteiro-teste-manual-lote4.md` reescrito (o log fica vazio com os dados atuais, com a consulta de conferência `cliente_id > 3000` = 0). O aviso de verificação pendente foi removido, e a seção de execução registra "log vazio é o esperado (DB-01, fechado)".
- O card DOC-02 (manual da base de dados) pode descrever o log com base neste veredito.

---

## FE-04: resposta obsoleta sobrescreve a mais nova nas listagens — 2026-09-25
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → roteiro manual do Lote 4 → validação do 6.3 em produção → aceite do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** aceite após o roteiro manual lote 4 e a confirmação do 6.3 em produção.

**Camada:** Frontend
**Início:** 2026-09-25
**Origem:** Lote 3 de 2026-09-24 (documentado pelo 🔴 TestBrain durante o FE-03). O problema já existia antes do FE-03.

**Causa:** as listagens não descartavam respostas obsoletas. Havia três efeitos:
1. Na paginação rápida, a resposta de uma página antiga sobrescrevia a da página atual.
2. O debounce dos filtros disparava na montagem: eram duas buscas ao abrir, e a tabela podia voltar à página 1 enquanto o paginador mostrava a página 2.
3. A linha excluída reaparecia em pedidos, pagamentos, oportunidades e visitas.

**O que foi feito:**
- **🟢 FrontBrain:** hook `lib/useListaSegura.ts` (`useUltimaResposta`, `useDebounceFiltros`, `useExcluidos`) aplicado nas 9 listagens.
- **🔴 TestBrain:** os 18 `it.fails` viraram `it` (`useListaSegura.test.ts`, `listasPaginadas.test.tsx`, `listasAdmin.test.tsx`, `crudPaginas.test.tsx`, `crudAdmin.test.tsx`; vitest 701/701).
- **Roteiro manual (2026-09-25):**
  - 6.1, 6.2, 6.4, 6.5 e 6.6 OK em todas as telas aplicáveis (respostas forçadas fora de ordem). 6.1 e 6.2 não se aplicam a `/admin/usuarios` (3 páginas) nem a `/admin/senha-historico` (1 página).
  - D6: as pré-condições do 6.6 (pagamento "Em aberto" e pedido sem pagamento) foram incluídas no roteiro.
- **6.3 em produção (2026-09-25, 🔴 TestBrain):**
  - Ambiente: `next build --webpack` numa cópia do frontend, servida por `next start -p 3001`.
  - As 9 listagens fizeram **1 único GET** ao abrir, sem GET extra do debounce em 1,5 s.
  - Os 2 GETs vistos em `next dev` vêm só do `reactStrictMode`. O D4 ficou resolvido como comportamento esperado do modo dev.

---

## FE-05: listagens chamam `fetch` direto, sem refresh automático no 401 — 2026-09-25
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → roteiro manual do Lote 4 → aceite do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** aceite após o roteiro manual lote 4.

**Camada:** Frontend
**Início:** 2026-09-25
**Origem:** Lote 3 de 2026-09-24.

**Causa:** as funções `apiList*`, `apiDashboardVendedores` e `apiListSenhaHistorico` (`frontend/src/lib/api.ts`) chamavam `fetch` direto, e um `401` não disparava o refresh automático do `fetchWithAuth`. Em `admin/clientes`, enquanto o `/me` carregava, o motivo do botão desabilitado dizia "Usuario sem vendedor vinculado".

**O que foi feito:**
- **🟢 FrontBrain:** `fetchEnvelopeWithAuth`/`keepEnvelope` em `apiClient.ts`; as 10 funções de listagem foram migradas. Em `admin/clientes`, o texto durante o carregamento passou a ser "Carregando dados do usuario...".
- **🔴 TestBrain:** `frontend/src/lib/apiListagensRefresh.test.ts` (refresh, fila única, logout, 403 de desligado, envelope preservado) e `app/admin/clientes/page.test.tsx` (vitest 701/701).
- **Roteiro manual (2026-09-25):**
  - 7.1 a 7.3 OK nas 10 telas (estoque e produtos como admin; em senha-historico, o gatilho foi o select "Itens por pagina").
  - 7.4: refresh `400` sem cookie, logout e `/login` sem repetir o `GET`. É o comportamento aceito no **SEC-05** (decidido em 2026-09-25, em `feito.md`).
  - 7.5 (D5): o texto de carregamento não chega a aparecer, porque o ProtectedRoute mostra "Verificando autenticacao..." até o `/me` responder. O objetivo (nunca mostrar o motivo errado) é atendido, e o roteiro foi ajustado.

---

## SEC-02: corrida no refresh token gera dois pares de tokens — 2026-09-25
**Agentes:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → roteiro manual do Lote 4 → aceite do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** aceite após o roteiro manual lote 4.

**Camada:** Backend (+ Frontend)
**Início:** 2026-09-25
**Origem:** Lote 3 de 2026-09-24.

**Causa:** `auth_handler.go` validava e revogava o refresh token em passos separados e ignorava o erro da revogação. Duas chamadas de refresh simultâneas com o mesmo token geravam dois pares de tokens válidos.

**O que foi feito:**
- **🟡 BackBrain:**
  - Revoke condicional (`WHERE id = ? AND revoked_at IS NULL`) e rotação transacional (`BeginRotation`/`Issue`).
  - Na corrida, a requisição que perde recebe `401` "refresh token revogado" (conta no rate limit).
  - Erro de banco na revogação ou na gravação do novo refresh token → `500` "erro interno", sem cookie.
- **🟢 FrontBrain:** `apiClient.ts` repete a requisição uma vez após um refresh com `401` e usa `navigator.locks` com o lock `"rp-auth-refresh"` (multi-aba).
- **🔴 TestBrain:** `TestRefresh_CorridaParalela`, `auth_refresh_logout_test.go`, `refresh_rotation_test.go`, `lote4_repository_test.go`, integração do Lote 4 (8 rodadas com um 200 e um 401) e `apiClient.test.ts`.
- **🔵 SubBrain:** Postman atualizado (exemplos `401` e `500` em "Refresh Token" e README).
- **Roteiro manual (2026-09-25):**
  - 2.1, 2.2, 2.4, 2.5, 2.6 e 2.7 OK (corrida real em duas abas, Web Lock funcionando). 2.3 não verificado (log da API no debugger).
  - 2.8 (D1): sem cookie, o refresh responde `400`, e o front faz logout e vai para `/login`. É o comportamento aceito no **SEC-05** (decidido em 2026-09-25, sem mudança de código, em `feito.md`).
- **Follow-ups em `afazer.md`:** SEC-04 (corrida legítima conta no rate limit por IP) e FE-06 (mensagem enganosa quando o retry falha por rede).

---

## SEC-05: refresh sem cookie responde `400` em vez de `401` (DECISÃO) — 2026-09-25
**Agentes:** 🔴 TestBrain → decisão do usuário → fechamento por 🔵 SubBrain

**Decidido pelo usuário em 2026-09-25:** manter o comportamento atual. **Sem mudança de código.**

**Camada:** Backend / Frontend
**Origem:** 🔴 TestBrain, execução do roteiro do Lote 4 (2026-09-25), divergência D1 (itens 2.8 e 7.4).

**Comportamento aceito:** sem nenhum cookie, `POST /api/auth/refresh` responde `400` "refresh_token é obrigatório (body ou cookie)". O `apiClient` então chama `/api/auth/logout` e redireciona para `/login` sem repetir o `GET`. O resultado final é seguro.

**O que foi feito:**
- 🔵 SubBrain: itens 2.8 e 7.4 e a seção "Execução 2026-09-25 (TestBrain)" de `docs/roteiro-teste-manual-lote4.md` registram o comportamento como aceito.
- Desbloqueou SEC-02 e FE-05 em `fazendo.md`.

---

## BUG-06: `PATCH /inativar` com body inválido inverte o estado — 2026-09-25
**Agentes:** 🟡 BackBrain → 🔴 TestBrain → roteiro manual do Lote 4 → aceite do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** aceite do usuário após o roteiro manual lote 4.

**Camada:** Backend
**Início:** 2026-09-25
**Origem:** Lote 3 de 2026-09-24.

**Causa:** três handlers (`cliente_handler.go`, `produto_handler.go` e `usuario_handler.go`) ignoravam o erro de decode do body. Um body inválido, como `{"ativo":"false"}`, era tratado como omitido e alternava o estado.

**O que foi feito:**
- **🟡 BackBrain:** novo `handlers/ativo_body.go`. Body preenchido e inválido → `400` "body JSON inválido". Vazio, `null`, `{}` ou `{"ativo":null}` → toggle; `{"ativo":true|false}` → define o valor. Em clientes, o decode vem depois do escopo: sem acesso ao registro, `404` antes do `400` (SEC-01).
- **🔴 TestBrain:** `handlers/inativar_body_test.go` e a integração em `lote4_http_integration_test.go` (go test: shared 823 PASS, api 1777 PASS; smoke HTTP 38/38 OK).
- **🔵 SubBrain:** Postman atualizado (exemplo `400` nos três PATCH `/inativar` e `404` antes do `400` em clientes; README).
- **Roteiro manual (2026-09-25):** itens 5.1 a 5.10 OK nos 3 endpoints, sem divergência.

---

## SEC-03: `GET /api/vendedores` sem escopo — 2026-09-25
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → roteiro manual do Lote 4 → aceite do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** aceite do usuário após o roteiro manual lote 4.

**Camada:** Backend
**Início:** 2026-09-25
**Origem:** Lote 3 de 2026-09-24.

**Causa:** em `apis/rotaperfumes-api/handlers/vendedor_handler.go`, o usuário `normal` recebia todos os vendedores, e o bloqueio de vendedor desligado não era aplicado.

**O que foi feito:**
- **🟡 BackBrain:** escopo aplicado em `vendedor_handler.go`, com `ListResumoByID`. Admin vê todos; `normal` com vínculo vê só o próprio vendedor; sem vínculo recebe `data: []`; vendedor desligado recebe `403`. `/api/produtos` não é bloqueado (decisão do 🟣 SecBrain: é catálogo).
- **🟢 FrontBrain:** hook `useVendedorTravado.ts`; `OportunidadeModal` e `VisitaModal` travam o vendedor. O `PedidoModal` já estava correto.
- **Limitação:** no `PedidoModal`, um `403` derruba o carregamento de produtos, mas o `CarteiraGuard` já bloqueia a tela.
- **🔴 TestBrain:** `vendedor_handler_test.go`, `services/vendedor_proprio_test.go`, `VendedorTravado.test.tsx` e a integração do Lote 4.
- **🔵 SubBrain:** Postman atualizado (nova pasta "Vendedores" com os 4 exemplos e README).
- **Roteiro manual (2026-09-25):** itens 3.1 a 3.8 OK, sem divergência.

---

## BUG-02: botão "Alterar Senha" nunca habilita em `/trocar-senha` (inclui BUG-03) — 2026-09-25
**Agentes:** 🟢 FrontBrain → 🟡 BackBrain → validação do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** OK.

**Camada:** Frontend (+ Backend no BUG-03)
**Início:** 2026-09-24

**Causa:** ao chegar em `/trocar-senha` por navegação client-side, com o script do Turnstile já carregado, o `next/script` deduplicava pelo `src` e não disparava `onLoad`. O widget não renderizava, nenhum token chegava e o botão ficava travado.

**O que foi feito:**
- **🟢 FrontBrain:** `frontend/src/components/ui/Turnstile.tsx` troca `onLoad` por `onReady` e inicia `scriptLoaded` como true quando `window.turnstile` já existe. O captcha continua obrigatório.
- **BUG-03 (achado na validação):** com a senha atual errada, a tela mostrava "Nao foi possivel validar o captcha", porque o `401` fazia o `fetchWithAuth` dar refresh e reenviar o mesmo `captchaToken` (uso único). 🟡 BackBrain mudou a resposta para `400` em `auth_handler.go` (`ResetPassword`), ajustou os testes e criou `TestResetPassword_SenhaAtualIncorreta_Retorna400` (`go vet`/`go test` OK). Postman atualizado.

---

## UI-04: accordion de itens do pedido abre no fim da tabela — 2026-09-25
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → validação do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** OK.

**Camada:** Frontend
**Origem:** relato do usuário durante a validação do `PedidoModal` (2026-09-25).

**O que foi feito:**
- Novo prop opcional `renderExpanded` em `frontend/src/components/ui/Table.tsx` (Fragment por linha e `<tr>` extra com `colSpan`): o detalhe abre logo abaixo da linha clicada.
- Novo componente `frontend/src/components/admin/PedidoItensDetalhe.tsx`.
- `frontend/src/app/admin/pedidos/page.tsx`: removido o bloco do fim da tabela; helper `fecharItens()` também limpa o erro antigo.
- Novo teste em `frontend/src/app/crudPaginas.test.tsx`. tsc OK; vitest 19/19 arquivos (635 passaram, 18 falhas esperadas do FE-04, 2 pulados); eslint OK.
- Não existe layout mobile separado: em telas estreitas, o detalhe fica dentro do scroll horizontal da tabela.

---

## Teste manual do Dashboard no navegador (normal com vendedor, normal sem vendedor, admin) — 2026-09-25
**Agentes:** 🔴 TestBrain → validação do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** OK.

**Camada:** Frontend (teste)
**Origem:** 🟢 FrontBrain, durante o card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23).

**Escopo validado (`frontend/src/app/dashboard/page.tsx`):**
- Rótulos "Minhas Vendas", "Meus Pedidos", "Minha Meta", "Meus Clientes" e "Clientes da minha carteira" para o usuário `normal`.
- Card "Meu Desempenho" no lugar do ranking; ranking completo para `admin`.
- Avisos de usuário sem vendedor e de vendedor desligado (`vendedor_desligado` da API).
- Tolerância a `null`/`[]` e valores monetários com 2 casas (`fmtCurrency`).
- `/api/dashboard/clientes` com escopo da carteira.

**Resultado:**
- **🔴 TestBrain (2026-09-24):** validou via API os perfis admin, normal com vendedor, normal sem vendedor e normal com vendedor desligado; roteiro e evidências em `docs/roteiro-teste-manual-dashboard.md`. As divergências viraram UI-01 e UI-02 (abaixo).
- **Usuário (2026-09-25):** executou o roteiro no navegador.

---

## UI-01: título do gráfico do Dashboard (admin) fixo em "Vendas nos Ultimos 30 Dias" — 2026-09-25
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → validação do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** OK.

**Camada:** Frontend (menor)
**Origem:** validação do card "Teste manual do Dashboard" (2026-09-24).

**O que foi feito:** 🟢 FrontBrain montou o título a partir dos dias selecionados em `frontend/src/app/dashboard/page.tsx` (ex.: "Vendas nos Últimos 7 Dias"). 🔴 TestBrain cobriu em `frontend/src/app/dashboard/page.test.tsx` (7/14/30/60 dias) e atualizou o item 4.8 de `docs/roteiro-teste-manual-dashboard.md`.

---

## UI-02: layout do Dashboard para normal sem vendedor / com vendedor desligado — 2026-09-25
**Agentes:** 🔵 SubBrain / 🤍 MegaBrain (decisão) → 🟢 FrontBrain → 🔴 TestBrain → validação do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** OK.

**Camada:** Frontend
**Origem:** validação do card "Teste manual do Dashboard" (2026-09-24): a tela mostrava só o aviso, sem KPIs nem gráfico zerados.

**DECISÃO DO USUÁRIO (2026-09-24):** opção **(b)** — aviso + KPIs zerados + gráfico com os dias zerados.

**O que foi feito:** 🟢 FrontBrain implementou a opção (b) em `frontend/src/app/dashboard/page.tsx`. 🔴 TestBrain cobriu em `frontend/src/app/dashboard/page.test.tsx` e atualizou `docs/roteiro-teste-manual-dashboard.md` (itens 2.2-2.6 e 3.3-3.8). O Dashboard depende da API para zerar os dados (follow-up FE-02, já concluído).

---

## Segurança: vendedor desligado ainda podia criar/editar pedidos, pagamentos, oportunidades e visitas (DECIDIDO: bloquear) — 2026-09-25
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → 🔵 SubBrain (Postman) → validação do usuário

**Validado pelo usuário em 2026-09-25:** OK.

**Camada:** Backend (com impacto em Frontend, Testes e Postman)
**Origem:** 🟣 SecBrain, lote de 2026-09-24. `resolverVendedorScope` ignorava `vendedores.data_desligamento`.

**DECISÃO DO USUÁRIO (2026-09-24):** bloquear o usuário `normal` vinculado a vendedor desligado em todas as rotas da carteira (leitura e escrita). Login e Dashboard continuavam acessíveis — premissa depois alterada pelo BUG-05 (o usuário vinculado passa a ser inativado e perde o login).

**O que foi feito:**
- **🟣 SecBrain:** contrato `403` `{"success":false,"error":"acesso bloqueado: vendedor desligado"}` em todas as rotas da carteira, fail-closed (`500` em falha de banco).
- **🟡 BackBrain (`apis/rotaperfumes-api/handlers/scope.go`):** `resolverVendedorScope` checa `data_desligamento` a cada requisição; `resolverVendedorScopeBase` restrito ao Dashboard (`200` zerado com `vendedor_desligado: true`); `errVendedorDesligado` + `responderErroEscopo`. Aplicado em clientes, pedidos, pagamentos, oportunidades, visitas e `GET /api/vendedores/{id}/clientes`. `GET /api/auth/me` devolve `vendedor_desligado`.
- **🟢 FrontBrain:** `ApiError`, `frontend/src/lib/vendedorDesligado.ts`, `frontend/src/lib/session.ts`, `frontend/src/components/layout/CarteiraGuard.tsx`; Navbar oculta os itens da carteira.
- **🔴 TestBrain:** `escopo_desligado_cobertura_test.go` e `scope_internal_test.go` (cobertura de `handlers` 87,3%); testes Vitest de sessão/layout; roteiro `docs/roteiro-teste-manual-vendedor-desligado.md`.
- **🔵 SubBrain:** `postman/collection.json` e `postman/README.md` com notas e exemplos `403` nas pastas da carteira e `vendedor_desligado` no `/me`.
- **Follow-ups:** FE-01 e SEC-01 (ambos concluídos no Lote 3).

---

## UX: `id_vendedor` do localStorage desatualizado após o admin mudar o vínculo do usuário — 2026-09-25
**Agentes:** 🟣 SecBrain (origem) → 🟢 FrontBrain → 🔴 TestBrain → validação do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** OK.

**Camada:** Frontend
**Origem:** 🟣 SecBrain (2026-09-24). `PedidoModal`, Oportunidades e Visitas travavam o vendedor antigo até novo login (sem vazamento de dados: o backend já aplicava o vendedor correto).

**O que foi feito:** 🟢 FrontBrain passou a revalidar o usuário via `GET /api/auth/me` (`frontend/src/lib/session.ts`) em vez de confiar só no `localStorage`. 🔴 TestBrain cobriu em `session.test.ts`, `ProtectedRoute.test.tsx` e `PedidoModal.test.tsx` e escreveu os roteiros (`docs/roteiro-teste-manual-vendedor-desligado.md`, seção 6; `docs/roteiro-teste-manual-pedidomodal.md`, seção 5b).

---

## BUG-05: inativar vendedor não inativava o usuário vinculado — 2026-09-25
**Agentes:** 🤍 MegaBrain (decisão) → 🟡 BackBrain → validação do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário em 2026-09-25:** OK.

**Camada:** Backend
**Origem:** relato do usuário, 2026-09-24. `DeleteVendedor` só gravava `vendedores.data_desligamento`; o usuário vinculado continuava com `ativo = 1` e fazia login.

**Decisão do 🤍 MegaBrain (2026-09-24):** inativar os usuários vinculados na mesma operação atômica. Reativar o vendedor **não** reativa os usuários (reativação manual na tela de usuários). Isso altera a premissa do card "Segurança: vendedor desligado" (o login do usuário vinculado deixa de ficar acessível).

**O que foi feito:** 🟡 BackBrain implementou `inativarVendedorEUsuarios` em `apis/rotaperfumes-api/services/vendedor_service.go`: desligamento do vendedor e `ativo = 0` dos usuários vinculados numa única transação.

---

## Teste manual/e2e do `PedidoModal` no navegador (usuário normal e admin) — 2026-09-25
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → validação do usuário → fechamento por 🔵 SubBrain

**Validado pelo usuário no navegador em 2026-09-25:** OK. Durante a validação, o usuário relatou o **UI-04** (accordion de itens do pedido abre no fim da tabela), aberto em `fazendo.md`.

**Camada:** Frontend (teste)
**Origem:** 🟢 FrontBrain, durante o card "Scope check em Create/Update de Pedidos" (2026-09-23). As mudanças em `frontend/src/components/admin/PedidoModal.tsx` tinham sido validadas só com typecheck.

**Escopo validado:**
- Vendedor travado para usuário `normal`.
- Aviso e botão Salvar bloqueado quando não há vendedor vinculado.
- Cascata vendedor → cliente via `apiListClientesDoVendedor`.
- Cliente "(fora da carteira)" mantido na edição.
- Mensagem amigável para o `400` de carteira.

**Resultado:**
- **🔴 TestBrain (2026-09-24):** validou via API os perfis admin, normal com vendedor, normal sem vendedor e normal com vendedor desligado. Evidências e roteiro manual em `docs/roteiro-teste-manual-pedidomodal.md`.
- **Usuário (2026-09-25):** executou o roteiro no navegador (criar/editar como `admin`, como `normal` com vendedor, `normal` sem vendedor e edição com cliente fora da carteira).

---

## UI-03: coluna própria para o ID principal nas tabelas — 2026-09-25
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → fechamento por 🔵 SubBrain

**Validado pelo usuário no navegador em 2026-09-25:** OK.

**Camada:** Frontend
**Origem:** pedido do usuário, 2026-09-25: "em todas as telas, crie uma coluna com o ID principal ao invés de colocar junto ao nome." Várias tabelas de `frontend/src/app/admin/` mostravam o ID principal na mesma célula do nome (ex.: `#id - razao_social`).

**Padrão de referência:** a coluna `ID` que já existia em `frontend/src/app/admin/usuarios/page.tsx` (key `"id"`, header `"ID"`, largura `80px`, ordenável, estilo `font-mono text-xs`, valor `#{id}`).

**Fora de escopo:** chaves estrangeiras exibidas em outras colunas (cliente, vendedor, `usuario_id` etc.), labels de select, toasts e mensagens de confirmação.

**Resultado (2026-09-25):**
- Coluna "ID" (primeira, ordenável, `#id` em `font-mono`) em clientes, estoque, oportunidades, pedidos, produtos, senha-historico, usuarios, vendedores, visitas e pagamentos. O prefixo `#id -` foi removido das colunas de nome.
- Oportunidades e visitas não exibiam o ID antes. Senha-historico ganhou ordenação por `id` (`order_by=id`).
- Pedidos: o header da coluna do nome passou de "Pedido" para "Cliente" (a coluna continua abrindo os itens).
- Pagamentos: a antiga coluna "Pagamento" virou a coluna "ID". O botão "Editar pagamento" foi para a coluna "Forma".
- **🟢 FrontBrain:** padrão aplicado nas 10 telas (typecheck OK).
- **🔴 TestBrain:** testes ajustados em `crudPaginas.test.tsx`, `listasPaginadas.test.tsx`, `admin/crudAdmin.test.tsx` e `admin/listasAdmin.test.tsx`, com novos blocos "UI-03 coluna ID". Suíte com **634 testes passando, 0 falhas**.
- **Pendência fora do escopo:** `frontend/src/components/admin/VendedorModal.tsx:57` ainda mostra `#id - razao_social` na lista de clientes vinculados dentro do modal do vendedor.

---

## Lote 3 de 2026-09-24: 6 cards concluídos (de 6)

> Lote executado pelo 🤍 MegaBrain, a pedido do usuário, com os 6 follow-ups do Lote 2. BUG-04, RISCO-01, FE-01 e FE-02 foram concluídos em 2026-09-24. SEC-01 e FE-03 foram implementados e cobertos por testes em 2026-09-24 e validados pelo usuário no navegador em 2026-09-25. Os follow-ups deste lote estão em `afazer.md`: SEC-02, SEC-03, NEG-01, BUG-06, FE-04 e FE-05. O card BUG-05 (em `fazendo.md`) é tratado em outra sessão.
>
> **Documentação (🔵 SubBrain) do lote:** `postman/collection.json` e `postman/README.md`:
> - Contrato do SEC-01 em `POST`/`PUT`/`PATCH` de clientes.
> - `PUT`/`PATCH` sem alteração passa a responder `200` (BUG-04).
> - `DELETE` de vendedor já desligado responde `200` e preserva a `data_desligamento`.
> - Nota do fuso fixo `-03:00` (RISCO-01).
> - Os rótulos "admin only" de Clientes e Pedidos foram corrigidos para "acesso comum, escopo por carteira", e as notas de "revisão pendente" foram removidas.
> - A validação do JSON da collection fica com o 🤍 MegaBrain (o SubBrain não tem shell).

## BUG-04: salvar registro sem alterações retorna 404 "não encontrado" — 2026-09-24
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** 🔴 TestBrain (2026-09-24). O DSN não tinha `clientFoundRows=true`, e o MySQL devolvia `RowsAffected=0` em `UPDATE` sem mudança. Os repositórios tratavam isso como `ErrNotFound`, então salvar sem alterações respondia `404`.

**O que foi feito:**
- **🟣 SecBrain:** avaliou o impacto de `clientFoundRows=true` nas checagens que dependem de `RowsAffected` (revogação de token e lockout).
- **🟡 BackBrain:**
  - `clientFoundRows=true` no DSN (`apis/shared/config/config.go` e `apis/shared/cmd/resetpassword/main.go`).
  - `VendedorRepository.MarcarDesligamento` com `COALESCE`: o DELETE de um vendedor já desligado responde `200` e preserva a `data_desligamento` original.
  - Nos importadores, o rótulo do contador passou a ser `inseridos_ou_inalterados`.
- **🔴 TestBrain:** integração HTTP em `apis/rotaperfumes-api/handlers/sec01_bug04_risco01_http_integration_test.go`:
  - `PUT` sem alteração em 9 recursos → `200`.
  - `PATCH` com o mesmo valor → `200`.
  - 18 casos de id inexistente → `404`.
  - `DELETE` de vendedor duas vezes.
  - **Teste de mutação:** retirar a flag do DSN derruba a suíte.

**Documentação (🔵 SubBrain):** nota transversal na descrição da collection e no `postman/README.md`, e notas nas pastas Clientes e Pedidos.

## RISCO-01: datas de início de horário de verão com `TZ=America/Sao_Paulo` (Linux/Docker) — 2026-09-24
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** lote 2 de 2026-09-24, durante o BUG-01. Em datas de início de horário de verão, a meia-noite não existe (ex.: `2018-11-04`), e `time.ParseInLocation` devolvia 23:00 do dia anterior.

**O que foi feito:**
- **🟡 BackBrain:** novo pacote `apis/shared/tz`, que fixa `time.Local` em um fuso fixo `-03:00`, sem horário de verão. Ele é ativado por import em branco em `apis/shared/config/config.go` e no `resetpassword`.
- **🔴 TestBrain:**
  - `apis/shared/tz/tz_test.go`.
  - `apis/shared/config/config_test.go`: cobertura do pacote `config` de 0% para **100%**.
  - `2018-11-04` com round-trip em 3 colunas.
  - Refresh token expirado → `401`; válido → `200`.
  - `TestBUG01_HorarioDeVeraoHistorico` reescrito, sem skip.

**Documentação (🔵 SubBrain):** o formato das datas na resposta não mudou. A nota do fuso fixo está na collection (descrição geral e pasta Pedidos) e no `postman/README.md`.

## FE-01: corrida em `refreshSessionUser` mantinha o bloqueio de vendedor desligado após a reativação — 2026-09-24
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → fechamento por 🔵 SubBrain

**Origem:** card "Segurança: vendedor desligado" (lote 2). A revalidação de focus/mount reaproveitava a promise em voo originada pelo `403` e não limpava `bloqueado403`.

**O que foi feito:**
- **🟢 FrontBrain:** `frontend/src/lib/session.ts` ganhou um contador de sequência. O bloqueio só é liberado por um `/me` iniciado depois do último `403`.
- **🔴 TestBrain:** 10 testes novos, mais testes de mutação.

## FE-02: Dashboard dependia da API para zerar os dados de vendedor desligado / sem vendedor — 2026-09-24
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → fechamento por 🔵 SubBrain

**Origem:** card UI-02 (lote 2). `frontend/src/app/dashboard/page.tsx` não forçava zeros; a tela só ficava correta porque o backend já zerava os dados.

**O que foi feito:**
- **🟢 FrontBrain:**
  - O Dashboard força os zeros para usuário sem vendedor ou com vendedor desligado.
  - Os estados de loading e de erro passaram a ser derivados no render.
  - Respostas obsoletas são descartadas.
- **🔴 TestBrain:** testes com payload cheio: mesmo que a API devolva números, a tela mostra zeros nesses dois casos.

## SEC-01: IDOR em Create/Update/Toggle de clientes — 2026-09-24
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Validado pelo usuário no navegador em 2026-09-25** (roteiro `docs/roteiro-teste-manual-clientes.md`): OK.

**Origem:** 🟣 SecBrain, durante a definição do contrato de bloqueio do vendedor desligado (lote 2). Em `apis/rotaperfumes-api/handlers/cliente_handler.go`, `CreateCliente`, `UpdateCliente` e `ToggleAtivoCliente` não chamavam `resolverVendedorScope`. Com isso, qualquer usuário `normal` conseguia editar ou inativar qualquer cliente pelo id.

**O que foi feito:**
- **🟣 SecBrain:** contrato para o usuário `normal`:
  - `PUT /api/clientes/{id}` e `PATCH /api/clientes/{id}/inativar`: fora da carteira ativa, ou sem vendedor → `404` "cliente não encontrado"; falha ao checar a carteira → `500`; vendedor desligado → `403`, como antes.
  - `POST /api/clientes`: sem vendedor → `403` "usuário sem vendedor vinculado"; com vendedor → `201`, com o cliente vinculado automaticamente à carteira do vendedor.
  - Admin: sem mudança.
- **🟡 BackBrain:** `apis/rotaperfumes-api/handlers/cliente_handler.go`:
  - Novo helper `autorizarEscritaCliente`, checado antes de ler o body.
  - `ClienteService.CreateClienteNaCarteira`: cria o cliente e o vínculo de carteira numa única transação.
- **🟢 FrontBrain:** na tela de clientes, o botão fica desabilitado quando não há permissão, os erros 403 e 404 são tratados e a lista é recarregada.
- **🔴 TestBrain:**
  - `apis/rotaperfumes-api/handlers/cliente_sec01_cobertura_test.go`
  - integração HTTP em `handlers/sec01_bug04_risco01_http_integration_test.go`
  - `apis/rotaperfumes-api/services/cliente_na_carteira_test.go`
  - `frontend/src/app/admin/clientes/page.test.tsx`
  - roteiro `docs/roteiro-teste-manual-clientes.md`

**Documentação (🔵 SubBrain):** Postman atualizado. Na pasta Clientes:
- Todos os itens foram renomeados para "acesso comum, escopo por carteira".
- Os exemplos `403` "não é admin", que não valiam mais, foram trocados pelos 404/403 do contrato.
- Os testes que verificavam `id` agora verificam `cliente_id_origem`.
- O `postman/README.md` tem a tabela do contrato.

## FE-03: 34 warnings `react-hooks/set-state-in-effect` — 2026-09-24
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → fechamento por 🔵 SubBrain

**Validado pelo usuário no navegador em 2026-09-25** (roteiro `docs/roteiro-smoke-pos-fe03.md`): OK.

**Origem:** card "Tooling frontend" (lote 2). O ESLint apontava 34 warnings `react-hooks/set-state-in-effect`, com a regra rebaixada para `warn`.

**O que foi feito:**
- **🟢 FrontBrain:**
  - `npm run lint` agora dá **0 erros e 0 warnings**, e a regra `react-hooks/set-state-in-effect` voltou para `error`.
  - Novo hook `frontend/src/lib/useResetOnOpen.ts`.
  - 9 páginas e 9 modais refatorados para tirar o `setState` de dentro de `useEffect`.
- **🔴 TestBrain:**
  - Suíte com **604 testes passando**, mais 18 `it.fails` que documentam o bug **FE-04** (em `afazer.md`).
  - Cobertura de **87% das linhas**.
  - Roteiro de smoke `docs/roteiro-smoke-pos-fe03.md`.

---

## Lote 2 de 2026-09-24: 2 cards concluídos (de 6)

> Lote executado pelo 🤍 MegaBrain com os 6 cards que estavam em `afazer.md`. BUG-01 e Tooling do frontend foram concluídos. UI-01, UI-02, Segurança do vendedor desligado e UX do `id_vendedor` foram implementados e cobertos por testes, mas continuam em `fazendo.md` até o usuário validar no navegador. Os follow-ups deste lote estão em `afazer.md`: SEC-01, BUG-04, RISCO-01, FE-01, FE-02 e FE-03.
>
> **Documentação (🔵 SubBrain) do lote:** `postman/collection.json` e `postman/README.md` foram atualizados com o contrato `403` "acesso bloqueado: vendedor desligado" nas rotas da carteira, o campo `vendedor_desligado` em `GET /api/auth/me` e a nota de datas do BUG-01.

## BUG-01: `data_pedido` gravada com um dia a menos (fuso UTC vs `loc=Local`) — 2026-09-24
**Agentes:** 🟡 BackBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** lote de 2026-09-24, durante a validação do card "Teste manual/e2e do `PedidoModal`". Um pedido enviado com `data_pedido: "2026-09-24"` era gravado como `2026-09-23`: `time.Parse` gerava meia-noite em UTC, e o DSN usa `loc=Local` (UTC-3). Cada re-salvamento podia recuar mais um dia.

**O que foi feito:**
- **🟡 BackBrain:** `time.Parse` → `time.ParseInLocation(..., time.Local)` em 11 pontos dos services (`apis/rotaperfumes-api/services/`) e nos importadores `apis/shared/cmd/*`. O DSN continua com `loc=Local`.
- **🔴 TestBrain:**
  - `apis/rotaperfumes-api/services/data_local_internal_test.go`: 11 pontos × 6 fusos × 5 datas.
  - `apis/rotaperfumes-api/services/data_local_integration_test.go` (roda com `INTEGRATION=1`): 24/24 casos gravam, leem e re-salvam sem deslocamento.
  - `parse_data_local_test.go` em 4 importadores (`importpedidos`, `importpagamentos`, `importoportunidades`, `importvisitas`).
  - Cobertura de `services`: **94,3%**.

**Documentação (🔵 SubBrain):** o formato das datas na resposta não mudou. Foi incluída uma nota em `postman/README.md` e nas pastas Pedidos, Pagamentos, Oportunidades e Visitas da collection.

**Follow-ups:** **RISCO-01** (horário de verão histórico com `TZ=America/Sao_Paulo`) e **BUG-04** (salvar sem alterações retorna 404, mais provável após esta correção), ambos em `afazer.md`.

## Tooling frontend: `npm run lint` quebrado e ausência de runner de testes — 2026-09-24
**Agentes:** 🟢 FrontBrain → 🔴 TestBrain → fechamento por 🔵 SubBrain

**Origem:** 🟢 FrontBrain (2026-09-24). `next lint` foi removido no Next 16, não havia ESLint configurado, e `make test-frontend` terminava com `|| true`, então nunca falhava.

**O que foi feito:**
- **🟢 FrontBrain:**
  - ESLint com flat config (`frontend/eslint.config.mjs`), eslint 9 + eslint-config-next 16.
  - O script `lint` agora é `eslint .`.
  - Resultado: **0 erros**, 38 warnings. Desses, 34 são `react-hooks/set-state-in-effect`, configurada como `warn`.
- **🔴 TestBrain:**
  - Vitest 4 + Testing Library + jsdom (`frontend/vitest.config.mts`, `frontend/vitest.setup.ts`).
  - Scripts `test`, `test:watch` e `test:coverage`.
  - `make test-frontend` sem `|| true`.
  - Resultado: **9 arquivos / 129 testes verdes**, cobertura de **91,96% das linhas** no escopo configurado.

**Follow-up:** **FE-03** (eliminar os 34 warnings `react-hooks/set-state-in-effect` e voltar a regra para `error`) em `afazer.md`.

---

## Lote de 2026-09-24: 8 cards concluídos (de 10)

> Lote executado pelo 🤍 MegaBrain com os 10 cards que estavam em `afazer.md`. Os 2 cards de teste manual (PedidoModal e Dashboard) continuam em `fazendo.md` até o usuário validar no navegador. Os follow-ups deste lote estão em `afazer.md`: BUG-01, UI-01, UI-02, vendedor desligado nas escritas, `id_vendedor` no localStorage e tooling do frontend.
>
> **Verificação final (🔴 TestBrain):** `go vet` e `go test` OK nos 2 módulos. Cobertura: `handlers` 82,7%, `services` 94,1%, `shared/repositories` 83,0%. Typecheck do frontend OK (🟢 FrontBrain).

## Elevar cobertura do pacote `handlers` para o mínimo de 80% — 2026-09-24
**Agentes:** 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** 🔴 TestBrain, durante os cards de escopo de 2026-09-23. A cobertura de `handlers` estava abaixo de 80%.

**O que foi feito:**
- **🔴 TestBrain:** novo `apis/rotaperfumes-api/handlers/auth_refresh_logout_test.go`, que cobre os fluxos de refresh e logout. Cobertura de `handlers`: **78,3% → 82,7%**, acima do mínimo de 80%.

## Padronizar status de `POST /api/pagamentos` com pedido inexistente (400 admin vs 404 normal) — 2026-09-24
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** 🟡 BackBrain / 🔴 TestBrain, durante o card "Scope check em Create/Update de Pedidos" (2026-09-23).

**O que foi feito:**
- **🟣 SecBrain:** recomendou `404` para os dois perfis. O corpo é idêntico ao de pedido de outro vendedor, então não permite enumeração.
- **🟡 BackBrain:** `apis/rotaperfumes-api/handlers/pagamento_handler.go`: `POST /api/pagamentos` com `pedido_id` inexistente agora retorna `404` "pedido não encontrado" para admin e normal. `pedido_id <= 0` continua `400` "pedido_id é obrigatório".
- **🟢 FrontBrain:** nenhuma mudança. `PagamentoModal` já exibe a mensagem do `404`.
- **🔴 TestBrain:** testes ajustados em `pagamento_handler_test.go` e `escopo_escrita_pedido_pagamento_test.go`.

**Documentação (🔵 SubBrain):**
- `postman/README.md`: a nota "Inconsistência conhecida" foi trocada por "Status padronizado (2026-09-24)", e a descrição de `POST /api/pagamentos` foi atualizada.
- `postman/collection.json`: em "Criar Pagamento", a descrição e os códigos de erro foram atualizados. O exemplo "400 pedido não encontrado (admin)" virou "404 Not Found — pedido inexistente (admin)", e entrou o exemplo "400 Bad Request — pedido_id ausente ou <= 0".

## Remover `meta_mensal` da resposta de `GET /api/vendedores` para usuário normal — 2026-09-24
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** 🟣 SecBrain, durante o card "Rotas de Vendedores admin-only" (2026-09-23). Decisão do usuário: omitir o campo para `normal`.

**O que foi feito:**
- **🟡 BackBrain:** verificou que `GET /api/vendedores` já devolvia `VendedorResumo` (`id`, `nome`, `regiao`, `uf`, `data_desligamento`), sem `meta_mensal`, para **todos** os perfis. O admin obtém a meta via `GET /api/vendedores/{id}` (admin only). **Nenhuma mudança de código.**
- **🔴 TestBrain:** teste de regressão garantindo que `meta_mensal` não aparece na listagem (`vendedor_handler_test.go` / `vendedor_repository_test.go`).

**Documentação (🔵 SubBrain):** `postman/README.md`: a nota desatualizada da seção Vendedores ("ainda expõe `meta_mensal`") foi substituída pela descrição correta do `VendedorResumo`.

## Aplicar `gofmt` em testes de Oportunidades, Visitas e Estoque — 2026-09-24
**Agentes:** 🔴 TestBrain → fechamento por 🔵 SubBrain

**O que foi feito:**
- **🔴 TestBrain:** `gofmt -w` em `apis/rotaperfumes-api/handlers/oportunidade_handler_test.go`, `apis/rotaperfumes-api/handlers/visita_handler_test.go` e `apis/shared/repositories/estoque_repository_test.go`. Só formatação. `go test` e `go vet` continuam OK.

## Bug: total de vendas truncado para `int` em `enrichWithVendas` / `enrichMetasWithVendas` (perda de centavos) — 2026-09-24
**Agentes:** 🟡 BackBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23).

**O que foi feito:**
- **🟡 BackBrain:** `apis/shared/repositories/dashboard_repository.go`: `top_vendedores[].total_vendas` e `metas_vendedores[].realizado` agora são `float64` com centavos. O percentual é calculado a partir do valor `float64`.
- **🟢 FrontBrain:** `fmtCurrency` em `frontend/src/app/dashboard/page.tsx` passou a mostrar 2 casas decimais.
- **🔴 TestBrain:** casos com centavos em `dashboard_repository_test.go`.

**Documentação (🔵 SubBrain):** os exemplos de `GET /api/dashboard/metrics` na collection e no README agora têm centavos (ex.: `9500.57`, percentual `95.0057`).

## Padronizar `top_vendedores`/`metas_vendedores` vazios como `[]` (hoje `null`) — 2026-09-24
**Agentes:** 🟡 BackBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23).

**O que foi feito:**
- **🟡 BackBrain:** `apis/shared/repositories/dashboard_repository.go`: `top_vendedores` e `metas_vendedores` (e também `por_segmento`/`por_uf` do dashboard de clientes) sempre devolvem `[]` quando vazios.
- **🔴 TestBrain:** caso vazio coberto nos testes do repositório.

**Documentação (🔵 SubBrain):**
- `postman/collection.json`: o teste de "Dashboard — Métricas" ganhou a asserção "top_vendedores e metas_vendedores são arrays (nunca null)", e o exemplo "Métricas do dia" mostra listas `[]`.
- `postman/README.md`: nota de contrato das listas na seção Dashboard.

## Dashboard de vendedor desligado (`data_desligamento`): bloquear acesso — 2026-09-24
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23). Decisão do usuário (2026-09-24): bloquear o acesso.

**O que foi feito:**
- **🟡 BackBrain:**
  - `apis/rotaperfumes-api/handlers/dashboard_handler.go`: um usuário normal com vendedor desligado (`data_desligamento` preenchida) ou com vendedor inexistente recebe os 4 endpoints (`metrics`, `vendas`, `vendedores`, `clientes`) vazios ou zerados. Erro de banco nessa checagem retorna `500`.
  - `apis/rotaperfumes-api/services/dashboard_service.go`: `/api/dashboard/metrics` ganhou o campo `vendedor_desligado`. Ele é `true` só para o vendedor desligado e `false` nos demais casos, inclusive para o admin.
  - `apis/shared/repositories/vendedor_repository.go`: suporte à checagem.
- **🟢 FrontBrain:** `frontend/src/lib/types.ts` recebeu `vendedor_desligado`. `frontend/src/app/dashboard/page.tsx` mostra o aviso de vendedor desligado com base nesse flag da API.
- **🔴 TestBrain:** casos de vendedor desligado nos testes de handler, service e repositório do dashboard.

**Documentação (🔵 SubBrain):**
- `postman/collection.json`: a descrição de "Dashboard — Métricas" documenta o `vendedor_desligado` e o `500`. Todos os exemplos têm o campo, e há um novo exemplo "200 OK — Normal com vendedor desligado (zerado, vendedor_desligado=true)". As descrições de "Vendas" e "Ranking Vendedores" cobrem o caso desligado.
- `postman/README.md`: seção Dashboard com a regra do vendedor desligado e do campo `vendedor_desligado`.

**Follow-up:** o bloqueio vale só para o Dashboard. As escritas (pedidos, pagamentos, oportunidades e visitas) continuam liberadas para o vendedor desligado. Card de decisão de negócio em `afazer.md`.

## `GET /api/dashboard/clientes`: escopo da carteira para usuário normal — 2026-09-24
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23). Decisão do usuário (2026-09-24): restringir à carteira do vendedor.

**O que foi feito:**
- **🟡 BackBrain:** em `dashboard_handler.go`, `dashboard_service.go` e `apis/shared/repositories/cliente_repository.go`, o `normal` vê só os clientes da carteira ativa do próprio vendedor (`data_fim IS NULL`), e o admin continua com a visão global. Sem acesso (sem vendedor, desligado ou inexistente), a resposta é `{"periodo":"month","total_clientes":0,"total_ativos":0,"total_inativos":0,"novos_no_periodo":0,"por_segmento":[],"por_uf":[]}`.
- **🟢 FrontBrain:** `dashboard/page.tsx` usa os rótulos "Meus Clientes" e "Clientes da minha carteira" para o normal.
- **🔴 TestBrain:** perfis admin e normal cobertos. Novos `apis/shared/repositories/cliente_repository_contagens_erro_test.go` e `apis/rotaperfumes-api/services/dashboard_service_cliente_erros_test.go`.

**Documentação (🔵 SubBrain):**
- `postman/collection.json`: a descrição de "Dashboard — Clientes" passou de "apenas admin" para acesso comum com escopo. O `403` saiu dos códigos de erro e entrou o `500`. O exemplo "403 Não é admin" foi substituído por "200 OK — Normal (só a carteira do próprio vendedor)" e "200 OK — Normal sem vendedor / vendedor desligado (zerado)".
- `postman/README.md`: o título da seção Clientes e o item `GET /api/dashboard/clientes` descrevem o escopo. A nota "`/api/dashboard/clientes` não mudou" da seção Dashboard foi substituída.

---

## Validar com o negócio: pedido com cliente transferido (visibilidade de pedidos antigos + escopo do Dashboard) — 2026-09-23
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** efeito colateral aceito no card "Scope check em Create/Update de Pedidos" (2026-09-23). Um pedido cujo cliente foi transferido para outro vendedor não pode mais ser editado pelo vendedor original (`400` carteira).

**Decisão do usuário (2026-09-23):**
- "Não mostrar os pedidos antigos do cliente ao novo vendedor; só o admin vê todos."
- Dashboard: "só os números dele".

**Regra resultante:**
- O pedido pertence ao vendedor que o registrou (`pedidos.vendedor_id`).
- Depois da transferência do cliente, o novo vendedor não vê os pedidos antigos desse cliente. Isso vale para pedidos, pagamentos e Dashboard.
- O vendedor original continua vendo os próprios pedidos, mas não consegue editá-los (`400` carteira, comportamento mantido).
- O admin vê tudo.

**O que foi feito:**
- **🟣 SecBrain:** a auditoria mostrou que pedidos e pagamentos já filtravam por `pedidos.vendedor_id`, então a regra já era cumprida nessas telas. O Dashboard era global, e qualquer usuário autenticado via os números de todos. Esse era o único gap.
- **🟡 BackBrain:**
  - `apis/rotaperfumes-api/handlers/dashboard_handler.go`: novo helper `resolverEscopo`, usado em `GetMetrics`, `GetVendas` e `GetVendedores`. Usuário normal sem vendedor recebe `200` zerado ou vazio.
  - `apis/rotaperfumes-api/services/dashboard_service.go`: parâmetro `vendedorID` em `GetMetrics`, `GetVendasSeries` e `GetVendedoresRanking`. Novos `EmptyMetrics` e `EmptyVendasSeries`.
  - `apis/shared/repositories/dashboard_repository.go`: filtro `vendedorFilter` aplicado às consultas e novo `EmptySeries`.
  - Testes existentes ajustados para as novas assinaturas.
  - `/api/dashboard/clientes` não mudou.
- **🟢 FrontBrain:** `frontend/src/app/dashboard/page.tsx`:
  - Rótulos "Minhas Vendas", "Meus Pedidos" e "Minha Meta" para o usuário normal.
  - Card "Meu Desempenho" no lugar do ranking.
  - Aviso para usuário sem vendedor vinculado.
  - Tolerância a `null` e `[]` nas listas.
- **🔴 TestBrain:** 4 arquivos de teste novos:
  - `apis/shared/repositories/dashboard_repository_escopo_test.go`
  - `apis/shared/repositories/dashboard_repository_internal_test.go`
  - `apis/rotaperfumes-api/handlers/dashboard_escopo_handler_test.go`
  - `apis/rotaperfumes-api/services/dashboard_service_escopo_test.go`

  Cobertura: `handlers` 78,1%, `services` 93,7%, `repositories` 80,3%.

**Documentação (🔵 SubBrain):**
- `postman/README.md`: a seção Dashboard deixou de dizer "admin only" e agora tem uma nota de escopo por vendedor (admin, normal, normal sem vendedor) e a regra de negócio do cliente transferido. `metrics`, `vendas` e `vendedores` descrevem o comportamento do normal. Há também uma nota nos testes automatizados sobre o uso de `{{vendedor_token}}`.
- `postman/collection.json`:
  - As descrições de "Dashboard — Métricas", "Vendas" e "Ranking Vendedores" agora documentam o escopo e a regra de negócio. O `403` saiu dos códigos de erro e entrou o `500`.
  - Os exemplos "403 Não é admin" foram substituídos por "200 Normal (só os próprios números / só a própria linha)" e "200 Normal sem vendedor vinculado" (zerado, N dias zerados, lista vazia).

**Follow-ups:** registrados em `afazer.md`: bug de centavos em `enrichWithVendas`, `null` vs `[]`, vendedor desligado, escopo de `/dashboard/clientes` e teste manual do Dashboard.

## Aplicar scope check (owner check + client scoping) em Create/Update de Pedidos — 2026-09-23
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** identificado por 🟣 SecBrain e confirmado por 🟡 BackBrain durante o card "Menu do vendedor: Visitas/Oportunidades + escopo por carteira; Produtos movido para Administração" (2026-09-22). Gap pré-existente em `pedido_handler.go`.

**Problema:** `CreatePedido`/`UpdatePedido` aceitavam `vendedor_id` do payload sem validar escopo. Um usuário `normal` podia forjar ou reatribuir um pedido para outro vendedor. O 🟣 SecBrain achou o mesmo bypass em Pagamentos (Create/Update), que foi incluído no card.

**O que foi feito:**
- **🟣 SecBrain:** parecer confirmando o gap em Pedidos e ampliando o escopo para Pagamentos. Definiu os status: `403` para usuário sem vendedor no Create, `404` com corpo idêntico ao de registro inexistente (sem enumeração) no Update/owner check, e `400` para cliente fora da carteira.
- **🟡 BackBrain:**
  - `apis/rotaperfumes-api/handlers/pedido_handler.go`:
    - `CreatePedido`: `403` "usuário sem vendedor vinculado"; `vendedor_id` forçado ao vendedor do usuário; `400` "cliente não pertence à carteira deste vendedor".
    - `UpdatePedido`: `404` "pedido não encontrado" para usuário sem vendedor, pedido de outro vendedor ou inexistente (corpo idêntico); `vendedor_id` forçado; `400` para cliente fora da carteira.
  - `apis/rotaperfumes-api/handlers/pagamento_handler.go`:
    - Novo helper `pedidoNoEscopo`.
    - `CreatePagamento`: `403` sem vendedor; `404` "pedido não encontrado" para pedido de outro vendedor ou inexistente.
    - `UpdatePagamento`: `404` "pagamento não encontrado" sem vendedor ou para pagamento de outro vendedor.
  - Testes existentes ajustados: `pagamento_handler_test.go` passou a usar token admin em Create/Update; `pedido_handler_test.go` ganhou mocks de escopo.
- **🟢 FrontBrain:** `frontend/src/components/admin/PedidoModal.tsx`:
  - Usuário `normal`: vendedor travado em `user.id_vendedor`; se não houver vendedor, mostra aviso e bloqueia o botão Salvar.
  - Clientes carregados por `apiListClientesDoVendedor`, em cascata vendedor → cliente (também para admin).
  - Na edição, o cliente atual é mantido na lista como "(fora da carteira)".
  - Mensagem amigável para o `400` de carteira.
  - Typecheck OK; lint não configurado.
- **🔴 TestBrain:** novo `apis/rotaperfumes-api/handlers/escopo_escrita_pedido_pagamento_test.go`. `go test ./...` e `go vet` OK. Cobertura de `handlers`: 73,9% → 77,7%.

**Efeito colateral aceito:** um pedido cujo cliente foi transferido para outro vendedor não pode mais ser editado pelo usuário `normal` (`400`). Precisa ser validado com o negócio (follow-up em `afazer.md`).

**Documentação (🔵 SubBrain):**
- `postman/README.md`: seção Pedidos deixou de dizer "admin only" e agora descreve o escopo por carteira em POST/PUT. Seção Pagamentos ganhou as regras de escopo em POST/PUT e a nota sobre a inconsistência 400/404.
- `postman/collection.json`:
  - "Criar Pedido" e "Editar Pedido" renomeados para "acesso comum, escopo por carteira", com descrições e códigos de erro atualizados.
  - Os exemplos "403 Não é admin" foram substituídos por "400 Cliente fora da carteira" e "403 sem vendedor vinculado".
  - "Criar Pagamento" e "Editar Pagamento" com escopo documentado e novos exemplos `404` e `403`.

## Rotas de Vendedores (`/api/vendedores*`) restritas a admin no backend — 2026-09-23
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** identificado por 🟢 FrontBrain (2026-09-22) durante o bugfix "menu 'Vendedores' aparecia para login de vendedor".

**Problema:** todas as rotas `/api/vendedores*` usavam `JWTMiddleware(cfg, true, false)`, ou seja, acesso comum. A restrição a admin existia só no frontend. O 🟣 SecBrain encontrou dois riscos além do card:
- `GET /api/vendedores/{id}` expunha a carteira de qualquer vendedor.
- `POST /api/vendedores/{id}/clientes` permitia a um usuário `normal` transferir clientes de outros vendedores para a própria carteira (**crítico**).

**O que foi feito:**
- **🟣 SecBrain:** parecer confirmando a intenção admin-only. Manteve com acesso comum só as duas rotas GET usadas por fluxos de vendedor: `GET /api/vendedores` (selects) e `GET /api/vendedores/{id}/clientes` (dropdown em cascata).
- **🟡 BackBrain:** `apis/rotaperfumes-api/routes/routes.go` passou a usar `requireAdmin=true` (usuário `normal` recebe `403`) em:
  - `POST /api/vendedores`
  - `GET/PUT/DELETE /api/vendedores/{id}`
  - `POST /api/vendedores/{id}/reativar`
  - `POST /api/vendedores/{id}/clientes`
  - `DELETE /api/vendedores/{id}/clientes/{clienteId}`

  Doc comments de `vendedor_handler.go` e o índice de rotas em `routes.go` foram atualizados.
- **🔴 TestBrain:** novo `apis/rotaperfumes-api/routes/routes_test.go`, que cobre `403` para `normal` nas rotas restritas e acesso para `admin` e nas rotas comuns. Cobertura de `routes`: 0% → 97,6%.

**Documentação (🔵 SubBrain):**
- `postman/README.md`: nova seção "Vendedores" com a tabela de acesso por rota.
- `postman/collection.json`: nota em "Listar Clientes do Vendedor" sobre quais rotas continuam com acesso comum e quais passaram a ser admin only. A collection não tinha requests para as demais rotas de vendedores.

**Responsável:** 🤍 MegaBrain

---

## Bugfix: sem mensagem clara ao trocar senha pela mesma senha atual — 2026-09-23
**Agentes:** 🟡 BackBrain + 🟢 FrontBrain (em paralelo)

**Origem:** pedido direto do usuário — ao informar a nova senha igual à atual, a tela não indicava que esse era o problema.

**Causa raiz:** a validação de força da senha rodava antes da checagem de reuso (no frontend em `trocar-senha/page.tsx` e no backend em `auth_handler.go`). Se a senha atual fosse fraca (ex.: senha padrão), o usuário via o erro de força em vez do erro de reuso. Além disso, quando o reuso era detectado, a mensagem era genérica ("uma das últimas senhas utilizadas").

**Correção:**
- `apis/rotaperfumes-api/handlers/auth_handler.go`: depois de confirmar a senha atual e antes de `ValidarForcaSenha`, `nova_senha == senha_atual` → 400 "a nova senha não pode ser igual à senha atual" (sem `RegisterFailure`, porque a senha atual já foi confirmada). A checagem bcrypt do histórico continua igual, com a mensagem genérica.
- `frontend/src/app/trocar-senha/page.tsx`: `validate()` mostra "A nova senha nao pode ser igual a senha atual" antes das regras de tamanho e de tipos de caractere.

**Testes:** `TestResetPassword_NovaSenhaIgualASenhaAtual` virou table-driven (senha atual forte e fraca), com igualdade exata da nova mensagem. `go build ./...`, `go vet` e `go test ./handlers/...` OK. Frontend: `tsc --noEmit` OK (não testado no navegador).

**Documentação:** o `POST /api/auth/reset-password` ganhou uma mensagem de erro 400 nova. O contrato não muda.

## Bugfix: nome do usuário não aparecia na Auditoria de Senha — 2026-09-23
**Agentes:** 🟡 BackBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** pedido direto do usuário.

**Causa raiz:** `FindByUsuario`/`FindAll` (`apis/shared/repositories/senha_historico_repository.go`) nunca faziam JOIN com `usuarios` — o struct `SenhaHistorico` não tinha campo de nome e o handler (`senha_historico_handler.go`) nunca incluía `usuario_nome`/`resetado_por_nome` no JSON, mesmo o frontend já esperando esses campos (`SenhaHistoricoItem`).

**Correção:**
- `senha_historico_repository.go`: struct ganhou `UsuarioNome string` e `ResetadoPorNome sql.NullString`; queries agora fazem `LEFT JOIN usuarios u1 ON u1.id = sh.usuario_id` e `LEFT JOIN usuarios u2 ON u2.id = sh.resetado_por_id` (ambos LEFT — `resetado_por_id` é nulo quando a troca foi feita pelo próprio usuário); colunas/whitelist de ordenação prefixadas com `sh.` para evitar ambiguidade de `id` entre as tabelas.
- `senha_historico_handler.go`: `ListarTodos`/`ListarPorUsuario` agora retornam `usuario_nome` sempre e `resetado_por_nome` só quando há reset por terceiro (mesmo padrão condicional já usado para `resetado_por_id`).

**Testes (🔴 TestBrain):** mocks de SQL atualizados nos 4 arquivos de teste afetados pela mudança de query (`senha_historico_repository_test.go`, `senha_historico_handler_test.go`, `senha_historico_service_test.go`, `auth_handler_test.go` — este último porque o fluxo de bloqueio de reuso de senha, adicionado no card anterior, também consulta o histórico). Casos novos cobrindo nome preenchido/nulo em ambos os campos. `go test ./...` passa em `apis/shared` e `apis/rotaperfumes-api`.

**Documentação (🔵 SubBrain):** contrato do `GET /api/senha-historico` e `GET /api/senha-historico/{usuario_id}` ganhou 2 campos novos (`usuario_nome`, `resetado_por_nome`) — sem breaking change (campos aditivos). Nada a atualizar em Postman além de nota informativa, se existir coleção salva com exemplos de resposta desses endpoints.

## Bugfix: tipo_reset errado na Auditoria de Senha + nota obsoleta — 2026-09-23
**Agentes:** 🟢 FrontBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** pedido direto do usuário — ao trocar a própria senha, a tela mostrava o tipo como "Primeiro Login" em vez de "Proprio".

**Causa raiz:** o backend sempre gravou `tipo_reset` corretamente (`usuario`, `admin`, `primeiro_acesso`, `esquecimento`), mas o frontend (`frontend/src/lib/types.ts`) definia `TipoReset` com valores diferentes (`"proprio" | "admin" | "primeiro_login"`). Como o valor real `"usuario"` nunca batia com `"proprio"`, a lógica de rótulo/badge em `frontend/src/app/admin/senha-historico/page.tsx` caía no `else` genérico e exibia "Primeiro Login" para qualquer troca que não fosse feita por admin. Nenhuma mudança de backend foi necessária.

**Correção:**
- `frontend/src/lib/types.ts`: `TipoReset` corrigido para os 4 valores reais do backend.
- `frontend/src/app/admin/senha-historico/page.tsx`: `TIPO_OPTIONS`, `tipoBadgeColor()` e `tipoLabel()` reescritos com mapeamento explícito (switch) dos 4 tipos — sem fallback silencioso (valor desconhecido agora mostra o valor bruto em vez de rotular errado, para evitar recorrência). Removida a nota obsoleta no rodapé sobre os endpoints `GET /api/senha-historico*` "talvez não implementados" (já existem e funcionam). Legenda dos tipos atualizada para os 4 valores reais.

**Verificação:** `tsc --noEmit` sem erros. Sem testes de frontend existentes para essa tela (não havia suíte prévia a atualizar).

**Documentação (🔵 SubBrain):** sem mudança de contrato de API — nada a atualizar em Postman/README.

## Autoatendimento de troca de senha (menu) + política de senha forte + bloqueio de reuso — 2026-09-23
**Agentes:** 🟣 SecBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** pedido direto do usuário.

**Descrição:**
1. **Menu (`frontend/src/components/layout/Navbar.tsx`):** adicionado link "Trocar Senha" (`/trocar-senha`) visível para qualquer usuário autenticado (admin ou normal/vendedor), ao lado do link "Dashboard". Antes, a página só era alcançada via redirect forçado de primeiro acesso (`deve_trocar_senha`); confirmado que ela já funciona por navegação livre.
2. **Política de senha forte:** nova função `services.ValidarForcaSenha` em `apis/shared/services/password_policy.go` — mínimo 8 caracteres (por rune), máximo 72 bytes (limite do bcrypt, rejeitado explicitamente em vez de truncar silenciosamente), e ao menos 3 das 4 classes (minúscula/maiúscula/dígito/símbolo). Usada como fonte de verdade em `POST /api/auth/reset-password` (`auth_handler.go`), substituindo o antigo check de 6 caracteres. Frontend (`trocar-senha/page.tsx`) replica a regra só como feedback de UX antecipado.
3. **Bloqueio de reuso das últimas 3 senhas:** em `ResetPassword`, a nova senha é comparada (bcrypt, sem short-circuit — evita side-channel de timing) contra o hash atual do usuário + os 2 registros mais recentes de `senha_historico.senha_hash_anterior` (3 candidatos = "últimas 3 senhas"). Em caso de reuso: 400 com mensagem genérica ("a nova senha não pode ser igual a uma das últimas senhas utilizadas", sem indicar qual bateu) e conta como falha no `resetPasswordLimiter` (rejeição por senha fraca não conta, é erro de usabilidade normal).
4. **Reset de senha pelo admin (`AdminResetPassword`, `usuario_service.go`):** a senha aleatória gerada agora é comparada com o hash atual do usuário-alvo; em caso de colisão (estatisticamente improvável, dado o espaço de busca da geração aleatória), a senha é regenerada automaticamente (até 3 tentativas) antes de persistir — defesa em profundidade sem impacto observável pelo admin.
5. **Testes:** `apis/shared/services/password_policy_test.go` (novo, 15 casos, 100% de cobertura de `ValidarForcaSenha`) e casos adicionados em `auth_handler_test.go`, `usuario_handler_test.go` e `usuario_service_test.go` cobrindo senha fraca, reuso contra senha atual/histórico, contagem no rate limiter e caminho de sucesso. `go test ./...` ok em `apis/shared` e `apis/rotaperfumes-api`.

**Observação registrada pelo 🔴 TestBrain (não bloqueante):** como o rate limiter do reset-password reseta o contador de falhas (`RegisterSuccess`) sempre que a `senha_atual` está correta, tentativas isoladas de reuso (sempre com senha atual correta) não acumulam sozinhas até o limite de bloqueio — só bloqueiam combinadas com falhas de `senha_atual`. Como o atacante já precisa saber a senha atual para tentar essa enumeração, o risco residual é baixo; mantido como está, sem ação adicional.

**Documentação (🔵 SubBrain):** sem novo endpoint público nem mudança de contrato de API (mensagens de erro do `POST /api/auth/reset-password` documentadas no card; `POST /api/admin/reset-password` mantém contrato idêntico) — Postman não precisa de atualização de rota, apenas nota interna sobre as novas mensagens de erro possíveis nesse endpoint.

## Seleção de pedido por combobox com busca na criação de pagamento — 2026-09-22
**Agentes:** 🟢 FrontBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** pedido direto do usuário (não estava registrado em `tarefas/afazer.md`/`tarefas/fazendo.md` — nenhum card precisou ser movido; card criado diretamente em `feito.md`).

**Descrição:** No modal de criação de pagamento (`frontend/src/components/PagamentoModal.tsx`), o antigo campo `<Input type="number">` de `pedido_id` (texto livre) foi substituído, no modo "create", por um campo de busca (filtra por nome do cliente ou ID do pedido, debounce de 350ms) + `<Select>` nativo, reaproveitando `apiListPedidos` já existente em `frontend/src/lib/api.ts` — sem novo endpoint de backend. Cada opção exibe `#<id> - <cliente_nome> - R$ <valor_total> - <data_pedido>`. Modo "edit" não foi alterado (`pedido_id` já era somente leitura, consistente com o backend que não permite alterar `pedido_id` via `PUT`). Validado com `tsc --noEmit`, sem erros.

**Documentação (🔵 SubBrain):** sem mudança de API/contrato de backend — Postman e README não alterados neste card.

**Responsável:** 🤍 MegaBrain

---

## Botão de excluir (delete) nas telas de Pedidos, Pagamentos, Oportunidades e Visitas — 2026-09-22
**Agentes:** 🟣 SecBrain (regras) → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Origem:** pedido direto do usuário (não estava previamente registrado em `tarefas/afazer.md`/`tarefas/fazendo.md` — nenhum card precisou ser movido; card criado diretamente em `feito.md`).

**Descrição:** Adição de botão "Excluir" nas 4 telas de listagem já existentes (Pedidos, Pagamentos, Oportunidades, Visitas), com hard delete no backend (sem soft-delete/exclusão lógica em nenhum dos 4 recursos).

**Regras de negócio (🟣 SecBrain):**
- Hard delete nos 4 recursos.
- Scope check idêntico ao já usado em `GET`/`PUT` de cada recurso (owner check via `vendedorScope`/`resolverVendedorScope`, `apis/rotaperfumes-api/handlers/scope.go`): usuário `normal` só exclui registro da própria carteira, recebendo `404` — nunca `403` — para registro de outro vendedor (evita IDOR por enumeração de ID).
- **Pedido:** bloqueado com `409` se tiver pagamento(s) vinculado(s), ou se `status = "Faturado"` (nesse caso só o status pode ser alterado, via `PUT`).
- **Pagamento:** bloqueado com `409` se `status_pagamento` for `"Pago"` ou `"Pago com atraso"` (preserva a trilha financeira de pagamentos já quitados).
- **Oportunidade** e **Visita:** sem restrição extra de negócio além do scope check.

**Backend (🟡 BackBrain) — Go, `apis/rotaperfumes-api` e `apis/shared`:**
- Novas rotas (`apis/rotaperfumes-api/routes/routes.go`), todas **acesso comum** (JWT obrigatório, sem `requireAdmin`): `DELETE /api/pedidos/{id}`, `DELETE /api/pagamentos/{id}`, `DELETE /api/oportunidades/{id}`, `DELETE /api/visitas/{id}`.
- Handlers `DeletePedido`/`DeletePagamento`/`DeleteOportunidade`/`DeleteVisita` (`handlers/pedido_handler.go`, `handlers/pagamento_handler.go`, `handlers/oportunidade_handler.go`, `handlers/visita_handler.go`): validam `id`, resolvem `vendedorScope`, buscam o registro atual para checar dono (404 se fora do escopo) e então chamam o service. Retorno de sucesso: `204 No Content` sem corpo.
- Services (`services/pedido_service.go`, `services/pagamento_service.go`, `services/oportunidade_service.go`, `services/visita_service.go`): novos métodos `DeletePedido`/`DeletePagamento`/`DeleteOportunidade`/`DeleteVisita`, com os novos erros de domínio `ErrPedidoPossuiPagamentosVinculados` ("pedido possui pagamentos vinculados: remova-os antes de excluir o pedido"), `ErrPedidoFaturadoNaoPodeSerExcluido` ("pedido faturado não pode ser excluído, apenas ter o status alterado") e `ErrPagamentoJaQuitadoNaoPodeSerExcluido` ("pagamento já quitado não pode ser excluído"), mapeados para `409` nos handlers.
- Novos métodos de repositório (`apis/shared/repositories/`): `DeleteComItens` (pedido — remove pedido + itens), `ExistsByPedidoID` e `Delete` (pagamento), `Delete` (oportunidade e visita).

**Frontend (🟢 FrontBrain) — Next.js, `frontend/`:**
- Novas funções `apiDeletePedido`/`apiDeletePagamento`/`apiDeleteOportunidade`/`apiDeleteVisita` em `frontend/src/lib/api.ts`.
- Botão "Excluir" com `confirm()` de confirmação, estado de loading e tratamento de erro (`404`/`409` exibidos ao usuário) nas 4 telas: `frontend/src/app/admin/pedidos/page.tsx`, `frontend/src/app/pagamentos/page.tsx`, `frontend/src/app/admin/oportunidades/page.tsx`, `frontend/src/app/admin/visitas/page.tsx`.

**Testes (🔴 TestBrain):** 26 novos casos de teste (handlers + services) cobrindo sucesso (204), 404 fora do escopo do vendedor, 404 registro inexistente, e os bloqueios 409 (pedido com pagamento vinculado, pedido faturado, pagamento já quitado). Suíte completa (`go test ./...`) 100% verde em `apis/shared` e `apis/rotaperfumes-api`; cobertura 100% nos novos métodos de service.

**Documentação (🔵 SubBrain):**
- `postman/collection.json` — adicionadas 4 novas requests `DELETE`, uma em cada pasta já existente ("Pedidos", "Pagamentos", "Oportunidades", "Visitas"), logo após o respectivo "Editar": "Excluir Pedido (admin only)" (auth `{{admin_token}}`, seguindo o padrão já usado nas demais requests de Pedidos da collection — nota: o rótulo "admin only" da pasta Pedidos já estava desatualizado antes deste card, pois a rota real é acesso comum desde a mudança de escopo por carteira; correção dessa nomenclatura ficou fora do escopo aqui), "Excluir Pagamento (acesso comum)" (auth `{{vendedor_token}}`, com teste explícito validando que não é `403`), "Excluir Oportunidade (acesso comum, escopo por carteira)" e "Excluir Visita (acesso comum, escopo por carteira)" (ambas auth `{{admin_token}}`, seguindo o padrão dos requests de Editar já existentes nessas pastas). Cada request inclui exemplos de resposta `204 No Content`, `404 Not Found` (inexistente/fora da carteira) e, para Pedido e Pagamento, os cenários `409 Conflict` específicos de cada regra de negócio.
- `postman/README.md` — adicionada nota "Botão de exclusão (2026-09-22)" no cabeçalho das 4 seções de endpoints (Pedidos, Pagamentos, Oportunidades, Visitas), removendo as antigas menções de "Não existe endpoint de DELETE"; nova subseção `#### DELETE /api/.../{id}` em cada uma, documentando auth, regras de bloqueio 409 (quando aplicável) e o comportamento de scope (404 para registro de outro vendedor). Seção "Testes automatizados (Postman)": adicionados os blocos `### Excluir Pedido`/`### Excluir Pagamento`/`### Excluir Oportunidade`/`### Excluir Visita`. Tabela "Resumo de testes por endpoint" atualizada com as 4 novas linhas.
- `tarefas/afazer.md`/`tarefas/fazendo.md` — nenhuma alteração necessária: a feature não estava registrada em nenhum dos dois (pedido direto do usuário, sem passagem formal pelo Kanban antes da execução).

**Responsável:** 🤍 MegaBrain

---

## Bugfix menu "Vendedores" visível para vendedor + restrição de Estoque a admin — 2026-09-22
**Agentes:** 🟢 FrontBrain, 🟡 BackBrain, 🔴 TestBrain (execução direta, pedido pontual do usuário) → documentação e fechamento por 🔵 SubBrain

> **Nota:** estas duas mudanças foram pedidas diretamente pelo usuário e implementadas/testadas na sessão sem passar formalmente por `tarefas/fazendo.md`. Registradas aqui retroativamente para manter o histórico completo.

**1) Bugfix: menu "Vendedores" aparecia para login de vendedor**
- **Pedido do usuário:** o dropdown "Administração" continuava mostrando o item "Vendedores" para um usuário logado como vendedor (`normal`), que não deveria ter acesso.
- **Causa raiz (🟢 FrontBrain):** `frontend/src/components/layout/Navbar.tsx` — o item "Vendedores" do dropdown "Administração" não tinha a checagem `user.role === "admin"`, diferente dos demais itens do mesmo dropdown.
- **Correção:** `Navbar.tsx` — gate `user.role === "admin"` adicionado ao item "Vendedores". `frontend/src/app/admin/vendedores/page.tsx` — envolvido com `<ProtectedRoute requireAdmin>` (não tinha proteção de rota nenhuma).
- **Achado extra (não corrigido neste card, virou backlog):** as rotas `/api/vendedores*` no backend (`apis/rotaperfumes-api/routes/routes.go`) não são admin-only — acesso comum, protegido só no frontend. Registrado em `tarefas/afazer.md`.

**2) Estoque restrito a admin (pedido do usuário)**
- **Pedido do usuário:** a página de Estoque deve ser visível apenas para admins.
- **🟢 FrontBrain:** `Navbar.tsx` — "Estoque" movido do dropdown "CRM" para "Administração", gated `role === "admin"`. `frontend/src/app/admin/estoque/page.tsx` — envolvido com `<ProtectedRoute requireAdmin>`.
- **🟡 BackBrain:** `apis/rotaperfumes-api/routes/routes.go` — `GET /api/estoque` e `GET /api/estoque/{id}` deixaram de ser acesso comum e viraram admin-only (`requireAdmin=true`), alinhando com `POST`/`PUT` que já eram admin-only. Comentários de documentação do topo do arquivo atualizados. Confirmado que `/api/estoque` só é consumido pela própria tela de estoque (nenhum outro fluxo, como pedidos, depende de leitura comum).
- **🔴 TestBrain:** `apis/rotaperfumes-api/handlers/estoque_handler_test.go` — `TestListEstoque_PermitidoParaNaoAdmin`/`TestGetEstoque_PermitidoParaNaoAdmin` renomeados para `..._NegadoParaNaoAdmin` e ajustados para esperar `403`. `go test ./...` 100% verde em `apis/shared` e `apis/rotaperfumes-api`.

**Documentação (🔵 SubBrain):**
- `postman/README.md` — seção "Estoque (`/api/estoque/*`)" renomeada de "GET acesso comum, POST/PUT admin only" para "admin only"; nota de assimetria de acesso substituída por nota de mudança (2026-09-22) explicando que os 4 endpoints agora exigem admin; `Auth` de `GET /api/estoque` e `GET /api/estoque/{id}` corrigido de "qualquer usuário autenticado" para "admin". Referências cruzadas obsoletas a "Pagamentos/Estoque" (que citavam a antiga cadeia de middleware `cfg, true, false` de Estoque) corrigidas nas seções de Produtos e Oportunidades, já que Estoque não segue mais esse padrão.
- `postman/collection.json` — pasta "Estoque": "Listar Estoque (acesso comum)" → "Listar Estoque (admin only)" e "Detalhe do Estoque (acesso comum)" → "Detalhe do Estoque (admin only)", token de autenticação nos dois requests trocado de `{{vendedor_token}}` para `{{admin_token}}`, descrições atualizadas, e adicionado exemplo de resposta `403 Forbidden — Não é admin` (`{"success": false, "error": "acesso restrito a administradores"}`) em ambos, substituindo o cenário desatualizado de acesso comum com token de vendedor.
- Menu "Vendedores": não existe pasta/seção dedicada equivalente no Postman (README/collection) a atualizar — confirmado que a documentação de API de Vendedores já refletia corretamente o estado de acesso comum do backend (que segue sem mudança neste card).
- `tarefas/afazer.md` — confirmado que o item sobre `/api/vendedores*` não ser admin-only no backend já estava registrado (adicionado anteriormente), sem necessidade de duplicar.

**Responsável:** 🤍 MegaBrain

---

## Menu do vendedor: Visitas/Oportunidades + escopo por carteira; Produtos movido para Administração — 2026-09-22
**Agentes:** 🟣 SecBrain (análise) → 🟡 BackBrain + 🟢 FrontBrain (implementação em paralelo) → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Pedido do usuário:**
1. Menu do vendedor não mostra "Visitas" e "Oportunidades" — precisa aparecer.
2. Essas telas (Visitas e Oportunidades) precisam filtrar somente os clientes do vendedor logado.
3. Produtos deve ser movido para a área de Administração, gerenciado apenas por admins.

**Achados da investigação (🤍 MegaBrain, via Explore):**
- `frontend/src/components/layout/Navbar.tsx`: menu "ERP" esconde Oportunidades/Visitas de não-admin (linhas ~81-94); "Produtos" está no dropdown "CRM" (linhas ~96-106), visível para qualquer autenticado, sem gate de admin.
- `apis/rotaperfumes-api/routes/routes.go`: rotas de `/api/oportunidades` e `/api/visitas` usam `middleware.JWTMiddleware(cfg, true, true)` (admin-only) — bloqueiam vendedor mesmo se o menu for liberado. Rotas de `/api/produtos*` usam `(cfg, true, false)` (acesso comum) — precisa virar `(cfg, true, true)`.
- `handlers/oportunidade_handler.go` e `handlers/visita_handler.go` não usam `resolverVendedorScope`/`vendedorScope` (padrão já existente em `handlers/scope.go`, usado em `pedido_handler.go`, `cliente_handler.go`, `pagamento_handler.go`) — precisa ser adicionado para restringir vendedor à própria carteira (`scope.Restrito` força `filtro.VendedorID = scope.VendedorID`).
- `app/admin/produtos/page.tsx` não tem `<ProtectedRoute requireAdmin>` (padrão usado em `app/admin/usuarios/page.tsx`).
- Vínculo usuário→vendedor: `usuarios.id_vendedor` (fonte de verdade no backend, resolvido via `UsuarioRepository.GetIDVendedorByUsuarioID`); no frontend, `User.id_vendedor` em `lib/types.ts`.

**Revisão 🟣 SecBrain — plano NÃO aprovado como estava, ajustes obrigatórios incorporados:**
- Padrão `resolverVendedorScope`/`scope.go` é seguro (deriva sempre do JWT/banco, nunca de input do client) — confirmado replicável.
- `oportunidade_handler.go`/`visita_handler.go` tinham ZERO checagem de dono em Get/Create/Update. Abrir a rota só com List corrigido deixaria IDOR grave: vendedor conseguiria ler (Get), forjar (Create com vendedor_id de outro) e editar/roubar (Update) registros de outros vendedores por enumeração de ID.
- Produtos: `GET /api/produtos` é usado por `PedidoModal.tsx` para o vendedor montar pedido — bloquear TODAS as rotas de produto quebra o fluxo de pedidos. Só as mutações (POST/PUT/PATCH) devem virar admin-only; GET continua acesso comum.
- Backlog separado (fora deste card): mesmo gap de Create/Update sem scope já existe hoje em `pedido_handler.go` — registrado em `tarefas/afazer.md`.

**Entregue:**
- **Backend (🟡 BackBrain)** — `routes.go`: oportunidades/visitas com `requireAdmin=false`; produtos com GET comum e POST/PUT/PATCH admin-only. `oportunidade_handler.go`/`visita_handler.go`: List/Get/Create/Update com `vendedorScope` completo (owner check em Get/Update, forçar `VendedorID` em Create/Update ignorando payload, 404 para não-dono — nunca 403, para não permitir enumeração de IDs). Nova `clienteNaCarteiraDoVendedor` em `scope.go` valida no Create/Update que o cliente pertence à carteira do vendedor (400 se não pertence) — vai além do pedido mínimo do SecBrain. `go build`/`go vet` OK nos dois módulos.
- **Frontend (🟢 FrontBrain)** — `Navbar.tsx`: Oportunidades/Visitas liberadas para todo autenticado no menu ERP; "Produtos" movido de "CRM" para "Administração" (gated admin). `app/admin/produtos/page.tsx`: envolvido com `<ProtectedRoute requireAdmin>`. `app/admin/oportunidades/page.tsx`/`app/admin/visitas/page.tsx`: filtro de vendedor travado na própria carteira para não-admin (seletor "todos os vendedores" só aparece para admin); usuário sem `id_vendedor` tratado como carteira vazia sem chamar a API. `tsc --noEmit` OK (exit 0).
- **Testes (🔴 TestBrain)** — cobertura completa de IDOR/escopo em `oportunidade_handler_test.go`, `visita_handler_test.go` (List/Get/Create/Update, incluindo tentativa de vendedor ler/editar/criar registro de outro vendedor) e `produto_handler_test.go` (GET comum, POST/PUT/PATCH admin-only). `go test ./...` 100% verde em `apis/shared` e `apis/rotaperfumes-api`.
- **Documentação (🔵 SubBrain)** — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` — pastas "Oportunidades" e "Visitas": todos os requests de list/get/create/update renomeados de "(admin only)" para "(acesso comum, escopo por carteira)"; descrições reescritas explicando a mudança de acesso, o forçamento de `vendedor_id` para usuário `normal`, a validação de `cliente_id` contra a carteira, e o 404 (não 403) em tentativa de acesso a registro de outro vendedor; adicionados exemplos de resposta novos: `200 OK` com `{{vendedor_token}}` mostrando escopo (vendedor_id da query ignorado), `404` de IDOR bloqueado em Get/Update, e `400` de "cliente não pertence à carteira deste vendedor" em Create. Pasta "Produtos": "Listar Produtos" e "Detalhe do Produto" renomeados para "(acesso comum)", descrições e exemplos de resposta 403 (incorretos, resquício de quando GET era admin-only) substituídos por exemplos `200 OK` com usuário normal; "Criar Produto"/"Editar Produto"/"Ativar/Inativar Produto" mantidos "(admin only)" com seus exemplos de erro 403 já corretos.
- `postman/README.md` — seção "Produtos (`/api/produtos/*`)" renomeada para "GET acesso comum, POST/PUT/PATCH admin only", com nota de assimetria e `Auth` de cada endpoint corrigido. Seções "Oportunidades (`/api/oportunidades/*`)" e "Visitas (`/api/visitas/*`)" renomeadas para "acesso comum, com escopo por carteira", com bloco de nota detalhando a regra de escopo (vendedor só vê/edita a própria carteira; 404 em vez de 403 para registro de outro vendedor; `vendedor_id` do payload ignorado para usuário `normal`; `cliente_id` precisa pertencer à carteira ativa do vendedor) e `Auth` de cada endpoint atualizado.
- `tarefas/afazer.md` — novo item de backlog registrando o gap de scope em `CreatePedido`/`UpdatePedido` (`pedido_handler.go`), identificado pelo SecBrain e confirmado pelo BackBrain como pré-existente e fora deste card.

**Responsável:** 🤍 MegaBrain

---

## Bugfix: "Failed to fetch" (unhandledRejection) no logout — 2026-09-22
**Agente:** 🟢 FrontBrain (delegado por 🤍 MegaBrain)

**Descrição:** Usuário reportou `TypeError: Failed to fetch` não tratado ao clicar em "Sair" (`handleLogout` em `Navbar.tsx` → `logout()` em `frontend/src/lib/auth.ts`). O snippet do console apontava para a linha errada (artefato de sourcemap do hot-reload, na verdade era `getUser()`).

**Causa raiz:** `logout()` fazia `fetch(POST /api/auth/logout, credentials: "include").finally(() => redirect)` sem `.catch`. Quando a chamada ao backend falha por erro de rede genuíno (indisponibilidade, timeout, CORS, etc.), o `fetch` rejeita e, sem handler, gera `unhandledRejection` — o redirect ainda acontecia via `finally`, então o usuário não ficava travado, mas o erro poluía o console.

**Correção:** `logout()` agora é `async`, com `await fetch(...)` dentro de `try/catch` (falha de rede é ignorada silenciosamente) e `finally` garantindo o redirect para `/login` em qualquer cenário. `clearTokens()` (limpeza do localStorage) já rodava antes do fetch, então a sessão local sempre é limpa. `Navbar.tsx` ajustado para `void logout()` (único caller). Validado com `tsc --noEmit`, sem erros.

**Confirmado por 🤍 MegaBrain:** o endpoint `POST /api/auth/logout` já existe no backend (`apis/rotaperfumes-api/routes/routes.go` / `handlers/auth_handler.go`), então não há lacuna de backend — a causa era puramente falta de tratamento de erro no frontend.

**Responsável:** 🤍 MegaBrain

---

## Bugfix: warning Cloudflare Turnstile "Cannot find Widget" — 2026-09-22
**Agente:** 🟢 FrontBrain (delegado por 🤍 MegaBrain)

**Descrição:** Usuário reportou no console do browser: `[Cloudflare Turnstile] Cannot find Widget cf-chl-widget-krygo, consider using turnstile.remove() to clean up a widget.`

**Causa raiz:** em `frontend/src/components/ui/Turnstile.tsx`, o `useEffect` que renderiza o widget (`window.turnstile.render(...)`) não tinha função de cleanup, e `renderWidget()` limpava o DOM manualmente (`containerRef.current.innerHTML = ""`) sem antes chamar `window.turnstile.remove(widgetId)`. Isso deixa o script do Turnstile com referência interna "órfã" a um widgetId cujo elemento DOM já foi apagado — reproduz facilmente em dev com React Strict Mode (dupla execução do efeito).

**Correção:** adicionada função `removeWidget()` que chama `window.turnstile.remove(widgetIdRef.current)` e limpa a ref; `renderWidget()` chama `removeWidget()` antes de renderizar nova instância (evita duplicidade no mesmo container); o `useEffect` retorna cleanup chamando `removeWidget()` no unmount/re-execução. Validado com `tsc --noEmit` (sem erros). Nenhuma mudança de contrato de API — sem impacto em Postman/backend.

**Responsável:** 🤍 MegaBrain

---

## Controle de Estoque — 2026-09-22
**Agentes:** 🟣 SecBrain → 🌸 DataBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Descrição:** Nova feature de controle de estoque, baseada em `dados/erp/estoque.csv`: série temporal de snapshots diários de saldo por SKU, com CRUD igual às demais páginas, baixa automática de saldo ao faturar um pedido, e tela dedicada.

**Entregue:**
- **Banco (`sql/17_ddl_estoque.sql`):** tabela `estoque` (`id, data_snapshot DATE, sku FK->produtos.sku, saldo INT, ruptura TINYINT(1) derivado de saldo<=0, origem ENUM('import_csv','faturamento','manual'), created_at, updated_at`), `UNIQUE (data_snapshot, sku)`.
- **Importador:** `apis/shared/cmd/importestoque` (`make db-import-estoque`), upsert idempotente por `(data_snapshot, sku)` a partir de `dados/erp/estoque.csv`, `origem=import_csv`.
- **API REST** (`/api/estoque`, JWT obrigatório): `GET /api/estoque` e `GET /api/estoque/{id}` — **acesso comum**; `POST /api/estoque` e `PUT /api/estoque/{id}` — **admin only**. `GET /api/estoque` sem `data_de`/`data_ate` retorna a última posição por SKU; com o intervalo, retorna o último movimento do período por SKU; `historico=true` traz a série completa.
- **Hook de faturamento:** `PUT /api/pedidos/{id}` com transição de status para "Faturado" decrementa automaticamente o saldo de cada item do pedido (dentro da mesma transação de `UpdateComItens`, com `SELECT ... FOR UPDATE`), `origem=faturamento`. Idempotente; bloqueia edição de itens de pedido já faturado (HTTP 409).
- **Frontend:** `frontend/src/app/admin/estoque/page.tsx` + `components/admin/EstoqueModal.tsx` — listagem com busca por SKU, filtro por data e ruptura, sort, paginação, modal de criar/editar restrito a admins (usuários comuns só visualizam).
- **Testes:** cobertura completa (repository/service/handler/importador + hook de faturamento), incluindo correção dos testes existentes de pedido quebrados pela nova trava transacional. `go test ./...` 100% verde.
- **Documentação (🔵 SubBrain):** `postman/collection.json` — nova pasta "Estoque" com os 4 endpoints (listar, detalhe, criar, editar), exemplos de request/response e nota de acesso comum (GET) vs. admin only (POST/PUT). `postman/README.md` — nova seção "Estoque (`/api/estoque/*`)" em Endpoints, seção "Importação de estoque (ERP)" em "Subindo o ambiente", testes automatizados e tabela de resumo atualizados.

**Nota importante (decisão de negócio, não bug):** a baixa de estoque no faturamento usa a data/hora atual do servidor (`time.Now()`) como `data_snapshot`, e não a data de criação do pedido — o snapshot reflete o dia em que o faturamento efetivamente ocorreu. Identificado pelo 🔴 TestBrain e registrado conscientemente.

**Responsável:** 🤍 MegaBrain

---

## CAPTCHA (Cloudflare Turnstile) em Login e Troca de Senha — 2026-09-22
**Agentes:** 🟣 SecBrain (revisão) → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain → documentação e fechamento por 🔵 SubBrain

**Descrição:** Adicionada proteção CAPTCHA via Cloudflare Turnstile em `POST /api/auth/login` e `POST /api/auth/reset-password`, exigindo o campo `captchaToken` (string, obrigatório) no body de ambas as requisições.

**Camadas:**
- [x] 🟣 SecBrain — revisão concluída. Requisitos definidos: validação sempre server-side via `siteverify`, fail-closed, timeout curto (3-5s), nunca logar token/secret, Turnstile não substitui rate limiting existente, enviar `remoteip`, token de uso único. Lacuna extra identificada: `/api/auth/reset-password` não tinha rate limiting — tarefa complementar passada ao BackBrain.
- [x] 🟡 BackBrain — implementado. Campo `captchaToken` em `LoginRequest`/`ResetPasswordRequest` (`apis/rotaperfumes-api/handlers/auth_handler.go`). Novo `apis/shared/services/turnstile_service.go` (`TurnstileService`/`CaptchaVerifier`), validando o token contra `https://challenges.cloudflare.com/turnstile/v0/siteverify`, fail-closed, timeout 4s, sem logs de token/secret. Rate limiting adicionado a `/api/auth/reset-password` (10 falhas/5min, bloqueio de 10min) — lacuna identificada pelo SecBrain. Nova env var `TURNSTILE_SECRET_KEY`. Build/vet OK.
- [x] 🟢 FrontBrain — implementado. Novo componente `frontend/src/components/ui/Turnstile.tsx`, widget carregado via `next/script`, integrado em `frontend/src/app/login/page.tsx` e `frontend/src/app/trocar-senha/page.tsx`. Campo `captchaToken` já bate com o contrato do BackBrain (`frontend/src/lib/types.ts`, `frontend/src/lib/api.ts`). Nova env var `NEXT_PUBLIC_TURNSTILE_SITE_KEY`. Build/typecheck OK.
- [x] 🔴 TestBrain — concluído. Novos testes em `apis/shared/services/turnstile_service_test.go` (fail-closed, timeout, JSON malformado etc.) e `apis/rotaperfumes-api/handlers/auth_handler_test.go` (captcha ausente/inválido/válido). `go test ./...` OK em ambos os módulos, sem falhas. Cobertura do pacote `handlers` 77,8% (gaps pré-existentes em `Refresh`/`Logout`, fora do escopo).
- [x] 🔵 SubBrain — documentação concluída, ver detalhes abaixo. Card movido para `feito.md`.

**Documentação (SubBrain):**
- `.env.example` (raiz do projeto) — adicionada `TURNSTILE_SECRET_KEY` com comentário explicando onde obtê-la (dashboard Cloudflare Turnstile) e a natureza fail-closed da validação.
- `postman/collection.json` — requests **Login** e **Reset Password (própria senha)**: body principal e todos os exemplos de resposta salvos (200/401/400) atualizados incluindo `"captchaToken": "TESTING_TOKEN"`; descrições atualizadas explicando o campo, a validação server-side fail-closed, e o novo rate limiting de `/api/auth/reset-password`.
- `postman/README.md` — seção "Autenticação (`/api/auth/*`)": `POST /api/auth/login` e `POST /api/auth/reset-password` atualizados com o campo `captchaToken` no body e nota de CAPTCHA. Nova seção "CAPTCHA (Cloudflare Turnstile) em Login e Troca de Senha" em "Subindo o ambiente", seguindo o mesmo padrão da seção "Envio de email (senha inicial / reset de senha)" já existente — explica onde obter as chaves no dashboard Cloudflare, como preencher `TURNSTILE_SECRET_KEY` (backend) e `NEXT_PUBLIC_TURNSTILE_SITE_KEY` (frontend, já documentada em `frontend/.env.local.example` pelo FrontBrain), e o novo rate limiting do reset-password.

**Responsável:** 🤍 MegaBrain

---

## Auditoria de Segurança (Site + API) e Correção de Falhas — 2026-09-17
**Agentes:** 🟣 SecBrain (auditoria) → 🟡 BackBrain, 🌸 DataBrain, 🟢 FrontBrain (correções por camada) → 🔴 TestBrain (validação) → documentação e fechamento por 🔵 SubBrain

**Descrição:** Auditoria de segurança completa do projeto (API Go `apis/rotaperfumes-api`/`apis/shared`, Frontend Next.js `frontend/`, Banco de Dados `sql/`/`dados/`, segredos no repositório), liderada por 🟣 SecBrain. Relatório completo em `FalhasEncontradas.md`. Identificados 13 itens (1 crítico, 1 alto, 4 médios, 6 baixos, 1 recomendação de processo).

**Resultado:** todos os 12 itens de falha real (1-12) corrigidos e validados. Item 13 (dependências) é recomendação de processo, sem ação de código — mantido como "a validar" continuamente.

**Correções por camada:**
- **Segredos no repositório:** `itenslocais.txt` (credenciais texto puro de `gerente_admin`/`rh_admin`) e `.env` (JWT_SECRET, senha de app Gmail) removidos de todo o histórico do git via `git filter-branch`; `.gitignore` reforçado; `JWT_SECRET` regenerado; force-push realizado.
- **🟡 BackBrain (API Go):** rate limiting/anti-bruteforce no login (`middleware/ratelimit_middleware.go`); headers de segurança HTTP — CSP, HSTS, X-Content-Type-Options, X-Frame-Options, Referrer-Policy (`middleware/security_headers_middleware.go`); CORS restrito (sem prefix-match genérico de qualquer porta localhost); `getClientIP` não confia mais cegamente em `X-Forwarded-For`/`X-Real-IP`; log de login não expõe mais e-mail em texto claro; `senha_hash_anterior` removido da resposta da API de auditoria; `seedusers` não imprime mais senha em stdout/argv; escopo por vendedor aplicado a `/api/clientes`, `/api/pedidos`, `/api/pagamentos`, `/api/vendedores/{id}/clientes` (filtro server-side, nunca a partir de parâmetro do cliente — `handlers/scope.go`).
- **🌸 DataBrain:** seed de vendedores (`sql/03_seed_vendedores.sql`) ajustado para forçar troca de senha (`deve_trocar_senha = 1`) em vez de manter senha padrão idêntica sem rotação obrigatória.
- **🟢 FrontBrain:** `ProtectedRoute.tsx` passou a revalidar `role`/sessão contra `/api/auth/me` em vez de confiar apenas em `localStorage`.
- **🔴 TestBrain:** validou as correções (novos testes de rate limit, security headers, CORS, `cors_middleware_test.go`, `ratelimit_middleware_test.go`, `security_headers_middleware_test.go`, além de ajustes nos testes existentes de handlers/repositories impactados pelo escopo por vendedor).
- **🔵 SubBrain:** revisão final de `FalhasEncontradas.md` (tabela-resumo e priorização coerentes com o estado final — itens 1-12 "Corrigido", item 13 mantido como recomendação/"a validar") e adicionada a seção "Pendências Operacionais (fora do escopo dos agentes)"; Kanban fechado.

**Pendências operacionais registradas em `FalhasEncontradas.md` (fora do escopo dos agentes, dependem do usuário):**
1. Rotacionar senhas das contas `gerente_admin`/`rh_admin` no banco (expostas em texto puro no GitHub até a limpeza do histórico).
2. Gerar nova senha de app Gmail (myaccount.google.com/apppasswords) e preencher `SMTP_USER`/`SMTP_PASSWORD` no `.env` local (a anterior foi destruída na limpeza do histórico e não pôde ser recuperada).
3. Verificar/tratar forks do repositório GitHub (`residentivo/RotaPerfume`), se existirem — ainda conteriam o histórico antigo com os segredos.
4. Configurar suíte de testes automatizados no frontend (sem Jest/Testing Library) — a mudança em `ProtectedRoute.tsx` ficou sem teste automatizado; registrado como item de backlog.
5. Adotar rotina periódica de `npm audit`/`go list -m -u` (item 13) — recomendação de processo, sem ação imediata.

**Nota:** nenhuma rota nova foi criada nesta auditoria (apenas comportamento de rotas existentes mudou), portanto `postman/collection.json` e `postman/README.md` não precisaram de alteração.

**Responsável:** 🤍 MegaBrain

---

## Ranking de vendedores deve ordenar por meta e desempatar por atingimento — 2026-09-17
**Agentes:** 🟡 BackBrain → 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** `GetVendedoresRanking` (dashboard_repository.go) ordenava só por `meta_mensal DESC` na query SQL e computava o `atingimento_meta` depois em Go (pós-paginação), sem desempate real. Pedido do usuário: ordenar por meta (maior primeiro) e usar atingimento como critério de desempate, resolvido no nível do banco para funcionar corretamente com paginação.

**Subtarefas:**
- [x] 🟡 BackBrain: reescrita `GetVendedoresRanking` (`apis/shared/repositories/dashboard_repository.go`) para uma query única (LEFT JOIN vendedores + subquery agregada de pedidos do mês), ordenando `ORDER BY v.meta_mensal DESC, atingimento_meta DESC` no banco, antes do LIMIT/OFFSET. Removida a função auxiliar `getVendasMap` (não usada em mais nenhum lugar). Validado manualmente no banco local com dados reais.
- [x] 🔴 TestBrain: adicionado teste específico de desempate por atingimento (`TestDashboardGetVendedoresRanking_EmpateMeta_OrdenaPorAtingimento`), validando a cláusula ORDER BY via regex e a preservação da ordem. Cobertura ~80-95%, tudo passando.
- [x] 🔵 SubBrain: card movido para `feito.md`; documentação atualizada — ver detalhes abaixo.

**Nota:** 🟢 FrontBrain não foi acionado — mudança é só de ordenação no backend, não altera o contrato JSON nem exige alteração no frontend.

**Documentação (SubBrain):**
- `postman/collection.json` — request "Dashboard — Ranking Vendedores": descrição atualizada com a nota "Ordenação: `meta_mensal DESC`, com desempate por `atingimento_meta DESC` — calculada no SQL (join/subquery agregada de pedidos do mês), antes do LIMIT/OFFSET."
- `postman/README.md` — seção "Dashboard (`/api/dashboard/*`) — admin only", item `GET /api/dashboard/vendedores`: adicionada linha "Ordenação: por `meta_mensal DESC`, com desempate por `atingimento_meta DESC` (calculado no SQL, antes da paginação)."

**Responsável:** 🤍 MegaBrain

---

## Bug: barras do gráfico "Vendas nos Últimos 30 Dias" invisíveis (CSS) — 2026-09-16
**Agente:** 🤍 MegaBrain (correção direta, pré-existente no componente `BarChart`)

**Descrição:** No componente `BarChart` (`frontend/src/app/dashboard/page.tsx`), o container das barras usava `items-end` (alinha ao final, mas dimensiona pelo conteúdo). Cada barra ficava dentro de um wrapper sem altura definida, e `height: {pct}%` sobre um pai com altura `auto` resolve para `0` (regra CSS de percentage-height). Resultado: datas do eixo X apareciam corretamente, mas nenhuma barra era renderizada, independente dos valores de vendas — bug de layout, não de dados.

**Correção:** container mudou para `items-stretch` (cada wrapper ocupa a altura total, 192px/h-48) e cada wrapper virou `flex flex-col justify-end`, empurrando a barra interna para o fundo. Validado com `tsc --noEmit` (sem erros).

---

## Bug: erro 500 (only_full_group_by) em GET /api/dashboard/vendas — 2026-09-16
**Agente:** 🤍 MegaBrain (correção direta, regressão da tarefa anterior)

**Descrição:** A correção do bug "NaN/09" trocou o SELECT de `GetVendasSeries` para `DATE_FORMAT(data_pedido, '%Y-%m-%d')`, mas o `GROUP BY` continuou como `DATE(data_pedido)` — expressão diferente da do SELECT. Com `sql_mode=only_full_group_by` (padrão do MySQL), isso gera `Error 1055`, quebrando o endpoint com 500.

**Correção:** `GROUP BY` alterado para `DATE_FORMAT(data_pedido, '%Y-%m-%d')` em `apis/shared/repositories/dashboard_repository.go`, casando com o SELECT. Validado manualmente no banco (query executa sem erro) e via `go test ./...` em `apis/shared` e `apis/rotaperfumes-api` (todos os pacotes passando).

---

## Bug: gráfico de vendas mostra "NaN/09" no eixo de datas — 2026-09-16
**Agentes:** 🟡 BackBrain → 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** A conexão MySQL usa `parseTime=true` (config.go). Colunas DATE/DATE_SUB retornadas por `DATE(data_pedido)` e `DATE_SUB(CURDATE(), INTERVAL n DAY)` em `dashboard_repository.go` (linhas ~352 e ~432) eram convertidas pelo driver para `time.Time` e, ao serem escaneadas numa string Go, viravam algo como `"2026-09-16 00:00:00 -0300 -03"` em vez de `"2026-09-16"`. O frontend fazia `dateStr.split("-")` esperando `YYYY-MM-DD`, então o "dia" virava `NaN`.

**Subtarefas:**
- [x] 🟡 BackBrain: corrigidas as 2 queries (`GetVendasSeries` e `buildFullSeries`) usando `DATE_FORMAT(..., '%Y-%m-%d')` em `apis/shared/repositories/dashboard_repository.go`, garantindo string pura `YYYY-MM-DD` independente do driver. Validado manualmente no banco local.
- [x] 🟢 FrontBrain: não precisou de alteração.
- [x] 🔴 TestBrain: adicionado teste validando formato exato `YYYY-MM-DD` via regex, cobrindo os dois caminhos (query real e `buildFullSeries`). Cobertura 80-95%, tudo passando.
- [x] 🔵 SubBrain: card movido para `feito.md`; `postman/collection.json` verificado — nenhum exemplo de resposta com data no formato errado (ex: `"2026-09-16 00:00:00 -0300 -03"`) foi encontrado, então nenhuma correção foi necessária (é detalhe de formatação interna, não muda o contrato já documentado).

**Responsável:** 🤍 MegaBrain

---

## Bug: gráfico "Vendas nos Últimos 30 Dias" não mostra nada — 2026-09-16
**Agentes:** 🟡 BackBrain (delegado por 🤍 MegaBrain) → 🔴 TestBrain → documentação por 🔵 SubBrain

**Descrição:** `GET /api/dashboard/vendas` retornava um array puro de pontos com campos `data`/`valor`/`quantidade`, enquanto o frontend (tipo `VendasSeries`) esperava um objeto `{dias, pontos: [{dia, total_vendas, total_pedidos}]}`. O mismatch de shape fazia o gráfico cair sempre no estado vazio.

**Subtarefas:**
- [x] 🟡 BackBrain: `GetVendas` corrigido para responder `{dias, pontos: [{dia, total_vendas, total_pedidos}]}`, batendo com o tipo `VendasSeries` do frontend. Corrigido também `emptySeries` para gerar datas reais (`YYYY-MM-DD`) em vez do texto `"N dias atras"`.
- [x] 🟢 FrontBrain: não precisou de alteração — o frontend já esperava esse shape.
- [x] 🔴 TestBrain: reforçados testes de repository/service/handler cobrindo o novo shape, cobertura 80-95%, suíte 100% verde.
- [x] 🔵 SubBrain: card movido para `feito.md`; `postman/collection.json` e `postman/README.md` corrigidos — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` — request "Dashboard — Vendas (série temporal)": descrição e exemplo de resposta 200 corrigidos de array puro (`data: [{data, valor, pedidos}]`) para o shape real `data: {dias, pontos: [{dia, total_vendas, total_pedidos}]}`, com nota de que dias sem venda vêm zerados (não omitidos). Script de teste (`pm.test`) atualizado para checar `jsonData.data.dias` e as propriedades `dia`/`total_vendas`/`total_pedidos` de `jsonData.data.pontos[0]`.
- `postman/README.md` — seção "Dashboard (`/api/dashboard/*`) — admin only", item `GET /api/dashboard/vendas`: adicionada linha "Resposta" documentando o shape real `{dias, pontos: [...]}`.

**Responsável:** 🤍 MegaBrain

---

## Bug: Total de Vendas exibe NaN e Ranking de Vendedores sempre 0 — 2026-09-16
**Agentes:** 🟡 BackBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** Dois bugs pré-existentes no dashboard, encontrados durante testes manuais do usuário: (1) `dashboard_service.go` retornava `total_vendas_valor` mas o frontend esperava `total_vendas`, causando NaN no card "Total de Vendas"; (2) `dashboard_repository.go` usava a coluna inexistente `id_vendedor` em 3 queries (`enrichWithVendas`, `enrichMetasWithVendas`, `getVendasMap`), quando a coluna real em `pedidos` é `vendedor_id`, fazendo as queries falharem silenciosamente e o ranking de vendedores ficar sempre zerado.

**Subtarefas:**
- [x] 🟡 BackBrain: trocado `id_vendedor` por `vendedor_id` nas 3 queries de `apis/shared/repositories/dashboard_repository.go` (`enrichWithVendas`, `enrichMetasWithVendas`, `getVendasMap`), corrigindo o ranking de vendedores sempre zerado; renomeada a chave `"total_vendas_valor"` para `"total_vendas"` no retorno de `GetMetrics` em `apis/rotaperfumes-api/services/dashboard_service.go`, corrigindo o NaN no card "Total de Vendas" (mismatch com o campo esperado pelo frontend).
- [x] 🟢 FrontBrain: não precisou de alteração — o frontend já esperava o campo `total_vendas`.
- [x] 🔴 TestBrain: mocks/asserts ajustados em `dashboard_repository_test.go`, `dashboard_service_test.go` e `dashboard_handler_test.go`. Build e testes passam.
- [x] 🔵 SubBrain: card movido para `feito.md`; `postman/collection.json` verificado — já usava `total_vendas` nos exemplos de resposta de `GET /api/dashboard/metrics`, nenhuma correção necessária.

**Responsável:** 🤍 MegaBrain

---

## Filtro "Semana" do dashboard retorna os mesmos dados que "Hoje" — 2026-09-16
**Agentes:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** O backend só suportava `periodo=today`/`month`; o frontend remapeava `week` para `today` como contorno (escondendo o problema em vez de resolvê-lo), fazendo o filtro "Semana" do dashboard exibir exatamente os mesmos dados que "Hoje".

**Subtarefas:**
- [x] 🟡 BackBrain: adicionado suporte real a `periodo="week"` (7 dias corridos, `DATE_SUB(CURDATE(), INTERVAL 6 DAY)` até hoje) em `GetVendasTotais`, `GetTotalPedidos` (`apis/shared/repositories/dashboard_repository.go`) e `CountNovosNoPeriodo` (`apis/shared/repositories/cliente_repository.go`); validação dos handlers (`apis/rotaperfumes-api/handlers/dashboard_handler.go`) passou a aceitar `"today"`/`"week"`/`"month"`.
- [x] 🟢 FrontBrain: removido o remapeamento `"week" -> "today"` em `frontend/src/app/dashboard/page.tsx` e o tipo `DashboardPeriodo` (`frontend/src/lib/types.ts`) passou a incluir `"week"`. Typecheck (`tsc --noEmit`) OK.
- [x] 🔴 TestBrain: corrigido um mock quebrado pré-existente (`GetMetaMensalTotal` faltando em um teste) e escritos testes novos cobrindo o caso `"week"` nas 3 funções alteradas do backend + teste de handler. Suíte 100% verde, cobertura 80-95%.
- [x] 🔵 SubBrain: documentação/Postman atualizada — ver detalhes abaixo — e card movido para `feito.md`.

**Documentação (SubBrain):**
- `postman/collection.json` — pasta "Dashboard", requests `GET /api/dashboard/metrics` e `GET /api/dashboard/clientes`: descrições de query param `periodo` atualizadas de `today | month` para `today | week | month`, descrições longas dos requests atualizadas explicando que `week` considera os últimos 7 dias corridos (`DATE_SUB(CURDATE(), INTERVAL 6 DAY)`), e as mensagens de erro 400 de exemplo atualizadas de `"periodo deve ser 'today' ou 'month'"` para `"periodo deve ser 'today', 'week' ou 'month'"`.
- `postman/README.md` — seção "Dashboard (`/api/dashboard/*`) — admin only": `GET /api/dashboard/metrics` e `GET /api/dashboard/clientes` com a query documentada como `?periodo=today|week|month` e nova linha explicando que `week` = últimos 7 dias corridos (não semana civil).

**Responsável:** 🤍 MegaBrain

---

## Dashboard 100% orientado a dados reais da base — 2026-09-16
**Agentes:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** O dashboard admin misturava dados reais da API com fallbacks/mocks decorativos no frontend (métricas demo, série de vendas com `Math.random`, lista fixa de 10 vendedores fictícios, metas de pedidos/clientes calculadas por multiplicador arbitrário) sempre que a resposta da API vinha vazia ou incompleta. Objetivo: tornar o dashboard 100% orientado a dados reais do banco, com estados vazios/zero honestos em vez de dados fake.

**Camadas:**
- [x] Backend (🟡 BackBrain) — `apis/shared/repositories/dashboard_repository.go`: novo `GetMetaMensalTotal` (SUM de `vendedores.meta_mensal` filtrando vendedores ativos). `apis/rotaperfumes-api/services/dashboard_service.go`: `GetMetrics` passou a compor e retornar o campo real `meta_mes` em `GET /api/dashboard/metrics`, substituindo qualquer valor calculado no frontend.
- [x] Frontend (🟢 FrontBrain) — `frontend/src/app/dashboard/page.tsx`: removidos todos os fallbacks fake (`demoMetrics`, `demoVendas` com `Math.random`, lista de 10 vendedores fictícios, metas de pedidos/clientes calculadas por multiplicador arbitrário). Passou a usar `displayMetrics`/`displayVendas` vindos diretamente da API, com estados vazios/zero honestos quando não há dado. A meta de vendas exibida agora vem do campo real `meta_mes`. Os cards de "meta de pedidos" e "meta de clientes" foram removidos por não existir fonte real desses dados no banco (decisão de não inventar métrica sem lastro).
- [x] Teste (🔴 TestBrain) — `apis/shared/repositories/dashboard_repository_test.go` e `apis/rotaperfumes-api/services/dashboard_service_test.go`: novos casos cobrindo `GetMetaMensalTotal` (sucesso, tabela vazia, tabela inexistente, erro de DB) e `GetMetrics` ajustado para o novo campo `meta_mes`. Cobertura 89-95%, suíte 100% verde. Confirmado via grep que não restou nenhum mock/fallback fake no frontend do dashboard.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` — pasta "Dashboard", request `GET /api/dashboard/metrics`: descrição e os dois exemplos de resposta 200 (mês e dia) atualizados incluindo o novo campo `meta_mes` (com nota explicando que é a soma real de `vendedores.meta_mensal` dos vendedores ativos, sem filtro por período). Script de teste (`pm.test`) atualizado para checar `jsonData.data.meta_mes`.
- `postman/README.md` — seção "Dashboard (`/api/dashboard/*`) — admin only", item `GET /api/dashboard/metrics`: descrição atualizada mencionando "meta do mês" e adicionada nota explicando a origem real do campo `meta_mes`.
- Não existe manual dedicado de "como usar as telas" nem manual de banco de dados no repositório (apenas `postman/README.md` e o `Makefile`/`make help`) — nenhum outro documento precisou de ajuste para esta tarefa.

**Responsável:** 🤍 MegaBrain

---

## Cadastro de Visitas (CRM) — 2026-09-15
**Agentes:** 🌸 DataBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** Nova tabela/tela de CRM para registro de Visitas, baseada em `dados/crm/visitas.csv` (colunas: `visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min`, 37936 linhas). Segue o mesmo padrão já aplicado ao módulo de Oportunidades (ver "Cadastro de Oportunidades (CRM)" e "Importador de Oportunidades (CRM)" logo abaixo): PK própria `visita_id BIGINT AUTO_INCREMENT`, FKs para `clientes`/`vendedores`, importador CSV dedicado, CRUD completo, tela com dropdowns em cascata Vendedor → Cliente e filtros por coluna, testes com cobertura, documentação Postman/README. Diferente de Oportunidades, foi entregue já com o importador incluído desde o início (sem necessidade de tarefa complementar) e com `data_visita` obrigatório e sem default (em Oportunidades, `data_abertura` vazio assume a data de hoje).

**Camadas:**
- [x] Database (🌸 DataBrain) — `sql/16_ddl_visitas.sql` (PK `visita_id BIGINT AUTO_INCREMENT`, FKs `cliente_id → clientes.cliente_id_origem` ON DELETE CASCADE, `vendedor_id → vendedores.id` ON DELETE RESTRICT, `resultado VARCHAR(40)` texto livre, índices cliente_id/vendedor_id/data_visita/resultado); registrada no Makefile (`db-up`, após oportunidades); importador `apis/shared/cmd/importvisitas/main.go` (target `db-import-visitas`, incluído em `db-rebuild`). Validado ponta a ponta contra o banco local: 37936 linhas importadas, 0 erros; segunda execução confirmou idempotência (0 inseridos, 37936 atualizados).
- [x] Backend (🟡 BackBrain) — `apis/shared/models/visita.go`, `apis/shared/repositories/visita_repository.go`, `apis/rotaperfumes-api/services/visita_service.go`, `apis/rotaperfumes-api/handlers/visita_handler.go`. Rotas admin-only `GET/POST /api/visitas`, `GET/PUT /api/visitas/{id}` (filtros: cliente_id, vendedor_id, resultado, data_visita_de/ate, q; order_by com whitelist id/visita_id/data_visita/duracao_min/resultado/created_at/updated_at). Reaproveita `GET /api/vendedores/{id}/clientes` já existente para o dropdown em cascata. Validações: cliente_id/vendedor_id obrigatórios e existentes, data_visita obrigatória (sem default, diferente de oportunidades), resultado obrigatório, duracao_min >= 0. `go build`/`go vet` OK.
- [x] Frontend (🟢 FrontBrain) — `frontend/src/app/admin/visitas/page.tsx` (listagem com filtros vendedor/cliente/resultado/data, paginação, ordenação, botão "Nova Visita") + `frontend/src/components/admin/VisitaModal.tsx` (dropdowns em cascata Vendedor → Cliente via `GET /api/vendedores/{id}/clientes`). Tipos em `types.ts`, chamadas em `api.ts`, item de menu "Visitas" em `admin/layout.tsx`. `tsc --noEmit` OK (exit 0).
- [x] Teste (🔴 TestBrain) — `apis/shared/repositories/visita_repository_test.go`, `apis/rotaperfumes-api/services/visita_service_test.go`, `apis/rotaperfumes-api/handlers/visita_handler_test.go` (novos). `go test ./...` OK em `apis/shared` e `apis/rotaperfumes-api`, sem regressão. Cobertura: repositories 89.3%, services 94.9%, handlers 80.5% (funções novas em sua maioria 100%). Nenhum bug encontrado no código de produção. Nota: flake pré-existente e não relacionado em `TestValidateJWT/token_manipulado_é_rejeitado` (apis/shared/services/auth_service_test.go), fora do escopo desta tarefa.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` — nova pasta "Visitas" com os 4 endpoints (`Listar Visitas`, `Detalhe da Visita`, `Criar Visita`, `Editar Visita`), com exemplos de query params de filtro, bodies de `POST`/`PUT`, respostas de sucesso (`200`/`201`) e erro (`400`/`404`/`403`) e testes automatizados, seguindo o padrão da pasta "Oportunidades". O endpoint compartilhado `GET /api/vendedores/{id}/clientes` não foi duplicado — já documentado na pasta "Oportunidades".
- `postman/README.md` — nova seção "Visitas (`/api/visitas/*`) — admin only" em Endpoints, documentando os 4 endpoints, os filtros, e a regra de `data_visita` ser obrigatória e sem default (diferente de oportunidades), com referência ao endpoint compartilhado `GET /api/vendedores/{id}/clientes`. Seção "Testes automatizados (Postman)" e tabela "Resumo de testes por endpoint" atualizadas com as 4 novas requests. Nova seção "Visitas (CRM)" em "Subindo o ambiente" documentando `make db-up && make db-import-visitas` — já registrando que o importador foi executado com sucesso (37936 linhas, 0 erros, idempotência validada), diferente de Oportunidades que inicialmente não teve importador dedicado.

**Bugfix — 2026-09-15 (🟢 FrontBrain):** usuário reportou que faltava o link de "Visitas" na barra de navegação superior. Causa: o item de menu só havia sido adicionado ao sidebar admin (`admin/layout.tsx`), não ao `frontend/src/components/layout/Navbar.tsx` (barra superior, componente separado). Corrigido adicionando o link "Visitas" (`/admin/visitas`) ao Navbar.tsx, condicionado a `user?.role === "admin"` (mesmo padrão do link "Administração", já que Visitas é admin-only), posicionado após "Vendedores". `tsc --noEmit` OK. **Bugfix — 2026-09-15 (🟢 FrontBrain), a pedido do usuário:** a lacuna identificada acima também existia para Oportunidades — corrigida na sequência. Adicionado o link "Oportunidades" (`/admin/oportunidades`) ao Navbar.tsx, também condicionado a `user?.role === "admin"`, posicionado entre "Vendedores" e "Visitas" (mesma ordem do sidebar em `admin/layout.tsx`: Vendedores → Oportunidades → Visitas → Administração). `tsc --noEmit` OK.

**Responsável:** 🤍 MegaBrain

---

## Importador de Oportunidades (CRM) — complemento da feature — 2026-09-15
**Agentes:** 🌸 DataBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** Complemento à feature "Cadastro de Oportunidades (CRM)" (ver entrada logo abaixo), que foi entregue sem importador de dados — a tabela `oportunidades` existia criada mas vazia (documentado na entrega anterior como "não há importador dedicado nesta tarefa").

**Camadas:**
- [x] Database (🌸 DataBrain) — criado `apis/shared/cmd/importoportunidades/main.go` (upsert idempotente a partir de `dados/crm/oportunidades.csv`), seguindo o padrão de `importcarteiras`/`importpagamentos`. Adicionado target `make db-import-oportunidades` no Makefile, incluído em `db-rebuild`, posicionado após `db-import-carteiras`. Executado contra o banco local: **5979 linhas importadas, 0 erros**. Segunda execução confirmou idempotência (0 inseridos, 5979 atualizados).
- [x] Documentação (🔵 SubBrain) — `postman/README.md`, seção "Oportunidades (CRM)": atualizada para remover a nota "não há importador dedicado" e adicionar o passo `make db-up && make db-import-oportunidades`, no mesmo formato usado para Clientes/Produtos/Pedidos/Pagamentos. `postman/collection.json` verificado — não há menção a comandos `make db-import-*` nas descrições de pastas/requests da collection (o padrão de citar o importador é exclusivo do README), então nenhuma alteração foi necessária nesse arquivo.

**Responsável:** 🤍 MegaBrain

---

## Cadastro de Oportunidades (CRM) — 2026-09-15
**Agentes:** 🌸 DataBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** Nova tela de CRM para cadastro e gestão do funil de vendas (Oportunidades), baseada em `dados/crm/oportunidades.csv` (colunas: `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda`). Pedido do usuário: nova tabela `oportunidades` (PK `oportunidade_id`), tela de cadastro com dois dropdowns em cascata (Vendedor → Cliente, onde Clientes é filtrado pelo Vendedor selecionado), campos de filtro por coluna na listagem, e botão para criar nova oportunidade.

**Camadas:**
- [x] Database (🌸 DataBrain) — `sql/15_ddl_oportunidades.sql`: tabela `oportunidades`, PK `oportunidade_id BIGINT AUTO_INCREMENT`, FKs `cliente_id → clientes.cliente_id_origem` e `vendedor_id → vendedores.id`, índices para os filtros de listagem (cliente_id, vendedor_id, etapa, origem, data_abertura). Registrada no Makefile (`db-up`), aplicada após `clientes`/`vendedores`. Não validada via `make db-reset` no sandbox (sem `make`/`mysql` disponíveis) — revisão manual comparando com `14_ddl_carteiras.sql`.
- [x] Backend (🟡 BackBrain) — model/repository/service/handler em `apis/shared/models/oportunidade.go`, `apis/shared/repositories/oportunidade_repository.go`, `apis/rotaperfumes-api/services/oportunidade_service.go`, `apis/rotaperfumes-api/handlers/oportunidade_handler.go`. Rotas **admin-only**: `GET/POST /api/oportunidades`, `GET/PUT /api/oportunidades/{id}` (filtros: `cliente_id`, `vendedor_id`, `etapa`, `origem`, `data_abertura_de`/`data_abertura_ate`, `q` — busca em origem OU etapa; `order_by`/`order_dir` com whitelist: id, data_abertura, valor_estimado, probabilidade_pct, etapa, origem, created_at, updated_at). Validações de negócio: `cliente_id`/`vendedor_id` obrigatórios e devem existir, `origem`/`etapa` obrigatórios, `probabilidade_pct` entre 0-100, `valor_estimado >= 0`, `motivo_perda` obrigatório quando `etapa = "Fechado perdido"`. Endpoint novo **de acesso comum** `GET /api/vendedores/{id}/clientes` (`VendedorHandler.ListClientesDoVendedor`, mesma cadeia de middleware de Pagamentos `cfg, true, false`) — lista clientes da carteira ativa (`data_fim IS NULL`) do vendedor, usado só para alimentar o dropdown em cascata do formulário. `go build`/`go vet` OK.
- [x] Frontend (🟢 FrontBrain) — `frontend/src/app/admin/oportunidades/page.tsx` (listagem com filtros por vendedor/cliente/etapa/origem/data, paginação, botão "Nova Oportunidade") + `frontend/src/components/admin/OportunidadeModal.tsx` (dropdowns em cascata Vendedor → Cliente, carregando o segundo via `GET /api/vendedores/{id}/clientes` ao mudar o vendedor selecionado). Tipos novos em `frontend/src/lib/types.ts`, chamadas em `frontend/src/lib/api.ts`, item de menu "Oportunidades" adicionado em `frontend/src/app/admin/layout.tsx`. `tsc --noEmit` OK (revisado no ambiente do FrontBrain).
- [x] Teste (🔴 TestBrain) — `apis/shared/repositories/oportunidade_repository_test.go`, `apis/rotaperfumes-api/services/oportunidade_service_test.go`, `apis/rotaperfumes-api/handlers/oportunidade_handler_test.go` (novos); `apis/rotaperfumes-api/handlers/vendedor_handler_test.go` estendido com casos para `ListClientesDoVendedor` (sucesso com/sem clientes, vendedor inexistente, id inválido, erro interno). `go test ./...` OK em `apis/shared` e `apis/rotaperfumes-api`, sem regressão. Cobertura ≥80% nas funções novas (repositories 89%, services 94%, handlers novos 75-100%). Nenhum bug encontrado no código de produção.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Decisões de arquitetura importantes:**
- **`vendedor_id` é campo próprio da oportunidade**, informado diretamente no `POST`/`PUT` — **não depende** de a oportunidade ter um vínculo de carteira ativo entre aquele cliente e aquele vendedor em `carteiras`. Uma oportunidade pode existir mesmo que o cliente esteja hoje na carteira de outro vendedor, ou sem vínculo ativo algum. O endpoint `GET /api/vendedores/{id}/clientes` é usado **apenas para alimentar o dropdown em cascata do formulário** (sugestão de clientes prováveis por vendedor), não como restrição de integridade na criação/edição da oportunidade.
- **Acesso admin-only para o CRUD de Oportunidades** (`GET/POST /api/oportunidades`, `GET/PUT /api/oportunidades/{id}`) vs. **acesso comum** para `GET /api/vendedores/{id}/clientes` (mesmo padrão de cadeia de middleware já usado em Pagamentos: `middleware.JWTMiddleware(cfg, true, false)`) — decisão consistente com o restante do sistema, em que endpoints de suporte a formulário (dropdowns) tendem a ser mais permissivos que o CRUD principal da entidade.

**Documentação (SubBrain):**
- `postman/collection.json` — nova pasta "Oportunidades" com os 5 endpoints (`Listar Oportunidades`, `Detalhe da Oportunidade`, `Criar Oportunidade`, `Editar Oportunidade`, `Listar Clientes do Vendedor`), com exemplos de query params de filtro, bodies de `POST`/`PUT`, respostas de sucesso (`200`/`201`) e erro (`400`/`404`/`403`) e testes automatizados, seguindo o padrão da pasta "Pagamentos" (mais recente e completa). Os 4 endpoints de CRUD de Oportunidades usam `{{admin_token}}`; `Listar Clientes do Vendedor` usa `{{vendedor_token}}`, com teste explícito validando que o usuário `normal` recebe `200` e não `403` (mesmo padrão usado em Pagamentos para deixar explícito o acesso comum).
- `postman/README.md` — nova seção "Oportunidades (`/api/oportunidades/*`) — admin only, + `GET /api/vendedores/{id}/clientes` (acesso comum)" em Endpoints, documentando os 5 endpoints, a decisão de `vendedor_id` ser campo independente de vínculo de carteira, e a assimetria de acesso (admin-only vs. acesso comum). Seção "Testes automatizados (Postman)" e tabela "Resumo de testes por endpoint" atualizadas com as 5 novas requests. Nova seção "Importação de oportunidades (CRM)" em "Subindo o ambiente" — documentando que não há importador dedicado nesta tarefa (base populada apenas via `POST` manual).

**Responsável:** 🤍 MegaBrain

---

## Validação de build do frontend (resolve pendência recorrente) — 2026-09-15
**Agentes:** 🤍 MegaBrain

**Descrição:** As últimas 3 tarefas de frontend ("Promover colunas *_id_origem a PK autoincremento", "Filtros (região/UF/status) e ação ativar/inativar na tela de vendedores", "Padronizar botão de status ativo/inativo da tela de vendedores com a de usuários") ficaram com a validação de build pendente porque os agentes não encontravam `node`/`npm`/`npx` no PATH do shell de execução. Investigado a pedido do usuário: o shell usado pelas ferramentas roda com `PATH` vazio (nem `where.exe` do Windows resolve por nome) — não é falta de Node instalado, é uma restrição do ambiente de execução das ferramentas, diferente do terminal interativo do usuário (onde `make dev-frontend` funciona normalmente).

**Solução:** localizado um Node v24.12.0 completo (com npm/npx) empacotado junto ao Visual Studio em `C:\Program Files\Microsoft Visual Studio\18\Community\MSBuild\Microsoft\VisualStudio\NodeJs\`. Usando o caminho completo, rodado:
- `npx tsc --noEmit` — sem erros.
- `npm run build` (Next.js) — compilou com sucesso, todas as 13 rotas geradas, incluindo `/admin/vendedores`.

**Resultado:** resolve a pendência de validação de build registrada nos 3 cards anteriores — o frontend compila corretamente com todas as mudanças acumuladas (migração de PKs, filtros de vendedores, botão de status unificado). Validação visual manual das telas ainda é responsabilidade do usuário.

**Responsável:** 🤍 MegaBrain

---

## Padronizar botão de status ativo/inativo da tela de vendedores com a de usuários — 2026-09-15
**Agentes:** 🟢 FrontBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** A tela `admin/usuarios/page.tsx` usa um botão-pílula único na coluna "Status" que exibe o estado (bolinha verde/vermelha + "Ativo"/"Inativo") e ao ser clicado alterna o status diretamente. A tela `admin/vendedores/page.tsx` tinha uma coluna "Status" só de exibição e um botão separado "Ativar"/"Inativar" na coluna de ações. Unificado no mesmo padrão visual/interativo da tela de usuários.

**Camadas:**
- [x] Frontend (🟢 FrontBrain) — em `admin/vendedores/page.tsx`, a coluna "Status" passou a ser um botão-pílula clicável (bolinha verde/vermelha + texto "Ativo"/"Inativo"), reaproveitando os endpoints já existentes `apiDeleteVendedor`/`apiReativarVendedor` (nenhum endpoint novo foi necessário). O botão separado "Ativar"/"Inativar" que existia na coluna de ações foi removido. Mudança 100% frontend, sem impacto em contrato de API.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` e `postman/README.md` — não alterados nesta tarefa: mudança puramente visual/de interação na UI, sem alteração de endpoint, payload, resposta ou contrato de API (os endpoints `DELETE /api/vendedores/{id}` e `POST /api/vendedores/{id}/reativar` já estavam documentados desde a tarefa "Filtros (região/UF/status) e ação ativar/inativar na tela de vendedores").

**Nota pendente — ação manual do usuário (acumulada, 3ª vez consecutiva em tarefas de frontend):** não foi possível rodar `npm run build`/`tsc --noEmit` neste ambiente (Node não instalado) nesta nem nas duas tarefas de frontend anteriores ("Listar vendedores inativos..." e "Filtros (região/UF/status) e ação ativar/inativar..."). Recomenda-se, antes de considerar o conjunto dessas mudanças de frontend 100% fechado: (1) rodar `npm run build` localmente no frontend; (2) validar visualmente `/admin/vendedores` — o novo botão-pílula de status alternando corretamente entre Ativo/Inativo ao clicar, e a ausência de regressão nos filtros de Região/UF/Status já existentes na tela.

**Responsável:** 🤍 MegaBrain

---

## Filtros (região/UF/status) e ação ativar/inativar na tela de vendedores — 2026-09-15
**Agentes:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** A tela `admin/vendedores/page.tsx` tinha apenas uma busca livre por texto; adicionados filtros dedicados por região, UF e status (ativo/inativo), e a ação da lista passou a alternar entre "Inativar"/"Ativar" conforme o status do vendedor. O backend só expunha soft-delete (`DELETE /api/vendedores/{id}` → seta `data_desligamento`); não existia endpoint para reverter isso, embora o repositório já suportasse (`VendedorRepository.SetDataDesligamento` passando `nil`).

**Camadas:**
- [x] Backend (🟡 BackBrain) — novo endpoint `POST /api/vendedores/{id}/reativar` (handler `VendedorHandler.ReativarVendedor`, service `VendedorService.ReativarVendedor`), limpa `data_desligamento` reaproveitando `VendedorRepository.SetDataDesligamento`. Retorna 200 com o vendedor atualizado (mesmo formato de `DeleteVendedor`), 404 se não existir. Helper privado `setDataDesligamento` compartilhado entre `DeleteVendedor` e `ReativarVendedor`.
- [x] Frontend (🟢 FrontBrain) — adicionados selects de Região, UF (opções derivadas dinamicamente da lista carregada) e Status (Todos/Ativo/Inativo) em `admin/vendedores/page.tsx`, combinando (AND) com a busca livre já existente dentro do mesmo `useMemo` `filteredSorted`; página reseta para 1 ao mudar qualquer filtro. Botão de ação dinâmico: "Inativar" (`variant="danger"`, `handleDelete`) quando ativo, "Ativar" (`variant="primary"`, nova função `handleReativar` usando nova `apiReativarVendedor` em `frontend/src/lib/api.ts`) quando `data_desligamento` preenchido; estado de loading unificado em `togglingId` (renomeado de `deletingId`). Nota de rodapé da página atualizada mencionando os novos filtros.
- [x] Teste (🔴 TestBrain) — baseline `go build ./...`/`go test ./...` OK antes da mudança. Adicionados testes para o fluxo de reativação, espelhando a cobertura de `DeleteVendedor`: service (`vendedor_service_test.go`) `TestVendedorService_ReativarVendedor_Sucesso/_NaoEncontrado/_ErroGenericoDoRepo/_ErroNoGetByIDApósUpdate`; handler (`vendedor_handler_test.go`) `TestReativarVendedor_Success/_IDInvalido/_NaoEncontrado/_ErroInterno`. Helper `setDataDesligamento` não recebeu teste dedicado (já exercitado por `DeleteVendedor` e `ReativarVendedor`). Suíte completa 100% verde; cobertura `apis/rotaperfumes-api/services` = 95.9%, `apis/rotaperfumes-api/handlers` = 79.0%, `apis/shared/repositories` = 88.8%. Nenhum bug de código encontrado.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` e `postman/README.md` — verificados novamente: confirmado o gap já registrado nas tarefas "Promover colunas *_id_origem a PK autoincremento" e "Listar vendedores inativos com marcador [X]" — **não existe pasta "Vendedores" na collection** (só há "Login — Vendedor (primeiro acesso)" e "Dashboard — Ranking Vendedores", que são endpoints diferentes) nem seção "Vendedor" de CRUD no README (só há as seções "Vendedor (42 vendedores...)" com dados de seed e "Login — Vendedor"). Como não existe base de request/response de vendedor para seguir o padrão, não foi adicionado o novo `POST /api/vendedores/{id}/reativar` isoladamente (ficaria deslocado sem a pasta completa) — mantendo a decisão já tomada nas duas tarefas anteriores de tratar isso como item de escopo futuro. Nenhuma alteração feita em `postman/collection.json` nem `postman/README.md` nesta tarefa.
- Reforça-se a recomendação (3ª vez registrada) de, em uma tarefa futura dedicada, criar a pasta "Vendedores" na collection cobrindo `GET/POST/PUT/DELETE /api/vendedores(/{id})`, `POST /api/vendedores/{id}/reativar`, `POST/DELETE /api/vendedores/{id}/clientes(/{clienteId})` e a seção correspondente no README.

**Nota pendente — ação manual do usuário:** não foi possível rodar `npm run build`/`tsc --noEmit` em nenhuma etapa desta tarefa (Node não instalado no ambiente de execução). Recomenda-se, antes de considerar a tarefa 100% fechada: (1) rodar `npm run build` localmente no frontend; (2) testar manualmente na tela `/admin/vendedores` os três novos filtros (Região, UF, Status) combinados com a busca livre, e o botão de ação alternando corretamente entre "Inativar"/"Ativar" para vendedores ativos e inativos.

**Responsável:** 🤍 MegaBrain

---

## Listar vendedores inativos com marcador [X] (corrige bug de vínculo "sumido") — 2026-09-15
**Agentes:** 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** `GET /api/vendedores` filtrava `WHERE data_desligamento IS NULL` e não retornava esse campo. Selects que dependem dessa lista (usuário → vendedor em `UserModal.tsx`, pedido → vendedor em `PedidoModal.tsx`, e a própria listagem `admin/vendedores/page.tsx`) perdiam a opção quando o vendedor vinculado estava inativo, fazendo o campo aparecer "sem seleção" mesmo com o vínculo salvo (caso reportado: usuário Henrique Rodrigues, vinculado a um vendedor já desligado). Corrigido: `GET /api/vendedores` passa a retornar todos os vendedores (ativos e inativos) com um indicador de status; os selects/listagens no frontend marcam os inativos com `[X]`.

**Camadas:**
- [x] Backend (🟡 BackBrain) — `VendedorRepository.List` (`apis/shared/repositories/vendedor_repository.go`) não filtra mais por `data_desligamento IS NULL` e passa a retornar `data_desligamento` em cada item (novo tipo `repositories.VendedorResumo`: `id, nome, regiao, uf, data_desligamento`). `GET /api/vendedores` agora responde `data: [{id, nome, regiao, uf, data_desligamento}]` (null = ativo), ordenado por nome.
- [x] Frontend (🟢 FrontBrain) — `Vendedor` (`frontend/src/lib/types.ts`) ganhou `data_desligamento: string | null`. Selects de `UserModal.tsx` e `PedidoModal.tsx` sufixam o label com `" [X]"` quando o vendedor está inativo (sem desabilitar a opção). `admin/vendedores/page.tsx` ganhou coluna "Status" (`Ativo` / `[X] Inativo`), busca por texto passou a casar com "ativo"/"inativo", `openEdit` preenche `data_desligamento` real no fallback, e `handleDelete` agora recarrega a lista (`loadVendedores()`) em vez de remover o item otimisticamente da UI (evita o vendedor "sumir" da listagem após ser inativado).
- [x] Teste (🔴 TestBrain) — `go build ./...` e `go test ./...` OK em `apis/shared` e `apis/rotaperfumes-api`. Corrigidos `TestVendedorList_Success/_Vazio/_QueryError/_ScanError/_IterError` (`apis/shared/repositories/vendedor_repository_test.go`), `TestVendedorService_ListVendedores` (`apis/rotaperfumes-api/services/vendedor_service_test.go`) e `TestListVendedores_Success/_ListaVazia/_ErroInterno/_PermitidoParaNaoAdmin` (`apis/rotaperfumes-api/handlers/vendedor_handler_test.go`) para o novo shape sem `WHERE data_desligamento IS NULL`. Casos novos cobrindo o comportamento central da correção (ativos + inativos juntos, `DataDesligamento` populado só para inativos): `TestVendedorList_AtivosEInativos`, subteste em `TestVendedorService_ListVendedores` e `TestListVendedores_AtivosEInativos`. `vendedor_repository.go` com 100% de cobertura em todas as funções. Suíte completa passou 100%.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` e `postman/README.md` — verificados: **não existe request para `GET /api/vendedores`** na coleção nem seção correspondente no README (só há `GET /api/dashboard/vendedores`, endpoint de ranking, que é diferente e não filtra vendedores por status). Esse gap já havia sido identificado e registrado na tarefa "Promover colunas *_id_origem a PK autoincremento — 2026-09-15" (linha "a collection nunca chegou a ter uma pasta 'Vendedores'..."). Não havia, portanto, exemplo de resposta ou afirmação de "retorna apenas vendedores ativos" para corrigir. Nenhuma alteração feita nesses arquivos nesta tarefa. Fica reforçada aqui a recomendação de, em uma futura tarefa, criar a pasta "Vendedores" na collection cobrindo `GET/POST/PUT/DELETE /api/vendedores(/{id})` e os vínculos de carteira, já documentando o novo shape com `data_desligamento`.

**Nota pendente — ação manual do usuário:** o FrontBrain não conseguiu rodar `npm run build` / `npx tsc --noEmit` neste ambiente (Node não disponível no sandbox). Recomenda-se validar visualmente as telas afetadas (`admin/vendedores`, `UserModal`, `PedidoModal`) e rodar `npm run build` localmente antes de considerar esta tarefa 100% fechada.

**Responsável:** 🤍 MegaBrain

---

## Promover colunas *_id_origem a PK autoincremento — 2026-09-15
**Agentes:** 🌸 DataBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** As colunas auxiliares `pedido_id_origem` (pedidos), `cliente_id_origem` (clientes), `item_id_origem` (itens_pedido) e `carteira_id_origem` (carteiras) deixaram de ser colunas auxiliares e passaram a ser a própria PK `BIGINT AUTO_INCREMENT` de cada tabela, substituindo a antiga `id` interna — seguindo o mesmo padrão já usado em `pagamentos` (PK = `pagamento_id`). A geração manual "MAX+1" (`NextPedidoIDOrigem`, `NextClienteIDOrigem`, `nextItemIDOrigemTx`, `NextCarteiraIDOrigem`) foi removida, eliminando o débito técnico de risco de colisão em criações concorrentes registrado na tarefa "[Clientes — Criação e Edição] — 2026-09-13".

**Camadas:**
- [x] Database (🌸 DataBrain) — em `sql/04_ddl_pedidos.sql`, `sql/09_ddl_clientes.sql`, `sql/11_ddl_itens_pedido.sql`, `sql/14_ddl_carteiras.sql`: removida a antiga PK `id BIGINT AUTO_INCREMENT` de `pedidos`, `clientes`, `itens_pedido`, `carteiras`; as colunas `pedido_id_origem`, `cliente_id_origem` (padronizada de `INT` para `BIGINT`), `item_id_origem` e `carteira_id_origem` passaram a ser `BIGINT AUTO_INCREMENT PRIMARY KEY`. FKs atualizadas: `itens_pedido.pedido_id -> pedidos.pedido_id_origem`, `pagamentos.pedido_id -> pedidos.pedido_id_origem`, `pedidos.cliente_id -> clientes.cliente_id_origem`, `carteiras.cliente_id -> clientes.cliente_id_origem`. A UNIQUE composta de `carteiras` (`cliente_id`, `vendedor_id`, `data_inicio`) foi preservada. `vendedores` e `produtos` não foram alterados (mantêm `id` como PK própria).
- [x] Backend (🟡 BackBrain) — campo `ID` removido dos models `Pedido`, `Cliente`, `ItemPedido`, `Carteira` (`apis/shared/models/`). Removida a geração manual "MAX+1"; o ID agora é gerado nativamente pelo MySQL via `AUTO_INCREMENT` e capturado via `LastInsertId()` (`apis/shared/repositories/`). As rotas HTTP (`/api/pedidos/{id}`, `/api/clientes/{id}` etc.) mantiveram a mesma interface externa (path param continua `{id}`, agora mapeado para a nova PK). Os importadores (`importpedidos`, `importclientes`, `importcarteiras`, `importpagamentos`) continuam inserindo o valor do CSV explicitamente na coluna `AUTO_INCREMENT`, preservando compatibilidade com dados legados. A struct auxiliar `ClienteResumo` (resposta de `GET /api/vendedores/{id}`) manteve os nomes de campo antigos (`id`, `carteira_id`) por decisão de compatibilidade — não afetada externamente por esta tarefa.
- [x] Frontend (🟢 FrontBrain) — tipos `Pedido`, `Cliente`, `ItemPedido` em `frontend/src/lib/types.ts` perderam o campo `id` (restando `pedido_id_origem`/`cliente_id_origem`/`item_id_origem`). Páginas e modais ajustados: `frontend/src/app/admin/pedidos/page.tsx`, `frontend/src/app/admin/clientes/page.tsx`, `frontend/src/components/admin/PedidoModal.tsx`, `frontend/src/components/admin/VendedorModal.tsx`. Build e typecheck do frontend passaram limpos.
- [x] Teste (🔴 TestBrain) — toda a suíte Go (`apis/shared`, `apis/rotaperfumes-api`) foi corrigida e passa 100%, com cobertura mantida (88-100% conforme pacote). Não há testes de frontend configurados no projeto.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` — pastas "Clientes" e "Pedidos": removido o campo `"id"` de todos os exemplos de resposta JSON (`Listar`/`Criar`/`Detalhe`/`Editar`/`Ativar-Inativar Cliente`; `Listar`/`Criar`/`Detalhe`/`Editar Pedido`, inclusive nos itens aninhados `itens[]`), mantendo `cliente_id_origem`/`pedido_id_origem`/`item_id_origem` como identificador. Scripts de teste (`pm.test`) que checavam `jsonData.data.id` ou `.to.have.property('id')` ajustados para checar apenas os campos `*_id_origem`. Descrições dos requests atualizadas para deixar claro que o path param `{id}` da URL agora corresponde à PK `*_id_origem` (não a uma coluna `id` separada, que não existe mais) e que `order_by=id` continua aceito pela API como alias de coluna mapeado para `*_id_origem`.
- `postman/README.md` — seções "Clientes" e "Pedidos" atualizadas (`GET/POST/PUT /api/clientes(/{id})`, `GET/POST/PUT /api/pedidos(/{id})`, tabela de ordenação e "Resumo de testes por endpoint") removendo menções a campo `id` de resposta e ao antigo débito técnico "MAX+1" (marcado como resolvido nesta tarefa).
- Não havia coleção Postman nem documentação dedicada para `itens_pedido` isoladamente (sempre aninhado em `pedidos`) nem para `carteiras`/vínculo de cliente-vendedor — a collection nunca chegou a ter uma pasta "Vendedores" com os endpoints `POST/DELETE /api/vendedores/{id}/clientes(/{clienteId})` (gap pré-existente, não introduzido por esta tarefa). Registrado aqui para eventual priorização futura, se desejado.
- Não existe manual central do projeto (raiz/`docs/`) além do `Makefile` (`make help`) e do `postman/README.md` — nenhum documento novo foi criado além do estritamente necessário.

**Responsável:** 🤍 MegaBrain

---

## Vincular/Desvincular Cliente a Vendedor (Carteiras) — 2026-09-14
**Agentes:** 🟡 BackBrain + 🟢 FrontBrain + 🔴 TestBrain (delegado por 🤍 MegaBrain)

**Descrição:** A tela de Vendedor já listava os clientes vinculados (carteira ativa), mas não existia forma de incluir (ou encerrar) um vínculo cliente-vendedor pela UI. Adicionados endpoints de vínculo e ação na tela de Vendedor para selecionar um cliente e associá-lo à carteira do vendedor.

**Camadas:**
- [x] Backend (🟡 BackBrain): endpoint para criar vínculo (`POST /api/vendedores/{id}/clientes`) usando `CarteiraRepository.Create`, e endpoint para encerrar vínculo (`DELETE /api/vendedores/{id}/clientes/{clienteId}` usando `EncerrarVinculo`). Valida cliente/vendedor existentes (`VendedorRepository.ExistsByID`, `ClienteRepository.GetByID`/`ExistsByID` novos); se o cliente já tiver carteira ativa com outro vendedor, transfere automaticamente (encerra o vínculo anterior e cria o novo). Novos `CarteiraRepository.GetVinculoAtivoByClienteID`/`GetVinculoAtivo`, `VendedorService.VincularCliente`/`DesvincularCliente`, erro `ErrVinculoNaoEncontrado`. Concluído em `apis/shared/repositories/carteira_repository.go`, `apis/shared/repositories/cliente_repository.go`, `apis/rotaperfumes-api/services/vendedor_service.go`, `apis/rotaperfumes-api/handlers/vendedor_handler.go`, `apis/rotaperfumes-api/routes/routes.go`.
- [x] Frontend (🟢 FrontBrain): no `VendedorModal.tsx`, adicionado seletor de cliente (combobox, reaproveitando o padrão de carregamento de clientes do `PedidoModal.tsx` via `apiListClientes`) + botão "Adicionar" para vincular (`apiVincularCliente`), e ação "Remover" por linha na tabela de clientes vinculados (`apiDesvincularCliente`, com confirmação via `window.confirm`, seguindo o padrão de `vendedores/page.tsx`).
- [x] Teste (🔴 TestBrain): `apis/shared/repositories/carteira_repository_test.go` estendido com `GetVinculoAtivoByClienteID`/`GetVinculoAtivo` (encontrado/não encontrado/erro de query, 6 novos casos). `apis/shared/repositories/cliente_repository_test.go` estendido com `ExistsByID` (existe/não existe/erro, 3 novos casos parametrizados + 1 dedicado). `apis/rotaperfumes-api/services/vendedor_service_test.go` estendido com `VincularCliente` (7 casos: sucesso simples, transferência automática, idempotente, vendedor inexistente, cliente inexistente, 2 erros de repository) e `DesvincularCliente` (5 casos: sucesso, vínculo não encontrado, vendedor/cliente inexistente, 2 erros de repository). `apis/rotaperfumes-api/handlers/vendedor_handler_test.go` estendido com `VincularCliente` (201, 400 body inválido parametrizado, 404 vendedor/cliente, id inválido) e `DesvincularCliente` (200/204 sucesso, 404 vínculo não encontrado, 400 id inválido parametrizado). `go build ./...`, `go vet ./...` e `go test ./... -cover` passaram em `apis/shared` e `apis/rotaperfumes-api` (repositories 89.2%, services 95.9%, handlers 78.5%). Cobertura das funções novas: `VincularCliente`/`DesvincularCliente` (service) 90.0%/91.7%, `clienteResumoDoVinculo` 100%, `VincularCliente`/`DesvincularCliente` (handler) 86.4%/83.3%, `GetVinculoAtivoByClienteID`/`GetVinculoAtivo`/`ExistsByID` (repository) 100% — todas ≥80%. Nenhum bug encontrado no código de produção desta tarefa. **Pendência não relacionada:** `TestValidateJWT/token_manipulado_é_rejeitado` em `apis/shared/services/auth_service_test.go:170` está falhando (teste pré-existente, não tocado nesta tarefa) — encaminhado ao Analista para investigação.

**Decisão de design — transferência automática de carteira:** `VincularCliente` verifica se o cliente já possui um vínculo ativo (`GetVinculoAtivoByClienteID`) antes de criar o novo. Se o vínculo ativo já for com o mesmo vendedor, a operação é idempotente (retorna o vínculo existente sem novo INSERT). Se for com outro vendedor, o vínculo anterior é encerrado (`EncerrarVinculo`, seta `data_fim = agora`) e um novo vínculo é criado para o vendedor informado — preservando o histórico completo via múltiplas linhas em `carteiras`, sem exigir uma chamada explícita de "desvincular" antes de vincular a outro vendedor.

**Bugfix — 2026-09-14 (🟢 FrontBrain):** corrigido `TypeError: Cannot read properties of null (reading 'filter')` em `VendedorModal.tsx` ao vincular o primeiro cliente de um vendedor sem carteira. Causa raiz: `apis/shared/repositories/carteira_repository.go#ListClientesByVendedorID` usava `var out []ClienteResumo` (slice nil) quando não havia clientes vinculados, serializando `"clientes": null` no JSON; o frontend assumia array e chamava `.filter()`/`.map()` diretamente. Corrigido em duas camadas: backend agora inicializa `out := []ClienteResumo{}` (sempre retorna `[]` no JSON); frontend normaliza `res.clientes ?? []` ao setar `detalhe` após o fetch e trata `prev.clientes ?? []` nos updates de `handleVincularCliente`/`handleDesvincularCliente` como defesa adicional. `go build ./...` e `go vet ./...` OK em `apis/shared` e `apis/rotaperfumes-api`.

**Bugfix — 2026-09-14 (🟡 BackBrain):** corrigido `Error 1062 (23000): Duplicate entry '...' for key 'carteiras.uk_carteiras_cliente_vendedor_inicio'` ao vincular → desvincular → vincular o mesmo cliente ao mesmo vendedor NO MESMO DIA. Causa raiz: `carteiras.data_inicio` é `DATE` (sem hora) com unique key `(cliente_id, vendedor_id, data_inicio)`; `VincularCliente` sempre tentava `INSERT` um novo registro, colidindo com a linha já existente (mesmo encerrada) do vínculo anterior daquele dia. Corrigido sem alterar schema: novo `CarteiraRepository.GetVinculoByClienteVendedorData(ctx, db, clienteID, vendedorID, dataInicio)` (retorna `ErrNotFound` se não existir) e `CarteiraRepository.ReativarVinculo(ctx, db, id)` (`UPDATE carteiras SET data_fim = NULL WHERE id = ?`). Em `VendedorService.VincularCliente`, antes do `Create`, verifica se já existe linha para `(cliente_id, vendedor_id, data_inicio=hoje)`: se ativa, idempotente (sem duplicar); se encerrada, reativa via `ReativarVinculo` em vez de inserir; se não existir, segue o fluxo normal (`Create`). `DesvincularCliente` não foi alterado. Arquivos: `apis/shared/repositories/carteira_repository.go`, `apis/rotaperfumes-api/services/vendedor_service.go`, `apis/rotaperfumes-api/services/vendedor_service_test.go`, `apis/rotaperfumes-api/handlers/vendedor_handler_test.go`. `go build ./...`, `go vet ./...` e `go test ./...` OK em `apis/shared` e `apis/rotaperfumes-api` (mocks sqlmock ajustados para a nova query de checagem do dia).

**Responsável:** 🤍 MegaBrain

---

## Cadastro de Vendedor com Lista de Clientes (Carteiras) — 2026-09-14
**Agentes:** 🌸 DataBrain + 🟡 BackBrain + 🟢 FrontBrain + 🔴 TestBrain (delegado por 🤍 MegaBrain)

**Descrição:** Criar cadastro completo de Vendedor (antes era apenas leitura via `GET /api/vendedores`, sem CRUD e sem tela própria no frontend), incluindo uma lista de clientes vinculados a cada vendedor. O vínculo cliente ↔ vendedor não existia como tabela persistida — só um CSV bruto (`dados/crm/carteira.csv`) — sendo necessário criar a tabela `carteiras` para isso. A feature seguiu o mesmo padrão de "lista aninhada" já usado em Pedidos (repository com query `ListXByParentID` separada, service compondo um DTO `*Detalhe`, handler expondo via `GET` do pai, frontend com modal carregando o array aninhado), referências:
- `apis/shared/repositories/pedido_repository.go:191` (`ListItensByPedidoID`)
- `apis/rotaperfumes-api/services/pedido_service.go:90` (`GetPedidoDetalhe`)
- `apis/rotaperfumes-api/handlers/pedido_handler.go:91` (`GetPedido`)
- `frontend/src/components/admin/PedidoModal.tsx`

**Camadas:**
- [x] Database (🌸 DataBrain): migration `carteiras` (`cliente_id`, `vendedor_id`, FKs para `clientes`/`vendedores`), model, repository básico. Concluído em `sql/14_ddl_carteiras.sql`, `apis/shared/models/carteira.go`, `apis/shared/repositories/carteira_repository.go`.
- [x] Backend (🟡 BackBrain): CRUD completo de Vendedor (`Create`/`Update`/`Delete` — antes só existia `List`) + endpoint de detalhe do vendedor com lista de clientes aninhada, seguindo o padrão de pedidos (`ListXByParentID` no repository, DTO `*Detalhe` no service, exposição via `GET` do pai no handler) + registro das novas rotas. Concluído em `apis/shared/models/vendedor.go`, `apis/shared/repositories/vendedor_repository.go`, `apis/rotaperfumes-api/services/vendedor_service.go`, `apis/rotaperfumes-api/handlers/vendedor_handler.go`, `apis/rotaperfumes-api/routes/routes.go`.
- [x] Frontend (🟢 FrontBrain): página de cadastro/edição de Vendedor em `frontend/src/app/admin/vendedores/page.tsx` (baseada no padrão de `frontend/src/app/admin/clientes/page.tsx`) + `frontend/src/components/admin/VendedorModal.tsx` listando os clientes vinculados ao vendedor (somente leitura), no padrão de `frontend/src/components/admin/PedidoModal.tsx`. Tipos novos em `frontend/src/lib/types.ts` (`VendedorCompleto`, `ClienteResumo`, `VendedorDetalhe`, `VendedorInput`) e funções em `frontend/src/lib/api.ts` (`apiGetVendedor`, `apiCreateVendedor`, `apiUpdateVendedor`, `apiDeleteVendedor`). Link "Vendedores" adicionado em `frontend/src/app/admin/layout.tsx` e `frontend/src/components/layout/Navbar.tsx`.
- [x] Teste (🔴 TestBrain): novo `apis/shared/repositories/carteira_repository_test.go` (25 testes cobrindo `ListClientesByVendedorID`/`GetByID`/`Create`/`EncerrarVinculo`/`Delete`/`NextCarteiraIDOrigem`, sucesso/erro/vazio). `apis/shared/repositories/vendedor_repository_test.go` estendido com `GetByID`/`Create`/`Update`/`SetDataDesligamento` (17 novos casos). `apis/rotaperfumes-api/services/vendedor_service_test.go` estendido com `GetVendedorDetalhe` (parametrizado, 5 casos: com clientes, sem clientes, inexistente, erro no GetByID, erro no repo de carteira) e `CreateVendedor`/`UpdateVendedor`/`DeleteVendedor` (validações + sucesso + erros, ~20 novos casos). `apis/rotaperfumes-api/handlers/vendedor_handler_test.go` estendido com `GetVendedor` (200 com/sem clientes aninhados, 404, id inválido), `CreateVendedor` (201, 400 JSON inválido, 400 validação parametrizado), `UpdateVendedor` (200, 404, 400) e `DeleteVendedor` (200, 404, id inválido) — 20 novos casos. `go test ./... -cover` passou 100% em `apis/shared` e `apis/rotaperfumes-api`; cobertura: `carteira_repository.go` 100% (exceto `scanClienteResumo` em 83.3%), `vendedor_repository.go` 100%, `vendedor_service.go` 100%, `vendedor_handler.go` 76.9%-100% por função (`GetVendedor` 76.9%, demais ≥80%). Nenhum bug encontrado no código de produção.

**Decisões de design importantes:**
- **Soft-delete de vendedor:** `DELETE /api/vendedores/{id}` não apaga a linha — grava `data_desligamento = NOW()` via `VendedorRepository.SetDataDesligamento` (`apis/shared/repositories/vendedor_repository.go`). `DataDesligamento == nil` significa vendedor ativo (`apis/shared/models/vendedor.go`). Evita quebrar histórico de pedidos/carteiras que referenciam o vendedor.
- **Histórico de vínculos em carteiras:** trocar o vendedor de um cliente não sobrescreve/apaga o vínculo antigo — `CarteiraRepository.EncerrarVinculo` apenas seta `data_fim` no registro existente (`UPDATE carteiras SET data_fim = ? WHERE id = ?`), preservando o histórico completo de qual vendedor atendeu qual cliente e por quanto tempo. `ListClientesByVendedorID` só traz os vínculos ativos (`data_fim IS NULL`), então o modal do vendedor sempre reflete a carteira atual, sem exibir vínculos encerrados.

**Correção pós-entrega — 2026-09-14 (🌸 DataBrain):** a migration `sql/14_ddl_carteiras.sql` havia sido criada mas nunca aplicada ao `make db-up`, e não existia importador para `dados/crm/carteira.csv` — causando `Table 'rotaperfumes.carteiras' doesn't exist` em bancos existentes. Corrigido: (1) `Makefile` — `db-up` agora aplica `sql/14_ddl_carteiras.sql` logo após `pagamentos` (depende de `clientes` + `vendedores` já criados); novo target `db-import-carteiras` (adicionado ao `.PHONY`); (2) novo importador `apis/shared/cmd/importcarteiras/main.go`, seguindo o padrão de `importclientes`/`importpedidos` (upsert idempotente via `carteira_id_origem`, lookup de `cliente_id_origem` → `clientes.id`, `vendedor_id` usado direto, `data_fim` opcional/NULL); (3) teste `apis/shared/cmd/importcarteiras/main_test.go`. `go build ./...`, `go vet ./...` e `go test ./apis/shared/cmd/importcarteiras/...` passaram sem erros. Bancos já existentes (sem `db-reset`) precisam rodar `mysql $(MYSQL_OPTS) rotaperfumes < sql/14_ddl_carteiras.sql` seguido de `make db-import-carteiras`.

**Responsável:** 🤍 MegaBrain

---

## [BUG] Dashboard não carrega para vendedores — 2026-09-14
**Agentes:** 🟡 BackBrain + 🟢 FrontBrain (investigação paralela, delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** Usuário reportou que ao logar como vendedor, a tela de dashboard não carregava (ficava travada em "Verificando autenticação...").

**Causa raiz (dupla):**
1. **Não é bug de permissão de API.** Por design, toda a API `/api/dashboard/*` é `admin only` (`apis/rotaperfumes-api/routes/routes.go:105-123` + `apis/rotaperfumes-api/middleware/auth_middleware.go:86-89`, `JWTMiddleware` com `requireAdmin=true`). O role "vendedor" nunca teve acesso a essas rotas — comportamento intencional.
2. **Bug real no frontend, causando loop infinito de redirecionamento:** `frontend/src/app/login/page.tsx` redirecionava TODO usuário (inclusive vendedor) para `/dashboard` após login, sem checar role. Como a página `/dashboard` exige `requireAdmin=true` via `ProtectedRoute.tsx`, e o `ProtectedRoute` antigo redirecionava usuário rejeitado de volta para a própria `/dashboard`, o vendedor entrava em loop de redirect — tela travada em "Verificando autenticação...", dando a impressão de "dashboard não carrega".

**Correção (🟢 FrontBrain):**
- `frontend/src/components/layout/ProtectedRoute.tsx` — ao rejeitar usuário não-admin, agora redireciona para `/pagamentos` (rota de acesso comum) em vez de `/dashboard`, eliminando o loop.
- `frontend/src/app/login/page.tsx` — redirect pós-login (e redirect quando já logado) passa a usar `isAdmin() ? "/dashboard" : "/pagamentos"` em vez de sempre `/dashboard`.

**Camadas:**
- [x] Backend — investigado, sem alteração (comportamento admin-only confirmado como intencional, não bug)
- [x] Frontend (🟢 FrontBrain) — correção acima
- [ ] Database (N/A)
- [ ] Teste (N/A — correção de guard de rota simples, sem novo teste automatizado dedicado)
- [x] Documentação (🔵 SubBrain) — este registro

**Possível melhoria futura (não aberta no backlog, aguardando decisão do usuário):** hoje não existe uma tela "dashboard"/home dedicada para o role vendedor — ele cai em `/pagamentos` após login. Se o usuário desejar uma dashboard própria para vendedor (ex.: métricas dos próprios pedidos/vendas), isso seria uma feature nova (Database → Backend → Frontend → Teste), a ser priorizada em `tarefas/afazer.md` somente se solicitado.

---

## Ordenação via API (server-side sort) — 2026-09-14
**Agentes:** 🟡 BackBrain + 🟢 FrontBrain + 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** A ordenação das tabelas de listagem (produtos, clientes, pedidos, pagamentos, usuários, senha-histórico) era feita apenas client-side (JS `.sort()` sobre a página atual), o que só ordenava os itens já carregados na página em vez do dataset inteiro. Passou a enviar parâmetros de ordenação (`order_by`/`order_dir`) para a API e ordenar no backend, com whitelist de colunas por entidade contra SQL injection.

**Camadas:**
- [ ] Database (N/A — sem mudança de schema)
- [x] Backend (🟡 BackBrain) — os 6 endpoints de listagem (`GET /api/produtos`, `/api/clientes`, `/api/pedidos`, `/api/pagamentos`, `/api/usuarios`, `/api/senha-historico` e `/api/senha-historico/{usuario_id}`) passaram a aceitar `order_by`/`order_dir` (`asc`/`desc`, case-insensitive), com whitelist de colunas por entidade. Valor inválido ou ausente cai silenciosamente no default de cada entidade (sem erro 400). Novo helper compartilhado `apis/shared/repositories/sort.go`. `go build ./...` OK.
- [x] Frontend (🟢 FrontBrain) — `frontend/src/lib/api.ts` e as 6 páginas de listagem (`admin/produtos`, `admin/clientes`, `admin/pedidos`, `pagamentos`, `admin/usuarios`, `admin/senha-historico`) passaram a enviar `order_by`/`order_dir` para a API em vez de ordenar localmente em JS — corrige o bug em que a ordenação client-side só ordenava os itens da página atual, não o dataset inteiro. Typecheck não pôde ser rodado no sandbox (sem Node); revisado manualmente pelo MegaBrain.
- [x] Teste (🔴 TestBrain) — ~58 subtestes novos cobrindo ordenação válida, case-insensitive, whitelist bypass/SQL injection e `order_dir` inválido, nas camadas repository/handler das 6 entidades. `go build`, `go vet` e `go test` OK em ambos os módulos.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Contrato de `order_by`/default por endpoint:**
- `GET /api/produtos`: id, sku, descricao, categoria, marca, preco_tabela, custo_unitario, data_lancamento, ativo, created_at, updated_at — default `id asc`
- `GET /api/clientes`: id, razao_social, cnpj, segmento, cidade, uf, data_cadastro, ativo, created_at, updated_at — default `id asc`
- `GET /api/pedidos`: id, data_pedido, canal, status, valor_total, created_at, updated_at, cliente_nome, vendedor_nome — default `id desc`
- `GET /api/pagamentos`: pagamento_id, pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento, created_at, updated_at — default `pagamento_id asc`
- `GET /api/usuarios`: id, nome, email, role, ativo, created_at, updated_at, ultimo_login_at — default `id asc`
- `GET /api/senha-historico` e `/{usuario_id}`: id, usuario_id, tipo_reset, created_at — default `id desc`

**Documentação (SubBrain):**
- `postman/collection.json` — nas 6 requests "Listar ..." (Usuários, Clientes, Produtos, Pedidos, Pagamentos, Histórico de Senhas — Todos e Por Usuário), adicionados os query params `order_by`/`order_dir` (desabilitados por padrão, com descrição da whitelist de colunas e default de cada entidade), e as descrições dos requests atualizadas com a nova seção de ordenação.
- `postman/README.md` — cada um dos 6 endpoints `GET` de listagem (`/api/usuarios`, `/api/clientes`, `/api/produtos`, `/api/pedidos`, `/api/pagamentos`, `/api/senha-historico` e `/{usuario_id}`) ganhou uma linha "Ordenação (`order_by`/`order_dir`, opcionais)" documentando a whitelist de colunas aceitas e o default.
- `apis/rotaperfumes-api/routes/routes.go` — comentário de documentação de rotas no topo do arquivo atualizado para citar `order_by`/`order_dir opcionais` nos 6 endpoints de listagem afetados.
- Não havia manual central do projeto (raiz/`docs/`) além do `Makefile` autoexplicativo via `make help` e do `postman/README.md` — nenhum documento novo foi criado além do estritamente necessário.

---

## [Tela de Pagamentos (CRUD + Importação CSV)] — 2026-09-14
**Agentes:** 🌸 DataBrain → 🟡 BackBrain → 🟢 FrontBrain → 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** Nova tela de cadastro e gerenciamento de Pagamentos, baseada em `dados/erp/pagamentos.csv` (colunas: `pagamento_id,pedido_id,forma_pagamento,parcelas,valor,taxa_pct,valor_liquido,data_vencimento,data_pagamento,status_pagamento`, ~27.7k linhas). Diferente das telas anteriores (Clientes/Produtos/Pedidos, todas admin only), esta é de **acesso comum** — qualquer usuário autenticado (`admin` ou `normal`) pode acessar, não só admin.

**Decisão de schema (instrução explícita do usuário):** a chave primária da tabela é `pagamento_id` (BIGINT AUTO_INCREMENT), não o padrão `id` desacoplado usado em Clientes/Produtos/Pedidos — alinhado 1:1 ao `pagamento_id` do CSV de origem, que já é sequencial e único.

**Camadas:**
- [x] Database (🌸 DataBrain) — `sql/12_ddl_pagamentos.sql` (tabela `pagamentos`, PK `pagamento_id` BIGINT AUTO_INCREMENT, ENUMs `forma_pagamento`/`status_pagamento`, FK para `pedidos`), `apis/shared/cmd/importpagamentos/main.go` (importador de `dados/erp/pagamentos.csv`, upsert idempotente por `pagamento_id`, resolve `pedido_id` via lookup em `pedidos.pedido_id_origem`), `Makefile` (`sql/12_ddl_pagamentos.sql` adicionado em `db-up`, novo target `make db-import-pagamentos`). `go build ./...` OK; importação real não executada (sem MySQL no sandbox).
- [x] Backend (🟡 BackBrain) — `apis/shared/models/pagamento.go`, `apis/shared/repositories/pagamento_repository.go` (+ `ExistsByID` adicionado em `pedido_repository.go`), `apis/rotaperfumes-api/services/pagamento_service.go`, `apis/rotaperfumes-api/handlers/pagamento_handler.go`. Rotas registradas em `routes.go`/`main.go` com **acesso comum** (`middleware.JWTMiddleware(cfg, true, false)`, qualquer usuário autenticado — NÃO admin only, diferente de Clientes/Produtos/Pedidos): `GET /api/pagamentos` (paginado, filtros `status_pagamento`/`forma_pagamento`/`pedido_id`/`vencimento_de`/`vencimento_ate`), `POST /api/pagamentos`, `GET /api/pagamentos/{id}`, `PUT /api/pagamentos/{id}` (não permite alterar `pagamento_id`/`pedido_id`). `go build ./...` e `go vet ./...` OK em `apis/shared` e `apis/rotaperfumes-api`.
- [x] Frontend (🟢 FrontBrain) — `frontend/src/app/pagamentos/page.tsx` (fora de `admin/`, envolto em `<ProtectedRoute>` sem `requireAdmin` — acesso comum), `frontend/src/components/PagamentoModal.tsx` (criar/editar), tipos/API client em `frontend/src/lib/types.ts`/`frontend/src/lib/api.ts`, link "Pagamentos" no `Navbar.tsx` visível para todo usuário autenticado (não só admin).
- [x] Teste (🔴 TestBrain) — `apis/rotaperfumes-api/services/pagamento_service_test.go`, `apis/rotaperfumes-api/handlers/pagamento_handler_test.go`, `apis/shared/repositories/pagamento_repository_test.go`. Cobertura 100% no service, 85-100% no repository, 77-100% no handler. `go build ./...`, `go vet ./...` e `go test ./... -cover` OK, nenhum bug encontrado no código de produção.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` — nova pasta "Pagamentos" com os 4 endpoints (`Listar Pagamentos`, `Criar Pagamento`, `Detalhe do Pagamento`, `Editar Pagamento`), com exemplos de query params/body, respostas de sucesso (`201`/`200`) e erro (`400`/`404`) e testes automatizados, seguindo o mesmo padrão das demais pastas (ex.: "Produtos"/"Pedidos"). **Diferença importante:** ao contrário das demais pastas (que usam `{{admin_token}}`), os requests de Pagamentos usam `{{vendedor_token}}` (usuário `normal`) nos exemplos, já que a rota aceita ambos os perfis — cada request inclui um teste explícito validando que o usuário `normal` recebe `200`/`201` e não `403`.
- `postman/README.md` — seção "Pagamentos" adicionada em Endpoints (destacando explicitamente que é **acesso comum, não admin-only**, diferente das demais telas), testes automatizados, tabela "Resumo de testes por endpoint" e nova seção "Importação de pagamentos (ERP)" em "Subindo o ambiente" documentando `make db-up && make db-import-pagamentos`.

**Nota importante — ação pendente do usuário:**
1. A importação do CSV para o banco **ainda não foi executada** em nenhum ambiente (o sandbox dos agentes não tem `mysql`/`make` disponíveis). Antes de usar a feature em um ambiente novo ou já existente, rodar manualmente:
```bash
make db-up && make db-import-pagamentos
```
2. O sandbox não tem Node disponível para rodar `npx tsc --noEmit` do frontend (`pagamentos/page.tsx`, `PagamentoModal.tsx`). Revisão manual de tipos foi feita pelo FrontBrain, mas recomenda-se rodar o typecheck localmente antes do merge definitivo.

---

## Cobertura de Testes — shared/repositories — 2026-09-14
**Agente:** 🔴 TestBrain (delegado por 🤍 MegaBrain) → validado e Kanban atualizado por 🔵 SubBrain

**Descrição:** `apis/shared/repositories` estava com 6.6% de cobertura (só `usuario_repository_test.go` existia). Pendência identificada na tarefa "Cobertura de Testes Backend — 2026-09-14" (abaixo). Escritos testes para os 7 repositórios restantes.

**Camadas:**
- [ ] Database
- [ ] Backend
- [ ] Frontend
- [x] Teste

**Arquivos de teste criados:**
- `apis/shared/repositories/`: `cliente_repository_test.go`, `dashboard_repository_test.go`, `pedido_repository_test.go`, `produto_repository_test.go`, `refresh_token_repository_test.go`, `senha_historico_repository_test.go`, `vendedor_repository_test.go`

**Resultado:** `go build ./...` e `go test ./... -cover` limpos em `apis/shared`. Cobertura do pacote `repositories` foi de 6.6% para 88.1% (149 testes, todos passando). Nenhum bug encontrado no código de produção.

**Documentação (SubBrain):** não foi necessário gerar/atualizar Postman ou manuais para esta tarefa — trata-se apenas de testes internos de backend, sem mudança de contrato de API ou de schema.

---

## Cobertura de Testes Backend — 2026-09-14
**Agente:** 🔴 TestBrain (delegado por 🤍 MegaBrain) → Kanban atualizado por 🔵 SubBrain

**Descrição:** Backend Go (apis/rotaperfumes-api e apis/shared) tinha cobertura de testes incompleta. Foram criados testes para os arquivos que ainda não possuíam.

**Camadas:**
- [ ] Database
- [x] Backend
- [ ] Frontend
- [x] Teste

**Arquivos de teste criados:**
- `apis/rotaperfumes-api/handlers/`: `dashboard_handler_test.go`, `senha_historico_handler_test.go`, `usuario_handler_test.go`, `vendedor_handler_test.go`
- `apis/rotaperfumes-api/services/`: `dashboard_service_test.go`, `refresh_token_service_test.go`, `senha_historico_service_test.go`, `usuario_service_test.go`, `vendedor_service_test.go`
- `apis/shared/services/`: `password_generator_test.go`, `email_service_test.go`

**Resultado:** `go build ./...` e `go test ./... -cover` limpos em `apis/rotaperfumes-api` e `apis/shared`. Cobertura final: handlers 74.3%, middleware 52.6%, services (rotaperfumes-api) 95.0%, shared/services 46.9% (limitado pela parte SMTP não testável sem rede — documentado em comentário no próprio arquivo de teste), shared/models 100%, shared/repositories 6.6% (repositórios não foram alvo desta tarefa; ficam para uma tarefa futura, se desejado).

**Documentação (SubBrain):** não foi necessário gerar/atualizar Postman ou manuais para esta tarefa — trata-se apenas de testes internos de backend, sem mudança de contrato de API ou de schema.

---

## Tela de Pedidos — 2026-09-13
**Agentes:** 🌸 DataBrain + 🟡 BackBrain + 🟢 FrontBrain + 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** Tela de Pedidos (master-detail): lista de pedidos, ao selecionar uma linha mostra sub-lista de itens do pedido (produtos); criação/edição de pedido na mesma tela. Dados de referência: `dados/erp/pedidos.csv` (28.729 linhas) e `dados/erp/itens_pedido.csv` (197.724 linhas). Link no menu superior.

**Camadas:**
- [x] Database (🌸 DataBrain) — `sql/04_ddl_pedidos.sql` (tabela `pedidos` alinhada ao CSV: `cliente_id_origem`, canal, status reais Cancelado/Em separação/Entregue/Faturado), `sql/05_seed_pedidos.sql`, nova tabela `sql/11_ddl_itens_pedido.sql` (`itens_pedido`).
- [x] Backend (🟡 BackBrain) — `apis/shared/models/pedido.go`, `apis/shared/models/item_pedido.go`, `apis/shared/repositories/pedido_repository.go`, importador `apis/shared/cmd/importpedidos` (upsert idempotente por `pedido_id_origem`/`item_id_origem`), `apis/rotaperfumes-api/services/pedido_service.go`, `apis/rotaperfumes-api/handlers/pedido_handler.go`. Rotas novas (todas admin only): `GET /api/pedidos` (paginado, filtros `status`/`canal`/`cliente_id`/`vendedor_id`/`data_inicio`/`data_fim`/`q`), `POST /api/pedidos` (cria pedido + itens, calcula `valor_bruto`/`valor_total`), `GET /api/pedidos/{id}` (detalhe com itens), `PUT /api/pedidos/{id}` (substitui itens). Alvo `make db-import-pedidos` já presente no `Makefile`.
- [x] Frontend (🟢 FrontBrain) — `frontend/src/app/admin/pedidos` (tela master-detail: lista de pedidos + sub-lista de itens ao selecionar linha), `frontend/src/components/admin/PedidoModal.tsx` (criação/edição com itens), link no `Navbar.tsx`.
- [x] Teste (🔴 TestBrain) — `pedido_service_test.go`, `pedido_handler_test.go`: 63 testes cobrindo validações, sucesso e erros de `ListPedidos`/`CreatePedido`/`UpdatePedido`/`GetPedidoDetalhe`, com 100% de cobertura no `pedido_service.go`.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Resumo:** schema (`pedidos` + `itens_pedido`) alinhado ao ERP, 4 endpoints REST (`GET/POST /api/pedidos`, `GET/PUT /api/pedidos/{id}`) admin only, tela master-detail completa no frontend, 63 testes com 100% de cobertura no service.

**Documentação (SubBrain):**
- `postman/collection.json` — nova pasta "Pedidos" com os 4 endpoints (`Listar Pedidos`, `Criar Pedido`, `Detalhe do Pedido`, `Editar Pedido`), com exemplos de query params/payload com itens, respostas de sucesso/erro (`201`/`200`/`400`/`403`/`404`) e testes automatizados, seguindo o mesmo padrão das demais pastas da collection (ex.: "Produtos").
- `postman/README.md` — seção "Pedidos" adicionada em Endpoints, testes automatizados, tabela "Resumo de testes por endpoint" e nova seção "Importação de pedidos (ERP)" em "Subindo o ambiente" documentando `make db-up && make db-import-pedidos`.
- `Makefile` já continha o alvo `db-import-pedidos`; nenhuma duplicação adicionada. Não havia manual central do projeto (raiz/`docs/`) além do `Makefile` autoexplicativo via `make help` e do `postman/README.md` — nenhum documento novo foi criado além do estritamente necessário.

---

## [Produtos (CRUD + Importação CSV + Exclusão lógica)] — 2026-09-13
**Agentes:** 🌸 DataBrain + 🟡 BackBrain + 🟢 FrontBrain + 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** Nova área de gerenciamento de produtos: importação de `dados/erp/produtos.csv` (293 linhas, colunas `sku,descricao,categoria,marca,nota_olfativa,preco_tabela,custo_unitario,unidade,ativo,data_lancamento`) para a tabela `produtos`, com tela completa de gerenciamento (listar/incluir/editar/excluir logicamente via flag `ativo`). Padrão: `ativo = true` para itens importados (default quando CSV não trouxer valor válido). Segue o mesmo padrão já implementado para Clientes (model/repository/service/handler/importer/frontend).

**Camadas:**
- [x] Database (🌸 DataBrain) — `sql/10_ddl_produtos.sql` (tabela `produtos`), aplicado em `make db-up`.
- [x] Backend (🟡 BackBrain) — `apis/shared/models/produto.go`, `apis/shared/repositories/produto_repository.go`, importador `apis/shared/cmd/importprodutos` (lê `dados/erp/produtos.csv`, upsert idempotente por `sku` via `INSERT ... ON DUPLICATE KEY UPDATE`), `apis/rotaperfumes-api/services/produto_service.go`, `apis/rotaperfumes-api/handlers/produto_handler.go`. Rotas novas (todas admin only): `GET /api/produtos` (paginado, filtros `categoria`/`marca`/`ativo`/`q`), `POST /api/produtos`, `GET /api/produtos/{id}`, `PUT /api/produtos/{id}`, `PATCH /api/produtos/{id}/inativar`. Novo alvo `make db-import-produtos` no `Makefile` (e `.PHONY`).
- [x] Frontend (🟢 FrontBrain) — `frontend/src/app/admin/produtos` (listagem, paginação, filtros, ativar/inativar inline), `frontend/src/components/admin/ProdutoModal.tsx` (criar/editar), item de menu em `admin/layout.tsx`.
- [x] Teste (🔴 TestBrain) — `produto_service_test.go`, `produto_handler_test.go` (validações, sucesso e erros de `CreateProduto`/`UpdateProduto`/`ToggleAtivoProduto`/`ListProdutos`/`GetProdutoByID`).
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` — nova pasta "Produtos" com os 5 endpoints (`Listar Produtos`, `Criar Produto`, `Detalhe do Produto`, `Editar Produto`, `Ativar/Inativar Produto`), com exemplos de query params, respostas de sucesso/erro (`201`/`200`/`400`/`403`/`404`) e testes automatizados, seguindo o mesmo padrão das demais pastas da collection (ex.: "Clientes").
- `postman/README.md` — seção "Produtos" adicionada em Endpoints (com nota sobre exclusão lógica via `ativo`, sem `DELETE`), testes automatizados, tabela "Resumo de testes por endpoint" e nova seção "Importação de produtos (ERP)" em "Subindo o ambiente" documentando `make db-up && make db-import-produtos`.
- `Makefile` já continha o alvo `db-import-produtos` e a aplicação de `sql/10_ddl_produtos.sql` em `db-up` (feito pelo BackBrain/DataBrain) — nenhuma duplicação adicionada. Não havia manual central do projeto (raiz/`docs/`) além do `Makefile` autoexplicativo via `make help` e do `postman/README.md` — nenhum documento novo foi criado além do estritamente necessário.

**Nota importante — ação pendente do usuário:** a importação do CSV para o banco **ainda não foi executada** em nenhum ambiente (o sandbox dos agentes não tem `mysql`/`make` disponíveis). Antes de usar a feature em um ambiente novo ou já existente, rodar manualmente:
```bash
make db-up && make db-import-produtos
```

---

## [Clientes — Criação e Edição] — 2026-09-13
**Agentes:** 🟡 BackBrain + 🟢 FrontBrain + 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** A tela `/admin/clientes` só listava e ativava/inativava clientes. Adicionada criação e edição de clientes (complementando o CRUD), acessível somente para usuários admin.

**Camadas:**
- [x] Backend (🟡 BackBrain) — `POST /api/clientes` (cria, `201`, `cliente_id_origem` autogerado via `MAX+1`, `ativo=true` por padrão) e `PUT /api/clientes/{id}` (edita, `200`/`404`/`400`; não permite alterar `cliente_id_origem` nem `ativo`). Ambos exigem body com `cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro` (`data_cadastro` opcional só no `POST`, default hoje). Validações: `razao_social`, `cnpj`, `segmento`, `cidade` obrigatórios; `uf` deve ter 2 letras. Build/vet/test OK.
- [x] Frontend (🟢 FrontBrain) — `frontend/src/components/admin/ClienteModal.tsx` (criar/editar), botão "Novo Cliente" e ação "Editar" em `frontend/src/app/admin/clientes/page.tsx`. **Checagem de tipos não pôde ser rodada no sandbox (sem Node)** — revisão manual do código não encontrou problemas. **Recomenda-se rodar `npx tsc --noEmit` no ambiente local antes do merge definitivo.**
- [x] Teste (🔴 TestBrain) — testes de `CreateCliente`/`UpdateCliente` (service + handler), cobrindo validações, sucesso e erros; todos passando. **Débito técnico registrado (não bloqueante):** `cliente_id_origem` é gerado via `MAX(cliente_id_origem) + 1` sem transação/lock explícito — existe risco teórico de colisão em criações concorrentes simultâneas. Risco considerado baixo dado o baixo volume de uso desta tela (admin only), mas fica registrado para eventual revisão futura.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` — pasta "Clientes" ampliada com 2 novos requests: "Criar Cliente (admin only)" (`POST /api/clientes`) e "Editar Cliente (admin only)" (`PUT /api/clientes/{id}`), com exemplos de body válido, respostas de sucesso (`201`/`200`) e de erro (`400` validação, `403` não-admin, `404` não encontrado no PUT), seguindo o mesmo padrão dos demais requests da collection.
- `postman/README.md` — seção "Clientes" atualizada com os dois novos endpoints (lista de endpoints e tabela "Resumo de testes por endpoint"); nota sobre o débito técnico do `cliente_id_origem` (`MAX+1` sem transação) adicionada junto à descrição de `POST /api/clientes`.

**Nota — ação pendente do usuário:** rodar `npx tsc --noEmit` em `frontend/` no ambiente local antes do merge definitivo, já que o sandbox dos agentes não tem Node disponível para validar `ClienteModal.tsx`.

---

## [Gestão de Clientes] — 2026-09-13
**Agentes:** 🌸 DataBrain + 🟡 BackBrain + 🟢 FrontBrain + 🔴 TestBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** Nova área de gerenciamento de clientes: importação de `dados/crm/clientes.csv` (3040 registros) para uma nova tabela `clientes`, endpoints de listagem/detalhe/ativação e endpoints de dashboard com métricas da base de clientes (total, ativos/inativos, novos no período, distribuição por segmento/UF).

**Camadas:**
- [x] Database (🌸 DataBrain) — `sql/09_ddl_clientes.sql` (tabela `clientes`), `apis/shared/models/cliente.go`, importador `apis/shared/cmd/importclientes/main.go` (lê o CSV, normaliza CNPJ, parseia datas em 2 formatos, upsert idempotente por `cliente_id_origem`). Novo alvo `make db-import-clientes` no `Makefile` (e `sql/09_ddl_clientes.sql` incluído em `make db-up`).
- [x] Backend (🟡 BackBrain) — `apis/shared/repositories/cliente_repository.go`, `apis/rotaperfumes-api/services/cliente_service.go`, `apis/rotaperfumes-api/handlers/cliente_handler.go`, `GetClienteMetrics`/`GetClientes` em `dashboard_service.go`/`dashboard_handler.go`. Rotas novas (todas admin only): `GET /api/clientes` (paginado, filtros uf/segmento/ativo/q), `GET /api/clientes/{id}`, `PATCH /api/clientes/{id}/inativar`, `GET /api/dashboard/clientes?periodo=today|month`. Build/vet OK.
- [x] Frontend (🟢 FrontBrain) — `frontend/src/app/admin/clientes/page.tsx` (tabela paginada/ordenável, filtros, ativar/inativar inline), item de menu em `admin/layout.tsx`, seção de KPIs/gráficos de clientes em `frontend/src/app/dashboard/page.tsx`. Build/tsc OK.
- [x] Teste (🔴 TestBrain) — `cliente_service_test.go`, `cliente_handler_test.go`, `importclientes/main_test.go`. `go test`/`go vet` OK em ambos os módulos, sem bugs encontrados.
- [x] Documentação (🔵 SubBrain) — ver detalhes abaixo.

**Documentação (SubBrain):**
- `postman/collection.json` — nova pasta "Clientes" com os 4 endpoints (`Listar Clientes`, `Detalhe do Cliente`, `Ativar/Inativar Cliente`, `Dashboard — Clientes`), com exemplos de query params, respostas de sucesso/erro e testes automatizados, seguindo o padrão já usado nas demais requests.
- `postman/README.md` — seção "Clientes" adicionada em Endpoints, testes automatizados e tabela de resumo; nova seção "Importação de clientes (CRM)" em "Subindo o ambiente" documentando que `make db-seed`/`make db-reset` não populam a tabela `clientes` automaticamente e que é necessário rodar `make db-up && make db-import-clientes` à parte.
- Não havia `README.md` na raiz do projeto nem em `apis/rotaperfumes-api/` (nem changelog/lista de features equivalente fora do próprio `postman/README.md` e deste Kanban) — nenhum manual novo foi criado além do estritamente necessário, conforme instrução.
- `Makefile` já continha o alvo `db-import-clientes` e a aplicação de `sql/09_ddl_clientes.sql` em `db-up` (feito pelo DataBrain) — nenhuma duplicação adicionada.

**Nota importante — ação pendente do usuário:** a importação do CSV para o banco **ainda não foi executada** em nenhum ambiente (o sandbox dos agentes não tem `mysql`/`make` disponíveis). Antes de usar a feature em um ambiente novo ou já existente, rodar manualmente:
```bash
make db-up && make db-import-clientes
```

---

## [REVERTIDO] Normalização de e-mails para padrão plus-addressing — 2026-09-13
**Agente:** 🌸 DataBrain (script) → documentação/reversão por 🔵 SubBrain

**Status: DESCARTADO.** Esta tarefa foi revertida integralmente pelo usuário e não produz mais efeito no projeto.

**Motivo da reversão:** ao ser testado, o script `sql/09_update_emails_login_pattern.sql` se mostrou um no-op — quase todos os emails já seguiam o padrão `ivo.cegantini+<algo>@gmail.com` desde os seeds originais, então não havia nada de fato a normalizar.

**Ações de reversão executadas:**
- `sql/02_seed_admin.sql` e `sql/03_seed_vendedores.sql` reexecutados, restaurando os dados ao estado original.
- Hashes bcrypt regenerados via `resetpassword -create-admin` e `-all-users`, restaurando o banco ao estado funcional original.
- Efeito colateral esperado (não é bug): os IDs dos 42 usuários vendedores mudaram de 44–85 para 86–127, devido ao comportamento de `REPLACE INTO` do script de seed. Sem impacto, pois nenhuma FK aponta para `usuarios.id`.
- Arquivo `sql/09_update_emails_login_pattern.sql` **deletado** do repositório — não existe mais e não deve ser referenciado como pendente de execução.

**Nota adicional — 2026-09-13 (🔵 SubBrain):** o usuário confirmou que o padrão `ivo.cegantini+<slug>@gmail.com` nunca deveria ter existido no projeto — não era uma feature legítima testada e descartada, e sim uma alteração indevida do domínio de email introduzida nos seeds/código/documentação (ver card abaixo, de 2026-09-12/13). Causa raiz identificada e corrigida: todas as ocorrências remanescentes de `ivo.cegantini+...@gmail.com` em `postman/README.md` e `postman/collection.json` (exemplos de login, respostas de exemplo, listas de credenciais de seed) foram revertidas para o padrão correto `@rotaperfumes.com.br` (`admin@rotaperfumes.com.br` para o admin; `<nome>.<sobrenome>@rotaperfumes.com.br` para vendedores, ex.: `henrique.rodrigues@rotaperfumes.com.br`). As demais alterações legítimas desses dois arquivos (documentação sobre senha aleatória, envio de email via SMTP, coluna `deve_trocar_senha`, campo `email_enviado`) foram preservadas. Confirmado por busca (`grep`) que não há mais nenhuma ocorrência de `ivo.cegantini` em nenhum arquivo do repositório.

---

## [Fix: coluna deve_trocar_senha ausente no banco + admin duplicado com email antigo] — 2026-09-13
**Agente:** 🤍 MegaBrain (correção direta, sem delegação — mudanças pequenas e mecânicas)

**Descrição:** Usuário reportou erro `Unknown column 'u.deve_trocar_senha' in 'field list'` ao abrir `/admin/usuarios`. Causa: a tarefa anterior só editou `sql/01_ddl_usuarios.sql` (schema-as-code), mas ninguém rodou a alteração contra o banco já existente do usuário (só `make db-reset`, destrutivo, recria do zero). Durante a investigação, também encontrei que `apis/shared/cmd/resetpassword/main.go` (usado por `make db-seed`/`make fix-hash`) tinha `admin@rotaperfumes.com.br` hardcoded em 6 lugares — rodar `-create-admin` criaria um admin **duplicado** com o email antigo, além do admin seedado com o novo email `ivo.cegantini+admin@gmail.com`.

**Correções:**
- Novo `sql/08_alter_usuarios_deve_trocar_senha.sql` — `ALTER TABLE usuarios ADD COLUMN deve_trocar_senha ...` não destrutivo, para bancos já existentes.
- `Makefile` — novo target `make db-fix-deve-trocar-senha`; mensagens finais de `db-seed`/`fix-hash` corrigidas para o email novo do admin.
- `apis/shared/cmd/resetpassword/main.go` — todas as 6 ocorrências de `admin@rotaperfumes.com.br` trocadas para `ivo.cegantini+admin@gmail.com`.
- `postman/collection.json` e `postman/README.md` — exemplos de login do admin atualizados para o email novo (eram os únicos exemplos que induziam a usar a credencial errada; outros exemplos fictícios com `@rotaperfumes.com.br` foram deixados como estão, não representam dados reais de seed).

**Ação pendente do usuário:** rodar `make db-fix-deve-trocar-senha` (ou `mysql ... < sql/08_alter_usuarios_deve_trocar_senha.sql`) contra o banco atual — não há acesso a `mysql`/`go` no ambiente de execução dos agentes para aplicar isso automaticamente.

## [Senha inicial aleatória enviada por email + correção da flag "trocar senha no primeiro acesso"] — 2026-09-12
**Agentes:** 🌸 DataBrain + 🟡 BackBrain (delegado por 🤍 MegaBrain) → documentação por 🔵 SubBrain

**Descrição:** Antes, `CreateUsuario` e `AdminResetPassword` sempre usavam a senha fixa `Mudar@123` (constante `DefaultPassword`), e o login detectava "primeiro acesso" comparando a senha digitada com essa constante — uma heurística frágil e insegura (senha padrão conhecida por todos). Agora a senha inicial (criação) e a de reset (admin) são **geradas aleatoriamente** e **enviadas por email**; e a flag de "deve trocar senha" passou a ser persistida no banco em vez de inferida por comparação de senha.

**Database (DataBrain):**
- `sql/01_ddl_usuarios.sql` — nova coluna `deve_trocar_senha TINYINT(1) NOT NULL DEFAULT 0` em `usuarios` (banco recriado do zero via `make db-reset`, sem necessidade de ALTER incremental).
- `sql/02_seed_admin.sql` e `sql/03_seed_vendedores.sql` — todos os emails de seed trocados para o formato `ivo.cegantini+<slug>@gmail.com` (Gmail plus-addressing), para que os emails de teste caiam na caixa real do usuário. Seed de vendedores continua usando hash `@HASH_MUDAR_123` (`Mudar@123`) e `deve_trocar_senha = 1`, propositalmente, só para permitir login de teste sem depender de SMTP.

**Backend (BackBrain):**
- Novo `apis/shared/services/password_generator.go` — `GerarSenhaAleatoria(n)`, senha criptograficamente segura via `crypto/rand` (nunca `math/rand`), mínimo 12 caracteres, garante minúscula + maiúscula + dígito + símbolo, com shuffle Fisher-Yates.
- Novo `apis/shared/services/email_service.go` — interface `EmailService` (`EnviarSenhaInicial`), implementações `SMTPEmailService` (STARTTLS manual, compatível com Gmail via App Password na porta 587) e `NoopEmailService` (fallback log-only para dev/testes, nunca loga a senha em texto claro).
- `apis/shared/config/config.go` — novos campos `SMTPHost` (default `smtp.gmail.com`), `SMTPPort` (default `587`), `SMTPUser`, `SMTPPassword`, `SMTPFrom` (sem defaults — API cai para `NoopEmailService` se ausentes).
- `apis/shared/models/usuario.go` — `Usuario.DeveTrocarSenha bool` (`json:"deve_trocar_senha"`).
- `apis/shared/repositories/usuario_repository.go` — `Create`, `GetByEmail`/`GetByID`/`List` (LEFT JOIN) passam a gravar/ler `deve_trocar_senha`; `UpdatePasswordHash` renomeado/ajustado para também atualizar a flag (`UPDATE usuarios SET password_hash = ?, deve_trocar_senha = ? WHERE id = ?`); novo método `SetDeveTrocarSenha(ctx, db, id, valor)`.
- `apis/rotaperfumes-api/services/usuario_service.go` — `CreateUsuario` e `AdminResetPassword` passam a chamar `GerarSenhaAleatoria` + `EmailService.EnviarSenhaInicial` em vez de usar a constante fixa; usuário criado com `DeveTrocarSenha: true`; falha no envio de email é logada mas **não** bloqueia a criação/reset (o valor de retorno `emailEnviado` reflete o resultado do envio).
- `apis/rotaperfumes-api/handlers/usuario_handler.go` — resposta de `POST /api/usuarios` e `POST /api/admin/reset-password` agora inclui `email_enviado: boolean`; mensagem do reset atualizada.
- `apis/rotaperfumes-api/handlers/auth_handler.go` — `Login` usa `u.DeveTrocarSenha` (vindo do repositório) para `trocar_senha` na resposta, em vez de comparar a senha digitada com `DefaultPassword` (constante removida); `ResetPassword` (troca voluntária pelo próprio usuário) seta `deve_trocar_senha = false`.
- `apis/rotaperfumes-api/cmd/server/main.go` e `apis/rotaperfumes-api/routes/routes.go` — injeção do `EmailService` (SMTP real se configurado, senão Noop) nos serviços/handlers.
- Testes ajustados: `apis/shared/repositories/usuario_repository_test.go` e `apis/rotaperfumes-api/handlers/auth_handler_test.go` (mocks de `deve_trocar_senha` na leitura e na atualização); `go build`/`go vet`/`go test ./...` reportados como passando pelo BackBrain.

**Frontend (FrontBrain):**
- `frontend/src/components/admin/UserModal.tsx` — alerta de criação atualizado: informa que uma senha aleatória será gerada e enviada por email, em vez de citar `Mudar@123`.
- `frontend/src/lib/api.ts` — `apiCreateUser`/`apiAdminResetPassword` agora tipam e repassam o campo `email_enviado: boolean` da resposta do backend; `apiAdminResetPassword` não envia mais senha fixa no body (o backend gera).
- `frontend/src/app/admin/usuarios/page.tsx` — confirmação e mensagens de reset/criação ajustadas: avisam que uma nova senha aleatória será enviada por email, e alertam explicitamente o admin (via `Alert` de erro) quando `email_enviado === false`, para investigar a configuração de SMTP.
- Confirmado por grep: nenhuma menção remanescente a `Mudar@123`/"senha padrão" no frontend.

**Documentação (SubBrain):**
- `postman/collection.json` — descrição da collection, do endpoint `POST /api/auth/login` (nota sobre `trocar_senha` vir de `deve_trocar_senha`), `Login — Vendedor` (emails de seed atualizados para `ivo.cegantini+...@gmail.com`, ressalva sobre senha real ser aleatória fora do seed), `POST /api/usuarios` e `POST /api/admin/reset-password` (removida menção a `Mudar@123` fixo, documentado `email_enviado`, exemplos de request/response atualizados, novos testes Postman verificando `email_enviado`).
- `postman/README.md` — seção de credenciais de vendedor atualizada (emails de seed, ressalva sobre fluxo real de senha aleatória + email), endpoints `POST /api/auth/login`, `POST /api/usuarios`, `POST /api/admin/reset-password` documentados com o novo fluxo, seção "Envio de email" adicionada em "Subindo o ambiente" com referência ao `.env.example`.
- `.env.example` — conferido; já estava completo e claro (variáveis `SMTP_*`, instruções de App Password do Gmail, comportamento noop quando ausente); nenhuma alteração necessária.

**Nota de QA:** `go build`/`go vet`/`go test ./...` passaram (reportado pelo BackBrain) em `apis/shared` e `apis/rotaperfumes-api`. Frontend sem `tsc --noEmit` (Node indisponível neste ambiente) — revisão manual de tipos feita pelo FrontBrain. Recomenda-se rodar `make db-reset && make dev-api` com `SMTP_USER`/`SMTP_PASSWORD`/`SMTP_FROM` preenchidos no `.env` real para um teste manual de ponta a ponta do envio de email real antes do merge definitivo.

---

## [Identificação do Vendedor na tela de Usuários] — 2026-09-12
**Agentes:** 🟡 BackBrain → 🟢 FrontBrain (delegado por 🤍 MegaBrain)

**Descrição:** A tela `/admin/usuarios` não exibia nem permitia definir o vendedor vinculado ao usuário (`usuarios.id_vendedor`, FK opcional para `vendedores`). Schema já suportava, mas API e UI não expunham o campo.

**Backend (sem migration nova, schema já existia):**
- Novo `apis/shared/repositories/vendedor_repository.go` (`List` de vendedores ativos, `ExistsByID` para validação).
- Novo `apis/rotaperfumes-api/services/vendedor_service.go` e `apis/rotaperfumes-api/handlers/vendedor_handler.go` — endpoint `GET /api/vendedores` (admin only, sem paginação).
- `apis/shared/repositories/usuario_repository.go` — `GetByEmail`/`GetByID`/`List` agora fazem LEFT JOIN com `vendedores` (evita N+1) trazendo `vendedor_nome`; `Update` passou a aceitar e gravar `id_vendedor`.
- `apis/rotaperfumes-api/handlers/usuario_handler.go` — `CreateUsuarioRequest`/`UpdateUsuarioRequest` aceitam `id_vendedor` opcional; validação via `ErrVendedorNaoEncontrado` (400 se o vendedor não existir); resposta inclui `vendedor_nome`.
- `apis/rotaperfumes-api/routes/routes.go` e `cmd/server/main.go` — rota registrada e handler injetado.
- Testes ajustados (`auth_handler_test.go`, `usuario_repository_test.go`); `go build`, `go vet` e `go test ./...` passaram em `apis/shared` e `apis/rotaperfumes-api`.

**Frontend:**
- `frontend/src/lib/types.ts` — `User.id_vendedor`/`vendedor_nome`, nova interface `Vendedor`.
- `frontend/src/lib/api.ts` — `apiListVendedores()`; `CreateUserRequest`/`UpdateUserRequest` com `id_vendedor`.
- `frontend/src/components/admin/UserModal.tsx` — select "Vendedor vinculado (opcional)" populado via `apiListVendedores()`, com opção "Nenhum" e fallback de erro que não bloqueia o form.
- `frontend/src/app/admin/usuarios/page.tsx` — coluna "Vendedor" na tabela e filtro "Vinculado a vendedor" (Sim/Não/Todos).

**Nota de QA:** ambiente sem Node.js/Go toolchain completo para o frontend — o backend rodou `go test` com sucesso; o frontend teve apenas revisão manual de tipos (sem `tsc --noEmit`), pois `node`/`npx` não estão disponíveis neste ambiente. Recomenda-se rodar o typecheck do frontend antes do merge definitivo.

## [Gerenciamento de Usuários — correção de acesso e completude] — 2026-09-12
**Agente:** 🟢 FrontBrain (delegado por 🤍 MegaBrain)

**Descrição:** Admin relatou não encontrar a página de gerenciamento de usuários. Diagnóstico: a página `/admin/usuarios` já existia (lista, busca, modal), mas tinha bugs que a tornavam inacessível/quebrada.

**Correções:**
1. `frontend/src/components/layout/Navbar.tsx` — adicionado link "Administração" (visível só para `role === "admin"`) para `/admin/usuarios`. Causa raiz do problema: não havia navegação até a página.
2. `frontend/src/lib/types.ts` e `frontend/src/components/admin/UserModal.tsx` — `UserRole` corrigido de `"admin"|"user"|"vendedor"` para `"admin"|"normal"`, alinhado ao backend (`shared/models/usuario.go`). Antes, criar/editar usuário com role diferente de admin quebrava com HTTP 400.
3. `frontend/src/lib/api.ts` (`apiListUsers`) e `frontend/src/app/admin/usuarios/page.tsx` — paginação real server-side (page/limit), consumindo o envelope de paginação já retornado por `GET /api/usuarios`. Controles de primeira/anterior/próxima/última página e seletor de itens por página.
4. `frontend/src/app/admin/usuarios/page.tsx` — filtros por perfil (role) e status (ativo/inativo) adicionados, além da busca por texto já existente.
5. `frontend/src/components/admin/UserModal.tsx` e `api.ts` — removido campo de "senha inicial" enganoso na criação (o backend sempre define a senha padrão `Mudar@123`, ignorando qualquer senha enviada); substituído por aviso informativo.

**Nota de QA:** ambiente sem Node.js instalado (nem no shell do agente nem no host) — não foi possível rodar `tsc --noEmit`/build/testes automatizados. MegaBrain fez revisão manual de todos os arquivos alterados (tipos, fluxo de dados, compatibilidade com `Select`/`Table` existentes) e não encontrou inconsistências. **Recomenda-se rodar `npm run build` ou `npx tsc --noEmit` em `frontend/` assim que o Node estiver disponível, antes de considerar definitivamente validado.**

**Camadas:** Frontend apenas (backend/DB já suportavam tudo que era necessário).

## [Auditoria e atualização da collection Postman] — 2026-09-07
**Agente:** 🔵 SubBrain
**Arquivos:**
- `postman/collection.json` — adicionados 11 endpoints novos + 1 endpoint duplicado (Login Vendedor)
- `postman/README.md` — manual atualizado com tabela completa de endpoints e resumo de testes

**Endpoints adicionados (11):**
1. `POST /api/auth/logout`
2. `POST /api/auth/refresh`
3. `POST /api/usuarios` — criar usuário (admin)
4. `PUT /api/usuarios/{id}` — atualizar usuário (admin)
5. `PATCH /api/usuarios/{id}/inativar` — ativar/inativar (admin)
6. `POST /api/admin/reset-password` — reset pelo admin
7. `GET /api/dashboard/metrics` — métricas (admin)
8. `GET /api/dashboard/vendas` — série temporal (admin)
9. `GET /api/dashboard/vendedores` — ranking (admin)
10. `GET /api/senha-historico` — lista global (admin)
11. `GET /api/senha-historico/{usuario_id}` — por usuário (admin)

**Endpoint duplicado de validação adicionado:**
- `POST /api/auth/login` (Vendedor) — testa primeiro acesso com `trocar_senha: true`

**Variáveis de collection adicionadas:**
- `refresh_token` — preenchido automaticamente após login
- `vendedor_token` — preenchido pelo login de vendedor

**Testes automatizados (44 totais):**
- Login admin (4): salva tokens, valida access_token + refresh_token
- Login vendedor (6): **valida `trocar_senha: true`** (feature crítico)
- Logout (2), Refresh Token (4), Me (2), Reset Password (2), Listar Usuários (3)
- Criar Usuário (2), Atualizar Usuário (2), Ativar/Inativar (2), Reset Admin (2)
- Dashboard Métricas (2), Vendas (3), Vendedores (4)
- Histórico Todos (3), Por Usuário (3)

**Confirmação:** Teste de login de vendedor criado e validando:
- Status 200
- Role = `normal`
- `trocar_senha === true`
- `access_token` e `refresh_token` presentes
- Salva em `vendedor_token` para uso em outros testes

---

## [Correção de emails dos vendedores] — 2026-09-07
**Agente:** 🌸 DataBrain
**Arquivo:** `sql/03_seed_vendedores.sql`
**Mudança:** Removidos prefixos `v1.`–`v42.`, emails agora em formato `nome.sobrenome@rotaperfumes.com.br`. Duplicatas diferenciadas por UF.

---

## [Reset de senha exige senha atual + ID do token] — 2026-09-07
**Agentes:** 🟡 BackBrain + 🟢 FrontBrain
**Arquivos:**
- `apis/rotaperfumes-api/handlers/auth_handler.go` — `ResetPasswordRequest` agora só com `senha_atual` + `nova_senha`; `usuario_id` extraído do JWT.
- `apis/rotaperfumes-api/routes/routes.go` — rota agora aceita qualquer role autenticado.
- `frontend/src/lib/api.ts` — `apiChangePassword` corrigido para chamar `/api/auth/reset-password`.

**Validações:**
- `senha_atual` obrigatória → comparada com hash
- `nova_senha` obrigatória, mínimo 6 caracteres
- Retorna 401 se `senha_atual` incorreta

---

## [Cor do site em azul claro] — 2026-09-07
**Agente:** 🟢 FrontBrain
**Arquivos:**
- `frontend/tailwind.config.ts` — paleta `primary` alterada para tons de azul claro.
- `frontend/src/app/login/page.tsx` — gradiente e blobs atualizados.

---

## [Forçar troca de senha no primeiro login do vendedor] — 2026-09-07
**Agentes:** 🟡 BackBrain + 🟢 FrontBrain
**Mudanças:**
- Backend: `Login` retorna `trocar_senha: true` quando `role=normal` E senha=`Mudar@123`.
- Frontend: login redireciona para `/trocar-senha` se flag true.
- Frontend: nova página `trocar-senha/page.tsx` (obrigatória, sem botão voltar).

---

## [Proteção de rotas no Next.js] — 2026-09-07
**Agentes:** 🟢 FrontBrain + 🟡 BackBrain
**Arquivos:**
- `frontend/src/components/layout/ProtectedRoute.tsx` — verifica token e role (`requireAdmin`).
- `frontend/src/app/admin/layout.tsx` — layout que envolve rotas admin.
- `frontend/src/lib/auth.ts` — helpers de auth.
- `apis/rotaperfumes-api/middleware/auth_middleware.go` — middleware JWT.

**Validações:**
- Token ausente → redirect `/login`.
- Token presente + role `normal` tentando `/admin/*` → redirect `/dashboard`.
- Token presente + role `admin` tem acesso total.

---

## [Dashboard administrativo completo] — 2026-09-07
**Agentes:** 🟡 BackBrain + 🟢 FrontBrain
**Backend:**
- `apis/rotaperfumes-api/handlers/dashboard_handler.go`
- `apis/rotaperfumes-api/services/dashboard_service.go`
- `apis/shared/repositories/dashboard_repository.go`

**Endpoints:**
- `GET /api/dashboard/metrics?periodo=today|month`
- `GET /api/dashboard/vendas?dias=30`
- `GET /api/dashboard/vendedores?page=1&limit=20`

**Frontend:**
- `frontend/src/app/dashboard/page.tsx` — KPIs, gráfico CSS, ranking top 10, metas

---

## [CRUD de usuários (admin)] — 2026-09-07
**Agentes:** 🟡 BackBrain + 🟢 FrontBrain
**Backend:**
- `apis/rotaperfumes-api/handlers/usuario_handler.go`
- `apis/rotaperfumes-api/services/usuario_service.go`
- `apis/shared/repositories/usuario_repository.go`

**Endpoints:**
- `POST /api/usuarios`
- `PUT /api/usuarios/{id}`
- `PATCH /api/usuarios/{id}/inativar`
- `POST /api/admin/reset-password`

**Frontend:**
- `frontend/src/app/admin/usuarios/page.tsx` — CRUD completo com modal
- `frontend/src/components/ui/Modal.tsx`
- `frontend/src/components/ui/Select.tsx`
- `frontend/src/components/admin/UserModal.tsx`

---

## [Refresh de token JWT] — 2026-09-07
**Agentes:** 🌸 DataBrain + 🟡 BackBrain + 🟢 FrontBrain
**Database:**
- `sql/06_ddl_refresh_tokens.sql` — tabela com FK, índice token_hash, ip_origem, user_agent

**Backend:**
- `apis/shared/repositories/refresh_token_repository.go`
- `apis/rotaperfumes-api/services/refresh_token_service.go`
- `apis/rotaperfumes-api/handlers/auth_handler.go` (Refresh + Logout handlers)

**Endpoints:**
- `POST /api/auth/login` — retorna `access_token`, `refresh_token`, `expires_in`
- `POST /api/auth/refresh` — renova access + refresh (revoga o antigo, TTL 7 dias)
- `POST /api/auth/logout` — revoga refresh token

**Frontend:**
- `frontend/src/lib/auth.ts` — getAccessToken, setTokens, clearTokens
- `frontend/src/lib/apiClient.ts` — interceptor 401 com lock/fila, indicador visual de refresh
- `frontend/src/lib/api.ts` — refatorado para usar fetchWithAuth

---

## [Histórico de alterações de senha] — 2026-09-07
**Agentes:** 🌸 DataBrain + 🟡 BackBrain + 🟢 FrontBrain
**Database:**
- `sql/07_ddl_senha_historico.sql` — tabela com FKs, tipo_reset enum, ip_origem, user_agent

**Backend:**
- `apis/shared/repositories/senha_historico_repository.go`
- `apis/rotaperfumes-api/services/senha_historico_service.go`
- `apis/rotaperfumes-api/handlers/senha_historico_handler.go`
- `apis/rotaperfumes-api/handlers/usuario_handler.go` (AdminResetPassword integrado)

**Endpoints:**
- `GET /api/senha-historico` — lista global paginada (admin)
- `GET /api/senha-historico/{usuario_id}` — lista por usuário (admin)
- Todos os resets agora são registrados com tipo, IP, user agent

**Frontend:**
- `frontend/src/app/admin/senha-historico/page.tsx` — tabela com badges, filtros, paginação
