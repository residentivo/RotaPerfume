/**
 * NEG-02: campo CNPJ do ClienteModal com CNPJ alfanumerico. O input aceita
 * letras (convertidas para maiusculas), aplica a mascara enquanto digita, o
 * DV aceita so digitos, a validacao espelha a do backend e o payload vai sem
 * mascara e em maiusculas.
 */
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Cliente } from "@/lib/types";
import { ClienteModal } from "@/components/admin/ClienteModal";

function montar(props: Partial<Parameters<typeof ClienteModal>[0]> = {}) {
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  render(
    <ClienteModal
      open
      mode="create"
      cliente={null}
      onClose={vi.fn()}
      onSubmit={onSubmit}
      {...props}
    />
  );
  return { onSubmit };
}

function campoCnpj(): HTMLInputElement {
  return screen.getByLabelText("CNPJ") as HTMLInputElement;
}

async function preencherResto(u: ReturnType<typeof userEvent.setup>) {
  await u.type(screen.getByLabelText("Razao social"), "Loja");
  await u.type(screen.getByLabelText("Segmento"), "Varejo");
  await u.type(screen.getByLabelText("Cidade"), "Santos");
  await u.type(screen.getByLabelText("UF"), "SP");
}

async function submeter(u: ReturnType<typeof userEvent.setup>) {
  await u.click(screen.getByRole("button", { name: "Criar cliente" }));
}

describe("ClienteModal - CNPJ (NEG-02)", () => {
  it("input nao e numerico e mostra placeholder/ajuda alfanumericos", () => {
    montar();
    const input = campoCnpj();
    expect(input).toHaveAttribute("inputmode", "text");
    expect(input).not.toHaveAttribute("type", "number");
    expect(input).toHaveAttribute("placeholder", "Ex: 12.ABC.345/01DE-35");
    expect(
      screen.getByText("Aceita letras e numeros; os 2 ultimos caracteres (DV) sao numericos.")
    ).toBeInTheDocument();
  });

  it("numerico: mascara ao digitar e envia sem mascara", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar();
    await u.type(campoCnpj(), "11222333000181");
    expect(campoCnpj()).toHaveValue("11.222.333/0001-81");
    await preencherResto(u);
    await submeter(u);
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ cnpj: "11222333000181" })
    );
  });

  it("alfanumerico: mascara com letras e envia em maiusculas", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar();
    await u.type(campoCnpj(), "12ABC34501DE35");
    expect(campoCnpj()).toHaveValue("12.ABC.345/01DE-35");
    await preencherResto(u);
    await submeter(u);
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ cnpj: "12ABC34501DE35" })
    );
  });

  it("minusculas viram maiusculas enquanto digita", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar();
    await u.type(campoCnpj(), "12abc");
    expect(campoCnpj()).toHaveValue("12.ABC");
    await u.type(campoCnpj(), "34501de35");
    expect(campoCnpj()).toHaveValue("12.ABC.345/01DE-35");
    await preencherResto(u);
    await submeter(u);
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ cnpj: "12ABC34501DE35" })
    );
  });

  it("colar com mascara (e minusculas) normaliza o valor", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar();
    await u.click(campoCnpj());
    await u.paste("12.abc.345/01de-35");
    expect(campoCnpj()).toHaveValue("12.ABC.345/01DE-35");
    await preencherResto(u);
    await submeter(u);
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ cnpj: "12ABC34501DE35" })
    );
  });

  it("colar numerico com mascara", async () => {
    const u = userEvent.setup();
    montar();
    await u.click(campoCnpj());
    await u.paste("11.222.333/0001-81");
    expect(campoCnpj()).toHaveValue("11.222.333/0001-81");
  });

  it("letra no DV e ignorada ao digitar", async () => {
    const u = userEvent.setup();
    montar();
    await u.type(campoCnpj(), "12ABC34501DEX3Y");
    expect(campoCnpj()).toHaveValue("12.ABC.345/01DE-3");
  });

  it("caracteres fora de [0-9A-Z] sao descartados", async () => {
    const u = userEvent.setup();
    montar();
    await u.type(campoCnpj(), "12#a_b!c");
    expect(campoCnpj()).toHaveValue("12.ABC");
  });

  it("DV errado mostra 'CNPJ invalido.' e nao chama onSubmit", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar();
    await u.type(campoCnpj(), "12ABC34501DE36");
    await preencherResto(u);
    await submeter(u);
    expect(await screen.findByText("CNPJ invalido.")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("CNPJ incompleto (letra no DV descartada) e invalido", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar();
    await u.type(campoCnpj(), "12ABC34501DE3A");
    await preencherResto(u);
    await submeter(u);
    expect(await screen.findByText("CNPJ invalido.")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("todos os caracteres iguais e invalido", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar();
    await u.type(campoCnpj(), "00000000000000");
    await preencherResto(u);
    await submeter(u);
    expect(await screen.findByText("CNPJ invalido.")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("CNPJ vazio mostra 'CNPJ e obrigatorio.'", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar();
    await preencherResto(u);
    const form = campoCnpj().closest("form")!;
    // Dispara o submit direto para passar pelo `required` nativo.
    form.noValidate = true;
    await submeter(u);
    expect(await screen.findByText("CNPJ e obrigatorio.")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("modo editar exibe o CNPJ alfanumerico do backend com mascara", async () => {
    const u = userEvent.setup();
    const cliente: Cliente = {
      cliente_id_origem: 9,
      cnpj: "12ABC34501DE35",
      razao_social: "Loja Alfa",
      segmento: "Varejo",
      cidade: "Santos",
      uf: "SP",
      bairro: "",
      data_cadastro: "2026-01-01T00:00:00Z",
      ativo: true,
      created_at: "",
      updated_at: "",
    };
    const { onSubmit } = montar({ mode: "edit", cliente });
    expect(campoCnpj()).toHaveValue("12.ABC.345/01DE-35");
    await u.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ cnpj: "12ABC34501DE35" })
    );
  });

  it("erro 409 do backend aparece no modal", async () => {
    const u = userEvent.setup();
    const onSubmit = vi.fn().mockRejectedValue(new Error("cnpj já cadastrado"));
    montar({ onSubmit });
    await u.type(campoCnpj(), "12abc34501de35");
    await preencherResto(u);
    await submeter(u);
    expect(await screen.findByText("cnpj já cadastrado")).toBeInTheDocument();
  });
});
