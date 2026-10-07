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
import { Navbar } from "@/components/layout/Navbar";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { CarteiraGuard } from "@/components/layout/CarteiraGuard";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Column, Badge } from "@/components/ui/Table";
import { Paginador } from "@/components/ui/Paginador";
import { useMensagemTemporaria } from "@/lib/useMensagemTemporaria";
import { PagamentoModal } from "@/components/PagamentoModal";
import {
  apiListPagamentos,
  apiCreatePagamento,
  apiUpdatePagamento,
  apiDeletePagamento,
} from "@/lib/api";
import {
  Pagamento,
  PagamentoCreateInput,
  PagamentoUpdateInput,
} from "@/lib/types";
import { formatarData } from "@/lib/formatarData";
import { vlog } from "@/lib/vlog";

const FILE = "pagamentos/page.tsx";

type SortKey =
  | "pagamento_id"
  | "pedido_id"
  | "forma_pagamento"
  | "valor"
  | "valor_liquido"
  | "data_vencimento"
  | "data_pagamento"
  | "status_pagamento";
type SortDir = "asc" | "desc";

const STATUS_FILTER_OPTIONS = [
  { value: "", label: "Todos os status" },
  { value: "Em aberto", label: "Em aberto" },
  { value: "Inadimplente", label: "Inadimplente" },
  { value: "Pago", label: "Pago" },
  { value: "Pago com atraso", label: "Pago com atraso" },
];

const FORMA_FILTER_OPTIONS = [
  { value: "", label: "Todas as formas" },
  { value: "Boleto 14 dias", label: "Boleto 14 dias" },
  { value: "Boleto 28 dias", label: "Boleto 28 dias" },
  { value: "Cartão de crédito", label: "Cartão de crédito" },
  { value: "Cartão de débito", label: "Cartão de débito" },
  { value: "Cheque a prazo", label: "Cheque a prazo" },
  { value: "Dinheiro", label: "Dinheiro" },
  { value: "PIX", label: "PIX" },
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

function statusBadgeColor(status: string): "green" | "yellow" | "red" | "gray" {
  switch (status) {
    case "Pago":
      return "green";
    case "Em aberto":
      return "yellow";
    case "Inadimplente":
    case "Pago com atraso":
      return "red";
    default:
      return "gray";
  }
}

function PagamentosContent() {
  vlog(FILE, "PagamentosContent", "inicializando estado da lista de pagamentos");
  const [pagamentos, setPagamentos] = useState<Pagamento[]>([]);
  vlog(FILE, "PagamentosContent", "inicializando estado de loading");
  const [loading, setLoading] = useState(true);
  vlog(FILE, "PagamentosContent", "inicializando estado de erro");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "PagamentosContent", "inicializando estado de erro de carga");
  const [erroCarga, setErroCarga] = useState(false);
  vlog(FILE, "PagamentosContent", "obtendo controle de mensagem temporaria de sucesso");
  const {
    mensagem: success,
    mostrar: mostrarSucesso,
    limpar: limparSucesso,
  } = useMensagemTemporaria();

  vlog(FILE, "PagamentosContent", "inicializando estado do filtro de status");
  const [statusFilter, setStatusFilter] = useState("");
  vlog(FILE, "PagamentosContent", "inicializando estado do filtro de forma de pagamento");
  const [formaFilter, setFormaFilter] = useState("");
  vlog(FILE, "PagamentosContent", "inicializando estado do filtro de pedido");
  const [pedidoIdFilter, setPedidoIdFilter] = useState("");
  vlog(FILE, "PagamentosContent", "inicializando estado do filtro de vencimento inicial");
  const [vencimentoDe, setVencimentoDe] = useState("");
  vlog(FILE, "PagamentosContent", "inicializando estado do filtro de vencimento final");
  const [vencimentoAte, setVencimentoAte] = useState("");

  vlog(FILE, "PagamentosContent", "inicializando estado da coluna de ordenacao");
  const [sortKey, setSortKey] = useState<SortKey>("pagamento_id");
  vlog(FILE, "PagamentosContent", "inicializando estado da direcao de ordenacao");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  vlog(FILE, "PagamentosContent", "inicializando estado da pagina atual");
  const [page, setPage] = useState(1);
  vlog(FILE, "PagamentosContent", "inicializando estado do limite por pagina");
  const [limit, setLimit] = useState(20);
  vlog(FILE, "PagamentosContent", "inicializando estado do total de registros");
  const [total, setTotal] = useState(0);
  vlog(FILE, "PagamentosContent", "inicializando estado do total de paginas");
  const [pages, setPages] = useState(0);
  // FE-10: pagina/limite exibidos (a pedida, ou a ultima carregada se a
  // ultima carga falhou).
  vlog(FILE, "PagamentosContent", "obtendo pagina exibida (page=%d, limit=%d, erroCarga=%s)", page, limit, erroCarga);
  const { exibida, registrar: registrarCarregada } = usePaginaCarregada(
    page,
    limit,
    erroCarga
  );

  vlog(FILE, "PagamentosContent", "inicializando estado de abertura do modal");
  const [modalOpen, setModalOpen] = useState(false);
  vlog(FILE, "PagamentosContent", "inicializando estado do modo do modal");
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  vlog(FILE, "PagamentosContent", "inicializando estado do pagamento em edicao");
  const [editingPagamento, setEditingPagamento] = useState<Pagamento | null>(
    null
  );
  vlog(FILE, "PagamentosContent", "inicializando estado do id em exclusao");
  const [deletingId, setDeletingId] = useState<number | null>(null);

  // FE-04: so a busca mais recente aplica o resultado (respostas obsoletas
  // sao descartadas), inclusive entre o efeito e as recargas imperativas.
  vlog(FILE, "PagamentosContent", "obtendo executor de busca com descarte de respostas obsoletas");
  const executarBusca = useUltimaResposta();
  // FE-04: linha excluida nao reaparece por uma resposta que saiu antes do
  // DELETE e chegou depois dele.
  vlog(FILE, "PagamentosContent", "obtendo controle de ids excluidos");
  const excluidos = useExcluidos((x: Pagamento) => x.pagamento_id);

  // Busca separada em requisicao pura + aplicacao do resultado no callback
  // assincrono (.then): o efeito nunca chama setState de forma sincrona.
  vlog(FILE, "PagamentosContent", "definindo funcao buscarPagamentos");
  const buscarPagamentos = () => {
    vlog(FILE, "PagamentosContent.buscarPagamentos", "convertendo filtro de pedido para numero");
    const pedidoIdNum = pedidoIdFilter.trim()
      ? Number(pedidoIdFilter.trim())
      : undefined;
    vlog(FILE, "PagamentosContent.buscarPagamentos", "chamando apiListPagamentos (page=%d, limit=%d, sortKey=%s, sortDir=%s)", page, limit, sortKey, sortDir);
    return apiListPagamentos(
      page,
      limit,
      {
        status_pagamento: statusFilter || undefined,
        forma_pagamento: formaFilter || undefined,
        pedido_id:
          pedidoIdNum !== undefined && !Number.isNaN(pedidoIdNum)
            ? pedidoIdNum
            : undefined,
        vencimento_de: vencimentoDe || undefined,
        vencimento_ate: vencimentoAte || undefined,
      },
      sortKey,
      sortDir
    );
  };

  vlog(FILE, "PagamentosContent", "definindo funcao aplicarPagamentos");
  const aplicarPagamentos = (res: Awaited<ReturnType<typeof apiListPagamentos>>) => {
    vlog(FILE, "PagamentosContent.aplicarPagamentos", "aplicando lista de pagamentos (qtd=%d)", res.data.length);
    setPagamentos(res.data);
    vlog(FILE, "PagamentosContent.aplicarPagamentos", "atualizando total (total=%d)", res.total);
    setTotal(res.total);
    vlog(FILE, "PagamentosContent.aplicarPagamentos", "atualizando total de paginas (pages=%d)", res.pages);
    setPages(res.pages);
    vlog(FILE, "PagamentosContent.aplicarPagamentos", "registrando pagina carregada (page=%d, limit=%d)", page, limit);
    registrarCarregada(page, limit);
    vlog(FILE, "PagamentosContent.aplicarPagamentos", "limpando erro de carga");
    setErroCarga(false);
    vlog(FILE, "PagamentosContent.aplicarPagamentos", "desativando loading");
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  vlog(FILE, "PagamentosContent", "definindo funcao aplicarErroPagamentos");
  const aplicarErroPagamentos = (err: unknown): string => {
    vlog(FILE, "PagamentosContent.aplicarErroPagamentos", "extraindo mensagem do erro de carga");
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar pagamentos. O endpoint /api/pagamentos pode nao existir no backend.";
    vlog(FILE, "PagamentosContent.aplicarErroPagamentos", "exibindo mensagem de erro");
    setError(message);
    vlog(FILE, "PagamentosContent.aplicarErroPagamentos", "marcando erro de carga");
    setErroCarga(true);
    vlog(FILE, "PagamentosContent.aplicarErroPagamentos", "desativando loading");
    setLoading(false);
    return message;
  };

  // Recarga imperativa (handlers e timers). FE-10: devolve a mensagem de erro se a
  // recarga falhar (null se deu certo ou foi superada por outra busca).
  vlog(FILE, "PagamentosContent", "definindo funcao loadPagamentos");
  const loadPagamentos = async (): Promise<string | null> => {
    vlog(FILE, "PagamentosContent.loadPagamentos", "ativando loading");
    setLoading(true);
    vlog(FILE, "PagamentosContent.loadPagamentos", "limpando erro");
    setError(null);
    vlog(FILE, "PagamentosContent.loadPagamentos", "inicializando variavel de falha");
    let falha: string | null = null;
    vlog(FILE, "PagamentosContent.loadPagamentos", "executando busca de pagamentos sem excluidos");
    await executarBusca(buscarPagamentos().then(excluidos.filtrar), aplicarPagamentos, (err) => {
      vlog(FILE, "PagamentosContent.loadPagamentos.func", "registrando falha da recarga");
      falha = aplicarErroPagamentos(err);
    });
    return falha;
  };

  // FE-10: navegacao a partir da pagina exibida. Se o destino ja e a pagina
  // pedida (a troca anterior falhou), repete a busca em vez de nao fazer nada.
  vlog(FILE, "PagamentosContent", "definindo funcao irParaPagina");
  const irParaPagina = (n: number) => {
    vlog(FILE, "PagamentosContent.irParaPagina", "verificando se destino e a pagina atual (destino=%d, atual=%d)", n, page);
    if (n === page) loadPagamentos();
    else setPage(n);
  };

  // Paginacao/ordenacao mudou: liga o loading durante o render (padrao
  // "ajustar estado quando a entrada muda") e o efeito so faz a busca.
  vlog(FILE, "PagamentosContent", "montando chave de paginacao/ordenacao");
  const chaveLista = `${page}|${limit}|${sortKey}|${sortDir}`;
  vlog(FILE, "PagamentosContent", "inicializando estado da chave anterior");
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  vlog(FILE, "PagamentosContent", "verificando se a chave de paginacao/ordenacao mudou");
  if (chaveAnterior !== chaveLista) {
    vlog(FILE, "PagamentosContent", "atualizando chave anterior");
    setChaveAnterior(chaveLista);
    vlog(FILE, "PagamentosContent", "ativando loading");
    setLoading(true);
    vlog(FILE, "PagamentosContent", "limpando erro");
    setError(null);
  }

  vlog(FILE, "PagamentosContent", "registrando efeito de busca por paginacao/ordenacao");
  useEffect(() => {
    vlog(FILE, "PagamentosContent.useEffect", "executando busca de pagamentos sem excluidos");
    executarBusca(buscarPagamentos().then(excluidos.filtrar), aplicarPagamentos, aplicarErroPagamentos);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce dos filtros e reset para pagina 1 quando eles mudam
  // FE-04: o debounce nao dispara na montagem, so quando a chave dos filtros
  // muda (evita a busca dupla e a tabela voltando para a pagina 1).
  vlog(FILE, "PagamentosContent", "montando chave dos filtros");
  const chaveFiltros = JSON.stringify([statusFilter, formaFilter, pedidoIdFilter, vencimentoDe, vencimentoAte]);
  vlog(FILE, "PagamentosContent", "registrando debounce dos filtros");
  useDebounceFiltros(chaveFiltros, () => {
    vlog(FILE, "PagamentosContent.useDebounceFiltros", "filtros mudaram; verificando se esta na pagina 1 (page=%d)", page);
    if (page !== 1) {
      vlog(FILE, "PagamentosContent.useDebounceFiltros", "voltando para a pagina 1");
      setPage(1);
    } else {
      vlog(FILE, "PagamentosContent.useDebounceFiltros", "recarregando pagamentos");
      loadPagamentos();
    }
  });

  vlog(FILE, "PagamentosContent", "definindo handler handleSort");
  const handleSort = (key: SortKey) => {
    vlog(FILE, "PagamentosContent.handleSort", "verificando se coluna ja e a ordenada (key=%s)", key);
    if (sortKey === key) {
      vlog(FILE, "PagamentosContent.handleSort", "invertendo direcao da ordenacao");
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      vlog(FILE, "PagamentosContent.handleSort", "definindo nova coluna de ordenacao");
      setSortKey(key);
      vlog(FILE, "PagamentosContent.handleSort", "definindo direcao asc");
      setSortDir("asc");
    }
  };

  vlog(FILE, "PagamentosContent", "definindo handler openCreate");
  const openCreate = () => {
    vlog(FILE, "PagamentosContent.openCreate", "definindo modo create");
    setModalMode("create");
    vlog(FILE, "PagamentosContent.openCreate", "limpando pagamento em edicao");
    setEditingPagamento(null);
    vlog(FILE, "PagamentosContent.openCreate", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "PagamentosContent", "definindo handler openEdit");
  const openEdit = (pagamento: Pagamento) => {
    vlog(FILE, "PagamentosContent.openEdit", "definindo modo edit (pagamento_id=%d)", pagamento.pagamento_id);
    setModalMode("edit");
    vlog(FILE, "PagamentosContent.openEdit", "definindo pagamento em edicao");
    setEditingPagamento(pagamento);
    vlog(FILE, "PagamentosContent.openEdit", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "PagamentosContent", "definindo handler handleModalSubmit");
  const handleModalSubmit = async (
    data: PagamentoCreateInput | PagamentoUpdateInput
  ) => {
    vlog(FILE, "PagamentosContent.handleModalSubmit", "limpando erro");
    setError(null);
    vlog(FILE, "PagamentosContent.handleModalSubmit", "verificando modo do modal (modo=%s)", modalMode);
    if (modalMode === "create") {
      vlog(FILE, "PagamentosContent.handleModalSubmit", "chamando apiCreatePagamento");
      const created = await apiCreatePagamento(data as PagamentoCreateInput);
      vlog(FILE, "PagamentosContent.handleModalSubmit", "montando mensagem de sucesso (pagamento_id=%d)", created.pagamento_id);
      const msg = `Pagamento #${created.pagamento_id} criado com sucesso.`;
      vlog(FILE, "PagamentosContent.handleModalSubmit", "recarregando pagamentos apos criacao");
      const erroRecarga = await loadPagamentos();
      vlog(FILE, "PagamentosContent.handleModalSubmit", "verificando falha na recarga (falhou=%s)", erroRecarga !== null);
      if (erroRecarga) setError(mensagemRecargaFalhou(msg, erroRecarga));
      else mostrarSucesso(msg);
    } else if (editingPagamento) {
      vlog(FILE, "PagamentosContent.handleModalSubmit", "chamando apiUpdatePagamento (pagamento_id=%d)", editingPagamento.pagamento_id);
      const updated = await apiUpdatePagamento(
        editingPagamento.pagamento_id,
        data as PagamentoUpdateInput
      );
      vlog(FILE, "PagamentosContent.handleModalSubmit", "substituindo pagamento atualizado na lista");
      setPagamentos((prev) =>
        prev.map((p) =>
          p.pagamento_id === updated.pagamento_id ? updated : p
        )
      );
      vlog(FILE, "PagamentosContent.handleModalSubmit", "exibindo mensagem de sucesso");
      mostrarSucesso(`Pagamento #${updated.pagamento_id} atualizado com sucesso.`);
    }
    vlog(FILE, "PagamentosContent.handleModalSubmit", "fechando modal");
    setModalOpen(false);
  };

  vlog(FILE, "PagamentosContent", "definindo handler handleDelete");
  const handleDelete = async (pagamento: Pagamento) => {
    vlog(FILE, "PagamentosContent.handleDelete", "pedindo confirmacao de exclusao (pagamento_id=%d)", pagamento.pagamento_id);
    const confirmed = window.confirm(
      `Tem certeza que deseja excluir o pagamento #${pagamento.pagamento_id} - ${pagamento.forma_pagamento}?`
    );
    vlog(FILE, "PagamentosContent.handleDelete", "verificando confirmacao (ok=%s)", confirmed);
    if (!confirmed) return;

    vlog(FILE, "PagamentosContent.handleDelete", "limpando erro");
    setError(null);
    vlog(FILE, "PagamentosContent.handleDelete", "marcando id em exclusao");
    setDeletingId(pagamento.pagamento_id);
    vlog(FILE, "PagamentosContent.handleDelete", "iniciando chamada de exclusao");
    try {
      vlog(FILE, "PagamentosContent.handleDelete", "chamando apiDeletePagamento");
      await apiDeletePagamento(pagamento.pagamento_id);
      vlog(FILE, "PagamentosContent.handleDelete", "marcando id como excluido");
      excluidos.marcar(pagamento.pagamento_id);
      vlog(FILE, "PagamentosContent.handleDelete", "removendo pagamento da lista");
      setPagamentos((prev) =>
        prev.filter((p) => p.pagamento_id !== pagamento.pagamento_id)
      );
      vlog(FILE, "PagamentosContent.handleDelete", "decrementando total");
      setTotal((t) => Math.max(0, t - 1));
      vlog(FILE, "PagamentosContent.handleDelete", "exibindo mensagem de sucesso");
      mostrarSucesso(`Pagamento #${pagamento.pagamento_id} excluido com sucesso.`);
    } catch (err) {
      vlog(FILE, "PagamentosContent.handleDelete", "falha na exclusao; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao excluir pagamento.";
      vlog(FILE, "PagamentosContent.handleDelete", "exibindo mensagem de erro");
      setError(message);
    } finally {
      vlog(FILE, "PagamentosContent.handleDelete", "limpando id em exclusao");
      setDeletingId(null);
    }
  };

  vlog(FILE, "PagamentosContent", "montando definicao das colunas da tabela");
  const columns: Column<Pagamento>[] = [
    {
      key: "pagamento_id",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      render: (p) => (
        <span className="font-mono text-xs">#{p.pagamento_id}</span>
      ),
    },
    {
      key: "pedido_id",
      header: "Pedido",
      width: "100px",
      sortable: true,
      render: (p) => (
        <span className="text-slate-600">#{p.pedido_id}</span>
      ),
    },
    {
      key: "forma_pagamento",
      header: "Forma",
      width: "160px",
      sortable: true,
      render: (p) => (
        <button
          type="button"
          onClick={() => openEdit(p)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Editar pagamento"
        >
          {p.forma_pagamento}
        </button>
      ),
    },
    {
      key: "valor",
      header: "Valor",
      width: "130px",
      align: "right",
      sortable: true,
      render: (p) => <span className="text-slate-700">{fmtValor(p.valor)}</span>,
    },
    {
      key: "valor_liquido",
      header: "Valor liquido",
      width: "130px",
      align: "right",
      sortable: true,
      render: (p) => (
        <span className="text-slate-700">{fmtValor(p.valor_liquido)}</span>
      ),
    },
    {
      key: "data_vencimento",
      header: "Vencimento",
      width: "120px",
      sortable: true,
      render: (p) => (
        <span className="text-slate-600">{formatarData(p.data_vencimento)}</span>
      ),
    },
    {
      key: "data_pagamento",
      header: "Pagamento em",
      width: "120px",
      sortable: true,
      render: (p) => (
        <span className="text-slate-600">{formatarData(p.data_pagamento)}</span>
      ),
    },
    {
      key: "status_pagamento",
      header: "Status",
      width: "150px",
      align: "center",
      sortable: true,
      render: (p) => (
        <Badge color={statusBadgeColor(p.status_pagamento)}>
          {p.status_pagamento}
        </Badge>
      ),
    },
    {
      key: "actions",
      header: "Acoes",
      width: "170px",
      align: "right",
      render: (p) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(p)}
            title="Editar pagamento"
          >
            Editar
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => handleDelete(p)}
            disabled={deletingId === p.pagamento_id}
            title="Excluir pagamento"
          >
            {deletingId === p.pagamento_id ? "Excluindo..." : "Excluir"}
          </Button>
        </div>
      ),
    },
  ];

  // FE-10: contador baseado na pagina exibida; oculto se nada foi carregado
  // (a primeira carga falhou), para nao afirmar "0 pagamentos".
  vlog(FILE, "PagamentosContent", "calculando faixa exibida (pagina=%d, limite=%d, total=%d)", exibida.pagina, exibida.limite, total);
  const { inicio: startItem, fim: endItem } = faixaExibida(
    exibida.pagina,
    exibida.limite,
    total
  );
  vlog(FILE, "PagamentosContent", "calculando se oculta o contador (qtd=%d)", pagamentos.length);
  const ocultarContador = erroCarga && pagamentos.length === 0;
  vlog(FILE, "PagamentosContent", "verificando se ha filtros aplicados");
  const hasFilters = Boolean(
    statusFilter || formaFilter || pedidoIdFilter || vencimentoDe || vencimentoAte
  );

  return (
    <>
      <div>
        <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-2xl font-bold text-slate-900">Pagamentos</h1>
            <p className="mt-1 text-sm text-slate-500">
              Consulte e gerencie os pagamentos dos pedidos.
            </p>
          </div>
          <Button onClick={openCreate}>+ Novo Pagamento</Button>
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
                <div className="sm:w-40">
                  <Input
                    label="Pedido (ID)"
                    placeholder="Ex: 123"
                    type="number"
                    min="1"
                    value={pedidoIdFilter}
                    onChange={(e) => setPedidoIdFilter(e.target.value)}
                  />
                </div>
                <div className="sm:w-52">
                  <Select
                    label="Status"
                    options={STATUS_FILTER_OPTIONS}
                    value={statusFilter}
                    onChange={(e) => setStatusFilter(e.target.value)}
                  />
                </div>
                <div className="sm:w-52">
                  <Select
                    label="Forma de pagamento"
                    options={FORMA_FILTER_OPTIONS}
                    value={formaFilter}
                    onChange={(e) => setFormaFilter(e.target.value)}
                  />
                </div>
                <div className="sm:w-40">
                  <Input
                    label="Vencimento de"
                    type="date"
                    value={vencimentoDe}
                    onChange={(e) => setVencimentoDe(e.target.value)}
                  />
                </div>
                <div className="sm:w-40">
                  <Input
                    label="Vencimento ate"
                    type="date"
                    value={vencimentoAte}
                    onChange={(e) => setVencimentoAte(e.target.value)}
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
                    ? "0 pagamentos"
                    : `${startItem}-${endItem} de ${total} ${
                        total === 1 ? "pagamento" : "pagamentos"
                      }`}
                </div>
              )}
            </div>
          </div>

          <div className="p-4">
            <Table
              columns={columns}
              data={pagamentos}
              keyExtractor={(p) => p.pagamento_id}
              loading={loading}
              erroCarga={erroCarga}
              sortKey={sortKey}
              sortDir={sortDir}
              onSort={(key) => handleSort(key as SortKey)}
              emptyMessage={
                hasFilters
                  ? "Nenhum pagamento encontrado para os filtros aplicados."
                  : "Nenhum pagamento cadastrado."
              }
            />
          </div>

          <Paginador pagina={exibida.pagina} paginas={pages} onIrPara={irParaPagina} />
        </Card>

        <div className="mt-4 text-xs text-slate-400">
          <strong>Nota:</strong> esta tela e de acesso comum (qualquer
          usuario autenticado). Ao editar, o ID do pagamento e o ID do
          pedido de origem nao podem ser alterados.
        </div>

        <PagamentoModal
          open={modalOpen}
          mode={modalMode}
          pagamento={editingPagamento}
          onClose={() => setModalOpen(false)}
          onSubmit={handleModalSubmit}
        />
      </div>
    </>
  );
}

export default function PagamentosPage() {
  return (
    <ProtectedRoute>
      <div className="min-h-screen bg-slate-50">
        <Navbar />
        <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <CarteiraGuard title="Pagamentos">
            <PagamentosContent />
          </CarteiraGuard>
        </main>
      </div>
    </ProtectedRoute>
  );
}
