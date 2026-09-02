'use client'

import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import apiFetch from './api'

export type TipoUsuario = 'admin' | 'gerente' | 'vendedor' | 'operador' | string

export interface User {
  id?: number
  login?: string
  nome?: string
  email?: string
  tipoUsuario?: TipoUsuario
}

interface AuthContextData {
  user: User | null
  token: string | null
  tipoUsuario: TipoUsuario | null
  loading: boolean
  login: (login: string, senha: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextData | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [tipoUsuario, setTipoUsuario] = useState<TipoUsuario | null>(null)
  const [loading, setLoading] = useState(true)
  const router = useRouter()
  const pathname = usePathname()

  useEffect(() => {
    const storedToken = typeof window !== 'undefined' ? localStorage.getItem('token') : null
    const storedUser = typeof window !== 'undefined' ? localStorage.getItem('user') : null
    const storedTipo = typeof window !== 'undefined' ? localStorage.getItem('tipoUsuario') : null

    if (storedToken) {
      setToken(storedToken)
      if (storedUser) {
        try {
          setUser(JSON.parse(storedUser))
        } catch {
          setUser(null)
        }
      }
      if (storedTipo) setTipoUsuario(storedTipo)
    }
    setLoading(false)
  }, [])

  useEffect(() => {
    if (loading) return
    const publicPaths = ['/login']
    const isPublic = publicPaths.some((p) => pathname?.startsWith(p))
    if (!token && !isPublic) {
      router.replace('/login')
    } else if (token && pathname === '/') {
      router.replace('/dashboard')
    }
  }, [token, loading, pathname, router])

  const login = async (loginInput: string, senha: string) => {
    const data = await apiFetch('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ login: loginInput, senha }),
    })

    const tk = data.token || data.access_token || data.Token
    if (!tk) throw new Error('Token não retornado pelo servidor')

    const userData: User = {
      id: data.id ?? data.user?.id,
      login: data.login ?? data.user?.login ?? loginInput,
      nome: data.nome ?? data.user?.nome ?? loginInput,
      email: data.email ?? data.user?.email,
      tipoUsuario: data.tipoUsuario ?? data.tipo ?? data.user?.tipoUsuario,
    }
    const tipo = userData.tipoUsuario || data.tipo || null

    localStorage.setItem('token', tk)
    localStorage.setItem('user', JSON.stringify(userData))
    if (tipo) localStorage.setItem('tipoUsuario', String(tipo))

    setToken(tk)
    setUser(userData)
    setTipoUsuario(tipo)
    router.replace('/dashboard')
  }

  const logout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    localStorage.removeItem('tipoUsuario')
    setToken(null)
    setUser(null)
    setTipoUsuario(null)
    router.replace('/login')
  }

  return (
    <AuthContext.Provider value={{ user, token, tipoUsuario, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextData {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within an AuthProvider')
  return ctx
}
