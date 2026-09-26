# A Fazer

> Os cards registrados no Lote 5 (2026-09-25) foram para `fazendo.md` no Lote 6 (2026-09-26), exceto o DOC-04, que só entra em execução se o usuário pedir o manual completo. Os cards BUG-09, SEC-08, FE-10 e SEC-09 foram registrados no fechamento do Lote 6 (2026-09-26). Os cards BUG-10, FE-11, FE-12, SEC-10 e INFO-01 foram registrados no fechamento do Lote 7 (2026-09-26).

---

## BUG-10: `seedusers.FindProjectRoot` não acha a raiz do projeto ao rodar de `apis/shared` — prioridade MÉDIA

**Status:** não iniciado
**Camada:** Backend shared
**Responsável:** 🟡 BackBrain (testes: 🔴 TestBrain)
**Origem:** 🔴 TestBrain, TST-03 (Lote 7, 2026-09-26). O bug já existia antes do lote e não foi corrigido nele.

**Descrição:**
- O `seedusers.FindProjectRoot` (`apis/shared/tools/seedusers`) exige um `go.mod` na raiz do repositório, que não existe.
- Rodando de `apis/shared` (como faz o `make` do seedusers), ele cai no fallback `cwd/../../..`, que é a pasta **acima** do SistemaCompleto, e aí não acha `sql/02_seed_admin.sql`.
- O `tests/tools/seedusers/seedusers_test.go` (`TestFindProjectRoot`) registra o comportamento atual.

**Ação esperada:**
- 🟡 BackBrain: usar `cmdutil.FindProjectRoot` (correção sugerida) e conferir o alvo do seedusers no `Makefile`.
- 🔴 TestBrain: atualizar o `TestFindProjectRoot` para o comportamento correto e manter a cobertura do `tools/seedusers` (hoje 94.3%).

---

## FE-11: timers não são limpos no unmount (trocar-senha, usuarios e vendedores) — prioridade BAIXA

**Status:** não iniciado
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain (testes: 🔴 TestBrain)
**Origem:** 🔴 TestBrain, TST-01 (Lote 7, 2026-09-26).

**Descrição:**
- O `setTimeout` em `frontend/src/app/trocar-senha/page.tsx:88` não é cancelado quando a tela desmonta.
- O `setTimeout(() => setSuccess(null), 4000)` em `frontend/src/app/admin/usuarios/page.tsx` (linhas 228, 248 e 271) e `frontend/src/app/admin/vendedores/page.tsx` (linhas 233, 249 e 272) também não é cancelado. O efeito é um `setState` depois do unmount (e, na trocar-senha, a ação atrasada roda mesmo se o usuário já saiu da tela).
- **Nota do 🔵 SubBrain:** o mesmo padrão de 4 s aparece em `admin/clientes`, `admin/estoque`, `admin/produtos`, `admin/visitas`, `admin/oportunidades`, `admin/pedidos` e `pagamentos`. Vale tratar todos juntos (ex.: um hook comum de mensagem temporária).

**Ação esperada:**
- 🟢 FrontBrain: guardar o id do timer e fazer `clearTimeout` no cleanup do effect (ou no unmount), em todas as telas acima.
- 🔴 TestBrain: cobrir o unmount antes do timer vencer, em `frontend/tests/`.

---

## FE-12: `sortValue` das colunas nunca é usado pelo `Table` — prioridade BAIXA

**Status:** não iniciado
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain (testes: 🔴 TestBrain)
**Origem:** 🔴 TestBrain, TST-01 (Lote 7, 2026-09-26).

**Descrição:** As colunas de `admin/usuarios`, `admin/senha-historico` e `admin/vendedores` definem `sortValue`, mas o componente `Table` (`frontend/src/components/ui/Table.tsx`) nunca o chama. É código morto, ou a ordenação dessas telas não está usando o valor esperado.
- **Nota do 🔵 SubBrain:** a propriedade `sortValue` também é definida em colunas de `admin/clientes`, `admin/estoque`, `admin/produtos`, `admin/visitas`, `admin/oportunidades`, `admin/pedidos` e `pagamentos`. A decisão vale para todas.

**Ação esperada:**
- 🟢 FrontBrain: decidir entre implementar o uso do `sortValue` no `Table` (se a ordenação local dessas telas precisar dele) ou remover a propriedade das colunas.
- 🔴 TestBrain: ajustar ou cobrir em `frontend/tests/`.

---

## SEC-10: endurecimentos sugeridos na revisão do Lote 7 — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Segurança (+ Backend shared)
**Responsável:** 🟣 SecBrain (avaliação) → 🟡 BackBrain (aplicação) → 🔴 TestBrain
**Origem:** 🟣 SecBrain, revisão dos construtores novos do Lote 7 (2026-09-26). Os três construtores foram aprovados; os itens abaixo são melhorias, não bloqueios.

**Descrição e ação esperada:**
1. **`NewSMTPEmailServiceWithTLSConfig`:** forçar `InsecureSkipVerify = false` e `MinVersion = tls.VersionTLS12` na config clonada, para que nenhum chamador desligue a validação por engano.
2. **`GenerateRandomPassword`:** o `alphabet[b%62]` tem viés de módulo. Trocar por `crypto/rand.Int` (ou descarte por rejeição).
3. **CLI do `resetpassword`:** a senha passada em `-password=` fica no histórico do shell. Avaliar leitura por prompt sem eco, stdin ou variável de ambiente, e ajustar o `docs/roteiro-teste-manual-lote6.md` (que usa `-password`).
4. **`seedusers`:** remover as constantes mortas de senha padrão e os defaults `golang/golang` do DSN.
5. **`seedusers.ReplaceInFile`:** grava hashes reais em `sql/0*_seed*.sql`, com risco de commit acidental. Avaliar gravar em arquivo temporário ou ignorado pelo git.

---

## INFO-01: linhas não alcançáveis aceitas como fora da meta de cobertura — informativo

**Status:** registrado; nenhuma ação pendente
**Camada:** Testes
**Origem:** 🔴 TestBrain, TST-02 e TST-03 (Lote 7, 2026-09-26). Aceito pelo 🤍 MegaBrain.

**Descrição:** As linhas que restam sem cobertura no Go só são alcançáveis mudando o código de produção: erros de `crypto/rand` (`password_generator`), `os.Getwd`/`os.Executable`, `rows.Columns()`, a escrita no `DATA` do SMTP, a escrita do cabeçalho num `csv.Writer` com buffer e os ramos de `maskDSN` sem `@`/`:`. Também ficam fora da meta os `cmd/*/main.go` finos e o `cmd/server`. Não há ação a tomar, salvo se o usuário pedir 100%.

---

## SEC-08: access token emitido antes da inativação volta a valer se o usuário for reativado — prioridade BAIXA

**Status:** não iniciado; aguarda avaliação do 🟣 SecBrain
**Camada:** Backend (+ Segurança)
**Origem:** 🔴 TestBrain, regressão do Lote 6 (2026-09-26), no SEC-06.

**Descrição:**
- O SEC-06 revoga só os refresh tokens ao inativar o usuário (ou desligar o vendedor dele). O access token não é revogado: ele só deixa de valer porque o middleware confere `usuarios.ativo` a cada requisição.
- Se o usuário for reativado dentro do TTL do access token (24h), um token emitido antes da inativação volta a ser aceito. Confirmado na integração: `200` no `/api/auth/me` depois de reativar.

**Ação esperada:**
- 🟣 SecBrain: avaliar o risco e a correção. Possível correção: coluna `tokens_validos_desde` em `usuarios`, gravada na inativação (e, se fizer sentido, na troca de senha e na revogação em massa), comparada com o `iat` do JWT no middleware.
- 🌸 DataBrain: migração, se aprovada. 🟡 BackBrain: aplicar. 🔴 TestBrain: cobrir.

---

## BUG-09: creates devolvem o objeto em memória, com timestamps zerados — prioridade BAIXA

**Status:** não iniciado
**Camada:** Backend
**Origem:** 🟡 BackBrain, Lote 6 (2026-09-26), ao corrigir o BUG-08.

**Descrição:** Mesmo padrão do BUG-08 (corrigido só em clientes). Os creates abaixo devolvem o struct em memória sem reler do banco, então a resposta `201` pode trazer `created_at`/`updated_at` = `0001-01-01T00:00:00Z` (e outros campos preenchidos pelo banco):
- `apis/rotaperfumes-api/services/vendedor_service.go:198`
- `apis/rotaperfumes-api/services/usuario_service.go:220`
- `apis/rotaperfumes-api/services/produto_service.go:189`
- `apis/rotaperfumes-api/services/pagamento_service.go:215`
- `apis/rotaperfumes-api/services/oportunidade_service.go:209`
- `apis/rotaperfumes-api/services/visita_service.go:157`
- `apis/rotaperfumes-api/services/estoque_service.go:182`

**Ação esperada:**
- 🟡 BackBrain: reler o registro depois do INSERT, como o `relerClienteCriado` do BUG-08.
- 🔴 TestBrain: cobrir os 7 creates.
- 🔵 SubBrain: atualizar os exemplos do Postman.

---

## FE-10: estados inconsistentes nas listagens quando a carga falha — prioridade BAIXA

**Status:** não iniciado
**Camada:** Frontend
**Origem:** 🟢 FrontBrain, observações no FE-09 (Lote 6, 2026-09-26). São comportamentos que já existiam antes do lote.

**Descrição:**
- **Troca de página que falha:** a tabela mantém as linhas da página anterior, mas o indicador de paginação mostra a página nova.
- **Contador do cabeçalho:** mostra "0 clientes" (e equivalentes nas outras listagens) quando a primeira carga falha.
- **Vendedores:** quando a recarga depois de inativar falha, o alerta de erro aparece junto com a mensagem de sucesso da inativação.

**Ação esperada:**
- 🟢 FrontBrain: manter a página e o contador coerentes com as linhas exibidas (ou ocultá-los) quando a carga falhar, e separar o sucesso da ação do erro da recarga em Vendedores.
- 🔴 TestBrain: cobrir.

---

## SEC-09: integrar o alerta `[auth][seguranca]` a monitoramento ou notificação — prioridade BAIXA (opcional)

**Status:** não iniciado; opcional, aguarda a priorização do usuário
**Camada:** Segurança (+ Backend)
**Origem:** 🟡 BackBrain, SEC-07 (Lote 6, 2026-09-26).

**Descrição:** Com o SEC-07, o reuso de refresh token já rotacionado gera o alerta `[auth][seguranca]` (user_id, token_id, IP e UA) e revoga todas as sessões do usuário. Hoje o alerta só vai para o log da API: ninguém é avisado.

**Ação esperada:**
- 🟣 SecBrain: definir o destino (monitoramento de logs, e-mail ao admin e/ou ao usuário) e o volume aceitável.
- 🟡 BackBrain: aplicar.

---

## DOC-04: manual completo da base de dados — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Documentação
**Origem:** 🔵 SubBrain, DOC-02 (2026-09-25).

**Descrição:** O `docs/manual-base-de-dados.md` foi criado no DOC-02 com o detalhe da tabela `clientes`, do `uq_clientes_cnpj` e da migração 19. As demais tabelas aparecem só no índice.

**Ação esperada:**
- 🔵 SubBrain: detalhar as demais tabelas, se o usuário quiser o manual completo.
