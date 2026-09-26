/**
 * Layout raiz (metadata + <html>/<body>) e pagina inicial, que apenas decide
 * para onde mandar o usuario conforme a sessao em cache.
 */
import { render, waitFor } from "@testing-library/react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const replaceMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: replaceMock }) }));

import RootLayout, { metadata } from "@/app/layout";
import Home from "@/app/page";

beforeEach(() => {
  replaceMock.mockReset();
});

describe("RootLayout", () => {
  it("define titulo e descricao da aplicacao", () => {
    expect(metadata.title).toBe("RotaPerfumes - Sistema de Gestao");
    expect(metadata.description).toBe("Sistema de gestao de clientes, estoque e pedidos");
  });

  it("renderiza <html lang=pt-BR> com o conteudo dentro do <body>", () => {
    const html = renderToStaticMarkup(
      <RootLayout>
        <p id="filho">conteudo</p>
      </RootLayout>
    );
    expect(html).toMatch(/^<html lang="pt-BR">/);
    expect(html).toContain('<body class="antialiased"><p id="filho">conteudo</p></body>');
  });
});

describe("Home (/)", () => {
  it.each<[string, string | null, string]>([
    ["sem sessao", null, "/login"],
    ["com sessao", JSON.stringify({ id: 1, nome: "Ana", email: "a@x", role: "normal", ativo: true }), "/dashboard"],
  ])("%s redireciona para %s", async (_n, salvo, destino) => {
    if (salvo) localStorage.setItem("auth_user", salvo);
    const { container } = render(<Home />);
    expect(container.querySelector(".animate-spin")).not.toBeNull();
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith(destino));
    expect(replaceMock).toHaveBeenCalledTimes(1);
  });

  it("sessao corrompida no localStorage e tratada como sem sessao", async () => {
    localStorage.setItem("auth_user", "{quebrado");
    render(<Home />);
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/login"));
  });
});
