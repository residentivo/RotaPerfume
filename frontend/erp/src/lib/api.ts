const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8083'

async function apiFetch(path: string, options?: RequestInit) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options?.headers as Record<string, string> || {}),
  }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers })
  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: 'Erro desconhecido' }))
    throw new Error(error.error || `HTTP ${res.status}`)
  }
  return res.json()
}

export default apiFetch
