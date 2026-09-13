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
 * 4. Se refresh falhar -> redireciona para /login
 * 5. Lock/fila garante que apenas uma chamada de refresh aconteça por vez
 */

import { clearTokens } from "./auth";

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
let refreshSubscribers: Array<(success: boolean) => void> = [];

function subscribeTokenRefresh(callback: (success: boolean) => void): void {
  refreshSubscribers.push(callback);
}

function onRefreshComplete(success: boolean): void {
  refreshSubscribers.forEach((callback) => callback(success));
  refreshSubscribers = [];
}

// ============================================
// API Call para refresh
// ============================================

async function callRefreshToken(): Promise<boolean> {
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), REFRESH_TIMEOUT);

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

    clearTimeout(timeoutId);

    if (!res.ok) {
      console.warn("[apiClient] Refresh falhou com status:", res.status);
      return false;
    }

    return true;
  } catch (error) {
    if (error instanceof Error && error.name === "AbortError") {
      console.warn("[apiClient] Refresh timeout");
    } else {
      console.error("[apiClient] Erro no refresh:", error);
    }
    return false;
  }
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
}

export async function fetchWithAuth<T = unknown>(
  endpoint: string,
  options: ApiClientOptions = {}
): Promise<T> {
  const { noRefresh = false, headers: customHeaders, ...fetchOptions } = options;

  const makeRequest = async (): Promise<Response> => {
    const url = endpoint.startsWith("http") ? endpoint : `${API_BASE}${endpoint}`;

    const defaultHeaders: HeadersInit = {
      "Content-Type": "application/json",
    };

    // Merge com headers customizados
    const mergedHeaders = { ...defaultHeaders, ...customHeaders };

    // access_token vai via cookie HttpOnly — o navegador o anexa sozinho.
    return fetch(url, {
      ...fetchOptions,
      headers: mergedHeaders,
      credentials: "include",
    });
  };

  // Primeira tentativa
  let response = await makeRequest();

  // Se 401 e pode fazer refresh
  if (response.status === 401 && !noRefresh) {
    // Se já está fazendo refresh, aguarda resultado
    if (isRefreshing) {
      return new Promise((resolve, reject) => {
        subscribeTokenRefresh(async (success) => {
          if (!success) {
            reject(new Error("Sessão expirada. Faça login novamente."));
            return;
          }
          try {
            const retryResponse = await makeRequest();
            if (!retryResponse.ok) {
              const error = await parseError(retryResponse);
              reject(new Error(error));
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
      const refreshed = await callRefreshToken();

      if (refreshed) {
        // Notifica todas as requisições pendentes
        onRefreshComplete(true);
        isRefreshing = false;
        hideRefreshIndicator();

        // Reenvia requisição original (cookies já atualizados pelo backend)
        const retryResponse = await makeRequest();

        if (!retryResponse.ok) {
          const error = await parseError(retryResponse);
          throw new Error(error);
        }

        return parseResponse<T>(retryResponse);
      } else {
        // Refresh falhou
        onRefreshComplete(false);
        isRefreshing = false;
        hideRefreshIndicator();
        clearTokens();
        redirectToLogin();
        throw new Error("Sessão expirada. Faça login novamente.");
      }
    } catch (error) {
      isRefreshing = false;
      hideRefreshIndicator();

      if (error instanceof Error) {
        throw error;
      }
      throw new Error("Erro ao processar requisição");
    }
  }

  // Response não é 401 ou não pode fazer refresh
  if (!response.ok) {
    const error = await parseError(response);
    throw new Error(error);
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

  // Desenvelope: {data: ...} -> ...
  if (data && typeof data === "object" && "data" in data) {
    return (data as { data: T }).data;
  }

  return data as T;
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
      window.location.href = "/login";
    });
  }
}
