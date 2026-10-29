/**
 * DASH-01: apiDashboardFunil sobre o transporte real (apiClient) com fetch
 * stubado. A URL leva só ?janela= (nunca vendedor_id: o escopo é do
 * backend), o envelope {success, data} é desembrulhado e o erro vira
 * ApiError com o status.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FunilJanela } from "@/lib/types";

type ApiModule = typeof import("@/lib/api");
type ApiErrorModule = typeof import("@/lib/apiError");

let api: ApiModule;
let apiError: ApiErrorModule;
let fetchMock: ReturnType<typeof vi.fn>;

const FUNIL = {
  janela: "30d",
  total_oportunidades: 3,
  etapas: [{ etapa: "Prospecção", quantidade: 3, valor_estimado: 10, conversao_anterior_pct: 100, conversao_topo_pct: 100 }],
  perdidas: { quantidade: 0, valor_estimado: 0 },
  taxa_ganho_pct: 0,
  vendedor_desligado: false,
};

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

beforeEach(async () => {
  vi.resetModules();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  apiError = await import("@/lib/apiError");
  api = await import("@/lib/api");
  vi.spyOn(console, "warn").mockImplementation(() => {});
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("apiDashboardFunil", () => {
  it.each<[FunilJanela | undefined, string]>([
    [undefined, "all"],
    ["all", "all"],
    ["30d", "30d"],
    ["90d", "90d"],
    ["365d", "365d"],
  ])("janela %s → GET /api/dashboard/funil?janela=%s, só esse parâmetro", async (janela, esperado) => {
    fetchMock.mockResolvedValue(jsonResponse(200, { success: true, data: FUNIL }));
    await (janela === undefined ? api.apiDashboardFunil() : api.apiDashboardFunil(janela));

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const u = new URL(url);
    expect(`${u.origin}${u.pathname}`).toBe("http://api.test/api/dashboard/funil");
    expect([...u.searchParams.keys()]).toEqual(["janela"]);
    expect(u.searchParams.get("janela")).toBe(esperado);
    expect(url).not.toContain("vendedor_id");
    expect(init.method).toBe("GET");
    expect(init.credentials).toBe("include");
    expect(init.body).toBeUndefined();
  });

  it("desembrulha o envelope e devolve só data", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { success: true, data: FUNIL }));
    await expect(api.apiDashboardFunil("30d")).resolves.toEqual(FUNIL);
  });

  it("valor fora do enum é codificado (não injeta parâmetros)", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { success: true, data: FUNIL }));
    await api.apiDashboardFunil("30d&vendedor_id=9" as FunilJanela);
    const u = new URL(fetchMock.mock.calls[0][0] as string);
    expect([...u.searchParams.keys()]).toEqual(["janela"]);
    expect(u.searchParams.get("janela")).toBe("30d&vendedor_id=9");
  });

  it.each<[number, string]>([
    [400, "janela deve ser '30d', '90d', '365d' ou 'all'"],
    [500, "erro interno"],
  ])("%i → ApiError com o status", async (status, msg) => {
    fetchMock.mockResolvedValue(jsonResponse(status, { success: false, error: msg }));
    const err = await api.apiDashboardFunil("all").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(apiError.ApiError);
    expect((err as InstanceType<ApiErrorModule["ApiError"]>).status).toBe(status);
  });
});
