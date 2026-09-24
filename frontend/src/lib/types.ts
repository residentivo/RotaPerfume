// === Senha Historico ===

export type TipoReset =
  | "usuario"
  | "admin"
  | "primeiro_acesso"
  | "esquecimento";

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
  data_desligamento: string | null; // null = vendedor ativo; qualquer data = inativo
}

// === Vendedores (CRUD completo) ===
//
// Vendedor acima e o formato resumido de GET /api/vendedores (lista simples,
// usada em selects). VendedorCompleto e o formato retornado no
// header de GET/POST/PUT/DELETE /api/vendedores/{id} (models.Vendedor no
// backend, ver apis/shared/models/vendedor.go).

export interface VendedorCompleto {
  id: number;
  nome: string;
  regiao: string;
  uf: string;
  data_admissao: string; // formato AAAA-MM-DD (ou ISO datetime)
  data_desligamento: string | null; // null = vendedor ativo
  meta_mensal: number;
  created_at: string;
  updated_at: string;
}

// Cliente vinculado a um vendedor via carteira ativa (ver
// repositories.ClienteResumo no backend). Retornado dentro de
// VendedorDetalhe.clientes.
export interface ClienteResumo {
  id: number;
  cnpj: string;
  razao_social: string;
  segmento: string;
  cidade: string;
  uf: string;
  carteira_id: number;
  data_inicio: string;
  data_fim: string | null;
}

// Vendedor com a lista de clientes vinculados (carteira ativa), retornado
// por GET /api/vendedores/{id} (services.VendedorDetalhe no backend).
export interface VendedorDetalhe extends VendedorCompleto {
  clientes: ClienteResumo[];
}

// Payload usado tanto para POST /api/vendedores (criar) quanto para
// PUT /api/vendedores/{id} (editar). data_admissao e opcional apenas na
// criacao (default hoje no backend); data_desligamento nao e editavel por
// esta rota (ver apiDeleteVendedor / soft-delete).
export interface VendedorInput {
  nome: string;
  regiao: string;
  uf: string;
  data_admissao?: string; // formato AAAA-MM-DD
  meta_mensal: number;
}

// Nome do campo do token do Cloudflare Turnstile enviado ao backend.
// FrontBrain implementou usando "captchaToken" por padrão — se o BackBrain
// definir outro nome (ex: "turnstileToken"), ajustar aqui e em
// apiLogin/apiChangePassword (lib/api.ts).
export interface LoginRequest {
  email: string;
  password: string;
  captchaToken?: string;
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
  // Nome do campo alinhado com LoginRequest.captchaToken — ver comentário lá.
  captchaToken?: string;
}

// === Dashboard ===

export type DashboardPeriodo = "today" | "week" | "month";

export interface DashboardMetrics {
  total_vendas: number;
  total_pedidos: number;
  ticket_medio: number;
  total_clientes: number;
  meta_mes?: number;
  atingimento_meta?: number; // 0..100
  periodo: DashboardPeriodo;
  // true quando o usuario normal esta vinculado a um vendedor com
  // data_desligamento; nesse caso o backend devolve todos os numeros
  // zerados e as listas vazias. Sempre false/ausente para admin.
  vendedor_desligado?: boolean;
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

// === Pedidos ===

// Canais e status aceitos pelo backend (ver services.canaisValidos /
// statusValidos em apis/rotaperfumes-api/services/pedido_service.go).
export type PedidoCanal = "App" | "Telefone" | "Visita" | "WhatsApp";
export type PedidoStatus =
  | "Cancelado"
  | "Em separação"
  | "Entregue"
  | "Faturado";

// Pedido (cabecalho), como retornado por GET /api/pedidos e no cabecalho de
// GET /api/pedidos/{id}. cliente_nome/vendedor_nome vem via JOIN no backend.
export interface Pedido {
  pedido_id_origem: number;
  cliente_id: number;
  vendedor_id: number;
  data_pedido: string; // formato AAAA-MM-DD
  canal: string;
  status: string;
  valor_total: number;
  created_at: string;
  updated_at: string;
  cliente_nome: string;
  vendedor_nome: string;
}

// Item de pedido, como retornado dentro de itens de GET /api/pedidos/{id}.
// produto_sku/produto_descricao vem via JOIN no backend.
export interface ItemPedido {
  item_id_origem: number;
  pedido_id: number;
  produto_id: number;
  quantidade: number;
  preco_praticado: number;
  desconto_pct: number;
  valor_bruto: number;
  created_at: string;
  updated_at: string;
  produto_sku: string;
  produto_descricao: string;
}

// Pedido com itens, retornado por GET /api/pedidos/{id} (e pelo POST/PUT).
export interface PedidoDetalhe extends Pedido {
  itens: ItemPedido[];
}

// Item no payload de criacao/edicao de pedido (POST/PUT /api/pedidos).
export interface ItemPedidoInput {
  produto_id: number;
  quantidade: number;
  preco_praticado: number;
  desconto_pct: number;
}

// Payload usado tanto para POST /api/pedidos (criar) quanto para
// PUT /api/pedidos/{id} (editar, substitui a lista de itens integralmente).
export interface PedidoInput {
  cliente_id: number;
  vendedor_id: number;
  data_pedido: string; // formato AAAA-MM-DD
  canal: string;
  status: string;
  itens: ItemPedidoInput[];
}

// === Pagamentos ===
//
// Tela de acesso comum (qualquer usuário autenticado, admin ou normal) —
// ver frontend/src/app/pagamentos/page.tsx. Diferente das demais telas
// administrativas (Clientes/Produtos/Pedidos/Usuários), que ficam sob
// /admin/* e exigem role admin.

// Valores de ENUM aceitos pelo backend (ver
// apis/rotaperfumes-api/services/pagamento_service.go).
export type FormaPagamento =
  | "Boleto 14 dias"
  | "Boleto 28 dias"
  | "Cartão de crédito"
  | "Cartão de débito"
  | "Cheque a prazo"
  | "Dinheiro"
  | "PIX";

export type StatusPagamento =
  | "Em aberto"
  | "Inadimplente"
  | "Pago"
  | "Pago com atraso";

// Pagamento, como retornado por GET /api/pagamentos e GET /api/pagamentos/{id}.
// Diferente das demais tabelas, a PK e literalmente pagamento_id (nao ha
// coluna id separada) — ver apis/shared/models/pagamento.go.
export interface Pagamento {
  pagamento_id: number;
  pedido_id: number;
  forma_pagamento: string;
  parcelas: number;
  valor: number;
  taxa_pct: number;
  valor_liquido: number;
  data_vencimento: string; // formato AAAA-MM-DD
  data_pagamento: string | null; // formato AAAA-MM-DD ou null
  status_pagamento: string;
  created_at: string;
  updated_at: string;
}

// Payload de POST /api/pagamentos (criar). pedido_id e obrigatorio e
// imutavel apos a criacao.
export interface PagamentoCreateInput {
  pedido_id: number;
  forma_pagamento: string;
  parcelas: number;
  valor: number;
  taxa_pct: number;
  valor_liquido: number;
  data_vencimento: string; // formato AAAA-MM-DD, obrigatorio
  data_pagamento?: string; // formato AAAA-MM-DD, opcional
  status_pagamento: string;
}

// Payload de PUT /api/pagamentos/{id} (editar). pagamento_id e pedido_id
// NAO sao aceitos/editaveis por esta rota (o backend rejeita alteracao do
// vinculo com o pedido de origem).
export interface PagamentoUpdateInput {
  forma_pagamento: string;
  parcelas: number;
  valor: number;
  taxa_pct: number;
  valor_liquido: number;
  data_vencimento: string; // formato AAAA-MM-DD, obrigatorio
  data_pagamento?: string; // formato AAAA-MM-DD, opcional
  status_pagamento: string;
}

// Filtros aceitos por GET /api/pagamentos.
export interface ListPagamentosFilters {
  status_pagamento?: string;
  forma_pagamento?: string;
  pedido_id?: number;
  vencimento_de?: string; // formato AAAA-MM-DD
  vencimento_ate?: string; // formato AAAA-MM-DD
}

// === Oportunidades (CRM) ===
//
// Tela admin-only — ver apis/rotaperfumes-api/handlers (rota /api/oportunidades).

export type OportunidadeEtapa =
  | "Prospecção"
  | "Qualificação"
  | "Proposta enviada"
  | "Negociação"
  | "Fechado ganho"
  | "Fechado perdido";

export type OportunidadeOrigem =
  | "WhatsApp"
  | "Indicação"
  | "Inbound site"
  | "Instagram"
  | "Feira de beleza"
  | "Reativação"
  | "Prospecção ativa";

// Oportunidade, como retornada por GET /api/oportunidades e
// GET /api/oportunidades/{id}.
export interface Oportunidade {
  oportunidade_id: number;
  cliente_id: number;
  vendedor_id: number;
  origem: string;
  data_abertura: string; // formato AAAA-MM-DD
  etapa: string;
  probabilidade_pct: number;
  valor_estimado: number;
  data_fechamento: string | null; // formato AAAA-MM-DD ou null
  ciclo_dias: number | null;
  motivo_perda: string | null;
  created_at: string;
  updated_at: string;
}

// Payload usado tanto para POST /api/oportunidades (criar) quanto para
// PUT /api/oportunidades/{id} (editar). motivo_perda e exigido pelo backend
// quando etapa = "Fechado perdido".
export interface OportunidadeInput {
  cliente_id: number;
  vendedor_id: number;
  origem: string;
  data_abertura?: string; // formato AAAA-MM-DD
  etapa: string;
  probabilidade_pct: number;
  valor_estimado: number;
  data_fechamento?: string; // formato AAAA-MM-DD
  ciclo_dias?: number;
  motivo_perda?: string;
}

// Filtros aceitos por GET /api/oportunidades.
export interface ListOportunidadesFilters {
  cliente_id?: number;
  vendedor_id?: number;
  etapa?: string;
  origem?: string;
  data_abertura_de?: string; // formato AAAA-MM-DD
  data_abertura_ate?: string; // formato AAAA-MM-DD
  q?: string;
}

export interface ListOportunidadesResponse {
  data: Oportunidade[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

// === Visitas (CRM) ===
//
// Tela admin-only — ver apis/rotaperfumes-api/handlers/visita_handler.go
// (rota /api/visitas). Mesmo padrao de Oportunidades, porem data_visita e
// sempre obrigatoria (sem default no backend).

// Valores observados no dataset de origem (ver sql/16_ddl_visitas.sql).
// Coluna e VARCHAR(40) livre no backend, entao novos valores nao quebram a
// tela — apenas nao aparecerao pre-listados nos selects de filtro/form.
export type VisitaResultado =
  | "Sem pedido"
  | "Pedido realizado"
  | "Reagendada"
  | "Cliente ausente"
  | "Apenas relacionamento";

// Visita, como retornada por GET /api/visitas e GET /api/visitas/{id}.
export interface Visita {
  visita_id: number;
  cliente_id: number;
  vendedor_id: number;
  data_visita: string; // formato AAAA-MM-DD
  resultado: string;
  duracao_min: number;
  created_at: string;
  updated_at: string;
}

// Payload usado tanto para POST /api/visitas (criar) quanto para
// PUT /api/visitas/{id} (editar). Diferente de OportunidadeInput,
// data_visita e sempre obrigatoria (backend nao aplica default).
export interface VisitaInput {
  cliente_id: number;
  vendedor_id: number;
  data_visita: string; // formato AAAA-MM-DD
  resultado: string;
  duracao_min: number;
}

// Filtros aceitos por GET /api/visitas.
export interface ListVisitasFilters {
  cliente_id?: number;
  vendedor_id?: number;
  resultado?: string;
  data_visita_de?: string; // formato AAAA-MM-DD
  data_visita_ate?: string; // formato AAAA-MM-DD
  q?: string;
}

export interface ListVisitasResponse {
  data: Visita[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}

// === Estoque ===
//
// Tela de acesso comum para leitura (qualquer usuário autenticado); criar e
// editar são admin-only — ver apis/rotaperfumes-api/handlers/estoque_handler.go
// (rota /api/estoque).

// Estoque, como retornado por GET /api/estoque e GET /api/estoque/{id}.
// Sem data_de/data_ate no filtro da listagem, a API devolve a última
// posição de estoque de cada SKU; com data_ate, devolve a última posição de
// cada SKU até (inclusive) aquela data.
export interface Estoque {
  id: number;
  data_snapshot: string; // formato AAAA-MM-DD
  sku: string;
  produto_descricao: string;
  saldo: number;
  ruptura: boolean;
  created_at: string;
  updated_at: string;
}

// Payload de POST /api/estoque (criar, admin only). ruptura NÃO é enviado —
// é derivado automaticamente pelo backend a partir do saldo.
// Payload de PUT /api/estoque/{id} (editar, admin only) usa apenas `saldo`
// (sku e data_snapshot não são editáveis após a criação).
export interface EstoqueInput {
  sku: string;
  data_snapshot: string; // formato AAAA-MM-DD
  saldo: number;
}

// Filtros aceitos por GET /api/estoque.
export interface ListEstoqueFilters {
  sku?: string;
  data_de?: string; // formato AAAA-MM-DD
  data_ate?: string; // formato AAAA-MM-DD
  ruptura?: boolean;
}

export interface ListEstoqueResponse {
  data: Estoque[];
  page: number;
  limit: number;
  total: number;
  pages: number;
}
