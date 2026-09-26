/**
 * Tela de login: redirecionamento de quem ja esta logado, validacao local,
 * destino apos login (troca de senha obrigatoria / admin / usuario padrao),
 * tratamento de erro (mensagem generica para falha de captcha) e integracao
 * com o widget Turnstile (token obrigatorio quando configurado, reset apos
 * falha).
 */
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { LoginResponse, User } from "@/lib/types";

const replaceMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: replaceMock }) }));

const apiLoginMock = vi.fn<(email: string, senha: string, token?: string) => Promise<LoginResponse>>();
vi.mock("@/lib/api", () => ({
  apiLogin: (email: string, senha: string, token?: string) => apiLoginMock(email, senha, token),
}));

// Turnstile falso: botoes simulam os callbacks do widget real e o `reset`
// imperativo fica observavel. `isTurnstileEnabled` e um getter para cada
// teste escolher se o captcha esta configurado.
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

import LoginPage from "@/app/login/page";

function usuario(role: User["role"] = "normal"): User {
  return { id: 1, nome: "Ana", email: "ana@x.com", role, ativo: true, id_vendedor: null };
}

function deferred<T>() {
  let resolve: (v: T) => void = () => {};
  let reject: (e: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const emailInput = () => screen.getByLabelText("Email");
const senhaInput = () => screen.getByLabelText("Senha");
const entrar = () => screen.getByRole("button", { name: "Entrar" });

async function preencherEEnviar(email = "ana@x.com", senha = "segredo") {
  if (email) await userEvent.type(emailInput(), email);
  if (senha) await userEvent.type(senhaInput(), senha);
  await userEvent.click(entrar());
}

beforeEach(() => {
  replaceMock.mockReset();
  apiLoginMock.mockReset();
  captcha.reset.mockReset();
  captcha.enabled = false;
});

describe("LoginPage - sessao existente", () => {
  it("sem usuario salvo permanece no login", () => {
    render(<LoginPage />);
    expect(screen.getByText("Entrar na sua conta")).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it.each<[User["role"], string]>([
    ["admin", "/dashboard"],
    ["normal", "/pagamentos"],
  ])("usuario %s ja logado vai para %s", async (role, destino) => {
    localStorage.setItem("auth_user", JSON.stringify(usuario(role)));
    render(<LoginPage />);
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith(destino));
  });
});

describe("LoginPage - validacao local", () => {
  it.each<[string, string, string, string[]]>([
    ["tudo vazio", "", "", ["Email e obrigatorio", "Senha e obrigatoria"]],
    ["email invalido", "ana-sem-arroba", "segredo", ["Email invalido"]],
    ["email sem dominio", "ana@x", "segredo", ["Email invalido"]],
    ["senha vazia", "ana@x.com", "", ["Senha e obrigatoria"]],
  ])("%s: mostra os erros e nao chama a API", async (_n, email, senha, erros) => {
    render(<LoginPage />);
    await preencherEEnviar(email, senha);
    for (const msg of erros) expect(screen.getByText(msg)).toBeInTheDocument();
    expect(apiLoginMock).not.toHaveBeenCalled();
  });

  it("corrigir o campo e reenviar limpa o erro", async () => {
    apiLoginMock.mockResolvedValue({ token: "t", user: usuario() });
    render(<LoginPage />);
    await preencherEEnviar("", "segredo");
    expect(screen.getByText("Email e obrigatorio")).toBeInTheDocument();
    await userEvent.type(emailInput(), "ana@x.com");
    await userEvent.click(entrar());
    await waitFor(() => expect(apiLoginMock).toHaveBeenCalled());
    expect(screen.queryByText("Email e obrigatorio")).not.toBeInTheDocument();
  });
});

describe("LoginPage - login com sucesso", () => {
  it.each<[string, Partial<LoginResponse>, User["role"], string]>([
    ["troca de senha obrigatoria", { trocar_senha: true }, "admin", "/trocar-senha"],
    ["admin", { trocar_senha: false }, "admin", "/dashboard"],
    ["usuario padrao", {}, "normal", "/pagamentos"],
  ])("%s -> %s", async (_n, extra, role, destino) => {
    apiLoginMock.mockResolvedValue({ token: "t", user: usuario(role), ...extra });
    render(<LoginPage />);
    await preencherEEnviar();
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith(destino));
    // Sem captcha configurado, nao envia token.
    expect(apiLoginMock).toHaveBeenCalledWith("ana@x.com", "segredo", undefined);
    // Usuario persistido para a UI (whitelist do saveUser).
    expect(JSON.parse(localStorage.getItem("auth_user")!)).toMatchObject({ id: 1, role });
  });

  it("mostra 'Entrando...' e desabilita o botao enquanto aguarda a API", async () => {
    const d = deferred<LoginResponse>();
    apiLoginMock.mockReturnValue(d.promise);
    render(<LoginPage />);
    await preencherEEnviar();
    const botao = screen.getByRole("button", { name: /Entrando\.\.\./ });
    expect(botao).toBeDisabled();
    await act(async () => d.resolve({ token: "t", user: usuario() }));
    expect(screen.getByRole("button", { name: "Entrar" })).toBeEnabled();
  });
});

describe("LoginPage - falha no login", () => {
  it.each<[string, unknown, string]>([
    ["credenciais invalidas", new Error("Credenciais invalidas"), "Credenciais invalidas"],
    ["falha de captcha", new Error("captcha verification failed"), "Nao foi possivel validar o captcha. Tente novamente."],
    ["falha do Turnstile", new Error("TURNSTILE token expired"), "Nao foi possivel validar o captcha. Tente novamente."],
    ["erro sem mensagem", new Error(""), "Erro ao fazer login"],
    ["rejeicao que nao e Error", "boom", "Erro ao fazer login"],
  ])("%s: mostra a mensagem certa e reseta o captcha", async (_n, erro, msg) => {
    apiLoginMock.mockRejectedValue(erro);
    render(<LoginPage />);
    await preencherEEnviar();
    expect(await screen.findByText(msg)).toBeInTheDocument();
    expect(captcha.reset).toHaveBeenCalledTimes(1);
    expect(replaceMock).not.toHaveBeenCalled();
    expect(localStorage.getItem("auth_user")).toBeNull();
    expect(screen.getByRole("button", { name: "Entrar" })).toBeEnabled();
  });

  it("o alerta de erro pode ser fechado", async () => {
    apiLoginMock.mockRejectedValue(new Error("Credenciais invalidas"));
    render(<LoginPage />);
    await preencherEEnviar();
    await screen.findByText("Credenciais invalidas");
    await userEvent.click(screen.getByRole("button", { name: "Fechar alerta" }));
    expect(screen.queryByText("Credenciais invalidas")).not.toBeInTheDocument();
  });

  it("novo envio limpa o erro anterior", async () => {
    apiLoginMock.mockRejectedValueOnce(new Error("Credenciais invalidas"));
    apiLoginMock.mockResolvedValueOnce({ token: "t", user: usuario() });
    render(<LoginPage />);
    await preencherEEnviar();
    await screen.findByText("Credenciais invalidas");
    await userEvent.click(entrar());
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/pagamentos"));
    expect(screen.queryByText("Credenciais invalidas")).not.toBeInTheDocument();
  });
});

describe("LoginPage - captcha (Turnstile configurado)", () => {
  beforeEach(() => {
    captcha.enabled = true;
  });

  it("Entrar fica desabilitado ate o captcha ser resolvido e envia o token", async () => {
    apiLoginMock.mockResolvedValue({ token: "t", user: usuario("admin") });
    render(<LoginPage />);
    await userEvent.type(emailInput(), "ana@x.com");
    await userEvent.type(senhaInput(), "segredo");
    expect(entrar()).toBeDisabled();

    await userEvent.click(screen.getByRole("button", { name: "captcha-ok" }));
    expect(entrar()).toBeEnabled();
    await userEvent.click(entrar());
    await waitFor(() =>
      expect(apiLoginMock).toHaveBeenCalledWith("ana@x.com", "segredo", "tok-captcha")
    );
  });

  it.each([["captcha-expira"], ["captcha-erro"]])(
    "%s invalida o token e volta a bloquear o envio",
    async (evento) => {
      render(<LoginPage />);
      await userEvent.click(screen.getByRole("button", { name: "captcha-ok" }));
      expect(entrar()).toBeEnabled();
      await userEvent.click(screen.getByRole("button", { name: evento }));
      expect(entrar()).toBeDisabled();
    }
  );

  it("apos falha, o token e descartado (uso unico) e o envio volta a exigir captcha", async () => {
    apiLoginMock.mockRejectedValue(new Error("Credenciais invalidas"));
    render(<LoginPage />);
    await userEvent.type(emailInput(), "ana@x.com");
    await userEvent.type(senhaInput(), "segredo");
    await userEvent.click(screen.getByRole("button", { name: "captcha-ok" }));
    await userEvent.click(entrar());
    await screen.findByText("Credenciais invalidas");
    expect(captcha.reset).toHaveBeenCalledTimes(1);
    expect(entrar()).toBeDisabled();
  });
});
