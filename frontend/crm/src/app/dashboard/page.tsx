'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth'
import apiFetch from '@/lib/api'
import Navbar from '@/components/Navbar'
import Sidebar from '@/components/Sidebar'

interface RankingVendedor {
  vendedor_id: number
  vendedor_nome: string
  total_vendas: number
  total_valor: number
}

interface DistribuicaoCarteira {
  segmento: string
  quantidade: number
  percentual: number
}

interface Totais {
  total_clientes: number
  total_visitas: number
  total_oportunidades: number
}

export default function DashboardPage() {
  const { token, isLoading: authLoading } = useAuth()
  const router = require('next/navigation').useRouter()
  const [ranking, setRanking] = useState<RankingVendedor[]>([])
  const [distribuicao, setDistribuicao] = useState<DistribuicaoCarteira[]>([])
  const [totais, setTotais] = useState<Totais | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!authLoading && !token) {
      router.push('/login')
      return
    }

    if (token) {
      loadDashboard()
    }
  }, [token, authLoading, router])

  const loadDashboard = async () => {
    try {
      setLoading(true)
      const [rankingData, distribuicaoData, totaisData] = await Promise.all([
        apiFetch('/api/dashboard/vendas').catch(() => []),
        apiFetch('/api/dashboard/carteira').catch(() => []),
        apiFetch('/api/dashboard/totais').catch(() => ({ total_clientes: 0, total_visitas: 0, total_oportunidades: 0 })),
      ])
      setRanking(rankingData)
      setDistribuicao(distribuicaoData)
      setTotais(totaisData)
    } catch (err: any) {
      setError(err.message || 'Erro ao carregar dashboard')
    } finally {
      setLoading(false)
    }
  }

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
        <Navbar title="Dashboard" />
        <main className="p-6">
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-6">
              {error}
            </div>
          )}

          {loading ? (
            <div className="flex justify-center items-center py-12">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
            </div>
          ) : (
            <>
              {/* Cards de Totais */}
              <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
                <div className="bg-white rounded-lg shadow p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm text-gray-600">Total de Clientes</p>
                      <p className="text-3xl font-bold text-gray-900 mt-1">
                        {totais?.total_clientes || 0}
                      </p>
                    </div>
                    <div className="text-4xl">🏢</div>
                  </div>
                </div>

                <div className="bg-white rounded-lg shadow p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm text-gray-600">Total de Visitas</p>
                      <p className="text-3xl font-bold text-gray-900 mt-1">
                        {totais?.total_visitas || 0}
                      </p>
                    </div>
                    <div className="text-4xl">📅</div>
                  </div>
                </div>

                <div className="bg-white rounded-lg shadow p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm text-gray-600">Total de Oportunidades</p>
                      <p className="text-3xl font-bold text-gray-900 mt-1">
                        {totais?.total_oportunidades || 0}
                      </p>
                    </div>
                    <div className="text-4xl">💰</div>
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* Ranking de Vendedores */}
                <div className="bg-white rounded-lg shadow">
                  <div className="px-6 py-4 border-b border-gray-200">
                    <h3 className="text-lg font-semibold text-gray-900">Ranking de Vendedores</h3>
                  </div>
                  <div className="p-6">
                    {ranking && ranking.length > 0 ? (
                      <div className="space-y-4">
                        {ranking.map((item, index) => (
                          <div key={item.vendedor_id} className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                              <span className={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold ${
                                index === 0 ? 'bg-yellow-100 text-yellow-700' :
                                index === 1 ? 'bg-gray-200 text-gray-700' :
                                index === 2 ? 'bg-orange-100 text-orange-700' :
                                'bg-gray-100 text-gray-600'
                              }`}>
                                {index + 1}
                              </span>
                              <span className="font-medium text-gray-900">{item.vendedor_nome}</span>
                            </div>
                            <div className="text-right">
                              <p className="font-semibold text-gray-900">{item.total_vendas} vendas</p>
                              <p className="text-sm text-gray-500">
                                R$ {item.total_valor?.toLocaleString('pt-BR') || 0}
                              </p>
                            </div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500 text-center py-4">Nenhum dado disponível</p>
                    )}
                  </div>
                </div>

                {/* Distribuição de Carteira */}
                <div className="bg-white rounded-lg shadow">
                  <div className="px-6 py-4 border-b border-gray-200">
                    <h3 className="text-lg font-semibold text-gray-900">Distribuição por Segmento</h3>
                  </div>
                  <div className="p-6">
                    {distribuicao && distribuicao.length > 0 ? (
                      <div className="space-y-4">
                        {distribuicao.map((item) => (
                          <div key={item.segmento}>
                            <div className="flex justify-between text-sm mb-1">
                              <span className="font-medium text-gray-700">{item.segmento}</span>
                              <span className="text-gray-500">{item.percentual}%</span>
                            </div>
                            <div className="w-full bg-gray-200 rounded-full h-2">
                              <div
                                className="bg-primary-600 h-2 rounded-full"
                                style={{ width: `${item.percentual}%` }}
                              ></div>
                            </div>
                            <p className="text-xs text-gray-500 mt-1">{item.quantidade} clientes</p>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500 text-center py-4">Nenhum dado disponível</p>
                    )}
                  </div>
                </div>
              </div>
            </>
          )}
        </main>
      </div>
    </div>
  )
}
