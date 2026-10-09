"use client";

import { useEffect, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Oportunidade, OportunidadeInput, Vendedor, ClienteResumo } from "@/lib/types";
import { useAjustarAoMudar, useResetOnOpen } from "@/lib/useResetOnOpen";
import { useVendedorTravado, vendedorInicial } from "@/lib/useVendedorTravado";
import { apiListVendedores, apiListClientesDoVendedor } from "@/lib/api";
import { vlog } from "@/lib/vlog";

const F = "OportunidadeModal.tsx";

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
  vlog(F, "OportunidadeModal", "criando estado vendedorId, modo:", mode);
  const [vendedorId, setVendedorId] = useState("");
  vlog(F, "OportunidadeModal", "criando estado clienteId");
  const [clienteId, setClienteId] = useState("");
  vlog(F, "OportunidadeModal", "criando estado origem");
  const [origem, setOrigem] = useState(ORIGEM_OPTIONS[0].value);
  vlog(F, "OportunidadeModal", "criando estado dataAbertura");
  const [dataAbertura, setDataAbertura] = useState("");
  vlog(F, "OportunidadeModal", "criando estado etapa");
  const [etapa, setEtapa] = useState(ETAPA_OPTIONS[0].value);
  vlog(F, "OportunidadeModal", "criando estado probabilidadePct");
  const [probabilidadePct, setProbabilidadePct] = useState("");
  vlog(F, "OportunidadeModal", "criando estado valorEstimado");
  const [valorEstimado, setValorEstimado] = useState("");
  vlog(F, "OportunidadeModal", "criando estado dataFechamento");
  const [dataFechamento, setDataFechamento] = useState("");
  vlog(F, "OportunidadeModal", "criando estado cicloDias");
  const [cicloDias, setCicloDias] = useState("");
  vlog(F, "OportunidadeModal", "criando estado motivoPerda");
  const [motivoPerda, setMotivoPerda] = useState("");
  vlog(F, "OportunidadeModal", "criando estado submitting");
  const [submitting, setSubmitting] = useState(false);
  vlog(F, "OportunidadeModal", "criando estado error");
  const [error, setError] = useState<string | null>(null);

  // Listas auxiliares: vendedores (sempre) e clientes (cascata por vendedor).
  vlog(F, "OportunidadeModal", "criando estado vendedores");
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  vlog(F, "OportunidadeModal", "criando estado loadingVendedores");
  const [loadingVendedores, setLoadingVendedores] = useState(false);
  vlog(F, "OportunidadeModal", "criando estado vendedoresError");
  const [vendedoresError, setVendedoresError] = useState<string | null>(null);

  vlog(F, "OportunidadeModal", "criando estado clientes");
  const [clientes, setClientes] = useState<ClienteResumo[]>([]);
  vlog(F, "OportunidadeModal", "criando estado loadingClientes");
  const [loadingClientes, setLoadingClientes] = useState(false);
  vlog(F, "OportunidadeModal", "criando estado clientesError");
  const [clientesError, setClientesError] = useState<string | null>(null);

  // SEC-03: usuario normal tem o vendedor travado no proprio id_vendedor da
  // sessao (/me). GET /api/vendedores devolve so ele ([] sem vinculo).
  vlog(F, "OportunidadeModal", "obtendo vendedor travado da sessão");
  const travado = useVendedorTravado();
  vlog(F, "OportunidadeModal", "desestruturando vendedor travado");
  const { normal, vendedorTravadoId, vendedorTravadoNome, semVendedor } = travado;

  // Mantem o vendedor travado sincronizado com a sessao (usuario normal).
  vlog(F, "OportunidadeModal", "registrando sincronização do vendedor travado");
  useAjustarAoMudar([open, normal, vendedorTravadoId], () => {
    vlog(F, "OportunidadeModal.ajustarVendedor", "verificando se modal aberto e usuário normal:", open, normal);
    if (open && normal) {
      vlog(F, "OportunidadeModal.ajustarVendedor", "travando vendedor da sessão, id:", vendedorTravadoId);
      setVendedorId(vendedorTravadoId ? String(vendedorTravadoId) : "");
    }
  });

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  vlog(F, "OportunidadeModal", "registrando reset do formulário ao abrir");
  useResetOnOpen(open, [mode, oportunidade], () => {
    vlog(F, "OportunidadeModal.reset", "limpando erro");
    setError(null);
    vlog(F, "OportunidadeModal.reset", "limpando estado de envio");
    setSubmitting(false);
    vlog(F, "OportunidadeModal.reset", "verificando se é edição com oportunidade, id:", oportunidade?.oportunidade_id);
    if (mode === "edit" && oportunidade) {
      vlog(F, "OportunidadeModal.reset", "preenchendo vendedor, id:", oportunidade.vendedor_id);
      setVendedorId(vendedorInicial(travado, String(oportunidade.vendedor_id)));
      vlog(F, "OportunidadeModal.reset", "preenchendo cliente, id:", oportunidade.cliente_id);
      setClienteId(String(oportunidade.cliente_id));
      vlog(F, "OportunidadeModal.reset", "preenchendo origem");
      setOrigem(oportunidade.origem);
      vlog(F, "OportunidadeModal.reset", "preenchendo data de abertura");
      setDataAbertura(
        oportunidade.data_abertura ? oportunidade.data_abertura.slice(0, 10) : todayISO()
      );
      vlog(F, "OportunidadeModal.reset", "preenchendo etapa");
      setEtapa(oportunidade.etapa);
      vlog(F, "OportunidadeModal.reset", "preenchendo probabilidade");
      setProbabilidadePct(String(oportunidade.probabilidade_pct));
      vlog(F, "OportunidadeModal.reset", "preenchendo valor estimado");
      setValorEstimado(String(oportunidade.valor_estimado));
      vlog(F, "OportunidadeModal.reset", "preenchendo data de fechamento");
      setDataFechamento(
        oportunidade.data_fechamento ? oportunidade.data_fechamento.slice(0, 10) : ""
      );
      vlog(F, "OportunidadeModal.reset", "preenchendo ciclo em dias");
      setCicloDias(
        oportunidade.ciclo_dias !== null && oportunidade.ciclo_dias !== undefined
          ? String(oportunidade.ciclo_dias)
          : ""
      );
      vlog(F, "OportunidadeModal.reset", "preenchendo motivo da perda");
      setMotivoPerda(oportunidade.motivo_perda || "");
    } else {
      vlog(F, "OportunidadeModal.reset", "definindo vendedor inicial para criação");
      setVendedorId(vendedorInicial(travado, ""));
      vlog(F, "OportunidadeModal.reset", "limpando cliente");
      setClienteId("");
      vlog(F, "OportunidadeModal.reset", "definindo origem padrão");
      setOrigem(ORIGEM_OPTIONS[0].value);
      vlog(F, "OportunidadeModal.reset", "definindo data de abertura como hoje");
      setDataAbertura(todayISO());
      vlog(F, "OportunidadeModal.reset", "definindo etapa padrão");
      setEtapa(ETAPA_OPTIONS[0].value);
      vlog(F, "OportunidadeModal.reset", "limpando probabilidade");
      setProbabilidadePct("");
      vlog(F, "OportunidadeModal.reset", "limpando valor estimado");
      setValorEstimado("");
      vlog(F, "OportunidadeModal.reset", "limpando data de fechamento");
      setDataFechamento("");
      vlog(F, "OportunidadeModal.reset", "limpando ciclo em dias");
      setCicloDias("");
      vlog(F, "OportunidadeModal.reset", "limpando motivo da perda");
      setMotivoPerda("");
    }
  });

  // Carrega a lista de vendedores ao abrir o modal (mesmo padrao de outros
  // modais que usam apiListVendedores).
  // Parte sincrona (liga o loading / limpa o erro) roda durante o render
  // ao abrir; o efeito so busca e aplica o resultado nos callbacks.
  vlog(F, "OportunidadeModal", "registrando reset do carregamento de vendedores");
  useResetOnOpen(open, [], () => {
    vlog(F, "OportunidadeModal.resetVendedores", "ligando loading de vendedores");
    setLoadingVendedores(true);
    vlog(F, "OportunidadeModal.resetVendedores", "limpando erro de vendedores");
    setVendedoresError(null);
  });

  vlog(F, "OportunidadeModal", "registrando efeito de carga de vendedores");
  useEffect(() => {
    vlog(F, "OportunidadeModal.useEffect", "verificando se o modal está aberto (vendedores):", open);
    if (!open) return;
    vlog(F, "OportunidadeModal.useEffect", "inicializando flag de cancelamento (vendedores)");
    let cancelled = false;
    vlog(F, "OportunidadeModal.useEffect", "buscando vendedores");
    apiListVendedores()
      .then((res) => {
        vlog(F, "OportunidadeModal.useEffect", "vendedores recebidos, qtd/cancelado:", res.length, cancelled);
        if (!cancelled) setVendedores(res);
      })
      .catch((err) => {
        vlog(F, "OportunidadeModal.useEffect", "falha ao buscar vendedores, cancelado:", cancelled);
        if (!cancelled) {
          vlog(F, "OportunidadeModal.useEffect", "montando mensagem de erro de vendedores");
          const message =
            err instanceof Error ? err.message : "Erro ao carregar vendedores.";
          vlog(F, "OportunidadeModal.useEffect", "exibindo erro de vendedores");
          setVendedoresError(message);
        }
      })
      .finally(() => {
        vlog(F, "OportunidadeModal.useEffect", "desligando loading de vendedores se não cancelado");
        if (!cancelled) setLoadingVendedores(false);
      });
    return () => {
      vlog(F, "OportunidadeModal.useEffect", "cleanup: cancelando carga de vendedores");
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
  vlog(F, "OportunidadeModal", "registrando reset da cascata de clientes");
  useResetOnOpen(open, [vendedorId], () => {
    vlog(F, "OportunidadeModal.resetClientes", "limpando erro de clientes");
    setClientesError(null);
    vlog(F, "OportunidadeModal.resetClientes", "verificando se há vendedor selecionado:", !!vendedorId);
    if (!vendedorId) {
      vlog(F, "OportunidadeModal.resetClientes", "limpando lista de clientes");
      setClientes([]);
      vlog(F, "OportunidadeModal.resetClientes", "desligando loading de clientes");
      setLoadingClientes(false);
      return;
    }
    vlog(F, "OportunidadeModal.resetClientes", "ligando loading de clientes");
    setLoadingClientes(true);
  });

  vlog(F, "OportunidadeModal", "registrando efeito de carga de clientes do vendedor");
  useEffect(() => {
    vlog(F, "OportunidadeModal.useEffect", "verificando modal aberto e vendedor selecionado:", open, vendedorId);
    if (!open || !vendedorId) return;
    vlog(F, "OportunidadeModal.useEffect", "inicializando flag de cancelamento (clientes)");
    let cancelled = false;
    vlog(F, "OportunidadeModal.useEffect", "buscando clientes do vendedor, id:", vendedorId);
    apiListClientesDoVendedor(Number(vendedorId))
      .then((res) => {
        vlog(F, "OportunidadeModal.useEffect", "clientes recebidos, qtd/cancelado:", res.length, cancelled);
        if (cancelled) return;
        vlog(F, "OportunidadeModal.useEffect", "aplicando lista de clientes");
        setClientes(res);
        // Se o cliente selecionado anteriormente nao pertence mais a
        // carteira do novo vendedor, limpa a selecao.
        vlog(F, "OportunidadeModal.useEffect", "validando cliente selecionado na nova carteira");
        setClienteId((prev) =>
          prev && res.some((c) => String(c.id) === prev) ? prev : ""
        );
      })
      .catch((err) => {
        vlog(F, "OportunidadeModal.useEffect", "falha ao buscar clientes, cancelado:", cancelled);
        if (!cancelled) {
          vlog(F, "OportunidadeModal.useEffect", "montando mensagem de erro de clientes");
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar clientes do vendedor.";
          vlog(F, "OportunidadeModal.useEffect", "exibindo erro de clientes");
          setClientesError(message);
          vlog(F, "OportunidadeModal.useEffect", "limpando lista de clientes");
          setClientes([]);
        }
      })
      .finally(() => {
        vlog(F, "OportunidadeModal.useEffect", "desligando loading de clientes se não cancelado");
        if (!cancelled) setLoadingClientes(false);
      });
    return () => {
      vlog(F, "OportunidadeModal.useEffect", "cleanup: cancelando carga de clientes");
      cancelled = true;
    };
  }, [open, vendedorId]);

  vlog(F, "OportunidadeModal", "memorizando opções de vendedor");
  const vendedorOptions = useMemo(() => {
    vlog(F, "OportunidadeModal.vendedorOptions", "montando opções de vendedor, qtd:", vendedores.length);
    const opts = [
      { value: "", label: semVendedor ? "Sem vendedor vinculado" : "Selecione um vendedor" },
      ...vendedores.map((v) => ({
        value: String(v.id),
        label: `#${v.id} - ${v.nome}${v.data_desligamento ? " [X]" : ""}`,
      })),
    ];
    // Usuario normal: o proprio vendedor aparece no select travado mesmo
    // antes da lista (escopada) carregar.
    vlog(F, "OportunidadeModal.vendedorOptions", "verificando se o vendedor travado precisa ser incluído:", vendedorTravadoId);
    if (vendedorTravadoId && !vendedores.some((v) => v.id === vendedorTravadoId)) {
      vlog(F, "OportunidadeModal.vendedorOptions", "incluindo vendedor travado nas opções");
      opts.push({
        value: String(vendedorTravadoId),
        label: `#${vendedorTravadoId} - ${vendedorTravadoNome ?? "Meu vendedor"}`,
      });
    }
    return opts;
  }, [vendedores, semVendedor, vendedorTravadoId, vendedorTravadoNome]);

  vlog(F, "OportunidadeModal", "memorizando opções de cliente");
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

  vlog(F, "OportunidadeModal", "calculando se a etapa é Fechado perdido");
  const isFechadoPerdido = etapa === "Fechado perdido";

  const handleSubmit = async (e: React.FormEvent) => {
    vlog(F, "OportunidadeModal.handleSubmit", "impedindo submit padrão do form");
    e.preventDefault();
    vlog(F, "OportunidadeModal.handleSubmit", "limpando erro");
    setError(null);

    vlog(F, "OportunidadeModal.handleSubmit", "verificando se usuário está sem vendedor:", semVendedor);
    if (semVendedor) {
      vlog(F, "OportunidadeModal.handleSubmit", "usuário sem vendedor vinculado");
      setError(
        "Seu usuario nao esta vinculado a um vendedor. Solicite ao administrador o vinculo para registrar oportunidades."
      );
      return;
    }
    vlog(F, "OportunidadeModal.handleSubmit", "validando vendedor");
    if (!vendedorId) {
      vlog(F, "OportunidadeModal.handleSubmit", "vendedor ausente");
      setError("Vendedor e obrigatorio.");
      return;
    }
    vlog(F, "OportunidadeModal.handleSubmit", "validando cliente");
    if (!clienteId) {
      vlog(F, "OportunidadeModal.handleSubmit", "cliente ausente");
      setError("Cliente e obrigatorio.");
      return;
    }
    vlog(F, "OportunidadeModal.handleSubmit", "validando origem");
    if (!origem) {
      vlog(F, "OportunidadeModal.handleSubmit", "origem ausente");
      setError("Origem e obrigatoria.");
      return;
    }
    vlog(F, "OportunidadeModal.handleSubmit", "validando etapa");
    if (!etapa) {
      vlog(F, "OportunidadeModal.handleSubmit", "etapa ausente");
      setError("Etapa e obrigatoria.");
      return;
    }

    vlog(F, "OportunidadeModal.handleSubmit", "convertendo probabilidade para número");
    const probabilidade = Number(probabilidadePct);
    vlog(F, "OportunidadeModal.handleSubmit", "validando probabilidade:", probabilidade);
    if (!Number.isFinite(probabilidade) || probabilidade < 0 || probabilidade > 100) {
      vlog(F, "OportunidadeModal.handleSubmit", "probabilidade inválida");
      setError("Probabilidade deve ser um numero entre 0 e 100.");
      return;
    }

    vlog(F, "OportunidadeModal.handleSubmit", "convertendo valor estimado para número");
    const valor = Number(valorEstimado);
    vlog(F, "OportunidadeModal.handleSubmit", "validando valor estimado:", valor);
    if (!Number.isFinite(valor) || valor < 0) {
      vlog(F, "OportunidadeModal.handleSubmit", "valor estimado inválido");
      setError("Valor estimado deve ser um numero maior ou igual a zero.");
      return;
    }

    vlog(F, "OportunidadeModal.handleSubmit", "declarando ciclo em dias");
    let ciclo: number | undefined;
    vlog(F, "OportunidadeModal.handleSubmit", "verificando se ciclo em dias foi informado");
    if (cicloDias.trim() !== "") {
      vlog(F, "OportunidadeModal.handleSubmit", "convertendo ciclo em dias para número");
      ciclo = Number(cicloDias);
      vlog(F, "OportunidadeModal.handleSubmit", "validando ciclo em dias:", ciclo);
      if (!Number.isFinite(ciclo) || ciclo < 0) {
        vlog(F, "OportunidadeModal.handleSubmit", "ciclo em dias inválido");
        setError("Ciclo (dias) deve ser um numero maior ou igual a zero.");
        return;
      }
    }

    vlog(F, "OportunidadeModal.handleSubmit", "validando motivo da perda, fechado perdido:", isFechadoPerdido);
    if (isFechadoPerdido && !motivoPerda.trim()) {
      vlog(F, "OportunidadeModal.handleSubmit", "motivo da perda ausente");
      setError("Motivo da perda e obrigatorio quando a etapa e Fechado perdido.");
      return;
    }

    vlog(F, "OportunidadeModal.handleSubmit", "marcando envio em andamento");
    setSubmitting(true);
    try {
      vlog(F, "OportunidadeModal.handleSubmit", "enviando oportunidade, cliente/vendedor:", Number(clienteId), Number(vendedorId));
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
      vlog(F, "OportunidadeModal.handleSubmit", "falha ao salvar oportunidade: montando mensagem de erro");
      const message =
        err instanceof Error ? err.message : "Erro ao salvar oportunidade.";
      vlog(F, "OportunidadeModal.handleSubmit", "exibindo erro");
      setError(message);
    } finally {
      vlog(F, "OportunidadeModal.handleSubmit", "finalizando envio");
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
        {semVendedor && (
          <Alert variant="warning">
            Seu usuario nao esta vinculado a um vendedor, por isso nao e
            possivel registrar oportunidades. Solicite o vinculo ao administrador.
          </Alert>
        )}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
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

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
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

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
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

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
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

        <div className="flex flex-wrap justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button type="submit" loading={submitting} disabled={loadingVendedores || semVendedor}>
            {mode === "create" ? "Criar oportunidade" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
