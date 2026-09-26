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
}));
vi.mock("@/lib/api", () => api);

import DashboardPage from "./page";

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
