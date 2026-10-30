package vendasapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const arqUoW = "uow.go"

// O passo abaixo roda DENTRO do Transactor[TxPedido] (§5.2, C9): a
// tx sobe do repositório legado para a aplicação, com a MESMA fronteira e a
// mesma sequência de SQL. Use só os repositórios de r (O8-ii). É
// exportado porque é o roteiro da unidade de trabalho, também exercitado
// pelos testes de repositório portados (mesmos nomes do legado).

// Atualizar aplica o PUT de pedido na tx: lê o status ATUAL com lock e os
// itens atuais, decide pelo domínio (vendas.PlanejarAtualizacao: máquina de
// status do BUG-16 e faturado/entregue não troca itens), grava o cabeçalho, substitui os itens se o pedido não
// estava faturado e, na transição para Faturado, baixa o estoque de cada
// item no dia quando. Qualquer falha desfaz tudo (rollback do faturamento).
func (r TxPedido) Atualizar(ctx context.Context, id int64, p *vendas.Pedido, quando time.Time) error {
	statusAtual, err := r.Pedidos.StatusParaAtualizar(ctx, id)
	if err != nil {
		return err
	}
	itensAtuais, err := r.Pedidos.ItensAtuais(ctx, id)
	if err != nil {
		return err
	}
	plano, err := vendas.PlanejarAtualizacao(statusAtual, itensAtuais, p)
	if err != nil {
		vlog.Printf(arqUoW, "TxPedido.Atualizar", "decisão bloqueado %s id=%d", motivoBloqueio(err), id)
		return err
	}
	if err = r.Pedidos.AtualizarCabecalho(ctx, id, p); err != nil {
		return err
	}
	if plano.ReescreverItens {
		if err = r.Pedidos.SubstituirItens(ctx, id, p.Itens); err != nil {
			return err
		}
	}
	if plano.BaixarEstoque {
		return r.baixarEstoque(ctx, p.Itens, quando)
	}
	return nil
}

// Criar grava o pedido e, se ele nasce Faturado (BUG-20), baixa o estoque
// de cada item no dia quando, na mesma tx: falha na baixa desfaz o INSERT.
func (r TxPedido) Criar(ctx context.Context, p *vendas.Pedido, baixarEstoque bool, quando time.Time) error {
	if err := r.Pedidos.Inserir(ctx, p); err != nil {
		return err
	}
	if baixarEstoque {
		return r.baixarEstoque(ctx, p.Itens, quando)
	}
	return nil
}

// motivoBloqueio rotula a recusa do PUT para o vlog (sem valores).
func motivoBloqueio(err error) string {
	if errors.Is(err, vendas.ErrPedidoJaFaturadoNaoPodeAlterarItens) {
		return "itens_faturado"
	}
	return "status_faturado"
}

// baixarEstoque dá baixa de cada item no dia do faturamento (delta = a
// quantidade). O wrap leva só o nome da operação (O6-iv).
func (r TxPedido) baixarEstoque(ctx context.Context, itens []vendas.ItemPedido, quando time.Time) error {
	for _, it := range itens {
		sku, err := r.Estoque.SKUDoProduto(ctx, it.ProdutoID)
		if err != nil {
			return err
		}
		if err = r.Estoque.AjustarSaldoPorFaturamento(ctx, sku, quando, it.Quantidade); err != nil {
			return fmt.Errorf("vendasapp: baixa de estoque no faturamento do pedido: %w", err)
		}
	}
	return nil
}
