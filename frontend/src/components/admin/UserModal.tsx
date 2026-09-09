"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { User, UserRole } from "@/lib/types";

interface UserModalProps {
  open: boolean;
  mode: "create" | "edit";
  user: User | null;
  onClose: () => void;
  onSubmit: (data: {
    nome: string;
    email: string;
    role: UserRole;
    senha?: string;
  }) => Promise<void>;
}

const ROLE_OPTIONS = [
  { value: "admin", label: "Administrador" },
  { value: "user", label: "Usuario" },
  { value: "vendedor", label: "Vendedor" },
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
  const [role, setRole] = useState<UserRole>("user");
  const [senha, setSenha] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setError(null);
      setSubmitting(false);
      if (mode === "edit" && user) {
        setNome(user.nome);
        setEmail(user.email);
        setRole(user.role);
        setSenha("");
      } else {
        setNome("");
        setEmail("");
        setRole("user");
        setSenha("");
      }
    }
  }, [open, mode, user]);

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
    if (mode === "create" && !senha.trim()) {
      setError("Senha inicial e obrigatoria.");
      return;
    }

    setSubmitting(true);
    try {
      await onSubmit({
        nome: nome.trim(),
        email: email.trim(),
        role,
        senha: senha.trim() || undefined,
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

        {mode === "create" && (
          <Input
            label="Senha inicial"
            type="password"
            value={senha}
            onChange={(e) => setSenha(e.target.value)}
            placeholder="Defina uma senha temporaria"
            required
            helperText="O usuario podera altera-la no primeiro acesso."
          />
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
