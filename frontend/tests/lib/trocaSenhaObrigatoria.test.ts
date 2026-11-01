/**
 * SEC-23: contrato do 403 de troca de senha obrigatoria. Igualdade EXATA de
 * status + mensagem, como no vendedorDesligado.
 */
import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/apiError";
import { MSG_VENDEDOR_DESLIGADO } from "@/lib/vendedorDesligado";
import {
  MSG_TROCA_SENHA_OBRIGATORIA,
  ROTA_TROCAR_SENHA,
  isTrocaSenhaObrigatoriaError,
  isTrocaSenhaObrigatoriaResponse,
} from "@/lib/trocaSenhaObrigatoria";

describe("MSG_TROCA_SENHA_OBRIGATORIA", () => {
  it("segue o contrato exato do backend (com acento)", () => {
    expect(MSG_TROCA_SENHA_OBRIGATORIA).toBe("troca de senha obrigatória");
    expect(ROTA_TROCAR_SENHA).toBe("/trocar-senha");
  });
});

describe("isTrocaSenhaObrigatoriaError", () => {
  it.each<[string, unknown, boolean]>([
    ["403 + mensagem exata", new ApiError(403, MSG_TROCA_SENHA_OBRIGATORIA), true],
    ["401 + mensagem exata", new ApiError(401, MSG_TROCA_SENHA_OBRIGATORIA), false],
    ["400 + mensagem exata", new ApiError(400, MSG_TROCA_SENHA_OBRIGATORIA), false],
    ["500 + mensagem exata", new ApiError(500, MSG_TROCA_SENHA_OBRIGATORIA), false],
    ["403 + outra mensagem", new ApiError(403, "acesso negado"), false],
    ["403 + vendedor desligado", new ApiError(403, MSG_VENDEDOR_DESLIGADO), false],
    ["403 + sem acento", new ApiError(403, "troca de senha obrigatoria"), false],
    ["403 + maiusculas", new ApiError(403, "Troca de senha obrigatória"), false],
    ["403 + ponto final", new ApiError(403, `${MSG_TROCA_SENHA_OBRIGATORIA}.`), false],
    ["403 + espaco extra", new ApiError(403, ` ${MSG_TROCA_SENHA_OBRIGATORIA}`), false],
    ["403 + prefixo", new ApiError(403, `erro: ${MSG_TROCA_SENHA_OBRIGATORIA}`), false],
    ["Error comum com a mensagem exata", new Error(MSG_TROCA_SENHA_OBRIGATORIA), false],
    [
      "objeto com status/message (nao ApiError)",
      { status: 403, message: MSG_TROCA_SENHA_OBRIGATORIA },
      false,
    ],
    ["string", MSG_TROCA_SENHA_OBRIGATORIA, false],
    ["null", null, false],
    ["undefined", undefined, false],
  ])("%s -> %s", (_nome, err, esperado) => {
    expect(isTrocaSenhaObrigatoriaError(err)).toBe(esperado);
  });
});

describe("isTrocaSenhaObrigatoriaResponse", () => {
  it.each<[number, string, boolean]>([
    [403, MSG_TROCA_SENHA_OBRIGATORIA, true],
    [401, MSG_TROCA_SENHA_OBRIGATORIA, false],
    [404, MSG_TROCA_SENHA_OBRIGATORIA, false],
    [403, "acesso negado", false],
    [403, MSG_VENDEDOR_DESLIGADO, false],
    [403, "", false],
    [403, MSG_TROCA_SENHA_OBRIGATORIA.toUpperCase(), false],
  ])("status %i + %j -> %s", (status, msg, esperado) => {
    expect(isTrocaSenhaObrigatoriaResponse(status, msg)).toBe(esperado);
  });
});
