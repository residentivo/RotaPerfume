"use client";

import { ReactNode } from "react";
import Link from "next/link";
import { Alert } from "@/components/ui/Alert";
import { useVendedorDesligado } from "@/lib/session";
import { AVISO_VENDEDOR_DESLIGADO } from "@/lib/vendedorDesligado";

/** Aviso padrao de carteira bloqueada, com link para o Dashboard. */
export function VendedorDesligadoAviso() {
  return (
    <Alert variant="warning">
      <p>{AVISO_VENDEDOR_DESLIGADO}</p>
      <p className="mt-2">
        <Link
          href="/dashboard"
          className="font-medium text-primary-700 underline hover:text-primary-800"
        >
          Ir para o Dashboard
        </Link>
      </p>
    </Alert>
  );
}

/**
 * Envolve as telas da carteira (clientes, pedidos, pagamentos, oportunidades,
 * visitas). Se o usuario `normal` estiver vinculado a vendedor desligado
 * (flag de /api/auth/me ou 403 "acesso bloqueado: vendedor desligado"
 * recebido de qualquer rota), desmonta a tela e mostra o aviso — sem retry
 * automatico das requisicoes.
 */
export function CarteiraGuard({ title, children }: { title: string; children: ReactNode }) {
  const bloqueado = useVendedorDesligado();

  if (bloqueado) {
    return (
      <div>
        <h1 className="mb-6 text-2xl font-bold text-slate-900">{title}</h1>
        <VendedorDesligadoAviso />
      </div>
    );
  }

  return <>{children}</>;
}
