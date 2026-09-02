'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth'
import apiFetch from '@/lib/api'
import Navbar from '@/components/Navbar'
import Sidebar from '@/components/Sidebar'
import DataTable, { Column } from '@/components/DataTable'

interface Oportunidade {
  id: number
  cliente_id: number
  cliente_nome?: string
  vendedor_id: number
  vendedor_nome?: string
  titulo?: string
  descricao?: string
  etapa: string
  valor_estimado?: number
  probabilidade?: number
  data_fechamento_esperada?: string
}

interface Cliente {
  id: number
  razao_social: string
}

interface Vendedor {
  id: number
  nome: string
}

const ETAPAS = [
  'Prospecção',
  'Qualificação',
  'Proposta',
  'Negociação',
  'Fechamento',
  'Perdido',
  'Ganho',
]

const oportunidadeVazia: Partial<Oportunidade> = {
  cliente_id: 0,
  vendedor_id: 0,
  titulo: '',
  descricao: '',
  etapa: 'Prospecção',
  valor_estimado: 0,
  probabilidade: 0,
  data_fechamento_esperada: '',
}

export default function OportunidadesPage() {
  const { token, usuario, isLoading: authLoading } = useAuth()
  const router = require('next/navigation').useRouter()
  const [oportunidades, setOportunidades] = useState<Oportunidade[]>([])
  const [clientes, setClientes] = useState<Cliente[]>([])
  const [vendedores, setVendedores] = useState<Vendedor[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showModal, setShowModal] = useState(false)
  const [editando, setEditando] = useState<Partial<Oportunidade>>(oportunidadeVazia)
  const [filtroEtapa, setFiltroEtapa] = useState('')

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
      const [oppData, clientesData, vendedoresData] = await Promise.all([
        apiFetch('/api/oportunidades').catch(() => []),
        apiFetch('/api/clientes').catch(() => []),
        apiFetch('/api/vendedores').catch(() => []),
      ])
      setOportunidades(Array.isArray(oppData) ? oppData : oppData.oportunidades || [])
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
        await apiFetch(`/api/oportunidades/${editando.id}`, {
          method: 'PUT',
          body: JSON.stringify(editando),
        })
      } else {
        await apiFetch('/api/oportunidades', {
          method: 'POST',
          body: JSON.stringify(editando),
        })
      }
      setShowModal(false)
      setEditando(oportunidadeVazia)
      loadData()
    } catch (err: any) {
      setError(err.message || 'Erro ao salvar oportunidade')
    }
  }

  const handleEditar = (opp: Oportunidade) => {
    setEditando(opp)
    setShowModal(true)
  }

  const handleNovo = () => {
    setEditando({ ...oportunidadeVazia, vendedor_id: usuario?.id || 0 })
    setShowModal(true)
  }

  const oportunidadesFiltradas = oportunidades.filter((o) => {
    if (filtroEtapa && o.etapa !== filtroEtapa) return false
    return true
  })

  const formatarMoeda = (valor?: number) => {
    if (!valor) return '-'
    return valor.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
  }

  const getEtapaColor = (etapa: string) => {
    switch (etapa) {
      case 'Prospecção': return 'bg-blue-100 text-blue-800'
      case 'Qualificação': return 'bg-purple-100 text-purple-800'
      case 'Proposta': return 'bg-yellow-100 text-yellow-800'
      case 'Negociação': return 'bg-orange-100 text-orange-800'
      case 'Fechamento': return 'bg-indigo-100 text-indigo-800'
      case 'Ganho': return 'bg-green-100 text-green-800'
      case 'Perdido': return 'bg-red-100 text-red-800'
      default: return 'bg-gray-100 text-gray-800'
    }
  }

  const columns: Column<Oportunidade>[] = [
    { key: 'titulo', label: 'Título', render: (o) => o.titulo || '-' },
    { key: 'cliente_nome', label: 'Cliente', render: (o) => o.cliente_nome || '-' },
    { key: 'vendedor_nome', label: 'Vendedor', render: (o) => o.vendedor_nome || '-' },
    {
      key: 'etapa',
      label: 'Etapa',
      render: (o) => (
        <span className={`px-2 py-1 text-xs rounded-full ${getEtapaColor(o.etapa)}`}>
          {o.etapa}
        </span>
      ),
    },
    {
      key: 'valor_estimado',
      label: 'Valor',
      render: (o) => formatarMoeda(o.valor_estimado),
    },
    {
      key: 'probabilidade',
      label: 'Prob.',
      render: (o) => o.probabilidade ? `${o.probabilidade}%` : '-',
    },
    {
      key: 'acoes',
      label: 'Ações',
      render: (o) => (
        <button
          onClick={() => handleEditar(o)}
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
        <Navbar title="Oportunidades" />
        <main className="p-6">
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-4">
              {error}
            </div>
          )}

          <div className="bg-white rounded-lg shadow p-4 mb-6">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Filtrar por Etapa</label>
                <select
                  value={filtroEtapa}
                  onChange={(e) => setFiltroEtapa(e.target.value)}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500"
                >
                  <option value="">Todas as etapas</option>
                  {ETAPAS.map((e) => (
                    <option key={e} value={e}>{e}</option>
                  ))}
                </select>
              </div>
              <div className="flex items-end">
                <button
                  onClick={handleNovo}
                  className="w-full bg-primary-600 hover:bg-primary-700 text-white px-4 py-2 rounded-lg transition-colors"
                >
                  + Nova Oportunidade
                </button>
              </div>
            </div>
          </div>

          <DataTable data={oportunidadesFiltradas} columns={columns} loading={loading} />

          {showModal && (
            <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
              <div className="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
                <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
                  <h3 className="text-lg font-semibold">
                    {editando.id ? 'Editar Oportunidade' : 'Nova Oportunidade'}
                  </h3>
                  <button onClick={() => setShowModal(false)} className="text-gray-500 hover:text-gray-700">
                    ✕
                  </button>
                </div>
                <div className="p-6 grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="md:col-span-2">
                    <label className="block text-sm font-medium text-gray-700 mb-1">Título *</label>
                    <input
                      type="text"
                      value={editando.titulo || ''}
                      onChange={(e) => setEditando({ ...editando, titulo: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                      placeholder="Nome da oportunidade"
                    />
                  </div>
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
                    <label className="block text-sm font-medium text-gray-700 mb-1">Etapa</label>
                    <select
                      value={editando.etapa || 'Prospecção'}
                      onChange={(e) => setEditando({ ...editando, etapa: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    >
                      {ETAPAS.map((e) => (
                        <option key={e} value={e}>{e}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Valor Estimado (R$)</label>
                    <input
                      type="number"
                      step="0.01"
                      value={editando.valor_estimado || ''}
                      onChange={(e) => setEditando({ ...editando, valor_estimado: Number(e.target.value) })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Probabilidade (%)</label>
                    <input
                      type="number"
                      min="0"
                      max="100"
                      value={editando.probabilidade || ''}
                      onChange={(e) => setEditando({ ...editando, probabilidade: Number(e.target.value) })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Data Fechamento Esperada</label>
                    <input
                      type="date"
                      value={editando.data_fechamento_esperada || ''}
                      onChange={(e) => setEditando({ ...editando, data_fechamento_esperada: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                    />
                  </div>
                  <div className="md:col-span-2">
                    <label className="block text-sm font-medium text-gray-700 mb-1">Descrição</label>
                    <textarea
                      value={editando.descricao || ''}
                      onChange={(e) => setEditando({ ...editando, descricao: e.target.value })}
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
