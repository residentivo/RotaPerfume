"use client";

import { useEffect, useState } from "react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { PedidoModal } from "@/components/admin/PedidoModal";
import {
  apiListPedidos,
  apiGetPedido,
  apiCreatePedido,
  apiUpdatePedido,
} from "@/lib/api";
import { Pedido, PedidoDetalhe, PedidoInput } from "@/lib/types";

type SortKey =
  | "id"
  | "pedido_id_origem"
  | "cliente_nome"
  | "vendedor_nome"
  | "data_pedido"
  | "canal"
  | "status"
  | "valor_total";
type SortDir = "asc" | "desc";

const STATUS_OPTIONS = [
  { value: "", label: "Todos os status" },
  { value: "Em separação", label: "Em separação" },
  { value: "Faturado", label: "Faturado" },
  { value: "Entregue", label: "Entregue" },
  { value: "Cancelado", label: "Cancelado" },
];

const CANAL_OPTIONS = [
  { value: "", label: "Todos os canais" },
  { value: "App", label: "App" },
  { value: "Telefone", label: "Telefone" },
  { value: "Visita", label: "Visita" },
  { value: "WhatsApp", label: "WhatsApp" },
];

const LIMIT_OPTIONS = [
  { value: "10", label: "10 por pagina" },
  { value: "20", label: "20 por pagina" },
  { value: "50", label: "50 por pagina" },
  { value: "100", label: "100 por pagina" },
];

const currencyFmt = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
});

function fmtValor(v: number): string {
  return currencyFmt.format(v ?? 0);
}

function fmtDate(dateStr: string): string {
  if (!dateStr) return "-";
  const d = new Date(dateStr.length <= 10 ? `${dateStr}T00:00:00` : dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  return d.toLocaleDateString("pt-BR");
}

// Mapeia a sortKey interna do frontend para o campo aceito pelo backend em
// order_by. `pedido_id_origem` nao esta na whitelist do backend, entao nao
// enviamos order_by nesse caso (cai no default do backend: id desc).
const ORDER_BY_MAP: Partial<Record<SortKey, string>> = {
  id: "id",
  cliente_nome: "cliente_nome",
  vendedor_nome: "vendedor_nome",
  data_pedido: "data_pedido",
  canal: "canal",
  status: "status",
  valor_total: "valor_total",
};

const statusColor: Record<string, string> = {
  Faturado: "bg-green-100 text-green-700",
  Entregue: "bg-blue-100 text-blue-700",
  "Em separação": "bg-yellow-100 text-yellow-700",
  Cancelado: "bg-red-100 text-red-700",
};

export default function PedidosPage() {
  const [pedidos, setPedidos] = useState<Pedido[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [canalFilter, setCanalFilter] = useState("");
  const [dataInicio, setDataInicio] = useState("");
  const [dataFim, setDataFim] = useState("");

  const [sortKey, setSortKey] = useState<SortKey>("id");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  // Master-detail: pedido selecionado (itens exibidos abaixo da tabela).
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [selectedDetalhe, setSelectedDetalhe] = useState<PedidoDetalhe | null>(
    null
  );
  const [loadingItens, setLoadingItens] = useState(false);
  const [itensError, setItensError] = useState<string | null>(null);

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingPedido, setEditingPedido] = useState<PedidoDetalhe | null>(null);
  const [loadingEdit, setLoadingEdit] = useState(false);

  const loadPedidos = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await apiListPedidos(
        page,
        limit,
        {
          status: statusFilter || undefined,
          canal: canalFilter || undefined,
          data_inicio: dataInicio || undefined,
          data_fim: dataFim || undefined,
          q: search.trim() || undefined,
        },
        ORDER_BY_MAP[sortKey],
        sortDir
      );
      setPedidos(res.data);
      setTotal(res.total);
      setPages(res.pages);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao carregar pedidos. O endpoint /api/pedidos pode nao existir no backend.";
      setError(message);
      setPedidos([]);
      setTotal(0);
      setPages(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadPedidos();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual e reset para pagina 1 quando filtros mudam
  useEffect(() => {
    const timer = setTimeout(() => {
      if (page !== 1) {
        setPage(1);
      } else {
        loadPedidos();
      }
    }, 350);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search, statusFilter, canalFilter, dataInicio, dataFim]);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const loadItens = async (id: number) => {
    setLoadingItens(true);
    setItensError(null);
    try {
      const detalhe = await apiGetPedido(id);
      setSelectedDetalhe(detalhe);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao carregar itens do pedido.";
      setItensError(message);
      setSelectedDetalhe(null);
    } finally {
      setLoadingItens(false);
    }
  };

  const toggleItens = (pedido: Pedido) => {
    if (selectedId === pedido.id) {
      setSelectedId(null);
      setSelectedDetalhe(null);
      setItensError(null);
      return;
    }
    setSelectedId(pedido.id);
    setSelectedDetalhe(null);
    loadItens(pedido.id);
  };

  const openCreate = () => {
    setModalMode("create");
    setEditingPedido(null);
    setModalOpen(true);
  };

  const openEdit = async (pedido: Pedido) => {
    setError(null);
    setLoadingEdit(true);
    try {
      const detalhe = await apiGetPedido(pedido.id);
      setModalMode("edit");
      setEditingPedido(detalhe);
      setModalOpen(true);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao carregar pedido para edicao.";
      setError(message);
    } finally {
      setLoadingEdit(false);
    }
  };

  const handleModalSubmit = async (data: PedidoInput) => {
    setError(null);
    if (modalMode === "create") {
      const created = await apiCreatePedido(data);
      await loadPedidos();
      setSuccess(`Pedido #${created.id} criado com sucesso.`);
    } else if (editingPedido) {
      const updated = await apiUpdatePedido(editingPedido.id, data);
      setPedidos((prev) =>
        prev.map((p) => (p.id === updated.id ? updated : p))
      );
      if (selectedId === updated.id) {
        setSelectedDetalhe(updated);
      }
      setSuccess(`Pedido #${updated.id} atualizado com sucesso.`);
    }
    setModalOpen(false);
    setTimeout(() => setSuccess(null), 4000);
  };

  const columns: Column<Pedido>[] = [
    {
      key: "cliente_nome",
      header: "Pedido",
      sortable: true,
      render: (p) => (
        <button
          type="button"
          onClick={() => toggleItens(p)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Ver itens do pedido"
        >
          #{p.id} - {p.cliente_nome}
        </button>
      ),
    },
    {
      key: "pedido_id_origem",
      header: "ID Origem",
      width: "110px",
      align: "center",
      sortable: true,
      render: (p) => (
        <span className="text-slate-600">{p.pedido_id_origem}</span>
      ),
    },
    {
      key: "vendedor_nome",
      header: "Vendedor",
      width: "160px",
      sortable: true,
      render: (p) => <span className="text-slate-600">{p.vendedor_nome}</span>,
    },
    {
      key: "data_pedido",
      header: "Data",
      width: "120px",
      align: "center",
      sortable: true,
      render: (p) => (
        <span className="text-slate-600">{fmtDate(p.data_pedido)}</span>
      ),
    },
    {
      key: "canal",
      header: "Canal",
      width: "110px",
      align: "center",
      sortable: true,
      render: (p) => <span className="text-slate-600">{p.canal}</span>,
    },
    {
      key: "status",
      header: "Status",
      width: "140px",
      align: "center",
      sortable: true,
      render: (p) => (
        <span
          className={[
            "inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium",
            statusColor[p.status] || "bg-slate-100 text-slate-700",
          ].join(" ")}
        >
          {p.status}
        </span>
      ),
    },
    {
      key: "valor_total",
      header: "Valor Total",
      width: "140px",
      align: "right",
      sortable: true,
      render: (p) => (
        <span className="text-slate-700">{fmtValor(p.valor_total)}</span>
      ),
    },
    {
      key: "actions",
      header: "Acoes",
      width: "170px",
      align: "right",
      render: (p) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => toggleItens(p)}
            title="Ver itens do pedido"
          >
            {selectedId === p.id ? "Ocultar itens" : "Itens"}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(p)}
            disabled={loadingEdit}
            title="Editar pedido"
          >
            Editar
          </Button>
        </div>
      ),
    },
  ];

  const startItem = total === 0 ? 0 : (page - 1) * limit + 1;
  const endItem = Math.min(page * limit, total);

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Pedidos</h1>
          <p className="mt-1 text-sm text-slate-500">
            Consulte pedidos e seus itens, e gerencie novos pedidos.
          </p>
        </div>
        <Button onClick={openCreate}>+ Novo Pedido</Button>
      </div>

      {error && (
        <div className="mb-4">
          <Alert variant="error" onClose={() => setError(null)}>
            {error}
          </Alert>
        </div>
      )}

      {success && (
        <div className="mb-4">
          <Alert variant="success" onClose={() => setSuccess(null)}>
            {success}
          </Alert>
        </div>
      )}

      <Card padded={false}>
        <div className="border-b border-slate-200 p-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
            <div className="flex flex-1 flex-col gap-3 sm:flex-row sm:items-end sm:flex-wrap">
              <div className="flex-1 sm:max-w-xs">
                <Input
                  label="Buscar"
                  placeholder="Razao social do cliente..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  icon={
                    <svg
                      className="h-5 w-5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M21 21l-4.35-4.35M11 19a8 8 0 100-16 8 8 0 000 16z"
                      />
                    </svg>
                  }
                />
              </div>
              <div className="sm:w-48">
                <Select
                  label="Status"
                  options={STATUS_OPTIONS}
                  value={statusFilter}
                  onChange={(e) => setStatusFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-40">
                <Select
                  label="Canal"
                  options={CANAL_OPTIONS}
                  value={canalFilter}
                  onChange={(e) => setCanalFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-40">
                <Input
                  label="De"
                  type="date"
                  value={dataInicio}
                  onChange={(e) => setDataInicio(e.target.value)}
                />
              </div>
              <div className="sm:w-40">
                <Input
                  label="Ate"
                  type="date"
                  value={dataFim}
                  onChange={(e) => setDataFim(e.target.value)}
                />
              </div>
              <div className="sm:w-44">
                <Select
                  label="Itens por pagina"
                  options={LIMIT_OPTIONS}
                  value={String(limit)}
                  onChange={(e) => setLimit(Number(e.target.value))}
                />
              </div>
            </div>
            <div className="text-sm text-slate-500">
              {total === 0
                ? "0 pedidos"
                : `${startItem}-${endItem} de ${total} ${
                    total === 1 ? "pedido" : "pedidos"
                  }`}
            </div>
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={pedidos}
            keyExtractor={(p) => p.id}
            loading={loading}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              search || statusFilter || canalFilter || dataInicio || dataFim
                ? "Nenhum pedido encontrado para os filtros aplicados."
                : "Nenhum pedido cadastrado."
            }
          />
        </div>

        {pages > 1 && (
          <div className="flex flex-col items-center justify-between gap-3 border-t border-slate-200 px-4 py-3 sm:flex-row">
            <div className="text-sm text-slate-500">
              Pagina <strong>{page}</strong> de <strong>{pages}</strong>
            </div>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="secondary"
                disabled={page <= 1}
                onClick={() => setPage(1)}
                title="Primeira pagina"
              >
                {"<<"}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                title="Pagina anterior"
              >
                {"<"}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={page >= pages}
                onClick={() => setPage((p) => Math.min(pages, p + 1))}
                title="Proxima pagina"
              >
                {">"}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={page >= pages}
                onClick={() => setPage(pages)}
                title="Ultima pagina"
              >
                {">>"}
              </Button>
            </div>
          </div>
        )}
      </Card>

      {selectedId !== null && (
        <Card className="mt-4">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-slate-800">
              Itens do pedido #{selectedId}
            </h2>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setSelectedId(null);
                setSelectedDetalhe(null);
              }}
            >
              Fechar
            </Button>
          </div>

          {itensError && <Alert variant="error">{itensError}</Alert>}

          {loadingItens && (
            <div className="flex items-center justify-center py-8">
              <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-200 border-t-primary-600" />
            </div>
          )}

          {!loadingItens && selectedDetalhe && (
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
                    {selectedDetalhe.itens.length === 0 && (
                      <tr>
                        <td
                          colSpan={5}
                          className="px-4 py-6 text-center text-sm text-slate-500"
                        >
                          Este pedido nao possui itens.
                        </td>
                      </tr>
                    )}
                    {selectedDetalhe.itens.map((item) => (
                      <tr key={item.id} className="hover:bg-slate-50">
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
                Total do pedido: {fmtValor(selectedDetalhe.valor_total)}
              </div>
            </div>
          )}
        </Card>
      )}

      <div className="mt-4 text-xs text-slate-400">
        <strong>Nota:</strong> A busca e os filtros de status/canal/periodo sao
        aplicados via API. Caso a lista esteja vazia ou retorne erro, verifique
        se os endpoints <code>GET /api/pedidos</code> e{" "}
        <code>GET /api/pedidos/&#123;id&#125;</code> estao implementados no
        backend.
      </div>

      <PedidoModal
        open={modalOpen}
        mode={modalMode}
        pedido={editingPedido}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}
