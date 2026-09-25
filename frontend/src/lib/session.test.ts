import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { MeResponse } from "./types";

// Mock de GET /api/auth/me (unica dependencia de rede da sessao).
const apiMeMock = vi.fn<() => Promise<MeResponse>>();
vi.mock("./api", () => ({ apiMe: () => apiMeMock() }));

type SessionModule = typeof import("./session");
type VdModule = typeof import("./vendedorDesligado");
let session: SessionModule;
let vd: VdModule;

function me(extra: Partial<MeResponse> = {}): MeResponse {
  return {
    id: 1,
    nome: "Ana",
    email: "ana@x.com",
    role: "normal",
    ativo: true,
    id_vendedor: 7,
    vendedor_nome: "Vend 7",
    ...extra,
  };
}

beforeEach(async () => {
  vi.resetModules();
  apiMeMock.mockReset();
  vd = await import("./vendedorDesligado");
  session = await import("./session");
});

describe("refreshSessionUser", () => {
  it("atualiza a sessao em memoria e o cache sem vendedor_desligado", async () => {
    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: true }));
    const { result } = renderHook(() => session.useSessionUser());
    expect(result.current).toBeNull();

    await act(() => session.refreshSessionUser());

    expect(result.current).toMatchObject({ id_vendedor: 7, vendedor_desligado: true });
    expect(session.getSessionUser()?.id_vendedor).toBe(7);
    const cache = JSON.parse(localStorage.getItem("auth_user") ?? "{}");
    expect(cache.id_vendedor).toBe(7);
    expect(cache).not.toHaveProperty("vendedor_desligado");
  });

  it("chamadas concorrentes compartilham a mesma requisicao /me", async () => {
    let resolve: (u: MeResponse) => void = () => {};
    apiMeMock.mockReturnValueOnce(new Promise((r) => (resolve = r)));

    const a = session.refreshSessionUser();
    const b = session.refreshSessionUser();
    resolve(me());
    await Promise.all([a, b]);

    expect(apiMeMock).toHaveBeenCalledTimes(1);
    apiMeMock.mockResolvedValueOnce(me({ id_vendedor: 9 }));
    await session.refreshSessionUser();
    expect(apiMeMock).toHaveBeenCalledTimes(2);
    expect(session.getSessionUser()?.id_vendedor).toBe(9);
  });

  it("propaga erro de /me ao chamador", async () => {
    apiMeMock.mockRejectedValueOnce(new Error("401"));
    await expect(session.refreshSessionUser()).rejects.toThrow("401");
  });

  it("id_vendedor alterado pelo admin reflete sem novo login", async () => {
    apiMeMock.mockResolvedValueOnce(me({ id_vendedor: 3 }));
    const { result } = renderHook(() => session.useSessionUser());
    await act(() => session.refreshSessionUser());
    expect(result.current?.id_vendedor).toBe(3);

    apiMeMock.mockResolvedValueOnce(me({ id_vendedor: 8, vendedor_nome: "Vend 8" }));
    await act(() => session.refreshSessionUser());
    expect(result.current).toMatchObject({ id_vendedor: 8, vendedor_nome: "Vend 8" });
  });

  it("clearSession zera usuario e bloqueio", async () => {
    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: true }));
    await session.refreshSessionUser();
    act(() => session.clearSession());
    expect(session.getSessionUser()).toBeNull();
  });
});

describe("useVendedorDesligado", () => {
  it.each<[string, Partial<MeResponse>, boolean]>([
    ["normal com vendedor_desligado true", { vendedor_desligado: true }, true],
    ["normal com vendedor_desligado false", { vendedor_desligado: false }, false],
    ["normal sem o campo", {}, false],
    ["admin com vendedor_desligado true", { role: "admin", vendedor_desligado: true }, false],
  ])("%s -> %s", async (_n, extra, esperado) => {
    apiMeMock.mockResolvedValueOnce(me(extra));
    const { result } = renderHook(() => session.useVendedorDesligado());
    await act(() => session.refreshSessionUser());
    expect(result.current).toBe(esperado);
  });

  it("403 desligado marca o bloqueio e rebusca /me (origem403)", async () => {
    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: false }));
    const { result } = renderHook(() => session.useVendedorDesligado());
    await act(() => session.refreshSessionUser());
    expect(result.current).toBe(false);

    // /me ainda diz false (cache/latencia no backend), mas o 403 manda.
    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: false }));
    await act(async () => {
      vd.notifyVendedorDesligado();
    });
    expect(result.current).toBe(true);
    expect(apiMeMock).toHaveBeenCalledTimes(2);

    // A rebusca disparada pelo 403 nao limpa o bloqueio.
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(result.current).toBe(true);

    // Revalidacao normal (foco/montagem) com false explicito limpa.
    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: false }));
    await act(() => session.refreshSessionUser());
    expect(result.current).toBe(false);
  });

  it("falha ao rebuscar /me apos o 403 mantem o bloqueio", async () => {
    apiMeMock.mockRejectedValueOnce(new Error("rede"));
    const { result } = renderHook(() => session.useVendedorDesligado());
    await act(async () => {
      vd.notifyVendedorDesligado();
    });
    expect(result.current).toBe(true);
  });

  it("revalidacao normal sem o campo vendedor_desligado nao limpa o bloqueio 403", async () => {
    apiMeMock.mockResolvedValue(me());
    const { result } = renderHook(() => session.useVendedorDesligado());
    await act(async () => {
      vd.notifyVendedorDesligado();
      await new Promise((r) => setTimeout(r, 0));
    });
    await act(() => session.refreshSessionUser());
    expect(result.current).toBe(true);
  });

  it("FE-01: chamada normal nao reaproveita /me de origem 403 em voo; encadeia um novo", async () => {
    const { result } = renderHook(() => session.useVendedorDesligado());
    let resolve403: (u: MeResponse) => void = () => {};
    apiMeMock.mockReturnValueOnce(new Promise((r) => (resolve403 = r)));
    act(() => vd.notifyVendedorDesligado());
    expect(result.current).toBe(true);
    expect(apiMeMock).toHaveBeenCalledTimes(1);

    // Revalidacao normal enquanto o /me do 403 esta em voo.
    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: false }));
    const normal = session.refreshSessionUser();
    // Uma segunda chamada normal reaproveita a encadeada (nao gera 3a req.).
    const normal2 = session.refreshSessionUser();
    expect(apiMeMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolve403(me({ vendedor_desligado: false }));
      await Promise.all([normal, normal2]);
    });
    expect(apiMeMock).toHaveBeenCalledTimes(2);
    // O /me novo foi iniciado depois do 403 e trouxe false explicito.
    expect(result.current).toBe(false);
  });

  it("FE-01: /me normal iniciado ANTES do 403 nao limpa o bloqueio", async () => {
    const { result } = renderHook(() => session.useVendedorDesligado());
    let resolveNormal: (u: MeResponse) => void = () => {};
    apiMeMock.mockReturnValueOnce(new Promise((r) => (resolveNormal = r)));
    const antigo = session.refreshSessionUser();

    // 403 chega com o /me normal ainda em voo (reaproveitado pelo 403).
    act(() => vd.notifyVendedorDesligado());
    expect(result.current).toBe(true);
    expect(apiMeMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveNormal(me({ vendedor_desligado: false }));
      await antigo;
    });
    expect(result.current).toBe(true);
  });

  it("FE-01: chamada normal com /me antigo (pre-403) em voo encadeia e limpa so com o novo", async () => {
    const { result } = renderHook(() => session.useVendedorDesligado());
    let resolveNormal: (u: MeResponse) => void = () => {};
    apiMeMock.mockReturnValueOnce(new Promise((r) => (resolveNormal = r)));
    const antigo = session.refreshSessionUser();
    act(() => vd.notifyVendedorDesligado());

    let resolveNovo: (u: MeResponse) => void = () => {};
    apiMeMock.mockReturnValueOnce(new Promise((r) => (resolveNovo = r)));
    const novo = session.refreshSessionUser();
    await act(async () => {
      resolveNormal(me({ vendedor_desligado: false }));
      await antigo;
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(result.current).toBe(true);
    await act(async () => {
      resolveNovo(me({ vendedor_desligado: false }));
      await novo;
    });
    expect(apiMeMock).toHaveBeenCalledTimes(2);
    expect(result.current).toBe(false);
  });

  it("FE-01: erro de rede no /me encadeado nao limpa o bloqueio", async () => {
    const { result } = renderHook(() => session.useVendedorDesligado());
    let reject403: (e: Error) => void = () => {};
    apiMeMock.mockReturnValueOnce(new Promise((_r, rej) => (reject403 = rej)));
    act(() => vd.notifyVendedorDesligado());
    apiMeMock.mockRejectedValueOnce(new Error("rede"));
    const normal = session.refreshSessionUser();
    await act(async () => {
      reject403(new Error("rede"));
      await expect(normal).rejects.toThrow("rede");
    });
    expect(apiMeMock).toHaveBeenCalledTimes(2);
    expect(result.current).toBe(true);
  });

  it("FE-01: 2o 403 com um /me normal (pos-1o 403) em voo: esse /me nao limpa e a proxima chamada normal encadeia", async () => {
    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: false }));
    const { result } = renderHook(() => session.useVendedorDesligado());
    // 1o 403 e sua rebusca (origem403) terminam.
    await act(async () => {
      vd.notifyVendedorDesligado();
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(result.current).toBe(true);

    // /me normal iniciado DEPOIS do 1o 403...
    let resolveNormal: (u: MeResponse) => void = () => {};
    apiMeMock.mockReturnValueOnce(new Promise((r) => (resolveNormal = r)));
    const normal = session.refreshSessionUser();
    expect(apiMeMock).toHaveBeenCalledTimes(2);

    // ...mas ANTES do 2o 403 (reaproveitado pelo 403, sem nova requisicao).
    act(() => vd.notifyVendedorDesligado());
    expect(apiMeMock).toHaveBeenCalledTimes(2);

    // Chamada normal agora nao pode reaproveitar o /me antigo: encadeia.
    let resolveNovo: (u: MeResponse) => void = () => {};
    apiMeMock.mockReturnValueOnce(new Promise((r) => (resolveNovo = r)));
    const novo = session.refreshSessionUser();

    await act(async () => {
      resolveNormal(me({ vendedor_desligado: false }));
      await normal;
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(result.current).toBe(true);
    expect(apiMeMock).toHaveBeenCalledTimes(3);

    await act(async () => {
      resolveNovo(me({ vendedor_desligado: false }));
      await novo;
    });
    expect(result.current).toBe(false);
  });

  it("FE-01: 403 enquanto a chamada encadeada ainda nao comecou: ela comeca depois e pode limpar", async () => {
    const { result } = renderHook(() => session.useVendedorDesligado());
    let resolve403: (u: MeResponse) => void = () => {};
    apiMeMock.mockReturnValueOnce(new Promise((r) => (resolve403 = r)));
    act(() => vd.notifyVendedorDesligado());

    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: false }));
    const normal = session.refreshSessionUser(); // encadeada (seq ainda null)
    act(() => vd.notifyVendedorDesligado()); // 2o 403 reaproveita a encadeada
    expect(apiMeMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolve403(me({ vendedor_desligado: false }));
      await normal;
    });
    // A encadeada foi iniciada depois dos dois 403: false explicito limpa.
    expect(apiMeMock).toHaveBeenCalledTimes(2);
    expect(result.current).toBe(false);
  });

  it("FE-01: /me normal pos-403 com vendedor_desligado true mantem o bloqueio; o seguinte com false libera", async () => {
    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: true }));
    const { result } = renderHook(() => session.useVendedorDesligado());
    await act(async () => {
      vd.notifyVendedorDesligado();
      await new Promise((r) => setTimeout(r, 0));
    });
    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: true }));
    await act(() => session.refreshSessionUser());
    expect(result.current).toBe(true);

    apiMeMock.mockResolvedValueOnce(me({ vendedor_desligado: false }));
    await act(() => session.refreshSessionUser());
    expect(result.current).toBe(false);
  });

  it("FE-01: chamada de origem 403 sem /me em voo inicia uma requisicao propria que nao limpa", async () => {
    apiMeMock.mockResolvedValue(me({ vendedor_desligado: false }));
    const { result } = renderHook(() => session.useVendedorDesligado());
    await act(async () => {
      vd.notifyVendedorDesligado();
    });
    await act(async () => {
      await session.refreshSessionUser({ origem403: true });
    });
    expect(apiMeMock).toHaveBeenCalledTimes(2);
    expect(result.current).toBe(true);
  });

  it("FE-01: clearSession (logout) limpa o bloqueio 403", async () => {
    apiMeMock.mockResolvedValue(me());
    const { result } = renderHook(() => session.useVendedorDesligado());
    await act(async () => {
      vd.notifyVendedorDesligado();
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(result.current).toBe(true);
    act(() => session.clearSession());
    expect(result.current).toBe(false);
  });

  it("admin nunca e bloqueado, mesmo apos 403", async () => {
    apiMeMock.mockResolvedValue(me({ role: "admin" }));
    const { result } = renderHook(() => session.useVendedorDesligado());
    await act(() => session.refreshSessionUser());
    await act(async () => {
      vd.notifyVendedorDesligado();
    });
    expect(result.current).toBe(false);
  });
});
