"use client";

import { useEffect, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Oportunidade, OportunidadeInput, Vendedor, ClienteResumo } from "@/lib/types";
import { apiListVendedores, apiListClientesDoVendedor } from "@/lib/api";

interface OportunidadeModalProps {
  open: boolean;
  mode: "create" | "edit";
  oportunidade: Oportunidade | null;
  onClose: () => void;
  onSubmit: (data: OportunidadeInput) => Promise<void>;
}

const ETAPA_OPTIONS = [
  { value: "Prospecção", label: "Prospecção" },
  { value: "Qualificação", label: "Qualificação" },
  { value: "Proposta enviada", label: "Proposta enviada" },
  { value: "Negociação", label: "Negociação" },
  { value: "Fechado ganho", label: "Fechado ganho" },
  { value: "Fechado perdido", label: "Fechado perdido" },
];

const ORIGEM_OPTIONS = [
  { value: "WhatsApp", label: "WhatsApp" },
  { value: "Indicação", label: "Indicação" },
  { value: "Inbound site", label: "Inbound site" },
  { value: "Instagram", label: "Instagram" },
  { value: "Feira de beleza", label: "Feira de beleza" },
  { value: "Reativação", label: "Reativação" },
  { value: "Prospecção ativa", label: "Prospecção ativa" },
];

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

export function OportunidadeModal({
  open,
  mode,
  oportunidade,
  onClose,
  onSubmit,
}: OportunidadeModalProps) {
  const [vendedorId, setVendedorId] = useState("");
  const [clienteId, setClienteId] = useState("");
  const [origem, setOrigem] = useState(ORIGEM_OPTIONS[0].value);
  const [dataAbertura, setDataAbertura] = useState("");
  const [etapa, setEtapa] = useState(ETAPA_OPTIONS[0].value);
  const [probabilidadePct, setProbabilidadePct] = useState("");
  const [valorEstimado, setValorEstimado] = useState("");
  const [dataFechamento, setDataFechamento] = useState("");
  const [cicloDias, setCicloDias] = useState("");
  const [motivoPerda, setMotivoPerda] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Listas auxiliares: vendedores (sempre) e clientes (cascata por vendedor).
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  const [loadingVendedores, setLoadingVendedores] = useState(false);
  const [vendedoresError, setVendedoresError] = useState<string | null>(null);

  const [clientes, setClientes] = useState<ClienteResumo[]>([]);
  const [loadingClientes, setLoadingClientes] = useState(false);
  const [clientesError, setClientesError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setError(null);
      setSubmitting(false);
      if (mode === "edit" && oportunidade) {
        setVendedorId(String(oportunidade.vendedor_id));
        setClienteId(String(oportunidade.cliente_id));
        setOrigem(oportunidade.origem);
        setDataAbertura(
          oportunidade.data_abertura ? oportunidade.data_abertura.slice(0, 10) : todayISO()
        );
        setEtapa(oportunidade.etapa);
        setProbabilidadePct(String(oportunidade.probabilidade_pct));
        setValorEstimado(String(oportunidade.valor_estimado));
        setDataFechamento(
          oportunidade.data_fechamento ? oportunidade.data_fechamento.slice(0, 10) : ""
        );
        setCicloDias(
          oportunidade.ciclo_dias !== null && oportunidade.ciclo_dias !== undefined
            ? String(oportunidade.ciclo_dias)
            : ""
        );
        setMotivoPerda(oportunidade.motivo_perda || "");
      } else {
        setVendedorId("");
        setClienteId("");
        setOrigem(ORIGEM_OPTIONS[0].value);
        setDataAbertura(todayISO());
        setEtapa(ETAPA_OPTIONS[0].value);
        setProbabilidadePct("");
        setValorEstimado("");
        setDataFechamento("");
        setCicloDias("");
        setMotivoPerda("");
      }
    }
  }, [open, mode, oportunidade]);

  // Carrega a lista de vendedores ao abrir o modal (mesmo padrao de outros
  // modais que usam apiListVendedores).
  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setLoadingVendedores(true);
    setVendedoresError(null);
    apiListVendedores()
      .then((res) => {
        if (!cancelled) setVendedores(res);
      })
      .catch((err) => {
        if (!cancelled) {
          const message =
            err instanceof Error ? err.message : "Erro ao carregar vendedores.";
          setVendedoresError(message);
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingVendedores(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open]);

  // Dropdown em cascata: sempre que o Vendedor selecionado mudar, recarrega
  // a lista de Clientes chamando GET /api/vendedores/{id}/clientes (carteira
  // ativa daquele vendedor). Enquanto nao houver vendedor escolhido, ou
  // enquanto a busca estiver em andamento, o select de Cliente fica vazio
  // e desabilitado.
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
        // Se o cliente selecionado anteriormente nao pertence mais a
        // carteira do novo vendedor, limpa a selecao.
        setClienteId((prev) =>
          prev && res.some((c) => String(c.id) === prev) ? prev : ""
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

  const vendedorOptions = useMemo(
    () => [
      { value: "", label: "Selecione um vendedor" },
      ...vendedores.map((v) => ({
        value: String(v.id),
        label: `#${v.id} - ${v.nome}${v.data_desligamento ? " [X]" : ""}`,
      })),
    ],
    [vendedores]
  );

  const clienteOptions = useMemo(
    () => [
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
    ],
    [clientes, vendedorId, loadingClientes]
  );

  const isFechadoPerdido = etapa === "Fechado perdido";

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!vendedorId) {
      setError("Vendedor e obrigatorio.");
      return;
    }
    if (!clienteId) {
      setError("Cliente e obrigatorio.");
      return;
    }
    if (!origem) {
      setError("Origem e obrigatoria.");
      return;
    }
    if (!etapa) {
      setError("Etapa e obrigatoria.");
      return;
    }

    const probabilidade = Number(probabilidadePct);
    if (!Number.isFinite(probabilidade) || probabilidade < 0 || probabilidade > 100) {
      setError("Probabilidade deve ser um numero entre 0 e 100.");
      return;
    }

    const valor = Number(valorEstimado);
    if (!Number.isFinite(valor) || valor < 0) {
      setError("Valor estimado deve ser um numero maior ou igual a zero.");
      return;
    }

    let ciclo: number | undefined;
    if (cicloDias.trim() !== "") {
      ciclo = Number(cicloDias);
      if (!Number.isFinite(ciclo) || ciclo < 0) {
        setError("Ciclo (dias) deve ser um numero maior ou igual a zero.");
        return;
      }
    }

    if (isFechadoPerdido && !motivoPerda.trim()) {
      setError("Motivo da perda e obrigatorio quando a etapa e Fechado perdido.");
      return;
    }

    setSubmitting(true);
    try {
      await onSubmit({
        cliente_id: Number(clienteId),
        vendedor_id: Number(vendedorId),
        origem,
        data_abertura: dataAbertura || undefined,
        etapa,
        probabilidade_pct: probabilidade,
        valor_estimado: valor,
        data_fechamento: dataFechamento || undefined,
        ciclo_dias: ciclo,
        motivo_perda: isFechadoPerdido ? motivoPerda.trim() : undefined,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao salvar oportunidade.";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Nova Oportunidade" : "Editar Oportunidade"}
      size="lg"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Alert variant="error">{error}</Alert>}
        {vendedoresError && (
          <Alert variant="error">
            Nao foi possivel carregar vendedores: {vendedoresError}
          </Alert>
        )}
        {clientesError && (
          <Alert variant="error">
            Nao foi possivel carregar clientes do vendedor: {clientesError}
          </Alert>
        )}

        <div className="grid grid-cols-2 gap-3">
          <Select
            label="Vendedor"
            value={vendedorId}
            onChange={(e) => setVendedorId(e.target.value)}
            options={vendedorOptions}
            disabled={loadingVendedores}
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

        <div className="grid grid-cols-2 gap-3">
          <Select
            label="Origem"
            value={origem}
            onChange={(e) => setOrigem(e.target.value)}
            options={ORIGEM_OPTIONS}
            required
          />
          <Select
            label="Etapa"
            value={etapa}
            onChange={(e) => setEtapa(e.target.value)}
            options={ETAPA_OPTIONS}
            required
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Input
            label="Data de abertura"
            type="date"
            value={dataAbertura}
            onChange={(e) => setDataAbertura(e.target.value)}
            helperText={mode === "create" ? "Se nao informada, sera usada a data de hoje." : undefined}
          />
          <Input
            label="Data de fechamento"
            type="date"
            value={dataFechamento}
            onChange={(e) => setDataFechamento(e.target.value)}
          />
        </div>

        <div className="grid grid-cols-3 gap-3">
          <Input
            label="Probabilidade (%)"
            type="number"
            min="0"
            max="100"
            step="1"
            value={probabilidadePct}
            onChange={(e) => setProbabilidadePct(e.target.value)}
            required
          />
          <Input
            label="Valor estimado"
            type="number"
            min="0"
            step="0.01"
            value={valorEstimado}
            onChange={(e) => setValorEstimado(e.target.value)}
            required
          />
          <Input
            label="Ciclo (dias)"
            type="number"
            min="0"
            step="1"
            value={cicloDias}
            onChange={(e) => setCicloDias(e.target.value)}
          />
        </div>

        {isFechadoPerdido && (
          <Input
            label="Motivo da perda"
            value={motivoPerda}
            onChange={(e) => setMotivoPerda(e.target.value)}
            placeholder="Ex: Preco, concorrencia, sem retorno..."
            required
          />
        )}

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button type="submit" loading={submitting} disabled={loadingVendedores}>
            {mode === "create" ? "Criar oportunidade" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
