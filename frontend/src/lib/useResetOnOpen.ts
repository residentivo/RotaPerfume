import { useState } from "react";

/**
 * Padrao "ajustar estado quando a prop muda"
 * (react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes).
 *
 * Executa `ajustar` DURANTE o render na primeira renderizacao e sempre que
 * alguma dependencia mudar (comparacao por Object.is) — a mesma semantica de
 *   useEffect(() => { ajustar(); }, deps);
 * mas sem setState sincrono dentro de efeito: o React descarta o JSX dessa
 * renderizacao e renderiza de novo com o estado ja ajustado, antes de pintar
 * (sem o "flash" do valor antigo e sem render em cascata apos o commit).
 *
 * `ajustar` so deve chamar setters do proprio componente e nao pode ter
 * efeitos colaterais externos (requisicoes ficam no useEffect).
 */
export function useAjustarAoMudar(deps: readonly unknown[], ajustar: () => void): void {
  const [anterior, setAnterior] = useState<readonly unknown[] | null>(null);

  const mudou =
    anterior === null ||
    anterior.length !== deps.length ||
    anterior.some((d, i) => !Object.is(d, deps[i]));

  if (mudou) {
    setAnterior(deps);
    ajustar();
  }
}

/**
 * Atalho para formularios em modal. Substitui
 *   useEffect(() => { if (open) reset(); }, [open, ...deps]);
 * `reset` roda durante o render quando o modal abre (ou quando alguma dep
 * muda com ele aberto). Ver useAjustarAoMudar.
 */
export function useResetOnOpen(
  open: boolean,
  deps: readonly unknown[],
  reset: () => void
): void {
  useAjustarAoMudar([open, ...deps], () => {
    if (open) reset();
  });
}
