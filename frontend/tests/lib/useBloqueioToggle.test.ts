/**
 * FE-18: hook de bloqueio do toggle de ativo (ref sincrona + state para o
 * disabled, bloqueio de 1500 ms no finally, timers limpos no unmount).
 */
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TOGGLE_BLOQUEIO_MS, useBloqueioToggle } from "@/lib/useBloqueioToggle";

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe("useBloqueioToggle", () => {
  it("o padrao e 1500 ms", () => {
    expect(TOGGLE_BLOQUEIO_MS).toBe(1500);
  });

  it("adquirir e sincrono: a segunda chamada no mesmo tick e recusada", () => {
    const { result } = renderHook(() => useBloqueioToggle<number>());
    let a = false;
    let b = true;
    act(() => {
      a = result.current.adquirir(1);
      b = result.current.adquirir(1);
    });
    expect(a).toBe(true);
    expect(b).toBe(false);
    // Em andamento nao conta como "bloqueado" para o disabled (so o finally).
    expect(result.current.estaBloqueado(1)).toBe(false);
  });

  it("a chave e independente: outra chave continua livre", () => {
    const { result } = renderHook(() => useBloqueioToggle<number>());
    expect(result.current.adquirir(1)).toBe(true);
    expect(result.current.adquirir(2)).toBe(true);
  });

  it("liberar devolve a chave sem bloqueio nem timer", () => {
    const { result } = renderHook(() => useBloqueioToggle<number>());
    result.current.adquirir(1);
    result.current.liberar(1);
    expect(vi.getTimerCount()).toBe(0);
    expect(result.current.estaBloqueado(1)).toBe(false);
    expect(result.current.adquirir(1)).toBe(true);
  });

  it("bloquear segura a chave por 1500 ms e depois libera", () => {
    const { result } = renderHook(() => useBloqueioToggle<number>());
    result.current.adquirir(7);
    act(() => result.current.bloquear(7));
    expect(result.current.estaBloqueado(7)).toBe(true);
    expect(result.current.adquirir(7)).toBe(false);
    act(() => {
      vi.advanceTimersByTime(1499);
    });
    expect(result.current.estaBloqueado(7)).toBe(true);
    expect(result.current.adquirir(7)).toBe(false);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current.estaBloqueado(7)).toBe(false);
    expect(vi.getTimerCount()).toBe(0);
    expect(result.current.adquirir(7)).toBe(true);
  });

  it("aceita outro tempo e chaves string", () => {
    const { result } = renderHook(() => useBloqueioToggle<string>(200));
    act(() => result.current.bloquear("x"));
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current.estaBloqueado("x")).toBe(false);
  });

  it("bloquear de novo reinicia o timer (um so timer por chave)", () => {
    const { result } = renderHook(() => useBloqueioToggle<number>());
    act(() => result.current.bloquear(1));
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    act(() => result.current.bloquear(1));
    expect(vi.getTimerCount()).toBe(1);
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(result.current.estaBloqueado(1)).toBe(true);
    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(result.current.estaBloqueado(1)).toBe(false);
  });

  it("unmount limpa os timers e bloquear depois dele nao arma timer", () => {
    const erro = vi.spyOn(console, "error");
    const { result, unmount } = renderHook(() => useBloqueioToggle<number>());
    act(() => {
      result.current.bloquear(1);
      result.current.bloquear(2);
    });
    expect(vi.getTimerCount()).toBe(2);
    const bloquear = result.current.bloquear;
    unmount();
    expect(vi.getTimerCount()).toBe(0);
    bloquear(3);
    expect(vi.getTimerCount()).toBe(0);
    vi.advanceTimersByTime(5000);
    expect(erro).not.toHaveBeenCalled();
  });
});
