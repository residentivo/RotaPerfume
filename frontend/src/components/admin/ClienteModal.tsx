"use client";

import { useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Cliente, ClienteInput } from "@/lib/types";
import { useResetOnOpen } from "@/lib/useResetOnOpen";
import {
  CNPJ_HELPER,
  CNPJ_PLACEHOLDER,
  maskCnpjInput,
  normalizeCnpj,
  validateCnpj,
} from "@/lib/cnpj";
import { vlog } from "@/lib/vlog";

const F = "ClienteModal.tsx";

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
  vlog(F, "ClienteModal", "criando estado razaoSocial, modo:", mode);
  const [razaoSocial, setRazaoSocial] = useState("");
  vlog(F, "ClienteModal", "criando estado cnpj");
  const [cnpj, setCnpj] = useState("");
  vlog(F, "ClienteModal", "criando estado segmento");
  const [segmento, setSegmento] = useState("");
  vlog(F, "ClienteModal", "criando estado cidade");
  const [cidade, setCidade] = useState("");
  vlog(F, "ClienteModal", "criando estado uf");
  const [uf, setUf] = useState("");
  vlog(F, "ClienteModal", "criando estado bairro");
  const [bairro, setBairro] = useState("");
  vlog(F, "ClienteModal", "criando estado dataCadastro");
  const [dataCadastro, setDataCadastro] = useState("");
  vlog(F, "ClienteModal", "criando estado submitting");
  const [submitting, setSubmitting] = useState(false);
  vlog(F, "ClienteModal", "criando estado error");
  const [error, setError] = useState<string | null>(null);

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  vlog(F, "ClienteModal", "registrando reset do formulário ao abrir");
  useResetOnOpen(open, [mode, cliente], () => {
    vlog(F, "ClienteModal.reset", "limpando erro");
    setError(null);
    vlog(F, "ClienteModal.reset", "limpando estado de envio");
    setSubmitting(false);
    vlog(F, "ClienteModal.reset", "verificando se é edição com cliente, id:", cliente?.cliente_id_origem);
    if (mode === "edit" && cliente) {
      vlog(F, "ClienteModal.reset", "preenchendo razão social do cliente");
      setRazaoSocial(cliente.razao_social);
      vlog(F, "ClienteModal.reset", "preenchendo CNPJ mascarado do cliente");
      setCnpj(maskCnpjInput(cliente.cnpj));
      vlog(F, "ClienteModal.reset", "preenchendo segmento do cliente");
      setSegmento(cliente.segmento);
      vlog(F, "ClienteModal.reset", "preenchendo cidade do cliente");
      setCidade(cliente.cidade);
      vlog(F, "ClienteModal.reset", "preenchendo UF do cliente");
      setUf(cliente.uf);
      vlog(F, "ClienteModal.reset", "preenchendo bairro do cliente");
      setBairro(cliente.bairro);
      vlog(F, "ClienteModal.reset", "definindo data de cadastro");
      setDataCadastro(
        cliente.data_cadastro ? cliente.data_cadastro.slice(0, 10) : todayISO()
      );
    } else {
      vlog(F, "ClienteModal.reset", "limpando razão social");
      setRazaoSocial("");
      vlog(F, "ClienteModal.reset", "limpando CNPJ");
      setCnpj("");
      vlog(F, "ClienteModal.reset", "limpando segmento");
      setSegmento("");
      vlog(F, "ClienteModal.reset", "limpando cidade");
      setCidade("");
      vlog(F, "ClienteModal.reset", "limpando UF");
      setUf("");
      vlog(F, "ClienteModal.reset", "limpando bairro");
      setBairro("");
      vlog(F, "ClienteModal.reset", "definindo data de cadastro como hoje");
      setDataCadastro(todayISO());
    }
  });

  const handleSubmit = async (e: React.FormEvent) => {
    vlog(F, "ClienteModal.handleSubmit", "impedindo submit padrão do form");
    e.preventDefault();
    vlog(F, "ClienteModal.handleSubmit", "limpando erro");
    setError(null);

    vlog(F, "ClienteModal.handleSubmit", "validando razão social");
    if (!razaoSocial.trim()) {
      vlog(F, "ClienteModal.handleSubmit", "razão social ausente");
      setError("Razao social e obrigatoria.");
      return;
    }
    // NEG-02: mesma regra do backend (CNPJ numerico ou alfanumerico + DV).
    vlog(F, "ClienteModal.handleSubmit", "validando CNPJ");
    const erroCnpj = validateCnpj(cnpj);
    vlog(F, "ClienteModal.handleSubmit", "verificando resultado da validação do CNPJ:", !erroCnpj);
    if (erroCnpj) {
      vlog(F, "ClienteModal.handleSubmit", "CNPJ inválido");
      setError(erroCnpj);
      return;
    }
    vlog(F, "ClienteModal.handleSubmit", "validando segmento");
    if (!segmento.trim()) {
      vlog(F, "ClienteModal.handleSubmit", "segmento ausente");
      setError("Segmento e obrigatorio.");
      return;
    }
    vlog(F, "ClienteModal.handleSubmit", "validando cidade");
    if (!cidade.trim()) {
      vlog(F, "ClienteModal.handleSubmit", "cidade ausente");
      setError("Cidade e obrigatoria.");
      return;
    }
    vlog(F, "ClienteModal.handleSubmit", "validando UF");
    if (uf.trim().length !== 2) {
      vlog(F, "ClienteModal.handleSubmit", "UF inválida");
      setError("UF deve ter 2 letras.");
      return;
    }
    vlog(F, "ClienteModal.handleSubmit", "validando data de cadastro na edição");
    if (mode === "edit" && !dataCadastro) {
      vlog(F, "ClienteModal.handleSubmit", "data de cadastro ausente");
      setError("Data de cadastro e obrigatoria.");
      return;
    }

    vlog(F, "ClienteModal.handleSubmit", "marcando envio em andamento");
    setSubmitting(true);
    try {
      vlog(F, "ClienteModal.handleSubmit", "enviando formulário do cliente, modo:", mode);
      await onSubmit({
        razao_social: razaoSocial.trim(),
        cnpj: normalizeCnpj(cnpj),
        segmento: segmento.trim(),
        cidade: cidade.trim(),
        uf: uf.trim().toUpperCase(),
        bairro: bairro.trim(),
        data_cadastro: dataCadastro || undefined,
      });
    } catch (err) {
      vlog(F, "ClienteModal.handleSubmit", "falha ao salvar cliente: montando mensagem de erro");
      const message =
        err instanceof Error ? err.message : "Erro ao salvar cliente.";
      vlog(F, "ClienteModal.handleSubmit", "exibindo erro");
      setError(message);
    } finally {
      vlog(F, "ClienteModal.handleSubmit", "finalizando envio");
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
          onChange={(e) => setCnpj(maskCnpjInput(e.target.value))}
          placeholder={CNPJ_PLACEHOLDER}
          helperText={CNPJ_HELPER}
          inputMode="text"
          autoCapitalize="characters"
          autoComplete="off"
          spellCheck={false}
          required
        />

        <Input
          label="Segmento"
          value={segmento}
          onChange={(e) => setSegmento(e.target.value)}
          placeholder="Ex: Varejo"
          required
        />

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div className="sm:col-span-2">
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

        <div className="flex flex-wrap justify-end gap-2 pt-2">
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
