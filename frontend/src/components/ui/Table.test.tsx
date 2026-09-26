import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Table, type Column } from "./Table";

type Linha = { id: number; nome: string };

const colunas: Column<Linha>[] = [
  { key: "id", header: "ID" },
  { key: "nome", header: "Nome" },
];

function renderTabela(data: Linha[], erroCarga?: boolean) {
  return render(
    <Table
      columns={colunas}
      data={data}
      keyExtractor={(l) => l.id}
      emptyMessage="Nenhum item cadastrado."
      erroCarga={erroCarga}
    />
  );
}

describe("Table - erroCarga (FE-09)", () => {
  it("sem erro e sem dados: mostra o estado vazio", () => {
    renderTabela([]);
    expect(screen.getByText("Nenhum item cadastrado.")).toBeInTheDocument();
  });

  it("com erro e sem dados: nao mostra o estado vazio nem a tabela", () => {
    const { container } = renderTabela([], true);
    expect(screen.queryByText("Nenhum item cadastrado.")).not.toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(container).toBeEmptyDOMElement();
  });

  it("com erro e com dados (lista anterior mantida): mostra as linhas", () => {
    renderTabela([{ id: 1, nome: "Item 1" }], true);
    expect(screen.getByText("Item 1")).toBeInTheDocument();
    expect(screen.queryByText("Nenhum item cadastrado.")).not.toBeInTheDocument();
  });
});
