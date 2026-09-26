import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  faixaExibida,
  mensagemRecargaFalhou,
  useDebounceFiltros,
  useExcluidos,
  usePaginaCarregada,
  useUltimaResposta,
} from "@/lib/useListaSegura";

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

// ─── FE-10 ────────────────────────────────────────────────────────────────────

describe("usePaginaCarregada (FE-10)", () => {
  type Props = { page: number; limit: number; erro: boolean };
  const montar = (inicial: Props) =>
    renderHook(({ page, limit, erro }: Props) => usePaginaCarregada(page, limit, erro), {
      initialProps: inicial,
    });

  it("sem erro, exibe a pagina/limite pedidos (inclusive antes de registrar)", () => {
    const { result, rerender } = montar({ page: 1, limit: 20, erro: false });
    expect(result.current.exibida).toEqual({ pagina: 1, limite: 20 });
    rerender({ page: 3, limit: 50, erro: false });
    expect(result.current.exibida).toEqual({ pagina: 3, limite: 50 });
  });

  it("com erro, exibe a ultima pagina registrada (a das linhas na tela)", () => {
    const { result, rerender } = montar({ page: 1, limit: 20, erro: false });
    act(() => result.current.registrar(1, 20));
    rerender({ page: 2, limit: 20, erro: true });
    expect(result.current.exibida).toEqual({ pagina: 1, limite: 20 });
  });

  it("com erro na primeira carga, exibe os valores iniciais", () => {
    const { result } = montar({ page: 1, limit: 20, erro: true });
    expect(result.current.exibida).toEqual({ pagina: 1, limite: 20 });
  });

  it("registrar atualiza a carregada; o erro seguinte volta para ela", () => {
    const { result, rerender } = montar({ page: 1, limit: 20, erro: false });
    rerender({ page: 2, limit: 50, erro: false });
    act(() => result.current.registrar(2, 50));
    rerender({ page: 3, limit: 50, erro: true });
    expect(result.current.exibida).toEqual({ pagina: 2, limite: 50 });
    // Sucesso depois do erro: volta a exibir a pedida.
    rerender({ page: 3, limit: 50, erro: false });
    expect(result.current.exibida).toEqual({ pagina: 3, limite: 50 });
  });

  it("registrar os mesmos valores nao provoca novo render", () => {
    let renders = 0;
    const { result } = renderHook(() => {
      renders++;
      return usePaginaCarregada(1, 20, false);
    });
    const antes = renders;
    act(() => result.current.registrar(1, 20));
    expect(renders).toBe(antes);
    act(() => result.current.registrar(2, 20));
    expect(renders).toBe(antes + 1);
  });

  it("registrar tem identidade estavel", () => {
    const { result, rerender } = montar({ page: 1, limit: 20, erro: false });
    const registrar = result.current.registrar;
    rerender({ page: 2, limit: 20, erro: true });
    expect(result.current.registrar).toBe(registrar);
  });
});

describe("faixaExibida (FE-10)", () => {
  it.each<[string, number, number, number, { inicio: number; fim: number }]>([
    ["total zero", 1, 20, 0, { inicio: 0, fim: 0 }],
    ["total zero em pagina alta", 3, 20, 0, { inicio: 0, fim: 0 }],
    ["pagina 1 cheia", 1, 20, 60, { inicio: 1, fim: 20 }],
    ["pagina do meio", 2, 20, 60, { inicio: 21, fim: 40 }],
    ["ultima pagina parcial", 3, 20, 45, { inicio: 41, fim: 45 }],
    ["menos itens que o limite", 1, 50, 7, { inicio: 1, fim: 7 }],
    ["um unico item", 1, 10, 1, { inicio: 1, fim: 1 }],
  ])("%s", (_n, pagina, limite, total, esperado) => {
    expect(faixaExibida(pagina, limite, total)).toEqual(esperado);
  });
});

describe("mensagemRecargaFalhou (FE-10)", () => {
  it.each<[string, string, string]>([
    [
      'Vendedor "X" inativado com sucesso.',
      "Failed to fetch",
      'Vendedor "X" inativado com sucesso. Porem, nao foi possivel recarregar a lista (Failed to fetch). Os dados exibidos podem estar desatualizados.',
    ],
    [
      "Pedido #9 criado com sucesso.",
      "erro 500",
      "Pedido #9 criado com sucesso. Porem, nao foi possivel recarregar a lista (erro 500). Os dados exibidos podem estar desatualizados.",
    ],
  ])("junta sucesso (%s) e erro (%s) em uma unica frase", (sucesso, erro, esperado) => {
    expect(mensagemRecargaFalhou(sucesso, erro)).toBe(esperado);
  });
});

describe("useExcluidos - resposta sem ids excluidos", () => {
  it("com excluidos marcados mas ausentes da resposta, devolve a mesma resposta", () => {
    const { result } = renderHook(() => useExcluidos((x: { id: number }) => x.id));
    result.current.marcar(99);
    const res = { data: [{ id: 1 }, { id: 2 }], total: 2 };
    expect(result.current.filtrar(res)).toBe(res);
  });
});
