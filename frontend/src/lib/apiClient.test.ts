import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MSG_VENDEDOR_DESLIGADO } from "./vendedorDesligado";

// apiClient tem estado de modulo (lock de refresh, flag de redirect): cada
// teste importa uma instancia nova via vi.resetModules().
type ApiClientModule = typeof import("./apiClient");
type VendedorDesligadoModule = typeof import("./vendedorDesligado");

let client: ApiClientModule;
let vd: VendedorDesligadoModule;
let fetchMock: ReturnType<typeof vi.fn>;

function jsonResponse(status: number, body: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function calledUrls(): string[] {
  return fetchMock.mock.calls.map((c) => String(c[0]));
}

const USER_JSON = JSON.stringify({ id: 1, nome: "Ana", role: "normal" });

beforeEach(async () => {
  vi.resetModules();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  vd = await import("./vendedorDesligado");
  client = await import("./apiClient");
  localStorage.setItem("auth_user", USER_JSON);
  vi.spyOn(console, "warn").mockImplementation(() => {});
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("buildApiError", () => {
  it.each<[number, string, boolean]>([
    [403, MSG_VENDEDOR_DESLIGADO, true],
    [403, "acesso negado", false],
    [401, MSG_VENDEDOR_DESLIGADO, false],
    [500, MSG_VENDEDOR_DESLIGADO, false],
  ])("status %i + %j -> notifica sessao: %s", (status, msg, notifica) => {
    const listener = vi.fn();
    vd.subscribeVendedorDesligado(listener);

    const err = client.buildApiError(status, msg);

    expect(err).toBeInstanceOf(client.ApiError);
    expect(err.status).toBe(status);
    expect(err.message).toBe(msg);
    expect(listener).toHaveBeenCalledTimes(notifica ? 1 : 0);
  });
});

describe("fetchWithAuth - 403 vendedor desligado", () => {
  it("rejeita com ApiError 403, notifica a sessao e NAO faz refresh/logout", async () => {
    const listener = vi.fn();
    vd.subscribeVendedorDesligado(listener);
    fetchMock.mockResolvedValueOnce(
      jsonResponse(403, { success: false, error: MSG_VENDEDOR_DESLIGADO })
    );

    const err = await client.fetchWithAuth("/api/clientes").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(client.ApiError);
    expect(err).toMatchObject({ status: 403, message: MSG_VENDEDOR_DESLIGADO });
    expect(vd.isVendedorDesligadoError(err)).toBe(true);

    expect(listener).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(calledUrls()).toEqual(["http://api.test/api/clientes"]);
    expect(calledUrls().some((u) => u.includes("/api/auth/refresh"))).toBe(false);
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
    // Sessao local nao e derrubada.
    expect(localStorage.getItem("auth_user")).toBe(USER_JSON);
  });

  it.each<[string, number, unknown]>([
    ["403 generico", 403, { error: "acesso negado" }],
    ["403 com message", 403, { message: "proibido" }],
    ["404", 404, { error: "nao encontrado" }],
    ["500 sem corpo", 500, undefined],
  ])("%s nao notifica a sessao nem faz refresh", async (_n, status, body) => {
    const listener = vi.fn();
    vd.subscribeVendedorDesligado(listener);
    fetchMock.mockResolvedValueOnce(jsonResponse(status, body));

    const err = await client.fetchWithAuth("/api/x").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(client.ApiError);
    expect(err).toMatchObject({ status });
    expect(listener).not.toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("usa 'Erro <status>: <statusText>' quando o corpo nao tem error/message", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response("texto puro", { status: 502, statusText: "Bad Gateway" })
    );
    await expect(client.fetchWithAuth("/api/x")).rejects.toMatchObject({
      status: 502,
      message: "Erro 502: Bad Gateway",
    });
  });
});

describe("fetchWithAuth - 401 continua disparando refresh", () => {
  it("401 -> POST /api/auth/refresh -> reenvia a requisicao original", async () => {
    const listener = vi.fn();
    vd.subscribeVendedorDesligado(listener);
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(200, { success: true }))
      .mockResolvedValueOnce(jsonResponse(200, { success: true, data: { ok: 1 } }));

    await expect(client.fetchWithAuth("/api/pedidos")).resolves.toEqual({ ok: 1 });

    expect(calledUrls()).toEqual([
      "http://api.test/api/pedidos",
      "http://api.test/api/auth/refresh",
      "http://api.test/api/pedidos",
    ]);
    expect(fetchMock.mock.calls[1][1]).toMatchObject({
      method: "POST",
      credentials: "include",
    });
    expect(listener).not.toHaveBeenCalled();
  });

  it("401 -> refresh OK -> retry com 403 desligado: rejeita e notifica, sem logout", async () => {
    const listener = vi.fn();
    vd.subscribeVendedorDesligado(listener);
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(200, {}))
      .mockResolvedValueOnce(jsonResponse(403, { error: MSG_VENDEDOR_DESLIGADO }));

    await expect(client.fetchWithAuth("/api/visitas")).rejects.toMatchObject({
      status: 403,
      message: MSG_VENDEDOR_DESLIGADO,
    });
    expect(listener).toHaveBeenCalledTimes(1);
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
    expect(localStorage.getItem("auth_user")).toBe(USER_JSON);
  });

  it("401 concorrentes compartilham um unico refresh", async () => {
    let releaseRefresh: (r: Response) => void = () => {};
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/api/auth/refresh")) {
        return new Promise<Response>((res) => {
          releaseRefresh = res;
        });
      }
      const n = fetchMock.mock.calls.filter((c) => c[0] === url).length;
      return Promise.resolve(
        n === 1
          ? jsonResponse(401, { error: "expirado" })
          : jsonResponse(200, { data: url.slice(-1) })
      );
    });

    const pa = client.fetchWithAuth("/api/a");
    const pb = client.fetchWithAuth("/api/b");
    await vi.waitFor(() => {
      expect(calledUrls().filter((u) => u.endsWith("/api/auth/refresh"))).toHaveLength(1);
      expect(calledUrls().filter((u) => u.endsWith("/api/b"))).toHaveLength(1);
    });
    releaseRefresh(jsonResponse(200, {}));

    await expect(pa).resolves.toBe("a");
    await expect(pb).resolves.toBe("b");
    expect(calledUrls().filter((u) => u.endsWith("/api/auth/refresh"))).toHaveLength(1);
  });

  it("401 -> refresh falha -> limpa o usuario e chama logout", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(401, { error: "refresh invalido" }))
      // logout: nunca resolve, para nao disparar navegacao no jsdom
      .mockReturnValueOnce(new Promise(() => {}));

    await expect(client.fetchWithAuth("/api/pedidos")).rejects.toThrow(
      "Sessão expirada. Faça login novamente."
    );
    expect(localStorage.getItem("auth_user")).toBeNull();
    expect(calledUrls()).toEqual([
      "http://api.test/api/pedidos",
      "http://api.test/api/auth/refresh",
      "http://api.test/api/auth/logout",
    ]);
  });

  it("401 com noRefresh nao chama refresh", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(401, { error: "credenciais invalidas" }));

    await expect(
      client.fetchWithAuth("/api/auth/login", { method: "POST", noRefresh: true })
    ).rejects.toMatchObject({ status: 401, message: "credenciais invalidas" });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});

describe("fetchWithAuth - sucesso", () => {
  it.each<[string, unknown, unknown]>([
    ["desenvelopa {data}", { success: true, data: [1, 2] }, [1, 2]],
    ["retorna objeto sem data", { a: 1 }, { a: 1 }],
    ["corpo vazio -> null", undefined, null],
  ])("%s", async (_n, body, esperado) => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, body));
    await expect(client.fetchWithAuth("/api/x")).resolves.toEqual(esperado);
  });

  it("envia credentials include e mescla headers customizados", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, {}));
    await client.fetchWithAuth("http://outro.host/api/y", {
      method: "PUT",
      headers: { "X-Teste": "1" },
    });
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://outro.host/api/y");
    expect(init).toMatchObject({
      method: "PUT",
      credentials: "include",
      headers: { "Content-Type": "application/json", "X-Teste": "1" },
    });
  });
});
