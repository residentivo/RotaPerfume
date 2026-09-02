'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useAuth } from '@/lib/auth'

export default function Sidebar() {
  const pathname = usePathname()
  const { user, logout } = useAuth()

  const menuItems = [
    { href: '/dashboard', label: 'Dashboard', icon: '🏠' },
    { href: '/vendedores', label: 'Vendedores', icon: '👥' },
    { href: '/usuarios', label: 'Usuários', icon: '🔐' },
  ]

  const isActive = (path: string) => pathname === path

  return (
    <aside className="fixed top-0 left-0 h-full w-64 bg-gray-900 text-white shadow-xl z-50">
      <div className="p-6 border-b border-gray-700">
        <h1 className="text-2xl font-bold">Sistema RH</h1>
        <p className="text-gray-400 text-sm mt-1">Gestão de Pessoas</p>
      </div>

      <nav className="mt-6 px-3">
        {menuItems.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className={`flex items-center gap-3 px-4 py-3 mb-1 rounded-lg transition-colors ${
              isActive(item.href)
                ? 'bg-blue-600 text-white'
                : 'text-gray-300 hover:bg-gray-800'
            }`}
          >
            <span className="text-xl">{item.icon}</span>
            <span className="font-medium">{item.label}</span>
          </Link>
        ))}
      </nav>

      <div className="absolute bottom-0 left-0 right-0 p-4 border-t border-gray-700">
        {user && (
          <div className="mb-3 px-2">
            <p className="text-sm font-medium text-white">{user.nome}</p>
            <p className="text-xs text-gray-400 capitalize">{user.tipoUsuario}</p>
          </div>
        )}
        <button
          onClick={logout}
          className="w-full bg-red-600 hover:bg-red-700 text-white font-medium py-2 px-4 rounded-lg transition-colors"
        >
          Sair
        </button>
      </div>
    </aside>
  )
}
