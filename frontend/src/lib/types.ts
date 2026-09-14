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

export type UserRole = "admin" | "normal";

export interface User {
  id: number;
  nome: string;
  email: string;
  role: UserRole;
  ativo: boolean;
  created_at?: string;
  updated_at?: string;
  id_vendedor?: number | null;
  vendedor_nome?: string | null;
}

export interface Vendedor {
  id: number;
  nome: string;
  regiao: string;
  uf: string;
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

// === Clientes ===

export interface Cliente {
  id: number;
  cliente_id_origem: number;
  cnpj: string;
  razao_social: string;
  segmento: string;
  cidade: string;
  uf: string;
  bairro: string;
  data_cadastro: string;
  ativo: boolean;
  created_at: string;
  updated_at: string;
}

// Payload usado tanto para POST /api/clientes (criar) quanto para
// PUT /api/clientes/{id} (editar). Em criacao, data_cadastro e opcional
// (default hoje no backend); em edicao, e obrigatorio.
export interface ClienteInput {
  cnpj: string;
  razao_social: string;
  segmento: string;
  cidade: string;
  uf: string;
  bairro: string;
  data_cadastro?: string; // formato AAAA-MM-DD
}

export interface ClienteSegmentoTotal {
  segmento: string;
  total: number;
}

export interface ClienteUfTotal {
  uf: string;
  total: number;
}

export interface ClienteDashboardMetrics {
  periodo: DashboardPeriodo;
  total_clientes: number;
  total_ativos: number;
  total_inativos: number;
  novos_no_periodo: number;
  por_segmento: ClienteSegmentoTotal[];
  por_uf: ClienteUfTotal[];
}

// === Produtos ===

export interface Produto {
  id: number;
  sku: string;
  descricao: string;
  categoria: string;
  marca: string;
  nota_olfativa: string;
  preco_tabela: number;
  custo_unitario: number;
  unidade: string;
  data_lancamento: string | null;
  ativo: boolean;
  created_at: string;
  updated_at: string;
}

// Payload usado tanto para POST /api/produtos (criar) quanto para
// PUT /api/produtos/{id} (editar). sku e obrigatorio apenas na criacao
// (a rota de edicao nao permite alterar sku).
export interface ProdutoInput {
  sku: string;
  descricao: string;
  categoria: string;
  marca: string;
  nota_olfativa?: string;
  preco_tabela: number;
  custo_unitario: number;
  unidade: string;
  data_lancamento?: string; // formato AAAA-MM-DD
}
