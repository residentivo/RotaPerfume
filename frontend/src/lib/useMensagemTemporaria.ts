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
import { vlog } from "./vlog";

const F = "useMensagemTemporaria.ts";

/** Duracao padrao das mensagens de sucesso das listagens. */
export const DURACAO_MENSAGEM_MS = 4000;

/**
 * Um unico timer por instancia. `agendar` cancela o anterior antes de armar
 * o novo; o timer pendente e cancelado no unmount.
 */
export function useTimeoutSeguro() {
  vlog(F, "useTimeoutSeguro", "criando ref do timer");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  vlog(F, "useTimeoutSeguro", "criando callback cancelar");
  const cancelar = useCallback(() => {
    vlog(F, "useTimeoutSeguro.cancelar", "verificando se há timer pendente:", timer.current !== null);
    if (timer.current !== null) {
      vlog(F, "useTimeoutSeguro.cancelar", "cancelando timer pendente");
      clearTimeout(timer.current);
      vlog(F, "useTimeoutSeguro.cancelar", "zerando ref do timer");
      timer.current = null;
    }
  }, []);

  vlog(F, "useTimeoutSeguro", "registrando efeito de cancelamento no unmount");
  useEffect(() => cancelar, [cancelar]);

  vlog(F, "useTimeoutSeguro", "criando callback agendar");
  const agendar = useCallback(
    (acao: () => void, atrasoMs: number) => {
      vlog(F, "useTimeoutSeguro.agendar", "cancelando timer anterior");
      cancelar();
      vlog(F, "useTimeoutSeguro.agendar", "armando novo timer (ms):", atrasoMs);
      timer.current = setTimeout(() => {
        vlog(F, "useTimeoutSeguro.func", "timer vencido: zerando ref");
        timer.current = null;
        vlog(F, "useTimeoutSeguro.func", "executando ação agendada");
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
  vlog(F, "useMensagemTemporaria", "criando estado da mensagem");
  const [mensagem, setMensagem] = useState<string | null>(null);
  vlog(F, "useMensagemTemporaria", "obtendo timer seguro");
  const { agendar, cancelar } = useTimeoutSeguro();

  vlog(F, "useMensagemTemporaria", "criando callback mostrar");
  const mostrar = useCallback(
    (texto: string) => {
      vlog(F, "useMensagemTemporaria.mostrar", "exibindo mensagem temporária");
      setMensagem(texto);
      vlog(F, "useMensagemTemporaria.mostrar", "agendando ocultação da mensagem (ms):", duracaoMs);
      agendar(() => setMensagem(null), duracaoMs);
    },
    [agendar, duracaoMs]
  );

  vlog(F, "useMensagemTemporaria", "criando callback limpar");
  const limpar = useCallback(() => {
    vlog(F, "useMensagemTemporaria.limpar", "cancelando timer da mensagem");
    cancelar();
    vlog(F, "useMensagemTemporaria.limpar", "ocultando mensagem");
    setMensagem(null);
  }, [cancelar]);

  return { mensagem, mostrar, limpar };
}
