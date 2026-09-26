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
      // SEC-02: retry unico da original tambem com 401
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      // logout: nunca resolve, para nao disparar navegacao no jsdom
      .mockReturnValueOnce(new Promise(() => {}));

    await expect(client.fetchWithAuth("/api/pedidos")).rejects.toThrow(
      "Sessão expirada. Faça login novamente."
    );
    expect(localStorage.getItem("auth_user")).toBeNull();
    expect(calledUrls()).toEqual([
      "http://api.test/api/pedidos",
      "http://api.test/api/auth/refresh",
      "http://api.test/api/pedidos",
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

// SEC-02: os cookies sao compartilhados entre abas. Se outra aba renovou
// primeiro, o refresh desta aba volta 401 (token antigo revogado), mas os
// cookies novos ja valem: a requisicao original e repetida uma unica vez.
describe("fetchWithAuth - refresh multi-aba (SEC-02)", () => {
  it("refresh 401 -> retry OK: devolve os dados e NAO desloga", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(401, { error: "refresh token revogado" }))
      .mockResolvedValueOnce(jsonResponse(200, { success: true, data: { ok: 1 } }));

    await expect(client.fetchWithAuth("/api/pedidos")).resolves.toEqual({ ok: 1 });

    expect(calledUrls()).toEqual([
      "http://api.test/api/pedidos",
      "http://api.test/api/auth/refresh",
      "http://api.test/api/pedidos",
    ]);
    expect(localStorage.getItem("auth_user")).toBe(USER_JSON);
  });

  it("refresh 401 -> retry com erro nao-401: rejeita com o ApiError, sem logout", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(401, {}))
      .mockResolvedValueOnce(jsonResponse(500, { error: "falhou" }));

    await expect(client.fetchWithAuth("/api/pedidos")).rejects.toMatchObject({
      status: 500,
      message: "falhou",
    });
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
    expect(localStorage.getItem("auth_user")).toBe(USER_JSON);
  });

  it("refresh 401 -> retry 401: desloga (um unico retry)", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(401, {}))
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockReturnValueOnce(new Promise(() => {}));

    await expect(client.fetchWithAuth("/api/pedidos")).rejects.toThrow("Sessão expirada");
    expect(calledUrls().filter((u) => u.endsWith("/api/pedidos"))).toHaveLength(2);
    expect(calledUrls().filter((u) => u.endsWith("/api/auth/refresh"))).toHaveLength(1);
    expect(calledUrls()).toContain("http://api.test/api/auth/logout");
    expect(localStorage.getItem("auth_user")).toBeNull();
  });

  it("refresh 429: nao repete a requisicao e segue o fluxo de logout", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(429, { error: "muitas tentativas" }))
      .mockReturnValueOnce(new Promise(() => {}));

    await expect(client.fetchWithAuth("/api/pedidos")).rejects.toThrow("Sessão expirada");
    expect(calledUrls()).toEqual([
      "http://api.test/api/pedidos",
      "http://api.test/api/auth/refresh",
      "http://api.test/api/auth/logout",
    ]);
  });

  it("refresh com timeout: nao repete a requisicao", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockRejectedValueOnce(Object.assign(new Error("aborted"), { name: "AbortError" }))
      .mockReturnValueOnce(new Promise(() => {}));

    await expect(client.fetchWithAuth("/api/pedidos")).rejects.toThrow("Sessão expirada");
    expect(calledUrls()).toEqual([
      "http://api.test/api/pedidos",
      "http://api.test/api/auth/refresh",
      "http://api.test/api/auth/logout",
    ]);
  });

  it("refresh 401 -> retry OK libera as requisicoes concorrentes da fila", async () => {
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
    releaseRefresh(jsonResponse(401, { error: "refresh token revogado" }));

    await expect(pa).resolves.toBe("a");
    await expect(pb).resolves.toBe("b");
    expect(calledUrls().filter((u) => u.endsWith("/api/auth/refresh"))).toHaveLength(1);
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
  });

  it("com Web Locks, o refresh roda dentro do lock rp-auth-refresh", async () => {
    const request = vi.fn((_nome: string, cb: () => Promise<unknown>) => cb());
    vi.stubGlobal("navigator", { ...navigator, locks: { request } });
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(200, {}))
      .mockResolvedValueOnce(jsonResponse(200, { data: 1 }));

    await expect(client.fetchWithAuth("/api/x")).resolves.toBe(1);
    expect(request).toHaveBeenCalledTimes(1);
    expect(request.mock.calls[0][0]).toBe("rp-auth-refresh");
  });

  // 🔴 TestBrain (Lote 4): caminhos de borda do retry unico.
  it("refresh 401 -> retry com falha de rede: rejeita com o erro de rede, sem logout, e libera a fila", async () => {
    let releaseRefresh: (r: Response) => void = () => {};
    let chamadasA = 0;
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/api/auth/refresh")) {
        return new Promise<Response>((res) => {
          releaseRefresh = res;
        });
      }
      if (url.endsWith("/api/a")) {
        chamadasA++;
        return chamadasA === 1
          ? Promise.resolve(jsonResponse(401, { error: "expirado" }))
          : Promise.reject(new TypeError("Failed to fetch"));
      }
      return Promise.resolve(jsonResponse(401, { error: "expirado" }));
    });

    const pa = client.fetchWithAuth("/api/a");
    const pb = client.fetchWithAuth("/api/b");
    await vi.waitFor(() => {
      expect(calledUrls().filter((u) => u.endsWith("/api/auth/refresh"))).toHaveLength(1);
      expect(calledUrls().filter((u) => u.endsWith("/api/b"))).toHaveLength(1);
    });
    releaseRefresh(jsonResponse(401, { error: "refresh token revogado" }));

    const errA = await pa.catch((e: unknown) => e);
    const errB = await pb.catch((e: unknown) => e);
    // FE-08: o erro de rede vira NetworkError amigavel, com o original em cause.
    expect(errA).toBeInstanceOf(client.NetworkError);
    expect(errA).toMatchObject({ name: "NetworkError", kind: "conexao", message: client.MSG_ERRO_REDE });
    expect((errA as Error).cause).toBeInstanceOf(TypeError);
    expect(((errA as Error).cause as Error).message).toBe("Failed to fetch");
    // FE-06: a requisicao pendente na fila e rejeitada (nao fica pendurada)
    // com o MESMO erro de rede, e nao com "Sessão expirada" (nao houve logout).
    expect(errB).toBe(errA);
    expect((errB as Error).message).not.toMatch(/Sessão expirada/);
    // A fila nao reenvia /api/b: so a primeira tentativa.
    expect(calledUrls().filter((u) => u.endsWith("/api/b"))).toHaveLength(1);
    expect(calledUrls().filter((u) => u.endsWith("/api/a"))).toHaveLength(2);
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
    expect(localStorage.getItem("auth_user")).toBe(USER_JSON);
  });

  it("FE-06/FE-08: retry com falha de rede nao-Error: original e fila recebem o mesmo NetworkError, sem logout", async () => {
    let releaseRefresh: (r: Response) => void = () => {};
    let chamadasA = 0;
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/api/auth/refresh")) {
        return new Promise<Response>((res) => {
          releaseRefresh = res;
        });
      }
      if (url.endsWith("/api/a")) {
        chamadasA++;
        return chamadasA === 1
          ? Promise.resolve(jsonResponse(401, { error: "expirado" }))
          : Promise.reject("offline");
      }
      return Promise.resolve(jsonResponse(401, { error: "expirado" }));
    });

    const pa = client.fetchWithAuth("/api/a");
    const pb = client.fetchWithAuth("/api/b");
    await vi.waitFor(() => {
      expect(calledUrls().filter((u) => u.endsWith("/api/auth/refresh"))).toHaveLength(1);
      expect(calledUrls().filter((u) => u.endsWith("/api/b"))).toHaveLength(1);
    });
    releaseRefresh(jsonResponse(401, {}));

    const errA = await pa.catch((e: unknown) => e);
    const errB = await pb.catch((e: unknown) => e);
    expect(errA).toBeInstanceOf(client.NetworkError);
    expect((errA as Error).message).toBe(client.MSG_ERRO_REDE);
    expect((errA as Error).cause).toBe("offline");
    expect(errB).toBe(errA);
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
    expect(localStorage.getItem("auth_user")).toBe(USER_JSON);
  });

  it("FE-06 (regressao): refresh 401 -> retry 401 desloga e a fila recebe 'Sessão expirada'", async () => {
    let releaseRefresh: (r: Response) => void = () => {};
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/api/auth/refresh")) {
        return new Promise<Response>((res) => {
          releaseRefresh = res;
        });
      }
      // logout: nunca resolve, para nao disparar navegacao no jsdom
      if (url.endsWith("/api/auth/logout")) return new Promise<Response>(() => {});
      return Promise.resolve(jsonResponse(401, { error: "expirado" }));
    });

    const pa = client.fetchWithAuth("/api/a");
    const pb = client.fetchWithAuth("/api/b");
    await vi.waitFor(() => {
      expect(calledUrls().filter((u) => u.endsWith("/api/auth/refresh"))).toHaveLength(1);
      expect(calledUrls().filter((u) => u.endsWith("/api/b"))).toHaveLength(1);
    });
    releaseRefresh(jsonResponse(401, { error: "refresh token revogado" }));

    await expect(pa).rejects.toThrow("Sessão expirada. Faça login novamente.");
    await expect(pb).rejects.toThrow("Sessão expirada. Faça login novamente.");
    expect(calledUrls().filter((u) => u.endsWith("/api/a"))).toHaveLength(2);
    expect(calledUrls().filter((u) => u.endsWith("/api/b"))).toHaveLength(1);
    expect(calledUrls()).toContain("http://api.test/api/auth/logout");
    expect(localStorage.getItem("auth_user")).toBeNull();
  });

  it.each<[string, () => Promise<Response>]>([
    ["erro de rede no refresh", () => Promise.reject(new TypeError("Failed to fetch"))],
    ["refresh 500", () => Promise.resolve(jsonResponse(500, { error: "erro interno" }))],
  ])("%s: nao repete a requisicao original e desloga", async (_n, refreshResp) => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockImplementationOnce(refreshResp)
      .mockReturnValueOnce(new Promise(() => {}));

    await expect(client.fetchWithAuth("/api/pedidos")).rejects.toThrow("Sessão expirada");
    expect(calledUrls()).toEqual([
      "http://api.test/api/pedidos",
      "http://api.test/api/auth/refresh",
      "http://api.test/api/auth/logout",
    ]);
  });

  it("Web Locks rejeitando: trata como erro (sem retry) e desloga", async () => {
    const request = vi.fn(() => Promise.reject(new Error("lock abortado")));
    vi.stubGlobal("navigator", { ...navigator, locks: { request } });
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockReturnValueOnce(new Promise(() => {}));

    await expect(client.fetchWithAuth("/api/x")).rejects.toThrow("Sessão expirada");
    expect(request).toHaveBeenCalledTimes(1);
    expect(calledUrls()).toEqual(["http://api.test/api/x", "http://api.test/api/auth/logout"]);
  });

  it("refresh 401 -> retry OK com keepEnvelope devolve o envelope inteiro", async () => {
    const envelope = { data: [1], pagination: { page: 1, total: 1 } };
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(401, {}))
      .mockResolvedValueOnce(jsonResponse(200, envelope));

    await expect(client.fetchEnvelopeWithAuth("/api/pedidos")).resolves.toEqual(envelope);
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

// 🔴 TestBrain (Lote 5, FE-06): a fila so recebe "Sessão expirada" quando ha
// logout; falha de rede no retry nao desloga e nao trava o estado do modulo.
describe("fetchWithAuth - fila de refresh (FE-06, regressao do Lote 5)", () => {
  type RespostaRefresh = () => Promise<Response>;

  // Sobe N requisicoes com 401; a primeira dispara o refresh (segurado) e as
  // outras entram na fila. Devolve as promessas e a funcao que libera o refresh.
  async function filaComRefreshSegurado(
    n: number,
    retryPrimeira: () => Promise<Response>
  ) {
    let liberar: (r: RespostaRefresh) => void = () => {};
    let chamadasPrimeira = 0;
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/api/auth/refresh")) {
        return new Promise<Response>((res, rej) => {
          liberar = (r) => r().then(res, rej);
        });
      }
      if (url.endsWith("/api/auth/logout")) return new Promise<Response>(() => {});
      if (url.endsWith("/api/r0")) {
        chamadasPrimeira++;
        return chamadasPrimeira === 1
          ? Promise.resolve(jsonResponse(401, { error: "expirado" }))
          : retryPrimeira();
      }
      return Promise.resolve(jsonResponse(401, { error: "expirado" }));
    });
    const ps = Array.from({ length: n }, (_, i) =>
      client.fetchWithAuth(`/api/r${i}`).catch((e: unknown) => e)
    );
    await vi.waitFor(() => {
      expect(calledUrls().filter((u) => u.endsWith("/api/auth/refresh"))).toHaveLength(1);
      expect(calledUrls().filter((u) => /\/api\/r\d$/.test(u))).toHaveLength(n);
    });
    return { ps, liberar: (r: RespostaRefresh) => liberar(r) };
  }

  it("falha de rede no retry: TODAS as pendentes (3) recebem o mesmo NetworkError, sem logout", async () => {
    const erroRede = new TypeError("Failed to fetch");
    const { ps, liberar } = await filaComRefreshSegurado(4, () => Promise.reject(erroRede));
    liberar(() => Promise.resolve(jsonResponse(401, { error: "refresh token revogado" })));

    const erros = await Promise.all(ps);
    // FE-08: mensagem amigavel, erro original do navegador preservado em cause.
    expect(erros[0]).toBeInstanceOf(client.NetworkError);
    expect((erros[0] as Error).message).toBe(client.MSG_ERRO_REDE);
    expect((erros[0] as Error).cause).toBe(erroRede);
    for (const e of erros) expect(e).toBe(erros[0]);
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
    expect(localStorage.getItem("auth_user")).toBe(USER_JSON);
  });

  it.each<[string, RespostaRefresh]>([
    ["refresh 401 e retry 401", () => Promise.resolve(jsonResponse(401, { error: "refresh token revogado" }))],
    ["refresh 429 (rate limit)", () => Promise.resolve(jsonResponse(429, { error: "muitas tentativas" }))],
    ["refresh 500", () => Promise.resolve(jsonResponse(500, { error: "erro interno" }))],
    ["refresh com timeout", () => Promise.reject(Object.assign(new Error("aborted"), { name: "AbortError" }))],
    ["refresh com erro de rede", () => Promise.reject(new TypeError("Failed to fetch"))],
  ])("%s: ha logout e toda a fila recebe 'Sessão expirada'", async (_n, respostaRefresh) => {
    const { ps, liberar } = await filaComRefreshSegurado(3, () =>
      Promise.resolve(jsonResponse(401, { error: "expirado" }))
    );
    liberar(respostaRefresh);

    const erros = await Promise.all(ps);
    for (const e of erros) {
      expect(e).toBeInstanceOf(Error);
      expect((e as Error).message).toBe("Sessão expirada. Faça login novamente.");
    }
    expect(calledUrls()).toContain("http://api.test/api/auth/logout");
    expect(localStorage.getItem("auth_user")).toBeNull();
  });

  it("depois da falha de rede no retry, o estado e liberado: um novo 401 dispara um novo refresh", async () => {
    const erroRede = new TypeError("Failed to fetch");
    const { ps, liberar } = await filaComRefreshSegurado(2, () => Promise.reject(erroRede));
    liberar(() => Promise.resolve(jsonResponse(401, {})));
    await Promise.all(ps);

    // Rede voltou: novo 401 -> novo refresh OK -> retry OK.
    fetchMock.mockReset();
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "expirado" }))
      .mockResolvedValueOnce(jsonResponse(200, {}))
      .mockResolvedValueOnce(jsonResponse(200, { data: "ok" }));
    await expect(client.fetchWithAuth("/api/depois")).resolves.toBe("ok");
    expect(calledUrls()).toEqual([
      "http://api.test/api/depois",
      "http://api.test/api/auth/refresh",
      "http://api.test/api/depois",
    ]);
    expect(document.getElementById("api-refresh-indicator")?.style.display ?? "none").toBe("none");
  });
});

// 🟢 FrontBrain (Lote 5, FE-08): falha de rede/timeout vira NetworkError com
// mensagem amigavel; erro original preservado em `cause`.
describe("fetchWithAuth - erro de rede (FE-08)", () => {
  it("mensagens em portugues com acentos", () => {
    expect(client.MSG_ERRO_REDE).toBe(
      "Não foi possível conectar ao servidor. Verifique sua conexão com a internet e tente novamente."
    );
    expect(client.MSG_ERRO_TIMEOUT).toBe("O servidor demorou para responder. Tente novamente.");
  });

  it.each([
    ["Chrome", new TypeError("Failed to fetch")],
    ["Firefox", new TypeError("NetworkError when attempting to fetch resource.")],
    ["Safari", new TypeError("Load failed")],
  ])("falha de rede na requisicao original (%s): NetworkError, sem refresh nem logout", async (_n, original) => {
    fetchMock.mockRejectedValueOnce(original);

    const err = await client.fetchWithAuth("/api/clientes").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(client.NetworkError);
    expect(err).toBeInstanceOf(Error);
    expect(err).toMatchObject({ name: "NetworkError", kind: "conexao", message: client.MSG_ERRO_REDE });
    expect((err as Error).cause).toBe(original);
    expect(calledUrls()).toEqual(["http://api.test/api/clientes"]);
    expect(localStorage.getItem("auth_user")).toBe(USER_JSON);
  });

  it("timeout (TimeoutError, ex.: AbortSignal.timeout): NetworkError com mensagem de timeout", async () => {
    const original = new DOMException("The operation timed out.", "TimeoutError");
    fetchMock.mockRejectedValueOnce(original);

    const err = await client
      .fetchWithAuth("/api/x", { signal: AbortSignal.timeout(60000) })
      .catch((e: unknown) => e);

    expect(err).toBeInstanceOf(client.NetworkError);
    expect(err).toMatchObject({ kind: "timeout", message: client.MSG_ERRO_TIMEOUT });
    expect((err as Error).cause).toBe(original);
  });

  it("timeout real do signal do chamador vira NetworkError de timeout", async () => {
    fetchMock.mockImplementationOnce(
      (_url: string, init: RequestInit) =>
        new Promise<Response>((_res, rej) => {
          init.signal?.addEventListener("abort", () => rej(init.signal?.reason));
        })
    );

    const err = await client
      .fetchWithAuth("/api/x", { signal: AbortSignal.timeout(5) })
      .catch((e: unknown) => e);

    expect(err).toBeInstanceOf(client.NetworkError);
    expect((err as Error).message).toBe(client.MSG_ERRO_TIMEOUT);
  });

  it("abort intencional do chamador (AbortError) NAO vira erro de rede", async () => {
    const controller = new AbortController();
    const abortErr = new DOMException("The user aborted a request.", "AbortError");
    fetchMock.mockRejectedValueOnce(abortErr);
    controller.abort();

    const err = await client
      .fetchWithAuth("/api/x", { signal: controller.signal })
      .catch((e: unknown) => e);

    expect(err).toBe(abortErr);
    expect(err).not.toBeInstanceOf(client.NetworkError);
  });

  it("abort do chamador com motivo proprio repassa o motivo sem conversao", async () => {
    const controller = new AbortController();
    controller.abort("cancelado pela tela");
    fetchMock.mockRejectedValueOnce("cancelado pela tela");

    const err = await client
      .fetchWithAuth("/api/x", { signal: controller.signal })
      .catch((e: unknown) => e);

    expect(err).toBe("cancelado pela tela");
  });

  it("401 -> refresh OK -> retry com falha de rede: NetworkError, sem logout", async () => {
    const original = new TypeError("Failed to fetch");
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, { error: "token expirado" }))
      .mockResolvedValueOnce(jsonResponse(200, {}))
      .mockRejectedValueOnce(original);

    const err = await client.fetchWithAuth("/api/pedidos").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(client.NetworkError);
    expect((err as Error).message).toBe(client.MSG_ERRO_REDE);
    expect((err as Error).cause).toBe(original);
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
    expect(localStorage.getItem("auth_user")).toBe(USER_JSON);
  });

  it("refresh OK -> reenvio de requisicao da fila com falha de rede: NetworkError", async () => {
    let releaseRefresh: (r: Response) => void = () => {};
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/api/auth/refresh")) {
        return new Promise<Response>((res) => {
          releaseRefresh = res;
        });
      }
      const n = fetchMock.mock.calls.filter((c) => c[0] === url).length;
      if (n === 1) return Promise.resolve(jsonResponse(401, { error: "expirado" }));
      return url.endsWith("/api/b")
        ? Promise.reject(new TypeError("Load failed"))
        : Promise.resolve(jsonResponse(200, { data: "a" }));
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
    expect(errB).toBeInstanceOf(client.NetworkError);
    expect((errB as Error).message).toBe(client.MSG_ERRO_REDE);
    expect(((errB as Error).cause as Error).message).toBe("Load failed");
    expect(calledUrls().some((u) => u.includes("/api/auth/logout"))).toBe(false);
  });

  it("erro HTTP da API continua com a mensagem do backend (nao e NetworkError)", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(409, { error: "CNPJ já cadastrado" }));

    const err = await client.fetchWithAuth("/api/clientes").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(client.ApiError);
    expect(err).not.toBeInstanceOf(client.NetworkError);
    expect(err).toMatchObject({ status: 409, message: "CNPJ já cadastrado" });
  });
});
