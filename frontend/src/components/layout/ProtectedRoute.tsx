"use client";

import { ReactNode, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getUser, clearUser } from "@/lib/auth";
import { clearSession, getSessionUser, refreshSessionUser } from "@/lib/session";
import { vlog } from "@/lib/vlog";

const F = "ProtectedRoute.tsx";

interface ProtectedRouteProps {
  children: ReactNode;
  requireAdmin?: boolean;
}

export function ProtectedRoute({ children, requireAdmin = false }: ProtectedRouteProps) {
  vlog(F, "ProtectedRoute", "obtendo router");
  const router = useRouter();
  // Se a sessao em memoria ja foi validada por /me (navegacao client-side
  // entre telas), renderiza de imediato e revalida em segundo plano.
  vlog(F, "ProtectedRoute", "criando estado de autorização, requireAdmin:", requireAdmin);
  const [authorized, setAuthorized] = useState(() => {
    vlog(F, "ProtectedRoute.func", "lendo usuário da sessão em memória");
    const u = getSessionUser();
    return !!u && (!requireAdmin || u.role === "admin");
  });

  vlog(F, "ProtectedRoute", "registrando efeito de validação da sessão");
  useEffect(() => {
    vlog(F, "ProtectedRoute.useEffect", "inicializando flag de cancelamento");
    let cancelled = false;

    async function validate(background: boolean) {
      vlog(F, "ProtectedRoute.validate", "lendo usuário em cache, background:", background);
      const cachedUser = getUser();

      // Sem usuario em cache: nunca logou neste navegador (ou logout ja
      // limpou o cache) - manda direto pro login sem round-trip.
      vlog(F, "ProtectedRoute.validate", "verificando se há usuário em cache ou sessão:", !!cachedUser);
      if (!cachedUser && !getSessionUser()) {
        vlog(F, "ProtectedRoute.validate", "sem usuário: redirecionando para /login");
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
        vlog(F, "ProtectedRoute.validate", "revalidando sessão no backend (/me)");
        const freshUser = await refreshSessionUser();
        vlog(F, "ProtectedRoute.validate", "verificando se o efeito foi cancelado:", cancelled);
        if (cancelled) return;

        vlog(F, "ProtectedRoute.validate", "verificando permissão de admin, id/papel:", freshUser.id, freshUser.role);
        if (requireAdmin && freshUser.role !== "admin") {
          // Nao redirecionar para /dashboard (loop historico com
          // requireAdmin); vai para uma rota acessivel a qualquer usuario.
          vlog(F, "ProtectedRoute.validate", "não é admin: negando autorização");
          setAuthorized(false);
          vlog(F, "ProtectedRoute.validate", "redirecionando para /pagamentos");
          router.replace("/pagamentos");
          return;
        }

        vlog(F, "ProtectedRoute.validate", "autorizando renderização");
        setAuthorized(true);
      } catch {
        vlog(F, "ProtectedRoute.validate", "falha ao revalidar: verificando se o efeito foi cancelado:", cancelled);
        if (cancelled) return;
        // Revalidacao em segundo plano (foco da aba): uma falha de rede nao
        // derruba a sessao. Se foi 401 sem refresh possivel, o apiClient ja
        // redireciona para o login.
        vlog(F, "ProtectedRoute.validate", "verificando se é revalidação em segundo plano:", background);
        if (background) return;
        // /api/auth/me falhou na montagem - sessao invalida: nao ha base
        // segura para decidir o que renderizar.
        vlog(F, "ProtectedRoute.validate", "limpando usuário em cache");
        clearUser();
        vlog(F, "ProtectedRoute.validate", "limpando sessão em memória");
        clearSession();
        vlog(F, "ProtectedRoute.validate", "redirecionando para /login");
        router.replace("/login");
      }
    }

    vlog(F, "ProtectedRoute.useEffect", "validando sessão na montagem");
    validate(false);

    // Revalida quando a aba volta a ficar visivel (ex.: admin mudou o
    // vinculo usuario -> vendedor enquanto o usuario estava em outra aba).
    const onVisible = () => {
      vlog(F, "ProtectedRoute.onVisible", "verificando se a aba ficou visível");
      if (document.visibilityState === "visible") validate(true);
    };
    const onFocus = () => validate(true);
    vlog(F, "ProtectedRoute.useEffect", "registrando listener de visibilitychange");
    document.addEventListener("visibilitychange", onVisible);
    vlog(F, "ProtectedRoute.useEffect", "registrando listener de focus");
    window.addEventListener("focus", onFocus);

    return () => {
      vlog(F, "ProtectedRoute.useEffect", "cleanup: marcando efeito como cancelado");
      cancelled = true;
      vlog(F, "ProtectedRoute.useEffect", "cleanup: removendo listener de visibilitychange");
      document.removeEventListener("visibilitychange", onVisible);
      vlog(F, "ProtectedRoute.useEffect", "cleanup: removendo listener de focus");
      window.removeEventListener("focus", onFocus);
    };
  }, [router, requireAdmin]);

  vlog(F, "ProtectedRoute", "verificando se está autorizado:", authorized);
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
