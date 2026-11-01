/**
 * trocaSenhaObrigatoria.ts - Contrato do bloqueio por troca de senha pendente.
 *
 * Contrato definido pelo SecBrain (SEC-23): enquanto o usuario tiver
 * `deve_trocar_senha`, o backend responde
 *   403 {"success":false,"error":"troca de senha obrigatória"}
 * em todas as rotas autenticadas, exceto POST /api/auth/reset-password e
 * GET /api/auth/me (que passa a devolver `trocar_senha: boolean`).
 *
 * A deteccao e feita por igualdade EXATA da mensagem + status 403 (sem
 * includes/regex), como em vendedorDesligado.ts, para nao confundir com
 * outros 403.
 */

import { ApiError } from "./apiError";

/** Mensagem exata do backend (com acento). */
export const MSG_TROCA_SENHA_OBRIGATORIA = "troca de senha obrigatória";

/** Rota da tela de troca de senha (destino do redirecionamento). */
export const ROTA_TROCAR_SENHA = "/trocar-senha";

/** Deteccao a partir do status + mensagem crus (usada pelo apiClient). */
export function isTrocaSenhaObrigatoriaResponse(status: number, message: string): boolean {
  return status === 403 && message === MSG_TROCA_SENHA_OBRIGATORIA;
}

export function isTrocaSenhaObrigatoriaError(err: unknown): boolean {
  return (
    err instanceof ApiError &&
    err.status === 403 &&
    err.message === MSG_TROCA_SENHA_OBRIGATORIA
  );
}
