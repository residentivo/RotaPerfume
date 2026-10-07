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
import { formatarData } from "@/lib/formatarData";
import { vlog } from "@/lib/vlog";

const F = "PagamentoModal.tsx";

const currencyFmt = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
});

function pedidoOptionLabel(p: Pedido): string {
  return `#${p.pedido_id_origem} - ${p.cliente_nome} - ${currencyFmt.format(
    p.valor_total ?? 0
  )} - ${formatarData(p.data_pedido)}`;
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
  vlog(F, "PagamentoModal", "criando estado pedidoId, modo:", mode);
  const [pedidoId, setPedidoId] = useState("");
  vlog(F, "PagamentoModal", "criando estado pedidoQuery");
  const [pedidoQuery, setPedidoQuery] = useState("");
  vlog(F, "PagamentoModal", "criando estado pedidoOptions");
  const [pedidoOptions, setPedidoOptions] = useState<Pedido[]>([]);
  vlog(F, "PagamentoModal", "criando estado loadingPedidos");
  const [loadingPedidos, setLoadingPedidos] = useState(false);
  vlog(F, "PagamentoModal", "criando estado formaPagamento");
  const [formaPagamento, setFormaPagamento] = useState(
    FORMA_PAGAMENTO_OPTIONS[0].value
  );
  vlog(F, "PagamentoModal", "criando estado parcelas");
  const [parcelas, setParcelas] = useState("1");
  vlog(F, "PagamentoModal", "criando estado valor");
  const [valor, setValor] = useState("");
  vlog(F, "PagamentoModal", "criando estado taxaPct");
  const [taxaPct, setTaxaPct] = useState("0");
  vlog(F, "PagamentoModal", "criando estado valorLiquido");
  const [valorLiquido, setValorLiquido] = useState("");
  vlog(F, "PagamentoModal", "criando estado dataVencimento");
  const [dataVencimento, setDataVencimento] = useState("");
  vlog(F, "PagamentoModal", "criando estado dataPagamento");
  const [dataPagamento, setDataPagamento] = useState("");
  vlog(F, "PagamentoModal", "criando estado statusPagamento");
  const [statusPagamento, setStatusPagamento] = useState(
    STATUS_PAGAMENTO_OPTIONS[0].value
  );
  vlog(F, "PagamentoModal", "criando estado submitting");
  const [submitting, setSubmitting] = useState(false);
  vlog(F, "PagamentoModal", "criando estado error");
  const [error, setError] = useState<string | null>(null);

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  vlog(F, "PagamentoModal", "registrando reset do formulário ao abrir");
  useResetOnOpen(open, [mode, pagamento], () => {
    vlog(F, "PagamentoModal.reset", "limpando erro");
    setError(null);
    vlog(F, "PagamentoModal.reset", "limpando estado de envio");
    setSubmitting(false);
    vlog(F, "PagamentoModal.reset", "verificando se é edição com pagamento, id:", pagamento?.pagamento_id);
    if (mode === "edit" && pagamento) {
      vlog(F, "PagamentoModal.reset", "preenchendo pedido, id:", pagamento.pedido_id);
      setPedidoId(String(pagamento.pedido_id));
      vlog(F, "PagamentoModal.reset", "preenchendo forma de pagamento");
      setFormaPagamento(pagamento.forma_pagamento);
      vlog(F, "PagamentoModal.reset", "preenchendo parcelas");
      setParcelas(String(pagamento.parcelas));
      vlog(F, "PagamentoModal.reset", "preenchendo valor");
      setValor(String(pagamento.valor));
      vlog(F, "PagamentoModal.reset", "preenchendo taxa");
      setTaxaPct(String(pagamento.taxa_pct));
      vlog(F, "PagamentoModal.reset", "preenchendo valor líquido");
      setValorLiquido(String(pagamento.valor_liquido));
      vlog(F, "PagamentoModal.reset", "preenchendo data de vencimento");
      setDataVencimento(pagamento.data_vencimento.slice(0, 10));
      vlog(F, "PagamentoModal.reset", "preenchendo data de pagamento");
      setDataPagamento(
        pagamento.data_pagamento ? pagamento.data_pagamento.slice(0, 10) : ""
      );
      vlog(F, "PagamentoModal.reset", "preenchendo status do pagamento");
      setStatusPagamento(pagamento.status_pagamento);
    } else {
      vlog(F, "PagamentoModal.reset", "limpando pedido");
      setPedidoId("");
      vlog(F, "PagamentoModal.reset", "limpando busca de pedido");
      setPedidoQuery("");
      vlog(F, "PagamentoModal.reset", "limpando opções de pedido");
      setPedidoOptions([]);
      vlog(F, "PagamentoModal.reset", "definindo forma de pagamento padrão");
      setFormaPagamento(FORMA_PAGAMENTO_OPTIONS[0].value);
      vlog(F, "PagamentoModal.reset", "definindo parcelas padrão");
      setParcelas("1");
      vlog(F, "PagamentoModal.reset", "limpando valor");
      setValor("");
      vlog(F, "PagamentoModal.reset", "definindo taxa padrão");
      setTaxaPct("0");
      vlog(F, "PagamentoModal.reset", "limpando valor líquido");
      setValorLiquido("");
      vlog(F, "PagamentoModal.reset", "limpando data de vencimento");
      setDataVencimento("");
      vlog(F, "PagamentoModal.reset", "limpando data de pagamento");
      setDataPagamento("");
      vlog(F, "PagamentoModal.reset", "definindo status padrão");
      setStatusPagamento(STATUS_PAGAMENTO_OPTIONS[0].value);
    }
  });

  // Busca a lista de pedidos para o select (create), com filtro por texto
  // (parametro `q`) e debounce ao digitar. Refaz a busca a cada mudanca de
  // pedidoQuery enquanto o modal estiver aberto em modo de criacao.
  vlog(F, "PagamentoModal", "registrando efeito de busca de pedidos");
  useEffect(() => {
    vlog(F, "PagamentoModal.useEffect", "verificando se modal aberto em criação:", open, mode);
    if (!open || mode !== "create") return;
    vlog(F, "PagamentoModal.useEffect", "inicializando flag de cancelamento");
    let cancelled = false;
    vlog(F, "PagamentoModal.useEffect", "armando debounce da busca de pedidos (350ms)");
    const timer = setTimeout(async () => {
      vlog(F, "PagamentoModal.buscarPedidos", "ligando loading de pedidos");
      setLoadingPedidos(true);
      try {
        // LOG-02: o texto da busca nao e logado, so se foi informado.
        vlog(F, "PagamentoModal.buscarPedidos", "buscando pedidos, com filtro de texto:", !!pedidoQuery.trim());
        const res = await apiListPedidos(
          1,
          20,
          { q: pedidoQuery.trim() || undefined },
          "data_pedido",
          "desc"
        );
        vlog(F, "PagamentoModal.buscarPedidos", "pedidos recebidos, qtd/cancelado:", res.data.length, cancelled);
        if (!cancelled) {
          vlog(F, "PagamentoModal.buscarPedidos", "aplicando opções de pedido");
          setPedidoOptions(res.data);
        }
      } catch {
        vlog(F, "PagamentoModal.buscarPedidos", "falha ao buscar pedidos: limpando opções se não cancelado");
        if (!cancelled) setPedidoOptions([]);
      } finally {
        vlog(F, "PagamentoModal.buscarPedidos", "desligando loading de pedidos se não cancelado");
        if (!cancelled) setLoadingPedidos(false);
      }
    }, 350);
    return () => {
      vlog(F, "PagamentoModal.useEffect", "cleanup: marcando busca como cancelada");
      cancelled = true;
      vlog(F, "PagamentoModal.useEffect", "cleanup: cancelando debounce");
      clearTimeout(timer);
    };
  }, [open, mode, pedidoQuery]);

  const handleSubmit = async (e: React.FormEvent) => {
    vlog(F, "PagamentoModal.handleSubmit", "impedindo submit padrão do form");
    e.preventDefault();
    vlog(F, "PagamentoModal.handleSubmit", "limpando erro");
    setError(null);

    vlog(F, "PagamentoModal.handleSubmit", "convertendo id do pedido");
    const pedidoIdNum = Number(pedidoId);
    vlog(F, "PagamentoModal.handleSubmit", "validando pedido na criação, id:", pedidoIdNum);
    if (mode === "create" && (!pedidoId.trim() || Number.isNaN(pedidoIdNum) || pedidoIdNum <= 0)) {
      vlog(F, "PagamentoModal.handleSubmit", "pedido não selecionado");
      setError("Selecione um pedido.");
      return;
    }
    vlog(F, "PagamentoModal.handleSubmit", "convertendo parcelas");
    const parcelasNum = Number(parcelas);
    vlog(F, "PagamentoModal.handleSubmit", "validando parcelas:", parcelasNum);
    if (parcelas.trim() === "" || Number.isNaN(parcelasNum) || parcelasNum < 1) {
      vlog(F, "PagamentoModal.handleSubmit", "parcelas inválidas");
      setError("Parcelas deve ser um numero maior ou igual a 1.");
      return;
    }
    vlog(F, "PagamentoModal.handleSubmit", "convertendo valor");
    const valorNum = Number(valor);
    vlog(F, "PagamentoModal.handleSubmit", "validando valor:", valorNum);
    if (valor.trim() === "" || Number.isNaN(valorNum) || valorNum < 0) {
      vlog(F, "PagamentoModal.handleSubmit", "valor inválido");
      setError("Valor deve ser um numero maior ou igual a zero.");
      return;
    }
    vlog(F, "PagamentoModal.handleSubmit", "convertendo taxa");
    const taxaPctNum = Number(taxaPct);
    vlog(F, "PagamentoModal.handleSubmit", "validando taxa:", taxaPctNum);
    if (taxaPct.trim() === "" || Number.isNaN(taxaPctNum) || taxaPctNum < 0) {
      vlog(F, "PagamentoModal.handleSubmit", "taxa inválida");
      setError("Taxa (%) deve ser um numero maior ou igual a zero.");
      return;
    }
    vlog(F, "PagamentoModal.handleSubmit", "convertendo valor líquido");
    const valorLiquidoNum = Number(valorLiquido);
    vlog(F, "PagamentoModal.handleSubmit", "validando valor líquido:", valorLiquidoNum);
    if (
      valorLiquido.trim() === "" ||
      Number.isNaN(valorLiquidoNum) ||
      valorLiquidoNum < 0
    ) {
      vlog(F, "PagamentoModal.handleSubmit", "valor líquido inválido");
      setError("Valor liquido deve ser um numero maior ou igual a zero.");
      return;
    }
    vlog(F, "PagamentoModal.handleSubmit", "validando data de vencimento");
    if (!dataVencimento) {
      vlog(F, "PagamentoModal.handleSubmit", "data de vencimento ausente");
      setError("Data de vencimento e obrigatoria.");
      return;
    }

    vlog(F, "PagamentoModal.handleSubmit", "marcando envio em andamento");
    setSubmitting(true);
    try {
      vlog(F, "PagamentoModal.handleSubmit", "verificando modo do envio:", mode);
      if (mode === "create") {
        vlog(F, "PagamentoModal.handleSubmit", "montando payload de criação, pedido:", pedidoIdNum);
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
        vlog(F, "PagamentoModal.handleSubmit", "enviando criação do pagamento");
        await onSubmit(payload);
      } else {
        vlog(F, "PagamentoModal.handleSubmit", "montando payload de edição");
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
        vlog(F, "PagamentoModal.handleSubmit", "enviando edição do pagamento");
        await onSubmit(payload);
      }
    } catch (err) {
      vlog(F, "PagamentoModal.handleSubmit", "falha ao salvar pagamento: montando mensagem de erro");
      const message =
        err instanceof Error ? err.message : "Erro ao salvar pagamento.";
      vlog(F, "PagamentoModal.handleSubmit", "exibindo erro");
      setError(message);
    } finally {
      vlog(F, "PagamentoModal.handleSubmit", "finalizando envio");
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
