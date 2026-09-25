"use client";

import { useEffect, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Visita, VisitaInput, Vendedor, ClienteResumo } from "@/lib/types";
import { useResetOnOpen } from "@/lib/useResetOnOpen";
import { apiListVendedores, apiListClientesDoVendedor } from "@/lib/api";

interface VisitaModalProps {
  open: boolean;
  mode: "create" | "edit";
  visita: Visita | null;
  onClose: () => void;
  onSubmit: (data: VisitaInput) => Promise<void>;
}

const RESULTADO_OPTIONS = [
  { value: "Sem pedido", label: "Sem pedido" },
  { value: "Pedido realizado", label: "Pedido realizado" },
  { value: "Reagendada", label: "Reagendada" },
  { value: "Cliente ausente", label: "Cliente ausente" },
  { value: "Apenas relacionamento", label: "Apenas relacionamento" },
];

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

export function VisitaModal({
  open,
  mode,
  visita,
  onClose,
  onSubmit,
}: VisitaModalProps) {
  const [vendedorId, setVendedorId] = useState("");
  const [clienteId, setClienteId] = useState("");
  const [dataVisita, setDataVisita] = useState("");
  const [resultado, setResultado] = useState(RESULTADO_OPTIONS[0].value);
  const [duracaoMin, setDuracaoMin] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Listas auxiliares: vendedores (sempre) e clientes (cascata por vendedor).
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  const [loadingVendedores, setLoadingVendedores] = useState(false);
  const [vendedoresError, setVendedoresError] = useState<string | null>(null);

  const [clientes, setClientes] = useState<ClienteResumo[]>([]);
  const [loadingClientes, setLoadingClientes] = useState(false);
  const [clientesError, setClientesError] = useState<string | null>(null);

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  useResetOnOpen(open, [mode, visita], () => {
    setError(null);
    setSubmitting(false);
    if (mode === "edit" && visita) {
      setVendedorId(String(visita.vendedor_id));
      setClienteId(String(visita.cliente_id));
      setDataVisita(
        visita.data_visita ? visita.data_visita.slice(0, 10) : todayISO()
      );
      setResultado(visita.resultado);
      setDuracaoMin(String(visita.duracao_min));
    } else {
      setVendedorId("");
      setClienteId("");
      setDataVisita(todayISO());
      setResultado(RESULTADO_OPTIONS[0].value);
      setDuracaoMin("");
    }
  });

  // Carrega a lista de vendedores ao abrir o modal (mesmo padrao de
  // OportunidadeModal).
  // Parte sincrona (liga o loading / limpa o erro) roda durante o render
  // ao abrir; o efeito so busca e aplica o resultado nos callbacks.
  useResetOnOpen(open, [], () => {
    setLoadingVendedores(true);
    setVendedoresError(null);
  });

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
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
  // Parte sincrona roda durante o render quando o modal abre ou o
  // vendedor muda (useResetOnOpen); o efeito so busca e aplica o
  // resultado nos callbacks assincronos.
  useResetOnOpen(open, [vendedorId], () => {
    setClientesError(null);
    if (!vendedorId) {
      setClientes([]);
      setLoadingClientes(false);
      return;
    }
    setLoadingClientes(true);
  });

  useEffect(() => {
    if (!open || !vendedorId) return;
    let cancelled = false;
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
    if (!dataVisita) {
      setError("Data da visita e obrigatoria.");
      return;
    }
    if (!resultado) {
      setError("Resultado e obrigatorio.");
      return;
    }

    const duracao = duracaoMin.trim() === "" ? 0 : Number(duracaoMin);
    if (!Number.isFinite(duracao) || duracao < 0) {
      setError("Duracao (min) deve ser um numero maior ou igual a zero.");
      return;
    }

    setSubmitting(true);
    try {
      await onSubmit({
        cliente_id: Number(clienteId),
        vendedor_id: Number(vendedorId),
        data_visita: dataVisita,
        resultado,
        duracao_min: duracao,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Erro ao salvar visita.";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Nova Visita" : "Editar Visita"}
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
          <Input
            label="Data da visita"
            type="date"
            value={dataVisita}
            onChange={(e) => setDataVisita(e.target.value)}
            required
          />
          <Select
            label="Resultado"
            value={resultado}
            onChange={(e) => setResultado(e.target.value)}
            options={RESULTADO_OPTIONS}
            required
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Input
            label="Duracao (min)"
            type="number"
            min="0"
            step="1"
            value={duracaoMin}
            onChange={(e) => setDuracaoMin(e.target.value)}
            required
          />
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button type="submit" loading={submitting} disabled={loadingVendedores}>
            {mode === "create" ? "Criar visita" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
