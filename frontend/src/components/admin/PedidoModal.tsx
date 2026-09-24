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
import {
  apiListVendedores,
  apiListProdutos,
  apiListClientesDoVendedor,
} from "@/lib/api";
import { getSessionUser, refreshSessionUser, useSessionUser } from "@/lib/session";

// Traduz mensagens de erro do backend para textos mais amigaveis. Hoje trata
// o 400 "cliente nao pertence a carteira deste vendedor" (escopo por carteira
// aplicado em POST/PUT /api/pedidos para usuarios nao-admin).
function friendlyPedidoError(message: string): string {
  const normalized = message
    .toLowerCase()
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "");
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
  const q = Number(row.quantidade);
  const p = Number(row.preco_praticado);
  const d = Number(row.desconto_pct);
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
  const [clienteId, setClienteId] = useState("");
  const [vendedorId, setVendedorId] = useState("");
  const [dataPedido, setDataPedido] = useState("");
  const [canal, setCanal] = useState("App");
  const [status, setStatus] = useState("Em separação");
  const [itens, setItens] = useState<ItemFormRow[]>([emptyItemRow()]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Listas auxiliares para os selects (mesmo padrão do UserModal com
  // apiListVendedores).
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  const [produtos, setProdutos] = useState<Produto[]>([]);
  const [loadingAux, setLoadingAux] = useState(false);
  const [auxError, setAuxError] = useState<string | null>(null);

  // Clientes em cascata pelo vendedor selecionado (carteira ativa via
  // GET /api/vendedores/{id}/clientes), mesmo padrao de Visita/Oportunidade.
  const [clientes, setClientes] = useState<ClienteResumo[]>([]);
  const [loadingClientes, setLoadingClientes] = useState(false);
  const [clientesError, setClientesError] = useState<string | null>(null);

  // Usuario logado: sessao em memoria validada por GET /api/auth/me (fonte da
  // verdade; revalidada ao montar o ProtectedRoute, ao voltar o foco da aba e
  // ao abrir este modal). Usuario nao-admin tem o vendedor travado no proprio
  // id_vendedor — o backend tambem forca isso em POST/PUT /api/pedidos.
  const sessionUser = useSessionUser();
  const isAdmin = sessionUser?.role === "admin";
  const ownVendedorId =
    !isAdmin && sessionUser?.id_vendedor ? sessionUser.id_vendedor : null;
  const ownVendedorNome = !isAdmin ? sessionUser?.vendedor_nome ?? null : null;
  const semCarteira = !isAdmin && !ownVendedorId;

  // Ao abrir, rebusca /me para pegar um vinculo alterado pelo admin.
  useEffect(() => {
    if (!open) return;
    refreshSessionUser().catch(() => {
      // Falha aqui nao bloqueia o formulario; o backend valida o vendedor.
    });
  }, [open]);

  // Mantem o vendedor travado sincronizado com a sessao (usuario normal).
  useEffect(() => {
    if (open && !isAdmin) {
      setVendedorId(ownVendedorId ? String(ownVendedorId) : "");
    }
  }, [open, isAdmin, ownVendedorId]);

  useEffect(() => {
    if (open) {
      setError(null);
      setSubmitting(false);
      const user = getSessionUser();
      const admin = user?.role === "admin";
      const ownId = !admin && user?.id_vendedor ? user.id_vendedor : null;
      const lockedVendedor = !admin ? (ownId ? String(ownId) : "") : null;
      if (mode === "edit" && pedido) {
        setClienteId(String(pedido.cliente_id));
        setVendedorId(lockedVendedor ?? String(pedido.vendedor_id));
        setDataPedido(pedido.data_pedido ? pedido.data_pedido.slice(0, 10) : "");
        setCanal(pedido.canal);
        setStatus(pedido.status);
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
        setClienteId("");
        setVendedorId(lockedVendedor ?? "");
        setDataPedido(todayISO());
        setCanal("App");
        setStatus("Em separação");
        setItens([emptyItemRow()]);
      }
    }
  }, [open, mode, pedido]);

  // Carrega clientes/vendedores/produtos para popular os selects. Reutiliza
  // os endpoints ja existentes GET /api/clientes, /api/vendedores e
  // /api/produtos (apenas ativos, limite alto para cobrir a maioria dos casos).
  //
  // O backend limita `limit` a no maximo 100 itens por pagina (ver
  // services.ParsePagination), entao para o catalogo de produtos (que pode
  // ter varias centenas de itens) e preciso paginar ate esgotar todas as
  // paginas, em vez de confiar em um unico limit alto.
  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setLoadingAux(true);
    setAuxError(null);

    const loadAllProdutos = async (): Promise<Produto[]> => {
      const PAGE_SIZE = 100;
      const first = await apiListProdutos(1, PAGE_SIZE, { ativo: true });
      const all = [...first.data];
      const totalPages = first.pages || 1;
      for (let p = 2; p <= totalPages; p++) {
        const res = await apiListProdutos(p, PAGE_SIZE, { ativo: true });
        all.push(...res.data);
      }
      return all;
    };

    Promise.all([apiListVendedores(), loadAllProdutos()])
      .then(([vendedoresRes, todosProdutos]) => {
        if (cancelled) return;
        setVendedores(vendedoresRes);
        setProdutos(todosProdutos);
      })
      .catch((err) => {
        if (!cancelled) {
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar vendedores/produtos.";
          setAuxError(message);
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingAux(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open]);

  // Na edicao, o cliente original do pedido deve continuar selecionado mesmo
  // que nao esteja (mais) na carteira ativa do vendedor. So vale enquanto o
  // vendedor selecionado for o proprio vendedor do pedido.
  const clienteOriginal =
    mode === "edit" && pedido && vendedorId === String(pedido.vendedor_id)
      ? { id: String(pedido.cliente_id), nome: pedido.cliente_nome }
      : null;
  const clienteOriginalId = clienteOriginal?.id ?? "";

  // Dropdown em cascata: sempre que o Vendedor selecionado mudar, recarrega
  // a lista de Clientes via GET /api/vendedores/{id}/clientes (mesmo padrao
  // de VisitaModal/OportunidadeModal, para admin e usuario comum).
  useEffect(() => {
    if (!open) return;
    if (!vendedorId) {
      setClientes([]);
      setClientesError(null);
      return;
    }
    let cancelled = false;
    setLoadingClientes(true);
    setClientesError(null);
    apiListClientesDoVendedor(Number(vendedorId))
      .then((res) => {
        if (cancelled) return;
        setClientes(res);
        // Limpa a selecao se o cliente nao pertence a carteira do vendedor,
        // exceto o cliente original do pedido em edicao.
        setClienteId((prev) =>
          prev &&
          (res.some((c) => String(c.id) === prev) || prev === clienteOriginalId)
            ? prev
            : ""
        );
      })
      .catch((err) => {
        if (!cancelled) {
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar clientes do vendedor.";
          setClientesError(message);
          setClientes([]);
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingClientes(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, vendedorId]);

  const clienteOptions = useMemo(() => {
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
    if (
      clienteOriginal &&
      !clientes.some((c) => String(c.id) === clienteOriginal.id)
    ) {
      opts.push({
        value: clienteOriginal.id,
        label: `#${clienteOriginal.id} - ${clienteOriginal.nome}${
          loadingClientes ? "" : " (fora da carteira)"
        }`,
      });
    }
    return opts;
  }, [clientes, vendedorId, loadingClientes, clienteOriginal?.id, clienteOriginal?.nome]);

  const vendedorOptions = useMemo(() => {
    const opts = [
      { value: "", label: isAdmin ? "Selecione um vendedor" : "Sem vendedor vinculado" },
      ...vendedores.map((v) => ({
        value: String(v.id),
        label: `#${v.id} - ${v.nome}${v.data_desligamento ? " [X]" : ""}`,
      })),
    ];
    // Usuario comum: garante que o proprio vendedor aparece no select travado
    // mesmo que a lista (escopada) ainda nao tenha carregado.
    if (
      !isAdmin &&
      ownVendedorId &&
      !vendedores.some((v) => v.id === ownVendedorId)
    ) {
      opts.push({
        value: String(ownVendedorId),
        label: `#${ownVendedorId} - ${ownVendedorNome ?? "Meu vendedor"}`,
      });
    }
    return opts;
  }, [vendedores, isAdmin, ownVendedorId, ownVendedorNome]);

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

  const valorTotal = useMemo(
    () => itens.reduce((acc, row) => acc + calcValorBruto(row), 0),
    [itens]
  );

  const updateItem = (localId: string, patch: Partial<ItemFormRow>) => {
    setItens((prev) =>
      prev.map((row) => (row.localId === localId ? { ...row, ...patch } : row))
    );
  };

  const handleProdutoChange = (localId: string, produtoId: string) => {
    const produto = produtos.find((p) => String(p.id) === produtoId);
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
    setItens((prev) => [...prev, emptyItemRow()]);
  };

  const removeItem = (localId: string) => {
    setItens((prev) =>
      prev.length > 1 ? prev.filter((row) => row.localId !== localId) : prev
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (semCarteira) {
      setError(
        "Seu usuario nao esta vinculado a um vendedor. Solicite ao administrador o vinculo para registrar pedidos."
      );
      return;
    }
    if (!clienteId) {
      setError("Cliente e obrigatorio.");
      return;
    }
    if (!vendedorId) {
      setError("Vendedor e obrigatorio.");
      return;
    }
    if (!dataPedido) {
      setError("Data do pedido e obrigatoria.");
      return;
    }
    if (itens.length === 0) {
      setError("O pedido deve ter ao menos um item.");
      return;
    }

    const itensPayload: ItemPedidoInput[] = [];
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

    setSubmitting(true);
    try {
      await onSubmit({
        cliente_id: Number(clienteId),
        vendedor_id: Number(vendedorId),
        data_pedido: dataPedido,
        canal,
        status,
        itens: itensPayload,
      });
    } catch (err) {
      const message =
        err instanceof Error
          ? friendlyPedidoError(err.message)
          : "Erro ao salvar pedido.";
      setError(message);
    } finally {
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
