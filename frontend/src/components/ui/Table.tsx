"use client";

import { Fragment, ReactNode } from "react";

export interface Column<T> {
  key: keyof T | string;
  header: string;
  render?: (row: T) => ReactNode;
  sortable?: boolean;
  sortValue?: (row: T) => string | number;
  width?: string;
  align?: "left" | "center" | "right";
}

interface TableProps<T> {
  columns: Column<T>[];
  data: T[];
  keyExtractor: (row: T) => string | number;
  emptyMessage?: string;
  loading?: boolean;
  /** Chave da coluna atualmente ordenada (habilita indicador visual no cabecalho) */
  sortKey?: keyof T | string;
  /** Direcao da ordenacao atual */
  sortDir?: "asc" | "desc";
  /** Chamado quando o usuario clica em um cabecalho `sortable` */
  onSort?: (key: keyof T | string) => void;
  /**
   * Conteudo expandido (accordion) de uma linha. Quando retorna algo diferente
   * de null/undefined/false, e renderizado em uma <tr> extra logo abaixo da
   * linha, ocupando todas as colunas.
   */
  renderExpanded?: (row: T) => ReactNode;
  /**
   * FE-09: a ultima carga da listagem falhou. Com dados (lista anterior
   * mantida pela tela) a tabela e exibida normalmente; sem dados, o estado
   * vazio NAO e exibido — a tela mostra so o alerta de erro, pois "nenhum
   * registro" seria uma afirmacao falsa.
   */
  erroCarga?: boolean;
}

export function Table<T>({
  columns,
  data,
  keyExtractor,
  emptyMessage = "Nenhum registro encontrado.",
  loading = false,
  sortKey,
  sortDir,
  onSort,
  renderExpanded,
  erroCarga = false,
}: TableProps<T>) {
  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-200 border-t-primary-600" />
      </div>
    );
  }

  if (data.length === 0) {
    if (erroCarga) return null;
    return (
      <div className="py-12 text-center text-sm text-slate-500">
        {emptyMessage}
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-slate-200">
        <thead className="bg-slate-50">
          <tr>
            {columns.map((col) => (
              <th
                key={String(col.key)}
                scope="col"
                style={{ width: col.width }}
                className={[
                  "px-4 py-3 text-xs font-semibold uppercase tracking-wider text-slate-600",
                  col.align === "center"
                    ? "text-center"
                    : col.align === "right"
                    ? "text-right"
                    : "text-left",
                ].join(" ")}
              >
                {col.sortable && onSort ? (
                  <button
                    type="button"
                    onClick={() => onSort(col.key)}
                    className="inline-flex items-center gap-1 hover:text-slate-900"
                  >
                    {col.header}
                    <span className="text-slate-400">
                      {sortKey === col.key ? (sortDir === "asc" ? "↑" : "↓") : "↕"}
                    </span>
                  </button>
                ) : (
                  col.header
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 bg-white">
          {data.map((row) => {
            const expanded = renderExpanded ? renderExpanded(row) : null;
            const hasExpanded =
              expanded !== null && expanded !== undefined && expanded !== false;
            return (
              <Fragment key={keyExtractor(row)}>
                <tr className="hover:bg-slate-50">
                  {columns.map((col) => (
                    <td
                      key={String(col.key)}
                      className={[
                        "whitespace-nowrap px-4 py-3 text-sm text-slate-700",
                        col.align === "center"
                          ? "text-center"
                          : col.align === "right"
                          ? "text-right"
                          : "text-left",
                      ].join(" ")}
                    >
                      {col.render
                        ? col.render(row)
                        : String((row as Record<string, unknown>)[col.key as string] ?? "")}
                    </td>
                  ))}
                </tr>
                {hasExpanded && (
                  <tr data-expanded-row="true" className="bg-slate-50">
                    <td colSpan={columns.length} className="px-4 py-3">
                      {expanded}
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

export function Badge({
  children,
  color = "gray",
}: {
  children: ReactNode;
  color?: "gray" | "blue" | "green" | "red" | "yellow" | "purple";
}) {
  const colorMap: Record<string, string> = {
    gray: "bg-slate-100 text-slate-700",
    blue: "bg-blue-100 text-blue-700",
    green: "bg-green-100 text-green-700",
    red: "bg-red-100 text-red-700",
    yellow: "bg-yellow-100 text-yellow-700",
    purple: "bg-purple-100 text-purple-700",
  };
  return (
    <span
      className={[
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium",
        colorMap[color] || colorMap.gray,
      ].join(" ")}
    >
      {children}
    </span>
  );
}
