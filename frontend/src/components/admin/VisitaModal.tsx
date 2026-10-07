"use client";

import { useEffect, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Visita, VisitaInput, Vendedor, ClienteResumo } from "@/lib/types";
import { useAjustarAoMudar, useResetOnOpen } from "@/lib/useResetOnOpen";
import { useVendedorTravado, vendedorInicial } from "@/lib/useVendedorTravado";
import { apiListVendedores, apiListClientesDoVendedor } from "@/lib/api";
import { vlog } from "@/lib/vlog";

const F = "VisitaModal.tsx";

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
  vlog(F, "VisitaModal", "criando estado vendedorId, modo:", mode);
  const [vendedorId, setVendedorId] = useState("");
  vlog(F, "VisitaModal", "criando estado clienteId");
  const [clienteId, setClienteId] = useState("");
  vlog(F, "VisitaModal", "criando estado dataVisita");
  const [dataVisita, setDataVisita] = useState("");
  vlog(F, "VisitaModal", "criando estado resultado");
  const [resultado, setResultado] = useState(RESULTADO_OPTIONS[0].value);
  vlog(F, "VisitaModal", "criando estado duracaoMin");
  const [duracaoMin, setDuracaoMin] = useState("");
  vlog(F, "VisitaModal", "criando estado submitting");
  const [submitting, setSubmitting] = useState(false);
  vlog(F, "VisitaModal", "criando estado error");
  const [error, setError] = useState<string | null>(null);

  // Listas auxiliares: vendedores (sempre) e clientes (cascata por vendedor).
  vlog(F, "VisitaModal", "criando estado vendedores");
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  vlog(F, "VisitaModal", "criando estado loadingVendedores");
  const [loadingVendedores, setLoadingVendedores] = useState(false);
  vlog(F, "VisitaModal", "criando estado vendedoresError");
  const [vendedoresError, setVendedoresError] = useState<string | null>(null);

  vlog(F, "VisitaModal", "criando estado clientes");
  const [clientes, setClientes] = useState<ClienteResumo[]>([]);
  vlog(F, "VisitaModal", "criando estado loadingClientes");
  const [loadingClientes, setLoadingClientes] = useState(false);
  vlog(F, "VisitaModal", "criando estado clientesError");
  const [clientesError, setClientesError] = useState<string | null>(null);

  // SEC-03: usuario normal tem o vendedor travado no proprio id_vendedor da
  // sessao (/me). GET /api/vendedores devolve so ele ([] sem vinculo).
  vlog(F, "VisitaModal", "obtendo vendedor travado da sessão");
  const travado = useVendedorTravado();
  vlog(F, "VisitaModal", "desestruturando vendedor travado");
  const { normal, vendedorTravadoId, vendedorTravadoNome, semVendedor } = travado;

  // Mantem o vendedor travado sincronizado com a sessao (usuario normal).
  vlog(F, "VisitaModal", "registrando sincronização do vendedor travado");
  useAjustarAoMudar([open, normal, vendedorTravadoId], () => {
    vlog(F, "VisitaModal.ajustarVendedor", "verificando se modal aberto e usuário normal:", open, normal);
    if (open && normal) {
      vlog(F, "VisitaModal.ajustarVendedor", "travando vendedor da sessão, id:", vendedorTravadoId);
      setVendedorId(vendedorTravadoId ? String(vendedorTravadoId) : "");
    }
  });

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  vlog(F, "VisitaModal", "registrando reset do formulário ao abrir");
  useResetOnOpen(open, [mode, visita], () => {
    vlog(F, "VisitaModal.reset", "limpando erro");
    setError(null);
    vlog(F, "VisitaModal.reset", "limpando estado de envio");
    setSubmitting(false);
    vlog(F, "VisitaModal.reset", "verificando se é edição com visita, id:", visita?.visita_id);
    if (mode === "edit" && visita) {
      vlog(F, "VisitaModal.reset", "preenchendo vendedor, id:", visita.vendedor_id);
      setVendedorId(vendedorInicial(travado, String(visita.vendedor_id)));
      vlog(F, "VisitaModal.reset", "preenchendo cliente, id:", visita.cliente_id);
      setClienteId(String(visita.cliente_id));
      vlog(F, "VisitaModal.reset", "preenchendo data da visita");
      setDataVisita(
        visita.data_visita ? visita.data_visita.slice(0, 10) : todayISO()
      );
      vlog(F, "VisitaModal.reset", "preenchendo resultado");
      setResultado(visita.resultado);
      vlog(F, "VisitaModal.reset", "preenchendo duração");
      setDuracaoMin(String(visita.duracao_min));
    } else {
      vlog(F, "VisitaModal.reset", "definindo vendedor inicial para criação");
      setVendedorId(vendedorInicial(travado, ""));
      vlog(F, "VisitaModal.reset", "limpando cliente");
      setClienteId("");
      vlog(F, "VisitaModal.reset", "definindo data da visita como hoje");
      setDataVisita(todayISO());
      vlog(F, "VisitaModal.reset", "definindo resultado padrão");
      setResultado(RESULTADO_OPTIONS[0].value);
      vlog(F, "VisitaModal.reset", "limpando duração");
      setDuracaoMin("");
    }
  });

  // Carrega a lista de vendedores ao abrir o modal (mesmo padrao de
  // OportunidadeModal).
  // Parte sincrona (liga o loading / limpa o erro) roda durante o render
  // ao abrir; o efeito so busca e aplica o resultado nos callbacks.
  vlog(F, "VisitaModal", "registrando reset do carregamento de vendedores");
  useResetOnOpen(open, [], () => {
    vlog(F, "VisitaModal.resetVendedores", "ligando loading de vendedores");
    setLoadingVendedores(true);
    vlog(F, "VisitaModal.resetVendedores", "limpando erro de vendedores");
    setVendedoresError(null);
  });

  vlog(F, "VisitaModal", "registrando efeito de carga de vendedores");
  useEffect(() => {
    vlog(F, "VisitaModal.useEffect", "verificando se o modal está aberto (vendedores):", open);
    if (!open) return;
    vlog(F, "VisitaModal.useEffect", "inicializando flag de cancelamento (vendedores)");
    let cancelled = false;
    vlog(F, "VisitaModal.useEffect", "buscando vendedores");
    apiListVendedores()
      .then((res) => {
        vlog(F, "VisitaModal.useEffect", "vendedores recebidos, qtd/cancelado:", res.length, cancelled);
        if (!cancelled) setVendedores(res);
      })
      .catch((err) => {
        vlog(F, "VisitaModal.useEffect", "falha ao buscar vendedores, cancelado:", cancelled);
        if (!cancelled) {
          vlog(F, "VisitaModal.useEffect", "montando mensagem de erro de vendedores");
          const message =
            err instanceof Error ? err.message : "Erro ao carregar vendedores.";
          vlog(F, "VisitaModal.useEffect", "exibindo erro de vendedores");
          setVendedoresError(message);
        }
      })
      .finally(() => {
        vlog(F, "VisitaModal.useEffect", "desligando loading de vendedores se não cancelado");
        if (!cancelled) setLoadingVendedores(false);
      });
    return () => {
      vlog(F, "VisitaModal.useEffect", "cleanup: cancelando carga de vendedores");
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
  vlog(F, "VisitaModal", "registrando reset da cascata de clientes");
  useResetOnOpen(open, [vendedorId], () => {
    vlog(F, "VisitaModal.resetClientes", "limpando erro de clientes");
    setClientesError(null);
    vlog(F, "VisitaModal.resetClientes", "verificando se há vendedor selecionado:", !!vendedorId);
    if (!vendedorId) {
      vlog(F, "VisitaModal.resetClientes", "limpando lista de clientes");
      setClientes([]);
      vlog(F, "VisitaModal.resetClientes", "desligando loading de clientes");
      setLoadingClientes(false);
      return;
    }
    vlog(F, "VisitaModal.resetClientes", "ligando loading de clientes");
    setLoadingClientes(true);
  });

  vlog(F, "VisitaModal", "registrando efeito de carga de clientes do vendedor");
  useEffect(() => {
    vlog(F, "VisitaModal.useEffect", "verificando modal aberto e vendedor selecionado:", open, vendedorId);
    if (!open || !vendedorId) return;
    vlog(F, "VisitaModal.useEffect", "inicializando flag de cancelamento (clientes)");
    let cancelled = false;
    vlog(F, "VisitaModal.useEffect", "buscando clientes do vendedor, id:", vendedorId);
    apiListClientesDoVendedor(Number(vendedorId))
      .then((res) => {
        vlog(F, "VisitaModal.useEffect", "clientes recebidos, qtd/cancelado:", res.length, cancelled);
        if (cancelled) return;
        vlog(F, "VisitaModal.useEffect", "aplicando lista de clientes");
        setClientes(res);
        // Se o cliente selecionado anteriormente nao pertence mais a
        // carteira do novo vendedor, limpa a selecao.
        vlog(F, "VisitaModal.useEffect", "validando cliente selecionado na nova carteira");
        setClienteId((prev) =>
          prev && res.some((c) => String(c.id) === prev) ? prev : ""
        );
      })
      .catch((err) => {
        vlog(F, "VisitaModal.useEffect", "falha ao buscar clientes, cancelado:", cancelled);
        if (!cancelled) {
          vlog(F, "VisitaModal.useEffect", "montando mensagem de erro de clientes");
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar clientes do vendedor.";
          vlog(F, "VisitaModal.useEffect", "exibindo erro de clientes");
          setClientesError(message);
          vlog(F, "VisitaModal.useEffect", "limpando lista de clientes");
          setClientes([]);
        }
      })
      .finally(() => {
        vlog(F, "VisitaModal.useEffect", "desligando loading de clientes se não cancelado");
        if (!cancelled) setLoadingClientes(false);
      });
    return () => {
      vlog(F, "VisitaModal.useEffect", "cleanup: cancelando carga de clientes");
      cancelled = true;
    };
  }, [open, vendedorId]);

  vlog(F, "VisitaModal", "memorizando opções de vendedor");
  const vendedorOptions = useMemo(() => {
    vlog(F, "VisitaModal.vendedorOptions", "montando opções de vendedor, qtd:", vendedores.length);
    const opts = [
      { value: "", label: semVendedor ? "Sem vendedor vinculado" : "Selecione um vendedor" },
      ...vendedores.map((v) => ({
        value: String(v.id),
        label: `#${v.id} - ${v.nome}${v.data_desligamento ? " [X]" : ""}`,
      })),
    ];
    // Usuario normal: o proprio vendedor aparece no select travado mesmo
    // antes da lista (escopada) carregar.
    vlog(F, "VisitaModal.vendedorOptions", "verificando se o vendedor travado precisa ser incluído:", vendedorTravadoId);
    if (vendedorTravadoId && !vendedores.some((v) => v.id === vendedorTravadoId)) {
      vlog(F, "VisitaModal.vendedorOptions", "incluindo vendedor travado nas opções");
      opts.push({
        value: String(vendedorTravadoId),
        label: `#${vendedorTravadoId} - ${vendedorTravadoNome ?? "Meu vendedor"}`,
      });
    }
    return opts;
  }, [vendedores, semVendedor, vendedorTravadoId, vendedorTravadoNome]);

  vlog(F, "VisitaModal", "memorizando opções de cliente");
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
    vlog(F, "VisitaModal.handleSubmit", "impedindo submit padrão do form");
    e.preventDefault();
    vlog(F, "VisitaModal.handleSubmit", "limpando erro");
    setError(null);

    vlog(F, "VisitaModal.handleSubmit", "verificando se usuário está sem vendedor:", semVendedor);
    if (semVendedor) {
      vlog(F, "VisitaModal.handleSubmit", "usuário sem vendedor vinculado");
      setError(
        "Seu usuario nao esta vinculado a um vendedor. Solicite ao administrador o vinculo para registrar visitas."
      );
      return;
    }
    vlog(F, "VisitaModal.handleSubmit", "validando vendedor");
    if (!vendedorId) {
      vlog(F, "VisitaModal.handleSubmit", "vendedor ausente");
      setError("Vendedor e obrigatorio.");
      return;
    }
    vlog(F, "VisitaModal.handleSubmit", "validando cliente");
    if (!clienteId) {
      vlog(F, "VisitaModal.handleSubmit", "cliente ausente");
      setError("Cliente e obrigatorio.");
      return;
    }
    vlog(F, "VisitaModal.handleSubmit", "validando data da visita");
    if (!dataVisita) {
      vlog(F, "VisitaModal.handleSubmit", "data da visita ausente");
      setError("Data da visita e obrigatoria.");
      return;
    }
    vlog(F, "VisitaModal.handleSubmit", "validando resultado");
    if (!resultado) {
      vlog(F, "VisitaModal.handleSubmit", "resultado ausente");
      setError("Resultado e obrigatorio.");
      return;
    }

    vlog(F, "VisitaModal.handleSubmit", "convertendo duração para número");
    const duracao = duracaoMin.trim() === "" ? 0 : Number(duracaoMin);
    vlog(F, "VisitaModal.handleSubmit", "validando duração:", duracao);
    if (!Number.isFinite(duracao) || duracao < 0) {
      vlog(F, "VisitaModal.handleSubmit", "duração inválida");
      setError("Duracao (min) deve ser um numero maior ou igual a zero.");
      return;
    }

    vlog(F, "VisitaModal.handleSubmit", "marcando envio em andamento");
    setSubmitting(true);
    try {
      vlog(F, "VisitaModal.handleSubmit", "enviando visita, cliente/vendedor:", Number(clienteId), Number(vendedorId));
      await onSubmit({
        cliente_id: Number(clienteId),
        vendedor_id: Number(vendedorId),
        data_visita: dataVisita,
        resultado,
        duracao_min: duracao,
      });
    } catch (err) {
      vlog(F, "VisitaModal.handleSubmit", "falha ao salvar visita: montando mensagem de erro");
      const message = err instanceof Error ? err.message : "Erro ao salvar visita.";
      vlog(F, "VisitaModal.handleSubmit", "exibindo erro");
      setError(message);
    } finally {
      vlog(F, "VisitaModal.handleSubmit", "finalizando envio");
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
        {semVendedor && (
          <Alert variant="warning">
            Seu usuario nao esta vinculado a um vendedor, por isso nao e
            possivel registrar visitas. Solicite o vinculo ao administrador.
          </Alert>
        )}

        <div className="grid grid-cols-2 gap-3">
          <Select
            label="Vendedor"
            value={vendedorId}
            onChange={(e) => setVendedorId(e.target.value)}
            options={vendedorOptions}
            disabled={loadingVendedores || normal}
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
          <Button type="submit" loading={submitting} disabled={loadingVendedores || semVendedor}>
            {mode === "create" ? "Criar visita" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
