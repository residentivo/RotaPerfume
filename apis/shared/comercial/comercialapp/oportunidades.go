package comercialapp

import (
	"context"
	"log"
	"time"

	"github.com/rotaperfumes/shared/appkit"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqOportunidades = "oportunidades.go"

func vendedorDaOportunidade(o *comercial.Oportunidade) int64 { return o.VendedorID }

// ListarOportunidades pagina as oportunidades do escopo (E-03).
func (c *Casos) ListarOportunidades(ctx context.Context, p kernel.Principal, f comercial.FiltroOportunidade, pg kernel.Pagina) (r kernel.Resultado[comercial.Oportunidade], err error) {
	defer vlog.Trace(arqOportunidades, "Comercial.ListarOportunidades", "page=%d limit=%d busca=%t ordem=%s",
		pg.Numero(), pg.Limite(), f.Busca != "", f.Ordenacao.Campo())(&err)
	e, err := c.escopo.Resolver(ctx, p)
	if err != nil {
		return r, err
	}
	if e.SemAcesso() {
		return vazio[comercial.Oportunidade](pg), nil
	}
	return c.d.Oportunidades.Listar(ctx, e, f, pg)
}

// ObterOportunidade devolve a oportunidade; de outro vendedor responde como
// inexistente (E-04).
func (c *Casos) ObterOportunidade(ctx context.Context, p kernel.Principal, id int64) (o *comercial.Oportunidade, err error) {
	defer vlog.Trace(arqOportunidades, "Comercial.ObterOportunidade", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, comercial.ErrOportunidadeNaoEncontrada)
	if err != nil {
		return nil, err
	}
	return obterNoEscopo(ctx, e, id, c.d.Oportunidades.PorID, vendedorDaOportunidade, comercial.ErrOportunidadeNaoEncontrada)
}

// CriarOportunidade: sem carteira, 403 antes do decode; restrito grava com
// o próprio vendedor e só para cliente da carteira ativa (E-05).
// data_abertura vazia vira agora.
func (c *Casos) CriarOportunidade(ctx context.Context, p kernel.Principal, ler Leitor[comercial.DadosOportunidade]) (o *comercial.Oportunidade, err error) {
	defer vlog.Trace(arqOportunidades, "Comercial.CriarOportunidade", "user_id=%d", p.UserID())(&err)
	e, err := c.noEscopo(ctx, p, comercial.ErrSemVendedor)
	if err != nil {
		return nil, err
	}
	agora := c.agora()
	if o, err = c.oportunidadeDoComando(ctx, e, ler, &agora); err != nil {
		return nil, err
	}
	if err = c.d.Oportunidades.Inserir(ctx, o); err != nil {
		return nil, err
	}
	log.Printf("[oportunidades] criada: id=%d por usuario role=%s", o.OportunidadeID, p.Role())
	return appkit.RelerCriado(ctx, "[oportunidades] criada, mas falhou a releitura: id", o.OportunidadeID, o, c.d.Oportunidades.PorID), nil
}

// AtualizarOportunidade: posse do registro atual antes do decode;
// data_abertura obrigatória.
func (c *Casos) AtualizarOportunidade(ctx context.Context, p kernel.Principal, id int64, ler Leitor[comercial.DadosOportunidade]) (o *comercial.Oportunidade, err error) {
	defer vlog.Trace(arqOportunidades, "Comercial.AtualizarOportunidade", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, comercial.ErrOportunidadeNaoEncontrada)
	if err != nil {
		return nil, err
	}
	if _, err = obterNoEscopo(ctx, e, id, c.d.Oportunidades.PorID, vendedorDaOportunidade, comercial.ErrOportunidadeNaoEncontrada); err != nil {
		return nil, err
	}
	if o, err = c.oportunidadeDoComando(ctx, e, ler, nil); err != nil {
		return nil, err
	}
	if err = c.d.Oportunidades.Atualizar(ctx, id, o); err != nil {
		return nil, err
	}
	log.Printf("[oportunidades] atualizada: id=%d por usuario role=%s", id, p.Role())
	return appkit.RelerAtualizado(ctx, "comercialapp", id, c.d.Oportunidades.PorID)
}

// ExcluirOportunidade remove a oportunidade (hard delete), só da própria
// carteira.
func (c *Casos) ExcluirOportunidade(ctx context.Context, p kernel.Principal, id int64) (err error) {
	defer vlog.Trace(arqOportunidades, "Comercial.ExcluirOportunidade", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, comercial.ErrOportunidadeNaoEncontrada)
	if err != nil {
		return err
	}
	if _, err = obterNoEscopo(ctx, e, id, c.d.Oportunidades.PorID, vendedorDaOportunidade, comercial.ErrOportunidadeNaoEncontrada); err != nil {
		return err
	}
	if err = c.d.Oportunidades.Excluir(ctx, id); err != nil {
		return err
	}
	log.Printf("[oportunidades] excluída: id=%d por usuario role=%s", id, p.Role())
	return nil
}

// oportunidadeDoComando lê o body, aplica a política de escrita e valida na
// ordem do legado: campos, existência de cliente e vendedor, datas.
func (c *Casos) oportunidadeDoComando(ctx context.Context, e kernel.Escopo, ler Leitor[comercial.DadosOportunidade], agora *time.Time) (*comercial.Oportunidade, error) {
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
	return d.Oportunidade(agora)
}
