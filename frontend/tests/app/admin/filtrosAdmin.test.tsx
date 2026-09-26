/**
 * Filtros, ordenacao, paginacao e renderizacao condicional das telas
 * administrativas com busca client-side: senha-historico, usuarios e
 * vendedores. Complementa listasAdmin (FE-03/FE-04/FE-09) e crudAdmin (CRUD)
 * cobrindo os ramos de cada filtro, rotulos/badges e mensagens de erro padrao.
 */
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SenhaHistoricoItem, TipoReset, User, Vendedor } from "@/lib/types";

vi.mock("@/components/layout/ProtectedRoute", () => ({
  ProtectedRoute: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

const api = vi.hoisted(() => ({
  apiListUsers: vi.fn(),
  apiCreateUser: vi.fn(),
  apiUpdateUser: vi.fn(),
  apiToggleUserStatus: vi.fn(),
  apiAdminResetPassword: vi.fn(),
  apiListSenhaHistorico: vi.fn(),
  apiListVendedores: vi.fn(),
  apiCreateVendedor: vi.fn(),
  apiUpdateVendedor: vi.fn(),
  apiDeleteVendedor: vi.fn(),
  apiReativarVendedor: vi.fn(),
  apiGetVendedor: vi.fn(),
  apiListClientes: vi.fn(),
  apiVincularCliente: vi.fn(),
  apiDesvincularCliente: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import SenhaHistoricoPage from "@/app/admin/senha-historico/page";
import UsuariosPage from "@/app/admin/usuarios/page";
import VendedoresPage from "@/app/admin/vendedores/page";

function botaoCabecalho(header: string): HTMLElement {
  const th = screen
    .getAllByRole("columnheader")
    .find((el) => el.textContent?.replace(/[↑↓↕]/g, "").trim() === header);
  if (!th) throw new Error(`cabecalho ${header} nao encontrado`);
  return within(th).getByRole("button");
}

/** Textos da coluna `idx` (0-based) em todas as linhas de dados. */
function coluna(idx: number): string[] {
  return screen
    .getAllByRole("row")
    .map((r) => within(r).queryAllByRole("cell"))
    .filter((cells) => cells.length > idx)
    .map((cells) => (cells[idx].textContent ?? "").trim());
}

function linhaDe(texto: string): HTMLElement {
  return screen.getByText(texto).closest("tr")!;
}

beforeEach(() => {
  vi.clearAllMocks();
  Object.values(api).forEach((f) => f.mockReset());
  api.apiListVendedores.mockResolvedValue([]);
  api.apiListClientes.mockResolvedValue({ data: [], page: 1, limit: 20, total: 0, pages: 0 });
});

// ─── Historico de senha ───────────────────────────────────────────────────────

function hist(id: number, extra: Partial<SenhaHistoricoItem> = {}): SenhaHistoricoItem {
  return {
    id,
    usuario_id: id,
    usuario_nome: `Pessoa ${id}`,
    resetado_por_id: null,
    resetado_por_nome: null,
    tipo_reset: "admin",
    ip_origem: "10.0.0.1",
    created_at: "2026-09-01T10:00:00Z",
    ...extra,
  };
}

function paginaHist(data: SenhaHistoricoItem[], total = data.length, pages = 1) {
  return { data, page: 1, limit: 20, total, pages };
}

describe("Senha historico - renderizacao das colunas", () => {
  it.each<[TipoReset | string, string, string]>([
    ["usuario", "Proprio", "bg-blue-100"],
    ["admin", "Admin", "bg-yellow-100"],
    ["primeiro_acesso", "Primeiro Acesso", "bg-green-100"],
    ["esquecimento", "Esquecimento", "bg-red-100"],
    ["tipo_novo", "tipo_novo", "bg-slate-100"],
  ])("tipo %s -> badge '%s' (%s)", async (tipo, rotulo, cor) => {
    api.apiListSenhaHistorico.mockResolvedValue(paginaHist([hist(1, { tipo_reset: tipo as TipoReset })]));
    render(<SenhaHistoricoPage />);
    const linha = (await screen.findByText("#1 - Pessoa 1")).closest("tr")!;
    const badge = within(linha).getByText(rotulo);
    expect(badge).toHaveClass(cor);
  });

  it.each<[string, string, (t: string) => boolean]>([
    ["data vazia", "", (t) => t === "-"],
    ["data invalida", "nao-e-data", (t) => t === "nao-e-data"],
    ["data valida", "2026-09-01T10:00:00Z", (t) => /^\d{2}\/\d{2}\/2026/.test(t)],
  ])("%s na coluna Data/Hora", async (_n, created_at, confere) => {
    api.apiListSenhaHistorico.mockResolvedValue(paginaHist([hist(1, { created_at })]));
    render(<SenhaHistoricoPage />);
    await screen.findByText("#1 - Pessoa 1");
    expect(confere(coluna(1)[0])).toBe(true);
  });

  it("sem nome, sem resetador e sem IP mostra placeholders; com resetador mostra #id - nome", async () => {
    api.apiListSenhaHistorico.mockResolvedValue(
      paginaHist([
        hist(1, { usuario_nome: "", ip_origem: null as unknown as string }),
        hist(2, { resetado_por_id: 9, resetado_por_nome: "Chefe", ip_origem: "192.168.0.5" }),
      ])
    );
    render(<SenhaHistoricoPage />);
    expect(await screen.findByText("#1 - (sem nome)")).toBeInTheDocument();
    const l1 = linhaDe("#1 - (sem nome)");
    const cells1 = within(l1).getAllByRole("cell");
    expect(cells1[4]).toHaveTextContent(/^-$/);
    expect(cells1[5]).toHaveTextContent(/^-$/);

    const l2 = linhaDe("#2 - Pessoa 2");
    expect(within(l2).getByText("#9 - Chefe")).toBeInTheDocument();
    expect(within(l2).getByText("192.168.0.5")).toBeInTheDocument();
  });

  it.each<[number, number, string]>([
    [1, 1, "1-1 de 1 registro"],
    [60, 3, "1-20 de 60 registros"],
    [0, 0, "0 registros"],
  ])("total %i -> contador '%s'", async (total, pages, texto) => {
    api.apiListSenhaHistorico.mockResolvedValue(paginaHist(total ? [hist(1)] : [], total, pages));
    render(<SenhaHistoricoPage />);
    expect(await screen.findByText(texto)).toBeInTheDocument();
  });
});

describe("Senha historico - busca local", () => {
  const itens = [
    hist(11, { usuario_nome: "Ana", resetado_por_id: 1, resetado_por_nome: "Chefe Carlos", ip_origem: "10.1.1.1" }),
    hist(22, { usuario_nome: "Bruno", ip_origem: "172.16.0.9" }),
    hist(33, { usuario_nome: null as unknown as string, ip_origem: null as unknown as string }),
  ];

  beforeEach(() => {
    api.apiListSenhaHistorico.mockResolvedValue(paginaHist(itens));
  });

  it.each<[string, string, number[]]>([
    ["nome do usuario", "bruno", [22]],
    ["nome do resetador", "carlos", [11]],
    ["IP de origem", "172.16", [22]],
    ["ID do usuario", "33", [33]],
    ["termo com espacos e maiusculas", "  ANA ", [11]],
  ])("filtra por %s", async (_n, termo, ids) => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#11 - Ana");
    await userEvent.type(screen.getByLabelText("Buscar"), termo);
    await waitFor(() => expect(coluna(0)).toEqual(ids.map((id) => `#${id}`)));
  });

  it("sem resultado com filtro mostra a mensagem de filtros", async () => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#11 - Ana");
    await userEvent.type(screen.getByLabelText("Buscar"), "zzz");
    expect(await screen.findByText("Nenhum registro encontrado para os filtros aplicados.")).toBeInTheDocument();
  });
});

describe("Senha historico - ordenacao, limite e paginacao", () => {
  beforeEach(() => {
    api.apiListSenhaHistorico.mockImplementation(async (page: number, limit: number) => ({
      data: [hist(page * 100)],
      page,
      limit,
      total: 60,
      pages: 3,
    }));
  });

  it("clicar na coluna ja ordenada (data desc) inverte para asc", async () => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#100 - Pessoa 100");
    await userEvent.click(botaoCabecalho("Data/Hora"));
    await waitFor(() =>
      expect(api.apiListSenhaHistorico).toHaveBeenLastCalledWith(1, 20, undefined, undefined, "created_at", "asc")
    );
  });

  it.each([["Usuario"], ["Resetado Por"], ["IP Origem"]])(
    "coluna %s (fora da whitelist) nao envia order_by",
    async (header) => {
      render(<SenhaHistoricoPage />);
      await screen.findByText("#100 - Pessoa 100");
      await userEvent.click(botaoCabecalho(header));
      await waitFor(() =>
        expect(api.apiListSenhaHistorico).toHaveBeenLastCalledWith(1, 20, undefined, undefined, undefined, "asc")
      );
    }
  );

  it("mudar itens por pagina busca com o novo limite", async () => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#100 - Pessoa 100");
    await userEvent.selectOptions(screen.getByLabelText("Itens por pagina"), "50");
    await waitFor(() =>
      expect(api.apiListSenhaHistorico).toHaveBeenLastCalledWith(1, 50, undefined, undefined, "created_at", "desc")
    );
  });

  it("navega por ultima, anterior e primeira pagina", async () => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#100 - Pessoa 100");
    expect(screen.getByTitle("Primeira pagina")).toBeDisabled();

    await userEvent.click(screen.getByTitle("Ultima pagina"));
    expect(await screen.findByText("#300 - Pessoa 300")).toBeInTheDocument();
    expect(screen.getByTitle("Proxima pagina")).toBeDisabled();

    await userEvent.click(screen.getByTitle("Pagina anterior"));
    expect(await screen.findByText("#200 - Pessoa 200")).toBeInTheDocument();

    await userEvent.click(screen.getByTitle("Primeira pagina"));
    expect(await screen.findByText("#100 - Pessoa 100")).toBeInTheDocument();
    expect(screen.getByText(/Pagina/, { selector: "div" })).toHaveTextContent("Pagina 1 de 3");
  });
});

describe("Senha historico - erros", () => {
  it("rejeicao que nao e Error usa a mensagem padrao, e o alerta pode ser fechado", async () => {
    api.apiListSenhaHistorico.mockRejectedValue("falhou");
    render(<SenhaHistoricoPage />);
    const msg = await screen.findByText(/Erro ao carregar historico de senhas/);
    expect(msg).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Fechar alerta" }));
    expect(screen.queryByText(/Erro ao carregar historico de senhas/)).not.toBeInTheDocument();
  });
});

// ─── Usuarios ─────────────────────────────────────────────────────────────────

function usuario(id: number, extra: Partial<User> = {}): User {
  return {
    id,
    nome: `Usuario ${id}`,
    email: `u${id}@x.com`,
    role: "normal",
    ativo: true,
    id_vendedor: null,
    ...extra,
  };
}

const USUARIOS: User[] = [
  usuario(1, { nome: "Ana Admin", email: "ana@empresa.com", role: "admin" }),
  usuario(2, { nome: "Bia", email: "bia@loja.com", ativo: false }),
  usuario(3, { nome: "Caio", email: "caio@loja.com", id_vendedor: 7, vendedor_nome: "Vend Sete" }),
  usuario(14, { nome: "Duda", email: "duda@loja.com", id_vendedor: 8, vendedor_nome: null }),
];

function paginaUsuarios(data: User[], total = data.length, pages = 1) {
  return { data, page: 1, limit: 20, total, pages };
}

describe("Usuarios - filtros client-side", () => {
  beforeEach(() => {
    api.apiListUsers.mockResolvedValue(paginaUsuarios(USUARIOS));
  });

  it.each<[string, string, string[]]>([
    ["nome", "caio", ["Caio"]],
    ["email", "@loja", ["Bia", "Caio", "Duda"]],
    ["ID", "14", ["Duda"]],
  ])("busca por %s", async (_n, termo, nomes) => {
    render(<UsuariosPage />);
    await screen.findByText("Ana Admin");
    await userEvent.type(screen.getByLabelText("Buscar"), termo);
    await waitFor(() => expect(coluna(1)).toEqual(nomes));
  });

  it.each<[string, string, string, string[]]>([
    ["Status ativo", "Status", "ativo", ["Ana Admin", "Caio", "Duda"]],
    ["Status inativo", "Status", "inativo", ["Bia"]],
    ["vinculado a vendedor", "Vinculado a vendedor", "sim", ["Caio", "Duda"]],
    ["sem vendedor", "Vinculado a vendedor", "nao", ["Ana Admin", "Bia"]],
    ["perfil admin", "Perfil", "admin", ["Ana Admin"]],
  ])("filtro %s", async (_n, label, valor, nomes) => {
    render(<UsuariosPage />);
    await screen.findByText("Ana Admin");
    await userEvent.selectOptions(screen.getByLabelText(label), valor);
    await waitFor(() => expect(coluna(1)).toEqual(nomes));
    // Filtros sao locais: nao disparam nova busca na pagina 1.
    expect(api.apiListUsers).toHaveBeenCalledTimes(1);
  });

  it("filtros combinados sem resultado mostram a mensagem de filtros", async () => {
    render(<UsuariosPage />);
    await screen.findByText("Ana Admin");
    await userEvent.selectOptions(screen.getByLabelText("Perfil"), "admin");
    await userEvent.selectOptions(screen.getByLabelText("Status"), "inativo");
    expect(await screen.findByText("Nenhum usuario encontrado para os filtros aplicados.")).toBeInTheDocument();
  });

  it("coluna Vendedor mostra '#id - nome' so quando ha id e nome; status ativo/inativo", async () => {
    render(<UsuariosPage />);
    await screen.findByText("Ana Admin");
    expect(within(linhaDe("Caio")).getByText("#7 - Vend Sete")).toBeInTheDocument();
    expect(within(linhaDe("Duda")).getByText("—")).toBeInTheDocument();
    expect(within(linhaDe("Bia")).getByTitle("Clique para reativar")).toHaveTextContent("Inativo");
    expect(within(linhaDe("Caio")).getByTitle("Clique para inativar")).toHaveTextContent("Ativo");
  });
});

describe("Usuarios - ordenacao, limite e paginacao", () => {
  beforeEach(() => {
    api.apiListUsers.mockImplementation(async (page: number, limit: number) => ({
      data: [usuario(page * 100)],
      page,
      limit,
      total: 60,
      pages: 3,
    }));
  });

  it("clicar duas vezes na mesma coluna alterna asc -> desc -> asc", async () => {
    render(<UsuariosPage />);
    await screen.findByText("Usuario 100");
    await userEvent.click(botaoCabecalho("ID"));
    await waitFor(() => expect(api.apiListUsers).toHaveBeenLastCalledWith(1, 20, "id", "desc"));
    await userEvent.click(botaoCabecalho("ID"));
    await waitFor(() => expect(api.apiListUsers).toHaveBeenLastCalledWith(1, 20, "id", "asc"));
  });

  it("mudar itens por pagina busca com o novo limite", async () => {
    render(<UsuariosPage />);
    await screen.findByText("Usuario 100");
    await userEvent.selectOptions(screen.getByLabelText("Itens por pagina"), "100");
    await waitFor(() => expect(api.apiListUsers).toHaveBeenLastCalledWith(1, 100, "id", "asc"));
  });

  it("navega por ultima, anterior e primeira pagina", async () => {
    render(<UsuariosPage />);
    await screen.findByText("Usuario 100");
    expect(screen.getByText("1-20 de 60 usuarios")).toBeInTheDocument();
    await userEvent.click(screen.getByTitle("Ultima pagina"));
    expect(await screen.findByText("Usuario 300")).toBeInTheDocument();
    expect(screen.getByText("41-60 de 60 usuarios")).toBeInTheDocument();
    await userEvent.click(screen.getByTitle("Pagina anterior"));
    expect(await screen.findByText("Usuario 200")).toBeInTheDocument();
    await userEvent.click(screen.getByTitle("Primeira pagina"));
    expect(await screen.findByText("Usuario 100")).toBeInTheDocument();
  });

  it.each<[number, string]>([
    [1, "1-1 de 1 usuario"],
    [0, "0 usuarios"],
  ])("total %i -> contador '%s'", async (total, texto) => {
    api.apiListUsers.mockResolvedValue(paginaUsuarios(total ? [usuario(1)] : [], total));
    render(<UsuariosPage />);
    expect(await screen.findByText(texto)).toBeInTheDocument();
  });
});

describe("Usuarios - acoes", () => {
  beforeEach(() => {
    api.apiListUsers.mockResolvedValue(paginaUsuarios(USUARIOS));
  });

  it("reativar usuario inativo envia ativo=true e atualiza so a linha dele", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api.apiToggleUserStatus.mockResolvedValue({ ...USUARIOS[1], ativo: true });
    render(<UsuariosPage />);
    await screen.findByText("Bia");
    await userEvent.click(within(linhaDe("Bia")).getByTitle("Clique para reativar"));
    expect(await screen.findByText("Usuario reativado com sucesso.")).toBeInTheDocument();
    expect(api.apiToggleUserStatus).toHaveBeenCalledWith(2, true);
    expect(window.confirm).toHaveBeenCalledWith('Tem certeza que deseja reativar o usuario "Bia"?');
    expect(within(linhaDe("Bia")).getByTitle("Clique para inativar")).toBeInTheDocument();
    expect(within(linhaDe("Caio")).getByTitle("Clique para inativar")).toBeInTheDocument();
  });

  it.each<[string, "apiToggleUserStatus" | "apiAdminResetPassword", () => HTMLElement, string]>([
    ["alterar status", "apiToggleUserStatus", () => within(linhaDe("Caio")).getByTitle("Clique para inativar"), "Erro ao alterar status."],
    ["resetar senha", "apiAdminResetPassword", () => within(linhaDe("Caio")).getByRole("button", { name: "Resetar" }), "Erro ao resetar senha."],
  ])("%s: rejeicao que nao e Error usa a mensagem padrao", async (_n, fn, botao, msg) => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api[fn].mockRejectedValue({ status: 500 });
    render(<UsuariosPage />);
    await screen.findByText("Caio");
    await userEvent.click(botao());
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });

  it("editar pela acao mantem as outras linhas e o alerta de sucesso pode ser fechado", async () => {
    api.apiUpdateUser.mockResolvedValue({ ...USUARIOS[2], nome: "Caio Lima" });
    render(<UsuariosPage />);
    await screen.findByText("Caio");
    await userEvent.click(within(linhaDe("Caio")).getByRole("button", { name: "Editar" }));
    const d = await screen.findByRole("dialog");
    await waitFor(() => expect(within(d).getByLabelText("Vendedor vinculado (opcional)")).toBeEnabled());
    await userEvent.click(within(d).getByRole("button", { name: "Salvar alteracoes" }));

    expect(await screen.findByText('Usuario "Caio Lima" atualizado com sucesso.')).toBeInTheDocument();
    expect(screen.getByText("Caio Lima")).toBeInTheDocument();
    expect(screen.getByText("Ana Admin")).toBeInTheDocument();
    expect(screen.getByText("Bia")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Fechar alerta" }));
    expect(screen.queryByText('Usuario "Caio Lima" atualizado com sucesso.')).not.toBeInTheDocument();
  });

  it("rejeicao na carga que nao e Error usa a mensagem padrao e o alerta pode ser fechado", async () => {
    api.apiListUsers.mockRejectedValue(null);
    render(<UsuariosPage />);
    expect(await screen.findByText(/Erro ao carregar usuarios/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Fechar alerta" }));
    expect(screen.queryByText(/Erro ao carregar usuarios/)).not.toBeInTheDocument();
  });
});

// ─── Vendedores ───────────────────────────────────────────────────────────────

function vend(id: number, nome: string, regiao: string, uf: string, desligado = false): Vendedor {
  return { id, nome, regiao, uf, data_desligamento: desligado ? "2026-01-01" : null };
}

const VENDEDORES: Vendedor[] = [
  vend(1, "Carla", "Sul", "PR"),
  vend(2, "Bruno", "Norte", "AM", true),
  vend(3, "Alice", "Sudeste", "SP"),
  vend(4, "Diego", "Sul", "SC", true),
];

describe("Vendedores - filtros client-side", () => {
  beforeEach(() => {
    api.apiListVendedores.mockResolvedValue(VENDEDORES);
  });

  it.each<[string, string, string[]]>([
    ["nome", "ali", ["Alice"]],
    ["regiao", "norte", ["Bruno"]],
    ["UF", "sc", ["Diego"]],
    ["ID", "3", ["Alice"]],
    ["status 'inativo'", "inativo", ["Bruno", "Diego"]],
  ])("busca por %s", async (_n, termo, nomes) => {
    render(<VendedoresPage />);
    await screen.findByText("Alice");
    await userEvent.type(screen.getByLabelText("Buscar"), termo);
    await waitFor(() => expect(coluna(1)).toEqual(nomes));
  });

  it.each<[string, string, string, string[]]>([
    ["regiao Sul", "Regiao", "Sul", ["Carla", "Diego"]],
    ["UF SP", "UF", "SP", ["Alice"]],
    ["status ativo", "Status", "ativo", ["Alice", "Carla"]],
    ["status inativo", "Status", "inativo", ["Bruno", "Diego"]],
  ])("filtro %s", async (_n, label, valor, nomes) => {
    render(<VendedoresPage />);
    await screen.findByText("Alice");
    await userEvent.selectOptions(screen.getByLabelText(label), valor);
    await waitFor(() => expect(coluna(1)).toEqual(nomes));
    expect(api.apiListVendedores).toHaveBeenCalledTimes(1);
  });

  it("opcoes de Regiao e UF vem da lista, sem repeticao e ordenadas", async () => {
    render(<VendedoresPage />);
    await screen.findByText("Alice");
    const opcoes = (label: string) =>
      Array.from((screen.getByLabelText(label) as HTMLSelectElement).options).map((o) => o.value);
    expect(opcoes("Regiao")).toEqual(["", "Norte", "Sudeste", "Sul"]);
    expect(opcoes("UF")).toEqual(["", "AM", "PR", "SC", "SP"]);
  });

  it("filtros sem resultado mostram a mensagem de filtros", async () => {
    render(<VendedoresPage />);
    await screen.findByText("Alice");
    await userEvent.selectOptions(screen.getByLabelText("UF"), "SP");
    await userEvent.selectOptions(screen.getByLabelText("Status"), "inativo");
    expect(
      await screen.findByText("Nenhum vendedor encontrado para a busca/filtros aplicados.")
    ).toBeInTheDocument();
    expect(screen.getByText("0 vendedores")).toBeInTheDocument();
  });

  it("um unico resultado usa o singular no contador", async () => {
    render(<VendedoresPage />);
    await screen.findByText("Alice");
    await userEvent.type(screen.getByLabelText("Buscar"), "alice");
    expect(await screen.findByText("1-1 de 1 vendedor")).toBeInTheDocument();
  });
});

describe("Vendedores - ordenacao local", () => {
  beforeEach(() => {
    api.apiListVendedores.mockResolvedValue(VENDEDORES);
  });

  it("padrao por nome asc; clicar de novo inverte e depois volta", async () => {
    render(<VendedoresPage />);
    await screen.findByText("Alice");
    expect(coluna(1)).toEqual(["Alice", "Bruno", "Carla", "Diego"]);
    await userEvent.click(botaoCabecalho("Vendedor"));
    await waitFor(() => expect(coluna(1)).toEqual(["Diego", "Carla", "Bruno", "Alice"]));
    await userEvent.click(botaoCabecalho("Vendedor"));
    await waitFor(() => expect(coluna(1)).toEqual(["Alice", "Bruno", "Carla", "Diego"]));
  });

  it("Status asc lista inativos primeiro; desc lista ativos primeiro", async () => {
    render(<VendedoresPage />);
    await screen.findByText("Alice");
    await userEvent.click(botaoCabecalho("Status"));
    await waitFor(() => expect(coluna(4).slice(0, 2)).toEqual(["Inativo", "Inativo"]));
    await userEvent.click(botaoCabecalho("Status"));
    await waitFor(() => expect(coluna(4).slice(0, 2)).toEqual(["Ativo", "Ativo"]));
  });

  it.each<[string, number, string[]]>([
    ["Regiao", 2, ["Norte", "Sudeste", "Sul", "Sul"]],
    ["UF", 3, ["AM", "PR", "SC", "SP"]],
  ])("ordena por %s (texto)", async (header, idx, esperado) => {
    render(<VendedoresPage />);
    await screen.findByText("Alice");
    await userEvent.click(botaoCabecalho(header));
    await waitFor(() => expect(coluna(idx)).toEqual(esperado));
  });
});

describe("Vendedores - paginacao local e limite", () => {
  const muitos = Array.from({ length: 45 }, (_, i) =>
    vend(i + 1, `Vend ${String(i + 1).padStart(2, "0")}`, "Sul", "PR")
  );

  beforeEach(() => {
    api.apiListVendedores.mockResolvedValue(muitos);
  });

  it("navega por ultima, anterior e primeira pagina", async () => {
    render(<VendedoresPage />);
    await screen.findByText("Vend 01");
    expect(screen.getByText("1-20 de 45 vendedores")).toBeInTheDocument();
    await userEvent.click(screen.getByTitle("Ultima pagina"));
    expect(await screen.findByText("Vend 41")).toBeInTheDocument();
    expect(screen.getByText("41-45 de 45 vendedores")).toBeInTheDocument();
    await userEvent.click(screen.getByTitle("Pagina anterior"));
    expect(await screen.findByText("Vend 21")).toBeInTheDocument();
    await userEvent.click(screen.getByTitle("Primeira pagina"));
    expect(await screen.findByText("Vend 01")).toBeInTheDocument();
    expect(api.apiListVendedores).toHaveBeenCalledTimes(1);
  });

  it("mudar o limite volta para a pagina 1 e remove a paginacao se couber tudo", async () => {
    render(<VendedoresPage />);
    await screen.findByText("Vend 01");
    await userEvent.click(screen.getByTitle("Ultima pagina"));
    await screen.findByText("Vend 41");
    await userEvent.selectOptions(screen.getByLabelText("Itens por pagina"), "50");
    expect(await screen.findByText("1-45 de 45 vendedores")).toBeInTheDocument();
    expect(screen.queryByTitle("Ultima pagina")).not.toBeInTheDocument();
  });
});

describe("Vendedores - acoes e erros", () => {
  beforeEach(() => {
    api.apiListVendedores.mockResolvedValue(VENDEDORES);
  });

  it.each<[string, string, "apiDeleteVendedor" | "apiReativarVendedor"]>([
    ["inativar", "Alice", "apiDeleteVendedor"],
    ["reativar", "Bruno", "apiReativarVendedor"],
  ])("cancelar a confirmacao de %s nao chama a API", async (_n, nome, fn) => {
    const confirmar = vi.spyOn(window, "confirm").mockReturnValue(false);
    render(<VendedoresPage />);
    await screen.findByText(nome);
    await userEvent.click(within(linhaDe(nome)).getByRole("button", { name: /Ativo|Inativo/ }));
    expect(confirmar).toHaveBeenCalledTimes(1);
    expect(api[fn]).not.toHaveBeenCalled();
    expect(api.apiListVendedores).toHaveBeenCalledTimes(1);
  });

  it.each<[string, string, "apiDeleteVendedor" | "apiReativarVendedor", string]>([
    ["inativar", "Alice", "apiDeleteVendedor", "Erro ao inativar vendedor."],
    ["reativar", "Bruno", "apiReativarVendedor", "Erro ao reativar vendedor."],
  ])("%s: rejeicao que nao e Error usa a mensagem padrao", async (_n, nome, fn, msg) => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api[fn].mockRejectedValue("falhou");
    render(<VendedoresPage />);
    await screen.findByText(nome);
    await userEvent.click(within(linhaDe(nome)).getByRole("button", { name: /Ativo|Inativo/ }));
    expect(await screen.findByText(msg)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Fechar alerta" }));
    expect(screen.queryByText(msg)).not.toBeInTheDocument();
  });

  it("sucesso ao inativar mostra alerta que pode ser fechado", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api.apiDeleteVendedor.mockResolvedValue(undefined);
    render(<VendedoresPage />);
    await screen.findByText("Alice");
    await userEvent.click(within(linhaDe("Alice")).getByRole("button", { name: "Ativo" }));
    const msg = 'Vendedor "Alice" inativado com sucesso.';
    expect(await screen.findByText(msg)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Fechar alerta" }));
    expect(screen.queryByText(msg)).not.toBeInTheDocument();
  });

  it("rejeicao na carga que nao e Error usa a mensagem padrao", async () => {
    api.apiListVendedores.mockRejectedValue(undefined);
    render(<VendedoresPage />);
    expect(await screen.findByText(/Erro ao carregar vendedores/)).toBeInTheDocument();
  });

  it("botao Editar da linha abre o modal com o detalhe; fechar o modal o remove", async () => {
    api.apiGetVendedor.mockResolvedValue({
      ...VENDEDORES[2],
      data_admissao: "2020-01-01",
      meta_mensal: 100,
      clientes: [],
    });
    render(<VendedoresPage />);
    await screen.findByText("Alice");
    await userEvent.click(within(linhaDe("Alice")).getByRole("button", { name: "Editar" }));
    const d = await screen.findByRole("dialog");
    await waitFor(() => expect(api.apiGetVendedor).toHaveBeenCalledWith(3));
    expect((within(d).getByLabelText("Nome") as HTMLInputElement).value).toBe("Alice");
    await userEvent.click(within(d).getByRole("button", { name: "Fechar" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });
});
