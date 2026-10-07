"use client";

import { useEffect, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import {
  PedidoDetalhe,
  PedidoInput,
  ItemPedidoInput,
  ClienteResumo,
  Vendedor,
  Produto,
} from "@/lib/types";
import { useAjustarAoMudar, useResetOnOpen } from "@/lib/useResetOnOpen";
import {
  apiListVendedores,
  apiListProdutos,
  apiListClientesDoVendedor,
} from "@/lib/api";
import { refreshSessionUser, useSessionUser } from "@/lib/session";
import { vlog } from "@/lib/vlog";

const F = "PedidoModal.tsx";

// Traduz mensagens de erro do backend para textos mais amigaveis. Hoje trata
// o 400 "cliente nao pertence a carteira deste vendedor" (escopo por carteira
// aplicado em POST/PUT /api/pedidos para usuarios nao-admin).
function friendlyPedidoError(message: string): string {
  vlog(F, "friendlyPedidoError", "normalizando mensagem de erro do backend");
  const normalized = message
    .toLowerCase()
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "");
  vlog(F, "friendlyPedidoError", "verificando se é erro de cliente fora da carteira");
  if (normalized.includes("nao pertence") && normalized.includes("carteira")) {
    return "O cliente selecionado nao pertence a carteira ativa do vendedor. Escolha um cliente da carteira e tente novamente.";
  }
  return message;
}

interface PedidoModalProps {
  open: boolean;
  mode: "create" | "edit";
  pedido: PedidoDetalhe | null;
  onClose: () => void;
  onSubmit: (data: PedidoInput) => Promise<void>;
}

const CANAL_OPTIONS = [
  { value: "App", label: "App" },
  { value: "Telefone", label: "Telefone" },
  { value: "Visita", label: "Visita" },
  { value: "WhatsApp", label: "WhatsApp" },
];

const STATUS_OPTIONS = [
  { value: "Em separação", label: "Em separação" },
  { value: "Faturado", label: "Faturado" },
  { value: "Entregue", label: "Entregue" },
  { value: "Cancelado", label: "Cancelado" },
];

// Linha de item do formulário: mesma forma de ItemPedidoInput, mas com um id
// local (para key do React) e os campos numéricos como string (facilita
// edição livre no input antes da validação final no submit).
interface ItemFormRow {
  localId: string;
  produto_id: string;
  quantidade: string;
  preco_praticado: string;
  desconto_pct: string;
}

function emptyItemRow(): ItemFormRow {
  return {
    localId: Math.random().toString(36).slice(2, 10),
    produto_id: "",
    quantidade: "1",
    preco_praticado: "",
    desconto_pct: "0",
  };
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

const currencyFmt = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
});

function calcValorBruto(row: ItemFormRow): number {
  vlog(F, "calcValorBruto", "convertendo quantidade");
  const q = Number(row.quantidade);
  vlog(F, "calcValorBruto", "convertendo preço praticado");
  const p = Number(row.preco_praticado);
  vlog(F, "calcValorBruto", "convertendo desconto");
  const d = Number(row.desconto_pct);
  vlog(F, "calcValorBruto", "verificando valores numéricos válidos");
  if (Number.isNaN(q) || Number.isNaN(p) || Number.isNaN(d)) return 0;
  return q * p * (1 - d / 100);
}

export function PedidoModal({
  open,
  mode,
  pedido,
  onClose,
  onSubmit,
}: PedidoModalProps) {
  vlog(F, "PedidoModal", "criando estado clienteId, modo:", mode);
  const [clienteId, setClienteId] = useState("");
  vlog(F, "PedidoModal", "criando estado vendedorId");
  const [vendedorId, setVendedorId] = useState("");
  vlog(F, "PedidoModal", "criando estado dataPedido");
  const [dataPedido, setDataPedido] = useState("");
  vlog(F, "PedidoModal", "criando estado canal");
  const [canal, setCanal] = useState("App");
  vlog(F, "PedidoModal", "criando estado status");
  const [status, setStatus] = useState("Em separação");
  vlog(F, "PedidoModal", "criando estado itens");
  const [itens, setItens] = useState<ItemFormRow[]>([emptyItemRow()]);
  vlog(F, "PedidoModal", "criando estado submitting");
  const [submitting, setSubmitting] = useState(false);
  vlog(F, "PedidoModal", "criando estado error");
  const [error, setError] = useState<string | null>(null);

  // Listas auxiliares para os selects (mesmo padrão do UserModal com
  // apiListVendedores).
  vlog(F, "PedidoModal", "criando estado vendedores");
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  vlog(F, "PedidoModal", "criando estado produtos");
  const [produtos, setProdutos] = useState<Produto[]>([]);
  vlog(F, "PedidoModal", "criando estado loadingAux");
  const [loadingAux, setLoadingAux] = useState(false);
  vlog(F, "PedidoModal", "criando estado auxError");
  const [auxError, setAuxError] = useState<string | null>(null);

  // Clientes em cascata pelo vendedor selecionado (carteira ativa via
  // GET /api/vendedores/{id}/clientes), mesmo padrao de Visita/Oportunidade.
  vlog(F, "PedidoModal", "criando estado clientes");
  const [clientes, setClientes] = useState<ClienteResumo[]>([]);
  vlog(F, "PedidoModal", "criando estado loadingClientes");
  const [loadingClientes, setLoadingClientes] = useState(false);
  vlog(F, "PedidoModal", "criando estado clientesError");
  const [clientesError, setClientesError] = useState<string | null>(null);

  // Usuario logado: sessao em memoria validada por GET /api/auth/me (fonte da
  // verdade; revalidada ao montar o ProtectedRoute, ao voltar o foco da aba e
  // ao abrir este modal). Usuario nao-admin tem o vendedor travado no proprio
  // id_vendedor — o backend tambem forca isso em POST/PUT /api/pedidos.
  vlog(F, "PedidoModal", "lendo usuário da sessão");
  const sessionUser = useSessionUser();
  vlog(F, "PedidoModal", "verificando se é admin");
  const isAdmin = sessionUser?.role === "admin";
  vlog(F, "PedidoModal", "calculando vendedor próprio do usuário, admin:", isAdmin);
  const ownVendedorId =
    !isAdmin && sessionUser?.id_vendedor ? sessionUser.id_vendedor : null;
  vlog(F, "PedidoModal", "calculando nome do vendedor próprio");
  const ownVendedorNome = !isAdmin ? sessionUser?.vendedor_nome ?? null : null;
  vlog(F, "PedidoModal", "calculando se usuário está sem carteira");
  const semCarteira = !isAdmin && !ownVendedorId;

  // Ao abrir, rebusca /me para pegar um vinculo alterado pelo admin.
  vlog(F, "PedidoModal", "registrando efeito de revalidação da sessão ao abrir");
  useEffect(() => {
    vlog(F, "PedidoModal.useEffect", "verificando se o modal está aberto (sessão):", open);
    if (!open) return;
    vlog(F, "PedidoModal.useEffect", "revalidando sessão (/me)");
    refreshSessionUser().catch(() => {
      // Falha aqui nao bloqueia o formulario; o backend valida o vendedor.
      vlog(F, "PedidoModal.useEffect", "falha ao revalidar sessão (não bloqueia o formulário)");
    });
  }, [open]);

  // Mantem o vendedor travado sincronizado com a sessao (usuario normal).
  // Ajustado durante o render quando open/isAdmin/ownVendedorId mudam.
  vlog(F, "PedidoModal", "registrando sincronização do vendedor travado");
  useAjustarAoMudar([open, isAdmin, ownVendedorId], () => {
    vlog(F, "PedidoModal.ajustarVendedor", "verificando se modal aberto e usuário não admin:", open, !isAdmin);
    if (open && !isAdmin) {
      vlog(F, "PedidoModal.ajustarVendedor", "travando vendedor da sessão, id:", ownVendedorId);
      setVendedorId(ownVendedorId ? String(ownVendedorId) : "");
    }
  });

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  vlog(F, "PedidoModal", "registrando reset do formulário ao abrir");
  useResetOnOpen(open, [mode, pedido], () => {
    vlog(F, "PedidoModal.reset", "limpando erro");
    setError(null);
    vlog(F, "PedidoModal.reset", "limpando estado de envio");
    setSubmitting(false);
    // Sessao em memoria (/me) lida de forma reativa: o reset roda no render.
    vlog(F, "PedidoModal.reset", "lendo usuário da sessão");
    const user = sessionUser;
    vlog(F, "PedidoModal.reset", "verificando se é admin");
    const admin = user?.role === "admin";
    vlog(F, "PedidoModal.reset", "calculando vendedor próprio");
    const ownId = !admin && user?.id_vendedor ? user.id_vendedor : null;
    vlog(F, "PedidoModal.reset", "calculando vendedor travado, id:", ownId);
    const lockedVendedor = !admin ? (ownId ? String(ownId) : "") : null;
    vlog(F, "PedidoModal.reset", "verificando se é edição com pedido, id:", pedido?.pedido_id_origem);
    if (mode === "edit" && pedido) {
      vlog(F, "PedidoModal.reset", "preenchendo cliente, id:", pedido.cliente_id);
      setClienteId(String(pedido.cliente_id));
      vlog(F, "PedidoModal.reset", "preenchendo vendedor, id:", pedido.vendedor_id);
      setVendedorId(lockedVendedor ?? String(pedido.vendedor_id));
      vlog(F, "PedidoModal.reset", "preenchendo data do pedido");
      setDataPedido(pedido.data_pedido ? pedido.data_pedido.slice(0, 10) : "");
      vlog(F, "PedidoModal.reset", "preenchendo canal");
      setCanal(pedido.canal);
      vlog(F, "PedidoModal.reset", "preenchendo status");
      setStatus(pedido.status);
      vlog(F, "PedidoModal.reset", "preenchendo itens, qtd:", pedido.itens.length);
      setItens(
        pedido.itens.length > 0
          ? pedido.itens.map((it) => ({
              localId: String(it.item_id_origem),
              produto_id: String(it.produto_id),
              quantidade: String(it.quantidade),
              preco_praticado: String(it.preco_praticado),
              desconto_pct: String(it.desconto_pct),
            }))
          : [emptyItemRow()]
      );
    } else {
      vlog(F, "PedidoModal.reset", "limpando cliente");
      setClienteId("");
      vlog(F, "PedidoModal.reset", "definindo vendedor inicial");
      setVendedorId(lockedVendedor ?? "");
      vlog(F, "PedidoModal.reset", "definindo data do pedido como hoje");
      setDataPedido(todayISO());
      vlog(F, "PedidoModal.reset", "definindo canal padrão");
      setCanal("App");
      vlog(F, "PedidoModal.reset", "definindo status padrão");
      setStatus("Em separação");
      vlog(F, "PedidoModal.reset", "definindo um item vazio");
      setItens([emptyItemRow()]);
    }
  });

  // Carrega clientes/vendedores/produtos para popular os selects. Reutiliza
  // os endpoints ja existentes GET /api/clientes, /api/vendedores e
  // /api/produtos (apenas ativos, limite alto para cobrir a maioria dos casos).
  //
  // O backend limita `limit` a no maximo 100 itens por pagina (ver
  // services.ParsePagination), entao para o catalogo de produtos (que pode
  // ter varias centenas de itens) e preciso paginar ate esgotar todas as
  // paginas, em vez de confiar em um unico limit alto.
  // Parte sincrona (liga o loading / limpa o erro) roda durante o render ao
  // abrir; o efeito so busca e aplica o resultado nos callbacks.
  vlog(F, "PedidoModal", "registrando reset do carregamento auxiliar");
  useResetOnOpen(open, [], () => {
    vlog(F, "PedidoModal.resetAux", "ligando loading auxiliar");
    setLoadingAux(true);
    vlog(F, "PedidoModal.resetAux", "limpando erro auxiliar");
    setAuxError(null);
  });

  vlog(F, "PedidoModal", "registrando efeito de carga de vendedores/produtos");
  useEffect(() => {
    vlog(F, "PedidoModal.useEffect", "verificando se o modal está aberto (auxiliares):", open);
    if (!open) return;
    vlog(F, "PedidoModal.useEffect", "inicializando flag de cancelamento (auxiliares)");
    let cancelled = false;

    const loadAllProdutos = async (): Promise<Produto[]> => {
      vlog(F, "PedidoModal.loadAllProdutos", "definindo tamanho de página");
      const PAGE_SIZE = 100;
      vlog(F, "PedidoModal.loadAllProdutos", "buscando primeira página de produtos ativos");
      const first = await apiListProdutos(1, PAGE_SIZE, { ativo: true });
      vlog(F, "PedidoModal.loadAllProdutos", "acumulando produtos da primeira página, qtd:", first.data.length);
      const all = [...first.data];
      vlog(F, "PedidoModal.loadAllProdutos", "lendo total de páginas:", first.pages);
      const totalPages = first.pages || 1;
      vlog(F, "PedidoModal.loadAllProdutos", "buscando páginas restantes de produtos");
      for (let p = 2; p <= totalPages; p++) {
        const res = await apiListProdutos(p, PAGE_SIZE, { ativo: true });
        all.push(...res.data);
      }
      vlog(F, "PedidoModal.loadAllProdutos", "produtos carregados, qtd:", all.length);
      return all;
    };

    vlog(F, "PedidoModal.useEffect", "buscando vendedores e produtos em paralelo");
    Promise.all([apiListVendedores(), loadAllProdutos()])
      .then(([vendedoresRes, todosProdutos]) => {
        vlog(F, "PedidoModal.useEffect", "auxiliares recebidos, cancelado:", cancelled);
        if (cancelled) return;
        vlog(F, "PedidoModal.useEffect", "aplicando vendedores, qtd:", vendedoresRes.length);
        setVendedores(vendedoresRes);
        vlog(F, "PedidoModal.useEffect", "aplicando produtos, qtd:", todosProdutos.length);
        setProdutos(todosProdutos);
      })
      .catch((err) => {
        vlog(F, "PedidoModal.useEffect", "falha ao carregar auxiliares, cancelado:", cancelled);
        if (!cancelled) {
          vlog(F, "PedidoModal.useEffect", "montando mensagem de erro auxiliar");
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar vendedores/produtos.";
          vlog(F, "PedidoModal.useEffect", "exibindo erro auxiliar");
          setAuxError(message);
        }
      })
      .finally(() => {
        vlog(F, "PedidoModal.useEffect", "desligando loading auxiliar se não cancelado");
        if (!cancelled) setLoadingAux(false);
      });
    return () => {
      vlog(F, "PedidoModal.useEffect", "cleanup: cancelando carga auxiliar");
      cancelled = true;
    };
  }, [open]);

  // Na edicao, o cliente original do pedido deve continuar selecionado mesmo
  // que nao esteja (mais) na carteira ativa do vendedor. So vale enquanto o
  // vendedor selecionado for o proprio vendedor do pedido.
  vlog(F, "PedidoModal", "calculando cliente original do pedido em edição");
  const clienteOriginal =
    mode === "edit" && pedido && vendedorId === String(pedido.vendedor_id)
      ? { id: String(pedido.cliente_id), nome: pedido.cliente_nome }
      : null;
  vlog(F, "PedidoModal", "lendo id do cliente original");
  const clienteOriginalId = clienteOriginal?.id ?? "";
  vlog(F, "PedidoModal", "lendo nome do cliente original");
  const clienteOriginalNome = clienteOriginal?.nome ?? "";

  // Dropdown em cascata: sempre que o Vendedor selecionado mudar, recarrega
  // a lista de Clientes via GET /api/vendedores/{id}/clientes (mesmo padrao
  // de VisitaModal/OportunidadeModal, para admin e usuario comum).
  // Parte sincrona roda durante o render quando o modal abre ou o
  // vendedor muda (useResetOnOpen); o efeito so busca e aplica o
  // resultado nos callbacks assincronos.
  vlog(F, "PedidoModal", "registrando reset da cascata de clientes");
  useResetOnOpen(open, [vendedorId], () => {
    vlog(F, "PedidoModal.resetClientes", "limpando erro de clientes");
    setClientesError(null);
    vlog(F, "PedidoModal.resetClientes", "verificando se há vendedor selecionado:", !!vendedorId);
    if (!vendedorId) {
      vlog(F, "PedidoModal.resetClientes", "limpando lista de clientes");
      setClientes([]);
      vlog(F, "PedidoModal.resetClientes", "desligando loading de clientes");
      setLoadingClientes(false);
      return;
    }
    vlog(F, "PedidoModal.resetClientes", "ligando loading de clientes");
    setLoadingClientes(true);
  });

  vlog(F, "PedidoModal", "registrando efeito de carga de clientes do vendedor");
  useEffect(() => {
    vlog(F, "PedidoModal.useEffect", "verificando modal aberto e vendedor selecionado:", open, vendedorId);
    if (!open || !vendedorId) return;
    vlog(F, "PedidoModal.useEffect", "inicializando flag de cancelamento (clientes)");
    let cancelled = false;
    vlog(F, "PedidoModal.useEffect", "buscando clientes do vendedor, id:", vendedorId);
    apiListClientesDoVendedor(Number(vendedorId))
      .then((res) => {
        vlog(F, "PedidoModal.useEffect", "clientes recebidos, qtd/cancelado:", res.length, cancelled);
        if (cancelled) return;
        vlog(F, "PedidoModal.useEffect", "aplicando lista de clientes");
        setClientes(res);
        // Limpa a selecao se o cliente nao pertence a carteira do vendedor,
        // exceto o cliente original do pedido em edicao.
        vlog(F, "PedidoModal.useEffect", "validando cliente selecionado na nova carteira");
        setClienteId((prev) =>
          prev &&
          (res.some((c) => String(c.id) === prev) || prev === clienteOriginalId)
            ? prev
            : ""
        );
      })
      .catch((err) => {
        vlog(F, "PedidoModal.useEffect", "falha ao buscar clientes, cancelado:", cancelled);
        if (!cancelled) {
          vlog(F, "PedidoModal.useEffect", "montando mensagem de erro de clientes");
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar clientes do vendedor.";
          vlog(F, "PedidoModal.useEffect", "exibindo erro de clientes");
          setClientesError(message);
          vlog(F, "PedidoModal.useEffect", "limpando lista de clientes");
          setClientes([]);
        }
      })
      .finally(() => {
        vlog(F, "PedidoModal.useEffect", "desligando loading de clientes se não cancelado");
        if (!cancelled) setLoadingClientes(false);
      });
    return () => {
      vlog(F, "PedidoModal.useEffect", "cleanup: cancelando carga de clientes");
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, vendedorId]);

  vlog(F, "PedidoModal", "memorizando opções de cliente");
  const clienteOptions = useMemo(() => {
    vlog(F, "PedidoModal.clienteOptions", "montando opções de cliente, qtd:", clientes.length);
    const opts = [
      {
        value: "",
        label: !vendedorId
          ? "Selecione um vendedor primeiro"
          : loadingClientes
          ? "Carregando clientes..."
          : "Selecione um cliente",
      },
      ...clientes.map((c) => ({
        value: String(c.id),
        label: `#${c.id} - ${c.razao_social}`,
      })),
    ];
    vlog(F, "PedidoModal.clienteOptions", "verificando se o cliente original precisa ser incluído:", clienteOriginalId);
    if (
      clienteOriginalId &&
      !clientes.some((c) => String(c.id) === clienteOriginalId)
    ) {
      vlog(F, "PedidoModal.clienteOptions", "incluindo cliente original fora da carteira");
      opts.push({
        value: clienteOriginalId,
        label: `#${clienteOriginalId} - ${clienteOriginalNome}${
          loadingClientes ? "" : " (fora da carteira)"
        }`,
      });
    }
    return opts;
  }, [clientes, vendedorId, loadingClientes, clienteOriginalId, clienteOriginalNome]);

  vlog(F, "PedidoModal", "memorizando opções de vendedor");
  const vendedorOptions = useMemo(() => {
    vlog(F, "PedidoModal.vendedorOptions", "montando opções de vendedor, qtd:", vendedores.length);
    const opts = [
      { value: "", label: isAdmin ? "Selecione um vendedor" : "Sem vendedor vinculado" },
      ...vendedores.map((v) => ({
        value: String(v.id),
        label: `#${v.id} - ${v.nome}${v.data_desligamento ? " [X]" : ""}`,
      })),
    ];
    // Usuario comum: garante que o proprio vendedor aparece no select travado
    // mesmo que a lista (escopada) ainda nao tenha carregado.
    vlog(F, "PedidoModal.vendedorOptions", "verificando se o vendedor próprio precisa ser incluído:", ownVendedorId);
    if (
      !isAdmin &&
      ownVendedorId &&
      !vendedores.some((v) => v.id === ownVendedorId)
    ) {
      vlog(F, "PedidoModal.vendedorOptions", "incluindo vendedor próprio nas opções");
      opts.push({
        value: String(ownVendedorId),
        label: `#${ownVendedorId} - ${ownVendedorNome ?? "Meu vendedor"}`,
      });
    }
    return opts;
  }, [vendedores, isAdmin, ownVendedorId, ownVendedorNome]);

  vlog(F, "PedidoModal", "memorizando opções de produto");
  const produtoOptions = useMemo(
    () => [
      { value: "", label: "Selecione um produto" },
      ...produtos.map((p) => ({
        value: String(p.id),
        label: `#${p.id} - ${p.descricao}`,
      })),
    ],
    [produtos]
  );

  vlog(F, "PedidoModal", "memorizando valor total do pedido");
  const valorTotal = useMemo(
    () => itens.reduce((acc, row) => acc + calcValorBruto(row), 0),
    [itens]
  );

  const updateItem = (localId: string, patch: Partial<ItemFormRow>) => {
    vlog(F, "PedidoModal.updateItem", "atualizando linha de item do formulário");
    setItens((prev) =>
      prev.map((row) => (row.localId === localId ? { ...row, ...patch } : row))
    );
  };

  const handleProdutoChange = (localId: string, produtoId: string) => {
    vlog(F, "PedidoModal.handleProdutoChange", "buscando produto selecionado, id:", produtoId);
    const produto = produtos.find((p) => String(p.id) === produtoId);
    vlog(F, "PedidoModal.handleProdutoChange", "atualizando produto e preço do item, produto encontrado:", !!produto);
    updateItem(localId, {
      produto_id: produtoId,
      // Preenche o preco praticado com o preco de tabela do produto (ainda
      // editavel pelo usuario) apenas quando o campo estiver vazio.
      preco_praticado:
        produto && !itens.find((r) => r.localId === localId)?.preco_praticado
          ? String(produto.preco_tabela)
          : itens.find((r) => r.localId === localId)?.preco_praticado ?? "",
    });
  };

  const addItem = () => {
    vlog(F, "PedidoModal.addItem", "adicionando linha de item vazia");
    setItens((prev) => [...prev, emptyItemRow()]);
  };

  const removeItem = (localId: string) => {
    vlog(F, "PedidoModal.removeItem", "removendo linha de item (mantém ao menos uma)");
    setItens((prev) =>
      prev.length > 1 ? prev.filter((row) => row.localId !== localId) : prev
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    vlog(F, "PedidoModal.handleSubmit", "impedindo submit padrão do form");
    e.preventDefault();
    vlog(F, "PedidoModal.handleSubmit", "limpando erro");
    setError(null);

    vlog(F, "PedidoModal.handleSubmit", "verificando se usuário está sem carteira:", semCarteira);
    if (semCarteira) {
      vlog(F, "PedidoModal.handleSubmit", "usuário sem vendedor vinculado");
      setError(
        "Seu usuario nao esta vinculado a um vendedor. Solicite ao administrador o vinculo para registrar pedidos."
      );
      return;
    }
    vlog(F, "PedidoModal.handleSubmit", "validando cliente");
    if (!clienteId) {
      vlog(F, "PedidoModal.handleSubmit", "cliente ausente");
      setError("Cliente e obrigatorio.");
      return;
    }
    vlog(F, "PedidoModal.handleSubmit", "validando vendedor");
    if (!vendedorId) {
      vlog(F, "PedidoModal.handleSubmit", "vendedor ausente");
      setError("Vendedor e obrigatorio.");
      return;
    }
    vlog(F, "PedidoModal.handleSubmit", "validando data do pedido");
    if (!dataPedido) {
      vlog(F, "PedidoModal.handleSubmit", "data do pedido ausente");
      setError("Data do pedido e obrigatoria.");
      return;
    }
    vlog(F, "PedidoModal.handleSubmit", "validando quantidade de itens:", itens.length);
    if (itens.length === 0) {
      vlog(F, "PedidoModal.handleSubmit", "pedido sem itens");
      setError("O pedido deve ter ao menos um item.");
      return;
    }

    vlog(F, "PedidoModal.handleSubmit", "inicializando payload de itens");
    const itensPayload: ItemPedidoInput[] = [];
    vlog(F, "PedidoModal.handleSubmit", "validando e convertendo itens, qtd:", itens.length);
    for (const row of itens) {
      if (!row.produto_id) {
        setError("Selecione o produto em todos os itens.");
        return;
      }
      const quantidade = Number(row.quantidade);
      if (!Number.isFinite(quantidade) || quantidade <= 0) {
        setError("Quantidade deve ser maior que zero em todos os itens.");
        return;
      }
      const precoPraticado = Number(row.preco_praticado);
      if (!Number.isFinite(precoPraticado) || precoPraticado < 0) {
        setError(
          "Preco praticado deve ser um numero maior ou igual a zero em todos os itens."
        );
        return;
      }
      const descontoPct = Number(row.desconto_pct || "0");
      if (!Number.isFinite(descontoPct) || descontoPct < 0 || descontoPct > 100) {
        setError("Desconto deve estar entre 0 e 100 em todos os itens.");
        return;
      }
      itensPayload.push({
        produto_id: Number(row.produto_id),
        quantidade,
        preco_praticado: precoPraticado,
        desconto_pct: descontoPct,
      });
    }
    vlog(F, "PedidoModal.handleSubmit", "itens validados, qtd:", itensPayload.length);

    vlog(F, "PedidoModal.handleSubmit", "marcando envio em andamento");
    setSubmitting(true);
    try {
      vlog(F, "PedidoModal.handleSubmit", "enviando pedido, cliente/vendedor:", Number(clienteId), Number(vendedorId));
      await onSubmit({
        cliente_id: Number(clienteId),
        vendedor_id: Number(vendedorId),
        data_pedido: dataPedido,
        canal,
        status,
        itens: itensPayload,
      });
    } catch (err) {
      vlog(F, "PedidoModal.handleSubmit", "falha ao salvar pedido: montando mensagem de erro");
      const message =
        err instanceof Error
          ? friendlyPedidoError(err.message)
          : "Erro ao salvar pedido.";
      vlog(F, "PedidoModal.handleSubmit", "exibindo erro");
      setError(message);
    } finally {
      vlog(F, "PedidoModal.handleSubmit", "finalizando envio");
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Novo Pedido" : "Editar Pedido"}
      size="lg"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Alert variant="error">{error}</Alert>}
        {auxError && (
          <Alert variant="error">
            Nao foi possivel carregar vendedores/produtos: {auxError}
          </Alert>
        )}
        {clientesError && (
          <Alert variant="error">
            Nao foi possivel carregar clientes do vendedor: {clientesError}
          </Alert>
        )}
        {semCarteira && (
          <Alert variant="warning">
            Seu usuario nao esta vinculado a um vendedor, por isso nao e
            possivel registrar pedidos. Solicite o vinculo ao administrador.
          </Alert>
        )}

        <div className="grid grid-cols-2 gap-3">
          <Select
            label="Vendedor"
            value={vendedorId}
            onChange={(e) => setVendedorId(e.target.value)}
            options={vendedorOptions}
            disabled={loadingAux || !isAdmin}
            required
          />
          <Select
            label="Cliente"
            value={clienteId}
            onChange={(e) => setClienteId(e.target.value)}
            options={clienteOptions}
            disabled={!vendedorId || loadingClientes}
            required
          />
        </div>

        <div className="grid grid-cols-3 gap-3">
          <Input
            label="Data do pedido"
            type="date"
            value={dataPedido}
            onChange={(e) => setDataPedido(e.target.value)}
            required
          />
          <Select
            label="Canal"
            value={canal}
            onChange={(e) => setCanal(e.target.value)}
            options={CANAL_OPTIONS}
            required
          />
          <Select
            label="Status"
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            options={STATUS_OPTIONS}
            required
          />
        </div>

        <div>
          <div className="mb-2 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-700">Itens</h3>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={addItem}
              disabled={loadingAux}
            >
              + Adicionar item
            </Button>
          </div>

          <div className="max-h-80 space-y-3 overflow-y-auto overflow-x-auto rounded-md border border-slate-200 p-3">
            {itens.map((row) => (
              <div
                key={row.localId}
                className="grid grid-cols-12 gap-2 rounded-md border border-slate-100 bg-slate-50 p-2"
              >
                <div className="col-span-4">
                  <Select
                    label="Produto"
                    value={row.produto_id}
                    onChange={(e) => handleProdutoChange(row.localId, e.target.value)}
                    options={produtoOptions}
                    disabled={loadingAux}
                  />
                </div>
                <div className="col-span-2">
                  <Input
                    label="Qtd"
                    type="number"
                    min="1"
                    step="1"
                    value={row.quantidade}
                    onChange={(e) =>
                      updateItem(row.localId, { quantidade: e.target.value })
                    }
                  />
                </div>
                <div className="col-span-2">
                  <Input
                    label="Preco"
                    type="number"
                    min="0"
                    step="0.01"
                    value={row.preco_praticado}
                    onChange={(e) =>
                      updateItem(row.localId, { preco_praticado: e.target.value })
                    }
                  />
                </div>
                <div className="col-span-2">
                  <Input
                    label="Desc. %"
                    type="number"
                    min="0"
                    max="100"
                    step="0.01"
                    value={row.desconto_pct}
                    onChange={(e) =>
                      updateItem(row.localId, { desconto_pct: e.target.value })
                    }
                  />
                </div>
                <div className="col-span-1 flex items-end justify-end pb-2 text-xs font-medium text-slate-600">
                  {currencyFmt.format(calcValorBruto(row))}
                </div>
                <div className="col-span-1 flex items-end justify-end pb-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => removeItem(row.localId)}
                    disabled={itens.length <= 1}
                    title="Remover item"
                  >
                    ✕
                  </Button>
                </div>
              </div>
            ))}
          </div>

          <div className="mt-2 flex justify-end text-sm font-semibold text-slate-800">
            Total do pedido: {currencyFmt.format(valorTotal)}
          </div>
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button
            type="submit"
            loading={submitting}
            disabled={loadingAux || loadingClientes || semCarteira}
          >
            {mode === "create" ? "Criar pedido" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
