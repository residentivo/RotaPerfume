# Roteiro de teste manual: Clientes (SEC-01)

**Card:** SEC-01, IDOR em Create/Update/Toggle de clientes (`tarefas/fazendo.md`)
**Tela:** `frontend/src/app/admin/clientes/page.tsx` (+ `frontend/src/components/admin/ClienteModal.tsx`)
**Endpoints:** `GET /api/clientes`, `POST /api/clientes`, `PUT /api/clientes/{id}`, `PATCH /api/clientes/{id}/inativar`
**Contrato (backend, `apis/rotaperfumes-api/handlers/cliente_handler.go`):**

| Perfil | Criar | Editar / Inativar cliente da própria carteira | Editar / Inativar cliente fora da carteira |
| --- | --- | --- | --- |
| admin | 201, sem vínculo de carteira | 200 | 200 |
| normal com vendedor | 201, e o cliente já nasce **na carteira** do vendedor | 200 | **404** `cliente não encontrado` |
| normal sem vendedor | **403** `usuário sem vendedor vinculado` | - (a lista vem vazia) | **404** `cliente não encontrado` |
| normal com vendedor desligado | **403** `acesso bloqueado: vendedor desligado` | 403 | 403 |

**Autor:** TestBrain (2026-09-24)

Marque cada checkbox depois de conferir no navegador, com o DevTools aberto na aba **Network**.

---

## 0. Preparação

1. Suba a API e o frontend (`make dev-api` e `make dev-frontend`).
2. Usuários:

   | Perfil | Usuário (e-mail) | id | Vendedor |
   | --- | --- | --- | --- |
   | admin | `admin@rotaperfumes.com.br` | 1 | - |
   | normal com vendedor | `rafael.carvalho@rotaperfumes.com.br` | 5 | 4 - Rafael Carvalho |
   | normal com vendedor desligado | `henrique.rodrigues@rotaperfumes.com.br` | 2 | 1 - Henrique Rodrigues |
   | normal sem vendedor | criar temporário (SQL abaixo) | - | nenhum |

3. Usuário temporário sem vendedor (defina a senha pela tela de Usuários como admin):
   ```sql
   INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo)
   VALUES ('QA Sem Vendedor', 'qa.semvendedor@rotaperfumes.test', '!definir-pela-tela!', 'normal', NULL, 1);
   ```
4. Anote um cliente **da carteira** do vendedor 4 (ID_DENTRO) e um **fora** dela (ID_FORA):
   ```sql
   -- Dentro da carteira ativa do vendedor 4
   SELECT c.cliente_id_origem, c.razao_social, c.ativo
   FROM clientes c JOIN carteiras k ON k.cliente_id = c.cliente_id_origem
   WHERE k.vendedor_id = 4 AND k.data_fim IS NULL LIMIT 1;

   -- Fora da carteira ativa do vendedor 4
   SELECT c.cliente_id_origem, c.razao_social, c.ativo
   FROM clientes c
   WHERE c.cliente_id_origem NOT IN (SELECT cliente_id FROM carteiras WHERE vendedor_id = 4 AND data_fim IS NULL)
   LIMIT 1;
   ```
5. **Limpeza ao final:** veja a seção 6.

---

## 1. Botão "+ Novo Cliente"

- [ ] 1.1 **Admin:** o botão fica habilitado e sem texto de motivo.
- [ ] 1.2 **Normal com vendedor (id 5):** o botão fica habilitado.
- [ ] 1.3 **Normal sem vendedor:** o botão fica **desabilitado**. Abaixo dele aparece "Usuario sem vendedor vinculado: solicite o vinculo a um administrador." (o mesmo texto aparece no tooltip). Clicar não abre o modal.
- [ ] 1.4 **Normal com vendedor desligado:** a tela da carteira mostra só o aviso de vendedor desligado (CarteiraGuard). Se o aviso não aparecer (ex.: sessão ainda não revalidada), o botão fica desabilitado com "Vendedor desligado: cadastro de clientes bloqueado.".
- [ ] 1.5 Logo depois do login, enquanto o `/api/auth/me` não respondeu, o botão fica desabilitado por um instante e depois habilita (normal com vendedor ou admin). Veja a observação na seção 7.

## 2. Criar cliente (normal com vendedor, id 5)

- [ ] 2.1 Clique em "+ Novo Cliente". O modal abre **vazio**, com a data de cadastro de hoje.
- [ ] 2.2 Preencha razão social, CNPJ, segmento, cidade e UF e clique em "Criar cliente". Aparece "Cliente "<razão>" criado com sucesso." e o modal fecha.
- [ ] 2.3 **O cliente novo aparece na lista do próprio usuário** (a lista é recarregada: há uma nova chamada `GET /api/clientes` depois do `POST`). Use a busca pela razão social para achá-lo.
- [ ] 2.4 Conferência no banco: o cliente nasceu na carteira do vendedor 4.
   ```sql
   SELECT k.vendedor_id, k.data_inicio, k.data_fim FROM carteiras k
   WHERE k.cliente_id = <id do cliente criado>;
   -- esperado: vendedor_id = 4, data_inicio = hoje, data_fim = NULL
   ```
- [ ] 2.5 Reabra "+ Novo Cliente": o modal abre **vazio** outra vez, sem os dados do cadastro anterior.

## 3. Editar / inativar cliente da própria carteira (id 5)

- [ ] 3.1 Clique no nome de ID_DENTRO. O modal "Editar Cliente" abre **preenchido** com os dados dele.
- [ ] 3.2 Altere o bairro e salve. Aparece "Cliente "<razão>" atualizado com sucesso." e a linha é atualizada (`PUT` → 200).
- [ ] 3.3 Clique no status "Ativo" de ID_DENTRO e confirme. Aparece "Cliente inativado com sucesso." (`PATCH` → 200). Clique de novo e confirme para **reativar**.
- [ ] 3.4 Cancelar a confirmação não dispara nenhuma chamada.

## 4. Cliente fora da carteira: 404 (normal com vendedor, id 5)

A lista do usuário normal só mostra a carteira dele, então o cenário é simulado de duas formas.

**4a. Pelo Console do DevTools** (os cookies HttpOnly vão sozinhos):
```js
const API = "http://localhost:8080";
await fetch(`${API}/api/clientes/ID_FORA`, { method: "PUT", credentials: "include",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ razao_social: "X", cnpj: "00000000000000", segmento: "X", cidade: "X", uf: "SP", bairro: "", data_cadastro: "2026-01-01" }) })
  .then(r => r.status + " " + r.statusText);
await fetch(`${API}/api/clientes/ID_FORA/inativar`, { method: "PATCH", credentials: "include",
  headers: { "Content-Type": "application/json" }, body: JSON.stringify({ ativo: false }) })
  .then(async r => r.status + " " + await r.text());
```
- [ ] 4.1 As duas chamadas respondem **404** `{"success":false,"error":"cliente não encontrado"}`.
- [ ] 4.2 No banco, ID_FORA **não** foi alterado (`SELECT razao_social, ativo FROM clientes WHERE cliente_id_origem = ID_FORA;`).

**4b. Pela tela (cliente que sai da carteira com a tela aberta):**
- [ ] 4.3 Com a lista aberta como id 5, o admin encerra o vínculo de ID_DENTRO em outra janela (Vendedores → Vend 4 → "Remover" no cliente) ou por SQL (`UPDATE carteiras SET data_fim = CURDATE() WHERE vendedor_id = 4 AND cliente_id = ID_DENTRO AND data_fim IS NULL;`).
- [ ] 4.4 Sem recarregar, o usuário 5 clica em "Editar" em ID_DENTRO e salva. O modal **fecha**, aparece o alerta vermelho "cliente não encontrado" e a lista é **recarregada**, já sem ID_DENTRO.
- [ ] 4.5 Mesmo cenário pelo botão de status (inativar): aparece "cliente não encontrado" e a lista é recarregada.
- [ ] 4.6 **Desfaça** o vínculo removido (seção 6).

## 5. Outros perfis

- [ ] 5.1 **Normal sem vendedor:** a lista vem vazia ("Nenhum cliente cadastrado."). No Console, `PUT`/`PATCH` em qualquer id respondem **404** e `POST /api/clientes` responde **403** `usuário sem vendedor vinculado`.
- [ ] 5.2 **Normal com vendedor desligado (id 2):** `/admin/clientes` mostra só o aviso de desligado. No Console, `POST`, `PUT` e `PATCH` respondem **403** `acesso bloqueado: vendedor desligado`.
- [ ] 5.3 **Admin:** edita e inativa ID_FORA normalmente (200). Criar como admin **não** cria vínculo de carteira.

## 6. Limpeza

- [ ] 6.1 Usuário temporário removido: `DELETE FROM usuarios WHERE email = 'qa.semvendedor@rotaperfumes.test';`
- [ ] 6.2 Vínculo de ID_DENTRO restaurado se o item 4.3 foi executado:
   ```sql
   UPDATE carteiras SET data_fim = NULL
   WHERE vendedor_id = 4 AND cliente_id = ID_DENTRO AND data_fim = CURDATE();
   ```
- [ ] 6.3 Clientes de teste criados na seção 2 removidos ou inativados (anote os ids).

## 7. Observações

- Enquanto o `/me` não responde, o botão desabilitado mostra o motivo "Usuario sem vendedor vinculado..." mesmo para quem tem vendedor. O texto some assim que a sessão carrega. É só um detalhe de texto (ver o relatório do TestBrain).

## Testes automatizados

`cd frontend && npm test` cobre:

- `frontend/src/app/admin/clientes/page.test.tsx`: botão habilitado ou desabilitado por perfil, com o motivo; `/me` ainda não carregado; criar e recarregar a lista; 403 com a mensagem da API e o texto padrão; 404 ao editar ou inativar (fecha o modal, recarrega e mostra a mensagem); editar e inativar com sucesso; cancelar a confirmação.
- `frontend/src/app/listasPaginadas.test.tsx`: loading, paginação, ordenação, filtro voltando para a página 1 e erro da API.
- `frontend/src/components/admin/FormModais.test.tsx` (ClienteModal): abre resetado em "novo", preenchido em "editar", reabre resetado e limpa o erro anterior.
