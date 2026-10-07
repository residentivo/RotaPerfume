"use client";

import { useEffect, useState } from "react";
import {
  faixaExibida,
  mensagemRecargaFalhou,
  useDebounceFiltros,
  usePaginaCarregada,
  useUltimaResposta,
} from "@/lib/useListaSegura";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { Paginador } from "@/components/ui/Paginador";
import { useMensagemTemporaria } from "@/lib/useMensagemTemporaria";
import { EstoqueModal } from "@/components/admin/EstoqueModal";
import { apiListEstoque, apiCreateEstoque, apiUpdateEstoque } from "@/lib/api";
import { Estoque, EstoqueInput } from "@/lib/types";
import { useSessionUser } from "@/lib/session";
import { formatarData } from "@/lib/formatarData";
import { vlog } from "@/lib/vlog";

const FILE = "admin/estoque/page.tsx";

type SortKey =
  | "id"
  | "sku"
  | "saldo"
  | "ruptura"
  | "data_snapshot"
  | "created_at"
  | "updated_at";
type SortDir = "asc" | "desc";

const RUPTURA_OPTIONS: { value: "" | "sim" | "nao"; label: string }[] = [
  { value: "", label: "Todos" },
  { value: "sim", label: "Em ruptura" },
  { value: "nao", label: "Sem ruptura" },
];

const LIMIT_OPTIONS = [
  { value: "10", label: "10 por pagina" },
  { value: "20", label: "20 por pagina" },
  { value: "50", label: "50 por pagina" },
  { value: "100", label: "100 por pagina" },
];

function EstoquePageContent() {
  // Papel vem da sessao em memoria validada por /api/auth/me (nao do cache
  // do localStorage); derivado no render, sem efeito.
  vlog(FILE, "EstoquePageContent", "calculando se usuario da sessao e admin");
  const admin = useSessionUser()?.role === "admin";
  vlog(FILE, "EstoquePageContent", "inicializando estado da lista de registros");
  const [registros, setRegistros] = useState<Estoque[]>([]);
  vlog(FILE, "EstoquePageContent", "inicializando estado de loading");
  const [loading, setLoading] = useState(true);
  vlog(FILE, "EstoquePageContent", "inicializando estado de erro");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "EstoquePageContent", "inicializando estado de erro de carga");
  const [erroCarga, setErroCarga] = useState(false);
  vlog(FILE, "EstoquePageContent", "obtendo controle de mensagem temporaria de sucesso");
  const {
    mensagem: success,
    mostrar: mostrarSucesso,
    limpar: limparSucesso,
  } = useMensagemTemporaria();

  vlog(FILE, "EstoquePageContent", "inicializando estado da busca por SKU");
  const [search, setSearch] = useState("");
  vlog(FILE, "EstoquePageContent", "inicializando estado do filtro de data");
  const [dataFiltro, setDataFiltro] = useState("");
  vlog(FILE, "EstoquePageContent", "inicializando estado do filtro de ruptura");
  const [rupturaFilter, setRupturaFilter] = useState<"" | "sim" | "nao">("");

  vlog(FILE, "EstoquePageContent", "inicializando estado da coluna de ordenacao");
  const [sortKey, setSortKey] = useState<SortKey>("sku");
  vlog(FILE, "EstoquePageContent", "inicializando estado da direcao de ordenacao");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  vlog(FILE, "EstoquePageContent", "inicializando estado da pagina atual");
  const [page, setPage] = useState(1);
  vlog(FILE, "EstoquePageContent", "inicializando estado do limite por pagina");
  const [limit, setLimit] = useState(20);
  vlog(FILE, "EstoquePageContent", "inicializando estado do total de registros");
  const [total, setTotal] = useState(0);
  vlog(FILE, "EstoquePageContent", "inicializando estado do total de paginas");
  const [pages, setPages] = useState(0);
  // FE-10: pagina/limite exibidos (a pedida, ou a ultima carregada se a
  // ultima carga falhou).
  vlog(FILE, "EstoquePageContent", "obtendo pagina exibida (page=%d, limit=%d, erroCarga=%s)", page, limit, erroCarga);
  const { exibida, registrar: registrarCarregada } = usePaginaCarregada(
    page,
    limit,
    erroCarga
  );

  // Modal state
  vlog(FILE, "EstoquePageContent", "inicializando estado de abertura do modal");
  const [modalOpen, setModalOpen] = useState(false);
  vlog(FILE, "EstoquePageContent", "inicializando estado do modo do modal");
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  vlog(FILE, "EstoquePageContent", "inicializando estado do registro em edicao");
  const [editingEstoque, setEditingEstoque] = useState<Estoque | null>(null);

  // FE-04: so a busca mais recente aplica o resultado (respostas obsoletas
  // sao descartadas), inclusive entre o efeito e as recargas imperativas.
  vlog(FILE, "EstoquePageContent", "obtendo executor de busca com descarte de respostas obsoletas");
  const executarBusca = useUltimaResposta();

  // Busca separada em requisicao pura + aplicacao do resultado no callback
  // assincrono (.then): o efeito nunca chama setState de forma sincrona.
  vlog(FILE, "EstoquePageContent", "definindo funcao buscarEstoque");
  const buscarEstoque = () => {
    vlog(FILE, "EstoquePageContent.buscarEstoque", "chamando apiListEstoque (page=%d, limit=%d, sortKey=%s, sortDir=%s)", page, limit, sortKey, sortDir);
    return apiListEstoque(
      page,
      limit,
      {
        sku: search.trim() || undefined,
        data_ate: dataFiltro || undefined,
        ruptura:
          rupturaFilter === "" ? undefined : rupturaFilter === "sim",
      },
      sortKey,
      sortDir
    );
  };

  vlog(FILE, "EstoquePageContent", "definindo funcao aplicarEstoque");
  const aplicarEstoque = (res: Awaited<ReturnType<typeof apiListEstoque>>) => {
    vlog(FILE, "EstoquePageContent.aplicarEstoque", "aplicando lista de registros (qtd=%d)", res.data.length);
    setRegistros(res.data);
    vlog(FILE, "EstoquePageContent.aplicarEstoque", "atualizando total (total=%d)", res.total);
    setTotal(res.total);
    vlog(FILE, "EstoquePageContent.aplicarEstoque", "atualizando total de paginas (pages=%d)", res.pages);
    setPages(res.pages);
    vlog(FILE, "EstoquePageContent.aplicarEstoque", "registrando pagina carregada (page=%d, limit=%d)", page, limit);
    registrarCarregada(page, limit);
    vlog(FILE, "EstoquePageContent.aplicarEstoque", "limpando erro de carga");
    setErroCarga(false);
    vlog(FILE, "EstoquePageContent.aplicarEstoque", "desativando loading");
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  vlog(FILE, "EstoquePageContent", "definindo funcao aplicarErroEstoque");
  const aplicarErroEstoque = (err: unknown): string => {
    vlog(FILE, "EstoquePageContent.aplicarErroEstoque", "extraindo mensagem do erro de carga");
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar estoque. O endpoint /api/estoque pode nao existir no backend.";
    vlog(FILE, "EstoquePageContent.aplicarErroEstoque", "exibindo mensagem de erro");
    setError(message);
    vlog(FILE, "EstoquePageContent.aplicarErroEstoque", "marcando erro de carga");
    setErroCarga(true);
    vlog(FILE, "EstoquePageContent.aplicarErroEstoque", "desativando loading");
    setLoading(false);
    return message;
  };

  // Recarga imperativa (handlers e timers). FE-10: devolve a mensagem de erro se a
  // recarga falhar (null se deu certo ou foi superada por outra busca).
  vlog(FILE, "EstoquePageContent", "definindo funcao loadEstoque");
  const loadEstoque = async (): Promise<string | null> => {
    vlog(FILE, "EstoquePageContent.loadEstoque", "ativando loading");
    setLoading(true);
    vlog(FILE, "EstoquePageContent.loadEstoque", "limpando erro");
    setError(null);
    vlog(FILE, "EstoquePageContent.loadEstoque", "inicializando variavel de falha");
    let falha: string | null = null;
    vlog(FILE, "EstoquePageContent.loadEstoque", "executando busca de estoque");
    await executarBusca(buscarEstoque(), aplicarEstoque, (err) => {
      vlog(FILE, "EstoquePageContent.loadEstoque.func", "registrando falha da recarga");
      falha = aplicarErroEstoque(err);
    });
    return falha;
  };

  // FE-10: navegacao a partir da pagina exibida. Se o destino ja e a pagina
  // pedida (a troca anterior falhou), repete a busca em vez de nao fazer nada.
  vlog(FILE, "EstoquePageContent", "definindo funcao irParaPagina");
  const irParaPagina = (n: number) => {
    vlog(FILE, "EstoquePageContent.irParaPagina", "verificando se destino e a pagina atual (destino=%d, atual=%d)", n, page);
    if (n === page) loadEstoque();
    else setPage(n);
  };

  // Paginacao/ordenacao mudou: liga o loading durante o render (padrao
  // "ajustar estado quando a entrada muda") e o efeito so faz a busca.
  vlog(FILE, "EstoquePageContent", "montando chave de paginacao/ordenacao");
  const chaveLista = `${page}|${limit}|${sortKey}|${sortDir}`;
  vlog(FILE, "EstoquePageContent", "inicializando estado da chave anterior");
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  vlog(FILE, "EstoquePageContent", "verificando se a chave de paginacao/ordenacao mudou");
  if (chaveAnterior !== chaveLista) {
    vlog(FILE, "EstoquePageContent", "atualizando chave anterior");
    setChaveAnterior(chaveLista);
    vlog(FILE, "EstoquePageContent", "ativando loading");
    setLoading(true);
    vlog(FILE, "EstoquePageContent", "limpando erro");
    setError(null);
  }

  vlog(FILE, "EstoquePageContent", "registrando efeito de busca por paginacao/ordenacao");
  useEffect(() => {
    vlog(FILE, "EstoquePageContent.useEffect", "executando busca de estoque");
    executarBusca(buscarEstoque(), aplicarEstoque, aplicarErroEstoque);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, sortKey, sortDir]);

  // Debounce da busca textual/data e reset para pagina 1 quando filtros mudam
  // FE-04: o debounce nao dispara na montagem, so quando a chave dos filtros
  // muda (evita a busca dupla e a tabela voltando para a pagina 1).
  vlog(FILE, "EstoquePageContent", "montando chave dos filtros");
  const chaveFiltros = JSON.stringify([search, dataFiltro, rupturaFilter]);
  vlog(FILE, "EstoquePageContent", "registrando debounce dos filtros");
  useDebounceFiltros(chaveFiltros, () => {
    vlog(FILE, "EstoquePageContent.useDebounceFiltros", "filtros mudaram; verificando se esta na pagina 1 (page=%d)", page);
    if (page !== 1) {
      vlog(FILE, "EstoquePageContent.useDebounceFiltros", "voltando para a pagina 1");
      setPage(1);
    } else {
      vlog(FILE, "EstoquePageContent.useDebounceFiltros", "recarregando estoque");
      loadEstoque();
    }
  });

  vlog(FILE, "EstoquePageContent", "definindo handler handleSort");
  const handleSort = (key: SortKey) => {
    vlog(FILE, "EstoquePageContent.handleSort", "verificando se coluna ja e a ordenada (key=%s)", key);
    if (sortKey === key) {
      vlog(FILE, "EstoquePageContent.handleSort", "invertendo direcao da ordenacao");
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      vlog(FILE, "EstoquePageContent.handleSort", "definindo nova coluna de ordenacao");
      setSortKey(key);
      vlog(FILE, "EstoquePageContent.handleSort", "definindo direcao asc");
      setSortDir("asc");
    }
  };

  vlog(FILE, "EstoquePageContent", "definindo handler openCreate");
  const openCreate = () => {
    vlog(FILE, "EstoquePageContent.openCreate", "definindo modo create");
    setModalMode("create");
    vlog(FILE, "EstoquePageContent.openCreate", "limpando registro em edicao");
    setEditingEstoque(null);
    vlog(FILE, "EstoquePageContent.openCreate", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "EstoquePageContent", "definindo handler openEdit");
  const openEdit = (registro: Estoque) => {
    vlog(FILE, "EstoquePageContent.openEdit", "verificando permissao de admin (admin=%s)", admin);
    if (!admin) return;
    vlog(FILE, "EstoquePageContent.openEdit", "definindo modo edit (estoque_id=%d)", registro.id);
    setModalMode("edit");
    vlog(FILE, "EstoquePageContent.openEdit", "definindo registro em edicao");
    setEditingEstoque(registro);
    vlog(FILE, "EstoquePageContent.openEdit", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "EstoquePageContent", "definindo handler handleModalSubmit");
  const handleModalSubmit = async (data: EstoqueInput) => {
    vlog(FILE, "EstoquePageContent.handleModalSubmit", "limpando erro");
    setError(null);
    vlog(FILE, "EstoquePageContent.handleModalSubmit", "iniciando envio do formulario (modo=%s)", modalMode);
    try {
      vlog(FILE, "EstoquePageContent.handleModalSubmit", "verificando modo do modal");
      if (modalMode === "create") {
        vlog(FILE, "EstoquePageContent.handleModalSubmit", "chamando apiCreateEstoque");
        const created = await apiCreateEstoque(data);
        vlog(FILE, "EstoquePageContent.handleModalSubmit", "montando mensagem de sucesso (estoque_id=%d)", created.id);
        const msg = `Registro de estoque #${created.id} (${created.sku}) criado com sucesso.`;
        vlog(FILE, "EstoquePageContent.handleModalSubmit", "recarregando estoque apos criacao");
        const erroRecarga = await loadEstoque();
        vlog(FILE, "EstoquePageContent.handleModalSubmit", "verificando falha na recarga (falhou=%s)", erroRecarga !== null);
        if (erroRecarga) setError(mensagemRecargaFalhou(msg, erroRecarga));
        else mostrarSucesso(msg);
      } else if (editingEstoque) {
        vlog(FILE, "EstoquePageContent.handleModalSubmit", "chamando apiUpdateEstoque (estoque_id=%d)", editingEstoque.id);
        const updated = await apiUpdateEstoque(editingEstoque.id, {
          saldo: data.saldo,
        });
        vlog(FILE, "EstoquePageContent.handleModalSubmit", "substituindo registro atualizado na lista");
        setRegistros((prev) =>
          prev.map((r) => (r.id === updated.id ? updated : r))
        );
        vlog(FILE, "EstoquePageContent.handleModalSubmit", "exibindo mensagem de sucesso");
        mostrarSucesso(`Registro de estoque #${updated.id} (${updated.sku}) atualizado com sucesso.`);
      }
      vlog(FILE, "EstoquePageContent.handleModalSubmit", "fechando modal");
      setModalOpen(false);
    } catch (err) {
      vlog(FILE, "EstoquePageContent.handleModalSubmit", "falha no envio; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao salvar registro de estoque.";
      vlog(FILE, "EstoquePageContent.handleModalSubmit", "convertendo para mensagem amigavel se for erro de permissao");
      const friendly = /403|forbidden|permiss|acesso negado/i.test(message)
        ? "Voce nao tem permissao para criar ou editar registros de estoque (acao restrita a administradores)."
        : message;
      // Relancado com mensagem amigavel: o EstoqueModal exibe este erro no
      // proprio Alert do formulario (mesmo padrao do ProdutoModal).
      vlog(FILE, "EstoquePageContent.handleModalSubmit", "relancando erro para o modal");
      throw new Error(friendly);
    }
  };

  vlog(FILE, "EstoquePageContent", "montando definicao das colunas da tabela (admin=%s)", admin);
  const columns: Column<Estoque>[] = [
    {
      key: "id",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      render: (e) => <span className="font-mono text-xs">#{e.id}</span>,
    },
    {
      key: "sku",
      header: "SKU",
      sortable: true,
      render: (e) =>
        admin ? (
          <button
            type="button"
            onClick={() => openEdit(e)}
            className="text-left hover:text-primary-600"
            title="Editar registro de estoque"
          >
            <span className="block font-mono text-xs font-medium text-slate-900 hover:underline">
              {e.sku}
            </span>
            {e.produto_descricao && (
              <span className="block text-xs text-slate-500">
                {e.produto_descricao}
              </span>
            )}
          </button>
        ) : (
          <span>
            <span className="block font-mono text-xs font-medium text-slate-900">
              {e.sku}
            </span>
            {e.produto_descricao && (
              <span className="block text-xs text-slate-500">
                {e.produto_descricao}
              </span>
            )}
          </span>
        ),
    },
    {
      key: "saldo",
      header: "Saldo",
      width: "110px",
      align: "right",
      sortable: true,
      render: (e) => <span className="text-slate-700">{e.saldo}</span>,
    },
    {
      key: "ruptura",
      header: "Ruptura",
      width: "120px",
      align: "center",
      sortable: true,
      render: (e) => (
        <span
          className={[
            "inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium",
            e.ruptura
              ? "bg-red-100 text-red-700"
              : "bg-green-100 text-green-700",
          ].join(" ")}
        >
          <span
            className={[
              "h-2 w-2 rounded-full",
              e.ruptura ? "bg-red-500" : "bg-green-500",
            ].join(" ")}
          />
          {e.ruptura ? "Em ruptura" : "OK"}
        </span>
      ),
    },
    {
      key: "data_snapshot",
      header: "Data do snapshot",
      width: "150px",
      align: "center",
      sortable: true,
      render: (e) => (
        <span className="text-slate-600">{formatarData(e.data_snapshot)}</span>
      ),
    },
    ...(admin
      ? ([
          {
            key: "actions",
            header: "Acoes",
            width: "110px",
            align: "right",
            render: (e: Estoque) => (
              <div className="inline-flex items-center justify-end gap-2">
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => openEdit(e)}
                  title="Editar registro de estoque"
                >
                  Editar
                </Button>
              </div>
            ),
          } as Column<Estoque>,
        ])
      : []),
  ];

  // FE-10: contador baseado na pagina exibida; oculto se nada foi carregado
  // (a primeira carga falhou), para nao afirmar "0 registros".
  vlog(FILE, "EstoquePageContent", "calculando faixa exibida (pagina=%d, limite=%d, total=%d)", exibida.pagina, exibida.limite, total);
  const { inicio: startItem, fim: endItem } = faixaExibida(
    exibida.pagina,
    exibida.limite,
    total
  );
  vlog(FILE, "EstoquePageContent", "calculando se oculta o contador (qtd=%d)", registros.length);
  const ocultarContador = erroCarga && registros.length === 0;
  vlog(FILE, "EstoquePageContent", "verificando se ha filtros aplicados");
  const hasFilters = search || dataFiltro || rupturaFilter;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Estoque</h1>
          <p className="mt-1 text-sm text-slate-500">
            Consulte a posicao de estoque por SKU. Sem filtro de data, mostra
            a ultima posicao de cada produto.
          </p>
        </div>
        {admin && <Button onClick={openCreate}>+ Novo registro</Button>}
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
                  placeholder="SKU..."
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
                  label="Pesquisar por data"
                  type="date"
                  value={dataFiltro}
                  onChange={(e) => setDataFiltro(e.target.value)}
                  helperText="Ultima posicao de cada produto ate a data"
                />
              </div>
              <div className="sm:w-44">
                <Select
                  label="Ruptura"
                  options={RUPTURA_OPTIONS}
                  value={rupturaFilter}
                  onChange={(e) =>
                    setRupturaFilter(e.target.value as "" | "sim" | "nao")
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
                  ? "0 registros"
                  : `${startItem}-${endItem} de ${total} ${
                      total === 1 ? "registro" : "registros"
                    }`}
              </div>
            )}
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={registros}
            keyExtractor={(e) => e.id}
            loading={loading}
            erroCarga={erroCarga}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              hasFilters
                ? "Nenhum registro de estoque encontrado para os filtros aplicados."
                : "Nenhum registro de estoque cadastrado."
            }
          />
        </div>

        <Paginador pagina={exibida.pagina} paginas={pages} onIrPara={irParaPagina} />
      </Card>

      <div className="mt-4 text-xs text-slate-400">
        <strong>Nota:</strong> A busca por SKU e o filtro de data sao
        aplicados via API. Criar/editar registros de estoque e restrito a
        administradores. Verifique se o endpoint <code>GET /api/estoque</code>{" "}
        esta implementado no backend caso a lista fique vazia ou retorne erro.
      </div>

      <EstoqueModal
        open={modalOpen}
        mode={modalMode}
        estoque={editingEstoque}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}

export default function EstoquePage() {
  return (
    <ProtectedRoute requireAdmin>
      <EstoquePageContent />
    </ProtectedRoute>
  );
}
