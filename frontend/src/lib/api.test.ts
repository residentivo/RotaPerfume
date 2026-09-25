/**
 * Contrato HTTP de lib/api.ts (URLs, metodos, corpo, query string e
 * normalizacao das respostas paginadas). A camada de transporte e mockada:
 * - fetchWithAuth (apiClient) para as rotas que usam o interceptador;
 * - fetch global para as listagens que chamam fetch direto.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./apiError";

const client = vi.hoisted(() => ({
  fetchWithAuth: vi.fn(),
  buildApiError: vi.fn(),
}));
vi.mock("./apiClient", () => client);

import * as api from "./api";

const BASE = "http://api.test";
const fetchMock = vi.fn();

beforeEach(() => {
  client.fetchWithAuth.mockReset();
  client.fetchWithAuth.mockResolvedValue({ ok: true });
  client.buildApiError.mockReset();
  client.buildApiError.mockImplementation((s: number, m: string) => new ApiError(s, m));
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

// ─── Rotas via fetchWithAuth ──────────────────────────────────────────────────

type Caso = [string, () => Promise<unknown>, string, Record<string, unknown>];

const body = (o: unknown) => JSON.stringify(o);

const CASOS: Caso[] = [
  ["apiLogin", () => api.apiLogin("a@x", "s", "tk"), "/api/auth/login", {
    method: "POST", body: body({ email: "a@x", password: "s", captchaToken: "tk" }), noRefresh: true,
  }],
  ["apiRefreshToken", () => api.apiRefreshToken("r1"), "/api/auth/refresh", {
    method: "POST", body: body({ refresh_token: "r1" }), noRefresh: true,
  }],
  ["apiMe", () => api.apiMe(), "/api/auth/me", { method: "GET" }],
  ["apiResetPassword (com senha atual)", () => api.apiResetPassword(3, "nova", "velha"), "/api/auth/reset-password", {
    method: "POST", body: body({ usuario_id: 3, nova_senha: "nova", senha_atual: "velha" }),
  }],
  ["apiResetPassword (sem senha atual)", () => api.apiResetPassword(3, "nova"), "/api/auth/reset-password", {
    method: "POST", body: body({ usuario_id: 3, nova_senha: "nova" }),
  }],
  ["apiChangePassword", () => api.apiChangePassword("a", "b", "tk"), "/api/auth/reset-password", {
    method: "POST", body: body({ senha_atual: "a", nova_senha: "b", captchaToken: "tk" }),
  }],
  ["apiCreateUser", () => api.apiCreateUser({ nome: "N", email: "e", role: "normal" }), "/api/usuarios", {
    method: "POST", body: body({ nome: "N", email: "e", role: "normal" }),
  }],
  ["apiUpdateUser", () => api.apiUpdateUser(4, { nome: "N" }), "/api/usuarios/4", { method: "PUT", body: body({ nome: "N" }) }],
  ["apiToggleUserStatus", () => api.apiToggleUserStatus(4, false), "/api/usuarios/4/inativar", {
    method: "PATCH", body: body({ ativo: false }),
  }],
  ["apiAdminResetPassword", () => api.apiAdminResetPassword(4), "/api/admin/reset-password", {
    method: "POST", body: body({ usuario_id: 4 }),
  }],
  ["apiDashboardMetrics", () => api.apiDashboardMetrics("week"), "/api/dashboard/metrics?periodo=week", { method: "GET" }],
  ["apiDashboardMetrics (padrao)", () => api.apiDashboardMetrics(), "/api/dashboard/metrics?periodo=month", { method: "GET" }],
  ["apiDashboardVendas", () => api.apiDashboardVendas(7), "/api/dashboard/vendas?dias=7", { method: "GET" }],
  ["apiDashboardVendas (padrao)", () => api.apiDashboardVendas(), "/api/dashboard/vendas?dias=30", { method: "GET" }],
  ["apiDashboardClientes", () => api.apiDashboardClientes("today"), "/api/dashboard/clientes?periodo=today", { method: "GET" }],
  ["apiDashboardClientes (padrao)", () => api.apiDashboardClientes(), "/api/dashboard/clientes?periodo=month", { method: "GET" }],
  ["apiListVendedores", () => api.apiListVendedores(), "/api/vendedores", { method: "GET" }],
  ["apiGetVendedor", () => api.apiGetVendedor(3), "/api/vendedores/3", { method: "GET" }],
  ["apiCreateVendedor", () => api.apiCreateVendedor({ nome: "V", regiao: "S", uf: "SP", meta_mensal: 1 }), "/api/vendedores", {
    method: "POST", body: body({ nome: "V", regiao: "S", uf: "SP", meta_mensal: 1 }),
  }],
  ["apiUpdateVendedor", () => api.apiUpdateVendedor(3, { nome: "V", regiao: "S", uf: "SP", meta_mensal: 1 }), "/api/vendedores/3", {
    method: "PUT", body: body({ nome: "V", regiao: "S", uf: "SP", meta_mensal: 1 }),
  }],
  ["apiDeleteVendedor", () => api.apiDeleteVendedor(3), "/api/vendedores/3", { method: "DELETE" }],
  ["apiReativarVendedor", () => api.apiReativarVendedor(3), "/api/vendedores/3/reativar", { method: "POST" }],
  ["apiVincularCliente", () => api.apiVincularCliente(3, 10), "/api/vendedores/3/clientes", {
    method: "POST", body: body({ cliente_id: 10 }),
  }],
  ["apiDesvincularCliente", () => api.apiDesvincularCliente(3, 10), "/api/vendedores/3/clientes/10", { method: "DELETE" }],
  ["apiListClientesDoVendedor", () => api.apiListClientesDoVendedor(3), "/api/vendedores/3/clientes", { method: "GET" }],
  ["apiGetCliente", () => api.apiGetCliente(10), "/api/clientes/10", { method: "GET" }],
  ["apiCreateCliente", () => api.apiCreateCliente({ razao_social: "L" } as never), "/api/clientes", {
    method: "POST", body: body({ razao_social: "L" }),
  }],
  ["apiUpdateCliente", () => api.apiUpdateCliente(10, { razao_social: "L" } as never), "/api/clientes/10", {
    method: "PUT", body: body({ razao_social: "L" }),
  }],
  ["apiToggleClienteStatus (com ativo)", () => api.apiToggleClienteStatus(10, true), "/api/clientes/10/inativar", {
    method: "PATCH", body: body({ ativo: true }),
  }],
  ["apiToggleClienteStatus (sem ativo)", () => api.apiToggleClienteStatus(10), "/api/clientes/10/inativar", {
    method: "PATCH", body: undefined,
  }],
  ["apiGetProduto", () => api.apiGetProduto(50), "/api/produtos/50", { method: "GET" }],
  ["apiCreateProduto", () => api.apiCreateProduto({ sku: "P" } as never), "/api/produtos", { method: "POST", body: body({ sku: "P" }) }],
  ["apiUpdateProduto", () => api.apiUpdateProduto(50, { sku: "P" } as never), "/api/produtos/50", { method: "PUT", body: body({ sku: "P" }) }],
  ["apiToggleProdutoStatus (com ativo)", () => api.apiToggleProdutoStatus(50, false), "/api/produtos/50/inativar", {
    method: "PATCH", body: body({ ativo: false }),
  }],
  ["apiToggleProdutoStatus (sem ativo)", () => api.apiToggleProdutoStatus(50), "/api/produtos/50/inativar", {
    method: "PATCH", body: undefined,
  }],
  ["apiGetPedido", () => api.apiGetPedido(5), "/api/pedidos/5", { method: "GET" }],
  ["apiCreatePedido", () => api.apiCreatePedido({ cliente_id: 1 } as never), "/api/pedidos", { method: "POST", body: body({ cliente_id: 1 }) }],
  ["apiUpdatePedido", () => api.apiUpdatePedido(5, { cliente_id: 1 } as never), "/api/pedidos/5", { method: "PUT", body: body({ cliente_id: 1 }) }],
  ["apiDeletePedido", () => api.apiDeletePedido(5), "/api/pedidos/5", { method: "DELETE" }],
  ["apiGetPagamento", () => api.apiGetPagamento(6), "/api/pagamentos/6", { method: "GET" }],
  ["apiCreatePagamento", () => api.apiCreatePagamento({ pedido_id: 1 } as never), "/api/pagamentos", { method: "POST", body: body({ pedido_id: 1 }) }],
  ["apiUpdatePagamento", () => api.apiUpdatePagamento(6, { valor: 1 } as never), "/api/pagamentos/6", { method: "PUT", body: body({ valor: 1 }) }],
  ["apiDeletePagamento", () => api.apiDeletePagamento(6), "/api/pagamentos/6", { method: "DELETE" }],
  ["apiGetOportunidade", () => api.apiGetOportunidade(7), "/api/oportunidades/7", { method: "GET" }],
  ["apiCreateOportunidade", () => api.apiCreateOportunidade({ etapa: "X" } as never), "/api/oportunidades", { method: "POST", body: body({ etapa: "X" }) }],
  ["apiUpdateOportunidade", () => api.apiUpdateOportunidade(7, { etapa: "X" } as never), "/api/oportunidades/7", { method: "PUT", body: body({ etapa: "X" }) }],
  ["apiDeleteOportunidade", () => api.apiDeleteOportunidade(7), "/api/oportunidades/7", { method: "DELETE" }],
  ["apiGetVisita", () => api.apiGetVisita(8), "/api/visitas/8", { method: "GET" }],
  ["apiCreateVisita", () => api.apiCreateVisita({ resultado: "X" } as never), "/api/visitas", { method: "POST", body: body({ resultado: "X" }) }],
  ["apiUpdateVisita", () => api.apiUpdateVisita(8, { resultado: "X" } as never), "/api/visitas/8", { method: "PUT", body: body({ resultado: "X" }) }],
  ["apiDeleteVisita", () => api.apiDeleteVisita(8), "/api/visitas/8", { method: "DELETE" }],
  ["apiGetEstoque", () => api.apiGetEstoque(9), "/api/estoque/9", { method: "GET" }],
  ["apiCreateEstoque", () => api.apiCreateEstoque({ sku: "S", data_snapshot: "2026-01-01", saldo: 1 }), "/api/estoque", {
    method: "POST", body: body({ sku: "S", data_snapshot: "2026-01-01", saldo: 1 }),
  }],
  ["apiUpdateEstoque", () => api.apiUpdateEstoque(9, { saldo: 2 }), "/api/estoque/9", { method: "PUT", body: body({ saldo: 2 }) }],
];

describe("rotas via fetchWithAuth", () => {
  it.each(CASOS)("%s", async (_n, chamar, path, opts) => {
    await chamar();
    expect(client.fetchWithAuth).toHaveBeenCalledTimes(1);
    const [p, o] = client.fetchWithAuth.mock.calls[0];
    expect(p).toBe(path);
    expect(o).toEqual(opts);
  });

  it("propaga o erro do fetchWithAuth", async () => {
    client.fetchWithAuth.mockRejectedValue(new ApiError(403, "bloqueado"));
    await expect(api.apiListVendedores()).rejects.toThrow("bloqueado");
  });
});

// ─── Listagens com fetch direto ───────────────────────────────────────────────

function resposta(status: number, corpo: unknown, statusText = "Status") {
  const raw = corpo === undefined ? "" : typeof corpo === "string" ? corpo : JSON.stringify(corpo);
  return { ok: status >= 200 && status < 300, status, statusText, text: async () => raw };
}

interface Lista {
  nome: string;
  chamar: () => Promise<{ data: unknown[]; page: number; limit: number; total: number; pages: number }>;
  /** Chamada minima (pagina 2, limit 5, sem filtros). */
  minima: () => Promise<{ data: unknown[]; page: number; limit: number; total: number; pages: number }>;
  url: string;
  urlMinima: string;
  /** Aceita o envelope { data, pagination }. */
  pagination: boolean;
}

const LISTAS: Lista[] = [
  {
    nome: "apiListUsers",
    chamar: () => api.apiListUsers(1, 20, "nome", "asc"),
    minima: () => api.apiListUsers(2, 5),
    url: "/api/usuarios?page=1&limit=20&order_by=nome&order_dir=asc",
    urlMinima: "/api/usuarios?page=2&limit=5",
    pagination: true,
  },
  {
    nome: "apiDashboardVendedores",
    chamar: () => api.apiDashboardVendedores(1, 10),
    minima: () => api.apiDashboardVendedores(2, 5),
    url: "/api/dashboard/vendedores?page=1&limit=10",
    urlMinima: "/api/dashboard/vendedores?page=2&limit=5",
    pagination: false,
  },
  {
    nome: "apiListSenhaHistorico",
    chamar: () => api.apiListSenhaHistorico(1, 20, 4, "admin", "created_at", "desc"),
    minima: () => api.apiListSenhaHistorico(2, 5),
    url: "/api/senha-historico/4?page=1&limit=20&usuario_id=4&tipo=admin&order_by=created_at&order_dir=desc",
    urlMinima: "/api/senha-historico?page=2&limit=5",
    pagination: false,
  },
  {
    nome: "apiListClientes",
    chamar: () => api.apiListClientes(1, 20, { uf: "SP", segmento: "Varejo", ativo: false, q: "lo" }, "id", "asc"),
    minima: () => api.apiListClientes(2, 5),
    url: "/api/clientes?page=1&limit=20&uf=SP&segmento=Varejo&ativo=false&q=lo&order_by=id&order_dir=asc",
    urlMinima: "/api/clientes?page=2&limit=5",
    pagination: true,
  },
  {
    nome: "apiListProdutos",
    chamar: () => api.apiListProdutos(1, 20, { categoria: "C", marca: "M", ativo: true, q: "p" }, "id", "desc"),
    minima: () => api.apiListProdutos(2, 5),
    url: "/api/produtos?page=1&limit=20&categoria=C&marca=M&ativo=true&q=p&order_by=id&order_dir=desc",
    urlMinima: "/api/produtos?page=2&limit=5",
    pagination: true,
  },
  {
    nome: "apiListPedidos",
    chamar: () =>
      api.apiListPedidos(
        1,
        20,
        { status: "Faturado", canal: "App", cliente_id: 1, vendedor_id: 2, data_inicio: "2026-01-01", data_fim: "2026-02-01", q: "x" },
        "id",
        "desc"
      ),
    minima: () => api.apiListPedidos(2, 5),
    url: "/api/pedidos?page=1&limit=20&status=Faturado&canal=App&cliente_id=1&vendedor_id=2&data_inicio=2026-01-01&data_fim=2026-02-01&q=x&order_by=id&order_dir=desc",
    urlMinima: "/api/pedidos?page=2&limit=5",
    pagination: true,
  },
  {
    nome: "apiListPagamentos",
    chamar: () =>
      api.apiListPagamentos(
        1,
        20,
        { status_pagamento: "Pago", forma_pagamento: "PIX", pedido_id: 3, vencimento_de: "2026-01-01", vencimento_ate: "2026-02-01" },
        "valor",
        "asc"
      ),
    minima: () => api.apiListPagamentos(2, 5),
    url: "/api/pagamentos?page=1&limit=20&status_pagamento=Pago&forma_pagamento=PIX&pedido_id=3&vencimento_de=2026-01-01&vencimento_ate=2026-02-01&order_by=valor&order_dir=asc",
    urlMinima: "/api/pagamentos?page=2&limit=5",
    pagination: true,
  },
  {
    nome: "apiListOportunidades",
    chamar: () =>
      api.apiListOportunidades(
        1,
        20,
        { cliente_id: 1, vendedor_id: 2, etapa: "E", origem: "O", data_abertura_de: "2026-01-01", data_abertura_ate: "2026-02-01", q: "x" },
        "id",
        "desc"
      ),
    minima: () => api.apiListOportunidades(2, 5),
    url: "/api/oportunidades?page=1&limit=20&cliente_id=1&vendedor_id=2&etapa=E&origem=O&data_abertura_de=2026-01-01&data_abertura_ate=2026-02-01&q=x&order_by=id&order_dir=desc",
    urlMinima: "/api/oportunidades?page=2&limit=5",
    pagination: true,
  },
  {
    nome: "apiListVisitas",
    chamar: () =>
      api.apiListVisitas(
        1,
        20,
        { cliente_id: 1, vendedor_id: 2, resultado: "R", data_visita_de: "2026-01-01", data_visita_ate: "2026-02-01", q: "x" },
        "id",
        "desc"
      ),
    minima: () => api.apiListVisitas(2, 5),
    url: "/api/visitas?page=1&limit=20&cliente_id=1&vendedor_id=2&resultado=R&data_visita_de=2026-01-01&data_visita_ate=2026-02-01&q=x&order_by=id&order_dir=desc",
    urlMinima: "/api/visitas?page=2&limit=5",
    pagination: true,
  },
  {
    nome: "apiListEstoque",
    chamar: () =>
      api.apiListEstoque(1, 20, { sku: "S", data_de: "2026-01-01", data_ate: "2026-02-01", ruptura: false }, "sku", "asc"),
    minima: () => api.apiListEstoque(2, 5),
    url: "/api/estoque?page=1&limit=20&sku=S&data_de=2026-01-01&data_ate=2026-02-01&ruptura=false&order_by=sku&order_dir=asc",
    urlMinima: "/api/estoque?page=2&limit=5",
    pagination: true,
  },
];

describe.each(LISTAS)("$nome", (l) => {
  it("monta a URL com filtros e ordenacao, com cookie (credentials: include)", async () => {
    fetchMock.mockResolvedValue(resposta(200, []));
    await l.chamar();
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${BASE}${l.url}`);
    expect(init).toMatchObject({ method: "GET", credentials: "include" });
  });

  it("sem filtros envia so page e limit", async () => {
    fetchMock.mockResolvedValue(resposta(200, []));
    await l.minima();
    expect(fetchMock.mock.calls[0][0]).toBe(`${BASE}${l.urlMinima}`);
  });

  it.each<[string, unknown, { data: unknown[]; page: number; limit: number; total: number; pages: number }]>([
    ["envelope com paginacao no topo", { data: [1, 2], page: 3, limit: 2, total: 9, pages: 5 }, { data: [1, 2], page: 3, limit: 2, total: 9, pages: 5 }],
    ["envelope sem paginacao usa os padroes", { data: [1, 2, 3] }, { data: [1, 2, 3], page: 2, limit: 5, total: 3, pages: 1 }],
    ["array puro", [7, 8], { data: [7, 8], page: 2, limit: 5, total: 2, pages: 1 }],
    ["objeto sem data", { foo: 1 }, { data: [], page: 2, limit: 5, total: 0, pages: 0 }],
    ["corpo vazio", undefined, { data: [], page: 2, limit: 5, total: 0, pages: 0 }],
    ["texto nao-JSON", "ok", { data: [], page: 2, limit: 5, total: 0, pages: 0 }],
  ])("resposta: %s", async (_n, corpo, esperado) => {
    fetchMock.mockResolvedValue(resposta(200, corpo));
    await expect(l.minima()).resolves.toEqual(esperado);
  });

  it.runIf(l.pagination)("aceita o envelope { data, pagination }", async () => {
    fetchMock.mockResolvedValue(
      resposta(200, { data: [1], pagination: { page: 4, limit: 1, total: 8, pages: 8 } })
    );
    await expect(l.minima()).resolves.toEqual({ data: [1], page: 4, limit: 1, total: 8, pages: 8 });
  });

  it.each<[string, unknown, string]>([
    ["campo error", { error: "acesso bloqueado: vendedor desligado" }, "acesso bloqueado: vendedor desligado"],
    ["campo message", { message: "falhou" }, "falhou"],
    ["sem corpo", undefined, "Erro 500: Server Error"],
    ["corpo texto", "boom", "Erro 500: Server Error"],
  ])("erro HTTP com %s vira ApiError via buildApiError", async (_n, corpo, msg) => {
    const status = msg.startsWith("acesso") ? 403 : 500;
    fetchMock.mockResolvedValue(resposta(status, corpo, "Server Error"));
    const err = await l.minima().catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toBe(msg);
    expect(err.status).toBe(status);
    expect(client.buildApiError).toHaveBeenCalledWith(status, msg);
  });
});
