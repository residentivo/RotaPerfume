"use client";

import { Button } from "@/components/ui/Button";

interface PaginadorProps {
  /**
   * Pagina exibida. FE-10: nas listagens paginadas no servidor, se a ultima
   * carga falhou, e a ultima carregada com sucesso (a das linhas na tela).
   */
  pagina: number;
  /** Total de paginas. Com 1 pagina ou menos o paginador nao e exibido. */
  paginas: number;
  /** Pedido de navegacao para a pagina `n` (ja limitada a 1..paginas). */
  onIrPara: (n: number) => void;
}

/**
 * Rodape de paginacao das listagens (primeira, anterior, proxima, ultima).
 * FE-10: os botoes calculam o destino a partir de `pagina` (a exibida), e nao
 * da pagina pedida, para que uma troca que falhou nao desloque a navegacao.
 */
export function Paginador({ pagina, paginas, onIrPara }: PaginadorProps) {
  if (paginas <= 1) return null;

  return (
    <div className="flex flex-col items-center justify-between gap-3 border-t border-slate-200 px-4 py-3 sm:flex-row">
      <div className="text-sm text-slate-500">
        Pagina <strong>{pagina}</strong> de <strong>{paginas}</strong>
      </div>
      <div className="flex items-center gap-2">
        <Button
          size="sm"
          variant="secondary"
          disabled={pagina <= 1}
          onClick={() => onIrPara(1)}
          title="Primeira pagina"
        >
          {"<<"}
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={pagina <= 1}
          onClick={() => onIrPara(Math.max(1, pagina - 1))}
          title="Pagina anterior"
        >
          {"<"}
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={pagina >= paginas}
          onClick={() => onIrPara(Math.min(paginas, pagina + 1))}
          title="Proxima pagina"
        >
          {">"}
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={pagina >= paginas}
          onClick={() => onIrPara(paginas)}
          title="Ultima pagina"
        >
          {">>"}
        </Button>
      </div>
    </div>
  );
}
