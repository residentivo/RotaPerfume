'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'

export default function Home() {
  const router = useRouter()
  useEffect(() => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null
    if (token) router.replace('/dashboard')
    else router.replace('/login')
  }, [router])

  return (
    <div className="flex items-center justify-center min-h-screen text-gray-500">
      Redirecionando...
    </div>
  )
}
