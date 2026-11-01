package vendasapp

import (
	"context"
	"errors"
	"log"

	"github.com/rotaperfumes/shared/appkit"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const (
	arqPagamentos         = "pagamentos.go"
	rotuloPagamentoRelido = "[pagamentos] criado, mas falhou a releitura: pagamento_id"
)

// O escopo de pagamento é HERDADO do pedido (V-02): a tabela pagamentos não
// tem vendedor_id; a posse é a de pedidos.vendedor_id.

// ListarPagamentos pagina os pagamentos do escopo (E-03: restrito vê só os
// de pedidos da própria carteira; sem carteira, 200 vazio sem SQL). O
// filtro da query só é lido DEPOIS do escopo (pedido_id inválido → 400 só
// para quem tem acesso, como no legado).
func (c *Casos) ListarPagamentos(ctx context.Context, p kernel.Principal, ler Leitor[vendas.FiltroPagamento], pg kernel.Pagina) (r kernel.Resultado[vendas.Pagamento], err error) {
	defer vlog.Trace(arqPagamentos, "Vendas.ListarPagamentos", "page=%d limit=%d", pg.Numero(), pg.Limite())(&err)
	e, err := c.d.Escopo.Resolver(ctx, p)
	if err != nil {
		return r, err
	}
	if e.SemAcesso() {
		return vazio[vendas.Pagamento](pg), nil
	}
	f, err := ler()
	if err != nil {
		return r, err
	}
	return c.d.Pagamentos.Listar(ctx, e, f, pg)
}

// ObterPagamento devolve o pagamento; de pedido de outro vendedor responde
// como inexistente (E-04). Falha ao ler o pedido do pagamento também é 404
// (V-02: o FK garante o pedido; na dúvida, "sem acesso", nunca 500).
func (c *Casos) ObterPagamento(ctx context.Context, p kernel.Principal, id int64) (pag *vendas.Pagamento, err error) {
	defer vlog.Trace(arqPagamentos, "Vendas.ObterPagamento", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, vendas.ErrPagamentoNaoEncontrado)
	if err != nil {
		return nil, err
	}
	if pag, err = c.d.Pagamentos.PorID(ctx, id); err != nil {
		return nil, err
	}
	if err = c.posseDoPagamento(ctx, e, pag.PedidoID, "GetPagamento"); err != nil {
		return nil, err
	}
	return pag, nil
}

// CriarPagamento: sem carteira, 403 antes do decode; pedido_id obrigatório
// (400, sem consulta). O resto roda no TxPedido com o pedido travado
// (SEC-35, TxPedido.CriarPagamento): pedido inexistente ou, no restrito, de
// outra carteira → 404 "pedido não encontrado"; depois os campos (400).
func (c *Casos) CriarPagamento(ctx context.Context, p kernel.Principal, ler Leitor[vendas.DadosNovoPagamento]) (pag *vendas.Pagamento, err error) {
	defer vlog.Trace(arqPagamentos, "Vendas.CriarPagamento", "user_id=%d", p.UserID())(&err)
	e, err := c.noEscopo(ctx, p, vendas.ErrSemVendedor)
	if err != nil {
		return nil, err
	}
	d, err := ler()
	if err != nil {
		return nil, err
	}
	if d.PedidoID <= 0 {
		return nil, vendas.ErrPedidoIDObrigatorio
	}
	err = c.d.TxPedido.WithTx(ctx, func(r TxPedido) error {
		pag, err = r.CriarPagamento(ctx, e, d)
		return err
	})
	if err != nil {
		return nil, err
	}
	log.Printf("[pagamentos] criado: pagamento_id=%d pedido_id=%d", pag.PagamentoID, pag.PedidoID)
	return appkit.RelerCriado(ctx, rotuloPagamentoRelido, pag.PagamentoID, pag, c.d.Pagamentos.PorID), nil
}

// AtualizarPagamento: no escopo restrito, a posse (via pedido do pagamento
// atual) é checada antes do decode. O comando não tem pedido_id (V-02):
// não há como mover o pagamento para pedido de outro vendedor. Pagamento
// quitado só o admin edita (SEC-27): para o restrito, 409 depois da
// validação do comando (400).
func (c *Casos) AtualizarPagamento(ctx context.Context, p kernel.Principal, id int64, ler Leitor[vendas.DadosPagamento]) (pag *vendas.Pagamento, err error) {
	defer vlog.Trace(arqPagamentos, "Vendas.AtualizarPagamento", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, vendas.ErrPagamentoNaoEncontrado)
	if err != nil {
		return nil, err
	}
	if err = c.posseParaEditar(ctx, e, id); err != nil {
		return nil, err
	}
	d, err := ler()
	if err != nil {
		return nil, err
	}
	if pag, err = d.Pagamento(0); err != nil { // o UPDATE não grava pedido_id
		return nil, err
	}
	if err = c.gravarEdicao(ctx, e, id, pag); err != nil {
		return nil, err
	}
	log.Printf("[pagamentos] atualizado: pagamento_id=%d", id)
	return appkit.RelerAtualizado(ctx, "vendasapp", id, c.d.Pagamentos.PorID)
}

// ExcluirPagamento remove o pagamento (hard delete), só da própria carteira
// e nunca quitado (409). A leitura fora da tx só obtém o pedido (imutável,
// V-02); posse e status são conferidos sob lock no TxPedido (SEC-35,
// TxPedido.ExcluirPagamento).
func (c *Casos) ExcluirPagamento(ctx context.Context, p kernel.Principal, id int64) (err error) {
	defer vlog.Trace(arqPagamentos, "Vendas.ExcluirPagamento", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, vendas.ErrPagamentoNaoEncontrado)
	if err != nil {
		return err
	}
	pag, err := c.d.Pagamentos.PorID(ctx, id)
	if err != nil {
		return err
	}
	err = c.d.TxPedido.WithTx(ctx, func(r TxPedido) error { return r.ExcluirPagamento(ctx, e, id, pag.PedidoID) })
	if err != nil {
		return err
	}
	log.Printf("[pagamentos] excluído: pagamento_id=%d", id)
	return nil
}

// gravarEdicao: o admin grava com o UPDATE normal; o restrito, com o UPDATE
// condicionado a "não quitado" (SEC-27, atômico). Zero linhas no restrito:
// relê para separar o inexistente (404) do quitado (409). Se a releitura
// achar o pagamento editável, ele mudou entre o UPDATE e a releitura
// (edição concorrente): também 409, sem nova tentativa.
func (c *Casos) gravarEdicao(ctx context.Context, e kernel.Escopo, id int64, pag *vendas.Pagamento) error {
	if !restrito(e) {
		return c.d.Pagamentos.Atualizar(ctx, id, pag)
	}
	err := c.d.Pagamentos.AtualizarSeNaoQuitado(ctx, id, pag)
	if !errors.Is(err, vendas.ErrPagamentoNaoEncontrado) {
		return err
	}
	atual, err := c.d.Pagamentos.PorID(ctx, id)
	if err != nil {
		return err
	}
	log.Printf("[pagamentos] edição bloqueada: pagamento_id=%d quitado", id)
	if err = atual.PodeEditar(); err != nil {
		return err
	}
	return vendas.ErrPagamentoJaQuitadoNaoPodeSerEditado
}

// pedidoNoEscopo (Update) exige o pedido na carteira do escopo restrito: inexistente
// ou de outro vendedor → naoEncontrado (mesmo corpo, sem enumeração); erro
// de banco → 500.
func (c *Casos) pedidoNoEscopo(ctx context.Context, e kernel.Escopo, pedidoID int64, naoEncontrado error) error {
	ped, err := c.d.Pedidos.PorID(ctx, pedidoID)
	switch {
	case errors.Is(err, vendas.ErrPedidoNaoEncontrado):
		return naoEncontrado
	case err != nil:
		return err
	case !e.Permite(ped.VendedorID):
		vlog.Printf(arqPagamentos, "Vendas.pedidoNoEscopo", "decisão fora_da_carteira pedido_id=%d", pedidoID)
		return naoEncontrado
	}
	return nil
}

// posseDoPagamento (Get, só no escopo restrito): QUALQUER falha ao ler o
// pedido do pagamento é 404 "pagamento não encontrado" (V-02), com o log
// operacional do legado; fora da carteira, o mesmo 404. O Delete faz a
// mesma conferência sob lock (TxPedido.posseDoPagamentoTravada).
func (c *Casos) posseDoPagamento(ctx context.Context, e kernel.Escopo, pedidoID int64, operacao string) error {
	if !restrito(e) {
		return nil
	}
	ped, err := c.d.Pedidos.PorID(ctx, pedidoID)
	if err != nil {
		log.Printf("[pagamentos] %s vendedor do pedido: %v", operacao, err)
		return vendas.ErrPagamentoNaoEncontrado
	}
	if !e.Permite(ped.VendedorID) {
		vlog.Printf(arqPagamentos, "Vendas.posseDoPagamento", "decisão fora_da_carteira pedido_id=%d", pedidoID)
		return vendas.ErrPagamentoNaoEncontrado
	}
	return nil
}

// posseParaEditar (Update, só no escopo restrito): lê o pagamento atual e
// exige o pedido dele na carteira. Aqui, diferente de Get/Delete, a falha
// de banco ao ler o pedido é 500 (contrato legado do pedidoNoEscopo).
func (c *Casos) posseParaEditar(ctx context.Context, e kernel.Escopo, id int64) error {
	if !restrito(e) {
		return nil
	}
	atual, err := c.d.Pagamentos.PorID(ctx, id)
	if err != nil {
		return err
	}
	return c.pedidoNoEscopo(ctx, e, atual.PedidoID, vendas.ErrPagamentoNaoEncontrado)
}
