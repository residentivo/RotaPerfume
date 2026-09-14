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
  Cliente,
  Vendedor,
  Produto,
} from "@/lib/types";
import { apiListClientes, apiListVendedores, apiListProdutos } from "@/lib/api";

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
  const [clientes, setClientes] = useState<Cliente[]>([]);
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  const [produtos, setProdutos] = useState<Produto[]>([]);
  const [loadingAux, setLoadingAux] = useState(false);
  const [auxError, setAuxError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setError(null);
      setSubmitting(false);
      if (mode === "edit" && pedido) {
        setClienteId(String(pedido.cliente_id));
        setVendedorId(String(pedido.vendedor_id));
        setDataPedido(pedido.data_pedido ? pedido.data_pedido.slice(0, 10) : "");
        setCanal(pedido.canal);
        setStatus(pedido.status);
        setItens(
          pedido.itens.length > 0
            ? pedido.itens.map((it) => ({
                localId: String(it.id),
                produto_id: String(it.produto_id),
                quantidade: String(it.quantidade),
                preco_praticado: String(it.preco_praticado),
                desconto_pct: String(it.desconto_pct),
              }))
            : [emptyItemRow()]
        );
      } else {
        setClienteId("");
        setVendedorId("");
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

    Promise.all([
      apiListClientes(1, 100, { ativo: true }),
      apiListVendedores(),
      loadAllProdutos(),
    ])
      .then(([clientesRes, vendedoresRes, todosProdutos]) => {
        if (cancelled) return;
        setClientes(clientesRes.data);
        setVendedores(vendedoresRes);
        setProdutos(todosProdutos);
      })
      .catch((err) => {
        if (!cancelled) {
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar clientes/vendedores/produtos.";
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

  const clienteOptions = useMemo(
    () => [
      { value: "", label: "Selecione um cliente" },
      ...clientes.map((c) => ({
        value: String(c.id),
        label: `#${c.id} - ${c.razao_social}`,
      })),
    ],
    [clientes]
  );

  const vendedorOptions = useMemo(
    () => [
      { value: "", label: "Selecione um vendedor" },
      ...vendedores.map((v) => ({
        value: String(v.id),
        label: `#${v.id} - ${v.nome}`,
      })),
    ],
    [vendedores]
  );

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
        err instanceof Error ? err.message : "Erro ao salvar pedido.";
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
            Nao foi possivel carregar clientes/vendedores/produtos: {auxError}
          </Alert>
        )}

        <div className="grid grid-cols-2 gap-3">
          <Select
            label="Cliente"
            value={clienteId}
            onChange={(e) => setClienteId(e.target.value)}
            options={clienteOptions}
            disabled={loadingAux}
            required
          />
          <Select
            label="Vendedor"
            value={vendedorId}
            onChange={(e) => setVendedorId(e.target.value)}
            options={vendedorOptions}
            disabled={loadingAux}
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
          <Button type="submit" loading={submitting} disabled={loadingAux}>
            {mode === "create" ? "Criar pedido" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
