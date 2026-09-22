# A Fazer

## Aplicar scope check (owner check + client scoping) em Create/Update de Pedidos

**Origem:** identificado por 🟣 SecBrain e confirmado por 🟡 BackBrain durante o card "Menu do vendedor: Visitas/Oportunidades + escopo por carteira; Produtos movido para Administração" (2026-09-22, ver `tarefas/feito.md`). Gap pré-existente em `pedido_handler.go`, fora do escopo daquele card.

**Descrição:** `CreatePedido`/`UpdatePedido` (`apis/rotaperfumes-api/handlers/pedido_handler.go`) aceitam `vendedor_id` diretamente do payload sem nenhuma validação de escopo — usuário `normal` pode, em tese, forjar/reatribuir um pedido para outro vendedor via `vendedor_id` do body. Diferente do padrão já aplicado em `oportunidade_handler.go`/`visita_handler.go` (e no restante do List/Get de pedidos), que usa `vendedorScope`/`resolverVendedorScope` (`apis/rotaperfumes-api/handlers/scope.go`) para forçar `vendedor_id` ao vendedor vinculado ao usuário logado e validar que o `cliente_id` pertence à carteira desse vendedor (`clienteNaCarteiraDoVendedor`).

**Ação esperada:** replicar em `CreatePedido`/`UpdatePedido` o mesmo padrão: para usuário `normal`, ignorar/forçar `vendedor_id` do payload ao vendedor vinculado, e validar `cliente_id` contra a carteira ativa desse vendedor (`400` se não pertencer). `GetPedido`/`UpdatePedido` por ID já devem ser revisados também quanto a owner check (404 para pedido de outro vendedor), seguindo o padrão de Oportunidades/Visitas.

---

## Rotas de Vendedores (`/api/vendedores*`) não são admin-only no backend

**Origem:** identificado por 🟢 FrontBrain (2026-09-22) durante o bugfix "menu 'Vendedores' aparecia para login de vendedor" (ver `tarefas/feito.md`).

**Descrição:** Após restringir no frontend o link/tela de "Vendedores" (dropdown Administração + `<ProtectedRoute requireAdmin>` em `app/admin/vendedores/page.tsx`) a usuários `admin`, foi identificado que todas as rotas `/api/vendedores*` (GET/POST/PUT/DELETE de vendedores e vínculo de clientes) no backend (`apis/rotaperfumes-api/routes/routes.go`) usam `middleware.JWTMiddleware(cfg, true, false)` — acesso comum, não admin-only. A proteção de frontend é só cosmética: um usuário `normal` autenticado ainda consegue chamar essas rotas diretamente (curl/Postman) e gerenciar vendedores.

**Ação esperada:** 🟣 SecBrain avaliar se a intenção é realmente restringir gestão de vendedores a admins (parece que sim, já que o menu foi para "Administração") e, se confirmado, 🟡 BackBrain restringir as rotas de escrita (POST/PUT/DELETE, vínculo/desvínculo de clientes) a `requireAdmin=true`, mantendo GET como está hoje se algum fluxo de vendedor depender de listar vendedores (checar antes de restringir GET também).
