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
 *
 * LOG-02: logs verbose aqui nunca recebem o objeto User — so id, papel e
 * booleanos.
 */

import { useSyncExternalStore } from "react";
import { apiMe } from "./api";
import { saveUser } from "./auth";
import { MeResponse } from "./types";
import { subscribeVendedorDesligado } from "./vendedorDesligado";
import { vlog } from "./vlog";

const F = "session.ts";

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
  vlog(F, "setState", "mesclando novo estado da sessão");
  state = { ...state, ...next };
  vlog(F, "setState", "notificando listeners da sessão:", listeners.size);
  listeners.forEach((l) => l());
}

function subscribe(listener: () => void): () => void {
  vlog(F, "subscribe", "registrando listener da sessão");
  listeners.add(listener);
  return () => {
    vlog(F, "subscribe.func", "removendo listener da sessão");
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
  vlog(F, "runMe", "incrementando relógio lógico da sessão");
  const mySeq = ++seqCounter;
  vlog(F, "runMe", "atribuindo seq ao /me em voo:", mySeq);
  entry.seq = mySeq;
  vlog(F, "runMe", "chamando GET /api/auth/me, origem403:", entry.origem403);
  return apiMe().then((user) => {
    vlog(F, "runMe.func", "salvando cache do usuário, id/papel:", user.id, user.role);
    saveUser(user);
    vlog(F, "runMe.func", "preparando novo estado com o usuário");
    const next: Partial<SessionState> = { user };
    vlog(F, "runMe.func", "verificando se pode limpar bloqueio 403", mySeq, seqBloqueio, user.vendedor_desligado === false);
    if (
      !entry.origem403 &&
      mySeq > seqBloqueio &&
      user.vendedor_desligado === false
    ) {
      vlog(F, "runMe.func", "limpando bloqueio 403");
      next.bloqueado403 = false;
    }
    vlog(F, "runMe.func", "atualizando estado da sessão");
    setState(next);
    return user;
  });
}

function track(entry: Inflight, promise: Promise<MeResponse>): Promise<MeResponse> {
  vlog(F, "track", "registrando promise do /me com limpeza ao finalizar");
  entry.promise = promise.finally(() => {
    vlog(F, "track.func", "liberando /me em voo se ainda for o atual");
    if (inflight === entry) inflight = null;
  });
  vlog(F, "track", "marcando /me como em voo");
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
  vlog(F, "refreshSessionUser", "lendo origem da revalidação");
  const origem403 = opts.origem403 === true;
  vlog(F, "refreshSessionUser", "lendo /me em voo");
  const atual = inflight;

  vlog(F, "refreshSessionUser", "verificando se há /me em voo:", !!atual, "origem403:", origem403);
  if (atual) {
    vlog(F, "refreshSessionUser", "verificando se é origem 403 para reaproveitar");
    if (origem403) return atual.promise;
    vlog(F, "refreshSessionUser", "calculando se pode reaproveitar /me em voo");
    const podeReaproveitar =
      !atual.origem403 && (atual.seq === null || atual.seq > seqBloqueio);
    vlog(F, "refreshSessionUser", "verificando se pode reaproveitar:", podeReaproveitar);
    if (podeReaproveitar) return atual.promise;

    vlog(F, "refreshSessionUser", "criando /me encadeado após o atual");
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

  vlog(F, "refreshSessionUser", "criando novo /me");
  const entry: Inflight = {
    promise: undefined as unknown as Promise<MeResponse>,
    origem403,
    seq: null,
  };
  return track(entry, runMe(entry));
}

export function clearSession(): void {
  vlog(F, "clearSession", "limpando sessão em memória");
  setState({ user: null, bloqueado403: false });
}

// 403 "acesso bloqueado: vendedor desligado" recebido em qualquer rota:
// marca o bloqueio (gravando seqBloqueio) e rebusca /me (sem refresh de
// token nem logout).
if (typeof window !== "undefined") {
  subscribeVendedorDesligado(() => {
    vlog(F, "subscribeVendedorDesligado.func", "registrando seq do bloqueio 403");
    seqBloqueio = ++seqCounter;
    vlog(F, "subscribeVendedorDesligado.func", "marcando bloqueio 403 se ainda não marcado:", state.bloqueado403);
    if (!state.bloqueado403) setState({ bloqueado403: true });
    vlog(F, "subscribeVendedorDesligado.func", "rebuscando /me com origem 403");
    refreshSessionUser({ origem403: true }).catch(() => {
      // Falha ao rebuscar /me nao muda o bloqueio ja detectado.
      vlog(F, "subscribeVendedorDesligado.func", "falha ao rebuscar /me após 403 (bloqueio mantido)");
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
  vlog(F, "useVendedorDesligado", "lendo snapshot da sessão");
  const s = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  vlog(F, "useVendedorDesligado", "verificando se é admin");
  if (s.user?.role === "admin") return false;
  return s.user?.vendedor_desligado === true || s.bloqueado403;
}
