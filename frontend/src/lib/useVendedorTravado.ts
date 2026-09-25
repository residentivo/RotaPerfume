/**
 * useVendedorTravado.ts - Vendedor do usuario `normal` nos modais com
 * select de vendedor (SEC-03).
 *
 * Para o usuario normal, GET /api/vendedores devolve so o proprio vendedor
 * ([] sem vinculo; 403 se desligado). A fonte da verdade do vinculo e a
 * sessao validada por /api/auth/me (id_vendedor), a mesma usada no
 * PedidoModal e nas listagens. Enquanto o /me nao chegou (user null), nada
 * e travado: o backend continua validando o vendedor no POST/PUT.
 */
import { useSessionUser } from "./session";

export interface VendedorTravado {
  /** Sessao carregada e usuario nao-admin. */
  normal: boolean;
  /** id_vendedor do usuario normal (select pre-selecionado e travado). */
  vendedorTravadoId: number | null;
  vendedorTravadoNome: string | null;
  /** Usuario normal sem vendedor vinculado: nao pode salvar. */
  semVendedor: boolean;
}

export function useVendedorTravado(): VendedorTravado {
  const user = useSessionUser();
  const normal = !!user && user.role !== "admin";
  const vendedorTravadoId = normal && user?.id_vendedor ? user.id_vendedor : null;
  return {
    normal,
    vendedorTravadoId,
    vendedorTravadoNome: normal ? user?.vendedor_nome ?? null : null,
    semVendedor: normal && !vendedorTravadoId,
  };
}

/** Valor inicial do select de vendedor: travado (normal) ou o do registro. */
export function vendedorInicial(t: VendedorTravado, doRegistro: string): string {
  if (!t.normal) return doRegistro;
  return t.vendedorTravadoId ? String(t.vendedorTravadoId) : "";
}
