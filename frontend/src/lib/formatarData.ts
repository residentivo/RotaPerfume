/**
 * FE-14: formatador de data unico do frontend (pt-BR).
 *
 * Regras comuns as duas variantes:
 *  - string vazia, null ou undefined -> "-";
 *  - data zero do Go ("0001-01-01T00:00:00Z", devolvida quando a coluna e
 *    NULL e o campo e time.Time) -> "-" (ano <= 1);
 *  - texto que nao e data valida -> devolvido como veio (comportamento que
 *    as telas ja tinham);
 *  - "AAAA-MM-DD" sem hora e interpretado como meia-noite LOCAL, para nao
 *    deslocar um dia por fuso (new Date("AAAA-MM-DD") seria meia-noite UTC,
 *    que no Brasil vira o dia anterior).
 */

const SOMENTE_DATA = /^\d{4}-\d{2}-\d{2}$/;

const OPCOES_DATA_HORA: Intl.DateTimeFormatOptions = {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
};

/**
 * Converte o valor em Date. Devolve null para ausencia de data (vazio ou
 * data zero do Go) e undefined para texto invalido.
 */
function interpretar(valor: string): Date | null | undefined {
  const somenteData = SOMENTE_DATA.test(valor);
  const d = new Date(somenteData ? `${valor}T00:00:00` : valor);
  if (Number.isNaN(d.getTime())) return undefined;
  const ano = somenteData ? d.getFullYear() : d.getUTCFullYear();
  if (ano <= 1) return null;
  return d;
}

function formatar(
  valor: string | null | undefined,
  render: (d: Date) => string
): string {
  if (!valor) return "-";
  const d = interpretar(valor);
  if (d === null) return "-";
  if (d === undefined) return valor;
  return render(d);
}

/** Formata so a data: "dd/mm/aaaa". */
export function formatarData(valor: string | null | undefined): string {
  return formatar(valor, (d) => d.toLocaleDateString("pt-BR"));
}

/** Formata data e hora: "dd/mm/aaaa, hh:mm:ss". */
export function formatarDataHora(valor: string | null | undefined): string {
  return formatar(valor, (d) => d.toLocaleString("pt-BR", OPCOES_DATA_HORA));
}
