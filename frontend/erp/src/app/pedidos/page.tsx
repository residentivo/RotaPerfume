'use client'

import { useEffect, useState } from 'react'
import Sidebar from '@/components/Sidebar'
import Navbar from '@/components/Navbar'
import DataTable, { Column } from '@/components/DataTable'
import apiFetch from '@/lib/api'

interface Pedido {
  id?: number
  cliente?: string
  vendedor?: string
  data?: string
  canal?: string
  status?: string
  valor_total?: number
  valor?: number
  itens?: PedidoItem[]
}

interface PedidoItem {
  sku?: string
  descricao?: string
  quantidade?: number
  preco?: number
}

export default function PedidosPage() {
  const [pedidos, setPedidos] = useState<Pedido[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selectedPedido, setSelectedPedido] = useState<Pedido | null>(null)
  const [filters, setFilters] = useState({ status: '', vendedor_id: '' })

  useEffect(() => {
    loadPedidos()
  }, [])

  async function loadPedidos() {
    setLoading(true)
    setError('')
    try {
      const data = await apiFetch('/api/pedidos')
      const list: Pedido[] = Array.isArray(data) ? data : data.pedidos ?? data.data ?? []
      setPedidos(list)
    } catch (err: any) {
      setError(err.message || 'Erro ao carregar pedidos')
    } finally {
      setLoading(false)
    }
  }

  const statusList = [...new Set(pedidos.map((p) => p.status).filter(Boolean))] as string[]

  const filtered = pedidos.filter((p) => {
    if (filters.status && p.status !== filters.status) return false
    return true
  })

  const columns: Column<Pedido>[] = [
    { key: 'id', label: 'ID', render: (row) => <span className="font-medium">#{row.id}</span> },
    { key: 'cliente', label: 'Cliente' },
    { key: 'vendedor', label: 'Vendedor' },
    {
      key: 'data',
      label: 'Data',
      render: (row) =>
        row.data
          ? new Date(row.data).toLocaleDateString('pt-BR')
          : '-',
    },
    { key: 'canal', label: 'Canal' },
    {
      key: 'status',
      label: 'Status',
      render: (row) => {
        const colors: Record<string, string> = {
          pendente: 'bg-yellow-100 text-yellow-700',
          confirmado: 'bg-blue-100 text-blue-700',
          enviado: 'bg-purple-100 text-purple-700',
          entregue: 'bg-green-100 text-green-700',
          cancelado: 'bg-red-100 text-red-700',
        }
        const cls = colors[row.status?.toLowerCase() ?? ''] || 'bg-gray-100 text-gray-600'
        return <span className={`px-2 py-1 rounded text-xs font-medium ${cls}`}>{row.status}</span>
      },
    },
    {
      key: 'valor_total',
      label: 'Valor',
      render: (row) =>
        (row.valor_total ?? row.valor ?? 0).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' }),
    },
  ]

  return (
    <div className="flex min-h-screen bg-gray-100">
      <Sidebar />
      <div className="flex-1 flex flex-col">
        <Navbar />
        <main className="p-6 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold text-gray-800">Pedidos</h2>
            <button
              onClick={loadPedidos}
              className="bg-gray-100 hover:bg-gray-200 text-gray-700 text-sm font-medium py-2 px-4 rounded-md transition-colors"
            >
              Atualizar
            </button>
          </div>

          <div className="flex gap-4 flex-wrap">
            <select
              value={filters.status}
              onChange={(e) => setFilters({ ...filters, status: e.target.value })}
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
            emptyMessage="Nenhum pedido encontrado"
            onRowClick={(p) => setSelectedPedido(p)}
            keyExtractor={(p) => p.id ?? 0}
          />
        </main>
      </div>

      {selectedPedido && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-lg shadow-xl w-full max-w-lg">
            <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
              <h3 className="text-lg font-semibold text-gray-800">
                Pedido #{selectedPedido.id}
              </h3>
              <button
                onClick={() => setSelectedPedido(null)}
                className="text-gray-400 hover:text-gray-600"
              >
                ✕
              </button>
            </div>
            <div className="p-6 space-y-4">
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div>
                  <span className="text-gray-500">Cliente:</span>
                  <p className="font-medium text-gray-800">{selectedPedido.cliente || '-'}</p>
                </div>
                <div>
                  <span className="text-gray-500">Vendedor:</span>
                  <p className="font-medium text-gray-800">{selectedPedido.vendedor || '-'}</p>
                </div>
                <div>
                  <span className="text-gray-500">Canal:</span>
                  <p className="font-medium text-gray-800">{selectedPedido.canal || '-'}</p>
                </div>
                <div>
                  <span className="text-gray-500">Status:</span>
                  <p className="font-medium text-gray-800">{selectedPedido.status || '-'}</p>
                </div>
                <div>
                  <span className="text-gray-500">Data:</span>
                  <p className="font-medium text-gray-800">
                    {selectedPedido.data ? new Date(selectedPedido.data).toLocaleDateString('pt-BR') : '-'}
                  </p>
                </div>
                <div>
                  <span className="text-gray-500">Valor Total:</span>
                  <p className="font-medium text-gray-800">
                    {(selectedPedido.valor_total ?? selectedPedido.valor ?? 0).toLocaleString('pt-BR', {
                      style: 'currency',
                      currency: 'BRL',
                    })}
                  </p>
                </div>
              </div>

              {selectedPedido.itens && selectedPedido.itens.length > 0 && (
                <div>
                  <h4 className="text-sm font-semibold text-gray-700 mb-2">Itens</h4>
                  <div className="border border-gray-200 rounded-md overflow-hidden">
                    <table className="w-full text-sm">
                      <thead className="bg-gray-50 border-b border-gray-200">
                        <tr>
                          <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">SKU</th>
                          <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">Descricao</th>
                          <th className="px-3 py-2 text-right text-xs font-semibold text-gray-600">Qtd</th>
                          <th className="px-3 py-2 text-right text-xs font-semibold text-gray-600">Preco</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-gray-100">
                        {selectedPedido.itens.map((item, i) => (
                          <tr key={i}>
                            <td className="px-3 py-2 text-gray-700">{item.sku}</td>
                            <td className="px-3 py-2 text-gray-700">{item.descricao}</td>
                            <td className="px-3 py-2 text-gray-700 text-right">{item.quantidade}</td>
                            <td className="px-3 py-2 text-gray-700 text-right">
                              {(item.preco ?? 0).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
