/**
 * Layout das telas administrativas: todo o conteudo fica atras do
 * ProtectedRoute, com a Navbar no topo e as paginas dentro de <main>.
 */
import { render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/components/layout/ProtectedRoute", () => ({
  ProtectedRoute: ({ children }: { children: ReactNode }) => (
    <section data-testid="protected">{children}</section>
  ),
}));
vi.mock("@/components/layout/Navbar", () => ({
  Navbar: () => <nav data-testid="navbar">navbar</nav>,
}));

import AdminLayout from "@/app/admin/layout";

describe("AdminLayout", () => {
  it("envolve Navbar e conteudo no ProtectedRoute", () => {
    render(
      <AdminLayout>
        <h1>Pagina admin</h1>
      </AdminLayout>
    );
    const protegido = screen.getByTestId("protected");
    expect(within(protegido).getByTestId("navbar")).toBeInTheDocument();
    expect(within(protegido).getByRole("heading", { name: "Pagina admin" })).toBeInTheDocument();
  });

  it("renderiza as paginas dentro de <main>, depois da Navbar", () => {
    render(
      <AdminLayout>
        <p>conteudo</p>
      </AdminLayout>
    );
    const main = screen.getByRole("main");
    expect(within(main).getByText("conteudo")).toBeInTheDocument();
    expect(within(main).queryByTestId("navbar")).toBeNull();
    const navbar = screen.getByTestId("navbar");
    expect(navbar.compareDocumentPosition(main) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
