import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { MeResponse } from "@/lib/types";

const replaceMock = vi.fn();
const router = { replace: replaceMock };
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const refreshSessionUserMock = vi.fn<() => Promise<MeResponse>>();
const getSessionUserMock = vi.fn<() => MeResponse | null>();
const clearSessionMock = vi.fn();
vi.mock("@/lib/session", () => ({
  refreshSessionUser: () => refreshSessionUserMock(),
  getSessionUser: () => getSessionUserMock(),
  clearSession: () => clearSessionMock(),
}));

import { ProtectedRoute } from "./ProtectedRoute";

function me(role: MeResponse["role"] = "normal"): MeResponse {
  return { id: 1, nome: "Ana", email: "a@x", role, ativo: true, id_vendedor: 7 };
}

function seedCache(role: MeResponse["role"] = "normal") {
  localStorage.setItem("auth_user", JSON.stringify(me(role)));
}

beforeEach(() => {
  replaceMock.mockReset();
  refreshSessionUserMock.mockReset();
  getSessionUserMock.mockReset().mockReturnValue(null);
  clearSessionMock.mockReset();
});

describe("ProtectedRoute", () => {
  it("sem cache nem sessao: vai para /login sem chamar /me", async () => {
    render(<ProtectedRoute>conteudo</ProtectedRoute>);
    await vi.waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/login"));
    expect(refreshSessionUserMock).not.toHaveBeenCalled();
    expect(screen.queryByText("conteudo")).not.toBeInTheDocument();
  });

  it("valida em /me no mount e libera o conteudo", async () => {
    seedCache();
    refreshSessionUserMock.mockResolvedValue(me());
    render(<ProtectedRoute>conteudo</ProtectedRoute>);
    expect(screen.getByText("Verificando autenticacao...")).toBeInTheDocument();
    expect(await screen.findByText("conteudo")).toBeInTheDocument();
    expect(refreshSessionUserMock).toHaveBeenCalledTimes(1);
  });

  it("sessao em memoria ja validada renderiza de imediato", () => {
    getSessionUserMock.mockReturnValue(me());
    refreshSessionUserMock.mockReturnValue(new Promise(() => {}));
    render(<ProtectedRoute>conteudo</ProtectedRoute>);
    expect(screen.getByText("conteudo")).toBeInTheDocument();
  });

  it("falha de /me no mount: limpa cache/sessao e vai para /login", async () => {
    seedCache();
    refreshSessionUserMock.mockRejectedValue(new Error("401"));
    render(<ProtectedRoute>conteudo</ProtectedRoute>);
    await vi.waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/login"));
    expect(localStorage.getItem("auth_user")).toBeNull();
    expect(clearSessionMock).toHaveBeenCalled();
  });

  it("requireAdmin com usuario normal redireciona para /pagamentos", async () => {
    seedCache("normal");
    refreshSessionUserMock.mockResolvedValue(me("normal"));
    render(<ProtectedRoute requireAdmin>conteudo</ProtectedRoute>);
    await vi.waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/pagamentos"));
    expect(screen.queryByText("conteudo")).not.toBeInTheDocument();
  });

  it.each<[string, () => void]>([
    ["focus da janela", () => window.dispatchEvent(new Event("focus"))],
    [
      "visibilitychange visivel",
      () => document.dispatchEvent(new Event("visibilitychange")),
    ],
  ])("revalida /me no %s (vinculo alterado pelo admin)", async (_n, dispara) => {
    seedCache();
    refreshSessionUserMock.mockResolvedValue(me());
    render(<ProtectedRoute>conteudo</ProtectedRoute>);
    await screen.findByText("conteudo");
    expect(refreshSessionUserMock).toHaveBeenCalledTimes(1);

    await act(async () => dispara());
    expect(refreshSessionUserMock).toHaveBeenCalledTimes(2);
  });

  it("falha na revalidacao em segundo plano nao derruba a sessao", async () => {
    seedCache();
    refreshSessionUserMock.mockResolvedValueOnce(me()).mockRejectedValueOnce(new Error("rede"));
    render(<ProtectedRoute>conteudo</ProtectedRoute>);
    await screen.findByText("conteudo");

    await act(async () => {
      window.dispatchEvent(new Event("focus"));
    });
    expect(screen.getByText("conteudo")).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
    expect(localStorage.getItem("auth_user")).not.toBeNull();
  });

  it("remove os listeners ao desmontar", async () => {
    seedCache();
    refreshSessionUserMock.mockResolvedValue(me());
    const { unmount } = render(<ProtectedRoute>conteudo</ProtectedRoute>);
    await screen.findByText("conteudo");
    unmount();
    window.dispatchEvent(new Event("focus"));
    expect(refreshSessionUserMock).toHaveBeenCalledTimes(1);
  });
});
