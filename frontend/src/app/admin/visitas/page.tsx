"use client";

import { useEffect, useMemo, useState } from "react";
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
import { VisitaModal } from "@/components/admin/VisitaModal";
import {
  apiListVisitas,
  apiCreateVisita,
  apiUpdateVisita,
  apiDeleteVisita,
  apiListVendedores,
  apiListClientes,
} from "@/lib/api";
import { useSessionUser } from "@/lib/session";
import { Visita, VisitaInput, Vendedor, Cliente } from "@/lib/types";
import { formatarData } from "@/lib/formatarData";
import { vlog } from "@/lib/vlog";

const FILE = "admin/visitas/page.tsx";

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

function VisitasContent() {
  vlog(FILE, "VisitasContent", "inicializando estado da lista de visitas");
  const [visitas, setVisitas] = useState<Visita[]>([]);
  vlog(FILE, "VisitasContent", "inicializando estado de loading");
  const [loading, setLoading] = useState(true);
  vlog(FILE, "VisitasContent", "inicializando estado de erro");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "VisitasContent", "inicializando estado de erro de carga");
  const [erroCarga, setErroCarga] = useState(false);
  vlog(FILE, "VisitasContent", "obtendo controle de mensagem temporaria de sucesso");
  const {
    mensagem: success,
    mostrar: mostrarSucesso,
    limpar: limparSucesso,
  } = useMensagemTemporaria();

  // Listas auxiliares para exibir nome do cliente/vendedor nas linhas e
  // popular os filtros de coluna.
  vlog(FILE, "VisitasContent", "inicializando estado da lista de vendedores");
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  vlog(FILE, "VisitasContent", "inicializando estado da lista de clientes");
  const [clientes, setClientes] = useState<Cliente[]>([]);

  // Usuário logado: vendedores (role !== "admin") só enxergam a própria
  // carteira — o filtro de vendedor é travado com o id_vendedor do usuário
  // e o seletor "todos os vendedores" fica oculto. Admin mantém o filtro
  // livre (padrão anterior). Sem id_vendedor vinculado, o vendedor não tem
  // carteira: a lista permanece vazia (mesmo comportamento do backend).
  // Fonte da verdade: sessao em memoria validada por GET /api/auth/me
  // (revalidada ao montar e ao voltar o foco), nao o localStorage.
  vlog(FILE, "VisitasContent", "obtendo usuario da sessao em memoria");
  const currentUser = useSessionUser();
  vlog(FILE, "VisitasContent", "calculando se usuario e admin");
  const isAdmin = currentUser?.role === "admin";
  vlog(FILE, "VisitasContent", "calculando se usuario esta sem carteira");
  const semCarteira = !isAdmin && !currentUser?.id_vendedor;
  vlog(FILE, "VisitasContent", "definindo vendedor travado do usuario (admin=%s, semCarteira=%s)", isAdmin, semCarteira);
  const meuVendedorId =
    !isAdmin && currentUser?.id_vendedor ? String(currentUser.id_vendedor) : "";

  vlog(FILE, "VisitasContent", "inicializando estado da busca textual");
  const [search, setSearch] = useState("");
  // Filtro livre so para admin; usuario normal usa sempre meuVendedorId
  // (acompanha mudancas do vinculo sem novo login).
  vlog(FILE, "VisitasContent", "inicializando estado do filtro de vendedor");
  const [vendedorFilter, setVendedorFilter] = useState("");
  vlog(FILE, "VisitasContent", "definindo filtro de vendedor efetivo");
  const filtroVendedor = isAdmin ? vendedorFilter : meuVendedorId;
  vlog(FILE, "VisitasContent", "inicializando estado do filtro de cliente");
  const [clienteFilter, setClienteFilter] = useState("");
  vlog(FILE, "VisitasContent", "inicializando estado do filtro de resultado");
  const [resultadoFilter, setResultadoFilter] = useState("");
  vlog(FILE, "VisitasContent", "inicializando estado do filtro de data inicial");
  const [dataVisitaDe, setDataVisitaDe] = useState("");
  vlog(FILE, "VisitasContent", "inicializando estado do filtro de data final");
  const [dataVisitaAte, setDataVisitaAte] = useState("");

  vlog(FILE, "VisitasContent", "inicializando estado da coluna de ordenacao");
  const [sortKey, setSortKey] = useState<SortKey>("visita_id");
  vlog(FILE, "VisitasContent", "inicializando estado da direcao de ordenacao");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  vlog(FILE, "VisitasContent", "inicializando estado da pagina atual");
  const [page, setPage] = useState(1);
  vlog(FILE, "VisitasContent", "inicializando estado do limite por pagina");
  const [limit, setLimit] = useState(20);
  vlog(FILE, "VisitasContent", "inicializando estado do total de registros");
  const [total, setTotal] = useState(0);
  vlog(FILE, "VisitasContent", "inicializando estado do total de paginas");
  const [pages, setPages] = useState(0);
  // FE-10: pagina/limite exibidos (a pedida, ou a ultima carregada se a
  // ultima carga falhou).
  vlog(FILE, "VisitasContent", "obtendo pagina exibida (page=%d, limit=%d, erroCarga=%s)", page, limit, erroCarga);
  const { exibida, registrar: registrarCarregada } = usePaginaCarregada(
    page,
    limit,
    erroCarga
  );

  // Modal state
  vlog(FILE, "VisitasContent", "inicializando estado de abertura do modal");
  const [modalOpen, setModalOpen] = useState(false);
  vlog(FILE, "VisitasContent", "inicializando estado do modo do modal");
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  vlog(FILE, "VisitasContent", "inicializando estado da visita em edicao");
  const [editingVisita, setEditingVisita] = useState<Visita | null>(null);
  vlog(FILE, "VisitasContent", "inicializando estado do id em exclusao");
  const [deletingId, setDeletingId] = useState<number | null>(null);

  // Carrega vendedores/clientes uma vez, para os selects de filtro e para
  // exibir "ID - Nome" nas linhas da tabela (a API de visitas so retorna
  // os IDs).
  vlog(FILE, "VisitasContent", "registrando efeito de carga das listas auxiliares");
  useEffect(() => {
    vlog(FILE, "VisitasContent.useEffect", "chamando apiListVendedores");
    apiListVendedores()
      .then(setVendedores)
      .catch(() => setVendedores([]));

    vlog(FILE, "VisitasContent.useEffect", "definindo funcao loadAllClientes");
    const loadAllClientes = async () => {
      vlog(FILE, "VisitasContent.loadAllClientes", "definindo tamanho de pagina");
      const PAGE_SIZE = 100;
      vlog(FILE, "VisitasContent.loadAllClientes", "iniciando carga paginada de clientes");
      try {
        vlog(FILE, "VisitasContent.loadAllClientes", "chamando apiListClientes da pagina 1");
        const first = await apiListClientes(1, PAGE_SIZE, {});
        vlog(FILE, "VisitasContent.loadAllClientes", "acumulando clientes da pagina 1 (qtd=%d)", first.data.length);
        const all = [...first.data];
        vlog(FILE, "VisitasContent.loadAllClientes", "obtendo total de paginas (pages=%d)", first.pages);
        const totalPages = first.pages || 1;
        vlog(FILE, "VisitasContent.loadAllClientes", "buscando paginas restantes de clientes");
        for (let p = 2; p <= totalPages; p++) {
          const res = await apiListClientes(p, PAGE_SIZE, {});
          all.push(...res.data);
        }
        vlog(FILE, "VisitasContent.loadAllClientes", "aplicando lista completa de clientes (qtd=%d)", all.length);
        setClientes(all);
      } catch {
        vlog(FILE, "VisitasContent.loadAllClientes", "falha na carga; zerando lista de clientes");
        setClientes([]);
      }
    };
    vlog(FILE, "VisitasContent.useEffect", "disparando loadAllClientes");
    loadAllClientes();
  }, []);

  vlog(FILE, "VisitasContent", "memorizando mapa de nomes de vendedores");
  const vendedorNome = useMemo(() => {
    vlog(FILE, "VisitasContent.vendedorNome", "criando mapa id->nome");
    const map = new Map<number, string>();
    vlog(FILE, "VisitasContent.vendedorNome", "preenchendo mapa (qtd=%d)", vendedores.length);
    vendedores.forEach((v) => map.set(v.id, v.nome));
    return map;
  }, [vendedores]);

  vlog(FILE, "VisitasContent", "memorizando mapa de nomes de clientes");
  const clienteNome = useMemo(() => {
    vlog(FILE, "VisitasContent.clienteNome", "criando mapa id->razao social");
    const map = new Map<number, string>();
    vlog(FILE, "VisitasContent.clienteNome", "preenchendo mapa (qtd=%d)", clientes.length);
    clientes.forEach((c) => map.set(c.cliente_id_origem, c.razao_social));
    return map;
  }, [clientes]);

  vlog(FILE, "VisitasContent", "memorizando opcoes do filtro de vendedor");
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

  vlog(FILE, "VisitasContent", "memorizando opcoes do filtro de cliente");
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

  // FE-04: so a busca mais recente aplica o resultado (respostas obsoletas
  // sao descartadas), inclusive entre o efeito e as recargas imperativas.
  vlog(FILE, "VisitasContent", "obtendo executor de busca com descarte de respostas obsoletas");
  const executarBusca = useUltimaResposta();
  // FE-04: linha excluida nao reaparece por uma resposta que saiu antes do
  // DELETE e chegou depois dele.
  vlog(FILE, "VisitasContent", "obtendo controle de ids excluidos");
  const excluidos = useExcluidos((x: Visita) => x.visita_id);

  // Busca separada em requisicao pura + aplicacao do resultado no callback
  // assincrono (.then): o efeito nunca chama setState de forma sincrona.
  // Vendedor sem carteira vinculada (id_vendedor null): não há dados a
  // buscar, resolve uma lista vazia sem chamar a API.
  vlog(FILE, "VisitasContent", "definindo funcao buscarVisitas");
  const buscarVisitas = (): Promise<{
    data: Visita[];
    total: number;
    pages: number;
  }> => {
    vlog(FILE, "VisitasContent.buscarVisitas", "verificando se usuario esta sem carteira (semCarteira=%s)", semCarteira);
    if (semCarteira) return Promise.resolve({ data: [], total: 0, pages: 0 });
    vlog(FILE, "VisitasContent.buscarVisitas", "chamando apiListVisitas (page=%d, limit=%d, sortKey=%s, sortDir=%s)", page, limit, sortKey, sortDir);
    return apiListVisitas(
      page,
      limit,
      {
        vendedor_id: filtroVendedor ? Number(filtroVendedor) : undefined,
        cliente_id: clienteFilter ? Number(clienteFilter) : undefined,
        resultado: resultadoFilter || undefined,
        data_visita_de: dataVisitaDe || undefined,
        data_visita_ate: dataVisitaAte || undefined,
        q: search.trim() || undefined,
      },
      ORDER_BY_MAP[sortKey],
      sortDir
    );
  };

  vlog(FILE, "VisitasContent", "definindo funcao aplicarVisitas");
  const aplicarVisitas = (res: {
    data: Visita[];
    total: number;
    pages: number;
  }) => {
    vlog(FILE, "VisitasContent.aplicarVisitas", "aplicando lista de visitas (qtd=%d)", res.data.length);
    setVisitas(res.data);
    vlog(FILE, "VisitasContent.aplicarVisitas", "atualizando total (total=%d)", res.total);
    setTotal(res.total);
    vlog(FILE, "VisitasContent.aplicarVisitas", "atualizando total de paginas (pages=%d)", res.pages);
    setPages(res.pages);
    vlog(FILE, "VisitasContent.aplicarVisitas", "registrando pagina carregada (page=%d, limit=%d)", page, limit);
    registrarCarregada(page, limit);
    vlog(FILE, "VisitasContent.aplicarVisitas", "limpando erro de carga");
    setErroCarga(false);
    vlog(FILE, "VisitasContent.aplicarVisitas", "desativando loading");
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  vlog(FILE, "VisitasContent", "definindo funcao aplicarErroVisitas");
  const aplicarErroVisitas = (err: unknown): string => {
    vlog(FILE, "VisitasContent.aplicarErroVisitas", "extraindo mensagem do erro de carga");
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar visitas. O endpoint /api/visitas pode nao existir no backend.";
    vlog(FILE, "VisitasContent.aplicarErroVisitas", "exibindo mensagem de erro");
    setError(message);
    vlog(FILE, "VisitasContent.aplicarErroVisitas", "marcando erro de carga");
    setErroCarga(true);
    vlog(FILE, "VisitasContent.aplicarErroVisitas", "desativando loading");
    setLoading(false);
    return message;
  };

  // Recarga imperativa (handlers e timers). FE-10: devolve a mensagem de erro se a
  // recarga falhar (null se deu certo ou foi superada por outra busca).
  vlog(FILE, "VisitasContent", "definindo funcao loadVisitas");
  const loadVisitas = async (): Promise<string | null> => {
    vlog(FILE, "VisitasContent.loadVisitas", "ativando loading");
    setLoading(true);
    vlog(FILE, "VisitasContent.loadVisitas", "limpando erro");
    setError(null);
    vlog(FILE, "VisitasContent.loadVisitas", "inicializando variavel de falha");
    let falha: string | null = null;
    vlog(FILE, "VisitasContent.loadVisitas", "executando busca de visitas sem excluidos");
    await executarBusca(buscarVisitas().then(excluidos.filtrar), aplicarVisitas, (err) => {
      vlog(FILE, "VisitasContent.loadVisitas.func", "registrando falha da recarga");
      falha = aplicarErroVisitas(err);
    });
    return falha;
  };

  // FE-10: navegacao a partir da pagina exibida. Se o destino ja e a pagina
  // pedida (a troca anterior falhou), repete a busca em vez de nao fazer nada.
  vlog(FILE, "VisitasContent", "definindo funcao irParaPagina");
  const irParaPagina = (n: number) => {
    vlog(FILE, "VisitasContent.irParaPagina", "verificando se destino e a pagina atual (destino=%d, atual=%d)", n, page);
    if (n === page) loadVisitas();
    else setPage(n);
  };

  // Paginacao/ordenacao mudou: liga o loading durante o render (padrao
  // "ajustar estado quando a entrada muda") e o efeito so faz a busca.
  vlog(FILE, "VisitasContent", "montando chave de paginacao/ordenacao");
  const chaveLista = `${page}|${limit}|${sortKey}|${sortDir}`;
  vlog(FILE, "VisitasContent", "inicializando estado da chave anterior");
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  vlog(FILE, "VisitasContent", "verificando se a chave de paginacao/ordenacao mudou");
  if (chaveAnterior !== chaveLista) {
    vlog(FILE, "VisitasContent", "atualizando chave anterior");
    setChaveAnterior(chaveLista);
    vlog(FILE, "VisitasContent", "ativando loading");
    setLoading(true);
    vlog(FILE, "VisitasContent", "limpando erro");
    setError(null);
  }

  vlog(FILE, "VisitasContent", "registrando efeito de busca por paginacao/ordenacao");
  useEffect(() => {
    vlog(FILE, "VisitasContent.useEffect", "executando busca de visitas sem excluidos");
    executarBusca(buscarVisitas().then(excluidos.filtrar), aplicarVisitas, aplicarErroVisitas);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual e reset para pagina 1 quando filtros mudam.
  // Selects (vendedor/cliente/resultado) tambem passam por este efeito, mas
  // como o debounce e curto (350ms) o efeito pratico e quase imediato.
  // FE-04: o debounce nao dispara na montagem, so quando a chave dos filtros
  // muda (evita a busca dupla e a tabela voltando para a pagina 1).
  vlog(FILE, "VisitasContent", "montando chave dos filtros");
  const chaveFiltros = JSON.stringify([
    search,
    vendedorFilter,
    meuVendedorId,
    clienteFilter,
    resultadoFilter,
    dataVisitaDe,
    dataVisitaAte,
  ]);
  vlog(FILE, "VisitasContent", "registrando debounce dos filtros");
  useDebounceFiltros(chaveFiltros, () => {
    vlog(FILE, "VisitasContent.useDebounceFiltros", "filtros mudaram; verificando se esta na pagina 1 (page=%d)", page);
    if (page !== 1) {
      vlog(FILE, "VisitasContent.useDebounceFiltros", "voltando para a pagina 1");
      setPage(1);
    } else {
      vlog(FILE, "VisitasContent.useDebounceFiltros", "recarregando visitas");
      loadVisitas();
    }
  });

  vlog(FILE, "VisitasContent", "definindo handler handleSort");
  const handleSort = (key: SortKey) => {
    vlog(FILE, "VisitasContent.handleSort", "verificando se coluna ja e a ordenada (key=%s)", key);
    if (sortKey === key) {
      vlog(FILE, "VisitasContent.handleSort", "invertendo direcao da ordenacao");
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      vlog(FILE, "VisitasContent.handleSort", "definindo nova coluna de ordenacao");
      setSortKey(key);
      vlog(FILE, "VisitasContent.handleSort", "definindo direcao asc");
      setSortDir("asc");
    }
  };

  vlog(FILE, "VisitasContent", "definindo handler openCreate");
  const openCreate = () => {
    vlog(FILE, "VisitasContent.openCreate", "definindo modo create");
    setModalMode("create");
    vlog(FILE, "VisitasContent.openCreate", "limpando visita em edicao");
    setEditingVisita(null);
    vlog(FILE, "VisitasContent.openCreate", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "VisitasContent", "definindo handler openEdit");
  const openEdit = (visita: Visita) => {
    vlog(FILE, "VisitasContent.openEdit", "definindo modo edit (visita_id=%d)", visita.visita_id);
    setModalMode("edit");
    vlog(FILE, "VisitasContent.openEdit", "definindo visita em edicao");
    setEditingVisita(visita);
    vlog(FILE, "VisitasContent.openEdit", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "VisitasContent", "definindo handler handleModalSubmit");
  const handleModalSubmit = async (data: VisitaInput) => {
    vlog(FILE, "VisitasContent.handleModalSubmit", "limpando erro");
    setError(null);
    vlog(FILE, "VisitasContent.handleModalSubmit", "verificando modo do modal (modo=%s)", modalMode);
    if (modalMode === "create") {
      vlog(FILE, "VisitasContent.handleModalSubmit", "chamando apiCreateVisita");
      const created = await apiCreateVisita(data);
      vlog(FILE, "VisitasContent.handleModalSubmit", "montando mensagem de sucesso (visita_id=%d)", created.visita_id);
      const msg = `Visita #${created.visita_id} criada com sucesso.`;
      vlog(FILE, "VisitasContent.handleModalSubmit", "recarregando visitas apos criacao");
      const erroRecarga = await loadVisitas();
      vlog(FILE, "VisitasContent.handleModalSubmit", "verificando falha na recarga (falhou=%s)", erroRecarga !== null);
      if (erroRecarga) setError(mensagemRecargaFalhou(msg, erroRecarga));
      else mostrarSucesso(msg);
    } else if (editingVisita) {
      vlog(FILE, "VisitasContent.handleModalSubmit", "chamando apiUpdateVisita (visita_id=%d)", editingVisita.visita_id);
      const updated = await apiUpdateVisita(editingVisita.visita_id, data);
      vlog(FILE, "VisitasContent.handleModalSubmit", "substituindo visita atualizada na lista");
      setVisitas((prev) =>
        prev.map((v) => (v.visita_id === updated.visita_id ? updated : v))
      );
      vlog(FILE, "VisitasContent.handleModalSubmit", "exibindo mensagem de sucesso");
      mostrarSucesso(`Visita #${updated.visita_id} atualizada com sucesso.`);
    }
    vlog(FILE, "VisitasContent.handleModalSubmit", "fechando modal");
    setModalOpen(false);
  };

  vlog(FILE, "VisitasContent", "definindo handler handleDelete");
  const handleDelete = async (visita: Visita) => {
    vlog(FILE, "VisitasContent.handleDelete", "pedindo confirmacao de exclusao (visita_id=%d)", visita.visita_id);
    const confirmed = window.confirm(
      `Tem certeza que deseja excluir a visita #${visita.visita_id}?`
    );
    vlog(FILE, "VisitasContent.handleDelete", "verificando confirmacao (ok=%s)", confirmed);
    if (!confirmed) return;

    vlog(FILE, "VisitasContent.handleDelete", "limpando erro");
    setError(null);
    vlog(FILE, "VisitasContent.handleDelete", "marcando id em exclusao");
    setDeletingId(visita.visita_id);
    vlog(FILE, "VisitasContent.handleDelete", "iniciando chamada de exclusao");
    try {
      vlog(FILE, "VisitasContent.handleDelete", "chamando apiDeleteVisita");
      await apiDeleteVisita(visita.visita_id);
      vlog(FILE, "VisitasContent.handleDelete", "marcando id como excluido");
      excluidos.marcar(visita.visita_id);
      vlog(FILE, "VisitasContent.handleDelete", "removendo visita da lista");
      setVisitas((prev) => prev.filter((v) => v.visita_id !== visita.visita_id));
      vlog(FILE, "VisitasContent.handleDelete", "decrementando total");
      setTotal((t) => Math.max(0, t - 1));
      vlog(FILE, "VisitasContent.handleDelete", "exibindo mensagem de sucesso");
      mostrarSucesso(`Visita #${visita.visita_id} excluida com sucesso.`);
    } catch (err) {
      vlog(FILE, "VisitasContent.handleDelete", "falha na exclusao; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao excluir visita.";
      vlog(FILE, "VisitasContent.handleDelete", "exibindo mensagem de erro");
      setError(message);
    } finally {
      vlog(FILE, "VisitasContent.handleDelete", "limpando id em exclusao");
      setDeletingId(null);
    }
  };

  vlog(FILE, "VisitasContent", "montando definicao das colunas da tabela");
  const columns: Column<Visita>[] = [
    {
      key: "visita_id",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      render: (v) => <span className="font-mono text-xs">#{v.visita_id}</span>,
    },
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
      render: (v) => <span className="text-slate-600">{formatarData(v.data_visita)}</span>,
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

  // FE-10: contador baseado na pagina exibida; oculto se nada foi carregado
  // (a primeira carga falhou), para nao afirmar "0 visitas".
  vlog(FILE, "VisitasContent", "calculando faixa exibida (pagina=%d, limite=%d, total=%d)", exibida.pagina, exibida.limite, total);
  const { inicio: startItem, fim: endItem } = faixaExibida(
    exibida.pagina,
    exibida.limite,
    total
  );
  vlog(FILE, "VisitasContent", "calculando se oculta o contador (qtd=%d)", visitas.length);
  const ocultarContador = erroCarga && visitas.length === 0;

  vlog(FILE, "VisitasContent", "verificando se ha filtros aplicados");
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
            {!ocultarContador && (
              <div className="text-sm text-slate-500">
                {total === 0
                  ? "0 visitas"
                  : `${startItem}-${endItem} de ${total} ${
                      total === 1 ? "visita" : "visitas"
                    }`}
              </div>
            )}
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={visitas}
            keyExtractor={(v) => v.visita_id}
            loading={loading}
            erroCarga={erroCarga}
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

        <Paginador pagina={exibida.pagina} paginas={pages} onIrPara={irParaPagina} />
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

export default function VisitasPage() {
  return (
    <CarteiraGuard title="Visitas">
      <VisitasContent />
    </CarteiraGuard>
  );
}
