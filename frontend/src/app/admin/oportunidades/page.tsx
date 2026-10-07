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
import { formatarData } from "@/lib/formatarData";
import { vlog } from "@/lib/vlog";

const FILE = "admin/oportunidades/page.tsx";

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

function OportunidadesContent() {
  vlog(FILE, "OportunidadesContent", "inicializando estado da lista de oportunidades");
  const [oportunidades, setOportunidades] = useState<Oportunidade[]>([]);
  vlog(FILE, "OportunidadesContent", "inicializando estado de loading");
  const [loading, setLoading] = useState(true);
  vlog(FILE, "OportunidadesContent", "inicializando estado de erro");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "OportunidadesContent", "inicializando estado de erro de carga");
  const [erroCarga, setErroCarga] = useState(false);
  vlog(FILE, "OportunidadesContent", "obtendo controle de mensagem temporaria de sucesso");
  const {
    mensagem: success,
    mostrar: mostrarSucesso,
    limpar: limparSucesso,
  } = useMensagemTemporaria();

  // Listas auxiliares para exibir nome do cliente/vendedor nas linhas e
  // popular os filtros de coluna.
  vlog(FILE, "OportunidadesContent", "inicializando estado da lista de vendedores");
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  vlog(FILE, "OportunidadesContent", "inicializando estado da lista de clientes");
  const [clientes, setClientes] = useState<Cliente[]>([]);

  // Usuário logado: vendedores (role !== "admin") só enxergam a própria
  // carteira — o filtro de vendedor é travado com o id_vendedor do usuário
  // e o seletor "todos os vendedores" fica oculto. Admin mantém o filtro
  // livre (padrão anterior). Sem id_vendedor vinculado, o vendedor não tem
  // carteira: a lista permanece vazia (mesmo comportamento do backend).
  // Fonte da verdade: sessao em memoria validada por GET /api/auth/me
  // (revalidada ao montar e ao voltar o foco), nao o localStorage.
  vlog(FILE, "OportunidadesContent", "obtendo usuario da sessao em memoria");
  const currentUser = useSessionUser();
  vlog(FILE, "OportunidadesContent", "calculando se usuario e admin");
  const isAdmin = currentUser?.role === "admin";
  vlog(FILE, "OportunidadesContent", "calculando se usuario esta sem carteira");
  const semCarteira = !isAdmin && !currentUser?.id_vendedor;
  vlog(FILE, "OportunidadesContent", "definindo vendedor travado do usuario (admin=%s, semCarteira=%s)", isAdmin, semCarteira);
  const meuVendedorId =
    !isAdmin && currentUser?.id_vendedor ? String(currentUser.id_vendedor) : "";

  vlog(FILE, "OportunidadesContent", "inicializando estado da busca textual");
  const [search, setSearch] = useState("");
  // Filtro livre so para admin; usuario normal usa sempre meuVendedorId
  // (acompanha mudancas do vinculo sem novo login).
  vlog(FILE, "OportunidadesContent", "inicializando estado do filtro de vendedor");
  const [vendedorFilter, setVendedorFilter] = useState("");
  vlog(FILE, "OportunidadesContent", "definindo filtro de vendedor efetivo");
  const filtroVendedor = isAdmin ? vendedorFilter : meuVendedorId;
  vlog(FILE, "OportunidadesContent", "inicializando estado do filtro de cliente");
  const [clienteFilter, setClienteFilter] = useState("");
  vlog(FILE, "OportunidadesContent", "inicializando estado do filtro de etapa");
  const [etapaFilter, setEtapaFilter] = useState("");
  vlog(FILE, "OportunidadesContent", "inicializando estado do filtro de origem");
  const [origemFilter, setOrigemFilter] = useState("");
  vlog(FILE, "OportunidadesContent", "inicializando estado do filtro de abertura inicial");
  const [dataAberturaDe, setDataAberturaDe] = useState("");
  vlog(FILE, "OportunidadesContent", "inicializando estado do filtro de abertura final");
  const [dataAberturaAte, setDataAberturaAte] = useState("");

  vlog(FILE, "OportunidadesContent", "inicializando estado da coluna de ordenacao");
  const [sortKey, setSortKey] = useState<SortKey>("oportunidade_id");
  vlog(FILE, "OportunidadesContent", "inicializando estado da direcao de ordenacao");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  vlog(FILE, "OportunidadesContent", "inicializando estado da pagina atual");
  const [page, setPage] = useState(1);
  vlog(FILE, "OportunidadesContent", "inicializando estado do limite por pagina");
  const [limit, setLimit] = useState(20);
  vlog(FILE, "OportunidadesContent", "inicializando estado do total de registros");
  const [total, setTotal] = useState(0);
  vlog(FILE, "OportunidadesContent", "inicializando estado do total de paginas");
  const [pages, setPages] = useState(0);
  // FE-10: pagina/limite exibidos (a pedida, ou a ultima carregada se a
  // ultima carga falhou).
  vlog(FILE, "OportunidadesContent", "obtendo pagina exibida (page=%d, limit=%d, erroCarga=%s)", page, limit, erroCarga);
  const { exibida, registrar: registrarCarregada } = usePaginaCarregada(
    page,
    limit,
    erroCarga
  );

  // Modal state
  vlog(FILE, "OportunidadesContent", "inicializando estado de abertura do modal");
  const [modalOpen, setModalOpen] = useState(false);
  vlog(FILE, "OportunidadesContent", "inicializando estado do modo do modal");
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  vlog(FILE, "OportunidadesContent", "inicializando estado da oportunidade em edicao");
  const [editingOportunidade, setEditingOportunidade] = useState<Oportunidade | null>(
    null
  );
  vlog(FILE, "OportunidadesContent", "inicializando estado do id em exclusao");
  const [deletingId, setDeletingId] = useState<number | null>(null);

  // Carrega vendedores/clientes uma vez, para os selects de filtro e para
  // exibir "ID - Nome" nas linhas da tabela (a API de oportunidades so
  // retorna os IDs).
  vlog(FILE, "OportunidadesContent", "registrando efeito de carga das listas auxiliares");
  useEffect(() => {
    vlog(FILE, "OportunidadesContent.useEffect", "chamando apiListVendedores");
    apiListVendedores()
      .then(setVendedores)
      .catch(() => setVendedores([]));

    vlog(FILE, "OportunidadesContent.useEffect", "definindo funcao loadAllClientes");
    const loadAllClientes = async () => {
      vlog(FILE, "OportunidadesContent.loadAllClientes", "definindo tamanho de pagina");
      const PAGE_SIZE = 100;
      vlog(FILE, "OportunidadesContent.loadAllClientes", "iniciando carga paginada de clientes");
      try {
        vlog(FILE, "OportunidadesContent.loadAllClientes", "chamando apiListClientes da pagina 1");
        const first = await apiListClientes(1, PAGE_SIZE, {});
        vlog(FILE, "OportunidadesContent.loadAllClientes", "acumulando clientes da pagina 1 (qtd=%d)", first.data.length);
        const all = [...first.data];
        vlog(FILE, "OportunidadesContent.loadAllClientes", "obtendo total de paginas (pages=%d)", first.pages);
        const totalPages = first.pages || 1;
        vlog(FILE, "OportunidadesContent.loadAllClientes", "buscando paginas restantes de clientes");
        for (let p = 2; p <= totalPages; p++) {
          const res = await apiListClientes(p, PAGE_SIZE, {});
          all.push(...res.data);
        }
        vlog(FILE, "OportunidadesContent.loadAllClientes", "aplicando lista completa de clientes (qtd=%d)", all.length);
        setClientes(all);
      } catch {
        vlog(FILE, "OportunidadesContent.loadAllClientes", "falha na carga; zerando lista de clientes");
        setClientes([]);
      }
    };
    vlog(FILE, "OportunidadesContent.useEffect", "disparando loadAllClientes");
    loadAllClientes();
  }, []);

  vlog(FILE, "OportunidadesContent", "memorizando mapa de nomes de vendedores");
  const vendedorNome = useMemo(() => {
    vlog(FILE, "OportunidadesContent.vendedorNome", "criando mapa id->nome");
    const map = new Map<number, string>();
    vlog(FILE, "OportunidadesContent.vendedorNome", "preenchendo mapa (qtd=%d)", vendedores.length);
    vendedores.forEach((v) => map.set(v.id, v.nome));
    return map;
  }, [vendedores]);

  vlog(FILE, "OportunidadesContent", "memorizando mapa de nomes de clientes");
  const clienteNome = useMemo(() => {
    vlog(FILE, "OportunidadesContent.clienteNome", "criando mapa id->razao social");
    const map = new Map<number, string>();
    vlog(FILE, "OportunidadesContent.clienteNome", "preenchendo mapa (qtd=%d)", clientes.length);
    clientes.forEach((c) => map.set(c.cliente_id_origem, c.razao_social));
    return map;
  }, [clientes]);

  vlog(FILE, "OportunidadesContent", "memorizando opcoes do filtro de vendedor");
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

  vlog(FILE, "OportunidadesContent", "memorizando opcoes do filtro de cliente");
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
  vlog(FILE, "OportunidadesContent", "obtendo executor de busca com descarte de respostas obsoletas");
  const executarBusca = useUltimaResposta();
  // FE-04: linha excluida nao reaparece por uma resposta que saiu antes do
  // DELETE e chegou depois dele.
  vlog(FILE, "OportunidadesContent", "obtendo controle de ids excluidos");
  const excluidos = useExcluidos((x: Oportunidade) => x.oportunidade_id);

  // Busca separada em requisicao pura + aplicacao do resultado no callback
  // assincrono (.then): o efeito nunca chama setState de forma sincrona.
  // Vendedor sem carteira vinculada (id_vendedor null): não há dados a
  // buscar, resolve uma lista vazia sem chamar a API.
  vlog(FILE, "OportunidadesContent", "definindo funcao buscarOportunidades");
  const buscarOportunidades = (): Promise<{
    data: Oportunidade[];
    total: number;
    pages: number;
  }> => {
    vlog(FILE, "OportunidadesContent.buscarOportunidades", "verificando se usuario esta sem carteira (semCarteira=%s)", semCarteira);
    if (semCarteira) return Promise.resolve({ data: [], total: 0, pages: 0 });
    vlog(FILE, "OportunidadesContent.buscarOportunidades", "chamando apiListOportunidades (page=%d, limit=%d, sortKey=%s, sortDir=%s)", page, limit, sortKey, sortDir);
    return apiListOportunidades(
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
  };

  vlog(FILE, "OportunidadesContent", "definindo funcao aplicarOportunidades");
  const aplicarOportunidades = (res: {
    data: Oportunidade[];
    total: number;
    pages: number;
  }) => {
    vlog(FILE, "OportunidadesContent.aplicarOportunidades", "aplicando lista de oportunidades (qtd=%d)", res.data.length);
    setOportunidades(res.data);
    vlog(FILE, "OportunidadesContent.aplicarOportunidades", "atualizando total (total=%d)", res.total);
    setTotal(res.total);
    vlog(FILE, "OportunidadesContent.aplicarOportunidades", "atualizando total de paginas (pages=%d)", res.pages);
    setPages(res.pages);
    vlog(FILE, "OportunidadesContent.aplicarOportunidades", "registrando pagina carregada (page=%d, limit=%d)", page, limit);
    registrarCarregada(page, limit);
    vlog(FILE, "OportunidadesContent.aplicarOportunidades", "limpando erro de carga");
    setErroCarga(false);
    vlog(FILE, "OportunidadesContent.aplicarOportunidades", "desativando loading");
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  vlog(FILE, "OportunidadesContent", "definindo funcao aplicarErroOportunidades");
  const aplicarErroOportunidades = (err: unknown): string => {
    vlog(FILE, "OportunidadesContent.aplicarErroOportunidades", "extraindo mensagem do erro de carga");
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar oportunidades. O endpoint /api/oportunidades pode nao existir no backend.";
    vlog(FILE, "OportunidadesContent.aplicarErroOportunidades", "exibindo mensagem de erro");
    setError(message);
    vlog(FILE, "OportunidadesContent.aplicarErroOportunidades", "marcando erro de carga");
    setErroCarga(true);
    vlog(FILE, "OportunidadesContent.aplicarErroOportunidades", "desativando loading");
    setLoading(false);
    return message;
  };

  // Recarga imperativa (handlers e timers). FE-10: devolve a mensagem de erro se a
  // recarga falhar (null se deu certo ou foi superada por outra busca).
  vlog(FILE, "OportunidadesContent", "definindo funcao loadOportunidades");
  const loadOportunidades = async (): Promise<string | null> => {
    vlog(FILE, "OportunidadesContent.loadOportunidades", "ativando loading");
    setLoading(true);
    vlog(FILE, "OportunidadesContent.loadOportunidades", "limpando erro");
    setError(null);
    vlog(FILE, "OportunidadesContent.loadOportunidades", "inicializando variavel de falha");
    let falha: string | null = null;
    vlog(FILE, "OportunidadesContent.loadOportunidades", "executando busca de oportunidades sem excluidos");
    await executarBusca(buscarOportunidades().then(excluidos.filtrar), aplicarOportunidades, (err) => {
      vlog(FILE, "OportunidadesContent.loadOportunidades.func", "registrando falha da recarga");
      falha = aplicarErroOportunidades(err);
    });
    return falha;
  };

  // FE-10: navegacao a partir da pagina exibida. Se o destino ja e a pagina
  // pedida (a troca anterior falhou), repete a busca em vez de nao fazer nada.
  vlog(FILE, "OportunidadesContent", "definindo funcao irParaPagina");
  const irParaPagina = (n: number) => {
    vlog(FILE, "OportunidadesContent.irParaPagina", "verificando se destino e a pagina atual (destino=%d, atual=%d)", n, page);
    if (n === page) loadOportunidades();
    else setPage(n);
  };

  // Paginacao/ordenacao mudou: liga o loading durante o render (padrao
  // "ajustar estado quando a entrada muda") e o efeito so faz a busca.
  vlog(FILE, "OportunidadesContent", "montando chave de paginacao/ordenacao");
  const chaveLista = `${page}|${limit}|${sortKey}|${sortDir}`;
  vlog(FILE, "OportunidadesContent", "inicializando estado da chave anterior");
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  vlog(FILE, "OportunidadesContent", "verificando se a chave de paginacao/ordenacao mudou");
  if (chaveAnterior !== chaveLista) {
    vlog(FILE, "OportunidadesContent", "atualizando chave anterior");
    setChaveAnterior(chaveLista);
    vlog(FILE, "OportunidadesContent", "ativando loading");
    setLoading(true);
    vlog(FILE, "OportunidadesContent", "limpando erro");
    setError(null);
  }

  vlog(FILE, "OportunidadesContent", "registrando efeito de busca por paginacao/ordenacao");
  useEffect(() => {
    vlog(FILE, "OportunidadesContent.useEffect", "executando busca de oportunidades sem excluidos");
    executarBusca(buscarOportunidades().then(excluidos.filtrar), aplicarOportunidades, aplicarErroOportunidades);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual e reset para pagina 1 quando filtros mudam.
  // Selects (vendedor/cliente/etapa/origem) tambem passam por este efeito,
  // mas como o debounce e curto (350ms) o efeito pratico e quase imediato.
  // FE-04: o debounce nao dispara na montagem, so quando a chave dos filtros
  // muda (evita a busca dupla e a tabela voltando para a pagina 1).
  vlog(FILE, "OportunidadesContent", "montando chave dos filtros");
  const chaveFiltros = JSON.stringify([
    search,
    vendedorFilter,
    meuVendedorId,
    clienteFilter,
    etapaFilter,
    origemFilter,
    dataAberturaDe,
    dataAberturaAte,
  ]);
  vlog(FILE, "OportunidadesContent", "registrando debounce dos filtros");
  useDebounceFiltros(chaveFiltros, () => {
    vlog(FILE, "OportunidadesContent.useDebounceFiltros", "filtros mudaram; verificando se esta na pagina 1 (page=%d)", page);
    if (page !== 1) {
      vlog(FILE, "OportunidadesContent.useDebounceFiltros", "voltando para a pagina 1");
      setPage(1);
    } else {
      vlog(FILE, "OportunidadesContent.useDebounceFiltros", "recarregando oportunidades");
      loadOportunidades();
    }
  });

  vlog(FILE, "OportunidadesContent", "definindo handler handleSort");
  const handleSort = (key: SortKey) => {
    vlog(FILE, "OportunidadesContent.handleSort", "verificando se coluna ja e a ordenada (key=%s)", key);
    if (sortKey === key) {
      vlog(FILE, "OportunidadesContent.handleSort", "invertendo direcao da ordenacao");
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      vlog(FILE, "OportunidadesContent.handleSort", "definindo nova coluna de ordenacao");
      setSortKey(key);
      vlog(FILE, "OportunidadesContent.handleSort", "definindo direcao asc");
      setSortDir("asc");
    }
  };

  vlog(FILE, "OportunidadesContent", "definindo handler openCreate");
  const openCreate = () => {
    vlog(FILE, "OportunidadesContent.openCreate", "definindo modo create");
    setModalMode("create");
    vlog(FILE, "OportunidadesContent.openCreate", "limpando oportunidade em edicao");
    setEditingOportunidade(null);
    vlog(FILE, "OportunidadesContent.openCreate", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "OportunidadesContent", "definindo handler openEdit");
  const openEdit = (oportunidade: Oportunidade) => {
    vlog(FILE, "OportunidadesContent.openEdit", "definindo modo edit (oportunidade_id=%d)", oportunidade.oportunidade_id);
    setModalMode("edit");
    vlog(FILE, "OportunidadesContent.openEdit", "definindo oportunidade em edicao");
    setEditingOportunidade(oportunidade);
    vlog(FILE, "OportunidadesContent.openEdit", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "OportunidadesContent", "definindo handler handleModalSubmit");
  const handleModalSubmit = async (data: OportunidadeInput) => {
    vlog(FILE, "OportunidadesContent.handleModalSubmit", "limpando erro");
    setError(null);
    vlog(FILE, "OportunidadesContent.handleModalSubmit", "verificando modo do modal (modo=%s)", modalMode);
    if (modalMode === "create") {
      vlog(FILE, "OportunidadesContent.handleModalSubmit", "chamando apiCreateOportunidade");
      const created = await apiCreateOportunidade(data);
      vlog(FILE, "OportunidadesContent.handleModalSubmit", "montando mensagem de sucesso (oportunidade_id=%d)", created.oportunidade_id);
      const msg = `Oportunidade #${created.oportunidade_id} criada com sucesso.`;
      vlog(FILE, "OportunidadesContent.handleModalSubmit", "recarregando oportunidades apos criacao");
      const erroRecarga = await loadOportunidades();
      vlog(FILE, "OportunidadesContent.handleModalSubmit", "verificando falha na recarga (falhou=%s)", erroRecarga !== null);
      if (erroRecarga) setError(mensagemRecargaFalhou(msg, erroRecarga));
      else mostrarSucesso(msg);
    } else if (editingOportunidade) {
      vlog(FILE, "OportunidadesContent.handleModalSubmit", "chamando apiUpdateOportunidade (oportunidade_id=%d)", editingOportunidade.oportunidade_id);
      const updated = await apiUpdateOportunidade(
        editingOportunidade.oportunidade_id,
        data
      );
      vlog(FILE, "OportunidadesContent.handleModalSubmit", "substituindo oportunidade atualizada na lista");
      setOportunidades((prev) =>
        prev.map((o) =>
          o.oportunidade_id === updated.oportunidade_id ? updated : o
        )
      );
      vlog(FILE, "OportunidadesContent.handleModalSubmit", "exibindo mensagem de sucesso");
      mostrarSucesso(`Oportunidade #${updated.oportunidade_id} atualizada com sucesso.`);
    }
    vlog(FILE, "OportunidadesContent.handleModalSubmit", "fechando modal");
    setModalOpen(false);
  };

  vlog(FILE, "OportunidadesContent", "definindo handler handleDelete");
  const handleDelete = async (oportunidade: Oportunidade) => {
    vlog(FILE, "OportunidadesContent.handleDelete", "pedindo confirmacao de exclusao (oportunidade_id=%d)", oportunidade.oportunidade_id);
    const confirmed = window.confirm(
      `Tem certeza que deseja excluir a oportunidade #${oportunidade.oportunidade_id}?`
    );
    vlog(FILE, "OportunidadesContent.handleDelete", "verificando confirmacao (ok=%s)", confirmed);
    if (!confirmed) return;

    vlog(FILE, "OportunidadesContent.handleDelete", "limpando erro");
    setError(null);
    vlog(FILE, "OportunidadesContent.handleDelete", "marcando id em exclusao");
    setDeletingId(oportunidade.oportunidade_id);
    vlog(FILE, "OportunidadesContent.handleDelete", "iniciando chamada de exclusao");
    try {
      vlog(FILE, "OportunidadesContent.handleDelete", "chamando apiDeleteOportunidade");
      await apiDeleteOportunidade(oportunidade.oportunidade_id);
      vlog(FILE, "OportunidadesContent.handleDelete", "marcando id como excluido");
      excluidos.marcar(oportunidade.oportunidade_id);
      vlog(FILE, "OportunidadesContent.handleDelete", "removendo oportunidade da lista");
      setOportunidades((prev) =>
        prev.filter((o) => o.oportunidade_id !== oportunidade.oportunidade_id)
      );
      vlog(FILE, "OportunidadesContent.handleDelete", "decrementando total");
      setTotal((t) => Math.max(0, t - 1));
      vlog(FILE, "OportunidadesContent.handleDelete", "exibindo mensagem de sucesso");
      mostrarSucesso(
        `Oportunidade #${oportunidade.oportunidade_id} excluida com sucesso.`
      );
    } catch (err) {
      vlog(FILE, "OportunidadesContent.handleDelete", "falha na exclusao; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao excluir oportunidade.";
      vlog(FILE, "OportunidadesContent.handleDelete", "exibindo mensagem de erro");
      setError(message);
    } finally {
      vlog(FILE, "OportunidadesContent.handleDelete", "limpando id em exclusao");
      setDeletingId(null);
    }
  };

  vlog(FILE, "OportunidadesContent", "montando definicao das colunas da tabela");
  const columns: Column<Oportunidade>[] = [
    {
      key: "oportunidade_id",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      render: (o) => (
        <span className="font-mono text-xs">#{o.oportunidade_id}</span>
      ),
    },
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
        <span className="text-slate-600">{formatarData(o.data_abertura)}</span>
      ),
    },
    {
      key: "data_fechamento",
      header: "Fechamento",
      width: "120px",
      align: "center",
      sortable: true,
      render: (o) => (
        <span className="text-slate-600">{formatarData(o.data_fechamento)}</span>
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

  // FE-10: contador baseado na pagina exibida; oculto se nada foi carregado
  // (a primeira carga falhou), para nao afirmar "0 oportunidades".
  vlog(FILE, "OportunidadesContent", "calculando faixa exibida (pagina=%d, limite=%d, total=%d)", exibida.pagina, exibida.limite, total);
  const { inicio: startItem, fim: endItem } = faixaExibida(
    exibida.pagina,
    exibida.limite,
    total
  );
  vlog(FILE, "OportunidadesContent", "calculando se oculta o contador (qtd=%d)", oportunidades.length);
  const ocultarContador = erroCarga && oportunidades.length === 0;

  vlog(FILE, "OportunidadesContent", "verificando se ha filtros aplicados");
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
            {!ocultarContador && (
              <div className="text-sm text-slate-500">
                {total === 0
                  ? "0 oportunidades"
                  : `${startItem}-${endItem} de ${total} ${
                      total === 1 ? "oportunidade" : "oportunidades"
                    }`}
              </div>
            )}
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={oportunidades}
            keyExtractor={(o) => o.oportunidade_id}
            loading={loading}
            erroCarga={erroCarga}
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

        <Paginador pagina={exibida.pagina} paginas={pages} onIrPara={irParaPagina} />
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
