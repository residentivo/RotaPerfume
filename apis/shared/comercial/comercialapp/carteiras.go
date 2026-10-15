package comercialapp

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqCarteiras = "carteiras.go"

// VincularCliente inclui o cliente na carteira ativa do vendedor (admin).
// Se o cliente estiver na carteira de OUTRO vendedor, o vínculo anterior é
// encerrado (transferência implícita, histórico preservado). Revincular no
// mesmo dia reabre a linha do dia (a chave única é cliente+vendedor+data).
//
// P4/BUG-14: a sequência é a do legado, SEM transação (card próprio).
func (c *Casos) VincularCliente(ctx context.Context, p kernel.Principal, vendedorID, clienteID int64) (r *comercial.ClienteResumo, err error) {
	defer vlog.Trace(arqCarteiras, "Comercial.VincularCliente", "vendedor_id=%d cliente_id=%d", vendedorID, clienteID)(&err)
	if err = exigirAdmin(p); err != nil {
		return nil, err
	}
	if clienteID <= 0 {
		return nil, comercial.ErrClienteIDObrigatorio
	}
	cliente, err := c.vendedorEClienteExistentes(ctx, vendedorID, clienteID)
	if err != nil {
		return nil, err
	}
	vinculo, err := c.vincular(ctx, vendedorID, clienteID)
	if err != nil {
		return nil, err
	}
	log.Printf("[vendedores] cliente vinculado: vendedor_id=%d cliente_id=%d por usuario role=%s", vendedorID, clienteID, p.Role())
	return comercial.ResumoDoVinculo(cliente, vinculo), nil
}

// vendedorEClienteExistentes: 404 do vendedor, depois 404 do cliente.
func (c *Casos) vendedorEClienteExistentes(ctx context.Context, vendedorID, clienteID int64) (*comercial.Cliente, error) {
	existe, err := c.d.Vendedores.Existe(ctx, vendedorID)
	if err != nil {
		return nil, err
	}
	if !existe {
		return nil, comercial.ErrVendedorNaoEncontrado
	}
	return c.d.Clientes.PorID(ctx, clienteID)
}

// vincular encerra o vínculo ativo com outro vendedor (se houver) e cria
// ou reabre o vínculo do dia. "Não encontrado" na escrita de um vínculo
// recém-lido é corrida: 500, como no legado (interno tira o sentinela).
func (c *Casos) vincular(ctx context.Context, vendedorID, clienteID int64) (*comercial.Carteira, error) {
	agora := c.agora()
	anterior, err := c.d.Carteiras.VinculoAtivoDoCliente(ctx, clienteID)
	switch {
	case err == nil && anterior.VendedorID == vendedorID:
		return anterior, nil
	case err == nil:
		if err := c.d.Carteiras.Encerrar(ctx, anterior.CarteiraIDOrigem, agora); err != nil {
			return nil, interno(err)
		}
		vlog.Printf(arqCarteiras, "Comercial.VincularCliente", "decisão transferida cliente_id=%d de_vendedor_id=%d", clienteID, anterior.VendedorID)
	case !errors.Is(err, comercial.ErrVinculoNaoEncontrado):
		return nil, err
	}
	doDia, err := c.d.Carteiras.VinculoDoDia(ctx, clienteID, vendedorID, agora)
	switch {
	case err == nil && doDia.Ativa():
		return doDia, nil
	case err == nil:
		if err := c.d.Carteiras.Reabrir(ctx, doDia.CarteiraIDOrigem); err != nil {
			return nil, interno(err)
		}
		doDia.DataFim = nil
		return doDia, nil
	case !errors.Is(err, comercial.ErrVinculoNaoEncontrado):
		return nil, err
	}
	novo := &comercial.Carteira{ClienteID: clienteID, VendedorID: vendedorID, DataInicio: agora}
	if err := c.d.Carteiras.Inserir(ctx, novo); err != nil {
		return nil, err
	}
	return novo, nil
}

// DesvincularCliente encerra o vínculo ativo entre o vendedor e o cliente
// (não afeta vínculos do cliente com outros vendedores).
func (c *Casos) DesvincularCliente(ctx context.Context, p kernel.Principal, vendedorID, clienteID int64) (err error) {
	defer vlog.Trace(arqCarteiras, "Comercial.DesvincularCliente", "vendedor_id=%d cliente_id=%d", vendedorID, clienteID)(&err)
	if err = exigirAdmin(p); err != nil {
		return err
	}
	vinculo, err := c.d.Carteiras.VinculoAtivo(ctx, vendedorID, clienteID)
	if err != nil {
		return err
	}
	if err = c.d.Carteiras.Encerrar(ctx, vinculo.CarteiraIDOrigem, c.agora()); err != nil {
		return err
	}
	log.Printf("[vendedores] cliente desvinculado: vendedor_id=%d cliente_id=%d por usuario role=%s", vendedorID, clienteID, p.Role())
	return nil
}

// interno reembrulha SEM %w: o sentinela de domínio não pode virar 4xx
// quando o legado respondia 500 (erro cru do repositório).
func interno(err error) error {
	return fmt.Errorf("comercialapp: escrita de vínculo: %v", err)
}
