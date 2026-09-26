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
 *
 * FE-10: helpers para manter paginador/contador coerentes com as linhas
 * exibidas quando uma carga falha (`usePaginaCarregada`, `faixaExibida`) e
 * para a mensagem de "acao feita, mas a recarga falhou"
 * (`mensagemRecargaFalhou`).
 */
import { useCallback, useEffect, useRef, useState } from "react";

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

/**
 * FE-10: pagina/limite que o paginador e o contador devem exibir.
 *
 * A tela continua guardando a pagina pedida (`page`, que dispara a busca) e
 * registra, a cada sucesso, a pagina/limite da carga que deu certo — ou seja,
 * das linhas que estao na tela. `exibida` e:
 * - a pedida, enquanto a ultima carga nao falhou (inclusive durante o
 *   loading: dois cliques rapidos em "proxima" avancam duas paginas);
 * - a ultima carregada com sucesso, se a ultima carga falhou. Assim a tabela
 *   (linhas antigas mantidas pelo FE-09) e o paginador falam da mesma pagina.
 *
 * `registrar(pagina, limite)` deve ser chamado no sucesso da busca, com os
 * valores usados na requisicao.
 */
export function usePaginaCarregada(page: number, limit: number, erroCarga: boolean) {
  const [carregada, setCarregada] = useState({ pagina: page, limite: limit });

  const registrar = useCallback((pagina: number, limite: number) => {
    setCarregada((prev) =>
      prev.pagina === pagina && prev.limite === limite ? prev : { pagina, limite }
    );
  }, []);

  const exibida = erroCarga ? carregada : { pagina: page, limite: limit };
  return { exibida, registrar };
}

/** Faixa "inicio-fim" exibida no contador do cabecalho. */
export function faixaExibida(pagina: number, limite: number, total: number) {
  if (total === 0) return { inicio: 0, fim: 0 };
  return {
    inicio: (pagina - 1) * limite + 1,
    fim: Math.min(pagina * limite, total),
  };
}

/**
 * FE-10: a acao (criar, editar, inativar...) deu certo, mas a recarga da
 * lista que vem logo depois falhou. Em vez de exibir o alerta de sucesso e o
 * de erro ao mesmo tempo (contraditorios: "inativado com sucesso" com a linha
 * ainda "Ativo" na tabela), a tela mostra um unico alerta de erro que diz as
 * duas coisas. Ele nao some sozinho, pois a lista na tela esta desatualizada.
 */
export function mensagemRecargaFalhou(sucesso: string, erroRecarga: string): string {
  return `${sucesso} Porem, nao foi possivel recarregar a lista (${erroRecarga}). Os dados exibidos podem estar desatualizados.`;
}
