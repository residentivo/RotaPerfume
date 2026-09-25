"use client";

import { useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Estoque, EstoqueInput } from "@/lib/types";
import { useResetOnOpen } from "@/lib/useResetOnOpen";

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
  const [sku, setSku] = useState("");
  const [dataSnapshot, setDataSnapshot] = useState("");
  const [saldo, setSaldo] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  useResetOnOpen(open, [mode, estoque], () => {
    setError(null);
    setSubmitting(false);
    if (mode === "edit" && estoque) {
      setSku(estoque.sku);
      setDataSnapshot(estoque.data_snapshot.slice(0, 10));
      setSaldo(String(estoque.saldo));
    } else {
      setSku("");
      setDataSnapshot("");
      setSaldo("");
    }
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (mode === "create" && !sku.trim()) {
      setError("SKU e obrigatorio.");
      return;
    }
    if (mode === "create") {
      if (!dataSnapshot) {
        setError("Data do snapshot e obrigatoria.");
        return;
      }
      if (dataSnapshot > todayStr()) {
        setError("Data do snapshot nao pode ser futura.");
        return;
      }
    }
    const saldoNum = Number(saldo);
    if (saldo.trim() === "" || Number.isNaN(saldoNum) || saldoNum < 0) {
      setError("Saldo deve ser um numero maior ou igual a zero.");
      return;
    }

    setSubmitting(true);
    try {
      await onSubmit({
        sku: sku.trim(),
        data_snapshot: dataSnapshot,
        saldo: saldoNum,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao salvar registro de estoque.";
      setError(message);
    } finally {
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
