"use client";

import { useEffect, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Table, Column } from "@/components/ui/Table";
import {
  apiGetVendedor,
  apiListClientes,
  apiVincularCliente,
  apiDesvincularCliente,
} from "@/lib/api";
import { Cliente, ClienteResumo, VendedorCompleto, VendedorDetalhe, VendedorInput } from "@/lib/types";
import { useAjustarAoMudar, useResetOnOpen } from "@/lib/useResetOnOpen";
import { formatCnpj } from "@/lib/cnpj";
import { formatarData } from "@/lib/formatarData";
import { vlog } from "@/lib/vlog";

const F = "VendedorModal.tsx";

interface VendedorModalProps {
  open: boolean;
  mode: "create" | "edit";
  vendedor: VendedorCompleto | null;
  onClose: () => void;
  onSubmit: (data: VendedorInput) => Promise<void>;
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

// Normaliza a data vinda da API (AAAA-MM-DD ou ISO datetime) para o formato
// AAAA-MM-DD exigido pelo <input type="date">. Retorna "" se nao reconhecer.
function toDateInput(dateStr: string | null | undefined): string {
  vlog(F, "toDateInput", "verificando se há data:", !!dateStr);
  if (!dateStr) return "";
  vlog(F, "toDateInput", "extraindo AAAA-MM-DD do início da data");
  const m = /^(\d{4}-\d{2}-\d{2})/.exec(dateStr);
  vlog(F, "toDateInput", "verificando se o formato AAAA-MM-DD foi reconhecido:", !!m);
  if (m) return m[1];
  vlog(F, "toDateInput", "convertendo data para Date");
  const d = new Date(dateStr);
  vlog(F, "toDateInput", "verificando se a data é válida");
  if (Number.isNaN(d.getTime())) return "";
  return d.toISOString().slice(0, 10);
}

function buildClienteColumns(
  onRemover: (cliente: ClienteResumo) => void,
  removendoId: number | null
): Column<ClienteResumo>[] {
  return [
    {
      key: "razao_social",
      header: "Cliente",
      render: (c) => (
        <span className="font-medium text-slate-800">
          #{c.id} - {c.razao_social}
        </span>
      ),
    },
    {
      key: "cnpj",
      header: "CNPJ",
      width: "160px",
      render: (c) => (
        <span className="font-mono text-xs text-slate-600">{formatCnpj(c.cnpj)}</span>
      ),
    },
    {
      key: "segmento",
      header: "Segmento",
      width: "140px",
      render: (c) => <span className="text-slate-600">{c.segmento || "-"}</span>,
    },
    {
      key: "cidade",
      header: "Cidade/UF",
      width: "160px",
      render: (c) => (
        <span className="text-slate-600">
          {c.cidade}
          {c.uf ? `/${c.uf}` : ""}
        </span>
      ),
    },
    {
      key: "data_inicio",
      header: "Vinculado em",
      width: "120px",
      align: "center",
      render: (c) => <span className="text-slate-600">{formatarData(c.data_inicio)}</span>,
    },
    {
      key: "actions",
      header: "Acoes",
      width: "100px",
      align: "right",
      render: (c) => (
        <Button
          type="button"
          size="sm"
          variant="danger"
          onClick={() => onRemover(c)}
          disabled={removendoId === c.id}
          title="Encerrar vinculo com este vendedor"
        >
          Remover
        </Button>
      ),
    },
  ];
}

export function VendedorModal({
  open,
  mode,
  vendedor,
  onClose,
  onSubmit,
}: VendedorModalProps) {
  vlog(F, "VendedorModal", "criando estado nome, modo:", mode);
  const [nome, setNome] = useState("");
  vlog(F, "VendedorModal", "criando estado regiao");
  const [regiao, setRegiao] = useState("");
  vlog(F, "VendedorModal", "criando estado uf");
  const [uf, setUf] = useState("");
  vlog(F, "VendedorModal", "criando estado dataAdmissao");
  const [dataAdmissao, setDataAdmissao] = useState("");
  vlog(F, "VendedorModal", "criando estado metaMensal");
  const [metaMensal, setMetaMensal] = useState("");
  vlog(F, "VendedorModal", "criando estado submitting");
  const [submitting, setSubmitting] = useState(false);
  vlog(F, "VendedorModal", "criando estado error");
  const [error, setError] = useState<string | null>(null);

  // Detalhe do vendedor (com clientes vinculados), carregado apenas no modo
  // de edicao (vendedor precisa ter id) — mesmo padrao master-detail do
  // PedidoModal, porem aqui a lista de clientes e somente-leitura.
  vlog(F, "VendedorModal", "criando estado detalhe");
  const [detalhe, setDetalhe] = useState<VendedorDetalhe | null>(null);
  vlog(F, "VendedorModal", "criando estado loadingDetalhe");
  const [loadingDetalhe, setLoadingDetalhe] = useState(false);
  vlog(F, "VendedorModal", "criando estado detalheError");
  const [detalheError, setDetalheError] = useState<string | null>(null);

  // Combobox de clientes ativos para vincular a este vendedor (mesmo padrao
  // de carregamento usado no PedidoModal, via GET /api/clientes).
  vlog(F, "VendedorModal", "criando estado todosClientes");
  const [todosClientes, setTodosClientes] = useState<Cliente[]>([]);
  vlog(F, "VendedorModal", "criando estado loadingClientes");
  const [loadingClientes, setLoadingClientes] = useState(false);
  vlog(F, "VendedorModal", "criando estado clientesError");
  const [clientesError, setClientesError] = useState<string | null>(null);
  vlog(F, "VendedorModal", "criando estado clienteSelecionado");
  const [clienteSelecionado, setClienteSelecionado] = useState("");
  vlog(F, "VendedorModal", "criando estado vinculando");
  const [vinculando, setVinculando] = useState(false);
  vlog(F, "VendedorModal", "criando estado vincularError");
  const [vincularError, setVincularError] = useState<string | null>(null);
  vlog(F, "VendedorModal", "criando estado removendoId");
  const [removendoId, setRemovendoId] = useState<number | null>(null);
  vlog(F, "VendedorModal", "criando estado removerError");
  const [removerError, setRemoverError] = useState<string | null>(null);

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  vlog(F, "VendedorModal", "registrando reset do formulário ao abrir");
  useResetOnOpen(open, [mode, vendedor], () => {
    vlog(F, "VendedorModal.reset", "limpando erro");
    setError(null);
    vlog(F, "VendedorModal.reset", "limpando estado de envio");
    setSubmitting(false);
    vlog(F, "VendedorModal.reset", "verificando se é edição com vendedor, id:", vendedor?.id);
    if (mode === "edit" && vendedor) {
      // BUG-07: a listagem (GET /api/vendedores) nao traz data_admissao nem
      // meta_mensal (projecao do SEC-03); o `vendedor` recebido tem valores
      // provisorios nesses campos. Eles NAO sao usados: o formulario fica
      // vazio/bloqueado ate o detalhe (GET /api/vendedores/{id}) chegar e
      // preencher todos os campos (ver efeito do detalhe abaixo).
      vlog(F, "VendedorModal.reset", "preenchendo nome provisório");
      setNome(vendedor.nome);
      vlog(F, "VendedorModal.reset", "preenchendo região provisória");
      setRegiao(vendedor.regiao);
      vlog(F, "VendedorModal.reset", "preenchendo UF provisória");
      setUf(vendedor.uf);
      vlog(F, "VendedorModal.reset", "limpando data de admissão até o detalhe chegar");
      setDataAdmissao("");
      vlog(F, "VendedorModal.reset", "limpando meta mensal até o detalhe chegar");
      setMetaMensal("");
    } else {
      vlog(F, "VendedorModal.reset", "limpando nome");
      setNome("");
      vlog(F, "VendedorModal.reset", "limpando região");
      setRegiao("");
      vlog(F, "VendedorModal.reset", "limpando UF");
      setUf("");
      vlog(F, "VendedorModal.reset", "definindo data de admissão como hoje");
      setDataAdmissao(todayISO());
      vlog(F, "VendedorModal.reset", "definindo meta mensal zero");
      setMetaMensal("0");
    }
  });

  // Detalhe (clientes vinculados): a parte sincrona roda durante o render
  // quando open/mode/vendedor mudam (useAjustarAoMudar); o efeito so busca
  // e aplica o resultado nos callbacks assincronos.
  vlog(F, "VendedorModal", "registrando reset do detalhe");
  useAjustarAoMudar([open, mode, vendedor], () => {
    vlog(F, "VendedorModal.resetDetalhe", "limpando erro do detalhe");
    setDetalheError(null);
    // Sempre descarta o detalhe anterior: com outro vendedor (ou reabertura)
    // o salvar volta a ficar bloqueado ate o novo detalhe chegar (BUG-07).
    vlog(F, "VendedorModal.resetDetalhe", "descartando detalhe anterior");
    setDetalhe(null);
    vlog(F, "VendedorModal.resetDetalhe", "verificando se deve carregar detalhe (edição aberta)");
    if (!open || mode !== "edit" || !vendedor) {
      vlog(F, "VendedorModal.resetDetalhe", "desligando loading do detalhe");
      setLoadingDetalhe(false);
      return;
    }
    vlog(F, "VendedorModal.resetDetalhe", "ligando loading do detalhe");
    setLoadingDetalhe(true);
  });

  vlog(F, "VendedorModal", "registrando efeito de carga do detalhe");
  useEffect(() => {
    vlog(F, "VendedorModal.useEffect", "verificando se deve buscar detalhe:", open, mode, vendedor?.id);
    if (!open || mode !== "edit" || !vendedor) return;
    vlog(F, "VendedorModal.useEffect", "inicializando flag de cancelamento (detalhe)");
    let cancelled = false;
    vlog(F, "VendedorModal.useEffect", "buscando detalhe do vendedor, id:", vendedor.id);
    apiGetVendedor(vendedor.id)
      .then((res) => {
        // O backend pode retornar `clientes: null` em JSON quando o vendedor
        // ainda nao tem nenhum cliente vinculado (slice Go nil/vazio serializa
        // como null). Normalizamos aqui para `[]` para que todo o restante do
        // componente possa assumir que `detalhe.clientes` e sempre um array.
        vlog(F, "VendedorModal.useEffect", "detalhe recebido, cancelado:", cancelled);
        if (cancelled) return;
        vlog(F, "VendedorModal.useEffect", "aplicando detalhe, clientes vinculados:", (res.clientes ?? []).length);
        setDetalhe({ ...res, clientes: res.clientes ?? [] });
        // BUG-07: o formulario de edicao e preenchido com os dados reais do
        // banco (detalhe), nunca com os valores provisorios da listagem.
        vlog(F, "VendedorModal.useEffect", "preenchendo nome do detalhe");
        setNome(res.nome ?? "");
        vlog(F, "VendedorModal.useEffect", "preenchendo região do detalhe");
        setRegiao(res.regiao ?? "");
        vlog(F, "VendedorModal.useEffect", "preenchendo UF do detalhe");
        setUf(res.uf ?? "");
        vlog(F, "VendedorModal.useEffect", "preenchendo data de admissão do detalhe");
        setDataAdmissao(toDateInput(res.data_admissao));
        vlog(F, "VendedorModal.useEffect", "preenchendo meta mensal do detalhe");
        setMetaMensal(String(res.meta_mensal ?? 0));
      })
      .catch((err) => {
        vlog(F, "VendedorModal.useEffect", "falha ao buscar detalhe, cancelado:", cancelled);
        if (!cancelled) {
          vlog(F, "VendedorModal.useEffect", "montando mensagem de erro do detalhe");
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar os dados do vendedor.";
          vlog(F, "VendedorModal.useEffect", "exibindo erro do detalhe");
          setDetalheError(message);
        }
      })
      .finally(() => {
        vlog(F, "VendedorModal.useEffect", "desligando loading do detalhe se não cancelado");
        if (!cancelled) setLoadingDetalhe(false);
      });
    return () => {
      vlog(F, "VendedorModal.useEffect", "cleanup: cancelando carga do detalhe");
      cancelled = true;
    };
  }, [open, mode, vendedor]);

  // Carrega clientes ativos para popular o combobox de vinculo (mesmo padrao
  // do PedidoModal, via GET /api/clientes). Reseta os estados auxiliares
  // toda vez que o modal abre.
  // Parte sincrona (reset dos estados auxiliares) roda durante o render
  // quando open/mode/vendedor mudam; o efeito so busca.
  vlog(F, "VendedorModal", "registrando reset do combobox de clientes");
  useAjustarAoMudar([open, mode, vendedor], () => {
    vlog(F, "VendedorModal.resetClientes", "limpando cliente selecionado");
    setClienteSelecionado("");
    vlog(F, "VendedorModal.resetClientes", "limpando erro de clientes");
    setClientesError(null);
    vlog(F, "VendedorModal.resetClientes", "limpando erro de vínculo");
    setVincularError(null);
    vlog(F, "VendedorModal.resetClientes", "limpando erro de remoção");
    setRemoverError(null);
    vlog(F, "VendedorModal.resetClientes", "verificando se deve carregar clientes (edição aberta)");
    if (!open || mode !== "edit" || !vendedor) {
      vlog(F, "VendedorModal.resetClientes", "limpando lista de clientes");
      setTodosClientes([]);
      vlog(F, "VendedorModal.resetClientes", "desligando loading de clientes");
      setLoadingClientes(false);
      return;
    }
    vlog(F, "VendedorModal.resetClientes", "ligando loading de clientes");
    setLoadingClientes(true);
  });

  vlog(F, "VendedorModal", "registrando efeito de carga de clientes ativos");
  useEffect(() => {
    vlog(F, "VendedorModal.useEffect", "verificando se deve buscar clientes:", open, mode, vendedor?.id);
    if (!open || mode !== "edit" || !vendedor) return;
    vlog(F, "VendedorModal.useEffect", "inicializando flag de cancelamento (clientes)");
    let cancelled = false;
    vlog(F, "VendedorModal.useEffect", "buscando clientes ativos");
    apiListClientes(1, 100, { ativo: true })
      .then((res) => {
        vlog(F, "VendedorModal.useEffect", "clientes recebidos, qtd/cancelado:", res.data.length, cancelled);
        if (!cancelled) setTodosClientes(res.data);
      })
      .catch((err) => {
        vlog(F, "VendedorModal.useEffect", "falha ao buscar clientes, cancelado:", cancelled);
        if (!cancelled) {
          vlog(F, "VendedorModal.useEffect", "montando mensagem de erro de clientes");
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar clientes disponiveis.";
          vlog(F, "VendedorModal.useEffect", "exibindo erro de clientes");
          setClientesError(message);
        }
      })
      .finally(() => {
        vlog(F, "VendedorModal.useEffect", "desligando loading de clientes se não cancelado");
        if (!cancelled) setLoadingClientes(false);
      });
    return () => {
      vlog(F, "VendedorModal.useEffect", "cleanup: cancelando carga de clientes");
      cancelled = true;
    };
  }, [open, mode, vendedor]);

  vlog(F, "VendedorModal", "memorizando opções de cliente para vínculo");
  const clienteOptions = useMemo(
    () => [
      { value: "", label: "Selecione um cliente" },
      ...todosClientes
        .filter(
          (c) =>
            !(detalhe?.clientes ?? []).some(
              (v) => v.id === c.cliente_id_origem
            )
        )
        .map((c) => ({
          value: String(c.cliente_id_origem),
          label: `#${c.cliente_id_origem} - ${c.razao_social}`,
        })),
    ],
    [todosClientes, detalhe]
  );

  const handleVincularCliente = async () => {
    vlog(F, "VendedorModal.handleVincularCliente", "verificando vendedor e cliente selecionados:", vendedor?.id, clienteSelecionado);
    if (!vendedor || !clienteSelecionado) return;
    vlog(F, "VendedorModal.handleVincularCliente", "limpando erro de vínculo");
    setVincularError(null);
    vlog(F, "VendedorModal.handleVincularCliente", "marcando vínculo em andamento");
    setVinculando(true);
    try {
      vlog(F, "VendedorModal.handleVincularCliente", "vinculando cliente ao vendedor, vendedor/cliente:", vendedor.id, Number(clienteSelecionado));
      const clienteResumo = await apiVincularCliente(
        vendedor.id,
        Number(clienteSelecionado)
      );
      vlog(F, "VendedorModal.handleVincularCliente", "adicionando cliente vinculado ao detalhe, id:", clienteResumo.id);
      setDetalhe((prev) =>
        prev
          ? {
              ...prev,
              clientes: [
                ...(prev.clientes ?? []).filter((c) => c.id !== clienteResumo.id),
                clienteResumo,
              ],
            }
          : prev
      );
      vlog(F, "VendedorModal.handleVincularCliente", "limpando cliente selecionado");
      setClienteSelecionado("");
    } catch (err) {
      vlog(F, "VendedorModal.handleVincularCliente", "falha ao vincular: montando mensagem de erro");
      const message =
        err instanceof Error ? err.message : "Erro ao vincular cliente ao vendedor.";
      vlog(F, "VendedorModal.handleVincularCliente", "exibindo erro de vínculo");
      setVincularError(message);
    } finally {
      vlog(F, "VendedorModal.handleVincularCliente", "finalizando vínculo");
      setVinculando(false);
    }
  };

  const handleDesvincularCliente = async (cliente: ClienteResumo) => {
    vlog(F, "VendedorModal.handleDesvincularCliente", "verificando vendedor:", vendedor?.id);
    if (!vendedor) return;
    vlog(F, "VendedorModal.handleDesvincularCliente", "pedindo confirmação para desvincular cliente, id:", cliente.id);
    const ok = window.confirm(
      `Tem certeza que deseja encerrar o vinculo do cliente "#${cliente.id} - ${cliente.razao_social}" com este vendedor?`
    );
    vlog(F, "VendedorModal.handleDesvincularCliente", "verificando confirmação:", ok);
    if (!ok) return;

    vlog(F, "VendedorModal.handleDesvincularCliente", "limpando erro de remoção");
    setRemoverError(null);
    vlog(F, "VendedorModal.handleDesvincularCliente", "marcando cliente em remoção");
    setRemovendoId(cliente.id);
    try {
      vlog(F, "VendedorModal.handleDesvincularCliente", "desvinculando cliente, vendedor/cliente:", vendedor.id, cliente.id);
      await apiDesvincularCliente(vendedor.id, cliente.id);
      vlog(F, "VendedorModal.handleDesvincularCliente", "removendo cliente do detalhe");
      setDetalhe((prev) =>
        prev
          ? { ...prev, clientes: (prev.clientes ?? []).filter((c) => c.id !== cliente.id) }
          : prev
      );
    } catch (err) {
      vlog(F, "VendedorModal.handleDesvincularCliente", "falha ao desvincular: montando mensagem de erro");
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao encerrar o vinculo com o cliente.";
      vlog(F, "VendedorModal.handleDesvincularCliente", "exibindo erro de remoção");
      setRemoverError(message);
    } finally {
      vlog(F, "VendedorModal.handleDesvincularCliente", "limpando cliente em remoção");
      setRemovendoId(null);
    }
  };

  vlog(F, "VendedorModal", "memorizando colunas da tabela de clientes");
  const clienteColumns = useMemo(
    () => buildClienteColumns(handleDesvincularCliente, removendoId),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [removendoId, vendedor]
  );

  const handleSubmit = async (e: React.FormEvent) => {
    vlog(F, "VendedorModal.handleSubmit", "impedindo submit padrão do form");
    e.preventDefault();
    vlog(F, "VendedorModal.handleSubmit", "limpando erro");
    setError(null);

    // BUG-07: no modo edicao so salva depois que o detalhe carregou; antes
    // disso o formulario nao tem data_admissao/meta_mensal reais.
    vlog(F, "VendedorModal.handleSubmit", "verificando se o detalhe foi carregado na edição:", !!detalhe);
    if (mode === "edit" && !detalhe) {
      vlog(F, "VendedorModal.handleSubmit", "detalhe não carregado: bloqueando salvar");
      setError(
        detalheError
          ? "Nao e possivel salvar: os dados do vendedor nao foram carregados."
          : "Aguarde o carregamento dos dados do vendedor."
      );
      return;
    }

    vlog(F, "VendedorModal.handleSubmit", "validando nome");
    if (!nome.trim()) {
      vlog(F, "VendedorModal.handleSubmit", "nome ausente");
      setError("Nome e obrigatorio.");
      return;
    }
    vlog(F, "VendedorModal.handleSubmit", "validando região");
    if (!regiao.trim()) {
      vlog(F, "VendedorModal.handleSubmit", "região ausente");
      setError("Regiao e obrigatoria.");
      return;
    }
    vlog(F, "VendedorModal.handleSubmit", "validando UF");
    if (uf.trim().length !== 2) {
      vlog(F, "VendedorModal.handleSubmit", "UF inválida");
      setError("UF deve ter 2 letras.");
      return;
    }
    vlog(F, "VendedorModal.handleSubmit", "validando data de admissão na edição");
    if (mode === "edit" && !dataAdmissao) {
      vlog(F, "VendedorModal.handleSubmit", "data de admissão ausente");
      setError("Data de admissao e obrigatoria.");
      return;
    }
    vlog(F, "VendedorModal.handleSubmit", "convertendo meta mensal");
    const meta = Number(metaMensal);
    vlog(F, "VendedorModal.handleSubmit", "validando meta mensal:", meta);
    if (!Number.isFinite(meta) || meta < 0) {
      vlog(F, "VendedorModal.handleSubmit", "meta mensal inválida");
      setError("Meta mensal deve ser um numero maior ou igual a zero.");
      return;
    }

    vlog(F, "VendedorModal.handleSubmit", "marcando envio em andamento");
    setSubmitting(true);
    try {
      vlog(F, "VendedorModal.handleSubmit", "enviando vendedor, modo:", mode);
      await onSubmit({
        nome: nome.trim(),
        regiao: regiao.trim(),
        uf: uf.trim().toUpperCase(),
        data_admissao: dataAdmissao || undefined,
        meta_mensal: meta,
      });
    } catch (err) {
      vlog(F, "VendedorModal.handleSubmit", "falha ao salvar vendedor: montando mensagem de erro");
      const message =
        err instanceof Error ? err.message : "Erro ao salvar vendedor.";
      vlog(F, "VendedorModal.handleSubmit", "exibindo erro");
      setError(message);
    } finally {
      vlog(F, "VendedorModal.handleSubmit", "finalizando envio");
      setSubmitting(false);
    }
  };

  // Edicao bloqueada enquanto o detalhe nao carregou (ou se falhou).
  vlog(F, "VendedorModal", "calculando se aguarda o detalhe");
  const aguardandoDetalhe = mode === "edit" && !detalhe;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Novo Vendedor" : "Editar Vendedor"}
      size="lg"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Alert variant="error">{error}</Alert>}

        {mode === "edit" && loadingDetalhe && (
          <Alert variant="info">Carregando dados do vendedor...</Alert>
        )}
        {mode === "edit" && detalheError && (
          <Alert variant="error">
            Nao foi possivel carregar os dados do vendedor: {detalheError}
          </Alert>
        )}

        <fieldset disabled={aguardandoDetalhe} className="space-y-4">
          <Input
            label="Nome"
            value={nome}
            onChange={(e) => setNome(e.target.value)}
            placeholder="Ex: Joao da Silva"
            required
            autoFocus
          />

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div className="sm:col-span-2">
              <Input
                label="Regiao"
                value={regiao}
                onChange={(e) => setRegiao(e.target.value)}
                placeholder="Ex: Sudeste"
                required
              />
            </div>
            <Input
              label="UF"
              value={uf}
              onChange={(e) => setUf(e.target.value.toUpperCase())}
              placeholder="SP"
              maxLength={2}
              required
            />
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Input
              label="Data de admissao"
              type="date"
              value={dataAdmissao}
              onChange={(e) => setDataAdmissao(e.target.value)}
              required={mode === "edit"}
              helperText={
                mode === "create"
                  ? "Se nao informada, sera usada a data de hoje."
                  : undefined
              }
            />
            <Input
              label="Meta mensal"
              type="number"
              min="0"
              step="0.01"
              value={metaMensal}
              onChange={(e) => setMetaMensal(e.target.value)}
              required
            />
          </div>
        </fieldset>

        {mode === "edit" && (
          <div>
            <h3 className="mb-2 text-sm font-semibold text-slate-700">
              Clientes vinculados
            </h3>
            {clientesError && (
              <Alert variant="error">
                Nao foi possivel carregar os clientes disponiveis: {clientesError}
              </Alert>
            )}
            {vincularError && <Alert variant="error">{vincularError}</Alert>}
            {removerError && <Alert variant="error">{removerError}</Alert>}

            <div className="mb-3 flex flex-wrap items-end gap-2">
              <div className="min-w-0 flex-1">
                <Select
                  label="Vincular cliente"
                  options={clienteOptions}
                  value={clienteSelecionado}
                  onChange={(e) => setClienteSelecionado(e.target.value)}
                  disabled={loadingClientes || vinculando}
                />
              </div>
              <Button
                type="button"
                variant="secondary"
                onClick={handleVincularCliente}
                disabled={!clienteSelecionado || vinculando}
                loading={vinculando}
              >
                Adicionar
              </Button>
            </div>

            <div className="max-h-64 overflow-y-auto rounded-md border border-slate-200">
              <Table
                columns={clienteColumns}
                data={detalhe?.clientes ?? []}
                keyExtractor={(c) => c.carteira_id}
                loading={loadingDetalhe}
                emptyMessage="Nenhum cliente vinculado a este vendedor."
              />
            </div>
          </div>
        )}

        <div className="flex flex-wrap justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button
            type="submit"
            loading={submitting}
            disabled={aguardandoDetalhe}
            title={
              aguardandoDetalhe ? "Aguarde o carregamento dos dados do vendedor" : undefined
            }
          >
            {mode === "create" ? "Criar vendedor" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
