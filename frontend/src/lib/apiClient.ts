/**
 * apiClient.ts - Cliente HTTP com interceptador de 401 + refresh automático
 *
 * Fluxo:
 * 1. fetchWithAuth() adiciona Authorization automaticamente
 * 2. Se resposta 401 -> tenta refresh do token
 * 3. Se refresh OK -> atualiza tokens e reenvia requisição original
 * 4. Se refresh falhar -> redireciona para /login
 * 5. Lock/fila garante que apenas uma chamada de refresh aconteça por vez
 */

import {
  getAccessToken,
  setTokens,
  clearTokens,
} from "./auth";

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
let refreshSubscribers: Array<(token: string) => void> = [];

function subscribeTokenRefresh(callback: (token: string) => void): void {
  refreshSubscribers.push(callback);
}

function onRefreshComplete(newToken: string): void {
  refreshSubscribers.forEach((callback) => callback(newToken));
  refreshSubscribers = [];
}

// ============================================
// API Call para refresh
// ============================================

interface RefreshResponse {
  access_token: string;
  refresh_token?: string;
}

async function callRefreshToken(): Promise<{ token: string; refresh_token?: string } | null> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) return null;

  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), REFRESH_TIMEOUT);

    const res = await fetch(`${API_BASE}/api/auth/refresh`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ refresh_token: refreshToken }),
      signal: controller.signal,
    });

    clearTimeout(timeoutId);

    if (!res.ok) {
      console.warn("[apiClient] Refresh falhou com status:", res.status);
      return null;
    }

    const text = await res.text();
    let data: unknown = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {
      data = text;
    }

    // Extrai token do envelope {data: {access_token: ...}} ou direto
    let newAccessToken: string | null = null;
    let newRefreshToken: string | undefined = undefined;

    if (data && typeof data === "object") {
      const d = data as Record<string, unknown>;
      if ("access_token" in d) {
        newAccessToken = String(d.access_token);
        newRefreshToken = d.refresh_token ? String(d.refresh_token) : undefined;
      } else if ("data" in d && typeof d.data === "object") {
        const inner = d.data as Record<string, unknown>;
        if ("access_token" in inner) {
          newAccessToken = String(inner.access_token);
          newRefreshToken = inner.refresh_token ? String(inner.refresh_token) : undefined;
        }
      }
    }

    if (!newAccessToken) {
      console.warn("[apiClient] Refresh não retornou access_token");
      return null;
    }

    return { token: newAccessToken, refresh_token: newRefreshToken };
  } catch (error) {
    if (error instanceof Error && error.name === "AbortError") {
      console.warn("[apiClient] Refresh timeout");
    } else {
      console.error("[apiClient] Erro no refresh:", error);
    }
    return null;
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

  const makeRequest = async (token: string | null): Promise<Response> => {
    const url = endpoint.startsWith("http") ? endpoint : `${API_BASE}${endpoint}`;

    const defaultHeaders: HeadersInit = {
      "Content-Type": "application/json",
    };

    // Adiciona Authorization se tiver token
    if (token) {
      (defaultHeaders as Record<string, string>)["Authorization"] = `Bearer ${token}`;
    }

    // Merge com headers customizados
    const mergedHeaders = { ...defaultHeaders, ...customHeaders };

    return fetch(url, {
      ...fetchOptions,
      headers: mergedHeaders,
    });
  };

  // Primeira tentativa
  const token = getAccessToken();
  let response = await makeRequest(token);

  // Se 401 e pode fazer refresh
  if (response.status === 401 && !noRefresh && token) {
    // Se já está fazendo refresh, aguarda resultado
    if (isRefreshing) {
      return new Promise((resolve, reject) => {
        subscribeTokenRefresh(async (newToken) => {
          try {
            const retryResponse = await makeRequest(newToken);
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
      const refreshResult = await callRefreshToken();

      if (refreshResult) {
        // Atualiza tokens
        setTokens(refreshResult.token, refreshResult.refresh_token || getRefreshToken() || "");

        // Notifica todas as requisições pendentes
        onRefreshComplete(refreshResult.token);
        isRefreshing = false;
        hideRefreshIndicator();

        // Reenvia requisição original com novo token
        const retryResponse = await makeRequest(refreshResult.token);

        if (!retryResponse.ok) {
          const error = await parseError(retryResponse);
          throw new Error(error);
        }

        return parseResponse<T>(retryResponse);
      } else {
        // Refresh falhou
        onRefreshComplete("");
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

  // Limpa estado antes de redirecionar
  clearTokens();

  // Salva URL atual para voltar depois do login
  const currentUrl = window.location.href;
  const loginUrl = currentUrl.includes("/login")
    ? "/login"
    : `/login?redirect=${encodeURIComponent(currentUrl)}`;

  window.location.href = loginUrl;
}

// ============================================
// Função para forçar logout (útil para erros críticos)
// ============================================

export function forceLogout(message = "Sessão expirada"): void {
  clearTokens();
  if (typeof window !== "undefined") {
    alert(message);
    window.location.href = "/login";
  }
}

// ============================================
// Verifica se precisa de refresh (útil para inicialização)
// ============================================

export function needsRefresh(): boolean {
  const token = getAccessToken();
  const refresh = getRefreshToken();

  if (!refresh) return false;
  if (!token) return true;

  // Verifica se token está expirado (se conseguir decodificar)
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return true;

    const payload = parts[1];
    const padded = payload + "=".repeat((4 - (payload.length % 4)) % 4);
    const decoded = JSON.parse(atob(padded.replace(/-/g, "+").replace(/_/g, "/")));

    // Se tem exp e está a menos de 5 minutos, já renova
    if (decoded.exp) {
      const now = Math.floor(Date.now() / 1000);
      const fiveMinutes = 5 * 60;
      return decoded.exp - now < fiveMinutes;
    }
  } catch {
    return true;
  }

  return false;
}

// Re-export getAccessToken para conveniência
export { getAccessToken };
