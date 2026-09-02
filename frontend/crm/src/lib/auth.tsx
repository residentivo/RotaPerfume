'use client'

import React, { createContext, useContext, useState, useEffect, useCallback } from 'react'

interface Usuario {
  id: number
  login: string
  nome: string
  tipo: string
}

interface AuthContextType {
  token: string | null
  usuario: Usuario | null
  login: (login: string, senha: string) => Promise<void>
  logout: () => void
  isLoading: boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string | null>(null)
  const [usuario, setUsuario] = useState<Usuario | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const savedToken = localStorage.getItem('token')
    const savedUsuario = localStorage.getItem('usuario')
    if (savedToken && savedUsuario) {
      setToken(savedToken)
      try {
        setUsuario(JSON.parse(savedUsuario))
      } catch {
        setUsuario(null)
      }
    }
    setIsLoading(false)
  }, [])

  const login = useCallback(async (login: string, senha: string) => {
    // Usa API_URL ou fallback para /api/auth/login (via proxy do Next.js)
    const API_URL = process.env.NEXT_PUBLIC_API_URL
    const url = API_URL ? `${API_URL}/auth/login` : '/api/auth/login'

    const res = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login, senha }),
    })

    if (!res.ok) {
      const error = await res.json().catch(() => ({ error: 'Login inválido' }))
      throw new Error(error.error || 'Login inválido')
    }

    const data = await res.json()
    setToken(data.token)
    setUsuario(data.usuario)
    localStorage.setItem('token', data.token)
    localStorage.setItem('usuario', JSON.stringify(data.usuario))
  }, [])

  const logout = useCallback(() => {
    setToken(null)
    setUsuario(null)
    localStorage.removeItem('token')
    localStorage.removeItem('usuario')
  }, [])

  return (
    <AuthContext.Provider value={{ token, usuario, login, logout, isLoading }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}

export function useProtectedRoute() {
  const { token, isLoading } = useAuth()
  const router = require('next/navigation').useRouter()

  useEffect(() => {
    if (!isLoading && !token) {
      router.push('/login')
    }
  }, [token, isLoading, router])

  return { isLoading }
}
