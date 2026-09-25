/**
 * Fluxos de CRUD nas telas refatoradas no FE-03 (pedidos, pagamentos,
 * oportunidades e visitas): excluir, editar via modal, criar via modal e o
 * master-detail de itens do pedido. API e sessao mockadas.
 */
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentType, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { MeResponse } from "@/lib/types";

vi.mock("@/components/layout/CarteiraGuard", () => ({
  CarteiraGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/layout/ProtectedRoute", () => ({
  ProtectedRoute: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/layout/Navbar", () => ({ Navbar: () => null }));

const ADMIN: MeResponse = { id: 1, nome: "Adm", email: "a@x", role: "admin", ativo: true, id_vendedor: null };
vi.mock("@/lib/session", () => ({
  useSessionUser: () => ADMIN,
  useVendedorDesligado: () => false,
  getSessionUser: () => ADMIN,
  refreshSessionUser: () => Promise.resolve(ADMIN),
}));

const api = vi.hoisted(() => ({
  apiListPedidos: vi.fn(),
  apiGetPedido: vi.fn(),
  apiCreatePedido: vi.fn(),
  apiUpdatePedido: vi.fn(),
  apiDeletePedido: vi.fn(),
  apiListPagamentos: vi.fn(),
  apiCreatePagamento: vi.fn(),
  apiUpdatePagamento: vi.fn(),
  apiDeletePagamento: vi.fn(),
  apiListOportunidades: vi.fn(),
  apiCreateOportunidade: vi.fn(),
  apiUpdateOportunidade: vi.fn(),
  apiDeleteOportunidade: vi.fn(),
  apiListVisitas: vi.fn(),
  apiCreateVisita: vi.fn(),
  apiUpdateVisita: vi.fn(),
  apiDeleteVisita: vi.fn(),
  apiListVendedores: vi.fn(),
  apiListClientes: vi.fn(),
  apiListClientesDoVendedor: vi.fn(),
  apiListProdutos: vi.fn(),
  apiMe: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import PedidosPage from "./admin/pedidos/page";
import PagamentosPage from "./pagamentos/page";
import OportunidadesPage from "./admin/oportunidades/page";
import VisitasPage from "./admin/visitas/page";

// ─── Fixtures ─────────────────────────────────────────────────────────────────

const pedido = (id: number, extra: Record<string, unknown> = {}) => ({
  pedido_id_origem: id,
  cliente_id: 100,
  vendedor_id: 1,
  cliente_nome: `Cliente ${id}`,
  vendedor_nome: "Vend 1",
  data_pedido: "2026-09-01",
  canal: "App",
  status: "Faturado",
  valor_total: 10,
  ...extra,
});

const pagamento = (id: number, extra: Record<string, unknown> = {}) => ({
  pagamento_id: id,
  pedido_id: 1,
  forma_pagamento: "PIX",
  parcelas: 1,
  valor: 10,
  taxa_pct: 0,
  valor_liquido: 10,
  data_vencimento: "2026-09-01",
  data_pagamento: null,
  status_pagamento: "Pago",
  ...extra,
});

const oportunidade = (id: number, extra: Record<string, unknown> = {}) => ({
  oportunidade_id: id,
  cliente_id: 100,
  vendedor_id: 1,
  origem: "WhatsApp",
  data_abertura: "2026-09-01",
  etapa: "Prospecção",
  probabilidade_pct: 10,
  valor_estimado: 100,
  data_fechamento: null,
  ciclo_dias: null,
  motivo_perda: null,
  ...extra,
});

const visita = (id: number, extra: Record<string, unknown> = {}) => ({
  visita_id: id,
  cliente_id: 100,
  vendedor_id: 1,
  data_visita: "2026-09-01",
  resultado: "Sem pedido",
  duracao_min: 30,
  ...extra,
});

function pagina<T>(data: T[]) {
  return { data, page: 1, limit: 20, total: data.length, pages: 1 };
}

/** Espera a busca extra do debounce de filtros (350ms) que roda na montagem
 * (ver BUG FE-04 em listasPaginadas.test.tsx) antes de interagir. */
async function aposMontagem(texto: string) {
  await screen.findByRole("button", { name: texto });
  await act(() => new Promise((r) => setTimeout(r, 420)));
}

// ─── Configuracao por tela ────────────────────────────────────────────────────

type Fn = keyof typeof api;

interface Tela {
  nome: string;
  Page: ComponentType;
  list: Fn;
  del: Fn;
  update: Fn;
  create: Fn;
  linha: (id: number, extra?: Record<string, unknown>) => Record<string, unknown>;
  /** Texto da linha na tabela (id 5). */
  texto: string;
  tituloExcluir: string;
  msgExcluido: string;
  tituloEditar: string;
  modalEditar: string;
  msgAtualizado: string;
  botaoNovo: string;
  modalNovo: string;
  /** Preenche o modal de "novo" (dentro do dialog). */
  preencherNovo: (d: HTMLElement) => Promise<void>;
  botaoCriar: string;
  msgCriado: string;
}

const TELAS: Tela[] = [
  {
    nome: "pedidos",
    Page: PedidosPage,
    list: "apiListPedidos",
    del: "apiDeletePedido",
    update: "apiUpdatePedido",
    create: "apiCreatePedido",
    linha: pedido,
    texto: "Cliente 5",
    tituloExcluir: "Excluir pedido",
    msgExcluido: "Pedido #5 excluido com sucesso.",
    tituloEditar: "Editar pedido",
    modalEditar: "Editar Pedido",
    msgAtualizado: "Pedido #5 atualizado com sucesso.",
    botaoNovo: "+ Novo Pedido",
    modalNovo: "Novo Pedido",
    preencherNovo: async (d) => {
      const w = within(d);
      await waitFor(() => expect(w.getByLabelText("Vendedor")).toBeEnabled());
      await userEvent.selectOptions(w.getByLabelText("Vendedor"), "1");
      await w.findByRole("option", { name: "#100 - Cliente 100" });
      await userEvent.selectOptions(w.getByLabelText("Cliente"), "100");
      await userEvent.selectOptions(w.getByLabelText("Produto"), "50");
    },
    botaoCriar: "Criar pedido",
    msgCriado: "Pedido #9 criado com sucesso.",
  },
  {
    nome: "pagamentos",
    Page: PagamentosPage,
    list: "apiListPagamentos",
    del: "apiDeletePagamento",
    update: "apiUpdatePagamento",
    create: "apiCreatePagamento",
    linha: pagamento,
    texto: "PIX",
    tituloExcluir: "Excluir pagamento",
    msgExcluido: "Pagamento #5 excluido com sucesso.",
    tituloEditar: "Editar pagamento",
    modalEditar: "Editar Pagamento",
    msgAtualizado: "Pagamento #5 atualizado com sucesso.",
    botaoNovo: "+ Novo Pagamento",
    modalNovo: "Novo Pagamento",
    preencherNovo: async (d) => {
      const w = within(d);
      await w.findByRole("option", { name: /^#5 - Cliente 5/ });
      await userEvent.selectOptions(w.getByLabelText("Pedido"), "5");
      await userEvent.type(w.getByLabelText("Valor"), "10");
      await userEvent.type(w.getByLabelText("Valor liquido"), "10");
      await userEvent.type(w.getByLabelText("Data de vencimento"), "2026-10-01");
    },
    botaoCriar: "Criar pagamento",
    msgCriado: "Pagamento #9 criado com sucesso.",
  },
  {
    nome: "oportunidades",
    Page: OportunidadesPage,
    list: "apiListOportunidades",
    del: "apiDeleteOportunidade",
    update: "apiUpdateOportunidade",
    create: "apiCreateOportunidade",
    linha: oportunidade,
    texto: "#100 - Cliente 100",
    tituloExcluir: "Excluir oportunidade",
    msgExcluido: "Oportunidade #5 excluida com sucesso.",
    tituloEditar: "Editar oportunidade",
    modalEditar: "Editar Oportunidade",
    msgAtualizado: "Oportunidade #5 atualizada com sucesso.",
    botaoNovo: "+ Nova Oportunidade",
    modalNovo: "Nova Oportunidade",
    preencherNovo: async (d) => {
      const w = within(d);
      await waitFor(() => expect(w.getByLabelText("Vendedor")).toBeEnabled());
      await userEvent.selectOptions(w.getByLabelText("Vendedor"), "1");
      await w.findByRole("option", { name: "#100 - Cliente 100" });
      await userEvent.selectOptions(w.getByLabelText("Cliente"), "100");
      await userEvent.type(w.getByLabelText("Probabilidade (%)"), "20");
      await userEvent.type(w.getByLabelText("Valor estimado"), "300");
    },
    botaoCriar: "Criar oportunidade",
    msgCriado: "Oportunidade #9 criada com sucesso.",
  },
  {
    nome: "visitas",
    Page: VisitasPage,
    list: "apiListVisitas",
    del: "apiDeleteVisita",
    update: "apiUpdateVisita",
    create: "apiCreateVisita",
    linha: visita,
    texto: "#100 - Cliente 100",
    tituloExcluir: "Excluir visita",
    msgExcluido: "Visita #5 excluida com sucesso.",
    tituloEditar: "Editar visita",
    modalEditar: "Editar Visita",
    msgAtualizado: "Visita #5 atualizada com sucesso.",
    botaoNovo: "+ Nova Visita",
    modalNovo: "Nova Visita",
    preencherNovo: async (d) => {
      const w = within(d);
      await waitFor(() => expect(w.getByLabelText("Vendedor")).toBeEnabled());
      await userEvent.selectOptions(w.getByLabelText("Vendedor"), "1");
      await w.findByRole("option", { name: "#100 - Cliente 100" });
      await userEvent.selectOptions(w.getByLabelText("Cliente"), "100");
      await userEvent.type(w.getByLabelText("Duracao (min)"), "15");
    },
    botaoCriar: "Criar visita",
    msgCriado: "Visita #9 criada com sucesso.",
  },
];

const detalhePedido = (id: number) => ({
  ...pedido(id),
  itens: [
    {
      item_id_origem: 1,
      produto_id: 50,
      produto_descricao: "Perfume 50",
      produto_sku: "P50",
      quantidade: 2,
      preco_praticado: 10,
      desconto_pct: 5,
      valor_bruto: 20,
    },
  ],
});

beforeEach(() => {
  vi.clearAllMocks();
  Object.values(api).forEach((f) => f.mockReset());
  api.apiListVendedores.mockResolvedValue([
    { id: 1, nome: "Vend 1", regiao: "S", uf: "SP", data_desligamento: null },
  ]);
  api.apiListClientes.mockResolvedValue(
    pagina([{ cliente_id_origem: 100, razao_social: "Cliente 100" }])
  );
  api.apiListClientesDoVendedor.mockResolvedValue([
    { id: 100, razao_social: "Cliente 100", cnpj: "", segmento: "", cidade: "", uf: "SP", carteira_id: 1, data_inicio: "", data_fim: null },
  ]);
  api.apiListProdutos.mockResolvedValue(
    pagina([{ id: 50, sku: "P50", descricao: "Perfume 50", preco_tabela: 10, ativo: true }])
  );
  api.apiGetPedido.mockImplementation(async (id: number) => detalhePedido(id));
  // Lista de pedidos tambem alimenta o select do PagamentoModal.
  api.apiListPedidos.mockResolvedValue(pagina([pedido(5)]));
});

describe.each(TELAS)("CRUD $nome", (t) => {
  beforeEach(() => {
    api[t.list].mockResolvedValue(pagina([t.linha(5)]));
  });

  it("excluir: confirma, chama a API, remove a linha e mostra sucesso", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api[t.del].mockResolvedValue(undefined);
    render(<t.Page />);
    await aposMontagem(t.texto);
    await userEvent.click(screen.getByTitle(t.tituloExcluir));
    expect(await screen.findByText(t.msgExcluido)).toBeInTheDocument();
    expect(api[t.del]).toHaveBeenCalledWith(5);
    expect(screen.queryByRole("button", { name: t.texto })).not.toBeInTheDocument();
  });

  // BUG FE-04 (variante): a busca extra do debounce da montagem (350ms)
  // nao e descartada. Se ela sair antes de um DELETE ser gravado e responder
  // depois dele (consulta lenta), a linha excluida volta para a tabela.
  // Remover o `.fails` quando corrigido.
  it.fails("excluir com a busca do debounce em voo: a resposta antiga nao ressuscita a linha", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    let resolverDel: (v?: unknown) => void = () => {};
    api[t.del].mockReturnValue(new Promise((r) => (resolverDel = r)));
    render(<t.Page />);
    await screen.findByRole("button", { name: t.texto });

    // 2a busca (debounce) sai com o snapshot anterior ao DELETE e demora.
    let resolverLista: (v: unknown) => void = () => {};
    api[t.list].mockReturnValueOnce(new Promise((r) => (resolverLista = r)));
    await userEvent.click(screen.getByTitle(t.tituloExcluir));
    await waitFor(() => expect(api[t.list]).toHaveBeenCalledTimes(2));

    resolverDel();
    await screen.findByText(t.msgExcluido);
    expect(screen.queryByRole("button", { name: t.texto })).not.toBeInTheDocument();

    await act(async () => resolverLista(pagina([t.linha(5)])));
    expect(screen.queryByRole("button", { name: t.texto })).not.toBeInTheDocument();
  });

  it("excluir: cancelar a confirmacao nao chama a API", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    render(<t.Page />);
    await aposMontagem(t.texto);
    await userEvent.click(screen.getByTitle(t.tituloExcluir));
    expect(api[t.del]).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: t.texto })).toBeInTheDocument();
  });

  it("excluir: erro da API aparece e a linha continua", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api[t.del].mockRejectedValue(new Error(`nao pode excluir ${t.nome}`));
    render(<t.Page />);
    await aposMontagem(t.texto);
    await userEvent.click(screen.getByTitle(t.tituloExcluir));
    expect(await screen.findByText(`nao pode excluir ${t.nome}`)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: t.texto })).toBeInTheDocument();
  });

  it("editar: abre o modal preenchido, salva, atualiza a linha e fecha", async () => {
    api[t.update].mockImplementation(async (id: number) => t.linha(id, { canal: "Telefone" }));
    render(<t.Page />);
    await aposMontagem(t.texto);
    await userEvent.click(screen.getAllByTitle(t.tituloEditar).at(-1)!);
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(t.modalEditar)).toBeInTheDocument();
    const salvar = within(dialog).getByRole("button", { name: "Salvar alteracoes" });
    await waitFor(() => expect(salvar).toBeEnabled());
    await userEvent.click(salvar);

    expect(await screen.findByText(t.msgAtualizado)).toBeInTheDocument();
    expect(api[t.update]).toHaveBeenCalledWith(5, expect.any(Object));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("criar: modal abre resetado, cria, recarrega a lista e mostra sucesso", async () => {
    api[t.create].mockResolvedValue(
      t.linha(9, { pedido_id_origem: 9, pagamento_id: 9, oportunidade_id: 9, visita_id: 9 })
    );
    render(<t.Page />);
    await aposMontagem(t.texto);
    const antes = api[t.list].mock.calls.length;

    await userEvent.click(screen.getByRole("button", { name: t.botaoNovo }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(t.modalNovo)).toBeInTheDocument();
    await t.preencherNovo(dialog);
    const criar = within(dialog).getByRole("button", { name: t.botaoCriar });
    await waitFor(() => expect(criar).toBeEnabled());
    await userEvent.click(criar);

    expect(await screen.findByText(t.msgCriado)).toBeInTheDocument();
    expect(api[t.create]).toHaveBeenCalledTimes(1);
    expect(api[t.list].mock.calls.length).toBeGreaterThan(antes);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("erro ao salvar fica no modal (que continua aberto)", async () => {
    api[t.update].mockRejectedValue(new Error("regra de negocio"));
    render(<t.Page />);
    await aposMontagem(t.texto);
    await userEvent.click(screen.getAllByTitle(t.tituloEditar).at(-1)!);
    const dialog = await screen.findByRole("dialog");
    const salvar = within(dialog).getByRole("button", { name: "Salvar alteracoes" });
    await waitFor(() => expect(salvar).toBeEnabled());
    await userEvent.click(salvar);
    expect(await within(dialog).findByText(/regra de negocio/)).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
});

describe("Pedidos - itens do pedido (master-detail)", () => {
  beforeEach(() => {
    api.apiListPedidos.mockResolvedValue(pagina([pedido(5)]));
  });

  it("'Itens' mostra os itens do pedido e 'Ocultar itens' esconde", async () => {
    render(<PedidosPage />);
    await aposMontagem("Cliente 5");
    await userEvent.click(screen.getByRole("button", { name: "Itens" }));
    expect(await screen.findByText("Itens do pedido #5")).toBeInTheDocument();
    expect(await screen.findByText(/#50 - Perfume 50/)).toBeInTheDocument();
    expect(api.apiGetPedido).toHaveBeenCalledWith(5);

    await userEvent.click(screen.getByRole("button", { name: "Ocultar itens" }));
    expect(screen.queryByText("Itens do pedido #5")).not.toBeInTheDocument();
  });

  it("erro ao carregar os itens mostra o alerta", async () => {
    api.apiGetPedido.mockRejectedValue(new Error("itens indisponiveis"));
    render(<PedidosPage />);
    await aposMontagem("Cliente 5");
    await userEvent.click(screen.getByRole("button", { name: "Itens" }));
    expect(await screen.findByText("itens indisponiveis")).toBeInTheDocument();
  });

  it("erro ao abrir o pedido para edicao mostra o alerta e nao abre o modal", async () => {
    api.apiGetPedido.mockRejectedValue(new Error("pedido sumiu"));
    render(<PedidosPage />);
    await aposMontagem("Cliente 5");
    await userEvent.click(screen.getByTitle("Editar pedido"));
    expect(await screen.findByText("pedido sumiu")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("excluir o pedido aberto no detalhe fecha o detalhe", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api.apiDeletePedido.mockResolvedValue(undefined);
    render(<PedidosPage />);
    await aposMontagem("Cliente 5");
    await userEvent.click(screen.getByRole("button", { name: "Itens" }));
    await screen.findByText("Itens do pedido #5");
    await userEvent.click(screen.getByTitle("Excluir pedido"));
    await screen.findByText("Pedido #5 excluido com sucesso.");
    expect(screen.queryByText("Itens do pedido #5")).not.toBeInTheDocument();
  });
});
