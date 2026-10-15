package comercialapp

import (
	"context"
	"log"

	"github.com/rotaperfumes/shared/appkit"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqVendedores = "vendedores.go"

// ListarVendedores devolve a lista de seleção no escopo (E-06): admin vê
// todos (ativos e desligados); restrito, só o próprio; sem carteira, lista
// vazia ([] e não null); desligado, 403.
func (c *Casos) ListarVendedores(ctx context.Context, p kernel.Principal) (vs []comercial.VendedorResumo, err error) {
	defer vlog.Trace(arqVendedores, "Comercial.ListarVendedores", "user_id=%d", p.UserID())(&err)
	e, err := c.escopo.Resolver(ctx, p)
	if err != nil {
		return nil, err
	}
	if e.SemAcesso() {
		return []comercial.VendedorResumo{}, nil
	}
	return c.d.Vendedores.ListarResumo(ctx, e)
}

// ObterVendedor devolve o vendedor com os clientes da carteira ativa (admin).
func (c *Casos) ObterVendedor(ctx context.Context, p kernel.Principal, id int64) (d *comercial.DetalheVendedor, err error) {
	defer vlog.Trace(arqVendedores, "Comercial.ObterVendedor", "id=%d", id)(&err)
	if err = exigirAdmin(p); err != nil {
		return nil, err
	}
	v, err := c.d.Vendedores.PorID(ctx, id)
	if err != nil {
		return nil, err
	}
	clientes, err := c.d.Carteiras.ClientesDoVendedor(ctx, id)
	if err != nil {
		return nil, err
	}
	return &comercial.DetalheVendedor{Vendedor: *v, Clientes: clientes}, nil
}

// ClientesDoVendedor lista a carteira ativa do vendedor id. Restrito só vê a
// própria; a de outro vendedor responde como inexistente (E-06, E-04).
func (c *Casos) ClientesDoVendedor(ctx context.Context, p kernel.Principal, id int64) (cs []comercial.ClienteResumo, err error) {
	defer vlog.Trace(arqVendedores, "Comercial.ClientesDoVendedor", "id=%d", id)(&err)
	e, err := c.escopo.Resolver(ctx, p)
	if err != nil {
		return nil, err
	}
	if !e.Permite(id) {
		return nil, comercial.ErrVendedorNaoEncontrado
	}
	existe, err := c.d.Vendedores.Existe(ctx, id)
	if err != nil {
		return nil, err
	}
	if !existe {
		return nil, comercial.ErrVendedorNaoEncontrado
	}
	return c.d.Carteiras.ClientesDoVendedor(ctx, id)
}

// CriarVendedor valida, grava e devolve o vendedor relido (admin).
func (c *Casos) CriarVendedor(ctx context.Context, p kernel.Principal, d comercial.DadosVendedor) (v *comercial.Vendedor, err error) {
	defer vlog.Trace(arqVendedores, "Comercial.CriarVendedor", "")(&err)
	if err = exigirAdmin(p); err != nil {
		return nil, err
	}
	if v, err = comercial.NovoVendedor(d, c.agora()); err != nil {
		return nil, err
	}
	if err = c.d.Vendedores.Inserir(ctx, v); err != nil {
		return nil, err
	}
	log.Printf("[vendedores] criado: id=%d por usuario role=%s", v.ID, p.Role())
	return appkit.RelerCriado(ctx, "[vendedores] criado, mas falhou a releitura: id", v.ID, v, c.d.Vendedores.PorID), nil
}

// AtualizarVendedor grava os campos editáveis (não o desligamento).
func (c *Casos) AtualizarVendedor(ctx context.Context, p kernel.Principal, id int64, d comercial.DadosVendedor) (v *comercial.Vendedor, err error) {
	defer vlog.Trace(arqVendedores, "Comercial.AtualizarVendedor", "id=%d", id)(&err)
	if err = exigirAdmin(p); err != nil {
		return nil, err
	}
	if v, err = comercial.VendedorEditado(d); err != nil {
		return nil, err
	}
	if err = c.d.Vendedores.Atualizar(ctx, id, v); err != nil {
		return nil, err
	}
	log.Printf("[vendedores] atualizado: id=%d por usuario role=%s", id, p.Role())
	return appkit.RelerAtualizado(ctx, "comercialapp", id, c.d.Vendedores.PorID)
}

// DesligarVendedor é o soft-delete (I-09), numa tx única: grava
// data_desligamento com COALESCE (idempotente, BUG-04), inativa os usuários
// do vendedor gravando o corte de sessão e revoga os refresh tokens deles.
// Falha em qualquer passo desfaz tudo; vendedor inexistente → 404 sem
// alterar usuário.
func (c *Casos) DesligarVendedor(ctx context.Context, p kernel.Principal, id int64) (v *comercial.Vendedor, err error) {
	defer vlog.Trace(arqVendedores, "Comercial.DesligarVendedor", "id=%d", id)(&err)
	if err = exigirAdmin(p); err != nil {
		return nil, err
	}
	var inativados, revogados int64
	err = c.d.Desligamento.WithTx(ctx, func(r TxDesligamento) error {
		if err := r.Vendedores.MarcarDesligamento(ctx, id, c.agora()); err != nil {
			return err
		}
		var err error
		if inativados, err = r.Identidade.InativarUsuariosDoVendedor(ctx, id); err != nil {
			return err
		}
		revogados, err = r.Identidade.RevogarSessoesDoVendedor(ctx, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqVendedores, "Comercial.DesligarVendedor", "decisão desligado id=%d usuarios_inativados=%d refresh_tokens_revogados=%d", id, inativados, revogados)
	log.Printf("[vendedores] inativado: id=%d por usuario role=%s", id, p.Role())
	return appkit.RelerAtualizado(ctx, "comercialapp", id, c.d.Vendedores.PorID)
}

// ReativarVendedor limpa data_desligamento. Os usuários NÃO são reativados
// (o admin pode tê-los inativado por outro motivo).
func (c *Casos) ReativarVendedor(ctx context.Context, p kernel.Principal, id int64) (v *comercial.Vendedor, err error) {
	defer vlog.Trace(arqVendedores, "Comercial.ReativarVendedor", "id=%d", id)(&err)
	if err = exigirAdmin(p); err != nil {
		return nil, err
	}
	if err = c.d.Vendedores.DefinirDesligamento(ctx, id, nil); err != nil {
		return nil, err
	}
	log.Printf("[vendedores] reativado: id=%d por usuario role=%s", id, p.Role())
	return appkit.RelerAtualizado(ctx, "comercialapp", id, c.d.Vendedores.PorID)
}
