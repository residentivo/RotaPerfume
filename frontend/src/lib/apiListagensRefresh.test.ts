/**
 * FE-05: as listagens paginadas de lib/api.ts passam pelo `fetchWithAuth`
 * (via `fetchEnvelopeWithAuth`). Aqui o apiClient e REAL e so o `fetch`
 * global e mockado, para cobrir o fluxo 401 -> refresh -> retry de ponta a
 * ponta sem perder a paginacao do envelope.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MSG_VENDEDOR_DESLIGADO } from "./vendedorDesligado";

type ApiModule = typeof import("./api");
type ApiClientModule = typeof import("./apiClient");
type VendedorDesligadoModule = typeof import("./vendedorDesligado");

let api: ApiModule;
let client: ApiClientModule;
let vd: VendedorDesligadoModule;
let fetchMock: ReturnType<typeof vi.fn>;

const BASE = "http://api.test";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function calledUrls(): string[] {
  return fetchMock.mock.calls.map((c) => String(c[0]));
}

const ENVELOPE = {
  success: true,
  data: [{ id: 1 }, { id: 2 }],
  pagination: { page: 2, limit: 2, total: 7, pages: 4 },
};
const ESPERADO = { data: [{ id: 1 }, { id: 2 }], page: 2, limit: 2, total: 7, pages: 4 };

beforeEach(async () => {
  // apiClient tem estado de modulo (lock de refresh, flag de redirect).
  vi.resetModules();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  vd = await import("./vendedorDesligado");
  client = await import("./apiClient");
  api = await import("./api");
  vi.spyOn(console, "warn").mockImplementation(() => {});
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchEnvelopeWithAuth", () => {
  it("devolve o corpo inteiro (nao desenvelopa data) e nao repassa a flag ao fetch", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, ENVELOPE));
    await expect(client.fetchEnvelopeWithAuth("/api/x", { method: "GET" })).resolves.toEqual(
      ENVELOPE
    );
    const init = fetchMock.mock.calls[0][1] as Record<string, unknown>;
    expect(init).toMatchObject({ method: "GET", credentials: "include" });
    expect(init).not.toHaveProperty("keepEnvelope");
  });

  it("fetchWithAuth continua desenvelopando data", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(200, ENVELOPE))
      .mockResolvedValueOnce(jsonResponse(200, ENVELOPE));
    await client.fetchEnvelopeWithAuth("/api/x");
    await expect(client.fetchWithAuth("/api/x")).resolves.toEqual(ENVELOPE.data);
  });
});

type Chamada = [string, () => Promise<unknown>, string];

const LISTAGENS: Chamada[] = [
  ["apiListPedidos", () => api.apiListPedidos(2, 2), "/api/pedidos?page=2&limit=2"],
  ["apiListPagamentos", () => api.apiListPagamentos(2, 2), "/api/pagamentos?page=2&limit=2"],
  ["apiListClientes", () => api.apiListClientes(2, 2), "/api/clientes?page=2&limit=2"],
  ["apiListProdutos", () => api.apiListProdutos(2, 2), "/api/produtos?page=2&limit=2"],
  ["apiListOportunidades", () => api.apiListOportunidades(2, 2), "/api/oportunidades?page=2&limit=2"],
  ["apiListVisitas", () => api.apiListVisitas(2, 2), "/api/visitas?page=2&limit=2"],
  ["apiListEstoque", () => api.apiListEstoque(2, 2), "/api/estoque?page=2&limit=2"],
  ["apiListUsers", () => api.apiListUsers(2, 2), "/api/usuarios?page=2&limit=2"],
  ["apiListSenhaHistorico", () => api.apiListSenhaHistorico(2, 2), "/api/senha-historico?page=2&limit=2"],
  ["apiDashboardVendedores", () => api.apiDashboardVendedores(2, 2), "/api/dashboard/vendedores?page=2&limit=2"],
];

describe.each(LISTAGENS)("%s", (_nome, chamar, path) => {
  it("401 -> refresh -> retry: devolve a pagina com a paginacao do envelope", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(200, { success: true }))
      .mockResolvedValueOnce(jsonResponse(200, ENVELOPE));

    await expect(chamar()).resolves.toEqual(ESPERADO);

    expect(calledUrls()).toEqual([`${BASE}${path}`, `${BASE}/api/auth/refresh`, `${BASE}${path}`]);
  });
});

describe("apiListPedidos - demais caminhos pelo fetchWithAuth", () => {
  it("refresh falhou: rejeita com sessao expirada e faz logout", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(401, { error: "refresh invalido" }))
      // SEC-02: o retry unico da original tambem volta 401
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValue(jsonResponse(200, {}));

    await expect(api.apiListPedidos(1, 20)).rejects.toThrow("Sessão expirada");
    expect(calledUrls()).toContain(`${BASE}/api/auth/logout`);
  });

  it("duas listagens com 401 ao mesmo tempo fazem um unico refresh", async () => {
    let liberarRefresh: (r: Response) => void = () => {};
    fetchMock.mockImplementation((url: string) => {
      if (url.includes("/api/auth/refresh")) {
        return new Promise<Response>((r) => (liberarRefresh = r));
      }
      const jaTentou = fetchMock.mock.calls.filter((c) => c[0] === url).length > 1;
      return Promise.resolve(
        jaTentou ? jsonResponse(200, ENVELOPE) : jsonResponse(401, { error: "expirado" })
      );
    });

    const a = api.apiListPedidos(2, 2);
    const b = api.apiListClientes(2, 2);
    await vi.waitFor(() =>
      expect(calledUrls().filter((u) => u.includes("/api/auth/refresh"))).toHaveLength(1)
    );
    liberarRefresh(jsonResponse(200, { success: true }));

    await expect(a).resolves.toEqual(ESPERADO);
    await expect(b).resolves.toEqual(ESPERADO);
    expect(calledUrls().filter((u) => u.includes("/api/auth/refresh"))).toHaveLength(1);
  });

  it("erro HTTP vira ApiError com a mensagem do corpo", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(500, { error: "falhou" }));
    await expect(api.apiListPedidos(1, 20)).rejects.toMatchObject({
      status: 500,
      message: "falhou",
    });
  });

  it("SEC-02: refresh 401 (outra aba renovou) -> retry OK mantem a paginacao, sem logout", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(401, { error: "refresh token revogado" }))
      .mockResolvedValueOnce(jsonResponse(200, ENVELOPE));

    await expect(api.apiListPedidos(2, 2)).resolves.toEqual(ESPERADO);
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
  });

  it("403 de vendedor desligado notifica a sessao sem refresh", async () => {
    const listener = vi.fn();
    vd.subscribeVendedorDesligado(listener);
    fetchMock.mockResolvedValueOnce(jsonResponse(403, { error: MSG_VENDEDOR_DESLIGADO }));

    await expect(api.apiListPedidos(1, 20)).rejects.toMatchObject({ status: 403 });
    expect(listener).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
