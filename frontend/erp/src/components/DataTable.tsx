'use client'

import { ReactNode } from 'react'

export interface Column<T> {
  key: string
  label: string
  render?: (row: T) => ReactNode
  className?: string
}

interface DataTableProps<T> {
  columns: Column<T>[]
  rows: T[]
  loading?: boolean
  emptyMessage?: string
  onRowClick?: (row: T) => void
  keyExtractor?: (row: T, idx: number) => string | number
}

export default function DataTable<T extends Record<string, any>>({
  columns,
  rows,
  loading,
  emptyMessage = 'Nenhum registro encontrado',
  onRowClick,
  keyExtractor,
}: DataTableProps<T>) {
  if (loading) {
    return (
      <div className="bg-white rounded-md border border-gray-200 p-8 text-center text-gray-500">
        Carregando...
      </div>
    )
  }

  if (!rows || rows.length === 0) {
    return (
      <div className="bg-white rounded-md border border-gray-200 p-8 text-center text-gray-500">
        {emptyMessage}
      </div>
    )
  }

  return (
    <div className="bg-white rounded-md border border-gray-200 overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              {columns.map((c) => (
                <th
                  key={c.key}
                  className={`px-4 py-3 text-left text-xs font-semibold text-gray-600 uppercase tracking-wider ${c.className || ''}`}
                >
                  {c.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {rows.map((row, idx) => {
              const key = keyExtractor ? keyExtractor(row, idx) : idx
              return (
                <tr
                  key={key}
                  onClick={onRowClick ? () => onRowClick(row) : undefined}
                  className={onRowClick ? 'hover:bg-gray-50 cursor-pointer' : 'hover:bg-gray-50'}
                >
                  {columns.map((c) => (
                    <td
                      key={c.key}
                      className={`px-4 py-3 text-gray-700 ${c.className || ''}`}
                    >
                      {c.render ? c.render(row) : (row[c.key] ?? '-')}
                    </td>
                  ))}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
