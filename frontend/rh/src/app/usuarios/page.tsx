'use client'

import { useEffect, useState } from 'react'
import PrivateLayout from '../layout-privado/layout'
import DataTable, { Column } from '@/components/DataTable'
import apiFetch from '@/lib/api'

interface Usuario {
  id: number
  login: string
  email: string
  tipoUsuario: 'vendedor' | 'gerente' | 'rh'
  ativo: boolean
  vendedorId: number | null
}

interface NovoUsuario {
  login: string
  email: string
  senha: string
  tipoUsuario: 'vendedor' | 'gerente' | 'rh'
  vendedorId: number | null
}

const USUARIO_VAZIO: NovoUsuario = {
  login: '',
  email: '',
  senha: '',
  tipoUsuario: 'vendedor',
  vendedorId: null,
}

function UsuariosContent() {
  const [usuarios, setUsuarios] = useState<Usuario[]>([])
  const [loading, setLoading] = useState(true)
  const [modalAberto, setModalAberto] = useState(false)
  const [form, setForm] = useState<NovoUsuario>(USUARIO_VAZIO)
  const [erro, setErro] = useState('')
  const [salvando, setSalvando] = useState(false)
  const [senhaTemp, setSenhaTemp] = useState<string | null>(null)
  const [usuarioReset, setUsuarioReset] = useState<number | null>(null)

  useEffect(() => {
    carregarUsuarios()
  }, [])

  async function carregarUsuarios() {
    setLoading(true)
    try {
      const data = await apiFetch('/api/usuarios')
      setUsuarios(Array.isArray(data) ? data : [])
    } catch (error: any) {
      console.error('Erro ao carregar usuários:', error)
      setErro(error.message)
    } finally {
      setLoading(false)
    }
  }

  async function criar(e: React.FormEvent) {
    e.preventDefault()
    setSalvando(true)
    setErro('')

    try {
      await apiFetch('/api/usuarios', {
        method: 'POST',
        body: JSON.stringify(form),
      })
      setModalAberto(false)
      setForm(USUARIO_VAZIO)
      carregarUsuarios()
    } catch (error: any) {
      setErro(error.message)
    } finally {
      setSalvando(false)
    }
  }

  async function resetarSenha(id: number) {
    if (!confirm('Deseja resetar a senha deste usuário?')) return

    try {
      const data = await apiFetch(`/api/usuarios/${id}/reset-senha`, {
        method: 'POST',
      })
      setSenhaTemp(data.senhaTemporaria || data.senha || 'Senha resetada')
      setUsuarioReset(id)
    } catch (error: any) {
      alert('Erro ao resetar senha: ' + error.message)
    }
  }

  const columns: Column<Usuario>[] = [
    { key: 'login', label: 'Login' },
    { key: 'email', label: 'Email' },
    {
      key: 'tipoUsuario',
      label: 'Tipo',
      render: (row) => (
        <span className="px-2 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-800 capitalize">
          {row.tipoUsuario}
        </span>
      ),
    },
    {
      key: 'vendedorId',
      label: 'Vendedor ID',
      render: (row) => row.vendedorId ?? '-',
    },
    {
      key: 'ativo',
      label: 'Status',
      render: (row) => (
        <span
          className={`px-2 py-1 rounded-full text-xs font-medium ${
            row.ativo ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
          }`}
        >
          {row.ativo ? 'Ativo' : 'Inativo'}
        </span>
      ),
    },
  ]

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-3xl font-bold text-gray-800">Usuários</h1>
        <button
          onClick={() => {
            setForm(USUARIO_VAZIO)
            setErro('')
            setModalAberto(true)
          }}
          className="bg-blue-600 hover:bg-blue-700 text-white font-semibold py-2 px-4 rounded-lg transition"
        >
          + Novo Usuário
        </button>
      </div>

      {erro && !modalAberto && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg mb-4">
          {erro}
        </div>
      )}

      {senhaTemp && (
        <div className="bg-green-50 border border-green-200 text-green-800 px-4 py-3 rounded-lg mb-4 flex justify-between items-center">
          <div>
            <p className="font-semibold">Senha temporária gerada para o usuário #{usuarioReset}:</p>
            <p className="font-mono text-lg mt-1">{senhaTemp}</p>
          </div>
          <button
            onClick={() => {
              setSenhaTemp(null)
              setUsuarioReset(null)
            }}
            className="text-green-800 hover:text-green-900 font-bold text-xl"
          >
            ×
          </button>
        </div>
      )}

      <DataTable
        columns={columns}
        data={usuarios}
        loading={loading}
        keyExtractor={(row) => row.id}
        actions={(row) => (
          <button
            onClick={() => resetarSenha(row.id)}
            className="text-orange-600 hover:text-orange-800 font-medium"
          >
            Resetar Senha
          </button>
        )}
      />

      {modalAberto && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4">Novo Usuário</h2>
            <form onSubmit={criar} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Login</label>
                <input
                  type="text"
                  required
                  value={form.login}
                  onChange={(e) => setForm({ ...form, login: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
                <input
                  type="email"
                  required
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Senha</label>
                <input
                  type="password"
                  required
                  value={form.senha}
                  onChange={(e) => setForm({ ...form, senha: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Tipo</label>
                <select
                  value={form.tipoUsuario}
                  onChange={(e) =>
                    setForm({ ...form, tipoUsuario: e.target.value as 'vendedor' | 'gerente' | 'rh' })
                  }
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                >
                  <option value="vendedor">Vendedor</option>
                  <option value="gerente">Gerente</option>
                  <option value="rh">RH</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Vendedor ID (opcional)
                </label>
                <input
                  type="number"
                  value={form.vendedorId ?? ''}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      vendedorId: e.target.value ? parseInt(e.target.value) : null,
                    })
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
                  {salvando ? 'Salvando...' : 'Criar'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

export default function UsuariosPage() {
  return (
    <PrivateLayout>
      <UsuariosContent />
    </PrivateLayout>
  )
}
