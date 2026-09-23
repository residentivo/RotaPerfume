"use client";

import { useEffect, useMemo, useState } from "react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { VisitaModal } from "@/components/admin/VisitaModal";
import {
  apiListVisitas,
  apiCreateVisita,
  apiUpdateVisita,
  apiDeleteVisita,
  apiListVendedores,
  apiListClientes,
} from "@/lib/api";
import { getUser } from "@/lib/auth";
import { Visita, VisitaInput, Vendedor, Cliente } from "@/lib/types";

type SortKey =
  | "visita_id"
  | "cliente_id"
  | "vendedor_id"
  | "data_visita"
  | "resultado"
  | "duracao_min";
type SortDir = "asc" | "desc";

const RESULTADO_OPTIONS = [
  { value: "", label: "Todos os resultados" },
  { value: "Sem pedido", label: "Sem pedido" },
  { value: "Pedido realizado", label: "Pedido realizado" },
  { value: "Reagendada", label: "Reagendada" },
  { value: "Cliente ausente", label: "Cliente ausente" },
  { value: "Apenas relacionamento", label: "Apenas relacionamento" },
];

const LIMIT_OPTIONS = [
  { value: "10", label: "10 por pagina" },
  { value: "20", label: "20 por pagina" },
  { value: "50", label: "50 por pagina" },
  { value: "100", label: "100 por pagina" },
];

// Mapeia a sortKey interna do frontend para o campo aceito pelo backend em
// order_by (mesmo padrao das demais telas administrativas).
const ORDER_BY_MAP: Partial<Record<SortKey, string>> = {
  visita_id: "id",
  cliente_id: "cliente_id",
  vendedor_id: "vendedor_id",
  data_visita: "data_visita",
  resultado: "resultado",
  duracao_min: "duracao_min",
};

const resultadoColor: Record<string, string> = {
  "Sem pedido": "bg-slate-100 text-slate-700",
  "Pedido realizado": "bg-green-100 text-green-700",
  "Reagendada": "bg-yellow-100 text-yellow-700",
  "Cliente ausente": "bg-red-100 text-red-700",
  "Apenas relacionamento": "bg-blue-100 text-blue-700",
};

function fmtDate(dateStr: string | null): string {
  if (!dateStr) return "-";
  const d = new Date(dateStr.length <= 10 ? `${dateStr}T00:00:00` : dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  return d.toLocaleDateString("pt-BR");
}

export default function VisitasPage() {
  const [visitas, setVisitas] = useState<Visita[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Listas auxiliares para exibir nome do cliente/vendedor nas linhas e
  // popular os filtros de coluna.
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  const [clientes, setClientes] = useState<Cliente[]>([]);

  // Usuário logado: vendedores (role !== "admin") só enxergam a própria
  // carteira — o filtro de vendedor é travado com o id_vendedor do usuário
  // e o seletor "todos os vendedores" fica oculto. Admin mantém o filtro
  // livre (padrão anterior). Sem id_vendedor vinculado, o vendedor não tem
  // carteira: a lista permanece vazia (mesmo comportamento do backend).
  const currentUser = getUser();
  const isAdmin = currentUser?.role === "admin";
  const semCarteira = !isAdmin && !currentUser?.id_vendedor;

  const [search, setSearch] = useState("");
  const [vendedorFilter, setVendedorFilter] = useState(
    !isAdmin && currentUser?.id_vendedor ? String(currentUser.id_vendedor) : ""
  );
  const [clienteFilter, setClienteFilter] = useState("");
  const [resultadoFilter, setResultadoFilter] = useState("");
  const [dataVisitaDe, setDataVisitaDe] = useState("");
  const [dataVisitaAte, setDataVisitaAte] = useState("");

  const [sortKey, setSortKey] = useState<SortKey>("visita_id");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingVisita, setEditingVisita] = useState<Visita | null>(null);
  const [deletingId, setDeletingId] = useState<number | null>(null);

  // Carrega vendedores/clientes uma vez, para os selects de filtro e para
  // exibir "ID - Nome" nas linhas da tabela (a API de visitas so retorna
  // os IDs).
  useEffect(() => {
    apiListVendedores()
      .then(setVendedores)
      .catch(() => setVendedores([]));

    const loadAllClientes = async () => {
      const PAGE_SIZE = 100;
      try {
        const first = await apiListClientes(1, PAGE_SIZE, {});
        const all = [...first.data];
        const totalPages = first.pages || 1;
        for (let p = 2; p <= totalPages; p++) {
          const res = await apiListClientes(p, PAGE_SIZE, {});
          all.push(...res.data);
        }
        setClientes(all);
      } catch {
        setClientes([]);
      }
    };
    loadAllClientes();
  }, []);

  const vendedorNome = useMemo(() => {
    const map = new Map<number, string>();
    vendedores.forEach((v) => map.set(v.id, v.nome));
    return map;
  }, [vendedores]);

  const clienteNome = useMemo(() => {
    const map = new Map<number, string>();
    clientes.forEach((c) => map.set(c.cliente_id_origem, c.razao_social));
    return map;
  }, [clientes]);

  const vendedorOptions = useMemo(
    () => [
      { value: "", label: "Todos os vendedores" },
      ...vendedores.map((v) => ({
        value: String(v.id),
        label: `#${v.id} - ${v.nome}`,
      })),
    ],
    [vendedores]
  );

  const clienteOptions = useMemo(
    () => [
      { value: "", label: "Todos os clientes" },
      ...clientes.map((c) => ({
        value: String(c.cliente_id_origem),
        label: `#${c.cliente_id_origem} - ${c.razao_social}`,
      })),
    ],
    [clientes]
  );

  const loadVisitas = async () => {
    // Vendedor sem carteira vinculada (id_vendedor null): não há dados a
    // buscar, mantém a lista vazia sem chamar a API.
    if (semCarteira) {
      setLoading(false);
      setError(null);
      setVisitas([]);
      setTotal(0);
      setPages(0);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const res = await apiListVisitas(
        page,
        limit,
        {
          vendedor_id: vendedorFilter ? Number(vendedorFilter) : undefined,
          cliente_id: clienteFilter ? Number(clienteFilter) : undefined,
          resultado: resultadoFilter || undefined,
          data_visita_de: dataVisitaDe || undefined,
          data_visita_ate: dataVisitaAte || undefined,
          q: search.trim() || undefined,
        },
        ORDER_BY_MAP[sortKey],
        sortDir
      );
      setVisitas(res.data);
      setTotal(res.total);
      setPages(res.pages);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao carregar visitas. O endpoint /api/visitas pode nao existir no backend.";
      setError(message);
      setVisitas([]);
      setTotal(0);
      setPages(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadVisitas();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual e reset para pagina 1 quando filtros mudam.
  // Selects (vendedor/cliente/resultado) tambem passam por este efeito, mas
  // como o debounce e curto (350ms) o efeito pratico e quase imediato.
  useEffect(() => {
    const timer = setTimeout(() => {
      if (page !== 1) {
        setPage(1);
      } else {
        loadVisitas();
      }
    }, 350);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search, vendedorFilter, clienteFilter, resultadoFilter, dataVisitaDe, dataVisitaAte]);

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
    setEditingVisita(null);
    setModalOpen(true);
  };

  const openEdit = (visita: Visita) => {
    setModalMode("edit");
    setEditingVisita(visita);
    setModalOpen(true);
  };

  const handleModalSubmit = async (data: VisitaInput) => {
    setError(null);
    if (modalMode === "create") {
      const created = await apiCreateVisita(data);
      await loadVisitas();
      setSuccess(`Visita #${created.visita_id} criada com sucesso.`);
    } else if (editingVisita) {
      const updated = await apiUpdateVisita(editingVisita.visita_id, data);
      setVisitas((prev) =>
        prev.map((v) => (v.visita_id === updated.visita_id ? updated : v))
      );
      setSuccess(`Visita #${updated.visita_id} atualizada com sucesso.`);
    }
    setModalOpen(false);
    setTimeout(() => setSuccess(null), 4000);
  };

  const handleDelete = async (visita: Visita) => {
    const confirmed = window.confirm(
      `Tem certeza que deseja excluir a visita #${visita.visita_id}?`
    );
    if (!confirmed) return;

    setError(null);
    setDeletingId(visita.visita_id);
    try {
      await apiDeleteVisita(visita.visita_id);
      setVisitas((prev) => prev.filter((v) => v.visita_id !== visita.visita_id));
      setTotal((t) => Math.max(0, t - 1));
      setSuccess(`Visita #${visita.visita_id} excluida com sucesso.`);
      setTimeout(() => setSuccess(null), 4000);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao excluir visita.";
      setError(message);
    } finally {
      setDeletingId(null);
    }
  };

  const columns: Column<Visita>[] = [
    {
      key: "cliente_id",
      header: "Cliente",
      sortable: true,
      render: (v) => (
        <button
          type="button"
          onClick={() => openEdit(v)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Editar visita"
        >
          #{v.cliente_id} - {clienteNome.get(v.cliente_id) || "Cliente"}
        </button>
      ),
    },
    {
      key: "vendedor_id",
      header: "Vendedor",
      width: "170px",
      sortable: true,
      render: (v) => (
        <span className="text-slate-600">
          #{v.vendedor_id} - {vendedorNome.get(v.vendedor_id) || "Vendedor"}
        </span>
      ),
    },
    {
      key: "data_visita",
      header: "Data",
      width: "120px",
      align: "center",
      sortable: true,
      render: (v) => <span className="text-slate-600">{fmtDate(v.data_visita)}</span>,
    },
    {
      key: "resultado",
      header: "Resultado",
      width: "180px",
      align: "center",
      sortable: true,
      render: (v) => (
        <span
          className={[
            "inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium",
            resultadoColor[v.resultado] || "bg-slate-100 text-slate-700",
          ].join(" ")}
        >
          {v.resultado}
        </span>
      ),
    },
    {
      key: "duracao_min",
      header: "Duracao (min)",
      width: "130px",
      align: "center",
      sortable: true,
      render: (v) => <span className="text-slate-600">{v.duracao_min}</span>,
    },
    {
      key: "actions",
      header: "Acoes",
      width: "180px",
      align: "right",
      render: (v) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(v)}
            title="Editar visita"
          >
            Editar
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => handleDelete(v)}
            disabled={deletingId === v.visita_id}
            title="Excluir visita"
          >
            {deletingId === v.visita_id ? "Excluindo..." : "Excluir"}
          </Button>
        </div>
      ),
    },
  ];

  const startItem = total === 0 ? 0 : (page - 1) * limit + 1;
  const endItem = Math.min(page * limit, total);

  const hasFilters =
    search ||
    vendedorFilter ||
    clienteFilter ||
    resultadoFilter ||
    dataVisitaDe ||
    dataVisitaAte;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Visitas</h1>
          <p className="mt-1 text-sm text-slate-500">
            Gerencie as visitas do CRM: registros de visitas por cliente e vendedor.
          </p>
        </div>
        <Button onClick={openCreate}>+ Nova Visita</Button>
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
                  placeholder="Cliente, vendedor..."
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
              {isAdmin && (
                <div className="sm:w-56">
                  <Select
                    label="Vendedor"
                    options={vendedorOptions}
                    value={vendedorFilter}
                    onChange={(e) => setVendedorFilter(e.target.value)}
                  />
                </div>
              )}
              <div className="sm:w-56">
                <Select
                  label="Cliente"
                  options={clienteOptions}
                  value={clienteFilter}
                  onChange={(e) => setClienteFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-52">
                <Select
                  label="Resultado"
                  options={RESULTADO_OPTIONS}
                  value={resultadoFilter}
                  onChange={(e) => setResultadoFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-40">
                <Input
                  label="Data de"
                  type="date"
                  value={dataVisitaDe}
                  onChange={(e) => setDataVisitaDe(e.target.value)}
                />
              </div>
              <div className="sm:w-40">
                <Input
                  label="Data ate"
                  type="date"
                  value={dataVisitaAte}
                  onChange={(e) => setDataVisitaAte(e.target.value)}
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
                ? "0 visitas"
                : `${startItem}-${endItem} de ${total} ${
                    total === 1 ? "visita" : "visitas"
                  }`}
            </div>
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={visitas}
            keyExtractor={(v) => v.visita_id}
            loading={loading}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              hasFilters
                ? "Nenhuma visita encontrada para os filtros aplicados."
                : "Nenhuma visita cadastrada."
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
        <strong>Nota:</strong> A busca e os filtros de vendedor/cliente/resultado/
        periodo sao aplicados via API. Caso a lista esteja vazia ou retorne erro,
        verifique se o endpoint <code>GET /api/visitas</code> esta implementado no
        backend.
      </div>

      <VisitaModal
        open={modalOpen}
        mode={modalMode}
        visita={editingVisita}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}
