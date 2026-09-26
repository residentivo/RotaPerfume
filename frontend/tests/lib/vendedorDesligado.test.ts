import { describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/apiError";
import {
  MSG_VENDEDOR_DESLIGADO,
  isVendedorDesligadoError,
  isVendedorDesligadoResponse,
  notifyVendedorDesligado,
  subscribeVendedorDesligado,
} from "@/lib/vendedorDesligado";

describe("MSG_VENDEDOR_DESLIGADO", () => {
  it("segue o contrato exato do backend", () => {
    expect(MSG_VENDEDOR_DESLIGADO).toBe("acesso bloqueado: vendedor desligado");
  });
});

describe("isVendedorDesligadoError", () => {
  it.each<[string, unknown, boolean]>([
    ["403 + mensagem exata", new ApiError(403, MSG_VENDEDOR_DESLIGADO), true],
    ["401 + mensagem exata", new ApiError(401, MSG_VENDEDOR_DESLIGADO), false],
    ["400 + mensagem exata", new ApiError(400, MSG_VENDEDOR_DESLIGADO), false],
    ["500 + mensagem exata", new ApiError(500, MSG_VENDEDOR_DESLIGADO), false],
    ["403 + outra mensagem", new ApiError(403, "acesso negado"), false],
    ["403 + maiusculas", new ApiError(403, "Acesso bloqueado: vendedor desligado"), false],
    ["403 + ponto final", new ApiError(403, `${MSG_VENDEDOR_DESLIGADO}.`), false],
    ["403 + espaco extra", new ApiError(403, ` ${MSG_VENDEDOR_DESLIGADO}`), false],
    ["403 + prefixo", new ApiError(403, `erro: ${MSG_VENDEDOR_DESLIGADO}`), false],
    ["403 + so 'vendedor desligado'", new ApiError(403, "vendedor desligado"), false],
    ["Error comum com a mensagem exata", new Error(MSG_VENDEDOR_DESLIGADO), false],
    [
      "objeto com status/message (nao ApiError)",
      { status: 403, message: MSG_VENDEDOR_DESLIGADO },
      false,
    ],
    ["string", MSG_VENDEDOR_DESLIGADO, false],
    ["null", null, false],
    ["undefined", undefined, false],
  ])("%s -> %s", (_nome, err, esperado) => {
    expect(isVendedorDesligadoError(err)).toBe(esperado);
  });
});

describe("isVendedorDesligadoResponse", () => {
  it.each<[number, string, boolean]>([
    [403, MSG_VENDEDOR_DESLIGADO, true],
    [401, MSG_VENDEDOR_DESLIGADO, false],
    [404, MSG_VENDEDOR_DESLIGADO, false],
    [403, "acesso negado", false],
    [403, "", false],
    [403, MSG_VENDEDOR_DESLIGADO.toUpperCase(), false],
  ])("status %i + %j -> %s", (status, msg, esperado) => {
    expect(isVendedorDesligadoResponse(status, msg)).toBe(esperado);
  });
});

describe("subscribe/notify", () => {
  it("notifica todos os inscritos e para de notificar apos unsubscribe", () => {
    const a = vi.fn();
    const b = vi.fn();
    const offA = subscribeVendedorDesligado(a);
    const offB = subscribeVendedorDesligado(b);

    notifyVendedorDesligado();
    expect(a).toHaveBeenCalledTimes(1);
    expect(b).toHaveBeenCalledTimes(1);

    offA();
    notifyVendedorDesligado();
    expect(a).toHaveBeenCalledTimes(1);
    expect(b).toHaveBeenCalledTimes(2);

    offB();
    notifyVendedorDesligado();
    expect(b).toHaveBeenCalledTimes(2);
  });
});

describe("ApiError", () => {
  it("preserva status, mensagem e nome", () => {
    const e = new ApiError(418, "teapot");
    expect(e).toBeInstanceOf(Error);
    expect(e.status).toBe(418);
    expect(e.message).toBe("teapot");
    expect(e.name).toBe("ApiError");
  });
});
