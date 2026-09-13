import { User } from "./types";

// ============================================
// SEGURANCA: access_token e refresh_token sao cookies HttpOnly
// definidos pelo BACKEND via Set-Cookie. Por serem HttpOnly, o
// JavaScript do frontend NÃO consegue ler nem escrever esses cookies
// (document.cookie nunca os expõe) — o navegador os envia sozinho em
// toda requisição para a API (ver credentials: "include" em apiClient.ts).
// A sessão real é sempre validada pelo backend; o client só guarda
// o User (não sensível) para decisões de UI.
// ============================================

const USER_STORAGE_KEY = "auth_user";

// ============================================
// Gerenciamento de tokens — limpeza client-side
// ============================================

// Não é possível limpar cookies HttpOnly via JS; a limpeza real
// acontece no backend (POST /api/auth/logout). Aqui só limpamos o
// que o client de fato controla: o usuário em localStorage.
export function clearTokens(): void {
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
  return !!getUser();
}

export function logout(): void {
  clearTokens();
  if (typeof window !== "undefined") {
    // Revoga o refresh token e limpa os cookies HttpOnly no backend.
    fetch(`${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/auth/logout`, {
      method: "POST",
      credentials: "include",
    }).finally(() => {
      window.location.href = "/login";
    });
  }
}
