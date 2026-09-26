/**
 * apiClient.ts - Cliente HTTP com interceptador de 401 + refresh automático
 *
 * access_token e refresh_token são cookies HttpOnly definidos pelo backend:
 * o JS nunca os lê nem os envia manualmente — o navegador os anexa sozinho
 * em toda requisição feita com `credentials: "include"`.
 *
 * Fluxo:
 * 1. fetchWithAuth() chama a API com credentials: "include" (cookie vai sozinho)
 * 2. Se resposta 401 -> tenta POST /api/auth/refresh (também via cookie)
 * 3. Se refresh OK -> backend seta novos cookies e reenviamos a requisição original
 * 4. Se refresh falhar -> redireciona para /login. Exceção (SEC-02,
 *    multi-aba): se o refresh voltar 401, outra aba pode já ter renovado os
 *    cookies; a requisição original é repetida UMA vez e só um novo 401
 *    leva ao logout. 429 não repete e desloga. FE-07: timeout/erro de rede
 *    do próprio refresh (ou falha do Web Lock) NÃO desloga: rejeita com
 *    NetworkError (fila inclusa) e a próxima chamada refaz o refresh.
 * 5. Lock/fila garante que apenas uma chamada de refresh aconteça por vez
 *    na aba; entre abas, o refresh é serializado por Web Locks
 *    (`navigator.locks`, "rp-auth-refresh") quando o navegador suporta
 * 6. Erros HTTP viram ApiError (com status). O 403 de vendedor desligado
 *    NÃO dispara refresh nem logout: só notifica a sessão em memória.
 * 7. FE-08: falha de rede (fetch rejeitado: "Failed to fetch", "Load failed",
 *    "NetworkError when attempting to fetch resource"...) ou timeout vira
 *    NetworkError com mensagem amigável em português (o erro original fica
 *    em `cause`). Abort intencional do chamador passa sem conversão.
 */

import { clearTokens } from "./auth";
import { ApiError } from "./apiError";
import {
  isVendedorDesligadoResponse,
  notifyVendedorDesligado,
} from "./vendedorDesligado";

export { ApiError };

// ============================================
// Configuração
// ============================================

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Tempo máximo de espera pelo refresh (ms)
const REFRESH_TIMEOUT = 10000;

// ============================================
// Lock para refresh - evita chamadas simultâneas
// ============================================

let isRefreshing = false;

/**
 * Callback da fila de requisições que aguardam o refresh. Em falha, `erro`
 * diz o motivo; sem ele, a falha é de sessão (houve logout).
 */
type RefreshSubscriber = (success: boolean, erro?: Error) => void;

let refreshSubscribers: RefreshSubscriber[] = [];

// Mensagem usada apenas quando a sessão de fato é encerrada (logout).
const MSG_SESSAO_EXPIRADA = "Sessão expirada. Faça login novamente.";

// Fallback para rejeições que não são instâncias de Error.
const MSG_ERRO_REQUISICAO = "Erro ao processar requisição";

// ============================================
// Erro de rede (FE-08)
// ============================================

/** Mensagem exibida quando a requisição não chega ao servidor. */
export const MSG_ERRO_REDE =
  "Não foi possível conectar ao servidor. Verifique sua conexão com a internet e tente novamente.";

/** Mensagem exibida quando a requisição expira (timeout). */
export const MSG_ERRO_TIMEOUT = "O servidor demorou para responder. Tente novamente.";

export type NetworkErrorKind = "conexao" | "timeout";

/**
 * Falha de rede/timeout de uma requisição do apiClient: não houve resposta
 * HTTP. A mensagem é amigável; o erro original do navegador fica em `cause`.
 */
export class NetworkError extends Error {
  readonly kind: NetworkErrorKind;

  constructor(kind: NetworkErrorKind, cause?: unknown) {
    super(kind === "timeout" ? MSG_ERRO_TIMEOUT : MSG_ERRO_REDE);
    this.name = "NetworkError";
    this.kind = kind;
    // Atribuição explícita: navegadores antigos ignoram o 2º argumento de Error.
    this.cause = cause;
  }
}

// Nome do erro (Error ou DOMException) sem depender de `instanceof DOMException`.
function errorName(err: unknown): string | undefined {
  if (typeof err !== "object" || err === null) return undefined;
  const name = (err as { name?: unknown }).name;
  return typeof name === "string" ? name : undefined;
}

/**
 * Converte a rejeição de `fetch` no erro que a tela deve ver:
 * - TimeoutError (ex.: AbortSignal.timeout do chamador) -> NetworkError "timeout";
 * - abort intencional (signal do chamador abortado ou AbortError) -> original;
 * - qualquer outra rejeição (fetch só rejeita por falha de rede) -> NetworkError.
 */
function toFetchError(err: unknown, signal?: AbortSignal | null): unknown {
  const name = errorName(err);
  if (name === "TimeoutError") return new NetworkError("timeout", err);
  if (name === "AbortError" || signal?.aborted) return err;
  return new NetworkError("conexao", err);
}

function subscribeTokenRefresh(callback: RefreshSubscriber): void {
  refreshSubscribers.push(callback);
}

function onRefreshComplete(success: boolean, erro?: Error): void {
  refreshSubscribers.forEach((callback) => callback(success, erro));
  refreshSubscribers = [];
}

// ============================================
// API Call para refresh
// ============================================

/**
 * Resultado do POST /api/auth/refresh (SEC-02): o status HTTP, ou um
 * NetworkError quando nao houve resposta (FE-07: falha de rede, timeout do
 * proprio refresh ou falha do Web Lock). Quem chama decide o que fazer com
 * cada caso (ex.: 401 pode significar que outra aba ja renovou; NetworkError
 * e transitorio e nao desloga).
 */
export type RefreshStatus = number | NetworkError;

function refreshOk(status: RefreshStatus): boolean {
  return typeof status === "number" && status >= 200 && status < 300;
}

async function callRefreshToken(): Promise<RefreshStatus> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), REFRESH_TIMEOUT);
  try {
    // refresh_token vai via cookie HttpOnly (credentials: "include");
    // backend responde com novos Set-Cookie para access_token/refresh_token.
    const res = await fetch(`${API_BASE}/api/auth/refresh`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      credentials: "include",
      signal: controller.signal,
    });

    if (!res.ok) {
      console.warn("[apiClient] Refresh falhou com status:", res.status);
    }

    return res.status;
  } catch (error) {
    // FE-07: sem resposta HTTP nao ha como saber se a sessao expirou; o
    // abort aqui so vem do timeout do proprio refresh.
    const name = errorName(error);
    if (name === "AbortError" || name === "TimeoutError") {
      console.warn("[apiClient] Refresh timeout");
      return new NetworkError("timeout", error);
    }
    console.error("[apiClient] Erro no refresh:", error);
    return new NetworkError("conexao", error);
  } finally {
    clearTimeout(timeoutId);
  }
}

// Nome do Web Lock que serializa o refresh entre abas do mesmo navegador.
const REFRESH_LOCK = "rp-auth-refresh";

/**
 * SEC-02 (multi-aba): os cookies sao compartilhados entre as abas, entao
 * duas abas renovando ao mesmo tempo com o mesmo refresh_token geram um 401
 * na segunda (o backend revoga o token no primeiro uso). Quando o navegador
 * suporta Web Locks, o refresh e serializado entre as abas: a segunda so
 * chama o refresh depois que a primeira terminou (com o cookie ja novo).
 * Sem suporte, chama direto; o retry apos o 401 do refresh (fetchWithAuth)
 * cobre a corrida.
 */
async function refreshSerializado(): Promise<RefreshStatus> {
  const locks =
    typeof navigator !== "undefined"
      ? (navigator as Navigator & { locks?: LockManager }).locks
      : undefined;
  if (locks && typeof locks.request === "function") {
    try {
      return await locks.request(REFRESH_LOCK, () => callRefreshToken());
    } catch (err) {
      // FE-07: falha do proprio Web Lock e transitoria (nao diz nada sobre a
      // sessao): nao desloga, a proxima chamada tenta de novo.
      console.error("[apiClient] Falha no Web Lock do refresh:", err);
      return new NetworkError("conexao", err);
    }
  }
  return callRefreshToken();
}

// ============================================
// Indicador visual de refresh (opcional)
// ============================================

let refreshIndicatorTimeout: ReturnType<typeof setTimeout> | null = null;

function showRefreshIndicator(): void {
  if (typeof window === "undefined") return;

  // Cria/mostra indicador se não existir
  let indicator = document.getElementById("api-refresh-indicator");
  if (!indicator) {
    indicator = document.createElement("div");
    indicator.id = "api-refresh-indicator";
    indicator.style.cssText = `
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      height: 3px;
      background: linear-gradient(90deg, #16A34A, #22C55E);
      z-index: 99999;
      animation: apiRefreshPulse 1s ease-in-out infinite;
    `;
    const style = document.createElement("style");
    style.id = "api-refresh-indicator-style";
    style.textContent = `
      @keyframes apiRefreshPulse {
        0%, 100% { opacity: 0.5; }
        50% { opacity: 1; }
      }
    `;
    document.head.appendChild(style);
    document.body.appendChild(indicator);
  }

  indicator.style.display = "block";

  // Remove após 3s se não houver nova chamada
  if (refreshIndicatorTimeout) clearTimeout(refreshIndicatorTimeout);
  refreshIndicatorTimeout = setTimeout(() => {
    const el = document.getElementById("api-refresh-indicator");
    if (el) el.style.display = "none";
  }, 3000);
}

function hideRefreshIndicator(): void {
  if (refreshIndicatorTimeout) {
    clearTimeout(refreshIndicatorTimeout);
    refreshIndicatorTimeout = null;
  }
  const indicator = document.getElementById("api-refresh-indicator");
  if (indicator) indicator.style.display = "none";
}

// ============================================
// Fetch principal com interceptador de 401
// ============================================

export interface ApiClientOptions extends RequestInit {
  /** Se true, não tenta refresh em caso de 401 (ex: login) */
  noRefresh?: boolean;
  /** Headers adicionais */
  headers?: HeadersInit;
  /**
   * Se true, devolve o corpo JSON inteiro, sem desenvelopar `data`. Usado
   * pelas listagens paginadas ({data, pagination} / {data, page, ...}).
   * Prefira `fetchEnvelopeWithAuth`.
   */
  keepEnvelope?: boolean;
}

// Respostas de requisicoes feitas com `keepEnvelope` (ver makeRequest).
const respostasComEnvelope = new WeakSet<Response>();

/**
 * Igual ao `fetchWithAuth` (mesmo refresh automatico no 401 e mesmo
 * tratamento de erro), mas devolve o corpo inteiro da resposta, sem
 * desenvelopar `data`. As listagens paginadas precisam da paginacao que vem
 * junto com `data`.
 */
export function fetchEnvelopeWithAuth<T = unknown>(
  endpoint: string,
  options: ApiClientOptions = {}
): Promise<T> {
  return fetchWithAuth<T>(endpoint, { ...options, keepEnvelope: true });
}

export async function fetchWithAuth<T = unknown>(
  endpoint: string,
  options: ApiClientOptions = {}
): Promise<T> {
  const {
    noRefresh = false,
    headers: customHeaders,
    keepEnvelope = false,
    ...fetchOptions
  } = options;

  const makeRequest = async (): Promise<Response> => {
    const url = endpoint.startsWith("http") ? endpoint : `${API_BASE}${endpoint}`;

    const defaultHeaders: HeadersInit = {
      "Content-Type": "application/json",
    };

    // Merge com headers customizados
    const mergedHeaders = { ...defaultHeaders, ...customHeaders };

    // access_token vai via cookie HttpOnly — o navegador o anexa sozinho.
    // FE-08: falha de rede/timeout vira NetworkError (vale para a requisição
    // original, para o reenvio após o refresh e para as da fila).
    let res: Response;
    try {
      res = await fetch(url, {
        ...fetchOptions,
        headers: mergedHeaders,
        credentials: "include",
      });
    } catch (err) {
      throw toFetchError(err, fetchOptions.signal);
    }
    // FE-05: marca a resposta para o parseResponse devolver o corpo inteiro
    // (envelope paginado). Vale tambem para o reenvio apos o refresh.
    if (keepEnvelope) respostasComEnvelope.add(res);
    return res;
  };

  // Primeira tentativa
  const response = await makeRequest();

  // Se 401 e pode fazer refresh
  if (response.status === 401 && !noRefresh) {
    // Se já está fazendo refresh, aguarda resultado
    if (isRefreshing) {
      return new Promise((resolve, reject) => {
        subscribeTokenRefresh(async (success, erro) => {
          if (!success) {
            // FE-06: sem `erro` houve logout (sessão expirada); com `erro`
            // (ex.: falha de rede no retry) a sessão continua e a fila
            // recebe o mesmo erro da requisição que falhou.
            reject(erro ?? new Error(MSG_SESSAO_EXPIRADA));
            return;
          }
          try {
            const retryResponse = await makeRequest();
            if (!retryResponse.ok) {
              reject(await toApiError(retryResponse));
              return;
            }
            const data = await parseResponse<T>(retryResponse);
            resolve(data);
          } catch (err) {
            reject(err);
          }
        });
      });
    }

    // Inicia refresh
    isRefreshing = true;
    showRefreshIndicator();

    try {
      const status = await refreshSerializado();

      if (refreshOk(status)) {
        // Notifica todas as requisições pendentes
        onRefreshComplete(true);
        isRefreshing = false;
        hideRefreshIndicator();

        // Reenvia requisição original (cookies já atualizados pelo backend)
        const retryResponse = await makeRequest();

        if (!retryResponse.ok) {
          throw await toApiError(retryResponse);
        }

        return parseResponse<T>(retryResponse);
      }

      // FE-07 (opcao A): falha de rede/timeout do proprio refresh (ou do Web
      // Lock) nao desloga — mesmo padrao do retry (FE-06/FE-08). A fila recebe
      // o mesmo NetworkError, o estado e liberado e a proxima chamada refaz o
      // refresh. Sem clearTokens, redirect ou /api/auth/logout.
      if (status instanceof NetworkError) {
        onRefreshComplete(false, status);
        isRefreshing = false;
        hideRefreshIndicator();
        throw status;
      }

      // SEC-02 (multi-aba): refresh com 401 pode significar que outra aba ja
      // renovou (o refresh_token do cookie foi rotacionado e o antigo, que
      // esta aba enviou, foi revogado). Os cookies novos valem para esta aba
      // tambem: repete a requisicao original UMA vez antes de deslogar.
      // 429 (rate limit), timeout e erro de rede NAO repetem.
      let retryAposOutraAba: Response | null = null;
      if (status === 401) {
        try {
          retryAposOutraAba = await makeRequest();
        } catch (err) {
          // Falha de rede no retry: nao desloga, mas libera a fila. FE-06: as
          // pendentes recebem o mesmo erro de rede (nao "Sessão expirada",
          // pois nao houve logout). FE-08: esse erro ja e o NetworkError
          // gerado em makeRequest.
          const erroRede =
            err instanceof Error ? err : new Error(MSG_ERRO_REQUISICAO);
          onRefreshComplete(false, erroRede);
          isRefreshing = false;
          hideRefreshIndicator();
          throw erroRede;
        }
      }

      if (retryAposOutraAba && retryAposOutraAba.status !== 401) {
        // Sessao valida: libera a fila (as pendentes repetem com o cookie novo).
        onRefreshComplete(true);
        isRefreshing = false;
        hideRefreshIndicator();

        if (!retryAposOutraAba.ok) {
          throw await toApiError(retryAposOutraAba);
        }
        return parseResponse<T>(retryAposOutraAba);
      }

      // Refresh respondeu com erro HTTP (e, se foi 401, o retry tambem deu
      // 401): logout.
      onRefreshComplete(false);
      isRefreshing = false;
      hideRefreshIndicator();
      clearTokens();
      redirectToLogin();
      throw new Error(MSG_SESSAO_EXPIRADA);
    } catch (error) {
      isRefreshing = false;
      hideRefreshIndicator();

      if (error instanceof Error) {
        throw error;
      }
      throw new Error(MSG_ERRO_REQUISICAO);
    }
  }

  // Response não é 401 ou não pode fazer refresh. Inclui o 403 de vendedor
  // desligado, que nunca passa pelo refresh/logout acima (só 401 passa).
  if (!response.ok) {
    throw await toApiError(response);
  }

  return parseResponse<T>(response);
}

// ============================================
// Helpers para parsing de response
// ============================================

async function parseResponse<T>(res: Response): Promise<T> {
  const text = await res.text();
  let data: unknown = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = text;
  }

  // Desenvelope: {data: ...} -> ... (exceto quando pedido o envelope inteiro)
  if (respostasComEnvelope.has(res)) return data as T;
  if (data && typeof data === "object" && "data" in data) {
    return (data as { data: T }).data;
  }

  return data as T;
}

/**
 * Monta o ApiError de uma resposta não-OK. Exportado para as listagens de
 * api.ts que fazem fetch manual (envelope paginado). Se for o 403 de
 * vendedor desligado, notifica a sessão (sem refresh/logout).
 */
export function buildApiError(status: number, message: string): ApiError {
  if (isVendedorDesligadoResponse(status, message)) {
    notifyVendedorDesligado();
  }
  return new ApiError(status, message);
}

/** Converte uma resposta não-OK em ApiError (status preservado). */
async function toApiError(res: Response): Promise<ApiError> {
  return buildApiError(res.status, await parseError(res));
}

async function parseError(res: Response): Promise<string> {
  const text = await res.text();
  let data: unknown = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = text;
  }

  if (data && typeof data === "object") {
    const d = data as Record<string, unknown>;
    if ("error" in d) return String(d.error);
    if ("message" in d) return String(d.message);
  }

  return `Erro ${res.status}: ${res.statusText}`;
}

// ============================================
// Redirect para login
// ============================================

let redirecting = false;

function redirectToLogin(): void {
  if (redirecting || typeof window === "undefined") return;
  redirecting = true;

  // Limpa estado local e pede ao backend para limpar os cookies HttpOnly.
  clearTokens();

  const currentUrl = window.location.href;
  const loginUrl = currentUrl.includes("/login")
    ? "/login"
    : `/login?redirect=${encodeURIComponent(currentUrl)}`;

  fetch(`${API_BASE}/api/auth/logout`, {
    method: "POST",
    credentials: "include",
  }).finally(() => {
    window.location.href = loginUrl;
  });
}

// ============================================
// Função para forçar logout (útil para erros críticos)
// ============================================

export function forceLogout(message = "Sessão expirada"): void {
  clearTokens();
  if (typeof window !== "undefined") {
    alert(message);
    fetch(`${API_BASE}/api/auth/logout`, {
      method: "POST",
      credentials: "include",
    }).finally(() => {
      // Intencional: modulo fora de componente (sem useRouter) e o logout precisa de
      // recarga completa para descartar sessao em memoria, caches e estado do React.
      // eslint-disable-next-line @next/next/no-location-assign-relative-destination
      window.location.href = "/login";
    });
  }
}
