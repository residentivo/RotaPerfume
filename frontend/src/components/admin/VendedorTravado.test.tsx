/**
 * SEC-03: para o usuario normal, GET /api/vendedores devolve so o proprio
 * vendedor ([] sem vinculo). Nos modais com cascata vendedor -> cliente
 * (OportunidadeModal e VisitaModal), o vendedor do usuario normal vem
 * pre-selecionado e travado pela sessao (/me); sem vinculo, o modal avisa e
 * nao deixa salvar. Admin e sessao ainda vazia continuam com o select livre.
 */
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentType } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ClienteResumo, MeResponse, Oportunidade, Vendedor, Visita } from "@/lib/types";

const api = vi.hoisted(() => ({
  apiListVendedores: vi.fn(),
  apiListClientesDoVendedor: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

const sessao = vi.hoisted(() => ({ user: null as MeResponse | null }));
vi.mock("@/lib/session", () => ({
  useSessionUser: () => sessao.user,
}));

import { OportunidadeModal } from "./OportunidadeModal";
import { VisitaModal } from "./VisitaModal";

const VEND_7: Vendedor = { id: 7, nome: "Vend 7", regiao: "S", uf: "SP", data_desligamento: null };
const VEND_3: Vendedor = { id: 3, nome: "Vend 3", regiao: "S", uf: "SP", data_desligamento: null };

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

function usuario(extra: Partial<MeResponse>): MeResponse {
  return { id: 1, nome: "Ana", email: "a@x", role: "normal", ativo: true, ...extra };
}

const oportunidade = {
  oportunidade_id: 1,
  cliente_id: 301,
  vendedor_id: 3,
  origem: "Instagram",
  data_abertura: "2026-08-01",
  etapa: "Negociação",
  probabilidade_pct: 60,
  valor_estimado: 1500,
  data_fechamento: null,
  ciclo_dias: null,
  motivo_perda: null,
} as unknown as Oportunidade;

const visita = {
  visita_id: 1,
  cliente_id: 301,
  vendedor_id: 3,
  data_visita: "2026-08-15",
  resultado: "Reagendada",
  duracao_min: 45,
} as unknown as Visita;

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
  botaoCriar: string;
  aviso: RegExp;
}

const CASOS: Caso[] = [
  {
    nome: "OportunidadeModal",
    Modal: ({ registro, ...p }) => (
      <OportunidadeModal {...p} oportunidade={registro as Oportunidade | null} />
    ),
    registro: oportunidade,
    botaoCriar: "Criar oportunidade",
    aviso: /possivel registrar oportunidades/,
  },
  {
    nome: "VisitaModal",
    Modal: ({ registro, ...p }) => <VisitaModal {...p} visita={registro as Visita | null} />,
    registro: visita,
    botaoCriar: "Criar visita",
    aviso: /possivel registrar visitas/,
  },
];

function sel(label: string): HTMLSelectElement {
  return screen.getByLabelText(label) as HTMLSelectElement;
}

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
  return { ...r, props: base };
}

beforeEach(() => {
  vi.clearAllMocks();
  Object.values(api).forEach((f) => f.mockReset());
  sessao.user = null;
  api.apiListClientesDoVendedor.mockImplementation(async (id: number) => [cliente(id * 100 + 1)]);
});

describe.each(CASOS)("$nome - vendedor do usuario normal (SEC-03)", (c) => {
  it("normal com vendedor (lista com 1 item): pre-seleciona, trava e carrega a carteira", async () => {
    sessao.user = usuario({ id_vendedor: 7 });
    api.apiListVendedores.mockResolvedValue([VEND_7]);
    montar(c);

    await screen.findByRole("option", { name: "#701 - Cliente 701" });
    expect(sel("Vendedor").value).toBe("7");
    expect(sel("Vendedor")).toBeDisabled();
    expect(screen.getAllByRole("option", { name: "#7 - Vend 7" })).toHaveLength(1);
    expect(api.apiListClientesDoVendedor).toHaveBeenCalledWith(7);
    expect(screen.queryByText(c.aviso)).not.toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: c.botaoCriar })).toBeEnabled());
  });

  it("normal com vendedor: o proprio vendedor aparece mesmo antes da lista carregar", async () => {
    sessao.user = usuario({ id_vendedor: 7, vendedor_nome: "Vend 7" });
    api.apiListVendedores.mockReturnValue(new Promise(() => {}));
    montar(c);
    expect(sel("Vendedor").value).toBe("7");
    expect(screen.getByRole("option", { name: "#7 - Vend 7" })).toBeInTheDocument();
  });

  it("normal sem vendedor (lista vazia): aviso, 'Sem vendedor vinculado' e salvar desabilitado", async () => {
    sessao.user = usuario({ id_vendedor: null });
    api.apiListVendedores.mockResolvedValue([]);
    const m = montar(c);

    expect(await screen.findByText(c.aviso)).toBeInTheDocument();
    await waitFor(() => expect(api.apiListVendedores).toHaveBeenCalled());
    expect(screen.getByRole("option", { name: "Sem vendedor vinculado" })).toBeInTheDocument();
    expect(sel("Vendedor")).toBeDisabled();
    expect(sel("Cliente")).toBeDisabled();
    const botao = screen.getByRole("button", { name: c.botaoCriar });
    await waitFor(() => expect(botao).toBeDisabled());
    expect(api.apiListClientesDoVendedor).not.toHaveBeenCalled();

    // Mesmo forcando o submit do form, nada e enviado.
    await act(async () => {
      botao.closest("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(m.props.onSubmit).not.toHaveBeenCalled();
    expect(screen.getByText(/nao esta vinculado a um vendedor\. Solicite/)).toBeInTheDocument();
  });

  it("normal editando: usa o vendedor da sessao, nao o do registro", async () => {
    sessao.user = usuario({ id_vendedor: 7 });
    api.apiListVendedores.mockResolvedValue([VEND_7]);
    montar(c, { mode: "edit", registro: c.registro });
    await screen.findByRole("option", { name: "#701 - Cliente 701" });
    expect(sel("Vendedor").value).toBe("7");
    expect(sel("Vendedor")).toBeDisabled();
  });

  it("vinculo chega depois de abrir (/me): o select passa a ficar travado no vendedor", async () => {
    sessao.user = usuario({ id_vendedor: null });
    api.apiListVendedores.mockResolvedValue([]);
    const m = montar(c);
    await screen.findByText(c.aviso);

    sessao.user = usuario({ id_vendedor: 7, vendedor_nome: "Vend 7" });
    m.rerender(<c.Modal {...m.props} />);
    await waitFor(() => expect(sel("Vendedor").value).toBe("7"));
    expect(screen.queryByText(c.aviso)).not.toBeInTheDocument();
  });

  it("admin: select livre com todos os vendedores", async () => {
    sessao.user = usuario({ role: "admin", id_vendedor: null });
    api.apiListVendedores.mockResolvedValue([VEND_3, VEND_7]);
    montar(c);
    await waitFor(() => expect(sel("Vendedor")).toBeEnabled());
    expect(sel("Vendedor").value).toBe("");
    expect(screen.getByRole("option", { name: "Selecione um vendedor" })).toBeInTheDocument();
    await userEvent.selectOptions(sel("Vendedor"), "3");
    await screen.findByRole("option", { name: "#301 - Cliente 301" });
    expect(screen.queryByText(c.aviso)).not.toBeInTheDocument();
  });

  it("sessao ainda sem /me: nao trava nem avisa (o backend valida o vendedor)", async () => {
    api.apiListVendedores.mockResolvedValue([VEND_7]);
    montar(c);
    await waitFor(() => expect(sel("Vendedor")).toBeEnabled());
    expect(sel("Vendedor").value).toBe("");
    expect(screen.queryByText(c.aviso)).not.toBeInTheDocument();
  });

  it("403 de vendedor desligado na lista mostra o erro e nao quebra o modal", async () => {
    sessao.user = usuario({ id_vendedor: 7 });
    api.apiListVendedores.mockRejectedValue(new Error("acesso bloqueado: vendedor desligado"));
    montar(c);
    expect(
      await screen.findByText(/Nao foi possivel carregar vendedores: acesso bloqueado/)
    ).toBeInTheDocument();
    expect(sel("Vendedor").value).toBe("7");
  });
});
