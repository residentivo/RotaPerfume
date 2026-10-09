"use client";

import { useEffect, useMemo, useState } from "react";
import {
  faixaExibida,
  mensagemRecargaFalhou,
  usePaginaCarregada,
  useUltimaResposta,
} from "@/lib/useListaSegura";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Badge, Column } from "@/components/ui/Table";
import { Paginador } from "@/components/ui/Paginador";
import { useMensagemTemporaria } from "@/lib/useMensagemTemporaria";
import { UserModal } from "@/components/admin/UserModal";
import {
  apiListUsers,
  apiCreateUser,
  apiUpdateUser,
  apiToggleUserStatus,
  apiAdminResetPassword,
} from "@/lib/api";
import { User, UserRole } from "@/lib/types";
import { vlog } from "@/lib/vlog";

const FILE = "admin/usuarios/page.tsx";

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
  vlog(FILE, "UsuariosPageContent", "inicializando estado da lista de usuarios");
  const [users, setUsers] = useState<User[]>([]);
  vlog(FILE, "UsuariosPageContent", "inicializando estado de loading");
  const [loading, setLoading] = useState(true);
  vlog(FILE, "UsuariosPageContent", "inicializando estado de erro");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "UsuariosPageContent", "inicializando estado de erro de carga");
  const [erroCarga, setErroCarga] = useState(false);
  vlog(FILE, "UsuariosPageContent", "obtendo controle de mensagem temporaria de sucesso");
  const {
    mensagem: success,
    mostrar: mostrarSucesso,
    limpar: limparSucesso,
  } = useMensagemTemporaria();
  vlog(FILE, "UsuariosPageContent", "inicializando estado da busca textual");
  const [search, setSearch] = useState("");
  vlog(FILE, "UsuariosPageContent", "inicializando estado do filtro de perfil");
  const [roleFilter, setRoleFilter] = useState<"" | UserRole>("");
  vlog(FILE, "UsuariosPageContent", "inicializando estado do filtro de status");
  const [statusFilter, setStatusFilter] = useState<"" | "ativo" | "inativo">("");
  vlog(FILE, "UsuariosPageContent", "inicializando estado do filtro de vinculo a vendedor");
  const [vendedorFilter, setVendedorFilter] = useState<"" | "sim" | "nao">("");
  vlog(FILE, "UsuariosPageContent", "inicializando estado da coluna de ordenacao");
  const [sortKey, setSortKey] = useState<SortKey>("id");
  vlog(FILE, "UsuariosPageContent", "inicializando estado da direcao de ordenacao");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  vlog(FILE, "UsuariosPageContent", "inicializando estado da acao em andamento");
  const [action, setAction] = useState<ActionState>({ type: null, userId: null });

  vlog(FILE, "UsuariosPageContent", "inicializando estado da pagina atual");
  const [page, setPage] = useState(1);
  vlog(FILE, "UsuariosPageContent", "inicializando estado do limite por pagina");
  const [limit, setLimit] = useState(20);
  vlog(FILE, "UsuariosPageContent", "inicializando estado do total de registros");
  const [total, setTotal] = useState(0);
  vlog(FILE, "UsuariosPageContent", "inicializando estado do total de paginas");
  const [pages, setPages] = useState(0);
  // FE-10: pagina/limite exibidos (a pedida, ou a ultima carregada se a
  // ultima carga falhou).
  vlog(FILE, "UsuariosPageContent", "obtendo pagina exibida (page=%d, limit=%d, erroCarga=%s)", page, limit, erroCarga);
  const { exibida, registrar: registrarCarregada } = usePaginaCarregada(
    page,
    limit,
    erroCarga
  );

  // Modal state
  vlog(FILE, "UsuariosPageContent", "inicializando estado de abertura do modal");
  const [modalOpen, setModalOpen] = useState(false);
  vlog(FILE, "UsuariosPageContent", "inicializando estado do modo do modal");
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  vlog(FILE, "UsuariosPageContent", "inicializando estado do usuario em edicao");
  const [editingUser, setEditingUser] = useState<User | null>(null);

  // FE-04: so a busca mais recente aplica o resultado (respostas obsoletas
  // sao descartadas), inclusive entre o efeito e as recargas imperativas.
  vlog(FILE, "UsuariosPageContent", "obtendo executor de busca com descarte de respostas obsoletas");
  const executarBusca = useUltimaResposta();

  // Busca separada em requisicao pura + aplicacao do resultado no callback
  // assincrono (.then): o efeito nunca chama setState de forma sincrona.
  vlog(FILE, "UsuariosPageContent", "definindo funcao buscarUsers");
  const buscarUsers = () => {
    vlog(FILE, "UsuariosPageContent.buscarUsers", "chamando apiListUsers (page=%d, limit=%d, sortKey=%s, sortDir=%s)", page, limit, sortKey, sortDir);
    return apiListUsers(page, limit, sortKey, sortDir);
  };

  vlog(FILE, "UsuariosPageContent", "definindo funcao aplicarUsers");
  const aplicarUsers = (res: Awaited<ReturnType<typeof apiListUsers>>) => {
    vlog(FILE, "UsuariosPageContent.aplicarUsers", "aplicando lista de usuarios (qtd=%d)", res.data.length);
    setUsers(res.data);
    vlog(FILE, "UsuariosPageContent.aplicarUsers", "atualizando total (total=%d)", res.total);
    setTotal(res.total);
    vlog(FILE, "UsuariosPageContent.aplicarUsers", "atualizando total de paginas (pages=%d)", res.pages);
    setPages(res.pages);
    vlog(FILE, "UsuariosPageContent.aplicarUsers", "registrando pagina carregada (page=%d, limit=%d)", page, limit);
    registrarCarregada(page, limit);
    vlog(FILE, "UsuariosPageContent.aplicarUsers", "limpando erro de carga");
    setErroCarga(false);
    vlog(FILE, "UsuariosPageContent.aplicarUsers", "desativando loading");
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  vlog(FILE, "UsuariosPageContent", "definindo funcao aplicarErroUsers");
  const aplicarErroUsers = (err: unknown): string => {
    vlog(FILE, "UsuariosPageContent.aplicarErroUsers", "extraindo mensagem do erro de carga");
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar usuarios. O endpoint /api/usuarios pode nao existir no backend.";
    vlog(FILE, "UsuariosPageContent.aplicarErroUsers", "exibindo mensagem de erro");
    setError(message);
    vlog(FILE, "UsuariosPageContent.aplicarErroUsers", "marcando erro de carga");
    setErroCarga(true);
    vlog(FILE, "UsuariosPageContent.aplicarErroUsers", "desativando loading");
    setLoading(false);
    return message;
  };

  // Recarga imperativa (apos criar/editar). FE-10: devolve a mensagem de
  // erro se a recarga falhar (null se deu certo ou foi superada por outra).
  vlog(FILE, "UsuariosPageContent", "definindo funcao loadUsers");
  const loadUsers = async (): Promise<string | null> => {
    vlog(FILE, "UsuariosPageContent.loadUsers", "ativando loading");
    setLoading(true);
    vlog(FILE, "UsuariosPageContent.loadUsers", "limpando erro");
    setError(null);
    vlog(FILE, "UsuariosPageContent.loadUsers", "inicializando variavel de falha");
    let falha: string | null = null;
    vlog(FILE, "UsuariosPageContent.loadUsers", "executando busca de usuarios");
    await executarBusca(buscarUsers(), aplicarUsers, (err) => {
      vlog(FILE, "UsuariosPageContent.loadUsers.func", "registrando falha da recarga");
      falha = aplicarErroUsers(err);
    });
    return falha;
  };

  // FE-10: navegacao a partir da pagina exibida. Se o destino ja e a pagina
  // pedida (a troca anterior falhou), repete a busca em vez de nao fazer nada.
  vlog(FILE, "UsuariosPageContent", "definindo funcao irParaPagina");
  const irParaPagina = (n: number) => {
    vlog(FILE, "UsuariosPageContent.irParaPagina", "verificando se destino e a pagina atual (destino=%d, atual=%d)", n, page);
    if (n === page) loadUsers();
    else setPage(n);
  };

  // Reset para pagina 1 quando filtros mudam — ajustado durante o render
  // (padrao "ajustar estado quando a entrada muda"), sem efeito.
  vlog(FILE, "UsuariosPageContent", "montando chave dos filtros");
  const chaveFiltros = `${search}|${roleFilter}|${statusFilter}|${vendedorFilter}`;
  vlog(FILE, "UsuariosPageContent", "inicializando estado dos filtros anteriores");
  const [filtrosAnteriores, setFiltrosAnteriores] = useState(chaveFiltros);
  vlog(FILE, "UsuariosPageContent", "verificando se os filtros mudaram");
  if (filtrosAnteriores !== chaveFiltros) {
    vlog(FILE, "UsuariosPageContent", "atualizando filtros anteriores");
    setFiltrosAnteriores(chaveFiltros);
    vlog(FILE, "UsuariosPageContent", "voltando para a pagina 1");
    setPage(1);
  }

  // Paginacao/ordenacao mudou: liga o loading durante o render e o efeito
  // so faz a busca.
  vlog(FILE, "UsuariosPageContent", "montando chave de paginacao/ordenacao");
  const chaveLista = `${page}|${limit}|${sortKey}|${sortDir}`;
  vlog(FILE, "UsuariosPageContent", "inicializando estado da chave anterior");
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  vlog(FILE, "UsuariosPageContent", "verificando se a chave de paginacao/ordenacao mudou");
  if (chaveAnterior !== chaveLista) {
    vlog(FILE, "UsuariosPageContent", "atualizando chave anterior");
    setChaveAnterior(chaveLista);
    vlog(FILE, "UsuariosPageContent", "ativando loading");
    setLoading(true);
    vlog(FILE, "UsuariosPageContent", "limpando erro");
    setError(null);
  }

  vlog(FILE, "UsuariosPageContent", "registrando efeito de busca por paginacao/ordenacao");
  useEffect(() => {
    vlog(FILE, "UsuariosPageContent.useEffect", "executando busca de usuarios");
    executarBusca(buscarUsers(), aplicarUsers, aplicarErroUsers);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  vlog(FILE, "UsuariosPageContent", "definindo handler handleSort");
  const handleSort = (key: SortKey) => {
    vlog(FILE, "UsuariosPageContent.handleSort", "verificando se coluna ja e a ordenada (key=%s)", key);
    if (sortKey === key) {
      vlog(FILE, "UsuariosPageContent.handleSort", "invertendo direcao da ordenacao");
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      vlog(FILE, "UsuariosPageContent.handleSort", "definindo nova coluna de ordenacao");
      setSortKey(key);
      vlog(FILE, "UsuariosPageContent.handleSort", "definindo direcao asc");
      setSortDir("asc");
    }
  };

  // Busca e filtros continuam client-side (aplicados sobre os itens da
  // pagina atual); a ordenacao agora e feita pela API (ver loadUsers).
  vlog(FILE, "UsuariosPageContent", "memorizando lista filtrada client-side");
  const filtered = useMemo(() => {
    vlog(FILE, "UsuariosPageContent.filtered", "normalizando termo de busca (tamanho=%d)", search.trim().length);
    const term = search.trim().toLowerCase();
    vlog(FILE, "UsuariosPageContent.filtered", "iniciando lista com usuarios da pagina (qtd=%d)", users.length);
    let list = users;
    vlog(FILE, "UsuariosPageContent.filtered", "verificando se ha termo de busca");
    if (term) {
      vlog(FILE, "UsuariosPageContent.filtered", "filtrando por nome, email ou id");
      list = list.filter(
        (u) =>
          u.nome.toLowerCase().includes(term) ||
          u.email.toLowerCase().includes(term) ||
          String(u.id).includes(term)
      );
    }
    vlog(FILE, "UsuariosPageContent.filtered", "verificando filtro de perfil (perfil=%s)", roleFilter);
    if (roleFilter) {
      vlog(FILE, "UsuariosPageContent.filtered", "filtrando por perfil");
      list = list.filter((u) => u.role === roleFilter);
    }
    vlog(FILE, "UsuariosPageContent.filtered", "verificando filtro de status (status=%s)", statusFilter);
    if (statusFilter) {
      vlog(FILE, "UsuariosPageContent.filtered", "calculando status desejado");
      const wantAtivo = statusFilter === "ativo";
      vlog(FILE, "UsuariosPageContent.filtered", "filtrando por status (ativo=%s)", wantAtivo);
      list = list.filter((u) => u.ativo === wantAtivo);
    }
    vlog(FILE, "UsuariosPageContent.filtered", "verificando filtro de vinculo a vendedor (vinculo=%s)", vendedorFilter);
    if (vendedorFilter) {
      vlog(FILE, "UsuariosPageContent.filtered", "calculando vinculo desejado");
      const wantVinculado = vendedorFilter === "sim";
      vlog(FILE, "UsuariosPageContent.filtered", "filtrando por vinculo a vendedor (vinculado=%s)", wantVinculado);
      list = list.filter((u) =>
        wantVinculado
          ? u.id_vendedor !== null && u.id_vendedor !== undefined
          : u.id_vendedor === null || u.id_vendedor === undefined
      );
    }
    vlog(FILE, "UsuariosPageContent.filtered", "filtros aplicados (qtd=%d)", list.length);
    return list;
  }, [users, search, roleFilter, statusFilter, vendedorFilter]);

  // === Acoes ===

  vlog(FILE, "UsuariosPageContent", "definindo handler openCreate");
  const openCreate = () => {
    vlog(FILE, "UsuariosPageContent.openCreate", "definindo modo create");
    setModalMode("create");
    vlog(FILE, "UsuariosPageContent.openCreate", "limpando usuario em edicao");
    setEditingUser(null);
    vlog(FILE, "UsuariosPageContent.openCreate", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "UsuariosPageContent", "definindo handler openEdit");
  const openEdit = (user: User) => {
    vlog(FILE, "UsuariosPageContent.openEdit", "definindo modo edit (usuario_id=%d)", user.id);
    setModalMode("edit");
    vlog(FILE, "UsuariosPageContent.openEdit", "definindo usuario em edicao");
    setEditingUser(user);
    vlog(FILE, "UsuariosPageContent.openEdit", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "UsuariosPageContent", "definindo handler handleModalSubmit");
  const handleModalSubmit = async (data: {
    nome: string;
    email: string;
    role: UserRole;
    id_vendedor: number | null;
  }) => {
    vlog(FILE, "UsuariosPageContent.handleModalSubmit", "limpando erro");
    setError(null);
    vlog(FILE, "UsuariosPageContent.handleModalSubmit", "verificando modo do modal (modo=%s)", modalMode);
    if (modalMode === "create") {
      vlog(FILE, "UsuariosPageContent.handleModalSubmit", "chamando apiCreateUser (papel=%s)", data.role);
      const created = await apiCreateUser({
        nome: data.nome,
        email: data.email,
        role: data.role,
        id_vendedor: data.id_vendedor,
      });
      vlog(FILE, "UsuariosPageContent.handleModalSubmit", "recarregando usuarios apos criacao (usuario_id=%d)", created.id);
      const erroRecarga = await loadUsers();
      vlog(FILE, "UsuariosPageContent.handleModalSubmit", "montando mensagem de resultado (email_enviado=%s)", !!created.email_enviado);
      const resultado = created.email_enviado
        ? `Usuario "${created.nome}" criado com sucesso.`
        : `Usuario "${created.nome}" foi criado, mas o email com a senha inicial NAO pode ser enviado — verifique a configuracao de SMTP.`;
      vlog(FILE, "UsuariosPageContent.handleModalSubmit", "verificando falha na recarga (falhou=%s)", erroRecarga !== null);
      if (erroRecarga) {
        vlog(FILE, "UsuariosPageContent.handleModalSubmit", "exibindo erro de recarga com resultado");
        setError(mensagemRecargaFalhou(resultado, erroRecarga));
      } else if (created.email_enviado) {
        vlog(FILE, "UsuariosPageContent.handleModalSubmit", "exibindo mensagem de sucesso");
        mostrarSucesso(resultado);
      } else {
        vlog(FILE, "UsuariosPageContent.handleModalSubmit", "exibindo aviso de email nao enviado");
        setError(resultado);
      }
    } else if (editingUser) {
      vlog(FILE, "UsuariosPageContent.handleModalSubmit", "chamando apiUpdateUser (usuario_id=%d, papel=%s)", editingUser.id, data.role);
      const updated = await apiUpdateUser(editingUser.id, {
        nome: data.nome,
        role: data.role,
        id_vendedor: data.id_vendedor,
      });
      vlog(FILE, "UsuariosPageContent.handleModalSubmit", "substituindo usuario atualizado na lista");
      setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
      vlog(FILE, "UsuariosPageContent.handleModalSubmit", "exibindo mensagem de sucesso");
      mostrarSucesso(`Usuario "${updated.nome}" atualizado com sucesso.`);
    }
    vlog(FILE, "UsuariosPageContent.handleModalSubmit", "fechando modal");
    setModalOpen(false);
  };

  vlog(FILE, "UsuariosPageContent", "definindo handler handleToggleStatus");
  const handleToggleStatus = async (user: User) => {
    vlog(FILE, "UsuariosPageContent.handleToggleStatus", "calculando novo status (usuario_id=%d)", user.id);
    const novoStatus = !user.ativo;
    vlog(FILE, "UsuariosPageContent.handleToggleStatus", "definindo acao (novoStatus=%s)", novoStatus);
    const acao = novoStatus ? "reativar" : "inativar";
    vlog(FILE, "UsuariosPageContent.handleToggleStatus", "pedindo confirmacao ao usuario");
    const ok = window.confirm(
      `Tem certeza que deseja ${acao} o usuario "${user.nome}"?`
    );
    vlog(FILE, "UsuariosPageContent.handleToggleStatus", "verificando confirmacao (ok=%s)", ok);
    if (!ok) return;

    vlog(FILE, "UsuariosPageContent.handleToggleStatus", "marcando acao toggle em andamento");
    setAction({ type: "toggle", userId: user.id });
    vlog(FILE, "UsuariosPageContent.handleToggleStatus", "limpando erro");
    setError(null);
    vlog(FILE, "UsuariosPageContent.handleToggleStatus", "limpando mensagem de sucesso");
    limparSucesso();
    vlog(FILE, "UsuariosPageContent.handleToggleStatus", "iniciando chamada de alteracao de status");
    try {
      vlog(FILE, "UsuariosPageContent.handleToggleStatus", "chamando apiToggleUserStatus");
      const updated = await apiToggleUserStatus(user.id, novoStatus);
      vlog(FILE, "UsuariosPageContent.handleToggleStatus", "substituindo usuario atualizado na lista (usuario_id=%d)", updated.id);
      setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
      vlog(FILE, "UsuariosPageContent.handleToggleStatus", "exibindo mensagem de sucesso");
      mostrarSucesso(
        `Usuario ${novoStatus ? "reativado" : "inativado"} com sucesso.`
      );
    } catch (err) {
      vlog(FILE, "UsuariosPageContent.handleToggleStatus", "falha ao alterar status; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao alterar status.";
      vlog(FILE, "UsuariosPageContent.handleToggleStatus", "exibindo mensagem de erro");
      setError(message);
    } finally {
      vlog(FILE, "UsuariosPageContent.handleToggleStatus", "limpando acao em andamento");
      setAction({ type: null, userId: null });
    }
  };

  vlog(FILE, "UsuariosPageContent", "definindo handler handleResetPassword");
  const handleResetPassword = async (user: User) => {
    vlog(FILE, "UsuariosPageContent.handleResetPassword", "pedindo confirmacao de reset de senha (usuario_id=%d)", user.id);
    const ok = window.confirm(
      `Resetar a senha de "${user.nome}"? Uma nova senha aleatoria sera enviada para o email cadastrado.`
    );
    vlog(FILE, "UsuariosPageContent.handleResetPassword", "verificando confirmacao (ok=%s)", ok);
    if (!ok) return;

    vlog(FILE, "UsuariosPageContent.handleResetPassword", "marcando acao reset em andamento");
    setAction({ type: "reset", userId: user.id });
    vlog(FILE, "UsuariosPageContent.handleResetPassword", "limpando erro");
    setError(null);
    vlog(FILE, "UsuariosPageContent.handleResetPassword", "limpando mensagem de sucesso");
    limparSucesso();
    vlog(FILE, "UsuariosPageContent.handleResetPassword", "iniciando chamada de reset de senha");
    try {
      vlog(FILE, "UsuariosPageContent.handleResetPassword", "chamando apiAdminResetPassword");
      const result = await apiAdminResetPassword(user.id);
      vlog(FILE, "UsuariosPageContent.handleResetPassword", "verificando envio do email (email_enviado=%s)", !!result.email_enviado);
      if (result.email_enviado) {
        vlog(FILE, "UsuariosPageContent.handleResetPassword", "exibindo mensagem de sucesso");
        mostrarSucesso(`Nova senha enviada para o email de ${user.nome}.`);
      } else {
        vlog(FILE, "UsuariosPageContent.handleResetPassword", "exibindo aviso de email nao enviado");
        setError(
          `Senha de ${user.nome} foi resetada, mas o email NAO pode ser enviado — verifique a configuracao de SMTP.`
        );
      }
    } catch (err) {
      vlog(FILE, "UsuariosPageContent.handleResetPassword", "falha no reset; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao resetar senha.";
      vlog(FILE, "UsuariosPageContent.handleResetPassword", "exibindo mensagem de erro");
      setError(message);
    } finally {
      vlog(FILE, "UsuariosPageContent.handleResetPassword", "limpando acao em andamento");
      setAction({ type: null, userId: null });
    }
  };

  vlog(FILE, "UsuariosPageContent", "montando definicao das colunas da tabela");
  const columns: Column<User>[] = [
    {
      key: "id",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      render: (u) => <span className="font-mono text-xs">#{u.id}</span>,
    },
    {
      key: "nome",
      header: "Nome",
      sortable: true,
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
      render: (u) => <span className="text-slate-600">{u.email}</span>,
    },
    {
      key: "role",
      header: "Perfil",
      width: "150px",
      align: "center",
      sortable: true,
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

  // FE-10: contador baseado na pagina exibida; oculto se nada foi carregado
  // (a primeira carga falhou), para nao afirmar "0 usuarios".
  vlog(FILE, "UsuariosPageContent", "calculando faixa exibida (pagina=%d, limite=%d, total=%d)", exibida.pagina, exibida.limite, total);
  const { inicio: startItem, fim: endItem } = faixaExibida(
    exibida.pagina,
    exibida.limite,
    total
  );
  vlog(FILE, "UsuariosPageContent", "calculando se oculta o contador (qtd=%d)", users.length);
  const ocultarContador = erroCarga && users.length === 0;

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
          <Alert variant="success" onClose={() => limparSucesso()}>
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
            {!ocultarContador && (
              <div className="text-sm text-slate-500">
                {total === 0
                  ? "0 usuarios"
                  : `${startItem}-${endItem} de ${total} ${
                      total === 1 ? "usuario" : "usuarios"
                    }`}
              </div>
            )}
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

        <Paginador pagina={exibida.pagina} paginas={pages} onIrPara={irParaPagina} />
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
