"use client";

import { useEffect, useMemo, useState } from "react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
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

export default function UsuariosPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("id");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  const [action, setAction] = useState<ActionState>({ type: null, userId: null });

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingUser, setEditingUser] = useState<User | null>(null);

  const loadUsers = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await apiListUsers();
      setUsers(data);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao carregar usuarios. O endpoint /api/usuarios pode nao existir no backend.";
      setError(message);
      setUsers([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadUsers();
  }, []);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const filteredAndSorted = useMemo(() => {
    const term = search.trim().toLowerCase();
    let list = users;
    if (term) {
      list = list.filter(
        (u) =>
          u.nome.toLowerCase().includes(term) ||
          u.email.toLowerCase().includes(term)
      );
    }
    const sorted = [...list].sort((a, b) => {
      const av = a[sortKey];
      const bv = b[sortKey];
      if (av === undefined || bv === undefined) return 0;
      let cmp = 0;
      if (typeof av === "number" && typeof bv === "number") {
        cmp = av - bv;
      } else {
        cmp = String(av).localeCompare(String(bv), "pt-BR");
      }
      return sortDir === "asc" ? cmp : -cmp;
    });
    return sorted;
  }, [users, search, sortKey, sortDir]);

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
    senha?: string;
  }) => {
    setError(null);
    if (modalMode === "create") {
      const created = await apiCreateUser({
        nome: data.nome,
        email: data.email,
        role: data.role,
        senha: data.senha,
      });
      setUsers((prev) => [...prev, created]);
      setSuccess(`Usuario "${created.nome}" criado com sucesso.`);
    } else if (editingUser) {
      const updated = await apiUpdateUser(editingUser.id, {
        nome: data.nome,
        role: data.role,
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
      `Resetar a senha de "${user.nome}" para "Mudar@123"?`
    );
    if (!ok) return;

    setAction({ type: "reset", userId: user.id });
    setError(null);
    setSuccess(null);
    try {
      await apiAdminResetPassword(user.id, "Mudar@123");
      setSuccess(`Senha de ${user.nome} resetada com sucesso.`);
      setTimeout(() => setSuccess(null), 4000);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao resetar senha.";
      setError(message);
    } finally {
      setAction({ type: null, userId: null });
    }
  };

  const SortableHeader = ({
    label,
    sortKeyName,
    align,
  }: {
    label: string;
    sortKeyName: SortKey;
    align?: "left" | "center" | "right";
  }) => (
    <button
      type="button"
      onClick={() => handleSort(sortKeyName)}
      className={[
        "inline-flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-slate-600 hover:text-slate-900",
        align === "center"
          ? "justify-center"
          : align === "right"
          ? "justify-end"
          : "",
      ].join(" ")}
    >
      {label}
      {sortKey === sortKeyName && (
        <span className="text-primary-600">
          {sortDir === "asc" ? "↑" : "↓"}
        </span>
      )}
    </button>
  );

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
      width: "120px",
      align: "center",
      sortable: true,
      sortValue: (u) => u.role,
      render: (u) => (
        <Badge color={u.role === "admin" ? "blue" : "gray"}>
          {u.role === "admin" ? "admin" : u.role}
        </Badge>
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
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex-1 sm:max-w-xs">
              <Input
                placeholder="Buscar por nome ou email..."
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
            <div className="text-sm text-slate-500">
              {filteredAndSorted.length}{" "}
              {filteredAndSorted.length === 1 ? "usuario" : "usuarios"}
            </div>
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={filteredAndSorted}
            keyExtractor={(u) => u.id}
            loading={loading}
            emptyMessage={
              search
                ? "Nenhum usuario encontrado para a busca."
                : "Nenhum usuario cadastrado."
            }
          />
        </div>
      </Card>

      <div className="mt-4 text-xs text-slate-400">
        <strong>Nota:</strong> Caso a lista esteja vazia ou retorne erro,
        verifique se os endpoints <code>GET/POST /api/usuarios</code>,
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
