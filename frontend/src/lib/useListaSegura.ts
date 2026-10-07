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
import { vlog } from "./vlog";

const F = "useListaSegura.ts";

/**
 * Devolve `executar(requisicao, aoSucesso, aoErro)`. Cada chamada invalida as
 * anteriores: se uma resposta antiga chegar depois de uma nova busca ter
 * sido iniciada, ela e descartada (nem sucesso nem erro sao aplicados).
 * Ao desmontar, tudo o que estiver em voo e descartado.
 */
export function useUltimaResposta() {
  vlog(F, "useUltimaResposta", "criando ref do contador de respostas");
  const ultima = useRef(0);

  vlog(F, "useUltimaResposta", "registrando efeito de invalidação no unmount");
  useEffect(() => {
    vlog(F, "useUltimaResposta.useEffect", "capturando ref do contador");
    const ref = ultima;
    return () => {
      vlog(F, "useUltimaResposta.useEffect", "desmontando: invalidando respostas em voo");
      ref.current++;
    };
  }, []);

  return useCallback(
    <T>(
      requisicao: Promise<T>,
      aoSucesso: (valor: T) => void,
      aoErro: (err: unknown) => void
    ): Promise<void> => {
      vlog(F, "useUltimaResposta.executar", "numerando nova requisição");
      const minha = ++ultima.current;
      return requisicao.then(
        (valor) => {
          vlog(F, "useUltimaResposta.executar", "sucesso: verificando se a resposta é a mais recente:", minha === ultima.current);
          if (minha === ultima.current) aoSucesso(valor);
        },
        (err) => {
          vlog(F, "useUltimaResposta.executar", "erro: verificando se a resposta é a mais recente:", minha === ultima.current);
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
  vlog(F, "useDebounceFiltros", "criando ref da chave aplicada");
  const aplicada = useRef(chave);
  vlog(F, "useDebounceFiltros", "criando ref do callback");
  const callback = useRef(aoMudar);

  vlog(F, "useDebounceFiltros", "registrando efeito de atualização do callback");
  useEffect(() => {
    vlog(F, "useDebounceFiltros.useEffect", "atualizando ref do callback");
    callback.current = aoMudar;
  });

  vlog(F, "useDebounceFiltros", "registrando efeito do debounce");
  useEffect(() => {
    vlog(F, "useDebounceFiltros.useEffect", "verificando se a chave dos filtros mudou:", chave !== aplicada.current);
    if (chave === aplicada.current) return;
    vlog(F, "useDebounceFiltros.useEffect", "armando timer do debounce (ms):", atrasoMs);
    const timer = setTimeout(() => {
      vlog(F, "useDebounceFiltros.func", "aplicando nova chave dos filtros");
      aplicada.current = chave;
      vlog(F, "useDebounceFiltros.func", "disparando callback de mudança dos filtros");
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
  vlog(F, "useExcluidos", "criando ref do conjunto de ids excluídos");
  const excluidos = useRef<Set<K>>(new Set());

  const marcar = (id: K) => {
    vlog(F, "useExcluidos.marcar", "marcando id como excluído:", id);
    excluidos.current.add(id);
  };

  const filtrar = <R extends { data: T[]; total: number }>(res: R): R => {
    vlog(F, "useExcluidos.filtrar", "verificando se há ids excluídos:", excluidos.current.size);
    if (excluidos.current.size === 0) return res;
    vlog(F, "useExcluidos.filtrar", "removendo excluídos da resposta, linhas:", res.data.length);
    const data = res.data.filter((item) => !excluidos.current.has(idDe(item)));
    vlog(F, "useExcluidos.filtrar", "calculando quantidade removida");
    const removidos = res.data.length - data.length;
    vlog(F, "useExcluidos.filtrar", "verificando se algo foi removido:", removidos);
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
  vlog(F, "usePaginaCarregada", "criando estado da página carregada");
  const [carregada, setCarregada] = useState({ pagina: page, limite: limit });

  vlog(F, "usePaginaCarregada", "criando callback registrar");
  const registrar = useCallback((pagina: number, limite: number) => {
    vlog(F, "usePaginaCarregada.registrar", "registrando página/limite carregados:", pagina, limite);
    setCarregada((prev) =>
      prev.pagina === pagina && prev.limite === limite ? prev : { pagina, limite }
    );
  }, []);

  vlog(F, "usePaginaCarregada", "calculando página exibida, erroCarga:", erroCarga);
  const exibida = erroCarga ? carregada : { pagina: page, limite: limit };
  return { exibida, registrar };
}

/** Faixa "inicio-fim" exibida no contador do cabecalho. */
export function faixaExibida(pagina: number, limite: number, total: number) {
  vlog(F, "faixaExibida", "verificando se o total é zero:", total);
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
