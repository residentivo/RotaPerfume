"use client";

import { useEffect, useMemo, useState } from "react";
import {
  faixaExibida,
  usePaginaCarregada,
  useUltimaResposta,
} from "@/lib/useListaSegura";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Table, Badge, Column } from "@/components/ui/Table";
import { Paginador } from "@/components/ui/Paginador";
import { Select } from "@/components/ui/Select";
import { apiListSenhaHistorico } from "@/lib/api";
import { SenhaHistoricoItem, TipoReset } from "@/lib/types";
import { formatarDataHora } from "@/lib/formatarData";
import { vlog } from "@/lib/vlog";

const FILE = "admin/senha-historico/page.tsx";

type SortKey = "id" | "created_at" | "usuario_nome" | "tipo_reset" | "resetado_por_nome" | "ip_origem";
type SortDir = "asc" | "desc";

// Mapeia a sortKey interna do frontend para o campo aceito pelo backend em
// order_by. FE-13: a whitelist do backend aceita id, usuario_id, tipo_reset,
// created_at, usuario_nome, resetado_por_nome e ip_origem. O Record completo
// (nao Partial) garante em tempo de compilacao que toda coluna ordenavel da
// tela tem chave valida no backend.
const ORDER_BY_MAP: Record<SortKey, string> = {
  id: "id",
  created_at: "created_at",
  usuario_nome: "usuario_nome",
  tipo_reset: "tipo_reset",
  resetado_por_nome: "resetado_por_nome",
  ip_origem: "ip_origem",
};

const TIPO_OPTIONS: { value: "" | TipoReset; label: string }[] = [
  { value: "", label: "Todos os tipos" },
  { value: "usuario", label: "Proprio" },
  { value: "admin", label: "Admin" },
  { value: "primeiro_acesso", label: "Primeiro Acesso" },
  { value: "esquecimento", label: "Esquecimento" },
];

const LIMIT_OPTIONS = [
  { value: "10", label: "10 por pagina" },
  { value: "20", label: "20 por pagina" },
  { value: "50", label: "50 por pagina" },
  { value: "100", label: "100 por pagina" },
];

function tipoBadgeColor(
  tipo: TipoReset
): "blue" | "yellow" | "green" | "red" | "gray" {
  switch (tipo) {
    case "usuario":
      return "blue";
    case "admin":
      return "yellow";
    case "primeiro_acesso":
      return "green";
    case "esquecimento":
      return "red";
    default:
      return "gray";
  }
}

function tipoLabel(tipo: TipoReset): string {
  switch (tipo) {
    case "usuario":
      return "Proprio";
    case "admin":
      return "Admin";
    case "primeiro_acesso":
      return "Primeiro Acesso";
    case "esquecimento":
      return "Esquecimento";
    default:
      // Valor desconhecido: exibe o valor bruto em vez de rotular errado.
      return tipo;
  }
}

export default function SenhaHistoricoPage() {
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado da lista de registros");
  const [items, setItems] = useState<SenhaHistoricoItem[]>([]);
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado de loading");
  const [loading, setLoading] = useState(true);
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado de erro");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado de erro de carga");
  const [erroCarga, setErroCarga] = useState(false);
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado da busca textual");
  const [search, setSearch] = useState("");
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado do filtro de tipo");
  const [tipo, setTipo] = useState<"" | TipoReset>("");

  vlog(FILE, "SenhaHistoricoPage", "inicializando estado da pagina atual");
  const [page, setPage] = useState(1);
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado do limite por pagina");
  const [limit, setLimit] = useState(20);
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado do total de registros");
  const [total, setTotal] = useState(0);
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado do total de paginas");
  const [pages, setPages] = useState(0);
  // FE-10: pagina/limite exibidos (a pedida, ou a ultima carregada se a
  // ultima carga falhou).
  vlog(FILE, "SenhaHistoricoPage", "obtendo pagina exibida (page=%d, limit=%d, erroCarga=%s)", page, limit, erroCarga);
  const { exibida, registrar: registrarCarregada } = usePaginaCarregada(
    page,
    limit,
    erroCarga
  );

  vlog(FILE, "SenhaHistoricoPage", "inicializando estado da coluna de ordenacao");
  const [sortKey, setSortKey] = useState<SortKey>("created_at");
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado da direcao de ordenacao");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  // FE-04: so a busca mais recente aplica o resultado (respostas obsoletas
  // sao descartadas), inclusive entre o efeito e as recargas imperativas.
  vlog(FILE, "SenhaHistoricoPage", "obtendo executor de busca com descarte de respostas obsoletas");
  const executarBusca = useUltimaResposta();

  // Busca separada em requisicao pura + aplicacao do resultado no callback
  // assincrono (.then): o efeito nunca chama setState de forma sincrona.
  vlog(FILE, "SenhaHistoricoPage", "definindo funcao buscar");
  const buscar = () => {
    vlog(FILE, "SenhaHistoricoPage.buscar", "chamando apiListSenhaHistorico (page=%d, limit=%d, tipo=%s, sortKey=%s, sortDir=%s)", page, limit, tipo, sortKey, sortDir);
    return apiListSenhaHistorico(
      page,
      limit,
      undefined,
      tipo || undefined,
      ORDER_BY_MAP[sortKey],
      sortDir
    );
  };

  vlog(FILE, "SenhaHistoricoPage", "definindo funcao aplicar");
  const aplicar = (res: Awaited<ReturnType<typeof apiListSenhaHistorico>>) => {
    vlog(FILE, "SenhaHistoricoPage.aplicar", "aplicando lista de registros (qtd=%d)", res.data.length);
    setItems(res.data);
    vlog(FILE, "SenhaHistoricoPage.aplicar", "atualizando total (total=%d)", res.total);
    setTotal(res.total);
    vlog(FILE, "SenhaHistoricoPage.aplicar", "atualizando total de paginas (pages=%d)", res.pages);
    setPages(res.pages);
    vlog(FILE, "SenhaHistoricoPage.aplicar", "registrando pagina carregada (page=%d, limit=%d)", page, limit);
    registrarCarregada(page, limit);
    vlog(FILE, "SenhaHistoricoPage.aplicar", "limpando erro de carga");
    setErroCarga(false);
    vlog(FILE, "SenhaHistoricoPage.aplicar", "desativando loading");
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  vlog(FILE, "SenhaHistoricoPage", "definindo funcao aplicarErro");
  const aplicarErro = (err: unknown) => {
    vlog(FILE, "SenhaHistoricoPage.aplicarErro", "extraindo mensagem do erro de carga");
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar historico de senhas. O endpoint /api/senha-historico pode nao existir no backend.";
    vlog(FILE, "SenhaHistoricoPage.aplicarErro", "exibindo mensagem de erro");
    setError(message);
    vlog(FILE, "SenhaHistoricoPage.aplicarErro", "marcando erro de carga");
    setErroCarga(true);
    vlog(FILE, "SenhaHistoricoPage.aplicarErro", "desativando loading");
    setLoading(false);
  };

  // Recarga imperativa (botao "Atualizar").
  vlog(FILE, "SenhaHistoricoPage", "definindo funcao load");
  const load = async () => {
    vlog(FILE, "SenhaHistoricoPage.load", "ativando loading");
    setLoading(true);
    vlog(FILE, "SenhaHistoricoPage.load", "limpando erro");
    setError(null);
    vlog(FILE, "SenhaHistoricoPage.load", "executando busca do historico");
    await executarBusca(buscar(), aplicar, aplicarErro);
  };

  // Reset para pagina 1 quando filtros mudam — ajustado durante o render
  // (padrao "ajustar estado quando a entrada muda"), sem efeito.
  vlog(FILE, "SenhaHistoricoPage", "montando chave dos filtros");
  const chaveFiltros = `${search}|${tipo}`;
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado dos filtros anteriores");
  const [filtrosAnteriores, setFiltrosAnteriores] = useState(chaveFiltros);
  vlog(FILE, "SenhaHistoricoPage", "verificando se os filtros mudaram");
  if (filtrosAnteriores !== chaveFiltros) {
    vlog(FILE, "SenhaHistoricoPage", "atualizando filtros anteriores");
    setFiltrosAnteriores(chaveFiltros);
    vlog(FILE, "SenhaHistoricoPage", "voltando para a pagina 1");
    setPage(1);
  }

  // Paginacao/ordenacao/tipo mudou: liga o loading durante o render e o
  // efeito so faz a busca.
  vlog(FILE, "SenhaHistoricoPage", "montando chave de paginacao/ordenacao/tipo");
  const chaveLista = `${page}|${limit}|${tipo}|${sortKey}|${sortDir}`;
  vlog(FILE, "SenhaHistoricoPage", "inicializando estado da chave anterior");
  const [chaveAnterior, setChaveAnterior] = useState(chaveLista);
  vlog(FILE, "SenhaHistoricoPage", "verificando se a chave de paginacao/ordenacao mudou");
  if (chaveAnterior !== chaveLista) {
    vlog(FILE, "SenhaHistoricoPage", "atualizando chave anterior");
    setChaveAnterior(chaveLista);
    vlog(FILE, "SenhaHistoricoPage", "ativando loading");
    setLoading(true);
    vlog(FILE, "SenhaHistoricoPage", "limpando erro");
    setError(null);
  }

  vlog(FILE, "SenhaHistoricoPage", "registrando efeito de busca por paginacao/ordenacao/tipo");
  useEffect(() => {
    vlog(FILE, "SenhaHistoricoPage.useEffect", "executando busca do historico");
    executarBusca(buscar(), aplicar, aplicarErro);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, tipo, sortKey, sortDir]);

  vlog(FILE, "SenhaHistoricoPage", "definindo handler handleSort");
  const handleSort = (key: SortKey) => {
    vlog(FILE, "SenhaHistoricoPage.handleSort", "verificando se coluna ja e a ordenada (key=%s)", key);
    if (sortKey === key) {
      vlog(FILE, "SenhaHistoricoPage.handleSort", "invertendo direcao da ordenacao");
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      vlog(FILE, "SenhaHistoricoPage.handleSort", "definindo nova coluna de ordenacao");
      setSortKey(key);
      vlog(FILE, "SenhaHistoricoPage.handleSort", "definindo direcao asc");
      setSortDir("asc");
    }
  };

  // FE-10: navegacao a partir da pagina exibida. Se o destino ja e a pagina
  // pedida (a troca anterior falhou), repete a busca em vez de nao fazer nada.
  vlog(FILE, "SenhaHistoricoPage", "definindo funcao irParaPagina");
  const irParaPagina = (n: number) => {
    vlog(FILE, "SenhaHistoricoPage.irParaPagina", "verificando se destino e a pagina atual (destino=%d, atual=%d)", n, page);
    if (n === page) load();
    else setPage(n);
  };

  vlog(FILE, "SenhaHistoricoPage", "definindo handler handleRefresh");
  const handleRefresh = () => {
    vlog(FILE, "SenhaHistoricoPage.handleRefresh", "recarregando historico");
    load();
  };

  // Busca continua client-side (aplicada sobre os itens da pagina atual);
  // a ordenacao e feita pela API para todas as colunas (ver ORDER_BY_MAP).
  vlog(FILE, "SenhaHistoricoPage", "memorizando lista filtrada client-side");
  const filtered = useMemo(() => {
    vlog(FILE, "SenhaHistoricoPage.filtered", "normalizando termo de busca (tamanho=%d)", search.trim().length);
    const term = search.trim().toLowerCase();
    vlog(FILE, "SenhaHistoricoPage.filtered", "iniciando lista com itens da pagina (qtd=%d)", items.length);
    let list = items;
    vlog(FILE, "SenhaHistoricoPage.filtered", "verificando se ha termo de busca");
    if (term) {
      vlog(FILE, "SenhaHistoricoPage.filtered", "filtrando itens pelo termo");
      list = list.filter((it) => {
        const usuario = (it.usuario_nome || "").toLowerCase();
        const resetador = (it.resetado_por_nome || "").toLowerCase();
        const ip = (it.ip_origem || "").toLowerCase();
        return (
          usuario.includes(term) ||
          resetador.includes(term) ||
          ip.includes(term) ||
          String(it.usuario_id).includes(term)
        );
      });
      vlog(FILE, "SenhaHistoricoPage.filtered", "filtro aplicado (qtd=%d)", list.length);
    }
    return list;
  }, [items, search]);

  vlog(FILE, "SenhaHistoricoPage", "montando definicao das colunas da tabela");
  const columns: Column<SenhaHistoricoItem>[] = [
    {
      key: "id",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      render: (it) => <span className="font-mono text-xs">#{it.id}</span>,
    },
    {
      key: "created_at",
      header: "Data/Hora",
      sortable: true,
      render: (it) => (
        <span className="font-mono text-xs text-slate-700">
          {formatarDataHora(it.created_at)}
        </span>
      ),
    },
    {
      key: "usuario_nome",
      header: "Usuario",
      sortable: true,
      render: (it) => (
        <div className="flex flex-col">
          <span className="font-medium text-slate-900">
            #{it.usuario_id} - {it.usuario_nome || "(sem nome)"}
          </span>
        </div>
      ),
    },
    {
      key: "tipo_reset",
      header: "Tipo Reset",
      width: "160px",
      align: "center",
      sortable: true,
      render: (it) => (
        <Badge color={tipoBadgeColor(it.tipo_reset)}>
          {tipoLabel(it.tipo_reset)}
        </Badge>
      ),
    },
    {
      key: "resetado_por_nome",
      header: "Resetado Por",
      sortable: true,
      render: (it) =>
        it.resetado_por_nome ? (
          <span className="text-slate-700">
            #{it.resetado_por_id} - {it.resetado_por_nome}
          </span>
        ) : (
          <span className="text-slate-400">-</span>
        ),
    },
    {
      key: "ip_origem",
      header: "IP Origem",
      width: "160px",
      sortable: true,
      render: (it) =>
        it.ip_origem ? (
          <span className="font-mono text-xs text-slate-600">
            {it.ip_origem}
          </span>
        ) : (
          <span className="text-slate-400">-</span>
        ),
    },
  ];

  // FE-10: contador baseado na pagina exibida; oculto se nada foi carregado
  // (a primeira carga falhou), para nao afirmar "0 registros".
  vlog(FILE, "SenhaHistoricoPage", "calculando faixa exibida (pagina=%d, limite=%d, total=%d)", exibida.pagina, exibida.limite, total);
  const { inicio: startItem, fim: endItem } = faixaExibida(
    exibida.pagina,
    exibida.limite,
    total
  );
  vlog(FILE, "SenhaHistoricoPage", "calculando se oculta o contador (qtd=%d)", items.length);
  const ocultarContador = erroCarga && items.length === 0;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">
            Auditoria de Senha
          </h1>
          <p className="mt-1 text-sm text-slate-500">
            Historico de todas as alteracoes de senha do sistema. Apenas
            administradores tem acesso.
          </p>
        </div>
        <Button
          onClick={handleRefresh}
          variant="secondary"
          title="Recarregar"
        >
          <svg
            className="mr-1.5 inline-block h-4 w-4"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M4 4v6h6M20 20v-6h-6M5 13a8 8 0 0014.36 4M19 11A8 8 0 004.64 7"
            />
          </svg>
          Atualizar
        </Button>
      </div>

      {error && (
        <div className="mb-4">
          <Alert variant="error" onClose={() => setError(null)}>
            {error}
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
                  placeholder="Nome, IP ou ID..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  icon={
                    <svg
                      className="h-5 w-5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      aria-hidden="true"
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
                  label="Tipo de Reset"
                  options={TIPO_OPTIONS}
                  value={tipo}
                  onChange={(e) =>
                    setTipo(e.target.value as "" | TipoReset)
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
            data={filtered}
            keyExtractor={(it) => it.id}
            loading={loading}
            erroCarga={erroCarga}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              search || tipo
                ? "Nenhum registro encontrado para os filtros aplicados."
                : "Nenhuma alteracao de senha registrada."
            }
          />
        </div>

        <Paginador pagina={exibida.pagina} paginas={pages} onIrPara={irParaPagina} />
      </Card>

      <div className="mt-4 text-xs text-slate-400">
        <strong>Legenda dos tipos:</strong>{" "}
        <Badge color="blue">Proprio</Badge> alteracao feita pelo proprio
        usuario,{" "}
        <Badge color="yellow">Admin</Badge> reset feito por um administrador,{" "}
        <Badge color="green">Primeiro Acesso</Badge> alteracao obrigatoria no
        primeiro acesso,{" "}
        <Badge color="red">Esquecimento</Badge> reset via fluxo de senha
        esquecida.
      </div>
    </div>
  );
}
