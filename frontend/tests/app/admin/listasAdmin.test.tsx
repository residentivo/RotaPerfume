/**
 * Smoke/regressao do FE-03 nas telas administrativas com busca client-side:
 * usuarios e senha-historico (paginacao pela API, busca local) e vendedores
 * (lista inteira carregada uma vez, paginacao local).
 *
 * Mudanca declarada no FE-03: em usuarios e senha-historico, trocar filtro
 * faz 1 fetch a menos (o reset de pagina acontece no render, sem efeito).
 */
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SenhaHistoricoItem, User, Vendedor } from "@/lib/types";

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

import UsuariosPage from "@/app/admin/usuarios/page";
import SenhaHistoricoPage from "@/app/admin/senha-historico/page";
import VendedoresPage from "@/app/admin/vendedores/page";

function spinner() {
  return document.querySelector(".animate-spin");
}

function deferred<T>() {
  let resolve: (v: T) => void = () => {};
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

function botaoCabecalho(header: string): HTMLElement {
  const th = screen
    .getAllByRole("columnheader")
    .find((el) => el.textContent?.replace(/[↑↓↕]/g, "").trim() === header);
  if (!th) throw new Error(`cabecalho ${header} nao encontrado`);
  return within(th).getByRole("button");
}

beforeEach(() => {
  vi.clearAllMocks();
  Object.values(api).forEach((f) => f.mockReset());
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

describe("Usuarios (FE-03)", () => {
  beforeEach(() => {
    api.apiListUsers.mockImplementation(async (page: number) => ({
      data: [usuario(page * 100), usuario(page * 100 + 1, { role: "admin" })],
      page,
      limit: 20,
      total: 60,
      pages: 3,
    }));
  });

  it("loading -> lista da pagina 1 com a ordenacao padrao", async () => {
    const d = deferred<unknown>();
    api.apiListUsers.mockReturnValueOnce(d.promise);
    render(<UsuariosPage />);
    expect(spinner()).not.toBeNull();
    expect(api.apiListUsers).toHaveBeenCalledWith(1, 20, "id", "asc");
    await act(async () =>
      d.resolve({ data: [usuario(100)], page: 1, limit: 20, total: 1, pages: 1 })
    );
    expect(await screen.findByText("Usuario 100")).toBeInTheDocument();
    expect(spinner()).toBeNull();
  });

  it("FE-04: resposta obsoleta (pagina 2 chega depois da 3) nao sobrescreve a mais nova", async () => {
    render(<UsuariosPage />);
    await screen.findByText("Usuario 100");
    const p2 = deferred<unknown>();
    const p3 = deferred<unknown>();
    api.apiListUsers.mockReturnValueOnce(p2.promise).mockReturnValueOnce(p3.promise);
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    await userEvent.click(screen.getByTitle("Proxima pagina"));

    await act(async () =>
      p3.resolve({ data: [usuario(300)], page: 3, limit: 20, total: 60, pages: 3 })
    );
    await screen.findByText("Usuario 300");
    await act(async () =>
      p2.resolve({ data: [usuario(200)], page: 2, limit: 20, total: 60, pages: 3 })
    );
    expect(screen.getByText("Usuario 300")).toBeInTheDocument();
    expect(screen.queryByText("Usuario 200")).not.toBeInTheDocument();
  });

  it("paginacao e ordenacao disparam busca com os parametros certos", async () => {
    render(<UsuariosPage />);
    await screen.findByText("Usuario 100");
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    expect(await screen.findByText("Usuario 200")).toBeInTheDocument();
    expect(api.apiListUsers).toHaveBeenLastCalledWith(2, 20, "id", "asc");

    await userEvent.click(botaoCabecalho("Nome"));
    await waitFor(() => expect(api.apiListUsers).toHaveBeenLastCalledWith(2, 20, "nome", "asc"));
  });

  it("filtro na pagina 2 volta para a 1 com uma unica busca; na pagina 1 nao busca de novo", async () => {
    render(<UsuariosPage />);
    await screen.findByText("Usuario 100");
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    await screen.findByText("Usuario 200");

    const antes = api.apiListUsers.mock.calls.length;
    await userEvent.selectOptions(screen.getByLabelText("Perfil"), "admin");
    await screen.findByText("Usuario 101");
    expect(api.apiListUsers.mock.calls.length).toBe(antes + 1);
    expect(api.apiListUsers).toHaveBeenLastCalledWith(1, 20, "id", "asc");
    // Filtro client-side aplicado sobre a pagina.
    expect(screen.queryByText("Usuario 100")).not.toBeInTheDocument();

    const naPagina1 = api.apiListUsers.mock.calls.length;
    await userEvent.selectOptions(screen.getByLabelText("Perfil"), "");
    expect(await screen.findByText("Usuario 100")).toBeInTheDocument();
    expect(api.apiListUsers.mock.calls.length).toBe(naPagina1);
  });

  it("erro da API mostra a mensagem e desliga o loading", async () => {
    api.apiListUsers.mockRejectedValue(new Error("sem usuarios"));
    render(<UsuariosPage />);
    expect(await screen.findByText("sem usuarios")).toBeInTheDocument();
    expect(spinner()).toBeNull();
  });

  it("modal: novo abre resetado; editar abre preenchido", async () => {
    api.apiListVendedores.mockResolvedValue([]);
    render(<UsuariosPage />);
    await screen.findByText("Usuario 100");
    await userEvent.click(screen.getByRole("button", { name: "+ Novo Usuario" }));
    const dialog = await screen.findByRole("dialog");
    expect((within(dialog).getByLabelText("Nome completo") as HTMLInputElement).value).toBe("");
    await userEvent.click(within(dialog).getByRole("button", { name: "Cancelar" }));

    await userEvent.click(screen.getByText("Usuario 100"));
    const edit = await screen.findByRole("dialog");
    expect((within(edit).getByLabelText("Nome completo") as HTMLInputElement).value).toBe(
      "Usuario 100"
    );
  });
});

// ─── Historico de senha ───────────────────────────────────────────────────────

function item(id: number): SenhaHistoricoItem {
  return {
    id,
    usuario_id: id,
    usuario_nome: `Pessoa ${id}`,
    resetado_por_id: null,
    resetado_por_nome: null,
    tipo_reset: "admin",
    ip_origem: "10.0.0.1",
    created_at: "2026-09-01T10:00:00Z",
  };
}

describe("Historico de senha (FE-03)", () => {
  beforeEach(() => {
    api.apiListSenhaHistorico.mockImplementation(async (page: number) => ({
      data: [item(page * 100)],
      page,
      limit: 20,
      total: 60,
      pages: 3,
    }));
  });

  it("loading -> lista da pagina 1 ordenada por data desc", async () => {
    render(<SenhaHistoricoPage />);
    expect(spinner()).not.toBeNull();
    expect(await screen.findByText("#100 - Pessoa 100")).toBeInTheDocument();
    expect(api.apiListSenhaHistorico).toHaveBeenCalledWith(1, 20, undefined, undefined, "created_at", "desc");
    expect(api.apiListSenhaHistorico).toHaveBeenCalledTimes(1);
  });

  it("FE-04: resposta obsoleta (pagina 2 chega depois da 3) nao sobrescreve a mais nova", async () => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#100 - Pessoa 100");
    const p2 = deferred<unknown>();
    const p3 = deferred<unknown>();
    api.apiListSenhaHistorico.mockReturnValueOnce(p2.promise).mockReturnValueOnce(p3.promise);
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    await userEvent.click(screen.getByTitle("Proxima pagina"));

    await act(async () => p3.resolve({ data: [item(300)], page: 3, limit: 20, total: 60, pages: 3 }));
    await screen.findByText("#300 - Pessoa 300");
    await act(async () => p2.resolve({ data: [item(200)], page: 2, limit: 20, total: 60, pages: 3 }));
    expect(screen.getByText("#300 - Pessoa 300")).toBeInTheDocument();
    expect(screen.queryByText("#200 - Pessoa 200")).not.toBeInTheDocument();
  });

  it("paginacao e ordenacao pela API", async () => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#100 - Pessoa 100");
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    expect(await screen.findByText("#200 - Pessoa 200")).toBeInTheDocument();
    expect(api.apiListSenhaHistorico).toHaveBeenLastCalledWith(2, 20, undefined, undefined, "created_at", "desc");

    await userEvent.click(botaoCabecalho("Tipo Reset"));
    await waitFor(() =>
      expect(api.apiListSenhaHistorico).toHaveBeenLastCalledWith(2, 20, undefined, undefined, "tipo_reset", "asc")
    );
  });

  it("mudar o tipo na pagina 2 volta para a 1 com uma unica busca", async () => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#100 - Pessoa 100");
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    await screen.findByText("#200 - Pessoa 200");

    const antes = api.apiListSenhaHistorico.mock.calls.length;
    await userEvent.selectOptions(screen.getByLabelText("Tipo de Reset"), "esquecimento");
    await screen.findByText("#100 - Pessoa 100");
    expect(api.apiListSenhaHistorico.mock.calls.length).toBe(antes + 1);
    expect(api.apiListSenhaHistorico).toHaveBeenLastCalledWith(1, 20, undefined, "esquecimento", "created_at", "desc");
  });

  it("busca textual e local: na pagina 1 nao dispara requisicao", async () => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#100 - Pessoa 100");
    const antes = api.apiListSenhaHistorico.mock.calls.length;
    await userEvent.type(screen.getByLabelText("Buscar"), "nada-bate");
    await act(() => new Promise((r) => setTimeout(r, 400)));
    expect(api.apiListSenhaHistorico.mock.calls.length).toBe(antes);
    expect(screen.queryByText("#100 - Pessoa 100")).not.toBeInTheDocument();
  });

  it("botao Atualizar recarrega", async () => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#100 - Pessoa 100");
    await userEvent.click(screen.getByRole("button", { name: /Atualizar/ }));
    await waitFor(() => expect(api.apiListSenhaHistorico).toHaveBeenCalledTimes(2));
  });

  it("erro da API mostra a mensagem e desliga o loading", async () => {
    api.apiListSenhaHistorico.mockRejectedValue(new Error("sem historico"));
    render(<SenhaHistoricoPage />);
    expect(await screen.findByText("sem historico")).toBeInTheDocument();
    expect(spinner()).toBeNull();
  });
});

// ─── Vendedores ───────────────────────────────────────────────────────────────

function vend(id: number): Vendedor {
  return {
    id,
    nome: `Vend ${String(id).padStart(2, "0")}`,
    regiao: id % 2 ? "Sul" : "Norte",
    uf: "SP",
    data_desligamento: null,
  };
}

describe("Vendedores (FE-03)", () => {
  beforeEach(() => {
    api.apiListVendedores.mockResolvedValue(Array.from({ length: 25 }, (_, i) => vend(i + 1)));
  });

  it("loading -> carrega uma vez e pagina localmente", async () => {
    render(<VendedoresPage />);
    expect(spinner()).not.toBeNull();
    expect(await screen.findByText("Vend 01")).toBeInTheDocument();
    expect(screen.queryByText("Vend 21")).not.toBeInTheDocument();

    await userEvent.click(screen.getByTitle("Proxima pagina"));
    expect(await screen.findByText("Vend 21")).toBeInTheDocument();
    expect(api.apiListVendedores).toHaveBeenCalledTimes(1);
  });

  it.each<[string, (u: ReturnType<typeof userEvent.setup>) => Promise<void>]>([
    ["busca", async (u) => u.type(screen.getByLabelText("Buscar"), "Vend")],
    ["ordenacao", async (u) => u.click(botaoCabecalho("Vendedor"))],
  ])("mudar %s volta para a pagina 1", async (_n, mudar) => {
    render(<VendedoresPage />);
    await screen.findByText("Vend 01");
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    await screen.findByText("Vend 21");
    await mudar(userEvent.setup());
    // Pagina 1: "Pagina anterior" desabilitado.
    await waitFor(() => expect(screen.getByTitle("Pagina anterior")).toBeDisabled());
  });

  it("erro da API mostra a mensagem e desliga o loading", async () => {
    api.apiListVendedores.mockRejectedValue(new Error("sem vendedores"));
    render(<VendedoresPage />);
    expect(await screen.findByText("sem vendedores")).toBeInTheDocument();
    expect(spinner()).toBeNull();
  });
});

// ─── UI-03: coluna propria para o ID principal ────────────────────────────────

function textoCabecalho(el: Element): string {
  return (el.textContent ?? "").replace(/[↑↓↕]/g, "").trim();
}

function primeiraCelulaDaLinha(n: number): HTMLElement {
  const linhas = screen.getAllByRole("row").filter((r) => within(r).queryAllByRole("cell").length);
  return within(linhas[n]).getAllByRole("cell")[0];
}

describe("UI-03 coluna ID - telas administrativas", () => {
  beforeEach(() => {
    api.apiListUsers.mockResolvedValue({ data: [usuario(100)], page: 1, limit: 20, total: 1, pages: 1 });
    api.apiListVendedores.mockResolvedValue(Array.from({ length: 25 }, (_, i) => vend(i + 1)));
    api.apiListSenhaHistorico.mockResolvedValue({ data: [item(100)], page: 1, limit: 20, total: 1, pages: 1 });
  });

  it.each<[string, () => ReactElement, string, string, string]>([
    ["usuarios", () => <UsuariosPage />, "#100", "Nome", "Usuario 100"],
    ["vendedores", () => <VendedoresPage />, "#1", "Vendedor", "Vend 01"],
    ["senha-historico", () => <SenhaHistoricoPage />, "#100", "Usuario", "Pessoa 100"],
  ])("%s: primeira coluna 'ID' com #<id> em fonte mono", async (_n, Page, idTexto, nomeHeader, nome) => {
    render(Page());
    const celulaId = await screen.findByText(idTexto);
    const headers = screen.getAllByRole("columnheader");
    expect(textoCabecalho(headers[0])).toBe("ID");
    expect(celulaId).toHaveClass("font-mono");
    const linha = celulaId.closest("tr")!;
    expect(within(linha).getAllByRole("cell")[0]).toHaveTextContent(new RegExp(`^${idTexto}$`));
    const idx = headers.map(textoCabecalho).indexOf(nomeHeader);
    expect(idx).toBeGreaterThan(0);
    expect(within(linha).getAllByRole("cell")[idx]).toHaveTextContent(nome);
  });

  it.each<[string, () => ReactElement, string]>([
    ["usuarios", () => <UsuariosPage />, "Usuario 100"],
    ["vendedores", () => <VendedoresPage />, "Vend 01"],
  ])("%s: nome nao tem mais o prefixo '#<id> - '", async (_n, Page, nome) => {
    render(Page());
    const botao = await screen.findByRole("button", { name: nome });
    expect(botao.textContent).toBe(nome);
    expect(screen.queryByText(/^#\d+ - (Usuario|Vend) /)).toBeNull();
  });

  it("vendedores: ordenar pelo ID ordena localmente asc/desc", async () => {
    render(<VendedoresPage />);
    await screen.findByText("Vend 01");
    await userEvent.click(botaoCabecalho("ID"));
    await waitFor(() => expect(primeiraCelulaDaLinha(0)).toHaveTextContent(/^#1$/));
    await userEvent.click(botaoCabecalho("ID"));
    await waitFor(() => expect(primeiraCelulaDaLinha(0)).toHaveTextContent(/^#25$/));
    expect(api.apiListVendedores).toHaveBeenCalledTimes(1);
  });

  it("senha-historico: ordenar pelo ID envia order_by=id para a API", async () => {
    render(<SenhaHistoricoPage />);
    await screen.findByText("#100");
    await userEvent.click(botaoCabecalho("ID"));
    await waitFor(() =>
      expect(api.apiListSenhaHistorico).toHaveBeenLastCalledWith(1, 20, undefined, undefined, "id", "asc")
    );
    await userEvent.click(botaoCabecalho("ID"));
    await waitFor(() =>
      expect(api.apiListSenhaHistorico).toHaveBeenLastCalledWith(1, 20, undefined, undefined, "id", "desc")
    );
  });
});

// ─── FE-09: erro de carga nao zera a lista nem mostra o estado vazio ─────────

describe("FE-09 - erro de carga nas telas administrativas", () => {
  const ERRO_REDE = "Failed to fetch";
  const paginaUsuarios = (page: number) => ({
    data: [usuario(page * 100)],
    page,
    limit: 20,
    total: 60,
    pages: 3,
  });
  const paginaHistorico = (page: number) => ({
    data: [item(page * 100)],
    page,
    limit: 20,
    total: 60,
    pages: 3,
  });
  const vazioPaginado = { data: [], page: 1, limit: 20, total: 0, pages: 0 };

  it.each<[string, () => void, () => ReactElement, string]>([
    ["usuarios", () => api.apiListUsers.mockRejectedValue(new TypeError(ERRO_REDE)), () => <UsuariosPage />, "Nenhum usuario cadastrado."],
    [
      "senha-historico",
      () => api.apiListSenhaHistorico.mockRejectedValue(new TypeError(ERRO_REDE)),
      () => <SenhaHistoricoPage />,
      "Nenhuma alteracao de senha registrada.",
    ],
    ["vendedores", () => api.apiListVendedores.mockRejectedValue(new TypeError(ERRO_REDE)), () => <VendedoresPage />, "Nenhum vendedor cadastrado."],
  ])("%s: erro de rede na primeira carga mostra so o alerta", async (_n, falhar, Page, vazio) => {
    falhar();
    render(Page());
    expect(await screen.findByText(ERRO_REDE)).toBeInTheDocument();
    expect(screen.queryByText(vazio)).not.toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(spinner()).toBeNull();
  });

  it.each<[string, () => void, () => ReactElement, string]>([
    ["usuarios", () => api.apiListUsers.mockResolvedValue(vazioPaginado), () => <UsuariosPage />, "Nenhum usuario cadastrado."],
    [
      "senha-historico",
      () => api.apiListSenhaHistorico.mockResolvedValue(vazioPaginado),
      () => <SenhaHistoricoPage />,
      "Nenhuma alteracao de senha registrada.",
    ],
    ["vendedores", () => api.apiListVendedores.mockResolvedValue([]), () => <VendedoresPage />, "Nenhum vendedor cadastrado."],
  ])("%s: sucesso com lista vazia mostra o estado vazio normal", async (_n, vazioOk, Page, vazio) => {
    vazioOk();
    render(Page());
    expect(await screen.findByText(vazio)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("usuarios: erro ao trocar de pagina mantem a lista anterior + alerta", async () => {
    api.apiListUsers.mockImplementation(async (page: number) => paginaUsuarios(page));
    render(<UsuariosPage />);
    await screen.findByText("Usuario 100");
    api.apiListUsers.mockRejectedValueOnce(new TypeError(ERRO_REDE));
    await userEvent.click(screen.getByTitle("Proxima pagina"));
    expect(await screen.findByText(ERRO_REDE)).toBeInTheDocument();
    expect(screen.getByText("Usuario 100")).toBeInTheDocument();
    expect(screen.queryByText("Nenhum usuario cadastrado.")).not.toBeInTheDocument();
  });

  it("senha-historico: erro no 'Atualizar' mantem a lista anterior + alerta", async () => {
    api.apiListSenhaHistorico.mockImplementation(async (page: number) => paginaHistorico(page));
    render(<SenhaHistoricoPage />);
    await screen.findByText("#100 - Pessoa 100");
    api.apiListSenhaHistorico.mockRejectedValueOnce(new TypeError(ERRO_REDE));
    await userEvent.click(screen.getByRole("button", { name: /Atualizar/ }));
    expect(await screen.findByText(ERRO_REDE)).toBeInTheDocument();
    expect(screen.getByText("#100 - Pessoa 100")).toBeInTheDocument();
    expect(screen.queryByText("Nenhuma alteracao de senha registrada.")).not.toBeInTheDocument();
  });

  it("vendedores: erro na recarga apos inativar mantem a lista anterior + alerta", async () => {
    const confirmar = vi.spyOn(window, "confirm").mockReturnValue(true);
    api.apiListVendedores.mockResolvedValueOnce([vend(1), vend(2)]);
    api.apiListVendedores.mockRejectedValueOnce(new TypeError(ERRO_REDE));
    api.apiDeleteVendedor.mockResolvedValue(undefined);
    render(<VendedoresPage />);
    await screen.findByText("Vend 01");

    const linha = screen.getByText("Vend 01").closest("tr")!;
    await userEvent.click(within(linha).getByTitle("Clique para inativar"));

    expect(await screen.findByText(ERRO_REDE)).toBeInTheDocument();
    expect(screen.getByText("Vend 01")).toBeInTheDocument();
    expect(screen.getByText("Vend 02")).toBeInTheDocument();
    expect(screen.queryByText("Nenhum vendedor cadastrado.")).not.toBeInTheDocument();
    confirmar.mockRestore();
  });
});
