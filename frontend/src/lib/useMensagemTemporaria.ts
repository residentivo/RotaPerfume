/**
 * useMensagemTemporaria.ts - Timers seguros para telas (FE-11).
 *
 * Antes, as telas faziam `setTimeout(() => setSuccess(null), 4000)` (e a
 * trocar-senha, `setTimeout(() => router.replace(...), 1500)`) sem guardar o
 * id do timer: o callback rodava mesmo depois do unmount (setState em tela
 * desmontada e, na trocar-senha, redirecionamento depois de o usuario ja ter
 * saido da tela), e uma mensagem nova podia ser apagada antes do tempo pelo
 * timer da mensagem anterior.
 */
import { useCallback, useEffect, useRef, useState } from "react";

/** Duracao padrao das mensagens de sucesso das listagens. */
export const DURACAO_MENSAGEM_MS = 4000;

/**
 * Um unico timer por instancia. `agendar` cancela o anterior antes de armar
 * o novo; o timer pendente e cancelado no unmount.
 */
export function useTimeoutSeguro() {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const cancelar = useCallback(() => {
    if (timer.current !== null) {
      clearTimeout(timer.current);
      timer.current = null;
    }
  }, []);

  useEffect(() => cancelar, [cancelar]);

  const agendar = useCallback(
    (acao: () => void, atrasoMs: number) => {
      cancelar();
      timer.current = setTimeout(() => {
        timer.current = null;
        acao();
      }, atrasoMs);
    },
    [cancelar]
  );

  return { agendar, cancelar };
}

/**
 * Mensagem que some sozinha depois de `duracaoMs`.
 * - `mostrar(texto)`: exibe e (re)inicia o timer — trocar a mensagem cancela
 *   o timer da anterior, entao a nova fica visivel pelo tempo inteiro.
 * - `limpar()`: esconde na hora e cancela o timer.
 * No unmount o timer pendente e cancelado.
 */
export function useMensagemTemporaria(duracaoMs = DURACAO_MENSAGEM_MS) {
  const [mensagem, setMensagem] = useState<string | null>(null);
  const { agendar, cancelar } = useTimeoutSeguro();

  const mostrar = useCallback(
    (texto: string) => {
      setMensagem(texto);
      agendar(() => setMensagem(null), duracaoMs);
    },
    [agendar, duracaoMs]
  );

  const limpar = useCallback(() => {
    cancelar();
    setMensagem(null);
  }, [cancelar]);

  return { mensagem, mostrar, limpar };
}
