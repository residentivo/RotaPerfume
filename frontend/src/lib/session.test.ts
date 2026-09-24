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
