# Smoke test pós-refatoração FE-03

**Card:** FE-03, remoção de `setState` síncrono em `useEffect` (`tarefas/fazendo.md`). A refatoração mexeu na carga das listagens e no reset dos modais de 10 telas e 9 modais, e criou o hook `useResetOnOpen`.
**Tempo estimado:** 15 a 20 minutos, logado como **admin** (e como normal com vendedor onde indicado).
**Autor:** TestBrain (2026-09-24)

Em cada tela, com o DevTools aberto na aba **Network**:

1. **Abrir:** aparece o spinner e depois a lista, sem erro vermelho.
2. **Paginar:** ">" e "<" trocam a página, com spinner, e a numeração "Pagina N de M" acompanha.
3. **Ordenar:** clicar em um cabeçalho ordena; clicar de novo inverte a seta.
4. **Filtrar estando na página 2:** a tela volta para a página 1 com o filtro aplicado.
5. **Modal "Novo":** abre vazio. Digite algo, feche, abra de novo: continua vazio.
6. **Modal "Editar":** abre preenchido. Feche e abra "Novo": nada do registro anterior sobra.

| # | Tela | URL | Além dos passos 1 a 6, conferir |
| --- | --- | --- | --- |
| 1 | Pedidos | `/admin/pedidos` | "Itens" abre o detalhe e "Ocultar itens" fecha. No modal, trocar o vendedor recarrega os clientes ("Carregando clientes..." some). Voltar para "Selecione um vendedor" **não** deixa o select de cliente travado em "Carregando". |
| 2 | Pagamentos | `/pagamentos` | No modal "Novo", a busca de pedido filtra depois de uma pausa na digitação. Ao reabrir, a busca e a seleção vêm vazias. |
| 3 | Clientes | `/admin/clientes` | Roteiro completo em `docs/roteiro-teste-manual-clientes.md`. |
| 4 | Oportunidades | `/admin/oportunidades` | Mesma checagem de cascata vendedor → cliente do item 1. Etapa "Fechado perdido" exige o motivo. Como **normal com vendedor**, o filtro de vendedor não aparece e a lista vem só da carteira dele. |
| 5 | Visitas | `/admin/visitas` | Mesma checagem de cascata e de usuário normal do item 4. |
| 6 | Estoque | `/admin/estoque` | "+ Novo registro" aparece para o admin. Editar altera só o saldo. **Mudança do FE-03:** o papel de admin vem do `/api/auth/me`, e não do `localStorage`. |
| 7 | Produtos | `/admin/produtos` | Inativar e reativar pelo status. |
| 8 | Usuários | `/admin/usuarios` | Na página 2, mudar "Perfil" volta para a página 1 com **uma** requisição `GET /api/usuarios` só (antes eram duas). Na página 1, mudar o filtro **não** faz requisição, porque o filtro é local. |
| 9 | Vendedores | `/admin/vendedores` | A lista carrega uma vez só (paginação local). Editar mostra "Clientes vinculados". Fechar e abrir "Novo" não mostra a lista do vendedor anterior. |
| 10 | Histórico de senha | `/admin/senha-historico` | Mudar "Tipo de Reset" na página 2 volta para a página 1 com **uma** requisição só. "Atualizar" recarrega. |
| 11 | Dashboard | `/dashboard` | Hoje / Semana / Mês e o select de dias recarregam. Detalhes do FE-02 em `docs/roteiro-teste-manual-dashboard.md` (seção 5b). |

**Bug conhecido (FE-04, reportado ao Analista):** as listagens não descartam respostas atrasadas.
- Com a rede em "Slow 3G", clicar ">" duas vezes seguidas pode deixar a tabela com os dados da página 2 e o paginador na página 3.
- Clicar ">" menos de meio segundo depois de abrir a tela pode trazer de volta os dados da página 1.

Se isso acontecer, anote a tela, mas não é regressão do FE-03: o comportamento já existia antes.

Cobertura automatizada destes passos:
- `frontend/src/app/listasPaginadas.test.tsx`
- `frontend/src/app/crudPaginas.test.tsx`
- `frontend/src/app/admin/listasAdmin.test.tsx`
- `frontend/src/app/admin/crudAdmin.test.tsx`
- `frontend/src/components/admin/CascataVendedorCliente.test.tsx`
- `frontend/src/components/admin/FormModais.test.tsx`
- `frontend/src/components/PagamentoModal.test.tsx`
- `frontend/src/components/admin/PedidoModal.test.tsx`
- `frontend/src/lib/useResetOnOpen.test.ts`
