import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { MeResponse } from "@/lib/types";

const useSessionUserMock = vi.fn<() => MeResponse | null>();
const useVendedorDesligadoMock = vi.fn<() => boolean>();
vi.mock("@/lib/session", () => ({
  useSessionUser: () => useSessionUserMock(),
  useVendedorDesligado: () => useVendedorDesligadoMock(),
}));

const logoutMock = vi.fn();
vi.mock("@/lib/auth", () => ({ logout: () => logoutMock() }));

import { Navbar } from "./Navbar";

const CARTEIRA = ["Clientes", "Oportunidades", "Visitas", "Pagamentos", "Pedidos"];
const ADMIN_ITENS = ["Auditoria de Senha", "Vendedores", "Usuários", "Produtos", "Estoque"];

function user(role: MeResponse["role"]): MeResponse {
  return { id: 1, nome: "ana", email: "a@x", role, ativo: true, id_vendedor: 7 };
}

describe("Navbar", () => {
  beforeEach(() => {
    useSessionUserMock.mockReset();
    useVendedorDesligadoMock.mockReset();
    logoutMock.mockReset();
  });

  it.each<[string, MeResponse["role"], boolean, boolean, boolean]>([
    // nome, role, desligado, ve carteira, ve itens admin
    ["normal ativo", "normal", false, true, false],
    ["normal desligado", "normal", true, false, false],
    ["admin", "admin", false, true, true],
  ])("%s", (_n, role, desligado, veCarteira, veAdmin) => {
    useSessionUserMock.mockReturnValue(user(role));
    useVendedorDesligadoMock.mockReturnValue(desligado);
    render(<Navbar />);

    // Dashboard e Trocar Senha continuam acessiveis para todos.
    expect(screen.getByRole("link", { name: "Dashboard" })).toHaveAttribute("href", "/dashboard");
    expect(screen.getByRole("link", { name: "Trocar Senha" })).toHaveAttribute(
      "href",
      "/trocar-senha"
    );

    for (const label of CARTEIRA) {
      if (veCarteira) expect(screen.getByRole("link", { name: label })).toBeInTheDocument();
      else expect(screen.queryByRole("link", { name: label })).not.toBeInTheDocument();
    }
    // Dropdowns vazios nao sao renderizados.
    expect(!!screen.queryByRole("button", { name: "ERP" })).toBe(veCarteira);
    expect(!!screen.queryByRole("button", { name: "CRM" })).toBe(veCarteira);

    for (const label of ADMIN_ITENS) {
      expect(!!screen.queryByRole("link", { name: label })).toBe(veAdmin);
    }
    expect(screen.getByText(role === "admin" ? "Administrador" : "normal")).toBeInTheDocument();
    expect(screen.getByText("A")).toBeInTheDocument();
  });

  it("sem usuario: nao mostra menus, avatar '?'", () => {
    useSessionUserMock.mockReturnValue(null);
    useVendedorDesligadoMock.mockReturnValue(false);
    render(<Navbar />);
    expect(screen.queryByRole("link", { name: "Dashboard" })).not.toBeInTheDocument();
    expect(screen.getByText("?")).toBeInTheDocument();
  });

  it("Sair chama logout", async () => {
    useSessionUserMock.mockReturnValue(user("normal"));
    useVendedorDesligadoMock.mockReturnValue(false);
    render(<Navbar />);
    await userEvent.click(screen.getByRole("button", { name: "Sair" }));
    expect(logoutMock).toHaveBeenCalledTimes(1);
  });
});
