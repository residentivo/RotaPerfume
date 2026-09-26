# A Fazer

> Os cards registrados no Lote 5 (2026-09-25) foram para `fazendo.md` no Lote 6 (2026-09-26), exceto o DOC-04, que só entra em execução se o usuário pedir o manual completo. Os cards BUG-09, SEC-08, FE-10 e SEC-09 foram registrados no fechamento do Lote 6 (2026-09-26).

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
