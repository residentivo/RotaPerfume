import { render, screen, within } from "@testing-library/react";
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

let pathnameAtual = "/dashboard";
vi.mock("next/navigation", () => ({ usePathname: () => pathnameAtual }));

import { Navbar } from "@/components/layout/Navbar";

const CARTEIRA = ["Clientes", "Oportunidades", "Visitas", "Pagamentos", "Pedidos"];
const ADMIN_ITENS = ["Auditoria de Senha", "Vendedores", "Usuários", "Produtos", "Estoque"];

function user(role: MeResponse["role"]): MeResponse {
  return { id: 1, nome: "ana", email: "a@x", role, ativo: true, id_vendedor: 7 };
}

function botaoMenu() {
  return screen.getByRole("button", { name: /menu$/ });
}

function painel() {
  return document.getElementById(botaoMenu().getAttribute("aria-controls")!);
}

function sessao(role: MeResponse["role"] | null, desligado = false) {
  useSessionUserMock.mockReturnValue(role ? user(role) : null);
  useVendedorDesligadoMock.mockReturnValue(desligado);
}

beforeEach(() => {
  useSessionUserMock.mockReset();
  useVendedorDesligadoMock.mockReset();
  logoutMock.mockReset();
  pathnameAtual = "/dashboard";
});

describe("Navbar", () => {
  it.each<[string, MeResponse["role"], boolean, boolean, boolean]>([
    // nome, role, desligado, ve carteira, ve itens admin
    ["normal ativo", "normal", false, true, false],
    ["normal desligado", "normal", true, false, false],
    ["admin", "admin", false, true, true],
  ])("%s", (_n, role, desligado, veCarteira, veAdmin) => {
    sessao(role, desligado);
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
    sessao(null);
    render(<Navbar />);
    expect(screen.queryByRole("link", { name: "Dashboard" })).not.toBeInTheDocument();
    expect(screen.getByText("?")).toBeInTheDocument();
  });

  it("Sair chama logout", async () => {
    sessao("normal");
    render(<Navbar />);
    await userEvent.click(screen.getByRole("button", { name: "Sair" }));
    expect(logoutMock).toHaveBeenCalledTimes(1);
  });
});

describe("Navbar - menu hamburguer (UX-01)", () => {
  it("renderiza o botao fechado, acessivel e oculto no desktop (lg)", () => {
    sessao("normal");
    render(<Navbar />);
    const botao = botaoMenu();
    expect(botao).toHaveAccessibleName("Abrir menu");
    expect(botao).toHaveAttribute("aria-expanded", "false");
    expect(botao).toHaveAttribute("aria-controls", "navbar-menu-mobile");
    // Alvo de toque de 44px (h-11/w-11) e escondido a partir de lg.
    expect(botao).toHaveClass("lg:hidden", "h-11", "w-11");
    expect(painel()).toBeNull();
  });

  it("abre e fecha alternando aria-expanded", async () => {
    sessao("normal");
    render(<Navbar />);

    await userEvent.click(botaoMenu());
    expect(botaoMenu()).toHaveAttribute("aria-expanded", "true");
    expect(botaoMenu()).toHaveAccessibleName("Fechar menu");
    expect(painel()).toBeInTheDocument();
    expect(painel()).toHaveClass("lg:hidden");

    await userEvent.click(botaoMenu());
    expect(botaoMenu()).toHaveAttribute("aria-expanded", "false");
    expect(painel()).toBeNull();
  });

  it.each<[string, MeResponse["role"], boolean, string[], string[]]>([
    ["normal ativo", "normal", false, ["Dashboard", "Trocar Senha", ...CARTEIRA], ADMIN_ITENS],
    [
      "normal desligado",
      "normal",
      true,
      ["Dashboard", "Trocar Senha"],
      [...CARTEIRA, ...ADMIN_ITENS],
    ],
    ["admin", "admin", false, ["Dashboard", "Trocar Senha", ...CARTEIRA, ...ADMIN_ITENS], []],
  ])("painel mostra os links conforme o papel: %s", async (_n, role, desligado, ve, naoVe) => {
    sessao(role, desligado);
    render(<Navbar />);
    await userEvent.click(botaoMenu());

    const p = within(painel()!);
    for (const label of ve) expect(p.getByRole("link", { name: label })).toBeInTheDocument();
    for (const label of naoVe) expect(p.queryByRole("link", { name: label })).toBeNull();
    expect(p.getAllByRole("link")).toHaveLength(ve.length);
    // Grupos sem itens nao aparecem no painel.
    expect(!!p.queryByText("ERP")).toBe(!desligado);
    expect(!!p.queryByText("CRM")).toBe(!desligado);
    expect(!!p.queryByText("Administração")).toBe(role === "admin");
    // Info do usuario e Sair tambem ficam no painel.
    expect(p.getByText("ana")).toBeInTheDocument();
    expect(p.getByText(role === "admin" ? "Administrador" : "normal")).toBeInTheDocument();
    expect(p.getByRole("button", { name: "Sair" })).toBeInTheDocument();
  });

  it("links do painel tem alvo de toque >= 44px", async () => {
    sessao("admin");
    render(<Navbar />);
    await userEvent.click(botaoMenu());
    for (const link of within(painel()!).getAllByRole("link")) {
      expect(link).toHaveClass("min-h-[44px]");
    }
  });

  it("fecha ao clicar num link", async () => {
    sessao("normal");
    render(<Navbar />);
    await userEvent.click(botaoMenu());
    const link = within(painel()!).getByRole("link", { name: "Clientes" });
    expect(link).toHaveAttribute("href", "/admin/clientes");
    // jsdom nao navega; evita o aviso de navegacao nao implementada.
    link.addEventListener("click", (e) => e.preventDefault());
    await userEvent.click(link);
    expect(botaoMenu()).toHaveAttribute("aria-expanded", "false");
    expect(painel()).toBeNull();
  });

  it("fecha com Esc e devolve o foco ao botao", async () => {
    sessao("normal");
    render(<Navbar />);
    await userEvent.click(botaoMenu());
    expect(painel()).toBeInTheDocument();

    await userEvent.keyboard("{Escape}");
    expect(botaoMenu()).toHaveAttribute("aria-expanded", "false");
    expect(painel()).toBeNull();
    expect(botaoMenu()).toHaveFocus();
  });

  it("outras teclas nao fecham o menu", async () => {
    sessao("normal");
    render(<Navbar />);
    await userEvent.click(botaoMenu());
    await userEvent.keyboard("a");
    expect(painel()).toBeInTheDocument();
  });

  it("fecha ao mudar de rota", async () => {
    sessao("normal");
    const { rerender } = render(<Navbar />);
    await userEvent.click(botaoMenu());
    expect(painel()).toBeInTheDocument();

    pathnameAtual = "/admin/clientes";
    rerender(<Navbar />);
    expect(botaoMenu()).toHaveAttribute("aria-expanded", "false");
    expect(painel()).toBeNull();
  });

  it("Sair no painel chama logout", async () => {
    sessao("normal");
    render(<Navbar />);
    await userEvent.click(botaoMenu());
    await userEvent.click(within(painel()!).getByRole("button", { name: "Sair" }));
    expect(logoutMock).toHaveBeenCalledTimes(1);
  });

  it("sem usuario: painel so tem Sair", async () => {
    sessao(null);
    render(<Navbar />);
    await userEvent.click(botaoMenu());
    const p = within(painel()!);
    expect(p.queryAllByRole("link")).toHaveLength(0);
    expect(p.getByRole("button", { name: "Sair" })).toBeInTheDocument();
  });
});
