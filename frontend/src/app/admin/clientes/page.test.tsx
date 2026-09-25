import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/apiError";
import type { Cliente, MeResponse } from "@/lib/types";

// CarteiraGuard vira passthrough: o bloqueio de desligado e testado no
// proprio guard; aqui interessa a defesa do botao "+ Novo Cliente".
vi.mock("@/components/layout/CarteiraGuard", () => ({
  CarteiraGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

const useSessionUserMock = vi.fn<() => MeResponse | null>();
const useVendedorDesligadoMock = vi.fn<() => boolean>();
vi.mock("@/lib/session", () => ({
  useSessionUser: () => useSessionUserMock(),
  useVendedorDesligado: () => useVendedorDesligadoMock(),
}));

const api = vi.hoisted(() => ({
  apiListClientes: vi.fn(),
  apiToggleClienteStatus: vi.fn(),
  apiCreateCliente: vi.fn(),
  apiUpdateCliente: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import ClientesPage from "./page";

function user(extra: Partial<MeResponse>): MeResponse {
  return { id: 1, nome: "Ana", email: "a@x", role: "normal", ativo: true, ...extra };
}

function cliente(id: number, razao: string, extra: Partial<Cliente> = {}): Cliente {
  return {
    cliente_id_origem: id,
    razao_social: razao,
    cnpj: "12345678000199",
    segmento: "Varejo",
    cidade: "Sao Paulo",
    uf: "SP",
    bairro: "Centro",
    data_cadastro: "2026-01-10",
    ativo: true,
    ...extra,
  } as Cliente;
}

function page(data: Cliente[]) {
  return { data, page: 1, limit: 20, total: data.length, pages: 1 };
}

beforeEach(() => {
  vi.clearAllMocks();
  useVendedorDesligadoMock.mockReturnValue(false);
  api.apiListClientes.mockResolvedValue(page([cliente(10, "Loja A")]));
});

describe("Clientes - botao + Novo Cliente (SEC-01)", () => {
  it.each<[string, Partial<MeResponse>, boolean, boolean]>([
    ["admin sem vendedor", { role: "admin", id_vendedor: null }, false, true],
    ["normal com vendedor", { id_vendedor: 7 }, false, true],
    ["normal sem vendedor", { id_vendedor: null }, false, false],
    ["normal sem o campo id_vendedor", {}, false, false],
    ["normal com vendedor desligado", { id_vendedor: 7 }, true, false],
  ])("%s -> habilitado=%s", async (_n, u, desligado, habilitado) => {
    useSessionUserMock.mockReturnValue(user(u));
    useVendedorDesligadoMock.mockReturnValue(desligado);
    render(<ClientesPage />);
    const botao = await screen.findByRole("button", { name: "+ Novo Cliente" });
    if (habilitado) {
      expect(botao).toBeEnabled();
    } else {
      expect(botao).toBeDisabled();
      await userEvent.click(botao);
      expect(screen.queryByText("Novo Cliente", { selector: "h2,h3" })).not.toBeInTheDocument();
    }
  });

  it("sessao ainda sem /me (user null) mantem o botao desabilitado", async () => {
    useSessionUserMock.mockReturnValue(null);
    render(<ClientesPage />);
    expect(await screen.findByRole("button", { name: "+ Novo Cliente" })).toBeDisabled();
  });
});

async function preencherNovoCliente() {
  await userEvent.click(await screen.findByRole("button", { name: "+ Novo Cliente" }));
  const dialog = await screen.findByRole("dialog");
  const w = within(dialog);
  await userEvent.type(w.getByLabelText("Razao social"), "Loja Nova");
  await userEvent.type(w.getByLabelText("CNPJ"), "11222333000144");
  await userEvent.type(w.getByLabelText("Segmento"), "Varejo");
  await userEvent.type(w.getByLabelText("Cidade"), "Campinas");
  await userEvent.type(w.getByLabelText("UF"), "SP");
  await userEvent.click(w.getByRole("button", { name: "Criar cliente" }));
  return dialog;
}

describe("Clientes - criacao (SEC-01)", () => {
  it("apos criar, recarrega a lista e o cliente novo aparece para o usuario normal", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    render(<ClientesPage />);
    await screen.findByText("#10 - Loja A");

    api.apiCreateCliente.mockResolvedValue(cliente(11, "Loja Nova"));
    api.apiListClientes.mockResolvedValue(
      page([cliente(10, "Loja A"), cliente(11, "Loja Nova")])
    );
    const chamadasAntes = api.apiListClientes.mock.calls.length;
    await preencherNovoCliente();

    expect(await screen.findByText("#11 - Loja Nova")).toBeInTheDocument();
    expect(api.apiListClientes.mock.calls.length).toBeGreaterThan(chamadasAntes);
    expect(screen.getByText('Cliente "Loja Nova" criado com sucesso.')).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("403 ao criar mostra a mensagem da API no modal", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    api.apiCreateCliente.mockRejectedValue(
      new ApiError(403, "usuário sem vendedor vinculado")
    );
    render(<ClientesPage />);
    const dialog = await preencherNovoCliente();
    expect(
      await within(dialog).findByText("usuário sem vendedor vinculado")
    ).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
});

describe("Clientes - erros da API (SEC-01)", () => {
  it("404 ao editar fecha o modal, mostra a mensagem da API e recarrega a lista", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    api.apiUpdateCliente.mockRejectedValue(new ApiError(404, "cliente não encontrado"));
    render(<ClientesPage />);
    await screen.findByText("#10 - Loja A");
    const chamadasAntes = api.apiListClientes.mock.calls.length;
    api.apiListClientes.mockResolvedValue(page([]));

    await userEvent.click(screen.getByRole("button", { name: "Editar" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar alteracoes" }));

    expect(await screen.findByText("cliente não encontrado")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(api.apiListClientes.mock.calls.length).toBeGreaterThan(chamadasAntes);
    await waitFor(() => expect(screen.queryByText("#10 - Loja A")).not.toBeInTheDocument());
  });

  it("404 ao inativar mostra a mensagem da API e recarrega a lista", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api.apiToggleClienteStatus.mockRejectedValue(new ApiError(404, "cliente não encontrado"));
    render(<ClientesPage />);
    await screen.findByText("#10 - Loja A");
    const chamadasAntes = api.apiListClientes.mock.calls.length;

    api.apiListClientes.mockResolvedValue(page([]));
    await userEvent.click(screen.getByRole("button", { name: /Ativo/ }));

    expect(await screen.findByText("cliente não encontrado")).toBeInTheDocument();
    expect(api.apiListClientes.mock.calls.length).toBeGreaterThan(chamadasAntes);
    await waitFor(() => expect(screen.queryByText("#10 - Loja A")).not.toBeInTheDocument());
  });

  it("403 ao inativar mostra a mensagem da API", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api.apiToggleClienteStatus.mockRejectedValue(
      new ApiError(403, "usuário sem vendedor vinculado")
    );
    render(<ClientesPage />);
    await screen.findByText("#10 - Loja A");
    await userEvent.click(screen.getByRole("button", { name: /Ativo/ }));
    expect(await screen.findByText("usuário sem vendedor vinculado")).toBeInTheDocument();
  });
});

describe("Clientes - motivo do bloqueio e fallbacks (SEC-01)", () => {
  it.each<[string, Partial<MeResponse>, boolean, string]>([
    [
      "normal sem vendedor",
      { id_vendedor: null },
      false,
      "Usuario sem vendedor vinculado: solicite o vinculo a um administrador.",
    ],
    [
      "normal com vendedor desligado",
      { id_vendedor: 7 },
      true,
      "Vendedor desligado: cadastro de clientes bloqueado.",
    ],
  ])("%s: botao mostra o motivo (title e texto)", async (_n, u, desligado, motivo) => {
    useSessionUserMock.mockReturnValue(user(u));
    useVendedorDesligadoMock.mockReturnValue(desligado);
    render(<ClientesPage />);
    const botao = await screen.findByRole("button", { name: "+ Novo Cliente" });
    expect(botao).toHaveAttribute("title", motivo);
    expect(screen.getByText(motivo)).toBeInTheDocument();
  });

  it("habilitado nao mostra motivo", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    render(<ClientesPage />);
    const botao = await screen.findByRole("button", { name: "+ Novo Cliente" });
    expect(botao).not.toHaveAttribute("title");
  });

  it("403 sem mensagem ao criar usa o texto padrao", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    api.apiCreateCliente.mockRejectedValue(new ApiError(403, ""));
    render(<ClientesPage />);
    const dialog = await preencherNovoCliente();
    expect(await within(dialog).findByText("usuário sem vendedor vinculado")).toBeInTheDocument();
  });

  it("404 sem mensagem ao editar usa o texto padrao", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    api.apiUpdateCliente.mockRejectedValue(new ApiError(404, ""));
    render(<ClientesPage />);
    await screen.findByText("#10 - Loja A");
    await userEvent.click(screen.getByRole("button", { name: "Editar" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText("cliente não encontrado")).toBeInTheDocument();
  });

  it("editar com sucesso atualiza a linha sem recarregar a lista", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    api.apiUpdateCliente.mockResolvedValue(cliente(10, "Loja A Editada"));
    render(<ClientesPage />);
    await screen.findByText("#10 - Loja A");
    const antes = api.apiListClientes.mock.calls.length;
    await userEvent.click(screen.getByRole("button", { name: "Editar" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText("#10 - Loja A Editada")).toBeInTheDocument();
    expect(screen.getByText('Cliente "Loja A Editada" atualizado com sucesso.')).toBeInTheDocument();
    expect(api.apiListClientes.mock.calls.length).toBe(antes);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("inativar com sucesso troca o status na linha", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api.apiToggleClienteStatus.mockResolvedValue(cliente(10, "Loja A", { ativo: false }));
    render(<ClientesPage />);
    await screen.findByText("#10 - Loja A");
    await userEvent.click(screen.getByRole("button", { name: /Ativo/ }));
    expect(await screen.findByText("Cliente inativado com sucesso.")).toBeInTheDocument();
    expect(api.apiToggleClienteStatus).toHaveBeenCalledWith(10, false);
  });

  it("cancelar a confirmacao nao chama a API", async () => {
    useSessionUserMock.mockReturnValue(user({ id_vendedor: 7 }));
    vi.spyOn(window, "confirm").mockReturnValue(false);
    render(<ClientesPage />);
    await screen.findByText("#10 - Loja A");
    await userEvent.click(screen.getByRole("button", { name: /Ativo/ }));
    expect(api.apiToggleClienteStatus).not.toHaveBeenCalled();
  });
});
