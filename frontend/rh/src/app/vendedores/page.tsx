'use client'

import { useEffect, useState } from 'react'
import PrivateLayout from '../layout-privado/layout'
import DataTable, { Column } from '@/components/DataTable'
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

const VENDEDOR_VAZIO: Omit<Vendedor, 'id'> = {
  nome: '',
  regiao: '',
  uf: '',
  dataAdmissao: '',
  meta: 0,
  dataDesligamento: null,
}

function VendedoresContent() {
  const [vendedores, setVendedores] = useState<Vendedor[]>([])
  const [loading, setLoading] = useState(true)
  const [modalAberto, setModalAberto] = useState(false)
  const [editando, setEditando] = useState<Vendedor | null>(null)
  const [form, setForm] = useState<Omit<Vendedor, 'id'>>(VENDEDOR_VAZIO)
  const [filtroNome, setFiltroNome] = useState('')
  const [filtroUf, setFiltroUf] = useState('')
  const [erro, setErro] = useState('')
  const [salvando, setSalvando] = useState(false)

  useEffect(() => {
    carregarVendedores()
  }, [])

  async function carregarVendedores() {
    setLoading(true)
    try {
      const data = await apiFetch('/api/vendedores')
      setVendedores(Array.isArray(data) ? data : [])
    } catch (error: any) {
      console.error('Erro ao carregar vendedores:', error)
      setErro(error.message)
    } finally {
      setLoading(false)
    }
  }

  function abrirCriar() {
    setEditando(null)
    setForm(VENDEDOR_VAZIO)
    setErro('')
    setModalAberto(true)
  }

  function abrirEditar(vendedor: Vendedor) {
    setEditando(vendedor)
    setForm({
      nome: vendedor.nome,
      regiao: vendedor.regiao,
      uf: vendedor.uf,
      dataAdmissao: vendedor.dataAdmissao ? vendedor.dataAdmissao.split('T')[0] : '',
      meta: vendedor.meta,
      dataDesligamento: vendedor.dataDesligamento,
    })
    setErro('')
    setModalAberto(true)
  }

  async function salvar(e: React.FormEvent) {
    e.preventDefault()
    setSalvando(true)
    setErro('')

    try {
      if (editando) {
        await apiFetch(`/api/vendedores/${editando.id}`, {
          method: 'PUT',
          body: JSON.stringify(form),
        })
      } else {
        await apiFetch('/api/vendedores', {
          method: 'POST',
          body: JSON.stringify(form),
        })
      }
      setModalAberto(false)
      carregarVendedores()
    } catch (error: any) {
      setErro(error.message)
    } finally {
      setSalvando(false)
    }
  }

  const vendedoresFiltrados = vendedores.filter((v) => {
    const matchNome = v.nome.toLowerCase().includes(filtroNome.toLowerCase())
    const matchUf = filtroUf === '' || v.uf.toLowerCase() === filtroUf.toLowerCase()
    return matchNome && matchUf
  })

  const columns: Column<Vendedor>[] = [
    { key: 'nome', label: 'Nome' },
    { key: 'regiao', label: 'Região' },
    { key: 'uf', label: 'UF' },
    {
      key: 'dataAdmissao',
      label: 'Admissão',
      render: (row) => (row.dataAdmissao ? new Date(row.dataAdmissao).toLocaleDateString('pt-BR') : '-'),
    },
    {
      key: 'meta',
      label: 'Meta',
      render: (row) => row.meta.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' }),
    },
    {
      key: 'status',
      label: 'Status',
      render: (row) => (
        <span
          className={`px-2 py-1 rounded-full text-xs font-medium ${
            !row.dataDesligamento ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
          }`}
        >
          {!row.dataDesligamento ? 'Ativo' : 'Inativo'}
        </span>
      ),
    },
  ]

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-3xl font-bold text-gray-800">Vendedores</h1>
        <button
          onClick={abrirCriar}
          className="bg-blue-600 hover:bg-blue-700 text-white font-semibold py-2 px-4 rounded-lg transition"
        >
          + Novo Vendedor
        </button>
      </div>

      <div className="bg-white p-4 rounded-lg shadow mb-4 flex gap-4">
        <input
          type="text"
          placeholder="Filtrar por nome..."
          value={filtroNome}
          onChange={(e) => setFiltroNome(e.target.value)}
          className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
        />
        <input
          type="text"
          placeholder="Filtrar por UF..."
          value={filtroUf}
          onChange={(e) => setFiltroUf(e.target.value.toUpperCase())}
          maxLength={2}
          className="w-32 px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
        />
      </div>

      {erro && !modalAberto && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg mb-4">
          {erro}
        </div>
      )}

      <DataTable
        columns={columns}
        data={vendedoresFiltrados}
        loading={loading}
        keyExtractor={(row) => row.id}
        actions={(row) => (
          <button
            onClick={() => abrirEditar(row)}
            className="text-blue-600 hover:text-blue-800 font-medium"
          >
            Editar
          </button>
        )}
      />

      {modalAberto && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4">
              {editando ? 'Editar Vendedor' : 'Novo Vendedor'}
            </h2>
            <form onSubmit={salvar} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Nome</label>
                <input
                  type="text"
                  required
                  value={form.nome}
                  onChange={(e) => setForm({ ...form, nome: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Região</label>
                  <input
                    type="text"
                    required
                    value={form.regiao}
                    onChange={(e) => setForm({ ...form, regiao: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">UF</label>
                  <input
                    type="text"
                    required
                    maxLength={2}
                    value={form.uf}
                    onChange={(e) => setForm({ ...form, uf: e.target.value.toUpperCase() })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Data Admissão
                </label>
                <input
                  type="date"
                  required
                  value={form.dataAdmissao}
                  onChange={(e) => setForm({ ...form, dataAdmissao: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Meta</label>
                <input
                  type="number"
                  required
                  step="0.01"
                  value={form.meta}
                  onChange={(e) => setForm({ ...form, meta: parseFloat(e.target.value) || 0 })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Data Desligamento
                </label>
                <input
                  type="date"
                  value={form.dataDesligamento ? form.dataDesligamento.split('T')[0] : ''}
                  onChange={(e) =>
                    setForm({ ...form, dataDesligamento: e.target.value || null })
                  }
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>

              {erro && (
                <div className="bg-red-50 border border-red-200 text-red-700 px-3 py-2 rounded-lg text-sm">
                  {erro}
                </div>
              )}

              <div className="flex gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setModalAberto(false)}
                  className="flex-1 bg-gray-200 hover:bg-gray-300 text-gray-800 font-semibold py-2 px-4 rounded-lg"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  disabled={salvando}
                  className="flex-1 bg-blue-600 hover:bg-blue-700 text-white font-semibold py-2 px-4 rounded-lg disabled:opacity-50"
                >
                  {salvando ? 'Salvando...' : 'Salvar'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

export default function VendedoresPage() {
  return (
    <PrivateLayout>
      <VendedoresContent />
    </PrivateLayout>
  )
}
