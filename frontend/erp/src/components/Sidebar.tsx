'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useAuth } from '@/lib/auth'

const items = [
  { href: '/dashboard', label: 'Dashboard', icon: '📊' },
  { href: '/produtos', label: 'Produtos', icon: '📦' },
  { href: '/pedidos', label: 'Pedidos', icon: '🛒' },
  { href: '/pagamentos', label: 'Pagamentos', icon: '💳' },
]

export default function Sidebar() {
  const pathname = usePathname()
  const { user, logout } = useAuth()

  return (
    <aside className="w-64 bg-white border-r border-gray-200 flex flex-col h-screen sticky top-0">
      <div className="px-6 py-5 border-b border-gray-200">
        <Link href="/dashboard" className="flex items-center gap-2">
          <span className="text-2xl">🏢</span>
          <span className="text-lg font-bold text-gray-800">ERP System</span>
        </Link>
      </div>

      <nav className="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
        {items.map((item) => {
          const active = pathname === item.href || pathname?.startsWith(item.href + '/')
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                active
                  ? 'bg-primary-50 text-primary-700'
                  : 'text-gray-700 hover:bg-gray-100'
              }`}
            >
              <span className="text-lg">{item.icon}</span>
              <span>{item.label}</span>
            </Link>
          )
        })}
      </nav>

      <div className="border-t border-gray-200 p-4">
        <div className="mb-3">
          <p className="text-sm font-semibold text-gray-800 truncate">
            {user?.nome || user?.login || 'Usuário'}
          </p>
          <p className="text-xs text-gray-500 truncate">
            {user?.email || user?.tipoUsuario || ''}
          </p>
        </div>
        <button
          onClick={logout}
          className="w-full bg-red-50 hover:bg-red-100 text-red-700 text-sm font-medium py-2 px-3 rounded-md transition-colors"
        >
          Sair
        </button>
      </div>
    </aside>
  )
}
