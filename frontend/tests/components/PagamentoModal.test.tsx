import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { MeResponse, Pagamento, Pedido } from "@/lib/types";

// apiMe alimenta a sessao real (/me), usada para decidir o modo leitura.
const api = vi.hoisted(() => ({ apiListPedidos: vi.fn(), apiMe: vi.fn() }));
vi.mock("@/lib/api", () => api);

import { PagamentoModal } from "@/components/PagamentoModal";
import { clearSession, refreshSessionUser } from "@/lib/session";

function me(role: "admin" | "normal"): MeResponse {
  return {
    id: role === "admin" ? 1 : 2,
    nome: role,
    email: `${role}@x`,
    role,
    ativo: true,
    id_vendedor: role === "admin" ? null : 7,
  };
}

async function logar(role: "admin" | "normal") {
  api.apiMe.mockResolvedValue(me(role));
  await act(async () => {
    await refreshSessionUser();
  });
}

type Props = Parameters<typeof PagamentoModal>[0];

function pedido(id: number, nome = `Cliente ${id}`): Pedido {
  return {
    pedido_id_origem: id,
    cliente_id: id,
    vendedor_id: 1,
    data_pedido: "2026-09-01T12:00:00Z",
    canal: "App",
    status: "Faturado",
    valor_total: 100,
    created_at: "",
    updated_at: "",
    cliente_nome: nome,
    vendedor_nome: "V",
  };
}

function pagina(pedidos: Pedido[]) {
  return { data: pedidos, page: 1, limit: 20, total: pedidos.length, pages: 1 };
}

const pagamento: Pagamento = {
  pagamento_id: 77,
  pedido_id: 12,
  forma_pagamento: "PIX",
  parcelas: 3,
  valor: 300,
  taxa_pct: 1.5,
  valor_liquido: 295.5,
  data_vencimento: "2026-10-05T00:00:00Z",
  data_pagamento: "2026-10-01T00:00:00Z",
  status_pagamento: "Pago",
  created_at: "",
  updated_at: "",
};

function montar(props: Partial<Props> = {}) {
  const base: Props = {
    open: true,
    mode: "create",
    pagamento: null,
    onClose: vi.fn(),
    onSubmit: vi.fn().mockResolvedValue(undefined),
    ...props,
  };
  const r = render(<PagamentoModal {...base} />);
  return {
    props: base,
    set: (p: Partial<Props>) => {
      Object.assign(base, p);
      r.rerender(<PagamentoModal {...base} />);
    },
  };
}

function valor(label: string): string {
  return (screen.getByLabelText(label) as HTMLInputElement).value;
}

function deferred<T>() {
  let resolve: (v: T) => void = () => {};
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

const NOVO = {
  "Buscar pedido": "",
  Pedido: "",
  "Forma de pagamento": "Boleto 14 dias",
  Status: "Em aberto",
  Parcelas: "1",
  Valor: "",
  "Taxa (%)": "0",
  "Valor liquido": "",
  "Data de vencimento": "",
  "Data de pagamento": "",
};

beforeEach(async () => {
  vi.clearAllMocks();
  clearSession();
  // Padrao: admin (edita qualquer pagamento, inclusive quitado).
  await logar("admin");
  api.apiListPedidos.mockReset();
  api.apiListPedidos.mockImplementation(async (_p: number, _l: number, f: { q?: string }) =>
    pagina(f.q ? [pedido(900, `Busca ${f.q}`)] : [pedido(12), pedido(13)])
  );
});

describe("PagamentoModal (FE-03)", () => {
  it("abre resetado em modo novo e busca os pedidos (debounce)", async () => {
    montar();
    expect(screen.getByText("Novo Pagamento")).toBeInTheDocument();
    for (const [label, v] of Object.entries(NOVO)) expect(valor(label)).toBe(v);
    expect(api.apiListPedidos).not.toHaveBeenCalled();

    expect(await screen.findByRole("option", { name: /^#12 - Cliente 12/ })).toBeInTheDocument();
    expect(api.apiListPedidos).toHaveBeenCalledTimes(1);
    expect(api.apiListPedidos).toHaveBeenCalledWith(1, 20, { q: undefined }, "data_pedido", "desc");
    expect(screen.getByRole("option", { name: "Selecione um pedido" })).toBeInTheDocument();
  });

  it("abre preenchido em modo editar, com ids travados e sem buscar pedidos", async () => {
    montar({ mode: "edit", pagamento });
    expect(screen.getByText("Editar Pagamento")).toBeInTheDocument();
    expect(screen.getByLabelText("ID do pagamento")).toBeDisabled();
    expect(valor("ID do pagamento")).toBe("77");
    expect(valor("ID do pedido")).toBe("12");
    expect(screen.queryByLabelText("Buscar pedido")).not.toBeInTheDocument();
    expect(valor("Forma de pagamento")).toBe("PIX");
    expect(valor("Status")).toBe("Pago");
    expect(valor("Parcelas")).toBe("3");
    expect(valor("Valor")).toBe("300");
    expect(valor("Taxa (%)")).toBe("1.5");
    expect(valor("Valor liquido")).toBe("295.5");
    expect(valor("Data de vencimento")).toBe("2026-10-05");
    expect(valor("Data de pagamento")).toBe("2026-10-01");
    await act(() => new Promise((r) => setTimeout(r, 400)));
    expect(api.apiListPedidos).not.toHaveBeenCalled();
  });

  it("data_pagamento null vira campo vazio no modo editar", () => {
    montar({ mode: "edit", pagamento: { ...pagamento, data_pagamento: null } });
    expect(valor("Data de pagamento")).toBe("");
  });

  it("reabre resetado depois de fechar (busca, selecao e valores somem)", async () => {
    const m = montar();
    await screen.findByRole("option", { name: /^#12 - / });
    await userEvent.selectOptions(screen.getByLabelText("Pedido"), "12");
    await userEvent.type(screen.getByLabelText("Valor"), "50");
    await userEvent.type(screen.getByLabelText("Buscar pedido"), "abc");
    await screen.findByRole("option", { name: /^#900 - Busca abc/ });

    m.set({ open: false });
    m.set({ open: true });
    for (const [label, v] of Object.entries(NOVO)) expect(valor(label)).toBe(v);
    expect(screen.queryByRole("option", { name: /^#900/ })).not.toBeInTheDocument();
    // E volta a buscar a lista sem filtro.
    await screen.findByRole("option", { name: /^#12 - / });
  });

  it("editar -> fechar -> novo: nada do pagamento anterior sobra", () => {
    const m = montar({ mode: "edit", pagamento });
    m.set({ open: false });
    m.set({ open: true, mode: "create", pagamento: null });
    expect(screen.getByText("Novo Pagamento")).toBeInTheDocument();
    for (const [label, v] of Object.entries(NOVO)) expect(valor(label)).toBe(v);
  });

  it("digitar na busca faz 1 requisicao so depois do debounce, com o texto", async () => {
    montar();
    await screen.findByRole("option", { name: /^#12 - / });
    api.apiListPedidos.mockClear();
    await userEvent.type(screen.getByLabelText("Buscar pedido"), "  loja ");
    expect(await screen.findByRole("option", { name: /^#900 - Busca loja/ })).toBeInTheDocument();
    expect(api.apiListPedidos).toHaveBeenCalledTimes(1);
    expect(api.apiListPedidos).toHaveBeenCalledWith(1, 20, { q: "loja" }, "data_pedido", "desc");
  });

  it("busca obsoleta nao sobrescreve a mais nova", async () => {
    montar();
    await screen.findByRole("option", { name: /^#12 - / });
    const velha = deferred<ReturnType<typeof pagina>>();
    api.apiListPedidos.mockReturnValueOnce(velha.promise);

    await userEvent.type(screen.getByLabelText("Buscar pedido"), "a");
    await waitFor(() => expect(api.apiListPedidos).toHaveBeenCalledWith(1, 20, { q: "a" }, "data_pedido", "desc"));
    expect(screen.getByRole("option", { name: "Carregando pedidos..." })).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Buscar pedido"), "b");
    await screen.findByRole("option", { name: /^#900 - Busca ab/ });
    await act(async () => velha.resolve(pagina([pedido(555, "Velho")])));

    expect(screen.queryByRole("option", { name: /^#555/ })).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: /^#900 - Busca ab/ })).toBeInTheDocument();
  });

  it("erro na busca de pedidos deixa a lista vazia e desliga o loading", async () => {
    api.apiListPedidos.mockRejectedValueOnce(new Error("x"));
    montar();
    await waitFor(() => expect(api.apiListPedidos).toHaveBeenCalledTimes(1));
    expect(await screen.findByRole("option", { name: "Nenhum pedido encontrado" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Carregando pedidos..." })).not.toBeInTheDocument();
  });

  it("submit em modo novo envia o payload de criacao", async () => {
    const m = montar();
    await screen.findByRole("option", { name: /^#12 - / });
    const u = userEvent.setup();
    await u.selectOptions(screen.getByLabelText("Pedido"), "12");
    await u.selectOptions(screen.getByLabelText("Forma de pagamento"), "PIX");
    await u.type(screen.getByLabelText("Valor"), "100");
    await u.type(screen.getByLabelText("Valor liquido"), "98");
    await u.type(screen.getByLabelText("Data de vencimento"), "2026-11-01");
    await u.click(screen.getByRole("button", { name: "Criar pagamento" }));

    await waitFor(() => expect(m.props.onSubmit).toHaveBeenCalledTimes(1));
    expect(m.props.onSubmit).toHaveBeenCalledWith({
      pedido_id: 12,
      forma_pagamento: "PIX",
      parcelas: 1,
      valor: 100,
      taxa_pct: 0,
      valor_liquido: 98,
      data_vencimento: "2026-11-01",
      data_pagamento: undefined,
      status_pagamento: "Em aberto",
    });
  });

  it("submit em modo editar nao envia pedido_id", async () => {
    const m = montar({ mode: "edit", pagamento });
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    await waitFor(() => expect(m.props.onSubmit).toHaveBeenCalledTimes(1));
    const payload = (m.props.onSubmit as ReturnType<typeof vi.fn>).mock.calls[0][0];
    expect(payload).not.toHaveProperty("pedido_id");
    expect(payload).toMatchObject({ parcelas: 3, valor: 300, data_pagamento: "2026-10-01" });
  });

  it.each<[string, (u: ReturnType<typeof userEvent.setup>) => Promise<void>, string]>([
    ["sem pedido", async () => {}, "Selecione um pedido."],
    [
      "parcelas zero",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Pedido"), "12");
        await u.clear(screen.getByLabelText("Parcelas"));
        await u.type(screen.getByLabelText("Parcelas"), "0");
      },
      "Parcelas deve ser um numero maior ou igual a 1.",
    ],
    [
      "sem valor",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Pedido"), "12");
      },
      "Valor deve ser um numero maior ou igual a zero.",
    ],
    [
      "taxa vazia",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Pedido"), "12");
        await u.type(screen.getByLabelText("Valor"), "1");
        await u.clear(screen.getByLabelText("Taxa (%)"));
      },
      "Taxa (%) deve estar entre 0 e 30.",
    ],
    [
      "taxa acima de 30",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Pedido"), "12");
        await u.type(screen.getByLabelText("Valor"), "1");
        await u.clear(screen.getByLabelText("Taxa (%)"));
        await u.type(screen.getByLabelText("Taxa (%)"), "30.01");
      },
      "Taxa (%) deve estar entre 0 e 30.",
    ],
    [
      "taxa negativa",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Pedido"), "12");
        await u.type(screen.getByLabelText("Valor"), "1");
        await u.clear(screen.getByLabelText("Taxa (%)"));
        await u.type(screen.getByLabelText("Taxa (%)"), "-1");
      },
      "Taxa (%) deve estar entre 0 e 30.",
    ],
    [
      "sem valor liquido",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Pedido"), "12");
        await u.type(screen.getByLabelText("Valor"), "1");
      },
      "Valor liquido deve ser um numero maior ou igual a zero.",
    ],
    [
      "sem vencimento",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Pedido"), "12");
        await u.type(screen.getByLabelText("Valor"), "1");
        await u.type(screen.getByLabelText("Valor liquido"), "1");
      },
      "Data de vencimento e obrigatoria.",
    ],
  ])("validacao: %s", async (_n, preparar, msg) => {
    const m = montar();
    await screen.findByRole("option", { name: /^#12 - / });
    await preparar(userEvent.setup());
    const form = screen.getByRole("button", { name: "Criar pagamento" }).closest("form")!;
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(await screen.findByText(msg)).toBeInTheDocument();
    expect(m.props.onSubmit).not.toHaveBeenCalled();
  });

  it("erro do onSubmit aparece e some ao reabrir", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("pagamento recusado"));
    const m = montar({ mode: "edit", pagamento, onSubmit });
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText("pagamento recusado")).toBeInTheDocument();
    m.set({ open: false });
    m.set({ open: true });
    expect(screen.queryByText("pagamento recusado")).not.toBeInTheDocument();
  });
});

describe("PagamentoModal - SEC-27 (taxa e quitado por perfil)", () => {
  const CAMPOS = [
    "Forma de pagamento",
    "Status",
    "Parcelas",
    "Valor",
    "Taxa (%)",
    "Valor liquido",
    "Data de vencimento",
    "Data de pagamento",
  ];

  it("campo Taxa (%) tem max=30 para qualquer perfil", async () => {
    await logar("normal");
    montar();
    expect(screen.getByLabelText("Taxa (%)")).toHaveAttribute("max", "30");
    expect(screen.getByLabelText("Taxa (%)")).toHaveAttribute("min", "0");
  });

  it("taxa 30 (limite) e aceita e enviada", async () => {
    const m = montar({ mode: "edit", pagamento: { ...pagamento, status_pagamento: "Em aberto" } });
    const u = userEvent.setup();
    await u.clear(screen.getByLabelText("Taxa (%)"));
    await u.type(screen.getByLabelText("Taxa (%)"), "30");
    await u.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    await waitFor(() => expect(m.props.onSubmit).toHaveBeenCalledTimes(1));
    expect(m.props.onSubmit).toHaveBeenCalledWith(expect.objectContaining({ taxa_pct: 30 }));
  });

  it("admin edita taxa acima de 30: bloqueado no front com a mensagem", async () => {
    const m = montar({ mode: "edit", pagamento });
    const u = userEvent.setup();
    await u.clear(screen.getByLabelText("Taxa (%)"));
    await u.type(screen.getByLabelText("Taxa (%)"), "100");
    const form = screen.getByRole("button", { name: "Salvar alteracoes" }).closest("form")!;
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(await screen.findByText("Taxa (%) deve estar entre 0 e 30.")).toBeInTheDocument();
    expect(m.props.onSubmit).not.toHaveBeenCalled();
  });

  it.each(["Pago", "Pago com atraso"])(
    "normal + %s: modo leitura (campos desabilitados, sem salvar, com aviso)",
    async (status) => {
      await logar("normal");
      const m = montar({ mode: "edit", pagamento: { ...pagamento, status_pagamento: status } });
      expect(screen.getByText("Ver Pagamento")).toBeInTheDocument();
      expect(screen.getByText("Pagamento quitado nao pode ser editado.")).toBeInTheDocument();
      for (const c of CAMPOS) expect(screen.getByLabelText(c)).toBeDisabled();
      expect(valor("Status")).toBe(status);
      expect(screen.queryByRole("button", { name: "Salvar alteracoes" })).not.toBeInTheDocument();
      // Submit forcado (ex.: Enter) nao chama o onSubmit.
      // O "Fechar" do rodape substitui o "Cancelar" (o X do Modal tambem se chama "Fechar").
      const fecharRodape = screen.getAllByRole("button", { name: "Fechar" }).at(-1)!;
      expect(screen.queryByRole("button", { name: "Cancelar" })).not.toBeInTheDocument();
      const form = fecharRodape.closest("form")!;
      await act(async () => {
        form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });
      expect(m.props.onSubmit).not.toHaveBeenCalled();
      await userEvent.click(fecharRodape);
      expect(m.props.onClose).toHaveBeenCalled();
    }
  );

  it.each(["Em aberto", "Inadimplente"])(
    "normal + %s: edita normalmente (inclusive para Pago)",
    async (status) => {
      await logar("normal");
      const m = montar({ mode: "edit", pagamento: { ...pagamento, status_pagamento: status } });
      expect(screen.getByText("Editar Pagamento")).toBeInTheDocument();
      expect(screen.queryByText("Pagamento quitado nao pode ser editado.")).not.toBeInTheDocument();
      for (const c of CAMPOS) expect(screen.getByLabelText(c)).toBeEnabled();
      // Trocar o status no select para Pago nao trava o formulario (vale o gravado).
      await userEvent.selectOptions(screen.getByLabelText("Status"), "Pago");
      expect(screen.getByLabelText("Valor")).toBeEnabled();
      await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
      await waitFor(() => expect(m.props.onSubmit).toHaveBeenCalledTimes(1));
      expect(m.props.onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ status_pagamento: "Pago" })
      );
    }
  );

  it.each(["Pago", "Pago com atraso"])("admin + %s: editavel", async (status) => {
    const m = montar({ mode: "edit", pagamento: { ...pagamento, status_pagamento: status } });
    expect(screen.getByText("Editar Pagamento")).toBeInTheDocument();
    expect(screen.queryByText("Pagamento quitado nao pode ser editado.")).not.toBeInTheDocument();
    for (const c of CAMPOS) expect(screen.getByLabelText(c)).toBeEnabled();
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    await waitFor(() => expect(m.props.onSubmit).toHaveBeenCalledTimes(1));
  });

  it("sessao ainda vazia + quitado: modo leitura (fail-safe ate o /me)", () => {
    clearSession();
    montar({ mode: "edit", pagamento });
    expect(screen.queryByRole("button", { name: "Salvar alteracoes" })).not.toBeInTheDocument();
  });

  it("normal em criacao: formulario editavel", async () => {
    await logar("normal");
    montar();
    for (const c of CAMPOS) expect(screen.getByLabelText(c)).toBeEnabled();
    expect(screen.getByRole("button", { name: "Criar pagamento" })).toBeInTheDocument();
  });

  it("409 do backend (quitado) aparece no modal", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("pagamento já quitado não pode ser editado"));
    montar({ mode: "edit", pagamento, onSubmit });
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText("pagamento já quitado não pode ser editado")).toBeInTheDocument();
  });
});
