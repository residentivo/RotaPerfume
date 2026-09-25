import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  ClienteResumo,
  MeResponse,
  PedidoDetalhe,
  Produto,
  Vendedor,
} from "@/lib/types";

// Mock da camada HTTP (inclui GET /api/auth/me usado pela sessao real).
const api = vi.hoisted(() => ({
  apiMe: vi.fn(),
  apiListVendedores: vi.fn(),
  apiListProdutos: vi.fn(),
  apiListClientesDoVendedor: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import { PedidoModal } from "./PedidoModal";
import { clearSession, refreshSessionUser } from "@/lib/session";

// ─── Fixtures ─────────────────────────────────────────────────────────────────

function me(extra: Partial<MeResponse> = {}): MeResponse {
  return {
    id: 1,
    nome: "Ana",
    email: "a@x",
    role: "normal",
    ativo: true,
    id_vendedor: 7,
    vendedor_nome: "Vend 7",
    ...extra,
  };
}

const vendedores: Vendedor[] = [
  { id: 3, nome: "Vend 3", regiao: "S", uf: "SP", data_desligamento: null },
  { id: 7, nome: "Vend 7", regiao: "S", uf: "SP", data_desligamento: null },
  { id: 9, nome: "Vend 9", regiao: "S", uf: "SP", data_desligamento: null },
];

function cliente(id: number): ClienteResumo {
  return {
    id,
    cnpj: "00",
    razao_social: `Cliente ${id}`,
    segmento: "",
    cidade: "",
    uf: "SP",
    carteira_id: 1,
    data_inicio: "2026-01-01",
    data_fim: null,
  };
}

const produto = {
  id: 50,
  sku: "P50",
  descricao: "Perfume 50",
  preco_tabela: 99.9,
  ativo: true,
} as Produto;

function staleCache(id_vendedor: number) {
  localStorage.setItem(
    "auth_user",
    JSON.stringify({ ...me(), id_vendedor, vendedor_nome: `Vend ${id_vendedor}` })
  );
}

function vendedorSelect(): HTMLSelectElement {
  return screen.getByLabelText("Vendedor") as HTMLSelectElement;
}

function renderModal(props: Partial<Parameters<typeof PedidoModal>[0]> = {}) {
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  render(
    <PedidoModal
      open
      mode="create"
      pedido={null}
      onClose={onClose}
      onSubmit={onSubmit}
      {...props}
    />
  );
  return { onSubmit, onClose };
}

beforeEach(() => {
  vi.clearAllMocks();
  clearSession();
  api.apiListVendedores.mockResolvedValue(vendedores);
  api.apiListProdutos.mockResolvedValue({ data: [produto], page: 1, limit: 100, total: 1, pages: 1 });
  api.apiListClientesDoVendedor.mockImplementation(async (id: number) => [cliente(id * 100)]);
});

// ─── Testes ───────────────────────────────────────────────────────────────────

describe("PedidoModal - vendedor travado pela sessao (/me)", () => {
  it.each<[string, number, number]>([
    // nome, id no localStorage (desatualizado), id devolvido por /me
    ["admin trocou 3 -> 7", 3, 7],
    ["admin trocou 9 -> 3", 9, 3],
  ])("%s: usa o id_vendedor de /me e nao o do localStorage", async (_n, cacheId, meId) => {
    // Sessao validada antes com o vinculo antigo.
    api.apiMe.mockResolvedValueOnce(me({ id_vendedor: cacheId }));
    await refreshSessionUser();
    // Admin altera o vinculo; o localStorage continua com o antigo.
    staleCache(cacheId);
    api.apiMe.mockResolvedValue(me({ id_vendedor: meId, vendedor_nome: `Vend ${meId}` }));

    renderModal();

    await waitFor(() => expect(vendedorSelect().value).toBe(String(meId)));
    expect(vendedorSelect()).toBeDisabled();
    await waitFor(() =>
      expect(api.apiListClientesDoVendedor).toHaveBeenLastCalledWith(meId)
    );
    // Ao abrir o modal, /me e rebuscado.
    expect(api.apiMe).toHaveBeenCalledTimes(2);
    expect(await screen.findByRole("option", { name: `#${meId * 100} - Cliente ${meId * 100}` })).toBeInTheDocument();
  });

  it("sessao ainda vazia + cache desatualizado: espera /me e nunca usa o id do cache", async () => {
    staleCache(3);
    api.apiMe.mockResolvedValue(me({ id_vendedor: 7 }));

    renderModal();

    await waitFor(() => expect(vendedorSelect().value).toBe("7"));
    expect(api.apiListClientesDoVendedor).not.toHaveBeenCalledWith(3);
  });

  it("vinculo alterado com o modal aberto (revalidacao de foco) atualiza sem novo login", async () => {
    api.apiMe.mockResolvedValue(me({ id_vendedor: 7 }));
    renderModal();
    await waitFor(() => expect(vendedorSelect().value).toBe("7"));

    api.apiMe.mockResolvedValue(me({ id_vendedor: 9, vendedor_nome: "Vend 9" }));
    await act(async () => {
      await refreshSessionUser();
    });

    await waitFor(() => expect(vendedorSelect().value).toBe("9"));
    await waitFor(() => expect(api.apiListClientesDoVendedor).toHaveBeenLastCalledWith(9));
  });

  it("vendedor proprio fora da lista de vendedores ainda aparece no select", async () => {
    api.apiListVendedores.mockResolvedValue([]);
    api.apiMe.mockResolvedValue(me({ id_vendedor: 42, vendedor_nome: "Vend 42" }));
    renderModal();
    await waitFor(() => expect(vendedorSelect().value).toBe("42"));
    expect(screen.getByRole("option", { name: "#42 - Vend 42" })).toBeInTheDocument();
  });

  it("submit envia o vendedor_id da sessao", async () => {
    api.apiMe.mockResolvedValueOnce(me({ id_vendedor: 3 }));
    await refreshSessionUser();
    staleCache(3);
    api.apiMe.mockResolvedValue(me({ id_vendedor: 7 }));
    const { onSubmit } = renderModal();

    await waitFor(() => expect(vendedorSelect().value).toBe("7"));
    await screen.findByRole("option", { name: "#700 - Cliente 700" });
    await screen.findByRole("option", { name: "#50 - Perfume 50" });
    await userEvent.selectOptions(screen.getByLabelText("Cliente"), "700");
    await userEvent.selectOptions(screen.getByLabelText("Produto"), "50");
    await userEvent.click(screen.getByRole("button", { name: "Criar pedido" }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      vendedor_id: 7,
      cliente_id: 700,
      itens: [{ produto_id: 50, quantidade: 1, preco_praticado: 99.9, desconto_pct: 0 }],
    });
  });

  it("edicao por usuario normal usa o vendedor da sessao, nao o do pedido", async () => {
    api.apiMe.mockResolvedValue(me({ id_vendedor: 7 }));
    await refreshSessionUser();
    const pedido = {
      id: 1,
      cliente_id: 300,
      cliente_nome: "Cliente 300",
      vendedor_id: 3,
      data_pedido: "2026-09-01T00:00:00Z",
      canal: "App",
      status: "Faturado",
      itens: [
        { item_id_origem: 1, produto_id: 50, quantidade: 2, preco_praticado: 10, desconto_pct: 0 },
      ],
    } as unknown as PedidoDetalhe;

    renderModal({ mode: "edit", pedido });

    await waitFor(() => expect(vendedorSelect().value).toBe("7"));
    expect(api.apiListClientesDoVendedor).not.toHaveBeenCalledWith(3);
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeInTheDocument();
  });
});

describe("PedidoModal - formulario", () => {
  async function prontoParaEnviar() {
    api.apiMe.mockResolvedValue(me({ id_vendedor: 7 }));
    await refreshSessionUser();
    const ctx = renderModal();
    await screen.findByRole("option", { name: "#700 - Cliente 700" });
    await screen.findByRole("option", { name: "#50 - Perfume 50" });
    return ctx;
  }

  it.each<[string, (u: ReturnType<typeof userEvent.setup>) => Promise<void>, RegExp]>([
    ["sem cliente", async () => {}, /Cliente e obrigatorio/],
    [
      "sem produto",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Cliente"), "700");
      },
      /Selecione o produto em todos os itens/,
    ],
    [
      "quantidade zero",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Cliente"), "700");
        await u.selectOptions(screen.getByLabelText("Produto"), "50");
        await u.clear(screen.getByLabelText("Qtd"));
        await u.type(screen.getByLabelText("Qtd"), "0");
      },
      /Quantidade deve ser maior que zero/,
    ],
    [
      "desconto acima de 100",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Cliente"), "700");
        await u.selectOptions(screen.getByLabelText("Produto"), "50");
        await u.clear(screen.getByLabelText("Desc. %"));
        await u.type(screen.getByLabelText("Desc. %"), "150");
      },
      /Desconto deve estar entre 0 e 100/,
    ],
    [
      "sem data",
      async (u) => {
        await u.selectOptions(screen.getByLabelText("Cliente"), "700");
        await u.clear(screen.getByLabelText("Data do pedido"));
      },
      /Data do pedido e obrigatoria/,
    ],
  ])("validacao: %s", async (_n, preencher, msg) => {
    const { onSubmit } = await prontoParaEnviar();
    const u = userEvent.setup();
    await preencher(u);
    // submit direto no form para contornar a validacao nativa (required/min/max)
    const form = screen.getByRole("button", { name: "Criar pedido" }).closest("form")!;
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(await screen.findByText(msg)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("adiciona/remove itens e recalcula o total", async () => {
    await prontoParaEnviar();
    const u = userEvent.setup();
    await u.selectOptions(screen.getByLabelText("Produto"), "50");
    expect(screen.getByText(/Total do pedido: R\$\s99,90/)).toBeInTheDocument();

    await u.click(screen.getByRole("button", { name: "+ Adicionar item" }));
    expect(screen.getAllByLabelText("Produto")).toHaveLength(2);
    await u.selectOptions(screen.getAllByLabelText("Produto")[1], "50");
    await u.clear(screen.getAllByLabelText("Qtd")[1]);
    await u.type(screen.getAllByLabelText("Qtd")[1], "2");
    expect(screen.getByText(/Total do pedido: R\$\s299,70/)).toBeInTheDocument();

    const remover = screen.getAllByTitle("Remover item");
    await u.click(remover[0]);
    expect(screen.getAllByLabelText("Produto")).toHaveLength(1);
    expect(screen.getByTitle("Remover item")).toBeDisabled();
  });

  it("carrega todas as paginas de produtos", async () => {
    api.apiListProdutos.mockImplementation(async (page: number) => ({
      data: [{ ...produto, id: 50 + page, descricao: `Perfume p${page}` }],
      page,
      limit: 100,
      total: 3,
      pages: 3,
    }));
    await prontoParaEnviarSemEspera();
    for (const p of [1, 2, 3]) {
      expect(await screen.findByRole("option", { name: `#${50 + p} - Perfume p${p}` })).toBeInTheDocument();
    }
    expect(api.apiListProdutos).toHaveBeenCalledTimes(3);
  });

  it.each<[string, () => void, RegExp]>([
    [
      "vendedores/produtos",
      () => api.apiListVendedores.mockRejectedValue(new Error("boom vend")),
      /Nao foi possivel carregar vendedores\/produtos: boom vend/,
    ],
    [
      "clientes do vendedor",
      () => api.apiListClientesDoVendedor.mockRejectedValue(new Error("boom cli")),
      /Nao foi possivel carregar clientes do vendedor: boom cli/,
    ],
  ])("erro ao carregar %s mostra alerta", async (_n, falhar, msg) => {
    falhar();
    await prontoParaEnviarSemEspera();
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });

  async function prontoParaEnviarSemEspera() {
    api.apiMe.mockResolvedValue(me({ id_vendedor: 7 }));
    await refreshSessionUser();
    return renderModal();
  }
});

describe("PedidoModal - outros perfis", () => {
  it("admin: vendedor nao e travado e comeca vazio", async () => {
    api.apiMe.mockResolvedValue(me({ role: "admin", id_vendedor: 7 }));
    await refreshSessionUser();
    renderModal();

    await waitFor(() => expect(vendedorSelect()).toBeEnabled());
    expect(vendedorSelect().value).toBe("");
    expect(api.apiListClientesDoVendedor).not.toHaveBeenCalled();

    await userEvent.selectOptions(vendedorSelect(), "9");
    await waitFor(() => expect(api.apiListClientesDoVendedor).toHaveBeenLastCalledWith(9));
  });

  it("normal sem vendedor: aviso e botao de criar desabilitado", async () => {
    api.apiMe.mockResolvedValue(me({ id_vendedor: null, vendedor_nome: null }));
    await refreshSessionUser();
    renderModal();

    expect(
      await screen.findByText(/Seu usuario nao esta vinculado a um vendedor, por isso/)
    ).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: "Criar pedido" })).toBeDisabled());
    expect(vendedorSelect().value).toBe("");
  });

  it("falha em /me ao abrir nao bloqueia o formulario e mantem o vendedor da sessao", async () => {
    api.apiMe.mockResolvedValueOnce(me({ id_vendedor: 7 }));
    await refreshSessionUser();
    // Cache local divergente (ex.: outra aba/adulterado) nao e a fonte da verdade.
    staleCache(3);
    api.apiMe.mockRejectedValue(new Error("rede"));
    renderModal();
    await waitFor(() => expect(api.apiListClientesDoVendedor).toHaveBeenCalled());
    expect(vendedorSelect().value).toBe("7");
    expect(api.apiListClientesDoVendedor).not.toHaveBeenCalledWith(3);
    expect(screen.getByRole("button", { name: "Criar pedido" })).toBeInTheDocument();
  });

  it("erro 400 de carteira no submit vira mensagem amigavel", async () => {
    api.apiMe.mockResolvedValue(me({ id_vendedor: 7 }));
    await refreshSessionUser();
    const onSubmit = vi
      .fn()
      .mockRejectedValue(new Error("cliente não pertence à carteira deste vendedor"));
    renderModal({ onSubmit });

    await screen.findByRole("option", { name: "#700 - Cliente 700" });
    await screen.findByRole("option", { name: "#50 - Perfume 50" });
    await userEvent.selectOptions(screen.getByLabelText("Cliente"), "700");
    await userEvent.selectOptions(screen.getByLabelText("Produto"), "50");
    await userEvent.click(screen.getByRole("button", { name: "Criar pedido" }));

    expect(
      await screen.findByText(/nao pertence a carteira ativa do vendedor/)
    ).toBeInTheDocument();
  });
});

// ─── FE-03: reset ao abrir e cascata vendedor -> cliente ─────────────────────

describe("PedidoModal - FE-03 (reset ao abrir e cascata)", () => {
  const hoje = new Date().toISOString().slice(0, 10);
  const pedidoEdit = {
    pedido_id_origem: 5,
    cliente_id: 301,
    cliente_nome: "Cliente 301",
    vendedor_id: 3,
    data_pedido: "2026-09-01T00:00:00Z",
    canal: "WhatsApp",
    status: "Faturado",
    valor_total: 20,
    itens: [
      { item_id_origem: 1, produto_id: 50, quantidade: 2, preco_praticado: 10, desconto_pct: 5 },
    ],
  } as unknown as PedidoDetalhe;

  function deferred<T>() {
    let resolve: (v: T) => void = () => {};
    const promise = new Promise<T>((r) => (resolve = r));
    return { promise, resolve };
  }

  function montarControlado(props: Partial<Parameters<typeof PedidoModal>[0]> = {}) {
    const base = {
      open: true,
      mode: "create" as "create" | "edit",
      pedido: null as PedidoDetalhe | null,
      onClose: vi.fn(),
      onSubmit: vi.fn().mockResolvedValue(undefined),
      ...props,
    };
    const r = render(<PedidoModal {...base} />);
    return {
      set: (p: Partial<typeof base>) => {
        Object.assign(base, p);
        r.rerender(<PedidoModal {...base} />);
      },
    };
  }

  function v(label: string): string {
    return (screen.getByLabelText(label) as HTMLInputElement).value;
  }

  async function comoAdmin() {
    api.apiMe.mockResolvedValue(me({ role: "admin", id_vendedor: null }));
    await refreshSessionUser();
  }

  it("admin: editar abre preenchido (cabecalho, itens e cliente original)", async () => {
    await comoAdmin();
    api.apiListClientesDoVendedor.mockResolvedValue([cliente(302)]);
    montarControlado({ mode: "edit", pedido: pedidoEdit });

    expect(screen.getByText("Editar Pedido")).toBeInTheDocument();
    expect(v("Data do pedido")).toBe("2026-09-01");
    expect(v("Canal")).toBe("WhatsApp");
    expect(v("Status")).toBe("Faturado");
    expect(v("Qtd")).toBe("2");
    expect(v("Preco")).toBe("10");
    expect(v("Desc. %")).toBe("5");
    await waitFor(() => expect(vendedorSelect().value).toBe("3"));
    // Cliente fora da carteira atual continua selecionado na edicao.
    expect(
      await screen.findByRole("option", { name: "#301 - Cliente 301 (fora da carteira)" })
    ).toBeInTheDocument();
    expect(v("Cliente")).toBe("301");
  });

  it("admin: editar -> fechar -> novo abre resetado", async () => {
    await comoAdmin();
    const m = montarControlado({ mode: "edit", pedido: pedidoEdit });
    await waitFor(() => expect(vendedorSelect().value).toBe("3"));
    m.set({ open: false });
    m.set({ open: true, mode: "create", pedido: null });

    expect(screen.getByText("Novo Pedido")).toBeInTheDocument();
    expect(vendedorSelect().value).toBe("");
    expect(v("Cliente")).toBe("");
    expect(v("Data do pedido")).toBe(hoje);
    expect(v("Canal")).toBe("App");
    expect(v("Status")).toBe("Em separação");
    expect(screen.getAllByLabelText("Produto")).toHaveLength(1);
    expect(v("Qtd")).toBe("1");
  });

  it("normal: novo sujo -> fechar -> reabrir volta resetado e com o vendedor travado", async () => {
    api.apiMe.mockResolvedValue(me({ id_vendedor: 7 }));
    await refreshSessionUser();
    const m = montarControlado();
    await screen.findByRole("option", { name: "#700 - Cliente 700" });
    await userEvent.selectOptions(screen.getByLabelText("Cliente"), "700");
    await userEvent.click(screen.getByRole("button", { name: "+ Adicionar item" }));
    expect(screen.getAllByLabelText("Produto")).toHaveLength(2);

    m.set({ open: false });
    m.set({ open: true });
    expect(screen.getAllByLabelText("Produto")).toHaveLength(1);
    expect(vendedorSelect().value).toBe("7");
    await screen.findByRole("option", { name: "#700 - Cliente 700" });
    expect(v("Cliente")).toBe("");
  });

  it("admin: limpar o vendedor com a busca de clientes em voo nao deixa o loading travado", async () => {
    await comoAdmin();
    montarControlado();
    await waitFor(() => expect(vendedorSelect()).toBeEnabled());
    const d = deferred<ClienteResumo[]>();
    api.apiListClientesDoVendedor.mockReturnValueOnce(d.promise);

    await userEvent.selectOptions(vendedorSelect(), "9");
    expect(screen.getByRole("option", { name: "Carregando clientes..." })).toBeInTheDocument();
    await userEvent.selectOptions(vendedorSelect(), "");
    expect(screen.queryByRole("option", { name: "Carregando clientes..." })).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Selecione um vendedor primeiro" })).toBeInTheDocument();

    await act(async () => d.resolve([cliente(900)]));
    expect(screen.queryByRole("option", { name: "#900 - Cliente 900" })).not.toBeInTheDocument();
  });

  it("admin: resposta obsoleta da cascata (3 chega depois do 9) e descartada", async () => {
    await comoAdmin();
    montarControlado();
    await waitFor(() => expect(vendedorSelect()).toBeEnabled());
    const d3 = deferred<ClienteResumo[]>();
    const d9 = deferred<ClienteResumo[]>();
    api.apiListClientesDoVendedor.mockReturnValueOnce(d3.promise).mockReturnValueOnce(d9.promise);

    await userEvent.selectOptions(vendedorSelect(), "3");
    await userEvent.selectOptions(vendedorSelect(), "9");
    await act(async () => d9.resolve([cliente(900)]));
    await screen.findByRole("option", { name: "#900 - Cliente 900" });
    await act(async () => d3.resolve([cliente(300)]));
    expect(screen.queryByRole("option", { name: "#300 - Cliente 300" })).not.toBeInTheDocument();
    expect(screen.getByLabelText("Cliente")).toBeEnabled();
  });

  it("normal sem vendedor: nao busca clientes e o select nao fica em loading", async () => {
    api.apiMe.mockResolvedValue(me({ id_vendedor: null, vendedor_nome: null }));
    await refreshSessionUser();
    montarControlado();
    await screen.findByText(/Seu usuario nao esta vinculado a um vendedor/);
    expect(screen.getByRole("option", { name: "Selecione um vendedor primeiro" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Carregando clientes..." })).not.toBeInTheDocument();
    expect(api.apiListClientesDoVendedor).not.toHaveBeenCalled();
  });

  it("fechar com vendedores/produtos em voo e reabrir: o loading termina", async () => {
    await comoAdmin();
    const d = deferred<Vendedor[]>();
    api.apiListVendedores.mockReturnValueOnce(d.promise);
    const m = montarControlado();
    m.set({ open: false });
    await act(async () => d.resolve([]));
    m.set({ open: true });
    await waitFor(() => expect(vendedorSelect()).toBeEnabled());
    expect(screen.getByRole("option", { name: "#9 - Vend 9" })).toBeInTheDocument();
  });
});
