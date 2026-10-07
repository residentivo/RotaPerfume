// Log verbose de rastreamento (LOG-02).
// NEXT_PUBLIC_VERBOSE é embutido no bundle em tempo de build: em produção
// deve ficar ausente ou "false" para o bundler eliminar as chamadas.
// Nunca passe valores sensíveis (senhas, tokens, captcha, User, e-mail, CNPJ,
// corpo de request/response) — ver LOG-02 spec de segurança §3.
const VERBOSE = process.env.NEXT_PUBLIC_VERBOSE === "true";

export function vlog(arquivo: string, funcao: string, msg: string, ...args: unknown[]): void {
  if (!VERBOSE) return;
  console.log(`[${arquivo}] [${funcao}] ${msg}`, ...args);
}
