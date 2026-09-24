import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AVISO_VENDEDOR_DESLIGADO } from "@/lib/vendedorDesligado";

const useVendedorDesligadoMock = vi.fn<() => boolean>();
vi.mock("@/lib/session", () => ({
  useVendedorDesligado: () => useVendedorDesligadoMock(),
}));

import { CarteiraGuard, VendedorDesligadoAviso } from "./CarteiraGuard";

describe("CarteiraGuard", () => {
  beforeEach(() => {
    useVendedorDesligadoMock.mockReset();
  });

  it.each(["Clientes", "Pedidos", "Pagamentos", "Oportunidades", "Visitas"])(
    "desligado em %s: mostra titulo, aviso e link para o Dashboard, sem o conteudo",
    (title) => {
      useVendedorDesligadoMock.mockReturnValue(true);
      render(
        <CarteiraGuard title={title}>
          <p>conteudo da carteira</p>
        </CarteiraGuard>
      );

      expect(screen.getByRole("heading", { level: 1, name: title })).toBeInTheDocument();
      expect(screen.getByText(AVISO_VENDEDOR_DESLIGADO)).toBeInTheDocument();
      expect(screen.getByRole("link", { name: "Ir para o Dashboard" })).toHaveAttribute(
        "href",
        "/dashboard"
      );
      expect(screen.queryByText("conteudo da carteira")).not.toBeInTheDocument();
    }
  );

  it("nao desligado: renderiza apenas o conteudo", () => {
    useVendedorDesligadoMock.mockReturnValue(false);
    render(
      <CarteiraGuard title="Clientes">
        <p>conteudo da carteira</p>
      </CarteiraGuard>
    );

    expect(screen.getByText("conteudo da carteira")).toBeInTheDocument();
    expect(screen.queryByText(AVISO_VENDEDOR_DESLIGADO)).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Clientes" })).not.toBeInTheDocument();
  });
});

describe("VendedorDesligadoAviso", () => {
  it("renderiza o texto padrao e o link", () => {
    render(<VendedorDesligadoAviso />);
    expect(screen.getByText(AVISO_VENDEDOR_DESLIGADO)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Ir para o Dashboard" })).toHaveAttribute(
      "href",
      "/dashboard"
    );
  });
});
