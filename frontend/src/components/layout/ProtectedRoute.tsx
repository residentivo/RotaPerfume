"use client";

import { ReactNode, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getUser, isAdmin } from "@/lib/auth";

interface ProtectedRouteProps {
  children: ReactNode;
  requireAdmin?: boolean;
}

export function ProtectedRoute({ children, requireAdmin = false }: ProtectedRouteProps) {
  const router = useRouter();
  const [ready, setReady] = useState(false);
  const [authorized, setAuthorized] = useState(false);

  useEffect(() => {
    const user = getUser();

    // O access_token é HttpOnly (não legível via JS); a sessão real é
    // validada pelo backend em cada chamada de API (fetchWithAuth trata 401).
    if (!user) {
      router.replace("/login");
      return;
    }

    if (requireAdmin && !isAdmin()) {
      // Nao redirecionar para /dashboard aqui: a pagina de dashboard tambem
      // exige requireAdmin, o que causava um loop infinito de redirect para
      // usuarios nao-admin (ex.: role "vendedor"), deixando a tela travada
      // em "Verificando autenticacao...". Redireciona para uma rota
      // acessivel a qualquer usuario autenticado.
      router.replace("/pagamentos");
      return;
    }

    setAuthorized(true);
    setReady(true);
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
