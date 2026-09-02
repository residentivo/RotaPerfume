'use client'

import { usePathname } from 'next/navigation'

export default function Navbar() {
  const pathname = usePathname()

  const getPageTitle = () => {
    switch (pathname) {
      case '/dashboard':
        return 'Dashboard'
      case '/vendedores':
        return 'Vendedores'
      case '/usuarios':
        return 'Usuários'
      default:
        return 'Sistema RH'
    }
  }

  return (
    <header className="bg-white shadow-sm border-b border-gray-200">
      <div className="px-6 py-4">
        <h2 className="text-2xl font-bold text-gray-800">{getPageTitle()}</h2>
      </div>
    </header>
  )
}
