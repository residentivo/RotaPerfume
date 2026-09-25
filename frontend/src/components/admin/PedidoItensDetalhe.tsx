"use client";

import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { PedidoDetalhe } from "@/lib/types";

const currencyFmt = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
});

function fmtValor(v: number): string {
  return currencyFmt.format(v ?? 0);
}

interface PedidoItensDetalheProps {
  pedidoId: number;
  detalhe: PedidoDetalhe | null;
  loading: boolean;
  error: string | null;
  onClose: () => void;
}

/**
 * Detalhe (accordion) dos itens de um pedido: titulo, botao fechar,
 * loading, erro e sub-tabela de itens com total. Renderizado logo abaixo da
 * linha do pedido na tela de Pedidos.
 */
export function PedidoItensDetalhe({
  pedidoId,
  detalhe,
  loading,
  error,
  onClose,
}: PedidoItensDetalheProps) {
  return (
    <div
      className="rounded-md border border-slate-200 bg-white p-4 shadow-sm"
      data-testid={`pedido-itens-${pedidoId}`}
    >
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-slate-800">
          Itens do pedido #{pedidoId}
        </h2>
        <Button size="sm" variant="ghost" onClick={onClose}>
          Fechar
        </Button>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      {loading && (
        <div className="flex items-center justify-center py-8">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-200 border-t-primary-600" />
        </div>
      )}

      {!loading && detalhe && (
        <div>
          <div className="max-h-80 overflow-y-auto overflow-x-auto rounded-md border border-slate-200">
            <table className="min-w-full divide-y divide-slate-200">
              <thead className="sticky top-0 z-10 bg-slate-50">
                <tr>
                  <th className="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wider text-slate-600">
                    Produto
                  </th>
                  <th className="px-4 py-2 text-right text-xs font-semibold uppercase tracking-wider text-slate-600">
                    Qtd
                  </th>
                  <th className="px-4 py-2 text-right text-xs font-semibold uppercase tracking-wider text-slate-600">
                    Preco praticado
                  </th>
                  <th className="px-4 py-2 text-right text-xs font-semibold uppercase tracking-wider text-slate-600">
                    Desconto
                  </th>
                  <th className="px-4 py-2 text-right text-xs font-semibold uppercase tracking-wider text-slate-600">
                    Valor bruto
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 bg-white">
                {detalhe.itens.length === 0 && (
                  <tr>
                    <td
                      colSpan={5}
                      className="px-4 py-6 text-center text-sm text-slate-500"
                    >
                      Este pedido nao possui itens.
                    </td>
                  </tr>
                )}
                {detalhe.itens.map((item) => (
                  <tr key={item.item_id_origem} className="hover:bg-slate-50">
                    <td className="px-4 py-2 text-sm text-slate-700">
                      #{item.produto_id} - {item.produto_descricao}{" "}
                      <span className="text-xs text-slate-400">
                        ({item.produto_sku})
                      </span>
                    </td>
                    <td className="px-4 py-2 text-right text-sm text-slate-700">
                      {item.quantidade}
                    </td>
                    <td className="px-4 py-2 text-right text-sm text-slate-700">
                      {fmtValor(item.preco_praticado)}
                    </td>
                    <td className="px-4 py-2 text-right text-sm text-slate-700">
                      {item.desconto_pct}%
                    </td>
                    <td className="px-4 py-2 text-right text-sm font-medium text-slate-800">
                      {fmtValor(item.valor_bruto)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="mt-3 flex justify-end text-sm font-semibold text-slate-800">
            Total do pedido: {fmtValor(detalhe.valor_total)}
          </div>
        </div>
      )}
    </div>
  );
}
