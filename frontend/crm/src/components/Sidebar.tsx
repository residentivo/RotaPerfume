'use client'

import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import { useAuth } from '@/lib/auth'

const menuItems = [
  { href: '/dashboard', label: 'Dashboard', icon: '📊' },
  { href: '/clientes', label: 'Clientes', icon: '🏢' },
  { href: '/visitas', label: 'Visitas', icon: '📅' },
  { href: '/oportunidades', label: 'Oportunidades', icon: '💰' },
]

export default function Sidebar() {
  const pathname = usePathname()
  const router = useRouter()
  const { usuario, logout } = useAuth()

  const handleLogout = () => {
    logout()
    router.push('/login')
  }

  return (
    <aside className="bg-slate-900 text-white w-64 min-h-screen flex flex-col">
      <div className="p-6 border-b border-slate-700">
        <h1 className="text-2xl font-bold">CRM</h1>
        <p className="text-slate-400 text-sm">Sistema de Vendas</p>
      </div>

      <nav className="flex-1 p-4">
        <ul className="space-y-2">
          {menuItems.map((item) => {
            const isActive = pathname === item.href
            return (
              <li key={item.href}>
                <Link
                  href={item.href}
                  className={`flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${
                    isActive
                      ? 'bg-primary-600 text-white'
                      : 'text-slate-300 hover:bg-slate-800'
                  }`}
                >
                  <span className="text-xl">{item.icon}</span>
                  <span className="font-medium">{item.label}</span>
                </Link>
              </li>
            )
          })}
        </ul>
      </nav>

      <div className="p-4 border-t border-slate-700">
        <div className="mb-3">
          <p className="text-sm text-slate-400">Usuário</p>
          <p className="font-semibold">{usuario?.nome || 'Convidado'}</p>
          <p className="text-xs text-slate-500">{usuario?.tipo || '-'}</p>
        </div>
        <button
          onClick={handleLogout}
          className="w-full bg-red-600 hover:bg-red-700 text-white px-4 py-2 rounded-lg transition-colors"
        >
          Sair
        </button>
      </div>
    </aside>
  )
}
