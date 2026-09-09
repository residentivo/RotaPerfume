// === Senha Historico ===

export type TipoReset = "proprio" | "admin" | "primeiro_login";

export interface SenhaHistoricoItem {
  id: number;
  usuario_id: number;
  usuario_nome: string;
  resetado_por_id: number | null;
  resetado_por_nome: string | null;
  tipo_reset: TipoReset;
  ip_origem: string | null;
  created_at: string;
}

export interface SenhaHistoricoResponse {
  data: SenhaHistoricoItem[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

export type UserRole = "admin" | "user" | "vendedor";

export interface User {
  id: number;
  nome: string;
  email: string;
  role: UserRole;
  ativo: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  /** Access token (curta duração) */
  token: string;
  /** Refresh token (longa duração) - opcional, depende do backend */
  refresh_token?: string;
  /** Access token no novo formato (caso backend use access_token/refresh_token) */
  access_token?: string;
  user: User;
  trocar_senha?: boolean;
}

export interface ApiError {
  error: string;
  message?: string;
}

export interface ResetPasswordRequest {
  usuario_id: number;
  nova_senha: string;
  senha_atual?: string;
}

// === Dashboard ===

export type DashboardPeriodo = "today" | "month";

export interface DashboardMetrics {
  total_vendas: number;
  total_pedidos: number;
  ticket_medio: number;
  total_clientes: number;
  meta_mes?: number;
  atingimento_meta?: number; // 0..100
  periodo: DashboardPeriodo;
}

export interface VendaDiaria {
  dia: string;          // YYYY-MM-DD
  total_vendas: number;
  total_pedidos: number;
}

export interface VendasSeries {
  dias: number;
  pontos: VendaDiaria[];
}

export interface VendedorRanking {
  vendedor_id: number;
  vendedor_nome: string;
  total_vendas: number;
  total_pedidos: number;
  ticket_medio: number;
  meta?: number;
  atingimento_meta?: number; // 0..100
  posicao?: number;
}
