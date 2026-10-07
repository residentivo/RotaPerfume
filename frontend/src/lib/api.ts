import {
  LoginRequest,
  LoginResponse,
  ResetPasswordRequest,
  User,
  MeResponse,
  UserRole,
  DashboardMetrics,
  VendasSeries,
  VendedorRanking,
  DashboardPeriodo,
  SenhaHistoricoResponse,
  TipoReset,
  Vendedor,
  VendedorCompleto,
  VendedorDetalhe,
  VendedorInput,
  ClienteResumo,
  Cliente,
  ClienteInput,
  ClienteDashboardMetrics,
  Produto,
  ProdutoInput,
  Pedido,
  PedidoDetalhe,
  PedidoInput,
  Pagamento,
  PagamentoCreateInput,
  PagamentoUpdateInput,
  ListPagamentosFilters,
  Oportunidade,
  OportunidadeInput,
  ListOportunidadesFilters,
  Visita,
  VisitaInput,
  ListVisitasFilters,
  Estoque,
  EstoqueInput,
  ListEstoqueFilters,
} from "./types";
import { fetchEnvelopeWithAuth, fetchWithAuth } from "./apiClient";
import { vlog } from "./vlog";

// LOG-02: logs verbose daqui nunca recebem senha, captcha, tokens, corpo de
// request/response nem valores de filtros digitados pelo usuario.
const F = "api.ts";

export interface CreateUserRequest {
  nome: string;
  email: string;
  role: UserRole;
  id_vendedor?: number | null;
}

export interface UpdateUserRequest {
  nome?: string;
  role?: UserRole;
  id_vendedor?: number | null;
}

// Resposta de POST /api/usuarios: usuário criado + flag de envio do email
// com a senha inicial gerada aleatoriamente pelo backend.
export interface CreateUserResponse extends User {
  email_enviado: boolean;
}

// Resposta de POST /api/admin/reset-password.
export interface AdminResetPasswordResponse {
  sucesso: boolean;
  mensagem: string;
  email_enviado: boolean;
}

export interface ListUsersResponse {
  data: User[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

// Remove barra(s) final(is): NEXT_PUBLIC_API_URL pode vir com sufixo de caminho
// (ex. https://host:8443/api) e os endpoints ja comecam com "/api/...".
const API_BASE = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/+$/, "");

interface Paginado<L extends unknown[]> {
  data: L;
  page: number;
  limit: number;
  total: number;
  pages: number;
}

/**
 * GET de listagem paginada via `fetchEnvelopeWithAuth` (FE-05): passa pelo
 * refresh automatico no 401 e pelo tratamento de erro do `fetchWithAuth`
 * (ApiError com status, 403 de vendedor desligado), mas recebe o corpo
 * inteiro para ler a paginacao. Normaliza os formatos aceitos:
 * - `{ data, pagination: { page, limit, total, pages } }`
 * - `{ data, page, limit, total, pages }`
 * - array puro
 * `page`/`limit` pedidos entram como padrao quando a resposta nao informa.
 */
async function listarPaginado<L extends unknown[]>(
  path: string,
  page: number,
  limit: number
): Promise<Paginado<L>> {
  vlog(F, "listarPaginado", "buscando listagem paginada, página/limite:", page, limit);
  const parsed = await fetchEnvelopeWithAuth<unknown>(path, { method: "GET" });

  vlog(F, "listarPaginado", "verificando se a resposta é um objeto (envelope)");
  if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
    vlog(F, "listarPaginado", "convertendo envelope para Record");
    const d = parsed as Record<string, unknown>;
    vlog(F, "listarPaginado", "verificando se o envelope tem array data");
    if ("data" in d && Array.isArray(d.data)) {
      vlog(F, "listarPaginado", "lendo paginação do envelope, linhas:", (d.data as unknown[]).length);
      const pag = (d.pagination && typeof d.pagination === "object"
        ? (d.pagination as Record<string, unknown>)
        : d) as Record<string, unknown>;
      return {
        data: d.data as L,
        page: Number(pag.page ?? page),
        limit: Number(pag.limit ?? limit),
        total: Number(pag.total ?? (d.data as unknown[]).length),
        pages: Number(pag.pages ?? 1),
      };
    }
  }

  vlog(F, "listarPaginado", "verificando se a resposta é array puro");
  if (Array.isArray(parsed)) {
    return { data: parsed as L, page, limit, total: parsed.length, pages: 1 };
  }
  return { data: [] as unknown[] as L, page, limit, total: 0, pages: 0 };
}

export async function apiLogin(
  email: string,
  password: string,
  captchaToken?: string
): Promise<LoginResponse> {
  vlog(F, "apiLogin", "montando corpo do login (credenciais não logadas)");
  const body: LoginRequest = { email, password, captchaToken };
  return fetchWithAuth<LoginResponse>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify(body),
    noRefresh: true,
  });
}

export async function apiRefreshToken(refresh_token: string): Promise<{
  access_token: string;
  refresh_token?: string;
}> {
  return fetchWithAuth<{
    access_token: string;
    refresh_token?: string;
  }>("/api/auth/refresh", {
    method: "POST",
    body: JSON.stringify({ refresh_token }),
    noRefresh: true,
  });
}

export async function apiMe(): Promise<MeResponse> {
  return fetchWithAuth<MeResponse>("/api/auth/me", {
    method: "GET",
  });
}

export async function apiResetPassword(
  usuarioId: number,
  novaSenha: string,
  senhaAtual?: string
): Promise<{ message: string }> {
  vlog(F, "apiResetPassword", "montando corpo do reset de senha, usuario_id:", usuarioId);
  const body: ResetPasswordRequest = {
    usuario_id: usuarioId,
    nova_senha: novaSenha,
  };
  vlog(F, "apiResetPassword", "verificando se senha atual foi informada:", !!senhaAtual);
  if (senhaAtual) {
    vlog(F, "apiResetPassword", "incluindo senha atual no corpo");
    body.senha_atual = senhaAtual;
  }
  return fetchWithAuth<{ message: string }>("/api/auth/reset-password", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function apiChangePassword(
  senhaAtual: string,
  novaSenha: string,
  captchaToken?: string
): Promise<{ message: string }> {
  return fetchWithAuth<{ message: string }>("/api/auth/reset-password", {
    method: "POST",
    body: JSON.stringify({
      senha_atual: senhaAtual,
      nova_senha: novaSenha,
      captchaToken,
    }),
  });
}

// GET /api/usuarios — lista paginada. Retorna envelope {data, page, limit, total, pages}.
export async function apiListUsers(
  page = 1,
  limit = 20,
  orderBy?: string,
  orderDir?: "asc" | "desc"
): Promise<ListUsersResponse> {
  vlog(F, "apiListUsers", "montando query string");
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  vlog(F, "apiListUsers", "adicionando order_by na query se informado");
  if (orderBy) qs.set("order_by", orderBy);
  vlog(F, "apiListUsers", "adicionando order_dir na query se informado");
  if (orderDir) qs.set("order_dir", orderDir);

  return listarPaginado<User[]>(`/api/usuarios?${qs.toString()}`, page, limit);
}

// POST /api/usuarios — admin cria novo usuario
// A senha inicial é gerada aleatoriamente pelo backend e enviada por email
// ao endereço cadastrado — nunca retornada pela API. `email_enviado` indica
// se o envio deu certo.
export async function apiCreateUser(
  data: CreateUserRequest
): Promise<CreateUserResponse> {
  return fetchWithAuth<CreateUserResponse>("/api/usuarios", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// PUT /api/usuarios/{id} — admin atualiza nome e/ou role
export async function apiUpdateUser(
  id: number,
  data: UpdateUserRequest
): Promise<User> {
  return fetchWithAuth<User>(`/api/usuarios/${id}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

// PATCH /api/usuarios/{id}/inativar — alterna ativo (toggle)
export async function apiToggleUserStatus(
  id: number,
  ativo: boolean
): Promise<User> {
  return fetchWithAuth<User>(`/api/usuarios/${id}/inativar`, {
    method: "PATCH",
    body: JSON.stringify({ ativo }),
  });
}

// POST /api/admin/reset-password — admin reseta senha de um usuario.
// A nova senha é gerada aleatoriamente pelo backend e enviada por email ao
// endereço cadastrado do usuário — nunca retornada pela API. `email_enviado`
// indica se o envio deu certo.
export async function apiAdminResetPassword(
  usuario_id: number
): Promise<AdminResetPasswordResponse> {
  return fetchWithAuth<AdminResetPasswordResponse>(
    "/api/admin/reset-password",
    {
      method: "POST",
      body: JSON.stringify({ usuario_id }),
    }
  );
}

// === Dashboard ===

export async function apiDashboardMetrics(
  periodo: DashboardPeriodo = "month"
): Promise<DashboardMetrics> {
  vlog(F, "apiDashboardMetrics", "montando query string");
  const qs = new URLSearchParams({ periodo });
  return fetchWithAuth<DashboardMetrics>(
    `/api/dashboard/metrics?${qs.toString()}`,
    {
      method: "GET",
    }
  );
}

export async function apiDashboardVendas(dias = 30): Promise<VendasSeries> {
  vlog(F, "apiDashboardVendas", "montando query string");
  const qs = new URLSearchParams({ dias: String(dias) });
  return fetchWithAuth<VendasSeries>(
    `/api/dashboard/vendas?${qs.toString()}`,
    {
      method: "GET",
    }
  );
}

export interface VendedoresRankingResponse {
  data: VendedorRanking[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

export async function apiDashboardVendedores(
  page = 1,
  limit = 10
): Promise<VendedoresRankingResponse> {
  vlog(F, "apiDashboardVendedores", "montando query string");
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });

  return listarPaginado<VendedorRanking[]>(`/api/dashboard/vendedores?${qs.toString()}`, page, limit);
}

// === Senha Historico ===

export async function apiListSenhaHistorico(
  page = 1,
  limit = 20,
  usuarioId?: number,
  tipo?: TipoReset,
  orderBy?: string,
  orderDir?: "asc" | "desc"
): Promise<SenhaHistoricoResponse> {
  vlog(F, "apiListSenhaHistorico", "montando query string");
  const params = new URLSearchParams({
    page: String(page),
    limit: String(limit),
  });
  vlog(F, "apiListSenhaHistorico", "adicionando usuario_id na query se informado");
  if (usuarioId) params.set("usuario_id", String(usuarioId));
  vlog(F, "apiListSenhaHistorico", "adicionando tipo na query se informado");
  if (tipo) params.set("tipo", tipo);
  vlog(F, "apiListSenhaHistorico", "adicionando order_by na query se informado");
  if (orderBy) params.set("order_by", orderBy);
  vlog(F, "apiListSenhaHistorico", "adicionando order_dir na query se informado");
  if (orderDir) params.set("order_dir", orderDir);

  vlog(F, "apiListSenhaHistorico", "montando rota do histórico de senhas");
  const path = `/api/senha-historico${usuarioId ? `/${usuarioId}` : ""}`;
  return listarPaginado<SenhaHistoricoResponse["data"]>(`${path}?${params.toString()}`, page, limit);
}

// === Vendedores ===

// GET /api/vendedores — lista simples (sem paginação) de vendedores ativos,
// usada para popular selects. Envelope padrão {success, data, error}.
export async function apiListVendedores(): Promise<Vendedor[]> {
  return fetchWithAuth<Vendedor[]>("/api/vendedores", {
    method: "GET",
  });
}

// GET /api/vendedores/{id} — detalhe de um vendedor + clientes vinculados
// (carteira ativa). Envelope padrao {success, data, error}.
export async function apiGetVendedor(id: number): Promise<VendedorDetalhe> {
  return fetchWithAuth<VendedorDetalhe>(`/api/vendedores/${id}`, {
    method: "GET",
  });
}

// POST /api/vendedores — cria novo vendedor. data_admissao e opcional
// (default hoje no backend).
export async function apiCreateVendedor(
  input: VendedorInput
): Promise<VendedorCompleto> {
  return fetchWithAuth<VendedorCompleto>("/api/vendedores", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// PUT /api/vendedores/{id} — atualiza dados cadastrais do vendedor.
// data_desligamento nao e editavel por esta rota (use apiDeleteVendedor).
export async function apiUpdateVendedor(
  id: number,
  input: VendedorInput
): Promise<VendedorCompleto> {
  return fetchWithAuth<VendedorCompleto>(`/api/vendedores/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

// DELETE /api/vendedores/{id} — inativa o vendedor (soft-delete via
// data_desligamento = hoje), preservando o historico de carteiras/pedidos.
export async function apiDeleteVendedor(id: number): Promise<VendedorCompleto> {
  return fetchWithAuth<VendedorCompleto>(`/api/vendedores/${id}`, {
    method: "DELETE",
  });
}

// POST /api/vendedores/{id}/reativar — reverte o soft-delete do vendedor
// (limpa data_desligamento), tornando-o ativo novamente.
export async function apiReativarVendedor(id: number): Promise<VendedorCompleto> {
  return fetchWithAuth<VendedorCompleto>(`/api/vendedores/${id}/reativar`, {
    method: "POST",
  });
}

// POST /api/vendedores/{id}/clientes — vincula um cliente a carteira ativa
// do vendedor. Se o cliente ja tiver vendedor ativo, a API transfere a
// carteira automaticamente (encerrando o vinculo anterior). Retorna 201 com
// o ClienteResumo do cliente vinculado.
export async function apiVincularCliente(
  vendedorId: number,
  clienteId: number
): Promise<ClienteResumo> {
  return fetchWithAuth<ClienteResumo>(`/api/vendedores/${vendedorId}/clientes`, {
    method: "POST",
    body: JSON.stringify({ cliente_id: clienteId }),
  });
}

// DELETE /api/vendedores/{id}/clientes/{clienteId} — encerra o vinculo ativo
// entre aquele cliente e aquele vendedor especificamente.
export async function apiDesvincularCliente(
  vendedorId: number,
  clienteId: number
): Promise<void> {
  vlog(F, "apiDesvincularCliente", "chamando API (sem corpo de resposta)");
  await fetchWithAuth<null>(
    `/api/vendedores/${vendedorId}/clientes/${clienteId}`,
    {
      method: "DELETE",
    }
  );
}

// GET /api/vendedores/{id}/clientes — acesso comum, lista os clientes da
// carteira ativa daquele vendedor. Usado para popular o dropdown de
// Clientes filtrado pelo Vendedor selecionado (ex: OportunidadeModal).
// Envelope padrao {success, data, error}.
export async function apiListClientesDoVendedor(
  vendedorId: number
): Promise<ClienteResumo[]> {
  return fetchWithAuth<ClienteResumo[]>(
    `/api/vendedores/${vendedorId}/clientes`,
    {
      method: "GET",
    }
  );
}

// === Clientes ===

export interface ListClientesFilters {
  uf?: string;
  segmento?: string;
  ativo?: boolean;
  q?: string;
}

export interface ListClientesResponse {
  data: Cliente[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

// GET /api/clientes — lista paginada com filtros. Envelope
// {success, data, pagination: {page, limit, total, pages}, error}.
export async function apiListClientes(
  page = 1,
  limit = 20,
  filters: ListClientesFilters = {},
  orderBy?: string,
  orderDir?: "asc" | "desc"
): Promise<ListClientesResponse> {
  vlog(F, "apiListClientes", "montando query string");
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  vlog(F, "apiListClientes", "adicionando uf na query se informado");
  if (filters.uf) qs.set("uf", filters.uf);
  vlog(F, "apiListClientes", "adicionando segmento na query se informado");
  if (filters.segmento) qs.set("segmento", filters.segmento);
  vlog(F, "apiListClientes", "adicionando ativo na query se informado");
  if (filters.ativo !== undefined) qs.set("ativo", String(filters.ativo));
  vlog(F, "apiListClientes", "adicionando q na query se informado");
  if (filters.q) qs.set("q", filters.q);
  vlog(F, "apiListClientes", "adicionando order_by na query se informado");
  if (orderBy) qs.set("order_by", orderBy);
  vlog(F, "apiListClientes", "adicionando order_dir na query se informado");
  if (orderDir) qs.set("order_dir", orderDir);

  return listarPaginado<Cliente[]>(`/api/clientes?${qs.toString()}`, page, limit);
}

// GET /api/clientes/{id} — detalhe de um cliente
export async function apiGetCliente(id: number): Promise<Cliente> {
  return fetchWithAuth<Cliente>(`/api/clientes/${id}`, {
    method: "GET",
  });
}

// POST /api/clientes — admin cria novo cliente. cliente_id_origem e gerado
// automaticamente pelo backend. data_cadastro e opcional (default hoje).
export async function apiCreateCliente(input: ClienteInput): Promise<Cliente> {
  return fetchWithAuth<Cliente>("/api/clientes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// PUT /api/clientes/{id} — admin atualiza dados cadastrais do cliente.
// Nao permite editar cliente_id_origem nem ativo (use apiToggleClienteStatus).
export async function apiUpdateCliente(
  id: number,
  input: ClienteInput
): Promise<Cliente> {
  return fetchWithAuth<Cliente>(`/api/clientes/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

// PATCH /api/clientes/{id}/inativar — alterna (ou define) o status ativo
export async function apiToggleClienteStatus(
  id: number,
  ativo?: boolean
): Promise<Cliente> {
  return fetchWithAuth<Cliente>(`/api/clientes/${id}/inativar`, {
    method: "PATCH",
    body: ativo === undefined ? undefined : JSON.stringify({ ativo }),
  });
}

// GET /api/dashboard/clientes — metricas de clientes para o dashboard
export async function apiDashboardClientes(
  periodo: DashboardPeriodo = "month"
): Promise<ClienteDashboardMetrics> {
  vlog(F, "apiDashboardClientes", "montando query string");
  const qs = new URLSearchParams({ periodo });
  return fetchWithAuth<ClienteDashboardMetrics>(
    `/api/dashboard/clientes?${qs.toString()}`,
    {
      method: "GET",
    }
  );
}

// === Produtos ===

export interface ListProdutosFilters {
  categoria?: string;
  marca?: string;
  ativo?: boolean;
  q?: string;
}

export interface ListProdutosResponse {
  data: Produto[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

// GET /api/produtos — lista paginada com filtros. Envelope
// {success, data, pagination: {page, limit, total, pages}, error}.
export async function apiListProdutos(
  page = 1,
  limit = 20,
  filters: ListProdutosFilters = {},
  orderBy?: string,
  orderDir?: "asc" | "desc"
): Promise<ListProdutosResponse> {
  vlog(F, "apiListProdutos", "montando query string");
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  vlog(F, "apiListProdutos", "adicionando categoria na query se informado");
  if (filters.categoria) qs.set("categoria", filters.categoria);
  vlog(F, "apiListProdutos", "adicionando marca na query se informado");
  if (filters.marca) qs.set("marca", filters.marca);
  vlog(F, "apiListProdutos", "adicionando ativo na query se informado");
  if (filters.ativo !== undefined) qs.set("ativo", String(filters.ativo));
  vlog(F, "apiListProdutos", "adicionando q na query se informado");
  if (filters.q) qs.set("q", filters.q);
  vlog(F, "apiListProdutos", "adicionando order_by na query se informado");
  if (orderBy) qs.set("order_by", orderBy);
  vlog(F, "apiListProdutos", "adicionando order_dir na query se informado");
  if (orderDir) qs.set("order_dir", orderDir);

  return listarPaginado<Produto[]>(`/api/produtos?${qs.toString()}`, page, limit);
}

// GET /api/produtos/{id} — detalhe de um produto
export async function apiGetProduto(id: number): Promise<Produto> {
  return fetchWithAuth<Produto>(`/api/produtos/${id}`, {
    method: "GET",
  });
}

// POST /api/produtos — admin cria novo produto. ativo e sempre TRUE no create.
export async function apiCreateProduto(input: ProdutoInput): Promise<Produto> {
  return fetchWithAuth<Produto>("/api/produtos", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// PUT /api/produtos/{id} — admin atualiza dados cadastrais do produto.
// Nao permite editar sku nem ativo (use apiToggleProdutoStatus).
export async function apiUpdateProduto(
  id: number,
  input: ProdutoInput
): Promise<Produto> {
  return fetchWithAuth<Produto>(`/api/produtos/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

// PATCH /api/produtos/{id}/inativar — alterna (ou define) o status ativo
export async function apiToggleProdutoStatus(
  id: number,
  ativo?: boolean
): Promise<Produto> {
  return fetchWithAuth<Produto>(`/api/produtos/${id}/inativar`, {
    method: "PATCH",
    body: ativo === undefined ? undefined : JSON.stringify({ ativo }),
  });
}

// === Pedidos ===

export interface ListPedidosFilters {
  status?: string;
  canal?: string;
  cliente_id?: number;
  vendedor_id?: number;
  data_inicio?: string; // AAAA-MM-DD
  data_fim?: string; // AAAA-MM-DD
  q?: string;
}

export interface ListPedidosResponse {
  data: Pedido[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

// GET /api/pedidos — lista paginada com filtros. Envelope
// {success, data, pagination: {page, limit, total, pages}, error}.
export async function apiListPedidos(
  page = 1,
  limit = 20,
  filters: ListPedidosFilters = {},
  orderBy?: string,
  orderDir?: "asc" | "desc"
): Promise<ListPedidosResponse> {
  vlog(F, "apiListPedidos", "montando query string");
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  vlog(F, "apiListPedidos", "adicionando status na query se informado");
  if (filters.status) qs.set("status", filters.status);
  vlog(F, "apiListPedidos", "adicionando canal na query se informado");
  if (filters.canal) qs.set("canal", filters.canal);
  vlog(F, "apiListPedidos", "adicionando cliente_id na query se informado");
  if (filters.cliente_id) qs.set("cliente_id", String(filters.cliente_id));
  vlog(F, "apiListPedidos", "adicionando vendedor_id na query se informado");
  if (filters.vendedor_id) qs.set("vendedor_id", String(filters.vendedor_id));
  vlog(F, "apiListPedidos", "adicionando data_inicio na query se informado");
  if (filters.data_inicio) qs.set("data_inicio", filters.data_inicio);
  vlog(F, "apiListPedidos", "adicionando data_fim na query se informado");
  if (filters.data_fim) qs.set("data_fim", filters.data_fim);
  vlog(F, "apiListPedidos", "adicionando q na query se informado");
  if (filters.q) qs.set("q", filters.q);
  vlog(F, "apiListPedidos", "adicionando order_by na query se informado");
  if (orderBy) qs.set("order_by", orderBy);
  vlog(F, "apiListPedidos", "adicionando order_dir na query se informado");
  if (orderDir) qs.set("order_dir", orderDir);

  return listarPaginado<Pedido[]>(`/api/pedidos?${qs.toString()}`, page, limit);
}

// GET /api/pedidos/{id} — detalhe de um pedido (cabecalho + itens)
export async function apiGetPedido(id: number): Promise<PedidoDetalhe> {
  return fetchWithAuth<PedidoDetalhe>(`/api/pedidos/${id}`, {
    method: "GET",
  });
}

// POST /api/pedidos — admin cria novo pedido com itens. valor_bruto de cada
// item e valor_total do pedido sao calculados no backend. pedido_id_origem e
// gerado automaticamente.
export async function apiCreatePedido(
  input: PedidoInput
): Promise<PedidoDetalhe> {
  return fetchWithAuth<PedidoDetalhe>("/api/pedidos", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// PUT /api/pedidos/{id} — admin atualiza cabecalho do pedido e substitui a
// lista de itens integralmente (delete + insert no backend).
export async function apiUpdatePedido(
  id: number,
  input: PedidoInput
): Promise<PedidoDetalhe> {
  return fetchWithAuth<PedidoDetalhe>(`/api/pedidos/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

// DELETE /api/pedidos/{id} — exclui um pedido. Backend retorna 409 se o
// pedido possuir pagamentos vinculados ou estiver faturado.
export async function apiDeletePedido(id: number): Promise<void> {
  vlog(F, "apiDeletePedido", "chamando API (sem corpo de resposta)");
  await fetchWithAuth<null>(`/api/pedidos/${id}`, {
    method: "DELETE",
  });
}

// === Pagamentos ===
//
// Acesso comum (qualquer usuário autenticado, admin ou normal) — ver
// apis/rotaperfumes-api/handlers/pagamento_handler.go.

export interface ListPagamentosResponse {
  data: Pagamento[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

// GET /api/pagamentos — lista paginada com filtros. Envelope
// {success, data, pagination: {page, limit, total, pages}, error}.
export async function apiListPagamentos(
  page = 1,
  limit = 20,
  filters: ListPagamentosFilters = {},
  orderBy?: string,
  orderDir?: "asc" | "desc"
): Promise<ListPagamentosResponse> {
  vlog(F, "apiListPagamentos", "montando query string");
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  vlog(F, "apiListPagamentos", "adicionando status_pagamento na query se informado");
  if (filters.status_pagamento) qs.set("status_pagamento", filters.status_pagamento);
  vlog(F, "apiListPagamentos", "adicionando forma_pagamento na query se informado");
  if (filters.forma_pagamento) qs.set("forma_pagamento", filters.forma_pagamento);
  vlog(F, "apiListPagamentos", "adicionando pedido_id na query se informado");
  if (filters.pedido_id) qs.set("pedido_id", String(filters.pedido_id));
  vlog(F, "apiListPagamentos", "adicionando vencimento_de na query se informado");
  if (filters.vencimento_de) qs.set("vencimento_de", filters.vencimento_de);
  vlog(F, "apiListPagamentos", "adicionando vencimento_ate na query se informado");
  if (filters.vencimento_ate) qs.set("vencimento_ate", filters.vencimento_ate);
  vlog(F, "apiListPagamentos", "adicionando order_by na query se informado");
  if (orderBy) qs.set("order_by", orderBy);
  vlog(F, "apiListPagamentos", "adicionando order_dir na query se informado");
  if (orderDir) qs.set("order_dir", orderDir);

  return listarPaginado<Pagamento[]>(`/api/pagamentos?${qs.toString()}`, page, limit);
}

// GET /api/pagamentos/{id} — detalhe de um pagamento (id = pagamento_id)
export async function apiGetPagamento(id: number): Promise<Pagamento> {
  return fetchWithAuth<Pagamento>(`/api/pagamentos/${id}`, {
    method: "GET",
  });
}

// POST /api/pagamentos — cria novo pagamento. valor_liquido e exigido
// explicitamente no payload (nao e calculado automaticamente pelo backend).
export async function apiCreatePagamento(
  input: PagamentoCreateInput
): Promise<Pagamento> {
  return fetchWithAuth<Pagamento>("/api/pagamentos", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// PUT /api/pagamentos/{id} — atualiza um pagamento existente. pagamento_id
// e pedido_id nao sao editaveis por esta rota (vinculo com o pedido de
// origem e definitivo).
export async function apiUpdatePagamento(
  id: number,
  input: PagamentoUpdateInput
): Promise<Pagamento> {
  return fetchWithAuth<Pagamento>(`/api/pagamentos/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

// DELETE /api/pagamentos/{id} — exclui um pagamento. Backend retorna 409 se
// o pagamento ja estiver quitado.
export async function apiDeletePagamento(id: number): Promise<void> {
  vlog(F, "apiDeletePagamento", "chamando API (sem corpo de resposta)");
  await fetchWithAuth<null>(`/api/pagamentos/${id}`, {
    method: "DELETE",
  });
}

// === Oportunidades (CRM) ===
//
// Tela admin-only — ver rota /api/oportunidades no backend.

export interface ListOportunidadesResponse {
  data: Oportunidade[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

// GET /api/oportunidades — lista paginada com filtros. Envelope
// {success, data, pagination: {page, limit, total, pages}, error}.
export async function apiListOportunidades(
  page = 1,
  limit = 20,
  filters: ListOportunidadesFilters = {},
  orderBy?: string,
  orderDir?: "asc" | "desc"
): Promise<ListOportunidadesResponse> {
  vlog(F, "apiListOportunidades", "montando query string");
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  vlog(F, "apiListOportunidades", "adicionando cliente_id na query se informado");
  if (filters.cliente_id) qs.set("cliente_id", String(filters.cliente_id));
  vlog(F, "apiListOportunidades", "adicionando vendedor_id na query se informado");
  if (filters.vendedor_id) qs.set("vendedor_id", String(filters.vendedor_id));
  vlog(F, "apiListOportunidades", "adicionando etapa na query se informado");
  if (filters.etapa) qs.set("etapa", filters.etapa);
  vlog(F, "apiListOportunidades", "adicionando origem na query se informado");
  if (filters.origem) qs.set("origem", filters.origem);
  vlog(F, "apiListOportunidades", "adicionando data_abertura_de na query se informado");
  if (filters.data_abertura_de) qs.set("data_abertura_de", filters.data_abertura_de);
  vlog(F, "apiListOportunidades", "adicionando data_abertura_ate na query se informado");
  if (filters.data_abertura_ate) qs.set("data_abertura_ate", filters.data_abertura_ate);
  vlog(F, "apiListOportunidades", "adicionando q na query se informado");
  if (filters.q) qs.set("q", filters.q);
  vlog(F, "apiListOportunidades", "adicionando order_by na query se informado");
  if (orderBy) qs.set("order_by", orderBy);
  vlog(F, "apiListOportunidades", "adicionando order_dir na query se informado");
  if (orderDir) qs.set("order_dir", orderDir);

  return listarPaginado<Oportunidade[]>(`/api/oportunidades?${qs.toString()}`, page, limit);
}

// GET /api/oportunidades/{id} — detalhe de uma oportunidade
export async function apiGetOportunidade(id: number): Promise<Oportunidade> {
  return fetchWithAuth<Oportunidade>(`/api/oportunidades/${id}`, {
    method: "GET",
  });
}

// POST /api/oportunidades — admin cria nova oportunidade.
export async function apiCreateOportunidade(
  input: OportunidadeInput
): Promise<Oportunidade> {
  return fetchWithAuth<Oportunidade>("/api/oportunidades", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// PUT /api/oportunidades/{id} — admin atualiza uma oportunidade existente.
export async function apiUpdateOportunidade(
  id: number,
  input: OportunidadeInput
): Promise<Oportunidade> {
  return fetchWithAuth<Oportunidade>(`/api/oportunidades/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

// DELETE /api/oportunidades/{id} — admin exclui uma oportunidade existente.
export async function apiDeleteOportunidade(id: number): Promise<void> {
  vlog(F, "apiDeleteOportunidade", "chamando API (sem corpo de resposta)");
  await fetchWithAuth<null>(`/api/oportunidades/${id}`, {
    method: "DELETE",
  });
}

// === Visitas (CRM) ===
//
// Tela admin-only — ver rota /api/visitas no backend.

export interface ListVisitasResponse {
  data: Visita[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

// GET /api/visitas — lista paginada com filtros. Envelope
// {success, data, pagination: {page, limit, total, pages}, error}.
export async function apiListVisitas(
  page = 1,
  limit = 20,
  filters: ListVisitasFilters = {},
  orderBy?: string,
  orderDir?: "asc" | "desc"
): Promise<ListVisitasResponse> {
  vlog(F, "apiListVisitas", "montando query string");
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  vlog(F, "apiListVisitas", "adicionando cliente_id na query se informado");
  if (filters.cliente_id) qs.set("cliente_id", String(filters.cliente_id));
  vlog(F, "apiListVisitas", "adicionando vendedor_id na query se informado");
  if (filters.vendedor_id) qs.set("vendedor_id", String(filters.vendedor_id));
  vlog(F, "apiListVisitas", "adicionando resultado na query se informado");
  if (filters.resultado) qs.set("resultado", filters.resultado);
  vlog(F, "apiListVisitas", "adicionando data_visita_de na query se informado");
  if (filters.data_visita_de) qs.set("data_visita_de", filters.data_visita_de);
  vlog(F, "apiListVisitas", "adicionando data_visita_ate na query se informado");
  if (filters.data_visita_ate) qs.set("data_visita_ate", filters.data_visita_ate);
  vlog(F, "apiListVisitas", "adicionando q na query se informado");
  if (filters.q) qs.set("q", filters.q);
  vlog(F, "apiListVisitas", "adicionando order_by na query se informado");
  if (orderBy) qs.set("order_by", orderBy);
  vlog(F, "apiListVisitas", "adicionando order_dir na query se informado");
  if (orderDir) qs.set("order_dir", orderDir);

  return listarPaginado<Visita[]>(`/api/visitas?${qs.toString()}`, page, limit);
}

// GET /api/visitas/{id} — detalhe de uma visita
export async function apiGetVisita(id: number): Promise<Visita> {
  return fetchWithAuth<Visita>(`/api/visitas/${id}`, {
    method: "GET",
  });
}

// POST /api/visitas — admin cria nova visita.
export async function apiCreateVisita(input: VisitaInput): Promise<Visita> {
  return fetchWithAuth<Visita>("/api/visitas", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// PUT /api/visitas/{id} — admin atualiza uma visita existente.
export async function apiUpdateVisita(
  id: number,
  input: VisitaInput
): Promise<Visita> {
  return fetchWithAuth<Visita>(`/api/visitas/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

// DELETE /api/visitas/{id} — admin exclui uma visita existente.
export async function apiDeleteVisita(id: number): Promise<void> {
  vlog(F, "apiDeleteVisita", "chamando API (sem corpo de resposta)");
  await fetchWithAuth<null>(`/api/visitas/${id}`, {
    method: "DELETE",
  });
}

// === Estoque ===
//
// Leitura e acesso comum (qualquer usuário autenticado); criar/editar são
// admin-only — ver apis/rotaperfumes-api/handlers/estoque_handler.go.

export interface ListEstoqueResponse {
  data: Estoque[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

// GET /api/estoque — lista paginada com filtros. Envelope
// {success, data, pagination: {page, limit, total, pages}, error}.
// Sem data_de/data_ate, retorna a última posição de estoque de cada SKU;
// com data_de/data_ate, retorna apenas o último movimento de cada SKU
// dentro do período informado.
export async function apiListEstoque(
  page = 1,
  limit = 20,
  filters: ListEstoqueFilters = {},
  orderBy?: string,
  orderDir?: "asc" | "desc"
): Promise<ListEstoqueResponse> {
  vlog(F, "apiListEstoque", "montando query string");
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  vlog(F, "apiListEstoque", "adicionando sku na query se informado");
  if (filters.sku) qs.set("sku", filters.sku);
  vlog(F, "apiListEstoque", "adicionando data_de na query se informado");
  if (filters.data_de) qs.set("data_de", filters.data_de);
  vlog(F, "apiListEstoque", "adicionando data_ate na query se informado");
  if (filters.data_ate) qs.set("data_ate", filters.data_ate);
  vlog(F, "apiListEstoque", "adicionando ruptura na query se informado");
  if (filters.ruptura !== undefined) qs.set("ruptura", String(filters.ruptura));
  vlog(F, "apiListEstoque", "adicionando order_by na query se informado");
  if (orderBy) qs.set("order_by", orderBy);
  vlog(F, "apiListEstoque", "adicionando order_dir na query se informado");
  if (orderDir) qs.set("order_dir", orderDir);

  return listarPaginado<Estoque[]>(`/api/estoque?${qs.toString()}`, page, limit);
}

// GET /api/estoque/{id} — detalhe de um registro de estoque
export async function apiGetEstoque(id: number): Promise<Estoque> {
  return fetchWithAuth<Estoque>(`/api/estoque/${id}`, {
    method: "GET",
  });
}

// POST /api/estoque — admin cria novo registro de estoque. ruptura NÃO é
// enviada (derivada automaticamente pelo backend a partir do saldo).
export async function apiCreateEstoque(input: EstoqueInput): Promise<Estoque> {
  return fetchWithAuth<Estoque>("/api/estoque", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// PUT /api/estoque/{id} — admin atualiza o saldo de um registro existente.
// sku e data_snapshot não são editáveis após a criação.
export async function apiUpdateEstoque(
  id: number,
  input: { saldo: number }
): Promise<Estoque> {
  return fetchWithAuth<Estoque>(`/api/estoque/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

// Re-exporta API_BASE para uso externo se necessário
export { API_BASE };
