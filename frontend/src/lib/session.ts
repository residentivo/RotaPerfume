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
   * desligado. So e limpo por uma revalidacao "normal" (montagem/foco),
   * iniciada depois do ultimo 403, em que /me devolva explicitamente
   * vendedor_desligado:false — nunca pela rebusca disparada pelo proprio 403
   * (evita loop de requisicoes). O logout (clearSession) tambem limpa.
   */
  bloqueado403: boolean;
}

let state: SessionState = { user: null, bloqueado403: false };
const listeners = new Set<() => void>();

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
 * Relogio logico da sessao. Cada 403 de vendedor desligado e cada /me
 * iniciado recebem um numero crescente. Invariante (SecBrain, FE-01):
 * `bloqueado403` so e limpo pela resposta de um /me NORMAL que foi INICIADO
 * depois do ultimo 403 (`seq > seqBloqueio`) e que traz
 * `vendedor_desligado === false` explicito. Cache do localStorage,
 * `undefined` ou erro de rede nunca limpam o bloqueio.
 */
let seqCounter = 0;
let seqBloqueio = 0;

interface Inflight {
  promise: Promise<MeResponse>;
  origem403: boolean;
  /** seq do /me; null enquanto a chamada encadeada ainda nao comecou. */
  seq: number | null;
}
let inflight: Inflight | null = null;

function runMe(entry: Inflight): Promise<MeResponse> {
  const mySeq = ++seqCounter;
  entry.seq = mySeq;
  return apiMe().then((user) => {
    saveUser(user);
    const next: Partial<SessionState> = { user };
    if (
      !entry.origem403 &&
      mySeq > seqBloqueio &&
      user.vendedor_desligado === false
    ) {
      next.bloqueado403 = false;
    }
    setState(next);
    return user;
  });
}

function track(entry: Inflight, promise: Promise<MeResponse>): Promise<MeResponse> {
  entry.promise = promise.finally(() => {
    if (inflight === entry) inflight = null;
  });
  inflight = entry;
  return entry.promise;
}

/**
 * Busca /api/auth/me e atualiza a sessao em memoria + o cache (whitelist) do
 * localStorage. Erros sao propagados ao chamador (ex.: ProtectedRoute decide
 * o logout).
 *
 * Compartilhamento de requisicoes concorrentes:
 * - chamada de origem 403 reaproveita qualquer /me em voo (so rebusca dados,
 *   nunca limpa o bloqueio);
 * - chamada normal reaproveita um /me normal em voo apenas se ele ja foi
 *   iniciado depois do ultimo 403 (ou ainda vai iniciar). Se o /me em voo e
 *   de origem 403, ou foi iniciado antes do ultimo 403, encadeia um /me novo
 *   depois dele — assim a revalidacao normal nunca e "engolida" por uma
 *   resposta que nao pode limpar o bloqueio.
 */
export function refreshSessionUser(
  opts: { origem403?: boolean } = {}
): Promise<MeResponse> {
  const origem403 = opts.origem403 === true;
  const atual = inflight;

  if (atual) {
    if (origem403) return atual.promise;
    const podeReaproveitar =
      !atual.origem403 && (atual.seq === null || atual.seq > seqBloqueio);
    if (podeReaproveitar) return atual.promise;

    const encadeada: Inflight = {
      promise: undefined as unknown as Promise<MeResponse>,
      origem403: false,
      seq: null,
    };
    return track(
      encadeada,
      atual.promise.catch(() => undefined).then(() => runMe(encadeada))
    );
  }

  const entry: Inflight = {
    promise: undefined as unknown as Promise<MeResponse>,
    origem403,
    seq: null,
  };
  return track(entry, runMe(entry));
}

export function clearSession(): void {
  setState({ user: null, bloqueado403: false });
}

// 403 "acesso bloqueado: vendedor desligado" recebido em qualquer rota:
// marca o bloqueio (gravando seqBloqueio) e rebusca /me (sem refresh de
// token nem logout).
if (typeof window !== "undefined") {
  subscribeVendedorDesligado(() => {
    seqBloqueio = ++seqCounter;
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
