/**
 * FE-11: timers seguros (useTimeoutSeguro) e mensagem temporaria
 * (useMensagemTemporaria). Tudo com fake timers do vitest.
 */
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  DURACAO_MENSAGEM_MS,
  useMensagemTemporaria,
  useTimeoutSeguro,
} from "@/lib/useMensagemTemporaria";

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe("useTimeoutSeguro (FE-11)", () => {
  it("agendar executa a acao depois do atraso, uma unica vez", () => {
    const { result } = renderHook(() => useTimeoutSeguro());
    const acao = vi.fn();
    act(() => result.current.agendar(acao, 1500));
    vi.advanceTimersByTime(1499);
    expect(acao).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(acao).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(10_000);
    expect(acao).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("agendar de novo cancela o timer anterior (um timer por vez)", () => {
    const { result } = renderHook(() => useTimeoutSeguro());
    const primeira = vi.fn();
    const segunda = vi.fn();
    act(() => result.current.agendar(primeira, 1000));
    vi.advanceTimersByTime(600);
    act(() => result.current.agendar(segunda, 1000));
    expect(vi.getTimerCount()).toBe(1);
    vi.advanceTimersByTime(600);
    expect(primeira).not.toHaveBeenCalled();
    expect(segunda).not.toHaveBeenCalled();
    vi.advanceTimersByTime(400);
    expect(segunda).toHaveBeenCalledTimes(1);
    expect(primeira).not.toHaveBeenCalled();
  });

  it("cancelar impede a acao; cancelar sem timer pendente nao falha", () => {
    const { result } = renderHook(() => useTimeoutSeguro());
    const acao = vi.fn();
    act(() => result.current.cancelar());
    act(() => result.current.agendar(acao, 500));
    act(() => result.current.cancelar());
    expect(vi.getTimerCount()).toBe(0);
    vi.advanceTimersByTime(1000);
    expect(acao).not.toHaveBeenCalled();
  });

  it("cancelar depois que o timer ja venceu nao chama clearTimeout", () => {
    const { result } = renderHook(() => useTimeoutSeguro());
    const acao = vi.fn();
    act(() => result.current.agendar(acao, 100));
    vi.advanceTimersByTime(100);
    const clear = vi.spyOn(globalThis, "clearTimeout");
    act(() => result.current.cancelar());
    expect(clear).not.toHaveBeenCalled();
    expect(acao).toHaveBeenCalledTimes(1);
  });

  it.each<[string, number]>([
    ["logo apos agendar", 0],
    ["no meio do prazo", 750],
    ["1ms antes de vencer", 1499],
  ])("unmount %s cancela o timer e a acao nunca roda", (_n, decorrido) => {
    const { result, unmount } = renderHook(() => useTimeoutSeguro());
    const acao = vi.fn();
    act(() => result.current.agendar(acao, 1500));
    vi.advanceTimersByTime(decorrido);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
    vi.advanceTimersByTime(5000);
    expect(acao).not.toHaveBeenCalled();
  });

  it("unmount sem timer pendente nao falha", () => {
    const { unmount } = renderHook(() => useTimeoutSeguro());
    expect(() => unmount()).not.toThrow();
  });

  it("agendar/cancelar tem identidade estavel entre renders", () => {
    const { result, rerender } = renderHook(() => useTimeoutSeguro());
    const { agendar, cancelar } = result.current;
    rerender();
    expect(result.current.agendar).toBe(agendar);
    expect(result.current.cancelar).toBe(cancelar);
  });
});

describe("useMensagemTemporaria (FE-11)", () => {
  it("duracao padrao e de 4 s", () => {
    expect(DURACAO_MENSAGEM_MS).toBe(4000);
  });

  it.each<[string, number | undefined, number]>([
    ["padrao (4000ms)", undefined, 4000],
    ["personalizada (1000ms)", 1000, 1000],
    ["personalizada (250ms)", 250, 250],
  ])("mostrar exibe e a mensagem some sozinha apos a duracao %s", (_n, duracao, ms) => {
    const { result } = renderHook(() => useMensagemTemporaria(duracao));
    expect(result.current.mensagem).toBeNull();
    act(() => result.current.mostrar("Salvo."));
    expect(result.current.mensagem).toBe("Salvo.");
    act(() => vi.advanceTimersByTime(ms - 1));
    expect(result.current.mensagem).toBe("Salvo.");
    act(() => vi.advanceTimersByTime(1));
    expect(result.current.mensagem).toBeNull();
  });

  it("mensagem nova reinicia o prazo (o timer da anterior nao a apaga)", () => {
    const { result } = renderHook(() => useMensagemTemporaria());
    act(() => result.current.mostrar("primeira"));
    act(() => vi.advanceTimersByTime(3000));
    act(() => result.current.mostrar("segunda"));
    expect(vi.getTimerCount()).toBe(1);
    // 4 s depois da primeira: a segunda continua visivel.
    act(() => vi.advanceTimersByTime(1500));
    expect(result.current.mensagem).toBe("segunda");
    act(() => vi.advanceTimersByTime(2499));
    expect(result.current.mensagem).toBe("segunda");
    act(() => vi.advanceTimersByTime(1));
    expect(result.current.mensagem).toBeNull();
  });

  it("limpar esconde na hora e cancela o timer", () => {
    const { result } = renderHook(() => useMensagemTemporaria());
    act(() => result.current.mostrar("x"));
    act(() => result.current.limpar());
    expect(result.current.mensagem).toBeNull();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("limpar e mostrar de novo: a nova mensagem fica o prazo inteiro", () => {
    const { result } = renderHook(() => useMensagemTemporaria(1000));
    act(() => result.current.mostrar("a"));
    act(() => vi.advanceTimersByTime(900));
    act(() => result.current.limpar());
    act(() => result.current.mostrar("b"));
    act(() => vi.advanceTimersByTime(900));
    expect(result.current.mensagem).toBe("b");
    act(() => vi.advanceTimersByTime(100));
    expect(result.current.mensagem).toBeNull();
  });

  it("unmount antes do prazo: nenhum timer pendente e nenhum setState depois", () => {
    const erro = vi.spyOn(console, "error").mockImplementation(() => {});
    const renders = vi.fn();
    const { result, unmount } = renderHook(() => {
      const m = useMensagemTemporaria();
      renders(m.mensagem);
      return m;
    });
    act(() => result.current.mostrar("some depois"));
    act(() => vi.advanceTimersByTime(2000));
    const antes = renders.mock.calls.length;
    unmount();
    expect(vi.getTimerCount()).toBe(0);
    vi.advanceTimersByTime(10_000);
    expect(renders.mock.calls.length).toBe(antes);
    expect(result.current.mensagem).toBe("some depois");
    expect(erro).not.toHaveBeenCalled();
  });
});
