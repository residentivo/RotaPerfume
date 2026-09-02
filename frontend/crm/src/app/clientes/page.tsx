'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth'
import apiFetch from '@/lib/api'
import Navbar from '@/components/Navbar'
import Sidebar from '@/components/Sidebar'
import DataTable, { Column } from '@/components/DataTable'

interface Cliente {
  id: number
  cnpj: string
  razao_social: string
  nome_fantasia?: string
  segmento?: string
  cidade?: string
  uf?: string
  ativo: boolean
  telefone?: string
  email?: string
}

const clienteVazio: Partial<Cliente> = {
  cnpj: '',
  razao_social: '',
  nome_fantasia: '',
  segmento: '',
  cidade: '',
  uf: '',
  ativo: true,
  telefone: '',
  email: '',
}

export default function ClientesPage() {
  const { token, isLoading: authLoading } = useAuth()
  const router = require('next/navigation').useRouter()
  const [clientes, setClientes] = useState<Cliente[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showModal, setShowModal] = useState(false)
  const [editando, setEditando] = useState<Partial<Cliente>>(clienteVazio)
  const [filtroUf, setFiltroUf] = useState('')
  const [filtroSegmento, setFiltroSegmento] = useState('')
  const [busca, setBusca] = useState('')

  useEffect(() => {
    if (!authLoading && !token) {
      router.push('/login')
      return
    }
    if (token) {
      loadClientes()
    }
  }, [token, authLoading, router])

  const loadClientes = async () => {
    try {
      setLoading(true)
      const data = await apiFetch('/api/clientes')
      setClientes(Array.isArray(data) ? data : data.clientes || [])
    } catch (err: any) {
      setError(err.message || 'Erro ao carregar clientes')
      setClientes([])
    } finally {
      setLoading(false)
    }
  }

  const handleSalvar = async () => {
    try {
      if (editando.id) {
        await apiFetch(`/api/clientes/${editando.id}`, {
          method: 'PUT',
          body: JSON.stringify(editando),
        })
      } else {
        await apiFetch('/api/clientes', {
          method: 'POST',
          body: JSON.stringify(editando),
        })
      }
      setShowModal(false)
      setEditando(clienteVazio)
      loadClientes()
    } catch (err: any) {
      setError(err.message || 'Erro ao salvar cliente')
    }
  }

  const handleEditar = (cliente: Cliente) => {
    setEditando(cliente)
    setShowModal(true)
  }

  const handleNovo = () => {
    setEditando(clienteVazio)
    setShowModal(true)
  }

  const clientesFiltrados = clientes.filter((c) => {
    if (filtroUf && c.uf !== filtroUf) return false
    if (filtroSegmento && c.segmento !== filtroSegmento) return false
    if (busca) {
      const b = busca.toLowerCase()
      return (
        c.razao_social?.toLowerCase().includes(b) ||
        c.cnpj?.toLowerCase().includes(b) ||
        c.nome_fantasia?.toLowerCase().includes(b)
      )
    }
    return true
  })

  const ufs = Array.from(new Set(clientes.map((c) => c.uf).filter(Boolean)))
  const segmentos = Array.from(new Set(clientes.map((c) => c.segmento).filter(Boolean)))

  const columns: Column<Cliente>[] = [
    { key: 'cnpj', label: 'CNPJ', render: (c) => c.cnpj },
    { key: 'razao_social', label: 'Razão Social', render: (c) => c.razao_social },
    { key: 'segmento', label: 'Segmento', render: (c) => c.segmento || '-' },
    { key: 'cidade', label: 'Cidade', render: (c) => c.cidade || '-' },
    { key: 'uf', label: 'UF', render: (c) => c.uf || '-' },
    {
      key: 'ativo',
      label: 'Status',
      render: (c) => (
        <span className={`px-2 py-1 text-xs rounded-full ${
          c.ativo ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
        }`}>
          {c.ativo ? 'Ativo' : 'Inativo'}
        </span>
      ),
    },
    {
      key: 'acoes',
      label: 'Ações',
      render: (c) => (
        <button
          onClick={() => handleEditar(c)}
          className="text-primary-600 hover:text-primary-700 font-medium"
        >
          Editar
        </button>
      ),
    },
  ]

  if (authLoading || !token) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"></div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen bg-gray-100">
      <Sidebar />
      <div className="flex-1">
        <Navbar title="Clientes" />
        <main className="p-6">
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-4">
              {error}
            </div>
          )}

          {/* Filtros e ações */}
          <div className="bg-white rounded-lg shadow p-4 mb-6">
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
              <input
                type="text"
                placeholder="Buscar por CNPJ, razão social..."
                value={busca}
                onChange={(e) => setBusca(e.target.value)}
                className="px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500"
              />
              <select
                value={filtroUf}
                onChange={(e) => setFiltroUf(e.target.value)}
                className="px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500"
              >
                <option value="">Todos os estados</option>
                {ufs.map((uf) => (
                  <option key={uf} value={uf}>{uf}</option>
                ))}
              </select>
              <select
                value={filtroSegmento}
                onChange={(e) => setFiltroSegmento(e.target.value)}
                className="px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500"
              >
                <option value="">Todos os segmentos</option>
                {segmentos.map((s) => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </select>
              <button
                onClick={handleNovo}
                className="bg-primary-600 hover:bg-primary-700 text-white px-4 py-2 rounded-lg transition-colors"
              >
                + Novo Cliente
              </button>
            </div>
          </div>

          <DataTable data={clientesFiltrados} columns={columns} loading={loading} />

          {/* Modal de edição/criação */}
          {showModal && (
            <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
              <div className="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
                <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
                  <h3 className="text-lg font-semibold">
                    {editando.id ? 'Editar Cliente' : 'Novo Cliente'}
                  </h3>
                  <button onClick={() => setShowModal(false)} className="text-gray-500 hover:text-gray-700">
                    ✕
                  </button>
                </div>
                <div className="p-6 grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">CNPJ *</label>
                    <input
                      type="text"
                      value={editando.cnpj || ''}
                      onChange={(e) => setEditando({ ...editando, cnpj: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Razão Social *</label>
                    <input
                      type="text"
                      value={editando.razao_social || ''}
                      onChange={(e) => setEditando({ ...editando, razao_social: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Nome Fantasia</label>
                    <input
                      type="text"
                      value={editando.nome_fantasia || ''}
                      onChange={(e) => setEditando({ ...editando, nome_fantasia: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Segmento</label>
                    <input
                      type="text"
                      value={editando.segmento || ''}
                      onChange={(e) => setEditando({ ...editando, segmento: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Cidade</label>
                    <input
                      type="text"
                      value={editando.cidade || ''}
                      onChange={(e) => setEditando({ ...editando, cidade: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">UF</label>
                    <input
                      type="text"
                      maxLength={2}
                      value={editando.uf || ''}
                      onChange={(e) => setEditando({ ...editando, uf: e.target.value.toUpperCase() })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Telefone</label>
                    <input
                      type="text"
                      value={editando.telefone || ''}
                      onChange={(e) => setEditando({ ...editando, telefone: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
                    <input
                      type="email"
                      value={editando.email || ''}
                      onChange={(e) => setEditando({ ...editando, email: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div className="md:col-span-2">
                    <label className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={editando.ativo ?? true}
                        onChange={(e) => setEditando({ ...editando, ativo: e.target.checked })}
                        className="w-4 h-4"
                      />
                      <span className="text-sm font-medium text-gray-700">Cliente ativo</span>
                    </label>
                  </div>
                </div>
                <div className="px-6 py-4 border-t border-gray-200 flex justify-end gap-3">
                  <button
                    onClick={() => setShowModal(false)}
                    className="px-4 py-2 border border-gray-300 rounded-lg hover:bg-gray-50"
                  >
                    Cancelar
                  </button>
                  <button
                    onClick={handleSalvar}
                    className="px-4 py-2 bg-primary-600 hover:bg-primary-700 text-white rounded-lg"
                  >
                    Salvar
                  </button>
                </div>
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  )
}
