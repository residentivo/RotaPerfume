/**
 * NEG-02: util unico de CNPJ (numerico e alfanumerico). Espelha a regra do
 * backend (apis/shared/cnpj): normalizacao, formato, DV modulo 11 com valor
 * ASCII - 48 e rejeicao de todos os caracteres iguais.
 */
import { describe, expect, it } from "vitest";
import {
  CNPJ_LENGTH,
  CNPJ_MASKED_LENGTH,
  MSG_CNPJ_INVALIDO,
  MSG_CNPJ_OBRIGATORIO,
  formatCnpj,
  isValidCnpj,
  maskCnpjInput,
  normalizeCnpj,
  sanitizeCnpjInput,
  validateCnpj,
} from "@/lib/cnpj";

const NUMERICO = "11222333000181";
const ALFA = "12ABC34501DE35"; // exemplo oficial da Receita Federal

describe("normalizeCnpj", () => {
  it.each([
    ["11.222.333/0001-81", NUMERICO],
    ["12.abc.345/01de-35", ALFA],
    [" 12 ABC 345 01DE 35 ", ALFA],
    ["12abc34501de35", ALFA],
    [null, ""],
    [undefined, ""],
  ])("%j -> %j", (entrada, esperado) => {
    expect(normalizeCnpj(entrada)).toBe(esperado);
  });

  it("mantem caracteres fora da mascara (para a validacao rejeitar)", () => {
    expect(normalizeCnpj("12_ABC#34501DE35")).toBe("12_ABC#34501DE35");
  });
});

describe("isValidCnpj", () => {
  it.each([
    ["numerico", NUMERICO],
    ["numerico com mascara", "11.222.333/0001-81"],
    ["alfanumerico", ALFA],
    ["alfanumerico com mascara", "12.ABC.345/01DE-35"],
    ["alfanumerico em minusculas", "12abc34501de35"],
    ["minusculas com mascara colada", "12.abc.345/01de-35"],
  ])("aceita %s", (_n, v) => {
    expect(isValidCnpj(v)).toBe(true);
  });

  it.each([
    ["vazio", ""],
    ["curto", "1122233300018"],
    ["longo", "112223330001810"],
    ["DV errado (numerico)", "11222333000182"],
    ["DV errado (alfanumerico)", "12ABC34501DE36"],
    ["primeiro DV errado", "12ABC34501DE45"],
    ["letra no DV", "12ABC34501DE3A"],
    ["letras nos dois DVs", "12ABC34501DEAB"],
    ["caractere invalido", "12ABC345_1DE35"],
    ["acento", "12ÁBC34501DE35"],
    ["todos iguais (zeros)", "00000000000000"],
    ["todos iguais (uns)", "11111111111111"],
    ["todos iguais (letra, DV nao numerico)", "AAAAAAAAAAAAAA"],
  ])("rejeita %s", (_n, v) => {
    expect(isValidCnpj(v)).toBe(false);
  });

  // NEG-02-A/B: espelho de strings.TrimSpace + cnpj.Normalizar do Go.
  it.each([
    ["tab no meio", "11.222.333\t0001-81"],
    ["NBSP no meio", "11.222.333/0001 -81"],
    ["quebra de linha no meio", "12ABC345\n01DE35"],
    ["i sem ponto (U+0131) virando I", "12ABı34501DE42"],
    ["s longo (U+017F) virando S", "12ABC34501DE35".replace("C", "ſ")],
    ["BOM na borda (Go TrimSpace nao remove)", "﻿11222333000181"],
  ])("rejeita %s (contrato do back)", (_n, v) => {
    expect(isValidCnpj(v)).toBe(false);
    expect(validateCnpj(v)).toBe(MSG_CNPJ_INVALIDO);
  });

  it.each([
    ["espacos/tab/quebra nas bordas", " \t11.222.333/0001-81\r\n "],
    ["NBSP e U+0085 nas bordas", " \u008512ABC34501DE35　"],
  ])("aceita %s (TrimSpace do service)", (_n, v) => {
    expect(isValidCnpj(v)).toBe(true);
  });

  it("rejeita null/undefined", () => {
    expect(isValidCnpj(null)).toBe(false);
    expect(isValidCnpj(undefined)).toBe(false);
  });
});

describe("validateCnpj", () => {
  it("vazio ou so mascara -> obrigatorio", () => {
    expect(validateCnpj("")).toBe(MSG_CNPJ_OBRIGATORIO);
    expect(validateCnpj("  ")).toBe(MSG_CNPJ_OBRIGATORIO);
    expect(validateCnpj("../-")).toBe(MSG_CNPJ_OBRIGATORIO);
  });

  it("invalido -> mensagem de invalido", () => {
    expect(validateCnpj("11222333000182")).toBe(MSG_CNPJ_INVALIDO);
    expect(validateCnpj("12ABC34501DE3A")).toBe(MSG_CNPJ_INVALIDO);
  });

  it("valido -> null", () => {
    expect(validateCnpj(NUMERICO)).toBeNull();
    expect(validateCnpj("12.abc.345/01de-35")).toBeNull();
  });
});

describe("sanitizeCnpjInput", () => {
  it("converte para maiusculas e remove mascara", () => {
    expect(sanitizeCnpjInput("12.abc.345/01de-35")).toBe(ALFA);
  });

  it("descarta caracteres fora de [0-9A-Z]", () => {
    expect(sanitizeCnpjInput("12#ab_c!")).toBe("12ABC");
  });

  it("descarta letras nao ASCII em vez de converte-las (ı, ß, Á)", () => {
    expect(sanitizeCnpjInput("12ıßÁab")).toBe("12AB");
  });

  it("aceita letras nas 12 primeiras posicoes, mas nao no DV", () => {
    expect(sanitizeCnpjInput("12ABC34501DE")).toBe("12ABC34501DE");
    expect(sanitizeCnpjInput("12ABC34501DEX")).toBe("12ABC34501DE");
    expect(sanitizeCnpjInput("12ABC34501DE3X")).toBe("12ABC34501DE3");
    expect(sanitizeCnpjInput("12ABC34501DEX3Y5")).toBe(ALFA);
  });

  it(`limita a ${CNPJ_LENGTH} caracteres`, () => {
    expect(sanitizeCnpjInput(NUMERICO + "999")).toBe(NUMERICO);
  });
});

describe("maskCnpjInput (mascara progressiva)", () => {
  it.each([
    ["", ""],
    ["1", "1"],
    ["12", "12"],
    ["12a", "12.A"],
    ["12abc3", "12.ABC.3"],
    ["12abc345", "12.ABC.345"],
    ["12abc3450", "12.ABC.345/0"],
    ["12abc34501de", "12.ABC.345/01DE"],
    ["12abc34501de3", "12.ABC.345/01DE-3"],
    ["12abc34501de35", "12.ABC.345/01DE-35"],
    ["11222333000181", "11.222.333/0001-81"],
    ["12.ABC.345/01DE-35", "12.ABC.345/01DE-35"],
  ])("%j -> %j", (entrada, esperado) => {
    expect(maskCnpjInput(entrada)).toBe(esperado);
  });

  it(`nunca passa de ${CNPJ_MASKED_LENGTH} caracteres`, () => {
    expect(maskCnpjInput("12abc34501de35999AAA")).toHaveLength(CNPJ_MASKED_LENGTH);
  });
});

describe("formatCnpj (exibicao)", () => {
  it("numerico", () => {
    expect(formatCnpj(NUMERICO)).toBe("11.222.333/0001-81");
  });

  it("alfanumerico", () => {
    expect(formatCnpj(ALFA)).toBe("12.ABC.345/01DE-35");
  });

  it("minusculas viram maiusculas", () => {
    expect(formatCnpj("12abc34501de35")).toBe("12.ABC.345/01DE-35");
  });

  it("ja mascarado permanece igual", () => {
    expect(formatCnpj("12.ABC.345/01DE-35")).toBe("12.ABC.345/01DE-35");
  });

  it("fora do padrao devolve o valor original (dado legado)", () => {
    expect(formatCnpj("00")).toBe("00");
    expect(formatCnpj("12ABC34501DEXY")).toBe("12ABC34501DEXY");
  });

  it("vazio/null -> string vazia", () => {
    expect(formatCnpj("")).toBe("");
    expect(formatCnpj(null)).toBe("");
    expect(formatCnpj(undefined)).toBe("");
  });
});
