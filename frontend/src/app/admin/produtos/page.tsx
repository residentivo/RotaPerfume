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
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { Paginador } from "@/components/ui/Paginador";
import { useMensagemTemporaria } from "@/lib/useMensagemTemporaria";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { ProdutoModal } from "@/components/admin/ProdutoModal";
import {
  apiListProdutos,
  apiToggleProdutoStatus,
  apiCreateProduto,
  apiUpdateProduto,
} from "@/lib/api";
import { Produto, ProdutoInput } from "@/lib/types";
import { vlog } from "@/lib/vlog";

const FILE = "admin/produtos/page.tsx";

type SortKey =
  | "id"
  | "sku"
  | "descricao"
  | "categoria"
  | "marca"
  | "preco_tabela"
  | "ativo";
type SortDir = "asc" | "desc";

type ActionState = {
  type: "toggle" | null;
  produtoId: number | null;
};

const STATUS_OPTIONS: { value: "" | "ativo" | "inativo"; label: string }[] = [
  { value: "", label: "Todos os status" },
  { value: "ativo", label: "Ativo" },
  { value: "inativo", label: "Inativo" },
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

function fmtPreco(v: number): string {
  return currencyFmt.format(v ?? 0);
}

function ProdutosPageContent() {
  vlog(FILE, "ProdutosPageContent", "inicializando estado da lista de produtos");
  const [produtos, setProdutos] = useState<Produto[]>([]);
  vlog(FILE, "ProdutosPageContent", "inicializando estado de loading");
  const [loading, setLoading] = useState(true);
  vlog(FILE, "ProdutosPageContent", "inicializando estado de erro");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "ProdutosPageContent", "inicializando estado de erro de carga");
  const [erroCarga, setErroCarga] = useState(false);
  vlog(FILE, "ProdutosPageContent", "obtendo controle de mensagem temporaria de sucesso");
  const {
    mensagem: success,
    mostrar: mostrarSucesso,
    limpar: limparSucesso,
  } = useMensagemTemporaria();
  vlog(FILE, "ProdutosPageContent", "inicializando estado da busca textual");
  const [search, setSearch] = useState("");
  vlog(FILE, "ProdutosPageContent", "inicializando estado do filtro de categoria");
  const [categoriaFilter, setCategoriaFilter] = useState("");
  vlog(FILE, "ProdutosPageContent", "inicializando estado do filtro de marca");
  const [marcaFilter, setMarcaFilter] = useState("");
  vlog(FILE, "ProdutosPageContent", "inicializando estado do filtro de status");
  const [statusFilter, setStatusFilter] = useState<"" | "ativo" | "inativo">("");
  vlog(FILE, "ProdutosPageContent", "inicializando estado da coluna de ordenacao");
  const [sortKey, setSortKey] = useState<SortKey>("id");
  vlog(FILE, "ProdutosPageContent", "inicializando estado da direcao de ordenacao");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  vlog(FILE, "ProdutosPageContent", "inicializando estado da acao em andamento");
  const [action, setAction] = useState<ActionState>({ type: null, produtoId: null });

  vlog(FILE, "ProdutosPageContent", "inicializando estado da pagina atual");
  const [page, setPage] = useState(1);
  vlog(FILE, "ProdutosPageContent", "inicializando estado do limite por pagina");
  const [limit, setLimit] = useState(20);
  vlog(FILE, "ProdutosPageContent", "inicializando estado do total de registros");
  const [total, setTotal] = useState(0);
  vlog(FILE, "ProdutosPageContent", "inicializando estado do total de paginas");
  const [pages, setPages] = useState(0);
  // FE-10: pagina/limite exibidos (a pedida, ou a ultima carregada se a
  // ultima carga falhou).
  vlog(FILE, "ProdutosPageContent", "obtendo pagina exibida (page=%d, limit=%d, erroCarga=%s)", page, limit, erroCarga);
  const { exibida, registrar: registrarCarregada } = usePaginaCarregada(
    page,
    limit,
    erroCarga
  );

  // Modal state
  vlog(FILE, "ProdutosPageContent", "inicializando estado de abertura do modal");
  const [modalOpen, setModalOpen] = useState(false);
  vlog(FILE, "ProdutosPageContent", "inicializando estado do modo do modal");
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  vlog(FILE, "ProdutosPageContent", "inicializando estado do produto em edicao");
  const [editingProduto, setEditingProduto] = useState<Produto | null>(null);

  // FE-04: so a busca mais recente aplica o resultado (respostas obsoletas
  // sao descartadas), inclusive entre o efeito e as recargas imperativas.
  vlog(FILE, "ProdutosPageContent", "obtendo executor de busca com descarte de respostas obsoletas");
  const executarBusca = useUltimaResposta();

  // Busca separada em requisicao pura + aplicacao do resultado no callback
  // assincrono (.then): o efeito nunca chama setState de forma sincrona.
  vlog(FILE, "ProdutosPageContent", "definindo funcao buscarProdutos");
  const buscarProdutos = () => {
    vlog(FILE, "ProdutosPageContent.buscarProdutos", "chamando apiListProdutos (page=%d, limit=%d, sortKey=%s, sortDir=%s)", page, limit, sortKey, sortDir);
    return apiListProdutos(
      page,
      limit,
      {
        categoria: categoriaFilter || undefined,
        marca: marcaFilter || undefined,
        ativo:
          statusFilter === ""
            ? undefined
            : statusFilter === "ativo",
        q: search.trim() || undefined,
      },
      sortKey,
      sortDir
    );
  };

  vlog(FILE, "ProdutosPageContent", "definindo funcao aplicarProdutos");
  const aplicarProdutos = (res: Awaited<ReturnType<typeof apiListProdutos>>) => {
    vlog(FILE, "ProdutosPageContent.aplicarProdutos", "aplicando lista de produtos (qtd=%d)", res.data.length);
    setProdutos(res.data);
    vlog(FILE, "ProdutosPageContent.aplicarProdutos", "atualizando total (total=%d)", res.total);
    setTotal(res.total);
    vlog(FILE, "ProdutosPageContent.aplicarProdutos", "atualizando total de paginas (pages=%d)", res.pages);
    setPages(res.pages);
    vlog(FILE, "ProdutosPageContent.aplicarProdutos", "registrando pagina carregada (page=%d, limit=%d)", page, limit);
    registrarCarregada(page, limit);
    vlog(FILE, "ProdutosPageContent.aplicarProdutos", "limpando erro de carga");
    setErroCarga(false);
    vlog(FILE, "ProdutosPageContent.aplicarProdutos", "desativando loading");
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  vlog(FILE, "ProdutosPageContent", "definindo funcao aplicarErroProdutos");
  const aplicarErroProdutos = (err: unknown): string => {
    vlog(FILE, "ProdutosPageContent.aplicarErroProdutos", "extraindo mensagem do erro de carga");
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar produtos. O endpoint /api/produtos pode nao existir no backend.";
    vlog(FILE, "ProdutosPageContent.aplicarErroProdutos", "exibindo mensagem de erro");
    setError(message);
    vlog(FILE, "ProdutosPageContent.aplicarErroProdutos", "marcando erro de carga");
    setErroCarga(true);
    vlog(FILE, "ProdutosPageContent.aplicarErroProdutos", "desativando loading");
    setLoading(false);
    return message;
  };

  // Recarga imperativa (handlers e timers). FE-10: devolve a mensagem de erro se a
  // recarga falhar (null se deu certo ou foi superada por outra busca).
  vlog(FILE, "ProdutosPageContent", "definindo funcao loadProdutos");
  const loadProdutos = async (): Promise<string | null> => {
    vlog(FILE, "ProdutosPageContent.loadProdutos", "ativando loading");
    setLoading(true);
    vlog(FILE, "ProdutosPageContent.loadProdutos", "limpando erro");
    setError(null);
    vlog(FILE, "ProdutosPageContent.loadProdutos", "inicializando variavel de falha");
    let falha: string | null = null;
    vlog(FILE, "ProdutosPageContent.loadProdutos", "executando busca de produtos");
    await executarBusca(buscarProdutos(), aplicarProdutos, (err) => {
      vlog(FILE, "ProdutosPageContent.loadProdutos.func", "registrando falha da recarga");
      falha = aplicarErroProdutos(err);
    });
    return falha;
  };

  // FE-10: navegacao a partir da pagina exibida. Se o destino ja e a pagina
  // pedida (a troca anterior falhou), repete a busca em vez de nao fazer nada.
  vlog(FILE, "ProdutosPageContent", "definindo funcao irParaPagina");
  const irParaPagina = (n: number) => {
    vlog(FILE, "ProdutosPageContent.irParaPagina", "verificando se destino e a pagina atual (destino=%d, atual=%d)", n, page);
    if (n === page) loadProdutos();
    else setPage(n);
  };

  // Paginacao/ordenacao mudou: liga o loading durante o render (padrao
  // "ajustar estado quando a entrada muda") e o efeito so faz a busca.
  vlog(FILE, "ProdutosPageContent", "montando chave de paginacao/ordenacao");
  const chaveLista = `${page}|${limit}|${sortKey}|${sortDir}`;
  vlog(FILE, "ProdutosPageContent", "inicializando estado da chave anterior");
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  vlog(FILE, "ProdutosPageContent", "verificando se a chave de paginacao/ordenacao mudou");
  if (chaveAnterior !== chaveLista) {
    vlog(FILE, "ProdutosPageContent", "atualizando chave anterior");
    setChaveAnterior(chaveLista);
    vlog(FILE, "ProdutosPageContent", "ativando loading");
    setLoading(true);
    vlog(FILE, "ProdutosPageContent", "limpando erro");
    setError(null);
  }

  vlog(FILE, "ProdutosPageContent", "registrando efeito de busca por paginacao/ordenacao");
  useEffect(() => {
    vlog(FILE, "ProdutosPageContent.useEffect", "executando busca de produtos");
    executarBusca(buscarProdutos(), aplicarProdutos, aplicarErroProdutos);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual e reset para pagina 1 quando filtros mudam
  // FE-04: o debounce nao dispara na montagem, so quando a chave dos filtros
  // muda (evita a busca dupla e a tabela voltando para a pagina 1).
  vlog(FILE, "ProdutosPageContent", "montando chave dos filtros");
  const chaveFiltros = JSON.stringify([search, categoriaFilter, marcaFilter, statusFilter]);
  vlog(FILE, "ProdutosPageContent", "registrando debounce dos filtros");
  useDebounceFiltros(chaveFiltros, () => {
    vlog(FILE, "ProdutosPageContent.useDebounceFiltros", "filtros mudaram; verificando se esta na pagina 1 (page=%d)", page);
    if (page !== 1) {
      vlog(FILE, "ProdutosPageContent.useDebounceFiltros", "voltando para a pagina 1");
      setPage(1);
    } else {
      vlog(FILE, "ProdutosPageContent.useDebounceFiltros", "recarregando produtos");
      loadProdutos();
    }
  });

  vlog(FILE, "ProdutosPageContent", "definindo handler handleSort");
  const handleSort = (key: SortKey) => {
    vlog(FILE, "ProdutosPageContent.handleSort", "verificando se coluna ja e a ordenada (key=%s)", key);
    if (sortKey === key) {
      vlog(FILE, "ProdutosPageContent.handleSort", "invertendo direcao da ordenacao");
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      vlog(FILE, "ProdutosPageContent.handleSort", "definindo nova coluna de ordenacao");
      setSortKey(key);
      vlog(FILE, "ProdutosPageContent.handleSort", "definindo direcao asc");
      setSortDir("asc");
    }
  };

  vlog(FILE, "ProdutosPageContent", "definindo handler handleToggleStatus");
  const handleToggleStatus = async (produto: Produto) => {
    vlog(FILE, "ProdutosPageContent.handleToggleStatus", "calculando novo status (produto_id=%d)", produto.id);
    const novoStatus = !produto.ativo;
    vlog(FILE, "ProdutosPageContent.handleToggleStatus", "definindo acao (novoStatus=%s)", novoStatus);
    const acao = novoStatus ? "reativar" : "inativar";
    vlog(FILE, "ProdutosPageContent.handleToggleStatus", "pedindo confirmacao ao usuario");
    const ok = window.confirm(
      `Tem certeza que deseja ${acao} o produto "${produto.descricao}"?`
    );
    vlog(FILE, "ProdutosPageContent.handleToggleStatus", "verificando confirmacao (ok=%s)", ok);
    if (!ok) return;

    vlog(FILE, "ProdutosPageContent.handleToggleStatus", "marcando acao toggle em andamento");
    setAction({ type: "toggle", produtoId: produto.id });
    vlog(FILE, "ProdutosPageContent.handleToggleStatus", "limpando erro");
    setError(null);
    vlog(FILE, "ProdutosPageContent.handleToggleStatus", "limpando mensagem de sucesso");
    limparSucesso();
    vlog(FILE, "ProdutosPageContent.handleToggleStatus", "iniciando chamada de alteracao de status");
    try {
      vlog(FILE, "ProdutosPageContent.handleToggleStatus", "chamando apiToggleProdutoStatus");
      const updated = await apiToggleProdutoStatus(produto.id, novoStatus);
      vlog(FILE, "ProdutosPageContent.handleToggleStatus", "substituindo produto atualizado na lista (produto_id=%d)", updated.id);
      setProdutos((prev) =>
        prev.map((p) => (p.id === updated.id ? updated : p))
      );
      vlog(FILE, "ProdutosPageContent.handleToggleStatus", "exibindo mensagem de sucesso");
      mostrarSucesso(
        `Produto ${novoStatus ? "reativado" : "inativado"} com sucesso.`
      );
    } catch (err) {
      vlog(FILE, "ProdutosPageContent.handleToggleStatus", "falha ao alterar status; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao alterar status.";
      vlog(FILE, "ProdutosPageContent.handleToggleStatus", "exibindo mensagem de erro");
      setError(message);
    } finally {
      vlog(FILE, "ProdutosPageContent.handleToggleStatus", "limpando acao em andamento");
      setAction({ type: null, produtoId: null });
    }
  };

  vlog(FILE, "ProdutosPageContent", "definindo handler openCreate");
  const openCreate = () => {
    vlog(FILE, "ProdutosPageContent.openCreate", "definindo modo create");
    setModalMode("create");
    vlog(FILE, "ProdutosPageContent.openCreate", "limpando produto em edicao");
    setEditingProduto(null);
    vlog(FILE, "ProdutosPageContent.openCreate", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "ProdutosPageContent", "definindo handler openEdit");
  const openEdit = (produto: Produto) => {
    vlog(FILE, "ProdutosPageContent.openEdit", "definindo modo edit (produto_id=%d)", produto.id);
    setModalMode("edit");
    vlog(FILE, "ProdutosPageContent.openEdit", "definindo produto em edicao");
    setEditingProduto(produto);
    vlog(FILE, "ProdutosPageContent.openEdit", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "ProdutosPageContent", "definindo handler handleModalSubmit");
  const handleModalSubmit = async (data: ProdutoInput) => {
    vlog(FILE, "ProdutosPageContent.handleModalSubmit", "limpando erro");
    setError(null);
    vlog(FILE, "ProdutosPageContent.handleModalSubmit", "verificando modo do modal (modo=%s)", modalMode);
    if (modalMode === "create") {
      vlog(FILE, "ProdutosPageContent.handleModalSubmit", "chamando apiCreateProduto");
      const created = await apiCreateProduto(data);
      vlog(FILE, "ProdutosPageContent.handleModalSubmit", "montando mensagem de sucesso (produto_id=%d)", created.id);
      const msg = `Produto "${created.descricao}" criado com sucesso.`;
      vlog(FILE, "ProdutosPageContent.handleModalSubmit", "recarregando produtos apos criacao");
      const erroRecarga = await loadProdutos();
      vlog(FILE, "ProdutosPageContent.handleModalSubmit", "verificando falha na recarga (falhou=%s)", erroRecarga !== null);
      if (erroRecarga) setError(mensagemRecargaFalhou(msg, erroRecarga));
      else mostrarSucesso(msg);
    } else if (editingProduto) {
      vlog(FILE, "ProdutosPageContent.handleModalSubmit", "chamando apiUpdateProduto (produto_id=%d)", editingProduto.id);
      const updated = await apiUpdateProduto(editingProduto.id, data);
      vlog(FILE, "ProdutosPageContent.handleModalSubmit", "substituindo produto atualizado na lista");
      setProdutos((prev) =>
        prev.map((p) => (p.id === updated.id ? updated : p))
      );
      vlog(FILE, "ProdutosPageContent.handleModalSubmit", "exibindo mensagem de sucesso");
      mostrarSucesso(`Produto "${updated.descricao}" atualizado com sucesso.`);
    }
    vlog(FILE, "ProdutosPageContent.handleModalSubmit", "fechando modal");
    setModalOpen(false);
  };

  vlog(FILE, "ProdutosPageContent", "montando definicao das colunas da tabela");
  const columns: Column<Produto>[] = [
    {
      key: "id",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      render: (p) => <span className="font-mono text-xs">#{p.id}</span>,
    },
    {
      key: "descricao",
      header: "Descricao",
      sortable: true,
      render: (p) => (
        <button
          type="button"
          onClick={() => openEdit(p)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Editar produto"
        >
          {p.descricao}
        </button>
      ),
    },
    {
      key: "sku",
      header: "SKU",
      width: "140px",
      sortable: true,
      render: (p) => (
        <span className="font-mono text-xs text-slate-600">{p.sku}</span>
      ),
    },
    {
      key: "categoria",
      header: "Categoria",
      width: "160px",
      sortable: true,
      render: (p) => <span className="text-slate-600">{p.categoria || "-"}</span>,
    },
    {
      key: "marca",
      header: "Marca",
      width: "140px",
      sortable: true,
      render: (p) => <span className="text-slate-600">{p.marca || "-"}</span>,
    },
    {
      key: "preco_tabela",
      header: "Preco",
      width: "130px",
      align: "right",
      sortable: true,
      render: (p) => (
        <span className="text-slate-700">{fmtPreco(p.preco_tabela)}</span>
      ),
    },
    {
      key: "ativo",
      header: "Status",
      width: "140px",
      align: "center",
      sortable: true,
      render: (p) => (
        <button
          type="button"
          onClick={() => handleToggleStatus(p)}
          disabled={action.type === "toggle" && action.produtoId === p.id}
          title={p.ativo ? "Clique para inativar" : "Clique para reativar"}
          className={[
            "inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium transition-colors",
            "disabled:cursor-not-allowed disabled:opacity-60",
            p.ativo
              ? "bg-green-100 text-green-700 hover:bg-green-200"
              : "bg-red-100 text-red-700 hover:bg-red-200",
          ].join(" ")}
        >
          <span
            className={[
              "h-2 w-2 rounded-full",
              p.ativo ? "bg-green-500" : "bg-red-500",
            ].join(" ")}
          />
          {p.ativo ? "Ativo" : "Inativo"}
        </button>
      ),
    },
    {
      key: "actions",
      header: "Acoes",
      width: "120px",
      align: "right",
      render: (p) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(p)}
            title="Editar produto"
          >
            Editar
          </Button>
        </div>
      ),
    },
  ];

  // FE-10: contador baseado na pagina exibida; oculto se nada foi carregado
  // (a primeira carga falhou), para nao afirmar "0 produtos".
  vlog(FILE, "ProdutosPageContent", "calculando faixa exibida (pagina=%d, limite=%d, total=%d)", exibida.pagina, exibida.limite, total);
  const { inicio: startItem, fim: endItem } = faixaExibida(
    exibida.pagina,
    exibida.limite,
    total
  );
  vlog(FILE, "ProdutosPageContent", "calculando se oculta o contador (qtd=%d)", produtos.length);
  const ocultarContador = erroCarga && produtos.length === 0;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Produtos</h1>
          <p className="mt-1 text-sm text-slate-500">
            Consulte e gerencie o catalogo de produtos.
          </p>
        </div>
        <Button onClick={openCreate}>+ Novo Produto</Button>
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
                  placeholder="Descricao ou SKU..."
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
                <Input
                  label="Categoria"
                  placeholder="Ex: Perfumaria"
                  value={categoriaFilter}
                  onChange={(e) => setCategoriaFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-48">
                <Input
                  label="Marca"
                  placeholder="Ex: Marca X"
                  value={marcaFilter}
                  onChange={(e) => setMarcaFilter(e.target.value)}
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
                  ? "0 produtos"
                  : `${startItem}-${endItem} de ${total} ${
                      total === 1 ? "produto" : "produtos"
                    }`}
              </div>
            )}
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={produtos}
            keyExtractor={(p) => p.id}
            loading={loading}
            erroCarga={erroCarga}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              search || categoriaFilter || marcaFilter || statusFilter
                ? "Nenhum produto encontrado para os filtros aplicados."
                : "Nenhum produto cadastrado."
            }
          />
        </div>

        <Paginador pagina={exibida.pagina} paginas={pages} onIrPara={irParaPagina} />
      </Card>

      <div className="mt-4 text-xs text-slate-400">
        <strong>Nota:</strong> A busca e os filtros de categoria/marca/status
        sao aplicados via API. Caso a lista esteja vazia ou retorne erro,
        verifique se os endpoints <code>GET /api/produtos</code> e{" "}
        <code>PATCH /api/produtos/&#123;id&#125;/inativar</code> estao
        implementados no backend.
      </div>

      <ProdutoModal
        open={modalOpen}
        mode={modalMode}
        produto={editingProduto}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}

export default function ProdutosPage() {
  return (
    <ProtectedRoute requireAdmin>
      <ProdutosPageContent />
    </ProtectedRoute>
  );
}
