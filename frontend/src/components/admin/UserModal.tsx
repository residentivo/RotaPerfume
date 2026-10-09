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
import { vlog } from "@/lib/vlog";

const F = "UserModal.tsx";

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
  vlog(F, "UserModal", "criando estado nome, modo:", mode);
  const [nome, setNome] = useState("");
  vlog(F, "UserModal", "criando estado email");
  const [email, setEmail] = useState("");
  vlog(F, "UserModal", "criando estado role");
  const [role, setRole] = useState<UserRole>("normal");
  vlog(F, "UserModal", "criando estado idVendedor");
  const [idVendedor, setIdVendedor] = useState("");
  vlog(F, "UserModal", "criando estado submitting");
  const [submitting, setSubmitting] = useState(false);
  vlog(F, "UserModal", "criando estado error");
  const [error, setError] = useState<string | null>(null);

  vlog(F, "UserModal", "criando estado vendedores");
  const [vendedores, setVendedores] = useState<Vendedor[]>([]);
  vlog(F, "UserModal", "criando estado vendedoresError");
  const [vendedoresError, setVendedoresError] = useState<string | null>(null);
  vlog(F, "UserModal", "criando estado loadingVendedores");
  const [loadingVendedores, setLoadingVendedores] = useState(false);

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  vlog(F, "UserModal", "registrando reset do formulário ao abrir");
  useResetOnOpen(open, [mode, user], () => {
    vlog(F, "UserModal.reset", "limpando erro");
    setError(null);
    vlog(F, "UserModal.reset", "limpando estado de envio");
    setSubmitting(false);
    vlog(F, "UserModal.reset", "verificando se é edição com usuário, id:", user?.id);
    if (mode === "edit" && user) {
      vlog(F, "UserModal.reset", "preenchendo nome");
      setNome(user.nome);
      // LOG-02: e-mail nunca vai para o log.
      vlog(F, "UserModal.reset", "preenchendo e-mail");
      setEmail(user.email);
      vlog(F, "UserModal.reset", "preenchendo papel:", user.role);
      setRole(user.role);
      vlog(F, "UserModal.reset", "preenchendo vendedor vinculado, id:", user.id_vendedor);
      setIdVendedor(
        user.id_vendedor !== null && user.id_vendedor !== undefined
          ? String(user.id_vendedor)
          : ""
      );
    } else {
      vlog(F, "UserModal.reset", "limpando nome");
      setNome("");
      vlog(F, "UserModal.reset", "limpando e-mail");
      setEmail("");
      vlog(F, "UserModal.reset", "definindo papel padrão");
      setRole("normal");
      vlog(F, "UserModal.reset", "limpando vendedor vinculado");
      setIdVendedor("");
    }
  });

  // Parte sincrona (liga o loading / limpa o erro) roda durante o render
  // ao abrir; o efeito so busca e aplica o resultado nos callbacks.
  vlog(F, "UserModal", "registrando reset do carregamento de vendedores");
  useResetOnOpen(open, [], () => {
    vlog(F, "UserModal.resetVendedores", "ligando loading de vendedores");
    setLoadingVendedores(true);
    vlog(F, "UserModal.resetVendedores", "limpando erro de vendedores");
    setVendedoresError(null);
  });

  vlog(F, "UserModal", "registrando efeito de carga de vendedores");
  useEffect(() => {
    vlog(F, "UserModal.useEffect", "verificando se o modal está aberto:", open);
    if (!open) return;
    vlog(F, "UserModal.useEffect", "inicializando flag de cancelamento");
    let cancelled = false;
    vlog(F, "UserModal.useEffect", "buscando vendedores");
    apiListVendedores()
      .then((data) => {
        vlog(F, "UserModal.useEffect", "vendedores recebidos, qtd/cancelado:", data.length, cancelled);
        if (!cancelled) setVendedores(data);
      })
      .catch((err) => {
        vlog(F, "UserModal.useEffect", "falha ao buscar vendedores, cancelado:", cancelled);
        if (!cancelled) {
          vlog(F, "UserModal.useEffect", "montando mensagem de erro de vendedores");
          const message =
            err instanceof Error
              ? err.message
              : "Erro ao carregar vendedores.";
          vlog(F, "UserModal.useEffect", "exibindo erro de vendedores");
          setVendedoresError(message);
        }
      })
      .finally(() => {
        vlog(F, "UserModal.useEffect", "desligando loading de vendedores se não cancelado");
        if (!cancelled) setLoadingVendedores(false);
      });
    return () => {
      vlog(F, "UserModal.useEffect", "cleanup: cancelando carga de vendedores");
      cancelled = true;
    };
  }, [open]);

  vlog(F, "UserModal", "montando opções de vendedor, qtd:", vendedores.length);
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
    vlog(F, "UserModal.handleSubmit", "impedindo submit padrão do form");
    e.preventDefault();
    vlog(F, "UserModal.handleSubmit", "limpando erro");
    setError(null);

    vlog(F, "UserModal.handleSubmit", "validando nome");
    if (!nome.trim()) {
      vlog(F, "UserModal.handleSubmit", "nome ausente");
      setError("Nome e obrigatorio.");
      return;
    }
    vlog(F, "UserModal.handleSubmit", "validando e-mail na criação");
    if (mode === "create" && !email.trim()) {
      vlog(F, "UserModal.handleSubmit", "e-mail ausente");
      setError("Email e obrigatorio.");
      return;
    }

    vlog(F, "UserModal.handleSubmit", "marcando envio em andamento");
    setSubmitting(true);
    try {
      vlog(F, "UserModal.handleSubmit", "enviando usuário, modo/papel:", mode, role);
      await onSubmit({
        nome: nome.trim(),
        email: email.trim(),
        role,
        id_vendedor: idVendedor ? Number(idVendedor) : null,
      });
    } catch (err) {
      vlog(F, "UserModal.handleSubmit", "falha ao salvar usuário: montando mensagem de erro");
      const message =
        err instanceof Error ? err.message : "Erro ao salvar usuario.";
      vlog(F, "UserModal.handleSubmit", "exibindo erro");
      setError(message);
    } finally {
      vlog(F, "UserModal.handleSubmit", "finalizando envio");
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

        <div className="flex flex-wrap justify-end gap-2 pt-2">
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
