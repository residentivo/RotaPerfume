"use client";

import { useEffect, useMemo, useState } from "react";
import { useUltimaResposta } from "@/lib/useListaSegura";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Badge, Column } from "@/components/ui/Table";
import { UserModal } from "@/components/admin/UserModal";
import {
  apiListUsers,
  apiCreateUser,
  apiUpdateUser,
  apiToggleUserStatus,
  apiAdminResetPassword,
} from "@/lib/api";
import { User, UserRole } from "@/lib/types";

type SortKey = "id" | "nome" | "email" | "role" | "ativo";
type SortDir = "asc" | "desc";

type ActionState = {
  type: "toggle" | "reset" | null;
  userId: number | null;
};

const ROLE_FILTER_OPTIONS: { value: "" | UserRole; label: string }[] = [
  { value: "", label: "Todos os perfis" },
  { value: "admin", label: "Administrador" },
  { value: "normal", label: "Usuario Padrao" },
];

const STATUS_OPTIONS: { value: "" | "ativo" | "inativo"; label: string }[] = [
  { value: "", label: "Todos os status" },
  { value: "ativo", label: "Ativo" },
  { value: "inativo", label: "Inativo" },
];

const VENDEDOR_FILTER_OPTIONS: { value: "" | "sim" | "nao"; label: string }[] = [
  { value: "", label: "Todos" },
  { value: "sim", label: "Sim" },
  { value: "nao", label: "Nao" },
];

const LIMIT_OPTIONS = [
  { value: "10", label: "10 por pagina" },
  { value: "20", label: "20 por pagina" },
  { value: "50", label: "50 por pagina" },
  { value: "100", label: "100 por pagina" },
];

function roleLabel(role: UserRole): string {
  return role === "admin" ? "Administrador" : "Usuario Padrao";
}

function UsuariosPageContent() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [erroCarga, setErroCarga] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState<"" | UserRole>("");
  const [statusFilter, setStatusFilter] = useState<"" | "ativo" | "inativo">("");
  const [vendedorFilter, setVendedorFilter] = useState<"" | "sim" | "nao">("");
  const [sortKey, setSortKey] = useState<SortKey>("id");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  const [action, setAction] = useState<ActionState>({ type: null, userId: null });

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingUser, setEditingUser] = useState<User | null>(null);

  // FE-04: so a busca mais recente aplica o resultado (respostas obsoletas
  // sao descartadas), inclusive entre o efeito e as recargas imperativas.
  const executarBusca = useUltimaResposta();

  // Busca separada em requisicao pura + aplicacao do resultado no callback
  // assincrono (.then): o efeito nunca chama setState de forma sincrona.
  const buscarUsers = () => apiListUsers(page, limit, sortKey, sortDir);

  const aplicarUsers = (res: Awaited<ReturnType<typeof apiListUsers>>) => {
    setUsers(res.data);
    setTotal(res.total);
    setPages(res.pages);
    setErroCarga(false);
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  const aplicarErroUsers = (err: unknown) => {
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar usuarios. O endpoint /api/usuarios pode nao existir no backend.";
    setError(message);
    setErroCarga(true);
    setLoading(false);
  };

  // Recarga imperativa (apos criar/editar).
  const loadUsers = async () => {
    setLoading(true);
    setError(null);
    await executarBusca(buscarUsers(), aplicarUsers, aplicarErroUsers);
  };

  // Reset para pagina 1 quando filtros mudam — ajustado durante o render
  // (padrao "ajustar estado quando a entrada muda"), sem efeito.
  const chaveFiltros = `${search}|${roleFilter}|${statusFilter}|${vendedorFilter}`;
  const [filtrosAnteriores, setFiltrosAnteriores] = useState(chaveFiltros);
  if (filtrosAnteriores !== chaveFiltros) {
    setFiltrosAnteriores(chaveFiltros);
    setPage(1);
  }

  // Paginacao/ordenacao mudou: liga o loading durante o render e o efeito
  // so faz a busca.
  const chaveLista = `${page}|${limit}|${sortKey}|${sortDir}`;
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  if (chaveAnterior !== chaveLista) {
    setChaveAnterior(chaveLista);
    setLoading(true);
    setError(null);
  }

  useEffect(() => {
    executarBusca(buscarUsers(), aplicarUsers, aplicarErroUsers);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  // Busca e filtros continuam client-side (aplicados sobre os itens da
  // pagina atual); a ordenacao agora e feita pela API (ver loadUsers).
  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    let list = users;
    if (term) {
      list = list.filter(
        (u) =>
          u.nome.toLowerCase().includes(term) ||
          u.email.toLowerCase().includes(term) ||
          String(u.id).includes(term)
      );
    }
    if (roleFilter) {
      list = list.filter((u) => u.role === roleFilter);
    }
    if (statusFilter) {
      const wantAtivo = statusFilter === "ativo";
      list = list.filter((u) => u.ativo === wantAtivo);
    }
    if (vendedorFilter) {
      const wantVinculado = vendedorFilter === "sim";
      list = list.filter((u) =>
        wantVinculado
          ? u.id_vendedor !== null && u.id_vendedor !== undefined
          : u.id_vendedor === null || u.id_vendedor === undefined
      );
    }
    return list;
  }, [users, search, roleFilter, statusFilter, vendedorFilter]);

  // === Acoes ===

  const openCreate = () => {
    setModalMode("create");
    setEditingUser(null);
    setModalOpen(true);
  };

  const openEdit = (user: User) => {
    setModalMode("edit");
    setEditingUser(user);
    setModalOpen(true);
  };

  const handleModalSubmit = async (data: {
    nome: string;
    email: string;
    role: UserRole;
    id_vendedor: number | null;
  }) => {
    setError(null);
    if (modalMode === "create") {
      const created = await apiCreateUser({
        nome: data.nome,
        email: data.email,
        role: data.role,
        id_vendedor: data.id_vendedor,
      });
      await loadUsers();
      if (created.email_enviado) {
        setSuccess(`Usuario "${created.nome}" criado com sucesso.`);
      } else {
        setError(
          `Usuario "${created.nome}" foi criado, mas o email com a senha inicial NAO pode ser enviado — verifique a configuracao de SMTP.`
        );
      }
    } else if (editingUser) {
      const updated = await apiUpdateUser(editingUser.id, {
        nome: data.nome,
        role: data.role,
        id_vendedor: data.id_vendedor,
      });
      setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
      setSuccess(`Usuario "${updated.nome}" atualizado com sucesso.`);
    }
    setModalOpen(false);
    setTimeout(() => setSuccess(null), 4000);
  };

  const handleToggleStatus = async (user: User) => {
    const novoStatus = !user.ativo;
    const acao = novoStatus ? "reativar" : "inativar";
    const ok = window.confirm(
      `Tem certeza que deseja ${acao} o usuario "${user.nome}"?`
    );
    if (!ok) return;

    setAction({ type: "toggle", userId: user.id });
    setError(null);
    setSuccess(null);
    try {
      const updated = await apiToggleUserStatus(user.id, novoStatus);
      setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
      setSuccess(
        `Usuario ${novoStatus ? "reativado" : "inativado"} com sucesso.`
      );
      setTimeout(() => setSuccess(null), 4000);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao alterar status.";
      setError(message);
    } finally {
      setAction({ type: null, userId: null });
    }
  };

  const handleResetPassword = async (user: User) => {
    const ok = window.confirm(
      `Resetar a senha de "${user.nome}"? Uma nova senha aleatoria sera enviada para o email cadastrado.`
    );
    if (!ok) return;

    setAction({ type: "reset", userId: user.id });
    setError(null);
    setSuccess(null);
    try {
      const result = await apiAdminResetPassword(user.id);
      if (result.email_enviado) {
        setSuccess(`Nova senha enviada para o email de ${user.nome}.`);
        setTimeout(() => setSuccess(null), 4000);
      } else {
        setError(
          `Senha de ${user.nome} foi resetada, mas o email NAO pode ser enviado — verifique a configuracao de SMTP.`
        );
      }
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao resetar senha.";
      setError(message);
    } finally {
      setAction({ type: null, userId: null });
    }
  };

  const columns: Column<User>[] = [
    {
      key: "id",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      sortValue: (u) => u.id,
      render: (u) => <span className="font-mono text-xs">#{u.id}</span>,
    },
    {
      key: "nome",
      header: "Nome",
      sortable: true,
      sortValue: (u) => u.nome,
      render: (u) => (
        <button
          type="button"
          onClick={() => openEdit(u)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Editar usuario"
        >
          {u.nome}
        </button>
      ),
    },
    {
      key: "email",
      header: "Email",
      sortable: true,
      sortValue: (u) => u.email,
      render: (u) => <span className="text-slate-600">{u.email}</span>,
    },
    {
      key: "role",
      header: "Perfil",
      width: "150px",
      align: "center",
      sortable: true,
      sortValue: (u) => u.role,
      render: (u) => (
        <Badge color={u.role === "admin" ? "blue" : "gray"}>
          {roleLabel(u.role)}
        </Badge>
      ),
    },
    {
      key: "vendedor",
      header: "Vendedor",
      sortable: false,
      render: (u) => (
        <span className="text-slate-600">
          {u.id_vendedor && u.vendedor_nome
            ? `#${u.id_vendedor} - ${u.vendedor_nome}`
            : "—"}
        </span>
      ),
    },
    {
      key: "ativo",
      header: "Status",
      width: "140px",
      align: "center",
      sortable: true,
      sortValue: (u) => (u.ativo ? 1 : 0),
      render: (u) => (
        <button
          type="button"
          onClick={() => handleToggleStatus(u)}
          disabled={action.type === "toggle" && action.userId === u.id}
          title={u.ativo ? "Clique para inativar" : "Clique para reativar"}
          className={[
            "inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium transition-colors",
            "disabled:cursor-not-allowed disabled:opacity-60",
            u.ativo
              ? "bg-green-100 text-green-700 hover:bg-green-200"
              : "bg-red-100 text-red-700 hover:bg-red-200",
          ].join(" ")}
        >
          <span
            className={[
              "h-2 w-2 rounded-full",
              u.ativo ? "bg-green-500" : "bg-red-500",
            ].join(" ")}
          />
          {u.ativo ? "Ativo" : "Inativo"}
        </button>
      ),
    },
    {
      key: "actions",
      header: "Acoes",
      width: "220px",
      align: "right",
      render: (u) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(u)}
            title="Editar usuario"
          >
            Editar
          </Button>
          <Button
            size="sm"
            variant="danger"
            loading={
              action.type === "reset" && action.userId === u.id
            }
            disabled={action.type !== null}
            onClick={() => handleResetPassword(u)}
          >
            Resetar
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
          <h1 className="text-2xl font-bold text-slate-900">Usuarios</h1>
          <p className="mt-1 text-sm text-slate-500">
            Gerencie os usuarios do sistema. Apenas administradores tem acesso.
          </p>
        </div>
        <Button onClick={openCreate}>+ Novo Usuario</Button>
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
            <div className="flex flex-1 flex-col gap-3 sm:flex-row sm:items-end">
              <div className="flex-1 sm:max-w-xs">
                <Input
                  label="Buscar"
                  placeholder="Nome, email ou ID..."
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
              <div className="sm:w-56">
                <Select
                  label="Perfil"
                  options={ROLE_FILTER_OPTIONS}
                  value={roleFilter}
                  onChange={(e) => setRoleFilter(e.target.value as "" | UserRole)}
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
              <div className="sm:w-48">
                <Select
                  label="Vinculado a vendedor"
                  options={VENDEDOR_FILTER_OPTIONS}
                  value={vendedorFilter}
                  onChange={(e) =>
                    setVendedorFilter(e.target.value as "" | "sim" | "nao")
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
                ? "0 usuarios"
                : `${startItem}-${endItem} de ${total} ${
                    total === 1 ? "usuario" : "usuarios"
                  }`}
            </div>
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={filtered}
            keyExtractor={(u) => u.id}
            loading={loading}
            erroCarga={erroCarga}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              search || roleFilter || statusFilter || vendedorFilter
                ? "Nenhum usuario encontrado para os filtros aplicados."
                : "Nenhum usuario cadastrado."
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
        <strong>Nota:</strong> A busca e os filtros de perfil/status sao
        aplicados sobre os itens da pagina atual. Caso a lista esteja vazia ou
        retorne erro, verifique se os endpoints{" "}
        <code>GET/POST /api/usuarios</code>,
        <code> PUT /api/usuarios/&#123;id&#125;</code>,
        <code> PATCH /api/usuarios/&#123;id&#125;/inativar</code> e
        <code> POST /api/admin/reset-password</code> estao implementados no
        backend.
      </div>

      <UserModal
        open={modalOpen}
        mode={modalMode}
        user={editingUser}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}

export default function UsuariosPage() {
  return (
    <ProtectedRoute requireAdmin>
      <UsuariosPageContent />
    </ProtectedRoute>
  );
}
