"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Cliente, ClienteInput } from "@/lib/types";

interface ClienteModalProps {
  open: boolean;
  mode: "create" | "edit";
  cliente: Cliente | null;
  onClose: () => void;
  onSubmit: (data: ClienteInput) => Promise<void>;
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

export function ClienteModal({
  open,
  mode,
  cliente,
  onClose,
  onSubmit,
}: ClienteModalProps) {
  const [razaoSocial, setRazaoSocial] = useState("");
  const [cnpj, setCnpj] = useState("");
  const [segmento, setSegmento] = useState("");
  const [cidade, setCidade] = useState("");
  const [uf, setUf] = useState("");
  const [bairro, setBairro] = useState("");
  const [dataCadastro, setDataCadastro] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setError(null);
      setSubmitting(false);
      if (mode === "edit" && cliente) {
        setRazaoSocial(cliente.razao_social);
        setCnpj(cliente.cnpj);
        setSegmento(cliente.segmento);
        setCidade(cliente.cidade);
        setUf(cliente.uf);
        setBairro(cliente.bairro);
        setDataCadastro(
          cliente.data_cadastro ? cliente.data_cadastro.slice(0, 10) : todayISO()
        );
      } else {
        setRazaoSocial("");
        setCnpj("");
        setSegmento("");
        setCidade("");
        setUf("");
        setBairro("");
        setDataCadastro(todayISO());
      }
    }
  }, [open, mode, cliente]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!razaoSocial.trim()) {
      setError("Razao social e obrigatoria.");
      return;
    }
    if (!cnpj.trim()) {
      setError("CNPJ e obrigatorio.");
      return;
    }
    if (!segmento.trim()) {
      setError("Segmento e obrigatorio.");
      return;
    }
    if (!cidade.trim()) {
      setError("Cidade e obrigatoria.");
      return;
    }
    if (uf.trim().length !== 2) {
      setError("UF deve ter 2 letras.");
      return;
    }
    if (mode === "edit" && !dataCadastro) {
      setError("Data de cadastro e obrigatoria.");
      return;
    }

    setSubmitting(true);
    try {
      await onSubmit({
        razao_social: razaoSocial.trim(),
        cnpj: cnpj.trim(),
        segmento: segmento.trim(),
        cidade: cidade.trim(),
        uf: uf.trim().toUpperCase(),
        bairro: bairro.trim(),
        data_cadastro: dataCadastro || undefined,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao salvar cliente.";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Novo Cliente" : "Editar Cliente"}
      size="md"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Alert variant="error">{error}</Alert>}

        <Input
          label="Razao social"
          value={razaoSocial}
          onChange={(e) => setRazaoSocial(e.target.value)}
          placeholder="Ex: Perfumaria Exemplo Ltda"
          required
          autoFocus
        />

        <Input
          label="CNPJ"
          value={cnpj}
          onChange={(e) => setCnpj(e.target.value)}
          placeholder="00.000.000/0000-00"
          required
        />

        <Input
          label="Segmento"
          value={segmento}
          onChange={(e) => setSegmento(e.target.value)}
          placeholder="Ex: Varejo"
          required
        />

        <div className="grid grid-cols-3 gap-3">
          <div className="col-span-2">
            <Input
              label="Cidade"
              value={cidade}
              onChange={(e) => setCidade(e.target.value)}
              placeholder="Ex: Sao Paulo"
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

        <Input
          label="Bairro"
          value={bairro}
          onChange={(e) => setBairro(e.target.value)}
          placeholder="Ex: Centro"
        />

        <Input
          label="Data de cadastro"
          type="date"
          value={dataCadastro}
          onChange={(e) => setDataCadastro(e.target.value)}
          required={mode === "edit"}
          helperText={
            mode === "create"
              ? "Se nao informada, sera usada a data de hoje."
              : undefined
          }
        />

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button type="submit" loading={submitting}>
            {mode === "create" ? "Criar cliente" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
