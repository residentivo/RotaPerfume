"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/Button";
import { logout, getUser } from "@/lib/auth";
import { User } from "@/lib/types";

interface NavDropdownItem {
  label: string;
  href: string;
}

function NavDropdown({ label, items }: { label: string; items: NavDropdownItem[] }) {
  if (items.length === 0) {
    return null;
  }

  return (
    <div className="group relative hidden sm:block">
      <button
        type="button"
        className="flex items-center gap-1 text-sm font-medium text-slate-600 hover:text-primary-600"
      >
        {label}
        <svg className="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      <div className="invisible absolute left-0 top-full z-20 min-w-[10rem] rounded-lg border border-slate-200 bg-white py-1 opacity-0 shadow-lg transition-opacity duration-150 group-hover:visible group-hover:opacity-100">
        {items.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className="block px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 hover:text-primary-600"
          >
            {item.label}
          </Link>
        ))}
      </div>
    </div>
  );
}

export function Navbar() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    setUser(getUser());
  }, []);

  const handleLogout = () => {
    void logout();
  };

  return (
    <nav className="border-b border-slate-200 bg-white shadow-sm">
      <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3 sm:px-6 lg:px-8">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary-600 text-white">
            <span className="text-lg font-bold">R</span>
          </div>
          <div>
            <h1 className="text-base font-semibold text-slate-900">RotaPerfumes</h1>
            <p className="text-xs text-slate-500">Sistema de Gestao</p>
          </div>
        </div>

        <div className="flex items-center gap-4">
          {user && (
            <Link
              href="/dashboard"
              className="hidden text-sm font-medium text-slate-600 hover:text-primary-600 sm:block"
            >
              Dashboard
            </Link>
          )}

          {user && (
            <Link
              href="/trocar-senha"
              className="hidden text-sm font-medium text-slate-600 hover:text-primary-600 sm:block"
            >
              Trocar Senha
            </Link>
          )}

          {user && (
            <NavDropdown
              label="ERP"
              items={[
                { label: "Clientes", href: "/admin/clientes" },
                { label: "Oportunidades", href: "/admin/oportunidades" },
                { label: "Visitas", href: "/admin/visitas" },
              ]}
            />
          )}

          {user && (
            <NavDropdown
              label="CRM"
              items={[
                { label: "Pagamentos", href: "/pagamentos" },
                { label: "Pedidos", href: "/admin/pedidos" },
              ]}
            />
          )}

          {user && (
            <NavDropdown
              label="Administração"
              items={[
                ...(user.role === "admin"
                  ? [{ label: "Auditoria de Senha", href: "/admin/senha-historico" }]
                  : []),
                ...(user.role === "admin"
                  ? [{ label: "Vendedores", href: "/admin/vendedores" }]
                  : []),
                ...(user.role === "admin"
                  ? [{ label: "Usuários", href: "/admin/usuarios" }]
                  : []),
                ...(user.role === "admin"
                  ? [{ label: "Produtos", href: "/admin/produtos" }]
                  : []),
                ...(user.role === "admin"
                  ? [{ label: "Estoque", href: "/admin/estoque" }]
                  : []),
              ]}
            />
          )}

          {user && (
            <div className="hidden text-right sm:block">
              <p className="text-sm font-medium text-slate-900">{user.nome}</p>
              <p className="text-xs text-slate-500">
                {user.role === "admin" ? "Administrador" : user.role}
              </p>
            </div>
          )}
          <div className="flex h-9 w-9 items-center justify-center rounded-full bg-slate-200 text-sm font-semibold text-slate-700">
            {user?.nome?.charAt(0).toUpperCase() || "?"}
          </div>
          <Button variant="secondary" size="sm" onClick={handleLogout}>
            Sair
          </Button>
        </div>
      </div>
    </nav>
  );
}
