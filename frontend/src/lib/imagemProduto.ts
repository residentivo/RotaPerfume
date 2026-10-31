/**
 * IMG-01: helpers da imagem do produto (sem dependencia do api.ts, para os
 * testes que mockam "@/lib/api" nao precisarem declara-los).
 */
import { vlog } from "./vlog";

const F = "imagemProduto.ts";

// Mesma base usada pelo apiClient (sem barra final).
const API_BASE = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/+$/, "");

/** Prefixo aceito para `imagem_url` (rota publica do backend). */
export const PREFIXO_IMAGEM_PRODUTO = "/api/public/produtos/imagens/";

/** Tipos aceitos no upload (o backend valida pelo conteudo). */
export const TIPOS_IMAGEM_PRODUTO = "image/jpeg,image/png,image/webp";

/** Limite do upload (1 MB), checado no cliente so para UX. */
export const MAX_BYTES_IMAGEM_PRODUTO = 1_048_576;

/**
 * URL absoluta da imagem do produto, ou null se nao houver imagem ou se o
 * caminho nao for o da rota publica de imagens (nunca renderiza URL arbitraria).
 */
export function urlImagemProduto(imagemUrl: string | null | undefined): string | null {
  vlog(F, "urlImagemProduto", "verificando prefixo da imagem_url");
  if (!imagemUrl || !imagemUrl.startsWith(PREFIXO_IMAGEM_PRODUTO)) return null;
  return `${API_BASE}${imagemUrl}`;
}

/** Mensagem de erro de validacao local do arquivo, ou null se ok. */
export function validarArquivoImagem(file: File): string | null {
  vlog(F, "validarArquivoImagem", "validando tipo e tamanho do arquivo:", file.type, file.size);
  if (!TIPOS_IMAGEM_PRODUTO.split(",").includes(file.type)) {
    return "arquivo de imagem inválido (use JPEG, PNG ou WebP)";
  }
  if (file.size > MAX_BYTES_IMAGEM_PRODUTO) {
    return "imagem excede 1 MB";
  }
  return null;
}
