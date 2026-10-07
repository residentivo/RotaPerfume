"use client";

import { useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Estoque, EstoqueInput } from "@/lib/types";
import { useResetOnOpen } from "@/lib/useResetOnOpen";
import { vlog } from "@/lib/vlog";

const F = "EstoqueModal.tsx";

interface EstoqueModalProps {
  open: boolean;
  mode: "create" | "edit";
  estoque: Estoque | null;
  onClose: () => void;
  onSubmit: (data: EstoqueInput) => Promise<void>;
}

function todayStr(): string {
  return new Date().toISOString().slice(0, 10);
}

export function EstoqueModal({
  open,
  mode,
  estoque,
  onClose,
  onSubmit,
}: EstoqueModalProps) {
  vlog(F, "EstoqueModal", "criando estado sku, modo:", mode);
  const [sku, setSku] = useState("");
  vlog(F, "EstoqueModal", "criando estado dataSnapshot");
  const [dataSnapshot, setDataSnapshot] = useState("");
  vlog(F, "EstoqueModal", "criando estado saldo");
  const [saldo, setSaldo] = useState("");
  vlog(F, "EstoqueModal", "criando estado submitting");
  const [submitting, setSubmitting] = useState(false);
  vlog(F, "EstoqueModal", "criando estado error");
  const [error, setError] = useState<string | null>(null);

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  vlog(F, "EstoqueModal", "registrando reset do formulário ao abrir");
  useResetOnOpen(open, [mode, estoque], () => {
    vlog(F, "EstoqueModal.reset", "limpando erro");
    setError(null);
    vlog(F, "EstoqueModal.reset", "limpando estado de envio");
    setSubmitting(false);
    vlog(F, "EstoqueModal.reset", "verificando se é edição com registro, id:", estoque?.id);
    if (mode === "edit" && estoque) {
      vlog(F, "EstoqueModal.reset", "preenchendo SKU");
      setSku(estoque.sku);
      vlog(F, "EstoqueModal.reset", "preenchendo data do snapshot");
      setDataSnapshot(estoque.data_snapshot.slice(0, 10));
      vlog(F, "EstoqueModal.reset", "preenchendo saldo");
      setSaldo(String(estoque.saldo));
    } else {
      vlog(F, "EstoqueModal.reset", "limpando SKU");
      setSku("");
      vlog(F, "EstoqueModal.reset", "limpando data do snapshot");
      setDataSnapshot("");
      vlog(F, "EstoqueModal.reset", "limpando saldo");
      setSaldo("");
    }
  });

  const handleSubmit = async (e: React.FormEvent) => {
    vlog(F, "EstoqueModal.handleSubmit", "impedindo submit padrão do form");
    e.preventDefault();
    vlog(F, "EstoqueModal.handleSubmit", "limpando erro");
    setError(null);

    vlog(F, "EstoqueModal.handleSubmit", "validando SKU na criação");
    if (mode === "create" && !sku.trim()) {
      vlog(F, "EstoqueModal.handleSubmit", "SKU ausente");
      setError("SKU e obrigatorio.");
      return;
    }
    vlog(F, "EstoqueModal.handleSubmit", "verificando se é criação para validar data");
    if (mode === "create") {
      vlog(F, "EstoqueModal.handleSubmit", "validando presença da data do snapshot");
      if (!dataSnapshot) {
        vlog(F, "EstoqueModal.handleSubmit", "data do snapshot ausente");
        setError("Data do snapshot e obrigatoria.");
        return;
      }
      vlog(F, "EstoqueModal.handleSubmit", "validando se a data do snapshot não é futura");
      if (dataSnapshot > todayStr()) {
        vlog(F, "EstoqueModal.handleSubmit", "data do snapshot futura");
        setError("Data do snapshot nao pode ser futura.");
        return;
      }
    }
    vlog(F, "EstoqueModal.handleSubmit", "convertendo saldo para número");
    const saldoNum = Number(saldo);
    vlog(F, "EstoqueModal.handleSubmit", "validando saldo:", saldoNum);
    if (saldo.trim() === "" || Number.isNaN(saldoNum) || saldoNum < 0) {
      vlog(F, "EstoqueModal.handleSubmit", "saldo inválido");
      setError("Saldo deve ser um numero maior ou igual a zero.");
      return;
    }

    vlog(F, "EstoqueModal.handleSubmit", "marcando envio em andamento");
    setSubmitting(true);
    try {
      vlog(F, "EstoqueModal.handleSubmit", "enviando formulário de estoque, modo:", mode);
      await onSubmit({
        sku: sku.trim(),
        data_snapshot: dataSnapshot,
        saldo: saldoNum,
      });
    } catch (err) {
      vlog(F, "EstoqueModal.handleSubmit", "falha ao salvar estoque: montando mensagem de erro");
      const message =
        err instanceof Error ? err.message : "Erro ao salvar registro de estoque.";
      vlog(F, "EstoqueModal.handleSubmit", "exibindo erro");
      setError(message);
    } finally {
      vlog(F, "EstoqueModal.handleSubmit", "finalizando envio");
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Novo Registro de Estoque" : "Editar Estoque"}
      size="sm"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Alert variant="error">{error}</Alert>}

        <Input
          label="SKU"
          value={sku}
          onChange={(e) => setSku(e.target.value)}
          placeholder="Ex: PRF-0001"
          required
          autoFocus={mode === "create"}
          disabled={mode === "edit"}
        />

        <Input
          label="Data do snapshot"
          type="date"
          value={dataSnapshot}
          onChange={(e) => setDataSnapshot(e.target.value)}
          max={todayStr()}
          required
          disabled={mode === "edit"}
          helperText={mode === "edit" ? "Nao editavel apos a criacao." : undefined}
        />

        <Input
          label="Saldo"
          type="number"
          step="1"
          min="0"
          value={saldo}
          onChange={(e) => setSaldo(e.target.value)}
          placeholder="0"
          required
          autoFocus={mode === "edit"}
        />

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button type="submit" loading={submitting}>
            {mode === "create" ? "Criar registro" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
