"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { apiListPedidos } from "@/lib/api";
import {
  Pagamento,
  PagamentoCreateInput,
  PagamentoUpdateInput,
  Pedido,
} from "@/lib/types";
import { useResetOnOpen } from "@/lib/useResetOnOpen";

const currencyFmt = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
});

function fmtDate(dateStr: string): string {
  if (!dateStr) return "-";
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  return d.toLocaleDateString("pt-BR");
}

function pedidoOptionLabel(p: Pedido): string {
  return `#${p.pedido_id_origem} - ${p.cliente_nome} - ${currencyFmt.format(
    p.valor_total ?? 0
  )} - ${fmtDate(p.data_pedido)}`;
}

interface PagamentoModalProps {
  open: boolean;
  mode: "create" | "edit";
  pagamento: Pagamento | null;
  onClose: () => void;
  onSubmit: (data: PagamentoCreateInput | PagamentoUpdateInput) => Promise<void>;
}

// Valores de ENUM espelhando o backend (ver
// apis/rotaperfumes-api/services/pagamento_service.go).
const FORMA_PAGAMENTO_OPTIONS = [
  { value: "Boleto 14 dias", label: "Boleto 14 dias" },
  { value: "Boleto 28 dias", label: "Boleto 28 dias" },
  { value: "Cartão de crédito", label: "Cartão de crédito" },
  { value: "Cartão de débito", label: "Cartão de débito" },
  { value: "Cheque a prazo", label: "Cheque a prazo" },
  { value: "Dinheiro", label: "Dinheiro" },
  { value: "PIX", label: "PIX" },
];

const STATUS_PAGAMENTO_OPTIONS = [
  { value: "Em aberto", label: "Em aberto" },
  { value: "Inadimplente", label: "Inadimplente" },
  { value: "Pago", label: "Pago" },
  { value: "Pago com atraso", label: "Pago com atraso" },
];

export function PagamentoModal({
  open,
  mode,
  pagamento,
  onClose,
  onSubmit,
}: PagamentoModalProps) {
  const [pedidoId, setPedidoId] = useState("");
  const [pedidoQuery, setPedidoQuery] = useState("");
  const [pedidoOptions, setPedidoOptions] = useState<Pedido[]>([]);
  const [loadingPedidos, setLoadingPedidos] = useState(false);
  const [formaPagamento, setFormaPagamento] = useState(
    FORMA_PAGAMENTO_OPTIONS[0].value
  );
  const [parcelas, setParcelas] = useState("1");
  const [valor, setValor] = useState("");
  const [taxaPct, setTaxaPct] = useState("0");
  const [valorLiquido, setValorLiquido] = useState("");
  const [dataVencimento, setDataVencimento] = useState("");
  const [dataPagamento, setDataPagamento] = useState("");
  const [statusPagamento, setStatusPagamento] = useState(
    STATUS_PAGAMENTO_OPTIONS[0].value
  );
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  useResetOnOpen(open, [mode, pagamento], () => {
    setError(null);
    setSubmitting(false);
    if (mode === "edit" && pagamento) {
      setPedidoId(String(pagamento.pedido_id));
      setFormaPagamento(pagamento.forma_pagamento);
      setParcelas(String(pagamento.parcelas));
      setValor(String(pagamento.valor));
      setTaxaPct(String(pagamento.taxa_pct));
      setValorLiquido(String(pagamento.valor_liquido));
      setDataVencimento(pagamento.data_vencimento.slice(0, 10));
      setDataPagamento(
        pagamento.data_pagamento ? pagamento.data_pagamento.slice(0, 10) : ""
      );
      setStatusPagamento(pagamento.status_pagamento);
    } else {
      setPedidoId("");
      setPedidoQuery("");
      setPedidoOptions([]);
      setFormaPagamento(FORMA_PAGAMENTO_OPTIONS[0].value);
      setParcelas("1");
      setValor("");
      setTaxaPct("0");
      setValorLiquido("");
      setDataVencimento("");
      setDataPagamento("");
      setStatusPagamento(STATUS_PAGAMENTO_OPTIONS[0].value);
    }
  });

  // Busca a lista de pedidos para o select (create), com filtro por texto
  // (parametro `q`) e debounce ao digitar. Refaz a busca a cada mudanca de
  // pedidoQuery enquanto o modal estiver aberto em modo de criacao.
  useEffect(() => {
    if (!open || mode !== "create") return;
    let cancelled = false;
    const timer = setTimeout(async () => {
      setLoadingPedidos(true);
      try {
        const res = await apiListPedidos(
          1,
          20,
          { q: pedidoQuery.trim() || undefined },
          "data_pedido",
          "desc"
        );
        if (!cancelled) {
          setPedidoOptions(res.data);
        }
      } catch {
        if (!cancelled) setPedidoOptions([]);
      } finally {
        if (!cancelled) setLoadingPedidos(false);
      }
    }, 350);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [open, mode, pedidoQuery]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const pedidoIdNum = Number(pedidoId);
    if (mode === "create" && (!pedidoId.trim() || Number.isNaN(pedidoIdNum) || pedidoIdNum <= 0)) {
      setError("Selecione um pedido.");
      return;
    }
    const parcelasNum = Number(parcelas);
    if (parcelas.trim() === "" || Number.isNaN(parcelasNum) || parcelasNum < 1) {
      setError("Parcelas deve ser um numero maior ou igual a 1.");
      return;
    }
    const valorNum = Number(valor);
    if (valor.trim() === "" || Number.isNaN(valorNum) || valorNum < 0) {
      setError("Valor deve ser um numero maior ou igual a zero.");
      return;
    }
    const taxaPctNum = Number(taxaPct);
    if (taxaPct.trim() === "" || Number.isNaN(taxaPctNum) || taxaPctNum < 0) {
      setError("Taxa (%) deve ser um numero maior ou igual a zero.");
      return;
    }
    const valorLiquidoNum = Number(valorLiquido);
    if (
      valorLiquido.trim() === "" ||
      Number.isNaN(valorLiquidoNum) ||
      valorLiquidoNum < 0
    ) {
      setError("Valor liquido deve ser um numero maior ou igual a zero.");
      return;
    }
    if (!dataVencimento) {
      setError("Data de vencimento e obrigatoria.");
      return;
    }

    setSubmitting(true);
    try {
      if (mode === "create") {
        const payload: PagamentoCreateInput = {
          pedido_id: pedidoIdNum,
          forma_pagamento: formaPagamento,
          parcelas: parcelasNum,
          valor: valorNum,
          taxa_pct: taxaPctNum,
          valor_liquido: valorLiquidoNum,
          data_vencimento: dataVencimento,
          data_pagamento: dataPagamento || undefined,
          status_pagamento: statusPagamento,
        };
        await onSubmit(payload);
      } else {
        const payload: PagamentoUpdateInput = {
          forma_pagamento: formaPagamento,
          parcelas: parcelasNum,
          valor: valorNum,
          taxa_pct: taxaPctNum,
          valor_liquido: valorLiquidoNum,
          data_vencimento: dataVencimento,
          data_pagamento: dataPagamento || undefined,
          status_pagamento: statusPagamento,
        };
        await onSubmit(payload);
      }
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao salvar pagamento.";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Novo Pagamento" : "Editar Pagamento"}
      size="md"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Alert variant="error">{error}</Alert>}

        {mode === "edit" && pagamento && (
          <div className="grid grid-cols-2 gap-3">
            <Input
              label="ID do pagamento"
              value={String(pagamento.pagamento_id)}
              disabled
              readOnly
            />
            <Input
              label="ID do pedido"
              value={String(pagamento.pedido_id)}
              disabled
              readOnly
              helperText="Nao editavel"
            />
          </div>
        )}

        {mode === "create" && (
          <div className="space-y-2">
            <Input
              label="Buscar pedido"
              value={pedidoQuery}
              onChange={(e) => setPedidoQuery(e.target.value)}
              placeholder="Filtrar por cliente ou ID do pedido"
              autoFocus
            />
            <Select
              label="Pedido"
              options={[
                {
                  value: "",
                  label: loadingPedidos
                    ? "Carregando pedidos..."
                    : pedidoOptions.length === 0
                    ? "Nenhum pedido encontrado"
                    : "Selecione um pedido",
                },
                ...pedidoOptions.map((p) => ({
                  value: String(p.pedido_id_origem),
                  label: pedidoOptionLabel(p),
                })),
              ]}
              value={pedidoId}
              onChange={(e) => setPedidoId(e.target.value)}
              required
            />
          </div>
        )}

        <div className="grid grid-cols-2 gap-3">
          <Select
            label="Forma de pagamento"
            options={FORMA_PAGAMENTO_OPTIONS}
            value={formaPagamento}
            onChange={(e) => setFormaPagamento(e.target.value)}
          />
          <Select
            label="Status"
            options={STATUS_PAGAMENTO_OPTIONS}
            value={statusPagamento}
            onChange={(e) => setStatusPagamento(e.target.value)}
          />
        </div>

        <div className="grid grid-cols-3 gap-3">
          <Input
            label="Parcelas"
            type="number"
            min="1"
            step="1"
            value={parcelas}
            onChange={(e) => setParcelas(e.target.value)}
            required
          />
          <Input
            label="Valor"
            type="number"
            step="0.01"
            min="0"
            value={valor}
            onChange={(e) => setValor(e.target.value)}
            placeholder="0,00"
            required
          />
          <Input
            label="Taxa (%)"
            type="number"
            step="0.01"
            min="0"
            value={taxaPct}
            onChange={(e) => setTaxaPct(e.target.value)}
            placeholder="0,00"
            required
          />
        </div>

        <Input
          label="Valor liquido"
          type="number"
          step="0.01"
          min="0"
          value={valorLiquido}
          onChange={(e) => setValorLiquido(e.target.value)}
          placeholder="0,00"
          helperText="Informado explicitamente, nao e calculado automaticamente"
          required
        />

        <div className="grid grid-cols-2 gap-3">
          <Input
            label="Data de vencimento"
            type="date"
            value={dataVencimento}
            onChange={(e) => setDataVencimento(e.target.value)}
            required
          />
          <Input
            label="Data de pagamento"
            type="date"
            value={dataPagamento}
            onChange={(e) => setDataPagamento(e.target.value)}
            helperText="Opcional"
          />
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button type="submit" loading={submitting}>
            {mode === "create" ? "Criar pagamento" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
