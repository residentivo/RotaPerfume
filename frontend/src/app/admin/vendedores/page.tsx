"use client";

import { useEffect, useMemo, useState } from "react";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Select } from "@/components/ui/Select";
import { Table, Column } from "@/components/ui/Table";
import { Paginador } from "@/components/ui/Paginador";
import { useMensagemTemporaria } from "@/lib/useMensagemTemporaria";
import { VendedorModal } from "@/components/admin/VendedorModal";
import {
  apiListVendedores,
  apiCreateVendedor,
  apiUpdateVendedor,
  apiDeleteVendedor,
  apiReativarVendedor,
} from "@/lib/api";
import { Vendedor, VendedorCompleto, VendedorInput } from "@/lib/types";
import { faixaExibida, mensagemRecargaFalhou } from "@/lib/useListaSegura";
import { vlog } from "@/lib/vlog";

const FILE = "admin/vendedores/page.tsx";

// GET /api/vendedores nao aceita paginacao/filtros/ordenacao no backend
// (endpoint simples, historicamente usado so para popular combobox) — ver
// apis/rotaperfumes-api/handlers/vendedor_handler.go (ListVendedores). Por
// isso a busca, ordenacao e paginacao desta tela sao aplicadas no cliente,
// sobre a lista completa de vendedores (ativos e inativos) retornada pela
// API.
type SortKey = "id" | "nome" | "regiao" | "uf" | "status";
type SortDir = "asc" | "desc";

const LIMIT_OPTIONS = [
  { value: "10", label: "10 por pagina" },
  { value: "20", label: "20 por pagina" },
  { value: "50", label: "50 por pagina" },
  { value: "100", label: "100 por pagina" },
];

type StatusFilter = "todos" | "ativo" | "inativo";

const STATUS_OPTIONS: { value: string; label: string }[] = [
  { value: "todos", label: "Todos" },
  { value: "ativo", label: "Ativo" },
  { value: "inativo", label: "Inativo" },
];

const TODAS_OPTION = { value: "", label: "Todas" };

function VendedoresPageContent() {
  vlog(FILE, "VendedoresPageContent", "inicializando estado da lista de vendedores");
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  vlog(FILE, "VendedoresPageContent", "inicializando estado de loading");
  const [loading, setLoading] = useState(true);
  vlog(FILE, "VendedoresPageContent", "inicializando estado de erro");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "VendedoresPageContent", "inicializando estado de erro de carga");
  const [erroCarga, setErroCarga] = useState(false);
  vlog(FILE, "VendedoresPageContent", "obtendo controle de mensagem temporaria de sucesso");
  const {
    mensagem: success,
    mostrar: mostrarSucesso,
    limpar: limparSucesso,
  } = useMensagemTemporaria();
  vlog(FILE, "VendedoresPageContent", "inicializando estado da busca textual");
  const [search, setSearch] = useState("");
  vlog(FILE, "VendedoresPageContent", "inicializando estado do filtro de regiao");
  const [regiaoFilter, setRegiaoFilter] = useState("");
  vlog(FILE, "VendedoresPageContent", "inicializando estado do filtro de UF");
  const [ufFilter, setUfFilter] = useState("");
  vlog(FILE, "VendedoresPageContent", "inicializando estado do filtro de status");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("todos");
  vlog(FILE, "VendedoresPageContent", "inicializando estado da coluna de ordenacao");
  const [sortKey, setSortKey] = useState<SortKey>("nome");
  vlog(FILE, "VendedoresPageContent", "inicializando estado da direcao de ordenacao");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  vlog(FILE, "VendedoresPageContent", "inicializando estado do id em alteracao de status");
  const [togglingId, setTogglingId] = useState<number | null>(null);

  vlog(FILE, "VendedoresPageContent", "inicializando estado da pagina atual");
  const [page, setPage] = useState(1);
  vlog(FILE, "VendedoresPageContent", "inicializando estado do limite por pagina");
  const [limit, setLimit] = useState(20);

  // Modal state
  vlog(FILE, "VendedoresPageContent", "inicializando estado de abertura do modal");
  const [modalOpen, setModalOpen] = useState(false);
  vlog(FILE, "VendedoresPageContent", "inicializando estado do modo do modal");
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  vlog(FILE, "VendedoresPageContent", "inicializando estado do vendedor em edicao");
  const [editingVendedor, setEditingVendedor] = useState<VendedorCompleto | null>(
    null
  );

  vlog(FILE, "VendedoresPageContent", "definindo funcao aplicarVendedores");
  const aplicarVendedores = (res: Vendedor[]) => {
    vlog(FILE, "VendedoresPageContent.aplicarVendedores", "aplicando lista de vendedores (qtd=%d)", res.length);
    setVendedores(res);
    vlog(FILE, "VendedoresPageContent.aplicarVendedores", "limpando erro de carga");
    setErroCarga(false);
    vlog(FILE, "VendedoresPageContent.aplicarVendedores", "desativando loading");
    setLoading(false);
  };

  // FE-09: erro de carga mantem a ultima lista carregada (nao zera) e marca
  // erroCarga para a tabela nao exibir o estado vazio junto do alerta.
  vlog(FILE, "VendedoresPageContent", "definindo funcao aplicarErroVendedores");
  const aplicarErroVendedores = (err: unknown): string => {
    vlog(FILE, "VendedoresPageContent.aplicarErroVendedores", "extraindo mensagem do erro de carga");
    const message =
      err instanceof Error
        ? err.message
        : "Erro ao carregar vendedores. O endpoint /api/vendedores pode nao existir no backend.";
    vlog(FILE, "VendedoresPageContent.aplicarErroVendedores", "exibindo mensagem de erro");
    setError(message);
    vlog(FILE, "VendedoresPageContent.aplicarErroVendedores", "marcando erro de carga");
    setErroCarga(true);
    vlog(FILE, "VendedoresPageContent.aplicarErroVendedores", "desativando loading");
    setLoading(false);
    return message;
  };

  // Recarga imperativa (apos criar/editar/ativar). FE-10: devolve a
  // mensagem de erro se a recarga falhar (null se deu certo).
  vlog(FILE, "VendedoresPageContent", "definindo funcao loadVendedores");
  const loadVendedores = async (): Promise<string | null> => {
    vlog(FILE, "VendedoresPageContent.loadVendedores", "ativando loading");
    setLoading(true);
    vlog(FILE, "VendedoresPageContent.loadVendedores", "limpando erro");
    setError(null);
    vlog(FILE, "VendedoresPageContent.loadVendedores", "chamando apiListVendedores");
    return apiListVendedores().then(
      (res) => {
        vlog(FILE, "VendedoresPageContent.loadVendedores.func", "recarga concluida; aplicando lista");
        aplicarVendedores(res);
        return null;
      },
      (err: unknown) => aplicarErroVendedores(err)
    );
  };

  // FE-10: a acao deu certo; se a recarga que vem depois falhar, mostra um
  // unico alerta de erro dizendo as duas coisas (em vez de sucesso + erro
  // juntos, contraditorios com a linha desatualizada na tabela).
  vlog(FILE, "VendedoresPageContent", "definindo funcao concluirAcao");
  const concluirAcao = async (msgSucesso: string) => {
    vlog(FILE, "VendedoresPageContent.concluirAcao", "recarregando vendedores apos a acao");
    const erroRecarga = await loadVendedores();
    vlog(FILE, "VendedoresPageContent.concluirAcao", "verificando falha na recarga (falhou=%s)", erroRecarga !== null);
    if (erroRecarga) setError(mensagemRecargaFalhou(msgSucesso, erroRecarga));
    else mostrarSucesso(msgSucesso);
  };

  // Carga inicial: o loading ja comeca true; o efeito so faz a busca e
  // aplica o resultado no callback assincrono.
  vlog(FILE, "VendedoresPageContent", "registrando efeito de carga inicial");
  useEffect(() => {
    vlog(FILE, "VendedoresPageContent.useEffect", "chamando apiListVendedores");
    apiListVendedores().then(aplicarVendedores, aplicarErroVendedores);
  }, []);

  // Reseta para pagina 1 quando busca/filtros/ordenacao mudam — ajustado
  // durante o render (padrao "ajustar estado quando a entrada muda").
  vlog(FILE, "VendedoresPageContent", "montando chave dos filtros/ordenacao");
  const chaveFiltros = `${search}|${regiaoFilter}|${ufFilter}|${statusFilter}|${sortKey}|${sortDir}`;
  vlog(FILE, "VendedoresPageContent", "inicializando estado dos filtros anteriores");
  const [filtrosAnteriores, setFiltrosAnteriores] = useState(chaveFiltros);
  vlog(FILE, "VendedoresPageContent", "verificando se os filtros mudaram");
  if (filtrosAnteriores !== chaveFiltros) {
    vlog(FILE, "VendedoresPageContent", "atualizando filtros anteriores");
    setFiltrosAnteriores(chaveFiltros);
    vlog(FILE, "VendedoresPageContent", "voltando para a pagina 1");
    setPage(1);
  }

  vlog(FILE, "VendedoresPageContent", "memorizando opcoes do filtro de regiao");
  const regiaoOptions = useMemo(() => {
    vlog(FILE, "VendedoresPageContent.regiaoOptions", "extraindo regioes unicas ordenadas (vendedores=%d)", vendedores.length);
    const values = Array.from(new Set(vendedores.map((v) => v.regiao))).sort(
      (a, b) => a.localeCompare(b, "pt-BR")
    );
    return [TODAS_OPTION, ...values.map((v) => ({ value: v, label: v }))];
  }, [vendedores]);

  vlog(FILE, "VendedoresPageContent", "memorizando opcoes do filtro de UF");
  const ufOptions = useMemo(() => {
    vlog(FILE, "VendedoresPageContent.ufOptions", "extraindo UFs unicas ordenadas (vendedores=%d)", vendedores.length);
    const values = Array.from(new Set(vendedores.map((v) => v.uf))).sort(
      (a, b) => a.localeCompare(b, "pt-BR")
    );
    return [TODAS_OPTION, ...values.map((v) => ({ value: v, label: v }))];
  }, [vendedores]);

  vlog(FILE, "VendedoresPageContent", "definindo handler handleSort");
  const handleSort = (key: SortKey) => {
    vlog(FILE, "VendedoresPageContent.handleSort", "verificando se coluna ja e a ordenada (key=%s)", key);
    if (sortKey === key) {
      vlog(FILE, "VendedoresPageContent.handleSort", "invertendo direcao da ordenacao");
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      vlog(FILE, "VendedoresPageContent.handleSort", "definindo nova coluna de ordenacao");
      setSortKey(key);
      vlog(FILE, "VendedoresPageContent.handleSort", "definindo direcao asc");
      setSortDir("asc");
    }
  };

  vlog(FILE, "VendedoresPageContent", "memorizando lista filtrada e ordenada client-side");
  const filteredSorted = useMemo(() => {
    vlog(FILE, "VendedoresPageContent.filteredSorted", "normalizando termo de busca (tamanho=%d)", search.trim().length);
    const term = search.trim().toLowerCase();
    vlog(FILE, "VendedoresPageContent.filteredSorted", "iniciando lista com todos os vendedores (qtd=%d)", vendedores.length);
    let list = vendedores;
    vlog(FILE, "VendedoresPageContent.filteredSorted", "verificando se ha termo de busca");
    if (term) {
      vlog(FILE, "VendedoresPageContent.filteredSorted", "filtrando por id, nome, regiao, UF ou status");
      list = list.filter(
        (v) =>
          String(v.id).includes(term) ||
          v.nome.toLowerCase().includes(term) ||
          v.regiao.toLowerCase().includes(term) ||
          v.uf.toLowerCase().includes(term) ||
          (v.data_desligamento
            ? "inativo".includes(term)
            : "ativo".includes(term))
      );
    }
    vlog(FILE, "VendedoresPageContent.filteredSorted", "verificando filtro de regiao");
    if (regiaoFilter) {
      vlog(FILE, "VendedoresPageContent.filteredSorted", "filtrando por regiao");
      list = list.filter((v) => v.regiao === regiaoFilter);
    }
    vlog(FILE, "VendedoresPageContent.filteredSorted", "verificando filtro de UF");
    if (ufFilter) {
      vlog(FILE, "VendedoresPageContent.filteredSorted", "filtrando por UF");
      list = list.filter((v) => v.uf === ufFilter);
    }
    vlog(FILE, "VendedoresPageContent.filteredSorted", "verificando filtro de status (status=%s)", statusFilter);
    if (statusFilter !== "todos") {
      vlog(FILE, "VendedoresPageContent.filteredSorted", "filtrando por status");
      list = list.filter((v) =>
        statusFilter === "ativo" ? !v.data_desligamento : !!v.data_desligamento
      );
    }
    vlog(FILE, "VendedoresPageContent.filteredSorted", "ordenando lista (qtd=%d, sortKey=%s, sortDir=%s)", list.length, sortKey, sortDir);
    const sorted = [...list].sort((a, b) => {
      if (sortKey === "status") {
        const av = a.data_desligamento ? 0 : 1;
        const bv = b.data_desligamento ? 0 : 1;
        return sortDir === "asc" ? av - bv : bv - av;
      }
      const av = a[sortKey];
      const bv = b[sortKey];
      if (typeof av === "number" && typeof bv === "number") {
        return sortDir === "asc" ? av - bv : bv - av;
      }
      const cmp = String(av).localeCompare(String(bv), "pt-BR", {
        numeric: true,
      });
      return sortDir === "asc" ? cmp : -cmp;
    });
    return sorted;
  }, [
    vendedores,
    search,
    regiaoFilter,
    ufFilter,
    statusFilter,
    sortKey,
    sortDir,
  ]);

  vlog(FILE, "VendedoresPageContent", "calculando total filtrado (total=%d)", filteredSorted.length);
  const total = filteredSorted.length;
  vlog(FILE, "VendedoresPageContent", "calculando total de paginas (limit=%d)", limit);
  const pages = Math.max(1, Math.ceil(total / limit));
  vlog(FILE, "VendedoresPageContent", "limitando pagina atual ao total (page=%d, pages=%d)", page, pages);
  const pageSafe = Math.min(page, pages);
  vlog(FILE, "VendedoresPageContent", "memorizando fatia paginada (pageSafe=%d)", pageSafe);
  const paginated = useMemo(
    () => filteredSorted.slice((pageSafe - 1) * limit, pageSafe * limit),
    [filteredSorted, pageSafe, limit]
  );

  vlog(FILE, "VendedoresPageContent", "definindo handler openCreate");
  const openCreate = () => {
    vlog(FILE, "VendedoresPageContent.openCreate", "definindo modo create");
    setModalMode("create");
    vlog(FILE, "VendedoresPageContent.openCreate", "limpando vendedor em edicao");
    setEditingVendedor(null);
    vlog(FILE, "VendedoresPageContent.openCreate", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "VendedoresPageContent", "definindo handler openEdit");
  const openEdit = (vendedor: Vendedor) => {
    vlog(FILE, "VendedoresPageContent.openEdit", "definindo modo edit (vendedor_id=%d)", vendedor.id);
    setModalMode("edit");
    // O modal busca o detalhe completo (GET /api/vendedores/{id}) e preenche
    // o formulario inteiro com ele (BUG-07). data_admissao/meta_mensal abaixo
    // sao apenas provisorios (a listagem nao traz esses campos, SEC-03) e o
    // modal os ignora; o salvar fica bloqueado ate o detalhe carregar.
    vlog(FILE, "VendedoresPageContent.openEdit", "definindo vendedor provisorio em edicao");
    setEditingVendedor({
      id: vendedor.id,
      nome: vendedor.nome,
      regiao: vendedor.regiao,
      uf: vendedor.uf,
      data_admissao: "",
      data_desligamento: vendedor.data_desligamento,
      meta_mensal: 0,
      created_at: "",
      updated_at: "",
    });
    vlog(FILE, "VendedoresPageContent.openEdit", "abrindo modal");
    setModalOpen(true);
  };

  vlog(FILE, "VendedoresPageContent", "definindo handler handleModalSubmit");
  const handleModalSubmit = async (data: VendedorInput) => {
    vlog(FILE, "VendedoresPageContent.handleModalSubmit", "limpando erro");
    setError(null);
    vlog(FILE, "VendedoresPageContent.handleModalSubmit", "verificando modo do modal (modo=%s)", modalMode);
    if (modalMode === "create") {
      vlog(FILE, "VendedoresPageContent.handleModalSubmit", "chamando apiCreateVendedor");
      const created = await apiCreateVendedor(data);
      vlog(FILE, "VendedoresPageContent.handleModalSubmit", "concluindo criacao (vendedor_id=%d)", created.id);
      await concluirAcao(`Vendedor "${created.nome}" criado com sucesso.`);
    } else if (editingVendedor) {
      vlog(FILE, "VendedoresPageContent.handleModalSubmit", "chamando apiUpdateVendedor (vendedor_id=%d)", editingVendedor.id);
      await apiUpdateVendedor(editingVendedor.id, data);
      vlog(FILE, "VendedoresPageContent.handleModalSubmit", "concluindo atualizacao");
      await concluirAcao(`Vendedor "${data.nome}" atualizado com sucesso.`);
    }
    vlog(FILE, "VendedoresPageContent.handleModalSubmit", "fechando modal");
    setModalOpen(false);
  };

  vlog(FILE, "VendedoresPageContent", "definindo handler handleDelete");
  const handleDelete = async (vendedor: Vendedor) => {
    vlog(FILE, "VendedoresPageContent.handleDelete", "pedindo confirmacao de inativacao (vendedor_id=%d)", vendedor.id);
    const ok = window.confirm(
      `Tem certeza que deseja inativar o vendedor "${vendedor.nome}"? O historico de pedidos/carteiras sera preservado.`
    );
    vlog(FILE, "VendedoresPageContent.handleDelete", "verificando confirmacao (ok=%s)", ok);
    if (!ok) return;

    vlog(FILE, "VendedoresPageContent.handleDelete", "marcando id em alteracao");
    setTogglingId(vendedor.id);
    vlog(FILE, "VendedoresPageContent.handleDelete", "limpando erro");
    setError(null);
    vlog(FILE, "VendedoresPageContent.handleDelete", "limpando mensagem de sucesso");
    limparSucesso();
    vlog(FILE, "VendedoresPageContent.handleDelete", "iniciando chamada de inativacao");
    try {
      vlog(FILE, "VendedoresPageContent.handleDelete", "chamando apiDeleteVendedor");
      await apiDeleteVendedor(vendedor.id);
      vlog(FILE, "VendedoresPageContent.handleDelete", "concluindo inativacao");
      await concluirAcao(`Vendedor "${vendedor.nome}" inativado com sucesso.`);
    } catch (err) {
      vlog(FILE, "VendedoresPageContent.handleDelete", "falha na inativacao; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao inativar vendedor.";
      vlog(FILE, "VendedoresPageContent.handleDelete", "exibindo mensagem de erro");
      setError(message);
    } finally {
      vlog(FILE, "VendedoresPageContent.handleDelete", "limpando id em alteracao");
      setTogglingId(null);
    }
  };

  vlog(FILE, "VendedoresPageContent", "definindo handler handleReativar");
  const handleReativar = async (vendedor: Vendedor) => {
    vlog(FILE, "VendedoresPageContent.handleReativar", "pedindo confirmacao de reativacao (vendedor_id=%d)", vendedor.id);
    const ok = window.confirm(
      `Tem certeza que deseja reativar o vendedor "${vendedor.nome}"?`
    );
    vlog(FILE, "VendedoresPageContent.handleReativar", "verificando confirmacao (ok=%s)", ok);
    if (!ok) return;

    vlog(FILE, "VendedoresPageContent.handleReativar", "marcando id em alteracao");
    setTogglingId(vendedor.id);
    vlog(FILE, "VendedoresPageContent.handleReativar", "limpando erro");
    setError(null);
    vlog(FILE, "VendedoresPageContent.handleReativar", "limpando mensagem de sucesso");
    limparSucesso();
    vlog(FILE, "VendedoresPageContent.handleReativar", "iniciando chamada de reativacao");
    try {
      vlog(FILE, "VendedoresPageContent.handleReativar", "chamando apiReativarVendedor");
      await apiReativarVendedor(vendedor.id);
      vlog(FILE, "VendedoresPageContent.handleReativar", "concluindo reativacao");
      await concluirAcao(`Vendedor "${vendedor.nome}" reativado com sucesso.`);
    } catch (err) {
      vlog(FILE, "VendedoresPageContent.handleReativar", "falha na reativacao; extraindo mensagem do erro");
      const message =
        err instanceof Error ? err.message : "Erro ao reativar vendedor.";
      vlog(FILE, "VendedoresPageContent.handleReativar", "exibindo mensagem de erro");
      setError(message);
    } finally {
      vlog(FILE, "VendedoresPageContent.handleReativar", "limpando id em alteracao");
      setTogglingId(null);
    }
  };

  vlog(FILE, "VendedoresPageContent", "definindo handler handleToggleStatus");
  const handleToggleStatus = (vendedor: Vendedor) => {
    vlog(FILE, "VendedoresPageContent.handleToggleStatus", "verificando se vendedor esta desligado (vendedor_id=%d)", vendedor.id);
    if (vendedor.data_desligamento) {
      vlog(FILE, "VendedoresPageContent.handleToggleStatus", "vendedor desligado; reativando");
      handleReativar(vendedor);
    } else {
      vlog(FILE, "VendedoresPageContent.handleToggleStatus", "vendedor ativo; inativando");
      handleDelete(vendedor);
    }
  };

  vlog(FILE, "VendedoresPageContent", "montando definicao das colunas da tabela");
  const columns: Column<Vendedor>[] = [
    {
      key: "id",
      header: "ID",
      width: "80px",
      align: "left",
      sortable: true,
      render: (v) => <span className="font-mono text-xs">#{v.id}</span>,
    },
    {
      key: "nome",
      header: "Vendedor",
      sortable: true,
      render: (v) => (
        <button
          type="button"
          onClick={() => openEdit(v)}
          className="font-medium text-slate-900 hover:text-primary-600 hover:underline text-left"
          title="Editar vendedor"
        >
          {v.nome}
        </button>
      ),
    },
    {
      key: "regiao",
      header: "Regiao",
      width: "200px",
      sortable: true,
      render: (v) => <span className="text-slate-600">{v.regiao}</span>,
    },
    {
      key: "uf",
      header: "UF",
      width: "100px",
      align: "center",
      sortable: true,
      render: (v) => <span className="text-slate-600">{v.uf}</span>,
    },
    {
      key: "status",
      header: "Status",
      width: "140px",
      align: "center",
      sortable: true,
      render: (v) => (
        <button
          type="button"
          onClick={() => handleToggleStatus(v)}
          disabled={togglingId === v.id}
          title={
            v.data_desligamento
              ? "Clique para reativar"
              : "Clique para inativar"
          }
          className={[
            "inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium transition-colors",
            "disabled:cursor-not-allowed disabled:opacity-60",
            v.data_desligamento
              ? "bg-red-100 text-red-700 hover:bg-red-200"
              : "bg-green-100 text-green-700 hover:bg-green-200",
          ].join(" ")}
        >
          <span
            className={[
              "h-2 w-2 rounded-full",
              v.data_desligamento ? "bg-red-500" : "bg-green-500",
            ].join(" ")}
          />
          {v.data_desligamento ? "Inativo" : "Ativo"}
        </button>
      ),
    },
    {
      key: "actions",
      header: "Acoes",
      width: "100px",
      align: "right",
      render: (v) => (
        <div className="inline-flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEdit(v)}
            title="Editar vendedor"
          >
            Editar
          </Button>
        </div>
      ),
    },
  ];

  // Paginacao local: a pagina exibida (pageSafe) sempre corresponde as
  // linhas. FE-10: contador oculto se nada foi carregado (a primeira carga
  // falhou), para nao afirmar "0 vendedores".
  vlog(FILE, "VendedoresPageContent", "calculando faixa exibida (pagina=%d, limite=%d, total=%d)", pageSafe, limit, total);
  const { inicio: startItem, fim: endItem } = faixaExibida(pageSafe, limit, total);
  vlog(FILE, "VendedoresPageContent", "calculando se oculta o contador (qtd=%d)", vendedores.length);
  const ocultarContador = erroCarga && vendedores.length === 0;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Vendedores</h1>
          <p className="mt-1 text-sm text-slate-500">
            Consulte e gerencie os vendedores cadastrados no CRM.
          </p>
        </div>
        <Button onClick={openCreate}>+ Novo Vendedor</Button>
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
                  placeholder="Nome, regiao ou UF..."
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
              <div className="sm:w-44">
                <Select
                  label="Regiao"
                  options={regiaoOptions}
                  value={regiaoFilter}
                  onChange={(e) => setRegiaoFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-32">
                <Select
                  label="UF"
                  options={ufOptions}
                  value={ufFilter}
                  onChange={(e) => setUfFilter(e.target.value)}
                />
              </div>
              <div className="sm:w-36">
                <Select
                  label="Status"
                  options={STATUS_OPTIONS}
                  value={statusFilter}
                  onChange={(e) =>
                    setStatusFilter(e.target.value as StatusFilter)
                  }
                />
              </div>
              <div className="sm:w-44">
                <Select
                  label="Itens por pagina"
                  options={LIMIT_OPTIONS}
                  value={String(limit)}
                  onChange={(e) => {
                    setLimit(Number(e.target.value));
                    setPage(1);
                  }}
                />
              </div>
            </div>
            {!ocultarContador && (
              <div className="text-sm text-slate-500">
                {total === 0
                  ? "0 vendedores"
                  : `${startItem}-${endItem} de ${total} ${
                      total === 1 ? "vendedor" : "vendedores"
                    }`}
              </div>
            )}
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={paginated}
            keyExtractor={(v) => v.id}
            loading={loading}
            erroCarga={erroCarga}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              search || regiaoFilter || ufFilter || statusFilter !== "todos"
                ? "Nenhum vendedor encontrado para a busca/filtros aplicados."
                : "Nenhum vendedor cadastrado."
            }
          />
        </div>

        <Paginador pagina={pageSafe} paginas={pages} onIrPara={setPage} />
      </Card>

      <div className="mt-4 text-xs text-slate-400">
        <strong>Nota:</strong> A busca, os filtros (regiao/UF/status),
        ordenacao e paginacao desta tela sao aplicados no navegador, pois{" "}
        <code>GET /api/vendedores</code> retorna a lista completa de
        vendedores (ativos e inativos), sem parametros de
        paginacao/filtro/ordenacao no backend. Vendedores inativados
        continuam aparecendo na lista, marcados com{" "}
        <strong>[X] Inativo</strong> na coluna Status — use os filtros de
        Status/Regiao/UF para restringir a visualizacao.
      </div>

      <VendedorModal
        open={modalOpen}
        mode={modalMode}
        vendedor={editingVendedor}
        onClose={() => setModalOpen(false)}
        onSubmit={handleModalSubmit}
      />
    </div>
  );
}

export default function VendedoresPage() {
  return (
    <ProtectedRoute requireAdmin>
      <VendedoresPageContent />
    </ProtectedRoute>
  );
}
