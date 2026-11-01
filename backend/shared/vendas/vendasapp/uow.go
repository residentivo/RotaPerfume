package vendasapp

import (
	"context"
	"errors"
	"fmt"
	"log"
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

// CriarPagamento aplica o POST de pagamento na tx (SEC-35): trava o pedido
// (inexistente → 404 "pedido não encontrado", também para o admin; outro
// erro → 500), confere a posse sob o lock (de outro vendedor, o mesmo 404),
// valida os campos (400) e grava. Um DELETE do pedido concorrente espera o
// lock; o 1452 da FK, se ainda vier, também é 404 (adapter).
func (r TxPedido) CriarPagamento(ctx context.Context, e kernel.Escopo, d vendas.DadosNovoPagamento) (*vendas.Pagamento, error) {
	if _, err := r.travarNoEscopo(ctx, e, d.PedidoID, "TxPedido.CriarPagamento"); err != nil {
		return nil, err
	}
	pag, err := d.Pagamento(d.PedidoID)
	if err != nil {
		return nil, err
	}
	if err = r.Pagamentos.Inserir(ctx, pag); err != nil {
		return nil, err
	}
	return pag, nil
}

// ExcluirPagamento aplica o DELETE de pagamento na tx (SEC-35), na ordem de
// lock pedido → pagamento (a mesma do Excluir do pedido): no escopo
// restrito trava o pedido e confere a posse (V-02: qualquer falha → 404
// "pagamento não encontrado"); o admin não trava o pedido. Depois trava o
// pagamento (sumiu ou mudou de pedido → 404), aplica a regra de quitado
// (409) e apaga com o DELETE condicionado a não quitado.
func (r TxPedido) ExcluirPagamento(ctx context.Context, e kernel.Escopo, id, pedidoID int64) error {
	if err := r.posseDoPagamentoTravada(ctx, e, pedidoID); err != nil {
		return err
	}
	atual, err := r.Pagamentos.TravarParaExcluir(ctx, id)
	if err != nil {
		return err
	}
	if atual.PedidoID != pedidoID {
		vlog.Printf(arqUoW, "TxPedido.ExcluirPagamento", "decisão pedido_divergente id=%d", id)
		return vendas.ErrPagamentoNaoEncontrado
	}
	if err = atual.PodeExcluir(); err != nil {
		vlog.Printf(arqUoW, "TxPedido.ExcluirPagamento", "decisão bloqueado quitado id=%d", id)
		return err
	}
	return r.Pagamentos.ExcluirSeNaoQuitado(ctx, id)
}

// posseDoPagamentoTravada é o posseDoPagamento do Delete sob lock: só no
// escopo restrito, trava o pedido do pagamento; QUALQUER falha é 404
// "pagamento não encontrado" (V-02), com o log operacional do legado.
func (r TxPedido) posseDoPagamentoTravada(ctx context.Context, e kernel.Escopo, pedidoID int64) error {
	if !restrito(e) {
		return nil
	}
	ped, err := r.Pedidos.TravarParaEscrita(ctx, pedidoID)
	if err != nil {
		log.Printf("[pagamentos] DeletePagamento vendedor do pedido: %v", err)
		return vendas.ErrPagamentoNaoEncontrado
	}
	if !e.Permite(ped.VendedorID) {
		vlog.Printf(arqUoW, "TxPedido.posseDoPagamentoTravada", "decisão fora_da_carteira pedido_id=%d", pedidoID)
		return vendas.ErrPagamentoNaoEncontrado
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
