/**
 * Regressao do FE-03 nos modais com cascata vendedor -> cliente
 * (OportunidadeModal e VisitaModal). Os dois compartilham o mesmo contrato,
 * entao o roteiro roda parametrizado sobre ambos. O PedidoModal tem roteiro
 * proprio (PedidoModal.test.tsx), porque trava o vendedor pela sessao.
 */
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentType } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ClienteResumo, Oportunidade, Vendedor, Visita } from "@/lib/types";

const api = vi.hoisted(() => ({
  apiListVendedores: vi.fn(),
  apiListClientesDoVendedor: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import { OportunidadeModal } from "./OportunidadeModal";
import { VisitaModal } from "./VisitaModal";

// ─── Fixtures ─────────────────────────────────────────────────────────────────

const vendedores: Vendedor[] = [
  { id: 3, nome: "Vend 3", regiao: "S", uf: "SP", data_desligamento: null },
  { id: 9, nome: "Vend 9", regiao: "S", uf: "SP", data_desligamento: "2026-01-01" },
];

function cliente(id: number): ClienteResumo {
  return {
    id,
    cnpj: "00",
    razao_social: `Cliente ${id}`,
    segmento: "",
    cidade: "",
    uf: "SP",
    carteira_id: id,
    data_inicio: "2026-01-01",
    data_fim: null,
  };
}

const oportunidade: Oportunidade = {
  oportunidade_id: 1,
  cliente_id: 301,
  vendedor_id: 3,
  origem: "Instagram",
  data_abertura: "2026-08-01",
  etapa: "Negociação",
  probabilidade_pct: 60,
  valor_estimado: 1500,
  data_fechamento: "2026-09-10",
  ciclo_dias: 40,
  motivo_perda: null,
  created_at: "",
  updated_at: "",
};

const visita: Visita = {
  visita_id: 1,
  cliente_id: 301,
  vendedor_id: 3,
  data_visita: "2026-08-15",
  resultado: "Reagendada",
  duracao_min: 45,
  created_at: "",
  updated_at: "",
};

type Props = {
  open: boolean;
  mode: "create" | "edit";
  registro: unknown;
  onClose: () => void;
  onSubmit: (d: unknown) => Promise<void>;
};

interface CasoModal {
  nome: string;
  Modal: ComponentType<Props>;
  registro: unknown;
  tituloNovo: string;
  tituloEditar: string;
  botaoCriar: string;
  /** Campo de texto livre para provar que o form foi resetado. */
  campoLivre: string;
  /** Campos preenchidos no modo editar: label -> valor esperado. */
  camposEditar: Record<string, string>;
  /** Campos no modo novo: label -> valor esperado. */
  camposNovo: Record<string, string>;
}

const hoje = new Date().toISOString().slice(0, 10);

const CASOS: CasoModal[] = [
  {
    nome: "OportunidadeModal",
    Modal: ({ registro, ...p }) => (
      <OportunidadeModal {...p} oportunidade={registro as Oportunidade | null} />
    ),
    registro: oportunidade,
    tituloNovo: "Nova Oportunidade",
    tituloEditar: "Editar Oportunidade",
    botaoCriar: "Criar oportunidade",
    campoLivre: "Valor estimado",
    camposEditar: {
      Origem: "Instagram",
      Etapa: "Negociação",
      "Data de abertura": "2026-08-01",
      "Data de fechamento": "2026-09-10",
      "Probabilidade (%)": "60",
      "Valor estimado": "1500",
      "Ciclo (dias)": "40",
    },
    camposNovo: {
      Origem: "WhatsApp",
      Etapa: "Prospecção",
      "Data de abertura": hoje,
      "Data de fechamento": "",
      "Probabilidade (%)": "",
      "Valor estimado": "",
      "Ciclo (dias)": "",
    },
  },
  {
    nome: "VisitaModal",
    Modal: ({ registro, ...p }) => <VisitaModal {...p} visita={registro as Visita | null} />,
    registro: visita,
    tituloNovo: "Nova Visita",
    tituloEditar: "Editar Visita",
    botaoCriar: "Criar visita",
    campoLivre: "Duracao (min)",
    camposEditar: {
      "Data da visita": "2026-08-15",
      Resultado: "Reagendada",
      "Duracao (min)": "45",
    },
    camposNovo: {
      "Data da visita": hoje,
      Resultado: "Sem pedido",
      "Duracao (min)": "",
    },
  },
];

// ─── Helpers ──────────────────────────────────────────────────────────────────

function sel(label: string): HTMLSelectElement {
  return screen.getByLabelText(label) as HTMLSelectElement;
}

function valor(label: string): string {
  return (screen.getByLabelText(label) as HTMLInputElement | HTMLSelectElement).value;
}

function deferred<T>() {
  let resolve: (v: T) => void = () => {};
  let reject: (e: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function montar(c: CasoModal, props: Partial<Props> = {}) {
  const base: Props = {
    open: true,
    mode: "create",
    registro: null,
    onClose: vi.fn(),
    onSubmit: vi.fn().mockResolvedValue(undefined),
    ...props,
  };
  const r = render(<c.Modal {...base} />);
  return {
    ...r,
    props: base,
    set: (p: Partial<Props>) => {
      Object.assign(base, p);
      r.rerender(<c.Modal {...base} />);
    },
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  Object.values(api).forEach((f) => f.mockReset());
  api.apiListVendedores.mockResolvedValue(vendedores);
  api.apiListClientesDoVendedor.mockImplementation(async (id: number) => [
    cliente(id * 100 + 1),
    cliente(id * 100 + 2),
  ]);
});

// ─── Roteiro ──────────────────────────────────────────────────────────────────

describe.each(CASOS)("$nome (FE-03)", (c) => {
  it("abre em modo novo resetado: sem vendedor, cliente desabilitado, sem cascata", async () => {
    montar(c);
    expect(screen.getByText(c.tituloNovo)).toBeInTheDocument();
    // Vendedores carregando: select e submit desabilitados.
    expect(sel("Vendedor")).toBeDisabled();
    expect(screen.getByRole("button", { name: c.botaoCriar })).toBeDisabled();
    await waitFor(() => expect(sel("Vendedor")).toBeEnabled());
    expect(screen.getByRole("option", { name: "#9 - Vend 9 [X]" })).toBeInTheDocument();

    expect(sel("Vendedor").value).toBe("");
    expect(sel("Cliente")).toBeDisabled();
    expect(screen.getByRole("option", { name: "Selecione um vendedor primeiro" })).toBeInTheDocument();
    for (const [label, v] of Object.entries(c.camposNovo)) expect(valor(label)).toBe(v);
    expect(api.apiListClientesDoVendedor).not.toHaveBeenCalled();
  });

  it("abre em modo editar preenchido e carrega os clientes do vendedor do registro", async () => {
    montar(c, { mode: "edit", registro: c.registro });
    expect(screen.getByText(c.tituloEditar)).toBeInTheDocument();
    for (const [label, v] of Object.entries(c.camposEditar)) expect(valor(label)).toBe(v);

    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    await waitFor(() => expect(sel("Vendedor").value).toBe("3"));
    expect(api.apiListClientesDoVendedor).toHaveBeenCalledWith(3);
    expect(sel("Cliente").value).toBe("301");
    expect(sel("Cliente")).toBeEnabled();
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeInTheDocument();
  });

  it("reabre resetado depois de fechar (novo -> digitar -> fechar -> novo)", async () => {
    const m = montar(c);
    await waitFor(() => expect(sel("Vendedor")).toBeEnabled());
    await userEvent.selectOptions(sel("Vendedor"), "3");
    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    await userEvent.selectOptions(sel("Cliente"), "301");
    await userEvent.type(screen.getByLabelText(c.campoLivre), "77");

    m.set({ open: false });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    m.set({ open: true });

    expect(sel("Vendedor").value).toBe("");
    expect(sel("Cliente").value).toBe("");
    expect(sel("Cliente")).toBeDisabled();
    expect(valor(c.campoLivre)).toBe("");
    // Recarrega vendedores a cada abertura.
    await waitFor(() => expect(api.apiListVendedores).toHaveBeenCalledTimes(2));
  });

  it("editar -> fechar -> novo: nada do registro anterior sobra", async () => {
    const m = montar(c, { mode: "edit", registro: c.registro });
    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    m.set({ open: false });
    m.set({ open: true, mode: "create", registro: null });

    expect(screen.getByText(c.tituloNovo)).toBeInTheDocument();
    for (const [label, v] of Object.entries(c.camposNovo)) expect(valor(label)).toBe(v);
    expect(sel("Vendedor").value).toBe("");
    expect(screen.queryByRole("option", { name: "#301 - Cliente 301" })).not.toBeInTheDocument();
  });

  it("trocar o registro com o modal aberto re-preenche o formulario", async () => {
    const m = montar(c, { mode: "edit", registro: c.registro });
    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    const outro = { ...(c.registro as object), vendedor_id: 9, cliente_id: 902 };
    m.set({ registro: outro });
    expect(sel("Vendedor").value).toBe("9");
    await screen.findByRole("option", { name: "#902 - Cliente 902" });
    expect(sel("Cliente").value).toBe("902");
  });

  it("cascata: mostra 'Carregando clientes...' e libera o select ao terminar", async () => {
    montar(c);
    await waitFor(() => expect(sel("Vendedor")).toBeEnabled());
    const d = deferred<ClienteResumo[]>();
    api.apiListClientesDoVendedor.mockReturnValueOnce(d.promise);

    await userEvent.selectOptions(sel("Vendedor"), "3");
    expect(screen.getByRole("option", { name: "Carregando clientes..." })).toBeInTheDocument();
    expect(sel("Cliente")).toBeDisabled();

    await act(async () => d.resolve([cliente(301)]));
    expect(await screen.findByRole("option", { name: "#301 - Cliente 301" })).toBeInTheDocument();
    expect(sel("Cliente")).toBeEnabled();
  });

  it("cascata: limpar o vendedor com a busca em voo nao deixa o loading travado", async () => {
    montar(c);
    await waitFor(() => expect(sel("Vendedor")).toBeEnabled());
    const d = deferred<ClienteResumo[]>();
    api.apiListClientesDoVendedor.mockReturnValueOnce(d.promise);

    await userEvent.selectOptions(sel("Vendedor"), "3");
    expect(screen.getByRole("option", { name: "Carregando clientes..." })).toBeInTheDocument();
    await userEvent.selectOptions(sel("Vendedor"), "");

    expect(screen.getByRole("option", { name: "Selecione um vendedor primeiro" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Carregando clientes..." })).not.toBeInTheDocument();

    // A resposta tardia do vendedor 3 e descartada.
    await act(async () => d.resolve([cliente(301)]));
    expect(screen.queryByRole("option", { name: "#301 - Cliente 301" })).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Selecione um vendedor primeiro" })).toBeInTheDocument();
  });

  it("cascata: resposta obsoleta (vendedor 3 chega depois do 9) nao sobrescreve", async () => {
    montar(c);
    await waitFor(() => expect(sel("Vendedor")).toBeEnabled());
    const d3 = deferred<ClienteResumo[]>();
    const d9 = deferred<ClienteResumo[]>();
    api.apiListClientesDoVendedor.mockReturnValueOnce(d3.promise).mockReturnValueOnce(d9.promise);

    await userEvent.selectOptions(sel("Vendedor"), "3");
    await userEvent.selectOptions(sel("Vendedor"), "9");
    await act(async () => d9.resolve([cliente(901)]));
    await screen.findByRole("option", { name: "#901 - Cliente 901" });
    await act(async () => d3.resolve([cliente(301)]));

    expect(screen.getByRole("option", { name: "#901 - Cliente 901" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "#301 - Cliente 301" })).not.toBeInTheDocument();
    expect(sel("Cliente")).toBeEnabled();
  });

  it("cascata: trocar de vendedor limpa o cliente que nao e da nova carteira", async () => {
    montar(c, { mode: "edit", registro: c.registro });
    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    expect(sel("Cliente").value).toBe("301");
    await userEvent.selectOptions(sel("Vendedor"), "9");
    await screen.findByRole("option", { name: "#901 - Cliente 901" });
    expect(sel("Cliente").value).toBe("");
  });

  it("cascata: fechar com a busca em voo e reabrir no mesmo vendedor nao trava o loading", async () => {
    const d = deferred<ClienteResumo[]>();
    api.apiListClientesDoVendedor.mockReturnValueOnce(d.promise);
    const m = montar(c, { mode: "edit", registro: c.registro });
    expect(screen.getByRole("option", { name: "Carregando clientes..." })).toBeInTheDocument();

    m.set({ open: false });
    await act(async () => d.resolve([cliente(301)]));
    m.set({ open: true });

    expect(await screen.findByRole("option", { name: "#301 - Cliente 301" })).toBeInTheDocument();
    expect(sel("Cliente")).toBeEnabled();
    expect(api.apiListClientesDoVendedor).toHaveBeenCalledTimes(2);
  });

  it("cascata: erro ao buscar clientes mostra o alerta e desliga o loading", async () => {
    api.apiListClientesDoVendedor.mockRejectedValueOnce(new Error("boom"));
    montar(c, { mode: "edit", registro: c.registro });
    expect(
      await screen.findByText("Nao foi possivel carregar clientes do vendedor: boom")
    ).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Carregando clientes..." })).not.toBeInTheDocument();
  });

  it("erro ao carregar vendedores mostra o alerta e libera o select", async () => {
    api.apiListVendedores.mockRejectedValueOnce(new Error("sem rede"));
    montar(c);
    expect(await screen.findByText("Nao foi possivel carregar vendedores: sem rede")).toBeInTheDocument();
    expect(sel("Vendedor")).toBeEnabled();
  });

  it("fechar com vendedores em voo e reabrir: loading termina com a nova busca", async () => {
    const d = deferred<Vendedor[]>();
    api.apiListVendedores.mockReturnValueOnce(d.promise);
    const m = montar(c);
    m.set({ open: false });
    await act(async () => d.resolve([]));
    m.set({ open: true });
    await waitFor(() => expect(sel("Vendedor")).toBeEnabled());
    expect(screen.getByRole("option", { name: "#3 - Vend 3" })).toBeInTheDocument();
  });

  it("submit sem cliente mostra a validacao; com os dados envia o payload", async () => {
    const m = montar(c, { mode: "edit", registro: c.registro });
    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    await waitFor(() => expect(m.props.onSubmit).toHaveBeenCalledTimes(1));
    expect(m.props.onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ vendedor_id: 3, cliente_id: 301 })
    );
  });

  it("erro do onSubmit aparece no modal", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("falhou salvar"));
    montar(c, { mode: "edit", registro: c.registro, onSubmit });
    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText("falhou salvar")).toBeInTheDocument();
  });

  it.each<[string, (u: ReturnType<typeof userEvent.setup>) => Promise<void>, string]>([
    ["sem vendedor", async () => {}, "Vendedor e obrigatorio."],
    [
      "sem cliente",
      async (u) => {
        await u.selectOptions(sel("Vendedor"), "3");
        await screen.findByRole("option", { name: "#301 - Cliente 301" });
      },
      "Cliente e obrigatorio.",
    ],
  ])("validacao: %s", async (_n, preparar, msg) => {
    const m = montar(c);
    await waitFor(() => expect(sel("Vendedor")).toBeEnabled());
    await preparar(userEvent.setup());
    const form = screen.getByRole("button", { name: c.botaoCriar }).closest("form")!;
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(await screen.findByText(msg)).toBeInTheDocument();
    expect(m.props.onSubmit).not.toHaveBeenCalled();
  });
});

describe("OportunidadeModal - regras proprias", () => {
  const c = CASOS[0];

  it("etapa 'Fechado perdido' exige o motivo da perda", async () => {
    const m = montar(c, { mode: "edit", registro: c.registro });
    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    await userEvent.selectOptions(sel("Etapa"), "Fechado perdido");
    const form = screen.getByRole("button", { name: "Salvar alteracoes" }).closest("form")!;
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(
      await screen.findByText("Motivo da perda e obrigatorio quando a etapa e Fechado perdido.")
    ).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Motivo da perda"), " Preco ");
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    await waitFor(() =>
      expect(m.props.onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ etapa: "Fechado perdido", motivo_perda: "Preco", ciclo_dias: 40 })
      )
    );
  });

  it.each<[string, string, string, string]>([
    ["probabilidade > 100", "Probabilidade (%)", "150", "Probabilidade deve ser um numero entre 0 e 100."],
    ["valor negativo", "Valor estimado", "-1", "Valor estimado deve ser um numero maior ou igual a zero."],
    ["ciclo negativo", "Ciclo (dias)", "-2", "Ciclo (dias) deve ser um numero maior ou igual a zero."],
  ])("validacao numerica: %s", async (_n, label, v, msg) => {
    montar(c, { mode: "edit", registro: c.registro });
    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    await userEvent.clear(screen.getByLabelText(label));
    await userEvent.type(screen.getByLabelText(label), v);
    const form = screen.getByRole("button", { name: "Salvar alteracoes" }).closest("form")!;
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });
});

describe("VisitaModal - regras proprias", () => {
  const c = CASOS[1];

  it("duracao vazia vira 0 e data vazia e barrada", async () => {
    const m = montar(c, { mode: "edit", registro: c.registro });
    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    await userEvent.clear(screen.getByLabelText("Duracao (min)"));
    await userEvent.clear(screen.getByLabelText("Data da visita"));
    const form = screen.getByRole("button", { name: "Salvar alteracoes" }).closest("form")!;
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(await screen.findByText("Data da visita e obrigatoria.")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Data da visita"), "2026-09-01");
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    await waitFor(() =>
      expect(m.props.onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ duracao_min: 0, data_visita: "2026-09-01" })
      )
    );
  });
});
