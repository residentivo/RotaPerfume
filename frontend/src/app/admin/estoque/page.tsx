"use client";

import { useEffect, useState } from "react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { EstoqueModal } from "@/components/admin/EstoqueModal";
import { apiListEstoque, apiCreateEstoque, apiUpdateEstoque } from "@/lib/api";
import { Estoque, EstoqueInput } from "@/lib/types";
import { isAdmin } from "@/lib/auth";

type SortKey =
  | "id"
  | "sku"
  | "saldo"
  | "ruptura"
  | "data_snapshot"
  | "created_at"
  | "updated_at";
type SortDir = "asc" | "desc";

const RUPTURA_OPTIONS: { value: "" | "sim" | "nao"; label: string }[] = [
  { value: "", label: "Todos" },
  { value: "sim", label: "Em ruptura" },
  { value: "nao", label: "Sem ruptura" },
];

const LIMIT_OPTIONS = [
  { value: "10", label: "10 por pagina" },
  { value: "20", label: "20 por pagina" },
  { value: "50", label: "50 por pagina" },
  { value: "100", label: "100 por pagina" },
];

const ORIGEM_LABEL: Record<string, string> = {
  import_csv: "Importação CSV",
  faturamento: "Faturamento",
  manual: "Manual",
};

function fmtDate(dateStr: string | null): string {
  if (!dateStr) return "-";
  const d = new Date(dateStr.length <= 10 ? `${dateStr}T00:00:00` : dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  return d.toLocaleDateString("pt-BR");
}

export default function EstoquePage() {
  const [admin, setAdmin] = useState(false);
  const [registros, setRegistros] = useState<Estoque[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [dataFiltro, setDataFiltro] = useState("");
  const [rupturaFilter, setRupturaFilter] = useState<"" | "sim" | "nao">("");

  const [sortKey, setSortKey] = useState<SortKey>("sku");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingEstoque, setEditingEstoque] = useState<Estoque | null>(null);

  useEffect(() => {
    setAdmin(isAdmin());
  }, []);

  const loadEstoque = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await apiListEstoque(
        page,
        limit,
        {
          sku: search.trim() || undefined,
          data_de: dataFiltro || undefined,
          data_ate: dataFiltro || undefined,
          ruptura:
            rupturaFilter === "" ? undefined : rupturaFilter === "sim",
        },
        sortKey,
        sortDir
      );
      setRegistros(res.data);
      setTotal(res.total);
      setPages(res.pages);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao carregar estoque. O endpoint /api/estoque pode nao existir no backend.";
      setError(message);
      setRegistros([]);
      setTotal(0);
      setPages(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadEstoque();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual/data e reset para pagina 1 quando filtros mudam
  useEffect(() => {
    const timer = setTimeout(() => {
      if (page !== 1) {
        setPage(1);
      } else {
        loadEstoque();
      }
    }, 350);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search, dataFiltro, rupturaFilter]);

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
    setEditingEstoque(null);
    setModalOpen(true);
  };

  const openEdit = (registro: Estoque) => {
    if (!admin) return;
    setModalMode("edit");
    setEditingEstoque(registro);
    setModalOpen(true);
  };

  const handleModalSubmit = async (data: EstoqueInput) => {
    setError(null);
    try {
      if (modalMode === "create") {
        const created = await apiCreateEstoque(data);
        await loadEstoque();
        setSuccess(`Registro de estoque #${created.id} (${created.sku}) criado com sucesso.`);
      } else if (editingEstoque) {
        const updated = await apiUpdateEstoque(editingEstoque.id, {
          saldo: data.saldo,
        });
        setRegistros((prev) =>
          prev.map((r) => (r.id === updated.id ? updated : r))
        );
        setSuccess(`Registro de estoque #${updated.id} (${updated.sku}) atualizado com sucesso.`);
      }
      setModalOpen(false);
      setTimeout(() => setSuccess(null), 4000);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao salvar registro de estoque.";
      const friendly = /403|forbidden|permiss|acesso negado/i.test(message)
        ? "Voce nao tem permissao para criar ou editar registros de estoque (acao restrita a administradores)."
        : message;
      // Relancado com mensagem amigavel: o EstoqueModal exibe este erro no
      // proprio Alert do formulario (mesmo padrao do ProdutoModal).
      throw new Error(friendly);
    }
  };

  const columns: Column<Estoque>[] = [
    {
      key: "sku",
      header: "SKU",
      sortable: true,
      render: (e) =>
        admin ? (
          <button
            type="button"
            onClick={() => openEdit(e)}
            className="font-mono text-xs font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
            title="Editar registro de estoque"
          >
            #{e.id} - {e.sku}
          </button>
        ) : (
          <span className="font-mono text-xs font-medium text-slate-900">
            #{e.id} - {e.sku}
          </span>
        ),
    },
    {
      key: "saldo",
      header: "Saldo",
      width: "110px",
      align: "right",
      sortable: true,
      render: (e) => <span className="text-slate-700">{e.saldo}</span>,
    },
    {
      key: "ruptura",
      header: "Ruptura",
      width: "120px",
      align: "center",
      sortable: true,
      render: (e) => (
        <span
          className={[
            "inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium",
            e.ruptura
              ? "bg-red-100 text-red-700"
              : "bg-green-100 text-green-700",
          ].join(" ")}
        >
          <span
            className={[
              "h-2 w-2 rounded-full",
              e.ruptura ? "bg-red-500" : "bg-green-500",
            ].join(" ")}
          />
          {e.ruptura ? "Em ruptura" : "OK"}
        </span>
      ),
    },
    {
      key: "origem",
      header: "Origem",
      width: "150px",
      render: (e) => (
        <span className="text-slate-600">
          {ORIGEM_LABEL[e.origem] || e.origem}
        </span>
      ),
    },
    {
      key: "data_snapshot",
      header: "Data do snapshot",
      width: "150px",
      align: "center",
      sortable: true,
      render: (e) => (
        <span className="text-slate-600">{fmtDate(e.data_snapshot)}</span>
      ),
    },
    ...(admin
      ? ([
          {
            key: "actions",
            header: "Acoes",
            width: "110px",
            align: "right",
            render: (e: Estoque) => (
              <div className="inline-flex items-center justify-end gap-2">
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => openEdit(e)}
                  title="Editar registro de estoque"
                >
                  Editar
                </Button>
              </div>
            ),
          } as Column<Estoque>,
        ])
      : []),
  ];

  const startItem = total === 0 ? 0 : (page - 1) * limit + 1;
  const endItem = Math.min(page * limit, total);
  const hasFilters = search || dataFiltro || rupturaFilter;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Estoque</h1>
          <p className="mt-1 text-sm text-slate-500">
            Consulte a posicao de estoque por SKU. Sem filtro de data, mostra
            a ultima posicao de cada produto.
          </p>
        </div>
        {admin && <Button onClick={openCreate}>+ Novo registro</Button>}
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
                  placeholder="SKU..."
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
                <Input
                  label="Pesquisar por data"
                  type="date"
                  value={dataFiltro}
                  onChange={(e) => setDataFiltro(e.target.value)}
                  helperText="Vazio = ultima posicao"
                />
              </div>
              <div className="sm:w-44">
                <Select
                  label="Ruptura"
                  options={RUPTURA_OPTIONS}
                  value={rupturaFilter}
                  onChange={(e) =>
                    setRupturaFilter(e.target.value as "" | "sim" | "nao")
                  }
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
                ? "0 registros"
                : `${startItem}-${endItem} de ${total} ${
                    total === 1 ? "registro" : "registros"
                  }`}
            </div>
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={registros}
            keyExtractor={(e) => e.id}
            loading={loading}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              hasFilters
                ? "Nenhum registro de estoque encontrado para os filtros aplicados."
                : "Nenhum registro de estoque cadastrado."
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
        <strong>Nota:</strong> A busca por SKU e o filtro de data sao
        aplicados via API. Criar/editar registros de estoque e restrito a
        administradores. Verifique se o endpoint <code>GET /api/estoque</code>{" "}
        esta implementado no backend caso a lista fique vazia ou retorne erro.
      </div>

      <EstoqueModal
        open={modalOpen}
        mode={modalMode}
        estoque={editingEstoque}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}
