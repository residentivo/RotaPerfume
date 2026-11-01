package vendasapp

import (
	"context"
	"log"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const arqPedidos = "pedidos.go"

// ListarPedidos pagina os pedidos do escopo (E-03): restrito vê só a
// própria carteira (o vendedor_id da query é ignorado); sem carteira, 200
// vazio sem SQL.
func (c *Casos) ListarPedidos(ctx context.Context, p kernel.Principal, f vendas.FiltroPedido, pg kernel.Pagina) (r kernel.Resultado[vendas.Pedido], err error) {
	defer vlog.Trace(arqPedidos, "Vendas.ListarPedidos", "page=%d limit=%d busca=%t ordem=%s",
		pg.Numero(), pg.Limite(), f.Busca != "", f.Ordenacao.Campo())(&err)
	e, err := c.d.Escopo.Resolver(ctx, p)
	if err != nil {
		return r, err
	}
	if e.SemAcesso() {
		return vazio[vendas.Pedido](pg), nil
	}
	return c.d.Pedidos.Listar(ctx, e, f, pg)
}

// ObterPedido devolve o pedido com os itens; de outro vendedor responde
// como inexistente (E-04).
func (c *Casos) ObterPedido(ctx context.Context, p kernel.Principal, id int64) (ped *vendas.Pedido, err error) {
	defer vlog.Trace(arqPedidos, "Vendas.ObterPedido", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, vendas.ErrPedidoNaoEncontrado)
	if err != nil {
		return nil, err
	}
	return c.detalheNoEscopo(ctx, e, id)
}

// CriarPedido: sem carteira, 403 antes do decode; restrito grava sempre com
// o próprio vendedor e só para cliente da carteira ativa (E-05). Os valores
// vêm do domínio (V-03); cabeçalho e itens numa tx (C9). Criado como
// Faturado, baixa o estoque na mesma tx; como Entregue, 400 (BUG-20).
func (c *Casos) CriarPedido(ctx context.Context, p kernel.Principal, ler Leitor[vendas.DadosPedido]) (ped *vendas.Pedido, err error) {
	defer vlog.Trace(arqPedidos, "Vendas.CriarPedido", "user_id=%d", p.UserID())(&err)
	e, err := c.noEscopo(ctx, p, vendas.ErrSemVendedor)
	if err != nil {
		return nil, err
	}
	novo, err := c.pedidoDoComando(ctx, e, ler)
	if err != nil {
		return nil, err
	}
	baixar, err := vendas.PlanejarCriacao(novo)
	if err != nil {
		vlog.Printf(arqPedidos, "Vendas.CriarPedido", "decisão bloqueado status_entregue_sem_faturamento")
		return nil, err
	}
	quando := c.d.Relogio()
	err = c.d.TxPedido.WithTx(ctx, func(r TxPedido) error { return r.Criar(ctx, novo, baixar, quando) })
	if err != nil {
		return nil, err
	}
	if ped, err = c.detalhe(ctx, novo.PedidoIDOrigem); err != nil {
		return nil, err
	}
	log.Printf("[pedidos] criado: id=%d por usuario role=%s", ped.PedidoIDOrigem, p.Role())
	return ped, nil
}

// AtualizarPedido: a posse do registro atual é checada antes do decode (só
// no escopo restrito, como no legado: 404 antes do 400) e de novo sob o
// lock da tx (SEC-26); o vendedor não pode ser reatribuído por restrito
// (E-05). Itens e baixa de estoque na tx do pedido.
func (c *Casos) AtualizarPedido(ctx context.Context, p kernel.Principal, id int64, ler Leitor[vendas.DadosPedido]) (ped *vendas.Pedido, err error) {
	defer vlog.Trace(arqPedidos, "Vendas.AtualizarPedido", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, vendas.ErrPedidoNaoEncontrado)
	if err != nil {
		return nil, err
	}
	if restrito(e) {
		if _, err = c.detalheNoEscopo(ctx, e, id); err != nil {
			return nil, err
		}
	}
	novo, err := c.pedidoDoComando(ctx, e, ler)
	if err != nil {
		return nil, err
	}
	quando := c.d.Relogio()
	err = c.d.TxPedido.WithTx(ctx, func(r TxPedido) error { return r.Atualizar(ctx, e, id, novo, quando) })
	if err != nil {
		return nil, err
	}
	if ped, err = c.detalhe(ctx, id); err != nil {
		return nil, err
	}
	log.Printf("[pedidos] atualizado: id=%d por usuario role=%s", id, p.Role())
	return ped, nil
}

// ExcluirPedido remove o pedido e os itens (hard delete), só da própria
// carteira e respeitando as travas V-01 (pagamentos vinculados; Faturado ou
// Entregue). Existência, posse, pagamentos e status são lidos DENTRO da tx,
// sob o lock do pedido (SEC-26), na mesma ordem de erros do legado.
func (c *Casos) ExcluirPedido(ctx context.Context, p kernel.Principal, id int64) (err error) {
	defer vlog.Trace(arqPedidos, "Vendas.ExcluirPedido", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, vendas.ErrPedidoNaoEncontrado)
	if err != nil {
		return err
	}
	if err = c.d.TxPedido.WithTx(ctx, func(r TxPedido) error { return r.Excluir(ctx, e, id) }); err != nil {
		return err
	}
	log.Printf("[pedidos] excluído: id=%d por usuario role=%s", id, p.Role())
	return nil
}

// pedidoDoComando lê o body (depois da autorização), aplica a política de
// escrita e monta o aggregate pelo domínio, na ordem do legado: carteira
// antes da validação dos campos.
func (c *Casos) pedidoDoComando(ctx context.Context, e kernel.Escopo, ler Leitor[vendas.DadosPedido]) (*vendas.Pedido, error) {
	d, err := ler()
	if err != nil {
		return nil, err
	}
	if d.VendedorID, err = c.vendedorDoComando(ctx, e, d.ClienteID, d.VendedorID); err != nil {
		return nil, err
	}
	return vendas.NovoPedido(d)
}

// detalhe lê o cabeçalho e os itens (master-detail).
func (c *Casos) detalhe(ctx context.Context, id int64) (*vendas.Pedido, error) {
	ped, err := c.d.Pedidos.PorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ped.Itens, err = c.d.Pedidos.Itens(ctx, id); err != nil {
		return nil, err
	}
	return ped, nil
}

// detalheNoEscopo é o detalhe escondido como inexistente quando o pedido é
// de outro vendedor (E-04: nunca 403).
func (c *Casos) detalheNoEscopo(ctx context.Context, e kernel.Escopo, id int64) (*vendas.Pedido, error) {
	ped, err := c.detalhe(ctx, id)
	if err != nil {
		return nil, err
	}
	if !e.Permite(ped.VendedorID) {
		vlog.Printf(arqPedidos, "Vendas.detalheNoEscopo", "decisão fora_da_carteira id=%d", id)
		return nil, vendas.ErrPedidoNaoEncontrado
	}
	return ped, nil
}
