import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import { friendlyPedidoError } from "@/components/admin/PedidoModal";

// CHORE-07 (Lote 31), contrato entre camadas: os textos de erro vem do
// backend (backend/shared/vendas/erros.go). Os outros testes copiam a
// string; este le a fonte do Go, entao uma mudanca so de um lado quebra aqui
// em vez de o usuario ver a mensagem crua do backend.
// O vitest roda com cwd em frontend/ (make test-frontend, npm test); no jsdom
// o import.meta.url nao e file://, por isso o caminho parte do cwd.
const ERROS_GO = resolve(process.cwd(), "..", "backend", "shared", "vendas", "erros.go");

function textoDoErro(nome: string): string {
  const fonte = readFileSync(ERROS_GO, "utf8");
  const re = new RegExp(`${nome}\\s*=\\s*kernel\\.NovoErro\\([^,]+,\\s*"([^"]+)"\\)`);
  const m = fonte.match(re);
  if (!m) throw new Error(`${nome} nao encontrado em erros.go`);
  return m[1];
}

describe("CHORE-07: textos do backend (erros.go) x friendlyPedidoError", () => {
  it.each<[string, string, string]>([
    [
      "ErrPedidoJaFaturadoNaoPodeAlterarItens",
      "pedido faturado ou entregue: não é possível alterar os itens",
      "Pedido faturado ou entregue: nao e possivel alterar os itens.",
    ],
    [
      "ErrPedidoFaturadoNaoPodeSerExcluido",
      "pedido faturado não pode ser excluído; só pode ter o status alterado para Entregue",
      "Pedido faturado nao pode ser excluido; so pode ter o status alterado para Entregue.",
    ],
  ])("%s tem o texto aprovado e vira a mensagem amigavel", (nome, aprovado, amigavel) => {
    const texto = textoDoErro(nome);
    expect(texto).toBe(aprovado);
    expect(friendlyPedidoError(texto)).toBe(amigavel);
  });

  it.each([
    "ErrPedidoFaturadoSoPodeIrParaEntregue",
    "ErrPedidoEntregueNaoPodeMudarStatus",
    "ErrPedidoEntregueSemFaturamento",
  ])("%s (BUG-16) continua traduzido", (nome) => {
    const texto = textoDoErro(nome);
    expect(friendlyPedidoError(texto)).not.toBe(texto);
  });

  it("o extrator falha alto para erro inexistente", () => {
    expect(() => textoDoErro("ErrQueNaoExiste")).toThrow(/nao encontrado/);
  });
});
