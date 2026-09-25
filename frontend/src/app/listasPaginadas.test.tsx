/**
 * Regressao do FE-03 (setState sincrono removido dos useEffect) nas telas de
 * listagem paginada pela API. Um unico roteiro parametrizado roda em todas as
 * telas que seguem o mesmo contrato:
 *   apiListX(page, limit, filtros, orderBy, orderDir)
 *
 * Cobre: loading inicial, paginacao, "itens por pagina", ordenacao, reset
 * para a pagina 1 ao mudar filtro, erro da API e respostas fora de ordem.
 */
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentType, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { MeResponse } from "@/lib/types";

// ─── Mocks ────────────────────────────────────────────────────────────────────

vi.mock("@/components/layout/CarteiraGuard", () => ({
  CarteiraGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/layout/ProtectedRoute", () => ({
  ProtectedRoute: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/layout/Navbar", () => ({ Navbar: () => null }));

const sessao = vi.hoisted(() => ({ user: null as MeResponse | null }));
vi.mock("@/lib/session", () => ({
  useSessionUser: () => sessao.user,
  useVendedorDesligado: () => false,
  getSessionUser: () => sessao.user,
  refreshSessionUser: () => Promise.resolve(sessao.user),
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
  apiListClientes: vi.fn(),
  apiToggleClienteStatus: vi.fn(),
  apiCreateCliente: vi.fn(),
  apiUpdateCliente: vi.fn(),
  apiListOportunidades: vi.fn(),
  apiCreateOportunidade: vi.fn(),
  apiUpdateOportunidade: vi.fn(),
  apiDeleteOportunidade: vi.fn(),
  apiListVisitas: vi.fn(),
  apiCreateVisita: vi.fn(),
  apiUpdateVisita: vi.fn(),
  apiDeleteVisita: vi.fn(),
  apiListVendedores: vi.fn(),
  apiListClientesDoVendedor: vi.fn(),
  apiListEstoque: vi.fn(),
  apiCreateEstoque: vi.fn(),
  apiUpdateEstoque: vi.fn(),
  apiListProdutos: vi.fn(),
  apiCreateProduto: vi.fn(),
  apiUpdateProduto: vi.fn(),
  apiToggleProdutoStatus: vi.fn(),
  apiMe: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import PedidosPage from "./admin/pedidos/page";
import PagamentosPage from "./pagamentos/page";
import ClientesPage from "./admin/clientes/page";
import OportunidadesPage from "./admin/oportunidades/page";
import VisitasPage from "./admin/visitas/page";
import EstoquePage from "./admin/estoque/page";
import ProdutosPage from "./admin/produtos/page";

// ─── Configuracao por tela ────────────────────────────────────────────────────

type ListFn = keyof typeof api;

interface Tela {
  nome: string;
  Page: ComponentType;
  listFn: ListFn;
  /** Linha fake da API para o id informado. */
  linha: (id: number) => Record<string, unknown>;
  /** Texto da linha renderizada na tabela. */
  texto: (id: number) => string;
  ordemPadrao: [string | undefined, "asc" | "desc"];
  /** Cabecalho clicavel e o order_by esperado. */
  ordenar: { header: string; orderBy: string };
  filtro: {
    label: string;
    tipo: "select" | "input";
    valor: string;
    campo: string;
    esperado: unknown;
  };
}

const TELAS: Tela[] = [
  {
    nome: "pedidos",
    Page: PedidosPage,
    listFn: "apiListPedidos",
    linha: (id) => ({
      pedido_id_origem: id,
      cliente_id: id,
      vendedor_id: 1,
      cliente_nome: `Cliente ${id}`,
      vendedor_nome: "Vend",
      data_pedido: "2026-09-01",
      canal: "App",
      status: "Faturado",
      valor_total: 10,
    }),
    texto: (id) => `Cliente ${id}`,
    ordemPadrao: ["id", "desc"],
    ordenar: { header: "Status", orderBy: "status" },
    filtro: { label: "Status", tipo: "select", valor: "Faturado", campo: "status", esperado: "Faturado" },
  },
  {
    nome: "pagamentos",
    Page: PagamentosPage,
    listFn: "apiListPagamentos",
    linha: (id) => ({
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
    }),
    texto: (id) => `#${id}`,
    ordemPadrao: ["pagamento_id", "desc"],
    ordenar: { header: "Forma", orderBy: "forma_pagamento" },
    filtro: {
      label: "Status",
      tipo: "select",
      valor: "Inadimplente",
      campo: "status_pagamento",
      esperado: "Inadimplente",
    },
  },
  {
    nome: "clientes",
    Page: ClientesPage,
    listFn: "apiListClientes",
    linha: (id) => ({
      cliente_id_origem: id,
      razao_social: `Loja ${id}`,
      cnpj: "12345678000199",
      segmento: "Varejo",
      cidade: "SP",
      uf: "SP",
      bairro: "",
      data_cadastro: "2026-01-10",
      ativo: true,
    }),
    texto: (id) => `Loja ${id}`,
    ordemPadrao: ["id", "asc"],
    ordenar: { header: "Segmento", orderBy: "segmento" },
    filtro: { label: "Status", tipo: "select", valor: "inativo", campo: "ativo", esperado: false },
  },
  {
    nome: "oportunidades",
    Page: OportunidadesPage,
    listFn: "apiListOportunidades",
    linha: (id) => ({
      oportunidade_id: id,
      cliente_id: id,
      vendedor_id: 1,
      origem: "WhatsApp",
      data_abertura: "2026-09-01",
      etapa: "Prospecção",
      probabilidade_pct: 10,
      valor_estimado: 100,
      data_fechamento: null,
      ciclo_dias: null,
      motivo_perda: null,
    }),
    texto: (id) => `#${id} - Cliente`,
    ordemPadrao: ["id", "desc"],
    ordenar: { header: "Etapa", orderBy: "etapa" },
    filtro: { label: "Etapa", tipo: "select", valor: "Negociação", campo: "etapa", esperado: "Negociação" },
  },
  {
    nome: "visitas",
    Page: VisitasPage,
    listFn: "apiListVisitas",
    linha: (id) => ({
      visita_id: id,
      cliente_id: id,
      vendedor_id: 1,
      data_visita: "2026-09-01",
      resultado: "Sem pedido",
      duracao_min: 30,
    }),
    texto: (id) => `#${id} - Cliente`,
    ordemPadrao: ["id", "desc"],
    ordenar: { header: "Resultado", orderBy: "resultado" },
    filtro: {
      label: "Resultado",
      tipo: "select",
      valor: "Pedido realizado",
      campo: "resultado",
      esperado: "Pedido realizado",
    },
  },
  {
    nome: "estoque",
    Page: EstoquePage,
    listFn: "apiListEstoque",
    linha: (id) => ({
      id,
      data_snapshot: "2026-09-01",
      sku: `SKU${id}`,
      produto_descricao: "",
      saldo: 5,
      ruptura: false,
    }),
    texto: (id) => `SKU${id}`,
    ordemPadrao: ["sku", "asc"],
    ordenar: { header: "Saldo", orderBy: "saldo" },
    filtro: { label: "Ruptura", tipo: "select", valor: "sim", campo: "ruptura", esperado: true },
  },
  {
    nome: "produtos",
    Page: ProdutosPage,
    listFn: "apiListProdutos",
    linha: (id) => ({
      id,
      sku: `P${id}`,
      descricao: `Perfume ${id}`,
      categoria: "Perf",
      marca: "M",
      nota_olfativa: "",
      preco_tabela: 10,
      custo_unitario: 5,
      unidade: "UN",
      data_lancamento: null,
      ativo: true,
    }),
    texto: (id) => `Perfume ${id}`,
    ordemPadrao: ["id", "asc"],
    ordenar: { header: "Marca", orderBy: "marca" },
    filtro: { label: "Marca", tipo: "input", valor: "Chanel", campo: "marca", esperado: "Chanel" },
  },
];

// ─── Helpers ──────────────────────────────────────────────────────────────────

const PAGINAS = 3;

function resposta(t: Tela, ids: number[], pages = PAGINAS) {
  return { data: ids.map(t.linha), page: 1, limit: 20, total: pages * 20, pages };
}

/** Resposta padrao: uma linha cujo id identifica a pagina pedida (p*100). */
function mockPorPagina(t: Tela) {
  api[t.listFn].mockImplementation(async (page: number) => resposta(t, [page * 100]));
}

interface Chamada {
  page: number;
  limit: number;
  filtros: Record<string, unknown>;
  orderBy: string | undefined;
  orderDir: string | undefined;
}

function chamadas(t: Tela): Chamada[] {
  return api[t.listFn].mock.calls.map((c: unknown[]) => ({
    page: c[0] as number,
    limit: c[1] as number,
    filtros: (c[2] ?? {}) as Record<string, unknown>,
    orderBy: c[3] as string | undefined,
    orderDir: c[4] as string | undefined,
  }));
}

function ultima(t: Tela): Chamada {
  const cs = chamadas(t);
  return cs[cs.length - 1];
}

function spinner(): Element | null {
  return document.querySelector(".animate-spin");
}

/** Deixa passar o debounce de 350ms dos filtros (roda tambem na montagem). */
async function esperarDebounce() {
  await act(() => new Promise((r) => setTimeout(r, 420)));
}

async function montarCarregada(t: Tela) {
  render(<t.Page />);
  await screen.findByText(t.texto(100));
  await esperarDebounce();
  await screen.findByText(t.texto(100));
}

function botaoCabecalho(header: string): HTMLElement {
  const th = screen
    .getAllByRole("columnheader")
    .find((el) => el.textContent?.replace(/[↑↓↕]/g, "").trim() === header);
  if (!th) throw new Error(`cabecalho ${header} nao encontrado`);
  return within(th).getByRole("button");
}

interface Deferred<T> {
  promise: Promise<T>;
  resolve: (v: T) => void;
}
function deferred<T>(): Deferred<T> {
  let resolve: (v: T) => void = () => {};
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

const ADMIN: MeResponse = {
  id: 1,
  nome: "Adm",
  email: "a@x",
  role: "admin",
  ativo: true,
  id_vendedor: null,
};

beforeEach(() => {
  vi.clearAllMocks();
  Object.values(api).forEach((f) => f.mockReset());
  sessao.user = ADMIN;
  api.apiListVendedores.mockResolvedValue([]);
  api.apiListClientes.mockResolvedValue({ data: [], page: 1, limit: 100, total: 0, pages: 1 });
});

// ─── Roteiro parametrizado ────────────────────────────────────────────────────

describe.each(TELAS)("Listagem $nome (FE-03)", (t) => {
  beforeEach(() => mockPorPagina(t));

  it("mostra o loading e depois a listagem da pagina 1 com a ordenacao padrao", async () => {
    const d = deferred<ReturnType<typeof resposta>>();
    api[t.listFn].mockReturnValueOnce(d.promise);
    render(<t.Page />);

    expect(spinner()).not.toBeNull();
    const primeira = chamadas(t)[0];
    expect(primeira).toMatchObject({ page: 1, limit: 20 });
    expect([primeira.orderBy, primeira.orderDir]).toEqual(t.ordemPadrao);

    await act(async () => d.resolve(resposta(t, [100])));
    expect(await screen.findByText(t.texto(100))).toBeInTheDocument();
    expect(spinner()).toBeNull();
  });

  it("erro da API: mostra a mensagem e desliga o loading", async () => {
    api[t.listFn].mockRejectedValue(new Error(`falha ${t.nome}`));
    render(<t.Page />);
    expect(await screen.findByText(`falha ${t.nome}`)).toBeInTheDocument();
    expect(spinner()).toBeNull();
  });

  it("paginacao dispara nova busca com a pagina certa e liga o loading", async () => {
    await montarCarregada(t);
    const d = deferred<ReturnType<typeof resposta>>();
    api[t.listFn].mockReturnValueOnce(d.promise);

    await userEvent.click(screen.getByTitle("Proxima pagina"));
    expect(ultima(t)).toMatchObject({ page: 2, limit: 20 });
    expect(spinner()).not.toBeNull();

    await act(async () => d.resolve(resposta(t, [200])));
    expect(await screen.findByText(t.texto(200))).toBeInTheDocument();
    expect(spinner()).toBeNull();

    await userEvent.click(screen.getByTitle("Pagina anterior"));
    await screen.findByText(t.texto(100));
    expect(ultima(t)).toMatchObject({ page: 1 });
  });

  it("mudar 'Itens por pagina' busca com o novo limit", async () => {
    await montarCarregada(t);
    await userEvent.selectOptions(screen.getByLabelText("Itens por pagina"), "50");
    await waitFor(() => expect(ultima(t)).toMatchObject({ limit: 50 }));
  });

  it("ordenacao: 1o clique ordena asc pela coluna, 2o clique inverte", async () => {
    await montarCarregada(t);
    await userEvent.click(botaoCabecalho(t.ordenar.header));
    await waitFor(() =>
      expect(ultima(t)).toMatchObject({ orderBy: t.ordenar.orderBy, orderDir: "asc" })
    );
    await screen.findByText(t.texto(100));
    await userEvent.click(botaoCabecalho(t.ordenar.header));
    await waitFor(() =>
      expect(ultima(t)).toMatchObject({ orderBy: t.ordenar.orderBy, orderDir: "desc" })
    );
  });

  it("mudar o filtro volta para a pagina 1 e envia o filtro", async () => {
    await montarCarregada(t);
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    await screen.findByText(t.texto(200));

    const n = chamadas(t).length;
    const campo = screen.getByLabelText(t.filtro.label);
    if (t.filtro.tipo === "select") {
      await userEvent.selectOptions(campo, t.filtro.valor);
    } else {
      await userEvent.type(campo, t.filtro.valor);
    }

    await waitFor(() =>
      expect(ultima(t)).toMatchObject({
        page: 1,
        filtros: expect.objectContaining({ [t.filtro.campo]: t.filtro.esperado }),
      })
    );
    expect(await screen.findByText(t.texto(100))).toBeInTheDocument();
    // Nenhuma busca da pagina 2 com o filtro novo (o reset vem antes).
    const depois = chamadas(t).slice(n);
    expect(
      depois.some((c) => c.page === 2 && c.filtros[t.filtro.campo] === t.filtro.esperado)
    ).toBe(false);
  });

  // BUG FE-04 (reportado ao Analista, nao corrigido aqui): as listagens nao
  // descartam respostas obsoletas. Se a pagina 2 responder depois da 3, a
  // tabela mostra os dados da pagina 2 com o paginador na pagina 3.
  // `it.fails` documenta o bug: quando for corrigido, este teste passa a
  // falhar e o `.fails` deve ser removido.
  it.fails("resposta obsoleta (pagina 2 chega depois da 3) nao sobrescreve a mais nova", async () => {
    await montarCarregada(t);
    const p2 = deferred<ReturnType<typeof resposta>>();
    const p3 = deferred<ReturnType<typeof resposta>>();
    api[t.listFn].mockReturnValueOnce(p2.promise).mockReturnValueOnce(p3.promise);

    await userEvent.click(screen.getByTitle("Proxima pagina"));
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    expect(ultima(t)).toMatchObject({ page: 3 });

    await act(async () => p3.resolve(resposta(t, [300])));
    await screen.findByText(t.texto(300));
    await act(async () => p2.resolve(resposta(t, [200])));

    expect(screen.getByText(t.texto(300))).toBeInTheDocument();
    expect(screen.queryByText(t.texto(200))).not.toBeInTheDocument();
  });

  // BUG FE-04 (variante): o efeito de debounce dos filtros tambem roda na
  // montagem e, 350ms depois, dispara uma 2a busca com o `page` capturado
  // na montagem (1). Se o usuario ja foi para a pagina 2, a tabela volta a
  // mostrar a pagina 1 com o paginador na 2.
  it.fails("pagina trocada logo apos abrir a tela nao e sobrescrita pela busca do debounce", async () => {
    render(<t.Page />);
    await screen.findByText(t.texto(100));
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    await screen.findByText(t.texto(200));
    await esperarDebounce();
    expect(screen.getByText(t.texto(200))).toBeInTheDocument();
  });
});

// ─── Casos especificos ────────────────────────────────────────────────────────

describe.each([
  { nome: "oportunidades", Page: OportunidadesPage, listFn: "apiListOportunidades" as ListFn, vazio: "Nenhuma oportunidade cadastrada." },
  { nome: "visitas", Page: VisitasPage, listFn: "apiListVisitas" as ListFn, vazio: "Nenhuma visita cadastrada." },
])("Listagem $nome - escopo do usuario normal", ({ Page, listFn, vazio }) => {
  beforeEach(() => {
    api[listFn].mockResolvedValue({ data: [], page: 1, limit: 20, total: 0, pages: 0 });
  });

  it("normal com vendedor: filtro de vendedor travado no id da sessao e select oculto", async () => {
    sessao.user = { ...ADMIN, role: "normal", id_vendedor: 7 };
    render(<Page />);
    await waitFor(() =>
      expect(api[listFn]).toHaveBeenCalledWith(
        1,
        20,
        expect.objectContaining({ vendedor_id: 7 }),
        "id",
        "desc"
      )
    );
    expect(screen.queryByLabelText("Vendedor")).not.toBeInTheDocument();
  });

  it("normal sem vendedor: nao chama a API e o loading nao fica travado", async () => {
    sessao.user = { ...ADMIN, role: "normal", id_vendedor: null };
    render(<Page />);
    expect(await screen.findByText(vazio)).toBeInTheDocument();
    await esperarDebounce();
    expect(api[listFn]).not.toHaveBeenCalled();
    expect(spinner()).toBeNull();
  });

  it("admin: filtro de vendedor livre", async () => {
    api.apiListVendedores.mockResolvedValue([
      { id: 9, nome: "Vend 9", regiao: "S", uf: "SP", data_desligamento: null },
    ]);
    render(<Page />);
    await screen.findByRole("option", { name: "#9 - Vend 9" });
    await userEvent.selectOptions(screen.getByLabelText("Vendedor"), "9");
    await waitFor(() =>
      expect(api[listFn]).toHaveBeenLastCalledWith(
        1,
        20,
        expect.objectContaining({ vendedor_id: 9 }),
        "id",
        "desc"
      )
    );
  });
});

describe("Estoque - admin vem da sessao (/me), nao do localStorage", () => {
  beforeEach(() => mockPorPagina(TELAS.find((t) => t.nome === "estoque")!));

  it.each<[string, MeResponse | null, boolean]>([
    ["admin na sessao", ADMIN, true],
    ["normal na sessao", { ...ADMIN, role: "normal" }, false],
    ["sessao ainda vazia", null, false],
  ])("%s -> botao '+ Novo registro' visivel=%s", async (_n, user, visivel) => {
    // Cache adulterado no localStorage nao pode conceder o papel.
    localStorage.setItem("auth_user", JSON.stringify({ ...ADMIN, role: "admin" }));
    sessao.user = user;
    render(<EstoquePage />);
    await screen.findByText("SKU100");
    expect(!!screen.queryByRole("button", { name: "+ Novo registro" })).toBe(visivel);
  });
});

// ─── UI-03: coluna propria para o ID principal ────────────────────────────────

/** Texto do cabecalho sem os indicadores de ordenacao. */
function textoCabecalho(el: Element): string {
  return (el.textContent ?? "").replace(/[↑↓↕]/g, "").trim();
}

interface ColunaId {
  nome: string;
  /** Cabecalho da coluna do nome/descricao (null = tela sem coluna de nome). */
  nomeHeader: string | null;
  /** Texto esperado na coluna do nome para o id 100. */
  nomeTexto: string | null;
  /** order_by/dir esperados apos clicar no cabecalho "ID". */
  ordenarId: [string, "asc" | "desc"];
}

const COLUNA_ID: ColunaId[] = [
  { nome: "pedidos", nomeHeader: "Cliente", nomeTexto: "Cliente 100", ordenarId: ["id", "asc"] },
  { nome: "pagamentos", nomeHeader: "Forma", nomeTexto: "PIX", ordenarId: ["pagamento_id", "asc"] },
  { nome: "clientes", nomeHeader: "Razao Social", nomeTexto: "Loja 100", ordenarId: ["id", "desc"] },
  { nome: "estoque", nomeHeader: "SKU", nomeTexto: "SKU100", ordenarId: ["id", "asc"] },
  { nome: "produtos", nomeHeader: "Descricao", nomeTexto: "Perfume 100", ordenarId: ["id", "desc"] },
  { nome: "oportunidades", nomeHeader: null, nomeTexto: null, ordenarId: ["id", "asc"] },
  { nome: "visitas", nomeHeader: null, nomeTexto: null, ordenarId: ["id", "asc"] },
];

describe.each(COLUNA_ID)("UI-03 coluna ID - $nome", (c) => {
  const t = TELAS.find((x) => x.nome === c.nome)!;
  beforeEach(() => mockPorPagina(t));

  it("primeira coluna e 'ID' e a celula mostra #<id> em fonte mono", async () => {
    render(<t.Page />);
    const celulaId = await screen.findByText("#100");

    const headers = screen.getAllByRole("columnheader");
    expect(textoCabecalho(headers[0])).toBe("ID");
    expect(headers.filter((h) => textoCabecalho(h) === "ID")).toHaveLength(1);

    expect(celulaId).toHaveClass("font-mono");
    const linha = celulaId.closest("tr")!;
    const celulas = within(linha).getAllByRole("cell");
    expect(celulas[0]).toHaveTextContent(/^#100$/);
  });

  it("coluna do nome nao tem mais o prefixo '#<id> - '", async () => {
    if (!c.nomeHeader) return; // telas sem coluna de nome propria
    render(<t.Page />);
    const linha = (await screen.findByText("#100")).closest("tr")!;
    const headers = screen.getAllByRole("columnheader").map(textoCabecalho);
    const idx = headers.indexOf(c.nomeHeader);
    expect(idx).toBeGreaterThan(0);

    const celula = within(linha).getAllByRole("cell")[idx];
    expect(celula).toHaveTextContent(c.nomeTexto!);
    expect(celula.textContent).not.toMatch(/#100\s*-/);
    expect(screen.queryByText(/^#100 - /)).toBeNull();
  });

  it("clicar no cabecalho 'ID' ordena pelo id na API", async () => {
    await montarCarregada(t);
    await userEvent.click(botaoCabecalho("ID"));
    await waitFor(() =>
      expect(ultima(t)).toMatchObject({ orderBy: c.ordenarId[0], orderDir: c.ordenarId[1] })
    );
  });
});

describe("UI-03 - especificos", () => {
  it("pedidos: cabecalho da coluna do nome virou 'Cliente' (nao 'Pedido')", async () => {
    const t = TELAS.find((x) => x.nome === "pedidos")!;
    mockPorPagina(t);
    render(<t.Page />);
    await screen.findByText("#100");
    const headers = screen.getAllByRole("columnheader").map(textoCabecalho);
    expect(headers).toContain("Cliente");
    expect(headers).not.toContain("Pedido");
    // O nome do cliente e o botao que abre os itens/edicao.
    expect(screen.getByRole("button", { name: "Cliente 100" })).toHaveAttribute(
      "title",
      "Ver itens do pedido"
    );
  });

  it("pagamentos: ID e texto puro e o botao 'Editar pagamento' fica na coluna Forma", async () => {
    const t = TELAS.find((x) => x.nome === "pagamentos")!;
    mockPorPagina(t);
    render(<t.Page />);
    const celulaId = await screen.findByText("#100");
    expect(celulaId.tagName).toBe("SPAN");
    expect(celulaId.closest("button")).toBeNull();

    const headers = screen.getAllByRole("columnheader").map(textoCabecalho);
    expect(headers).not.toContain("Pagamento");
    const linha = celulaId.closest("tr")!;
    const celulaForma = within(linha).getAllByRole("cell")[headers.indexOf("Forma")];
    const botao = within(celulaForma).getByRole("button", { name: "PIX" });
    expect(botao).toHaveAttribute("title", "Editar pagamento");
  });
});
