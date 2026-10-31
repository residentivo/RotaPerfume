package vendasapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const arqUoW = "uow.go"

// O passo abaixo roda DENTRO do Transactor[TxPedido] (§5.2, C9): a
// tx sobe do repositório legado para a aplicação, com a MESMA fronteira e a
// mesma sequência de SQL. Use só os repositórios de r (O8-ii). É
// exportado porque é o roteiro da unidade de trabalho, também exercitado
// pelos testes de repositório portados (mesmos nomes do legado).

// Atualizar aplica o PUT de pedido na tx: trava o pedido (status e
// vendedor), confere a posse sob o lock (SEC-26: de outro vendedor, 404),
// lê os itens atuais, decide pelo domínio (vendas.PlanejarAtualizacao:
// máquina de status do BUG-16 e faturado/entregue não troca itens), grava
// o cabeçalho, substitui os itens se o pedido não estava faturado e, na
// transição para Faturado, baixa o estoque de cada item no dia quando.
// Qualquer falha desfaz tudo (rollback do faturamento).
func (r TxPedido) Atualizar(ctx context.Context, e kernel.Escopo, id int64, p *vendas.Pedido, quando time.Time) error {
	atual, err := r.travarNoEscopo(ctx, e, id, "TxPedido.Atualizar")
	if err != nil {
		return err
	}
	itensAtuais, err := r.Pedidos.ItensAtuais(ctx, id)
	if err != nil {
		return err
	}
	plano, err := vendas.PlanejarAtualizacao(atual.Status, itensAtuais, p)
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

// Excluir aplica o DELETE de pedido na tx (SEC-26), na ordem de erros do
// legado: trava o pedido (404), confere a posse sob o lock (404), lê os
// pagamentos e aplica as travas V-01 (409s) antes de apagar itens e
// cabeçalho.
func (r TxPedido) Excluir(ctx context.Context, e kernel.Escopo, id int64) error {
	atual, err := r.travarNoEscopo(ctx, e, id, "TxPedido.Excluir")
	if err != nil {
		return err
	}
	temPagamentos, err := r.Pagamentos.ExistePorPedido(ctx, id)
	if err != nil {
		return err
	}
	if err = atual.PodeExcluir(temPagamentos); err != nil {
		vlog.Printf(arqUoW, "TxPedido.Excluir", "decisão bloqueado id=%d pagamentos=%t status_faturado=%t status_entregue=%t",
			id, temPagamentos, vendas.StatusPedido(atual.Status) == vendas.StatusFaturado, vendas.StatusPedido(atual.Status) == vendas.StatusEntregue)
		return err
	}
	return r.Pedidos.Excluir(ctx, id)
}

// travarNoEscopo trava o pedido e o esconde como inexistente quando, sob o
// lock, ele é de outro vendedor (E-04: nunca 403). Fecha o TOCTOU entre a
// checagem de posse antes do decode e a escrita.
func (r TxPedido) travarNoEscopo(ctx context.Context, e kernel.Escopo, id int64, rotulo string) (*vendas.Pedido, error) {
	atual, err := r.Pedidos.TravarParaEscrita(ctx, id)
	if err != nil {
		return nil, err
	}
	if !e.Permite(atual.VendedorID) {
		vlog.Printf(arqUoW, rotulo, "decisão fora_da_carteira id=%d", id)
		return nil, vendas.ErrPedidoNaoEncontrado
	}
	return atual, nil
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
