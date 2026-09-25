"use client";

import { useEffect, useState } from "react";
import { useDebounceFiltros, useUltimaResposta } from "@/lib/useListaSegura";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { CarteiraGuard } from "@/components/layout/CarteiraGuard";
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { ClienteModal } from "@/components/admin/ClienteModal";
import {
  apiListClientes,
  apiToggleClienteStatus,
  apiCreateCliente,
  apiUpdateCliente,
} from "@/lib/api";
import { ApiError } from "@/lib/apiError";
import { useSessionUser, useVendedorDesligado } from "@/lib/session";
import { Cliente, ClienteInput } from "@/lib/types";

// Mensagens do backend (SEC-01), usadas so como fallback se a API nao
// devolver texto.
const MSG_SEM_VENDEDOR = "usuário sem vendedor vinculado";
const MSG_CLIENTE_NAO_ENCONTRADO = "cliente não encontrado";

type SortKey =
  | "cliente_id_origem"
  | "razao_social"
  | "cnpj"
  | "segmento"
  | "cidade"
  | "data_cadastro"
  | "ativo";
type SortDir = "asc" | "desc";

type ActionState = {
  type: "toggle" | null;
  clienteId: number | null;
};

const STATUS_OPTIONS: { value: "" | "ativo" | "inativo"; label: string }[] = [
  { value: "", label: "Todos os status" },
  { value: "ativo", label: "Ativo" },
  { value: "inativo", label: "Inativo" },
];

// Mapeia a sortKey interna do frontend para o campo aceito pelo backend em
// order_by. A whitelist do backend (clienteOrderWhitelist, em
// apis/shared/repositories/cliente_repository.go) usa a chave "id" mapeada
// para a coluna cliente_id_origem, entao a ordenacao por cliente_id_origem
// continua enviando order_by=id.
const ORDER_BY_MAP: Partial<Record<SortKey, string>> = {
  cliente_id_origem: "id",
  razao_social: "razao_social",
  cnpj: "cnpj",
  segmento: "segmento",
  cidade: "cidade",
  data_cadastro: "data_cadastro",
  ativo: "ativo",
};

const LIMIT_OPTIONS = [
  { value: "10", label: "10 por pagina" },
  { value: "20", label: "20 por pagina" },
  { value: "50", label: "50 por pagina" },
  { value: "100", label: "100 por pagina" },
];

function fmtDate(dateStr: string): string {
  if (!dateStr) return "-";
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  return d.toLocaleDateString("pt-BR");
}

function fmtCnpj(cnpj: string): string {
  const digits = (cnpj || "").replace(/\D/g, "");
  if (digits.length !== 14) return cnpj;
  return digits.replace(
    /(\d{2})(\d{3})(\d{3})(\d{4})(\d{2})/,
    "$1.$2.$3/$4-$5"
  );
}

function ClientesContent() {
  // SEC-01: permissao de criacao vem da sessao em memoria validada por
  // GET /api/auth/me (mesmo padrao de admin/visitas), nunca do localStorage.
  // Usuario normal sem vendedor vinculado, ou com vendedor desligado, nao
  // pode cadastrar cliente (o backend tambem rejeita com 403).
  const currentUser = useSessionUser();
  const vendedorDesligado = useVendedorDesligado();
  const isAdmin = currentUser?.role === "admin";
  const semCarteira = !isAdmin && !currentUser?.id_vendedor;
  const podeCriar = isAdmin || (!semCarteira && !vendedorDesligado);
  // FE-05: enquanto o /me nao chega (user null), nao da para afirmar que o
  // usuario nao tem vendedor; o botao fica desabilitado com texto de espera.
  const motivoSemCriar = !currentUser
    ? "Carregando dados do usuario..."
    : semCarteira
    ? "Usuario sem vendedor vinculado: solicite o vinculo a um administrador."
    : vendedorDesligado
    ? "Vendedor desligado: cadastro de clientes bloqueado."
    : undefined;

  const [clientes, setClientes] = useState<Cliente[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [ufFilter, setUfFilter] = useState("");
  const [segmentoFilter, setSegmentoFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "ativo" | "inativo">("");
  const [sortKey, setSortKey] = useState<SortKey>("cliente_id_origem");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  const [action, setAction] = useState<ActionState>({ type: null, clienteId: null });

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingCliente, setEditingCliente] = useState<Cliente | null>(null);

  // FE-04: so a busca mais recente aplica o resultado (respostas obsoletas
  // sao descartadas), inclusive entre o efeito e as recargas imperativas.
  const executarBusca = useUltimaResposta();

  // Busca da pagina atual separada em: requisicao pura (sem setState) +
  // aplicacao do resultado, que roda no callback assincrono (.then) — o
  // efeito nunca chama setState de forma sincrona.
  const buscarClientes = () =>
    apiListClientes(
      page,
      limit,
      {
        uf: ufFilter || undefined,
        segmento: segmentoFilter || undefined,
        ativo:
          statusFilter === ""
            ? undefined
            : statusFilter === "ativo",
        q: search.trim() || undefined,
      },
      ORDER_BY_MAP[sortKey],
      sortDir
    );

  const aplicarClientes = (res: Awaited<ReturnType<typeof apiListClientes>>) => {
    setClientes(res.data);
    setTotal(res.total);
    setPages(res.pages);
    setLoading(false);
  };

  const aplicarErroClientes = (err: unknown) => {
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar clientes. O endpoint /api/clientes pode nao existir no backend.";
    setError(message);
    setClientes([]);
    setTotal(0);
    setPages(0);
    setLoading(false);
  };

  // Recarga imperativa (handlers, timers e apos criar/404).
  const loadClientes = async () => {
    setLoading(true);
    setError(null);
    await executarBusca(buscarClientes(), aplicarClientes, aplicarErroClientes);
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
    executarBusca(buscarClientes(), aplicarClientes, aplicarErroClientes);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual e reset para pagina 1 quando filtros mudam
  // FE-04: o debounce nao dispara na montagem, so quando a chave dos filtros
  // muda (evita a busca dupla e a tabela voltando para a pagina 1).
  const chaveFiltros = JSON.stringify([search, ufFilter, segmentoFilter, statusFilter]);
  useDebounceFiltros(chaveFiltros, () => {
    if (page !== 1) {
      setPage(1);
    } else {
      loadClientes();
    }
  });

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  // SEC-01: 404 = cliente fora da carteira (ou removido) — exibe a mensagem
  // da API e recarrega a lista, que pode estar desatualizada.
  const tratarClienteNaoEncontrado = async (err: ApiError) => {
    setModalOpen(false);
    await loadClientes();
    setError(err.message || MSG_CLIENTE_NAO_ENCONTRADO);
  };

  const handleToggleStatus = async (cliente: Cliente) => {
    const novoStatus = !cliente.ativo;
    const acao = novoStatus ? "reativar" : "inativar";
    const ok = window.confirm(
      `Tem certeza que deseja ${acao} o cliente "${cliente.razao_social}"?`
    );
    if (!ok) return;

    setAction({ type: "toggle", clienteId: cliente.cliente_id_origem });
    setError(null);
    setSuccess(null);
    try {
      const updated = await apiToggleClienteStatus(
        cliente.cliente_id_origem,
        novoStatus
      );
      setClientes((prev) =>
        prev.map((c) =>
          c.cliente_id_origem === updated.cliente_id_origem ? updated : c
        )
      );
      setSuccess(
        `Cliente ${novoStatus ? "reativado" : "inativado"} com sucesso.`
      );
      setTimeout(() => setSuccess(null), 4000);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        await tratarClienteNaoEncontrado(err);
      } else if (err instanceof ApiError && err.status === 403) {
        setError(err.message || MSG_SEM_VENDEDOR);
      } else {
        const message =
          err instanceof Error ? err.message : "Erro ao alterar status.";
        setError(message);
      }
    } finally {
      setAction({ type: null, clienteId: null });
    }
  };

  const openCreate = () => {
    if (!podeCriar) return;
    setModalMode("create");
    setEditingCliente(null);
    setModalOpen(true);
  };

  const openEdit = (cliente: Cliente) => {
    setModalMode("edit");
    setEditingCliente(cliente);
    setModalOpen(true);
  };

  const handleModalSubmit = async (data: ClienteInput) => {
    setError(null);
    try {
      if (modalMode === "create") {
        const created = await apiCreateCliente(data);
        // O backend vincula o cliente novo a carteira do vendedor: recarregar
        // faz ele aparecer tambem para o usuario normal.
        await loadClientes();
        setSuccess(`Cliente "${created.razao_social}" criado com sucesso.`);
      } else if (editingCliente) {
        const updated = await apiUpdateCliente(
          editingCliente.cliente_id_origem,
          data
        );
        setClientes((prev) =>
          prev.map((c) =>
            c.cliente_id_origem === updated.cliente_id_origem ? updated : c
          )
        );
        setSuccess(`Cliente "${updated.razao_social}" atualizado com sucesso.`);
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        await tratarClienteNaoEncontrado(err);
        return;
      }
      if (err instanceof ApiError && err.status === 403) {
        // Exibido no proprio modal (ClienteModal mostra err.message).
        throw new ApiError(403, err.message || MSG_SEM_VENDEDOR);
      }
      throw err;
    }
    setModalOpen(false);
    setTimeout(() => setSuccess(null), 4000);
  };

  const columns: Column<Cliente>[] = [
    {
      key: "cliente_id_origem",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      sortValue: (c) => c.cliente_id_origem,
      render: (c) => (
        <span className="font-mono text-xs">#{c.cliente_id_origem}</span>
      ),
    },
    {
      key: "razao_social",
      header: "Razao Social",
      sortable: true,
      render: (c) => (
        <button
          type="button"
          onClick={() => openEdit(c)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Editar cliente"
        >
          {c.razao_social}
        </button>
      ),
    },
    {
      key: "cnpj",
      header: "CNPJ",
      width: "180px",
      sortable: true,
      render: (c) => (
        <span className="font-mono text-xs text-slate-600">
          {fmtCnpj(c.cnpj)}
        </span>
      ),
    },
    {
      key: "segmento",
      header: "Segmento",
      width: "160px",
      sortable: true,
      render: (c) => <span className="text-slate-600">{c.segmento || "-"}</span>,
    },
    {
      key: "cidade",
      header: "Cidade/UF",
      width: "180px",
      sortable: true,
      render: (c) => (
        <span className="text-slate-600">
          {c.cidade}
          {c.uf ? `/${c.uf}` : ""}
        </span>
      ),
    },
    {
      key: "data_cadastro",
      header: "Cadastro",
      width: "120px",
      align: "center",
      sortable: true,
      render: (c) => (
        <span className="text-slate-600">{fmtDate(c.data_cadastro)}</span>
      ),
    },
    {
      key: "ativo",
      header: "Status",
      width: "140px",
      sortable: true,
      align: "center",
      render: (c) => (
        <button
          type="button"
          onClick={() => handleToggleStatus(c)}
          disabled={
            action.type === "toggle" &&
            action.clienteId === c.cliente_id_origem
          }
          title={c.ativo ? "Clique para inativar" : "Clique para reativar"}
          className={[
            "inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium transition-colors",
            "disabled:cursor-not-allowed disabled:opacity-60",
            c.ativo
              ? "bg-green-100 text-green-700 hover:bg-green-200"
              : "bg-red-100 text-red-700 hover:bg-red-200",
          ].join(" ")}
        >
          <span
            className={[
              "h-2 w-2 rounded-full",
              c.ativo ? "bg-green-500" : "bg-red-500",
            ].join(" ")}
          />
          {c.ativo ? "Ativo" : "Inativo"}
        </button>
      ),
    },
    {
      key: "actions",
      header: "Acoes",
      width: "120px",
      align: "right",
      render: (c) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(c)}
            title="Editar cliente"
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
          <h1 className="text-2xl font-bold text-slate-900">Clientes</h1>
          <p className="mt-1 text-sm text-slate-500">
            Consulte e gerencie os clientes cadastrados no CRM.
          </p>
        </div>
        <div className="flex flex-col items-start gap-1 sm:items-end">
          <Button
            onClick={openCreate}
            disabled={!podeCriar}
            title={motivoSemCriar}
          >
            + Novo Cliente
          </Button>
          {!podeCriar && motivoSemCriar && (
            <p className="max-w-xs text-xs text-slate-500">{motivoSemCriar}</p>
          )}
        </div>
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
                  placeholder="Razao social ou CNPJ..."
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
              <div className="sm:w-32">
                <Input
                  label="UF"
                  placeholder="Ex: SP"
                  maxLength={2}
                  value={ufFilter}
                  onChange={(e) => setUfFilter(e.target.value.toUpperCase())}
                />
              </div>
              <div className="sm:w-56">
                <Input
                  label="Segmento"
                  placeholder="Ex: Varejo"
                  value={segmentoFilter}
                  onChange={(e) => setSegmentoFilter(e.target.value)}
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
                ? "0 clientes"
                : `${startItem}-${endItem} de ${total} ${
                    total === 1 ? "cliente" : "clientes"
                  }`}
            </div>
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={clientes}
            keyExtractor={(c) => c.cliente_id_origem}
            loading={loading}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              search || ufFilter || segmentoFilter || statusFilter
                ? "Nenhum cliente encontrado para os filtros aplicados."
                : "Nenhum cliente cadastrado."
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
        <strong>Nota:</strong> A busca e os filtros de UF/segmento/status sao
        aplicados via API. Caso a lista esteja vazia ou retorne erro, verifique
        se os endpoints <code>GET /api/clientes</code> e{" "}
        <code>PATCH /api/clientes/&#123;id&#125;/inativar</code> estao
        implementados no backend.
      </div>

      <ClienteModal
        open={modalOpen}
        mode={modalMode}
        cliente={editingCliente}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}

export default function ClientesPage() {
  return (
    <CarteiraGuard title="Clientes">
      <ClientesContent />
    </CarteiraGuard>
  );
}
