/**
 * Smoke de CRUD nas telas administrativas refatoradas no FE-03: usuarios,
 * vendedores, produtos e estoque. Abrir modal, salvar, alternar status e
 * tratar erros, com API e sessao mockadas.
 */
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { MeResponse } from "@/lib/types";

vi.mock("@/components/layout/ProtectedRoute", () => ({
  ProtectedRoute: ({ children }: { children: ReactNode }) => <>{children}</>,
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
  apiCreateVendedor: vi.fn(),
  apiUpdateVendedor: vi.fn(),
  apiDeleteVendedor: vi.fn(),
  apiReativarVendedor: vi.fn(),
  apiGetVendedor: vi.fn(),
  apiListClientes: vi.fn(),
  apiVincularCliente: vi.fn(),
  apiDesvincularCliente: vi.fn(),
  apiListProdutos: vi.fn(),
  apiCreateProduto: vi.fn(),
  apiUpdateProduto: vi.fn(),
  apiToggleProdutoStatus: vi.fn(),
  apiListEstoque: vi.fn(),
  apiCreateEstoque: vi.fn(),
  apiUpdateEstoque: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import UsuariosPage from "@/app/admin/usuarios/page";
import VendedoresPage from "@/app/admin/vendedores/page";
import ProdutosPage from "@/app/admin/produtos/page";
import EstoquePage from "@/app/admin/estoque/page";

function pagina<T>(data: T[]) {
  return { data, page: 1, limit: 20, total: data.length, pages: 1 };
}

async function dialogo() {
  return screen.findByRole("dialog");
}

/** Espera a janela do debounce dos filtros (350ms) passar. Desde o FE-04 ele
 * nao dispara na montagem; a espera garante que nao ha busca extra. */
async function esperarDebounce() {
  await act(() => new Promise((r) => setTimeout(r, 420)));
}

beforeEach(() => {
  vi.clearAllMocks();
  Object.values(api).forEach((f) => f.mockReset());
  api.apiListVendedores.mockResolvedValue([]);
  api.apiListClientes.mockResolvedValue(pagina([]));
});

// ─── Usuarios ─────────────────────────────────────────────────────────────────

const bia = { id: 2, nome: "Bia", email: "bia@x", role: "normal", ativo: true, id_vendedor: null };

describe("Usuarios - CRUD", () => {
  beforeEach(() => {
    api.apiListUsers.mockResolvedValue(pagina([bia]));
  });

  it.each<[string, boolean, string]>([
    ["email enviado", true, 'Usuario "Caio" criado com sucesso.'],
    ["email NAO enviado", false, "foi criado, mas o email com a senha inicial NAO pode ser enviado"],
  ])("criar (%s) recarrega a lista e informa", async (_n, enviado, msg) => {
    api.apiCreateUser.mockResolvedValue({ ...bia, id: 3, nome: "Caio", email_enviado: enviado });
    render(<UsuariosPage />);
    await screen.findByText("Bia");
    await userEvent.click(screen.getByRole("button", { name: "+ Novo Usuario" }));
    const d = await dialogo();
    await waitFor(() => expect(within(d).getByLabelText("Vendedor vinculado (opcional)")).toBeEnabled());
    await userEvent.type(within(d).getByLabelText("Nome completo"), "Caio");
    await userEvent.type(within(d).getByLabelText("Email"), "caio@x.com");
    await userEvent.click(within(d).getByRole("button", { name: "Criar usuario" }));
    expect(await screen.findByText(new RegExp(msg.replace(/[()".]/g, ".")))).toBeInTheDocument();
    expect(api.apiListUsers).toHaveBeenCalledTimes(2);
  });

  it("editar atualiza a linha", async () => {
    api.apiUpdateUser.mockResolvedValue({ ...bia, nome: "Bia Souza" });
    render(<UsuariosPage />);
    await userEvent.click(await screen.findByText("Bia"));
    const d = await dialogo();
    await waitFor(() => expect(within(d).getByLabelText("Vendedor vinculado (opcional)")).toBeEnabled());
    await userEvent.click(within(d).getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText('Usuario "Bia Souza" atualizado com sucesso.')).toBeInTheDocument();
    expect(api.apiUpdateUser).toHaveBeenCalledWith(2, { nome: "Bia", role: "normal", id_vendedor: null });
  });

  it.each<[string, () => void, string]>([
    ["sucesso", () => api.apiToggleUserStatus.mockResolvedValue({ ...bia, ativo: false }), "Usuario inativado com sucesso."],
    ["erro", () => api.apiToggleUserStatus.mockRejectedValue(new Error("nao pode")), "nao pode"],
  ])("inativar (%s)", async (_n, preparar, msg) => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    preparar();
    render(<UsuariosPage />);
    await screen.findByText("Bia");
    await userEvent.click(screen.getByTitle("Clique para inativar"));
    expect(await screen.findByText(msg)).toBeInTheDocument();
    expect(api.apiToggleUserStatus).toHaveBeenCalledWith(2, false);
  });

  it.each<[string, () => void, RegExp]>([
    ["email enviado", () => api.apiAdminResetPassword.mockResolvedValue({ sucesso: true, mensagem: "", email_enviado: true }), /Nova senha enviada para o email de Bia/],
    ["email NAO enviado", () => api.apiAdminResetPassword.mockResolvedValue({ sucesso: true, mensagem: "", email_enviado: false }), /foi resetada, mas o email NAO pode ser enviado/],
    ["erro", () => api.apiAdminResetPassword.mockRejectedValue(new Error("falha reset")), /falha reset/],
  ])("resetar senha (%s)", async (_n, preparar, msg) => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    preparar();
    render(<UsuariosPage />);
    await screen.findByText("Bia");
    await userEvent.click(screen.getByRole("button", { name: "Resetar" }));
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });

  it("cancelar confirmacoes nao chama a API", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    render(<UsuariosPage />);
    await screen.findByText("Bia");
    await userEvent.click(screen.getByTitle("Clique para inativar"));
    await userEvent.click(screen.getByRole("button", { name: "Resetar" }));
    expect(api.apiToggleUserStatus).not.toHaveBeenCalled();
    expect(api.apiAdminResetPassword).not.toHaveBeenCalled();
  });
});

// ─── Vendedores ───────────────────────────────────────────────────────────────

const ativo = { id: 3, nome: "Vend 3", regiao: "Sul", uf: "PR", data_desligamento: null };
const inativo = { id: 4, nome: "Vend 4", regiao: "Sul", uf: "PR", data_desligamento: "2026-01-01" };

describe("Vendedores - CRUD", () => {
  beforeEach(() => {
    api.apiListVendedores.mockResolvedValue([ativo, inativo]);
    api.apiGetVendedor.mockImplementation(async (id: number) => ({
      ...(id === 3 ? ativo : inativo),
      data_admissao: "2020-01-01",
      meta_mensal: 100,
      clientes: [],
    }));
  });

  it.each<[string, string, "apiDeleteVendedor" | "apiReativarVendedor", string]>([
    ["inativar ativo", "Vend 3", "apiDeleteVendedor", 'Vendedor "Vend 3" inativado com sucesso.'],
    ["reativar inativo", "Vend 4", "apiReativarVendedor", 'Vendedor "Vend 4" reativado com sucesso.'],
  ])("%s", async (_n, linha, fn, msg) => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api[fn].mockResolvedValue({});
    render(<VendedoresPage />);
    const row = (await screen.findByText(linha)).closest("tr")!;
    await userEvent.click(within(row).getByRole("button", { name: /Ativo|Inativo/ }));
    expect(await screen.findByText(msg)).toBeInTheDocument();
    expect(api.apiListVendedores).toHaveBeenCalledTimes(2);
  });

  it.each<[string, "apiDeleteVendedor" | "apiReativarVendedor", string]>([
    ["Vend 3", "apiDeleteVendedor", "erro inativar"],
    ["Vend 4", "apiReativarVendedor", "erro reativar"],
  ])("erro ao alternar %s mostra a mensagem", async (linha, fn, msg) => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api[fn].mockRejectedValue(new Error(msg));
    render(<VendedoresPage />);
    const row = (await screen.findByText(linha)).closest("tr")!;
    await userEvent.click(within(row).getByRole("button", { name: /Ativo|Inativo/ }));
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });

  it("criar recarrega a lista", async () => {
    api.apiCreateVendedor.mockResolvedValue({ ...ativo, id: 9, nome: "Novo" });
    render(<VendedoresPage />);
    await screen.findByText("Vend 3");
    await userEvent.click(screen.getByRole("button", { name: "+ Novo Vendedor" }));
    const d = await dialogo();
    await userEvent.type(within(d).getByLabelText("Nome"), "Novo");
    await userEvent.type(within(d).getByLabelText("Regiao"), "Sul");
    await userEvent.type(within(d).getByLabelText("UF"), "pr");
    await userEvent.click(within(d).getByRole("button", { name: "Criar vendedor" }));
    expect(await screen.findByText('Vendedor "Novo" criado com sucesso.')).toBeInTheDocument();
  });

  it("editar busca o detalhe, salva e recarrega", async () => {
    api.apiUpdateVendedor.mockResolvedValue({});
    render(<VendedoresPage />);
    await userEvent.click(await screen.findByText("Vend 3"));
    const d = await dialogo();
    await within(d).findByText("Nenhum cliente vinculado a este vendedor.");
    await userEvent.click(within(d).getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText('Vendedor "Vend 3" atualizado com sucesso.')).toBeInTheDocument();
    expect(api.apiUpdateVendedor).toHaveBeenCalledWith(3, expect.objectContaining({ nome: "Vend 3" }));
  });
});

// ─── Produtos ─────────────────────────────────────────────────────────────────

const perfume = {
  id: 50, sku: "P50", descricao: "Perfume 50", categoria: "C", marca: "M", nota_olfativa: "",
  preco_tabela: 10, custo_unitario: 5, unidade: "UN", data_lancamento: null, ativo: true,
};

describe("Produtos - CRUD", () => {
  beforeEach(() => {
    api.apiListProdutos.mockResolvedValue(pagina([perfume]));
  });

  it.each<[string, () => void, string]>([
    ["sucesso", () => api.apiToggleProdutoStatus.mockResolvedValue({ ...perfume, ativo: false }), "Produto inativado com sucesso."],
    ["erro", () => api.apiToggleProdutoStatus.mockRejectedValue(new Error("em uso")), "em uso"],
  ])("inativar (%s)", async (_n, preparar, msg) => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    preparar();
    render(<ProdutosPage />);
    await screen.findByText("Perfume 50");
    await esperarDebounce();
    await userEvent.click(screen.getByTitle("Clique para inativar"));
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });

  it("editar atualiza a linha", async () => {
    api.apiUpdateProduto.mockResolvedValue({ ...perfume, descricao: "Perfume Novo" });
    render(<ProdutosPage />);
    await screen.findByText("Perfume 50");
    await esperarDebounce();
    await userEvent.click(await screen.findByText("Perfume 50"));
    const d = await dialogo();
    await userEvent.click(within(d).getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText('Produto "Perfume Novo" atualizado com sucesso.')).toBeInTheDocument();
    expect(screen.getByText("Perfume Novo")).toBeInTheDocument();
  });

  it("criar recarrega a lista", async () => {
    api.apiCreateProduto.mockResolvedValue({ ...perfume, id: 51, descricao: "Outro" });
    render(<ProdutosPage />);
    await screen.findByText("Perfume 50");
    await esperarDebounce();
    await userEvent.click(screen.getByRole("button", { name: "+ Novo Produto" }));
    const d = await dialogo();
    const w = within(d);
    await userEvent.type(w.getByLabelText("SKU"), "P51");
    await userEvent.type(w.getByLabelText("Descricao"), "Outro");
    await userEvent.type(w.getByLabelText("Categoria"), "C");
    await userEvent.type(w.getByLabelText("Marca"), "M");
    await userEvent.type(w.getByLabelText("Preco de tabela"), "1");
    await userEvent.type(w.getByLabelText("Custo unitario"), "1");
    await userEvent.type(w.getByLabelText("Unidade"), "UN");
    await userEvent.click(w.getByRole("button", { name: "Criar produto" }));
    expect(await screen.findByText('Produto "Outro" criado com sucesso.')).toBeInTheDocument();
  });
});

// ─── Estoque ──────────────────────────────────────────────────────────────────

const registro = { id: 5, data_snapshot: "2026-09-01", sku: "SKU-5", produto_descricao: "Perfume", saldo: 3, ruptura: false };

describe("Estoque - CRUD (admin)", () => {
  beforeEach(() => {
    api.apiListEstoque.mockResolvedValue(pagina([registro]));
  });

  it("editar salva so o saldo e atualiza a linha", async () => {
    api.apiUpdateEstoque.mockResolvedValue({ ...registro, saldo: 9 });
    render(<EstoquePage />);
    await screen.findByText("SKU-5");
    await esperarDebounce();
    await userEvent.click((await screen.findAllByTitle("Editar registro de estoque"))[0]);
    const d = await dialogo();
    await userEvent.clear(within(d).getByLabelText("Saldo"));
    await userEvent.type(within(d).getByLabelText("Saldo"), "9");
    await userEvent.click(within(d).getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText("Registro de estoque #5 (SKU-5) atualizado com sucesso.")).toBeInTheDocument();
    expect(api.apiUpdateEstoque).toHaveBeenCalledWith(5, { saldo: 9 });
  });

  it("criar recarrega a lista", async () => {
    api.apiCreateEstoque.mockResolvedValue({ ...registro, id: 6, sku: "SKU-6" });
    render(<EstoquePage />);
    await screen.findByText("SKU-5");
    await esperarDebounce();
    await screen.findByText("SKU-5");
    await userEvent.click(screen.getByRole("button", { name: "+ Novo registro" }));
    const d = await dialogo();
    await userEvent.type(within(d).getByLabelText("SKU"), "SKU-6");
    await userEvent.type(within(d).getByLabelText("Data do snapshot"), "2026-01-01");
    await userEvent.type(within(d).getByLabelText("Saldo"), "1");
    await userEvent.click(within(d).getByRole("button", { name: "Criar registro" }));
    expect(await screen.findByText("Registro de estoque #6 (SKU-6) criado com sucesso.")).toBeInTheDocument();
    // FE-04: montagem + recarga (o debounce nao dispara mais na montagem).
    expect(api.apiListEstoque).toHaveBeenCalledTimes(2);
  });

  it.each<[string, string, string]>([
    ["403 vira mensagem amigavel", "403 forbidden", "Voce nao tem permissao para criar ou editar registros de estoque"],
    ["outro erro passa direto", "saldo invalido", "saldo invalido"],
  ])("erro ao salvar: %s", async (_n, erro, msg) => {
    api.apiUpdateEstoque.mockRejectedValue(new Error(erro));
    render(<EstoquePage />);
    await screen.findByText("SKU-5");
    await esperarDebounce();
    await userEvent.click((await screen.findAllByTitle("Editar registro de estoque"))[0]);
    const d = await dialogo();
    await userEvent.click(within(d).getByRole("button", { name: "Salvar alteracoes" }));
    expect(await within(d).findByText(new RegExp(msg))).toBeInTheDocument();
  });
});

// ─── FE-10 (c): acao deu certo, mas a recarga falhou -> um unico alerta ──────

const ERRO_REDE = "Failed to fetch";
const recargaFalhou = (sucesso: string, erro: string) =>
  `${sucesso} Porem, nao foi possivel recarregar a lista (${erro}). Os dados exibidos podem estar desatualizados.`;

/** Exatamente um alerta, vermelho, com o texto esperado; nenhum de sucesso. */
async function unicoAlertaDeErro(msg: string, sucesso: string) {
  expect(await screen.findByText(msg)).toBeInTheDocument();
  const alertas = screen.getAllByRole("alert");
  expect(alertas).toHaveLength(1);
  expect(alertas[0]).toHaveClass("bg-red-50");
  expect(screen.queryByText(sucesso)).not.toBeInTheDocument();
}

describe("FE-10 - recarga que falha depois da acao", () => {
  it.each<[string, boolean, string]>([
    ["email enviado", true, 'Usuario "Caio" criado com sucesso.'],
    [
      "email NAO enviado",
      false,
      'Usuario "Caio" foi criado, mas o email com a senha inicial NAO pode ser enviado — verifique a configuracao de SMTP.',
    ],
  ])("usuarios: criar (%s)", async (_n, enviado, sucesso) => {
    api.apiListUsers.mockResolvedValueOnce(pagina([bia])).mockRejectedValueOnce(new TypeError(ERRO_REDE));
    api.apiCreateUser.mockResolvedValue({ ...bia, id: 3, nome: "Caio", email_enviado: enviado });
    render(<UsuariosPage />);
    await screen.findByText("Bia");
    await userEvent.click(screen.getByRole("button", { name: "+ Novo Usuario" }));
    const d = await dialogo();
    await waitFor(() => expect(within(d).getByLabelText("Vendedor vinculado (opcional)")).toBeEnabled());
    await userEvent.type(within(d).getByLabelText("Nome completo"), "Caio");
    await userEvent.type(within(d).getByLabelText("Email"), "caio@x.com");
    await userEvent.click(within(d).getByRole("button", { name: "Criar usuario" }));
    await unicoAlertaDeErro(recargaFalhou(sucesso, ERRO_REDE), sucesso);
    expect(screen.getByText("Bia")).toBeInTheDocument();
  });

  describe("vendedores (concluirAcao)", () => {
    beforeEach(() => {
      api.apiListVendedores.mockResolvedValueOnce([ativo, inativo]).mockRejectedValueOnce(new TypeError(ERRO_REDE));
      api.apiGetVendedor.mockImplementation(async (id: number) => ({
        ...(id === 3 ? ativo : inativo),
        data_admissao: "2020-01-01",
        meta_mensal: 100,
        clientes: [],
      }));
    });

    it.each<[string, string, "apiDeleteVendedor" | "apiReativarVendedor", string, string]>([
      ["inativar", "Vend 3", "apiDeleteVendedor", 'Vendedor "Vend 3" inativado com sucesso.', "Ativo"],
      ["reativar", "Vend 4", "apiReativarVendedor", 'Vendedor "Vend 4" reativado com sucesso.', "Inativo"],
    ])("%s", async (_n, linha, fn, sucesso, statusAntigo) => {
      vi.spyOn(window, "confirm").mockReturnValue(true);
      api[fn].mockResolvedValue({});
      render(<VendedoresPage />);
      const row = (await screen.findByText(linha)).closest("tr")!;
      await userEvent.click(within(row).getByRole("button", { name: /Ativo|Inativo/ }));
      await unicoAlertaDeErro(recargaFalhou(sucesso, ERRO_REDE), sucesso);
      expect(api.apiListVendedores).toHaveBeenCalledTimes(2);
      // Lista anterior mantida (a linha continua com o status antigo).
      const rowDepois = screen.getByText(linha).closest("tr")!;
      expect(within(rowDepois).getByRole("button", { name: statusAntigo })).toBeInTheDocument();
    });

    it("criar", async () => {
      api.apiCreateVendedor.mockResolvedValue({ ...ativo, id: 9, nome: "Novo" });
      render(<VendedoresPage />);
      await screen.findByText("Vend 3");
      await userEvent.click(screen.getByRole("button", { name: "+ Novo Vendedor" }));
      const d = await dialogo();
      await userEvent.type(within(d).getByLabelText("Nome"), "Novo");
      await userEvent.type(within(d).getByLabelText("Regiao"), "Sul");
      await userEvent.type(within(d).getByLabelText("UF"), "pr");
      await userEvent.click(within(d).getByRole("button", { name: "Criar vendedor" }));
      const sucesso = 'Vendedor "Novo" criado com sucesso.';
      await unicoAlertaDeErro(recargaFalhou(sucesso, ERRO_REDE), sucesso);
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    });

    it("editar", async () => {
      api.apiUpdateVendedor.mockResolvedValue({});
      render(<VendedoresPage />);
      await userEvent.click(await screen.findByText("Vend 3"));
      const d = await dialogo();
      await within(d).findByText("Nenhum cliente vinculado a este vendedor.");
      await userEvent.click(within(d).getByRole("button", { name: "Salvar alteracoes" }));
      const sucesso = 'Vendedor "Vend 3" atualizado com sucesso.';
      await unicoAlertaDeErro(recargaFalhou(sucesso, ERRO_REDE), sucesso);
    });

    it("a recarga que da certo mostra so o sucesso (sem alerta de erro)", async () => {
      api.apiListVendedores.mockReset();
      api.apiListVendedores.mockResolvedValue([ativo, inativo]);
      vi.spyOn(window, "confirm").mockReturnValue(true);
      api.apiDeleteVendedor.mockResolvedValue({});
      render(<VendedoresPage />);
      const row = (await screen.findByText("Vend 3")).closest("tr")!;
      await userEvent.click(within(row).getByRole("button", { name: /Ativo/ }));
      expect(await screen.findByText('Vendedor "Vend 3" inativado com sucesso.')).toBeInTheDocument();
      const alertas = screen.getAllByRole("alert");
      expect(alertas).toHaveLength(1);
      expect(alertas[0]).toHaveClass("bg-green-50");
    });
  });

  it("produtos: criar", async () => {
    api.apiListProdutos.mockResolvedValueOnce(pagina([perfume])).mockRejectedValueOnce(new TypeError(ERRO_REDE));
    api.apiCreateProduto.mockResolvedValue({ ...perfume, id: 51, descricao: "Outro" });
    render(<ProdutosPage />);
    await screen.findByText("Perfume 50");
    await esperarDebounce();
    await userEvent.click(screen.getByRole("button", { name: "+ Novo Produto" }));
    const w = within(await dialogo());
    await userEvent.type(w.getByLabelText("SKU"), "P51");
    await userEvent.type(w.getByLabelText("Descricao"), "Outro");
    await userEvent.type(w.getByLabelText("Categoria"), "C");
    await userEvent.type(w.getByLabelText("Marca"), "M");
    await userEvent.type(w.getByLabelText("Preco de tabela"), "1");
    await userEvent.type(w.getByLabelText("Custo unitario"), "1");
    await userEvent.type(w.getByLabelText("Unidade"), "UN");
    await userEvent.click(w.getByRole("button", { name: "Criar produto" }));
    const sucesso = 'Produto "Outro" criado com sucesso.';
    await unicoAlertaDeErro(recargaFalhou(sucesso, ERRO_REDE), sucesso);
    expect(screen.getByText("Perfume 50")).toBeInTheDocument();
  });

  it("estoque: criar", async () => {
    api.apiListEstoque.mockResolvedValueOnce(pagina([registro])).mockRejectedValueOnce(new TypeError(ERRO_REDE));
    api.apiCreateEstoque.mockResolvedValue({ ...registro, id: 6, sku: "SKU-6" });
    render(<EstoquePage />);
    await screen.findByText("SKU-5");
    await esperarDebounce();
    await userEvent.click(screen.getByRole("button", { name: "+ Novo registro" }));
    const d = await dialogo();
    await userEvent.type(within(d).getByLabelText("SKU"), "SKU-6");
    await userEvent.type(within(d).getByLabelText("Data do snapshot"), "2026-01-01");
    await userEvent.type(within(d).getByLabelText("Saldo"), "1");
    await userEvent.click(within(d).getByRole("button", { name: "Criar registro" }));
    const sucesso = "Registro de estoque #6 (SKU-6) criado com sucesso.";
    await unicoAlertaDeErro(recargaFalhou(sucesso, ERRO_REDE), sucesso);
    expect(screen.getByText("SKU-5")).toBeInTheDocument();
  });
});

// ─── FE-11: mensagem de sucesso temporaria nas telas ─────────────────────────

type CasoMsg = [string, () => void, () => ReturnType<typeof render>, string, string];
const CASOS_MSG: CasoMsg[] = [
  [
    "usuarios",
    () => {
      api.apiListUsers.mockResolvedValue(pagina([bia]));
      api.apiToggleUserStatus.mockResolvedValue({ ...bia, ativo: false });
    },
    () => render(<UsuariosPage />),
    "Bia",
    "Usuario inativado com sucesso.",
  ],
  [
    "vendedores",
    () => {
      api.apiListVendedores.mockResolvedValue([ativo]);
      api.apiDeleteVendedor.mockResolvedValue({});
    },
    () => render(<VendedoresPage />),
    "Vend 3",
    'Vendedor "Vend 3" inativado com sucesso.',
  ],
  [
    "produtos",
    () => {
      api.apiListProdutos.mockResolvedValue(pagina([perfume]));
      api.apiToggleProdutoStatus.mockResolvedValue({ ...perfume, ativo: false });
    },
    () => render(<ProdutosPage />),
    "Perfume 50",
    "Produto inativado com sucesso.",
  ],
];

describe("FE-11 - mensagem temporaria (usuarios, vendedores, produtos)", () => {
  async function inativar(
    preparar: () => void,
    montar: () => ReturnType<typeof render>,
    texto: string,
    msg: string
  ) {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const u = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    vi.spyOn(window, "confirm").mockReturnValue(true);
    preparar();
    const r = montar();
    await screen.findByText(texto);
    await act(async () => {
      vi.advanceTimersByTime(420);
    });
    await u.click(screen.getByTitle("Clique para inativar"));
    await screen.findByText(msg);
    return r;
  }

  it.each(CASOS_MSG)("%s: o sucesso some apos 4 s", async (_n, preparar, montar, texto, msg) => {
    try {
      await inativar(preparar, montar, texto, msg);
      await act(async () => {
        vi.advanceTimersByTime(3000);
      });
      expect(screen.getByText(msg)).toBeInTheDocument();
      await act(async () => {
        vi.advanceTimersByTime(1100);
      });
      expect(screen.queryByText(msg)).not.toBeInTheDocument();
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it.each(CASOS_MSG)("%s: fechar o alerta cancela o timer", async (_n, preparar, montar, texto, msg) => {
    try {
      await inativar(preparar, montar, texto, msg);
      await act(async () => {
        screen.getByRole("button", { name: "Fechar alerta" }).click();
      });
      expect(screen.queryByText(msg)).not.toBeInTheDocument();
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it.each(CASOS_MSG)("%s: unmount antes dos 4 s nao deixa timer pendente", async (_n, preparar, montar, texto, msg) => {
    const erro = vi.spyOn(console, "error");
    try {
      const { unmount } = await inativar(preparar, montar, texto, msg);
      await act(async () => {
        vi.advanceTimersByTime(2000);
      });
      unmount();
      expect(vi.getTimerCount()).toBe(0);
      await act(async () => {
        vi.advanceTimersByTime(5000);
      });
      expect(erro).not.toHaveBeenCalled();
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });
});
