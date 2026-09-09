# Concluídas ✅

> Histórico de tarefas finalizadas.

---

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
