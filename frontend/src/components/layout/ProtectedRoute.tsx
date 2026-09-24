"use client";

import { ReactNode, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getUser, clearUser } from "@/lib/auth";
import { clearSession, getSessionUser, refreshSessionUser } from "@/lib/session";

interface ProtectedRouteProps {
  children: ReactNode;
  requireAdmin?: boolean;
}

export function ProtectedRoute({ children, requireAdmin = false }: ProtectedRouteProps) {
  const router = useRouter();
  // Se a sessao em memoria ja foi validada por /me (navegacao client-side
  // entre telas), renderiza de imediato e revalida em segundo plano.
  const [authorized, setAuthorized] = useState(() => {
    const u = getSessionUser();
    return !!u && (!requireAdmin || u.role === "admin");
  });

  useEffect(() => {
    let cancelled = false;

    async function validate(background: boolean) {
      const cachedUser = getUser();

      // Sem usuario em cache: nunca logou neste navegador (ou logout ja
      // limpou o cache) - manda direto pro login sem round-trip.
      if (!cachedUser && !getSessionUser()) {
        router.replace("/login");
        return;
      }

      // SEGURANCA: o cache em localStorage nao e HttpOnly e pode ser
      // adulterado via DevTools (ex.: trocar role para "admin"). Por isso
      // a decisao de autorizacao da UI nunca confia nele - revalidamos a
      // sessao/role direto no backend (GET /api/auth/me, que exige o cookie
      // HttpOnly de access_token). O resultado fica na sessao em memoria
      // (session.ts), fonte da verdade para id_vendedor e vendedor_desligado.
      try {
        const freshUser = await refreshSessionUser();
        if (cancelled) return;

        if (requireAdmin && freshUser.role !== "admin") {
          // Nao redirecionar para /dashboard (loop historico com
          // requireAdmin); vai para uma rota acessivel a qualquer usuario.
          setAuthorized(false);
          router.replace("/pagamentos");
          return;
        }

        setAuthorized(true);
      } catch {
        if (cancelled) return;
        // Revalidacao em segundo plano (foco da aba): uma falha de rede nao
        // derruba a sessao. Se foi 401 sem refresh possivel, o apiClient ja
        // redireciona para o login.
        if (background) return;
        // /api/auth/me falhou na montagem - sessao invalida: nao ha base
        // segura para decidir o que renderizar.
        clearUser();
        clearSession();
        router.replace("/login");
      }
    }

    validate(false);

    // Revalida quando a aba volta a ficar visivel (ex.: admin mudou o
    // vinculo usuario -> vendedor enquanto o usuario estava em outra aba).
    const onVisible = () => {
      if (document.visibilityState === "visible") validate(true);
    };
    const onFocus = () => validate(true);
    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("focus", onFocus);

    return () => {
      cancelled = true;
      document.removeEventListener("visibilitychange", onVisible);
      window.removeEventListener("focus", onFocus);
    };
  }, [router, requireAdmin]);

  if (!authorized) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-50">
        <div className="flex flex-col items-center gap-3">
          <div className="h-10 w-10 animate-spin rounded-full border-4 border-primary-200 border-t-primary-600" />
          <p className="text-sm text-slate-500">Verificando autenticacao...</p>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
