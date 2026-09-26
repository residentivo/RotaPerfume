/**
 * Regressao do FE-03 (useResetOnOpen) nos modais de formulario sem cascata:
 * ClienteModal, EstoqueModal, ProdutoModal, UserModal e VendedorModal.
 * Roteiro parametrizado: abre resetado em "novo", preenchido em "editar",
 * reabre resetado depois de fechar e limpa o erro do submit anterior.
 */
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentType } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  Cliente,
  ClienteResumo,
  Estoque,
  Produto,
  User,
  VendedorCompleto,
} from "@/lib/types";

const api = vi.hoisted(() => ({
  apiListVendedores: vi.fn(),
  apiGetVendedor: vi.fn(),
  apiListClientes: vi.fn(),
  apiVincularCliente: vi.fn(),
  apiDesvincularCliente: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import { ClienteModal } from "@/components/admin/ClienteModal";
import { EstoqueModal } from "@/components/admin/EstoqueModal";
import { ProdutoModal } from "@/components/admin/ProdutoModal";
import { UserModal } from "@/components/admin/UserModal";
import { VendedorModal } from "@/components/admin/VendedorModal";

const hoje = new Date().toISOString().slice(0, 10);

type Props = {
  open: boolean;
  mode: "create" | "edit";
  registro: unknown;
  onClose: () => void;
  onSubmit: (d: unknown) => Promise<void>;
};

interface Caso {
  nome: string;
  Modal: ComponentType<Props>;
  registro: unknown;
  tituloNovo: string;
  tituloEditar: string;
  botaoCriar: string;
  /** Campo de texto usado para "sujar" o formulario. */
  campoLivre: string;
  camposNovo: Record<string, string>;
  camposEditar: Record<string, string>;
  /** Preenche o minimo para o submit passar na validacao (modo novo). */
  preencherValido: (u: ReturnType<typeof userEvent.setup>) => Promise<void>;
  payloadNovo: Record<string, unknown>;
  payloadEditar: Record<string, unknown>;
  /** Espera assincrona extra ao abrir (ex.: lista de vendedores). */
  pronto?: () => Promise<void>;
}

const cliente: Cliente = {
  cliente_id_origem: 10,
  cnpj: "11222333000181",
  razao_social: "Loja A",
  segmento: "Varejo",
  cidade: "Campinas",
  uf: "SP",
  bairro: "Centro",
  data_cadastro: "2025-05-20T00:00:00Z",
  ativo: true,
  created_at: "",
  updated_at: "",
};

const estoque: Estoque = {
  id: 5,
  data_snapshot: "2026-09-01T00:00:00Z",
  sku: "SKU-5",
  produto_descricao: "Perfume",
  saldo: 12,
  ruptura: false,
  created_at: "",
  updated_at: "",
};

const produto: Produto = {
  id: 50,
  sku: "P50",
  descricao: "Perfume 50",
  categoria: "Perfumaria",
  marca: "Marca X",
  nota_olfativa: "Floral",
  preco_tabela: 99.9,
  custo_unitario: 40,
  unidade: "UN",
  data_lancamento: "2024-03-01",
  ativo: true,
  created_at: "",
  updated_at: "",
};

const usuario: User = {
  id: 2,
  nome: "Bia",
  email: "bia@x.com",
  role: "admin",
  ativo: true,
  id_vendedor: 3,
};

const vendedor: VendedorCompleto = {
  id: 3,
  nome: "Vend 3",
  regiao: "Sudeste",
  uf: "SP",
  data_admissao: "2020-02-02",
  data_desligamento: null,
  meta_mensal: 5000,
  created_at: "",
  updated_at: "",
};

async function type(u: ReturnType<typeof userEvent.setup>, label: string, v: string) {
  await u.type(screen.getByLabelText(label), v);
}

const CASOS: Caso[] = [
  {
    nome: "ClienteModal",
    Modal: ({ registro, ...p }) => <ClienteModal {...p} cliente={registro as Cliente | null} />,
    registro: cliente,
    tituloNovo: "Novo Cliente",
    tituloEditar: "Editar Cliente",
    botaoCriar: "Criar cliente",
    campoLivre: "Razao social",
    camposNovo: {
      "Razao social": "",
      CNPJ: "",
      Segmento: "",
      Cidade: "",
      UF: "",
      Bairro: "",
      "Data de cadastro": hoje,
    },
    camposEditar: {
      "Razao social": "Loja A",
      CNPJ: "11.222.333/0001-81",
      Segmento: "Varejo",
      Cidade: "Campinas",
      UF: "SP",
      Bairro: "Centro",
      "Data de cadastro": "2025-05-20",
    },
    preencherValido: async (u) => {
      await type(u, "Razao social", " Loja Nova ");
      await type(u, "CNPJ", "12abc34501de35");
      await type(u, "Segmento", "Varejo");
      await type(u, "Cidade", "Santos");
      await type(u, "UF", "sp");
    },
    payloadNovo: { razao_social: "Loja Nova", cnpj: "12ABC34501DE35", uf: "SP", data_cadastro: hoje },
    payloadEditar: { razao_social: "Loja A", cnpj: "11222333000181", data_cadastro: "2025-05-20" },
  },
  {
    nome: "EstoqueModal",
    Modal: ({ registro, ...p }) => <EstoqueModal {...p} estoque={registro as Estoque | null} />,
    registro: estoque,
    tituloNovo: "Novo Registro de Estoque",
    tituloEditar: "Editar Estoque",
    botaoCriar: "Criar registro",
    campoLivre: "Saldo",
    camposNovo: { SKU: "", "Data do snapshot": "", Saldo: "" },
    camposEditar: { SKU: "SKU-5", "Data do snapshot": "2026-09-01", Saldo: "12" },
    preencherValido: async (u) => {
      await type(u, "SKU", "SKU-9");
      await type(u, "Data do snapshot", "2026-01-01");
      await type(u, "Saldo", "3");
    },
    payloadNovo: { sku: "SKU-9", data_snapshot: "2026-01-01", saldo: 3 },
    payloadEditar: { saldo: 12 },
  },
  {
    nome: "ProdutoModal",
    Modal: ({ registro, ...p }) => <ProdutoModal {...p} produto={registro as Produto | null} />,
    registro: produto,
    tituloNovo: "Novo Produto",
    tituloEditar: "Editar Produto",
    botaoCriar: "Criar produto",
    campoLivre: "Descricao",
    camposNovo: {
      SKU: "",
      Descricao: "",
      Categoria: "",
      Marca: "",
      "Nota olfativa": "",
      "Preco de tabela": "",
      "Custo unitario": "",
      Unidade: "",
      "Data de lancamento": "",
    },
    camposEditar: {
      Descricao: "Perfume 50",
      Categoria: "Perfumaria",
      Marca: "Marca X",
      "Nota olfativa": "Floral",
      "Preco de tabela": "99.9",
      "Custo unitario": "40",
      Unidade: "UN",
      "Data de lancamento": "2024-03-01",
    },
    preencherValido: async (u) => {
      await type(u, "SKU", "P77");
      await type(u, "Descricao", "Novo");
      await type(u, "Categoria", "Cat");
      await type(u, "Marca", "M");
      await type(u, "Preco de tabela", "10");
      await type(u, "Custo unitario", "4");
      await type(u, "Unidade", "UN");
    },
    payloadNovo: { sku: "P77", preco_tabela: 10, custo_unitario: 4, nota_olfativa: undefined },
    payloadEditar: { sku: "P50", nota_olfativa: "Floral", data_lancamento: "2024-03-01" },
  },
  {
    nome: "UserModal",
    Modal: ({ registro, ...p }) => <UserModal {...p} user={registro as User | null} />,
    registro: usuario,
    tituloNovo: "Novo Usuario",
    tituloEditar: "Editar Usuario",
    botaoCriar: "Criar usuario",
    campoLivre: "Nome completo",
    camposNovo: {
      "Nome completo": "",
      Email: "",
      Perfil: "normal",
      "Vendedor vinculado (opcional)": "",
    },
    camposEditar: {
      "Nome completo": "Bia",
      Email: "bia@x.com",
      Perfil: "admin",
      "Vendedor vinculado (opcional)": "3",
    },
    preencherValido: async (u) => {
      await type(u, "Nome completo", "Caio");
      await type(u, "Email", "caio@x.com");
    },
    payloadNovo: { nome: "Caio", email: "caio@x.com", role: "normal", id_vendedor: null },
    payloadEditar: { nome: "Bia", role: "admin", id_vendedor: 3 },
    pronto: async () => {
      await waitFor(() =>
        expect(screen.getByLabelText("Vendedor vinculado (opcional)")).toBeEnabled()
      );
    },
  },
  {
    nome: "VendedorModal",
    Modal: ({ registro, ...p }) => (
      <VendedorModal {...p} vendedor={registro as VendedorCompleto | null} />
    ),
    registro: vendedor,
    tituloNovo: "Novo Vendedor",
    tituloEditar: "Editar Vendedor",
    botaoCriar: "Criar vendedor",
    campoLivre: "Nome",
    camposNovo: { Nome: "", Regiao: "", UF: "", "Data de admissao": hoje, "Meta mensal": "0" },
    camposEditar: {
      Nome: "Vend 3",
      Regiao: "Sudeste",
      UF: "SP",
      "Data de admissao": "2020-02-02",
      "Meta mensal": "5000",
    },
    preencherValido: async (u) => {
      await type(u, "Nome", "Novo");
      await type(u, "Regiao", "Sul");
      await type(u, "UF", "pr");
    },
    payloadNovo: { nome: "Novo", regiao: "Sul", uf: "PR", meta_mensal: 0 },
    payloadEditar: { nome: "Vend 3", meta_mensal: 5000 },
    // BUG-07: em "editar" o formulario so e preenchido quando o detalhe
    // (GET /api/vendedores/{id}) chega; em "novo" nao ha carregamento.
    pronto: async () => {
      await waitFor(() =>
        expect(screen.queryByText("Carregando dados do vendedor...")).not.toBeInTheDocument()
      );
    },
  },
];

function montar(c: Caso, props: Partial<Props> = {}) {
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
    props: base,
    set: (p: Partial<Props>) => {
      Object.assign(base, p);
      r.rerender(<c.Modal {...base} />);
    },
  };
}

function valor(label: string): string {
  return (screen.getByLabelText(label) as HTMLInputElement).value;
}

beforeEach(() => {
  vi.clearAllMocks();
  Object.values(api).forEach((f) => f.mockReset());
  api.apiListVendedores.mockResolvedValue([
    { id: 3, nome: "Vend 3", regiao: "S", uf: "SP", data_desligamento: null },
  ]);
  api.apiGetVendedor.mockResolvedValue({ ...vendedor, clientes: null });
  api.apiListClientes.mockResolvedValue({ data: [], page: 1, limit: 100, total: 0, pages: 1 });
});

describe.each(CASOS)("$nome (FE-03)", (c) => {
  it("fechado nao renderiza nada", () => {
    montar(c, { open: false });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("abre resetado em modo novo", async () => {
    montar(c);
    await c.pronto?.();
    expect(screen.getByText(c.tituloNovo)).toBeInTheDocument();
    for (const [label, v] of Object.entries(c.camposNovo)) expect(valor(label)).toBe(v);
  });

  it("abre preenchido em modo editar", async () => {
    montar(c, { mode: "edit", registro: c.registro });
    await c.pronto?.();
    expect(screen.getByText(c.tituloEditar)).toBeInTheDocument();
    for (const [label, v] of Object.entries(c.camposEditar)) expect(valor(label)).toBe(v);
  });

  it("reabre resetado depois de fechar (novo sujo -> fechar -> novo)", async () => {
    const m = montar(c);
    await c.pronto?.();
    await userEvent.type(screen.getByLabelText(c.campoLivre), "sujo");
    m.set({ open: false });
    m.set({ open: true });
    await c.pronto?.();
    for (const [label, v] of Object.entries(c.camposNovo)) expect(valor(label)).toBe(v);
  });

  it("editar -> fechar -> novo: nada do registro anterior sobra", async () => {
    const m = montar(c, { mode: "edit", registro: c.registro });
    await c.pronto?.();
    m.set({ open: false });
    m.set({ open: true, mode: "create", registro: null });
    await c.pronto?.();
    expect(screen.getByText(c.tituloNovo)).toBeInTheDocument();
    for (const [label, v] of Object.entries(c.camposNovo)) expect(valor(label)).toBe(v);
  });

  it("editar com o modal aberto e trocar para outro registro re-preenche", async () => {
    const m = montar(c, { mode: "edit", registro: c.registro });
    await c.pronto?.();
    await userEvent.type(screen.getByLabelText(c.campoLivre), "X");
    m.set({ registro: { ...(c.registro as object) } });
    await c.pronto?.();
    for (const [label, v] of Object.entries(c.camposEditar)) expect(valor(label)).toBe(v);
  });

  it("submit em modo novo envia o payload", async () => {
    const m = montar(c);
    await c.pronto?.();
    await c.preencherValido(userEvent.setup());
    await userEvent.click(screen.getByRole("button", { name: c.botaoCriar }));
    await waitFor(() => expect(m.props.onSubmit).toHaveBeenCalledTimes(1));
    expect(m.props.onSubmit).toHaveBeenCalledWith(expect.objectContaining(c.payloadNovo));
  });

  it("submit em modo editar envia o payload", async () => {
    const m = montar(c, { mode: "edit", registro: c.registro });
    await c.pronto?.();
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    await waitFor(() => expect(m.props.onSubmit).toHaveBeenCalledTimes(1));
    expect(m.props.onSubmit).toHaveBeenCalledWith(expect.objectContaining(c.payloadEditar));
  });

  it("erro do submit aparece e some ao reabrir o modal", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("falhou salvar"));
    const m = montar(c, { mode: "edit", registro: c.registro, onSubmit });
    await c.pronto?.();
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText("falhou salvar")).toBeInTheDocument();
    // Botao volta a ficar habilitado (submitting desligado).
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeEnabled();

    m.set({ open: false });
    m.set({ open: true });
    await c.pronto?.();
    expect(screen.queryByText("falhou salvar")).not.toBeInTheDocument();
  });

  it("submit vazio em modo novo mostra validacao e nao chama onSubmit", async () => {
    const m = montar(c);
    await c.pronto?.();
    const form = screen.getByRole("button", { name: c.botaoCriar }).closest("form")!;
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect((await screen.findAllByRole("alert")).length).toBeGreaterThan(0);
    expect(m.props.onSubmit).not.toHaveBeenCalled();
  });
});

// ─── UserModal: carga de vendedores ─────────────────────────────────────────────

describe("UserModal - vendedores", () => {
  it("erro ao carregar vendedores desabilita o select e mostra o motivo", async () => {
    api.apiListVendedores.mockRejectedValueOnce(new Error("x"));
    montar(CASOS[3]);
    expect(
      await screen.findByText("Nao foi possivel carregar a lista de vendedores.")
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Vendedor vinculado (opcional)")).toBeDisabled();
  });

  it("reabrir depois de um erro limpa o erro e busca de novo", async () => {
    api.apiListVendedores.mockRejectedValueOnce(new Error("x"));
    const m = montar(CASOS[3]);
    await screen.findByText("Nao foi possivel carregar a lista de vendedores.");
    m.set({ open: false });
    m.set({ open: true });
    await CASOS[3].pronto!();
    expect(
      screen.queryByText("Nao foi possivel carregar a lista de vendedores.")
    ).not.toBeInTheDocument();
    expect(api.apiListVendedores).toHaveBeenCalledTimes(2);
  });
});

// ─── VendedorModal: detalhe (clientes vinculados) ─────────────────────────────

describe("VendedorModal - clientes vinculados (useAjustarAoMudar)", () => {
  const c = CASOS[4];
  const vinculado: ClienteResumo = {
    id: 44,
    cnpj: "11222333000144",
    razao_social: "Cliente 44",
    segmento: "Varejo",
    cidade: "Campinas",
    uf: "SP",
    carteira_id: 1,
    data_inicio: "2026-01-01",
    data_fim: null,
  };

  it("modo novo nao busca detalhe nem clientes", async () => {
    montar(c);
    await act(async () => {});
    expect(api.apiGetVendedor).not.toHaveBeenCalled();
    expect(api.apiListClientes).not.toHaveBeenCalled();
    expect(screen.queryByText("Clientes vinculados")).not.toBeInTheDocument();
  });

  it("modo editar carrega os vinculados (clientes:null vira lista vazia)", async () => {
    montar(c, { mode: "edit", registro: vendedor });
    expect(
      await screen.findByText("Nenhum cliente vinculado a este vendedor.")
    ).toBeInTheDocument();
    expect(api.apiGetVendedor).toHaveBeenCalledWith(3);
    expect(api.apiListClientes).toHaveBeenCalledWith(1, 100, { ativo: true });
  });

  it("editar -> fechar -> novo: some a lista do vendedor anterior", async () => {
    api.apiGetVendedor.mockResolvedValue({ ...vendedor, clientes: [vinculado] });
    const m = montar(c, { mode: "edit", registro: vendedor });
    expect(await screen.findByText("#44 - Cliente 44")).toBeInTheDocument();
    m.set({ open: false });
    m.set({ open: true, mode: "create", registro: null });
    expect(screen.queryByText("#44 - Cliente 44")).not.toBeInTheDocument();
    expect(screen.queryByText("Clientes vinculados")).not.toBeInTheDocument();
  });

  it("vinculados exibem CNPJ numerico e alfanumerico com mascara (NEG-02)", async () => {
    api.apiGetVendedor.mockResolvedValue({
      ...vendedor,
      clientes: [
        { ...vinculado, cnpj: "11222333000181" },
        { ...vinculado, id: 45, razao_social: "Cliente 45", cnpj: "12ABC34501DE35" },
      ],
    });
    montar(c, { mode: "edit", registro: vendedor });
    expect(await screen.findByText("11.222.333/0001-81")).toBeInTheDocument();
    expect(screen.getByText("12.ABC.345/01DE-35")).toBeInTheDocument();
  });

  it("detalhe obsoleto (fechou antes de responder) nao aparece no proximo vendedor", async () => {
    let resolveAntigo: (v: unknown) => void = () => {};
    api.apiGetVendedor
      .mockReturnValueOnce(new Promise((r) => (resolveAntigo = r)))
      .mockResolvedValueOnce({ ...vendedor, id: 8, nome: "Vend 8", clientes: [] });
    const m = montar(c, { mode: "edit", registro: vendedor });
    m.set({ open: false });
    m.set({ open: true, registro: { ...vendedor, id: 8, nome: "Vend 8" } });
    expect(
      await screen.findByText("Nenhum cliente vinculado a este vendedor.")
    ).toBeInTheDocument();
    await act(async () => resolveAntigo({ ...vendedor, clientes: [vinculado] }));
    expect(screen.queryByText("#44 - Cliente 44")).not.toBeInTheDocument();
  });

  it("erro no detalhe mostra o alerta e desliga o loading", async () => {
    api.apiGetVendedor.mockRejectedValueOnce(new Error("sem detalhe"));
    montar(c, { mode: "edit", registro: vendedor });
    expect(
      await screen.findByText("Nao foi possivel carregar os dados do vendedor: sem detalhe")
    ).toBeInTheDocument();
    expect(document.querySelector(".animate-spin")).toBeNull();
  });
});

// ─── VendedorModal: BUG-07 (formulario preenchido pelo detalhe) ──────────────

describe("VendedorModal - edicao usa o detalhe da API (BUG-07)", () => {
  const c = CASOS[4];
  // Como a pagina /admin/vendedores monta o registro a partir da listagem:
  // data_admissao e meta_mensal sao provisorios.
  const provisorio: VendedorCompleto = {
    ...vendedor,
    id: 4,
    data_admissao: "",
    meta_mensal: 0,
  };
  const detalhe4 = {
    ...vendedor,
    id: 4,
    nome: "Vend 4",
    regiao: "Sul",
    uf: "PR",
    data_admissao: "2023-06-22T00:00:00Z",
    meta_mensal: 55000,
    clientes: [],
  };

  it("preenche o formulario com o detalhe e normaliza a data para AAAA-MM-DD", async () => {
    api.apiGetVendedor.mockResolvedValue(detalhe4);
    montar(c, { mode: "edit", registro: provisorio });
    await c.pronto!();
    expect(api.apiGetVendedor).toHaveBeenCalledWith(4);
    expect(valor("Nome")).toBe("Vend 4");
    expect(valor("Regiao")).toBe("Sul");
    expect(valor("UF")).toBe("PR");
    expect(valor("Data de admissao")).toBe("2023-06-22");
    expect(valor("Meta mensal")).toBe("55000");
  });

  it("salvar sem alterar envia os valores do banco, nao os provisorios", async () => {
    api.apiGetVendedor.mockResolvedValue(detalhe4);
    const m = montar(c, { mode: "edit", registro: provisorio });
    await c.pronto!();
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    await waitFor(() => expect(m.props.onSubmit).toHaveBeenCalledTimes(1));
    expect(m.props.onSubmit).toHaveBeenCalledWith({
      nome: "Vend 4",
      regiao: "Sul",
      uf: "PR",
      data_admissao: "2023-06-22",
      meta_mensal: 55000,
    });
  });

  it("enquanto o detalhe carrega mostra o loading e bloqueia o salvar", async () => {
    let resolver: (v: unknown) => void = () => {};
    api.apiGetVendedor.mockReturnValueOnce(new Promise((r) => (resolver = r)));
    const m = montar(c, { mode: "edit", registro: provisorio });
    expect(screen.getByText("Carregando dados do vendedor...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeDisabled();
    expect(screen.getByLabelText("Data de admissao")).toBeDisabled();
    expect(valor("Data de admissao")).toBe("");
    expect(valor("Meta mensal")).toBe("");

    // Submit forcado (ex.: Enter) tambem nao envia nada.
    const form = screen.getByRole("button", { name: "Salvar alteracoes" }).closest("form")!;
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(m.props.onSubmit).not.toHaveBeenCalled();

    await act(async () => resolver(detalhe4));
    await c.pronto!();
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeEnabled();
    expect(valor("Meta mensal")).toBe("55000");
  });

  it("falha no detalhe mostra o erro e mantem o salvar bloqueado", async () => {
    api.apiGetVendedor.mockRejectedValueOnce(new Error("falhou detalhe"));
    const m = montar(c, { mode: "edit", registro: provisorio });
    expect(
      await screen.findByText("Nao foi possivel carregar os dados do vendedor: falhou detalhe")
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeDisabled();
    const form = screen.getByRole("button", { name: "Salvar alteracoes" }).closest("form")!;
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(m.props.onSubmit).not.toHaveBeenCalled();
  });

  it("trocar de vendedor com o modal aberto volta a bloquear ate o novo detalhe", async () => {
    let resolver: (v: unknown) => void = () => {};
    api.apiGetVendedor
      .mockResolvedValueOnce({ ...vendedor, clientes: [] })
      .mockReturnValueOnce(new Promise((r) => (resolver = r)));
    const m = montar(c, { mode: "edit", registro: vendedor });
    await c.pronto!();
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeEnabled();

    m.set({ registro: provisorio });
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeDisabled();
    expect(valor("Meta mensal")).toBe("");

    await act(async () => resolver(detalhe4));
    await c.pronto!();
    expect(valor("Data de admissao")).toBe("2023-06-22");
    expect(valor("Meta mensal")).toBe("55000");
  });

  // TestBrain: normalizacao de data (toDateInput) e meta nula vindas da API.
  it.each([
    { caso: "AAAA-MM-DD puro", data: "2021-03-15", esperado: "2021-03-15" },
    { caso: "ISO com hora e fuso", data: "2021-03-15T10:20:30-03:00", esperado: "2021-03-15" },
    { caso: "RFC (Date.parse)", data: "Mon, 15 Mar 2021 12:00:00 GMT", esperado: "2021-03-15" },
    { caso: "invalida", data: "nao-e-data", esperado: "" },
    { caso: "vazia", data: "", esperado: "" },
    { caso: "null", data: null, esperado: "" },
  ])("data_admissao $caso -> '$esperado'", async ({ data, esperado }) => {
    api.apiGetVendedor.mockResolvedValue({ ...detalhe4, data_admissao: data });
    montar(c, { mode: "edit", registro: provisorio });
    await c.pronto!();
    expect(valor("Data de admissao")).toBe(esperado);
  });

  it.each([
    { caso: "null", meta: null, esperado: "0" },
    { caso: "undefined", meta: undefined, esperado: "0" },
    { caso: "decimal", meta: 1234.5, esperado: "1234.5" },
  ])("meta_mensal $caso -> '$esperado'", async ({ meta, esperado }) => {
    api.apiGetVendedor.mockResolvedValue({ ...detalhe4, meta_mensal: meta });
    montar(c, { mode: "edit", registro: provisorio });
    await c.pronto!();
    expect(valor("Meta mensal")).toBe(esperado);
  });

  it("detalhe sem data_admissao nao e enviado (campo obrigatorio no modo edicao)", async () => {
    api.apiGetVendedor.mockResolvedValue({ ...detalhe4, data_admissao: null });
    const m = montar(c, { mode: "edit", registro: provisorio });
    await c.pronto!();
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    expect(m.props.onSubmit).not.toHaveBeenCalled();
  });

  it("detalhe antigo que chega depois da troca de vendedor e descartado", async () => {
    let resolverA: (v: unknown) => void = () => {};
    let resolverB: (v: unknown) => void = () => {};
    api.apiGetVendedor
      .mockReturnValueOnce(new Promise((r) => (resolverA = r)))
      .mockReturnValueOnce(new Promise((r) => (resolverB = r)));
    const m = montar(c, { mode: "edit", registro: vendedor });
    m.set({ registro: provisorio });

    // Resposta atrasada do vendedor 3 nao pode preencher o form do vendedor 4.
    await act(async () => resolverA({ ...vendedor, clientes: [] }));
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeDisabled();
    expect(valor("Meta mensal")).toBe("");
    expect(valor("Data de admissao")).toBe("");

    await act(async () => resolverB(detalhe4));
    await c.pronto!();
    expect(valor("Nome")).toBe("Vend 4");
    expect(valor("Meta mensal")).toBe("55000");
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeEnabled();
  });

  it("fechar e reabrir o mesmo vendedor busca o detalhe de novo e rebloqueia", async () => {
    api.apiGetVendedor.mockResolvedValue(detalhe4);
    const m = montar(c, { mode: "edit", registro: provisorio });
    await c.pronto!();
    m.set({ open: false });
    let resolver: (v: unknown) => void = () => {};
    api.apiGetVendedor.mockReturnValueOnce(new Promise((r) => (resolver = r)));
    m.set({ open: true });
    expect(screen.getByRole("button", { name: "Salvar alteracoes" })).toBeDisabled();
    expect(valor("Meta mensal")).toBe("");
    await act(async () => resolver({ ...detalhe4, meta_mensal: 77000 }));
    await c.pronto!();
    expect(valor("Meta mensal")).toBe("77000");
    expect(api.apiGetVendedor).toHaveBeenCalledTimes(2);
  });
});

// ─── VendedorModal: validacao do submit e vinculo de clientes (TestBrain) ────

describe("VendedorModal - validacao e vinculos (edicao)", () => {
  const c = CASOS[4];
  const resumo: ClienteResumo = {
    id: 10,
    cnpj: "11222333000181",
    razao_social: "Loja A",
    segmento: "",
    cidade: "Campinas",
    uf: "",
    carteira_id: 9,
    data_inicio: "2026-02-03T00:00:00Z",
    data_fim: null,
  };

  async function abrirEdicao(clientes: ClienteResumo[] = []) {
    api.apiGetVendedor.mockResolvedValue({ ...vendedor, clientes });
    const m = montar(c, { mode: "edit", registro: vendedor });
    await c.pronto!();
    return m;
  }

  function submeter() {
    const form = screen.getByRole("button", { name: "Salvar alteracoes" }).closest("form")!;
    return act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
  }

  it.each([
    { campo: "Nome", v: "   ", erro: "Nome e obrigatorio." },
    { campo: "Regiao", v: "   ", erro: "Regiao e obrigatoria." },
    { campo: "UF", v: "S", erro: "UF deve ter 2 letras." },
    { campo: "Data de admissao", v: "", erro: "Data de admissao e obrigatoria." },
    { campo: "Meta mensal", v: "-1", erro: "Meta mensal deve ser um numero maior ou igual a zero." },
  ])("submit com $campo invalido mostra '$erro'", async ({ campo, v, erro }) => {
    const m = await abrirEdicao();
    const input = screen.getByLabelText(campo);
    await userEvent.clear(input);
    if (v) await userEvent.type(input, v);
    await submeter();
    expect(await screen.findByText(erro)).toBeInTheDocument();
    expect(m.props.onSubmit).not.toHaveBeenCalled();
  });

  it.each([
    { caso: "Error", rej: new Error("conflito"), msg: "conflito" },
    { caso: "nao-Error", rej: "x", msg: "Erro ao salvar vendedor." },
  ])("falha no onSubmit ($caso) mostra a mensagem", async ({ rej, msg }) => {
    const m = await abrirEdicao();
    (m.props.onSubmit as ReturnType<typeof vi.fn>).mockRejectedValueOnce(rej);
    await userEvent.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });

  it.each([
    { caso: "Error", rej: new Error("lista off"), msg: "Nao foi possivel carregar os clientes disponiveis: lista off" },
    { caso: "nao-Error", rej: "x", msg: "Nao foi possivel carregar os clientes disponiveis: Erro ao carregar clientes disponiveis." },
  ])("falha ao listar clientes ($caso) mostra alerta", async ({ rej, msg }) => {
    api.apiListClientes.mockRejectedValueOnce(rej);
    await abrirEdicao();
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });

  it("vincula cliente: some do combo e entra na tabela", async () => {
    api.apiListClientes.mockResolvedValue({ data: [cliente], page: 1, limit: 100, total: 1, pages: 1 });
    api.apiVincularCliente.mockResolvedValueOnce(resumo);
    await abrirEdicao();
    const select = await screen.findByLabelText("Vincular cliente");
    await waitFor(() => expect(screen.getByRole("option", { name: "#10 - Loja A" })).toBeInTheDocument());
    await userEvent.selectOptions(select, "10");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar" }));
    expect(api.apiVincularCliente).toHaveBeenCalledWith(3, 10);
    expect(await screen.findByText("#10 - Loja A")).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "#10 - Loja A" })).not.toBeInTheDocument();
    // Vincular nao mexe nos campos do formulario (BUG-07).
    expect(valor("Meta mensal")).toBe("5000");
    expect(valor("Data de admissao")).toBe("2020-02-02");
  });

  it.each([
    { caso: "Error", rej: new Error("ja vinculado"), msg: "ja vinculado" },
    { caso: "nao-Error", rej: "x", msg: "Erro ao vincular cliente ao vendedor." },
  ])("falha ao vincular ($caso) mostra erro", async ({ rej, msg }) => {
    api.apiListClientes.mockResolvedValue({ data: [cliente], page: 1, limit: 100, total: 1, pages: 1 });
    api.apiVincularCliente.mockRejectedValueOnce(rej);
    await abrirEdicao();
    await waitFor(() => expect(screen.getByRole("option", { name: "#10 - Loja A" })).toBeInTheDocument());
    await userEvent.selectOptions(screen.getByLabelText("Vincular cliente"), "10");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar" }));
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });

  it("remover vinculo: cancelar no confirm nao chama a API", async () => {
    const conf = vi.spyOn(window, "confirm").mockReturnValue(false);
    await abrirEdicao([resumo]);
    await userEvent.click(await screen.findByRole("button", { name: "Remover" }));
    expect(api.apiDesvincularCliente).not.toHaveBeenCalled();
    conf.mockRestore();
  });

  it("remover vinculo confirmado tira da tabela sem mexer no formulario", async () => {
    const conf = vi.spyOn(window, "confirm").mockReturnValue(true);
    api.apiDesvincularCliente.mockResolvedValueOnce(undefined);
    await abrirEdicao([resumo]);
    await userEvent.click(await screen.findByRole("button", { name: "Remover" }));
    expect(api.apiDesvincularCliente).toHaveBeenCalledWith(3, 10);
    expect(await screen.findByText("Nenhum cliente vinculado a este vendedor.")).toBeInTheDocument();
    expect(valor("Meta mensal")).toBe("5000");
    conf.mockRestore();
  });

  it.each([
    { caso: "Error", rej: new Error("bloqueado"), msg: "bloqueado" },
    { caso: "nao-Error", rej: "x", msg: "Erro ao encerrar o vinculo com o cliente." },
  ])("falha ao remover vinculo ($caso) mostra erro", async ({ rej, msg }) => {
    const conf = vi.spyOn(window, "confirm").mockReturnValue(true);
    api.apiDesvincularCliente.mockRejectedValueOnce(rej);
    await abrirEdicao([resumo]);
    await userEvent.click(await screen.findByRole("button", { name: "Remover" }));
    expect(await screen.findByText(msg)).toBeInTheDocument();
    expect(screen.getByText("#10 - Loja A")).toBeInTheDocument();
    conf.mockRestore();
  });
});
