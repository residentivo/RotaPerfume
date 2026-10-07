import { User } from "./types";
import { vlog } from "./vlog";

// ============================================
// SEGURANCA: access_token e refresh_token sao cookies HttpOnly
// definidos pelo BACKEND via Set-Cookie. Por serem HttpOnly, o
// JavaScript do frontend NÃO consegue ler nem escrever esses cookies
// (document.cookie nunca os expõe) — o navegador os envia sozinho em
// toda requisição para a API (ver credentials: "include" em apiClient.ts).
// A sessão real é sempre validada pelo backend; o client só guarda
// o User (não sensível) para decisões de UI.
//
// LOG-02: os logs verbose daqui nunca recebem o objeto User, e-mail ou
// tokens — no máximo o id numérico e o papel.
// ============================================

const F = "auth.ts";

const USER_STORAGE_KEY = "auth_user";

// ============================================
// Gerenciamento de tokens — limpeza client-side
// ============================================

// Não é possível limpar cookies HttpOnly via JS; a limpeza real
// acontece no backend (POST /api/auth/logout). Aqui só limpamos o
// que o client de fato controla: o usuário em localStorage.
export function clearTokens(): void {
  vlog(F, "clearTokens", "limpando usuário do localStorage");
  clearUser();
}

// ============================================
// Gerenciamento de User (permanece em localStorage)
// User não é sensível - apenas dados públicos do perfil
// ============================================

// Grava apenas os campos conhecidos de User (whitelist). Campos extras que
// venham da API (ex.: `vendedor_desligado` de /api/auth/me) ficam só em
// memória (session.ts) e nunca vão para o localStorage.
export function saveUser(user: User): void {
  vlog(F, "saveUser", "verificando se está no navegador");
  if (typeof window === "undefined") return;
  vlog(F, "saveUser", "montando whitelist do usuário, id/papel:", user.id, user.role);
  const safe: User = {
    id: user.id,
    nome: user.nome,
    email: user.email,
    role: user.role,
    ativo: user.ativo,
    created_at: user.created_at,
    updated_at: user.updated_at,
    id_vendedor: user.id_vendedor ?? null,
    vendedor_nome: user.vendedor_nome ?? null,
  };
  vlog(F, "saveUser", "gravando usuário no localStorage");
  localStorage.setItem(USER_STORAGE_KEY, JSON.stringify(safe));
}

export function getUser(): User | null {
  vlog(F, "getUser", "verificando se está no navegador");
  if (typeof window === "undefined") return null;
  vlog(F, "getUser", "lendo usuário do localStorage");
  const raw = localStorage.getItem(USER_STORAGE_KEY);
  vlog(F, "getUser", "verificando se há usuário salvo:", !!raw);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

export function clearUser(): void {
  vlog(F, "clearUser", "verificando se está no navegador");
  if (typeof window === "undefined") return;
  vlog(F, "clearUser", "removendo usuário do localStorage");
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

export async function logout(): Promise<void> {
  vlog(F, "logout", "limpando sessão local");
  clearTokens();
  vlog(F, "logout", "verificando se está no navegador");
  if (typeof window !== "undefined") {
    // Revoga o refresh token e limpa os cookies HttpOnly no backend.
    // O objetivo do usuário (sair) deve ser cumprido mesmo que o backend
    // esteja indisponível ou a chamada falhe por erro de rede — por isso
    // o fetch é protegido por try/catch e nunca bloqueia o redirect.
    try {
      vlog(F, "logout", "enviando POST /api/auth/logout");
      await fetch(`${(process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/+$/, "")}/api/auth/logout`, {
        method: "POST",
        credentials: "include",
      });
    } catch {
      // Falha de rede ao notificar o backend: ignora e segue com o logout local.
      vlog(F, "logout", "falha de rede no logout do backend, seguindo com logout local");
    } finally {
      // Intencional: modulo fora de componente (sem useRouter) e o logout precisa de
      // recarga completa para descartar sessao em memoria, caches e estado do React.
      vlog(F, "logout", "navegando para /login");
      // eslint-disable-next-line @next/next/no-location-assign-relative-destination
      window.location.href = "/login";
    }
  }
}
