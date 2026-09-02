'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth'
import apiFetch from '@/lib/api'
import Navbar from '@/components/Navbar'
import Sidebar from '@/components/Sidebar'
import DataTable, { Column } from '@/components/DataTable'

interface Visita {
  id: number
  cliente_id: number
  cliente_nome?: string
  vendedor_id: number
  vendedor_nome?: string
  data_visita: string
  resultado?: string
  duracao_minutos?: number
  observacao?: string
}

interface Cliente {
  id: number
  razao_social: string
}

interface Vendedor {
  id: number
  nome: string
}

const visitaVazia: Partial<Visita> = {
  cliente_id: 0,
  vendedor_id: 0,
  data_visita: new Date().toISOString().split('T')[0],
  resultado: '',
  duracao_minutos: 0,
  observacao: '',
}

export default function VisitasPage() {
  const { token, usuario, isLoading: authLoading } = useAuth()
  const router = require('next/navigation').useRouter()
  const [visitas, setVisitas] = useState<Visita[]>([])
  const [clientes, setClientes] = useState<Cliente[]>([])
  const [vendedores, setVendedores] = useState<Vendedor[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showModal, setShowModal] = useState(false)
  const [editando, setEditando] = useState<Partial<Visita>>(visitaVazia)
  const [filtroVendedor, setFiltroVendedor] = useState('')

  useEffect(() => {
    if (!authLoading && !token) {
      router.push('/login')
      return
    }
    if (token) {
      loadData()
    }
  }, [token, authLoading, router])

  const loadData = async () => {
    try {
      setLoading(true)
      const [visitasData, clientesData, vendedoresData] = await Promise.all([
        apiFetch('/api/visitas').catch(() => []),
        apiFetch('/api/clientes').catch(() => []),
        apiFetch('/api/vendedores').catch(() => []),
      ])
      setVisitas(Array.isArray(visitasData) ? visitasData : visitasData.visitas || [])
      setClientes(Array.isArray(clientesData) ? clientesData : clientesData.clientes || [])
      setVendedores(Array.isArray(vendedoresData) ? vendedoresData : vendedoresData.vendedores || [])
    } catch (err: any) {
      setError(err.message || 'Erro ao carregar dados')
    } finally {
      setLoading(false)
    }
  }

  const handleSalvar = async () => {
    try {
      if (editando.id) {
        await apiFetch(`/api/visitas/${editando.id}`, {
          method: 'PUT',
          body: JSON.stringify(editando),
        })
      } else {
        await apiFetch('/api/visitas', {
          method: 'POST',
          body: JSON.stringify(editando),
        })
      }
      setShowModal(false)
      setEditando(visitaVazia)
      loadData()
    } catch (err: any) {
      setError(err.message || 'Erro ao salvar visita')
    }
  }

  const handleEditar = (visita: Visita) => {
    setEditando(visita)
    setShowModal(true)
  }

  const handleNovo = () => {
    setEditando({ ...visitaVazia, vendedor_id: usuario?.id || 0 })
    setShowModal(true)
  }

  const visitasFiltradas = visitas.filter((v) => {
    if (filtroVendedor && v.vendedor_id.toString() !== filtroVendedor) return false
    return true
  })

  const formatarData = (data: string) => {
    if (!data) return '-'
    const date = new Date(data)
    return date.toLocaleDateString('pt-BR')
  }

  const columns: Column<Visita>[] = [
    { key: 'cliente_nome', label: 'Cliente', render: (v) => v.cliente_nome || '-' },
    { key: 'vendedor_nome', label: 'Vendedor', render: (v) => v.vendedor_nome || '-' },
    { key: 'data_visita', label: 'Data', render: (v) => formatarData(v.data_visita) },
    { key: 'resultado', label: 'Resultado', render: (v) => v.resultado || '-' },
    {
      key: 'duracao_minutos',
      label: 'Duração',
      render: (v) => v.duracao_minutos ? `${v.duracao_minutos} min` : '-',
    },
    {
      key: 'acoes',
      label: 'Ações',
      render: (v) => (
        <button
          onClick={() => handleEditar(v)}
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
        <Navbar title="Visitas" />
        <main className="p-6">
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-4">
              {error}
            </div>
          )}

          <div className="bg-white rounded-lg shadow p-4 mb-6">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Filtrar por Vendedor</label>
                <select
                  value={filtroVendedor}
                  onChange={(e) => setFiltroVendedor(e.target.value)}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500"
                >
                  <option value="">Todos os vendedores</option>
                  {vendedores.map((v) => (
                    <option key={v.id} value={v.id}>{v.nome}</option>
                  ))}
                </select>
              </div>
              <div className="flex items-end">
                <button
                  onClick={handleNovo}
                  className="w-full bg-primary-600 hover:bg-primary-700 text-white px-4 py-2 rounded-lg transition-colors"
                >
                  + Nova Visita
                </button>
              </div>
            </div>
          </div>

          <DataTable data={visitasFiltradas} columns={columns} loading={loading} />

          {showModal && (
            <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
              <div className="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
                <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
                  <h3 className="text-lg font-semibold">
                    {editando.id ? 'Editar Visita' : 'Nova Visita'}
                  </h3>
                  <button onClick={() => setShowModal(false)} className="text-gray-500 hover:text-gray-700">
                    ✕
                  </button>
                </div>
                <div className="p-6 grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Cliente *</label>
                    <select
                      value={editando.cliente_id || ''}
                      onChange={(e) => setEditando({ ...editando, cliente_id: Number(e.target.value) })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    >
                      <option value="">Selecione...</option>
                      {clientes.map((c) => (
                        <option key={c.id} value={c.id}>{c.razao_social}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Vendedor *</label>
                    <select
                      value={editando.vendedor_id || ''}
                      onChange={(e) => setEditando({ ...editando, vendedor_id: Number(e.target.value) })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    >
                      <option value="">Selecione...</option>
                      {vendedores.map((v) => (
                        <option key={v.id} value={v.id}>{v.nome}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Data da Visita *</label>
                    <input
                      type="date"
                      value={editando.data_visita || ''}
                      onChange={(e) => setEditando({ ...editando, data_visita: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Duração (minutos)</label>
                    <input
                      type="number"
                      value={editando.duracao_minutos || ''}
                      onChange={(e) => setEditando({ ...editando, duracao_minutos: Number(e.target.value) })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div className="md:col-span-2">
                    <label className="block text-sm font-medium text-gray-700 mb-1">Resultado</label>
                    <input
                      type="text"
                      value={editando.resultado || ''}
                      onChange={(e) => setEditando({ ...editando, resultado: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                      placeholder="Ex: Positivo, Negativo, Retornar..."
                    />
                  </div>
                  <div className="md:col-span-2">
                    <label className="block text-sm font-medium text-gray-700 mb-1">Observação</label>
                    <textarea
                      value={editando.observacao || ''}
                      onChange={(e) => setEditando({ ...editando, observacao: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                      rows={3}
                    />
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
