"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { User, UserRole, Vendedor } from "@/lib/types";
import { useResetOnOpen } from "@/lib/useResetOnOpen";
import { apiListVendedores } from "@/lib/api";

interface UserModalProps {
  open: boolean;
  mode: "create" | "edit";
  user: User | null;
  onClose: () => void;
  onSubmit: (data: {
    nome: string;
    email: string;
    role: UserRole;
    id_vendedor: number | null;
  }) => Promise<void>;
}

const ROLE_OPTIONS = [
  { value: "admin", label: "Administrador" },
  { value: "normal", label: "Usuario Padrao" },
];

export function UserModal({
  open,
  mode,
  user,
  onClose,
  onSubmit,
}: UserModalProps) {
  const [nome, setNome] = useState("");
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<UserRole>("normal");
  const [idVendedor, setIdVendedor] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  const [vendedoresError, setVendedoresError] = useState<string | null>(null);
  const [loadingVendedores, setLoadingVendedores] = useState(false);

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  useResetOnOpen(open, [mode, user], () => {
    setError(null);
    setSubmitting(false);
    if (mode === "edit" && user) {
      setNome(user.nome);
      setEmail(user.email);
      setRole(user.role);
      setIdVendedor(
        user.id_vendedor !== null && user.id_vendedor !== undefined
          ? String(user.id_vendedor)
          : ""
      );
    } else {
      setNome("");
      setEmail("");
      setRole("normal");
      setIdVendedor("");
    }
  });

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
      .then((data) => {
        if (!cancelled) setVendedores(data);
      })
      .catch((err) => {
        if (!cancelled) {
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar vendedores.";
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

  const vendedorOptions = [
    { value: "", label: "Nenhum" },
    ...vendedores.map((v) => ({
      value: String(v.id),
      label: `${v.nome} — ${v.regiao}/${v.uf}${
        v.data_desligamento ? " [X]" : ""
      }`,
    })),
  ];

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!nome.trim()) {
      setError("Nome e obrigatorio.");
      return;
    }
    if (mode === "create" && !email.trim()) {
      setError("Email e obrigatorio.");
      return;
    }

    setSubmitting(true);
    try {
      await onSubmit({
        nome: nome.trim(),
        email: email.trim(),
        role,
        id_vendedor: idVendedor ? Number(idVendedor) : null,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao salvar usuario.";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Novo Usuario" : "Editar Usuario"}
      size="md"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Alert variant="error">{error}</Alert>}

        <Input
          label="Nome completo"
          value={nome}
          onChange={(e) => setNome(e.target.value)}
          placeholder="Ex: Joao da Silva"
          required
          autoFocus
        />

        {mode === "create" && (
          <Input
            label="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="usuario@empresa.com"
            required
          />
        )}

        {mode === "edit" && (
          <Input
            label="Email"
            type="email"
            value={email}
            disabled
            helperText="O email nao pode ser alterado."
          />
        )}

        <Select
          label="Perfil"
          value={role}
          onChange={(e) => setRole(e.target.value as UserRole)}
          options={ROLE_OPTIONS}
          required
        />

        <Select
          label="Vendedor vinculado (opcional)"
          value={idVendedor}
          onChange={(e) => setIdVendedor(e.target.value)}
          options={vendedorOptions}
          disabled={loadingVendedores || !!vendedoresError}
          error={
            vendedoresError
              ? "Nao foi possivel carregar a lista de vendedores."
              : undefined
          }
        />

        {mode === "create" && (
          <Alert variant="info">
            Uma senha aleatoria sera gerada e enviada por email para o usuario
            no endereco cadastrado. Ele devera troca-la no primeiro acesso.
          </Alert>
        )}

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button type="submit" loading={submitting}>
            {mode === "create" ? "Criar usuario" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
