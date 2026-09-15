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

function fmtDate(dateStr: string | null): string {
  if (!dateStr) return "-";
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  return d.toLocaleDateString("pt-BR");
}

function fmtCnpj(cnpj: string): string {
  const digits = (cnpj || "").replace(/\D/g, "");
  if (digits.length !== 14) return cnpj;
  return digits.replace(
    /(\d{2})(\d{3})(\d{3})(\d{4})(\d{2})/,
    "$1.$2.$3/$4-$5"
  );
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
        <span className="font-mono text-xs text-slate-600">{fmtCnpj(c.cnpj)}</span>
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
      render: (c) => <span className="text-slate-600">{fmtDate(c.data_inicio)}</span>,
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
  const [nome, setNome] = useState("");
  const [regiao, setRegiao] = useState("");
  const [uf, setUf] = useState("");
  const [dataAdmissao, setDataAdmissao] = useState("");
  const [metaMensal, setMetaMensal] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Detalhe do vendedor (com clientes vinculados), carregado apenas no modo
  // de edicao (vendedor precisa ter id) — mesmo padrao master-detail do
  // PedidoModal, porem aqui a lista de clientes e somente-leitura.
  const [detalhe, setDetalhe] = useState<VendedorDetalhe | null>(null);
  const [loadingDetalhe, setLoadingDetalhe] = useState(false);
  const [detalheError, setDetalheError] = useState<string | null>(null);

  // Combobox de clientes ativos para vincular a este vendedor (mesmo padrao
  // de carregamento usado no PedidoModal, via GET /api/clientes).
  const [todosClientes, setTodosClientes] = useState<Cliente[]>([]);
  const [loadingClientes, setLoadingClientes] = useState(false);
  const [clientesError, setClientesError] = useState<string | null>(null);
  const [clienteSelecionado, setClienteSelecionado] = useState("");
  const [vinculando, setVinculando] = useState(false);
  const [vincularError, setVincularError] = useState<string | null>(null);
  const [removendoId, setRemovendoId] = useState<number | null>(null);
  const [removerError, setRemoverError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setError(null);
      setSubmitting(false);
      if (mode === "edit" && vendedor) {
        setNome(vendedor.nome);
        setRegiao(vendedor.regiao);
        setUf(vendedor.uf);
        setDataAdmissao(
          vendedor.data_admissao ? vendedor.data_admissao.slice(0, 10) : todayISO()
        );
        setMetaMensal(String(vendedor.meta_mensal ?? 0));
      } else {
        setNome("");
        setRegiao("");
        setUf("");
        setDataAdmissao(todayISO());
        setMetaMensal("0");
      }
    }
  }, [open, mode, vendedor]);

  useEffect(() => {
    if (!open || mode !== "edit" || !vendedor) {
      setDetalhe(null);
      setDetalheError(null);
      return;
    }
    let cancelled = false;
    setLoadingDetalhe(true);
    setDetalheError(null);
    apiGetVendedor(vendedor.id)
      .then((res) => {
        // O backend pode retornar `clientes: null` em JSON quando o vendedor
        // ainda nao tem nenhum cliente vinculado (slice Go nil/vazio serializa
        // como null). Normalizamos aqui para `[]` para que todo o restante do
        // componente possa assumir que `detalhe.clientes` e sempre um array.
        if (!cancelled) setDetalhe({ ...res, clientes: res.clientes ?? [] });
      })
      .catch((err) => {
        if (!cancelled) {
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar clientes vinculados ao vendedor.";
          setDetalheError(message);
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingDetalhe(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, mode, vendedor]);

  // Carrega clientes ativos para popular o combobox de vinculo (mesmo padrao
  // do PedidoModal, via GET /api/clientes). Reseta os estados auxiliares
  // toda vez que o modal abre.
  useEffect(() => {
    if (!open || mode !== "edit" || !vendedor) {
      setTodosClientes([]);
      setClienteSelecionado("");
      setClientesError(null);
      setVincularError(null);
      setRemoverError(null);
      return;
    }
    let cancelled = false;
    setClienteSelecionado("");
    setVincularError(null);
    setRemoverError(null);
    setLoadingClientes(true);
    setClientesError(null);
    apiListClientes(1, 100, { ativo: true })
      .then((res) => {
        if (!cancelled) setTodosClientes(res.data);
      })
      .catch((err) => {
        if (!cancelled) {
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar clientes disponiveis.";
          setClientesError(message);
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingClientes(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, mode, vendedor]);

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
    if (!vendedor || !clienteSelecionado) return;
    setVincularError(null);
    setVinculando(true);
    try {
      const clienteResumo = await apiVincularCliente(
        vendedor.id,
        Number(clienteSelecionado)
      );
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
      setClienteSelecionado("");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao vincular cliente ao vendedor.";
      setVincularError(message);
    } finally {
      setVinculando(false);
    }
  };

  const handleDesvincularCliente = async (cliente: ClienteResumo) => {
    if (!vendedor) return;
    const ok = window.confirm(
      `Tem certeza que deseja encerrar o vinculo do cliente "#${cliente.id} - ${cliente.razao_social}" com este vendedor?`
    );
    if (!ok) return;

    setRemoverError(null);
    setRemovendoId(cliente.id);
    try {
      await apiDesvincularCliente(vendedor.id, cliente.id);
      setDetalhe((prev) =>
        prev
          ? { ...prev, clientes: (prev.clientes ?? []).filter((c) => c.id !== cliente.id) }
          : prev
      );
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao encerrar o vinculo com o cliente.";
      setRemoverError(message);
    } finally {
      setRemovendoId(null);
    }
  };

  const clienteColumns = useMemo(
    () => buildClienteColumns(handleDesvincularCliente, removendoId),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [removendoId, vendedor]
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!nome.trim()) {
      setError("Nome e obrigatorio.");
      return;
    }
    if (!regiao.trim()) {
      setError("Regiao e obrigatoria.");
      return;
    }
    if (uf.trim().length !== 2) {
      setError("UF deve ter 2 letras.");
      return;
    }
    if (mode === "edit" && !dataAdmissao) {
      setError("Data de admissao e obrigatoria.");
      return;
    }
    const meta = Number(metaMensal);
    if (!Number.isFinite(meta) || meta < 0) {
      setError("Meta mensal deve ser um numero maior ou igual a zero.");
      return;
    }

    setSubmitting(true);
    try {
      await onSubmit({
        nome: nome.trim(),
        regiao: regiao.trim(),
        uf: uf.trim().toUpperCase(),
        data_admissao: dataAdmissao || undefined,
        meta_mensal: meta,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao salvar vendedor.";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Novo Vendedor" : "Editar Vendedor"}
      size="lg"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Alert variant="error">{error}</Alert>}

        <Input
          label="Nome"
          value={nome}
          onChange={(e) => setNome(e.target.value)}
          placeholder="Ex: Joao da Silva"
          required
          autoFocus
        />

        <div className="grid grid-cols-3 gap-3">
          <div className="col-span-2">
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

        <div className="grid grid-cols-2 gap-3">
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

        {mode === "edit" && (
          <div>
            <h3 className="mb-2 text-sm font-semibold text-slate-700">
              Clientes vinculados
            </h3>
            {detalheError && (
              <Alert variant="error">
                Nao foi possivel carregar os clientes vinculados: {detalheError}
              </Alert>
            )}
            {clientesError && (
              <Alert variant="error">
                Nao foi possivel carregar os clientes disponiveis: {clientesError}
              </Alert>
            )}
            {vincularError && <Alert variant="error">{vincularError}</Alert>}
            {removerError && <Alert variant="error">{removerError}</Alert>}

            <div className="mb-3 flex items-end gap-2">
              <div className="flex-1">
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

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button type="submit" loading={submitting}>
            {mode === "create" ? "Criar vendedor" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
