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

// travarNoEscopo trava o pedido e o esconde como inexistente quando ele é
// de outro vendedor (E-04: nunca 403). Fecha o TOCTOU entre a checagem de
// posse antes do decode e a escrita. No escopo restrito (SEC-38), a ordem
// é: (1) leitura sem lock (inexistente/fora da carteira → 404; infra →
// 500), para o restrito não travar pedido alheio; (2) o lock (sumiu → 404;
// infra → 500); (3) a posse de novo, sob o lock (→ 404). O admin só trava.
func (r TxPedido) travarNoEscopo(ctx context.Context, e kernel.Escopo, id int64, rotulo string) (*vendas.Pedido, error) {
	return r.travarComPosse(ctx, e, id, rotulo, vendas.ErrPedidoNaoEncontrado)
}

// travarComPosse é o roteiro do SEC-38 com o 404 de cada operação
// (naoEncontrado): pedido não encontrado no PUT/DELETE de pedido e no POST
// de pagamento; pagamento não encontrado no DELETE de pagamento.
func (r TxPedido) travarComPosse(ctx context.Context, e kernel.Escopo, id int64, rotulo string, naoEncontrado error) (*vendas.Pedido, error) {
	if restrito(e) {
		if err := r.posseSemLock(ctx, e, id, rotulo, naoEncontrado); err != nil {
			return nil, err
		}
	}
	atual, err := r.Pedidos.TravarParaEscrita(ctx, id)
	if errors.Is(err, vendas.ErrPedidoNaoEncontrado) {
		return nil, naoEncontrado
	}
	if err != nil {
		log.Printf("[pedidos] %s: travar pedido_id=%d: %v", rotulo, id, err)
		return nil, err
	}
	if !e.Permite(atual.VendedorID) {
		vlog.Printf(arqUoW, rotulo, "decisão fora_da_carteira_sob_lock id=%d", id)
		return nil, naoEncontrado
	}
	return atual, nil
}

// posseSemLock é o passo (1) do SEC-38: lê o vendedor sem lock e recusa o
// pedido inexistente ou de outro vendedor (404) antes de travar.
func (r TxPedido) posseSemLock(ctx context.Context, e kernel.Escopo, id int64, rotulo string, naoEncontrado error) error {
	vendedorID, err := r.Pedidos.VendedorDoPedido(ctx, id)
	if errors.Is(err, vendas.ErrPedidoNaoEncontrado) {
		return naoEncontrado
	}
	if err != nil {
		log.Printf("[pedidos] %s: vendedor do pedido_id=%d: %v", rotulo, id, err)
		return err
	}
	if !e.Permite(vendedorID) {
		vlog.Printf(arqUoW, rotulo, "decisão fora_da_carteira id=%d", id)
		return naoEncontrado
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
// restrito confere a posse e trava o pedido (V-02/SEC-38: inexistente ou
// fora da carteira → 404 "pagamento não encontrado"; erro de banco → 500);
// o admin não trava o pedido. Depois trava o
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
// escopo restrito, o roteiro do SEC-38 (leitura sem lock → lock → posse sob
// o lock) com o 404 "pagamento não encontrado"; erro de banco → 500.
func (r TxPedido) posseDoPagamentoTravada(ctx context.Context, e kernel.Escopo, pedidoID int64) error {
	if !restrito(e) {
		return nil
	}
	_, err := r.travarComPosse(ctx, e, pedidoID, "TxPedido.posseDoPagamentoTravada", vendas.ErrPagamentoNaoEncontrado)
	return err
}

// motivoBloqueio rotula a recusa do PUT para o vlog (sem valores), um
// rótulo por regra da máquina de status (CHORE-08).
func motivoBloqueio(err error) string {
	switch {
	case errors.Is(err, vendas.ErrPedidoJaFaturadoNaoPodeAlterarItens):
		return "itens_faturado"
	case errors.Is(err, vendas.ErrPedidoFaturadoSoPodeIrParaEntregue):
		return "status_faturado_so_entregue"
	case errors.Is(err, vendas.ErrPedidoEntregueNaoPodeMudarStatus):
		return "status_entregue_terminal"
	case errors.Is(err, vendas.ErrPedidoEntregueSemFaturamento):
		return "status_entregue_sem_faturamento"
	default:
		return "outro"
	}
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
