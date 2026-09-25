/**
 * useListaSegura.ts - Hooks das listagens paginadas (FE-04).
 *
 * Tres problemas corrigidos juntos:
 * 1. Resposta obsoleta: cada busca recebe um numero crescente; so a mais
 *    recente aplica o resultado (mesma ideia do `cancelado` do Dashboard,
 *    mas valendo tambem para as recargas imperativas dos handlers).
 * 2. Debounce dos filtros disparado na montagem: o timer so e armado quando
 *    a chave dos filtros muda em relacao a ultima chave aplicada.
 * 3. Linha excluida que reaparece: ids excluidos nesta tela sao removidos de
 *    qualquer resposta que chegue depois (ela pode ter saido antes do DELETE).
 */
import { useCallback, useEffect, useRef } from "react";

/**
 * Devolve `executar(requisicao, aoSucesso, aoErro)`. Cada chamada invalida as
 * anteriores: se uma resposta antiga chegar depois de uma nova busca ter
 * sido iniciada, ela e descartada (nem sucesso nem erro sao aplicados).
 * Ao desmontar, tudo o que estiver em voo e descartado.
 */
export function useUltimaResposta() {
  const ultima = useRef(0);

  useEffect(() => {
    const ref = ultima;
    return () => {
      ref.current++;
    };
  }, []);

  return useCallback(
    <T>(
      requisicao: Promise<T>,
      aoSucesso: (valor: T) => void,
      aoErro: (err: unknown) => void
    ): Promise<void> => {
      const minha = ++ultima.current;
      return requisicao.then(
        (valor) => {
          if (minha === ultima.current) aoSucesso(valor);
        },
        (err) => {
          if (minha === ultima.current) aoErro(err);
        }
      );
    },
    []
  );
}

/**
 * Chama `aoMudar` `atrasoMs` depois que `chave` (serializacao dos filtros)
 * muda. Nao dispara na montagem nem quando a chave volta ao valor ja
 * aplicado antes do timer vencer. `aoMudar` sempre enxerga o render mais
 * recente (lido por ref).
 */
export function useDebounceFiltros(chave: string, aoMudar: () => void, atrasoMs = 350) {
  const aplicada = useRef(chave);
  const callback = useRef(aoMudar);

  useEffect(() => {
    callback.current = aoMudar;
  });

  useEffect(() => {
    if (chave === aplicada.current) return;
    const timer = setTimeout(() => {
      aplicada.current = chave;
      callback.current();
    }, atrasoMs);
    return () => clearTimeout(timer);
  }, [chave, atrasoMs]);
}

/**
 * Registro dos ids excluidos nesta tela. `marcar(id)` apos o DELETE dar
 * certo; `filtrar(res)` remove esses ids de uma resposta paginada que chegue
 * depois e desconta o `total`. Ids nao sao reaproveitados pelo backend,
 * entao o registro vale enquanto a tela estiver montada.
 */
export function useExcluidos<T, K>(idDe: (item: T) => K) {
  const excluidos = useRef<Set<K>>(new Set());

  const marcar = (id: K) => {
    excluidos.current.add(id);
  };

  const filtrar = <R extends { data: T[]; total: number }>(res: R): R => {
    if (excluidos.current.size === 0) return res;
    const data = res.data.filter((item) => !excluidos.current.has(idDe(item)));
    const removidos = res.data.length - data.length;
    if (removidos === 0) return res;
    return { ...res, data, total: Math.max(0, res.total - removidos) };
  };

  return { marcar, filtrar };
}
