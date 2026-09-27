import { describe, expect, it } from "vitest";
import { formatarData, formatarDataHora } from "@/lib/formatarData";

// Os valores esperados sao calculados com o mesmo Date local usado pelo
// formatador, para que o teste nao dependa do fuso da maquina; os casos de
// "so data" conferem tambem o dia literal (sem deslocamento por fuso).

const OPCOES_DATA_HORA: Intl.DateTimeFormatOptions = {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
};

describe("formatarData", () => {
  it.each<[string | null | undefined, string]>([
    ["", "vazio"],
    [null, "null"],
    [undefined, "undefined"],
  ])(
    "devolve '-' para %s (%s)",
    (valor) => {
      expect(formatarData(valor)).toBe("-");
    }
  );

  it("devolve '-' para a data zero do Go", () => {
    expect(formatarData("0001-01-01T00:00:00Z")).toBe("-");
  });

  it("devolve '-' para a data zero sem hora", () => {
    expect(formatarData("0001-01-01")).toBe("-");
  });

  it("devolve o texto original quando a data e invalida", () => {
    expect(formatarData("nao-e-data")).toBe("nao-e-data");
    expect(formatarData("2024-13-45")).toBe("2024-13-45");
  });

  it("formata AAAA-MM-DD sem deslocar o dia por fuso", () => {
    expect(formatarData("2024-01-15")).toBe("15/01/2024");
    expect(formatarData("2024-12-31")).toBe("31/12/2024");
    expect(formatarData("2024-01-01")).toBe("01/01/2024");
  });

  it("formata data com hora (ISO) no fuso local", () => {
    const iso = "2024-06-10T15:30:00Z";
    expect(formatarData(iso)).toBe(new Date(iso).toLocaleDateString("pt-BR"));
    expect(formatarData(iso)).toMatch(/^\d{2}\/\d{2}\/2024$/);
  });

  it("aceita a menor data valida apos o zero (ano 2)", () => {
    expect(formatarData("0002-06-15T12:00:00Z")).not.toBe("-");
  });
});

describe("formatarDataHora", () => {
  it.each<[string | null | undefined, string]>([
    ["", "vazio"],
    [null, "null"],
    [undefined, "undefined"],
  ])(
    "devolve '-' para %s (%s)",
    (valor) => {
      expect(formatarDataHora(valor)).toBe("-");
    }
  );

  it("devolve '-' para a data zero do Go", () => {
    expect(formatarDataHora("0001-01-01T00:00:00Z")).toBe("-");
  });

  it("devolve o texto original quando a data e invalida", () => {
    expect(formatarDataHora("xyz")).toBe("xyz");
  });

  it("formata data e hora com dia, mes, ano, hora, minuto e segundo", () => {
    const iso = "2026-09-26T13:45:07Z";
    const esperado = new Date(iso).toLocaleString("pt-BR", OPCOES_DATA_HORA);
    expect(formatarDataHora(iso)).toBe(esperado);
    expect(formatarDataHora(iso)).toMatch(
      /^\d{2}\/\d{2}\/2026,? \d{2}:\d{2}:07$/
    );
  });

  it("interpreta AAAA-MM-DD como meia-noite local", () => {
    expect(formatarDataHora("2024-01-15")).toMatch(/^15\/01\/2024,? 00:00:00$/);
  });
});
