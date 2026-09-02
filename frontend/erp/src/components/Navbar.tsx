'use client'

import { usePathname } from 'next/navigation'

const titles: Record<string, string> = {
  '/dashboard': 'Dashboard',
  '/produtos': 'Produtos',
  '/pedidos': 'Pedidos',
  '/pagamentos': 'Pagamentos',
}

export default function Navbar() {
  const pathname = usePathname()
  const title =
    Object.entries(titles).find(([k]) => pathname?.startsWith(k))?.[1] || 'ERP'

  return (
    <header className="bg-white border-b border-gray-200 px-6 py-4 flex items-center justify-between">
      <h1 className="text-xl font-semibold text-gray-800">{title}</h1>
      <div className="text-sm text-gray-500">
        {new Date().toLocaleDateString('pt-BR', {
          weekday: 'long',
          year: 'numeric',
          month: 'long',
          day: 'numeric',
        })}
      </div>
    </header>
  )
}
