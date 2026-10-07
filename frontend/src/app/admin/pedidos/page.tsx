"use client";

import { useEffect, useState } from "react";
import {
  faixaExibida,
  mensagemRecargaFalhou,
  useDebounceFiltros,
  useExcluidos,
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
import { PedidoModal } from "@/components/admin/PedidoModal";
import { PedidoItensDetalhe } from "@/components/admin/PedidoItensDetalhe";
import {
  apiListPedidos,
  apiGetPedido,
  apiCreatePedido,
  apiUpdatePedido,
  apiDeletePedido,
} from "@/lib/api";
import { Pedido, PedidoDetalhe, PedidoInput } from "@/lib/types";
import { formatarData } from "@/lib/formatarData";
import { vlog } from "@/lib/vlog";

const FILE = "admin/pedidos/page.tsx";

type SortKey =
  | "pedido_id_origem"
  | "cliente_nome"
  | "vendedor_nome"
  | "data_pedido"
  | "canal"
  | "status"
  | "valor_total";
type SortDir = "asc" | "desc";

const STATUS_OPTIONS = [
  { value: "", label: "Todos os status" },
  { value: "Em separação", label: "Em separação" },
  { value: "Faturado", label: "Faturado" },
  { value: "Entregue", label: "Entregue" },
  { value: "Cancelado", label: "Cancelado" },
];

const CANAL_OPTIONS = [
  { value: "", label: "Todos os canais" },
  { value: "App", label: "App" },
  { value: "Telefone", label: "Telefone" },
  { value: "Visita", label: "Visita" },
  { value: "WhatsApp", label: "WhatsApp" },
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

function fmtValor(v: number): string {
  return currencyFmt.format(v ?? 0);
}

// Mapeia a sortKey interna do frontend para o campo aceito pelo backend em
// order_by. A whitelist do backend (pedidoOrderWhitelist, em
// apis/shared/repositories/pedido_repository.go) usa a chave "id" mapeada
// para a coluna pedido_id_origem, entao a ordenacao por pedido_id_origem
// continua enviando order_by=id.
const ORDER_BY_MAP: Partial<Record<SortKey, string>> = {
  pedido_id_origem: "id",
  cliente_nome: "cliente_nome",
  vendedor_nome: "vendedor_nome",
  data_pedido: "data_pedido",
  canal: "canal",
  status: "status",
  valor_total: "valor_total",
};

const statusColor: Record<string, string> = {
  Faturado: "bg-green-100 text-green-700",
  Entregue: "bg-blue-100 text-blue-700",
  "Em separação": "bg-yellow-100 text-yellow-700",
  Cancelado: "bg-red-100 text-red-700",
};

function PedidosContent() {
  vlog(FILE, "PedidosContent", "inicializando estado da lista de pedidos");
  const [pedidos, setPedidos] = useState<Pedido[]>([]);
  vlog(FILE, "PedidosContent", "inicializando estado de loading");
  const [loading, setLoading] = useState(true);
  vlog(FILE, "PedidosContent", "inicializando estado de erro");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "PedidosContent", "inicializando estado de erro de carga");
  const [erroCarga, setErroCarga] = useState(false);
  vlog(FILE, "PedidosContent", "obtendo controle de mensagem temporaria de sucesso");
  const {
    mensagem: success,
    mostrar: mostrarSucesso,
    limpar: limparSucesso,
  } = useMensagemTemporaria();

  vlog(FILE, "PedidosContent", "inicializando estado da busca textual");
  const [search, setSearch] = useState("");
  vlog(FILE, "PedidosContent", "inicializando estado do filtro de status");
  const [statusFilter, setStatusFilter] = useState("");
  vlog(FILE, "PedidosContent", "inicializando estado do filtro de canal");
  const [canalFilter, setCanalFilter] = useState("");
  vlog(FILE, "PedidosContent", "inicializando estado do filtro de data inicial");
  const [dataInicio, setDataInicio] = useState("");
  vlog(FILE, "PedidosContent", "inicializando estado do filtro de data final");
  const [dataFim, setDataFim] = useState("");

  vlog(FILE, "PedidosContent", "inicializando estado da coluna de ordenacao");
  const [sortKey, setSortKey] = useState<SortKey>("pedido_id_origem");
  vlog(FILE, "PedidosContent", "inicializando estado da direcao de ordenacao");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  vlog(FILE, "PedidosContent", "inicializando estado da pagina atual");
  const [page, setPage] = useState(1);
  vlog(FILE, "PedidosContent", "inicializando estado do limite por pagina");
  const [limit, setLimit] = useState(20);
  vlog(FILE, "PedidosContent", "inicializando estado do total de registros");
  const [total, setTotal] = useState(0);
  vlog(FILE, "PedidosContent", "inicializando estado do total de paginas");
  const [pages, setPages] = useState(0);
  // FE-10: pagina/limite exibidos (a pedida, ou a ultima carregada se a
  // ultima carga falhou).
  vlog(FILE, "PedidosContent", "obtendo pagina exibida (page=%d, limit=%d, erroCarga=%s)", page, limit, erroCarga);
  const { exibida, registrar: registrarCarregada } = usePaginaCarregada(
    page,
    limit,
    erroCarga
  );

  // Master-detail em accordion: pedido selecionado tem seus itens exibidos em
  // uma linha expandida logo abaixo da propria linha na tabela.
  vlog(FILE, "PedidosContent", "inicializando estado do pedido selecionado");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  vlog(FILE, "PedidosContent", "inicializando estado do detalhe selecionado");
  const [selectedDetalhe, setSelectedDetalhe] = useState<PedidoDetalhe | null>(
    null
  );
  vlog(FILE, "PedidosContent", "inicializando estado de loading dos itens");
  const [loadingItens, setLoadingItens] = useState(false);
  vlog(FILE, "PedidosContent", "inicializando estado de erro dos itens");
  const [itensError, setItensError] = useState<string | null>(null);

  // Modal state
  vlog(FILE, "PedidosContent", "inicializando estado de abertura do modal");
  const [modalOpen, setModalOpen] = useState(false);
  vlog(FILE, "PedidosContent", "inicializando estado do modo do modal");
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  vlog(FILE, "PedidosContent", "inicializando estado do pedido em edicao");
  const [editingPedido, setEditingPedido] = useState<PedidoDetalhe | null>(null);
  vlog(FILE, "PedidosContent", "inicializando estado de loading da edicao");
  const [loadingEdit, setLoadingEdit] = useState(false);
  vlog(FILE, "PedidosContent", "inicializando estado do id em exclusao");
  const [deletingId, setDeletingId] = useState<number | null>(null);

  // FE-04: so a busca mais recente aplica o resultado (respostas obsoletas
  // sao descartadas), inclusive entre o efeito e as recargas imperativas.
  vlog(FILE, "PedidosContent", "obtendo executor de busca com descarte de respostas obsoletas");
  const executarBusca = useUltimaResposta();
  // FE-04: linha excluida nao reaparece por uma resposta que saiu antes do
  // DELETE e chegou depois dele.
  vlog(FILE, "PedidosContent", "obtendo controle de ids excluidos");
  const excluidos = useExcluidos((x: Pedido) => x.pedido_id_origem);

  // Busca separada em requisicao pura + aplicacao do resultado no callback
  // assincrono (.then): o efeito nunca chama setState de forma sincrona.
  vlog(FILE, "PedidosContent", "definindo funcao buscarPedidos");
  const buscarPedidos = () => {
    vlog(FILE, "PedidosContent.buscarPedidos", "chamando apiListPedidos (page=%d, limit=%d, sortKey=%s, sortDir=%s)", page, limit, sortKey, sortDir);
    return apiListPedidos(
      page,
      limit,
      {
        status: statusFilter || undefined,
        canal: canalFilter || undefined,
        data_inicio: dataInicio || undefined,
        data_fim: dataFim || undefined,
        q: search.trim() || undefined,
      },
      ORDER_BY_MAP[sortKey],
      sortDir
    );
  };

  vlog(FILE, "PedidosContent", "definindo funcao aplicarPedidos");
  const aplicarPedidos = (res: Awaited<ReturnType<typeof apiListPedidos>>) => {
    vlog(FILE, "PedidosContent.aplicarPedidos", "aplicando lista de pedidos (qtd=%d)", res.data.length);
    setPedidos(res.data);
    vlog(FILE, "PedidosContent.aplicarPedidos", "atualizando total (total=%d)", res.total);
    setTotal(res.total);
    vlog(FILE, "PedidosContent.aplicarPedidos", "atualizando total de paginas (pages=%d)", res.pages);
    setPages(res.pages);
    vlog(FILE, "PedidosContent.aplicarPedidos", "registrando pagina carregada (page=%d, limit=%d)", page, limit);
    registrarCarregada(page, limit);
    vlog(FILE, "PedidosContent.aplicarPedidos", "limpando erro de carga");
    setErroCarga(false);
    vlog(FILE, "PedidosContent.aplicarPedidos", "desativando loading");
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  vlog(FILE, "PedidosContent", "definindo funcao aplicarErroPedidos");
  const aplicarErroPedidos = (err: unknown): string => {
    vlog(FILE, "PedidosContent.aplicarErroPedidos", "extraindo mensagem do erro de carga");
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar pedidos. O endpoint /api/pedidos pode nao existir no backend.";
    vlog(FILE, "PedidosContent.aplicarErroPedidos", "exibindo mensagem de erro");
    setError(message);
    vlog(FILE, "PedidosContent.aplicarErroPedidos", "marcando erro de carga");
    setErroCarga(true);
    vlog(FILE, "PedidosContent.aplicarErroPedidos", "desativando loading");
    setLoading(false);
    return message;
  };

  // Recarga imperativa (handlers e timers). FE-10: devolve a mensagem de erro se a
  // recarga falhar (null se deu certo ou foi superada por outra busca).
  vlog(FILE, "PedidosContent", "definindo funcao loadPedidos");
  const loadPedidos = async (): Promise<string | null> => {
    vlog(FILE, "PedidosContent.loadPedidos", "ativando loading");
    setLoading(true);
    vlog(FILE, "PedidosContent.loadPedidos", "limpando erro");
    setError(null);
    vlog(FILE, "PedidosContent.loadPedidos", "inicializando variavel de falha");
    let falha: string | null = null;
    vlog(FILE, "PedidosContent.loadPedidos", "executando busca de pedidos sem excluidos");
    await executarBusca(buscarPedidos().then(excluidos.filtrar), aplicarPedidos, (err) => {
      vlog(FILE, "PedidosContent.loadPedidos.func", "registrando falha da recarga");
      falha = aplicarErroPedidos(err);
    });
    return falha;
  };

  // FE-10: navegacao a partir da pagina exibida. Se o destino ja e a pagina
  // pedida (a troca anterior falhou), repete a busca em vez de nao fazer nada.
  vlog(FILE, "PedidosContent", "definindo funcao irParaPagina");
  const irParaPagina = (n: number) => {
    vlog(FILE, "PedidosContent.irParaPagina", "verificando se destino e a pagina atual (destino=%d, atual=%d)", n, page);
    if (n === page) loadPedidos();
    else setPage(n);
  };

  // Paginacao/ordenacao mudou: liga o loading durante o render (padrao
  // "ajustar estado quando a entrada muda") e o efeito so faz a busca.
  vlog(FILE, "PedidosContent", "montando chave de paginacao/ordenacao");
  const chaveLista = `${page}|${limit}|${sortKey}|${sortDir}`;
  vlog(FILE, "PedidosContent", "inicializando estado da chave anterior");
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  vlog(FILE, "PedidosContent", "verificando se a chave de paginacao/ordenacao mudou");
  if (chaveAnterior !== chaveLista) {
    vlog(FILE, "PedidosContent", "atualizando chave anterior");
    setChaveAnterior(chaveLista);
    vlog(FILE, "PedidosContent", "ativando loading");
    setLoading(true);
    vlog(FILE, "PedidosContent", "limpando erro");
    setError(null);
  }

  vlog(FILE, "PedidosContent", "registrando efeito de busca por paginacao/ordenacao");
  useEffect(() => {
    vlog(FILE, "PedidosContent.useEffect", "executando busca de pedidos sem excluidos");
    executarBusca(buscarPedidos().then(excluidos.filtrar), aplicarPedidos, aplicarErroPedidos);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual e reset para pagina 1 quando filtros mudam
  // FE-04: o debounce nao dispara na montagem, so quando a chave dos filtros
  // muda (evita a busca dupla e a tabela voltando para a pagina 1).
  vlog(FILE, "PedidosContent", "montando chave dos filtros");
  const chaveFiltros = JSON.stringify([search, statusFilter, canalFilter, dataInicio, dataFim]);
  vlog(FILE, "PedidosContent", "registrando debounce dos filtros");
  useDebounceFiltros(chaveFiltros, () => {
    vlog(FILE, "PedidosContent.useDebounceFiltros", "filtros mudaram; verificando se esta na pagina 1 (page=%d)", page);
    if (page !== 1) {
      vlog(FILE, "PedidosContent.useDebounceFiltros", "voltando para a pagina 1");
      setPage(1);
    } else {
      vlog(FILE, "PedidosContent.useDebounceFiltros", "recarregando pedidos");
      loadPedidos();
    }
  });

  vlog(FILE, "PedidosContent", "definindo handler handleSort");
  const handleSort = (key: SortKey) => {
    vlog(FILE, "PedidosContent.handleSort", "verificando se coluna ja e a ordenada (key=%s)", key);
    if (sortKey === key) {
      vlog(FILE, "PedidosContent.handleSort", "invertendo direcao da ordenacao");
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      vlog(FILE, "PedidosContent.handleSort", "definindo nova coluna de ordenacao");
      setSortKey(key);
      vlog(FILE, "PedidosContent.handleSort", "definindo direcao asc");
      setSortDir("asc");
    }
  };

  vlog(FILE, "PedidosContent", "definindo funcao loadItens");
  const loadItens = async (id: number) => {
    vlog(FILE, "PedidosContent.loadItens", "ativando loading dos itens (pedido_id=%d)", id);
    setLoadingItens(true);
    vlog(FILE, "PedidosContent.loadItens", "limpando erro dos itens");
    setItensError(null);
    vlog(FILE, "PedidosContent.loadItens", "iniciando carga do detalhe do pedido");
    try {
      vlog(FILE, "PedidosContent.loadItens", "chamando apiGetPedido");
      const detalhe = await apiGetPedido(id);
      vlog(FILE, "PedidosContent.loadItens", "aplicando detalhe do pedido");
      setSelectedDetalhe(detalhe);
    } catch (err) {
      vlog(FILE, "PedidosContent.loadItens", "falha ao carregar itens; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao carregar itens do pedido.";
      vlog(FILE, "PedidosContent.loadItens", "exibindo erro dos itens");
      setItensError(message);
      vlog(FILE, "PedidosContent.loadItens", "limpando detalhe selecionado");
      setSelectedDetalhe(null);
    } finally {
      vlog(FILE, "PedidosContent.loadItens", "desativando loading dos itens");
      setLoadingItens(false);
    }
  };

  vlog(FILE, "PedidosContent", "definindo funcao fecharItens");
  const fecharItens = () => {
    vlog(FILE, "PedidosContent.fecharItens", "limpando pedido selecionado");
    setSelectedId(null);
    vlog(FILE, "PedidosContent.fecharItens", "limpando detalhe selecionado");
    setSelectedDetalhe(null);
    vlog(FILE, "PedidosContent.fecharItens", "limpando erro dos itens");
    setItensError(null);
  };

  vlog(FILE, "PedidosContent", "definindo funcao toggleItens");
  const toggleItens = (pedido: Pedido) => {
    vlog(FILE, "PedidosContent.toggleItens", "verificando se pedido ja esta expandido (pedido_id=%d)", pedido.pedido_id_origem);
    if (selectedId === pedido.pedido_id_origem) {
      vlog(FILE, "PedidosContent.toggleItens", "fechando itens do pedido");
      fecharItens();
      return;
    }
    vlog(FILE, "PedidosContent.toggleItens", "selecionando pedido");
    setSelectedId(pedido.pedido_id_origem);
    vlog(FILE, "PedidosContent.toggleItens", "limpando detalhe anterior");
    setSelectedDetalhe(null);
    vlog(FILE, "PedidosContent.toggleItens", "carregando itens do pedido");
    loadItens(pedido.pedido_id_origem);
  };

  vlog(FILE, "PedidosContent", "definindo handler openCreate");
  const openCreate = () => {
    vlog(FILE, "PedidosContent.openCreate", "definindo modo create");
    setModalMode("create");
    vlog(FILE, "PedidosContent.openCreate", "limpando pedido em edicao");
    setEditingPedido(null);
    vlog(FILE, "PedidosContent.openCreate", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "PedidosContent", "definindo handler openEdit");
  const openEdit = async (pedido: Pedido) => {
    vlog(FILE, "PedidosContent.openEdit", "limpando erro (pedido_id=%d)", pedido.pedido_id_origem);
    setError(null);
    vlog(FILE, "PedidosContent.openEdit", "ativando loading da edicao");
    setLoadingEdit(true);
    vlog(FILE, "PedidosContent.openEdit", "iniciando carga do pedido para edicao");
    try {
      vlog(FILE, "PedidosContent.openEdit", "chamando apiGetPedido");
      const detalhe = await apiGetPedido(pedido.pedido_id_origem);
      vlog(FILE, "PedidosContent.openEdit", "definindo modo edit");
      setModalMode("edit");
      vlog(FILE, "PedidosContent.openEdit", "definindo pedido em edicao");
      setEditingPedido(detalhe);
      vlog(FILE, "PedidosContent.openEdit", "abrindo modal");
      setModalOpen(true);
    } catch (err) {
      vlog(FILE, "PedidosContent.openEdit", "falha ao carregar pedido; extraindo mensagem do erro");
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao carregar pedido para edicao.";
      vlog(FILE, "PedidosContent.openEdit", "exibindo mensagem de erro");
      setError(message);
    } finally {
      vlog(FILE, "PedidosContent.openEdit", "desativando loading da edicao");
      setLoadingEdit(false);
    }
  };

  vlog(FILE, "PedidosContent", "definindo handler handleModalSubmit");
  const handleModalSubmit = async (data: PedidoInput) => {
    vlog(FILE, "PedidosContent.handleModalSubmit", "limpando erro");
    setError(null);
    vlog(FILE, "PedidosContent.handleModalSubmit", "verificando modo do modal (modo=%s)", modalMode);
    if (modalMode === "create") {
      vlog(FILE, "PedidosContent.handleModalSubmit", "chamando apiCreatePedido");
      const created = await apiCreatePedido(data);
      vlog(FILE, "PedidosContent.handleModalSubmit", "montando mensagem de sucesso (pedido_id=%d)", created.pedido_id_origem);
      const msg = `Pedido #${created.pedido_id_origem} criado com sucesso.`;
      vlog(FILE, "PedidosContent.handleModalSubmit", "recarregando pedidos apos criacao");
      const erroRecarga = await loadPedidos();
      vlog(FILE, "PedidosContent.handleModalSubmit", "verificando falha na recarga (falhou=%s)", erroRecarga !== null);
      if (erroRecarga) setError(mensagemRecargaFalhou(msg, erroRecarga));
      else mostrarSucesso(msg);
    } else if (editingPedido) {
      vlog(FILE, "PedidosContent.handleModalSubmit", "chamando apiUpdatePedido (pedido_id=%d)", editingPedido.pedido_id_origem);
      const updated = await apiUpdatePedido(
        editingPedido.pedido_id_origem,
        data
      );
      vlog(FILE, "PedidosContent.handleModalSubmit", "substituindo pedido atualizado na lista");
      setPedidos((prev) =>
        prev.map((p) =>
          p.pedido_id_origem === updated.pedido_id_origem ? updated : p
        )
      );
      vlog(FILE, "PedidosContent.handleModalSubmit", "verificando se o pedido atualizado esta expandido");
      if (selectedId === updated.pedido_id_origem) {
        vlog(FILE, "PedidosContent.handleModalSubmit", "atualizando detalhe expandido");
        setSelectedDetalhe(updated);
      }
      vlog(FILE, "PedidosContent.handleModalSubmit", "exibindo mensagem de sucesso");
      mostrarSucesso(`Pedido #${updated.pedido_id_origem} atualizado com sucesso.`);
    }
    vlog(FILE, "PedidosContent.handleModalSubmit", "fechando modal");
    setModalOpen(false);
  };

  vlog(FILE, "PedidosContent", "definindo handler handleDelete");
  const handleDelete = async (pedido: Pedido) => {
    vlog(FILE, "PedidosContent.handleDelete", "pedindo confirmacao de exclusao (pedido_id=%d)", pedido.pedido_id_origem);
    const confirmed = window.confirm(
      `Tem certeza que deseja excluir o pedido #${pedido.pedido_id_origem} - ${pedido.cliente_nome}?`
    );
    vlog(FILE, "PedidosContent.handleDelete", "verificando confirmacao (ok=%s)", confirmed);
    if (!confirmed) return;

    vlog(FILE, "PedidosContent.handleDelete", "limpando erro");
    setError(null);
    vlog(FILE, "PedidosContent.handleDelete", "marcando id em exclusao");
    setDeletingId(pedido.pedido_id_origem);
    vlog(FILE, "PedidosContent.handleDelete", "iniciando chamada de exclusao");
    try {
      vlog(FILE, "PedidosContent.handleDelete", "chamando apiDeletePedido");
      await apiDeletePedido(pedido.pedido_id_origem);
      vlog(FILE, "PedidosContent.handleDelete", "marcando id como excluido");
      excluidos.marcar(pedido.pedido_id_origem);
      vlog(FILE, "PedidosContent.handleDelete", "removendo pedido da lista");
      setPedidos((prev) =>
        prev.filter((p) => p.pedido_id_origem !== pedido.pedido_id_origem)
      );
      vlog(FILE, "PedidosContent.handleDelete", "verificando se o pedido excluido estava expandido");
      if (selectedId === pedido.pedido_id_origem) {
        vlog(FILE, "PedidosContent.handleDelete", "fechando itens do pedido excluido");
        fecharItens();
      }
      vlog(FILE, "PedidosContent.handleDelete", "decrementando total");
      setTotal((t) => Math.max(0, t - 1));
      vlog(FILE, "PedidosContent.handleDelete", "exibindo mensagem de sucesso");
      mostrarSucesso(`Pedido #${pedido.pedido_id_origem} excluido com sucesso.`);
    } catch (err) {
      vlog(FILE, "PedidosContent.handleDelete", "falha na exclusao; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao excluir pedido.";
      vlog(FILE, "PedidosContent.handleDelete", "exibindo mensagem de erro");
      setError(message);
    } finally {
      vlog(FILE, "PedidosContent.handleDelete", "limpando id em exclusao");
      setDeletingId(null);
    }
  };

  vlog(FILE, "PedidosContent", "montando definicao das colunas da tabela");
  const columns: Column<Pedido>[] = [
    {
      key: "pedido_id_origem",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      render: (p) => (
        <span className="font-mono text-xs">#{p.pedido_id_origem}</span>
      ),
    },
    {
      key: "cliente_nome",
      header: "Cliente",
      sortable: true,
      render: (p) => (
        <button
          type="button"
          onClick={() => toggleItens(p)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Ver itens do pedido"
        >
          {p.cliente_nome}
        </button>
      ),
    },
    {
      key: "vendedor_nome",
      header: "Vendedor",
      width: "160px",
      sortable: true,
      render: (p) => <span className="text-slate-600">{p.vendedor_nome}</span>,
    },
    {
      key: "data_pedido",
      header: "Data",
      width: "120px",
      align: "center",
      sortable: true,
      render: (p) => (
        <span className="text-slate-600">{formatarData(p.data_pedido)}</span>
      ),
    },
    {
      key: "canal",
      header: "Canal",
      width: "110px",
      align: "center",
      sortable: true,
      render: (p) => <span className="text-slate-600">{p.canal}</span>,
    },
    {
      key: "status",
      header: "Status",
      width: "140px",
      align: "center",
      sortable: true,
      render: (p) => (
        <span
          className={[
            "inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium",
            statusColor[p.status] || "bg-slate-100 text-slate-700",
          ].join(" ")}
        >
          {p.status}
        </span>
      ),
    },
    {
      key: "valor_total",
      header: "Valor Total",
      width: "140px",
      align: "right",
      sortable: true,
      render: (p) => (
        <span className="text-slate-700">{fmtValor(p.valor_total)}</span>
      ),
    },
    {
      key: "actions",
      header: "Acoes",
      width: "230px",
      align: "right",
      render: (p) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => toggleItens(p)}
            title="Ver itens do pedido"
          >
            {selectedId === p.pedido_id_origem ? "Ocultar itens" : "Itens"}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(p)}
            disabled={loadingEdit}
            title="Editar pedido"
          >
            Editar
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => handleDelete(p)}
            disabled={deletingId === p.pedido_id_origem}
            title="Excluir pedido"
          >
            {deletingId === p.pedido_id_origem ? "Excluindo..." : "Excluir"}
          </Button>
        </div>
      ),
    },
  ];

  // FE-10: contador baseado na pagina exibida; oculto se nada foi carregado
  // (a primeira carga falhou), para nao afirmar "0 pedidos".
  vlog(FILE, "PedidosContent", "calculando faixa exibida (pagina=%d, limite=%d, total=%d)", exibida.pagina, exibida.limite, total);
  const { inicio: startItem, fim: endItem } = faixaExibida(
    exibida.pagina,
    exibida.limite,
    total
  );
  vlog(FILE, "PedidosContent", "calculando se oculta o contador (qtd=%d)", pedidos.length);
  const ocultarContador = erroCarga && pedidos.length === 0;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Pedidos</h1>
          <p className="mt-1 text-sm text-slate-500">
            Consulte pedidos e seus itens, e gerencie novos pedidos.
          </p>
        </div>
        <Button onClick={openCreate}>+ Novo Pedido</Button>
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
                  placeholder="Razao social do cliente..."
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
                <Select
                  label="Status"
                  options={STATUS_OPTIONS}
                  value={statusFilter}
                  onChange={(e) => setStatusFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-40">
                <Select
                  label="Canal"
                  options={CANAL_OPTIONS}
                  value={canalFilter}
                  onChange={(e) => setCanalFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-40">
                <Input
                  label="De"
                  type="date"
                  value={dataInicio}
                  onChange={(e) => setDataInicio(e.target.value)}
                />
              </div>
              <div className="sm:w-40">
                <Input
                  label="Ate"
                  type="date"
                  value={dataFim}
                  onChange={(e) => setDataFim(e.target.value)}
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
                  ? "0 pedidos"
                  : `${startItem}-${endItem} de ${total} ${
                      total === 1 ? "pedido" : "pedidos"
                    }`}
              </div>
            )}
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={pedidos}
            keyExtractor={(p) => p.pedido_id_origem}
            loading={loading}
            erroCarga={erroCarga}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            renderExpanded={(p) =>
              selectedId === p.pedido_id_origem ? (
                <PedidoItensDetalhe
                  pedidoId={p.pedido_id_origem}
                  detalhe={selectedDetalhe}
                  loading={loadingItens}
                  error={itensError}
                  onClose={fecharItens}
                />
              ) : null
            }
            emptyMessage={
              search || statusFilter || canalFilter || dataInicio || dataFim
                ? "Nenhum pedido encontrado para os filtros aplicados."
                : "Nenhum pedido cadastrado."
            }
          />
        </div>

        <Paginador pagina={exibida.pagina} paginas={pages} onIrPara={irParaPagina} />
      </Card>

      <div className="mt-4 text-xs text-slate-400">
        <strong>Nota:</strong> A busca e os filtros de status/canal/periodo sao
        aplicados via API. Caso a lista esteja vazia ou retorne erro, verifique
        se os endpoints <code>GET /api/pedidos</code> e{" "}
        <code>GET /api/pedidos/&#123;id&#125;</code> estao implementados no
        backend.
      </div>

      <PedidoModal
        open={modalOpen}
        mode={modalMode}
        pedido={editingPedido}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}

export default function PedidosPage() {
  return (
    <CarteiraGuard title="Pedidos">
      <PedidosContent />
    </CarteiraGuard>
  );
}
