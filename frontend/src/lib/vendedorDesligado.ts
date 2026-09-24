/**
 * vendedorDesligado.ts - Contrato do bloqueio de vendedor desligado.
 *
 * Contrato definido pelo SecBrain: para usuario `normal` vinculado a um
 * vendedor com data de desligamento, o backend responde
 *   403 {"success":false,"error":"acesso bloqueado: vendedor desligado"}
 * em todas as rotas da carteira (clientes, pedidos, pagamentos,
 * oportunidades, visitas e /api/vendedores/{id}/clientes). Login e Dashboard
 * continuam acessiveis.
 *
 * A deteccao e feita por igualdade EXATA da mensagem + status 403 (sem
 * includes/regex), para nao confundir com outros 403.
 */

import { ApiError } from "./apiError";

export const MSG_VENDEDOR_DESLIGADO = "acesso bloqueado: vendedor desligado";

/** Texto exibido ao usuario nas telas da carteira bloqueadas. */
export const AVISO_VENDEDOR_DESLIGADO =
  "Seu vendedor foi desligado; o acesso aos dados da carteira está bloqueado.";

/** Deteccao a partir do status + mensagem crus (usada pelo apiClient). */
export function isVendedorDesligadoResponse(status: number, message: string): boolean {
  return status === 403 && message === MSG_VENDEDOR_DESLIGADO;
}

// ============================================
// Notificacao: o apiClient avisa quando recebe o 403 de vendedor desligado
// para que a sessao em memoria (session.ts) marque o bloqueio e rebusque
// GET /api/auth/me. Mantido aqui (e nao no apiClient/session) para evitar
// import circular entre apiClient -> session -> api -> apiClient.
// ============================================

type Listener = () => void;
const listeners = new Set<Listener>();

export function subscribeVendedorDesligado(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function notifyVendedorDesligado(): void {
  listeners.forEach((l) => l());
}

export function isVendedorDesligadoError(err: unknown): boolean {
  return (
    err instanceof ApiError &&
    err.status === 403 &&
    err.message === MSG_VENDEDOR_DESLIGADO
  );
}
