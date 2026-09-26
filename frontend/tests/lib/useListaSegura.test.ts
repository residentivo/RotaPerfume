import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useDebounceFiltros, useExcluidos, useUltimaResposta } from "@/lib/useListaSegura";

function deferred<T>() {
  let resolve: (v: T) => void = () => {};
  let reject: (e: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("useUltimaResposta (FE-04)", () => {
  it("so a requisicao mais recente aplica sucesso ou erro", async () => {
    const { result } = renderHook(() => useUltimaResposta());
    const a = deferred<string>();
    const b = deferred<string>();
    const ok = vi.fn();
    const erro = vi.fn();

    const pa = result.current(a.promise, ok, erro);
    const pb = result.current(b.promise, ok, erro);
    b.resolve("nova");
    await pb;
    a.reject(new Error("velha"));
    await pa;

    expect(ok).toHaveBeenCalledTimes(1);
    expect(ok).toHaveBeenCalledWith("nova");
    expect(erro).not.toHaveBeenCalled();
  });

  it("aplica o erro da mais recente", async () => {
    const { result } = renderHook(() => useUltimaResposta());
    const erro = vi.fn();
    await result.current(Promise.reject(new Error("x")), vi.fn(), erro);
    expect(erro).toHaveBeenCalledWith(new Error("x"));
  });

  it("descarta o que estava em voo ao desmontar", async () => {
    const { result, unmount } = renderHook(() => useUltimaResposta());
    const a = deferred<number>();
    const ok = vi.fn();
    const p = result.current(a.promise, ok, vi.fn());
    unmount();
    a.resolve(1);
    await p;
    expect(ok).not.toHaveBeenCalled();
  });

  it("a funcao devolvida e estavel entre renders", () => {
    const { result, rerender } = renderHook(() => useUltimaResposta());
    const primeira = result.current;
    rerender();
    expect(result.current).toBe(primeira);
  });
});

describe("useDebounceFiltros (FE-04)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("nao dispara na montagem", () => {
    const cb = vi.fn();
    renderHook(() => useDebounceFiltros("a", cb));
    act(() => vi.advanceTimersByTime(1000));
    expect(cb).not.toHaveBeenCalled();
  });

  it("dispara uma vez 350ms depois da ultima mudanca", () => {
    const cb = vi.fn();
    const { rerender } = renderHook(({ k }) => useDebounceFiltros(k, cb), {
      initialProps: { k: "a" },
    });
    rerender({ k: "ab" });
    act(() => vi.advanceTimersByTime(200));
    rerender({ k: "abc" });
    act(() => vi.advanceTimersByTime(349));
    expect(cb).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(1));
    expect(cb).toHaveBeenCalledTimes(1);
  });

  it("voltar ao valor ja aplicado antes do timer vencer nao dispara", () => {
    const cb = vi.fn();
    const { rerender } = renderHook(({ k }) => useDebounceFiltros(k, cb), {
      initialProps: { k: "a" },
    });
    rerender({ k: "b" });
    act(() => vi.advanceTimersByTime(100));
    rerender({ k: "a" });
    act(() => vi.advanceTimersByTime(1000));
    expect(cb).not.toHaveBeenCalled();
  });

  it("usa o callback do render mais recente", () => {
    const primeiro = vi.fn();
    const segundo = vi.fn();
    const { rerender } = renderHook(({ k, cb }) => useDebounceFiltros(k, cb), {
      initialProps: { k: "a", cb: primeiro },
    });
    rerender({ k: "b", cb: primeiro });
    rerender({ k: "b", cb: segundo });
    act(() => vi.advanceTimersByTime(350));
    expect(primeiro).not.toHaveBeenCalled();
    expect(segundo).toHaveBeenCalledTimes(1);
  });
});

describe("useExcluidos (FE-04)", () => {
  const pagina = (ids: number[], total = ids.length) => ({
    data: ids.map((id) => ({ id })),
    total,
    pages: 1,
  });

  it("sem exclusoes devolve a mesma resposta", () => {
    const { result } = renderHook(() => useExcluidos((x: { id: number }) => x.id));
    const res = pagina([1, 2]);
    expect(result.current.filtrar(res)).toBe(res);
  });

  it("remove os ids marcados e desconta o total", () => {
    const { result } = renderHook(() => useExcluidos((x: { id: number }) => x.id));
    result.current.marcar(2);
    expect(result.current.filtrar(pagina([1, 2, 3], 30))).toEqual({
      data: [{ id: 1 }, { id: 3 }],
      total: 29,
      pages: 1,
    });
  });

  it("o registro sobrevive a novos renders", () => {
    const { result, rerender } = renderHook(() => useExcluidos((x: { id: number }) => x.id));
    result.current.marcar(1);
    rerender();
    expect(result.current.filtrar(pagina([1, 2])).data).toEqual([{ id: 2 }]);
  });
});
