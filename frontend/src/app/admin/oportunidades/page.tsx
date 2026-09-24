"use client";

import { useEffect, useMemo, useState } from "react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { CarteiraGuard } from "@/components/layout/CarteiraGuard";
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { OportunidadeModal } from "@/components/admin/OportunidadeModal";
import {
  apiListOportunidades,
  apiCreateOportunidade,
  apiUpdateOportunidade,
  apiDeleteOportunidade,
  apiListVendedores,
  apiListClientes,
} from "@/lib/api";
import { useSessionUser } from "@/lib/session";
import { Oportunidade, OportunidadeInput, Vendedor, Cliente } from "@/lib/types";

type SortKey =
  | "oportunidade_id"
  | "cliente_id"
  | "vendedor_id"
  | "origem"
  | "etapa"
  | "probabilidade_pct"
  | "valor_estimado"
  | "data_abertura"
  | "data_fechamento";
type SortDir = "asc" | "desc";

const ETAPA_OPTIONS = [
  { value: "", label: "Todas as etapas" },
  { value: "Prospecção", label: "Prospecção" },
  { value: "Qualificação", label: "Qualificação" },
  { value: "Proposta enviada", label: "Proposta enviada" },
  { value: "Negociação", label: "Negociação" },
  { value: "Fechado ganho", label: "Fechado ganho" },
  { value: "Fechado perdido", label: "Fechado perdido" },
];

const ORIGEM_OPTIONS = [
  { value: "", label: "Todas as origens" },
  { value: "WhatsApp", label: "WhatsApp" },
  { value: "Indicação", label: "Indicação" },
  { value: "Inbound site", label: "Inbound site" },
  { value: "Instagram", label: "Instagram" },
  { value: "Feira de beleza", label: "Feira de beleza" },
  { value: "Reativação", label: "Reativação" },
  { value: "Prospecção ativa", label: "Prospecção ativa" },
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
  oportunidade_id: "id",
  cliente_id: "cliente_id",
  vendedor_id: "vendedor_id",
  origem: "origem",
  etapa: "etapa",
  probabilidade_pct: "probabilidade_pct",
  valor_estimado: "valor_estimado",
  data_abertura: "data_abertura",
  data_fechamento: "data_fechamento",
};

const etapaColor: Record<string, string> = {
  "Prospecção": "bg-slate-100 text-slate-700",
  "Qualificação": "bg-blue-100 text-blue-700",
  "Proposta enviada": "bg-yellow-100 text-yellow-700",
  "Negociação": "bg-orange-100 text-orange-700",
  "Fechado ganho": "bg-green-100 text-green-700",
  "Fechado perdido": "bg-red-100 text-red-700",
};

const currencyFmt = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
});

function fmtValor(v: number): string {
  return currencyFmt.format(v ?? 0);
}

function fmtDate(dateStr: string | null): string {
  if (!dateStr) return "-";
  const d = new Date(dateStr.length <= 10 ? `${dateStr}T00:00:00` : dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  return d.toLocaleDateString("pt-BR");
}

function OportunidadesContent() {
  const [oportunidades, setOportunidades] = useState<Oportunidade[]>([]);
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
  // Fonte da verdade: sessao em memoria validada por GET /api/auth/me
  // (revalidada ao montar e ao voltar o foco), nao o localStorage.
  const currentUser = useSessionUser();
  const isAdmin = currentUser?.role === "admin";
  const semCarteira = !isAdmin && !currentUser?.id_vendedor;
  const meuVendedorId =
    !isAdmin && currentUser?.id_vendedor ? String(currentUser.id_vendedor) : "";

  const [search, setSearch] = useState("");
  // Filtro livre so para admin; usuario normal usa sempre meuVendedorId
  // (acompanha mudancas do vinculo sem novo login).
  const [vendedorFilter, setVendedorFilter] = useState("");
  const filtroVendedor = isAdmin ? vendedorFilter : meuVendedorId;
  const [clienteFilter, setClienteFilter] = useState("");
  const [etapaFilter, setEtapaFilter] = useState("");
  const [origemFilter, setOrigemFilter] = useState("");
  const [dataAberturaDe, setDataAberturaDe] = useState("");
  const [dataAberturaAte, setDataAberturaAte] = useState("");

  const [sortKey, setSortKey] = useState<SortKey>("oportunidade_id");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingOportunidade, setEditingOportunidade] = useState<Oportunidade | null>(
    null
  );
  const [deletingId, setDeletingId] = useState<number | null>(null);

  // Carrega vendedores/clientes uma vez, para os selects de filtro e para
  // exibir "ID - Nome" nas linhas da tabela (a API de oportunidades so
  // retorna os IDs).
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

  const loadOportunidades = async () => {
    // Vendedor sem carteira vinculada (id_vendedor null): não há dados a
    // buscar, mantém a lista vazia sem chamar a API.
    if (semCarteira) {
      setLoading(false);
      setError(null);
      setOportunidades([]);
      setTotal(0);
      setPages(0);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const res = await apiListOportunidades(
        page,
        limit,
        {
          vendedor_id: filtroVendedor ? Number(filtroVendedor) : undefined,
          cliente_id: clienteFilter ? Number(clienteFilter) : undefined,
          etapa: etapaFilter || undefined,
          origem: origemFilter || undefined,
          data_abertura_de: dataAberturaDe || undefined,
          data_abertura_ate: dataAberturaAte || undefined,
          q: search.trim() || undefined,
        },
        ORDER_BY_MAP[sortKey],
        sortDir
      );
      setOportunidades(res.data);
      setTotal(res.total);
      setPages(res.pages);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao carregar oportunidades. O endpoint /api/oportunidades pode nao existir no backend.";
      setError(message);
      setOportunidades([]);
      setTotal(0);
      setPages(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadOportunidades();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual e reset para pagina 1 quando filtros mudam.
  // Selects (vendedor/cliente/etapa/origem) tambem passam por este efeito,
  // mas como o debounce e curto (350ms) o efeito pratico e quase imediato.
  useEffect(() => {
    const timer = setTimeout(() => {
      if (page !== 1) {
        setPage(1);
      } else {
        loadOportunidades();
      }
    }, 350);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    search,
    vendedorFilter,
    meuVendedorId,
    clienteFilter,
    etapaFilter,
    origemFilter,
    dataAberturaDe,
    dataAberturaAte,
  ]);

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
    setEditingOportunidade(null);
    setModalOpen(true);
  };

  const openEdit = (oportunidade: Oportunidade) => {
    setModalMode("edit");
    setEditingOportunidade(oportunidade);
    setModalOpen(true);
  };

  const handleModalSubmit = async (data: OportunidadeInput) => {
    setError(null);
    if (modalMode === "create") {
      const created = await apiCreateOportunidade(data);
      await loadOportunidades();
      setSuccess(`Oportunidade #${created.oportunidade_id} criada com sucesso.`);
    } else if (editingOportunidade) {
      const updated = await apiUpdateOportunidade(
        editingOportunidade.oportunidade_id,
        data
      );
      setOportunidades((prev) =>
        prev.map((o) =>
          o.oportunidade_id === updated.oportunidade_id ? updated : o
        )
      );
      setSuccess(`Oportunidade #${updated.oportunidade_id} atualizada com sucesso.`);
    }
    setModalOpen(false);
    setTimeout(() => setSuccess(null), 4000);
  };

  const handleDelete = async (oportunidade: Oportunidade) => {
    const confirmed = window.confirm(
      `Tem certeza que deseja excluir a oportunidade #${oportunidade.oportunidade_id}?`
    );
    if (!confirmed) return;

    setError(null);
    setDeletingId(oportunidade.oportunidade_id);
    try {
      await apiDeleteOportunidade(oportunidade.oportunidade_id);
      setOportunidades((prev) =>
        prev.filter((o) => o.oportunidade_id !== oportunidade.oportunidade_id)
      );
      setTotal((t) => Math.max(0, t - 1));
      setSuccess(
        `Oportunidade #${oportunidade.oportunidade_id} excluida com sucesso.`
      );
      setTimeout(() => setSuccess(null), 4000);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao excluir oportunidade.";
      setError(message);
    } finally {
      setDeletingId(null);
    }
  };

  const columns: Column<Oportunidade>[] = [
    {
      key: "cliente_id",
      header: "Cliente",
      sortable: true,
      render: (o) => (
        <button
          type="button"
          onClick={() => openEdit(o)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Editar oportunidade"
        >
          #{o.cliente_id} - {clienteNome.get(o.cliente_id) || "Cliente"}
        </button>
      ),
    },
    {
      key: "vendedor_id",
      header: "Vendedor",
      width: "170px",
      sortable: true,
      render: (o) => (
        <span className="text-slate-600">
          #{o.vendedor_id} - {vendedorNome.get(o.vendedor_id) || "Vendedor"}
        </span>
      ),
    },
    {
      key: "origem",
      header: "Origem",
      width: "150px",
      sortable: true,
      render: (o) => <span className="text-slate-600">{o.origem}</span>,
    },
    {
      key: "etapa",
      header: "Etapa",
      width: "160px",
      align: "center",
      sortable: true,
      render: (o) => (
        <span
          className={[
            "inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium",
            etapaColor[o.etapa] || "bg-slate-100 text-slate-700",
          ].join(" ")}
        >
          {o.etapa}
        </span>
      ),
    },
    {
      key: "probabilidade_pct",
      header: "Probabilidade",
      width: "120px",
      align: "center",
      sortable: true,
      render: (o) => <span className="text-slate-600">{o.probabilidade_pct}%</span>,
    },
    {
      key: "valor_estimado",
      header: "Valor estimado",
      width: "140px",
      align: "right",
      sortable: true,
      render: (o) => (
        <span className="text-slate-700">{fmtValor(o.valor_estimado)}</span>
      ),
    },
    {
      key: "data_abertura",
      header: "Abertura",
      width: "120px",
      align: "center",
      sortable: true,
      render: (o) => (
        <span className="text-slate-600">{fmtDate(o.data_abertura)}</span>
      ),
    },
    {
      key: "data_fechamento",
      header: "Fechamento",
      width: "120px",
      align: "center",
      sortable: true,
      render: (o) => (
        <span className="text-slate-600">{fmtDate(o.data_fechamento)}</span>
      ),
    },
    {
      key: "actions",
      header: "Acoes",
      width: "180px",
      align: "right",
      render: (o) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(o)}
            title="Editar oportunidade"
          >
            Editar
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => handleDelete(o)}
            disabled={deletingId === o.oportunidade_id}
            title="Excluir oportunidade"
          >
            {deletingId === o.oportunidade_id ? "Excluindo..." : "Excluir"}
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
    etapaFilter ||
    origemFilter ||
    dataAberturaDe ||
    dataAberturaAte;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Oportunidades</h1>
          <p className="mt-1 text-sm text-slate-500">
            Gerencie o funil de vendas (CRM): oportunidades por cliente e vendedor.
          </p>
        </div>
        <Button onClick={openCreate}>+ Nova Oportunidade</Button>
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
              <div className="sm:w-48">
                <Select
                  label="Etapa"
                  options={ETAPA_OPTIONS}
                  value={etapaFilter}
                  onChange={(e) => setEtapaFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-48">
                <Select
                  label="Origem"
                  options={ORIGEM_OPTIONS}
                  value={origemFilter}
                  onChange={(e) => setOrigemFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-40">
                <Input
                  label="Abertura de"
                  type="date"
                  value={dataAberturaDe}
                  onChange={(e) => setDataAberturaDe(e.target.value)}
                />
              </div>
              <div className="sm:w-40">
                <Input
                  label="Abertura ate"
                  type="date"
                  value={dataAberturaAte}
                  onChange={(e) => setDataAberturaAte(e.target.value)}
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
                ? "0 oportunidades"
                : `${startItem}-${endItem} de ${total} ${
                    total === 1 ? "oportunidade" : "oportunidades"
                  }`}
            </div>
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={oportunidades}
            keyExtractor={(o) => o.oportunidade_id}
            loading={loading}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              hasFilters
                ? "Nenhuma oportunidade encontrada para os filtros aplicados."
                : "Nenhuma oportunidade cadastrada."
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
        <strong>Nota:</strong> A busca e os filtros de vendedor/cliente/etapa/origem/
        periodo sao aplicados via API. Caso a lista esteja vazia ou retorne erro,
        verifique se o endpoint <code>GET /api/oportunidades</code> esta
        implementado no backend.
      </div>

      <OportunidadeModal
        open={modalOpen}
        mode={modalMode}
        oportunidade={editingOportunidade}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}

export default function OportunidadesPage() {
  return (
    <CarteiraGuard title="Oportunidades">
      <OportunidadesContent />
    </CarteiraGuard>
  );
}
