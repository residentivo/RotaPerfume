/**
 * FE-18: o botao de ativo de clientes e usuarios fica bloqueado por 1500 ms
 * depois de cada alteracao (mesmo padrao do BUG-17 em produtos). Clique duplo
 * dentro de 1500 ms gera uma so chamada; depois de 1500 ms, uma nova.
 */
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Cliente, MeResponse, User } from "@/lib/types";

vi.mock("@/components/layout/ProtectedRoute", () => ({
  ProtectedRoute: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/layout/CarteiraGuard", () => ({
  CarteiraGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

const ADMIN: MeResponse = { id: 1, nome: "Adm", email: "a@x", role: "admin", ativo: true, id_vendedor: null };
vi.mock("@/lib/session", () => ({
  useSessionUser: () => ADMIN,
  useVendedorDesligado: () => false,
}));

const api = vi.hoisted(() => ({
  apiListUsers: vi.fn(),
  apiCreateUser: vi.fn(),
  apiUpdateUser: vi.fn(),
  apiToggleUserStatus: vi.fn(),
  apiAdminResetPassword: vi.fn(),
  apiListVendedores: vi.fn(),
  apiListClientes: vi.fn(),
  apiToggleClienteStatus: vi.fn(),
  apiCreateCliente: vi.fn(),
  apiUpdateCliente: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import UsuariosPage from "@/app/admin/usuarios/page";
import ClientesPage from "@/app/admin/clientes/page";

function pagina<T>(data: T[]) {
  return { data, page: 1, limit: 20, total: data.length, pages: 1 };
}

const bia = { id: 2, nome: "Bia", email: "bia@x", role: "normal", ativo: true, id_vendedor: null } as User;
const caio = { ...bia, id: 3, nome: "Caio", email: "caio@x" } as User;

function cliente(id: number, razao: string): Cliente {
  return {
    cliente_id_origem: id,
    razao_social: razao,
    cnpj: "11222333000181",
    segmento: "Varejo",
    cidade: "Sao Paulo",
    uf: "SP",
    bairro: "Centro",
    data_cadastro: "2026-01-10",
    ativo: true,
  } as Cliente;
}
const lojaA = cliente(10, "Loja A");
const lojaB = cliente(11, "Loja B");

type Caso = {
  nome: string;
  toggle: ReturnType<typeof vi.fn>;
  montar: () => ReturnType<typeof render>;
  preparar: (itens: "um" | "dois") => void;
  texto: string;
  outroTexto: string;
  id: number;
  outroId: number;
  inativado: string;
  reativado: string;
};

const CASOS: Caso[] = [
  {
    nome: "clientes",
    toggle: api.apiToggleClienteStatus,
    montar: () => render(<ClientesPage />),
    preparar: (itens) => {
      api.apiListClientes.mockResolvedValue(pagina(itens === "um" ? [lojaA] : [lojaA, lojaB]));
      api.apiToggleClienteStatus.mockImplementation(async (id: number, ativo: boolean) => ({
        ...(id === 10 ? lojaA : lojaB),
        ativo,
      }));
    },
    texto: "Loja A",
    outroTexto: "Loja B",
    id: 10,
    outroId: 11,
    inativado: "Cliente inativado com sucesso.",
    reativado: "Cliente reativado com sucesso.",
  },
  {
    nome: "usuarios",
    toggle: api.apiToggleUserStatus,
    montar: () => render(<UsuariosPage />),
    preparar: (itens) => {
      api.apiListUsers.mockResolvedValue(pagina(itens === "um" ? [bia] : [bia, caio]));
      api.apiToggleUserStatus.mockImplementation(async (id: number, ativo: boolean) => ({
        ...(id === 2 ? bia : caio),
        ativo,
      }));
    },
    texto: "Bia",
    outroTexto: "Caio",
    id: 2,
    outroId: 3,
    inativado: "Usuario inativado com sucesso.",
    reativado: "Usuario reativado com sucesso.",
  },
];

beforeEach(() => {
  vi.clearAllMocks();
  Object.values(api).forEach((f) => f.mockReset());
  api.apiListVendedores.mockResolvedValue([]);
  api.apiListClientes.mockResolvedValue(pagina([]));
  api.apiListUsers.mockResolvedValue(pagina([]));
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

async function montar(caso: Caso, itens: "um" | "dois" = "um") {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const u = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  vi.spyOn(window, "confirm").mockReturnValue(true);
  caso.preparar(itens);
  const r = caso.montar();
  await screen.findByText(caso.texto);
  // Janela do debounce dos filtros.
  await act(async () => {
    vi.advanceTimersByTime(420);
  });
  return { u, r };
}

function confirmMock() {
  return window.confirm as unknown as ReturnType<typeof vi.fn>;
}

describe.each(CASOS)("FE-18 - bloqueio do toggle de ativo ($nome)", (caso) => {
  it("clique duplo em menos de 1500 ms -> 1 chamada; depois de 1500 ms -> nova chamada", async () => {
    const { u } = await montar(caso);

    await u.click(screen.getByTitle("Clique para inativar"));
    await screen.findByText(caso.inativado);
    expect(caso.toggle).toHaveBeenCalledTimes(1);
    expect(caso.toggle).toHaveBeenNthCalledWith(1, caso.id, false);

    // Segundo clique dentro da janela: nem confirmacao nem API.
    const botao = screen.getByTitle("Clique para reativar");
    expect(botao).toBeDisabled();
    confirmMock().mockClear();
    await act(async () => {
      vi.advanceTimersByTime(1000);
    });
    await act(async () => {
      botao.click();
    });
    expect(window.confirm).not.toHaveBeenCalled();
    expect(caso.toggle).toHaveBeenCalledTimes(1);

    // shouldAdvanceTime deixa o relogio real correr alguns ms: margem.
    await act(async () => {
      vi.advanceTimersByTime(300);
    });
    expect(screen.getByTitle("Clique para reativar")).toBeDisabled();
    await act(async () => {
      vi.advanceTimersByTime(250);
    });
    expect(screen.getByTitle("Clique para reativar")).toBeEnabled();

    await u.click(screen.getByTitle("Clique para reativar"));
    await screen.findByText(caso.reativado);
    expect(caso.toggle).toHaveBeenCalledTimes(2);
    expect(caso.toggle).toHaveBeenNthCalledWith(2, caso.id, true);
    expect(screen.getByTitle("Clique para inativar")).toBeDisabled();
  });

  it("dois cliques no mesmo tick (antes do re-render) -> 1 chamada", async () => {
    let resolver: (v: unknown) => void = () => {};
    await montar(caso);
    caso.toggle.mockImplementationOnce(() => new Promise((r) => (resolver = r)));
    const botao = screen.getByTitle("Clique para inativar");
    await act(async () => {
      botao.click();
      botao.click();
    });
    expect(window.confirm).toHaveBeenCalledTimes(1);
    expect(caso.toggle).toHaveBeenCalledTimes(1);
    await act(async () => {
      resolver({ ...(caso.nome === "clientes" ? lojaA : bia), ativo: false });
    });
    await screen.findByText(caso.inativado);
    expect(caso.toggle).toHaveBeenCalledTimes(1);
  });

  it("tambem bloqueia depois de erro da API", async () => {
    await montar(caso);
    caso.toggle.mockRejectedValueOnce(new Error("falhou"));
    await act(async () => {
      screen.getByTitle("Clique para inativar").click();
    });
    await screen.findByText("falhou");
    expect(screen.getByTitle("Clique para inativar")).toBeDisabled();
    await act(async () => {
      vi.advanceTimersByTime(1600);
    });
    expect(screen.getByTitle("Clique para inativar")).toBeEnabled();
  });

  it("cancelar a confirmacao nao bloqueia", async () => {
    const { u } = await montar(caso);
    confirmMock().mockReturnValue(false);
    await u.click(screen.getByTitle("Clique para inativar"));
    expect(caso.toggle).not.toHaveBeenCalled();
    expect(screen.getByTitle("Clique para inativar")).toBeEnabled();
    confirmMock().mockReturnValue(true);
    await u.click(screen.getByTitle("Clique para inativar"));
    expect(caso.toggle).toHaveBeenCalledTimes(1);
  });

  it("o bloqueio e por item: outro item continua clicavel", async () => {
    const { u } = await montar(caso, "dois");
    const linha = screen.getByText(caso.texto).closest("tr")!;
    const outra = screen.getByText(caso.outroTexto).closest("tr")!;
    await u.click(within(linha).getByTitle("Clique para inativar"));
    await screen.findByText(caso.inativado);
    expect(within(linha).getByTitle("Clique para reativar")).toBeDisabled();
    expect(within(outra).getByTitle("Clique para inativar")).toBeEnabled();
    await u.click(within(outra).getByTitle("Clique para inativar"));
    expect(caso.toggle).toHaveBeenLastCalledWith(caso.outroId, false);
  });

  it("unmount durante o bloqueio limpa o timer", async () => {
    const erro = vi.spyOn(console, "error");
    const { u, r } = await montar(caso);
    await u.click(screen.getByTitle("Clique para inativar"));
    await screen.findByText(caso.inativado);
    await act(async () => {
      screen.getByRole("button", { name: "Fechar alerta" }).click();
    });
    expect(vi.getTimerCount()).toBe(1);
    r.unmount();
    expect(vi.getTimerCount()).toBe(0);
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(erro).not.toHaveBeenCalled();
  });
});
