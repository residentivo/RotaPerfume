/**
 * DASH-01: FunilVendas (seção "Funil de vendas" do Dashboard).
 * - helpers puros (whitelist de etapa, largura da barra, zerado, normalização);
 * - componente: seletor de janela, 5 etapas com percentuais, rodapé, vazio,
 *   loading, erro genérico (sem corpo), desligado nas 3 origens, recarga e
 *   cancelamento de buscas antigas.
 * A API e a sessão são mockadas.
 */
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/apiError";
import type { FunilJanela, FunilVendas as FunilVendasDados } from "@/lib/types";

const api = vi.hoisted(() => ({ apiDashboardFunil: vi.fn() }));
vi.mock("@/lib/api", () => api);

const useVendedorDesligadoMock = vi.fn<() => boolean>();
vi.mock("@/lib/session", () => ({
  useVendedorDesligado: () => useVendedorDesligadoMock(),
}));

import {
  ERRO_FUNIL,
  ETAPAS_FUNIL,
  FunilVendas,
  JANELAS_FUNIL,
  JANELA_LABELS,
  LARGURA_MINIMA_PCT,
  etapaSegura,
  funilZerado,
  larguraBarra,
  normalizarFunil,
} from "@/components/dashboard/FunilVendas";

// ─── Fixtures ─────────────────────────────────────────────────────────────────

const AVISO_DESLIGADO = /Vendedor desligado\. Nao ha oportunidades/;
const VAZIO = "Nenhuma oportunidade na janela selecionada";

function funilCheio(extra: Partial<FunilVendasDados> = {}): FunilVendasDados {
  return {
    janela: "all",
    total_oportunidades: 1234,
    etapas: [
      { etapa: "Prospecção", quantidade: 1000, valor_estimado: 100000, conversao_anterior_pct: 100, conversao_topo_pct: 100 },
      { etapa: "Qualificação", quantidade: 600, valor_estimado: 60000, conversao_anterior_pct: 60, conversao_topo_pct: 60 },
      { etapa: "Proposta enviada", quantidade: 300, valor_estimado: 30000, conversao_anterior_pct: 50, conversao_topo_pct: 30 },
      { etapa: "Negociação", quantidade: 200, valor_estimado: 20000, conversao_anterior_pct: 66.67, conversao_topo_pct: 20 },
      { etapa: "Fechado ganho", quantidade: 50, valor_estimado: 5000.5, conversao_anterior_pct: 25, conversao_topo_pct: 5 },
    ],
    perdidas: { quantidade: 150, valor_estimado: 15000.25 },
    taxa_ganho_pct: 25,
    vendedor_desligado: false,
    ...extra,
  };
}

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function barras(): HTMLElement[] {
  const lista = screen.getByRole("list", { name: "Etapas do funil de vendas" });
  return within(lista).getAllByRole("img");
}

/** Texto do <dd> que segue o <dt> com o rótulo informado. */
function rodape(rotulo: string): string {
  return screen.getByText(rotulo, { selector: "dt" }).nextElementSibling?.textContent ?? "";
}

function botaoJanela(j: FunilJanela): HTMLElement {
  return within(screen.getByRole("group", { name: "Janela do funil" })).getByRole("button", {
    name: JANELA_LABELS[j],
  });
}

async function renderCarregado(props: Partial<Parameters<typeof FunilVendas>[0]> = {}) {
  const r = render(<FunilVendas isAdmin {...props} />);
  await waitFor(() => expect(screen.queryByLabelText("Carregando funil")).not.toBeInTheDocument());
  return r;
}

beforeEach(() => {
  vi.clearAllMocks();
  useVendedorDesligadoMock.mockReturnValue(false);
  api.apiDashboardFunil.mockResolvedValue(funilCheio());
});

// ─── Helpers puros ────────────────────────────────────────────────────────────

describe("FunilVendas - constantes", () => {
  it("janelas do enum, na ordem do seletor, com rótulos", () => {
    expect(JANELAS_FUNIL).toEqual(["30d", "90d", "365d", "all"]);
    expect(JANELA_LABELS).toEqual({ "30d": "30 dias", "90d": "90 dias", "365d": "12 meses", all: "Tudo" });
    expect(ETAPAS_FUNIL).toEqual(["Prospecção", "Qualificação", "Proposta enviada", "Negociação", "Fechado ganho"]);
    expect(LARGURA_MINIMA_PCT).toBe(8);
    expect(ERRO_FUNIL).toBe("Nao foi possivel carregar o funil de vendas. Tente novamente.");
  });
});

describe("etapaSegura (whitelist)", () => {
  it.each(ETAPAS_FUNIL.map((e) => [e]))("%s é conhecida", (etapa) => {
    expect(etapaSegura(etapa)).toBe(etapa);
  });

  it.each<[string, unknown]>([
    ["Fechado perdido", "Fechado perdido"],
    ["caixa diferente", "prospecção"],
    ["espaço extra", "Prospecção "],
    ["vazia", ""],
    ["html", "<img src=x onerror=alert(1)>"],
    ["Outras literal", "Outras"],
    ["null", null],
    ["undefined", undefined],
    ["número", 1],
    ["objeto", { toString: () => "Prospecção" }],
  ])("%s → Outras", (_nome, etapa) => {
    expect(etapaSegura(etapa)).toBe("Outras");
  });
});

describe("larguraBarra", () => {
  it.each<[unknown, number]>([
    [100, 100],
    [50, 50],
    [8, 8],
    [7.99, LARGURA_MINIMA_PCT],
    [0, LARGURA_MINIMA_PCT],
    [-5, LARGURA_MINIMA_PCT],
    [150, 100],
    [Number.NaN, LARGURA_MINIMA_PCT],
    [Number.POSITIVE_INFINITY, LARGURA_MINIMA_PCT],
    ["abc", LARGURA_MINIMA_PCT],
    ["42", 42],
    [null, LARGURA_MINIMA_PCT],
    [undefined, LARGURA_MINIMA_PCT],
  ])("%s → %s", (entrada, esperado) => {
    expect(larguraBarra(entrada as number)).toBe(esperado);
  });
});

describe("funilZerado", () => {
  it.each(JANELAS_FUNIL.map((j) => [j]))("%s: 5 etapas zeradas e sem aviso", (janela) => {
    const f = funilZerado(janela);
    expect(f.janela).toBe(janela);
    expect(f.total_oportunidades).toBe(0);
    expect(f.etapas.map((e) => e.etapa)).toEqual([...ETAPAS_FUNIL]);
    for (const e of f.etapas) {
      expect(e).toEqual({ etapa: e.etapa, quantidade: 0, valor_estimado: 0, conversao_anterior_pct: 0, conversao_topo_pct: 0 });
    }
    expect(f.perdidas).toEqual({ quantidade: 0, valor_estimado: 0 });
    expect(f.taxa_ganho_pct).toBe(0);
    expect(f.vendedor_desligado).toBe(false);
  });

  it("devolve objetos novos a cada chamada", () => {
    expect(funilZerado("all")).not.toBe(funilZerado("all"));
    expect(funilZerado("all").etapas).not.toBe(funilZerado("all").etapas);
  });
});

describe("normalizarFunil", () => {
  it.each<[string, unknown]>([
    ["null", null],
    ["undefined", undefined],
    ["string", "erro"],
    ["número", 42],
    ["objeto vazio", {}],
    ["etapas não é array", { etapas: "x", total_oportunidades: 9 }],
    ["etapas null", { etapas: null, total_oportunidades: 9 }],
  ])("%s → funil zerado da janela pedida", (_nome, dados) => {
    expect(normalizarFunil(dados, "90d")).toEqual(funilZerado("90d"));
  });

  it("payload válido passa sem alteração (janela = a pedida)", () => {
    const dados = funilCheio({ janela: "all" });
    expect(normalizarFunil(dados, "30d")).toEqual({ ...dados, janela: "30d" });
  });

  it("campos inválidos viram 0 / '' / false (nunca NaN)", () => {
    const f = normalizarFunil(
      {
        total_oportunidades: "abc",
        etapas: [
          null,
          { etapa: 7, quantidade: "3", valor_estimado: Number.NaN, conversao_anterior_pct: Infinity, conversao_topo_pct: undefined },
          { etapa: "Negociação" },
        ],
        perdidas: null,
        taxa_ganho_pct: "50",
        vendedor_desligado: "true",
      },
      "all"
    );
    expect(f).toEqual({
      janela: "all",
      total_oportunidades: 0,
      etapas: [
        { etapa: "", quantidade: 0, valor_estimado: 0, conversao_anterior_pct: 0, conversao_topo_pct: 0 },
        { etapa: "", quantidade: 3, valor_estimado: 0, conversao_anterior_pct: 0, conversao_topo_pct: 0 },
        { etapa: "Negociação", quantidade: 0, valor_estimado: 0, conversao_anterior_pct: 0, conversao_topo_pct: 0 },
      ],
      perdidas: { quantidade: 0, valor_estimado: 0 },
      taxa_ganho_pct: 50,
      vendedor_desligado: false,
    });
  });

  it.each<[unknown, boolean]>([
    [true, true],
    [false, false],
    [undefined, false],
    [1, false],
    ["true", false],
  ])("vendedor_desligado %s → %s (só o booleano true liga)", (valor, esperado) => {
    expect(normalizarFunil(funilCheio({ vendedor_desligado: valor as boolean }), "all").vendedor_desligado).toBe(esperado);
  });
});

// ─── Componente ──────────────────────────────────────────────────────────────

describe("FunilVendas - carga e seletor de janela", () => {
  it("busca a janela padrão 'all' uma vez e mostra o spinner enquanto carrega", async () => {
    const d = deferred<FunilVendasDados>();
    api.apiDashboardFunil.mockReturnValue(d.promise);
    render(<FunilVendas isAdmin />);
    expect(screen.getByLabelText("Carregando funil")).toBeInTheDocument();
    expect(screen.queryByText(VAZIO)).not.toBeInTheDocument();
    await waitFor(() => expect(api.apiDashboardFunil).toHaveBeenCalledTimes(1));
    expect(api.apiDashboardFunil).toHaveBeenCalledWith("all");
    expect(botaoJanela("all")).toHaveAttribute("aria-pressed", "true");

    await act(async () => d.resolve(funilCheio()));
    expect(screen.queryByLabelText("Carregando funil")).not.toBeInTheDocument();
    expect(barras()).toHaveLength(5);
  });

  it.each<[FunilJanela]>([["30d"], ["90d"], ["365d"]])(
    "clicar em %s chama a API com o valor do enum e marca o botão",
    async (janela) => {
      await renderCarregado();
      api.apiDashboardFunil.mockResolvedValue(funilCheio({ janela, total_oportunidades: 77 }));
      await userEvent.click(botaoJanela(janela));
      await waitFor(() => expect(rodape("Oportunidades")).toBe("77"));
      expect(api.apiDashboardFunil).toHaveBeenLastCalledWith(janela);
      expect(api.apiDashboardFunil).toHaveBeenCalledTimes(2);
      for (const j of JANELAS_FUNIL) {
        expect(botaoJanela(j)).toHaveAttribute("aria-pressed", String(j === janela));
      }
    }
  );

  it("voltar para 'Tudo' busca 'all' de novo", async () => {
    await renderCarregado();
    await userEvent.click(botaoJanela("30d"));
    await userEvent.click(botaoJanela("all"));
    await waitFor(() => expect(api.apiDashboardFunil).toHaveBeenCalledTimes(3));
    expect(api.apiDashboardFunil.mock.calls.map((c) => c[0])).toEqual(["all", "30d", "all"]);
  });

  it("título por papel", async () => {
    const { unmount } = await renderCarregado({ isAdmin: true });
    expect(screen.getByRole("heading", { name: "Funil de vendas" })).toBeInTheDocument();
    unmount();
    await renderCarregado({ isAdmin: false });
    expect(screen.getByRole("heading", { name: "Meu funil de vendas" })).toBeInTheDocument();
  });

  it("recarga incrementada refaz a busca com a mesma janela", async () => {
    const { rerender } = await renderCarregado({ recarga: 0 });
    await userEvent.click(botaoJanela("90d"));
    await waitFor(() => expect(api.apiDashboardFunil).toHaveBeenCalledTimes(2));
    rerender(<FunilVendas isAdmin recarga={1} />);
    await waitFor(() => expect(api.apiDashboardFunil).toHaveBeenCalledTimes(3));
    expect(api.apiDashboardFunil).toHaveBeenLastCalledWith("90d");
    rerender(<FunilVendas isAdmin recarga={1} />);
    expect(api.apiDashboardFunil).toHaveBeenCalledTimes(3);
  });
});

describe("FunilVendas - render das etapas e do rodapé", () => {
  it("5 barras na ordem, com quantidade, valor e percentuais", async () => {
    await renderCarregado();
    const bs = barras();
    expect(bs.map((b) => b.querySelector("span")?.textContent)).toEqual([...ETAPAS_FUNIL]);
    expect(bs[0]).toHaveAccessibleName(
      /^Prospecção: 1\.000 oportunidades, R\$\s100\.000,00 estimados, 100\.0% do topo, 100\.0% da etapa anterior$/
    );
    expect(bs[3]).toHaveAccessibleName(
      /^Negociação: 200 oportunidades, R\$\s20\.000,00 estimados, 20\.0% do topo, 66\.7% da etapa anterior$/
    );
    expect(bs.map((b) => b.style.width)).toEqual(["100%", "60%", "30%", "20%", "8%"]); // 5% → mínimo 8
    // Conversão da etapa anterior não aparece na primeira barra.
    expect(screen.getAllByText(/da etapa anterior$/, { selector: "span" }).map((s) => s.textContent)).toEqual([
      "60.0% da etapa anterior",
      "50.0% da etapa anterior",
      "66.7% da etapa anterior",
      "25.0% da etapa anterior",
    ]);
    expect(screen.getByText(/^R\$\s5\.000,50$/)).toBeInTheDocument();
  });

  it("rodapé: oportunidades, taxa de ganho e perdidas", async () => {
    await renderCarregado();
    expect(rodape("Oportunidades")).toBe("1.234");
    expect(rodape("Taxa de ganho")).toBe("25.0%");
    expect(rodape("Perdidas")).toBe("150");
    expect(screen.getByText(/^R\$\s15\.000,25$/)).toBeInTheDocument();
  });

  it("percentuais fora de 0..100 são limitados na exibição", async () => {
    const f = funilCheio({ taxa_ganho_pct: 180 });
    f.etapas[1] = { ...f.etapas[1], conversao_anterior_pct: -20, conversao_topo_pct: 250 };
    api.apiDashboardFunil.mockResolvedValue(f);
    await renderCarregado();
    expect(rodape("Taxa de ganho")).toBe("100.0%");
    expect(barras()[1]).toHaveAccessibleName(/100\.0% do topo, 0\.0% da etapa anterior$/);
    expect(barras()[1].style.width).toBe("100%");
  });

  it("etapa desconhecida vira 'Outras' (rótulo e cor neutra)", async () => {
    const f = funilCheio();
    f.etapas[2] = { ...f.etapas[2], etapa: "Arquivada" };
    api.apiDashboardFunil.mockResolvedValue(f);
    await renderCarregado();
    const b = barras()[2];
    expect(b.querySelector("span")?.textContent).toBe("Outras");
    expect(b).toHaveAccessibleName(/^Outras: 300 oportunidades/);
    expect(b.className).toContain("bg-slate-300");
    expect(screen.queryByText("Arquivada")).not.toBeInTheDocument();
  });

  it("texto malicioso na etapa nunca vira HTML nem aparece", async () => {
    const payload = "<img src=x onerror=\"window.__xss=1\"><script>window.__xss=2</script>";
    const f = funilCheio();
    f.etapas[0] = { ...f.etapas[0], etapa: payload };
    api.apiDashboardFunil.mockResolvedValue(f);
    const { container } = await renderCarregado();
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("script")).toBeNull();
    expect(container.innerHTML).not.toContain("onerror");
    expect(container.textContent).not.toContain("<img");
    expect(barras()[0].querySelector("span")?.textContent).toBe("Outras");
    expect((window as unknown as { __xss?: number }).__xss).toBeUndefined();
  });
});

describe("FunilVendas - vazio e payload inválido", () => {
  it.each<[string, unknown]>([
    ["total 0", funilZerado("all")],
    ["payload null", null],
    ["payload sem etapas", { total_oportunidades: 5 }],
  ])("%s → estado vazio, sem barras nem rodapé", async (_nome, dados) => {
    api.apiDashboardFunil.mockResolvedValue(dados);
    await renderCarregado();
    expect(screen.getByText(VAZIO)).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: "Etapas do funil de vendas" })).not.toBeInTheDocument();
    expect(screen.queryByText("Taxa de ganho")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});

describe("FunilVendas - erro genérico", () => {
  it.each<[string, unknown]>([
    ["400", new ApiError(400, "janela deve ser '30d', '90d', '365d' ou 'all'")],
    ["401", new ApiError(401, "token expirado: SELECT * FROM usuarios")],
    ["500", new ApiError(500, "Error 1146: Table 'rota.oportunidades' doesn't exist")],
    ["rede", new TypeError("Failed to fetch")],
  ])("%s na primeira carga: só o alerta genérico (sem corpo, sem vazio falso)", async (_nome, erro) => {
    api.apiDashboardFunil.mockRejectedValue(erro);
    const { container } = await renderCarregado();
    expect(screen.getByRole("alert")).toHaveTextContent(ERRO_FUNIL);
    const msg = (erro as Error).message;
    expect(container.textContent).not.toContain(msg);
    expect(screen.queryByText(VAZIO)).not.toBeInTheDocument();
    expect(screen.queryByRole("list", { name: "Etapas do funil de vendas" })).not.toBeInTheDocument();
  });

  it("erro após sucesso mantém os últimos dados e o alerta some no próximo sucesso", async () => {
    await renderCarregado();
    api.apiDashboardFunil.mockRejectedValueOnce(new ApiError(500, "sql: detalhe interno"));
    await userEvent.click(botaoJanela("30d"));
    expect(await screen.findByRole("alert")).toHaveTextContent(ERRO_FUNIL);
    expect(screen.queryByText(/detalhe interno/)).not.toBeInTheDocument();
    expect(barras()).toHaveLength(5);
    expect(rodape("Oportunidades")).toBe("1.234");

    api.apiDashboardFunil.mockResolvedValueOnce(funilCheio({ total_oportunidades: 9 }));
    await userEvent.click(botaoJanela("90d"));
    await waitFor(() => expect(rodape("Oportunidades")).toBe("9"));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});

describe("FunilVendas - buscas antigas são descartadas", () => {
  it("resposta atrasada da janela anterior não sobrescreve a atual", async () => {
    const antiga = deferred<FunilVendasDados>();
    api.apiDashboardFunil.mockReturnValueOnce(antiga.promise);
    api.apiDashboardFunil.mockResolvedValueOnce(funilCheio({ janela: "30d", total_oportunidades: 30 }));
    render(<FunilVendas isAdmin />);
    await waitFor(() => expect(api.apiDashboardFunil).toHaveBeenCalledTimes(1));
    await userEvent.click(botaoJanela("30d"));
    await waitFor(() => expect(rodape("Oportunidades")).toBe("30"));

    await act(async () => antiga.resolve(funilCheio({ total_oportunidades: 999 })));
    expect(rodape("Oportunidades")).toBe("30");
  });

  it("erro atrasado da janela anterior não mostra alerta", async () => {
    const antiga = deferred<FunilVendasDados>();
    api.apiDashboardFunil.mockReturnValueOnce(antiga.promise);
    api.apiDashboardFunil.mockResolvedValueOnce(funilCheio({ total_oportunidades: 30 }));
    render(<FunilVendas isAdmin />);
    await waitFor(() => expect(api.apiDashboardFunil).toHaveBeenCalledTimes(1));
    await userEvent.click(botaoJanela("90d"));
    await waitFor(() => expect(rodape("Oportunidades")).toBe("30"));

    await act(async () => antiga.reject(new ApiError(500, "tarde demais")));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});

describe("FunilVendas - vendedor desligado (não admin)", () => {
  it("forcarZeros (decidido pela página): zerado ignorando o payload, sem o aviso próprio", async () => {
    await renderCarregado({ isAdmin: false, forcarZeros: true });
    expect(screen.getByText(VAZIO)).toBeInTheDocument();
    expect(screen.queryByText("1.234")).not.toBeInTheDocument();
    expect(screen.queryByText(AVISO_DESLIGADO)).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("forcarZeros não mostra spinner nem durante a carga", () => {
    api.apiDashboardFunil.mockReturnValue(new Promise(() => {}));
    render(<FunilVendas isAdmin={false} forcarZeros />);
    expect(screen.queryByLabelText("Carregando funil")).not.toBeInTheDocument();
    expect(screen.getByText(VAZIO)).toBeInTheDocument();
  });

  it("desligado na sessão: zerado + aviso", async () => {
    useVendedorDesligadoMock.mockReturnValue(true);
    await renderCarregado({ isAdmin: false });
    expect(screen.getByText(VAZIO)).toBeInTheDocument();
    expect(screen.queryByText("1.234")).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(AVISO_DESLIGADO);
  });

  it("vendedor_desligado no payload: zerado + aviso", async () => {
    api.apiDashboardFunil.mockResolvedValue(funilCheio({ vendedor_desligado: true }));
    await renderCarregado({ isAdmin: false });
    expect(screen.getByText(VAZIO)).toBeInTheDocument();
    expect(screen.queryByText("1.234")).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(AVISO_DESLIGADO);
  });

  it("vendedor_desligado no payload + forcarZeros: só zerado (o aviso fica com a página)", async () => {
    api.apiDashboardFunil.mockResolvedValue(funilCheio({ vendedor_desligado: true }));
    await renderCarregado({ isAdmin: false, forcarZeros: true });
    expect(screen.getByText(VAZIO)).toBeInTheDocument();
    expect(screen.queryByText(AVISO_DESLIGADO)).not.toBeInTheDocument();
  });

  it("admin ignora as três origens de desligamento", async () => {
    useVendedorDesligadoMock.mockReturnValue(true);
    api.apiDashboardFunil.mockResolvedValue(funilCheio({ vendedor_desligado: true }));
    await renderCarregado({ isAdmin: true, forcarZeros: true });
    expect(rodape("Oportunidades")).toBe("1.234");
    expect(barras()).toHaveLength(5);
    expect(screen.queryByText(AVISO_DESLIGADO)).not.toBeInTheDocument();
  });

  it("normal ativo vê os dados do próprio escopo", async () => {
    await renderCarregado({ isAdmin: false });
    expect(rodape("Oportunidades")).toBe("1.234");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
