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
 *
 * LOG-02: os logs verbose daqui registram SOMENTE método, rota sem query
 * string e status HTTP — nunca corpo de request/response, headers ou cookies.
 */

import { clearTokens } from "./auth";
import { ApiError } from "./apiError";
import {
  isVendedorDesligadoResponse,
  notifyVendedorDesligado,
} from "./vendedorDesligado";
import { vlog } from "./vlog";

export { ApiError };

const F = "apiClient.ts";

// ============================================
// Configuração
// ============================================

// Remove barra(s) final(is): NEXT_PUBLIC_API_URL pode vir com sufixo de caminho
// (ex. https://host:8443/api) e os endpoints ja comecam com "/api/...".
const API_BASE = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/+$/, "");

// Tempo máximo de espera pelo refresh (ms)
const REFRESH_TIMEOUT = 10000;

/** LOG-02: rota sem query string (nunca loga filtros/buscas do usuário). */
function rotaSemQuery(endpoint: string): string {
  return endpoint.split("?")[0];
}

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
    vlog(F, "NetworkError.constructor", "definindo name do erro de rede");
    this.name = "NetworkError";
    vlog(F, "NetworkError.constructor", "definindo kind do erro de rede:", kind);
    this.kind = kind;
    // Atribuição explícita: navegadores antigos ignoram o 2º argumento de Error.
    vlog(F, "NetworkError.constructor", "atribuindo cause do erro original");
    this.cause = cause;
  }
}

// Nome do erro (Error ou DOMException) sem depender de `instanceof DOMException`.
function errorName(err: unknown): string | undefined {
  vlog(F, "errorName", "verificando se o erro é um objeto não nulo");
  if (typeof err !== "object" || err === null) return undefined;
  vlog(F, "errorName", "lendo propriedade name do erro");
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
  vlog(F, "toFetchError", "obtendo nome do erro do fetch");
  const name = errorName(err);
  vlog(F, "toFetchError", "verificando se é TimeoutError:", name === "TimeoutError");
  if (name === "TimeoutError") return new NetworkError("timeout", err);
  vlog(F, "toFetchError", "verificando se é abort intencional:", name === "AbortError" || !!signal?.aborted);
  if (name === "AbortError" || signal?.aborted) return err;
  return new NetworkError("conexao", err);
}

function subscribeTokenRefresh(callback: RefreshSubscriber): void {
  vlog(F, "subscribeTokenRefresh", "adicionando requisição à fila de espera do refresh");
  refreshSubscribers.push(callback);
}

function onRefreshComplete(success: boolean, erro?: Error): void {
  vlog(F, "onRefreshComplete", "notificando fila do refresh, pendentes/sucesso:", refreshSubscribers.length, success);
  refreshSubscribers.forEach((callback) => callback(success, erro));
  vlog(F, "onRefreshComplete", "limpando fila do refresh");
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
  vlog(F, "callRefreshToken", "criando AbortController do refresh");
  const controller = new AbortController();
  vlog(F, "callRefreshToken", "agendando timeout do refresh (ms):", REFRESH_TIMEOUT);
  const timeoutId = setTimeout(() => controller.abort(), REFRESH_TIMEOUT);
  try {
    // refresh_token vai via cookie HttpOnly (credentials: "include");
    // backend responde com novos Set-Cookie para access_token/refresh_token.
    vlog(F, "callRefreshToken", "enviando POST /api/auth/refresh");
    const res = await fetch(`${API_BASE}/api/auth/refresh`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      credentials: "include",
      signal: controller.signal,
    });

    vlog(F, "callRefreshToken", "verificando se refresh retornou ok, status:", res.status);
    if (!res.ok) {
      console.warn("[apiClient] Refresh falhou com status:", res.status);
    }

    return res.status;
  } catch (error) {
    // FE-07: sem resposta HTTP nao ha como saber se a sessao expirou; o
    // abort aqui so vem do timeout do proprio refresh.
    vlog(F, "callRefreshToken", "obtendo nome do erro do refresh");
    const name = errorName(error);
    vlog(F, "callRefreshToken", "verificando se o erro do refresh foi timeout/abort");
    if (name === "AbortError" || name === "TimeoutError") {
      console.warn("[apiClient] Refresh timeout");
      return new NetworkError("timeout", error);
    }
    console.error("[apiClient] Erro no refresh:", error);
    return new NetworkError("conexao", error);
  } finally {
    vlog(F, "callRefreshToken", "cancelando timeout do refresh");
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
  vlog(F, "refreshSerializado", "obtendo navigator.locks (Web Locks)");
  const locks =
    typeof navigator !== "undefined"
      ? (navigator as Navigator & { locks?: LockManager }).locks
      : undefined;
  vlog(F, "refreshSerializado", "verificando suporte a Web Locks:", !!locks && typeof locks.request === "function");
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
  vlog(F, "showRefreshIndicator", "verificando se está no navegador");
  if (typeof window === "undefined") return;

  // Cria/mostra indicador se não existir
  vlog(F, "showRefreshIndicator", "buscando elemento do indicador de refresh");
  let indicator = document.getElementById("api-refresh-indicator");
  vlog(F, "showRefreshIndicator", "verificando se o indicador já existe:", !!indicator);
  if (!indicator) {
    vlog(F, "showRefreshIndicator", "criando div do indicador");
    indicator = document.createElement("div");
    vlog(F, "showRefreshIndicator", "definindo id do indicador");
    indicator.id = "api-refresh-indicator";
    vlog(F, "showRefreshIndicator", "definindo estilo do indicador");
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
    vlog(F, "showRefreshIndicator", "criando elemento style da animação");
    const style = document.createElement("style");
    vlog(F, "showRefreshIndicator", "definindo id do style");
    style.id = "api-refresh-indicator-style";
    vlog(F, "showRefreshIndicator", "definindo keyframes da animação");
    style.textContent = `
      @keyframes apiRefreshPulse {
        0%, 100% { opacity: 0.5; }
        50% { opacity: 1; }
      }
    `;
    vlog(F, "showRefreshIndicator", "anexando style ao head");
    document.head.appendChild(style);
    vlog(F, "showRefreshIndicator", "anexando indicador ao body");
    document.body.appendChild(indicator);
  }

  vlog(F, "showRefreshIndicator", "exibindo indicador");
  indicator.style.display = "block";

  // Remove após 3s se não houver nova chamada
  vlog(F, "showRefreshIndicator", "cancelando timeout anterior do indicador, se houver");
  if (refreshIndicatorTimeout) clearTimeout(refreshIndicatorTimeout);
  vlog(F, "showRefreshIndicator", "agendando ocultação do indicador em 3s");
  refreshIndicatorTimeout = setTimeout(() => {
    vlog(F, "showRefreshIndicator.func", "buscando indicador para ocultar");
    const el = document.getElementById("api-refresh-indicator");
    vlog(F, "showRefreshIndicator.func", "ocultando indicador se existir");
    if (el) el.style.display = "none";
  }, 3000);
}

function hideRefreshIndicator(): void {
  vlog(F, "hideRefreshIndicator", "verificando se há timeout pendente do indicador");
  if (refreshIndicatorTimeout) {
    vlog(F, "hideRefreshIndicator", "cancelando timeout do indicador");
    clearTimeout(refreshIndicatorTimeout);
    vlog(F, "hideRefreshIndicator", "zerando referência do timeout");
    refreshIndicatorTimeout = null;
  }
  vlog(F, "hideRefreshIndicator", "buscando elemento do indicador");
  const indicator = document.getElementById("api-refresh-indicator");
  vlog(F, "hideRefreshIndicator", "ocultando indicador se existir");
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
  vlog(F, "fetchWithAuth", "separando opções do apiClient das opções do fetch");
  const {
    noRefresh = false,
    headers: customHeaders,
    keepEnvelope = false,
    ...fetchOptions
  } = options;
  vlog(F, "fetchWithAuth", "calculando método e rota sem query para log");
  const metodo = (fetchOptions.method || "GET").toUpperCase();
  const rota = rotaSemQuery(endpoint);

  const makeRequest = async (): Promise<Response> => {
    vlog(F, "fetchWithAuth.makeRequest", "montando URL completa da requisição");
    const url = endpoint.startsWith("http") ? endpoint : `${API_BASE}${endpoint}`;

    vlog(F, "fetchWithAuth.makeRequest", "definindo headers padrão");
    const defaultHeaders: HeadersInit = {
      "Content-Type": "application/json",
    };

    // Merge com headers customizados
    vlog(F, "fetchWithAuth.makeRequest", "mesclando headers customizados");
    const mergedHeaders = { ...defaultHeaders, ...customHeaders };

    // access_token vai via cookie HttpOnly — o navegador o anexa sozinho.
    // FE-08: falha de rede/timeout vira NetworkError (vale para a requisição
    // original, para o reenvio após o refresh e para as da fila).
    let res: Response;
    try {
      vlog(F, "fetchWithAuth.makeRequest", "enviando requisição", metodo, rota);
      res = await fetch(url, {
        ...fetchOptions,
        headers: mergedHeaders,
        credentials: "include",
      });
    } catch (err) {
      vlog(F, "fetchWithAuth.makeRequest", "fetch rejeitado, convertendo erro de rede", metodo, rota);
      throw toFetchError(err, fetchOptions.signal);
    }
    vlog(F, "fetchWithAuth.makeRequest", "resposta recebida", metodo, rota, res.status);
    // FE-05: marca a resposta para o parseResponse devolver o corpo inteiro
    // (envelope paginado). Vale tambem para o reenvio apos o refresh.
    vlog(F, "fetchWithAuth.makeRequest", "marcando resposta com envelope se pedido:", keepEnvelope);
    if (keepEnvelope) respostasComEnvelope.add(res);
    return res;
  };

  // Primeira tentativa
  vlog(F, "fetchWithAuth", "executando primeira tentativa", metodo, rota);
  const response = await makeRequest();

  // Se 401 e pode fazer refresh
  vlog(F, "fetchWithAuth", "verificando se precisa de refresh (401 e refresh permitido)", response.status, !noRefresh);
  if (response.status === 401 && !noRefresh) {
    // Se já está fazendo refresh, aguarda resultado
    vlog(F, "fetchWithAuth", "verificando se já há refresh em andamento:", isRefreshing);
    if (isRefreshing) {
      return new Promise((resolve, reject) => {
        vlog(F, "fetchWithAuth.func", "inscrevendo requisição na fila do refresh", metodo, rota);
        subscribeTokenRefresh(async (success, erro) => {
          vlog(F, "fetchWithAuth.func", "verificando resultado do refresh para requisição da fila:", success);
          if (!success) {
            // FE-06: sem `erro` houve logout (sessão expirada); com `erro`
            // (ex.: falha de rede no retry) a sessão continua e a fila
            // recebe o mesmo erro da requisição que falhou.
            vlog(F, "fetchWithAuth.func", "rejeitando requisição da fila", metodo, rota);
            reject(erro ?? new Error(MSG_SESSAO_EXPIRADA));
            return;
          }
          try {
            vlog(F, "fetchWithAuth.func", "reenviando requisição da fila", metodo, rota);
            const retryResponse = await makeRequest();
            vlog(F, "fetchWithAuth.func", "verificando se reenvio da fila retornou ok:", retryResponse.status);
            if (!retryResponse.ok) {
              vlog(F, "fetchWithAuth.func", "rejeitando com ApiError do reenvio", retryResponse.status);
              reject(await toApiError(retryResponse));
              return;
            }
            vlog(F, "fetchWithAuth.func", "lendo resposta do reenvio da fila");
            const data = await parseResponse<T>(retryResponse);
            vlog(F, "fetchWithAuth.func", "resolvendo requisição da fila", metodo, rota);
            resolve(data);
          } catch (err) {
            vlog(F, "fetchWithAuth.func", "rejeitando requisição da fila por erro no reenvio", metodo, rota);
            reject(err);
          }
        });
      });
    }

    // Inicia refresh
    vlog(F, "fetchWithAuth", "marcando refresh em andamento");
    isRefreshing = true;
    vlog(F, "fetchWithAuth", "exibindo indicador de refresh");
    showRefreshIndicator();

    try {
      vlog(F, "fetchWithAuth", "executando refresh serializado");
      const status = await refreshSerializado();

      vlog(F, "fetchWithAuth", "verificando se refresh foi bem-sucedido:", refreshOk(status));
      if (refreshOk(status)) {
        // Notifica todas as requisições pendentes
        vlog(F, "fetchWithAuth", "liberando fila com sucesso");
        onRefreshComplete(true);
        vlog(F, "fetchWithAuth", "desmarcando refresh em andamento");
        isRefreshing = false;
        vlog(F, "fetchWithAuth", "ocultando indicador de refresh");
        hideRefreshIndicator();

        // Reenvia requisição original (cookies já atualizados pelo backend)
        vlog(F, "fetchWithAuth", "reenviando requisição original após refresh", metodo, rota);
        const retryResponse = await makeRequest();

        vlog(F, "fetchWithAuth", "verificando se reenvio retornou ok:", retryResponse.status);
        if (!retryResponse.ok) {
          vlog(F, "fetchWithAuth", "lançando ApiError do reenvio", retryResponse.status);
          throw await toApiError(retryResponse);
        }

        return parseResponse<T>(retryResponse);
      }

      // FE-07 (opcao A): falha de rede/timeout do proprio refresh (ou do Web
      // Lock) nao desloga — mesmo padrao do retry (FE-06/FE-08). A fila recebe
      // o mesmo NetworkError, o estado e liberado e a proxima chamada refaz o
      // refresh. Sem clearTokens, redirect ou /api/auth/logout.
      vlog(F, "fetchWithAuth", "verificando se refresh falhou por erro de rede");
      if (status instanceof NetworkError) {
        vlog(F, "fetchWithAuth", "liberando fila com erro de rede do refresh");
        onRefreshComplete(false, status);
        vlog(F, "fetchWithAuth", "desmarcando refresh em andamento");
        isRefreshing = false;
        vlog(F, "fetchWithAuth", "ocultando indicador de refresh");
        hideRefreshIndicator();
        vlog(F, "fetchWithAuth", "lançando erro de rede do refresh, kind:", status.kind);
        throw status;
      }

      // SEC-02 (multi-aba): refresh com 401 pode significar que outra aba ja
      // renovou (o refresh_token do cookie foi rotacionado e o antigo, que
      // esta aba enviou, foi revogado). Os cookies novos valem para esta aba
      // tambem: repete a requisicao original UMA vez antes de deslogar.
      // 429 (rate limit), timeout e erro de rede NAO repetem.
      vlog(F, "fetchWithAuth", "inicializando retry após outra aba");
      let retryAposOutraAba: Response | null = null;
      vlog(F, "fetchWithAuth", "verificando se refresh retornou 401 (possível outra aba):", status);
      if (status === 401) {
        try {
          vlog(F, "fetchWithAuth", "repetindo requisição original uma vez (multi-aba)", metodo, rota);
          retryAposOutraAba = await makeRequest();
        } catch (err) {
          // Falha de rede no retry: nao desloga, mas libera a fila. FE-06: as
          // pendentes recebem o mesmo erro de rede (nao "Sessão expirada",
          // pois nao houve logout). FE-08: esse erro ja e o NetworkError
          // gerado em makeRequest.
          vlog(F, "fetchWithAuth", "normalizando erro de rede do retry multi-aba");
          const erroRede =
            err instanceof Error ? err : new Error(MSG_ERRO_REQUISICAO);
          vlog(F, "fetchWithAuth", "liberando fila com erro de rede do retry");
          onRefreshComplete(false, erroRede);
          vlog(F, "fetchWithAuth", "desmarcando refresh em andamento");
          isRefreshing = false;
          vlog(F, "fetchWithAuth", "ocultando indicador de refresh");
          hideRefreshIndicator();
          vlog(F, "fetchWithAuth", "lançando erro de rede do retry multi-aba");
          throw erroRede;
        }
      }

      vlog(F, "fetchWithAuth", "verificando se retry multi-aba validou a sessão:", retryAposOutraAba?.status);
      if (retryAposOutraAba && retryAposOutraAba.status !== 401) {
        // Sessao valida: libera a fila (as pendentes repetem com o cookie novo).
        vlog(F, "fetchWithAuth", "liberando fila com sucesso (sessão renovada por outra aba)");
        onRefreshComplete(true);
        vlog(F, "fetchWithAuth", "desmarcando refresh em andamento");
        isRefreshing = false;
        vlog(F, "fetchWithAuth", "ocultando indicador de refresh");
        hideRefreshIndicator();

        vlog(F, "fetchWithAuth", "verificando se retry multi-aba retornou ok:", retryAposOutraAba.status);
        if (!retryAposOutraAba.ok) {
          vlog(F, "fetchWithAuth", "lançando ApiError do retry multi-aba", retryAposOutraAba.status);
          throw await toApiError(retryAposOutraAba);
        }
        return parseResponse<T>(retryAposOutraAba);
      }

      // Refresh respondeu com erro HTTP (e, se foi 401, o retry tambem deu
      // 401): logout.
      vlog(F, "fetchWithAuth", "refresh falhou definitivamente: liberando fila com logout");
      onRefreshComplete(false);
      vlog(F, "fetchWithAuth", "desmarcando refresh em andamento");
      isRefreshing = false;
      vlog(F, "fetchWithAuth", "ocultando indicador de refresh");
      hideRefreshIndicator();
      vlog(F, "fetchWithAuth", "limpando sessão local");
      clearTokens();
      vlog(F, "fetchWithAuth", "redirecionando para login");
      redirectToLogin();
      vlog(F, "fetchWithAuth", "lançando erro de sessão expirada");
      throw new Error(MSG_SESSAO_EXPIRADA);
    } catch (error) {
      vlog(F, "fetchWithAuth", "tratando erro do fluxo de refresh: desmarcando refresh");
      isRefreshing = false;
      vlog(F, "fetchWithAuth", "ocultando indicador de refresh");
      hideRefreshIndicator();

      vlog(F, "fetchWithAuth", "verificando se o erro é instância de Error");
      if (error instanceof Error) {
        vlog(F, "fetchWithAuth", "relançando erro do fluxo de refresh");
        throw error;
      }
      vlog(F, "fetchWithAuth", "lançando erro genérico de requisição");
      throw new Error(MSG_ERRO_REQUISICAO);
    }
  }

  // Response não é 401 ou não pode fazer refresh. Inclui o 403 de vendedor
  // desligado, que nunca passa pelo refresh/logout acima (só 401 passa).
  vlog(F, "fetchWithAuth", "verificando se resposta é ok", metodo, rota, response.status);
  if (!response.ok) {
    vlog(F, "fetchWithAuth", "lançando ApiError", metodo, rota, response.status);
    throw await toApiError(response);
  }

  return parseResponse<T>(response);
}

// ============================================
// Helpers para parsing de response
// ============================================

async function parseResponse<T>(res: Response): Promise<T> {
  vlog(F, "parseResponse", "lendo corpo da resposta como texto, status:", res.status);
  const text = await res.text();
  vlog(F, "parseResponse", "inicializando dados da resposta");
  let data: unknown = null;
  try {
    vlog(F, "parseResponse", "fazendo parse JSON do corpo");
    data = text ? JSON.parse(text) : null;
  } catch {
    vlog(F, "parseResponse", "corpo não é JSON, mantendo texto");
    data = text;
  }

  // Desenvelope: {data: ...} -> ... (exceto quando pedido o envelope inteiro)
  vlog(F, "parseResponse", "verificando se deve manter envelope");
  if (respostasComEnvelope.has(res)) return data as T;
  vlog(F, "parseResponse", "verificando se a resposta tem envelope data");
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
  vlog(F, "buildApiError", "verificando se é 403 de vendedor desligado, status:", status);
  if (isVendedorDesligadoResponse(status, message)) {
    vlog(F, "buildApiError", "notificando sessão de vendedor desligado");
    notifyVendedorDesligado();
  }
  return new ApiError(status, message);
}

/** Converte uma resposta não-OK em ApiError (status preservado). */
async function toApiError(res: Response): Promise<ApiError> {
  return buildApiError(res.status, await parseError(res));
}

async function parseError(res: Response): Promise<string> {
  vlog(F, "parseError", "lendo corpo do erro como texto, status:", res.status);
  const text = await res.text();
  vlog(F, "parseError", "inicializando dados do erro");
  let data: unknown = null;
  try {
    vlog(F, "parseError", "fazendo parse JSON do corpo do erro");
    data = text ? JSON.parse(text) : null;
  } catch {
    vlog(F, "parseError", "corpo do erro não é JSON, mantendo texto");
    data = text;
  }

  vlog(F, "parseError", "verificando se o erro é um objeto");
  if (data && typeof data === "object") {
    vlog(F, "parseError", "convertendo erro para Record");
    const d = data as Record<string, unknown>;
    vlog(F, "parseError", "verificando campo error");
    if ("error" in d) return String(d.error);
    vlog(F, "parseError", "verificando campo message");
    if ("message" in d) return String(d.message);
  }

  return `Erro ${res.status}: ${res.statusText}`;
}

// ============================================
// Redirect para login
// ============================================

let redirecting = false;

function redirectToLogin(): void {
  vlog(F, "redirectToLogin", "verificando se já está redirecionando ou fora do navegador:", redirecting);
  if (redirecting || typeof window === "undefined") return;
  vlog(F, "redirectToLogin", "marcando redirecionamento em andamento");
  redirecting = true;

  // Limpa estado local e pede ao backend para limpar os cookies HttpOnly.
  vlog(F, "redirectToLogin", "limpando sessão local");
  clearTokens();

  vlog(F, "redirectToLogin", "lendo URL atual");
  const currentUrl = window.location.href;
  vlog(F, "redirectToLogin", "montando URL de login com redirect");
  const loginUrl = currentUrl.includes("/login")
    ? "/login"
    : `/login?redirect=${encodeURIComponent(currentUrl)}`;

  vlog(F, "redirectToLogin", "enviando POST /api/auth/logout");
  fetch(`${API_BASE}/api/auth/logout`, {
    method: "POST",
    credentials: "include",
  }).finally(() => {
    vlog(F, "redirectToLogin.func", "navegando para /login");
    window.location.href = loginUrl;
  });
}

// ============================================
// Função para forçar logout (útil para erros críticos)
// ============================================

export function forceLogout(message = "Sessão expirada"): void {
  vlog(F, "forceLogout", "limpando sessão local");
  clearTokens();
  vlog(F, "forceLogout", "verificando se está no navegador");
  if (typeof window !== "undefined") {
    vlog(F, "forceLogout", "exibindo alerta de logout");
    alert(message);
    vlog(F, "forceLogout", "enviando POST /api/auth/logout");
    fetch(`${API_BASE}/api/auth/logout`, {
      method: "POST",
      credentials: "include",
    }).finally(() => {
      // Intencional: modulo fora de componente (sem useRouter) e o logout precisa de
      // recarga completa para descartar sessao em memoria, caches e estado do React.
      vlog(F, "forceLogout.func", "navegando para /login");
      // eslint-disable-next-line @next/next/no-location-assign-relative-destination
      window.location.href = "/login";
    });
  }
}
