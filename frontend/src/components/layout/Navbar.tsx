"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/Button";
import { logout } from "@/lib/auth";
import { useSessionUser, useVendedorDesligado } from "@/lib/session";
import { useAjustarAoMudar } from "@/lib/useResetOnOpen";
import { vlog } from "@/lib/vlog";

interface NavDropdownItem {
  label: string;
  href: string;
}

interface NavGrupo {
  label: string;
  items: NavDropdownItem[];
}

/** id do painel do menu mobile (alvo do aria-controls do hamburguer). */
const MENU_MOBILE_ID = "navbar-menu-mobile";

function NavDropdown({ label, items }: { label: string; items: NavDropdownItem[] }) {
  vlog("Navbar.tsx", "NavDropdown", "verificando se o dropdown tem itens:", items.length);
  if (items.length === 0) {
    return null;
  }

  return (
    <div className="group relative">
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

const LINK_MOBILE =
  "flex min-h-[44px] items-center rounded-md px-3 text-base font-medium text-slate-700 hover:bg-slate-50 hover:text-primary-600";

export function Navbar() {
  // A Navbar so e renderizada dentro do ProtectedRoute, que popula a sessao
  // em memoria via /api/auth/me antes de liberar os filhos; a sessao e a
  // fonte da verdade (inclui id_vendedor e vendedor_desligado).
  vlog("Navbar.tsx", "Navbar", "lendo usuário da sessão");
  const user = useSessionUser();
  // Proativo (UX): vendedor desligado nao ve os itens da carteira no menu.
  // O bloqueio real e do backend (403).
  vlog("Navbar.tsx", "Navbar", "lendo bloqueio da carteira");
  const carteiraBloqueada = useVendedorDesligado();

  // UX-01: menu hamburguer (abaixo de lg). Fecha ao mudar de rota, ao clicar
  // num link e com Esc.
  vlog("Navbar.tsx", "Navbar", "lendo rota atual");
  const pathname = usePathname();
  vlog("Navbar.tsx", "Navbar", "criando estado do menu mobile");
  const [menuAberto, setMenuAberto] = useState(false);
  const botaoMenuRef = useRef<HTMLButtonElement>(null);

  vlog("Navbar.tsx", "Navbar", "registrando fechamento do menu ao mudar de rota");
  useAjustarAoMudar([pathname], () => {
    vlog("Navbar.tsx", "Navbar.func", "rota mudou; fechando menu mobile");
    setMenuAberto(false);
  });

  vlog("Navbar.tsx", "Navbar", "registrando efeito de fechar o menu com Esc");
  useEffect(() => {
    vlog("Navbar.tsx", "Navbar.useEffect", "verificando se o menu mobile está aberto:", menuAberto);
    if (!menuAberto) return;
    const handler = (e: KeyboardEvent) => {
      vlog("Navbar.tsx", "Navbar.handler", "verificando se a tecla é Escape");
      if (e.key === "Escape") {
        vlog("Navbar.tsx", "Navbar.handler", "fechando menu mobile e devolvendo o foco ao botão");
        setMenuAberto(false);
        botaoMenuRef.current?.focus();
      }
    };
    vlog("Navbar.tsx", "Navbar.useEffect", "registrando listener de keydown");
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [menuAberto]);

  const handleLogout = () => {
    vlog("Navbar.tsx", "Navbar.handleLogout", "executando logout");
    void logout();
  };

  const alternarMenu = () => {
    vlog("Navbar.tsx", "Navbar.alternarMenu", "alternando menu mobile, aberto antes:", menuAberto);
    setMenuAberto((v) => !v);
  };

  const fecharMenu = () => {
    vlog("Navbar.tsx", "Navbar.fecharMenu", "fechando menu mobile após clique em link");
    setMenuAberto(false);
  };

  // Itens da carteira: ocultos para vendedor desligado (o dropdown sem itens
  // nao e renderizado). Mesmas regras no desktop e no menu mobile.
  vlog("Navbar.tsx", "Navbar", "montando grupos do menu, carteira bloqueada:", carteiraBloqueada);
  const isAdmin = user?.role === "admin";
  const grupos: NavGrupo[] = [
    {
      label: "ERP",
      items: carteiraBloqueada
        ? []
        : [
            { label: "Clientes", href: "/admin/clientes" },
            { label: "Oportunidades", href: "/admin/oportunidades" },
            { label: "Visitas", href: "/admin/visitas" },
          ],
    },
    {
      label: "CRM",
      items: carteiraBloqueada
        ? []
        : [
            { label: "Pagamentos", href: "/pagamentos" },
            { label: "Pedidos", href: "/admin/pedidos" },
          ],
    },
    {
      label: "Administração",
      items: isAdmin
        ? [
            { label: "Auditoria de Senha", href: "/admin/senha-historico" },
            { label: "Vendedores", href: "/admin/vendedores" },
            { label: "Usuários", href: "/admin/usuarios" },
            { label: "Produtos", href: "/admin/produtos" },
            { label: "Estoque", href: "/admin/estoque" },
          ]
        : [],
    },
  ];

  const papel = user ? (user.role === "admin" ? "Administrador" : user.role) : "";
  const inicial = user?.nome?.charAt(0).toUpperCase() || "?";

  return (
    <nav className="border-b border-slate-200 bg-white shadow-sm">
      <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3 sm:px-6 lg:px-8">
        <div className="flex min-w-0 items-center gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary-600 text-white">
            <span className="text-lg font-bold">R</span>
          </div>
          <div className="min-w-0">
            <h1 className="text-base font-semibold text-slate-900">RotaPerfumes</h1>
            <p className="text-xs text-slate-500">Sistema de Gestao</p>
          </div>
        </div>

        {/* Desktop (lg+): links em linha, como antes. */}
        <div className="hidden items-center gap-4 lg:flex">
          {user && (
            <Link
              href="/dashboard"
              className="block text-sm font-medium text-slate-600 hover:text-primary-600"
            >
              Dashboard
            </Link>
          )}

          {user && (
            <Link
              href="/trocar-senha"
              className="block text-sm font-medium text-slate-600 hover:text-primary-600"
            >
              Trocar Senha
            </Link>
          )}

          {user &&
            grupos.map((g) => <NavDropdown key={g.label} label={g.label} items={g.items} />)}

          {user && (
            <div className="block text-right">
              <p className="text-sm font-medium text-slate-900">{user.nome}</p>
              <p className="text-xs text-slate-500">{papel}</p>
            </div>
          )}
          <div className="flex h-9 w-9 items-center justify-center rounded-full bg-slate-200 text-sm font-semibold text-slate-700">
            {inicial}
          </div>
          <Button variant="secondary" size="sm" onClick={handleLogout}>
            Sair
          </Button>
        </div>

        {/* Mobile/tablet (< lg): botao hamburguer. */}
        <button
          ref={botaoMenuRef}
          type="button"
          onClick={alternarMenu}
          aria-label={menuAberto ? "Fechar menu" : "Abrir menu"}
          aria-expanded={menuAberto}
          aria-controls={MENU_MOBILE_ID}
          className="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-md text-slate-600 hover:bg-slate-100 hover:text-slate-900 focus:outline-none focus:ring-2 focus:ring-primary-500 lg:hidden"
        >
          {menuAberto ? (
            <svg className="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          ) : (
            <svg className="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          )}
        </button>
      </div>

      {menuAberto && (
        <div
          id={MENU_MOBILE_ID}
          className="max-h-[calc(100vh-4rem)] overflow-y-auto border-t border-slate-200 bg-white px-4 pb-4 pt-2 sm:px-6 lg:hidden"
        >
          {user && (
            <div className="flex items-center gap-3 border-b border-slate-100 px-3 py-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-slate-200 text-sm font-semibold text-slate-700">
                {inicial}
              </div>
              <div className="min-w-0">
                <p className="truncate text-sm font-medium text-slate-900">{user.nome}</p>
                <p className="text-xs text-slate-500">{papel}</p>
              </div>
            </div>
          )}

          {user && (
            <ul className="mt-2 space-y-1">
              <li>
                <Link href="/dashboard" onClick={fecharMenu} className={LINK_MOBILE}>
                  Dashboard
                </Link>
              </li>
              <li>
                <Link href="/trocar-senha" onClick={fecharMenu} className={LINK_MOBILE}>
                  Trocar Senha
                </Link>
              </li>
            </ul>
          )}

          {user &&
            grupos
              .filter((g) => g.items.length > 0)
              .map((g) => (
                <div key={g.label} className="mt-3">
                  <p className="px-3 pb-1 text-xs font-semibold uppercase tracking-wider text-slate-400">
                    {g.label}
                  </p>
                  <ul className="space-y-1">
                    {g.items.map((item) => (
                      <li key={item.href}>
                        <Link href={item.href} onClick={fecharMenu} className={LINK_MOBILE}>
                          {item.label}
                        </Link>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}

          <div className="mt-4 border-t border-slate-100 pt-4">
            <Button variant="secondary" className="min-h-[44px] w-full" onClick={handleLogout}>
              Sair
            </Button>
          </div>
        </div>
      )}
    </nav>
  );
}
