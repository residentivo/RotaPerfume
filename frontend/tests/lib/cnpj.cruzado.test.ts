/**
 * NEG-02 (TestBrain, Lote 5): verificacao cruzada back x front.
 *
 * A massa apis/shared/cnpj/testdata/casos_cruzados.json e a MESMA lida pelos
 * testes Go (apis/shared/cnpj/cruzado_test.go e
 * apis/rotaperfumes-api/services/cnpj_cruzado_internal_test.go). Foi gerada
 * por uma 3a implementacao de referencia do DV, independente das duas.
 *
 * - `casos`: back e front precisam concordar (validade e valor normalizado).
 * - `divergencias`: casos em que o front divergia do back (NEG-02-A/B, Lote 5:
 *   aceitava tab/NBSP/quebra de linha no meio e "ı" como "I"). Corrigido;
 *   ficam como regressao e o esperado continua sendo o contrato do backend.
 */
import { describe, expect, it } from "vitest";
import massa from "../../../apis/shared/cnpj/testdata/casos_cruzados.json";
import {
  formatCnpj,
  isValidCnpj,
  maskCnpjInput,
  MSG_CNPJ_INVALIDO,
  normalizeCnpj,
  validateCnpj,
} from "@/lib/cnpj";

interface CasoCruzado {
  descricao: string;
  entrada: string;
  valido: boolean;
  normalizado?: string;
}

const casos = massa.casos as CasoCruzado[];
const divergencias = massa.divergencias as CasoCruzado[];

describe("cnpj: massa comum com o backend (Go)", () => {
  it("a massa tem casos validos e invalidos suficientes", () => {
    expect(casos.filter((c) => c.valido).length).toBeGreaterThanOrEqual(20);
    expect(casos.filter((c) => !c.valido).length).toBeGreaterThanOrEqual(20);
    expect(casos.map((c) => c.entrada)).toEqual(
      expect.arrayContaining(["12ABC34501DE35", "11222333000181"])
    );
  });

  it.each(casos)("isValidCnpj: $descricao", ({ entrada, valido }) => {
    expect(isValidCnpj(entrada)).toBe(valido);
  });

  it.each(casos.filter((c) => c.valido))(
    "normalizeCnpj envia o mesmo valor que o back grava: $descricao",
    ({ entrada, normalizado }) => {
      expect(normalizeCnpj(entrada)).toBe(normalizado);
    }
  );

  it.each(casos.filter((c) => c.valido))(
    "fluxo da tela (digitar -> mascara -> validar -> enviar): $descricao",
    ({ entrada, normalizado }) => {
      const noInput = maskCnpjInput(entrada);
      expect(validateCnpj(noInput)).toBeNull();
      expect(normalizeCnpj(noInput)).toBe(normalizado);
    }
  );

  it.each(casos.filter((c) => c.valido))(
    "formatCnpj do valor devolvido pela API e reversivel: $descricao",
    ({ normalizado }) => {
      const exibido = formatCnpj(normalizado!);
      expect(exibido).toHaveLength(18);
      expect(normalizeCnpj(exibido)).toBe(normalizado);
      expect(isValidCnpj(exibido)).toBe(true);
    }
  );
});

describe("cnpj: ex-divergencias com o backend (Lote 5, corrigidas)", () => {
  // NEG-02-A/B: o front aceitava tab/NBSP/quebra de linha no meio e "ı" como
  // "I". Alinhado ao contrato do back; os casos ficam como regressao.
  it.each(divergencias)(
    "contrato do back: $descricao",
    ({ entrada, valido }) => {
      expect(isValidCnpj(entrada)).toBe(valido);
      expect(validateCnpj(entrada)).toBe(valido ? null : MSG_CNPJ_INVALIDO);
    }
  );

  it("na tela a entrada e sanitizada: so [0-9A-Z] e mascara chegam a API", () => {
    for (const { entrada } of divergencias) {
      const noInput = maskCnpjInput(entrada);
      expect(noInput).toMatch(/^[0-9A-Z./-]*$/);
      expect(normalizeCnpj(noInput)).toMatch(/^[0-9A-Z]*$/);
    }
    // "ı" e descartado no input, nao convertido em "I".
    expect(maskCnpjInput("12ABı34501DE42")).toBe("12.AB3.450/1DE4-2");
  });
});
