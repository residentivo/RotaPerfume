"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/Button";
import { logout, getUser } from "@/lib/auth";
import { User } from "@/lib/types";

export function Navbar() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    setUser(getUser());
  }, []);

  const handleLogout = () => {
    logout();
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
          {user?.role === "admin" && (
            <Link
              href="/admin/usuarios"
              className="hidden text-sm font-medium text-slate-600 hover:text-primary-600 sm:block"
            >
              Administração
            </Link>
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
