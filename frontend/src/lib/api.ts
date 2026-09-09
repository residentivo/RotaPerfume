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
} from "./types";
import { fetchWithAuth } from "./apiClient";

export interface CreateUserRequest {
  nome: string;
  email: string;
  role: UserRole;
  senha?: string;
}

export interface UpdateUserRequest {
  nome?: string;
  role?: UserRole;
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export async function apiLogin(
  email: string,
  password: string
): Promise<LoginResponse> {
  const body: LoginRequest = { email, password };
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
  novaSenha: string
): Promise<{ message: string }> {
  return fetchWithAuth<{ message: string }>("/api/auth/reset-password", {
    method: "POST",
    body: JSON.stringify({
      senha_atual: senhaAtual,
      nova_senha: novaSenha,
    }),
  });
}

// PLACEHOLDER: endpoint de listagem de usuarios pode nao existir no backend ainda.
// Esperado: GET /api/usuarios  -> { usuarios: User[] } ou User[]
export async function apiListUsers(): Promise<User[]> {
  const data = await fetchWithAuth<User[] | { usuarios: User[] }>(
    "/api/usuarios",
    {
      method: "GET",
    }
  );
  if (Array.isArray(data)) return data;
  if (data && Array.isArray((data as { usuarios: User[] }).usuarios)) {
    return (data as { usuarios: User[] }).usuarios;
  }
  return [];
}

// POST /api/usuarios — admin cria novo usuario
export async function apiCreateUser(data: CreateUserRequest): Promise<User> {
  return fetchWithAuth<User>("/api/usuarios", {
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

// POST /api/admin/reset-password — admin reseta senha de um usuario
export async function apiAdminResetPassword(
  usuario_id: number,
  nova_senha: string
): Promise<{ message: string }> {
  return fetchWithAuth<{ message: string }>("/api/admin/reset-password", {
    method: "POST",
    body: JSON.stringify({ usuario_id, nova_senha }),
  });
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
        Authorization: `Bearer ${getAccessTokenForDirectFetch()}`,
      },
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
  tipo?: TipoReset
): Promise<SenhaHistoricoResponse> {
  const params = new URLSearchParams({
    page: String(page),
    limit: String(limit),
  });
  if (usuarioId) params.set("usuario_id", String(usuarioId));
  if (tipo) params.set("tipo", tipo);

  const path = `/api/senha-historico${usuarioId ? `/${usuarioId}` : ""}`;
  const res = await fetch(`${API_BASE}${path}?${params.toString()}`, {
    method: "GET",
    headers: getAuthHeaders(),
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

// Helper usado pelo endpoint de vendedores que tem estrutura especial
// SEGURANCA: Lê token do cookie (não localStorage para evitar XSS)
function getAccessTokenForDirectFetch(): string {
  if (typeof window === "undefined") return "";
  const match = document.cookie.match(/(^| )access_token=([^;]+)/);
  return match ? decodeURIComponent(match[2]) : "";
}

// Re-exporta API_BASE para uso externo se necessário
export { API_BASE };