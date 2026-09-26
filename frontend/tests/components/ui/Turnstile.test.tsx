/**
 * Widget Cloudflare Turnstile. A site key e lida uma vez no carregamento do
 * modulo, entao cada teste recarrega o modulo com a env desejada. O script
 * oficial (next/script) e substituido por um fake que expoe o `onReady`, e o
 * `window.turnstile` e um mock com render/reset/remove.
 */
import { act, render, screen } from "@testing-library/react";
import { StrictMode, createRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const script = vi.hoisted(() => ({
  onReady: undefined as (() => void) | undefined,
  src: undefined as string | undefined,
}));
vi.mock("next/script", () => ({
  default: (props: { src: string; onReady?: () => void }) => {
    script.onReady = props.onReady;
    script.src = props.src;
    return <span data-testid="turnstile-script" />;
  },
}));

type TurnstileModule = typeof import("@/components/ui/Turnstile");
type Opcoes = Parameters<NonNullable<Window["turnstile"]>["render"]>[1];

async function carregar(siteKey: string): Promise<TurnstileModule> {
  vi.stubEnv("NEXT_PUBLIC_TURNSTILE_SITE_KEY", siteKey);
  vi.resetModules();
  return import("@/components/ui/Turnstile");
}

function instalarTurnstile() {
  let seq = 0;
  const api = {
    render: vi.fn<(el: HTMLElement, o: Opcoes) => string>(() => `w-${++seq}`),
    reset: vi.fn<(id?: string) => void>(),
    remove: vi.fn<(id?: string) => void>(),
  };
  window.turnstile = api;
  return api;
}

function callbacks() {
  return { onVerify: vi.fn(), onExpire: vi.fn(), onError: vi.fn() };
}

beforeEach(() => {
  script.onReady = undefined;
  script.src = undefined;
});

afterEach(() => {
  delete window.turnstile;
  vi.unstubAllEnvs();
});

describe("Turnstile sem site key", () => {
  it("fica desabilitado, avisa e nao carrega o script", async () => {
    const { Turnstile, isTurnstileEnabled } = await carregar("");
    expect(isTurnstileEnabled).toBe(false);
    const ref = createRef<{ reset: () => void }>();
    render(<Turnstile ref={ref} {...callbacks()} />);
    expect(screen.getByText(/CAPTCHA nao configurado/)).toBeInTheDocument();
    expect(screen.queryByTestId("turnstile-script")).not.toBeInTheDocument();
    // reset sem widget e no-op (nao lanca).
    expect(() => ref.current!.reset()).not.toThrow();
  });

  it("mesmo com window.turnstile presente, nao renderiza widget", async () => {
    const api = instalarTurnstile();
    const { Turnstile } = await carregar("");
    render(<Turnstile {...callbacks()} />);
    expect(api.render).not.toHaveBeenCalled();
  });
});

describe("Turnstile com site key", () => {
  it("isTurnstileEnabled reflete a configuracao", async () => {
    const { isTurnstileEnabled } = await carregar("site-key-123");
    expect(isTurnstileEnabled).toBe(true);
  });

  it("carrega o script oficial e so renderiza o widget quando o script fica pronto", async () => {
    const { Turnstile } = await carregar("site-key-123");
    const cb = callbacks();
    const { container } = render(<Turnstile {...cb} />);
    expect(screen.getByTestId("turnstile-script")).toBeInTheDocument();
    expect(script.src).toBe("https://challenges.cloudflare.com/turnstile/v0/api.js");

    const api = instalarTurnstile();
    expect(api.render).not.toHaveBeenCalled();
    act(() => script.onReady!());

    expect(api.render).toHaveBeenCalledTimes(1);
    const [alvo, opcoes] = api.render.mock.calls[0];
    expect(container.contains(alvo)).toBe(true);
    expect(opcoes).toMatchObject({ sitekey: "site-key-123", theme: "light" });
  });

  it("onReady antes de window.turnstile existir nao quebra nem renderiza", async () => {
    const { Turnstile } = await carregar("site-key-123");
    render(<Turnstile {...callbacks()} />);
    expect(() => act(() => script.onReady!())).not.toThrow();
    expect(screen.getByTestId("turnstile-script")).toBeInTheDocument();
  });

  it("script ja carregado por outra tela: renderiza de imediato", async () => {
    const api = instalarTurnstile();
    const { Turnstile } = await carregar("site-key-123");
    render(<Turnstile {...callbacks()} />);
    expect(api.render).toHaveBeenCalledTimes(1);
  });

  it.each<[string, keyof Opcoes, "onVerify" | "onExpire" | "onError", unknown[]]>([
    ["token verificado", "callback", "onVerify", ["tok-1"]],
    ["token expirado", "expired-callback", "onExpire", []],
    ["erro do widget", "error-callback", "onError", []],
  ])("%s repassa para o callback da tela", async (_n, opcao, prop, args) => {
    const api = instalarTurnstile();
    const { Turnstile } = await carregar("site-key-123");
    const cb = callbacks();
    render(<Turnstile {...cb} />);
    const opcoes = api.render.mock.calls[0][1];
    (opcoes[opcao] as (...a: unknown[]) => void)(...args);
    expect(cb[prop]).toHaveBeenCalledWith(...args);
  });

  it("reset() via ref reseta o widget atual", async () => {
    const api = instalarTurnstile();
    const { Turnstile } = await carregar("site-key-123");
    const ref = createRef<{ reset: () => void }>();
    render(<Turnstile ref={ref} {...callbacks()} />);
    ref.current!.reset();
    expect(api.reset).toHaveBeenCalledWith("w-1");
  });

  it("reset() antes do widget existir e no-op", async () => {
    const { Turnstile } = await carregar("site-key-123");
    const ref = createRef<{ reset: () => void }>();
    render(<Turnstile ref={ref} {...callbacks()} />);
    const api = instalarTurnstile();
    ref.current!.reset();
    expect(api.reset).not.toHaveBeenCalled();
  });

  it("desmontar remove o widget do Turnstile (sem widget orfao)", async () => {
    const api = instalarTurnstile();
    const { Turnstile } = await carregar("site-key-123");
    const { unmount } = render(<Turnstile {...callbacks()} />);
    unmount();
    expect(api.remove).toHaveBeenCalledWith("w-1");
  });

  it("desmontar depois que o script sumiu nao lanca", async () => {
    instalarTurnstile();
    const { Turnstile } = await carregar("site-key-123");
    const { unmount } = render(<Turnstile {...callbacks()} />);
    delete window.turnstile;
    expect(() => unmount()).not.toThrow();
  });

  it("StrictMode (efeito duplo) remove a instancia anterior e o reset usa a nova", async () => {
    const api = instalarTurnstile();
    const { Turnstile } = await carregar("site-key-123");
    const ref = createRef<{ reset: () => void }>();
    render(
      <StrictMode>
        <Turnstile ref={ref} {...callbacks()} />
      </StrictMode>
    );
    expect(api.render).toHaveBeenCalledTimes(2);
    expect(api.remove).toHaveBeenCalledWith("w-1");
    ref.current!.reset();
    expect(api.reset).toHaveBeenCalledWith("w-2");
  });
});
