# A Fazer

## Elevar cobertura do pacote `handlers` para o mínimo de 80%

**Origem:** 🔴 TestBrain, durante os cards "Scope check em Create/Update de Pedidos", "Rotas de Vendedores admin-only" e "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23, ver `tarefas/feito.md`).

**Descrição:** A cobertura de `apis/rotaperfumes-api/handlers` subiu de 73,9% para 77,7% e está em **78,1%** (valor atual, 2026-09-23), mas continua abaixo do mínimo de 80%. O que falta está em outros handlers, fora do escopo daqueles cards.

**Ação esperada:** 🔴 TestBrain mapear (`go test -coverprofile`) os handlers com menor cobertura e escrever os testes que faltam até atingir pelo menos 80% no pacote.

---

## Padronizar status de `POST /api/pagamentos` com pedido inexistente (400 admin vs 404 normal)

**Origem:** 🟡 BackBrain / 🔴 TestBrain, durante o card "Scope check em Create/Update de Pedidos" (2026-09-23).

**Descrição:** Em `POST /api/pagamentos` com `pedido_id` inexistente, o status depende do perfil:
- Usuário `admin`: `400` "pedido não encontrado" (validação do service).
- Usuário `normal`: `404` "pedido não encontrado" (checagem de escopo `pedidoNoEscopo` em `pagamento_handler.go`).

A mensagem é a mesma nos dois casos, mas o status muda conforme o perfil.

**Ação esperada:**
- 🟡 BackBrain avaliar e padronizar um único status. Sugestão: manter `404` para os dois, ou `400` para os dois, desde que o comportamento para `normal` continue sem permitir enumeração.
- 🟢 FrontBrain ajustar o tratamento de erro em `PagamentoModal.tsx`, se necessário.
- 🔵 SubBrain atualizar o Postman.

---

## (Opcional) Remover `meta_mensal` da resposta de `GET /api/vendedores`

**Origem:** 🟣 SecBrain, durante o card "Rotas de Vendedores admin-only" (2026-09-23).

**Descrição:** `GET /api/vendedores` continua com acesso comum porque alimenta os selects do frontend. A resposta inclui `meta_mensal` de todos os vendedores, que é um dado gerencial exposto a qualquer usuário `normal`. O risco é baixo, e por isso o item é opcional.

**Ação esperada:**
- 🟣 SecBrain confirmar se `meta_mensal` deve ficar só com admin.
- Se confirmado, 🟡 BackBrain omitir o campo para usuário `normal` (ou criar um DTO enxuto `id`/`nome` para os selects).
- 🟢 FrontBrain verificar se alguma tela de usuário `normal` depende do campo.

---

## Aplicar `gofmt` em testes de Oportunidades, Visitas e Estoque

**Origem:** 🔴 TestBrain (2026-09-23).

**Descrição:** `gofmt -l` aponta estes arquivos como não formatados, todos anteriores aos cards de 2026-09-23:
- `apis/rotaperfumes-api/handlers/oportunidade_handler_test.go`
- `apis/rotaperfumes-api/handlers/visita_handler_test.go`
- `apis/shared/repositories/estoque_repository_test.go` (incluído em 2026-09-23)

**Ação esperada:** 🔴 TestBrain rodar `gofmt -w` nos três arquivos e confirmar que `go test ./...` e `go vet` continuam OK. A mudança é só de formatação.

---

## Teste manual/e2e do `PedidoModal` no navegador (usuário normal e admin)

**Origem:** 🟢 FrontBrain, durante o card "Scope check em Create/Update de Pedidos" (2026-09-23).

**Descrição:** As mudanças em `frontend/src/components/admin/PedidoModal.tsx` foram validadas só com typecheck. Não houve teste no navegador. As mudanças são:
- Vendedor travado para usuário `normal`.
- Aviso e botão Salvar bloqueado quando não há vendedor vinculado.
- Cascata vendedor → cliente via `apiListClientesDoVendedor`.
- Cliente "(fora da carteira)" mantido na edição.
- Mensagem amigável para o `400` de carteira.

**Ação esperada:** 🔴 TestBrain (ou 🟢 FrontBrain) validar manualmente ou com e2e os fluxos abaixo:
- Criar e editar pedido como `admin`.
- Criar e editar pedido como `normal` com vendedor vinculado.
- Usuário `normal` sem vendedor vinculado.
- Edição de pedido com cliente fora da carteira.

Registrar evidências e abrir bugs, se houver.

---

## Bug: total de vendas truncado para `int` em `enrichWithVendas` / `enrichMetasWithVendas` (perda de centavos)

**Origem:** encontrado durante o card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23).

**Descrição:** Em `apis/shared/repositories/dashboard_repository.go`, `enrichWithVendas` e `enrichMetasWithVendas` convertem o total de vendas para `int`. Com isso, os centavos se perdem em:
- `top_vendedores[].total_vendas`
- `metas_vendedores[].realizado`
- o percentual calculado a partir desses valores

Isso afeta `GET /api/dashboard/metrics` (admin e normal).

**Ação esperada:**
- 🟡 BackBrain manter o valor como `float64`, sem truncar, e recalcular o percentual a partir dele.
- 🔴 TestBrain cobrir com um caso que tenha centavos.
- 🔵 SubBrain revisar os exemplos no Postman.

---

## Padronizar `top_vendedores`/`metas_vendedores` vazios como `[]` (hoje `null`)

**Origem:** encontrado durante o card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23).

**Descrição:** Quando não há dados, `GetTopVendedores` e `GetMetasVendedores` (`apis/shared/repositories/dashboard_repository.go`) devolvem `nil`, que é serializado como `null`. Já `EmptyMetrics` (normal sem vendedor) devolve `[]`. O frontend passou a tolerar os dois, mas o contrato está inconsistente.

**Ação esperada:**
- 🟡 BackBrain inicializar os slices vazios, para que a API sempre devolva `[]`.
- 🔴 TestBrain cobrir o caso vazio.
- 🔵 SubBrain atualizar o Postman, se necessário.

---

## Confirmar com o negócio: Dashboard de vendedor desligado (`data_desligamento`)

**Origem:** encontrado durante o card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23).

**Descrição:** Um usuário `normal` vinculado a um vendedor desligado (`vendedores.data_desligamento` preenchida) tem este comportamento:
- Continua vendo os próprios totais em `GET /api/dashboard/metrics`.
- Recebe `meta_mes = 0`, porque a meta só soma vendedores ativos.
- Não aparece em `GET /api/dashboard/vendedores`, que retorna lista vazia.

O resultado é um Dashboard inconsistente, com vendas e sem meta ou ranking.

**Ação esperada:**
- Confirmar com o usuário o comportamento desejado: bloquear o acesso, mostrar tudo, ou manter como está.
- 🟡 BackBrain e 🟢 FrontBrain ajustarem conforme a decisão.

---

## Confirmar se `GET /api/dashboard/clientes` deve continuar global para o usuário normal

**Origem:** encontrado durante o card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23).

**Descrição:** `/api/dashboard/clientes` não recebeu escopo por vendedor. O usuário `normal` vê as métricas da base inteira de clientes: `total_clientes`, ativos, inativos, novos, por segmento e por UF. Com isso, ele fica sabendo o tamanho da base da empresa.

**Ação esperada:**
- 🟣 SecBrain avaliar se isso é intencional.
- Se não for, 🟡 BackBrain restringir à carteira do vendedor (ou tornar admin-only), e 🟢 FrontBrain ajustar o Dashboard.

---

## Teste manual do Dashboard no navegador (normal com vendedor, normal sem vendedor, admin)

**Origem:** 🟢 FrontBrain, durante o card "Pedido com cliente transferido / escopo do Dashboard" (2026-09-23).

**Descrição:** As mudanças em `frontend/src/app/dashboard/page.tsx` não foram validadas no navegador. As mudanças são:
- Rótulos "Minhas Vendas", "Meus Pedidos" e "Minha Meta".
- Card "Meu Desempenho" no lugar do ranking.
- Aviso para usuário sem vendedor vinculado.
- Tolerância a `null` e `[]`.

**Ação esperada:** 🔴 TestBrain (ou 🟢 FrontBrain) validar no navegador com três perfis:
- `normal` com vendedor: só os próprios números e a própria meta.
- `normal` sem vendedor: aviso, números zerados e gráfico com dias zerados.
- `admin`: todos os números e o ranking completo.

Registrar evidências e abrir bugs, se houver.
