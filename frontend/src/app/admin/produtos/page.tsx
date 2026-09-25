"use client";

import { useEffect, useState } from "react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { ProdutoModal } from "@/components/admin/ProdutoModal";
import {
  apiListProdutos,
  apiToggleProdutoStatus,
  apiCreateProduto,
  apiUpdateProduto,
} from "@/lib/api";
import { Produto, ProdutoInput } from "@/lib/types";

type SortKey =
  | "id"
  | "sku"
  | "descricao"
  | "categoria"
  | "marca"
  | "preco_tabela"
  | "ativo";
type SortDir = "asc" | "desc";

type ActionState = {
  type: "toggle" | null;
  produtoId: number | null;
};

const STATUS_OPTIONS: { value: "" | "ativo" | "inativo"; label: string }[] = [
  { value: "", label: "Todos os status" },
  { value: "ativo", label: "Ativo" },
  { value: "inativo", label: "Inativo" },
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

function fmtPreco(v: number): string {
  return currencyFmt.format(v ?? 0);
}

function ProdutosPageContent() {
  const [produtos, setProdutos] = useState<Produto[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [categoriaFilter, setCategoriaFilter] = useState("");
  const [marcaFilter, setMarcaFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "ativo" | "inativo">("");
  const [sortKey, setSortKey] = useState<SortKey>("id");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  const [action, setAction] = useState<ActionState>({ type: null, produtoId: null });

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingProduto, setEditingProduto] = useState<Produto | null>(null);

  // Busca separada em requisicao pura + aplicacao do resultado no callback
  // assincrono (.then): o efeito nunca chama setState de forma sincrona.
  const buscarProdutos = () =>
    apiListProdutos(
      page,
      limit,
      {
        categoria: categoriaFilter || undefined,
        marca: marcaFilter || undefined,
        ativo:
          statusFilter === ""
            ? undefined
            : statusFilter === "ativo",
        q: search.trim() || undefined,
      },
      sortKey,
      sortDir
    );

  const aplicarProdutos = (res: Awaited<ReturnType<typeof apiListProdutos>>) => {
    setProdutos(res.data);
    setTotal(res.total);
    setPages(res.pages);
    setLoading(false);
  };

  const aplicarErroProdutos = (err: unknown) => {
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar produtos. O endpoint /api/produtos pode nao existir no backend.";
    setError(message);
    setProdutos([]);
    setTotal(0);
    setPages(0);
    setLoading(false);
  };

  // Recarga imperativa (handlers e timers).
  const loadProdutos = async () => {
    setLoading(true);
    setError(null);
    await buscarProdutos().then(aplicarProdutos, aplicarErroProdutos);
  };

  // Paginacao/ordenacao mudou: liga o loading durante o render (padrao
  // "ajustar estado quando a entrada muda") e o efeito so faz a busca.
  const chaveLista = `${page}|${limit}|${sortKey}|${sortDir}`;
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  if (chaveAnterior !== chaveLista) {
    setChaveAnterior(chaveLista);
    setLoading(true);
    setError(null);
  }

  useEffect(() => {
    buscarProdutos().then(aplicarProdutos, aplicarErroProdutos);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual e reset para pagina 1 quando filtros mudam
  useEffect(() => {
    const timer = setTimeout(() => {
      if (page !== 1) {
        setPage(1);
      } else {
        loadProdutos();
      }
    }, 350);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search, categoriaFilter, marcaFilter, statusFilter]);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const handleToggleStatus = async (produto: Produto) => {
    const novoStatus = !produto.ativo;
    const acao = novoStatus ? "reativar" : "inativar";
    const ok = window.confirm(
      `Tem certeza que deseja ${acao} o produto "${produto.descricao}"?`
    );
    if (!ok) return;

    setAction({ type: "toggle", produtoId: produto.id });
    setError(null);
    setSuccess(null);
    try {
      const updated = await apiToggleProdutoStatus(produto.id, novoStatus);
      setProdutos((prev) =>
        prev.map((p) => (p.id === updated.id ? updated : p))
      );
      setSuccess(
        `Produto ${novoStatus ? "reativado" : "inativado"} com sucesso.`
      );
      setTimeout(() => setSuccess(null), 4000);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao alterar status.";
      setError(message);
    } finally {
      setAction({ type: null, produtoId: null });
    }
  };

  const openCreate = () => {
    setModalMode("create");
    setEditingProduto(null);
    setModalOpen(true);
  };

  const openEdit = (produto: Produto) => {
    setModalMode("edit");
    setEditingProduto(produto);
    setModalOpen(true);
  };

  const handleModalSubmit = async (data: ProdutoInput) => {
    setError(null);
    if (modalMode === "create") {
      const created = await apiCreateProduto(data);
      await loadProdutos();
      setSuccess(`Produto "${created.descricao}" criado com sucesso.`);
    } else if (editingProduto) {
      const updated = await apiUpdateProduto(editingProduto.id, data);
      setProdutos((prev) =>
        prev.map((p) => (p.id === updated.id ? updated : p))
      );
      setSuccess(`Produto "${updated.descricao}" atualizado com sucesso.`);
    }
    setModalOpen(false);
    setTimeout(() => setSuccess(null), 4000);
  };

  const columns: Column<Produto>[] = [
    {
      key: "id",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      sortValue: (p) => p.id,
      render: (p) => <span className="font-mono text-xs">#{p.id}</span>,
    },
    {
      key: "descricao",
      header: "Descricao",
      sortable: true,
      render: (p) => (
        <button
          type="button"
          onClick={() => openEdit(p)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Editar produto"
        >
          {p.descricao}
        </button>
      ),
    },
    {
      key: "sku",
      header: "SKU",
      width: "140px",
      sortable: true,
      render: (p) => (
        <span className="font-mono text-xs text-slate-600">{p.sku}</span>
      ),
    },
    {
      key: "categoria",
      header: "Categoria",
      width: "160px",
      sortable: true,
      render: (p) => <span className="text-slate-600">{p.categoria || "-"}</span>,
    },
    {
      key: "marca",
      header: "Marca",
      width: "140px",
      sortable: true,
      render: (p) => <span className="text-slate-600">{p.marca || "-"}</span>,
    },
    {
      key: "preco_tabela",
      header: "Preco",
      width: "130px",
      align: "right",
      sortable: true,
      render: (p) => (
        <span className="text-slate-700">{fmtPreco(p.preco_tabela)}</span>
      ),
    },
    {
      key: "ativo",
      header: "Status",
      width: "140px",
      align: "center",
      sortable: true,
      render: (p) => (
        <button
          type="button"
          onClick={() => handleToggleStatus(p)}
          disabled={action.type === "toggle" && action.produtoId === p.id}
          title={p.ativo ? "Clique para inativar" : "Clique para reativar"}
          className={[
            "inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium transition-colors",
            "disabled:cursor-not-allowed disabled:opacity-60",
            p.ativo
              ? "bg-green-100 text-green-700 hover:bg-green-200"
              : "bg-red-100 text-red-700 hover:bg-red-200",
          ].join(" ")}
        >
          <span
            className={[
              "h-2 w-2 rounded-full",
              p.ativo ? "bg-green-500" : "bg-red-500",
            ].join(" ")}
          />
          {p.ativo ? "Ativo" : "Inativo"}
        </button>
      ),
    },
    {
      key: "actions",
      header: "Acoes",
      width: "120px",
      align: "right",
      render: (p) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(p)}
            title="Editar produto"
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
          <h1 className="text-2xl font-bold text-slate-900">Produtos</h1>
          <p className="mt-1 text-sm text-slate-500">
            Consulte e gerencie o catalogo de produtos.
          </p>
        </div>
        <Button onClick={openCreate}>+ Novo Produto</Button>
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
                  placeholder="Descricao ou SKU..."
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
                  label="Categoria"
                  placeholder="Ex: Perfumaria"
                  value={categoriaFilter}
                  onChange={(e) => setCategoriaFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-48">
                <Input
                  label="Marca"
                  placeholder="Ex: Marca X"
                  value={marcaFilter}
                  onChange={(e) => setMarcaFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-48">
                <Select
                  label="Status"
                  options={STATUS_OPTIONS}
                  value={statusFilter}
                  onChange={(e) =>
                    setStatusFilter(e.target.value as "" | "ativo" | "inativo")
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
                ? "0 produtos"
                : `${startItem}-${endItem} de ${total} ${
                    total === 1 ? "produto" : "produtos"
                  }`}
            </div>
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={produtos}
            keyExtractor={(p) => p.id}
            loading={loading}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              search || categoriaFilter || marcaFilter || statusFilter
                ? "Nenhum produto encontrado para os filtros aplicados."
                : "Nenhum produto cadastrado."
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
        <strong>Nota:</strong> A busca e os filtros de categoria/marca/status
        sao aplicados via API. Caso a lista esteja vazia ou retorne erro,
        verifique se os endpoints <code>GET /api/produtos</code> e{" "}
        <code>PATCH /api/produtos/&#123;id&#125;/inativar</code> estao
        implementados no backend.
      </div>

      <ProdutoModal
        open={modalOpen}
        mode={modalMode}
        produto={editingProduto}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}

export default function ProdutosPage() {
  return (
    <ProtectedRoute requireAdmin>
      <ProdutosPageContent />
    </ProtectedRoute>
  );
}
