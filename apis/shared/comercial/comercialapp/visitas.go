package comercialapp

import (
	"context"
	"log"

	"github.com/rotaperfumes/shared/appkit"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqVisitas = "visitas.go"

func vendedorDaVisita(v *comercial.Visita) int64 { return v.VendedorID }

// ListarVisitas pagina as visitas do escopo (E-03: o vendedor_id da query é
// ignorado no restrito; sem carteira, vazio sem SQL).
func (c *Casos) ListarVisitas(ctx context.Context, p kernel.Principal, f comercial.FiltroVisita, pg kernel.Pagina) (r kernel.Resultado[comercial.Visita], err error) {
	defer vlog.Trace(arqVisitas, "Comercial.ListarVisitas", "page=%d limit=%d busca=%t ordem=%s",
		pg.Numero(), pg.Limite(), f.Busca != "", f.Ordenacao.Campo())(&err)
	e, err := c.escopo.Resolver(ctx, p)
	if err != nil {
		return r, err
	}
	if e.SemAcesso() {
		return vazio[comercial.Visita](pg), nil
	}
	return c.d.Visitas.Listar(ctx, e, f, pg)
}

// ObterVisita devolve a visita; de outro vendedor responde como inexistente.
func (c *Casos) ObterVisita(ctx context.Context, p kernel.Principal, id int64) (v *comercial.Visita, err error) {
	defer vlog.Trace(arqVisitas, "Comercial.ObterVisita", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, comercial.ErrVisitaNaoEncontrada)
	if err != nil {
		return nil, err
	}
	return obterNoEscopo(ctx, e, id, c.d.Visitas.PorID, vendedorDaVisita, comercial.ErrVisitaNaoEncontrada)
}

// CriarVisita: sem carteira, 403 antes do decode; restrito grava sempre com
// o próprio vendedor e só para cliente da carteira ativa (E-05).
func (c *Casos) CriarVisita(ctx context.Context, p kernel.Principal, ler Leitor[comercial.DadosVisita]) (v *comercial.Visita, err error) {
	defer vlog.Trace(arqVisitas, "Comercial.CriarVisita", "user_id=%d", p.UserID())(&err)
	e, err := c.noEscopo(ctx, p, comercial.ErrSemVendedor)
	if err != nil {
		return nil, err
	}
	if v, err = c.visitaDoComando(ctx, e, ler); err != nil {
		return nil, err
	}
	if err = c.d.Visitas.Inserir(ctx, v); err != nil {
		return nil, err
	}
	log.Printf("[visitas] criada: id=%d por usuario role=%s", v.VisitaID, p.Role())
	return appkit.RelerCriado(ctx, "[visitas] criada, mas falhou a releitura: id", v.VisitaID, v, c.d.Visitas.PorID), nil
}

// AtualizarVisita: a posse do registro atual é checada antes do decode; o
// vendedor não pode ser reatribuído por restrito (E-05).
func (c *Casos) AtualizarVisita(ctx context.Context, p kernel.Principal, id int64, ler Leitor[comercial.DadosVisita]) (v *comercial.Visita, err error) {
	defer vlog.Trace(arqVisitas, "Comercial.AtualizarVisita", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, comercial.ErrVisitaNaoEncontrada)
	if err != nil {
		return nil, err
	}
	if _, err = obterNoEscopo(ctx, e, id, c.d.Visitas.PorID, vendedorDaVisita, comercial.ErrVisitaNaoEncontrada); err != nil {
		return nil, err
	}
	if v, err = c.visitaDoComando(ctx, e, ler); err != nil {
		return nil, err
	}
	if err = c.d.Visitas.Atualizar(ctx, id, v); err != nil {
		return nil, err
	}
	log.Printf("[visitas] atualizada: id=%d por usuario role=%s", id, p.Role())
	return appkit.RelerAtualizado(ctx, "comercialapp", id, c.d.Visitas.PorID)
}

// ExcluirVisita remove a visita (hard delete), só da própria carteira.
func (c *Casos) ExcluirVisita(ctx context.Context, p kernel.Principal, id int64) (err error) {
	defer vlog.Trace(arqVisitas, "Comercial.ExcluirVisita", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, comercial.ErrVisitaNaoEncontrada)
	if err != nil {
		return err
	}
	if _, err = obterNoEscopo(ctx, e, id, c.d.Visitas.PorID, vendedorDaVisita, comercial.ErrVisitaNaoEncontrada); err != nil {
		return err
	}
	if err = c.d.Visitas.Excluir(ctx, id); err != nil {
		return err
	}
	log.Printf("[visitas] excluída: id=%d por usuario role=%s", id, p.Role())
	return nil
}

// visitaDoComando lê o body, aplica a política de escrita e valida, na
// ordem do legado: campos, existência de cliente e vendedor, data.
func (c *Casos) visitaDoComando(ctx context.Context, e kernel.Escopo, ler Leitor[comercial.DadosVisita]) (*comercial.Visita, error) {
	d, err := ler()
	if err != nil {
		return nil, err
	}
	if d.VendedorID, err = c.vendedorDoComando(ctx, e, d.ClienteID, d.VendedorID); err != nil {
		return nil, err
	}
	if err = d.Validar(); err != nil {
		return nil, err
	}
	if err = c.exigirClienteEVendedor(ctx, d.ClienteID, d.VendedorID); err != nil {
		return nil, err
	}
	return d.Visita()
}
