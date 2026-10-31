/**
 * IMG-01: secao de imagem do ProdutoModal (preview, validacao de UX,
 * envio/remocao, erros da API, visibilidade por perfil) e miniatura na
 * tabela de produtos.
 */
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { MeResponse, Produto } from "@/lib/types";
import { ApiError } from "@/lib/apiError";

vi.mock("@/components/layout/ProtectedRoute", () => ({
  ProtectedRoute: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

const sessao = vi.hoisted(() => ({ user: null as MeResponse | null }));
vi.mock("@/lib/session", () => ({
  useSessionUser: () => sessao.user,
  useVendedorDesligado: () => false,
}));

const api = vi.hoisted(() => ({
  enviarImagemProduto: vi.fn(),
  removerImagemProduto: vi.fn(),
  apiListProdutos: vi.fn(),
  apiCreateProduto: vi.fn(),
  apiUpdateProduto: vi.fn(),
  apiToggleProdutoStatus: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import { ProdutoModal } from "@/components/admin/ProdutoModal";
import ProdutosPage from "@/app/admin/produtos/page";

const ADMIN: MeResponse = { id: 1, nome: "Adm", email: "a@x", role: "admin", ativo: true, id_vendedor: null };
const NORMAL: MeResponse = { ...ADMIN, id: 2, role: "normal" };

const IMG = "/api/public/produtos/imagens/0b6c1d2e-1111-4222-8333-944455556666.png";

const produto: Produto = {
  id: 50, sku: "P50", descricao: "Perfume 50", categoria: "C", marca: "M", nota_olfativa: "",
  preco_tabela: 10, custo_unitario: 5, unidade: "UN", data_lancamento: null, ativo: true,
  imagem_url: IMG, created_at: "", updated_at: "",
};

function montar(p: Partial<Produto> = {}, extra: { onImagemAlterada?: (p: Produto) => void } = {}) {
  return render(
    <ProdutoModal
      open
      mode="edit"
      produto={{ ...produto, ...p }}
      onClose={vi.fn()}
      onSubmit={vi.fn().mockResolvedValue(undefined)}
      {...extra}
    />
  );
}

function arquivo(nome = "a.png", tipo = "image/png", bytes = 10): File {
  const f = new File(["x"], nome, { type: tipo });
  Object.defineProperty(f, "size", { value: bytes });
  return f;
}

let criarUrl: ReturnType<typeof vi.fn>;
let revogarUrl: ReturnType<typeof vi.fn>;

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  sessao.user = ADMIN;
  criarUrl = vi.fn(() => "blob:preview-1");
  revogarUrl = vi.fn();
  Object.assign(URL, { createObjectURL: criarUrl, revokeObjectURL: revogarUrl });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("ProdutoModal - secao de imagem", () => {
  it("mostra a imagem atual com a base da API", () => {
    montar();
    const img = screen.getByAltText("Perfume 50") as HTMLImageElement;
    expect(img.src).toMatch(new RegExp(`${IMG}$`));
    expect(img).toHaveAttribute("loading", "lazy");
    expect(screen.getByRole("button", { name: "Trocar imagem" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Remover imagem" })).toBeInTheDocument();
  });

  it.each([
    ["nula", null],
    ["fora da rota publica", "https://evil.example/x.png"],
    ["relativa fora da rota publica", "/api/produtos/50"],
  ])("placeholder quando imagem_url e %s", (_n, url) => {
    montar({ imagem_url: url });
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Produto sem imagem")).toBeInTheDocument();
  });

  it("sem imagem: botao Enviar e sem Remover", () => {
    montar({ imagem_url: null });
    expect(screen.getByRole("button", { name: "Enviar imagem" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Remover imagem" })).not.toBeInTheDocument();
  });

  it("nao aparece na criacao", () => {
    render(
      <ProdutoModal open mode="create" produto={null} onClose={vi.fn()} onSubmit={vi.fn()} />
    );
    expect(screen.queryByText("Imagem")).not.toBeInTheDocument();
  });

  it("nao-admin ve a imagem, mas sem seletor nem botoes", () => {
    sessao.user = NORMAL;
    montar();
    expect(screen.getByAltText("Perfume 50")).toBeInTheDocument();
    expect(screen.queryByLabelText(/Arquivo/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /imagem/ })).not.toBeInTheDocument();
  });

  it("input aceita so JPEG, PNG e WebP", () => {
    montar();
    expect(screen.getByLabelText(/Arquivo/)).toHaveAttribute("accept", "image/jpeg,image/png,image/webp");
  });

  it("recusa no cliente arquivo acima de 1 MB", async () => {
    montar();
    await userEvent.upload(screen.getByLabelText(/Arquivo/), arquivo("g.png", "image/png", 1_048_577));
    expect(screen.getByText("imagem excede 1 MB")).toBeInTheDocument();
    expect(criarUrl).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Trocar imagem" })).toBeDisabled();
  });

  it("preview, envio e liberacao do object URL", async () => {
    const novo = { ...produto, imagem_url: "/api/public/produtos/imagens/novo.png" };
    api.enviarImagemProduto.mockResolvedValue(novo);
    const onImagemAlterada = vi.fn();
    montar({}, { onImagemAlterada });
    const f = arquivo();
    await userEvent.upload(screen.getByLabelText(/Arquivo/), f);
    expect((screen.getByAltText("Pre-visualizacao: Perfume 50") as HTMLImageElement).src).toBe("blob:preview-1");

    await userEvent.click(screen.getByRole("button", { name: "Trocar imagem" }));
    expect(await screen.findByText("Imagem enviada com sucesso.")).toBeInTheDocument();
    expect(api.enviarImagemProduto).toHaveBeenCalledWith(50, f);
    expect(onImagemAlterada).toHaveBeenCalledWith(novo);
    expect(revogarUrl).toHaveBeenCalledWith("blob:preview-1");
    expect((screen.getByAltText("Perfume 50") as HTMLImageElement).src).toMatch(/\/imagens\/novo\.png$/);
  });

  it("revoga o preview no unmount", async () => {
    const r = montar();
    await userEvent.upload(screen.getByLabelText(/Arquivo/), arquivo());
    r.unmount();
    expect(revogarUrl).toHaveBeenCalledWith("blob:preview-1");
  });

  it.each([
    [400, "arquivo de imagem inválido (use JPEG, PNG ou WebP)"],
    [413, "imagem excede 1 MB"],
    [404, "produto não encontrado"],
  ])("exibe o erro %i da API", async (status, msg) => {
    api.enviarImagemProduto.mockRejectedValue(new ApiError(status, msg));
    montar();
    await userEvent.upload(screen.getByLabelText(/Arquivo/), arquivo());
    await userEvent.click(screen.getByRole("button", { name: "Trocar imagem" }));
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });

  it("remove a imagem apos confirmar", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api.removerImagemProduto.mockResolvedValue({ ...produto, imagem_url: null });
    const onImagemAlterada = vi.fn();
    montar({}, { onImagemAlterada });
    await userEvent.click(screen.getByRole("button", { name: "Remover imagem" }));
    expect(await screen.findByText("Imagem removida com sucesso.")).toBeInTheDocument();
    expect(api.removerImagemProduto).toHaveBeenCalledWith(50);
    expect(onImagemAlterada).toHaveBeenCalledWith({ ...produto, imagem_url: null });
    expect(screen.getByLabelText("Produto sem imagem")).toBeInTheDocument();
  });

  it("nao remove sem confirmar", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    montar();
    await userEvent.click(screen.getByRole("button", { name: "Remover imagem" }));
    expect(api.removerImagemProduto).not.toHaveBeenCalled();
  });
});

describe("Produtos - miniatura na tabela", () => {
  it("renderiza miniatura so para imagem_url da rota publica", async () => {
    api.apiListProdutos.mockResolvedValue({
      data: [produto, { ...produto, id: 51, descricao: "Sem foto", imagem_url: null }],
      page: 1, limit: 20, total: 2, pages: 1,
    });
    render(<ProdutosPage />);
    const img = (await screen.findByAltText("50 - Perfume 50")) as HTMLImageElement;
    expect(img.src).toMatch(new RegExp(`${IMG}$`));
    expect(img).toHaveAttribute("loading", "lazy");
    expect(img).toHaveAttribute("decoding", "async");
    expect(img.className).toMatch(/object-cover/);
    expect(screen.queryByAltText("51 - Sem foto")).not.toBeInTheDocument();
    expect(screen.getAllByLabelText("Sem imagem")).toHaveLength(1);
  });

  it("upload no modal atualiza a miniatura da linha", async () => {
    api.apiListProdutos.mockResolvedValue({ data: [{ ...produto, imagem_url: null }], page: 1, limit: 20, total: 1, pages: 1 });
    api.enviarImagemProduto.mockResolvedValue(produto);
    render(<ProdutosPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Perfume 50" }));
    const d = await screen.findByRole("dialog");
    await userEvent.upload(within(d).getByLabelText(/Arquivo/), arquivo());
    await userEvent.click(within(d).getByRole("button", { name: "Enviar imagem" }));
    await waitFor(() => expect(screen.getByAltText("50 - Perfume 50")).toBeInTheDocument());
  });
});
