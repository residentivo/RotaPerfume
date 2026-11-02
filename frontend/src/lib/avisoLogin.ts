// FE-17: aviso de uma vez so exibido na tela de login depois de uma
// navegacao client-side (router.replace). Fica so em memoria do modulo: nao
// vai para a URL (nenhum texto controlado por terceiros e exibido) nem para o
// storage. Numa recarga completa o modulo recomeca e o aviso some, o que e o
// comportamento desejado.
import { vlog } from "./vlog";

const F = "avisoLogin.ts";

// Texto aprovado no card FE-17.
export const AVISO_SENHA_ALTERADA = "Senha alterada. Faça login com a nova senha.";

let aviso: string | null = null;

export function definirAvisoLogin(texto: string): void {
  vlog(F, "definirAvisoLogin", "gravando aviso para a tela de login");
  aviso = texto;
}

// Leitura sem consumo: segura para inicializadores de useState, que o
// StrictMode chama duas vezes. Quem exibe o aviso chama limparAvisoLogin
// num efeito depois de montar.
export function lerAvisoLogin(): string | null {
  vlog(F, "lerAvisoLogin", "lendo aviso pendente:", aviso !== null);
  return aviso;
}

export function limparAvisoLogin(): void {
  vlog(F, "limparAvisoLogin", "limpando aviso pendente");
  aviso = null;
}
