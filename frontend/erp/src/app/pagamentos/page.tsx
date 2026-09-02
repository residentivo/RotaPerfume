'use client'

import { useEffect, useState } from 'react'
import Sidebar from '@/components/Sidebar'
import Navbar from '@/components/Navbar'
import DataTable, { Column } from '@/components/DataTable'
import apiFetch from '@/lib/api'

interface Pagamento {
  id?: number
  pedido_id?: number
  forma?: string
  parcelas?: number
  valor?: number
  vencimento?: string
  status?: string
  data_vencimento?: string
}

export default function PagamentosPage() {
  const [pagamentos, setPagamentos] = useState<Pagamento[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [filterStatus, setFilterStatus] = useState('')
  const [editItem, setEditItem] = useState<Pagamento | null>(null)

  useEffect(() => {
    loadPagamentos()
  }, [])

  async function loadPagamentos() {
    setLoading(true)
    setError('')
    try {
      const data = await apiFetch('/api/pagamentos')
      const list: Pagamento[] = Array.isArray(data) ? data : data.pagamentos ?? data.data ?? []
      setPagamentos(list)
    } catch (err: any) {
      setError(err.message || 'Erro ao carregar pagamentos')
    } finally {
      setLoading(false)
    }
  }

  const statusList = [...new Set(pagamentos.map((p) => p.status).filter(Boolean))] as string[]

  const filtered = pagamentos.filter((p) => {
    if (filterStatus && p.status !== filterStatus) return false
    return true
  })

  async function handleUpdateStatus(p: Pagamento, newStatus: string) {
    try {
      await apiFetch(`/api/pagamentos/${p.id}`, {
        method: 'PUT',
        body: JSON.stringify({ ...p, status: newStatus }),
      })
      loadPagamentos()
      setEditItem(null)
    } catch (err: any) {
      alert(err.message)
    }
  }

  const columns: Column<Pagamento>[] = [
    {
      key: 'pedido_id',
      label: 'Pedido',
      render: (row) => <span className="font-medium">#{row.pedido_id}</span>,
    },
    { key: 'forma', label: 'Forma Pagamento' },
    {
      key: 'parcelas',
      label: 'Parcelas',
      render: (row) => (row.parcelas ? `${row.parcelas}x` : '-'),
    },
    {
      key: 'valor',
      label: 'Valor',
      render: (row) =>
        (row.valor ?? 0).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' }),
    },
    {
      key: 'vencimento',
      label: 'Vencimento',
      render: (row) => {
        const dateStr = row.vencimento || row.data_vencimento
        return dateStr ? new Date(dateStr).toLocaleDateString('pt-BR') : '-'
      },
    },
    {
      key: 'status',
      label: 'Status',
      render: (row) => {
        const colors: Record<string, string> = {
          pendente: 'bg-yellow-100 text-yellow-700',
          pago: 'bg-green-100 text-green-700',
          cancelado: 'bg-red-100 text-red-700',
          vencido: 'bg-orange-100 text-orange-700',
        }
        const cls = colors[row.status?.toLowerCase() ?? ''] || 'bg-gray-100 text-gray-600'
        return (
          <div className="flex items-center gap-2">
            <span className={`px-2 py-1 rounded text-xs font-medium ${cls}`}>{row.status}</span>
          </div>
        )
      },
    },
    {
      key: 'actions',
      label: 'Acoes',
      render: (row) => (
        <button
          onClick={(e) => {
            e.stopPropagation()
            setEditItem(row)
          }}
          className="text-primary-600 hover:text-primary-800 text-sm font-medium"
        >
          Editar
        </button>
      ),
    },
  ]

  return (
    <div className="flex min-h-screen bg-gray-100">
      <Sidebar />
      <div className="flex-1 flex flex-col">
        <Navbar />
        <main className="p-6 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold text-gray-800">Pagamentos</h2>
            <button
              onClick={loadPagamentos}
              className="bg-gray-100 hover:bg-gray-200 text-gray-700 text-sm font-medium py-2 px-4 rounded-md transition-colors"
            >
              Atualizar
            </button>
          </div>

          <div className="flex gap-4 flex-wrap">
            <select
              value={filterStatus}
              onChange={(e) => setFilterStatus(e.target.value)}
              className="border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
            >
              <option value="">Todos Status</option>
              {statusList.map((s) => (
                <option key={s} value={s}>{s}</option>
              ))}
            </select>
          </div>

          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md text-sm">
              {error}
            </div>
          )}

          <DataTable
            columns={columns}
            rows={filtered}
            loading={loading}
            emptyMessage="Nenhum pagamento encontrado"
            keyExtractor={(p) => p.id ?? 0}
          />
        </main>
      </div>

      {editItem && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-lg shadow-xl w-full max-w-sm">
            <div className="px-6 py-4 border-b border-gray-200">
              <h3 className="text-lg font-semibold text-gray-800">
                Atualizar Pagamento #{editItem.id}
              </h3>
            </div>
            <div className="p-6 space-y-4">
              <div className="text-sm space-y-2 text-gray-600">
                <p>
                  <strong>Pedido:</strong> #{editItem.pedido_id}
                </p>
                <p>
                  <strong>Forma:</strong> {editItem.forma}
                </p>
                <p>
                  <strong>Valor:</strong>{' '}
                  {(editItem.valor ?? 0).toLocaleString('pt-BR', {
                    style: 'currency',
                    currency: 'BRL',
                  })}
                </p>
                <p>
                  <strong>Status Atual:</strong> {editItem.status}
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Novo Status
                </label>
                <select
                  id="newStatus"
                  className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
                >
                  <option value="pendente">Pendente</option>
                  <option value="pago">Pago</option>
                  <option value="cancelado">Cancelado</option>
                  <option value="vencido">Vencido</option>
                </select>
              </div>

              <div className="flex gap-3">
                <button
                  onClick={() => setEditItem(null)}
                  className="flex-1 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium py-2 px-4 rounded-md transition-colors"
                >
                  Cancelar
                </button>
                <button
                  onClick={() => {
                    const sel = document.getElementById('newStatus') as HTMLSelectElement
                    handleUpdateStatus(editItem, sel.value)
                  }}
                  className="flex-1 bg-primary-600 hover:bg-primary-700 text-white font-medium py-2 px-4 rounded-md transition-colors"
                >
                  Salvar
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
