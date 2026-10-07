"use client";

import { useEffect, useState } from "react";
import {
  faixaExibida,
  mensagemRecargaFalhou,
  useDebounceFiltros,
  usePaginaCarregada,
  useUltimaResposta,
} from "@/lib/useListaSegura";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { CarteiraGuard } from "@/components/layout/CarteiraGuard";
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { Paginador } from "@/components/ui/Paginador";
import { useMensagemTemporaria } from "@/lib/useMensagemTemporaria";
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
import { formatCnpj } from "@/lib/cnpj";
import { formatarData } from "@/lib/formatarData";
import { vlog } from "@/lib/vlog";

const FILE = "admin/clientes/page.tsx";

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

function ClientesContent() {
  // SEC-01: permissao de criacao vem da sessao em memoria validada por
  // GET /api/auth/me (mesmo padrao de admin/visitas), nunca do localStorage.
  // Usuario normal sem vendedor vinculado, ou com vendedor desligado, nao
  // pode cadastrar cliente (o backend tambem rejeita com 403).
  vlog(FILE, "ClientesContent", "obtendo usuario da sessao em memoria");
  const currentUser = useSessionUser();
  vlog(FILE, "ClientesContent", "verificando se o vendedor esta desligado");
  const vendedorDesligado = useVendedorDesligado();
  vlog(FILE, "ClientesContent", "calculando se usuario e admin");
  const isAdmin = currentUser?.role === "admin";
  vlog(FILE, "ClientesContent", "calculando se usuario esta sem carteira");
  const semCarteira = !isAdmin && !currentUser?.id_vendedor;
  vlog(FILE, "ClientesContent", "calculando permissao de criacao");
  const podeCriar = isAdmin || (!semCarteira && !vendedorDesligado);
  // FE-05: enquanto o /me nao chega (user null), nao da para afirmar que o
  // usuario nao tem vendedor; o botao fica desabilitado com texto de espera.
  vlog(FILE, "ClientesContent", "definindo motivo de bloqueio de criacao (admin=%s, semCarteira=%s, desligado=%s)", isAdmin, semCarteira, vendedorDesligado);
  const motivoSemCriar = !currentUser
    ? "Carregando dados do usuario..."
    : semCarteira
    ? "Usuario sem vendedor vinculado: solicite o vinculo a um administrador."
    : vendedorDesligado
    ? "Vendedor desligado: cadastro de clientes bloqueado."
    : undefined;

  vlog(FILE, "ClientesContent", "inicializando estado da lista de clientes");
  const [clientes, setClientes] = useState<Cliente[]>([]);
  vlog(FILE, "ClientesContent", "inicializando estado de loading");
  const [loading, setLoading] = useState(true);
  vlog(FILE, "ClientesContent", "inicializando estado de erro");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "ClientesContent", "inicializando estado de erro de carga");
  const [erroCarga, setErroCarga] = useState(false);
  vlog(FILE, "ClientesContent", "obtendo controle de mensagem temporaria de sucesso");
  const {
    mensagem: success,
    mostrar: mostrarSucesso,
    limpar: limparSucesso,
  } = useMensagemTemporaria();
  vlog(FILE, "ClientesContent", "inicializando estado da busca textual");
  const [search, setSearch] = useState("");
  vlog(FILE, "ClientesContent", "inicializando estado do filtro de UF");
  const [ufFilter, setUfFilter] = useState("");
  vlog(FILE, "ClientesContent", "inicializando estado do filtro de segmento");
  const [segmentoFilter, setSegmentoFilter] = useState("");
  vlog(FILE, "ClientesContent", "inicializando estado do filtro de status");
  const [statusFilter, setStatusFilter] = useState<"" | "ativo" | "inativo">("");
  vlog(FILE, "ClientesContent", "inicializando estado da coluna de ordenacao");
  const [sortKey, setSortKey] = useState<SortKey>("cliente_id_origem");
  vlog(FILE, "ClientesContent", "inicializando estado da direcao de ordenacao");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  vlog(FILE, "ClientesContent", "inicializando estado da acao em andamento");
  const [action, setAction] = useState<ActionState>({ type: null, clienteId: null });

  vlog(FILE, "ClientesContent", "inicializando estado da pagina atual");
  const [page, setPage] = useState(1);
  vlog(FILE, "ClientesContent", "inicializando estado do limite por pagina");
  const [limit, setLimit] = useState(20);
  vlog(FILE, "ClientesContent", "inicializando estado do total de registros");
  const [total, setTotal] = useState(0);
  vlog(FILE, "ClientesContent", "inicializando estado do total de paginas");
  const [pages, setPages] = useState(0);
  // FE-10: pagina/limite exibidos (a pedida, ou a ultima carregada se a
  // ultima carga falhou).
  vlog(FILE, "ClientesContent", "obtendo pagina exibida (page=%d, limit=%d, erroCarga=%s)", page, limit, erroCarga);
  const { exibida, registrar: registrarCarregada } = usePaginaCarregada(
    page,
    limit,
    erroCarga
  );

  // Modal state
  vlog(FILE, "ClientesContent", "inicializando estado de abertura do modal");
  const [modalOpen, setModalOpen] = useState(false);
  vlog(FILE, "ClientesContent", "inicializando estado do modo do modal");
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  vlog(FILE, "ClientesContent", "inicializando estado do cliente em edicao");
  const [editingCliente, setEditingCliente] = useState<Cliente | null>(null);

  // FE-04: so a busca mais recente aplica o resultado (respostas obsoletas
  // sao descartadas), inclusive entre o efeito e as recargas imperativas.
  vlog(FILE, "ClientesContent", "obtendo executor de busca com descarte de respostas obsoletas");
  const executarBusca = useUltimaResposta();

  // Busca da pagina atual separada em: requisicao pura (sem setState) +
  // aplicacao do resultado, que roda no callback assincrono (.then) — o
  // efeito nunca chama setState de forma sincrona.
  vlog(FILE, "ClientesContent", "definindo funcao buscarClientes");
  const buscarClientes = () => {
    vlog(FILE, "ClientesContent.buscarClientes", "chamando apiListClientes (page=%d, limit=%d, sortKey=%s, sortDir=%s)", page, limit, sortKey, sortDir);
    return apiListClientes(
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
  };

  vlog(FILE, "ClientesContent", "definindo funcao aplicarClientes");
  const aplicarClientes = (res: Awaited<ReturnType<typeof apiListClientes>>) => {
    vlog(FILE, "ClientesContent.aplicarClientes", "aplicando lista de clientes (qtd=%d)", res.data.length);
    setClientes(res.data);
    vlog(FILE, "ClientesContent.aplicarClientes", "atualizando total (total=%d)", res.total);
    setTotal(res.total);
    vlog(FILE, "ClientesContent.aplicarClientes", "atualizando total de paginas (pages=%d)", res.pages);
    setPages(res.pages);
    vlog(FILE, "ClientesContent.aplicarClientes", "registrando pagina carregada (page=%d, limit=%d)", page, limit);
    registrarCarregada(page, limit);
    vlog(FILE, "ClientesContent.aplicarClientes", "limpando erro de carga");
    setErroCarga(false);
    vlog(FILE, "ClientesContent.aplicarClientes", "desativando loading");
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  vlog(FILE, "ClientesContent", "definindo funcao aplicarErroClientes");
  const aplicarErroClientes = (err: unknown): string => {
    vlog(FILE, "ClientesContent.aplicarErroClientes", "extraindo mensagem do erro de carga");
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar clientes. O endpoint /api/clientes pode nao existir no backend.";
    vlog(FILE, "ClientesContent.aplicarErroClientes", "exibindo mensagem de erro");
    setError(message);
    vlog(FILE, "ClientesContent.aplicarErroClientes", "marcando erro de carga");
    setErroCarga(true);
    vlog(FILE, "ClientesContent.aplicarErroClientes", "desativando loading");
    setLoading(false);
    return message;
  };

  // Recarga imperativa (handlers, timers e apos criar/404). FE-10: devolve a mensagem de erro se a
  // recarga falhar (null se deu certo ou foi superada por outra busca).
  vlog(FILE, "ClientesContent", "definindo funcao loadClientes");
  const loadClientes = async (): Promise<string | null> => {
    vlog(FILE, "ClientesContent.loadClientes", "ativando loading");
    setLoading(true);
    vlog(FILE, "ClientesContent.loadClientes", "limpando erro");
    setError(null);
    vlog(FILE, "ClientesContent.loadClientes", "inicializando variavel de falha");
    let falha: string | null = null;
    vlog(FILE, "ClientesContent.loadClientes", "executando busca de clientes");
    await executarBusca(buscarClientes(), aplicarClientes, (err) => {
      vlog(FILE, "ClientesContent.loadClientes.func", "registrando falha da recarga");
      falha = aplicarErroClientes(err);
    });
    return falha;
  };

  // FE-10: navegacao a partir da pagina exibida. Se o destino ja e a pagina
  // pedida (a troca anterior falhou), repete a busca em vez de nao fazer nada.
  vlog(FILE, "ClientesContent", "definindo funcao irParaPagina");
  const irParaPagina = (n: number) => {
    vlog(FILE, "ClientesContent.irParaPagina", "verificando se destino e a pagina atual (destino=%d, atual=%d)", n, page);
    if (n === page) loadClientes();
    else setPage(n);
  };

  // Paginacao/ordenacao mudou: liga o loading durante o render (padrao
  // "ajustar estado quando a entrada muda") e o efeito so faz a busca.
  vlog(FILE, "ClientesContent", "montando chave de paginacao/ordenacao");
  const chaveLista = `${page}|${limit}|${sortKey}|${sortDir}`;
  vlog(FILE, "ClientesContent", "inicializando estado da chave anterior");
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  vlog(FILE, "ClientesContent", "verificando se a chave de paginacao/ordenacao mudou");
  if (chaveAnterior !== chaveLista) {
    vlog(FILE, "ClientesContent", "atualizando chave anterior");
    setChaveAnterior(chaveLista);
    vlog(FILE, "ClientesContent", "ativando loading");
    setLoading(true);
    vlog(FILE, "ClientesContent", "limpando erro");
    setError(null);
  }

  vlog(FILE, "ClientesContent", "registrando efeito de busca por paginacao/ordenacao");
  useEffect(() => {
    vlog(FILE, "ClientesContent.useEffect", "executando busca de clientes");
    executarBusca(buscarClientes(), aplicarClientes, aplicarErroClientes);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual e reset para pagina 1 quando filtros mudam
  // FE-04: o debounce nao dispara na montagem, so quando a chave dos filtros
  // muda (evita a busca dupla e a tabela voltando para a pagina 1).
  vlog(FILE, "ClientesContent", "montando chave dos filtros");
  const chaveFiltros = JSON.stringify([search, ufFilter, segmentoFilter, statusFilter]);
  vlog(FILE, "ClientesContent", "registrando debounce dos filtros");
  useDebounceFiltros(chaveFiltros, () => {
    vlog(FILE, "ClientesContent.useDebounceFiltros", "filtros mudaram; verificando se esta na pagina 1 (page=%d)", page);
    if (page !== 1) {
      vlog(FILE, "ClientesContent.useDebounceFiltros", "voltando para a pagina 1");
      setPage(1);
    } else {
      vlog(FILE, "ClientesContent.useDebounceFiltros", "recarregando clientes");
      loadClientes();
    }
  });

  vlog(FILE, "ClientesContent", "definindo handler handleSort");
  const handleSort = (key: SortKey) => {
    vlog(FILE, "ClientesContent.handleSort", "verificando se coluna ja e a ordenada (key=%s)", key);
    if (sortKey === key) {
      vlog(FILE, "ClientesContent.handleSort", "invertendo direcao da ordenacao");
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      vlog(FILE, "ClientesContent.handleSort", "definindo nova coluna de ordenacao");
      setSortKey(key);
      vlog(FILE, "ClientesContent.handleSort", "definindo direcao asc");
      setSortDir("asc");
    }
  };

  // SEC-01: 404 = cliente fora da carteira (ou removido) — exibe a mensagem
  // da API e recarrega a lista, que pode estar desatualizada.
  vlog(FILE, "ClientesContent", "definindo funcao tratarClienteNaoEncontrado");
  const tratarClienteNaoEncontrado = async (err: ApiError) => {
    vlog(FILE, "ClientesContent.tratarClienteNaoEncontrado", "fechando modal");
    setModalOpen(false);
    vlog(FILE, "ClientesContent.tratarClienteNaoEncontrado", "recarregando clientes");
    await loadClientes();
    vlog(FILE, "ClientesContent.tratarClienteNaoEncontrado", "exibindo erro de cliente nao encontrado (status=%d)", err.status);
    setError(err.message || MSG_CLIENTE_NAO_ENCONTRADO);
  };

  vlog(FILE, "ClientesContent", "definindo handler handleToggleStatus");
  const handleToggleStatus = async (cliente: Cliente) => {
    vlog(FILE, "ClientesContent.handleToggleStatus", "calculando novo status (cliente_id=%d)", cliente.cliente_id_origem);
    const novoStatus = !cliente.ativo;
    vlog(FILE, "ClientesContent.handleToggleStatus", "definindo acao (novoStatus=%s)", novoStatus);
    const acao = novoStatus ? "reativar" : "inativar";
    vlog(FILE, "ClientesContent.handleToggleStatus", "pedindo confirmacao ao usuario");
    const ok = window.confirm(
      `Tem certeza que deseja ${acao} o cliente "${cliente.razao_social}"?`
    );
    vlog(FILE, "ClientesContent.handleToggleStatus", "verificando confirmacao (ok=%s)", ok);
    if (!ok) return;

    vlog(FILE, "ClientesContent.handleToggleStatus", "marcando acao toggle em andamento");
    setAction({ type: "toggle", clienteId: cliente.cliente_id_origem });
    vlog(FILE, "ClientesContent.handleToggleStatus", "limpando erro");
    setError(null);
    vlog(FILE, "ClientesContent.handleToggleStatus", "limpando mensagem de sucesso");
    limparSucesso();
    vlog(FILE, "ClientesContent.handleToggleStatus", "iniciando chamada de alteracao de status");
    try {
      vlog(FILE, "ClientesContent.handleToggleStatus", "chamando apiToggleClienteStatus");
      const updated = await apiToggleClienteStatus(
        cliente.cliente_id_origem,
        novoStatus
      );
      vlog(FILE, "ClientesContent.handleToggleStatus", "substituindo cliente atualizado na lista (cliente_id=%d)", updated.cliente_id_origem);
      setClientes((prev) =>
        prev.map((c) =>
          c.cliente_id_origem === updated.cliente_id_origem ? updated : c
        )
      );
      vlog(FILE, "ClientesContent.handleToggleStatus", "exibindo mensagem de sucesso");
      mostrarSucesso(
        `Cliente ${novoStatus ? "reativado" : "inativado"} com sucesso.`
      );
    } catch (err) {
      vlog(FILE, "ClientesContent.handleToggleStatus", "falha ao alterar status; verificando tipo do erro");
      if (err instanceof ApiError && err.status === 404) {
        vlog(FILE, "ClientesContent.handleToggleStatus", "cliente nao encontrado (404)");
        await tratarClienteNaoEncontrado(err);
      } else if (err instanceof ApiError && err.status === 403) {
        vlog(FILE, "ClientesContent.handleToggleStatus", "acesso negado (403); exibindo erro");
        setError(err.message || MSG_SEM_VENDEDOR);
      } else {
        vlog(FILE, "ClientesContent.handleToggleStatus", "extraindo mensagem de erro generico");
        const message =
          err instanceof Error ? err.message : "Erro ao alterar status.";
        vlog(FILE, "ClientesContent.handleToggleStatus", "exibindo mensagem de erro");
        setError(message);
      }
    } finally {
      vlog(FILE, "ClientesContent.handleToggleStatus", "limpando acao em andamento");
      setAction({ type: null, clienteId: null });
    }
  };

  vlog(FILE, "ClientesContent", "definindo handler openCreate");
  const openCreate = () => {
    vlog(FILE, "ClientesContent.openCreate", "verificando permissao de criacao (podeCriar=%s)", podeCriar);
    if (!podeCriar) return;
    vlog(FILE, "ClientesContent.openCreate", "definindo modo create");
    setModalMode("create");
    vlog(FILE, "ClientesContent.openCreate", "limpando cliente em edicao");
    setEditingCliente(null);
    vlog(FILE, "ClientesContent.openCreate", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "ClientesContent", "definindo handler openEdit");
  const openEdit = (cliente: Cliente) => {
    vlog(FILE, "ClientesContent.openEdit", "definindo modo edit (cliente_id=%d)", cliente.cliente_id_origem);
    setModalMode("edit");
    vlog(FILE, "ClientesContent.openEdit", "definindo cliente em edicao");
    setEditingCliente(cliente);
    vlog(FILE, "ClientesContent.openEdit", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "ClientesContent", "definindo handler handleModalSubmit");
  const handleModalSubmit = async (data: ClienteInput) => {
    vlog(FILE, "ClientesContent.handleModalSubmit", "limpando erro");
    setError(null);
    vlog(FILE, "ClientesContent.handleModalSubmit", "iniciando envio do formulario (modo=%s)", modalMode);
    try {
      vlog(FILE, "ClientesContent.handleModalSubmit", "verificando modo do modal");
      if (modalMode === "create") {
        vlog(FILE, "ClientesContent.handleModalSubmit", "chamando apiCreateCliente");
        const created = await apiCreateCliente(data);
        // O backend vincula o cliente novo a carteira do vendedor: recarregar
        // faz ele aparecer tambem para o usuario normal.
        vlog(FILE, "ClientesContent.handleModalSubmit", "montando mensagem de sucesso (cliente_id=%d)", created.cliente_id_origem);
        const msg = `Cliente "${created.razao_social}" criado com sucesso.`;
        vlog(FILE, "ClientesContent.handleModalSubmit", "recarregando clientes apos criacao");
        const erroRecarga = await loadClientes();
        vlog(FILE, "ClientesContent.handleModalSubmit", "verificando falha na recarga (falhou=%s)", erroRecarga !== null);
        if (erroRecarga) setError(mensagemRecargaFalhou(msg, erroRecarga));
        else mostrarSucesso(msg);
      } else if (editingCliente) {
        vlog(FILE, "ClientesContent.handleModalSubmit", "chamando apiUpdateCliente (cliente_id=%d)", editingCliente.cliente_id_origem);
        const updated = await apiUpdateCliente(
          editingCliente.cliente_id_origem,
          data
        );
        vlog(FILE, "ClientesContent.handleModalSubmit", "substituindo cliente atualizado na lista");
        setClientes((prev) =>
          prev.map((c) =>
            c.cliente_id_origem === updated.cliente_id_origem ? updated : c
          )
        );
        vlog(FILE, "ClientesContent.handleModalSubmit", "exibindo mensagem de sucesso");
        mostrarSucesso(`Cliente "${updated.razao_social}" atualizado com sucesso.`);
      }
    } catch (err) {
      vlog(FILE, "ClientesContent.handleModalSubmit", "falha no envio; verificando se e 404");
      if (err instanceof ApiError && err.status === 404) {
        vlog(FILE, "ClientesContent.handleModalSubmit", "cliente nao encontrado (404)");
        await tratarClienteNaoEncontrado(err);
        return;
      }
      vlog(FILE, "ClientesContent.handleModalSubmit", "verificando se e 403");
      if (err instanceof ApiError && err.status === 403) {
        // Exibido no proprio modal (ClienteModal mostra err.message).
        vlog(FILE, "ClientesContent.handleModalSubmit", "acesso negado (403); repassando erro ao modal");
        throw new ApiError(403, err.message || MSG_SEM_VENDEDOR);
      }
      vlog(FILE, "ClientesContent.handleModalSubmit", "repassando erro ao modal");
      throw err;
    }
    vlog(FILE, "ClientesContent.handleModalSubmit", "fechando modal");
    setModalOpen(false);
  };

  vlog(FILE, "ClientesContent", "montando definicao das colunas da tabela");
  const columns: Column<Cliente>[] = [
    {
      key: "cliente_id_origem",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
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
          {formatCnpj(c.cnpj)}
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
        <span className="text-slate-600">{formatarData(c.data_cadastro)}</span>
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

  // FE-10: contador baseado na pagina exibida; oculto se nada foi carregado
  // (a primeira carga falhou), para nao afirmar "0 clientes".
  vlog(FILE, "ClientesContent", "calculando faixa exibida (pagina=%d, limite=%d, total=%d)", exibida.pagina, exibida.limite, total);
  const { inicio: startItem, fim: endItem } = faixaExibida(
    exibida.pagina,
    exibida.limite,
    total
  );
  vlog(FILE, "ClientesContent", "calculando se oculta o contador (qtd=%d)", clientes.length);
  const ocultarContador = erroCarga && clientes.length === 0;

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
            {!ocultarContador && (
              <div className="text-sm text-slate-500">
                {total === 0
                  ? "0 clientes"
                  : `${startItem}-${endItem} de ${total} ${
                      total === 1 ? "cliente" : "clientes"
                    }`}
              </div>
            )}
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={clientes}
            keyExtractor={(c) => c.cliente_id_origem}
            loading={loading}
            erroCarga={erroCarga}
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

        <Paginador pagina={exibida.pagina} paginas={pages} onIrPara={irParaPagina} />
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
