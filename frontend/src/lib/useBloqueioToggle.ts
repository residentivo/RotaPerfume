"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { vlog } from "@/lib/vlog";

const FILE = "lib/useBloqueioToggle.ts";

/** BUG-17/FE-18: tempo (ms) de bloqueio do toggle de ativo apos cada alteracao. */
export const TOGGLE_BLOQUEIO_MS = 1500;

export type BloqueioToggle<K> = {
  /** Estado (para o `disabled` do botao): true durante o bloqueio pos-acao. */
  estaBloqueado: (chave: K) => boolean;
  /**
   * Checagem sincrona (ref): devolve false se a chave ja esta em andamento ou
   * bloqueada; senao marca a chave e devolve true. Dois cliques no mesmo tick
   * (antes do re-render) nao passam, porque a ref nao depende da closure.
   */
  adquirir: (chave: K) => boolean;
  /** Libera a chave sem bloqueio (ex.: o usuario cancelou a confirmacao). */
  liberar: (chave: K) => void;
  /** Chamar no `finally`: mantem a chave bloqueada por `ms` e depois libera. */
  bloquear: (chave: K) => void;
};

/**
 * FE-18: bloqueio por chave do toggle de ativo (produtos, clientes, usuarios).
 * A ref garante a exclusao sincrona; o state so alimenta o `disabled`. Os
 * timers sao limpos no unmount e nao sao armados depois dele.
 */
export function useBloqueioToggle<K extends string | number>(
  ms: number = TOGGLE_BLOQUEIO_MS
): BloqueioToggle<K> {
  vlog(FILE, "useBloqueioToggle", "inicializando estado de chaves bloqueadas");
  const [bloqueados, setBloqueados] = useState<Set<K>>(() => new Set());
  vlog(FILE, "useBloqueioToggle", "inicializando ref sincrona de chaves ocupadas");
  const ocupados = useRef<Set<K>>(new Set());
  vlog(FILE, "useBloqueioToggle", "inicializando mapa de timers");
  const timers = useRef<Map<K, ReturnType<typeof setTimeout>>>(new Map());
  const montado = useRef(false);

  useEffect(() => {
    vlog(FILE, "useBloqueioToggle.useEffect", "montado");
    montado.current = true;
    const mapa = timers.current;
    return () => {
      vlog(FILE, "useBloqueioToggle.useEffect", "cleanup: limpando timers de bloqueio (qtd=%d)", mapa.size);
      montado.current = false;
      mapa.forEach((t) => clearTimeout(t));
      mapa.clear();
    };
  }, []);

  const estaBloqueado = useCallback((chave: K) => bloqueados.has(chave), [bloqueados]);

  const adquirir = useCallback((chave: K) => {
    vlog(FILE, "useBloqueioToggle.adquirir", "verificando chave (chave=%s)", String(chave));
    if (ocupados.current.has(chave)) return false;
    ocupados.current.add(chave);
    return true;
  }, []);

  const liberar = useCallback((chave: K) => {
    vlog(FILE, "useBloqueioToggle.liberar", "liberando chave sem bloqueio (chave=%s)", String(chave));
    ocupados.current.delete(chave);
  }, []);

  const bloquear = useCallback(
    (chave: K) => {
      if (!montado.current) {
        vlog(FILE, "useBloqueioToggle.bloquear", "desmontado: nao arma timer (chave=%s)", String(chave));
        ocupados.current.delete(chave);
        return;
      }
      vlog(FILE, "useBloqueioToggle.bloquear", "bloqueando por %d ms (chave=%s)", ms, String(chave));
      ocupados.current.add(chave);
      setBloqueados((prev) => new Set(prev).add(chave));
      const anterior = timers.current.get(chave);
      if (anterior) clearTimeout(anterior);
      timers.current.set(
        chave,
        setTimeout(() => {
          vlog(FILE, "useBloqueioToggle.bloquear", "liberando chave (chave=%s)", String(chave));
          timers.current.delete(chave);
          ocupados.current.delete(chave);
          setBloqueados((prev) => {
            const next = new Set(prev);
            next.delete(chave);
            return next;
          });
        }, ms)
      );
    },
    [ms]
  );

  return { estaBloqueado, adquirir, liberar, bloquear };
}
