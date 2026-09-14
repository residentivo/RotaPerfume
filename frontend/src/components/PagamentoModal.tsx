"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import {
  Pagamento,
  PagamentoCreateInput,
  PagamentoUpdateInput,
} from "@/lib/types";

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

  useEffect(() => {
    if (open) {
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
        setFormaPagamento(FORMA_PAGAMENTO_OPTIONS[0].value);
        setParcelas("1");
        setValor("");
        setTaxaPct("0");
        setValorLiquido("");
        setDataVencimento("");
        setDataPagamento("");
        setStatusPagamento(STATUS_PAGAMENTO_OPTIONS[0].value);
      }
    }
  }, [open, mode, pagamento]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const pedidoIdNum = Number(pedidoId);
    if (mode === "create" && (!pedidoId.trim() || Number.isNaN(pedidoIdNum) || pedidoIdNum <= 0)) {
      setError("Pedido (ID) e obrigatorio e deve ser um numero maior que zero.");
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
          <Input
            label="ID do pedido"
            type="number"
            min="1"
            step="1"
            value={pedidoId}
            onChange={(e) => setPedidoId(e.target.value)}
            placeholder="Ex: 123"
            required
            autoFocus
          />
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
