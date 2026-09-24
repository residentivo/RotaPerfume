/**
 * session.ts - Usuario da sessao em memoria (fonte da verdade: /api/auth/me).
 *
 * O localStorage (auth.ts) guarda so um cache nao sensivel do User para
 * decisoes de UI antes da validacao. Dados que podem mudar sem novo login
 * (ex.: id_vendedor alterado pelo admin, vendedor desligado) sao lidos daqui:
 * - o ProtectedRoute revalida ao montar e quando a aba volta ao foco;
 * - o 403 de vendedor desligado (apiClient) marca o bloqueio e rebusca /me.
 *
 * `vendedor_desligado` e o flag de bloqueio existem apenas em memoria.
 */

import { useSyncExternalStore } from "react";
import { apiMe } from "./api";
import { saveUser } from "./auth";
import { MeResponse } from "./types";
import { subscribeVendedorDesligado } from "./vendedorDesligado";

interface SessionState {
  user: MeResponse | null;
  /**
   * true quando alguma rota da carteira respondeu o 403 de vendedor
   * desligado. So e limpo por uma revalidacao "normal" (montagem/foco) em
   * que /me devolva explicitamente vendedor_desligado:false — nunca pela
   * rebusca disparada pelo proprio 403, para nao gerar loop de requisicoes.
   */
  bloqueado403: boolean;
}

let state: SessionState = { user: null, bloqueado403: false };
const listeners = new Set<() => void>();
let inflight: Promise<MeResponse> | null = null;

function setState(next: Partial<SessionState>): void {
  state = { ...state, ...next };
  listeners.forEach((l) => l());
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getSnapshot(): SessionState {
  return state;
}

const SERVER_SNAPSHOT: SessionState = { user: null, bloqueado403: false };
function getServerSnapshot(): SessionState {
  return SERVER_SNAPSHOT;
}

/** Leitura nao reativa (para handlers/efeitos que nao devem re-disparar). */
export function getSessionUser(): MeResponse | null {
  return state.user;
}

/**
 * Busca /api/auth/me e atualiza a sessao em memoria + o cache (whitelist) do
 * localStorage. Chamadas concorrentes compartilham a mesma requisicao.
 * Erros sao propagados ao chamador (ex.: ProtectedRoute decide o logout).
 */
export function refreshSessionUser(
  opts: { origem403?: boolean } = {}
): Promise<MeResponse> {
  if (!inflight) {
    inflight = apiMe()
      .then((user) => {
        saveUser(user);
        const next: Partial<SessionState> = { user };
        if (!opts.origem403 && user.vendedor_desligado === false) {
          next.bloqueado403 = false;
        }
        setState(next);
        return user;
      })
      .finally(() => {
        inflight = null;
      });
  }
  return inflight;
}

export function clearSession(): void {
  setState({ user: null, bloqueado403: false });
}

// 403 "acesso bloqueado: vendedor desligado" recebido em qualquer rota:
// marca o bloqueio e rebusca /me (sem refresh de token nem logout).
if (typeof window !== "undefined") {
  subscribeVendedorDesligado(() => {
    if (!state.bloqueado403) setState({ bloqueado403: true });
    refreshSessionUser({ origem403: true }).catch(() => {
      // Falha ao rebuscar /me nao muda o bloqueio ja detectado.
    });
  });
}

/** Usuario da sessao (reativo). null antes da primeira validacao. */
export function useSessionUser(): MeResponse | null {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot).user;
}

/**
 * true quando o usuario `normal` esta vinculado a vendedor desligado, seja
 * pelo /me (proativo) ou pelo 403 recebido de alguma rota da carteira.
 * Admin nunca e bloqueado.
 */
export function useVendedorDesligado(): boolean {
  const s = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  if (s.user?.role === "admin") return false;
  return s.user?.vendedor_desligado === true || s.bloqueado403;
}
