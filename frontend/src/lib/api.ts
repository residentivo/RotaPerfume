import {
  LoginRequest,
  LoginResponse,
  ResetPasswordRequest,
  User,
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
} from "./types";
import { fetchWithAuth } from "./apiClient";

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

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export async function apiLogin(
  email: string,
  password: string,
  captchaToken?: string
): Promise<LoginResponse> {
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

export async function apiMe(): Promise<User> {
  return fetchWithAuth<User>("/api/auth/me", {
    method: "GET",
  });
}

export async function apiResetPassword(
  usuarioId: number,
  novaSenha: string,
  senhaAtual?: string
): Promise<{ message: string }> {
  const body: ResetPasswordRequest = {
    usuario_id: usuarioId,
    nova_senha: novaSenha,
  };
  if (senhaAtual) {
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
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  if (orderBy) qs.set("order_by", orderBy);
  if (orderDir) qs.set("order_dir", orderDir);

  const res = await fetch(`${API_BASE}/api/usuarios?${qs.toString()}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  });

  const raw = await res.text();
  let parsed: unknown = null;
  try {
    parsed = raw ? JSON.parse(raw) : null;
  } catch {
    parsed = raw;
  }

  if (!res.ok) {
    let errorMessage = `Erro ${res.status}: ${res.statusText}`;
    if (parsed && typeof parsed === "object") {
      const d = parsed as Record<string, unknown>;
      if ("error" in d) errorMessage = String(d.error);
      else if ("message" in d) errorMessage = String(d.message);
    }
    throw new Error(errorMessage);
  }

  if (parsed && typeof parsed === "object") {
    const d = parsed as Record<string, unknown>;
    if ("data" in d && Array.isArray(d.data)) {
      // Envelope do backend: { success, data, pagination: {page, limit, total, pages} }
      const pag = (d.pagination && typeof d.pagination === "object"
        ? (d.pagination as Record<string, unknown>)
        : d) as Record<string, unknown>;
      return {
        data: d.data as User[],
        page: Number(pag.page ?? page),
        limit: Number(pag.limit ?? limit),
        total: Number(pag.total ?? (d.data as unknown[]).length),
        pages: Number(pag.pages ?? 1),
      };
    }
  }

  if (Array.isArray(parsed)) {
    return { data: parsed as User[], page, limit, total: parsed.length, pages: 1 };
  }
  return { data: [], page, limit, total: 0, pages: 0 };
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
  const qs = new URLSearchParams({ periodo });
  return fetchWithAuth<DashboardMetrics>(
    `/api/dashboard/metrics?${qs.toString()}`,
    {
      method: "GET",
    }
  );
}

export async function apiDashboardVendas(dias = 30): Promise<VendasSeries> {
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
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });

  // Endpoint devolve envelope com data + meta de paginacao, sem desenvelope
  const res = await fetch(
    `${API_BASE}/api/dashboard/vendedores?${qs.toString()}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
      credentials: "include",
    }
  );

  // Necessário fazer parse manual pois este endpoint retorna estrutura customizada
  const raw = await res.text();
  let parsed: unknown = null;
  try {
    parsed = raw ? JSON.parse(raw) : null;
  } catch {
    parsed = raw;
  }

  if (!res.ok) {
    let errorMessage = `Erro ${res.status}: ${res.statusText}`;
    if (parsed && typeof parsed === "object") {
      const d = parsed as Record<string, unknown>;
      if ("error" in d) errorMessage = String(d.error);
      else if ("message" in d) errorMessage = String(d.message);
    }
    throw new Error(errorMessage);
  }

  // Tenta extrair {data, page, limit, total, pages}
  if (parsed && typeof parsed === "object") {
    const d = parsed as Record<string, unknown>;
    if ("data" in d && Array.isArray(d.data)) {
      return {
        data: d.data as VendedorRanking[],
        page: Number(d.page ?? page),
        limit: Number(d.limit ?? limit),
        total: Number(d.total ?? (d.data as unknown[]).length),
        pages: Number(d.pages ?? 1),
      };
    }
  }

  // Fallback: array puro
  if (Array.isArray(parsed)) {
    return {
      data: parsed as VendedorRanking[],
      page,
      limit,
      total: parsed.length,
      pages: 1,
    };
  }
  return { data: [], page, limit, total: 0, pages: 0 };
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
  const params = new URLSearchParams({
    page: String(page),
    limit: String(limit),
  });
  if (usuarioId) params.set("usuario_id", String(usuarioId));
  if (tipo) params.set("tipo", tipo);
  if (orderBy) params.set("order_by", orderBy);
  if (orderDir) params.set("order_dir", orderDir);

  const path = `/api/senha-historico${usuarioId ? `/${usuarioId}` : ""}`;
  const res = await fetch(`${API_BASE}${path}?${params.toString()}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  });

  // Mesmo pattern de parse do dashboard vendedores
  const raw = await res.text();
  let parsed: unknown = null;
  try {
    parsed = raw ? JSON.parse(raw) : null;
  } catch {
    parsed = raw;
  }
  if (!res.ok) {
    let errorMessage = `Erro ${res.status}: ${res.statusText}`;
    if (parsed && typeof parsed === "object") {
      const d = parsed as Record<string, unknown>;
      if ("error" in d) errorMessage = String(d.error);
      else if ("message" in d) errorMessage = String(d.message);
    }
    throw new Error(errorMessage);
  }
  if (parsed && typeof parsed === "object") {
    const d = parsed as Record<string, unknown>;
    if ("data" in d && Array.isArray(d.data)) {
      return {
        data: d.data as SenhaHistoricoResponse["data"],
        page: Number(d.page ?? page),
        limit: Number(d.limit ?? limit),
        total: Number(d.total ?? (d.data as unknown[]).length),
        pages: Number(d.pages ?? 1),
      };
    }
  }
  if (Array.isArray(parsed)) {
    return { data: parsed as SenhaHistoricoResponse["data"], page, limit, total: parsed.length, pages: 1 };
  }
  return { data: [], page, limit, total: 0, pages: 0 };
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
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  if (filters.uf) qs.set("uf", filters.uf);
  if (filters.segmento) qs.set("segmento", filters.segmento);
  if (filters.ativo !== undefined) qs.set("ativo", String(filters.ativo));
  if (filters.q) qs.set("q", filters.q);
  if (orderBy) qs.set("order_by", orderBy);
  if (orderDir) qs.set("order_dir", orderDir);

  const res = await fetch(`${API_BASE}/api/clientes?${qs.toString()}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  });

  const raw = await res.text();
  let parsed: unknown = null;
  try {
    parsed = raw ? JSON.parse(raw) : null;
  } catch {
    parsed = raw;
  }

  if (!res.ok) {
    let errorMessage = `Erro ${res.status}: ${res.statusText}`;
    if (parsed && typeof parsed === "object") {
      const d = parsed as Record<string, unknown>;
      if ("error" in d) errorMessage = String(d.error);
      else if ("message" in d) errorMessage = String(d.message);
    }
    throw new Error(errorMessage);
  }

  if (parsed && typeof parsed === "object") {
    const d = parsed as Record<string, unknown>;
    if ("data" in d && Array.isArray(d.data)) {
      const pag = (d.pagination && typeof d.pagination === "object"
        ? (d.pagination as Record<string, unknown>)
        : d) as Record<string, unknown>;
      return {
        data: d.data as Cliente[],
        page: Number(pag.page ?? page),
        limit: Number(pag.limit ?? limit),
        total: Number(pag.total ?? (d.data as unknown[]).length),
        pages: Number(pag.pages ?? 1),
      };
    }
  }

  if (Array.isArray(parsed)) {
    return { data: parsed as Cliente[], page, limit, total: parsed.length, pages: 1 };
  }
  return { data: [], page, limit, total: 0, pages: 0 };
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
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  if (filters.categoria) qs.set("categoria", filters.categoria);
  if (filters.marca) qs.set("marca", filters.marca);
  if (filters.ativo !== undefined) qs.set("ativo", String(filters.ativo));
  if (filters.q) qs.set("q", filters.q);
  if (orderBy) qs.set("order_by", orderBy);
  if (orderDir) qs.set("order_dir", orderDir);

  const res = await fetch(`${API_BASE}/api/produtos?${qs.toString()}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  });

  const raw = await res.text();
  let parsed: unknown = null;
  try {
    parsed = raw ? JSON.parse(raw) : null;
  } catch {
    parsed = raw;
  }

  if (!res.ok) {
    let errorMessage = `Erro ${res.status}: ${res.statusText}`;
    if (parsed && typeof parsed === "object") {
      const d = parsed as Record<string, unknown>;
      if ("error" in d) errorMessage = String(d.error);
      else if ("message" in d) errorMessage = String(d.message);
    }
    throw new Error(errorMessage);
  }

  if (parsed && typeof parsed === "object") {
    const d = parsed as Record<string, unknown>;
    if ("data" in d && Array.isArray(d.data)) {
      const pag = (d.pagination && typeof d.pagination === "object"
        ? (d.pagination as Record<string, unknown>)
        : d) as Record<string, unknown>;
      return {
        data: d.data as Produto[],
        page: Number(pag.page ?? page),
        limit: Number(pag.limit ?? limit),
        total: Number(pag.total ?? (d.data as unknown[]).length),
        pages: Number(pag.pages ?? 1),
      };
    }
  }

  if (Array.isArray(parsed)) {
    return { data: parsed as Produto[], page, limit, total: parsed.length, pages: 1 };
  }
  return { data: [], page, limit, total: 0, pages: 0 };
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
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  if (filters.status) qs.set("status", filters.status);
  if (filters.canal) qs.set("canal", filters.canal);
  if (filters.cliente_id) qs.set("cliente_id", String(filters.cliente_id));
  if (filters.vendedor_id) qs.set("vendedor_id", String(filters.vendedor_id));
  if (filters.data_inicio) qs.set("data_inicio", filters.data_inicio);
  if (filters.data_fim) qs.set("data_fim", filters.data_fim);
  if (filters.q) qs.set("q", filters.q);
  if (orderBy) qs.set("order_by", orderBy);
  if (orderDir) qs.set("order_dir", orderDir);

  const res = await fetch(`${API_BASE}/api/pedidos?${qs.toString()}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  });

  const raw = await res.text();
  let parsed: unknown = null;
  try {
    parsed = raw ? JSON.parse(raw) : null;
  } catch {
    parsed = raw;
  }

  if (!res.ok) {
    let errorMessage = `Erro ${res.status}: ${res.statusText}`;
    if (parsed && typeof parsed === "object") {
      const d = parsed as Record<string, unknown>;
      if ("error" in d) errorMessage = String(d.error);
      else if ("message" in d) errorMessage = String(d.message);
    }
    throw new Error(errorMessage);
  }

  if (parsed && typeof parsed === "object") {
    const d = parsed as Record<string, unknown>;
    if ("data" in d && Array.isArray(d.data)) {
      const pag = (d.pagination && typeof d.pagination === "object"
        ? (d.pagination as Record<string, unknown>)
        : d) as Record<string, unknown>;
      return {
        data: d.data as Pedido[],
        page: Number(pag.page ?? page),
        limit: Number(pag.limit ?? limit),
        total: Number(pag.total ?? (d.data as unknown[]).length),
        pages: Number(pag.pages ?? 1),
      };
    }
  }

  if (Array.isArray(parsed)) {
    return { data: parsed as Pedido[], page, limit, total: parsed.length, pages: 1 };
  }
  return { data: [], page, limit, total: 0, pages: 0 };
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
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  if (filters.status_pagamento) qs.set("status_pagamento", filters.status_pagamento);
  if (filters.forma_pagamento) qs.set("forma_pagamento", filters.forma_pagamento);
  if (filters.pedido_id) qs.set("pedido_id", String(filters.pedido_id));
  if (filters.vencimento_de) qs.set("vencimento_de", filters.vencimento_de);
  if (filters.vencimento_ate) qs.set("vencimento_ate", filters.vencimento_ate);
  if (orderBy) qs.set("order_by", orderBy);
  if (orderDir) qs.set("order_dir", orderDir);

  const res = await fetch(`${API_BASE}/api/pagamentos?${qs.toString()}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  });

  const raw = await res.text();
  let parsed: unknown = null;
  try {
    parsed = raw ? JSON.parse(raw) : null;
  } catch {
    parsed = raw;
  }

  if (!res.ok) {
    let errorMessage = `Erro ${res.status}: ${res.statusText}`;
    if (parsed && typeof parsed === "object") {
      const d = parsed as Record<string, unknown>;
      if ("error" in d) errorMessage = String(d.error);
      else if ("message" in d) errorMessage = String(d.message);
    }
    throw new Error(errorMessage);
  }

  if (parsed && typeof parsed === "object") {
    const d = parsed as Record<string, unknown>;
    if ("data" in d && Array.isArray(d.data)) {
      const pag = (d.pagination && typeof d.pagination === "object"
        ? (d.pagination as Record<string, unknown>)
        : d) as Record<string, unknown>;
      return {
        data: d.data as Pagamento[],
        page: Number(pag.page ?? page),
        limit: Number(pag.limit ?? limit),
        total: Number(pag.total ?? (d.data as unknown[]).length),
        pages: Number(pag.pages ?? 1),
      };
    }
  }

  if (Array.isArray(parsed)) {
    return { data: parsed as Pagamento[], page, limit, total: parsed.length, pages: 1 };
  }
  return { data: [], page, limit, total: 0, pages: 0 };
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
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  if (filters.cliente_id) qs.set("cliente_id", String(filters.cliente_id));
  if (filters.vendedor_id) qs.set("vendedor_id", String(filters.vendedor_id));
  if (filters.etapa) qs.set("etapa", filters.etapa);
  if (filters.origem) qs.set("origem", filters.origem);
  if (filters.data_abertura_de) qs.set("data_abertura_de", filters.data_abertura_de);
  if (filters.data_abertura_ate) qs.set("data_abertura_ate", filters.data_abertura_ate);
  if (filters.q) qs.set("q", filters.q);
  if (orderBy) qs.set("order_by", orderBy);
  if (orderDir) qs.set("order_dir", orderDir);

  const res = await fetch(`${API_BASE}/api/oportunidades?${qs.toString()}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  });

  const raw = await res.text();
  let parsed: unknown = null;
  try {
    parsed = raw ? JSON.parse(raw) : null;
  } catch {
    parsed = raw;
  }

  if (!res.ok) {
    let errorMessage = `Erro ${res.status}: ${res.statusText}`;
    if (parsed && typeof parsed === "object") {
      const d = parsed as Record<string, unknown>;
      if ("error" in d) errorMessage = String(d.error);
      else if ("message" in d) errorMessage = String(d.message);
    }
    throw new Error(errorMessage);
  }

  if (parsed && typeof parsed === "object") {
    const d = parsed as Record<string, unknown>;
    if ("data" in d && Array.isArray(d.data)) {
      const pag = (d.pagination && typeof d.pagination === "object"
        ? (d.pagination as Record<string, unknown>)
        : d) as Record<string, unknown>;
      return {
        data: d.data as Oportunidade[],
        page: Number(pag.page ?? page),
        limit: Number(pag.limit ?? limit),
        total: Number(pag.total ?? (d.data as unknown[]).length),
        pages: Number(pag.pages ?? 1),
      };
    }
  }

  if (Array.isArray(parsed)) {
    return { data: parsed as Oportunidade[], page, limit, total: parsed.length, pages: 1 };
  }
  return { data: [], page, limit, total: 0, pages: 0 };
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
  const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
  if (filters.cliente_id) qs.set("cliente_id", String(filters.cliente_id));
  if (filters.vendedor_id) qs.set("vendedor_id", String(filters.vendedor_id));
  if (filters.resultado) qs.set("resultado", filters.resultado);
  if (filters.data_visita_de) qs.set("data_visita_de", filters.data_visita_de);
  if (filters.data_visita_ate) qs.set("data_visita_ate", filters.data_visita_ate);
  if (filters.q) qs.set("q", filters.q);
  if (orderBy) qs.set("order_by", orderBy);
  if (orderDir) qs.set("order_dir", orderDir);

  const res = await fetch(`${API_BASE}/api/visitas?${qs.toString()}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  });

  const raw = await res.text();
  let parsed: unknown = null;
  try {
    parsed = raw ? JSON.parse(raw) : null;
  } catch {
    parsed = raw;
  }

  if (!res.ok) {
    let errorMessage = `Erro ${res.status}: ${res.statusText}`;
    if (parsed && typeof parsed === "object") {
      const d = parsed as Record<string, unknown>;
      if ("error" in d) errorMessage = String(d.error);
      else if ("message" in d) errorMessage = String(d.message);
    }
    throw new Error(errorMessage);
  }

  if (parsed && typeof parsed === "object") {
    const d = parsed as Record<string, unknown>;
    if ("data" in d && Array.isArray(d.data)) {
      const pag = (d.pagination && typeof d.pagination === "object"
        ? (d.pagination as Record<string, unknown>)
        : d) as Record<string, unknown>;
      return {
        data: d.data as Visita[],
        page: Number(pag.page ?? page),
        limit: Number(pag.limit ?? limit),
        total: Number(pag.total ?? (d.data as unknown[]).length),
        pages: Number(pag.pages ?? 1),
      };
    }
  }

  if (Array.isArray(parsed)) {
    return { data: parsed as Visita[], page, limit, total: parsed.length, pages: 1 };
  }
  return { data: [], page, limit, total: 0, pages: 0 };
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

// Re-exporta API_BASE para uso externo se necessário
export { API_BASE };
