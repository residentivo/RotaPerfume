import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// 🔴 TestBrain (Lote 6): complementos de cobertura do apiClient.
// - FE-07: o timeout REAL do refresh (setTimeout de 10s + AbortController),
//   e nao so um fetch que ja rejeita com AbortError.
// - SEC-06: usuario inativado no servidor -> 401 "usuário inativo" na
//   requisicao, 401 no refresh e 401 no retry: a sessao cai (logout).
// - FE-06: requisicao da fila cujo retry responde erro HTTP recebe ApiError.
type ApiClientModule = typeof import("./apiClient");

let client: ApiClientModule;
let fetchMock: ReturnType<typeof vi.fn>;

const USER_JSON = JSON.stringify({ id: 1, nome: "Ana", role: "normal" });

function jsonResponse(status: number, body: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function calledUrls(): string[] {
  return fetchMock.mock.calls.map((c) => String(c[0]));
}

beforeEach(async () => {
  vi.resetModules();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  client = await import("./apiClient");
  localStorage.setItem("auth_user", USER_JSON);
  vi.spyOn(console, "warn").mockImplementation(() => {});
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  localStorage.clear();
});

describe("FE-07: timeout real do refresh (10s)", () => {
  it.each<[string, number, boolean]>([
    ["antes dos 10s o refresh segue pendente", 9_999, false],
    ["aos 10s o AbortController dispara e vira NetworkError de timeout", 10_000, true],
  ])("%s", async (_nome, avancoMs, deveAbortar) => {
    vi.useFakeTimers();
    let sinalRefresh: AbortSignal | undefined;
    fetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith("/api/auth/refresh")) {
        sinalRefresh = init?.signal ?? undefined;
        return new Promise<Response>((_res, rej) => {
          init?.signal?.addEventListener("abort", () =>
            rej(new DOMException("The operation was aborted.", "AbortError"))
          );
        });
      }
      return Promise.resolve(jsonResponse(401, { error: "token expirado" }));
    });

    let resultado: unknown = "pendente";
    const p = client.fetchWithAuth("/api/pedidos").then(
      (v) => (resultado = v),
      (e: unknown) => (resultado = e)
    );
    await vi.advanceTimersByTimeAsync(avancoMs);

    expect(sinalRefresh).toBeDefined();
    expect(sinalRefresh?.aborted).toBe(deveAbortar);
    if (!deveAbortar) {
      expect(resultado).toBe("pendente");
      await vi.advanceTimersByTimeAsync(1);
    }
    await p;

    expect(resultado).toBeInstanceOf(client.NetworkError);
    expect(resultado).toMatchObject({ kind: "timeout", message: client.MSG_ERRO_TIMEOUT });
    // Sem retry da original e sem logout (FE-07 opcao A).
    expect(calledUrls()).toEqual([
      "http://api.test/api/pedidos",
      "http://api.test/api/auth/refresh",
    ]);
    expect(localStorage.getItem("auth_user")).toBe(USER_JSON);
  });
});

describe("SEC-06: usuario inativado no servidor derruba a sessao", () => {
  it("401 'usuário inativo' -> refresh 401 -> retry 401: logout com 'Sessão expirada'", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { success: false, error: "usuário inativo" }))
      .mockResolvedValueOnce(jsonResponse(401, { success: false, error: "refresh token revogado" }))
      .mockResolvedValueOnce(jsonResponse(401, { success: false, error: "usuário inativo" }))
      .mockReturnValueOnce(new Promise(() => {})); // logout (redirect nao completa no jsdom)

    await expect(client.fetchWithAuth("/api/clientes")).rejects.toThrow("Sessão expirada");

    expect(calledUrls()).toEqual([
      "http://api.test/api/clientes",
      "http://api.test/api/auth/refresh",
      "http://api.test/api/clientes",
      "http://api.test/api/auth/logout",
    ]);
    expect(localStorage.getItem("auth_user")).toBeNull();
  });
});

describe("FE-06: fila de requisicoes com retry que responde erro HTTP", () => {
  it("a pendente da fila recebe ApiError do seu retry; a original segue OK", async () => {
    let releaseRefresh: (r: Response) => void = () => {};
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/api/auth/refresh")) {
        return new Promise<Response>((res) => {
          releaseRefresh = res;
        });
      }
      const n = fetchMock.mock.calls.filter((c) => c[0] === url).length;
      if (n === 1) return Promise.resolve(jsonResponse(401, { error: "expirado" }));
      return Promise.resolve(
        url.endsWith("/api/b")
          ? jsonResponse(500, { success: false, error: "falha no b" })
          : jsonResponse(200, { data: "a" })
      );
    });

    const pa = client.fetchWithAuth("/api/a");
    const pb = client.fetchWithAuth("/api/b").catch((e: unknown) => e);
    await vi.waitFor(() => {
      expect(calledUrls().filter((u) => u.endsWith("/api/auth/refresh"))).toHaveLength(1);
      expect(calledUrls().filter((u) => u.endsWith("/api/b"))).toHaveLength(1);
    });
    releaseRefresh(jsonResponse(200, {}));

    await expect(pa).resolves.toBe("a");
    const errB = await pb;
    expect(errB).toBeInstanceOf(client.ApiError);
    expect(errB).toMatchObject({ status: 500, message: "falha no b" });
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
  });
});
