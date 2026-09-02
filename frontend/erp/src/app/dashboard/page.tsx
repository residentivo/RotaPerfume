'use client'

import { useEffect, useState } from 'react'
import Sidebar from '@/components/Sidebar'
import Navbar from '@/components/Navbar'
import apiFetch from '@/lib/api'

interface DashboardEstoque {
  ruptura?: { sku?: string; descricao?: string }[]
  total_ruptura?: number
}

interface DashboardFinanceiro {
  por_forma_pagamento?: { [forma: string]: number }
  total?: number
}

interface ResumoPedidos {
  total_pedidos?: number
  total_valor?: number
  valor?: number
  total?: number
}

export default function DashboardPage() {
  const [estoque, setEstoque] = useState<DashboardEstoque | null>(null)
  const [financeiro, setFinanceiro] = useState<DashboardFinanceiro | null>(null)
  const [pedidos, setPedidos] = useState<ResumoPedidos | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    async function load() {
      setLoading(true)
      setError('')
      try {
        const [est, fin, ped] = await Promise.allSettled([
          apiFetch('/api/dashboard/estoque'),
          apiFetch('/api/dashboard/financeiro'),
          apiFetch('/api/pedidos/resumo').catch(() => apiFetch('/api/dashboard/pedidos').catch(() => null)),
        ])

        if (est.status === 'fulfilled') setEstoque(est.value as DashboardEstoque)
        if (fin.status === 'fulfilled') setFinanceiro(fin.value as DashboardFinanceiro)
        if (ped.status === 'fulfilled') setPedidos(ped.value as ResumoPedidos)
      } catch (err: any) {
        setError(err.message || 'Erro ao carregar dados')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [])

  const totalPedidos = pedidos?.total_pedidos ?? pedidos?.total ?? 0
  const valorTotal = pedidos?.total_valor ?? pedidos?.valor ?? 0
  const ruptura = estoque?.total_ruptura ?? estoque?.ruptura?.length ?? 0
  const porForma = financeiro?.por_forma_pagamento ?? {}

  if (loading) {
    return (
      <div className="flex min-h-screen bg-gray-100">
        <Sidebar />
        <div className="flex-1 flex flex-col">
          <Navbar />
          <main className="p-6">
            <div className="text-center text-gray-500 py-12">Carregando...</div>
          </main>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen bg-gray-100">
      <Sidebar />
      <div className="flex-1 flex flex-col">
        <Navbar />
        <main className="p-6 space-y-6">
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md text-sm">
              {error}
            </div>
          )}

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-gray-500">Total de Pedidos</p>
                  <p className="text-2xl font-bold text-gray-800 mt-1">{totalPedidos}</p>
                </div>
                <span className="text-3xl">🛒</span>
              </div>
            </div>

            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-gray-500">Valor Total</p>
                  <p className="text-2xl font-bold text-gray-800 mt-1">
                    {valorTotal.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })}
                  </p>
                </div>
                <span className="text-3xl">💰</span>
              </div>
            </div>

            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-gray-500">Produtos em Ruptura</p>
                  <p className="text-2xl font-bold text-gray-800 mt-1">{ruptura}</p>
                </div>
                <span className="text-3xl">⚠️</span>
              </div>
            </div>

            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-gray-500">Formas de Pagamento</p>
                  <p className="text-2xl font-bold text-gray-800 mt-1">
                    {Object.keys(porForma).length}
                  </p>
                </div>
                <span className="text-3xl">💳</span>
              </div>
            </div>
          </div>

          {Object.keys(porForma).length > 0 && (
            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
              <h2 className="text-lg font-semibold text-gray-800 mb-4">Totais por Forma de Pagamento</h2>
              <div className="space-y-3">
                {Object.entries(porForma).map(([forma, valor]) => (
                  <div key={forma} className="flex items-center justify-between">
                    <span className="text-sm text-gray-600">{forma}</span>
                    <span className="text-sm font-semibold text-gray-800">
                      {(valor as number).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {estoque?.ruptura && estoque.ruptura.length > 0 && (
            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
              <h2 className="text-lg font-semibold text-gray-800 mb-4">Produtos em Ruptura</h2>
              <div className="space-y-2">
                {estoque.ruptura.slice(0, 10).map((item, i) => (
                  <div key={i} className="flex items-center gap-2 text-sm text-gray-600">
                    <span className="text-yellow-500">⚠️</span>
                    <span className="font-medium">{item.sku}</span>
                    <span>{item.descricao}</span>
                  </div>
                ))}
                {estoque.ruptura.length > 10 && (
                  <p className="text-sm text-gray-500 mt-2">
                    E mais {estoque.ruptura.length - 10} produtos...
                  </p>
                )}
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  )
}
