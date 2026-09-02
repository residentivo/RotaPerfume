'use client'

import { useEffect, useState } from 'react'
import PrivateLayout from '../layout-privado/layout'
import apiFetch from '@/lib/api'

interface Vendedor {
  id: number
  nome: string
  regiao: string
  uf: string
  dataAdmissao: string
  meta: number
  dataDesligamento: string | null
}

interface Usuario {
  id: number
  login: string
  email: string
  tipoUsuario: string
  ativo: boolean
  vendedorId: number | null
}

function DashboardContent() {
  const [vendedores, setVendedores] = useState<Vendedor[]>([])
  const [usuarios, setUsuarios] = useState<Usuario[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function carregarDados() {
      try {
        const [v, u] = await Promise.all([
          apiFetch('/api/vendedores'),
          apiFetch('/api/usuarios'),
        ])
        setVendedores(Array.isArray(v) ? v : [])
        setUsuarios(Array.isArray(u) ? u : [])
      } catch (error) {
        console.error('Erro ao carregar dados:', error)
      } finally {
        setLoading(false)
      }
    }
    carregarDados()
  }, [])

  const totalVendedores = vendedores.length
  const vendedoresAtivos = vendedores.filter((v) => !v.dataDesligamento).length
  const vendedoresInativos = vendedores.filter((v) => v.dataDesligamento).length

  const totalUsuarios = usuarios.length
  const usuariosAtivos = usuarios.filter((u) => u.ativo).length

  const usuariosPorTipo = {
    rh: usuarios.filter((u) => u.tipoUsuario === 'rh').length,
    gerente: usuarios.filter((u) => u.tipoUsuario === 'gerente').length,
    vendedor: usuarios.filter((u) => u.tipoUsuario === 'vendedor').length,
  }

  return (
    <div>
      <h1 className="text-3xl font-bold text-gray-800 mb-6">Dashboard</h1>

      {loading ? (
        <div className="flex justify-center items-center py-12">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
            <div className="bg-white p-6 rounded-lg shadow">
              <p className="text-gray-500 text-sm">Total Vendedores</p>
              <p className="text-3xl font-bold text-blue-600 mt-2">{totalVendedores}</p>
            </div>
            <div className="bg-white p-6 rounded-lg shadow">
              <p className="text-gray-500 text-sm">Vendedores Ativos</p>
              <p className="text-3xl font-bold text-green-600 mt-2">{vendedoresAtivos}</p>
            </div>
            <div className="bg-white p-6 rounded-lg shadow">
              <p className="text-gray-500 text-sm">Vendedores Inativos</p>
              <p className="text-3xl font-bold text-red-600 mt-2">{vendedoresInativos}</p>
            </div>
            <div className="bg-white p-6 rounded-lg shadow">
              <p className="text-gray-500 text-sm">Usuários Ativos</p>
              <p className="text-3xl font-bold text-purple-600 mt-2">{usuariosAtivos}</p>
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <div className="bg-white p-6 rounded-lg shadow">
              <h2 className="text-lg font-semibold text-gray-800 mb-4">Usuários por Tipo</h2>
              <div className="space-y-3">
                <div className="flex justify-between items-center">
                  <span className="text-gray-600">RH</span>
                  <span className="font-semibold text-gray-800">{usuariosPorTipo.rh}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-gray-600">Gerente</span>
                  <span className="font-semibold text-gray-800">{usuariosPorTipo.gerente}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-gray-600">Vendedor</span>
                  <span className="font-semibold text-gray-800">{usuariosPorTipo.vendedor}</span>
                </div>
              </div>
            </div>

            <div className="bg-white p-6 rounded-lg shadow">
              <h2 className="text-lg font-semibold text-gray-800 mb-4">Resumo Geral</h2>
              <div className="space-y-3">
                <div className="flex justify-between items-center">
                  <span className="text-gray-600">Total de Usuários</span>
                  <span className="font-semibold text-gray-800">{totalUsuarios}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-gray-600">Total de Vendedores</span>
                  <span className="font-semibold text-gray-800">{totalVendedores}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-gray-600">Taxa de Atividade</span>
                  <span className="font-semibold text-gray-800">
                    {totalVendedores > 0
                      ? Math.round((vendedoresAtivos / totalVendedores) * 100)
                      : 0}
                    %
                  </span>
                </div>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  )
}

export default function DashboardPage() {
  return (
    <PrivateLayout>
      <DashboardContent />
    </PrivateLayout>
  )
}
