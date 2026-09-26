/**
 * NEG-02: CNPJ alfanumerico da Receita Federal.
 *
 * Unico ponto do frontend que manipula CNPJ (entrada, mascara, validacao e
 * exibicao). A validacao espelha a do backend (apis/shared/cnpj):
 *  - apara espacos das bordas (TrimSpace do service), remove a mascara
 *    (".", "/", "-" e o espaco comum) e passa a-z para maiusculas;
 *  - qualquer outro caractere (tab, NBSP, quebra de linha no meio, letras
 *    nao ASCII como "ı" ou "Á") torna o CNPJ invalido;
 *  - o valor normalizado deve bater com ^[0-9A-Z]{12}[0-9]{2}$;
 *  - rejeita todos os caracteres iguais;
 *  - DV por modulo 11, valor de cada caractere = codigo ASCII - 48,
 *    pesos 5..2,9..2 (1o DV) e 6..2,9..2 (2o DV); resto < 2 -> 0,
 *    senao 11 - resto.
 * O backend devolve sempre 14 caracteres sem mascara e em maiusculas; a
 * mascara de exibicao (XX.XXX.XXX/XXXX-XX) fica com o frontend.
 */

/** Tamanho do CNPJ sem mascara. */
export const CNPJ_LENGTH = 14;
/** Tamanho do CNPJ com mascara (XX.XXX.XXX/XXXX-XX). */
export const CNPJ_MASKED_LENGTH = 18;
/** Posicoes da raiz + ordem (alfanumericas); as 2 ultimas sao o DV. */
const CNPJ_BASE_LENGTH = 12;

export const CNPJ_PLACEHOLDER = "Ex: 12.ABC.345/01DE-35";
export const CNPJ_HELPER =
  "Aceita letras e numeros; os 2 ultimos caracteres (DV) sao numericos.";
export const MSG_CNPJ_OBRIGATORIO = "CNPJ e obrigatorio.";
export const MSG_CNPJ_INVALIDO = "CNPJ invalido.";

const CNPJ_REGEX = /^[0-9A-Z]{12}[0-9]{2}$/;
/**
 * Mascara aceita no meio do valor: so ".", "/", "-" e o espaco comum
 * (U+0020), exatamente como cnpj.Normalizar no Go. Tab, NBSP, quebra de linha
 * etc. NAO sao mascara e tornam o CNPJ invalido.
 */
const MASK_CHARS = /[./\- ]/g;
/**
 * Espacos das bordas removidos pelo strings.TrimSpace do service
 * (validarClienteInput): propriedade Unicode White_Space, igual a
 * unicode.IsSpace do Go. Nao usa String.prototype.trim()/\s, que diferem
 * (incluem U+FEFF e nao incluem U+0085).
 */
const GO_SPACE =
  "\\t\\n\\v\\f\\r \\u0085\\u00A0\\u1680\\u2000-\\u200A\\u2028\\u2029\\u202F\\u205F\\u3000";
const EDGE_SPACES = new RegExp(`^[${GO_SPACE}]+|[${GO_SPACE}]+$`, "g");
const PESOS_DV1 = [5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2];
const PESOS_DV2 = [6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2];

/**
 * Espelha o fluxo da API: strings.TrimSpace + cnpj.Normalizar. Apara os
 * espacos das bordas, remove a mascara ([./- ]) e passa para maiusculas SO as
 * letras ASCII a-z (toUpperCase() generico transformaria "ı" em "I").
 * Qualquer outro caractere e mantido para que a validacao o rejeite, como o
 * back faz.
 */
export function normalizeCnpj(value: string | null | undefined): string {
  return (value ?? "")
    .replace(EDGE_SPACES, "")
    .replace(MASK_CHARS, "")
    .replace(/[a-z]/g, (c) => c.toUpperCase());
}

function calcDV(base: string, pesos: number[]): number {
  let soma = 0;
  for (let i = 0; i < pesos.length; i++) {
    soma += (base.charCodeAt(i) - 48) * pesos[i];
  }
  const resto = soma % 11;
  return resto < 2 ? 0 : 11 - resto;
}

/** Valida o CNPJ (numerico ou alfanumerico), com ou sem mascara. */
export function isValidCnpj(value: string | null | undefined): boolean {
  const cnpj = normalizeCnpj(value);
  if (!CNPJ_REGEX.test(cnpj)) return false;
  if (/^(.)\1*$/.test(cnpj)) return false;
  const dv1 = calcDV(cnpj, PESOS_DV1);
  if (dv1 !== Number(cnpj[12])) return false;
  const dv2 = calcDV(cnpj, PESOS_DV2);
  return dv2 === Number(cnpj[13]);
}

/**
 * Mensagem de erro de validacao do formulario, ou null se o CNPJ e valido.
 */
export function validateCnpj(value: string | null | undefined): string | null {
  const cnpj = normalizeCnpj(value);
  if (!cnpj) return MSG_CNPJ_OBRIGATORIO;
  return isValidCnpj(cnpj) ? null : MSG_CNPJ_INVALIDO;
}

/**
 * Filtra o que o usuario digitou/colou: descarta mascara e caracteres fora de
 * ASCII [0-9A-Za-z] ANTES de converter para maiusculas (assim "ı"/"ß" sao
 * descartados em vez de virarem "I"/"SS"), aceita letras so nas 12
 * primeiras posicoes (as 2 do DV aceitam so digitos) e limita a 14.
 */
export function sanitizeCnpjInput(value: string | null | undefined): string {
  const chars = (value ?? "").replace(/[^0-9A-Za-z]/g, "").toUpperCase();
  let out = "";
  for (const ch of chars) {
    if (out.length >= CNPJ_LENGTH) break;
    if (out.length < CNPJ_BASE_LENGTH || /[0-9]/.test(ch)) out += ch;
  }
  return out;
}

/** Aplica a mascara XX.XXX.XXX/XXXX-XX progressivamente (valor parcial). */
function applyMask(raw: string): string {
  let out = raw.slice(0, 2);
  if (raw.length > 2) out += "." + raw.slice(2, 5);
  if (raw.length > 5) out += "." + raw.slice(5, 8);
  if (raw.length > 8) out += "/" + raw.slice(8, 12);
  if (raw.length > 12) out += "-" + raw.slice(12, 14);
  return out;
}

/**
 * Valor para o input controlado: sanitiza e aplica a mascara parcial
 * enquanto o usuario digita (letras viram maiusculas).
 */
export function maskCnpjInput(value: string | null | undefined): string {
  return applyMask(sanitizeCnpjInput(value));
}

/**
 * Formatacao para exibicao (tabelas, detalhes). Com 14 caracteres no formato
 * do CNPJ (numerico ou alfanumerico) aplica a mascara; caso contrario devolve
 * o valor original, sem esconder dado legado fora do padrao.
 */
export function formatCnpj(value: string | null | undefined): string {
  const cnpj = normalizeCnpj(value);
  if (!CNPJ_REGEX.test(cnpj)) return value ?? "";
  return applyMask(cnpj);
}
