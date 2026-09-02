'use client'

import { useAuth } from '@/lib/auth'
import { useRouter } from 'next/navigation'
import Link from 'next/link'

interface NavbarProps {
  title?: string
}

export default function Navbar({ title }: NavbarProps) {
  const { usuario, logout } = useAuth()
  const router = useRouter()

  const handleLogout = () => {
    logout()
    router.push('/login')
  }

  return (
    <header className="bg-white shadow-sm border-b border-gray-200">
      <div className="px-6 py-4 flex items-center justify-between">
        <div>
          {title && <h2 className="text-xl font-semibold text-gray-800">{title}</h2>}
        </div>

        <div className="flex items-center gap-4">
          <div className="text-right">
            <p className="text-sm font-medium text-gray-700">{usuario?.nome || 'Convidado'}</p>
            <p className="text-xs text-gray-500">{usuario?.tipo || '-'}</p>
          </div>
          <button
            onClick={handleLogout}
            className="text-sm text-red-600 hover:text-red-700 font-medium"
          >
            Sair
          </button>
        </div>
      </div>
    </header>
  )
}
