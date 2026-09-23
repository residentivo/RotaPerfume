"use client";

import { useEffect, useState } from "react";
import { Navbar } from "@/components/layout/Navbar";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Column, Badge } from "@/components/ui/Table";
import { PagamentoModal } from "@/components/PagamentoModal";
import {
  apiListPagamentos,
  apiCreatePagamento,
  apiUpdatePagamento,
  apiDeletePagamento,
} from "@/lib/api";
import {
  Pagamento,
  PagamentoCreateInput,
  PagamentoUpdateInput,
} from "@/lib/types";

type SortKey =
  | "pagamento_id"
  | "pedido_id"
  | "forma_pagamento"
  | "valor"
  | "valor_liquido"
  | "data_vencimento"
  | "data_pagamento"
  | "status_pagamento";
type SortDir = "asc" | "desc";

const STATUS_FILTER_OPTIONS = [
  { value: "", label: "Todos os status" },
  { value: "Em aberto", label: "Em aberto" },
  { value: "Inadimplente", label: "Inadimplente" },
  { value: "Pago", label: "Pago" },
  { value: "Pago com atraso", label: "Pago com atraso" },
];

const FORMA_FILTER_OPTIONS = [
  { value: "", label: "Todas as formas" },
  { value: "Boleto 14 dias", label: "Boleto 14 dias" },
  { value: "Boleto 28 dias", label: "Boleto 28 dias" },
  { value: "Cartão de crédito", label: "Cartão de crédito" },
  { value: "Cartão de débito", label: "Cartão de débito" },
  { value: "Cheque a prazo", label: "Cheque a prazo" },
  { value: "Dinheiro", label: "Dinheiro" },
  { value: "PIX", label: "PIX" },
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

function fmtDate(dateStr: string | null): string {
  if (!dateStr) return "-";
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  return d.toLocaleDateString("pt-BR");
}

function statusBadgeColor(status: string): "green" | "yellow" | "red" | "gray" {
  switch (status) {
    case "Pago":
      return "green";
    case "Em aberto":
      return "yellow";
    case "Inadimplente":
    case "Pago com atraso":
      return "red";
    default:
      return "gray";
  }
}

function PagamentosContent() {
  const [pagamentos, setPagamentos] = useState<Pagamento[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [statusFilter, setStatusFilter] = useState("");
  const [formaFilter, setFormaFilter] = useState("");
  const [pedidoIdFilter, setPedidoIdFilter] = useState("");
  const [vencimentoDe, setVencimentoDe] = useState("");
  const [vencimentoAte, setVencimentoAte] = useState("");

  const [sortKey, setSortKey] = useState<SortKey>("pagamento_id");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingPagamento, setEditingPagamento] = useState<Pagamento | null>(
    null
  );
  const [deletingId, setDeletingId] = useState<number | null>(null);

  const loadPagamentos = async () => {
    setLoading(true);
    setError(null);
    try {
      const pedidoIdNum = pedidoIdFilter.trim()
        ? Number(pedidoIdFilter.trim())
        : undefined;
      const res = await apiListPagamentos(
        page,
        limit,
        {
          status_pagamento: statusFilter || undefined,
          forma_pagamento: formaFilter || undefined,
          pedido_id:
            pedidoIdNum !== undefined && !Number.isNaN(pedidoIdNum)
              ? pedidoIdNum
              : undefined,
          vencimento_de: vencimentoDe || undefined,
          vencimento_ate: vencimentoAte || undefined,
        },
        sortKey,
        sortDir
      );
      setPagamentos(res.data);
      setTotal(res.total);
      setPages(res.pages);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao carregar pagamentos. O endpoint /api/pagamentos pode nao existir no backend.";
      setError(message);
      setPagamentos([]);
      setTotal(0);
      setPages(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadPagamentos();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce dos filtros e reset para pagina 1 quando eles mudam
  useEffect(() => {
    const timer = setTimeout(() => {
      if (page !== 1) {
        setPage(1);
      } else {
        loadPagamentos();
      }
    }, 350);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter, formaFilter, pedidoIdFilter, vencimentoDe, vencimentoAte]);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const openCreate = () => {
    setModalMode("create");
    setEditingPagamento(null);
    setModalOpen(true);
  };

  const openEdit = (pagamento: Pagamento) => {
    setModalMode("edit");
    setEditingPagamento(pagamento);
    setModalOpen(true);
  };

  const handleModalSubmit = async (
    data: PagamentoCreateInput | PagamentoUpdateInput
  ) => {
    setError(null);
    if (modalMode === "create") {
      const created = await apiCreatePagamento(data as PagamentoCreateInput);
      await loadPagamentos();
      setSuccess(
        `Pagamento #${created.pagamento_id} criado com sucesso.`
      );
    } else if (editingPagamento) {
      const updated = await apiUpdatePagamento(
        editingPagamento.pagamento_id,
        data as PagamentoUpdateInput
      );
      setPagamentos((prev) =>
        prev.map((p) =>
          p.pagamento_id === updated.pagamento_id ? updated : p
        )
      );
      setSuccess(`Pagamento #${updated.pagamento_id} atualizado com sucesso.`);
    }
    setModalOpen(false);
    setTimeout(() => setSuccess(null), 4000);
  };

  const handleDelete = async (pagamento: Pagamento) => {
    const confirmed = window.confirm(
      `Tem certeza que deseja excluir o pagamento #${pagamento.pagamento_id} - ${pagamento.forma_pagamento}?`
    );
    if (!confirmed) return;

    setError(null);
    setDeletingId(pagamento.pagamento_id);
    try {
      await apiDeletePagamento(pagamento.pagamento_id);
      setPagamentos((prev) =>
        prev.filter((p) => p.pagamento_id !== pagamento.pagamento_id)
      );
      setTotal((t) => Math.max(0, t - 1));
      setSuccess(`Pagamento #${pagamento.pagamento_id} excluido com sucesso.`);
      setTimeout(() => setSuccess(null), 4000);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao excluir pagamento.";
      setError(message);
    } finally {
      setDeletingId(null);
    }
  };

  const columns: Column<Pagamento>[] = [
    {
      key: "pagamento_id",
      header: "Pagamento",
      sortable: true,
      render: (p) => (
        <button
          type="button"
          onClick={() => openEdit(p)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Editar pagamento"
        >
          #{p.pagamento_id} - {p.forma_pagamento}
        </button>
      ),
    },
    {
      key: "pedido_id",
      header: "Pedido",
      width: "100px",
      sortable: true,
      render: (p) => (
        <span className="text-slate-600">#{p.pedido_id}</span>
      ),
    },
    {
      key: "forma_pagamento",
      header: "Forma",
      width: "160px",
      sortable: true,
      render: (p) => <span className="text-slate-600">{p.forma_pagamento}</span>,
    },
    {
      key: "valor",
      header: "Valor",
      width: "130px",
      align: "right",
      sortable: true,
      render: (p) => <span className="text-slate-700">{fmtValor(p.valor)}</span>,
    },
    {
      key: "valor_liquido",
      header: "Valor liquido",
      width: "130px",
      align: "right",
      sortable: true,
      render: (p) => (
        <span className="text-slate-700">{fmtValor(p.valor_liquido)}</span>
      ),
    },
    {
      key: "data_vencimento",
      header: "Vencimento",
      width: "120px",
      sortable: true,
      render: (p) => (
        <span className="text-slate-600">{fmtDate(p.data_vencimento)}</span>
      ),
    },
    {
      key: "data_pagamento",
      header: "Pagamento em",
      width: "120px",
      sortable: true,
      render: (p) => (
        <span className="text-slate-600">{fmtDate(p.data_pagamento)}</span>
      ),
    },
    {
      key: "status_pagamento",
      header: "Status",
      width: "150px",
      align: "center",
      sortable: true,
      render: (p) => (
        <Badge color={statusBadgeColor(p.status_pagamento)}>
          {p.status_pagamento}
        </Badge>
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
            onClick={() => openEdit(p)}
            title="Editar pagamento"
          >
            Editar
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => handleDelete(p)}
            disabled={deletingId === p.pagamento_id}
            title="Excluir pagamento"
          >
            {deletingId === p.pagamento_id ? "Excluindo..." : "Excluir"}
          </Button>
        </div>
      ),
    },
  ];

  const startItem = total === 0 ? 0 : (page - 1) * limit + 1;
  const endItem = Math.min(page * limit, total);
  const hasFilters = Boolean(
    statusFilter || formaFilter || pedidoIdFilter || vencimentoDe || vencimentoAte
  );

  return (
    <div className="min-h-screen bg-slate-50">
      <Navbar />
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-2xl font-bold text-slate-900">Pagamentos</h1>
            <p className="mt-1 text-sm text-slate-500">
              Consulte e gerencie os pagamentos dos pedidos.
            </p>
          </div>
          <Button onClick={openCreate}>+ Novo Pagamento</Button>
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
                <div className="sm:w-40">
                  <Input
                    label="Pedido (ID)"
                    placeholder="Ex: 123"
                    type="number"
                    min="1"
                    value={pedidoIdFilter}
                    onChange={(e) => setPedidoIdFilter(e.target.value)}
                  />
                </div>
                <div className="sm:w-52">
                  <Select
                    label="Status"
                    options={STATUS_FILTER_OPTIONS}
                    value={statusFilter}
                    onChange={(e) => setStatusFilter(e.target.value)}
                  />
                </div>
                <div className="sm:w-52">
                  <Select
                    label="Forma de pagamento"
                    options={FORMA_FILTER_OPTIONS}
                    value={formaFilter}
                    onChange={(e) => setFormaFilter(e.target.value)}
                  />
                </div>
                <div className="sm:w-40">
                  <Input
                    label="Vencimento de"
                    type="date"
                    value={vencimentoDe}
                    onChange={(e) => setVencimentoDe(e.target.value)}
                  />
                </div>
                <div className="sm:w-40">
                  <Input
                    label="Vencimento ate"
                    type="date"
                    value={vencimentoAte}
                    onChange={(e) => setVencimentoAte(e.target.value)}
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
                  ? "0 pagamentos"
                  : `${startItem}-${endItem} de ${total} ${
                      total === 1 ? "pagamento" : "pagamentos"
                    }`}
              </div>
            </div>
          </div>

          <div className="p-4">
            <Table
              columns={columns}
              data={pagamentos}
              keyExtractor={(p) => p.pagamento_id}
              loading={loading}
              sortKey={sortKey}
              sortDir={sortDir}
              onSort={(key) => handleSort(key as SortKey)}
              emptyMessage={
                hasFilters
                  ? "Nenhum pagamento encontrado para os filtros aplicados."
                  : "Nenhum pagamento cadastrado."
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

        <div className="mt-4 text-xs text-slate-400">
          <strong>Nota:</strong> esta tela e de acesso comum (qualquer
          usuario autenticado). Ao editar, o ID do pagamento e o ID do
          pedido de origem nao podem ser alterados.
        </div>

        <PagamentoModal
          open={modalOpen}
          mode={modalMode}
          pagamento={editingPagamento}
          onClose={() => setModalOpen(false)}
          onSubmit={handleModalSubmit}
        />
      </main>
    </div>
  );
}

export default function PagamentosPage() {
  return (
    <ProtectedRoute>
      <PagamentosContent />
    </ProtectedRoute>
  );
}
