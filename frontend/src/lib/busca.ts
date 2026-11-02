// CHORE-10: limite dos campos de busca das listagens com busca no backend
// (`q` do SEC-25, ou `sku` do estoque). E so UX: o usuario nao digita alem do
// limite e nao descobre o teto pelo 400. O limite real continua no backend
// (acima de 100 caracteres, 400).
export const MAX_BUSCA = 100;
