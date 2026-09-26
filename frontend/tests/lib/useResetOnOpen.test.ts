import { renderHook } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { useAjustarAoMudar, useResetOnOpen } from "@/lib/useResetOnOpen";

const OBJ = { id: 1 };

describe("useAjustarAoMudar", () => {
  it("roda na primeira renderizacao", () => {
    const ajustar = vi.fn();
    renderHook(() => useAjustarAoMudar([1, "a"], ajustar));
    expect(ajustar).toHaveBeenCalledTimes(1);
  });

  it.each<[string, unknown[], unknown[], boolean]>([
    ["mesmas deps primitivas", [1, "a"], [1, "a"], false],
    ["mesmo objeto (referencia)", [OBJ], [OBJ], false],
    ["objeto novo com mesmo conteudo", [{ a: 1 }], [{ a: 1 }], true],
    ["primitivo mudou", [1, "a"], [2, "a"], true],
    ["NaN e igual a NaN (Object.is)", [NaN], [NaN], false],
    ["+0 e -0 sao diferentes (Object.is)", [0], [-0], true],
    ["quantidade de deps mudou", [1], [1, 2], true],
    ["null -> undefined", [null], [undefined], true],
  ])("%s -> roda de novo = %s", (_n, antes, depois, rodaDeNovo) => {
    const ajustar = vi.fn();
    const { rerender } = renderHook(({ deps }) => useAjustarAoMudar(deps, ajustar), {
      initialProps: { deps: antes },
    });
    ajustar.mockClear();
    rerender({ deps: depois });
    expect(ajustar).toHaveBeenCalledTimes(rodaDeNovo ? 1 : 0);
  });

  it("setState dentro de ajustar e aplicado antes do render ser exposto (sem loop)", () => {
    const renders: number[] = [];
    const { result, rerender } = renderHook(
      ({ v }) => {
        const [espelho, setEspelho] = useState(-1);
        useAjustarAoMudar([v], () => setEspelho(v * 10));
        renders.push(espelho);
        return espelho;
      },
      { initialProps: { v: 1 } }
    );
    expect(result.current).toBe(10);
    rerender({ v: 2 });
    expect(result.current).toBe(20);
    // Re-render sem mudar a dep nao ajusta de novo.
    rerender({ v: 2 });
    expect(result.current).toBe(20);
    expect(renders.at(-1)).toBe(20);
  });
});


describe("useResetOnOpen", () => {
  function montar(initial: { open: boolean; deps: unknown[] }) {
    const reset = vi.fn();
    const hook = renderHook(({ open, deps }) => useResetOnOpen(open, deps, reset), {
      initialProps: initial,
    });
    return { reset, ...hook };
  }

  it("nao roda enquanto o modal esta fechado", () => {
    const { reset, rerender } = montar({ open: false, deps: ["create", null] });
    rerender({ open: false, deps: ["edit", OBJ] });
    expect(reset).not.toHaveBeenCalled();
  });

  it("roda ao montar ja aberto", () => {
    const { reset } = montar({ open: true, deps: ["create", null] });
    expect(reset).toHaveBeenCalledTimes(1);
  });

  it("roda ao abrir, nao roda de novo em re-render com as mesmas deps", () => {
    const { reset, rerender } = montar({ open: false, deps: ["create", null] });
    rerender({ open: true, deps: ["create", null] });
    expect(reset).toHaveBeenCalledTimes(1);
    rerender({ open: true, deps: ["create", null] });
    expect(reset).toHaveBeenCalledTimes(1);
  });

  it("fechar nao roda; reabrir roda de novo (modal reabre resetado)", () => {
    const { reset, rerender } = montar({ open: true, deps: ["create", null] });
    rerender({ open: false, deps: ["create", null] });
    expect(reset).toHaveBeenCalledTimes(1);
    rerender({ open: true, deps: ["create", null] });
    expect(reset).toHaveBeenCalledTimes(2);
  });

  it.each<[string, unknown[]]>([
    ["mode muda", ["edit", null]],
    ["registro editado muda", ["create", OBJ]],
  ])("com o modal aberto, roda quando %s", (_n, novasDeps) => {
    const { reset, rerender } = montar({ open: true, deps: ["create", null] });
    rerender({ open: true, deps: novasDeps });
    expect(reset).toHaveBeenCalledTimes(2);
  });

  it("deps mudam com o modal fechado e ele abre depois: roda uma vez so", () => {
    const { reset, rerender } = montar({ open: false, deps: ["create", null] });
    rerender({ open: false, deps: ["edit", OBJ] });
    rerender({ open: true, deps: ["edit", OBJ] });
    expect(reset).toHaveBeenCalledTimes(1);
  });
});
