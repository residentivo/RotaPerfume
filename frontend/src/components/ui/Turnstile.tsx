"use client";

// Widget Cloudflare Turnstile (CAPTCHA).
//
// Escolha de implementação: carregamos o script oficial
// `https://challenges.cloudflare.com/turnstile/v0/api.js` via `next/script`
// (em vez de adicionar uma lib React de terceiros) para não introduzir uma
// nova dependência de pacote no projeto. O componente expõe uma API mínima
// (onVerify/onExpire/onError) e um método imperativo `reset()` via ref, que
// deve ser chamado após falha de login/reset — o token do Turnstile é de uso
// único e não pode ser reenviado.
//
// A site key é pública (não é segredo) e vem de NEXT_PUBLIC_TURNSTILE_SITE_KEY.
// A secret key NUNCA deve aparecer no frontend — a validação real do token
// acontece só no backend.

import Script from "next/script";
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";

declare global {
  interface Window {
    turnstile?: {
      render: (
        container: HTMLElement,
        options: {
          sitekey: string;
          callback?: (token: string) => void;
          "expired-callback"?: () => void;
          "error-callback"?: () => void;
          theme?: "light" | "dark" | "auto";
        }
      ) => string;
      reset: (widgetId?: string) => void;
      remove: (widgetId?: string) => void;
    };
  }
}

export interface TurnstileHandle {
  /** Reseta o widget, invalidando o token atual e exigindo novo desafio. */
  reset: () => void;
}

interface TurnstileProps {
  onVerify: (token: string) => void;
  onExpire?: () => void;
  onError?: () => void;
}

const SITE_KEY = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY || "";

// Indica se o Turnstile está configurado (site key presente). Útil para as
// telas decidirem se devem exigir um token antes de habilitar o submit —
// em ambientes sem a env var (ex: dev local sem chave), não bloqueamos o
// fluxo apenas por falta de configuração do widget.
export const isTurnstileEnabled = Boolean(SITE_KEY);

export const Turnstile = forwardRef<TurnstileHandle, TurnstileProps>(
  function Turnstile({ onVerify, onExpire, onError }, ref) {
    const containerRef = useRef<HTMLDivElement>(null);
    const widgetIdRef = useRef<string | undefined>(undefined);
    const [scriptLoaded, setScriptLoaded] = useState(false);

    const renderWidget = () => {
      if (!window.turnstile || !containerRef.current || !SITE_KEY) return;
      // Evita renderizar duplicado se o efeito rodar mais de uma vez.
      containerRef.current.innerHTML = "";
      widgetIdRef.current = window.turnstile.render(containerRef.current, {
        sitekey: SITE_KEY,
        callback: onVerify,
        "expired-callback": onExpire,
        "error-callback": onError,
        theme: "light",
      });
    };

    useEffect(() => {
      if (scriptLoaded) {
        renderWidget();
      }
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [scriptLoaded]);

    useImperativeHandle(ref, () => ({
      reset: () => {
        if (window.turnstile && widgetIdRef.current) {
          window.turnstile.reset(widgetIdRef.current);
        }
      },
    }));

    if (!SITE_KEY) {
      // Sem site key configurada: não bloqueia o dev local, mas avisa.
      return (
        <p className="text-xs text-amber-600">
          CAPTCHA nao configurado (defina NEXT_PUBLIC_TURNSTILE_SITE_KEY).
        </p>
      );
    }

    return (
      <>
        <Script
          src="https://challenges.cloudflare.com/turnstile/v0/api.js"
          strategy="afterInteractive"
          onLoad={() => setScriptLoaded(true)}
        />
        <div ref={containerRef} />
      </>
    );
  }
);
