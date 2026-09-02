'use client'

import { useEffect, useState } from 'react'
import Sidebar from '@/components/Sidebar'
import Navbar from '@/components/Navbar'
import DataTable, { Column } from '@/components/DataTable'
import apiFetch from '@/lib/api'

interface Produto {
  id?: number
  sku?: string
  descricao?: string
  categoria?: string
  marca?: string
  preco_tabela?: number
  preco?: number
  ativo?: boolean
}

interface FormData {
  id?: number
  sku: string
  descricao: string
  categoria: string
  marca: string
  preco_tabela: string
  ativo: boolean
}

export default function ProdutosPage() {
  const [produtos, setProdutos] = useState<Produto[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showModal, setShowModal] = useState(false)
  const [editItem, setEditItem] = useState<Produto | null>(null)
  const [form, setForm] = useState<FormData>({
    sku: '',
    descricao: '',
    categoria: '',
    marca: '',
    preco_tabela: '',
    ativo: true,
  })

  const [filters, setFilters] = useState({
    categoria: '',
    marca: '',
    ativo: '',
  })

  useEffect(() => {
    loadProdutos()
  }, [])

  async function loadProdutos() {
    setLoading(true)
    setError('')
    try {
      const data = await apiFetch('/api/produtos')
      const list: Produto[] = Array.isArray(data) ? data : data.produtos ?? data.data ?? []
      setProdutos(list)
    } catch (err: any) {
      setError(err.message || 'Erro ao carregar produtos')
    } finally {
      setLoading(false)
    }
  }

  const filtered = produtos.filter((p) => {
    if (filters.categoria && p.categoria !== filters.categoria) return false
    if (filters.marca && p.marca !== filters.marca) return false
    if (filters.ativo === 'true' && !p.ativo) return false
    if (filters.ativo === 'false' && p.ativo) return false
    return true
  })

  const categorias = [...new Set(produtos.map((p) => p.categoria).filter(Boolean))] as string[]
  const marcas = [...new Set(produtos.map((p) => p.marca).filter(Boolean))] as string[]

  function openCreate() {
    setEditItem(null)
    setForm({ sku: '', descricao: '', categoria: '', marca: '', preco_tabela: '', ativo: true })
    setShowModal(true)
  }

  function openEdit(p: Produto) {
    setEditItem(p)
    setForm({
      id: p.id,
      sku: p.sku ?? '',
      descricao: p.descricao ?? '',
      categoria: p.categoria ?? '',
      marca: p.marca ?? '',
      preco_tabela: String(p.preco_tabela ?? p.preco ?? ''),
      ativo: p.ativo ?? true,
    })
    setShowModal(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      const payload = {
        ...form,
        preco_tabela: parseFloat(form.preco_tabela) || 0,
      }
      if (editItem) {
        await apiFetch(`/api/produtos/${editItem.id}`, { method: 'PUT', body: JSON.stringify(payload) })
      } else {
        await apiFetch('/api/produtos', { method: 'POST', body: JSON.stringify(payload) })
      }
      setShowModal(false)
      loadProdutos()
    } catch (err: any) {
      alert(err.message)
    }
  }

  const columns: Column<Produto>[] = [
    { key: 'sku', label: 'SKU' },
    { key: 'descricao', label: 'Descricao' },
    { key: 'categoria', label: 'Categoria' },
    { key: 'marca', label: 'Marca' },
    {
      key: 'preco_tabela',
      label: 'Preco Tabela',
      render: (row) =>
        (row.preco_tabela ?? row.preco ?? 0).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' }),
    },
    {
      key: 'ativo',
      label: 'Ativo',
      render: (row) => (
        <span
          className={`px-2 py-1 rounded text-xs font-medium ${
            row.ativo ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'
          }`}
        >
          {row.ativo ? 'Sim' : 'Nao'}
        </span>
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
            <h2 className="text-lg font-semibold text-gray-800">Produtos</h2>
            <button
              onClick={openCreate}
              className="bg-primary-600 hover:bg-primary-700 text-white text-sm font-medium py-2 px-4 rounded-md transition-colors"
            >
              + Novo Produto
            </button>
          </div>

          <div className="flex gap-4 flex-wrap">
            <select
              value={filters.categoria}
              onChange={(e) => setFilters({ ...filters, categoria: e.target.value })}
              className="border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
            >
              <option value="">Todas Categorias</option>
              {categorias.map((c) => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
            <select
              value={filters.marca}
              onChange={(e) => setFilters({ ...filters, marca: e.target.value })}
              className="border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
            >
              <option value="">Todas Marcas</option>
              {marcas.map((m) => (
                <option key={m} value={m}>{m}</option>
              ))}
            </select>
            <select
              value={filters.ativo}
              onChange={(e) => setFilters({ ...filters, ativo: e.target.value })}
              className="border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
            >
              <option value="">Todos Status</option>
              <option value="true">Ativos</option>
              <option value="false">Inativos</option>
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
            emptyMessage="Nenhum produto encontrado"
            onRowClick={openEdit}
          />
        </main>
      </div>

      {showModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-lg shadow-xl w-full max-w-md">
            <div className="px-6 py-4 border-b border-gray-200">
              <h3 className="text-lg font-semibold text-gray-800">
                {editItem ? 'Editar Produto' : 'Novo Produto'}
              </h3>
            </div>
            <form onSubmit={handleSave} className="p-6 space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">SKU</label>
                <input
                  type="text"
                  value={form.sku}
                  onChange={(e) => setForm({ ...form, sku: e.target.value })}
                  className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Descricao</label>
                <input
                  type="text"
                  value={form.descricao}
                  onChange={(e) => setForm({ ...form, descricao: e.target.value })}
                  className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
                  required
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Categoria</label>
                  <input
                    type="text"
                    value={form.categoria}
                    onChange={(e) => setForm({ ...form, categoria: e.target.value })}
                    className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Marca</label>
                  <input
                    type="text"
                    value={form.marca}
                    onChange={(e) => setForm({ ...form, marca: e.target.value })}
                    className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Preco Tabela</label>
                <input
                  type="number"
                  step="0.01"
                  value={form.preco_tabela}
                  onChange={(e) => setForm({ ...form, preco_tabela: e.target.value })}
                  className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
                  required
                />
              </div>
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="ativo"
                  checked={form.ativo}
                  onChange={(e) => setForm({ ...form, ativo: e.target.checked })}
                  className="w-4 h-4 text-primary-600 border-gray-300 rounded focus:ring-primary-500"
                />
                <label htmlFor="ativo" className="text-sm text-gray-700">Produto Ativo</label>
              </div>
              <div className="flex gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="flex-1 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium py-2 px-4 rounded-md transition-colors"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  className="flex-1 bg-primary-600 hover:bg-primary-700 text-white font-medium py-2 px-4 rounded-md transition-colors"
                >
                  Salvar
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
