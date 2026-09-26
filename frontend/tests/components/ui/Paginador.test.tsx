/**
 * Paginador (rodape de paginacao extraido das listagens no Lote 8 / FE-10):
 * o destino dos botoes e sempre calculado a partir da pagina exibida.
 */
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Paginador } from "@/components/ui/Paginador";

const TITULOS = ["Primeira pagina", "Pagina anterior", "Proxima pagina", "Ultima pagina"] as const;

describe("Paginador", () => {
  it.each<[number]>([[0], [1], [-1]])("com %i pagina(s) nao renderiza nada", (paginas) => {
    const { container } = render(<Paginador pagina={1} paginas={paginas} onIrPara={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("mostra 'Pagina N de M'", () => {
    render(<Paginador pagina={2} paginas={5} onIrPara={vi.fn()} />);
    const texto = screen.getByText(/Pagina/, { selector: "div" });
    expect(texto).toHaveTextContent("Pagina 2 de 5");
  });

  it.each<[string, number, number, boolean[]]>([
    // [primeira, anterior, proxima, ultima] desabilitados
    ["primeira pagina", 1, 3, [true, true, false, false]],
    ["pagina do meio", 2, 3, [false, false, false, false]],
    ["ultima pagina", 3, 3, [false, false, true, true]],
    ["pagina alem do total", 5, 3, [false, false, true, true]],
  ])("%s: botoes habilitados/desabilitados", (_n, pagina, paginas, desabilitados) => {
    render(<Paginador pagina={pagina} paginas={paginas} onIrPara={vi.fn()} />);
    TITULOS.forEach((t, i) => {
      if (desabilitados[i]) expect(screen.getByTitle(t)).toBeDisabled();
      else expect(screen.getByTitle(t)).toBeEnabled();
    });
  });

  it.each<[string, number, number, (typeof TITULOS)[number], number]>([
    ["primeira", 3, 5, "Primeira pagina", 1],
    ["anterior", 3, 5, "Pagina anterior", 2],
    ["proxima", 3, 5, "Proxima pagina", 4],
    ["ultima", 3, 5, "Ultima pagina", 5],
    ["anterior a partir da 2", 2, 5, "Pagina anterior", 1],
    ["proxima a partir da penultima", 4, 5, "Proxima pagina", 5],
  ])("%s: chama onIrPara com o destino calculado da pagina exibida", async (_n, pagina, paginas, titulo, destino) => {
    const onIrPara = vi.fn();
    render(<Paginador pagina={pagina} paginas={paginas} onIrPara={onIrPara} />);
    await userEvent.click(screen.getByTitle(titulo));
    expect(onIrPara).toHaveBeenCalledTimes(1);
    expect(onIrPara).toHaveBeenCalledWith(destino);
  });

  it("botao desabilitado nao chama onIrPara", async () => {
    const onIrPara = vi.fn();
    render(<Paginador pagina={1} paginas={2} onIrPara={onIrPara} />);
    await userEvent.click(screen.getByTitle("Pagina anterior"));
    await userEvent.click(screen.getByTitle("Primeira pagina"));
    expect(onIrPara).not.toHaveBeenCalled();
  });
});
