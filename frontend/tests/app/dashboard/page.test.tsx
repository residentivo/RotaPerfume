import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  ClienteDashboardMetrics,
  DashboardMetrics,
  MeResponse,
  VendasSeries,
  VendedorRanking,
} from "@/lib/types";

// ─── Mocks ────────────────────────────────────────────────────────────────────

vi.mock("@/components/layout/ProtectedRoute", () => ({
  ProtectedRoute: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/layout/Navbar", () => ({ Navbar: () => <nav data-testid="navbar" /> }));

const useSessionUserMock = vi.fn<() => MeResponse | null>();
const useVendedorDesligadoMock = vi.fn<() => boolean>();
vi.mock("@/lib/session", () => ({
  useSessionUser: () => useSessionUserMock(),
  useVendedorDesligado: () => useVendedorDesligadoMock(),
}));

const api = vi.hoisted(() => ({
  apiDashboardMetrics: vi.fn(),
  apiDashboardVendas: vi.fn(),
  apiDashboardVendedores: vi.fn(),
  apiDashboardClientes: vi.fn(),
  apiDashboardFunil: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import DashboardPage from "@/app/dashboard/page";

// ─── Fixtures ─────────────────────────────────────────────────────────────────

const AVISO_SEM_VENDEDOR = /Usuario sem vendedor vinculado/;
const AVISO_DESLIGADO = /Vendedor desligado\. O vendedor vinculado/;
const ZERO_BRL = /^R\$\s0,00$/;

function user(extra: Partial<MeResponse>): MeResponse {
  return { id: 1, nome: "Ana", email: "a@x", role: "normal", ativo: true, ...extra };
}

function zeroMetrics(extra: Partial<DashboardMetrics> = {}): DashboardMetrics {
  return {
    total_vendas: 0,
    total_pedidos: 0,
    ticket_medio: 0,
    total_clientes: 0,
    atingimento_meta: 0,
    periodo: "month",
    ...extra,
  } as DashboardMetrics;
}

const zeroClientes = {
  total_clientes: 0,
  total_ativos: 0,
  total_inativos: 0,
  novos_no_periodo: 0,
  por_segmento: [],
  por_uf: [],
} as unknown as ClienteDashboardMetrics;

const cheioMetrics = zeroMetrics({
  total_vendas: 1500.5,
  total_pedidos: 12,
  ticket_medio: 125.04,
  meta_mes: 3000,
});

const cheioVendas: VendasSeries = {
  dias: 30,
  pontos: [
    { dia: "2026-09-01", total_vendas: 1000, total_pedidos: 8 },
    { dia: "2026-09-02", total_vendas: 500.5, total_pedidos: 4 },
  ],
} as VendasSeries;

const ranking = [
  {
    vendedor_id: 7,
    vendedor_nome: "Vend 7",
    total_vendas: 1500.5,
    total_pedidos: 12,
    ticket_medio: 125.04,
    meta: 3000,
    atingimento_meta: 50,
  },
] as VendedorRanking[];

function mockApiZerada(metrics: DashboardMetrics = zeroMetrics()) {
  api.apiDashboardMetrics.mockResolvedValue(metrics);
  api.apiDashboardVendas.mockImplementation(async (dias: number) => ({ dias, pontos: [] }));
  api.apiDashboardVendedores.mockResolvedValue({ data: [], page: 1, limit: 10, total: 0, pages: 0 });
  api.apiDashboardClientes.mockResolvedValue(zeroClientes);
}

function mockApiCheia() {
  api.apiDashboardMetrics.mockResolvedValue(cheioMetrics);
  api.apiDashboardVendas.mockImplementation(async (dias: number) => ({ ...cheioVendas, dias }));
  api.apiDashboardVendedores.mockResolvedValue({
    data: ranking,
    page: 1,
    limit: 10,
    total: 1,
    pages: 1,
  });
  api.apiDashboardClientes.mockResolvedValue({ ...zeroClientes, total_clientes: 5 });
}

/** Valor exibido no KpiCard cujo rotulo e `label`. */
function kpiValue(label: string): string {
  const el = screen.getByText(label, { selector: "p" });
  return el.nextElementSibling?.textContent ?? "";
}

async function renderLoaded() {
  render(<DashboardPage />);
  await waitFor(() => expect(screen.queryByText("Atualizando dados...")).not.toBeInTheDocument());
}

beforeEach(() => {
  vi.clearAllMocks();
  useVendedorDesligadoMock.mockReturnValue(false);
});

// ─── UI-01: titulo dinamico do grafico ────────────────────────────────────────

describe("Dashboard - titulo do grafico (UI-01)", () => {
  it.each<[MeResponse["role"], string]>([
    ["admin", "Vendas nos Últimos"],
    ["normal", "Minhas Vendas nos Últimos"],
  ])("%s: padrao 30 dias", async (role, prefixo) => {
    useSessionUserMock.mockReturnValue(user({ role, id_vendedor: 7 }));
    mockApiCheia();
    await renderLoaded();
    expect(screen.getByRole("heading", { name: `${prefixo} 30 Dias` })).toBeInTheDocument();
    expect(api.apiDashboardVendas).toHaveBeenCalledWith(30);
  });

  it.each([7, 14, 60, 30])(
    "admin: select em %i dias atualiza o titulo e a busca",
    async (dias) => {
      useSessionUserMock.mockReturnValue(user({ role: "admin" }));
      mockApiCheia();
      await renderLoaded();

      await userEvent.selectOptions(screen.getByRole("combobox"), String(dias));

      expect(
        await screen.findByRole("heading", { name: `Vendas nos Últimos ${dias} Dias` })
      ).toBeInTheDocument();
      expect(api.apiDashboardVendas).toHaveBeenLastCalledWith(dias);
      expect(screen.getAllByRole("heading", { name: /Vendas nos Últimos \d+ Dias/ })).toHaveLength(1);
    }
  );

  it("normal: select em 7 dias atualiza o titulo 'Minhas Vendas'", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    mockApiCheia();
    await renderLoaded();
    await userEvent.selectOptions(screen.getByRole("combobox"), "7");
    expect(
      await screen.findByRole("heading", { name: "Minhas Vendas nos Últimos 7 Dias" })
    ).toBeInTheDocument();
  });
});

// ─── UI-02 opcao (b): aviso + KPIs/grafico zerados ────────────────────────────

describe("Dashboard - aviso + zerados (UI-02 opcao b)", () => {
  it.each<[string, Partial<MeResponse>, boolean, Partial<DashboardMetrics>, RegExp]>([
    ["normal sem vendedor", { id_vendedor: null }, false, {}, AVISO_SEM_VENDEDOR],
    ["normal sem o campo id_vendedor", {}, false, {}, AVISO_SEM_VENDEDOR],
    ["desligado via sessao (/me ou 403)", { id_vendedor: 7 }, true, {}, AVISO_DESLIGADO],
    [
      "desligado via /api/dashboard/metrics",
      { id_vendedor: 7 },
      false,
      { vendedor_desligado: true },
      AVISO_DESLIGADO,
    ],
  ])("%s: mostra aviso E KPIs/grafico zerados", async (_n, u, desligado, m, aviso) => {
    useSessionUserMock.mockReturnValue(user(u));
    useVendedorDesligadoMock.mockReturnValue(desligado);
    mockApiZerada(zeroMetrics(m));
    await renderLoaded();

    // Aviso
    const alerta = screen.getByText(aviso);
    expect(alerta).toBeInTheDocument();
    const outro = aviso === AVISO_DESLIGADO ? AVISO_SEM_VENDEDOR : AVISO_DESLIGADO;
    expect(screen.queryByText(outro)).not.toBeInTheDocument();

    // KPIs visiveis e zerados
    expect(kpiValue("Minhas Vendas")).toMatch(ZERO_BRL);
    expect(kpiValue("Meus Pedidos")).toBe("0");
    expect(kpiValue("Meu Ticket Medio")).toMatch(ZERO_BRL);
    expect(kpiValue("Minha Meta")).toBe("-");
    expect(kpiValue("Meus Clientes")).toBe("0");
    expect(kpiValue("Meus Clientes Ativos")).toBe("0");

    // Grafico visivel e zerado
    const titulo = screen.getByRole("heading", { name: "Minhas Vendas nos Últimos 30 Dias" });
    const card = titulo.closest("div.rounded-xl") as HTMLElement;
    expect(within(card).getByText(/^Total: R\$\s0,00$/)).toBeInTheDocument();
    expect(within(card).getByText("Nenhum dado de vendas no periodo")).toBeInTheDocument();

    // Meu desempenho sem dados
    expect(screen.getByText("Nenhum dado de desempenho encontrado")).toBeInTheDocument();
  });

  it.each<[string, Partial<MeResponse>, boolean, boolean, RegExp]>([
    ["sem vendedor", { id_vendedor: null }, false, false, AVISO_SEM_VENDEDOR],
    ["desligado via sessao", { id_vendedor: 7 }, true, false, AVISO_DESLIGADO],
    ["desligado via metrics", { id_vendedor: 7 }, false, true, AVISO_DESLIGADO],
  ])(
    "FE-02 %s: ignora payload preenchido da API e forca tudo zerado",
    async (_n, u, desligadoSessao, desligadoMetrics, aviso) => {
      useSessionUserMock.mockReturnValue(user(u));
      useVendedorDesligadoMock.mockReturnValue(desligadoSessao);
      mockApiCheia();
      api.apiDashboardMetrics.mockResolvedValue({
        ...cheioMetrics,
        vendedor_desligado: desligadoMetrics,
      });
      api.apiDashboardClientes.mockResolvedValue({
        ...zeroClientes,
        total_clientes: 5,
        total_ativos: 4,
        total_inativos: 1,
        novos_no_periodo: 2,
        por_segmento: [{ segmento: "Varejo", total: 5 }],
        por_uf: [{ uf: "SP", total: 5 }],
      });
      await renderLoaded();

      expect(screen.getByText(aviso)).toBeInTheDocument();
      expect(kpiValue("Minhas Vendas")).toMatch(ZERO_BRL);
      expect(kpiValue("Meus Pedidos")).toBe("0");
      expect(kpiValue("Meu Ticket Medio")).toMatch(ZERO_BRL);
      expect(kpiValue("Minha Meta")).toBe("-");
      expect(kpiValue("Meus Clientes")).toBe("0");
      expect(kpiValue("Meus Clientes Ativos")).toBe("0");
      expect(kpiValue("Meus Clientes Inativos")).toBe("0");
      expect(kpiValue("Meus Novos no Periodo")).toBe("0");
      expect(screen.getByText(/^Total: R\$\s0,00$/)).toBeInTheDocument();
      expect(screen.getByText("Nenhum dado de vendas no periodo")).toBeInTheDocument();
      expect(screen.getByText("Nenhum dado de desempenho encontrado")).toBeInTheDocument();
      expect(screen.queryByText("7 - Vend 7")).not.toBeInTheDocument();
      expect(screen.queryByText("Varejo")).not.toBeInTheDocument();
      expect(screen.queryByText("SP")).not.toBeInTheDocument();
    }
  );

  it("aviso de sem vendedor tem prioridade sobre desligado", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: null }));
    useVendedorDesligadoMock.mockReturnValue(true);
    mockApiZerada();
    await renderLoaded();
    expect(screen.getByText(AVISO_SEM_VENDEDOR)).toBeInTheDocument();
    expect(screen.queryByText(AVISO_DESLIGADO)).not.toBeInTheDocument();
  });

  it.each<[string, Partial<MeResponse>, boolean, Partial<DashboardMetrics>]>([
    ["normal ativo com vendedor", { id_vendedor: 7 }, false, { vendedor_desligado: false }],
    ["admin sem vendedor", { role: "admin", id_vendedor: null }, false, {}],
    ["admin com flag desligado", { role: "admin" }, true, { vendedor_desligado: true }],
  ])("%s: sem aviso", async (_n, u, desligado, m) => {
    useSessionUserMock.mockReturnValue(user(u));
    useVendedorDesligadoMock.mockReturnValue(desligado);
    mockApiZerada(zeroMetrics(m));
    await renderLoaded();
    expect(screen.queryByText(AVISO_SEM_VENDEDOR)).not.toBeInTheDocument();
    expect(screen.queryByText(AVISO_DESLIGADO)).not.toBeInTheDocument();
  });
});

// ─── Dados preenchidos / erro ─────────────────────────────────────────────────

describe("Dashboard - dados", () => {
  it("normal ativo: exibe KPIs, grafico e meu desempenho com dados da API", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    mockApiCheia();
    await renderLoaded();

    expect(kpiValue("Minhas Vendas")).toMatch(/^R\$\s1\.500,50$/);
    expect(kpiValue("Meus Pedidos")).toBe("12");
    expect(kpiValue("Minha Meta")).toMatch(/^R\$\s3\.000,00$/);
    expect(screen.getByText(/^Total: R\$\s1\.500,50$/)).toBeInTheDocument();
    expect(screen.getByText("7 - Vend 7")).toBeInTheDocument();
    expect(screen.getByText("Clientes da minha carteira")).toBeInTheDocument();
  });

  it("admin: exibe ranking e rotulos globais", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiCheia();
    await renderLoaded();

    expect(kpiValue("Total de Vendas")).toMatch(/^R\$\s1\.500,50$/);
    expect(screen.getByText("Lider: Vend 7")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Ranking de Vendedores" })).toBeInTheDocument();
    expect(screen.queryByText("Clientes da minha carteira")).not.toBeInTheDocument();
  });

  it.each<[string, "today" | "week" | "month"]>([
    ["Hoje", "today"],
    ["Semana", "week"],
  ])("filtro de periodo %s refaz a busca", async (label, periodo) => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiCheia();
    await renderLoaded();
    await userEvent.click(screen.getByRole("button", { name: label }));
    await waitFor(() => expect(api.apiDashboardMetrics).toHaveBeenLastCalledWith(periodo));
    expect(api.apiDashboardClientes).toHaveBeenLastCalledWith(periodo);
  });

  it("erro da API mostra alerta de erro e KPIs zerados", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiZerada();
    api.apiDashboardMetrics.mockRejectedValue(new Error("falha ao carregar"));
    await renderLoaded();
    expect(screen.getByText("falha ao carregar")).toBeInTheDocument();
    expect(kpiValue("Total de Vendas")).toMatch(ZERO_BRL);
  });
});

// ─── FE-02: zeros forcados so para normal; loading/erro derivados da chave ────

describe("Dashboard - FE-02 (zeros forcados e respostas obsoletas)", () => {
  it.each<[string, boolean, boolean]>([
    ["admin com vendedor_desligado no metrics", false, true],
    ["admin com flag de desligado na sessao", true, false],
  ])("%s: nunca forca zeros", async (_n, desligadoSessao, desligadoMetrics) => {
    useSessionUserMock.mockReturnValue(user({ role: "admin", id_vendedor: null }));
    useVendedorDesligadoMock.mockReturnValue(desligadoSessao);
    mockApiCheia();
    api.apiDashboardMetrics.mockResolvedValue({
      ...cheioMetrics,
      vendedor_desligado: desligadoMetrics,
    });
    await renderLoaded();
    expect(kpiValue("Total de Vendas")).toMatch(/^R\$\s1\.500,50$/);
    expect(screen.getByText("Lider: Vend 7")).toBeInTheDocument();
    expect(screen.queryByText(AVISO_DESLIGADO)).not.toBeInTheDocument();
    expect(screen.queryByText(AVISO_SEM_VENDEDOR)).not.toBeInTheDocument();
  });

  it("normal com vendedor e payload cheio: nao forca zeros (controle)", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    mockApiCheia();
    await renderLoaded();
    expect(kpiValue("Minhas Vendas")).toMatch(/^R\$\s1\.500,50$/);
    expect(kpiValue("Meus Clientes")).toBe("5");
  });

  it("vendedor desligado detectado depois (403 na sessao) zera a tela ja carregada", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    mockApiCheia();
    const { rerender } = render(<DashboardPage />);
    await waitFor(() => expect(kpiValue("Minhas Vendas")).toMatch(/^R\$\s1\.500,50$/));

    useVendedorDesligadoMock.mockReturnValue(true);
    rerender(<DashboardPage />);
    expect(kpiValue("Minhas Vendas")).toMatch(ZERO_BRL);
    expect(kpiValue("Meus Clientes")).toBe("0");
    expect(screen.getByText(AVISO_DESLIGADO)).toBeInTheDocument();
  });

  it("resposta obsoleta (periodo anterior chega depois) e descartada", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiCheia();
    let resolveMes: (m: DashboardMetrics) => void = () => {};
    api.apiDashboardMetrics
      .mockReturnValueOnce(new Promise((r) => (resolveMes = r)))
      .mockResolvedValueOnce(zeroMetrics({ total_vendas: 42, periodo: "today" }));

    render(<DashboardPage />);
    await userEvent.click(screen.getByRole("button", { name: "Hoje" }));
    await waitFor(() => expect(kpiValue("Total de Vendas")).toMatch(/^R\$\s42,00$/));
    await waitFor(() =>
      expect(screen.queryByText("Atualizando dados...")).not.toBeInTheDocument()
    );

    await act(async () => resolveMes(cheioMetrics));
    expect(kpiValue("Total de Vendas")).toMatch(/^R\$\s42,00$/);
    expect(screen.queryByText("Atualizando dados...")).not.toBeInTheDocument();
  });

  it("loading aparece durante a busca e some ao terminar", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiCheia();
    let resolver: (m: DashboardMetrics) => void = () => {};
    api.apiDashboardMetrics.mockReturnValueOnce(new Promise((r) => (resolver = r)));
    render(<DashboardPage />);
    expect(screen.getByText("Atualizando dados...")).toBeInTheDocument();
    await act(async () => resolver(cheioMetrics));
    await waitFor(() =>
      expect(screen.queryByText("Atualizando dados...")).not.toBeInTheDocument()
    );
  });

  it("'Atualizar' refaz a busca e um erro anterior some apos sucesso", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiCheia();
    api.apiDashboardMetrics.mockRejectedValueOnce(new Error("falha temporaria"));
    await renderLoaded();
    expect(screen.getByText("falha temporaria")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Atualizar" }));
    await waitFor(() => expect(kpiValue("Total de Vendas")).toMatch(/^R\$\s1\.500,50$/));
    expect(screen.queryByText("falha temporaria")).not.toBeInTheDocument();
    expect(api.apiDashboardMetrics).toHaveBeenCalledTimes(2);
  });
});

// ─── FE-09: erro de carga nao mostra estados vazios falsos ────────────────────

describe("Dashboard - FE-09 (erro de carga)", () => {
  const VAZIOS_ADMIN = [
    "Nenhum dado de vendas no periodo",
    "Nenhum vendedor encontrado",
    "Nenhuma meta de vendas cadastrada",
    "Sem dados disponiveis",
  ];

  it("admin: erro de rede na primeira carga mostra so o alerta, sem estados vazios", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiCheia();
    api.apiDashboardMetrics.mockRejectedValue(new TypeError("Failed to fetch"));
    await renderLoaded();
    expect(screen.getByRole("alert")).toHaveTextContent("Failed to fetch");
    for (const vazio of VAZIOS_ADMIN) {
      expect(screen.queryByText(vazio)).not.toBeInTheDocument();
    }
    expect(screen.queryByText("Carregando...")).not.toBeInTheDocument();
  });

  it("normal com vendedor: erro na primeira carga nao mostra 'Nenhum dado de desempenho'", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    mockApiCheia();
    api.apiDashboardMetrics.mockRejectedValue(new TypeError("Failed to fetch"));
    await renderLoaded();
    expect(screen.getByText("Failed to fetch")).toBeInTheDocument();
    expect(screen.queryByText("Nenhum dado de desempenho encontrado")).not.toBeInTheDocument();
    expect(screen.queryByText("Nenhum dado de vendas no periodo")).not.toBeInTheDocument();
    expect(screen.queryByText("Nenhuma meta cadastrada para voce")).not.toBeInTheDocument();
  });

  it("admin: erro apos carga bem-sucedida mantem os dados anteriores + alerta", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiCheia();
    await renderLoaded();
    expect(screen.getByText("Lider: Vend 7")).toBeInTheDocument();

    api.apiDashboardMetrics.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    await userEvent.click(screen.getByRole("button", { name: "Atualizar" }));
    expect(await screen.findByText("Failed to fetch")).toBeInTheDocument();
    expect(kpiValue("Total de Vendas")).toMatch(/^R\$\s1\.500,50$/);
    expect(screen.getByText("Lider: Vend 7")).toBeInTheDocument();
    expect(screen.getByText(/^Total: R\$\s1\.500,50$/)).toBeInTheDocument();
    expect(screen.queryByText("Nenhum vendedor encontrado")).not.toBeInTheDocument();
  });

  it("admin: sucesso com dados vazios mostra os estados vazios normais, sem alerta", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiZerada();
    await renderLoaded();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByText("Nenhum dado de vendas no periodo")).toBeInTheDocument();
    expect(screen.getByText("Nenhum vendedor encontrado")).toBeInTheDocument();
    expect(screen.getByText("Nenhuma meta de vendas cadastrada")).toBeInTheDocument();
    expect(screen.getAllByText("Sem dados disponiveis")).toHaveLength(2);
  });

  it("normal sem vendedor (zeros forcados): erro nao esconde os estados vazios", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: null }));
    mockApiZerada();
    api.apiDashboardMetrics.mockRejectedValue(new TypeError("Failed to fetch"));
    await renderLoaded();
    expect(screen.getByText("Failed to fetch")).toBeInTheDocument();
    expect(screen.getByText("Nenhum dado de vendas no periodo")).toBeInTheDocument();
    expect(screen.getByText("Nenhum dado de desempenho encontrado")).toBeInTheDocument();
  });
});

// ─── DASH-01: funil de vendas integrado à página ──────────────────────────────

describe("Dashboard - funil de vendas (DASH-01)", () => {
  const AVISO_DESLIGADO_FUNIL = /Vendedor desligado\. Nao ha oportunidades/;
  const funil = {
    janela: "all",
    total_oportunidades: 4321,
    etapas: [
      { etapa: "Prospecção", quantidade: 10, valor_estimado: 100, conversao_anterior_pct: 100, conversao_topo_pct: 100 },
      { etapa: "Qualificação", quantidade: 5, valor_estimado: 50, conversao_anterior_pct: 50, conversao_topo_pct: 50 },
      { etapa: "Proposta enviada", quantidade: 4, valor_estimado: 40, conversao_anterior_pct: 80, conversao_topo_pct: 40 },
      { etapa: "Negociação", quantidade: 2, valor_estimado: 20, conversao_anterior_pct: 50, conversao_topo_pct: 20 },
      { etapa: "Fechado ganho", quantidade: 1, valor_estimado: 10, conversao_anterior_pct: 50, conversao_topo_pct: 10 },
    ],
    perdidas: { quantidade: 3, valor_estimado: 30 },
    taxa_ganho_pct: 25,
    vendedor_desligado: false,
  };

  function totalFunil(): string | null {
    return screen.queryByText("Oportunidades", { selector: "dt" })?.nextElementSibling?.textContent ?? null;
  }

  it.each<[MeResponse["role"], string]>([
    ["admin", "Funil de vendas"],
    ["normal", "Meu funil de vendas"],
  ])("%s: seção aparece com os dados e busca a janela 'all'", async (role, titulo) => {
    useSessionUserMock.mockReturnValue(user({ role, id_vendedor: 7 }));
    mockApiCheia();
    api.apiDashboardFunil.mockResolvedValue(funil);
    await renderLoaded();
    expect(screen.getByRole("heading", { name: titulo })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Janela do funil" })).toBeInTheDocument();
    await waitFor(() => expect(totalFunil()).toBe("4.321"));
    expect(api.apiDashboardFunil).toHaveBeenCalledWith("all");
  });

  it("'Atualizar' recarrega o funil na janela selecionada", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiCheia();
    api.apiDashboardFunil.mockResolvedValue(funil);
    await renderLoaded();
    const grupo = screen.getByRole("group", { name: "Janela do funil" });
    await userEvent.click(within(grupo).getByRole("button", { name: "90 dias" }));
    await waitFor(() => expect(api.apiDashboardFunil).toHaveBeenCalledTimes(2));

    api.apiDashboardFunil.mockResolvedValue({ ...funil, total_oportunidades: 8 });
    await userEvent.click(screen.getByRole("button", { name: "Atualizar" }));
    await waitFor(() => expect(totalFunil()).toBe("8"));
    expect(api.apiDashboardFunil).toHaveBeenCalledTimes(3);
    expect(api.apiDashboardFunil).toHaveBeenLastCalledWith("90d");
  });

  it("trocar a janela do funil não recarrega os demais blocos", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiCheia();
    api.apiDashboardFunil.mockResolvedValue(funil);
    await renderLoaded();
    const antes = api.apiDashboardMetrics.mock.calls.length;
    await userEvent.click(screen.getByRole("button", { name: "30 dias" }));
    await waitFor(() => expect(api.apiDashboardFunil).toHaveBeenLastCalledWith("30d"));
    expect(api.apiDashboardMetrics).toHaveBeenCalledTimes(antes);
  });

  it.each<[string, Partial<MeResponse>, boolean]>([
    ["sem vendedor vinculado", {}, false],
    ["desligado na sessão", { id_vendedor: 7 }, true],
  ])("normal %s: funil zerado ignorando o payload, sem o aviso próprio do funil", async (_nome, extra, desligado) => {
    useSessionUserMock.mockReturnValue(user(extra));
    useVendedorDesligadoMock.mockReturnValue(desligado);
    mockApiZerada();
    api.apiDashboardFunil.mockResolvedValue(funil);
    await renderLoaded();
    await waitFor(() => expect(screen.getByText("Nenhuma oportunidade na janela selecionada")).toBeInTheDocument());
    expect(screen.queryByText("4.321")).not.toBeInTheDocument();
    expect(screen.queryByText(AVISO_DESLIGADO_FUNIL)).not.toBeInTheDocument();
  });

  it("desligado vindo das métricas também zera o funil", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    mockApiZerada(zeroMetrics({ vendedor_desligado: true } as Partial<DashboardMetrics>));
    api.apiDashboardFunil.mockResolvedValue(funil);
    await renderLoaded();
    expect(screen.getByText(AVISO_DESLIGADO)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("Nenhuma oportunidade na janela selecionada")).toBeInTheDocument());
    expect(screen.queryByText(AVISO_DESLIGADO_FUNIL)).not.toBeInTheDocument();
  });

  it("erro do funil mostra só o alerta genérico e não afeta os KPIs", async () => {
    useSessionUserMock.mockReturnValue(user({ role: "admin" }));
    mockApiCheia();
    api.apiDashboardFunil.mockRejectedValue(new Error("sql: Table 'oportunidades' doesn't exist"));
    await renderLoaded();
    expect(await screen.findByText("Nao foi possivel carregar o funil de vendas. Tente novamente.")).toBeInTheDocument();
    expect(screen.queryByText(/doesn't exist/)).not.toBeInTheDocument();
    expect(kpiValue("Total de Vendas")).toMatch(/^R\$\s1\.500,50$/);
  });
});
