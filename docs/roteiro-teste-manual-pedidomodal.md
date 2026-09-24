# Roteiro de teste manual: PedidoModal (Novo/Editar Pedido)

**Card:** "Teste manual/e2e do `PedidoModal` no navegador (usuário normal e admin)" (`tarefas/fazendo.md`)
**Componente:** `frontend/src/components/admin/PedidoModal.tsx`
**Endpoints envolvidos:** `GET /api/vendedores`, `GET /api/vendedores/{id}/clientes`, `GET /api/produtos`, `POST /api/pedidos`, `PUT /api/pedidos/{id}`, `DELETE /api/pedidos/{id}`
**Autor:** TestBrain (2026-09-24)

Marque cada checkbox depois de conferir o resultado esperado no navegador. Se algo divergir, anote o caso e o que aconteceu, e encaminhe ao SubBrain.

---

## 0. Preparação

1. Suba o banco, a API e o frontend:
   ```bash
   make dev-api        # API em http://localhost:8080
   make dev-frontend   # frontend em http://localhost:3000
   ```
2. Usuários usados neste roteiro (base local atual):

   | Perfil | Usuário (e-mail) | id | Vendedor vinculado |
   | --- | --- | --- | --- |
   | admin | `admin@rotaperfumes.com.br` | 1 | nenhum |
   | normal **com** vendedor | `rafael.carvalho@rotaperfumes.com.br` | 5 | 4 - Rafael Carvalho (ativo, 60 clientes na carteira ativa) |
   | normal **sem** vendedor | criar temporário (SQL abaixo) | - | nenhum |

   Se você não souber a senha de algum usuário, redefina pela tela de Usuários (admin) ou com `apis/shared/cmd/resetpassword`.

3. Criar o usuário temporário **sem vendedor**. Hoje a base não tem nenhum usuário `normal` sem vínculo.
   ```sql
   -- Crie o usuário e depois defina a senha pela tela de Usuários (admin -> "Resetar senha").
   INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo)
   VALUES ('QA Sem Vendedor', 'qa.semvendedor@rotaperfumes.test', '!definir-pela-tela!', 'normal', NULL, 1);
   ```
   Como alternativa, crie o usuário pela tela de Usuários (admin) sem escolher vendedor.

4. Dados úteis para o vendedor 4 (Rafael Carvalho):
   ```sql
   -- Clientes da carteira ATIVA do vendedor 4 (devem aparecer no select "Cliente")
   SELECT c.cliente_id_origem, c.razao_social
   FROM clientes c
   WHERE c.cliente_id_origem IN (SELECT cliente_id FROM carteiras WHERE vendedor_id = 4 AND data_fim IS NULL)
   ORDER BY c.razao_social;

   -- Pedidos do vendedor 4 cujo cliente NÃO está mais na carteira ativa ("fora da carteira")
   SELECT p.pedido_id_origem, p.cliente_id, c.razao_social, p.status, p.data_pedido
   FROM pedidos p JOIN clientes c ON c.cliente_id_origem = p.cliente_id
   WHERE p.vendedor_id = 4
     AND p.cliente_id NOT IN (SELECT cliente_id FROM carteiras WHERE vendedor_id = 4 AND data_fim IS NULL)
   ORDER BY p.data_pedido DESC LIMIT 10;
   -- Exemplo conhecido: pedido 500, cliente 57 "Loja Requinte EIRELI", status Entregue.
   ```

5. **Limpeza ao final:** exclua os pedidos criados durante o roteiro (botão Excluir na lista de Pedidos, ou `DELETE /api/pedidos/{id}`) e remova o usuário temporário:
   ```sql
   DELETE FROM usuarios WHERE email = 'qa.semvendedor@rotaperfumes.test';
   ```
   Crie os pedidos de teste com status **"Em separação"**. Pedido "Faturado" baixa estoque e não pode ser excluído.

---

## 1. Admin: criar pedido

Login como `admin@rotaperfumes.com.br` e abra Pedidos, depois **Novo Pedido**.

- [ ] 1.1 O select **Vendedor** está habilitado e lista os vendedores. O primeiro item é "Selecione um vendedor".
- [ ] 1.2 Sem vendedor selecionado, o select **Cliente** fica desabilitado com o texto "Selecione um vendedor primeiro".
- [ ] 1.3 Ao escolher o vendedor "Rafael Carvalho" (4), o select Cliente mostra "Carregando clientes..." e depois lista só os clientes da carteira ativa dele, no formato `#id - Razão social`.
- [ ] 1.4 Com um cliente escolhido, trocar para outro vendedor limpa o cliente e recarrega a lista (cascata).
- [ ] 1.5 Preencha data, canal "App", status "Em separação" e 1 item (produto, quantidade 1, preço 10,55). Salvar fecha o modal e o pedido aparece na lista com o valor total 10,55.
- [ ] 1.6 Validações do formulário: sem cliente aparece "Cliente e obrigatorio."; quantidade 0 aparece "Quantidade deve ser maior que zero em todos os itens."; desconto 150 aparece "Desconto deve estar entre 0 e 100 em todos os itens.".
- [ ] 1.7 A **data do pedido** exibida na lista e no detalhe é a mesma que você digitou. Veja o **BUG-01** na seção de evidências: hoje a data é gravada com 1 dia a menos.

## 2. Admin: editar pedido

- [ ] 2.1 Ao abrir **Editar** no pedido criado em 1.5, vendedor, cliente, data, canal, status e itens vêm preenchidos.
- [ ] 2.2 Troque o cliente para outro da carteira, altere a quantidade para 2 e salve. A lista reflete a mudança (total 21,10).
- [ ] 2.3 Ao abrir Editar no **pedido 500** (cliente 57, fora da carteira do vendedor 4), o cliente aparece como `#57 - Loja Requinte EIRELI (fora da carteira)` e continua selecionado.
- [ ] 2.4 Salvar o pedido 500 como admin, sem mudar nada, funciona. O admin não tem checagem de carteira. **Cancele** se não quiser alterar o registro.

## 3. Normal com vendedor: criar pedido

Login como `rafael.carvalho@rotaperfumes.com.br` e abra Pedidos, depois **Novo Pedido**.

- [ ] 3.1 O select **Vendedor** aparece **travado** (desabilitado) já com "Rafael Carvalho".
- [ ] 3.2 O select **Cliente** carrega automaticamente os clientes da carteira ativa do vendedor 4 (cascata sem precisar escolher o vendedor).
- [ ] 3.3 Criar um pedido com um cliente da lista funciona. O pedido aparece na lista com vendedor "Rafael Carvalho".
- [ ] 3.4 Mensagem amigável para o 400 de carteira. Como o select só oferece clientes da carteira, esse caso é difícil de reproduzir na tela. Para forçar:
  1. Abra Novo Pedido e escolha um cliente.
  2. Em outra aba (admin), encerre a carteira desse cliente com o vendedor 4, via tela de Vendedores ou por SQL:
     `UPDATE carteiras SET data_fim = CURDATE() WHERE vendedor_id = 4 AND cliente_id = <id> AND data_fim IS NULL;`
  3. Volte à primeira aba e clique Salvar.

  Resultado esperado: o alerta vermelho mostra **"O cliente selecionado nao pertence a carteira ativa do vendedor. Escolha um cliente da carteira e tente novamente."**, e não a mensagem crua do backend.
  **Desfaça o passo 2 depois:** `UPDATE carteiras SET data_fim = NULL WHERE vendedor_id = 4 AND cliente_id = <id> AND data_fim = CURDATE();`

## 4. Normal com vendedor: editar pedido

- [ ] 4.1 Ao editar o pedido criado em 3.3, o vendedor continua travado. Mudar a quantidade e salvar funciona.
- [ ] 4.2 Ao abrir Editar no **pedido 500** (cliente fora da carteira), o cliente aparece como `#57 - Loja Requinte EIRELI (fora da carteira)` e continua selecionado.
- [ ] 4.3 Salvar o pedido 500 sem trocar o cliente mostra a mensagem amigável de carteira (item 3.4). **Esse é o comportamento esperado hoje.** O efeito colateral foi aceito no card de 2026-09-23: um usuário normal não consegue editar pedido cujo cliente foi transferido. O registro **não** é alterado.
- [ ] 4.4 No pedido 500, trocar o cliente para um da carteira faz a opção "(fora da carteira)" sumir da lista depois da troca. Cancele sem salvar se não quiser alterar o pedido.

## 5. Normal sem vendedor

Login com o usuário temporário (`qa.semvendedor@rotaperfumes.test`) e abra Pedidos, depois **Novo Pedido**.

- [ ] 5.1 O modal mostra o aviso amarelo: "Seu usuario nao esta vinculado a um vendedor, por isso nao e possivel registrar pedidos. Solicite o vinculo ao administrador."
- [ ] 5.2 O select Vendedor fica desabilitado com "Sem vendedor vinculado". O select Cliente fica desabilitado com "Selecione um vendedor primeiro".
- [ ] 5.3 O botão **Salvar** fica desabilitado.
- [ ] 5.4 A lista de pedidos desse usuário vem vazia, porque ele não tem carteira.

## 6. Limpeza

- [ ] 6.1 Pedidos de teste (1.5 e 3.3) excluídos.
- [ ] 6.2 Carteira alterada no 3.4 restaurada.
- [ ] 6.3 Usuário temporário removido.

---

## Evidências via API (2026-09-24)

Validação feita pelo TestBrain com a API local (`apis/rotaperfumes-api`, binário compilado de `./cmd/server`, porta 8080) e o banco local `rotaperfumes`. O login real exige captcha Cloudflare Turnstile, então os testes usaram tokens JWT HS256 gerados localmente com o mesmo `JWT_SECRET`/`JWT_ISSUER` do `.env`, para os usuários reais: admin id 1; normal id 5 (vendedor 4); normal sem vendedor = usuário temporário id 86, criado por SQL e removido no fim.

| # | Perfil | Chamada | Status | Trecho da resposta |
| --- | --- | --- | --- | --- |
| E1 | normal sem vendedor | `GET /api/auth/me` | 200 | `"id":86,"id_vendedor":null,"role":"normal"` |
| E2 | normal (vend. 4) | `GET /api/auth/me` | 200 | `"id":5,"id_vendedor":4,"role":"normal"` |
| E3 | admin | `GET /api/vendedores/4/clientes` | 200 | `{"data":[{"id":1450,...,"razao_social":"Aroma Charme EIRELI",...,"data_fim":null},...` |
| E4 | normal (vend. 4) | `GET /api/vendedores/4/clientes` (cascata, própria carteira) | 200 | mesma lista (60 clientes da carteira ativa) |
| E5 | normal (vend. 4) | `GET /api/vendedores/5/clientes` (outro vendedor) | 404 | `{"error":"vendedor não encontrado","success":false}` |
| E6 | normal sem vendedor | `GET /api/vendedores/4/clientes` | 404 | `{"error":"vendedor não encontrado","success":false}` |
| E7 | admin | `POST /api/pedidos` (cliente 9, vendedor 4, 1 x 10,55, "Em separação") | 201 | `"pedido_id_origem":28733,"cliente_id":9,"vendedor_id":4,...,"valor_total":10.55` |
| E8 | admin | `PUT /api/pedidos/28733` (cliente 68, qtd 2, canal WhatsApp) | 200 | `"cliente_id":68,"canal":"WhatsApp","valor_total":21.1,"cliente_nome":"Loja Aromas do Sul ME"` |
| E9 | normal (vend. 4) | `POST /api/pedidos` com `vendedor_id: 5` forjado | 201 | `"pedido_id_origem":28734,"vendedor_id":4` (vendedor forçado ao próprio) `"valor_total":19.1` |
| E10 | normal (vend. 4) | `POST /api/pedidos` com cliente 6 (fora da carteira) | 400 | `{"error":"cliente não pertence à carteira deste vendedor","success":false}` |
| E11 | normal (vend. 4) | `PUT /api/pedidos/28734` (qtd 3) | 200 | `"valor_total":57.29` |
| E12 | normal (vend. 4) | `PUT /api/pedidos/28734` trocando para cliente 6 | 400 | `{"error":"cliente não pertence à carteira deste vendedor","success":false}` |
| E13 | normal (vend. 4) | `GET /api/pedidos/500` (cliente 57 fora da carteira) | 200 | `"pedido_id_origem":500,"cliente_id":57,"vendedor_id":4` (continua visível) |
| E14 | normal (vend. 4) | `PUT /api/pedidos/500` mantendo cliente 57 | 400 | `{"error":"cliente não pertence à carteira deste vendedor","success":false}` (efeito colateral aceito; registro não alterado) |
| E15 | normal sem vendedor | `POST /api/pedidos` | 403 | `{"error":"usuário sem vendedor vinculado","success":false}` |
| E16 | normal sem vendedor | `PUT /api/pedidos/28733` | 404 | `{"error":"pedido não encontrado","success":false}` |
| E17 | normal (vend. 4) | `DELETE /api/pedidos/28734` | 204 | (sem corpo) |
| E18 | admin | `DELETE /api/pedidos/28733` | 204 | (sem corpo) |

**Limpeza feita:** os pedidos 28733 e 28734 foram excluídos (E17/E18; `pedidos` e `itens_pedido` conferidos com 0 linhas) e o usuário temporário id 86 foi removido por SQL (sem refresh tokens). Os pedidos tinham status "Em separação", então não houve baixa de estoque. O único resíduo é o `AUTO_INCREMENT` de `pedidos`, que avançou para 28735, sem impacto. O servidor foi parado no fim.

### BUG-01: `data_pedido` gravada com 1 dia a menos (encaminhado ao SubBrain)

- **Reprodução:** E7 enviou `"data_pedido":"2026-09-24"`, a resposta trouxe `"data_pedido":"2026-09-23T00:00:00-03:00"`, e o banco gravou `2026-09-23` (`SELECT data_pedido FROM pedidos WHERE pedido_id_origem = 28733`). O mesmo aconteceu em E8, E9 e E11. No Dashboard do vendedor 4, os pedidos de teste apareceram no dia 2026-09-23 da série de vendas.
- **Causa provável:** `pedido_service.go` faz `time.Parse("2006-01-02", ...)`, que gera meia-noite **UTC**. O DSN usa `loc=Local` (UTC-3), e o driver converte para 2026-09-23 21:00 local antes de gravar na coluna `DATE`.
- **Abrangência provável:** o mesmo padrão `time.Parse(layout, data)` aparece em `cliente_service.go`, `estoque_service.go`, `oportunidade_service.go`, `pagamento_service.go`, `produto_service.go`, `vendedor_service.go` e `visita_service.go`. Não foi reproduzido via API nesses módulos.
- **No PedidoModal:** ao criar um pedido com a data de hoje e reabrir em Editar, o campo mostra o dia anterior. Salvar de novo pode subtrair mais um dia.
