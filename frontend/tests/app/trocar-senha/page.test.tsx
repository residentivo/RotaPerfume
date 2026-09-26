/**
 * Tela de troca de senha obrigatoria: exige sessao, valida a politica de
 * senha (tamanho, 3 de 4 classes, diferente da atual, confirmacao), redireciona
 * para o dashboard apos sucesso e trata erros (incluindo captcha).
 */
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const replaceMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: replaceMock }) }));

const apiChangePasswordMock = vi.fn<(a: string, n: string, t?: string) => Promise<{ message: string }>>();
vi.mock("@/lib/api", () => ({
  apiChangePassword: (a: string, n: string, t?: string) => apiChangePasswordMock(a, n, t),
}));

const captcha = vi.hoisted(() => ({ enabled: false, reset: vi.fn() }));
vi.mock("@/components/ui/Turnstile", async () => {
  const React = await import("react");
  type Props = { onVerify: (t: string) => void; onExpire?: () => void; onError?: () => void };
  const Turnstile = React.forwardRef<{ reset: () => void }, Props>(function FakeTurnstile(
    { onVerify, onExpire, onError },
    ref
  ) {
    React.useImperativeHandle(ref, () => ({ reset: captcha.reset }));
    return (
      <div>
        <button type="button" onClick={() => onVerify("tok-captcha")}>
          captcha-ok
        </button>
        <button type="button" onClick={() => onExpire?.()}>
          captcha-expira
        </button>
        <button type="button" onClick={() => onError?.()}>
          captcha-erro
        </button>
      </div>
    );
  });
  return {
    Turnstile,
    get isTurnstileEnabled() {
      return captcha.enabled;
    },
  };
});

import TrocarSenhaPage from "@/app/trocar-senha/page";

const SENHA_ATUAL = "Antiga@123";
const SENHA_NOVA = "Nova@12345";

function logar() {
  localStorage.setItem(
    "auth_user",
    JSON.stringify({ id: 1, nome: "Ana", email: "a@x", role: "normal", ativo: true })
  );
}

const alterar = () => screen.getByRole("button", { name: "Alterar Senha" });

async function preencher(atual: string, nova: string, confirmar: string) {
  if (atual) await user.type(screen.getByLabelText("Senha Atual"), atual);
  if (nova) await user.type(screen.getByLabelText("Nova Senha"), nova);
  if (confirmar) await user.type(screen.getByLabelText("Confirmar Nova Senha"), confirmar);
}

async function enviarValido() {
  await preencher(SENHA_ATUAL, SENHA_NOVA, SENHA_NOVA);
  await user.click(alterar());
}

let user: ReturnType<typeof userEvent.setup>;

beforeEach(() => {
  // O redirect pos-sucesso usa setTimeout(1500) que a tela nao cancela no
  // unmount; com fake timers + clearAllTimers ele nao vaza para o proximo teste.
  vi.useFakeTimers({ shouldAdvanceTime: true });
  user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  replaceMock.mockReset();
  apiChangePasswordMock.mockReset();
  captcha.reset.mockReset();
  captcha.enabled = false;
  logar();
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe("TrocarSenhaPage - sessao", () => {
  it("sem usuario logado redireciona para /login", async () => {
    localStorage.clear();
    render(<TrocarSenhaPage />);
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/login"));
  });

  it("com usuario logado permanece na tela", () => {
    render(<TrocarSenhaPage />);
    expect(screen.getByText("Trocar Senha")).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });
});

describe("TrocarSenhaPage - politica de senha", () => {
  it.each<[string, string, string, string, string[]]>([
    [
      "tudo vazio",
      "",
      "",
      "",
      ["Senha atual e obrigatoria", "Nova senha e obrigatoria", "Confirmacao de senha e obrigatoria"],
    ],
    ["nova igual a atual", SENHA_ATUAL, SENHA_ATUAL, SENHA_ATUAL, ["A nova senha nao pode ser igual a senha atual"]],
    ["menos de 8 caracteres", SENHA_ATUAL, "Ab1!", "Ab1!", ["Nova senha deve ter pelo menos 8 caracteres"]],
    [
      "so minusculas",
      SENHA_ATUAL,
      "abcdefgh",
      "abcdefgh",
      ["Nova senha deve conter ao menos 3 dos 4 tipos: letra minuscula, letra maiuscula, digito e simbolo"],
    ],
    [
      "2 classes (minuscula + digito)",
      SENHA_ATUAL,
      "abcdefg1",
      "abcdefg1",
      ["Nova senha deve conter ao menos 3 dos 4 tipos: letra minuscula, letra maiuscula, digito e simbolo"],
    ],
    ["confirmacao diferente", SENHA_ATUAL, SENHA_NOVA, "Nova@99999", ["As senhas nao conferem"]],
    ["sem senha atual", "", SENHA_NOVA, SENHA_NOVA, ["Senha atual e obrigatoria"]],
  ])("%s: mostra os erros e nao chama a API", async (_n, atual, nova, confirmar, erros) => {
    render(<TrocarSenhaPage />);
    await preencher(atual, nova, confirmar);
    await user.click(alterar());
    for (const msg of erros) expect(screen.getByText(msg)).toBeInTheDocument();
    expect(apiChangePasswordMock).not.toHaveBeenCalled();
  });

  it.each<[string, string]>([
    ["minuscula + maiuscula + digito", "Abcdefg1"],
    ["minuscula + maiuscula + simbolo", "Abcdefg!"],
    ["minuscula + digito + simbolo", "abcdef1!"],
    ["as 4 classes", "Abcde1!x"],
  ])("aceita senha com %s", async (_n, nova) => {
    apiChangePasswordMock.mockResolvedValue({ message: "ok" });
    render(<TrocarSenhaPage />);
    await preencher(SENHA_ATUAL, nova, nova);
    await user.click(alterar());
    await waitFor(() => expect(apiChangePasswordMock).toHaveBeenCalledWith(SENHA_ATUAL, nova, undefined));
  });
});

describe("TrocarSenhaPage - envio", () => {
  it("sucesso mostra a confirmacao e vai para o dashboard apos 1,5s", async () => {
    apiChangePasswordMock.mockResolvedValue({ message: "ok" });
    render(<TrocarSenhaPage />);
    await enviarValido();
    expect(await screen.findByText("Senha alterada com sucesso!")).toBeInTheDocument();
    expect(apiChangePasswordMock).toHaveBeenCalledWith(SENHA_ATUAL, SENHA_NOVA, undefined);

    await act(async () => {
      vi.advanceTimersByTime(1200);
    });
    expect(replaceMock).not.toHaveBeenCalledWith("/dashboard");
    await act(async () => {
      vi.advanceTimersByTime(400);
    });
    expect(replaceMock).toHaveBeenCalledWith("/dashboard");
  });

  it("o alerta de sucesso pode ser fechado", async () => {
    apiChangePasswordMock.mockResolvedValue({ message: "ok" });
    render(<TrocarSenhaPage />);
    await enviarValido();
    await screen.findByText("Senha alterada com sucesso!");
    await user.click(screen.getByRole("button", { name: "Fechar alerta" }));
    expect(screen.queryByText("Senha alterada com sucesso!")).not.toBeInTheDocument();
  });

  it("mostra 'Alterando...' e desabilita o botao enquanto aguarda a API", async () => {
    let resolver: (v: { message: string }) => void = () => {};
    apiChangePasswordMock.mockReturnValue(new Promise((r) => (resolver = r)));
    render(<TrocarSenhaPage />);
    await enviarValido();
    expect(screen.getByRole("button", { name: /Alterando\.\.\./ })).toBeDisabled();
    await act(async () => resolver({ message: "ok" }));
    expect(alterar()).toBeEnabled();
  });

  it.each<[string, unknown, string]>([
    ["senha atual incorreta", new Error("Senha atual incorreta"), "Senha atual incorreta"],
    ["falha de captcha", new Error("invalid captcha"), "Nao foi possivel validar o captcha. Tente novamente."],
    ["falha do Turnstile", new Error("turnstile timeout"), "Nao foi possivel validar o captcha. Tente novamente."],
    ["erro sem mensagem", new Error(""), "Erro ao alterar senha"],
    ["rejeicao que nao e Error", { status: 500 }, "Erro ao alterar senha"],
  ])("%s: mostra a mensagem certa, reseta o captcha e nao redireciona", async (_n, erro, msg) => {
    apiChangePasswordMock.mockRejectedValue(erro);
    render(<TrocarSenhaPage />);
    await enviarValido();
    expect(await screen.findByText(msg)).toBeInTheDocument();
    expect(captcha.reset).toHaveBeenCalledTimes(1);
    expect(screen.queryByText("Senha alterada com sucesso!")).not.toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("o alerta de erro pode ser fechado", async () => {
    apiChangePasswordMock.mockRejectedValue(new Error("Senha atual incorreta"));
    render(<TrocarSenhaPage />);
    await enviarValido();
    await screen.findByText("Senha atual incorreta");
    await user.click(screen.getByRole("button", { name: "Fechar alerta" }));
    expect(screen.queryByText("Senha atual incorreta")).not.toBeInTheDocument();
  });
});

describe("TrocarSenhaPage - captcha (Turnstile configurado)", () => {
  beforeEach(() => {
    captcha.enabled = true;
  });

  it("exige o captcha e envia o token", async () => {
    apiChangePasswordMock.mockResolvedValue({ message: "ok" });
    render(<TrocarSenhaPage />);
    await preencher(SENHA_ATUAL, SENHA_NOVA, SENHA_NOVA);
    expect(alterar()).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "captcha-ok" }));
    await user.click(alterar());
    await waitFor(() =>
      expect(apiChangePasswordMock).toHaveBeenCalledWith(SENHA_ATUAL, SENHA_NOVA, "tok-captcha")
    );
  });

  it.each([["captcha-expira"], ["captcha-erro"]])("%s volta a bloquear o envio", async (evento) => {
    render(<TrocarSenhaPage />);
    await user.click(screen.getByRole("button", { name: "captcha-ok" }));
    expect(alterar()).toBeEnabled();
    await user.click(screen.getByRole("button", { name: evento }));
    expect(alterar()).toBeDisabled();
  });

  it("apos falha o token e descartado e o botao volta a ficar bloqueado", async () => {
    apiChangePasswordMock.mockRejectedValue(new Error("invalid captcha"));
    render(<TrocarSenhaPage />);
    await preencher(SENHA_ATUAL, SENHA_NOVA, SENHA_NOVA);
    await user.click(screen.getByRole("button", { name: "captcha-ok" }));
    await user.click(alterar());
    await screen.findByText("Nao foi possivel validar o captcha. Tente novamente.");
    expect(alterar()).toBeDisabled();
  });
});
