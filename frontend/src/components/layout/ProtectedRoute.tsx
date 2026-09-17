"use client";

import { ReactNode, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getUser, saveUser, clearUser } from "@/lib/auth";
import { apiMe } from "@/lib/api";

interface ProtectedRouteProps {
  children: ReactNode;
  requireAdmin?: boolean;
}

export function ProtectedRoute({ children, requireAdmin = false }: ProtectedRouteProps) {
  const router = useRouter();
  const [ready, setReady] = useState(false);
  const [authorized, setAuthorized] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function validate() {
      const cachedUser = getUser();

      // Sem usuario em cache: nunca logou neste navegador (ou logout ja
      // limpou o cache) - manda direto pro login sem round-trip.
      if (!cachedUser) {
        router.replace("/login");
        return;
      }

      // SEGURANCA: o cache em localStorage nao e HttpOnly e pode ser
      // adulterado via DevTools (ex.: trocar role para "admin"). Por isso
      // a decisao de autorizacao da UI nunca deve confiar cegamente nele -
      // revalidamos a sessao/role direto no backend (GET /api/auth/me, que
      // exige o cookie HttpOnly de access_token) antes de liberar a tela.
      try {
        const freshUser = await apiMe();
        if (cancelled) return;

        // Ressincroniza o cache local com o valor real vindo do backend
        // (corrige qualquer adulteracao local e mantem o cache util para
        // outras telas nao protegidas, ex.: saudacao no header).
        saveUser(freshUser);

        if (requireAdmin && freshUser.role !== "admin") {
          // Nao redirecionar para /dashboard aqui: a pagina de dashboard
          // tambem exige requireAdmin, o que causava um loop infinito de
          // redirect para usuarios nao-admin (ex.: role "vendedor"),
          // deixando a tela travada em "Verificando autenticacao...".
          // Redireciona para uma rota acessivel a qualquer usuario
          // autenticado.
          router.replace("/pagamentos");
          return;
        }

        setAuthorized(true);
        setReady(true);
      } catch {
        if (cancelled) return;
        // /api/auth/me falhou (401 mesmo apos tentativa de refresh, erro de
        // rede, etc.) - trata como sessao invalida: nao ha base segura para
        // decidir o que renderizar.
        clearUser();
        router.replace("/login");
      }
    }

    validate();

    return () => {
      cancelled = true;
    };
  }, [router, requireAdmin]);

  if (!ready || !authorized) {
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
