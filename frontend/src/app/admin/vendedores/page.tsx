"use client";

import { useEffect, useMemo, useState } from "react";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { VendedorModal } from "@/components/admin/VendedorModal";
import {
  apiListVendedores,
  apiCreateVendedor,
  apiUpdateVendedor,
  apiDeleteVendedor,
  apiReativarVendedor,
} from "@/lib/api";
import { Vendedor, VendedorCompleto, VendedorInput } from "@/lib/types";

// GET /api/vendedores nao aceita paginacao/filtros/ordenacao no backend
// (endpoint simples, historicamente usado so para popular combobox) — ver
// apis/rotaperfumes-api/handlers/vendedor_handler.go (ListVendedores). Por
// isso a busca, ordenacao e paginacao desta tela sao aplicadas no cliente,
// sobre a lista completa de vendedores (ativos e inativos) retornada pela
// API.
type SortKey = "id" | "nome" | "regiao" | "uf" | "status";
type SortDir = "asc" | "desc";

const LIMIT_OPTIONS = [
  { value: "10", label: "10 por pagina" },
  { value: "20", label: "20 por pagina" },
  { value: "50", label: "50 por pagina" },
  { value: "100", label: "100 por pagina" },
];

type StatusFilter = "todos" | "ativo" | "inativo";

const STATUS_OPTIONS: { value: string; label: string }[] = [
  { value: "todos", label: "Todos" },
  { value: "ativo", label: "Ativo" },
  { value: "inativo", label: "Inativo" },
];

const TODAS_OPTION = { value: "", label: "Todas" };

function VendedoresPageContent() {
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [regiaoFilter, setRegiaoFilter] = useState("");
  const [ufFilter, setUfFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("todos");
  const [sortKey, setSortKey] = useState<SortKey>("nome");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  const [togglingId, setTogglingId] = useState<number | null>(null);

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingVendedor, setEditingVendedor] = useState<VendedorCompleto | null>(
    null
  );

  const aplicarVendedores = (res: Vendedor[]) => {
    setVendedores(res);
    setLoading(false);
  };

  const aplicarErroVendedores = (err: unknown) => {
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar vendedores. O endpoint /api/vendedores pode nao existir no backend.";
    setError(message);
    setVendedores([]);
    setLoading(false);
  };

  // Recarga imperativa (apos criar/editar/ativar).
  const loadVendedores = async () => {
    setLoading(true);
    setError(null);
    await apiListVendedores().then(aplicarVendedores, aplicarErroVendedores);
  };

  // Carga inicial: o loading ja comeca true; o efeito so faz a busca e
  // aplica o resultado no callback assincrono.
  useEffect(() => {
    apiListVendedores().then(aplicarVendedores, aplicarErroVendedores);
  }, []);

  // Reseta para pagina 1 quando busca/filtros/ordenacao mudam — ajustado
  // durante o render (padrao "ajustar estado quando a entrada muda").
  const chaveFiltros = `${search}|${regiaoFilter}|${ufFilter}|${statusFilter}|${sortKey}|${sortDir}`;
  const [filtrosAnteriores, setFiltrosAnteriores] = useState(chaveFiltros);
  if (filtrosAnteriores !== chaveFiltros) {
    setFiltrosAnteriores(chaveFiltros);
    setPage(1);
  }

  const regiaoOptions = useMemo(() => {
    const values = Array.from(new Set(vendedores.map((v) => v.regiao))).sort(
      (a, b) => a.localeCompare(b, "pt-BR")
    );
    return [TODAS_OPTION, ...values.map((v) => ({ value: v, label: v }))];
  }, [vendedores]);

  const ufOptions = useMemo(() => {
    const values = Array.from(new Set(vendedores.map((v) => v.uf))).sort(
      (a, b) => a.localeCompare(b, "pt-BR")
    );
    return [TODAS_OPTION, ...values.map((v) => ({ value: v, label: v }))];
  }, [vendedores]);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const filteredSorted = useMemo(() => {
    const term = search.trim().toLowerCase();
    let list = vendedores;
    if (term) {
      list = list.filter(
        (v) =>
          String(v.id).includes(term) ||
          v.nome.toLowerCase().includes(term) ||
          v.regiao.toLowerCase().includes(term) ||
          v.uf.toLowerCase().includes(term) ||
          (v.data_desligamento
            ? "inativo".includes(term)
            : "ativo".includes(term))
      );
    }
    if (regiaoFilter) {
      list = list.filter((v) => v.regiao === regiaoFilter);
    }
    if (ufFilter) {
      list = list.filter((v) => v.uf === ufFilter);
    }
    if (statusFilter !== "todos") {
      list = list.filter((v) =>
        statusFilter === "ativo" ? !v.data_desligamento : !!v.data_desligamento
      );
    }
    const sorted = [...list].sort((a, b) => {
      if (sortKey === "status") {
        const av = a.data_desligamento ? 0 : 1;
        const bv = b.data_desligamento ? 0 : 1;
        return sortDir === "asc" ? av - bv : bv - av;
      }
      const av = a[sortKey];
      const bv = b[sortKey];
      if (typeof av === "number" && typeof bv === "number") {
        return sortDir === "asc" ? av - bv : bv - av;
      }
      const cmp = String(av).localeCompare(String(bv), "pt-BR", {
        numeric: true,
      });
      return sortDir === "asc" ? cmp : -cmp;
    });
    return sorted;
  }, [
    vendedores,
    search,
    regiaoFilter,
    ufFilter,
    statusFilter,
    sortKey,
    sortDir,
  ]);

  const total = filteredSorted.length;
  const pages = Math.max(1, Math.ceil(total / limit));
  const pageSafe = Math.min(page, pages);
  const paginated = useMemo(
    () => filteredSorted.slice((pageSafe - 1) * limit, pageSafe * limit),
    [filteredSorted, pageSafe, limit]
  );

  const openCreate = () => {
    setModalMode("create");
    setEditingVendedor(null);
    setModalOpen(true);
  };

  const openEdit = (vendedor: Vendedor) => {
    setModalMode("edit");
    // O modal busca o detalhe completo (GET /api/vendedores/{id}) para
    // preencher meta_mensal/data_admissao e a lista de clientes vinculados;
    // aqui passamos os campos ja conhecidos como fallback imediato.
    setEditingVendedor({
      id: vendedor.id,
      nome: vendedor.nome,
      regiao: vendedor.regiao,
      uf: vendedor.uf,
      data_admissao: "",
      data_desligamento: vendedor.data_desligamento,
      meta_mensal: 0,
      created_at: "",
      updated_at: "",
    });
    setModalOpen(true);
  };

  const handleModalSubmit = async (data: VendedorInput) => {
    setError(null);
    if (modalMode === "create") {
      const created = await apiCreateVendedor(data);
      await loadVendedores();
      setSuccess(`Vendedor "${created.nome}" criado com sucesso.`);
    } else if (editingVendedor) {
      await apiUpdateVendedor(editingVendedor.id, data);
      await loadVendedores();
      setSuccess(`Vendedor "${data.nome}" atualizado com sucesso.`);
    }
    setModalOpen(false);
    setTimeout(() => setSuccess(null), 4000);
  };

  const handleDelete = async (vendedor: Vendedor) => {
    const ok = window.confirm(
      `Tem certeza que deseja inativar o vendedor "${vendedor.nome}"? O historico de pedidos/carteiras sera preservado.`
    );
    if (!ok) return;

    setTogglingId(vendedor.id);
    setError(null);
    setSuccess(null);
    try {
      await apiDeleteVendedor(vendedor.id);
      await loadVendedores();
      setSuccess(`Vendedor "${vendedor.nome}" inativado com sucesso.`);
      setTimeout(() => setSuccess(null), 4000);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao inativar vendedor.";
      setError(message);
    } finally {
      setTogglingId(null);
    }
  };

  const handleReativar = async (vendedor: Vendedor) => {
    const ok = window.confirm(
      `Tem certeza que deseja reativar o vendedor "${vendedor.nome}"?`
    );
    if (!ok) return;

    setTogglingId(vendedor.id);
    setError(null);
    setSuccess(null);
    try {
      await apiReativarVendedor(vendedor.id);
      await loadVendedores();
      setSuccess(`Vendedor "${vendedor.nome}" reativado com sucesso.`);
      setTimeout(() => setSuccess(null), 4000);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao reativar vendedor.";
      setError(message);
    } finally {
      setTogglingId(null);
    }
  };

  const handleToggleStatus = (vendedor: Vendedor) => {
    if (vendedor.data_desligamento) {
      handleReativar(vendedor);
    } else {
      handleDelete(vendedor);
    }
  };

  const columns: Column<Vendedor>[] = [
    {
      key: "nome",
      header: "Vendedor",
      sortable: true,
      render: (v) => (
        <button
          type="button"
          onClick={() => openEdit(v)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Editar vendedor"
        >
          #{v.id} - {v.nome}
        </button>
      ),
    },
    {
      key: "regiao",
      header: "Regiao",
      width: "200px",
      sortable: true,
      render: (v) => <span className="text-slate-600">{v.regiao}</span>,
    },
    {
      key: "uf",
      header: "UF",
      width: "100px",
      align: "center",
      sortable: true,
      render: (v) => <span className="text-slate-600">{v.uf}</span>,
    },
    {
      key: "status",
      header: "Status",
      width: "140px",
      align: "center",
      sortable: true,
      sortValue: (v) => (v.data_desligamento ? 0 : 1),
      render: (v) => (
        <button
          type="button"
          onClick={() => handleToggleStatus(v)}
          disabled={togglingId === v.id}
          title={
            v.data_desligamento
              ? "Clique para reativar"
              : "Clique para inativar"
          }
          className={[
            "inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium transition-colors",
            "disabled:cursor-not-allowed disabled:opacity-60",
            v.data_desligamento
              ? "bg-red-100 text-red-700 hover:bg-red-200"
              : "bg-green-100 text-green-700 hover:bg-green-200",
          ].join(" ")}
        >
          <span
            className={[
              "h-2 w-2 rounded-full",
              v.data_desligamento ? "bg-red-500" : "bg-green-500",
            ].join(" ")}
          />
          {v.data_desligamento ? "Inativo" : "Ativo"}
        </button>
      ),
    },
    {
      key: "actions",
      header: "Acoes",
      width: "100px",
      align: "right",
      render: (v) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(v)}
            title="Editar vendedor"
          >
            Editar
          </Button>
        </div>
      ),
    },
  ];

  const startItem = total === 0 ? 0 : (pageSafe - 1) * limit + 1;
  const endItem = Math.min(pageSafe * limit, total);

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Vendedores</h1>
          <p className="mt-1 text-sm text-slate-500">
            Consulte e gerencie os vendedores cadastrados no CRM.
          </p>
        </div>
        <Button onClick={openCreate}>+ Novo Vendedor</Button>
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
                  placeholder="Nome, regiao ou UF..."
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
              <div className="sm:w-44">
                <Select
                  label="Regiao"
                  options={regiaoOptions}
                  value={regiaoFilter}
                  onChange={(e) => setRegiaoFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-32">
                <Select
                  label="UF"
                  options={ufOptions}
                  value={ufFilter}
                  onChange={(e) => setUfFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-36">
                <Select
                  label="Status"
                  options={STATUS_OPTIONS}
                  value={statusFilter}
                  onChange={(e) =>
                    setStatusFilter(e.target.value as StatusFilter)
                  }
                />
              </div>
              <div className="sm:w-44">
                <Select
                  label="Itens por pagina"
                  options={LIMIT_OPTIONS}
                  value={String(limit)}
                  onChange={(e) => {
                    setLimit(Number(e.target.value));
                    setPage(1);
                  }}
                />
              </div>
            </div>
            <div className="text-sm text-slate-500">
              {total === 0
                ? "0 vendedores"
                : `${startItem}-${endItem} de ${total} ${
                    total === 1 ? "vendedor" : "vendedores"
                  }`}
            </div>
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={paginated}
            keyExtractor={(v) => v.id}
            loading={loading}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              search || regiaoFilter || ufFilter || statusFilter !== "todos"
                ? "Nenhum vendedor encontrado para a busca/filtros aplicados."
                : "Nenhum vendedor cadastrado."
            }
          />
        </div>

        {pages > 1 && (
          <div className="flex flex-col items-center justify-between gap-3 border-t border-slate-200 px-4 py-3 sm:flex-row">
            <div className="text-sm text-slate-500">
              Pagina <strong>{pageSafe}</strong> de <strong>{pages}</strong>
            </div>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="secondary"
                disabled={pageSafe <= 1}
                onClick={() => setPage(1)}
                title="Primeira pagina"
              >
                {"<<"}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={pageSafe <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                title="Pagina anterior"
              >
                {"<"}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={pageSafe >= pages}
                onClick={() => setPage((p) => Math.min(pages, p + 1))}
                title="Proxima pagina"
              >
                {">"}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={pageSafe >= pages}
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
        <strong>Nota:</strong> A busca, os filtros (regiao/UF/status),
        ordenacao e paginacao desta tela sao aplicados no navegador, pois{" "}
        <code>GET /api/vendedores</code> retorna a lista completa de
        vendedores (ativos e inativos), sem parametros de
        paginacao/filtro/ordenacao no backend. Vendedores inativados
        continuam aparecendo na lista, marcados com{" "}
        <strong>[X] Inativo</strong> na coluna Status — use os filtros de
        Status/Regiao/UF para restringir a visualizacao.
      </div>

      <VendedorModal
        open={modalOpen}
        mode={modalMode}
        vendedor={editingVendedor}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}

export default function VendedoresPage() {
  return (
    <ProtectedRoute requireAdmin>
      <VendedoresPageContent />
    </ProtectedRoute>
  );
}
