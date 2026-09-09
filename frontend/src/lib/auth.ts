import { User } from "./types";

// ============================================
// SEGURANCA: Tokens agora usam cookies HttpOnly
// Migração de localStorage -> cookies
// Importante: O HttpOnly deve ser setado pelo BACKEND via Set-Cookie header
// ============================================

const ACCESS_TOKEN_COOKIE = "access_token";
const REFRESH_TOKEN_COOKIE = "refresh_token";
const USER_STORAGE_KEY = "auth_user"; // User permanece em localStorage (não é sensível)

// ============================================
// Funções utilitárias para gerenciamento de cookies
// ============================================

/**
 * Lê um cookie pelo nome
 * Retorna null se não encontrar ou se estiver em ambiente SSR
 */
export function getCookie(name: string): string | null {
  if (typeof window === "undefined") return null;

  const match = document.cookie.match(
    new RegExp("(^| )" + name + "=([^;]+)")
  );
  return match ? decodeURIComponent(match[2]) : null;
}

/**
 * Define um cookie com flags de segurança
 * NOTA: Para HttpOnly real, o BACKEND deve enviar Set-Cookie header
 * Este método é usado para transição ou quando HttpOnly não está disponível
 */
export function setCookie(
  name: string,
  value: string,
  options: {
    secure?: boolean;
    sameSite?: "Strict" | "Lax" | "None";
    path?: string;
    maxAge?: number; // em segundos
  } = {}
): void {
  if (typeof window === "undefined") return;

  const {
    secure = true,
    sameSite = "Strict",
    path = "/",
    maxAge,
  } = options;

  let cookie = `${name}=${encodeURIComponent(value)}`;
  cookie += `; Path=${path}`;
  cookie += `; SameSite=${sameSite}`;

  if (secure) {
    cookie += "; Secure";
  }

  if (maxAge !== undefined) {
    cookie += `; Max-Age=${maxAge}`;
  }

  document.cookie = cookie;
}

/**
 * Remove um cookie definindo expiração no passado
 */
export function clearCookie(name: string, path = "/"): void {
  if (typeof window === "undefined") return;
  document.cookie = `${name}=; Path=${path}; Expires=Thu, 01 Jan 1970 00:00:00 GMT`;
}

// ============================================
// Gerenciamento de Access Token (via Cookie)
// ============================================

export function getAccessToken(): string | null {
  return getCookie(ACCESS_TOKEN_COOKIE);
}

export function setAccessToken(token: string, maxAgeSeconds = 3600): void {
  setCookie(ACCESS_TOKEN_COOKIE, token, {
    secure: true,
    sameSite: "Strict",
    path: "/",
    maxAge: maxAgeSeconds,
  });
}

export function clearAccessToken(): void {
  clearCookie(ACCESS_TOKEN_COOKIE);
}

// ============================================
// Gerenciamento de Refresh Token (via Cookie)
// ============================================

export function getRefreshToken(): string | null {
  return getCookie(REFRESH_TOKEN_COOKIE);
}

export function setRefreshToken(token: string, maxAgeSeconds = 604800): void {
  // 7 dias por padrão
  setCookie(REFRESH_TOKEN_COOKIE, token, {
    secure: true,
    sameSite: "Lax", // Lax para permitir redirect после login
    path: "/",
    maxAge: maxAgeSeconds,
  });
}

export function clearRefreshToken(): void {
  clearCookie(REFRESH_TOKEN_COOKIE);
}

// ============================================
// Gerenciamento combinado de tokens
// ============================================

export function setTokens(
  access_token: string,
  refresh_token: string
): void {
  setAccessToken(access_token);
  if (refresh_token) {
    setRefreshToken(refresh_token);
  }
}

export function getTokens(): { access_token: string; refresh_token: string } | null {
  const access = getAccessToken();
  const refresh = getRefreshToken();
  if (!access && !refresh) return null;
  return {
    access_token: access || "",
    refresh_token: refresh || "",
  };
}

export function clearTokens(): void {
  clearAccessToken();
  clearRefreshToken();
  clearUser();
}

// ============================================
// Gerenciamento de User (permanece em localStorage)
// User não é sensível - apenas dados públicos do perfil
// ============================================

export function saveUser(user: User): void {
  if (typeof window === "undefined") return;
  localStorage.setItem(USER_STORAGE_KEY, JSON.stringify(user));
}

export function getUser(): User | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem(USER_STORAGE_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

export function clearUser(): void {
  if (typeof window === "undefined") return;
  localStorage.removeItem(USER_STORAGE_KEY);
}

// ============================================
// Utilitários de autenticação
// ============================================

export function isAdmin(): boolean {
  return getUser()?.role === "admin";
}

export function isAuthenticated(): boolean {
  return !!getAccessToken();
}

export function logout(): void {
  clearTokens();
  if (typeof window !== "undefined") {
    window.location.href = "/login";
  }
}

export function decodeJwtPayload(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = parts[1];
    const padded = payload + "=".repeat((4 - (payload.length % 4)) % 4);
    const decoded = atob(padded.replace(/-/g, "+").replace(/_/g, "/"));
    return JSON.parse(decoded);
  } catch {
    return null;
  }
}
