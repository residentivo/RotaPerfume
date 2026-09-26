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

import { ClienteModal } from "./ClienteModal";
import { EstoqueModal } from "./EstoqueModal";
import { ProdutoModal } from "./ProdutoModal";
import { UserModal } from "./UserModal";
import { VendedorModal } from "./VendedorModal";

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
      await screen.findByText("Nao foi possivel carregar os clientes vinculados: sem detalhe")
    ).toBeInTheDocument();
    expect(document.querySelector(".animate-spin")).toBeNull();
  });
});
